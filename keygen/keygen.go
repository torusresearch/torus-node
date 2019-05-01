package keygen

import (
	"errors"
	"fmt"
	"math"
	"math/big"
	"sort"
	"strconv"
	"time"

	"github.com/looplab/fsm"
	"github.com/torusresearch/bijson"
	"github.com/torusresearch/torus-public/common"
	"github.com/torusresearch/torus-public/idmutex"
	"github.com/torusresearch/torus-public/logging"
	"github.com/torusresearch/torus-public/pvss"
)

type KeyIndex string
type NodeIndex string

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
	C        big.Int
	U1       big.Int
	U2       big.Int
	Gsi      common.Point
	Gsihr    common.Point
}

type KEYGENDKGComplete struct {
	Proofs          []KEYGENShareComplete
	NodeSet         []string                                    // sorted nodeIndexes in Set
	ReadySignatures map[string](map[string](map[string][]byte)) // Keyindex => DealerIndex => NodeIndex (Who signed)
}

// KeyIndex => NodeIndex => KEYGENLog
type KEYGENLog struct {
	KeyIndex       big.Int
	NodeIndex      big.Int
	C              [][]common.Point       // big.Int (in hex) to Commitment matrix
	ReceivedSend   *KEYGENSend            // Polynomials for respective commitment matrix.
	ReceivedEchoes map[string]KEYGENEcho  // From(M) big.Int (in hex) to Echo
	ReceivedReadys map[string]KEYGENReady // From(M) big.Int (in hex) to Ready
	// ReceivedShareCompletes map[string]KEYGENShareComplete // From(M) big.Int (in hex) to ShareComplete
	SentEcho  bool // Tracking of sent ready
	SentReady bool // Tracking of sent ready
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
	StoreCompletedShare(keyIndex big.Int, si big.Int, siprime big.Int, publicKey common.Point) error
}
type AVSSAuth interface {
	Sign(msg string) ([]byte, error)
	Verify(text string, nodeIndex big.Int, signature []byte) bool
}

// Main Keygen Struct
type KeygenInstance struct {
	idmutex.Mutex
	NodeIndex            big.Int
	Threshold            int                                // in AVSS Paper this is k
	NumMalNodes          int                                // in AVSS Paper this is t
	TotalNodes           int                                // in AVSS Paper this is n
	NodeLog              map[string]int                     // nodeindex => count of PerfectSubshares
	UnqualifiedNodes     map[string]int                     // nodeindex => count of PerfectSubshares
	KeyLog               map[string](map[string]*KEYGENLog) // keyindex => nodeindex => log
	Secrets              map[string]KEYGENSecrets           // keyindex => KEYGENSecrets
	StartIndex           big.Int
	NumOfKeys            int
	SubsharesComplete    int // We keep a count of number of subshares that are fully complete to avoid checking on every iteration
	Transport            AVSSKeygenTransport
	Store                AVSSKeygenStorage
	MsgBuffer            KEYGENBuffer
	Auth                 AVSSAuth
	FinalNodeSet         []string                      // Final Nodeset Decided by the first valid DKGComplete
	ReceivedDKGCompleted map[string]*KEYGENDKGComplete // KeyIndex => KEYGENDKGComplete
	ComChannel           chan string
}

const retryBroadcastingKEYGENDKGComplete = 1
const retryEndingKeygen = 1
const retryKEYGENSend = 1
const readyPrefix = "mug"
const SIKeygenCompleted = "keygen_completed"

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
	ki.ReceivedDKGCompleted = make(map[string]*KEYGENDKGComplete)
	ki.NodeLog = make(map[string]int)
	ki.UnqualifiedNodes = make(map[string]int)
	ki.Transport = transport
	ki.Store = store
	ki.Auth = auth
	ki.ComChannel = comChannel
	// Initialize buffer
	ki.MsgBuffer = KEYGENBuffer{}
	ki.MsgBuffer.InitializeMsgBuffer(startingIndex, numOfKeys, nodeIndexes)

	// Node Log FSM tracks the state of other nodes involved in this Keygen phase
	for _, nodeIndex := range nodeIndexes {
		ki.NodeLog[nodeIndex.Text(16)] = 0
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
				ReceivedEchoes: make(map[string]KEYGENEcho),  // From(M) big.Int (in hex) to Echo
				ReceivedReadys: make(map[string]KEYGENReady), // From(M) big.Int (in hex) to Ready
				// ReceivedShareCompletes: make(map[string]KEYGENShareComplete), // From(M) big.Int (in hex) to ShareComplete
				SentEcho:  false,
				SentReady: false,
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
	// check length of commitment matrix is right
	if len(msg.CommitmentMatrixes) != ki.NumOfKeys {
		return errors.New("length of commitment matrix is not correct")
	}

	for i, commitmentMatrix := range msg.CommitmentMatrixes {
		index := big.NewInt(int64(i))
		index.Add(index, &ki.StartIndex)
		if len(commitmentMatrix) != ki.Threshold || len(commitmentMatrix[0]) != ki.Threshold {
			return fmt.Errorf("commitment matrix not the same threshold, expected: %v, got: %v", ki.Threshold, len(commitmentMatrix))
		}
		//reject commitment matrix if we have stored it before
		if ki.KeyLog[index.Text(16)][nodeIndex.Text(16)].C != nil {
			return errors.New("commitment matrix committed before")
		}
	}

	// store commitment matrix
	for i, commitmentMatrix := range msg.CommitmentMatrixes {
		index := big.NewInt(int64(i))
		index.Add(index, &ki.StartIndex)
		// TODO: create state to handle time out of t2
		ki.KeyLog[index.Text(16)][nodeIndex.Text(16)].C = commitmentMatrix
	}

	// if all of the commitments are in, lets start sending KeygenSends
	allIn := true
	for _, dealers := range ki.KeyLog {
		for _, subKey := range dealers {
			if subKey.C == nil {
				allIn = false
			}
		}
	}
	if allIn {
		ki.prepareAndSendKEYGENSend()
	}

	return nil
}

