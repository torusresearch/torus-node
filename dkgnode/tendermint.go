package dkgnode

/* All useful imports */
import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"

	"github.com/torusresearch/torus-node/telemetry"

	"math/big"
	"strings"
	"time"

	retry "github.com/avast/retry-go"
	logging "github.com/sirupsen/logrus"
	tmbtcec "github.com/tendermint/btcd/btcec"
	amino "github.com/tendermint/go-amino"
	"github.com/tidwall/gjson"
	tmconfig "github.com/torusresearch/tendermint/config"
	cryptoAmino "github.com/torusresearch/tendermint/crypto/encoding/amino"
	tmsecp "github.com/torusresearch/tendermint/crypto/secp256k1"
	"github.com/torusresearch/tendermint/libs/log"
	"github.com/torusresearch/tendermint/node"
	tmnode "github.com/torusresearch/tendermint/node"
	tmp2p "github.com/torusresearch/tendermint/p2p"
	"github.com/torusresearch/tendermint/privval"
	tmclient "github.com/torusresearch/tendermint/rpc/client"
	rpcclient "github.com/torusresearch/tendermint/rpc/lib/client"
	rpctypes "github.com/torusresearch/tendermint/rpc/lib/types"
	tmtypes "github.com/torusresearch/tendermint/types"
	pcmn "github.com/torusresearch/torus-node/common"
	"github.com/torusresearch/torus-node/config"
	"github.com/torusresearch/torus-node/eventbus"
	"github.com/torusresearch/torus-node/idmutex"

	"github.com/torusresearch/torus-node/tmlog"
)

func NewTendermintService(ctx context.Context, eventBus eventbus.Bus) *BaseService {
	tendermintCtx, cancel := context.WithCancel(context.WithValue(ctx, ContextID, "tendermint"))
	tendermintService := TendermintService{
		cancel:   cancel,
		ctx:      tendermintCtx,
		eventBus: eventBus,
	}
	tendermintService.serviceLibrary = NewServiceLibrary(tendermintService.eventBus, tendermintService.Name())
	return NewBaseService(&tendermintService)
}

type TendermintService struct {
	bs             *BaseService
	tmNodeKey      *tmp2p.NodeKey
	cancel         context.CancelFunc
	ctx            context.Context
	eventBus       eventbus.Bus
	serviceLibrary ServiceLibrary

	bftSemaphore         *pcmn.Semaphore
	bftRPC               *BFTRPC
	bftRPCWS             *rpcclient.WSClient
	bftNode              *node.Node
	bftRPCWSQueryHandler *BFTRPCWSQueryHandler
	bftRPCWSStatus       BFTRPCWSStatus
	// responseChannelMap   map[string]chan []byte
}

type BFTRPCWSStatus int

const (
	BftRPCWSStatusDown BFTRPCWSStatus = iota
	BftRPCWSStatusUp
)

func (t *TendermintService) Name() string {
	return "tendermint"
}

