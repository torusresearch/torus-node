package keygen

import (
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/torusresearch/torus-public/common"
	"github.com/torusresearch/torus-public/logging"
)

// type Transport struct {
// 	BroadcastInitiateKeygen      func(commitmentMatrixes [][][]common.Point) error
// 	SendKEYGENSend               func(msg KEYGENSend, nodeIndex big.Int) error
// 	SendKEYGENEcho               func(msg KEYGENEcho, nodeIndex big.Int) error
// 	SendKEYGENReady              func(msg KEYGENReady, nodeIndex big.Int) error
// 	BroadcastKEYGENShareComplete func(keygenShareCompletes []KEYGENShareComplete) error
// }
type Transport struct {
	nodeIndex          big.Int
	nodeKegenInstances *map[string]*KeygenInstance
}

func (transport *Transport) SendKEYGENSend(msg KEYGENSend, to big.Int) error {
	fmt.Println("SendKEYGENSend Called: ", to)
	go func(ins map[string]*KeygenInstance) {
		err := ins[to.Text(16)].OnKEYGENSend(msg, transport.nodeIndex)
		if err != nil {
			fmt.Println("ERRROR SendKEYGENSend: ", err)
		}
	}((*transport.nodeKegenInstances))
	return nil
}

func (transport *Transport) SendKEYGENEcho(msg KEYGENEcho, to big.Int) error {
	fmt.Println("SendKEYGENEcho Called: ", to)
	go func(ins map[string]*KeygenInstance) {
		err := ins[to.Text(16)].OnKEYGENEcho(msg, transport.nodeIndex)
		if err != nil {
			fmt.Println("ERRROR OnKEYGENEcho: ", err)
		}
	}((*transport.nodeKegenInstances))
	return nil
}

func (transport *Transport) SendKEYGENReady(msg KEYGENReady, to big.Int) error {
	fmt.Println("SendKEYGENReady: ", to)
	go func(ins map[string]*KeygenInstance) {
		err := ins[to.Text(16)].OnKEYGENReady(msg, transport.nodeIndex)
		if err != nil {
			fmt.Println("ERRROR OnKEYGENReady: ", err)
		}
	}((*transport.nodeKegenInstances))
	return nil
}

func (transport *Transport) BroadcastInitiateKeygen(commitmentMatrixes [][][]common.Point) error {
	logging.Debugf("Broadcast Initiate Keygen Called: %s", transport.nodeIndex)
	time.Sleep(1 * time.Second)
	for _, instance := range *transport.nodeKegenInstances {
		// logging.Debugf("index: %s", k)
		go func(ins *KeygenInstance, cm [][][]common.Point, tns big.Int) {
			err := ins.OnInitiateKeygen(commitmentMatrixes, tns)
			if err != nil {
				fmt.Println("ERRROR BroadcastInitiateKeygen: ", err)
			}
		}(instance, commitmentMatrixes, transport.nodeIndex)
	}
	return nil
}

func (transport *Transport) BroadcastKEYGENShareComplete(keygenShareCompletes []KEYGENShareComplete) error {
	fmt.Println("BroadcastKEYGENShareComplete Called: ", transport.nodeIndex)
	time.Sleep(1 * time.Second)
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

func TestKeygen(t *testing.T) {
	logging.SetLevelString("debug")
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
		transport := Transport{nodeIndex: nodeIndex, nodeKegenInstances: &nodeKegenInstances}
		v.Transport = &transport
	}

	//start!
	for _, nodeIndex := range nodeList {
		t.Log("Initiating Nodes. Index: ", nodeIndex.Text(16))
		go func(nIndex big.Int) {
			err := nodeKegenInstances[nIndex.Text(16)].InitiateKeygen(*big.NewInt(int64(0)), 1, nodeList, threshold, nIndex)
			defer func() {
				if err != nil {
					t.Logf("Initiate Keygen error: %s", err)
				}
			}()
		}(nodeIndex)
	}

	time.Sleep(15 * time.Second)

	for _, nodeIndex := range nodeList {
		instance := nodeKegenInstances[nodeIndex.Text(16)]
		instance.Lock()
		t.Log(nodeIndex.Text(16), instance.State.Current())
		instance.Unlock()
	}

}
