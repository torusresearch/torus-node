package dkgnode

import (
	// "fmt"
	"fmt"
	"io/ioutil"
	"strconv"
	"strings"
	"time"

	"github.com/torusresearch/torus-public/idmutex"

	// "log"
	"crypto/ecdsa"
	"errors"
	"math/big"

	// uuid "github.com/google/uuid"
	crypto "github.com/libp2p/go-libp2p-crypto"
	inet "github.com/libp2p/go-libp2p-net"
	peer "github.com/libp2p/go-libp2p-peer"
	protocol "github.com/libp2p/go-libp2p-protocol"
	tmcmn "github.com/tendermint/tendermint/libs/common"
	"github.com/torusresearch/bijson"

	// "github.com/torusresearch/torus-public/common"
	"github.com/torusresearch/torus-public/keygen"
	"github.com/torusresearch/torus-public/logging"
	"github.com/torusresearch/torus-public/telemetry"
)

type KeygenSuite struct {
	LocalStatus *LocalStatus
}

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

type KEYGENTransport struct {
	Protocol  *KEYGENProtocol
	ProtoName protocol.ID
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
//TODO : Change startingIndex and endingIndex in node to big.Int
func getKeygenID(shareStartingIndex int, shareEndingIndex int) keygenID {
	return keygenID(keygenConsts.RequestPrefix + strconv.FormatInt(int64(shareStartingIndex), 16) + "|" + strconv.FormatInt(int64(shareEndingIndex), 16))
}
func getStartEndIndexesFromKeygenID(keygenID keygenID) (int, int, error) {
	split := strings.Split(string(keygenID)[18:], "|")
	startIndex, err := strconv.ParseInt(split[0], 16, 64)
	if err != nil {
		return 0, 0, err
	}
	endIndex, err := strconv.ParseInt(split[1], 16, 64)
	if err != nil {
		return 0, 0, err
	}
	return int(startIndex), int(endIndex), nil
}

// KEYGENProtocol type
type KEYGENProtocol struct {
	suite           *Suite
	localHost       *P2PSuite // local host
	KeygenInstances map[keygenID]*keygen.KeygenInstance
	requests        map[string]*P2PBasicMsg // used to access request data from response handlers
	counters        map[string]*telemetry.Counter
	MainChannel     chan string
	idmutex.Mutex
}

type BFTKeygenMsg struct {
	P2PBasicMsg
	Protocol string
}

func NewKeygenProtocol(suite *Suite, localHost *P2PSuite) *KEYGENProtocol {
	// for logging statistics
	counters := make(map[string]*telemetry.Counter)
	counters["num_shares_verified"] = telemetry.NewCounter("num_shares_verified", "how many times shares were verified")
	counters["num_shares_invalid"] = telemetry.NewCounter("num_shares_invalid", "how many times shares could not be verified")
	telemetry.Register(counters["num_shares_verified"])
	telemetry.Register(counters["num_shares_invalid"])
	mainChan := make(chan string)
	k := &KEYGENProtocol{
		suite:           suite,
		localHost:       localHost,
		KeygenInstances: make(map[keygenID]*keygen.KeygenInstance),
		requests:        make(map[string]*P2PBasicMsg),
		counters:        counters,
		MainChannel:     mainChan,
	}
	// initiate channel aggregator here
	go k.handleMainChannel()
	return k
}

// We react to communication from KEYGEN Instances here
func (kp *KEYGENProtocol) handleMainChannel() {
	for {
		select {
		case msg := <-kp.MainChannel:
			logging.Infof("KEYGEN Finished Msg: %v", msg)
			// For now we just increase the telementry number by X amount
			splits := strings.Split(msg, "|")
			if splits[0] == keygen.SIKeygenCompleted {
				kp.suite.LocalStatus.Event(LocalStatusConstants.Events.KeygenComplete)
				for i := 0; i < kp.suite.Config.KeysPerEpoch; i++ {
					kp.counters["num_shares_verified"].Inc()
				}
				// Clean up
				startIndex, err := strconv.Atoi(splits[1])
				if err != nil {
					logging.Errorf("could not parse keygen complete : %s", msg)
				}
				numGen, err := strconv.Atoi(splits[2])
				if err != nil {
					logging.Errorf("could not parse keygen complete : %s", msg)
				}
				endIndex := startIndex + numGen
				kp.CleanUpKeygenInstance(getKeygenID(startIndex, endIndex))
			}
		}
	}
}

// Cleans up keygen instance, usuallly run after completion of keygen
// Concurrent safe
func (kp *KEYGENProtocol) CleanUpKeygenInstance(kID keygenID) error {
	kp.Lock()
	defer kp.Unlock()
	_, ok := kp.KeygenInstances[kID]
	if !ok {
		return fmt.Errorf("No KeygenInstance to delete for keygenID: %s", kID)
	}
	delete(kp.KeygenInstances, kID)
	return nil
}

// NewKeygen instanciates a new keygenInstance that is assigned to keygenProto
// Exported to be concurrent safe
func (kp *KEYGENProtocol) NewKeygen(suite *Suite, shareStartingIndex int, shareEndingIndex int) (keygenID, error) {
	kp.Lock()
	defer kp.Unlock()
	return kp.newKeygen(suite, shareStartingIndex, shareEndingIndex)
}

// NewKeygenSafe instanciates a new keygenInstance that is assigned to keygenProto
// ensuring that any keygen that has already been initiated is not replaced
func (kp *KEYGENProtocol) NewKeygenSafe(suite *Suite, id keygenID) error {
	kp.Lock()
	defer kp.Unlock()
	_, ok := kp.KeygenInstances[id]
	if !ok {
		start, end, err := getStartEndIndexesFromKeygenID(id)
		if err != nil {
			logging.Error(err.Error())
			return err
		}
		id, err = kp.newKeygen(kp.suite, start, end)
		if err != nil {
			logging.Error(err.Error())
			return err
		}
	}
	return nil
}

// NewKeygen instanciates a new keygenInstance that is assigned to keygenProto
func (kp *KEYGENProtocol) newKeygen(suite *Suite, shareStartingIndex int, shareEndingIndex int) (keygenID, error) {
	logging.Debugf("NewKeygen from %v to  %v", shareStartingIndex, shareEndingIndex)

	keygenID := getKeygenID(shareStartingIndex, shareEndingIndex)

	// set up our keygen instance
	nodeIndexList := make([]big.Int, len(suite.EthSuite.EpochNodeRegister[suite.EthSuite.CurrentEpoch].NodeList))
	var ownNodeIndex big.Int
	for i, nodeRef := range suite.EthSuite.EpochNodeRegister[suite.EthSuite.CurrentEpoch].NodeList {
		nodeIndexList[i] = *nodeRef.Index
		if nodeRef.Address.String() == suite.EthSuite.NodeAddress.String() {
			ownNodeIndex = *nodeRef.Index
		}
	}

	logging.Debugf("With k: %v, t: %v, nodeIndexes: %v", suite.Config.Threshold, suite.Config.NumMalNodes, nodeIndexList)
	logging.Debugf("and own index: %v", ownNodeIndex)

	keygenTp := KEYGENTransport{
		Protocol:  kp,
		ProtoName: protocol.ID(keygenID),
	}
	// c := make(chan string)
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
		kp.MainChannel,
	)
	if err != nil {
		return "", err
	}

	// attach listners
	kp.localHost.SetStreamHandler(protocol.ID(keygenID), kp.onP2PKeygenMessage)

	// peg it to the protocol
	kp.KeygenInstances[keygenID] = instance

	return keygenID, nil
}

