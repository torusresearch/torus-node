package dkgnode

import (
	crypto "github.com/libp2p/go-libp2p-crypto"
	peer "github.com/libp2p/go-libp2p-peer"
	"github.com/torusresearch/bijson"
	"github.com/torusresearch/torus-public/pvss"
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

const pingMsg = `{"timestamp":1556109515,"id":"62a186c9-e33e-4c0c-8f39-66bbeb03967d","nodeId":"16Uiu2HAmHjD8EayRbnbNkvFWnrWvw3qQGH8qM23UEXpkMUM5ATjK","nodePubKey":"CAISIQNLXzPX3YTqC3oeuc3v4z28xoIpM8+kGcARLpy+M+hLJg==","sign":"MEQCIF2XujTNsCdRkPSU6lNh3rxu+syLMg6sTDB5RQm+iWNGAiBFrydWS755pTlxVLilOoRkPaizB3S4kf6JvjVfrdXzKA==","payload":"eyJNZXNzYWdlIjoiUGluZyBmcm9tIFx1MDAzY3BlZXIuSUQgMTYqTTVBVGpLXHUwMDNlIn0="}`

func TestPeerSerializtionPing(t *testing.T) {

	p2pmsg := &P2PBasicMsg{}
	bijson.Unmarshal([]byte(pingMsg), p2pmsg)
	t.Log(p2pmsg.GetNodeId())
	_, err := peer.IDB58Decode(p2pmsg.GetNodeId())
	if err != nil {
		t.Fatal(err)
	}
}
func TestAuthentication(t *testing.T) {
	p2pSuite := P2PSuite{}
	p2pmsg := &P2PBasicMsg{}
	bijson.Unmarshal([]byte(pingMsg), p2pmsg)
	if !p2pSuite.authenticateMessage(p2pmsg) {
		t.Fatal("could not authenticate msg")
	}

}
func TestPeerSerializtion(t *testing.T) {
	ran := pvss.RandomBigInt()
	priv, err := crypto.UnmarshalSecp256k1PrivateKey(ran.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	id, err := peer.IDFromPrivateKey(priv)
	if err != nil {
		t.Fatal(err)
	}
	encoded := peer.IDHexEncode(id)
	decoded, err := peer.IDHexDecode(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if !(id.String() == decoded.String()) {
		t.Fatal("could serialize equally")
	}
}

func TestPeerSerializtionPublic(t *testing.T) {
	ran := pvss.RandomBigInt()
	priv, err := crypto.UnmarshalSecp256k1PrivateKey(ran.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	id, err := peer.IDFromPublicKey(priv.GetPublic())
	if err != nil {
		t.Fatal(err)
	}
	encoded := peer.IDB58Encode(id)
	decoded, err := peer.IDB58Decode(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if !(id.String() == decoded.String()) {
		t.Fatal("could serialize equally")
	}
}

type testStructID struct {
	Id string
}

func TestPeerSerializtionBijson(t *testing.T) {
	ran := pvss.RandomBigInt()
	priv, err := crypto.UnmarshalSecp256k1PrivateKey(ran.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	id, err := peer.IDFromPublicKey(priv.GetPublic())
	if err != nil {
		t.Fatal(err)
	}
	encoded := peer.IDB58Encode(id)
	t.Log(encoded)
	str := testStructID{encoded}
	strBytes, err := bijson.Marshal(str)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(string(strBytes))
	recStr := &testStructID{}
	err = bijson.Unmarshal(strBytes, recStr)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(recStr.Id)
	decoded, err := peer.IDB58Decode(recStr.Id)
	if err != nil {
		t.Fatal(err)
	}
	if id.String() != decoded.String() {
		t.Fatal("couldnt serialize equally")
	}
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
