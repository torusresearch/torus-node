package dkgnode

import (
	"fmt"
	"math/big"
	"sort"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	logging "github.com/sirupsen/logrus"
	"github.com/torusresearch/bijson"
	tmcommon "github.com/torusresearch/tendermint/libs/common"
	"github.com/torusresearch/torus-common/common"
	"github.com/torusresearch/torus-common/crypto"
	pcmn "github.com/torusresearch/torus-node/common"
	"github.com/torusresearch/torus-node/config"
	"github.com/torusresearch/torus-node/dealer"
	"github.com/torusresearch/torus-node/keygennofsm"
	"github.com/torusresearch/torus-node/mapping"
	"github.com/torusresearch/torus-node/pss"
	"github.com/torusresearch/torus-node/pvss"
	"github.com/torusresearch/torus-node/telemetry"
)

const MaxFailedPubKeyAssigns = 100

// Validates transactions to be delivered to the BFT. is the master switch for all tx
func (app *ABCIApp) ValidateAndUpdateAndTagBFTTx(bftTx []byte, msgType byte, senderDetails NodeDetails) (bool, *[]tmcommon.KVPair, error) {
	telemetry.IncrementCounter(pcmn.TelemetryConstants.BFTRuleSet.TransactionsCounter, pcmn.TelemetryConstants.BFTRuleSet.Prefix)

	var tags []tmcommon.KVPair
	currEpoch := abciServiceLibrary.EthereumMethods().GetCurrentEpoch()
	currEpochInfo, err := abciServiceLibrary.EthereumMethods().GetEpochInfo(currEpoch, false)
	if err != nil {
		return false, &tags, fmt.Errorf("could not get current epoch with err: %v", err)
	}
	numberOfThresholdNodes := int(currEpochInfo.K.Int64())
	numberOfMaliciousNodes := int(currEpochInfo.T.Int64())

	switch msgType {
	case byte(1): // AssignmentBFTTx
		telemetry.IncrementCounter(pcmn.TelemetryConstants.BFTRuleSet.AssignmentCounter, pcmn.TelemetryConstants.BFTRuleSet.Prefix)

		// no assignments after propose freeze is confirmed
		for _, mappingProposeFreeze := range app.state.MappingProposeFreezes {
			if len(mappingProposeFreeze) >= numberOfThresholdNodes+numberOfMaliciousNodes {
				return false, &tags, errors.New("could not key assign since mapping propose freeze is already confirmed")
			}
		}

		// no assignments until after mapping propose summary is confirmed, for epoch > 1
		if !config.GlobalMutableConfig.GetB("IgnoreEpochForKeyAssign") && currEpoch != 1 {
			mappingProposeSummaryConfirmed := false
			for _, mappingProposeSummary := range app.state.MappingProposeSummarys {
				for _, mapReceivedNodeSummary := range mappingProposeSummary {
					if len(mapReceivedNodeSummary) >= numberOfThresholdNodes {
						mappingProposeSummaryConfirmed = true
					}
				}
			}
			if !mappingProposeSummaryConfirmed {
				return false, &tags, errors.New("could not key assign since no mapping summary has been confirmed")
			}
		}

		logging.Debug("starting assignment bft tx")
		var parsedTx AssignmentBFTTx
		err := bijson.Unmarshal(bftTx, &parsedTx)
		if err != nil {
			logging.WithError(err).Error("AssignmentBFTTx failed")
			return false, &tags, err
		}

		// assign user email to key index
		if app.state.LastUnassignedIndex >= app.state.LastCreatedIndex {
			return false, &tags, errors.New("Last assigned index is exceeding last created index")
		}

		assignedKeyIndex := *big.NewInt(int64(app.state.LastUnassignedIndex))

		// Prepare Data Structs to be stored on state, these should not fail
		keyIndexes, err := app.retrieveVerifierToKeyIndex(parsedTx.Verifier, parsedTx.VerifierID)
		if err != nil {
			// Store verifier into db
			keyIndexes = []big.Int{assignedKeyIndex}
		} else {
			keyIndexes = append(keyIndexes, assignedKeyIndex)
		}
		sort.Slice(keyIndexes, func(a, b int) bool {
			return keyIndexes[a].Cmp(&keyIndexes[b]) == -1
		})
		dkgID := string(keygennofsm.GenerateDKGID(assignedKeyIndex))
		pk := app.state.KeygenPubKeys[dkgID].GS
		if app.state.ConsecutiveFailedPubKeyAssigns == MaxFailedPubKeyAssigns {
			app.state.ConsecutiveFailedPubKeyAssigns = 0
			// increment to lastcreatedindex
			app.state.LastUnassignedIndex = app.state.LastCreatedIndex
			return true, &tags, nil
		} else if pk.X.Cmp(big.NewInt(0)) == 0 || pk.Y.Cmp(big.NewInt(0)) == 0 {
			logging.Error("pubkey not found")
			app.state.ConsecutiveFailedPubKeyAssigns++
			return false, &tags, fmt.Errorf("pubkey not found")
		}
		app.state.ConsecutiveFailedPubKeyAssigns = 0
		verifierMap := make(map[string][]string)
		verifierMap[parsedTx.Verifier] = []string{parsedTx.VerifierID}
		newKeyMapping := KeyAssignmentPublic{
			Index:     assignedKeyIndex,
			PublicKey: pk,
			Threshold: 1,
			Verifiers: verifierMap,
		}

		// Store mappings on db for queries
		err = app.storeKeyMapping(assignedKeyIndex, newKeyMapping)
		if err != nil {
			return false, &tags, fmt.Errorf("Could not storeKeyMapping: %v ", err)
		}
		err = app.storeVerifierToKeyIndex(parsedTx.Verifier, parsedTx.VerifierID, keyIndexes)
		if err != nil {
			return false, &tags, fmt.Errorf("Could not storeVerifierToKeyIndex: %v ", err)
		}

		// increment counters
		app.state.LastUnassignedIndex = app.state.LastUnassignedIndex + 1
		// add to assignment change log
		app.state.NewKeyAssignments = append(app.state.NewKeyAssignments, newKeyMapping)
		// clean up pubkeys generated and stored on-chain from keygen
		delete(app.state.KeygenPubKeys, dkgID)
		// add final tags
		tags = []tmcommon.KVPair{
			{Key: []byte("assignment"), Value: []byte("1")},
		}
		return true, &tags, nil

	case byte(2): // keygennofsm.KeygenMessage
		telemetry.IncrementCounter(pcmn.TelemetryConstants.BFTRuleSet.KeygenMessageCounter, pcmn.TelemetryConstants.BFTRuleSet.Prefix)

		logging.WithField("tx", stringify(bftTx)).Debug("bftruleset received KeygenMessage")
		var keygenMessage = keygennofsm.KeygenMessage{}
		err := bijson.Unmarshal(bftTx, &keygenMessage)
		if err != nil {
			logging.Errorf("keygenMessage unmarshalling failed with error %s", err)
			return false, &tags, err
		}
		logging.WithFields(logging.Fields{
			"pssMessage": stringify(keygenMessage),
			"Data":       string(keygenMessage.Data),
		}).Debug("managed to get keygenMessage")
		if keygenMessage.Method == "propose" {
			telemetry.IncrementCounter(pcmn.TelemetryConstants.BFTRuleSet.KeygenProposeCounter, pcmn.TelemetryConstants.BFTRuleSet.Prefix)
			var keygenMsgPropose keygennofsm.KeygenMsgPropose
			err = bijson.Unmarshal(keygenMessage.Data, &keygenMsgPropose)
			if err != nil {
				return false, &tags, err
			}

			logging.WithField("keygenMsgPropose", keygenMsgPropose).Debug("got keygenMsgPropose")

			if app.state.KeygenDecisions[string(keygenMsgPropose.DKGID)] {
				logging.WithField("keygenMsgPropose", keygenMsgPropose).Debug("keygenMsgPropose rejected, already decided")
				return false, &tags, nil
			}

			logging.WithField("keygenMsgPropose", keygenMsgPropose).Debug("keygenMsgPropose not already decided")

			if len(keygenMsgPropose.Keygens) < numberOfThresholdNodes {
				logging.Error("Propose message did not have enough keygenids")
				return false, &tags, nil
			}

			if len(keygenMsgPropose.Keygens) != len(keygenMsgPropose.ProposeProofs) {
				logging.Error("Propose message had different lengths for keygenids and proposeProofs")
				return false, &tags, nil
			}

			GSHSprime := common.Point{X: *big.NewInt(0), Y: *big.NewInt(0)}
			for i, keygenid := range keygenMsgPropose.Keygens {
				C00Map := make(map[string]int)
				var keygenIDDetails keygennofsm.KeygenIDDetails
				err := keygenIDDetails.FromKeygenID(keygenid)
				if err != nil {
					logging.WithError(err).Error("Could not get keygenIDDetails")
					return false, &tags, nil
				}
				if keygenIDDetails.DKGID != keygenMsgPropose.DKGID {
					logging.Error("SharingID for keygenMsgPropose did not match keygenids")
					return false, &tags, nil
				}
				for nodeDetailsID, proposeProof := range keygenMsgPropose.ProposeProofs[i] {
					var nodeDetails keygennofsm.NodeDetails
					nodeDetails.FromNodeDetailsID(nodeDetailsID)
					// validate node
					foundNode, err := validateNode(
						abciServiceLibrary,
						nodeDetails.PubKey.X,
						nodeDetails.PubKey.Y,
						nodeDetails.Index,
						abciServiceLibrary.EthereumMethods().GetCurrentEpoch(),
					)
					if err != nil {
						return false, &tags, err
					}
					signedTextDetails := keygennofsm.SignedTextDetails{
						Text: strings.Join([]string{string(keygenid), "ready"}, pcmn.Delimiter1),
						C00:  proposeProof.C00,
					}
					verified := pvss.ECDSAVerifyBytes(signedTextDetails.ToBytes(), &foundNode.PubKey, proposeProof.SignedTextDetails)
					if !verified {
						logging.Error("Could not verify signed text")
						return false, &tags, nil
					}
					c00Hex := crypto.PointToEthAddress(proposeProof.C00).Hex()
					C00Map[c00Hex] = C00Map[c00Hex] + 1
					if C00Map[c00Hex] == numberOfThresholdNodes {
						GSHSprime = pvss.SumPoints(GSHSprime, proposeProof.C00)
					}
				}
			}
			// state changes
			go func() {
				keygenMsgDecide := keygennofsm.KeygenMsgDecide{
					DKGID:   keygenMsgPropose.DKGID,
					Keygens: keygenMsgPropose.Keygens,
				}
				data, err := bijson.Marshal(keygenMsgDecide)
				if err != nil {
					logging.WithError(err).Error("Could not marshal keygenMsgDecide")
					return
				}
				err = abciServiceLibrary.KeygennofsmMethods().ReceiveBFTMessage(keygennofsm.CreateKeygenMessage(keygennofsm.KeygenMessageRaw{
					KeygenID: keygennofsm.NullKeygenID,
					Method:   "decide",
					Data:     data,
				}))
				if err != nil {
					logging.WithError(err).Error("Could not send bft message for decided keygen")
					return
				}
			}()
			app.state.KeygenDecisions[string(keygenMsgPropose.DKGID)] = true
			app.state.KeygenPubKeys[string(keygenMsgPropose.DKGID)] = KeygenPubKey{
				DKGID:     string(keygenMsgPropose.DKGID),
				Decided:   true,
				GSHSprime: GSHSprime,
			}
			return true, &tags, nil
		} else if keygenMessage.Method == "pubkey" {
			telemetry.IncrementCounter(pcmn.TelemetryConstants.BFTRuleSet.KeygenPubKeyCounter, pcmn.TelemetryConstants.BFTRuleSet.Prefix)
			var keygenMsgPubKey keygennofsm.KeygenMsgPubKey
			err = bijson.Unmarshal(keygenMessage.Data, &keygenMsgPubKey)
			if err != nil {
				return false, &tags, err
			}
			logging.WithField("keygenMsgPubKey", keygenMsgPubKey).Debug("got keygenMsgPubKey")

			if len(keygenMsgPubKey.PubKeyProofs) != numberOfThresholdNodes {
				logging.WithField("pubKeyProofs", keygenMsgPubKey.PubKeyProofs).Error("Invalid pubkey proofs length")
				return false, &tags, errors.New("Invalid pubkey proofs length")
			}
			keygenPubKeyDecision := app.state.KeygenPubKeys[string(keygenMsgPubKey.DKGID)]

			if !keygenPubKeyDecision.Decided {
				logging.WithField("DKGID", keygenMsgPubKey.DKGID).Error("keygen not decided yet")
				return false, &tags, errors.New("Undecided keygen")
			}

			if keygenPubKeyDecision.GS.X.Cmp(big.NewInt(0)) != 0 || keygenPubKeyDecision.GS.Y.Cmp(big.NewInt(0)) != 0 {
				logging.Error("Already decided on pubkey")
				return false, &tags, errors.New("Already decided on pubkey")
			}

			var pedersenIndexes []int
			var pedersenPoints []common.Point
			for _, nizkp := range keygenMsgPubKey.PubKeyProofs {
				pedersenIndexes = append(pedersenIndexes, nizkp.NodeIndex)
				pedersenPoints = append(pedersenPoints, nizkp.GSiHSiprime)
			}
			GSHSprime := pvss.LagrangeCurvePts(pedersenIndexes, pedersenPoints)
			if GSHSprime.X.Cmp(&keygenPubKeyDecision.GSHSprime.X) != 0 || GSHSprime.Y.Cmp(&keygenPubKeyDecision.GSHSprime.Y) != 0 {
				logging.Error("Lagranged pedersen commitment does not match")
				return false, &tags, errors.New("Lagranged pedersen commitment does not match")
			}
			var indexes []int
			var points []common.Point
			for _, nizkp := range keygenMsgPubKey.PubKeyProofs {
				if !pvss.VerifyNIZKPK(nizkp.C, nizkp.U1, nizkp.U2, nizkp.GSi, nizkp.GSiHSiprime) {
					logging.Error("Could not verify NIZKP")
					return false, &tags, errors.New("Could not verify NIZKP")
				}
				indexes = append(indexes, nizkp.NodeIndex)
				points = append(points, nizkp.GSi)
			}
			//state changes
			GS := pvss.LagrangeCurvePts(indexes, points)
			app.state.KeygenPubKeys[string(keygenMsgPubKey.DKGID)] = KeygenPubKey{
				DKGID:     keygenPubKeyDecision.DKGID,
				Decided:   keygenPubKeyDecision.Decided,
				GSHSprime: keygenPubKeyDecision.GSHSprime,
				GS:        *GS,
			}
			keyIndex, err := keygenMsgPubKey.DKGID.GetIndex()
			if err != nil {
				return false, &tags, err
			}
			err = abciServiceLibrary.DatabaseMethods().StorePublicKeyToIndex(*GS, keyIndex)
			if err != nil {
				logging.Error("Could not store completed keygen pubkey")
				return false, &tags, err
			}
			app.state.LastCreatedIndex = app.state.LastCreatedIndex + uint(1)
			return true, &tags, nil
		}
		return false, &tags, errors.New("tendermint received keygenMessage with unimplemented method:" + keygenMessage.Method)

	case byte(3): // pss.PSSMessage
		telemetry.IncrementCounter(pcmn.TelemetryConstants.BFTRuleSet.PSSMessageCounter, pcmn.TelemetryConstants.BFTRuleSet.Prefix)

		logging.WithField("tx", stringify(bftTx)).Debug("bftruleset received PSSMessage")
		var pssMessage = pss.PSSMessage{}
		err := bijson.Unmarshal(bftTx, &pssMessage)
		if err != nil {
			logging.WithError(err).Error("pssMessage unmarshalling failed with error")
			return false, &tags, err
		}
		logging.WithFields(logging.Fields{
			"pssMessage": stringify(pssMessage),
			"Data":       string(pssMessage.Data),
		}).Debug("managed to get pssMessage")

		if pssMessage.Method == "propose" {
			telemetry.IncrementCounter(pcmn.TelemetryConstants.BFTRuleSet.PSSProposeCounter, pcmn.TelemetryConstants.BFTRuleSet.Prefix)
			var pssMsgPropose pss.PSSMsgPropose
			err = bijson.Unmarshal(pssMessage.Data, &pssMsgPropose)
			if err != nil {
				return false, &tags, err
			}

			logging.WithField("pssMsgPropose", pssMsgPropose).Debug("got pssMsgPropose")

			if app.state.PSSDecisions[string(pssMsgPropose.SharingID)] {
				logging.WithField("pssMsgPropose", pssMsgPropose).Debug("PSSMsgPropose rejected, already decided")
				return false, &tags, nil
			}

			logging.WithField("pssMsgPropose", pssMsgPropose).Debug("pssMsgPropose not already decided")

			epochParams, err := pssMsgPropose.SharingID.GetEpochParams()
			if err != nil {
				return false, &tags, err
			}
			epochOld := epochParams[0]
			kOld := epochParams[2]
			epochNew := epochParams[4]
			kNew := epochParams[6]
			tNew := epochParams[7]
			if kOld == 0 || kNew == 0 {
				return false, &tags, errors.New("k cannot be 0")
			}
			logging.WithFields(logging.Fields{
				"kOld":        kOld,
				"kNew":        kNew,
				"tNew":        tNew,
				"epochParams": epochParams,
			}).Debug()

			if len(pssMsgPropose.PSSs) < kOld {
				return false, &tags, fmt.Errorf("propose message had only %v PSSs, expected %v", len(pssMsgPropose.PSSs), kOld)
			}

			logging.WithField("PSSs", pssMsgPropose.PSSs).Debug("Propose message had enough PSSs")

			if len(pssMsgPropose.PSSs) != len(pssMsgPropose.SignedTexts) {
				return false, &tags, errors.New("Propose message had different lengths for pssids and signedTexts")
			}

			logging.WithField("SignedTexts", pssMsgPropose.SignedTexts).Debug("propose message had enough sets of SignTexts")

			for i, pssid := range pssMsgPropose.PSSs {
				logging.WithField("pssid", pssid).Debug("checking pssid")
				var pssIDDetails pss.PSSIDDetails
				err := pssIDDetails.FromPSSID(pssid)
				if err != nil {
					return false, &tags, err
				}
				if pssIDDetails.SharingID != pssMsgPropose.SharingID {
					return false, &tags, errors.New("SharingID for pssMsgPropose did not match pssids")
				}
				if len(pssMsgPropose.SignedTexts[i]) < tNew+kNew {
					return false, &tags, errors.New("Not enough signed ready texts in proof")
				}
				for nodeDetailsID, signedText := range pssMsgPropose.SignedTexts[i] {
					var nodeDetails pss.NodeDetails
					nodeDetails.FromNodeDetailsID(nodeDetailsID)
					// validate node
					foundNode, err := validateNode(
						abciServiceLibrary,
						nodeDetails.PubKey.X,
						nodeDetails.PubKey.Y,
						nodeDetails.Index,
						abciServiceLibrary.EthereumMethods().GetCurrentEpoch(),
					)
					if err != nil {
						return false, &tags, err
					}
					verified := pvss.ECDSAVerify(
						strings.Join([]string{string(pssid), "ready"}, pcmn.Delimiter1),
						&foundNode.PubKey,
						signedText,
					)
					if !verified {
						return false, &tags, errors.New("Could not verify signed text")
					}
				}
			}

			logging.WithField("PSSs", pssMsgPropose.PSSs).Debug("completed check")
			// state changes
			app.state.PSSDecisions[string(pssMsgPropose.SharingID)] = true
			tags = []tmcommon.KVPair{
				{Key: []byte("psspropose"), Value: []byte("1")},
			}
			logging.WithField("PSSDecisions", app.state.PSSDecisions).Debug("proposed finally")
			pssMsgDecide := pss.PSSMsgDecide{
				SharingID: pssMsgPropose.SharingID,
				PSSs:      pssMsgPropose.PSSs,
			}
			byt, err := bijson.Marshal(pssMsgDecide)
			if err != nil {
				return false, &tags, fmt.Errorf("could not marshal pssMsgDecide %v", err.Error())
			}
			nextPSSMessage := pss.CreatePSSMessage(pss.PSSMessageRaw{
				PSSID:  pss.NullPSSID,
				Method: "decide",
				Data:   byt,
			})
			protocolPrefix := PSSProtocolPrefix("pss" + "-" + strconv.Itoa(epochOld) + "-" + strconv.Itoa(epochNew) + "/")
			go func(prefix PSSProtocolPrefix, pssMsg pss.PSSMessage) {
				err := abciServiceLibrary.PSSMethods().ReceiveBFTMessage(prefix, pssMsg)
				if err != nil {
					logging.WithError(err).Error("could not receive BFT message when sending decide")
					return
				}
			}(protocolPrefix, nextPSSMessage)
			return true, &tags, nil
		}
		return false, &tags, errors.New("tendermint received pssMessage with unimplemented method:" + pssMessage.Method)
	case byte(4): // mapping.MappingMessage
		telemetry.IncrementCounter(pcmn.TelemetryConstants.BFTRuleSet.MappingMessageCounter, pcmn.TelemetryConstants.BFTRuleSet.Prefix)

		logging.WithField("tx", stringify(bftTx)).Debug("bftruleset received MappingMessage")
		var mappingMessage mapping.MappingMessage
		err := bijson.Unmarshal(bftTx, &mappingMessage)
		if err != nil {
			logging.WithError(err).Error("mappingMessage unmarshalling failed")
			return false, &tags, err
		}
		mappingID := mappingMessage.MappingID
		if app.state.MappingThawed[mappingID] {
			return false, &tags, errors.New("mapping is already thawed")
		}
		if mappingMessage.Method == "mapping_propose_freeze" {
			telemetry.IncrementCounter(pcmn.TelemetryConstants.BFTRuleSet.MappingProposeFreezeCounter, pcmn.TelemetryConstants.BFTRuleSet.Prefix)
			var mappingProposeFreezeBroadcastMessage mapping.MappingProposeFreezeBroadcastMessage
			err := bijson.Unmarshal(mappingMessage.Data, &mappingProposeFreezeBroadcastMessage)
			if err != nil {
				logging.WithError(err).Error("could not unmarshal mapping propose freeze broadcast message")
				return false, &tags, err
			}
			proposedMappingID := mappingProposeFreezeBroadcastMessage.MappingID
			if app.state.MappingProposeFreezes[proposedMappingID] == nil {
				app.state.MappingProposeFreezes[proposedMappingID] = make(map[NodeDetailsID]bool)
			}
			if app.state.MappingProposeFreezes[proposedMappingID][senderDetails.ToNodeDetailsID()] {
				return false, &tags, fmt.Errorf("already set to true for incoming mapping propose freeze %v", mappingMessage)
			}
			// state changes
			app.state.MappingProposeFreezes[proposedMappingID][senderDetails.ToNodeDetailsID()] = true
			if len(app.state.MappingProposeFreezes[proposedMappingID]) == numberOfThresholdNodes+numberOfMaliciousNodes {
				go func(proposedMID mapping.MappingID) {
					// continue pss trigger
					// external state updates should be run in goroutines
					abciServiceLibrary.MappingMethods().SetFreezeState(proposedMID, 2, app.state.LastUnassignedIndex)
				}(proposedMappingID)
			}
			return true, &tags, nil
		} else if mappingMessage.Method == "mapping_summary_broadcast" {
			telemetry.IncrementCounter(pcmn.TelemetryConstants.BFTRuleSet.MappingSummaryCounter, pcmn.TelemetryConstants.BFTRuleSet.Prefix)
			var mappingSummaryBroadcastMessage mapping.MappingSummaryBroadcastMessage
			err := bijson.Unmarshal(mappingMessage.Data, &mappingSummaryBroadcastMessage)
			if err != nil {
				logging.WithError(err).Error("could not unmarshal mapping summary broadcast message")
				return false, &tags, err
			}
			if app.state.MappingProposeSummarys[mappingID] == nil {
				app.state.MappingProposeSummarys[mappingID] = make(map[mapping.TransferSummaryID]map[NodeDetailsID]bool)
			}
			if app.state.MappingProposeSummarys[mappingID][mappingSummaryBroadcastMessage.TransferSummary.ID()] == nil {
				app.state.MappingProposeSummarys[mappingID][mappingSummaryBroadcastMessage.TransferSummary.ID()] = make(map[NodeDetailsID]bool)
			}
			if app.state.MappingProposeSummarys[mappingID][mappingSummaryBroadcastMessage.TransferSummary.ID()][senderDetails.ToNodeDetailsID()] {
				return false, &tags, fmt.Errorf("already set to true for incoming mapping propose summary %v", mappingMessage)
			}
			// state changes
			app.state.MappingProposeSummarys[mappingID][mappingSummaryBroadcastMessage.TransferSummary.ID()][senderDetails.ToNodeDetailsID()] = true
			if len(app.state.MappingProposeSummarys[mappingID][mappingSummaryBroadcastMessage.TransferSummary.ID()]) == numberOfThresholdNodes {
				mappingCounter := app.state.MappingCounters[mappingID]
				mappingCounter.RequiredCount = int(mappingSummaryBroadcastMessage.TransferSummary.LastUnassignedIndex)
				app.state.MappingCounters[mappingID] = mappingCounter
				app.state.LastUnassignedIndex = mappingSummaryBroadcastMessage.TransferSummary.LastUnassignedIndex

				// this handles the case where more keys have been generated than keys being transferred via PSS
				// since we will not be able to start keygens again after they have been started
				if app.state.LastCreatedIndex < mappingSummaryBroadcastMessage.TransferSummary.LastUnassignedIndex {
					app.state.LastCreatedIndex = mappingSummaryBroadcastMessage.TransferSummary.LastUnassignedIndex
				}

				for i := 0; i < int(mappingSummaryBroadcastMessage.TransferSummary.LastUnassignedIndex); i++ {
					dkgID := keygennofsm.GenerateDKGID(*big.NewInt(int64(i)))
					if abciServiceLibrary.DatabaseMethods().GetKeygenStarted(string(dkgID)) {
						logging.WithField("dkgID", dkgID).Info("Keygen already started for pss")
						continue
					}
					err = abciServiceLibrary.DatabaseMethods().SetKeygenStarted(string(dkgID), true)
					if err != nil {
						logging.WithError(err).Error("could not write to database")
						continue
					}
				}
				if app.isThawed(mappingID) {
					app.state.MappingThawed[mappingID] = true
				}
			}
			return true, &tags, nil
		} else if mappingMessage.Method == "mapping_key_broadcast" {
			telemetry.IncrementCounter(pcmn.TelemetryConstants.BFTRuleSet.MappingKeyCounter, pcmn.TelemetryConstants.BFTRuleSet.Prefix)
			var mappingKeyBroadcastMessage mapping.MappingKeyBroadcastMessage
			err := bijson.Unmarshal(mappingMessage.Data, &mappingKeyBroadcastMessage)
			if err != nil {
				logging.WithError(err).Error("could not unmarshal mapping key broadcast message")
				return false, &tags, err
			}
			if app.state.MappingProposeKeys[mappingID] == nil {
				app.state.MappingProposeKeys[mappingID] = make(map[mapping.MappingKeyID]map[NodeDetailsID]bool)
			}
			if app.state.MappingProposeKeys[mappingID][mappingKeyBroadcastMessage.MappingKey.ID()] == nil {
				app.state.MappingProposeKeys[mappingID][mappingKeyBroadcastMessage.MappingKey.ID()] = make(map[NodeDetailsID]bool)
			}
			if app.state.MappingProposeKeys[mappingID][mappingKeyBroadcastMessage.MappingKey.ID()][senderDetails.ToNodeDetailsID()] {
				return false, &tags, fmt.Errorf("already set to true for incoming mapping propose message %v", mappingMessage)
			}
			// state changes
			app.state.MappingProposeKeys[mappingID][mappingKeyBroadcastMessage.MappingKey.ID()][senderDetails.ToNodeDetailsID()] = true
			if len(app.state.MappingProposeKeys[mappingID][mappingKeyBroadcastMessage.MappingKey.ID()]) == numberOfThresholdNodes {
				mappingCounter := app.state.MappingCounters[mappingID]
				mappingCounter.KeyCount++
				app.state.MappingCounters[mappingID] = mappingCounter
				mappingKey := mappingKeyBroadcastMessage.MappingKey
				err = abciServiceLibrary.DatabaseMethods().StorePublicKeyToIndex(mappingKey.PublicKey, mappingKey.Index)
				if err != nil {
					logging.WithError(err).Error("could not store key mapping in database")
				}
				err = app.storeKeyMapping(mappingKey.Index, KeyAssignmentPublic{
					Index:     mappingKey.Index,
					PublicKey: mappingKey.PublicKey,
					Threshold: mappingKey.Threshold,
					Verifiers: mappingKey.Verifiers,
				})
				if err != nil {
					logging.WithError(err).Error("could not store key mapping")
				}
				for verifier, verifierIDs := range mappingKey.Verifiers {
					for _, verifierID := range verifierIDs {
						keyIndexes, err := app.retrieveVerifierToKeyIndex(verifier, verifierID)
						if err != nil {
							logging.
								WithField("verifier", verifier).
								WithField("verifierID", verifierID).
								WithError(err).Debug("could not get keyIndexes for verifier and verifierID, might be empty")
						}
						var found bool
						for _, keyIndex := range keyIndexes {
							if keyIndex.Cmp(&mappingKey.Index) == 0 {
								found = true
							}
						}
						if !found {
							keyIndexes = append(keyIndexes, mappingKey.Index)
							sort.Slice(keyIndexes, func(a, b int) bool {
								return keyIndexes[a].Cmp(&keyIndexes[b]) == -1
							})
							err := app.storeVerifierToKeyIndex(verifier, verifierID, keyIndexes)
							if err != nil {
								logging.WithError(err).Error("could not store verifier to key index mapping")
							}
						}
					}
				}
				if app.isThawed(mappingID) {
					app.state.MappingThawed[mappingID] = true
				}
			}
			return true, &tags, nil
		}
		return false, &tags, errors.New("tendermint received mappingMessage with unimplemented method:" + mappingMessage.Method)
	case byte(5): // dealer.Message
		telemetry.IncrementCounter(pcmn.TelemetryConstants.BFTRuleSet.DealerMessageCounter, pcmn.TelemetryConstants.BFTRuleSet.Prefix)

		logging.WithField("tx", stringify(bftTx)).Debug("bftruleset received DealerMessage")
		var dealerMessage dealer.Message
		err := bijson.Unmarshal(bftTx, &dealerMessage)
		if err != nil {
			return false, &tags, errors.New("could not unmarshal dealerMessage")
		}
		keyAssignmentPublic, err := app.retrieveKeyMapping(dealerMessage.KeyIndex)
		if err != nil {
			return false, &tags, fmt.Errorf("could not get keyAssignmentPublic %v %v", dealerMessage.KeyIndex, err.Error())
		}
		if !dealerMessage.Validate(keyAssignmentPublic.PublicKey) {
			return false, &tags, errors.New("could not validate dealer message")
		}
		if dealerMessage.Method == "updatePubKey" {
			var dealerMsgUpdatePublicKey dealer.MsgUpdatePublicKey
			err := bijson.Unmarshal(dealerMessage.Data, &dealerMsgUpdatePublicKey)
			if err != nil {
				return false, &tags, errors.New("could not unmarshal dealerMsgUpdatePublicKey")
			}
			if !dealerMsgUpdatePublicKey.Validate(keyAssignmentPublic.PublicKey) {
				return false, &tags, errors.New("could not validate dealerMsgUpdatePublicKey")
			}
			// state changes
			newKeyAssignmentPublic := KeyAssignmentPublic{
				Index:     keyAssignmentPublic.Index,
				PublicKey: dealerMsgUpdatePublicKey.NewPubKey,
				Threshold: keyAssignmentPublic.Threshold,
				Verifiers: keyAssignmentPublic.Verifiers,
			}
			err = app.storeKeyMapping(newKeyAssignmentPublic.Index, newKeyAssignmentPublic)
			if err != nil {
				return false, &tags, errors.New("could not store key mapping")
			}
			err = abciServiceLibrary.DatabaseMethods().StorePublicKeyToIndex(newKeyAssignmentPublic.PublicKey, newKeyAssignmentPublic.Index)
			if err != nil {
				return false, &tags, errors.New("could not store public key to index")
			}
			telemetry.IncGauge("dealer_update_pub_key")
			return true, &tags, nil
		}
		return false, &tags, errors.New("tendermint received dealerMessage with unimplemented method:" + dealerMessage.Method)
	}
	return false, &tags, errors.New("Tx type not recognized")
}

