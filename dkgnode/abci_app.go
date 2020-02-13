package dkgnode

import (
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"sync"
	"time"

	tronCrypto "github.com/TRON-US/go-eccrypto"
	logging "github.com/sirupsen/logrus"
	"github.com/torusresearch/bijson"
	"github.com/torusresearch/tendermint/abci/example/code"
	"github.com/torusresearch/tendermint/abci/types"
	tmcmn "github.com/torusresearch/tendermint/libs/common"
	"github.com/torusresearch/tendermint/version"
	dbm "github.com/torusresearch/tm-db"
	"github.com/torusresearch/torus-common/common"
	"github.com/torusresearch/torus-common/secp256k1"
	"github.com/torusresearch/torus-node/auth"
	pcmn "github.com/torusresearch/torus-node/common"
	"github.com/torusresearch/torus-node/config"
	"github.com/torusresearch/torus-node/keygennofsm"
	"github.com/torusresearch/torus-node/mapping"
	"github.com/torusresearch/torus-node/telemetry"
)

// KeyAssignmentPublic - holds key fields safe to be shown to non-keyholders
type KeyAssignmentPublic struct {
	Index     big.Int
	PublicKey common.Point
	Threshold int
	Verifiers map[string][]string // Verifier => VerifierID
}

// KeyAssignmentOld - ensures backward compatibility with older versions of the frontend
type KeyAssignmentOld struct {
	KeyAssignmentPublic
	Share big.Int
}

// KeyAssignment - contains fields necessary to derive/validate a users key
// contains sensitive share data
type KeyAssignment struct {
	KeyAssignmentPublic
	Share    []byte
	Metadata tronCrypto.EciesMetadata
}

// State - nothing in state should be a pointer
type State struct {
	LastUnassignedIndex    uint                                                                       `json:"last_unassigned_index"`
	LastCreatedIndex       uint                                                                       `json:"last_created_index"`
	BlockTime              time.Time                                                                  `json:"-"`
	NewKeyAssignments      []KeyAssignmentPublic                                                      `json:"new_key_assignments"`
	PSSDecisions           map[string]bool                                                            `json:"pss_decisions"`
	KeygenDecisions        map[string]bool                                                            `json:"keygen_decisions"`
	KeygenPubKeys          map[string]KeygenPubKey                                                    `json:"keygen_pubkeys"`
	MappingProposeFreezes  map[mapping.MappingID]map[NodeDetailsID]bool                               `json:"mapping_propose_freezes"`
	MappingProposeSummarys map[mapping.MappingID]map[mapping.TransferSummaryID]map[NodeDetailsID]bool `json:"mapping_propose_summarys"`
	MappingProposeKeys     map[mapping.MappingID]map[mapping.MappingKeyID]map[NodeDetailsID]bool      `json:"mapping_propose_keys"`
	MappingCounters        map[mapping.MappingID]MappingCounter                                       `json:"mapping_counters"`
	MappingThawed          map[mapping.MappingID]bool                                                 `json:"mapping_thawed"`
	// counter to dump buffer when keygeneration fails across the system
	ConsecutiveFailedPubKeyAssigns uint `json:"consecutive_failed_pubkey_assigns"`
}

type AppInfo struct {
	Height  int64  `json:"height"`
	AppHash []byte `json:"app_hash"`
}

type hexstring string

func IntToHexstring(i int) hexstring {
	return hexstring(big.NewInt(int64(i)).Text(16))
}

func HexstringToInt(hs hexstring) int {
	bi, ok := new(big.Int).SetString(string(hs), 16)
	if !ok {
		return 0
	}
	return int(bi.Int64())
}

// MapFreeze -
type MapFreeze struct {
	Frozen         bool
	ProposedFreeze map[hexstring]bool
}

type mappingHash string

// MapThaw -
type MapThaw struct {
	Thawed       bool
	ProposedThaw map[mappingHash]map[hexstring]bool
}

