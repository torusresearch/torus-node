package auth

import (
	"encoding/json"
	"errors"

	"github.com/intel-go/fastjson"
)

// Verifier describes verification of a token, without checking for token uniqueness
type Verifier interface {
	GetIdentifier() string
	CleanToken(string) string
	VerifyRequestIdentity(*fastjson.RawMessage) (bool, error)
}

// IdentityVerifier describes a common implementation shared among torus
// identity verifiers

type VerifyMessage struct {
	Token              string `json:"token"`
	VerifierIdentifier string `json:"verifieridentifier"`
}

// GeneralVerifier accepts an identifier string and returns an IdentityVerifier
type GeneralVerifier interface {
	ListVerifiers() []string
	Verify(*fastjson.RawMessage) (bool, error)
	Lookup(string) (Verifier, error)
}

// DefaultGeneralVerifier is the defualt general verifier that is used
type DefaultGeneralVerifier struct {
	Verifiers map[string]Verifier
}

// ListVerifiers gets List of Registered Verifiers
func (tgv *DefaultGeneralVerifier) ListVerifiers() []string {
	list := make([]string, len(tgv.Verifiers))
	count := 0
	for k := range tgv.Verifiers {
		list[count] = k
		count++
	}
	return list
}

// Verify reroutes the json request to the appropriate sub-verifier within generalVerifier
func (tgv *DefaultGeneralVerifier) Verify(rawMessage *fastjson.RawMessage) (bool, error) {
	var verifyMessage VerifyMessage
	if err := fastjson.Unmarshal(*rawMessage, &verifyMessage); err != nil {
		return false, err
	}
	v, err := tgv.Lookup(verifyMessage.VerifierIdentifier)
	if err != nil {
		return false, err
	}
	cleanedToken := v.CleanToken(verifyMessage.Token)
	jsonMap := make(map[string]interface{})
	err = json.Unmarshal(*rawMessage, &jsonMap)
	if err != nil {
		return false, err
	}
	jsonMap["token"] = cleanedToken
	cleanedRawMessageBytes, err := fastjson.Marshal(jsonMap)
	if err != nil {
		return false, err
	}
	cleanedRawMessage := fastjson.RawMessage(cleanedRawMessageBytes)
	return v.VerifyRequestIdentity(&cleanedRawMessage)
}

// Lookup returns the appropriate verifier
func (tgv *DefaultGeneralVerifier) Lookup(verifierIdentifier string) (Verifier, error) {
	if tgv.Verifiers == nil {
		return nil, errors.New("Verifiers mapping not initialized")
	}
	if tgv.Verifiers[verifierIdentifier] == nil {
		return nil, errors.New("Verifier with verifierIdentifier " + verifierIdentifier + " could not be found")
	}
	return tgv.Verifiers[verifierIdentifier], nil
}

// NewGeneralVerifier - Initialization function for a generic GeneralVerifier
func NewGeneralVerifier(verifiers ...Verifier) GeneralVerifier {
	dgv := &DefaultGeneralVerifier{
		Verifiers: make(map[string]Verifier),
	}
	for _, verifier := range verifiers {
		dgv.Verifiers[verifier.GetIdentifier()] = verifier
	}
	return dgv
}
