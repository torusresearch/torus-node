package auth

import (
	"errors"
	"strings"

	"github.com/torusresearch/bijson"
)

type DemoVerifier struct {
	ExpectedKey string
	Store       map[string]bool
}

type DemoVerifierParams struct {
	Index   int    `json:"index"`
	IDToken string `json:"idtoken"`
	Email   string `json:"email"`
}

// GetIdentifier - return identifier string for verifier
func (v *DemoVerifier) GetIdentifier() string {
	return "demo"
}

// CleanToken - ensure that incoming token conforms to strict format to prevent replay attacks
func (v *DemoVerifier) CleanToken(token string) string {
	return strings.Trim(token, " ")
}

// VerifyRequestIdentity - verifies identity of user based on their token
func (v *DemoVerifier) VerifyRequestIdentity(jsonToken *bijson.RawMessage) (bool, string, error) {
	var p DemoVerifierParams
	if err := bijson.Unmarshal(*jsonToken, &p); err != nil {
		return false, "", err
	}

	p.IDToken = v.CleanToken(p.IDToken)

	if p.IDToken != v.ExpectedKey {
		return false, "", errors.New("invalid idtoken")
	}

	v.Store[p.IDToken] = true
	return true, p.Email, nil
}
