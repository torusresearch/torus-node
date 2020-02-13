package mapping

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/torusresearch/bijson"
	"github.com/torusresearch/torus-common/common"
	"github.com/torusresearch/torus-common/secp256k1"
	pcmn "github.com/torusresearch/torus-node/common"
	"github.com/torusresearch/torus-node/idmutex"
	"github.com/torusresearch/torus-node/pvss"
)

type hexstring string

func TestOptimisticMapping(t *testing.T) {
	numberOfKeys := 1000
	oldEpoch := 1
	oldEpochN := 5
	oldEpochK := 3
	oldEpochT := 1
	newEpoch := 2
	newEpochN := 9
	newEpochK := 5
	newEpochT := 2
	mockState := MockTMEngineState{
		MappingProposeFreezes:  make(map[MappingID]map[NodeDetailsID]bool),
		MappingProposeSummarys: make(map[MappingID]map[TransferSummaryID]map[NodeDetailsID]bool),
		MappingProposeKeys:     make(map[MappingID]map[MappingKeyID]map[NodeDetailsID]bool),
	}
	outputCh, keyMappings, verifiers, _, oldNodes, newNodes := setupTestNodes(
		oldEpoch,
		oldEpochN,
		oldEpochK,
		oldEpochT,
		newEpoch,
		newEpochN,
		newEpochK,
		newEpochT,
		numberOfKeys,
		&mockState,
	)
	mockState.OldNodes = oldNodes
	mockState.NewNodes = newNodes
	mockState.OutputCh = outputCh
	mappingIDDetails := MappingIDDetails{
		OldEpoch: oldEpoch,
		NewEpoch: newEpoch,
	}
	mappingID := mappingIDDetails.ToMappingID()
	SeedMappings(keyMappings, verifiers, numberOfKeys)
	for _, oldNode := range oldNodes {
		go func(node *MappingNode) {
			mappingProposeFreezeMessage := MappingProposeFreezeMessage{
				MappingID: mappingID,
			}
			byt, _ := bijson.Marshal(mappingProposeFreezeMessage)
			_ = node.Transport.Send(node.NodeDetails, CreateMappingMessage(MappingMessageRaw{
				MappingID: mappingID,
				Method:    "mapping_propose_freeze",
				Data:      byt,
			}))
		}(oldNode)
	}
	mappingSummaryConfirmedCount := 0
	mappingKeyConfirmedCount := 0
	for output := range outputCh {
		outputStr, ok := output.(string)
		if !ok {
			t.Fatal("could not parse output into string")
		}
		if outputStr == "mapping_summary_confirmed" {
			mappingSummaryConfirmedCount++
		} else if outputStr == "mapping_key_confirmed" {
			mappingKeyConfirmedCount++
		} else {
			t.Fatal("unexpected string output " + outputStr)
		}
		if mappingSummaryConfirmedCount == 1 && mappingKeyConfirmedCount == numberOfKeys {
			break
		}
	}
}

