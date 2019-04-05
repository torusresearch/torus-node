package keygen

import (
	"errors"
	"fmt"
	"math"
	"math/big"
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
}

type KEYGENShareComplete struct {
	KeyIndex big.Int
	c        big.Int
	u1       big.Int
	u2       big.Int
	gsi      common.Point
	gsihr    common.Point
}

// KeyIndex => NodeIndex => KEYGENLog
type KEYGENLog struct {
	KeyIndex               big.Int
	NodeIndex              big.Int
	C                      [][]common.Point               // big.Int (in hex) to Commitment matrix
	ReceivedSend           KEYGENSend                     // Polynomials for respective commitment matrix.
	ReceivedEchoes         map[string]KEYGENEcho          // From(M) big.Int (in hex) to Echo
	ReceivedReadys         map[string]KEYGENReady         // From(M) big.Int (in hex) to Ready
	ReceivedShareCompletes map[string]KEYGENShareComplete // From(M) big.Int (in hex) to ShareComplete
	SubshareState          *fsm.FSM                       // For tracking the state of our share
}

// KeyIndex => KEYGENSecrets
// Used to keep secrets and polynomials for secrets (As well as KEYGENSends sent by the node)
type KEYGENSecrets struct {
	secret big.Int
	f      [][]big.Int
	fprime [][]big.Int
}

type AVSSKeygen interface {
	// Trigger Start for Keygen and Initialize
	InitiateKeygen(startingIndex big.Int, numOfKeys int, nodeIndexes []big.Int, threshold int, numMalNodes int, nodeIndex big.Int, comChannel chan string) error

	// For this, these listeners must be triggered on incoming messages
	// Listeners and Reactions
	OnInitiateKeygen(commitmentMatrixes [][][]common.Point, nodeIndex big.Int) error
	OnKEYGENSend(msg KEYGENSend, fromNodeIndex big.Int) error
	OnKEYGENEcho(msg KEYGENEcho, fromNodeIndex big.Int) error
	OnKEYGENReady(msg KEYGENReady, fromNodeIndex big.Int) error
	OnKEYGENShareComplete(keygenShareCompletes []KEYGENShareComplete, fromNodeIndex big.Int) error

	// Storage for Secrets/Shares/etc... go here
}

type AVSSKeygenTransport interface {
	//Implementing the Code below will allow KEYGEN to run
	// "Client" Actions
	BroadcastInitiateKeygen(commitmentMatrixes [][][]common.Point) error
	SendKEYGENSend(msg KEYGENSend, nodeIndex big.Int) error
	SendKEYGENEcho(msg KEYGENEcho, nodeIndex big.Int) error
	SendKEYGENReady(msg KEYGENReady, nodeIndex big.Int) error
	BroadcastKEYGENShareComplete(keygenShareCompletes []KEYGENShareComplete) error
}

// To store necessary shares and secrets
type AVSSKeygenStorage interface {
	StoreKEYGENSecret(keyIndex big.Int, secret KEYGENSecrets) error
	StoreCompletedShare(keyIndex big.Int, si big.Int, siprime big.Int) error
}

// Main Keygen Struct
type KeygenInstance struct {
	sync.Mutex
	NodeIndex         big.Int
	Threshold         int // in AVSS Paper this is k
	NumMalNodes       int // in AVSS Paper this is t
	TotalNodes        int // in AVSS Paper this is n
	State             *fsm.FSM
	NodeLog           map[string]*fsm.FSM                // nodeindex => fsm equivilent to qualified set
	UnqualifiedNodes  map[string]*fsm.FSM                // nodeindex => fsm equivilent to qualified set
	KeyLog            map[string](map[string]*KEYGENLog) // keyindex => nodeindex => log
	Secrets           map[string]KEYGENSecrets           // keyindex => KEYGENSecrets
	StartIndex        big.Int
	NumOfKeys         int
	SubsharesComplete int // We keep a count of number of subshares that are fully complete to avoid checking on every iteration
	Transport         AVSSKeygenTransport
	Store             AVSSKeygenStorage
	MsgBuffer         KEYGENBuffer
	ComChannel        chan string
}

