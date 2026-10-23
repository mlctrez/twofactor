package store

import (
	"bytes"
	"errors"
	"testing"

	otpm "github.com/dim13/otpauth/migration"
)

// dummy secrets only
func renameStorage() *Storage {
	return &Storage{OtpParams: []*otpm.Payload_OtpParameters{
		{Secret: []byte("dummy-secret-0001"), Name: "alice", Issuer: "Example",
			Algorithm: otpm.Payload_ALGORITHM_SHA256, Digits: otpm.Payload_DIGIT_COUNT_EIGHT, Type: otpm.Payload_OTP_TYPE_TOTP},
		{Secret: []byte("dummy-secret-0002"), Name: "bob"},
	}}
}

func TestRename(t *testing.T) {
	s := renameStorage()
	if err := s.Rename(0, "  New Issuer ", "  carol@example.com  "); err != nil {
		t.Fatal(err)
	}
	p := s.OtpParams[0]
	if p.Issuer != "New Issuer" || p.Name != "carol@example.com" {
		t.Errorf("got issuer %q name %q", p.Issuer, p.Name)
	}
	if !bytes.Equal(p.Secret, []byte("dummy-secret-0001")) || p.Algorithm != otpm.Payload_ALGORITHM_SHA256 ||
		p.Digits != otpm.Payload_DIGIT_COUNT_EIGHT || p.Type != otpm.Payload_OTP_TYPE_TOTP {
		t.Errorf("rename changed other fields: %+v", p)
	}
	if s.OtpParams[1].Name != "bob" {
		t.Errorf("rename touched another entry")
	}

	// issuer may be cleared
	if err := s.Rename(1, "   ", "robert"); err != nil {
		t.Fatal(err)
	}
	if s.OtpParams[1].Issuer != "" || s.OtpParams[1].Name != "robert" {
		t.Errorf("got %+v", s.OtpParams[1])
	}
}

func TestRenameRejectsEmptyName(t *testing.T) {
	s := renameStorage()
	for _, name := range []string{"", "   ", "\t\n"} {
		if err := s.Rename(0, "Issuer", name); !errors.Is(err, ErrEmptyName) {
			t.Errorf("Rename with name %q: got %v want ErrEmptyName", name, err)
		}
	}
	if s.OtpParams[0].Name != "alice" || s.OtpParams[0].Issuer != "Example" {
		t.Errorf("failed rename modified entry: %+v", s.OtpParams[0])
	}
}

func TestRenameBounds(t *testing.T) {
	s := renameStorage()
	for _, index := range []int{-1, 2, 100} {
		if err := s.Rename(index, "i", "n"); err == nil {
			t.Errorf("Rename(%d): expected out of range error", index)
		}
	}
	if err := (&Storage{}).Rename(0, "i", "n"); err == nil {
		t.Error("Rename on empty storage: expected error")
	}
}

func TestRenameLegacyPayloads(t *testing.T) {
	s := &Storage{Payloads: []*otpm.Payload{
		{OtpParameters: []*otpm.Payload_OtpParameters{{Secret: []byte("dummy-a"), Name: "a"}}},
		{OtpParameters: []*otpm.Payload_OtpParameters{{Secret: []byte("dummy-b"), Name: "b"}}},
	}}
	if err := s.Rename(1, "Issuer", "renamed"); err != nil {
		t.Fatal(err)
	}
	if s.Payloads != nil {
		t.Error("expected legacy payloads to be converted")
	}
	if len(s.OtpParams) != 2 || s.OtpParams[0].Name != "a" || s.OtpParams[1].Name != "renamed" ||
		s.OtpParams[1].Issuer != "Issuer" || string(s.OtpParams[1].Secret) != "dummy-b" {
		t.Errorf("unexpected params after rename: %+v", s.OtpParams)
	}
}