func TestOfflineMapping(t *testing.T) {
	numberOfKeys := 1000
	oldEpoch := 1
	oldEpochN := 5
	oldEpochK := 3
	oldEpochT := 1
	newEpoch := 2
	newEpochN := 9
	newEpochK := 5
	newEpochT := 2

	offlineNodes := 1

	mockState := MockTMEngineState{
		MappingProposeFreezes:  make(map[MappingID]map[NodeDetailsID]bool),
		MappingProposeSummarys: make(map[MappingID]map[TransferSummaryID]map[NodeDetailsID]bool),
		MappingProposeKeys:     make(map[MappingID]map[MappingKeyID]map[NodeDetailsID]bool),
	}
	outputCh, keyMappings, verifiers, localNodeDirectory, oldNodes, newNodes := setupTestNodes(
		oldEpoch,
		oldEpochN,
		oldEpochK,
		oldEpochT,
		newEpoch,
		newEpochN,
		newEpochK,
		newEpochT,
		numberOfKeys,
		&mockState,
	)
	for i := 0; i < offlineNodes; i++ {
		offlineTransport := &LocalOfflineMappingTransport{}
		oldNodes[i].Transport = offlineTransport
		_ = oldNodes[i].Transport.SetMappingNode(oldNodes[i])
		oldNodes[i].Transport.Init()
		localNodeDirectory[oldNodes[i].NodeDetails.ToNodeDetailsID()] = offlineTransport
	}
	mockState.OldNodes = oldNodes
	mockState.NewNodes = newNodes
	mockState.OutputCh = outputCh
	mappingIDDetails := MappingIDDetails{
		OldEpoch: oldEpoch,
		NewEpoch: newEpoch,
	}
	mappingID := mappingIDDetails.ToMappingID()
	SeedMappings(keyMappings, verifiers, numberOfKeys)
	for _, oldNode := range oldNodes {
		go func(node *MappingNode) {
			mappingProposeFreezeMessage := MappingProposeFreezeMessage{
				MappingID: mappingID,
			}
			byt, _ := bijson.Marshal(mappingProposeFreezeMessage)
			_ = node.Transport.Send(node.NodeDetails, CreateMappingMessage(MappingMessageRaw{
				MappingID: mappingID,
				Method:    "mapping_propose_freeze",
				Data:      byt,
			}))
		}(oldNode)
	}
	mappingSummaryConfirmedCount := 0
	mappingKeyConfirmedCount := 0
	for output := range outputCh {
		outputStr, ok := output.(string)
		if !ok {
			t.Fatal("could not parse output into string")
		}
		if outputStr == "mapping_summary_confirmed" {
			mappingSummaryConfirmedCount++
		} else if outputStr == "mapping_key_confirmed" {
			mappingKeyConfirmedCount++
		} else {
			t.Fatal("unexpected string output " + outputStr)
		}
		if mappingSummaryConfirmedCount == 1 && mappingKeyConfirmedCount == numberOfKeys {
			break
		}
	}
}

func SeedMappings(keyMappings map[hexstring]MappingKey, verifiers *[]pcmn.VerifierData, numberOfKeys int) {
	var verifiersArr []pcmn.VerifierData
	for i := 0; i < numberOfKeys; i++ {
		bigIntI := big.NewInt(int64(i))
		verifierMap := make(map[string][]string)
		verifierID := fmt.Sprintf("testemail-%s@gmail.com", bigIntI.Text(16))
		verifierMap["test"] = []string{verifierID}
		keyMappings[hexstring(bigIntI.Text(16))] = MappingKey{
			Index:     *bigIntI,
			PublicKey: common.BigIntToPoint(secp256k1.Curve.ScalarBaseMult(pvss.RandomBigInt().Bytes())),
			Threshold: 1,
			Verifiers: verifierMap,
		}
		verifiersArr = append(verifiersArr, pcmn.VerifierData{
			Ok:         true,
			Verifier:   "test",
			VerifierID: verifierID,
			KeyIndexes: []big.Int{*bigIntI},
			Err:        nil,
		})
	}
	*verifiers = verifiersArr
}

