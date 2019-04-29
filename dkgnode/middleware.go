package dkgnode

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/gorilla/context"
	"github.com/torusresearch/torus-public/logging"
	"github.com/torusresearch/torus-public/telemetry"
)

func augmentRequestMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// set RPC method
		method := getJRPCMethod(r)
		context.Set(r, "JRPCMethod", method)

		next.ServeHTTP(w, r)
	})
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		methodStr := context.Get(r, "JRPCMethod")
		if methodStr != "" {
			methodStr = fmt.Sprintf(" with jrpc method: %s", methodStr)
		}
		logging.Info(fmt.Sprintf("%s requested %s%s", r.RemoteAddr, r.RequestURI, methodStr))
		next.ServeHTTP(w, r)
	})
}

type jRPCRequeust struct {
	Method string `json:"method"`
}

// getJRPCMethod grabs the `method` field from the request body and returns it
func getJRPCMethod(r *http.Request) string {
	var j jRPCRequeust
	// NOTE: This is necessary, as we are expecting to reread the body later
	// on in the middleware / request chain
	body, err := ioutil.ReadAll(r.Body)
	r.Body = ioutil.NopCloser(bytes.NewReader(body))
	err = json.Unmarshal(body, &j)
	if err != nil {
		logging.Error("could not unmarshal body inside getJRPCMethod")
		return ""
	}

	return j.Method
}

func telemetryMiddleware(next http.Handler) http.Handler {
	// We count requests for particular jRPC / http endpoints
	pingCounter := telemetry.NewCounter("ping_method_count", "counts the number of requests received for ping jrpc method")
	shareRequestCounter := telemetry.NewCounter("share_request_method_count", "counts the number of requests received for ShareRequest jrpc method")
	secretAssignCounter := telemetry.NewCounter("secret_assign_method_count", "counts the number of requests received for SecretAssign jrpc method")
	commitmentRequestCounter := telemetry.NewCounter("commitment_request_method_count", "counts the number of requests received for CommitmentRequest jrpc method")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method := context.Get(r, "JRPCMethod")
		switch method {
		case PingMethod:
			pingCounter.Inc()
		case ShareRequestMethod:
			shareRequestCounter.Inc()
		case SecretAssignMethod:
			secretAssignCounter.Inc()
		case CommitmentRequestMethod:
			commitmentRequestCounter.Inc()
		case "":
			logging.Debug("empty method received")

		default:
			logging.Infof("unknown method received requested: %s", method)
		}
		next.ServeHTTP(w, r)
	})

}

func authenticationMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// NOTE: Always use absolute paths
		// TODO: Should be moved into a seperate router
		if r.URL.Path == "/healthz" {
			next.ServeHTTP(w, r)
			return
		}

		next.ServeHTTP(w, r)
	})

}
