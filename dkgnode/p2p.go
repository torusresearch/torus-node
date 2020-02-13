package dkgnode

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"math/big"
	"time"

	pcmn "github.com/torusresearch/torus-node/common"
	"github.com/torusresearch/torus-node/telemetry"

	libp2p "github.com/libp2p/go-libp2p"
	libp2pcrypto "github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/helpers"
	"github.com/libp2p/go-libp2p-core/host"
	inet "github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	pstore "github.com/libp2p/go-libp2p-core/peerstore"
	"github.com/libp2p/go-libp2p-core/protocol"
	ma "github.com/multiformats/go-multiaddr"
	logging "github.com/sirupsen/logrus"
	"github.com/torusresearch/bijson"
	"github.com/torusresearch/torus-common/common"
	"github.com/torusresearch/torus-common/crypto"
	"github.com/torusresearch/torus-node/config"
	"github.com/torusresearch/torus-node/eventbus"
	"github.com/torusresearch/torus-node/tcontext"
	"github.com/torusresearch/torus-node/version"
)

var p2pServiceLibrary ServiceLibrary

type P2PMessage interface {
	GetTimestamp() big.Int
	GetId() string
	GetGossip() bool
	GetNodeId() string
	GetNodePubKey() []byte
	GetSign() []byte
	SetSign(sig []byte)
	GetMsgType() string
	GetPayload() []byte
	GetSerializedBody() []byte
}

type p2pMessageVersion string

func CreateP2PBasicMsg(r P2PBasicMsgRaw) P2PBasicMsg {
	return P2PBasicMsg{
		Version:    p2pMessageVersion(version.NodeVersion),
		Timestamp:  r.Timestamp,
		Id:         r.Id,
		Gossip:     r.Gossip,
		NodeId:     r.NodeId,
		NodePubKey: r.NodePubKey,
		Sign:       r.Sign,
		MsgType:    r.MsgType,
		Payload:    r.Payload,
	}
}

type P2PBasicMsgRaw struct {
	// shared between all requests
	Timestamp  big.Int
	Id         string
	Gossip     bool
	NodeId     string
	NodePubKey []byte
	Sign       []byte
	MsgType    string
	Payload    []byte
}

type P2PBasicMsg struct {
	// shared between all requests
	Version    p2pMessageVersion `json:"version,omitempty"`
	Timestamp  big.Int           `json:"timestamp,omitempty"`  // unix time
	Id         string            `json:"id,omitempty"`         // allows requesters to use request data when processing a response
	Gossip     bool              `json:"gossip,omitempty"`     // true to have receiver peer gossip the message to neighbors
	NodeId     string            `json:"nodeId,omitempty"`     // id of node that created the message (not the peer that may have sent it). =base58(multihash(nodePubKey))
	NodePubKey []byte            `json:"nodePubKey,omitempty"` // Authoring node Secp256k1 public key (32bytes)
	Sign       []byte            `json:"sign,omitempty"`       // signature of message data + method specific data by message authoring node.
	MsgType    string            `json:"msgtype,omitempty"`    // identifyng message type
	Payload    []byte            `json:"payload"`              // payload data to be unmarshalled
}

type P2PSuite struct {
	host.Host
	HostAddress ma.Multiaddr
	PingProto   *PingProtocol
}

func (msg *P2PBasicMsg) GetTimestamp() big.Int {
	return msg.Timestamp
}
func (msg *P2PBasicMsg) GetId() string {
	return msg.Id
}
func (msg *P2PBasicMsg) GetGossip() bool {
	return msg.Gossip
}
func (msg *P2PBasicMsg) GetNodeId() string {
	return msg.NodeId
}

