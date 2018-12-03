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
	"strconv"
	"strings"
	"time"

	"github.com/Rican7/retry"
	"github.com/Rican7/retry/backoff"
	"github.com/Rican7/retry/strategy"
	"github.com/YZhenY/torus/common"
	"github.com/YZhenY/torus/pvss"
	"github.com/YZhenY/torus/secp256k1"
	ethCommon "github.com/ethereum/go-ethereum/common"
	tmconfig "github.com/tendermint/tendermint/config"
	tmsecp "github.com/tendermint/tendermint/crypto/secp256k1"
	tmlog "github.com/tendermint/tendermint/libs/log"
	tmnode "github.com/tendermint/tendermint/node"
	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/privval"
	tmtypes "github.com/tendermint/tendermint/types"
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

type NodeListUpdates struct {
	Type    string
	Payload interface{}
}

type KeyGenUpdates struct {
	Type    string
	Payload interface{}
}

func startKeyGenerationMonitor(suite *Suite, keyGenMonitorUpdates chan KeyGenUpdates) {
	for {
		fmt.Println("in start keygen monitor")
		time.Sleep(1 * time.Second)
		if suite.ABCIApp.state.LocalStatus["all_initiate_keygen"] != "" {
			fmt.Println("WAITING FOR ALL INITIATE KEYGEN TO STOP BEING IN PROGRESS")
			continue
		}
		percentLeft := 100 * (suite.ABCIApp.state.LastCreatedIndex - suite.ABCIApp.state.LastUnassignedIndex) / uint(suite.Config.KeysPerEpoch)
		if percentLeft > 40 {
			continue
		}
		startingIndex := int(suite.ABCIApp.state.LastCreatedIndex)
		endingIndex := suite.Config.KeysPerEpoch + int(suite.ABCIApp.state.LastCreatedIndex)

		initiateKeyGenerationStatusWrapper := DefaultBFTTxWrapper{
			StatusBFTTx{
				FromPubKeyX: suite.EthSuite.NodePublicKey.X.Text(16),
				FromPubKeyY: suite.EthSuite.NodePublicKey.Y.Text(16),
				Epoch:       suite.ABCIApp.state.Epoch,
				StatusType:  "initiate_keygen",
				StatusValue: "Y",
				Data:        []byte(strconv.Itoa(endingIndex)),
			},
		}
		_, err := suite.BftSuite.BftRPC.Broadcast(initiateKeyGenerationStatusWrapper)
		if err != nil {
			fmt.Println("coudl not broadcast initiateKeygeneration", err)
			continue
		}

		for {
			fmt.Println("WAITING FOR ALL INITIATE KEYGEN TO BE Y")
			fmt.Println(suite.ABCIApp.state)
			time.Sleep(1 * time.Second)
			if suite.ABCIApp.state.LocalStatus["all_initiate_keygen"] == "Y" {
				//reset keygen flag
				suite.ABCIApp.state.LocalStatus["all_initiate_keygen"] = "IP"
				//report back to main process
				keyGenMonitorUpdates <- KeyGenUpdates{
					Type:    "start_keygen",
					Payload: []int{startingIndex, endingIndex},
				}
				break
			}
		}

	}
}

