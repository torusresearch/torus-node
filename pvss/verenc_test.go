package pvss

import (
	"testing"

	"github.com/YZhenY/torus/secp256k1"
	"github.com/stretchr/testify/assert"
)

func TestDLEQ(t *testing.T) {
	x := RandomBigInt()
	p, xG, xH := GenerateDLEQProof(secp256k1.G, secp256k1.H, *x)
	assert.True(t, p.Verify(secp256k1.G, secp256k1.H, xG, xH))
}
