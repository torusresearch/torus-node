package keygen

import (
	"errors"
	"fmt"
	"math"
	"math/big"
	"sort"
	"strconv"
	"sync"

	"github.com/torusresearch/torus-public/pvss"

	"github.com/torusresearch/torus-public/logging"

	"github.com/looplab/fsm"

	"github.com/torusresearch/torus-public/common"
)

// Below are the stated Keygen Types necessary for Communication between nodes
type KEYGENSend struct {
	KeyIndex big.Int
	AIY      common.PrimaryPolynomial
	AIprimeY common.PrimaryPolynomial
	BIX      common.PrimaryPolynomial
	BIprimeX common.PrimaryPolynomial
}

type KEYGENEcho struct {
	KeyIndex big.Int
	Dealer   big.Int
	Aij      big.Int
	Aprimeij big.Int
	Bij      big.Int
	Bprimeij big.Int
}
type KEYGENReady struct {
	KeyIndex big.Int
	Dealer   big.Int
	Aij      big.Int
	Aprimeij big.Int
	Bij      big.Int
	Bprimeij big.Int
	ReadySig []byte
}

type KEYGENInitiate struct {
	CommitmentMatrixes [][][]common.Point
}

type KEYGENShareComplete struct {
	KeyIndex big.Int
	c        big.Int
	u1       big.Int
	u2       big.Int
	gsi      common.Point
	gsihr    common.Point
}

type KEYGENDKGComplete struct {
	Nonce           int
	Proofs          []KEYGENShareComplete
	NodeSet         []string                                    // sorted nodeIndexes in Set
	ReadySignatures map[string](map[string](map[string][]byte)) // Keyindex => DealerIndex => NodeIndex (Who signed)
}

// KeyIndex => NodeIndex => KEYGENLog
type KEYGENLog struct {
	KeyIndex               big.Int
	NodeIndex              big.Int
	C                      [][]common.Point               // big.Int (in hex) to Commitment matrix
	ReceivedSend           *KEYGENSend                    // Polynomials for respective commitment matrix.
	ReceivedEchoes         map[string]KEYGENEcho          // From(M) big.Int (in hex) to Echo
	ReceivedReadys         map[string]KEYGENReady         // From(M) big.Int (in hex) to Ready
	ReceivedShareCompletes map[string]KEYGENShareComplete // From(M) big.Int (in hex) to ShareComplete
	SubshareState          *fsm.FSM                       // For tracking the state of our share
}

// KeyIndex => KEYGENSecrets
// Used to keep secrets and polynomials for secrets (As well as KEYGENSends sent by the node)
type KEYGENSecrets struct {
	Secret big.Int
	F      [][]big.Int
	Fprime [][]big.Int
}

type NodeLog struct {
	fsm.FSM
	PerfectShareCount int
}

type AVSSKeygen interface {
	// Trigger Start for Keygen and Initialize
	InitiateKeygen() error

	// For this, these listeners must be triggered on incoming messages
	// Listeners and Reactions
	OnInitiateKeygen(msg KEYGENInitiate, nodeIndex big.Int) error
	OnKEYGENSend(msg KEYGENSend, fromNodeIndex big.Int) error
	OnKEYGENEcho(msg KEYGENEcho, fromNodeIndex big.Int) error
	OnKEYGENReady(msg KEYGENReady, fromNodeIndex big.Int) error
	OnKEYGENDKGComplete(keygenShareCompletes KEYGENDKGComplete, fromNodeIndex big.Int) error

	// Storage for Secrets/Shares/etc... go here
}

type AVSSKeygenTransport interface {
	// Implementing the Code below will allow KEYGEN to run
	// "Client" Actions
	BroadcastInitiateKeygen(msg KEYGENInitiate) error
	SendKEYGENSend(msg KEYGENSend, nodeIndex big.Int) error
	SendKEYGENEcho(msg KEYGENEcho, nodeIndex big.Int) error
	SendKEYGENReady(msg KEYGENReady, nodeIndex big.Int) error
	BroadcastKEYGENDKGComplete(msg KEYGENDKGComplete) error
}

// To store necessary shares and secrets
type AVSSKeygenStorage interface {
	StoreKEYGENSecret(keyIndex big.Int, secret KEYGENSecrets) error
	StoreCompletedShare(keyIndex big.Int, si big.Int, siprime big.Int) error
}
type AVSSAuth interface {
	Sign(msg string) ([]byte, error)
	Verify(text string, nodeIndex big.Int, signature []byte) bool
}

// Main Keygen Struct
type KeygenInstance struct {
	sync.Mutex
	NodeIndex         big.Int
	Threshold         int // in AVSS Paper this is k
	NumMalNodes       int // in AVSS Paper this is t
	TotalNodes        int // in AVSS Paper this is n
	State             *fsm.FSM
	NodeLog           map[string]*NodeLog                // nodeindex => fsm equivilent to qualified set
	UnqualifiedNodes  map[string]*NodeLog                // nodeindex => fsm equivilent to unqualified set
	KeyLog            map[string](map[string]*KEYGENLog) // keyindex => nodeindex => log
	Secrets           map[string]KEYGENSecrets           // keyindex => KEYGENSecrets
	StartIndex        big.Int
	NumOfKeys         int
	SubsharesComplete int // We keep a count of number of subshares that are fully complete to avoid checking on every iteration
	Transport         AVSSKeygenTransport
	Store             AVSSKeygenStorage
	MsgBuffer         KEYGENBuffer
	Auth              AVSSAuth
	FinalNodeSet      []string
	Nonce             int
	ComChannel        chan string
}

