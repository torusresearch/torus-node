package keygennofsm

import (
	"errors"
	"fmt"
	"math/big"
	"math/rand"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/torusresearch/bijson"
	"github.com/torusresearch/torus-public/idmutex"
	"github.com/torusresearch/torus-public/logging"
	"github.com/torusresearch/torus-public/secp256k1"

	"github.com/stretchr/testify/assert"

	"github.com/torusresearch/torus-public/common"
	"github.com/torusresearch/torus-public/pvss"
)

func TestKeygenOptimistic(test *testing.T) {
	logging.SetLevelString("error")
	runtime.GOMAXPROCS(10)
	keys := 1
	n := 9
	k := 5
	t := 2
	sharedCh, generatedCh, nodes, nodeList, _ := SetupTestNodes(n, k, t)
	secretSets, sharingIDs := SeedKeys(keys, k, nodeList, nodes)
	masterSecrets := make(map[SharingID]*big.Int)
	for _, sharingID := range sharingIDs {
		for _, node := range nodes {
			keygenMsgShare := KeygenMsgShare{
				SharingID: sharingID,
			}
			keygenID := (&KeygenIDDetails{
				SharingID:   sharingID,
				DealerIndex: node.NodeDetails.Index,
			}).ToKeygenID()
			data, err := bijson.Marshal(keygenMsgShare)
			if err != nil {
				test.Fatal(err)
			}
			node.Transport.Send(node.NodeDetails, KeygenMessage{
				KeygenID: keygenID,
				Method:   "share",
				Data:     data,
			})
		}
		hindex, _ := strconv.Atoi((strings.Split(string(sharingID), "-")[1]))
		ms := pvss.SumScalars(secretSets[hindex]...)
		masterSecrets[sharingID] = &ms
	}
	completeMessages := 0
	completeEnough := make(chan bool)
	nizkpMessages := 0
	nizkpEnough := make(chan bool)
	go func() {
		for {
			msg := <-sharedCh
			if strings.Contains(msg, "shared") {
				completeMessages++
				if completeMessages == n*n*keys {
					go func() {
						completeEnough <- true
					}()
				}
			} else if strings.Contains(msg, "nizkp") {
				nizkpMessages++
				if nizkpMessages == n*keys {
					go func() {
						nizkpEnough <- true
					}()
				}
			}
		}
	}()

	<-completeEnough

	generatedMessages := 0
	generatedEnough := make(chan bool)
	go func() {
		for {
			msg := <-generatedCh
			if strings.Contains(msg, "generated") {
				generatedMessages++
				if generatedMessages == n*keys {
					go func() {
						generatedEnough <- true
					}()
				}
			}
		}
	}()

	<-generatedEnough

	assert.Equal(test, generatedMessages, n*keys)

	for _, sharingID := range sharingIDs {
		var shares []common.PrimaryShare
		for _, node := range nodes {
			var subshares []big.Int
			for _, noderef := range nodes { // assuming that all nodes are part of the valid set
				val := node.KeygenStore[(&KeygenIDDetails{
					SharingID:   sharingID,
					DealerIndex: noderef.NodeDetails.Index,
				}).ToKeygenID()].Si
				if val.Cmp(big.NewInt(int64(0))) != 0 {
					subshares = append(subshares, val)
				} else {
					assert.Fail(test, "fail")
				}
			}
			sumSi := pvss.SumScalars(subshares...)
			shares = append(shares, common.PrimaryShare{
				Index: node.NodeDetails.Index,
				Value: sumSi,
			})
		}
		reconstructedSecret := pvss.LagrangeScalar(shares[0:k], 0)
		assert.Equal(test, reconstructedSecret.Text(16), masterSecrets[sharingID].Text(16))
	}
	<-nizkpEnough
	for _, sharingID := range sharingIDs {
		var prevGS string
		for _, node := range nodes {
			dkg := node.DKGStore[sharingID]
			if dkg == nil {
				assert.Fail(test, "dkg is nil")
			}
			if prevGS != "" {
				assert.Equal(test, dkg.GS.X.Text(16), prevGS)
			} else {
				prevGS = dkg.GS.X.Text(16)
			}
		}
	}
}

