package pvss

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"

	"github.com/decred/dcrd/dcrec/secp256k1"
	"github.com/ethereum/go-ethereum/crypto/sha3"
)

type SigncryptedOutput struct {
	NodePubKey       Point
	NodeIndex        int
	SigncryptedShare Signcryption
}

type Signcryption struct {
	Ciphertext []byte
	R          Point
	Signature  big.Int
}

type PrimaryPolynomial struct {
	coeff     []big.Int
	threshold int
}

type PrimaryShare struct {
	Index int
	Value big.Int
}

type Point struct {
	X big.Int
	Y big.Int
}

func fromHex(s string) *big.Int {
	r, ok := new(big.Int).SetString(s, 16)
	if !ok {
		panic("invalid hex in source file: " + s)
	}
	return r
}

func pt(x, y *big.Int) Point {
	return Point{X: *x, Y: *y}
}

var (
	s              = secp256k1.S256()
	fieldOrder     = fromHex("fffffffffffffffffffffffffffffffffffffffffffffffffffffffefffffc2f")
	generatorOrder = fromHex("fffffffffffffffffffffffffffffffebaaedce6af48a03bbfd25e8cd0364141")
	// scalar to the power of this is like square root, eg. y^sqRoot = y^0.5 (if it exists)
	sqRoot         = fromHex("3fffffffffffffffffffffffffffffffffffffffffffffffffffffffbfffff0c")
	G              = Point{X: *s.Gx, Y: *s.Gy}
	H              = hashToPoint(G.X.Bytes())
	GeneratorOrder = fromHex("fffffffffffffffffffffffffffffffebaaedce6af48a03bbfd25e8cd0364141")
)

func Keccak256(data ...[]byte) []byte {
	d := sha3.NewKeccak256()
	for _, b := range data {
		d.Write(b)
	}
	return d.Sum(nil)
}

func hashToPoint(data []byte) *Point {
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
			return &Point{X: *x, Y: *y}
		} else {
			x.Add(x, big.NewInt(1))
		}
	}
}

func RandomBigInt() *big.Int {
	randomInt, _ := rand.Int(rand.Reader, fromHex("3ffffffffffffffffffffffffffffffffffffffffffffffffbfffff0c"))
	return randomInt
}

// Eval computes the private share v = p(i).
func polyEval(polynomial PrimaryPolynomial, x int) *big.Int { // get private share
	xi := new(big.Int).SetInt64(int64(x))
	sum := new(big.Int)
	// for i := polynomial.threshold - 1; i >= 0; i-- {
	// 	fmt.Println("i: ", i)
	// 	sum.Mul(sum, xi)
	// 	sum.Add(sum, &polynomial.coeff[i])
	// }
	// sum.Mod(sum, fieldOrder)
	sum.Add(sum, &polynomial.coeff[0])

	for i := 1; i < polynomial.threshold; i++ {
		tmp := new(big.Int).Mul(xi, &polynomial.coeff[i])
		sum.Add(sum, tmp)
		sum.Mod(sum, generatorOrder)
		xi.Mul(xi, big.NewInt(int64(x)))
		xi.Mod(xi, generatorOrder)
	}
	return sum
}

func getShares(polynomial PrimaryPolynomial, n int) []PrimaryShare {
	shares := make([]PrimaryShare, n)
	for i := range shares {
		shares[i] = PrimaryShare{Index: n + 1, Value: *polyEval(polynomial, i+1)}
	}
	return shares
}

// Commit creates a public commitment polynomial for the given base point b or
// the standard base if b == nil.
func getCommit(polynomial PrimaryPolynomial) []Point {
	commits := make([]Point, polynomial.threshold)
	for i := range commits {
		commits[i] = pt(s.ScalarBaseMult(polynomial.coeff[i].Bytes()))
	}
	// fmt.Println(commits[0].X.Text(16), commits[0].Y.Text(16), "commit0")
	// fmt.Println(commits[1].X.Text(16), commits[1].Y.Text(16), "commit1")
	return commits
}

