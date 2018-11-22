package pvss

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenPolyForTarget(t *testing.T) {
	target := big.NewInt(int64(100))
	poly := genPolyForTarget(*target, 6)
	assert.Equal(t, polyEval(*poly, *target).Int64(), int64(0))
}
