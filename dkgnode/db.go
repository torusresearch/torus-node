package dkgnode

import (
	"context"
	"errors"
	"fmt"
	"math/big"

	ethCommon "github.com/ethereum/go-ethereum/common"
	pcmn "github.com/torusresearch/torus-node/common"
	"github.com/torusresearch/torus-node/telemetry"

	"github.com/torusresearch/torus-common/common"

	"github.com/torusresearch/torus-node/config"
	"github.com/torusresearch/torus-node/db"
	"github.com/torusresearch/torus-node/eventbus"
)

func NewDatabaseService(ctx context.Context, eb eventbus.Bus) *BaseService {
	dbCtx, cancel := context.WithCancel(context.WithValue(ctx, ContextID, "database"))
	databaseService := DatabaseService{
		cancel:        cancel,
		parentContext: ctx,
		context:       dbCtx,
		eventBus:      eb,
	}
	databaseService.serviceLibrary = NewServiceLibrary(databaseService.eventBus, databaseService.Name())
	return NewBaseService(&databaseService)
}

type DatabaseService struct {
	dbInstance *db.TorusLDB

	bs             *BaseService
	cancel         context.CancelFunc
	parentContext  context.Context
	context        context.Context
	eventBus       eventbus.Bus
	serviceLibrary ServiceLibrary
}

type TokenCommitmentData struct {
	Exists bool
	PubKey common.Point
}

func (d *DatabaseService) Name() string {
	return "database"
}

