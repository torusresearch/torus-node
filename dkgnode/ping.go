package dkgnode

import (
	"fmt"
	"io/ioutil"
	"log"

	pcmn "github.com/torusresearch/torus-node/common"
	"github.com/torusresearch/torus-node/telemetry"

	uuid "github.com/google/uuid"
	inet "github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	logging "github.com/sirupsen/logrus"
	"github.com/torusresearch/bijson"
)

// pattern: /protocol-name/request-or-response-message/version
const pingRequest = "/ping/pingreq/0.0.1"
const pingResponse = "/ping/pingresp/0.0.1"

// PingProtocol type
type PingProtocol struct {
	p2pService *P2PService
	requests   map[string]*P2PBasicMsg // used to access request data from response handlers
}

type Ping struct {
	Message string
}

func NewPingProtocol(p2pService *P2PService) *PingProtocol {
	p := &PingProtocol{p2pService: p2pService, requests: make(map[string]*P2PBasicMsg)}
	p2pService.host.SetStreamHandler(pingRequest, p.onPingRequest)
	p2pService.host.SetStreamHandler(pingResponse, p.onPingResponse)
	return p
}

// remote peer requests handler
func (p *PingProtocol) onPingRequest(s inet.Stream) {
	// get request data
	data := &P2PBasicMsg{}
	buf, err := ioutil.ReadAll(s)
	if err != nil {
		e := s.Reset()
		if e != nil {
			logging.WithError(e).Error("could not reset stream")
		}
		logging.WithError(err).Error("could not readall from inetstream")
		return
	}
	err = s.Close()
	if err != nil {
		logging.WithError(err).Error("could not close stream")
	}

	// unmarshal it
	err = bijson.Unmarshal(buf, data)
	if err != nil {
		logging.WithError(err).Error("could not unmarshal data in onpingrequest")
		return
	}

	err = verifySigOnMsg(data.GetSerializedBody(), data.GetSign(), data.GetNodePubKey())
	if err != nil {
		logging.WithError(err).Error("Failed to verify p2pMsg error")
		return
	}

	// generate response message
	log.Printf("%s: Sending ping response to %s. Message id: %s...", s.Conn().LocalPeer(), s.Conn().RemotePeer(), data.GetId())
	pingBytes, err := bijson.Marshal(Ping{Message: fmt.Sprintf("Ping response from %s", p.p2pService.host.ID())})
	if err != nil {
		logging.Error("could not Marshal ping")
		return
	}
	resp := p.p2pService.NewP2PMessage(data.GetId(), false, pingBytes, "")

	// sign the data
	signature, err := p2pServiceLibrary.P2PMethods().SignP2PMessage(resp)
	if err != nil {
		logging.Error("failed to sign response")
		return
	}

	// add the signature to the message
	resp.Sign = signature

	// send the response
	err = p2pServiceLibrary.P2PMethods().SendP2PMessage(s.Conn().RemotePeer(), pingResponse, resp)

	if err == nil {
		logging.WithFields(logging.Fields{
			"LocalPeer":  s.Conn().LocalPeer().String(),
			"RemotePeer": s.Conn().RemotePeer().String(),
		}).Debug("ping response sent")
	}
}

// remote ping response handler
func (p *PingProtocol) onPingResponse(s inet.Stream) {
	data := &P2PBasicMsg{}
	buf, err := ioutil.ReadAll(s)
	if err != nil {
		logging.WithError(err).Error("could not read from buffer")
		e := s.Reset()
		if e != nil {
			logging.WithError(e).Error("could not reset stream")
		}
		return
	}
	err = s.Close()
	if err != nil {
		logging.WithError(err).Error("could not close stream")
	}

	// unmarshal it
	err = bijson.Unmarshal(buf, data)
	if err != nil {
		logging.WithError(err).Error("could not unmarshal data in onpingresponse")
		return
	}

	err = verifySigOnMsg(data.GetSerializedBody(), data.GetSign(), data.GetNodePubKey())
	if err != nil {
		logging.Error("Failed to verifySigOnMsg message")
		return
	}

	ping := &Ping{}
	err = bijson.Unmarshal(data.Payload, ping)
	if err != nil {
		logging.WithError(err).Error("could not unmarshal payload in onpingresponse")
		return
	}

	// locate request data and remove it if found
	_, ok := p.requests[data.GetId()]
	if ok {
		// remove request from map as we have processed it here
		delete(p.requests, data.GetId())
	} else {
		logging.Error("Failed to locate request data object for response")
		return
	}

	logging.WithFields(logging.Fields{
		"LocalPeer":  s.Conn().LocalPeer(),
		"RemotePeer": s.Conn().RemotePeer(),
		"Message ID": data.GetId(),
		"Message":    ping.Message,
	}).Debug("received ping response")
}

// Pings a peer
func (p *PingProtocol) Ping(peerID peer.ID) error {
	logging.WithFields(logging.Fields{
		"From": p2pServiceLibrary.P2PMethods().ID(),
		"To":   peerID,
	}).Debug("sending ping")
	telemetry.IncrementCounter(pcmn.TelemetryConstants.Generic.TotalServiceCalls, pcmn.TelemetryConstants.Ping.Prefix)

	pBytes, err := bijson.Marshal(Ping{fmt.Sprintf("Ping from %s", p2pServiceLibrary.P2PMethods().ID())})
	if err != nil {
		return fmt.Errorf("Failed marshal ping %v", Ping{fmt.Sprintf("Ping from %s", p2pServiceLibrary.P2PMethods().ID())})

	}
	// create message data
	req := p2pServiceLibrary.P2PMethods().NewP2PMessage(uuid.New().String(), false, pBytes, "")

	// sign the data
	signature, err := p2pServiceLibrary.P2PMethods().SignP2PMessage(&req)
	if err != nil {
		return fmt.Errorf("Failed to sign pb data: %s", err.Error())
	}

	// add the signature to the message
	req.Sign = signature

	err = p2pServiceLibrary.P2PMethods().SendP2PMessage(peerID, pingRequest, &req)
	if err != nil {
		return fmt.Errorf("Failed to send proto message: %s", err.Error())
	}

	// store ref request so response handler has access to it
	p.requests[req.GetId()] = &req

	return nil
}