func (ki *KeygenInstance) prepareAndSendKEYGENSend() error {
	// send all KEGENSends to  respective nodes
	// logging.Debugf("NODE" + ki.NodeIndex.Text(16) + " Sending KEYGENSends")
	for i := int(ki.StartIndex.Int64()); i < ki.NumOfKeys+int(ki.StartIndex.Int64()); i++ {
		keyIndex := big.NewInt(int64(i))
		committedSecrets := ki.Secrets[keyIndex.Text(16)]
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
				return err
			}
		}
	}
	return nil
}

func (ki *KeygenInstance) OnKEYGENSend(msg KEYGENSend, fromNodeIndex big.Int) error {
	ki.Lock()
	defer ki.Unlock()
	keyLog, ok := ki.KeyLog[msg.KeyIndex.Text(16)][fromNodeIndex.Text(16)]
	if !ok {
		return errors.New("Keylog not found for keygen send")
	}
	// If C isnt in
	if keyLog.C == nil {
		go func() {
			time.Sleep(retryKEYGENSend * time.Second)
			err := ki.OnKEYGENSend(msg, fromNodeIndex)
			if err != nil {
				logging.Debugf(err.Error())
			}
		}()
		return errors.New("for send havent registered commitment matrix yet")
	}
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
	if !keyLog.SentEcho {
		err := ki.prepareAndSendKEYGENEchoFor(msg.KeyIndex, fromNodeIndex)
		if err != nil {
			// TODO: Handle failure, resend?
			logging.Debugf("NODE"+ki.NodeIndex.Text(16)+"Error sending echo: %v", err)
		}
	}

	return nil
}

