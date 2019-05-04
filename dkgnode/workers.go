package dkgnode

import (
	"fmt"
	"math/big"
	"strings"

	"github.com/tendermint/tendermint/p2p"
	"github.com/torusresearch/torus-public/logging"
)

// "github.com/tendermint/tendermint/p2p"
// "github.com/torusresearch/torus-public/logging"
// "strings"
// // "github.com/torusresearch/torus-public/pss"
// "math/big"

func keyGenWorker(suite *Suite, keyGenMonitorMsgs <-chan KeyGenUpdates) {
	for keyGenMonitorMsg := range keyGenMonitorMsgs {
		logging.Debug("KEYGEN: keygenmonitor received message")
		if keyGenMonitorMsg.Type == "start_keygen" {
			//starts keygeneration with starting and ending index
			logging.Debugf("KEYGEN: starting keygen with indexes: %d %d", keyGenMonitorMsg.Payload.([]int)[0], keyGenMonitorMsg.Payload.([]int)[1])

			suite.LocalStatus.Event(LocalStatusConstants.Events.StartKeygen, keyGenMonitorMsg.Payload.([]int)[0], keyGenMonitorMsg.Payload.([]int)[1])
			// go startKeyGeneration(suite, keyGenMonitorMsg.Payload.([]int)[0], keyGenMonitorMsg.Payload.([]int)[1])
			// go suite.P2PSuite.KeygenProto.NewKeygen(suite, keyGenMonitorMsg.Payload.([]int)[0], keyGenMonitorMsg.Payload.([]int)[1])
		}
	}
}

func whitelistWorker(suite *Suite, tmNodeKey *p2p.NodeKey, whitelistMonitorMsgs <-chan WhitelistMonitorUpdates) {
	for whitelistMonitorMsg := range whitelistMonitorMsgs {
		if whitelistMonitorMsg.Type == "node_whitelisted" {
			if !suite.Config.ShouldRegister {
				continue
			}
			externalAddr := "tcp://" + suite.Config.ProvidedIPAddress + ":" + strings.Split(suite.Config.TMP2PListenAddress, ":")[2]
			logging.Infof("Registering node with %v %v", suite.Config.MainServerAddress, p2p.IDAddressString(tmNodeKey.ID(), externalAddr))
			_, err := suite.EthSuite.registerNode(*big.NewInt(int64(whitelistMonitorMsg.Payload.(int))), suite.Config.MainServerAddress, p2p.IDAddressString(tmNodeKey.ID(), externalAddr), suite.P2PSuite.HostAddress.String())
			if err != nil {
				logging.Fatal(err.Error())
			}
		}
	}
}

func nodeListWorker(suite *Suite, nodeListMonitorMsgs <-chan NodeListUpdates, nodeListWorkerMsgs chan<- string) {
	for nlMonitorMsg := range nodeListMonitorMsgs {
		if nlMonitorMsg.Type == "all_connected" {
			fmt.Println("Inside of nodelistworker")
			nodeList := suite.EthSuite.EpochNodeRegister[suite.EthSuite.CurrentEpoch].NodeList
			if len(nodeList) != suite.Config.NumberOfNodes {
				logging.Warning("ethlist not equal in length to nodelist")
				continue
			}
			if suite.EthSuite.EpochNodeRegister[suite.EthSuite.CurrentEpoch].AllConnected {
				logging.Warning("AllConnected has already been set to true")
				continue
			}
			logging.Infof("Starting tendermint core... NodeList: %v", nodeList)
			suite.EthSuite.EpochNodeRegister[suite.EthSuite.CurrentEpoch].AllConnected = true
			nodeListWorkerMsgs <- "all_connected"
			break
		}
	}
}

func pssWorker(suite *Suite, pssWorkerMsgs <-chan PSSWorkerUpdate) {
	for pssWorkerMsg := range pssWorkerMsgs {
		if pssWorkerMsg.Type == "all_connected" {
			logging.Info("PSS Msg all connected")
		}
	}
}
