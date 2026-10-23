package store

import (
	"errors"
	"testing"
	"time"

	otpm "github.com/dim13/otpauth/migration"
)

// RFC 6238 appendix B test seeds. These are public test vectors, not real secrets.
var (
	rfcSeedSHA1   = []byte("12345678901234567890")
	rfcSeedSHA256 = []byte("12345678901234567890123456789012")
	rfcSeedSHA512 = []byte("1234567890123456789012345678901234567890123456789012345678901234")
)

func TestGenerateCodeRFC6238(t *testing.T) {
	tests := []struct {
		unix      int64
		algorithm otpm.Payload_Algorithm
		seed      []byte
		want      string
	}{
		{59, otpm.Payload_ALGORITHM_SHA1, rfcSeedSHA1, "94287082"},
		{59, otpm.Payload_ALGORITHM_SHA256, rfcSeedSHA256, "46119246"},
		{59, otpm.Payload_ALGORITHM_SHA512, rfcSeedSHA512, "90693936"},
		{1111111109, otpm.Payload_ALGORITHM_SHA1, rfcSeedSHA1, "07081804"},
		{1111111109, otpm.Payload_ALGORITHM_SHA256, rfcSeedSHA256, "68084774"},
		{1111111109, otpm.Payload_ALGORITHM_SHA512, rfcSeedSHA512, "25091201"},
		{1234567890, otpm.Payload_ALGORITHM_SHA1, rfcSeedSHA1, "89005924"},
		{2000000000, otpm.Payload_ALGORITHM_SHA1, rfcSeedSHA1, "69279037"},
		{20000000000, otpm.Payload_ALGORITHM_SHA512, rfcSeedSHA512, "47863826"},
	}
	for _, tt := range tests {
		p := &otpm.Payload_OtpParameters{
			Secret:    tt.seed,
			Name:      "rfc",
			Algorithm: tt.algorithm,
			Digits:    otpm.Payload_DIGIT_COUNT_EIGHT,
			Type:      otpm.Payload_OTP_TYPE_TOTP,
		}
		got, err := GenerateCode(p, time.Unix(tt.unix, 0))
		if err != nil {
			t.Fatalf("%d %v: unexpected error %v", tt.unix, tt.algorithm, err)
		}
		if got.Code != tt.want {
			t.Errorf("%d %v: got %s want %s", tt.unix, tt.algorithm, got.Code, tt.want)
		}
	}
}

func TestGenerateCodeDefaultsAndRemaining(t *testing.T) {
	// unspecified algorithm, digits and type behave like the UI: SHA1, 6 digits, TOTP
	p := &otpm.Payload_OtpParameters{Secret: rfcSeedSHA1, Name: "alice", Issuer: "Example"}
	got, err := GenerateCode(p, time.Unix(59, 0))
	if err != nil {
		t.Fatal(err)
	}
	want := Code{Label: "Example alice", Code: "287082", Remaining: 1, Period: 30}
	if *got != want {
		t.Errorf("got %+v want %+v", *got, want)
	}

	got, err = GenerateCode(p, time.Unix(60, 0))
	if err != nil {
		t.Fatal(err)
	}
	if got.Remaining != 30 {
		t.Errorf("remaining at period start: got %d want 30", got.Remaining)
	}
}

func TestGenerateCodeRejectsHotp(t *testing.T) {
	p := &otpm.Payload_OtpParameters{Secret: rfcSeedSHA1, Name: "h", Type: otpm.Payload_OTP_TYPE_HOTP}
	if _, err := GenerateCode(p, time.Unix(59, 0)); !errors.Is(err, ErrNotTotp) {
		t.Fatalf("got %v want ErrNotTotp", err)
	}
}

func TestGenerateCodeEmptySecret(t *testing.T) {
	p := &otpm.Payload_OtpParameters{Name: "empty", Type: otpm.Payload_OTP_TYPE_TOTP}
	if _, err := GenerateCode(p, time.Unix(59, 0)); err == nil {
		t.Fatal("expected error for empty secret")
	}
}

func TestLabel(t *testing.T) {
	tests := []struct {
		name, issuer, want string
	}{
		{"alice@example.com", "", "alice@example.com"},
		{"alice@example.com", "Example", "Example alice@example.com"},
		{"Example:alice@example.com", "Example", "Example:alice@example.com"},
	}
	for _, tt := range tests {
		got := Label(&otpm.Payload_OtpParameters{Name: tt.name, Issuer: tt.issuer})
		if got != tt.want {
			t.Errorf("Label(%q,%q) = %q want %q", tt.name, tt.issuer, got, tt.want)
		}
	}
}

func testStorage() *Storage {
	return &Storage{OtpParams: []*otpm.Payload_OtpParameters{
		{Secret: []byte("dummy-secret-0001"), Name: "alice", Issuer: "GitHub"},
		{Secret: []byte("dummy-secret-0002"), Name: "github alice"},
		{Secret: []byte("dummy-secret-0003"), Name: "bob", Issuer: "AWS"},
		{Secret: []byte("dummy-secret-0004"), Name: "dup"},
		{Secret: []byte("dummy-secret-0005"), Name: "dup"},
		{Secret: []byte("dummy-secret-0006"), Name: "Mixed"},
		{Secret: []byte("dummy-secret-0007"), Name: "MIXED"},
	}}
}

func TestFind(t *testing.T) {
	s := testStorage()

	found := []struct{ query, wantSecret string }{
		{"GitHub alice", "dummy-secret-0001"}, // exact wins over case-insensitive
		{"github alice", "dummy-secret-0002"}, // exact wins over case-insensitive
		{"aws BOB", "dummy-secret-0003"},      // case-insensitive fallback
		{"  AWS bob  ", "dummy-secret-0003"},  // surrounding whitespace ignored
		{"Mixed", "dummy-secret-0006"},        // exact match beats two case-insensitive matches
	}
	for _, tt := range found {
		p, err := s.Find(tt.query)
		if err != nil {
			t.Errorf("Find(%q): unexpected error %v", tt.query, err)
			continue
		}
		if string(p.Secret) != tt.wantSecret {
			t.Errorf("Find(%q) returned wrong entry %q", tt.query, Label(p))
		}
	}

	for _, q := range []string{"nope", "", "GitHub"} {
		if _, err := s.Find(q); !errors.Is(err, ErrNotFound) {
			t.Errorf("Find(%q): got %v want ErrNotFound", q, err)
		}
	}

	for _, q := range []string{"dup", "mixed", "DUP"} {
		_, err := s.Find(q)
		if !errors.Is(err, ErrAmbiguous) {
			t.Errorf("Find(%q): got %v want ErrAmbiguous", q, err)
			continue
		}
		var ae *AmbiguousError
		if !errors.As(err, &ae) || len(ae.Matches) != 2 {
			t.Errorf("Find(%q): expected 2 matches, got %v", q, err)
		}
	}
}

func TestParametersLegacyPayloads(t *testing.T) {
	s := &Storage{Payloads: []*otpm.Payload{
		{OtpParameters: []*otpm.Payload_OtpParameters{{Secret: []byte("dummy-a"), Name: "a"}}},
		{OtpParameters: []*otpm.Payload_OtpParameters{{Secret: []byte("dummy-b"), Name: "b", Issuer: "X"}}},
	}}
	labels := s.Labels()
	if len(labels) != 2 || labels[0] != "a" || labels[1] != "X b" {
		t.Fatalf("got %v", labels)
	}
	if _, err := s.Find("x B"); err != nil {
		t.Fatal(err)
	}
}
