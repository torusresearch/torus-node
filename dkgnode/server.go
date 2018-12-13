package dkgnode

/* All useful imports */
import (
	"context"
	b64 "encoding/base64"
	"encoding/hex"
	"fmt"
	"log"
	"math/big"
	"math/rand"
	"net/http"
	"strconv"
	"time"

	"github.com/intel-go/fastjson"
	"github.com/osamingo/jsonrpc"
	"github.com/patrickmn/go-cache"
	"github.com/rs/cors"
	tmquery "github.com/tendermint/tendermint/libs/pubsub/query"
	"github.com/tidwall/gjson"
	"github.com/torusresearch/torus/common"
	"github.com/torusresearch/torus/pvss"
)

type (
	PingHandler struct {
		ethSuite *EthSuite
	}
	PingParams struct {
		Message string `json:"message"`
	}
	PingResult struct {
		Message string `json:"message"`
	}
	SigncryptedHandler struct {
		suite *Suite
	}
	ShareLog struct {
		Timestamp          time.Time
		LogNumber          int
		ShareIndex         int
		UnsigncryptedShare []byte
		BroadcastId        []byte
	}
	ShareRequestHandler struct {
		suite *Suite
	}
	ShareRequestParams struct {
		Index   int    `json:"index"`
		IDToken string `json:"idtoken"`
		Email   string `json:"email"`
	}
	ShareRequestResult struct {
		Index    int    `json:"index"`
		HexShare string `json:"hexshare"`
	}
	SecretAssignHandler struct {
		suite *Suite
	}
	SecretAssignParams struct {
		Email string `json:"email"`
	}
	SecretAssignResult struct {
		ShareIndex int    `json:"id"`
		PubShareX  string `json:pubshare`
		PubShareY  string `json:pubshare`
		Address    string `json:"address`
	}
)

func (h PingHandler) ServeJSONRPC(c context.Context, params *fastjson.RawMessage) (interface{}, *jsonrpc.Error) {

	var p PingParams
	if err := jsonrpc.Unmarshal(params, &p); err != nil {
		return nil, err
	}
	// fmt.Println("Ping called from " + p.Message)

	return PingResult{
		Message: h.ethSuite.NodeAddress.Hex(),
	}, nil
}

