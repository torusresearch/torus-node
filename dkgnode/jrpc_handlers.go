package dkgnode

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"
	"sort"
	"strconv"
	"strings"
	"time"

	pcmn "github.com/torusresearch/torus-node/common"
	"github.com/torusresearch/torus-node/dealer"
	"github.com/torusresearch/torus-node/telemetry"

	tronCrypto "github.com/TRON-US/go-eccrypto"
	logging "github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"
	"github.com/torusresearch/bijson"
	"github.com/torusresearch/jsonrpc"
	tmquery "github.com/torusresearch/tendermint/libs/pubsub/query"
	tmtypes "github.com/torusresearch/tendermint/types"
	"github.com/torusresearch/torus-common/common"
	"github.com/torusresearch/torus-common/crypto"
	"github.com/torusresearch/torus-common/secp256k1"
	"github.com/torusresearch/torus-node/config"
	"github.com/torusresearch/torus-node/eventbus"
)

const requestTimer = 10

func (h CommitmentRequestHandler) ServeJSONRPC(c context.Context, params *bijson.RawMessage) (interface{}, *jsonrpc.Error) {
	telemetry.IncrementCounter(pcmn.TelemetryConstants.JRPC.CommitmentRequestCounter, pcmn.TelemetryConstants.JRPC.Prefix)

	serviceLibrary := NewServiceLibrary(h.eventBus, "commitment_request_handler")
	logging.WithField("params", stringify(params)).Debug("CommitmentRequestHandler")
	var p CommitmentRequestParams
	if err := jsonrpc.Unmarshal(params, &p); err != nil {
		return nil, err
	}
	tokenCommitment := p.TokenCommitment
	verifierIdentifier := p.VerifierIdentifier

	// check if message prefix is correct
	if p.MessagePrefix != "mug00" {
		logging.WithField("params", stringify(params)).Debug("incorrect message prefix")
		return nil, &jsonrpc.Error{Code: -32603, Message: "Internal error", Data: "Incorrect message prefix"}
	}

	found := serviceLibrary.CacheMethods().TokenCommitExists(verifierIdentifier, tokenCommitment)
	if found {
		// allow for loadtesting in debug mode
		if !config.GlobalConfig.IsDebug {
			logging.WithFields(logging.Fields{
				"verifierIdentifier": verifierIdentifier,
				"tokenCommitment":    stringify(tokenCommitment),
			}).Debug("duplicate token found")
			return nil, &jsonrpc.Error{Code: -32603, Message: "Internal error", Data: "Duplicate token found"}
		}
	}

	tempPubKey := common.Point{
		X: *common.HexToBigInt(p.TempPubX),
		Y: *common.HexToBigInt(p.TempPubY),
	}

	serviceLibrary.CacheMethods().RecordTokenCommit(verifierIdentifier, tokenCommitment, tempPubKey)

	// sign data
	commitmentRequestResultData := CommitmentRequestResultData{
		p.MessagePrefix,
		p.TokenCommitment,
		p.TempPubX,
		p.TempPubY,
		p.VerifierIdentifier,
		strconv.FormatInt(time.Now().Unix(), 10),
	}

	logging.WithField("CURRENTTIME", strconv.FormatInt(time.Now().Unix(), 10)).Debug()

	k := serviceLibrary.EthereumMethods().GetSelfPrivateKey()
	pk := serviceLibrary.EthereumMethods().GetSelfPublicKey()
	sig := crypto.SignData([]byte(commitmentRequestResultData.ToString()), crypto.BigIntToECDSAPrivateKey(k))
	res := CommitmentRequestResult{
		Signature: crypto.SigToHex(sig),
		Data:      commitmentRequestResultData.ToString(),
		NodePubX:  pk.X.Text(16),
		NodePubY:  pk.Y.Text(16),
	}
	logging.WithField("CommitmentRequestResult", stringify(res)).Debug()
	return res, nil
}

