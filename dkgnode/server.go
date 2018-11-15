package dkgnode

/* All useful imports */
import (
	"context"
	"encoding/hex"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"time"

	"github.com/YZhenY/DKGNode/pvss"
	"github.com/intel-go/fastjson"
	"github.com/osamingo/jsonrpc"
	"github.com/patrickmn/go-cache"
	"github.com/rs/cors"
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
		BroadcastId        int
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

func (h SigncryptedHandler) ServeJSONRPC(c context.Context, params *fastjson.RawMessage) (interface{}, *jsonrpc.Error) {

	var p SigncryptedMessage
	if err := jsonrpc.Unmarshal(params, &p); err != nil {
		return nil, err
	}

	tmpRx, parsed := new(big.Int).SetString(p.RX, 16)
	if parsed == false {
		return nil, jsonrpc.ErrParse()
	}
	tmpRy, parsed := new(big.Int).SetString(p.RY, 16)
	if parsed == false {
		return nil, jsonrpc.ErrParse()
	}
	tmpSig, parsed := new(big.Int).SetString(p.Signature, 16)
	if parsed == false {
		return nil, jsonrpc.ErrParse()
	}
	tmpPubKeyX, parsed := new(big.Int).SetString(p.FromPubKeyX, 16)
	if parsed == false {
		return nil, jsonrpc.ErrParse()
	}
	tmpPubKeyY, parsed := new(big.Int).SetString(p.FromPubKeyY, 16)
	if parsed == false {
		return nil, jsonrpc.ErrParse()
	}

	tmpCiphertext, err := hex.DecodeString(p.Ciphertext)
	if err != nil {
		return nil, jsonrpc.ErrParse()
	}

	signcryption := pvss.Signcryption{
		Ciphertext: tmpCiphertext,
		R:          pvss.Point{X: *tmpRx, Y: *tmpRy},
		Signature:  *tmpSig,
	}
	// fmt.Println("RECIEVED SIGNCRYPTION")
	// fmt.Println(signcryption)
	unsigncryptedData, err := pvss.UnsigncryptShare(signcryption, *h.suite.EthSuite.NodePrivateKey.D, pvss.Point{*tmpPubKeyX, *tmpPubKeyY})
	if err != nil {
		fmt.Println("Error unsigncrypting share")
		fmt.Println(err)
		return nil, jsonrpc.ErrInvalidParams()
	}

	// deserialize share and broadcastId from signcrypted data
	n := len(*unsigncryptedData)
	shareBytes := (*unsigncryptedData)[:n-2]
	broadcastId := int((new(big.Int).SetBytes((*unsigncryptedData)[n-2:])).Uint64())

	fmt.Println("Saved share from ", p.FromAddress)
	savedLog, found := h.suite.CacheSuite.CacheInstance.Get(p.FromAddress + "_LOG")
	newShareLog := ShareLog{time.Now().UTC(), 0, p.ShareIndex, shareBytes, broadcastId}
	// if not found, we create a new mapping
	if found {
		var tempLog = savedLog.([]ShareLog)
		// newShareLog = ShareLog{time.Now().UTC(), len(tempLog), p.ShareIndex, shareBytes, broadcastId}
		newShareLog.LogNumber = len(tempLog)
		tempLog = append(tempLog, newShareLog)
		h.suite.CacheSuite.CacheInstance.Set(p.FromAddress+"_LOG", tempLog, cache.NoExpiration)
	} else {
		newLog := make([]ShareLog, 1)
		newLog[0] = newShareLog
		h.suite.CacheSuite.CacheInstance.Set(p.FromAddress+"_LOG", newLog, cache.NoExpiration)
	}

	savedMapping, found := h.suite.CacheSuite.CacheInstance.Get(p.FromAddress + "_MAPPING")
	// if not found, we create a new mapping
	if found {
		var tmpMapping = savedMapping.(map[int]ShareLog)
		tmpMapping[p.ShareIndex] = newShareLog
		h.suite.CacheSuite.CacheInstance.Set(p.FromAddress+"_MAPPING", tmpMapping, cache.NoExpiration)
	} else {
		newMapping := make(map[int]ShareLog)
		newMapping[p.ShareIndex] = newShareLog
		// fmt.Println("CACHING SHARE FROM | ", h.suite.EthSuite.NodeAddress.Hex(), "=>", p.FromAddress)
		// fmt.Println(newShareLog)
		h.suite.CacheSuite.CacheInstance.Set(p.FromAddress+"_MAPPING", newMapping, cache.NoExpiration)
	}

	return PingResult{
		Message: h.suite.EthSuite.NodeAddress.Hex(),
	}, nil
}

//checks id for assignment and then teh auth token for verification
func (h ShareRequestHandler) ServeJSONRPC(c context.Context, params *fastjson.RawMessage) (interface{}, *jsonrpc.Error) {
	var p ShareRequestParams
	if err := jsonrpc.Unmarshal(params, &p); err != nil {
		return nil, err
	}
	tmpSi, found := h.suite.CacheSuite.CacheInstance.Get("Si_MAPPING")
	if !found {
		return nil, jsonrpc.ErrInternal()
	}
	siMapping := tmpSi.(map[int]pvss.PrimaryShare)
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
	} else {
		tmpSecretAssignment, found := h.suite.CacheSuite.CacheInstance.Get("Secret_ASSIGNMENT")
		if !found {
			return nil, jsonrpc.ErrInternal()
		}
		secretAssignment := tmpSecretAssignment.(map[string]SecretAssignment)

		//checking oAuth token
		if oAuthCorrect, _ := testOauth(p.IDToken, p.Email); !*oAuthCorrect {
			return nil, jsonrpc.ErrInvalidParams()
		}

		if val, ok := secretAssignment[p.Email]; ok {
			return ShareRequestResult{
				Index:    siMapping[p.Index].Index,
				HexShare: val.Share.Text(16),
			}, nil
		} else {
			return nil, jsonrpc.ErrInvalidParams()
		}
	}
}

