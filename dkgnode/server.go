package dkgnode

/* All useful imports */
import (
	// b64 "encoding/base64"

	"bytes"
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/mholt/certmagic"
	"github.com/rs/cors"
	logging "github.com/sirupsen/logrus"
	"github.com/torusresearch/bijson"
	pcmn "github.com/torusresearch/torus-node/common"
	"github.com/torusresearch/torus-node/config"
	"github.com/torusresearch/torus-node/eventbus"
	"github.com/torusresearch/torus-node/mrpc"
	"github.com/torusresearch/torus-node/pvss"
	"github.com/torusresearch/torus-node/telemetry"
)

const contentType = "application/json; charset=utf-8"

var serverServiceLibrary ServiceLibrary

func NewServerService(ctx context.Context, eventBus eventbus.Bus) *BaseService {
	serverCtx, cancel := context.WithCancel(context.WithValue(ctx, ContextID, "server"))
	serverService := ServerService{
		cancel:        cancel,
		parentContext: ctx,
		context:       serverCtx,
		eventBus:      eventBus,
	}
	serverServiceLibrary = NewServiceLibrary(serverService.eventBus, serverService.Name())
	return NewBaseService(&serverService)
}

type ServerService struct {
	bs            *BaseService
	cancel        context.CancelFunc
	parentContext context.Context
	context       context.Context
	eventBus      eventbus.Bus

	c *http.Client
}

func (s *ServerService) Name() string {
	return "server"
}

func startManagementRPC(e eventbus.Bus) {
	managementRPCAddr := fmt.Sprintf(":%s", config.GlobalConfig.ManagementRPCPort)
	mrpcServiceLibrary := NewServiceLibrary(e, "mrpc")
	triggerFunctions := mrpc.Actions{
		RetriggerPSS: func() error {
			logging.Debug("retriggering pss...")
			return mrpcServiceLibrary.EthereumMethods().StartPSSMonitor()
		},
	}

	managementRPCHandler, err := mrpc.SetupManagementRPCHander(triggerFunctions)
	if err != nil {
		logging.WithError(err).Fatal("Unable to start ManagementRPCServer")
	} else {
		managementRPCServer := &http.Server{
			Addr:    managementRPCAddr,
			Handler: managementRPCHandler,
		}
		err := managementRPCServer.ListenAndServe()
		if err != nil {
			logging.WithError(err).Fatal("Unable to start ManagementRPCServer")
		}
	}
}

func (s *ServerService) OnStart() error {
	router := setUpRouter(s.eventBus)
	addr := fmt.Sprintf(":%s", config.GlobalConfig.HttpServerPort)
	s.c = &http.Client{
		Timeout: 3 * time.Second,
	}

	if config.GlobalConfig.UseManagementRPC {
		go startManagementRPC(s.eventBus)
	}

	server := &http.Server{
		Addr:    addr,
		Handler: router,
	}
	if config.GlobalConfig.ServeUsingTLS {
		if config.GlobalConfig.UseAutoCert {
			if config.GlobalConfig.PublicURL == "" {
				logging.Fatal("PublicURL is required when UseAutoCert is true")
			}
			logging.Info("starting server with CertMagic...")

			// Force http-01 challenges
			certmagic.Default.DisableTLSALPNChallenge = true

			go func() {
				err := certmagic.HTTPS([]string{config.GlobalConfig.PublicURL}, router)
				if err != nil {
					logging.WithError(err).Fatal()
				}
			}()
			return nil
		}

		if config.GlobalConfig.ServerCert != "" {
			logging.Info("starting HTTPS server with preconfigured certs...")
			go func() {
				err := server.ListenAndServeTLS(config.GlobalConfig.ServerCert,
					config.GlobalConfig.ServerKey)
				if err != nil {
					logging.WithError(err).Fatal()
				}
			}()
			return nil
		} else {
			logging.Fatal("certs not supplied, try running with UseAutoCert")
		}
	} else {
		logging.Info("starting HTTP server...")
		go func() {
			err := server.ListenAndServe()
			if err != nil {
				logging.WithError(err).Fatal()
			}
		}()
	}

	return nil
}
func (s *ServerService) OnStop() error {
	return nil
}
func (s *ServerService) Call(method string, args ...interface{}) (interface{}, error) {
	telemetry.IncrementCounter(pcmn.TelemetryConstants.Generic.TotalServiceCalls, pcmn.TelemetryConstants.Server.Prefix)

	switch method {
	case "request_connection_details":
		telemetry.IncrementCounter(pcmn.TelemetryConstants.Server.RequestConnectionDetailsCounter, pcmn.TelemetryConstants.Server.Prefix)

		var args0 string
		_ = castOrUnmarshal(args[0], &args0)

		return s.RequestConnectionDetails(args0)
	}
	return nil, fmt.Errorf("server service method %v not found", method)
}

