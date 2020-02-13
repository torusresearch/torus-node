package dkgnode

import (
	"crypto/rand"
	"errors"
	"fmt"
	"reflect"

	logging "github.com/sirupsen/logrus"
	"github.com/torusresearch/bijson"
	"github.com/torusresearch/tendermint/rpc/client"
	tmtypes "github.com/torusresearch/tendermint/rpc/core/types"
	"github.com/torusresearch/torus-common/common"
	"github.com/torusresearch/torus-common/secp256k1"
	pcmn "github.com/torusresearch/torus-node/common"
	"github.com/torusresearch/torus-node/config"
	"github.com/torusresearch/torus-node/dealer"
	"github.com/torusresearch/torus-node/keygennofsm"
	"github.com/torusresearch/torus-node/mapping"
	"github.com/torusresearch/torus-node/msgqueue"
	"github.com/torusresearch/torus-node/pss"
	"github.com/torusresearch/torus-node/telemetry"
)

type BFTRPC struct {
	client.Client
	BftMsgQueue *msgqueue.MessageQueue
}

type BFTTxWrapper interface {
	//Create byte type and attach nonce
	PrepareBFTTx() ([]byte, error)
	//Decode byte type
	DecodeBFTTx([]byte) error
	GetSerializedBody() []byte
}

type DefaultBFTTxWrapper struct {
	BFTTx     []byte       `json:"bft_tx,omitempty"`
	Nonce     uint32       `json:"nonce,omitempty"`
	PubKey    common.Point `json:"pub_key,omitempty"`
	MsgType   byte         `json:"msg_type,omitempty"`
	Signature []byte       `json:"signature,omitempty"`
}

type AssignmentBFTTx struct {
	Verifier   string
	VerifierID string
}

// mapping of name of struct to id
var bftTxs = map[string]byte{
	getType(AssignmentBFTTx{}):           byte(1),
	getType(keygennofsm.KeygenMessage{}): byte(2),
	getType(pss.PSSMessage{}):            byte(3),
	getType(mapping.MappingMessage{}):    byte(4),
	getType(dealer.Message{}):            byte(5),
}

func (wrapper *DefaultBFTTxWrapper) PrepareBFTTx(bftTx interface{}) ([]byte, error) {
	// type byte
	msgType, ok := bftTxs[getType(bftTx)]
	if !ok {
		return nil, fmt.Errorf("Msg type does not exist for BFT: %s ", getType(bftTx))
	}
	wrapper.MsgType = msgType
	nonce, err := rand.Int(rand.Reader, secp256k1.GeneratorOrder)
	if err != nil {
		return nil, fmt.Errorf("Could not generate random number")
	}
	wrapper.Nonce = uint32(nonce.Int64())
	pk := abciServiceLibrary.EthereumMethods().GetSelfPublicKey()
	wrapper.PubKey.X = pk.X
	wrapper.PubKey.Y = pk.Y
	bftRaw, err := bijson.Marshal(bftTx)
	if err != nil {
		return nil, err
	}
	wrapper.BFTTx = bftRaw

	// sign message data
	data := wrapper.GetSerializedBody()
	wrapper.Signature = abciServiceLibrary.EthereumMethods().SelfSignData(data)

	rawMsg, err := bijson.Marshal(wrapper)
	if err != nil {
		return nil, err
	}
	return rawMsg, nil
}

func (wrapper *DefaultBFTTxWrapper) DecodeBFTTx(data []byte) error {
	err := bijson.Unmarshal(data, &wrapper.BFTTx)
	if err != nil {
		return err
	}
	return nil
}

func (wrapper DefaultBFTTxWrapper) GetSerializedBody() []byte {
	wrapper.Signature = nil
	bin, err := bijson.Marshal(wrapper)
	if err != nil {
		logging.Errorf("could not GetSerializedBody bfttx, %v", err)
	}
	return bin
}

func getType(myvar interface{}) string {
	if t := reflect.TypeOf(myvar); t.Kind() == reflect.Ptr {
		return t.Elem().Name()
	} else {
		return t.Name()
	}
}

// BroadcastTxSync Wrapper (input should be bijsoned) to tendermint.
// blocks until message is sent
func (bftrpc BFTRPC) Broadcast(bftTx interface{}) (*pcmn.Hash, error) {

	var wrapper DefaultBFTTxWrapper
	preparedTx, err := wrapper.PrepareBFTTx(bftTx)
	if err != nil {
		return nil, err
	}

	res := bftrpc.BftMsgQueue.Add(preparedTx)
	response, ok := res.(*tmtypes.ResultBroadcastTx)
	if !ok {
		return nil, fmt.Errorf("return type for broadcast was not *tmtypes.ResultBroadcastTx: %v", res)
	}
	logging.Debugf("TENDERBFT RESPONSE code %v : %v", response.Code, response)
	logging.Debugf("TENDERBFT LOG: %s", response.Log)

	if response.Code != 0 {
		return nil, errors.New("Could not broadcast, ErrorCode: " + string(response.Code))
	}

	return &pcmn.Hash{HexBytes: response.Hash.Bytes()}, nil
}

// Retrieves tx from the bft and gives back results.
func (bftrpc BFTRPC) Retrieve(hash []byte, txStruct BFTTxWrapper) (err error) {
	result, err := bftrpc.Tx(hash, false)
	if err != nil {
		return err
	}
	logging.Debugf("WE ARE RETRIEVING %v", result)
	if result.TxResult.Code != 0 {
		logging.Debugf("Transaction not accepted %v", result.TxResult.Code)
	}

	err = (txStruct).DecodeBFTTx(result.Tx)
	if err != nil {
		return err
	}

	return nil
}

func NewBFTRPC(tmclient client.Client) *BFTRPC {
	var rpc BFTRPC
	rpc.Client = tmclient
	q := msgqueue.NewMessageQueue(rpc.messageRunFunc)
	rpc.BftMsgQueue = q
	rpc.BftMsgQueue.RunMsgEngine(config.GlobalConfig.TMMessageQueueProcesses)
	return &rpc
}

func (bftrpc BFTRPC) messageRunFunc(msg []byte) (interface{}, error) {
	telemetry.IncrementCounter(pcmn.TelemetryConstants.Broadcast.BFTMessageQueueRunCounter, pcmn.TelemetryConstants.ABCIApp.Prefix)
	response, err := bftrpc.BroadcastTxSync(msg)
	if err != nil {
		logging.WithError(err).Debug("Retrying submission of bfttx... posssible pressure/overloaded tm")
		return nil, err
	}
	return response, nil
}
