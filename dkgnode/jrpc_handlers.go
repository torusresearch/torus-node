package dkgnode

import (
	"context"
	"encoding/hex"
	"math/big"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/torusresearch/torus-public/secp256k1"

	ethCmn "github.com/ethereum/go-ethereum/common"
	cache "github.com/patrickmn/go-cache"
	tmquery "github.com/tendermint/tendermint/libs/pubsub/query"
	"github.com/tidwall/gjson"
	"github.com/torusresearch/bijson"
	"github.com/torusresearch/jsonrpc"
	"github.com/torusresearch/torus-public/common"
	"github.com/torusresearch/torus-public/logging"
)

func (h CommitmentRequestHandler) ServeJSONRPC(c context.Context, params *bijson.RawMessage) (interface{}, *jsonrpc.Error) {
	var p CommitmentRequestParams
	if err := jsonrpc.Unmarshal(params, &p); err != nil {
		return nil, err
	}
	timestamp := p.Timestamp
	tokenCommitment := p.TokenCommitment
	verifierIdentifier := p.VerifierIdentifier

	// check if message prefix is correct
	if p.MessagePrefix != "mug00" {
		return nil, &jsonrpc.Error{Code: 32603, Message: "Internal error", Data: "Incorrect message prefix"}
	}

	// check if timestamp has expired
	sec, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return nil, &jsonrpc.Error{Code: 32603, Message: "Internal error", Data: "Could not parse timestamp"}
	}
	if h.TimeNow().After(time.Unix(sec+60, 0)) {
		return nil, &jsonrpc.Error{Code: 32603, Message: "Internal error", Data: "Expired token (> 60 seconds)"}
	}

	// check if tokenCommitment has been seen before for that particular verifierIdentifier
	if h.suite.CacheSuite.TokenCaches[verifierIdentifier] == nil {
		h.suite.CacheSuite.TokenCaches[verifierIdentifier] = cache.New(cache.NoExpiration, 10*time.Minute)
	}
	tokenCache := h.suite.CacheSuite.TokenCaches[verifierIdentifier]
	_, found := tokenCache.Get(tokenCommitment)
	if found {
		return nil, &jsonrpc.Error{Code: 32603, Message: "Internal error", Data: "Duplicate token found"}
	}
	tokenCache.Set(string(tokenCommitment), true, 1*time.Minute)

	// sign data
	commitmentRequestResultData := CommitmentRequestResultData{
		p.MessagePrefix,
		p.TokenCommitment,
		p.TempPubX,
		p.TempPubY,
		p.Timestamp,
		p.VerifierIdentifier,
		strconv.FormatInt(time.Now().Unix(), 10),
	}

	sig := ECDSASign([]byte(commitmentRequestResultData.ToString()), h.suite.EthSuite.NodePrivateKey)

	res := CommitmentRequestResult{
		Signature: ECDSASigToHex(sig),
		Data:      commitmentRequestResultData.ToString(),
		NodePubX:  h.suite.EthSuite.NodePublicKey.X.Text(16),
		NodePubY:  h.suite.EthSuite.NodePublicKey.Y.Text(16),
	}
	return res, nil
}

func (h PingHandler) ServeJSONRPC(c context.Context, params *bijson.RawMessage) (interface{}, *jsonrpc.Error) {

	var p PingParams
	if err := jsonrpc.Unmarshal(params, &p); err != nil {
		return nil, err
	}

	return PingResult{
		Message: h.ethSuite.NodeAddress.Hex(),
	}, nil
}

