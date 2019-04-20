package dkgnode

/* All useful imports */
// import (
// 	"crypto/ecdsa"
// 	"encoding/hex"
// 	"encoding/json"
// 	"fmt"
// 	"io/ioutil"

// 	"strings"
// 	"time"

// 	"github.com/Rican7/retry"
// 	"github.com/Rican7/retry/backoff"
// 	"github.com/Rican7/retry/strategy"
// 	tmconfig "github.com/tendermint/tendermint/config"
// 	tmsecp "github.com/tendermint/tendermint/crypto/secp256k1"
// 	tmnode "github.com/tendermint/tendermint/node"
// 	"github.com/tendermint/tendermint/p2p"
// 	"github.com/tendermint/tendermint/privval"
// 	tmtypes "github.com/tendermint/tendermint/types"

// 	"github.com/torusresearch/torus-public/keygen"
// 	"github.com/torusresearch/torus-public/logging"
// 	"github.com/torusresearch/torus-public/pvss"
// 	"github.com/torusresearch/torus-public/telemetry"
// )

import (
	// "fmt"
	// "io/ioutil"
	// "log"
	"math/big"
	// uuid "github.com/google/uuid"
	// inet "github.com/libp2p/go-libp2p-net"
	// peer "github.com/libp2p/go-libp2p-peer"
	"github.com/torusresearch/torus-public/common"
	"github.com/torusresearch/torus-public/keygen"
	"github.com/torusresearch/torus-public/logging"
)

// pattern: /protocol-name/request-or-response-message/starting-endingindex
const KEYGENRequestPrefix = "/KEYGEN/KEYGENreq/"
const KEYGENResponsePrefix = "/KEYGEN/KEYGENresp/"

type keygenID string // "startingIndex-endingIndex"
func getKeygenID(shareStartingIndex int, shareEndingIndex int) keygenID {
	return keygenID(string(shareStartingIndex) + "-" + string(shareEndingIndex))
}

// KEYGENProtocol type
type KEYGENProtocol struct {
	localHost       *P2PSuite // local host
	KeygenInstances map[keygenID]*keygen.KeygenInstance
	requests        map[string]*P2PBasicMsg // used to access request data from response handlers
}

// type AVSSKeygenTransport interface {
// 	// Implementing the Code below will allow KEYGEN to run
// 	// "Client" Actions
// 	BroadcastInitiateKeygen(commitmentMatrixes [][][]common.Point) error
// 	SendKEYGENSend(msg KEYGENSend, nodeIndex big.Int) error
// 	SendKEYGENEcho(msg KEYGENEcho, nodeIndex big.Int) error
// 	SendKEYGENReady(msg KEYGENReady, nodeIndex big.Int) error
// 	BroadcastKEYGENDKGComplete(msg KEYGENDKGComplete) error
// }

func NewKeygenProtocol(localHost *P2PSuite) *KEYGENProtocol {
	k := &KEYGENProtocol{
		localHost:       localHost,
		KeygenInstances: make(map[keygenID]*keygen.KeygenInstance),
		requests:        make(map[string]*P2PBasicMsg)}
	return k
}

func (kp *KEYGENProtocol) NewKeygen(suite *Suite, shareStartingIndex int, shareEndingIndex int) error {
	logging.Debugf("Keygen started from %v to  %v", shareStartingIndex, shareEndingIndex)

	keygenID := getKeygenID(shareStartingIndex, shareEndingIndex)

	// set up our keygen instance
	nodeIndexList := make([]big.Int, len(suite.EthSuite.NodeList))
	var ownNodeIndex big.Int
	for i, nodeRef := range suite.EthSuite.NodeList {
		nodeIndexList[i] = *nodeRef.Index
		if nodeRef.Address.String() == suite.EthSuite.NodeAddress.String() {
			ownNodeIndex = *nodeRef.Index
		}
	}
	keygenTp := KEYGENTransport{}
	keygenAuth := KEYGENAuth{}
	c := make(chan string)
	instance, err := keygen.NewAVSSKeygen(
		*big.NewInt(int64(shareStartingIndex)),
		shareEndingIndex-shareStartingIndex,
		nodeIndexList,
		suite.Config.Threshold,
		suite.Config.NumMalNodes,
		ownNodeIndex,
		&keygenTp,
		suite.DBSuite.Instance,
		&keygenAuth,
		c,
	)
	if err != nil {
		return err
	}

	// peg it to the protocol
	kp.KeygenInstances[keygenID] = instance

	return nil
}

type KEYGENTransport struct {
}

func (kt *KEYGENTransport) BroadcastInitiateKeygen(commitmentMatrixes [][][]common.Point) error {
	return nil
}
func (kt *KEYGENTransport) SendKEYGENSend(msg keygen.KEYGENSend, nodeIndex big.Int) error {
	return nil
}
func (kt *KEYGENTransport) SendKEYGENEcho(msg keygen.KEYGENEcho, nodeIndex big.Int) error {
	return nil
}
func (kt *KEYGENTransport) SendKEYGENReady(msg keygen.KEYGENReady, nodeIndex big.Int) error {
	return nil
}
func (kt *KEYGENTransport) BroadcastKEYGENDKGComplete(msg keygen.KEYGENDKGComplete) error {
	return nil
}

type KEYGENAuth struct {
}

func (ka *KEYGENAuth) Sign(msg string) ([]byte, error) {
	return nil, nil
}
func (ka *KEYGENAuth) Verify(text string, nodeIndex big.Int, signature []byte) bool {
	return false
}
