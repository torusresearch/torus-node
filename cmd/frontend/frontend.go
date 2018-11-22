package main

/* All useful imports */
import (
	"fmt"
	"math/big"

	"github.com/YZhenY/torus/common"
	"github.com/YZhenY/torus/pvss"
	"github.com/YZhenY/torus/solidity/goContracts"
	ethCommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	config "github.com/micro/go-config"
	"github.com/micro/go-config/source/file"
	jsonrpcclient "github.com/ybbus/jsonrpc"
)

type NodeReference struct {
	Address    *ethCommon.Address
	JSONClient jsonrpcclient.RPCClient
	Index      *big.Int
	PublicKey  *common.Point
}

type Person struct {
	Name string `json:"name"`
}

type (
	PingParams struct {
		Message string `json:"message"`
	}
	PingResult struct {
		Message string `json:"message"`
	}
	Message struct {
		Message string `json:"message"`
	}
	ShareRequestParams struct {
		Index int    `json:"index"`
		Token string `json:"idtoken"`
		Id    string `json:"email"`
	}
	ShareRequestResult struct {
		Index    int    `json:"index"`
		HexShare string `json:"hexshare"`
	}
)

type Config struct {
	EthConnection   string `json:"ethconnection"`
	EthPrivateKey   string `json:"ethprivatekey"`
	NodeListAddress string `json:"nodelistaddress"`
}

// func setUpClient(nodeListStrings []string) {
// 	// nodeListStruct make(NodeReference[], 0)
// 	// for index, element := range nodeListStrings {
// 	time.Sleep(1000 * time.Millisecond)
// 	for {
// 		rpcClient := jsonrpcclient.NewClient(nodeListStrings[0])

// 		response, err := rpcClient.Call("Main.Echo", &Person{"John"})
// 		if err != nil {
// 			fmt.Println("couldnt connect")
// 		}

// 		fmt.Println("response: ", response)
// 		fmt.Println(time.Now().UTC())
// 		time.Sleep(1000 * time.Millisecond)
// 	}
// 	// }
// }