var LocalNodeDirectory map[string]*LocalTransport

type Middleware func(KeygenMessage) (modifiedMessage KeygenMessage, end bool, err error)

type LocalOfflineTransport struct {
	OutputSharedChannel    *chan string
	OutputGeneratedChannel *chan string
}

func (l *LocalOfflineTransport) GetType() string {
	return "offline"
}

func (l *LocalOfflineTransport) SetKeygenNode(ref *KeygenNode) error {
	return nil
}

func (l *LocalOfflineTransport) SetTMEngine(ref *func(NodeDetails, KeygenMessage) error) error {
	return nil
}

func (l *LocalOfflineTransport) Send(nodeDetails NodeDetails, keygenMessage KeygenMessage) error {
	return nil
}

func (l *LocalOfflineTransport) Receive(senderDetails NodeDetails, keygenMessage KeygenMessage) error {
	return nil
}

func (l *LocalOfflineTransport) SendBroadcast(keygenMessage KeygenMessage) error {
	return nil
}

func (l *LocalOfflineTransport) ReceiveBroadcast(keygenMessage KeygenMessage) error {
	return nil
}

func (l *LocalOfflineTransport) Output(s string) {
	if strings.Contains(s, "shared") || strings.Contains(s, "nizkp") {
		go func() {
			*l.OutputSharedChannel <- "Output: " + s
		}()
	} else if strings.Contains(s, "generated") {
		go func() {
			*l.OutputGeneratedChannel <- "Output: " + s
		}()
	}
}

func (l *LocalOfflineTransport) Sign(s string) ([]byte, error) {
	return []byte{}, nil
}

type LocalTransport struct {
	KeygenNode             *KeygenNode
	PrivateKey             *big.Int
	NodeDirectory          *map[NodeDetailsID]KeygenTransport
	OutputSharedChannel    *chan string
	OutputGeneratedChannel *chan string
	SendMiddleware         []Middleware
	ReceiveMiddleware      []Middleware
	MockTMEngine           *func(NodeDetails, KeygenMessage) error
}

func (l *LocalTransport) GetType() string {
	return "local"
}

func (l *LocalTransport) SetKeygenNode(ref *KeygenNode) error {
	l.KeygenNode = ref
	return nil
}

func (l *LocalTransport) SetTMEngine(ref *func(NodeDetails, KeygenMessage) error) error {
	l.MockTMEngine = ref
	return nil
}

func (l *LocalTransport) Send(nodeDetails NodeDetails, keygenMessage KeygenMessage) error {
	modifiedMessage, err := l.runSendMiddleware(keygenMessage)
	if err != nil {
		return err
	}
	return (*l.NodeDirectory)[nodeDetails.ToNodeDetailsID()].Receive(l.KeygenNode.NodeDetails, modifiedMessage)
}

func (l *LocalTransport) Receive(senderDetails NodeDetails, keygenMessage KeygenMessage) error {
	modifiedMessage, err := l.runReceiveMiddleware(keygenMessage)
	if err != nil {
		return err
	}
	return l.KeygenNode.ProcessMessage(senderDetails, modifiedMessage)
}

func (l *LocalTransport) SendBroadcast(keygenMessage KeygenMessage) error {
	return (*l.MockTMEngine)(l.KeygenNode.NodeDetails, keygenMessage)
}

func (l *LocalTransport) ReceiveBroadcast(keygenMessage KeygenMessage) error {
	return l.KeygenNode.ProcessBroadcastMessage(keygenMessage)
}

func (l *LocalTransport) Output(s string) {
	if strings.Contains(s, "shared") || strings.Contains(s, "nizkp") {
		go func() {
			*l.OutputSharedChannel <- "Output: " + s
		}()
	} else if strings.Contains(s, "generated") {
		go func() {
			*l.OutputGeneratedChannel <- "Output: " + s
		}()
	}
}

