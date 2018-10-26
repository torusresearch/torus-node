package main

/* Al useful imports */
import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/intel-go/fastjson"
	"github.com/osamingo/jsonrpc"
)

type (
	PingHandler struct {
		ethSuite EthSuite
	}
	PingParams struct {
		Message string `json:"message"`
	}
	PingResult struct {
		Message string `json:"message"`
	}
)

func (h PingHandler) ServeJSONRPC(c context.Context, params *fastjson.RawMessage) (interface{}, *jsonrpc.Error) {
	fmt.Println("Ping called")

	var p PingParams
	if err := jsonrpc.Unmarshal(params, &p); err != nil {
		return nil, err
	}

	return PingResult{
		Message: "Hello, " + p.Message + "I am " + h.ethSuite.NodeAddress.Hex(),
	}, nil
}

func setUpServer(ethSuite EthSuite, port string) {
	mr := jsonrpc.NewMethodRepository()
	if err := mr.RegisterMethod("Ping", PingHandler{ethSuite}, PingParams{}, PingResult{}); err != nil {
		log.Fatalln(err)
	}

	http.Handle("/jrpc", mr)
	http.HandleFunc("/jrpc/debug", mr.ServeDebug)
	fmt.Println(port)
	if err := http.ListenAndServe(":"+port, http.DefaultServeMux); err != nil {
		log.Fatalln(err)
	}
}
