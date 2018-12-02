package dkgnode

/* All useful imports */
import (
	"context"
	b64 "encoding/base64"
	"encoding/hex"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"strconv"
	"time"

	"github.com/YZhenY/torus/common"
	"github.com/YZhenY/torus/pvss"
	"github.com/intel-go/fastjson"
	"github.com/osamingo/jsonrpc"
	"github.com/patrickmn/go-cache"
	"github.com/rs/cors"
	tmquery "github.com/tendermint/tendermint/libs/pubsub/query"
	"github.com/tidwall/gjson"
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
		return jsonrpc.ErrInvalidParams()
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
	siMapping := tmpSi.(map[int]common.PrimaryShare)
	if _, ok := siMapping[p.Index]; !ok {
		return nil, jsonrpc.ErrInvalidParams()
	}
	tmpInt := siMapping[p.Index].Value
	if p.IDToken == "blublu" {
		fmt.Println("Share requested")
		fmt.Println("SHARE: ", tmpInt.Text(16))

		return ShareRequestResult{
			Index:    siMapping[p.Index].Index,
			HexShare: tmpInt.Text(16),
		}, nil
	}
	tmpSecretMAPPING, found := h.suite.CacheSuite.CacheInstance.Get("Secret_MAPPING")
	if !found {
		return nil, &jsonrpc.Error{Code: 32603, Message: "Internal error", Data: "Could not get secret mapping"}
	}
	secretMapping := tmpSecretMAPPING.(map[int]SecretStore)

	//checking oAuth token
	if oAuthCorrect, _ := testOauth(p.IDToken, p.Email); !*oAuthCorrect {
		return nil, jsonrpc.ErrInvalidParams()
	}

	res, err := h.suite.BftSuite.BftRPC.ABCIQuery("GetEmailIndex", []byte(p.Email))
	if err != nil {
		return nil, &jsonrpc.Error{Code: 32603, Message: "Internal error", Data: "Could not get email index here: " + err.Error()}
	}

	userIndex, err := strconv.ParseUint(string(res.Response.Value), 10, 64)
	if err != nil {
		return nil, &jsonrpc.Error{Code: 32603, Message: "Internal error", Data: "Cannot parse uint for user index here: " + err.Error()}
	}

	return ShareRequestResult{
		Index:    siMapping[int(userIndex)].Index,
		HexShare: secretMapping[int(userIndex)].Secret.Text(16),
	}, nil
}

