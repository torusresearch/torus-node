package nodetest

import (
	"testing"

	"gopkg.in/h2non/baloo.v3"
)

// test stores the HTTP testing client preconfigured
var test = baloo.New("https://localhost:8000")

var payload = `{"jsonrpc": "2.0","method": "SecretAssign","id": 6,"params": {"email": "test@gmail.com"}}`

func TestShareAssign(t *testing.T) {
	test.Post("/jrpc").
		JSON([]byte(payload)).
		Expect(t).
		Status(200).
		Type("json").
		Done()
}