func main() {

	authToken := "blublu"
	config := loadConfig("../config/config.json")

	/* Connect to Ethereum */
	client, err := ethclient.Dial(config.EthConnection)
	if err != nil {
		fmt.Println("Could not connect to eth connection " + config.EthConnection)
	}

	/*Creating contract instances */
	nodeListInstance, err := nodelist.NewNodelist(ethCommon.HexToAddress(config.NodeListAddress), client)
	if err != nil {
		fmt.Println(err)
	}

	list, err := nodeListInstance.ViewNodes(nil)
	if err != nil {
		fmt.Println(err)
	}

	nodeList := make([]*NodeReference, len(list.Nodes))
	for i := range list.Nodes {
		nodeList[i], err = connectToJSONRPCNode(nodeListInstance, list.Nodes[i])
		if err != nil {
			fmt.Println(err)
		}
	}

	correctCount := 0

	for shareIndex := 6; shareIndex < 7; shareIndex++ {
		//get shares
		shareList := make([]common.PrimaryShare, len(nodeList))
		for i := range nodeList {
			response, err := nodeList[i].JSONClient.Call("ShareRequest", &ShareRequestParams{shareIndex, authToken, "zheeen"})
			if err != nil {
				fmt.Println("ERROR CALLING")
				fmt.Println(err)
			}
			fmt.Println(response)
			var tmpShare ShareRequestResult
			err = response.GetObject(&tmpShare)
			if err != nil {
				fmt.Println("ERROR CASTING")
				fmt.Println(err)
			}
			shareVal, ok := new(big.Int).SetString(tmpShare.HexShare, 16)
			if !ok {
				fmt.Println("Couldnt parse hex share from ", nodeList[i].Address.Hex())
			}
			shareList[i] = common.PrimaryShare{Index: tmpShare.Index, Value: *shareVal}
		}

		// fmt.Println("FINAL PRIVATE KEY: ")
		temppp := make([]common.PrimaryShare, 1)
		temppp[0] = shareList[0]
		equal := true
		final := pvss.LagrangeScalar(append(append(temppp, shareList[1]), shareList[2])) // nodes: 0, 1, 2
		// fmt.Println("123: ", final.Text(16))
		testFinal := final
		final = pvss.LagrangeScalar(append(append(temppp, shareList[1]), shareList[3])) // nodes: 0, 1, 3
		// fmt.Println("124: ", final.Text(16))
		if testFinal.Cmp(final) != 0 {
			equal = false
		}
		final = pvss.LagrangeScalar(append(append(temppp, shareList[1]), shareList[4])) // nodes: 0, 1, 4
		// fmt.Println("125", final.Text(16))
		if testFinal.Cmp(final) != 0 {
			equal = false
		}
		final = pvss.LagrangeScalar(append(append(temppp, shareList[2]), shareList[3])) // nodes: 0, 2, 3
		// fmt.Println("134", final.Text(16))
		if testFinal.Cmp(final) != 0 {
			equal = false
		}
		final = pvss.LagrangeScalar(append(append(temppp, shareList[2]), shareList[4])) // nodes: 0, 2, 4
		// fmt.Println("135", final.Text(16))
		if testFinal.Cmp(final) != 0 {
			equal = false
		}
		final = pvss.LagrangeScalar(append(append(temppp, shareList[3]), shareList[4])) // nodes: 0, 3, 4
		// fmt.Println("145", final.Text(16))
		if testFinal.Cmp(final) != 0 {
			equal = false
		}
		temppp[0] = shareList[1]
		final = pvss.LagrangeScalar(append(append(temppp, shareList[2]), shareList[3])) // nodes: 1, 2, 3
		// fmt.Println("234", final.Text(16))
		if testFinal.Cmp(final) != 0 {
			equal = false
		}
		final = pvss.LagrangeScalar(append(append(temppp, shareList[2]), shareList[4])) // nodes: 1, 2, 4
		// fmt.Println("235", final.Text(16))
		if testFinal.Cmp(final) != 0 {
			equal = false
		}
		final = pvss.LagrangeScalar(append(append(temppp, shareList[3]), shareList[4])) // nodes: 1, 3, 4
		// fmt.Println("245", final.Text(16))
		if testFinal.Cmp(final) != 0 {
			equal = false
		}
		temppp[0] = shareList[2]
		final = pvss.LagrangeScalar(append(append(temppp, shareList[3]), shareList[4])) // nodes: 2, 3, 4
		fmt.Println("345", final.Text(16))
		if testFinal.Cmp(final) != 0 {
			equal = false
		}

		// fmt.Println("EQUAL :", equal)

		if equal {
			correctCount++
		}

	}
	fmt.Println("Correct Count: ", correctCount)

	// fmt.Println("TEST R")
	// testShares := new(big.Int)
	// val, _ := new(big.Int).SetString("32c8d23f805f0c7f224f30b5cd07574df574e0b96fc73db9906d1d2cb75a31a9", 16)
	// testShares.Add(testShares, val)
	// testShares.Mod(testShares, pvss.GeneratorOrder)
	// val, _ = new(big.Int).SetString("55731cd8c0f2c35a72b8e75d606c278d138f0a8b68f512d083f112f8a8c174ec", 16)
	// testShares.Add(testShares, val)
	// testShares.Mod(testShares, pvss.GeneratorOrder)
	// val, _ = new(big.Int).SetString("6e0a9234cc75faf707a696c633bc31b5af6d98ecd24828cd1bef86118d38b59b", 16)
	// testShares.Add(testShares, val)
	// testShares.Mod(testShares, pvss.GeneratorOrder)
	// val, _ = new(big.Int).SetString("b97ce77d7551e93afafdfc86699f01ebede565398a48ac09c132fdc1fc010735", 16)
	// testShares.Add(testShares, val)
	// testShares.Mod(testShares, pvss.GeneratorOrder)
	// val, _ = new(big.Int).SetString("729b1afacbd4d4673911c6ed5a85f6f92b68a5733471241eeca69be3c6cc0138", 16)
	// testShares.Add(testShares, val)
	// testShares.Mod(testShares, pvss.GeneratorOrder)
	// fmt.Println(testShares.Text(16))
}

func loadConfig(path string) *Config {
	/* Load Config */
	config.Load(file.NewSource(
		file.WithPath(path),
	))
	// retrieve map[string]interface{}
	var conf Config
	config.Scan(&conf)

	return &conf
}

func connectToJSONRPCNode(nodeListInstance *nodelist.Nodelist, nodeAddress ethCommon.Address) (*NodeReference, error) {
	details, err := nodeListInstance.NodeDetails(nil, nodeAddress)
	if err != nil {
		return nil, err
	}

	//if in production use https
	var nodeIPAddress string
	// nodeIPAddress = "https://" + details.DeclaredIp + "/jrpc"
	nodeIPAddress = "http://" + details.DeclaredIp + "/jrpc"

	rpcClient := jsonrpcclient.NewClient(nodeIPAddress)

	//TODO: possibble replace with signature?
	response, err := rpcClient.Call("Ping", &Message{"HEYO"})
	if err != nil {
		return nil, err
	}
	fmt.Println(response)

	return &NodeReference{
		Address:    &nodeAddress,
		JSONClient: rpcClient,
		Index:      details.Position,
		PublicKey: &common.Point{
			X: *details.PubKx,
			Y: *details.PubKy,
		},
	}, nil
}
