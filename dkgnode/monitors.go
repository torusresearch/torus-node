package dkgnode

import (
	"math/big"
	// "strconv"
	"time"

	"github.com/torusresearch/torus-public/logging"
	"github.com/torusresearch/torus-public/pss"
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
		// logging.Debugf("KEYGEN: in start keygen monitor %s", suite.LocalStatus)
		time.Sleep(1 * time.Second)
		// if suite.LocalStatus["all_initiate_keygen"] != "" {
		var localStatus = suite.LocalStatus.Current()
		if localStatus == "running_keygen" {
			// logging.Debugf("KEYGEN: WAITING FOR ALL INITIATE KEYGEN TO STOP BEING IN PROGRESS %s", suite.LocalStatus.Current())
			continue
		} else {
			// logging.Debugf("KEYGEN: KEYGEN NOT IN PROGRESS %s", localStatus)
		}

		percentLeft := 100 * (suite.ABCIApp.state.LastCreatedIndex - suite.ABCIApp.state.LastUnassignedIndex) / uint(suite.Config.KeysPerEpoch)
		if percentLeft > uint(suite.Config.KeyBufferTriggerPercentage) {
			logging.Debugf("KEYGEN: keygeneration trigger percent left not reached %d", percentLeft)
			continue
		}
		startingIndex := int(suite.ABCIApp.state.LastCreatedIndex)
		endingIndex := suite.Config.KeysPerEpoch + int(suite.ABCIApp.state.LastCreatedIndex)

		logging.Debugf("KEYGEN: we are starting keygen %v", suite.LocalStatus)

		//report back to main process
		keyGenMonitorUpdates <- KeyGenUpdates{
			Type:    "start_keygen",
			Payload: []int{startingIndex, endingIndex},
		}
	}
}

