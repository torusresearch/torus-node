package mapping

import (
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/torusresearch/torus-common/common"
	"github.com/torusresearch/torus-node/config"
	"github.com/torusresearch/torus-node/telemetry"

	logging "github.com/sirupsen/logrus"
	"github.com/torusresearch/bijson"
	pcmn "github.com/torusresearch/torus-node/common"
)

// ProcessMessage is called when the transport for the node receives a message via direct send.
// It works similar to a router, processing different messages differently based on their associated method.
// Each method handler's code path consists of parsing the message -> state checks -> logic -> state updates.
// Defer state changes until the end of the function call to ensure that the state is consistent in the handler.
// When sending messages to other nodes, it's important to use a goroutine to ensure that it isnt synchronous
// We assume that senderDetails have already been validated by the transport and are always correct
func (mappingNode *MappingNode) ProcessMessage(senderDetails NodeDetails, mappingMessage MappingMessage) error {
	mappingNode.Lock()
	defer mappingNode.Unlock()
	if mappingMessage.Method == "mapping_propose_freeze" {
		if !mappingNode.IsOldNode {
			return errors.New("mapping propose freeze must be handled by old nodes")
		}
		var mappingProposeFreezeMessage MappingProposeFreezeMessage
		err := bijson.Unmarshal(mappingMessage.Data, &mappingProposeFreezeMessage)
		if err != nil {
			return err
		}
		if senderDetails.ToNodeDetailsID() != mappingNode.NodeDetails.ToNodeDetailsID() {
			return errors.New("propose freeze message can only be sent by the node itself")
		}
		if mappingProposeFreezeMessage.MappingID != mappingNode.getMappingID() {
			return fmt.Errorf("unexpected mappingID %v %v", mappingProposeFreezeMessage.MappingID, mappingNode.getMappingID())
		}
		mappingProposeFreezeBroadcastMessage := MappingProposeFreezeBroadcastMessage(mappingProposeFreezeMessage)
		byt, err := bijson.Marshal(mappingProposeFreezeBroadcastMessage)
		if err != nil {
			return err
		}
		go func() {
			err := mappingNode.Transport.SendBroadcast(CreateMappingMessage(MappingMessageRaw{
				MappingID: mappingNode.getMappingID(),
				Method:    "mapping_propose_freeze",
				Data:      byt,
			}))
			if err != nil {
				logging.WithError(err).Error("could not mapping propose freeze")
			}
		}()
		return nil
	} else if mappingMessage.Method == "mapping_summary" {
		if !mappingNode.IsNewNode {
			return errors.New("mapping propose freeze must be handled by new nodes")
		}

		// Add to metrics
		telemetry.IncrementCounter(pcmn.TelemetryConstants.Mapping.ReceiveSummaryMessage, pcmn.TelemetryConstants.Mapping.Prefix)

		var mappingSummaryMessage MappingSummaryMessage
		err := bijson.Unmarshal(mappingMessage.Data, &mappingSummaryMessage)
		if err != nil {
			return err
		}
		transferSummary := mappingSummaryMessage.TransferSummary
		if !mappingNode.OldNodes.NodeExists(senderDetails) {
			return fmt.Errorf("Could not validate that sender %v was in previous node network  for mapping summary", senderDetails)
		}
		if mappingNode.MappingSummary.SendCount.Get(senderDetails.ToNodeDetailsID()) != 0 {
			return fmt.Errorf("Sender %v already sent mapping frozen message", senderDetails)
		}
		transferSummaryID := transferSummary.ID()
		mappingNode.MappingSummary.MappingSummaryStore.Set(transferSummaryID, transferSummary)
		mappingNode.MappingSummary.SendCount.Set(senderDetails.ToNodeDetailsID(), 1)
		mappingNode.MappingSummary.MappingSummaryCount.SyncUpdate(transferSummaryID, func(valueInMap int) (newValue int) {
			newValue = valueInMap + 1
			if newValue == mappingNode.OldNodes.K {
				mappingSummaryBroadcastMessage := MappingSummaryBroadcastMessage{
					TransferSummary: transferSummary,
				}
				byt, err := bijson.Marshal(mappingSummaryBroadcastMessage)
				if err != nil {
					logging.WithError(err).Error("could not marshal mapping summary broadcast message")
					return
				}
				go func() {

					// Add telemetry
					telemetry.IncrementCounter(pcmn.TelemetryConstants.Mapping.SummarySendBroadcast, pcmn.TelemetryConstants.Mapping.Prefix)

					err := mappingNode.Transport.SendBroadcast(CreateMappingMessage(MappingMessageRaw{
						MappingID: mappingNode.getMappingID(),
						Method:    "mapping_summary_broadcast",
						Data:      byt,
					}))
					if err != nil {
						logging.WithError(err).Error("Could not send broadcast for mapping_summary_send_broadcast")
					}
				}()
			}
			return
		})
		return nil
	} else if mappingMessage.Method == "mapping_key" {
		if !mappingNode.IsNewNode {
			return errors.New("mapping propose freeze must be handled by new nodes")
		}

		// Add to metrics
		telemetry.IncrementCounter(pcmn.TelemetryConstants.Mapping.ReceiveKeyMessage, pcmn.TelemetryConstants.Mapping.Prefix)

		var mappingKeyMessage MappingKeyMessage
		err := bijson.Unmarshal(mappingMessage.Data, &mappingKeyMessage)
		if err != nil {
			return err
		}
		mappingKey := mappingKeyMessage.MappingKey
		if !mappingNode.OldNodes.NodeExists(senderDetails) {
			return fmt.Errorf("Could not validate that sender %v was in previous node network  for mapping key", senderDetails)
		}
		if mappingNode.MappingKeys.SendCount.Get(mappingKey.ID()) == nil {
			mappingNode.MappingKeys.SendCount.Set(mappingKey.ID(), NewNodeDetailsIDToIntCMap())
		}
		if mappingNode.MappingKeys.SendCount.Get(mappingKey.ID()).Get(senderDetails.ToNodeDetailsID()) != 0 {
			return fmt.Errorf("Sender %v already sent this mapping key message %v", senderDetails, mappingKeyMessage)
		}
		mappingNode.MappingKeys.SendCount.Get(mappingKey.ID()).Set(senderDetails.ToNodeDetailsID(), 1)
		mappingNode.MappingKeys.SendCount.SyncUpdate(mappingKey.ID(), func(valueInMap *NodeDetailsIDToIntCMap) (newValue *NodeDetailsIDToIntCMap) {
			newValue = valueInMap
			if valueInMap.Count() == mappingNode.OldNodes.K {
				mappingKeyBroadcastMessage := MappingKeyBroadcastMessage{
					MappingKey: mappingKey,
				}
				byt, err := bijson.Marshal(mappingKeyBroadcastMessage)
				if err != nil {
					logging.WithError(err).Error("could not marshal mapping key broadcast message")
					return
				}
				go func() {

					telemetry.IncrementCounter(pcmn.TelemetryConstants.Mapping.KeySendBroadcast, pcmn.TelemetryConstants.Mapping.Prefix)

					err = mappingNode.Transport.SendBroadcast(CreateMappingMessage(MappingMessageRaw{
						MappingID: mappingNode.getMappingID(),
						Method:    "mapping_key_broadcast",
						Data:      byt,
					}))
					if err != nil {
						logging.WithError(err).Error("Could not send broadcast for mapping_key_send_broadcast")
					}
				}()
			}
			return
		})
		return nil
	}
	logging.WithField("mappingMessage", mappingMessage).Error("Could not find method in processMessage for mappingNode")
	return errors.New("Unimplemented method in mappingNode processMessage")
}

