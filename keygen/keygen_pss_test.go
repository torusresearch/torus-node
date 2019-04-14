package keygen

import (
	"errors"
	"fmt"
	"math/big"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/torusresearch/bijson"
	"github.com/torusresearch/torus-public/logging"

	"github.com/stretchr/testify/assert"

	"github.com/torusresearch/torus-public/secp256k1"

	"github.com/torusresearch/torus-public/common"
	"github.com/torusresearch/torus-public/pvss"
)

type MockEngineState struct {
	sync.Mutex
	PSSDecision map[SharingID]bool
}

func SetupTestNodes(n, k, t int) (chan string, chan string, []*PSSNode, []common.Node) {
	// setup
	sharedCh := make(chan string)
	refreshedCh := make(chan string)
	var nodePrivKeys []big.Int
	var nodePubKeys []common.Point
	var nodeIndexes []int
	for i := 0; i < n; i++ {
		nodePrivKeys = append(nodePrivKeys, *pvss.RandomBigInt())
		nodePubKeys = append(nodePubKeys, common.BigIntToPoint(secp256k1.Curve.ScalarBaseMult(nodePrivKeys[i].Bytes())))
		nodeIndexes = append(nodeIndexes, i+1)
	}
	var nodeList []common.Node
	for i := 0; i < n; i++ {
		nodeList = append(nodeList, common.Node{
			Index:  nodeIndexes[i],
			PubKey: nodePubKeys[i],
		})
	}
	engineState := MockEngineState{
		PSSDecision: make(map[SharingID]bool),
	}
	localTransportNodeDirectory := make(map[NodeDetailsID]*LocalTransport)
	runEngine := func(senderDetails NodeDetails, pssMessage PSSMessage) error {
		// fmt.Println("MockTMEngine - Node:", nodeDetails.Index, " pssMessage:", pssMessage)
		if pssMessage.Method != "propose" {
			return errors.New("MockTMEngine received pssMessage with unimplemented method:" + pssMessage.Method)
		}

		// parse message
		var pssMsgPropose PSSMsgPropose
		err := bijson.Unmarshal(pssMessage.Data, &pssMsgPropose)
		if err != nil {
			return errors.New("Could not unmarshal propose pssMessage")
		}

		if len(pssMsgPropose.PSSs) < k {
			fmt.Println(len(pssMsgPropose.PSSs))
			return errors.New("Propose message did not have enough pssids")
		}

		if len(pssMsgPropose.PSSs) != len(pssMsgPropose.SignedTexts) {
			return errors.New("Propose message had different lengths for pssids and signedTexts")
		}

		for i, pssid := range pssMsgPropose.PSSs {
			var pssIDDetails PSSIDDetails
			err := pssIDDetails.FromPSSID(pssid)
			if err != nil {
				return err
			}
			if pssIDDetails.SharingID != pssMsgPropose.SharingID {
				return errors.New("SharingID for pssMsgPropose did not match pssids")
			}
			for nodeDetailsID, signedText := range pssMsgPropose.SignedTexts[i] {
				var nodeDetails NodeDetails
				nodeDetails.FromNodeDetailsID(nodeDetailsID)
				verified := pvss.ECDSAVerify(string(pssid)+"|"+"ready", &nodeDetails.PubKey, signedText)
				if !verified {
					return errors.New("Could not verify signed text")
				}
			}
		}
		engineState.Lock()
		defer engineState.Unlock()
		if engineState.PSSDecision[pssMsgPropose.SharingID] == false {
			engineState.PSSDecision[pssMsgPropose.SharingID] = true
			logging.Info("Verified proposed message with pssids:" + fmt.Sprint(pssMsgPropose.PSSs))
			if len(localTransportNodeDirectory) == 0 {
				return errors.New("localTransportNodeDirectory is empty, it may not have been initialized")
			}
			for _, l := range localTransportNodeDirectory {
				if l == nil {
					return errors.New("Node directory contains empty transport pointer")
				}
				pssMsgDecide := PSSMsgDecide{
					SharingID: pssMsgPropose.SharingID,
					PSSs:      pssMsgPropose.PSSs,
				}
				data, err := bijson.Marshal(pssMsgDecide)
				if err != nil {
					return err
				}
				go func(l *LocalTransport, data []byte) {
					l.ReceiveBroadcast(PSSMessage{
						PSSID:  NullPSSID,
						Method: "decide",
						Data:   data,
					})
				}(l, data)
			}
		} else {
			logging.Info("Already decided")
		}
		return nil
	}
	var nodes []*PSSNode
	for i := 0; i < n; i++ {
		node := common.Node{
			Index:  nodeIndexes[i],
			PubKey: nodePubKeys[i],
		}
		localTransport := LocalTransport{
			NodeDirectory:          &localTransportNodeDirectory,
			OutputSharedChannel:    &sharedCh,
			OutputRefreshedChannel: &refreshedCh,
			MockTMEngine:           &runEngine,
			PrivateKey:             &nodePrivKeys[i],
		}
		newPssNode := NewPSSNode(
			node,
			nodeList,
			t,
			k,
			nodeList,
			t,
			k,
			*big.NewInt(int64(i + 1)),
			&localTransport,
		)
		nodes = append(nodes, newPssNode)
		localTransport.SetPSSNode(newPssNode)
		nodeDetails := NodeDetails(node)
		localTransportNodeDirectory[nodeDetails.ToNodeDetailsID()] = &localTransport
	}
	return sharedCh, refreshedCh, nodes, nodeList
}