func (d *DatabaseService) OnStart() error {
	torusLdb, err := db.NewTorusLDB(fmt.Sprintf("%s/torusdb", config.GlobalConfig.BasePath))
	if err != nil {
		return errors.New("Was not able to start leveldb: " + err.Error())
	}
	d.dbInstance = torusLdb
	return nil
}
func (d *DatabaseService) OnStop() error {
	return nil
}
func (d *DatabaseService) Call(method string, args ...interface{}) (interface{}, error) {

	telemetry.IncrementCounter(pcmn.TelemetryConstants.Generic.TotalServiceCalls, pcmn.TelemetryConstants.DB.Prefix)

	switch method {
	// StoreNodePubKey(nodeAddress ethCommon.Address, pubKey common.Point) error
	case "store_node_pub_key":
		telemetry.IncrementCounter(pcmn.TelemetryConstants.DB.StoreNodePubKeyCounter, pcmn.TelemetryConstants.DB.Prefix)

		var args0 ethCommon.Address
		var args1 common.Point
		_ = castOrUnmarshal(args[0], &args0)
		_ = castOrUnmarshal(args[1], &args1)

		err := d.dbInstance.StoreNodePubKey(args0, args1)
		return nil, err

		// RetrieveNodePubKey(nodeAddress ethCommon.Address) (pubKey common.Point, err error)
	case "retrieve_node_pub_key":
		telemetry.IncrementCounter(pcmn.TelemetryConstants.DB.RetrieveNodePubKeyCounter, pcmn.TelemetryConstants.DB.Prefix)

		var args0 ethCommon.Address
		_ = castOrUnmarshal(args[0], &args0)

		return d.dbInstance.RetrieveNodePubKey(args0)

	// StoreConnectionDetails(nodeAddress ethCommon.Address, connectionDetails ConnectionDetails) error
	case "store_connection_details":
		telemetry.IncrementCounter(pcmn.TelemetryConstants.DB.StoreConnectionDetailsCounter, pcmn.TelemetryConstants.DB.Prefix)

		var args0 ethCommon.Address
		var args1 ConnectionDetails
		_ = castOrUnmarshal(args[0], &args0)
		_ = castOrUnmarshal(args[1], &args1)

		err := d.dbInstance.StoreConnectionDetails(args0, args1.TMP2PConnection, args1.P2PConnection)
		return nil, err

	// RetrieveConnectionDetails(nodeAddress ethCommon.Address) (connectionDetails ConnectionDetails, err error)
	case "retrieve_connection_details":
		telemetry.IncrementCounter(pcmn.TelemetryConstants.DB.RetrieveConnectionDetailsCounter, pcmn.TelemetryConstants.DB.Prefix)

		var args0 ethCommon.Address
		_ = castOrUnmarshal(args[0], &args0)

		tmP2PConnection, P2PConnection, err := d.dbInstance.RetrieveConnectionDetails(args0)
		return ConnectionDetails{
			TMP2PConnection: tmP2PConnection,
			P2PConnection:   P2PConnection,
		}, err

	// StoreKeygenCommitmentMatrix(keyIndex big.Int, c [][]common.Point) error
	case "store_keygen_commitment_matrix":
		telemetry.IncrementCounter(pcmn.TelemetryConstants.DB.StoreKeygenCommitmentMatrixCounter, pcmn.TelemetryConstants.DB.Prefix)

		var args0 big.Int
		var args1 [][]common.Point
		_ = castOrUnmarshal(args[0], &args0)
		_ = castOrUnmarshal(args[1], &args1)

		err := d.dbInstance.StoreKeygenCommitmentMatrix(args0, args1)
		return nil, err
	// StorePSSCommitmentMatrix(keyIndex big.Int, c [][]common.Point) error
	case "store_PSS_commitment_matrix":
		telemetry.IncrementCounter(pcmn.TelemetryConstants.DB.StorePSSCommitmentMatrixCounter, pcmn.TelemetryConstants.DB.Prefix)

		var args0 big.Int
		var args1 [][]common.Point
		_ = castOrUnmarshal(args[0], &args0)
		_ = castOrUnmarshal(args[1], &args1)

		err := d.dbInstance.StorePSSCommitmentMatrix(args0, args1)
		return nil, err
	// StoreCompletedKeygenShare(keyIndex big.Int, si big.Int, siprime big.Int) error
	case "store_completed_keygen_share":
		telemetry.IncrementCounter(pcmn.TelemetryConstants.DB.StoreCompletedKeygenShareCounter, pcmn.TelemetryConstants.DB.Prefix)

		var args0, args1, args2 big.Int
		_ = castOrUnmarshal(args[0], &args0)
		_ = castOrUnmarshal(args[1], &args1)
		_ = castOrUnmarshal(args[2], &args2)

		err := d.dbInstance.StoreCompletedKeygenShare(args0, args1, args2)
		return nil, err
	// StoreCompletedPSSShare(keyIndex big.Int, si big.Int, siprime big.Int) error
	case "store_completed_PSS_share":
		telemetry.IncrementCounter(pcmn.TelemetryConstants.DB.StoreCompletedPssShareCounter, pcmn.TelemetryConstants.DB.Prefix)

		var args0, args1, args2 big.Int
		_ = castOrUnmarshal(args[0], &args0)
		_ = castOrUnmarshal(args[1], &args1)
		_ = castOrUnmarshal(args[2], &args2)

		err := d.dbInstance.StoreCompletedPSSShare(args0, args1, args2)
		return nil, err
	// StorePublicKeyToIndex(publicKey common.Point, keyIndex big.Int) error
	case "store_public_key_to_index":
		telemetry.IncrementCounter(pcmn.TelemetryConstants.DB.StorePublicKeyToIndexCounter, pcmn.TelemetryConstants.DB.Prefix)

		var args0 common.Point
		var args1 big.Int
		_ = castOrUnmarshal(args[0], &args0)
		_ = castOrUnmarshal(args[1], &args1)

		err := d.dbInstance.StorePublicKeyToKeyIndex(args0, args1)
		return nil, err
	// RetrieveCommitmentMatrix(keyIndex big.Int) (c [][]common.Point, err error)
	case "retrieve_commitment_matrix":
		telemetry.IncrementCounter(pcmn.TelemetryConstants.DB.RetrieveCommitmentMatrixCounter, pcmn.TelemetryConstants.DB.Prefix)

		var args0 big.Int
		_ = castOrUnmarshal(args[0], &args0)

		keyIndex := args0
		c, err := d.dbInstance.RetrieveCommitmentMatrix(keyIndex)
		if err != nil {
			return nil, err
		}
		return c, nil
	// RetrievePublicKeyToIndex(publicKey common.Point) (keyIndex big.Int, err error)
	case "retrieve_public_key_to_index":
		telemetry.IncrementCounter(pcmn.TelemetryConstants.DB.RetrievePublicKeyToIndexCounter, pcmn.TelemetryConstants.DB.Prefix)

		var args0 common.Point
		_ = castOrUnmarshal(args[0], &args0)

		keyIndex, err := d.dbInstance.RetrievePublicKeyToKeyIndex(args0)
		if err != nil || keyIndex == nil {
			return big.NewInt(-1), err
		}
		return *keyIndex, err
	// RetrieveIndexToPublicKey(keyIndex big.Int) (publicKey common.Point, err error)
	case "retrieve_index_to_public_key":
		telemetry.IncrementCounter(pcmn.TelemetryConstants.DB.RetrieveIndexToPublicKeyCounter, pcmn.TelemetryConstants.DB.Prefix)

		var args0 big.Int
		_ = castOrUnmarshal(args[0], &args0)

		publicKey, err := d.dbInstance.RetrieveKeyIndexToPublicKey(args0)
		if err != nil || publicKey == nil {
			return common.Point{}, err
		}
		return *publicKey, err
		// IndexToPublicKeyExists(keyIndex big.Int) (bool)
	case "index_to_public_key_exists":
		telemetry.IncrementCounter(pcmn.TelemetryConstants.DB.IndexToPublicKeyCounterExists, pcmn.TelemetryConstants.DB.Prefix)

		var args0 big.Int
		_ = castOrUnmarshal(args[0], &args0)

		exists := d.dbInstance.KeyIndexToPublicKeyExists(args0)
		return exists, nil

	// RetrieveCompletedShare(keyIndex big.Int) (Si big.Int, Siprime big.Int, err error)
	case "retrieve_completed_share":
		telemetry.IncrementCounter(pcmn.TelemetryConstants.DB.RetrieveCompletedShareCounter, pcmn.TelemetryConstants.DB.Prefix)

		var args0 big.Int
		_ = castOrUnmarshal(args[0], &args0)

		rs := new(struct {
			Si      big.Int
			Siprime big.Int
		})
		si, sip, err := d.dbInstance.RetrieveCompletedShare(args0)
		if si == nil {
			si = big.NewInt(0)
		}
		if sip == nil {
			sip = big.NewInt(0)
		}
		rs.Si = *si
		rs.Siprime = *sip
		return *rs, err
	// GetShareCount() (count int)
	case "get_share_count":
		telemetry.IncrementCounter(pcmn.TelemetryConstants.DB.GetShareCountCounter, pcmn.TelemetryConstants.DB.Prefix)

		return d.dbInstance.GetShareCount(), nil
	case "get_keygen_started":
		telemetry.IncrementCounter(pcmn.TelemetryConstants.DB.GetKeygenStartedCounter, pcmn.TelemetryConstants.DB.Prefix)

		var args0 string
		_ = castOrUnmarshal(args[0], &args0)

		return d.dbInstance.GetKeygenStarted(args0), nil
	case "set_keygen_started":
		telemetry.IncrementCounter(pcmn.TelemetryConstants.DB.SetKeygenStartedCounter, pcmn.TelemetryConstants.DB.Prefix)

		var args0 string
		var args1 bool
		_ = castOrUnmarshal(args[0], &args0)
		_ = castOrUnmarshal(args[1], &args1)

		d.dbInstance.SetKeygenStarted(args0, args1)
		return nil, nil
	}

	return nil, fmt.Errorf("database service method %v not found", method)
}
func (d *DatabaseService) SetBaseService(bs *BaseService) {
	d.bs = bs
}
