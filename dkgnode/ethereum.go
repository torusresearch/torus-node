package dkgnode

/* All useful imports */
import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/torusresearch/torus-node/telemetry"

	"github.com/avast/retry-go"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	ethCommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	ethCrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/libp2p/go-libp2p-core/peer"
	ma "github.com/multiformats/go-multiaddr"
	logging "github.com/sirupsen/logrus"
	"github.com/torusresearch/bijson"
	tmp2p "github.com/torusresearch/tendermint/p2p"
	"github.com/torusresearch/torus-common/common"
	"github.com/torusresearch/torus-common/crypto"
	"github.com/torusresearch/torus-common/secp256k1"
	pcmn "github.com/torusresearch/torus-node/common"
	"github.com/torusresearch/torus-node/config"
	"github.com/torusresearch/torus-node/eventbus"
	"github.com/torusresearch/torus-node/idmutex"
	"github.com/torusresearch/torus-node/mapping"
	"github.com/torusresearch/torus-node/pss"
	nodelist "github.com/torusresearch/torus-node/solidity/goContracts"
)

type epochInfo struct {
	Id        big.Int
	N         big.Int
	K         big.Int
	T         big.Int
	PrevEpoch big.Int
	NextEpoch big.Int
}

type EthNodeDetails struct {
	DeclaredIp         string
	Position           big.Int
	TmP2PListenAddress string
	P2pListenAddress   string
}

type EthEpochParams struct {
	EpochID int
	N       int
	T       int
	K       int
}
type NodeRegister struct {
	AllConnected bool
	PSSSending   bool
	NodeList     []*NodeReference
}

type NodeReference struct {
	Address         *ethCommon.Address
	Index           *big.Int
	PeerID          peer.ID
	PublicKey       *ecdsa.PublicKey
	TMP2PConnection string
	P2PConnection   string
}

type SerializedNodeReference struct {
	Address         [20]byte
	Index           big.Int
	PeerID          string
	PublicKey       common.Point
	TMP2PConnection string
	P2PConnection   string
}

func (nodeRef NodeReference) Serialize() SerializedNodeReference {
	var nodeRefAddress [20]byte
	var nodeRefIndex big.Int
	var nodeRefPublicKey common.Point
	if nodeRef.Address != nil {
		nodeRefAddress = *nodeRef.Address
	}
	if nodeRef.Index != nil {
		nodeRefIndex = *nodeRef.Index
	}
	if nodeRef.PublicKey != nil {
		nodeRefPublicKey = common.Point{
			X: *nodeRef.PublicKey.X,
			Y: *nodeRef.PublicKey.Y,
		}
	}
	return SerializedNodeReference{
		Address:         nodeRefAddress,
		Index:           nodeRefIndex,
		PeerID:          string(nodeRef.PeerID),
		PublicKey:       nodeRefPublicKey,
		TMP2PConnection: nodeRef.TMP2PConnection,
		P2PConnection:   nodeRef.P2PConnection,
	}
}

func (nodeRef NodeReference) Deserialize(serializedNodeRef SerializedNodeReference) NodeReference {
	addr := ethCommon.Address(serializedNodeRef.Address)
	nodeRef.Address = &addr
	nodeRef.Index = &serializedNodeRef.Index
	nodeRef.PeerID = peer.ID(serializedNodeRef.PeerID)
	nodeRef.PublicKey = crypto.PointToECDSAPublicKey(serializedNodeRef.PublicKey)
	nodeRef.TMP2PConnection = serializedNodeRef.TMP2PConnection
	nodeRef.P2PConnection = serializedNodeRef.P2PConnection
	return nodeRef
}

func (nr *NodeRegister) GetNodeByIndex(index int) *NodeReference {
	for _, nodeRef := range nr.NodeList {
		if int(nodeRef.Index.Int64()) == index {
			return nodeRef
		}
	}
	return nil
}

func (e *EthereumService) GetNodeRef(nodeAddress ethCommon.Address) (n *NodeReference, err error) {
	details, err := e.nodeList.NodeDetails(nil, nodeAddress)
	if err != nil {
		return nil, err
	}

	var connectionDetails ConnectionDetails
	if details.DeclaredIp != "" && details.P2pListenAddress == "" || details.TmP2PListenAddress == "" {
		err = retry.Do(func() error {
			var retryErr error
			connectionDetails, retryErr = e.serviceLibrary.ServerMethods().RequestConnectionDetails(details.DeclaredIp)
			logging.WithField("connectionDetails", connectionDetails).Debug("got back connection details from node")
			if retryErr != nil {
				return fmt.Errorf("could not get hidden connection details %v", retryErr)
			}
			retryErr = e.serviceLibrary.DatabaseMethods().StoreConnectionDetails(nodeAddress, connectionDetails)
			if retryErr != nil {
				return fmt.Errorf("could not store connection details %v", retryErr)
			}
			return nil
		})
		if err != nil {
			logging.WithField("nodeAddress", nodeAddress).WithError(err).Error("could not get connection details from node, get from DB")
			connectionDetails, err = e.serviceLibrary.DatabaseMethods().RetrieveConnectionDetails(nodeAddress)
			if err != nil {
				logging.WithField("nodeAddress", nodeAddress).Error("could not get connection details from DB either")
				return nil, fmt.Errorf("unable to get connection details for nodeAddress %v", nodeAddress)
			}
		}
	} else {
		connectionDetails = ConnectionDetails{
			TMP2PConnection: details.TmP2PListenAddress,
			P2PConnection:   details.P2pListenAddress,
		}
	}

	ipfsaddr, err := ma.NewMultiaddr(connectionDetails.P2PConnection)
	if err != nil {
		logging.WithError(err).Error("could not get ipfsaddr")
		return nil, err
	}

	pid, err := ipfsaddr.ValueForProtocol(ma.P_IPFS)
	if err != nil {
		logging.WithError(err).Error("could not get pid")
		return nil, err
	}

	peerid, err := peer.IDB58Decode(pid)
	if err != nil {
		logging.WithError(err).Error("could not get peerid")
		return nil, err
	}
	return &NodeReference{
		Address:         &nodeAddress,
		PeerID:          peerid,
		Index:           details.Position,
		PublicKey:       &ecdsa.PublicKey{Curve: e.ethCurve, X: details.PubKx, Y: details.PubKy},
		TMP2PConnection: connectionDetails.TMP2PConnection,
		P2PConnection:   connectionDetails.P2PConnection,
	}, nil
}

