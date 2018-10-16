package main

/* Al useful imports */
import (
	"fmt"
	"log"
	"net/rpc"

	jsonrpcclient "github.com/ybbus/jsonrpc"
)

type NodeList struct {
	AddressHex string
	JsonClient rpc.Client
}

type Person struct {
	Name string `json:"name"`
}

func setUpClient(nodeListStrings []string) {
	// nodeListStruct make(NodeList[], 0)
	// for index, element := range nodeListStrings {
	rpcClient := jsonrpcclient.NewClient(nodeListStrings[0])

	response, err := rpcClient.Call("Main.Echo", &Person{"John"})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(response)
	// }
}
