package auth

import (
	"errors"
	"strings"

	"github.com/intel-go/fastjson"
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
func (v *DemoVerifier) CleanToken(jsonToken *fastjson.RawMessage) *fastjson.RawMessage {
	var p DemoVerifierParams
	if err := fastjson.Unmarshal(*jsonToken, &p); err != nil {
		return nil
	}
	p.IDToken = strings.Trim(p.IDToken, " ")
	res, err := fastjson.Marshal(p)
	if err != nil {
		return nil
	}
	r := fastjson.RawMessage(res)
	return &r
}

// VerifyRequestIdentity - verifies identity of user based on their token
func (v *DemoVerifier) VerifyRequestIdentity(jsonToken *fastjson.RawMessage) (bool, error) {
	var p DemoVerifierParams
	if err := fastjson.Unmarshal(*v.CleanToken(jsonToken), &p); err != nil {
		return false, err
	}

	if p.IDToken != v.ExpectedKey {
		return false, errors.New("invalid idtoken")
	}

	v.Store[p.IDToken] = true
	return true, nil
}

// UniqueTokenCheck - checks if token has been duplicated
func (v *DemoVerifier) UniqueTokenCheck(jsonToken *fastjson.RawMessage) (bool, error) {
	var p DemoVerifierParams
	if err := fastjson.Unmarshal(*v.CleanToken(jsonToken), &p); err != nil {
		return false, err
	}
	if v.Store[p.IDToken] {
		return false, errors.New("token has been used before")
	}

	return true, nil
}
