package pvss

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"

	"github.com/YZhenY/torus/common"
	"github.com/decred/dcrd/dcrec/secp256k1"
	"github.com/ethereum/go-ethereum/crypto/sha3"
)

var (
	s              = secp256k1.S256()
	fieldOrder     = common.HexToBigInt("fffffffffffffffffffffffffffffffffffffffffffffffffffffffefffffc2f")
	generatorOrder = common.HexToBigInt("fffffffffffffffffffffffffffffffebaaedce6af48a03bbfd25e8cd0364141")
	// scalar to the power of this is like square root, eg. y^sqRoot = y^0.5 (if it exists)
	sqRoot         = common.HexToBigInt("3fffffffffffffffffffffffffffffffffffffffffffffffffffffffbfffff0c")
	G              = common.Point{X: *s.Gx, Y: *s.Gy}
	H              = *hashToPoint(G.X.Bytes())
	GeneratorOrder = common.HexToBigInt("fffffffffffffffffffffffffffffffebaaedce6af48a03bbfd25e8cd0364141")
)

func Keccak256(data ...[]byte) []byte {
	d := sha3.NewKeccak256()
	for _, b := range data {
		d.Write(b)
	}
	return d.Sum(nil)
}

func hashToPoint(data []byte) *common.Point {
	keccakHash := Keccak256(data)
	x := new(big.Int)
	x.SetBytes(keccakHash)
	for {
		beta := new(big.Int)
		beta.Exp(x, big.NewInt(3), fieldOrder)
		beta.Add(beta, big.NewInt(7))
		beta.Mod(beta, fieldOrder)
		y := new(big.Int)
		y.Exp(beta, sqRoot, fieldOrder)
		if new(big.Int).Exp(y, big.NewInt(2), fieldOrder).Cmp(beta) == 0 {
			return &common.Point{X: *x, Y: *y}
		} else {
			x.Add(x, big.NewInt(1))
		}
	}
}

func RandomBigInt() *big.Int {
	randomInt, _ := rand.Int(rand.Reader, GeneratorOrder)
	return randomInt
}

// Eval computes the private share v = p(i).
func polyEval(polynomial common.PrimaryPolynomial, x int) *big.Int { // get private share
	xi := new(big.Int).SetInt64(int64(x))
	sum := new(big.Int)
	// for i := polynomial.Threshold - 1; i >= 0; i-- {
	// 	fmt.Println("i: ", i)
	// 	sum.Mul(sum, xi)
	// 	sum.Add(sum, &polynomial.Coeff[i])
	// }
	// sum.Mod(sum, fieldOrder)
	sum.Add(sum, &polynomial.Coeff[0])

	for i := 1; i < polynomial.Threshold; i++ {
		tmp := new(big.Int).Mul(xi, &polynomial.Coeff[i])
		sum.Add(sum, tmp)
		sum.Mod(sum, generatorOrder)
		xi.Mul(xi, big.NewInt(int64(x)))
		xi.Mod(xi, generatorOrder)
	}
	return sum
}

func getShares(polynomial common.PrimaryPolynomial, n int) []common.PrimaryShare { // TODO: should we assume that it's always evaluated from 1 to N?
	shares := make([]common.PrimaryShare, n)
	for i := range shares {
		shares[i] = common.PrimaryShare{Index: n + 1, Value: *polyEval(polynomial, i+1)}
	}
	return shares
}

// Commit creates a public commitment polynomial for the given base point b or
// the standard base if b == nil.
func getCommit(polynomial common.PrimaryPolynomial) []common.Point {
	commits := make([]common.Point, polynomial.Threshold)
	for i := range commits {
		commits[i] = common.BigIntToPoint(s.ScalarBaseMult(polynomial.Coeff[i].Bytes()))
	}
	// fmt.Println(commits[0].X.Text(16), commits[0].Y.Text(16), "commit0")
	// fmt.Println(commits[1].X.Text(16), commits[1].Y.Text(16), "commit1")
	return commits
}

func generateRandomPolynomial(secret big.Int, threshold int) *common.PrimaryPolynomial {
	// Create secret sharing polynomial
	coeff := make([]big.Int, threshold)
	coeff[0] = secret                //assign secret as coeff of x^0
	for i := 1; i < threshold; i++ { //randomly choose coeffs
		coeff[i] = *RandomBigInt()
	}
	return &common.PrimaryPolynomial{coeff, threshold}
}