func startNodeListMonitor(suite *Suite, tickerChan <-chan time.Time, nodeListUpdates chan NodeListUpdates) {
	logging.Info("Started Node List Monitor")
	for _ = range tickerChan {
		logging.Debug("Checking Node List...")
		// Fetch Node List from contract address
		//TODO: make epoch vairable
		epoch := big.NewInt(int64(0))
		ethList, positions, err := suite.EthSuite.NodeListContract.ViewNodes(nil, epoch)
		// If we can't reach ethereum node, lets try next time
		if err != nil {
			logging.Errorf("Could not View Nodes on ETH Network %s", err)
		} else {
			// Build count of nodes connected to
			logging.Debugf("Indexes %v %v", positions, ethList)
			connectedNodes := 0
			nodeList := make([]*NodeReference, len(ethList))
			if len(ethList) > 0 {
				for _, ethListNode := range ethList {
					// Check if node is online by pinging
					temp, err := connectToP2PNode(suite, *epoch, ethListNode)
					if err != nil {
						logging.Errorf("%v", err)
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
	}

}

var MessageSourceMapping = map[MessageSource]string{
	BFT: "BFT",
	P2P: "P2P",
}

type MessageSource int

const (
	BFT MessageSource = iota
	P2P
)

type PSSWorkerUpdate struct {
	Type    string
	Payload interface{}
}

type BftWorkerUpdates struct {
	Type    string
	Payload interface{}
}

type WhitelistMonitorUpdates struct {
	Type    string
	Payload interface{}
}

type PSSUpdate struct {
	MessageSource MessageSource
	NodeDetails   *NodeReference
	PSSMessage    pss.PSSMessage
}

// func whitelistMonitor(suite *Suite, tickerChan <-chan time.Time, whitelistMonitorMsgs chan<- WhitelistMonitorUpdates) {
// 	for range tickerChan {
// 		currentEpoch := suite.EthSuite.CurrentEpoch
// 		fmt.Println("SUITE ETHSUITE CURRENT EPOCH IS WHAT")
// 		fmt.Println(currentEpoch)
// 		isWhitelisted, err := suite.EthSuite.NodeListContract.ViewWhitelist(nil, big.NewInt(int64(currentEpoch)), *suite.EthSuite.NodeAddress)
// 		if err != nil {
// 			logging.Errorf("Could not check ethereum whitelist: %s", err.Error())
// 		}
// 		if isWhitelisted {
// 			whitelistMonitorMsgs <- WhitelistMonitorUpdates{"node_whitelisted", currentEpoch}
// 			break
// 		}
// 		logging.Warning("Node is not whitelisted")
// 	}
// }

// func keyGenMonitor(suite *Suite, tmCoreMsgs <-chan string, tickerChan <-chan time.Time, keyGenMonitorMsgs chan<- KeyGenUpdates) {
// 	logging.Info("Started keygeneration monitor")
// 	for coreMsg := range tmCoreMsgs {
// 		logging.Debugf("received: %s", coreMsg)
// 		if coreMsg == "started_tmcore" {
// 			// Start key generation monitor when bft is done setting up
// 			nodeRegister := suite.EthSuite.EpochNodeRegister[suite.EthSuite.CurrentEpoch]
// 			logging.Infof("Start Key generation monitor: %v, %v", nodeRegister.NodeList, suite.BftSuite.BftRPC)
// 			break
// 		}
// 	}
// 	for range tickerChan {
// 		// logging.Debugf("KEYGEN: in start keygen monitor %s", suite.LocalStatus)
// 		// if suite.LocalStatus["all_initiate_keygen"] != "" {
// 		var localStatus = suite.LocalStatus.Current()
// 		if localStatus == "running_keygen" {
// 			// logging.Debugf("KEYGEN: WAITING FOR ALL INITIATE KEYGEN TO STOP BEING IN PROGRESS %s", suite.LocalStatus.Current())
// 			continue
// 		} else {
// 			// logging.Debugf("KEYGEN: KEYGEN NOT IN PROGRESS %s", localStatus)
// 		}

// 		percentLeft := 100 * (suite.ABCIApp.state.LastCreatedIndex - suite.ABCIApp.state.LastUnassignedIndex) / uint(suite.Config.KeysPerEpoch)
// 		if percentLeft > uint(suite.Config.KeyBufferTriggerPercentage) {
// 			logging.Debugf("KEYGEN: keygeneration trigger percent left not reached %d", percentLeft)
// 			continue
// 		}
// 		startingIndex := int(suite.ABCIApp.state.LastCreatedIndex)
// 		endingIndex := suite.Config.KeysPerEpoch + int(suite.ABCIApp.state.LastCreatedIndex)

// 		logging.Debugf("KEYGEN: we are starting keygen %v", suite.LocalStatus)

// 		//report back to main process
// 		keyGenMonitorMsgs <- KeyGenUpdates{
// 			Type:    "start_keygen",
// 			Payload: []int{startingIndex, endingIndex},
// 		}
// 	}
// }

// func nodeListMonitor(suite *Suite, tickerChan <-chan time.Time, nodeListUpdates chan<- NodeListUpdates, pssWorkerUpdates chan<- PSSWorkerUpdate) {
// 	logging.Info("Started Node List Monitor")
// 	for range tickerChan {
// 		logging.Debug("Checking Node List...")
// 		epoch := suite.EthSuite.CurrentEpoch
// 		if _, ok := suite.EthSuite.EpochNodeRegister[epoch]; !ok {
// 			suite.EthSuite.EpochNodeRegister[epoch] = &NodeRegister{}
// 		}
// 		// Get current nodes
// 		ethList, positions, err := suite.EthSuite.NodeListContract.ViewNodes(nil, big.NewInt(int64(epoch)))
// 		// If we can't reach ethereum node, lets try next time
// 		if err != nil {
// 			logging.Errorf("Could not view Current Nodes on ETH Network %s", err)
// 			continue
// 		}
// 		// Build count of nodes connected to
// 		logging.Debugf("Indexes %v %v", positions, ethList)
// 		if len(ethList) != suite.Config.NumberOfNodes {
// 			logging.Debug("ethList length not equal to total number of nodes in config...")
// 			continue
// 		}
// 		connectedNodes := make([]*NodeReference, len(ethList))
// 		for _, ethListNode := range ethList {
// 			// Check if node is online by pinging
// 			temp, err := connectToP2PNode(suite, *big.NewInt(int64(epoch)), ethListNode)
// 			if err != nil {
// 				logging.Errorf("%v", err)
// 			}

// 			if temp != nil {
// 				if connectedNodes[int(temp.Index.Int64())-1] == nil { // TODO: use mapping?
// 					connectedNodes[int(temp.Index.Int64())-1] = temp
// 				}
// 			}
// 		}
// 		// if we've connected to all nodes we send back the most recent list
// 		if len(connectedNodes) == len(ethList) {
// 			nodeRegister := suite.EthSuite.EpochNodeRegister[epoch]
// 			(*nodeRegister).NodeList = connectedNodes
// 			nodeListUpdates <- NodeListUpdates{"all_connected", connectedNodes}
// 			pssWorkerUpdates <- PSSWorkerUpdate{"all_connected", connectedNodes}
// 		}

// 		// Get previous nodes
// 		// if epoch >= 2 {
// 		// 	prevEthList, prevPositions, err := suite.EthSuite.NodeListContract.ViewNodes(nil, big.NewInt(int64(epoch-1)))
// 		// 	if err != nil {
// 		// 		logging.Errorf("Could not view previous nodes on ETH Network %s", err)
// 		// 	} else {
// 		// 		logging.Debugf("Indexes %v %v", positions, ethList)
// 		// 		nodeList := make([]*NodeReference, len(ethList))
// 		// 		if len(ethList) > 0 {
// 		// 			for _, ethListNode := range ethList {
// 		// 				// Check if node is online by pinging
// 		// 				temp, err := connectToP2PNode(suite, *big.NewInt(int64(epoch)), ethListNode)
// 		// 				if err != nil {
// 		// 					logging.Errorf("%v", err)
// 		// 				}

// 		// 				if temp != nil {
// 		// 					if nodeList[int(temp.Index.Int64())-1] == nil { // TODO: use mapping?
// 		// 						nodeList[int(temp.Index.Int64())-1] = temp
// 		// 					}
// 		// 				}
// 		// 			}
// 		// 		}
// 		// 	}
// 		// }
// 	}

// }
