package keygennofsm

import (
	"encoding/hex"
	"errors"
	"math/big"
	"strconv"
	"strings"
	"sync"

	logging "github.com/sirupsen/logrus"
	"github.com/torusresearch/bijson"

	pcmn "github.com/torusresearch/torus-node/common"
	"github.com/torusresearch/torus-node/idmutex"

	"github.com/torusresearch/torus-common/common"
	"github.com/torusresearch/torus-common/secp256k1"
	"github.com/torusresearch/torus-node/version"
)

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

type DKGID string

// KeygenID is the identifying string for KeygenMessage
type KeygenID string

const NullKeygenID = KeygenID("")

type KeygenIDDetails struct {
	DKGID       DKGID
	DealerIndex int
}

// GetIndex - get index from dkgid
func (dkgID *DKGID) GetIndex() (big.Int, error) {
	str := string(*dkgID)
	substrs := strings.Split(str, pcmn.Delimiter3)

	if len(substrs) != 2 {
		return *new(big.Int), errors.New("could not parse dkgid")
	}

	index, ok := new(big.Int).SetString(substrs[1], 16)
	if !ok {
		return *new(big.Int), errors.New("could not get back index from dkgid")
	}

	return *index, nil
}

func (keygenIDDetails *KeygenIDDetails) ToKeygenID() KeygenID {
	return KeygenID(strings.Join([]string{string(keygenIDDetails.DKGID), strconv.Itoa(keygenIDDetails.DealerIndex)}, pcmn.Delimiter4))
}
func (keygenIDDetails *KeygenIDDetails) FromKeygenID(keygenID KeygenID) error {
	s := string(keygenID)
	substrings := strings.Split(s, pcmn.Delimiter4)

	if len(substrings) != 2 {
		return errors.New("Error parsing keygenIDDetails, did not find 2 fields exactly")
	}
	keygenIDDetails.DKGID = DKGID(substrings[0])
	index, err := strconv.Atoi(substrings[1])
	if err != nil {
		return err
	}
	keygenIDDetails.DealerIndex = index
	return nil
}

type KeygenTransport interface {
	Init()
	GetType() string
	SetKeygenNode(*KeygenNode) error
	Sign([]byte) ([]byte, error)
	Send(NodeDetails, KeygenMessage) error
	Receive(NodeDetails, KeygenMessage) error
	SendBroadcast(KeygenMessage) error
	ReceiveBroadcast(KeygenMessage) error
	Output(interface{})

	// For Storage Optimization of NIZKP
	CheckIfNIZKPProcessed(keyIndex big.Int) bool
}

type NodeNetwork struct {
	Nodes map[NodeDetailsID]NodeDetails
	N     int
	T     int
	K     int
	ID    string
}

type ProposeProof struct {
	SignedTextDetails []byte
	C00               common.Point
}

type SignedText []byte

type SignedTextDetails struct {
	Text string
	C00  common.Point
}

func (s *SignedTextDetails) ToBytes() []byte {
	byt, err := bijson.Marshal(s)
	if err != nil {
		logging.WithField("SignedTextDetails", s).Error("Could not marshal signed text details")
	}
	return byt
}

func (s *SignedTextDetails) FromBytes(byt []byte) {
	err := bijson.Unmarshal(byt, s)
	if err != nil {
		logging.WithField("SignedTextDetails", s).Error("Could not unmarshal signed text details")
	}
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
	CStore   map[CID]*C
	State    KeygenState
}

type Sharing struct {
	idmutex.Mutex
	DKGID DKGID
	// Nodes  []pcmn.Node
	// Epoch  int
	I      int
	S      big.Int
	Sprime big.Int
}

type DKG struct {
	idmutex.Mutex
	DKGID      DKGID
	GS         common.Point
	NIZKPStore map[NodeDetailsID]NIZKP
	DMap       map[KeygenID][]common.Point
	Si         big.Int
	Siprime    big.Int
	Dbar       []common.Point
}

type ShareStoreSyncMap struct {
	sync.Map
}

func (m *ShareStoreSyncMap) Get(dkgID DKGID) (sharing *Sharing, found bool) {
	inter, found := m.Map.Load(dkgID)
	sharing, _ = inter.(*Sharing)
	return
}
func (m *ShareStoreSyncMap) Set(dkgID DKGID, sharing *Sharing) {
	m.Map.Store(dkgID, sharing)
}
func (m *ShareStoreSyncMap) GetOrSet(dkgID DKGID, input *Sharing) (sharing *Sharing, found bool) {
	inter, found := m.Map.LoadOrStore(dkgID, input)
	sharing, _ = inter.(*Sharing)
	return
}
func (m *ShareStoreSyncMap) Complete(dkgID DKGID) {
	m.Map.Store(dkgID, nil)
}
func (m *ShareStoreSyncMap) IsComplete(dkgID DKGID) (complete bool) {
	inter, found := m.Map.Load(dkgID)
	if found {
		if inter == nil {
			return true
		}
	}
	return false
}

type DKGStoreSyncMap struct {
	sync.Map
}

