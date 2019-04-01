package auth

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/intel-go/fastjson"
)

type GoogleIdentityVerifier struct {
	GoogleVerifier
	tokenStore map[string]bool
}

func (g *GoogleIdentityVerifier) UniqueTokenCheck(rM *fastjson.RawMessage) (bool, error) {
	var p GoogleVerifierParams
	if err := fastjson.Unmarshal(*rM, &p); err != nil {
		return false, err
	}
	if !g.tokenStore[p.IDToken] {
		g.tokenStore[p.IDToken] = true
		return true, nil
	}
	return false, errors.New("token has been used before")
}

func TestEmptyEmailVerifier(t *testing.T) {
	payload := "{\"email\":\"\",\"idToken\":\"\"}"
	rawMsg := fastjson.RawMessage([]byte(payload))
	var v IdentityVerifier
	v = &GoogleIdentityVerifier{
		GoogleVerifier{},
		make(map[string]bool),
	}
	assert.Equal(t, v.GetIdentifier(), "google")
	ok, err := v.VerifyRequestIdentity(&rawMsg)
	if ok || err.Error() != "invalid payload parameters" {
		t.Fatal("a request with empty email and idToken passed without error")
	}
	ok, err = v.UniqueTokenCheck(&rawMsg)
	ok, err = v.UniqueTokenCheck(&rawMsg)
	if ok {
		t.Fatal("expected error when reusing same token")
	}

	if err.Error() != "token has been used before" {
		t.Fatal("unexpected error message: " + err.Error())
	}
}
