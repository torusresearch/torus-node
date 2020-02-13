package pss

import (
	"math/big"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/torusresearch/bijson"

	"github.com/torusresearch/torus-common/common"
)

func TestTypeSerialization(t *testing.T) {
	p1 := PSSMsgRecover{
		SharingID: SharingID("testtypeserialization"),
		V: []common.Point{
			common.Point{
				X: *big.NewInt(int64(2)),
				Y: *big.NewInt(int64(4)),
			},
			common.Point{
				X: *big.NewInt(int64(22)),
				Y: *big.NewInt(int64(8)),
			},
		},
	}

	var p2 PSSMsgRecover
	byt, err := bijson.Marshal(p1)
	if err != nil {
		t.Fatal(err)
	}
	err = bijson.Unmarshal(byt, &p2)
	if err != nil {
		t.Fatal(err)
	}
	assert.True(t, reflect.DeepEqual(p1, p2))
}
