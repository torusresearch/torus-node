package keygennofsm

import (
	"errors"
	"fmt"
	"math/big"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/torusresearch/torus-node/telemetry"

	logging "github.com/sirupsen/logrus"
	"github.com/torusresearch/bijson"
	"github.com/torusresearch/torus-common/common"
	pcmn "github.com/torusresearch/torus-node/common"
	"github.com/torusresearch/torus-node/pvss"
)

func (keygenNode *KeygenNode) processShareMessage(keygenMsgShare KeygenMsgShare, keygen *Keygen) error {

	// state checks
	if keygen.State.Phase == States.Phases.Ended {

		// Add to metrics
		telemetry.IncrementCounter(pcmn.TelemetryConstants.Keygen.IgnoredMessage, pcmn.TelemetryConstants.Keygen.Prefix)

		return nil
	}

	// Add to metrics
	telemetry.IncrementCounter(pcmn.TelemetryConstants.Keygen.ShareMessage, pcmn.TelemetryConstants.Keygen.Prefix)

	S := *pvss.RandomBigInt()
	Sprime := *pvss.RandomBigInt()
	sharing, _ := keygenNode.ShareStore.GetOrSet(keygenMsgShare.DKGID, &Sharing{
		DKGID:  keygenMsgShare.DKGID,
		I:      keygenNode.NodeIndex,
		S:      S,
		Sprime: Sprime,
	})

	sharing.Lock()
	defer sharing.Unlock()
	if sharing.S.Cmp(big.NewInt(int64(0))) == 0 || sharing.Sprime.Cmp(big.NewInt(int64(0))) == 0 {
		return fmt.Errorf("DKGID %v was not initialized with Si and Siprime. Si: %v, Siprime: %v", keygenMsgShare.DKGID, sharing.Sprime, sharing.Sprime)
	}
	keygen.F = pvss.GenerateRandomBivariatePolynomial(sharing.S, keygenNode.CurrNodes.K)
	keygen.Fprime = pvss.GenerateRandomBivariatePolynomial(sharing.Sprime, keygenNode.CurrNodes.K)
	keygen.C = pvss.GetCommitmentMatrix(keygen.F, keygen.Fprime)

	for _, newNode := range keygenNode.CurrNodes.Nodes {
		keygenID := (&KeygenIDDetails{
			DKGID:       sharing.DKGID,
			DealerIndex: sharing.I,
		}).ToKeygenID()
		keygenMsgSend := &KeygenMsgSend{
			KeygenID: keygenID,
			C:        keygen.C,
			A:        pvss.EvaluateBivarPolyAtX(keygen.F, *big.NewInt(int64(newNode.Index))).Coeff,
			Aprime:   pvss.EvaluateBivarPolyAtX(keygen.Fprime, *big.NewInt(int64(newNode.Index))).Coeff,
			B:        pvss.EvaluateBivarPolyAtY(keygen.F, *big.NewInt(int64(newNode.Index))).Coeff,
			Bprime:   pvss.EvaluateBivarPolyAtY(keygen.Fprime, *big.NewInt(int64(newNode.Index))).Coeff,
		}
		data, err := bijson.Marshal(keygenMsgSend)
		if err != nil {
			return err
		}
		nextKeygenMessage := CreateKeygenMessage(KeygenMessageRaw{
			KeygenID: keygenID,
			Method:   "send",
			Data:     data,
		})
		go func(newN NodeDetails, msg KeygenMessage) {
			err := keygenNode.Transport.Send(newN, msg)
			if err != nil {
				logging.WithError(err).Error("could not send keygenMsgSend")
			}
		}(newNode, nextKeygenMessage)
	}

	// state updates
	keygen.State.Phase = States.Phases.Started

	return nil
}

