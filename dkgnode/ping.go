package dkgnode

import (
	"fmt"
	"io/ioutil"
	"log"

	"github.com/torusresearch/torus-public/logging"

	"github.com/gogo/protobuf/proto"
	uuid "github.com/google/uuid"
	inet "github.com/libp2p/go-libp2p-net"
	peer "github.com/libp2p/go-libp2p-peer"
	p2p "github.com/torusresearch/torus-public/dkgnode/pb"
)

// pattern: /protocol-name/request-or-response-message/version
const pingRequest = "/ping/pingreq/0.0.1"
const pingResponse = "/ping/pingresp/0.0.1"

// PingProtocol type
type PingProtocol struct {
	localHost *P2PSuite                   // local host
	requests  map[string]*p2p.PingRequest // used to access request data from response handlers
}

func NewPingProtocol(localHost *P2PSuite) *PingProtocol {
	p := &PingProtocol{localHost: localHost, requests: make(map[string]*p2p.PingRequest)}
	localHost.SetStreamHandler(pingRequest, p.onPingRequest)
	localHost.SetStreamHandler(pingResponse, p.onPingResponse)
	return p
}

// remote peer requests handler
func (p *PingProtocol) onPingRequest(s inet.Stream) {

	// get request data
	data := &p2p.PingRequest{}
	buf, err := ioutil.ReadAll(s)
	if err != nil {
		s.Reset()
		log.Println(err)
		return
	}
	s.Close()

	// unmarshal it
	proto.Unmarshal(buf, data)
	if err != nil {
		log.Println(err)
		return
	}

	log.Printf("%s: Received ping request from %s. Message: %s", s.Conn().LocalPeer(), s.Conn().RemotePeer(), data.Message)

	valid := p.localHost.authenticateMessage(data, data.MessageData)

	if !valid {
		log.Println("Failed to authenticate message")
		return
	}

	// generate response message
	log.Printf("%s: Sending ping response to %s. Message id: %s...", s.Conn().LocalPeer(), s.Conn().RemotePeer(), data.MessageData.Id)

	resp := &p2p.PingResponse{MessageData: p.localHost.NewMessageData(data.MessageData.Id, false),
		Message: fmt.Sprintf("Ping response from %s", p.localHost.ID())}

	// sign the data
	signature, err := p.localHost.signProtoMessage(resp)
	if err != nil {
		log.Println("failed to sign response")
		return
	}

	// add the signature to the message
	resp.MessageData.Sign = signature

	// send the response
	err = p.localHost.sendProtoMessage(s.Conn().RemotePeer(), pingResponse, resp)

	if err == nil {
		logging.Debugf("%s: Ping response to %s sent.", s.Conn().LocalPeer().String(), s.Conn().RemotePeer().String())
	}
}

// remote ping response handler
func (p *PingProtocol) onPingResponse(s inet.Stream) {
	data := &p2p.PingResponse{}
	buf, err := ioutil.ReadAll(s)
	if err != nil {
		s.Reset()
		log.Println(err)
		return
	}
	s.Close()

	// unmarshal it
	proto.Unmarshal(buf, data)
	if err != nil {
		log.Println(err)
		return
	}

	valid := p.localHost.authenticateMessage(data, data.MessageData)

	if !valid {
		log.Println("Failed to authenticate message")
		return
	}

	// locate request data and remove it if found
	_, ok := p.requests[data.MessageData.Id]
	if ok {
		// remove request from map as we have processed it here
		delete(p.requests, data.MessageData.Id)
	} else {
		log.Println("Failed to locate request data boject for response")
		return
	}

	log.Printf("%s: Received ping response from %s. Message id:%s. Message: %s.", s.Conn().LocalPeer(), s.Conn().RemotePeer(), data.MessageData.Id, data.Message)
}

// Pings a peer
func (p *PingProtocol) Ping(peerid peer.ID) error {
	logging.Debugf("%s: Sending ping to: %s....", p.localHost.ID(), peerid)

	// create message data
	req := &p2p.PingRequest{
		MessageData: p.localHost.NewMessageData(uuid.New().String(), false),
		Message:     fmt.Sprintf("Ping from %s", p.localHost.ID()),
	}

	// sign the data
	signature, err := p.localHost.signProtoMessage(req)
	if err != nil {
		return fmt.Errorf("Failed to sign pb data: &s", err)
	}

	// add the signature to the message
	req.MessageData.Sign = signature

	err = p.localHost.sendProtoMessage(peerid, pingRequest, req)
	if err != nil {
		return fmt.Errorf("Failed to send proto message: &s", err)
	}

	// store ref request so response handler has access to it
	p.requests[req.MessageData.Id] = req
	logging.Debugf("%s: Ping to: %s was sent. Message Id: %s, Message: %s", p.localHost.ID(), peerid, req.MessageData.Id, req.Message)
	return nil
}