const retryDelay = 2
const readyPrefix = "mug"

// KEYGEN STATES (SK)
const (
	// State - Internal
	SIWaitingInitiateKeygen   = "waiting_initiate_keygen"
	SIRunningKeygen           = "running_keygen"
	SIWaitingToFinishUpKeygen = "waiting_to_finish_up_keygen"
	SIKeygenCompleted         = "keygen_completed"

	// For State - node log
	SNStandby                  = "standby"
	SNKeygening                = "keygening"
	SNQualifiedNode            = "qualified_node"
	SNInculdedInKEYGENComplete = "included_in_keygen_complete"
	SNSyncedShareComplete      = "synced_share_complete"
	SNUnqualifiedNode          = "unqualified_node"

	// State - KeyLog
	SKWaitingForSend     = "waiting_for_sends"
	SKWaitingForEchos    = "waiting_for_echos"
	SKWaitingForReadys   = "waiting_for_readys"
	SKValidSubshare      = "valid_subshare"
	SKPerfectSubshare    = "perfect_subshare"
	SKEchoReconstructing = "echo_reconstructing"
)

// KEYGEN Events (EK)
const (
	// Internal Events
	EIAllInitiateKeygen     = "all_initiate_keygen"
	EIAllSubsharesDone      = "all_subshares_done"
	EISentKeygenDKGComplete = "sent_keygen_dkg_complete"

	// For node log events
	ENInitiateKeygen      = "initiate_keygen"
	ENValidShares         = "valid_shares"
	ENFailedRoundOne      = "failed_round_one"
	ENFailedRoundTwo      = "failed_round_two"
	ENSentKEYGENComplete  = "sent_share_complete"
	ENSyncKEYGENComplete  = "sync_keygen_complete"
	ENResetKEYGENComplete = "reset_keygen_complete"

	// Events - KeyLog
	EKSendEcho           = "send_echo"
	EKSendReady          = "send_ready"
	EKTReachedSubshare   = "t_reached_subshare"
	EKAllReachedSubshare = "all_reached_subshare"
	EKEchoReconstruct    = "echo_reconstruct"
)