func (h PingHandler) ServeJSONRPC(c context.Context, params *bijson.RawMessage) (interface{}, *jsonrpc.Error) {
	telemetry.IncrementCounter(pcmn.TelemetryConstants.JRPC.PingRequestCounter, pcmn.TelemetryConstants.JRPC.Prefix)

	serviceLibrary := NewServiceLibrary(h.eventBus, "ping_handler")
	var p PingParams
	if err := jsonrpc.Unmarshal(params, &p); err != nil {
		return nil, err
	}
	return PingResult{
		Message: serviceLibrary.EthereumMethods().GetSelfAddress().Hex(),
	}, nil
}

func (h ConnectionDetailsHandler) ServeJSONRPC(c context.Context, params *bijson.RawMessage) (interface{}, *jsonrpc.Error) {
	telemetry.IncrementCounter(pcmn.TelemetryConstants.JRPC.ConnectionDetailsCounter, pcmn.TelemetryConstants.JRPC.Prefix)
	logging.WithField("connectiondetailsparams", string(*params)).Debug("connection details handler handling request")
	serviceLibrary := NewServiceLibrary(h.eventBus, "connection_details_handler")
	var p ConnectionDetailsParams
	if err := jsonrpc.Unmarshal(params, &p); err != nil {
		return nil, err
	}
	if !serviceLibrary.EthereumMethods().ValidateEpochPubKey(p.ConnectionDetailsMessage.NodeAddress, common.Point{X: p.PubKeyX, Y: p.PubKeyY}) {
		return nil, &jsonrpc.Error{Code: -32602, Message: "Internal error", Data: "Invalid pub key for provided epoch"}
	}
	valid, err := p.ConnectionDetailsMessage.Validate(p.PubKeyX, p.PubKeyY, p.Signature)
	if err != nil {
		return nil, &jsonrpc.Error{Code: -32602, Message: "Internal error", Data: fmt.Sprintf("Could not validate connection details message, err: %v", err)}
	}
	if !valid {
		return nil, &jsonrpc.Error{Code: -32602, Message: "Internal error", Data: "invalid connection details message"}
	}
	return ConnectionDetailsResult{
		TMP2PConnection: serviceLibrary.EthereumMethods().GetTMP2PConnection(),
		P2PConnection:   serviceLibrary.EthereumMethods().GetP2PConnection(),
	}, nil
}

