package store

import (
	"errors"
	"fmt"
	"strings"

	otpm "github.com/dim13/otpauth/migration"
)

// ErrEmptyName is returned by Rename when the trimmed name is empty.
var ErrEmptyName = errors.New("name must not be empty")

// normalize converts the legacy Payloads format into OtpParams, the same
// conversion Read performs.
func (s *Storage) normalize() {
	if s.Payloads == nil {
		return
	}
	var parameters []*otpm.Payload_OtpParameters
	for _, payload := range s.Payloads {
		parameters = append(parameters, payload.OtpParameters...)
	}
	s.OtpParams = parameters
	s.Payloads = nil
}

// Rename sets the issuer and name of the entry at index. Both values are
// trimmed; the name must not be empty, the issuer may be. The secret and all
// other fields are left unchanged. The caller is responsible for persisting
// the storage (see Write).
func (s *Storage) Rename(index int, issuer, name string) error {
	s.normalize()
	if index < 0 || index >= len(s.OtpParams) {
		return fmt.Errorf("index %d out of range", index)
	}
	issuer = strings.TrimSpace(issuer)
	name = strings.TrimSpace(name)
	if name == "" {
		return ErrEmptyName
	}
	param := s.OtpParams[index]
	param.Issuer = issuer
	param.Name = name
	return nil
}
