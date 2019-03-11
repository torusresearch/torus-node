package dkgnode

import (
	"fmt"
	"os"
	"testing"
)

const defaultTestConfig = "../config/config.local.5.json"

func TestDefaultConfig(t *testing.T) {

	conf := loadConfig(defaultTestConfig)
	// TODO: More detailed tests go here :)
	if conf.KeysPerEpoch != 10 || conf.BftURI != "tcp://0.0.0.0:26665" {
		fmt.Printf("%+v", conf)
		t.Fatal("invalid config loaded")
	}

}

func TestEnvOverride(t *testing.T) {
	os.Setenv("CPU_PROFILE", "test")
	os.Setenv("KEYS_PER_EPOCH", "6")
	os.Setenv("NODE_LIST_ADDRESS", "0xTestNodeListAddress")
	conf := loadConfig(defaultTestConfig)

	if conf.CPUProfileToFile != "test" || conf.KeysPerEpoch != 6 || conf.NodeListAddress != "0xTestNodeListAddress" {
		t.Fatal("ENV override not working as intended", conf.CPUProfileToFile, conf.KeysPerEpoch, conf.NodeListAddress)
	}

}

func TestMergeOrder(t *testing.T) {
	return
}