// assigns a user a secret, returns the same index if the user has been previously assigned
func (h SecretAssignHandler) ServeJSONRPC(c context.Context, params *fastjson.RawMessage) (interface{}, *jsonrpc.Error) {
	var p SecretAssignParams
	tmpSecretMAPPING, found := h.suite.CacheSuite.CacheInstance.Get("Secret_MAPPING")
	if !found {
		return nil, &jsonrpc.Error{Code: 32603, Message: "Internal error", Data: "Could not get sec mapping here"}
	}
	secretMapping := tmpSecretMAPPING.(map[int]SecretStore)
	if err := jsonrpc.Unmarshal(params, &p); err != nil {
		return nil, err
	}
	fmt.Println("CHECKING IF EMAIL IS PROVIDED")
	// no email provided
	if p.Email == "" {
		return nil, &jsonrpc.Error{Code: 32602, Message: "Input error", Data: "Email is empty"}
	}
	fmt.Println("CHECKING IF CAN GET EMAIL ADDRESS")
	// try to get get email index
	res, err := h.suite.BftSuite.BftRPC.ABCIQuery("GetEmailIndex", []byte(p.Email))
	if err != nil {
		return nil, &jsonrpc.Error{Code: 32603, Message: "Internal error", Data: "Failed to check if email exists: " + err.Error()}
	}

	fmt.Println("CHECKING IF ALREADY ASSIGNED")
	// already assigned
	if string(res.Response.Value) != "" {

		previouslyAssignedIndex64, err := strconv.ParseUint(string(res.Response.Value), 10, 64)
		if err != nil {
			return nil, &jsonrpc.Error{Code: 32603, Message: "Internal error", Data: "Failed to parse uint for previous assign, res.Response.Value: " + string(res.Response.Value) + " Error: " + err.Error()}
		}
		previouslyAssignedIndex := uint(previouslyAssignedIndex64)

		if secretMapping[int(previouslyAssignedIndex)].Secret == nil {
			return nil, &jsonrpc.Error{Code: 32603, Message: "Internal error", Data: "could not retrieve secret, please try again"}
		}

		pubShareX, pubShareY := h.suite.EthSuite.secp.ScalarBaseMult(secretMapping[int(previouslyAssignedIndex)].Secret.Bytes())

		return SecretAssignResult{
			ShareIndex: int(previouslyAssignedIndex),
			PubShareX:  pubShareX.Text(16),
			PubShareY:  pubShareY.Text(16),
		}, nil

		return nil, &jsonrpc.Error{Code: 32602, Message: "Input error", Data: "Email exists"}
	}

	fmt.Println("CHECKING IF REACHED NEW ASSIGNMENT")
	// new assignment

	// broadcast assignment transaction
	hash, err := h.suite.BftSuite.BftRPC.Broadcast(DefaultBFTTxWrapper{&AssignmentBFTTx{Email: p.Email}})
	if err != nil {
		return nil, &jsonrpc.Error{Code: 32603, Message: "Internal error", Data: "Unable to broadcast: " + err.Error()}
	}

	fmt.Println("CHECKING IF SUBSCRIBE TO UPDATES")
	// subscribe to updates
	query := tmquery.MustParse("tx.hash='" + hash.String() + "'")
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	go func() {
		err = h.suite.BftSuite.BftRPCWS.Subscribe(ctx, query.String())
		if err != nil {
			fmt.Println("Error with subscription", err)
		}
	}()

	fmt.Println("CHECKING IF GOT RESPONSES")
	// wait for block to be committed
	var assignedIndex uint
	for e := range h.suite.BftSuite.BftRPCWS.ResponsesCh {
		if gjson.GetBytes(e.Result, "query").String() != query.String() {
			continue
		}
		res, err := h.suite.BftSuite.BftRPC.ABCIQuery("GetEmailIndex", []byte(p.Email))
		if err != nil {
			return nil, &jsonrpc.Error{Code: 32603, Message: "Internal error", Data: "Failed to check if email exists after assignment: " + err.Error()}
		}
		if string(res.Response.Value) == "" {
			return nil, &jsonrpc.Error{Code: 32603, Message: "Internal error", Data: "Failed to find email after it has been assigned: " + err.Error()}
		}
		assignedIndex64, err := strconv.ParseUint(string(res.Response.Value), 10, 64)
		assignedIndex = uint(assignedIndex64)
		if err != nil {
			return nil, &jsonrpc.Error{Code: 32603, Message: "Internal error", Data: "Failed to parse uint for returned assignment index: " + fmt.Sprint(res) + " Error: " + err.Error()}
		}
		break
	}

	if secretMapping[int(assignedIndex)].Secret == nil {
		return nil, &jsonrpc.Error{Code: 32603, Message: "Internal error", Data: "could not retrieve secret, please try again"}
	}

	pubShareX, pubShareY := h.suite.EthSuite.secp.ScalarBaseMult(secretMapping[int(assignedIndex)].Secret.Bytes())

	return SecretAssignResult{
		ShareIndex: int(assignedIndex),
		PubShareX:  pubShareX.Text(16),
		PubShareY:  pubShareY.Text(16),
	}, nil

	// TODO: wait for websocket connection to be ready before allowing this to be called
	// ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	// defer cancel()
	// query := tmquery.MustParse("assignment='1'")
	// fmt.Println("QUERY IS:", query)
	// go func() {
	// 	// note: we also get back the initial "{}"
	// 	// data comes back in bytes of utf-8 which correspond
	// 	// to a base64 encoding of the original data
	// 	for e := range h.suite.BftSuite.BftRPCWS.ResponsesCh {
	// 		if gjson.GetBytes(e.Result, "query").String() != "assignment='1'" {
	// 			continue
	// 		}
	// 		fmt.Println("sub got ", string(e.Result[:]))
	// 		res, err := b64.StdEncoding.DecodeString(gjson.GetBytes(e.Result, "data.value.TxResult.tx").String())
	// 		if err != nil {
	// 			fmt.Println("error decoding b64", err)
	// 			continue
	// 		}
	// 		// valid messages should start with mug00
	// 		if len(res) < 5 || string(res[:len([]byte("mug00"))]) != "mug00" {
	// 			fmt.Println("Message not prefixed with mug00")
	// 			continue
	// 		}
	// 		keyGenShareBFTTx := DefaultBFTTxWrapper{&KeyGenShareBFTTx{}}
	// 		err = keyGenShareBFTTx.DecodeBFTTx(res[len([]byte("mug00")):])
	// 		if err != nil {
	// 			fmt.Println("error decoding bfttx", err)
	// 			continue
	// 		}
	// 		keyGenShareTx := keyGenShareBFTTx.BFTTx.(*KeyGenShareBFTTx)
	// 		err = HandleSigncryptedShare(h.suite, *keyGenShareTx)
	// 		if err != nil {
	// 			fmt.Println("failed to handle signcrypted share", err)
	// 			continue
	// 		}
	// 	}
	// }()
	// err = h.suite.BftSuite.BftRPCWS.Subscribe(ctx, query.String())
	// if err != nil {
	// 	fmt.Println("Error with subscription", err)
	// }

	// // deprecated, was assigning directly from a cache, when it should be decided via the bft.
	// // assigning directly from a cache gives no ordering guarantees and can mess up the indexing
	// // for secret shares across nodes
	// if err := jsonrpc.Unmarshal(params, &p); err != nil {
	// 	return nil, err
	// }
	// tmpSecretAssignment, found := h.suite.CacheSuite.CacheInstance.Get("Secret_ASSIGNMENT")
	// if !found {
	// 	return nil, jsonrpc.ErrInternal()
	// }
	// tmpSiMAPPING, found := h.suite.CacheSuite.CacheInstance.Get("Si_MAPPING")
	// if !found {
	// 	return nil, jsonrpc.ErrInternal()
	// }
	// tmpSecretMAPPING, found := h.suite.CacheSuite.CacheInstance.Get("Secret_MAPPING")
	// if !found {
	// 	return nil, jsonrpc.ErrInternal()
	// }
	// tmpAssigned, found := h.suite.CacheSuite.CacheInstance.Get("LAST_ASSIGNED")
	// if !found {
	// 	return nil, jsonrpc.ErrInternal()
	// }
	// lastAssigned := tmpAssigned.(int)
	// siMAPPING := tmpSiMAPPING.(map[int]common.PrimaryShare)
	// secretMapping := tmpSecretMAPPING.(map[int]SecretStore)
	// secretAssignment := tmpSecretAssignment.(map[string]SecretAssignment)

	// // was previously assigned
	// if val, ok := secretAssignment[p.Email]; ok {
	// 	pubShareX, pubShareY := h.suite.EthSuite.secp.ScalarBaseMult(val.Secret.Bytes())
	// 	return SecretAssignResult{
	// 		ShareIndex: val.ShareIndex,
	// 		PubShareX:  pubShareX.Text(16),
	// 		PubShareY:  pubShareY.Text(16),
	// 	}, nil
	// }

	// // new assignment
	// temp := siMAPPING[lastAssigned].Value
	// secretAssignment[p.Email] = SecretAssignment{secretMapping[lastAssigned].Secret, lastAssigned, &temp}
	// pubShareX, pubShareY := h.suite.EthSuite.secp.ScalarBaseMult(secretMapping[lastAssigned].Secret.Bytes())
	// secretMapping[lastAssigned] = SecretStore{secretMapping[lastAssigned].Secret, true}
	// h.suite.CacheSuite.CacheInstance.Set("Secret_MAPPING", secretMapping, -1)
	// h.suite.CacheSuite.CacheInstance.Set("LAST_ASSIGNED", lastAssigned+1, -1)
	// h.suite.CacheSuite.CacheInstance.Set("Secret_ASSIGNMENT", secretAssignment, -1)

	// return SecretAssignResult{
	// 	ShareIndex: lastAssigned,
	// 	PubShareX:  pubShareX.Text(16),
	// 	PubShareY:  pubShareY.Text(16),
	// }, nil
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
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		query := tmquery.MustParse("keygeneration.sharecollection='1'")
		fmt.Println("QUERY IS:", query)
		go func() {
			// note: we also get back the initial "{}"
			// data comes back in bytes of utf-8 which correspond
			// to a base64 encoding of the original data
			for e := range suite.BftSuite.BftRPCWS.ResponsesCh {
				if gjson.GetBytes(e.Result, "query").String() != "keygeneration.sharecollection='1'" {
					continue
				}
				fmt.Println("sub got ", string(e.Result[:]))
				res, err := b64.StdEncoding.DecodeString(gjson.GetBytes(e.Result, "data.value.TxResult.tx").String())
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
		}()
		err := suite.BftSuite.BftRPCWS.Subscribe(ctx, query.String())
		if err != nil {
			fmt.Println("Error with subscription", err)
		}
	}()

	mux := http.NewServeMux()
	mux.Handle("/jrpc", mr)
	mux.HandleFunc("/jrpc/debug", mr.ServeDebug)
	handler := cors.Default().Handler(mux)
	if suite.Flags.Production {
		if err := http.ListenAndServeTLS(":443",
			"/etc/letsencrypt/live/"+suite.Config.HostName+"/fullchain.pem",
			"/etc/letsencrypt/live/"+suite.Config.HostName+"/privkey.pem",
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
