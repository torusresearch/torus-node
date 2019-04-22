package dkgnode

import (
	"fmt"
	"io/ioutil"
	"log"

	uuid "github.com/google/uuid"
	inet "github.com/libp2p/go-libp2p-net"
	peer "github.com/libp2p/go-libp2p-peer"
	"github.com/torusresearch/bijson"
	"github.com/torusresearch/torus-public/logging"
)

// pattern: /protocol-name/request-or-response-message/version
const pingRequest = "/ping/pingreq/0.0.1"
const pingResponse = "/ping/pingresp/0.0.1"

// PingProtocol type
type PingProtocol struct {
	localHost *P2PSuite               // local host
	requests  map[string]*P2PBasicMsg // used to access request data from response handlers
}

type Ping struct {
	Message string
}

func NewPingProtocol(localHost *P2PSuite) *PingProtocol {
	p := &PingProtocol{localHost: localHost, requests: make(map[string]*P2PBasicMsg)}
	localHost.SetStreamHandler(pingRequest, p.onPingRequest)
	localHost.SetStreamHandler(pingResponse, p.onPingResponse)
	return p
}

// remote peer requests handler
func (p *PingProtocol) onPingRequest(s inet.Stream) {

	// get request data
	data := &P2PBasicMsg{}
	buf, err := ioutil.ReadAll(s)
	if err != nil {
		s.Reset()
		logging.Error(err.Error())
		return
	}
	s.Close()

	// unmarshal it
	bijson.Unmarshal(buf, data)
	if err != nil {
		logging.Error(err.Error())
		return
	}

	valid := p.localHost.authenticateMessage(data)

	if !valid {
		logging.Error("Failed to authenticate message")
		return
	}

	// generate response message
	log.Printf("%s: Sending ping response to %s. Message id: %s...", s.Conn().LocalPeer(), s.Conn().RemotePeer(), data.GetId())
	pingBytes, err := bijson.Marshal(Ping{Message: fmt.Sprintf("Ping response from %s", p.localHost.ID())})
	if err != nil {
		logging.Error("could not marshal ping")
		return
	}
	resp := p.localHost.NewP2PMessage(data.GetId(), false, pingBytes)

	// sign the data
	signature, err := p.localHost.signP2PMessage(resp)
	if err != nil {
		logging.Error("failed to sign response")
		return
	}

	// add the signature to the message
	resp.Sign = signature

	// send the response
	err = p.localHost.sendP2PMessage(s.Conn().RemotePeer(), pingResponse, resp)

	if err == nil {
		logging.Debugf("%s: Ping response to %s sent.", s.Conn().LocalPeer().String(), s.Conn().RemotePeer().String())
	}
}

// remote ping response handler
func (p *PingProtocol) onPingResponse(s inet.Stream) {
	data := &P2PBasicMsg{}
	buf, err := ioutil.ReadAll(s)
	if err != nil {
		s.Reset()
		logging.Error(err.Error())
		return
	}
	s.Close()

	// unmarshal it
	bijson.Unmarshal(buf, data)
	if err != nil {
		logging.Error(err.Error())
		return
	}

	valid := p.localHost.authenticateMessage(data)

	if !valid {
		logging.Error("Failed to authenticate message")
		return
	}

	ping := &Ping{}
	bijson.Unmarshal(data.Payload, ping)
	if err != nil {
		logging.Error(err.Error())
		return
	}

	// locate request data and remove it if found
	_, ok := p.requests[data.GetId()]
	if ok {
		// remove request from map as we have processed it here
		delete(p.requests, data.GetId())
	} else {
		logging.Error("Failed to locate request data boject for response")
		return
	}

	logging.Debugf("%s: Received ping response from %s. Message id:%s. Message: %s.", s.Conn().LocalPeer(), s.Conn().RemotePeer(), data.GetId(), ping.Message)
}

// Pings a peer
func (p *PingProtocol) Ping(peerid peer.ID) error {
	logging.Debugf("%s: Sending ping to: %s....", p.localHost.ID(), peerid)

	pBytes, err := bijson.Marshal(Ping{fmt.Sprintf("Ping from %s", p.localHost.ID())})
	if err != nil {
		return fmt.Errorf("Failed marshal ping %v", Ping{fmt.Sprintf("Ping from %s", p.localHost.ID())})

	}
	// create message data
	req := p.localHost.NewP2PMessage(uuid.New().String(), false, pBytes)

	// sign the data
	signature, err := p.localHost.signP2PMessage(req)
	if err != nil {
		return fmt.Errorf("Failed to sign pb data: %s", err.Error())
	}

	// add the signature to the message
	req.Sign = signature

	err = p.localHost.sendP2PMessage(peerid, pingRequest, req)
	if err != nil {
		return fmt.Errorf("Failed to send proto message: %s", err.Error())
	}

	// store ref request so response handler has access to it
	p.requests[req.GetId()] = req
	logging.Debugf("%s: Ping to: %s was sent. Message Id: %s, Message: %s", p.localHost.ID(), peerid, req.GetId(), string(req.Payload))
	return nil
}