func (kp *KEYGENProtocol) InitiateKeygen(suite *Suite, shareStartingIndex int, shareEndingIndex int) error {
	keygenID := getKeygenID(shareStartingIndex, shareEndingIndex)

	// Look if keygen instance exists
	ki, err := kp.GetKeygenInstance(keygenID)
	if err != nil {
		logging.Fatal("Keygen instance should have already been created")
		return err
	}

	logging.Debugf("Keygen Initaited from %v to  %v", shareStartingIndex, shareEndingIndex)
	//initiate Keygen
	err = ki.InitiateKeygen()
	if err != nil {
		logging.Errorf("error initiating keygen: ", err)
		return err
	}

	return nil
}

// NewKeygenSafe instanciates a new keygenInstance that is assigned to keygenProto
// ensuring that any keygen that has already been initiated is not replaced
func (kp *KEYGENProtocol) GetKeygenInstance(id keygenID) (*keygen.KeygenInstance, error) {
	kp.Lock()
	defer kp.Unlock()
	inst, ok := kp.KeygenInstances[id]
	if !ok {
		return nil, errors.New("Keygen instance " + string(id) + "does not exist")
	}
	return inst, nil
}

// remote peer requests handler
func (kp *KEYGENProtocol) onP2PKeygenMessage(s inet.Stream) {
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

	valid := kp.localHost.authenticateMessage(p2pMsg)

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
	for _, nodeRef := range kp.suite.EthSuite.EpochNodeRegister[kp.suite.EthSuite.CurrentEpoch].NodeList {
		if nodeRef.PeerID.MatchesPublicKey(pk) {
			nodeIndex = *nodeRef.Index
			break
		}
	}

	switch p2pMsg.GetMsgType() {
	case keygenConsts.Send:

		payload := &keygen.KEYGENSend{}
		err = bijson.Unmarshal(p2pMsg.Payload, payload)
		if err != nil {
			logging.Error(err.Error())
			return
		}
		logging.Debugf("got p2p send: %v", payload)
		ki := kp.KeygenInstances[keygenID(string(s.Protocol()))]
		err = ki.OnKEYGENSend(*payload, nodeIndex)
		if err != nil {
			logging.Error(err.Error())
			return
		}

	case keygenConsts.Echo:
		payload := &keygen.KEYGENEcho{}
		err = bijson.Unmarshal(p2pMsg.Payload, payload)
		if err != nil {
			logging.Error(err.Error())
			return
		}
		logging.Debugf("got p2p echo: %v", payload)
		err = kp.KeygenInstances[keygenID(string(s.Protocol()))].OnKEYGENEcho(*payload, nodeIndex)
		if err != nil {
			logging.Error(err.Error())
			return
		}
	case keygenConsts.Ready:
		payload := &keygen.KEYGENReady{}
		err = bijson.Unmarshal(p2pMsg.Payload, payload)
		if err != nil {
			logging.Error(err.Error())
			return
		}
		logging.Debugf("got p2p ready: %v", payload)
		err = kp.KeygenInstances[keygenID(string(s.Protocol()))].OnKEYGENReady(*payload, nodeIndex)
		if err != nil {
			logging.Error(err.Error())
			return
		}
	}
}