func NewAVSSKeygen(startingIndex big.Int, numOfKeys int, nodeIndexes []big.Int, threshold int, numMalNodes int, nodeIndex big.Int, transport AVSSKeygenTransport, store AVSSKeygenStorage, auth AVSSAuth, comChannel chan string) (*KeygenInstance, error) {
	ki := &KeygenInstance{}
	ki.Lock()
	defer ki.Unlock()
	ki.NodeIndex = nodeIndex
	ki.Threshold = threshold
	ki.NumMalNodes = numMalNodes
	ki.TotalNodes = len(nodeIndexes)
	ki.StartIndex = startingIndex
	ki.NumOfKeys = numOfKeys
	ki.SubsharesComplete = 0
	ki.Nonce = 0
	ki.NodeLog = make(map[string]*NodeLog)
	ki.UnqualifiedNodes = make(map[string]*NodeLog)
	ki.Transport = transport
	ki.Store = store
	ki.Auth = auth
	ki.ComChannel = comChannel
	// Initialize buffer
	ki.MsgBuffer = KEYGENBuffer{}
	ki.MsgBuffer.InitializeMsgBuffer(startingIndex, numOfKeys, nodeIndexes)
	// We start initiate keygen state at waiting_initiate_keygen
	ki.State = fsm.NewFSM(
		SIWaitingInitiateKeygen,
		fsm.Events{
			{Name: EIAllInitiateKeygen, Src: []string{SIWaitingInitiateKeygen}, Dst: SIRunningKeygen},
			{Name: EIAllSubsharesDone, Src: []string{SIWaitingToFinishUpKeygen, SIRunningKeygen}, Dst: SIKeygenCompleted},
			{Name: EISentKeygenDKGComplete, Src: []string{SIRunningKeygen}, Dst: SIWaitingToFinishUpKeygen},
		},
		fsm.Callbacks{
			"enter_state": func(e *fsm.Event) {
				logging.Debugf("NODE"+ki.NodeIndex.Text(16)+"KEYGEN: state transition from %s to %s", e.Src, e.Dst)
			},
			"after_" + EIAllInitiateKeygen: func(e *fsm.Event) {
				ki.Lock()
				defer ki.Unlock()
				// TODO: Take care of case where this is called by end in t1
				// send all KEGENSends to  respective nodes
				logging.Debugf("NODE" + ki.NodeIndex.Text(16) + " Sending KEYGENSends")
				for i := int(startingIndex.Int64()); i < numOfKeys+int(startingIndex.Int64()); i++ {
					keyIndex := big.NewInt(int64(i))
					committedSecrets := ki.Secrets[keyIndex.Text(16)]
					ki.Store.StoreKEYGENSecret(*keyIndex, committedSecrets)
					for k := range ki.NodeLog {

						nodeIndex := big.Int{}
						nodeIndex.SetString(k, 16)

						keygenSend := KEYGENSend{
							KeyIndex: *keyIndex,
							AIY:      pvss.EvaluateBivarPolyAtX(committedSecrets.F, nodeIndex),
							AIprimeY: pvss.EvaluateBivarPolyAtX(committedSecrets.Fprime, nodeIndex),
							BIX:      pvss.EvaluateBivarPolyAtY(committedSecrets.F, nodeIndex),
							BIprimeX: pvss.EvaluateBivarPolyAtY(committedSecrets.Fprime, nodeIndex),
						}
						// send to node
						err := ki.Transport.SendKEYGENSend(keygenSend, nodeIndex)
						if err != nil {
							// TODO: Resend?
							logging.Errorf("NODE"+ki.NodeIndex.Text(16)+"Could not send KEYGENSend : %s", err)
						}
					}
				}
			},
			"after_" + EIAllSubsharesDone: func(e *fsm.Event) {
				ki.Lock()
				defer ki.Unlock()
				if len(ki.FinalNodeSet) == 0 {
					// create qualified set
					var nodeSet []string
					for nodeIndex, log := range ki.NodeLog {
						if log.Is(SNQualifiedNode) {
							nodeSet = append(nodeSet, nodeIndex)
						}
					}
					sort.SliceStable(nodeSet, func(i, j int) bool { return nodeSet[i] < nodeSet[j] })

					// Here we prepare KEYGENShareComplete
					keygenShareCompletes := make([]KEYGENShareComplete, ki.NumOfKeys)
					for i := 0; i < ki.NumOfKeys; i++ {
						keyIndex := big.Int{}
						keyIndex.SetInt64(int64(i)).Add(&keyIndex, &ki.StartIndex)
						// form  Si
						si := big.NewInt(int64(0))
						siprime := big.NewInt(int64(0))
						count := 0
						for _, nodeIndex := range nodeSet {
							// add up subshares for qualified set
							count++
							v := ki.KeyLog[keyIndex.Text(16)][nodeIndex]
							si.Add(si, &v.ReceivedSend.AIY.Coeff[0])
							siprime.Add(siprime, &v.ReceivedSend.AIprimeY.Coeff[0])
						}
						// logging.Debugf("NODE"+ki.NodeIndex.Text(16)+" Count for KEYGENComplete: %v", count)
						c, u1, u2, gs, gshr := pvss.GenerateNIZKPKWithCommitments(*si, *siprime)
						keygenShareCompletes[i] = KEYGENShareComplete{
							KeyIndex: keyIndex,
							c:        c,
							u1:       u1,
							u2:       u2,
							gsi:      gs,
							gsihr:    gshr,
						}
					}

					tempReadySigMap := make(map[string](map[string](map[string][]byte)))
					// prepare keygen Ready signatures
					for keyIndex, keylog := range ki.KeyLog {
						tempReadySigMap[keyIndex] = make(map[string](map[string][]byte))
						for _, dealerIndex := range nodeSet {
							tempReadySigMap[keyIndex][dealerIndex] = make(map[string][]byte)
							for sigIndex, ready := range keylog[dealerIndex].ReceivedReadys {
								tempReadySigMap[keyIndex][dealerIndex][sigIndex] = ready.ReadySig
							}
						}
					}

					// broadcast keygen
					err := ki.Transport.BroadcastKEYGENDKGComplete(KEYGENDKGComplete{Nonce: ki.Nonce, NodeSet: nodeSet, Proofs: keygenShareCompletes, ReadySignatures: tempReadySigMap})
					if err != nil {
						logging.Errorf("NODE"+ki.NodeIndex.Text(16)+"Could not BroadcastKEYGENDKGComplete: %s", err)
					}
				} else {
					// End keygen.
					go func() {
						err := ki.State.Event(EIAllSubsharesDone)
						if err != nil {
							logging.Errorf("NODE"+ki.NodeIndex.Text(16)+" Node %s Could not %s. Err: %s", EIAllSubsharesDone, err)
						}
					}()
				}

			},
			"enter_" + SIKeygenCompleted: func(e *fsm.Event) {
				// To wrap things up we first store Secrets and respective Key Shares
				for i := 0; i < ki.NumOfKeys; i++ {
					keyIndex := big.Int{}
					keyIndex.SetInt64(int64(i)).Add(&keyIndex, &ki.StartIndex)
					// form  Si
					si := big.NewInt(int64(0))
					siprime := big.NewInt(int64(0))
					count := 0
					for _, nodeIndex := range ki.FinalNodeSet {
						// add up subshares for qualified set
						count++
						v := ki.KeyLog[keyIndex.Text(16)][nodeIndex]
						si.Add(si, &v.ReceivedSend.AIY.Coeff[0])
						siprime.Add(siprime, &v.ReceivedSend.AIprimeY.Coeff[0])
					}
					err := ki.Store.StoreCompletedShare(keyIndex, *si, *siprime)
					if err != nil {
						// TODO: Handle error in channel?
						logging.Error("error storing share: " + err.Error())
					}
					err = ki.Store.StoreKEYGENSecret(keyIndex, ki.Secrets[keyIndex.Text(16)])
					if err != nil {
						// TODO: Handle error in channel?
						logging.Error("error storing share: " + err.Error())
					}
				}
				// Communicate Keygen Completion
				completionMsg := SIKeygenCompleted + "|" + ki.StartIndex.Text(10) + "|" + strconv.Itoa(ki.NumOfKeys)
				logging.Debugf("This runs: %s", completionMsg)
				logging.Debugf("Nujm of keys %v", ki.NumOfKeys)
				ki.ComChannel <- completionMsg
			},
		},
	)

	// Node Log FSM tracks the state of other nodes involved in this Keygen phase
	for _, nodeIndex := range nodeIndexes {
		tempFsm := fsm.NewFSM(
			SNStandby,
			fsm.Events{
				{Name: ENInitiateKeygen, Src: []string{SNStandby}, Dst: SNKeygening},
				{Name: ENValidShares, Src: []string{SNKeygening}, Dst: SNQualifiedNode},
				{Name: ENFailedRoundOne, Src: []string{SNStandby}, Dst: SNUnqualifiedNode},
				{Name: ENFailedRoundTwo, Src: []string{SNStandby, SNKeygening}, Dst: SNUnqualifiedNode},
				{Name: ENSyncKEYGENComplete, Src: []string{SNKeygening, SNQualifiedNode}, Dst: SNSyncedShareComplete},
			},
			fsm.Callbacks{
				"enter_state": func(e *fsm.Event) {
					logging.Debugf("NODE"+ki.NodeIndex.Text(16)+"NodeLog State changed from %s to %s", e.Src, e.Dst)
				},
				"after_" + ENInitiateKeygen: func(e *fsm.Event) {
					ki.Lock()
					defer ki.Unlock()
					// See if all Initiate Keygens are in
					counter := 0
					for _, v := range ki.NodeLog {
						if v.Is(SNKeygening) {
							counter++
						}
					}

					if counter == len(ki.NodeLog) {
						go func() {
							err := ki.State.Event(EIAllInitiateKeygen)
							if err != nil {
								logging.Errorf("NODE"+ki.NodeIndex.Text(16)+"Node %s Could not %s. Err: %s", ki.NodeIndex.Text(16), EIAllInitiateKeygen, err)
							}
						}()
					}
				},
			},
		)
		ki.NodeLog[nodeIndex.Text(16)] = &NodeLog{FSM: *tempFsm, PerfectShareCount: 0}
	}

	// INitiatlize keylogs FSM that tracks each individual subshare
	ki.KeyLog = make(map[string](map[string]*KEYGENLog))
	// start with key indexes
	// then by the number of declared nodes
	for i := 0; i < ki.NumOfKeys; i++ {
		index := big.NewInt(int64(i))
		index.Add(index, &ki.StartIndex)
		ki.KeyLog[index.Text(16)] = make(map[string]*KEYGENLog)
		for _, ni := range nodeIndexes {
			nodeIndex := big.Int{}
			nodeIndex.Set(&ni)
			ki.KeyLog[index.Text(16)][nodeIndex.Text(16)] = &KEYGENLog{
				KeyIndex:  *index,
				NodeIndex: nodeIndex,
				// C:                      commitmentMatrix,
				ReceivedEchoes:         make(map[string]KEYGENEcho),          // From(M) big.Int (in hex) to Echo
				ReceivedReadys:         make(map[string]KEYGENReady),         // From(M) big.Int (in hex) to Ready
				ReceivedShareCompletes: make(map[string]KEYGENShareComplete), // From(M) big.Int (in hex) to ShareComplete
				SubshareState: fsm.NewFSM(
					SKWaitingForSend,
					fsm.Events{
						{Name: EKSendEcho, Src: []string{SKWaitingForSend}, Dst: SKWaitingForEchos},
						{Name: EKSendReady, Src: []string{SKWaitingForEchos, SKEchoReconstructing}, Dst: SKWaitingForReadys},
						{Name: EKTReachedSubshare, Src: []string{SKWaitingForReadys}, Dst: SKValidSubshare},
						{Name: EKAllReachedSubshare, Src: []string{SKValidSubshare}, Dst: SKPerfectSubshare},
						{Name: EKEchoReconstruct, Src: []string{SKWaitingForSend}, Dst: SKEchoReconstructing},
					},
					fsm.Callbacks{
						"enter_state": func(e *fsm.Event) {
							logging.Debugf("NODE"+ki.NodeIndex.Text(16)+"subshare set from %s to %s", e.Src, e.Dst)
						},
						"after_" + EKSendEcho: func(e *fsm.Event) {
							ki.Lock()
							defer ki.Unlock()
							for k := range ki.NodeLog {
								nodeToSendIndex := big.Int{}
								nodeToSendIndex.SetString(k, 16)
								msg := ki.KeyLog[index.Text(16)][nodeIndex.Text(16)].ReceivedSend
								if msg != nil {
									keygenEcho := KEYGENEcho{
										KeyIndex: *index,
										Dealer:   nodeIndex,
										Aij:      *pvss.PolyEval(msg.AIY, nodeToSendIndex),
										Aprimeij: *pvss.PolyEval(msg.AIprimeY, nodeToSendIndex),
										Bij:      *pvss.PolyEval(msg.BIX, nodeToSendIndex),
										Bprimeij: *pvss.PolyEval(msg.BIprimeX, nodeToSendIndex),
									}
									err := ki.Transport.SendKEYGENEcho(keygenEcho, nodeToSendIndex)
									if err != nil {
										// TODO: Handle failure, resend?
										logging.Debugf("NODE"+ki.NodeIndex.Text(16)+"Error sending echo: %v", err)
									}
								}
							}

						},
						"after_" + EKSendReady: func(e *fsm.Event) {
							ki.Lock()
							defer ki.Unlock()
							// we send readys here when we have collected enough echos
							for k := range ki.NodeLog {
								nodeToSendIndex := big.Int{}
								keyIndexStr := index.Text(16)
								dealerStr := nodeIndex.Text(16)
								nodeToSendIndex.SetString(k, 16)
								sig, err := ki.Auth.Sign(readyPrefix + keyIndexStr + dealerStr)
								if err != nil {
									logging.Error(err.Error())
								}
								keygenReady := KEYGENReady{
									KeyIndex: *index,
									Dealer:   nodeIndex,
									Aij:      *pvss.PolyEval(ki.KeyLog[keyIndexStr][dealerStr].ReceivedSend.AIY, nodeToSendIndex),
									Aprimeij: *pvss.PolyEval(ki.KeyLog[keyIndexStr][dealerStr].ReceivedSend.AIprimeY, nodeToSendIndex),
									Bij:      *pvss.PolyEval(ki.KeyLog[keyIndexStr][dealerStr].ReceivedSend.BIX, nodeToSendIndex),
									Bprimeij: *pvss.PolyEval(ki.KeyLog[keyIndexStr][dealerStr].ReceivedSend.BIprimeX, nodeToSendIndex),
									ReadySig: sig,
								}
								err = ki.Transport.SendKEYGENReady(keygenReady, nodeToSendIndex)
								if err != nil {
									// TODO: Handle failure, resend?
									logging.Errorf("NODE"+ki.NodeIndex.Text(16)+"Could not sent KEYGENReady %s", err)
								}
							}
						},
						"after_" + EKTReachedSubshare: func(e *fsm.Event) {
							ki.Lock()
							defer ki.Unlock()
							// to accomodate for when EKTReachedSubshare gets called after EKAllReachedSubshare
							if len(ki.KeyLog[index.Text(16)][nodeIndex.Text(16)].ReceivedReadys) == len(ki.NodeLog) {
								// keyLog.SubshareState.Is(SKWaitingForReadys) is to cater for when KEYGENReady is logged too fast and it isnt enough to call OnKEYGENReady twice?
								// if keyLog.SubshareState.Is(SKValidSubshare) || keyLog.SubshareState.Is(SKWaitingForReadys) {
								go func(innerKeyLog *KEYGENLog) {
									err := innerKeyLog.SubshareState.Event(EKAllReachedSubshare)
									if err != nil {
										logging.Error(err.Error())
									}
								}(ki.KeyLog[index.Text(16)][nodeIndex.Text(16)])
								// }
							}
						},
						"after_" + EKAllReachedSubshare: func(e *fsm.Event) {
							ki.Lock()
							defer ki.Unlock()
							// Add to counts
							ki.SubsharesComplete++
							ki.NodeLog[nodeIndex.Text(16)].PerfectShareCount++
							// Check if Node subshares are complete
							if ki.NodeLog[nodeIndex.Text(16)].PerfectShareCount == ki.NumOfKeys {
								go func(state *NodeLog) {
									// end keygen
									err := state.Event(ENValidShares)
									if err != nil {
										logging.Errorf("NODE"+ki.NodeIndex.Text(16)+"Could not change state to subshare done: %s", err)
									}
								}(ki.NodeLog[nodeIndex.Text(16)])
							}

							// Check if all subshares are complete
							if ki.SubsharesComplete == ki.NumOfKeys*len(ki.NodeLog) {
								go func(state *fsm.FSM) {
									// end keygen
									err := state.Event(EIAllSubsharesDone)
									if err != nil {
										logging.Errorf("NODE"+ki.NodeIndex.Text(16)+"Could not change state to subshare done: %s", err)
									}
								}(ki.State)
							}
						},
						"after_" + EKEchoReconstruct: func(e *fsm.Event) {
							ki.Lock()
							defer ki.Unlock()
							count := 0
							// prepare for derivation of polynomial
							aiyPoints := make([]common.Point, ki.Threshold)
							aiprimeyPoints := make([]common.Point, ki.Threshold)
							bixPoints := make([]common.Point, ki.Threshold)
							biprimexPoints := make([]common.Point, ki.Threshold)

							for k, v := range ki.KeyLog[index.Text(16)][nodeIndex.Text(16)].ReceivedEchoes {
								if count < ki.Threshold {
									fromNodeInt := big.Int{}
									fromNodeInt.SetString(k, 16)
									aiyPoints[count] = common.Point{X: fromNodeInt, Y: v.Aij}
									aiprimeyPoints[count] = common.Point{X: fromNodeInt, Y: v.Aprimeij}
									bixPoints[count] = common.Point{X: fromNodeInt, Y: v.Bij}
									biprimexPoints[count] = common.Point{X: fromNodeInt, Y: v.Bprimeij}
								}
								count++
							}

							aiy := pvss.LagrangeInterpolatePolynomial(bixPoints)
							aiprimey := pvss.LagrangeInterpolatePolynomial(biprimexPoints)
							bix := pvss.LagrangeInterpolatePolynomial(aiyPoints)
							biprimex := pvss.LagrangeInterpolatePolynomial(aiprimeyPoints)
							ki.KeyLog[index.Text(16)][nodeIndex.Text(16)].ReceivedSend = &KEYGENSend{
								KeyIndex: *index,
								AIY:      common.PrimaryPolynomial{Threshold: ki.Threshold, Coeff: aiy},
								AIprimeY: common.PrimaryPolynomial{Threshold: ki.Threshold, Coeff: aiprimey},
								BIX:      common.PrimaryPolynomial{Threshold: ki.Threshold, Coeff: bix},
								BIprimeX: common.PrimaryPolynomial{Threshold: ki.Threshold, Coeff: biprimex},
							}

							correctPoly := pvss.AVSSVerifyPoly(
								ki.KeyLog[index.Text(16)][nodeIndex.Text(16)].C,
								ki.NodeIndex,
								ki.KeyLog[index.Text(16)][nodeIndex.Text(16)].ReceivedSend.AIY,
								ki.KeyLog[index.Text(16)][nodeIndex.Text(16)].ReceivedSend.AIprimeY,
								ki.KeyLog[index.Text(16)][nodeIndex.Text(16)].ReceivedSend.BIX,
								ki.KeyLog[index.Text(16)][nodeIndex.Text(16)].ReceivedSend.BIprimeX,
							)
							if !correctPoly {
								logging.Errorf("Correct poly is not right for subshare index %s from node %s", index.Text(16), nodeIndex.Text(16))
							}
							// Now we send our readys
							go func(keyLog *KEYGENLog) {
								err := keyLog.SubshareState.Event(EKSendReady)
								if err != nil {
									logging.Errorf("NODE"+ki.NodeIndex.Text(16)+"Could not change subshare state: %s", err)
								}
							}(ki.KeyLog[index.Text(16)][nodeIndex.Text(16)])
						},
						// Below functions are for the state machines to catch up on previously sent messages using
						// MsgBuffer We dont need to lock as msg buffer should do so
						"enter_" + SKWaitingForSend: func(e *fsm.Event) {
							send := ki.MsgBuffer.RetrieveKEYGENSends(*index, nodeIndex)
							if send != nil {
								go ki.OnKEYGENSend(*send, nodeIndex)
							}
						},
						"enter_" + SKWaitingForEchos: func(e *fsm.Event) {
							bufferEchoes := ki.MsgBuffer.RetrieveKEYGENEchoes(*index, nodeIndex)
							for from, echo := range bufferEchoes {
								intNodeIndex := big.Int{}
								intNodeIndex.SetString(from, 16)
								go (ki).OnKEYGENEcho(*echo, intNodeIndex)
							}
						},
						"enter_" + SKWaitingForReadys: func(e *fsm.Event) {
							bufferReadys := ki.MsgBuffer.RetrieveKEYGENReadys(*index, nodeIndex)
							for from, ready := range bufferReadys {
								intNodeIndex := big.Int{}
								intNodeIndex.SetString(from, 16)
								go (ki).OnKEYGENReady(*ready, intNodeIndex)
							}
						},
					},
				),
			}
		}
	}
	ki.Secrets = make(map[string]KEYGENSecrets)

	// TODO: Trigger setting up of listeners here

	return ki, nil
}

