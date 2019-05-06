package secp256k1

// TEST TO COMPARE HASH FUNCS AND UPDATE
// import (
// 	"crypto/rand"
// 	"testing"

// 	"bytes"

// 	oldSha3 "github.com/ethereum/go-ethereum/crypto/sha3"
// )

// func oldKeccak256(data ...[]byte) []byte {
// 	d := oldSha3.NewKeccak256()
// 	for _, b := range data {
// 		d.Write(b)
// 	}
// 	return d.Sum(nil)
// }

// func TestNewHashFunc(t *testing.T) {
// 	ranInt, _ := rand.Int(rand.Reader, GeneratorOrder)
// 	hashedOld := oldKeccak256(ranInt.Bytes())
// 	hashedNew := Keccak256(ranInt.Bytes())
// 	if bytes.Compare(hashedOld, hashedNew) != 0 {
// 		t.Fatalf("Hashes should be equal")
// 	}

// }
