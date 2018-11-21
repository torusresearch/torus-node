package pvss

import (
	"bytes"
	"fmt"
	"math/big"
	"strings"
	"testing"

	"github.com/YZhenY/torus/common"
	"github.com/stretchr/testify/assert"
)

type nodeList struct {
	Nodes []common.Point
}

func createRandomNodes(number int) (*nodeList, []big.Int) {
	list := new(nodeList)
	privateKeys := make([]big.Int, number)
	for i := 0; i < number; i++ {
		pkey := RandomBigInt()
		list.Nodes = append(list.Nodes, common.BigIntToPoint(s.ScalarBaseMult(pkey.Bytes())))
		privateKeys[i] = *pkey
	}
	return list, privateKeys
}

// func randomMedInt() *big.Int {
// 	randomInt, _ := rand.Int(rand.Reader, fromHex("3fffffffffffffffffffffffffffffffffffffffffffbfffff0c"))
// 	return randomInt
// }

func TestHash(test *testing.T) {
	res := hashToPoint([]byte("this is a random message"))
	assert.True(test, s.IsOnCurve(&res.X, &res.Y))
}

func TestPolyEval(test *testing.T) {
	coeff := make([]big.Int, 5)
	coeff[0] = *big.NewInt(7) //assign secret as coeff of x^0
	for i := 1; i < 5; i++ {  //randomly choose coeffs
		coeff[i] = *big.NewInt(int64(i))
	}
	polynomial := common.PrimaryPolynomial{coeff, 5}
	assert.Equal(test, polyEval(polynomial, 10).Text(10), "43217")
}

func TestCommit(test *testing.T) {

	// coeff := make([]big.Int, 2)
	// coeff[0] = *big.NewInt(7) //assign secret as coeff of x^0
	// coeff[1] = *big.NewInt(10)
	// polynomial := common.PrimaryPolynomial{coeff, 2}

	// polyCommit := getCommit(polynomial)

	// share10 := polyEval(polynomial, 10)
	// assert.Equal(test, share10.Text(10), "107")

	// ten := *big.NewInt(10)

	// sumx := &polyCommit[0].X
	// sumy := &polyCommit[0].Y

	// tmpx, tmpy := s.ScalarMult(&polyCommit[1].X, &polyCommit[1].Y, ten.Bytes())

	// sumx, _ = s.Add(sumx, sumy, tmpx, tmpy)

	// gmul107x, _ := s.ScalarBaseMult(big.NewInt(107).Bytes())
	// assert.Equal(test, sumx.Text(16), gmul107x.Text(16))

	secret := *RandomBigInt()
	polynomial := *generateRandomPolynomial(secret, 11)
	polyCommit := getCommit(polynomial)

	sum := common.Point{X: polyCommit[0].X, Y: polyCommit[0].Y}
	var tmp common.Point

	index := big.NewInt(int64(10))

	for i := 1; i < len(polyCommit); i++ {
		tmp = common.BigIntToPoint(s.ScalarMult(&polyCommit[i].X, &polyCommit[i].Y, new(big.Int).Exp(index, big.NewInt(int64(i)), generatorOrder).Bytes()))
		sum = common.BigIntToPoint(s.Add(&tmp.X, &tmp.Y, &sum.X, &sum.Y))
	}

	final := common.BigIntToPoint(s.ScalarBaseMult(polyEval(polynomial, 10).Bytes()))

	assert.Equal(test, sum.X.Text(16), final.X.Text(16))
	// sumx, sumy := s.Add(sumx, sumy, )

	// secretx := polyCommit[0].X
	// secrety := polyCommit[0].Y

	// assert.Equal(test, big.NewInt(int64(10)).Text(10), "10")

	// onex, oney := s.ScalarMult(&polyCommit[1].X, &polyCommit[1].Y, big.NewInt(int64(10)).Bytes())

	// gshare10x, gshare10y := s.ScalarBaseMult(big.NewInt(int64(10)).Bytes())
	// sumx, sumy := s.Add(onex, oney, &secretx, &secrety)
	// fmt.Println(sumx.Text(16), sumy.Text(16), gshare10x.Text(16), gshare10y.Text(16))

	// five := big.NewInt(int64(5))
	// for i := 1; i < len(polyCommit); i++ {
	// 	committedPoint := polyCommit[i]
	// 	// eg. when i = 1, 342G
	// 	fivepowi := new(big.Int)
	// 	fivepowi.Exp(five, big.NewInt(int64(i)), fieldOrder)
	// 	tmpx, tmpy := s.ScalarMult(&committedPoint.X, &committedPoint.Y, fivepowi.Bytes())
	// 	tmpx, tmpy = s.Add(&sumx, &sumy, tmpx, tmpy)
	// 	sumx = *tmpx
	// 	sumy = *tmpy
	// }
	// sum := common.Point{X: sumx, Y: sumy}
	// gshare5x, gshare5y := s.ScalarBaseMult(share5.Bytes())
	// gshare := common.Point{X: *gshare5x, Y: *gshare5y}
	// assert.Equal(test, sum.X, gshare.X)
	// assert.Equal(test, sum.Y, gshare.Y)

}

