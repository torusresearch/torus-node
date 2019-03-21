package dkgnode

import (
	"testing"

	"github.com/intel-go/fastjson"
)

func TestEmptyEmailVerifier(t *testing.T) {
	payload := "{\"email\":\"\",\"idToken\":\"\"}"
	rawMsg := fastjson.RawMessage([]byte(payload))
	verifier := NewDefaultGoogleVerifier("")

	ok, err := verifier.VerifyRequestIdentity(&rawMsg)
	if ok || err.Error() != "invalid payload parameters" {
		t.Fatal("a request with empty email and idToken passed without error")
	}
}