// TODO: Potentially Stuff specific KEYGEN Debugger | set up transport here as well & store
func (ki *KeygenInstance) InitiateKeygen() error {
	ki.Lock()
	defer ki.Unlock()
	// prepare commitmentMatrixes for broadcast
	commitmentMatrixes := make([][][]common.Point, ki.NumOfKeys)
	for i := 0; i < ki.NumOfKeys; i++ {
		// help initialize all the keylogs
		secret := *pvss.RandomBigInt()
		f := pvss.GenerateRandomBivariatePolynomial(secret, ki.Threshold)
		fprime := pvss.GenerateRandomBivariatePolynomial(*pvss.RandomBigInt(), ki.Threshold)
		commitmentMatrixes[i] = pvss.GetCommitmentMatrix(f, fprime)

		// store secrets
		keyIndex := big.NewInt(int64(i))
		keyIndex.Add(keyIndex, &ki.StartIndex)
		ki.Secrets[keyIndex.Text(16)] = KEYGENSecrets{
			Secret: secret,
			F:      f,
			Fprime: fprime,
		}
	}
	err := ki.Transport.BroadcastInitiateKeygen(KEYGENInitiate{commitmentMatrixes})
	if err != nil {
		return err
	}

	// TODO: We neet to set a timing (t1) here (Deprecated)
	// Changed to tendermint triggering timing

	return nil
}

