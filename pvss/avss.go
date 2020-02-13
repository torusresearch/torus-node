package pvss

// Asynchronous Verifiable Secret Sharing and Proactive Cryptosystems

import (
	"errors"
	"math/big"

	"github.com/torusresearch/torus-common/common"
	"github.com/torusresearch/torus-common/secp256k1"
	pcmn "github.com/torusresearch/torus-node/common"
)

// GenerateRandomBivariatePolynomial -
// create a bivariate polynomial which is defined by coeffs of
// all possible combinations of powers of x and y, such that powers
// of x and y are less than t (threshold). accepts param secret = f(0,0)
// Example coeff. matrix:
// f00,     f01.y,    f02.y^2
// f10.x,   f11.x.y,  f12.x.y^2
// f20.x^2, f21.x^2.y f22.x^2.y^2
func GenerateRandomBivariatePolynomial(secret big.Int, threshold int) [][]big.Int {
	bivarPolyCoeffs := make([][]big.Int, threshold)
	for j := range bivarPolyCoeffs {
		bivarPolyCoeffs[j] = make([]big.Int, threshold)
		for l := range bivarPolyCoeffs[j] {
			if j == 0 && l == 0 {
				bivarPolyCoeffs[j][l] = secret // f_00
			} else {
				bivarPolyCoeffs[j][l] = *RandomBigInt() // f_jl . x^j . y^l
			}
		}
	}

	return bivarPolyCoeffs

}

// GetCommitmentMatrix - get a pedersen commitment matrix from two bivar polys
func GetCommitmentMatrix(f [][]big.Int, fprime [][]big.Int) [][]common.Point {
	threshold := len(f)
	bivarPolyCommits := make([][]common.Point, threshold)
	for j := range bivarPolyCommits {
		bivarPolyCommits[j] = make([]common.Point, threshold)
		for l := range bivarPolyCommits[j] {
			gfjl := common.BigIntToPoint(secp256k1.Curve.ScalarBaseMult(f[j][l].Bytes()))
			hfprimejl := common.BigIntToPoint(secp256k1.Curve.ScalarMult(
				&secp256k1.H.X,
				&secp256k1.H.Y,
				fprime[j][l].Bytes(),
			))
			bivarPolyCommits[j][l] = common.BigIntToPoint(secp256k1.Curve.Add(&gfjl.X, &gfjl.Y, &hfprimejl.X, &hfprimejl.Y))
		}
	}
	return bivarPolyCommits
}

// EvaluateBivarPolyAtX - evaluate bivar poly at given x
func EvaluateBivarPolyAtX(bivarPoly [][]big.Int, index big.Int) pcmn.PrimaryPolynomial {
	fiY := pcmn.PrimaryPolynomial{
		Coeff:     make([]big.Int, len(bivarPoly)),
		Threshold: len(bivarPoly),
	}
	for l := range bivarPoly[0] { // loop through horizontally first
		for j := range bivarPoly { // loop through vertically
			fiY.Coeff[l] = *new(big.Int).Add(
				&fiY.Coeff[l],
				new(big.Int).Mul(
					&bivarPoly[j][l],
					new(big.Int).Exp(&index, big.NewInt(int64(j)), secp256k1.GeneratorOrder),
				),
			)
		}
	}
	return fiY
}

// EvaluateBivarPolyAtY - evaluate bivar poly at given y
func EvaluateBivarPolyAtY(bivarPoly [][]big.Int, index big.Int) pcmn.PrimaryPolynomial {
	fiX := pcmn.PrimaryPolynomial{
		Coeff:     make([]big.Int, len(bivarPoly)),
		Threshold: len(bivarPoly),
	}
	for j := range bivarPoly { // loop through vertically first
		for l := range bivarPoly[j] { // loop through horizontally
			fiX.Coeff[j] = *new(big.Int).Add(
				&fiX.Coeff[j],
				new(big.Int).Mul(
					&bivarPoly[j][l],
					new(big.Int).Exp(&index, big.NewInt(int64(l)), secp256k1.GeneratorOrder),
				),
			)
		}
	}
	return fiX
}