// GetNodePubKey - in older versions <= v0.7.0
// returns pubkey that is unmarshallable to p2pID secp pubkey
// Post v0.7.0 this returns a pubkey that is unmarshallable to common.Point{}
func (msg *P2PBasicMsg) GetNodePubKey() []byte {
	return msg.NodePubKey
}
func (msg *P2PBasicMsg) GetSign() []byte {
	return msg.Sign
}
func (msg *P2PBasicMsg) GetMsgType() string {
	return msg.MsgType
}
func (msg *P2PBasicMsg) GetPayload() []byte {
	return msg.Payload
}
func (msg *P2PBasicMsg) SetSign(sig []byte) {
	msg.Sign = sig
}
func (msg P2PBasicMsg) GetSerializedBody() []byte {
	msg.SetSign(nil)
	// marshall msg without the signature to bytes format
	bin, err := bijson.Marshal(msg)
	if err != nil {
		logging.Errorf("failed to marshal pb message %v", err)
		return nil
	}
	return bin
}

func NewP2PService(ctx context.Context, eventBus eventbus.Bus) *BaseService {
	p2pCtx, cancel := context.WithCancel(context.WithValue(ctx, ContextID, "p2p"))
	p2pService := P2PService{
		cancel:        cancel,
		parentContext: ctx,
		context:       p2pCtx,
		eventBus:      eventBus,
	}
	p2pServiceLibrary = NewServiceLibrary(p2pService.eventBus, p2pService.Name())
	return NewBaseService(&p2pService)
}

type P2PService struct {
	bs            *BaseService
	cancel        context.CancelFunc
	parentContext context.Context
	context       context.Context
	eventBus      eventbus.Bus

	host        host.Host
	hostAddress ma.Multiaddr
	// p2pNodeKey                 *p2p.NodeKey
	pingProto                  *PingProtocol
	authenticateMessage        func(data P2PMessage) (err error)
	authenticateMessageInEpoch func(data P2PMessage, epoch int) (err error)
	signData                   func(data []byte) (rawSig []byte)
}

func (p *P2PService) StopForwardP2PToEventBus(proto string) {
	p.host.RemoveStreamHandler(protocol.ID(proto))
}

func (p *P2PService) ForwardP2PToEventBus(proto string) {
	p.host.SetStreamHandler(protocol.ID(proto), func(s inet.Stream) {
		buf, err := ioutil.ReadAll(s)
		if err != nil {
			e := s.Reset()
			logging.WithError(err).Error("could not ReadAll from io")
			if e != nil {
				logging.WithError(e).Error("could not reset stream")
			}
			return
		}
		s.Close()

		var p2pMsg P2PBasicMsg
		err = bijson.Unmarshal(buf, &p2pMsg)
		if err != nil {
			logging.WithError(err).Error("could not unmarshal p2pmsg")
			return
		}
		logging.WithFields(logging.Fields{
			"proto":  proto,
			"p2pMsg": stringify(p2pMsg),
		}).Debug("SetStreamHandler working and forwarding")

		err = p.authenticateMessage(&p2pMsg)
		if err != nil {
			logging.WithField("Payload", string(p2pMsg.Payload)).Error("failed to authenticate p2pMsg")
			return
		}
		p.eventBus.Publish("p2p:forward:"+proto, p2pMsg)
	})
}

// SetStreamHandler sets up listeners and triggers a query over to the P2P service
// which forwards P2P messages to the event bus.

func padPrivKeyBytes(kBytes []byte) []byte {
	if len(kBytes) < 32 {
		tmp := make([]byte, 32)
		copy(tmp[32-len(kBytes):], kBytes)
		return tmp
	}
	return kBytes
}

