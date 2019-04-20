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
// 	"github.com/torusresearch/torus-public/common"
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

func NewKeygen(suite *Suite, shareStartingIndex int, shareEndingIndex int, localHost *P2PSuite) *KEYGENProtocol {
	logging.Debugf("Keygen started from %v to  %v", shareStartingIndex, shareEndingIndex)
	k := &KEYGENProtocol{localHost: localHost, requests: make(map[string]*P2PBasicMsg)}
	keygenID := getKeygenID(shareStartingIndex, shareEndingIndex)
	
	// set up our keygen instance
	nodeIndexList := make([]big.Int, len(suite.EthSuite.NodeList))
	for i, nodeRef := range suite.EthSuite.NodeList {
		nodeIndexList[i] = *nodeRef.Index
	}
	instance := keygen.NewAVSSKeygen(
		big.NewInt(int64(shareStartingIndex)),
		big.NewInt(int64(shareEndingIndex - shareStartingIndex)),
		nodeIndexList.

		
		
	)
	return k
}