// PSSDecision -
type PSSDecision struct {
	SharingID     string   `json:"sharingid"`
	Decided       bool     `json:"decided"`
	DecidedPSSIDs []string `json:"decidedpssids"`
}

type KeygenDecision bool
type KeygenPubKey struct {
	DKGID     string       `json:"dkgid"`
	Decided   bool         `json:"decided"`
	GSHSprime common.Point `json:"gshsprime"`
	GS        common.Point `json:"gs"`
}

// ABCITransaction -
type ABCITransaction struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

// DB Stuff for App state and mappings
var (
	// Database Keys
	stateKey                    = []byte("sk")
	keyMappingPrefixKey         = []byte("km")
	verifierToKeyIndexPrefixKey = []byte("vt")
	appInfoKey                  = []byte("ai")

	// End Iteration Keys
	endVerifierToKeyIndexKey = incrementLastBit(verifierToKeyIndexPrefixKey)
	// ProtocolVersion -
	ProtocolVersion version.Protocol = 0x1
)

// Below are functions for the non-ABCI DB which acts as a cache
// Using TorusDB
func formVerifierKey(verifier, verifierID string) []byte {
	key := []byte(strings.Join([]string{verifier, verifierID}, pcmn.Delimiter1))
	return append(verifierToKeyIndexPrefixKey, key...)
}

func deconstructVerifierKey(key []byte) (verifier string, verifierID string, err error) {
	unprefixed := key[len(verifierToKeyIndexPrefixKey):]
	var verifierDetails auth.VerifierDetails
	err = verifierDetails.FromVerifierDetailsID(auth.VerifierDetailsID(string(unprefixed)))
	if err != nil {
		return
	}
	verifier = verifierDetails.Verifier
	verifierID = verifierDetails.VerifierID
	return
}

func prefixKeyMapping(key []byte) []byte {
	return append(keyMappingPrefixKey, key...)
}

func (app *ABCIApp) retrieveKeyMapping(keyIndex big.Int) (*KeyAssignmentPublic, error) {
	b := app.db.Get(prefixKeyMapping([]byte(keyIndex.Text(16))))
	if b == nil {
		return nil, fmt.Errorf("retrieveKeyMapping, KeyMapping do not exist for index")
	}
	var res KeyAssignmentPublic
	err := bijson.Unmarshal(b, &res)
	if err != nil {
		return nil, err
	}
	return &res, nil
}

