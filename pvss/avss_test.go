package pvss

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/torusresearch/torus-common/common"
	"github.com/torusresearch/torus-common/secp256k1"
	pcmn "github.com/torusresearch/torus-node/common"
)

func TestGenerateBivarPoly(t *testing.T) {
	threshold := 7
	secret := *RandomBigInt()
	bivarPoly := GenerateRandomBivariatePolynomial(secret, threshold)
	assert.Equal(t, len(bivarPoly), threshold)
	assert.Equal(t, len(bivarPoly[0]), threshold)
	assert.Equal(t, secret.Text(16), bivarPoly[0][0].Text(16))
}

func TestPedersonCommitmentMatrix(t *testing.T) {
	threshold := 7
	secret := *RandomBigInt()
	// f = summation of f_jl x^j y^l
	f := GenerateRandomBivariatePolynomial(secret, threshold)
	fprime := GenerateRandomBivariatePolynomial(*RandomBigInt(), threshold)
	C := GetCommitmentMatrix(f, fprime)
	gfjl := common.BigIntToPoint(secp256k1.Curve.ScalarBaseMult(f[1][2].Bytes()))
	hfprimejl := common.BigIntToPoint(secp256k1.Curve.ScalarMult(
		&secp256k1.H.X,
		&secp256k1.H.Y,
		fprime[1][2].Bytes(),
	))
	pt := common.BigIntToPoint(secp256k1.Curve.Add(&gfjl.X, &gfjl.Y, &hfprimejl.X, &hfprimejl.Y))
	assert.Equal(t, C[1][2].X, pt.X)
}

func TestBivarPolyEval(t *testing.T) {
	threshold := 7
	secret := *RandomBigInt()
	f := GenerateRandomBivariatePolynomial(secret, threshold)
	f5y := EvaluateBivarPolyAtX(f, *big.NewInt(int64(5)))
	fx3 := EvaluateBivarPolyAtY(f, *big.NewInt(int64(3)))
	f53_0 := polyEval(f5y, 3)
	f53_1 := polyEval(fx3, 5)
	assert.Equal(t, f53_0.Text(16), f53_1.Text(16))
}

func TestAVSSAddCommitment(t *testing.T) {
	C1 := make([][]common.Point, 1)
	C1[0] = make([]common.Point, 1)
	C2 := make([][]common.Point, 1)
	C2[0] = make([]common.Point, 1)
	testBytes := []byte{1}
	C1[0][0] = common.BigIntToPoint(secp256k1.Curve.ScalarBaseMult(testBytes))
	C2[0][0] = common.BigIntToPoint(secp256k1.Curve.ScalarBaseMult(testBytes))
	result, _ := AVSSAddCommitment(C1, C2)
	expectedResult := common.BigIntToPoint(secp256k1.Curve.Add(&C1[0][0].X, &C1[0][0].Y, &C1[0][0].X, &C1[0][0].Y))
	assert.Equal(t, result[0][0].X.Text(16), expectedResult.X.Text(16))
}

func TestAVSSVerifyPoly(t *testing.T) {
	// dealer chooses two bivar polys, f and fprime
	threshold := 7 // 7 out of 9 nodes
	secret := *RandomBigInt()
	f := GenerateRandomBivariatePolynomial(secret, threshold)
	fprime := GenerateRandomBivariatePolynomial(*RandomBigInt(), threshold)
	// commits to it using a matrix of commitments
	C := GetCommitmentMatrix(f, fprime)
	index := *big.NewInt(int64(5))
	AIY := EvaluateBivarPolyAtX(f, index)
	AIprimeY := EvaluateBivarPolyAtX(fprime, index)
	BIX := EvaluateBivarPolyAtY(f, index)
	BIprimeX := EvaluateBivarPolyAtY(fprime, index)

	assert.True(t, AVSSVerifyPoly(C, index, AIY, AIprimeY, BIX, BIprimeX))
}

