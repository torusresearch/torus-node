package auth

import (
	"github.com/intel-go/fastjson"
)

// Verifier describes verification of a token, without checking for token uniqueness
type Verifier interface {
	VerifyRequestIdentity(*fastjson.RawMessage) (bool, error)
}

// IdentityVerifier describes a common implementation shared among torus
// identity verifiers
type IdentityVerifier interface {
	Verifier
	UniqueTokenCheck(*fastjson.RawMessage) (bool, error)
}
