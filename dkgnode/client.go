package dkgnode

/* All useful imports */
import (
	"crypto/ecdsa"
	// "encoding/hex"
	// "encoding/json"
	"fmt"
	// "io/ioutil"
	"math/big"
	"strings"
	"time"

	// "github.com/Rican7/retry"
	// "github.com/Rican7/retry/backoff"
	// "github.com/Rican7/retry/strategy"
	tmconfig "github.com/tendermint/tendermint/config"
	tmsecp "github.com/tendermint/tendermint/crypto/secp256k1"
	tmnode "github.com/tendermint/tendermint/node"
	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/privval"
	tmtypes "github.com/tendermint/tendermint/types"
	"github.com/torusresearch/torus-public/common"
	"github.com/torusresearch/torus-public/logging"
	// "github.com/torusresearch/torus-public/pvss"
	// "github.com/torusresearch/torus-public/secp256k1"
	// "github.com/torusresearch/torus-public/telemetry"
)

type Message struct {
	Message string `json:"message"`
}

// this appears in the form of map[int]SecretStore, shareindex => Z
// called secretMapping
type SecretStore struct {
	Secret   *big.Int // this is Z, the first generated random number from pvss by the node during keygen
	Assigned bool
}

type SiStore struct {
	Index int
	Value *big.Int
}

// deprecated, we dont need this anymore since we are storing it on the bft
// this appears in the form of map[string]SecretAssignment, email => userdata
type SecretAssignment struct {
	Secret     *big.Int
	ShareIndex int
	// this the polynomial share Si given to you by the node when you send your oauth token over
	Share *big.Int
}

type SigncryptedMessage struct {
	FromAddress string `json:"fromaddress"`
	FromPubKeyX string `json:"frompubkeyx"`
	FromPubKeyY string `json:"frompubkeyy"`
	ToPubKeyX   string `json:"topubkeyx"`
	ToPubKeyY   string `json:"topubkeyy"`
	Ciphertext  string `json:"ciphertext"`
	RX          string `json:"rx"`
	RY          string `json:"ry"`
	Signature   string `json:"signature"`
	ShareIndex  uint   `json:"shareindex"`
}