func HandleSigncryptedShare(suite *Suite, tx KeyGenShareBFTTx) error {
	p := tx.SigncryptedMessage
	// check if this for us, or for another node
	tmpToPubKeyX, parsed := new(big.Int).SetString(p.ToPubKeyX, 16)
	if parsed == false {
		return jsonrpc.ErrParse()
	}
	if suite.EthSuite.NodePublicKey.X.Cmp(tmpToPubKeyX) != 0 {
		fmt.Println("Signcrypted share received but is not addressed to us")
		return nil
	}
	tmpRx, parsed := new(big.Int).SetString(p.RX, 16)
	if parsed == false {
		return jsonrpc.ErrParse()
	}
	tmpRy, parsed := new(big.Int).SetString(p.RY, 16)
	if parsed == false {
		return jsonrpc.ErrParse()
	}
	tmpSig, parsed := new(big.Int).SetString(p.Signature, 16)
	if parsed == false {
		return jsonrpc.ErrParse()
	}
	tmpPubKeyX, parsed := new(big.Int).SetString(p.FromPubKeyX, 16)
	if parsed == false {
		return jsonrpc.ErrParse()
	}
	tmpPubKeyY, parsed := new(big.Int).SetString(p.FromPubKeyY, 16)
	if parsed == false {
		return jsonrpc.ErrParse()
	}

	tmpCiphertext, err := hex.DecodeString(p.Ciphertext)
	if err != nil {
		return jsonrpc.ErrParse()
	}

	signcryption := common.Signcryption{
		Ciphertext: tmpCiphertext,
		R:          common.Point{X: *tmpRx, Y: *tmpRy},
		Signature:  *tmpSig,
	}
	// fmt.Println("RECIEVED SIGNCRYPTION")
	// fmt.Println(signcryption)
	unsigncryptedData, err := pvss.UnsigncryptShare(signcryption, *suite.EthSuite.NodePrivateKey.D, common.Point{*tmpPubKeyX, *tmpPubKeyY})
	if err != nil {
		fmt.Println("Error unsigncrypting share")
		fmt.Println(err)
		return &jsonrpc.Error{Code: 32602, Message: "Invalid params", Data: "error unsigncrypting share " + err.Error()}
	}

	// deserialize share and broadcastId from signcrypted data
	n := len(*unsigncryptedData)
	shareBytes := (*unsigncryptedData)[:n-32]
	broadcastId := (*unsigncryptedData)[n-32:]

	fmt.Println("Saved share from ", p.FromAddress)
	savedLog, found := suite.CacheSuite.CacheInstance.Get(p.FromAddress + "_LOG")
	newShareLog := ShareLog{time.Now().UTC(), 0, int(p.ShareIndex), shareBytes, broadcastId}
	// if not found, we create a new mapping
	if found {
		var tempLog = savedLog.([]ShareLog)
		// newShareLog = ShareLog{time.Now().UTC(), len(tempLog), p.ShareIndex, shareBytes, broadcastId}
		newShareLog.LogNumber = len(tempLog)
		tempLog = append(tempLog, newShareLog)
		suite.CacheSuite.CacheInstance.Set(p.FromAddress+"_LOG", tempLog, cache.NoExpiration)
	} else {
		newLog := make([]ShareLog, 1)
		newLog[0] = newShareLog
		suite.CacheSuite.CacheInstance.Set(p.FromAddress+"_LOG", newLog, cache.NoExpiration)
	}

	savedMapping, found := suite.CacheSuite.CacheInstance.Get(p.FromAddress + "_MAPPING")
	// if not found, we create a new mapping
	if found {
		var tmpMapping = savedMapping.(map[int]ShareLog)
		tmpMapping[int(p.ShareIndex)] = newShareLog
		suite.CacheSuite.CacheInstance.Set(p.FromAddress+"_MAPPING", tmpMapping, cache.NoExpiration)
	} else {
		newMapping := make(map[int]ShareLog)
		newMapping[int(p.ShareIndex)] = newShareLog
		// fmt.Println("CACHING SHARE FROM | ", h.suite.EthSuite.NodeAddress.Hex(), "=>", p.FromAddress)
		// fmt.Println(newShareLog)
		suite.CacheSuite.CacheInstance.Set(p.FromAddress+"_MAPPING", newMapping, cache.NoExpiration)
	}
	fmt.Println("SAVED SHARE FINISHED")
	return nil
}

//checks id for assignment and then the auth token for verification
// returns the node's share of the user's key
func (h ShareRequestHandler) ServeJSONRPC(c context.Context, params *fastjson.RawMessage) (interface{}, *jsonrpc.Error) {
	var p ShareRequestParams
	if err := jsonrpc.Unmarshal(params, &p); err != nil {
		return nil, err
	}
	tmpSi, found := h.suite.CacheSuite.CacheInstance.Get("Si_MAPPING")
	if !found {
		return nil, &jsonrpc.Error{Code: 32603, Message: "Internal error", Data: "Could not get si mapping here, not found"}
	}
	siMapping := tmpSi.(map[int]SiStore)
	if _, ok := siMapping[p.Index]; !ok {
		fmt.Println("LOOKUP: siMapping", siMapping)
		return nil, &jsonrpc.Error{Code: 32602, Message: "Invalid params", Data: "Could not lookup p.Index in siMapping"}
	}
	tmpInt := siMapping[p.Index].Value
	if p.IDToken == "blublu" { // TODO: remove
		fmt.Println("Share requested")
		fmt.Println("SHARE: ", tmpInt.Text(16))

		return ShareRequestResult{
			Index:    siMapping[p.Index].Index,
			HexShare: tmpInt.Text(16),
		}, nil
	}

	//checking oAuth token
	if oAuthCorrect, err := testOauth(h.suite, p.IDToken, p.Email); !oAuthCorrect {
		return nil, &jsonrpc.Error{Code: 32602, Message: "Invalid params", Data: "oauth is invalid, err: " + err.Error()}
	}

	res, err := h.suite.BftSuite.BftRPC.ABCIQuery("GetEmailIndex", []byte(p.Email))
	if err != nil {
		return nil, &jsonrpc.Error{Code: 32603, Message: "Internal error", Data: "Could not get email index here: " + err.Error()}
	}

	userIndex, err := strconv.ParseUint(string(res.Response.Value), 10, 64)
	if err != nil {
		return nil, &jsonrpc.Error{Code: 32603, Message: "Internal error", Data: "Cannot parse uint for user index here: " + err.Error()}
	}

	tmpInt = siMapping[int(userIndex)].Value

	return ShareRequestResult{
		Index:    siMapping[int(userIndex)].Index,
		HexShare: tmpInt.Text(16),
	}, nil
}

