package pvss

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/YZhenY/torus/common"
	"github.com/YZhenY/torus/secp256k1"
	"github.com/stretchr/testify/assert"
)

func TestGenPolyForTarget(t *testing.T) {
	target := big.NewInt(int64(100))
	poly := genPolyForTarget(*target, 6)
	assert.Equal(t, polyEval(*poly, *target).Int64(), int64(0))
}

func TestPSS(t *testing.T) {
	// set up share distribution
	n := 12
	threshold := 9
	nodeList, _ := createRandomNodes(n)

	// generate random additive shares
	zList := new([12]big.Int)
	for i, _ := range zList {
		zList[i] = *RandomBigInt()
	}

	// distribute polynomial shares, omitting pubpoly check
	// all shares for node 1 are found at sharesStore[0][0] sharesStore[0][1] ...
	subsharesStore := new([12][12]common.PrimaryShare)
	for i, z := range zList {
		tempSharesStore, _, err := CreateShares(nodeList.Nodes, z, threshold)
		if err != nil {
			fmt.Println("Error occurred when creating shares")
		}
		for j, tempShare := range *tempSharesStore {
			if j >= len(subsharesStore[i]) {
				fmt.Println("Exceeded array length, skipping")
				continue
			}
			subsharesStore[j][i] = tempShare
		}
	}

	// sum up subshares to get shares
	sharesStore := new([12]common.PrimaryShare)
	for i, shares := range subsharesStore {
		sum := new(big.Int)
		for _, share := range shares {
			sum.Add(sum, &share.Value)
		}
		sharesStore[i] = common.PrimaryShare{Index: *big.NewInt(int64(i + 1)), Value: *sum}
	}

	// check that lagrange interpolation of different threshold sets of shares work
	reconstructedSecret1 := new(big.Int).Mod(LagrangeScalar(sharesStore[:9]), secp256k1.GeneratorOrder)
	reconstructedSecret2 := new(big.Int).Mod(LagrangeScalar(sharesStore[1:10]), secp256k1.GeneratorOrder)
	assert.Equal(t, reconstructedSecret1, reconstructedSecret2)

	// secret := reconstructedSecret1
	// fmt.Println(secret)

	// we now have a set of nodes with polynomial shares of a shared secret

	// proactive secret sharing to a set of new nodes
	// one := uint8(1)
	// fmt.Println(byte(one))
}
