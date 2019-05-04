package dkgnode

import (
	"encoding/json"
	"fmt"
	"math/big"

	tmbtcec "github.com/tendermint/btcd/btcec"
	"github.com/tendermint/tendermint/abci/example/code"
	"github.com/tendermint/tendermint/abci/types"
	tmcmn "github.com/tendermint/tendermint/libs/common"
	dbm "github.com/tendermint/tendermint/libs/db"
	"github.com/tendermint/tendermint/version"
	"github.com/torusresearch/bijson"
	"github.com/torusresearch/torus-public/common"
	"github.com/torusresearch/torus-public/logging"
	"github.com/torusresearch/torus-public/secp256k1"
)

var (
	stateKey        = []byte("stateKey")
	kvPairPrefixKey = []byte("kvPairKey:")
	// ProtocolVersion -
	ProtocolVersion version.Protocol = 0x1
)

// KeyAssignmentPublic -
type KeyAssignmentPublic struct {
	Index     big.Int
	PublicKey common.Point
	Threshold int
	Verifiers map[string][]string // Verifier => VerifierID
}

// KeyAssignment -
type KeyAssignment struct {
	KeyAssignmentPublic
	Share big.Int // Or Si
}

// TorusID -
type TorusID struct {
	Index      int
	KeyIndexes []big.Int
}

// State - nothing in state should be a pointer
// Remember to initialize mappings in NewABCIApp()
type State struct {
	Epoch                    uint                            `json:"epoch"`
	Height                   int64                           `json:"height"`
	AppHash                  []byte                          `json:"app_hash"`
	LastUnassignedIndex      uint                            `json:"last_unassigned_index"`
	LastUnassignedTorusIndex uint                            `json:"last_unassigned_torus_index"`
	LastCreatedIndex         uint                            `json:"last_created_index"`
	KeyMapping               map[string]KeyAssignmentPublic  `json:"key_mapping"`           // KeyIndex => KeyAssignmentPublic
	VerifierToKeyIndex       map[string](map[string]TorusID) `json:"verifier_to_key_index"` // Verifier => VerifierID => KeyIndex
	ValidatorSet             []types.ValidatorUpdate         `json:"-"`                     // `json:"validator_set"`
	UpdateValidators         bool                            `json:"-"`                     // `json:"update_validators"`
}

// ABCITransaction -
type ABCITransaction struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

// LoadState -
func (app *ABCIApp) LoadState() State {
	stateBytes := app.db.Get(stateKey)
	var state State
	if len(stateBytes) != 0 {
		err := json.Unmarshal(stateBytes, &state)
		if err != nil {
			panic(err)
		}
	}
	app.state = &state
	return state
}

// SaveState -
func (app *ABCIApp) SaveState() State {
	stateBytes, err := json.Marshal(app.state)
	if err != nil {
		panic(err)
	}
	app.db.Set(stateKey, stateBytes)
	return *app.state
}

func prefixKey(key []byte) []byte {
	return append(kvPairPrefixKey, key...)
}

//---------------------------------------------------

var _ types.Application = (*ABCIApp)(nil)

// ABCIApp -
type ABCIApp struct {
	types.BaseApplication
	Suite *Suite
	state *State
	db    dbm.DB
}

// NewABCIApp -
func NewABCIApp(suite *Suite) *ABCIApp {
	db := dbm.NewMemDB()
	v := make(map[string](map[string]TorusID))
	for _, ver := range suite.DefaultVerifier.ListVerifiers() {
		v[ver] = make(map[string]TorusID)
	}
	abciApp := ABCIApp{
		Suite: suite, db: db,
		state: &State{
			Epoch:                    1,
			Height:                   0,
			LastUnassignedIndex:      0,
			LastUnassignedTorusIndex: 0,
			LastCreatedIndex:         0,
			KeyMapping:               make(map[string]KeyAssignmentPublic),
			VerifierToKeyIndex:       v,
		}}
	return &abciApp
}

func (app *ABCIApp) Info(req types.RequestInfo) (resInfo types.ResponseInfo) {
	return types.ResponseInfo{
		Version:          version.ABCIVersion,
		AppVersion:       ProtocolVersion.Uint64(),
		LastBlockAppHash: app.state.AppHash,
		LastBlockHeight:  app.state.Height,
	}
}

