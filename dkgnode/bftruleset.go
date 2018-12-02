package dkgnode

import (
	"bytes"
	"fmt"
	"math/big"
	"strconv"

	"github.com/pkg/errors"
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
		pubPolyTx := DefaultBFTTxWrapper{&PubPolyBFTTx{}}
		err := pubPolyTx.DecodeBFTTx(txNoSig)
		if err != nil {
			return false, &tags, err
		}
		fmt.Println("ATTACHING TAGS for pubpoly")
		tags = []common.KVPair{
			{Key: []byte("pubpoly"), Value: []byte("1")},
		}
		return true, &tags, nil
		//verify share index has not yet been submitted for epoch

	case byte(2): // EpochTx
		EpochTx := DefaultBFTTxWrapper{&EpochBFTTx{}}
		err := EpochTx.DecodeBFTTx(txNoSig)
		if err != nil {
			return false, &tags, err
		}
		//verify correct epoch
		epochTx := EpochTx.BFTTx.(*EpochBFTTx)
		if epochTx.EpochNumber != app.state.Epoch+1 {
			return false, &tags, errors.New("Invalid epoch number! was: " + fmt.Sprintf("%d", app.state.Epoch) + "now: " + fmt.Sprintf("%d", epochTx.EpochNumber))
		} else {
			app.state.Epoch = epochTx.EpochNumber
		}
		fmt.Println("ATTACHING TAGS for epoch")
		tags = []common.KVPair{
			// retrieve tag using "localhost:26657/tx_search?query=\"epoch='1'\""
			// remember to change tendermint config to use index_all_tags = true
			// tags should come back in base64 encoding so pass a string as the Value
			{Key: []byte("epoch"), Value: []byte("1")},
		}
		return true, &tags, nil

	case byte(3): // KeyGenShareBFTTx
		KeyGenShareTx := DefaultBFTTxWrapper{&KeyGenShareBFTTx{}}
		err := KeyGenShareTx.DecodeBFTTx(txNoSig)
		if err != nil {
			return false, &tags, err
		}
		// TODO: verify keygen share?
		fmt.Println("ATTACHING TAGS for keygenshare")
		tags = []common.KVPair{
			{Key: []byte("keygeneration.sharecollection"), Value: []byte("1")},
		}
		return true, &tags, nil

	case byte(4): // AssignmentBFTTx
		fmt.Println("assignmentbfttx happening")
		AssignmentTx := DefaultBFTTxWrapper{&AssignmentBFTTx{}}
		err := AssignmentTx.DecodeBFTTx(txNoSig)
		if err != nil {
			fmt.Println("assignmentbfttx failed with error", err)
			return false, &tags, err
		}
		assignmentTx := AssignmentTx.BFTTx.(*AssignmentBFTTx)
		if _, ok := app.state.EmailMapping[assignmentTx.Email]; ok {
			fmt.Println("assignmentbfttx failed with email already assigned")
			return false, &tags, errors.New("Email " + assignmentTx.Email + " has already been assigned")
		}
		// assign user email to key index
		app.state.EmailMapping[assignmentTx.Email] = uint(app.state.LastIndex + 1)
		fmt.Println("assignmentbfttx happened with app state emailmapping now equal to", app.state.EmailMapping)
		// increment counter
		app.state.LastIndex = uint(app.state.LastIndex + 1)
		tags = []common.KVPair{
			{Key: []byte("assignment"), Value: []byte("1")},
		}
		return true, &tags, nil

	case byte(5): // StatusBFTTx
		fmt.Println("Status broadcast")
		StatusTx := DefaultBFTTxWrapper{&StatusBFTTx{}}
		err := StatusTx.DecodeBFTTx(txNoSig)
		if err != nil {
			fmt.Println("Statustx decoding failed with error", err)
		}
		statusTx := StatusTx.BFTTx.(*StatusBFTTx)
		// TODO: check signature from node
		nodeIndex, err := matchNode(app.Suite, statusTx.FromPubKeyX, statusTx.FromPubKeyY)
		if err != nil {
			return false, &tags, err
		}

		// valid status update from node

		// initialize inner mapping if it does not exist
		if app.state.NodeStatus[uint(nodeIndex)] == nil {
			app.state.NodeStatus[uint(nodeIndex)] = make(map[string]string)
		}

		app.state.NodeStatus[uint(nodeIndex)][statusTx.StatusType] = statusTx.StatusValue
		counter := 0
		fmt.Println("REACHED HERE1")

		// update LocalStatus
		for _, nodeI := range app.Suite.EthSuite.NodeList {
			fmt.Println()
			if app.state.NodeStatus[uint(nodeI.Index.Int64())]["keygen_complete"] == "Y" {
				counter++
			}
		}
		fmt.Println("REACHED HERE2", counter)
		if counter == len(app.Suite.EthSuite.NodeList) {
			fmt.Println("REACHED HERE3")
			// TODO: make epoch variable
			app.state.LocalStatus["keygen_all_complete_epoch_0"] = "Y"
		}
		tags = []common.KVPair{
			{Key: []byte("status"), Value: []byte("1")},
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