func (ki *KeygenInstance) OnInitiateKeygen(msg KEYGENInitiate, nodeIndex big.Int) error {
	ki.Lock()
	defer ki.Unlock()
	// Only accept onInitiate on Standby phase to only accept initiate keygen once from one node index
	if ki.NodeLog[nodeIndex.Text(16)].Is(SNStandby) {
		// check length of commitment matrix is right
		if len(msg.CommitmentMatrixes) != ki.NumOfKeys {
			return errors.New("length of  commitment matrix is not correct")
		}
		// store commitment matrix
		for i, commitmentMatrix := range msg.CommitmentMatrixes {
			index := big.NewInt(int64(i))
			index.Add(index, &ki.StartIndex)
			// TODO: create state to handle time out of t2
			ki.KeyLog[index.Text(16)][nodeIndex.Text(16)].C = commitmentMatrix
		}

		go func(nodeLog *NodeLog) {
			err := nodeLog.Event(ENInitiateKeygen)
			if err != nil {
				logging.Error(err.Error())
			}
		}(ki.NodeLog[nodeIndex.Text(16)])

	}
	return nil
}

func (ki *KeygenInstance) OnKEYGENSend(msg KEYGENSend, fromNodeIndex big.Int) error {
	logging.Debug("send in keygen")
	ki.Lock()
	defer ki.Unlock()
	keyLog, ok := ki.KeyLog[msg.KeyIndex.Text(16)][fromNodeIndex.Text(16)]
	if !ok {
		return errors.New("Keylog not found for keygen send")
	}
	if ok && keyLog.SubshareState.Is(SKWaitingForSend) {
		logging.Debug("parsing send")
		// we verify keygen, if valid we log it here. Then we send an echo
		if !pvss.AVSSVerifyPoly(
			keyLog.C,
			ki.NodeIndex,
			msg.AIY,
			msg.AIprimeY,
			msg.BIX,
			msg.BIprimeX,
		) {
			return errors.New(fmt.Sprintf("KEYGENSend not valid to declared commitments. From: %s To: %s KEYGENSend: %+v", fromNodeIndex.Text(16), ki.NodeIndex.Text(16), msg))
		}

		// since valid we log
		ki.KeyLog[msg.KeyIndex.Text(16)][fromNodeIndex.Text(16)].ReceivedSend = &msg
		keyLog = ki.KeyLog[msg.KeyIndex.Text(16)][fromNodeIndex.Text(16)]

		// and send echo
		logging.Debug("Triggered sending an echo")
		go func(innerKeyLog *KEYGENLog) {
			err := innerKeyLog.SubshareState.Event(EKSendEcho)
			if err != nil {
				logging.Error(err.Error())
			}
		}(keyLog)

	} else {
		logging.Debugf("NODE" + ki.NodeIndex.Text(16) + " storing KEYGENSend")
		ki.MsgBuffer.StoreKEYGENSend(msg, fromNodeIndex)
	}
	return nil
}

