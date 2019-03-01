package dkgnode

import (
	"bytes"
	"fmt"
	"math/big"
	"strconv"
	"time"

	"github.com/torusresearch/torus-public/logging"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/pkg/errors"
	tmbtcec "github.com/tendermint/btcd/btcec"
	"github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/common"
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
		assignmentTx := AssignmentTx.BFTTx.(*AssignmentBFTTx)
		if assignmentTx.Epoch != app.state.Epoch { //check if epoch is correct
			return false, &tags, errors.New("Epoch mismatch for tx")
		}

		if _, ok := app.state.EmailMapping[assignmentTx.Email]; ok { //check if user has been assigned before
			logging.Errorf("assignmentbfttx failed with email already assigned")
			return false, &tags, errors.New("Email " + assignmentTx.Email + " has already been assigned")
		}
		// assign user email to key index
		if app.state.LastUnassignedIndex >= app.state.LastCreatedIndex {
			return false, &tags, errors.New("Last assigned index is exceeding last created index")
		}
		app.state.EmailMapping[assignmentTx.Email] = uint(app.state.LastUnassignedIndex)
		logging.Debugf("assignmentbfttx happened with app state emailmapping now equal to %s", app.state.EmailMapping)
		tags = []common.KVPair{
			{Key: []byte("assignment"), Value: []byte("1")},
		}
		// increment counter
		app.state.LastUnassignedIndex = uint(app.state.LastUnassignedIndex + 1)
		return true, &tags, nil

	case byte(5): // StatusBFTTx
		logging.Debug("Status broadcast")
		StatusTx := DefaultBFTTxWrapper{&StatusBFTTx{}}

		// validation

		err := StatusTx.DecodeBFTTx(txNoSig)
		if err != nil {
			logging.Errorf("Statustx decoding failed with error %s", err)
		}
		statusTx := StatusTx.BFTTx.(*StatusBFTTx)
		// TODO: check signature from node
		nodeIndex, err := matchNode(app.Suite, statusTx.FromPubKeyX, statusTx.FromPubKeyY)
		if err != nil {
			return false, &tags, err
		}

		if statusTx.Epoch != app.state.Epoch {
			return false, &tags, errors.New("Epoch mismatch for tx")
		}

		// initialize inner mapping if it does not exist
		if app.state.NodeStatus[uint(nodeIndex)] == nil {
			app.state.NodeStatus[uint(nodeIndex)] = make(map[string]string)
		}

		// set status

		app.state.NodeStatus[uint(nodeIndex)][statusTx.StatusType] = statusTx.StatusValue
		logging.Debugf("STATUSTX: status set for node %s, %s, %s", uint(nodeIndex), statusTx.StatusType, statusTx.StatusValue)

		// Update LocalStatus based on rules
		// check if all nodes have broadcasted keygen_complete == "Y"
		counter := 0
		for _, nodeI := range app.Suite.EthSuite.NodeList { // TODO: make epoch variable
			if app.state.NodeStatus[uint(nodeI.Index.Int64())]["keygen_complete"] == "Y" {
				counter++
			}
		}
		logging.Debugf("STATUSTX: counter is at %s", counter)
		if counter == len(app.Suite.EthSuite.NodeList) {
			logging.Debugf("STATUSTX: entered counter %s %s", counter, app.Suite.EthSuite.NodeList)
			// set all_keygen_complete to Y
			app.state.LocalStatus["all_keygen_complete"] = "Y" // TODO: make epoch variable
			// reset all other nodes' keygen completion status
			for _, nodeI := range app.Suite.EthSuite.NodeList { // TODO: make epoch variable
				app.state.NodeStatus[uint(nodeI.Index.Int64())]["keygen_complete"] = ""
			}
			logging.Debugf("STATUSTX: app state is: %s", app.state)
			// update total number of available keys
			app.state.LastCreatedIndex = app.state.LastCreatedIndex + uint(app.Suite.Config.KeysPerEpoch)
			logging.Debugf("STATUSTX: lastcreatedindex %s", app.state.LastCreatedIndex)
			// start listening again for the next time we initiate a keygen
			go func() {
				time.Sleep(5 * time.Second)
				app.state.LocalStatus["all_initiate_keygen"] = ""
			}()
			app.state.Epoch = app.state.Epoch + uint(1)
			logging.Debugf("STATUSTX: state is %s", app.state)
			logging.Debugf("STATUSTX: epoch is %s", app.state.Epoch)
		} else {
			logging.Debug("Number of keygen initiation messages does not match number of nodes")
		}

		// check if all nodes have broadcasted initiate_keygen == "Y"
		counter = 0
		for _, nodeI := range app.Suite.EthSuite.NodeList {
			if app.state.NodeStatus[uint(nodeI.Index.Int64())]["initiate_keygen"] == "Y" {
				stopIndex := string(statusTx.Data)
				logging.Debugf("STATUSTX: Initiate Key Gen Registered till %s", stopIndex)
				if stopIndex != strconv.Itoa(app.Suite.Config.KeysPerEpoch+int(app.state.LastCreatedIndex)) {
					logging.Debugf("here2 %s", strconv.Itoa(app.Suite.Config.KeysPerEpoch+int(app.state.LastCreatedIndex)))
					continue
				}
				percentLeft := 100 * (app.state.LastCreatedIndex - app.state.LastUnassignedIndex) / uint(app.Suite.Config.KeysPerEpoch)
				if percentLeft > uint(app.Suite.Config.KeyBufferTriggerPercentage) {
					logging.Debugf("KEYGEN: Haven't hit buffer amount, percentLeft, lastcreatedindex, lastunassignedindex", percentLeft, app.state.LastCreatedIndex, app.state.LastUnassignedIndex)
					continue
				}
				counter++
			}
		}
		fmt.Println("STATUSTX: another counter is at", counter)
		if counter == len(app.Suite.EthSuite.NodeList) {
			fmt.Println("STATUSTX: counter is equal at here", counter, app.Suite.EthSuite.NodeList)
			app.state.LocalStatus["all_initiate_keygen"] = "Y" // TODO: make epoch variable
			for _, nodeI := range app.Suite.EthSuite.NodeList {
				app.state.NodeStatus[uint(nodeI.Index.Int64())]["initiate_keygen"] = ""
			}
			fmt.Println("STATUSTX: app.state is", app.state.NodeStatus, app.state)
		} else {
			fmt.Println("Number of keygen initiation messages does not match number of nodes")
		}

		tags = []common.KVPair{
			{Key: []byte("status"), Value: []byte("1")},
		}
		return true, &tags, nil

	case byte(6): // ValidatorUpdateBFTTx
		fmt.Println("Validator update tx sent")
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
		fmt.Println("comparint validator structs", validatorUpdateStruct, convertNodeListToValidatorUpdate(app.Suite.EthSuite.NodeList))
		fmt.Println("it was:  ", cmp.Equal(validatorUpdateStruct,
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
			fmt.Println("update validator set to true")
		} else {
			//validators not accepted, might trigger cause nodelist not completely updated? perhpas call node list first
			return false, nil, errors.New("Validator update not accepted")
		}
		tags = []common.KVPair{
			{Key: []byte("updatevalidator"), Value: []byte("1")},
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
