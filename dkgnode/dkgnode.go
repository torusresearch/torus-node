package dkgnode

//TODO: export all "tm" imports to common folder
import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	tmbtcec "github.com/tendermint/btcd/btcec"
	tmsecp "github.com/tendermint/tendermint/crypto/secp256k1"
	tmnode "github.com/tendermint/tendermint/node"
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
	PSSSuite        *PSSSuite
}

type googleIdentityVerifier struct {
	*auth.GoogleVerifier
	suite *Suite
}

/* The entry point for our System */
func New() {
	logging.Info("starting telemetry")
	go telemetry.Serve()
	cfg := loadConfig(DefaultConfigPath)
	logging.Infof("Loaded config, BFTUri: %s, MainServerAddress: %s, tmp2plistenaddress: %s", cfg.BftURI, cfg.MainServerAddress, cfg.TMP2PListenAddress)

	//build folders for tendermint logs
	os.MkdirAll(cfg.BasePath+"/tendermint", os.ModePerm)
	os.MkdirAll(cfg.BasePath+"/tendermint/config", os.ModePerm)
	os.MkdirAll(cfg.BasePath+"/tendermint/data", os.ModePerm)
	os.MkdirAll(cfg.BasePath+"/config", os.ModePerm)
	os.MkdirAll(cfg.BasePath+"/data", os.ModePerm)

	// Main suite of functions used in node
	suite := Suite{}
	suite.Config = cfg

	// Initialize all necessary channels
	nodeListMonitorTicker := time.NewTicker(5 * time.Second)
	fmt.Println(nodeListMonitorTicker)
	keyGenMonitorTicker := time.NewTicker(5 * time.Second)
	fmt.Println(keyGenMonitorTicker)
	whitelistMonitorTicker := time.NewTicker(5 * time.Second)
	fmt.Println(whitelistMonitorTicker)
	abciServerMonitorTicker := time.NewTicker(5 * time.Second)
	fmt.Println(abciServerMonitorTicker)
	whitelistMonitorMsgs := make(chan WhitelistMonitorUpdates)
	fmt.Println(whitelistMonitorMsgs)
	bftWorkerMsgs := make(chan string)
	fmt.Println(bftWorkerMsgs)
	tmCoreMsgs := make(chan string)
	fmt.Println(tmCoreMsgs)
	nodeListMonitorMsgs := make(chan NodeListUpdates)
	fmt.Println(nodeListMonitorMsgs)
	nodeListWorkerMsgs := make(chan string)
	fmt.Println(nodeListWorkerMsgs)
	keyGenMonitorMsgs := make(chan KeyGenUpdates)
	fmt.Println(keyGenMonitorMsgs)
	pssWorkerMsgs := make(chan PSSWorkerUpdate)
	fmt.Println(pssWorkerMsgs)
	idleConnsClosed := make(chan struct{})
	fmt.Println(idleConnsClosed)

	SetupFSM(&suite)
	SetupPSS(&suite)
	SetupVerifier(&suite)
	err := SetupDB(&suite)
	if err != nil { // TODO: retry on error
		log.Fatal(err)
	}
	err = SetupEth(&suite)
	if err != nil { // TODO: retry on error
		log.Fatal(err)
	}
	_, err = SetupP2PHost(&suite)
	if err != nil { // TODO: retry on error
		log.Fatal(err)
	}

	go RunABCIServer(&suite) // tendermint handles sigterm on its own
	SetupBft(&suite)
	SetupCache(&suite)
	server := setUpServer(&suite, string(suite.Config.HttpServerPort))

	go whitelistMonitor(&suite, whitelistMonitorTicker.C, whitelistMonitorMsgs)
	go whitelistWorker(&suite, whitelistMonitorMsgs)

	// Initialize all necessary channels

	go startNodeListMonitor(&suite, nodeListMonitorTicker.C, nodeListMonitorMsgs)

	// Stop upon receiving SIGTERM or CTRL-C
	osSignal := make(chan os.Signal, 1)
	signal.Notify(osSignal, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-osSignal
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

	// run server
	go serverWorker(&suite, server)

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
	time.Sleep(20 * time.Second)
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
	go pssWorker(&suite, pssWorkerMsgs)
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

				suite.LocalStatus.Event(LocalStatusConstants.Events.StartKeygen, keyGenMonitorMsg.Payload.([]int)[0], keyGenMonitorMsg.Payload.([]int)[1])
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

func serverWorker(suite *Suite, server *http.Server) {
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

}