func (m *DKGStoreSyncMap) Get(dkgID DKGID) (dkg *DKG, found bool) {
	inter, found := m.Map.Load(dkgID)
	dkg, _ = inter.(*DKG)
	return
}
func (m *DKGStoreSyncMap) Set(dkgID DKGID, dkg *DKG) {
	m.Map.Store(dkgID, dkg)
}
func (m *DKGStoreSyncMap) GetOrSet(dkgID DKGID, input *DKG) (dkg *DKG, found bool) {
	inter, found := m.Map.LoadOrStore(dkgID, input)
	dkg, _ = inter.(*DKG)
	return
}
func (m *DKGStoreSyncMap) Complete(dkgID DKGID) {
	m.Map.Store(dkgID, nil)
}
func (m *DKGStoreSyncMap) GetOrSetIfNotComplete(dkgID DKGID, input *DKG) (dkg *DKG, complete bool) {
	inter, found := m.GetOrSet(dkgID, input)
	if found {
		if inter == nil {
			return inter, true
		}
	}
	return inter, false
}

type KeygenStoreSyncMap struct {
	sync.Map
	nodes *NodeNetwork // reference for cleanup purposes
}

func (m *KeygenStoreSyncMap) Get(keygenID KeygenID) (keygen *Keygen, found bool) {
	inter, found := m.Map.Load(keygenID)
	keygen, _ = inter.(*Keygen)
	return
}
func (m *KeygenStoreSyncMap) Set(keygenID KeygenID, keygen *Keygen) {
	m.Map.Store(keygenID, keygen)
}
func (m *KeygenStoreSyncMap) GetOrSet(keygenID KeygenID, input *Keygen) (keygen *Keygen, found bool) {
	inter, found := m.Map.LoadOrStore(keygenID, input)
	keygen, _ = inter.(*Keygen)
	return
}
func (m *KeygenStoreSyncMap) Complete(dkgID DKGID) {
	for _, v := range m.nodes.Nodes {
		keygenDetails := KeygenIDDetails{
			DKGID:       dkgID,
			DealerIndex: v.Index,
		}
		m.Map.Store(keygenDetails.ToKeygenID(), nil)
	}
}
func (m *KeygenStoreSyncMap) GetOrSetIfNotComplete(keygenID KeygenID, input *Keygen) (keygen *Keygen, complete bool) {
	inter, found := m.GetOrSet(keygenID, input)
	if found {
		if inter == nil {
			return inter, true
		}
	}
	return inter, false
}

type KeygenNode struct {
	NodeDetails NodeDetails
	CurrNodes   NodeNetwork
	NodeIndex   int
	ShareStore  *ShareStoreSyncMap
	DKGStore    *DKGStoreSyncMap
	Transport   KeygenTransport
	KeygenStore *KeygenStoreSyncMap
	CleanUp     func(*KeygenNode, DKGID) error
	// optimization for broadcast messages
	staggerDelay int
}

func CreateKeygenMessage(r KeygenMessageRaw) KeygenMessage {
	return KeygenMessage{
		Version:  keygenMessageVersion(version.NodeVersion),
		KeygenID: r.KeygenID,
		Method:   r.Method,
		Data:     r.Data,
	}
}

type KeygenMessageRaw struct {
	KeygenID KeygenID
	Method   string
	Data     []byte
}

type keygenMessageVersion string

type KeygenMessage struct {
	Version  keygenMessageVersion `json:"version,omitempty"`
	KeygenID KeygenID             `json:"keygenid"`
	Method   string               `json:"type"`
	Data     []byte               `json:"data"`
}
type KeygenMsgShare struct {
	DKGID DKGID
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
	DKGID         DKGID
	Keygens       []KeygenID
	ProposeProofs []map[NodeDetailsID]ProposeProof
}

type KeygenMsgPubKey struct {
	NodeDetailsID NodeDetailsID
	DKGID         DKGID
	PubKeyProofs  []NIZKP
}

type NIZKP struct {
	NodeIndex   int
	C           big.Int
	U1          big.Int
	U2          big.Int
	GSi         common.Point
	GSiHSiprime common.Point
}

type KeygenMsgComplete struct {
	KeygenID      KeygenID
	CommitmentArr []common.Point
}
type KeygenMsgDecide struct {
	DKGID   DKGID
	Keygens []KeygenID
}

type KeygenMsgNIZKP struct {
	DKGID DKGID
	NIZKP NIZKP
}

type KeyStorage struct {
	KeyIndex       big.Int
	Si             big.Int
	Siprime        big.Int
	CommitmentPoly []common.Point
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

func mapFromNodeList(nodeList []pcmn.Node) (res map[NodeDetailsID]NodeDetails) {
	res = make(map[NodeDetailsID]NodeDetails)
	for _, node := range nodeList {
		nodeDetails := NodeDetails(node)
		res[nodeDetails.ToNodeDetailsID()] = nodeDetails
	}
	return
}

func GenerateDKGID(index big.Int) DKGID {
	return DKGID(strings.Join([]string{"SHARING", index.Text(16)}, pcmn.Delimiter3))
}