// ProcessBroadcastMessage is called when the node receives a message via broadcast (eg. Tendermint)
func (mappingNode *MappingNode) ProcessBroadcastMessage(mappingMessage MappingMessage) error {
	mappingNode.Lock()
	defer mappingNode.Unlock()
	if mappingMessage.Method == "mapping_summary_frozen" {
		if !mappingNode.IsOldNode {
			return errors.New("mapping propose freeze must be handled by old nodes")
		}

		// Add to metrics
		telemetry.IncrementCounter(pcmn.TelemetryConstants.Mapping.ReceiveFrozenMessage, pcmn.TelemetryConstants.Mapping.Prefix)

		var mappingSummaryMessage MappingSummaryMessage
		err := bijson.Unmarshal(mappingMessage.Data, &mappingSummaryMessage)
		if err != nil {
			return err
		}
		for _, node := range mappingNode.NewNodes.Nodes {

			// Add to metrics
			telemetry.IncrementCounter(pcmn.TelemetryConstants.Mapping.SendSummary, pcmn.TelemetryConstants.Mapping.Prefix)
			go func(node NodeDetails) {
				err := mappingNode.Transport.Send(node, CreateMappingMessage(MappingMessageRaw{
					MappingID: mappingNode.getMappingID(),
					Method:    "mapping_summary",
					Data:      mappingMessage.Data,
				}))
				if err != nil {
					logging.WithField("message", stringify(mappingMessage)).WithError(err).Error("error when sending mapping message")
				}
			}(node)
		}

		go func() {
			for i := 0; i < int(mappingSummaryMessage.TransferSummary.LastUnassignedIndex); i++ {
				time.Sleep(time.Duration(config.GlobalMutableConfig.GetI("PSSShareDelayMS")) * time.Millisecond)
				key, err := mappingNode.DataSource.RetrieveKeyMapping(*big.NewInt(int64(i)))
				if err != nil {
					logging.WithField("index", i).WithError(err).Error("could not retrieve any key for index")
					key = MappingKey{
						Index:     *big.NewInt(int64(i)),
						PublicKey: common.Point{X: *big.NewInt(0), Y: *big.NewInt(0)},
						Threshold: 1,
						Verifiers: make(map[string][]string),
					}
				}

				mappingKeyMessage := MappingKeyMessage{
					MappingKey: key,
				}
				byt, err := bijson.Marshal(mappingKeyMessage)
				if err != nil {
					logging.WithField("index", i).WithField("key", key).Error("could not marshal key")
					continue
				}

				for _, node := range mappingNode.NewNodes.Nodes {

					// Add to metrics
					telemetry.IncrementCounter(pcmn.TelemetryConstants.Mapping.SendKey, pcmn.TelemetryConstants.Mapping.Prefix)
					go func(node NodeDetails) {
						err := mappingNode.Transport.Send(node, CreateMappingMessage(MappingMessageRaw{
							MappingID: mappingNode.getMappingID(),
							Method:    "mapping_key",
							Data:      byt,
						}))
						if err != nil {
							logging.WithError(err).Error("could not send mapping key")
						}
					}(node)
				}
			}
		}()

		return nil
	}
	logging.WithField("mappingMessage", mappingMessage).Error("Could not find method in processBroadcastMessage for mappingNode")
	return errors.New("Unimplemented method in mappingNode processBroadcastMessage")
}

