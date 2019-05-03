package dkgnode

import (
	"time"

	"github.com/looplab/fsm"
	"github.com/torusresearch/torus-public/logging"
)

// LocalStatusConstants: Constants for the Nodes Local Status
type localStatusConstants struct {
	States localStatusStates
	Events localStatusEvents
}

type localStatusStates struct {
	Standby       string
	RunningKeygen string
}

type localStatusEvents struct {
	StartKeygen    string
	KeygenComplete string
}

var lsStates = localStatusStates{
	Standby:       "standby",
	RunningKeygen: "running_keygen",
}

var lsEvents = localStatusEvents{
	StartKeygen:    "start_keygen",
	KeygenComplete: "keygen_complete",
}

type LocalStatus struct {
	fsm.FSM
	Constants localStatusConstants
}

func SetupKeygen(suite *Suite) {
	constants := LocalStatusConstants
	tempFsm := fsm.NewFSM(
		constants.States.Standby,
		fsm.Events{
			{Name: constants.Events.StartKeygen, Src: []string{constants.States.Standby}, Dst: constants.States.RunningKeygen},
			{Name: constants.Events.KeygenComplete, Src: []string{constants.States.RunningKeygen}, Dst: constants.States.Standby},
		},
		fsm.Callbacks{
			"enter_state": func(e *fsm.Event) { logging.Infof("STATUSTX: local status set from %s to %s", e.Src, e.Dst) },
			"after_" + constants.Events.StartKeygen: func(e *fsm.Event) {
				//caters for if Keygen has already been instanciated
				_, ok := suite.P2PSuite.KeygenProto.KeygenInstances[getKeygenID(e.Args[0].(int), e.Args[1].(int))]
				if !ok {
					suite.P2PSuite.KeygenProto.NewKeygen(suite, e.Args[0].(int), e.Args[1].(int))
				}
				time.Sleep(10 * time.Second) // TODO(Zen): Remove after testing
				go suite.P2PSuite.KeygenProto.InitiateKeygen(suite, e.Args[0].(int), e.Args[1].(int))
			},
			"after_" + constants.Events.KeygenComplete: func(e *fsm.Event) {
				// update total number of available keys and epoch
				// TODO: is this a good idea?? shouldnt these updates be deterministic
				suite.ABCIApp.state.LastCreatedIndex = suite.ABCIApp.state.LastCreatedIndex + uint(suite.ABCIApp.Suite.Config.KeysPerEpoch)
				// TODO: do we need to update epoch? isn't epoch between nodes?
			},
		},
	)
	suite.KeygenSuite = &KeygenSuite{
		&LocalStatus{
			*tempFsm,
		},
	}
}

func SetupFSM(suite *Suite) {
	constants := localStatusConstants{States: lsStates, Events: lsEvents}
	tempFsm := fsm.NewFSM(
		constants.States.Standby,
		fsm.Events{
			{Name: constants.Events.StartKeygen, Src: []string{constants.States.Standby}, Dst: constants.States.RunningKeygen},
			{Name: constants.Events.KeygenComplete, Src: []string{constants.States.RunningKeygen}, Dst: constants.States.Standby},
		},
		fsm.Callbacks{
			"enter_state": func(e *fsm.Event) { logging.Infof("STATUSTX: local status set from %s to %s", e.Src, e.Dst) },
			"after_" + constants.Events.StartKeygen: func(e *fsm.Event) {
				//caters for if Keygen has already been instanciated
				_, ok := suite.P2PSuite.KeygenProto.KeygenInstances[getKeygenID(e.Args[0].(int), e.Args[1].(int))]
				if !ok {
					suite.P2PSuite.KeygenProto.NewKeygen(suite, e.Args[0].(int), e.Args[1].(int))
				}
				time.Sleep(10 * time.Second) // TODO(Zen): Remove after testing
				go suite.P2PSuite.KeygenProto.InitiateKeygen(suite, e.Args[0].(int), e.Args[1].(int))
			},
			"after_" + constants.Events.KeygenComplete: func(e *fsm.Event) {
				// update total number of available keys and epoch
				suite.ABCIApp.state.LastCreatedIndex = suite.ABCIApp.state.LastCreatedIndex + uint(suite.ABCIApp.Suite.Config.KeysPerEpoch)
				suite.ABCIApp.state.Epoch = suite.ABCIApp.state.Epoch + uint(1)
			},
		},
	)
	suite.LocalStatus = &LocalStatus{
		*tempFsm,
		constants,
	}
}
