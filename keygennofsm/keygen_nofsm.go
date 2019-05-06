package keygennofsm

import (
	"errors"
	"math"
	"math/big"
	"sort"
	"strconv"
	"strings"
	"time"

	"fmt"

	"github.com/torusresearch/bijson"
	"github.com/torusresearch/torus-public/common"
	"github.com/torusresearch/torus-public/logging"
	"github.com/torusresearch/torus-public/pvss"
	"github.com/torusresearch/torus-public/secp256k1"
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
func (keygenNode *KeygenNode) ProcessMessage(senderDetails NodeDetails, keygenMessage KeygenMessage) error {
	defer common.TimeTrack(time.Now(), "processMessage"+keygenMessage.Method)
	keygenNode.Lock()
	defer keygenNode.Unlock()
	logging.Debugf("%v processing message from %v for keygenMessage %v", string(keygenNode.NodeDetails.ToNodeDetailsID())[0:8], string(senderDetails.ToNodeDetailsID())[0:8], keygenMessage.Method)
	if _, found := keygenNode.KeygenStore[keygenMessage.KeygenID]; !found && keygenMessage.KeygenID != NullKeygenID {
		keygenNode.KeygenStore[keygenMessage.KeygenID] = &Keygen{
			KeygenID: keygenMessage.KeygenID,
			State: KeygenState{
				States.Phases.Initial,
				// States.DKG.Initial,
				States.ReceivedSend.False,
				States.ReceivedEchoMap(),
				States.ReceivedReadyMap(),
			},
			CStore: make(map[CID]*C),
		}
	}
	keygen, found := keygenNode.KeygenStore[keygenMessage.KeygenID]
	if found {
		keygen.Lock()
		defer keygen.Unlock()
		keygen.Messages = append(keygen.Messages, keygenMessage)
	}

	// handle different messages here
	if keygenMessage.Method == "share" {
		// parse message
		var keygenMsgShare KeygenMsgShare
		err := bijson.Unmarshal(keygenMessage.Data, &keygenMsgShare)
		if err != nil {
			logging.Error(err.Error())
			return err
		}
		// state checks
		if keygen.State.Phase == States.Phases.Ended {
			return errors.New("Keygen has ended, ignored message " + keygenMessage.Method + " from " + string(senderDetails.ToNodeDetailsID())[0:8] + " ")
		}

		// logic
		sharing, found := keygenNode.ShareStore[keygenMsgShare.SharingID]
		if !found {
			return fmt.Errorf("Could not find sharing for sharingID %v", string(keygenMsgShare.SharingID))
		}
		sharing.Lock()
		defer sharing.Unlock()
		if sharing.S.Cmp(big.NewInt(int64(0))) == 0 || sharing.Sprime.Cmp(big.NewInt(int64(0))) == 0 {
			return fmt.Errorf("SharingID %v was not initialized with Si and Siprime. Si: %v, Siprime: %v", keygenMsgShare.SharingID, sharing.Sprime, sharing.Sprime)
		}
		keygen.F = pvss.GenerateRandomBivariatePolynomial(sharing.S, keygenNode.CurrNodes.K)
		keygen.Fprime = pvss.GenerateRandomBivariatePolynomial(sharing.Sprime, keygenNode.CurrNodes.K)
		keygen.C = pvss.GetCommitmentMatrix(keygen.F, keygen.Fprime)

		for _, newNode := range keygenNode.CurrNodes.Nodes {
			keygenID := (&KeygenIDDetails{
				SharingID:   sharing.SharingID,
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
			nextKeygenMessage := KeygenMessage{
				KeygenID: keygenID,
				Method:   "send",
				Data:     data,
			}
			go func(newN NodeDetails, msg KeygenMessage) {
				err := keygenNode.Transport.Send(newN, msg)
				if err != nil {
					logging.Error(err.Error())
				}
			}(newNode, nextKeygenMessage)
		}
		defer func() { keygen.State.Phase = States.Phases.Started }()
		return nil
	} else if keygenMessage.Method == "send" {
		// parse message
		var keygenMsgSend KeygenMsgSend
		err := bijson.Unmarshal(keygenMessage.Data, &keygenMsgSend)
		if err != nil {
			logging.Error(err.Error())
			return err
		}
		// state checks
		if keygen.State.Phase == States.Phases.Ended {
			return errors.New("Keygen has ended, ignored message " + keygenMessage.Method + " from " + string(senderDetails.ToNodeDetailsID())[0:8] + " ")
		}

		// logic
		var keygenIDDetails KeygenIDDetails
		err = keygenIDDetails.FromKeygenID(keygenMessage.KeygenID)
		if err != nil {
			logging.Error(err.Error())
			return err
		}
		senderID := keygenNode.CurrNodes.Nodes[senderDetails.ToNodeDetailsID()].Index
		if senderID == keygenIDDetails.DealerIndex {
			defer func() { keygen.State.ReceivedSend = States.ReceivedSend.True }()
		} else {
			return errors.New("'Send' message contains index of " + strconv.Itoa(keygenIDDetails.DealerIndex) + " was not sent by node " + strconv.Itoa(senderID))
		}
		verified := pvss.AVSSVerifyPoly(
			keygenMsgSend.C,
			*big.NewInt(int64(keygenNode.NodeDetails.Index)),
			common.PrimaryPolynomial{Coeff: keygenMsgSend.A, Threshold: keygenNode.CurrNodes.K},
			common.PrimaryPolynomial{Coeff: keygenMsgSend.Aprime, Threshold: keygenNode.CurrNodes.K},
			common.PrimaryPolynomial{Coeff: keygenMsgSend.B, Threshold: keygenNode.CurrNodes.K},
			common.PrimaryPolynomial{Coeff: keygenMsgSend.Bprime, Threshold: keygenNode.CurrNodes.K},
		)
		if !verified {
			return errors.New("Could not verify polys against commitment")
		}
		logging.Debug(string(keygenNode.NodeDetails.ToNodeDetailsID())[0:8] + " verified send message from " + string(senderDetails.ToNodeDetailsID())[0:8] + ", sending echo message")
		for _, newNode := range keygenNode.CurrNodes.Nodes {
			keygenMsgEcho := KeygenMsgEcho{
				KeygenID: keygenMsgSend.KeygenID,
				C:        keygenMsgSend.C,
				Alpha: *pvss.PolyEval(
					common.PrimaryPolynomial{Coeff: keygenMsgSend.A, Threshold: keygenNode.CurrNodes.K},
					*big.NewInt(int64(newNode.Index)),
				),
				Alphaprime: *pvss.PolyEval(
					common.PrimaryPolynomial{Coeff: keygenMsgSend.Aprime, Threshold: keygenNode.CurrNodes.K},
					*big.NewInt(int64(newNode.Index)),
				),
				Beta: *pvss.PolyEval(
					common.PrimaryPolynomial{Coeff: keygenMsgSend.B, Threshold: keygenNode.CurrNodes.K},
					*big.NewInt(int64(newNode.Index)),
				),
				Betaprime: *pvss.PolyEval(
					common.PrimaryPolynomial{Coeff: keygenMsgSend.Bprime, Threshold: keygenNode.CurrNodes.K},
					*big.NewInt(int64(newNode.Index)),
				),
			}
			data, err := bijson.Marshal(keygenMsgEcho)
			if err != nil {
				return err
			}
			nextKeygenMessage := KeygenMessage{
				KeygenID: keygen.KeygenID,
				Method:   "echo",
				Data:     data,
			}
			go func(newN NodeDetails, msg KeygenMessage) {
				err := keygenNode.Transport.Send(newN, msg)
				if err != nil {
					logging.Info(err.Error())
				}
			}(newNode, nextKeygenMessage)
		}
		return nil
	} else if keygenMessage.Method == "echo" {
		// parse message
		defer func() { keygen.State.ReceivedEcho[senderDetails.ToNodeDetailsID()] = States.ReceivedEcho.True }()
		var keygenMsgEcho KeygenMsgEcho
		err := bijson.Unmarshal(keygenMessage.Data, &keygenMsgEcho)
		if err != nil {
			logging.Error(err.Error())
			return err
		}

		// state checks
		if keygen.State.Phase == States.Phases.Ended {
			return errors.New("Keygen has ended, ignored message " + keygenMessage.Method + " from " + string(senderDetails.ToNodeDetailsID())[0:8] + " ")
		}
		receivedEcho, found := keygen.State.ReceivedEcho[senderDetails.ToNodeDetailsID()]
		if found && receivedEcho == States.ReceivedEcho.True {
			return errors.New("Already received a echo message for KeygenID " + string(keygen.KeygenID) + "from sender " + string(senderDetails.ToNodeDetailsID())[0:8])
		}

		// logic
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
			return errors.New("Could not verify point against commitments for echo message")
		}
		logging.Debug(string(keygenNode.NodeDetails.ToNodeDetailsID())[0:8] + " verified echo message from " + string(senderDetails.ToNodeDetailsID())[0:8])
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
			logging.Debug(string(keygenNode.NodeDetails.ToNodeDetailsID())[0:8] + " verified echo message from " + string(senderDetails.ToNodeDetailsID())[0:8] + " and sending ready message")
			// Note: Despite the name mismatch below, this is correct, and the AVSS spec is wrong.
			c.Abar = pvss.LagrangeInterpolatePolynomial(GetPointArrayFromMap(c.BC))
			c.Abarprime = pvss.LagrangeInterpolatePolynomial(GetPointArrayFromMap(c.BCprime))
			c.Bbar = pvss.LagrangeInterpolatePolynomial(GetPointArrayFromMap(c.AC))
			c.Bbarprime = pvss.LagrangeInterpolatePolynomial(GetPointArrayFromMap(c.ACprime))
			for _, newNode := range keygenNode.CurrNodes.Nodes {
				sigBytes, err := keygenNode.Transport.Sign(string(keygen.KeygenID) + "|" + "ready")
				if err != nil {
					return err
				}
				signedText := SignedText(sigBytes)
				keygenMsgReady := KeygenMsgReady{
					KeygenID: keygen.KeygenID,
					C:        c.C,
					Alpha: *pvss.PolyEval(
						common.PrimaryPolynomial{Coeff: c.Abar, Threshold: keygenNode.CurrNodes.K},
						*big.NewInt(int64(newNode.Index)),
					),
					Alphaprime: *pvss.PolyEval(
						common.PrimaryPolynomial{Coeff: c.Abarprime, Threshold: keygenNode.CurrNodes.K},
						*big.NewInt(int64(newNode.Index)),
					),
					Beta: *pvss.PolyEval(
						common.PrimaryPolynomial{Coeff: c.Bbar, Threshold: keygenNode.CurrNodes.K},
						*big.NewInt(int64(newNode.Index)),
					),
					Betaprime: *pvss.PolyEval(
						common.PrimaryPolynomial{Coeff: c.Bbarprime, Threshold: keygenNode.CurrNodes.K},
						*big.NewInt(int64(newNode.Index)),
					),
					SignedText: signedText,
				}
				data, err := bijson.Marshal(keygenMsgReady)
				if err != nil {
					return err
				}
				nextKeygenMessage := KeygenMessage{
					KeygenID: keygen.KeygenID,
					Method:   "ready",
					Data:     data,
				}
				go func(newN NodeDetails, msg KeygenMessage) {
					err := keygenNode.Transport.Send(newN, msg)
					if err != nil {
						logging.Info(err.Error())
					}
				}(newNode, nextKeygenMessage)
			}
		}
		return nil
	} else if keygenMessage.Method == "ready" {
		// parse message
		defer func() { keygen.State.ReceivedReady[senderDetails.ToNodeDetailsID()] = States.ReceivedReady.True }()
		var keygenMsgReady KeygenMsgReady
		err := bijson.Unmarshal(keygenMessage.Data, &keygenMsgReady)
		if err != nil {
			logging.Error(err.Error())
			return err
		}
		var keygenIDDetails KeygenIDDetails
		err = keygenIDDetails.FromKeygenID(keygenMsgReady.KeygenID)
		if err != nil {
			logging.Error(err.Error())
			return err
		}

		// state checks
		if keygen.State.Phase == States.Phases.Ended {
			return errors.New("Keygen has ended, ignored message " + keygenMessage.Method + " from " + string(senderDetails.ToNodeDetailsID())[0:8] + " ")
		}
		receivedReady, found := keygen.State.ReceivedReady[senderDetails.ToNodeDetailsID()]
		if found && receivedReady == States.ReceivedReady.True {
			return errors.New("Already received a ready message for KeygenID " + string(keygen.KeygenID))
		}

		// logic
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
			return errors.New("Could not verify point against commitments for ready message")
		}
		sigValid := pvss.ECDSAVerify(string(keygen.KeygenID)+"|"+"ready", &senderDetails.PubKey, keygenMsgReady.SignedText)
		if !sigValid {
			return errors.New("Could not verify signature on message: " + string(keygen.KeygenID) + "|" + "ready")
		}
		logging.Debug(string(keygenNode.NodeDetails.ToNodeDetailsID())[0:8] + " verified ready message from " + string(senderDetails.ToNodeDetailsID())[0:8])

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
			logging.Debug(string(keygenNode.NodeDetails.ToNodeDetailsID())[0:8] + " verified ready message from " + string(senderDetails.ToNodeDetailsID())[0:8] + " and sending ready message")
			c.Abar = pvss.LagrangeInterpolatePolynomial(GetPointArrayFromMap(c.AC))
			c.Abarprime = pvss.LagrangeInterpolatePolynomial(GetPointArrayFromMap(c.ACprime))
			c.Bbar = pvss.LagrangeInterpolatePolynomial(GetPointArrayFromMap(c.BC))
			c.Bbarprime = pvss.LagrangeInterpolatePolynomial(GetPointArrayFromMap(c.BCprime))
			for _, newNode := range keygenNode.CurrNodes.Nodes {
				sigBytes, err := keygenNode.Transport.Sign(string(keygen.KeygenID) + "|" + "ready")
				if err != nil {
					return err
				}
				signedText := SignedText(sigBytes)
				keygenMsgReady := KeygenMsgReady{
					KeygenID: keygen.KeygenID,
					C:        c.C,
					Alpha: *pvss.PolyEval(
						common.PrimaryPolynomial{Coeff: c.Abar, Threshold: keygenNode.CurrNodes.K},
						*big.NewInt(int64(newNode.Index)),
					),
					Alphaprime: *pvss.PolyEval(
						common.PrimaryPolynomial{Coeff: c.Abarprime, Threshold: keygenNode.CurrNodes.K},
						*big.NewInt(int64(newNode.Index)),
					),
					Beta: *pvss.PolyEval(
						common.PrimaryPolynomial{Coeff: c.Bbar, Threshold: keygenNode.CurrNodes.K},
						*big.NewInt(int64(newNode.Index)),
					),
					Betaprime: *pvss.PolyEval(
						common.PrimaryPolynomial{Coeff: c.Bbarprime, Threshold: keygenNode.CurrNodes.K},
						*big.NewInt(int64(newNode.Index)),
					),
					SignedText: signedText,
				}
				data, err := bijson.Marshal(keygenMsgReady)
				if err != nil {
					return err
				}
				nextKeygenMessage := KeygenMessage{
					KeygenID: keygen.KeygenID,
					Method:   "ready",
					Data:     data,
				}
				go func(newN NodeDetails, msg KeygenMessage) {
					err := keygenNode.Transport.Send(newN, msg)
					if err != nil {
						logging.Error(err.Error())
					}
				}(newNode, nextKeygenMessage)
			}
		} else if c.RC == keygenNode.CurrNodes.K+keygenNode.CurrNodes.T {
			logging.Debug(string(keygenNode.NodeDetails.ToNodeDetailsID())[0:8] + " verified ready message from " + string(senderDetails.ToNodeDetailsID())[0:8] + " and sending complete message")
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
			nextKeygenMessage := KeygenMessage{
				KeygenID: NullKeygenID,
				Method:   "complete",
				Data:     data,
			}
			go func(ownNode NodeDetails, ownMsg KeygenMessage) {
				err := keygenNode.Transport.Send(ownNode, ownMsg)
				if err != nil {
					logging.Error(err.Error())
				}
			}(keygenNode.NodeDetails, nextKeygenMessage)
			defer func() { keygen.State.Phase = States.Phases.Ended }()
		}
		return nil
	} else if keygenMessage.Method == "complete" {
		// parse message
		var keygenMsgComplete KeygenMsgComplete
		err := bijson.Unmarshal(keygenMessage.Data, &keygenMsgComplete)
		if err != nil {
			logging.Error(err.Error())
			return err
		}

		// state checks
		if senderDetails.ToNodeDetailsID() != keygenNode.NodeDetails.ToNodeDetailsID() {
			return errors.New("This message can only be accepted if its sent to ourselves")
		}

		// logic
		var keygenIDDetails KeygenIDDetails
		keygenIDDetails.FromKeygenID(keygenMsgComplete.KeygenID)
		sharingID := keygenIDDetails.SharingID
		if _, found := keygenNode.DKGStore[sharingID]; !found {
			// first node to complete their sharing for this sharingID
			keygenNode.DKGStore[sharingID] = &DKG{
				SharingID: sharingID,
				GSiStore:  make(map[NodeDetailsID]common.Point),
				DMap:      make(map[KeygenID][]common.Point),
			}
		}
		dkg := keygenNode.DKGStore[sharingID]

		dkg.DMap[keygenMsgComplete.KeygenID] = keygenMsgComplete.CommitmentArr

		// check if k + t dkg messages on same ID received
		if len(dkg.DMap) < keygenNode.CurrNodes.K+keygenNode.CurrNodes.T {
			logging.Info("Not enough dkgs received")
			return nil
		} else if len(dkg.DMap) > keygenNode.CurrNodes.K+keygenNode.CurrNodes.T {
			logging.Info("More than enough dkgs received, should have already sent propose message")
			return nil
		}

		logging.Debug(string(keygenNode.NodeDetails.ToNodeDetailsID())[0:8] + " reached k + t secret sharings, proposing a set...")
		// propose Li via validated byzantine agreement
		var keygens []KeygenID
		var signedTexts []map[NodeDetailsID]SignedText
		for keygenid := range dkg.DMap {
			keygens = append(keygens, keygenid)
		}
		sort.Slice(keygens, func(i, j int) bool {
			return strings.Compare(string(keygens[i]), string(keygens[j])) < 0
		})

		for _, keygenid := range keygens {
			keygen := keygenNode.KeygenStore[keygenid]
			if keygen == nil {
				return errors.New("Could not get completed keygen")
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
			signedTexts = append(signedTexts, c.SignedTextStore)
		}

		keygenMsgPropose := KeygenMsgPropose{
			NodeDetailsID: keygenNode.NodeDetails.ToNodeDetailsID(),
			SharingID:     sharingID,
			Keygens:       keygens,
			SignedTexts:   signedTexts,
		}
		data, err := bijson.Marshal(keygenMsgPropose)
		if err != nil {
			return err
		}
		nextKeygenMessage := KeygenMessage{
			KeygenID: NullKeygenID,
			Method:   "propose",
			Data:     data,
		}
		go func(keygenMessage KeygenMessage) {
			err := keygenNode.Transport.SendBroadcast(keygenMessage)
			if err != nil {
				logging.Error(err.Error())
			}
		}(nextKeygenMessage)
		return nil
	} else if keygenMessage.Method == "nizkp" {
		// parse message
		var keygenMsgNIZKP KeygenMsgNIZKP
		err := bijson.Unmarshal(keygenMessage.Data, &keygenMsgNIZKP)
		if err != nil {
			logging.Error(err.Error())
			return err
		}

		// state checks

		// logic
		// DKG not completed/started yet but nizkp already received
		if _, found := keygenNode.DKGStore[keygenMsgNIZKP.SharingID]; !found {
			keygenNode.DKGStore[keygenMsgNIZKP.SharingID] = &DKG{
				SharingID: keygenMsgNIZKP.SharingID,
				GSiStore:  make(map[NodeDetailsID]common.Point),
				DMap:      make(map[KeygenID][]common.Point),
			}
		}
		dkg := keygenNode.DKGStore[keygenMsgNIZKP.SharingID]
		if len(dkg.Dbar) == 0 {
			// TODO: wait for it to be done
		}
		if !pvss.VerifyShareCommitment(keygenMsgNIZKP.GSiHSiprime, dkg.Dbar, *big.NewInt(int64(keygenMsgNIZKP.NodeIndex))) {
			return errors.New("Could not verify share commitment against existing Dbar")
		}
		if !pvss.VerifyNIZKPK(keygenMsgNIZKP.C, keygenMsgNIZKP.U1, keygenMsgNIZKP.U2, keygenMsgNIZKP.GSi, keygenMsgNIZKP.GSiHSiprime) {
			return errors.New("Could not verify NIZKP")
		}
		dkg.GSiStore[senderDetails.ToNodeDetailsID()] = keygenMsgNIZKP.GSi

		if len(dkg.GSiStore) == keygenNode.CurrNodes.K {
			var indexes []int
			var points []common.Point
			for nodeDetailsID, pt := range dkg.GSiStore {
				var nodeDetails NodeDetails
				nodeDetails.FromNodeDetailsID(nodeDetailsID)
				indexes = append(indexes, nodeDetails.Index)
				points = append(points, pt)
			}
			dkg.GS = *pvss.LagrangeCurvePts(indexes, points)
			go func(msg string) {
				keygenNode.Transport.Output(msg + " nizkp completed")
			}(string(dkg.SharingID))
		}
		return nil
	}

	return fmt.Errorf("KeygenMessage method %v not found", keygenMessage.Method)
}

