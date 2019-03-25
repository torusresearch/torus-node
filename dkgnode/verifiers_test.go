package dkgnode

import (
	"fmt"
	"testing"

	"github.com/intel-go/fastjson"
)

func TestDemoVerifier(t *testing.T) {
	expectedKey := "testKeyWhichIsNotBluBlu"
	d := &DemoVerifier{expectedKey: expectedKey}
	payload := fmt.Sprintf("{\"email\":\"\",\"idToken\":%q}", expectedKey)
	rawMsg := fastjson.RawMessage([]byte(payload))

	ok, err := d.VerifyRequestIdentity(&rawMsg)
	if !ok || err != nil {
		t.Fatal("expected the demoverifier to succeed with the provided key")
	}
}