// DeliverTx - tx is either "key=value" or just arbitrary bytes
func (app *ABCIApp) DeliverTx(tx []byte) types.ResponseDeliverTx {
	//JSON Unmarshal transaction
	// logging.Debugf("DELIVERINGTX %s", tx)

	//Validate transaction here
	correct, tags, err := app.ValidateAndUpdateAndTagBFTTx(tx) // TODO: doesnt just validate now.. break out update from validate?
	if err != nil {
		logging.Errorf("Could not validate BFTTx %s", err)
	}

	if !correct {
		//If validated, we save the transaction into the db
		logging.Debug("BFTTX IS WRONG")
		return types.ResponseDeliverTx{Code: code.CodeTypeUnauthorized}
	}

	if tags == nil {
		tags = new([]tmcmn.KVPair)
	}

	return types.ResponseDeliverTx{Code: code.CodeTypeOK, Tags: *tags}
}

func (app *ABCIApp) CheckTx(tx []byte) types.ResponseCheckTx {

	return types.ResponseCheckTx{Code: code.CodeTypeOK}
}

// Commit happens before DeliverTx
func (app *ABCIApp) Commit() types.ResponseCommit {
	// logging.Debugf("COMMITING... HEIGHT: %s", app.state.Height)
	// retrieve state from memdb
	if app.state == nil {
		app.LoadState()
	}

	// update state
	app.state.AppHash = secp256k1.Keccak256(app.db.Get(stateKey))
	app.state.Height += 1
	// commit to memdb
	app.SaveState()
	// logging.Debugf("APP STATE COMMITTED: %s", app.state)

	return types.ResponseCommit{Data: app.state.AppHash}
}

// Query -
func (app *ABCIApp) Query(reqQuery types.RequestQuery) (resQuery types.ResponseQuery) {
	logging.Debugf("%v", app.state)
	logging.Debugf("QUERY TO ABCIAPP %s %s", reqQuery.Data, string(reqQuery.Data))
	switch reqQuery.Path {

	case "GetIndexesFromVerifierID":
		logging.Debug("GOT A QUERY FOR GetIndexesFromVerifierID")
		verifierRef, found := app.state.VerifierToKeyIndex["google"]
		if !found {
			logging.Debug("verifier not found for query")
			logging.Debugf("%v", reqQuery)
			logging.Debugf("%v", reqQuery.Data)
			logging.Debug(string(reqQuery.Data))
			return types.ResponseQuery{Value: []byte("")}
		}
		verifierIDRef, found := verifierRef[string(reqQuery.Data)]
		if !found {
			logging.Debug("val not found for query")
			logging.Debugf("%v", reqQuery)
			logging.Debugf("%v", reqQuery.Data)
			logging.Debug(string(reqQuery.Data))
			return types.ResponseQuery{Value: []byte("")}
		}
		b, err := bijson.Marshal(verifierIDRef.KeyIndexes)
		if err != nil {
			logging.Errorf("Error serializeing KeyIndexes: %v", err)
		}

		logging.Debug("val found for query")
		// uint -> string -> bytes, when receiving do bytes -> string -> uint
		logging.Debug(string(b))
		return types.ResponseQuery{Value: []byte(b)}

	// case "GetKeyGenComplete":
	// 	logging.Debug("GOT A QUERY FOR GETKEYGENCOMPLETE")
	// 	logging.Debugf("for Epoch: %s", string(reqQuery.Data))
	// 	return types.ResponseQuery{
	// 		Value: []byte(app.state.LocalStatus.Current()),
	// 	}

	default:
		return types.ResponseQuery{Log: fmt.Sprintf("Invalid query path. Expected hash or tx, got %v", reqQuery.Path)}
	}
	// return types.ResponseQuery{Height: int64(0)}
}

// EndBlock - update the validator set
func (app *ABCIApp) EndBlock(req types.RequestEndBlock) types.ResponseEndBlock {
	//TODO: add condition so that validator set is not dialed/updated constantly
	//Here we go through our nodelist in EthSuite, create the validator set and set it in "EndBlock" where we edit the validator set
	if app.state.UpdateValidators == true {
		valSet := app.state.ValidatorSet
		//set update val back to false
		app.state.UpdateValidators = false
		logging.Debugf("PEER SET: %s", app.Suite.BftSuite.BftNode.Switch().Peers())
		logging.Debugf("VALIDATOR SET: %s", valSet)
		return types.ResponseEndBlock{ValidatorUpdates: valSet}
	}
	return types.ResponseEndBlock{}
}

func convertNodeListToValidatorUpdate(nodeList []*NodeReference) []types.ValidatorUpdate {
	var valSet []types.ValidatorUpdate
	for i := range nodeList {
		//"address" for secp256k1 needs to bbe in some serialized method
		pubkeyObject := tmbtcec.PublicKey{
			X: nodeList[i].PublicKey.X,
			Y: nodeList[i].PublicKey.Y,
		}
		valSet = append(valSet, types.ValidatorUpdate{
			PubKey: types.PubKey{Type: "secp256k1", Data: pubkeyObject.SerializeCompressed()},
			Power:  1,
		})
	}
	return valSet
}