func NewEthereumService(ctx context.Context, eventBus eventbus.Bus) *BaseService {
	ethereumCtx, cancel := context.WithCancel(context.WithValue(ctx, ContextID, "ethereum"))
	ethereumService := EthereumService{
		cancel:        cancel,
		parentContext: ctx,
		context:       ethereumCtx,
		eventBus:      eventBus,
	}
	ethereumService.serviceLibrary = NewServiceLibrary(ethereumService.eventBus, ethereumService.Name())
	return NewBaseService(&ethereumService)
}

type EthereumService struct {
	bs             *BaseService
	cancel         context.CancelFunc
	parentContext  context.Context
	context        context.Context
	eventBus       eventbus.Bus
	serviceLibrary ServiceLibrary

	nodePubK         *ecdsa.PublicKey
	nodePrivK        *ecdsa.PrivateKey
	nodeAddr         *ethCommon.Address
	tmp2pConnection  string
	p2pConnection    string
	ethClient        *ethclient.Client
	ethCurve         elliptic.Curve
	nodeRegisterMap  map[int]*NodeRegister // only filled when nodes have all registered
	cachedEpochInfo  *cachedEpochInfoSyncMap
	nodeList         *nodelist.NodeList
	nodeIndex        int
	currentEpoch     int
	connectionStatus ConnectionStatus
	isWhitelisted    bool
	isRegistered     bool

	idmutex.Mutex
}

type cachedEpochInfoSyncMap struct {
	sync.Map
}

func (s *cachedEpochInfoSyncMap) Get(epoch int) (e epochInfo, found bool) {
	val, ok := s.Map.Load(epoch)
	if !ok {
		return e, false
	}
	return val.(epochInfo), true
}

func (s *cachedEpochInfoSyncMap) Set(epoch int, e epochInfo) {
	s.Map.Store(epoch, e)
}

type ConnectionStatus int

const (
	EthClientDisconnected ConnectionStatus = iota
	EthClientConnected
)

func (e *EthereumService) Name() string {
	return "ethereum"
}

func (e *EthereumService) OnStart() error {
	client, err := ethclient.Dial(config.GlobalConfig.EthConnection)
	if err != nil {
		return err
	}
	e.connectionStatus = EthClientConnected
	privateKeyECDSA, err := ethCrypto.HexToECDSA(string(config.GlobalConfig.EthPrivateKey))
	if err != nil {
		return err
	}
	nodePublicKey := privateKeyECDSA.Public()
	nodePublicKeyEC, ok := nodePublicKey.(*ecdsa.PublicKey)
	if !ok {
		return errors.New("error casting to Public Key")
	}
	nodeAddress := ethCrypto.PubkeyToAddress(*nodePublicKeyEC)
	nodeListAddress := ethCommon.HexToAddress(config.GlobalConfig.NodeListAddress)

	logging.WithFields(logging.Fields{
		"ETHConnection":  config.GlobalConfig.EthConnection,
		"nodePrivateKey": config.GlobalConfig.EthPrivateKey,
		"nodePublicKey":  nodeAddress.Hex(),
	}).Info("successful connection")

	NodeListContract, err := nodelist.NewNodeList(nodeListAddress, client)
	if err != nil {
		return err
	}
	e.nodePubK = nodePublicKeyEC
	e.nodePrivK = privateKeyECDSA
	e.nodeAddr = &nodeAddress
	e.ethClient = client
	e.ethCurve = secp256k1.Curve
	e.nodeRegisterMap = make(map[int]*NodeRegister)
	e.cachedEpochInfo = &cachedEpochInfoSyncMap{}
	e.nodeList = NodeListContract
	e.nodeIndex = 0 // will be set after it has been registered
	e.currentEpoch = config.GlobalConfig.InitEpoch

	// Check if node is whitelisted
	go whitelistMonitor(e)

	// Register node
	go registerNode(e)

	// Start current nodes monitor
	go currentNodesMonitor(e)

	// Start PSS related monitors
	go e.startPSSMonitor()

	go func() {
		<-e.context.Done()
		// do not call OnStop directly, instead, call Stop on the wrapper BaseService
		e.bs.Stop()
	}()

	return nil
}

func (e *EthereumService) startPSSMonitor() {
	// Start PSS monitors
	go incomingPSSMonitor(e.eventBus)
	go outgoingPSSMonitor(e.eventBus)

	// Start prev nodes monitor
	go previousNodesMonitor(e)

	// Start next nodes monitor
	go nextNodesMonitor(e)
}

func (e *EthereumService) OnStop() error {
	return nil
}

