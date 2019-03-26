package keygen

import (
	"math/big"
	"testing"

	"github.com/torusresearch/torus-public/common"
)

type Transport struct {
	BroadcastInitiateKeygen      func(commitmentMatrixes [][][]common.Point) error
	SendKEYGENSend               func(msg KEYGENSend, nodeIndex big.Int) error
	SendKEYGENEcho               func(msg KEYGENEcho, nodeIndex big.Int) error
	SendKEYGENReady              func(msg KEYGENReady, nodeIndex big.Int) error
	BroadcastKEYGENShareComplete func(keygenShareCompletes []KEYGENShareComplete) error
}

func TestKeygen(t *testing.T) {

	numOfNodes := 5
	threshold := 4
	nodeList := make([]big.Int, numOfNodes)
	nodeKegenInstances := make(map[string]*KeygenInstance)
	for i := range nodeList {
		nodeList[i] = *big.NewInt(int64(i))
		nodeKegenInstances[nodeList[i].Text(16)] = &KeygenInstance{}
	}

	//edit transport functions
	for _, v := range nodeKegenInstances {
		transport := Transport{}
		transport.SendKEYGENSend = func(msg KEYGENSend, to big.Int) error {
			nodeKegenInstances[to.Text(16)].OnKEYGENSend(msg, v.NodeIndex)
			return nil
		}

		transport.SendKEYGENEcho = func(msg KEYGENEcho, to big.Int) error {
			nodeKegenInstances[to.Text(16)].OnKEYGENEcho(msg, v.NodeIndex)
			return nil
		}

		transport.SendKEYGENReady = func(msg KEYGENReady, to big.Int) error {
			nodeKegenInstances[to.Text(16)].OnKEYGENReady(msg, v.NodeIndex)
			return nil
		}

		transport.BroadcastInitiateKeygen = func(commitmentMatrixes [][][]common.Point) error {
			for _, instance := range nodeKegenInstances {
				instance.OnInitiateKeygen(commitmentMatrixes, v.NodeIndex)
			}
			return nil
		}

		transport.BroadcastKEYGENShareComplete = func(keygenShareCompletes []KEYGENShareComplete) error {
			for _, instance := range nodeKegenInstances {
				instance.OnKEYGENShareComplete(keygenShareCompletes, v.NodeIndex)
			}
			return nil
		}

	}

}
