package dkgnode

/* All useful imports */
// import (
// 	"crypto/ecdsa"
// 	"encoding/hex"
// 	"encoding/json"
// 	"fmt"
// 	"io/ioutil"

// 	"strings"
// 	"time"

// 	"github.com/Rican7/retry"
// 	"github.com/Rican7/retry/backoff"
// 	"github.com/Rican7/retry/strategy"
// 	tmconfig "github.com/tendermint/tendermint/config"
// 	tmsecp "github.com/tendermint/tendermint/crypto/secp256k1"
// 	tmnode "github.com/tendermint/tendermint/node"
// 	"github.com/tendermint/tendermint/p2p"
// 	"github.com/tendermint/tendermint/privval"
// 	tmtypes "github.com/tendermint/tendermint/types"

// 	"github.com/torusresearch/torus-public/keygen"
// 	"github.com/torusresearch/torus-public/logging"
// 	"github.com/torusresearch/torus-public/pvss"
// 	"github.com/torusresearch/torus-public/telemetry"
// )

import (
	// "fmt"
	"io/ioutil"
	// "log"
	"errors"
	"math/big"
	// uuid "github.com/google/uuid"
	crypto "github.com/libp2p/go-libp2p-crypto"
	inet "github.com/libp2p/go-libp2p-net"
	peer "github.com/libp2p/go-libp2p-peer"
	protocol "github.com/libp2p/go-libp2p-protocol"
	"github.com/torusresearch/bijson"
	// "github.com/torusresearch/torus-public/common"
	"github.com/torusresearch/torus-public/keygen"
	"github.com/torusresearch/torus-public/logging"
)

var keygenConsts = keygenConstants{
	// pattern: /protocol-name/request-or-response-message/starting-endingindex
	RequestPrefix:  "/KEYGEN/KEYGENreq/",
	ResponsePrefix: "/KEYGEN/KEYGENresp/",

	// p2p keygen message types
	Send:  "keygensend",
	Echo:  "keygenecho",
	Ready: "keygenready",

	// bft keygen msg types
	Initiate: "keygeninitiate",
	Complete: "keygendkgcomplete",
}

type keygenConstants struct {
	RequestPrefix  string
	ResponsePrefix string
	Send           string
	Echo           string
	Ready          string
	Initiate       string
	Complete       string
}

type keygenID string // "startingIndex-endingIndex"
func getKeygenID(shareStartingIndex int, shareEndingIndex int) keygenID {
	return keygenID(keygenConsts.RequestPrefix + string(shareStartingIndex) + "-" + string(shareEndingIndex))
}

// KEYGENProtocol type
type KEYGENProtocol struct {
	suite           *Suite
	localHost       *P2PSuite // local host
	KeygenInstances map[keygenID]*keygen.KeygenInstance
	requests        map[string]*P2PBasicMsg // used to access request data from response handlers
}

type BFTKeygenMsg struct {
	P2PBasicMsg
	Protocol string
}

// type AVSSKeygenTransport interface {
// 	// Implementing the Code below will allow KEYGEN to run
// 	// "Client" Actions
// 	BroadcastInitiateKeygen(commitmentMatrixes [][][]common.Point) error
// 	SendKEYGENSend(msg KEYGENSend, nodeIndex big.Int) error
// 	SendKEYGENEcho(msg KEYGENEcho, nodeIndex big.Int) error
// 	SendKEYGENReady(msg KEYGENReady, nodeIndex big.Int) error
// 	BroadcastKEYGENDKGComplete(msg KEYGENDKGComplete) error
// }

func NewKeygenProtocol(suite *Suite, localHost *P2PSuite) *KEYGENProtocol {
	k := &KEYGENProtocol{
		suite:           suite,
		localHost:       localHost,
		KeygenInstances: make(map[keygenID]*keygen.KeygenInstance),
		requests:        make(map[string]*P2PBasicMsg)}
	return k
}