func TestAVSSVerifyPoint(t *testing.T) {
	// dealer chooses two bivar polys, f and fprime
	threshold := 7 // 7 out of 9 nodes
	secret := *RandomBigInt()
	f := GenerateRandomBivariatePolynomial(secret, threshold)
	fprime := GenerateRandomBivariatePolynomial(*RandomBigInt(), threshold)
	// commits to it using a matrix of commitments
	C := GetCommitmentMatrix(f, fprime)
	m := big.NewInt(int64(3))
	i := big.NewInt(int64(5))
	alpha := polyEval(EvaluateBivarPolyAtX(f, *big.NewInt(int64(3))), 5)
	alphaprime := polyEval(EvaluateBivarPolyAtX(fprime, *big.NewInt(int64(3))), 5)
	beta := polyEval(EvaluateBivarPolyAtX(f, *big.NewInt(int64(5))), 3)
	betaprime := polyEval(EvaluateBivarPolyAtX(fprime, *big.NewInt(int64(5))), 3)
	assert.True(t, AVSSVerifyPoint(C, *m, *i, *alpha, *alphaprime, *beta, *betaprime))
}

func TestAVSSVerifyShare(t *testing.T) {
	// dealer chooses two bivar polys, f and fprime
	threshold := 7 // 7 out of 9 nodes
	secret := *RandomBigInt()
	f := GenerateRandomBivariatePolynomial(secret, threshold)
	fprime := GenerateRandomBivariatePolynomial(*RandomBigInt(), threshold)
	// commits to it using a matrix of commitments
	C := GetCommitmentMatrix(f, fprime)
	sigma := polyEval(EvaluateBivarPolyAtX(f, *big.NewInt(int64(3))), 0)
	sigmaprime := polyEval(EvaluateBivarPolyAtX(fprime, *big.NewInt(int64(3))), 0)
	assert.True(t, AVSSVerifyShare(C, *big.NewInt(int64(3)), *sigma, *sigmaprime))
}