func (keygenNode *KeygenNode) processSendMessage(keygenMsgSend KeygenMsgSend, keygen *Keygen, keygenIDDetails KeygenIDDetails, senderDetails NodeDetails) error {
	// state checks
	if keygen.State.Phase == States.Phases.Ended {

		// Add to metrics
		telemetry.IncrementCounter(pcmn.TelemetryConstants.Keygen.IgnoredMessage, pcmn.TelemetryConstants.Keygen.Prefix)

		return nil
	}

	// Add to metrics
	telemetry.IncrementCounter(pcmn.TelemetryConstants.Keygen.SendMessage, pcmn.TelemetryConstants.Keygen.Prefix)

	senderID := keygenNode.CurrNodes.Nodes[senderDetails.ToNodeDetailsID()].Index
	if senderID == keygenIDDetails.DealerIndex {
		defer func() { keygen.State.ReceivedSend = States.ReceivedSend.True }()
	} else {
		return errors.New("'Send' message contains index of " + strconv.Itoa(keygenIDDetails.DealerIndex) + " was not sent by node " + strconv.Itoa(senderID))
	}
	verified := pvss.AVSSVerifyPoly(
		keygenMsgSend.C,
		*big.NewInt(int64(keygenNode.NodeDetails.Index)),
		pcmn.PrimaryPolynomial{Coeff: keygenMsgSend.A, Threshold: keygenNode.CurrNodes.K},
		pcmn.PrimaryPolynomial{Coeff: keygenMsgSend.Aprime, Threshold: keygenNode.CurrNodes.K},
		pcmn.PrimaryPolynomial{Coeff: keygenMsgSend.B, Threshold: keygenNode.CurrNodes.K},
		pcmn.PrimaryPolynomial{Coeff: keygenMsgSend.Bprime, Threshold: keygenNode.CurrNodes.K},
	)
	if !verified {
		return errors.New("Could not verify polys against commitment")
	}

	// Add to metrics
	telemetry.IncrementCounter(pcmn.TelemetryConstants.Keygen.SendingEcho, pcmn.TelemetryConstants.Keygen.Prefix)

	logging.WithFields(logging.Fields{
		"NodeDetails":   string(keygenNode.NodeDetails.ToNodeDetailsID())[0:8],
		"senderDetails": string(senderDetails.ToNodeDetailsID())[0:8],
	}).Debug("node verified send message from sender, sending echo message")
	for _, newNode := range keygenNode.CurrNodes.Nodes {
		keygenMsgEcho := KeygenMsgEcho{
			KeygenID: keygenMsgSend.KeygenID,
			C:        keygenMsgSend.C,
			Alpha: *pvss.PolyEval(
				pcmn.PrimaryPolynomial{Coeff: keygenMsgSend.A, Threshold: keygenNode.CurrNodes.K},
				*big.NewInt(int64(newNode.Index)),
			),
			Alphaprime: *pvss.PolyEval(
				pcmn.PrimaryPolynomial{Coeff: keygenMsgSend.Aprime, Threshold: keygenNode.CurrNodes.K},
				*big.NewInt(int64(newNode.Index)),
			),
			Beta: *pvss.PolyEval(
				pcmn.PrimaryPolynomial{Coeff: keygenMsgSend.B, Threshold: keygenNode.CurrNodes.K},
				*big.NewInt(int64(newNode.Index)),
			),
			Betaprime: *pvss.PolyEval(
				pcmn.PrimaryPolynomial{Coeff: keygenMsgSend.Bprime, Threshold: keygenNode.CurrNodes.K},
				*big.NewInt(int64(newNode.Index)),
			),
		}
		data, err := bijson.Marshal(keygenMsgEcho)
		if err != nil {
			return err
		}
		nextKeygenMessage := CreateKeygenMessage(KeygenMessageRaw{
			KeygenID: keygen.KeygenID,
			Method:   "echo",
			Data:     data,
		})
		go func(newN NodeDetails, msg KeygenMessage) {
			err := keygenNode.Transport.Send(newN, msg)
			if err != nil {
				logging.WithError(err).Info()
			}
		}(newNode, nextKeygenMessage)
	}
	return nil
}

