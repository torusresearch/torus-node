package dkgnode

import (
	"math/big"
	"strconv"
	"time"

	"github.com/torusresearch/torus-public/logging"
)

type NodeListUpdates struct {
	Type    string
	Payload interface{}
}

type KeyGenUpdates struct {
	Type    string
	Payload interface{}
}

func startKeyGenerationMonitor(suite *Suite, keyGenMonitorUpdates chan KeyGenUpdates) {
	for {
		logging.Debug("KEYGEN: in start keygen monitor", suite.ABCIApp.state.LocalStatus)
		time.Sleep(1 * time.Second)
		if suite.ABCIApp.state.LocalStatus["all_initiate_keygen"] != "" {
			logging.Debug("KEYGEN: WAITING FOR ALL INITIATE KEYGEN TO STOP BEING IN PROGRESS", suite.ABCIApp.state.LocalStatus)
			continue
		}
		percentLeft := 100 * (suite.ABCIApp.state.LastCreatedIndex - suite.ABCIApp.state.LastUnassignedIndex) / uint(suite.Config.KeysPerEpoch)
		if percentLeft > uint(suite.Config.KeyBufferTriggerPercentage) {
			logging.Debug("KEYGEN: keygeneration trigger percent left not reached", percentLeft)
			continue
		}
		startingIndex := int(suite.ABCIApp.state.LastCreatedIndex)
		endingIndex := suite.Config.KeysPerEpoch + int(suite.ABCIApp.state.LastCreatedIndex)

		logging.Debug("KEYGEN: we are starting keygen", suite.ABCIApp.state.LocalStatus)
		nodeIndex, err := matchNode(suite, suite.EthSuite.NodePublicKey.X.Text(16), suite.EthSuite.NodePublicKey.Y.Text(16))
		if err != nil {
			logging.Error("KEYGEN: could not get nodeIndex", err)
			continue
		}
		go listenForShares(suite, suite.Config.KeysPerEpoch*suite.Config.NumberOfNodes*suite.Config.NumberOfNodes)
		for {
			time.Sleep(1 * time.Second)
			initiateKeyGenerationStatusWrapper := DefaultBFTTxWrapper{
				StatusBFTTx{
					FromPubKeyX: suite.EthSuite.NodePublicKey.X.Text(16),
					FromPubKeyY: suite.EthSuite.NodePublicKey.Y.Text(16),
					Epoch:       suite.ABCIApp.state.Epoch,
					StatusType:  "initiate_keygen",
					StatusValue: "Y",
					Data:        []byte(strconv.Itoa(endingIndex)),
				},
			}
			_, err := suite.BftSuite.BftRPC.Broadcast(initiateKeyGenerationStatusWrapper)
			if err != nil {
				logging.Error("KEYGEN: could not broadcast initiateKeygeneration", err)
			}
			selfInitiateStatus, selfInitiateStatusFound := suite.ABCIApp.state.NodeStatus[uint(nodeIndex)]["initiate_keygen"]
			allInitiateStatus, allInitiateStatusFound := suite.ABCIApp.state.LocalStatus["all_initiate_keygen"]

			// TODO: expecting keygen process to take longer than 1 second
			if (selfInitiateStatusFound && selfInitiateStatus == "Y") || (allInitiateStatusFound && (allInitiateStatus == "Y" || allInitiateStatus == "IP")) {
				break
			}
		}
		for {
			logging.Debug("KEYGEN: WAITING FOR ALL INITIATE KEYGEN TO BE Y", suite.ABCIApp.state.LocalStatus)
			logging.Debug(suite.ABCIApp.state)
			time.Sleep(1 * time.Second)
			if suite.ABCIApp.state.LocalStatus["all_initiate_keygen"] == "Y" {
				logging.Debug("STATUSTX: localstatus all initiate keygen is Y, appstate", suite.ABCIApp.state.LocalStatus)
				//reset keygen flag
				suite.ABCIApp.state.LocalStatus["all_initiate_keygen"] = "IP"
				//report back to main process
				keyGenMonitorUpdates <- KeyGenUpdates{
					Type:    "start_keygen",
					Payload: []int{startingIndex, endingIndex},
				}
				break
			}
		}

	}
}

func startNodeListMonitor(suite *Suite, nodeListUpdates chan NodeListUpdates) {
	for {
		logging.Debug("Checking Node List...")
		// Fetch Node List from contract address
		//TODO: make epoch vairable
		epoch := big.NewInt(int64(0))
		ethList, positions, err := suite.EthSuite.NodeListContract.ViewNodes(nil, epoch)
		// If we can't reach ethereum node, lets try next time
		if err != nil {
			logging.Error("Could not View Nodes on ETH Network", err)
		} else {
			// Build count of nodes connected to
			logging.Debug("Indexes", positions, ethList)
			connectedNodes := 0
			nodeList := make([]*NodeReference, len(ethList))
			if len(ethList) > 0 {
				for i := range ethList {
					// Check if node is online by pinging
					temp, err := connectToJSONRPCNode(suite, *epoch, ethList[i])
					if err != nil {
						logging.Error(err)
					}

					if temp != nil {
						if nodeList[int(temp.Index.Int64())-1] == nil { // TODO: use mapping
							nodeList[int(temp.Index.Int64())-1] = temp
						}
						connectedNodes++
					}
				}
			}
			//if we've connected to all nodes we send back the most recent list
			if connectedNodes == len(ethList) {
				nodeListUpdates <- NodeListUpdates{"update", nodeList}
			}
		}
		time.Sleep(1 * time.Second) //check node list every second for updates
	}
}
