package keygen

import (
	"errors"
	"fmt"
	"math/big"
	"sync"

	"github.com/torusresearch/torus-public/pvss"

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

type KEYGENShareComplete struct {
	KeyIndex big.Int
	c        big.Int
	u1       big.Int
	u2       big.Int
	gsi      common.Point
	gsihr    common.Point
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
	InitiateKeygen(startingIndex big.Int, endingIndex big.Int, nodeIndexes []big.Int, threshold int) error

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

// Main Keygen Struct
type KeygenInstance struct {
	sync.Mutex
	NodeIndex         big.Int
	Threshold         int
	State             *fsm.FSM
	NodeLog           map[string]*fsm.FSM               // nodeindex => fsm
	KeyLog            map[string](map[string]KEYGENLog) // keyindex => nodeindex => log
	StartIndex        big.Int
	NumOfKeys         int
	Secrets           map[string]KEYGENSecrets // keyindex => KEYGENSecrets
	SubsharesComplete int                      // We keep a count of number of subshares that are fully complete to avoid checking on every iteration
	Transport         AVSSKeygenTransport
}

// KEYGEN STATES (SK)
const (
	// State - Internal
	SIWaitingInitiateKeygen      = "waiting_initiate_keygen"
	SIRunningKeygen              = "running_keygen"
	SIWaitingKEYGENShareComplete = "waiting_keygen_share_complete"
	SIKeygenCompleted            = "keygen_completed"

	// For State - node log
	SNStandby       = "standby"
	SNKeygening     = "keygening"
	SNQualifiedNode = "qualified_node"

	// State - KeyLog
	SKWaitingForEchos  = "waiting_for_echos"
	SKWaitingForReadys = "waiting_for_readys"
	SKValidSubshare    = "valid_subshare"
	SKPerfectSubshare  = "perfect_subshare"
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

	// Events - KeyLog
	EKSendReady          = "send_ready"
	EKTReachedSubshare   = "t_reached_subshare"
	EKAllReachedSubshare = "all_reached_subshare"
)

//TODO: Potentially Stuff specific KEYGEN Debugger | set up transport here as well
func (ki *KeygenInstance) InitiateKeygen(startingIndex big.Int, numOfKeys int, nodeIndexes []big.Int, threshold int, nodeIndex big.Int) error {
	ki.Lock()
	defer ki.Unlock()
	ki.NodeIndex = nodeIndex
	ki.Threshold = threshold
	ki.StartIndex = startingIndex
	ki.NumOfKeys = numOfKeys
	ki.SubsharesComplete = 0
	ki.NodeLog = make(map[string]*fsm.FSM)
	// We start initiate keygen state at waiting_initiate_keygen
	ki.State = fsm.NewFSM(
		SIWaitingInitiateKeygen,
		fsm.Events{
			{Name: EIAllInitiateKeygen, Src: []string{SIWaitingInitiateKeygen}, Dst: SIRunningKeygen},
			{Name: EIAllSubsharesDone, Src: []string{SIRunningKeygen}, Dst: SIWaitingKEYGENShareComplete},
			{Name: EIAllKeygenCompleted, Src: []string{SIWaitingKEYGENShareComplete}, Dst: SIKeygenCompleted},
		},
		fsm.Callbacks{
			"enter_state": func(e *fsm.Event) { logging.Debugf("KEYGEN: state transition from %s to %s", e.Src, e.Dst) },
			"after_" + EIAllInitiateKeygen: func(e *fsm.Event) {
				//TODO: Take care of case where this is called by end in t1
				// send all KEGENSends to  respective nodes
				for i := int(startingIndex.Int64()); i < numOfKeys+int(startingIndex.Int64()); i++ {
					keyIndex := big.NewInt(int64(i))
					committedSecrets := ki.Secrets[keyIndex.Text(16)]
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
							logging.Errorf("Could not send KEYGENSend : %s", err)
						}
					}
				}
			},
			"after_" + EIAllSubsharesDone: func(e *fsm.Event) {
				//Here we broadcast KEYGENShareComplete
				keygenShareCompletes := make([]KEYGENShareComplete, len(ki.NodeLog))
				for i := 0; i < ki.NumOfKeys; i++ {
					keyIndex := big.Int{}
					keyIndex.SetInt64(int64(i)).Add(&keyIndex, &ki.StartIndex)
					// form perfect Si
					si := big.NewInt(int64(0))
					siprime := big.NewInt(int64(0))
					// just a check for the right number of subshares
					if len(ki.KeyLog[keyIndex.Text(16)]) != len(ki.NodeLog) {
						logging.Errorf("Not correct number of subshares found for: keyindex %s, Expected %s Actual %s", keyIndex.Text(16), len(ki.NodeLog), len(ki.KeyLog[keyIndex.Text(16)]))
					}
					for _, v := range ki.KeyLog[keyIndex.Text(16)] {
						// add up subshares
						si.Add(si, &v.ReceivedSend.AIY.Coeff[0])
						siprime.Add(siprime, &v.ReceivedSend.AIprimeY.Coeff[0])
					}
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
				err := ki.Transport.BroadcastKEYGENShareComplete(keygenShareCompletes)
				if err != nil {
					logging.Errorf("Could not BroadcastKEYGENShareComplete: %s", err)
				}
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
			},
			fsm.Callbacks{
				"enter_state": func(e *fsm.Event) { logging.Debugf("NodeLog State changed from %s to %s", e.Src, e.Dst) },
				"after_" + ENInitiateKeygen: func(e *fsm.Event) {

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
								logging.Errorf("Could not %s. Err: %s", EIAllInitiateKeygen, err)
							}
						}()
					}
				},
				"after_" + ENValidShares: func(e *fsm.Event) {
					// See if all SharesCompleted are in
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
								logging.Errorf("Could not %s. Err: %s", EIAllInitiateKeygen, err)
							}
						}()
					}
				},
			},
		)
	}

	ki.KeyLog = make(map[string](map[string]KEYGENLog))
	ki.Secrets = make(map[string]KEYGENSecrets)

	// prepare commitmentMatrixes for broadcast
	commitmentMatrixes := make([][][]common.Point, ki.NumOfKeys)
	for i := 0; i < numOfKeys; i++ {
		//help initialize all the keylogs
		index := big.NewInt(int64(i))
		index.Add(index, &ki.StartIndex)
		ki.KeyLog[index.Text(16)] = make(map[string]KEYGENLog)
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
	err := ki.Transport.BroadcastInitiateKeygen(commitmentMatrixes)
	if err != nil {
		return err
	}
	//TODO: We neet to set a timing (t1) here
	//TODO: Trigger setting up of listeners here
	return nil
}

