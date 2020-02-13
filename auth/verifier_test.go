package auth

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/torusresearch/bijson"
)

func TestDemoVerifier(t *testing.T) {
	expectedKey := "testKeyWhichIsNotBluBlu"
	d := &DemoVerifier{ExpectedKey: expectedKey, Store: make(map[string]bool)}
	payload := fmt.Sprintf("{\"email\":\"\",\"idToken\":%q}", expectedKey)
	rawMsg := bijson.RawMessage([]byte(payload))

	verified, _, err := d.VerifyRequestIdentity(&rawMsg)
	if !verified || err != nil {
		t.Fatal("expected the demoverifier to succeed with the provided key")
	}
}

func TestGenVerifier(t *testing.T) {
	var (
		d1 Verifier
		d2 Verifier
	)
	d1 = &DemoVerifier{"d1expectedkey", make(map[string]bool)}
	d2 = &DemoVerifier{"d2expectedkey", make(map[string]bool)}
	gv := DefaultGeneralVerifier{
		Verifiers: make(map[string]Verifier),
	}
	gv.Verifiers["d1"] = d1
	gv.Verifiers["d2"] = d2
	req := []byte(`{"verifieridentifier": "d1", "idtoken": "d1expectedkey"}`)
	jsonreq := bijson.RawMessage(req)
	res, _, _ := gv.Verify(&jsonreq)
	assert.True(t, res)
	req2 := []byte(`{"verifieridentifier": "d1", "idtoken": "d2expectedkey"}`)
	jsonreq2 := bijson.RawMessage(req2)
	res2, _, _ := gv.Verify(&jsonreq2)
	assert.False(t, res2)
}
