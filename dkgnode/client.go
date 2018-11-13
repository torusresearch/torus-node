package dkgnode

/* All useful imports */
import (
	"crypto/ecdsa"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math/big"
	"time"

	"github.com/YZhenY/DKGNode/pvss"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	jsonrpcclient "github.com/ybbus/jsonrpc"
)

const NumberOfShares = 1000

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

type SecretStore struct {
	Secret   *big.Int
	Assigned bool
}

type SecretAssignment struct {
	Secret     *big.Int
	ShareIndex int
	Share      *big.Int
}

type SigncryptedMessage struct {
	FromAddress string `json:"fromaddress"`
	FromPubKeyX string `json:"frompubkeyx"`
	FromPubKeyY string `json:"frompubkeyy"`
	Ciphertext  string `json:"ciphertext"`
	RX          string `json:"rx"`
	RY          string `json:"ry"`
	Signature   string `json:"signature"`
	ShareIndex  int    `json:"shareindex"`
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
	time.Sleep(1000 * time.Millisecond) // TODO: why sleep?
	nodeList := make([]*NodeReference, suite.Config.NumberOfNodes)
	siMapping := make(map[int]pvss.PrimaryShare)
	for {
		/*Fetch Node List from contract address */
		ethList, err := suite.EthSuite.NodeListInstance.ViewNodeList(nil)
		if err != nil {
			fmt.Println(err)
		}
		if len(ethList) > 0 {
			fmt.Println("Connecting to other nodes ------------------")
			// fmt.Println("ETH LIST: ")
			//Build count of nodes connected to
			triggerSecretSharing := 0
			for i := range ethList {
				// fmt.Println(ethList[i].Hex())

				// Check if node is online
				temp, err := connectToJSONRPCNode(suite, ethList[i])
				if err != nil {
					fmt.Println(err)
				}
				// fmt.Println("ERROR HERE", int(temp.Index.Int64()))
				if temp != nil {
					if nodeList[int(temp.Index.Int64())-1] == nil {
						nodeList[int(temp.Index.Int64())-1] = temp
					} else {
						triggerSecretSharing++
					}
				}
			}

			if triggerSecretSharing > suite.Config.NumberOfNodes {
				log.Fatal("There are more nodes in the eth node list than the required number of nodes...")
			}

			// if we have connected to all nodes
			if triggerSecretSharing == suite.Config.NumberOfNodes {
				fmt.Println("Required number of nodes reached")
				fmt.Println("Sending shares -----------")
				numberOfShares := NumberOfShares
				secretMapping := make(map[int]SecretStore)
				for shareIndex := 0; shareIndex < numberOfShares; shareIndex++ {
					nodes := make([]pvss.Point, suite.Config.NumberOfNodes)

					for i := 0; i < triggerSecretSharing; i++ {
						nodes[i] = *ecdsaPttoPt(nodeList[i].PublicKey)
					}
					secret := pvss.RandomBigInt()
					// fmt.Println("Node "+suite.EthSuite.NodeAddress.Hex(), " Secret: ", secret.Text(16))
					signcryptedOut, pubpoly, err := pvss.CreateAndPrepareShares(nodes, *secret, suite.Config.Threshold, *suite.EthSuite.NodePrivateKey.D)
					if err != nil {
						fmt.Println(err)
					}
					// commit pubpoly

					// get hash of pubpoly
					// convert array of points to bytes array
					arrBytes := []byte{}
					for _, item := range *pubpoly {
						var num []byte
						num = abi.U256(&item.X)
						arrBytes = append(arrBytes, num...)
						num = abi.U256(&item.Y)
						arrBytes = append(arrBytes, num...)
					}
					// ecdsaSignature := ECDSASign(arrBytes, suite.EthSuite.NodePrivateKey) // TODO: check if it matches on-chain implementation
					// broadcast somewhere
					// jsonData, err := json.Marshal(ecdsaSignature)

					// send shares to nodes
					// TODO: CHANGE SHARE INDEX
					errArr := sendSharesToNodes(*suite.EthSuite, signcryptedOut, nodeList, shareIndex)
					if errArr != nil {
						fmt.Println("errors sending shares")
						fmt.Println(errArr)
					}
					secretMapping[shareIndex] = SecretStore{secret, false}
				}
				// decrypt done in server.js

				time.Sleep(8000 * time.Millisecond) // TODO: Check for communication termination from all other nodes
				// gather shares, decrypt and verify with pubpoly
				// - check if shares are here
				for shareIndex := 0; shareIndex < numberOfShares; shareIndex++ {
					unsigncryptedShares := make([]*big.Int, 0)
					for i := 0; i < suite.Config.NumberOfNodes; i++ {
						data, found := suite.CacheSuite.CacheInstance.Get(nodeList[i].Address.Hex() + "_MAPPING")
						if found {
							var shareMapping = data.(map[int]ShareLog)
							if val, ok := shareMapping[shareIndex]; ok {
								// fmt.Println("DRAWING SHARE FROM CACHE | ", suite.EthSuite.NodeAddress.Hex(), "=>", nodeList[i].Address.Hex())
								// fmt.Println(val.UnsigncryptedShare)
								unsigncryptedShares = append(unsigncryptedShares, new(big.Int).SetBytes(val.UnsigncryptedShare))
							}
						}
					}
					// TODO:need to verify

					// form Si
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
					si := pvss.PrimaryShare{Index: nodeIndex, Value: *tempSi}
					fmt.Println("STORED Si: ", shareIndex)
					siMapping[shareIndex] = si
				}
				suite.CacheSuite.CacheInstance.Set("Si_MAPPING", siMapping, -1)
				suite.CacheSuite.CacheInstance.Set("Secret_MAPPING", secretMapping, -1)
				//save cache
				cacheItems := suite.CacheSuite.CacheInstance.Items()
				cacheJSON, err := json.Marshal(cacheItems)
				if err != nil {
					fmt.Println(err)
				}
				err = ioutil.WriteFile("cache.json", cacheJSON, 0644)
				if err != nil {
					fmt.Println(err)
				}
				break
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
		for j := range signcryptedOutput {
			// sanity checks
			if signcryptedOutput[i].NodePubKey.X.Cmp(nodeList[j].PublicKey.X) == 0 {
				_, err := nodeList[j].JSONClient.Call("KeyGeneration.ShareCollection", &SigncryptedMessage{
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
			}
		}
	}
	if errorSlice[0] == nil {
		return nil
	}
	return &errorSlice
}

func ecdsaPttoPt(ecdsaPt *ecdsa.PublicKey) *pvss.Point {
	return &pvss.Point{X: *ecdsaPt.X, Y: *ecdsaPt.Y}
}

func connectToJSONRPCNode(suite *Suite, nodeAddress common.Address) (*NodeReference, error) {
	details, err := suite.EthSuite.NodeListInstance.NodeDetails(nil, nodeAddress)
	if err != nil {
		return nil, err
	}

	// if in production use https
	var nodeIPAddress string
	if suite.Flags.Production {
		nodeIPAddress = "https://" + details.DeclaredIp + "/jrpc"
	} else {
		nodeIPAddress = "http://" + details.DeclaredIp + "/jrpc"
	}
	rpcClient := jsonrpcclient.NewClient(nodeIPAddress)

	// TODO: possible replace with signature?
	_, err = rpcClient.Call("Ping", &Message{suite.EthSuite.NodeAddress.Hex()})
	if err != nil {
		return nil, err
	}
	return &NodeReference{Address: &nodeAddress, JSONClient: rpcClient, Index: details.Position, PublicKey: &ecdsa.PublicKey{Curve: suite.EthSuite.secp, X: details.PubKx, Y: details.PubKy}}, nil
}
