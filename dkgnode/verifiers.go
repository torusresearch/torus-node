package dkgnode

import (
	"errors"

	"github.com/intel-go/fastjson"
	"github.com/osamingo/jsonrpc"
)

// IdentityVerifier describes a common implementation shared among torus
// identity verifiers
type IdentityVerifier interface {
	VerifyRequestIdentity(rawPayload *fastjson.RawMessage) (bool, error)
}

type DemoVerifier struct {
	expectedKey string
}

func (v *DemoVerifier) VerifyRequestIdentity(rawPayload *fastjson.RawMessage) (bool, error) {
	var p ShareRequestParams
	if err := jsonrpc.Unmarshal(rawPayload, &p); err != nil {
		return false, err
	}

	if p.IDToken == v.expectedKey {
		return true, nil
	}

	return false, errors.New("invalid idtoken")
}
