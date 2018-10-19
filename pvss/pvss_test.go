package pvss

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"fmt"
	"math/big"
	"reflect"
	"testing"

	"github.com/decred/dcrd/dcrec/secp256k1"
	"github.com/ethereum/go-ethereum/crypto/sha3"
	"github.com/stretchr/testify/assert"
)

type NodeList struct {
	Index int
	Keys  ecdsa.PrivateKey
}

type PrimaryPolynomial struct {
	coeff     []big.Int
	threshold int
}

type PrimaryShares struct {
	Index int
	Value big.Int
}

type Point struct {
	x *big.Int
	y *big.Int
}

type DLEQProof struct {
	c  big.Int
	r  big.Int
	vG Point
	vH Point
	xG Point
	xH Point
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
	// scalar to the power of this is like square root, eg. y^sqRoot = y^0.5 (if it exists)
	sqRoot = fromHex("3fffffffffffffffffffffffffffffffffffffffffffffffffffffffbfffff0c")
	G      = Point{x: s.Gx, y: s.Gy}
	H      = hashToPoint(G.x.Bytes())
)

func Keccak256(data ...[]byte) []byte {
	d := sha3.NewKeccak256()
	for _, b := range data {
		d.Write(b)
	}
	return d.Sum(nil)
}

func hashToPoint(data []byte) Point {
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
			return Point{x: x, y: y}
		} else {
			x.Add(x, big.NewInt(1))
		}
	}
}

func TestHash(test *testing.T) {
	res := hashToPoint([]byte("this is a random message"))
	fmt.Println(res.x)
	fmt.Println(res.y)
	assert.True(test, s.IsOnCurve(res.x, res.y))
}

func assertEqual(t *testing.T, a interface{}, b interface{}) {
	if a == b {
		return
	}
	// debug.PrintStack()
	t.Errorf("Received %v (type %v), expected %v (type %v)", a, reflect.TypeOf(a), b, reflect.TypeOf(b))
}

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
func polyEval(polynomial PrimaryPolynomial, x int) big.Int { // get private share
	xi := new(big.Int).SetInt64(int64(x))
	sum := new(big.Int) //additive identity of curve = 0??? TODO: CHECK PLS
	fmt.Println("x", x)
	for i := polynomial.threshold - 1; i >= 0; i-- {
		fmt.Println("i: ", i)
		sum.Mul(sum, xi)
		sum.Add(sum, &polynomial.coeff[i])
	}
	sum.Mod(sum, fieldOrder)
	return *sum
}

func getShares(polynomial PrimaryPolynomial, n int) []big.Int {
	shares := make([]big.Int, n)
	for i := range shares {
		shares[i] = polyEval(polynomial, i+1)
	}
	return shares
}

// Commit creates a public commitment polynomial for the given base point b or
// the standard base if b == nil.
func getCommit(polynomial PrimaryPolynomial, threshold int, H Point) []Point {
	commits := make([]Point, threshold)
	for i := range commits {
		tmpx, tmpy := s.ScalarBaseMult(polynomial.coeff[i].Bytes())
		x, y := s.ScalarMult(tmpx, tmpy, H.Bytes())
		commits[i] = Point{x: x, y: y}
		tmpx, tmpy = s.ScalarBaseMult(polynomial.coeff[i].Bytes())
		x, y := s.Add(tmpx, tmpy, &H.x, &H.y)
		commits[i] = Point{x: *x, y: *y}
	}
	return commits
}

// NewDLEQProof computes a new NIZK dlog-equality proof for the scalar x with
// respect to base points G and H. It therefore randomly selects a commitment v
// and then computes the challenge c = H(xG,xH,vG,vH) and response r = v - cx.
// Besides the proof, this function also returns the encrypted base points xG
// and xH.
// func NewDLEQProof(suite Suite, G kyber.Point, H kyber.Point, x kyber.Scalar) (proof *Proof, xG kyber.Point, xH kyber.Point, err error) {
// 	// Encrypt base points with secret
// 	xG = suite.Point().Mul(x, G)
// 	xH = suite.Point().Mul(x, H)

// 	// Commitment
// 	v := suite.Scalar().Pick(suite.RandomStream())
// 	vG := suite.Point().Mul(v, G)
// 	vH := suite.Point().Mul(v, H)

// 	// Challenge
// 	h := suite.Hash()
// 	xG.MarshalTo(h)
// 	xH.MarshalTo(h)
// 	vG.MarshalTo(h)
// 	vH.MarshalTo(h)
// 	cb := h.Sum(nil)
// 	c := suite.Scalar().Pick(suite.XOF(cb))

// 	// Response
// 	r := suite.Scalar()
// 	r.Mul(x, c).Sub(v, r)

// 	return &Proof{c, r, vG, vH}, xG, xH, nil
// }

func createDlEQProof(secret big.Int, H Point) *DLEQProof {
	//Encrypt bbase points with secret
	x, y := s.ScalarBaseMult(secret.Bytes())
	xG := Point{x: x, y: y}
	x2, y2 := s.Add(&xG.x, &xG.y, &H.x, &H.y)
	xH := Point{x: *x2, y: *y2}

	// Commitment
	v := randomBigInt()
	x3, y3 := s.ScalarBaseMult(v.Bytes())
	x4, y4 := s.Add(x3, y3, &H.x, &H.y)
	vG := Point{x: *x3, y: *y3}
	vH := Point{x: *x4, y: *y4}

	//Concat hashing bytes
	cb := make([]byte, 0)
	for _, element := range [4]Point{xG, xH, vG, vH} {
		cb = append(cb[:], element.x.Bytes()...)
		cb = append(cb[:], element.y.Bytes()...)
	}

	//hash
	hashed := Keccak256(cb)
	c := new(big.Int).SetBytes(hashed)

	//response
	r := new(big.Int)
	r.Mul(c, &secret)
	r.Mod(r, fieldOrder)
	r.Sub(v, r) //do we need to mod here?

	return &DLEQProof{*c, *r, vG, vH, xG, xH}
}

func encShares(nodes []NodeList, secret big.Int, threshold int, H Point) {
	n := len(nodes)
	encryptedShares := make([]big.Int, n) // TODO: use struct for shares
	// Create secret sharing polynomial
	coeff := make([]big.Int, threshold)
	coeff[0] = secret                //assign secret as coeff of x^0
	for i := 1; i < threshold; i++ { //randomly choose coeffs
		coeff[i] = *randomBigInt()
	}
	polynomial := PrimaryPolynomial{coeff, threshold}

	// determine shares for polynomial with respect to basis H
	shares := getShares(polynomial, n)

	//committing Yi and proof
	commits := getCommit(polynomial, threshold, H)

	// Create NIZK discrete-logarithm equality proofs
	fmt.Println(encryptedShares, shares, commits)

}

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

}

func TestPVSS(test *testing.T) {
	nodeList := createRandomNodes(10)
	secret := randomBigInt()
	fmt.Println(len(nodeList))
	fmt.Println("ENCRYPTING SHARES ----------------------------------")
	encShares(nodeList, *secret, 3, Point{*s.Gx, *s.Gy})
}
