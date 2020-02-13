// This implements the NIZKPK presented in Distributed Key Generation in the Wild
// with the aims of increasing asynchronicity within AVSS
package pvss

import (
	"math/big"

	logging "github.com/sirupsen/logrus"
	"github.com/torusresearch/torus-common/common"
	"github.com/torusresearch/torus-common/secp256k1"
	"github.com/torusresearch/torus-node/pkcs7"
)

func pad(b []byte) []byte {
	byt, err := pkcs7.Pad(b, 64)
	if err != nil {
		logging.Error("Could not pad bytes")
		return b
	}
	return byt
}

// Generates NIZK Proof with the aims of increasing asynchronicity within AVSS
// Returns proof in the form of c, u1, u2
func GenerateNIZKPK(s big.Int, r big.Int) (big.Int, big.Int, big.Int) {
	// create randomness
	v1 := *RandomBigInt()
	v2 := *RandomBigInt()
	t1 := common.BigIntToPoint(secp256k1.Curve.ScalarBaseMult(v1.Bytes()))
	t2 := common.BigIntToPoint(secp256k1.Curve.ScalarMult(&secp256k1.H.X, &secp256k1.H.Y, v2.Bytes()))

	//create dlog and pederson commitments
	gs := common.BigIntToPoint(secp256k1.Curve.ScalarBaseMult(s.Bytes()))
	hr := common.BigIntToPoint(secp256k1.Curve.ScalarMult(&secp256k1.H.X, &secp256k1.H.Y, r.Bytes()))
	gshr := common.BigIntToPoint(secp256k1.Curve.Add(&gs.X, &gs.Y, &hr.X, &hr.Y))

	//prepare bytestring for hashing
	bytesToBeHashed := append(pad(secp256k1.G.X.Bytes()), pad(secp256k1.H.X.Bytes())...)
	bytesToBeHashed = append(bytesToBeHashed, pad(gs.X.Bytes())...)
	bytesToBeHashed = append(bytesToBeHashed, pad(gshr.X.Bytes())...)
	bytesToBeHashed = append(bytesToBeHashed, pad(t1.X.Bytes())...)
	bytesToBeHashed = append(bytesToBeHashed, pad(t2.X.Bytes())...)

	c := new(big.Int)
	c.SetBytes(secp256k1.Keccak256(bytesToBeHashed))

	u1 := new(big.Int)
	u1.Set(&v1)
	cs := new(big.Int)
	cs.Mul(c, &s)
	cs.Mod(cs, secp256k1.GeneratorOrder)
	u1.Sub(u1, cs)
	u1.Mod(u1, secp256k1.GeneratorOrder)

	u2 := new(big.Int)
	u2.Set(&v2)
	cr := new(big.Int)
	cr.Mul(c, &r)
	cr.Mod(cr, secp256k1.GeneratorOrder)
	u2.Sub(u2, cr)
	u2.Mod(u2, secp256k1.GeneratorOrder)

	return *c, *u1, *u2

}

// Generates NIZK Proof with commitments
// Returns proof in the form of c, u1, u2 gs and gshr
func GenerateNIZKPKWithCommitments(s big.Int, r big.Int) (c big.Int, u1 big.Int, u2 big.Int, gs common.Point, gshr common.Point) {
	c, u1, u2 = GenerateNIZKPK(s, r)
	gs = common.BigIntToPoint(secp256k1.Curve.ScalarBaseMult(s.Bytes()))
	hr := common.BigIntToPoint(secp256k1.Curve.ScalarMult(&secp256k1.H.X, &secp256k1.H.Y, r.Bytes()))
	gshr = common.BigIntToPoint(secp256k1.Curve.Add(&gs.X, &gs.Y, &hr.X, &hr.Y))
	return
}

func VerifyNIZKPK(c, u1, u2 big.Int, gs, gshr common.Point) bool {

	//compute t1prime
	t1prime := common.BigIntToPoint(secp256k1.Curve.ScalarBaseMult(u1.Bytes()))
	gsc := common.BigIntToPoint(secp256k1.Curve.ScalarMult(&gs.X, &gs.Y, c.Bytes()))
	t1prime = common.BigIntToPoint(secp256k1.Curve.Add(&t1prime.X, &t1prime.Y, &gsc.X, &gsc.Y))

	//compute t2
	t2prime := common.BigIntToPoint(secp256k1.Curve.ScalarMult(&secp256k1.H.X, &secp256k1.H.Y, u2.Bytes()))
	//computing gshr/gs^C
	neggsY := new(big.Int)
	neggsY.Set(&gs.Y)
	neggsY.Neg(neggsY)
	neggsY.Mod(neggsY, secp256k1.FieldOrder)
	gshrsubgs := common.BigIntToPoint(secp256k1.Curve.Add(&gshr.X, &gshr.Y, &gs.X, neggsY))
	gshrsubgsC := common.BigIntToPoint(secp256k1.Curve.ScalarMult(&gshrsubgs.X, &gshrsubgs.Y, c.Bytes()))
	t2prime = common.BigIntToPoint(secp256k1.Curve.Add(&t2prime.X, &t2prime.Y, &gshrsubgsC.X, &gshrsubgsC.Y)) // add them all up here

	bytesToBeHashed := append(pad(secp256k1.G.X.Bytes()), pad(secp256k1.H.X.Bytes())...)
	bytesToBeHashed = append(bytesToBeHashed, pad(gs.X.Bytes())...)
	bytesToBeHashed = append(bytesToBeHashed, pad(gshr.X.Bytes())...)
	bytesToBeHashed = append(bytesToBeHashed, pad(t1prime.X.Bytes())...)
	bytesToBeHashed = append(bytesToBeHashed, pad(t2prime.X.Bytes())...)

	cprime := new(big.Int)
	cprime.SetBytes(secp256k1.Keccak256(bytesToBeHashed))

	//compare c to RHS
	if cprime.Cmp(&c) == 0 {
		return true
	} else {
		return false
	}
}
