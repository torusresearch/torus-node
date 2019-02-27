package pvss

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/torusresearch/torus-public/common"
	"github.com/torusresearch/torus-public/secp256k1"
)

func TestPedersonCommitment(t *testing.T) {
	nodeList, _ := createRandomNodes(21)
	secrets := make([]big.Int, len(nodeList.Nodes))
	allShares := make([][]common.PrimaryShare, len(nodeList.Nodes))
	allSharesPrime := make([][]common.PrimaryShare, len(nodeList.Nodes))
	allPubPoly := make([][]common.Point, len(nodeList.Nodes))
	allCi := make([][]common.Point, len(nodeList.Nodes))
	for i := range nodeList.Nodes {
		shares, sharePrimes, pubPoly, ci, err := CreateSharesGen(nodeList.Nodes, secrets[i], 11)
		allShares[i] = *shares
		allSharesPrime[i] = *sharePrimes
		allPubPoly[i] = *pubPoly
		allCi[i] = *ci
		if err != nil {
			fmt.Println(err)
		}
	}

	for j := range nodeList.Nodes {
		for i := range nodeList.Nodes {
			index := new(big.Int).SetInt64(int64(allShares[i][j].Index))
			correct, _ := VerifyPedersonCommitment(allShares[i][j], allSharesPrime[i][j], allCi[i], *index)
			assert.True(t, correct, fmt.Sprintf("Not correct for node %d from %d (index %d)", j, i, index))
		}
	}

}

func TestGennaroDKG(t *testing.T) {
	nodeList, _ := createRandomNodes(21)
	secrets := make([]big.Int, len(nodeList.Nodes))
	allShares := make([][]common.PrimaryShare, len(nodeList.Nodes))
	allSharesPrime := make([][]common.PrimaryShare, len(nodeList.Nodes))
	allPubPoly := make([][]common.Point, len(nodeList.Nodes))
	allCi := make([][]common.Point, len(nodeList.Nodes))
	for i := range nodeList.Nodes {
		shares, sharePrimes, pubPoly, ci, err := CreateSharesGen(nodeList.Nodes, secrets[i], 11)
		allShares[i] = *shares
		allSharesPrime[i] = *sharePrimes
		allPubPoly[i] = *pubPoly
		allCi[i] = *ci
		if err != nil {
			fmt.Println(err)
		}
	}

	//verify pederson commitments
	for j := range nodeList.Nodes {
		for i := range nodeList.Nodes {
			index := new(big.Int).SetInt64(int64(allShares[i][j].Index))
			correct, _ := VerifyPedersonCommitment(allShares[i][j], allSharesPrime[i][j], allCi[i], *index)
			assert.True(t, correct, fmt.Sprintf("Pederson commitment not correct for node %d from %d (index %d)", j, i, index))
		}
	}

	//complain and create valid qualifying set here

	//here we broadcast pub polys for qualifying set, verify summed up share against pub poly
	// or equation (5) in gennaro
	for j := range nodeList.Nodes {
		for i := range nodeList.Nodes {
			index := new(big.Int).SetInt64(int64(allShares[i][j].Index))
			correct, _ := VerifyShare(allShares[i][j], allPubPoly[i], *index)
			assert.True(t, correct, fmt.Sprintf("public poly not correct for node %d from %d (index %d)", j, i, index))
		}
	}

	// we complain against nodes who do not fufill (5), we then do reconstruction of their share
	//form si, points on the polynomial f(z) = r + a1z + a2z^2....
	allSi := make([]common.PrimaryShare, len(nodeList.Nodes))
	for j := range nodeList.Nodes {
		sum := new(big.Int)
		for i := range nodeList.Nodes {
			sum.Add(sum, &allShares[i][j].Value)
		}
		sum.Mod(sum, secp256k1.GeneratorOrder)
		allSi[j] = common.PrimaryShare{j + 1, *sum}
	}

	//form r (and other components) to test lagrange
	r := new(big.Int)
	for i := range nodeList.Nodes {
		r.Add(r, &secrets[i])
	}
	r.Mod(r, secp256k1.GeneratorOrder)

	testr := LagrangeScalar(allSi[:11], 0)

	assert.True(t, testr.Cmp(r) == 0)
}