func (l *LocalTransport) Sign(s string) ([]byte, error) {
	return pvss.ECDSASign(s, l.PrivateKey), nil
}

func (l *LocalTransport) runSendMiddleware(keygenMessage KeygenMessage) (KeygenMessage, error) {
	modifiedMessage := keygenMessage
	for _, middleware := range l.SendMiddleware {
		var end bool
		var err error
		modifiedMessage, end, err = middleware(modifiedMessage)
		if end {
			break
		}
		if err != nil {
			return keygenMessage, err
		}
	}
	return modifiedMessage, nil
}

func (l *LocalTransport) runReceiveMiddleware(keygenMessage KeygenMessage) (KeygenMessage, error) {
	modifiedMessage := keygenMessage
	for _, middleware := range l.ReceiveMiddleware {
		var end bool
		var err error
		modifiedMessage, end, err = middleware(modifiedMessage)
		if end {
			break
		}
		if err != nil {
			return keygenMessage, err
		}
	}
	return modifiedMessage, nil
}

func randIndexes(min int, max int, length int) (res []int) {
	if max < min {
		return
	}
	diff := max - min
	rand.Seed(time.Now().UnixNano())
	p := rand.Perm(diff)
	for _, r := range p[:length] {
		res = append(res, r+min)
	}
	return
}

func MockEngine(engineState *MockEngineState, localTransportNodeDirectory *map[NodeDetailsID]KeygenTransport, k int) func(NodeDetails, KeygenMessage) error {
	return func(senderDetails NodeDetails, keygenMessage KeygenMessage) error {
		if keygenMessage.Method != "propose" {
			return errors.New("MockTMEngine received keygenMessage with unimplemented method:" + keygenMessage.Method)
		}

		// parse message
		var keygenMsgPropose KeygenMsgPropose
		err := bijson.Unmarshal(keygenMessage.Data, &keygenMsgPropose)
		if err != nil {
			return errors.New("Could not unmarshal propose keygenMessage")
		}

		if len(keygenMsgPropose.Keygens) < k {
			return errors.New("Propose message did not have enough keygenids")
		}

		if len(keygenMsgPropose.Keygens) != len(keygenMsgPropose.SignedTexts) {
			return errors.New("Propose message had different lengths for keygenids and signedTexts")
		}

		for i, keygenid := range keygenMsgPropose.Keygens {
			var keygenIDDetails KeygenIDDetails
			err := keygenIDDetails.FromKeygenID(keygenid)
			if err != nil {
				return err
			}
			if keygenIDDetails.SharingID != keygenMsgPropose.SharingID {
				return errors.New("SharingID for keygenMsgPropose did not match keygenids")
			}
			for nodeDetailsID, signedText := range keygenMsgPropose.SignedTexts[i] {
				var nodeDetails NodeDetails
				nodeDetails.FromNodeDetailsID(nodeDetailsID)
				verified := pvss.ECDSAVerify(string(keygenid)+"|"+"ready", &nodeDetails.PubKey, signedText)
				if !verified {
					return errors.New("Could not verify signed text")
				}
			}
		}
		engineState.Lock()
		defer engineState.Unlock()
		if engineState.KeygenDecision[keygenMsgPropose.SharingID] == false {
			engineState.KeygenDecision[keygenMsgPropose.SharingID] = true
			logging.Info("Verified proposed message with keygenids:" + fmt.Sprint(keygenMsgPropose.Keygens))
			if len(*localTransportNodeDirectory) == 0 {
				return errors.New("localTransportNodeDirectory is empty, it may not have been initialized")
			}
			for _, l := range *localTransportNodeDirectory {
				if l == nil {
					return errors.New("Node directory contains empty transport pointer")
				}
				keygenMsgDecide := KeygenMsgDecide{
					SharingID: keygenMsgPropose.SharingID,
					Keygens:   keygenMsgPropose.Keygens,
				}
				data, err := bijson.Marshal(keygenMsgDecide)
				if err != nil {
					return err
				}
				go func(l KeygenTransport, data []byte) {
					err := l.ReceiveBroadcast(KeygenMessage{
						KeygenID: NullKeygenID,
						Method:   "decide",
						Data:     data,
					})
					if err != nil {
						logging.Error("Could not receive broadcast decide " + err.Error())
					}
				}(l, data)
			}
		} else {
			logging.Info("Already decided")
		}
		return nil
	}

}