func generateRandomPolynomial(secret big.Int, threshold int) *PrimaryPolynomial {
	// Create secret sharing polynomial
	coeff := make([]big.Int, threshold)
	coeff[0] = secret                //assign secret as coeff of x^0
	for i := 1; i < threshold; i++ { //randomly choose coeffs
		coeff[i] = *RandomBigInt()
	}
	return &PrimaryPolynomial{coeff, threshold}
}

func signcryptShare(nodePubKey Point, share big.Int, privKey big.Int) (*Signcryption, error) {
	// Commitment
	r := RandomBigInt()
	rG := pt(s.ScalarBaseMult(r.Bytes()))
	rU := pt(s.ScalarMult(&nodePubKey.X, &nodePubKey.Y, r.Bytes()))

	//encrypt with AES
	ciphertext, err := AESencrypt(rU.X.Bytes(), share.Bytes())
	if err != nil {
		return nil, err
	}

	//Concat hashing bytes
	cb := share.Bytes()
	cb = append(cb[:], rG.X.Bytes()...)

	//hash h = H(M|r1)
	hashed := Keccak256(cb)
	h := new(big.Int).SetBytes(hashed)
	h.Mod(h, generatorOrder)

	s := new(big.Int)
	temp := new(big.Int)
	temp.Mul(h, r)
	temp.Mod(temp, generatorOrder)
	s.Sub(&privKey, temp)
	s.Mod(s, generatorOrder)

	return &Signcryption{*ciphertext, rG, *s}, nil
}

func batchSigncryptShare(nodeList []Point, shares []PrimaryShare, privKey big.Int) ([]*SigncryptedOutput, error) {
	signcryptedShares := make([]*SigncryptedOutput, len(nodeList))
	for i := range nodeList {
		temp, err := signcryptShare(nodeList[i], shares[i].Value, privKey)
		if err != nil {
			return nil, err
		}
		signcryptedShares[i] = &SigncryptedOutput{nodeList[i], shares[i].Index, *temp}
	}
	return signcryptedShares, nil
}

func CreateAndPrepareShares(nodes []Point, secret big.Int, threshold int, privKey big.Int) ([]*SigncryptedOutput, *[]Point, error) {
	n := len(nodes)

	polynomial := *generateRandomPolynomial(secret, threshold)

	// determine shares for polynomial with respect to basis point
	shares := getShares(polynomial, n)

	//committing to polynomial
	pubPoly := getCommit(polynomial)

	// Create NIZK discrete-logarithm equality proofs
	signcryptedShares, err := batchSigncryptShare(nodes, shares, privKey)
	if err != nil {
		return nil, nil, err
	}

	return signcryptedShares, &pubPoly, nil
}

func UnsigncryptShare(signcryption Signcryption, privKey big.Int, sendingNodePubKey Point) (*[]byte, error) {
	xR := pt(s.ScalarMult(&signcryption.R.X, &signcryption.R.Y, privKey.Bytes()))
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
	sG := pt(s.ScalarBaseMult(signcryption.Signature.Bytes()))
	hR := pt(s.ScalarMult(&signcryption.R.X, &signcryption.R.Y, h.Bytes()))
	testSendingNodePubKey := pt(s.Add(&sG.X, &sG.Y, &hR.X, &hR.Y))
	if sendingNodePubKey.X.Cmp(&testSendingNodePubKey.X) != 0 {
		fmt.Println(sendingNodePubKey.X.Cmp(&testSendingNodePubKey.X))
		fmt.Println(sendingNodePubKey)
		fmt.Println(testSendingNodePubKey)
		return nil, errors.New("sending node PK does not register with signcryption")
	}

	return M, nil
}

// func lagrangeNormal(shares []PrimaryShare) *big.Int {
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

func LagrangeElliptic(shares []PrimaryShare) *big.Int {
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
