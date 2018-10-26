package main

/* Al useful imports */
import (
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	jsonrpcclient "github.com/ybbus/jsonrpc"
)

type NodeReference struct {
	Address    *common.Address
	JSONClient jsonrpcclient.RPCClient
	Index      *big.Int
	PublicKey  *ecdsa.PublicKey
}

type Person struct {
	Name string `json:"name"`
}

type Message struct {
	Message string `json:"message"`
}

func setUpClient(nodeListStrings []string) {
	// nodeListStruct make(NodeReference[], 0)
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

func keyGenerationPhase(ethSuite *EthSuite) {
	time.Sleep(1000 * time.Millisecond)
	nodeList := make([]*NodeReference, 10)

	for {

		/*Fetch Node List from contract address */
		ethList, err := ethSuite.NodeListInstance.ViewNodeList(nil)
		if err != nil {
			fmt.Println(err)
		}
		if len(ethList) > 0 {
			fmt.Println("Connecting to other nodes")
			for i := range ethList {
				nodeList[i], err = connectToJSONRPCNode(ethSuite, ethList[i])
				if err != nil {
					fmt.Println(err)
				}
			}

		} else {
			fmt.Println("Not enough nodes in node ethList")
			fmt.Println(ethList)
		}
		time.Sleep(5000 * time.Millisecond)
	}
}

func connectToJSONRPCNode(ethSuite *EthSuite, nodeAddress common.Address) (*NodeReference, error) {
	details, err := ethSuite.NodeListInstance.NodeDetails(nil, nodeAddress)
	if err != nil {
		return nil, err
	}
	rpcClient := jsonrpcclient.NewClient("http://" + details.DeclaredIp + "/jrpc")

	//TODO: possibble replace with signature?
	_, err = rpcClient.Call("Ping", &Message{ethSuite.NodeAddress.Hex()})
	if err != nil {
		return nil, err
	}
	return &NodeReference{&nodeAddress, rpcClient, details.Position, &ecdsa.PublicKey{ethSuite.secp, details.PubKx, details.PubKy}}, nil
}