func (ki *KeygenInstance) prepareAndSendKEYGENEchoFor(keyIndex big.Int, nodeIndex big.Int) error {
	keyLog := ki.KeyLog[keyIndex.Text(16)][nodeIndex.Text(16)]
	if keyLog.SentEcho {
		logging.Fatal("Sent ready already")
	}
	keyLog.SentEcho = true
	for k := range ki.NodeLog {
		nodeToSendIndex := big.Int{}
		nodeToSendIndex.SetString(k, 16)
		msg := keyLog.ReceivedSend
		if msg != nil {
			keygenEcho := KEYGENEcho{
				KeyIndex: keyIndex,
				Dealer:   nodeIndex,
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
	keyLog, ok := ki.KeyLog[msg.KeyIndex.Text(16)][msg.Dealer.Text(16)]
	if !ok {
		return errors.New("Keylog not found OnKEYGENEcho")
	}
	// If C isnt in
	if keyLog.C == nil {
		go func() {
			time.Sleep(retryKEYGENSend * time.Second)
			err := ki.OnKEYGENEcho(msg, fromNodeIndex)
			if err != nil {
				logging.Debugf(err.Error())
			}
		}()
		return errors.New("for echo havent registered commitment matrix yet")
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
	if ki.Threshold <= len(keyLog.ReceivedEchoes) && ratio <= len(keyLog.ReceivedEchoes) && !keyLog.SentReady {
		// since threshoold and above

		// First we interpolate valid keygenEchos to keygenSend
		// prepare for derivation of polynomial
		index := &msg.KeyIndex
		nodeIndex := msg.Dealer
		aiyPoints := make([]common.Point, ki.Threshold)
		aiprimeyPoints := make([]common.Point, ki.Threshold)
		bixPoints := make([]common.Point, ki.Threshold)
		biprimexPoints := make([]common.Point, ki.Threshold)
		count := 0

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

		err := ki.prepareAndSendKEYGENReadyFor(*index, nodeIndex)
		if err != nil {
			// TODO: Handle failure, resend?
			logging.Errorf("NODE"+ki.NodeIndex.Text(16)+"Could not sent KEYGENReady %s", err)
		}
	}
	return nil
}

func (ki *KeygenInstance) prepareAndSendKEYGENReadyFor(keyIndex big.Int, dealerIndex big.Int) error {
	// we send readys here when we have collected enough echos
	keyIndexStr := keyIndex.Text(16)
	dealerStr := dealerIndex.Text(16)
	keyLog := ki.KeyLog[keyIndexStr][dealerStr]
	if keyLog.SentReady {
		logging.Fatal("Sent ready already")
	}
	keyLog.SentReady = true
	for k := range ki.NodeLog {
		nodeToSendIndex := big.Int{}
		nodeToSendIndex.SetString(k, 16)
		sig, err := ki.Auth.Sign(readyPrefix + keyIndexStr + dealerStr)
		if err != nil {
			logging.Error(err.Error())
		}
		keygenReady := KEYGENReady{
			KeyIndex: keyIndex,
			Dealer:   dealerIndex,
			Aij:      *pvss.PolyEval(keyLog.ReceivedSend.AIY, nodeToSendIndex),
			Aprimeij: *pvss.PolyEval(keyLog.ReceivedSend.AIprimeY, nodeToSendIndex),
			Bij:      *pvss.PolyEval(keyLog.ReceivedSend.BIX, nodeToSendIndex),
			Bprimeij: *pvss.PolyEval(keyLog.ReceivedSend.BIprimeX, nodeToSendIndex),
			ReadySig: sig,
		}
		err = ki.Transport.SendKEYGENReady(keygenReady, nodeToSendIndex)
		if err != nil {
			// TODO: Handle failure, resend?
			return err
		}
	}
	return nil
}

func (ki *KeygenInstance) OnKEYGENReady(msg KEYGENReady, fromNodeIndex big.Int) error {
	ki.Lock()
	defer ki.Unlock()
	if !ki.Auth.Verify(readyPrefix+msg.KeyIndex.Text(16)+msg.Dealer.Text(16), fromNodeIndex, msg.ReadySig) {
		return errors.New("Ready Signature is not right")
	}
	keyLog, ok := ki.KeyLog[msg.KeyIndex.Text(16)][msg.Dealer.Text(16)]
	if !ok {
		return errors.New("Keylog not found OnKEYGENReady")
	}
	// If C isnt in
	if keyLog.C == nil {
		go func() {
			time.Sleep(retryKEYGENSend * time.Second)
			err := ki.OnKEYGENReady(msg, fromNodeIndex)
			if err != nil {
				logging.Debugf(err.Error())
			}
		}()
		return errors.New("for ready havent registered commitment matrix yet")
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
	if ki.Threshold == len(keyLog.ReceivedReadys) && !keyLog.SentReady {
		// First we interpolate valid keygenEchos to keygenSend
		// prepare for derivation of polynomial
		index := &msg.KeyIndex
		nodeIndex := msg.Dealer
		aiyPoints := make([]common.Point, ki.Threshold)
		aiprimeyPoints := make([]common.Point, ki.Threshold)
		bixPoints := make([]common.Point, ki.Threshold)
		biprimexPoints := make([]common.Point, ki.Threshold)
		count := 0

		for k, v := range ki.KeyLog[index.Text(16)][nodeIndex.Text(16)].ReceivedReadys {
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

		err := ki.prepareAndSendKEYGENReadyFor(*index, nodeIndex)
		if err != nil {
			logging.Errorf("could not sendReady x %s from node %s", index.Text(16), nodeIndex.Text(16))
		}

	}

	if len(keyLog.ReceivedReadys) == ki.Threshold+ki.NumMalNodes {
		ki.finalizeSubshare(msg.Dealer)
	}

	return nil
}

func (ki *KeygenInstance) finalizeSubshare(nodeIndex big.Int) {
	// Add to counts
	ki.SubsharesComplete++
	ki.NodeLog[nodeIndex.Text(16)]++

	// Check if all subshares are complete
	if ki.SubsharesComplete == ki.NumOfKeys*len(ki.NodeLog) {
		ki.prepareAndSendKEYGENDKGComplete()
	}
}

func (ki *KeygenInstance) prepareAndSendKEYGENDKGComplete() {
	// create qualified set
	var nodeSet []string
	if len(ki.FinalNodeSet) == 0 {
		for nodeIndex, count := range ki.NodeLog {
			if count == ki.NumOfKeys {
				nodeSet = append(nodeSet, nodeIndex)
			}
		}
		sort.SliceStable(nodeSet, func(i, j int) bool { return nodeSet[i] < nodeSet[j] })
	} else {
		nodeSet = ki.FinalNodeSet
	}

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
			C:        c,
			U1:       u1,
			U2:       u2,
			Gsi:      gs,
			Gsihr:    gshr,
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
	err := ki.Transport.BroadcastKEYGENDKGComplete(KEYGENDKGComplete{NodeSet: nodeSet, Proofs: keygenShareCompletes, ReadySignatures: tempReadySigMap})
	if err != nil {
		logging.Errorf("NODE"+ki.NodeIndex.Text(16)+"Could not BroadcastKEYGENDKGComplete: %s", err)
	}

	go func() {
		time.Sleep(time.Second * retryBroadcastingKEYGENDKGComplete)
		ki.Lock()
		defer ki.Unlock()
		if len(ki.ReceivedDKGCompleted) < ki.Threshold {
			ki.prepareAndSendKEYGENDKGComplete()
		}
	}()
}

func (ki *KeygenInstance) OnKEYGENDKGComplete(msg KEYGENDKGComplete, fromNodeIndex big.Int) error {
	ki.Lock()
	defer ki.Unlock()
	if len(msg.Proofs) != ki.NumOfKeys {
		return errors.New("length of proofs is not correct")
	}
	if len(ki.FinalNodeSet) == 0 {
		if len(msg.NodeSet) < ki.Threshold+ki.NumMalNodes {
			return errors.New("Nodeset not large enough")
		}
		// return errors.New("Broadcast already accepted")
	} else {
		if !testEqualStringArr(ki.FinalNodeSet, msg.NodeSet) {
			return errors.New("Broadcast already accepted with different final nodeset")
		}
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
		if !pvss.VerifyNIZKPK(keygenShareCom.C, keygenShareCom.U1, keygenShareCom.U2, keygenShareCom.Gsi, keygenShareCom.Gsihr) {
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
		if !pvss.AVSSVerifyShareCommitment(sumCommitments, fromNodeIndex, keygenShareCom.Gsihr) {
			return errors.New("Faulty Share Commitment OnKEYGENDKGComplete")
		}

		// check ready signatures
		for keyIndex, key := range msg.ReadySignatures {
			for dealerIndex, dealer := range key {
				if len(dealer) < ki.Threshold+ki.NumMalNodes {
					b, _ := bijson.MarshalIndent(msg, "", "\t")
					fmt.Println("From: ", fromNodeIndex.Text(16), string(b))
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

	ki.ReceivedDKGCompleted[fromNodeIndex.Text(16)] = &msg

	if len(ki.FinalNodeSet) == 0 {
		// define set
		ki.FinalNodeSet = msg.NodeSet
	}

	// End keygen
	if len(ki.ReceivedDKGCompleted) == ki.Threshold {
		ki.endKeygen()
	}

	return nil
}

func (ki *KeygenInstance) endKeygen() {
	// check if final node set shares are all in
	allIn := true
	for _, nodeIndex := range ki.FinalNodeSet {
		if ki.NodeLog[nodeIndex] != ki.NumOfKeys {
			allIn = false
		}
	}
	if !allIn {
		go func() {
			ki.Lock()
			defer ki.Unlock()
			time.Sleep(retryEndingKeygen * time.Second)
			// if not retry later
			ki.endKeygen()
		}()
		return
	}
	// To wrap things up we first store Secrets and respective Key Shares
	for i := 0; i < ki.NumOfKeys; i++ {
		keyIndex := big.Int{}
		keyIndex.SetInt64(int64(i)).Add(&keyIndex, &ki.StartIndex)
		// form  Si
		si := big.NewInt(int64(0))
		siprime := big.NewInt(int64(0))
		for _, nodeIndex := range ki.FinalNodeSet {
			// add up subshares for qualified set

			v := ki.KeyLog[keyIndex.Text(16)][nodeIndex]
			si.Add(si, &v.ReceivedSend.AIY.Coeff[0])
			siprime.Add(siprime, &v.ReceivedSend.AIprimeY.Coeff[0])
		}

		// Store All Necessary Infos
		// Derive Public Key
		var pk common.Point
		points := make([]common.Point, 0)
		indexes := make([]int, 0)
		count := 0
		for fromNodeIndex, dkgComplete := range ki.ReceivedDKGCompleted {
			count++
			points = append(points, dkgComplete.Proofs[i].Gsi)
			tmp := big.Int{}
			tmp.SetString(fromNodeIndex, 16)
			indexes = append(indexes, int(tmp.Int64()))
			if count == ki.Threshold {
				break
			}
		}
		pk = *pvss.LagrangeCurvePts(indexes, points)

		err := ki.Store.StoreCompletedShare(keyIndex, *si, *siprime, pk)
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
	logging.Debugf("KeygenComplete: %s", completionMsg)
	ki.ComChannel <- completionMsg
}
