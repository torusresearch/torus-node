package common

import (
	"math/big"

	cmn "github.com/tendermint/tendermint/libs/common"
)

type SigncryptedOutput struct {
	NodePubKey       Point
	NodeIndex        int
	SigncryptedShare Signcryption
}

type Signcryption struct {
	Ciphertext []byte
	R          Point
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

type Point struct {
	X big.Int
	Y big.Int
}

type Hash struct {
	cmn.HexBytes
}

type Node struct {
	Index  int
	PubKey Point
}