func (app *ABCIApp) retrieveVerifierToKeyIndex(verifier, verifierID string) ([]big.Int, error) {
	b := app.db.Get(formVerifierKey(verifier, verifierID))
	if b == nil {
		return nil, fmt.Errorf("retrieveVerifierToKeyIndex keyIndexes do not exist for verifier, and verifierID")
	}
	var res []big.Int
	err := bijson.Unmarshal(b, &res)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (app *ABCIApp) storeKeyMapping(keyIndex big.Int, assignment KeyAssignmentPublic) error {
	b, err := bijson.Marshal(assignment)
	if err != nil {
		return err
	}
	app.db.Set(prefixKeyMapping([]byte(keyIndex.Text(16))), b)
	return nil
}

func (app *ABCIApp) storeVerifierToKeyIndex(verifier, verifierID string, keyIndexes []big.Int) error {
	b, err := bijson.Marshal(keyIndexes)
	if err != nil {
		return err
	}
	app.db.Set(formVerifierKey(verifier, verifierID), b)
	return nil
}

func (app *ABCIApp) LoadState() (State, bool) {
	stateBytes := app.db.Get(stateKey)
	infoBytes := app.db.Get(appInfoKey)
	var state, laggingState State
	var info AppInfo
	stateExists := false
	if len(stateBytes) != 0 {
		stateExists = true
		err := bijson.Unmarshal(stateBytes, &state)
		if err != nil {
			panic(err)
		}
		err = bijson.Unmarshal(stateBytes, &laggingState)
		if err != nil {
			panic(err)
		}
		err = bijson.Unmarshal(infoBytes, &info)
		if err != nil {
			panic(err)
		}
	}
	app.state = &state
	app.laggingState = &laggingState
	app.info = &info
	return state, stateExists
}

func (app *ABCIApp) SaveState() State {
	stateBytes, err := bijson.Marshal(app.state)
	if err != nil {
		panic(err)
	}
	app.db.Set(stateKey, stateBytes)
	infoBytes, err := bijson.Marshal(app.info)
	if err != nil {
		panic(err)
	}
	app.db.Set(appInfoKey, infoBytes)
	return *app.state
}

type ABCIApp struct {
	types.BaseApplication
	info  *AppInfo
	state *State
	// lagging state used by CheckTx() concurrently to check validity of transactions
	// updated only at every Commit()
	laggingState *State
	db           dbm.DB
	dbIterators  *DBIteratorsSyncMap
}

func (a *ABCIService) NewABCIApp() *ABCIApp {
	done := createDirectory(config.GlobalConfig.BasePath + "/tmstate")
	if !done {
		logging.WithField("path", config.GlobalConfig.BasePath+"/tmstate").Debug("could not create path (potential error if no restart)")
	}
	db, err := dbm.NewGoLevelDB("tmstate", config.GlobalConfig.BasePath+"/tmstate")
	if err != nil {
		logging.WithError(err).Fatal("could not start GoLevelDB for tendermint state")
	}
	// initialize app and telemetry
	abciApp := ABCIApp{db: db, dbIterators: &DBIteratorsSyncMap{}}

	// Load or initialize state
	_, stateExists := abciApp.LoadState()
	if !stateExists {
		abciApp.state = &State{
			LastUnassignedIndex:    0,
			LastCreatedIndex:       0,
			PSSDecisions:           make(map[string]bool),
			KeygenDecisions:        make(map[string]bool),
			KeygenPubKeys:          make(map[string]KeygenPubKey),
			MappingProposeFreezes:  make(map[mapping.MappingID]map[NodeDetailsID]bool),
			MappingProposeSummarys: make(map[mapping.MappingID]map[mapping.TransferSummaryID]map[NodeDetailsID]bool),
			MappingProposeKeys:     make(map[mapping.MappingID]map[mapping.MappingKeyID]map[NodeDetailsID]bool),
			MappingThawed:          make(map[mapping.MappingID]bool),
			MappingCounters:        make(map[mapping.MappingID]MappingCounter),
		}
		abciApp.info = &AppInfo{
			Height: 0,
		}
		abciApp.laggingState = &State{
			LastUnassignedIndex:    0,
			LastCreatedIndex:       0,
			PSSDecisions:           make(map[string]bool),
			KeygenDecisions:        make(map[string]bool),
			KeygenPubKeys:          make(map[string]KeygenPubKey),
			MappingProposeFreezes:  make(map[mapping.MappingID]map[NodeDetailsID]bool),
			MappingProposeSummarys: make(map[mapping.MappingID]map[mapping.TransferSummaryID]map[NodeDetailsID]bool),
			MappingProposeKeys:     make(map[mapping.MappingID]map[mapping.MappingKeyID]map[NodeDetailsID]bool),
			MappingThawed:          make(map[mapping.MappingID]bool),
			MappingCounters:        make(map[mapping.MappingID]MappingCounter),
		}
	}
	return &abciApp
}

func (app *ABCIApp) isThawed(mappingID mapping.MappingID) bool {
	// If MappingCounters hasn't been set, do not trigger thaw
	mappingCounter := app.state.MappingCounters[mappingID]
	if mappingCounter.RequiredCount == 0 {
		return false
	}
	if mappingCounter.RequiredCount != mappingCounter.KeyCount {
		return false
	}

	// Add telemetry
	telemetry.IncrementCounter(pcmn.TelemetryConstants.Mapping.Thawed, pcmn.TelemetryConstants.Mapping.Prefix)

	// Clean up Here
	delete(app.state.MappingCounters, mappingID)
	delete(app.state.MappingProposeKeys, mappingID)
	delete(app.state.MappingCounters, mappingID)
	return true
}

func (app *ABCIApp) Info(req types.RequestInfo) (resInfo types.ResponseInfo) {
	return types.ResponseInfo{
		Version:          version.ABCIVersion,
		AppVersion:       ProtocolVersion.Uint64(),
		LastBlockAppHash: app.info.AppHash,
		LastBlockHeight:  app.info.Height,
	}
}

func (app *ABCIApp) DeliverTx(req types.RequestDeliverTx) types.ResponseDeliverTx {
	// add to metrics
	telemetry.IncrementCounter(pcmn.TelemetryConstants.ABCIApp.TotalBftTxCounter, pcmn.TelemetryConstants.ABCIApp.Prefix)

	tx := req.GetTx()
	// parse headers and authenticate message from nodelist
	parsedTx, senderDetails, err := authenticateBftTx(tx)
	if err != nil {
		telemetry.IncrementCounter(pcmn.TelemetryConstants.ABCIApp.RejectedBftTxCounter, pcmn.TelemetryConstants.ABCIApp.Prefix)
		return types.ResponseDeliverTx{Code: code.CodeTypeUnauthorized}
	}

	// Validate transaction here
	correct, tags, err := app.ValidateAndUpdateAndTagBFTTx(parsedTx.BFTTx, parsedTx.MsgType, senderDetails)
	if err != nil {
		logging.WithError(err).Error("could not validate BFTTx")
	}

	if !correct {
		// If validated, we save the transaction into the db
		logging.Debug("BFTTX IS WRONG")

		telemetry.IncrementCounter(pcmn.TelemetryConstants.ABCIApp.RejectedBftTxCounter, pcmn.TelemetryConstants.ABCIApp.Prefix)
		return types.ResponseDeliverTx{Code: code.CodeTypeUnauthorized}
	}

	if tags == nil {
		tags = new([]tmcmn.KVPair)
	}

	return types.ResponseDeliverTx{Code: code.CodeTypeOK, Events: []types.Event{{Type: "transfer", Attributes: *tags}}}
}

func (app *ABCIApp) CheckTx(req types.RequestCheckTx) types.ResponseCheckTx {

	tx := req.GetTx()
	// parse headers and authenticate message from nodelist
	parsedTx, senderDetails, err := authenticateBftTx(tx)
	if err != nil {
		telemetry.IncrementCounter(pcmn.TelemetryConstants.ABCIApp.RejectedBftTxCounter, pcmn.TelemetryConstants.ABCIApp.CheckTxPrefix)
		return types.ResponseCheckTx{Code: code.CodeTypeUnauthorized}
	}

	correct, err := app.validateTx(parsedTx.BFTTx, parsedTx.MsgType, senderDetails, app.laggingState)
	if err != nil {
		logging.WithError(err).Error("could not validate BFTTx in checkTx")
	}

	if !correct {
		// If validated, we save the transaction into the db
		logging.Debug("BFTTX IS WRONG checkTx")
		telemetry.IncrementCounter(pcmn.TelemetryConstants.ABCIApp.RejectedBftTxCounter, pcmn.TelemetryConstants.ABCIApp.CheckTxPrefix)
		return types.ResponseCheckTx{Code: code.CodeTypeUnauthorized}
	}

	return types.ResponseCheckTx{Code: code.CodeTypeOK}
}

func (app *ABCIApp) Commit() types.ResponseCommit {
	// get the hash of the current state (including the previous app hash)
	byt, err := bijson.Marshal(app.state)
	if err != nil {
		logging.WithError(err).Fatal("could not marshal app state")
	}
	currAppHash := secp256k1.Keccak256(byt)

	// update prepare state for next block,
	app.info.AppHash = currAppHash
	app.info.Height += 1
	app.SaveState()
	app.laggingState = nil
	err = bijson.Unmarshal(byt, &app.laggingState)
	if err != nil {
		logging.WithError(err).Fatal("could not copy lagging state")
	}

	// submit consensus data with current app hash that is derived from current state (including the previous app hash)
	return types.ResponseCommit{Data: currAppHash}
}

// Track the block hash and header information
func (app *ABCIApp) BeginBlock(req types.RequestBeginBlock) types.ResponseBeginBlock {
	// store time for later use
	app.state.BlockTime = req.Header.GetTime()
	// remove new key assignments
	app.state.NewKeyAssignments = []KeyAssignmentPublic{}
	return types.ResponseBeginBlock{}
}

// EndBlock
func (app *ABCIApp) EndBlock(req types.RequestEndBlock) types.ResponseEndBlock {
	logging.WithFields(logging.Fields{
		"EndBlockHeight": req.Height,
		"State":          stringify(app.state),
	}).Debug("state")

	// Here we trigger keygens up to the buffer amount
	if int(app.state.LastCreatedIndex)-int(app.state.LastUnassignedIndex) < config.GlobalConfig.KeyBuffer {
		for i := int(app.state.LastCreatedIndex); i < int(app.state.LastUnassignedIndex)+config.GlobalConfig.KeyBuffer; i++ {
			dkgID := keygennofsm.DKGID(keygennofsm.GenerateDKGID(*big.NewInt(int64(i))))
			keygenMsgShare := keygennofsm.KeygenMsgShare{
				DKGID: dkgID,
			}
			data, err := bijson.Marshal(keygenMsgShare)
			if err != nil {
				logging.WithError(err).Error("Could not marshal keygenMsgShare")
				continue
			}
			err = abciServiceLibrary.KeygennofsmMethods().ReceiveMessage(keygennofsm.CreateKeygenMessage(keygennofsm.KeygenMessageRaw{
				KeygenID: (&keygennofsm.KeygenIDDetails{
					DKGID:       dkgID,
					DealerIndex: abciServiceLibrary.EthereumMethods().GetSelfIndex(),
				}).ToKeygenID(),
				Method: "share",
				Data:   data,
			}))
			if err != nil {
				logging.WithError(err).Error("Could not receive keygenmessage share")
				continue
			}
		}
	}
	return types.ResponseEndBlock{}
}

// Query -
func (app *ABCIApp) Query(reqQuery types.RequestQuery) (resQuery types.ResponseQuery) {
	logging.WithFields(logging.Fields{
		"Data":       reqQuery.Data,
		"stringData": string(reqQuery.Data),
	}).Debug("query to ABCIApp")

	telemetry.IncrementCounter(pcmn.TelemetryConstants.ABCIApp.QueryCounter, pcmn.TelemetryConstants.ABCIApp.Prefix)

	switch reqQuery.Path {
	case "GetIndexesFromVerifierID":
		logging.Debug("got a query for GetIndexesFromVerifierID")
		telemetry.IncrementCounter(pcmn.TelemetryConstants.ABCIApp.GetIndexesFromVerifierIDCounter, pcmn.TelemetryConstants.ABCIApp.Prefix)
		var queryArgs getIndexesQuery
		err := bijson.Unmarshal(reqQuery.Data, &queryArgs)
		if err != nil {
			return types.ResponseQuery{Code: 10, Info: fmt.Sprintf("could not parse query into arguments: %v string ver: %s ", reqQuery.Data, string(reqQuery.Data))}
		}

		keyIndexes, err := app.retrieveVerifierToKeyIndex(queryArgs.Verifier, queryArgs.VerifierID)
		if err != nil {
			return types.ResponseQuery{Code: 10, Info: fmt.Sprintf("val not found for query %v or data: %s, err: %v", reqQuery, string(reqQuery.Data), err)}
		}
		b, err := bijson.Marshal(keyIndexes)
		if err != nil {
			logging.WithError(err).Error("error serialising KeyIndexes")
		}

		logging.Debug("val found for query")
		// uint -> string -> bytes, when receiving do bytes -> string -> uint
		logging.Debug(string(b))
		return types.ResponseQuery{Code: 0, Value: []byte(b)}

	default:
		return types.ResponseQuery{Log: fmt.Sprintf("Invalid query path. Expected hash or tx, got %v", reqQuery.Path)}
	}
}

// Struct to parse arguments for query GetIndexesFromVerifierID
type getIndexesQuery struct {
	Verifier   string `json:"verifier"`
	VerifierID string `json:"verifier_id"`
}

func (app *ABCIApp) getIndexesFromVerifierID(verifier, verifierID string) (keyIndexes []big.Int, err error) {
	// struct for query args
	args := getIndexesQuery{Verifier: verifier, VerifierID: verifierID}
	argBytes, err := bijson.Marshal(args)
	if err != nil {
		return nil, fmt.Errorf("could not marshal query args error: %v", err)
	}
	reqQuery := types.RequestQuery{
		Data: argBytes,
		Path: "GetIndexesFromVerifierID",
	}

	res := app.Query(reqQuery)
	if res.Code == 10 {
		return nil, fmt.Errorf("Failed to find keyindexes with response code: %v", res.Info)
	}
	err = bijson.Unmarshal(res.Value, &keyIndexes)
	if err != nil {
		return nil, fmt.Errorf("could not parse retrieved keyindex list for %s error: %v", string(res.Value), err)
	}
	return keyIndexes, nil
}

type DBIteratorsSyncMap struct {
	sync.Map
}

func (d *DBIteratorsSyncMap) Get(identifier string) dbm.Iterator {
	iteratorInter, ok := d.Map.Load(identifier)
	if !ok {
		return nil
	}
	iterator, ok := iteratorInter.(dbm.Iterator)
	if !ok {
		return nil
	}
	return iterator
}
func (d *DBIteratorsSyncMap) Set(identifier string, iterator dbm.Iterator) {
	d.Map.Store(identifier, iterator)
}
func (d *DBIteratorsSyncMap) Exists(identifier string) (ok bool) {
	_, ok = d.Map.Load(identifier)
	return
}
func (d *DBIteratorsSyncMap) Delete(identifier string) {
	d.Map.Delete(identifier)
}

type NodeDetails pcmn.Node
type NodeDetailsID string

func (n *NodeDetails) ToNodeDetailsID() NodeDetailsID {
	return NodeDetailsID(
		strings.Join([]string{
			strconv.Itoa(n.Index),
			n.PubKey.X.Text(16),
			n.PubKey.Y.Text(16),
		}, pcmn.Delimiter1))
}
func (n *NodeDetails) FromNodeDetailsID(nodeDetailsID NodeDetailsID) {
	s := string(nodeDetailsID)
	substrings := strings.Split(s, pcmn.Delimiter1)

	if len(substrings) != 3 {
		return
	}
	index, err := strconv.Atoi(substrings[0])
	if err != nil {
		return
	}
	n.Index = index
	pubkeyX, ok := new(big.Int).SetString(substrings[1], 16)
	if !ok {
		return
	}
	n.PubKey.X = *pubkeyX
	pubkeyY, ok := new(big.Int).SetString(substrings[2], 16)
	if !ok {
		return
	}
	n.PubKey.Y = *pubkeyY
}

type MappingCounter struct {
	RequiredCount int
	KeyCount      int
}

// authenticateBftTx - parse headers and authenticate message from nodelist
func authenticateBftTx(tx []byte) (parsedTx DefaultBFTTxWrapper, senderDetails NodeDetails, err error) {
	err = bijson.Unmarshal(tx, &parsedTx)
	if err != nil {
		logging.Errorf("could not unmarshal headers from tx: %v", err)
		return parsedTx, senderDetails, err
	}

	curEpoch := abciServiceLibrary.EthereumMethods().GetCurrentEpoch()
	senderDetails, err = abciServiceLibrary.EthereumMethods().VerifyDataWithEpoch(parsedTx.PubKey, parsedTx.Signature, parsedTx.GetSerializedBody(), curEpoch)
	if err != nil {
		logging.Errorf("bfttx not valid: error %v, tx %v", err, stringify(parsedTx))
		return parsedTx, senderDetails, err
	}
	return
}
