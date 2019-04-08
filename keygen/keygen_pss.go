package keygen

import (
	"errors"
	"math/big"
	"strconv"
	"strings"
	"sync"

	"github.com/torusresearch/torus-public/pvss"

	"github.com/intel-go/fastjson"

	"github.com/torusresearch/torus-public/common"
)

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
	F        [][]big.Int
	Fprime   [][]big.Int
	Messages []PSSMessage
	CStore   map[PSSCID]PSSC
	State    PSSState
}

type PSSCID string

type PSSC struct {
	PSSCID  PSSCID
	C       [][]common.Point
	EC      int
	RC      int
	AC      []common.Point
	ACprime []common.Point
	BC      []common.Point
	BCprime []common.Point
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
	ShareStore  map[SharingID]*Sharing
	Transport   PSSTransport
	PSSStore    map[PSSID]*PSS
}

func (pssNode *PSSNode) ProcessMessage(nodeDetails NodeDetails, pssMessage PSSMessage) error {
	if _, found := pssNode.PSSStore[pssMessage.PSSID]; !found {
		pssNode.PSSStore[pssMessage.PSSID] = &PSS{
			PSSID: pssMessage.PSSID,
			State: PSSState{
				PSSTypes.Phases.Initial,
				PSSTypes.Dealer.NotDealer,
				PSSTypes.Player.NotPlayer,
				PSSTypes.ReceivedSend.False,
			},
		}
	}
	pss := pssNode.PSSStore[pssMessage.PSSID]
	pss.Mutex.Lock()
	defer pss.Mutex.Unlock()

	pss.Messages = append(pss.Messages, pssMessage)

	// handle different messages here
	if pssMessage.Method == "share" {
		// state checks
		if pss.State.Phase == PSSTypes.Phases.Ended {
			return errors.New("PSS has ended, ignored message " + string(*pssMessage.JSON()) + " from " + string(nodeDetails.ToNodeDetailsID()) + " ")
		}
		if pss.State.Dealer != PSSTypes.Dealer.IsDealer {
			return errors.New("PSS could not be started since the node is not a dealer")
		}

		// logic
		var pssMsgShare PSSMsgShare
		err := pssMsgShare.FromBytes(pssMessage.Data)
		if err != nil {
			return errors.New("Could not unmarshal data for pssMsgShare")
		}
		sharing, found := pssNode.ShareStore[pssMsgShare.SharingID]
		if !found {
			return errors.New("Could not find sharing for sharingID " + string(pssMsgShare.SharingID))
		}
		pss.F = pvss.GenerateRandomBivariatePolynomial(sharing.Si, pssNode.NewNodes.K)
		pss.Fprime = pvss.GenerateRandomBivariatePolynomial(sharing.Siprime, pssNode.NewNodes.K)
		// pss.C = pvss.GetCommitmentMatrix(pss.F, pss.Fprime)

		for _, newNode := range pssNode.NewNodes.Nodes {
			nodeDetails := NodeDetails(newNode)
			pssID := (&PSSIDDetails{
				SharingID: sharing.SharingID,
				Index:     sharing.I,
			}).ToPSSID()
			pssMsgSend := &PSSMsgSend{
				PSSID: pssID,
				// C:      pss.C,
				A:      pvss.EvaluateBivarPolyAtX(pss.F, *big.NewInt(int64(nodeDetails.Index))).Coeff,
				Aprime: pvss.EvaluateBivarPolyAtX(pss.Fprime, *big.NewInt(int64(nodeDetails.Index))).Coeff,
				B:      pvss.EvaluateBivarPolyAtY(pss.F, *big.NewInt(int64(nodeDetails.Index))).Coeff,
				Bprime: pvss.EvaluateBivarPolyAtY(pss.Fprime, *big.NewInt(int64(nodeDetails.Index))).Coeff,
			}
			nextPSSMessage := PSSMessage{
				PSSID:  pssID,
				Method: "send",
				Data:   pssMsgSend.ToBytes(),
			}
			go func() {
				pssNode.Transport.Send(nodeDetails, nextPSSMessage)
			}()
		}
	} else if pssMessage.Method == "send" {
		// state checks
		if pss.State.Phase == PSSTypes.Phases.Ended {
			return errors.New("PSS has ended, ignored message " + string(*pssMessage.JSON()) + " from " + string(nodeDetails.ToNodeDetailsID()) + " ")
		}
		if pss.State.Player != PSSTypes.Player.IsPlayer {
			return errors.New("Could not receive send message because node is not a player")
		}
		if pss.State.ReceivedSend == PSSTypes.ReceivedSend.True {
			return errors.New("Already received a send message for PSSID " + string(pss.PSSID))
		}

		// logic
		var pssMsgSend PSSMsgSend
		err := pssMsgSend.FromBytes(pssMessage.Data)
		if err != nil {
			return err
		}
		// pvss.AVSSVerifyPoly(pssMsgSend.C)

	} else if pssMessage.Method == "echo" {
		// state checks
		if pss.State.Phase == PSSTypes.Phases.Ended {
			return errors.New("PSS has ended, ignored message " + string(*pssMessage.JSON()) + " from " + string(nodeDetails.ToNodeDetailsID()) + " ")
		}

		// logic

	} else if pssMessage.Method == "ready" {
		// state checks
		if pss.State.Phase == PSSTypes.Phases.Ended {
			return errors.New("PSS has ended, ignored message " + string(*pssMessage.JSON()) + " from " + string(nodeDetails.ToNodeDetailsID()) + " ")
		}

		// logic

	} else if pssMessage.Method == "shared" {
		// state checks
		if pss.State.Phase == PSSTypes.Phases.Ended {
			return errors.New("PSS has ended, ignored message " + string(*pssMessage.JSON()) + " from " + string(nodeDetails.ToNodeDetailsID()) + " ")
		}

		// logic

	} else {
		return errors.New("PssMessage method not found")
	}
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
	shareStore map[SharingID]*Sharing,
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
