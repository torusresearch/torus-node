package dkgnode

/* All useful imports */
import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"encoding/hex"
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	ethCrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/torusresearch/torus-public/logging"
	"github.com/torusresearch/torus-public/secp256k1"
	nodelist "github.com/torusresearch/torus-public/solidity/goContracts"
)

type EthSuite struct {
	NodePublicKey    *ecdsa.PublicKey
	NodeAddress      *common.Address
	NodePrivateKey   *ecdsa.PrivateKey
	Client           *ethclient.Client
	NodeListContract *nodelist.Nodelist
	secp             elliptic.Curve
	NodeList         []*NodeReference
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

func SetupEth(suite *Suite) error {
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

	logging.Infof("We have an eth connection to %s", suite.Config.EthConnection)
	logging.Infof("Node Private Key: %s", suite.Config.EthPrivateKey)
	logging.Infof("Node Public Key: %s", nodeAddress.Hex())

	/*Creating contract instances */
	NodeListContract, err := nodelist.NewNodelist(nodeListAddress, client)
	if err != nil {
		return err
	}
	suite.EthSuite = &EthSuite{nodePublicKeyEC, &nodeAddress, privateKeyECDSA, client, NodeListContract, secp256k1.Curve, []*NodeReference{}}
	return nil
}

func (suite EthSuite) registerNode(epoch big.Int, declaredIP string, TMP2PConnection string, P2PConnection string) (*types.Transaction, error) {
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

	tx, err := suite.NodeListContract.ListNode(auth, &epoch, declaredIP, suite.NodePublicKey.X, suite.NodePublicKey.Y, TMP2PConnection, P2PConnection)
	if err != nil {
		return nil, err
	}
	return tx, nil
}
