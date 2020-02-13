package dkgnode

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"io/ioutil"
	"math/big"
	"net/http"
	"regexp"
	"strconv"
	"time"

	pcmn "github.com/torusresearch/torus-node/common"
	"github.com/torusresearch/torus-node/eventbus"

	"github.com/gorilla/context"
	logging "github.com/sirupsen/logrus"
	"github.com/torusresearch/bijson"
	"github.com/torusresearch/torus-common/common"
	"github.com/torusresearch/torus-common/crypto"
	"github.com/torusresearch/torus-common/secp256k1"
	"github.com/torusresearch/torus-node/config"

	"github.com/torusresearch/torus-node/telemetry"
)

const signatureHeaderKey = "torus-signature"
const nonceHeaderKey = "torus-nonce"
const timestampHeaderKey = "torus-timestamp"
const jrpcMethod = "method"
const requestBody = "body"

func parseBodyMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// NOTE: This is necessary, as we are expecting to reread the body later
		// on in the middleware / request chain
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			logging.WithError(err).Error("could not read request body")
			return
		}
		context.Set(r, requestBody, body)
		r.Body = ioutil.NopCloser(bytes.NewReader(body))
		next.ServeHTTP(w, r)
	})
}

func augmentRequestMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// set RPC method
		var j jRPCRequest
		body, ok := context.Get(r, requestBody).([]byte)
		if !ok {
			logging.Error("request body not set on context")
			next.ServeHTTP(w, r)
			return
		}
		err := bijson.Unmarshal(body, &j)
		if err != nil {
			logging.WithField("body", string(body)).WithError(err).Error("could not Unmarshal body getJRPCMethod")
			next.ServeHTTP(w, r)
			return
		}
		context.Set(r, jrpcMethod, j.Method)
		next.ServeHTTP(w, r)
	})
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		methodStr := context.Get(r, jrpcMethod)
		if methodStr != "" {
			logging.WithFields(logging.Fields{
				"RemoteAddr": r.RemoteAddr,
				"RequestURI": r.RequestURI,
				"method":     methodStr,
			}).Info("JRPC Method Requested")
		} else {
			logging.WithFields(logging.Fields{
				"RemoteAddr": r.RemoteAddr,
				"RequestURI": r.RequestURI,
			}).Info("JRPC Method Requested")
		}
		next.ServeHTTP(w, r)
	})
}

type jRPCRequest struct {
	Method string `json:"method"`
}

func telemetryMiddleware(next http.Handler) http.Handler {
	// We count requests for particular jRPC / http endpoints
	// Doesnt need to be concurrent as long as this is the only write

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		method := context.Get(r, jrpcMethod)

		switch method {

		case PingMethod:
			telemetry.IncrementCounter(pcmn.TelemetryConstants.Middleware.PingCounter, pcmn.TelemetryConstants.Middleware.Prefix)

		case ShareRequestMethod:
			telemetry.IncrementCounter(pcmn.TelemetryConstants.Middleware.ShareRequestCounter, pcmn.TelemetryConstants.Middleware.Prefix)

		case KeyAssignMethod:
			telemetry.IncrementCounter(pcmn.TelemetryConstants.Middleware.KeyAssignCounter, pcmn.TelemetryConstants.Middleware.Prefix)

		case CommitmentRequestMethod:
			telemetry.IncrementCounter(pcmn.TelemetryConstants.Middleware.CommitmentRequestCounter, pcmn.TelemetryConstants.Middleware.Prefix)

		case "":
			logging.Debug("empty method received")

		default:
			logging.WithField("method", method).Info("unknown method received requested")

		}
		next.ServeHTTP(w, r)
	})

}

type CustomAuthFields struct {
	Timestamp string `json:"torus-timestamp"`
	Nonce     string `json:"torus-nonce"`
	Signature []byte `json:"torus-signature"`
}

