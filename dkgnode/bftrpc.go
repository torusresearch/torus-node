package dkgnode

import (
	"errors"
	"fmt"

	"github.com/YZhenY/torus/common"
	"github.com/YZhenY/torus/tmabci"
	"github.com/tendermint/tendermint/rpc/client"
)

type BftRPC struct {
	*client.HTTP
}

// type PubPolyTransaction struct {
// 	PubPolyProof
// 	ShareIndex int
// 	Epoch      int
// }

// func (tx PubPolyTransaction) ValidatePubPolyTransaction(epoch int, shareIndex int) bool {

// 	//check if its the right epoch
// 	if tx.Epoch != epoch {
// 		return false
// 	}

// 	//check for duplicate share indexes from node
// 	if tx.ShareIndex != shareIndex {
// 		return false
// 	}

// 	//check that ECDSA Signature matches pubpolyarr
// 	// pubpoly := BytesArrayToPointsArray(tx.PointsBytesArray)

// 	return true
// }

//BroadcastTxSync Wrapper (input should be RLP encoded) to tendermint.
//All transactions are appended to a torus signature hexbytes(mug00 + versionNo)
// e.g mug00 => 6d75673030
func (bftrpc BftRPC) Broadcast(data []byte) (*common.Hash, error) {
	//adding mug00
	//TODO: make version configurable
	msg := append([]byte("mug00")[:], data[:]...)
	//tendermint rpc
	response, err := bftrpc.BroadcastTxSync(msg)
	if err != nil {
		return nil, err
	}
	fmt.Println("TENDERBFT RESPONSE: ", response)
	if response.Code != 0 {
		return nil, errors.New("Could not broadcast, ErrorCode: " + string(response.Code))
	}

	return &common.Hash{response.Hash.Bytes()}, nil
}

func NewBftRPC(uri string) *BftRPC {
	//TODO: keep server connection around for logging??
	go tmabci.RunABCIServer()

	bftClient := client.NewHTTP(uri, "/websocket")
	return &BftRPC{
		bftClient,
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

//Retrieves tx from the bft and gives back results. Takes off the donut
//TODO: this might be a tad redundent a function, to just use innate tendermint functions?
func (bftrpc BftRPC) Retrieve(hash common.Hash) (data []byte, err error) {
	fmt.Println("WE ARE RETRIEVING")
	result, err := bftrpc.Tx(hash.Bytes(), false)
	if err != nil {
		return nil, err
	}
	fmt.Println("WE ARE RETRIEVING2", result)
	if result.TxResult.Code != 0 {
		fmt.Println("Transaction not accepted", result.TxResult.Code)
	}

	return result.Tx[len([]byte("mug00")):], nil
}
