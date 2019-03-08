package dkgnode

import (
	"testing"
)

func TestDefaultConfig(t *testing.T) {

	conf := loadConfig("../config/config.local.5.json")
	// TODO: More detailed tests go here :)
	if conf.HostName != "192.167.10.6" || conf.BftURI != "tcp://0.0.0.0:26665" {
		t.Log(conf.HostName, conf.BftURI)
		t.Fatal("invalid config loaded")
	}
}
