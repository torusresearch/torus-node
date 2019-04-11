package keygen

import (
	"errors"
	"math/big"
	"strconv"

	"github.com/torusresearch/torus-public/logging"

	"github.com/torusresearch/torus-public/pvss"

	"github.com/torusresearch/torus-public/common"
)

// max(roundUp((n+t+1)/2), k)
func ecThreshold(n, k, t int) (res int) {
	nkt1half := n + t + 1
	nkt1half = (nkt1half + 1) / 2
	if nkt1half >= k {
		return nkt1half
	}
	return k
}

// ProcessMessage is called when the transport for the node receives a message via direct send.
// It works similar to a router, processing different messages differently based on their associated method.
// Each method handler's code path consists of parsing the message -> state checks -> logic -> state updates.
// Defer state changes until the end of the function call to ensure that the state is consistent in the handler.
// When sending messages to other nodes, it's important to use a goroutine to ensure that it isnt synchronous
func (pssNode *PSSNode) ProcessMessage(senderDetails NodeDetails, pssMessage PSSMessage) error {
	pssNode.Lock()
	defer pssNode.Unlock()
	if _, found := pssNode.PSSStore[pssMessage.PSSID]; !found && pssMessage.PSSID != NullPSSID {
		pssNode.PSSStore[pssMessage.PSSID] = &PSS{
			PSSID: pssMessage.PSSID,
			State: PSSState{
				States.Phases.Initial,
				States.Dealer.IsDealer,
				States.Player.IsPlayer,
				States.Recover.Initial,
				States.ReceivedSend.False,
				States.ReceivedEchoMap(),
				States.ReceivedReadyMap(),
			},
			CStore: make(map[CID]*C),
		}
	}
	pss, found := pssNode.PSSStore[pssMessage.PSSID]
	if found {
		pss.Lock()
		defer pss.Unlock()
		pss.Messages = append(pss.Messages, pssMessage)
	}

	// handle different messages here
	if pssMessage.Method == "share" {
		// parse message
		var pssMsgShare PSSMsgShare
		err := pssMsgShare.FromBytes(pssMessage.Data)
		if err != nil {
			logging.Error(err.Error())
			return err
		}

		// state checks
		if pss.State.Phase == States.Phases.Ended {
			return errors.New("PSS has ended, ignored message " + string(*pssMessage.JSON()) + " from " + string(senderDetails.ToNodeDetailsID()) + " ")
		}
		if pss.State.Dealer != States.Dealer.IsDealer {
			return errors.New("PSS could not be started since the node is not a dealer")
		}

		// logic
		sharing, found := pssNode.ShareStore[pssMsgShare.SharingID]
		if !found {
			return errors.New("Could not find sharing for sharingID " + string(pssMsgShare.SharingID))
		}
		sharing.Lock()
		defer sharing.Unlock()
		pss.F = pvss.GenerateRandomBivariatePolynomial(sharing.Si, pssNode.NewNodes.K)
		pss.Fprime = pvss.GenerateRandomBivariatePolynomial(sharing.Siprime, pssNode.NewNodes.K)
		pss.C = pvss.GetCommitmentMatrix(pss.F, pss.Fprime)

		for _, newNode := range pssNode.NewNodes.Nodes {
			pssID := (&PSSIDDetails{
				SharingID: sharing.SharingID,
				Index:     sharing.I,
			}).ToPSSID()
			pssMsgSend := &PSSMsgSend{
				PSSID:  pssID,
				C:      pss.C,
				A:      pvss.EvaluateBivarPolyAtX(pss.F, *big.NewInt(int64(newNode.Index))).Coeff,
				Aprime: pvss.EvaluateBivarPolyAtX(pss.Fprime, *big.NewInt(int64(newNode.Index))).Coeff,
				B:      pvss.EvaluateBivarPolyAtY(pss.F, *big.NewInt(int64(newNode.Index))).Coeff,
				Bprime: pvss.EvaluateBivarPolyAtY(pss.Fprime, *big.NewInt(int64(newNode.Index))).Coeff,
			}
			nextPSSMessage := PSSMessage{
				PSSID:  pssID,
				Method: "send",
				Data:   pssMsgSend.ToBytes(),
			}
			go func(newN NodeDetails, msg PSSMessage) {
				err := pssNode.Transport.Send(newN, msg)
				if err != nil {
					logging.Error(err.Error())
				}
			}(newNode, nextPSSMessage)

			nextNextPSSMessage := PSSMessage{
				PSSID:  NullPSSID,
				Method: "recover",
				Data: (&PSSMsgRecover{
					SharingID: pssMsgShare.SharingID,
					V:         sharing.C,
				}).ToBytes(),
			}
			go func(newN NodeDetails, msg PSSMessage) {
				err := pssNode.Transport.Send(newN, msg)
				if err != nil {
					logging.Error(err.Error())
				}
			}(newNode, nextNextPSSMessage)
		}
		defer func() { pss.State.Phase = States.Phases.Started }()
	} else if pssMessage.Method == "recover" {
		// parse message
		var pssMsgRecover PSSMsgRecover
		err := pssMsgRecover.FromBytes(pssMessage.Data)
		if err != nil {
			logging.Error(err.Error())
			return err
		}

		// state checks
		// if pss.State.Phase == States.Phases.Ended {
		// 	return errors.New("PSS has ended, ignored message " + string(*pssMessage.JSON()) + " from " + string(senderDetails.ToNodeDetailsID()) + " ")
		// }
		// if pss.State.Player != States.Player.IsPlayer {
		// 	return errors.New("Could not receive send message because node is not a player")
		// }
		// if pss.State.Recover != States.Recover.Initial || pss.State.Recover != States.Recover.WaitingForRecovers {
		// 	logging.Info("Already have enough recover messages")
		// 	return nil
		// }

		// logic
		if len(pssMsgRecover.V) == 0 {
			err := errors.New("Recover message commitment poly is of length 0")
			logging.Error(err.Error())
			return err
		}

		_, found := pssNode.RecoverStore[pssMsgRecover.SharingID]
		if !found {
			pssNode.RecoverStore[pssMsgRecover.SharingID] = &Recover{
				SharingID:        pssMsgRecover.SharingID,
				DCount:           make(map[VID]map[NodeDetailsID]bool),
				PSSCompleteCount: make(map[PSSID]bool),
			}
		}
		recover := pssNode.RecoverStore[pssMsgRecover.SharingID]
		vID := GetVIDFromPointArray(pssMsgRecover.V)
		if recover.DCount[vID] == nil {
			recover.DCount[vID] = make(map[NodeDetailsID]bool)
		}
		dCount := recover.DCount[vID]
		dCount[senderDetails.ToNodeDetailsID()] = true
		if len(dCount) == pssNode.NewNodes.T+1 {
			recover.D = &pssMsgRecover.V
			// defer func() { pss.State.Recover = States.Recover.WaitingForSelectedSharingsComplete }()
		}
	} else if pssMessage.Method == "send" {
		// parse message
		var pssMsgSend PSSMsgSend
		err := pssMsgSend.FromBytes(pssMessage.Data)
		if err != nil {
			logging.Error(err.Error())
			return err
		}
		// state checks
		if pss.State.Phase == States.Phases.Ended {
			return errors.New("PSS has ended, ignored message " + string(*pssMessage.JSON()) + " from " + string(senderDetails.ToNodeDetailsID()) + " ")
		}
		if pss.State.Player != States.Player.IsPlayer {
			return errors.New("Could not receive send message because node is not a player")
		}
		if pss.State.ReceivedSend == States.ReceivedSend.True {
			return errors.New("Already received a send message for PSSID " + string(pss.PSSID))
		}

		// logic
		var pssIDDetails PSSIDDetails
		err = pssIDDetails.FromPSSID(pssMessage.PSSID)
		if err != nil {
			logging.Error(err.Error())
			return err
		}
		senderID := pssNode.NewNodes.Nodes[senderDetails.ToNodeDetailsID()].Index
		if senderID == pssIDDetails.Index {
			defer func() { pss.State.ReceivedSend = States.ReceivedSend.True }()
		} else {
			return errors.New("'Send' message contains index of " + strconv.Itoa(pssIDDetails.Index) + " was not sent by node " + strconv.Itoa(senderID))
		}
		verified := pvss.AVSSVerifyPoly(
			pssMsgSend.C,
			*big.NewInt(int64(pssNode.NodeDetails.Index)),
			common.PrimaryPolynomial{Coeff: pssMsgSend.A, Threshold: pssNode.NewNodes.K},
			common.PrimaryPolynomial{Coeff: pssMsgSend.Aprime, Threshold: pssNode.NewNodes.K},
			common.PrimaryPolynomial{Coeff: pssMsgSend.B, Threshold: pssNode.NewNodes.K},
			common.PrimaryPolynomial{Coeff: pssMsgSend.Bprime, Threshold: pssNode.NewNodes.K},
		)
		if !verified {
			return errors.New("Could not verify polys against commitment")
		}
		for _, newNode := range pssNode.NewNodes.Nodes {
			pssMsgEcho := PSSMsgEcho{
				PSSID: pssMsgSend.PSSID,
				C:     pssMsgSend.C,
				Alpha: *pvss.PolyEval(
					common.PrimaryPolynomial{Coeff: pssMsgSend.A, Threshold: pssNode.NewNodes.K},
					*big.NewInt(int64(newNode.Index)),
				),
				Alphaprime: *pvss.PolyEval(
					common.PrimaryPolynomial{Coeff: pssMsgSend.Aprime, Threshold: pssNode.NewNodes.K},
					*big.NewInt(int64(newNode.Index)),
				),
				Beta: *pvss.PolyEval(
					common.PrimaryPolynomial{Coeff: pssMsgSend.B, Threshold: pssNode.NewNodes.K},
					*big.NewInt(int64(newNode.Index)),
				),
				Betaprime: *pvss.PolyEval(
					common.PrimaryPolynomial{Coeff: pssMsgSend.Bprime, Threshold: pssNode.NewNodes.K},
					*big.NewInt(int64(newNode.Index)),
				),
			}
			nextPSSMessage := PSSMessage{
				PSSID:  pss.PSSID,
				Method: "echo",
				Data:   pssMsgEcho.ToBytes(),
			}
			go func(newN NodeDetails, msg PSSMessage) {
				err := pssNode.Transport.Send(newN, msg)
				if err != nil {
					logging.Info(err.Error())
				}
			}(newNode, nextPSSMessage)
		}
	} else if pssMessage.Method == "echo" {
		// parse message
		defer func() { pss.State.ReceivedEcho[senderDetails.ToNodeDetailsID()] = States.ReceivedEcho.True }()
		var pssMsgEcho PSSMsgEcho
		err := pssMsgEcho.FromBytes(pssMessage.Data)
		if err != nil {
			logging.Error(err.Error())
			return err
		}

		// state checks
		if pss.State.Phase == States.Phases.Ended {
			return errors.New("PSS has ended, ignored message " + string(*pssMessage.JSON()) + " from " + string(senderDetails.ToNodeDetailsID()) + " ")
		}
		if pss.State.Player != States.Player.IsPlayer {
			return errors.New("Could not receive send message because node is not a player")
		}
		receivedEcho, found := pss.State.ReceivedEcho[senderDetails.ToNodeDetailsID()]
		if found && receivedEcho == States.ReceivedEcho.True {
			return errors.New("Already received a echo message for PSSID " + string(pss.PSSID) + "from sender " + string(senderDetails.ToNodeDetailsID()))
		}

		// logic
		verified := pvss.AVSSVerifyPoint(
			pssMsgEcho.C,
			*big.NewInt(int64(senderDetails.Index)),
			*big.NewInt(int64(pssNode.NodeDetails.Index)),
			pssMsgEcho.Alpha,
			pssMsgEcho.Alphaprime,
			pssMsgEcho.Beta,
			pssMsgEcho.Betaprime,
		)
		if !verified {
			return errors.New("Could not verify point against commitments for echo message")
		}

		cID := GetCIDFromPointMatrix(pssMsgEcho.C)
		_, found = pss.CStore[cID]
		if !found {
			pss.CStore[cID] = &C{
				CID:     cID,
				C:       pssMsgEcho.C,
				AC:      make(map[int]common.Point),
				ACprime: make(map[int]common.Point),
				BC:      make(map[int]common.Point),
				BCprime: make(map[int]common.Point),
			}
		}
		c := pss.CStore[cID]
		c.AC[senderDetails.Index] = common.Point{
			X: *big.NewInt(int64(senderDetails.Index)),
			Y: pssMsgEcho.Alpha,
		}
		c.ACprime[senderDetails.Index] = common.Point{
			X: *big.NewInt(int64(senderDetails.Index)),
			Y: pssMsgEcho.Alphaprime,
		}
		c.BC[senderDetails.Index] = common.Point{
			X: *big.NewInt(int64(senderDetails.Index)),
			Y: pssMsgEcho.Beta,
		}
		c.BCprime[senderDetails.Index] = common.Point{
			X: *big.NewInt(int64(senderDetails.Index)),
			Y: pssMsgEcho.Betaprime,
		}
		c.EC = c.EC + 1
		if c.EC == ecThreshold(pssNode.NewNodes.N, pssNode.NewNodes.K, pssNode.NewNodes.T) &&
			c.RC < pssNode.NewNodes.K {
			// Note: Despite the name mismatch below, this is correct, and the AVSS spec is wrong.
			c.Abar = pvss.LagrangeInterpolatePolynomial(GetPointArrayFromMap(c.BC))
			c.Abarprime = pvss.LagrangeInterpolatePolynomial(GetPointArrayFromMap(c.BCprime))
			c.Bbar = pvss.LagrangeInterpolatePolynomial(GetPointArrayFromMap(c.AC))
			c.Bbarprime = pvss.LagrangeInterpolatePolynomial(GetPointArrayFromMap(c.ACprime))
			for _, newNode := range pssNode.NewNodes.Nodes {
				pssMsgReady := PSSMsgReady{
					PSSID: pss.PSSID,
					C:     c.C,
					Alpha: *pvss.PolyEval(
						common.PrimaryPolynomial{Coeff: c.Abar, Threshold: pssNode.NewNodes.K},
						*big.NewInt(int64(newNode.Index)),
					),
					Alphaprime: *pvss.PolyEval(
						common.PrimaryPolynomial{Coeff: c.Abarprime, Threshold: pssNode.NewNodes.K},
						*big.NewInt(int64(newNode.Index)),
					),
					Beta: *pvss.PolyEval(
						common.PrimaryPolynomial{Coeff: c.Bbar, Threshold: pssNode.NewNodes.K},
						*big.NewInt(int64(newNode.Index)),
					),
					Betaprime: *pvss.PolyEval(
						common.PrimaryPolynomial{Coeff: c.Bbarprime, Threshold: pssNode.NewNodes.K},
						*big.NewInt(int64(newNode.Index)),
					),
				}
				nextPSSMessage := PSSMessage{
					PSSID:  pss.PSSID,
					Method: "ready",
					Data:   pssMsgReady.ToBytes(),
				}
				go func(newN NodeDetails, msg PSSMessage) {
					err := pssNode.Transport.Send(newN, msg)
					if err != nil {
						logging.Info(err.Error())
					}
				}(newNode, nextPSSMessage)
			}
		}
	} else if pssMessage.Method == "ready" {
		// parse message
		defer func() { pss.State.ReceivedReady[senderDetails.ToNodeDetailsID()] = States.ReceivedReady.True }()
		var pssMsgReady PSSMsgReady
		err := pssMsgReady.FromBytes(pssMessage.Data)
		if err != nil {
			logging.Error(err.Error())
			return err
		}
		var pssIDDetails PSSIDDetails
		err = pssIDDetails.FromPSSID(pssMsgReady.PSSID)
		if err != nil {
			logging.Error(err.Error())
			return err
		}

		// state checks
		if pss.State.Phase == States.Phases.Ended {
			return errors.New("PSS has ended, ignored message " + string(*pssMessage.JSON()) + " from " + string(senderDetails.ToNodeDetailsID()) + " ")
		}
		if pss.State.Player != States.Player.IsPlayer {
			return errors.New("Could not receive send message because node is not a player")
		}
		receivedReady, found := pss.State.ReceivedReady[senderDetails.ToNodeDetailsID()]
		if found && receivedReady == States.ReceivedReady.True {
			return errors.New("Already received a ready message for PSSID " + string(pss.PSSID))
		}

		// logic
		verified := pvss.AVSSVerifyPoint(
			pssMsgReady.C,
			*big.NewInt(int64(senderDetails.Index)),
			*big.NewInt(int64(pssNode.NodeDetails.Index)),
			pssMsgReady.Alpha,
			pssMsgReady.Alphaprime,
			pssMsgReady.Beta,
			pssMsgReady.Betaprime,
		)
		if !verified {
			return errors.New("Could not verify point against commitments for ready message")
		}

		cID := GetCIDFromPointMatrix(pssMsgReady.C)
		_, found = pss.CStore[cID]
		if !found {
			pss.CStore[cID] = &C{
				CID:     cID,
				C:       pssMsgReady.C,
				AC:      make(map[int]common.Point),
				ACprime: make(map[int]common.Point),
				BC:      make(map[int]common.Point),
				BCprime: make(map[int]common.Point),
			}
		}
		c := pss.CStore[cID]
		c.AC[senderDetails.Index] = common.Point{
			X: *big.NewInt(int64(senderDetails.Index)),
			Y: pssMsgReady.Alpha,
		}
		c.ACprime[senderDetails.Index] = common.Point{
			X: *big.NewInt(int64(senderDetails.Index)),
			Y: pssMsgReady.Alphaprime,
		}
		c.BC[senderDetails.Index] = common.Point{
			X: *big.NewInt(int64(senderDetails.Index)),
			Y: pssMsgReady.Beta,
		}
		c.BCprime[senderDetails.Index] = common.Point{
			X: *big.NewInt(int64(senderDetails.Index)),
			Y: pssMsgReady.Betaprime,
		}
		c.RC = c.RC + 1
		if c.RC == pssNode.NewNodes.K &&
			c.EC < ecThreshold(pssNode.NewNodes.N, pssNode.NewNodes.K, pssNode.NewNodes.T) {
			c.Abar = pvss.LagrangeInterpolatePolynomial(GetPointArrayFromMap(c.AC))
			c.Abarprime = pvss.LagrangeInterpolatePolynomial(GetPointArrayFromMap(c.ACprime))
			c.Bbar = pvss.LagrangeInterpolatePolynomial(GetPointArrayFromMap(c.BC))
			c.Bbarprime = pvss.LagrangeInterpolatePolynomial(GetPointArrayFromMap(c.BCprime))
			for _, newNode := range pssNode.NewNodes.Nodes {
				pssMsgReady := PSSMsgReady{
					PSSID: pss.PSSID,
					C:     c.C,
					Alpha: *pvss.PolyEval(
						common.PrimaryPolynomial{Coeff: c.Abar, Threshold: pssNode.NewNodes.K},
						*big.NewInt(int64(newNode.Index)),
					),
					Alphaprime: *pvss.PolyEval(
						common.PrimaryPolynomial{Coeff: c.Abarprime, Threshold: pssNode.NewNodes.K},
						*big.NewInt(int64(newNode.Index)),
					),
					Beta: *pvss.PolyEval(
						common.PrimaryPolynomial{Coeff: c.Bbar, Threshold: pssNode.NewNodes.K},
						*big.NewInt(int64(newNode.Index)),
					),
					Betaprime: *pvss.PolyEval(
						common.PrimaryPolynomial{Coeff: c.Bbarprime, Threshold: pssNode.NewNodes.K},
						*big.NewInt(int64(newNode.Index)),
					),
				}
				nextPSSMessage := PSSMessage{
					PSSID:  pss.PSSID,
					Method: "ready",
					Data:   pssMsgReady.ToBytes(),
				}
				go func(newN NodeDetails, msg PSSMessage) {
					err := pssNode.Transport.Send(newN, msg)
					if err != nil {
						logging.Error(err.Error())
					}
				}(newNode, nextPSSMessage)
			}
		} else if c.RC == pssNode.NewNodes.K+pssNode.NewNodes.T {
			pss.Cbar = c.C
			pss.Si = c.Abar[0]
			pss.Siprime = c.Abarprime[0]
			go func(msg string) {
				pssNode.Transport.Output(msg + " shared.")
			}(string(pss.PSSID))
			nextPSSMessage := PSSMessage{
				PSSID:  NullPSSID,
				Method: "complete",
				Data: (&PSSMsgComplete{
					PSSID: pss.PSSID,
					C00:   pss.Cbar[0][0],
				}).ToBytes(),
			}
			go func(ownNode NodeDetails, ownMsg PSSMessage) {
				err := pssNode.Transport.Send(ownNode, ownMsg)
				if err != nil {
					logging.Error(err.Error())
				}
			}(pssNode.NodeDetails, nextPSSMessage)
			defer func() { pss.State.Phase = States.Phases.Ended }()
		}
	} else if pssMessage.Method == "complete" {
		// parse message
		var pssMsgComplete PSSMsgComplete
		err := pssMsgComplete.FromBytes(pssMessage.Data)
		if err != nil {
			logging.Error(err.Error())
			return err
		}

		// state checks
		if senderDetails.ToNodeDetailsID() != pssNode.NodeDetails.ToNodeDetailsID() {
			return errors.New("This message can only be accepted if its sent to ourselves")
		}
		// if pss.State.Phase != States.Phases.Ended {
		// 	return errors.New("This pss should have ended for us to receive the complete message")
		// }

		// logic
		var pssIDDetails PSSIDDetails
		pssIDDetails.FromPSSID(pssMsgComplete.PSSID)
		sharingID := pssIDDetails.SharingID
		if _, found := pssNode.RecoverStore[sharingID]; !found {
			// no recover messages received
			return nil
		}
		recover := pssNode.RecoverStore[sharingID]

		// check if t+1 identical recover messages on same ID and same V received
		if recover.D == nil {
			logging.Info("Not enough recovers received")
			return nil
		}

		// add to psscompletecount if C00 is valid
		verified := pvss.VerifyShareCommitment(pssMsgComplete.C00, *recover.D, *big.NewInt(int64(pssIDDetails.Index)))
		if !verified {
			return errors.New("Could not verify share commitment in complete message for a threshold-agreed secret commitment")
		}
		recover.PSSCompleteCount[pssMsgComplete.PSSID] = true
		// check if t+1 sharings have completed
		if len(recover.PSSCompleteCount) == pssNode.NewNodes.T+1 {
			// propose Li via validated byzantine agreement
			var psss []PSSID
			for pssid := range recover.PSSCompleteCount {
				psss = append(psss, pssid) // make it deterministic
			}
			pssMsgPropose := PSSMsgPropose{
				NodeDetailsID: pssNode.NodeDetails.ToNodeDetailsID(),
				PSSs:          psss,
			}
			nextPSSMessage := PSSMessage{
				PSSID:  NullPSSID,
				Method: "propose",
				Data:   pssMsgPropose.ToBytes(),
			}
			go func(pssMessage PSSMessage) {
				err := pssNode.Transport.Broadcast(pssMessage)
				if err != nil {
					logging.Error(err.Error())
				}
			}(nextPSSMessage)
		}
	} else {
		return errors.New("PssMessage method not found")
	}
	return nil
}