func (kp *KEYGENProtocol) NewKeygen(suite *Suite, shareStartingIndex int, shareEndingIndex int) error {
	logging.Debugf("Keygen started from %v to  %v", shareStartingIndex, shareEndingIndex)

	keygenID := getKeygenID(shareStartingIndex, shareEndingIndex)

	// set up our keygen instance
	nodeIndexList := make([]big.Int, len(suite.EthSuite.NodeList))
	var ownNodeIndex big.Int
	for i, nodeRef := range suite.EthSuite.NodeList {
		nodeIndexList[i] = *nodeRef.Index
		if nodeRef.Address.String() == suite.EthSuite.NodeAddress.String() {
			ownNodeIndex = *nodeRef.Index
		}
	}
	keygenTp := KEYGENTransport{}
	keygenAuth := KEYGENAuth{}
	c := make(chan string)
	instance, err := keygen.NewAVSSKeygen(
		*big.NewInt(int64(shareStartingIndex)),
		shareEndingIndex-shareStartingIndex,
		nodeIndexList,
		suite.Config.Threshold,
		suite.Config.NumMalNodes,
		ownNodeIndex,
		&keygenTp,
		suite.DBSuite.Instance,
		&keygenAuth,
		c,
	)
	if err != nil {
		return err
	}

	// attach listners
	kp.localHost.SetStreamHandler(protocol.ID(keygenID), kp.onP2PKeygenMessage)

	// peg it to the protocol
	kp.KeygenInstances[keygenID] = instance

	return nil
}

// remote peer requests handler
func (p *KEYGENProtocol) onP2PKeygenMessage(s inet.Stream) {

	// get request data
	p2pMsg := &P2PBasicMsg{}
	buf, err := ioutil.ReadAll(s)
	if err != nil {
		s.Reset()
		logging.Error(err.Error())
		return
	}
	s.Close()

	// unmarshal it
	bijson.Unmarshal(buf, p2pMsg)
	if err != nil {
		logging.Error(err.Error())
		return
	}

	valid := p.localHost.authenticateMessage(p2pMsg)

	if !valid {
		logging.Error("Failed to authenticate message")
		return
	}

	// Derive NodeIndex From PK
	// TODO: this should be exported once nodelist becomes more modular
	var nodeIndex big.Int
	pk, err := crypto.UnmarshalPublicKey(p2pMsg.GetNodePubKey())
	if err != nil {
		logging.Error("Failed to derive pk")
		return
	}
	for _, nodeRef := range p.suite.EthSuite.NodeList {
		if nodeRef.PeerID.MatchesPublicKey(pk) {
			nodeIndex = *nodeRef.Index
			break
		}
	}

	switch p2pMsg.GetMsgType() {
	case keygenConsts.Send:
		payload := keygen.KEYGENSend{}
		err = bijson.Unmarshal(p2pMsg.Payload, payload)
		if err != nil {
			logging.Error(err.Error())
			return
		}
		err = p.KeygenInstances[keygenID(string(s.Protocol()))].OnKEYGENSend(payload, nodeIndex)
		if err != nil {
			logging.Error(err.Error())
			return
		}
	case keygenConsts.Echo:
		payload := keygen.KEYGENEcho{}
		err = bijson.Unmarshal(p2pMsg.Payload, payload)
		if err != nil {
			logging.Error(err.Error())
			return
		}
		err = p.KeygenInstances[keygenID(string(s.Protocol()))].OnKEYGENEcho(payload, nodeIndex)
		if err != nil {
			logging.Error(err.Error())
			return
		}
	case keygenConsts.Ready:
		payload := keygen.KEYGENReady{}
		err = bijson.Unmarshal(p2pMsg.Payload, payload)
		if err != nil {
			logging.Error(err.Error())
			return
		}
		err = p.KeygenInstances[keygenID(string(s.Protocol()))].OnKEYGENReady(payload, nodeIndex)
		if err != nil {
			logging.Error(err.Error())
			return
		}
	}

	// generate response message
	// log.Printf("%s: Sending ping response to %s. Message id: %s...", s.Conn().LocalPeer(), s.Conn().RemotePeer(), data.GetId())
	// pingBytes, err := bijson.Marshal(Ping{Message: fmt.Sprintf("Ping response from %s", p.localHost.ID())})
	// if err != nil {
	// 	logging.Error("could not marshal ping")
	// 	return
	// }
	// resp := p.localHost.NewP2PMessage(data.GetId(), false, pingBytes)

	// // sign the data
	// signature, err := p.localHost.signP2PMessage(resp)
	// if err != nil {
	// 	logging.Error("failed to sign response")
	// 	return
	// }

	// // add the signature to the message
	// resp.Sign = signature

	// // send the response
	// err = p.localHost.sendP2PMessage(s.Conn().RemotePeer(), pingResponse, resp)

	// if err == nil {
	// 	logging.Debugf("%s: Ping response to %s sent.", s.Conn().LocalPeer().String(), s.Conn().RemotePeer().String())
	// }
}

