package dkgnode

import (
	"math/big"

	"github.com/looplab/fsm"

	"github.com/torusresearch/torus-public/common"
)

type KEYGENSend struct {
	KeyIndex big.Int
	AIY      common.PrimaryPolynomial
	AIprimeY common.PrimaryPolynomial
	BIX      common.PrimaryPolynomial
	BIprimeX common.PrimaryPolynomial
}

type KEYGENEcho struct {
	KeyIndex big.Int
	Aij      big.Int
	Aprimeij big.Int
	Bij      big.Int
	Bprimeij big.Int
}
type KEYGENReady struct {
	KeyIndex big.Int
	Aij      big.Int
	Aprimeij big.Int
	Bij      big.Int
	Bprimeij big.Int
}

// KeyIndex => NodeIndex => KEYGENLog
type KEYGENLog struct {
	KeyIndex               big.Int
	NodeIndex              big.Int
	C                      [][]common.Point // big.Int (in hex) to Commitment matrix
	AIY                    common.PrimaryPolynomial
	AIprimeY               common.PrimaryPolynomial
	BIX                    common.PrimaryPolynomial
	BIprimeX               common.PrimaryPolynomial
	ReceivedEchoes         map[string]KEYGENEcho          // From(M) big.Int (in hex) to Echo
	ReceivedReadys         map[string]KEYGENReady         // From(M) big.Int (in hex) to Ready
	ReceivedShareCompletes map[string]KEYGENShareComplete // From(M) big.Int (in hex) to ShareComplete
}

type KEYGENShareComplete struct {
	KeyIndex big.Int
	c        big.Int
	u1       big.Int
	u2       big.Int
	gsi      common.Point
	gsihr    common.Point
}

type AVSSKeygen interface {
	// Trigger Start for Keygen and Initialize
	InitiateKeygen(startingIndex big.Int, endingIndex big.Int, nodeIndexes []big.Int, threshold int) error


	//Implementing the Code below will allow KEYGEN to run
	// "Client" Actions
	broadcastInitiateKeygen(commitmentMatrixes [][][]common.Point, nodeIndex big.Int) error
	sendKEYGENSend(msg KEYGENSend, nodeIndex big.Int) error
	sendKEYGENEcho(msg KEYGENEcho, nodeIndex big.Int) error
	sendKEYGENReady(msg KEYGENReady, nodeIndex big.Int) error
	broadcastKEYGENShareComplete(msg KEYGENShareComplete, nodeIndex big.Int) error

	// For this, these listeners must be triggered on incoming messages
	// Listeners and Reactions
	onInitiateKeygen(commitmentMatrixes [][][]common.Point, nodeIndex big.Int) error
	onKEYGENSend(msg KEYGENSend, fromNodeIndex big.Int) error
	onKEYGENEcho(msg KEYGENEcho, fromNodeIndex big.Int) error
	onKEYGENReady(msg KEYGENReady, fromNodeIndex big.Int) error
	onKEYGENShareComplete(msg KEYGENShareComplete, fromNodeIndex big.Int) error

	// Storage for Secrets/Shares/etc... go here
}

type KeygenInstance struct {
	State *fsm.FSM
}

(avssKeygen *AVSSKeygen) func InitiateKeygen(startingIndex big.Int, endingIndex big.Int, nodeIndexes []big.Int, threshold int) error {
	fsm.NewFSM
}