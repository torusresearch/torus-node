package dkgnode

import (
	// b64 "encoding/base64"

	"context"
	"errors"
	"fmt"
	"math/big"
	"strconv"

	"github.com/torusresearch/torus-node/telemetry"

	"github.com/libp2p/go-libp2p-core/protocol"
	pcmn "github.com/torusresearch/torus-node/common"
	"github.com/torusresearch/torus-node/eventbus"

	// tmquery "github.com/torusresearch/tendermint/libs/pubsub/query"
	// "github.com/tidwall/gjson"
	"github.com/avast/retry-go"
	logging "github.com/sirupsen/logrus"
	"github.com/torusresearch/bijson"
	"github.com/torusresearch/torus-common/crypto"
	"github.com/torusresearch/torus-node/config"

	"github.com/torusresearch/torus-common/common"

	"github.com/torusresearch/torus-node/idmutex"
	"github.com/torusresearch/torus-node/pss"
	"github.com/torusresearch/torus-node/pvss"
)

type PSSProtocolPrefix string

var pssServiceLibrary ServiceLibrary

func (pss *PSSService) GetPSSProtocolPrefix(oldEpoch int, newEpoch int) PSSProtocolPrefix {
	return PSSProtocolPrefix("pss" + "-" + strconv.Itoa(oldEpoch) + "-" + strconv.Itoa(newEpoch) + "/")
}

type PSSSuite struct {
	idmutex.Mutex
	Version          string
	FreezeState      int
	EndIndex         uint
	PSSNodeInstances map[PSSProtocolPrefix]*pss.PSSNode
}

func NewPSSService(ctx context.Context, eventBus eventbus.Bus) *BaseService {
	pssCtx, cancel := context.WithCancel(context.WithValue(ctx, ContextID, "pss"))
	pssService := PSSService{
		cancel:   cancel,
		ctx:      pssCtx,
		eventBus: eventBus,
	}
	pssServiceLibrary = NewServiceLibrary(pssService.eventBus, pssService.Name())
	return NewBaseService(&pssService)
}

type PSSService struct {
	bs       *BaseService
	cancel   context.CancelFunc
	ctx      context.Context
	eventBus eventbus.Bus

	version          string
	endIndex         int
	PSSNodeInstances map[PSSProtocolPrefix]*pss.PSSNode
}

func (p *PSSService) Name() string {
	return "pss"
}

func (p *PSSService) OnStart() error {
	p.endIndex = 10
	p.PSSNodeInstances = make(map[PSSProtocolPrefix]*pss.PSSNode)
	p.version = "0.0.2"

	return nil
}

func (p *PSSService) OnStop() error {
	return nil
}

