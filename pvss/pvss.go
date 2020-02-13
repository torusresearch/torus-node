package pvss

// A Simple Publicly Verifiable Secret Sharing
// Scheme and its Application to Electronic Voting

import (
	"crypto/ecdsa"
	"crypto/rand"
	"errors"
	"log"
	"math/big"
	"sort"

	ethCrypto "github.com/ethereum/go-ethereum/crypto"
	logging "github.com/sirupsen/logrus"
	"github.com/torusresearch/torus-common/common"
	"github.com/torusresearch/torus-common/secp256k1"
	pcmn "github.com/torusresearch/torus-node/common"
)

func RandomPoly(secret big.Int, threshold int) *pcmn.PrimaryPolynomial {
	return generateRandomZeroPolynomial(secret, threshold)
}

func RandomBigInt() *big.Int {
	randomInt, _ := rand.Int(rand.Reader, secp256k1.GeneratorOrder)
	return randomInt
}

// Exported function to evaluate polys
func PolyEval(polynomial pcmn.PrimaryPolynomial, x big.Int) *big.Int {
	return polyEval(polynomial, int(x.Int64()))
}

// Eval computes the private share v = p(i).
func polyEval(polynomial pcmn.PrimaryPolynomial, x int) *big.Int { // get private share
	xi := big.NewInt(int64(x))
	sum := new(big.Int)
	// for i := polynomial.Threshold - 1; i >= 0; i-- {
	// 	logging.WithField("i", i).Debug()
	// 	sum.Mul(sum, xi)
	// 	sum.Add(sum, &polynomial.Coeff[i])
	// }
	// sum.Mod(sum, secp256k1.FieldOrder)
	sum.Add(sum, &polynomial.Coeff[0])

	for i := 1; i < polynomial.Threshold; i++ {
		tmp := new(big.Int).Mul(xi, &polynomial.Coeff[i])
		sum.Add(sum, tmp)
		sum.Mod(sum, secp256k1.GeneratorOrder)
		xi.Mul(xi, big.NewInt(int64(x)))
		xi.Mod(xi, secp256k1.GeneratorOrder)
	}
	return sum
}

func getShares(polynomial pcmn.PrimaryPolynomial, nodes []pcmn.Node) []pcmn.PrimaryShare {
	shares := make([]pcmn.PrimaryShare, len(nodes))
	for i, node := range nodes {
		shares[i] = pcmn.PrimaryShare{Index: node.Index, Value: *polyEval(polynomial, node.Index)}
	}
	return shares
}

// Commit creates a public commitment polynomial
func GetCommit(polynomial pcmn.PrimaryPolynomial) []common.Point {
	commits := make([]common.Point, polynomial.Threshold)
	for i := range commits {
		commits[i] = common.BigIntToPoint(secp256k1.Curve.ScalarBaseMult(polynomial.Coeff[i].Bytes()))
	}
	return commits
}

func AddCommitments(commitments ...[]common.Point) (sumCommit []common.Point) {
	for i := range commitments[0] {
		var sumPt common.Point
		for _, commitment := range commitments {
			sumPt = common.BigIntToPoint(secp256k1.Curve.Add(&sumPt.X, &sumPt.Y, &commitment[i].X, &commitment[i].Y))
		}
		sumCommit = append(sumCommit, sumPt)
	}
	return
}

// AddPolynomials add two polynomials (modulo generator order Q)
func AddPolynomials(poly1 pcmn.PrimaryPolynomial, poly2 pcmn.PrimaryPolynomial) *pcmn.PrimaryPolynomial {
	var sumPoly []big.Int
	if poly1.Threshold != poly2.Threshold {
		logging.Error("thresholds of two polynomials are not equal")
		return &pcmn.PrimaryPolynomial{Coeff: sumPoly, Threshold: 0}
	}

	if len(poly1.Coeff) != len(poly2.Coeff) {
		logging.Error("order of two polynomials are not equal")
		return &pcmn.PrimaryPolynomial{Coeff: sumPoly, Threshold: 0}
	}

	for i := range poly1.Coeff {
		tmpCoeff := new(big.Int).Add(&poly1.Coeff[i], &poly2.Coeff[i])
		sumPoly = append(sumPoly, *new(big.Int).Mod(tmpCoeff, secp256k1.GeneratorOrder))
	}

	return &pcmn.PrimaryPolynomial{Coeff: sumPoly, Threshold: poly1.Threshold}
}

