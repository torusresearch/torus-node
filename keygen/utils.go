package keygen

import "errors"

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
