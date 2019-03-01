package pvss

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/torusresearch/torus-public/common"
	"github.com/torusresearch/torus-public/secp256k1"
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
	type Node struct {
		Index            big.Int
		CommitmentMatrix [][]common.Point
		AIY              common.PrimaryPolynomial
		AIprimeY         common.PrimaryPolynomial
		BIX              common.PrimaryPolynomial
		BIprimeX         common.PrimaryPolynomial
		ReceivedEchoes   []Echo
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
				Aij:      *polyEval(nodes[i].AIY, j),
				Aprimeij: *polyEval(nodes[i].AIprimeY, j),
				Bij:      *polyEval(nodes[i].BIX, j),
				Bprimeij: *polyEval(nodes[i].BIprimeX, j),
			}
		}
	}

	// verify points from other nodes
	for n := range nodes {
		for e := range nodes[n].ReceivedEchoes {
			echo := nodes[n].ReceivedEchoes[e]
			assert.True(t, AVSSVerifyPoint(echo.C, echo.M, echo.Index, echo.Aij, echo.Aprimeij, echo.Bij, echo.Bprimeij))
		}
	}

	assert.True(t, false)

	return
}