func (p *P2PService) OnStart() error {
	// Set keypair to node private key
	k := p2pServiceLibrary.EthereumMethods().GetSelfPrivateKey()

	priv, err := libp2pcrypto.UnmarshalSecp256k1PrivateKey(padPrivKeyBytes(k.Bytes()))
	if err != nil {
		logging.WithError(err).Fatal("could not Unmarshal privateKey")
	}

	opts := []libp2p.Option{
		libp2p.ListenAddrStrings(config.GlobalConfig.P2PListenAddress),
		libp2p.Identity(priv),
		libp2p.DisableRelay(),
	}

	h, err := libp2p.New(tcontext.From(context.Background()), opts...)
	if err != nil {
		return err
	}

	// Build host multiaddress
	hostAddr, _ := ma.NewMultiaddr(fmt.Sprintf("/ipfs/%s", h.ID().Pretty()))

	// Now we can build a full multiaddress to reach this host
	// by encapsulating both addresses:
	addr := h.Addrs()[0]
	fullAddr := addr.Encapsulate(hostAddr)
	logging.WithField("fullAddress", fullAddr).Info("P2P")

	p.host = h
	p.hostAddress = fullAddr
	p.pingProto = NewPingProtocol(p)
	p.authenticateMessage = authenticateMessage
	p.authenticateMessageInEpoch = authenticateMessageInEpoch
	p.signData = p2pServiceLibrary.EthereumMethods().SelfSignData

	logging.WithField("LocalHostID", p.host.ID().String()).Debug()
	return nil
}

func (p2p *P2PService) Name() string {
	return "p2p"
}

func (p2p *P2PService) OnStop() error {
	return nil
}

func (p2p *P2PService) Call(method string, args ...interface{}) (interface{}, error) {
	telemetry.IncrementCounter(pcmn.TelemetryConstants.Generic.TotalServiceCalls, pcmn.TelemetryConstants.P2P.Prefix)

	switch method {
	// ID() peer.ID
	case "id":
		telemetry.IncrementCounter(pcmn.TelemetryConstants.P2P.GetPeerIDCounter, pcmn.TelemetryConstants.P2P.Prefix)

		return p2p.host.ID(), nil
	// SetStreamHandler(protoName string, handler func(StreamMessage)) (err error)
	case "set_stream_handler":
		telemetry.IncrementCounter(pcmn.TelemetryConstants.P2P.SetStreamHandlerCounter, pcmn.TelemetryConstants.P2P.Prefix)

		var args0 string
		_ = castOrUnmarshal(args[0], &args0)

		proto := args0
		p2p.ForwardP2PToEventBus(proto)
		return true, nil
	// RemoveStreamHandler(protoName string) (err error)
	case "remove_stream_handler":
		telemetry.IncrementCounter(pcmn.TelemetryConstants.P2P.RemoveStreamHandlerCounter, pcmn.TelemetryConstants.P2P.Prefix)

		var args0 string
		_ = castOrUnmarshal(args[0], &args0)

		proto := args0
		p2p.StopForwardP2PToEventBus(proto)
		return true, nil
	// AuthenticateMessage(p2pMsg P2PMessage) (err error)
	case "authenticate_message":
		telemetry.IncrementCounter(pcmn.TelemetryConstants.P2P.AuthenticateMessageCounter, pcmn.TelemetryConstants.P2P.Prefix)

		var args0 P2PBasicMsg
		_ = castOrUnmarshal(args[0], &args0)

		p2pMsg := args0
		err := p2p.authenticateMessage(&p2pMsg)
		return nil, err
	case "authenticate_message_in_epoch":
		telemetry.IncrementCounter(pcmn.TelemetryConstants.P2P.AuthenticateMessageInEpochCounter, pcmn.TelemetryConstants.P2P.Prefix)

		var args0 P2PBasicMsg
		var args1 int
		_ = castOrUnmarshal(args[0], &args0)
		_ = castOrUnmarshal(args[1], &args1)

		p2pMsg := args0
		err := p2p.authenticateMessageInEpoch(&p2pMsg, args1)
		return nil, err
	// NewP2PMessage(messageId string, gossip bool, payload []byte, msgType string) (newMsg P2PBasicMsg)
	case "new_p2p_message":
		telemetry.IncrementCounter(pcmn.TelemetryConstants.P2P.NewP2PMessageCounter, pcmn.TelemetryConstants.P2P.Prefix)

		var args0, args3 string
		var args1 bool
		var args2 []byte
		_ = castOrUnmarshal(args[0], &args0)
		_ = castOrUnmarshal(args[1], &args1)
		_ = castOrUnmarshal(args[2], &args2)
		_ = castOrUnmarshal(args[3], &args3)

		newMsg := *p2p.NewP2PMessage(args0, args1, args2, args3)
		return newMsg, nil
	// SignP2PMessage(message P2PMessage) (signature []byte, err error)  NOT CONCURRENT SAFE
	case "sign_p2p_message":
		telemetry.IncrementCounter(pcmn.TelemetryConstants.P2P.SignP2PMessageCounter, pcmn.TelemetryConstants.P2P.Prefix)

		var args0 P2PBasicMsg
		_ = castOrUnmarshal(args[0], &args0)

		sig, err := p2p.signP2PMessage(&args0)
		return sig, err
	// SendP2PMessage(id peer.ID, p protocol.ID, msg P2PMessage) error
	case "send_p2p_message":
		telemetry.IncrementCounter(pcmn.TelemetryConstants.P2P.SendP2PMessageCounter, pcmn.TelemetryConstants.P2P.Prefix)

		var args0 peer.ID
		var args1 protocol.ID
		var args2 P2PBasicMsg
		_ = castOrUnmarshal(args[0], &args0)
		_ = castOrUnmarshal(args[1], &args1)
		_ = castOrUnmarshal(args[2], &args2)

		ctx := context.Background()
		err := p2p.sendP2PMessage(ctx, args0, args1, &args2)
		return nil, err
	// ConnectToP2PNode(nodeP2PConnection string, nodePeerID peer.ID) error
	case "connect_to_p2p_node":
		telemetry.IncrementCounter(pcmn.TelemetryConstants.P2P.ConnectToP2PNodeCounter, pcmn.TelemetryConstants.P2P.Prefix)

		var args0 string
		var args1 peer.ID
		_ = castOrUnmarshal(args[0], &args0)
		_ = castOrUnmarshal(args[1], &args1)

		err := p2p.ConnectToP2PNode(args0, args1)
		return nil, err
	// GetHostAddress() (hostAddres ma.Multiaddr) NOT CONCURRENT SAFE
	case "get_host_address":
		telemetry.IncrementCounter(pcmn.TelemetryConstants.P2P.GetHostAddressCounter, pcmn.TelemetryConstants.P2P.Prefix)

		if p2p.hostAddress == nil {
			return nil, errors.New("hostAddress not initialized")
		}
		return p2p.hostAddress.String(), nil
	default:
		return nil, fmt.Errorf("P2P service method %v not found", method)
	}
}

