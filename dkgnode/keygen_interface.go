package dkgnode

import (
	"math/big"

	"github.com/torusresearch/torus-public/common"
)

type KEYGENEcho struct {
	SecretIndex big.Int
	Aij         big.Int
	Aprimeij    big.Int
	Bij         big.Int
	Bprimeij    big.Int
}
type KEYGENReady struct {
	SecretIndex big.Int
	Aij         big.Int
	Aprimeij    big.Int
	Bij         big.Int
	Bprimeij    big.Int
}
type KEYGENNode struct {
	SecretIndex    big.Int
	AIY            common.PrimaryPolynomial
	AIprimeY       common.PrimaryPolynomial
	BIX            common.PrimaryPolynomial
	BIprimeX       common.PrimaryPolynomial
	ReceivedEchoes []KEYGENEcho
	ReceivedReadys []KEYGENReady
}

type KEYGENSend struct {
	SecretIndex big.Int
	AIY         common.PrimaryPolynomial
	AIprimeY    common.PrimaryPolynomial
	BIX         common.PrimaryPolynomial
	BIprimeX    common.PrimaryPolynomial
}

type KEYGENShareComplete struct {
	SecretIndex big.Int
	c           big.Int
	u1          big.Int
	u2          big.Int
	gsi         common.Point
	gsihr       common.Point
}

type AVSSKeygen interface {
	// Trigger Start for Keygen
	InitiateKeygen(startingIndex big.Int, endingIndex big.Int) error

	// "Client" Actions
	broadcastInitiateKeygen(commitmentMatrixes [][][]common.Point, nodeIndex big.Int) error
	sendKEYGENSend(msg KEYGENSend, nodeIndex big.Int) error
	sendKEYGENEcho(msg KEYGENEcho, nodeIndex big.Int) error
	sendKEYGENReady(msg KEYGENReady, nodeIndex big.Int) error
	broadcastKEYGENShareComplete(msg KEYGENShareComplete, nodeIndex big.Int) error

	// Listeners and Reactions
	onInitiateKeygen(commitmentMatrixes [][][]common.Point, nodeIndex big.Int) error
	onKEYGENSend(msg KEYGENSend, fromNodeIndex big.Int) error
	onKEYGENEcho(msg KEYGENEcho, fromNodeIndex big.Int) error
	onKEYGENReady(msg KEYGENReady, fromNodeIndex big.Int) error
	onKEYGENShareComplete(msg KEYGENShareComplete, fromNodeIndex big.Int) error
}
