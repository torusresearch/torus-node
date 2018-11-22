package dkgnode

import (
	"errors"
	"fmt"

	"github.com/YZhenY/torus/common"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/tendermint/tendermint/rpc/client"
)

type BFTTx interface {
	//Create byte type and append RLP encoded tx
	PrepareBFTTx() ([]byte, error)
	//Decode byte type and RLP into struct
	DecodeBFTTx([]byte) error
}

type BftRPC struct {
	*client.HTTP
}

type PubPolyBFTTx struct {
	PubPoly    []common.Point
	Epoch      uint
	ShareIndex uint
}

func (tx PubPolyBFTTx) PrepareBFTTx() ([]byte, error) {
	//type byte
	txType := make([]byte, 1)
	txType[0] = byte(uint8(1))

	data, err := rlp.EncodeToBytes(tx)
	if err != nil {
		return nil, err
	}
	preparedMsg := append(txType[:], data[:]...)
	return preparedMsg, nil
}

func (tx *PubPolyBFTTx) DecodeBFTTx(data []byte) error {
	err := rlp.DecodeBytes(data[1:], tx)
	if err != nil {
		return err
	}
	return nil
}

//BroadcastTxSync Wrapper (input should be RLP encoded) to tendermint.
//All transactions are appended to a torus signature hexbytes(mug00 + versionNo)
// e.g mug00 => 6d75673030
func (bftrpc BftRPC) Broadcast(tx BFTTx) (*common.Hash, error) {
	// prepare transaction with type and rlp encoding
	preparedTx, err := tx.PrepareBFTTx()
	if err != nil {
		return nil, err
	}

	//adding mug00
	//TODO: make version configurable
	msg := append([]byte("mug00")[:], preparedTx[:]...)
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

//Retrieves tx from the bft and gives back results. Takes off the donut
//TODO: this might be a tad redundent a function, to just use innate tendermint functions?
func (bftrpc BftRPC) Retrieve(hash []byte, txStruct BFTTx) (err error) {
	// fmt.Println("WE ARE RETRIEVING")
	result, err := bftrpc.Tx(hash, false)
	if err != nil {
		return err
	}
	fmt.Println("WE ARE RETRIEVING", result)
	if result.TxResult.Code != 0 {
		fmt.Println("Transaction not accepted", result.TxResult.Code)
	}

	err = (txStruct).DecodeBFTTx(result.Tx[len([]byte("mug00")):])
	if err != nil {
		return err
	}

	return nil
}

func NewBftRPC(uri string) *BftRPC {
	//TODO: keep server connection around for logging??
	//commented out for testing purposes
	// go tmabci.RunABCIServer()

	bftClient := client.NewHTTP(uri, "/websocket")
	return &BftRPC{
		bftClient,
	}
}

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
