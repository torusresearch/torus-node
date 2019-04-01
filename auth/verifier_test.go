package auth

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/intel-go/fastjson"
)

func TestDemoVerifier(t *testing.T) {
	expectedKey := "testKeyWhichIsNotBluBlu"
	var d IdentityVerifier
	d = &DemoVerifier{expectedKey: expectedKey, store: make(map[string]bool)}
	payload := fmt.Sprintf("{\"email\":\"\",\"idToken\":%q}", expectedKey)
	rawMsg := fastjson.RawMessage([]byte(payload))

	ok, err := d.VerifyRequestIdentity(&rawMsg)
	if !ok || err != nil {
		t.Fatal("expected the demoverifier to succeed with the provided key")
	}

	ok, err = d.UniqueTokenCheck(&rawMsg)
	if ok {
		t.Fatal("expected error when reusing same token")
	}

	if err.Error() != "token has been used before" {
		t.Fatal("unexpected error message: " + err.Error())
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
	jsonreq := fastjson.RawMessage(req)
	res, _ := gv.Verify(&jsonreq)
	assert.True(t, res)
	req2 := []byte(`{"verifieridentifier": "d1", "idtoken": "d2expectedkey"}`)
	jsonreq2 := fastjson.RawMessage(req2)
	res2, _ := gv.Verify(&jsonreq2)
	assert.False(t, res2)
}