// checks id for assignment and then the auth token for verification
// returns the node's share of the user's key
func (h ShareRequestHandler) ServeJSONRPC(c context.Context, params *bijson.RawMessage) (interface{}, *jsonrpc.Error) {
	var p ShareRequestParams
	if err := jsonrpc.Unmarshal(params, &p); err != nil {
		return nil, err
	}
	allKeyIndexes := make(map[string]*big.Int)   // String keyindex => keyindex
	allValidVerifierIDs := make(map[string]bool) // verifier + | + verifierIDs => bool

	// For Each VerifierItem we check its validity
	for _, rawItem := range p.Item {
		var parsedVerifierParams ShareRequestItem
		err := bijson.Unmarshal(rawItem, &parsedVerifierParams)
		if err != nil {
			return nil, &jsonrpc.Error{Code: 32602, Message: "Internal error", Data: "Error occurred while parsing sharerequestitem"}
		}

		// verify token validity against verifier, do not pass along nodesignatures
		jsonMap := make(map[string]interface{})
		bijson.Unmarshal(rawItem, &jsonMap)
		delete(jsonMap, "nodesignatures")
		redactedRawItem, err := bijson.Marshal(jsonMap)
		if err != nil {
			return nil, &jsonrpc.Error{Code: 32602, Message: "Internal error", Data: "Error occured while marshalling" + err.Error()}
		}
		verified, verifierID, err := h.suite.DefaultVerifier.Verify((*bijson.RawMessage)(&redactedRawItem))
		if err != nil {
			return nil, &jsonrpc.Error{Code: 32602, Message: "Internal error", Data: "Error occured while verifying params" + err.Error()}
		}
		if !verified {
			return nil, &jsonrpc.Error{Code: 32602, Message: "Internal error", Data: "Could not verify params"}
		}

		// Validate signatures
		var validSignatures []ValidatedNodeSignature
		for i := 0; i < len(parsedVerifierParams.NodeSignatures); i++ {
			nodeRef, err := parsedVerifierParams.NodeSignatures[i].NodeValidation(h.suite)
			if err == nil {
				validSignatures = append(validSignatures, ValidatedNodeSignature{
					parsedVerifierParams.NodeSignatures[i],
					*nodeRef.Index,
				})
			} else {
				logging.Error("Could not validate signatures" + err.Error())
			}
		}
		// Check if we have threshold number of signatures
		if len(validSignatures) < h.suite.Config.Threshold {
			return nil, &jsonrpc.Error{Code: 32602, Message: "Internal error", Data: "Not enough valid signatures. Only " + strconv.Itoa(len(validSignatures)) + "valid signatures found."}
		}
		// Find common data string, and filter valid signatures on the wrong data
		// this is to prevent nodes from submitting valid signatures on wrong data
		commonDataMap := make(map[string]int)
		for i := 0; i < len(validSignatures); i++ {
			var commitmentRequestResultData CommitmentRequestResultData
			commitmentRequestResultData.FromString(validSignatures[i].Data)
			stringData := commitmentRequestResultData.MessagePrefix + "|" +
				commitmentRequestResultData.TokenCommitment + "|" +
				commitmentRequestResultData.Timestamp + "|" +
				commitmentRequestResultData.VerifierIdentifier
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
			commitmentRequestResultData.FromString(validSignatures[i].Data)
			stringData := commitmentRequestResultData.MessagePrefix + "|" +
				commitmentRequestResultData.TokenCommitment + "|" +
				commitmentRequestResultData.Timestamp + "|" +
				commitmentRequestResultData.VerifierIdentifier
			if stringData == commonDataString {
				validCommonSignatures = append(validCommonSignatures, validSignatures[i])
			}
		}
		if len(validCommonSignatures) < h.suite.Config.Threshold {
			return nil, &jsonrpc.Error{Code: 32602, Message: "Internal error", Data: "Not enough valid signatures on the same data, " + strconv.Itoa(len(validCommonSignatures)) + " valid signatures."}
		}
		var commonData = strings.Split(commonDataString, "|")
		var commonTokenCommitment = commonData[1]
		var commonTimestamp = commonData[2]
		var commonVerifierIdentifier = commonData[3]

		// Lookup verifier
		verifier, err := h.suite.DefaultVerifier.Lookup(commonVerifierIdentifier)
		if err != nil {
			return nil, &jsonrpc.Error{Code: 32603, Message: "Internal error", Data: err.Error()}
		}

		// verify that hash of token = tokenCommitment
		cleanedToken := verifier.CleanToken(parsedVerifierParams.Token)
		if hex.EncodeToString(secp256k1.Keccak256([]byte(cleanedToken))) != commonTokenCommitment {
			return nil, &jsonrpc.Error{Code: 32602, Message: "Internal error", Data: "Token commitment and token are not compatible"}
		}

		// verify token timestamp
		sec, err := strconv.ParseInt(commonTimestamp, 10, 64)
		if err != nil {
			return nil, &jsonrpc.Error{Code: 32603, Message: "Internal error", Data: "Could not parse timestamp"}
		}
		if h.TimeNow().After(time.Unix(sec+60, 0)) {
			return nil, &jsonrpc.Error{Code: 32603, Message: "Internal error", Data: "Expired token (> 60 seconds)"}
		}

		// Get back list of indexes
		res, err := h.suite.BftSuite.BftRPC.ABCIQuery("GetIndexesFromVerifierID", []byte(verifierID)) // TODO: len
		if err != nil {
			return nil, &jsonrpc.Error{Code: 32603, Message: "Internal error", Data: "Could not get email index here: " + err.Error()}
		}
		var keyIndexes []big.Int
		err = bijson.Unmarshal(res.Response.Value, &keyIndexes)
		if err != nil {
			return nil, &jsonrpc.Error{Code: 32603, Message: "Internal error", Data: "could not parse keyindex list"}
		}

		// Add to overall list and valid verifierIDs
		for _, index := range keyIndexes {
			allKeyIndexes[index.Text(16)] = &index
		}

		allValidVerifierIDs[parsedVerifierParams.VerifierIdentifier+"|"+verifierID] = true
	}

	response := ShareRequestResult{}
	for _, index := range allKeyIndexes {

		// TODO: move code out into another function
		// check if we have enough validTokens according to Access Structure
		pubKeyAccessStructure := h.suite.ABCIApp.state.KeyMapping[index.Text(16)]
		validCount := 0
		for verifieridentifier, verifierIDs := range pubKeyAccessStructure.Verifiers {
			for _, verifierID := range verifierIDs {
				// check aganist all validVerifierIDs
				for verifierStrings := range allValidVerifierIDs {
					if verifieridentifier+"|"+verifierID == verifierStrings {
						validCount++
					}
				}
			}
		}
		si, _, _, err := h.suite.DBSuite.Instance.RetrieveCompletedShare(*index)
		if err != nil {
			return nil, &jsonrpc.Error{Code: 32603, Message: "Internal error", Data: "could not retrieve completed share"}
		}

		if validCount >= pubKeyAccessStructure.Threshold { // if we have enough authenticators we return Si
			keyAssignment := KeyAssignment{
				KeyAssignmentPublic: pubKeyAccessStructure,
			}
			keyAssignment.Share = *si
			response.Keys = append(response.Keys, keyAssignment)
		}
	}

	return response, nil
}

