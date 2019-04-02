package dkgnode

import (
	"errors"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/osamingo/jsonrpc"
	"github.com/torusresearch/torus-public/secp256k1"
)

type (
	PingHandler struct {
		ethSuite *EthSuite
	}
	PingParams struct {
		Message string `json:"message"`
	}
	PingResult struct {
		Message string `json:"message"`
	}
	SigncryptedHandler struct {
		suite *Suite
	}
	ShareLog struct {
		Timestamp          time.Time
		LogNumber          int
		ShareIndex         int
		UnsigncryptedShare []byte
		BroadcastId        []byte
	}
	ShareRequestHandler struct {
		suite   *Suite
		TimeNow func() time.Time
	}

	ValidatedNodeSignature struct {
		NodeSignature
		NodeIndex big.Int
	}

	NodeSignature struct {
		Signature   string
		Data        string
		NodePubKeyX string
		NodePubKeyY string
	}
	ShareRequestParams struct {
		ID                 string          `json:"id"`
		Token              string          `json:"token"`
		NodeSignatures     []NodeSignature `json:"nodesignatures"`
		VerifierIdentifier string          `json:verifieridentifier`
	}
	ShareRequestResult struct {
		Index    int    `json:"index"`
		HexShare string `json:"hexshare"`
	}
	CommitmentRequestHandler struct {
		suite   *Suite
		TimeNow func() time.Time
	}
	CommitmentRequestParams struct {
		MessagePrefix      string `json:"messageprefix"`
		TokenCommitment    string `json:"tokencommitment"`
		TempPubX           string `json:"temppubx"`
		TempPubY           string `json:"temppuby"`
		Timestamp          string `json:"timestamp"`
		VerifierIdentifier string `json:"verifieridentifier"`
	}

	CommitmentRequestResultData struct {
		MessagePrefix      string
		TokenCommitment    string
		TempPubX           string
		TempPubY           string
		Timestamp          string
		VerifierIdentifier string
		TimeSigned         string
	}
	CommitmentRequestResult struct {
		Signature string `json:"signature"`
		Data      string `json:"data"`
		NodePubX  string `json:"nodepubx"`
		NodePubY  string `json:"nodepuby"`
	}
	SecretAssignHandler struct {
		suite *Suite
	}
	SecretAssignParams struct {
		Email string `json:"email"`
	}
	SecretAssignResult struct {
		ShareIndex int    `json:"id"`
		PubShareX  string `json:"pubshareX"`
		PubShareY  string `json:"pubshareY"`
		Address    string `json:"address"`
	}
)

func (nodeSig *NodeSignature) NodeValidation(suite *Suite) (*NodeReference, error) {
	var node *NodeReference
	for i := 0; i < len(suite.EthSuite.NodeList); i++ {
		currNode := suite.EthSuite.NodeList[i]
		if currNode.PublicKey.X.Text(16) == nodeSig.NodePubKeyX &&
			currNode.PublicKey.Y.Text(16) == nodeSig.NodePubKeyY {
			node = currNode
		}
	}
	if node == nil {
		return nil, errors.New("Node not found")
	}
	recSig := HexToECDSASig(nodeSig.Signature)
	var sig32 [32]byte
	copy(sig32[:], secp256k1.Keccak256([]byte(nodeSig.Data))[:32])
	recoveredSig := ECDSASignature{
		recSig.Raw,
		sig32,
		recSig.R,
		recSig.S,
		recSig.V - 27,
	}
	valid := ECDSAVerify(*node.PublicKey, recoveredSig)
	if !valid {
		return nil, errors.New("Could not validate ecdsa signature")
	}

	// check if time signed is after timestamp
	var commitmentRequestResultData CommitmentRequestResultData
	ok, err := commitmentRequestResultData.FromString(nodeSig.Data)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errors.New("Could not parse data from string")
	}
	timestamp, err := strconv.ParseInt(commitmentRequestResultData.Timestamp, 10, 64)
	if err != nil {
		return nil, &jsonrpc.Error{Code: 32603, Message: "Internal error", Data: "Could not parse timestamp"}
	}
	now, err := strconv.ParseInt(commitmentRequestResultData.TimeSigned, 10, 64)
	if err != nil {
		return nil, &jsonrpc.Error{Code: 32603, Message: "Internal error", Data: "Could not parse timestamp"}
	}
	if time.Unix(now, 0).Before(time.Unix(timestamp, 0)) {
		return nil, &jsonrpc.Error{Code: 32603, Message: "Internal error", Data: "Node signed before timestamp"}
	}
	if time.Unix(now, 0).After(time.Unix(timestamp+60, 0)) {
		return nil, &jsonrpc.Error{Code: 32603, Message: "Internal error", Data: "Node took too long to sign (> 60 seconds)"}
	}

	return node, nil
}

func (p *CommitmentRequestParams) ToString() string {
	accumulator := ""
	accumulator = accumulator + p.MessagePrefix + "|"
	accumulator = accumulator + p.TokenCommitment + "|"
	accumulator = accumulator + p.TempPubX + "|"
	accumulator = accumulator + p.TempPubY + "|"
	accumulator = accumulator + p.Timestamp + "|"
	accumulator = accumulator + p.VerifierIdentifier
	return accumulator
}

func (c *CommitmentRequestResultData) ToString() string {
	accumulator := ""
	accumulator = accumulator + c.MessagePrefix + "|"
	accumulator = accumulator + c.TokenCommitment + "|"
	accumulator = accumulator + c.TempPubX + "|"
	accumulator = accumulator + c.TempPubY + "|"
	accumulator = accumulator + c.Timestamp + "|"
	accumulator = accumulator + c.VerifierIdentifier + "|"
	accumulator = accumulator + c.TimeSigned
	return accumulator
}

func (c *CommitmentRequestResultData) FromString(data string) (bool, error) {
	dataString := string(data)
	dataArray := strings.Split(dataString, "|")
	if len(dataArray) < 7 {
		return false, errors.New("Could not parse commitmentrequestresultdata")
	}
	c.MessagePrefix = dataArray[0]
	c.TokenCommitment = dataArray[1]
	c.TempPubX = dataArray[2]
	c.TempPubY = dataArray[3]
	c.Timestamp = dataArray[4]
	c.VerifierIdentifier = dataArray[5]
	c.TimeSigned = dataArray[6]
	return true, nil
}

func setUpJRPCHandler(suite *Suite) (*jsonrpc.MethodRepository, error) {
	mr := jsonrpc.NewMethodRepository()

	if err := mr.RegisterMethod("Ping", PingHandler{suite.EthSuite}, PingParams{}, PingResult{}); err != nil {
		return nil, err
	}
	if err := mr.RegisterMethod("ShareRequest", ShareRequestHandler{suite, time.Now}, ShareRequestParams{}, ShareRequestResult{}); err != nil {
		return nil, err
	}
	if err := mr.RegisterMethod("SecretAssign", SecretAssignHandler{suite}, SecretAssignParams{}, SecretAssignResult{}); err != nil {
		return nil, err
	}
	if err := mr.RegisterMethod("CommitmentRequest", CommitmentRequestHandler{suite, time.Now}, CommitmentRequestParams{}, CommitmentRequestResult{}); err != nil {
		return nil, err
	}

	return mr, nil
}

// GETHealthz always responds with 200 and can be used for basic readiness checks
func GETHealthz(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(200)
}
