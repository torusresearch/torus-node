package auth

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/torusresearch/bijson"
	"github.com/torusresearch/torus-common/common"
	"github.com/torusresearch/torus-common/crypto"
	"github.com/torusresearch/torus-node/config"
)

type TorusRequest struct {
	VerifierID string  `json:"verifier_id"`
	Timestamp  big.Int `json:"timestamp"`
}

type TorusVerifier struct {
	Timeout time.Duration
}

type TorusVerifierParams struct {
	VerifierID string  `json:"verifier_id"`
	Timestamp  big.Int `json:"timestamp"`
	IDToken    string  `json:"idtoken"`
}

func (t *TorusVerifier) GetIdentifier() string {
	return "torus"
}

func (t *TorusVerifier) CleanToken(token string) string {
	return strings.Trim(token, " ")
}

func (t *TorusVerifier) VerifyRequestIdentity(rawPayload *bijson.RawMessage) (bool, string, error) {
	var p TorusVerifierParams
	if err := bijson.Unmarshal(*rawPayload, &p); err != nil {
		return false, "", err
	}

	marshalled, err := bijson.Marshal(TorusRequest{
		VerifierID: p.VerifierID,
		Timestamp:  p.Timestamp,
	})
	if err != nil {
		return false, "", err
	}

	pubKey := common.BigIntToPoint(
		common.HexToBigInt(config.GlobalMutableConfig.GetS("TorusVerifierPubKeyX")),
		common.HexToBigInt(config.GlobalMutableConfig.GetS("TorusVerifierPubKeyY")),
	)

	decodedIDToken, err := hex.DecodeString(p.IDToken)
	if err != nil {
		return false, "", err
	}

	if !crypto.VerifyPtFromRaw(marshalled, pubKey, decodedIDToken) {
		return false, "", fmt.Errorf("signature is not valid")
	}

	timeSigned := time.Unix(0, p.Timestamp.Int64())
	if timeSigned.Add(t.Timeout).Before(time.Now()) {
		return false, "", fmt.Errorf("timesigned is more than 60 seconds ago %s", timeSigned.String())
	}

	return true, p.VerifierID, nil
}

func NewTorusVerifier() *TorusVerifier {
	return &TorusVerifier{
		Timeout: 60 * time.Second,
	}
}
