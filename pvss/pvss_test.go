package pvss

import (
	"bytes"
	"fmt"
	"math/big"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHash(test *testing.T) {
	res := hashToPoint([]byte("this is a random message"))
	assert.True(test, s.IsOnCurve(&res.x, &res.y))
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

func TestAES(test *testing.T) {
	key := randomBigInt().Bytes()
	encryptMsg, err := AESencrypt(key, []byte("Hello World"))
	if err != nil {
		fmt.Println(err)
	}
	msg, err := AESdecrypt(key, *encryptMsg)
	if err != nil {
		fmt.Println(err)
	}
	assert.True(test, strings.Compare(string("Hello World"), string(*msg)) == 0)
}

func TestSigncryption(test *testing.T) {
	secretShare := randomBigInt()
	privKeySender := randomBigInt()
	pubKeySender := pt(s.ScalarBaseMult(privKeySender.Bytes()))
	privKeyReceiver := randomBigInt()
	pubKeyReceiver := pt(s.ScalarBaseMult(privKeyReceiver.Bytes()))
	signcryption, err := signcryptShare(pubKeyReceiver, *secretShare, *privKeySender)
	if err != nil {
		fmt.Println(err)
	}
	supposedShare, err := unsigncryptionShare(*signcryption, *privKeyReceiver, pubKeySender)
	if err != nil {
		fmt.Println(err)
	}
	assert.True(test, bytes.Compare(*supposedShare, secretShare.Bytes()) == 0)
}

// func TestDLEQ(test *testing.T) {
// 	nodeList := createRandomNodes(10)
// 	secret := randomBigInt()
// 	privKey := randomBigInt()
// 	// fmt.Println("ENCRYPTING SHARES ----------------------------------")
// 	output, _ := EncShares(nodeList.Nodes, *secret, 3, *privKey)
// 	for i := range output {
// 		assert.True(test, verifyProof(output[i].Proof, output[i].NodePubKey))
// 	}
// 	assert.False(test, verifyProof(output[0].Proof, output[1].NodePubKey))
// }

// func TestPVSS(test *testing.T) {
// 	nodeList := createRandomNodes(21)
// 	secret := randomBigInt()
// 	fmt.Println("ENCRYPTING SHARES ----------------------------------")
// 	EncShares(nodeList.Nodes, *secret, 11)

// }
