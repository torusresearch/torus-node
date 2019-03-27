package auth

import (
	"fmt"
	"testing"

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
