package db

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"testing"
)

func TestNewSqliteDB(t *testing.T) {
	tmpFile, _ := ioutil.TempFile("", "testdb")
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	db, err := NewSqliteDB(tmpFile.Name())
	if err != nil {
		t.Fatal(err)
	}

	testKey := int64(123)
	testValue := int64(345)

	err = db.Set(testKey, testValue)

	if err != nil {
		t.Fatal(err)
	}

	returnedVal, _ := db.Get(testKey)
	if returnedVal != testValue {
		t.Fatal(fmt.Sprintf("expected %d, instead got %d", testValue, returnedVal))
	}
}

func BenchmarkRandomReadsWritesSqlite(b *testing.B) {
	b.StopTimer()

	numItems := int64(1000000)
	internal := map[int64]int64{}
	for i := 0; i < int(numItems); i++ {
		internal[int64(i)] = int64(0)
	}
	tmpFile, _ := ioutil.TempFile("", "testdb")
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	db, err := NewSqliteDB(tmpFile.Name())
	if err != nil {
		b.Fatal(err.Error())
		return
	}

	fmt.Println("ok, starting benchmark")
	b.StartTimer()

	for i := 0; i < b.N; i++ {
		// Write something
		{
			idx := (int64(rand.Int()) % numItems)
			internal[idx]++
			val := internal[idx]
			//fmt.Printf("Set %X -> %X\n", idxBytes, returnedVal)

			_ = db.Set(
				idx,
				val,
			)
		}
		// Read something
		{
			idx := (int64(rand.Int()) % numItems)
			val := internal[idx]
			returnedVal, _ := db.Get(idx)
			//fmt.Printf("Get %X -> %X\n", idxBytes, returnedVal)
			if returnedVal != val {
				b.Errorf("Expected %d, got %d", val, returnedVal)
			}

		}
	}

}
