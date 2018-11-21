package hbbft

/*
DEPRECATED
*/
import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"net/http"
	"reflect"
	"strings"
	"sync"

	"github.com/YZhenY/torus/dkgnode"
	"github.com/anthdm/hbbft"
	"github.com/fatih/structs"
	"github.com/intel-go/fastjson"
	"github.com/osamingo/jsonrpc"
	"github.com/rs/cors"
)

// NewTransport implements a local Transport. This is used to test hbbft
// without going over the network.
type NewTransport struct {
	lock          sync.RWMutex
	peers         map[uint64]*NewTransport
	nodeReference *dkgnode.NodeReference
	ConsumeCh     chan hbbft.RPC
	addr          uint64
}

// NewNewTransport returns a new NewTransport.
func NewNewTransport(addr uint64, nodeReference *dkgnode.NodeReference) *NewTransport {
	return &NewTransport{
		peers:         make(map[uint64]*NewTransport),
		nodeReference: nodeReference,
		ConsumeCh:     make(chan hbbft.RPC, 1024), // nodes * nodes should be fine here,
		addr:          addr,
	}
}

// Transport is an interface that allows the abstraction of network transports.

// Consume returns a channel used for consuming and responding to RPC
// requests.
// Consume implements the Transport interface.
func (t *NewTransport) Consume() <-chan hbbft.RPC {
	return t.ConsumeCh
}

// SendProofMessages will equally spread the given messages under the
// participating nodes.
// SendProofMessages implements the Transport interface.
func (t *NewTransport) SendProofMessages(id uint64, msgs []interface{}) error {
	i := 0
	for addr := range t.peers {
		if err := t.makeRPC(id, addr, msgs[i]); err != nil {
			return err
		}
		i++
	}
	return nil
}

// Broadcast multicasts the given messages to all connected nodes.
// Broadcast implements the Transport interface.
func (t *NewTransport) Broadcast(id uint64, msg interface{}) error {
	for addr := range t.peers {
		if err := t.makeRPC(id, addr, msg); err != nil {
			return err
		}
	}
	return nil
}

// SendMessage implements the transport interface.
func (t *NewTransport) SendMessage(from, to uint64, msg interface{}) error {
	return t.makeRPC(from, to, msg)
}

// Connect is used to connect this tranport to another transport.
// Connect implements the Transport interface.
func (t *NewTransport) Connect(addr uint64, tr hbbft.Transport) {
	trans := tr.(*NewTransport)
	t.lock.Lock()
	defer t.lock.Unlock()
	t.peers[addr] = trans
}

// Addr returns the address of the transport. We address transport by the
// id of the node.
// Addr implements the Transport interface.
func (t *NewTransport) Addr() uint64 {
	return t.addr
}

type HbbftRPC struct {
	NodeID  uint64
	Payload interface{}
	Type    []string
}

var listOfHbbbftStructs = []string{
	"hbbft.HBMessage",
	"hbbft.ACSMessage",
	"hbbft.BroadcastMessage",
	"hbbft.ProofRequest",
	"hbbft.AgreementMessage",
	"hbbft.AuxRequest",
	"hbbft.EchoRequest",
	"hbbft.MessageTuple",
	"hbbft.ReadyRequest",
	"hbbft.BvalRequest",
}

//Look into hbbft struct and return type
func printHbbbftMessageType(msg interface{}) ([]string, error) {
	endType := make([]string, 1)
	for i := range listOfHbbbftStructs {
		if strings.Contains(reflect.TypeOf(msg).String(), listOfHbbbftStructs[i]) {
			//check for pointers
			if strings.Contains(reflect.TypeOf(msg).String(), "*") {
				endType[0] = "*" + listOfHbbbftStructs[i]
			} else {
				endType[0] = listOfHbbbftStructs[i]
			}
		}
	}
	structsMsg := structs.New(msg)
	if payload, found := structsMsg.FieldOk("Payload"); found {
		str, err := printHbbbftMessageType(payload.Value())
		if err != nil {
			return nil, err
		}
		endType = append(endType, str...)
	}

	//Catering for edgecase hbbft.Agreement Message field interface{}
	//THIS ASSUMES THAT structs have EITHER Message OR Payload NOT BOTH
	if msgField, found := structsMsg.FieldOk("Message"); found {
		str, err := printHbbbftMessageType(msgField.Value())
		if err != nil {
			return nil, err
		}
		endType = append(endType, str...)
	}

	return endType, nil
}

