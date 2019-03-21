package dkgnode

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"log"
	"math/big"
	"time"

	ethCommon "github.com/ethereum/go-ethereum/common"
	"github.com/gogo/protobuf/proto"
	pstore "github.com/libp2p/go-libp2p-peerstore"
	"github.com/torusresearch/torus-public/logging"

	ggio "github.com/gogo/protobuf/io"
	libp2p "github.com/libp2p/go-libp2p"
	crypto "github.com/libp2p/go-libp2p-crypto"
	host "github.com/libp2p/go-libp2p-host"
	inet "github.com/libp2p/go-libp2p-net"
	peer "github.com/libp2p/go-libp2p-peer"
	protocol "github.com/libp2p/go-libp2p-protocol"
	ma "github.com/multiformats/go-multiaddr"
	p2p "github.com/torusresearch/torus-public/dkgnode/pb"
)

// node client version
const clientVersion = "go-p2p-node/0.0.1"

type NodeReference struct {
	Address         *ethCommon.Address
	Index           *big.Int
	PeerID          peer.ID
	PublicKey       *ecdsa.PublicKey
	TMP2PConnection string
	P2PConnection   string
}
type P2PSuite struct {
	host.Host
	HostAddress ma.Multiaddr
	PingProto   *PingProtocol
}

// SetupP2PHost creates a LibP2P host with an ID being the supplied private key and initiates
// the required suite
func SetupP2PHost(suite *Suite) (host.Host, error) {

	// Set keypair to node private key
	priv, err := crypto.UnmarshalSecp256k1PrivateKey(suite.EthSuite.NodePrivateKey.D.Bytes())
	if err != nil {
		panic(err)
	}

	//TODO: Configure security right
	opts := []libp2p.Option{
		libp2p.ListenAddrStrings(suite.Config.P2PListenAddress),
		libp2p.Identity(priv),
		libp2p.DisableRelay(),
	}

	h, err := libp2p.New(context.Background(), opts...)
	if err != nil {
		return nil, err
	}

	// Build host multiaddress
	hostAddr, _ := ma.NewMultiaddr(fmt.Sprintf("/ipfs/%s", h.ID().Pretty()))

	// Now we can build a full multiaddress to reach this host
	// by encapsulating both addresses:
	addr := h.Addrs()[0]
	fullAddr := addr.Encapsulate(hostAddr)
	logging.Infof("P2P Full Address: %s\n", fullAddr)

	suite.P2PSuite = &P2PSuite{
		Host:        h,
		HostAddress: fullAddr,
	}

	// Set a stream handlers or protocols
	suite.P2PSuite.PingProto = NewPingProtocol(suite.P2PSuite)

	return h, nil
}

// Authenticate incoming p2p message
// message: a protobufs go data object
// data: common p2p message data
func (localHost *P2PSuite) authenticateMessage(message proto.Message, data *p2p.MessageData) bool {
	// store a temp ref to signature and remove it from message data
	// sign is a string to allow easy reset to zero-value (empty string)
	sign := data.Sign
	data.Sign = nil

	// marshall data without the signature to protobufs3 binary format
	bin, err := proto.Marshal(message)
	if err != nil {
		log.Println(err, "failed to marshal pb message")
		return false
	}

	// restore sig in message data (for possible future use)
	data.Sign = sign

	// restore peer id binary format from base58 encoded node id data
	peerId, err := peer.IDB58Decode(data.NodeId)
	if err != nil {
		log.Println(err, "Failed to decode node id from base58")
		return false
	}

	// verify the data was authored by the signing peer identified by the public key
	// and signature included in the message
	return localHost.verifyData(bin, []byte(sign), peerId, data.NodePubKey)
}

// sign an outgoing p2p message payload
func (localHost *P2PSuite) signProtoMessage(message proto.Message) ([]byte, error) {
	data, err := proto.Marshal(message)
	if err != nil {
		return nil, err
	}
	return localHost.signData(data)
}

// sign binary data using the local node's private key
func (localHost *P2PSuite) signData(data []byte) ([]byte, error) {
	key := localHost.Peerstore().PrivKey(localHost.ID())
	res, err := key.Sign(data)
	return res, err
}