func Signcrypt(recipientPubKey common.Point, data []byte, privKey big.Int) (*common.Signcryption, error) {
	// Blinding
	r := RandomBigInt()
	rG := common.BigIntToPoint(s.ScalarBaseMult(r.Bytes()))
	rU := common.BigIntToPoint(s.ScalarMult(&recipientPubKey.X, &recipientPubKey.Y, r.Bytes()))

	// encrypt with AES
	ciphertext, err := AESencrypt(rU.X.Bytes(), data)
	if err != nil {
		return nil, err
	}

	// Concat hashing bytes
	cb := data
	cb = append(cb[:], rG.X.Bytes()...)

	// hash h = H(M|r1)
	hashed := Keccak256(cb)
	h := new(big.Int).SetBytes(hashed)
	h.Mod(h, generatorOrder)

	s := new(big.Int)
	temp := new(big.Int)
	temp.Mul(h, r)
	temp.Mod(temp, generatorOrder)
	s.Sub(&privKey, temp)
	s.Mod(s, generatorOrder)

	return &common.Signcryption{*ciphertext, rG, *s}, nil
}

func UnSignCrypt(signcryption common.Signcryption, privKey big.Int, senderPubKey common.Point) (*[]byte, error) {
	xR := common.BigIntToPoint(s.ScalarMult(&signcryption.R.X, &signcryption.R.Y, privKey.Bytes()))
	M, err := AESdecrypt(xR.X.Bytes(), signcryption.Ciphertext)
	if err != nil {
		return nil, err
	}

	//Concat hashing bytes
	cb := []byte(*M)
	cb = append(cb[:], signcryption.R.X.Bytes()...)

	//hash h = H(M|r1)
	hashed := Keccak256(cb)
	h := new(big.Int).SetBytes(hashed)
	h.Mod(h, generatorOrder)

	//Verify signcryption
	sG := common.BigIntToPoint(s.ScalarBaseMult(signcryption.Signature.Bytes()))
	hR := common.BigIntToPoint(s.ScalarMult(&signcryption.R.X, &signcryption.R.Y, h.Bytes()))
	testsenderPubKey := common.BigIntToPoint(s.Add(&sG.X, &sG.Y, &hR.X, &hR.Y))
	if senderPubKey.X.Cmp(&testsenderPubKey.X) != 0 {
		fmt.Println(senderPubKey.X.Cmp(&testsenderPubKey.X))
		fmt.Println(senderPubKey)
		fmt.Println(testsenderPubKey)
		return nil, errors.New("sending node PK does not register with signcryption")
	}

	return M, nil
}

func signcryptShare(nodePubKey common.Point, share big.Int, privKey big.Int) (*common.Signcryption, error) {
	// Blinding
	r := RandomBigInt()
	rG := common.BigIntToPoint(s.ScalarBaseMult(r.Bytes()))
	rU := common.BigIntToPoint(s.ScalarMult(&nodePubKey.X, &nodePubKey.Y, r.Bytes()))

	// encrypt with AES
	ciphertext, err := AESencrypt(rU.X.Bytes(), share.Bytes())
	if err != nil {
		return nil, err
	}

	// Concat hashing bytes
	cb := share.Bytes()
	cb = append(cb[:], rG.X.Bytes()...)

	// hash h = H(M|r1)
	hashed := Keccak256(cb)
	h := new(big.Int).SetBytes(hashed)
	h.Mod(h, generatorOrder)

	s := new(big.Int)
	temp := new(big.Int)
	temp.Mul(h, r)
	temp.Mod(temp, generatorOrder)
	s.Sub(&privKey, temp)
	s.Mod(s, generatorOrder)

	return &common.Signcryption{*ciphertext, rG, *s}, nil
}

func batchSigncryptShare(nodeList []common.Point, shares []common.PrimaryShare, privKey big.Int) ([]*common.SigncryptedOutput, error) {
	signcryptedShares := make([]*common.SigncryptedOutput, len(nodeList))
	for i := range nodeList {
		temp, err := signcryptShare(nodeList[i], shares[i].Value, privKey)
		if err != nil {
			return nil, err
		}
		signcryptedShares[i] = &common.SigncryptedOutput{nodeList[i], shares[i].Index, *temp}
	}
	return signcryptedShares, nil
}