func (keygenNode *KeygenNode) processEchoMessage(keygenMsgEcho KeygenMsgEcho, keygen *Keygen, keygenIDDetails KeygenIDDetails, senderDetails NodeDetails) error {
	// state checks
	if keygen.State.Phase == States.Phases.Ended {

		// Add to metrics
		telemetry.IncrementCounter(pcmn.TelemetryConstants.Keygen.IgnoredMessage, pcmn.TelemetryConstants.Keygen.Prefix)

		return nil
	}
	receivedEcho, found := keygen.State.ReceivedEcho[senderDetails.ToNodeDetailsID()]
	if found && receivedEcho == States.ReceivedEcho.True {
		return errors.New("Already received a echo message for KeygenID " + string(keygen.KeygenID) + "from sender " + string(senderDetails.ToNodeDetailsID())[0:8])
	}

	// Add to metrics
	telemetry.IncrementCounter(pcmn.TelemetryConstants.Keygen.EchoMessage, pcmn.TelemetryConstants.Keygen.Prefix)

	verified := pvss.AVSSVerifyPoint(
		keygenMsgEcho.C,
		*big.NewInt(int64(senderDetails.Index)),
		*big.NewInt(int64(keygenNode.NodeDetails.Index)),
		keygenMsgEcho.Alpha,
		keygenMsgEcho.Alphaprime,
		keygenMsgEcho.Beta,
		keygenMsgEcho.Betaprime,
	)
	if !verified {

		// Add to metrics
		telemetry.IncrementCounter(pcmn.TelemetryConstants.Keygen.InvalidEcho, pcmn.TelemetryConstants.Keygen.Prefix)

		return errors.New("could not verify point against commitments for echo message")
	}
	logging.WithFields(logging.Fields{
		"NodeDetails":   string(keygenNode.NodeDetails.ToNodeDetailsID())[0:8],
		"senderDetails": string(senderDetails.ToNodeDetailsID())[0:8],
	}).Debug("node verified echo message from sender")
	cID := GetCIDFromPointMatrix(keygenMsgEcho.C)
	_, found = keygen.CStore[cID]
	if !found {
		keygen.CStore[cID] = &C{
			CID:             cID,
			C:               keygenMsgEcho.C,
			AC:              make(map[NodeDetailsID]common.Point),
			ACprime:         make(map[NodeDetailsID]common.Point),
			BC:              make(map[NodeDetailsID]common.Point),
			BCprime:         make(map[NodeDetailsID]common.Point),
			SignedTextStore: make(map[NodeDetailsID]SignedText),
		}
	}
	c := keygen.CStore[cID]
	c.AC[senderDetails.ToNodeDetailsID()] = common.Point{
		X: *big.NewInt(int64(senderDetails.Index)),
		Y: keygenMsgEcho.Alpha,
	}
	c.ACprime[senderDetails.ToNodeDetailsID()] = common.Point{
		X: *big.NewInt(int64(senderDetails.Index)),
		Y: keygenMsgEcho.Alphaprime,
	}
	c.BC[senderDetails.ToNodeDetailsID()] = common.Point{
		X: *big.NewInt(int64(senderDetails.Index)),
		Y: keygenMsgEcho.Beta,
	}
	c.BCprime[senderDetails.ToNodeDetailsID()] = common.Point{
		X: *big.NewInt(int64(senderDetails.Index)),
		Y: keygenMsgEcho.Betaprime,
	}

	c.EC = c.EC + 1
	if c.EC == ecThreshold(keygenNode.CurrNodes.N, keygenNode.CurrNodes.K, keygenNode.CurrNodes.T) &&
		c.RC < keygenNode.CurrNodes.K {

		// Add to metrics
		telemetry.IncrementCounter(pcmn.TelemetryConstants.Keygen.SendingReady, pcmn.TelemetryConstants.Keygen.Prefix)

		logging.WithFields(logging.Fields{
			"NodeDetails":   string(keygenNode.NodeDetails.ToNodeDetailsID())[0:8],
			"senderDetails": string(senderDetails.ToNodeDetailsID())[0:8],
		}).Debug("node verified echo message from sender and sending ready message")
		// Note: Despite the name mismatch below, this is correct, and the AVSS spec is wrong.
		c.Abar = pvss.LagrangeInterpolatePolynomial(GetPointArrayFromMap(c.BC))
		c.Abarprime = pvss.LagrangeInterpolatePolynomial(GetPointArrayFromMap(c.BCprime))
		c.Bbar = pvss.LagrangeInterpolatePolynomial(GetPointArrayFromMap(c.AC))
		c.Bbarprime = pvss.LagrangeInterpolatePolynomial(GetPointArrayFromMap(c.ACprime))
		for _, newNode := range keygenNode.CurrNodes.Nodes {
			signedTextDetails := SignedTextDetails{
				Text: strings.Join([]string{string(keygen.KeygenID), "ready"}, pcmn.Delimiter1),
				C00:  c.C[0][0],
			}
			sigBytes, err := keygenNode.Transport.Sign(signedTextDetails.ToBytes())
			if err != nil {
				return err
			}
			signedText := SignedText(sigBytes)
			keygenMsgReady := KeygenMsgReady{
				KeygenID: keygen.KeygenID,
				C:        c.C,
				Alpha: *pvss.PolyEval(
					pcmn.PrimaryPolynomial{Coeff: c.Abar, Threshold: keygenNode.CurrNodes.K},
					*big.NewInt(int64(newNode.Index)),
				),
				Alphaprime: *pvss.PolyEval(
					pcmn.PrimaryPolynomial{Coeff: c.Abarprime, Threshold: keygenNode.CurrNodes.K},
					*big.NewInt(int64(newNode.Index)),
				),
				Beta: *pvss.PolyEval(
					pcmn.PrimaryPolynomial{Coeff: c.Bbar, Threshold: keygenNode.CurrNodes.K},
					*big.NewInt(int64(newNode.Index)),
				),
				Betaprime: *pvss.PolyEval(
					pcmn.PrimaryPolynomial{Coeff: c.Bbarprime, Threshold: keygenNode.CurrNodes.K},
					*big.NewInt(int64(newNode.Index)),
				),
				SignedText: signedText,
			}
			data, err := bijson.Marshal(keygenMsgReady)
			if err != nil {
				return err
			}
			nextKeygenMessage := CreateKeygenMessage(KeygenMessageRaw{
				KeygenID: keygen.KeygenID,
				Method:   "ready",
				Data:     data,
			})
			go func(newN NodeDetails, msg KeygenMessage) {
				err := keygenNode.Transport.Send(newN, msg)
				if err != nil {
					logging.WithError(err).Info()
				}
			}(newNode, nextKeygenMessage)
		}
	}
	return nil
}