func generateRandomZeroPolynomial(secret big.Int, threshold int) *pcmn.PrimaryPolynomial {
	// Create secret sharing polynomial
	coeff := make([]big.Int, threshold)
	coeff[0] = secret                //assign secret as coeff of x^0
	for i := 1; i < threshold; i++ { //randomly choose coeffs
		coeff[i] = *RandomBigInt()
	}
	return &pcmn.PrimaryPolynomial{Coeff: coeff, Threshold: threshold}
}

func Signcrypt(recipientPubKey common.Point, data []byte, privKey big.Int) (*pcmn.Signcryption, error) {
	// Blinding
	r := RandomBigInt()
	rG := common.BigIntToPoint(secp256k1.Curve.ScalarBaseMult(r.Bytes()))
	rU := common.BigIntToPoint(secp256k1.Curve.ScalarMult(&recipientPubKey.X, &recipientPubKey.Y, r.Bytes()))

	// encrypt with AES
	ciphertext, err := AESencrypt(rU.X.Bytes(), data)
	if err != nil {
		return nil, err
	}

	// Concat hashing bytes
	cb := data
	cb = append(cb[:], rG.X.Bytes()...)

	// hash h = secp256k1.H(M|r1)
	hashed := secp256k1.Keccak256(cb)
	h := new(big.Int).SetBytes(hashed)
	h.Mod(h, secp256k1.GeneratorOrder)

	szecret := new(big.Int)
	temp := new(big.Int)
	temp.Mul(h, r)
	temp.Mod(temp, secp256k1.GeneratorOrder)
	szecret.Sub(&privKey, temp)
	szecret.Mod(szecret, secp256k1.GeneratorOrder)

	return &pcmn.Signcryption{Ciphertext: *ciphertext, R: rG, Signature: *szecret}, nil
}

func bytes32(bytes []byte) [32]byte {
	tmp := [32]byte{}
	copy(tmp[:], bytes)
	return tmp
}

func ECDSASignBytes(b []byte, privKey *big.Int) []byte {
	return ECDSASign(string(b), privKey)
}

// Signs using Ethereum ECDSA (where randomness is deterministic)
func ECDSASign(s string, privKey *big.Int) []byte {
	pubKey := common.BigIntToPoint(secp256k1.Curve.ScalarBaseMult(privKey.Bytes()))
	ecdsaPrivKey := &ecdsa.PrivateKey{
		PublicKey: ecdsa.PublicKey{
			Curve: secp256k1.Curve,
			X:     &pubKey.X,
			Y:     &pubKey.Y,
		},
		D: privKey,
	}
	hashRaw := secp256k1.Keccak256([]byte(s))
	signature, err := ethCrypto.Sign(hashRaw, ecdsaPrivKey)
	if err != nil {
		log.Fatal(err)
	}
	return signature
}

// func ECDSAValidateRaw(ecdsaPubBytes []byte, messageHash []byte, signature []byte) bool {
// 	return ethCrypto.VerifySignature(ecdsaPubBytes, messageHash, signature)
// }

func ECDSAVerify(str string, pubKey *common.Point, signature []byte) bool {
	r := new(big.Int)
	s := new(big.Int)
	r.SetBytes(signature[:32])
	s.SetBytes(signature[32:64])

	ecdsaPubKey := &ecdsa.PublicKey{
		Curve: secp256k1.Curve,
		X:     &(*pubKey).X,
		Y:     &(*pubKey).Y,
	}

	return ecdsa.Verify(
		ecdsaPubKey,
		secp256k1.Keccak256([]byte(str)),
		r,
		s,
	)
}

func ECDSAVerifyBytes(b []byte, pubKey *common.Point, signature []byte) bool {
	return ECDSAVerify(string(b), pubKey, signature)
}

