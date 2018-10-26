package main

/* Al useful imports */
import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"

	"github.com/YZhenY/DKGNode/solidity/goContracts"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	ethCrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

type EthSuite struct {
	NodePublicKey    *ecdsa.PublicKey
	NodeAddress      *common.Address
	NodePrivateKey   *ecdsa.PrivateKey
	Client           *ethclient.Client
	NodeListInstance *nodelist.Nodelist
	secp             elliptic.Curve
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

func setUpEth(conf Config) (*EthSuite, error) {
	/* Connect to Ethereum */
	client, err := ethclient.Dial(conf.EthConnection)
	if err != nil {
		return nil, errors.New("Could not connect to eth connection " + conf.EthConnection)
	}

	privateKeyECDSA, err := ethCrypto.HexToECDSA(string(conf.EthPrivateKey))
	if err != nil {
		return nil, err
	}
	nodePublicKey := privateKeyECDSA.Public()
	nodePublicKeyEC, ok := nodePublicKey.(*ecdsa.PublicKey)
	if !ok {
		return nil, errors.New("error casting to Public Key")
	}
	nodeAddress := ethCrypto.PubkeyToAddress(*nodePublicKeyEC)
	nodeListAddress := common.HexToAddress(conf.NodeListAddress)

	fmt.Println("We have an eth connection to ", conf.EthConnection)
	fmt.Println("Node Private Key: ", conf.EthPrivateKey)
	fmt.Println("Node Public Key: ", nodeAddress.Hex())

	/*Creating contract instances */
	nodeListInstance, err := nodelist.NewNodelist(nodeListAddress, client)
	if err != nil {
		return nil, err
	}

	return &EthSuite{nodePublicKeyEC, &nodeAddress, privateKeyECDSA, client, nodeListInstance, ethCrypto.S256()}, nil
}

func (suite EthSuite) registerNode(declaredIP string) (*types.Transaction, error) {
	nonce, err := suite.Client.PendingNonceAt(context.Background(), ethCrypto.PubkeyToAddress(*suite.NodePublicKey))
	if err != nil {
		return nil, err
	}

	gasPrice, err := suite.Client.SuggestGasPrice(context.Background())
	if err != nil {
		return nil, err
	}

	auth := bind.NewKeyedTransactor(suite.NodePrivateKey)
	auth.Nonce = big.NewInt(int64(nonce))
	auth.Value = big.NewInt(0)      // in wei
	auth.GasLimit = uint64(4700000) // in units
	auth.GasPrice = gasPrice

	tx, err := suite.NodeListInstance.ListNode(auth, declaredIP, suite.NodePublicKey.X, suite.NodePublicKey.Y)
	if err != nil {
		return nil, err
	}
	return tx, nil
}
