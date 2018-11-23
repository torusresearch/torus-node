package dkgnode

import (
	"math/big"
	"reflect"
	"testing"

	"github.com/ethereum/go-ethereum/rlp"
	"github.com/stretchr/testify/assert"
)

type rlpTest struct {
	Index uint
	Str   string
	Byt   []byte
	In    innerStruct
}

type innerStruct struct {
	BigI big.Int
	Some string
}

type testMsg struct {
	Msg string
}

func TestRLP(t *testing.T) {
	testMessage := rlpTest{1, "heyo", []byte("blubblu"), innerStruct{*new(big.Int), "biging"}}
	t.Log("Initial message: ", testMessage)
	byt, err := rlp.EncodeToBytes(testMessage)
	if err != nil {
		t.Log(err)
	}
	t.Log(byt)

	var receivedMsg rlpTest
	err = rlp.DecodeBytes(byt, &receivedMsg)
	// err = rlp.Decode(bytes.NewReader(byt), &receivedMsg)
	t.Log(receivedMsg)
	assert.True(t, reflect.DeepEqual(testMessage, receivedMsg))
}

//Needs tendermint node running on 26657
//TODO: fix tests on new interface, Set up tendermint testin environment

// func TestBroadcastRLP(t *testing.T) {
// 	// BftURI := "http://localhost:26657/"
// 	go RunABCIServer()

// 	pls := NewBftRPC("tcp://localhost:26657")

// 	time.Sleep(5 * time.Second) // cater for server setting up

// 	testMessage := testMsg{"heyo"}
// 	t.Log("Initial message: ", testMessage)
// 	byt, err := rlp.EncodeToBytes(testMessage)
// 	if err != nil {
// 		t.Log(err)
// 	}
// 	t.Log(byt)

// 	hash, err := pls.Broadcast(byt)
// 	if err != nil {
// 		t.Log(err)
// 	}
// 	t.Log("RESPONSE: ", hash)

// 	time.Sleep(5 * time.Second) // cater for server setting up

// 	data, err := pls.Retrieve(hash.Bytes())
// 	if err != nil {
// 		t.Log(err)
// 	}
// 	t.Log("RETRIEVE: ", data)

// }
