package store

import (
	"encoding/base32"
	"encoding/json"
	"fmt"
	otpm "github.com/dim13/otpauth/migration"
	"github.com/makiuchi-d/gozxing"
	"github.com/makiuchi-d/gozxing/qrcode"
	"github.com/maxence-charriere/go-app/v9/pkg/app"
	"github.com/mlctrez/imgtofactbp/components/clipboard"
	"github.com/mlctrez/imgtofactbp/conversions"
	"github.com/pquerna/otp"
	"net/url"
	"os"
	"strings"
)

const StorageKey = "twoFactor_storage"

type Storage struct {
	Payloads  []*otpm.Payload               `json:"payloads,omitempty"`
	OtpParams []*otpm.Payload_OtpParameters `json:"parameters,omitempty"`
}

//func (s *Storage) Add(payload *otpm.Payload) error {
//	if s.Payloads == nil {
//		s.Payloads = []*otpm.Payload{payload}
//		return nil
//	}
//	for _, existing := range s.Payloads {
//		if existing.BatchId == payload.BatchId && existing.BatchIndex == payload.BatchIndex {
//			return fmt.Errorf("not replacing existing batch")
//		}
//	}
//	s.Payloads = append(s.Payloads, payload)
//	sort.SliceStable(s.Payloads, func(i, j int) bool {
//		return s.Payloads[i].BatchIndex < s.Payloads[j].BatchIndex
//	})
//	return nil
//}

func (s *Storage) Paste(evt *clipboard.PasteData) error {
	if evt == nil {
		return nil
	}

	if !strings.HasPrefix(evt.Data, "data:image") {
		// ignore pasted text
		return nil
	}

	img, _, err := conversions.Base64ToImage(evt.Data)
	if err != nil {
		return err
	}
	bmp, _ := gozxing.NewBinaryBitmapFromImage(img)
	qrReader := qrcode.NewQRCodeReader()
	result, err := qrReader.Decode(bmp, nil)
	if err != nil {
		return err
	}

	text := result.GetText()

	if strings.HasPrefix(text, "otpauth://totp") {
		return s.AddTotp(text)
	}

	payload, err := otpm.UnmarshalURL(text)
	if err != nil {
		return err
	}
	s.OtpParams = append(s.OtpParams, payload.OtpParameters...)
	return nil
}

func parseTotpFromString(text string) (*otpm.Payload_OtpParameters, error) {
	key, err := otp.NewKeyFromURL(text)
	if err != nil {
		return nil, err
	}

	secret, err := base32.StdEncoding.DecodeString(key.Secret())
	if err != nil {
		return nil, err
	}

	u, err := url.Parse(key.URL())
	if err != nil {
		return nil, err
	}

	// otpauth://totp/sample
	// ?algorithm=SHA1
	// &digits=6
	// &issuer=Proper+Key
	// &period=30
	// &secret=xxxxx
	algorithm := otpm.Payload_ALGORITHM_SHA1
	switch u.Query().Get("algorithm") {
	case "SHA1":
		algorithm = otpm.Payload_ALGORITHM_SHA1
	case "SHA256":
		algorithm = otpm.Payload_ALGORITHM_SHA256
	case "SHA512":
		algorithm = otpm.Payload_ALGORITHM_SHA512
	case "MD5":
		algorithm = otpm.Payload_ALGORITHM_MD5
	}
	digits := otpm.Payload_DIGIT_COUNT_SIX
	switch u.Query().Get("digits") {
	case "6":
		digits = otpm.Payload_DIGIT_COUNT_SIX
	case "8":
		digits = otpm.Payload_DIGIT_COUNT_EIGHT
	}

	var keyType otpm.Payload_OtpType
	switch key.Type() {
	case "totp":
		keyType = otpm.Payload_OTP_TYPE_TOTP
	case "hotp":
		keyType = otpm.Payload_OTP_TYPE_HOTP
	default:
		return nil, fmt.Errorf("unsupported type %q", key.Type())
	}

	newParams := &otpm.Payload_OtpParameters{
		Secret:    secret,
		Name:      key.AccountName(),
		Issuer:    key.Issuer(),
		Algorithm: algorithm,
		Digits:    digits,
		Type:      keyType,
	}

	return newParams, nil
}

func (s *Storage) AddTotp(text string) error {

	newParams, err := parseTotpFromString(text)
	if err != nil {
		return err
	}

	s.OtpParams = append(s.OtpParams, newParams)
	return nil
}

func (s *Storage) Switch(ctx app.Context, start int, end int) {
	if start < 0 || end < 0 || start > len(s.OtpParams) || end > len(s.OtpParams) {
		return
	}
	s.OtpParams[start], s.OtpParams[end] = s.OtpParams[end], s.OtpParams[start]
	Write(ctx, s)
	ctx.Dispatch(nil)
}

func (s *Storage) Delete(ctx app.Context, start int, end int) {
	var newParms []*otpm.Payload_OtpParameters
	for i, param := range s.OtpParams {
		if i == start && end == 9999 {
			// nothing
		} else {
			newParms = append(newParms, param)
		}
	}
	s.OtpParams = newParms
	Write(ctx, s)
}

func Read(ctx app.Context, s *Storage) (parameters []*otpm.Payload_OtpParameters) {
	ctx.GetState(StorageKey, s)
	json.NewEncoder(os.Stdout).Encode(s)
	// Convert old payloads format to new parameters format
	if s.Payloads != nil {
		for _, payload := range s.Payloads {
			for _, parameter := range payload.OtpParameters {
				parameters = append(parameters, parameter)
			}
		}
		s.OtpParams = parameters
		s.Payloads = nil
		Write(ctx, s)
		return s.OtpParams
	}
	return s.OtpParams
}

func Write(ctx app.Context, storage *Storage) (parameters []*otpm.Payload_OtpParameters) {
	ctx.SetState(StorageKey, storage, app.Persist, app.Encrypt)
	return storage.OtpParams
}
