package dkgnode

//TODO: export all "tm" imports to common folder
import (
	"crypto/ecdsa"
	"fmt"
	"log"
	"math/big"
	"os"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	tmbtcec "github.com/tendermint/btcd/btcec"
	tmsecp "github.com/tendermint/tendermint/crypto/secp256k1"
	tmnode "github.com/tendermint/tendermint/node"
	"github.com/tendermint/tendermint/p2p"
	tmtypes "github.com/tendermint/tendermint/types"
)

//old imports
// tmbtcec "github.com/tendermint/btcd/btcec"
// tmconfig "github.com/tendermint/tendermint/config"
// tmsecp "github.com/tendermint/tendermint/crypto/secp256k1"
// tmlog "github.com/tendermint/tendermint/libs/log"
// tmnode "github.com/tendermint/tendermint/node"
// "github.com/tendermint/tendermint/p2p"
// "github.com/tendermint/tendermint/privval"
// tmproxy "github.com/tendermint/tendermint/proxy"
// tmtypes "github.com/tendermint/tendermint/types"

type Suite struct {
	EthSuite   *EthSuite
	BftSuite   *BftSuite
	CacheSuite *CacheSuite
	Config     *Config
	Flags      *Flags
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
						fmt.Print("Starting tendermint core...")
						go startTendermintCore(&suite, buildPath, suite.EthSuite.NodeList, tmCoreMsgs)
					}

				}
			}
		case coreMsg := <-tmCoreMsgs:
			fmt.Println("received", coreMsg)
			if coreMsg == "Started Tendermint Core" {
				time.Sleep(35 * time.Second) // time is more then the subscriber 30 seconds
				//Start key generation when bft is done setting up
				fmt.Println("Started KEY GENERATION WITH:", suite.EthSuite.NodeList, suite.BftSuite.BftRPC)
				go startKeyGeneration(&suite, suite.EthSuite.NodeList, suite.BftSuite.BftRPC)
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
