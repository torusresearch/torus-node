package bft

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/crypto/sha3"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
)

func TestFetchAndModifyEpoch(t *testing.T) {
	epoch, _ := Epoch()
	assert.Equal(t, 0, epoch)
	SetEpoch(1)
	epoch, _ = Epoch()
	assert.Equal(t, 1, epoch)
	SetEpoch(0)
}

func TestBroadcast(t *testing.T) {
	res, _ := Broadcast([]byte("message"))
	fmt.Println(res.LastInsertId())
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
	// fmt.Println(arrBytes)
	fmt.Println(hex.EncodeToString(hash.Sum([]byte{})))
}