func startNodeListMonitor(suite *Suite, nodeListUpdates chan NodeListUpdates) {
	for {
		fmt.Println("Checking Node List...")
		// Fetch Node List from contract address
		//TODO: make epoch vairable
		epoch := big.NewInt(int64(0))
		ethList, positions, err := suite.EthSuite.NodeListContract.ViewNodes(nil, epoch)
		// If we can't reach ethereum node, lets try next time
		if err != nil {
			fmt.Println("Could not View Nodes on ETH Network", err)
		} else {
			// Build count of nodes connected to
			fmt.Println("Indexes", positions, ethList)
			connectedNodes := 0
			nodeList := make([]*NodeReference, len(ethList))
			if len(ethList) > 0 {
				for i := range ethList {
					// Check if node is online by pinging
					temp, err := connectToJSONRPCNode(suite, *epoch, ethList[i])
					if err != nil {
						fmt.Println(err)
					}

					if temp != nil {
						if nodeList[int(temp.Index.Int64())-1] == nil {
							nodeList[int(temp.Index.Int64())-1] = temp
						}
						connectedNodes++
					}
				}
			}
			//if we've connected to all nodes we send back the most recent list
			if connectedNodes == len(ethList) {
				nodeListUpdates <- NodeListUpdates{"update", nodeList}
			}
		}
		time.Sleep(1 * time.Second) //check node list every second for updates
	}
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

	bftRPC := suite.BftSuite.BftRPC
	//for testing purposes
	//TODO: FIX
	time.Sleep(10 * time.Second)
	if suite.Config.MyPort == "8001" {
		epochTxWrapper := DefaultBFTTxWrapper{
			&EpochBFTTx{uint(1)},
		}
		go func() {
			time.Sleep(time.Second * 10)
			_, err := bftRPC.Broadcast(epochTxWrapper)
			if err != nil {
				fmt.Println("error broadcasting epoch: ", err)
			} else {
				fmt.Println("updated epoch to 1")
			}
		}()
	}

	//Starts tendermint node here
	//builds default config
	defaultTmConfig := tmconfig.DefaultConfig()
	defaultTmConfig.SetRoot(buildPath)
	fmt.Println("TM BFT DATA ROOT STORE", defaultTmConfig.RootDir)
	//build root folders, done in dkg node now
	// os.MkdirAll(defaultTmConfig.RootDir+"/config", os.ModePerm)
	logger := tmlog.NewTMLogger(tmlog.NewSyncWriter(os.Stdout))
	// logger := NoLogger{}
	fmt.Println("Node key file: ", defaultTmConfig.NodeKeyFile())
	// defaultTmConfig.NodeKey = "config/"
	// defaultTmConfig.PrivValidator = "config/"
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

	// pv := privval.GenFilePVSecp(defaultTmConfig.PrivValidatorFile())
	nodeKey, err := p2p.LoadOrGenNodeKey(defaultTmConfig.NodeKeyFile())
	if err != nil {
		fmt.Println("Node Key generation issue")
		fmt.Println(err)
	}

	genDoc := tmtypes.GenesisDoc{
		ChainID:     fmt.Sprintf("test-chain-%v", "BLUBLU"),
		GenesisTime: time.Now(),
		// ConsensusParams: tmtypes.DefaultConsensusParams(),
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
	//TODO: CHange back, for testing purposes we limit to 5
	genDoc.Validators = temp[:5]

	fmt.Println("SAVED GENESIS FILE IN: ", defaultTmConfig.GenesisFile())
	if err := genDoc.SaveAs(defaultTmConfig.GenesisFile()); err != nil {
		fmt.Print(err)
	}

	//Other changes to config go here
	defaultTmConfig.FastSync = false
	defaultTmConfig.RPC.ListenAddress = suite.Config.BftURI
	defaultTmConfig.P2P.ListenAddress = suite.Config.P2PListenAddress
	defaultTmConfig.P2P.MaxNumInboundPeers = 300
	defaultTmConfig.P2P.MaxNumOutboundPeers = 300
	//TODO: change to true in production?
	defaultTmConfig.P2P.AddrBookStrict = false
	// defaultTmConfig.Consensus.CreateEmptyBlocks = false
	defaultTmConfig.LogLevel = "*:error"
	// err = defaultTmConfig.ValidateBasic()
	// if err != nil {
	// 	fmt.Println("VALIDATEBASIC FAILED: ", err)
	// }
	fmt.Println("NODEKEY: ", nodeKey)
	// fmt.Println(nodeKey.ID())
	fmt.Println(nodeKey.PubKey().Address())
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
	// if suite.Config.MyPort == "8001" {
	// 	epochTxWrapper := DefaultBFTTxWrapper{
	// 		&EpochBFTTx{uint(1)},
	// 	}
	// 	_, err := bftRPC.Broadcast(epochTxWrapper)
	// 	if err != nil {
	// 		fmt.Println("error broadcasting epoch: ", err)
	// 	}
	// }
	fmt.Println("Required number of nodes reached")
	fmt.Println("Sending shares -----------")

	secretMapping := make(map[int]SecretStore)
	siMapping := make(map[int]common.PrimaryShare)
	for shareIndex := shareStartingIndex; shareIndex < shareEndingIndex; shareIndex++ {
		nodes := make([]common.Node, suite.Config.NumberOfNodes)

		for i := 0; i < suite.Config.NumberOfNodes; i++ {
			nodes[i] = common.Node{int(nodeList[i].Index.Int64()), *ecdsaPttoPt(nodeList[i].PublicKey)}
		}

		// this is the secret zi generated by each node
		secret := pvss.RandomBigInt()
		// fmt.Println("Node "+suite.EthSuite.NodeAddress.Hex(), " Secret: ", secret.Text(16))

		// create shares and public polynomial commitment
		shares, pubpoly, err := pvss.CreateShares(nodes, *secret, suite.Config.Threshold)
		if err != nil {
			fmt.Println(err)
		}

		// commit pubpoly by signing it and broadcasting it

		//TODO: Make epoch variable
		pubPolyTx := PubPolyBFTTx{
			PubPoly:    *pubpoly,
			Epoch:      uint(0),
			ShareIndex: uint(shareIndex),
		}

		wrapper := DefaultBFTTxWrapper{&pubPolyTx}

		//Commented out ECDSA Verification for now. Need to check out tm signing on chain
		// ecdsaSignature := ECDSASign(arrBytes, suite.EthSuite.NodePrivateKey) // TODO: check if it matches on-chain implementation
		// pubPolyProof := PubPolyProof{EcdsaSignature: ecdsaSignature, PointsBytesArray: arrBytes}

		// broadcast signed pubpoly
		var id *common.Hash
		action := func(attempt uint) error {
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

		// id, err := bftRPC.Broadcast(wrapper)
		// if err != nil {
		// 	fmt.Println("Can't broadcast signed pubpoly")
		// 	fmt.Println(err)
		// }
		// fmt.Println(id)

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
				fmt.Println("Failed during signcryption")
			}
			signcryptedData[index] = &common.SigncryptedOutput{NodePubKey: nodes[index].PubKey, NodeIndex: share.Index, SigncryptedShare: *signcryption}
		}

		errArr := sendSharesToNodes(suite, signcryptedData, nodeList, shareIndex)
		if errArr != nil {
			fmt.Println("errors sending shares")
			fmt.Println(errArr)
		}
		secretMapping[shareIndex] = SecretStore{secret, false}
		statusTx := StatusBFTTx{
			StatusType:  "keygen_complete",
			StatusValue: "Y",
			// TODO: make epoch variable
			Epoch:       suite.ABCIApp.state.Epoch,
			FromPubKeyX: suite.EthSuite.NodePublicKey.X.Text(16),
			FromPubKeyY: suite.EthSuite.NodePublicKey.Y.Text(16),
		}
		suite.BftSuite.BftRPC.Broadcast(DefaultBFTTxWrapper{&statusTx})
	}

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
			fmt.Println("nodes have not finished sending shares for epoch 0")
			continue
		}
		fmt.Println("all nodes have finished sending shares for epoch 0")
		suite.ABCIApp.state.LocalStatus["all_keygen_complete"] = ""
		break
	}

	// Signcrypted shares are received by the other nodes and handled in server.go

	// time.Sleep(60 * time.Second) // TODO: Check for communication termination from all other nodes
	fmt.Println("STARTING TO GATHER SHARES AND PUT THEM TOGETHER")
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

			// ECDSA COMMENTED OUT
			// hashedData := bytes32(secp256k1.Keccak256(data.PointsBytesArray))
			// if bytes.Compare(data.EcdsaSignature.Hash[:], hashedData[:]) != 0 {
			// 	fmt.Println("Signed hash does not match retrieved hash")
			// 	fmt.Println(data.EcdsaSignature.Hash[:])
			// 	fmt.Println(hashedData[:])
			// 	continue
			// }
			// if !ECDSAVerify(*nodePubKeyArray[index], data.EcdsaSignature) {
			// 	fmt.Println("Signature does not verify")
			// 	continue
			// } else {
			// 	fmt.Println("Signature of pubpoly verified")
			// }

			//TODO: Check epoch number and share index against tx received
			broadcastedDataArray[index] = pubPolyTx.PubPoly
		}

		// verify share against pubpoly
		//TODO: shift to function in pvss.go
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
		si := common.PrimaryShare{Index: nodeIndex, Value: *tempSi}
		fmt.Println("STORED Si: ", shareIndex)
		siMapping[shareIndex] = si
	}
	suite.CacheSuite.CacheInstance.Set("Si_MAPPING", siMapping, -1)
	suite.CacheSuite.CacheInstance.Set("Secret_MAPPING", secretMapping, -1)
	//save cache
	cacheItems := suite.CacheSuite.CacheInstance.Items()
	cacheJSON, err := json.Marshal(cacheItems)
	if err != nil {
		fmt.Println(err)
	}
	err = ioutil.WriteFile("cache.json", cacheJSON, 0644)
	if err != nil {
		fmt.Println(err)
	}
	return err
}

func sendSharesToNodes(suite *Suite, signcryptedOutput []*common.SigncryptedOutput, nodeList []*NodeReference, shareIndex int) *[]error {
	fmt.Println("SHARES BEING SENT TO OTHER NODES")
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
				suite.BftSuite.BftRPC.Broadcast(DefaultBFTTxWrapper{broadcastMessage})
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

	// TODO: possible replace with signature?
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
