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
	EchoHandler struct{}
	EchoParams  struct {
		Name string `json:"name"`
	}
	EchoResult struct {
		Message string `json:"message"`
	}
)

func (h EchoHandler) ServeJSONRPC(c context.Context, params *fastjson.RawMessage) (interface{}, *jsonrpc.Error) {
	fmt.Println("Echo called")

	var p EchoParams
	if err := jsonrpc.Unmarshal(params, &p); err != nil {
		return nil, err
	}

	return EchoResult{
		Message: "Hello, " + p.Name,
	}, nil
}

func setUpServer(port string) {
	mr := jsonrpc.NewMethodRepository()
	if err := mr.RegisterMethod("Main.Echo", EchoHandler{}, EchoParams{}, EchoResult{}); err != nil {
		log.Fatalln(err)
	}

	http.Handle("/jrpc", mr)
	http.HandleFunc("/jrpc/debug", mr.ServeDebug)
	fmt.Println(port)
	if err := http.ListenAndServe(":"+port, http.DefaultServeMux); err != nil {
		log.Fatalln(err)
	}
}
