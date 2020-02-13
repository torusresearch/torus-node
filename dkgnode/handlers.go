package dkgnode

import (
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"time"

	ethCommon "github.com/ethereum/go-ethereum/common"
	pcmn "github.com/torusresearch/torus-node/common"
	"github.com/torusresearch/torus-node/pvss"

	logging "github.com/sirupsen/logrus"
	"github.com/torusresearch/bijson"
	"github.com/torusresearch/jsonrpc"
	"github.com/torusresearch/torus-common/common"
	torusCrypto "github.com/torusresearch/torus-common/crypto"
	"github.com/torusresearch/torus-common/secp256k1"
	"github.com/torusresearch/torus-node/eventbus"
)

// Method Constants
const (
	PingMethod                  = "Ping"
	ConnectionDetailsMethod     = "ConnectionDetails"
	ShareRequestMethod          = "ShareRequest"
	KeyAssignMethod             = "KeyAssign"
	CommitmentRequestMethod     = "CommitmentRequest"
	VerifierLookupRequestMethod = "VerifierLookupRequest"
	KeyLookupRequestMethod      = "KeyLookupRequest"
	UpdatePublicKeyMethod       = "updatePubKey"
	UpdateShareMethod           = "updateShare"
	UpdateCommitmentMethod      = "updateCommitment"
)

// Debug Handelers
const ShareCountMethod = "ShareCount"

type (
	PingHandler struct {
		eventBus eventbus.Bus
	}
	PingParams struct {
		Message string `json:"message"`
	}
	PingResult struct {
		Message string `json:"message"`
	}
	ConnectionDetailsHandler struct {
		eventBus eventbus.Bus
	}
	ConnectionDetailsParams struct {
		PubKeyX                  big.Int                  `json:"pubkeyx"`
		PubKeyY                  big.Int                  `json:"pubkeyy"`
		ConnectionDetailsMessage ConnectionDetailsMessage `json:"connection_details_message"`
		Signature                []byte                   `json:"signature"`
	}
	ConnectionDetailsMessage struct {
		Timestamp   string            `json:"timestamp"`
		Message     string            `json:"message"`
		NodeAddress ethCommon.Address `json:"node_address"`
	}
	ConnectionDetailsResult struct {
		TMP2PConnection string `json:"tm_p2p_connection"`
		P2PConnection   string `json:"p2p_connection"`
	}
	ShareLog struct {
		Timestamp          time.Time
		LogNumber          int
		ShareIndex         int
		UnsigncryptedShare []byte
	}
	ShareRequestHandler struct {
		eventBus eventbus.Bus
		TimeNow  func() time.Time
	}

	ValidatedNodeSignature struct {
		NodeSignature
		NodeIndex big.Int
	}

	NodeSignature struct {
		Signature   string `json:"signature"`
		Data        string `json:"data"`
		NodePubKeyX string `json:"nodepubx"`
		NodePubKeyY string `json:"nodepuby"`
	}
	ShareRequestParams struct {
		Item []bijson.RawMessage `json:"item"`
	}
	ShareRequestItem struct {
		IDToken            string          `json:"idtoken"`
		NodeSignatures     []NodeSignature `json:"nodesignatures"`
		VerifierIdentifier string          `json:"verifieridentifier"`
	}

	ShareRequestResult struct {
		Keys []interface{} `json:"keys"`
	}

	CommitmentRequestHandler struct {
		eventBus eventbus.Bus
		TimeNow  func() time.Time
	}
	CommitmentRequestParams struct {
		MessagePrefix      string `json:"messageprefix"`
		TokenCommitment    string `json:"tokencommitment"`
		TempPubX           string `json:"temppubx"`
		TempPubY           string `json:"temppuby"`
		VerifierIdentifier string `json:"verifieridentifier"`
	}

	CommitmentRequestResultData struct {
		MessagePrefix      string `json:"messageprefix"`
		TokenCommitment    string `json:"tokencommitment"`
		TempPubX           string `json:"temppubx"`
		TempPubY           string `json:"temppuby"`
		VerifierIdentifier string `json:"verifieridentifier"`
		TimeSigned         string `json:"timesigned"`
	}
	CommitmentRequestResult struct {
		Signature string `json:"signature"`
		Data      string `json:"data"`
		NodePubX  string `json:"nodepubx"`
		NodePubY  string `json:"nodepuby"`
	}
	KeyAssignHandler struct {
		eventBus eventbus.Bus
	}
	KeyAssignParams struct {
		Verifier   string `json:"verifier"`
		VerifierID string `json:"verifier_id"`
	}
	KeyAssignItem struct {
		KeyIndex string  `json:"key_index"`
		PubKeyX  big.Int `json:"pub_key_X"`
		PubKeyY  big.Int `json:"pub_key_Y"`
		Address  string  `json:"address"`
	}
	KeyAssignResult struct {
		Keys []KeyAssignItem `json:"keys"`
	}

	VerifierLookupHandler struct {
		eventBus eventbus.Bus
	}
	VerifierLookupParams struct {
		Verifier   string `json:"verifier"`
		VerifierID string `json:"verifier_id"`
	}
	VerifierLookupItem struct {
		KeyIndex string  `json:"key_index"`
		PubKeyX  big.Int `json:"pub_key_X"`
		PubKeyY  big.Int `json:"pub_key_Y"`
		Address  string  `json:"address"`
	}
	VerifierLookupResult struct {
		Keys []VerifierLookupItem `json:"keys"`
	}

	KeyLookupHandler struct {
		eventBus eventbus.Bus
	}
	KeyLookupParams struct {
		PubKeyX big.Int `json:"pub_key_X"`
		PubKeyY big.Int `json:"pub_key_Y"`
	}
	KeyLookupResult struct {
		KeyAssignmentPublic
	}

	UpdatePublicKeyHandler struct {
		eventBus eventbus.Bus
	}

	UpdateShareHandler struct {
		eventBus eventbus.Bus
	}

	UpdateCommitmentHandler struct {
		eventBus eventbus.Bus
	}

	DealerResult struct {
		Code    int
		Message string
	}
)