func UnSignCrypt(signcryption pcmn.Signcryption, privKey big.Int, senderPubKey common.Point) (*[]byte, error) {
	xR := common.BigIntToPoint(secp256k1.Curve.ScalarMult(&signcryption.R.X, &signcryption.R.Y, privKey.Bytes()))
	M, err := AESdecrypt(xR.X.Bytes(), signcryption.Ciphertext)
	if err != nil {
		return nil, err
	}

	//Concat hashing bytes
	cb := []byte(*M)
	cb = append(cb[:], signcryption.R.X.Bytes()...)

	//hash h = secp256k1.H(M|r1)
	hashed := secp256k1.Keccak256(cb)
	h := new(big.Int).SetBytes(hashed)
	h.Mod(h, secp256k1.GeneratorOrder)

	//Verify signcryption
	sG := common.BigIntToPoint(secp256k1.Curve.ScalarBaseMult(signcryption.Signature.Bytes()))
	hR := common.BigIntToPoint(secp256k1.Curve.ScalarMult(&signcryption.R.X, &signcryption.R.Y, h.Bytes()))
	testsenderPubKey := common.BigIntToPoint(secp256k1.Curve.Add(&sG.X, &sG.Y, &hR.X, &hR.Y))
	if senderPubKey.X.Cmp(&testsenderPubKey.X) != 0 {
		logging.WithFields(logging.Fields{
			"sign":             senderPubKey.X.Cmp(&testsenderPubKey.X),
			"senderPubKey":     senderPubKey,
			"testsenderPubKey": testsenderPubKey,
		}).Debug()
		return nil, errors.New("sending node PK does not register with signcryption unsigncrypt")
	}

	return M, nil
}

func signcryptShare(nodePubKey common.Point, share big.Int, privKey big.Int) (*pcmn.Signcryption, error) {
	// Blinding
	r := RandomBigInt()
	rG := common.BigIntToPoint(secp256k1.Curve.ScalarBaseMult(r.Bytes()))
	rU := common.BigIntToPoint(secp256k1.Curve.ScalarMult(&nodePubKey.X, &nodePubKey.Y, r.Bytes()))

	// encrypt with AES
	ciphertext, err := AESencrypt(rU.X.Bytes(), share.Bytes())
	if err != nil {
		return nil, err
	}

	// Concat hashing bytes
	cb := share.Bytes()
	cb = append(cb[:], rG.X.Bytes()...)

	// hash h = secp256k1.H(M|r1)
	hashed := secp256k1.Keccak256(cb)
	h := new(big.Int).SetBytes(hashed)
	h.Mod(h, secp256k1.GeneratorOrder)

	szecret := new(big.Int)
	temp := new(big.Int)
	temp.Mul(h, r)
	temp.Mod(temp, secp256k1.GeneratorOrder)
	szecret.Sub(&privKey, temp)
	szecret.Mod(szecret, secp256k1.GeneratorOrder)

	return &pcmn.Signcryption{Ciphertext: *ciphertext, R: rG, Signature: *szecret}, nil
}

func batchSigncryptShare(nodeList []pcmn.Node, shares []pcmn.PrimaryShare, privKey big.Int) ([]*pcmn.SigncryptedOutput, error) {
	signcryptedShares := make([]*pcmn.SigncryptedOutput, len(nodeList))
	for i := range nodeList {
		temp, err := signcryptShare(nodeList[i].PubKey, shares[i].Value, privKey)
		if err != nil {
			return nil, err
		}
		signcryptedShares[i] = &pcmn.SigncryptedOutput{NodePubKey: nodeList[i].PubKey, NodeIndex: shares[i].Index, SigncryptedShare: *temp}
	}
	return signcryptedShares, nil
}

// use this instead of CreateAndPrepareShares
func CreateShares(nodes []pcmn.Node, secret big.Int, threshold int) (*[]pcmn.PrimaryShare, *[]common.Point, error) {

	polynomial := *generateRandomZeroPolynomial(secret, threshold)

	// determine shares for polynomial with respect to basis point
	shares := getShares(polynomial, nodes)

	// committing to polynomial
	pubPoly := GetCommit(polynomial)

	return &shares, &pubPoly, nil
}

// deprecated: use CreateShares and let client handle signcryption. Client may need to add more information before signcrypting (eg. broadcast id)
func CreateAndPrepareShares(nodes []pcmn.Node, secret big.Int, threshold int, privKey big.Int) ([]*pcmn.SigncryptedOutput, *[]common.Point, *pcmn.PrimaryPolynomial, error) {
	polynomial := *generateRandomZeroPolynomial(secret, threshold)

	// determine shares for polynomial with respect to basis point
	shares := getShares(polynomial, nodes)

	// committing to polynomial
	pubPoly := GetCommit(polynomial)

	// signcrypt shares
	signcryptedShares, err := batchSigncryptShare(nodes, shares, privKey)
	if err != nil {
		return nil, nil, nil, err
	}

	return signcryptedShares, &pubPoly, &polynomial, nil
}