type MockEngineState struct {
	idmutex.Mutex
	KeygenDecision map[SharingID]bool
}

func SetupTestNodes(n int, k int, t int) (chan string, chan string, []*KeygenNode, []common.Node, *map[NodeDetailsID]KeygenTransport) {
	// setup
	sharedCh := make(chan string)
	generatedCh := make(chan string)
	var currNodePrivKeys []big.Int
	var currNodePubKeys []common.Point
	for i := 0; i < n; i++ {
		currNodePrivKeys = append(currNodePrivKeys, *pvss.RandomBigInt())
		currNodePubKeys = append(currNodePubKeys, common.BigIntToPoint(secp256k1.Curve.ScalarBaseMult(currNodePrivKeys[i].Bytes())))
	}
	currNodeIndexes := randIndexes(1, 50, n)
	var currNodeList []common.Node
	for i := 0; i < n; i++ {
		currNodeList = append(currNodeList, common.Node{
			Index:  currNodeIndexes[i],
			PubKey: currNodePubKeys[i],
		})
	}
	localTransportNodeDirectory := make(map[NodeDetailsID]KeygenTransport)
	currEngineState := MockEngineState{
		KeygenDecision: make(map[SharingID]bool),
	}
	currRunEngine := MockEngine(&currEngineState, &localTransportNodeDirectory, k)
	var currNodes []*KeygenNode
	for i := 0; i < n; i++ {
		node := common.Node{
			Index:  currNodeIndexes[i],
			PubKey: currNodePubKeys[i],
		}
		localTransport := LocalTransport{
			NodeDirectory:          &localTransportNodeDirectory,
			PrivateKey:             &currNodePrivKeys[i],
			OutputSharedChannel:    &sharedCh,
			OutputGeneratedChannel: &generatedCh,
			MockTMEngine:           &currRunEngine,
		}
		newKeygenNode := NewKeygenNode(
			node,
			currNodeList,
			t,
			k,
			currNodeIndexes[i],
			&localTransport,
		)
		currNodes = append(currNodes, newKeygenNode)
		localTransport.SetKeygenNode(newKeygenNode)
		nodeDetails := NodeDetails(node)
		localTransportNodeDirectory[nodeDetails.ToNodeDetailsID()] = &localTransport
	}

	return sharedCh, generatedCh, currNodes, currNodeList, &localTransportNodeDirectory
}

func SeedKeys(keys int, k int, currNodeList []common.Node, currNodes []*KeygenNode) ([][]big.Int, []SharingID) {
	var sharingIDs []SharingID
	var secretSets [][]big.Int
	for h := 0; h < keys; h++ {
		var secrets []big.Int
		for _, node := range currNodes {
			index := node.NodeDetails.Index
			S := *pvss.RandomBigInt()
			Sprime := *pvss.RandomBigInt()
			secrets = append(secrets, S)
			sharingID := SharingID("SHARING-" + strconv.Itoa(h))
			sharingIDs = append(sharingIDs, sharingID)
			node.ShareStore[sharingID] = &Sharing{
				SharingID: sharingID,
				Nodes:     currNodeList,
				Epoch:     1,
				I:         index,
				S:         S,
				Sprime:    Sprime,
			}
		}
		secretSets = append(secretSets, secrets)
	}
	return secretSets, sharingIDs
}
