package dealer

import (
	"math/big"
	"strconv"
	"strings"
	"time"

	logging "github.com/sirupsen/logrus"
	"github.com/torusresearch/torus-common/common"
	"github.com/torusresearch/torus-common/crypto"
	pcmn "github.com/torusresearch/torus-node/common"
	"github.com/torusresearch/torus-node/config"
)

type Message struct {
	Method    string       `json:"method"`
	KeyIndex  big.Int      `json:"keyindex"`
	PubKey    common.Point `json:"pubkey"`
	Data      []byte       `json:"data"`
	Signature []byte       `json:"signature"`
}

type MsgUpdatePublicKey struct {
	KeyIndex     big.Int      `json:"keyindex"`
	TMessage     TMessage     `json:"tmessage"`
	OldPubKey    common.Point `json:"oldpubkey"`
	OldPubKeySig []byte       `json:"oldpubkeysig"`
	NewPubKey    common.Point `json:"newpubkey"`
	NewPubKeySig []byte       `json:"newpubkeysig"`
}

type TMessage struct {
	Timestamp string `json:"timestamp"`
	Message   string `json:"message"`
}

type MsgUpdateShare struct {
	KeyIndex big.Int  `json:"keyindex"`
	TMessage TMessage `json:"tmessage"`
	Si       big.Int  `json:"si"`
	Siprime  big.Int  `json:"siprime"`
}

type MsgUpdateCommitment struct {
	KeyIndex   big.Int          `json:"keyindex"`
	TMessage   TMessage         `json:"tmessage"`
	Commitment [][]common.Point `json:"commitment"`
}

func (dealerMessage Message) Validate(existingPubKey common.Point) bool {
	if !config.GlobalMutableConfig.GetB("ValidateDealer") {
		return true
	}
	if existingPubKey.X.Cmp(&dealerMessage.PubKey.X) != 0 || existingPubKey.Y.Cmp(&dealerMessage.PubKey.Y) != 0 {
		return false
	}
	return crypto.VerifyPtFromRaw(dealerMessage.Data, dealerMessage.PubKey, dealerMessage.Signature)
}

func (t TMessage) Byte() []byte {
	return []byte(strings.Join([]string{t.Timestamp, t.Message}, pcmn.Delimiter2))
}

func (d MsgUpdatePublicKey) Validate(existingPubKey common.Point) bool {
	if !config.GlobalMutableConfig.GetB("ValidateDealer") {
		return true
	}
	if existingPubKey.X.Cmp(&d.OldPubKey.X) != 0 || existingPubKey.Y.Cmp(&d.OldPubKey.Y) != 0 {
		return false
	}

	substrs := strings.Split(d.TMessage.Message, pcmn.Delimiter1)

	if len(substrs) != 6 {
		return false
	}
	msgPrefix := substrs[0]
	msg := substrs[1]
	oldPubKeyX := substrs[2]
	oldPubKeyY := substrs[3]
	newPubKeyX := substrs[4]
	newPubKeyY := substrs[5]
	timeSigned := d.TMessage.Timestamp
	if msgPrefix != "mug00" {
		logging.Error("message prefix is incorrect")
		return false
	}
	if msg != "updatePubKey" {
		logging.Error("message is not updatePubKey")
		return false
	}
	unixTime, err := strconv.ParseInt(timeSigned, 10, 64)
	if err != nil {
		logging.WithError(err).Error("could not parse time signed from dealer msg update public key")
		return false
	}
	if time.Unix(unixTime, 0).Add(1 * time.Minute).Before(time.Now()) {
		logging.WithError(err).Error("signature expired")
		return false
	}
	if oldPubKeyX != d.OldPubKey.X.Text(16) ||
		oldPubKeyY != d.OldPubKey.Y.Text(16) ||
		newPubKeyX != d.NewPubKey.X.Text(16) ||
		newPubKeyY != d.NewPubKey.Y.Text(16) {
		logging.WithField("d", d).Error("signed message pubkeys do not match")
		return false
	}

	if !crypto.VerifyPtFromRaw([]byte(d.TMessage.Byte()), d.OldPubKey, d.OldPubKeySig) {
		logging.WithField("d", d).Error("incorrect old pub key signature")
		return false
	}

	if !crypto.VerifyPtFromRaw([]byte(d.TMessage.Byte()), d.NewPubKey, d.NewPubKeySig) {
		logging.WithField("d", d).Error("incorrect new pub key signature")
		return false
	}

	return true
}

func (d MsgUpdateShare) Validate() bool {
	if !config.GlobalMutableConfig.GetB("ValidateDealer") {
		return true
	}

	substrs := strings.Split(d.TMessage.Message, pcmn.Delimiter1)

	if len(substrs) != 2 {
		return false
	}
	msgPrefix := substrs[0]
	msg := substrs[1]
	timeSigned := d.TMessage.Timestamp
	if msgPrefix != "mug00" {
		logging.Error("message prefix is incorrect")
		return false
	}
	if msg != "updateShare" {
		logging.Error("message is not updatePubKey")
		return false
	}
	unixTime, err := strconv.ParseInt(timeSigned, 10, 64)
	if err != nil {
		logging.WithError(err).Error("could not parse time signed from dealer msg update share")
		return false
	}
	if time.Unix(unixTime, 0).Add(1 * time.Minute).Before(time.Now()) {
		logging.WithError(err).Error("signature expired")
		return false
	}
	return true
}

func (d MsgUpdateCommitment) Validate() bool {
	if !config.GlobalMutableConfig.GetB("ValidateDealer") {
		return true
	}

	substrs := strings.Split(d.TMessage.Message, pcmn.Delimiter1)

	if len(substrs) != 2 {
		return false
	}
	msgPrefix := substrs[0]
	msg := substrs[1]
	timeSigned := d.TMessage.Timestamp
	if msgPrefix != "mug00" {
		logging.Error("message prefix is incorrect")
		return false
	}
	if msg != "updateCommitment" {
		logging.Error("message is not updatePubKey")
		return false
	}
	unixTime, err := strconv.ParseInt(timeSigned, 10, 64)
	if err != nil {
		logging.WithError(err).Error("could not parse time signed for dealer msg update public key")
		return false
	}
	if time.Unix(unixTime, 0).Add(1 * time.Minute).Before(time.Now()) {
		logging.WithError(err).Error("signature expired")
		return false
	}
	return true
}
