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

	l logging.Logger
}

func NewTMLogger(logLevelString string) *TorusTMLogger {
	l := logging.NewDefault().WithLevelString(logLevelString)
	return &TorusTMLogger{
		l: l,
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
