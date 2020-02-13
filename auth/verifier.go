package auth

import (
	"errors"
	"strings"

	"github.com/torusresearch/bijson"
	pcmn "github.com/torusresearch/torus-node/common"
)

type VerifierDetailsID string

type VerifierDetails struct {
	Verifier   string
	VerifierID string
}

func (v *VerifierDetails) ToVerifierDetailsID() VerifierDetailsID {
	return VerifierDetailsID(strings.Join([]string{v.Verifier, v.VerifierID}, pcmn.Delimiter1))
}

func (v *VerifierDetails) FromVerifierDetailsID(vID VerifierDetailsID) error {
	s := string(vID)
	substrings := strings.Split(s, pcmn.Delimiter1)

	if len(substrings) != 2 {
		return errors.New("Error parsing PSSIDDetails, too few fields")
	}

	v.Verifier = substrings[0]
	v.VerifierID = substrings[1]
	return nil
}

// Verifier describes verification of a token, without checking for token uniqueness
type Verifier interface {
	GetIdentifier() string
	CleanToken(string) string
	VerifyRequestIdentity(*bijson.RawMessage) (verified bool, verifierID string, err error)
}

// IdentityVerifier describes a common implementation shared among torus
// identity verifiers

type VerifyMessage struct {
	Token              string `json:"idtoken"`
	VerifierIdentifier string `json:"verifieridentifier"`
}

// GeneralVerifier accepts an identifier string and returns an IdentityVerifier
type GeneralVerifier interface {
	ListVerifiers() []string
	Verify(*bijson.RawMessage) (verified bool, verifierID string, err error)
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
// Returns result, verifierID and error
func (tgv *DefaultGeneralVerifier) Verify(rawMessage *bijson.RawMessage) (bool, string, error) {
	var verifyMessage VerifyMessage
	if err := bijson.Unmarshal(*rawMessage, &verifyMessage); err != nil {
		return false, "", err
	}
	v, err := tgv.Lookup(verifyMessage.VerifierIdentifier)
	if err != nil {
		return false, "", err
	}
	cleanedToken := v.CleanToken(verifyMessage.Token)
	if cleanedToken != verifyMessage.Token {
		return false, "", errors.New("Cleaned token is different from original token")
	}
	return v.VerifyRequestIdentity(rawMessage)
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
func NewGeneralVerifier(verifiers []Verifier) GeneralVerifier {
	dgv := &DefaultGeneralVerifier{
		Verifiers: make(map[string]Verifier),
	}
	for _, verifier := range verifiers {
		dgv.Verifiers[verifier.GetIdentifier()] = verifier
	}
	return dgv
}