// checks id for assignment and then the auth token for verification
// returns the node's share of the user's key
func (h ShareRequestHandler) ServeJSONRPC(c context.Context, params *bijson.RawMessage) (interface{}, *jsonrpc.Error) {
	telemetry.IncrementCounter(pcmn.TelemetryConstants.JRPC.ShareRequestCounter, pcmn.TelemetryConstants.JRPC.Prefix)

	serviceLibrary := NewServiceLibrary(h.eventBus, "share_request_handler")
	currEpoch := abciServiceLibrary.EthereumMethods().GetCurrentEpoch()
	currEpochInfo, err := abciServiceLibrary.EthereumMethods().GetEpochInfo(currEpoch, false)
	if err != nil {
		return nil, &jsonrpc.Error{Code: -32602, Message: "Internal error", Data: "Error occurred while current epoch"}
	}

	logging.WithField("params", stringify(params)).Debug("ShareRequestHandler")
	var p ShareRequestParams
	if err := jsonrpc.Unmarshal(params, &p); err != nil {
		return nil, err
	}
	logging.WithField("params", stringify(p)).Debug("ShareRequesthandler after parsing params")

	numberOfThresholdNodes := int(currEpochInfo.K.Int64())
	allKeyIndexes := make(map[string]big.Int)    // String keyindex => keyindex
	allValidVerifierIDs := make(map[string]bool) // verifier + pcmn.Delimiter1 + verifierIDs => bool
	var pubKey common.Point

	// For Each VerifierItem we check its validity
	for _, rawItem := range p.Item {
		var parsedVerifierParams ShareRequestItem
		err := bijson.Unmarshal(rawItem, &parsedVerifierParams)
		if err != nil {
			return nil, &jsonrpc.Error{Code: -32602, Message: "Internal error", Data: "Error occurred while parsing sharerequestitem"}
		}
		logging.WithField("PARSEDVERIFIERPARAMS", stringify(parsedVerifierParams)).Debug()
		// verify token validity against verifier, do not pass along nodesignatures
		jsonMap := make(map[string]interface{})
		err = bijson.Unmarshal(rawItem, &jsonMap)
		if err != nil {
			return nil, &jsonrpc.Error{Code: -32602, Message: "Internal error", Data: "Error occurred while parsing jsonmap"}
		}
		delete(jsonMap, "nodesignatures")
		redactedRawItem, err := bijson.Marshal(jsonMap)
		if err != nil {
			return nil, &jsonrpc.Error{Code: -32602, Message: "Internal error", Data: "Error occurred while marshalling" + err.Error()}
		}
		verified, verifierID, err := serviceLibrary.VerifierMethods().Verify((*bijson.RawMessage)(&redactedRawItem))
		if err != nil {
			return nil, &jsonrpc.Error{Code: -32602, Message: "Internal error", Data: "Error occurred while verifying params" + err.Error()}
		}

		if !verified {
			return nil, &jsonrpc.Error{Code: -32602, Message: "Internal error", Data: "Could not verify params"}
		}
		// Validate signatures
		var validSignatures []ValidatedNodeSignature
		for i := 0; i < len(parsedVerifierParams.NodeSignatures); i++ {
			logging.WithField("NODESIGNATURE", stringify(parsedVerifierParams.NodeSignatures[i])).Debug()
			nodeRef, err := parsedVerifierParams.NodeSignatures[i].NodeValidation(h.eventBus)
			if err == nil {
				validSignatures = append(validSignatures, ValidatedNodeSignature{
					parsedVerifierParams.NodeSignatures[i],
					*nodeRef.Index,
				})
			} else {
				logging.WithError(err).Error("could not validate signatures")
			}
		}
		// Check if we have threshold number of signatures
		if len(validSignatures) < numberOfThresholdNodes {
			return nil, &jsonrpc.Error{Code: -32602, Message: "Internal error", Data: "Not enough valid signatures. Only " + strconv.Itoa(len(validSignatures)) + "valid signatures found."}
		}
		// Find common data string, and filter valid signatures on the wrong data
		// this is to prevent nodes from submitting valid signatures on wrong data
		commonDataMap := make(map[string]int)
		for i := 0; i < len(validSignatures); i++ {
			var commitmentRequestResultData CommitmentRequestResultData
			ok, err := commitmentRequestResultData.FromString(validSignatures[i].Data)
			if !ok || err != nil {
				logging.WithField("ok", ok).WithError(err).Error("could not get commitmentRequestResultData from string")
			}
			stringData := strings.Join([]string{
				commitmentRequestResultData.MessagePrefix,
				commitmentRequestResultData.TokenCommitment,
				commitmentRequestResultData.VerifierIdentifier,
			}, pcmn.Delimiter1)
			commonDataMap[stringData]++
		}
		var commonDataString string
		var commonDataCount int
		for k, v := range commonDataMap {
			if v > commonDataCount {
				commonDataString = k
			}
		}
		var validCommonSignatures []ValidatedNodeSignature
		for i := 0; i < len(validSignatures); i++ {
			var commitmentRequestResultData CommitmentRequestResultData
			ok, err := commitmentRequestResultData.FromString(validSignatures[i].Data)
			if !ok || err != nil {
				logging.WithField("ok", ok).WithError(err).Error("could not get commitmentRequestResultData from string")
			}
			stringData := strings.Join([]string{
				commitmentRequestResultData.MessagePrefix,
				commitmentRequestResultData.TokenCommitment,
				commitmentRequestResultData.VerifierIdentifier,
			}, pcmn.Delimiter1)
			if stringData == commonDataString {
				validCommonSignatures = append(validCommonSignatures, validSignatures[i])
			}
		}
		if len(validCommonSignatures) < numberOfThresholdNodes {
			return nil, &jsonrpc.Error{Code: -32602, Message: "Internal error", Data: "Not enough valid signatures on the same data, " + strconv.Itoa(len(validCommonSignatures)) + " valid signatures."}
		}

		commonData := strings.Split(commonDataString, pcmn.Delimiter1)

		if len(commonData) != 3 {
			return nil, &jsonrpc.Error{Code: -32602, Message: "Internal error", Data: "Could not parse common data"}
		}

		commonTokenCommitment := commonData[1]
		commonVerifierIdentifier := commonData[2]

		// Lookup verifier and
		// verify that hash of token = tokenCommitment
		cleanedToken, err := serviceLibrary.VerifierMethods().CleanToken(commonVerifierIdentifier, parsedVerifierParams.IDToken)
		if err != nil {
			return nil, &jsonrpc.Error{Code: -32602, Message: "Internal error", Data: "Error when cleaning token " + err.Error()}
		}
		if hex.EncodeToString(secp256k1.Keccak256([]byte(cleanedToken))) != commonTokenCommitment {
			return nil, &jsonrpc.Error{Code: -32602, Message: "Internal error", Data: "Token commitment and token are not compatible"}
		}

		keyIndexes, err := serviceLibrary.ABCIMethods().GetIndexesFromVerifierID(commonVerifierIdentifier, verifierID)
		if err != nil {
			return nil, &jsonrpc.Error{Code: -32603, Message: "Internal error", Data: fmt.Sprintf("share request could not retrieve keyIndexes: %v", err)}
		}

		// Add to overall list and valid verifierIDs
		for _, index := range keyIndexes {
			allKeyIndexes[index.Text(16)] = index
		}

		pubKey = serviceLibrary.CacheMethods().GetTokenCommitKey(commonVerifierIdentifier, commonTokenCommitment)

		allValidVerifierIDs[strings.Join([]string{parsedVerifierParams.VerifierIdentifier, verifierID}, pcmn.Delimiter1)] = true
	}

	response := ShareRequestResult{}
	var allKeyIndexesSorted []big.Int
	for _, index := range allKeyIndexes {
		allKeyIndexesSorted = append(allKeyIndexesSorted, index)
	}
	sort.Slice(allKeyIndexesSorted, func(a, b int) bool {
		return allKeyIndexesSorted[a].Cmp(&allKeyIndexesSorted[b]) == -1
	})
	for _, index := range allKeyIndexesSorted {
		// check if we have enough validTokens according to Access Structure
		pubKeyAccessStructure, err := serviceLibrary.ABCIMethods().RetrieveKeyMapping(index)
		if err != nil {
			return nil, &jsonrpc.Error{Code: -32603, Message: "Internal error", Data: fmt.Sprintf("could not retrieve access structure: %v", err)}
		}
		validCount := 0
		for verifieridentifier, verifierIDs := range pubKeyAccessStructure.Verifiers {
			for _, verifierID := range verifierIDs {
				// check against all validVerifierIDs
				for verifierStrings := range allValidVerifierIDs {
					if strings.Join([]string{verifieridentifier, verifierID}, pcmn.Delimiter1) == verifierStrings {
						validCount++
					}
				}
			}
		}

		si, _, err := serviceLibrary.DatabaseMethods().RetrieveCompletedShare(index)
		if err != nil {
			return nil, &jsonrpc.Error{Code: -32603, Message: "Internal error", Data: "could not retrieve completed share"}
		}

		if validCount >= pubKeyAccessStructure.Threshold { // if we have enough authenticators we return Si

			keyAssignment := KeyAssignment{
				KeyAssignmentPublic: pubKeyAccessStructure,
				Share:               si.Bytes(),
			}
			if config.GlobalMutableConfig.GetB("EncryptShares") {
				pubKeyHex := "04" + fmt.Sprintf("%064s", pubKey.X.Text(16)) + fmt.Sprintf("%064s", pubKey.Y.Text(16))
				encrypted, metadata, err := tronCrypto.Encrypt(pubKeyHex, keyAssignment.Share)
				if err != nil {
					return nil, &jsonrpc.Error{Code: -32603, Message: "Internal error", Data: fmt.Sprintf("could not encrypt shares with err: %v", err)}
				}

				if metadata == nil {
					return nil, &jsonrpc.Error{Code: -32603, Message: "Internal error", Data: "could not encrypt shares, metadata nil"}
				}

				keyAssignment.Share = []byte(encrypted)
				keyAssignment.Metadata = *metadata
			}
			response.Keys = append(response.Keys, keyAssignment)
		}
	}
	return response, nil
}

