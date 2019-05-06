package keygennofsm

import (
	"encoding/hex"
	"errors"
	"math/big"
	"strconv"
	"strings"

	"github.com/torusresearch/torus-public/common"
	"github.com/torusresearch/torus-public/idmutex"
	"github.com/torusresearch/torus-public/secp256k1"
)

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

type SharingID string

// KeygenID is the identifying string for KeygenMessage
type KeygenID string

const NullKeygenID = KeygenID("")

type KeygenIDDetails struct {
	SharingID   SharingID
	DealerIndex int
}

func (keygenIDDetails *KeygenIDDetails) ToKeygenID() KeygenID {
	return KeygenID(string(keygenIDDetails.SharingID) + "|" + strconv.Itoa(keygenIDDetails.DealerIndex))
}
func (keygenIDDetails *KeygenIDDetails) FromKeygenID(keygenID KeygenID) error {
	s := string(keygenID)
	substrings := strings.Split(s, "|")
	if len(substrings) != 2 {
		return errors.New("Error parsing keygenIDDetails, did not find 2 fields exactly")
	}
	keygenIDDetails.SharingID = SharingID(substrings[0])
	index, err := strconv.Atoi(substrings[1])
	if err != nil {
		return err
	}
	keygenIDDetails.DealerIndex = index
	return nil
}

type KeygenTransport interface {
	GetType() string
	SetKeygenNode(*KeygenNode) error
	Sign(string) ([]byte, error)
	Send(NodeDetails, KeygenMessage) error
	Receive(NodeDetails, KeygenMessage) error
	SendBroadcast(KeygenMessage) error
	ReceiveBroadcast(KeygenMessage) error
	Output(string)
}

type NodeNetwork struct {
	Nodes map[NodeDetailsID]NodeDetails
	N     int
	T     int
	K     int
	ID    string
}

type SignedText []byte

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

type phaseState string
type receivedSendState bool
type receivedEchoState bool
type receivedReadyState bool

type phaseStates struct {
	Initial   phaseState
	Started   phaseState
	Proposing phaseState
	Ended     phaseState
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

type KeygenState struct {
	Phase         phaseState
	ReceivedSend  receivedSendState
	ReceivedEcho  map[NodeDetailsID]receivedEchoState
	ReceivedReady map[NodeDetailsID]receivedReadyState
}

var States = struct {
	Phases           phaseStates
	ReceivedSend     receivedSendStates
	ReceivedEcho     receivedEchoStates
	ReceivedEchoMap  func() map[NodeDetailsID]receivedEchoState
	ReceivedReady    receivedReadyStates
	ReceivedReadyMap func() map[NodeDetailsID]receivedReadyState
}{
	phaseStates{
		Initial:   phaseState("initial"),
		Started:   phaseState("started"),
		Proposing: phaseState("proposing"),
		Ended:     phaseState("ended"),
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
}

func CreateReceivedEchoMap() map[NodeDetailsID]receivedEchoState {
	return make(map[NodeDetailsID]receivedEchoState)
}

func CreateReceivedReadyMap() map[NodeDetailsID]receivedReadyState {
	return make(map[NodeDetailsID]receivedReadyState)
}

type Keygen struct {
	idmutex.Mutex
	KeygenID KeygenID
	Epoch    int
	Si       big.Int
	Siprime  big.Int
	F        [][]big.Int
	Fprime   [][]big.Int
	Cbar     [][]common.Point
	C        [][]common.Point
	Messages []KeygenMessage
	CStore   map[CID]*C
	State    KeygenState
}

type Sharing struct {
	idmutex.Mutex
	SharingID SharingID
	Nodes     []common.Node
	Epoch     int
	I         int
	S         big.Int
	Sprime    big.Int
}

type DKG struct {
	SharingID SharingID
	GS        common.Point
	GSiStore  map[NodeDetailsID]common.Point
	DMap      map[KeygenID][]common.Point
	Si        big.Int
	Siprime   big.Int
	Dbar      []common.Point
}

type KeygenNode struct {
	idmutex.Mutex
	NodeDetails NodeDetails
	CurrNodes   NodeNetwork
	NodeIndex   int
	ShareStore  map[SharingID]*Sharing
	DKGStore    map[SharingID]*DKG
	Transport   KeygenTransport
	KeygenStore map[KeygenID]*Keygen
}

type KeygenMessage struct {
	KeygenID KeygenID `json:"pssid"`
	Method   string   `json:"type"`
	Data     []byte   `json:"data"`
}
type KeygenMsgShare struct {
	SharingID SharingID
}
type KeygenMsgSend struct {
	KeygenID KeygenID
	C        [][]common.Point
	A        []big.Int
	Aprime   []big.Int
	B        []big.Int
	Bprime   []big.Int
}
type KeygenMsgEcho struct {
	KeygenID   KeygenID
	C          [][]common.Point
	Alpha      big.Int
	Alphaprime big.Int
	Beta       big.Int
	Betaprime  big.Int
}
type KeygenMsgReady struct {
	KeygenID   KeygenID
	C          [][]common.Point
	Alpha      big.Int
	Alphaprime big.Int
	Beta       big.Int
	Betaprime  big.Int
	SignedText SignedText
}
type KeygenMsgPropose struct {
	NodeDetailsID NodeDetailsID
	SharingID     SharingID
	Keygens       []KeygenID
	SignedTexts   []map[NodeDetailsID]SignedText
}
type KeygenMsgComplete struct {
	KeygenID      KeygenID
	CommitmentArr []common.Point
}
type KeygenMsgDecide struct {
	SharingID SharingID
	Keygens   []KeygenID
}

type KeygenMsgNIZKP struct {
	SharingID   SharingID
	NodeIndex   int
	C           big.Int
	U1          big.Int
	U2          big.Int
	GSi         common.Point
	GSiHSiprime common.Point
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

func mapFromNodeList(nodeList []common.Node) (res map[NodeDetailsID]NodeDetails) {
	res = make(map[NodeDetailsID]NodeDetails)
	for _, node := range nodeList {
		nodeDetails := NodeDetails(node)
		res[nodeDetails.ToNodeDetailsID()] = nodeDetails
	}
	return
}
