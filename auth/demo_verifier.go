package auth

import (
	"errors"

	"github.com/intel-go/fastjson"
	"github.com/osamingo/jsonrpc"
)

type DemoVerifier struct {
	expectedKey string
	store       map[string]bool
}

type DemoVerifierParams struct {
	Index   int    `json:"index"`
	IDToken string `json:"idtoken"`
	Email   string `json:"email"`
}

func NewDefaultDemoVerifier(expectedKey string) *DemoVerifier {
	return &DemoVerifier{
		expectedKey: expectedKey,
	}
}

// VerifyRequestIdentity - verifies identity of user based on their token
func (v *DemoVerifier) VerifyRequestIdentity(jsonToken *fastjson.RawMessage) (bool, error) {
	var p DemoVerifierParams
	if err := jsonrpc.Unmarshal(jsonToken, &p); err != nil {
		return false, err
	}

	if p.IDToken != v.expectedKey {
		return false, errors.New("invalid idtoken")
	}

	v.store[p.IDToken] = true
	return true, nil
}

// UniqueTokenCheck - checks if token has been duplicated
func (v *DemoVerifier) UniqueTokenCheck(jsonToken *fastjson.RawMessage) (bool, error) {
	var p DemoVerifierParams
	if err := jsonrpc.Unmarshal(jsonToken, &p); err != nil {
		return false, err
	}
	if v.store[p.IDToken] {
		return false, errors.New("token has been used before")
	}

	return true, nil
}
