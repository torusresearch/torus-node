package pvss

import (
	"math/big"

	"github.com/jinzhu/copier"

	"github.com/torusresearch/torus-common/common"
	"github.com/torusresearch/torus-common/secp256k1"
	pcmn "github.com/torusresearch/torus-node/common"
)

// Mobile Proactive Secret Sharing

// Generate random polynomial
// Note: this does not have a y-intercept of 0
func generateRandomPolynomial(threshold int) *pcmn.PrimaryPolynomial {
	coeff := make([]big.Int, threshold)
	for i := 0; i < threshold; i++ { //randomly choose coeffs
		coeff[i] = *RandomBigInt()
	}
	return &pcmn.PrimaryPolynomial{Coeff: coeff, Threshold: threshold}
}

func genPolyForTarget(x int, threshold int) *pcmn.PrimaryPolynomial {
	tempPoly := generateRandomPolynomial(threshold)
	var poly pcmn.PrimaryPolynomial
	_ = copier.Copy(&poly, &tempPoly)
	valAtX := polyEval(*tempPoly, x)
	temp := new(big.Int).Sub(&poly.Coeff[0], valAtX)
	poly.Coeff[0].Mod(temp, secp256k1.GeneratorOrder)
	return &poly
}

// LagrangePolys is used in PSS
// When each share is subshared, each share is associated with a commitment polynomial
// we then choose k such subsharings to form the refreshed shares and secrets
// those refreshed shares are lagrange interpolated, but they also correspond to a langrage
// interpolated polynomial commitment that is different from the original commitment
// here, we calculate this interpolated polynomial commitment
func LagrangePolys(indexes []int, polys [][]common.Point) (res []common.Point) {
	if len(polys) == 0 {
		return
	}
	if len(indexes) != len(polys) {
		return
	}
	for l := 0; l < len(polys[0]); l++ {
		sum := common.Point{X: *big.NewInt(int64(0)), Y: *big.NewInt(int64(0))}
		for j, index := range indexes {
			lambda := new(big.Int).SetInt64(int64(1))
			upper := new(big.Int).SetInt64(int64(1))
			lower := new(big.Int).SetInt64(int64(1))
			for _, otherIndex := range indexes {
				if otherIndex != index {
					tempUpper := big.NewInt(int64(0))
					tempUpper.Sub(tempUpper, big.NewInt(int64(otherIndex)))
					upper.Mul(upper, tempUpper)
					upper.Mod(upper, secp256k1.GeneratorOrder)

					tempLower := big.NewInt(int64(index))
					tempLower.Sub(tempLower, big.NewInt(int64(otherIndex)))
					tempLower.Mod(tempLower, secp256k1.GeneratorOrder)

					lower.Mul(lower, tempLower)
					lower.Mod(lower, secp256k1.GeneratorOrder)
				}
			}
			// finite field division
			inv := new(big.Int)
			inv.ModInverse(lower, secp256k1.GeneratorOrder)
			lambda.Mul(upper, inv)
			lambda.Mod(lambda, secp256k1.GeneratorOrder)
			tempPt := common.BigIntToPoint(secp256k1.Curve.ScalarMult(&polys[j][l].X, &polys[j][l].Y, lambda.Bytes()))
			sum = common.BigIntToPoint(secp256k1.Curve.Add(&tempPt.X, &tempPt.Y, &sum.X, &sum.Y))
		}
		res = append(res, sum)
	}

	return
}
