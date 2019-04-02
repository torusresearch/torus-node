package keygen

import (
	"fmt"
	"log"
	"math/big"
	"os"
	"runtime"
	"runtime/pprof"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/torusresearch/torus-public/common"
	"github.com/torusresearch/torus-public/logging"
)

func TestOptimisticKeygen(t *testing.T) {

	f, err := os.Create("profile_optimistic")
	if err != nil {
		log.Fatal(err)
	}
	pprof.StartCPUProfile(f)
	defer pprof.StopCPUProfile()

	runtime.GOMAXPROCS(10)
	logging.SetLevelString("debug")
	comsChannel := make(chan string)
	numOfNodes := 5
	threshold := 4
	nodeList := make([]big.Int, numOfNodes)
	nodeKegenInstances := make(map[string]*KeygenInstance)
	for i := range nodeList {
		nodeList[i] = *big.NewInt(int64(i + 1))
		nodeKegenInstances[nodeList[i].Text(16)] = &KeygenInstance{}
	}

	//edit transport functions
	for k, v := range nodeKegenInstances {
		var nodeIndex big.Int
		nodeIndex.SetString(k, 16)
		transport := mockTransport{nodeIndex: nodeIndex, nodeKegenInstances: &nodeKegenInstances}
		v.Transport = &transport
		//set up store
		v.Store = &mockKeygenStore{}
	}

	//start!
	for _, nodeIndex := range nodeList {
		t.Log("Initiating Nodes. Index: ", nodeIndex.Text(16))
		go func(nIndex big.Int) {
			err := nodeKegenInstances[nIndex.Text(16)].InitiateKeygen(*big.NewInt(int64(0)), 10, nodeList, threshold, nIndex, comsChannel)
			defer func() {
				if err != nil {
					t.Logf("Initiate Keygen error: %s", err)
				}
			}()
		}(nodeIndex)
	}

	// wait till all nodes are done
	count := 0
	for {
		select {
		case msg := <-comsChannel:
			if msg == SIKeygenCompleted {
				count++
			}
		}
		if count >= len(nodeList) {
			break
		}
	}

	for _, nodeIndex := range nodeList {
		instance := nodeKegenInstances[nodeIndex.Text(16)]
		instance.Lock()
		t.Log(nodeIndex.Text(16), instance.State.Current())
		for _, ni := range nodeList {
			t.Log("KeyLogState from ", ni.Text(16), instance.KeyLog[big.NewInt(int64(0)).Text(16)][ni.Text(16)].SubshareState.Current())
			if instance.KeyLog[big.NewInt(int64(0)).Text(16)][ni.Text(16)].SubshareState.Current() != "perfect_subshare" {
				nodeLog := instance.KeyLog[big.NewInt(int64(0)).Text(16)][ni.Text(16)]
				t.Log("Number of Echos: ", len(nodeLog.ReceivedEchoes))
				t.Log("Number of Readys: ", len(nodeLog.ReceivedReadys))
				// 	t.Log("Ready:")
				// 	for _, ready := range nodeLog.ReceivedReadys {
				// 		t.Log(ready)
				// 	}
			}
		}
		instance.Unlock()
	}
	assert.True(t, count == len(nodeList))
}