// ProcessBroadcastMessage is called when the node receives a message via broadcast (eg. Tendermint)
func (keygenNode *KeygenNode) ProcessBroadcastMessage(keygenMessage KeygenMessage) error {
	defer common.TimeTrack(time.Now(), "processMessage"+keygenMessage.Method)
	keygenNode.Lock()
	defer keygenNode.Unlock()
	logging.Debug(string(keygenNode.NodeDetails.ToNodeDetailsID())[0:8] + " processing broadcast message for keygenMessage " + keygenMessage.Method)

	if keygenMessage.Method == "decide" {
		// if untrusted, request for a proof that the decided message was included in a block

		// parse message
		var keygenMsgDecide KeygenMsgDecide
		err := bijson.Unmarshal(keygenMessage.Data, &keygenMsgDecide)
		if err != nil {
			return err
		}

		// wait for all sharings in decided set to complete
		firstEntry := true
		for {
			if !firstEntry {
				keygenNode.Unlock()
				logging.Debug("Waiting for all sharings in decided set (" + fmt.Sprint(keygenMsgDecide.Keygens) + ") to complete")
				time.Sleep(300 * time.Millisecond)
				keygenNode.Lock()
			} else {
				firstEntry = false
			}
			for _, keygenid := range keygenMsgDecide.Keygens {
				if keygenNode.KeygenStore[keygenid] == nil {
					logging.Info("Waiting for keygenid " + string(keygenid) + " to complete. Still uninitialized.")
					continue
				}
				keygen := keygenNode.KeygenStore[keygenid]
				if keygen.State.Phase != States.Phases.Ended {
					logging.Info("Waiting for keygenid " + string(keygenid) + " to complete. Still at " + string(keygen.State.Phase))
					continue
				}
			}
			break
		}

		if _, found := keygenNode.DKGStore[keygenMsgDecide.SharingID]; !found {
			return errors.New("Sharings for sharingID " + string(keygenMsgDecide.SharingID) + " have not completed yet. DKGStore empty?")
		}

		dkg := keygenNode.DKGStore[keygenMsgDecide.SharingID]
		var abarArray []big.Int
		var abarprimeArray []big.Int
		for _, keygenid := range keygenMsgDecide.Keygens {
			keygen := keygenNode.KeygenStore[keygenid]
			if keygen == nil {
				return errors.New("Could not get keygen reference")
			}
			var keygenIDDetails KeygenIDDetails
			err := keygenIDDetails.FromKeygenID(keygenid)
			if err != nil {
				return err
			}
			abarArray = append(abarArray, keygen.Si)
			abarprimeArray = append(abarprimeArray, keygen.Siprime)
		}
		abar := pvss.SumScalars(abarArray...)
		abarprime := pvss.SumScalars(abarprimeArray...)
		dkg.Si = abar
		dkg.Siprime = abarprime
		var dbarInputPts [][]common.Point
		for _, keygenid := range keygenMsgDecide.Keygens {
			keygen := keygenNode.KeygenStore[keygenid]
			var keygenIDDetails KeygenIDDetails
			err := keygenIDDetails.FromKeygenID(keygenid)
			if err != nil {
				return err
			}
			columnPts := common.GetColumnPoint(keygen.Cbar, 0)
			if len(columnPts) == 0 {
				return errors.New("Cbar is empty for " + string(keygenid) + " Node:" + fmt.Sprint(keygenNode.NodeIndex))
			}
			dbarInputPts = append(dbarInputPts, columnPts)
		}
		dkg.Dbar = pvss.AddCommitments(dbarInputPts...)
		gsi := common.BigIntToPoint(secp256k1.Curve.ScalarBaseMult(dkg.Si.Bytes()))
		hsiprime := common.BigIntToPoint(secp256k1.Curve.ScalarMult(&secp256k1.H.X, &secp256k1.H.Y, dkg.Siprime.Bytes()))
		gsihsiprime := common.BigIntToPoint(secp256k1.Curve.Add(&gsi.X, &gsi.Y, &hsiprime.X, &hsiprime.Y))
		verified := pvss.VerifyShareCommitment(gsihsiprime, dkg.Dbar, *big.NewInt(int64(keygenNode.NodeDetails.Index)))
		if !verified {
			return fmt.Errorf("%v Could not verify shares against interpolated commitments", keygenNode.NodeDetails)
		}
		logging.Debug(string(keygenNode.NodeDetails.ToNodeDetailsID())[0:8] + " verified shares against newly generated commitments.")
		logging.Debug(string(keygenNode.NodeDetails.ToNodeDetailsID())[0:8] + "- Si: " + dkg.Si.Text(16) + ", Siprime: " + dkg.Siprime.Text(16))
		byt, _ := bijson.Marshal(dkg.Dbar)
		logging.Debug(string(keygenNode.NodeDetails.ToNodeDetailsID())[0:8] + "- dbar: " + string(byt))
		go func(msg string) {
			keygenNode.Transport.Output(msg + " generated")
		}(string(keygenMsgDecide.SharingID))

		// send NIZKP
		c, u1, u2 := pvss.GenerateNIZKPK(dkg.Si, dkg.Siprime)
		keygenMsgNIZKP := &KeygenMsgNIZKP{
			SharingID:   keygenMsgDecide.SharingID,
			NodeIndex:   keygenNode.NodeDetails.Index,
			C:           c,
			U1:          u1,
			U2:          u2,
			GSi:         gsi,
			GSiHSiprime: gsihsiprime,
		}
		byt, err = bijson.Marshal(keygenMsgNIZKP)
		if err != nil {
			return err
		}
		nextKeygenMessage := KeygenMessage{
			KeygenID: NullKeygenID,
			Method:   "nizkp",
			Data:     byt,
		}
		for _, currNode := range keygenNode.CurrNodes.Nodes {
			go func(currNode NodeDetails, msg KeygenMessage) {
				err := keygenNode.Transport.Send(currNode, msg)
				if err != nil {
					logging.Info(err.Error())
				}
			}(currNode, nextKeygenMessage)
		}
		return nil
	}

	return errors.New("PssMessage method '" + keygenMessage.Method + "' not found")
}

func NewKeygenNode(
	nodeDetails common.Node,
	currNodeList []common.Node,
	currNodesT int,
	currNodesK int,
	nodeIndex int,
	transport KeygenTransport,
) *KeygenNode {
	newKeygenNode := &KeygenNode{
		NodeDetails: NodeDetails(nodeDetails),
		CurrNodes: NodeNetwork{
			Nodes: mapFromNodeList(currNodeList),
			N:     len(currNodeList),
			T:     currNodesT,
			K:     currNodesK,
		},
		NodeIndex:   nodeIndex,
		ShareStore:  make(map[SharingID]*Sharing),
		DKGStore:    make(map[SharingID]*DKG),
		KeygenStore: make(map[KeygenID]*Keygen),
	}

	transport.SetKeygenNode(newKeygenNode)
	newKeygenNode.Transport = transport
	return newKeygenNode
}
