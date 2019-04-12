package keygen

import (
	"encoding/hex"
	"errors"
	"math/big"
	"strconv"
	"strings"
	"sync"

	"github.com/intel-go/fastjson"
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
type recoverState string

type phaseStates struct {
	Initial   phaseState
	Started   phaseState
	Proposing phaseState
	Ended     phaseState
}

type recoverStates struct {
	Initial                            recoverState
	WaitingForRecovers                 recoverState
	WaitingForThresholdSharing         recoverState
	WaitingForVBA                      recoverState
	WaitingForSelectedSharingsComplete recoverState
	Ended                              recoverState
}

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
	Phase         phaseState
	Dealer        dealerState
	Player        playerState
	Recover       recoverState
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
	Recover          recoverStates
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
	recoverStates{
		Initial:                            recoverState("initial"),
		WaitingForRecovers:                 recoverState("waitingforrecovers"),
		WaitingForThresholdSharing:         recoverState("waitingforthresholdsharing"),
		WaitingForVBA:                      recoverState("waitingforvba"),
		WaitingForSelectedSharingsComplete: recoverState("waitingforselectedsharingscomplete"),
		Ended:                              recoverState("ended"),
	},
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

func (pssMsgShare *PSSMsgShare) FromBytes(byt []byte) error {
	return fastjson.Unmarshal(byt, &pssMsgShare)
}

func (pssMsgShare *PSSMsgShare) ToBytes() []byte {
	byt, _ := fastjson.Marshal(pssMsgShare)
	return byt
}

type PSSMsgRecover struct {
	SharingID SharingID
	V         []common.Point
}

type pssMsgRecoverBase struct {
	SharingID string
	V         [][2]string
}

func (pssMsgRecover *PSSMsgRecover) ToBytes() []byte {
	p := pssMsgRecoverBase{
		SharingID: string(pssMsgRecover.SharingID),
		V:         BaseParser.MarshalPA(pssMsgRecover.V),
	}
	byt, _ := fastjson.Marshal(p)
	return byt
}

func (pssMsgRecover *PSSMsgRecover) FromBytes(data []byte) error {
	var p pssMsgRecoverBase
	err := fastjson.Unmarshal(data, &p)
	if err != nil {
		return err
	}
	pssMsgRecover.SharingID = SharingID(p.SharingID)
	pssMsgRecover.V = BaseParser.UnmarshalPA(p.V)
	return nil
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

type Parser interface {
	MarshalI(big.Int) string
	MarshalIA([]big.Int) []string
	MarshalIM([][]big.Int) [][]string
	UnmarshalI(string) big.Int
	UnmarshalIA([]string) []big.Int
	UnmarshalIM([][]string) [][]big.Int
	MarshalP(common.Point) [2]string
	MarshalPA([]common.Point) [][2]string
	MarshalPM([][]common.Point) [][][2]string
	UnmarshalP([2]string) common.Point
	UnmarshalPA([][2]string) []common.Point
	UnmarshalPM([][][2]string) [][]common.Point
}

var BaseParser = &parser{}

type parser struct {
}

func (p *parser) MarshalI(arg big.Int) string {
	return arg.Text(16)
}
func (p *parser) UnmarshalI(arg string) (res big.Int) {
	res.SetString(arg, 16)
	return
}
func (p *parser) MarshalIA(args []big.Int) (res []string) {
	for _, i := range args {
		res = append(res, p.MarshalI(i))
	}
	return
}
func (p *parser) UnmarshalIA(args []string) (res []big.Int) {
	for _, i := range args {
		res = append(res, p.UnmarshalI(i))
	}
	return
}
func (p *parser) MarshalIM(args [][]big.Int) (res [][]string) {
	for _, i := range args {
		res = append(res, p.MarshalIA(i))
	}
	return
}
func (p *parser) UnmarshalIM(args [][]string) (res [][]big.Int) {
	for _, i := range args {
		res = append(res, p.UnmarshalIA(i))
	}
	return
}
func (p *parser) MarshalP(arg common.Point) [2]string {
	return [2]string{arg.X.Text(16), arg.Y.Text(16)}
}
func (p *parser) UnmarshalP(arg [2]string) common.Point {
	var x, y big.Int
	x.SetString(arg[0], 16)
	y.SetString(arg[1], 16)
	return common.Point{X: x, Y: y}
}
func (p *parser) MarshalPA(args []common.Point) (res [][2]string) {
	for _, i := range args {
		res = append(res, p.MarshalP(i))
	}
	return
}
func (p *parser) UnmarshalPA(args [][2]string) (res []common.Point) {
	for _, i := range args {
		res = append(res, p.UnmarshalP(i))
	}
	return
}
func (p *parser) MarshalPM(args [][]common.Point) (res [][][2]string) {
	for _, i := range args {
		res = append(res, p.MarshalPA(i))
	}
	return
}
func (p *parser) UnmarshalPM(args [][][2]string) (res [][]common.Point) {
	for _, i := range args {
		res = append(res, p.UnmarshalPA(i))
	}
	return
}

type PSSMsgSend struct {
	PSSID  PSSID
	C      [][]common.Point
	A      []big.Int
	Aprime []big.Int
	B      []big.Int
	Bprime []big.Int
}

type pssMsgSendBase struct {
	PSSID  string        `json:"pssid"`
	C      [][][2]string `json:"c"`
	A      []string      `json:"a"`
	Aprime []string      `json:"aprime"`
	B      []string      `json:"b"`
	Bprime []string      `json:"bprime"`
}

func (pssMsgSend *PSSMsgSend) ToBytes() []byte {
	byt, _ := fastjson.Marshal(pssMsgSendBase{
		string(pssMsgSend.PSSID),
		BaseParser.MarshalPM(pssMsgSend.C),
		BaseParser.MarshalIA(pssMsgSend.A),
		BaseParser.MarshalIA(pssMsgSend.Aprime),
		BaseParser.MarshalIA(pssMsgSend.B),
		BaseParser.MarshalIA(pssMsgSend.Bprime),
	})
	return byt
}

func (pssMsgSend *PSSMsgSend) FromBytes(data []byte) error {
	var p pssMsgSendBase
	err := fastjson.Unmarshal(data, &p)
	if err != nil {
		return err
	}
	pssMsgSend.PSSID = PSSID(p.PSSID)
	pssMsgSend.C = BaseParser.UnmarshalPM(p.C)
	pssMsgSend.A = BaseParser.UnmarshalIA(p.A)
	pssMsgSend.Aprime = BaseParser.UnmarshalIA(p.Aprime)
	pssMsgSend.B = BaseParser.UnmarshalIA(p.B)
	pssMsgSend.Bprime = BaseParser.UnmarshalIA(p.Bprime)
	return nil
}

type PSSMsgEcho struct {
	PSSID      PSSID
	C          [][]common.Point
	Alpha      big.Int
	Alphaprime big.Int
	Beta       big.Int
	Betaprime  big.Int
}

type pssMsgEchoBase struct {
	PSSID      string        `json:"pssid"`
	C          [][][2]string `json:"c"`
	Alpha      string        `json:"alpha"`
	Alphaprime string        `json:"alphaprime"`
	Beta       string        `json:"beta"`
	Betaprime  string        `json:"betaprime"`
}

func (pssMsgEcho *PSSMsgEcho) ToBytes() []byte {
	byt, _ := fastjson.Marshal(pssMsgEchoBase{
		PSSID:      string(pssMsgEcho.PSSID),
		C:          BaseParser.MarshalPM(pssMsgEcho.C),
		Alpha:      BaseParser.MarshalI(pssMsgEcho.Alpha),
		Alphaprime: BaseParser.MarshalI(pssMsgEcho.Alphaprime),
		Beta:       BaseParser.MarshalI(pssMsgEcho.Beta),
		Betaprime:  BaseParser.MarshalI(pssMsgEcho.Betaprime),
	})
	return byt
}

func (pssMsgEcho *PSSMsgEcho) FromBytes(data []byte) error {
	var p pssMsgEchoBase
	err := fastjson.Unmarshal(data, &p)
	if err != nil {
		return err
	}
	pssMsgEcho.PSSID = PSSID(p.PSSID)
	pssMsgEcho.C = BaseParser.UnmarshalPM(p.C)
	pssMsgEcho.Alpha = BaseParser.UnmarshalI(p.Alpha)
	pssMsgEcho.Alphaprime = BaseParser.UnmarshalI(p.Alphaprime)
	pssMsgEcho.Beta = BaseParser.UnmarshalI(p.Beta)
	pssMsgEcho.Betaprime = BaseParser.UnmarshalI(p.Betaprime)
	return nil
}

type PSSMsgReady struct {
	PSSID      PSSID
	C          [][]common.Point
	Alpha      big.Int
	Alphaprime big.Int
	Beta       big.Int
	Betaprime  big.Int
}

type pssMsgReadyBase struct {
	PSSID      string        `json:"pssid"`
	C          [][][2]string `json:"c"`
	Alpha      string        `json:"alpha"`
	Alphaprime string        `json:"alphaprime"`
	Beta       string        `json:"beta"`
	Betaprime  string        `json:betaprime`
}

func (pssMsgReady *PSSMsgReady) ToBytes() []byte {
	byt, _ := fastjson.Marshal(pssMsgReadyBase{
		PSSID:      string(pssMsgReady.PSSID),
		C:          BaseParser.MarshalPM(pssMsgReady.C),
		Alpha:      BaseParser.MarshalI(pssMsgReady.Alpha),
		Alphaprime: BaseParser.MarshalI(pssMsgReady.Alphaprime),
		Beta:       BaseParser.MarshalI(pssMsgReady.Beta),
		Betaprime:  BaseParser.MarshalI(pssMsgReady.Betaprime),
	})
	return byt
}

func (pssMsgReady *PSSMsgReady) FromBytes(data []byte) error {
	var p pssMsgReadyBase
	err := fastjson.Unmarshal(data, &p)
	if err != nil {
		return err
	}
	pssMsgReady.PSSID = PSSID(p.PSSID)
	pssMsgReady.C = BaseParser.UnmarshalPM(p.C)
	pssMsgReady.Alpha = BaseParser.UnmarshalI(p.Alpha)
	pssMsgReady.Alphaprime = BaseParser.UnmarshalI(p.Alphaprime)
	pssMsgReady.Beta = BaseParser.UnmarshalI(p.Beta)
	pssMsgReady.Betaprime = BaseParser.UnmarshalI(p.Betaprime)
	return nil
}

type PSSMsgComplete struct {
	PSSID PSSID
	C00   common.Point
}

type pssMsgCompleteBase struct {
	PSSID string
	C00   [2]string
}

func (pssMsgComplete *PSSMsgComplete) ToBytes() []byte {
	var p pssMsgCompleteBase
	p.PSSID = string(pssMsgComplete.PSSID)
	p.C00 = BaseParser.MarshalP(pssMsgComplete.C00)
	byt, _ := fastjson.Marshal(p)
	return byt
}

func (pssMsgComplete *PSSMsgComplete) FromBytes(data []byte) error {
	var p pssMsgCompleteBase
	err := fastjson.Unmarshal(data, &p)
	if err != nil {
		return err
	}
	pssMsgComplete.PSSID = PSSID(p.PSSID)
	var x, y big.Int
	x.SetString(p.C00[0], 16)
	y.SetString(p.C00[1], 16)
	pssMsgComplete.C00 = common.Point{
		X: x,
		Y: y,
	}
	return nil
}

type PSSMsgPropose struct {
	NodeDetailsID NodeDetailsID
	PSSs          []PSSID
}

func (p *PSSMsgPropose) ToBytes() []byte {
	byt, _ := fastjson.Marshal(p)
	return byt
}

func (p *PSSMsgPropose) FromBytes(data []byte) error {
	return fastjson.Unmarshal(data, &p)
}

type VID string

type SharingID string
type Sharing struct {
	sync.Mutex
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
}

type PSS struct {
	sync.Mutex
	PSSID    PSSID
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
	CID       CID
	C         [][]common.Point
	EC        int
	RC        int
	AC        map[int]common.Point
	ACprime   map[int]common.Point
	BC        map[int]common.Point
	BCprime   map[int]common.Point
	Abar      []big.Int
	Abarprime []big.Int
	Bbar      []big.Int
	Bbarprime []big.Int
}

func GetPointArrayFromMap(m map[int]common.Point) (res []common.Point) {
	for _, pt := range m {
		res = append(res, pt)
	}
	return
}

type PSSMessage struct {
	PSSID  PSSID  `json:"pssid"`
	Method string `json:"type"`
	Data   []byte `json:"data"`
}

func (pssMessage *PSSMessage) JSON() *fastjson.RawMessage {
	json, err := fastjson.Marshal(pssMessage)
	if err != nil {
		return nil
	} else {
		res := fastjson.RawMessage(json)
		return &res
	}
}

// PSSID is the identifying string for PSSMessage
type PSSID string

const NullPSSID = PSSID("")

type PSSIDDetails struct {
	SharingID SharingID
	Index     int
}

func (pssIDDetails *PSSIDDetails) ToPSSID() PSSID {
	return PSSID(string(pssIDDetails.SharingID) + "|" + strconv.Itoa(pssIDDetails.Index))
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
	pssIDDetails.Index = index
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
	SetPSSNode(*PSSNode) error
	Send(NodeDetails, PSSMessage) error
	Receive(NodeDetails, PSSMessage) error
	Broadcast(PSSMessage) error
	ReceiveBroadcast(PSSMessage) error
	Output(string)
}

var LocalNodeDirectory map[string]*LocalTransport

type Middleware func(PSSMessage) (modifiedMessage PSSMessage, end bool, err error)
type LocalTransport struct {
	PSSNode           *PSSNode
	NodeDirectory     *map[NodeDetailsID]*LocalTransport
	OutputChannel     *chan string
	SendMiddleware    []Middleware
	ReceiveMiddleware []Middleware
	MockTMEngine      *func(NodeDetails, PSSMessage) error
}

func (l *LocalTransport) SetPSSNode(ref *PSSNode) error {
	l.PSSNode = ref
	return nil
}

func (l *LocalTransport) SetTMEngine(ref *func(NodeDetails, PSSMessage) error) error {
	l.MockTMEngine = ref
	return nil
}

func (l *LocalTransport) Send(nodeDetails NodeDetails, pssMessage PSSMessage) error {
	modifiedMessage, err := l.runSendMiddleware(pssMessage)
	if err != nil {
		return err
	}
	return (*l.NodeDirectory)[nodeDetails.ToNodeDetailsID()].Receive(l.PSSNode.NodeDetails, modifiedMessage)
}

func (l *LocalTransport) Receive(senderDetails NodeDetails, pssMessage PSSMessage) error {
	modifiedMessage, err := l.runReceiveMiddleware(pssMessage)
	if err != nil {
		return err
	}
	return l.PSSNode.ProcessMessage(senderDetails, modifiedMessage)
}

func (l *LocalTransport) Broadcast(pssMessage PSSMessage) error {
	(*l.MockTMEngine)(l.PSSNode.NodeDetails, pssMessage)
	return nil
}

func (l *LocalTransport) ReceiveBroadcast(pssMessage PSSMessage) error {
	return nil
}

func (l *LocalTransport) Output(s string) {
	if l.OutputChannel != nil {
		go func() {
			*l.OutputChannel <- "Output: " + s
		}()
	}
}

func (l *LocalTransport) runSendMiddleware(pssMessage PSSMessage) (PSSMessage, error) {
	modifiedMessage := pssMessage
	for _, middleware := range l.SendMiddleware {
		var end bool
		var err error
		modifiedMessage, end, err = middleware(modifiedMessage)
		if end {
			break
		}
		if err != nil {
			return pssMessage, err
		}
	}
	return modifiedMessage, nil
}

func (l *LocalTransport) runReceiveMiddleware(pssMessage PSSMessage) (PSSMessage, error) {
	modifiedMessage := pssMessage
	for _, middleware := range l.ReceiveMiddleware {
		var end bool
		var err error
		modifiedMessage, end, err = middleware(modifiedMessage)
		if end {
			break
		}
		if err != nil {
			return pssMessage, err
		}
	}
	return modifiedMessage, nil
}

type NodeNetwork struct {
	Nodes map[NodeDetailsID]NodeDetails
	N     int
	T     int
	K     int
	ID    string
}

type PSSNode struct {
	sync.Mutex
	NodeDetails  NodeDetails
	OldNodes     NodeNetwork
	NewNodes     NodeNetwork
	NodeIndex    big.Int
	ShareStore   map[SharingID]*Sharing
	RecoverStore map[SharingID]*Recover
	Transport    PSSTransport
	PSSStore     map[PSSID]*PSS
}