func (t *NewTransport) makeRPC(id, addr uint64, msg interface{}) error {
	t.lock.RLock()
	peer, ok := t.peers[addr]
	t.lock.RUnlock()

	if !ok {
		return fmt.Errorf("failed to connect with %d", addr)
	}
	//form type of message
	var typeOfMessage = make([]string, 1)
	if strings.Compare(reflect.TypeOf(msg).String(), "hbbft.HBMessage") == 0 {
		typeOfMessage, _ = printHbbbftMessageType(msg)
	} else {
		typeOfMessage[0] = reflect.TypeOf(msg).String()
	}

	_, err := peer.nodeReference.JSONClient.Call(
		"hbbft",
		&HbbftRPC{
			NodeID:  id,
			Payload: msg,
			Type:    typeOfMessage,
		})
	if err != nil {
		fmt.Println(err)
	}
	//hbbft.go responds to this request
	return nil
}

type HbbftHandler struct {
	node          *Server
	nodes         []*Server
	nodeTransport *NewTransport
}

func (h HbbftHandler) ServeJSONRPC(c context.Context, params *fastjson.RawMessage) (interface{}, *jsonrpc.Error) {

	var p HbbftRPC
	if err := jsonrpc.Unmarshal(params, &p); err != nil {
		fmt.Println("json parse error", err)
		return nil, err
	}

	if strings.Contains(p.Type[0], "h") {
		restructedMsg, err := restructHbbfftMessage(p.Payload, p.Type)
		if err != nil {
			fmt.Println("ERROR", err)
		}
		h.nodeTransport.ConsumeCh <- hbbft.RPC{p.NodeID, restructedMsg}
	} else {
		h.nodeTransport.ConsumeCh <- hbbft.RPC{p.NodeID, p.Payload}
	}

	return nil, nil
}