func (p *PSSService) Call(method string, args ...interface{}) (interface{}, error) {
	telemetry.IncrementCounter(pcmn.TelemetryConstants.Generic.TotalServiceCalls, pcmn.TelemetryConstants.PSS.Prefix)

	switch method {
	case "PSS_instance_exists":
		telemetry.IncrementCounter(pcmn.TelemetryConstants.PSS.PSSInstanceExistsCounter, pcmn.TelemetryConstants.PSS.Prefix)

		var args0 PSSProtocolPrefix
		_ = castOrUnmarshal(args[0], &args0)

		return p.PSSInstanceExists(args0), nil
	case "get_PSS_protocol_prefix":
		telemetry.IncrementCounter(pcmn.TelemetryConstants.PSS.GetPSSProtocolPrefixCounter, pcmn.TelemetryConstants.PSS.Prefix)

		var args0, args1 int
		_ = castOrUnmarshal(args[0], &args0)
		_ = castOrUnmarshal(args[1], &args1)

		return p.GetPSSProtocolPrefix(args0, args1), nil
	case "receive_BFT_message":
		telemetry.IncrementCounter(pcmn.TelemetryConstants.PSS.ReceiveBFTMessageCounter, pcmn.TelemetryConstants.PSS.Prefix)

		var args0 PSSProtocolPrefix
		var args1 pss.PSSMessage
		_ = castOrUnmarshal(args[0], &args0)
		_ = castOrUnmarshal(args[1], &args1)

		pssProtocolPrefix := args0
		pssMessage := args1
		if !p.PSSInstanceExists(pssProtocolPrefix) {
			return nil, fmt.Errorf("PSS instance not available %v", pssProtocolPrefix)
		}
		return nil, p.PSSNodeInstances[pssProtocolPrefix].Transport.ReceiveBroadcast(pssMessage)
	case "get_new_nodes_n":
		telemetry.IncrementCounter(pcmn.TelemetryConstants.PSS.GetNewNodesNCounter, pcmn.TelemetryConstants.PSS.Prefix)

		var args0 PSSProtocolPrefix
		_ = castOrUnmarshal(args[0], &args0)

		pssProtocolPrefix := args0
		if !p.PSSInstanceExists(pssProtocolPrefix) {
			return nil, fmt.Errorf("PSS instance not available %v", pssProtocolPrefix)
		}
		return p.PSSNodeInstances[pssProtocolPrefix].NewNodes.N, nil
	case "get_new_nodes_k":
		telemetry.IncrementCounter(pcmn.TelemetryConstants.PSS.GetNewNodesKCounter, pcmn.TelemetryConstants.PSS.Prefix)

		var args0 PSSProtocolPrefix
		_ = castOrUnmarshal(args[0], &args0)

		pssProtocolPrefix := args0
		if !p.PSSInstanceExists(pssProtocolPrefix) {
			return nil, fmt.Errorf("PSS instance not available %v", pssProtocolPrefix)
		}
		return p.PSSNodeInstances[pssProtocolPrefix].NewNodes.K, nil
	case "get_new_nodes_t":
		telemetry.IncrementCounter(pcmn.TelemetryConstants.PSS.GetNewNodesTCounter, pcmn.TelemetryConstants.PSS.Prefix)

		var args0 PSSProtocolPrefix
		_ = castOrUnmarshal(args[0], &args0)

		pssProtocolPrefix := args0
		if !p.PSSInstanceExists(pssProtocolPrefix) {
			return nil, fmt.Errorf("PSS instance not available %v", pssProtocolPrefix)
		}
		return p.PSSNodeInstances[pssProtocolPrefix].NewNodes.T, nil
	case "get_old_nodes_n":
		telemetry.IncrementCounter(pcmn.TelemetryConstants.PSS.GetOldNodesNCounter, pcmn.TelemetryConstants.PSS.Prefix)

		var args0 PSSProtocolPrefix
		_ = castOrUnmarshal(args[0], &args0)

		pssProtocolPrefix := args0
		if !p.PSSInstanceExists(pssProtocolPrefix) {
			return nil, fmt.Errorf("PSS instance not available %v", pssProtocolPrefix)
		}
		return p.PSSNodeInstances[pssProtocolPrefix].OldNodes.N, nil
	case "get_old_nodes_k":
		telemetry.IncrementCounter(pcmn.TelemetryConstants.PSS.GetOldNodesKCounter, pcmn.TelemetryConstants.PSS.Prefix)

		var args0 PSSProtocolPrefix
		_ = castOrUnmarshal(args[0], &args0)

		pssProtocolPrefix := args0
		if !p.PSSInstanceExists(pssProtocolPrefix) {
			return nil, fmt.Errorf("PSS instance not available %v", pssProtocolPrefix)
		}
		return p.PSSNodeInstances[pssProtocolPrefix].OldNodes.K, nil
	case "get_old_nodes_t":
		telemetry.IncrementCounter(pcmn.TelemetryConstants.PSS.GetOldNodesTCounter, pcmn.TelemetryConstants.PSS.Prefix)

		var args0 PSSProtocolPrefix
		_ = castOrUnmarshal(args[0], &args0)

		pssProtocolPrefix := args0
		if !p.PSSInstanceExists(pssProtocolPrefix) {
			return nil, fmt.Errorf("PSS instance not available %v", pssProtocolPrefix)
		}
		return p.PSSNodeInstances[pssProtocolPrefix].OldNodes.T, nil
	case "new_PSS_node":
		telemetry.IncrementCounter(pcmn.TelemetryConstants.PSS.NewPSSNodeCounter, pcmn.TelemetryConstants.PSS.Prefix)

		var args0 PSSStartData
		var args1, args2 bool
		_ = castOrUnmarshal(args[0], &args0)
		_ = castOrUnmarshal(args[1], &args1)
		_ = castOrUnmarshal(args[2], &args2)

		pssSendMsg := args0
		isDealer := args1
		isPlayer := args2
		protocolPrefix := p.GetPSSProtocolPrefix(pssSendMsg.OldEpoch, pssSendMsg.NewEpoch)
		oldNodeList := getCommonNodesFromNodeRefArray(pssServiceLibrary.EthereumMethods().AwaitCompleteNodeList(pssSendMsg.OldEpoch))
		newNodeList := getCommonNodesFromNodeRefArray(pssServiceLibrary.EthereumMethods().AwaitCompleteNodeList(pssSendMsg.NewEpoch))
		selfPubKey := pssServiceLibrary.EthereumMethods().GetSelfPublicKey()
		p.PSSNodeInstances[protocolPrefix] = pss.NewPSSNode(
			pcmn.Node{
				Index:  pssServiceLibrary.EthereumMethods().GetSelfIndex(),
				PubKey: selfPubKey,
			},
			pssSendMsg.OldEpoch,
			oldNodeList,
			pssSendMsg.OldEpochT,
			pssSendMsg.OldEpochK,
			pssSendMsg.NewEpoch,
			newNodeList,
			pssSendMsg.NewEpochT,
			pssSendMsg.NewEpochK,
			pssServiceLibrary.EthereumMethods().GetSelfIndex(),
			&DKGPSSDataSource{
				eventBus: p.eventBus,
			},
			&DKGPSSTransport{
				eventBus: p.eventBus,
				Prefix:   protocolPrefix,
			},
			isDealer,
			isPlayer,
			config.GlobalConfig.StaggerDelay,
		)
		err := p.PSSNodeInstances[protocolPrefix].Transport.SetPSSNode(p.PSSNodeInstances[protocolPrefix])
		if err != nil {
			return nil, err
		}
		return nil, nil
	case "send_PSS_message_to_node":
		telemetry.IncrementCounter(pcmn.TelemetryConstants.PSS.SendPSSMessageToNodeCounter, pcmn.TelemetryConstants.PSS.Prefix)

		var args0 PSSProtocolPrefix
		var args1 pss.NodeDetails
		var args2 pss.PSSMessage
		_ = castOrUnmarshal(args[0], &args0)
		_ = castOrUnmarshal(args[1], &args1)
		_ = castOrUnmarshal(args[2], &args2)

		prefix := args0
		nodeDetails := args1
		pssMessage := args2
		pssN, ok := p.PSSNodeInstances[prefix]
		if !ok {
			return nil, fmt.Errorf("Could not get pssNode for prefix %v", prefix)
		}
		err := pssN.Transport.Send(nodeDetails, pssMessage)
		return nil, err
	}
	return nil, fmt.Errorf("PSS service method %v not found", method)
}