func setupTestNodes(o, oN, oK, oT, n, nN, nK, nT, numberOfKeys int, mockTMEngineState *MockTMEngineState) (
	outputCh chan interface{},
	keyMappings map[hexstring]MappingKey,
	keyVerifiers *[]pcmn.VerifierData,
	localNodeDirectory map[NodeDetailsID]MappingTransport,
	oldNodes []*MappingNode,
	newNodes []*MappingNode,
) {
	mockTMEngine := NewMockTMEngine(mockTMEngineState, &localNodeDirectory, oN, oK, oT, nN, nK, nT, numberOfKeys)
	outputCh = make(chan interface{})
	keyMappings = make(map[hexstring]MappingKey)
	var verifiersArr []pcmn.VerifierData
	keyVerifiers = &verifiersArr
	var oldNodePrivKeys []big.Int
	var newNodePrivKeys []big.Int
	var oldNodePubKeys []common.Point
	var newNodePubKeys []common.Point
	for i := 0; i < oN; i++ {
		oldNodePrivKeys = append(oldNodePrivKeys, *pvss.RandomBigInt())
		oldNodePubKeys = append(oldNodePubKeys, common.BigIntToPoint(secp256k1.Curve.ScalarBaseMult(oldNodePrivKeys[i].Bytes())))
	}
	for i := 0; i < nN; i++ {
		newNodePrivKeys = append(newNodePrivKeys, *pvss.RandomBigInt())
		newNodePubKeys = append(newNodePubKeys, common.BigIntToPoint(secp256k1.Curve.ScalarBaseMult(newNodePrivKeys[i].Bytes())))
	}
	oldNodeIndexes := pcmn.RandIndexes(1, 50, oN)
	var oldNodeList []pcmn.Node
	for i := 0; i < oN; i++ {
		oldNodeList = append(oldNodeList, pcmn.Node{
			Index:  oldNodeIndexes[i],
			PubKey: oldNodePubKeys[i],
		})
	}
	newNodeIndexes := pcmn.RandIndexes(51, 100, nN)
	var newNodeList []pcmn.Node
	for i := 0; i < nN; i++ {
		newNodeList = append(newNodeList, pcmn.Node{
			Index:  newNodeIndexes[i],
			PubKey: newNodePubKeys[i],
		})
	}
	localNodeDirectory = make(map[NodeDetailsID]MappingTransport)
	for i := 0; i < oN; i++ {
		node := pcmn.Node{
			Index:  oldNodeIndexes[i],
			PubKey: oldNodePubKeys[i],
		}
		localMappingTransport := LocalMappingTransport{
			PrivateKey:    oldNodePrivKeys[i],
			PublicKey:     oldNodePubKeys[i],
			NodeDirectory: localNodeDirectory,
			OutputChannel: &outputCh,
			MockTMEngine:  mockTMEngine,
		}
		localMappingDataSource := LocalMappingDataSource{
			KeyMappings: keyMappings,
			Verifiers:   keyVerifiers,
		}
		newMappingNode := NewMappingNode(
			node,
			o,
			oldNodeList,
			oT,
			oK,
			n,
			newNodeList,
			nT,
			nK,
			node.Index,
			&localMappingTransport,
			&localMappingDataSource,
			true,
			false,
		)
		oldNodes = append(oldNodes, newMappingNode)
		nodeDetails := NodeDetails(node)
		localNodeDirectory[nodeDetails.ToNodeDetailsID()] = &localMappingTransport
	}
	for i := 0; i < nN; i++ {
		node := pcmn.Node{
			Index:  newNodeIndexes[i],
			PubKey: newNodePubKeys[i],
		}
		localMappingTransport := LocalMappingTransport{
			PrivateKey:    newNodePrivKeys[i],
			PublicKey:     newNodePubKeys[i],
			NodeDirectory: localNodeDirectory,
			OutputChannel: &outputCh,
			MockTMEngine:  mockTMEngine,
		}
		localMappingDataSource := LocalMappingDataSource{
			KeyMappings: keyMappings,
			Verifiers:   keyVerifiers,
		}
		newMappingNode := NewMappingNode(
			node,
			o,
			oldNodeList,
			oT,
			oK,
			n,
			newNodeList,
			nT,
			nK,
			node.Index,
			&localMappingTransport,
			&localMappingDataSource,
			false,
			true,
		)
		newNodes = append(newNodes, newMappingNode)
		_ = localMappingTransport.SetMappingNode(newMappingNode)
		_ = localMappingDataSource.SetMappingNode(newMappingNode)
		nodeDetails := NodeDetails(node)
		localNodeDirectory[nodeDetails.ToNodeDetailsID()] = &localMappingTransport
	}
	return
}

type LocalMappingTransport struct {
	MappingNode   *MappingNode
	PrivateKey    big.Int
	PublicKey     common.Point
	NodeDirectory map[NodeDetailsID]MappingTransport
	OutputChannel *chan interface{}
	MockTMEngine  *func(NodeDetails, MappingMessage) error
}

