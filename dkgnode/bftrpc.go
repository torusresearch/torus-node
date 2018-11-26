package dkgnode

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/YZhenY/torus/common"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/tendermint/tendermint/rpc/client"
)

type BftRPC struct {
	*client.HTTP
}

//BFT Transactions are registered under this interface
type BFTTx interface {
	//Create byte type and append RLP encoded tx
	// PrepareBFTTx() ([]byte, error)
	//Decode byte type and RLP into struct
	// DecodeBFTTx([]byte) error
}

type BFTTxWrapper interface {
	//Create byte type and append RLP encoded tx
	PrepareBFTTx() ([]byte, error)
	//Decode byte type and RLP into struct
	DecodeBFTTx([]byte) error
}

type PubPolyBFTTx struct {
	PubPoly    []common.Point
	Epoch      uint
	ShareIndex uint
}

type EpochBFTTx struct {
	EpochNumber uint
}

type DefaultBFTTxWrapper struct {
	BFTTx BFTTx
}

// mapping of name of struct to id
var bftTxs = map[string]byte{
	getType(PubPolyBFTTx{}): byte(1),
	getType(EpochBFTTx{}):   byte(2),
}

func (wrapper DefaultBFTTxWrapper) PrepareBFTTx() ([]byte, error) {
	//type byte
	txType := make([]byte, 1)
	tx := wrapper.BFTTx
	txType[0] = bftTxs[getType(tx)]
	// fmt.Println("BFTTX: ", bftTxs, txType, getType(tx))
	data, err := rlp.EncodeToBytes(tx)
	if err != nil {
		return nil, err
	}
	preparedMsg := append(txType[:], data[:]...)
	return preparedMsg, nil
}

func (wrapper *DefaultBFTTxWrapper) DecodeBFTTx(data []byte) error {
	err := rlp.DecodeBytes(data[1:], wrapper.BFTTx)
	if err != nil {
		return err
	}
	return nil
}

func getType(myvar interface{}) string {
	if t := reflect.TypeOf(myvar); t.Kind() == reflect.Ptr {
		return t.Elem().Name()
	} else {
		fmt.Println(t)
		return t.Name()
	}
}

// func (tx PubPolyBFTTx) PrepareBFTTx() ([]byte, error) {
// 	//type byte
// 	txType := make([]byte, 1)
// 	txType[0] = byte(uint8(1))

// 	data, err := rlp.EncodeToBytes(tx)
// 	if err != nil {
// 		return nil, err
// 	}
// 	preparedMsg := append(txType[:], data[:]...)
// 	return preparedMsg, nil
// }

// func (tx *PubPolyBFTTx) DecodeBFTTx(data []byte) error {
// 	err := rlp.DecodeBytes(data[1:], tx)
// 	if err != nil {
// 		return err
// 	}
// 	return nil
// }

//BroadcastTxSync Wrapper (input should be RLP encoded) to tendermint.
//All transactions are appended to a torus signature hexbytes(mug00 + versionNo)
// e.g mug00 => 6d75673030
func (bftrpc BftRPC) Broadcast(tx DefaultBFTTxWrapper) (*common.Hash, error) {
	// prepare transaction with type and rlp encoding
	preparedTx, err := tx.PrepareBFTTx()
	if err != nil {
		return nil, err
	}

	//adding mug00
	//TODO: make version configurable
	msg := append([]byte("mug00")[:], preparedTx[:]...)
	//tendermint rpc
	response, err := bftrpc.BroadcastTxSync(msg) // TODO: sync vs async?
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
func (bftrpc BftRPC) Retrieve(hash []byte, txStruct BFTTxWrapper) (err error) {
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
