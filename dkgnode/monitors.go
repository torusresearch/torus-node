package dkgnode

import (

	// "strconv"

	"github.com/torusresearch/torus-node/pss"
)

type NodeListUpdates struct {
	Type    string
	Payload interface{}
}

type KeyGenUpdates struct {
	Type    string
	Payload interface{}
}

var MessageSourceMapping = map[MessageSource]string{
	BFT: "BFT",
	P2P: "P2P",
}

type MessageSource int

const (
	BFT MessageSource = iota
	P2P
)

type PSSWorkerUpdate struct {
	Type    string
	Payload interface{}
}

type BftWorkerUpdates struct {
	Type    string
	Payload interface{}
}

type WhitelistMonitorUpdates struct {
	Type    string
	Payload interface{}
}

type PSSStartData struct {
	Message   string
	OldEpoch  int
	OldEpochN int
	OldEpochK int
	OldEpochT int
	NewEpoch  int
	NewEpochN int
	NewEpochK int
	NewEpochT int
}

type PSSUpdate struct {
	MessageSource MessageSource
	NodeDetails   *NodeReference
	PSSMessage    pss.PSSMessage
}
