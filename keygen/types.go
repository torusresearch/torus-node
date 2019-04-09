package keygen

import (
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"sync"

	"github.com/intel-go/fastjson"
	"github.com/torusresearch/torus-public/common"
	"github.com/torusresearch/torus-public/logging"
	"github.com/torusresearch/torus-public/secp256k1"
	fjson "github.com/valyala/fastjson"
)

// Dont try to cast, use the exported structs to get the types you need
type pssPhase string
type pssDealer bool
type pssPlayer bool
type pssReceivedSend bool
type pssReceivedEcho bool
type pssReceivedReady bool

type pssPhases struct {
	Initial   pssPhase
	Started   pssPhase
	Proposing pssPhase
	Ended     pssPhase
}

type pssDealers struct {
	IsDealer  pssDealer
	NotDealer pssDealer
}

type pssPlayers struct {
	IsPlayer  pssPlayer
	NotPlayer pssPlayer
}

type pssReceivedSends struct {
	True  pssReceivedSend
	False pssReceivedSend
}

type pssReceivedEchos struct {
	True  pssReceivedEcho
	False pssReceivedEcho
}

type pssReceivedReadys struct {
	True  pssReceivedReady
	False pssReceivedReady
}

type PSSState struct {
	Phase         pssPhase
	Dealer        pssDealer
	Player        pssPlayer
	ReceivedSend  pssReceivedSend
	ReceivedEcho  map[NodeDetailsID]pssReceivedEcho
	ReceivedReady map[NodeDetailsID]pssReceivedReady
}

var PSSTypes = struct {
	Phases           pssPhases
	Dealer           pssDealers
	Player           pssPlayers
	ReceivedSend     pssReceivedSends
	ReceivedEcho     pssReceivedEchos
	ReceivedEchoMap  map[NodeDetailsID]pssReceivedEcho
	ReceivedReady    pssReceivedReadys
	ReceivedReadyMap map[NodeDetailsID]pssReceivedReady
}{
	pssPhases{
		Initial:   pssPhase("initial"),
		Started:   pssPhase("started"),
		Proposing: pssPhase("proposing"),
		Ended:     pssPhase("ended"),
	},
	pssDealers{
		IsDealer:  pssDealer(true),
		NotDealer: pssDealer(false),
	},
	pssPlayers{
		IsPlayer:  pssPlayer(true),
		NotPlayer: pssPlayer(false),
	},
	pssReceivedSends{
		True:  pssReceivedSend(true),
		False: pssReceivedSend(false),
	},
	pssReceivedEchos{
		True:  pssReceivedEcho(true),
		False: pssReceivedEcho(false),
	},
	make(map[NodeDetailsID]pssReceivedEcho),
	pssReceivedReadys{
		True:  pssReceivedReady(true),
		False: pssReceivedReady(false),
	},
	make(map[NodeDetailsID]pssReceivedReady),
}

type PSSMsgShare struct {
	SharingID SharingID
}

func (pssMsgShare *PSSMsgShare) FromBytes(byt []byte) error {
	var p fjson.Parser
	val, err := p.Parse(string(byt))
	if err != nil {
		return err
	}
	pssMsgShare.SharingID = SharingID(val.GetStringBytes("sharingid"))
	return nil
}

func (pssMsgShare *PSSMsgShare) ToBytes() []byte {
	byt, err := fastjson.Marshal(pssMsgShare)
	if err != nil {
		logging.Error("Could not marshal PSSMsgShare struct")
		return []byte("")
	}
	return byt
}

type PSSMsgRecover struct {
	SharingID SharingID
}

func (pssMsgRecover *PSSMsgRecover) FromBytes(byt []byte) error {
	var p fjson.Parser
	val, err := p.Parse(string(byt))
	if err != nil {
		return err
	}
	pssMsgRecover.SharingID = SharingID(val.GetStringBytes("sharingid"))
	return nil
}

func (pssMsgRecover *PSSMsgRecover) ToBytes() []byte {
	byt, err := fastjson.Marshal(pssMsgRecover)
	if err != nil {
		logging.Error("Could not mpsshal PSSMsgRecover struct")
		return []byte("")
	}
	return byt
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
	PSSID  string
	C      [][][2]string
	A      []string
	Aprime []string
	B      []string
	Bprime []string
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
	PSSID      string
	C          [][][2]string
	Alpha      string
	Alphaprime string
	Beta       string
	Betaprime  string
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
	PSSID      string
	C          [][][2]string
	Alpha      string
	Alphaprime string
	Beta       string
	Betaprime  string
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
	CStore   map[CID]C
	State    PSSState
}

func GetCIDFromPointMatrix(pm [][]common.Point) CID {
	var bytes []byte
	for _, arr := range pm {
		for _, pt := range arr {
			bytes = append(bytes, pt.X.Bytes()...)
			bytes = append(bytes, pt.Y.Bytes()...)
		}
	}
	return CID(secp256k1.Keccak256(bytes))
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
	Send(NodeDetails, PSSMessage) error
	Receive(NodeDetails, PSSMessage) error
	Broadcast(NodeNetwork, PSSMessage) error
	Output(PSSMessage) error
}

var LocalNodeDirectory map[string]*LocalTransport

type LocalTransport struct {
	PSSNode       PSSNode
	NodeDirectory *map[NodeDetailsID]*LocalTransport
	// TODO: implement middleware feature
}

func (l *LocalTransport) Send(nodeDetails NodeDetails, pssMessage PSSMessage) error {
	return (*l.NodeDirectory)[nodeDetails.ToNodeDetailsID()].Receive(l.PSSNode.NodeDetails, pssMessage)
}

func (l *LocalTransport) Receive(senderDetails NodeDetails, pssMessage PSSMessage) error {
	return l.PSSNode.ProcessMessage(senderDetails, pssMessage)
}

func (l *LocalTransport) Broadcast(nodeNetwork NodeNetwork, pssMessage PSSMessage) error {
	for _, newNode := range nodeNetwork.Nodes {
		err := l.Send(NodeDetails(newNode), pssMessage)
		if err != nil {
			return err
		}
	}
	return nil
}

func (l *LocalTransport) Output(pssMessage PSSMessage) error {
	fmt.Println("OUTPUT:", pssMessage)
	return nil
}

type NodeNetwork struct {
	Nodes map[NodeDetailsID]NodeDetails
	N     int
	T     int
	K     int
	ID    string
}

type PSSNode struct {
	NodeDetails NodeDetails
	OldNodes    NodeNetwork
	NewNodes    NodeNetwork
	NodeIndex   big.Int
	ShareStore  map[SharingID]*Sharing
	Transport   PSSTransport
	PSSStore    map[PSSID]*PSS
}
