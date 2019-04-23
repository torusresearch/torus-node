package db

import (
	"crypto/rand"
	"io/ioutil"
	"math/big"
	"os"
	"reflect"
	"testing"

	"github.com/torusresearch/torus-public/keygen"
)

func randBigInt() *big.Int {
	var prime1, _ = new(big.Int).SetString("21888242871839275222246405745257275088548364400416034343698204186575808495617", 10)
	// Generate random numbers in range [0..prime1]
	// Ignore error values
	x, _ := rand.Int(rand.Reader, prime1)

	return x
}

func randBigIntArray(n int) []big.Int {
	var arr []big.Int

	for i := 0; i < n; i++ {
		arr = append(arr, *randBigInt())
	}

	return arr
}

func randBigF(n, m int) [][]big.Int {
	var arr [][]big.Int

	for i := 0; i < m; i++ {
		arr = append(arr, randBigIntArray(n))
	}

	return arr
}
func randomKEYGENSecret() *keygen.KEYGENSecrets {
	return &keygen.KEYGENSecrets{
		Secret: *randBigInt(),
		F:      randBigF(10, 10),
		Fprime: randBigF(10, 10),
	}
}

func TestStoreAndRetrieve(t *testing.T) {
	tmpDir, _ := ioutil.TempDir("", "testdb")
	defer os.Remove(tmpDir)

	db, err := NewTorusLDB(tmpDir)
	if err != nil {
		t.Fatal(err.Error())
		return
	}

	keyIndex := randBigInt()
	randomKeygen := randomKEYGENSecret()

	err = db.StoreKEYGENSecret(keyIndex, randomKeygen)
	if err != nil {
		t.Fatal(err)
	}

	retrievedKeygen, err := db.RetrieveKEYGENSecret(keyIndex)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(randomKeygen, retrievedKeygen) {
		t.Fatal("received different keygen values!")
	}

}

func BenchmarkStores(b *testing.B) {
	b.StopTimer()
	internal := map[int]*keygen.KEYGENSecrets{}
	bigKeys := map[int]*big.Int{}

	for i := 0; i < b.N; i++ {
		internal[i] = randomKEYGENSecret()
		bigKeys[i] = randBigInt()
	}

	tmpDir, _ := ioutil.TempDir("", "testdb")
	defer os.Remove(tmpDir)

	db, err := NewTorusLDB(tmpDir)
	if err != nil {
		b.Fatal(err.Error())
		return
	}

	b.StartTimer()

	for i := 0; i < b.N; i++ {
		idx := bigKeys[i]
		secret := internal[i]
		db.StoreKEYGENSecret(idx, secret)
	}
}
