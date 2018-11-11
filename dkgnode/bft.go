package dkgnode

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
)

type BftJsonInterface struct {
	Epoch int `json:"epoch"`
}

type Bft struct {
	Epoch    int
	MockPath string
}

func (bft Bft) FetchData() {
	jsonFile, err := os.Open(bft.MockPath)
	if err != nil {
		fmt.Println(err)
	}
	defer jsonFile.Close()
	byteValue, _ := ioutil.ReadAll(jsonFile)
	mockPath := bft.MockPath
	json.Unmarshal(byteValue, &bft)
	fmt.Println(bft.MockPath)
	bft.MockPath = mockPath
}
