package dkgnode

/* All useful imports */
import (
	"crypto/ecdsa"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math/big"
	"os"
	"strings"
	"time"

	"github.com/Rican7/retry"
	"github.com/Rican7/retry/backoff"
	"github.com/Rican7/retry/strategy"
	ethCommon "github.com/ethereum/go-ethereum/common"
	tmconfig "github.com/tendermint/tendermint/config"
	tmsecp "github.com/tendermint/tendermint/crypto/secp256k1"
	tmlog "github.com/tendermint/tendermint/libs/log"
	tmnode "github.com/tendermint/tendermint/node"
	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/privval"
	tmtypes "github.com/tendermint/tendermint/types"
	"github.com/torusresearch/torus/common"
	"github.com/torusresearch/torus/pvss"
	"github.com/torusresearch/torus/secp256k1"
	jsonrpcclient "github.com/ybbus/jsonrpc"
)

//TODO: rename nodePort
type NodeReference struct {
	Address       *ethCommon.Address
	JSONClient    jsonrpcclient.RPCClient
	Index         *big.Int
	PublicKey     *ecdsa.PublicKey
	P2PConnection string
}

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

type NoLogger struct {
	tmlog.Logger
}

func (NoLogger) Debug(msg string, keyvals ...interface{}) {
}

func (NoLogger) Info(msg string, keyvals ...interface{}) {
}

func (NoLogger) Error(msg string, keyvals ...interface{}) {
}

func (NoLogger) With(keyvals ...interface{}) tmlog.Logger {
	return NoLogger{}
}

func startTendermintCore(suite *Suite, buildPath string, nodeList []*NodeReference, tmCoreMsgs chan string) (string, error) {

	//Starts tendermint node here
	//builds default config
	defaultTmConfig := tmconfig.DefaultConfig()
	defaultTmConfig.SetRoot(buildPath)
	logger := tmlog.NewTMLogger(tmlog.NewSyncWriter(os.Stdout))
	// logger := NoLogger{}
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
		fmt.Println("Node Key generation issue")
		fmt.Println(err)
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
		persistantPeersList = append(persistantPeersList, nodeList[i].P2PConnection)
	}
	defaultTmConfig.P2P.PersistentPeers = strings.Join(persistantPeersList, ",")

	fmt.Println("PERSISTANT PEERS: ", defaultTmConfig.P2P.PersistentPeers)
	genDoc.Validators = temp

	fmt.Println("SAVED GENESIS FILE IN: ", defaultTmConfig.GenesisFile())
	if err := genDoc.SaveAs(defaultTmConfig.GenesisFile()); err != nil {
		fmt.Print(err)
	}

	//Other changes to config go here
	defaultTmConfig.BaseConfig.DBBackend = "cleveldb"
	defaultTmConfig.FastSync = false
	defaultTmConfig.RPC.ListenAddress = suite.Config.BftURI
	defaultTmConfig.P2P.ListenAddress = suite.Config.P2PListenAddress
	defaultTmConfig.P2P.MaxNumInboundPeers = 300
	defaultTmConfig.P2P.MaxNumOutboundPeers = 300
	//TODO: change to true in production?
	defaultTmConfig.P2P.AddrBookStrict = false
	defaultTmConfig.LogLevel = "*:error"
	fmt.Println("NodeKey ID: ", nodeKey.ID())
	//save config
	tmconfig.WriteConfigFile(defaultTmConfig.RootDir+"/config/config.toml", defaultTmConfig)

	n, err := tmnode.DefaultNewNode(defaultTmConfig, logger)

	suite.BftSuite.BftNode = n
	// n, err := tmnode.NewNode(
	// 	defaultTmConfig,
	// 	pvFile,
	// 	nodeKey,
	// 	tmproxy.DefaultClientCreator(defaultTmConfig.ProxyApp, defaultTmConfig.ABCI, defaultTmConfig.DBDir()),
	// 	ProvideGenDoc(&genDoc),
	// 	tmnode.DefaultDBProvider,
	// 	tmnode.DefaultMetricsProvider(defaultTmConfig.Instrumentation),
	// 	logger,
	// )

	if err != nil {
		log.Fatal("Failed to create tendermint node: %v", err)
	}

	//Start Tendermint Node
	fmt.Println("Tendermint P2P Connection on: ", defaultTmConfig.P2P.ListenAddress)
	fmt.Println("Tendermint Node RPC listening on: ", defaultTmConfig.RPC.ListenAddress)
	if err := n.Start(); err != nil {
		log.Fatal("Failed to start tendermint node: %v", err)
	}
	logger.Info("Started tendermint node", "nodeInfo", n.Switch().NodeInfo())

	//send back message saying ready
	tmCoreMsgs <- "started_tmcore"

	// Run forever, blocks goroutine
	select {}
	return "Keygen complete.", nil
}