func (t *TendermintService) OnStart() error {
	// build folders for tendermint logs
	err := os.MkdirAll(config.GlobalConfig.BasePath+"/tendermint", os.ModePerm)
	if err != nil {
		logging.WithError(err).Error("could not makedir for tendermint")
	}
	err = os.MkdirAll(config.GlobalConfig.BasePath+"/tendermint/config", os.ModePerm)
	if err != nil {
		logging.WithError(err).Error("could not makedir for tendermint config")
	}
	err = os.MkdirAll(config.GlobalConfig.BasePath+"/tendermint/data", os.ModePerm)
	if err != nil {
		logging.WithError(err).Error("could not makedir for tendermint data")
	}
	err = os.Remove(config.GlobalConfig.BasePath + "/tendermint/data/cs.wal/wal")
	if err != nil {
		logging.WithError(err).Error("could not remove write ahead log")
	} else {
		logging.Debug("Removed write ahead log")
	}
	nodeKey, err := os.Open(config.GlobalConfig.BasePath + "/tendermint/config/node_key.json")
	if err == nil {
		bytVal, _ := ioutil.ReadAll(nodeKey)
		logging.WithField("NodeKey", string(bytVal)).Debug()
	} else {
		logging.Debug("Could not find NodeKey")
	}
	privValidatorKey, err := os.Open(config.GlobalConfig.BasePath + "/tendermint/config/priv_validator_key.json")
	if err == nil {
		bytVal, _ := ioutil.ReadAll(privValidatorKey)
		logging.WithField("PrivValidatorKey", stringify(bytVal)).Debug()
	} else {
		logging.Debug("Could not find PrivValidatorKey")
	}
	// Get default tm base path for generation of nodekey
	dftConfig := tmconfig.DefaultConfig()
	tmRootPath := config.GlobalConfig.BasePath + "/tendermint" // TODO: move to config
	dftConfig.SetRoot(tmRootPath)
	tmNodeKey, err := tmp2p.LoadOrGenNodeKey(dftConfig.NodeKeyFile())
	if err != nil {
		logging.WithError(err).Fatal("NodeKey generation issue")
	}

	t.tmNodeKey = tmNodeKey
	t.bftSemaphore = pcmn.NewSemaphore(config.GlobalConfig.MaxBftConnections)
	t.bftRPC = nil
	t.bftRPCWS = nil
	t.bftRPCWSStatus = BftRPCWSStatusDown
	t.bftRPCWSQueryHandler = &BFTRPCWSQueryHandler{
		QueryMap:   make(map[string]chan []byte),
		QueryCount: make(map[string]int),
	}
	go abciMonitor(t)
	go bftWorker(t)
	go t.startTendermintCore(tmRootPath, tmNodeKey, config.GlobalConfig.TendermintMetrics)
	return nil
}

func abciMonitor(t *TendermintService) {
	interval := time.NewTicker(5 * time.Second)
	for range interval.C {
		bftClient := tmclient.NewHTTP(config.GlobalConfig.BftURI, "/websocket")
		// for subscribe and unsubscribe method calls, use this
		bftClientWS := rpcclient.NewWSClient(config.GlobalConfig.BftURI, "/websocket")
		err := bftClientWS.Start()
		if err != nil {
			logging.WithError(err).Error("could not start the bftWS")
		} else {
			t.bftRPC = NewBFTRPC(bftClient)
			t.bftRPCWS = bftClientWS
			t.bftRPCWSStatus = BftRPCWSStatusUp
			break
		}
	}
}

func bftWorker(t *TendermintService) {
	logging.WithField("bftWS", "listening to ResponsesCh").Debug()
	for {
		time.Sleep(1 * time.Second)
		if t.bftRPCWSStatus == BftRPCWSStatusUp {
			break
		}
	}
	for e := range t.bftRPCWS.ResponsesCh {
		logging.WithField("bftWS", "message received").Debug(e)
		err := t.RouteWSMessage(e)
		if err != nil {
			logging.WithError(err).Error("could not routewsmessage")
		}
	}
}

func (t *TendermintService) RouteWSMessage(e rpctypes.RPCResponse) error {
	t.bftRPCWSQueryHandler.Lock()
	defer t.bftRPCWSQueryHandler.Unlock()

	queryString := gjson.GetBytes(e.Result, "query").String()
	logging.WithField("query", queryString).Debug("bftWS")
	if e.Error != nil {
		return fmt.Errorf("BFTWS: websocket subscription received error: %s", e.Error.Error())
	}
	respCh, ok := t.bftRPCWSQueryHandler.QueryMap[queryString]
	if !ok {
		return fmt.Errorf("BFTWS: websocket subscription received message but no listener, querystring: %s", queryString)
	}
	respCh <- e.Result
	// if count == 0 clean up
	if t.bftRPCWSQueryHandler.QueryCount[queryString] != 0 {
		t.bftRPCWSQueryHandler.QueryCount[queryString] = t.bftRPCWSQueryHandler.QueryCount[queryString] - 1
		if t.bftRPCWSQueryHandler.QueryCount[queryString] == 0 {
			close(t.bftRPCWSQueryHandler.QueryMap[queryString])
			delete(t.bftRPCWSQueryHandler.QueryMap, queryString)
			ctx := context.Background()
			err := t.bftRPCWS.Unsubscribe(ctx, queryString)
			if err != nil {
				return fmt.Errorf("BFTWS: websocket could not unsubscribe, queryString: %s", queryString)
			}
			delete(t.bftRPCWSQueryHandler.QueryCount, queryString)
		}
	}
	return nil
}