// Checks if status update is from a valid node in a particular epoch
func validateNode(serviceLibrary ServiceLibrary, x big.Int, y big.Int, index int, epoch int) (*pss.NodeDetails, error) {
	nodeRef := serviceLibrary.EthereumMethods().GetNodeDetailsByEpochAndIndex(epoch, index)
	if nodeRef.PublicKey.X.Cmp(&x) != 0 || nodeRef.PublicKey.Y.Cmp(&y) != 0 {
		return nil, errors.New("could not find node")
	}
	return &pss.NodeDetails{Index: index, PubKey: common.Point{X: *nodeRef.PublicKey.X, Y: *nodeRef.PublicKey.Y}}, nil
}

func (app *ABCIApp) validateTx(bftTx []byte, msgType byte, senderDetails NodeDetails, state *State) (bool, error) {

	telemetry.IncrementCounter(pcmn.TelemetryConstants.BFTRuleSet.TransactionsCounter, pcmn.TelemetryConstants.ABCIApp.CheckTxPrefix)

	currEpoch := abciServiceLibrary.EthereumMethods().GetCurrentEpoch()
	currEpochInfo, err := abciServiceLibrary.EthereumMethods().GetEpochInfo(currEpoch, false)
	if err != nil {
		return false, fmt.Errorf("could not get current epoch with err: %v", err)
	}
	numberOfThresholdNodes := int(currEpochInfo.K.Int64())
	numberOfMaliciousNodes := int(currEpochInfo.T.Int64())

	switch msgType {
	case byte(1): // AssignmentBFTTx
		telemetry.IncrementCounter(pcmn.TelemetryConstants.BFTRuleSet.AssignmentCounter, pcmn.TelemetryConstants.ABCIApp.CheckTxPrefix)

		// no assignments after propose freeze is confirmed
		for _, mappingProposeFreeze := range app.state.MappingProposeFreezes {
			if len(mappingProposeFreeze) >= numberOfThresholdNodes+numberOfMaliciousNodes {
				return false, errors.New("could not key assign since mapping propose freeze is already true")
			}
		}

		// no assignments until after mapping propose summary is confirmed, for epoch > 1
		if !config.GlobalMutableConfig.GetB("IgnoreEpochForKeyAssign") && currEpoch != 1 {
			mappingProposeSummaryConfirmed := false
			for _, mappingProposeSummary := range app.state.MappingProposeSummarys {
				for _, mapReceivedNodeSummary := range mappingProposeSummary {
					if len(mapReceivedNodeSummary) >= numberOfThresholdNodes {
						mappingProposeSummaryConfirmed = true
					}
				}
			}
			if !mappingProposeSummaryConfirmed {
				return false, errors.New("could not key assign since no mapping summary has been confirmed")
			}
		}

		var parsedTx AssignmentBFTTx
		err := bijson.Unmarshal(bftTx, &parsedTx)
		if err != nil {
			logging.WithError(err).Error("AssignmentBFTTx failed")
			return false, err
		}

		// assign user email to key index
		if state.LastUnassignedIndex >= state.LastCreatedIndex {
			return false, errors.New("Last assigned index is exceeding last created index")
		}
		return true, nil

	case byte(2): // keygennofsm.KeygenMessage
		telemetry.IncrementCounter(pcmn.TelemetryConstants.BFTRuleSet.KeygenMessageCounter, pcmn.TelemetryConstants.ABCIApp.CheckTxPrefix)

		logging.WithField("tx", stringify(bftTx)).Debug("bftruleset received KeygenMessage")
		var keygenMessage = keygennofsm.KeygenMessage{}
		err := bijson.Unmarshal(bftTx, &keygenMessage)
		if err != nil {
			logging.Errorf("keygenMessage unmarshalling failed with error %s", err)
			return false, err
		}
		logging.WithFields(logging.Fields{
			"pssMessage": stringify(keygenMessage),
			"Data":       string(keygenMessage.Data),
		}).Debug("managed to get keygenMessage")
		if keygenMessage.Method == "propose" {
			var keygenMsgPropose keygennofsm.KeygenMsgPropose
			err = bijson.Unmarshal(keygenMessage.Data, &keygenMsgPropose)
			if err != nil {
				return false, err
			}

			logging.WithField("keygenMsgPropose", keygenMsgPropose).Debug("got keygenMsgPropose")

			if state.KeygenDecisions[string(keygenMsgPropose.DKGID)] {
				logging.WithField("keygenMsgPropose", keygenMsgPropose).Debug("keygenMsgPropose rejected, already decided")
				return false, nil
			}

			logging.WithField("keygenMsgPropose", keygenMsgPropose).Debug("keygenMsgPropose not already decided")

			if len(keygenMsgPropose.Keygens) < numberOfThresholdNodes {
				logging.Error("Propose message did not have enough keygenids")
				return false, nil
			}

			if len(keygenMsgPropose.Keygens) != len(keygenMsgPropose.ProposeProofs) {
				logging.Error("Propose message had different lengths for keygenids and proposeProofs")
				return false, nil
			}

			GSHSprime := common.Point{X: *big.NewInt(0), Y: *big.NewInt(0)}
			for i, keygenid := range keygenMsgPropose.Keygens {
				C00Map := make(map[string]int)
				var keygenIDDetails keygennofsm.KeygenIDDetails
				err := keygenIDDetails.FromKeygenID(keygenid)
				if err != nil {
					logging.WithError(err).Error("Could not get keygenIDDetails")
					return false, nil
				}
				if keygenIDDetails.DKGID != keygenMsgPropose.DKGID {
					logging.Error("SharingID for keygenMsgPropose did not match keygenids")
					return false, nil
				}
				for nodeDetailsID, proposeProof := range keygenMsgPropose.ProposeProofs[i] {
					var nodeDetails keygennofsm.NodeDetails
					nodeDetails.FromNodeDetailsID(nodeDetailsID)
					// validate node
					foundNode, err := validateNode(
						abciServiceLibrary,
						nodeDetails.PubKey.X,
						nodeDetails.PubKey.Y,
						nodeDetails.Index,
						abciServiceLibrary.EthereumMethods().GetCurrentEpoch(),
					)
					if err != nil {
						return false, err
					}
					signedTextDetails := keygennofsm.SignedTextDetails{
						Text: strings.Join([]string{string(keygenid), "ready"}, pcmn.Delimiter1),
						C00:  proposeProof.C00,
					}
					verified := pvss.ECDSAVerifyBytes(signedTextDetails.ToBytes(), &foundNode.PubKey, proposeProof.SignedTextDetails)
					if !verified {
						logging.Error("Could not verify signed text")
						return false, nil
					}
					c00Hex := crypto.PointToEthAddress(proposeProof.C00).Hex()
					C00Map[c00Hex] = C00Map[c00Hex] + 1
					if C00Map[c00Hex] == numberOfThresholdNodes {
						GSHSprime = pvss.SumPoints(GSHSprime, proposeProof.C00)
					}
				}
			}
			return true, nil
		} else if keygenMessage.Method == "pubkey" {
			var keygenMsgPubKey keygennofsm.KeygenMsgPubKey
			err = bijson.Unmarshal(keygenMessage.Data, &keygenMsgPubKey)
			if err != nil {
				return false, err
			}
			logging.WithField("keygenMsgPubKey", keygenMsgPubKey).Debug("got keygenMsgPubKey")

			if len(keygenMsgPubKey.PubKeyProofs) != numberOfThresholdNodes {
				logging.WithField("pubKeyProofs", keygenMsgPubKey.PubKeyProofs).Error("Invalid pubkey proofs length")
				return false, errors.New("Invalid pubkey proofs length")
			}
			keygenPubKeyDecision := state.KeygenPubKeys[string(keygenMsgPubKey.DKGID)]

			if !keygenPubKeyDecision.Decided {
				logging.WithField("DKGID", keygenMsgPubKey.DKGID).Error("keygen not decided yet")
				return false, errors.New("Undecided keygen")
			}

			if keygenPubKeyDecision.GS.X.Cmp(big.NewInt(0)) != 0 || keygenPubKeyDecision.GS.Y.Cmp(big.NewInt(0)) != 0 {
				logging.Error("Already decided on pubkey")
				return false, errors.New("Already decided on pubkey")
			}

			var pedersenIndexes []int
			var pedersenPoints []common.Point
			for _, nizkp := range keygenMsgPubKey.PubKeyProofs {
				pedersenIndexes = append(pedersenIndexes, nizkp.NodeIndex)
				pedersenPoints = append(pedersenPoints, nizkp.GSiHSiprime)
			}
			GSHSprime := pvss.LagrangeCurvePts(pedersenIndexes, pedersenPoints)
			if GSHSprime.X.Cmp(&keygenPubKeyDecision.GSHSprime.X) != 0 || GSHSprime.Y.Cmp(&keygenPubKeyDecision.GSHSprime.Y) != 0 {
				logging.Error("Lagranged pedersen commitment does not match")
				return false, errors.New("Lagranged pedersen commitment does not match")
			}

			for _, nizkp := range keygenMsgPubKey.PubKeyProofs {
				if !pvss.VerifyNIZKPK(nizkp.C, nizkp.U1, nizkp.U2, nizkp.GSi, nizkp.GSiHSiprime) {
					logging.Error("Could not verify NIZKP")
					return false, errors.New("Could not verify NIZKP")
				}
			}
			return true, nil
		}
		return false, errors.New("tendermint received keygenMessage with unimplemented method:" + keygenMessage.Method)

	case byte(3): // pss.PSSMessage
		telemetry.IncrementCounter(pcmn.TelemetryConstants.BFTRuleSet.PSSMessageCounter, pcmn.TelemetryConstants.ABCIApp.CheckTxPrefix)

		logging.WithField("tx", stringify(bftTx)).Debug("bftruleset received PSSMessage")
		var pssMessage = pss.PSSMessage{}
		err := bijson.Unmarshal(bftTx, &pssMessage)
		if err != nil {
			logging.WithError(err).Error("pssMessage unmarshalling failed with error")
			return false, err
		}
		logging.WithFields(logging.Fields{
			"pssMessage": stringify(pssMessage),
			"Data":       string(pssMessage.Data),
		}).Debug("managed to get pssMessage")

		if pssMessage.Method == "propose" {
			var pssMsgPropose pss.PSSMsgPropose
			err = bijson.Unmarshal(pssMessage.Data, &pssMsgPropose)
			if err != nil {
				return false, err
			}

			logging.WithField("pssMsgPropose", pssMsgPropose).Debug("got pssMsgPropose")

			if state.PSSDecisions[string(pssMsgPropose.SharingID)] {
				logging.WithField("pssMsgPropose", pssMsgPropose).Debug("PSSMsgPropose rejected, already decided")
				return false, nil
			}

			logging.WithField("pssMsgPropose", pssMsgPropose).Debug("pssMsgPropose not already decided")

			epochParams, err := pssMsgPropose.SharingID.GetEpochParams()
			if err != nil {
				return false, err
			}
			kOld := epochParams[2]
			kNew := epochParams[6]
			tNew := epochParams[7]
			if kOld == 0 || kNew == 0 {
				return false, errors.New("k cannot be 0")
			}
			logging.WithFields(logging.Fields{
				"kOld":        kOld,
				"kNew":        kNew,
				"tNew":        tNew,
				"epochParams": epochParams,
			}).Debug()

			if len(pssMsgPropose.PSSs) < kOld {
				return false, fmt.Errorf("propose message had only %v PSSs, expected %v", len(pssMsgPropose.PSSs), kOld)
			}

			logging.WithField("PSSs", pssMsgPropose.PSSs).Debug("Propose message had enough PSSs")

			if len(pssMsgPropose.PSSs) != len(pssMsgPropose.SignedTexts) {
				return false, errors.New("Propose message had different lengths for pssids and signedTexts")
			}

			logging.WithField("SignedTexts", pssMsgPropose.SignedTexts).Debug("propose message had enough sets of SignTexts")

			for i, pssid := range pssMsgPropose.PSSs {
				logging.WithField("pssid", pssid).Debug("checking pssid")
				var pssIDDetails pss.PSSIDDetails
				err := pssIDDetails.FromPSSID(pssid)
				if err != nil {
					return false, err
				}
				if pssIDDetails.SharingID != pssMsgPropose.SharingID {
					return false, errors.New("SharingID for pssMsgPropose did not match pssids")
				}
				if len(pssMsgPropose.SignedTexts[i]) < tNew+kNew {
					return false, errors.New("Not enough signed ready texts in proof")
				}
				for nodeDetailsID, signedText := range pssMsgPropose.SignedTexts[i] {
					var nodeDetails pss.NodeDetails
					nodeDetails.FromNodeDetailsID(nodeDetailsID)
					// validate node
					foundNode, err := validateNode(
						abciServiceLibrary,
						nodeDetails.PubKey.X,
						nodeDetails.PubKey.Y,
						nodeDetails.Index,
						abciServiceLibrary.EthereumMethods().GetCurrentEpoch(),
					)
					if err != nil {
						return false, err
					}
					verified := pvss.ECDSAVerify(
						strings.Join([]string{string(pssid), "ready"}, pcmn.Delimiter1),
						&foundNode.PubKey,
						signedText,
					)
					if !verified {
						return false, errors.New("Could not verify signed text")
					}
				}
			}

			logging.WithField("PSSs", pssMsgPropose.PSSs).Debug("completed check")
			return true, nil
		}
		return false, errors.New("tendermint received pssMessage with unimplemented method:" + pssMessage.Method)
	case byte(4): // mapping.MappingMessage
		telemetry.IncrementCounter(pcmn.TelemetryConstants.BFTRuleSet.MappingMessageCounter, pcmn.TelemetryConstants.ABCIApp.CheckTxPrefix)

		logging.WithField("tx", stringify(bftTx)).Debug("bftruleset received MappingMessage")
		var mappingMessage mapping.MappingMessage
		err := bijson.Unmarshal(bftTx, &mappingMessage)
		if err != nil {
			logging.WithError(err).Error("mappingMessage unmarshalling failed")
			return false, err
		}
		mappingID := mappingMessage.MappingID
		if state.MappingThawed[mappingID] {
			return false, errors.New("mapping is already thawed")
		}
		if mappingMessage.Method == "mapping_propose_freeze" {
			telemetry.IncrementCounter(pcmn.TelemetryConstants.BFTRuleSet.MappingProposeFreezeCounter, pcmn.TelemetryConstants.ABCIApp.CheckTxPrefix)
			var mappingProposeFreezeBroadcastMessage mapping.MappingProposeFreezeBroadcastMessage
			err := bijson.Unmarshal(mappingMessage.Data, &mappingProposeFreezeBroadcastMessage)
			if err != nil {
				logging.WithError(err).Error("could not unmarshal mapping propose freeze broadcast message")
				return false, err
			}
			proposedMappingID := mappingProposeFreezeBroadcastMessage.MappingID
			if state.MappingProposeFreezes[proposedMappingID] != nil {
				if state.MappingProposeFreezes[proposedMappingID][senderDetails.ToNodeDetailsID()] {
					return false, fmt.Errorf("already set to true for incoming mapping propose freeze %v", mappingMessage)
				}
			}
			return true, nil
		} else if mappingMessage.Method == "mapping_summary_broadcast" {
			telemetry.IncrementCounter(pcmn.TelemetryConstants.BFTRuleSet.MappingSummaryCounter, pcmn.TelemetryConstants.ABCIApp.CheckTxPrefix)
			var mappingSummaryBroadcastMessage mapping.MappingSummaryBroadcastMessage
			err := bijson.Unmarshal(mappingMessage.Data, &mappingSummaryBroadcastMessage)
			if err != nil {
				logging.WithError(err).Error("could not unmarshal mapping summary broadcast message")
				return false, err
			}
			if state.MappingProposeSummarys[mappingID] != nil {
				if state.MappingProposeSummarys[mappingID][mappingSummaryBroadcastMessage.TransferSummary.ID()] != nil {
					if state.MappingProposeSummarys[mappingID][mappingSummaryBroadcastMessage.TransferSummary.ID()][senderDetails.ToNodeDetailsID()] {
						return false, fmt.Errorf("already set to true for incoming mapping propose summary %v", mappingMessage)
					}
				}
			}
			return true, nil
		} else if mappingMessage.Method == "mapping_key_broadcast" {
			telemetry.IncrementCounter(pcmn.TelemetryConstants.BFTRuleSet.MappingKeyCounter, pcmn.TelemetryConstants.ABCIApp.CheckTxPrefix)
			var mappingKeyBroadcastMessage mapping.MappingKeyBroadcastMessage
			err := bijson.Unmarshal(mappingMessage.Data, &mappingKeyBroadcastMessage)
			if err != nil {
				logging.WithError(err).Error("could not unmarshal mapping key broadcast message")
				return false, err
			}
			if state.MappingProposeKeys[mappingID] != nil {
				if state.MappingProposeKeys[mappingID][mappingKeyBroadcastMessage.MappingKey.ID()] != nil {
					if state.MappingProposeKeys[mappingID][mappingKeyBroadcastMessage.MappingKey.ID()][senderDetails.ToNodeDetailsID()] {
						return false, fmt.Errorf("already set to true for incoming mapping propose message %v", mappingMessage)
					}
				}
			}
			return true, nil
		}
		return false, errors.New("tendermint received mappingMessage with unimplemented method:" + mappingMessage.Method)
	case byte(5): // dealer.Message
		telemetry.IncrementCounter(pcmn.TelemetryConstants.BFTRuleSet.DealerMessageCounter, pcmn.TelemetryConstants.ABCIApp.CheckTxPrefix)

		logging.WithField("tx", stringify(bftTx)).Debug("bftruleset received DealerMessage")
		var dealerMessage dealer.Message
		err := bijson.Unmarshal(bftTx, &dealerMessage)
		if err != nil {
			return false, errors.New("could not unmarshal dealerMessage")
		}
		keyAssignmentPublic, err := app.retrieveKeyMapping(dealerMessage.KeyIndex)
		if err != nil {
			return false, fmt.Errorf("could not get keyAssignmentPublic %v %v", dealerMessage.KeyIndex, err.Error())
		}
		if !dealerMessage.Validate(keyAssignmentPublic.PublicKey) {
			return false, errors.New("could not validate dealer message")
		}
		if dealerMessage.Method == "updatePubKey" {
			var dealerMsgUpdatePublicKey dealer.MsgUpdatePublicKey
			err := bijson.Unmarshal(dealerMessage.Data, &dealerMsgUpdatePublicKey)
			if err != nil {
				return false, errors.New("could not unmarshal dealerMsgUpdatePublicKey")
			}
			if !dealerMsgUpdatePublicKey.Validate(keyAssignmentPublic.PublicKey) {
				return false, errors.New("could not validate dealerMsgUpdatePublicKey")
			}
			return true, nil
		}
		return false, errors.New("tendermint received dealerMessage with unimplemented method:" + dealerMessage.Method)
	}
	return false, errors.New("Tx type not recognized")
}