// assigns a user a secret, returns the same index if the user has been previously assigned
func (h SecretAssignHandler) ServeJSONRPC(c context.Context, params *fastjson.RawMessage) (interface{}, *jsonrpc.Error) {
	randomInt := rand.Int()
	var p SecretAssignParams
	if err := jsonrpc.Unmarshal(params, &p); err != nil {
		return nil, err
	}
	fmt.Println("CHECKING IF EMAIL IS PROVIDED")
	// no email provided TODO: email validation/ ddos protection
	if p.Email == "" {
		return nil, &jsonrpc.Error{Code: 32602, Message: "Input error", Data: "Email is empty"}
	}
	fmt.Println("CHECKING IF CAN GET EMAIL ADDRESS")

	// try to get get email index
	fmt.Println("CHECKING IF ALREADY ASSIGNED")
	previouslyAssignedIndex, ok := h.suite.ABCIApp.state.EmailMapping[p.Email]
	// already assigned
	if ok {
		//create users publicKey
		fmt.Println("previouslyAssignedIndex: ", previouslyAssignedIndex)
		finalUserPubKey, err := retrieveUserPubKey(h.suite, int(previouslyAssignedIndex))
		if err != nil {
			return nil, &jsonrpc.Error{Code: 32603, Message: "Internal error", Data: "could not retrieve secret from previously assigned index, please try again err, " + err.Error()}
		}

		//form address eth
		addr, err := common.PointToEthAddress(*finalUserPubKey)
		if err != nil {
			fmt.Println("derived user pub key has issues with address")
			return nil, &jsonrpc.Error{Code: 32603, Message: "Internal error"}
		}

		return SecretAssignResult{
			ShareIndex: int(previouslyAssignedIndex),
			PubShareX:  finalUserPubKey.X.Text(16),
			PubShareY:  finalUserPubKey.Y.Text(16),
			Address:    addr.String(),
		}, nil

		return nil, &jsonrpc.Error{Code: 32602, Message: "Input error", Data: "Email exists"}
	}

	//if all indexes have been assigned, bounce request. threshold at 20% TODO: Make  percentage variable
	if h.suite.ABCIApp.state.LastCreatedIndex < h.suite.ABCIApp.state.LastUnassignedIndex+20 {
		return nil, &jsonrpc.Error{Code: 429, Message: "System is under heavy load for assignments, please try again later"}
	}

	fmt.Println("CHECKING IF REACHED NEW ASSIGNMENT")
	// new assignment

	// broadcast assignment transaction
	hash, err := h.suite.BftSuite.BftRPC.Broadcast(DefaultBFTTxWrapper{&AssignmentBFTTx{Email: p.Email, Epoch: h.suite.ABCIApp.state.Epoch}})
	if err != nil {
		return nil, &jsonrpc.Error{Code: 32603, Message: "Internal error", Data: "Unable to broadcast: " + err.Error()}
	}

	fmt.Println("CHECKING IF SUBSCRIBE TO UPDATES", randomInt)
	// subscribe to updates
	query := tmquery.MustParse("tx.hash='" + hash.String() + "'")
	fmt.Println("BFTWS:, hashstring", hash.String(), randomInt)
	fmt.Println("BFTWS: querystring", query.String(), randomInt)

	fmt.Println("CHECKING IF GOT RESPONSES", randomInt)
	// wait for block to be committed
	var assignedIndex uint
	responseCh, err := h.suite.BftSuite.RegisterQuery(query.String(), 1)
	if err != nil {
		fmt.Println("BFTWS: could not register query, ", query.String())
	}

	for e := range responseCh {
		fmt.Println("BFTWS: gjson:", gjson.GetBytes(e, "query").String(), randomInt)
		fmt.Println("BFTWS: queryString", query.String(), randomInt)
		if gjson.GetBytes(e, "query").String() != query.String() {
			continue
		}
		res, err := h.suite.BftSuite.BftRPC.ABCIQuery("GetEmailIndex", []byte(p.Email))
		if err != nil {
			return nil, &jsonrpc.Error{Code: 32603, Message: "Internal error", Data: "Failed to check if email exists after assignment: " + err.Error()}
		}
		if string(res.Response.Value) == "" {
			return nil, &jsonrpc.Error{Code: 32603, Message: "Internal error", Data: "Failed to find email after it has been assigned"}
		}
		assignedIndex64, err := strconv.ParseUint(string(res.Response.Value), 10, 64)
		assignedIndex = uint(assignedIndex64)
		if err != nil {
			return nil, &jsonrpc.Error{Code: 32603, Message: "Internal error", Data: "Failed to parse uint for returned assignment index: " + fmt.Sprint(res) + " Error: " + err.Error()}
		}
		fmt.Println("EXITING RESPONSES LISTENER", randomInt)
		break
	}

	// TODO: after ws response has returned as the secret mapping could have changed, should be initializable anywhere
	tmpSecretMAPPING, found := h.suite.CacheSuite.CacheInstance.Get("Secret_MAPPING")
	if !found {
		return nil, &jsonrpc.Error{Code: 32603, Message: "Internal error", Data: "Could not get sec mapping here after ws reply"}
	}
	secretMapping := tmpSecretMAPPING.(map[int]SecretStore)
	if err := jsonrpc.Unmarshal(params, &p); err != nil {
		return nil, err
	}

	if secretMapping[int(assignedIndex)].Secret == nil {
		fmt.Println("LOOKUP: secretmapping", secretMapping)
		fmt.Println("LOOKUP: SHOULD BE ERROR")
		// return nil, &jsonrpc.Error{Code: 32603, Message: "Internal error", Data: "Could not retrieve secret from secret mapping, please try again"}
	}

	//create users publicKey
	finalUserPubKey, err := retrieveUserPubKey(h.suite, int(assignedIndex))
	if err != nil {
		return nil, &jsonrpc.Error{Code: 32603, Message: "Internal error", Data: "Could not retrieve user public key through query, please try again, err " + err.Error()}
	}

	//form address eth
	addr, err := common.PointToEthAddress(*finalUserPubKey)
	if err != nil {
		fmt.Println("derived user pub key has issues with address")
		return nil, &jsonrpc.Error{Code: 32603, Message: "Internal error"}
	}

	return SecretAssignResult{
		ShareIndex: int(assignedIndex),
		PubShareX:  finalUserPubKey.X.Text(16),
		PubShareY:  finalUserPubKey.Y.Text(16),
		Address:    addr.String(),
	}, nil
}

