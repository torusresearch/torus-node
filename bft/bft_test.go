package bft

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/crypto/sha3"
	"github.com/stretchr/testify/assert"
)

func TestFetchEpochFromJson(t *testing.T) {
	assert.Equal(t, 0, Epoch())
	SetEpoch(1)
	assert.Equal(t, 1, Epoch())
	SetEpoch(0)
}

type Point struct {
	X int
	Y int
}

func TestHashStructArray(t *testing.T) {
	pointArray := make([]Point, 10, 10)
	for i := 0; i < 10; i++ {
		pointArray = append(pointArray, Point{X: i, Y: i})
	}
	arrBytes := []byte{}
	for _, item := range pointArray {
		var num []byte
		num = abi.U256(big.NewInt(int64(item.X)))
		arrBytes = append(arrBytes, num...)
		num = abi.U256(big.NewInt(int64(item.Y)))
		arrBytes = append(arrBytes, num...)
	}
	hash := sha3.NewKeccak256()
	hash.Write(arrBytes)
	fmt.Println(arrBytes)
	fmt.Println(hex.EncodeToString(hash.Sum([]byte{})))
}
