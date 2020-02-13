package common

import (
	"math/big"

	cmn "github.com/torusresearch/tendermint/libs/common"
	"github.com/torusresearch/torus-common/common"
)

const (
	Delimiter1 = "\x1c"
	Delimiter2 = "\x1d"
	Delimiter3 = "\x1e"
	Delimiter4 = "\x1f"
)

type SigncryptedOutput struct {
	NodePubKey       common.Point
	NodeIndex        int
	SigncryptedShare Signcryption
}

type Signcryption struct {
	Ciphertext []byte
	R          common.Point
	Signature  big.Int
}

type PrimaryPolynomial struct {
	Coeff     []big.Int
	Threshold int
}

type PrimaryShare struct {
	Index int
	Value big.Int
}

type Hash struct {
	cmn.HexBytes
}

type Node struct {
	Index  int
	PubKey common.Point
}

type VerifierData struct {
	Ok                   bool
	Verifier, VerifierID string
	KeyIndexes           []big.Int
	Err                  error
}

type Iterator struct {
	RandomID string
	CallNext func() bool
	Val      interface{}
	Err      error
}

func (iterator *Iterator) Next() bool {
	return iterator.CallNext()
}
func (iterator *Iterator) Value() (val interface{}) {
	return iterator.Val
}
