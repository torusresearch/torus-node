package dkgnode

//TODO: export all "tm" imports to common folder
import (
	"log"
	"math/big"
	"os"
	"os/signal"
	"syscall"
	"time"

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

	// build folders for tendermint logs
	os.MkdirAll(cfg.BasePath+"/tendermint", os.ModePerm)
	os.MkdirAll(cfg.BasePath+"/tendermint/config", os.ModePerm)
	os.MkdirAll(cfg.BasePath+"/tendermint/data", os.ModePerm)
	os.MkdirAll(cfg.BasePath+"/config", os.ModePerm)
	os.MkdirAll(cfg.BasePath+"/data", os.ModePerm)

	// Main suite of functions used in node
	suite := Suite{}
	suite.Config = cfg

	if suite.Config.IsDebug {
		logging.Info("--------------------------------------RUNNING IN DEBUG MODE --------------------------------")
	}
	// Initialize all necessary channels

	nodeListMonitorTicker := time.NewTicker(5 * time.Second)
	keyGenMonitorTicker := time.NewTicker(5 * time.Second)
	whitelistMonitorTicker := time.NewTicker(5 * time.Second)
	abciServerMonitorTicker := time.NewTicker(5 * time.Second)
	whitelistMonitorMsgs := make(chan WhitelistMonitorUpdates)
	bftWorkerMsgs := make(chan string)
	tmCoreMsgs := make(chan string)
	nodeListMonitorMsgs := make(chan NodeListUpdates)
	nodeListWorkerMsgs := make(chan string)
	keyGenMonitorMsgs := make(chan KeyGenUpdates)
	pssWorkerMsgs := make(chan PSSWorkerUpdate)
	idleConnsClosed := make(chan struct{})

	tmNodeKey, err := p2p.LoadOrGenNodeKey(suite.Config.BasePath + "/config/node_key.json")
	if err != nil {
		logging.Errorf("Node Key generation issue: %s", err)
	}

	SetupFSM(&suite)
	SetupPSS(&suite)
	SetupCache(&suite)
	SetupVerifier(&suite)
	err = SetupDB(&suite)
	if err != nil { // TODO: retry on error
		log.Fatal(err)
	}

	//TODO: Dont die on failure but retry
	// set up connection to ethereum blockchain
	err = SetupEth(&suite)
	if err != nil { // TODO: retry on error
		log.Fatal(err)
	}
	_, err = SetupP2PHost(&suite)
	if err != nil { // TODO: retry on error
		log.Fatal(err)
	}

	// Set Up Server
	go setUpAndRunHttpServer(&suite)
	go RunABCIServer(&suite) // tendermint handles sigterm on its own
	go SetupBft(&suite, abciServerMonitorTicker.C, bftWorkerMsgs)
	go bftWorker(&suite, bftWorkerMsgs)

	go whitelistMonitor(&suite, whitelistMonitorTicker.C, whitelistMonitorMsgs)
	go whitelistWorker(&suite, tmNodeKey, whitelistMonitorMsgs)

	// Stop upon receiving SIGTERM or CTRL-C
	osSignal := make(chan os.Signal, 1)
	signal.Notify(osSignal, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-osSignal
		logging.Info("Shutting down the node, received signal.")

		// Stop NodeList monitor ticker
		nodeListMonitorTicker.Stop()

		// Exit the blocking chan
		close(idleConnsClosed)
	}()

	// Set up standard server
	// server := setUpServer(&suite, string(suite.Config.HttpServerPort))

	// TODO(TEAM): This needs to be less verbose, and wrapped in some functions..
	// It really doesnt need to run forever right?...
	// So it runs forever

	// Setup Phase
	go nodeListMonitor(&suite, nodeListMonitorTicker.C, nodeListMonitorMsgs, pssWorkerMsgs)
	go nodeListWorker(&suite, nodeListMonitorMsgs, nodeListWorkerMsgs)
	go startTendermintCore(&suite, cfg.BasePath+"/tendermint", nodeListWorkerMsgs, tmCoreMsgs, idleConnsClosed)
	go keyGenMonitor(&suite, tmCoreMsgs, keyGenMonitorTicker.C, keyGenMonitorMsgs)
	go keyGenWorker(&suite, keyGenMonitorMsgs)
	go pssWorker(&suite, pssWorkerMsgs)
	<-idleConnsClosed
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