func getCommonNodesFromNodeRefArray(nodeRefs []NodeReference) (commonNodes []pcmn.Node) {
	for _, nodeRef := range nodeRefs {
		commonNodes = append(commonNodes, pcmn.Node{
			Index: int(nodeRef.Index.Int64()),
			PubKey: common.Point{
				X: *nodeRef.PublicKey.X,
				Y: *nodeRef.PublicKey.Y,
			},
		})
	}
	return
}

func (p *PSSService) PSSInstanceExists(pssProtocolPrefix PSSProtocolPrefix) bool {
	return p.PSSNodeInstances[pssProtocolPrefix] != nil
}

func (p *PSSService) SetBaseService(bs *BaseService) {
	p.bs = bs
}

type Middleware func(pss.PSSMessage) (modifiedMessage pss.PSSMessage, end bool, err error)

type DKGPSSDataSource struct {
	eventBus eventbus.Bus
	PSSNode  *pss.PSSNode
}

func (ds *DKGPSSDataSource) Init() {}

func (ds *DKGPSSDataSource) GetSharing(keygenID pss.KeygenID) *pss.Sharing {
	serviceLibrary := NewServiceLibrary(ds.eventBus, "dkgPSSDataSource")
	matrix, err := serviceLibrary.DatabaseMethods().RetrieveCommitmentMatrix(*big.NewInt(int64(keygenID.GetIndex())))
	if err != nil {
		logging.WithError(err).Error("could not retrieve commitmentmatrix")
	}
	var c []common.Point
	for _, arr := range matrix {
		c = append(c, arr[0])
	}
	si, siprime, err := serviceLibrary.DatabaseMethods().RetrieveCompletedShare(*big.NewInt(int64(keygenID.GetIndex())))
	logging.WithFields(logging.Fields{
		"si":      si,
		"siprime": siprime,
	}).WithError(err).Debug()
	if err != nil {
		logging.WithError(err).Error("error in suite dbsuite retrieve completed share")
	}
	nodeRefArray := serviceLibrary.EthereumMethods().GetNodeList(ds.PSSNode.OldNodes.EpochID)
	var oldNodeList []pcmn.Node
	for j := 0; j < len(nodeRefArray); j++ {
		oldNodeList = append(oldNodeList, pcmn.Node{
			Index: int(nodeRefArray[j].Index.Int64()),
			PubKey: common.Point{
				X: *nodeRefArray[j].PublicKey.X,
				Y: *nodeRefArray[j].PublicKey.Y,
			},
		})
	}
	return &pss.Sharing{
		KeygenID: keygenID,
		Nodes:    oldNodeList,
		Epoch:    ds.PSSNode.OldNodes.EpochID,
		I:        serviceLibrary.EthereumMethods().GetSelfIndex(),
		Si:       si,
		Siprime:  siprime,
		C:        c,
	}

}

