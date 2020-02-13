package pss

import (
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"sync"

	logging "github.com/sirupsen/logrus"
	"github.com/torusresearch/torus-common/common"
	"github.com/torusresearch/torus-common/secp256k1"

	pcmn "github.com/torusresearch/torus-node/common"
	"github.com/torusresearch/torus-node/idmutex"

	"github.com/torusresearch/torus-node/version"
)

// Dont try to cast strings or bools, use the exported structs to get the types you need
type phaseState string
type dealerState bool
type playerState bool
type receivedSendState bool
type receivedEchoState bool
type receivedReadyState bool

// type recoverState string

type phaseStates struct {
	Initial   phaseState
	Started   phaseState
	Proposing phaseState
	Ended     phaseState
}

// type recoverStates struct {
// 	Initial                            recoverState
// 	WaitingForRecovers                 recoverState
// 	WaitingForThresholdSharing         recoverState
// 	WaitingForVBA                      recoverState
// 	WaitingForSelectedSharingsComplete recoverState
// 	Ended                              recoverState
// }

type dealerStates struct {
	IsDealer  dealerState
	NotDealer dealerState
}

type playerStates struct {
	IsPlayer  playerState
	NotPlayer playerState
}

type receivedSendStates struct {
	True  receivedSendState
	False receivedSendState
}

type receivedEchoStates struct {
	True  receivedEchoState
	False receivedEchoState
}

type receivedReadyStates struct {
	True  receivedReadyState
	False receivedReadyState
}

type PSSState struct {
	Phase  phaseState
	Dealer dealerState
	Player playerState
	// Recover       recoverState
	ReceivedSend  receivedSendState
	ReceivedEcho  map[NodeDetailsID]receivedEchoState
	ReceivedReady map[NodeDetailsID]receivedReadyState
}

var States = struct {
	Phases           phaseStates
	Dealer           dealerStates
	Player           playerStates
	ReceivedSend     receivedSendStates
	ReceivedEcho     receivedEchoStates
	ReceivedEchoMap  func() map[NodeDetailsID]receivedEchoState
	ReceivedReady    receivedReadyStates
	ReceivedReadyMap func() map[NodeDetailsID]receivedReadyState
	// Recover          recoverStates
}{
	phaseStates{
		Initial:   phaseState("initial"),
		Started:   phaseState("started"),
		Proposing: phaseState("proposing"),
		Ended:     phaseState("ended"),
	},
	dealerStates{
		IsDealer:  dealerState(true),
		NotDealer: dealerState(false),
	},
	playerStates{
		IsPlayer:  playerState(true),
		NotPlayer: playerState(false),
	},
	receivedSendStates{
		True:  receivedSendState(true),
		False: receivedSendState(false),
	},
	receivedEchoStates{
		True:  receivedEchoState(true),
		False: receivedEchoState(false),
	},
	CreateReceivedEchoMap,
	receivedReadyStates{
		True:  receivedReadyState(true),
		False: receivedReadyState(false),
	},
	CreateReceivedReadyMap,
	// recoverStates{
	// 	Initial:                            recoverState("initial"),
	// 	WaitingForRecovers:                 recoverState("waitingforrecovers"),
	// 	WaitingForThresholdSharing:         recoverState("waitingforthresholdsharing"),
	// 	WaitingForVBA:                      recoverState("waitingforvba"),
	// 	WaitingForSelectedSharingsComplete: recoverState("waitingforselectedsharingscomplete"),
	// 	Ended:                              recoverState("ended"),
	// },
}

func CreateReceivedEchoMap() map[NodeDetailsID]receivedEchoState {
	return make(map[NodeDetailsID]receivedEchoState)
}

func CreateReceivedReadyMap() map[NodeDetailsID]receivedReadyState {
	return make(map[NodeDetailsID]receivedReadyState)
}

type PSSMsgShare struct {
	SharingID SharingID
}
type PSSMsgRecover struct {
	SharingID SharingID
	V         []common.Point
}

func GetVIDFromPointArray(ptArr []common.Point) VID {
	var bytes []byte
	for _, pt := range ptArr {
		bytes = append(bytes, pt.X.Bytes()...)
		bytes = append(bytes, pt.Y.Bytes()...)
	}
	vhash := secp256k1.Keccak256(bytes)
	return VID(hex.EncodeToString(vhash))
}

type PSSMsgSend struct {
	PSSID  PSSID
	C      [][]common.Point
	A      []big.Int
	Aprime []big.Int
	B      []big.Int
	Bprime []big.Int
}