func TestKeygenSharing(test *testing.T) {
	logging.SetLevelString("error")
	runtime.GOMAXPROCS(10)
	keys := 1
	n := 9
	k := 5
	t := 2
	sharedCh, _, nodes, nodeList := SetupTestNodes(n, k, t)
	fmt.Println("Running TestKeygenSharing for " + strconv.Itoa(keys) + " keys, " + strconv.Itoa(n) + " nodes with reconstruction " + strconv.Itoa(k) + " and threshold " + strconv.Itoa(t))
	var secrets []big.Int
	var sharingIDs []SharingID
	for h := 0; h < keys; h++ {
		secret := pvss.RandomBigInt()
		secrets = append(secrets, *secret)
		mask := pvss.RandomBigInt()
		randPoly := pvss.RandomPoly(*secret, k)
		randPolyprime := pvss.RandomPoly(*mask, k)
		commit := pvss.GetCommit(*randPoly)
		commitH := pvss.GetCommitH(*randPolyprime)
		sumCommitments := pvss.AddCommitments(commit, commitH)
		sharingID := SharingID("SHARING" + strconv.Itoa(h))
		sharingIDs = append(sharingIDs, sharingID)
		for _, node := range nodes {
			index := node.NodeDetails.Index
			Si := *pvss.PolyEval(*randPoly, *big.NewInt(int64(index)))
			Siprime := *pvss.PolyEval(*randPolyprime, *big.NewInt(int64(index)))
			node.ShareStore[sharingID] = &Sharing{
				SharingID: sharingID,
				Nodes:     nodeList,
				Epoch:     1,
				I:         index,
				Si:        Si,
				Siprime:   Siprime,
				C:         sumCommitments,
			}
		}
	}
	for _, sharingID := range sharingIDs {
		for _, node := range nodes {
			pssMsgShare := PSSMsgShare{
				SharingID: sharingID,
			}
			pssID := (&PSSIDDetails{
				SharingID: sharingID,
				Index:     node.NodeDetails.Index,
			}).ToPSSID()
			data, err := bijson.Marshal(pssMsgShare)
			if err != nil {
				test.Fatal(err)
			}
			err = node.Transport.Send(node.NodeDetails, PSSMessage{
				PSSID:  pssID,
				Method: "share",
				Data:   data,
			})
			assert.NoError(test, err)
		}
	}
	completeMessages := 0
	for completeMessages < n*n*keys {
		msg := <-sharedCh
		if strings.Contains(msg, "shared") {
			completeMessages++
		} else {
			assert.Fail(test, "did not get the required number of share complete messages")
		}
	}
	assert.Equal(test, completeMessages, n*n*keys)
	for g, sharingID := range sharingIDs {
		var shares []common.PrimaryShare
		for _, node := range nodes {
			var subshares []common.PrimaryShare
			for _, noderef := range nodes { // assuming that all nodes are part of the valid set
				val := node.PSSStore[(&PSSIDDetails{
					SharingID: sharingID,
					Index:     noderef.NodeDetails.Index,
				}).ToPSSID()].Si
				if val.Cmp(big.NewInt(int64(0))) != 0 {
					subshares = append(subshares, common.PrimaryShare{
						Index: noderef.NodeDetails.Index,
						Value: val,
					})
				}
			}
			reconstructedSi := pvss.LagrangeScalar(subshares[0:5], 0)
			shares = append(shares, common.PrimaryShare{
				Index: node.NodeDetails.Index,
				Value: *reconstructedSi,
			})
		}
		reconstructedSecret := pvss.LagrangeScalar(shares[2:7], 0)
		assert.Equal(test, reconstructedSecret.Text(16), secrets[g].Text(16))
	}

	time.Sleep(5 * time.Second)
	sharingID := sharingIDs[0]
	var pts []common.Point
	for _, node := range nodes {
		pts = append(pts, common.Point{
			X: *big.NewInt(int64(node.NodeDetails.Index)),
			Y: node.RecoverStore[sharingID].Si,
		})
	}
	fmt.Println(pvss.LagrangeScalarCP(pts, 0).Text(16))
	fmt.Println(secrets[0].Text(16))
}

var LocalNodeDirectory map[string]*LocalTransport

type Middleware func(PSSMessage) (modifiedMessage PSSMessage, end bool, err error)
type LocalTransport struct {
	PSSNode                *PSSNode
	PrivateKey             *big.Int
	NodeDirectory          *map[NodeDetailsID]*LocalTransport
	OutputSharedChannel    *chan string
	OutputRefreshedChannel *chan string
	SendMiddleware         []Middleware
	ReceiveMiddleware      []Middleware
	MockTMEngine           *func(NodeDetails, PSSMessage) error
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

func (l *LocalTransport) SendBroadcast(pssMessage PSSMessage) error {
	return (*l.MockTMEngine)(l.PSSNode.NodeDetails, pssMessage)
}

func (l *LocalTransport) ReceiveBroadcast(pssMessage PSSMessage) error {
	return l.PSSNode.ProcessBroadcastMessage(pssMessage)
}

func (l *LocalTransport) Output(s string) {
	if strings.Contains(s, "shared") {
		go func() {
			*l.OutputSharedChannel <- "Output: " + s
		}()
	} else if strings.Contains(s, "refreshed") {
		go func() {
			*l.OutputRefreshedChannel <- "Output: " + s
		}()
	}
}

func (l *LocalTransport) Sign(s string) ([]byte, error) {
	return pvss.ECDSASign(s, l.PrivateKey), nil
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
