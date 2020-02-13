package pvss

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/torusresearch/torus-common/common"
	"github.com/torusresearch/torus-common/secp256k1"
	pcmn "github.com/torusresearch/torus-node/common"
)

func TestPedersonCommitment(t *testing.T) {
	total := 21
	threshold := 15
	nodeList, _ := createRandomNodes(total)
	secrets := make([]big.Int, len(nodeList.Nodes))
	allShares := make([][]pcmn.PrimaryShare, len(nodeList.Nodes))
	allSharesPrime := make([][]pcmn.PrimaryShare, len(nodeList.Nodes))
	allPubPoly := make([][]common.Point, len(nodeList.Nodes))
	allCi := make([][]common.Point, len(nodeList.Nodes))
	for i := range nodeList.Nodes {
		shares, sharePrimes, pubPoly, ci, err := CreateSharesGen(nodeList.Nodes, secrets[i], threshold)
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
			correct := VerifyPedersonCommitment(allShares[i][j], allSharesPrime[i][j], allCi[i], *index)
			assert.True(t, correct, fmt.Sprintf("Not correct for node %d from %d (index %d)", j, i, index))
		}
	}

}

func TestGennaroDKG(t *testing.T) {
	total := 21
	threshold := 15
	nodeList, _ := createRandomNodes(total)
	secrets := make([]big.Int, total)
	for i := range secrets {
		secrets[i] = *RandomBigInt()
	}
	allShares := make([][]pcmn.PrimaryShare, total)
	allSharesPrime := make([][]pcmn.PrimaryShare, total)
	allPubPoly := make([][]common.Point, total)
	allCi := make([][]common.Point, total)
	for i := range nodeList.Nodes {
		shares, sharePrimes, pubPoly, ci, err := CreateSharesGen(nodeList.Nodes, secrets[i], threshold)
		allShares[i] = *shares
		allSharesPrime[i] = *sharePrimes
		allPubPoly[i] = *pubPoly
		allCi[i] = *ci
		if err != nil {
			fmt.Println(err)
		}
	}

	// verify pederson commitments
	for j := range nodeList.Nodes {
		for i := range nodeList.Nodes {
			index := new(big.Int).SetInt64(int64(allShares[i][j].Index))
			correct := VerifyPedersonCommitment(allShares[i][j], allSharesPrime[i][j], allCi[i], *index)
			assert.True(t, correct, fmt.Sprintf("Pederson commitment not correct for node %d from %d (index %d)", j, i, index))
		}
	}

	// complain and create valid qualifying set here

	// here we broadcast pub polys for qualifying set, verify summed up share against pub poly
	// or equation (5) in gennaro
	for j := range nodeList.Nodes {
		for i := range nodeList.Nodes {
			index := new(big.Int).SetInt64(int64(allShares[i][j].Index))
			correct := VerifyShare(allShares[i][j], allPubPoly[i], *index)
			assert.True(t, correct, fmt.Sprintf("public poly not correct for node %d from %d (index %d)", j, i, index))
		}
	}

	// we complain against nodes who do not fufill (5), we then do reconstruction of their share
	// from si, points on the polynomial f(z) = r + a1z + a2z^2....
	allSi := make([]pcmn.PrimaryShare, total)
	for j := range nodeList.Nodes {
		sum := new(big.Int)
		for i := range nodeList.Nodes {
			sum.Add(sum, &allShares[i][j].Value)
		}
		sum.Mod(sum, secp256k1.GeneratorOrder)
		allSi[j] = pcmn.PrimaryShare{Index: j + 1, Value: *sum}
	}

	// form r (and other components) to test lagrange
	r := new(big.Int)
	for i := range nodeList.Nodes {
		r.Add(r, &secrets[i])
	}
	r.Mod(r, secp256k1.GeneratorOrder)

	testr := LagrangeScalar(allSi[:threshold], 0)
	testr2 := LagrangeScalar(allSi[1:threshold+1], 0)
	assert.True(t, testr.Cmp(r) == 0)
	assert.True(t, testr2.Cmp(r) == 0)
}
