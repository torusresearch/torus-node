package dkgnode

import (
	"bytes"

	"github.com/pkg/errors"
)

//Validates transactions to be delivered to the BFT. is the master switch for all tx
//TODO: create variables for types here and in bftrpc.go
func (app *ABCIApp) ValidateBFTTx(tx []byte) (bool, error) {
	if bytes.Compare(tx[:len([]byte("mug00"))], []byte("mug00")) != 0 {
		return false, errors.New("Tx signature is not mug")
	}
	txNoSig := tx[len([]byte("mug00")):]
	switch txNoSig[0] {
	case byte(uint(1)): //PubpolyTx
		pubPolyTx := DefaultBFTTxWrapper{PubPolyBFTTx{}}
		pubPolyTx.DecodeBFTTx(txNoSig[1:])
		//verify correct epoch

		//verify share index has not yet been submitted for epoch

	}
	return false, errors.New("Tx type not recognised")
}
