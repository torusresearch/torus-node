package dkgnode

import (
	"math/big"
	// "strconv"
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