// AVSSVerifyPoly - verify-poly as described in Cachin et al 2002
func AVSSVerifyPoly(
	C [][]common.Point,
	index big.Int,
	a pcmn.PrimaryPolynomial,
	aprime pcmn.PrimaryPolynomial,
	b pcmn.PrimaryPolynomial,
	bprime pcmn.PrimaryPolynomial,
) bool {

	// check that polynomial thresholds are the same and Commitment Matrix Lentgh
	if a.Threshold != aprime.Threshold || a.Threshold != b.Threshold || a.Threshold != bprime.Threshold {
		return false
	}
	if a.Threshold != len(C) || a.Threshold != len(C[0]) {
		return false
	}

	// check that g^(a_l).h^(aprime_l) = product (C_jl)^(i^j)
	for l := 0; l < len(a.Coeff); l++ {
		gal := common.BigIntToPoint(secp256k1.Curve.ScalarBaseMult(a.Coeff[l].Bytes()))
		haprimel := common.BigIntToPoint(secp256k1.Curve.ScalarMult(&secp256k1.H.X, &secp256k1.H.Y, aprime.Coeff[l].Bytes()))
		galhaprimel := common.BigIntToPoint(secp256k1.Curve.Add(&gal.X, &gal.Y, &haprimel.X, &haprimel.Y))
		pt := common.Point{X: *big.NewInt(int64(0)), Y: *big.NewInt(int64(0))}
		for j := 0; j < a.Threshold; j++ {
			Cjl := C[j][l]
			Cjlij := common.BigIntToPoint(secp256k1.Curve.ScalarMult(&Cjl.X, &Cjl.Y, new(big.Int).Exp(&index, big.NewInt(int64(j)), secp256k1.GeneratorOrder).Bytes()))
			pt = common.BigIntToPoint(secp256k1.Curve.Add(&pt.X, &pt.Y, &Cjlij.X, &Cjlij.Y))
		}
		if galhaprimel.X.Cmp(&pt.X) != 0 || galhaprimel.Y.Cmp(&pt.Y) != 0 {
			return false
		}
	}

	// check that g^(b_j).h^(bprime_j) = product (C_jl)^(i^l)
	for j := 0; j < len(b.Coeff); j++ {
		gbj := common.BigIntToPoint(secp256k1.Curve.ScalarBaseMult(b.Coeff[j].Bytes()))
		hbprimej := common.BigIntToPoint(secp256k1.Curve.ScalarMult(&secp256k1.H.X, &secp256k1.H.Y, bprime.Coeff[j].Bytes()))
		gbjhbprimej := common.BigIntToPoint(secp256k1.Curve.Add(&gbj.X, &gbj.Y, &hbprimej.X, &hbprimej.Y))
		pt := common.Point{X: *big.NewInt(int64(0)), Y: *big.NewInt(int64(0))}
		for l := 0; l < b.Threshold; l++ {
			Cjl := C[j][l]
			Cjlil := common.BigIntToPoint(secp256k1.Curve.ScalarMult(&Cjl.X, &Cjl.Y, new(big.Int).Exp(&index, big.NewInt(int64(l)), secp256k1.GeneratorOrder).Bytes()))
			pt = common.BigIntToPoint(secp256k1.Curve.Add(&pt.X, &pt.Y, &Cjlil.X, &Cjlil.Y))
		}
		if gbjhbprimej.X.Cmp(&pt.X) != 0 || gbjhbprimej.Y.Cmp(&pt.Y) != 0 {
			return false
		}
	}

	return true
}

