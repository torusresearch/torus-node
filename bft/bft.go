package bft

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
)

const BftPath = "../config/bft.json"

type BftJsonInterface struct {
	Epoch int `json:"epoch"`
}

func Epoch() int {
	jsonFile, err := os.Open(BftPath)
	if err != nil {
		fmt.Println(err)
	}
	defer jsonFile.Close()
	byteValue, _ := ioutil.ReadAll(jsonFile)
	var tmpbft BftJsonInterface
	json.Unmarshal(byteValue, &tmpbft)
	return tmpbft.Epoch
}

func SetEpoch(val int) {
	tmpbft := BftJsonInterface{Epoch: val}
	bftJson, _ := json.Marshal(tmpbft)
	err := ioutil.WriteFile(BftPath, bftJson, 0644)
	if err != nil {
		log.Fatal(err)
	}
}
