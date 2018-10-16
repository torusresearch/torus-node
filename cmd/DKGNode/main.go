package main

/* Al useful imports */
import (
	"crypto/ecdsa"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"strconv"
	"time"

	"github.com/micro/go-config"
	"github.com/micro/go-config/source/file"

	"github.com/ethereum/go-ethereum/common"
	ethCrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"

	nodelist "../../solidity/goContracts"
)

/* Information/Metadata about node */
type NodeInfo struct {
	NodeId     int    `json:"nodeId"`
	NodeIpAddr string `json:"nodeIpAddr"`
	Port       string `json:"port"`
}

/* A standard format for a Request/Response for adding node to cluster */
type AddToClusterMessage struct {
	Source  NodeInfo `json:"source"`
	Dest    NodeInfo `json:"dest"`
	Message string   `json:"message"`
}

type Config struct {
	MakeMasterOnError string `json:"makeMasterOnError"`
	ClusterIp         string `json:"clusterip"`
	MyPort            string `json:"myport"`
	EthConnection     string `json:"ethconnection"`
	EthPrivateKey     string `json:"ethprivatekey"`
	NodeListAddress   string `json:"nodelistaddress`
}

/* Just for pretty printing the node info */
func (node NodeInfo) String() string {
	return "NodeInfo:{ nodeId:" + strconv.Itoa(node.NodeId) + ", nodeIpAddr:" + node.NodeIpAddr + ", port:" + node.Port + " }"
}

/* Just for pretty printing Request/Response info */
func (req AddToClusterMessage) String() string {
	return "AddToClusterMessage:{\n  source:" + req.Source.String() + ",\n  dest: " + req.Dest.String() + ",\n  message:" + req.Message + " }"
}

/* Form public key using private key */
func publicKeyFromPrivateKey(privateKey string) string {
	privateKeyBytes, _ := hex.DecodeString(privateKey)
	P1x, P1y := ethCrypto.S256().ScalarBaseMult(privateKeyBytes)
	Phex := P1x.Text(16) + P1y.Text(16)
	Pbyte, _ := hex.DecodeString(Phex)
	pubKHex := hex.EncodeToString(ethCrypto.Keccak256(Pbyte))
	return "0x" + pubKHex[len(pubKHex)-40:]
}

/* The entry point for our System */
func main() {
	/* Parse the provided parameters on command line */
	// ethPrivateKey := flag.String("ethprivatekey", "af090175729a5437ff3fedb766a5c8dc8a4a783ed41a384b83cf4647f0c99824", "provide eth private key, defaults af090175729a5437ff3fedb766a5c8dc8a4a783ed41a384b83cf4647f0c99824")
	flag.Parse()

	/* Load Config */
	config.Load(file.NewSource(
		file.WithPath("../../node/config.json"),
	))

	// retrieve map[string]interface{}
	var conf Config
	config.Scan(&conf)

	/* Connect to Ethereum */
	client, err := ethclient.Dial(conf.EthConnection)
	if err != nil {
		fmt.Println("Could not connect to eth connection ", conf.EthConnection)
		log.Fatal(err)
	}

	privateKeyECDSA, err := ethCrypto.HexToECDSA(string(conf.EthPrivateKey))
	if err != nil {
		log.Fatal(err)
	}
	nodePublicKey := privateKeyECDSA.Public()
	nodePublicKeyECDSA, ok := nodePublicKey.(*ecdsa.PublicKey)
	if !ok {
		log.Fatal("error casting public key to ECDSA")
	}
	nodeAddress := ethCrypto.PubkeyToAddress(*nodePublicKeyECDSA)
	nodeListAddress := common.HexToAddress(conf.NodeListAddress)

	fmt.Println("We have an eth connection to ", conf.EthConnection)
	fmt.Println("Node Private Key: ", conf.EthPrivateKey)
	fmt.Println("Node Public Key: ", nodeAddress)

	/*Creating contract instances */
	nodeListInstance, err := nodelist.NewNodelist(nodeListAddress, client)
	if err != nil {
		log.Fatal(err)
	}

	/*Fetch Node List from contract address */
	list, err := nodeListInstance.ViewNodeList(nil)
	if err != nil {
		log.Fatal(err)
	}

	/* Register Node */
	nodeIp, err := findExternalIP()
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println("Node IP Address: " + nodeIp + ":" + string(conf.MyPort))
	// err = nodeListInstance.ListNode(nodeIp + ":" + string(conf.MyPort))
	// if err != nil {
	// 	fmt.Println(err)
	// }

	if len(list) != 0 {
		fmt.Println("Connecting to other nodes: ", len(list), " nodes....")

	} else {
		fmt.Println("No existing nodes to connect to")
	}

	/* Generate id for myself */
	// rand.Seed(time.Now().UTC().UnixNano())
	// myid := rand.Intn(99999999)

	// myIp, _ := net.InterfaceAddrs()
	// me := NodeInfo{NodeId: myid, NodeIpAddr: myIp[0].String(), Port: *myport}
	// dest := NodeInfo{NodeId: -1, NodeIpAddr: strings.Split(*clusterip, ":")[0], Port: strings.Split(*clusterip, ":")[1]}
	// fmt.Println("My details:", me.String())

	// /* Try to connect to the cluster, and send request to cluster if able to connect */
	// ableToConnect := connectToCluster(me, dest)

	/*
	 * Listen for other incoming requests form other nodes to join cluster
	 * Note: We are not doing anything fancy right now to make this node as master. Not yet!
	 */
	// 	if ableToConnect || (!ableToConnect && *makeMasterOnError) {
	// 		if *makeMasterOnError {
	// 			fmt.Println("Will start this node as master.")
	// 		}
	// 		listenOnPort(me)
	// 	} else {
	// 		fmt.Println("Quitting system. Set makeMasterOnError flag to make the node master.", myid)
	// 	}
}