// ProcessBroadcastMessage is called when the node receives a message via broadcast (eg. Tendermint)
func (pssNode *PSSNode) ProcessBroadcastMessage(senderDetails NodeDetails, pssMessage PSSMessage) error {
	return nil
}

// NewPSSNode creates a new pss node instance
func NewPSSNode(
	nodeDetails common.Node,
	oldNodeList []common.Node,
	oldNodesT int,
	oldNodesK int,
	newNodeList []common.Node,
	newNodesT int,
	newNodesK int,
	nodeIndex big.Int,
	transport PSSTransport,
) *PSSNode {
	mapFromNodeList := func(nodeList []common.Node) (res map[NodeDetailsID]NodeDetails) {
		res = make(map[NodeDetailsID]NodeDetails)
		for _, node := range nodeList {
			nodeDetails := NodeDetails(node)
			res[nodeDetails.ToNodeDetailsID()] = nodeDetails
		}
		return
	}
	newPssNode := &PSSNode{
		NodeDetails: NodeDetails(nodeDetails),
		OldNodes: NodeNetwork{
			Nodes: mapFromNodeList(oldNodeList),
			T:     oldNodesT,
			K:     oldNodesK,
		},
		NewNodes: NodeNetwork{
			Nodes: mapFromNodeList(newNodeList),
			T:     newNodesT,
			K:     newNodesK,
		},
		NodeIndex:    nodeIndex,
		ShareStore:   make(map[SharingID]*Sharing),
		RecoverStore: make(map[SharingID]*Recover),
		PSSStore:     make(map[PSSID]*PSS),
	}
	transport.SetPSSNode(newPssNode)
	newPssNode.Transport = transport
	return newPssNode
}
