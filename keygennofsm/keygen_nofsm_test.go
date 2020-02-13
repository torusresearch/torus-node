package keygennofsm

import (
	"errors"
	"math/big"
	"runtime"
	"strings"
	"testing"

	logging "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/torusresearch/bijson"
	"github.com/torusresearch/torus-common/common"
	"github.com/torusresearch/torus-common/secp256k1"
	pcmn "github.com/torusresearch/torus-node/common"
	"github.com/torusresearch/torus-node/idmutex"
	"github.com/torusresearch/torus-node/pvss"
)

func TestKeygenOptimistic(test *testing.T) {
	logging.SetLevel(logging.ErrorLevel)
	keys := 10
	n := 9
	k := 5
	t := 2
	sharedCh, generatedCh, nodes, nodeList, _, mockEngineState := SetupTestNodes(n, k, t)
	secretSets, dkgIDs := SeedKeys(keys, k, nodeList, nodes)
	// masterSecrets := make(map[DKGID]*big.Int)
	for _, dkgID := range dkgIDs {
		for _, node := range nodes {
			keygenMsgShare := KeygenMsgShare{
				DKGID: dkgID,
			}
			keygenID := (&KeygenIDDetails{
				DKGID:       dkgID,
				DealerIndex: node.NodeDetails.Index,
			}).ToKeygenID()
			data, err := bijson.Marshal(keygenMsgShare)
			if err != nil {
				test.Fatal(err)
			}
			_ = node.Transport.Send(node.NodeDetails, CreateKeygenMessage(KeygenMessageRaw{
				KeygenID: keygenID,
				Method:   "share",
				Data:     data,
			}))
		}
		// hindex, ok := new(big.Int).SetString(strings.Split(string(dkgID), pcmn.Delimiter3)[1], 16)
		// if !ok {
		// 	logging.WithField("DKGID", dkgID).Error("Could not set string for big int")
		// 	return
		// }
		// ms := pvss.SumScalars(secretSets[hindex.Int64()]...)
		// masterSecrets[dkgID] = &ms
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

	masterSecrets := make(map[DKGID]big.Int)
	for _, dkgID := range dkgIDs {
		var shares []pcmn.PrimaryShare
		selectedKeygenIDs := mockEngineState.KeygenDecision[dkgID]
		var selectedIndexes []int
		for _, keygenID := range selectedKeygenIDs {
			var keygenIDDetails KeygenIDDetails
			_ = keygenIDDetails.FromKeygenID(keygenID)
			selectedIndexes = append(selectedIndexes, keygenIDDetails.DealerIndex)
		}
		if len(selectedKeygenIDs) == 0 {
			assert.Fail(test, "Could not get selected keygenIDs")
		}
		for _, node := range nodes {
			var subshares []big.Int
			for _, keygenID := range selectedKeygenIDs {
				keygen, ok := node.KeygenStore.Get(keygenID)
				if !ok {
					assert.Fail(test, "Could not find keygen")
				}
				keygen.Lock()
				val := keygen.Si
				keygen.Unlock()
				if val.Cmp(big.NewInt(int64(0))) != 0 {
					subshares = append(subshares, val)
				} else {
					assert.Fail(test, "fail")
				}
			}
			sumSi := pvss.SumScalars(subshares...)
			shares = append(shares, pcmn.PrimaryShare{
				Index: node.NodeDetails.Index,
				Value: sumSi,
			})
		}
		reconstructedSecret := pvss.LagrangeScalar(shares[0:k], 0)
		dkgIDIndex, err := dkgID.GetIndex()
		if err != nil {
			assert.Fail(test, "could not get dkgID Index")
		}
		dealerSecrets := secretSets[int(dkgIDIndex.Int64())]
		masterSecret := big.NewInt(0)
		for _, selectedIndex := range selectedIndexes {
			dealerSecret := dealerSecrets[selectedIndex]
			masterSecret = masterSecret.Add(masterSecret, &dealerSecret)
		}
		masterSecret = masterSecret.Mod(masterSecret, secp256k1.GeneratorOrder)
		masterSecrets[dkgID] = *masterSecret
		assert.Equal(test, reconstructedSecret.Text(16), masterSecret.Text(16))
	}
	<-nizkpEnough
	for _, dkgID := range dkgIDs {
		var prevGS string
		for _, node := range nodes {
			dkg, ok := node.DKGStore.Get(dkgID)
			if !ok {
				assert.Fail(test, "Could not find dkg")
			}
			if dkg == nil {
				assert.Fail(test, "dkg is nil")
			}
			dkg.Lock()
			if prevGS != "" {
				assert.Equal(test, dkg.GS.X.Text(16), prevGS)
			} else {
				prevGS = dkg.GS.X.Text(16)
				masterSecret := masterSecrets[dkgID]
				masterPubKey := common.BigIntToPoint(secp256k1.Curve.ScalarBaseMult(masterSecret.Bytes()))
				assert.Equal(test, prevGS, masterPubKey.X.Text(16))
			}
			dkg.Unlock()
		}
	}
}

func TestKeygenSomeOffline(test *testing.T) {
	logging.SetLevel(logging.ErrorLevel)
	runtime.GOMAXPROCS(10)
	keys := 40
	n := 5
	k := 3
	t := 1
	sharedCh, generatedCh, nodes, nodeList, _, mockEngineState := SetupTestNodes(n, k, t)
	nodes[n-1].Transport = &LocalOfflineTransport{}
	secretSets, dkgIDs := SeedKeys(keys, k, nodeList, nodes)
	// masterSecrets := make(map[DKGID]*big.Int)
	for _, dkgID := range dkgIDs {
		for _, node := range nodes {
			keygenMsgShare := KeygenMsgShare{
				DKGID: dkgID,
			}
			keygenID := (&KeygenIDDetails{
				DKGID:       dkgID,
				DealerIndex: node.NodeDetails.Index,
			}).ToKeygenID()
			data, err := bijson.Marshal(keygenMsgShare)
			if err != nil {
				test.Fatal(err)
			}
			_ = node.Transport.Send(node.NodeDetails, CreateKeygenMessage(KeygenMessageRaw{
				KeygenID: keygenID,
				Method:   "share",
				Data:     data,
			}))
		}
		// hindex, ok := new(big.Int).SetString(strings.Split(string(dkgID), pcmn.Delimiter3)[1], 16)
		// if !ok {
		// 	logging.WithField("DKGID", dkgID).Error("Could not set string for big int")
		// 	return
		// }
		// ms := pvss.SumScalars(secretSets[hindex.Int64()]...)
		// masterSecrets[dkgID] = &ms
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
				if completeMessages == (n-1)*(n-1)*keys {
					go func() {
						completeEnough <- true
					}()
				}
				if completeMessages > (n-1)*(n-1)*keys {
					assert.Fail(test, "more nizkp messages than expected")
				}
			} else if strings.Contains(msg, "nizkp") {
				nizkpMessages++
				if nizkpMessages == (n-1)*keys {
					go func() {
						nizkpEnough <- true
					}()
				}
				if nizkpMessages > (n-1)*keys {
					assert.Fail(test, "more nizkp messages than expected")
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
				if generatedMessages == (n-1)*keys {
					go func() {
						generatedEnough <- true
					}()
				}
				if generatedMessages > (n-1)*keys {
					assert.Fail(test, "more generated messages than expected")
				}
			}
		}
	}()

	<-generatedEnough

	assert.Equal(test, generatedMessages, (n-1)*keys)

	masterSecrets := make(map[DKGID]big.Int)
	for _, dkgID := range dkgIDs {
		var shares []pcmn.PrimaryShare
		selectedKeygenIDs := mockEngineState.KeygenDecision[dkgID]
		var selectedIndexes []int
		for _, keygenID := range selectedKeygenIDs {
			var keygenIDDetails KeygenIDDetails
			_ = keygenIDDetails.FromKeygenID(keygenID)
			selectedIndexes = append(selectedIndexes, keygenIDDetails.DealerIndex)
		}
		if len(selectedKeygenIDs) == 0 {
			assert.Fail(test, "Could not get selected keygenIDs")
		}
		for _, node := range nodes {
			var subshares []big.Int
			for _, keygenID := range selectedKeygenIDs {
				kStore, ok := node.KeygenStore.Get(keygenID)
				if !ok {
					assert.Fail(test, "could not get back keygen store")
				}
				val := kStore.Si
				if val.Cmp(big.NewInt(int64(0))) != 0 {
					subshares = append(subshares, val)
				} else {
					assert.Fail(test, "fail")
				}
			}
			sumSi := pvss.SumScalars(subshares...)
			shares = append(shares, pcmn.PrimaryShare{
				Index: node.NodeDetails.Index,
				Value: sumSi,
			})
		}
		reconstructedSecret := pvss.LagrangeScalar(shares[0:k], 0)
		dkgIDIndex, err := dkgID.GetIndex()
		if err != nil {
			assert.Fail(test, "could not get dkgID Index")
		}
		dealerSecrets := secretSets[int(dkgIDIndex.Int64())]
		masterSecret := big.NewInt(0)
		for _, selectedIndex := range selectedIndexes {
			dealerSecret := dealerSecrets[selectedIndex]
			masterSecret = masterSecret.Add(masterSecret, &dealerSecret)
		}
		masterSecret = masterSecret.Mod(masterSecret, secp256k1.GeneratorOrder)
		masterSecrets[dkgID] = *masterSecret
		assert.Equal(test, reconstructedSecret.Text(16), masterSecret.Text(16))
	}
	<-nizkpEnough
	for _, dkgID := range dkgIDs {
		var prevGS string
		for _, node := range nodes {
			if _, ok := node.Transport.(*LocalOfflineTransport); ok {
				continue
			}
			dkg, ok := node.DKGStore.Get(dkgID)
			if !ok {
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

// var LocalNodeDirectory map[string]*LocalTransport

type Middleware func(KeygenMessage) (modifiedMessage KeygenMessage, end bool, err error)

type LocalOfflineTransport struct {
}

func (l *LocalOfflineTransport) Init() {}
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
func (l *LocalOfflineTransport) Sign(s []byte) ([]byte, error) {
	return []byte{}, nil
}
func (l *LocalOfflineTransport) Output(sinter interface{}) {}
func (l *LocalOfflineTransport) CheckIfNIZKPProcessed(keyIndex big.Int) bool {
	return false
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

func (l *LocalTransport) Init() {}
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
func (l *LocalTransport) Sign(s []byte) ([]byte, error) {
	return pvss.ECDSASignBytes(s, l.PrivateKey), nil
}
func (l *LocalTransport) Output(sinter interface{}) {
	s, ok := sinter.(string)
	if !ok {
		s = "not a string output"
	}
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
func (l *LocalTransport) CheckIfNIZKPProcessed(keyIndex big.Int) bool {
	return false
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

		if len(keygenMsgPropose.Keygens) != len(keygenMsgPropose.ProposeProofs) {
			return errors.New("Propose message had different lengths for keygenids and signedTexts")
		}

		for i, keygenid := range keygenMsgPropose.Keygens {
			var keygenIDDetails KeygenIDDetails
			err := keygenIDDetails.FromKeygenID(keygenid)
			if err != nil {
				return err
			}
			if keygenIDDetails.DKGID != keygenMsgPropose.DKGID {
				return errors.New("DKGID for keygenMsgPropose did not match keygenids")
			}
			for nodeDetailsID, proposeProof := range keygenMsgPropose.ProposeProofs[i] {
				var nodeDetails NodeDetails
				nodeDetails.FromNodeDetailsID(nodeDetailsID)
				signedTextDetails := SignedTextDetails{
					Text: strings.Join([]string{string(keygenid), "ready"}, pcmn.Delimiter1),
					C00:  proposeProof.C00,
				}
				// Note: this does not check if the pubkey is in the list
				verified := pvss.ECDSAVerifyBytes(signedTextDetails.ToBytes(), &nodeDetails.PubKey, proposeProof.SignedTextDetails)
				if !verified {
					return errors.New("Could not verify signed text")
				}
			}
		}
		engineState.Lock()
		defer engineState.Unlock()
		if len(engineState.KeygenDecision[keygenMsgPropose.DKGID]) == 0 {
			engineState.KeygenDecision[keygenMsgPropose.DKGID] = keygenMsgPropose.Keygens
			logging.WithField("keygenids", keygenMsgPropose.Keygens).Info("verified proposed message")
			if len(*localTransportNodeDirectory) == 0 {
				return errors.New("localTransportNodeDirectory is empty, it may not have been initialized")
			}
			for _, l := range *localTransportNodeDirectory {
				if l == nil {
					return errors.New("Node directory contains empty transport pointer")
				}
				keygenMsgDecide := KeygenMsgDecide{
					DKGID:   keygenMsgPropose.DKGID,
					Keygens: keygenMsgPropose.Keygens,
				}
				data, err := bijson.Marshal(keygenMsgDecide)
				if err != nil {
					return err
				}
				go func(l KeygenTransport, data []byte) {
					err := l.ReceiveBroadcast(CreateKeygenMessage(KeygenMessageRaw{
						KeygenID: NullKeygenID,
						Method:   "decide",
						Data:     data,
					}))
					if err != nil {
						logging.WithError(err).Error("could not receive broadcast decide")
					}
				}(l, data)
			}
		} else {
			logging.Info("already decided")
		}
		return nil
	}

}

type MockEngineState struct {
	idmutex.Mutex
	KeygenDecision map[DKGID][]KeygenID
}

func SetupTestNodes(n int, k int, t int) (chan string, chan string, []*KeygenNode, []pcmn.Node, *map[NodeDetailsID]KeygenTransport, *MockEngineState) {
	// setup
	sharedCh := make(chan string)
	generatedCh := make(chan string)
	var currNodePrivKeys []big.Int
	var currNodePubKeys []common.Point
	for i := 0; i < n; i++ {
		currNodePrivKeys = append(currNodePrivKeys, *pvss.RandomBigInt())
		currNodePubKeys = append(currNodePubKeys, common.BigIntToPoint(secp256k1.Curve.ScalarBaseMult(currNodePrivKeys[i].Bytes())))
	}
	currNodeIndexes := pcmn.RandIndexes(1, 50, n)
	var currNodeList []pcmn.Node
	for i := 0; i < n; i++ {
		currNodeList = append(currNodeList, pcmn.Node{
			Index:  currNodeIndexes[i],
			PubKey: currNodePubKeys[i],
		})
	}
	localTransportNodeDirectory := make(map[NodeDetailsID]KeygenTransport)
	currEngineState := MockEngineState{
		KeygenDecision: make(map[DKGID][]KeygenID),
	}
	currRunEngine := MockEngine(&currEngineState, &localTransportNodeDirectory, k)
	var currNodes []*KeygenNode
	for i := 0; i < n; i++ {
		node := pcmn.Node{
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
			0,
		)
		newKeygenNode.CleanUp = noOpCleanUp
		currNodes = append(currNodes, newKeygenNode)
		_ = localTransport.SetKeygenNode(newKeygenNode)
		nodeDetails := NodeDetails(node)
		localTransportNodeDirectory[nodeDetails.ToNodeDetailsID()] = &localTransport
	}

	return sharedCh, generatedCh, currNodes, currNodeList, &localTransportNodeDirectory, &currEngineState
}

func SeedKeys(keys int, k int, currNodeList []pcmn.Node, currNodes []*KeygenNode) ([]map[int]big.Int, []DKGID) {
	var dkgIDs []DKGID
	var secretSets []map[int]big.Int
	for h := 0; h < keys; h++ {
		secrets := make(map[int]big.Int)
		dkgID := GenerateDKGID(*big.NewInt(int64(h)))
		dkgIDs = append(dkgIDs, dkgID)
		for _, node := range currNodes {
			index := node.NodeDetails.Index
			S := *pvss.RandomBigInt()
			Sprime := *pvss.RandomBigInt()
			secrets[index] = S
			node.ShareStore.Set(dkgID, &Sharing{
				DKGID:  dkgID,
				I:      index,
				S:      S,
				Sprime: Sprime,
			})
		}
		secretSets = append(secretSets, secrets)
	}
	return secretSets, dkgIDs
}
