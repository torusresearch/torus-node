package pss

import (
	"errors"
	"fmt"
	"math"
	"math/big"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/torusresearch/torus-node/telemetry"

	logging "github.com/sirupsen/logrus"
	"github.com/torusresearch/bijson"
	"github.com/torusresearch/torus-common/common"
	"github.com/torusresearch/torus-common/secp256k1"
	pcmn "github.com/torusresearch/torus-node/common"
	"github.com/torusresearch/torus-node/pvss"
)

// max(roundUp((n+t+1)/2), k)
func ecThreshold(n, k, t int) (res int) {
	nkt1half := int(math.Ceil((float64(n) + float64(t) + 1.0) / 2.0))
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
// We assume that senderDetails have already been validated by the transport and are always correct
func (pssNode *PSSNode) ProcessMessage(senderDetails NodeDetails, pssMessage PSSMessage) error {
	logging.WithFields(logging.Fields{
		"NodeDetails":      string(pssNode.NodeDetails.ToNodeDetailsID())[0:8],
		"senderDetails":    string(senderDetails.ToNodeDetailsID())[0:8],
		"pssMessageMethod": pssMessage.Method,
	}).Debug("pssNode processing message for pssMessageMethod")

	// Add to metrics
	telemetry.IncrementCounter(pcmn.TelemetryConstants.PSS.ProcessedMessageCounter, pcmn.TelemetryConstants.PSS.Prefix)

	var dealer dealerState
	var player playerState
	if pssNode.IsDealer {
		dealer = States.Dealer.IsDealer
	} else {
		dealer = States.Dealer.NotDealer
	}
	if pssNode.IsPlayer {
		player = States.Player.IsPlayer
	} else {
		player = States.Player.NotPlayer
	}
	pss, complete := pssNode.PSSStore.GetOrSetIfNotComplete(pssMessage.PSSID, &PSS{
		PSSID: pssMessage.PSSID,
		State: PSSState{
			States.Phases.Initial,
			dealer,
			player,
			// States.Recover.Initial,
			States.ReceivedSend.False,
			States.ReceivedEchoMap(),
			States.ReceivedReadyMap(),
		},
		CStore: make(map[CID]*C),
	})
	if complete {
		logging.WithField("pssid", pssMessage.PSSID).Debug("already cleaned up, ignoring message pss")
		return nil
	}

	pss.Lock()
	defer pss.Unlock()

	// handle different messages here
	if pssMessage.Method == "share" {
		// parse message
		var pssMsgShare PSSMsgShare
		err := bijson.Unmarshal(pssMessage.Data, &pssMsgShare)
		if err != nil {
			logging.WithField("method", "share").WithError(err).Error()
			return err
		}

		// state checks
		if pss.State.Phase == States.Phases.Ended {

			// Add to metrics
			telemetry.IncrementCounter(pcmn.TelemetryConstants.PSS.IgnoredMessageCounter, pcmn.TelemetryConstants.PSS.Prefix)

			return errors.New("PSS has ended, ignored message " + pssMessage.Method + " from " + string(senderDetails.ToNodeDetailsID())[0:8] + " ")
		}
		if pss.State.Dealer != States.Dealer.IsDealer {

			// Add to metrics
			telemetry.IncrementCounter(pcmn.TelemetryConstants.PSS.NotDealerCounter, pcmn.TelemetryConstants.PSS.Prefix)

			return errors.New("PSS could not be started since the node is not a dealer")
		}

		// logic
		sharing := pssNode.DataSource.GetSharing(pssMsgShare.SharingID.GetKeygenID())
		if sharing == nil {
			return fmt.Errorf("could not find sharing for sharingID %v", pssMsgShare.SharingID)
		}
		sharing.Lock()
		defer sharing.Unlock()
		pss.F = pvss.GenerateRandomBivariatePolynomial(sharing.Si, pssNode.NewNodes.K)
		pss.Fprime = pvss.GenerateRandomBivariatePolynomial(sharing.Siprime, pssNode.NewNodes.K)
		pss.C = pvss.GetCommitmentMatrix(pss.F, pss.Fprime)

		for _, newNode := range pssNode.NewNodes.Nodes {
			pssID := (&PSSIDDetails{
				SharingID:   pssMsgShare.SharingID,
				DealerIndex: sharing.I,
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
			nextPSSMessage := CreatePSSMessage(PSSMessageRaw{
				PSSID:  pssID,
				Method: "send",
				Data:   data,
			})
			go func(newN NodeDetails, msg PSSMessage) {
				err := pssNode.Transport.Send(newN, msg)
				if err != nil {
					logging.WithError(err).Error("could not send pssMsgSend")
				}
			}(newNode, nextPSSMessage)

			data, err = bijson.Marshal(PSSMsgRecover{
				SharingID: pssMsgShare.SharingID,
				V:         sharing.C,
			})
			if err != nil {
				return err
			}
			nextNextPSSMessage := CreatePSSMessage(PSSMessageRaw{
				PSSID:  NullPSSID,
				Method: "recover",
				Data:   data,
			})
			go func(newN NodeDetails, msg PSSMessage) {
				err := pssNode.Transport.Send(newN, msg)
				if err != nil {
					logging.WithError(err).Error("could not send pssMsgRecover")
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
			logging.WithField("method", "recover").WithError(err).Error()
			return err
		}
		// state checks

		// logic
		if len(pssMsgRecover.V) == 0 {
			err := errors.New("Recover message commitment poly is of length 0")
			logging.WithError(err).Error("incorrect recover message V length")
			return err
		}

		recover, complete := pssNode.RecoverStore.GetOrSetIfNotComplete(pssMsgRecover.SharingID, &Recover{
			SharingID:        pssMsgRecover.SharingID,
			DCount:           make(map[VID]map[NodeDetailsID]bool),
			PSSCompleteCount: make(map[PSSID]bool),
		})
		if complete {
			logging.WithField("sharingid", pssMsgRecover.SharingID).Debug("already completed sharing in recover message, ignoring message")
			return nil
		}
		recover.Lock()
		defer recover.Unlock()
		vID := GetVIDFromPointArray(pssMsgRecover.V)
		if recover.DCount[vID] == nil {
			recover.DCount[vID] = make(map[NodeDetailsID]bool)
		}
		dCount := recover.DCount[vID]
		dCount[senderDetails.ToNodeDetailsID()] = true
		if len(dCount) == pssNode.OldNodes.K {
			recover.D = &pssMsgRecover.V
			logging.WithField("commitmentPoly", pssMsgRecover.V).Debug("received k recover messages with the same commitment")
		}
		return nil
	} else if pssMessage.Method == "send" {
		// parse message
		var pssMsgSend PSSMsgSend
		err := bijson.Unmarshal(pssMessage.Data, &pssMsgSend)
		if err != nil {
			logging.WithField("method", "send").WithError(err).Error()
			return err
		}
		// state checks
		if pss.State.Phase == States.Phases.Ended {

			// Add to metrics
			telemetry.IncrementCounter(pcmn.TelemetryConstants.PSS.IgnoredMessageCounter, pcmn.TelemetryConstants.PSS.Prefix)

			return errors.New("PSS has ended, ignored message " + pssMessage.Method + " from " + string(senderDetails.ToNodeDetailsID())[0:8] + " ")
		}
		if pss.State.Player != States.Player.IsPlayer {

			// Add to metrics
			telemetry.IncrementCounter(pcmn.TelemetryConstants.PSS.NotPlayerCounter, pcmn.TelemetryConstants.PSS.Prefix)

			return errors.New("could not receive send message because node is not a player")
		}
		if pss.State.ReceivedSend == States.ReceivedSend.True {
			return errors.New("Already received a send message for PSSID " + string(pss.PSSID))
		}

		// logic
		var pssIDDetails PSSIDDetails
		err = pssIDDetails.FromPSSID(pssMessage.PSSID)
		if err != nil {
			logging.WithField("PSSID", pssMessage.PSSID).WithError(err).Error("could not get pssIDDetails from PSSID")
			return err
		}
		senderID := pssNode.OldNodes.Nodes[senderDetails.ToNodeDetailsID()].Index
		if senderID == pssIDDetails.DealerIndex {
			defer func() { pss.State.ReceivedSend = States.ReceivedSend.True }()
		} else {
			return errors.New("'Send' message contains index of " + strconv.Itoa(pssIDDetails.DealerIndex) + " was not sent by node " + strconv.Itoa(senderID))
		}
		verified := pvss.AVSSVerifyPoly(
			pssMsgSend.C,
			*big.NewInt(int64(pssNode.NodeDetails.Index)),
			pcmn.PrimaryPolynomial{Coeff: pssMsgSend.A, Threshold: pssNode.NewNodes.K},
			pcmn.PrimaryPolynomial{Coeff: pssMsgSend.Aprime, Threshold: pssNode.NewNodes.K},
			pcmn.PrimaryPolynomial{Coeff: pssMsgSend.B, Threshold: pssNode.NewNodes.K},
			pcmn.PrimaryPolynomial{Coeff: pssMsgSend.Bprime, Threshold: pssNode.NewNodes.K},
		)
		if !verified {
			return errors.New("Could not verify polys against commitment")
		}
		logging.WithFields(logging.Fields{
			"NodeDetails":   string(pssNode.NodeDetails.ToNodeDetailsID())[0:8],
			"senderDetails": string(senderDetails.ToNodeDetailsID())[0:8],
		}).Debug("node verified send message from sender, sending echo message")
		for _, newNode := range pssNode.NewNodes.Nodes {

			// Add to metrics
			telemetry.IncrementCounter(pcmn.TelemetryConstants.PSS.SendingEchoCounter, pcmn.TelemetryConstants.PSS.Prefix)

			pssMsgEcho := PSSMsgEcho{
				PSSID: pssMsgSend.PSSID,
				C:     pssMsgSend.C,
				Alpha: *pvss.PolyEval(
					pcmn.PrimaryPolynomial{Coeff: pssMsgSend.A, Threshold: pssNode.NewNodes.K},
					*big.NewInt(int64(newNode.Index)),
				),
				Alphaprime: *pvss.PolyEval(
					pcmn.PrimaryPolynomial{Coeff: pssMsgSend.Aprime, Threshold: pssNode.NewNodes.K},
					*big.NewInt(int64(newNode.Index)),
				),
				Beta: *pvss.PolyEval(
					pcmn.PrimaryPolynomial{Coeff: pssMsgSend.B, Threshold: pssNode.NewNodes.K},
					*big.NewInt(int64(newNode.Index)),
				),
				Betaprime: *pvss.PolyEval(
					pcmn.PrimaryPolynomial{Coeff: pssMsgSend.Bprime, Threshold: pssNode.NewNodes.K},
					*big.NewInt(int64(newNode.Index)),
				),
			}
			data, err := bijson.Marshal(pssMsgEcho)
			if err != nil {
				return err
			}
			nextPSSMessage := CreatePSSMessage(PSSMessageRaw{
				PSSID:  pss.PSSID,
				Method: "echo",
				Data:   data,
			})
			go func(newN NodeDetails, msg PSSMessage) {
				err := pssNode.Transport.Send(newN, msg)
				if err != nil {
					logging.WithError(err).Info()
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
			logging.WithField("method", "echo").WithError(err).Error()
			return err
		}

		// state checks
		if pss.State.Phase == States.Phases.Ended {

			// Add to metrics
			telemetry.IncrementCounter(pcmn.TelemetryConstants.PSS.IgnoredMessageCounter, pcmn.TelemetryConstants.PSS.Prefix)

			return errors.New("PSS has ended, ignored message " + pssMessage.Method + " from " + string(senderDetails.ToNodeDetailsID())[0:8] + " ")
		}
		if pss.State.Player != States.Player.IsPlayer {

			// Add to metrics
			telemetry.IncrementCounter(pcmn.TelemetryConstants.PSS.NotPlayerCounter, pcmn.TelemetryConstants.PSS.Prefix)

			return errors.New("could not receive echo message because node is not a player")
		}
		receivedEcho, found := pss.State.ReceivedEcho[senderDetails.ToNodeDetailsID()]
		if found && receivedEcho == States.ReceivedEcho.True {
			return errors.New("Already received a echo message for PSSID " + string(pss.PSSID) + "from sender " + string(senderDetails.ToNodeDetailsID())[0:8])
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

			// Add to metrics
			telemetry.IncrementCounter(pcmn.TelemetryConstants.PSS.InvalidEchoCounter, pcmn.TelemetryConstants.PSS.Prefix)

			return fmt.Errorf("Could not verify point against commitments for echo message for sender %v", senderDetails)
		}
		logging.WithFields(logging.Fields{
			"NodeDetails":   string(pssNode.NodeDetails.ToNodeDetailsID())[0:8],
			"senderDetails": string(senderDetails.ToNodeDetailsID())[0:8],
		}).Debug("node verified echo message from sender")
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

			// Add to metrics
			telemetry.IncrementCounter(pcmn.TelemetryConstants.PSS.SendingReadyCounter, pcmn.TelemetryConstants.PSS.Prefix)

			logging.WithFields(logging.Fields{
				"NodeDetails":   string(pssNode.NodeDetails.ToNodeDetailsID())[0:8],
				"senderDetails": string(senderDetails.ToNodeDetailsID())[0:8],
			}).Debug("node verified echo message from sender and sending ready message")
			// Note: Despite the name mismatch below, this is correct, and the AVSS spec is wrong.
			c.Abar = pvss.LagrangeInterpolatePolynomial(GetPointArrayFromMap(c.BC))
			c.Abarprime = pvss.LagrangeInterpolatePolynomial(GetPointArrayFromMap(c.BCprime))
			c.Bbar = pvss.LagrangeInterpolatePolynomial(GetPointArrayFromMap(c.AC))
			c.Bbarprime = pvss.LagrangeInterpolatePolynomial(GetPointArrayFromMap(c.ACprime))
			for _, newNode := range pssNode.NewNodes.Nodes {
				sigBytes, err := pssNode.Transport.Sign([]byte(strings.Join([]string{string(pss.PSSID), "ready"}, pcmn.Delimiter1)))
				if err != nil {
					return err
				}
				signedText := SignedText(sigBytes)
				pssMsgReady := PSSMsgReady{
					PSSID: pss.PSSID,
					C:     c.C,
					Alpha: *pvss.PolyEval(
						pcmn.PrimaryPolynomial{Coeff: c.Abar, Threshold: pssNode.NewNodes.K},
						*big.NewInt(int64(newNode.Index)),
					),
					Alphaprime: *pvss.PolyEval(
						pcmn.PrimaryPolynomial{Coeff: c.Abarprime, Threshold: pssNode.NewNodes.K},
						*big.NewInt(int64(newNode.Index)),
					),
					Beta: *pvss.PolyEval(
						pcmn.PrimaryPolynomial{Coeff: c.Bbar, Threshold: pssNode.NewNodes.K},
						*big.NewInt(int64(newNode.Index)),
					),
					Betaprime: *pvss.PolyEval(
						pcmn.PrimaryPolynomial{Coeff: c.Bbarprime, Threshold: pssNode.NewNodes.K},
						*big.NewInt(int64(newNode.Index)),
					),
					SignedText: signedText,
				}
				data, err := bijson.Marshal(pssMsgReady)
				if err != nil {
					return err
				}
				nextPSSMessage := CreatePSSMessage(PSSMessageRaw{
					PSSID:  pss.PSSID,
					Method: "ready",
					Data:   data,
				})
				go func(newN NodeDetails, msg PSSMessage) {
					err := pssNode.Transport.Send(newN, msg)
					if err != nil {
						logging.WithError(err).Info()
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
			logging.WithField("method", "ready").WithError(err).Error()
			return err
		}
		var pssIDDetails PSSIDDetails
		err = pssIDDetails.FromPSSID(pssMsgReady.PSSID)
		if err != nil {
			logging.WithField("PSSID", pssMsgReady.PSSID).WithError(err).Error("could not get pssIDDetails from pssMsgReady PSSID")
			return err
		}

		// state checks
		if pss.State.Phase == States.Phases.Ended {

			// Add to metrics
			telemetry.IncrementCounter(pcmn.TelemetryConstants.PSS.IgnoredMessageCounter, pcmn.TelemetryConstants.PSS.Prefix)

			return errors.New("PSS has ended, ignored message " + pssMessage.Method + " from " + string(senderDetails.ToNodeDetailsID())[0:8] + " ")
		}
		if pss.State.Player != States.Player.IsPlayer {

			// Add to metrics
			telemetry.IncrementCounter(pcmn.TelemetryConstants.PSS.NotPlayerCounter, pcmn.TelemetryConstants.PSS.Prefix)

			return errors.New("could not receive ready message because node is not a player")
		}
		receivedReady, found := pss.State.ReceivedReady[senderDetails.ToNodeDetailsID()]
		if found && receivedReady == States.ReceivedReady.True {

			// Add to metrics
			telemetry.IncrementCounter(pcmn.TelemetryConstants.PSS.AlreadyReceivedReadyCounter, pcmn.TelemetryConstants.PSS.Prefix)

			return errors.New("already received a ready message for PSSID " + string(pss.PSSID))
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

			// Add to metrics
			telemetry.IncrementCounter(pcmn.TelemetryConstants.PSS.InvalidReadyCounter, pcmn.TelemetryConstants.PSS.Prefix)

			return fmt.Errorf("Could not verify point against commitments for ready message for sender %v, msg %v, senderDetailsIndex %v, pssNodeIndex %v", senderDetails, stringify(pssMsgReady), senderDetails.Index, pssNode.NodeDetails.Index)
		}
		sigValid := pvss.ECDSAVerify(strings.Join([]string{string(pss.PSSID), "ready"}, pcmn.Delimiter1), &senderDetails.PubKey, pssMsgReady.SignedText)
		if !sigValid {

			// Add to metrics
			telemetry.IncrementCounter(pcmn.TelemetryConstants.PSS.ReadySigInvalidCounter, pcmn.TelemetryConstants.PSS.Prefix)

			return errors.New("Could not verify signature on message: " + strings.Join([]string{string(pss.PSSID), "ready"}, pcmn.Delimiter1))
		}
		logging.WithFields(logging.Fields{
			"NodeDetails":   string(pssNode.NodeDetails.ToNodeDetailsID())[0:8],
			"senderDetails": string(senderDetails.ToNodeDetailsID())[0:8],
		}).Debug("node verified ready message from sender")

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

			// Add to metrics
			telemetry.IncrementCounter(pcmn.TelemetryConstants.PSS.SendingReadyCounter, pcmn.TelemetryConstants.PSS.Prefix)
			telemetry.IncrementCounter(pcmn.TelemetryConstants.PSS.ReadyBeforeEchoCounter, pcmn.TelemetryConstants.PSS.Prefix)

			logging.WithFields(logging.Fields{
				"NodeDetails":   string(pssNode.NodeDetails.ToNodeDetailsID())[0:8],
				"senderDetails": string(senderDetails.ToNodeDetailsID())[0:8],
			}).Debug("node verified ready message from sender and sending ready message")
			// Note: Despite the name mismatch below, this is correct, and the AVSS spec is wrong.
			c.Abar = pvss.LagrangeInterpolatePolynomial(GetPointArrayFromMap(c.BC))
			c.Abarprime = pvss.LagrangeInterpolatePolynomial(GetPointArrayFromMap(c.BCprime))
			c.Bbar = pvss.LagrangeInterpolatePolynomial(GetPointArrayFromMap(c.AC))
			c.Bbarprime = pvss.LagrangeInterpolatePolynomial(GetPointArrayFromMap(c.ACprime))
			for _, newNode := range pssNode.NewNodes.Nodes {
				sigBytes, err := pssNode.Transport.Sign([]byte(strings.Join([]string{string(pss.PSSID), "ready"}, pcmn.Delimiter1)))
				if err != nil {
					return err
				}
				signedText := SignedText(sigBytes)
				pssMsgReady := PSSMsgReady{
					PSSID: pss.PSSID,
					C:     c.C,
					Alpha: *pvss.PolyEval(
						pcmn.PrimaryPolynomial{Coeff: c.Abar, Threshold: pssNode.NewNodes.K},
						*big.NewInt(int64(newNode.Index)),
					),
					Alphaprime: *pvss.PolyEval(
						pcmn.PrimaryPolynomial{Coeff: c.Abarprime, Threshold: pssNode.NewNodes.K},
						*big.NewInt(int64(newNode.Index)),
					),
					Beta: *pvss.PolyEval(
						pcmn.PrimaryPolynomial{Coeff: c.Bbar, Threshold: pssNode.NewNodes.K},
						*big.NewInt(int64(newNode.Index)),
					),
					Betaprime: *pvss.PolyEval(
						pcmn.PrimaryPolynomial{Coeff: c.Bbarprime, Threshold: pssNode.NewNodes.K},
						*big.NewInt(int64(newNode.Index)),
					),
					SignedText: signedText,
				}
				data, err := bijson.Marshal(pssMsgReady)
				if err != nil {
					return err
				}
				nextPSSMessage := CreatePSSMessage(PSSMessageRaw{
					PSSID:  pss.PSSID,
					Method: "ready",
					Data:   data,
				})
				go func(newN NodeDetails, msg PSSMessage) {
					err := pssNode.Transport.Send(newN, msg)
					if err != nil {
						logging.WithError(err).Error("could not send pssMsgReady")
					}
				}(newNode, nextPSSMessage)
			}
		} else if c.RC == pssNode.NewNodes.K+pssNode.NewNodes.T {

			// Add to metrics
			telemetry.IncrementCounter(pcmn.TelemetryConstants.PSS.SendingCompleteCounter, pcmn.TelemetryConstants.PSS.Prefix)

			logging.WithFields(logging.Fields{
				"NodeDetails":   string(pssNode.NodeDetails.ToNodeDetailsID())[0:8],
				"senderDetails": string(senderDetails.ToNodeDetailsID())[0:8],
			}).Debug("node verified ready message from sender and sending complete message")
			pss.Cbar = c.C
			pss.Si = c.Abar[0]
			pss.Siprime = c.Abarprime[0]
			go func(msg string) {
				pssNode.Transport.Output(msg + " shared.")
			}(string(pss.PSSID))
			data, err := bijson.Marshal(PSSMsgComplete{
				PSSID: pssMsgReady.PSSID,
				C00:   pss.Cbar[0][0],
			})
			if err != nil {
				return err
			}
			nextPSSMessage := CreatePSSMessage(PSSMessageRaw{
				PSSID:  NullPSSID,
				Method: "complete",
				Data:   data,
			})
			go func(ownNode NodeDetails, ownMsg PSSMessage) {
				err := pssNode.Transport.Send(ownNode, ownMsg)
				if err != nil {
					logging.WithError(err).Error("could not send pssMsgComplete in ready")
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
			logging.WithField("method", "complete").WithError(err).Error("could not unmarshal pssmsgcomplete")
			return err
		}

		// state checks
		if senderDetails.ToNodeDetailsID() != pssNode.NodeDetails.ToNodeDetailsID() {
			return errors.New("This message can only be accepted if its sent to ourselves")
		}

		// logic
		var pssIDDetails PSSIDDetails
		err = pssIDDetails.FromPSSID(pssMsgComplete.PSSID)
		if err != nil {
			logging.WithField("pssid", pssMsgComplete.PSSID).WithError(err).Error("could not get pssiddetails")
			return err
		}
		var recover *Recover
		var alreadyCleanUp, sufficientRecovers bool
		for {
			recover, alreadyCleanUp, sufficientRecovers = checkSufficientRecoversIfNotComplete(pssNode, pssIDDetails.SharingID)
			if alreadyCleanUp {
				logging.WithField("sharingID", pssIDDetails.SharingID).Debug("pss already complete and clean up, ignoring message in complete")
				return nil
			}
			if sufficientRecovers {
				break
			}

			// Add to metrics
			telemetry.IncrementCounter(pcmn.TelemetryConstants.PSS.InsufficientRecoversCounter, pcmn.TelemetryConstants.PSS.Prefix)

			pss.Unlock()
			time.Sleep(1 * time.Second)
			pss.Lock()
		}
		recover.Lock()
		defer recover.Unlock()

		logging.WithField("sharingid", pssIDDetails.SharingID).Debug("finally received enough recovers, continuing for pss complete")

		// add to psscompletecount if C00 is valid
		verified := pvss.VerifyShareCommitment(pssMsgComplete.C00, *recover.D, *big.NewInt(int64(pssIDDetails.DealerIndex)))
		if !verified {

			// Add to metrics
			telemetry.IncrementCounter(pcmn.TelemetryConstants.PSS.InvalidShareCommitmentCounter, pcmn.TelemetryConstants.PSS.Prefix)

			return errors.New("could not verify share commitment in complete message for a threshold-agreed secret commitment")
		}
		recover.PSSCompleteCount[pssMsgComplete.PSSID] = true
		// check if k sharings have completed
		if len(recover.PSSCompleteCount) == pssNode.OldNodes.K {

			// Add to metrics
			telemetry.IncrementCounter(pcmn.TelemetryConstants.PSS.SendingProposalCounter, pcmn.TelemetryConstants.PSS.Prefix)

			logging.WithField("NodeDetails", string(pssNode.NodeDetails.ToNodeDetailsID())[0:8]).Debug("node reached k complete sharings, proposing a set...")
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
				pss, _ := pssNode.PSSStore.Get(pssid)
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
				NodeDetailsID: NullNodeDetails,
				SharingID:     pssIDDetails.SharingID,
				PSSs:          psss,
				SignedTexts:   signedTexts,
			}
			data, err := bijson.Marshal(pssMsgPropose)
			if err != nil {
				return err
			}
			nextPSSMessage := CreatePSSMessage(PSSMessageRaw{
				PSSID:  NullPSSID,
				Method: "propose",
				Data:   data,
			})
			keygenID := pssIDDetails.SharingID.GetKeygenID()
			keyIndex := keygenID.GetIndex()
			go func(pssMessage PSSMessage, kI int) {
				// stagger as an optimization
				pssNode.stagger(keyIndex)
				recover.Lock()
				sendBroadcast := recover.Si.Cmp(big.NewInt(0)) == 0
				recover.Unlock()
				if sendBroadcast {
					err := pssNode.Transport.SendBroadcast(pssMessage)
					if err != nil {
						logging.WithError(err).Error("could not send broadcast pssMsgPropose")
					}
				}
			}(nextPSSMessage, keyIndex)
		} else {
			logging.WithFields(logging.Fields{
				"recoverPSSCompletecountLength": len(recover.PSSCompleteCount),
				"k":                             pssNode.OldNodes.K,
			}).Debug("length of recover PSSCompletecount is not equal to k")
		}

		return nil
	}

	// Add to metrics
	telemetry.IncrementCounter(pcmn.TelemetryConstants.PSS.InvalidMethodCounter, pcmn.TelemetryConstants.PSS.Prefix)

	return errors.New("PssMessage method '" + pssMessage.Method + "' not found")
}

// ProcessBroadcastMessage is called when the node receives a message via broadcast (eg. Tendermint)
func (pssNode *PSSNode) ProcessBroadcastMessage(pssMessage PSSMessage) error {

	// Add to metrics
	telemetry.IncrementCounter(pcmn.TelemetryConstants.PSS.ProcessedBroadcastMessagesCounter, pcmn.TelemetryConstants.PSS.Prefix)

	logging.WithFields(logging.Fields{
		"NodeDetails":      string(pssNode.NodeDetails.ToNodeDetailsID())[0:8],
		"pssMessageMethod": pssMessage.Method,
	}).Debug("node processing broadcast message for pssMessageMethod")
	if pssMessage.Method == "decide" {
		// if untrusted, request for a proof that the decided message was included in a block

		// parse message
		var pssMsgDecide PSSMsgDecide
		err := bijson.Unmarshal(pssMessage.Data, &pssMsgDecide)
		if err != nil {
			logging.WithField("method", "share").WithError(err)
			return err
		}

		// wait for all sharings in decided set to complete
		var recover *Recover
		var pssRefs []*PSS
		var alreadyCleanUp, pssEnded bool
		for {
			recover, pssRefs, alreadyCleanUp, pssEnded = checkAllPSSEndedIfNotComplete(pssNode, pssMsgDecide.SharingID, pssMsgDecide.PSSs)
			if alreadyCleanUp {
				logging.WithField("sharingID", pssMsgDecide.SharingID).Debug("pss already complete and clean up, ignoring message in decide")
				return nil
			}
			if pssEnded {
				break
			}

			// Add to metrics
			telemetry.IncrementCounter(pcmn.TelemetryConstants.PSS.DecidedSharingsNotCompleteCounter, pcmn.TelemetryConstants.PSS.Prefix)

			logging.WithField("pssMsgDecide", pssMsgDecide).WithField("pssRefs", pssRefs).WithField("set", pssMsgDecide.PSSs).Debug("waiting for all sharings in decided set to complete")
			time.Sleep(1 * time.Second)
		}
		recover.Lock()
		defer recover.Unlock()
		var abarArray []common.Point
		var abarprimeArray []common.Point
		for _, pss := range pssRefs {
			var pssIDDetails PSSIDDetails
			err := pssIDDetails.FromPSSID(pss.PSSID)
			if err != nil {
				return err
			}
			abarArray = append(abarArray, common.Point{
				X: *big.NewInt(int64(pssIDDetails.DealerIndex)),
				Y: pss.Si,
			})
			abarprimeArray = append(abarprimeArray, common.Point{
				X: *big.NewInt(int64(pssIDDetails.DealerIndex)),
				Y: pss.Siprime,
			})
		}
		abar := pvss.LagrangeScalarCP(abarArray, 0)
		abarprime := pvss.LagrangeScalarCP(abarprimeArray, 0)
		recover.Si = *abar
		recover.Siprime = *abarprime
		var vbarInputPts [][]common.Point
		var vbarInputIndexes []int
		for _, pss := range pssRefs {
			var pssIDDetails PSSIDDetails
			err := pssIDDetails.FromPSSID(pss.PSSID)
			if err != nil {
				return err
			}
			vbarInputIndexes = append(vbarInputIndexes, pssIDDetails.DealerIndex)
			vbarInputPts = append(vbarInputPts, pcmn.GetColumnPoint(pss.Cbar, 0))
		}
		recover.Vbar = pvss.LagrangePolys(vbarInputIndexes, vbarInputPts)
		gsi := common.BigIntToPoint(secp256k1.Curve.ScalarBaseMult(recover.Si.Bytes()))
		hsiprime := common.BigIntToPoint(secp256k1.Curve.ScalarMult(&secp256k1.H.X, &secp256k1.H.Y, recover.Siprime.Bytes()))
		gsihsiprime := common.BigIntToPoint(secp256k1.Curve.Add(&gsi.X, &gsi.Y, &hsiprime.X, &hsiprime.Y))
		verified := pvss.VerifyShareCommitment(gsihsiprime, recover.Vbar, *big.NewInt(int64(pssNode.NodeDetails.Index)))
		if !verified {

			// Add to metrics
			telemetry.IncrementCounter(pcmn.TelemetryConstants.PSS.FinalSharesInvalidCounter, pcmn.TelemetryConstants.PSS.Prefix)

			return errors.New("could not verify shares against interpolated commitments")
		}

		logging.WithFields(logging.Fields{
			"NodeDetails": string(pssNode.NodeDetails.ToNodeDetailsID())[0:8],
			"Si":          recover.Si.Text(16),
			"Siprime":     recover.Siprime.Text(16),
		}).Debug("node verified shares against newly generated commitments")
		byt, _ := bijson.Marshal(recover.Vbar)
		logging.WithFields(logging.Fields{
			"NodeDetails": string(pssNode.NodeDetails.ToNodeDetailsID())[0:8],
			"Vbar":        string(byt),
		}).Debug()
		go func(msg string) {
			pssNode.Transport.Output(msg + " refreshed")

			// Add to metrics
			telemetry.IncrementCounter(pcmn.TelemetryConstants.PSS.NumRefreshedCounter, pcmn.TelemetryConstants.PSS.Prefix)

		}(string(pssMsgDecide.SharingID))

		// get keyIndex for refreshed share
		keygenID := recover.SharingID.GetKeygenID()
		stringKeygenID := string(keygenID)
		keyIndexSubstrs := strings.Split(stringKeygenID, pcmn.Delimiter3)
		if len(keyIndexSubstrs) != 2 {
			return fmt.Errorf("could not split keygenID %v", stringKeygenID)
		}

		keyIndex, ok := new(big.Int).SetString(keyIndexSubstrs[len(keyIndexSubstrs)-1], 16)
		if !ok {
			return errors.New("Could not setstring for keygenID " + string(keygenID))
		}

		go func(keyIndex big.Int, si big.Int, siprime big.Int, commitmentPoly []common.Point) {
			pssNode.Transport.Output(RefreshKeyStorage{
				KeyIndex:       keyIndex,
				Si:             si,
				Siprime:        siprime,
				CommitmentPoly: commitmentPoly,
			})
		}(*keyIndex, recover.Si, recover.Siprime, recover.Vbar)
		err = pssNode.CleanUp(pssNode, recover.SharingID)
		if err != nil {
			logging.WithError(err).WithField("sharingID", recover.SharingID).Error("could not clean up pss")
			return err
		}
		return nil
	}

	return errors.New("PssMessage method '" + pssMessage.Method + "' not found")
}

var pssCleanUp = func(pssNode *PSSNode, sharingID SharingID) error {
	pssNode.RecoverStore.Complete(sharingID)
	pssNode.PSSStore.Complete(sharingID)
	return nil
}

var noOpPSSCleanUp = func(*PSSNode, SharingID) error { return nil }

// NewPSSNode creates a new pss node instance
func NewPSSNode(
	nodeDetails pcmn.Node,
	oldEpoch int,
	oldNodeList []pcmn.Node,
	oldNodesT int,
	oldNodesK int,
	newEpoch int,
	newNodeList []pcmn.Node,
	newNodesT int,
	newNodesK int,
	nodeIndex int,
	dataSource PSSDataSource,
	transport PSSTransport,
	isDealer bool,
	isPlayer bool,
	staggerDelay int,
) *PSSNode {
	newPSSNode := &PSSNode{
		NodeDetails: NodeDetails(nodeDetails),
		OldNodes: NodeNetwork{
			Nodes:   mapFromNodeList(oldNodeList),
			N:       len(oldNodeList),
			T:       oldNodesT,
			K:       oldNodesK,
			EpochID: oldEpoch,
		},
		NewNodes: NodeNetwork{
			Nodes:   mapFromNodeList(newNodeList),
			N:       len(newNodeList),
			T:       newNodesT,
			K:       newNodesK,
			EpochID: newEpoch,
		},
		NodeIndex:    nodeIndex,
		RecoverStore: &RecoverStoreSyncMap{},
		IsDealer:     isDealer,
		IsPlayer:     isPlayer,
		CleanUp:      pssCleanUp,
		staggerDelay: staggerDelay,
	}
	newPSSNode.PSSStore = &PSSStoreSyncMap{
		nodes: &newPSSNode.NewNodes,
	}
	transport.Init()
	err := transport.SetPSSNode(newPSSNode)
	if err != nil {
		logging.WithError(err).Error("could not set pss node for transport")
	}
	newPSSNode.Transport = transport
	dataSource.Init()
	err = dataSource.SetPSSNode(newPSSNode)
	if err != nil {
		logging.WithError(err).Error("could not set pss node for datasource")
	}
	newPSSNode.DataSource = dataSource
	return newPSSNode
}

func stringify(i interface{}) string {
	bytArr, ok := i.([]byte)
	if ok {
		return string(bytArr)
	}
	str, ok := i.(string)
	if ok {
		return str
	}
	byt, err := bijson.Marshal(i)
	if err != nil {
		logging.WithError(err).Error("Could not bijsonmarshal")
	}
	return string(byt)
}

func checkAllPSSEndedIfNotComplete(pssNode *PSSNode, sharingID SharingID, pssArr []PSSID) (recover *Recover, pssRefs []*PSS, alreadyCleanUp bool, pssEnded bool) {
	pssEnded = true
	recover, found := pssNode.RecoverStore.Get(sharingID)
	if found && recover == nil {
		alreadyCleanUp = true
		return
	}
	if !found {
		pssEnded = false
		return
	}
	for _, pssid := range pssArr {
		pss, found := pssNode.PSSStore.Get(pssid)
		if found && pss == nil {
			alreadyCleanUp = true
			continue
		}
		if !found {
			pssEnded = false
			continue
		}
		pss.Lock()
		if found && pss.State.Phase != States.Phases.Ended {
			pssEnded = false
		}
		pssRefs = append(pssRefs, pss)
		pss.Unlock()
	}
	return
}

// checkSufficientRecoversIfNotComplete - checks if there are sufficient recovers (by checking if recover.D is nil)
// if the recover instance has already been completed and cleaned up, this function
// notifies you in the return
func checkSufficientRecoversIfNotComplete(pssNode *PSSNode, sharingID SharingID) (recover *Recover, alreadyCleanUp bool, sufficientRecovers bool) {
	recover, found := pssNode.RecoverStore.Get(sharingID)
	if found && recover == nil {
		alreadyCleanUp = true
		return
	}
	if !found {
		sufficientRecovers = false
		return
	}
	recover.Lock()
	if recover.D != nil { // only set once we receive enough recovers from the old nodes
		sufficientRecovers = true
	}
	recover.Unlock()
	return
}

// stagger - delays/blocks for a duration depending on key index and node index
// used as an optimization to delay execution of broadcasts to bft
// seed is usually keyIndex
func (pssNode *PSSNode) stagger(seed int) {
	index := pssNode.NodeDetails.Index
	nodeSetLength := pssNode.NewNodes.N
	time.Sleep(time.Duration(((seed+index)%nodeSetLength)*pssNode.staggerDelay) * time.Millisecond)
}