func (keygenNode *KeygenNode) processReadyMessage(keygenMsgReady KeygenMsgReady, keygen *Keygen, keygenIDDetails KeygenIDDetails, senderDetails NodeDetails) error {
	// state checks
	if keygen.State.Phase == States.Phases.Ended {

		// Add to metrics
		telemetry.IncrementCounter(pcmn.TelemetryConstants.Keygen.IgnoredMessage, pcmn.TelemetryConstants.Keygen.Prefix)

		return nil
	}
	receivedReady, found := keygen.State.ReceivedReady[senderDetails.ToNodeDetailsID()]
	if found && receivedReady == States.ReceivedReady.True {

		// Add to metrics
		telemetry.IncrementCounter(pcmn.TelemetryConstants.Keygen.AlreadyReceivedReady, pcmn.TelemetryConstants.Keygen.Prefix)

		return errors.New("Already received a ready message for KeygenID " + string(keygen.KeygenID))
	}

	// Add to metrics
	telemetry.IncrementCounter(pcmn.TelemetryConstants.Keygen.ReadyMessage, pcmn.TelemetryConstants.Keygen.Prefix)

	verified := pvss.AVSSVerifyPoint(
		keygenMsgReady.C,
		*big.NewInt(int64(senderDetails.Index)),
		*big.NewInt(int64(keygenNode.NodeDetails.Index)),
		keygenMsgReady.Alpha,
		keygenMsgReady.Alphaprime,
		keygenMsgReady.Beta,
		keygenMsgReady.Betaprime,
	)
	if !verified {

		// Add telemetry
		telemetry.IncrementCounter(pcmn.TelemetryConstants.Keygen.InvalidReady, pcmn.TelemetryConstants.Keygen.Prefix)

		return errors.New("could not verify point against commitments for ready message")
	}
	signedTextDetails := SignedTextDetails{
		Text: strings.Join([]string{string(keygen.KeygenID), "ready"}, pcmn.Delimiter1),
		C00:  keygenMsgReady.C[0][0],
	}
	sigValid := pvss.ECDSAVerifyBytes(signedTextDetails.ToBytes(), &senderDetails.PubKey, keygenMsgReady.SignedText)
	if !sigValid {

		// Add telemetry
		telemetry.IncrementCounter(pcmn.TelemetryConstants.Keygen.ReadySigInvalid, pcmn.TelemetryConstants.Keygen.Prefix)

		return errors.New("could not verify signature on ready message for keygen " + string(keygen.KeygenID))
	}
	logging.WithFields(logging.Fields{
		"NodeDetails":   string(keygenNode.NodeDetails.ToNodeDetailsID())[0:8],
		"senderDetails": string(senderDetails.ToNodeDetailsID())[0:8],
	}).Debug("node verified ready message from sender")

	cID := GetCIDFromPointMatrix(keygenMsgReady.C)
	_, found = keygen.CStore[cID]
	if !found {
		keygen.CStore[cID] = &C{
			CID:             cID,
			C:               keygenMsgReady.C,
			AC:              make(map[NodeDetailsID]common.Point),
			ACprime:         make(map[NodeDetailsID]common.Point),
			BC:              make(map[NodeDetailsID]common.Point),
			BCprime:         make(map[NodeDetailsID]common.Point),
			SignedTextStore: make(map[NodeDetailsID]SignedText),
		}
	}
	c := keygen.CStore[cID]
	c.AC[senderDetails.ToNodeDetailsID()] = common.Point{
		X: *big.NewInt(int64(senderDetails.Index)),
		Y: keygenMsgReady.Alpha,
	}
	c.ACprime[senderDetails.ToNodeDetailsID()] = common.Point{
		X: *big.NewInt(int64(senderDetails.Index)),
		Y: keygenMsgReady.Alphaprime,
	}
	c.BC[senderDetails.ToNodeDetailsID()] = common.Point{
		X: *big.NewInt(int64(senderDetails.Index)),
		Y: keygenMsgReady.Beta,
	}
	c.BCprime[senderDetails.ToNodeDetailsID()] = common.Point{
		X: *big.NewInt(int64(senderDetails.Index)),
		Y: keygenMsgReady.Betaprime,
	}
	c.SignedTextStore[senderDetails.ToNodeDetailsID()] = keygenMsgReady.SignedText
	c.RC = c.RC + 1
	if c.RC == keygenNode.CurrNodes.K &&
		c.EC < ecThreshold(keygenNode.CurrNodes.N, keygenNode.CurrNodes.K, keygenNode.CurrNodes.T) {

		// Add to metrics
		telemetry.IncrementCounter(pcmn.TelemetryConstants.Keygen.SendingReady, pcmn.TelemetryConstants.Keygen.Prefix)
		telemetry.IncrementCounter(pcmn.TelemetryConstants.Keygen.ReadyBeforeEcho, pcmn.TelemetryConstants.Keygen.Prefix)

		logging.WithFields(logging.Fields{
			"NodeDetails":   string(keygenNode.NodeDetails.ToNodeDetailsID())[0:8],
			"senderDetails": string(senderDetails.ToNodeDetailsID())[0:8],
		}).Debug("node verified ready message from sender, and sending ready message")
		// Note: Despite the name mismatch below, this is correct, and the AVSS spec is wrong.
		c.Abar = pvss.LagrangeInterpolatePolynomial(GetPointArrayFromMap(c.BC))
		c.Abarprime = pvss.LagrangeInterpolatePolynomial(GetPointArrayFromMap(c.BCprime))
		c.Bbar = pvss.LagrangeInterpolatePolynomial(GetPointArrayFromMap(c.AC))
		c.Bbarprime = pvss.LagrangeInterpolatePolynomial(GetPointArrayFromMap(c.ACprime))
		for _, newNode := range keygenNode.CurrNodes.Nodes {
			signedTextDetails := SignedTextDetails{
				Text: strings.Join([]string{string(keygen.KeygenID), "ready"}, pcmn.Delimiter1),
				C00:  c.C[0][0],
			}
			sigBytes, err := keygenNode.Transport.Sign(signedTextDetails.ToBytes())
			if err != nil {
				return err
			}
			signedText := SignedText(sigBytes)
			keygenMsgReady := KeygenMsgReady{
				KeygenID: keygen.KeygenID,
				C:        c.C,
				Alpha: *pvss.PolyEval(
					pcmn.PrimaryPolynomial{Coeff: c.Abar, Threshold: keygenNode.CurrNodes.K},
					*big.NewInt(int64(newNode.Index)),
				),
				Alphaprime: *pvss.PolyEval(
					pcmn.PrimaryPolynomial{Coeff: c.Abarprime, Threshold: keygenNode.CurrNodes.K},
					*big.NewInt(int64(newNode.Index)),
				),
				Beta: *pvss.PolyEval(
					pcmn.PrimaryPolynomial{Coeff: c.Bbar, Threshold: keygenNode.CurrNodes.K},
					*big.NewInt(int64(newNode.Index)),
				),
				Betaprime: *pvss.PolyEval(
					pcmn.PrimaryPolynomial{Coeff: c.Bbarprime, Threshold: keygenNode.CurrNodes.K},
					*big.NewInt(int64(newNode.Index)),
				),
				SignedText: signedText,
			}
			data, err := bijson.Marshal(keygenMsgReady)
			if err != nil {
				return err
			}
			nextKeygenMessage := CreateKeygenMessage(KeygenMessageRaw{
				KeygenID: keygen.KeygenID,
				Method:   "ready",
				Data:     data,
			})
			go func(newN NodeDetails, msg KeygenMessage) {
				err := keygenNode.Transport.Send(newN, msg)
				if err != nil {
					logging.WithError(err).Error("could not send keygenMsgReady")
				}
			}(newNode, nextKeygenMessage)
		}
	} else if c.RC == keygenNode.CurrNodes.K+keygenNode.CurrNodes.T {

		// Add to metrics
		telemetry.IncrementCounter(pcmn.TelemetryConstants.Keygen.SendingComplete, pcmn.TelemetryConstants.Keygen.Prefix)

		logging.WithFields(logging.Fields{
			"NodeDetails":   string(keygenNode.NodeDetails.ToNodeDetailsID())[0:8],
			"senderDetails": string(senderDetails.ToNodeDetailsID())[0:8],
		}).Debug("node verified ready message from sender, sending complete message")
		keygen.Cbar = c.C
		keygen.Si = c.Abar[0]
		keygen.Siprime = c.Abarprime[0]
		go func(msg string) {
			keygenNode.Transport.Output(msg + " shared.")
		}(string(keygen.KeygenID))
		data, err := bijson.Marshal(KeygenMsgComplete{
			KeygenID:      keygen.KeygenID,
			CommitmentArr: keygen.Cbar[0],
		})
		if err != nil {
			return err
		}
		nextKeygenMessage := CreateKeygenMessage(KeygenMessageRaw{
			KeygenID: NullKeygenID,
			Method:   "complete",
			Data:     data,
		})
		go func(ownNode NodeDetails, ownMsg KeygenMessage) {
			err := keygenNode.Transport.Send(ownNode, ownMsg)
			if err != nil {
				logging.WithError(err).Error("could not send keygenMsgComplete")
			}
		}(keygenNode.NodeDetails, nextKeygenMessage)
		defer func() { keygen.State.Phase = States.Phases.Ended }()
	}
	return nil
}

