package keygennofsm

import (
	"errors"
	"fmt"
	"math"
	"math/big"
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
func (keygenNode *KeygenNode) ProcessMessage(senderDetails NodeDetails, keygenMessage KeygenMessage) error {
	// defer common.TimeTrack(time.Now(), "node"+strconv.Itoa(keygenNode.NodeIndex)+"keygen"+string(keygenMessage.KeygenID)+"processMessage"+keygenMessage.Method)
	logging.WithFields(logging.Fields{
		"NodeDetails":         string(keygenNode.NodeDetails.ToNodeDetailsID())[0:8],
		"senderDetails":       string(senderDetails.ToNodeDetailsID())[0:8],
		"keygenMessageMethod": keygenMessage.Method,
	}).Debug("node processing message from sender for keygenMessageMethod")

	// Add to metrics
	telemetry.IncrementCounter(pcmn.TelemetryConstants.Keygen.ProcessMessages, pcmn.TelemetryConstants.Keygen.Prefix)

	// ignore message if keygen is completed
	keygen, complete := keygenNode.KeygenStore.GetOrSetIfNotComplete(keygenMessage.KeygenID, &Keygen{
		KeygenID: keygenMessage.KeygenID,
		State: KeygenState{
			States.Phases.Initial,
			States.ReceivedSend.False,
			States.ReceivedEchoMap(),
			States.ReceivedReadyMap(),
		},
		CStore: make(map[CID]*C),
	})
	if complete {
		return nil
	}
	keygen.Lock()
	defer keygen.Unlock()

	// handle different messages here
	switch keygenMessage.Method {
	case "share":
		// parse message
		var keygenMsgShare KeygenMsgShare
		err := bijson.Unmarshal(keygenMessage.Data, &keygenMsgShare)
		if err != nil {
			logging.WithError(err).Error("could not unmarshal data for keygenMsgShare")
			return err
		}
		err = keygenNode.processShareMessage(keygenMsgShare, keygen)
		if err != nil {
			logging.WithError(err).Error("could not processShareMessage")
			return err
		}
	case "send":
		// parse message
		var keygenMsgSend KeygenMsgSend
		err := bijson.Unmarshal(keygenMessage.Data, &keygenMsgSend)
		if err != nil {
			logging.WithError(err).Error("could not unmarshal data for keygenMsgSend")
			return err
		}
		var keygenIDDetails KeygenIDDetails
		err = keygenIDDetails.FromKeygenID(keygenMessage.KeygenID)
		if err != nil {
			logging.WithError(err).Error("could not get keygenIDDetails from keygenID in keygenMsgSend")
			return err
		}
		err = keygenNode.processSendMessage(keygenMsgSend, keygen, keygenIDDetails, senderDetails)
		if err != nil {
			logging.WithError(err).Error("could not processSendMessage")
			return err
		}
	case "echo":
		// parse message
		defer func() { keygen.State.ReceivedEcho[senderDetails.ToNodeDetailsID()] = States.ReceivedEcho.True }()
		var keygenMsgEcho KeygenMsgEcho
		err := bijson.Unmarshal(keygenMessage.Data, &keygenMsgEcho)
		if err != nil {
			logging.WithError(err).Error("could not unmarshal data for keygenMsgEcho")
			return err
		}
		var keygenIDDetails KeygenIDDetails
		err = keygenIDDetails.FromKeygenID(keygenMsgEcho.KeygenID)
		if err != nil {
			logging.WithError(err).Error("could not get keygenIDDetails from keygenID in keygenMsgEcho")
			return err
		}
		err = keygenNode.processEchoMessage(keygenMsgEcho, keygen, keygenIDDetails, senderDetails)
		if err != nil {
			logging.WithError(err).Error("could not processEchoMessage")
			return err
		}
	case "ready":
		// parse message
		defer func() { keygen.State.ReceivedReady[senderDetails.ToNodeDetailsID()] = States.ReceivedReady.True }()
		var keygenMsgReady KeygenMsgReady
		err := bijson.Unmarshal(keygenMessage.Data, &keygenMsgReady)
		if err != nil {
			logging.WithError(err).Error("could not unmarshal data for keygenMsgReady")
			return err
		}
		var keygenIDDetails KeygenIDDetails
		err = keygenIDDetails.FromKeygenID(keygenMsgReady.KeygenID)
		if err != nil {
			logging.WithError(err).Error("could not get keygenIDDetails from keygenID in keygenMsgReady")
			return err
		}
		err = keygenNode.processReadyMessage(keygenMsgReady, keygen, keygenIDDetails, senderDetails)
		if err != nil {
			logging.WithError(err).Error("could not processReadyMessage")
			return err
		}
	case "complete":
		// parse message
		var keygenMsgComplete KeygenMsgComplete
		err := bijson.Unmarshal(keygenMessage.Data, &keygenMsgComplete)
		if err != nil {
			logging.WithError(err).Error("could not unmarshal data for keygenMsgComplete")
			return err
		}

		// Add to metrics
		telemetry.IncrementCounter(pcmn.TelemetryConstants.Keygen.CompleteMessage, pcmn.TelemetryConstants.Keygen.Prefix)

		var keygenIDDetails KeygenIDDetails
		err = keygenIDDetails.FromKeygenID(keygenMsgComplete.KeygenID)
		if err != nil {
			logging.WithError(err).Error("could not get keygenIDDetails from keygenID in keygenMsgComplete")
		}
		err = keygenNode.processCompleteMessage(keygenMsgComplete, keygenIDDetails, senderDetails)
		if err != nil {
			logging.WithError(err).Error("could not processCompleteMessage")
			return err
		}
	case "nizkp":
		// parse message
		var keygenMsgNIZKP KeygenMsgNIZKP
		err := bijson.Unmarshal(keygenMessage.Data, &keygenMsgNIZKP)
		if err != nil {
			logging.WithError(err).Error("could not unmarshal data for keygenMsgNIZKP")
			return err
		}
		err = keygenNode.processNIZKPMessage(keygenMsgNIZKP, keygen, senderDetails)
		if err != nil {
			logging.WithError(err).Error("could not processNIZKPMessage")
			return err
		}
	default:

		// Add to metrics
		telemetry.IncrementCounter(pcmn.TelemetryConstants.Keygen.InvalidMethod, pcmn.TelemetryConstants.Keygen.Prefix)

		return fmt.Errorf("KeygenMessage method %v not found", keygenMessage.Method)
	}
	return nil

}

// ProcessBroadcastMessage is called when the node receives a message via broadcast (eg. Tendermint)
// It works similar to a router, processing different messages differently based on their associated method.
// Each method handler's code path consists of parsing the message -> logic -> state updates.
// Defer state changes until the end of the function call to ensure that the state is consistent in the handler.
func (keygenNode *KeygenNode) ProcessBroadcastMessage(keygenMessage KeygenMessage) error {
	// defer common.TimeTrack(time.Now(), "node"+strconv.Itoa(keygenNode.NodeIndex)+"keygen"+string(keygenMessage.KeygenID)+"processMessage"+keygenMessage.Method)

	// Add to metrics
	telemetry.IncrementCounter(pcmn.TelemetryConstants.Keygen.ProcessedBroadcastMessages, pcmn.TelemetryConstants.Keygen.Prefix)

	logging.WithFields(logging.Fields{
		"NodeDetails":         string(keygenNode.NodeDetails.ToNodeDetailsID())[0:8],
		"keygenMessageMethod": keygenMessage.Method,
	}).Debug("node processing broadcast message for keygenMessage")

	if keygenMessage.Method == "decide" {
		// if untrusted, request for a proof that the decided message was included in a block

		// parse message
		var keygenMsgDecide KeygenMsgDecide
		err := bijson.Unmarshal(keygenMessage.Data, &keygenMsgDecide)
		if err != nil {
			return err
		}

		// Add to metrics
		telemetry.IncrementCounter(pcmn.TelemetryConstants.Keygen.DecideMessage, pcmn.TelemetryConstants.Keygen.Prefix)

		var dkg *DKG
		var keygens []*Keygen
		var alreadyCleanUp, keygensEnded bool
		// wait for all sharings in decided set to complete
		for {
			dkg, keygens, alreadyCleanUp, keygensEnded = getDKGAndKeygensIfKeygensEnded(keygenNode, keygenMsgDecide.DKGID, keygenMsgDecide.Keygens)
			if alreadyCleanUp {
				logging.WithField("dkgid", keygenMsgDecide.DKGID).Debug("already cleaned up, ignoring message")
				return nil
			}
			if keygensEnded {
				break
			}

			// Add to metrics
			telemetry.IncrementCounter(pcmn.TelemetryConstants.Keygen.DecidedSharingsNotComplete, pcmn.TelemetryConstants.Keygen.Prefix)

			logging.WithField("set", keygenMsgDecide.Keygens).Debug("waiting for all sharings in decided set to complete")
			time.Sleep(1 * time.Second)
		}

		var abarArray []big.Int
		var abarprimeArray []big.Int
		for _, keygen := range keygens {
			abarArray = append(abarArray, keygen.Si)
			abarprimeArray = append(abarprimeArray, keygen.Siprime)
		}
		abar := pvss.SumScalars(abarArray...)
		abarprime := pvss.SumScalars(abarprimeArray...)
		var dbarInputPts [][]common.Point
		for _, keygen := range keygens {
			columnPts := pcmn.GetColumnPoint(keygen.Cbar, 0)
			if len(columnPts) == 0 {
				return errors.New("Cbar is empty for " + string(keygen.KeygenID) + " Node:" + fmt.Sprint(keygenNode.NodeIndex))
			}
			dbarInputPts = append(dbarInputPts, columnPts)
		}
		dkgDbar := pvss.AddCommitments(dbarInputPts...)
		gsi := common.BigIntToPoint(secp256k1.Curve.ScalarBaseMult(abar.Bytes()))
		hsiprime := common.BigIntToPoint(secp256k1.Curve.ScalarMult(&secp256k1.H.X, &secp256k1.H.Y, abarprime.Bytes()))
		gsihsiprime := common.BigIntToPoint(secp256k1.Curve.Add(&gsi.X, &gsi.Y, &hsiprime.X, &hsiprime.Y))
		verified := pvss.VerifyShareCommitment(gsihsiprime, dkgDbar, *big.NewInt(int64(keygenNode.NodeDetails.Index)))
		if !verified {
			return fmt.Errorf("%v Could not verify shares against interpolated commitments", keygenNode.NodeDetails)
		}

		logging.WithFields(logging.Fields{
			"NodeDetails": string(keygenNode.NodeDetails.ToNodeDetailsID())[0:8],
			"abar":        abar.Text(16),
			"abarprime":   abarprime.Text(16),
		}).Debug("node verified shares against newly generated commitments")

		byt, _ := bijson.Marshal(dkgDbar)
		logging.WithFields(logging.Fields{
			"NodeDetails": string(keygenNode.NodeDetails.ToNodeDetailsID())[0:8],
			"dbar":        string(byt),
		}).Debug()
		go func(msg string) {
			keygenNode.Transport.Output(msg + " generated")
		}(string(keygenMsgDecide.DKGID))

		// send NIZKP
		c, u1, u2 := pvss.GenerateNIZKPK(abar, abarprime)
		keygenMsgNIZKP := &KeygenMsgNIZKP{
			DKGID: keygenMsgDecide.DKGID,
			NIZKP: NIZKP{
				NodeIndex:   keygenNode.NodeDetails.Index,
				C:           c,
				U1:          u1,
				U2:          u2,
				GSi:         gsi,
				GSiHSiprime: gsihsiprime,
			},
		}
		byt, err = bijson.Marshal(keygenMsgNIZKP)
		if err != nil {
			return err
		}
		nextKeygenMessage := CreateKeygenMessage(KeygenMessageRaw{
			KeygenID: NullKeygenID,
			Method:   "nizkp",
			Data:     byt,
		})
		for _, currNode := range keygenNode.CurrNodes.Nodes {
			go func(currNode NodeDetails, msg KeygenMessage) {
				err := keygenNode.Transport.Send(currNode, msg)
				if err != nil {
					logging.WithError(err).Info()
				}
			}(currNode, nextKeygenMessage)
		}
		// state updates
		dkg.Lock()
		defer dkg.Unlock()
		dkg.Dbar = dkgDbar
		dkg.Si = abar
		dkg.Siprime = abarprime
		return nil
	}

	return fmt.Errorf("PssMessage method %v not found", keygenMessage.Method)
}

func dkgCleanUp(keygenNode *KeygenNode, dkgID DKGID) error {
	keygenNode.DKGStore.Complete(dkgID)
	keygenNode.ShareStore.Complete(dkgID)
	keygenNode.KeygenStore.Complete(dkgID)
	return nil
}
func noOpCleanUp(*KeygenNode, DKGID) error { return nil }

func NewKeygenNode(
	nodeDetails pcmn.Node,
	currNodeList []pcmn.Node,
	currNodesT int,
	currNodesK int,
	nodeIndex int,
	transport KeygenTransport,
	staggerDelay int,
) *KeygenNode {
	newKeygenNode := &KeygenNode{
		NodeDetails: NodeDetails(nodeDetails),
		CurrNodes: NodeNetwork{
			Nodes: mapFromNodeList(currNodeList),
			N:     len(currNodeList),
			T:     currNodesT,
			K:     currNodesK,
		},
		NodeIndex:    nodeIndex,
		ShareStore:   &ShareStoreSyncMap{},
		DKGStore:     &DKGStoreSyncMap{},
		CleanUp:      dkgCleanUp,
		staggerDelay: staggerDelay,
	}
	newKeygenNode.KeygenStore = &KeygenStoreSyncMap{nodes: &newKeygenNode.CurrNodes}
	transport.Init()
	err := transport.SetKeygenNode(newKeygenNode)
	if err != nil {
		logging.WithError(err).Error("could not set keygen node")
	}
	newKeygenNode.Transport = transport
	return newKeygenNode
}

// getDKGAndKeygensIfKeygensEnded - gets references to dkg and keygens, which are safe to use and won't be affected by cleanup
// if the dkg has already ended and is cleaned up (alreadyCleanUp) or the keygens have ended (keygensEnded) this function
// notifies you in the return
func getDKGAndKeygensIfKeygensEnded(keygenNode *KeygenNode, dkgID DKGID, keygenIDs []KeygenID) (dkg *DKG, keygens []*Keygen, alreadyCleanUp bool, keygensEnded bool) {
	keygensEnded = true
	dkg, found := keygenNode.DKGStore.Get(dkgID)
	if found && dkg == nil {
		alreadyCleanUp = true
		return
	}
	if !found {
		keygensEnded = false
		return
	}
	for _, keygenID := range keygenIDs {
		keygen, found := keygenNode.KeygenStore.Get(keygenID)
		if found && keygen == nil {
			alreadyCleanUp = true
			continue
		}
		if !found {
			keygensEnded = false
			continue
		}
		keygen.Lock()
		if found && keygen.State.Phase != States.Phases.Ended {
			keygensEnded = false
		}
		keygens = append(keygens, keygen)
		keygen.Unlock()
	}
	return
}