func TestTimeboundOne(t *testing.T) {

	f, err := os.Create("profile_timebound_one")
	if err != nil {
		log.Fatal(err)
	}
	pprof.StartCPUProfile(f)
	defer pprof.StopCPUProfile()

	runtime.GOMAXPROCS(10)
	logging.SetLevelString("debug")
	comsChannel := make(chan string)
	numOfNodes := 5
	threshold := 4
	nodeList := make([]big.Int, numOfNodes)
	nodeKegenInstances := make(map[string]*KeygenInstance)
	for i := range nodeList {
		nodeList[i] = *big.NewInt(int64(i + 1))
		nodeKegenInstances[nodeList[i].Text(16)] = &KeygenInstance{}
	}

	done := false
	// build timer function to kill of exceeds time
	go func(d *bool) {
		time.Sleep(3 * time.Second)
		if !*d {
			assert.True(t, *d, "TestTimeboundOne timed out")
		}
	}(&done)

	//edit transport functions
	for k, v := range nodeKegenInstances {
		var nodeIndex big.Int
		nodeIndex.SetString(k, 16)
		transport := mockTransport{nodeIndex: nodeIndex, nodeKegenInstances: &nodeKegenInstances}
		if nodeIndex.Cmp(big.NewInt(int64(1))) == 0 {
			v.Transport = &mockDeadTransport{}
		} else {
			v.Transport = &transport
		}

		//set up store
		v.Store = &mockKeygenStore{}
	}

	//start!
	for _, nodeIndex := range nodeList {
		// // dont start first node (to malicious node)
		// if i == 0 {
		// 	continue
		// }
		t.Log("Initiating Nodes. Index: ", nodeIndex.Text(16))
		go func(nIndex big.Int) {
			err := nodeKegenInstances[nIndex.Text(16)].InitiateKeygen(*big.NewInt(int64(0)), 1, nodeList, threshold, nIndex, comsChannel)
			defer func() {
				if err != nil {
					t.Logf("Initiate Keygen error: %s", err)
				}
			}()
		}(nodeIndex)
	}

	time.Sleep(1 * time.Second)

	// log node status
	for i, nodeIndex := range nodeList {
		// to not cause a panic
		if i == 0 {
			continue
		}
		instance := nodeKegenInstances[nodeIndex.Text(16)]
		instance.Lock()
		t.Log(nodeIndex.Text(16), instance.State.Current())
		// for _, ni := range nodeList {
		// 	t.Log("KeyLogState from ", ni.Text(16), instance.KeyLog[big.NewInt(int64(0)).Text(16)][ni.Text(16)].SubshareState.Current())
		// }
		instance.Unlock()
	}

	// trigger timebound one here
	for i, nodeIndex := range nodeList {
		// to not cause a panic
		if i == 0 {
			continue
		}
		instance := nodeKegenInstances[nodeIndex.Text(16)]
		err := instance.TriggerRoundOneTimebound()
		if err != nil {
			t.Log(err)
		}
	}

	// wait till nodes are done (w/o malicious node)
	count := 0
	for {
		select {
		case msg := <-comsChannel:
			if msg == SIKeygenCompleted {
				count++
			}
		}
		if count >= len(nodeList)-1 { // accounted for here
			break
		}
	}

	// log node status
	for i, nodeIndex := range nodeList {
		// to not cause a panic
		if i == 0 {
			continue
		}
		instance := nodeKegenInstances[nodeIndex.Text(16)]
		instance.Lock()
		t.Log(nodeIndex.Text(16), instance.State.Current())
		assert.True(t, instance.State.Current() == SIKeygenCompleted, "Keygen not completed in TimeboundOne")
		instance.Unlock()
	}
}

