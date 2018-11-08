package message

import (
	"bytes"
	"crypto/ecdsa"
	"encoding/gob"

	"github.com/insolar/insolar/core"
	crypto_helper "github.com/insolar/insolar/cryptohelpers/ecdsa"
	"github.com/insolar/insolar/cryptohelpers/hash"
	"github.com/pkg/errors"
)

// RoutingToken is an auth token for coordinating messages
type RoutingToken struct {
	To      *core.RecordRef
	from    *core.RecordRef
	pulse   core.PulseNumber
	msgHash []byte
	sign    []byte
}

func (t *RoutingToken) GetTo() *core.RecordRef {
	return t.To
}

func (t *RoutingToken) GetFrom() *core.RecordRef {
	return t.from
}

func (t *RoutingToken) GetPulse() core.PulseNumber {
	return t.pulse
}

func (t *RoutingToken) GetMsgHash() []byte {
	return t.msgHash
}

func (t *RoutingToken) GetSign() []byte {
	return t.sign
}

// NewToken creates new token with sign of its fields
func NewToken(to *core.RecordRef, from *core.RecordRef, pulseNumber core.PulseNumber, msgHash []byte, key *ecdsa.PrivateKey) *RoutingToken {
	token := &RoutingToken{
		To:      to,
		from:    from,
		msgHash: msgHash,
		pulse:   pulseNumber,
	}

	var tokenBuffer bytes.Buffer
	enc := gob.NewEncoder(&tokenBuffer)
	err := enc.Encode(token)
	if err != nil {
		panic(err)
	}

	sign, err := crypto_helper.Sign(tokenBuffer.Bytes(), key)
	if err != nil {
		panic(err)
	}
	token.sign = sign
	return token
}

// ValidateToken checks that a routing token is valid
func ValidateToken(pubKey *ecdsa.PublicKey, msg core.SignedMessage) error {
	serialized, err := ToBytes(msg.Message())
	if err != nil {
		return errors.Wrap(err, "filed GetTo serialize message")
	}
	msgHash := hash.SHA3Bytes256(serialized)
	token := RoutingToken{
		To:      msg.GetToken().GetTo(),
		from:    msg.GetToken().GetFrom(),
		msgHash: msgHash,
		pulse:   msg.Pulse(),
	}

	var tokenBuffer bytes.Buffer
	enc := gob.NewEncoder(&tokenBuffer)
	err = enc.Encode(token)
	if err != nil {
		panic(err)
	}

	ok, err := crypto_helper.VerifyWithFullKey(tokenBuffer.Bytes(), msg.GetToken().GetSign(), pubKey)
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("token isn't valid")
	}

	return nil
}
