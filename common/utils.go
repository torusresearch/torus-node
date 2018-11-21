package common

import (
	"math/big"
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