func (h SecretAssignHandler) ServeJSONRPC(c context.Context, params *fastjson.RawMessage) (interface{}, *jsonrpc.Error) {

	var p SecretAssignParams
	if err := jsonrpc.Unmarshal(params, &p); err != nil {
		return nil, err
	}
	// fmt.Println("SecretAssign called from " + p.Message)
	tmpSecretAssignment, found := h.suite.CacheSuite.CacheInstance.Get("Secret_ASSIGNMENT")
	if !found {
		return nil, jsonrpc.ErrInternal()
	}
	tmpSiMAPPING, found := h.suite.CacheSuite.CacheInstance.Get("Si_MAPPING")
	if !found {
		return nil, jsonrpc.ErrInternal()
	}
	tmpSecretMAPPING, found := h.suite.CacheSuite.CacheInstance.Get("Secret_MAPPING")
	if !found {
		return nil, jsonrpc.ErrInternal()
	}
	tmpAssigned, found := h.suite.CacheSuite.CacheInstance.Get("LAST_ASSIGNED")
	if !found {
		return nil, jsonrpc.ErrInternal()
	}
	lastAssigned := tmpAssigned.(int)
	siMAPPING := tmpSiMAPPING.(map[int]pvss.PrimaryShare)
	secretMapping := tmpSecretMAPPING.(map[int]SecretStore)
	secretAssignment := tmpSecretAssignment.(map[string]SecretAssignment)
	if val, ok := secretAssignment[p.Email]; ok {
		pubShareX, pubShareY := h.suite.EthSuite.secp.ScalarBaseMult(val.Secret.Bytes())
		return SecretAssignResult{
			ShareIndex: val.ShareIndex,
			PubShareX:  pubShareX.Text(16),
			PubShareY:  pubShareY.Text(16),
		}, nil
	}
	temp := siMAPPING[lastAssigned].Value
	secretAssignment[p.Email] = SecretAssignment{secretMapping[lastAssigned].Secret, lastAssigned, &temp}
	pubShareX, pubShareY := h.suite.EthSuite.secp.ScalarBaseMult(secretMapping[lastAssigned].Secret.Bytes())
	secretMapping[lastAssigned] = SecretStore{secretMapping[lastAssigned].Secret, true}
	h.suite.CacheSuite.CacheInstance.Set("Secret_MAPPING", secretMapping, -1)
	h.suite.CacheSuite.CacheInstance.Set("LAST_ASSIGNED", lastAssigned+1, -1)
	h.suite.CacheSuite.CacheInstance.Set("Secret_ASSIGNMENT", secretAssignment, -1)

	return SecretAssignResult{
		ShareIndex: lastAssigned,
		PubShareX:  pubShareX.Text(16),
		PubShareY:  pubShareY.Text(16),
	}, nil
}

func setUpServer(suite *Suite, port string) {
	mr := jsonrpc.NewMethodRepository()
	if err := mr.RegisterMethod("Ping", PingHandler{suite.EthSuite}, PingParams{}, PingResult{}); err != nil {
		log.Fatalln(err)
	}
	if err := mr.RegisterMethod("KeyGeneration.ShareCollection", SigncryptedHandler{suite}, SigncryptedMessage{}, PingResult{}); err != nil {
		log.Fatalln(err)
	}
	if err := mr.RegisterMethod("ShareRequest", ShareRequestHandler{suite}, ShareRequestParams{}, ShareRequestResult{}); err != nil {
		log.Fatalln(err)
	}
	if err := mr.RegisterMethod("SecretAssign", SecretAssignHandler{suite}, SecretAssignParams{}, SecretAssignResult{}); err != nil {
		log.Fatalln(err)
	}

	mux := http.NewServeMux()
	mux.Handle("/jrpc", mr)
	mux.HandleFunc("/jrpc/debug", mr.ServeDebug)
	// fmt.Println(port)
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
		if err := http.ListenAndServe(":"+port, handler); err != nil {
			log.Fatalln(err)
		}
	}
}