//gets assigned index and returns users public key
func retrieveUserPubKey(suite *Suite, assignedIndex int) (*common.Point, error) {

	resultPubPolys, err := suite.BftSuite.BftRPC.TxSearch("share_index<="+strconv.Itoa(assignedIndex)+" AND "+"share_index>="+strconv.Itoa(assignedIndex), false, 10, 10)
	if err != nil {
		return nil, err
	}
	fmt.Println("SEARCH RESULT NUMBER", resultPubPolys.TotalCount)

	//create users publicKey
	var finalUserPubKey common.Point //initialize empty pubkey to fill
	for i := 0; i < resultPubPolys.TotalCount; i++ {
		var PubPolyTx PubPolyBFTTx
		defaultWrapper := DefaultBFTTxWrapper{&PubPolyTx}
		//get rid of signature on bft
		err = defaultWrapper.DecodeBFTTx(resultPubPolys.Txs[i].Tx[len([]byte("mug00")):])
		if err != nil {
			fmt.Println("Could not decode pubpolybfttx", err)
		}
		pubPolyTx := defaultWrapper.BFTTx.(*PubPolyBFTTx)
		if i == 0 {
			finalUserPubKey = common.Point{pubPolyTx.PubPoly[0].X, pubPolyTx.PubPoly[0].Y}
		} else {
			//get g^z and add
			tempX, tempY := suite.EthSuite.secp.Add(&finalUserPubKey.X, &finalUserPubKey.Y, &pubPolyTx.PubPoly[0].X, &pubPolyTx.PubPoly[0].Y)
			finalUserPubKey = common.Point{*tempX, *tempY}
		}
	}

	return &finalUserPubKey, nil
}

