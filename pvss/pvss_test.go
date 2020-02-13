package pvss

import (
	"bytes"
	"fmt"
	"math/big"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/torusresearch/torus-common/common"
	"github.com/torusresearch/torus-common/secp256k1"
	pcmn "github.com/torusresearch/torus-node/common"
)

type nodeList struct {
	Nodes []pcmn.Node
}

func TestBytes32(t *testing.T) {
	var byteSlice []byte
	if len(bytes32(byteSlice)) != 32 {
		t.Fatal("bytes32 doesnt turn empty byte slice into slice of 32")
	}
}

func createRandomNodes(number int) (*nodeList, []big.Int) {
	list := new(nodeList)
	privateKeys := make([]big.Int, number)
	for i := 0; i < number; i++ {
		pkey := RandomBigInt()
		list.Nodes = append(list.Nodes, pcmn.Node{
			Index:  i + 1,
			PubKey: common.BigIntToPoint(secp256k1.Curve.ScalarBaseMult(pkey.Bytes())),
		})
		privateKeys[i] = *pkey
	}
	return list, privateKeys
}

// func randomMedInt() *big.Int {
// 	randomInt, _ := rand.Int(rand.Reader, common.HexToBigInt("3fffffffffffffffffffffffffffffffffffffffffffbfffff0c"))
// 	return randomInt
// }

func TestHash(test *testing.T) {
	res := secp256k1.HashToPoint([]byte("this is a random message"))
	assert.True(test, secp256k1.Curve.IsOnCurve(&res.X, &res.Y))
}

func TestPolyEval(test *testing.T) {
	coeff := make([]big.Int, 5)
	coeff[0] = *big.NewInt(7) //assign secret as coeff of x^0
	for i := 1; i < 5; i++ {  //randomly choose coeffs
		coeff[i] = *big.NewInt(int64(i))
	}
	polynomial := pcmn.PrimaryPolynomial{Coeff: coeff, Threshold: 5}
	assert.Equal(test, polyEval(polynomial, 10).Text(10), "43217")
}

func TestCommit(test *testing.T) {

	// coeff := make([]big.Int, 2)
	// coeff[0] = *big.NewInt(7) //assign secret as coeff of x^0
	// coeff[1] = *big.NewInt(10)
	// polynomial := pcmn.PrimaryPolynomial{coeff, 2}

	// polyCommit := getCommit(polynomial)

	// share10 := polyEval(polynomial, 10)
	// assert.Equal(test, share10.Text(10), "107")

	// ten := *big.NewInt(10)

	// sumx := &polyCommit[0].X
	// sumy := &polyCommit[0].Y

	// tmpx, tmpy := secp256k1.Curve.ScalarMult(&polyCommit[1].X, &polyCommit[1].Y, ten.Bytes())

	// sumx, _ = secp256k1.Curve.Add(sumx, sumy, tmpx, tmpy)

	// gmul107x, _ := secp256k1.Curve.ScalarBaseMult(big.NewInt(107).Bytes())
	// assert.Equal(test, sumx.Text(16), gmul107x.Text(16))

	secret := *RandomBigInt()
	polynomial := *generateRandomZeroPolynomial(secret, 11)
	polyCommit := GetCommit(polynomial)

	sum := common.Point{X: polyCommit[0].X, Y: polyCommit[0].Y}
	var tmp common.Point

	index := big.NewInt(int64(10))

	for i := 1; i < len(polyCommit); i++ {
		tmp = common.BigIntToPoint(secp256k1.Curve.ScalarMult(&polyCommit[i].X, &polyCommit[i].Y, new(big.Int).Exp(index, big.NewInt(int64(i)), secp256k1.GeneratorOrder).Bytes()))
		sum = common.BigIntToPoint(secp256k1.Curve.Add(&tmp.X, &tmp.Y, &sum.X, &sum.Y))
	}

	final := common.BigIntToPoint(secp256k1.Curve.ScalarBaseMult(polyEval(polynomial, 10).Bytes()))

	assert.Equal(test, sum.X.Text(16), final.X.Text(16))
	// sumx, sumy := secp256k1.Curve.Add(sumx, sumy, )

	// secretx := polyCommit[0].X
	// secrety := polyCommit[0].Y

	// assert.Equal(test, big.NewInt(int64(10)).Text(10), "10")

	// onex, oney := secp256k1.Curve.ScalarMult(&polyCommit[1].X, &polyCommit[1].Y, big.NewInt(int64(10)).Bytes())

	// gshare10x, gshare10y := secp256k1.Curve.ScalarBaseMult(big.NewInt(int64(10)).Bytes())
	// sumx, sumy := secp256k1.Curve.Add(onex, oney, &secretx, &secrety)
	// fmt.Println(sumx.Text(16), sumy.Text(16), gshare10x.Text(16), gshare10y.Text(16))

	// five := big.NewInt(int64(5))
	// for i := 1; i < len(polyCommit); i++ {
	// 	committedPoint := polyCommit[i]
	// 	// eg. when i = 1, 342G
	// 	fivepowi := new(big.Int)
	// 	fivepowi.Exp(five, big.NewInt(int64(i)), secp256k1.FieldOrder)
	// 	tmpx, tmpy := secp256k1.Curve.ScalarMult(&committedPoint.X, &committedPoint.Y, fivepowi.Bytes())
	// 	tmpx, tmpy = secp256k1.Curve.Add(&sumx, &sumy, tmpx, tmpy)
	// 	sumx = *tmpx
	// 	sumy = *tmpy
	// }
	// sum := common.Point{X: sumx, Y: sumy}
	// gshare5x, gshare5y := secp256k1.Curve.ScalarBaseMult(share5.Bytes())
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
	pubKeySender := common.BigIntToPoint(secp256k1.Curve.ScalarBaseMult(privKeySender.Bytes()))
	privKeyReceiver := RandomBigInt()
	pubKeyReceiver := common.BigIntToPoint(secp256k1.Curve.ScalarBaseMult(privKeyReceiver.Bytes()))
	signcryption, err := signcryptShare(pubKeyReceiver, *secretShare, *privKeySender)
	if err != nil {
		fmt.Println(err)
	}
	supposedShare, err := UnsigncryptShare(*signcryption, *privKeyReceiver, pubKeySender)
	if err != nil {
		fmt.Println(err)
	}
	assert.True(test, bytes.Equal(*supposedShare, secretShare.Bytes()))
}

