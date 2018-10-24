package pvss

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"math/big"

	"../pkcs7"
	"github.com/decred/dcrd/dcrec/secp256k1"
	"github.com/ethereum/go-ethereum/crypto/sha3"
)

type NodeList struct {
	Nodes []Point
}

type SigncryptedOutput struct {
	NodePubKey       Point
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

func AESencrypt(key []byte, plainText []byte) (c *[]byte, err error) {
	//Following PKCS#7 described in RFC 5652 for padding key

	key, err = pkcs7.pkcs7Pad(key, 32)
	if err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return
	}

	//IV needs to be unique, but doesn't have to be secure.
	//It's common to put it at the beginning of the ciphertext.
	cipherText := make([]byte, aes.BlockSize+len(plainText))
	iv := cipherText[:aes.BlockSize]
	if _, err = io.ReadFull(rand.Reader, iv); err != nil {
		return
	}

	stream := cipher.NewCFBEncrypter(block, iv)
	stream.XORKeyStream(cipherText[aes.BlockSize:], plainText)

	return &cipherText, nil
}

func AESdecrypt(key []byte, cipherText []byte) (message *[]byte, err error) {
	//Following PKCS#7 described in RFC 5652 for padding key

	key, err = pkcs7.pkcs7Pad(key, 32)
	if err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return
	}

	if len(cipherText) < aes.BlockSize {
		err = errors.New("Ciphertext block size is too short!")
		return
	}

	//IV needs to be unique, but doesn't have to be secure.
	//It's common to put it at the beginning of the ciphertext.
	iv := cipherText[:aes.BlockSize]
	cipherText = cipherText[aes.BlockSize:]

	stream := cipher.NewCFBDecrypter(block, iv)
	// XORKeyStream can work in-place if the two arguments are the same.
	stream.XORKeyStream(cipherText, cipherText)

	message = &cipherText
	return message, nil
}

func signcryptShare(nodePubKey Point, share big.Int, privKey big.Int) (*Signcryption, error) {
	// Commitment
	r := randomBigInt()
	rG := pt(s.ScalarBaseMult(r.Bytes()))
	rU := pt(s.ScalarMult(&nodePubKey.x, &nodePubKey.y, r.Bytes()))

	//encrypt with AES
	ciphertext, err := AESencrypt(rU.x.Bytes(), share.Bytes())
	if err != nil {
		return nil, err
	}

	//Concat hashing bytes
	cb := share.Bytes()
	cb = append(cb[:], rG.x.Bytes()...)

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
		signcryptedShares[i] = &SigncryptedOutput{nodeList[i], *temp}
	}
	return signcryptedShares, nil
}

func encShares(nodes []Point, secret big.Int, threshold int, privKey big.Int) ([]*SigncryptedOutput, *[]Point, error) {
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

func unsigncryptShare(signcryption Signcryption, privKey big.Int, sendingNodePubKey Point) (*[]byte, error) {
	xR := pt(s.ScalarMult(&signcryption.R.x, &signcryption.R.y, privKey.Bytes()))
	M, err := AESdecrypt(xR.x.Bytes(), signcryption.Ciphertext)
	if err != nil {
		return nil, err
	}

	//Concat hashing bytes
	cb := []byte(*M)
	cb = append(cb[:], signcryption.R.x.Bytes()...)

	//hash h = H(M|r1)
	hashed := Keccak256(cb)
	h := new(big.Int).SetBytes(hashed)
	h.Mod(h, generatorOrder)

	//Verify signcryption
	sG := pt(s.ScalarBaseMult(signcryption.Signature.Bytes()))
	hR := pt(s.ScalarMult(&signcryption.R.x, &signcryption.R.y, h.Bytes()))
	testSendingNodePubKey := pt(s.Add(&sG.x, &sG.y, &hR.x, &hR.y))
	if sendingNodePubKey.x.Cmp(&testSendingNodePubKey.x) != 0 {
		fmt.Println(sendingNodePubKey.x.Cmp(&testSendingNodePubKey.x))
		fmt.Println(sendingNodePubKey)
		fmt.Println(testSendingNodePubKey)
		return nil, errors.New("sending node PK does not register with signcryption")
	}

	return M, nil
}

// DecryptShare first verifies the encrypted share against the encryption
// consistency proof and, if valid, decrypts it and creates a decryption
// consistency proof.
// func decShare(encShareOutputs EncShareOutputs, nodePubKey Point, nodePrivateKey big.Int) (*big.Int, error) {
// 	if err := verifyProof(encShareOutputs.Proof, nodePubKey); err != true {
// 		return nil, errors.New("share failed proof validation")
// 	}
// 	// G := suite.Point().Base()
// 	// V := suite.Point().Mul(suite.Scalar().Inv(x), encShare.S.V) // decryption: x^{-1} * (xS)
// 	invPrivKey := new(big.Int)
// 	invPrivKey.ModInverse(&nodePrivateKey, generatorOrder)
// 	shareGx, shareGy := s.ScalarMult(&encShareOutputs.EncryptedShare.Value.x, &encShareOutputs.EncryptedShare.Value.y, invPrivKey.Bytes())
// 	// g^ share
// 	// ps := &share.PubShare{I: encShare.S.I, V: V}
// 	// P, _, _, err := dleq.NewDLEQProof(suite, G, V, x)
// 	shareG := Point{shareGx, shareGy}
// 	proof := getDlEQProof(nodePrivateKey, shareG)
// 	// if err != nil {
// 	// 	return nil, err
// 	// }
// 	// return &PubVerShare{*ps, *P}, nil
// 	return nil, nil
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
