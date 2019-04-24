package dkgnode

import (
	"errors"
	"reflect"

	"github.com/tendermint/tendermint/rpc/client"
	"github.com/torusresearch/bijson"
	"github.com/torusresearch/torus-public/common"
	"github.com/torusresearch/torus-public/keygen"
	"github.com/torusresearch/torus-public/logging"
)

type BftRPC struct {
	client.Client
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

type KeyGenShareBFTTx struct {
	SigncryptedMessage
}

type DefaultBFTTxWrapper struct {
	BFTTx BFTTx
}

type AssignmentBFTTx struct {
	Email string
	Epoch uint //implemented to allow retries for assignments on the bft
}

type StatusBFTTx struct {
	// TODO: implement signed message from node
	StatusType  string
	StatusValue string
	Epoch       uint
	FromPubKeyX string
	FromPubKeyY string
	Data        []byte
}

type ValidatorUpdateBFTTx struct {
	ValidatorPubKey []common.Point
	ValidatorPower  []uint
}

// mapping of name of struct to id
var bftTxs = map[string]byte{
	getType(PubPolyBFTTx{}):             byte(1),
	getType(KeyGenShareBFTTx{}):         byte(3),
	getType(AssignmentBFTTx{}):          byte(4),
	getType(StatusBFTTx{}):              byte(5),
	getType(ValidatorUpdateBFTTx{}):     byte(6),
	getType(BFTKeygenMsg{}):             byte(7),
	getType(keygen.KEYGENDKGComplete{}): byte(8),
}

func (wrapper DefaultBFTTxWrapper) PrepareBFTTx() ([]byte, error) {
	//type byte
	txType := make([]byte, 1)
	tx := wrapper.BFTTx
	txType[0] = bftTxs[getType(tx)]
	data, err := bijson.Marshal(tx)
	if err != nil {
		return nil, err
	}
	preparedMsg := append(txType[:], data[:]...)
	return preparedMsg, nil
}

func (wrapper *DefaultBFTTxWrapper) DecodeBFTTx(data []byte) error {
	err := bijson.Unmarshal(data[1:], &wrapper.BFTTx)
	if err != nil {
		return err
	}
	return nil
}

func getType(myvar interface{}) string {
	if t := reflect.TypeOf(myvar); t.Kind() == reflect.Ptr {
		return t.Elem().Name()
	} else {
		return t.Name()
	}
}

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
	logging.Debugf("TENDERBFT RESPONSE: %s", response)
	if response.Code != 0 {
		return nil, errors.New("Could not broadcast, ErrorCode: " + string(response.Code))
	}

	return &common.Hash{response.Hash.Bytes()}, nil
}

//Retrieves tx from the bft and gives back results. Takes off the donut
func (bftrpc BftRPC) Retrieve(hash []byte, txStruct BFTTxWrapper) (err error) {
	result, err := bftrpc.Tx(hash, false)
	if err != nil {
		return err
	}
	logging.Debugf("WE ARE RETRIEVING %s", result)
	if result.TxResult.Code != 0 {
		logging.Debugf("Transaction not accepted %s", result.TxResult.Code)
	}

	err = (txStruct).DecodeBFTTx(result.Tx[len([]byte("mug00")):])
	if err != nil {
		return err
	}

	return nil
}
