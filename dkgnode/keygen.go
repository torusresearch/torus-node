package dkgnode

import (
	"math/big"

	"github.com/torusresearch/torus-public/logging"

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

// fsm.NewFSM(
// 	"standby",
// 	fsm.Events{
// 		{Name: "all_initiate_keygen", Src: []string{"standby"}, Dst: "ready_for_keygen"},
// 		{Name: "start_keygen", Src: []string{"ready_for_keygen"}, Dst: "running_keygen"},
// 		{Name: "all_keygen_complete", Src: []string{"running_keygen"}, Dst: "verifying_shares"},
// 		{Name: "shares_verified", Src: []string{"verifying_shares"}, Dst: "standby"},
// 	},
// 	fsm.Callbacks{
// 		"enter_state": func(e *fsm.Event) { fmt.Printf("STATUSTX: local status set from %s to %s", e.Src, e.Dst) },
// 		"after_all_keygen_complete": func(e *fsm.Event) {
// 			// update total number of available keys and epoch
// 			suite.ABCIApp.state.LastCreatedIndex = suite.ABCIApp.state.LastCreatedIndex + uint(suite.ABCIApp.Suite.Config.KeysPerEpoch)
// 			fmt.Println("STATUSTX: lastcreatedindex", suite.ABCIApp.state.LastCreatedIndex)
// 			suite.ABCIApp.state.Epoch = suite.ABCIApp.state.Epoch + uint(1)
// 			fmt.Println("STATUSTX: state is", suite.ABCIApp.state)
// 			fmt.Println("STATUSTX: epoch is", suite.ABCIApp.state.Epoch)
// 		},
// 	},
// )

type KeygenInstance struct {
	State   *fsm.FSM
	NodeLog map[string]*fsm.FSM
	KeyLog  map[string](map[string]KEYGENLog)
}

// KEYGEN STATES (SK)
const (
	// Internal States
	SKWaitingInitiateKeygen = "waiting_initiate_keygen"
	SKReadyForKeygen        = "ready_for_keygen"

	// For node log
	SKStandby   = "standby"
	SKKeygening = "keygening"
)

// KEYGEN Events (EK)
const (
	// Internal Events
	EKAllInitiateKeygen = "all_initiate_keygen"

	// For node log events
	EKInitiateKeygen = "initiate_keygen"
)

//TODO: Potentially Stuff specific KEYGEN Debugger
func (ki *KeygenInstance) InitiateKeygen(startingIndex big.Int, endingIndex big.Int, nodeIndexes []big.Int, threshold int) error {
	// We start initiate keygen state at waiting_initiate_keygen
	ki.State = fsm.NewFSM(
		SKWaitingInitiateKeygen,
		fsm.Events{
			{Name: EKAllInitiateKeygen, Src: []string{SKWaitingInitiateKeygen}, Dst: SKReadyForKeygen},
			{Name: "", Src: []string{SKWaitingInitiateKeygen}, Dst: SKReadyForKeygen},
		},
		fsm.Callbacks{
			"enter_state": func(e *fsm.Event) { logging.Debugf("STATUSTX: local status set from %s to %s", e.Src, e.Dst) },
		},
	)

	ki.NodeLog = make(map[string]*fsm.FSM)
	for _, nodeIndex := range nodeIndexes {
		ki.NodeLog[nodeIndex.Text(16)] = fsm.NewFSM(
			SKStandby,
			fsm.Events{
				{Name: EKInitiateKeygen, Src: []string{SKStandby}, Dst: SKKeygening},
				{Name: "", Src: []string{"waiting_initiate_keygen"}, Dst: "keygening"},
			},
			fsm.Callbacks{
				"enter_state": func(e *fsm.Event) { logging.Debugf("STATUSTX: local status set from %s to %s", e.Src, e.Dst) },
				"after_" + EKInitiateKeygen: func(e *fsm.Event) {

					// See if all Initiate Keygens are in
					counter := 0
					for _, v := range ki.NodeLog {
						if v.Current() == SKKeygening {
							counter++
						}
					}

					if counter == len(ki.NodeLog) {
						go func() {
							err := ki.State.Event(EKAllInitiateKeygen)
							if err != nil {
								logging.Errorf("Could not %s. Err: %s", EKAllInitiateKeygen, err)
							}
						}()
					}
				},
			},
		)
	}

	ki.KeyLog = make(map[string](map[string]KEYGENLog))

	//TODO: We neet to set a timing (t1) here
	return nil
}

func (ki *KeygenInstance) onInitiateKeygen(commitmentMatrixes [][][]common.Point, nodeIndex big.Int) error {

	return nil
}