func TestAES(test *testing.T) {
	key := RandomBigInt().Bytes()
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
	secretShare := RandomBigInt()
	privKeySender := RandomBigInt()
	pubKeySender := common.BigIntToPoint(s.ScalarBaseMult(privKeySender.Bytes()))
	privKeyReceiver := RandomBigInt()
	pubKeyReceiver := common.BigIntToPoint(s.ScalarBaseMult(privKeyReceiver.Bytes()))
	signcryption, err := signcryptShare(pubKeyReceiver, *secretShare, *privKeySender)
	if err != nil {
		fmt.Println(err)
	}
	supposedShare, err := UnsigncryptShare(*signcryption, *privKeyReceiver, pubKeySender)
	if err != nil {
		fmt.Println(err)
	}
	assert.True(test, bytes.Compare(*supposedShare, secretShare.Bytes()) == 0)
}

// func TestDLEQ(test *testing.T) {
// 	nodeList := createRandomNodes(10)
// 	secret := RandomBigInt()
// 	privKey := RandomBigInt()
// 	// fmt.Println("ENCRYPTING SHARES ----------------------------------")
// 	output, _ := CreateAndPrepareShares(nodeList.Nodes, *secret, 3, *privKey)
// 	for i := range output {
// 		assert.True(test, verifyProof(output[i].Proof, output[i].NodePubKey))
// 	}
// 	assert.False(test, verifyProof(output[0].Proof, output[1].NodePubKey))
// }

func TestPVSS(test *testing.T) {
	nodeList, privateKeys := createRandomNodes(20)
	secret := RandomBigInt()
	privKeySender := RandomBigInt()
	pubKeySender := common.BigIntToPoint(s.ScalarBaseMult(privKeySender.Bytes()))

	errorsExist := false
	signcryptedShares, _, err := CreateAndPrepareShares(nodeList.Nodes, *secret, 10, *privKeySender)
	if err != nil {
		fmt.Println(err)
		errorsExist = true
	}
	for i := range signcryptedShares {
		_, err := UnsigncryptShare(signcryptedShares[i].SigncryptedShare, privateKeys[i], pubKeySender)
		if err != nil {
			fmt.Println(err)
			errorsExist = true
		}
	}
	assert.False(test, errorsExist)
}