func (keygenNode *KeygenNode) processCompleteMessage(keygenMsgComplete KeygenMsgComplete, keygenIDDetails KeygenIDDetails, senderDetails NodeDetails) error {
	// state checks
	if senderDetails.ToNodeDetailsID() != keygenNode.NodeDetails.ToNodeDetailsID() {
		return errors.New("This message can only be accepted if its sent to ourselves")
	}

	// logic
	dkgID := keygenIDDetails.DKGID
	dkg, complete := keygenNode.DKGStore.GetOrSetIfNotComplete(dkgID, &DKG{
		DKGID:      dkgID,
		NIZKPStore: make(map[NodeDetailsID]NIZKP),
		DMap:       make(map[KeygenID][]common.Point),
	})
	if complete {
		logging.WithField("dkgid", dkgID).Debug("already completed in processCompleteMessage, ignoring complete message")
		return nil
	}
	dkg.Lock()
	defer dkg.Unlock()
	dkg.DMap[keygenMsgComplete.KeygenID] = keygenMsgComplete.CommitmentArr

	// check if k + t dkg messages on same ID received
	if len(dkg.DMap) < keygenNode.CurrNodes.K+keygenNode.CurrNodes.T {
		logging.Debug("not enough dkgs received")
		return nil
	} else if len(dkg.DMap) > keygenNode.CurrNodes.K+keygenNode.CurrNodes.T {
		logging.Debug("more than enough dkgs received, should have already sent propose message")
		return nil
	}

	// Add to metrics
	telemetry.IncrementCounter(pcmn.TelemetryConstants.Keygen.SendingProposal, pcmn.TelemetryConstants.Keygen.Prefix)

	logging.WithField("NodeDetails", string(keygenNode.NodeDetails.ToNodeDetailsID())[0:8]).Debug("node reached k + t secret sharings, proposing a set...")
	// propose Li via validated byzantine agreement
	var keygens []KeygenID
	var proposeProofs []map[NodeDetailsID]ProposeProof
	for keygenid := range dkg.DMap {
		keygens = append(keygens, keygenid)
	}
	sort.Slice(keygens, func(i, j int) bool {
		return strings.Compare(string(keygens[i]), string(keygens[j])) < 0
	})

	for _, keygenid := range keygens {
		keygen, found := keygenNode.KeygenStore.Get(keygenid)
		if !found {
			return errors.New("Could not get completed keygen")
		}
		if found && keygen == nil {
			return errors.New("Already cleaned up")
		}
		cbar := keygen.Cbar
		if len(cbar) == 0 {
			return errors.New("Could not get completed cbar")
		}
		cid := GetCIDFromPointMatrix(cbar)
		c := keygen.CStore[cid]
		if c == nil {
			return errors.New("Could not get completed c")
		}
		proposeProofs = append(proposeProofs, SignedTextToProposeProof(c.C[0][0], c.SignedTextStore))
	}

	keygenMsgPropose := KeygenMsgPropose{
		NodeDetailsID: keygenNode.NodeDetails.ToNodeDetailsID(),
		DKGID:         dkgID,
		Keygens:       keygens,
		ProposeProofs: proposeProofs,
	}
	data, err := bijson.Marshal(keygenMsgPropose)
	if err != nil {
		return err
	}
	nextKeygenMessage := CreateKeygenMessage(KeygenMessageRaw{
		KeygenID: NullKeygenID,
		Method:   "propose",
		Data:     data,
	})
	keyIndex, err := dkgID.GetIndex()
	if err != nil {
		return err
	}

	// send broadcast if propose message has not been fully processed
	go func(keygenMessage KeygenMessage, kI big.Int) {
		// Stagger propose message for optimization
		keygenNode.stagger(int(kI.Int64()))
		dkg.Lock()
		sendBroadcast := dkg.Si.Cmp(big.NewInt(0)) == 0
		dkg.Unlock()
		if sendBroadcast {
			err := keygenNode.Transport.SendBroadcast(keygenMessage)
			if err != nil {
				logging.WithError(err).Error("could not send keygenMsgPropose")
			}
		}
	}(nextKeygenMessage, keyIndex)

	return nil
}

