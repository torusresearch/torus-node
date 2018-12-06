package dkgnode

//TODO: export all "tm" imports to common folder
import (
	"crypto/ecdsa"
	"fmt"
	"log"
	"math/big"
	"os"
	"runtime/pprof"
	"strings"
	"time"

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
func New(configPath string, register bool, production bool, buildPath string, cpuProfile string, nodeIPAddress string, privateKey string) {
	if cpuProfile != "" {
		f, err := os.Create(cpuProfile)
		if err != nil {
			log.Fatal(err)
		}
		pprof.StartCPUProfile(f)
		go func() {
			time.Sleep(5 * time.Minute)
			pprof.StopCPUProfile()
		}()
	}

	// Stop upon receiving SIGTERM or CTRL-C
	// c := make(chan os.Signal, 1)
	// signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	// go func() {
	// 	for sig := range c {
	// 		fmt.Println("Ending Process", sig)
	// 		//TODO: Exit process not working when abci dies first
	// 		// pprof.StopCPUProfile()
	// 		time.Sleep(5 * time.Second)
	// 		os.Exit(1)
	// 	}
	// }()

	//Main suite of functions used in node
	suite := Suite{}
	suite.Flags = &Flags{production}
	fmt.Println(configPath)
	loadConfig(&suite, configPath, nodeIPAddress, privateKey, buildPath)
	//TODO: Dont die on failure but retry

	// set up connection to ethereum blockchain
	err := SetUpEth(&suite)
	if err != nil {
		log.Fatal(err)
	}

	// run tendermint ABCI server
	go RunABCIServer(&suite)
	// setup connection to tendermint BFT
	SetUpBft(&suite)
	// setup local caching
	SetUpCache(&suite)

	//build folders for tendermint logs
	os.MkdirAll(buildPath+"/config", os.ModePerm)
	os.MkdirAll(buildPath+"/data", os.ModePerm)
	// we generate nodekey first cause we need it in node list TODO: find a better way
	nodekey, err := p2p.LoadOrGenNodeKey(buildPath + "/config/node_key.json")
	if err != nil {
		fmt.Println("Node Key generation issue")
		fmt.Println(err)
	}
	var mainServerIPAddress string
	if production {
		fmt.Println("---PRODUCTION MODE---")
		mainServerIPAddress = suite.Config.HostName + ":" + string(suite.Config.MyPort)
	} else {
		fmt.Println("---DEVELOPMENT MODE---")
		mainServerIPAddress = suite.Config.MainServerAddress
	}

	fmt.Println("Node IP Address: " + mainServerIPAddress)
	whitelisted := false

	for !whitelisted {
		isWhitelisted, err := suite.EthSuite.NodeListContract.ViewWhitelist(nil, big.NewInt(0), *suite.EthSuite.NodeAddress)
		if err != nil {
			fmt.Println("Could not check ethereum whitelist", err.Error())
		}
		if isWhitelisted {
			whitelisted = true
			break
		}
		fmt.Println("Node is not whitelisted")
		time.Sleep(4 * time.Second)
	}

	if register && whitelisted {
		// register Node
		fmt.Println("Registering node...")
		//TODO: Make epoch variable when needeed
		externalAddr := "tcp://" + nodeIPAddress + ":" + strings.Split(suite.Config.P2PListenAddress, ":")[2]
		_, err := suite.EthSuite.registerNode(*big.NewInt(int64(0)), mainServerIPAddress, p2p.IDAddressString(nodekey.ID(), externalAddr))
		if err != nil {
			log.Fatal(err)
		}
	}

	// Initialize all necessary channels
	tmCoreMsgs := make(chan string)
	nodeListMonitorMsgs := make(chan NodeListUpdates)
	keyGenMonitorMsgs := make(chan KeyGenUpdates)
	go startNodeListMonitor(&suite, nodeListMonitorMsgs)

	// Set up standard server
	go setUpServer(&suite, string(suite.Config.MyPort))

	// So it runs forever
	for {
		select {
		case nlMonitorMsg := <-nodeListMonitorMsgs:
			if nlMonitorMsg.Type == "update" {
				// Compare existing nodelist to updated node list. Cmp options are there to not compare too deep. If NodeReference is changed this might bug up (need to include new excludes)
				if !cmp.Equal(nlMonitorMsg.Payload.([]*NodeReference), suite.EthSuite.NodeList,
					cmpopts.IgnoreTypes(ecdsa.PublicKey{}),
					cmpopts.IgnoreUnexported(big.Int{}),
					cmpopts.IgnoreFields(NodeReference{}, "JSONClient")) {
					fmt.Println("Node Monitor updating node list...", nlMonitorMsg.Payload)
					suite.EthSuite.NodeList = nlMonitorMsg.Payload.([]*NodeReference)

					if len(suite.EthSuite.NodeList) == suite.Config.NumberOfNodes {
						fmt.Println("Starting tendermint core... NodeList:", suite.EthSuite.NodeList)
						//initialize app val set for the first time and update validators to false
						go startTendermintCore(&suite, buildPath, suite.EthSuite.NodeList, tmCoreMsgs)
					} else {
						fmt.Println("ethlist not equal in length to nodelist")
					}
				}
			}

		case coreMsg := <-tmCoreMsgs:
			fmt.Println("received", coreMsg)
			if coreMsg == "started_tmcore" {
				time.Sleep(35 * time.Second) // time is more then the subscriber 30 seconds
				//Start key generation monitor when bft is done setting up
				fmt.Println("Start Key generation monitor:", suite.EthSuite.NodeList, suite.BftSuite.BftRPC)
				go startKeyGenerationMonitor(&suite, keyGenMonitorMsgs)
			}

		case keyGenMonitorMsg := <-keyGenMonitorMsgs:
			if keyGenMonitorMsg.Type == "start_keygen" {
				//starts keygeneration with starting and ending index
				fmt.Println("starting keygen with indexes: ", keyGenMonitorMsg.Payload.([]int)[0], keyGenMonitorMsg.Payload.([]int)[1])
				go startKeyGeneration(&suite, keyGenMonitorMsg.Payload.([]int)[0], keyGenMonitorMsg.Payload.([]int)[1])
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
