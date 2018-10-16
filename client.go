package main

/* Al useful imports */
import (
	"fmt"
	"net/rpc"
	"time"

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
	time.Sleep(1000 * time.Millisecond)
	for {
		rpcClient := jsonrpcclient.NewClient(nodeListStrings[0])

		response, err := rpcClient.Call("Main.Echo", &Person{"John"})
		if err != nil {
			fmt.Println("couldnt connect")
		}

		fmt.Println("response: ", response)
		fmt.Println(time.Now().UTC())
		time.Sleep(1000 * time.Millisecond)
	}
	// }
}
