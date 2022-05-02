package store

import (
	"testing"

	"github.com/dim13/otpauth/migration"
	"github.com/stretchr/testify/require"
)

func TestStorage_Add(t *testing.T) {
	req := require.New(t)
	s := &Storage{}

	p1 := &migration.Payload{BatchId: 1234, BatchIndex: 1}
	req.Nil(s.Add(p1))
	req.Equal(p1, s.Payloads[0])

	req.NotNil(s.Add(p1))

	p2 := &migration.Payload{BatchId: 1234, BatchIndex: 0}
	req.Nil(s.Add(p2))
	req.Equal(p2, s.Payloads[0])
	req.Equal(p1, s.Payloads[1])

}