func (t *TendermintService) OnStop() error {
	return nil
}

func (t *TendermintService) Call(method string, args ...interface{}) (interface{}, error) {
	telemetry.IncrementCounter(pcmn.TelemetryConstants.Generic.TotalServiceCalls, pcmn.TelemetryConstants.Tendermint.Prefix)

	switch method {
	case "get_node_key":
		telemetry.IncrementCounter(pcmn.TelemetryConstants.Tendermint.GetNodeKeyCounter, pcmn.TelemetryConstants.Tendermint.Prefix)

		cdc := amino.NewCodec()
		cryptoAmino.RegisterAmino(cdc)
		return cdc.MarshalJSON(*t.tmNodeKey)
	case "get_status":
		telemetry.IncrementCounter(pcmn.TelemetryConstants.Tendermint.GetStatusCounter, pcmn.TelemetryConstants.Tendermint.Prefix)

		return t.bftRPCWSStatus, nil
		// Broadcast(tx DefaultBFTTxWrapper) (txHash common.Hash, err error)
	case "broadcast":
		telemetry.IncrementCounter(pcmn.TelemetryConstants.Tendermint.BroadcastCounter, pcmn.TelemetryConstants.Tendermint.Prefix)

		var args0 interface{}
		_ = castOrUnmarshal(args[0], &args0)

		err := retry.Do(func() error {
			if t.bftRPC == nil {
				logging.Error("bftRPC is still uninitialized")
				return fmt.Errorf("bftRPC is still uninitialized")
			}
			if t.bftRPCWSStatus == BftRPCWSStatusDown {
				logging.Error("bftrpcwsstatus is still down")
				return fmt.Errorf("bftrpcwsstatus is still down")
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
		release := t.bftSemaphore.Acquire()
		txHash, err := t.bftRPC.Broadcast(args0)
		release()
		if err != nil {
			return nil, err
		}
		return *txHash, nil
	// RegisterQuery(query string, count int) (respChannel chan []byte, err error)
	case "register_query":
		telemetry.IncrementCounter(pcmn.TelemetryConstants.Tendermint.RegisterQueryCounter, pcmn.TelemetryConstants.Tendermint.Prefix)

		var args0 string
		var args1 int
		_ = castOrUnmarshal(args[0], &args0)
		_ = castOrUnmarshal(args[1], &args1)

		query := args0
		count := args1
		release := t.bftSemaphore.Acquire()
		responseCh, cancel, err := t.RegisterQuery(query, count)
		release()
		if err != nil {
			return nil, err
		}
		go func() {
			for response := range responseCh {
				t.eventBus.Publish("tendermint:forward:"+query, MethodResponse{
					Error: nil,
					Data:  response,
				})
			}
			cancel()
		}()
		return nil, nil
	// DeregisterQuery(query string) (err error)
	case "deregister_query":
		telemetry.IncrementCounter(pcmn.TelemetryConstants.Tendermint.DeRegisterQueryCounter, pcmn.TelemetryConstants.Tendermint.Prefix)

		var args0 string
		_ = castOrUnmarshal(args[0], &args0)

		release := t.bftSemaphore.Acquire()
		err := t.DeregisterQuery(args0)
		release()
		if err != nil {
			return nil, err
		}
		return nil, nil
	default:
		return nil, fmt.Errorf("Tendermint service method %v not found", method)
	}
}

func (t *TendermintService) SetBaseService(bs *BaseService) {
	t.bs = bs
}

func (t *TendermintService) startTendermintCore(buildPath string, nodeKey *tmp2p.NodeKey, enableMetrics bool) {
	nodeList := t.serviceLibrary.EthereumMethods().AwaitCompleteNodeList(t.serviceLibrary.EthereumMethods().GetCurrentEpoch())
	// Starts tendermint node here
	logging.WithField("nodeList", nodeList).Debug("Started tendermint node with nodelist")

	// builds default config
	defaultTmConfig := tmconfig.DefaultConfig()
	defaultTmConfig.SetRoot(buildPath)

	// logging.SetLevel(logLevelMap[config.GlobalConfig.LogLevel])
	// logger := EventForwardingLogger{}
	var logger log.Logger
	if config.GlobalConfig.TendermintLogging {
		logger = tmlog.NewTMLoggerLogrus()
	} else {
		logger = tmlog.NewNoopLogger()
	}

	defaultTmConfig.ProxyApp = config.GlobalConfig.ABCIServer
	defaultTmConfig.Consensus.CreateEmptyBlocks = false

	defaultTmConfig.Instrumentation.Prometheus = enableMetrics
	logger.Debug("Tendermint: Prometheus Metric - " + strconv.FormatBool(enableMetrics))

	// converts own pv to tendermint key
	// Note: DO NOT use tendermints GenPrivKeySecp256k1, it alters the key
	pv := tmPrivateKeyFromBigInt(t.serviceLibrary.EthereumMethods().GetSelfPrivateKey())
	pvF := privval.GenFilePVFromPrivKey(pv, defaultTmConfig.PrivValidatorKeyFile(), defaultTmConfig.PrivValidatorStateFile())
	pvF.Save()
	genDoc := tmtypes.GenesisDoc{
		ChainID:     "main-chain-BLUBLU",
		GenesisTime: time.Unix(1578036594, 0),
	}

	//add validators and persistant peers
	var temp []tmtypes.GenesisValidator
	var persistantPeersList []string
	for i := range nodeList {
		//convert pubkey X and Y to tmpubkey
		pubkeyBytes := rawPointToTMPubKey(nodeList[i].PublicKey.X, nodeList[i].PublicKey.Y)
		temp = append(temp, tmtypes.GenesisValidator{
			Address: pubkeyBytes.Address(),
			PubKey:  pubkeyBytes,
			Power:   1,
		})

		persistantPeersList = append(persistantPeersList, nodeList[i].TMP2PConnection)
	}
	defaultTmConfig.P2P.PersistentPeers = strings.Join(persistantPeersList, ",")

	logging.WithField("PersistentPeers", defaultTmConfig.P2P.PersistentPeers).Info()
	genDoc.Validators = temp

	logging.WithField("in", defaultTmConfig.GenesisFile()).Info("saved GenesisFile")
	if err := genDoc.SaveAs(defaultTmConfig.GenesisFile()); err != nil {
		logging.WithError(err).Error("could not save gendoc")
	}
	// Other changes to config go here
	defaultTmConfig.BaseConfig.DBBackend = "goleveldb"
	defaultTmConfig.FastSyncMode = false
	defaultTmConfig.RPC.ListenAddress = config.GlobalConfig.BftURI
	defaultTmConfig.P2P.ListenAddress = config.GlobalConfig.TMP2PListenAddress
	defaultTmConfig.P2P.MaxNumInboundPeers = 300
	defaultTmConfig.P2P.MaxNumOutboundPeers = 300
	// recommended to run in production
	defaultTmConfig.P2P.SendRate = 20000000
	defaultTmConfig.P2P.RecvRate = 20000000
	defaultTmConfig.P2P.FlushThrottleTimeout = 10
	defaultTmConfig.P2P.MaxPacketMsgPayloadSize = 10240 // 10KB

	// testing config
	defaultTmConfig.P2P.AddrBookStrict = false
	logging.WithField("nodeKeyID", nodeKey.ID()).Info("nodeKeyID")

	tmconfig.WriteConfigFile(defaultTmConfig.RootDir+"/config/config.toml", defaultTmConfig)

	err := defaultTmConfig.ValidateBasic()
	if err != nil {
		logging.WithError(err).Fatal("config doesnt pass validation checks")
	}

	n, err := tmnode.DefaultNewNode(defaultTmConfig, logger)
	if err != nil {
		logging.WithError(err).Fatal("failed to create tendermint node")
	}

	t.bftNode = n
	//Start Tendermint Node
	logging.WithField("ListenAddress", defaultTmConfig.P2P.ListenAddress).Info("tendermint P2P Connection")
	logging.WithField("ListenAddress", defaultTmConfig.RPC.ListenAddress).Info("tendermint Node RPC listening")
	if err := n.Start(); err != nil {
		logging.WithError(err).Fatal("failed to start tendermint node")
	}
	logging.WithField("NodeInfo", n.Switch().NodeInfo()).Info("started tendermint")
}

func tmPrivateKeyFromBigInt(key big.Int) tmsecp.PrivKeySecp256k1 {
	var pv tmsecp.PrivKeySecp256k1
	keyBytes := padPrivKeyBytes(key.Bytes())
	for i := 0; i < 32; i++ {
		pv[i] = keyBytes[i]
	}
	return pv
}

func rawPointToTMPubKey(X, Y *big.Int) tmsecp.PubKeySecp256k1 {
	//convert pubkey X and Y to tmpubkey
	var pubkeyBytes tmsecp.PubKeySecp256k1
	pubkeyObject := tmbtcec.PublicKey{
		X: X,
		Y: Y,
	}
	copy(pubkeyBytes[:], pubkeyObject.SerializeCompressed())
	// pubkeyObject.SerializeUncompressed()
	return pubkeyBytes
}

type BFTRPCWSQueryHandler struct {
	idmutex.Mutex
	QueryMap   map[string]chan []byte
	QueryCount map[string]int
}

func (t *TendermintService) RegisterQuery(query string, count int) (chan []byte, context.CancelFunc, error) {
	t.bftRPCWSQueryHandler.Lock()
	defer t.bftRPCWSQueryHandler.Unlock()
	logging.WithField("query", query).Debug("bftWS registering query")
	if t.bftRPCWSQueryHandler.QueryMap[query] != nil {
		return nil, nil, fmt.Errorf("BFTWS: query has already been registered for query: %v", query)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	responseCh := make(chan []byte, 1)
	t.bftRPCWSQueryHandler.QueryMap[query] = responseCh
	t.bftRPCWSQueryHandler.QueryCount[query] = count
	err := t.bftRPCWS.Subscribe(ctx, query)
	if err != nil {
		delete(t.bftRPCWSQueryHandler.QueryMap, query)
		delete(t.bftRPCWSQueryHandler.QueryCount, query)
		cancel()
		return nil, nil, err
	}

	return responseCh, cancel, nil
}

func (t *TendermintService) DeregisterQuery(query string) error {
	t.bftRPCWSQueryHandler.Lock()
	defer t.bftRPCWSQueryHandler.Unlock()
	logging.WithField("query", query).Debug("bftWS deregistering query")
	ctx := context.Background()
	err := t.bftRPCWS.Unsubscribe(ctx, query)
	if err != nil {
		logging.WithField("query", query).Debug("bftWS websocket could not unsubscribe")
		return err
	}
	if responseCh, found := t.bftRPCWSQueryHandler.QueryMap[query]; found {
		delete(t.bftRPCWSQueryHandler.QueryMap, query)
		close(responseCh)
	}
	delete(t.bftRPCWSQueryHandler.QueryCount, query)
	return nil
}