func (tp *LocalMappingTransport) Init() {}
func (tp *LocalMappingTransport) GetType() string {
	return "local"
}
func (tp *LocalMappingTransport) SetMappingNode(m *MappingNode) error {
	tp.MappingNode = m
	return nil
}
func (tp *LocalMappingTransport) SetTMEngine(tm *func(NodeDetails, MappingMessage) error) {
	tp.MockTMEngine = tm
}
func (tp *LocalMappingTransport) Send(nodeDetails NodeDetails, mappingMessage MappingMessage) error {
	return tp.NodeDirectory[nodeDetails.ToNodeDetailsID()].Receive(tp.MappingNode.NodeDetails, mappingMessage)
}
func (tp *LocalMappingTransport) Receive(senderDetails NodeDetails, mappingMessage MappingMessage) error {
	return tp.MappingNode.ProcessMessage(senderDetails, mappingMessage)
}
func (tp *LocalMappingTransport) SendBroadcast(mappingMessage MappingMessage) error {
	return (*tp.MockTMEngine)(tp.MappingNode.NodeDetails, mappingMessage)
}
func (tp *LocalMappingTransport) ReceiveBroadcast(mappingMessage MappingMessage) error {
	return tp.MappingNode.ProcessBroadcastMessage(mappingMessage)
}
func (tp *LocalMappingTransport) Output(inter interface{}) {
	*tp.OutputChannel <- inter
}

func (tp *LocalMappingTransport) Sign(byt []byte) (sig []byte, err error) { return }

type LocalOfflineMappingTransport struct {
	MappingNode   *MappingNode
	PrivateKey    big.Int
	PublicKey     common.Point
	NodeDirectory map[NodeDetailsID]MappingTransport
	OutputChannel *chan interface{}
	MockTMEngine  *func(NodeDetails, MappingMessage) error
}

func (tp *LocalOfflineMappingTransport) Init() {}
func (tp *LocalOfflineMappingTransport) GetType() string {
	return "local"
}
func (tp *LocalOfflineMappingTransport) SetMappingNode(m *MappingNode) error {
	tp.MappingNode = m
	return nil
}
func (tp *LocalOfflineMappingTransport) SetTMEngine(tm *func(NodeDetails, MappingMessage) error) {
	tp.MockTMEngine = tm
}
func (tp *LocalOfflineMappingTransport) Send(nodeDetails NodeDetails, mappingMessage MappingMessage) error {
	return nil
}
func (tp *LocalOfflineMappingTransport) Receive(senderDetails NodeDetails, mappingMessage MappingMessage) error {
	return tp.MappingNode.ProcessMessage(senderDetails, mappingMessage)
}
func (tp *LocalOfflineMappingTransport) SendBroadcast(mappingMessage MappingMessage) error {
	return nil
}
func (tp *LocalOfflineMappingTransport) ReceiveBroadcast(mappingMessage MappingMessage) error {
	return tp.MappingNode.ProcessBroadcastMessage(mappingMessage)
}
func (tp *LocalOfflineMappingTransport) Output(inter interface{}) {
	*tp.OutputChannel <- inter
}
func (tp *LocalOfflineMappingTransport) Sign(byt []byte) (sig []byte, err error) { return }

type LocalMappingDataSource struct {
	MappingNode *MappingNode
	KeyMappings map[hexstring]MappingKey
	Verifiers   *[]pcmn.VerifierData
}

func (ds *LocalMappingDataSource) Init() {}
func (ds *LocalMappingDataSource) GetType() string {
	return "local"
}
func (ds *LocalMappingDataSource) SetMappingNode(m *MappingNode) error {
	ds.MappingNode = m
	return nil
}
func (ds *LocalMappingDataSource) RetrieveKeyMapping(index big.Int) (mappingKey MappingKey, err error) {
	mappingKey, ok := ds.KeyMappings[hexstring(index.Text(16))]
	if !ok {
		err = fmt.Errorf("could not find index %v", index)
		return
	}
	return
}

type MockTMEngineState struct {
	idmutex.Mutex
	OldNodes               []*MappingNode
	NewNodes               []*MappingNode
	MappingProposeFreezes  map[MappingID]map[NodeDetailsID]bool
	MappingProposeSummarys map[MappingID]map[TransferSummaryID]map[NodeDetailsID]bool
	MappingProposeKeys     map[MappingID]map[MappingKeyID]map[NodeDetailsID]bool
	OutputCh               chan interface{}
}

