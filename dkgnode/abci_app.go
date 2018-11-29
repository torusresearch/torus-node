package dkgnode

import (
	"encoding/json"
	"fmt"

	"github.com/YZhenY/torus/secp256k1"
	"github.com/tendermint/tendermint/abci/example/code"
	"github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/common"
	dbm "github.com/tendermint/tendermint/libs/db"
	"github.com/tendermint/tendermint/version"
)

var (
	stateKey        = []byte("stateKey")
	kvPairPrefixKey = []byte("kvPairKey:")

	ProtocolVersion version.Protocol = 0x1
)

type TransientState struct {
	State
}

// Nothing in state should be a pointer
type State struct {
	Epoch   uint   `json:"epoch"`
	Height  int64  `json:"height"`
	AppHash []byte `json:"app_hash"`
}

type ABCITransaction struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

func (app *ABCIApp) LoadState() State {
	stateBytes := app.db.Get(stateKey)
	var state State
	if len(stateBytes) != 0 {
		err := json.Unmarshal(stateBytes, &state)
		if err != nil {
			panic(err)
		}
	}
	app.state = state
	return state
}

func (app *ABCIApp) SaveState() State {
	stateBytes, err := json.Marshal(app.state)
	if err != nil {
		panic(err)
	}
	app.db.Set(stateKey, stateBytes)
	return app.state
}

func prefixKey(key []byte) []byte {
	return append(kvPairPrefixKey, key...)
}

//---------------------------------------------------

var _ types.Application = (*ABCIApp)(nil)

type ABCIApp struct {
	types.BaseApplication
	Suite          *Suite
	state          State
	db             dbm.DB
	transientState TransientState
}

func NewABCIApp(suite *Suite) *ABCIApp {
	db := dbm.NewMemDB()
	abciApp := ABCIApp{Suite: suite, db: db}

	state := abciApp.LoadState()
	abciApp.transientState = TransientState{state}
	return &abciApp
}

func (app *ABCIApp) Info(req types.RequestInfo) (resInfo types.ResponseInfo) {
	return types.ResponseInfo{
		Version:    version.ABCIVersion,
		AppVersion: ProtocolVersion.Uint64(),
	}
}

// tx is either "key=value" or just arbitrary bytes
func (app *ABCIApp) DeliverTx(tx []byte) types.ResponseDeliverTx {
	//JSON Unmarshal transaction
	fmt.Println("DELIVERINGTX", tx)

	//Validate transaction here
	correct, tags, err := app.ValidateBFTTx(tx) // TODO: doesnt just validate now.. break out update from validate?
	if err != nil {
		fmt.Println("could not validate BFTTx", err)
	}

	if !correct {
		//If validated, we save the transaction into the db
		fmt.Println("BFTTX IS WRONG")
		return types.ResponseDeliverTx{Code: code.CodeTypeUnauthorized}
	}

	if tags == nil {
		tags = new([]common.KVPair)
	}

	return types.ResponseDeliverTx{Code: code.CodeTypeOK, Tags: *tags}

	// var p Message
	// if err := rlp.DecodeBytes(tx, p); err != nil {
	// 	fmt.Println("ERROR DECODING RLP")
	// }
	// var p ABCITransaction
	// if err := json.Unmarshal(tx, &p); err != nil {
	// 	fmt.Println("transaction parse error", err)
	// 	// return types.ResponseDeliverTx{Code: code.CodeTypeEncodingError}
	// }

	// switch p.Type {
	// case "publicpoly":
	// 	fmt.Println("this is a public polyyyyyy")
	// }

	// var key, value []byte
	// parts := bytes.Split(tx, []byte("="))
	// if len(parts) == 2 {
	// 	key, value = parts[0], parts[1]
	// } else {
	// 	key, value = tx, tx
	// }
	// app.state.db.Set(prefixKey(key), value)
	// app.state.Size += 1

}

func (app *ABCIApp) CheckTx(tx []byte) types.ResponseCheckTx {

	return types.ResponseCheckTx{Code: code.CodeTypeOK}
}

func (app *ABCIApp) Commit() types.ResponseCommit {
	fmt.Println("COMMITING... HEIGHT:", app.state.Height)
	// retrieve state from memdb
	app.LoadState()
	app.state.AppHash = secp256k1.Keccak256(app.db.Get(stateKey))
	app.state.Epoch = app.transientState.Epoch
	app.state.Height += 1
	// commit to memdb
	app.transientState = TransientState{app.SaveState()}
	fmt.Println("APP STATE COMMITTED: ", app.state)
	return types.ResponseCommit{Data: app.state.AppHash}
}

func (app *ABCIApp) Query(reqQuery types.RequestQuery) (resQuery types.ResponseQuery) {
	// if reqQuery.Prove
	// 	value := app.state.db.Get(prefixKey(reqQuery.Data))
	// 	resQuery.Index = -1 // TODO make Proof return index
	// 	resQuery.Key = reqQuery.Data
	// 	resQuery.Value = value
	// 	if value != nil {
	// 		resQuery.Log = "exists"
	// 	} else {
	// 		resQuery.Log = "does not exist"
	// 	}
	// 	return
	// } else {
	// 	resQuery.Key = reqQuery.Data
	// 	value := app.state.db.Get(prefixKey(reqQuery.Data))
	// 	resQuery.Value = value
	// 	if value != nil {
	// 		resQuery.Log = "exists"
	// 	} else {
	// 		resQuery.Log = "does not exist"
	// 	}
	// 	return
	// }
	return
}
