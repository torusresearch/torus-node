package common

import (
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/assert"
)

func TestPointToAddress(t *testing.T) {
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Error("could not generate key", err)
	}
	correctAddr := crypto.PubkeyToAddress(key.PublicKey)
	addr, err := PointToEthAddress(Point{*key.PublicKey.X, *key.PublicKey.Y})
	if err != nil {
		t.Error("could not generate key", err)
	}
	t.Log(addr.String())
	t.Log(correctAddr.String())
	assert.True(t, addr.String() == correctAddr.String())

}