func startKeyGeneration(suite *Suite, shareStartingIndex int, shareEndingIndex int) error {
	nodeList := suite.EthSuite.NodeList
	bftRPC := suite.BftSuite.BftRPC

	fmt.Println("Required number of nodes reached")
	fmt.Println("KEYGEN: Sending shares -----------")

	secretMapping := make(map[int]SecretStore)
	siMapping := make(map[int]SiStore)
	for shareIndex := shareStartingIndex; shareIndex < shareEndingIndex; shareIndex++ {
		nodes := make([]common.Node, suite.Config.NumberOfNodes)

		for i := 0; i < suite.Config.NumberOfNodes; i++ {
			nodes[i] = common.Node{int(nodeList[i].Index.Int64()), *ecdsaPttoPt(nodeList[i].PublicKey)}
		}

		// this is the secret zi generated by each node
		secret := pvss.RandomBigInt()

		// create shares and public polynomial commitment
		shares, pubpoly, err := pvss.CreateShares(nodes, *secret, suite.Config.Threshold)
		if err != nil {
			fmt.Println(err)
		}
		fmt.Println("Shares created", shares)
		// commit pubpoly by signing it and broadcasting it
		pubPolyTx := PubPolyBFTTx{
			PubPoly:    *pubpoly,
			Epoch:      suite.ABCIApp.state.Epoch,
			ShareIndex: uint(shareIndex),
		}

		wrapper := DefaultBFTTxWrapper{&pubPolyTx}

		// broadcast signed pubpoly
		var id *common.Hash
		action := func(attempt uint) error {
			fmt.Println("KEYGEN: trying to broadcast for shareIndex", shareIndex, "attempt", attempt)
			id, err = bftRPC.Broadcast(wrapper)
			if err != nil {
				fmt.Println("failed to fetch (attempt #%d) with error: %d", err)
				err = fmt.Errorf("failed to fetch (attempt #%d) with error: %d", err)
			}
			return err
		}
		err = retry.Retry(
			action,
			strategy.Backoff(backoff.Fibonacci(10*time.Millisecond)),
		)
		if nil != err {
			fmt.Println("Failed to publish pub poly with error %q", err)
		}
		fmt.Println("KEYGEN: broadcasted shareIndex", shareIndex)
		// signcrypt data
		signcryptedData := make([]*common.SigncryptedOutput, len(nodes))
		for index, share := range *shares {
			// serializing id + primary share value into bytes before signcryption
			var data []byte
			data = append(data, share.Value.Bytes()...)
			var broadcastIdBytes []byte
			broadcastIdBytes = append(broadcastIdBytes, id.Bytes()...)

			data = append(data, broadcastIdBytes...)
			signcryption, err := pvss.Signcrypt(nodes[index].PubKey, data, *suite.EthSuite.NodePrivateKey.D)
			// Code for using sqlite db for bft
			// broadcastIdBytes = append(broadcastIdBytes, big.NewInt(int64(id)).Bytes()...)
			// if len(broadcastIdBytes) == 1 {
			// 	broadcastIdBytes = append(make([]byte, 1), broadcastIdBytes...)
			// }
			// if err != nil {
			// 	fmt.Println("Failed during padding of broadcastId bytes")
			// }
			// data = append(data, broadcastIdBytes...) // length of big.Int is 2 bytes
			// signcryption, err := pvss.Signcrypt(nodes[index].PubKey, data, *suite.EthSuite.NodePrivateKey.D)
			if err != nil {
				fmt.Println("KEYGEN: Failed during signcryption", shareIndex)
			}
			signcryptedData[index] = &common.SigncryptedOutput{NodePubKey: nodes[index].PubKey, NodeIndex: share.Index, SigncryptedShare: *signcryption}
			fmt.Println("KEYGEN: Signcrypted", shareIndex)
		}
		errArr := sendSharesToNodes(suite, signcryptedData, nodeList, shareIndex)
		if errArr != nil {
			fmt.Println("errors sending shares")
			fmt.Println(errArr)
		}
		secretMapping[shareIndex] = SecretStore{secret, false}
	}
	fmt.Println("KEYGEN: broadcasting keygencomplete status")
	statusTx := StatusBFTTx{
		StatusType:  "keygen_complete",
		StatusValue: "Y",
		// TODO: make epoch variable
		Epoch:       suite.ABCIApp.state.Epoch,
		FromPubKeyX: suite.EthSuite.NodePublicKey.X.Text(16),
		FromPubKeyY: suite.EthSuite.NodePublicKey.Y.Text(16),
	}
	suite.BftSuite.BftRPC.Broadcast(DefaultBFTTxWrapper{&statusTx})

	// wait for websocket to be up
	for suite.BftSuite.BftRPCWSStatus != "up" {
		time.Sleep(1 * time.Second)
		fmt.Println("bftsuite websocket connection is not up")
	}

	for {
		time.Sleep(1 * time.Second)
		// TODO: make epoch variable
		allKeygenComplete := suite.ABCIApp.state.LocalStatus["all_keygen_complete"]
		if allKeygenComplete != "Y" {
			fmt.Println("KEYGEN: nodes have not finished sending shares for epoch, appstate", suite.ABCIApp.state)
			continue
		}
		fmt.Println("KEYGEN: all nodes have finished sending shares for epoch, appstate", suite.ABCIApp.state)
		suite.ABCIApp.state.LocalStatus["all_keygen_complete"] = ""
		// err := suite.BftSuite.DeregisterQuery("keygeneration.sharecollection='1'")
		// if err != nil {
		// 	fmt.Println("Could not deregister", err)
		// }
		break
	}

	// Signcrypted shares are received by the other nodes and handled in server.go

	fmt.Println("KEYGEN: STARTING TO GATHER SHARES AND PUT THEM TOGETHER", shareStartingIndex, shareEndingIndex)
	// gather shares, decrypt and verify with pubpoly
	// - check if shares are here
	// Approach: for each shareIndex, we gather all shares shared by nodes for that share index
	// we retrieve the broadcasted signature via the broadcastID for each share and verify its correct
	// we then addmod all shares and get our actual final share
	for shareIndex := shareStartingIndex; shareIndex < shareEndingIndex; shareIndex++ {
		var unsigncryptedShares []*big.Int
		var broadcastIdArray [][]byte
		var nodePubKeyArray []*ecdsa.PublicKey
		var nodeId []int
		for i := 0; i < suite.Config.NumberOfNodes; i++ { // TODO: inefficient, we are looping unnecessarily
			data, found := suite.CacheSuite.CacheInstance.Get(nodeList[i].Address.Hex() + "_MAPPING")
			if found {
				var shareMapping = data.(map[int]ShareLog)
				if val, ok := shareMapping[shareIndex]; ok {
					unsigncryptedShares = append(unsigncryptedShares, new(big.Int).SetBytes(val.UnsigncryptedShare))
					broadcastIdArray = append(broadcastIdArray, val.BroadcastId)
					nodePubKeyArray = append(nodePubKeyArray, nodeList[i].PublicKey)
					nodeId = append(nodeId, i+1)
				}
			} else {
				fmt.Println("Could not find mapping for node ", i)
				break
			}
		}
		// Retrieve previously broadcasted signed pubpoly data
		broadcastedDataArray := make([][]common.Point, len(broadcastIdArray))
		for index, broadcastId := range broadcastIdArray {
			fmt.Println("BROADCASTID WAS: ", broadcastId)

			pubPolyTx := PubPolyBFTTx{}
			wrappedPubPolyTx := DefaultBFTTxWrapper{&pubPolyTx}
			err := bftRPC.Retrieve(broadcastId, &wrappedPubPolyTx) // TODO: use a goroutine to run this concurrently
			if err != nil {
				fmt.Println("Could not retrieve broadcast")
				fmt.Println(err)
				continue
			}

			broadcastedDataArray[index] = pubPolyTx.PubPoly
		}

		// verify share against pubpoly
		//TODO: shift to function in pvss.go
		fmt.Println("KEYGEN: share verification", shareIndex)
		s := secp256k1.Curve
		for index, pubpoly := range broadcastedDataArray {
			var sumX, sumY = big.NewInt(int64(0)), big.NewInt(int64(0))
			var myNodeReference *NodeReference
			for _, noderef := range nodeList {
				if noderef.Address.Hex() == suite.EthSuite.NodeAddress.Hex() {
					myNodeReference = noderef
				}
			}
			nodeI := myNodeReference.Index
			fmt.Println("nodeI ", nodeI)
			for ind, pt := range pubpoly {
				x_i := new(big.Int)
				x_i.Exp(nodeI, big.NewInt(int64(ind)), secp256k1.GeneratorOrder)
				tempX, tempY := s.ScalarMult(&pt.X, &pt.Y, x_i.Bytes())
				sumX, sumY = s.Add(sumX, sumY, tempX, tempY)
			}
			fmt.Println("SHOULD EQL PUB", sumX, sumY)
			subshare := unsigncryptedShares[index]
			tempX, tempY := s.ScalarBaseMult(subshare.Bytes())
			fmt.Println("SHOULD EQL REC", tempX, tempY)
			if sumX.Text(16) != tempX.Text(16) || sumY.Text(16) != tempY.Text(16) {
				fmt.Println("Could not verify share from node")
			} else {
				fmt.Println("Share verified")
			}
		}

		fmt.Println("KEYGEN: storing Si")
		// form Si
		tempSi := new(big.Int)
		for i := range unsigncryptedShares {
			tempSi.Add(tempSi, unsigncryptedShares[i])
		}
		tempSi.Mod(tempSi, secp256k1.GeneratorOrder)
		var nodeIndex int
		for i := range unsigncryptedShares {
			if nodeList[i].Address.Hex() == suite.EthSuite.NodeAddress.Hex() {
				nodeIndex = int(nodeList[i].Index.Int64())
			}
		}
		si := SiStore{Index: nodeIndex, Value: tempSi}
		fmt.Println("STORED Si: ", shareIndex)
		siMapping[shareIndex] = si
	}
	fmt.Println("KEYGEN: replacing Simapping and secret mapping")
	if previousSiMapping, found := suite.CacheSuite.CacheInstance.Get("Si_MAPPING"); !found {
		suite.CacheSuite.CacheInstance.Set("Si_MAPPING", siMapping, -1)
	} else {
		for k, v := range previousSiMapping.(map[int]SiStore) {
			siMapping[k] = v
			suite.CacheSuite.CacheInstance.Set("Si_MAPPING", siMapping, -1)
		}
	}

	if previousSecretMapping, found := suite.CacheSuite.CacheInstance.Get("Secret_MAPPING"); !found {
		suite.CacheSuite.CacheInstance.Set("Secret_MAPPING", secretMapping, -1)
	} else {
		for k, v := range previousSecretMapping.(map[int]SecretStore) {
			secretMapping[k] = v
			suite.CacheSuite.CacheInstance.Set("Secret_MAPPING", secretMapping, -1)
		}
	}
	//save cache
	cacheItems := suite.CacheSuite.CacheInstance.Items()
	cacheJSON, err := json.Marshal(cacheItems)
	if err != nil {
		fmt.Println(err)
	}
	err = ioutil.WriteFile(suite.Config.BuildPath+"/"+time.Now().String()+"_secrets.json", cacheJSON, 0644)
	if err != nil {
		fmt.Println(err)
	}
	return err
}

