package main

/* Al useful imports */
import (
	"context"
	"encoding/hex"
	"fmt"
	"log"
	"math/big"
	"net/http"

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
)

func (h PingHandler) ServeJSONRPC(c context.Context, params *fastjson.RawMessage) (interface{}, *jsonrpc.Error) {

	var p PingParams
	if err := jsonrpc.Unmarshal(params, &p); err != nil {
		return nil, err
	}
	fmt.Println("Ping called from " + p.Message)

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
	h.suite.CacheSuite.CacheInstance.Set(p.FromAddress, unsigncryptedShare, cache.NoExpiration)

	return PingResult{
		Message: h.suite.EthSuite.NodeAddress.Hex(),
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

	http.Handle("/jrpc", mr)
	http.HandleFunc("/jrpc/debug", mr.ServeDebug)
	fmt.Println(port)
	if err := http.ListenAndServe(":"+port, http.DefaultServeMux); err != nil {
		log.Fatalln(err)
	}
}
