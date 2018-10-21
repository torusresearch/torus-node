package pvss

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"reflect"
	"testing"

	"github.com/decred/dcrd/dcrec/secp256k1"
	"github.com/ethereum/go-ethereum/crypto/sha3"
	"github.com/stretchr/testify/assert"
)

type NodeList struct {
	Nodes []Point
}

type EncShareOutputs struct {
	NodePubKey     Point
	EncryptedShare PublicShare
	Proof          DLEQProof
}

type PrimaryPolynomial struct {
	coeff     []big.Int
	threshold int
}

type PrimaryShare struct {
	Index int
	Value big.Int
}

type PublicShare struct {
	Index int
	Value Point
}

type Point struct {
	x big.Int
	y big.Int
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

func pt(x, y *big.Int) Point {
	return Point{x: *x, y: *y}
}

var (
	s              = secp256k1.S256()
	fieldOrder     = fromHex("fffffffffffffffffffffffffffffffffffffffffffffffffffffffefffffc2f")
	generatorOrder = fromHex("fffffffffffffffffffffffffffffffebaaedce6af48a03bbfd25e8cd0364141")
	// scalar to the power of this is like square root, eg. y^sqRoot = y^0.5 (if it exists)
	sqRoot = fromHex("3fffffffffffffffffffffffffffffffffffffffffffffffffffffffbfffff0c")
	G      = Point{x: *s.Gx, y: *s.Gy}
	H      = hashToPoint(G.x.Bytes())
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
			return &Point{x: *x, y: *y}
		} else {
			x.Add(x, big.NewInt(1))
		}
	}
}