func assignKey(c context.Context, eventBus eventbus.Bus, verifier string, verifierID string) *jsonrpc.Error {
	serviceLibrary := NewServiceLibrary(eventBus, "key_assign_handler")
	requestContext, requestContextCancel := context.WithTimeout(c, time.Duration(requestTimer)*time.Second)
	defer requestContextCancel()

	logging.Debug("checking if verifierID is provided")
	if verifierID == "" {
		return &jsonrpc.Error{Code: -32602, Message: "Input error", Data: "VerifierID is empty"}
	}

	logging.Debug("checking if verifier is supported")
	// check if verifier is valid
	verifiers := serviceLibrary.VerifierMethods().ListVerifiers()
	found := false
	for _, v := range verifiers {
		if v == verifier {
			found = true
			break
		}
	}

	if !found {
		return &jsonrpc.Error{Code: -32602, Message: "Input error", Data: "Verifier not supported"}
	}

	logging.Debug("broadcasting assignment transaction")
	// new assignment
	// broadcast assignment transaction
	assMsg := AssignmentBFTTx{VerifierID: verifierID, Verifier: verifier}
	hash, err := serviceLibrary.TendermintMethods().Broadcast(assMsg)
	if err != nil {
		return &jsonrpc.Error{Code: -32603, Message: "Internal error", Data: "Unable to broadcast: " + err.Error()}
	}

	// subscribe to updates
	logging.Debug("subscribing to updates")
	query := tmquery.MustParse("tx.hash='" + hash.String() + "'")
	logging.WithField("queryString", query.String()).Debug("BFTWS")

	responseCh, err := serviceLibrary.TendermintMethods().RegisterQuery(query.String(), 1)
	if err != nil {
		logging.WithField("queryString", query.String()).Debug("BFTWS could not register query")
	}

	var tmpJrpcErr jsonrpc.Error
	for {
		responseReceived := false
		select {
		case e := <-responseCh:
			if err != nil {
				tmpJrpcErr = jsonrpc.Error{Code: -32603, Message: "Internal error", Data: fmt.Sprintf("unable to parse websocket tm response")}
			}
			logging.WithField("gjson", gjson.GetBytes(e, "query").String()).Debug("BFTWS")
			if gjson.GetBytes(e, "query").String() != query.String() {
				break
			}

			parsed := gjson.GetBytes(e, "data")
			parsedTwo := gjson.GetBytes([]byte(parsed.Raw), "value")
			parsedThree := gjson.GetBytes([]byte(parsedTwo.Raw), "TxResult")
			var txResult tmtypes.TxResult
			err = bijson.Unmarshal([]byte(parsedThree.Raw), &txResult)

			if txResult.Result.Code != 0 {
				logging.WithFields(logging.Fields{
					"txQuery": query.String(),
					"code":    txResult.Result.GetCode(),
				}).Debug("BFTWS Got response")
				break
			}

			responseReceived = true
		case <-requestContext.Done():
			tmpJrpcErr = jsonrpc.Error{Code: -32603, Message: "Internal error", Data: fmt.Sprintf("key assignment error: timed out")}
			responseReceived = true
		}

		if responseReceived {
			break
		}
	}

	if tmpJrpcErr != (jsonrpc.Error{}) {
		return &tmpJrpcErr
	}

	return nil
}