func (ki *KeygenInstance) OnKEYGENEcho(msg KEYGENEcho, fromNodeIndex big.Int) error {
	ki.Lock()
	defer ki.Unlock()
	if ki.State.Is(SIRunningKeygen) {
		keyLog, ok := ki.KeyLog[msg.KeyIndex.Text(16)][msg.Dealer.Text(16)]
		if !ok {
			return errors.New("Keylog not found OnKEYGENEcho")
		}
		// verify echo, if correct log echo. If there are more then threshold Echos we send ready
		if !pvss.AVSSVerifyPoint(
			keyLog.C,
			fromNodeIndex,
			ki.NodeIndex,
			msg.Aij,
			msg.Aprimeij,
			msg.Bij,
			msg.Bprimeij,
		) {
			// TODO: potentially invalidate nodes here
			return errors.New(fmt.Sprintf("KEYGENEcho not valid to declared commitments. From: %s To: %s KEYGENEcho: %+v", fromNodeIndex.Text(16), ki.NodeIndex.Text(16), msg))
		}

		// log echo
		keyLog.ReceivedEchoes[fromNodeIndex.Text(16)] = msg

		// check for echos
		ratio := int(math.Ceil((float64(ki.TotalNodes) + float64(ki.NumMalNodes) + 1.0) / 2.0)) // this is just greater equal than (differs from AVSS Paper equals) because we fsm to only send ready at most once
		if ki.Threshold <= len(keyLog.ReceivedEchoes) && ratio <= len(keyLog.ReceivedEchoes) {
			// since threshoold and above

			// we etiher send ready
			// if keyLog.SubshareState.Is(SKWaitingForEchos) {
			go func(innerKeyLog *KEYGENLog) {
				err := innerKeyLog.SubshareState.Event(EKSendReady)
				if err != nil {
					logging.Error(err.Error())
				}
			}(keyLog)
			// }
			// Or here we cater for reconstruction in the case of malcious nodes refusing to send KEYGENSend
			// if keyLog.SubshareState.Is(SKWaitingForSend) {
			go func(innerKeyLog *KEYGENLog) {
				err := innerKeyLog.SubshareState.Event(EKEchoReconstruct)
				if err != nil {
					logging.Error(err.Error())
				}
			}(keyLog)
			// }
		}
	} else {
		ki.MsgBuffer.StoreKEYGENEcho(msg, fromNodeIndex)
	}
	return nil
}