// Verify incoming p2p message data integrity
// data: data to verify
// signature: author signature provided in the message payload
// peerId: author peer id from the message payload
// pubKeyData: author public key from the message payload
func (localHost *P2PSuite) verifyData(data []byte, signature []byte, peerId peer.ID, pubKeyData []byte) bool {
	key, err := crypto.UnmarshalPublicKey(pubKeyData)
	if err != nil {
		log.Println(err, "Failed to extract key from message key data")
		return false
	}

	// extract node id from the provided public key
	idFromKey, err := peer.IDFromPublicKey(key)

	if err != nil {
		log.Println(err, "Failed to extract peer id from public key")
		return false
	}

	// verify that message author node id matches the provided node public key
	if idFromKey != peerId {
		log.Println(err, "P2PSuite id and provided public key mismatch")
		return false
	}

	res, err := key.Verify(data, signature)
	if err != nil {
		log.Println(err, "Error authenticating data")
		return false
	}

	return res
}

// helper method - generate message data shared between all node's p2p protocols
// messageId: unique for requests, copied from request for responses
func (localHost *P2PSuite) NewMessageData(messageId string, gossip bool) *p2p.MessageData {
	// Add protobufs bin data for message author public key
	// this is useful for authenticating  messages forwarded by a node authored by another node
	nodePubKey, err := localHost.Peerstore().PubKey(localHost.ID()).Bytes()

	if err != nil {
		panic("Failed to get public key for sender from local peer store.")
	}

	return &p2p.MessageData{ClientVersion: clientVersion,
		NodeId:     peer.IDB58Encode(localHost.ID()),
		NodePubKey: nodePubKey,
		Timestamp:  time.Now().Unix(),
		Id:         messageId,
		Gossip:     gossip}
}

// helper method - writes a protobuf go data object to a network stream
// data: reference of protobuf go data object to send (not the object itself)
// s: network stream to write the data to
func (localHost *P2PSuite) sendProtoMessage(id peer.ID, p protocol.ID, data proto.Message) error {
	s, err := localHost.NewStream(context.Background(), id, p)
	if err != nil {
		return err
	}
	writer := ggio.NewFullWriter(s)
	err = writer.WriteMsg(data)
	if err != nil {
		s.Reset()
		return err
	}
	// FullClose closes the stream and waits for the other side to close their half.
	err = inet.FullClose(s)
	if err != nil {
		s.Reset()
		return err
	}
	return nil
}

func connectToP2PNode(suite *Suite, epoch big.Int, nodeAddress ethCommon.Address) (*NodeReference, error) {
	details, err := suite.EthSuite.NodeListContract.AddressToNodeDetailsLog(nil, nodeAddress, &epoch)
	if err != nil {
		return nil, err
	}

	ipfsaddr, err := ma.NewMultiaddr(details.P2pListenAddress)
	if err != nil {
		log.Fatalln(err)
	}

	pid, err := ipfsaddr.ValueForProtocol(ma.P_IPFS)
	if err != nil {
		log.Fatalln(err)
	}

	peerid, err := peer.IDB58Decode(pid)
	if err != nil {
		log.Fatalln(err)
	}

	peerAdded := false
	for _, peer := range suite.P2PSuite.Peerstore().Peers() {
		if peer == peerid {
			peerAdded = true
		}
	}

	//ignore pings to self
	if peerid != suite.P2PSuite.ID() && !peerAdded {
		logging.Debugf("Adding %s to addressbook", details.P2pListenAddress)
		// Decapsulate the /ipfs/<peerID> part from the target
		// /ip4/<a.b.c.d>/ipfs/<peer> becomes /ip4/<a.b.c.d>
		targetPeerAddr, _ := ma.NewMultiaddr(fmt.Sprintf("/ipfs/%s", peer.IDB58Encode(peerid)))
		targetAddr := ipfsaddr.Decapsulate(targetPeerAddr)

		// We have a peer ID and a targetAddr so we add it to the peerstore
		// so LibP2P knows how to contact it
		suite.P2PSuite.Peerstore().AddAddr(peerid, targetAddr, pstore.PermanentAddrTTL)

		err = suite.P2PSuite.PingProto.Ping(peerid)
		if err != nil {
			return nil, err
		}
	}

	return &NodeReference{
		Address:         &nodeAddress,
		PeerID:          peerid,
		Index:           details.Position,
		PublicKey:       &ecdsa.PublicKey{Curve: suite.EthSuite.secp, X: details.PubKx, Y: details.PubKy},
		TMP2PConnection: details.TmP2PListenAddress,
		P2PConnection:   details.P2pListenAddress,
	}, nil
}