func UnsigncryptShare(signcryption pcmn.Signcryption, privKey big.Int, sendingNodePubKey common.Point) (*[]byte, error) {
	xR := common.BigIntToPoint(secp256k1.Curve.ScalarMult(&signcryption.R.X, &signcryption.R.Y, privKey.Bytes()))
	M, err := AESdecrypt(xR.X.Bytes(), signcryption.Ciphertext)
	if err != nil {
		return nil, err
	}

	//Concat hashing bytes
	cb := []byte(*M)
	cb = append(cb[:], signcryption.R.X.Bytes()...)

	//hash h = secp256k1.H(M|r1)
	hashed := secp256k1.Keccak256(cb)
	h := new(big.Int).SetBytes(hashed)
	h.Mod(h, secp256k1.GeneratorOrder)

	//Verify signcryption
	sG := common.BigIntToPoint(secp256k1.Curve.ScalarBaseMult(signcryption.Signature.Bytes()))
	hR := common.BigIntToPoint(secp256k1.Curve.ScalarMult(&signcryption.R.X, &signcryption.R.Y, h.Bytes()))
	testSendingNodePubKey := common.BigIntToPoint(secp256k1.Curve.Add(&sG.X, &sG.Y, &hR.X, &hR.Y))
	if sendingNodePubKey.X.Cmp(&testSendingNodePubKey.X) != 0 {
		logging.WithFields(logging.Fields{
			"sign":             sendingNodePubKey.X.Cmp(&testSendingNodePubKey.X),
			"senderNodePubKey": sendingNodePubKey,
			"testSenderPubKey": testSendingNodePubKey,
		}).Debug()
		return nil, errors.New("sending node PK does not register with signcryption")
	}

	return M, nil
}

// func lagrangeNormal(shares []pcmn.PrimaryShare) *big.Int {
// 	secret := new(big.Int)
// 	for _, share := range shares {
// 		//when x =0
// 		delta := new(big.Int).SetInt64(int64(1))
// 		upper := new(big.Int).SetInt64(int64(1))
// 		lower := new(big.Int).SetInt64(int64(1))
// 		for j := range shares {
// 			if shares[j].Index != share.Index {
// 				upper.Mul(upper, new(big.Int).SetInt64(int64(shares[j].Index)))
// 				upper.Neg(upper)

// 				tempLower := new(big.Int).SetInt64(int64(share.Index))
// 				tempLower.Sub(tempLower, new(big.Int).SetInt64(int64(shares[j].Index)))

// 				lower.Mul(lower, tempLower)
// 			}
// 		}
// 		delta.Div(upper, lower)
// 		delta.Mul(&share.Value, delta)
// 		secret.Add(secret, delta)
// 	}
// 	// secret.Mod(secret, secp256k1.GeneratorOrder)
// 	return secret
// }