func startTendermintCore(suite *Suite, buildPath string, nodeListWorkerMsgs <-chan string, tmCoreMsgs chan<- string, idleConnsClosed chan struct{}) (string, error) {
	for nodeListWorkerMsg := range nodeListWorkerMsgs {
		if nodeListWorkerMsg == "all_connected" {
			break
		} else {
			logging.Error("Received unknown message in starttendermintcore: " + nodeListWorkerMsg)
		}
	}
	nodeList := suite.EthSuite.EpochNodeRegister[suite.EthSuite.CurrentEpoch].NodeList
	//Starts tendermint node here
	//builds default config
	defaultTmConfig := tmconfig.DefaultConfig()
	defaultTmConfig.SetRoot(buildPath)
	// logger := tmlog.NewTMLogger(tmlog.NewSyncWriter(os.Stdout))
	// logger := NewTMLogger(suite.Config.LogLevel)
	logger := EventForwardingLogger{}

	defaultTmConfig.ProxyApp = suite.Config.ABCIServer

	//converts own pv to tendermint key TODO: Double check verification
	var pv tmsecp.PrivKeySecp256k1
	for i := 0; i < 32; i++ {
		pv[i] = suite.EthSuite.NodePrivateKey.D.Bytes()[i]
	}

	pvF := privval.GenFilePVFromPrivKey(pv, defaultTmConfig.PrivValidatorFile())
	pvF.Save()
	//to load it up just like in the config
	// pvFile := privval.LoadFilePV(defaultTmConfig.PrivValidatorFile())

	nodeKey, err := p2p.LoadOrGenNodeKey(defaultTmConfig.NodeKeyFile())
	if err != nil {
		logging.Debug("Node Key generation issue")
		logging.Error(err.Error())
		// QUESTION(TEAM): Why is this error unhandled?
	}

	genDoc := tmtypes.GenesisDoc{
		ChainID:     fmt.Sprintf("test-chain-%v", "BLUBLU"),
		GenesisTime: time.Now(),
	}
	//add validators and persistant peers
	var temp []tmtypes.GenesisValidator
	var persistantPeersList []string
	for i := range nodeList {
		//convert pubkey X and Y to tmpubkey
		pubkeyBytes := RawPointToTMPubKey(nodeList[i].PublicKey.X, nodeList[i].PublicKey.Y)
		temp = append(temp, tmtypes.GenesisValidator{
			Address: pubkeyBytes.Address(),
			PubKey:  pubkeyBytes,
			Power:   1,
		})
		persistantPeersList = append(persistantPeersList, nodeList[i].TMP2PConnection)
	}
	defaultTmConfig.P2P.PersistentPeers = strings.Join(persistantPeersList, ",")

	logging.Infof("PERSISTANT PEERS: %s", defaultTmConfig.P2P.PersistentPeers)
	genDoc.Validators = temp

	logging.Infof("SAVED GENESIS FILE IN: %s", defaultTmConfig.GenesisFile())
	if err := genDoc.SaveAs(defaultTmConfig.GenesisFile()); err != nil {
		logging.Errorf("%s", err)
	}

	//Other changes to config go here
	defaultTmConfig.BaseConfig.DBBackend = "cleveldb"
	defaultTmConfig.FastSync = false
	defaultTmConfig.RPC.ListenAddress = suite.Config.BftURI
	defaultTmConfig.P2P.ListenAddress = suite.Config.TMP2PListenAddress
	defaultTmConfig.P2P.MaxNumInboundPeers = 300
	defaultTmConfig.P2P.MaxNumOutboundPeers = 300
	//TODO: change to true in production?
	defaultTmConfig.P2P.AddrBookStrict = false
	logging.Infof("NodeKey ID: %s", nodeKey.ID())

	//QUESTION(TEAM): Why do we save the config file?
	tmconfig.WriteConfigFile(defaultTmConfig.RootDir+"/config/config.toml", defaultTmConfig)

	n, err := tmnode.DefaultNewNode(defaultTmConfig, logger)
	if err != nil {
		logging.Fatalf("Failed to create tendermint node: %v", err)
	}

	suite.BftSuite.BftNode = n

	//Start Tendermint Node
	logging.Infof("Tendermint P2P Connection on: %s", defaultTmConfig.P2P.ListenAddress)
	logging.Infof("Tendermint Node RPC listening on: %s", defaultTmConfig.RPC.ListenAddress)
	if err := n.Start(); err != nil {
		logging.Fatalf("Failed to start tendermint node: %v", err)
	}
	logging.Infof("Started tendermint nodeInfo: %s", n.Switch().NodeInfo())

	//send back message saying ready
	time.Sleep(35 * time.Second)
	tmCoreMsgs <- "started_tmcore"

	<-idleConnsClosed
	return "Keygen complete.", nil
}

// func startTendermintCore(suite *Suite, buildPath string, nodeList []*NodeReference, tmCoreMsgs chan string, idleConnsClosed chan struct{}) (string, error) {

// 	//Starts tendermint node here
// 	//builds default config
// 	defaultTmConfig := tmconfig.DefaultConfig()
// 	defaultTmConfig.SetRoot(buildPath)
// 	// logger := tmlog.NewTMLogger(tmlog.NewSyncWriter(os.Stdout))
// 	// logger := NewTMLogger(suite.Config.LogLevel)
// 	logger := NoLogger{}

// 	defaultTmConfig.ProxyApp = suite.Config.ABCIServer

// 	//converts own pv to tendermint key TODO: Double check verification
// 	var pv tmsecp.PrivKeySecp256k1
// 	for i := 0; i < 32; i++ {
// 		pv[i] = suite.EthSuite.NodePrivateKey.D.Bytes()[i]
// 	}

