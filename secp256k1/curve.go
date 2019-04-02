package secp256k1

import (
	"math/big"

	secp256k1 "github.com/btcsuite/btcd/btcec"
	"github.com/ethereum/go-ethereum/crypto/sha3"
	"github.com/torusresearch/torus-public/common"
)

type KoblitzCurve struct {
	*secp256k1.KoblitzCurve
}

var (
	Curve = &KoblitzCurve{secp256k1.S256()}
	// field order, also known as p, usually used for scalars
	FieldOrder = common.HexToBigInt("fffffffffffffffffffffffffffffffffffffffffffffffffffffffefffffc2f")
	// group order, also known as q, it is the number of points in the curve, and is usually used in exponents
	GeneratorOrder = common.HexToBigInt("fffffffffffffffffffffffffffffffebaaedce6af48a03bbfd25e8cd0364141")
	// scalar to the power of this is like square root, eg. y^sqRoot = y^0.5 (if it exists)
	SqRoot = common.HexToBigInt("3fffffffffffffffffffffffffffffffffffffffffffffffffffffffbfffff0c")
	G      = common.Point{X: *Curve.Gx, Y: *Curve.Gy}
	H      = *HashToPoint(G.X.Bytes())
)

func HashToPoint(data []byte) *common.Point {
	keccakHash := Keccak256(data)
	x := new(big.Int)
	x.SetBytes(keccakHash)
	for {
		beta := new(big.Int)
		beta.Exp(x, big.NewInt(3), FieldOrder)
		beta.Add(beta, big.NewInt(7))
		beta.Mod(beta, FieldOrder)
		y := new(big.Int)
		y.Exp(beta, SqRoot, FieldOrder)
		if new(big.Int).Exp(y, big.NewInt(2), FieldOrder).Cmp(beta) == 0 {
			return &common.Point{X: *x, Y: *y}
		} else {
			x.Add(x, big.NewInt(1))
		}
	}
}

func Keccak256(data ...[]byte) []byte {
	d := sha3.NewKeccak256()
	for _, b := range data {
		d.Write(b)
	}
	return d.Sum(nil)
}