func retrieveKeysFromVerifierID(c context.Context, eventBus eventbus.Bus, verifier string, verifierID string) ([]KeyAssignItem, *jsonrpc.Error) {
	serviceLibrary := NewServiceLibrary(eventBus, "key_assign_handler")

	keyIndexes, err := serviceLibrary.ABCIMethods().GetIndexesFromVerifierID(verifier, verifierID)
	if err != nil {
		return nil, &jsonrpc.Error{Code: -32603, Message: "Internal error", Data: fmt.Sprintf("Unable to retieve keyIndexes error: %v", err)}
	}

	var keys []KeyAssignItem

	for _, index := range keyIndexes {
		pk, err := serviceLibrary.DatabaseMethods().RetrieveIndexToPublicKey(index)
		if err != nil {
			return nil, &jsonrpc.Error{Code: -32603, Message: fmt.Sprintf("Could not find address to key index error: %v", err)}
		}
		//form address eth
		addr := crypto.PointToEthAddress(pk)
		keys = append(keys, KeyAssignItem{
			KeyIndex: index.Text(16),
			PubKeyX:  pk.X,
			PubKeyY:  pk.Y,
			Address:  addr.String(),
		})
	}

	return keys, nil
}

// assigns a user a secret, returns the same index if the user has been previously assigned
func (h KeyAssignHandler) ServeJSONRPC(c context.Context, params *bijson.RawMessage) (interface{}, *jsonrpc.Error) {
	telemetry.IncrementCounter(pcmn.TelemetryConstants.JRPC.KeyAssignCounter, pcmn.TelemetryConstants.JRPC.Prefix)

	var p KeyAssignParams
	if err := jsonrpc.Unmarshal(params, &p); err != nil {
		return nil, err
	}

	tmpJrpcErr := assignKey(c, h.eventBus, p.Verifier, p.VerifierID)
	if tmpJrpcErr != nil {
		return nil, tmpJrpcErr
	}

	keys, tmpJrpcErr := retrieveKeysFromVerifierID(c, h.eventBus, p.Verifier, p.VerifierID)
	if tmpJrpcErr != nil {
		return nil, tmpJrpcErr
	}

	return KeyAssignResult{Keys: keys}, nil
}

