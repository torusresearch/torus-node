package pvss

// A Simple Publicly Verifiable Secret Sharing
// Scheme and its Application to Electronic Voting

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"

	"github.com/YZhenY/torus/common"
	"github.com/YZhenY/torus/secp256k1"
)

func RandomBigInt() *big.Int {
	randomInt, _ := rand.Int(rand.Reader, secp256k1.GeneratorOrder)
	return randomInt
}

// Eval computes the private share v = p(i).
func polyEval(polynomial common.PrimaryPolynomial, x big.Int) *big.Int { // get private share
	xi := new(big.Int).SetBytes(x.Bytes())
	sum := new(big.Int)
	// for i := polynomial.Threshold - 1; i >= 0; i-- {
	// 	fmt.Println("i: ", i)
	// 	sum.Mul(sum, xi)
	// 	sum.Add(sum, &polynomial.Coeff[i])
	// }
	// sum.Mod(sum, secp256k1.FieldOrder)
	sum.Add(sum, &polynomial.Coeff[0])

	for i := 1; i < polynomial.Threshold; i++ {
		tmp := new(big.Int).Mul(xi, &polynomial.Coeff[i])
		sum.Add(sum, tmp)
		sum.Mod(sum, secp256k1.GeneratorOrder)
		xi.Mul(xi, &x)
		xi.Mod(xi, secp256k1.GeneratorOrder)
	}
	return sum
}

func getShares(polynomial common.PrimaryPolynomial, nodes []common.Node) []common.PrimaryShare { // TODO: should we assume that it's always evaluated from 1 to N?
	shares := make([]common.PrimaryShare, len(nodes))
	for i, node := range nodes {
		shares[i] = common.PrimaryShare{Index: node.Index, Value: *polyEval(polynomial, node.Index)}
	}
	return shares
}

// Commit creates a public commitment polynomial for the given base point b or
// the standard base if b == nil.
func getCommit(polynomial common.PrimaryPolynomial) []common.Point {
	commits := make([]common.Point, polynomial.Threshold)
	for i := range commits {
		commits[i] = common.BigIntToPoint(secp256k1.Curve.ScalarBaseMult(polynomial.Coeff[i].Bytes()))
	}
	// fmt.Println(commits[0].X.Text(16), commits[0].Y.Text(16), "commit0")
	// fmt.Println(commits[1].X.Text(16), commits[1].Y.Text(16), "commit1")
	return commits
}

func generateRandomZeroPolynomial(secret big.Int, threshold int) *common.PrimaryPolynomial {
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

	return &common.Signcryption{*ciphertext, rG, *szecret}, nil
}

func UnSignCrypt(signcryption common.Signcryption, privKey big.Int, senderPubKey common.Point) (*[]byte, error) {
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

	return &common.Signcryption{*ciphertext, rG, *szecret}, nil
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
func CreateShares(nodePubKeys []common.Point, secret big.Int, threshold int) (*[]common.PrimaryShare, *[]common.Point, error) {

	polynomial := *generateRandomZeroPolynomial(secret, threshold)

	// determine shares for polynomial with respect to basis point
	// TODO: IMPT
	nodes := make([]common.Node, len(nodePubKeys))
	for i, nodePubKey := range nodePubKeys {
		nodes[i] = common.Node{*big.NewInt(int64(i + 1)), nodePubKey}
	}
	shares := getShares(polynomial, nodes)

	// committing to polynomial
	pubPoly := getCommit(polynomial)

	return &shares, &pubPoly, nil
}

// deprecated: use CreateShares and let client handle signcryption. Client may need to add more information before signcrypting (eg. broadcast id)
func CreateAndPrepareShares(nodePubKeys []common.Point, secret big.Int, threshold int, privKey big.Int) ([]*common.SigncryptedOutput, *[]common.Point, error) {

	polynomial := *generateRandomZeroPolynomial(secret, threshold)

	// determine shares for polynomial with respect to basis point
	nodes := make([]common.Node, len(nodePubKeys))
	for i, nodePubKey := range nodePubKeys {
		nodes[i] = common.Node{*big.NewInt(int64(i + 1)), nodePubKey}
	}
	shares := getShares(polynomial, nodes)

	// committing to polynomial
	pubPoly := getCommit(polynomial)

	// signcrypt shares
	signcryptedShares, err := batchSigncryptShare(nodePubKeys, shares, privKey)
	if err != nil {
		return nil, nil, err
	}

	return signcryptedShares, &pubPoly, nil
}

func UnsigncryptShare(signcryption common.Signcryption, privKey big.Int, sendingNodePubKey common.Point) (*[]byte, error) {
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
// 	// secret.Mod(secret, secp256k1.GeneratorOrder)
// 	return secret
// }

func LagrangeScalar(shares []common.PrimaryShare) *big.Int {
	secret := new(big.Int)
	for _, share := range shares {
		//when x =0
		delta := new(big.Int).SetInt64(int64(1))
		upper := new(big.Int).SetInt64(int64(1))
		lower := new(big.Int).SetInt64(int64(1))
		for j := range shares {
			if shares[j].Index.Cmp(&share.Index) != 0 {
				upper.Mul(upper, &shares[j].Index)
				upper.Mod(upper, secp256k1.GeneratorOrder)
				upper.Neg(upper)

				tempLower := new(big.Int).SetBytes(share.Index.Bytes())
				tempLower.Sub(tempLower, &shares[j].Index)
				tempLower.Mod(tempLower, secp256k1.GeneratorOrder)

				lower.Mul(lower, tempLower)
				lower.Mod(lower, secp256k1.GeneratorOrder)
			}
		}
		//elliptic devision
		inv := new(big.Int)
		inv.ModInverse(lower, secp256k1.GeneratorOrder)
		delta.Mul(upper, inv)
		delta.Mod(delta, secp256k1.GeneratorOrder)

		delta.Mul(&share.Value, delta)
		delta.Mod(delta, secp256k1.GeneratorOrder)

		secret.Add(secret, delta)
	}
	secret.Mod(secret, secp256k1.GeneratorOrder)
	// secret.Mod(secret, secp256k1.GeneratorOrder)
	return secret
}