func NewMockTMEngine(mockTMEngineState *MockTMEngineState, localNodeDirectory *map[NodeDetailsID]MappingTransport, oN, oK, oT, nN, nK, nT, lastUnassignedIndex int) *func(NodeDetails, MappingMessage) error {
	engineState := mockTMEngineState
	engine := func(senderDetails NodeDetails, mappingMessage MappingMessage) error {
		engineState.Lock()
		defer engineState.Unlock()
		mappingID := mappingMessage.MappingID
		if mappingMessage.Method == "mapping_propose_freeze" {
			var mappingProposeFreezeBroadcastMessage MappingProposeFreezeBroadcastMessage
			err := bijson.Unmarshal(mappingMessage.Data, &mappingProposeFreezeBroadcastMessage)
			if err != nil {
				return err
			}
			proposedMappingID := mappingProposeFreezeBroadcastMessage.MappingID
			if engineState.MappingProposeFreezes[proposedMappingID] == nil {
				engineState.MappingProposeFreezes[proposedMappingID] = make(map[NodeDetailsID]bool)
			}
			engineState.MappingProposeFreezes[proposedMappingID][senderDetails.ToNodeDetailsID()] = true
			if len(engineState.MappingProposeFreezes[proposedMappingID]) == oT+oK {
				go func() {
					for _, oldNode := range engineState.OldNodes {
						mappingTransport := oldNode.Transport
						mappingSummaryMessage := MappingSummaryMessage{
							TransferSummary: TransferSummary{
								LastUnassignedIndex: uint(lastUnassignedIndex),
							},
						}
						byt, _ := bijson.Marshal(mappingSummaryMessage)
						_ = mappingTransport.ReceiveBroadcast(CreateMappingMessage(MappingMessageRaw{
							MappingID: mappingID,
							Method:    "mapping_summary_frozen",
							Data:      byt,
						}))
					}
				}()
			}
			return nil
		} else if mappingMessage.Method == "mapping_summary_broadcast" {
			var mappingSummaryBroadcastMessage MappingSummaryBroadcastMessage
			err := bijson.Unmarshal(mappingMessage.Data, &mappingSummaryBroadcastMessage)
			if err != nil {
				return err
			}
			if engineState.MappingProposeSummarys[mappingID] == nil {
				engineState.MappingProposeSummarys[mappingID] = make(map[TransferSummaryID]map[NodeDetailsID]bool)
			}
			transferSummaryID := mappingSummaryBroadcastMessage.TransferSummary.ID()
			if engineState.MappingProposeSummarys[mappingID][transferSummaryID] == nil {
				engineState.MappingProposeSummarys[mappingID][transferSummaryID] = make(map[NodeDetailsID]bool)
			}
			engineState.MappingProposeSummarys[mappingID][transferSummaryID][senderDetails.ToNodeDetailsID()] = true
			if len(engineState.MappingProposeSummarys[mappingID][transferSummaryID]) == nK {
				engineState.OutputCh <- "mapping_summary_confirmed"
			}
			return nil
		} else if mappingMessage.Method == "mapping_key_broadcast" {
			var mappingKeyBroadcastMessage MappingKeyBroadcastMessage
			err := bijson.Unmarshal(mappingMessage.Data, &mappingKeyBroadcastMessage)
			if err != nil {
				return err
			}
			if engineState.MappingProposeKeys[mappingID] == nil {
				engineState.MappingProposeKeys[mappingID] = make(map[MappingKeyID]map[NodeDetailsID]bool)
			}
			mappingKeyID := mappingKeyBroadcastMessage.MappingKey.ID()
			if engineState.MappingProposeKeys[mappingID][mappingKeyID] == nil {
				engineState.MappingProposeKeys[mappingID][mappingKeyID] = make(map[NodeDetailsID]bool)
			}
			engineState.MappingProposeKeys[mappingID][mappingKeyID][senderDetails.ToNodeDetailsID()] = true
			if len(engineState.MappingProposeKeys[mappingID][mappingKeyID]) == nK {
				engineState.OutputCh <- "mapping_key_confirmed"
			}
			return nil
		}
		return fmt.Errorf("unimplemented method %v", mappingMessage.Method)
	}
	return &engine
}
