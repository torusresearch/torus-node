package dkgnode

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"sync"

	"github.com/torusresearch/torus-node/telemetry"

	"github.com/avast/retry-go"
	"github.com/libp2p/go-libp2p-core/protocol"
	logging "github.com/sirupsen/logrus"
	"github.com/torusresearch/bijson"
	"github.com/torusresearch/torus-common/common"
	"github.com/torusresearch/torus-common/crypto"
	pcmn "github.com/torusresearch/torus-node/common"
	"github.com/torusresearch/torus-node/eventbus"
	"github.com/torusresearch/torus-node/mapping"
	"github.com/torusresearch/torus-node/pvss"
)

type MappingProtocolPrefix string

var mappingServiceLibrary ServiceLibrary

type MappingService struct {
	bs            *BaseService
	cancel        context.CancelFunc
	parentContext context.Context
	context       context.Context
	eventBus      eventbus.Bus

	MappingInstances map[mapping.MappingID]*mapping.MappingNode
	FreezeState      *FreezeStateSyncMap
}

func (m *MappingService) Name() string {
	return "mapping"
}
func (m *MappingService) SetBaseService(bs *BaseService) {
	m.bs = bs
}
func (m *MappingService) OnStart() error {
	m.MappingInstances = make(map[mapping.MappingID]*mapping.MappingNode)
	return nil
}
func (m *MappingService) OnStop() error {
	return nil
}
func (m *MappingService) Call(method string, args ...interface{}) (result interface{}, err error) {
	telemetry.IncrementCounter(pcmn.TelemetryConstants.Generic.TotalServiceCalls, pcmn.TelemetryConstants.Mapping.Prefix)

	switch method {
	case "new_mapping_node":
		telemetry.IncrementCounter(pcmn.TelemetryConstants.Mapping.NewMappingNodeCounter, pcmn.TelemetryConstants.Mapping.Prefix)
		var args0 MappingStartData
		var args1, args2 bool
		_ = castOrUnmarshal(args[0], &args0)
		_ = castOrUnmarshal(args[1], &args1)
		_ = castOrUnmarshal(args[2], &args2)

		mappingStartData := args0
		isOldNode := args1
		isNewNode := args2
		mappingID := m.GetMappingID(mappingStartData.OldEpoch, mappingStartData.NewEpoch)
		oldNodeList := getCommonNodesFromNodeRefArray(mappingServiceLibrary.EthereumMethods().AwaitCompleteNodeList(mappingStartData.OldEpoch))
		newNodeList := getCommonNodesFromNodeRefArray(mappingServiceLibrary.EthereumMethods().AwaitCompleteNodeList(mappingStartData.NewEpoch))
		selfPubKey := mappingServiceLibrary.EthereumMethods().GetSelfPublicKey()
		m.MappingInstances[mappingID] = mapping.NewMappingNode(
			pcmn.Node{
				Index:  mappingServiceLibrary.EthereumMethods().GetSelfIndex(),
				PubKey: selfPubKey,
			},
			mappingStartData.OldEpoch,
			oldNodeList,
			mappingStartData.OldEpochT,
			mappingStartData.OldEpochK,
			mappingStartData.NewEpoch,
			newNodeList,
			mappingStartData.NewEpochT,
			mappingStartData.NewEpochK,
			mappingServiceLibrary.EthereumMethods().GetSelfIndex(),
			NewDKGMappingTransport(m.eventBus, mappingStartData.OldEpoch, mappingStartData.NewEpoch),
			NewDKGMappingDataSource(m.eventBus),
			isOldNode,
			isNewNode,
		)
		return nil, nil
	case "receive_BFT_message":
		telemetry.IncrementCounter(pcmn.TelemetryConstants.Mapping.ReceiveBFTMessageCounter, pcmn.TelemetryConstants.Mapping.Prefix)

		var args0 mapping.MappingID
		var args1 mapping.MappingMessage
		_ = castOrUnmarshal(args[0], &args0)
		_ = castOrUnmarshal(args[1], &args1)

		mappingID := args0
		mappingMessage := args1
		if !m.MappingInstanceExists(mappingID) {
			return nil, fmt.Errorf("Mapping instance not available %v", mappingID)
		}
		return m.MappingInstances[mappingID].Transport.ReceiveBroadcast(mappingMessage), nil
	case "mapping_instance_exists":
		telemetry.IncrementCounter(pcmn.TelemetryConstants.Mapping.MappingInstanceExistsCounter, pcmn.TelemetryConstants.Mapping.Prefix)

		var args0 mapping.MappingID
		_ = castOrUnmarshal(args[0], &args0)

		mappingID := args0
		return m.MappingInstanceExists(mappingID), nil
	case "get_mapping_ID":
		telemetry.IncrementCounter(pcmn.TelemetryConstants.Mapping.GetMappingIDCounter, pcmn.TelemetryConstants.Mapping.Prefix)

		var args0, args1 int
		_ = castOrUnmarshal(args[0], &args0)
		_ = castOrUnmarshal(args[1], &args1)

		return m.GetMappingID(args0, args1), nil
	case "get_freeze_state":
		telemetry.IncrementCounter(pcmn.TelemetryConstants.Mapping.GetFreezeStateCounter, pcmn.TelemetryConstants.Mapping.Prefix)

		var args0 mapping.MappingID
		_ = castOrUnmarshal(args[0], &args0)

		mappingID := args0
		return m.FreezeState.Get(mappingID), nil
	case "set_freeze_state":
		telemetry.IncrementCounter(pcmn.TelemetryConstants.Mapping.SetFreezeStateCounter, pcmn.TelemetryConstants.Mapping.Prefix)

		var args0 mapping.MappingID
		var args1 int
		var args2 uint
		_ = castOrUnmarshal(args[0], &args0)
		_ = castOrUnmarshal(args[1], &args1)
		_ = castOrUnmarshal(args[2], &args2)

		mappingID := args0
		freezeState := args1
		lastUnassignedIndex := args2
		m.FreezeState.Set(mappingID, FreezeStateData{
			FreezeState:         freezeState,
			LastUnassignedIndex: lastUnassignedIndex,
		})
		return nil, nil
	case "propose_freeze":
		telemetry.IncrementCounter(pcmn.TelemetryConstants.Mapping.ProposeFreezeCounter, pcmn.TelemetryConstants.Mapping.Prefix)

		var args0 mapping.MappingID
		_ = castOrUnmarshal(args[0], &args0)

		mappingID := args0
		if !m.MappingInstanceExists(mappingID) {
			return nil, fmt.Errorf("could not find mappingInstance for mappingID %s", mappingID)
		}
		mappingProposeFreezeMessage := mapping.MappingProposeFreezeMessage{
			MappingID: mappingID,
		}
		byt, err := bijson.Marshal(mappingProposeFreezeMessage)
		if err != nil {
			return nil, fmt.Errorf("could not marshal mapping propose freeze message %v", mappingProposeFreezeMessage)
		}
		err = m.MappingInstances[mappingID].Transport.Receive(mapping.NodeDetails{
			Index:  mappingServiceLibrary.EthereumMethods().GetSelfIndex(),
			PubKey: mappingServiceLibrary.EthereumMethods().GetSelfPublicKey(),
		}, mapping.CreateMappingMessage(mapping.MappingMessageRaw{
			Method:    "mapping_propose_freeze",
			MappingID: mappingID,
			Data:      byt,
		}))
		return nil, err
	case "mapping_summary_frozen":
		telemetry.IncrementCounter(pcmn.TelemetryConstants.Mapping.MappingSummaryFrozenCounter, pcmn.TelemetryConstants.Mapping.Prefix)

		var args0 mapping.MappingID
		var args1 mapping.MappingSummaryMessage
		_ = castOrUnmarshal(args[0], &args0)
		_ = castOrUnmarshal(args[1], &args1)

		mappingID := args0
		mappingSummaryMessage := args1
		if !m.MappingInstanceExists(mappingID) {
			return nil, fmt.Errorf("could not find mappingInstance for mappingID %s", mappingID)
		}
		byt, err := bijson.Marshal(mappingSummaryMessage)
		if err != nil {
			return nil, err
		}
		err = m.MappingInstances[mappingID].Transport.ReceiveBroadcast(mapping.MappingMessage{
			MappingID: mappingID,
			Method:    "mapping_summary_frozen",
			Data:      byt,
		})
		return nil, err
	case "get_mapping_protocol_prefix":
		telemetry.IncrementCounter(pcmn.TelemetryConstants.Mapping.GetMappingProtocolPrefixCounter, pcmn.TelemetryConstants.Mapping.Prefix)

		var args0, args1 int
		_ = castOrUnmarshal(args[0], &args0)
		_ = castOrUnmarshal(args[1], &args1)

		oldEpoch := args0
		newEpoch := args1
		return m.GetMappingProtocolPrefix(oldEpoch, newEpoch), nil
	default:
		return nil, fmt.Errorf("Unimplemented MappingService method %s", method)
	}
}
func (m *MappingService) MappingInstanceExists(mappingID mapping.MappingID) bool {
	_, found := m.MappingInstances[mappingID]
	return found
}
func (m *MappingService) GetMappingProtocolPrefix(oldEpoch, newEpoch int) MappingProtocolPrefix {
	return MappingProtocolPrefix("mapping" + "-" + strconv.Itoa(oldEpoch) + "-" + strconv.Itoa(newEpoch) + "/")
}
func (m *MappingService) GetMappingID(oldEpoch, newEpoch int) mapping.MappingID {
	mappingIDDetails := mapping.MappingIDDetails{
		OldEpoch: oldEpoch,
		NewEpoch: newEpoch,
	}
	return mappingIDDetails.ToMappingID()
}

