package dkgnode

//TODO: export all "tm" imports to common folder
import (
	"fmt"
	"log"
	"math/big"

	tmbtcec "github.com/tendermint/btcd/btcec"
	tmsecp "github.com/tendermint/tendermint/crypto/secp256k1"
	tmnode "github.com/tendermint/tendermint/node"
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
	err := SetUpEth(&suite)
	if err != nil {
		log.Fatal(err)
	}
	go RunABCIServer(&suite)
	SetUpBftRPC(&suite)
	SetUpCache(&suite)
	var nodeIPAddress string
	/*
		//Starts tendermint node here
		//TODO: Abstract to function?
		//builds default config
		defaultTmConfig := tmconfig.DefaultConfig()
		defaultTmConfig.SetRoot(buildPath)
		fmt.Println("ROOT DIR", defaultTmConfig.RootDir)
		logger := tmlog.NewTMLogger(tmlog.NewSyncWriter(os.Stdout))
		fmt.Println("Node key file: ", defaultTmConfig.NodeKeyFile())
		// defaultTmConfig.NodeKey = "config/"
		// defaultTmConfig.PrivValidator = "config/"
		defaultTmConfig.ProxyApp = suite.Config.ABCIServer

		//converts own pv to tendermint key TODO: Double check verification
		var testpv tmsecp.PrivKeySecp256k1
		for i := range suite.EthSuite.NodePrivateKey.D.Bytes() {
			testpv[i] = suite.EthSuite.NodePrivateKey.D.Bytes()[i]
		}
		//SEEMS RIGHT (there are some bytes earlier but they use btcecc)
		//From their docs
		// PubKeySecp256k1Size is comprised of 32 bytes for one field element
		// (the x-coordinate), plus one byte for the parity of the y-coordinate.
		fmt.Println("ETH PUB KEY: ", suite.EthSuite.NodePublicKey.X.Bytes())
		fmt.Println("TM PUB KEY: ", testpv.PubKey().Bytes())

		//convert pubkey X and Y to tmpubkey
		pubkeyBytes := RawPointToTMPubKey(suite.EthSuite.NodePublicKey.X, suite.EthSuite.NodePublicKey.Y)

		fmt.Println("DOEST IT EQUAL", pubkeyBytes.Equals(testpv.PubKey()))

		// pv := &privval.FilePV{
		// 	Address:  testpv.PubKey().Address(),
		// 	PubKey:   testpv.PubKey(),
		// 	PrivKey:  testpv,
		// 	LastStep: int8(0),
		// 	// filePath: defaultTmConfig.PrivValidatorFile(),
		// }

		// pv := privval.GenFilePVSecp(defaultTmConfig.PrivValidatorFile())
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
			Address: pubkeyBytes.Address(),
			PubKey:  pubkeyBytes,
			Power:   10,
		}}

		if err := genDoc.SaveAs(defaultTmConfig.GenesisFile()); err != nil {
			fmt.Print(err)
		}

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

	*/

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
	/*

		//Start Tendermint Node
		fmt.Println("Tendermint Node listening on: ", defaultTmConfig.RPC.ListenAddress)
		if err := n.Start(); err != nil {
			log.Fatal("Failed to start tendermint node: %v", err)
		}
		logger.Info("Started tendermint node", "nodeInfo", n.Switch().NodeInfo())
	*/

	go keyGenerationPhase(&suite, buildPath)

	setUpServer(&suite, string(suite.Config.MyPort))
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