func sendSharesToNodes(suite *Suite, signcryptedOutput []*common.SigncryptedOutput, nodeList []*NodeReference, shareIndex int) *[]error {
	fmt.Println("KEYGEN: SHARES BEING SENT TO OTHER NODES", len(signcryptedOutput))
	errorSlice := make([]error, len(signcryptedOutput))
	// fmt.Println("GIVEN SIGNCRYPTION")
	// fmt.Println(signcryptedOutput[0].SigncryptedShare.Ciphertext)
	for i := range signcryptedOutput {
		for j := range signcryptedOutput { // TODO: this is because we aren't sure about the ordering of nodeList/signcryptedOutput...
			if signcryptedOutput[i].NodePubKey.X.Cmp(nodeList[j].PublicKey.X) == 0 {
				// send shares through bft
				broadcastMessage := KeyGenShareBFTTx{
					SigncryptedMessage{
						suite.EthSuite.NodeAddress.Hex(),
						suite.EthSuite.NodePublicKey.X.Text(16),
						suite.EthSuite.NodePublicKey.Y.Text(16),
						signcryptedOutput[i].NodePubKey.X.Text(16),
						signcryptedOutput[i].NodePubKey.Y.Text(16),
						hex.EncodeToString(signcryptedOutput[i].SigncryptedShare.Ciphertext),
						signcryptedOutput[i].SigncryptedShare.R.X.Text(16),
						signcryptedOutput[i].SigncryptedShare.R.Y.Text(16),
						signcryptedOutput[i].SigncryptedShare.Signature.Text(16),
						uint(shareIndex),
					},
				}
				_, err := suite.BftSuite.BftRPC.Broadcast(DefaultBFTTxWrapper{broadcastMessage})
				if err != nil {
					fmt.Println("KEYGEN: FAILED TO BROADCAST", shareIndex, "msg", broadcastMessage)
				}
			}
		}
	}
	if errorSlice[0] == nil {
		return nil
	}
	return &errorSlice
}