// KEYGEN STATES (SK)
const (
	// State - Internal
	SIWaitingInitiateKeygen      = "waiting_initiate_keygen"
	SIRunningKeygen              = "running_keygen"
	SIWaitingKEYGENShareComplete = "waiting_keygen_share_complete"
	SIKeygenCompleted            = "keygen_completed"

	// For State - node log
	SNStandby         = "standby"
	SNKeygening       = "keygening"
	SNQualifiedNode   = "qualified_node"
	SNUnqualifiedNode = "unqualified_node"

	// State - KeyLog
	SKWaitingForSends    = "waiting_for_sends"
	SKWaitingForEchos    = "waiting_for_echos"
	SKWaitingForReadys   = "waiting_for_readys"
	SKValidSubshare      = "valid_subshare"
	SKPerfectSubshare    = "perfect_subshare"
	SKEchoReconstructing = "echo_reconstructing"
)

// KEYGEN Events (EK)
const (
	// Internal Events
	EIAllInitiateKeygen  = "all_initiate_keygen"
	EIAllSubsharesDone   = "all_subshares_done"
	EIAllKeygenCompleted = "all_keygen_completed"

	// For node log events
	ENInitiateKeygen = "initiate_keygen"
	ENValidShares    = "valid_shares"
	ENFailedRoundOne = "failed_round_one"
	ENFailedRoundTwo = "failed_round_two"

	// Events - KeyLog
	EKSendEcho           = "send_echo"
	EKSendReady          = "send_ready"
	EKTReachedSubshare   = "t_reached_subshare"
	EKAllReachedSubshare = "all_reached_subshare"
	EKEchoReconstruct    = "echo_reconstruct"
)