func (ki *KeygenInstance) OnInitiateKeygen(commitmentMatrixes [][][]common.Point, nodeIndex big.Int) error {
	ki.Lock()
	defer ki.Unlock()
	logging.Debug("On initiate keygen called")
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
			ki.KeyLog[index.Text(16)][nodeIndex.Text(16)] = KEYGENLog{
				KeyIndex:               *index,
				NodeIndex:              nodeIndex,
				C:                      commitmentMatrix,
				ReceivedEchoes:         make(map[string]KEYGENEcho),          // From(M) big.Int (in hex) to Echo
				ReceivedReadys:         make(map[string]KEYGENReady),         // From(M) big.Int (in hex) to Ready
				ReceivedShareCompletes: make(map[string]KEYGENShareComplete), // From(M) big.Int (in hex) to ShareComplete
				SubshareState: fsm.NewFSM(
					SKWaitingForEchos,
					fsm.Events{
						{Name: EKSendReady, Src: []string{SKWaitingForEchos}, Dst: SKWaitingForReadys},
						{Name: EKTReachedSubshare, Src: []string{SKWaitingForReadys}, Dst: SKValidSubshare},
						{Name: EKAllReachedSubshare, Src: []string{SKValidSubshare}, Dst: SKPerfectSubshare},
					},
					fsm.Callbacks{
						"enter_state": func(e *fsm.Event) { logging.Debugf("subshare set from %s to %s", e.Src, e.Dst) },
						"after_" + EKSendReady: func(e *fsm.Event) {
							// we send readys here when we have collected enough echos
							for k := range ki.NodeLog {
								nodeToSendIndex := big.Int{}
								nodeToSendIndex.SetString(k, 16)
								keyIndex := big.Int{}
								dealer := big.Int{}
								keyIndex.SetString(e.Args[0].(string), 16)
								dealer.SetString(e.Args[1].(string), 16)
								keygenReady := KEYGENReady{
									KeyIndex: keyIndex,
									Dealer:   dealer,
									Aij:      *pvss.PolyEval(ki.KeyLog[e.Args[0].(string)][e.Args[1].(string)].ReceivedSend.AIY, nodeToSendIndex),
									Aprimeij: *pvss.PolyEval(ki.KeyLog[e.Args[0].(string)][e.Args[1].(string)].ReceivedSend.AIprimeY, nodeToSendIndex),
									Bij:      *pvss.PolyEval(ki.KeyLog[e.Args[0].(string)][e.Args[1].(string)].ReceivedSend.BIX, nodeToSendIndex),
									Bprimeij: *pvss.PolyEval(ki.KeyLog[e.Args[0].(string)][e.Args[1].(string)].ReceivedSend.BIprimeX, nodeToSendIndex),
								}
								err := ki.Transport.SendKEYGENReady(keygenReady, nodeToSendIndex)
								if err != nil {
									// TODO: Handle failure, resend?
									logging.Errorf("Could not sent KEYGENReady %s", err)
								}
							}
						},
						"after_" + EKAllReachedSubshare: func(e *fsm.Event) {
							ki.SubsharesComplete++
							// Check if all subshares are complete
							if ki.SubsharesComplete == ki.NumOfKeys*len(ki.NodeLog) {
								go func() {
									// end keygen
									err := ki.State.Event(EIAllSubsharesDone)
									if err != nil {
										logging.Errorf("Could not change state to subshare done: %s", err)
									}
								}()
							}
						},
					},
				),
			}
		}
		fmt.Printf("NodeLog State changed initiate keygen is called ")
		err := ki.NodeLog[nodeIndex.Text(16)].Event(ENInitiateKeygen)
		if err != nil {
			return err
		}
	}
	return nil
}