func (ds *DKGPSSDataSource) SetPSSNode(pssNode *pss.PSSNode) error {
	ds.PSSNode = pssNode
	return nil
}

type DKGPSSTransport struct {
	eventBus          eventbus.Bus
	Prefix            PSSProtocolPrefix
	PSSNode           *pss.PSSNode
	SendMiddleware    []Middleware
	ReceiveMiddleware []Middleware
}

func (tp *DKGPSSTransport) Init() {
	err := NewServiceLibrary(tp.eventBus, "dkgPSSTransport").P2PMethods().SetStreamHandler(string(tp.Prefix), tp.streamHandler)
	if err != nil {
		logging.WithField("DKGPSSTransportPrefix", tp.Prefix).WithError(err).Error("could not set stream handler")
	}
}

func (tp *DKGPSSTransport) SetEventBus(e eventbus.Bus) {
	tp.eventBus = e
}

type TransportMessage struct {
	Sender common.Point
	Data   []byte
	Sig    []byte
}

func (tp *DKGPSSTransport) runSendMiddleware(pssMessage pss.PSSMessage) (pss.PSSMessage, error) {
	modifiedMessage := pssMessage
	for _, middleware := range tp.SendMiddleware {
		var end bool
		var err error
		modifiedMessage, end, err = middleware(modifiedMessage)
		if end {
			break
		}
		if err != nil {
			return pssMessage, err
		}
	}
	return modifiedMessage, nil
}

func (tp *DKGPSSTransport) runReceiveMiddleware(pssMessage pss.PSSMessage) (pss.PSSMessage, error) {
	modifiedMessage := pssMessage
	for _, middleware := range tp.ReceiveMiddleware {
		var end bool
		var err error
		modifiedMessage, end, err = middleware(modifiedMessage)
		if end {
			break
		}
		if err != nil {
			return pssMessage, err
		}
	}
	return modifiedMessage, nil
}