// Looksup Keys assigned to a Verifier + VerifierID
func (h VerifierLookupHandler) ServeJSONRPC(c context.Context, params *bijson.RawMessage) (interface{}, *jsonrpc.Error) {
	telemetry.IncrementCounter(pcmn.TelemetryConstants.JRPC.VerifierLookupCounter, pcmn.TelemetryConstants.JRPC.Prefix)

	serviceLibrary := NewServiceLibrary(h.eventBus, "verifier_lookup_handler")
	var p VerifierLookupParams
	if err := jsonrpc.Unmarshal(params, &p); err != nil {
		return nil, err
	}
	if p.VerifierID == "" {
		return nil, &jsonrpc.Error{Code: -32602, Message: "Input error", Data: "VerifierID is empty"}
	}
	// check if verifier is valid
	verifiers := serviceLibrary.VerifierMethods().ListVerifiers()
	found := false
	for _, verifier := range verifiers {
		if verifier == p.Verifier {
			found = true
		}
	}
	if !found {
		return nil, &jsonrpc.Error{Code: -32602, Message: "Input error", Data: "Verifier not supported"}
	}
	// retrieve index
	keyIndexes, err := serviceLibrary.ABCIMethods().GetIndexesFromVerifierID(p.Verifier, p.VerifierID)
	if err != nil {
		return nil, &jsonrpc.Error{Code: -32602, Message: "Input Error", Data: fmt.Sprintf("Verifier + VerifierID has not yet been assigned %v", err)}
	}

	// prepare and send response
	result := VerifierLookupResult{}
	for _, index := range keyIndexes {
		publicKeyAss, err := serviceLibrary.ABCIMethods().RetrieveKeyMapping(index)
		if err != nil {
			return nil, &jsonrpc.Error{Code: -32603, Message: fmt.Sprintf("Could not find address to key index error: %v", err)}
		}
		pk := publicKeyAss.PublicKey
		//form address eth
		addr := crypto.PointToEthAddress(pk)
		result.Keys = append(result.Keys, VerifierLookupItem{
			KeyIndex: index.Text(16),
			PubKeyX:  pk.X,
			PubKeyY:  pk.Y,
			Address:  addr.String(),
		})
	}

	return result, nil
}