func (c *ConnectionDetailsMessage) String() string {
	return strings.Join([]string{c.Timestamp, c.Message, c.NodeAddress.String()}, pcmn.Delimiter1)
}

func (c *ConnectionDetailsMessage) Validate(pubKeyX, pubKeyY big.Int, sig []byte) (bool, error) {
	message := c.Message
	if message != "ConnectionDetails" {
		logging.WithField("cMessage", c.Message).Error("message not ConnectionDetails")
		return false, errors.New("message is not ConnectionDetails")
	}
	timeSigned := c.Timestamp
	unixTime, err := strconv.ParseInt(timeSigned, 10, 64)
	if err != nil {
		logging.WithError(err).Error("could not parse time signed ")
		return false, err
	}
	if time.Unix(unixTime, 0).Add(10 * time.Minute).Before(time.Now()) {
		logging.WithError(err).Error("signature expired")
		return false, err
	}
	return pvss.ECDSAVerify(c.String(), &common.Point{X: pubKeyX, Y: pubKeyY}, sig), nil
}

func (nodeSig *NodeSignature) NodeValidation(eventBus eventbus.Bus) (*NodeReference, error) {
	var node *NodeReference
	currentEpoch := NewServiceLibrary(eventBus, "node_validation").EthereumMethods().GetCurrentEpoch()
	nodeList := NewServiceLibrary(eventBus, "node_validation").EthereumMethods().AwaitCompleteNodeList(currentEpoch)
	for i, currNode := range nodeList {
		logging.WithFields(logging.Fields{
			"currNode": stringify(currNode),
			"x":        currNode.PublicKey.X.Text(16),
			"y":        currNode.PublicKey.Y.Text(16),
			"nodeSig":  stringify(nodeSig),
		}).Debug()
		if currNode.PublicKey.X.Text(16) == nodeSig.NodePubKeyX &&
			currNode.PublicKey.Y.Text(16) == nodeSig.NodePubKeyY {
			node = &nodeList[i]
		}
	}
	if node == nil {
		return nil, fmt.Errorf("Node not found for nodeSig: %v", *nodeSig)
	}
	logging.WithField("node", stringify(node)).Debug("selected node")
	recSig := torusCrypto.HexToSig(nodeSig.Signature)
	var sig32 [32]byte
	copy(sig32[:], secp256k1.Keccak256([]byte(nodeSig.Data))[:32])
	recoveredSig := torusCrypto.Signature{
		Raw:  recSig.Raw,
		Hash: sig32,
		R:    recSig.R,
		S:    recSig.S,
		V:    recSig.V - 27,
	}
	valid := torusCrypto.IsValidSignature(*node.PublicKey, recoveredSig)
	if !valid {
		return nil, fmt.Errorf("Could not validate ecdsa signature %v", stringify(recoveredSig))
	}

	return node, nil
}

