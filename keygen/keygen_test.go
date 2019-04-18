package keygen

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/torusresearch/torus-public/pvss"
	"github.com/torusresearch/torus-public/secp256k1"
	"log"
	"math/big"
	"os"
	"runtime"
	"runtime/pprof"
	"testing"
	"time"

	"github.com/torusresearch/torus-public/common"
	"github.com/torusresearch/torus-public/logging"
)

const XXXTestLogging = "debug"

func TestOptimisticKeygen(t *testing.T) {

	f, err := os.Create("profile_optimistic.prof")
	if err != nil {
		log.Fatal(err)
	}
	pprof.StartCPUProfile(f)
	defer pprof.StopCPUProfile()
	runtime.GOMAXPROCS(10)
	logging.SetLevelString(XXXTestLogging)
	comsChannel := make(chan string)
	numOfNodes := 9
	threshold := 5
	malNodes := 2
	numKeys := 5
	nodeList := make([]big.Int, numOfNodes)
	nodeKegenInstances := make(map[string]*KeygenInstance)
	pubKeys := make(map[string]*common.Point)
	privKeys := make(map[string]*big.Int)
	// sets up node list
	for i := range nodeList {
		nodeList[i] = *big.NewInt(int64(i + 1))
	}

	for i := range nodeList {
		//set up store
		store := &mockKeygenStore{}
		instance, err := NewAVSSKeygen(*big.NewInt(int64(0)), numKeys, nodeList, threshold, malNodes, nodeList[i], nil, store, nil, comsChannel)
		if err != nil {
			t.Fatal(err)
		}
		privKeys[nodeList[i].Text(16)] = pvss.RandomBigInt()
		pt := common.BigIntToPoint(secp256k1.Curve.ScalarBaseMult(privKeys[nodeList[i].Text(16)].Bytes()))
		pubKeys[nodeList[i].Text(16)] = &pt
		nodeKegenInstances[nodeList[i].Text(16)] = instance
	}

	//edit transport functions
	for k, v := range nodeKegenInstances {
		var nodeIndex big.Int
		nodeIndex.SetString(k, 16)
		auth := nodeAuth{nodeIndex, pubKeys, privKeys}
		v.Auth = &auth
		transport := mockTransport{nodeIndex: nodeIndex, nodeKegenInstances: &nodeKegenInstances}
		t.Log(transport)
		v.Transport = &transport
	}

	//start!
	for _, nodeIndex := range nodeList {
		go func(nIndex big.Int) {
			err := nodeKegenInstances[nIndex.Text(16)].InitiateKeygen()
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

	f, err := os.Create("profile_timebound_one.prof")
	if err != nil {
		log.Fatal(err)
	}
	pprof.StartCPUProfile(f)
	defer pprof.StopCPUProfile()

	runtime.GOMAXPROCS(10)
	logging.SetLevelString(XXXTestLogging)
	comsChannel := make(chan string)
	numOfNodes := 9
	threshold := 5
	malNodes := 2
	numKeys := 5
	nodeList := make([]big.Int, numOfNodes)
	nodeKegenInstances := make(map[string]*KeygenInstance)
	done := false
	// build timer function to kill of exceeds time
	go func(d *bool) {
		time.Sleep(10 * time.Second)
		comsChannel <- "timeout"
	}(&done)

	pubKeys := make(map[string]*common.Point)
	privKeys := make(map[string]*big.Int)
	// sets up node list
	for i := range nodeList {
		nodeList[i] = *big.NewInt(int64(i + 1))
	}

	for i := range nodeList {
		//set up store
		store := &mockKeygenStore{}
		instance, err := NewAVSSKeygen(*big.NewInt(int64(0)), numKeys, nodeList, threshold, malNodes, nodeList[i], nil, store, nil, comsChannel)
		if err != nil {
			t.Fatal(err)
		}
		privKeys[nodeList[i].Text(16)] = pvss.RandomBigInt()
		pt := common.BigIntToPoint(secp256k1.Curve.ScalarBaseMult(privKeys[nodeList[i].Text(16)].Bytes()))
		pubKeys[nodeList[i].Text(16)] = &pt
		nodeKegenInstances[nodeList[i].Text(16)] = instance
	}

	//edit transport functions
	for k, v := range nodeKegenInstances {
		var nodeIndex big.Int
		nodeIndex.SetString(k, 16)
		auth := nodeAuth{nodeIndex, pubKeys, privKeys}
		v.Auth = &auth
		transport := mockTransport{nodeIndex: nodeIndex, nodeKegenInstances: &nodeKegenInstances}
		if nodeIndex.Cmp(big.NewInt(int64(1))) == 0 {
			v.Transport = &mockDeadTransport{}
		} else {
			v.Transport = &transport
		}
	}

	//start!
	for _, nodeIndex := range nodeList {
		go func(nIndex big.Int) {
			err := nodeKegenInstances[nIndex.Text(16)].InitiateKeygen()
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

	// trigger timebound one here
	for _, nodeIndex := range nodeList {
		// to not cause a panic
		// if i == 0 {
		// 	continue
		// }
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
			if msg == "timeout" {
				done = true
			}
		}
		if count >= len(nodeList)-1 || done { // accounted for here
			break
		}

	}
	// time.Sleep(12 * time.Second)

	// log node status
	for i, nodeIndex := range nodeList {
		// to not cause a panic
		if i == 0 {
			continue
		}
		instance := nodeKegenInstances[nodeIndex.Text(16)]
		instance.Lock()
		t.Log(nodeIndex.Text(16), instance.State.Current())
		for keyIndex := range instance.KeyLog {
			for g, ni := range nodeList {
				if g == 0 {
					continue
				}
				if instance.KeyLog[keyIndex][ni.Text(16)].SubshareState.Current() != "perfect_subshare" {
					t.Log("KeyLogState from ", ni.Text(16), instance.KeyLog[big.NewInt(int64(0)).Text(16)][ni.Text(16)].SubshareState.Current())
					nodeLog := instance.KeyLog[big.NewInt(int64(0)).Text(16)][ni.Text(16)]
					t.Log("Number of Echos: ", len(nodeLog.ReceivedEchoes))
					t.Log("Number of Readys: ", len(nodeLog.ReceivedReadys))
					// 	t.Log("Ready:")
					// 	for _, ready := range nodeLog.ReceivedReadys {
					// 		t.Log(ready)
					// 	}
				}
			}
		}
		// assert.True(t, instance.State.Current() == SIKeygenCompleted, "Keygen not completed in TimeboundOne")
		instance.Unlock()
	}
}

// func TestTimeboundTwo(t *testing.T) {

// 	f, err := os.Create("profile_timebound_two.prof")
// 	if err != nil {
// 		log.Fatal(err)
// 	}
// 	pprof.StartCPUProfile(f)
// 	defer pprof.StopCPUProfile()

// 	runtime.GOMAXPROCS(10)
// 	logging.SetLevelString(XXXTestLogging)
// 	comsChannel := make(chan string)
// 	numOfNodes := 9
// 	threshold := 5
// 	malNodes := 2
// 	numKeys := 5
// 	nodeList := make([]big.Int, numOfNodes)
// 	nodeKegenInstances := make(map[string]*KeygenInstance)

// 	done := false
// 	// build timer function to kill of exceeds time
// 	go func(d *bool) {
// 		time.Sleep(10 * time.Second)
// 		comsChannel <- "timeout"
// 	}(&done)
// 	pubKeys := make(map[string]*common.Point)
// 	privKeys := make(map[string]*big.Int)
// 	// sets up node list
// 	for i := range nodeList {
// 		nodeList[i] = *big.NewInt(int64(i + 1))
// 	}

// 	for i := range nodeList {
// 		//set up store
// 		store := &mockKeygenStore{}
// 		instance, err := NewAVSSKeygen(*big.NewInt(int64(0)), numKeys, nodeList, threshold, malNodes, nodeList[i], nil, store, nil, comsChannel)
// 		if err != nil {
// 			t.Fatal(err)
// 		}
// 		privKeys[nodeList[i].Text(16)] = pvss.RandomBigInt()
// 		pt := common.BigIntToPoint(secp256k1.Curve.ScalarBaseMult(privKeys[nodeList[i].Text(16)].Bytes()))
// 		pubKeys[nodeList[i].Text(16)] = &pt
// 		nodeKegenInstances[nodeList[i].Text(16)] = instance
// 	}

// 	//edit transport functions
// 	for k, v := range nodeKegenInstances {
// 		var nodeIndex big.Int
// 		nodeIndex.SetString(k, 16)
// 		auth := nodeAuth{nodeIndex, pubKeys, privKeys}
// 		v.Auth = &auth
// 		transport := mockTransport{nodeIndex: nodeIndex, nodeKegenInstances: &nodeKegenInstances}
// 		if nodeIndex.Cmp(big.NewInt(int64(1))) == 0 {
// 			v.Transport = &mockDeadTransportTwo{nodeIndex: nodeIndex, nodeKegenInstances: &nodeKegenInstances}
// 		} else {
// 			v.Transport = &transport
// 		}
// 	}

// 	//start!
// 	for _, nodeIndex := range nodeList {
// 		go func(nIndex big.Int) {
// 			err := nodeKegenInstances[nIndex.Text(16)].InitiateKeygen()
// 			defer func() {
// 				if err != nil {
// 					t.Logf("Initiate Keygen error: %s", err)
// 				}
// 			}()
// 		}(nodeIndex)
// 	}

// 	time.Sleep(5 * time.Second)

// 	// log node status
// 	for i, nodeIndex := range nodeList {
// 		// to not cause a panic
// 		if i == 0 {
// 			continue
// 		}
// 		instance := nodeKegenInstances[nodeIndex.Text(16)]
// 		instance.Lock()
// 		t.Log(nodeIndex.Text(16), instance.State.Current())
// 		// for _, ni := range nodeList {
// 		// 	t.Log("KeyLogState from ", ni.Text(16), instance.KeyLog[big.NewInt(int64(0)).Text(16)][ni.Text(16)].SubshareState.Current())
// 		// }
// 		instance.Unlock()
// 	}

// 	// trigger timebound two here
// 	for i, nodeIndex := range nodeList {
// 		// to not cause a panic
// 		if i == 0 {
// 			continue
// 		}
// 		instance := nodeKegenInstances[nodeIndex.Text(16)]
// 		err := instance.TriggerRoundTwoTimebound()
// 		if err != nil {
// 			t.Log(err)
// 		}
// 	}

// 	// wait till nodes are done (w/o malicious node)
// 	count := 0
// 	for {
// 		select {
// 		case msg := <-comsChannel:
// 			if msg == SIKeygenCompleted {
// 				count++
// 			}
// 			if msg == "timeout" {
// 				done = true
// 			}
// 		}
// 		if count >= len(nodeList)-1 || done { // accounted for here
// 			break
// 		}
// 	}

// 	// time.Sleep(5 * time.Second)

// 	// // log node status
// 	for i, nodeIndex := range nodeList {
// 		// to not cause a panic
// 		if i == 0 {
// 			continue
// 		}
// 		instance := nodeKegenInstances[nodeIndex.Text(16)]
// 		instance.Lock()
// 		t.Log(nodeIndex.Text(16), instance.State.Current())
// 		assert.True(t, instance.State.Current() == SIKeygenCompleted, "Keygen not completed in TimeboundTwo")
// 		instance.Unlock()
// 	}
// }

func TestEchoReconstruction(t *testing.T) {
	f, err := os.Create("profile_echo_reconstruction.prof")
	if err != nil {
		log.Fatal(err)
	}
	pprof.StartCPUProfile(f)
	defer pprof.StopCPUProfile()

	runtime.GOMAXPROCS(10)
	logging.SetLevelString(XXXTestLogging)
	comsChannel := make(chan string)
	numOfNodes := 5
	threshold := 3
	malNodes := 0
	numKeys := 1
	nodeList := make([]big.Int, numOfNodes)
	nodeKegenInstances := make(map[string]*KeygenInstance)

	done := false
	// build timer function to kill of exceeds time
	go func(d *bool) {
		time.Sleep(10 * time.Second)
		comsChannel <- "timeout"
	}(&done)
	pubKeys := make(map[string]*common.Point)
	privKeys := make(map[string]*big.Int)
	// sets up node list
	for i := range nodeList {
		nodeList[i] = *big.NewInt(int64(i + 1))
	}

	for i := range nodeList {
		//set up store
		store := &mockKeygenStore{}
		instance, err := NewAVSSKeygen(*big.NewInt(int64(0)), numKeys, nodeList, threshold, malNodes, nodeList[i], nil, store, nil, comsChannel)
		if err != nil {
			t.Fatal(err)
		}
		privKeys[nodeList[i].Text(16)] = pvss.RandomBigInt()
		pt := common.BigIntToPoint(secp256k1.Curve.ScalarBaseMult(privKeys[nodeList[i].Text(16)].Bytes()))
		pubKeys[nodeList[i].Text(16)] = &pt
		nodeKegenInstances[nodeList[i].Text(16)] = instance
	}

	//edit transport functions
	for k, v := range nodeKegenInstances {
		var nodeIndex big.Int
		nodeIndex.SetString(k, 16)
		auth := nodeAuth{nodeIndex, pubKeys, privKeys}
		v.Auth = &auth
		transport := mockTransport{nodeIndex: nodeIndex, nodeKegenInstances: &nodeKegenInstances}
		if nodeIndex.Cmp(big.NewInt(int64(1))) == 0 {
			v.Transport = &mockEvilTransport{nodeIndex: nodeIndex, nodeKegenInstances: &nodeKegenInstances, ignore: nodeList[1]}
		} else {
			v.Transport = &transport
		}
		//set up store
		v.Store = &mockKeygenStore{}
	}

	//start!
	for _, nodeIndex := range nodeList {
		go func(nIndex big.Int) {
			err := nodeKegenInstances[nIndex.Text(16)].InitiateKeygen()
			defer func() {
				if err != nil {
					t.Logf("Initiate Keygen error: %s", err)
				}
			}()
		}(nodeIndex)
	}

	// wait till nodes are done (w/o malicious node)
	count := 0
	for {
		select {
		case msg := <-comsChannel:
			if msg == SIKeygenCompleted {
				count++
			}
			if msg == "timeout" {
				done = true
			}
		}
		if count >= len(nodeList) || done { // accounted for here
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
			}
		}
		instance.Unlock()
	}
	assert.True(t, !done)
}

func TestDKGCompleteSync(t *testing.T) {

	f, err := os.Create("profile_dkg_complete_sync.prof")
	if err != nil {
		log.Fatal(err)
	}
	pprof.StartCPUProfile(f)
	defer pprof.StopCPUProfile()

	runtime.GOMAXPROCS(10)
	logging.SetLevelString(XXXTestLogging)
	comsChannel := make(chan string)
	numOfNodes := 9
	threshold := 5
	malNodes := 2
	numKeys := 1
	nodeList := make([]big.Int, numOfNodes)
	nodeKegenInstances := make(map[string]*KeygenInstance)

	done := false
	// build timer function to kill of exceeds time
	go func(d *bool) {
		time.Sleep(10 * time.Second)
		comsChannel <- "timeout"
	}(&done)

	pubKeys := make(map[string]*common.Point)
	privKeys := make(map[string]*big.Int)
	// sets up node list
	for i := range nodeList {
		nodeList[i] = *big.NewInt(int64(i + 1))
	}

	for i := range nodeList {
		//set up store
		store := &mockKeygenStore{}
		instance, err := NewAVSSKeygen(*big.NewInt(int64(0)), numKeys, nodeList, threshold, malNodes, nodeList[i], nil, store, nil, comsChannel)
		if err != nil {
			t.Fatal(err)
		}
		privKeys[nodeList[i].Text(16)] = pvss.RandomBigInt()
		pt := common.BigIntToPoint(secp256k1.Curve.ScalarBaseMult(privKeys[nodeList[i].Text(16)].Bytes()))
		pubKeys[nodeList[i].Text(16)] = &pt
		nodeKegenInstances[nodeList[i].Text(16)] = instance
	}

	//edit transport functions
	for k, v := range nodeKegenInstances {
		var nodeIndex big.Int
		nodeIndex.SetString(k, 16)
		auth := nodeAuth{nodeIndex, pubKeys, privKeys}
		v.Auth = &auth
		transport := mockTransport{nodeIndex: nodeIndex, nodeKegenInstances: &nodeKegenInstances}
		if nodeIndex.Cmp(big.NewInt(int64(1))) == 0 || nodeIndex.Cmp(big.NewInt(int64(1))) == 1 || nodeIndex.Cmp(big.NewInt(int64(1))) == 2 {
			v.Transport = &mockLaggingTransport{nodeIndex: nodeIndex, nodeKegenInstances: &nodeKegenInstances}
		} else {
			v.Transport = &transport
		}
	}

	//start!
	for _, nodeIndex := range nodeList {
		go func(nIndex big.Int) {
			err := nodeKegenInstances[nIndex.Text(16)].InitiateKeygen()
			defer func() {
				if err != nil {
					t.Logf("Initiate Keygen error: %s", err)
				}
			}()
		}(nodeIndex)
	}
	// wait till nodes are done (w/o malicious node)
	count := 0
	for {
		select {
		case msg := <-comsChannel:
			if msg == SIKeygenCompleted {
				count++
			}
			if msg == "timeout" {
				done = true
			}
		}
		if count >= len(nodeList) || done { // accounted for here
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
			}
		}
		instance.Unlock()
	}
	assert.True(t, !done)
}

type nodeAuth struct {
	NodeIndex   big.Int
	PublicKeys  map[string]*common.Point
	PrivateKeys map[string]*big.Int
}

func (l *nodeAuth) Sign(msg string) ([]byte, error) {
	return pvss.ECDSASign(msg, l.PrivateKeys[l.NodeIndex.Text(16)]), nil
}

func (l *nodeAuth) Verify(msg string, nodeIndex big.Int, sig []byte) bool {
	return pvss.ECDSAVerify(msg, l.PublicKeys[nodeIndex.Text(16)], sig)
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

func (transport *mockTransport) BroadcastKEYGENDKGComplete(keygenShareCompletes KEYGENDKGComplete) error {
	for _, instance := range *transport.nodeKegenInstances {
		go func(ins *KeygenInstance, cm KEYGENDKGComplete, tns big.Int) {
			err := ins.OnKEYGENDKGComplete(cm, tns)
			if err != nil {
				fmt.Println("ERRROR BroadcastKEYGENDKGComplete: ", err)
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

func (transport *mockDeadTransport) BroadcastKEYGENDKGComplete(keygenShareCompletes KEYGENDKGComplete) error {
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

func (transport *mockDeadTransportTwo) BroadcastKEYGENDKGComplete(keygenShareCompletes KEYGENDKGComplete) error {
	return nil
}

type mockEvilTransport struct {
	nodeIndex          big.Int
	nodeKegenInstances *map[string]*KeygenInstance
	ignore             big.Int
}

func (transport *mockEvilTransport) SendKEYGENSend(msg KEYGENSend, to big.Int) error {
	if to.Text(16) != transport.ignore.Text(16) {
		go func(ins map[string]*KeygenInstance) {
			err := ins[to.Text(16)].OnKEYGENSend(msg, transport.nodeIndex)
			if err != nil {
				fmt.Println("ERRROR SendKEYGENSend: ", err)
			}
		}((*transport.nodeKegenInstances))
	}
	return nil
}

func (transport *mockEvilTransport) SendKEYGENEcho(msg KEYGENEcho, to big.Int) error {
	if to.Text(16) != transport.ignore.Text(16) {
		go func(ins map[string]*KeygenInstance) {
			err := ins[to.Text(16)].OnKEYGENEcho(msg, transport.nodeIndex)
			if err != nil {
				fmt.Println("ERRROR OnKEYGENEcho: ", err)
			}
		}((*transport.nodeKegenInstances))
	}
	return nil
}

func (transport *mockEvilTransport) SendKEYGENReady(msg KEYGENReady, to big.Int) error {
	if to.Text(16) != transport.ignore.Text(16) {
		go func(ins map[string]*KeygenInstance) {
			err := ins[to.Text(16)].OnKEYGENReady(msg, transport.nodeIndex)
			if err != nil {
				fmt.Println("ERRROR OnKEYGENReady: ", err)
			}
		}((*transport.nodeKegenInstances))
	}
	return nil
}

func (transport *mockEvilTransport) BroadcastInitiateKeygen(commitmentMatrixes [][][]common.Point) error {
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

func (transport *mockEvilTransport) BroadcastKEYGENDKGComplete(keygenShareCompletes KEYGENDKGComplete) error {
	for _, instance := range *transport.nodeKegenInstances {
		go func(ins *KeygenInstance, cm KEYGENDKGComplete, tns big.Int) {
			err := ins.OnKEYGENDKGComplete(cm, tns)
			if err != nil {
				fmt.Println("ERRROR BroadcastKEYGENDKGComplete: ", err)
			}
		}(instance, keygenShareCompletes, transport.nodeIndex)
	}
	return nil
}

type mockLaggingTransport struct {
	nodeIndex          big.Int
	nodeKegenInstances *map[string]*KeygenInstance
}

func (transport *mockLaggingTransport) SendKEYGENSend(msg KEYGENSend, to big.Int) error {
	go func(ins map[string]*KeygenInstance) {
		err := ins[to.Text(16)].OnKEYGENSend(msg, transport.nodeIndex)
		if err != nil {
			fmt.Println("ERRROR SendKEYGENSend: ", err)
		}
	}((*transport.nodeKegenInstances))
	return nil
}

func (transport *mockLaggingTransport) SendKEYGENEcho(msg KEYGENEcho, to big.Int) error {
	go func(ins map[string]*KeygenInstance) {
		err := ins[to.Text(16)].OnKEYGENEcho(msg, transport.nodeIndex)
		if err != nil {
			fmt.Println("ERRROR OnKEYGENEcho: ", err)
		}
	}((*transport.nodeKegenInstances))
	return nil
}

func (transport *mockLaggingTransport) SendKEYGENReady(msg KEYGENReady, to big.Int) error {
	go func(ins map[string]*KeygenInstance) {
		err := ins[to.Text(16)].OnKEYGENReady(msg, transport.nodeIndex)
		if err != nil {
			fmt.Println("ERRROR OnKEYGENReady: ", err)
		}
	}((*transport.nodeKegenInstances))
	return nil
}

func (transport *mockLaggingTransport) BroadcastInitiateKeygen(commitmentMatrixes [][][]common.Point) error {
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

func (transport *mockLaggingTransport) BroadcastKEYGENDKGComplete(keygenShareCompletes KEYGENDKGComplete) error {
	if keygenShareCompletes.Nonce != 0 && keygenShareCompletes.Nonce != 1 {
		for _, instance := range *transport.nodeKegenInstances {
			go func(ins *KeygenInstance, cm KEYGENDKGComplete, tns big.Int) {
				err := ins.OnKEYGENDKGComplete(cm, tns)
				if err != nil {
					fmt.Println("ERRROR BroadcastKEYGENDKGComplete: ", err)
				}
			}(instance, keygenShareCompletes, transport.nodeIndex)
		}
	}
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
func (mn *mockDeadNode) OnKEYGENDKGComplete(keygenShareCompletes KEYGENDKGComplete, fromNodeIndex big.Int) error {
	return nil
}