// use this instead of CreateAndPrepareShares
func CreateShares(nodes []common.Point, secret big.Int, threshold int, privKey big.Int) (*[]common.PrimaryShare, *[]common.Point, error) {
	n := len(nodes)

	polynomial := *generateRandomPolynomial(secret, threshold)

	// determine shares for polynomial with respect to basis point
	shares := getShares(polynomial, n)

	// committing to polynomial
	pubPoly := getCommit(polynomial)

	return &shares, &pubPoly, nil
}

// deprecated: use CreateShares and let client handle signcryption. Client may need to add more information before signcrypting (eg. broadcast id)
func CreateAndPrepareShares(nodes []common.Point, secret big.Int, threshold int, privKey big.Int) ([]*common.SigncryptedOutput, *[]common.Point, error) {
	n := len(nodes)

	polynomial := *generateRandomPolynomial(secret, threshold)

	// determine shares for polynomial with respect to basis point
	shares := getShares(polynomial, n)

	// committing to polynomial
	pubPoly := getCommit(polynomial)

	// signcrypt shares
	signcryptedShares, err := batchSigncryptShare(nodes, shares, privKey)
	if err != nil {
		return nil, nil, err
	}

	return signcryptedShares, &pubPoly, nil
}

func UnsigncryptShare(signcryption common.Signcryption, privKey big.Int, sendingNodePubKey common.Point) (*[]byte, error) {
	xR := common.BigIntToPoint(s.ScalarMult(&signcryption.R.X, &signcryption.R.Y, privKey.Bytes()))
	M, err := AESdecrypt(xR.X.Bytes(), signcryption.Ciphertext)
	if err != nil {
		return nil, err
	}

	//Concat hashing bytes
	cb := []byte(*M)
	cb = append(cb[:], signcryption.R.X.Bytes()...)

	//hash h = H(M|r1)
	hashed := Keccak256(cb)
	h := new(big.Int).SetBytes(hashed)
	h.Mod(h, generatorOrder)

	//Verify signcryption
	sG := common.BigIntToPoint(s.ScalarBaseMult(signcryption.Signature.Bytes()))
	hR := common.BigIntToPoint(s.ScalarMult(&signcryption.R.X, &signcryption.R.Y, h.Bytes()))
	testSendingNodePubKey := common.BigIntToPoint(s.Add(&sG.X, &sG.Y, &hR.X, &hR.Y))
	if sendingNodePubKey.X.Cmp(&testSendingNodePubKey.X) != 0 {
		fmt.Println(sendingNodePubKey.X.Cmp(&testSendingNodePubKey.X))
		fmt.Println(sendingNodePubKey)
		fmt.Println(testSendingNodePubKey)
		return nil, errors.New("sending node PK does not register with signcryption")
	}

	return M, nil
}

// func lagrangeNormal(shares []common.PrimaryShare) *big.Int {
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
// 	// secret.Mod(secret, generatorOrder)
// 	return secret
// }

func LagrangeElliptic(shares []common.PrimaryShare) *big.Int {
	secret := new(big.Int)
	for _, share := range shares {
		//when x =0
		delta := new(big.Int).SetInt64(int64(1))
		upper := new(big.Int).SetInt64(int64(1))
		lower := new(big.Int).SetInt64(int64(1))
		for j := range shares {
			if shares[j].Index != share.Index {
				upper.Mul(upper, new(big.Int).SetInt64(int64(shares[j].Index)))
				upper.Mod(upper, generatorOrder)
				upper.Neg(upper)

				tempLower := new(big.Int).SetInt64(int64(share.Index))
				tempLower.Sub(tempLower, new(big.Int).SetInt64(int64(shares[j].Index)))
				tempLower.Mod(tempLower, generatorOrder)

				lower.Mul(lower, tempLower)
				lower.Mod(lower, generatorOrder)
			}
		}
		//elliptic devision
		inv := new(big.Int)
		inv.ModInverse(lower, generatorOrder)
		delta.Mul(upper, inv)
		delta.Mod(delta, generatorOrder)

		delta.Mul(&share.Value, delta)
		delta.Mod(delta, generatorOrder)

		secret.Add(secret, delta)
	}
	secret.Mod(secret, generatorOrder)
	// secret.Mod(secret, generatorOrder)
	return secret
}