func TestPVSS(test *testing.T) {
	nodeList, privateKeys := createRandomNodes(20)
	secret := RandomBigInt()
	privKeySender := RandomBigInt()
	pubKeySender := common.BigIntToPoint(secp256k1.Curve.ScalarBaseMult(privKeySender.Bytes()))

	errorsExist := false
	signcryptedShares, _, _, err := CreateAndPrepareShares(nodeList.Nodes, *secret, 10, *privKeySender)
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
// 	// poly := pcmn.PrimaryPolynomial{polyCoeff, 3}
// 	shares := make([]pcmn.PrimaryShare, 3)
// 	shares[0] = pcmn.PrimaryShare{1, *new(big.Int).SetInt64(int64(2))}
// 	shares[1] = pcmn.PrimaryShare{2, *new(big.Int).SetInt64(int64(6))}
// 	shares[2] = pcmn.PrimaryShare{3, *new(big.Int).SetInt64(int64(12))}
// 	// shares[3] = pcmn.PrimaryShare{4, *new(big.Int).SetInt64(int64(20))}
// 	testX := Lagrange(shares)
// 	// fmt.Println(testX)
// 	assert.True(test, testX.Cmp(new(big.Int).SetInt64(int64(0))) == 0)
// }

func TestLagrangeInterpolatePolynomial(test *testing.T) {
	nodeList, privateKeys := createRandomNodes(16)
	secret := RandomBigInt()
	privKeySender := RandomBigInt()
	pubKeySender := common.BigIntToPoint(secp256k1.Curve.ScalarBaseMult(privKeySender.Bytes()))

	errorsExist := false
	signcryptedShares, _, originalPoly, err := CreateAndPrepareShares(nodeList.Nodes, *secret, 11, *privKeySender)
	if err != nil {
		fmt.Println(err)
		errorsExist = true
	}
	decryptedShares := make([]pcmn.PrimaryShare, 11)
	for i := range decryptedShares {
		share, err := UnsigncryptShare(signcryptedShares[i].SigncryptedShare, privateKeys[i], pubKeySender)
		if err != nil {
			fmt.Println(err)
			errorsExist = true
		}
		decryptedShares[i] = pcmn.PrimaryShare{Index: i + 1, Value: *new(big.Int).SetBytes(*share)}
	}
	lagrange := LagrangeScalar(decryptedShares, 0)
	assert.True(test, secret.Cmp(lagrange) == 0)
	assert.False(test, errorsExist)
	points := make([]common.Point, 11)
	for i := 0; i < len(decryptedShares); i++ {
		points[i] = common.Point{
			X: *big.NewInt(int64(decryptedShares[i].Index)),
			Y: decryptedShares[i].Value,
		}
	}
	recoveredPoly := LagrangeInterpolatePolynomial(points)
	for i := 0; i < len(originalPoly.Coeff); i++ {
		origCoeff := originalPoly.Coeff[i]
		assert.Equal(test, recoveredPoly[i].Text(16), origCoeff.Text(16))
	}
}

func TestLagrangeInterpolation(test *testing.T) {
	nodeList, privateKeys := createRandomNodes(16)
	secret := RandomBigInt()
	privKeySender := RandomBigInt()
	pubKeySender := common.BigIntToPoint(secp256k1.Curve.ScalarBaseMult(privKeySender.Bytes()))

	errorsExist := false
	signcryptedShares, _, _, err := CreateAndPrepareShares(nodeList.Nodes, *secret, 11, *privKeySender)
	if err != nil {
		fmt.Println(err)
		errorsExist = true
	}
	decryptedShares := make([]pcmn.PrimaryShare, 16)
	for i := range decryptedShares {
		share, err := UnsigncryptShare(signcryptedShares[i].SigncryptedShare, privateKeys[i], pubKeySender)
		if err != nil {
			fmt.Println(err)
			errorsExist = true
		}
		decryptedShares[i] = pcmn.PrimaryShare{Index: i + 1, Value: *new(big.Int).SetBytes(*share)}
	}
	lagrange := LagrangeScalar(decryptedShares, 0)
	assert.True(test, secret.Cmp(lagrange) == 0)
	lagrange = LagrangeScalar(decryptedShares[0:11], 0)
	assert.True(test, secret.Cmp(lagrange) == 0)
	lagrange = LagrangeScalar(decryptedShares[0:12], 0)
	assert.True(test, secret.Cmp(lagrange) == 0)
	lagrange = LagrangeScalar(decryptedShares[2:15], 0)
	assert.True(test, secret.Cmp(lagrange) == 0)
	assert.False(test, errorsExist)
}

func TestECDSA(test *testing.T) {
	testStr := "test-string"
	privKey := big.NewInt(int64(1234))
	sig := ECDSASign(testStr, privKey)
	pubKey := common.BigIntToPoint(secp256k1.Curve.ScalarBaseMult(privKey.Bytes()))
	assert.True(test, ECDSAVerify(testStr, &pubKey, sig))
}

func TestPedersons(test *testing.T) {
	nodeList, privateKeys := createRandomNodes(21)
	secrets := make([]big.Int, len(nodeList.Nodes))
	errorsExist := false
	allSigncryptedShares := make([][]*pcmn.SigncryptedOutput, len(nodeList.Nodes))
	allPubPoly := make([][]common.Point, len(nodeList.Nodes))
	for i := range nodeList.Nodes {
		signcryptedShares, pubPoly, _, err := CreateAndPrepareShares(nodeList.Nodes, secrets[i], 11, privateKeys[i])
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
			decryptedShare, err := UnsigncryptShare(allSigncryptedShares[j][i].SigncryptedShare, privateKeys[i], nodeList.Nodes[j].PubKey)
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
	allSi := make([]pcmn.PrimaryShare, len(nodeList.Nodes))
	for i := range nodeList.Nodes {
		sum := new(big.Int)
		for j := range nodeList.Nodes {
			sum.Add(sum, &allDecryptedShares[i][j])
		}
		sum.Mod(sum, secp256k1.GeneratorOrder)
		allSi[i] = pcmn.PrimaryShare{Index: i + 1, Value: *sum}
	}

	//form r (and other components) to test
	r := new(big.Int)
	for i := range nodeList.Nodes {
		r.Add(r, &secrets[i])
	}
	r.Mod(r, secp256k1.GeneratorOrder)
	// rY := common.BigIntToPoint(secp256k1.Curve.ScalarBaseMult(r.Bytes()))

	testr := LagrangeScalar(allSi[:11], 0)

	assert.True(test, testr.Cmp(r) == 0)
	assert.False(test, errorsExist)

}

func TestLagrangeCurvePts(t *testing.T) {
	k := 5
	n := 9
	// generate shares
	origSecret := RandomBigInt()
	origPoly := generateRandomZeroPolynomial(*origSecret, k)
	secretCommitment := common.BigIntToPoint(secp256k1.Curve.ScalarBaseMult(origSecret.Bytes()))
	var shares []big.Int
	for i := 0; i < n; i++ {
		shares = append(shares, *polyEval(*origPoly, i+1))
	}

	var shareCommitments []common.Point
	for _, share := range shares {
		shareCommitments = append(shareCommitments, common.BigIntToPoint(secp256k1.Curve.ScalarBaseMult(share.Bytes())))
	}
	res := LagrangeCurvePts([]int{1, 2, 3, 4, 5}, shareCommitments[0:k])
	assert.Equal(t, res.X.Text(16), secretCommitment.X.Text(16))
}
