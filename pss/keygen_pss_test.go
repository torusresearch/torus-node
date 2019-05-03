package pss

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

	"github.com/torusresearch/torus-public/idmutex"

	"github.com/torusresearch/bijson"
	"github.com/torusresearch/torus-public/logging"

	"github.com/stretchr/testify/assert"

	"github.com/torusresearch/torus-public/secp256k1"

	"github.com/torusresearch/torus-public/common"
	"github.com/torusresearch/torus-public/pvss"
)

func TestPSSOptimistic(test *testing.T) {
	logging.SetLevelString("error")
	runtime.GOMAXPROCS(10)
	keys := 3
	n := 9
	k := 5
	t := 2
	sharedCh, refreshedCh, nodes, nodeList, _, _, _, _, _, _, _ := SetupTestNodes(n, k, t, n, k, t, true)
	fmt.Println("Running TestKeygenSharing for " + strconv.Itoa(keys) + " keys, " + strconv.Itoa(n) + " nodes with reconstruction " + strconv.Itoa(k) + " and threshold " + strconv.Itoa(t))
	secrets, sharingIDs := SeedKeys(keys, k, nodeList, nodes)
	for _, sharingID := range sharingIDs {
		for _, node := range nodes {
			pssMsgShare := PSSMsgShare{
				SharingID: sharingID,
			}
			pssID := (&PSSIDDetails{
				SharingID:   sharingID,
				DealerIndex: node.NodeDetails.Index,
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
			assert.Fail(test, "did not get shared message")
		}
	}
	assert.Equal(test, completeMessages, n*n*keys)

	refreshedMessages := 0
	for refreshedMessages < n*keys {
		msg := <-refreshedCh
		if strings.Contains(msg, "refreshed") {
			refreshedMessages++
		} else {
			assert.Fail(test, "did not get refreshed message")
		}
	}
	assert.Equal(test, refreshedMessages, n*keys)

	for g, sharingID := range sharingIDs {
		var shares []common.PrimaryShare
		for _, node := range nodes {
			var subshares []common.PrimaryShare
			for _, noderef := range nodes { // assuming that all nodes are part of the valid set
				val := node.PSSStore[(&PSSIDDetails{
					SharingID:   sharingID,
					DealerIndex: noderef.NodeDetails.Index,
				}).ToPSSID()].Si
				if val.Cmp(big.NewInt(int64(0))) != 0 {
					subshares = append(subshares, common.PrimaryShare{
						Index: noderef.NodeDetails.Index,
						Value: val,
					})
				}
			}
			reconstructedSi := pvss.LagrangeScalar(subshares[0:k], 0)
			shares = append(shares, common.PrimaryShare{
				Index: node.NodeDetails.Index,
				Value: *reconstructedSi,
			})
		}
		reconstructedSecret := pvss.LagrangeScalar(shares[0:k], 0)
		assert.Equal(test, reconstructedSecret.Text(16), secrets[g].Text(16))
	}

	for i, sharingID := range sharingIDs {
		var pts []common.Point
		for _, node := range nodes {
			pts = append(pts, common.Point{
				X: *big.NewInt(int64(node.NodeDetails.Index)),
				Y: node.RecoverStore[sharingID].Si,
			})
		}
		assert.Equal(test, pvss.LagrangeScalarCP(pts, 0).Text(16), secrets[i].Text(16))
	}
}

func TestPSSDifferentThresholds(test *testing.T) {
	logging.SetLevelString("error")
	runtime.GOMAXPROCS(10)
	keys := 1
	nOld := 9
	kOld := 5
	tOld := 2
	nNew := 13
	kNew := 7
	tNew := 3
	sharedCh, refreshedCh, oldNodes, oldNodeList, newNodes, newNodeList, _, _, _, _, _ := SetupTestNodes(nOld, kOld, tOld, nNew, kNew, tNew, false)
	secrets, sharingIDs := SeedKeys(keys, kOld, oldNodeList, oldNodes)
	fmt.Println("Running TestKeygenSharing for " + strconv.Itoa(keys) + " keys, " + strconv.Itoa(nOld) + " nodes with reconstruction " + strconv.Itoa(kOld) + " and threshold " + strconv.Itoa(tOld))
	fmt.Println("redistributing to " + strconv.Itoa(nNew) + " nodes with reconstruction " + strconv.Itoa(kNew) + " and threshold " + strconv.Itoa(tNew))
	logging.Debug("OldNodes: " + fmt.Sprint(oldNodeList))
	logging.Debug("NewNodes: " + fmt.Sprint(newNodeList))
	for _, sharingID := range sharingIDs {
		for _, node := range oldNodes {
			pssMsgShare := PSSMsgShare{
				SharingID: sharingID,
			}
			pssID := (&PSSIDDetails{
				SharingID:   sharingID,
				DealerIndex: node.NodeDetails.Index,
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
	for completeMessages < nOld*nNew*keys {
		msg := <-sharedCh
		if strings.Contains(msg, "shared") {
			completeMessages++
		} else {
			assert.Fail(test, "did not get shared message")
		}
	}
	assert.Equal(test, completeMessages, nOld*nNew*keys)

	refreshedMessages := 0
	for refreshedMessages < nNew*keys {
		msg := <-refreshedCh
		if strings.Contains(msg, "refreshed") {
			refreshedMessages++
		} else {
			assert.Fail(test, "did not get refreshed message")
		}
	}
	assert.Equal(test, refreshedMessages, nNew*keys)

	for g, sharingID := range sharingIDs {
		var shares []common.PrimaryShare
		for _, node := range newNodes {
			var subshares []common.PrimaryShare
			for _, noderef := range oldNodes { // assuming that all nodes are part of the valid set
				val := node.PSSStore[(&PSSIDDetails{
					SharingID:   sharingID,
					DealerIndex: noderef.NodeDetails.Index,
				}).ToPSSID()].Si
				if val.Cmp(big.NewInt(int64(0))) != 0 {
					subshares = append(subshares, common.PrimaryShare{
						Index: noderef.NodeDetails.Index,
						Value: val,
					})
				} else {
					test.Fatal("Si is 0")
				}
			}
			reconstructedSi := pvss.LagrangeScalar(subshares[0:kOld], 0)
			shares = append(shares, common.PrimaryShare{
				Index: node.NodeDetails.Index,
				Value: *reconstructedSi,
			})
		}
		reconstructedSecret := pvss.LagrangeScalar(shares[0:kNew], 0)
		assert.Equal(test, reconstructedSecret.Text(16), secrets[g].Text(16))
	}

	for i, sharingID := range sharingIDs {
		var pts []common.Point
		for _, node := range newNodes {
			pts = append(pts, common.Point{
				X: *big.NewInt(int64(node.NodeDetails.Index)),
				Y: node.RecoverStore[sharingID].Si,
			})
		}
		assert.Equal(test, pvss.LagrangeScalarCP(pts, 0).Text(16), secrets[i].Text(16))
	}

}

func TestPSSLaggyNodes(test *testing.T) {
	logging.SetLevelString("error")
	runtime.GOMAXPROCS(10)
	keys := 5
	nOld := 9
	kOld := 5
	tOld := 2
	nNew := 9
	kNew := 5
	tNew := 2
	sharedCh, refreshedCh, oldNodes, oldNodeList, newNodes, newNodeList, localDirectory, oldNodePrivKeys, newNodePrivKeys, oldTMEngine, newTMEngine := SetupTestNodes(nOld, kOld, tOld, nNew, kNew, tNew, false)
	secrets, sharingIDs := SeedKeys(keys, kOld, oldNodeList, oldNodes)
	fmt.Println("Running TestKeygenSharing for " + strconv.Itoa(keys) + " keys, " + strconv.Itoa(nOld) + " nodes with reconstruction " + strconv.Itoa(kOld) + " and threshold " + strconv.Itoa(tOld))
	fmt.Println("redistributing to " + strconv.Itoa(nNew) + " nodes with reconstruction " + strconv.Itoa(kNew) + " and threshold " + strconv.Itoa(tNew))
	logging.Debug("OldNodes: " + fmt.Sprint(oldNodeList))
	logging.Debug("NewNodes: " + fmt.Sprint(newNodeList))

	// make offline nodes
	for i := 0; i < tOld; i++ {
		oldNode := oldNodes[i]
		fmt.Println("Laggy Old Node: ", string(oldNode.NodeDetails.ToNodeDetailsID())[0:8])
		localLaggyTransport := &LocalLaggyTransport{
			NodeDirectory:          localDirectory,
			PrivateKey:             &oldNodePrivKeys[i],
			OutputSharedChannel:    &sharedCh,
			OutputRefreshedChannel: &refreshedCh,
			MockTMEngine:           oldTMEngine,
		}
		oldNode.Transport = localLaggyTransport
		localLaggyTransport.SetPSSNode(oldNode)
		(*localDirectory)[oldNode.NodeDetails.ToNodeDetailsID()] = localLaggyTransport
	}
	for i := 0; i < tNew; i++ {
		newNode := newNodes[i]
		fmt.Println("Laggy New Node: ", string(newNode.NodeDetails.ToNodeDetailsID())[0:8])
		localLaggyTransport := &LocalLaggyTransport{
			NodeDirectory:          localDirectory,
			PrivateKey:             &newNodePrivKeys[i],
			OutputSharedChannel:    &sharedCh,
			OutputRefreshedChannel: &refreshedCh,
			MockTMEngine:           newTMEngine,
		}

		newNode.Transport = localLaggyTransport
		localLaggyTransport.SetPSSNode(newNode)
		(*localDirectory)[newNode.NodeDetails.ToNodeDetailsID()] = localLaggyTransport
	}

	for _, sharingID := range sharingIDs {
		for _, node := range oldNodes {
			pssMsgShare := PSSMsgShare{
				SharingID: sharingID,
			}
			pssID := (&PSSIDDetails{
				SharingID:   sharingID,
				DealerIndex: node.NodeDetails.Index,
			}).ToPSSID()
			data, err := bijson.Marshal(pssMsgShare)
			if err != nil {
				test.Fatal(err)
			}
			// fmt.Println(node.Transport, node.NodeDetails, node.Transport)
			err = node.Transport.Send(node.NodeDetails, PSSMessage{
				PSSID:  pssID,
				Method: "share",
				Data:   data,
			})
			assert.NoError(test, err)
		}
	}
	cont := make(chan bool)
	completeMessages := 0
	go func() {
		for completeMessages < (tOld+kOld)*(tNew+kNew)*keys {
			msg := <-sharedCh
			if strings.Contains(msg, "shared") {
				logging.Debug("Received shared message")
				completeMessages++
				logging.Debug("Total completeMessages:" + strconv.Itoa(completeMessages))
			} else {
				assert.Fail(test, "did not get shared message")
			}
		}
		cont <- true
		for {
			<-sharedCh
			completeMessages++
		}
	}()
	<-cont

	// assert.Equal(test, completeMessages, (tOld+kOld)*(tNew+kNew)*keys)
	// logging.Debug("All completed messages received")

	refreshedMessages := 0
	go func(cont chan bool) {
		for refreshedMessages < (tNew+kNew)*keys {
			msg := <-refreshedCh
			if strings.Contains(msg, "refreshed") {
				logging.Debug("Received refreshed message")
				refreshedMessages++
				logging.Debug("Total refreshMessages:" + strconv.Itoa(refreshedMessages))
			} else {
				assert.Fail(test, "did not get refreshed message")
			}
		}
		cont <- true
		for {
			<-refreshedCh
			refreshedMessages++
		}
	}(cont)
	<-cont
	// assert.Equal(test, refreshedMessages, (tNew+kNew)*keys)
	// logging.Debug("All refreshed messages received")
here:
	fmt.Println("RERUN", refreshedMessages, completeMessages)
	for g, sharingID := range sharingIDs {
		var shares []common.PrimaryShare
		for _, node := range newNodes[2:] {
			var subshares []common.PrimaryShare
			for _, noderef := range oldNodes[2:] { // assuming that all nodes are part of the valid set
				val := node.PSSStore[(&PSSIDDetails{
					SharingID:   sharingID,
					DealerIndex: noderef.NodeDetails.Index,
				}).ToPSSID()].Si
				if val.Cmp(big.NewInt(int64(0))) != 0 {
					subshares = append(subshares, common.PrimaryShare{
						Index: noderef.NodeDetails.Index,
						Value: val,
					})
				} else {
					time.Sleep(50 * time.Millisecond)
					// test.Fatal("Si is 0")
					goto here
				}
			}
			reconstructedSi := pvss.LagrangeScalar(subshares[0:kOld], 0)
			shares = append(shares, common.PrimaryShare{
				Index: node.NodeDetails.Index,
				Value: *reconstructedSi,
			})
		}
		reconstructedSecret := pvss.LagrangeScalar(shares[0:kNew], 0)
		assert.Equal(test, reconstructedSecret.Text(16), secrets[g].Text(16))
	}

	for i, sharingID := range sharingIDs {
		var pts []common.Point
		for _, node := range newNodes[2:] {
			pts = append(pts, common.Point{
				X: *big.NewInt(int64(node.NodeDetails.Index)),
				Y: node.RecoverStore[sharingID].Si,
			})
		}

		// retry a few times
		fails := 0
		for fails < 3 {
			if pvss.LagrangeScalarCP(pts, 0).Text(16) == secrets[i].Text(16) {
				break
			} else {
				fails++
				time.Sleep(1 * time.Second)
			}
		}
		if fails == 2 {
			assert.Fail(test, "failed thrice")
		}
	}
}

func TestPSSOfflineNodes(test *testing.T) {
	logging.SetLevelString("error")
	runtime.GOMAXPROCS(10)
	keys := 1
	nOld := 9
	kOld := 5
	tOld := 2
	nNew := 9
	kNew := 5
	tNew := 2
	sharedCh, refreshedCh, oldNodes, oldNodeList, newNodes, newNodeList, localDirectory, _, _, _, _ := SetupTestNodes(nOld, kOld, tOld, nNew, kNew, tNew, false)
	secrets, sharingIDs := SeedKeys(keys, kOld, oldNodeList, oldNodes)
	fmt.Println("Running TestKeygenSharing for " + strconv.Itoa(keys) + " keys, " + strconv.Itoa(nOld) + " nodes with reconstruction " + strconv.Itoa(kOld) + " and threshold " + strconv.Itoa(tOld))
	fmt.Println("redistributing to " + strconv.Itoa(nNew) + " nodes with reconstruction " + strconv.Itoa(kNew) + " and threshold " + strconv.Itoa(tNew))
	logging.Debug("OldNodes: " + fmt.Sprint(oldNodeList))
	logging.Debug("NewNodes: " + fmt.Sprint(newNodeList))

	// make offline nodes
	for i := 0; i < tOld; i++ {
		oldNode := oldNodes[i]
		fmt.Println("Offline Old Node: ", string(oldNode.NodeDetails.ToNodeDetailsID())[0:8])
		localOfflineTransport := &LocalOfflineTransport{
			OutputSharedChannel:    &sharedCh,
			OutputRefreshedChannel: &refreshedCh,
		}
		oldNode.Transport = localOfflineTransport
		(*localDirectory)[oldNode.NodeDetails.ToNodeDetailsID()] = localOfflineTransport
	}
	for i := 0; i < tNew; i++ {
		newNode := newNodes[i]
		fmt.Println("Offline New Node: ", string(newNode.NodeDetails.ToNodeDetailsID())[0:8])
		localOfflineTransport := &LocalOfflineTransport{
			OutputSharedChannel:    &sharedCh,
			OutputRefreshedChannel: &refreshedCh,
		}
		newNode.Transport = localOfflineTransport
		(*localDirectory)[newNode.NodeDetails.ToNodeDetailsID()] = localOfflineTransport
	}

	for _, sharingID := range sharingIDs {
		for _, node := range oldNodes {
			pssMsgShare := PSSMsgShare{
				SharingID: sharingID,
			}
			pssID := (&PSSIDDetails{
				SharingID:   sharingID,
				DealerIndex: node.NodeDetails.Index,
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
	for completeMessages < (tOld+kOld)*(tNew+kNew)*keys {
		msg := <-sharedCh
		if strings.Contains(msg, "shared") {
			logging.Debug("Received shared message")
			completeMessages++
			logging.Debug("Total completeMessages:" + strconv.Itoa(completeMessages))
		} else {
			assert.Fail(test, "did not get shared message")
		}
	}
	assert.Equal(test, completeMessages, (tOld+kOld)*(tNew+kNew)*keys)
	logging.Debug("All completed messages received")

	refreshedMessages := 0
	for refreshedMessages < (tNew+kNew)*keys {
		msg := <-refreshedCh
		if strings.Contains(msg, "refreshed") {
			logging.Debug("Received refreshed message")
			refreshedMessages++
			logging.Debug("Total refreshMessages:" + strconv.Itoa(refreshedMessages))
		} else {
			assert.Fail(test, "did not get refreshed message")
		}
	}
	assert.Equal(test, refreshedMessages, (tNew+kNew)*keys)
	logging.Debug("All refreshed messages received")

	for g, sharingID := range sharingIDs {
		var shares []common.PrimaryShare
		for _, node := range newNodes[2:] {
			var subshares []common.PrimaryShare
			for _, noderef := range oldNodes[2:] { // assuming that all nodes are part of the valid set
				val := node.PSSStore[(&PSSIDDetails{
					SharingID:   sharingID,
					DealerIndex: noderef.NodeDetails.Index,
				}).ToPSSID()].Si
				if val.Cmp(big.NewInt(int64(0))) != 0 {
					subshares = append(subshares, common.PrimaryShare{
						Index: noderef.NodeDetails.Index,
						Value: val,
					})
				} else {
					test.Fatal("Si is 0")
				}
			}
			reconstructedSi := pvss.LagrangeScalar(subshares[0:kOld], 0)
			shares = append(shares, common.PrimaryShare{
				Index: node.NodeDetails.Index,
				Value: *reconstructedSi,
			})
		}
		reconstructedSecret := pvss.LagrangeScalar(shares[0:kNew], 0)
		assert.Equal(test, reconstructedSecret.Text(16), secrets[g].Text(16))
	}

	for i, sharingID := range sharingIDs {
		var pts []common.Point
		for _, node := range newNodes[2:] {
			fmt.Println(33, node)
			fmt.Println(44, node.RecoverStore)
			fmt.Println(55, node.RecoverStore[sharingID])
			pts = append(pts, common.Point{
				X: *big.NewInt(int64(node.NodeDetails.Index)),
				Y: node.RecoverStore[sharingID].Si,
			})
		}
		assert.Equal(test, pvss.LagrangeScalarCP(pts, 0).Text(16), secrets[i].Text(16))
	}
}

var LocalNodeDirectory map[string]*LocalTransport

type Middleware func(PSSMessage) (modifiedMessage PSSMessage, end bool, err error)

type LocalOfflineTransport struct {
	OutputSharedChannel    *chan string
	OutputRefreshedChannel *chan string
}

func (l *LocalOfflineTransport) GetType() string {
	return "offline"
}

func (l *LocalOfflineTransport) SetPSSNode(ref *PSSNode) error {
	return nil
}

func (l *LocalOfflineTransport) SetTMEngine(ref *func(NodeDetails, PSSMessage) error) error {
	return nil
}

func (l *LocalOfflineTransport) Send(nodeDetails NodeDetails, pssMessage PSSMessage) error {
	return nil
}

func (l *LocalOfflineTransport) Receive(senderDetails NodeDetails, pssMessage PSSMessage) error {
	return nil
}

func (l *LocalOfflineTransport) SendBroadcast(pssMessage PSSMessage) error {
	return nil
}

func (l *LocalOfflineTransport) ReceiveBroadcast(pssMessage PSSMessage) error {
	return nil
}

func (l *LocalOfflineTransport) Output(s string) {
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

func (l *LocalOfflineTransport) Sign(s string) ([]byte, error) {
	return []byte{}, nil
}

type LocalLaggyTransport struct {
	PSSNode                *PSSNode
	PrivateKey             *big.Int
	NodeDirectory          *map[NodeDetailsID]PSSTransport
	OutputSharedChannel    *chan string
	OutputRefreshedChannel *chan string
	SendMiddleware         []Middleware
	ReceiveMiddleware      []Middleware
	MockTMEngine           *func(NodeDetails, PSSMessage) error
}

func (l *LocalLaggyTransport) GetType() string {
	return "laggy"
}

func (l *LocalLaggyTransport) SetPSSNode(ref *PSSNode) error {
	l.PSSNode = ref
	return nil
}

func (l *LocalLaggyTransport) SetTMEngine(ref *func(NodeDetails, PSSMessage) error) error {
	l.MockTMEngine = ref
	return nil
}

func (l *LocalLaggyTransport) Send(nodeDetails NodeDetails, pssMessage PSSMessage) error {
	modifiedMessage, err := l.runSendMiddleware(pssMessage)
	if err != nil {
		return err
	}
	// time.Sleep(10 * time.Second)
	return (*l.NodeDirectory)[nodeDetails.ToNodeDetailsID()].Receive(l.PSSNode.NodeDetails, modifiedMessage)
}

func (l *LocalLaggyTransport) Receive(senderDetails NodeDetails, pssMessage PSSMessage) error {
	go func() {
		time.Sleep(50 * time.Millisecond)
		modifiedMessage, _ := l.runReceiveMiddleware(pssMessage)
		l.PSSNode.ProcessMessage(senderDetails, modifiedMessage)
	}()
	return nil
}

func (l *LocalLaggyTransport) SendBroadcast(pssMessage PSSMessage) error {
	go func() {
		time.Sleep(50 * time.Millisecond)
		(*l.MockTMEngine)(l.PSSNode.NodeDetails, pssMessage)
	}()
	return nil
}

func (l *LocalLaggyTransport) ReceiveBroadcast(pssMessage PSSMessage) error {
	go func() {
		time.Sleep(50 * time.Millisecond)
		l.PSSNode.ProcessBroadcastMessage(pssMessage)
	}()
	return nil
}

func (l *LocalLaggyTransport) Output(s string) {
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

func (l *LocalLaggyTransport) Sign(s string) ([]byte, error) {
	return pvss.ECDSASign(s, l.PrivateKey), nil
}

func (l *LocalLaggyTransport) runSendMiddleware(pssMessage PSSMessage) (PSSMessage, error) {
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

func (l *LocalLaggyTransport) runReceiveMiddleware(pssMessage PSSMessage) (PSSMessage, error) {
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

type LocalTransport struct {
	PSSNode                *PSSNode
	PrivateKey             *big.Int
	NodeDirectory          *map[NodeDetailsID]PSSTransport
	OutputSharedChannel    *chan string
	OutputRefreshedChannel *chan string
	SendMiddleware         []Middleware
	ReceiveMiddleware      []Middleware
	MockTMEngine           *func(NodeDetails, PSSMessage) error
}

func (l *LocalTransport) GetType() string {
	return "local"
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

func MockEngine(engineState *MockEngineState, localTransportNodeDirectory *map[NodeDetailsID]PSSTransport, kOld int) func(NodeDetails, PSSMessage) error {

	return func(senderDetails NodeDetails, pssMessage PSSMessage) error {
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

		if len(pssMsgPropose.PSSs) < kOld {
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
			if len(*localTransportNodeDirectory) == 0 {
				return errors.New("localTransportNodeDirectory is empty, it may not have been initialized")
			}
			for _, l := range *localTransportNodeDirectory {
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
				go func(l PSSTransport, data []byte) {
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

}

type MockEngineState struct {
	idmutex.Mutex
	PSSDecision map[SharingID]bool
}

func SetupTestNodes(nOld int, kOld int, tOld int, nNew int, kNew int, tNew int, sameNodes bool) (chan string, chan string, []*PSSNode, []common.Node, []*PSSNode, []common.Node, *map[NodeDetailsID]PSSTransport, []big.Int, []big.Int, *func(NodeDetails, PSSMessage) error, *func(NodeDetails, PSSMessage) error) {
	// setup
	sharedCh := make(chan string)
	refreshedCh := make(chan string)
	var oldNodePrivKeys []big.Int
	var newNodePrivKeys []big.Int
	var oldNodePubKeys []common.Point
	var newNodePubKeys []common.Point
	for i := 0; i < nOld; i++ {
		oldNodePrivKeys = append(oldNodePrivKeys, *pvss.RandomBigInt())
		oldNodePubKeys = append(oldNodePubKeys, common.BigIntToPoint(secp256k1.Curve.ScalarBaseMult(oldNodePrivKeys[i].Bytes())))
	}
	oldNodeIndexes := randIndexes(1, 50, nOld)
	for i := 0; i < nNew; i++ {
		newNodePrivKeys = append(newNodePrivKeys, *pvss.RandomBigInt())
		newNodePubKeys = append(newNodePubKeys, common.BigIntToPoint(secp256k1.Curve.ScalarBaseMult(newNodePrivKeys[i].Bytes())))
	}
	newNodeIndexes := randIndexes(51, 100, nNew)
	var oldNodeList []common.Node
	for i := 0; i < nOld; i++ {
		oldNodeList = append(oldNodeList, common.Node{
			Index:  oldNodeIndexes[i],
			PubKey: oldNodePubKeys[i],
		})
	}
	var newNodeList []common.Node
	for i := 0; i < nNew; i++ {
		newNodeList = append(newNodeList, common.Node{
			Index:  newNodeIndexes[i],
			PubKey: newNodePubKeys[i],
		})
	}
	localTransportNodeDirectory := make(map[NodeDetailsID]PSSTransport)
	oldEngineState := MockEngineState{
		PSSDecision: make(map[SharingID]bool),
	}
	oldRunEngine := MockEngine(&oldEngineState, &localTransportNodeDirectory, kOld)
	var oldNodes []*PSSNode
	for i := 0; i < nOld; i++ {
		node := common.Node{
			Index:  oldNodeIndexes[i],
			PubKey: oldNodePubKeys[i],
		}
		localTransport := LocalTransport{
			NodeDirectory:          &localTransportNodeDirectory,
			PrivateKey:             &oldNodePrivKeys[i],
			OutputSharedChannel:    &sharedCh,
			OutputRefreshedChannel: &refreshedCh,
			MockTMEngine:           &oldRunEngine,
		}
		newNodeL := newNodeList
		tN := tNew
		kN := kNew
		newNodeI := newNodeIndexes
		isP := false
		if sameNodes == true {
			newNodeL = oldNodeList
			tN = tOld
			kN = kOld
			newNodeI = oldNodeIndexes
			isP = true
		}
		newPssNode := NewPSSNode(
			node,
			oldNodeList,
			tOld,
			kOld,
			newNodeL,
			tN,
			kN,
			newNodeI[i],
			&localTransport,
			true,
			isP,
		)
		oldNodes = append(oldNodes, newPssNode)
		localTransport.SetPSSNode(newPssNode)
		nodeDetails := NodeDetails(node)
		localTransportNodeDirectory[nodeDetails.ToNodeDetailsID()] = &localTransport
	}

	newEngineState := MockEngineState{
		PSSDecision: make(map[SharingID]bool),
	}
	newRunEngine := MockEngine(&newEngineState, &localTransportNodeDirectory, kNew)
	var newNodes []*PSSNode
	if sameNodes != true {
		for i := 0; i < nNew; i++ {
			node := common.Node{
				Index:  newNodeIndexes[i],
				PubKey: newNodePubKeys[i],
			}
			localTransport := LocalTransport{
				NodeDirectory:          &localTransportNodeDirectory,
				PrivateKey:             &newNodePrivKeys[i],
				OutputSharedChannel:    &sharedCh,
				OutputRefreshedChannel: &refreshedCh,
				MockTMEngine:           &newRunEngine,
			}
			newPssNode := NewPSSNode(
				node,
				oldNodeList,
				tOld,
				kOld,
				newNodeList,
				tNew,
				kNew,
				newNodeIndexes[i],
				&localTransport,
				false,
				true,
			)
			newNodes = append(newNodes, newPssNode)
			localTransport.SetPSSNode(newPssNode)
			nodeDetails := NodeDetails(node)
			localTransportNodeDirectory[nodeDetails.ToNodeDetailsID()] = &localTransport
		}
	}

	return sharedCh, refreshedCh, oldNodes, oldNodeList, newNodes, newNodeList, &localTransportNodeDirectory, oldNodePrivKeys, newNodePrivKeys, &oldRunEngine, &newRunEngine
}

func SeedKeys(keys int, kOld int, oldNodeList []common.Node, oldNodes []*PSSNode) ([]big.Int, []SharingID) {
	var secrets []big.Int
	var sharingIDs []SharingID
	for h := 0; h < keys; h++ {
		secret := pvss.RandomBigInt()
		secrets = append(secrets, *secret)
		mask := pvss.RandomBigInt()
		randPoly := pvss.RandomPoly(*secret, kOld)
		randPolyprime := pvss.RandomPoly(*mask, kOld)
		commit := pvss.GetCommit(*randPoly)
		commitH := pvss.GetCommitH(*randPolyprime)
		sumCommitments := pvss.AddCommitments(commit, commitH)
		sharingID := SharingID("SHARING" + strconv.Itoa(h))
		sharingIDs = append(sharingIDs, sharingID)
		for _, node := range oldNodes {
			index := node.NodeDetails.Index
			Si := *pvss.PolyEval(*randPoly, *big.NewInt(int64(index)))
			Siprime := *pvss.PolyEval(*randPolyprime, *big.NewInt(int64(index)))
			node.ShareStore[sharingID] = &Sharing{
				SharingID: sharingID,
				Nodes:     oldNodeList,
				Epoch:     1,
				I:         index,
				Si:        Si,
				Siprime:   Siprime,
				C:         sumCommitments,
			}
		}
	}
	return secrets, sharingIDs
}
