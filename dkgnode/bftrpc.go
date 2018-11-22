package dkgnode

import (
	"fmt"

	"github.com/YZhenY/torus/bft"
	jsonrpcclient "github.com/ybbus/jsonrpc"
)

type BftRPC struct {
	jsonrpcclient.RPCClient
}

type PubPolyTransaction struct {
	PubPolyProof
	ShareIndex int
	Epoch      int
}

func (tx PubPolyTransaction) ValidatePubPolyTransaction(epoch int, shareIndex int) bool {

	//check if its the right epoch
	if tx.Epoch != epoch {
		return false
	}

	//check for duplicate share indexes from node
	if tx.ShareIndex != shareIndex {
		return false
	}

	//check that ECDSA Signature matches pubpolyarr
	// pubpoly := BytesArrayToPointsArray(tx.PointsBytesArray)

	return true
}

func (bftrpc BftRPC) Broadcast(data []byte) (int, error) {
	//tendermint rpc
	res, err := bftrpc.Call("broadcast_tx_sync", data)
	if err != nil {
		return 0, err
	}
	fmt.Println("TENDERBFT RESPONSE: ", res)
	// var obj bft.BroadcastResult
	// if err := res.GetObject(&obj); err != nil {
	// 	return 0, err
	// }
	return 0, nil
}

func NewBftRPC(uri string) *BftRPC {
	// go tmabci.RunBft()

	rpcClient := jsonrpcclient.NewClient(uri)
	return &BftRPC{
		RPCClient: rpcClient,
	}
}

// func (bftrpc BftRPC) Epoch() (int, error) {
// 	res, err := bftrpc.Call("Epoch", bft.EpochParams{})
// 	if err != nil {
// 		return 0, err
// 	}
// 	var obj bft.EpochResult
// 	if err := res.GetObject(&obj); err != nil {
// 		return 0, err
// 	}
// 	return obj.Epoch, nil
// }

// func (bftrpc BftRPC) SetEpoch(epoch int) (int, error) {
// 	res, err := bftrpc.Call("Epoch", bft.SetEpochParams{Epoch: epoch})
// 	if err != nil {
// 		return 0, err
// 	}
// 	var obj bft.SetEpochResult
// 	if err := res.GetObject(&obj); err != nil {
// 		return 0, err
// 	}
// 	return obj.Epoch, nil
// }

// func (bftrpc BftRPC) Broadcast(data []byte) (int, error) {
// 	stringData := string(data[:])
// 	res, err := bftrpc.Call("Broadcast", bft.BroadcastParams{Data: stringData, Length: len(stringData)})
// 	if err != nil {
// 		return 0, err
// 	}
// 	var obj bft.BroadcastResult
// 	if err := res.GetObject(&obj); err != nil {
// 		return 0, err
// 	}
// 	return obj.Id, nil
// }

func (bftrpc BftRPC) Retrieve(id int) (data []byte, length int, err error) {
	res, err := bftrpc.Call("Retrieve", bft.RetrieveParams{Id: id})
	if err != nil {
		return nil, 0, err
	}
	var obj bft.RetrieveResult
	if err := res.GetObject(&obj); err != nil {
		return nil, 0, err
	}
	data = []byte(obj.Data)
	length = obj.Length
	return
}