func TestHash(test *testing.T) {
	res := hashToPoint([]byte("this is a random message"))
	fmt.Println(res.x)
	fmt.Println(res.y)
	assert.True(test, s.IsOnCurve(&res.x, &res.y))
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

func createRandomNodes(number int) *NodeList {
	list := new(NodeList)
	for i := 0; i < number; i++ {
		list.Nodes = append(list.Nodes, *hashToPoint(randomBigInt().Bytes()))
	}
	return list
}

// func randomMedInt() *big.Int {
// 	randomInt, _ := rand.Int(rand.Reader, fromHex("3fffffffffffffffffffffffffffffffffffffffffffbfffff0c"))
// 	return randomInt
// }

func randomBigInt() *big.Int {
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

func TestPolyEval(test *testing.T) {
	coeff := make([]big.Int, 5)
	coeff[0] = *big.NewInt(7) //assign secret as coeff of x^0
	for i := 1; i < 5; i++ {  //randomly choose coeffs
		coeff[i] = *big.NewInt(int64(i))
	}
	polynomial := PrimaryPolynomial{coeff, 5}
	assert.Equal(test, polyEval(polynomial, 10).Text(10), "43217")
}

func getShares(polynomial PrimaryPolynomial, n int) []PrimaryShare {
	shares := make([]PrimaryShare, n)
	for i := range shares {
		shares[i] = PrimaryShare{Index: n, Value: *polyEval(polynomial, i+1)}
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
	// fmt.Println(commits[0].x.Text(16), commits[0].y.Text(16), "commit0")
	// fmt.Println(commits[1].x.Text(16), commits[1].y.Text(16), "commit1")
	return commits
}

func TestCommit(test *testing.T) {

	// coeff := make([]big.Int, 2)
	// coeff[0] = *big.NewInt(7) //assign secret as coeff of x^0
	// coeff[1] = *big.NewInt(10)
	// polynomial := PrimaryPolynomial{coeff, 2}

	// polyCommit := getCommit(polynomial)

	// share10 := polyEval(polynomial, 10)
	// assert.Equal(test, share10.Text(10), "107")

	// ten := *big.NewInt(10)

	// sumx := &polyCommit[0].x
	// sumy := &polyCommit[0].y

	// tmpx, tmpy := s.ScalarMult(&polyCommit[1].x, &polyCommit[1].y, ten.Bytes())

	// sumx, _ = s.Add(sumx, sumy, tmpx, tmpy)

	// gmul107x, _ := s.ScalarBaseMult(big.NewInt(107).Bytes())
	// assert.Equal(test, sumx.Text(16), gmul107x.Text(16))

	secret := *randomBigInt()
	polynomial := *generateRandomPolynomial(secret, 11)
	polyCommit := getCommit(polynomial)

	sum := Point{x: polyCommit[0].x, y: polyCommit[0].y}
	var tmp Point

	index := big.NewInt(int64(10))

	for i := 1; i < len(polyCommit); i++ {
		tmp = pt(s.ScalarMult(&polyCommit[i].x, &polyCommit[i].y, new(big.Int).Exp(index, big.NewInt(int64(i)), generatorOrder).Bytes()))
		sum = pt(s.Add(&tmp.x, &tmp.y, &sum.x, &sum.y))
	}

	final := pt(s.ScalarBaseMult(polyEval(polynomial, 10).Bytes()))

	assert.Equal(test, sum.x.Text(16), final.x.Text(16))
	// sumx, sumy := s.Add(sumx, sumy, )

	// secretx := polyCommit[0].x
	// secrety := polyCommit[0].y

	// assert.Equal(test, big.NewInt(int64(10)).Text(10), "10")

	// onex, oney := s.ScalarMult(&polyCommit[1].x, &polyCommit[1].y, big.NewInt(int64(10)).Bytes())

	// gshare10x, gshare10y := s.ScalarBaseMult(big.NewInt(int64(10)).Bytes())
	// sumx, sumy := s.Add(onex, oney, &secretx, &secrety)
	// fmt.Println(sumx.Text(16), sumy.Text(16), gshare10x.Text(16), gshare10y.Text(16))

	// five := big.NewInt(int64(5))
	// for i := 1; i < len(polyCommit); i++ {
	// 	committedPoint := polyCommit[i]
	// 	// eg. when i = 1, 342G
	// 	fivepowi := new(big.Int)
	// 	fivepowi.Exp(five, big.NewInt(int64(i)), fieldOrder)
	// 	tmpx, tmpy := s.ScalarMult(&committedPoint.x, &committedPoint.y, fivepowi.Bytes())
	// 	tmpx, tmpy = s.Add(&sumx, &sumy, tmpx, tmpy)
	// 	sumx = *tmpx
	// 	sumy = *tmpy
	// }
	// sum := Point{x: sumx, y: sumy}
	// gshare5x, gshare5y := s.ScalarBaseMult(share5.Bytes())
	// gshare := Point{x: *gshare5x, y: *gshare5y}
	// assert.Equal(test, sum.x, gshare.x)
	// assert.Equal(test, sum.y, gshare.y)

}

// func TestGetCommit(test *testing.T) {
// 	dummyCoeff := make([]big.Int, 3)
// 	for i := 0; i < 3; i++ {
// 		dummyCoeff[i] = *new(big.Int).SetInt64(int64(i + 1))
// 	}
// 	dummyPolynomial := PrimaryPolynomial{dummyCoeff, len(dummyCoeff)}

// }

// NewDLEQProof computes a new NIZK dlog-equality proof for the scalar x with
// respect to base points G and H. It therefore randomly selects a commitment v
// and then computes the challenge c = H(xG,xH,vG,vH) and response r = v - cx.
// Besides the proof, this function also returns the encrypted base points xG
// and xH.
func getDLEQProof(secret big.Int, nodePubKey Point) *DLEQProof {
	//Encrypt bbase points with secret
	xG := pt(s.ScalarBaseMult(secret.Bytes()))
	xH := pt(s.ScalarMult(&nodePubKey.x, &nodePubKey.y, secret.Bytes()))

	// Commitment
	v := randomBigInt()
	vG := pt(s.ScalarBaseMult(v.Bytes()))
	vH := pt(s.ScalarMult(&nodePubKey.x, &nodePubKey.y, v.Bytes()))

	//Concat hashing bytes
	cb := make([]byte, 0)
	for _, element := range [4]Point{xG, xH, vG, vH} {
		cb = append(cb[:], element.x.Bytes()...)
		cb = append(cb[:], element.y.Bytes()...)
	}

	//hash
	hashed := Keccak256(cb)
	c := new(big.Int).SetBytes(hashed)
	c.Mod(c, generatorOrder)

	//response
	r := new(big.Int)
	r.Mul(c, &secret)
	r.Mod(r, generatorOrder)
	r.Sub(v, r) //do we need to mod here?
	r.Mod(r, generatorOrder)

	return &DLEQProof{*c, *r, vG, vH, xG, xH}
}

func batchGetDLEQProof(nodes []Point, shares []PrimaryShare) []*DLEQProof {
	if len(nodes) != len(shares) {
		return nil
	}
	proofs := make([]*DLEQProof, len(nodes))
	for i := range nodes {
		proofs[i] = getDLEQProof(shares[i].Value, nodes[i])
	}
	return proofs
}

func generateRandomPolynomial(secret big.Int, threshold int) *PrimaryPolynomial {
	// Create secret sharing polynomial
	coeff := make([]big.Int, threshold)
	coeff[0] = secret                //assign secret as coeff of x^0
	for i := 1; i < threshold; i++ { //randomly choose coeffs
		coeff[i] = *randomBigInt()
	}
	return &PrimaryPolynomial{coeff, threshold}
}

func EncShares(nodes []Point, secret big.Int, threshold int) ([]EncShareOutputs, []Point) {
	n := len(nodes)
	encryptedShares := make([]EncShareOutputs, n)

	polynomial := *generateRandomPolynomial(secret, threshold)

	// determine shares for polynomial with respect to basis point
	shares := getShares(polynomial, n)

	//committing to polynomial
	pubPoly := getCommit(polynomial)

	// Create NIZK discrete-logarithm equality proofs
	proofs := batchGetDLEQProof(nodes, shares)

	for i := 0; i < n; i++ {
		ps := PublicShare{Index: i, Value: proofs[i].xH}
		encryptedShares[i] = EncShareOutputs{nodes[i], ps, *proofs[i]}
	}

	return encryptedShares, pubPoly
}

// DecryptShare first verifies the encrypted share against the encryption
// consistency proof and, if valid, decrypts it and creates a decryption
// consistency proof.
func decShare(encShareOutputs EncShareOutputs, nodePubKey Point, nodePrivateKey big.Int) (*big.Int, error) {
	if err := verifyProof(encShareOutputs.Proof, nodePubKey); err != true {
		return nil, errors.New("share failed proof validation")
	}
	// G := suite.Point().Base()
	// V := suite.Point().Mul(suite.Scalar().Inv(x), encShare.S.V) // decryption: x^{-1} * (xS)
	invPrivKey := new(big.Int)
	invPrivKey.ModInverse(&nodePrivateKey, generatorOrder)
	decryptedSharex, decryptedSharey := s.ScalarMult(&encShareOutputs.EncryptedShare.Value.x, &encShareOutputs.EncryptedShare.Value.y, invPrivKey.Bytes())
	// V := s.ScalarMult(encSharexX, encShareY, modInv.Bytes())
	fmt.Println(decryptedSharex, decryptedSharey)
	// ps := &share.PubShare{I: encShare.S.I, V: V}
	// P, _, _, err := dleq.NewDLEQProof(suite, G, V, x)
	// if err != nil {
	// 	return nil, err
	// }
	// return &PubVerShare{*ps, *P}, nil
	return nil, nil
}

// VerifyEncShare checks that the encrypted share sX satisfies
// log_{H}(sH) == log_{X}(sX) where sH is the public commitment computed by
// evaluating the public commitment polynomial at the encrypted share's index i.
// func verifyEncShare(suite Suite, H kyber.Point, X kyber.Point, sH kyber.Point, encShare *PubVerShare) error {
// 	if err := encShare.P.Verify(suite, H, X, sH, encShare.S.V); err != nil {
// 		return errorEncVerification
// 	}
// 	return nil
// }

// Verify examines the validity of the NIZK dlog-equality proof.
// The proof is valid if the following two conditions hold:
//   vG == rG + c(xG)
//   vH == rH + c(xH)
func verifyProof(proof DLEQProof, nodePubKey Point) bool {
	rGx, rGy := s.ScalarBaseMult(proof.r.Bytes())
	rHx, rHy := s.ScalarMult(&nodePubKey.x, &nodePubKey.y, proof.r.Bytes())
	cxGx, cxGy := s.ScalarMult(&proof.xG.x, &proof.xG.y, proof.c.Bytes())
	cxHx, cxHy := s.ScalarMult(&proof.xH.x, &proof.xH.y, proof.c.Bytes())
	ax, ay := s.Add(rGx, rGy, cxGx, cxGy)
	bx, by := s.Add(rHx, rHy, cxHx, cxHy)
	if !(proof.vG.x.Cmp(ax) == 0 && proof.vG.y.Cmp(ay) == 0 && proof.vH.x.Cmp(bx) == 0 && proof.vH.y.Cmp(by) == 0) {
		return false
	}
	return true
}

func TestDLEQ(test *testing.T) {
	nodeList := createRandomNodes(10)
	secret := randomBigInt()
	// fmt.Println("ENCRYPTING SHARES ----------------------------------")
	output, _ := EncShares(nodeList.Nodes, *secret, 3)
	for i := range output {
		assert.True(test, verifyProof(output[i].Proof, output[i].NodePubKey))
	}
	assert.False(test, verifyProof(output[0].Proof, output[1].NodePubKey))
}

func TestPVSS(test *testing.T) {
	nodeList := createRandomNodes(21)
	secret := randomBigInt()
	fmt.Println("ENCRYPTING SHARES ----------------------------------")
	EncShares(nodeList.Nodes, *secret, 11)

}
