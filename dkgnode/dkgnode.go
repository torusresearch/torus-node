package dkgnode

//TODO: export all "tm" imports to common folder
import (
	"crypto/ecdsa"
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
	"github.com/torusresearch/torus-public/logging"
)

const DefaultConfigPath = "/.torus/config.json"

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
func New() {
	// QUESTION(TEAM) - was there a reason for passing in a reference to the suite, and setting the config by mutating suite itself?
	// for now I just made it like so: cfg := loadConfig()
	// and then suite.Config = cfg
	// BUT
	// Wouldn't it be better to have a "config" package that has an init()
	// that sets all the config variables and is available globally for read?
	// it should be immutable after initializing, but if not we can always stick a mutex.
	cfg := loadConfig(DefaultConfigPath)
	logging.Infof("Loaded config, BFTUri: %s, Hostname: %s, p2plistenaddress: %s", cfg.BftURI, cfg.HostName, cfg.P2PListenAddress)

	//Main suite of functions used in node
	suite := Suite{}
	suite.Config = cfg
	suite.Flags = &Flags{cfg.IsProduction}

	if cfg.CPUProfileToFile != "" {
		f, err := os.Create(cfg.CPUProfileToFile)
		if err != nil {
			log.Fatal(err)
		}
		pprof.StartCPUProfile(f)
		go func() {
			time.Sleep(5 * time.Minute)
			pprof.StopCPUProfile()
		}()
	}
	// QUESTION(TEAM) - SIGTERM and SIGKILL handling should be present
	// TODO: we need a graceful shutdown

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
	os.MkdirAll(cfg.BasePath+"/config", os.ModePerm)
	os.MkdirAll(cfg.BasePath+"/data", os.ModePerm)
	// we generate nodekey first cause we need it in node list TODO: find a better way
	nodekey, err := p2p.LoadOrGenNodeKey(cfg.BasePath + "/config/node_key.json")
	if err != nil {
		logging.Errorf("Node Key generation issue: %s", err)
	}

	//TODO: now somewhat redundent
	if cfg.IsProduction {
		logging.Debug("---PRODUCTION MODE---")
	} else {
		logging.Debug("---DEVELOPMENT MODE---")
	}

	logging.Infof("Node IP Address: %s", suite.Config.MainServerAddress)
	whitelisted := false

	for !whitelisted {
		isWhitelisted, err := suite.EthSuite.NodeListContract.ViewWhitelist(nil, big.NewInt(0), *suite.EthSuite.NodeAddress)
		if err != nil {
			logging.Errorf("Could not check ethereum whitelist: %s", err.Error())
		}
		if isWhitelisted {
			whitelisted = true
			break
		}
		logging.Warning("Node is not whitelisted")
		time.Sleep(4 * time.Second)
	}

	if cfg.ShouldRegister && whitelisted {
		// register Node
		logging.Info("Registering node...")
		var externalAddr string
		if cfg.ProvidedIPAddress != "" {
			//for external deploymets
			externalAddr = "tcp://" + cfg.ProvidedIPAddress + ":" + strings.Split(suite.Config.P2PListenAddress, ":")[2]
		} else {
			externalAddr = suite.Config.P2PListenAddress
		}
		//TODO: Make epoch variable when needeed
		_, err := suite.EthSuite.registerNode(*big.NewInt(int64(0)), suite.Config.MainServerAddress, p2p.IDAddressString(nodekey.ID(), externalAddr))
		if err != nil {
			logging.Fatal(err.Error())
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
					logging.Infof("Node Monitor updating node list: %v", nlMonitorMsg.Payload)
					suite.EthSuite.NodeList = nlMonitorMsg.Payload.([]*NodeReference)

					if len(suite.EthSuite.NodeList) == suite.Config.NumberOfNodes {
						logging.Infof("Starting tendermint core... NodeList: %v", suite.EthSuite.NodeList)
						//initialize app val set for the first time and update validators to false
						go startTendermintCore(&suite, cfg.BasePath+"/tendermint", suite.EthSuite.NodeList, tmCoreMsgs)
					} else {
						logging.Warning("ethlist not equal in length to nodelist")
					}
				}
			}

		case coreMsg := <-tmCoreMsgs:
			if coreMsg == "started_tmcore" {
				time.Sleep(35 * time.Second) // time is more then the subscriber 30 seconds
				//Start key generation monitor when bft is done setting up
				logging.Debugf("Start Key generation monitor: %v, %v", suite.EthSuite.NodeList, suite.BftSuite.BftRPC)
				go startKeyGenerationMonitor(&suite, keyGenMonitorMsgs)
			}

		case keyGenMonitorMsg := <-keyGenMonitorMsgs:
			logging.Debug("KEYGEN: keygenmonitor received message")
			if keyGenMonitorMsg.Type == "start_keygen" {
				//starts keygeneration with starting and ending index
				logging.Debugf("KEYGEN: starting keygen with indexes: %d %d", keyGenMonitorMsg.Payload.([]int)[0], keyGenMonitorMsg.Payload.([]int)[1])
				go startKeyGeneration(&suite, keyGenMonitorMsg.Payload.([]int)[0], keyGenMonitorMsg.Payload.([]int)[1])
			}
		}
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