// func TestLarangeInterpolationNormalNumbers(test *testing.T) {
// 	// polyCoeff := make([]big.Int, 3)
// 	// polyCoeff[0] = *new(big.Int).SetInt64(int64(0))
// 	// polyCoeff[1] = *new(big.Int).SetInt64(int64(1))
// 	// polyCoeff[2] = *new(big.Int).SetInt64(int64(1))
// 	// poly := common.PrimaryPolynomial{polyCoeff, 3}
// 	shares := make([]common.PrimaryShare, 3)
// 	shares[0] = common.PrimaryShare{1, *new(big.Int).SetInt64(int64(2))}
// 	shares[1] = common.PrimaryShare{2, *new(big.Int).SetInt64(int64(6))}
// 	shares[2] = common.PrimaryShare{3, *new(big.Int).SetInt64(int64(12))}
// 	// shares[3] = common.PrimaryShare{4, *new(big.Int).SetInt64(int64(20))}
// 	testX := Lagrange(shares)
// 	// fmt.Println(testX)
// 	assert.True(test, testX.Cmp(new(big.Int).SetInt64(int64(0))) == 0)
// }

func TestLagrangeInterpolation(test *testing.T) {
	nodeList, privateKeys := createRandomNodes(20)
	secret := RandomBigInt()
	privKeySender := RandomBigInt()
	pubKeySender := common.BigIntToPoint(s.ScalarBaseMult(privKeySender.Bytes()))

	errorsExist := false
	signcryptedShares, _, err := CreateAndPrepareShares(nodeList.Nodes, *secret, 11, *privKeySender)
	if err != nil {
		fmt.Println(err)
		errorsExist = true
	}
	decryptedShares := make([]common.PrimaryShare, 11)
	for i := range decryptedShares {
		share, err := UnsigncryptShare(signcryptedShares[i].SigncryptedShare, privateKeys[i], pubKeySender)
		if err != nil {
			fmt.Println(err)
			errorsExist = true
		}
		decryptedShares[i] = common.PrimaryShare{i + 1, *new(big.Int).SetBytes(*share)}
	}
	lagrange := LagrangeElliptic(decryptedShares)

	assert.True(test, secret.Cmp(lagrange) == 0)
	assert.False(test, errorsExist)
}

func TestPedersons(test *testing.T) {
	nodeList, privateKeys := createRandomNodes(21)
	secrets := make([]big.Int, len(nodeList.Nodes))
	errorsExist := false
	allSigncryptedShares := make([][]*common.SigncryptedOutput, len(nodeList.Nodes))
	allPubPoly := make([][]common.Point, len(nodeList.Nodes))
	for i := range nodeList.Nodes {
		signcryptedShares, pubPoly, err := CreateAndPrepareShares(nodeList.Nodes, secrets[i], 11, privateKeys[i])
		allSigncryptedShares[i] = signcryptedShares
		allPubPoly[i] = *pubPoly
		if err != nil {
			fmt.Println(err)
			errorsExist = true
		}
	}
	allDecryptedShares := make([][]big.Int, len(nodeList.Nodes))
	for i := range nodeList.Nodes {
		arrDecryptShares := make([]big.Int, len(nodeList.Nodes))
		for j := range nodeList.Nodes {
			decryptedShare, err := UnsigncryptShare(allSigncryptedShares[j][i].SigncryptedShare, privateKeys[i], nodeList.Nodes[j])
			temp := new(big.Int).SetBytes(*decryptedShare)
			if err != nil {
				fmt.Println(err)
				errorsExist = true
			}
			arrDecryptShares[j] = *temp
		}
		allDecryptedShares[i] = arrDecryptShares
	}
	//form si, points on the polynomial f(z) = r + a1z + a2z^2....
	allSi := make([]common.PrimaryShare, len(nodeList.Nodes))
	for i := range nodeList.Nodes {
		sum := new(big.Int)
		for j := range nodeList.Nodes {
			sum.Add(sum, &allDecryptedShares[i][j])
		}
		sum.Mod(sum, generatorOrder)
		allSi[i] = common.PrimaryShare{i + 1, *sum}
	}

	//form r (and other components) to test
	r := new(big.Int)
	for i := range nodeList.Nodes {
		r.Add(r, &secrets[i])
	}
	r.Mod(r, generatorOrder)
	// rY := common.BigIntToPoint(s.ScalarBaseMult(r.Bytes()))

	testr := LagrangeElliptic(allSi[:11])

	assert.True(test, testr.Cmp(r) == 0)
	assert.False(test, errorsExist)

}