// Looksup Verifier + VerifierIDs assigned to a key (access structure)
func (h KeyLookupHandler) ServeJSONRPC(c context.Context, params *bijson.RawMessage) (interface{}, *jsonrpc.Error) {
	telemetry.IncrementCounter(pcmn.TelemetryConstants.JRPC.KeyLookupCounter, pcmn.TelemetryConstants.JRPC.Prefix)

	serviceLibrary := NewServiceLibrary(h.eventBus, "key_lookup_handler")
	var p KeyLookupParams
	if err := jsonrpc.Unmarshal(params, &p); err != nil {
		return nil, err
	}
	if p.PubKeyX.Text(16) == "0" || p.PubKeyY.Text(16) == "0" {
		return nil, &jsonrpc.Error{Code: -32602, Message: "Input error", Data: "pubkey is empty"}
	}

	pubKey := common.BigIntToPoint(&p.PubKeyX, &p.PubKeyY)

	// prepare and send response
	keyIndex, err := serviceLibrary.DatabaseMethods().RetrievePublicKeyToIndex(pubKey)
	if err != nil {
		return nil, &jsonrpc.Error{Code: -32602, Message: "Input error", Data: "no log of public key"}
	}
	keyAssignmentPublic, err := serviceLibrary.ABCIMethods().RetrieveKeyMapping(keyIndex)
	if err != nil {
		return nil, &jsonrpc.Error{Code: -32602, Message: "Input error", Data: "no key assignment found for public key"}
	}

	return KeyLookupResult{keyAssignmentPublic}, nil
}

func (h UpdatePublicKeyHandler) ServeJSONRPC(c context.Context, params *bijson.RawMessage) (interface{}, *jsonrpc.Error) {
	telemetry.IncrementCounter(pcmn.TelemetryConstants.JRPC.UpdatePublicKeyCounter, pcmn.TelemetryConstants.JRPC.Prefix)

	serviceLibrary := NewServiceLibrary(h.eventBus, "update_public_key_handler")
	var dealerMessage dealer.Message
	if err := jsonrpc.Unmarshal(params, &dealerMessage); err != nil {
		return nil, err
	}

	existingPubKey, err := serviceLibrary.DatabaseMethods().RetrieveIndexToPublicKey(dealerMessage.KeyIndex)
	if err != nil {
		return nil, &jsonrpc.Error{Code: -32602, Message: err.Error(), Data: "error while getting the existing public key"}
	}
	if !dealerMessage.Validate(existingPubKey) {
		return nil, &jsonrpc.Error{Code: -32602, Message: "validation error", Data: "invalid dealer message"}
	}

	_, err = serviceLibrary.TendermintMethods().Broadcast(dealerMessage)
	if err != nil {
		return nil, &jsonrpc.Error{Code: -32602, Message: err.Error(), Data: "error while doing the tendermint broadcast"}
	}

	return DealerResult{
		Code:    200,
		Message: "public key updated",
	}, nil
}