func (p2p *P2PService) SetBaseService(bs *BaseService) {
	p2p.bs = bs
}

// Authenticate incoming p2p message
// message: a protobufs go data object
// data: common p2p message data
func authenticateMessage(data P2PMessage) error {
	var pk common.Point
	rawPk := data.GetNodePubKey()
	err := bijson.Unmarshal(rawPk, &pk)
	if err != nil {
		logging.WithError(err).Error("could not unmarshal rawpk")
		return err
	}

	bin := data.GetSerializedBody()
	// verify the data was authored by the signing peer identified by the public key
	// and signature included in the message
	_, err = p2pServiceLibrary.EthereumMethods().VerifyDataWithNodelist(pk, data.GetSign(), bin)
	return err
}

// Authenticate incoming p2p message with a specified epoch
// message: a protobufs go data object
// data: common p2p message data
func authenticateMessageInEpoch(data P2PMessage, epoch int) error {
	var pk common.Point
	rawPk := data.GetNodePubKey()
	err := bijson.Unmarshal(rawPk, &pk)
	if err != nil {
		return err
	}
	return nil
}

// sign an outgoing p2p message payload
func (p2pService *P2PService) signP2PMessage(message P2PMessage) ([]byte, error) {
	data, err := bijson.Marshal(message)
	if err != nil {
		return nil, err
	}
	return p2pService.signData(data), nil
}

