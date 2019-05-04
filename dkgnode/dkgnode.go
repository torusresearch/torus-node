package dkgnode

//TODO: export all "tm" imports to common folder
import (
	"context"
	"log"
	"math/big"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

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

	// build folders for tendermint logs
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

	SetupFSM(&suite)
	SetupPSS(&suite)
	SetupCache(&suite)
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
	go SetupBft(&suite, abciServerMonitorTicker.C, bftWorkerMsgs)
	go bftWorker(&suite, bftWorkerMsgs)
	server := setUpServer(&suite, string(suite.Config.HttpServerPort))

	go whitelistMonitor(&suite, whitelistMonitorTicker.C, whitelistMonitorMsgs)
	go whitelistWorker(&suite, whitelistMonitorMsgs)

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
