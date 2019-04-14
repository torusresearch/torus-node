package keygen

import (
	"errors"

	"github.com/torusresearch/torus-public/logging"
)

// Call this (to be triggered by timing set and checked by tendermint) when time has exceeded timebound one (t1)
func (ki *KeygenInstance) TriggerRoundOneTimebound() error {
	ki.Lock()
	defer ki.Unlock()
	if ki.State.Is(SIWaitingInitiateKeygen) { // check if in correct state
		for index, node := range ki.NodeLog {
			if node.Is(SNStandby) {
				// not in routine cause we need this to be synchronous
				err := node.Event(ENFailedRoundOne)
				if err != nil {
					return err
				}
				err = ki.removeNodeFromQualifedSet(index)
				if err != nil {
					return err
				}
			}
		}

		if len(ki.NodeLog) < ki.Threshold {
			err := errors.New("Number of qualified nodes under threshold in timebound 1")
			ki.ComChannel <- "keygen failed: " + err.Error()
			return err
		}

		// initiate keygen with qualified set
		go func() {
			err := ki.State.Event(EIAllInitiateKeygen)
			if err != nil {
				logging.Errorf("Error initiating keygen with smaller set: %v", err)
			}
		}()
	} else {
		return errors.New("Can't TriggerRoundOneTimebound when not in " + SIWaitingInitiateKeygen + " in state " + ki.State.Current())
	}
	return nil
}

// Call this (to be triggered by timing set and checked by tendermint) when time has exceeded timebound one (t1)
func (ki *KeygenInstance) TriggerRoundTwoTimebound() error {
	ki.Lock()
	defer ki.Unlock()
	if ki.State.Is(SIRunningKeygen) { // check if in correct state

		// continue with keygen with remaining qualified set
		go func() {
			err := ki.State.Event(EIAllSubsharesDone)
			if err != nil {
				logging.Errorf("Error initiating keygen with smaller set: %v", err)
			}
		}()
	} else {
		return errors.New("Can't TriggerRoundTwoTimebound when not in " + SIRunningKeygen + " in state " + ki.State.Current())
	}
	return nil
}