func (p *KEYGENProtocol) onBFTMsg(bftMsg BFTKeygenMsg) bool {

	valid := p.localHost.authenticateMessage(&bftMsg)

	if !valid {
		logging.Error("Failed to authenticate message")
		return false
	}

	// Derive NodeIndex From PK
	// TODO: this should be exported once nodelist becomes more modular
	var nodeIndex big.Int
	pk, err := crypto.UnmarshalPublicKey(bftMsg.GetNodePubKey())
	if err != nil {
		logging.Error("Failed to derive pk")
		return false
	}
	for _, nodeRef := range p.suite.EthSuite.NodeList {
		if nodeRef.PeerID.MatchesPublicKey(pk) {
			nodeIndex = *nodeRef.Index
			break
		}
	}

	switch bftMsg.GetMsgType() {
	case keygenConsts.Initiate:
		payload := keygen.KEYGENInitiate{}
		err = bijson.Unmarshal(bftMsg.Payload, payload)
		if err != nil {
			logging.Error(err.Error())
			return false
		}
		err = p.KeygenInstances[keygenID(bftMsg.Protocol)].OnInitiateKeygen(payload, nodeIndex)
		if err != nil {
			logging.Error(err.Error())
			return false
		}
	case keygenConsts.Complete:
		payload := keygen.KEYGENDKGComplete{}
		err = bijson.Unmarshal(bftMsg.Payload, payload)
		if err != nil {
			logging.Error(err.Error())
			return false
		}
		err = p.KeygenInstances[keygenID(bftMsg.Protocol)].OnKEYGENDKGComplete(payload, nodeIndex)
		if err != nil {
			logging.Error(err.Error())
			return false
		}
	}

	return true
}

type KEYGENTransport struct {
	Protocol  *KEYGENProtocol
	ProtoName protocol.ID
}

func (kt *KEYGENTransport) BroadcastInitiateKeygen(msg keygen.KEYGENInitiate) error {
	return nil
}
func (kt *KEYGENTransport) SendKEYGENSend(msg keygen.KEYGENSend, nodeIndex big.Int) error {
	// Derive ID From Index
	// TODO: this should be exported once nodelist becomes more modular
	var nodeId peer.ID
	for _, nodeRef := range kt.Protocol.suite.EthSuite.NodeList {
		if nodeRef.Index.Cmp(&nodeIndex) == 0 {
			nodeId = nodeRef.PeerID
			break
		}
	}

	plBytes, err := bijson.Marshal(msg)
	if err != nil {
		return errors.New("Could not marshal: " + err.Error())
	}

	p2pMsg := kt.Protocol.localHost.NewP2PMessage(HashToString(plBytes), false, plBytes)

	// sign the data
	signature, err := kt.Protocol.localHost.signP2PMessage(p2pMsg)
	if err != nil {
		return errors.New("failed to sign p2pMsgonse" + err.Error())
	}
	// add the signature to the message
	p2pMsg.Sign = signature

	// send the p2pMsgonse
	err = kt.Protocol.localHost.sendP2PMessage(nodeId, kt.ProtoName, p2pMsg)
	if err != nil {
		return errors.New("failed to send SendKEYGENSend " + err.Error())
	}
	return nil
}
func (kt *KEYGENTransport) SendKEYGENEcho(msg keygen.KEYGENEcho, nodeIndex big.Int) error {
	return nil
}
func (kt *KEYGENTransport) SendKEYGENReady(msg keygen.KEYGENReady, nodeIndex big.Int) error {
	return nil
}
func (kt *KEYGENTransport) BroadcastKEYGENDKGComplete(msg keygen.KEYGENDKGComplete) error {
	return nil
}

type KEYGENAuth struct {
}

func (ka *KEYGENAuth) Sign(msg string) ([]byte, error) {
	return nil, nil
}
func (ka *KEYGENAuth) Verify(text string, nodeIndex big.Int, signature []byte) bool {
	return false
}
