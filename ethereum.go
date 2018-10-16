package main

/* Al useful imports */
import (
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"
	"log"

	"github.com/ethereum/go-ethereum/common"
	ethCrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"

	nodelist "./solidity/goContracts"
)

/* Form public key using private key */
func publicKeyFromPrivateKey(privateKey string) string {
	privateKeyBytes, _ := hex.DecodeString(privateKey)
	P1x, P1y := ethCrypto.S256().ScalarBaseMult(privateKeyBytes)
	Phex := P1x.Text(16) + P1y.Text(16)
	Pbyte, _ := hex.DecodeString(Phex)
	pubKHex := hex.EncodeToString(ethCrypto.Keccak256(Pbyte))
	return "0x" + pubKHex[len(pubKHex)-40:]
}

func setUpEth(conf Config) []common.Address {
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
	fmt.Println("Node Public Key: ", nodeAddress.Hex())

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

	return list
}
