package dkgnode

//TODO: export all "tm" imports to common folder
import (
	"crypto/ecdsa"
	"fmt"
	"log"
	"math/big"
	"os"
	"time"

	"github.com/YZhenY/torus/common"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	tmbtcec "github.com/tendermint/btcd/btcec"
	tmsecp "github.com/tendermint/tendermint/crypto/secp256k1"
	tmnode "github.com/tendermint/tendermint/node"
	"github.com/tendermint/tendermint/p2p"
	tmtypes "github.com/tendermint/tendermint/types"
)

type Suite struct {
	EthSuite   *EthSuite
	BftSuite   *BftSuite
	CacheSuite *CacheSuite
	Config     *Config
	Flags      *Flags
	ABCIApp    *ABCIApp
}

type Flags struct {
	Production bool
}

/* The entry point for our System */
func New(configPath string, register bool, production bool, buildPath string) {

	//Main suite of functions used in node
	suite := Suite{}
	suite.Flags = &Flags{production}
	fmt.Println(configPath)
	loadConfig(&suite, configPath)
	//TODO: Dont die on failure but retry
	err := SetUpEth(&suite)
	if err != nil {
		log.Fatal(err)
	}
	go RunABCIServer(&suite)
	SetUpBft(&suite)
	SetUpCache(&suite)
	var nodeIPAddress string

	//build folders for tendermint logs
	os.MkdirAll(buildPath+"/config", os.ModePerm)
	// we generate nodekey first cause we need it in node list TODO: find a better way
	nodekey, err := p2p.LoadOrGenNodeKey(buildPath + "/config/node_key.json")
	if err != nil {
		fmt.Println("Node Key generation issue")
		fmt.Println(err)
	}

	var nodeIP string
	if production {
		fmt.Println("//PRODUCTION MDOE ")
		nodeIPAddress = suite.Config.HostName + ":" + string(suite.Config.MyPort)
	} else {
		fmt.Println("//DEVELOPMENT MDOE ")
		nodeIP, err = findExternalIP()
		if err != nil {
			fmt.Println(err)
		}
		nodeIPAddress = nodeIP + ":" + string(suite.Config.MyPort)
	}

	fmt.Println("Node IP Address: " + nodeIPAddress)
	if register {
		/* Register Node */
		fmt.Println("Registering node...")
		temp := p2p.IDAddressString(nodekey.ID(), nodeIP+suite.Config.P2PListenAddress[13:])
		// _, err = suite.EthSuite.registerNode(nodeIPAddress, nodekey.PubKey().Address().String()+"@"+suite.Config.P2PListenAddress[6:])
		_, err = suite.EthSuite.registerNode(nodeIPAddress, temp)
		if err != nil {
			log.Fatal(err)
		}
	}

	//Initialzie all necessary channels
	tmCoreMsgs := make(chan string)
	nodeListMonitorMsgs := make(chan NodeListUpdates)
	go startNodeListMonitor(&suite, nodeListMonitorMsgs)

	//Set up standard server
	go setUpServer(&suite, string(suite.Config.MyPort))

	//TODO: remove, testing purposes
	tmStarted := false
	// So it runs forever
	for {
		select {
		case nlMonitorMsg := <-nodeListMonitorMsgs:
			if nlMonitorMsg.Type == "update" {
				// fmt.Println("we got a message", nlMonitorMsg)
				// fmt.Println("Suite is: ", suite.EthSuite.NodeList, cmp.Equal(nlMonitorMsg.Payload.([]*NodeReference), suite.EthSuite.NodeList))

				//Compoare existing nodelist to updated node list. Cmp options are there to not compare too deep. If NodeReference is changed this might bug up (need to include new excludes)
				if !cmp.Equal(nlMonitorMsg.Payload.([]*NodeReference), suite.EthSuite.NodeList,
					cmpopts.IgnoreTypes(ecdsa.PublicKey{}),
					cmpopts.IgnoreUnexported(big.Int{}),
					cmpopts.IgnoreFields(NodeReference{}, "JSONClient")) {
					fmt.Println("NODE LIST IS UPDATED THANKS TO MONITOR", nlMonitorMsg.Payload)
					suite.EthSuite.NodeList = nlMonitorMsg.Payload.([]*NodeReference)

					// here we trigger eithe... A new "ENDBLOCK" to update validators or an initial start key generation for epochs
					if len(suite.EthSuite.NodeList) >= suite.Config.NumberOfNodes {
						fmt.Println("Starting tendermint core... NodeList:", suite.EthSuite.NodeList)

						//Update app.state or perhaps just reference suite? change for testing
						if tmStarted && len(suite.EthSuite.NodeList) > 9 {
							// suite.BftSuite.UpdateVal = true
							//check so that only 1 transaction is submitted
							valTx := ValidatorUpdateBFTTx{
								make([]common.Point, len(suite.EthSuite.NodeList)),
								make([]uint, len(suite.EthSuite.NodeList)),
							}
							for i := range suite.EthSuite.NodeList {
								valTx.ValidatorPower[i] = uint(1)
								valTx.ValidatorPubKey[i] = common.Point{X: *suite.EthSuite.NodeList[i].PublicKey.X, Y: *suite.EthSuite.NodeList[i].PublicKey.Y}
							}
							updateValTx := DefaultBFTTxWrapper{valTx}
							time.Sleep(3 * time.Second) //wait for all nodes to update their node lists
							suite.BftSuite.BftRPC.Broadcast(updateValTx)
						}

						//TODO: Remove temp checks to differenciate between starting node and joining a network?
						if !tmStarted {
							tmStarted = true
							//initialize app val set for the first time and update validators to false
							suite.ABCIApp.state.UpdateValidators = false
							//todo: change this when we edit nodelist for epochs
							suite.ABCIApp.state.ValidatorSet = convertNodeListToValidatorUpdate(suite.EthSuite.NodeList)
							go startTendermintCore(&suite, buildPath, suite.EthSuite.NodeList, tmCoreMsgs)
						}
					}
				}
			}
		case coreMsg := <-tmCoreMsgs:
			fmt.Println("received", coreMsg)
			if coreMsg == "Started Tendermint Core" {
				time.Sleep(35 * time.Second) // time is more then the subscriber 30 seconds
				//Start key generation when bft is done setting up
				fmt.Println("Started KEY GENERATION WITH:", suite.EthSuite.NodeList, suite.BftSuite.BftRPC)
				//TODO: For testing purposes, remove condition
				if len(suite.EthSuite.NodeList) != 6 {
					go startKeyGeneration(&suite, suite.EthSuite.NodeList, suite.BftSuite.BftRPC)
				}
			}
		}
		time.Sleep(1 * time.Second) //TODO: Is a time out necessary?
	}
}

func ProvideGenDoc(doc *tmtypes.GenesisDoc) tmnode.GenesisDocProvider {
	return func() (*tmtypes.GenesisDoc, error) {
		return doc, nil
	}
}

func RawPointToTMPubKey(X, Y *big.Int) tmsecp.PubKeySecp256k1 {
	//convert pubkey X and Y to tmpubkey
	var pubkeyBytes tmsecp.PubKeySecp256k1
	pubkeyObject := tmbtcec.PublicKey{
		X: X,
		Y: Y,
	}
	copy(pubkeyBytes[:], pubkeyObject.SerializeCompressed())
	return pubkeyBytes
}
