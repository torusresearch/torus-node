package dkgnode

/* All useful imports */
import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"

	"github.com/YZhenY/torus/secp256k1"
	"github.com/YZhenY/torus/solidity/goContracts"
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
	P1x, P1y := secp256k1.Curve.ScalarBaseMult(privateKeyBytes)
	Phex := P1x.Text(16) + P1y.Text(16)
	Pbyte, _ := hex.DecodeString(Phex)
	pubKHex := hex.EncodeToString(secp256k1.Keccak256(Pbyte))
	return "0x" + pubKHex[len(pubKHex)-40:]
}

func SetUpEth(suite *Suite) error {
	/* Connect to Ethereum */
	client, err := ethclient.Dial(suite.Config.EthConnection)
	if err != nil {
		return err
	}

	privateKeyECDSA, err := ethCrypto.HexToECDSA(string(suite.Config.EthPrivateKey))
	if err != nil {
		return err
	}
	nodePublicKey := privateKeyECDSA.Public()
	nodePublicKeyEC, ok := nodePublicKey.(*ecdsa.PublicKey)
	if !ok {
		return errors.New("error casting to Public Key")
	}
	nodeAddress := ethCrypto.PubkeyToAddress(*nodePublicKeyEC)
	nodeListAddress := common.HexToAddress(suite.Config.NodeListAddress)

	fmt.Println("We have an eth connection to ", suite.Config.EthConnection)
	fmt.Println("Node Private Key: ", suite.Config.EthPrivateKey)
	fmt.Println("Node Public Key: ", nodeAddress.Hex())

	/*Creating contract instances */
	nodeListInstance, err := nodelist.NewNodelist(nodeListAddress, client)
	if err != nil {
		return err
	}
	suite.EthSuite = &EthSuite{nodePublicKeyEC, &nodeAddress, privateKeyECDSA, client, nodeListInstance, secp256k1.Curve}
	return nil
}

func (suite EthSuite) registerNode(declaredIP string, TMP2PConnection string) (*types.Transaction, error) {
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

	tx, err := suite.NodeListInstance.ListNode(auth, declaredIP, suite.NodePublicKey.X, suite.NodePublicKey.Y, TMP2PConnection)
	if err != nil {
		return nil, err
	}
	return tx, nil
}
