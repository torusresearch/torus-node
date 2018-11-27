package dkgnode

import (
	"bytes"
	"fmt"

	"github.com/pkg/errors"
	"github.com/tendermint/tendermint/libs/common"
)

//Validates transactions to be delivered to the BFT. is the master switch for all tx
//TODO: create variables for types here and in bftrpc.go
func (app *ABCIApp) ValidateBFTTx(tx []byte) (bool, *[]common.KVPair, error) {
	if bytes.Compare(tx[:len([]byte("mug00"))], []byte("mug00")) != 0 {
		return false, nil, errors.New("Tx signature is not mug")
	}
	var tags []common.KVPair
	txNoSig := tx[len([]byte("mug00")):]

	// we use the first byte to denote message type
	switch txNoSig[0] {
	case byte(1): //PubpolyTx
		pubPolyTx := DefaultBFTTxWrapper{&PubPolyBFTTx{}}
		err := pubPolyTx.DecodeBFTTx(txNoSig)
		if err != nil {
			return false, nil, err
		}

		return true, nil, nil
		//verify share index has not yet been submitted for epoch

	case byte(2): // EpochTx
		EpochTx := DefaultBFTTxWrapper{&EpochBFTTx{}}
		err := EpochTx.DecodeBFTTx(txNoSig)
		if err != nil {
			return false, nil, err
		}
		//verify correct epoch
		epochTx := EpochTx.BFTTx.(*EpochBFTTx)
		if epochTx.EpochNumber != app.state.Epoch+1 {
			return false, nil, errors.New("Invalid epoch number! was: " + fmt.Sprintf("%d", app.state.Epoch) + "now: " + fmt.Sprintf("%d", epochTx.EpochNumber))
		} else {
			app.transientState.Epoch = epochTx.EpochNumber
		}
		fmt.Println("ATTACHING TAGS")
		tags = []common.KVPair{
			// retrieve tag using "localhost:26657/tx_search?query=\"epoch='1'\""
			// remember to change tendermint config to use index_all_tags = true
			// tags should come back in base64 encoding so pass a string as the Value
			{Key: []byte("epoch"), Value: []byte("1")},
		}
		return true, &tags, nil

	}
	return false, &tags, errors.New("Tx type not recognised")
}
