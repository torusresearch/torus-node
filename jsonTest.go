package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
)

type Jsonnn struct {
	Type    string
	Payload interface{}
}

func main() {
	ob := Jsonnn{Type: "test", Payload: Jsonnn{Type: "inner"}}

	obJson, _ := json.Marshal(ob)
	_ = ioutil.WriteFile("dummy.json", obJson, 0644)
	fmt.Printf("%+v", ob)
}