func TestTimeboundTwo(t *testing.T) {

	f, err := os.Create("profile_timebound_two")
	if err != nil {
		log.Fatal(err)
	}
	pprof.StartCPUProfile(f)
	defer pprof.StopCPUProfile()

	runtime.GOMAXPROCS(10)
	logging.SetLevelString("debug")
	comsChannel := make(chan string)
	numOfNodes := 5
	threshold := 4
	nodeList := make([]big.Int, numOfNodes)
	nodeKegenInstances := make(map[string]*KeygenInstance)
	for i := range nodeList {
		nodeList[i] = *big.NewInt(int64(i + 1))
		nodeKegenInstances[nodeList[i].Text(16)] = &KeygenInstance{}
	}

	done := false
	// build timer function to kill of exceeds time
	go func(d *bool) {
		time.Sleep(3 * time.Second)
		if !*d {
			assert.True(t, *d, "TestTimeboundOne timed out")
		}
	}(&done)

	//edit transport functions
	for k, v := range nodeKegenInstances {
		var nodeIndex big.Int
		nodeIndex.SetString(k, 16)
		transport := mockTransport{nodeIndex: nodeIndex, nodeKegenInstances: &nodeKegenInstances}
		if nodeIndex.Cmp(big.NewInt(int64(1))) == 0 {
			v.Transport = &mockDeadTransportTwo{nodeIndex: nodeIndex, nodeKegenInstances: &nodeKegenInstances}
		} else {
			v.Transport = &transport
		}

		//set up store
		v.Store = &mockKeygenStore{}
	}

	//start!
	for _, nodeIndex := range nodeList {
		t.Log("Initiating Nodes. Index: ", nodeIndex.Text(16))
		go func(nIndex big.Int) {
			err := nodeKegenInstances[nIndex.Text(16)].InitiateKeygen(*big.NewInt(int64(0)), 1, nodeList, threshold, nIndex, comsChannel)
			defer func() {
				if err != nil {
					t.Logf("Initiate Keygen error: %s", err)
				}
			}()
		}(nodeIndex)
	}

	time.Sleep(2 * time.Second)

	// log node status
	for i, nodeIndex := range nodeList {
		// to not cause a panic
		if i == 0 {
			continue
		}
		instance := nodeKegenInstances[nodeIndex.Text(16)]
		instance.Lock()
		t.Log(nodeIndex.Text(16), instance.State.Current())
		// for _, ni := range nodeList {
		// 	t.Log("KeyLogState from ", ni.Text(16), instance.KeyLog[big.NewInt(int64(0)).Text(16)][ni.Text(16)].SubshareState.Current())
		// }
		instance.Unlock()
	}

	// trigger timebound two here
	for i, nodeIndex := range nodeList {
		// to not cause a panic
		if i == 0 {
			continue
		}
		instance := nodeKegenInstances[nodeIndex.Text(16)]
		err := instance.TriggerRoundTwoTimebound()
		if err != nil {
			t.Log(err)
		}
	}

	// // wait till nodes are done (w/o malicious node)
	count := 0
	for {
		select {
		case msg := <-comsChannel:
			if msg == SIKeygenCompleted {
				count++
			}
		}
		if count >= len(nodeList)-1 { // accounted for here
			break
		}
	}

	// // log node status
	for i, nodeIndex := range nodeList {
		// to not cause a panic
		if i == 0 {
			continue
		}
		instance := nodeKegenInstances[nodeIndex.Text(16)]
		instance.Lock()
		t.Log(nodeIndex.Text(16), instance.State.Current())
		assert.True(t, instance.State.Current() == SIKeygenCompleted, "Keygen not completed in TimeboundTwo")
		instance.Unlock()
	}
}

type mockTransport struct {
	nodeIndex          big.Int
	nodeKegenInstances *map[string]*KeygenInstance
}

func (transport *mockTransport) SendKEYGENSend(msg KEYGENSend, to big.Int) error {
	// fmt.Println("SendKEYGENSend Called: ", to)
	go func(ins map[string]*KeygenInstance) {
		err := ins[to.Text(16)].OnKEYGENSend(msg, transport.nodeIndex)
		if err != nil {
			fmt.Println("ERRROR SendKEYGENSend: ", err)
		}
	}((*transport.nodeKegenInstances))
	return nil
}

func (transport *mockTransport) SendKEYGENEcho(msg KEYGENEcho, to big.Int) error {
	// fmt.Println("SendKEYGENEcho Called: ", to)
	go func(ins map[string]*KeygenInstance) {
		err := ins[to.Text(16)].OnKEYGENEcho(msg, transport.nodeIndex)
		if err != nil {
			fmt.Println("ERRROR OnKEYGENEcho: ", err)
		}
	}((*transport.nodeKegenInstances))
	return nil
}

func (transport *mockTransport) SendKEYGENReady(msg KEYGENReady, to big.Int) error {
	// fmt.Println("SendKEYGENReady: ", to)
	go func(ins map[string]*KeygenInstance) {
		err := ins[to.Text(16)].OnKEYGENReady(msg, transport.nodeIndex)
		if err != nil {
			fmt.Println("ERRROR OnKEYGENReady: ", err)
		}
	}((*transport.nodeKegenInstances))
	return nil
}

func (transport *mockTransport) BroadcastInitiateKeygen(commitmentMatrixes [][][]common.Point) error {
	// logging.Debugf("Broadcast Initiate Keygen Called: %s", transport.nodeIndex)
	for _, instance := range *transport.nodeKegenInstances {
		// logging.Debugf("index: %s", k)
		go func(ins *KeygenInstance, cm [][][]common.Point, tns big.Int) {
			err := ins.OnInitiateKeygen(cm, tns)
			if err != nil {
				fmt.Println("ERRROR BroadcastInitiateKeygen: ", err)
			}
		}(instance, commitmentMatrixes, transport.nodeIndex)
	}
	return nil
}