func (ki *KeygenInstance) OnKEYGENSend(msg KEYGENSend, fromNodeIndex big.Int) error {
	ki.Lock()
	defer ki.Unlock()
	if ki.State.Current() == SIRunningKeygen {
		// we verify keygen, if valid we log it here. Then we send an echo
		if !pvss.AVSSVerifyPoly(
			ki.KeyLog[msg.KeyIndex.Text(16)][fromNodeIndex.Text(16)].C,
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
		// and send echo
		for k := range ki.NodeLog {
			nodeToSendIndex := big.Int{}
			nodeToSendIndex.SetString(k, 16)
			keygenEcho := KEYGENEcho{
				KeyIndex: msg.KeyIndex,
				Dealer:   fromNodeIndex,
				Aij:      *pvss.PolyEval(msg.AIY, nodeToSendIndex),
				Aprimeij: *pvss.PolyEval(msg.AIprimeY, nodeToSendIndex),
				Bij:      *pvss.PolyEval(msg.BIX, nodeToSendIndex),
				Bprimeij: *pvss.PolyEval(msg.BIprimeX, nodeToSendIndex),
			}
			err := ki.Transport.SendKEYGENEcho(keygenEcho, nodeToSendIndex)
			if err != nil {
				// TODO: Handle failure, resend?
				return err
			}
		}
	}
	return nil
}
func (ki *KeygenInstance) OnKEYGENEcho(msg KEYGENEcho, fromNodeIndex big.Int) error {
	ki.Lock()
	defer ki.Unlock()
	if ki.State.Current() == SIRunningKeygen {
		//verify echo, if correct log echo. If there are more then threshold Echos we send ready
		if !pvss.AVSSVerifyPoint(
			ki.KeyLog[msg.KeyIndex.Text(16)][msg.Dealer.Text(16)].C,
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
		ki.KeyLog[msg.KeyIndex.Text(16)][msg.Dealer.Text(16)].ReceivedEchoes[fromNodeIndex.Text(16)] = msg

		// check for echos
		if ki.Threshold <= len(ki.KeyLog[msg.KeyIndex.Text(16)][msg.Dealer.Text(16)].ReceivedEchoes) {
			//since threshoold and above we send ready
			if ki.KeyLog[msg.KeyIndex.Text(16)][msg.Dealer.Text(16)].SubshareState.Current() == SKWaitingForEchos {
				err := ki.KeyLog[msg.KeyIndex.Text(16)][msg.Dealer.Text(16)].SubshareState.Event(EKSendReady, msg.KeyIndex.Text(16), msg.Dealer.Text(16))
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (ki *KeygenInstance) OnKEYGENReady(msg KEYGENReady, fromNodeIndex big.Int) error {
	ki.Lock()
	defer ki.Unlock()
	if ki.State.Current() == SIRunningKeygen {
		// we verify ready, if right we log and check if we have enough readys to validate shares
		if !pvss.AVSSVerifyPoint(
			ki.KeyLog[msg.KeyIndex.Text(16)][msg.Dealer.Text(16)].C,
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
		ki.KeyLog[msg.KeyIndex.Text(16)][fromNodeIndex.Text(16)].ReceivedReadys[fromNodeIndex.Text(16)] = msg

		// if we've reached the required number of readys
		if ki.Threshold <= len(ki.KeyLog[msg.KeyIndex.Text(16)][fromNodeIndex.Text(16)].ReceivedReadys) {
			if ki.KeyLog[msg.KeyIndex.Text(16)][msg.Dealer.Text(16)].SubshareState.Current() == SKWaitingForReadys {
				err := ki.KeyLog[msg.KeyIndex.Text(16)][msg.Dealer.Text(16)].SubshareState.Event(EKTReachedSubshare, msg.KeyIndex.Text(16), msg.Dealer.Text(16))
				if err != nil {
					return err
				}
			}

			// if we've got all of the readys we classify share as perfect
			if len(ki.KeyLog[msg.KeyIndex.Text(16)][msg.Dealer.Text(16)].ReceivedReadys) == len(ki.NodeLog) {
				if ki.KeyLog[msg.KeyIndex.Text(16)][msg.Dealer.Text(16)].SubshareState.Current() == SKValidSubshare {
					err := ki.KeyLog[msg.KeyIndex.Text(16)][msg.Dealer.Text(16)].SubshareState.Event(EKAllReachedSubshare, msg.KeyIndex.Text(16), msg.Dealer.Text(16))
					if err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

func (ki *KeygenInstance) OnKEYGENShareComplete(keygenShareCompletes []KEYGENShareComplete, fromNodeIndex big.Int) error {
	ki.Lock()
	defer ki.Unlock()
	//verify shareCompletes
	for i, keygenShareCom := range keygenShareCompletes {
		// ensure valid keyindex
		expectedKeyIndex := big.NewInt(int64(i))
		expectedKeyIndex.Add(expectedKeyIndex, &ki.StartIndex)
		if expectedKeyIndex.Cmp(&keygenShareCom.KeyIndex) != 0 {
			return errors.New("Faulty key index on OnKEYGENShareComplete")
		}
		// by first verifying NIZKPK Proof
		if !pvss.VerifyNIZKPK(keygenShareCom.c, keygenShareCom.u1, keygenShareCom.u2, keygenShareCom.gsi, keygenShareCom.gsihr) {
			return errors.New("Faulty NIZKPK Proof on OnKEYGENShareComplete")
		}

		// add up all commitments
		var sumCommitments [][]common.Point
		//TODO: Potentially quite intensive
		for _, keylog := range ki.KeyLog[keygenShareCom.KeyIndex.Text(16)] {
			if len(sumCommitments) == 0 {
				sumCommitments = keylog.C
			} else {
				sumCommitments, _ = pvss.AVSSAddCommitment(sumCommitments, keylog.C)
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
	ki.NodeLog[fromNodeIndex.Text(16)].Event(ENValidShares)

	// gshr should be a point on the sum commitment matix
	return nil
}
