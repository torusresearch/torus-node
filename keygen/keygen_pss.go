package keygen

import (
	"errors"
	"math/big"
	"strconv"

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

func (pssNode *PSSNode) ProcessMessage(senderDetails NodeDetails, pssMessage PSSMessage) error {
	pssNode.Lock()
	defer pssNode.Unlock()
	if _, found := pssNode.PSSStore[pssMessage.PSSID]; !found {
		pssNode.PSSStore[pssMessage.PSSID] = &PSS{
			PSSID: pssMessage.PSSID,
			State: PSSState{
				PSSTypes.Phases.Initial,
				PSSTypes.Dealer.IsDealer,
				PSSTypes.Player.IsPlayer,
				PSSTypes.ReceivedSend.False,
				PSSTypes.ReceivedEchoMap(),
				PSSTypes.ReceivedReadyMap(),
			},
			CStore: make(map[CID]*C),
		}
	}
	pss := pssNode.PSSStore[pssMessage.PSSID]
	pss.Lock()
	defer pss.Unlock()

	pss.Messages = append(pss.Messages, pssMessage)

	// handle different messages here
	if pssMessage.Method == "share" {
		// state checks
		if pss.State.Phase == PSSTypes.Phases.Ended {
			return errors.New("PSS has ended, ignored message " + string(*pssMessage.JSON()) + " from " + string(senderDetails.ToNodeDetailsID()) + " ")
		}
		if pss.State.Dealer != PSSTypes.Dealer.IsDealer {
			return errors.New("PSS could not be started since the node is not a dealer")
		}

		// logic
		var pssMsgShare PSSMsgShare
		err := pssMsgShare.FromBytes(pssMessage.Data)
		if err != nil {
			return errors.New("Could not unmarshal data for pssMsgShare")
		}
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
					pssNode.Transport.Output(err.Error())
				}
			}(newNode, nextPSSMessage)

			// TODO: uncomment below for PSS

			// nextNextPSSMessage := PSSMessage{
			// 	PSSID:  PSSID(""),
			// 	Method: "recover",
			// 	Data:   (&PSSMsgRecover{SharingID: pssMsgShare.SharingID}).ToBytes(),
			// }

			// go func(newN NodeDetails, msg PSSMessage) {
			// 	pssNode.Transport.Send(newN, msg)
			// }(newNode, nextNextPSSMessage)
		}
	} else if pssMessage.Method == "recover" {
		panic("RECOVER NOT IMPLEMENTED")
	} else if pssMessage.Method == "send" {
		// state checks
		if pss.State.Phase == PSSTypes.Phases.Ended {
			return errors.New("PSS has ended, ignored message " + string(*pssMessage.JSON()) + " from " + string(senderDetails.ToNodeDetailsID()) + " ")
		}
		if pss.State.Player != PSSTypes.Player.IsPlayer {
			return errors.New("Could not receive send message because node is not a player")
		}
		if pss.State.ReceivedSend == PSSTypes.ReceivedSend.True {
			return errors.New("Already received a send message for PSSID " + string(pss.PSSID))
		}

		// logic
		var pssMsgSend PSSMsgSend
		err := pssMsgSend.FromBytes(pssMessage.Data)
		if err != nil {
			return err
		}
		var pssIDDetails PSSIDDetails
		err = pssIDDetails.FromPSSID(pssMessage.PSSID)
		if err != nil {
			return err
		}
		senderID := pssNode.NewNodes.Nodes[senderDetails.ToNodeDetailsID()].Index
		if senderID == pssIDDetails.Index {
			pss.State.ReceivedSend = PSSTypes.ReceivedSend.True
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
					pssNode.Transport.Output(err.Error())
				}
			}(newNode, nextPSSMessage)
		}
	} else if pssMessage.Method == "echo" {
		// state checks
		if pss.State.Phase == PSSTypes.Phases.Ended {
			return errors.New("PSS has ended, ignored message " + string(*pssMessage.JSON()) + " from " + string(senderDetails.ToNodeDetailsID()) + " ")
		}
		receivedEcho, found := pss.State.ReceivedEcho[senderDetails.ToNodeDetailsID()]
		if found && receivedEcho == PSSTypes.ReceivedEcho.True {
			return errors.New("Already received a echo message for PSSID " + string(pss.PSSID) + "from sender " + string(senderDetails.ToNodeDetailsID()))
		}

		// logic
		pss.State.ReceivedEcho[senderDetails.ToNodeDetailsID()] = PSSTypes.ReceivedEcho.True
		var pssMsgEcho PSSMsgEcho
		err := pssMsgEcho.FromBytes(pssMessage.Data)
		if err != nil {
			return err
		}

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
		c, found := pss.CStore[cID]
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
		c = pss.CStore[cID]
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
						pssNode.Transport.Output(err.Error())
					}
				}(newNode, nextPSSMessage)
			}
		}
	} else if pssMessage.Method == "ready" {
		// state checks
		if pss.State.Phase == PSSTypes.Phases.Ended {
			return errors.New("PSS has ended, ignored message " + string(*pssMessage.JSON()) + " from " + string(senderDetails.ToNodeDetailsID()) + " ")
		}
		receivedReady, found := pss.State.ReceivedReady[senderDetails.ToNodeDetailsID()]
		if found && receivedReady == PSSTypes.ReceivedReady.True {
			return errors.New("Already received a ready message for PSSID " + string(pss.PSSID))
		}

		// logic
		pss.State.ReceivedReady[senderDetails.ToNodeDetailsID()] = PSSTypes.ReceivedReady.True
		var pssMsgReady PSSMsgReady
		err := pssMsgReady.FromBytes(pssMessage.Data)
		if err != nil {
			return err
		}
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
		c, found := pss.CStore[cID]
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
		c = pss.CStore[cID]
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
						pssNode.Transport.Output(err.Error())
					}
				}(newNode, nextPSSMessage)
			}
		} else if c.RC == pssNode.NewNodes.K+pssNode.NewNodes.T {
			pss.Cbar = c.C
			pss.Si = c.Abar[0]
			pss.Siprime = c.Abarprime[0]
			pssNode.Transport.Output(string(pss.PSSID) + " shared.")
		}
	} else {
		return errors.New("PssMessage method not found")
	}
	return nil
}

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
		NodeIndex:  nodeIndex,
		ShareStore: make(map[SharingID]*Sharing),
		PSSStore:   make(map[PSSID]*PSS),
	}
	transport.SetPSSNode(newPssNode)
	newPssNode.Transport = transport
	return newPssNode
}
