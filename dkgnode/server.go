package dkgnode

/* Al useful imports */
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
	}
	ShareRequestHandler struct {
		suite *Suite
	}
	ShareRequestParams struct {
		Index int    `json:"index"`
		Token string `json:"token"`
		Id    string `json:"id"`
	}
	ShareRequestResult struct {
		Index    int    `json:"index"`
		HexShare string `json:"hexshare"`
	}
	SecretAssignHandler struct {
		suite *Suite
	}
	SecretAssignParams struct {
		Id string `json:"id"`
	}
	SecretAssignResult struct {
		ShareIndex int `json:"id"`
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
		tmpCiphertext,
		pvss.Point{*tmpRx, *tmpRy},
		*tmpSig,
	}
	// fmt.Println("RECIEVED SIGNCRYPTION")
	// fmt.Println(signcryption)
	unsigncryptedShare, err := pvss.UnsigncryptShare(signcryption, *h.suite.EthSuite.NodePrivateKey.D, pvss.Point{*tmpPubKeyX, *tmpPubKeyY})
	if err != nil {
		fmt.Println("Error unsigncrypting share")
		fmt.Println(err)
		return nil, jsonrpc.ErrInvalidParams()
	}

	fmt.Println("Saved share from ", p.FromAddress)
	savedLog, found := h.suite.CacheSuite.CacheInstance.Get(p.FromAddress + "_LOG")
	newShareLog := ShareLog{time.Now().UTC(), 0, p.ShareIndex, *unsigncryptedShare}
	if found {
		var tempLog = savedLog.([]ShareLog)
		//TODO: possibly change to pointer
		newShareLog = ShareLog{time.Now().UTC(), len(tempLog), p.ShareIndex, *unsigncryptedShare}
		tempLog = append(tempLog, newShareLog)
		h.suite.CacheSuite.CacheInstance.Set(p.FromAddress+"_LOG", tempLog, cache.NoExpiration)
	} else {
		newLog := make([]ShareLog, 1)
		newLog[0] = newShareLog
		h.suite.CacheSuite.CacheInstance.Set(p.FromAddress+"_LOG", newLog, cache.NoExpiration)
	}

	savedMapping, found := h.suite.CacheSuite.CacheInstance.Get(p.FromAddress + "_MAPPING")
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
	if p.Token == "blublu" {
		tmpSi, found := h.suite.CacheSuite.CacheInstance.Get("Si_MAPPING")
		if !found {
			return nil, jsonrpc.ErrInternal()
		}
		siMapping := tmpSi.(map[int]pvss.PrimaryShare)
		if _, ok := siMapping[p.Index]; !ok {
			return nil, jsonrpc.ErrInvalidParams()
		}
		tmpInt := siMapping[p.Index].Value
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

		//TODO: check token here

		if val, ok := secretAssignment[p.Id]; ok {
			return ShareRequestResult{
				Index:    val.ShareIndex,
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
	if _, ok := secretAssignment[p.Id]; ok {
		return nil, jsonrpc.ErrInvalidRequest()
	}
	temp := siMAPPING[lastAssigned].Value
	secretAssignment[p.Id] = SecretAssignment{secretMapping[lastAssigned].Secret, lastAssigned, &temp}
	secretMapping[lastAssigned] = SecretStore{secretMapping[lastAssigned].Secret, true}
	h.suite.CacheSuite.CacheInstance.Set("Secret_MAPPING", secretMapping, -1)
	h.suite.CacheSuite.CacheInstance.Set("LAST_ASSIGNED", lastAssigned+1, -1)
	h.suite.CacheSuite.CacheInstance.Set("Secret_ASSIGNMENT", secretAssignment, -1)

	return SecretAssignResult{
		ShareIndex: lastAssigned,
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

	http.Handle("/jrpc", mr)
	http.HandleFunc("/jrpc/debug", mr.ServeDebug)
	fmt.Println(port)
	if err := http.ListenAndServe(":"+port, http.DefaultServeMux); err != nil {
		log.Fatalln(err)
	}
}