func (e *EthereumService) Call(method string, args ...interface{}) (interface{}, error) {
	telemetry.IncrementCounter(pcmn.TelemetryConstants.Generic.TotalServiceCalls, pcmn.TelemetryConstants.Ethereum.Prefix)

	switch method {
	// GetCurrentEpoch() (epoch int)
	case "get_current_epoch":
		telemetry.IncrementCounter(pcmn.TelemetryConstants.Ethereum.GetCurrentEpochCounter, pcmn.TelemetryConstants.Ethereum.Prefix)

		e.Lock()
		defer e.Unlock()
		return e.currentEpoch, nil
	// GetPreviousEpoch() (epoch int)
	case "get_previous_epoch":
		telemetry.IncrementCounter(pcmn.TelemetryConstants.Ethereum.GetPreviousEpochCounter, pcmn.TelemetryConstants.Ethereum.Prefix)

		epochInfo, err := e.GetEpochInfo(e.currentEpoch, false)
		if err != nil {
			return nil, err
		}
		prevEpoch := int(epochInfo.PrevEpoch.Int64())
		return prevEpoch, nil
	// GetNextEpoch() (epoch int)
	case "get_next_epoch":
		telemetry.IncrementCounter(pcmn.TelemetryConstants.Ethereum.GetNextEpochCounter, pcmn.TelemetryConstants.Ethereum.Prefix)

		epochInfo, err := e.GetEpochInfo(e.currentEpoch, false)
		if err != nil {
			return nil, err
		}
		nextEpoch := int(epochInfo.NextEpoch.Int64())
		return nextEpoch, nil
	// GetEpochInfo(epoch int) (epochInfo epochInfo, err error)
	case "get_epoch_info":
		telemetry.IncrementCounter(pcmn.TelemetryConstants.Ethereum.GetEpochInfoCounter, pcmn.TelemetryConstants.Ethereum.Prefix)

		var args0 int
		var args1 bool
		_ = castOrUnmarshal(args[0], &args0)
		_ = castOrUnmarshal(args[1], &args1)

		eInfo, err := e.GetEpochInfo(args0, args1)
		return eInfo, err
	// GetSelfIndex() (index int)
	case "get_self_index":
		telemetry.IncrementCounter(pcmn.TelemetryConstants.Ethereum.GetSelfIndexCounter, pcmn.TelemetryConstants.Ethereum.Prefix)

		e.Lock()
		defer e.Unlock()
		for {
			if e.nodeIndex != 0 {
				return e.nodeIndex, nil
			}
			e.Unlock()
			time.Sleep(1 * time.Second)
			e.Lock()
		}
	// GetSelfPrivateKey() (privKey ecdsa.PrivateKey)
	case "get_self_private_key":
		telemetry.IncrementCounter(pcmn.TelemetryConstants.Ethereum.GetSelfPrivateKeyCounter, pcmn.TelemetryConstants.Ethereum.Prefix)

		e.Lock()
		defer e.Unlock()
		if e.nodePrivK == nil {
			return nil, errors.New("Ethereum private key has not been initialized")
		}
		return *e.nodePrivK.D, nil
	// GetSelfPublicKey() (pubKey ecdsa.PublicKey)
	case "get_self_public_key":
		telemetry.IncrementCounter(pcmn.TelemetryConstants.Ethereum.GetSelfPublicKeyCounter, pcmn.TelemetryConstants.Ethereum.Prefix)

		e.Lock()
		defer e.Unlock()
		if e.nodePubK == nil {
			return nil, errors.New("Ethereum public key has not been initialized")
		}
		return common.Point{
			X: *e.nodePubK.X,
			Y: *e.nodePubK.Y,
		}, nil
	// GetSelfAddress() (address ethCommon.Address)
	case "get_self_address":
		telemetry.IncrementCounter(pcmn.TelemetryConstants.Ethereum.GetSelfAddressCounter, pcmn.TelemetryConstants.Ethereum.Prefix)

		e.Lock()
		defer e.Unlock()
		if e.nodeAddr == nil {
			return nil, errors.New("Ethereum node address has not been initialized")
		}
		return *e.nodeAddr, nil
	// SetSelfIndex(int)
	case "set_self_index":
		telemetry.IncrementCounter(pcmn.TelemetryConstants.Ethereum.GetSelfIndexCounter, pcmn.TelemetryConstants.Ethereum.Prefix)

		e.Lock()
		defer e.Unlock()
		var args0 int
		fmt.Println("setselfindex args", args)
		_ = castOrUnmarshal(args[0], &args0)

		e.nodeIndex = args0
		return nil, nil
	// SelfSignData(data []byte) (rawSig []byte)
	case "self_sign_data":
		telemetry.IncrementCounter(pcmn.TelemetryConstants.Ethereum.SelfSignDataCounter, pcmn.TelemetryConstants.Ethereum.Prefix)

		var args0 []byte
		_ = castOrUnmarshal(args[0], &args0)

		// doesn't lock as this should NEVER change
		rawSig := e.selfSignData(args0)
		return rawSig, nil
	case "await_complete_node_list":
		var args0 int
		_ = castOrUnmarshal(args[0], &args0)

		nodeEpoch := args0
		if e.nodeList == nil {
			return nil, errors.New("nodelist contract is undefined")
		}
		first := true
		for {
			if !first {
				time.Sleep(10 * time.Second)
			}
			first = false
			logging.Debug("attempting to retrieve complete node list")
			if e.nodeRegisterMap[nodeEpoch] == nil {
				logging.WithField("nodeRegisterMap", e.nodeRegisterMap).Error("could not get node list")
				continue
			}
			currEpochInfo, err := e.GetEpochInfo(nodeEpoch, true)
			if err != nil {
				logging.WithError(err).Error("could not get current epoch")
				continue
			}
			nodeList := e.nodeRegisterMap[nodeEpoch].NodeList
			if currEpochInfo.N.Cmp(big.NewInt(int64(len(nodeList)))) != 0 {
				logging.WithFields(logging.Fields{
					"nodeList":      nodeList,
					"expectedNodes": currEpochInfo.N,
				}).Error("NodeList and expected nodes not equal")
			} else {
				break
			}
		}
		nodeReferences := make([]SerializedNodeReference, 0)
		for _, nodeDetails := range e.nodeRegisterMap[nodeEpoch].NodeList {
			nodeReferences = append(nodeReferences, nodeDetails.Serialize())
		}
		return nodeReferences, nil
	// GetNodeList(epoch int) []NodeReference
	case "get_node_list":
		telemetry.IncrementCounter(pcmn.TelemetryConstants.Ethereum.GetNodeListCounter, pcmn.TelemetryConstants.Ethereum.Prefix)

		var args0 int
		_ = castOrUnmarshal(args[0], &args0)

		e.Lock()
		defer e.Unlock()
		nodeEpoch := args0
		if e.nodeRegisterMap[nodeEpoch] == nil {
			return nil, fmt.Errorf("Could not get node list %v", nodeEpoch)
		}
		nodeReferences := make([]SerializedNodeReference, 0)
		for _, nodeDetails := range e.nodeRegisterMap[nodeEpoch].NodeList {
			nodeReferences = append(nodeReferences, nodeDetails.Serialize())
		}
		return nodeReferences, nil
	// GetNodeDetailsByAddress(address ethCommon.Address) NodeReference
	case "get_node_details_by_address":
		telemetry.IncrementCounter(pcmn.TelemetryConstants.Ethereum.GetNodeDetailsByAddressCounter, pcmn.TelemetryConstants.Ethereum.Prefix)

		var args0 ethCommon.Address
		_ = castOrUnmarshal(args[0], &args0)

		e.Lock()
		defer e.Unlock()
		for _, nodeRegister := range e.nodeRegisterMap {
			for _, nodeDetails := range nodeRegister.NodeList {
				if nodeDetails.Address.String() == args0.String() {
					return nodeDetails.Serialize(), nil
				}
			}
		}
		return nil, fmt.Errorf("node could not be found for address %s", args0.String())
	// GetNodeDetailsByEpochAndIndex(epoch int, index int) NodeReference
	case "get_node_details_by_epoch_and_index":
		telemetry.IncrementCounter(pcmn.TelemetryConstants.Ethereum.GetNodeDetailsByEpochAndIndexCounter, pcmn.TelemetryConstants.Ethereum.Prefix)

		var args0, args1 int
		_ = castOrUnmarshal(args[0], &args0)
		_ = castOrUnmarshal(args[1], &args1)

		e.Lock()
		defer e.Unlock()
		for _, nodeDetails := range e.nodeRegisterMap[args0].NodeList {
			if int(nodeDetails.Index.Int64()) == args1 {
				return nodeDetails.Serialize(), nil
			}
		}
		return nil, fmt.Errorf("node could not be found for %v %v", args0, args1)
	case "await_nodes_connected":
		telemetry.IncrementCounter(pcmn.TelemetryConstants.Ethereum.AwaitNodesConnectedCounter, pcmn.TelemetryConstants.Ethereum.Prefix)

		var args0 int
		_ = castOrUnmarshal(args[0], &args0)

		interval := time.NewTicker(1 * time.Second)
		if e.nodeRegisterMap[args0] != nil && len(e.nodeRegisterMap[args0].NodeList) > 0 {
			return nil, nil
		}
		for range interval.C {
			if e.nodeRegisterMap[args0] != nil && len(e.nodeRegisterMap[args0].NodeList) > 0 {
				return nil, nil
			}
			logging.WithField("epoch", args0).Debug("waiting for nodes to be connected")
		}
	case "get_PSS_status":
		telemetry.IncrementCounter(pcmn.TelemetryConstants.Ethereum.GetPSSStatusCounter, pcmn.TelemetryConstants.Ethereum.Prefix)

		var args0, args1 int
		_ = castOrUnmarshal(args[0], &args0)
		_ = castOrUnmarshal(args[1], &args1)

		oldEpoch := args0
		newEpoch := args1
		pssStatus, err := e.nodeList.GetPssStatus(nil, big.NewInt(int64(oldEpoch)), big.NewInt(int64(newEpoch)))
		if err != nil {
			return nil, err
		}
		return int(pssStatus.Int64()), nil
	case "verify_data_with_nodelist":
		telemetry.IncrementCounter(pcmn.TelemetryConstants.Ethereum.VerifyDataWithNodeListCounter, pcmn.TelemetryConstants.Ethereum.Prefix)

		var args0 common.Point
		var args1, args2 []byte
		_ = castOrUnmarshal(args[0], &args0)
		_ = castOrUnmarshal(args[1], &args1)
		_ = castOrUnmarshal(args[2], &args2)

		return e.verifyDataWithNodelist(args0, args1, args2)

	case "verify_data_with_epoch":
		telemetry.IncrementCounter(pcmn.TelemetryConstants.Ethereum.VerifyDataWithEpochCounter, pcmn.TelemetryConstants.Ethereum.Prefix)

		var args0 common.Point
		var args1, args2 []byte
		var args3 int
		_ = castOrUnmarshal(args[0], &args0)
		_ = castOrUnmarshal(args[1], &args1)
		_ = castOrUnmarshal(args[2], &args2)
		_ = castOrUnmarshal(args[3], &args3)

		return e.verifyDataWithEpoch(args0, args1, args2, args3)

	case "start_PSS_monitor":
		telemetry.IncrementCounter(pcmn.TelemetryConstants.Ethereum.StartPSSMonitorCounter, pcmn.TelemetryConstants.Ethereum.Prefix)

		go e.startPSSMonitor()
		return nil, nil

	case "get_tm_p2p_connection":
		telemetry.IncrementCounter(pcmn.TelemetryConstants.Ethereum.GetTMP2PConnection, pcmn.TelemetryConstants.Ethereum.Prefix)
		return e.tmp2pConnection, nil

	case "get_p2p_connection":
		telemetry.IncrementCounter(pcmn.TelemetryConstants.Ethereum.GetP2PConnection, pcmn.TelemetryConstants.Ethereum.Prefix)
		return e.p2pConnection, nil

	case "validate_epoch_pub_key":
		var args0 ethCommon.Address
		var args1 common.Point
		_ = castOrUnmarshal(args[0], &args0)
		_ = castOrUnmarshal(args[1], &args1)
		pubKey, err := e.serviceLibrary.DatabaseMethods().RetrieveNodePubKey(args0)
		if err != nil {
			return false, err
		}
		if pubKey.X.Cmp(&args1.X) == 0 && pubKey.Y.Cmp(&args1.Y) == 0 {
			return true, nil
		}
		return false, errors.New("incorrect pubkey")
	}
	return nil, fmt.Errorf("ethereum service method %v not found", method)
}