func ecdsaPttoPt(ecdsaPt *ecdsa.PublicKey) *common.Point {
	return &common.Point{X: *ecdsaPt.X, Y: *ecdsaPt.Y}
}

func connectToJSONRPCNode(suite *Suite, epoch big.Int, nodeAddress ethCommon.Address) (*NodeReference, error) {
	details, err := suite.EthSuite.NodeListContract.AddressToNodeDetailsLog(nil, nodeAddress, &epoch)
	if err != nil {
		return nil, err
	}

	// if in production use https
	var nodeIPAddress string
	if suite.Flags.Production {
		nodeIPAddress = "https://" + details.DeclaredIp + "/jrpc"
	} else {
		nodeIPAddress = "http://" + details.DeclaredIp + "/jrpc"
	}
	rpcClient := jsonrpcclient.NewClient(nodeIPAddress)

	_, err = rpcClient.Call("Ping", &Message{suite.EthSuite.NodeAddress.Hex()})
	if err != nil {
		return nil, err
	}
	return &NodeReference{
		Address:       &nodeAddress,
		JSONClient:    rpcClient,
		Index:         details.Position,
		PublicKey:     &ecdsa.PublicKey{Curve: suite.EthSuite.secp, X: details.PubKx, Y: details.PubKy},
		P2PConnection: details.NodePort,
	}, nil
}
