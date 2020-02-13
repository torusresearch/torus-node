package tmlog

import (
	"fmt"
	"os"

	logging "github.com/sirupsen/logrus"
	"github.com/torusresearch/tendermint/libs/log"
	"github.com/torusresearch/torus-node/idmutex"
	//"github.com/torusresearch/torus-node/idmutex"
)

type tmLoggerLogrus struct {
	fields logging.Fields
	logger *logging.Logger
}

var globalLoggerMutex = struct {
	idmutex.Mutex
}{
	idmutex.Mutex{},
}

func init() {
	logging.SetOutput(os.Stdout)
}

var _ log.Logger = (*tmLoggerLogrus)(nil)

func keyvalsToFields(keyvals []interface{}) logging.Fields {
	var fields = logging.Fields{}

	for i := 0; i < len(keyvals); i += 2 {
		fields[fmt.Sprintf("%v", keyvals[i])] = keyvals[i+1]
	}

	return fields
}

func NewTMLoggerLogrus() log.Logger {
	return &tmLoggerLogrus{logging.Fields{}, logging.New()}
}

func (l *tmLoggerLogrus) Info(msg string, keyvals ...interface{}) {
	globalLoggerMutex.Lock()
	defer globalLoggerMutex.Unlock()
	l.logger.WithFields(l.fields).WithFields(keyvalsToFields(keyvals)).Info(msg)
}

func (l *tmLoggerLogrus) Debug(msg string, keyvals ...interface{}) {
	globalLoggerMutex.Lock()
	defer globalLoggerMutex.Unlock()
	l.logger.WithFields(l.fields).WithFields(keyvalsToFields(keyvals)).Debug(msg)
}

func (l *tmLoggerLogrus) Error(msg string, keyvals ...interface{}) {
	globalLoggerMutex.Lock()
	defer globalLoggerMutex.Unlock()
	l.logger.WithFields(l.fields).WithFields(keyvalsToFields(keyvals)).Error(msg)
}

func (l *tmLoggerLogrus) With(keyvals ...interface{}) log.Logger {
	globalLoggerMutex.Lock()
	defer globalLoggerMutex.Unlock()
	var fields = l.fields

	for k, v := range keyvalsToFields(keyvals) {
		fields[k] = v
	}

	return &tmLoggerLogrus{fields, logging.New()}
}

func NewNoopLogger() log.Logger {
	return log.NewNopLogger()
}