// func setNodeOnList(ipAddress, privateKey, instance, client) {
// 	publicKey := privateKey.Public()
// 	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
// 	if !ok {
// 			log.Fatal("error casting public key to ECDSA")
// 	}

// 	fromAddress := crypto.PubkeyToAddress(*publicKeyECDSA)
// 	nonce, err := client.PendingNonceAt(context.Background(), fromAddress)
// 	if err != nil {
// 			log.Fatal(err)
// 	}

// 	gasPrice, err := client.SuggestGasPrice(context.Background())
// 	if err != nil {
// 			log.Fatal(err)
// 	}

// 	auth := bind.NewKeyedTransactor(privateKey)
// 	auth.Nonce = big.NewInt(int64(nonce))
// 	auth.Value = big.NewInt(0)     // in wei
// 	auth.GasLimit = uint64(300000) // in units
// 	auth.GasPrice = gasPrice

// 	address := common.HexToAddress("0x147B8eb97fD247D06C4006D269c90C1908Fb5D54")
// 	instance, err := store.NewStore(address, client)
// 	if err != nil {
// 			log.Fatal(err)
// 	}

// 	key := [32]byte{}
// 	value := [32]byte{}
// 	copy(key[:], []byte("foo"))
// 	copy(value[:], []byte("bar"))

// 	tx, err := instance.ListNode(auth, key, value)
// 	if err != nil {
// 			log.Fatal(err)
// 	}

// 	fmt.Printf("tx sent: %s", tx.Hash().Hex()) // tx sent: 0x8d490e535678e9a24360e955d75b27ad307bdfb97a1dca51d0f3035dcee3e870
// }

func findExternalIP() (string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 {
			continue // interface down
		}
		if iface.Flags&net.FlagLoopback != 0 {
			continue // loopback interface
		}
		addrs, err := iface.Addrs()
		if err != nil {
			return "", err
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil || ip.IsLoopback() {
				continue
			}
			ip = ip.To4()
			if ip == nil {
				continue // not an ipv4 address
			}
			return ip.String(), nil
		}
	}
	return "", errors.New("are you connected to the network?")
}

/*
 * This is a useful utility to format the json packet to send requests
 * This tiny block is sort of important else you will end up sending blank messages.
 */
func getAddToClusterMessage(source NodeInfo, dest NodeInfo, message string) AddToClusterMessage {
	return AddToClusterMessage{
		Source: NodeInfo{
			NodeId:     source.NodeId,
			NodeIpAddr: source.NodeIpAddr,
			Port:       source.Port,
		},
		Dest: NodeInfo{
			NodeId:     dest.NodeId,
			NodeIpAddr: dest.NodeIpAddr,
			Port:       dest.Port,
		},
		Message: message,
	}
}

func connectToCluster(me NodeInfo, dest NodeInfo) bool {
	/* connect to this socket details provided */
	connOut, err := net.DialTimeout("tcp", dest.NodeIpAddr+":"+dest.Port, time.Duration(10)*time.Second)
	if err != nil {
		if _, ok := err.(net.Error); ok {
			fmt.Println("Couldn't connect to cluster.", me.NodeId)
			return false
		}
	} else {
		fmt.Println("Connected to cluster. Sending message to node.")
		text := "Hi nody.. please add me to the cluster.."
		requestMessage := getAddToClusterMessage(me, dest, text)
		json.NewEncoder(connOut).Encode(&requestMessage)

		decoder := json.NewDecoder(connOut)
		var responseMessage AddToClusterMessage
		decoder.Decode(&responseMessage)
		fmt.Println("Got response:\n" + responseMessage.String())

		return true
	}
	return false
}

func listenOnPort(me NodeInfo) {
	/* Listen for incoming messages */
	ln, _ := net.Listen("tcp", fmt.Sprint(":"+me.Port))
	/* accept connection on port */
	/* not sure if looping infinetely on ln.Accept() is good idea */
	for {
		connIn, err := ln.Accept()
		if err != nil {
			if _, ok := err.(net.Error); ok {
				fmt.Println("Error received while listening.", me.NodeId)
			}
		} else {
			var requestMessage AddToClusterMessage
			json.NewDecoder(connIn).Decode(&requestMessage)
			fmt.Println("Got request:\n" + requestMessage.String())

			text := "HEYO YOYOYOYOYO"
			responseMessage := getAddToClusterMessage(me, requestMessage.Source, text)
			json.NewEncoder(connIn).Encode(&responseMessage)
			connIn.Close()
		}
	}
}
