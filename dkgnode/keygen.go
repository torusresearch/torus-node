package dkgnode

import (
	"errors"
	"fmt"
	"math/big"

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

	//Implementing the Code below will allow KEYGEN to run
	// "Client" Actions
	broadcastInitiateKeygen(commitmentMatrixes [][][]common.Point) error
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
	NodeIndex  big.Int
	Threshold  int
	State      *fsm.FSM
	NodeLog    map[string]*fsm.FSM               // nodeindex => fsm
	KeyLog     map[string](map[string]KEYGENLog) // keyindex => nodeindex => log
	StartIndex big.Int
	NumOfKeys  int
	Secrets    map[string]KEYGENSecrets // keyindex => KEYGENSecrets
}

// KEYGEN STATES (SK)
const (
	// Internal States
	SKWaitingInitiateKeygen = "waiting_initiate_keygen"
	SKRunningKeygen         = "running_keygen"

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
func (ki *KeygenInstance) InitiateKeygen(startingIndex big.Int, numOfKeys int, nodeIndexes []big.Int, threshold int, nodeIndex big.Int) error {
	ki.NodeIndex = nodeIndex
	ki.Threshold = threshold
	ki.StartIndex = startingIndex
	ki.NumOfKeys = numOfKeys
	// We start initiate keygen state at waiting_initiate_keygen
	ki.State = fsm.NewFSM(
		SKWaitingInitiateKeygen,
		fsm.Events{
			{Name: EKAllInitiateKeygen, Src: []string{SKWaitingInitiateKeygen}, Dst: SKRunningKeygen},
			{Name: "", Src: []string{SKRunningKeygen}, Dst: SKRunningKeygen},
		},
		fsm.Callbacks{
			"enter_state": func(e *fsm.Event) { logging.Debugf("STATUSTX: local status set from %s to %s", e.Src, e.Dst) },
			"after_" + EKAllInitiateKeygen: func(e *fsm.Event) {
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
						err := ki.sendKEYGENSend(keygenSend, nodeIndex)
						if err != nil {
							//TODO: Resend
							logging.Errorf("Could not send KEYGENSend : %s", err)
						}
					}
				}
			},
		},
	)

	// Node Log tracks the state of other nodes involved in this Keygen phase
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
	ki.Secrets = make(map[string]KEYGENSecrets)

	// prepare commitmentMatrixes for broadcast
	var commitmentMatrixes [][][]common.Point
	for i := 0; i < numOfKeys; i++ {
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
	err := ki.broadcastInitiateKeygen(commitmentMatrixes)
	if err != nil {
		return err
	}
	//TODO: We neet to set a timing (t1) here
	//TODO: Trigger setting up of listeners here
	return nil
}

func (ki *KeygenInstance) onInitiateKeygen(commitmentMatrixes [][][]common.Point, nodeIndex big.Int) error {
	// Only accept onInitiate on Standby phase to only accept initiate keygen once from one node index
	if ki.NodeLog[nodeIndex.Text(16)].Current() == SKStandby {
		// check length of commitment matrix is right
		if len(commitmentMatrixes) != ki.NumOfKeys {
			return errors.New("length of  commitment matrix is not correct")
		}
		// store commitment matrix
		for i, commitmentMatrix := range commitmentMatrixes {
			index := big.NewInt(int64(i))
			index.Add(index, &ki.StartIndex)
			ki.KeyLog[index.Text(16)][nodeIndex.Text(16)] = KEYGENLog{
				KeyIndex:               *index,
				NodeIndex:              nodeIndex,
				C:                      commitmentMatrix,
				ReceivedEchoes:         make(map[string]KEYGENEcho),          // From(M) big.Int (in hex) to Echo
				ReceivedReadys:         make(map[string]KEYGENReady),         // From(M) big.Int (in hex) to Ready
				ReceivedShareCompletes: make(map[string]KEYGENShareComplete), // From(M) big.Int (in hex) to ShareComplete
			}
		}

	}
	return nil
}

func (ki *KeygenInstance) onKEYGENSend(msg KEYGENSend, fromNodeIndex big.Int) error {
	if ki.State.Current() == SKRunningKeygen {
		// we verify keygen, if valid we log it here. Then we send an echo
		if !pvss.AVSSVerifyPoly(
			ki.KeyLog[msg.KeyIndex.Text(16)][fromNodeIndex.Text(16)].C,
			ki.NodeIndex,
			msg.AIY,
			msg.AIprimeY,
			msg.BIX,
			msg.BIprimeX,
		) {
			return errors.New(fmt.Sprintf("KEYGENSend not valid to declared commitments. From: %s To: %s KEYGENSend: %s", fromNodeIndex.Text(16), ki.NodeIndex.Text(16), msg))
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
			ki.sendKEYGENEcho(keygenEcho, nodeToSendIndex)
		}
	}
	return nil
}
func (ki *KeygenInstance) onKEYGENEcho(msg KEYGENEcho, fromNodeIndex big.Int) error {
	if ki.State.Current() == SKRunningKeygen {
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
			return errors.New(fmt.Sprintf("KEYGENEcho not valid to declared commitments. From: %s To: %s KEYGENEcho: %s", fromNodeIndex.Text(16), ki.NodeIndex.Text(16), msg))
		}

		//log echo
		ki.KeyLog[msg.KeyIndex.Text(16)][msg.Dealer.Text(16)].ReceivedEchoes[fromNodeIndex.Text(16)] = msg

		// check for echos
		if ki.Threshold <= len(ki.KeyLog[msg.KeyIndex.Text(16)][msg.Dealer.Text(16)].ReceivedEchoes) {
			//since threshould and above we send ready

		}
	}
	return nil
}
func (ki *KeygenInstance) onKEYGENReady(msg KEYGENReady, fromNodeIndex big.Int) error {
	return nil
}
func (ki *KeygenInstance) onKEYGENShareComplete(msg KEYGENShareComplete, fromNodeIndex big.Int) error {
	return nil
}

//TODO: Initiate our client functions here before anything else is called (or perhaps even before initiate is called)
func (ki *KeygenInstance) broadcastInitiateKeygen(commitmentMatrixes [][][]common.Point) error {
	return nil
}
func (ki *KeygenInstance) sendKEYGENSend(msg KEYGENSend, nodeIndex big.Int) error {
	return nil
}
func (ki *KeygenInstance) sendKEYGENEcho(msg KEYGENEcho, nodeIndex big.Int) error {
	return nil
}
func (ki *KeygenInstance) sendKEYGENReady(msg KEYGENReady, nodeIndex big.Int) error {
	return nil
}
func (ki *KeygenInstance) broadcastKEYGENShareComplete(msg KEYGENShareComplete, nodeIndex big.Int) error {
	return nil
}