func (tp *DKGPSSTransport) streamHandler(streamMessage StreamMessage) {
	logging.Debug("streamHandler receiving message")
	// get request data
	p2pBasicMsg := streamMessage.Message
	if NewServiceLibrary(tp.eventBus, "dkgPSSTransport").P2PMethods().AuthenticateMessage(p2pBasicMsg) != nil {
		logging.WithField("p2pBasicMsg", p2pBasicMsg).Error("could not authenticate incoming p2pBasicMsg iin dkgpsstransport")
		return
	}

	pssMessage := pss.PSSMessage{}
	err := bijson.Unmarshal(p2pBasicMsg.Payload, &pssMessage)
	if err != nil {
		logging.WithError(err).Error("could not unmarshal payload in p2p streamhandler")
		return
	}
	var pubKey = common.Point{}
	err = bijson.Unmarshal(p2pBasicMsg.GetNodePubKey(), &pubKey)
	if err != nil {
		logging.WithError(err).Error("could not unmarshal pubkey in p2p streamhandler")
		return
	}
	logging.Debugf("able to get pubk X: %s, Y: %s", pubKey.X.Text(16), pubKey.Y.Text(16))
	ethNodeDetails := NewServiceLibrary(tp.eventBus, "dkgPSSTransport").EthereumMethods().GetNodeDetailsByAddress(*crypto.PointToEthAddress(pubKey))
	err = tp.Receive(pss.NodeDetails(pcmn.Node{
		Index:  int(ethNodeDetails.Index.Int64()),
		PubKey: pubKey,
	}), pssMessage)

	if err != nil {
		logging.WithError(err).Error("error when received message")
		return
	}
}

func (tp *DKGPSSTransport) GetType() string {
	return "dkgpss"
}

func (tp *DKGPSSTransport) SetPSSNode(ref *pss.PSSNode) error {
	tp.PSSNode = ref
	return nil
}

func (tp *DKGPSSTransport) Send(nodeDetails pss.NodeDetails, originalPSSMessage pss.PSSMessage) error {
	logging.WithFields(logging.Fields{
		"to":      stringify(nodeDetails),
		"message": stringify(originalPSSMessage),
		"data":    string(originalPSSMessage.Data),
	}).Debug("trying to send message")

	pssMessage, err := tp.runSendMiddleware(originalPSSMessage)
	if err != nil {
		logging.WithError(err).Error("Could not run send middleware")
		return err
	}

	// get recipient details
	ethNodeDetails := NewServiceLibrary(tp.eventBus, "dkgPSSTransport").EthereumMethods().GetNodeDetailsByAddress(*crypto.PointToEthAddress(nodeDetails.PubKey))

	logging.WithField("ethNodeDetails", ethNodeDetails).Debug("managed to get ethNodeDetails")
	pubKey := NewServiceLibrary(tp.eventBus, "dkgPSSTransport").EthereumMethods().GetSelfPublicKey()
	if nodeDetails.PubKey.X.Cmp(&pubKey.X) == 0 && nodeDetails.PubKey.Y.Cmp(&pubKey.Y) == 0 {
		return tp.Receive(nodeDetails, pssMessage)
	}

	byt, err := bijson.Marshal(pssMessage)
	if err != nil {
		return err
	}
	p2pMsg := NewServiceLibrary(tp.eventBus, "dkgPSSTransport").P2PMethods().NewP2PMessage(HashToString(byt), false, byt, "transportPSSMessage")
	peerID, err := GetPeerIDFromP2pListenAddress(ethNodeDetails.P2PConnection)
	if err != nil {
		return err
	}
	// sign the data
	signature, err := NewServiceLibrary(tp.eventBus, "dkgPSSTransport").P2PMethods().SignP2PMessage(&p2pMsg)
	if err != nil {
		return errors.New("failed to sign p2pMsgonse" + err.Error())
	}
	p2pMsg.Sign = signature
	err = retry.Do(func() error {
		err = NewServiceLibrary(tp.eventBus, "dkgPSSTransport").P2PMethods().SendP2PMessage(*peerID, protocol.ID(tp.Prefix), &p2pMsg)
		if err != nil {
			logging.WithFields(logging.Fields{
				"peerID":     peerID,
				"protocolID": protocol.ID(tp.Prefix),
				"p2pMsg":     stringify(p2pMsg),
			}).Debug("error when sending p2p message pss")
			return err
		}
		return nil
	})
	if err != nil {
		logging.WithError(err).Error("could not sendp2pmessage")
		return err
	}

	telemetry.IncrementCounter(pcmn.TelemetryConstants.Message.SentMessageCounter, pcmn.TelemetryConstants.PSS.Prefix)
	return nil
}

