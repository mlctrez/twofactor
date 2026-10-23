package store

import (
	"encoding/base32"
	"errors"
	"fmt"
	"strings"
	"time"

	otpm "github.com/dim13/otpauth/migration"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

// TotpPeriod is the TOTP period in seconds. The otpauth migration format has
// no period field, so every entry uses the standard 30 second period.
const TotpPeriod = 30

var (
	// ErrNotFound is returned when no entry matches a label.
	ErrNotFound = errors.New("no entry matches label")
	// ErrAmbiguous is returned when more than one entry matches a label.
	ErrAmbiguous = errors.New("label matches more than one entry")
	// ErrNotTotp is returned when code generation is requested for a non TOTP entry.
	ErrNotTotp = errors.New("entry is not a TOTP entry")
)

// Code is the current TOTP code for a single entry. It never contains the secret.
type Code struct {
	Label     string `json:"label"`
	Code      string `json:"code"`
	Remaining int    `json:"remaining"`
	Period    int    `json:"period"`
}

// AmbiguousError reports the labels that matched when a lookup was ambiguous.
type AmbiguousError struct {
	Matches []string
}

func (e *AmbiguousError) Error() string {
	return fmt.Sprintf("%s (%d matches)", ErrAmbiguous.Error(), len(e.Matches))
}

func (e *AmbiguousError) Is(target error) bool { return target == ErrAmbiguous }

// Label returns the label for an entry exactly as the UI displays it:
// "Issuer Name", or just Name when there is no issuer or Name already
// starts with the issuer.
func Label(p *otpm.Payload_OtpParameters) string {
	if p.Issuer != "" && !strings.HasPrefix(p.Name, p.Issuer) {
		return fmt.Sprintf("%s %s", p.Issuer, p.Name)
	}
	return p.Name
}

// Parameters returns the entries in storage, flattening the legacy Payloads
// format the same way Read does in the UI.
func (s *Storage) Parameters() []*otpm.Payload_OtpParameters {
	if s.Payloads != nil {
		var parameters []*otpm.Payload_OtpParameters
		for _, payload := range s.Payloads {
			parameters = append(parameters, payload.OtpParameters...)
		}
		return parameters
	}
	return s.OtpParams
}

// Labels returns the display label of every entry, in storage order.
func (s *Storage) Labels() []string {
	labels := []string{}
	for _, p := range s.Parameters() {
		labels = append(labels, Label(p))
	}
	return labels
}

// Find returns the single entry whose label matches. An exact match is tried
// first, then a case-insensitive match. Surrounding whitespace in label is
// ignored. ErrNotFound or an *AmbiguousError (matching ErrAmbiguous via
// errors.Is) is returned when there is not exactly one match.
func (s *Storage) Find(label string) (*otpm.Payload_OtpParameters, error) {
	label = strings.TrimSpace(label)
	if label == "" {
		return nil, ErrNotFound
	}
	params := s.Parameters()
	matchers := []func(string) bool{
		func(l string) bool { return l == label },
		func(l string) bool { return strings.EqualFold(l, label) },
	}
	for _, match := range matchers {
		var found []*otpm.Payload_OtpParameters
		var labels []string
		for _, p := range params {
			if l := Label(p); match(l) {
				found = append(found, p)
				labels = append(labels, l)
			}
		}
		switch len(found) {
		case 0:
			continue
		case 1:
			return found[0], nil
		default:
			return nil, &AmbiguousError{Matches: labels}
		}
	}
	return nil, ErrNotFound
}

// GenerateCode computes the TOTP code for an entry at time t, honoring the
// entry's algorithm and digit count. Unspecified values fall back to the same
// defaults the UI uses (SHA1, 6 digits, TOTP). HOTP entries return ErrNotTotp.
func GenerateCode(p *otpm.Payload_OtpParameters, t time.Time) (*Code, error) {
	if p == nil {
		return nil, ErrNotFound
	}
	switch p.Type {
	case otpm.Payload_OTP_TYPE_TOTP, otpm.Payload_OTP_TYPE_UNSPECIFIED:
	default:
		return nil, ErrNotTotp
	}
	if len(p.Secret) == 0 {
		return nil, errors.New("entry has an empty secret")
	}

	var algorithm otp.Algorithm
	switch p.Algorithm {
	case otpm.Payload_ALGORITHM_SHA1, otpm.Payload_ALGORITHM_UNSPECIFIED:
		algorithm = otp.AlgorithmSHA1
	case otpm.Payload_ALGORITHM_SHA256:
		algorithm = otp.AlgorithmSHA256
	case otpm.Payload_ALGORITHM_SHA512:
		algorithm = otp.AlgorithmSHA512
	case otpm.Payload_ALGORITHM_MD5:
		algorithm = otp.AlgorithmMD5
	default:
		return nil, fmt.Errorf("unsupported algorithm %d", p.Algorithm)
	}

	var digits otp.Digits
	switch p.Digits {
	case otpm.Payload_DIGIT_COUNT_SIX, otpm.Payload_DIGIT_COUNT_UNSPECIFIED:
		digits = otp.DigitsSix
	case otpm.Payload_DIGIT_COUNT_EIGHT:
		digits = otp.DigitsEight
	default:
		return nil, fmt.Errorf("unsupported digit count %d", p.Digits)
	}

	code, err := totp.GenerateCodeCustom(base32.StdEncoding.EncodeToString(p.Secret), t, totp.ValidateOpts{
		Period:    TotpPeriod,
		Digits:    digits,
		Algorithm: algorithm,
	})
	if err != nil {
		// never wrap err with the secret; the otp library errors are static
		return nil, fmt.Errorf("generating code: %w", err)
	}

	elapsed := int(t.Unix() % TotpPeriod)
	if elapsed < 0 {
		elapsed += TotpPeriod
	}
	return &Code{
		Label:     Label(p),
		Code:      code,
		Remaining: TotpPeriod - elapsed,
		Period:    TotpPeriod,
	}, nil
}
