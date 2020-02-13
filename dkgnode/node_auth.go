package dkgnode

import (
	// "crypto/ecdsa"
	"fmt"

	"github.com/torusresearch/torus-common/common"
	"github.com/torusresearch/torus-common/crypto"
)

// verifyDataWithNodelist- returns if data is valid
func (e *EthereumService) verifyDataWithNodelist(pk common.Point, sig []byte, data []byte) (senderDetails NodeDetails, err error) {
	// Check if PubKey Exists in Nodelist
	nodeExists := false
	var foundNode *NodeReference
	e.Lock()
	for _, nodeRegister := range e.nodeRegisterMap {
		for _, nodeRef := range nodeRegister.NodeList {
			if nodeRef.PublicKey.X.Cmp(&pk.X) == 0 && nodeRef.PublicKey.Y.Cmp(&pk.Y) == 0 {
				foundNode = nodeRef
				nodeExists = true
			}
		}
	}
	e.Unlock()
	if !nodeExists {
		err = fmt.Errorf("node doesnt exist in node register map")
		return
	}
	// Check validity of signature
	valid := crypto.VerifyPtFromRaw(data, pk, sig)
	if !valid {
		err = fmt.Errorf("invalid ecdsa sig for data %v", data)
		return
	}
	return NodeDetails{
		Index: int(foundNode.Index.Int64()),
		PubKey: common.Point{
			X: *foundNode.PublicKey.X,
			Y: *foundNode.PublicKey.Y,
		},
	}, err
}

// verifyDataWithEpoch- returns if data is valid in a particular epoch
func (e *EthereumService) verifyDataWithEpoch(pk common.Point, sig []byte, data []byte, epoch int) (senderDetails NodeDetails, err error) {
	// Check if PubKey Exists in Nodelist
	nodeExists := false
	var foundNode *NodeReference
	e.Lock()
	nodeRegister, ok := e.nodeRegisterMap[epoch]
	if !ok {
		err = fmt.Errorf("epoch doesnt exist in node register map, verifyDataWithEpoch")
		return
	}
	for _, nodeRef := range nodeRegister.NodeList {
		if nodeRef.PublicKey.X.Cmp(&pk.X) == 0 && nodeRef.PublicKey.Y.Cmp(&pk.Y) == 0 {
			foundNode = nodeRef
			nodeExists = true
		}
	}
	e.Unlock()
	if !nodeExists {
		err = fmt.Errorf("node doesnt exist in node register map")
		return
	}

	// Check validity of signature
	valid := crypto.VerifyPtFromRaw(data, pk, sig)
	if !valid {
		err = fmt.Errorf("invalid ecdsa sig for data %v", data)
		return
	}
	return NodeDetails{
		Index: int(foundNode.Index.Int64()),
		PubKey: common.Point{
			X: *foundNode.PublicKey.X,
			Y: *foundNode.PublicKey.Y,
		},
	}, err
}

func (e *EthereumService) selfSignData(data []byte) []byte {
	ecSig := crypto.SignData(data, e.nodePrivK)
	return ecSig.Raw
}