func (ki *KeygenInstance) OnKEYGENReady(msg KEYGENReady, fromNodeIndex big.Int) error {
	ki.Lock()
	defer ki.Unlock()
	if !ki.Auth.Verify(readyPrefix+msg.KeyIndex.Text(16)+msg.Dealer.Text(16), fromNodeIndex, msg.ReadySig) {
		return errors.New("Ready Signature is not right")
	}
	if ki.State.Is(SIRunningKeygen) {
		keyLog, ok := ki.KeyLog[msg.KeyIndex.Text(16)][msg.Dealer.Text(16)]
		if !ok {
			return errors.New("Keylog not found OnKEYGENReady")
		}
		// we verify ready, if right we log and check if we have enough readys to validate shares
		if !pvss.AVSSVerifyPoint(
			keyLog.C,
			fromNodeIndex,
			ki.NodeIndex,
			msg.Aij,
			msg.Aprimeij,
			msg.Bij,
			msg.Bprimeij,
		) {
			// TODO: potentially invalidate nodes here
			return errors.New(fmt.Sprintf("KEYGENReady not valid to declared commitments. From: %s To: %s KEYGENEcho: %v+", fromNodeIndex.Text(16), ki.NodeIndex.Text(16), msg))
		}

		// log ready
		keyLog.ReceivedReadys[fromNodeIndex.Text(16)] = msg

		// if we've reached the required number of readys
		if ki.Threshold <= len(keyLog.ReceivedReadys) {
			// if keyLog.SubshareState.Is(SKWaitingForReadys) {
			go func(innerKeyLog *KEYGENLog, keyIndex string, dealer string) {
				err := innerKeyLog.SubshareState.Event(EKTReachedSubshare, keyIndex, dealer)
				if err != nil {
					logging.Error(err.Error())
				}
			}(keyLog, msg.KeyIndex.Text(16), msg.Dealer.Text(16))
			// }

			if len(keyLog.ReceivedReadys) >= ki.Threshold+ki.NumMalNodes {
				// keyLog.SubshareState.Is(SKWaitingForReadys) is to cater for when KEYGENReady is logged too fast and it isnt enough to call OnKEYGENReady twice?
				// if keyLog.SubshareState.Is(SKValidSubshare) || keyLog.SubshareState.Is(SKWaitingForReadys) {
				go func(innerKeyLog *KEYGENLog) {
					err := innerKeyLog.SubshareState.Event(EKAllReachedSubshare)
					if err != nil {
						logging.Error(err.Error())
					}
				}(keyLog)
				// }
			}
		}
	} else {
		ki.MsgBuffer.StoreKEYGENReady(msg, fromNodeIndex)
	}
	return nil
}

