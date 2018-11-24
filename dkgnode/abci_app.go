package dkgnode

import (
	"encoding/binary"
	"encoding/json"
	"fmt"

	"github.com/tendermint/tendermint/abci/example/code"
	"github.com/tendermint/tendermint/abci/types"
	dbm "github.com/tendermint/tendermint/libs/db"
	"github.com/tendermint/tendermint/version"
)

var (
	stateKey        = []byte("stateKey")
	kvPairPrefixKey = []byte("kvPairKey:")

	ProtocolVersion version.Protocol = 0x1
)

type State struct {
	db      dbm.DB
	Size    int64  `json:"size"`
	Height  int64  `json:"height"`
	AppHash []byte `json:"app_hash"`
}

type ABCITransaction struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

func loadState(db dbm.DB) State {
	stateBytes := db.Get(stateKey)
	var state State
	if len(stateBytes) != 0 {
		err := json.Unmarshal(stateBytes, &state)
		if err != nil {
			panic(err)
		}
	}
	state.db = db
	return state
}

func saveState(state State) {
	stateBytes, err := json.Marshal(state)
	if err != nil {
		panic(err)
	}
	state.db.Set(stateKey, stateBytes)
}

func prefixKey(key []byte) []byte {
	return append(kvPairPrefixKey, key...)
}

//---------------------------------------------------

var _ types.Application = (*ABCIApp)(nil)

type ABCIApp struct {
	types.BaseApplication
	Suite *Suite
	state State
}

func NewABCIApp(suite *Suite) *ABCIApp {
	state := loadState(dbm.NewMemDB())
	return &ABCIApp{Suite: suite, state: state}
}

func (app *ABCIApp) Info(req types.RequestInfo) (resInfo types.ResponseInfo) {
	return types.ResponseInfo{
		Data:       fmt.Sprintf("{\"size\":%v}", app.state.Size),
		Version:    version.ABCIVersion,
		AppVersion: ProtocolVersion.Uint64(),
	}
}

// tx is either "key=value" or just arbitrary bytes
func (app *ABCIApp) DeliverTx(tx []byte) types.ResponseDeliverTx {
	//JSON Unmarshal transaction
	fmt.Println("DELIVERINGTX", tx)

	//Validate transaction here
	correct, tags, err := app.ValidateBFTTx(tx)
	if err != nil {
		fmt.Println("could not validate BFTTx", err)
	}

	if !correct {
		//If validated, we save the transaction into the db
		fmt.Println("BFTTX IS WRONG")
		return types.ResponseDeliverTx{Code: code.CodeTypeUnauthorized}
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

	return types.ResponseCheckTx{Code: code.CodeTypeOK, GasWanted: 1}
}

func (app *ABCIApp) Commit() types.ResponseCommit {
	// Using a memdb - just return the big endian size of the db
	appHash := make([]byte, 8)
	binary.PutVarint(appHash, app.state.Size)
	app.state.AppHash = appHash
	app.state.Height += 1
	saveState(app.state)
	return types.ResponseCommit{Data: appHash}
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