func NewMappingService(ctx context.Context, eventBus eventbus.Bus) *BaseService {

	mappingCtx, cancel := context.WithCancel(context.WithValue(ctx, ContextID, "mapping"))
	mappingService := MappingService{
		cancel:           cancel,
		parentContext:    ctx,
		context:          mappingCtx,
		eventBus:         eventBus,
		MappingInstances: make(map[mapping.MappingID]*mapping.MappingNode),
		FreezeState:      &FreezeStateSyncMap{},
	}

	mappingServiceLibrary = NewServiceLibrary(mappingService.eventBus, mappingService.Name())

	return NewBaseService(&mappingService)
}

type MappingStartData struct {
	OldEpoch  int
	OldEpochN int
	OldEpochK int
	OldEpochT int
	NewEpoch  int
	NewEpochN int
	NewEpochK int
	NewEpochT int
}

func NewDKGMappingDataSource(e eventbus.Bus) *DKGMappingDataSource {
	ds := &DKGMappingDataSource{}
	ds.SetEventBus(e)
	ds.serviceLibrary = NewServiceLibrary(e, "dkgMappingDataSource")
	return ds
}

type DKGMappingDataSource struct {
	eventBus       eventbus.Bus
	serviceLibrary ServiceLibrary
	MappingNode    *mapping.MappingNode
}

