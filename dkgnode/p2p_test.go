package dkgnode

import (
	"github.com/torusresearch/bijson"
	"testing"
)

var basicDummyMsg = P2PBasicMsg{
	Timestamp:  int64(100),
	Id:         "yoyo",
	Gossip:     false,
	NodeId:     "1",
	NodePubKey: []byte("something"),
	Sign:       []byte("somethin2"),
	MsgType:    "Ping",
}

func TestJSONSerializer(t *testing.T) {
	tempMsg := basicDummyMsg
	pb, _ := bijson.Marshal(Ping{Message: "Keyo"})
	tempMsg.Payload = pb
	b, err := bijson.Marshal(tempMsg)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(string(b))
	receivingStruct := P2PBasicMsg{}
	ping := Ping{}
	err = bijson.Unmarshal(b, &receivingStruct)
	if err != nil {
		t.Fatal(err)
	}
	bijson.Unmarshal(receivingStruct.Payload, &ping)
	t.Log(ping.Message)
}
