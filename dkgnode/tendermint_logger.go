package dkgnode

import (
	"fmt"

	tmlog "github.com/tendermint/tendermint/libs/log"
	"github.com/torusresearch/torus-public/logging"
)

type NoLogger struct {
	tmlog.Logger
}

func (NoLogger) Debug(msg string, keyvals ...interface{}) {
}

func (NoLogger) Info(msg string, keyvals ...interface{}) {
}

func (NoLogger) Error(msg string, keyvals ...interface{}) {
}

func (NoLogger) With(keyvals ...interface{}) tmlog.Logger {
	return NoLogger{}
}

type TorusTMLogger struct {
	tmlog.Logger
	prefix   string
	loglevel string
	logger   logging.Logger
}

func NewTMLogger(logLevelString string, prefix string) *TorusTMLogger {
	logger := logging.NewDefault().WithLevelString(logLevelString)
	return &TorusTMLogger{
		prefix:   prefix,
		loglevel: logLevelString,
		logger:   logger,
	}
}

func concatStringVals(msg string, keyvals ...interface{}) string {
	finalKVs := ""
	for _, v := range keyvals {
		finalKVs += fmt.Sprintf("%v", v)
	}
	return msg + finalKVs
}

func (t TorusTMLogger) Debug(msg string, keyvals ...interface{}) {
	finalMsg := concatStringVals(msg, keyvals)
	t.Debug(finalMsg)
}

func (t TorusTMLogger) Info(msg string, keyvals ...interface{}) {
	finalMsg := concatStringVals(msg, keyvals)
	t.Info(finalMsg)
}

func (t TorusTMLogger) Error(msg string, keyvals ...interface{}) {
	finalMsg := concatStringVals(msg, keyvals)
	t.Error(finalMsg)
}

func (t TorusTMLogger) With(keyvals ...interface{}) tmlog.Logger {
	return TorusTMLogger{}
}

type EventForwardingLogger struct {
	tmlog.Logger
}

func (EventForwardingLogger) Debug(msg string, keyvals ...interface{}) {
}

func (EventForwardingLogger) Info(msg string, keyvals ...interface{}) {
}

func (EventForwardingLogger) Error(msg string, keyvals ...interface{}) {
}

func (EventForwardingLogger) With(keyvals ...interface{}) tmlog.Logger {
	if keyvals[0].(string) == "module" && keyvals[1].(string) == "events" {
		return NewTMLogger("debug", "[TM][EVENTS]")
	} else if keyvals[0].(string) == "module" && keyvals[1].(string) == "rpc-server" {
		return NewTMLogger("debug", "[TM][RPC]")
	} else if keyvals[0].(string) == "module" && keyvals[1].(string) == "websocket" {
		return NewTMLogger("debug", "[TM][WS]")
	} else {
		return EventForwardingLogger{}
	}
}