func setUpServer(suite *Suite, port string) {
	mr := jsonrpc.NewMethodRepository()
	if err := mr.RegisterMethod("Ping", PingHandler{suite.EthSuite}, PingParams{}, PingResult{}); err != nil {
		log.Fatalln(err)
	}
	if err := mr.RegisterMethod("ShareRequest", ShareRequestHandler{suite}, ShareRequestParams{}, ShareRequestResult{}); err != nil {
		log.Fatalln(err)
	}
	if err := mr.RegisterMethod("SecretAssign", SecretAssignHandler{suite}, SecretAssignParams{}, SecretAssignResult{}); err != nil {
		log.Fatalln(err)
	}

	go func() {
		// TODO: waiting for websocket connection to be ready
		for suite.BftSuite.BftRPCWSStatus != "up" {
			time.Sleep(1 * time.Second)
		}
		listenForShares(suite, suite.Config.KeysPerEpoch*suite.Config.NumberOfNodes*suite.Config.NumberOfNodes)
	}()

	mux := http.NewServeMux()
	mux.Handle("/jrpc", mr)
	mux.HandleFunc("/jrpc/debug", mr.ServeDebug)
	handler := cors.Default().Handler(mux)
	if suite.Flags.Production {
		if err := http.ListenAndServeTLS(":443",
			"/root/https/fullchain.pem",
			"/root/https/privkey.pem",
			handler,
		); err != nil {
			log.Fatalln(err)
		}
	} else {
		// listenandserve creates a thread in the main that loops indefinitely
		if err := http.ListenAndServe(":"+port, handler); err != nil {
			log.Fatalln(err)
		}
	}
	fmt.Println("SERVER STOPPED")
}

func listenForShares(suite *Suite, count int) {
	query := tmquery.MustParse("keygeneration.sharecollection='1'")
	fmt.Println("QUERY IS:", query)
	// note: we also get back the initial "{}"
	// data comes back in bytes of utf-8 which correspond
	// to a base64 encoding of the original data
	responseCh, err := suite.BftSuite.RegisterQuery(query.String(), count)
	if err != nil {
		fmt.Println("BFTWS: failure to registerquery", query.String())
		return
	}
	for e := range responseCh {
		if gjson.GetBytes(e, "query").String() != "keygeneration.sharecollection='1'" {
			continue
		}
		fmt.Println("sub got ", string(e[:]))
		res, err := b64.StdEncoding.DecodeString(gjson.GetBytes(e, "data.value.TxResult.tx").String())
		if err != nil {
			fmt.Println("error decoding b64", err)
			continue
		}
		// valid messages should start with mug00
		if len(res) < 5 || string(res[:len([]byte("mug00"))]) != "mug00" {
			fmt.Println("Message not prefixed with mug00")
			continue
		}
		keyGenShareBFTTx := DefaultBFTTxWrapper{&KeyGenShareBFTTx{}}
		err = keyGenShareBFTTx.DecodeBFTTx(res[len([]byte("mug00")):])
		if err != nil {
			fmt.Println("error decoding bfttx", err)
			continue
		}
		keyGenShareTx := keyGenShareBFTTx.BFTTx.(*KeyGenShareBFTTx)
		err = HandleSigncryptedShare(suite, *keyGenShareTx)
		if err != nil {
			fmt.Println("failed to handle signcrypted share", err)
			continue
		}
	}
}