func (e *EthereumService) SetBaseService(bs *BaseService) {
	e.bs = bs
}

func (e *EthereumService) newStandardEthCallOpts() (*bind.CallOpts, error) {
	auth := bind.CallOpts{
		From: *e.nodeAddr,
	}
	return &auth, nil
}

func (e *EthereumService) IsSelfRegistered(epoch int) (bool, error) {
	opts, err := e.newStandardEthCallOpts()
	if err != nil {
		return false, err
	}
	result, err := e.nodeList.NodeRegistered(opts, big.NewInt(int64(epoch)), *e.nodeAddr)
	if err != nil {
		return false, err
	}
	return result, nil
}

func (e *EthereumService) GetEpochInfo(epoch int, skipCache bool) (epochInfo, error) {
	if !skipCache {
		eInfo, found := e.cachedEpochInfo.Get(epoch)
		if found {
			return eInfo, nil
		}
	}
	opts, err := e.newStandardEthCallOpts()
	if err != nil {
		return epochInfo{}, err
	}
	if epoch == 0 {
		return epochInfo{}, fmt.Errorf("Epoch %v is invalid", epoch)
	}
	result, err := e.nodeList.GetEpochInfo(opts, big.NewInt(int64(epoch)))
	if err != nil {
		return epochInfo{}, err
	}
	if result.Id.Cmp(big.NewInt(0)) == 0 {
		return epochInfo{}, fmt.Errorf("Epoch %v has not been initialized", epoch)
	}
	eInfo := epochInfo{
		Id:        *result.Id,
		N:         *result.N,
		K:         *result.K,
		T:         *result.T,
		PrevEpoch: *result.PrevEpoch,
		NextEpoch: *result.NextEpoch,
	}
	e.cachedEpochInfo.Set(epoch, eInfo)
	return eInfo, nil
}

