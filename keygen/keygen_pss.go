package keygen

import (
	"errors"
	"math/big"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/torusresearch/torus-public/secp256k1"

	"github.com/torusresearch/bijson"

	"github.com/torusresearch/torus-public/common"
	"github.com/torusresearch/torus-public/logging"
	"github.com/torusresearch/torus-public/pvss"
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
		err := bijson.Unmarshal(pssMessage.Data, &pssMsgShare)
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
			data, err := bijson.Marshal(pssMsgSend)
			if err != nil {
				return err
			}
			nextPSSMessage := PSSMessage{
				PSSID:  pssID,
				Method: "send",
				Data:   data,
			}
			go func(newN NodeDetails, msg PSSMessage) {
				// lock when send message through transport?
				pssNode.Lock()
				defer pssNode.Unlock()
				err := pssNode.Transport.Send(newN, msg)
				if err != nil {
					logging.Error(err.Error())
				}
			}(newNode, nextPSSMessage)

			data, err = bijson.Marshal(PSSMsgRecover{
				SharingID: pssMsgShare.SharingID,
				V:         sharing.C,
			})
			if err != nil {
				return err
			}
			nextNextPSSMessage := PSSMessage{
				PSSID:  NullPSSID,
				Method: "recover",
				Data:   data,
			}
			go func(newN NodeDetails, msg PSSMessage) {
				// lock when send message through transport?
				pssNode.Lock()
				defer pssNode.Unlock()
				err := pssNode.Transport.Send(newN, msg)
				if err != nil {
					logging.Error(err.Error())
				}
			}(newNode, nextNextPSSMessage)
		}
		defer func() { pss.State.Phase = States.Phases.Started }()
		return nil
	} else if pssMessage.Method == "recover" {
		// parse message
		var pssMsgRecover PSSMsgRecover
		err := bijson.Unmarshal(pssMessage.Data, &pssMsgRecover)
		if err != nil {
			logging.Error(err.Error())
			return err
		}
		// state checks

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
		if len(dCount) == pssNode.OldNodes.T+1 {
			recover.D = &pssMsgRecover.V
			// defer func() { pss.State.Recover = States.Recover.WaitingForSelectedSharingsComplete }()
		}
		return nil
	} else if pssMessage.Method == "send" {
		// parse message
		var pssMsgSend PSSMsgSend
		err := bijson.Unmarshal(pssMessage.Data, &pssMsgSend)
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
			data, err := bijson.Marshal(pssMsgEcho)
			if err != nil {
				return err
			}
			nextPSSMessage := PSSMessage{
				PSSID:  pss.PSSID,
				Method: "echo",
				Data:   data,
			}
			go func(newN NodeDetails, msg PSSMessage) {
				// lock when send message through transport?
				pssNode.Lock()
				defer pssNode.Unlock()
				err := pssNode.Transport.Send(newN, msg)
				if err != nil {
					logging.Info(err.Error())
				}
			}(newNode, nextPSSMessage)
		}
		return nil
	} else if pssMessage.Method == "echo" {
		// parse message
		defer func() { pss.State.ReceivedEcho[senderDetails.ToNodeDetailsID()] = States.ReceivedEcho.True }()
		var pssMsgEcho PSSMsgEcho
		err := bijson.Unmarshal(pssMessage.Data, &pssMsgEcho)
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
				CID:             cID,
				C:               pssMsgEcho.C,
				AC:              make(map[NodeDetailsID]common.Point),
				ACprime:         make(map[NodeDetailsID]common.Point),
				BC:              make(map[NodeDetailsID]common.Point),
				BCprime:         make(map[NodeDetailsID]common.Point),
				SignedTextStore: make(map[NodeDetailsID]SignedText),
			}
		}
		c := pss.CStore[cID]
		c.AC[senderDetails.ToNodeDetailsID()] = common.Point{
			X: *big.NewInt(int64(senderDetails.Index)),
			Y: pssMsgEcho.Alpha,
		}
		c.ACprime[senderDetails.ToNodeDetailsID()] = common.Point{
			X: *big.NewInt(int64(senderDetails.Index)),
			Y: pssMsgEcho.Alphaprime,
		}
		c.BC[senderDetails.ToNodeDetailsID()] = common.Point{
			X: *big.NewInt(int64(senderDetails.Index)),
			Y: pssMsgEcho.Beta,
		}
		c.BCprime[senderDetails.ToNodeDetailsID()] = common.Point{
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
				sigBytes, err := pssNode.Transport.Sign(string(pss.PSSID) + "|" + "ready")
				if err != nil {
					return err
				}
				signedText := SignedText(sigBytes)
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
					SignedText: signedText,
				}
				data, err := bijson.Marshal(pssMsgReady)
				if err != nil {
					return err
				}
				nextPSSMessage := PSSMessage{
					PSSID:  pss.PSSID,
					Method: "ready",
					Data:   data,
				}
				go func(newN NodeDetails, msg PSSMessage) {
					// lock when send message through transport?
					pssNode.Lock()
					defer pssNode.Unlock()
					err := pssNode.Transport.Send(newN, msg)
					if err != nil {
						logging.Info(err.Error())
					}
				}(newNode, nextPSSMessage)
			}
		}
		return nil
	} else if pssMessage.Method == "ready" {
		// parse message
		defer func() { pss.State.ReceivedReady[senderDetails.ToNodeDetailsID()] = States.ReceivedReady.True }()
		var pssMsgReady PSSMsgReady
		err := bijson.Unmarshal(pssMessage.Data, &pssMsgReady)
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
		sigValid := pvss.ECDSAVerify(string(pss.PSSID)+"|"+"ready", &senderDetails.PubKey, pssMsgReady.SignedText)
		if !sigValid {
			return errors.New("Could not verify signature on message: " + string(pss.PSSID) + "|" + "ready")
		}

		cID := GetCIDFromPointMatrix(pssMsgReady.C)
		_, found = pss.CStore[cID]
		if !found {
			pss.CStore[cID] = &C{
				CID:             cID,
				C:               pssMsgReady.C,
				AC:              make(map[NodeDetailsID]common.Point),
				ACprime:         make(map[NodeDetailsID]common.Point),
				BC:              make(map[NodeDetailsID]common.Point),
				BCprime:         make(map[NodeDetailsID]common.Point),
				SignedTextStore: make(map[NodeDetailsID]SignedText),
			}
		}
		c := pss.CStore[cID]
		c.AC[senderDetails.ToNodeDetailsID()] = common.Point{
			X: *big.NewInt(int64(senderDetails.Index)),
			Y: pssMsgReady.Alpha,
		}
		c.ACprime[senderDetails.ToNodeDetailsID()] = common.Point{
			X: *big.NewInt(int64(senderDetails.Index)),
			Y: pssMsgReady.Alphaprime,
		}
		c.BC[senderDetails.ToNodeDetailsID()] = common.Point{
			X: *big.NewInt(int64(senderDetails.Index)),
			Y: pssMsgReady.Beta,
		}
		c.BCprime[senderDetails.ToNodeDetailsID()] = common.Point{
			X: *big.NewInt(int64(senderDetails.Index)),
			Y: pssMsgReady.Betaprime,
		}
		c.SignedTextStore[senderDetails.ToNodeDetailsID()] = pssMsgReady.SignedText
		c.RC = c.RC + 1
		if c.RC == pssNode.NewNodes.K &&
			c.EC < ecThreshold(pssNode.NewNodes.N, pssNode.NewNodes.K, pssNode.NewNodes.T) {
			c.Abar = pvss.LagrangeInterpolatePolynomial(GetPointArrayFromMap(c.AC))
			c.Abarprime = pvss.LagrangeInterpolatePolynomial(GetPointArrayFromMap(c.ACprime))
			c.Bbar = pvss.LagrangeInterpolatePolynomial(GetPointArrayFromMap(c.BC))
			c.Bbarprime = pvss.LagrangeInterpolatePolynomial(GetPointArrayFromMap(c.BCprime))
			for _, newNode := range pssNode.NewNodes.Nodes {
				sigBytes, err := pssNode.Transport.Sign(string(pss.PSSID) + "|" + "ready")
				if err != nil {
					return err
				}
				signedText := SignedText(sigBytes)
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
					SignedText: signedText,
				}
				data, err := bijson.Marshal(pssMsgReady)
				if err != nil {
					return err
				}
				nextPSSMessage := PSSMessage{
					PSSID:  pss.PSSID,
					Method: "ready",
					Data:   data,
				}
				go func(newN NodeDetails, msg PSSMessage) {
					// lock when send message through transport?
					pssNode.Lock()
					defer pssNode.Unlock()
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
				// lock when send message through transport?
				pssNode.Lock()
				defer pssNode.Unlock()
				pssNode.Transport.Output(msg + " shared.")
			}(string(pss.PSSID))
			data, err := bijson.Marshal(PSSMsgComplete{
				PSSID: pss.PSSID,
				C00:   pss.Cbar[0][0],
			})
			if err != nil {
				return err
			}
			nextPSSMessage := PSSMessage{
				PSSID:  NullPSSID,
				Method: "complete",
				Data:   data,
			}
			go func(ownNode NodeDetails, ownMsg PSSMessage) {
				// lock when send message through transport?
				pssNode.Lock()
				defer pssNode.Unlock()
				err := pssNode.Transport.Send(ownNode, ownMsg)
				if err != nil {
					logging.Error(err.Error())
				}
			}(pssNode.NodeDetails, nextPSSMessage)
			defer func() { pss.State.Phase = States.Phases.Ended }()
		}
		return nil
	} else if pssMessage.Method == "complete" {
		// parse message
		var pssMsgComplete PSSMsgComplete
		err := bijson.Unmarshal(pssMessage.Data, &pssMsgComplete)
		if err != nil {
			logging.Error(err.Error())
			return err
		}

		// state checks
		if senderDetails.ToNodeDetailsID() != pssNode.NodeDetails.ToNodeDetailsID() {
			return errors.New("This message can only be accepted if its sent to ourselves")
		}

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
		// check if k sharings have completed
		if len(recover.PSSCompleteCount) == pssNode.NewNodes.K {
			// propose Li via validated byzantine agreement
			var psss []PSSID
			var signedTexts []map[NodeDetailsID]SignedText
			for pssid := range recover.PSSCompleteCount {
				psss = append(psss, pssid)
			}
			sort.Slice(psss, func(i, j int) bool {
				return strings.Compare(string(psss[i]), string(psss[j])) < 0
			})

			for _, pssid := range psss {
				pss := pssNode.PSSStore[pssid]
				if pss == nil {
					return errors.New("Could not get completed pss")
				}
				cbar := pss.Cbar
				if len(cbar) == 0 {
					return errors.New("Could not get completed cbar")
				}
				cid := GetCIDFromPointMatrix(cbar)
				c := pss.CStore[cid]
				if c == nil {
					return errors.New("Could not get completed c")
				}
				signedTexts = append(signedTexts, c.SignedTextStore)
			}

			pssMsgPropose := PSSMsgPropose{
				NodeDetailsID: pssNode.NodeDetails.ToNodeDetailsID(),
				SharingID:     sharingID,
				PSSs:          psss,
				SignedTexts:   signedTexts,
			}
			data, err := bijson.Marshal(pssMsgPropose)
			if err != nil {
				return err
			}
			nextPSSMessage := PSSMessage{
				PSSID:  NullPSSID,
				Method: "propose",
				Data:   data,
			}
			go func(pssMessage PSSMessage) {
				// lock when send message through transport?
				pssNode.Lock()
				defer pssNode.Unlock()
				err := pssNode.Transport.SendBroadcast(pssMessage)
				if err != nil {
					logging.Error(err.Error())
				}
			}(nextPSSMessage)
		}
		return nil
	}
	return errors.New("PssMessage method '" + pssMessage.Method + "' not found")
}

