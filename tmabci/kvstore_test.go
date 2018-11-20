package tmabci

import (
	"testing"
)

func TestDeliver(t *testing.T) {
	var byteStr = []byte(`{"Type":"publicpoly","Payload":{"Type":"inner","Payload":null}}`)
	app := NewKVStoreApplication()
	resp := app.DeliverTx(byteStr)
	t.Log(resp)
}