func restructHbbfftMessage(msg interface{}, typeDescriber []string) (interface{}, error) {
	if len(typeDescriber) == 0 {
		return nil, nil
	}
	mapMessage := msg.(map[string]interface{})
	//caters for pointers
	var typeOfMsg string
	if strings.Contains(typeDescriber[0], "*") {
		typeOfMsg = trimLeftChar(typeDescriber[0])
	} else {
		typeOfMsg = typeDescriber[0]
	}

	var restructuredMsg interface{}
	// var castedMsg interface{}
	switch typeOfMsg {
	case "hbbft.HBMessage":
		payload, err := restructHbbfftMessage(mapMessage["Payload"], typeDescriber[1:])
		if err != nil {
			return nil, err
		}
		restructuredMsg = hbbft.HBMessage{
			Epoch:   uint64(mapMessage["Epoch"].(float64)),
			Payload: payload,
		}
		castedMsg := restructuredMsg.(hbbft.HBMessage)
		if strings.Contains(typeDescriber[0], "*") {
			return &castedMsg, nil
		}
		return castedMsg, nil
	case "hbbft.ACSMessage":
		payload, err := restructHbbfftMessage(mapMessage["Payload"], typeDescriber[1:])
		if err != nil {
			return nil, err
		}
		restructuredMsg = hbbft.ACSMessage{
			ProposerID: uint64(mapMessage["ProposerID"].(float64)),
			Payload:    payload,
		}
		castedMsg := restructuredMsg.(hbbft.ACSMessage)
		if strings.Contains(typeDescriber[0], "*") {
			return &castedMsg, nil
		}
		return castedMsg, nil
		// return &tmp, nil
	case "hbbft.BroadcastMessage":
		payload, err := restructHbbfftMessage(mapMessage["Payload"], typeDescriber[1:])
		if err != nil {
			return nil, err
		}
		restructuredMsg = hbbft.BroadcastMessage{
			Payload: payload,
		}
		castedMsg := restructuredMsg.(hbbft.BroadcastMessage)
		if strings.Contains(typeDescriber[0], "*") {
			return &castedMsg, nil
		}
		return castedMsg, nil
	case "hbbft.ProofRequest":
		rootHash, err := base64.StdEncoding.DecodeString(mapMessage["RootHash"].(string))
		if err != nil {
			return nil, err
		}
		proof := make([][]byte, len(mapMessage["Proof"].([]interface{})))
		for i, val := range mapMessage["Proof"].([]interface{}) {
			b, err := base64.StdEncoding.DecodeString(val.(string))
			if err != nil {
				return nil, err
			}
			proof[i] = b
		}
		restructuredMsg = hbbft.ProofRequest{
			RootHash: rootHash,
			Proof:    proof,
			Index:    int(mapMessage["Index"].(float64)),
			Leaves:   int(mapMessage["Leaves"].(float64)),
		}
		castedMsg := restructuredMsg.(hbbft.ProofRequest)
		if strings.Contains(typeDescriber[0], "*") {
			return &castedMsg, nil
		}
		return castedMsg, nil
	case "hbbft.AgreementMessage":
		fieldMsg, err := restructHbbfftMessage(mapMessage["Message"], typeDescriber[1:])
		if err != nil {
			return nil, err
		}
		restructuredMsg = hbbft.AgreementMessage{
			Epoch:   int(mapMessage["Epoch"].(float64)),
			Message: fieldMsg,
		}
		castedMsg := restructuredMsg.(hbbft.AgreementMessage)
		if strings.Contains(typeDescriber[0], "*") {
			return &castedMsg, nil
		}
		return castedMsg, nil
	case "hbbft.AuxRequest":
		restructuredMsg = hbbft.AuxRequest{
			Value: mapMessage["Value"].(bool),
		}
		castedMsg := restructuredMsg.(hbbft.AuxRequest)
		if strings.Contains(typeDescriber[0], "*") {
			return &castedMsg, nil
		}
		return castedMsg, nil
	case "hbbft.EchoRequest":
		rootHash, err := base64.StdEncoding.DecodeString(mapMessage["RootHash"].(string))
		if err != nil {
			return nil, err
		}
		proof := make([][]byte, len(mapMessage["Proof"].([]interface{})))
		for i, val := range mapMessage["Proof"].([]interface{}) {
			b, err := base64.StdEncoding.DecodeString(val.(string))
			if err != nil {
				return nil, err
			}
			proof[i] = b
		}
		// fmt.Println("ECHO REQUEST: ", mapMessage)
		restructuredMsg = hbbft.EchoRequest{
			ProofRequest: hbbft.ProofRequest{
				RootHash: rootHash,
				Proof:    proof,
				Index:    int(mapMessage["Index"].(float64)),
				Leaves:   int(mapMessage["Leaves"].(float64)),
			},
		}
		castedMsg := restructuredMsg.(hbbft.EchoRequest)
		if strings.Contains(typeDescriber[0], "*") {
			return &castedMsg, nil
		}
		return castedMsg, nil
	case "hbbft.MessageTuple":
		payload, err := restructHbbfftMessage(mapMessage["Payload"], typeDescriber[1:])
		if err != nil {
			return nil, err
		}
		restructuredMsg = hbbft.MessageTuple{
			To:      uint64(mapMessage["To"].(float64)),
			Payload: payload,
		}
		castedMsg := restructuredMsg.(hbbft.MessageTuple)
		if strings.Contains(typeDescriber[0], "*") {
			return &castedMsg, nil
		}
		return castedMsg, nil
	case "hbbft.ReadyRequest":
		rootHash, err := base64.StdEncoding.DecodeString(mapMessage["RootHash"].(string))
		if err != nil {
			return nil, err
		}
		restructuredMsg = hbbft.ReadyRequest{
			RootHash: rootHash,
		}
		castedMsg := restructuredMsg.(hbbft.ReadyRequest)
		if strings.Contains(typeDescriber[0], "*") {
			return &castedMsg, nil
		}
		return castedMsg, nil
	case "hbbft.BvalRequest":
		restructuredMsg = hbbft.BvalRequest{
			Value: mapMessage["Value"].(bool),
		}
		castedMsg := restructuredMsg.(hbbft.BvalRequest)
		if strings.Contains(typeDescriber[0], "*") {
			return &castedMsg, nil
		}
		return castedMsg, nil
	default:
		return nil, errors.New("did not fit any cases")
	}

	// THIS DOESNT WORK I DONT KNOW WHY
	//return pointer if needed
	// if strings.Contains(typeDescriber[0], "*") {
	// 	return &castedMsg, nil
	// }
	// return castedMsg, nil

}

func trimLeftChar(s string) string {
	for i := range s {
		if i > 0 {
			return s[i:]
		}
	}
	return s[:0]
}

func setUpHbbftServer(port string, node *Server, nodes []*Server, nodeTransport *NewTransport) {
	mr := jsonrpc.NewMethodRepository()
	if err := mr.RegisterMethod("hbbft", HbbftHandler{node, nodes, nodeTransport}, HbbftRPC{}, HbbftRPC{}); err != nil {
		log.Fatalln(err)
	}

	mux := http.NewServeMux()
	mux.Handle("/jrpc", mr)
	mux.HandleFunc("/jrpc/debug", mr.ServeDebug)
	// fmt.Println(port)
	handler := cors.Default().Handler(mux)
	// if suite.Flags.Production {
	// 	if err := http.ListenAndServeTLS(":443",
	// 		"/etc/letsencrypt/live/"+suite.Config.HostName+"/fullchain.pem",
	// 		"/etc/letsencrypt/live/"+suite.Config.HostName+"/privkey.pem",
	// 		handler,
	// 	); err != nil {
	// 		log.Fatalln(err)
	// 	}
	// } else {
	fmt.Println("listening to ", port)
	if err := http.ListenAndServe(":"+port, handler); err != nil {
		log.Fatalln(err)
	}

	// }
}
