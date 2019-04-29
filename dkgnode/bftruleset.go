package dkgnode

import (
	"bytes"
	"math/big"
	"strconv"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/pkg/errors"
	tmbtcec "github.com/tendermint/btcd/btcec"
	"github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/common"
	"github.com/torusresearch/torus-public/logging"
)

//Validates transactions to be delivered to the BFT. is the master switch for all tx
//TODO: create variables for types here and in bftrpc.go
func (app *ABCIApp) ValidateAndUpdateAndTagBFTTx(tx []byte) (bool, *[]common.KVPair, error) {
	var tags []common.KVPair
	if bytes.Compare(tx[:len([]byte("mug00"))], []byte("mug00")) != 0 {
		return false, &tags, errors.New("Tx signature is not mug00")
	}
	txNoSig := tx[len([]byte("mug00")):]

	// we use the first byte to denote message type
	switch txNoSig[0] {
	case byte(1): //PubpolyTx
		PubPolyTx := DefaultBFTTxWrapper{&PubPolyBFTTx{}}
		err := PubPolyTx.DecodeBFTTx(txNoSig)
		if err != nil {
			return false, &tags, err
		}
		pubPolyTx := PubPolyTx.BFTTx.(*PubPolyBFTTx)
		logging.Debug("ATTACHING TAGS for pubpoly")
		tags = []common.KVPair{
			{Key: []byte("pubpoly"), Value: []byte("1")},
			{Key: []byte("share_index"), Value: []byte(strconv.Itoa(int(pubPolyTx.ShareIndex)))},
		}
		return true, &tags, nil
		//verify share index has not yet been submitted for epoch
	case byte(3): // KeyGenShareBFTTx
		KeyGenShareTx := DefaultBFTTxWrapper{&KeyGenShareBFTTx{}}
		err := KeyGenShareTx.DecodeBFTTx(txNoSig)
		if err != nil {
			return false, &tags, err
		}
		// TODO: verify keygen share?
		logging.Debug("ATTACHING TAGS for keygenshare")
		tags = []common.KVPair{
			{Key: []byte("keygeneration.sharecollection"), Value: []byte("1")},
		}
		return true, &tags, nil

	case byte(4): // AssignmentBFTTx
		logging.Debug("Assignmentbfttx happening")
		AssignmentTx := DefaultBFTTxWrapper{&AssignmentBFTTx{}}
		err := AssignmentTx.DecodeBFTTx(txNoSig)
		if err != nil {
			logging.Errorf("assignmentbfttx failed with error %s", err)
			return false, &tags, err
		}
		parsedTx := AssignmentTx.BFTTx.(AssignmentBFTTx)

		// User can be Assigned more then one key
		// if _, ok := app.state.[assignmentTx.Email]; ok { //check if user has been assigned before
		// 	logging.Errorf("assignmentbfttx failed with email already assigned")
		// 	return false, &tags, errors.New("Email " + assignmentTx.Email + " has already been assigned")
		// }

		// assign user email to key index
		if app.state.LastUnassignedIndex >= app.state.LastCreatedIndex {
			return false, &tags, errors.New("Last assigned index is exceeding last created index")
		}

		// 	Epoch               uint                            `json:"epoch"`
		// Height              int64                           `json:"height"`
		// AppHash             []byte                          `json:"app_hash"`
		// LastUnassignedIndex uint                            `json:"last_unassigned_index"`
		// LastCreatedIndex    uint                            `json:"last_created_index`
		// KeyMapping          map[string]KeyAssignmentPublic  `json:"key_mapping"`           // KeyIndex => KeyAssignmentPublic
		// VerifierToKeyIndex  map[string](map[string]TorusID) `json:"verifier_to_key_index"` // Verifier => VerifierID => KeyIndex
		// ValidatorSet        []types.ValidatorUpdate         `json:"-"`                     // `json:"validator_set"`
		// UpdateValidators    bool                            `json:"-"`                     // `json

		// Assign Stuff on State
		verifier, ok := app.state.VerifierToKeyIndex[parsedTx.Verifier]
		if !ok {
			return false, &tags, errors.New("Verifier doesnt exist for assignement")
		}
		torusID, ok := verifier[parsedTx.VerifierID]
		if !ok {
			verifier[parsedTx.VerifierID] = TorusID{
				Index:      int(app.state.LastUnassignedTorusIndex),
				KeyIndexes: []big.Int{*big.NewInt(int64(app.state.LastUnassignedIndex))},
			}
		} else {
			torusID.KeyIndexes = append(torusID.KeyIndexes, *big.NewInt(int64(app.state.LastUnassignedIndex)))
		}
		tags = []common.KVPair{
			{Key: []byte("assignment"), Value: []byte("1")},
		}

		// increment counters
		app.state.LastUnassignedTorusIndex = uint(app.state.LastUnassignedTorusIndex + 1)
		app.state.LastUnassignedIndex = uint(app.state.LastUnassignedIndex + 1)
		return true, &tags, nil

	case byte(6): // ValidatorUpdateBFTTx
		logging.Debug("Validator update tx sent")
		ValidatorUpdateTx := DefaultBFTTxWrapper{&ValidatorUpdateBFTTx{}}
		err := ValidatorUpdateTx.DecodeBFTTx(txNoSig)
		if err != nil {
			return false, nil, err
		}
		validatorUpdateTx := ValidatorUpdateTx.BFTTx.(*ValidatorUpdateBFTTx)
		//check if lengths are equal
		if len(validatorUpdateTx.ValidatorPower) != len(validatorUpdateTx.ValidatorPubKey) {
			return false, nil, errors.New("Lenghts not equal in validator update")
		}
		//convert to validator update struct
		validatorUpdateStruct := make([]types.ValidatorUpdate, len(validatorUpdateTx.ValidatorPower))
		for i := range validatorUpdateTx.ValidatorPower {
			tempKey := tmbtcec.PublicKey{
				X: &validatorUpdateTx.ValidatorPubKey[i].X,
				Y: &validatorUpdateTx.ValidatorPubKey[i].Y,
			}
			validatorUpdateStruct[i] = types.ValidatorUpdate{
				PubKey: types.PubKey{
					Type: "secp256k1",
					Data: tempKey.SerializeCompressed(),
				},
				Power: int64(validatorUpdateTx.ValidatorPower[i]),
			}
		}
		logging.Debugf("comparint validator structs %v", validatorUpdateStruct, convertNodeListToValidatorUpdate(app.Suite.EthSuite.NodeList))
		logging.Debugf("it was:  %v", cmp.Equal(validatorUpdateStruct,
			convertNodeListToValidatorUpdate(app.Suite.EthSuite.NodeList),
			cmpopts.IgnoreFields(types.ValidatorUpdate{}, "XXX_NoUnkeyedLiteral", "XXX_sizecache", "XXX_unrecognized")))
		//check agasint internal nodelist
		if cmp.Equal(validatorUpdateStruct,
			convertNodeListToValidatorUpdate(app.Suite.EthSuite.NodeList),
			cmpopts.IgnoreFields(types.ValidatorUpdate{}, "XXX_NoUnkeyedLiteral", "XXX_sizecache", "XXX_unrecognized")) {
			//update internal app state for next commit
			app.state.ValidatorSet = validatorUpdateStruct
			//set val update to true to trigger endblock
			app.state.UpdateValidators = true
			logging.Debug("update validator set to true")
		} else {
			//validators not accepted, might trigger cause nodelist not completely updated? perhpas call node list first
			return false, nil, errors.New("Validator update not accepted")
		}
		tags = []common.KVPair{
			{Key: []byte("updatevalidator"), Value: []byte("1")},
		}
		return true, &tags, nil
	case byte(7): // BFTKeygenMsg
		// TODO: Bring up router to this level (needed for PSS)
		wrapper := DefaultBFTTxWrapper{&BFTKeygenMsg{}}
		err := wrapper.DecodeBFTTx(txNoSig)
		if err != nil {
			return false, nil, err
		}
		bftMsg := wrapper.BFTTx.(*BFTKeygenMsg)
		if !app.Suite.P2PSuite.KeygenProto.onBFTMsg(*bftMsg) {
			return false, nil, errors.New("BFTMsg not accepted")
		}
		tags = []common.KVPair{
			{Key: []byte("bftmsg"), Value: []byte("1")},
		}
		return true, &tags, nil
	}

	return false, &tags, errors.New("Tx type not recognised")
}

// Checks if status update is from a valid node
func matchNode(suite *Suite, pkx, pky string) (int, error) {
	pubKeyX, parsed := new(big.Int).SetString(pkx, 16)
	if !parsed {
		return -1, errors.New("Could not parse pubkeyx when matching node")
	}
	pubKeyY, parsed := new(big.Int).SetString(pky, 16)
	if !parsed {
		return -1, errors.New("Could not parse pubkeyy when matching node")
	}
	// TODO: implement epoch-based lookup for nodelist
	for _, nodeRef := range suite.EthSuite.NodeList {
		if nodeRef.PublicKey.X.Cmp(pubKeyX) == 0 && nodeRef.PublicKey.Y.Cmp(pubKeyY) == 0 {
			nodeRefIndex, err := strconv.Atoi(nodeRef.Index.Text(10))
			if err != nil {
				return -1, errors.New("Error with casting index to int")
			}
			return nodeRefIndex, nil
		}
	}
	return -1, errors.New("Could not match node")
}
