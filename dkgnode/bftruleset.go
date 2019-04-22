package dkgnode

import (
	"bytes"
	"fmt"
	"math/big"
	"strconv"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/looplab/fsm"
	"github.com/pkg/errors"
	tmbtcec "github.com/tendermint/btcd/btcec"
	"github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/common"
	"github.com/torusresearch/torus-public/keygen"
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

		// initialize fsm if it does not exist
		// TODO: create dictionary for states, lets initialize elsewhere
		if app.state.NodeStatus[uint(nodeIndex)] == nil {
			app.state.NodeStatus[uint(nodeIndex)] = fsm.NewFSM(
				"standby",
				fsm.Events{
					{Name: "initiate_keygen", Src: []string{"standby"}, Dst: "initiated_keygen"},
					{Name: "keygen_complete", Src: []string{"initiated_keygen"}, Dst: "keygen_completed"},
					{Name: "end_keygen", Src: []string{"keygen_completed"}, Dst: "standby"},
				},
				fsm.Callbacks{
					"enter_state": func(e *fsm.Event) { fmt.Printf("STATUSTX: status set for node from %s to %s", e.Src, e.Dst) },
					"after_initiate_keygen": func(e *fsm.Event) {
						// check if all nodes have broadcasted initiate_keygen == "Y"
						counter := 0
						for _, nodeI := range app.Suite.EthSuite.NodeList {
							fsm, ok := app.state.NodeStatus[uint(nodeI.Index.Int64())]
							if ok && fsm.Current() == "initiated_keygen" {
								//statusTx.Data
								stopIndex := string(e.Args[0].([]byte))
								// TODO: Remove or convert to logging
								// fmt.Println("STATUSTX: DATA")
								// fmt.Println(e.Args)
								// fmt.Println(stopIndex)
								// fmt.Println("STATUSTX: Initiate Key Gen Registered till ", stopIndex)
								if stopIndex != strconv.Itoa(app.Suite.Config.KeysPerEpoch+int(app.state.LastCreatedIndex)) {

									// TODO: Remove or convert to logging
									// fmt.Println("here2", strconv.Itoa(app.Suite.Config.KeysPerEpoch+int(app.state.LastCreatedIndex)))
									continue
								}
								percentLeft := 100 * (app.state.LastCreatedIndex - app.state.LastUnassignedIndex) / uint(app.Suite.Config.KeysPerEpoch)
								if percentLeft > uint(app.Suite.Config.KeyBufferTriggerPercentage) {

									// TODO: Remove or convert to logging
									// fmt.Println("KEYGEN: Haven't hit buffer amount, percentLeft, lastcreatedindex, lastunassignedindex", percentLeft, app.state.LastCreatedIndex, app.state.LastUnassignedIndex)
									continue
								}
								counter++
							}
						}
						// fmt.Println("STATUSTX: another counter is at", counter)
						// if so we change local status to be ready for keygen
						if counter == len(app.Suite.EthSuite.NodeList) {
							// fmt.Println("STATUSTX: counter is equal at here", counter, app.Suite.EthSuite.NodeList)
							app.state.LocalStatus.Event("all_initiate_keygen") // TODO: make epoch variable
							// fmt.Println("STATUSTX: app.state is", app.state.NodeStatus, app.state)
						} else {
							// fmt.Println("Number of keygen initiation messages does not match number of nodes")
						}
					},
					"after_keygen_complete": func(e *fsm.Event) {
						// fmt.Println("KEYGEN: after_keygen_complete called")
						// Update LocalStatus based on rules
						// check if all nodes have broadcasted keygen_complete == "Y"
						counter := 0
						for _, nodeI := range app.Suite.EthSuite.NodeList { // TODO: make epoch variable
							fsm, ok := app.state.NodeStatus[uint(nodeI.Index.Int64())]
							if ok && fsm.Current() == "keygen_completed" {
								counter++
							}
						}
						// fmt.Println("STATUSTX: counter is at ", counter)
						if counter == len(app.Suite.EthSuite.NodeList) {
							// fmt.Println("STATUSTX: entered counter", counter, app.Suite.EthSuite.NodeList)
							// set all_keygen_complete to Y

							// reset all other nodes' keygen completion status
							for _, nodeI := range app.Suite.EthSuite.NodeList { // TODO: make epoch variable
								fsm, ok := app.state.NodeStatus[uint(nodeI.Index.Int64())]
								if ok {
									go func() {
										err = fsm.Event("end_keygen")
										// fmt.Println("KEYGEN: node status changed state to ", fsm.Current())
										if err != nil {
											// fmt.Println("ERROR: Error changing state ", err)
										}
									}()
								} else {
									// fmt.Println("KEYGEN: Could not find NodeStatus FSM for index ", uint(nodeI.Index.Int64()))
								}
							}
							// fmt.Println("KEYGEN: changed state to standby", app.state.NodeStatus)

							err := app.state.LocalStatus.Event("all_keygen_complete") // TODO: make epoch variable
							if err != nil {
								// fmt.Println("KEYGEN: Error changing state ", err)
								return
							}
							// fmt.Println("KEYGEN: changed state to verifying", app.state.LocalStatus.Current())
						} else {
							// fmt.Println("Number of keygen initiation messages does not match number of nodes")
						}
					},
				},
			)
		}

		// set status
		app.state.NodeStatus[uint(nodeIndex)].Event(statusTx.StatusType, statusTx.Data)
		// fmt.Println("STATUSTX: status set for node", uint(nodeIndex), statusTx.StatusType, statusTx.StatusValue)

		tags = []common.KVPair{
			{Key: []byte("status"), Value: []byte("1")},
		}
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
	case byte(7): // P2PBasicMsg
		// TODO: Bring up router to this level (needed for PSS)
		wrapper := DefaultBFTTxWrapper{&P2PBasicMsg{}}
		err := wrapper.DecodeBFTTx(txNoSig)
		if err != nil {
			return false, nil, err
		}
		p2pMsg := wrapper.BFTTx.(*P2PBasicMsg)
		if !app.Suite.P2PSuite.KeygenProto.onBFTMsg(*p2pMsg) {
			return false, nil, errors.New("P2PBasicMsg not accepted")
		}
		tags = []common.KVPair{
			{Key: []byte("p2pmsg"), Value: []byte("1")},
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