type PSSMsgEcho struct {
	PSSID      PSSID
	C          [][]common.Point
	Alpha      big.Int
	Alphaprime big.Int
	Beta       big.Int
	Betaprime  big.Int
}

type SignedText []byte
type PSSMsgReady struct {
	PSSID      PSSID
	C          [][]common.Point
	Alpha      big.Int
	Alphaprime big.Int
	Beta       big.Int
	Betaprime  big.Int
	SignedText SignedText
}

type PSSMsgComplete struct {
	PSSID PSSID
	C00   common.Point
}

type PSSMsgPropose struct {
	NodeDetailsID NodeDetailsID
	SharingID     SharingID
	PSSs          []PSSID
	SignedTexts   []map[NodeDetailsID]SignedText
}

type PSSMsgDecide struct {
	SharingID SharingID
	PSSs      []PSSID
}

type VID string

// Unique string that contains information about pss parameters (epoch numbers, thresholds) and keygenID (tied to each key index)
type SharingID string

type KeygenID string

func GenerateKeygenID(i int) KeygenID {
	return KeygenID(strings.Join([]string{"SHARING", big.NewInt(int64(i)).Text(16)}, pcmn.Delimiter3))
}

func (keygenID *KeygenID) GetIndex() int {
	s := string(*keygenID)
	substrs := strings.Split(s, pcmn.Delimiter3)

	if len(substrs) != 2 {
		logging.WithField("keygenID", keygenID).Error("could not get index of keygenID")
		return 0
	}
	i, ok := new(big.Int).SetString(substrs[1], 16)
	if !ok {
		logging.WithField("keygenID", keygenID).Error("could not convert index of keygenID")
	}
	return int(i.Int64())
}

func (sID *SharingID) GetKeygenID() (keygenID KeygenID) {
	s := string(*sID)
	substrs := strings.Split(s, pcmn.Delimiter2)
	if len(substrs) != 9 {
		logging.WithField("sharingID", *sID).Error("could not split sharingID to get keygenID")
		return keygenID
	}
	return KeygenID(substrs[len(substrs)-1])
}

// GetEpochParams - returns values for epochOld, nOld, kOld, tOld, epochNew, nNew, kNew, tNew
func (sID *SharingID) GetEpochParams() ([8]int, error) {
	var emptyRes [8]int
	s := string(*sID)
	resSlice := strings.Split(s, pcmn.Delimiter2)
	if len(resSlice) != 9 {
		return emptyRes, fmt.Errorf("could not split sharingID %v", *sID)
	}

	var res [8]int
	for i := 0; i < len(res); i++ {
		resInt, err := strconv.Atoi(resSlice[i])
		if err != nil {
			return emptyRes, err
		}
		res[i] = resInt
	}
	return res, nil
}

func (kID *KeygenID) GetSharingID(epochOld, nOld, kOld, tOld, epochNew, nNew, kNew, tNew int) SharingID {
	return SharingID(strings.Join([]string{
		strconv.Itoa(epochOld),
		strconv.Itoa(nOld),
		strconv.Itoa(kOld),
		strconv.Itoa(tOld),
		strconv.Itoa(epochNew),
		strconv.Itoa(nNew),
		strconv.Itoa(kNew),
		strconv.Itoa(tNew),
		string(*kID),
	}, pcmn.Delimiter2))
}

type SharingData struct {
	KeygenID KeygenID
	Nodes    []pcmn.Node
	Epoch    int
	I        int
	Si       big.Int
	Siprime  big.Int
	C        []common.Point
}

type Sharing struct {
	idmutex.Mutex
	KeygenID KeygenID
	Nodes    []pcmn.Node
	Epoch    int
	I        int
	Si       big.Int
	Siprime  big.Int
	C        []common.Point
}

type Recover struct {
	idmutex.Mutex
	SharingID        SharingID
	D                *[]common.Point
	DCount           map[VID]map[NodeDetailsID]bool
	PSSCompleteCount map[PSSID]bool
	Si               big.Int
	Siprime          big.Int
	Vbar             []common.Point
}

type PSS struct {
	idmutex.Mutex
	PSSID   PSSID
	Epoch   int
	Si      big.Int
	Siprime big.Int
	F       [][]big.Int
	Fprime  [][]big.Int
	Cbar    [][]common.Point
	C       [][]common.Point
	CStore  map[CID]*C
	State   PSSState
}

