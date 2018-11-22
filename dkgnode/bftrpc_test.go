package dkgnode

import (
	"math/big"
	"testing"
	"time"

	"github.com/YZhenY/torus/tmabci"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/tendermint/tendermint/rpc/client"
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
}

//Needs tendermint node running on 26657
//TODO:Set up tendermint testin environment
func TestBroadcastRLP(t *testing.T) {
	// BftURI := "http://localhost:26657/"

	app := tmabci.NewKVStoreApplication()
	go tmabci.RunBft(app)

	pls := client.NewHTTP("tcp://localhost:26657", "/websocket")

	// client, err := abcicli.NewLocalClient()
	// _, err := abcicli.NewClient("localhost:26657", "grpc", false)

	// if err != nil {
	// 	t.Log(err)
	// }

	time.Sleep(5 * time.Second) // cater for server setting up

	testMessage := Message{"heyo"}
	t.Log("Initial message: ", testMessage)
	byt, err := rlp.EncodeToBytes(testMessage)
	if err != nil {
		t.Log(err)
	}
	t.Log(byt)

	response, err := pls.BroadcastTxSync(byt)
	if err != nil {
		t.Log(err)
	}
	t.Log("RESPONSE: ", response)

	// response, err := ab.DeliverTxSync(byt)
	// if err != nil {
	// 	t.Log(err)
	// }
	// t.Log("RESPONSE: ", response)
	// _, err = rpc.Broadcast(byt)
	// if err != nil {
	// 	t.Log(err)
	// }

	// var receivedMsg rlpTest
	// err = rlp.DecodeBytes(byt, &receivedMsg)
	// t.Log(receivedMsg)

}