func (keygenNode *KeygenNode) processNIZKPMessage(keygenMsgNIZKP KeygenMsgNIZKP, keygen *Keygen, senderDetails NodeDetails) error {
	// logic
	// DKG not completed/started yet but nizkp already received

	// Add to metrics
	telemetry.IncrementCounter(pcmn.TelemetryConstants.Keygen.NizkpMessage, pcmn.TelemetryConstants.Keygen.Prefix)

	var dkg *DKG
	var alreadyCleanUp, dbarExists bool

	for {
		dkg, alreadyCleanUp, dbarExists = getDKGIfDbarExists(keygenNode, keygenMsgNIZKP.DKGID)
		if alreadyCleanUp {
			logging.WithField("dkgid", keygenMsgNIZKP.DKGID).Debug("already cleaned up dkgid, ignoring message")
			return nil
		}
		if dbarExists {
			break
		}
		if keygen != nil {
			keygen.Unlock()
		}
		logging.WithField("dkgid", keygenMsgNIZKP.DKGID).Debug("Waiting for dbar to be in...")
		time.Sleep(1 * time.Second)
		if keygen != nil {
			keygen.Lock()
		}
	}
	dkg.Lock()
	defer dkg.Unlock()
	if !pvss.VerifyShareCommitment(keygenMsgNIZKP.NIZKP.GSiHSiprime, dkg.Dbar, *big.NewInt(int64(keygenMsgNIZKP.NIZKP.NodeIndex))) {

		// Add to metrics
		telemetry.IncrementCounter(pcmn.TelemetryConstants.Keygen.InvalidShareCommitment, pcmn.TelemetryConstants.Keygen.Prefix)

		return errors.New("could not verify share commitment against existing Dbar")
	}
	if !pvss.VerifyNIZKPK(keygenMsgNIZKP.NIZKP.C, keygenMsgNIZKP.NIZKP.U1, keygenMsgNIZKP.NIZKP.U2, keygenMsgNIZKP.NIZKP.GSi, keygenMsgNIZKP.NIZKP.GSiHSiprime) {

		// Add to metrics
		telemetry.IncrementCounter(pcmn.TelemetryConstants.Keygen.InvalidNizkp, pcmn.TelemetryConstants.Keygen.Prefix)

		return errors.New("could not verify NIZKP")
	}
	// state updates
	dkg.NIZKPStore[senderDetails.ToNodeDetailsID()] = keygenMsgNIZKP.NIZKP
	if len(dkg.NIZKPStore) == keygenNode.CurrNodes.K {
		var indexes []int
		var points []common.Point
		var nizkps []NIZKP
		for nodeDetailsID, nizkp := range dkg.NIZKPStore {
			var nodeDetails NodeDetails
			nodeDetails.FromNodeDetailsID(nodeDetailsID)
			indexes = append(indexes, nodeDetails.Index)
			points = append(points, nizkp.GSi)
			nizkps = append(nizkps, nizkp)
		}
		dkg.GS = *pvss.LagrangeCurvePts(indexes, points)
		keyIndex, err := dkg.DKGID.GetIndex()
		if err != nil {
			return errors.New("could not get dkgID Index")
		}
		go func(msg string, keyIndex big.Int, si big.Int, siprime big.Int, commitmentPoly []common.Point) {
			keygenNode.Transport.Output(msg + " nizkp completed")
			keygenNode.Transport.Output(KeyStorage{
				KeyIndex:       keyIndex,
				Si:             si,
				Siprime:        siprime,
				CommitmentPoly: commitmentPoly,
			})
		}(string(dkg.DKGID), keyIndex, dkg.Si, dkg.Siprime, dkg.Dbar)

		// Add to metrics
		telemetry.IncrementCounter(pcmn.TelemetryConstants.Keygen.NumSharesVerified, pcmn.TelemetryConstants.Keygen.Prefix)

		keygenMsgPubKey := KeygenMsgPubKey{
			NodeDetailsID: keygenNode.NodeDetails.ToNodeDetailsID(),
			DKGID:         dkg.DKGID,
			PubKeyProofs:  nizkps,
		}
		data, err := bijson.Marshal(keygenMsgPubKey)
		if err != nil {
			return err
		}
		nextKeygenMessage := CreateKeygenMessage(KeygenMessageRaw{
			KeygenID: NullKeygenID,
			Method:   "pubkey",
			Data:     data,
		})

		err = keygenNode.CleanUp(keygenNode, dkg.DKGID)
		if err != nil {
			logging.WithError(err).WithField("dkgid", dkg.DKGID).Error("could not clean up dkg")
			return err
		}

		// send broadcast if propose message has not been fully processed
		go func(keygenMessage KeygenMessage, kI big.Int) {
			// Stagger nizkp message for optimization
			keygenNode.stagger(int(kI.Int64()))
			if !keygenNode.Transport.CheckIfNIZKPProcessed(keyIndex) {
				err := keygenNode.Transport.SendBroadcast(keygenMessage)
				if err != nil {
					logging.WithError(err).Error("could not send broadcast keygenMsgPubKey")
				}
			}
		}(nextKeygenMessage, keyIndex)

	}
	return nil
}