// verifySigOnMsg -  verifies that the corresponding message signature
// corresponds to the provided nodePubKey provided
func verifySigOnMsg(data []byte, signature []byte, rawPubKey []byte) error {
	var pk common.Point
	err := bijson.Unmarshal(rawPubKey, &pk)
	if err != nil {
		return fmt.Errorf("Could not unmarshall NodePubKey when verifyDataWithNodelist %v", err.Error())
	}

	// Check validity of signature
	valid := crypto.VerifyPtFromRaw(data, pk, signature)
	if !valid {
		return fmt.Errorf("invalid ecdsa sig in verifySigOnMsg  %v", data)
	}
	return nil
}

// helper method - generate message data shared between all node's p2p protocols
// messageId: unique for requests, copied from request for responses
func (p2pService *P2PService) NewP2PMessage(messageId string, gossip bool, payload []byte, msgType string) *P2PBasicMsg {
	// Add protobufs bin data for message author public key
	// this is useful for authenticating  messages forwarded by a node authored by another node
	ptPk := p2pServiceLibrary.EthereumMethods().GetSelfPublicKey()
	rawPk, err := bijson.Marshal(ptPk)
	if err != nil {
		logging.Error("could not marshal pk in newp2pmessage" + err.Error())
	}
	p2pBasicMsg := CreateP2PBasicMsg(P2PBasicMsgRaw{
		NodeId:     peer.IDB58Encode(p2pService.host.ID()),
		NodePubKey: rawPk,
		Timestamp:  *big.NewInt(time.Now().Unix()),
		Id:         messageId,
		Gossip:     gossip,
		Payload:    payload,
		MsgType:    msgType,
	})
	return &p2pBasicMsg
}

// helper method - writes a p2pMessage go data object to a network stream
// s: network stream to write the data to
func (localHost *P2PService) sendP2PMessage(ctx context.Context, id peer.ID, p protocol.ID, msg P2PMessage) error {
	data, err := bijson.Marshal(msg)
	if err != nil {
		return err
	}

	s, err := localHost.host.NewStream(ctx, id, p)
	if err != nil {
		return err
	}
	_, err = s.Write(data)
	if err != nil {
		e := s.Reset()
		if e != nil {
			logging.WithError(e).Error("could not reset stream")
		}
		return err
	}
	// FullClose closes the stream and waits for the other side to close their half.
	err = helpers.FullClose(s)
	if err != nil {
		e := s.Reset()
		if e != nil {
			logging.WithError(e).Error("could not reset stream")
		}
		return err
	}
	return nil
}

func (p2p *P2PService) ConnectToP2PNode(nodeP2PConnection string, nodePeerID peer.ID) error {

	peerAdded := false
	// TODO: no need to loop through all, we can reference by addr
	for _, peer := range p2p.host.Peerstore().Peers() {
		if peer == nodePeerID {
			peerAdded = true
		}
	}

	// ignore pings to self
	// Note: restarting the node does not require that they connect to all peers for the epoch
	// since this check is skipped if the peer is already in the peerstore addressbook, and the addressbook
	// is persisted to state
	if nodePeerID != p2p.host.ID() && !peerAdded {
		logging.WithField("nodeP2PConnection", nodeP2PConnection).Debug("adding nodeP2PConnection to addressbook")
		// Decapsulate the /ipfs/<peerID> part from the target
		// /ip4/<a.b.c.d>/ipfs/<peer> becomes /ip4/<a.b.c.d>
		targetPeerAddr, _ := ma.NewMultiaddr(fmt.Sprintf("/ipfs/%s", peer.IDB58Encode(nodePeerID)))
		a, _ := ma.NewMultiaddr(nodeP2PConnection) // TODO: check if we can remove this
		targetAddr := a.Decapsulate(targetPeerAddr)

		// We have a peer ID and a targetAddr so we add it to the peerstore
		// so LibP2P knows how to contact it
		p2p.host.Peerstore().AddAddr(nodePeerID, targetAddr, pstore.PermanentAddrTTL)

		err := p2p.pingProto.Ping(nodePeerID)
		if err != nil {
			return err
		}
	}

	return nil
}