func (kp *KEYGENProtocol) onBFTMsg(bftMsg BFTKeygenMsg) (bool, []tmcmn.KVPair) {
	logging.Debugf("BFT MSG ID: ", bftMsg.Id)
	valid := kp.localHost.authenticateMessage(&bftMsg)
	var tags []tmcmn.KVPair

	if !valid {
		logging.Error("Failed to authenticate message")
		return false, nil
	}

	// Derive NodeIndex From PK
	// TODO: this should be exported once nodelist becomes more modular
	var nodeIndex big.Int
	pk, err := crypto.UnmarshalPublicKey(bftMsg.GetNodePubKey())
	if err != nil {
		logging.Error("Failed to derive pk")
		return false, nil
	}
	for _, nodeRef := range kp.suite.EthSuite.EpochNodeRegister[kp.suite.EthSuite.CurrentEpoch].NodeList {
		if nodeRef.PeerID.MatchesPublicKey(pk) {
			nodeIndex = *nodeRef.Index
			break
		}
	}

	switch bftMsg.GetMsgType() {
	case keygenConsts.Initiate:
		payload := &keygen.KEYGENInitiate{}
		err = bijson.Unmarshal(bftMsg.Payload, payload)
		if err != nil {
			logging.Error(err.Error())
			return false, nil
		}
		err := kp.NewKeygenSafe(kp.suite, keygenID(bftMsg.Protocol))
		if err != nil {
			logging.Fatal(err.Error())
			return false, nil
		}
		ki, err := kp.GetKeygenInstance(keygenID(bftMsg.Protocol))
		if err != nil {
			logging.Fatal(err.Error())
			return false, nil
		}

		err = ki.OnInitiateKeygen(*payload, nodeIndex)
		if err != nil {
			logging.Error(err.Error())
			return false, nil
		}

	case keygenConsts.Complete:
		payload := &keygen.KEYGENDKGComplete{}
		err = bijson.Unmarshal(bftMsg.Payload, payload)
		if err != nil {
			logging.Error(err.Error())
			return false, nil
		}
		ki, err := kp.GetKeygenInstance(keygenID(bftMsg.Protocol))
		if err != nil {
			// TODO: Discuss how we are going to handle BFT Transactions that do depend on offchain state?
			// This example of KEYGENDKGComplete (KEYGENInitiate is fine)
			logging.Error(err.Error())
			return false, nil
		}
		logging.Debugf("DKGComplete received for node %s : %v", nodeIndex.Text(16), payload.NodeSet)

		err = ki.OnKEYGENDKGComplete(*payload, nodeIndex)
		if err != nil {
			logging.Debugf("DKGComplete error")
			logging.Error(err.Error())
			return false, nil
		}
		end := big.NewInt(int64(0))
		end.Add(&ki.StartIndex, big.NewInt(int64(ki.NumOfKeys)))
		tags = []tmcmn.KVPair{
			{Key: []byte("start_key_index"), Value: []byte(ki.StartIndex.Text(16))},
			{Key: []byte("end_key_index"), Value: []byte(end.Text(16))},
		}
	}

	return true, tags
}