// 	pvF := privval.GenFilePVFromPrivKey(pv, defaultTmConfig.PrivValidatorFile())
// 	pvF.Save()
// 	//to load it up just like in the config
// 	// pvFile := privval.LoadFilePV(defaultTmConfig.PrivValidatorFile())

// 	nodeKey, err := p2p.LoadOrGenNodeKey(defaultTmConfig.NodeKeyFile())
// 	if err != nil {
// 		logging.Debug("Node Key generation issue")
// 		logging.Error(err.Error())
// 		// QUESTION(TEAM): Why is this error unhandled?
// 	}

// 	genDoc := tmtypes.GenesisDoc{
// 		ChainID:     fmt.Sprintf("test-chain-%v", "BLUBLU"),
// 		GenesisTime: time.Now(),
// 	}
// 	//add validators and persistant peers
// 	var temp []tmtypes.GenesisValidator
// 	var persistantPeersList []string
// 	for i := range nodeList {
// 		//convert pubkey X and Y to tmpubkey
// 		pubkeyBytes := RawPointToTMPubKey(nodeList[i].PublicKey.X, nodeList[i].PublicKey.Y)
// 		temp = append(temp, tmtypes.GenesisValidator{
// 			Address: pubkeyBytes.Address(),
// 			PubKey:  pubkeyBytes,
// 			Power:   1,
// 		})
// 		persistantPeersList = append(persistantPeersList, nodeList[i].TMP2PConnection)
// 	}
// 	defaultTmConfig.P2P.PersistentPeers = strings.Join(persistantPeersList, ",")

// 	logging.Infof("PERSISTANT PEERS: %s", defaultTmConfig.P2P.PersistentPeers)
// 	genDoc.Validators = temp

// 	logging.Infof("SAVED GENESIS FILE IN: %s", defaultTmConfig.GenesisFile())
// 	if err := genDoc.SaveAs(defaultTmConfig.GenesisFile()); err != nil {
// 		logging.Errorf("%s", err)
// 	}

// 	//Other changes to config go here
// 	defaultTmConfig.BaseConfig.DBBackend = "cleveldb"
// 	defaultTmConfig.FastSync = false
// 	defaultTmConfig.RPC.ListenAddress = suite.Config.BftURI
// 	defaultTmConfig.P2P.ListenAddress = suite.Config.TMP2PListenAddress
// 	defaultTmConfig.P2P.MaxNumInboundPeers = 300
// 	defaultTmConfig.P2P.MaxNumOutboundPeers = 300
// 	//TODO: change to true in production?
// 	defaultTmConfig.P2P.AddrBookStrict = false
// 	logging.Infof("NodeKey ID: %s", nodeKey.ID())

// 	//QUESTION(TEAM): Why do we save the config file?
// 	tmconfig.WriteConfigFile(defaultTmConfig.RootDir+"/config/config.toml", defaultTmConfig)

// 	n, err := tmnode.DefaultNewNode(defaultTmConfig, logger)
// 	if err != nil {
// 		logging.Fatalf("Failed to create tendermint node: %v", err)
// 	}

// 	suite.BftSuite.BftNode = n

// 	//Start Tendermint Node
// 	logging.Infof("Tendermint P2P Connection on: %s", defaultTmConfig.P2P.ListenAddress)
// 	logging.Infof("Tendermint Node RPC listening on: %s", defaultTmConfig.RPC.ListenAddress)
// 	if err := n.Start(); err != nil {
// 		logging.Fatalf("Failed to start tendermint node: %v", err)
// 	}
// 	logging.Infof("Started tendermint nodeInfo: %s", n.Switch().NodeInfo())

// 	//send back message saying ready
// 	tmCoreMsgs <- "started_tmcore"

// 	<-idleConnsClosed
// 	return "Keygen complete.", nil
// }

func ecdsaPttoPt(ecdsaPt *ecdsa.PublicKey) *common.Point {
	return &common.Point{X: *ecdsaPt.X, Y: *ecdsaPt.Y}
}
