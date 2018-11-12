package bft

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFetchEpochFromJson(t *testing.T) {
	assert.Equal(t, 0, Epoch())
	SetEpoch(1)
	assert.Equal(t, 1, Epoch())
	SetEpoch(0)
}