func (transport *mockTransport) BroadcastKEYGENShareComplete(keygenShareCompletes []KEYGENShareComplete) error {
	// fmt.Println("BroadcastKEYGENShareComplete Called: ", transport.nodeIndex)
	for _, instance := range *transport.nodeKegenInstances {
		go func(ins *KeygenInstance, cm []KEYGENShareComplete, tns big.Int) {
			err := ins.OnKEYGENShareComplete(cm, tns)
			if err != nil {
				fmt.Println("ERRROR BroadcastKEYGENShareComplete: ", err)
			}
		}(instance, keygenShareCompletes, transport.nodeIndex)
	}
	return nil
}

type mockDeadTransport struct {
}

func (transport *mockDeadTransport) SendKEYGENSend(msg KEYGENSend, to big.Int) error {
	return nil
}

func (transport *mockDeadTransport) SendKEYGENEcho(msg KEYGENEcho, to big.Int) error {
	return nil
}

func (transport *mockDeadTransport) SendKEYGENReady(msg KEYGENReady, to big.Int) error {
	return nil
}

func (transport *mockDeadTransport) BroadcastInitiateKeygen(commitmentMatrixes [][][]common.Point) error {
	return nil
}

func (transport *mockDeadTransport) BroadcastKEYGENShareComplete(keygenShareCompletes []KEYGENShareComplete) error {
	return nil
}

type mockDeadTransportTwo struct {
	nodeIndex          big.Int
	nodeKegenInstances *map[string]*KeygenInstance
}

func (transport *mockDeadTransportTwo) SendKEYGENSend(msg KEYGENSend, to big.Int) error {
	return nil
}

func (transport *mockDeadTransportTwo) SendKEYGENEcho(msg KEYGENEcho, to big.Int) error {
	return nil
}

func (transport *mockDeadTransportTwo) SendKEYGENReady(msg KEYGENReady, to big.Int) error {
	return nil
}

func (transport *mockDeadTransportTwo) BroadcastInitiateKeygen(commitmentMatrixes [][][]common.Point) error {
	for _, instance := range *transport.nodeKegenInstances {
		// logging.Debugf("index: %s", k)
		go func(ins *KeygenInstance, cm [][][]common.Point, tns big.Int) {
			err := ins.OnInitiateKeygen(cm, tns)
			if err != nil {
				fmt.Println("ERRROR BroadcastInitiateKeygen: ", err)
			}
		}(instance, commitmentMatrixes, transport.nodeIndex)
	}
	return nil
}

func (transport *mockDeadTransportTwo) BroadcastKEYGENShareComplete(keygenShareCompletes []KEYGENShareComplete) error {
	return nil
}

type mockKeygenStore struct {
}

func (ks *mockKeygenStore) StoreKEYGENSecret(keyIndex big.Int, secret KEYGENSecrets) error {
	return nil
}
func (ks *mockKeygenStore) StoreCompletedShare(keyIndex big.Int, si big.Int, siprime big.Int) error {
	return nil
}

type mockDeadNode struct {
}

func (mn *mockDeadNode) InitiateKeygen(startingIndex big.Int, numOfKeys int, nodeIndexes []big.Int, threshold int, nodeIndex big.Int, comChannel chan string) error {
	return nil
}

func (mn *mockDeadNode) OnInitiateKeygen(commitmentMatrixes [][][]common.Point, nodeIndex big.Int) error {
	return nil
}
func (mn *mockDeadNode) OnKEYGENSend(msg KEYGENSend, fromNodeIndex big.Int) error {
	return nil
}
func (mn *mockDeadNode) OnKEYGENEcho(msg KEYGENEcho, fromNodeIndex big.Int) error {
	return nil
}
func (mn *mockDeadNode) OnKEYGENReady(msg KEYGENReady, fromNodeIndex big.Int) error {
	return nil
}
func (mn *mockDeadNode) OnKEYGENShareComplete(keygenShareCompletes []KEYGENShareComplete, fromNodeIndex big.Int) error {
	return nil
}