func (kp *KEYGENProtocol) Sign(msg string) ([]byte, error) {

	sig := ECDSASign([]byte(msg), kp.suite.EthSuite.NodePrivateKey)
	return sig.Raw, nil
}
func (kp *KEYGENProtocol) Verify(text string, nodeIndex big.Int, signature []byte) bool {
	// Derive ID From Index
	// TODO: this should be exported once nodelist becomes more modular
	var nodePK ecdsa.PublicKey
	for _, nodeRef := range kp.suite.EthSuite.EpochNodeRegister[kp.suite.EthSuite.CurrentEpoch].NodeList {
		if nodeRef.Index.Cmp(&nodeIndex) == 0 {
			nodePK = *nodeRef.PublicKey
			break
		}
	}

	return ECDSAVerifyFromRaw(text, nodePK, signature)
}

func (kt *KEYGENTransport) SendKEYGENSend(msg keygen.KEYGENSend, nodeIndex big.Int) error {
	logging.Debugf("attempting sent KEYGENSend to index: %s ", nodeIndex.Text(16))
	// cater to if sending to self
	if nodeIndex.Cmp(kt.Protocol.suite.EthSuite.NodeIndex) == 0 {
		go func() {
			err := kt.Protocol.KeygenInstances[keygenID(kt.ProtoName)].OnKEYGENSend(msg, nodeIndex)
			if err != nil {
				logging.Error("Could not send to self: " + err.Error())
			}
			logging.Debugf("success sent KEYGENSend to index: %s ", nodeIndex.Text(16))
		}()
		return nil
	}
	plBytes, err := bijson.Marshal(msg)
	if err != nil {
		return errors.New("Could not marshal: " + err.Error())
	}
	err = kt.prepAndSendKeygenMsg(plBytes, keygenConsts.Send, nodeIndex)
	if err != nil {
		return errors.New("Could not send KeygenP2PMsg: " + err.Error())
	}
	logging.Debugf("success sent KEYGENSend to index: %s ", nodeIndex.Text(16))
	return nil
}