// AVSSVerifyPoint - verify-point as described in Cachin et al 2002
func AVSSVerifyPoint(
	C [][]common.Point,
	m big.Int, // the sender index
	index big.Int, // the receiver index
	alpha big.Int,
	alphaprime big.Int,
	beta big.Int,
	betaprime big.Int,
) bool {
	galpha := common.BigIntToPoint(secp256k1.Curve.ScalarBaseMult(alpha.Bytes()))
	halphaprime := common.BigIntToPoint(secp256k1.Curve.ScalarMult(&secp256k1.H.X, &secp256k1.H.Y, alphaprime.Bytes()))
	galphahalphaprime := common.BigIntToPoint(secp256k1.Curve.Add(&galpha.X, &galpha.Y, &halphaprime.X, &halphaprime.Y))
	pt := common.Point{X: *big.NewInt(int64(0)), Y: *big.NewInt(int64(0))}
	for j := range C { // loop through vertically first
		for l := range C[0] { // loop through horizontally
			Cjl := C[j][l]
			mj := new(big.Int).Exp(&m, big.NewInt(int64(j)), secp256k1.GeneratorOrder)
			il := new(big.Int).Exp(&index, big.NewInt(int64(l)), secp256k1.GeneratorOrder)
			mjil := new(big.Int).Mul(mj, il)
			Cjlmjil := common.BigIntToPoint(secp256k1.Curve.ScalarMult(&Cjl.X, &Cjl.Y, mjil.Bytes()))
			pt = common.BigIntToPoint(secp256k1.Curve.Add(&pt.X, &pt.Y, &Cjlmjil.X, &Cjlmjil.Y))
		}
	}
	if galphahalphaprime.X.Cmp(&pt.X) != 0 || galphahalphaprime.Y.Cmp(&pt.Y) != 0 {
		return false
	}

	return true
}

// AVSSVerifyShare - Verify-share from Cachin et al. 2002
func AVSSVerifyShare(C [][]common.Point, m big.Int, sigma big.Int, sigmaprime big.Int) bool {
	gsigma := common.BigIntToPoint(secp256k1.Curve.ScalarBaseMult(sigma.Bytes()))
	hsigmaprime := common.BigIntToPoint(secp256k1.Curve.ScalarMult(&secp256k1.H.X, &secp256k1.H.Y, sigmaprime.Bytes()))
	gsigmahsigmaprime := common.BigIntToPoint(secp256k1.Curve.Add(&gsigma.X, &gsigma.Y, &hsigmaprime.X, &hsigmaprime.Y))
	pt := common.Point{X: *big.NewInt(int64(0)), Y: *big.NewInt(int64(0))}
	for j := range C {
		Cj0 := C[j][0]
		mj := new(big.Int).Exp(&m, big.NewInt(int64(j)), secp256k1.GeneratorOrder)
		Cj0mj := common.BigIntToPoint(secp256k1.Curve.ScalarMult(&Cj0.X, &Cj0.Y, mj.Bytes()))
		pt = common.BigIntToPoint(secp256k1.Curve.Add(&pt.X, &pt.Y, &Cj0mj.X, &Cj0mj.Y))
	}

	if gsigmahsigmaprime.X.Cmp(&pt.X) != 0 || gsigmahsigmaprime.Y.Cmp(&pt.Y) != 0 {
		return false
	}

	return true
}

// To add commitments together
func AVSSAddCommitment(C1 [][]common.Point, C2 [][]common.Point) ([][]common.Point, error) {
	if len(C1) != len(C2) {
		return nil, errors.New("Commitment not same width")
	}
	sumC := make([][]common.Point, len(C1))
	for i := range C1 {
		if len(C1[i]) != len(C2[i]) {
			return nil, errors.New("Commitment not same length")
		}
		sumC[i] = make([]common.Point, len(C1))
		for j := range C1[i] {
			sumC[i][j] = common.BigIntToPoint(secp256k1.Curve.Add(&C1[i][j].X, &C1[i][j].Y, &C2[i][j].X, &C2[i][j].Y))
		}
	}
	return sumC, nil
}

// AVSSVerifyShareCommitment to test gsihr against C
func AVSSVerifyShareCommitment(C [][]common.Point, m big.Int, gsigmahsigmaprime common.Point) bool {
	pt := common.Point{X: *big.NewInt(int64(0)), Y: *big.NewInt(int64(0))}
	for j := range C {
		Cj0 := C[j][0]
		mj := new(big.Int).Exp(&m, big.NewInt(int64(j)), secp256k1.GeneratorOrder)
		Cj0mj := common.BigIntToPoint(secp256k1.Curve.ScalarMult(&Cj0.X, &Cj0.Y, mj.Bytes()))
		pt = common.BigIntToPoint(secp256k1.Curve.Add(&pt.X, &pt.Y, &Cj0mj.X, &Cj0mj.Y))
	}

	if gsigmahsigmaprime.X.Cmp(&pt.X) != 0 || gsigmahsigmaprime.Y.Cmp(&pt.Y) != 0 {
		return false
	}
	return true
}
