package dkgnode

import (

	"github.com/looplab/fsm"
	"github.com/torusresearch/torus-public/logging"
)


func SetupFSM(suite *Suite) {
	suite.LocalStatus = fsm.NewFSM(
		"standby",
		fsm.Events{
			{Name: "start_keygen", Src: []string{"standby"}, Dst: "running_keygen"},
			{Name: "keygen_complete", Src: []string{"running_keygen"}, Dst: "standby"},
		},
		fsm.Callbacks{
			"enter_state": func(e *fsm.Event) { logging.Infof("STATUSTX: local status set from %s to %s", e.Src, e.Dst) },
			"after_keygen_complete": func(e *fsm.Event) {
				// update total number of available keys and epoch
				suite.ABCIApp.state.LastCreatedIndex = suite.ABCIApp.state.LastCreatedIndex + uint(suite.ABCIApp.Suite.Config.KeysPerEpoch)
				suite.ABCIApp.state.Epoch = suite.ABCIApp.state.Epoch + uint(1)
			},
		},
	)
}