func (kt *KEYGENTransport) SendKEYGENEcho(msg keygen.KEYGENEcho, nodeIndex big.Int) error {
	// cater to if sending to self
	if nodeIndex.Cmp(kt.Protocol.suite.EthSuite.NodeIndex) == 0 {
		go func() {
			err := kt.Protocol.KeygenInstances[keygenID(kt.ProtoName)].OnKEYGENEcho(msg, nodeIndex)
			if err != nil {
				logging.Error("Could not send to self: " + err.Error())
			}
			logging.Debugf("success sent KEYGENEcho to index: %s ", nodeIndex.Text(16))
		}()
		return nil
	}
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
	// cater to if sending to self
	if nodeIndex.Cmp(kt.Protocol.suite.EthSuite.NodeIndex) == 0 {
		go func() {
			err := kt.Protocol.KeygenInstances[keygenID(kt.ProtoName)].OnKEYGENReady(msg, nodeIndex)
			if err != nil {
				logging.Error("Could not send to self: " + err.Error())
			}
			logging.Debugf("success sent KEYGENReady to index: %s ", nodeIndex.Text(16))
		}()
		return nil
	}
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
	tempP2P := kt.Protocol.localHost.NewP2PMessage(HashToString(plBytes), false, plBytes, keygenConsts.Initiate)
	bftMsg := BFTKeygenMsg{
		P2PBasicMsg: *tempP2P,
		Protocol:    string(kt.ProtoName),
	}
	// sign the data
	signature, err := kt.Protocol.localHost.signP2PMessage(&bftMsg)
	if err != nil {
		return errors.New("failed to sign bftMsg" + err.Error())
	}
	// add the signature to the message
	bftMsg.Sign = signature

	wrap := DefaultBFTTxWrapper{bftMsg}
	_, err = kt.Protocol.suite.BftSuite.BftRPC.Broadcast(wrap)
	if err != nil {
		logging.Debug(err.Error())
		return err
	}
	return nil
}

func (kt *KEYGENTransport) BroadcastKEYGENDKGComplete(msg keygen.KEYGENDKGComplete) error {
	plBytes, err := bijson.Marshal(msg)
	if err != nil {
		return errors.New("Could not marshal: " + err.Error())
	}
	tempP2P := kt.Protocol.localHost.NewP2PMessage(HashToString(plBytes), false, plBytes, keygenConsts.Complete)
	bftMsg := BFTKeygenMsg{
		P2PBasicMsg: *tempP2P,
		Protocol:    string(kt.ProtoName),
	}
	// sign the data
	signature, err := kt.Protocol.localHost.signP2PMessage(&bftMsg)
	if err != nil {
		return errors.New("failed to sign bftMsg" + err.Error())
	}
	// add the signature to the message
	bftMsg.Sign = signature
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
	for _, nodeRef := range kt.Protocol.suite.EthSuite.EpochNodeRegister[kt.Protocol.suite.EthSuite.CurrentEpoch].NodeList {
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

	// send the p2pMsgonse with a retry if it fails
	// TODO: Implement backoff
	for {
		err = kt.Protocol.localHost.sendP2PMessage(nodeId, kt.ProtoName, p2pMsg)
		if err == nil {
			break
		}
		time.Sleep(500 * time.Millisecond)
		logging.Debugf("Retrying after failed to send SendKEYGENSend " + err.Error())
	}
	return nil
}
