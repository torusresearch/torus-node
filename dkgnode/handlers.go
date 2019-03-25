package dkgnode

import (
	"net/http"
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

	return mr, nil
}

// GETHealthz always responds with 200 and can be used for basic readiness checks
func GETHealthz(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(200)
}
