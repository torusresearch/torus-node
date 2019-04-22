package dkgnode

import (
	"testing"

	ethCrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/torusresearch/torus-public/secp256k1"
)

func TestECDSASignAndVerify(t *testing.T) {
	ecdsaKey, _ := ethCrypto.HexToECDSA("9945a4770ba9a71a5b0c82d7da734a541352b90d915a0b51071eb6681827e370")
	testStr := "this is a test message"
	signature := ECDSASign([]byte(testStr), ecdsaKey)
	valid := ECDSAVerify(ecdsaKey.PublicKey, signature)
	assert.False(t, valid)
}

func TestECDSASignAndVerifyFromRaw(t *testing.T) {
	ecdsaKey, _ := ethCrypto.HexToECDSA("9945a4770ba9a71a5b0c82d7da734a541352b90d915a0b51071eb6681827e370")
	testStr := "this is a test message"
	signature := ECDSASign([]byte(testStr), ecdsaKey)
	valid := ECDSAVerifyFromRaw(testStr, ecdsaKey.PublicKey, signature.Raw)
	assert.True(t, valid)
}

func TestECDSASigHex(t *testing.T) {
	ecdsaKey, _ := ethCrypto.HexToECDSA("9945a4770ba9a71a5b0c82d7da734a541352b90d915a0b51071eb6681827e370")
	hash := secp256k1.Keccak256([]byte("this is a test message"))
	signature := ECDSASign(hash, ecdsaKey)
	hex := ECDSASigToHex(signature)
	ecdsaSig := HexToECDSASig(hex)
	var hash32 [32]byte
	copy(hash32[:], hash[:32])
	ecdsaSignature := ECDSASignature{
		ecdsaSig.Raw,
		hash32,
		ecdsaSig.R,
		ecdsaSig.S,
		ecdsaSig.V,
	}
	assert.ObjectsAreEqualValues(signature, ecdsaSignature)
}
