package dkgnode

import (
	"encoding/json"
	"fmt"

	"github.com/YZhenY/torus/secp256k1"
	tmbtcec "github.com/tendermint/btcd/btcec"
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

// Nothing in state should be a pointer
type State struct {
	Epoch        uint            `json:"epoch"`
	Height       int64           `json:"height"`
	AppHash      []byte          `json:"app_hash"`
	LastIndex    uint            `json:"last_index"`
	EmailMapping map[string]uint `json:"email_mapping"`
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
	app.state = &state
	return state
}

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

type ABCIApp struct {
	types.BaseApplication
	Suite *Suite
	state *State
	db    dbm.DB
}

func NewABCIApp(suite *Suite) *ABCIApp {
	db := dbm.NewMemDB()
	abciApp := ABCIApp{Suite: suite, db: db, state: &State{Epoch: 0, Height: 0, LastIndex: 0, EmailMapping: make(map[string]uint)}}
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

// tx is either "key=value" or just arbitrary bytes
func (app *ABCIApp) DeliverTx(tx []byte) types.ResponseDeliverTx {
	//JSON Unmarshal transaction
	fmt.Println("DELIVERINGTX", tx)

	//Validate transaction here
	correct, tags, err := app.ValidateAndUpdateBFTTx(tx) // TODO: doesnt just validate now.. break out update from validate?
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

// NOTE: Commit happens before DeliverTx
func (app *ABCIApp) Commit() types.ResponseCommit {
	fmt.Println("COMMITING... HEIGHT:", app.state.Height)
	// retrieve state from memdb
	if app.state == nil {
		app.LoadState()
	}

	// init if does not exist
	if app.state.EmailMapping == nil {
		app.state.EmailMapping = make(map[string]uint)
		fmt.Println("INITIALIZED APP STATE EMAIL MAPPING")
	} else {
		fmt.Println("app state email mapping has stuff", app.state.EmailMapping)
	}

	// update state
	app.state.AppHash = secp256k1.Keccak256(app.db.Get(stateKey))
	app.state.Height += 1
	// commit to memdb
	app.SaveState()
	fmt.Println("APP STATE COMMITTED: ", app.state)

	return types.ResponseCommit{Data: app.state.AppHash}
}

func (app *ABCIApp) Query(reqQuery types.RequestQuery) (resQuery types.ResponseQuery) {
	fmt.Println("QUERY TO ABCIAPP", reqQuery.Data)
	fmt.Println("app state", app.state)
	fmt.Println("email mapping", app.state.EmailMapping)
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
	switch reqQuery.Path {
	case "GetEmailIndex":
		fmt.Println("GOT A QUERY FOR GETEMAILINDEX")
		val, found := app.state.EmailMapping[string(reqQuery.Data)]
		if !found {
			fmt.Println("val not found for query")
			fmt.Println(reqQuery)
			fmt.Println(reqQuery.Data)
			fmt.Println(string(reqQuery.Data))
			return types.ResponseQuery{Value: []byte("")}
		}
		fmt.Println("val found for query")
		// uint -> string -> bytes, when receiving do bytes -> string -> uint
		fmt.Println(fmt.Sprint(val))
		return types.ResponseQuery{Value: []byte(fmt.Sprint(val))}
	default:
		return types.ResponseQuery{Log: fmt.Sprintf("Invalid query path. Expected hash or tx, got %v", reqQuery.Path)}
	}
	return types.ResponseQuery{Height: int64(0)}
}

// Update the validator set
func (app *ABCIApp) EndBlock(req types.RequestEndBlock) types.ResponseEndBlock {
	//TODO: add condition so that validator set is not dialed/updated constantly
	//Here we go through our nodelist in EthSuite, create the validator set and set it in "EndBlock" where we edit the validator set
	if app.Suite.BftSuite.UpdateVal == true {
		var valSet []types.ValidatorUpdate
		for i := range app.Suite.EthSuite.NodeList {
			//Here we add the node as a persistent peer too
			// addr, err := p2p.NewNetAddressString(app.Suite.EthSuite.NodeList[i].P2PConnection)
			// if err != nil {
			// 	fmt.Println("Not able to add peer", err)
			// }
			//check if existing peer is dialed
			// if !app.Suite.BftSuite.BftNode.Switch().IsDialingOrExistingAddress(addr) {
			// 	fmt.Println("DIALING ADDRESS: ", addr)
			// 	err = app.Suite.BftSuite.BftNode.Switch().DialPeerWithAddress(addr, true) //if not add peer
			// 	if err != nil {
			// 		fmt.Println("Could not add peer: ", err)
			// 	}
			// }

			//"address" for secp256k1 needs to bbe in some serialized method

			pubkeyObject := tmbtcec.PublicKey{
				X: app.Suite.EthSuite.NodeList[i].PublicKey.X,
				Y: app.Suite.EthSuite.NodeList[i].PublicKey.Y,
			}
			valSet = append(valSet, types.ValidatorUpdate{
				PubKey: types.PubKey{Type: "secp256k1", Data: pubkeyObject.SerializeCompressed()},
				Power:  1,
			})
		}
		app.Suite.BftSuite.UpdateVal = false
		fmt.Println("PEER SET: ", app.Suite.BftSuite.BftNode.Switch().Peers())
		fmt.Println("VALIDATOR SET: ", valSet)
		return types.ResponseEndBlock{ValidatorUpdates: valSet}
	}
	return types.ResponseEndBlock{}
}
