package dkgnode

import (
	"github.com/looplab/fsm"
)


type AVSSKeygen interface {
	Phase *fsm.FSM
	
}