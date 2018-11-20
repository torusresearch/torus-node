package pvss

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDLEQ(t *testing.T) {
	x := RandomBigInt()
	p, xG, xH := GenerateDLEQProof(G, H, *x)
	assert.True(t, p.Verify(G, H, xG, xH))
}