func (ki *KeygenInstance) OnKEYGENDKGComplete(msg KEYGENDKGComplete, fromNodeIndex big.Int) error {
	ki.Lock()
	defer ki.Unlock()
	if len(msg.Proofs) != ki.NumOfKeys {
		return errors.New("length of proofs is not correct")
	}
	if len(msg.NodeSet) < ki.Threshold+ki.NumMalNodes {
		return errors.New("Nodeset not large enough")
	}
	if len(ki.FinalNodeSet) != 0 {
		return errors.New("Broadcast already accepted")
	}
	// verify shareCompletes
	for i, keygenShareCom := range msg.Proofs {
		// ensure valid keyindex
		expectedKeyIndex := big.NewInt(int64(i))
		expectedKeyIndex.Add(expectedKeyIndex, &ki.StartIndex)

		if expectedKeyIndex.Cmp(&keygenShareCom.KeyIndex) != 0 {
			logging.Debugf("NODE "+ki.NodeIndex.Text(16)+" KeyIndex %s, Expected %s", keygenShareCom.KeyIndex.Text(16), expectedKeyIndex.Text(16))
			return errors.New("Faulty key index on OnKEYGENDKGComplete")
		}
		// by first verifying NIZKPK Proof
		if !pvss.VerifyNIZKPK(keygenShareCom.c, keygenShareCom.u1, keygenShareCom.u2, keygenShareCom.gsi, keygenShareCom.gsihr) {
			return errors.New("Faulty NIZKPK Proof on OnKEYGENDKGComplete")
		}

		// add up all commitments
		var sumCommitments [][]common.Point
		// TODO: Potentially quite intensive
		for _, nodeIndex := range msg.NodeSet {
			keyLog, ok := ki.KeyLog[keygenShareCom.KeyIndex.Text(16)][nodeIndex]
			if ok {
				if len(sumCommitments) == 0 {
					sumCommitments = keyLog.C
				} else {
					sumCommitments, _ = pvss.AVSSAddCommitment(sumCommitments, keyLog.C)
					// if err != nil {
					// 	return err
					// }
				}
			}
		}

		// test commmitment
		if !pvss.AVSSVerifyShareCommitment(sumCommitments, fromNodeIndex, keygenShareCom.gsihr) {
			return errors.New("Faulty Share Commitment OnKEYGENDKGComplete")
		}

		// check ready signatures
		for keyIndex, key := range msg.ReadySignatures {
			for dealerIndex, dealer := range key {
				if len(dealer) < ki.Threshold+ki.NumMalNodes {
					return errors.New("Not enough ready statements")
				}
				for signerIndex, sig := range dealer {
					signerInt := big.Int{}
					signerInt.SetString(signerIndex, 16)
					if !ki.Auth.Verify(readyPrefix+keyIndex+dealerIndex, signerInt, sig) {
						return errors.New(" Ready statement Not valid" + readyPrefix + keyIndex + dealerIndex)
					}
				}
			}
		}
	}

	// define set
	ki.FinalNodeSet = msg.NodeSet

	// End keygen if we sent the/a broadcast
	if ki.State.Is(SIWaitingToFinishUpKeygen) {
		go func() {
			err := ki.State.Event(EIAllSubsharesDone)
			if err != nil {
				logging.Errorf("NODE"+ki.NodeIndex.Text(16)+" Node %s Could not %s. Err: %s", EIAllSubsharesDone, err)
			}
		}()
	}

	// // gshr should be a point on the sum commitment matix
	return nil
}
