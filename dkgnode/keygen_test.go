package dkgnode

import (
	"testing"
)

func TestKeygenID(t *testing.T) {
	id := getKeygenID(0, 10)
	t.Log(id)
	start, end, err := getStartEndIndexesFromKeygenID(id)
	if err != nil {
		t.Fatal(err)
	}
	if start != 0 || end != 10 {
		t.Fatal("keygen parse did not work", start, end)
	}
}
