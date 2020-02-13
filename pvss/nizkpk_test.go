package pvss

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/torusresearch/torus-common/common"
	"github.com/torusresearch/torus-common/secp256k1"
)

func TestNIZKPK(t *testing.T) {
	s := *RandomBigInt()
	r := *RandomBigInt()
	gs := common.BigIntToPoint(secp256k1.Curve.ScalarBaseMult(s.Bytes()))
	hr := common.BigIntToPoint(secp256k1.Curve.ScalarMult(&secp256k1.H.X, &secp256k1.H.Y, r.Bytes()))
	gshr := common.BigIntToPoint(secp256k1.Curve.Add(&gs.X, &gs.Y, &hr.X, &hr.Y))
	c, u1, u2 := GenerateNIZKPK(s, r)

	verify := VerifyNIZKPK(c, u1, u2, gs, gshr)
	assert.True(t, verify, "verification should be true")
	falseS := *RandomBigInt()
	falseGs := common.BigIntToPoint(secp256k1.Curve.ScalarBaseMult(falseS.Bytes()))
	falseVerify := VerifyNIZKPK(c, u1, u2, falseGs, gshr)
	assert.False(t, falseVerify, "false verify should be false")

}