func (mappingNode *MappingNode) getMappingID() MappingID {
	return (&MappingIDDetails{
		OldEpoch: mappingNode.OldNodes.EpochID,
		NewEpoch: mappingNode.NewNodes.EpochID,
	}).ToMappingID()
}

func NewMappingNode(
	nodeDetails pcmn.Node,
	oldEpoch int,
	oldNodeList []pcmn.Node,
	oldNodesT int,
	oldNodesK int,
	newEpoch int,
	newNodeList []pcmn.Node,
	newNodesT int,
	newNodesK int,
	nodeIndex int,
	transport MappingTransport,
	dataSource MappingDataSource,
	isOldNode bool,
	isNewNode bool,
) *MappingNode {
	newMappingNode := &MappingNode{
		NodeDetails: NodeDetails(nodeDetails),
		OldNodes: NodeNetwork{
			Nodes:   mapFromNodeList(oldNodeList),
			N:       len(oldNodeList),
			T:       oldNodesT,
			K:       oldNodesK,
			EpochID: oldEpoch,
		},
		NewNodes: NodeNetwork{
			Nodes:   mapFromNodeList(newNodeList),
			N:       len(newNodeList),
			T:       newNodesT,
			K:       newNodesK,
			EpochID: newEpoch,
		},
		NodeIndex: nodeIndex,
		MappingSummary: &MappingSummary{
			SendCount:           NewNodeDetailsToIntCMap(),
			MappingSummaryStore: NewTransferSummaryIDToTransferSummaryCMap(),
			MappingSummaryCount: NewTransferSummaryToIntCMap(),
		},
		MappingKeys: &MappingKeys{
			SendCount: NewMappingKeyIDToNodeDetailsIDToIntCMap(),
		},
		IsOldNode: isOldNode,
		IsNewNode: isNewNode,
	}
	transport.Init()
	err := transport.SetMappingNode(newMappingNode)
	if err != nil {
		logging.WithError(err).Error("could not set mapping node for transport")
	}
	dataSource.Init()
	err = dataSource.SetMappingNode(newMappingNode)
	if err != nil {
		logging.WithError(err).Error("Could not set mapping node for data source")
	}
	newMappingNode.Transport = transport
	newMappingNode.DataSource = dataSource
	return newMappingNode
}