// RegisterNode - the main function that registered the node on Ethereum node list with the details provided
func (e *EthereumService) RegisterNode(epoch int, declaredIP string, TMP2PConnection string, P2PConnection string) (*types.Transaction, error) {
	nonce, err := e.ethClient.PendingNonceAt(context.Background(), ethCrypto.PubkeyToAddress(*e.nodePubK))
	if err != nil {
		return nil, err
	}

	gasPrice, err := e.ethClient.SuggestGasPrice(context.Background())
	gasPrice.Add(gasPrice, gasPrice) // doubles gas price
	gasPrice.Add(gasPrice, gasPrice) // quadruples gas price
	if err != nil {
		return nil, err
	}

	auth := bind.NewKeyedTransactor(e.nodePrivK)
	auth.Nonce = big.NewInt(int64(nonce))
	auth.Value = big.NewInt(0)      // in wei
	auth.GasLimit = uint64(4700000) // in units
	auth.GasPrice = gasPrice

	tx, err := e.nodeList.ListNode(auth, big.NewInt(int64(epoch)), declaredIP, e.nodePubK.X, e.nodePubK.Y, "", "")
	if err != nil {
		return nil, err
	}
	return tx, nil
}

func whitelistMonitor(e *EthereumService) {
	interval := time.NewTicker(10 * time.Second)
	for range interval.C {
		isWhitelisted, err := e.nodeList.IsWhitelisted(nil, big.NewInt(int64(e.currentEpoch)), *e.nodeAddr)
		if err != nil {
			logging.WithError(err).Error("could not check ethereum whitelist")
		}
		if isWhitelisted {
			e.isWhitelisted = true
			break
		}
		logging.Warn("node is not whitelisted")
	}
}

func registerNode(e *EthereumService) {
	// check if node has been whitelisted
	if !config.GlobalConfig.ShouldRegister {
		return
	}
	for {
		if e.isWhitelisted {
			break
		}
		logging.WithField("nodeIndex", e.nodeIndex).Debug("node is not whitelisted yet")
		time.Sleep(10 * time.Second)
	}
	var registered bool
	err := retry.Do(func() error {
		res, err := e.IsSelfRegistered(e.currentEpoch)
		if err != nil {
			return fmt.Errorf("Could not check if node was registered on node list, %v", err.Error())
		}
		registered = res
		return nil
	})
	if err != nil {
		logging.WithError(err).Fatal()
	}
	externalAddr := "tcp://" + config.GlobalConfig.ProvidedIPAddress + ":" + strings.Split(config.GlobalConfig.TMP2PListenAddress, ":")[2]
	tmp2pNodeKey := e.serviceLibrary.TendermintMethods().GetNodeKey()
	p2pHostAddress := e.serviceLibrary.P2PMethods().GetHostAddress()
	splitP2PHostAddr := strings.Split(p2pHostAddress, "/")
	splitP2PHostAddr[2] = config.GlobalConfig.ProvidedIPAddress
	hostP2PAddressWithIP := strings.Join(splitP2PHostAddr, "/")

	e.tmp2pConnection = tmp2p.IDAddressString(tmp2pNodeKey.ID(), externalAddr)
	e.p2pConnection = hostP2PAddressWithIP

	if !registered {
		// Register node
		var registeredDeclaredConEndpoint string
		if config.GlobalConfig.PublicURL != "" { // if we are using TLS, submit the domain instead of IP on chain
			registeredDeclaredConEndpoint = config.GlobalConfig.PublicURL + ":" + config.GlobalConfig.HttpServerPort
		} else {
			registeredDeclaredConEndpoint = config.GlobalConfig.ProvidedIPAddress + ":" + config.GlobalConfig.HttpServerPort
		}
		logging.WithFields(logging.Fields{
			"host":                 config.GlobalConfig.ProvidedIPAddress + ":" + config.GlobalConfig.HttpServerPort,
			"IDAddressString":      tmp2p.IDAddressString(tmp2pNodeKey.ID(), externalAddr),
			"hostP2PAddressWithIP": hostP2PAddressWithIP,
		}).Info("registering node")

		_, err := e.RegisterNode(
			e.currentEpoch,
			registeredDeclaredConEndpoint,
			e.tmp2pConnection,
			e.p2pConnection,
		)
		if err != nil {
			logging.WithError(err).Fatal()
		}
	}
	e.isRegistered = true
}

