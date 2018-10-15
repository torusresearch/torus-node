package main

/* Al useful imports */
import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"strconv"
	"time"

	"encoding/hex"

	"github.com/micro/go-config"
	"github.com/micro/go-config/source/file"

	ethCrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
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
	// makeMasterOnError := flag.Bool("makeMasterOnError", false, "make this node master if unable to connect to the cluster ip provided.")
	// clusterip := flag.String("clusterip", "127.0.0.1:8001", "ip address of any node to connnect")
	// myport := flag.String("myport", "8001", "ip address to run this node on. default is 8001.")
	// ethConnection := flag.String("ethconnection", "https://mainnet.infura.io", "defaults to an infura connection at https://mainnet.infura.io")
	// ethPrivateKey := flag.String("ethprivatekey", "af090175729a5437ff3fedb766a5c8dc8a4a783ed41a384b83cf4647f0c99824", "provide eth private key, defaults af090175729a5437ff3fedb766a5c8dc8a4a783ed41a384b83cf4647f0c99824")
	flag.Parse()

	/* Load Config */
	config.Load(file.NewSource(
		file.WithPath("./config.json"),
	))

	// retrieve map[string]interface{}
	var conf Config
	config.Scan(&conf)

	/* Connect to Ethereum */
	_, err := ethclient.Dial(conf.EthConnection)
	if err != nil {
		fmt.Println("Could not connect to eth connection ", conf.EthConnection)
		log.Fatal(err)
	}
	ethPublicKey := publicKeyFromPrivateKey(string(conf.EthPrivateKey))

	fmt.Println("We have a eth connection to ", conf.EthConnection)
	fmt.Println("Node Private Key: ", conf.EthPrivateKey)
	fmt.Println("Node Public Key: ", ethPublicKey)

	/*Fetch Node List from contract address */

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
