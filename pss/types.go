package pss

import (
	"encoding/hex"
	"errors"
	"math/big"
	"strconv"
	"strings"

	"github.com/torusresearch/torus-public/idmutex"

	"github.com/torusresearch/bijson"
	"github.com/torusresearch/torus-public/common"
	"github.com/torusresearch/torus-public/secp256k1"
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

type SharingID string
type Sharing struct {
	idmutex.Mutex
	SharingID SharingID
	Nodes     []common.Node
	Epoch     int
	I         int
	Si        big.Int
	Siprime   big.Int
	C         []common.Point
}

type Recover struct {
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
	PSSID    PSSID
	Epoch    int
	Si       big.Int
	Siprime  big.Int
	F        [][]big.Int
	Fprime   [][]big.Int
	Cbar     [][]common.Point
	C        [][]common.Point
	Messages []PSSMessage
	CStore   map[CID]*C
	State    PSSState
}

func GetCIDFromPointMatrix(pm [][]common.Point) CID {
	var bytes []byte
	delimiter := []byte("|")
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

type PSSMessage struct {
	PSSID  PSSID  `json:"pssid"`
	Method string `json:"type"`
	Data   []byte `json:"data"`
}

func (pssMessage *PSSMessage) JSON() *bijson.RawMessage {
	json, err := bijson.Marshal(pssMessage)
	if err != nil {
		return nil
	} else {
		res := bijson.RawMessage(json)
		return &res
	}
}

// PSSID is the identifying string for PSSMessage
type PSSID string

const NullPSSID = PSSID("")

type PSSIDDetails struct {
	SharingID   SharingID
	DealerIndex int
}

func (pssIDDetails *PSSIDDetails) ToPSSID() PSSID {
	return PSSID(string(pssIDDetails.SharingID) + "|" + strconv.Itoa(pssIDDetails.DealerIndex))
}
func (pssIDDetails *PSSIDDetails) FromPSSID(pssID PSSID) error {
	s := string(pssID)
	substrings := strings.Split(s, "|")
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

type NodeDetails common.Node

func (n *NodeDetails) ToNodeDetailsID() NodeDetailsID {
	return NodeDetailsID(strconv.Itoa(n.Index) + "|" + n.PubKey.X.Text(16) + "|" + n.PubKey.Y.Text(16))
}
func (n *NodeDetails) FromNodeDetailsID(nodeDetailsID NodeDetailsID) {
	s := string(nodeDetailsID)
	substrings := strings.Split(s, "|")
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

type PSSTransport interface {
	GetType() string
	SetPSSNode(*PSSNode) error
	Sign(string) ([]byte, error)
	Send(NodeDetails, PSSMessage) error
	Receive(NodeDetails, PSSMessage) error
	SendBroadcast(PSSMessage) error
	ReceiveBroadcast(PSSMessage) error
	Output(string)
}

type NodeNetwork struct {
	Nodes map[NodeDetailsID]NodeDetails
	N     int
	T     int
	K     int
	ID    string
}

type Complete struct {
	CompleteMessageSent bool
	PSSID               PSSID
	C00                 common.Point
}

type PSSNode struct {
	idmutex.Mutex
	NodeDetails   NodeDetails
	OldNodes      NodeNetwork
	NewNodes      NodeNetwork
	NodeIndex     int
	ShareStore    map[SharingID]*Sharing
	RecoverStore  map[SharingID]*Recover
	CompleteStore map[SharingID]*Complete
	Transport     PSSTransport
	PSSStore      map[PSSID]*PSS
	IsDealer      bool
	IsPlayer      bool
}

func mapFromNodeList(nodeList []common.Node) (res map[NodeDetailsID]NodeDetails) {
	res = make(map[NodeDetailsID]NodeDetails)
	for _, node := range nodeList {
		nodeDetails := NodeDetails(node)
		res[nodeDetails.ToNodeDetailsID()] = nodeDetails
	}
	return
}