type ConnectionDetailsRequestBody struct {
	RpcVersion string                  `json:"jsonrpc"`
	Method     string                  `json:"method"`
	Id         int                     `json:"id"`
	Params     ConnectionDetailsParams `json:"params"`
}

type ConnectionDetails struct {
	TMP2PConnection string
	P2PConnection   string
}

type ConnectionDetailsJRPCResponse struct {
	RpcVersion string                  `json:"jsonrpc"`
	Id         int                     `json:"id"`
	Result     ConnectionDetailsResult `json:"result"`
}

func (s *ServerService) RequestConnectionDetails(endpoint string) (connectionDetails ConnectionDetails, err error) {
	sL := NewServiceLibrary(s.eventBus, "server")
	pubKey := sL.EthereumMethods().GetSelfPublicKey()
	privKey := sL.EthereumMethods().GetSelfPrivateKey()
	connectionDetailsMessage := ConnectionDetailsMessage{
		Message:     "ConnectionDetails",
		Timestamp:   strconv.FormatInt(time.Now().Unix(), 10),
		NodeAddress: sL.EthereumMethods().GetSelfAddress(),
	}
	sig := pvss.ECDSASign(connectionDetailsMessage.String(), &privKey)
	connectionDetailsParams := ConnectionDetailsParams{
		PubKeyX:                  pubKey.X,
		PubKeyY:                  pubKey.Y,
		ConnectionDetailsMessage: connectionDetailsMessage,
		Signature:                sig,
	}
	connectionDetailsRequestBody := ConnectionDetailsRequestBody{
		RpcVersion: "2.0",
		Method:     "ConnectionDetails",
		Id:         10,
		Params:     connectionDetailsParams,
	}

	body, err := bijson.Marshal(connectionDetailsRequestBody)
	if err != nil {
		return connectionDetails, err
	}

	logging.WithField("body", string(body)).Debug("connection details request body")

	substrs := strings.Split(endpoint, ":")
	var uri, port string
	if len(substrs) < 1 {
		return connectionDetails, errors.New("could not get uri from endpoint")
	} else if len(substrs) < 2 {
		uri = substrs[0]
	} else {
		uri = substrs[0]
		port = substrs[1]
	}

	var respErr error
	var resp *http.Response
	if port != "" {
		resp, respErr = s.c.Post(fmt.Sprintf("https://%s:%s/jrpc", uri, port), contentType, bytes.NewBuffer(body))
		if respErr != nil {
			logging.WithError(respErr).Error("could not get connection details https uri port")
			resp, respErr = s.c.Post(fmt.Sprintf("http://%s:%s/jrpc", uri, port), contentType, bytes.NewBuffer(body))
		}
	}
	if respErr != nil {
		logging.WithError(respErr).Error("could not get connection details http uri port")
		resp, respErr = s.c.Post(fmt.Sprintf("https://%s/jrpc", uri), contentType, bytes.NewBuffer(body))
	}
	if respErr != nil {
		logging.WithError(respErr).Error("could not get connection details https uri only")
		resp, respErr = s.c.Post(fmt.Sprintf("http://%s/jrpc", uri), contentType, bytes.NewBuffer(body))
	}
	if respErr != nil {
		logging.WithError(respErr).Error("could not get connection details http uri only")
		return connectionDetails, errors.New("could not get connection Details")
	}

	defer resp.Body.Close()
	responseBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return connectionDetails, err
	}
	logging.WithField("responseBody", string(responseBody)).Debug("responseBody")
	var connectionDetailsJRPCResponse ConnectionDetailsJRPCResponse
	err = bijson.Unmarshal(responseBody, &connectionDetailsJRPCResponse)
	if err != nil {
		return connectionDetails, err
	}
	res := ConnectionDetails{
		TMP2PConnection: connectionDetailsJRPCResponse.Result.TMP2PConnection,
		P2PConnection:   connectionDetailsJRPCResponse.Result.P2PConnection,
	}
	logging.WithField("res", res).Debug("got connection details")
	return res, nil
}

func setUpRouter(eventBus eventbus.Bus) http.Handler {
	mr, err := setUpJRPCHandler(eventBus)
	if err != nil {
		logging.WithError(err).Fatal()
	}

	router := mux.NewRouter().StrictSlash(true)

	router.Handle("/jrpc", mr)
	router.HandleFunc("/healthz", GETHealthz)
	router.HandleFunc("/bftStatus", GetBftStatus)

	router.Use(parseBodyMiddleware)
	router.Use(augmentRequestMiddleware)
	router.Use(loggingMiddleware)
	router.Use(telemetryMiddleware)
	router.Use(authMiddleware(eventBus))

	// Handles functions that should only be availible during debug mode
	if config.GlobalConfig.IsDebug {
		debugMethods, err := setUpDebugHandler(eventBus)
		if err != nil {
			logging.WithError(err).Fatal()
		}
		router.Handle("/debug", debugMethods)
	}

	handler := cors.Default().Handler(router)
	return handler
}

func (s *ServerService) SetBaseService(bs *BaseService) {
	s.bs = bs
}