func (ds *DKGMappingDataSource) Init() {}
func (ds *DKGMappingDataSource) SetEventBus(e eventbus.Bus) {
	ds.eventBus = e
}
func (ds *DKGMappingDataSource) GetType() string {
	return "dkgMappingDataSource"
}
func (ds *DKGMappingDataSource) SetMappingNode(m *mapping.MappingNode) error {
	ds.MappingNode = m
	return nil
}
func (ds *DKGMappingDataSource) RetrieveKeyMapping(index big.Int) (mapping.MappingKey, error) {
	keyDetails, err := NewServiceLibrary(ds.eventBus, "dkgMappingDataSource").ABCIMethods().RetrieveKeyMapping(index)
	var mappingKeyAssignmentPublic mapping.MappingKey
	if err != nil {
		return mappingKeyAssignmentPublic, err
	}
	mappingKeyAssignmentPublic.Index = keyDetails.Index
	mappingKeyAssignmentPublic.PublicKey = keyDetails.PublicKey
	mappingKeyAssignmentPublic.Threshold = keyDetails.Threshold
	mappingKeyAssignmentPublic.Verifiers = keyDetails.Verifiers
	return mappingKeyAssignmentPublic, nil
}

func NewDKGMappingTransport(e eventbus.Bus, oldEpoch, newEpoch int) *DKGMappingTransport {
	tp := &DKGMappingTransport{}
	tp.SetEventBus(e)
	tp.serviceLibrary = NewServiceLibrary(tp.eventBus, "dkgMappingTransport")
	tp.Prefix = tp.serviceLibrary.MappingMethods().GetMappingProtocolPrefix(oldEpoch, newEpoch)
	return tp
}

type DKGMappingTransport struct {
	eventBus       eventbus.Bus
	serviceLibrary ServiceLibrary
	Prefix         MappingProtocolPrefix
	MappingNode    *mapping.MappingNode
}

