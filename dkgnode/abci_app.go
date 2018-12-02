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
// Remember to initialize mappings in NewABCIApp()
type State struct {
	Epoch               uint                       `json:"epoch"`
	Height              int64                      `json:"height"`
	AppHash             []byte                     `json:"app_hash"`
	LastUnassignedIndex uint                       `json:"last_unassigned_index"`
	EmailMapping        map[string]uint            `json:"email_mapping"`
	NodeStatus          map[uint]map[string]string `json:"node_status"` // Node(Index=0) status value for keygen_complete is State.Status[0]["keygen_complete"] = "Y"
	LocalStatus         map[string]string          `json:"-"`           //
	ValidatorSet        []types.ValidatorUpdate    `json:"-"`           // `json:"validator_set"`
	UpdateValidators    bool                       `json:"-"`           // `json:"update_validators"`
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
	abciApp := ABCIApp{
		Suite: suite, db: db,
		state: &State{
			Epoch:               0,
			Height:              0,
			LastUnassignedIndex: 0,
			EmailMapping:        make(map[string]uint),
			NodeStatus:          make(map[uint]map[string]string),
			LocalStatus:         make(map[string]string),
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

// tx is either "key=value" or just arbitrary bytes
func (app *ABCIApp) DeliverTx(tx []byte) types.ResponseDeliverTx {
	//JSON Unmarshal transaction
	fmt.Println("DELIVERINGTX", tx)

	//Validate transaction here
	correct, tags, err := app.ValidateAndUpdateAndTagBFTTx(tx) // TODO: doesnt just validate now.. break out update from validate?
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
	fmt.Println(app.state)
	fmt.Println("QUERY TO ABCIAPP", reqQuery.Data, string(reqQuery.Data))
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

	case "GetKeyGenComplete":
		fmt.Println("GOT A QUERY FOR GETKEYGENCOMPLETE")
		fmt.Println("for Epoch: ", string(reqQuery.Data))
		return types.ResponseQuery{
			Value: []byte(app.state.LocalStatus["keygen_all_complete_epoch_"+string(reqQuery.Data)]),
		}

	default:
		return types.ResponseQuery{Log: fmt.Sprintf("Invalid query path. Expected hash or tx, got %v", reqQuery.Path)}
	}
	return types.ResponseQuery{Height: int64(0)}
}

// Update the validator set
func (app *ABCIApp) EndBlock(req types.RequestEndBlock) types.ResponseEndBlock {
	//TODO: add condition so that validator set is not dialed/updated constantly
	//Here we go through our nodelist in EthSuite, create the validator set and set it in "EndBlock" where we edit the validator set
	if app.state.UpdateValidators == true {
		valSet := app.state.ValidatorSet
		//set update val back to false
		app.state.UpdateValidators = false
		fmt.Println("PEER SET: ", app.Suite.BftSuite.BftNode.Switch().Peers())
		fmt.Println("VALIDATOR SET: ", valSet)
		return types.ResponseEndBlock{ValidatorUpdates: valSet}
	}
	return types.ResponseEndBlock{}
}

func convertNodeListToValidatorUpdate(nodeList []*NodeReference) []types.ValidatorUpdate {
	var valSet []types.ValidatorUpdate
	for i := range nodeList {
		//Here we add the node as a persistent peer too
		// addr, err := p2p.NewNetAddressString(nodeList[i].P2PConnection)
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
