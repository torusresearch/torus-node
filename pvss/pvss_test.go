package pvss

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"fmt"
	"math/big"
	"testing"

	"github.com/decred/dcrd/dcrec/secp256k1"
)

type NodeList struct {
	Index int
	Keys  ecdsa.PrivateKey
}

type PrimaryShares struct {
	Index int
	Value big.Int
}

type Point struct {
	x big.Int
	y big.Int
}

func fromHex(s string) *big.Int {
	r, ok := new(big.Int).SetString(s, 16)
	if !ok {
		panic("invalid hex in source file: " + s)
	}
	return r
}

var (
	s              = secp256k1.S256()
	fieldOrder     = fromHex("fffffffffffffffffffffffffffffffffffffffffffffffffffffffefffffc2f")
	generatorOrder = fromHex("fffffffffffffffffffffffffffffffebaaedce6af48a03bbfd25e8cd0364141")
)

func generateKeyPair() (pubkey, privkey []byte) {
	key, err := ecdsa.GenerateKey(secp256k1.S256(), rand.Reader)
	if err != nil {
		panic(err)
	}
	pubkey = elliptic.Marshal(secp256k1.S256(), key.X, key.Y)

	privkey = make([]byte, 32)
	blob := key.D.Bytes()
	copy(privkey[32-len(blob):], blob)

	return pubkey, privkey
}

func createRandomNodes(number int) []NodeList {
	list := make([]NodeList, number)
	for i := 0; i < number; i++ {
		key, _ := ecdsa.GenerateKey(secp256k1.S256(), rand.Reader)
		list[i] = NodeList{Index: i, Keys: *key}
	}
	return list
}

func randomBigInt() *big.Int {
	randomInt, _ := rand.Int(rand.Reader, fieldOrder)
	return randomInt
}

// Eval computes the private share v = p(i).
func polyEval(polynomial []big.Int, x int) big.Int { // get private share
	xi := new(big.Int).SetInt64(int64(x))
	sum := new(big.Int) //additive identity of curve = 0??? TODO: CHECK PLS
	for i := x - 1; i >= 0; i-- {
		sum.Mul(sum, xi)
		sum.Add(sum, &polynomial[i])
	}
	return *sum
}

func getShares(polynomial []big.Int, n int) []big.Int {
	shares := make([]big.Int, n)
	for i := range shares {
		shares[i] = polyEval(polynomial, i+1)
	}
	return shares
}

// Commit creates a public commitment polynomial for the given base point b or
// the standard base if b == nil.
func getCommit(polynomial []big.Int, threshold int, H big.Int) []Point {
	commits := make([]Point, threshold)
	for i := range commits {
		tmpx, tmpy := s.ScalarBaseMult(polynomial[i].Bytes())
		x, y := s.ScalarMult(tmpx, tmpy, H.Bytes())
		commits[i] = Point{x: *x, y: *y}
	}
	return commits
}

func encShares(nodes []NodeList, secret big.Int, threshold int, H big.Int) {
	n := len(nodes)
	encryptedShares := make([]big.Int, n) // TODO: structure for shares
	// Create secret sharing polynomial
	polynomial := make([]big.Int, threshold)
	polynomial[0] = secret           //assign secret as coeff of x^0
	for i := 1; i < threshold; i++ { //randomly choose polynomials
		polynomial[i] = *randomBigInt()
	}

	// determine shares for polynomial with respect to basis H
	shares := getShares(polynomial, n)

	//committing Yi and proof
	commits := getCommit(polynomial, threshold, H)

	// Create NIZK discrete-logarithm equality proofs
	fmt.Println(encryptedShares, shares, commits)

}

// Commit creates a public commitment polynomial for the given base point b or
// the standard base if b == nil.
// func (p *PriPoly) Commit(b kyber.Point) *PubPoly {
// 	commits := make([]kyber.Point, p.Threshold())
// 	for i := range commits {
// 		commits[i] = p.g.Point().Mul(p.coeffs[i], b)
// 	}
// 	return &PubPoly{p.g, b, commits}
// }

// DecryptShare first verifies the encrypted share against the encryption
// consistency proof and, if valid, decrypts it and creates a decryption
// consistency proof.
func DecShare(encShareX big.Int, encShareY big.Int, consistencyProof big.Int, key ecdsa.PrivateKey) big.Int {
	// if err := VerifyEncShare(suite, H, X, sH, encShare); err != nil {
	// 	return nil, err
	// }
	// G := suite.Point().Base()
	// V := suite.Point().Mul(suite.Scalar().Inv(x), encShare.S.V) // decryption: x^{-1} * (xS)
	modInv := new(big.Int)
	modInv.ModInverse(generatorOrder, key.D)
	// V := s.ScalarMult(encSharexX, encShareY, modInv.Bytes())
	// ps := &share.PubShare{I: encShare.S.I, V: V}
	// P, _, _, err := dleq.NewDLEQProof(suite, G, V, x)
	// if err != nil {
	// 	return nil, err
	// }
	// return &PubVerShare{*ps, *P}, nil
	i := new(big.Int)
	return *i
}

func TestRandom(test *testing.T) {
	fmt.Println("really?")
}

func TestPVSS(test *testing.T) {
	nodeWallets := createRandomNodes(21)
	fmt.Println(nodeWallets)
}