func (p *CommitmentRequestParams) ToString() string {
	return strings.Join([]string{
		p.MessagePrefix,
		p.TokenCommitment,
		p.TempPubX,
		p.TempPubY,
		p.VerifierIdentifier,
	}, pcmn.Delimiter1)
}

func (c *CommitmentRequestResultData) ToString() string {
	return strings.Join([]string{
		c.MessagePrefix,
		c.TokenCommitment,
		c.TempPubX,
		c.TempPubY,
		c.VerifierIdentifier,
		c.TimeSigned,
	}, pcmn.Delimiter1)
}

func (c *CommitmentRequestResultData) FromString(data string) (bool, error) {
	dataArray := strings.Split(data, pcmn.Delimiter1)

	if len(dataArray) < 6 {
		return false, errors.New("Could not parse commitmentrequestresultdata")
	}
	c.MessagePrefix = dataArray[0]
	c.TokenCommitment = dataArray[1]
	c.TempPubX = dataArray[2]
	c.TempPubY = dataArray[3]
	c.VerifierIdentifier = dataArray[4]
	c.TimeSigned = dataArray[5]
	return true, nil
}

func setUpJRPCHandler(eventBus eventbus.Bus) (*jsonrpc.MethodRepository, error) {
	mr := jsonrpc.NewMethodRepository()

	if err := mr.RegisterMethod(PingMethod, PingHandler{eventBus}, PingParams{}, PingResult{}); err != nil {
		return nil, err
	}
	if err := mr.RegisterMethod(ConnectionDetailsMethod, ConnectionDetailsHandler{eventBus}, ConnectionDetailsParams{}, ConnectionDetailsResult{}); err != nil {
		return nil, err
	}
	if err := mr.RegisterMethod(ShareRequestMethod, ShareRequestHandler{eventBus, time.Now}, ShareRequestParams{}, ShareRequestResult{}); err != nil {
		return nil, err
	}
	if err := mr.RegisterMethod(KeyAssignMethod, KeyAssignHandler{eventBus}, KeyAssignParams{}, KeyAssignResult{}); err != nil {
		return nil, err
	}
	if err := mr.RegisterMethod(CommitmentRequestMethod, CommitmentRequestHandler{eventBus, time.Now}, CommitmentRequestParams{}, CommitmentRequestResult{}); err != nil {
		return nil, err
	}
	if err := mr.RegisterMethod(VerifierLookupRequestMethod, VerifierLookupHandler{eventBus}, VerifierLookupParams{}, VerifierLookupResult{}); err != nil {
		return nil, err
	}
	if err := mr.RegisterMethod(KeyLookupRequestMethod, KeyLookupHandler{eventBus}, KeyLookupParams{}, KeyLookupResult{}); err != nil {
		return nil, err
	}
	// if err := mr.RegisterMethod(UpdatePublicKeyMethod, UpdatePublicKeyHandler{eventBus}, dealer.MsgUpdatePublicKey{}, DealerResult{}); err != nil {
	// 	return nil, err
	// }
	// if err := mr.RegisterMethod(UpdateShareMethod, UpdateShareHandler{eventBus}, dealer.MsgUpdateShare{}, DealerResult{}); err != nil {
	// 	return nil, err
	// }
	// if err := mr.RegisterMethod(UpdateCommitmentMethod, UpdateCommitmentHandler{eventBus}, dealer.MsgUpdateCommitment{}, DealerResult{}); err != nil {
	// 	return nil, err
	// }
	return mr, nil
}

func setUpDebugHandler(eventBus eventbus.Bus) (*jsonrpc.MethodRepository, error) {
	mr := jsonrpc.NewMethodRepository()
	if err := mr.RegisterMethod(ShareCountMethod, ShareCountHandler{eventBus}, ShareCountParams{}, ShareCountResult{}); err != nil {
		return nil, err
	}
	return mr, nil
}

// GETHealthz always responds with 200 and can be used for basic readiness checks
func GETHealthz(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(200)
}

func GetBftStatus(w http.ResponseWriter, r *http.Request) {
	status := serverServiceLibrary.TendermintMethods().GetStatus()
	if status == BftRPCWSStatusUp {
		w.WriteHeader(200)
	}
	w.WriteHeader(400)
}
