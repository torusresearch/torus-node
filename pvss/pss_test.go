package pvss

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/torusresearch/torus-public/common"
	"github.com/torusresearch/torus-public/secp256k1"
	"github.com/stretchr/testify/assert"
)

func TestGenPolyForTarget(t *testing.T) {
	target := 100
	poly := genPolyForTarget(target, 6)
	assert.Equal(t, polyEval(*poly, target).Int64(), int64(0))
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
		sharesStore[i] = common.PrimaryShare{Index: i + 1, Value: *sum}
	}

	// check that lagrange interpolation of different threshold sets of shares work
	secret := new(big.Int).Mod(LagrangeScalar(sharesStore[:9], 0), secp256k1.GeneratorOrder)
	secret2 := new(big.Int).Mod(LagrangeScalar(sharesStore[1:10], 0), secp256k1.GeneratorOrder)
	assert.Equal(t, secret, secret2)

	// generate newNodes, assume new nodes start from Index 13
	newNodeList, _ := createRandomNodes(n)
	for i, _ := range newNodeList.Nodes {
		// update new nodes with higher indexes
		newNodeList.Nodes[i].Index = newNodeList.Nodes[i].Index + n
	}

	// generate Qi poly for each old node
	QiPolys := new([12]*common.PrimaryPolynomial)
	for i, _ := range QiPolys {
		QiPolys[i] = generateRandomZeroPolynomial(*big.NewInt(0), threshold)
	}

	// for each old node (i), generate Rik poly for each new node (k)
	RikPolys := new([12][12]*common.PrimaryPolynomial)
	for i, _ := range nodeList.Nodes {
		for j, newNode := range newNodeList.Nodes {
			RikPolys[i][j] = genPolyForTarget(newNode.Index, threshold)
		}
	}

	// check that Riks have been generated correctly

	assert.Equal(t, int64(0), polyEval(*RikPolys[3][7], 1+7+n).Int64())
	assert.Equal(t, int64(0), polyEval(*RikPolys[3][1], 1+1+n).Int64())
	assert.Equal(t, int64(0), polyEval(*RikPolys[2][1], 1+1+n).Int64())

	// TODO: publish public polynomials
	// RikPubPolys := new([12][12][12]common.Point)

	// TODO: encrypt shares
	// TODO: verify encryption

	// Combine Qi + Rik polys, and evaluate polys at old node index
	// QiRikPolys[1][2] is Q2 + R2,15
	QiRikPolys := new([12][12]*common.PrimaryPolynomial)
	for i, _ := range QiRikPolys {
		for j, _ := range QiRikPolys[i] {
			QiRikPolys[i][j] = addPolynomials(*QiPolys[i], *RikPolys[i][j])
		}
	}

	// check that QiRikPolys have been generated correctly
	assert.Equal(t, polyEval(*QiRikPolys[2][7], 1+7+n).Int64(), polyEval(*QiPolys[2], 1+7+n).Int64())
	assert.Equal(t, polyEval(*QiRikPolys[2][8], 1+8+n).Int64(), polyEval(*QiPolys[2], 1+8+n).Int64())
	assert.Equal(t, polyEval(*QiRikPolys[3][8], 1+8+n).Int64(), polyEval(*QiPolys[3], 1+8+n).Int64())

	// for each old node i, evaluate QiRikPolys for other old nodes j
	// QiRikPoints[1][2][3] is Q2 + R2,15 evaluated for old node at index 4
	QiRikPoints := new([12][12][12]big.Int)

	for i, _ := range QiRikPolys {
		for j, _ := range QiRikPolys[i] {
			// QiRikPolys[0][1] is the Q1 + R1,14 polynomial
			for k, oldNode := range nodeList.Nodes {
				QiRikPoints[i][j][k] = *polyEval(*QiRikPolys[i][j], oldNode.Index)
			}
		}
	}

	// sum up all the shares shared between the old nodes
	// QRkPoints[1][2] is Q(2) + R15(2) + P(2)
	QRkPoints := new([12][12]big.Int)

	for i, _ := range QiRikPoints {
		for j, _ := range QiRikPoints[i] {
			sum := new(big.Int)
			for k, _ := range QiRikPoints[i][j] {
				// eg. sum of
				// + Q1(5) + R1,13(5)
				// + Q2(5) + R2,13(5)
				sum.Add(sum, &QiRikPoints[k][j][i])
			}
			sum.Mod(sum, secp256k1.GeneratorOrder)
			// eg.
			// Q(1) + R13(1) + P(1)
			QRkPoints[i][j] = *sum.Add(sum, &sharesStore[i].Value)
		}
	}

	// newNodesReceivedShares[1][2] is what new node 14 received from old node 3
	newNodesReceivedShares := new([12][12]common.PrimaryShare)

	for i, _ := range QRkPoints {
		for j, _ := range QRkPoints[i] {
			newNodesReceivedShares[j][i] = common.PrimaryShare{i + 1, QRkPoints[i][j]}
		}
	}

	// check that new nodes shares were generated correctly
	newSecret := LagrangeScalar(newNodesReceivedShares[0][:9], 13)
	newSecret2 := LagrangeScalar(newNodesReceivedShares[0][1:10], 13)
	assert.Equal(t, newSecret.Int64(), newSecret2.Int64())

	newShares := new([12]common.PrimaryShare)
	for i, _ := range newShares {
		newShares[i] = common.PrimaryShare{13 + i, *LagrangeScalar(newNodesReceivedShares[i][:9], 13+i)}
	}
	newFinalSecret := LagrangeScalar(newShares[:9], 0)
	newFinalSecret2 := LagrangeScalar(newShares[1:10], 0)
	assert.Equal(t, newFinalSecret.Int64(), newFinalSecret2.Int64())

}
