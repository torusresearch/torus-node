package dkgnode

import (
	"fmt"
	"math/big"
	"strconv"
	"time"
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
		fmt.Println("in start keygen monitor")
		time.Sleep(1 * time.Second)
		if suite.ABCIApp.state.LocalStatus["all_initiate_keygen"] != "" {
			fmt.Println("WAITING FOR ALL INITIATE KEYGEN TO STOP BEING IN PROGRESS")
			continue
		}
		percentLeft := 100 * (suite.ABCIApp.state.LastCreatedIndex - suite.ABCIApp.state.LastUnassignedIndex) / uint(suite.Config.KeysPerEpoch)
		if percentLeft > 40 {
			continue
		}
		startingIndex := int(suite.ABCIApp.state.LastCreatedIndex)
		endingIndex := suite.Config.KeysPerEpoch + int(suite.ABCIApp.state.LastCreatedIndex)

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
			fmt.Println("coudl not broadcast initiateKeygeneration", err)
			continue
		}

		for {
			fmt.Println("WAITING FOR ALL INITIATE KEYGEN TO BE Y")
			fmt.Println(suite.ABCIApp.state)
			time.Sleep(1 * time.Second)
			if suite.ABCIApp.state.LocalStatus["all_initiate_keygen"] == "Y" {
				fmt.Println("STATUSTX: localstatus all initiate keygen is Y, appstate", suite.ABCIApp.state)
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
		fmt.Println("Checking Node List...")
		// Fetch Node List from contract address
		//TODO: make epoch vairable
		epoch := big.NewInt(int64(0))
		ethList, positions, err := suite.EthSuite.NodeListContract.ViewNodes(nil, epoch)
		// If we can't reach ethereum node, lets try next time
		if err != nil {
			fmt.Println("Could not View Nodes on ETH Network", err)
		} else {
			// Build count of nodes connected to
			fmt.Println("Indexes", positions, ethList)
			connectedNodes := 0
			nodeList := make([]*NodeReference, len(ethList))
			if len(ethList) > 0 {
				for i := range ethList {
					// Check if node is online by pinging
					temp, err := connectToJSONRPCNode(suite, *epoch, ethList[i])
					if err != nil {
						fmt.Println(err)
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