func authMiddleware(eventBus eventbus.Bus) func(next http.Handler) http.Handler {
	authServiceLibrary := NewServiceLibrary(eventBus, "auth")
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !config.GlobalMutableConfig.GetB("JRPCAuth") {
				logging.WithField("Signature Header", r.Header.Get(signatureHeaderKey)).Debug("Skipping authentication checks")
				bb := context.Get(r, requestBody)
				logging.WithField("bb", bb).Debug("requestbody")
				byt, err := bijson.Marshal(r)
				logging.WithField("marshalerr", err).WithField("requestbyt", string(byt)).WithField("contentlength", r.ContentLength).Debug("request after jrpcauth skipped")
				next.ServeHTTP(w, r)
				return
			}
			method := context.Get(r, jrpcMethod)
			// skip auth checks if not KeyAssign
			if method != KeyAssignMethod {
				next.ServeHTTP(w, r)
				return
			}

			// KeyAssign
			signatureHeader := r.Header.Get(signatureHeaderKey)
			nonceHeader := r.Header.Get(nonceHeaderKey)
			timestampHeader := r.Header.Get(timestampHeaderKey)
			if signatureHeader != "" && nonceHeader != "" && timestampHeader != "" {
				bodyBytes, ok := context.Get(r, requestBody).([]byte)
				if !ok {
					rejectResponse(w, fmt.Errorf("body on request is not bytes"))
					return
				}

				timestamp, err := strconv.ParseInt(timestampHeader, 10, 64)
				if err != nil {
					rejectResponse(w, fmt.Errorf("error getting timestamp from header %v", err))
					return
				}

				if time.Now().After(time.Unix(timestamp, 0).Add(time.Minute)) {
					rejectResponse(w, fmt.Errorf("signature expired, signed at: %v", timestamp))
					return
				}

				if authServiceLibrary.CacheMethods().SignerSigExists(signatureHeader) {
					rejectResponse(w, fmt.Errorf("signature %v already seen", signatureHeader))
					return
				}

				authServiceLibrary.CacheMethods().RecordSignerSig(signatureHeader)

				var captchaX, captchaY big.Int
				captchaX.SetString(config.GlobalMutableConfig.GetS("JRPCAuthPubKeyX"), 16)
				captchaY.SetString(config.GlobalMutableConfig.GetS("JRPCAuthPubKeyY"), 16)

				captchaPubKey := common.Point{X: captchaX, Y: captchaY}
				logging.WithField("method", method).Debug()
				rawSig, err := base64.StdEncoding.DecodeString(signatureHeader)
				if err != nil {
					rejectResponse(w, fmt.Errorf("error decoding signature from hex string to bytes, err: %v", err))
					return
				}

				ethPrivKeyStr := config.GlobalConfig.EthPrivateKey
				ethPrivKey, ok := new(big.Int).SetString(ethPrivKeyStr, 16)
				if !ok {
					rejectResponse(w, fmt.Errorf("could not setString for ethPrivKey"))
					return
				}
				suffix := pcmn.Delimiter1 + timestampHeader + pcmn.Delimiter1 + nonceHeader
				sigData := append(bodyBytes, suffix...)

				pubKeyX, pubKeyY := secp256k1.Curve.ScalarBaseMult(ethPrivKey.Bytes())
				if !crypto.VerifyPtFromRawWithPubKey(sigData, pubKeyX.Text(16), pubKeyY.Text(16), captchaPubKey, rawSig) {
					rejectResponse(w, fmt.Errorf("invalid signature"))
					return
				}
				next.ServeHTTP(w, r)
				return
			}

			// TODO: remove, deprecated, kept here for backward compatibility
			bodyBytes, ok := context.Get(r, requestBody).([]byte)
			if !ok {
				rejectResponse(w, fmt.Errorf("in deprecated, body on request is not bytes"))
				return
			}
			var customAuthFields CustomAuthFields
			err := bijson.Unmarshal(bodyBytes, &customAuthFields)
			if err != nil {
				rejectResponse(w, fmt.Errorf("in deprecated, could not unmarshal to get customAuthFields %v", err))
				return
			}

			timestamp, err := strconv.ParseInt(customAuthFields.Timestamp, 10, 64)
			if err != nil {
				rejectResponse(w, fmt.Errorf("in deprecated, error timestamp from body %v", err))
				return
			}

			if time.Now().After(time.Unix(timestamp, 0).Add(time.Minute)) {
				rejectResponse(w, fmt.Errorf("in deprecated, signature expired, signed at: %v", timestamp))
				return
			}

			if authServiceLibrary.CacheMethods().SignerSigExists(base64.StdEncoding.EncodeToString(customAuthFields.Signature)) {
				rejectResponse(w, fmt.Errorf("in deprecated, signature %v already seen", customAuthFields.Signature))
				return
			}

			authServiceLibrary.CacheMethods().RecordSignerSig(base64.StdEncoding.EncodeToString(customAuthFields.Signature))

			var captchaX, captchaY big.Int
			captchaX.SetString(config.GlobalMutableConfig.GetS("JRPCAuthPubKeyX"), 16)
			captchaY.SetString(config.GlobalMutableConfig.GetS("JRPCAuthPubKeyY"), 16)

			captchaPubKey := common.Point{X: captchaX, Y: captchaY}
			logging.WithField("method", method).Debug()
			rawSig := customAuthFields.Signature
			if err != nil {
				rejectResponse(w, fmt.Errorf("in deprecated, error decoding signature from hex string to bytes, err: %v", err))
				return
			}

			filteredBytes := filterBytes(bodyBytes, "torus-signature", "torus-nonce", "torus-timestamp")

			ethPrivKeyStr := config.GlobalConfig.EthPrivateKey
			ethPrivKey, ok := new(big.Int).SetString(ethPrivKeyStr, 16)
			if !ok {
				rejectResponse(w, fmt.Errorf("in deprecated, could not setString for ethPrivKey"))
				return
			}
			suffix := pcmn.Delimiter1 + customAuthFields.Timestamp + pcmn.Delimiter1 + customAuthFields.Nonce
			sigData := append(filteredBytes, suffix...)

			pubKeyX, pubKeyY := secp256k1.Curve.ScalarBaseMult(ethPrivKey.Bytes())
			if !crypto.VerifyPtFromRawWithPubKey(sigData, pubKeyX.Text(16), pubKeyY.Text(16), captchaPubKey, rawSig) {
				rejectResponse(w, fmt.Errorf("in deprecated, invalid signature in body %v", string(filteredBytes)))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func filterBytes(bytes []byte, keys ...string) []byte {
	str := bytes
	for _, key := range keys {
		r, err := regexp.Compile(`\s*"` + key + `"\s*:\s*".*?",?`)
		if err != nil {
			continue
		}
		str = r.ReplaceAll(str, nil)
	}
	r, err := regexp.Compile(`(\s*),(\s*)}$`)
	if err != nil {
		return bytes
	}
	str = r.ReplaceAll(str, []byte("$1$2}"))
	return str
}

func rejectResponse(w http.ResponseWriter, errLog error) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8") // normal header
	w.WriteHeader(401)
	_, err := io.WriteString(w, "Error authenticating, err: "+errLog.Error())
	if err != nil {
		logging.WithError(err).Error("could not write string to io in reject response")
	}
}

func setupRequestLoggingMiddleware(serviceRegistry *ServiceRegistry) {
	requestLoggingMiddleware := func(methodRequest MethodRequest) MethodRequest {
		logging.Debugf("ID %v, Caller %v called Method %v with Data %v", methodRequest.ID, methodRequest.Caller, methodRequest.Method, stringify(methodRequest.Data))
		return methodRequest
	}
	serviceRegistry.AddRequestMiddleware(&requestLoggingMiddleware)
}

func setupResponseLoggingMiddleware(serviceRegistry *ServiceRegistry) {
	responseLoggingMiddleware := func(methodResponse MethodResponse) MethodResponse {
		logging.Debugf("ID %v Caller %v called Method %v returned Data %v", methodResponse.Request.ID, methodResponse.Request.Caller, methodResponse.Request.Method, methodResponse.Data)
		return methodResponse
	}
	serviceRegistry.AddResponseMiddleware(&responseLoggingMiddleware)
}