//TODO: Potentially Stuff specific KEYGEN Debugger | set up transport here as well & store
func (ki *KeygenInstance) InitiateKeygen(startingIndex big.Int, numOfKeys int, nodeIndexes []big.Int, threshold int, numMalNodes int, nodeIndex big.Int, comChannel chan string) error {
	ki.Lock()
	defer ki.Unlock()
	ki.NodeIndex = nodeIndex
	ki.Threshold = threshold
	ki.NumMalNodes = numMalNodes
	ki.TotalNodes = len(nodeIndexes)
	ki.StartIndex = startingIndex
	ki.NumOfKeys = numOfKeys
	ki.SubsharesComplete = 0
	ki.NodeLog = make(map[string]*fsm.FSM)
	ki.UnqualifiedNodes = make(map[string]*fsm.FSM)
	ki.ComChannel = comChannel
	// Initialize buffer
	ki.MsgBuffer = KEYGENBuffer{}
	ki.MsgBuffer.InitializeMsgBuffer(startingIndex, numOfKeys, nodeIndexes)
	// We start initiate keygen state at waiting_initiate_keygen
	ki.State = fsm.NewFSM(
		SIWaitingInitiateKeygen,
		fsm.Events{
			{Name: EIAllInitiateKeygen, Src: []string{SIWaitingInitiateKeygen}, Dst: SIRunningKeygen},
			{Name: EIAllSubsharesDone, Src: []string{SIRunningKeygen}, Dst: SIWaitingKEYGENShareComplete},
			{Name: EIAllKeygenCompleted, Src: []string{SIWaitingKEYGENShareComplete}, Dst: SIKeygenCompleted},
		},
		fsm.Callbacks{
			"enter_state": func(e *fsm.Event) {
				logging.Debugf("NODE"+ki.NodeIndex.Text(16)+"KEYGEN: state transition from %s to %s", e.Src, e.Dst)
			},
			"after_" + EIAllInitiateKeygen: func(e *fsm.Event) {
				ki.Lock()
				defer ki.Unlock()
				//TODO: Take care of case where this is called by end in t1
				// send all KEGENSends to  respective nodes
				for i := int(startingIndex.Int64()); i < numOfKeys+int(startingIndex.Int64()); i++ {
					keyIndex := big.NewInt(int64(i))
					committedSecrets := ki.Secrets[keyIndex.Text(16)]
					ki.Store.StoreKEYGENSecret(*keyIndex, committedSecrets)
					for k := range ki.NodeLog {
						nodeIndex := big.Int{}
						nodeIndex.SetString(k, 16)

						keygenSend := KEYGENSend{
							KeyIndex: *keyIndex,
							AIY:      pvss.EvaluateBivarPolyAtX(committedSecrets.f, nodeIndex),
							AIprimeY: pvss.EvaluateBivarPolyAtX(committedSecrets.fprime, nodeIndex),
							BIX:      pvss.EvaluateBivarPolyAtY(committedSecrets.f, nodeIndex),
							BIprimeX: pvss.EvaluateBivarPolyAtY(committedSecrets.fprime, nodeIndex),
						}
						//send to node
						err := ki.Transport.SendKEYGENSend(keygenSend, nodeIndex)
						if err != nil {
							//TODO: Resend
							logging.Errorf("NODE"+ki.NodeIndex.Text(16)+"Could not send KEYGENSend : %s", err)
						}
					}
				}
			},
			"after_" + EIAllSubsharesDone: func(e *fsm.Event) {
				ki.Lock()
				defer ki.Unlock()
				//Here we broadcast KEYGENShareComplete
				keygenShareCompletes := make([]KEYGENShareComplete, ki.NumOfKeys)
				for i := 0; i < ki.NumOfKeys; i++ {
					keyIndex := big.Int{}
					keyIndex.SetInt64(int64(i)).Add(&keyIndex, &ki.StartIndex)
					// form perfect Si
					si := big.NewInt(int64(0))
					siprime := big.NewInt(int64(0))

					for nodeIndex, _ := range ki.NodeLog {
						// add up subshares for qualified set
						v := ki.KeyLog[keyIndex.Text(16)][nodeIndex]
						si.Add(si, &v.ReceivedSend.AIY.Coeff[0])
						siprime.Add(siprime, &v.ReceivedSend.AIprimeY.Coeff[0])
					}
					c, u1, u2, gs, gshr := pvss.GenerateNIZKPKWithCommitments(*si, *siprime)
					ki.Store.StoreCompletedShare(keyIndex, *si, *siprime)
					keygenShareCompletes[i] = KEYGENShareComplete{
						KeyIndex: keyIndex,
						c:        c,
						u1:       u1,
						u2:       u2,
						gsi:      gs,
						gsihr:    gshr,
					}
				}
				err := ki.Transport.BroadcastKEYGENShareComplete(keygenShareCompletes)
				if err != nil {
					logging.Errorf("NODE"+ki.NodeIndex.Text(16)+"Could not BroadcastKEYGENShareComplete: %s", err)
				}
			},
			"enter_" + SIKeygenCompleted: func(e *fsm.Event) {
				ki.ComChannel <- SIKeygenCompleted
			},
		},
	)

	// Node Log tracks the state of other nodes involved in this Keygen phase
	for _, nodeIndex := range nodeIndexes {
		ki.NodeLog[nodeIndex.Text(16)] = fsm.NewFSM(
			SNStandby,
			fsm.Events{
				{Name: ENInitiateKeygen, Src: []string{SNStandby}, Dst: SNKeygening},
				{Name: ENValidShares, Src: []string{SNKeygening}, Dst: SNQualifiedNode},
				{Name: ENFailedRoundOne, Src: []string{SNStandby}, Dst: SNUnqualifiedNode},
				{Name: ENFailedRoundTwo, Src: []string{SNStandby, SNKeygening}, Dst: SNUnqualifiedNode},
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
						if v.Current() == SNKeygening {
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
				"after_" + ENValidShares: func(e *fsm.Event) {
					ki.Lock()
					defer ki.Unlock()
					// See if all SharesCompleted are in
					logging.Debugf("NODE"+ki.NodeIndex.Text(16)+"Valid ShareCompleted in %s", ki.NodeIndex.Text(16))
					counter := 0
					for _, v := range ki.NodeLog {
						if v.Current() == SNQualifiedNode {
							counter++
						}
					}
					if counter == len(ki.NodeLog) {
						go func() {
							err := ki.State.Event(EIAllKeygenCompleted)
							if err != nil {
								logging.Errorf("NODE"+ki.NodeIndex.Text(16)+"Could not %s. Err: %s", EIAllKeygenCompleted, err)
							}
						}()
					}
				},
			},
		)
	}

	ki.KeyLog = make(map[string](map[string]*KEYGENLog))
	ki.Secrets = make(map[string]KEYGENSecrets)

	// prepare commitmentMatrixes for broadcast
	commitmentMatrixes := make([][][]common.Point, ki.NumOfKeys)
	for i := 0; i < numOfKeys; i++ {
		//help initialize all the keylogs
		index := big.NewInt(int64(i))
		index.Add(index, &ki.StartIndex)
		ki.KeyLog[index.Text(16)] = make(map[string]*KEYGENLog)
		secret := *pvss.RandomBigInt()
		f := pvss.GenerateRandomBivariatePolynomial(secret, threshold)
		fprime := pvss.GenerateRandomBivariatePolynomial(*pvss.RandomBigInt(), threshold)
		commitmentMatrixes[i] = pvss.GetCommitmentMatrix(f, fprime)

		// store secrets
		keyIndex := big.NewInt(int64(i))
		keyIndex.Add(keyIndex, &startingIndex)
		ki.Secrets[keyIndex.Text(16)] = KEYGENSecrets{
			secret: secret,
			f:      f,
			fprime: fprime,
		}
	}

	//TODO: Trigger setting up of listeners here
	err := ki.Transport.BroadcastInitiateKeygen(commitmentMatrixes)
	if err != nil {
		return err
	}

	//TODO: We neet to set a timing (t1) here (Deprecated)
	// Changed to tendermint triggering timing

	return nil
}

func (ki *KeygenInstance) OnInitiateKeygen(commitmentMatrixes [][][]common.Point, nodeIndex big.Int) error {
	ki.Lock()
	defer ki.Unlock()
	// Only accept onInitiate on Standby phase to only accept initiate keygen once from one node index
	if ki.NodeLog[nodeIndex.Text(16)].Current() == SNStandby {
		// check length of commitment matrix is right
		if len(commitmentMatrixes) != ki.NumOfKeys {
			return errors.New("length of  commitment matrix is not correct")
		}
		// store commitment matrix
		for i, commitmentMatrix := range commitmentMatrixes {
			index := big.NewInt(int64(i))
			index.Add(index, &ki.StartIndex)
			//TODO: create state to handle time out of t2
			ki.KeyLog[index.Text(16)][nodeIndex.Text(16)] = &KEYGENLog{
				KeyIndex:               *index,
				NodeIndex:              nodeIndex,
				C:                      commitmentMatrix,
				ReceivedEchoes:         make(map[string]KEYGENEcho),          // From(M) big.Int (in hex) to Echo
				ReceivedReadys:         make(map[string]KEYGENReady),         // From(M) big.Int (in hex) to Ready
				ReceivedShareCompletes: make(map[string]KEYGENShareComplete), // From(M) big.Int (in hex) to ShareComplete
				SubshareState: fsm.NewFSM(
					SKWaitingForSends,
					fsm.Events{
						{Name: EKSendEcho, Src: []string{SKWaitingForSends}, Dst: SKWaitingForEchos},
						{Name: EKSendReady, Src: []string{SKWaitingForEchos, SKEchoReconstructing}, Dst: SKWaitingForReadys},
						{Name: EKTReachedSubshare, Src: []string{SKWaitingForReadys}, Dst: SKValidSubshare},
						{Name: EKAllReachedSubshare, Src: []string{SKValidSubshare}, Dst: SKPerfectSubshare},
						{Name: EKEchoReconstruct, Src: []string{SKWaitingForSends}, Dst: SKEchoReconstructing},
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
								keyIndex := big.Int{}
								dealer := big.Int{}
								nodeToSendIndex.SetString(k, 16)
								keyIndex.SetString(e.Args[0].(string), 16)
								dealer.SetString(e.Args[1].(string), 16)
								msg := ki.KeyLog[e.Args[0].(string)][e.Args[1].(string)].ReceivedSend
								keygenEcho := KEYGENEcho{
									KeyIndex: keyIndex,
									Dealer:   dealer,
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
								keygenReady := KEYGENReady{
									KeyIndex: *index,
									Dealer:   nodeIndex,
									Aij:      *pvss.PolyEval(ki.KeyLog[keyIndexStr][dealerStr].ReceivedSend.AIY, nodeToSendIndex),
									Aprimeij: *pvss.PolyEval(ki.KeyLog[keyIndexStr][dealerStr].ReceivedSend.AIprimeY, nodeToSendIndex),
									Bij:      *pvss.PolyEval(ki.KeyLog[keyIndexStr][dealerStr].ReceivedSend.BIX, nodeToSendIndex),
									Bprimeij: *pvss.PolyEval(ki.KeyLog[keyIndexStr][dealerStr].ReceivedSend.BIprimeX, nodeToSendIndex),
								}
								err := ki.Transport.SendKEYGENReady(keygenReady, nodeToSendIndex)
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
							ki.SubsharesComplete = ki.SubsharesComplete + 1
							logging.Debugf("NODE"+ki.NodeIndex.Text(16)+"Count of Perfect: %v", ki.SubsharesComplete)
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
							logging.Debug("Echo Reconstruct is called 2")
							count := 0
							// prepare for derivation of polynomial
							aiyPoints := make([]common.Point, ki.Threshold)
							aiprimeyPoints := make([]common.Point, ki.Threshold)
							bixPoints := make([]common.Point, ki.Threshold)
							biprimexPoints := make([]common.Point, ki.Threshold)

							for k, v := range ki.KeyLog[index.Text(16)][nodeIndex.Text(16)].ReceivedEchoes {
								if count <= ki.Threshold {
									fromNodeInt := big.Int{}
									fromNodeInt.SetString(k, 16)
									aiyPoints[count] = common.Point{X: fromNodeInt, Y: v.Aij}
									aiprimeyPoints[count] = common.Point{X: fromNodeInt, Y: v.Aprimeij}
									bixPoints[count] = common.Point{X: fromNodeInt, Y: v.Bij}
									biprimexPoints[count] = common.Point{X: fromNodeInt, Y: v.Bprimeij}
								}
								count++
							}

							aiy := pvss.LagrangeInterpolatePolynomial(aiyPoints)
							aiprimey := pvss.LagrangeInterpolatePolynomial(aiprimeyPoints)
							bix := pvss.LagrangeInterpolatePolynomial(bixPoints)
							biprimex := pvss.LagrangeInterpolatePolynomial(biprimexPoints)
							ki.KeyLog[index.Text(16)][nodeIndex.Text(16)].ReceivedSend = KEYGENSend{
								KeyIndex: *index,
								AIY:      common.PrimaryPolynomial{Threshold: ki.Threshold, Coeff: aiy},
								AIprimeY: common.PrimaryPolynomial{Threshold: ki.Threshold, Coeff: aiprimey},
								BIX:      common.PrimaryPolynomial{Threshold: ki.Threshold, Coeff: bix},
								BIprimeX: common.PrimaryPolynomial{Threshold: ki.Threshold, Coeff: biprimex},
							}
							logging.Debug("Echo Reconstruct is called 3")
							// Now we send our readys
							go func(keyLog *KEYGENLog) {
								logging.Debugf("Echo Reconstruct is called 4 %v", keyLog.SubshareState.Current())
								err := keyLog.SubshareState.Event(EKSendReady)
								if err != nil {
									logging.Errorf("NODE"+ki.NodeIndex.Text(16)+"Could not change subshare state: %s", err)
								}
							}(ki.KeyLog[index.Text(16)][nodeIndex.Text(16)])
						},
						// Below functions are for the state machines to catch up on previously sent messages using
						// MsgBuffer We dont need to lock as msg buffer should do so
						"enter_" + SKWaitingForSends: func(e *fsm.Event) {
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

		go func(nodeLog *fsm.FSM) {
			err := nodeLog.Event(ENInitiateKeygen)
			if err != nil {
				logging.Error(err.Error())
			}
		}(ki.NodeLog[nodeIndex.Text(16)])
	}
	return nil
}

func (ki *KeygenInstance) OnKEYGENSend(msg KEYGENSend, fromNodeIndex big.Int) error {
	ki.Lock()
	defer ki.Unlock()
	keyLog := ki.KeyLog[msg.KeyIndex.Text(16)][fromNodeIndex.Text(16)]
	if keyLog.SubshareState.Current() == SKWaitingForSends {
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

		//since valid we log
		// workaround https://github.com/golang/go/issues/3117
		var tmp = ki.KeyLog[msg.KeyIndex.Text(16)][fromNodeIndex.Text(16)]
		tmp.ReceivedSend = msg
		ki.KeyLog[msg.KeyIndex.Text(16)][fromNodeIndex.Text(16)] = tmp
		keyLog = ki.KeyLog[msg.KeyIndex.Text(16)][fromNodeIndex.Text(16)]

		// and send echo
		go func(innerKeyLog *KEYGENLog, keyIndex string, from string) {
			err := innerKeyLog.SubshareState.Event(EKSendEcho, keyIndex, from)
			if err != nil {
				logging.Error(err.Error())
			}
		}(keyLog, msg.KeyIndex.Text(16), fromNodeIndex.Text(16))

	} else {
		ki.MsgBuffer.StoreKEYGENSend(msg, fromNodeIndex)
	}
	return nil
}

func (ki *KeygenInstance) OnKEYGENEcho(msg KEYGENEcho, fromNodeIndex big.Int) error {
	ki.Lock()
	defer ki.Unlock()
	keyLog := ki.KeyLog[msg.KeyIndex.Text(16)][msg.Dealer.Text(16)]

	if ki.State.Current() == SIRunningKeygen {
		//verify echo, if correct log echo. If there are more then threshold Echos we send ready
		if !pvss.AVSSVerifyPoint(
			keyLog.C,
			fromNodeIndex,
			ki.NodeIndex,
			msg.Aij,
			msg.Aprimeij,
			msg.Bij,
			msg.Bprimeij,
		) {
			//TODO: potentially invalidate nodes here
			return errors.New(fmt.Sprintf("KEYGENEcho not valid to declared commitments. From: %s To: %s KEYGENEcho: %+v", fromNodeIndex.Text(16), ki.NodeIndex.Text(16), msg))
		}

		//log echo
		keyLog.ReceivedEchoes[fromNodeIndex.Text(16)] = msg

		// check for echos
		ratio := int(math.Ceil((float64(ki.TotalNodes) + float64(ki.NumMalNodes) + 1.0) / 2.0)) // this is just greater equal than (differs from AVSS Paper equals) because we fsm to only send ready at most once
		if ki.Threshold <= len(keyLog.ReceivedEchoes) && ratio <= len(keyLog.ReceivedEchoes) {
			//since threshoold and above

			// we etiher send ready
			if keyLog.SubshareState.Is(SKWaitingForEchos) {
				go func(innerKeyLog *KEYGENLog, keyIndex string, dealer string) {
					err := innerKeyLog.SubshareState.Event(EKSendReady, keyIndex, dealer)
					if err != nil {
						logging.Error(err.Error())
					}
				}(keyLog, msg.KeyIndex.Text(16), msg.Dealer.Text(16))
			}
			// Or here we cater for reconstruction in the case of malcious nodes refusing to send KEYGENSend
			if keyLog.SubshareState.Is(SKWaitingForSends) {
				go func(innerKeyLog *KEYGENLog, keyIndex string, dealer string) {
					logging.Debug("Echo Reconstruct is called 1")
					err := innerKeyLog.SubshareState.Event(EKEchoReconstruct, keyIndex, dealer)
					if err != nil {
						logging.Error(err.Error())
					}
				}(keyLog, msg.KeyIndex.Text(16), msg.Dealer.Text(16))
			}
		}
	} else {
		ki.MsgBuffer.StoreKEYGENEcho(msg, fromNodeIndex)
	}
	return nil
}

func (ki *KeygenInstance) OnKEYGENReady(msg KEYGENReady, fromNodeIndex big.Int) error {
	ki.Lock()
	defer ki.Unlock()
	if ki.State.Current() == SIRunningKeygen {
		keyLog := ki.KeyLog[msg.KeyIndex.Text(16)][msg.Dealer.Text(16)]
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
			//TODO: potentially invalidate nodes here
			return errors.New(fmt.Sprintf("KEYGENReady not valid to declared commitments. From: %s To: %s KEYGENEcho: %v+", fromNodeIndex.Text(16), ki.NodeIndex.Text(16), msg))
		}

		//log ready
		keyLog.ReceivedReadys[fromNodeIndex.Text(16)] = msg

		// if we've reached the required number of readys
		if ki.Threshold <= len(keyLog.ReceivedReadys) {
			logging.Errorf("NODE"+ki.NodeIndex.Text(16)+" We're in threshold %v", len(keyLog.ReceivedReadys))
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

func (ki *KeygenInstance) OnKEYGENShareComplete(keygenShareCompletes []KEYGENShareComplete, fromNodeIndex big.Int) error {
	ki.Lock()
	defer ki.Unlock()
	//verify shareCompletes
	logging.Debugf("NODE"+ki.NodeIndex.Text(16)+"For KkeygenShareComplete %s", fromNodeIndex.Text(16))
	for i, keygenShareCom := range keygenShareCompletes {
		// ensure valid keyindex
		expectedKeyIndex := big.NewInt(int64(i))
		expectedKeyIndex.Add(expectedKeyIndex, &ki.StartIndex)

		if expectedKeyIndex.Cmp(&keygenShareCom.KeyIndex) != 0 {
			logging.Debugf("NODE "+ki.NodeIndex.Text(16)+" KeyIndex %s, Expected %s", keygenShareCom.KeyIndex.Text(16), expectedKeyIndex.Text(16))
			return errors.New("Faulty key index on OnKEYGENShareComplete")

		}
		// by first verifying NIZKPK Proof
		if !pvss.VerifyNIZKPK(keygenShareCom.c, keygenShareCom.u1, keygenShareCom.u2, keygenShareCom.gsi, keygenShareCom.gsihr) {
			return errors.New("Faulty NIZKPK Proof on OnKEYGENShareComplete")
		}

		// add up all commitments
		var sumCommitments [][]common.Point
		//TODO: Potentially quite intensive
		logging.Debugf("NODE"+ki.NodeIndex.Text(16)+"nodelog length %v", len(ki.NodeLog))
		for nodeIndex, _ := range ki.NodeLog {
			keyLog := ki.KeyLog[keygenShareCom.KeyIndex.Text(16)][nodeIndex]
			if len(sumCommitments) == 0 {
				sumCommitments = keyLog.C
			} else {
				sumCommitments, _ = pvss.AVSSAddCommitment(sumCommitments, keyLog.C)
				// if err != nil {
				// 	return err
				// }
			}
		}

		//test commmitment
		if !pvss.AVSSVerifyShareCommitment(sumCommitments, fromNodeIndex, keygenShareCom.gsihr) {
			return errors.New("Faulty Share Commitment OnKEYGENShareComplete")
		}
	}

	// we get here if everything passes
	go func(nodeLog *fsm.FSM) {
		nodeLog.Event(ENValidShares)
	}(ki.NodeLog[fromNodeIndex.Text(16)])

	// gshr should be a point on the sum commitment matix
	return nil
}
