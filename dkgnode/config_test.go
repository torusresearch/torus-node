package dkgnode

import (
	"testing"
)

func TestDefaultConfig(t *testing.T) {

	conf := loadConfig("../config/config.local.5.json")
	// TODO: More detailed tests go here :)
	if conf.HostName != "node_five" || conf.BftURI != "tcp://0.0.0.0:26665" {
		t.Fatal("invalid config loaded")
	}
}
