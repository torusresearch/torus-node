package dkgnode

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"strconv"

	"github.com/avast/retry-go"
	"github.com/libp2p/go-libp2p-core/protocol"
	logging "github.com/sirupsen/logrus"
	"github.com/torusresearch/bijson"
	"github.com/torusresearch/torus-common/common"
	"github.com/torusresearch/torus-common/crypto"
	pcmn "github.com/torusresearch/torus-node/common"
	"github.com/torusresearch/torus-node/config"
	"github.com/torusresearch/torus-node/eventbus"
	"github.com/torusresearch/torus-node/idmutex"
	"github.com/torusresearch/torus-node/keygennofsm"
	"github.com/torusresearch/torus-node/pvss"
	"github.com/torusresearch/torus-node/telemetry"
)

type KeygenProtocolPrefix string

type KeygennofsmService struct {
	idmutex.Mutex

	bs       *BaseService
	cancel   context.CancelFunc
	ctx      context.Context
	eventBus eventbus.Bus

	KeygenNode *keygennofsm.KeygenNode
}

func NewKeygennofsmService(ctx context.Context, eventBus eventbus.Bus) *BaseService {
	keygenCtx, cancel := context.WithCancel(context.WithValue(ctx, ContextID, "keygennofsm"))
	return NewBaseService(&KeygennofsmService{
		cancel:   cancel,
		ctx:      keygenCtx,
		eventBus: eventBus,
	})
}

func (k *KeygennofsmService) Name() string {
	return "keygennofsm"
}
func (k *KeygennofsmService) OnStart() error {
	serviceLibrary := NewServiceLibrary(k.eventBus, "keygennofsm")
	selfIndex := serviceLibrary.EthereumMethods().GetSelfIndex()
	selfPubKey := serviceLibrary.EthereumMethods().GetSelfPublicKey()
	currNodeList := serviceLibrary.EthereumMethods().AwaitCompleteNodeList(config.GlobalConfig.InitEpoch)
	currEpoch := abciServiceLibrary.EthereumMethods().GetCurrentEpoch()
	currEpochInfo, err := abciServiceLibrary.EthereumMethods().GetEpochInfo(currEpoch, true)
	if err != nil {
		return err
	}

	k.KeygenNode = keygennofsm.NewKeygenNode(
		pcmn.Node{
			Index:  selfIndex,
			PubKey: selfPubKey,
		},
		getCommonNodesFromNodeRefArray(currNodeList),
		int(currEpochInfo.T.Int64()),
		int(currEpochInfo.K.Int64()),
		selfIndex,
		&DKGKeygennofsmTransport{
			Prefix:   k.GetKeygenProtocolPrefix(config.GlobalConfig.InitEpoch),
			eventBus: k.eventBus,
		},
		config.GlobalConfig.StaggerDelay,
	)

	return nil
}
func (k *KeygennofsmService) OnStop() error {
	return nil
}
func (k *KeygennofsmService) Call(method string, args ...interface{}) (interface{}, error) {

	telemetry.IncrementCounter(pcmn.TelemetryConstants.Keygen.KeygenTotalCallCounter, pcmn.TelemetryConstants.Keygen.Prefix)

	serviceLibrary := NewServiceLibrary(k.eventBus, "keygennofsm")

	switch method {

	case "receive_message":
		telemetry.IncrementCounter(pcmn.TelemetryConstants.Keygen.ReceiveMessageCounter, pcmn.TelemetryConstants.Keygen.Prefix)

		var args0 keygennofsm.KeygenMessage
		_ = castOrUnmarshal(args[0], &args0)

		keygenMessage := args0
		// Only run keygen once even if the share message is sent multiple times
		if keygenMessage.Method == "share" {
			k.Lock()
			defer k.Unlock()
			if serviceLibrary.DatabaseMethods().GetKeygenStarted(string(keygenMessage.KeygenID)) {
				logging.WithField("keygenID", keygenMessage.KeygenID).Info("Keygen already started")
				return nil, nil
			}
			err := serviceLibrary.DatabaseMethods().SetKeygenStarted(string(keygenMessage.KeygenID), true)
			if err != nil {
				return nil, err
			}
		}
		selfPubKey := serviceLibrary.EthereumMethods().GetSelfPublicKey()
		selfIndex := serviceLibrary.EthereumMethods().GetSelfIndex()
		selfNode := pcmn.Node{
			PubKey: selfPubKey,
			Index:  selfIndex,
		}

		return nil, k.KeygenNode.Transport.Receive(keygennofsm.NodeDetails(selfNode), args0)

	case "receive_BFT_message":
		telemetry.IncrementCounter(pcmn.TelemetryConstants.Keygen.ReceiveBFTMessageCounter, pcmn.TelemetryConstants.Keygen.Prefix)

		var args0 keygennofsm.KeygenMessage
		_ = castOrUnmarshal(args[0], &args0)

		return nil, k.KeygenNode.Transport.ReceiveBroadcast(args0)

	}

	return nil, fmt.Errorf("keygen service method %v not found", method)
}
func (k *KeygennofsmService) SetBaseService(bs *BaseService) {
	k.bs = bs
}
func (k *KeygennofsmService) GetKeygenProtocolPrefix(currEpoch int) KeygenProtocolPrefix {
	return KeygenProtocolPrefix("keygen" + "-" + strconv.Itoa(currEpoch) + "/")
}

