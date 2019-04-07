package keygen

import (
	"math/big"
	"strconv"
	"strings"
	"sync"

	"github.com/intel-go/fastjson"

	"github.com/torusresearch/torus-public/common"
)

type Sharing struct {
	Nodes   []common.Node
	Epoch   int
	PSSID   PSSID
	I       int
	Si      big.Int
	Siprime big.Int
	C       []common.Point
}

type PSS struct {
	sync.Mutex
	ID       big.Int
	Messages []PSSMessage
	C        [][]common.Point
	EC       int
	RC       int
	AC       []common.Point
	ACprime  []common.Point
	BC       []common.Point
	BCprime  []common.Point
	State    string
}

type PSSMessage struct {
	PSSID PSSID  `json:"pssid"`
	Type  string `json:"type"`
	Data  string `json:"data"`
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
	ID   big.Int
	Step string
	From int
	To   int
}

func (pssIDDetails *PSSIDDetails) ToString() string {
	return pssIDDetails.ID.Text(16) + "|" + pssIDDetails.Step + "|" + strconv.Itoa(pssIDDetails.From) + "|" + strconv.Itoa(pssIDDetails.To)
}
func (pssIDDetails *PSSIDDetails) FromString(s string) {
	substrings := strings.Split(s, "|")
	if len(substrings) != 4 {
		return
	}
	id, ok := new(big.Int).SetString(substrings[0], 16)
	if !ok {
		return
	}
	pssIDDetails.ID = *id
	pssIDDetails.Step = substrings[1]
	from, err := strconv.Atoi(substrings[2])
	if err != nil {
		return
	}
	pssIDDetails.From = from
	to, err := strconv.Atoi(substrings[3])
	if err != nil {
		return
	}
	pssIDDetails.To = to
}

type NodeDetails common.Node

func (n *NodeDetails) ToString() string {
	return strconv.Itoa(n.Index) + "|" + n.PubKey.X.Text(16) + "|" + n.PubKey.Y.Text(16)
}
func (n *NodeDetails) FromString(s string) {
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
}

var LocalNodeDirectory map[string]*LocalTransport

type LocalTransport struct {
	PSSNode       PSSNode
	NodeDirectory *map[string]*LocalTransport
}

func (l *LocalTransport) Send(nodeDetails NodeDetails, pssMessage PSSMessage) error {
	return (*l.NodeDirectory)[nodeDetails.ToString()].Receive(l.PSSNode.NodeDetails, pssMessage)
}

func (l *LocalTransport) Receive(senderDetails NodeDetails, pssMessage PSSMessage) error {
	return l.PSSNode.ProcessMessage(senderDetails, pssMessage)
}

type NodeNetwork struct {
	Nodes []common.Node
	T     int
	K     int
	ID    string
}

type PSSNode struct {
	NodeDetails NodeDetails
	OldNodes    NodeNetwork
	NewNodes    NodeNetwork
	NodeIndex   big.Int
	ShareStore  map[string]Sharing
	Transport   PSSTransport
	Resharings  []PSS
}

func (pssNode *PSSNode) ProcessMessage(nodeDetails NodeDetails, pssMessage PSSMessage) error {
	return nil
}

func NewPSSNode(
	nodeDetails common.Node,
	oldNodeList []common.Node,
	oldNodesT int,
	oldNodesK int,
	newNodeList []common.Node,
	newNodesT int,
	newNodesK int,
	nodeIndex big.Int,
	shareStore map[string]Sharing,
	transport PSSTransport,
) *PSSNode {
	return &PSSNode{
		NodeDetails: NodeDetails(nodeDetails),
		OldNodes: NodeNetwork{
			Nodes: oldNodeList,
			T:     oldNodesT,
			K:     oldNodesK,
		},
		NewNodes: NodeNetwork{
			Nodes: newNodeList,
			T:     newNodesT,
			K:     newNodesK,
		},
		NodeIndex:  nodeIndex,
		ShareStore: shareStore,
		Transport:  transport,
	}
}