func LagrangeInterpolatePolynomial(points []common.Point) []big.Int {
	denominator := func(i int, points []common.Point) big.Int {
		result := big.NewInt(int64(1))
		x_i := points[i].X
		for j := len(points) - 1; j >= 0; j-- {
			if i != j {
				tmp := new(big.Int).Sub(&x_i, &points[j].X)
				tmp.Mod(tmp, secp256k1.GeneratorOrder)
				result.Mul(result, tmp)
				result.Mod(result, secp256k1.GeneratorOrder)
			}
		}
		return *result
	}
	interpolationPoly := func(i int, points []common.Point) []big.Int {
		coefficients := make([]big.Int, len(points))
		d := denominator(i, points)
		coefficients[0] = *new(big.Int).ModInverse(&d, secp256k1.GeneratorOrder)
		for k := 0; k < len(points); k++ {
			new_coefficients := make([]big.Int, len(points))
			if k == i {
				continue
			}
			var j int
			if k < i {
				j = k + 1
			} else {
				j = k
			}
			j = j - 1
			for ; j >= 0; j-- {
				new_coefficients[j+1].Add(&new_coefficients[j+1], &coefficients[j])
				new_coefficients[j+1].Mod(&new_coefficients[j+1], secp256k1.GeneratorOrder)
				tmp := new(big.Int).Mul(&points[k].X, &coefficients[j])
				tmp.Mod(tmp, secp256k1.GeneratorOrder)
				new_coefficients[j].Sub(&new_coefficients[j], tmp)
				new_coefficients[j].Mod(&new_coefficients[j], secp256k1.GeneratorOrder)
			}
			coefficients = new_coefficients
		}
		return coefficients
	}
	pointSort := func(points []common.Point) []common.Point {
		sortedPoints := make([]common.Point, len(points))
		copy(sortedPoints, points)
		sort.SliceStable(sortedPoints, func(i, j int) bool {
			return sortedPoints[i].X.Cmp(&sortedPoints[j].X) == -1
		})
		return sortedPoints[:]
	}
	lagrange := func(unsortedPoints []common.Point) []big.Int {
		points := pointSort(unsortedPoints)
		polynomial := make([]big.Int, len(points))
		for i := 0; i < len(points); i++ {
			coefficients := interpolationPoly(i, points)
			for k := 0; k < len(points); k++ {
				polynomial[k].Add(&polynomial[k], new(big.Int).Mul(&points[i].Y, &coefficients[k]))
				polynomial[k].Mod(&polynomial[k], secp256k1.GeneratorOrder)
			}
		}
		return polynomial
	}
	return lagrange(points)
}

func LagrangeScalarCP(pts []common.Point, target int) *big.Int {
	var shares []pcmn.PrimaryShare
	for _, pt := range pts {
		shares = append(shares, pcmn.PrimaryShare{
			Index: int(pt.X.Int64()),
			Value: pt.Y,
		})
	}
	return LagrangeScalar(shares, target)
}

func LagrangeScalar(shares []pcmn.PrimaryShare, target int) *big.Int {
	secret := new(big.Int)
	for _, share := range shares {
		delta := new(big.Int).SetInt64(int64(1))
		upper := new(big.Int).SetInt64(int64(1))
		lower := new(big.Int).SetInt64(int64(1))
		for j := range shares {
			if shares[j].Index != share.Index {
				tempUpper := big.NewInt(int64(target))
				tempUpper.Sub(tempUpper, big.NewInt(int64(shares[j].Index)))
				upper.Mul(upper, tempUpper)
				upper.Mod(upper, secp256k1.GeneratorOrder)

				tempLower := big.NewInt(int64(share.Index))
				tempLower.Sub(tempLower, big.NewInt(int64(shares[j].Index)))
				tempLower.Mod(tempLower, secp256k1.GeneratorOrder)

				lower.Mul(lower, tempLower)
				lower.Mod(lower, secp256k1.GeneratorOrder)
			}
		}
		// finite field division
		inv := new(big.Int)
		inv.ModInverse(lower, secp256k1.GeneratorOrder)
		delta.Mul(upper, inv)
		delta.Mod(delta, secp256k1.GeneratorOrder)

		delta.Mul(&share.Value, delta)
		delta.Mod(delta, secp256k1.GeneratorOrder)

		secret.Add(secret, delta)
	}
	secret.Mod(secret, secp256k1.GeneratorOrder)
	return secret
}

// LagrangeCurvePts finds the ^0 coefficient for points given in points an indexes given
func LagrangeCurvePts(indexes []int, points []common.Point) *common.Point {
	var sm [][]common.Point
	for i := 0; i < len(points); i++ {
		var temp []common.Point
		temp = append(temp, points[i])
		sm = append(sm, temp)
	}
	poly := LagrangePolys(indexes, sm)
	return &poly[0]
}

func SumScalars(scalars ...big.Int) big.Int {
	sumScalar := big.NewInt(int64(0))
	for _, scalar := range scalars {
		sumScalar.Add(sumScalar, &scalar)
	}
	sumScalar.Mod(sumScalar, secp256k1.GeneratorOrder)
	return *sumScalar
}

func SumPoints(pts ...common.Point) (sumPt common.Point) {
	for _, pt := range pts {
		sumPt = common.BigIntToPoint(secp256k1.Curve.Add(&sumPt.X, &sumPt.Y, &pt.X, &pt.Y))
	}
	return
}