type DKGKeygennofsmTransport struct {
	eventBus eventbus.Bus

	Prefix     KeygenProtocolPrefix
	KeygenNode *keygennofsm.KeygenNode
}

func (tp *DKGKeygennofsmTransport) streamHandler(streamMessage StreamMessage) {
	logging.Debug("streamHandler receiving message")
	telemetry.IncrementCounter(pcmn.TelemetryConstants.Keygen.ReceivedStreamMessage, pcmn.TelemetryConstants.Keygen.Prefix)

	// get request data
	p2pBasicMsg := streamMessage.Message
	if NewServiceLibrary(tp.eventBus, "dkgKeygenTransport").P2PMethods().AuthenticateMessage(p2pBasicMsg) != nil {
		logging.WithField("p2pBasicMsg", p2pBasicMsg).Error("could not authenticate incoming p2pBasicMsg in keygennofsm")
		return
	}

	var keygenMessage keygennofsm.KeygenMessage
	err := bijson.Unmarshal(p2pBasicMsg.Payload, &keygenMessage)
	if err != nil {
		logging.WithError(err).Error("could not unmarshal payload to keyMessage")
		return
	}
	var pubKey common.Point
	err = bijson.Unmarshal(p2pBasicMsg.GetNodePubKey(), &pubKey)
	if err != nil {
		logging.WithError(err).Error("could not unmarshal pubkey")
		return
	}
	logging.Debugf("able to get pubk X: %s, Y: %s", pubKey.X.Text(16), pubKey.Y.Text(16))
	ethNodeDetails := NewServiceLibrary(tp.eventBus, "dkgPSSTransport").EthereumMethods().GetNodeDetailsByAddress(*crypto.PointToEthAddress(pubKey))
	index := int(ethNodeDetails.Index.Int64())
	go func(ind int, pubK common.Point, keygenMsg keygennofsm.KeygenMessage) {
		err := tp.Receive(keygennofsm.NodeDetails(pcmn.Node{
			Index:  ind,
			PubKey: pubK,
		}), keygenMsg)
		if err != nil {
			logging.WithError(err).Error("error when received message")
			return
		}
	}(index, pubKey, keygenMessage)
}