func GetCIDFromPointMatrix(pm [][]common.Point) CID {
	var bytes []byte
	delimiter := pcmn.Delimiter1
	for _, arr := range pm {
		for _, pt := range arr {
			bytes = append(bytes, pt.X.Bytes()...)
			bytes = append(bytes, delimiter...)
			bytes = append(bytes, pt.Y.Bytes()...)
			bytes = append(bytes, delimiter...)
		}
	}
	return CID(hex.EncodeToString(secp256k1.Keccak256(bytes)))
}

type CID string

type C struct {
	CID             CID
	C               [][]common.Point
	EC              int
	RC              int
	AC              map[NodeDetailsID]common.Point
	ACprime         map[NodeDetailsID]common.Point
	BC              map[NodeDetailsID]common.Point
	BCprime         map[NodeDetailsID]common.Point
	Abar            []big.Int
	Abarprime       []big.Int
	Bbar            []big.Int
	Bbarprime       []big.Int
	SignedTextStore map[NodeDetailsID]SignedText
}

func GetPointArrayFromMap(m map[NodeDetailsID]common.Point) (res []common.Point) {
	for _, pt := range m {
		res = append(res, pt)
	}
	return
}

func GetSignedTextArrayFromMap(sts map[NodeDetailsID]SignedText) (res []SignedText) {
	for _, signedText := range sts {
		res = append(res, signedText)
	}
	return
}

func CreatePSSMessage(r PSSMessageRaw) PSSMessage {
	return PSSMessage{
		Version: pssMessageVersion(version.NodeVersion),
		PSSID:   r.PSSID,
		Method:  r.Method,
		Data:    r.Data,
	}
}

type PSSMessageRaw struct {
	PSSID  PSSID
	Method string
	Data   []byte
}

type pssMessageVersion string

type PSSMessage struct {
	Version pssMessageVersion `json:"version,omitempty"`
	PSSID   PSSID             `json:"pssid"`
	Method  string            `json:"type"`
	Data    []byte            `json:"data"`
}

// PSSID is the identifying string for PSSMessage, each PSS has n PSSIDs, all associated with a single SharingID
type PSSID string

const NullNodeDetails = NodeDetailsID("")

const NullPSSID = PSSID("")

type PSSIDDetails struct {
	SharingID   SharingID
	DealerIndex int
}

func (pssIDDetails *PSSIDDetails) ToPSSID() PSSID {
	return PSSID(strings.Join([]string{string(pssIDDetails.SharingID), strconv.Itoa(pssIDDetails.DealerIndex)}, pcmn.Delimiter1))
}
func (pssIDDetails *PSSIDDetails) FromPSSID(pssID PSSID) error {
	s := string(pssID)
	substrings := strings.Split(s, pcmn.Delimiter1)

	if len(substrings) != 2 {
		return errors.New("Error parsing PSSIDDetails, too few fields")
	}
	pssIDDetails.SharingID = SharingID(substrings[0])
	index, err := strconv.Atoi(substrings[1])
	if err != nil {
		return err
	}
	pssIDDetails.DealerIndex = index
	return nil
}

type NodeDetailsID string

type NodeDetails pcmn.Node

func (n *NodeDetails) ToNodeDetailsID() NodeDetailsID {
	return NodeDetailsID(strings.Join([]string{
		strconv.Itoa(n.Index),
		n.PubKey.X.Text(16),
		n.PubKey.Y.Text(16),
	}, pcmn.Delimiter1))
}
func (n *NodeDetails) FromNodeDetailsID(nodeDetailsID NodeDetailsID) {
	s := string(nodeDetailsID)
	substrings := strings.Split(s, pcmn.Delimiter1)

	if len(substrings) != 3 {
		return
	}
	index, err := strconv.Atoi(substrings[0])
	if err != nil {
		return
	}
	n.Index = index
	pubkeyX, ok := new(big.Int).SetString(substrings[1], 16)
	if !ok {
		return
	}
	n.PubKey.X = *pubkeyX
	pubkeyY, ok := new(big.Int).SetString(substrings[2], 16)
	if !ok {
		return
	}
	n.PubKey.Y = *pubkeyY
}

type PSSDataSource interface {
	Init()
	GetSharing(keygenID KeygenID) *Sharing
	SetPSSNode(*PSSNode) error
}

type PSSTransport interface {
	Init()
	GetType() string
	SetPSSNode(*PSSNode) error
	Sign([]byte) ([]byte, error)
	Send(NodeDetails, PSSMessage) error
	Receive(NodeDetails, PSSMessage) error
	SendBroadcast(PSSMessage) error
	ReceiveBroadcast(PSSMessage) error
	Output(interface{})
}

