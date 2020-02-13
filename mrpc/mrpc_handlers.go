package mrpc

import (
	"context"
	"fmt"

	logging "github.com/sirupsen/logrus"
	"github.com/torusresearch/bijson"
	"github.com/torusresearch/jsonrpc"
	"github.com/torusresearch/torus-node/config"
)

type Trigger interface {
	TriggerPSS(pssProtocolPrefix string, keygenID string, epochOld int, nOld int, kOld int, tOld int, epochNew int, nNew int, kNew int, tNew int)
}

var (
	logLevelMap = map[string]logging.Level{
		// PanicLevel level, highest level of severity. Logs and then calls panic with the
		// message passed to Debug, Info, ...
		"panic": logging.PanicLevel,
		// FatalLevel level. Logs and then calls `logger.Exit(1)`. It will exit even if the
		// logging level is set to Panic.
		"fatal": logging.FatalLevel,
		// ErrorLevel level. Logs. Used for errors that should definitely be noted.
		// Commonly used for hooks to send errors to an error tracking service.
		"error": logging.ErrorLevel,
		// WarnLevel level. Non-critical entries that deserve eyes.
		"warn": logging.WarnLevel,
		// InfoLevel level. General operational entries about what's going on inside the
		// application.
		"info": logging.InfoLevel,
		// DebugLevel level. Usually only enabled when debugging. Very verbose logging.
		"debug": logging.DebugLevel,
		// TraceLevel level. Designates finer-grained informational events than the Debug.
		"trace": logging.TraceLevel,
	}
)

func (h SetLogLevelHandler) ServeJSONRPC(c context.Context, params *bijson.RawMessage) (interface{}, *jsonrpc.Error) {
	logging.WithField("params", params).Debug("setting log level")
	var p SetLogLevelParams
	if err := jsonrpc.Unmarshal(params, &p); err != nil {
		return nil, err
	}

	if val, ok := logLevelMap[p.Level]; ok {
		logging.SetLevel(val)
		return SetLogLevelResult{}, nil
	}

	return nil, jsonrpc.ErrInvalidParams()
}

func (h SetMutableConfigHandler) ServeJSONRPC(c context.Context, params *bijson.RawMessage) (interface{}, *jsonrpc.Error) {
	logging.WithField("params", params).Debug("setting mutable config")
	var p SetMutableConfigParams
	if err := jsonrpc.Unmarshal(params, &p); err != nil {
		return nil, err
	}
	if p.ValueType == "string" {
		config.GlobalMutableConfig.SetS(p.Key, p.StringValue)
	} else if p.ValueType == "int" {
		config.GlobalMutableConfig.SetI(p.Key, p.IntValue)
	} else if p.ValueType == "bool" {
		config.GlobalMutableConfig.SetB(p.Key, p.BoolValue)
	} else {
		return nil, jsonrpc.ErrInvalidParams()
	}

	return SetMutableConfigResult{NewConfigs: config.GlobalMutableConfig.ListMutableConfigs()}, nil
}

func (h RetriggerPSSHandler) ServeJSONRPC(c context.Context, params *bijson.RawMessage) (interface{}, *jsonrpc.Error) {
	logging.WithField("params", params).Debug("retriggering pss")
	var p RetriggerPSSParams
	if err := jsonrpc.Unmarshal(params, &p); err != nil {
		return nil, err
	}
	if h.RetriggerPSS == nil {
		return nil, &jsonrpc.Error{Code: -32604, Message: "actions are undefined"}
	}
	err := h.RetriggerPSS()
	if err != nil {
		return nil, &jsonrpc.Error{Code: -32604, Message: "Could not retrigger pss, error: " + err.Error()}
	}
	return nil, nil
}

// DealerMessage
func (h DealerMessageHandler) ServeJSONRPC(c context.Context, params *bijson.RawMessage) (interface{}, *jsonrpc.Error) {
	logging.WithField("params", params).Debug("calling DealerMessage")
	var p DealerMessageParams
	var resp DealerMessageResult
	if err := jsonrpc.Unmarshal(params, &p); err != nil {
		logging.WithError(err).Error("could not unmarshal params")
		return nil, err
	}
	if h.HandleDealerMessage == nil {
		return nil, &jsonrpc.Error{Code: -32604, Message: "actions are undefined"}
	}
	err := h.HandleDealerMessage(p.DealerMessage)
	if err != nil {
		return nil, &jsonrpc.Error{Code: -32604, Message: fmt.Sprintf("could not handle dealer message, error: %v", err.Error())}
	}
	return resp, nil
}
