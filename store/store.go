package store

import (
	"encoding/base32"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/dim13/otpauth/migration"
	"github.com/makiuchi-d/gozxing"
	"github.com/makiuchi-d/gozxing/qrcode"
	"github.com/maxence-charriere/go-app/v9/pkg/app"
	"github.com/mlctrez/imgtofactbp/components/clipboard"
	"github.com/mlctrez/imgtofactbp/conversions"
	"github.com/pquerna/otp"
)

const StorageKey = "twoFactor_storage"

type Storage struct {
	Payloads []*migration.Payload `json:"payloads"`
}

func (s *Storage) Add(payload *migration.Payload) error {
	if s.Payloads == nil {
		s.Payloads = []*migration.Payload{payload}
		return nil
	}
	for _, existing := range s.Payloads {
		if existing.BatchId == payload.BatchId && existing.BatchIndex == payload.BatchIndex {
			return fmt.Errorf("not replacing existing batch")
		}
	}
	s.Payloads = append(s.Payloads, payload)
	sort.SliceStable(s.Payloads, func(i, j int) bool {
		return s.Payloads[i].BatchIndex < s.Payloads[j].BatchIndex
	})
	return nil
}

type Parameter struct {
	*migration.Payload_OtpParameters
}

func (s *Storage) Parameters() (parameters []*Parameter) {
	if s == nil || s.Payloads == nil {
		return
	}
	for _, payload := range s.Payloads {
		for _, parameter := range payload.OtpParameters {
			parameters = append(parameters, &Parameter{parameter})
		}
	}
	return parameters
}

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

	payload, err := migration.UnmarshalURL(text)
	if err != nil {
		return err
	}
	return s.Add(payload)
}

func parseTotpFromString(text string) (*migration.Payload_OtpParameters, error) {
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
	var algorithm migration.Payload_Algorithm
	switch u.Query().Get("algorithm") {
	case "SHA1":
		algorithm = migration.Payload_ALGORITHM_SHA1
	case "SHA256":
		algorithm = migration.Payload_ALGORITHM_SHA256
	case "SHA512":
		algorithm = migration.Payload_ALGORITHM_SHA512
	case "MD5":
		algorithm = migration.Payload_ALGORITHM_MD5
	default:
		return nil, fmt.Errorf("unsupported algorithm %q", u.Query().Get("algorithm"))
	}
	var digits migration.Payload_DigitCount
	switch u.Query().Get("digits") {
	case "6":
		digits = migration.Payload_DIGIT_COUNT_SIX
	case "8":
		digits = migration.Payload_DIGIT_COUNT_EIGHT
	default:
		return nil, fmt.Errorf("unsupported digits %q", u.Query().Get("digits"))
	}

	var keyType migration.Payload_OtpType
	switch key.Type() {
	case "totp":
		keyType = migration.Payload_OTP_TYPE_TOTP
	case "hotp":
		keyType = migration.Payload_OTP_TYPE_HOTP
	default:
		return nil, fmt.Errorf("unsupported type %q", key.Type())
	}

	newParams := &migration.Payload_OtpParameters{
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

	added := false
	for _, payload := range s.Payloads {
		if len(payload.OtpParameters) < 10 {
			added = true
			payload.OtpParameters = append(payload.OtpParameters, newParams)
		}
	}

	if !added {
		var batchId int64
		batchId, err = strconv.ParseInt(time.Now().UTC().Format("2006010215"), 10, 32)
		if err != nil {
			return err
		}

		return s.Add(&migration.Payload{
			OtpParameters: []*migration.Payload_OtpParameters{newParams},
			Version:       1,
			BatchSize:     1,
			BatchIndex:    0,
			BatchId:       int32(batchId)},
		)
	}

	return nil

}

func Read(ctx app.Context, storage *Storage) (parameters []*Parameter) {
	ctx.GetState(StorageKey, storage)
	return storage.Parameters()
}

func Write(ctx app.Context, storage *Storage) (parameters []*Parameter) {
	ctx.SetState(StorageKey, storage, app.Persist)
	return storage.Parameters()
}
