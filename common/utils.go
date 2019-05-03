package common

import (
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

func BigIntToPoint(x, y *big.Int) Point {
	return Point{X: *x, Y: *y}
}

func HexToBigInt(s string) *big.Int {
	r, ok := new(big.Int).SetString(s, 16)
	if !ok {
		panic("invalid hex in source file: " + s)
	}
	return r
}

//converts common point to ETH address format borrowint ethcrypto functions
func PointToEthAddress(point Point) (*common.Address, error) {
	addr := crypto.PubkeyToAddress(ecdsa.PublicKey{X: &point.X, Y: &point.Y})
	return &addr, nil
}

func TimeTrack(start time.Time, name string) {
	elapsed := time.Since(start)
	fmt.Println("%s took %s", name, elapsed)
}