func (tp *DKGPSSTransport) Receive(senderDetails pss.NodeDetails, originalPSSMessage pss.PSSMessage) error {
	logging.WithFields(logging.Fields{
		"senderDetails": stringify(senderDetails),
		"message":       stringify(originalPSSMessage),
		"data":          stringify(originalPSSMessage.Data),
	}).Debug("message received")
	telemetry.IncrementCounter(pcmn.TelemetryConstants.Message.ReceivedMessageCounter, pcmn.TelemetryConstants.PSS.Prefix)

	pssMessage, err := tp.runReceiveMiddleware(originalPSSMessage)
	if err != nil {
		logging.WithError(err).Error("Could not run receive middleware")
		return err
	}

	return tp.PSSNode.ProcessMessage(senderDetails, pssMessage)
}

func (tp *DKGPSSTransport) SendBroadcast(originalPSSMessage pss.PSSMessage) error {
	logging.WithFields(logging.Fields{
		"pssMessage": stringify(originalPSSMessage),
		"data":       string(originalPSSMessage.Data),
	}).Debug("sending broadcast")

	pssMessage, err := tp.runSendMiddleware(originalPSSMessage)
	if err != nil {
		logging.WithError(err).Error("Could not run send middleware")
		return err
	}

	_, err = NewServiceLibrary(tp.eventBus, "dkgPSSTransport").TendermintMethods().Broadcast(pssMessage)
	if err == nil {
		telemetry.IncrementCounter(pcmn.TelemetryConstants.Broadcast.SentBroadcastCounter, pcmn.TelemetryConstants.PSS.Prefix)
		return nil
	}

	return err
}

func (tp *DKGPSSTransport) ReceiveBroadcast(originalPSSMessage pss.PSSMessage) error {
	logging.WithFields(logging.Fields{
		"pssMessage": stringify(originalPSSMessage),
		"data":       string(originalPSSMessage.Data),
	}).Debug("received broadcast")
	telemetry.IncrementCounter(pcmn.TelemetryConstants.Broadcast.ReceivedBroadcastCounter, pcmn.TelemetryConstants.PSS.Prefix)

	pssMessage, err := tp.runReceiveMiddleware(originalPSSMessage)
	if err != nil {
		logging.WithError(err).Error("Could not run receive middleware")
		return err
	}

	return tp.PSSNode.ProcessBroadcastMessage(pssMessage)
}
func (tp *DKGPSSTransport) Output(inter interface{}) {
	str, ok := inter.(string)
	if ok {
		logging.WithFields(logging.Fields{
			"tp":     tp,
			"output": str,
		}).Debug()
		return
	}

	rks, ok := inter.(pss.RefreshKeyStorage)
	if ok {
		logging.WithField("rks", stringify(rks)).Debug()
		var fakeCommitmentMatrix [][]common.Point
		for _, pt := range rks.CommitmentPoly {
			var fakePoly []common.Point
			fakePoly = append(fakePoly, pt)
			for i := 1; i < len(rks.CommitmentPoly); i++ {
				fakePoly = append(fakePoly, common.Point{})
			}
			fakeCommitmentMatrix = append(fakeCommitmentMatrix, fakePoly)
		}

		err := NewServiceLibrary(tp.eventBus, "dkgPSSTransport").DatabaseMethods().StorePSSCommitmentMatrix(rks.KeyIndex, fakeCommitmentMatrix)
		if err != nil {
			logging.WithError(err).Error("StoreCommitmentMatrix failed")
		}
		err = NewServiceLibrary(tp.eventBus, "dkgPSSTransport").DatabaseMethods().StoreCompletedPSSShare(rks.KeyIndex, rks.Si, rks.Siprime)
		if err != nil {
			logging.WithError(err).Error("StoreCompletedPSSShare failed")
		}
		return
	}

	logging.WithField("inter", inter).Error("Unexpected output type in dkgPSSTransport")
}

func (tp *DKGPSSTransport) Sign(s []byte) ([]byte, error) {
	k := NewServiceLibrary(tp.eventBus, "dkgPSSTransport").EthereumMethods().GetSelfPrivateKey()
	return pvss.ECDSASignBytes(s, &k), nil
}
