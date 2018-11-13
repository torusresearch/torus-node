package dkgnode

import (
	"testing"

	ethCrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/assert"
)

func TestECDSASignAndVerify(t *testing.T) {
	ecdsaKey, _ := ethCrypto.HexToECDSA("9945a4770ba9a71a5b0c82d7da734a541352b90d915a0b51071eb6681827e370")
	hash := ethCrypto.Keccak256([]byte("this is a test message"))
	signature := ECDSASign(hash, ecdsaKey)
	valid := ECDSAVerify(ecdsaKey.PublicKey, signature)
	assert.True(t, valid)
}
