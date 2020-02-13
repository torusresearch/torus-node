package pvss

// Secure Distributed Key Generation for Discrete-Log Based Cryptosystems

import (
	"math/big"

	"github.com/torusresearch/torus-common/common"
	"github.com/torusresearch/torus-common/secp256k1"
	pcmn "github.com/torusresearch/torus-node/common"
)

// Commit creates a public commitment polynomial for the h base point
func GetCommitH(polynomial pcmn.PrimaryPolynomial) []common.Point {
	commits := make([]common.Point, polynomial.Threshold)
	for i := range commits {
		commits[i] = common.BigIntToPoint(secp256k1.Curve.ScalarMult(&secp256k1.H.X, &secp256k1.H.Y, polynomial.Coeff[i].Bytes()))
	}
	return commits
}

// CreateSharesGen - Creating shares for gennaro DKG
func CreateSharesGen(nodes []pcmn.Node, secret big.Int, threshold int) (*[]pcmn.PrimaryShare, *[]pcmn.PrimaryShare, *[]common.Point, *[]common.Point, error) {
	// generate two polynomials, one for pederson commitments
	polynomial := *generateRandomZeroPolynomial(secret, threshold)
	polynomialPrime := *generateRandomZeroPolynomial(*RandomBigInt(), threshold)

	// determine shares for polynomial with respect to basis point
	shares := getShares(polynomial, nodes)
	sharesPrime := getShares(polynomialPrime, nodes)

	// committing to polynomial
	pubPoly := GetCommit(polynomial)
	pubPolyPrime := GetCommitH(polynomialPrime)

	// create Ci
	Ci := make([]common.Point, threshold)
	for i := range pubPoly {
		Ci[i] = common.BigIntToPoint(secp256k1.Curve.Add(&pubPoly[i].X, &pubPoly[i].Y, &pubPolyPrime[i].X, &pubPolyPrime[i].Y))
	}

	return &shares, &sharesPrime, &pubPoly, &Ci, nil
}

// Verify Pederson commitment, Equation (4) in Gennaro 2006
func VerifyPedersonCommitment(share pcmn.PrimaryShare, sharePrime pcmn.PrimaryShare, ci []common.Point, index big.Int) bool {

	// committing to polynomial
	gSik := common.BigIntToPoint(secp256k1.Curve.ScalarBaseMult(share.Value.Bytes()))
	hSikPrime := common.BigIntToPoint(secp256k1.Curve.ScalarMult(&secp256k1.H.X, &secp256k1.H.Y, sharePrime.Value.Bytes()))

	//computing LHS
	lhs := common.BigIntToPoint(secp256k1.Curve.Add(&gSik.X, &gSik.Y, &hSikPrime.X, &hSikPrime.Y))

	//computing RHS
	rhs := common.Point{X: *new(big.Int).SetInt64(0), Y: *new(big.Int).SetInt64(0)}
	for i := range ci {
		jt := new(big.Int).Set(&index)
		jt.Exp(jt, new(big.Int).SetInt64(int64(i)), secp256k1.GeneratorOrder)
		polyValue := common.BigIntToPoint(secp256k1.Curve.ScalarMult(&ci[i].X, &ci[i].Y, jt.Bytes()))
		rhs = common.BigIntToPoint(secp256k1.Curve.Add(&rhs.X, &rhs.Y, &polyValue.X, &polyValue.Y))
	}

	return lhs.X.Cmp(&rhs.X) == 0
}

// VerifyShare - verifies share against public polynomial
func VerifyShare(share pcmn.PrimaryShare, pubPoly []common.Point, index big.Int) bool {

	lhs := common.BigIntToPoint(secp256k1.Curve.ScalarBaseMult(share.Value.Bytes()))

	// computing RHS
	rhs := common.Point{X: *new(big.Int).SetInt64(0), Y: *new(big.Int).SetInt64(0)}
	for i := range pubPoly {
		jt := new(big.Int).Set(&index)
		jt.Exp(jt, new(big.Int).SetInt64(int64(i)), secp256k1.GeneratorOrder)
		polyValue := common.BigIntToPoint(secp256k1.Curve.ScalarMult(&pubPoly[i].X, &pubPoly[i].Y, jt.Bytes()))
		rhs = common.BigIntToPoint(secp256k1.Curve.Add(&rhs.X, &rhs.Y, &polyValue.X, &polyValue.Y))
	}

	if lhs.X.Cmp(&rhs.X) == 0 {
		return true
	} else {
		return false
	}
}

// VerifyShareCommitment - checks if a dlog commitment matches the original pubpoly
func VerifyShareCommitment(shareCommitment common.Point, pubPoly []common.Point, index big.Int) bool {
	lhs := shareCommitment

	// computing RHS
	rhs := common.Point{X: *new(big.Int).SetInt64(0), Y: *new(big.Int).SetInt64(0)}
	for i := range pubPoly {
		jt := new(big.Int).Set(&index)
		jt.Exp(jt, new(big.Int).SetInt64(int64(i)), secp256k1.GeneratorOrder)
		polyValue := common.BigIntToPoint(secp256k1.Curve.ScalarMult(&pubPoly[i].X, &pubPoly[i].Y, jt.Bytes()))
		rhs = common.BigIntToPoint(secp256k1.Curve.Add(&rhs.X, &rhs.Y, &polyValue.X, &polyValue.Y))
	}

	if lhs.X.Cmp(&rhs.X) == 0 {
		return true
	} else {
		return false
	}
}

func RHS(shareCommitment common.Point, pubPoly []common.Point, index big.Int) common.Point {
	rhs := common.Point{X: *new(big.Int).SetInt64(0), Y: *new(big.Int).SetInt64(0)}
	for i := range pubPoly {
		jt := new(big.Int).Set(&index)
		jt.Exp(jt, new(big.Int).SetInt64(int64(i)), secp256k1.GeneratorOrder)
		polyValue := common.BigIntToPoint(secp256k1.Curve.ScalarMult(&pubPoly[i].X, &pubPoly[i].Y, jt.Bytes()))
		rhs = common.BigIntToPoint(secp256k1.Curve.Add(&rhs.X, &rhs.Y, &polyValue.X, &polyValue.Y))
	}
	return rhs
}