func (tp *DKGMappingTransport) SetEventBus(e eventbus.Bus) {
	tp.eventBus = e
}
func (tp *DKGMappingTransport) Init() {
	err := tp.serviceLibrary.P2PMethods().SetStreamHandler(string(tp.Prefix), tp.streamHandler)
	if err != nil {
		logging.WithField("DKGMappingTransportPrefix", tp.Prefix).WithError(err).Error("could not set stream handler")
	}
}
func (tp *DKGMappingTransport) GetType() string {
	return "dkgMappingTransport"
}
func (tp *DKGMappingTransport) SetMappingNode(m *mapping.MappingNode) error {
	tp.MappingNode = m
	return nil
}
func (tp *DKGMappingTransport) Sign(s []byte) ([]byte, error) {
	k := tp.serviceLibrary.EthereumMethods().GetSelfPrivateKey()
	return pvss.ECDSASignBytes(s, &k), nil
}
func (tp *DKGMappingTransport) Send(nodeDetails mapping.NodeDetails, mappingMessage mapping.MappingMessage) error {
	// get recipient details
	ethNodeDetails := tp.serviceLibrary.EthereumMethods().GetNodeDetailsByAddress(*crypto.PointToEthAddress(nodeDetails.PubKey))
	pubKey := tp.serviceLibrary.EthereumMethods().GetSelfPublicKey()
	if nodeDetails.PubKey.X.Cmp(&pubKey.X) == 0 && nodeDetails.PubKey.Y.Cmp(&pubKey.Y) == 0 {
		return tp.Receive(nodeDetails, mappingMessage)
	}

	byt, err := bijson.Marshal(mappingMessage)
	if err != nil {
		return err
	}
	p2pMsg := tp.serviceLibrary.P2PMethods().NewP2PMessage(HashToString(byt), false, byt, "transportMappingMessage")
	peerID, err := GetPeerIDFromP2pListenAddress(ethNodeDetails.P2PConnection)
	if err != nil {
		return err
	}
	// sign the data
	signature, err := tp.serviceLibrary.P2PMethods().SignP2PMessage(&p2pMsg)
	if err != nil {
		return errors.New("failed to sign p2pMsg" + err.Error())
	}
	p2pMsg.Sign = signature
	err = retry.Do(func() error {
		err = tp.serviceLibrary.P2PMethods().SendP2PMessage(*peerID, protocol.ID(tp.Prefix), &p2pMsg)
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

	telemetry.IncrementCounter(pcmn.TelemetryConstants.Message.SentMessageCounter, pcmn.TelemetryConstants.Mapping.Prefix)
	return nil
}
func (tp *DKGMappingTransport) Receive(nodeDetails mapping.NodeDetails, mappingMessage mapping.MappingMessage) error {
	telemetry.IncrementCounter(pcmn.TelemetryConstants.Message.ReceivedMessageCounter, pcmn.TelemetryConstants.Mapping.Prefix)

	return tp.MappingNode.ProcessMessage(nodeDetails, mappingMessage)
}
func (tp *DKGMappingTransport) SendBroadcast(mappingMessage mapping.MappingMessage) error {
	_, err := tp.serviceLibrary.TendermintMethods().Broadcast(mappingMessage)
	if err != nil {
		return err
	}
	telemetry.IncrementCounter(pcmn.TelemetryConstants.Broadcast.SentBroadcastCounter, pcmn.TelemetryConstants.Mapping.Prefix)

	return nil
}
func (tp *DKGMappingTransport) ReceiveBroadcast(mappingMessage mapping.MappingMessage) error {
	telemetry.IncrementCounter(pcmn.TelemetryConstants.Broadcast.ReceivedBroadcastCounter, pcmn.TelemetryConstants.Mapping.Prefix)

	return tp.MappingNode.ProcessBroadcastMessage(mappingMessage)
}
func (tp *DKGMappingTransport) Output(inter interface{}) {}

func (tp *DKGMappingTransport) streamHandler(streamMessage StreamMessage) {
	logging.Debug("streamHandler receiving message")
	// get request data
	p2pBasicMsg := streamMessage.Message
	if tp.serviceLibrary.P2PMethods().AuthenticateMessage(p2pBasicMsg) != nil {
		logging.WithField("p2pBasicMsg", p2pBasicMsg).Error("could not authenticate incoming p2pBasicMsg iin dkgmappingtransport")
		return
	}

	var mappingMessage mapping.MappingMessage
	err := bijson.Unmarshal(p2pBasicMsg.Payload, &mappingMessage)
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
	ethNodeDetails := tp.serviceLibrary.EthereumMethods().GetNodeDetailsByAddress(*crypto.PointToEthAddress(pubKey))
	err = tp.Receive(mapping.NodeDetails(pcmn.Node{
		Index:  int(ethNodeDetails.Index.Int64()),
		PubKey: pubKey,
	}), mappingMessage)

	if err != nil {
		logging.WithError(err).Error("error when received message")
		return
	}
}

type FreezeStateSyncMap struct {
	sync.Map
}

type FreezeStateData struct {
	FreezeState         int
	LastUnassignedIndex uint
}

func (f *FreezeStateSyncMap) Get(mappingID mapping.MappingID) (freezeStateData FreezeStateData) {
	resInter, ok := f.Map.Load(mappingID)
	if !ok {
		return
	}
	res, ok := resInter.(FreezeStateData)
	if !ok {
		return
	}
	return res
}
func (f *FreezeStateSyncMap) Set(mappingID mapping.MappingID, freezeStateData FreezeStateData) {
	f.Map.Store(mappingID, freezeStateData)
}
