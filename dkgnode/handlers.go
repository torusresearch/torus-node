package dkgnode

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/osamingo/jsonrpc"
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
		suite *Suite
	}
	ShareRequestParams struct {
		Index   int    `json:"index"`
		IDToken string `json:"idtoken"`
		Email   string `json:"email"`
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
		TempPubX           string `json:"temppubx`
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
	if err := mr.RegisterMethod("ShareRequest", ShareRequestHandler{suite}, ShareRequestParams{}, ShareRequestResult{}); err != nil {
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