func outgoingPSSMonitor(e eventbus.Bus) {
	serviceLibrary := NewServiceLibrary(e, "outgoing_PSS_monitor")
	logging.Info("started outgoingPSSMonitor")
	currEpoch := serviceLibrary.EthereumMethods().GetCurrentEpoch()
	interval := time.NewTicker(10 * time.Second)
	var currEpochInfo epochInfo
	var nextEpoch int
	var err error
	for range interval.C {
		currEpochInfo, err = serviceLibrary.EthereumMethods().GetEpochInfo(currEpoch, true)
		if err != nil || currEpochInfo.NextEpoch.Int64() == 0 {
			logging.WithField("err", err).Debug("could not get previous epoch")
			continue
		}
		nextEpoch = int(currEpochInfo.PrevEpoch.Int64())
		break
	}
	for range interval.C {
		nextEpoch, err = serviceLibrary.EthereumMethods().GetNextEpoch()
		if err != nil || nextEpoch == 0 {
			logging.WithField("err", err).Debug("could not get currentEpoch")
			continue
		}
		break
	}
	serviceLibrary.EthereumMethods().AwaitNodesConnected(nextEpoch)
	var nextEpochInfo epochInfo
	for range interval.C {
		var err error
		currEpochInfo, err = serviceLibrary.EthereumMethods().GetEpochInfo(currEpoch, true)
		if err != nil {
			logging.WithError(err).Error("could not get currEpochInfo")
			continue
		}
		nextEpochInfo, err = serviceLibrary.EthereumMethods().GetEpochInfo(nextEpoch, true)
		if err != nil {
			logging.WithError(err).Error("could not get nextEpochInfo")
			continue
		}
		pssStatus, err := serviceLibrary.EthereumMethods().GetPSSStatus(currEpoch, nextEpoch)
		if err != nil {
			logging.WithError(err).Error("could not get pssStatus")
			continue
		}
		if pssStatus != 1 {
			logging.Error("pssStatus is not 1 yet, waiting...")
			continue
		}
		break
	}
	err = serviceLibrary.PSSMethods().NewPSSNode(PSSStartData{
		Message:   "start",
		OldEpoch:  int(currEpochInfo.Id.Int64()),
		OldEpochN: int(currEpochInfo.N.Int64()),
		OldEpochK: int(currEpochInfo.K.Int64()),
		OldEpochT: int(currEpochInfo.T.Int64()),
		NewEpoch:  int(nextEpochInfo.Id.Int64()),
		NewEpochN: int(nextEpochInfo.N.Int64()),
		NewEpochK: int(nextEpochInfo.K.Int64()),
		NewEpochT: int(nextEpochInfo.T.Int64()),
	}, true, false)
	if err != nil {
		logging.WithError(err).Error("could not start new PSSNode")
	}
	err = serviceLibrary.MappingMethods().NewMappingNode(MappingStartData{
		OldEpoch:  int(currEpochInfo.Id.Int64()),
		OldEpochN: int(currEpochInfo.N.Int64()),
		OldEpochK: int(currEpochInfo.K.Int64()),
		OldEpochT: int(currEpochInfo.T.Int64()),
		NewEpoch:  int(nextEpochInfo.Id.Int64()),
		NewEpochN: int(nextEpochInfo.N.Int64()),
		NewEpochK: int(nextEpochInfo.K.Int64()),
		NewEpochT: int(nextEpochInfo.T.Int64()),
	}, true, false)
	if err != nil {
		logging.WithError(err).Error("could not start new mappingNode")
	}
	mappingID := serviceLibrary.MappingMethods().GetMappingID(currEpoch, nextEpoch)

	var currFreezeState int
	var endIndex uint

	if currFreezeState, _ = serviceLibrary.MappingMethods().GetFreezeState(mappingID); currFreezeState == 0 {
		err := serviceLibrary.MappingMethods().ProposeFreeze(mappingID)
		if err != nil {
			logging.WithError(err).Error("could not send propose freeze broadcast")
		}
		serviceLibrary.MappingMethods().SetFreezeState(mappingID, 1, 0)
	}
	for range interval.C {
		if currFreezeState, endIndex = serviceLibrary.MappingMethods().GetFreezeState(mappingID); currFreezeState == 2 {
			break
		}
		logging.Debug("waiting for FreezeState to be 2")
	}

	go triggerMapping(e, endIndex, mappingID, currEpochInfo, nextEpochInfo)
	go triggerPSS(e, endIndex, currEpochInfo, nextEpochInfo)
	serviceLibrary.MappingMethods().SetFreezeState(mappingID, 3, endIndex)
}

func triggerMapping(e eventbus.Bus, endIndex uint, mappingID mapping.MappingID, currEpochInfo epochInfo, nextEpochInfo epochInfo) {
	serviceLibrary := NewServiceLibrary(e, "trigger_mapping")
	mappingSummaryMessage := mapping.MappingSummaryMessage{
		TransferSummary: mapping.TransferSummary{
			LastUnassignedIndex: endIndex,
		},
	}
	byt, err := bijson.Marshal(mappingSummaryMessage)
	if err != nil {
		logging.WithError(err).Error("could not marshal mapping summary message")
	}
	err = serviceLibrary.MappingMethods().ReceiveBFTMessage(mappingID, mapping.CreateMappingMessage(mapping.MappingMessageRaw{
		MappingID: mappingID,
		Method:    "mapping_summary_frozen",
		Data:      byt,
	}))
	if err != nil {
		logging.WithError(err).Error("mapping methods could not receive bft message")
	}
}

