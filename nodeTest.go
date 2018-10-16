package main

/* Al useful imports */
import (
	"crypto/ecdsa"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"

	"github.com/micro/go-config"
	"github.com/micro/go-config/source/file"

	"github.com/ethereum/go-ethereum/common"
	ethCrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"

	nodelist "./solidity/goContracts"
)

type Config struct {
	MakeMasterOnError string `json:"makeMasterOnError"`
	ClusterIp         string `json:"clusterip"`
	MyPort            string `json:"myport"`
	EthConnection     string `json:"ethconnection"`
	EthPrivateKey     string `json:"ethprivatekey"`
	NodeListAddress   string `json:"nodelistaddress`
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
		file.WithPath("./node/config.json"),
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

	// setUpServer(string(conf.MyPort))
	test := make([]string, 1)
	test[0] = "http://localhost:" + string(conf.MyPort) + "/jrpc"
	setUpClient(test)

}

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
