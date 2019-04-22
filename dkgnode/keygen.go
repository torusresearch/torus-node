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
	// "crypto/ecdsa"
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
		kp,
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

func (ka *KEYGENProtocol) Sign(msg string) ([]byte, error) {

	sig := ECDSASign([]byte(msg), ka.suite.EthSuite.NodePrivateKey)
	// bytes32(signature[:32]),
	// bytes32(signature[32:64]),
	// uint8(int(signature[64])) + 27, // Yes add 27, weird Ethereum quirk
	return sig.Raw, nil
}
func (ka *KEYGENProtocol) Verify(text string, nodeIndex big.Int, signature []byte) bool {
	// Derive ID From Index
	// TODO: this should be exported once nodelist becomes more modular
	// var nodePK ecdsa.PublicKey
	// for _, nodeRef := range ka.suite.EthSuite.NodeList {
	// 	if nodeRef.Index.Cmp(&nodeIndex) == 0 {
	// 		nodePK = *nodeRef.PublicKey
	// 		break
	// 	}
	// }

	// ecSig := ECDSASignature{
	// 	signature,

	// 	bytes32(signature[:32]),
	// 	bytes32(signature[32:64]),
	// 	uint8(int(signature[64])), +27, // Yes add 27, weird Ethereum quirk
	// }

	// return ECDSAVerify(nodePK, ecSig)
	return false
}

type KEYGENTransport struct {
	Protocol  *KEYGENProtocol
	ProtoName protocol.ID
}

func (kt *KEYGENTransport) SendKEYGENSend(msg keygen.KEYGENSend, nodeIndex big.Int) error {
	plBytes, err := bijson.Marshal(msg)
	if err != nil {
		return errors.New("Could not marshal: " + err.Error())
	}
	err = kt.prepAndSendKeygenMsg(plBytes, keygenConsts.Send, nodeIndex)
	if err != nil {
		return errors.New("Could not send KeygenP2PMsg: " + err.Error())
	}
	return nil
}

func (kt *KEYGENTransport) SendKEYGENEcho(msg keygen.KEYGENEcho, nodeIndex big.Int) error {
	plBytes, err := bijson.Marshal(msg)
	if err != nil {
		return errors.New("Could not marshal: " + err.Error())
	}
	err = kt.prepAndSendKeygenMsg(plBytes, keygenConsts.Echo, nodeIndex)
	if err != nil {
		return errors.New("Could not send KeygenP2PMsg: " + err.Error())
	}
	return nil
}

func (kt *KEYGENTransport) SendKEYGENReady(msg keygen.KEYGENReady, nodeIndex big.Int) error {
	plBytes, err := bijson.Marshal(msg)
	if err != nil {
		return errors.New("Could not marshal: " + err.Error())
	}
	err = kt.prepAndSendKeygenMsg(plBytes, keygenConsts.Ready, nodeIndex)
	if err != nil {
		return errors.New("Could not send KeygenP2PMsg: " + err.Error())
	}
	return nil
}

func (kt *KEYGENTransport) BroadcastInitiateKeygen(msg keygen.KEYGENInitiate) error {
	plBytes, err := bijson.Marshal(msg)
	if err != nil {
		return errors.New("Could not marshal: " + err.Error())
	}
	bftMsg := BFTKeygenMsg{
		P2PBasicMsg: *kt.Protocol.localHost.NewP2PMessage(HashToString(plBytes), false, plBytes, keygenConsts.Initiate),
		Protocol:    string(kt.ProtoName),
	}
	wrap := DefaultBFTTxWrapper{bftMsg}
	_, err = kt.Protocol.suite.BftSuite.BftRPC.Broadcast(wrap)
	if err != nil {
		return err
	}
	return nil
}

func (kt *KEYGENTransport) BroadcastKEYGENDKGComplete(msg keygen.KEYGENDKGComplete) error {
	plBytes, err := bijson.Marshal(msg)
	if err != nil {
		return errors.New("Could not marshal: " + err.Error())
	}
	bftMsg := BFTKeygenMsg{
		P2PBasicMsg: *kt.Protocol.localHost.NewP2PMessage(HashToString(plBytes), false, plBytes, keygenConsts.Ready),
		Protocol:    string(kt.ProtoName),
	}
	wrap := DefaultBFTTxWrapper{bftMsg}
	_, err = kt.Protocol.suite.BftSuite.BftRPC.Broadcast(wrap)
	if err != nil {
		return err
	}
	return nil
}

func (kt *KEYGENTransport) prepAndSendKeygenMsg(pl []byte, msgType string, nodeIndex big.Int) error {
	// Derive ID From Index
	// TODO: this should be exported once nodelist becomes more modular
	var nodeId peer.ID
	for _, nodeRef := range kt.Protocol.suite.EthSuite.NodeList {
		if nodeRef.Index.Cmp(&nodeIndex) == 0 {
			nodeId = nodeRef.PeerID
			break
		}
	}

	p2pMsg := kt.Protocol.localHost.NewP2PMessage(HashToString(pl), false, pl, msgType)

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