func triggerPSS(e eventbus.Bus, endIndex uint, currEpochInfo epochInfo, nextEpochInfo epochInfo) {
	serviceLibrary := NewServiceLibrary(e, "trigger_PSS")
	currEpoch := int(currEpochInfo.Id.Int64())
	nextEpoch := int(nextEpochInfo.Id.Int64())
	pssProtocolPrefix := serviceLibrary.PSSMethods().GetPSSProtocolPrefix(currEpoch, nextEpoch)
	for i := 0; i < int(endIndex); i++ {
		time.Sleep(time.Duration(config.GlobalMutableConfig.GetI("PSSShareDelayMS")) * time.Millisecond)
		keygenID := pss.GenerateKeygenID(i)
		serviceLibrary := NewServiceLibrary(e, "send_PSS_message")
		logging.WithFields(logging.Fields{
			"keygenID":      keygenID,
			"currEpochInfo": currEpochInfo,
			"nextEpochInfo": nextEpochInfo,
		}).Debug()
		sharingID := keygenID.GetSharingID(
			int(currEpochInfo.Id.Int64()),
			int(currEpochInfo.N.Int64()),
			int(currEpochInfo.K.Int64()),
			int(currEpochInfo.T.Int64()),
			int(nextEpochInfo.Id.Int64()),
			int(nextEpochInfo.N.Int64()),
			int(nextEpochInfo.K.Int64()),
			int(nextEpochInfo.T.Int64()),
		)
		pssMsgShare := pss.PSSMsgShare{
			SharingID: sharingID,
		}
		data, err := bijson.Marshal(pssMsgShare)
		if err != nil {
			logging.WithError(err).Error("could not marshal pssMsgShare")
		}
		pssID := (&pss.PSSIDDetails{
			SharingID:   sharingID,
			DealerIndex: serviceLibrary.EthereumMethods().GetSelfIndex(),
		}).ToPSSID()
		logging.WithField("pssID", pssID).Debug()
		selfPublicKey := serviceLibrary.EthereumMethods().GetSelfPublicKey()
		err = serviceLibrary.PSSMethods().SendPSSMessageToNode(
			pssProtocolPrefix,
			pss.NodeDetails(pcmn.Node{
				Index:  serviceLibrary.EthereumMethods().GetSelfIndex(),
				PubKey: selfPublicKey,
			}),
			pss.CreatePSSMessage(pss.PSSMessageRaw{
				PSSID:  pssID,
				Method: "share",
				Data:   data,
			}),
		)
		if err != nil {
			logging.WithError(err).Error("could not send PSSMessage to node")
		}
	}
}

func incomingPSSMonitor(e eventbus.Bus) {
	serviceLibrary := NewServiceLibrary(e, "incoming_PSS_monitor")
	logging.Info("started IncomingPSSMonitor")
	currEpoch := serviceLibrary.EthereumMethods().GetCurrentEpoch()
	interval := time.NewTicker(10 * time.Second)
	var currEpochInfo epochInfo
	var prevEpoch int
	var err error
	for range interval.C {
		currEpochInfo, err = serviceLibrary.EthereumMethods().GetEpochInfo(currEpoch, true)
		if err != nil || currEpochInfo.PrevEpoch.Int64() == 0 {
			logging.WithField("err", err).Debug("could not get previous epoch")
			continue
		}
		prevEpoch = int(currEpochInfo.PrevEpoch.Int64())
		break
	}
	serviceLibrary.EthereumMethods().AwaitNodesConnected(prevEpoch)
	for range interval.C {
		prevEpochInfo, err := serviceLibrary.EthereumMethods().GetEpochInfo(prevEpoch, true)
		if err != nil {
			logging.WithField("err", err).Debug("could not get prevEpochInfo")
			continue
		}
		currEpochInfo, err := serviceLibrary.EthereumMethods().GetEpochInfo(currEpoch, true)
		if err != nil {
			logging.WithField("err", err).Debug("could not get currEpochInfo")
			continue
		}
		err = serviceLibrary.PSSMethods().NewPSSNode(PSSStartData{
			Message:   "start",
			OldEpoch:  int(prevEpochInfo.Id.Int64()),
			OldEpochN: int(prevEpochInfo.N.Int64()),
			OldEpochK: int(prevEpochInfo.K.Int64()),
			OldEpochT: int(prevEpochInfo.T.Int64()),
			NewEpoch:  int(currEpochInfo.Id.Int64()),
			NewEpochN: int(currEpochInfo.N.Int64()),
			NewEpochK: int(currEpochInfo.K.Int64()),
			NewEpochT: int(currEpochInfo.T.Int64()),
		}, false, true)
		if err != nil {
			logging.WithError(err).Error("could not start new pss node")
		}
		err = serviceLibrary.MappingMethods().NewMappingNode(MappingStartData{
			OldEpoch:  int(prevEpochInfo.Id.Int64()),
			OldEpochN: int(prevEpochInfo.N.Int64()),
			OldEpochK: int(prevEpochInfo.K.Int64()),
			OldEpochT: int(prevEpochInfo.T.Int64()),
			NewEpoch:  int(currEpochInfo.Id.Int64()),
			NewEpochN: int(currEpochInfo.N.Int64()),
			NewEpochK: int(currEpochInfo.K.Int64()),
			NewEpochT: int(currEpochInfo.T.Int64()),
		}, false, true)
		if err != nil {
			logging.WithError(err).Error("could not start new mapping node")
		}
		break
	}
}

// getNodeRefsByEpoch - Retreives NodeList from ethereum and gets their details
// appends and returns an array of node references
func (e *EthereumService) getNodeRefsByEpoch(epoch int) ([]*NodeReference, error) {
	logging.WithField("epoch", epoch).Debug("getNodeRefsByEpoch called")
	ethList, err := e.nodeList.GetNodes(nil, big.NewInt(int64(epoch)))
	if err != nil {
		return nil, fmt.Errorf("Could not get node list %v", err.Error())
	}
	var currNodeList []*NodeReference

	for i := 0; i < len(ethList); i++ {
		detailsWithPubK, err := e.nodeList.NodeDetails(nil, ethList[i])
		if err != nil {
			return nil, fmt.Errorf("could not get node details with pub key %v", err.Error())
		}
		err = e.serviceLibrary.DatabaseMethods().StoreNodePubKey(ethList[i], common.Point{X: *detailsWithPubK.PubKx, Y: *detailsWithPubK.PubKy})
		if err != nil {
			return nil, fmt.Errorf("could not store node details with pub key %v", err.Error())
		}
	}

	for i := 0; i < len(ethList); i++ {
		nodeRef, err := e.GetNodeRef(ethList[i])
		if err != nil {
			return nil, fmt.Errorf("Could not get node refs %v", err.Error())
		}
		currNodeList = append(currNodeList, nodeRef)
	}
	return currNodeList, nil
}

