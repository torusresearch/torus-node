package keygen

import (
	"errors"
	"math/big"

	"github.com/torusresearch/torus-public/common"
)

// Check if its is a qualified node in this keygen instance
func (ki *KeygenInstance) isQualifiedNode(nodeIndex string) (bool, error) {
	nodeLog, ok := ki.NodeLog[nodeIndex]
	if !ok {
		return false, errors.New("Node does not exist in keygen")
	}

	if nodeLog.Is(SNUnqualifiedNode) {
		return false, errors.New("Node is unqualified")
	}
	return true, nil
}

// For external use - Check if its is a qualified node in this keygen instance
func (ki *KeygenInstance) IsQualifiedNode(nodeIndex string) (bool, error) {
	ki.Lock()
	defer ki.Unlock()
	return ki.isQualifiedNode(nodeIndex)
}

func (ki *KeygenInstance) removeNodeFromQualifedSet(index string) error {
	//remove node from qualified set
	node, ok := ki.NodeLog[index]
	if !ok {
		return errors.New("Node doesnt exist in set")
	}

	ki.UnqualifiedNodes[index] = node
	delete(ki.NodeLog, index)
	return nil
}

func derivePolynomialFromPoints([]common.Point) ([]big.Int, error) {
	return nil, nil
}

func testEqualStringArr(a, b []string) bool {

	// If one is nil, the other must also be nil.
	if (a == nil) != (b == nil) {
		return false
	}

	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}

	return true
}
