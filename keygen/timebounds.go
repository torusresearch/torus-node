package keygen

import (
	"errors"
)

// Call this (to be triggered by timing set and checked by tendermint) when time has exceeded timebound one (t1)
func (ki *KeygenInstance) TriggerRoundOneTimebound() error {
	ki.Lock()
	defer ki.Unlock()
	// if ki.State.Is(SIWaitingInitiateKeygen) { // check if in correct state
	nodesThatLackCommitments := make(map[string]bool)
	allIn := true
	for _, dealers := range ki.KeyLog {
		for nodeIndexStr, subKey := range dealers {
			if subKey.C == nil {
				allIn = false
				nodesThatLackCommitments[nodeIndexStr] = true
			}
		}
	}
	if allIn {
		return errors.New("Can't TriggerRoundOneTimebound when not in all initiate keygens are in")
	}

	for invalidNode := range nodesThatLackCommitments {
		ki.removeNodeFromQualifedSet(invalidNode)
	}

	if len(ki.NodeLog) < ki.Threshold {
		err := errors.New("Number of qualified nodes under threshold in timebound 1")
		ki.ComChannel <- "keygen failed: " + err.Error()
		return err
	}

	// sendKEYGENSends keygen with qualified set
	ki.prepareAndSendKEYGENSend()
	// err := ki.State.Event(EIAllInitiateKeygen)
	// if err != nil {
	// 	logging.Errorf("Error initiating keygen with smaller set: %v", err)
	// }

	return nil
}