func previousNodesMonitor(e *EthereumService) {
	interval := time.NewTicker(10 * time.Second)
	for range interval.C {
		previousEpoch, err := e.serviceLibrary.EthereumMethods().GetPreviousEpoch()
		if err != nil || previousEpoch == 0 {
			logging.WithField("err", err).Debug("could not get previous epoch in previous nodes monitor")
			continue
		}
		e.Lock()
		if _, ok := e.nodeRegisterMap[previousEpoch]; !ok {
			e.nodeRegisterMap[previousEpoch] = &NodeRegister{}
		}
		e.Unlock()
		logging.WithField("previousEpoch", previousEpoch).Debug("previousNodesMonitor calling nodeRefs")
		prevNodeList, err := e.getNodeRefsByEpoch(previousEpoch)
		if err != nil {
			logging.WithError(err).Error("could not get previous node list")
			continue
		}
		for _, nodeRef := range prevNodeList {
			err := e.serviceLibrary.P2PMethods().ConnectToP2PNode(nodeRef.P2PConnection, nodeRef.PeerID)
			if err != nil {
				logging.WithField("Address", *nodeRef.Address).Error("could not connect to p2p node ...continuing...")
			}
		}
		e.Lock()
		e.nodeRegisterMap[previousEpoch].NodeList = prevNodeList
		e.Unlock()
		break
	}
}

func currentNodesMonitor(e *EthereumService) {
	interval := time.NewTicker(10 * time.Second)
	for range interval.C {
		currEpoch := e.serviceLibrary.EthereumMethods().GetCurrentEpoch()
		currEpochInfo, err := e.GetEpochInfo(currEpoch, true)
		if err != nil {
			logging.WithError(err).Error("could not get curr epoch")
			continue
		}
		e.Lock()
		if _, ok := e.nodeRegisterMap[currEpoch]; !ok {
			e.nodeRegisterMap[currEpoch] = &NodeRegister{}
		}
		e.Unlock()
		logging.WithField("currEpoch", currEpoch).Debug("currentNodesMonitor calling NodeRefs")
		currNodeList, err := e.getNodeRefsByEpoch(currEpoch)
		if err != nil {
			logging.WithError(err).Error("could not get currNodeList")
			continue
		}
		if currEpochInfo.N.Cmp(big.NewInt(int64(len(currNodeList)))) != 0 {
			logging.WithFields(logging.Fields{
				"currNodeList":  currNodeList,
				"currEpochInfo": currEpochInfo,
			}).Error("currentNodeList does not equal in length to expected currEpochInfo")
			continue
		}
		allNodesConnected := true
		for _, nodeRef := range currNodeList {
			err = e.serviceLibrary.P2PMethods().ConnectToP2PNode(nodeRef.P2PConnection, nodeRef.PeerID)
			if err != nil {
				logging.WithField("Address", *nodeRef.Address).Error("could not connect to p2p node ...continuing...")
				allNodesConnected = false
			}
			// exported out of ConnectToP2PNode
			if nodeRef.PeerID == e.serviceLibrary.P2PMethods().ID() {
				e.serviceLibrary.EthereumMethods().SetSelfIndex(int(nodeRef.Index.Int64()))
			}
		}
		if !allNodesConnected {
			continue
		}
		logging.WithField("currNodeList", currNodeList).Debug("connected to all nodes in current epoch")
		e.Lock()
		e.nodeRegisterMap[currEpoch].NodeList = currNodeList
		e.Unlock()
		break
	}
}

func nextNodesMonitor(e *EthereumService) {
	interval := time.NewTicker(10 * time.Second)
	for range interval.C {
		nextEpoch, err := e.serviceLibrary.EthereumMethods().GetNextEpoch()
		if err != nil || nextEpoch == 0 {
			logging.WithField("err", err).Debug("could not get next epoch in next nodes monitor")
			continue
		}
		nextEpochInfo, err := e.GetEpochInfo(nextEpoch, true)
		if err != nil {
			logging.WithError(err).Debug()
			continue
		}
		e.Lock()
		if _, ok := e.nodeRegisterMap[nextEpoch]; !ok {
			e.nodeRegisterMap[nextEpoch] = &NodeRegister{}
		}
		e.Unlock()
		if e.nodeRegisterMap[nextEpoch].PSSSending {
			break
		}
		logging.WithField("nextEpoch", nextEpoch).Debug("nextNodesMonitor calling NodeRefs")
		nextNodeList, err := e.getNodeRefsByEpoch(nextEpoch)
		if err != nil {
			logging.WithError(err).Error("could not get nextNodeList")
			continue
		}
		if nextEpochInfo.N.Cmp(big.NewInt(int64(len(nextNodeList)))) != 0 {
			logging.WithFields(logging.Fields{
				"nextNodeList":  nextNodeList,
				"nextEpochInfo": nextEpochInfo,
			}).Error("nextNodeList does not equal in length to expected nextEpochInfo")
			continue
		}
		allNodesConnected := true
		for _, nodeRef := range nextNodeList {
			err = e.serviceLibrary.P2PMethods().ConnectToP2PNode(nodeRef.P2PConnection, nodeRef.PeerID)
			if err != nil {
				logging.WithField("Address", *nodeRef.Address).Error("could not connect to p2p node ...continuing...", *nodeRef.Address)
				allNodesConnected = false
				break
			}
		}
		if !allNodesConnected {
			continue
		}
		logging.WithField("nextNodeList", nextNodeList).Debug("connected to all nodes in next epoch")
		e.Lock()
		e.nodeRegisterMap[nextEpoch].NodeList = nextNodeList
		e.Unlock()
		break
	}
}