type NodeNetwork struct {
	Nodes   map[NodeDetailsID]NodeDetails
	N       int
	T       int
	K       int
	EpochID int
}

type Complete struct {
	idmutex.Mutex
	CompleteMessageSent bool
	SharingID           SharingID
	C00                 common.Point
}

type PSSNode struct {
	NodeDetails  NodeDetails
	OldNodes     NodeNetwork
	NewNodes     NodeNetwork
	NodeIndex    int
	DataSource   PSSDataSource
	Transport    PSSTransport
	RecoverStore *RecoverStoreSyncMap
	PSSStore     *PSSStoreSyncMap
	IsDealer     bool
	IsPlayer     bool
	CleanUp      func(*PSSNode, SharingID) error

	staggerDelay int
}

type PSSStoreSyncMap struct {
	sync.Map
	nodes *NodeNetwork // reference for cleanup purposes
}

func (m *PSSStoreSyncMap) Get(pssid PSSID) (pss *PSS, found bool) {
	inter, found := m.Map.Load(pssid)
	pss, _ = inter.(*PSS)
	return
}
func (m *PSSStoreSyncMap) Set(pssid PSSID, pss *PSS) {
	m.Map.Store(pssid, pss)
}
func (m *PSSStoreSyncMap) GetOrSet(pssid PSSID, input *PSS) (pss *PSS, found bool) {
	inter, found := m.Map.LoadOrStore(pssid, input)
	pss, _ = inter.(*PSS)
	return
}
func (m *PSSStoreSyncMap) GetOrSetIfNotComplete(pssid PSSID, input *PSS) (pss *PSS, complete bool) {
	inter, found := m.GetOrSet(pssid, input)
	if found {
		if inter == nil {
			return inter, true
		}
	}
	return inter, false
}
func (m *PSSStoreSyncMap) Complete(sharingID SharingID) {
	for _, v := range m.nodes.Nodes {
		keygenDetails := PSSIDDetails{
			SharingID:   sharingID,
			DealerIndex: v.Index,
		}
		m.Map.Store(keygenDetails.ToPSSID(), nil)
	}
}

type RecoverStoreSyncMap struct {
	sync.Map
}

func (m *RecoverStoreSyncMap) Get(sharingID SharingID) (recover *Recover, found bool) {
	inter, found := m.Map.Load(sharingID)
	recover, _ = inter.(*Recover)
	return
}
func (m *RecoverStoreSyncMap) Set(sharingID SharingID, recover *Recover) {
	m.Map.Store(sharingID, recover)
}
func (m *RecoverStoreSyncMap) GetOrSet(sharingID SharingID, input *Recover) (recover *Recover, found bool) {
	inter, found := m.Map.LoadOrStore(sharingID, input)
	recover, _ = inter.(*Recover)
	return
}
func (m *RecoverStoreSyncMap) GetOrSetIfNotComplete(sharingID SharingID, input *Recover) (recover *Recover, complete bool) {
	inter, found := m.GetOrSet(sharingID, input)
	if found {
		if inter == nil {
			return inter, true
		}
	}
	return inter, false
}
func (m *RecoverStoreSyncMap) Complete(sharingID SharingID) {
	m.Set(sharingID, nil)
}

type ShareStoreSyncMap struct {
	sync.Map
}

func (m *ShareStoreSyncMap) Get(keygenID KeygenID) (sharing *Sharing, found bool) {
	inter, found := m.Map.Load(keygenID)
	sharing, _ = inter.(*Sharing)
	return
}
func (m *ShareStoreSyncMap) Set(keygenID KeygenID, sharing *Sharing) {
	m.Map.Store(keygenID, sharing)
}
func (m *ShareStoreSyncMap) Complete(keygenID KeygenID) {
	m.Set(keygenID, nil)
}

type RefreshKeyStorage struct {
	KeyIndex       big.Int
	Si             big.Int
	Siprime        big.Int
	CommitmentPoly []common.Point
}

func mapFromNodeList(nodeList []pcmn.Node) (res map[NodeDetailsID]NodeDetails) {
	res = make(map[NodeDetailsID]NodeDetails)
	for _, node := range nodeList {
		nodeDetails := NodeDetails(node)
		res[nodeDetails.ToNodeDetailsID()] = nodeDetails
	}
	return
}