func (h UpdateShareHandler) ServeJSONRPC(c context.Context, params *bijson.RawMessage) (interface{}, *jsonrpc.Error) {
	telemetry.IncrementCounter(pcmn.TelemetryConstants.JRPC.UpdateShareCounter, pcmn.TelemetryConstants.JRPC.Prefix)

	serviceLibrary := NewServiceLibrary(h.eventBus, "update_public_key_handler")
	var dealerMessage dealer.Message
	if err := jsonrpc.Unmarshal(params, &dealerMessage); err != nil {
		return nil, err
	}

	existingPubKey, err := serviceLibrary.DatabaseMethods().RetrieveIndexToPublicKey(dealerMessage.KeyIndex)
	if err != nil {
		return nil, &jsonrpc.Error{Code: -32602, Message: err.Error(), Data: "error while getting the existing public key"}
	}
	if !dealerMessage.Validate(existingPubKey) {
		return nil, &jsonrpc.Error{Code: -32602, Message: "validation error", Data: "Invalid dealer message"}
	}

	var updateShareMessage dealer.MsgUpdateShare
	err = bijson.Unmarshal(dealerMessage.Data, &updateShareMessage)
	if err != nil {
		return nil, &jsonrpc.Error{Code: -32602, Message: err.Error(), Data: "error while unmarshalling dealerMessage.Data into dealer.MsgUpdateShare"}
	}
	if !updateShareMessage.Validate() {
		return nil, &jsonrpc.Error{Code: -32602, Message: "validation error", Data: "invalid dealer.MsgUpdateShare"}
	}

	err = serviceLibrary.DatabaseMethods().StoreCompletedPSSShare(updateShareMessage.KeyIndex, updateShareMessage.Si, updateShareMessage.Siprime)
	if err != nil {
		return nil, &jsonrpc.Error{Code: -32602, Message: err.Error(), Data: "error while storing completed PSS share"}
	}
	telemetry.IncGauge("dealer_update_share")
	return DealerResult{
		Code:    200,
		Message: "share updated",
	}, nil
}

func (h UpdateCommitmentHandler) ServeJSONRPC(c context.Context, params *bijson.RawMessage) (interface{}, *jsonrpc.Error) {
	telemetry.IncrementCounter(pcmn.TelemetryConstants.JRPC.UpdateCommitmentCounter, pcmn.TelemetryConstants.JRPC.Prefix)

	serviceLibrary := NewServiceLibrary(h.eventBus, "update_public_key_handler")
	var dealerMessage dealer.Message
	if err := jsonrpc.Unmarshal(params, &dealerMessage); err != nil {
		return nil, err
	}

	existingPubKey, err := serviceLibrary.DatabaseMethods().RetrieveIndexToPublicKey(dealerMessage.KeyIndex)
	if err != nil {
		return nil, &jsonrpc.Error{Code: -32602, Message: err.Error(), Data: "error while getting the existing public key"}
	}

	if !dealerMessage.Validate(existingPubKey) {
		return nil, &jsonrpc.Error{Code: -32602, Message: "public Key Validation error", Data: "invalid public key"}
	}

	var updateCommitmentMessage dealer.MsgUpdateCommitment
	err = bijson.Unmarshal(dealerMessage.Data, &updateCommitmentMessage)
	if err != nil {
		return nil, &jsonrpc.Error{Code: -32602, Message: err.Error(), Data: "error while unmarshalling the dealerMessage.Data into MsgUpdateCommitment"}
	}

	if !updateCommitmentMessage.Validate() {
		return nil, &jsonrpc.Error{Code: -32602, Message: "validation error", Data: "invalid dealer.MsgUpdateCommitment"}
	}

	err = serviceLibrary.DatabaseMethods().StorePSSCommitmentMatrix(updateCommitmentMessage.KeyIndex, updateCommitmentMessage.Commitment)
	if err != nil {
		return nil, &jsonrpc.Error{Code: -32602, Message: err.Error(), Data: "error while storing the commitment matrix"}
	}

	telemetry.IncGauge("dealer_update_commitment")

	return DealerResult{
		Code:    200,
		Message: "commitment updated!",
	}, nil
}