func (tp *DKGKeygennofsmTransport) Init() {
	err := NewServiceLibrary(tp.eventBus, "dkgKeygenTransport").P2PMethods().SetStreamHandler(string(tp.Prefix), tp.streamHandler)
	if err != nil {
		logging.WithField("DKGPSSTransportPrefix", tp.Prefix).WithError(err).Error("could not set stream handler")
	}
}
func (tp *DKGKeygennofsmTransport) GetType() string {
	return "dkgkeygen"
}
func (tp *DKGKeygennofsmTransport) SetKeygenNode(keygenNode *keygennofsm.KeygenNode) error {
	tp.KeygenNode = keygenNode
	return nil
}
func (tp *DKGKeygennofsmTransport) Sign(s []byte) ([]byte, error) {
	k := NewServiceLibrary(tp.eventBus, "dkgKeygenTransport").EthereumMethods().GetSelfPrivateKey()
	return pvss.ECDSASignBytes(s, &k), nil
}
func (tp *DKGKeygennofsmTransport) Send(nodeDetails keygennofsm.NodeDetails, keygenMessage keygennofsm.KeygenMessage) error {

	serviceLibrary := NewServiceLibrary(tp.eventBus, "dkgKeygenTransport")
	logging.WithFields(logging.Fields{
		"to":      stringify(nodeDetails),
		"message": stringify(keygenMessage),
		"data":    string(keygenMessage.Data),
	}).Debug("trying to send message")
	// get recipient details
	ethNodeDetails := serviceLibrary.EthereumMethods().GetNodeDetailsByAddress(*crypto.PointToEthAddress(nodeDetails.PubKey))

	logging.WithField("ethNodeDetails", ethNodeDetails).Debug("managed to get ethNodeDetails")
	pubKey := serviceLibrary.EthereumMethods().GetSelfPublicKey()
	if nodeDetails.PubKey.X.Cmp(&pubKey.X) == 0 && nodeDetails.PubKey.Y.Cmp(&pubKey.Y) == 0 {
		return tp.Receive(nodeDetails, keygenMessage)
	}

	byt, err := bijson.Marshal(keygenMessage)
	if err != nil {
		return err
	}
	p2pMsg := serviceLibrary.P2PMethods().NewP2PMessage(HashToString(byt), false, byt, "transportKeygenMessage")
	peerID, err := GetPeerIDFromP2pListenAddress(ethNodeDetails.P2PConnection)
	if err != nil {
		return err
	}
	// sign the data
	signature, err := serviceLibrary.P2PMethods().SignP2PMessage(&p2pMsg)
	if err != nil {
		return errors.New("failed to sign p2p Message" + err.Error())
	}
	p2pMsg.Sign = signature
	err = retry.Do(func() error {
		err := serviceLibrary.P2PMethods().SendP2PMessage(*peerID, protocol.ID(tp.Prefix), &p2pMsg)
		if err != nil {
			logging.WithFields(logging.Fields{
				"peerID":     peerID,
				"protocolID": protocol.ID(tp.Prefix),
				"p2pMsg":     stringify(p2pMsg),
			}).WithError(err).Debug("error when sending p2p message")
			return err
		}
		return nil
	})
	if err != nil {
		logging.Error("Could not send the p2p message, failed after retries " + err.Error())
		return err
	}

	telemetry.IncrementCounter(pcmn.TelemetryConstants.Message.SentMessageCounter, pcmn.TelemetryConstants.Keygen.Prefix)
	return nil
}
func (tp *DKGKeygennofsmTransport) Receive(senderDetails keygennofsm.NodeDetails, keygenMessage keygennofsm.KeygenMessage) error {
	logging.WithFields(logging.Fields{
		"senderDetails": stringify(senderDetails),
		"message":       stringify(keygenMessage),
		"data":          stringify(keygenMessage.Data),
	}).Debug("message received")
	telemetry.IncrementCounter(pcmn.TelemetryConstants.Message.ReceivedMessageCounter, pcmn.TelemetryConstants.Keygen.Prefix)

	return tp.KeygenNode.ProcessMessage(senderDetails, keygenMessage)
}
func (tp *DKGKeygennofsmTransport) SendBroadcast(keygenMessage keygennofsm.KeygenMessage) error {
	logging.WithFields(logging.Fields{
		"pssMessage": stringify(keygenMessage),
		"data":       string(keygenMessage.Data),
	}).Debug("sending broadcast")
	_, err := NewServiceLibrary(tp.eventBus, "dkgKeygenTransport").TendermintMethods().Broadcast(keygenMessage)

	if err == nil {
		telemetry.IncrementCounter(pcmn.TelemetryConstants.Broadcast.SentBroadcastCounter, pcmn.TelemetryConstants.Keygen.Prefix)
		return nil
	}

	return err
}
func (tp *DKGKeygennofsmTransport) ReceiveBroadcast(keygenMessage keygennofsm.KeygenMessage) error {
	logging.WithFields(logging.Fields{
		"pssMessage": stringify(keygenMessage),
		"data":       string(keygenMessage.Data),
	}).Debug("revived broadcast")
	telemetry.IncrementCounter(pcmn.TelemetryConstants.Broadcast.ReceivedBroadcastCounter, pcmn.TelemetryConstants.Keygen.Prefix)

	return tp.KeygenNode.ProcessBroadcastMessage(keygenMessage)
}

func (tp *DKGKeygennofsmTransport) Output(inter interface{}) {
	serviceLibrary := NewServiceLibrary(tp.eventBus, "dkgkeygennofsmtransport")
	str, ok := inter.(string)
	if ok {
		logging.Info("dkgkeygennofsmtransport got output " + str)
		return
	}
	keyStorage, ok := inter.(keygennofsm.KeyStorage)
	if ok {
		if len(keyStorage.CommitmentPoly) == 0 || keyStorage.CommitmentPoly[0].X.Cmp(big.NewInt(0)) == 0 {
			logging.WithField("keystorage", keyStorage).Error("Invalid keystorage commitment poly")
			return
		}
		var fakeCommitmentMatrix [][]common.Point
		for _, pt := range keyStorage.CommitmentPoly {
			var fakePoly []common.Point
			fakePoly = append(fakePoly, pt)
			for i := 1; i < len(keyStorage.CommitmentPoly); i++ {
				fakePoly = append(fakePoly, common.Point{})
			}
			fakeCommitmentMatrix = append(fakeCommitmentMatrix, fakePoly)
		}
		err := serviceLibrary.DatabaseMethods().StoreKeygenCommitmentMatrix(keyStorage.KeyIndex, fakeCommitmentMatrix)
		if err != nil {
			logging.WithField("keyStorage", keyStorage).Error("Could not store commitment matrix")
		}
		err = serviceLibrary.DatabaseMethods().StoreCompletedKeygenShare(
			keyStorage.KeyIndex,
			keyStorage.Si,
			keyStorage.Siprime,
		)
		if err != nil {
			logging.WithField("keyStorage", keyStorage).Error("Could not store completed share and pub key to index")
		}
		return
	}
	logging.WithField("inter", inter).Error("Unexpected output type in dkgkeygennofsmtransport")
}

func (tp *DKGKeygennofsmTransport) CheckIfNIZKPProcessed(keyIndex big.Int) bool {
	serviceLibrary := NewServiceLibrary(tp.eventBus, "dkgkeygennofsmtransport")
	return serviceLibrary.DatabaseMethods().IndexToPublicKeyExists(keyIndex)
}
