package dkgnode

import (
	"context"
	"fmt"
	"github.com/torusresearch/torus-node/telemetry"
	"math/big"

	logging "github.com/sirupsen/logrus"
	"github.com/torusresearch/bijson"
	"github.com/torusresearch/tendermint/abci/server"
	tmcmn "github.com/torusresearch/tendermint/libs/common"
	"github.com/torusresearch/tendermint/libs/log"
	pcmn "github.com/torusresearch/torus-node/common"
	"github.com/torusresearch/torus-node/config"
	"github.com/torusresearch/torus-node/eventbus"
	"github.com/torusresearch/torus-node/pvss"
	"github.com/torusresearch/torus-node/tmlog"
)

var abciServiceLibrary ServiceLibrary

func NewABCIService(ctx context.Context, eventBus eventbus.Bus) *BaseService {
	abciCtx, cancel := context.WithCancel(context.WithValue(ctx, ContextID, "abci"))
	abciService := ABCIService{
		cancel:   cancel,
		ctx:      abciCtx,
		eventBus: eventBus,
	}
	abciServiceLibrary = NewServiceLibrary(abciService.eventBus, abciService.Name())
	return NewBaseService(&abciService)
}

type ABCIService struct {
	bs       *BaseService
	cancel   context.CancelFunc
	ctx      context.Context
	eventBus eventbus.Bus

	ABCIApp *ABCIApp
}

func (a *ABCIService) Name() string {
	return "abci"
}
func (a *ABCIService) OnStart() error {
	return a.RunABCIServer(a.eventBus)
}
func (a *ABCIService) OnStop() error {
	return nil
}

func (a *ABCIService) Call(method string, args ...interface{}) (interface{}, error) {
	telemetry.IncrementCounter(pcmn.TelemetryConstants.Generic.TotalServiceCalls, pcmn.TelemetryConstants.ABCIServer.Prefix)

	switch method {
	// LastCreatedIndex() (keyIndex uint)
	case "last_created_index":
		telemetry.IncrementCounter(pcmn.TelemetryConstants.ABCIServer.LastCreatedIndexCounter, pcmn.TelemetryConstants.ABCIServer.Prefix)
		return a.ABCIApp.state.LastCreatedIndex, nil
	// LastUnassignedIndex() (keyIndex uint)
	case "last_unassigned_index":
		telemetry.IncrementCounter(pcmn.TelemetryConstants.ABCIServer.LastUnAssignedIndexCounter, pcmn.TelemetryConstants.ABCIServer.Prefix)
		return a.ABCIApp.state.LastUnassignedIndex, nil
	// RetrieveKeyMapping(keyIndex big.Int) (keyDetails KeyAssignmentPublic, err error)
	// Retrieves KeyAssignment mapping which provides information such as, which verifiers/access structure to the key
	// This is safe to the public
	case "retrieve_key_mapping":
		telemetry.IncrementCounter(pcmn.TelemetryConstants.ABCIServer.RetrieveKeyMappingCounter, pcmn.TelemetryConstants.ABCIServer.Prefix)
		var args0 big.Int
		_ = castOrUnmarshal(args[0], &args0)

		keyDetails, err := a.ABCIApp.retrieveKeyMapping(args0)
		if err != nil {
			return nil, err
		}
		return *keyDetails, err
	// GetIndexesFromVerifierID(verifier, veriferID string) (keyIndexes []big.Int, err error)
	case "get_indexes_from_verifier_id":
		telemetry.IncrementCounter(pcmn.TelemetryConstants.ABCIServer.GetIndexesFromVerifierIdCounter, pcmn.TelemetryConstants.ABCIServer.Prefix)
		var args0, args1 string
		_ = castOrUnmarshal(args[0], &args0)
		_ = castOrUnmarshal(args[1], &args1)

		keyIndexes, err := a.ABCIApp.getIndexesFromVerifierID(args0, args1)
		return keyIndexes, err
	case "get_verifier_iterator":
		telemetry.IncrementCounter(pcmn.TelemetryConstants.ABCIServer.GetVerifierIteratorCounter, pcmn.TelemetryConstants.ABCIServer.Prefix)
		randomID := pvss.RandomBigInt().Text(16)
		iterator := a.ABCIApp.db.Iterator(verifierToKeyIndexPrefixKey, endVerifierToKeyIndexKey)
		a.ABCIApp.dbIterators.Set(randomID, iterator)
		return randomID, nil
	case "get_verifier_iterator_next":
		telemetry.IncrementCounter(pcmn.TelemetryConstants.ABCIServer.GetVerifierIteratorNextCounter, pcmn.TelemetryConstants.ABCIServer.Prefix)
		var args0 string
		_ = castOrUnmarshal(args[0], &args0)

		var responseStruct pcmn.VerifierData
		randomID := args0
		if !a.ABCIApp.dbIterators.Exists(randomID) {
			return nil, fmt.Errorf("Could not find dbIterator for randomID %s", randomID)
		}
		iterator := a.ABCIApp.dbIterators.Get(randomID)
		if iterator == nil {
			return nil, fmt.Errorf("Iterator invalid for randomID %s", randomID)
		}
		if !iterator.Valid() {
			iterator.Close()
			a.ABCIApp.dbIterators.Delete(randomID)
			responseStruct.Ok = false
			return responseStruct, nil
		}
		verifier, verifierID, err := deconstructVerifierKey(iterator.Key())
		if err != nil {
			responseStruct.Err = fmt.Errorf("key is not in right format for %s", randomID)
			responseStruct.Ok = true
			return responseStruct, nil
		}
		var keyIndexes []big.Int
		err = bijson.Unmarshal(iterator.Value(), &keyIndexes)
		if err != nil {
			responseStruct.Ok = true
			responseStruct.Err = fmt.Errorf("could not unmarshal keyIndexes %s", randomID)
			return responseStruct, nil
		}
		iterator.Next()
		responseStruct.Ok = true
		responseStruct.Verifier = verifier
		responseStruct.VerifierID = verifierID
		responseStruct.KeyIndexes = keyIndexes
		return responseStruct, nil
	}

	return nil, fmt.Errorf("ABCI service method %v not found", method)
}
func (a *ABCIService) SetBaseService(bs *BaseService) {
	a.bs = bs
}

func (a *ABCIService) RunABCIServer(eventBus eventbus.Bus) error {
	var logger log.Logger
	if config.GlobalConfig.TendermintLogging {
		logger = tmlog.NewTMLoggerLogrus()
	} else {
		logger = tmlog.NewNoopLogger()
	}
	a.ABCIApp = a.NewABCIApp()
	// Start the listener
	srv, err := server.NewServer(config.GlobalConfig.ABCIServer, "socket", a.ABCIApp)
	if err != nil {
		return err
	}

	srv.SetLogger(logger.With("module", "abci-server"))
	if err := srv.Start(); err != nil {
		return err
	}
	go func() {
		// Wait forever
		tmcmn.TrapSignal(logger.With("module", "trap-signal"), func() {
			// Cleanup
			err := srv.Stop()
			if err != nil {
				logging.WithError(err).Error("could not stop service")
			}
		})
	}()
	return nil
}