// assigns a user a secret, returns the same index if the user has been previously assigned
func (h KeyAssignHandler) ServeJSONRPC(c context.Context, params *bijson.RawMessage) (interface{}, *jsonrpc.Error) {
	randomInt := rand.Int()
	var p KeyAssignParams
	if err := jsonrpc.Unmarshal(params, &p); err != nil {
		return nil, err
	}
	logging.Debug("CHECKING IF EMAIL IS PROVIDED")
	// no email provided TODO: email validation/ ddos protection
	if p.VerifierID == "" {
		return nil, &jsonrpc.Error{Code: 32602, Message: "Input error", Data: "VerifierID is empty"}
	}
	logging.Debug("CHECKING IF CAN GET EMAIL ADDRESS")

	// check if verifier is valid
	_, err := h.suite.DefaultVerifier.Lookup(p.Verifier)
	if err != nil {
		return nil, &jsonrpc.Error{Code: 32602, Message: "Input error", Data: "Verifier not supported"}
	}

	// try to get get email index
	logging.Debug("CHECKING IF ALREADY ASSIGNED")

	//if all indexes have been assigned, bounce request. threshold at 20% TODO: Make  percentage variable
	// TODO: Change this to be not parameter dependent
	if h.suite.ABCIApp.state.LastCreatedIndex < h.suite.ABCIApp.state.LastUnassignedIndex+20 {
		return nil, &jsonrpc.Error{Code: 32604, Message: "System is under heavy load for assignments, please try again later"}
	}

	logging.Debug("CHECKING IF REACHED NEW ASSIGNMENT")
	// new assignment
	// broadcast assignment transaction
	hash, err := h.suite.BftSuite.BftRPC.Broadcast(DefaultBFTTxWrapper{&AssignmentBFTTx{VerifierID: p.VerifierID, Verifier: p.Verifier}})
	if err != nil {
		return nil, &jsonrpc.Error{Code: 32603, Message: "Internal error", Data: "Unable to broadcast: " + err.Error()}
	}

	logging.Debugf("CHECKING IF SUBSCRIBE TO UPDATES %d", randomInt)
	// subscribe to updates
	query := tmquery.MustParse("tx.hash='" + hash.String() + "'")
	logging.Debugf("BFTWS:, hashstring %s %d", hash.String(), randomInt)
	logging.Debugf("BFTWS: querystring %s %d", query.String(), randomInt)

	logging.Debugf("CHECKING IF GOT RESPONSES %d", randomInt)
	// wait for block to be committed
	var keyIndexes []big.Int
	responseCh, err := h.suite.BftSuite.RegisterQuery(query.String(), 1)
	if err != nil {
		logging.Debugf("BFTWS: could not register query, %s", query.String())
	}

	for e := range responseCh {
		logging.Debugf("BFTWS: gjson: %s %d", gjson.GetBytes(e, "query").String(), randomInt)
		logging.Debugf("BFTWS: queryString %s %d", query.String(), randomInt)
		if gjson.GetBytes(e, "query").String() != query.String() {
			continue
		}
		res, err := h.suite.BftSuite.BftRPC.ABCIQuery("GetIndexesFromVerifierID", []byte(p.VerifierID))
		if err != nil {
			return nil, &jsonrpc.Error{Code: 32603, Message: "Internal error", Data: "Failed to check if email exists after assignment: " + err.Error()}
		}
		if string(res.Response.Value) == "" {
			return nil, &jsonrpc.Error{Code: 32603, Message: "Internal error", Data: "Failed to find email after it has been assigned"}
		}

		err = bijson.Unmarshal(res.Response.Value, keyIndexes)
		if err != nil {
			return nil, &jsonrpc.Error{Code: 32603, Message: "Internal error", Data: "could not parse keyindex list"}
		}
		logging.Debugf("EXITING RESPONSES LISTENER %d", randomInt)
		break
	}

	result := KeyAssignResult{}
	publicKeys := make([]common.Point, 0)
	addresses := make([]ethCmn.Address, 0)
	for _, index := range keyIndexes {
		_, _, pk, err := h.suite.DBSuite.Instance.RetrieveCompletedShare(index)
		if err != nil {
			return nil, &jsonrpc.Error{Code: 32603, Message: "Could not find address to key index"}
		}
		publicKeys = append(publicKeys, *pk)
		//form address eth
		addr, err := common.PointToEthAddress(*pk)
		if err != nil {
			logging.Debug("derived user pub key has issues with address")
			return nil, &jsonrpc.Error{Code: 32603, Message: "Internal error"}
		}
		addresses = append(addresses, *addr)
		result.Keys = append(result.Keys, KeyAssignItem{
			KeyIndex:  index.Text(16),
			PubShareX: pk.X.Text(16),
			PubShareY: pk.Y.Text(16),
			Address:   addr.String(),
		})
	}

	return result, nil
}