// ProcessBroadcastMessage is called when the node receives a message via broadcast (eg. Tendermint)
func (pssNode *PSSNode) ProcessBroadcastMessage(pssMessage PSSMessage) error {
	pssNode.Lock()
	defer pssNode.Unlock()
	if pssMessage.Method == "decide" {
		// if untrusted, request for a proof that the decided message was included in a block

		// parse message
		var pssMsgDecide PSSMsgDecide
		err := bijson.Unmarshal(pssMessage.Data, &pssMsgDecide)
		if err != nil {
			return err
		}

		// wait for all sharings in decided set to complete
		firstEntry := true
		for {
			if !firstEntry {
				pssNode.Unlock()
				time.Sleep(1 * time.Second)
				pssNode.Lock()
			} else {
				firstEntry = false
			}
			for _, pssid := range pssMsgDecide.PSSs {
				if pssNode.PSSStore[pssid] == nil {
					logging.Info("Waiting for pssid " + string(pssid) + " to complete. Still uninitialized.")
					continue
				}
				pss := pssNode.PSSStore[pssid]
				if pss.State.Phase != States.Phases.Ended {
					logging.Info("Waiting for pssid " + string(pssid) + " to complete. Still at " + string(pss.State.Phase))
					continue
				}
			}
			break
		}

		if _, found := pssNode.RecoverStore[pssMsgDecide.SharingID]; !found {
			return errors.New("Sharings for sharingID " + string(pssMsgDecide.SharingID) + " have not completed yet.")
		}
		recover := pssNode.RecoverStore[pssMsgDecide.SharingID]
		var abarArray []common.Point
		var abarprimeArray []common.Point
		for _, pssid := range pssMsgDecide.PSSs {
			pss := pssNode.PSSStore[pssid]
			if pss == nil {
				return errors.New("Could not get pss reference")
			}
			var pssIDDetails PSSIDDetails
			err := pssIDDetails.FromPSSID(pssid)
			if err != nil {
				return err
			}
			abarArray = append(abarArray, common.Point{
				X: *big.NewInt(int64(pssIDDetails.Index)),
				Y: pss.Si,
			})
			abarprimeArray = append(abarprimeArray, common.Point{
				X: *big.NewInt(int64(pssIDDetails.Index)),
				Y: pss.Siprime,
			})
		}
		abar := pvss.LagrangeScalarCP(abarArray, 0)
		abarprime := pvss.LagrangeScalarCP(abarprimeArray, 0)
		recover.Si = *abar
		recover.Siprime = *abarprime
		var vbarInputPts [][]common.Point
		var vbarInputIndexes []int
		for _, pssid := range pssMsgDecide.PSSs {
			pss := pssNode.PSSStore[pssid]
			var pssIDDetails PSSIDDetails
			err := pssIDDetails.FromPSSID(pssid)
			if err != nil {
				return err
			}
			vbarInputIndexes = append(vbarInputIndexes, pssIDDetails.Index)
			vbarInputPts = append(vbarInputPts, common.GetColumnPoint(pss.Cbar, 0))
		}
		recover.Vbar = pvss.LagrangePolys(vbarInputIndexes, vbarInputPts)
		gsi := common.BigIntToPoint(secp256k1.Curve.ScalarBaseMult(recover.Si.Bytes()))
		hsiprime := common.BigIntToPoint(secp256k1.Curve.ScalarMult(&secp256k1.H.X, &secp256k1.H.Y, recover.Siprime.Bytes()))
		gsihsiprime := common.BigIntToPoint(secp256k1.Curve.Add(&gsi.X, &gsi.Y, &hsiprime.X, &hsiprime.Y))
		verified := pvss.VerifyShareCommitment(gsihsiprime, recover.Vbar, *big.NewInt(int64(pssNode.NodeDetails.Index)))
		if !verified {
			return errors.New("Could not verify shares against interpolated commitments")
		}
		go func(msg string) {
			// lock when send message through transport?
			pssNode.Lock()
			defer pssNode.Unlock()
			pssNode.Transport.Output(msg + " refreshed")
		}(string(pssMsgDecide.SharingID))
		return nil
	}

	return errors.New("PssMessage method '" + pssMessage.Method + "' not found")
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