func SignedTextToProposeProof(c00 common.Point, st map[NodeDetailsID]SignedText) (pp map[NodeDetailsID]ProposeProof) {
	pp = make(map[NodeDetailsID]ProposeProof)
	for nodeDetailsID, signedTextDetails := range st {
		proposeProof := ProposeProof{
			SignedTextDetails: signedTextDetails,
			C00:               c00,
		}
		pp[nodeDetailsID] = proposeProof
	}
	return
}

// getDKGIfDbarExists - gets references to dkg, which are safe to use and won't be affected by cleanup
// if the dkg has been cleaned up (alreadyCleanUp) or dbar is in (dbarExists) this function
// notifies you in the return
func getDKGIfDbarExists(keygenNode *KeygenNode, dkgID DKGID) (dkg *DKG, alreadyCleanUp bool, dbarExists bool) {
	dkg, found := keygenNode.DKGStore.Get(dkgID)
	if !found {
		logging.WithField("dkgid", dkgID).Debug("dkg not created yet")
		return
	}
	if found && dkg == nil {
		logging.WithField("dkgid", dkgID).Debug("dkg already cleaned up")
		alreadyCleanUp = true
		return
	}
	dkg.Lock()
	defer dkg.Unlock()
	if len(dkg.Dbar) == 0 {
		logging.WithField("dkgid", dkg.DKGID).Debug("Dbar is not in yet")
		return
	}
	dbarExists = true
	return
}

// stagger - delays/blocks for a duration depending on key index and node index
// used as an optimization to delay execution of broadcasts to bft
// seed is usually keyIndex
func (keygenNode *KeygenNode) stagger(seed int) {
	index := keygenNode.NodeDetails.Index
	nodeSetLength := keygenNode.CurrNodes.N
	time.Sleep(time.Duration(((seed+index)%nodeSetLength)*keygenNode.staggerDelay) * time.Millisecond)
}
