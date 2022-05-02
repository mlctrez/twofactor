package store

import (
	"fmt"
	"sort"

	"github.com/dim13/otpauth/migration"
	"github.com/maxence-charriere/go-app/v9/pkg/app"
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

func (s *Storage) Parameters() (parameters []*migration.Payload_OtpParameters) {
	if s == nil || s.Payloads == nil {
		return
	}
	for _, payload := range s.Payloads {
		for _, parameter := range payload.OtpParameters {
			parameters = append(parameters, parameter)
		}
	}
	return parameters
}

func Read(ctx app.Context, storage *Storage) (parameters []*migration.Payload_OtpParameters) {
	ctx.GetState(StorageKey, storage)
	return storage.Parameters()
}

func Write(ctx app.Context, storage *Storage) (parameters []*migration.Payload_OtpParameters) {
	ctx.SetState(StorageKey, storage, app.Persist)
	return storage.Parameters()
}
