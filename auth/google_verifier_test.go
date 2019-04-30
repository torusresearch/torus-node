package auth

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/torusresearch/bijson"
)

func TestEmptyEmailVerifier(t *testing.T) {
	payload := "{\"email\":\"\",\"idToken\":\"\"}"
	rawMsg := bijson.RawMessage([]byte(payload))
	var v Verifier
	v = &GoogleVerifier{}
	assert.Equal(t, v.GetIdentifier(), "google")
	ok, err := v.VerifyRequestIdentity(&rawMsg)
	if ok || err.Error() != "invalid payload parameters" {
		t.Fatal("a request with empty email and idToken passed without error")
	}
}
