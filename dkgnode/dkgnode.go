package dkgnode

//TODO: export all "tm" imports to common folder
import (
	"context"
	"crypto/ecdsa"
	"log"
	"math/big"
	"os"
	"os/signal"
	"runtime/pprof"
	"strings"
	"syscall"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	tmbtcec "github.com/tendermint/btcd/btcec"
	tmsecp "github.com/tendermint/tendermint/crypto/secp256k1"
	tmnode "github.com/tendermint/tendermint/node"
	"github.com/tendermint/tendermint/p2p"
	tmtypes "github.com/tendermint/tendermint/types"
	"github.com/torusresearch/torus-public/auth"
	"github.com/torusresearch/torus-public/logging"
	"github.com/torusresearch/torus-public/telemetry"
)

const DefaultConfigPath = "/.torus/config.json"

type Suite struct {
	EthSuite        *EthSuite
	BftSuite        *BftSuite
	CacheSuite      *CacheSuite
	Config          *Config
	ABCIApp         *ABCIApp
	DefaultVerifier auth.GeneralVerifier
	P2PSuite        *P2PSuite
	DBSuite         *DBSuite
	LocalStatus     *LocalStatus
}

type googleIdentityVerifier struct {
	*auth.GoogleVerifier
	suite *Suite
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
	logging.Infof("Loaded config, BFTUri: %s, MainServerAddress: %s, tmp2plistenaddress: %s", cfg.BftURI, cfg.MainServerAddress, cfg.TMP2PListenAddress)

	//Main suite of functions used in node
	suite := Suite{}
	suite.Config = cfg

	nodeListMonitorTicker := time.NewTicker(5 * time.Second)

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

	SetupFSM(&suite)

	//TODO: Dont die on failure but retry
	// set up connection to ethereum blockchain
	err := SetupDB(&suite)
	if err != nil {
		log.Fatal(err)
	}

	//TODO: Dont die on failure but retry
	// set up connection to ethereum blockchain
	err = SetupEth(&suite)
	if err != nil {
		log.Fatal(err)
	}

	//setup p2p host
	_, err = SetupP2PHost(&suite)
	if err != nil {
		log.Fatal(err)
	}

	logging.Info("starting telemetry")
	go telemetry.Serve()

	// run tendermint ABCI server
	// It seems tendermint handles sigterm on its own..
	go RunABCIServer(&suite)

	// setup connection to tendermint BFT
	SetupBft(&suite)
	// setup local caching
	SetupCache(&suite)

	// We can use a flag here to change the default verifier
	// In the future we should allow a range of verifiers
	suite.DefaultVerifier = auth.NewGeneralVerifier(googleIdentityVerifier{
		auth.NewDefaultGoogleVerifier(cfg.GoogleClientID),
		&suite,
	})

	//build folders for tendermint logs
	os.MkdirAll(cfg.BasePath+"/tendermint", os.ModePerm)
	os.MkdirAll(cfg.BasePath+"/tendermint/config", os.ModePerm)
	os.MkdirAll(cfg.BasePath+"/tendermint/data", os.ModePerm)
	os.MkdirAll(cfg.BasePath+"/config", os.ModePerm)
	os.MkdirAll(cfg.BasePath+"/data", os.ModePerm)

	// we generate nodekey first cause we need it in node list TODO: find a better way
	tmNodeKey, err := p2p.LoadOrGenNodeKey(cfg.BasePath + "/config/node_key.json")
	if err != nil {
		logging.Errorf("Node Key generation issue: %s", err)
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
		time.Sleep(4 * time.Second) // TODO(LEN): move out into config
	}

	if cfg.ShouldRegister && whitelisted {
		// register Node
		externalAddr := "tcp://" + cfg.ProvidedIPAddress + ":" + strings.Split(suite.Config.TMP2PListenAddress, ":")[2]

		// var externalAddr string
		// if cfg.ProvidedIPAddress != "" {
		// 	//for external deploymets
		// 	externalAddr = "tcp://" + cfg.ProvidedIPAddress + ":" + strings.Split(suite.Config.P2PListenAddress, ":")[2]
		// } else {
		// 	externalAddr = suite.Config.P2PListenAddress
		// }

		logging.Infof("Registering node with %v %v", suite.Config.MainServerAddress, p2p.IDAddressString(tmNodeKey.ID(), externalAddr))
		//TODO: Make epoch variable when needeed
		_, err := suite.EthSuite.registerNode(*big.NewInt(int64(0)), suite.Config.MainServerAddress, p2p.IDAddressString(tmNodeKey.ID(), externalAddr), suite.P2PSuite.HostAddress.String())
		if err != nil {
			logging.Fatal(err.Error())
		}
	}

	// Initialize all necessary channels
	tmCoreMsgs := make(chan string)
	nodeListMonitorMsgs := make(chan NodeListUpdates)
	keyGenMonitorMsgs := make(chan KeyGenUpdates)

	go startNodeListMonitor(&suite, nodeListMonitorTicker.C, nodeListMonitorMsgs)
	// Set up standard server
	server := setUpServer(&suite, string(suite.Config.HttpServerPort))

	// Stop upon receiving SIGTERM or CTRL-C
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	idleConnsClosed := make(chan struct{})
	go func() {
		<-c
		logging.Info("Shutting down the node, received signal.")

		// Shutdown the jRPC server
		err := server.Shutdown(context.Background())
		if err != nil {
			logging.Errorf("Failed during shutdown: %s", err.Error())
		}

		// Stop NodeList monitor ticker
		nodeListMonitorTicker.Stop()

		// Exit the blocking chan
		close(idleConnsClosed)
	}()

	go func() {
		if suite.Config.ServeUsingTLS {
			if suite.Config.UseAutoCert {
				logging.Fatal("AUTO CERT NOT YET IMPLEMENTED")
			}

			if suite.Config.ServerCert != "" {
				err := server.ListenAndServeTLS(suite.Config.ServerCert,
					suite.Config.ServerKey)
				if err != nil {
					logging.Fatal(err.Error())
				}
			} else {
				logging.Fatal("Certs not supplied, try running with UseAutoCert")
			}

		} else {
			err := server.ListenAndServe()
			if err != nil {
				logging.Fatal(err.Error())
			}

		}

	}()

	// TODO(TEAM): This needs to be less verbose, and wrapped in some functions..
	// It really doesnt need to run forever right?...
	// So it runs forever

	// Setup Phase
	for {
		nlMonitorMsg := <-nodeListMonitorMsgs
		if nlMonitorMsg.Type == "update" {
			// Compare existing nodelist to updated node list. Cmp options are there to not compare too deep. If NodeReference is changed this might bug up (need to include new excludes)
			if !cmp.Equal(nlMonitorMsg.Payload.([]*NodeReference), suite.EthSuite.NodeList,
				cmpopts.IgnoreTypes(ecdsa.PublicKey{}),
				cmpopts.IgnoreUnexported(big.Int{}),
				cmpopts.IgnoreFields(NodeReference{}, "PeerID")) {
				logging.Infof("Node Monitor updating node list: %v", nlMonitorMsg.Payload)
				suite.EthSuite.NodeList = nlMonitorMsg.Payload.([]*NodeReference)

				if len(suite.EthSuite.NodeList) == suite.Config.NumberOfNodes {
					logging.Infof("Starting tendermint core... NodeList: %v", suite.EthSuite.NodeList)
					//initialize app val set for the first time and update validators to false
					go startTendermintCore(&suite, cfg.BasePath+"/tendermint", suite.EthSuite.NodeList, tmCoreMsgs, idleConnsClosed)

					break
				} else {
					logging.Warning("ethlist not equal in length to nodelist")
				}
			}
		}
	}

	logging.Info("Sleeeping...")
	time.Sleep(35 * time.Second)
	logging.Info("Waking up...")
	for {
		coreMsg := <-tmCoreMsgs
		logging.Debugf("received: %s", coreMsg)
		if coreMsg == "started_tmcore" {
			//Start key generation monitor when bft is done setting up
			logging.Infof("Start Key generation monitor: %v, %v", suite.EthSuite.NodeList, suite.BftSuite.BftRPC)
			go startKeyGenerationMonitor(&suite, keyGenMonitorMsgs)
			break
		}
	}

	go keyGenWorker(&suite, keyGenMonitorMsgs)
	<-idleConnsClosed
}

func keyGenWorker(suite *Suite, keyGenMonitorMsgs chan KeyGenUpdates) {
	for {
		select {

		case keyGenMonitorMsg := <-keyGenMonitorMsgs:
			logging.Debug("KEYGEN: keygenmonitor received message")
			if keyGenMonitorMsg.Type == "start_keygen" {
				//starts keygeneration with starting and ending index
				logging.Debugf("KEYGEN: starting keygen with indexes: %d %d", keyGenMonitorMsg.Payload.([]int)[0], keyGenMonitorMsg.Payload.([]int)[1])

				suite.LocalStatus.Event(suite.LocalStatus.Constants.Events.StartKeygen, keyGenMonitorMsg.Payload.([]int)[0], keyGenMonitorMsg.Payload.([]int)[1])
				// go startKeyGeneration(suite, keyGenMonitorMsg.Payload.([]int)[0], keyGenMonitorMsg.Payload.([]int)[1])
				// go suite.P2PSuite.KeygenProto.NewKeygen(suite, keyGenMonitorMsg.Payload.([]int)[0], keyGenMonitorMsg.Payload.([]int)[1])
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
