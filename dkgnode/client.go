package dkgnode

/* Al useful imports */
import (
	"crypto/ecdsa"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/YZhenY/DKGNode/pvss"
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

// type SigncryptedOutput struct {
// 	NodePubKey       Point
// 	NodeIndex        int
// 	SigncryptedShare Signcryption
// }
// type Signcryption struct {
// 	Ciphertext []byte
// 	R          Point
// 	Signature  big.Int
// }

type SigncryptedMessage struct {
	FromAddress string `json:fromaddress`
	FromPubKeyX string `json:frompubkeyx`
	FromPubKeyY string `json:frompubkeyy`
	Ciphertext  string `json:ciphertext`
	RX          string `json:rx`
	RY          string `json:ry`
	Signature   string `json:signature`
	ShareIndex  int    `json:shareindex`
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

func keyGenerationPhase(suite *Suite) {
	time.Sleep(1000 * time.Millisecond)
	nodeList := make([]*NodeReference, 99)
	siMapping := make(map[int]pvss.PrimaryShare)
	for {
		/*Fetch Node List from contract address */
		ethList, err := suite.EthSuite.NodeListInstance.ViewNodeList(nil)
		if err != nil {
			fmt.Println(err)
		}
		if len(ethList) > 0 {
			fmt.Println("Connecting to other nodes ------------------")
			triggerSecretSharing := 0
			for i := range ethList {
				if nodeList[i] == nil {
					nodeList[i], err = connectToJSONRPCNode(suite.EthSuite, ethList[i])
					if err != nil {
						fmt.Println(err)
					}
				} else {
					triggerSecretSharing++
				}
			}

			if triggerSecretSharing > 4 {
				nodes := make([]pvss.Point, triggerSecretSharing)

				for i := 0; i < triggerSecretSharing; i++ {
					nodes[i] = *ecdsaPttoPt(nodeList[i].PublicKey)
				}
				secret := pvss.RandomBigInt()
				signcryptedOut, _, err := pvss.CreateAndPrepareShares(nodes, *secret, 3, *suite.EthSuite.NodePrivateKey.D)
				if err != nil {
					fmt.Println(err)
				}
				//commit pubpoly
				// - publish on ethereum

				//send shares to nodes
				fmt.Println("Sending shares -----------")
				shareIndex := 0
				//TODO: CHANGE SHARE INDEX
				errArr := sendSharesToNodes(*suite.EthSuite, signcryptedOut, nodeList, shareIndex)
				if errArr != nil {
					fmt.Println("errors sending shares")
					fmt.Println(errArr)
				}
				//decrypt done in server.js

				time.Sleep(3000 * time.Millisecond) //TODO: Remove and handle errors
				//gather shares, decrypt and verify with pubpoly
				// - check if shares are here
				unsigncryptedShares := make([]*big.Int, 0)
				for i := 0; i < 5; i++ {
					data, found := suite.CacheSuite.CacheInstance.Get(nodeList[i].Address.Hex() + "_MAPPING")
					if found {
						var shareMapping = data.(map[int]ShareLog)
						if val, ok := shareMapping[shareIndex]; ok {
							unsigncryptedShares = append(unsigncryptedShares, new(big.Int).SetBytes(val.UnsigncryptedShare))
						}
					}
				}
				//- TODO:need to verify

				//form Si
				tempSi := new(big.Int)
				for i := range unsigncryptedShares {
					tempSi.Add(tempSi, unsigncryptedShares[i])
				}
				tempSi.Mod(tempSi, pvss.GeneratorOrder)
				var nodeIndex int
				for i := range unsigncryptedShares {
					if nodeList[i].Address.Hex() == suite.EthSuite.NodeAddress.Hex() {
						nodeIndex = int(nodeList[i].Index.Int64())
					}
				}
				si := pvss.PrimaryShare{nodeIndex, *tempSi}
				fmt.Println("STORED Si: ", shareIndex)
				siMapping[shareIndex] = si
			}
		} else {
			fmt.Println("No nodes in list/could not get from eth")
		}
		time.Sleep(5000 * time.Millisecond)
	}
}

func sendSharesToNodes(ethSuite EthSuite, signcryptedOutput []*pvss.SigncryptedOutput, nodeList []*NodeReference, shareIndex int) *[]error {
	errorSlice := make([]error, len(signcryptedOutput))
	// fmt.Println("GIVEN SIGNCRYPTION")
	// fmt.Println(signcryptedOutput[0].SigncryptedShare.Ciphertext)
	for i := range signcryptedOutput {
		//sanity checks
		if signcryptedOutput[i].NodePubKey.X.Cmp(nodeList[i].PublicKey.X) == 0 {
			_, err := nodeList[i].JSONClient.Call("KeyGeneration.ShareCollection", &SigncryptedMessage{
				ethSuite.NodeAddress.Hex(),
				ethSuite.NodePublicKey.X.Text(16),
				ethSuite.NodePublicKey.Y.Text(16),
				hex.EncodeToString(signcryptedOutput[i].SigncryptedShare.Ciphertext),
				signcryptedOutput[i].SigncryptedShare.R.X.Text(16),
				signcryptedOutput[i].SigncryptedShare.R.Y.Text(16),
				signcryptedOutput[i].SigncryptedShare.Signature.Text(16),
				shareIndex,
			})
			if err != nil {
				errorSlice = append(errorSlice, err)
			}

		} else {
			errorSlice = append(errorSlice, errors.New("signcryption and node list does not match at "+string(i)))
		}
	}
	if errorSlice[0] == nil {
		return nil
	}
	return &errorSlice
}

func ecdsaPttoPt(ecdsaPt *ecdsa.PublicKey) *pvss.Point {
	return &pvss.Point{*ecdsaPt.X, *ecdsaPt.Y}
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
