package dkgnode

/* All useful imports */
import (
	"fmt"
	"log"
	"os"
	"time"

	tmconfig "github.com/tendermint/tendermint/config"
	tmlog "github.com/tendermint/tendermint/libs/log"
	tmnode "github.com/tendermint/tendermint/node"
	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/privval"
	tmproxy "github.com/tendermint/tendermint/proxy"
	tmtypes "github.com/tendermint/tendermint/types"
)

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
func New(configPath string, register bool, production bool) {

	//Main suite of functions used in node
	suite := Suite{}
	suite.Flags = &Flags{production}
	fmt.Println(configPath)
	loadConfig(&suite, configPath)
	err := SetUpEth(&suite)
	if err != nil {
		log.Fatal(err)
	}
	go RunABCIServer(&suite)
	SetUpBftRPC(&suite)
	SetUpCache(&suite)
	var nodeIPAddress string

	//Starts tendermint node here
	//TODO: Abstract to function?
	//builds default config
	defaultTmConfig := tmconfig.DefaultConfig()
	fmt.Println("ROOT DIR", defaultTmConfig.RootDir)
	logger := tmlog.NewTMLogger(tmlog.NewSyncWriter(os.Stdout))
	fmt.Println("Node key file: ", defaultTmConfig.NodeKeyFile())
	// defaultTmConfig.NodeKey = "config/"
	// defaultTmConfig.PrivValidator = "config/"
	defaultTmConfig.ProxyApp = suite.Config.ABCIServer

	pv := privval.GenFilePVSecp(defaultTmConfig.PrivValidatorFile())
	nodeKey, err := p2p.LoadOrGenNodeKey(defaultTmConfig.NodeKeyFile())
	if err != nil {
		fmt.Println(err)
	}

	genDoc := tmtypes.GenesisDoc{
		ChainID:         fmt.Sprintf("test-chain-%v", "BLUBLU"),
		GenesisTime:     time.Now(),
		ConsensusParams: tmtypes.DefaultConsensusParams(),
	}
	genDoc.Validators = []tmtypes.GenesisValidator{{
		Address: pv.GetPubKey().Address(),
		PubKey:  pv.GetPubKey(),
		Power:   10,
	}}

	defaultTmConfig.RPC.ListenAddress = suite.Config.BftURI

	n, err := tmnode.NewNode(defaultTmConfig,
		privval.LoadOrGenFilePV(defaultTmConfig.PrivValidatorFile()),
		nodeKey,
		tmproxy.DefaultClientCreator(defaultTmConfig.ProxyApp, defaultTmConfig.ABCI, defaultTmConfig.DBDir()),
		ProvideGenDoc(&genDoc),
		tmnode.DefaultDBProvider,
		tmnode.DefaultMetricsProvider(defaultTmConfig.Instrumentation),
		logger,
	)

	// n, err := tmnode.DefaultNewNode(defaultTmConfig, logger)

	if err != nil {
		log.Fatal("Failed to create tendermint node: %v", err)
	}

	if production {
		fmt.Println("//PRODUCTION MDOE ")
		nodeIPAddress = suite.Config.HostName + ":" + string(suite.Config.MyPort)
	} else {
		fmt.Println("//DEVELOPMENT MDOE ")
		nodeIP, err := findExternalIP()
		if err != nil {
			fmt.Println(err)
		}
		nodeIPAddress = nodeIP + ":" + string(suite.Config.MyPort)
	}

	fmt.Println("Node IP Address: " + nodeIPAddress)
	if register {
		/* Register Node */
		fmt.Println("Registering node...")
		_, err = suite.EthSuite.registerNode(nodeIPAddress)
		if err != nil {
			log.Fatal(err)
		}
	}

	//Start Tendermint Node
	fmt.Println("Tendermint Node listening on: ", defaultTmConfig.RPC.ListenAddress)
	if err := n.Start(); err != nil {
		log.Fatal("Failed to start tendermint node: %v", err)
	}
	logger.Info("Started tendermint node", "nodeInfo", n.Switch().NodeInfo())

	go keyGenerationPhase(&suite)
	setUpServer(&suite, string(suite.Config.MyPort))
}

func ProvideGenDoc(doc *tmtypes.GenesisDoc) tmnode.GenesisDocProvider {
	return func() (*tmtypes.GenesisDoc, error) {
		return doc, nil
	}
}