func TestAVSS(t *testing.T) {
	// dealer chooses two bivar polys, f and fprime
	total := 9
	threshold := 7 // 7 out of 9 nodes
	secret := *RandomBigInt()
	f := GenerateRandomBivariatePolynomial(secret, threshold)
	fprime := GenerateRandomBivariatePolynomial(*RandomBigInt(), threshold)
	// commits to it using a matrix of commitments
	C := GetCommitmentMatrix(f, fprime)
	type Echo struct {
		M        big.Int
		Index    big.Int
		C        [][]common.Point
		Aij      big.Int
		Aprimeij big.Int
		Bij      big.Int
		Bprimeij big.Int
	}
	type Ready struct {
		M        big.Int
		Index    big.Int
		C        [][]common.Point
		Aij      big.Int
		Aprimeij big.Int
		Bij      big.Int
		Bprimeij big.Int
	}
	type Node struct {
		Index            big.Int
		CommitmentMatrix [][]common.Point
		AIY              pcmn.PrimaryPolynomial
		AIprimeY         pcmn.PrimaryPolynomial
		BIX              pcmn.PrimaryPolynomial
		BIprimeX         pcmn.PrimaryPolynomial
		ReceivedEchoes   []Echo
		ReceivedReadys   []Ready
	}
	nodes := make([]Node, total)

	// dealer sends commitments, share polys, and sub-share polys to nodes
	for n := range nodes {
		index := *big.NewInt(int64(n + 1))
		nodes[n] = Node{
			Index:            index,
			CommitmentMatrix: C,
			AIY:              EvaluateBivarPolyAtX(f, index),
			AIprimeY:         EvaluateBivarPolyAtX(fprime, index),
			BIX:              EvaluateBivarPolyAtY(f, index),
			BIprimeX:         EvaluateBivarPolyAtY(fprime, index),
			ReceivedEchoes:   make([]Echo, total),
			ReceivedReadys:   make([]Ready, threshold),
		}
	}

	// verify poly from dealer
	for n := range nodes {
		node := nodes[n]
		assert.True(t, AVSSVerifyPoly(node.CommitmentMatrix, node.Index, node.AIY, node.AIprimeY, node.BIX, node.BIprimeX))
	}

	// nodes echo commitment matrix and point overlaps to other nodes
	for i := range nodes {
		for j := range nodes {
			m := i + 1
			index := j + 1
			nodes[j].ReceivedEchoes[i] = Echo{
				M:        *big.NewInt(int64(m)),     // sender index
				Index:    *big.NewInt(int64(index)), // receiver index
				C:        C,
				Aij:      *polyEval(nodes[i].AIY, index),
				Aprimeij: *polyEval(nodes[i].AIprimeY, index),
				Bij:      *polyEval(nodes[i].BIX, index),
				Bprimeij: *polyEval(nodes[i].BIprimeX, index),
			}
		}
	}

	// verify points from other nodes echoes
	for n := range nodes {
		for e := range nodes[n].ReceivedEchoes {
			echo := nodes[n].ReceivedEchoes[e]
			// note: potentially some may fail this check
			// if they fail, exclude them from the next lagrange interpolation step.
			assert.True(t, AVSSVerifyPoint(echo.C, echo.M, echo.Index, echo.Aij, echo.Aprimeij, echo.Bij, echo.Bprimeij))
		}
	}

	// interpolate own share poly and subshare poly from echoes
	// and verify that the dealer dealt correct shares
	for n := range nodes {
		sharesAbari := make([]pcmn.PrimaryShare, threshold)
		sharesAprimebari := make([]pcmn.PrimaryShare, threshold)
		sharesBbari := make([]pcmn.PrimaryShare, threshold)
		sharesBprimebari := make([]pcmn.PrimaryShare, threshold)
		for e := 0; e < threshold; e++ { // assume that the first 7 (threshold) are the ones we use
			echo := nodes[n].ReceivedEchoes[e]
			sharesAbari[e] = pcmn.PrimaryShare{Index: int(echo.M.Int64()), Value: echo.Aij}
			sharesAprimebari[e] = pcmn.PrimaryShare{Index: int(echo.M.Int64()), Value: echo.Aprimeij}
			sharesBbari[e] = pcmn.PrimaryShare{Index: int(echo.M.Int64()), Value: echo.Bij}
			sharesBprimebari[e] = pcmn.PrimaryShare{Index: int(echo.M.Int64()), Value: echo.Bprimeij}
		}
		// "interpolate" shares here... but we really don't need to, because an honest server will have provided
		// the correct polys, and if the honest server didn't we would be broadcasting a ready message with wrong
		// shares, which would not be accepted by any honest server anyway...
		// all we need to do is check if the polys we received from the server really do evaluate to the points
		// that we have received via the echoes

		// Notation is a little confusing here, please refer to the paper again carefully when implementing elsewhere
		for i := range sharesAbari {
			assert.Equal(t, polyEval(nodes[n].BIX, sharesAbari[i].Index).Text(16), sharesAbari[i].Value.Text(16))
		}
		for i := range sharesAprimebari {
			assert.Equal(t, polyEval(nodes[n].BIprimeX, sharesAprimebari[i].Index).Text(16), sharesAprimebari[i].Value.Text(16))
		}
		for i := range sharesBbari {
			assert.Equal(t, polyEval(nodes[n].AIY, sharesBbari[i].Index).Text(16), sharesBbari[i].Value.Text(16))
		}
		for i := range sharesBprimebari {
			assert.Equal(t, polyEval(nodes[n].AIprimeY, sharesBprimebari[i].Index).Text(16), sharesBprimebari[i].Value.Text(16))
		}
	}

	shares := make([]pcmn.PrimaryShare, total)
	shareprimes := make([]pcmn.PrimaryShare, total)
	for i := range shares {
		shares[i] = pcmn.PrimaryShare{Index: int(nodes[i].Index.Int64()), Value: nodes[i].AIY.Coeff[0]}
		shareprimes[i] = pcmn.PrimaryShare{Index: int(nodes[i].Index.Int64()), Value: nodes[i].AIprimeY.Coeff[0]}
		assert.True(t, AVSSVerifyShare(C, *big.NewInt(int64(shares[i].Index)), shares[i].Value, shareprimes[i].Value))
	}

	assert.Equal(t, LagrangeScalar(shares[:7], 0).Text(16), LagrangeScalar(shares[1:8], 0).Text(16))
	assert.Equal(t, LagrangeScalar(shareprimes[:7], 0).Text(16), LagrangeScalar(shareprimes[1:8], 0).Text(16))
}
