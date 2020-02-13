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

func TestGenPolyForTarget(t *testing.T) {
	target := 100
	poly := genPolyForTarget(target, 6)
	assert.Equal(t, polyEval(*poly, target).Int64(), int64(0))
}

func TestHerzbergPSS(t *testing.T) {
	// set up share distribution
	n := 12
	threshold := 9
	nodeList, _ := createRandomNodes(n)

	// generate random additive shares
	zList := new([12]big.Int)
	for i := range zList {
		zList[i] = *RandomBigInt()
	}

	// distribute polynomial shares, omitting pubpoly check
	// all shares for node 1 are found at sharesStore[0][0] sharesStore[0][1] ...
	subsharesStore := new([12][12]pcmn.PrimaryShare)
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
	sharesStore := new([12]pcmn.PrimaryShare)
	for i, shares := range subsharesStore {
		sum := new(big.Int)
		for _, share := range shares {
			sum.Add(sum, &share.Value)
		}
		sharesStore[i] = pcmn.PrimaryShare{Index: i + 1, Value: *sum}
	}

	// check that lagrange interpolation of different threshold sets of shares work
	secret := new(big.Int).Mod(LagrangeScalar(sharesStore[:9], 0), secp256k1.GeneratorOrder)
	secret2 := new(big.Int).Mod(LagrangeScalar(sharesStore[1:10], 0), secp256k1.GeneratorOrder)
	assert.Equal(t, secret, secret2)

	// generate newNodes, assume new nodes start from Index 13
	newNodeList, _ := createRandomNodes(n)
	for i := range newNodeList.Nodes {
		// update new nodes with higher indexes
		newNodeList.Nodes[i].Index = newNodeList.Nodes[i].Index + n
	}

	// generate Qi poly for each old node
	QiPolys := new([12]*pcmn.PrimaryPolynomial)
	for i := range QiPolys {
		QiPolys[i] = generateRandomZeroPolynomial(*big.NewInt(0), threshold)
	}

	// for each old node (i), generate Rik poly for each new node (k)
	RikPolys := new([12][12]*pcmn.PrimaryPolynomial)
	for i := range nodeList.Nodes {
		for j, newNode := range newNodeList.Nodes {
			RikPolys[i][j] = genPolyForTarget(newNode.Index, threshold)
		}
	}

	// check that Riks have been generated correctly

	assert.Equal(t, int64(0), polyEval(*RikPolys[3][7], 1+7+n).Int64())
	assert.Equal(t, int64(0), polyEval(*RikPolys[3][1], 1+1+n).Int64())
	assert.Equal(t, int64(0), polyEval(*RikPolys[2][1], 1+1+n).Int64())

	// publish public polynomials

	// RikPubPolys := new([12][12][12]common.Point)

	// encrypt shares
	// verify encryption

	// Combine Qi + Rik polys, and evaluate polys at old node index
	// QiRikPolys[1][2] is Q2 + R2,15
	QiRikPolys := new([12][12]*pcmn.PrimaryPolynomial)
	for i := range QiRikPolys {
		for j := range QiRikPolys[i] {
			QiRikPolys[i][j] = AddPolynomials(*QiPolys[i], *RikPolys[i][j])
		}
	}

	// check that QiRikPolys have been generated correctly
	assert.Equal(t, polyEval(*QiRikPolys[2][7], 1+7+n).Int64(), polyEval(*QiPolys[2], 1+7+n).Int64())
	assert.Equal(t, polyEval(*QiRikPolys[2][8], 1+8+n).Int64(), polyEval(*QiPolys[2], 1+8+n).Int64())
	assert.Equal(t, polyEval(*QiRikPolys[3][8], 1+8+n).Int64(), polyEval(*QiPolys[3], 1+8+n).Int64())

	// for each old node i, evaluate QiRikPolys for other old nodes j
	// QiRikPoints[1][2][3] is Q2 + R2,15 evaluated for old node at index 4
	QiRikPoints := new([12][12][12]big.Int)

	for i := range QiRikPolys {
		for j := range QiRikPolys[i] {
			// QiRikPolys[0][1] is the Q1 + R1,14 polynomial
			for k, oldNode := range nodeList.Nodes {
				QiRikPoints[i][j][k] = *polyEval(*QiRikPolys[i][j], oldNode.Index)
			}
		}
	}

	// sum up all the shares shared between the old nodes
	// QRkPoints[1][2] is Q(2) + R15(2) + P(2)
	QRkPoints := new([12][12]big.Int)

	for i := range QiRikPoints {
		for j := range QiRikPoints[i] {
			sum := new(big.Int)
			for k := range QiRikPoints[i][j] {
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
	newNodesReceivedShares := new([12][12]pcmn.PrimaryShare)

	for i := range QRkPoints {
		for j := range QRkPoints[i] {
			newNodesReceivedShares[j][i] = pcmn.PrimaryShare{Index: i + 1, Value: QRkPoints[i][j]}
		}
	}

	// check that new nodes shares were generated correctly
	newSecret := LagrangeScalar(newNodesReceivedShares[0][:9], 13)
	newSecret2 := LagrangeScalar(newNodesReceivedShares[0][1:10], 13)
	assert.Equal(t, newSecret.Int64(), newSecret2.Int64())

	newShares := new([12]pcmn.PrimaryShare)
	for i := range newShares {
		newShares[i] = pcmn.PrimaryShare{Index: 13 + i, Value: *LagrangeScalar(newNodesReceivedShares[i][:9], 13+i)}
	}
	newFinalSecret := LagrangeScalar(newShares[:9], 0)
	newFinalSecret2 := LagrangeScalar(newShares[1:10], 0)
	assert.Equal(t, newFinalSecret.Int64(), newFinalSecret2.Int64())

}

func TestJajodiaPSS(t *testing.T) {
	// Do Gennaro DKG first
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
	allSi := make([]pcmn.PrimaryShare, len(nodeList.Nodes))
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

	// Share resharing (Jajodia, implemented using Gennaro2006 New-DKG)
	// 1. create subshares from shares
	// 2. each receiving node lagrange interpolates the subshares he receives
	originalTotalPubPoly := make([]common.Point, total)
	for i := range allPubPoly {
		pubPoly := allPubPoly[i]
		for j := range pubPoly {
			originalTotalPubPoly[j] = common.BigIntToPoint(secp256k1.Curve.Add(&originalTotalPubPoly[j].X, &originalTotalPubPoly[j].Y, &pubPoly[j].X, &pubPoly[j].Y))
		}
	}
	newTotal := 15
	newThreshold := 11
	tempNodes, _ := createRandomNodes(newTotal)
	allScalarShares := make([]big.Int, total)
	for i := range allSi {
		allScalarShares[i] = allSi[i].Value
	}
	type ReceiverNode struct {
		Index                  int
		FinalShare             pcmn.PrimaryShare
		ReceivedSubShares      []pcmn.PrimaryShare
		ReceivedSubSharesPrime []pcmn.PrimaryShare
	}
	receiverNodes := make([]ReceiverNode, newTotal)
	for i := range tempNodes.Nodes {
		receiverNodes[i] = ReceiverNode{
			Index:                  tempNodes.Nodes[i].Index,
			ReceivedSubShares:      make([]pcmn.PrimaryShare, total),
			ReceivedSubSharesPrime: make([]pcmn.PrimaryShare, total),
		}
	}
	allPSSPubPoly := make([]*[]common.Point, total)
	allPSSCi := make([]*[]common.Point, total)
	for i := 0; i < total; i++ {
		shares, sharesPrime, pubPoly, Ci, _ := CreateSharesGen(tempNodes.Nodes, allScalarShares[i], newThreshold)
		allPSSPubPoly[i] = pubPoly
		allPSSCi[i] = Ci
		for j := range *shares {
			receiverNodes[j].ReceivedSubShares[i] = pcmn.PrimaryShare{
				Index: i + 1, // index here should be the sender node's index
				Value: (*shares)[j].Value,
			}
			receiverNodes[j].ReceivedSubSharesPrime[i] = pcmn.PrimaryShare{
				Index: i + 1, // index here should be the sender node's index
				Value: (*sharesPrime)[j].Value,
			}
		}
	}
	// verify subshares match commitments
	for i := range receiverNodes {
		receiverNode := receiverNodes[i]
		for j := 0; j < len(receiverNode.ReceivedSubShares); j++ {
			assert.True(t, VerifyPedersonCommitment(receiverNode.ReceivedSubShares[j], receiverNode.ReceivedSubSharesPrime[j], *allPSSCi[j], *big.NewInt(int64(receiverNode.Index))))
		}
	}
	// verify subshares are sharings of the original secret share
	for i := range allPSSPubPoly {
		PSSPubPolySecretDlogCommitment := (*allPSSPubPoly[i])[0]
		assert.True(t, VerifyShareCommitment(PSSPubPolySecretDlogCommitment, originalTotalPubPoly, *big.NewInt(int64(i + 1))))
	}

	// get final new shares from subshares
	newAllShares := make([]pcmn.PrimaryShare, newTotal)
	for i := range receiverNodes {
		receiverNode := receiverNodes[i]
		finalShare := LagrangeScalar(receiverNode.ReceivedSubShares[:threshold], 0)
		// NOTE: lagrange interpolations of different sets of recievedsubshares will be different
		// even though the final interpolation will yield the same secret
		receiverNode.FinalShare = pcmn.PrimaryShare{
			Index: receiverNode.Index,
			Value: *finalShare,
		}
		newAllShares[i] = receiverNode.FinalShare
	}
	assert.True(t, LagrangeScalar(newAllShares[:newThreshold], 0).Cmp(testr) == 0)
	assert.True(t, LagrangeScalar(newAllShares[1:newThreshold+1], 0).Cmp(testr) == 0)
}

func TestLagrangePoly(t *testing.T) {
	k := 5
	n := 9
	// generate shares
	origSecret := RandomBigInt()
	origCommitments := generateRandomZeroPolynomial(*origSecret, k)
	var shares []big.Int
	for i := 0; i < n; i++ {
		shares = append(shares, *polyEval(*origCommitments, i+1))
	}

	// generate subshares
	var subshareCommitments [][]common.Point
	var subshares [][]pcmn.PrimaryShare
	for i, share := range shares {
		var sShares []pcmn.PrimaryShare
		sharePoly := generateRandomZeroPolynomial(share, k)
		subshareCommitments = append(subshareCommitments, GetCommit(*sharePoly))
		for j := 0; j < n; j++ {
			sh := pcmn.PrimaryShare{
				Index: i + 1,
				Value: *polyEval(*sharePoly, j+1),
			}
			assert.True(t, VerifyShare(sh, GetCommit(*sharePoly), *big.NewInt(int64(j + 1))))
			sShares = append(sShares, sh)
		}
		subshares = append(subshares, sShares)
	}

	// check subshares
	var interpolatedShares []pcmn.PrimaryShare
	for i := 0; i < n; i++ {
		interpolatedShares = append(interpolatedShares, pcmn.PrimaryShare{
			Index: i + 1,
			Value: *LagrangeScalar(pcmn.GetColumnPrimaryShare(subshares[0:k], i), 0),
		})
	}
	reconstructedSecret := *LagrangeScalar(interpolatedShares[0:k], 0)
	assert.Equal(t, reconstructedSecret.Text(16), origSecret.Text(16))

	// interpolate new commitments
	newCommitments := LagrangePolys([]int{1, 2, 3, 4, 5}, subshareCommitments[0:k])
	if len(newCommitments) == 0 {
		t.Fatal("No commitments")
	}
	for _, interpolatedShare := range interpolatedShares {
		assert.True(t, VerifyShare(interpolatedShare, newCommitments, *big.NewInt(int64(interpolatedShare.Index))))
	}
}

func TestLagrangePolyForFirstTerm(t *testing.T) {
	k := 5
	n := 9
	// generate shares
	origSecret := RandomBigInt()
	origPoly := generateRandomZeroPolynomial(*origSecret, k)
	secretCommitment := common.BigIntToPoint(secp256k1.Curve.ScalarBaseMult(origSecret.Bytes()))
	var shares []big.Int
	for i := 0; i < n; i++ {
		shares = append(shares, *polyEval(*origPoly, i+1))
	}

	var shareCommitments []common.Point
	for _, share := range shares {
		shareCommitments = append(shareCommitments, common.BigIntToPoint(secp256k1.Curve.ScalarBaseMult(share.Bytes())))
	}
	var sm [][]common.Point
	for i := 0; i < 5; i++ {
		var temp []common.Point
		temp = append(temp, shareCommitments[i])
		sm = append(sm, temp)
	}
	res := LagrangePolys([]int{1, 2, 3, 4, 5}, sm)
	assert.Equal(t, res[0].X.Text(16), secretCommitment.X.Text(16))
}
