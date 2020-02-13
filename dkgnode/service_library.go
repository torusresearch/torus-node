package dkgnode

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/avast/retry-go"
	ethCommon "github.com/ethereum/go-ethereum/common"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"
	logging "github.com/sirupsen/logrus"
	"github.com/tendermint/go-amino"
	"github.com/torusresearch/bijson"
	"github.com/torusresearch/tendermint/crypto/ed25519"
	cryptoAmino "github.com/torusresearch/tendermint/crypto/encoding/amino"
	tmp2p "github.com/torusresearch/tendermint/p2p"
	ctypes "github.com/torusresearch/tendermint/rpc/core/types"
	"github.com/torusresearch/torus-common/common"
	pcmn "github.com/torusresearch/torus-node/common"
	"github.com/torusresearch/torus-node/eventbus"
	"github.com/torusresearch/torus-node/keygennofsm"
	"github.com/torusresearch/torus-node/mapping"
	"github.com/torusresearch/torus-node/pss"
)

// ServiceLibrary - thin wrapper functionality around the event bus
// to facilitate method calls to other services

type ServiceLibrary interface {
	SetOwner(owner string)
	GetOwner() (owner string)
	GetEventBus() eventbus.Bus
	SetEventBus(eventbus.Bus)
	TelemetryMethods() TelemetryMethods
	EthereumMethods() EthereumMethods
	ABCIMethods() ABCIMethods
	TendermintMethods() TendermintMethods
	ServerMethods() ServerMethods
	P2PMethods() P2PMethods
	KeygennofsmMethods() KeygennofsmMethods
	PSSMethods() PSSMethods
	MappingMethods() MappingMethods
	DatabaseMethods() DatabaseMethods
	VerifierMethods() VerifierMethods
	CacheMethods() CacheMethods
}

type ServiceLibraryImpl struct {
	eventBus eventbus.Bus
	owner    string
}

func NewServiceLibrary(eventBus eventbus.Bus, owner string) *ServiceLibraryImpl {
	return &ServiceLibraryImpl{eventBus: eventBus, owner: owner}
}
func (sL *ServiceLibraryImpl) SetOwner(owner string) {
	sL.owner = owner
}
func (sL *ServiceLibraryImpl) GetOwner() (owner string) {
	return sL.owner
}
func (sL *ServiceLibraryImpl) GetEventBus() eventbus.Bus {
	return sL.eventBus
}
func (sL *ServiceLibraryImpl) SetEventBus(eventBus eventbus.Bus) {
	sL.eventBus = eventBus
}
func (sL *ServiceLibraryImpl) TelemetryMethods() TelemetryMethods {
	return &TelemetryMethodsImpl{eventBus: sL.GetEventBus(), owner: sL.owner}
}
func (sL *ServiceLibraryImpl) EthereumMethods() EthereumMethods {
	return &EthereumMethodsImpl{eventBus: sL.GetEventBus(), owner: sL.owner}
}
func (sL *ServiceLibraryImpl) ABCIMethods() ABCIMethods {
	return &ABCIMethodsImpl{eventBus: sL.GetEventBus(), owner: sL.owner}
}
func (sL *ServiceLibraryImpl) TendermintMethods() TendermintMethods {
	return &TendermintMethodsImpl{eventBus: sL.GetEventBus(), owner: sL.owner}
}
func (sL *ServiceLibraryImpl) ServerMethods() ServerMethods {
	return &ServerMethodsImpl{eventBus: sL.GetEventBus(), owner: sL.owner}
}
func (sL *ServiceLibraryImpl) P2PMethods() P2PMethods {
	return &P2PMethodsImpl{eventBus: sL.GetEventBus(), owner: sL.owner}
}
func (sL *ServiceLibraryImpl) KeygennofsmMethods() KeygennofsmMethods {
	return &KeygennofsmMethodsImpl{eventBus: sL.GetEventBus(), owner: sL.owner}
}
func (sL *ServiceLibraryImpl) PSSMethods() PSSMethods {
	return &PSSMethodsImpl{eventBus: sL.GetEventBus(), owner: sL.owner}
}
func (sL *ServiceLibraryImpl) MappingMethods() MappingMethods {
	return &MappingMethodsImpl{eventBus: sL.GetEventBus(), owner: sL.owner}
}
func (sL *ServiceLibraryImpl) DatabaseMethods() DatabaseMethods {
	return &DatabaseMethodsImpl{eventBus: sL.GetEventBus(), owner: sL.owner}
}
func (sL *ServiceLibraryImpl) VerifierMethods() VerifierMethods {
	return &VerifierMethodsImpl{eventBus: sL.GetEventBus(), owner: sL.owner}
}
func (sL *ServiceLibraryImpl) CacheMethods() CacheMethods {
	return &CacheMethodsImpl{eventBus: sL.GetEventBus(), owner: sL.owner}
}

type TelemetryMethods interface {
	SetOwner(owner string)
	GetOwner() (owner string)
}
type TelemetryMethodsImpl struct {
	owner    string
	eventBus eventbus.Bus
}

func (m *TelemetryMethodsImpl) GetOwner() (owner string) {
	return m.owner
}
func (m *TelemetryMethodsImpl) SetOwner(owner string) {
	m.owner = owner
}

type EthereumMethods interface {
	SetOwner(owner string)
	GetOwner() (owner string)

	GetCurrentEpoch() (epoch int)
	GetPreviousEpoch() (epoch int, err error)
	GetNextEpoch() (epoch int, err error)
	GetEpochInfo(epoch int, skipCache bool) (epochInfo epochInfo, err error)
	GetSelfIndex() (index int)
	GetSelfPrivateKey() (privKey big.Int)
	GetSelfPublicKey() (pubKey common.Point)
	GetSelfAddress() (address ethCommon.Address)
	SetSelfIndex(int)
	SelfSignData(data []byte) (rawSig []byte)

	AwaitCompleteNodeList(epoch int) []NodeReference
	GetNodeList(epoch int) []NodeReference
	GetNodeDetailsByAddress(address ethCommon.Address) NodeReference
	GetNodeDetailsByEpochAndIndex(epoch int, index int) NodeReference
	AwaitNodesConnected(epoch int)
	GetPSSStatus(oldEpoch int, newEpoch int) (pssStatus int, err error)
	VerifyDataWithNodelist(pk common.Point, sig []byte, data []byte) (senderDetails NodeDetails, err error)
	VerifyDataWithEpoch(pk common.Point, sig []byte, data []byte, epoch int) (senderDetails NodeDetails, err error)
	StartPSSMonitor() error
	GetTMP2PConnection() string
	GetP2PConnection() string
	ValidateEpochPubKey(nodeAddress ethCommon.Address, pubK common.Point) (valid bool)
}
type EthereumMethodsImpl struct {
	owner    string
	eventBus eventbus.Bus
}

func (m *EthereumMethodsImpl) GetOwner() (owner string) {
	return m.owner
}
func (m *EthereumMethodsImpl) SetOwner(owner string) {
	m.owner = owner
}
func (e *EthereumMethodsImpl) GetCurrentEpoch() (epoch int) {
	err := retry.Do(func() error {
		methodResponse := ServiceMethod(e.eventBus, e.owner, "ethereum", "get_current_epoch")
		if methodResponse.Error != nil {
			return methodResponse.Error
		}
		var data int
		err := castOrUnmarshal(methodResponse.Data, &data)
		if err != nil {
			return err
		}
		if data == 0 {
			return errors.New("could not get current epoch")
		}
		epoch = data
		return nil
	})
	if err != nil || epoch == 0 {
		logging.WithError(err).Fatal("could not get current epoch")
	}
	return
}
func (e *EthereumMethodsImpl) GetPreviousEpoch() (epoch int, err error) {
	methodResponse := ServiceMethod(e.eventBus, e.owner, "ethereum", "get_previous_epoch", nil)
	if methodResponse.Error != nil {
		return 0, methodResponse.Error
	}
	var data int
	err = castOrUnmarshal(methodResponse.Data, &data)
	if err != nil {
		logging.WithError(err).Error("could not castOrUnmarshal data")
		return
	}
	epoch = data
	return
}
func (e *EthereumMethodsImpl) GetNextEpoch() (epoch int, err error) {
	methodResponse := ServiceMethod(e.eventBus, e.owner, "ethereum", "get_next_epoch")
	if methodResponse.Error != nil {
		return 0, methodResponse.Error
	}
	var data int
	err = castOrUnmarshal(methodResponse.Data, &data)
	if err != nil {
		logging.WithError(err).Error("could not castOrUnmarshal data")
		return
	}
	epoch = data
	return
}
func (e *EthereumMethodsImpl) GetEpochInfo(epoch int, skipCache bool) (eInfo epochInfo, err error) {
	methodResponse := ServiceMethod(e.eventBus, e.owner, "ethereum", "get_epoch_info", epoch, skipCache)
	if methodResponse.Error != nil {
		return eInfo, methodResponse.Error
	}
	var data epochInfo
	err = castOrUnmarshal(methodResponse.Data, &data)
	if err != nil {
		logging.WithError(err).Error("could not castOrUnmarshal data")
		return
	}
	if data.Id.Cmp(big.NewInt(0)) == 0 {
		return eInfo, errors.New("data is invalid, epochID is 0")
	}
	eInfo = data
	return
}
func (e *EthereumMethodsImpl) GetSelfIndex() (index int) {
	methodResponse := ServiceMethod(e.eventBus, e.owner, "ethereum", "get_self_index")
	if methodResponse.Error != nil {
		logging.Fatalf("Get self index returned error, should be blocking until success")
	}
	var data int
	err := castOrUnmarshal(methodResponse.Data, &data)
	if err != nil {
		logging.WithError(err).Error("could not castOrUnmarshal data")
	}
	index = data
	return
}
func (e *EthereumMethodsImpl) GetSelfPrivateKey() (privKey big.Int) {
	err := retry.Do(func() error {
		methodResponse := ServiceMethod(e.eventBus, e.owner, "ethereum", "get_self_private_key")
		if methodResponse.Error != nil {
			return methodResponse.Error
		}
		var data big.Int
		err := castOrUnmarshal(methodResponse.Data, &data)
		if err != nil {
			return err
		}
		privKey = data
		return nil
	})
	if err != nil {
		logging.WithError(err).Fatal("could not get self private key")
	}
	return
}
func (e *EthereumMethodsImpl) GetSelfPublicKey() (pubKey common.Point) {
	err := retry.Do(func() error {
		methodResponse := ServiceMethod(e.eventBus, e.owner, "ethereum", "get_self_public_key")
		if methodResponse.Error != nil {
			return methodResponse.Error
		}
		var data common.Point
		err := castOrUnmarshal(methodResponse.Data, &data)
		if err != nil {
			return err
		}
		pubKey = data
		return nil
	})
	if err != nil {
		logging.WithError(err).Fatal("could not get self public key")
	}
	return
}
func (e *EthereumMethodsImpl) GetSelfAddress() (address ethCommon.Address) {
	err := retry.Do(func() error {
		methodResponse := ServiceMethod(e.eventBus, e.owner, "ethereum", "get_self_address")
		if methodResponse.Error != nil {
			return methodResponse.Error
		}
		var data ethCommon.Address
		err := castOrUnmarshal(methodResponse.Data, &data)
		if err != nil {
			return err
		}
		address = data
		return nil
	})
	if err != nil {
		logging.WithError(err).Fatal("could not get self address")
	}
	return
}
func (e *EthereumMethodsImpl) SetSelfIndex(index int) {
	err := retry.Do(func() error {
		methodResponse := ServiceMethod(e.eventBus, e.owner, "ethereum", "set_self_index", index)
		if methodResponse.Error != nil {
			return methodResponse.Error
		}
		return nil
	})
	if err != nil {
		logging.WithError(err).Fatal("could not set self index")
	}
}
func (e *EthereumMethodsImpl) SelfSignData(input []byte) (rawSig []byte) {
	err := retry.Do(func() error {
		methodResponse := ServiceMethod(e.eventBus, e.owner, "ethereum", "self_sign_data", input)
		if methodResponse.Error != nil {
			return methodResponse.Error
		}
		var data []byte
		err := castOrUnmarshal(methodResponse.Data, &data)
		if err != nil {
			return err
		}
		rawSig = data
		return nil
	})
	if err != nil {
		logging.Fatalf("Could not set self sign data, %v", err.Error())
	}
	return
}
func (e *EthereumMethodsImpl) AwaitCompleteNodeList(epoch int) (nodeRefs []NodeReference) {
	err := retry.Do(func() error {
		methodResponse := ServiceMethod(e.eventBus, e.owner, "ethereum", "await_complete_node_list", epoch)
		if methodResponse.Error != nil {
			return methodResponse.Error
		}
		var data []SerializedNodeReference
		err := castOrUnmarshal(methodResponse.Data, &data)
		if err != nil {
			return err
		}
		var deserializedData []NodeReference
		for i := 0; i < len(data); i++ {
			deserializedData = append(deserializedData, NodeReference{}.Deserialize(data[i]))
		}
		nodeRefs = deserializedData
		return nil
	})
	if err != nil {
		logging.WithError(err).Fatal("could not get complete node list")
	}
	return
}
func (e *EthereumMethodsImpl) GetNodeList(epoch int) (nodeRefs []NodeReference) {
	err := retry.Do(func() error {
		methodResponse := ServiceMethod(e.eventBus, e.owner, "ethereum", "get_node_list", epoch)
		if methodResponse.Error != nil {
			return methodResponse.Error
		}
		var data []SerializedNodeReference
		err := castOrUnmarshal(methodResponse.Data, &data)
		if err != nil {
			return err
		}
		var deserializedData []NodeReference
		for i := 0; i < len(data); i++ {
			deserializedData = append(deserializedData, NodeReference{}.Deserialize(data[i]))
		}
		nodeRefs = deserializedData
		return nil
	})
	if err != nil {
		logging.WithError(err).Fatal("could not get node list")
	}
	return
}
func (e *EthereumMethodsImpl) GetNodeDetailsByAddress(address ethCommon.Address) (nodeRef NodeReference) {
	err := retry.Do(func() error {
		methodResponse := ServiceMethod(e.eventBus, e.owner, "ethereum", "get_node_details_by_address", address)
		if methodResponse.Error != nil {
			return methodResponse.Error
		}
		var data SerializedNodeReference
		err := castOrUnmarshal(methodResponse.Data, &data)
		if err != nil {
			return err
		}
		nodeRef = NodeReference{}.Deserialize(data)
		return nil
	})
	if err != nil {
		logging.WithError(err).Fatal("could not get node details by address")
	}
	return
}
func (e *EthereumMethodsImpl) GetNodeDetailsByEpochAndIndex(epoch int, index int) (nodeRef NodeReference) {
	err := retry.Do(func() error {
		methodResponse := ServiceMethod(e.eventBus, e.owner, "ethereum", "get_node_details_by_epoch_and_index", epoch, index)
		if methodResponse.Error != nil {
			return methodResponse.Error
		}
		var data SerializedNodeReference
		err := castOrUnmarshal(methodResponse.Data, &data)
		if err != nil {
			return err
		}
		nodeRef = NodeReference{}.Deserialize(data)
		return nil
	})
	if err != nil {
		logging.WithError(err).Fatal("could not get node details by epoch and index")
	}
	return
}
func (e *EthereumMethodsImpl) AwaitNodesConnected(epoch int) {
	methodResponse := ServiceMethod(e.eventBus, e.owner, "ethereum", "await_nodes_connected", epoch)
	if methodResponse.Error != nil {
		logging.WithError(methodResponse.Error).Fatal("await nodes connected returned error")
	}
}
func (e *EthereumMethodsImpl) GetPSSStatus(oldEpoch int, newEpoch int) (pssStatus int, err error) {
	methodResponse := ServiceMethod(e.eventBus, e.owner, "ethereum", "get_PSS_status", oldEpoch, newEpoch)
	if methodResponse.Error != nil {
		return pssStatus, methodResponse.Error
	}
	var data int
	err = castOrUnmarshal(methodResponse.Data, &data)
	if err != nil {
		return pssStatus, err
	}
	return data, nil
}

func (e *EthereumMethodsImpl) VerifyDataWithNodelist(pk common.Point, sig []byte, input []byte) (senderDetails NodeDetails, err error) {
	methodResponse := ServiceMethod(e.eventBus, e.owner, "ethereum", "verify_data_with_nodelist", pk, sig, input)
	var data NodeDetails
	err = castOrUnmarshal(methodResponse.Data, &data)
	if err != nil {
		return senderDetails, err
	}
	return data, nil
}
func (e *EthereumMethodsImpl) VerifyDataWithEpoch(pk common.Point, sig []byte, input []byte, epoch int) (senderDetails NodeDetails, err error) {
	methodResponse := ServiceMethod(e.eventBus, e.owner, "ethereum", "verify_data_with_epoch", pk, sig, input, epoch)
	var data NodeDetails
	err = castOrUnmarshal(methodResponse.Data, &data)
	if err != nil {
		return senderDetails, err
	}
	return data, nil
}
func (e *EthereumMethodsImpl) StartPSSMonitor() error {
	methodResponse := ServiceMethod(e.eventBus, e.owner, "ethereum", "start_PSS_monitor")
	if methodResponse.Error != nil {
		return methodResponse.Error
	}
	return nil
}
func (e *EthereumMethodsImpl) GetTMP2PConnection() (tmp2pconnection string) {
	err := retry.Do(func() error {
		methodResponse := ServiceMethod(e.eventBus, e.owner, "ethereum", "get_tm_p2p_connection")
		if methodResponse.Error != nil {
			return methodResponse.Error
		}
		var data string
		err := castOrUnmarshal(methodResponse.Data, &data)
		if err != nil {
			return err
		}
		tmp2pconnection = data
		return nil
	})
	if err != nil {
		logging.WithError(err).Fatal("could not get node details by epoch and index")
	}
	return
}
func (e *EthereumMethodsImpl) GetP2PConnection() (p2pconnection string) {
	err := retry.Do(func() error {
		methodResponse := ServiceMethod(e.eventBus, e.owner, "ethereum", "get_p2p_connection")
		if methodResponse.Error != nil {
			return methodResponse.Error
		}
		var data string
		err := castOrUnmarshal(methodResponse.Data, &data)
		if err != nil {
			return err
		}
		p2pconnection = data
		return nil
	})
	if err != nil {
		logging.WithError(err).Fatal("could not get node details by epoch and index")
	}
	return
}

func (e *EthereumMethodsImpl) ValidateEpochPubKey(nodeAddress ethCommon.Address, pubK common.Point) (valid bool) {
	methodResponse := ServiceMethod(e.eventBus, e.owner, "ethereum", "validate_epoch_pub_key", nodeAddress, pubK)
	if methodResponse.Error != nil {
		return false
	}
	var data bool
	err := castOrUnmarshal(methodResponse.Data, &data)
	if err != nil {
		return false
	}
	return data
}

type ABCIMethods interface {
	SetOwner(owner string)
	GetOwner() (owner string)

	RetrieveKeyMapping(keyIndex big.Int) (keyDetails KeyAssignmentPublic, err error)
	GetIndexesFromVerifierID(verifier, veriferID string) (keyIndexes []big.Int, err error)
	GetVerifierIterator() (iterator *mapping.VerifierIterator, err error)
}

type ABCIMethodsImpl struct {
	owner    string
	eventBus eventbus.Bus
}

func (a *ABCIMethodsImpl) GetVerifierIterator() (iterator *mapping.VerifierIterator, err error) {
	methodResponse := ServiceMethod(a.eventBus, a.owner, "abci", "get_verifier_iterator")
	if methodResponse.Error != nil {
		return nil, methodResponse.Error
	}
	var data string
	err = castOrUnmarshal(methodResponse.Data, &data)
	if err != nil {
		return nil, err
	}
	randomID := data
	handler := func(inter interface{}) {
		var data MethodResponse
		err := castOrUnmarshal(inter, &data)
		if err != nil {
			iterator.Err = err
		} else {
			iterator.Val = methodResponse.Data
			iterator.Err = methodResponse.Error
		}
	}
	iterator = &mapping.VerifierIterator{
		Iterator: pcmn.Iterator{
			RandomID: randomID,
			CallNext: func() bool {
				methodResponse := ServiceMethod(a.eventBus, a.owner, "abci", "get_verifier_iterator_next", randomID)
				if methodResponse.Error != nil {
					logging.WithError(methodResponse.Error).Error("Could not get next iterator")
					return false
				}
				var responseStruct pcmn.VerifierData
				err := castOrUnmarshal(methodResponse.Data, &responseStruct)
				if err != nil {
					logging.WithError(err).WithField("methodResponse", methodResponse).Error("could not castOrUnmarshal")
					return false
				}
				if !responseStruct.Ok {
					err := a.eventBus.Unsubscribe("iterator_value:"+randomID, handler)
					if err != nil {
						logging.WithError(err).Error("could not unsubscribe from event bus")
						return false
					}
				}
				iterator.Val = responseStruct
				iterator.Err = responseStruct.Err
				return responseStruct.Ok
			},
		}}
	err = a.eventBus.SubscribeAsync("iterator_value:"+randomID, handler, false)
	if err != nil {
		return nil, err
	}
	return iterator, nil
}

func (a *ABCIMethodsImpl) GetOwner() (owner string) {
	return a.owner
}
func (a *ABCIMethodsImpl) SetOwner(owner string) {
	a.owner = owner
}
func (a *ABCIMethodsImpl) RetrieveKeyMapping(keyIndex big.Int) (keyDetails KeyAssignmentPublic, err error) {
	methodResponse := ServiceMethod(a.eventBus, a.owner, "abci", "retrieve_key_mapping", keyIndex)
	if methodResponse.Error != nil {
		return keyDetails, methodResponse.Error
	}
	var data KeyAssignmentPublic
	err = castOrUnmarshal(methodResponse.Data, &data)
	if err != nil {
		return keyDetails, err
	}
	keyDetails = data
	return
}
func (a *ABCIMethodsImpl) GetIndexesFromVerifierID(verifier, verifierID string) (keyIndexes []big.Int, err error) {
	methodResponse := ServiceMethod(a.eventBus, a.owner, "abci", "get_indexes_from_verifier_id", verifier, verifierID)
	if methodResponse.Error != nil {
		return keyIndexes, methodResponse.Error
	}
	var data []big.Int
	err = castOrUnmarshal(methodResponse.Data, &data)
	if err != nil {
		return keyIndexes, err
	}
	keyIndexes = data
	return
}

type TendermintMethods interface {
	SetOwner(owner string)
	GetOwner() (owner string)

	GetNodeKey() (nodeKey tmp2p.NodeKey)
	GetStatus() (status BFTRPCWSStatus)

	Broadcast(tx interface{}) (txHash pcmn.Hash, err error)
	RegisterQuery(query string, count int) (respChannel chan []byte, err error)
	DeregisterQuery(query string) (err error)
}
type TendermintMethodsImpl struct {
	owner    string
	eventBus eventbus.Bus
}

func (m *TendermintMethodsImpl) GetOwner() (owner string) {
	return m.owner
}
func (m *TendermintMethodsImpl) SetOwner(owner string) {
	m.owner = owner
}
func (t *TendermintMethodsImpl) GetNodeKey() (nodeKey tmp2p.NodeKey) {
	err := retry.Do(func() error {
		methodResponse := ServiceMethod(t.eventBus, t.owner, "tendermint", "get_node_key")
		if methodResponse.Error != nil {
			return methodResponse.Error
		}

		var data []byte
		err := castOrUnmarshal(methodResponse.Data, &data)
		if err != nil {
			return err
		}
		cdc := amino.NewCodec()
		cryptoAmino.RegisterAmino(cdc)
		newKey := &tmp2p.NodeKey{
			PrivKey: ed25519.PrivKeyEd25519{},
		}
		err = cdc.UnmarshalJSON(data, newKey)
		if err != nil {
			return err
		}
		nodeKey = *newKey
		return nil
	})
	if err != nil {
		logging.WithError(err).Fatal("could not get nodeKey")
	}
	return
}

func (t *TendermintMethodsImpl) GetStatus() (status BFTRPCWSStatus) {
	err := retry.Do(func() error {
		methodResponse := ServiceMethod(t.eventBus, t.owner, "tendermint", "get_status")
		if methodResponse.Error != nil {
			return methodResponse.Error
		}
		var data BFTRPCWSStatus
		err := castOrUnmarshal(methodResponse.Data, &data)
		if err != nil {
			return err
		}
		status = data
		return nil
	})
	if err != nil {
		logging.WithError(err).Fatal("could not get tendermint status")
	}
	return
}

func (t *TendermintMethodsImpl) Broadcast(tx interface{}) (txHash pcmn.Hash, err error) {
	methodResponse := ServiceMethod(t.eventBus, t.owner, "tendermint", "broadcast", tx)
	if methodResponse.Error != nil {
		return txHash, methodResponse.Error
	}
	var data pcmn.Hash
	err = castOrUnmarshal(methodResponse.Data, &data)
	if err != nil {
		return txHash, err
	}
	txHash = data
	return
}

type TendermintMethodResponse struct {
	Error     error
	ByteSlice []byte
}

func (t *TendermintMethodsImpl) RegisterQuery(query string, count int) (respChannel chan []byte, err error) {
	respChannel = make(chan []byte)
	eventBus := t.eventBus
	if eventBus.HasCallback("tendermint:forward:" + query) {
		err = fmt.Errorf("Cannot call RegisterQuery on query %v as it already has a handler", query)
		return
	}
	numOfRes := 0
	handler := func(inter interface{}) {
		numOfRes++
		if numOfRes == count {
			go func() {
				err := t.DeregisterQuery(query)
				if err != nil {
					logging.WithField("query", query).WithError(err).Error("could not deregister")
				}
			}()
		} else if numOfRes > count {
			return
		}
		var methodResponse MethodResponse
		err := castOrUnmarshal(inter, &methodResponse)
		if err != nil {
			logging.WithError(err).WithField("methodResponse", methodResponse).Error("could not castOrUnmarshal")
			return
		}
		if methodResponse.Error != nil {
			logging.WithError(methodResponse.Error).Error("could not query tendermint, got error")
			return
		}
		var data []byte
		err = castOrUnmarshal(methodResponse.Data, &data)
		if err != nil {
			logging.WithError(err).WithField("Data", methodResponse.Data).Error("could not castOrUnmarshal")
		}
		respChannel <- data
	}
	err = eventBus.SubscribeAsync("tendermint:forward:"+query, handler, false)
	methodResponse := ServiceMethod(eventBus, t.owner, "tendermint", "register_query", query, count)
	if methodResponse.Error != nil {
		err = methodResponse.Error
	}
	return
}
func (t *TendermintMethodsImpl) DeregisterQuery(query string) error {
	err := t.eventBus.UnsubscribeAll("tendermint:forward:" + query)
	if err != nil {
		return err
	}
	methodResponse := ServiceMethod(t.eventBus, t.owner, "tendermint", "deregister_query", query)
	if methodResponse.Error != nil {
		return methodResponse.Error
	}
	return nil
}
func (t *TendermintMethodsImpl) ABCIQuery(path string, data []byte) (*ctypes.ResultABCIQuery, error) {
	return nil, nil
}

type ServerMethods interface {
	SetOwner(owner string)
	GetOwner() (owner string)
	RequestConnectionDetails(endpoint string) (connectionDetails ConnectionDetails, err error)
}
type ServerMethodsImpl struct {
	owner    string
	eventBus eventbus.Bus
}

func (s *ServerMethodsImpl) GetOwner() (owner string) {
	return s.owner
}
func (s *ServerMethodsImpl) SetOwner(owner string) {
	s.owner = owner
}
func (s *ServerMethodsImpl) RequestConnectionDetails(endpoint string) (connectionDetails ConnectionDetails, err error) {
	methodResponse := ServiceMethod(s.eventBus, s.owner, "server", "request_connection_details", endpoint)
	if methodResponse.Error != nil {
		return connectionDetails, methodResponse.Error
	}
	var data ConnectionDetails
	err = castOrUnmarshal(methodResponse.Data, &data)
	if err != nil {
		return connectionDetails, err
	}
	connectionDetails = data
	return
}

type StreamMessage struct {
	Protocol string
	Message  P2PBasicMsg
}

type P2PMethods interface {
	SetOwner(owner string)
	GetOwner() (owner string)

	ID() peer.ID
	SetStreamHandler(protoName string, handler func(StreamMessage)) (err error)
	RemoveStreamHandler(protoName string) (err error)
	AuthenticateMessage(p2pBasicMsg P2PBasicMsg) (err error)
	AuthenticateMessageInEpoch(p2pBasicMsg P2PBasicMsg, epoch int) (err error)
	NewP2PMessage(messageId string, gossip bool, payload []byte, msgType string) (newMsg P2PBasicMsg)
	SignP2PMessage(message P2PMessage) (signature []byte, err error)
	SendP2PMessage(id peer.ID, p protocol.ID, msg P2PMessage) error
	ConnectToP2PNode(nodeP2PConnection string, nodePeerID peer.ID) error
	GetHostAddress() (hostAddress string)
}
type P2PMethodsImpl struct {
	owner    string
	eventBus eventbus.Bus
}

func (m *P2PMethodsImpl) GetOwner() (owner string) {
	return m.owner
}
func (m *P2PMethodsImpl) SetOwner(owner string) {
	m.owner = owner
}
func (p2p *P2PMethodsImpl) ID() (peerID peer.ID) {
	err := retry.Do(func() error {
		methodResponse := ServiceMethod(p2p.eventBus, p2p.owner, "p2p", "id")
		if methodResponse.Error != nil {
			return methodResponse.Error
		}
		var data peer.ID
		err := castOrUnmarshal(methodResponse.Data, &data)
		if err != nil {
			return fmt.Errorf("could not castOrUnmarshal %v", err.Error())
		}
		peerID = data
		return nil
	})
	if err != nil {
		logging.WithError(err).Fatal("could not get id")
	}
	return
}

func (p2p *P2PMethodsImpl) SetStreamHandler(proto string, handler func(StreamMessage)) error {
	eventBus := p2p.eventBus
	if eventBus.HasCallback("p2p:forward:" + proto) {
		return fmt.Errorf("Cannot call setStreamHandler on proto %v as it already has a handler", proto)
	}
	err := eventBus.SubscribeAsync("p2p:forward:"+proto, func(inter interface{}) {
		var data P2PBasicMsg
		err := castOrUnmarshal(inter, &data)
		if err != nil {
			logging.WithField("inter", inter).Error("could not castOrUnmarshal")
		}
		logging.WithFields(logging.Fields{
			"forward":     proto,
			"p2pBasicMsg": stringify(data),
		}).Debug("received p2p")
		handler(StreamMessage{
			Protocol: proto,
			Message:  data,
		})
	}, false)
	if err != nil {
		return err
	}
	methodResponse := ServiceMethod(eventBus, p2p.owner, "p2p", "set_stream_handler", proto)
	if methodResponse.Error != nil {
		return methodResponse.Error
	}
	return nil
}

func (p2p *P2PMethodsImpl) RemoveStreamHandler(proto string) error {
	eventBus := p2p.eventBus
	err := eventBus.UnsubscribeAll("p2p:forward:" + proto)
	if err != nil {
		return err
	}
	methodResponse := ServiceMethod(eventBus, p2p.owner, "p2p", "remove_stream_handler", proto)
	if methodResponse.Error != nil {
		return methodResponse.Error
	}
	return nil
}

func (p2p *P2PMethodsImpl) AuthenticateMessage(p2pBasicMsg P2PBasicMsg) (err error) {
	eventBus := p2p.eventBus
	methodResponse := ServiceMethod(eventBus, p2p.owner, "p2p", "authenticate_message", p2pBasicMsg)
	return methodResponse.Error
}
func (p2p *P2PMethodsImpl) AuthenticateMessageInEpoch(p2pBasicMsg P2PBasicMsg, epoch int) (err error) {
	eventBus := p2p.eventBus
	methodResponse := ServiceMethod(eventBus, p2p.owner, "p2p", "authenticate_message_in_epoch", p2pBasicMsg, epoch)
	return methodResponse.Error
}
func (p2p *P2PMethodsImpl) NewP2PMessage(messageId string, gossip bool, payload []byte, msgType string) (newMsg P2PBasicMsg) {
	eventBus := p2p.eventBus
	methodResponse := ServiceMethod(eventBus, p2p.owner, "p2p", "new_p2p_message", messageId, gossip, payload, msgType)
	if methodResponse.Error != nil {
		logging.WithError(methodResponse.Error).Fatal()
	}
	var data P2PBasicMsg
	err := castOrUnmarshal(methodResponse.Data, &data)
	if err != nil {
		logging.WithField("Data", methodResponse.Data).Error("could not castOrUnmarshal")
	}
	newMsg = data
	return
}

func (p2p *P2PMethodsImpl) SignP2PMessage(message P2PMessage) (signature []byte, err error) {
	eventBus := p2p.eventBus
	methodResponse := ServiceMethod(eventBus, p2p.owner, "p2p", "sign_p2p_message", message)
	if methodResponse.Error != nil {
		return signature, methodResponse.Error
	}
	var data []byte
	err = castOrUnmarshal(methodResponse.Data, &data)
	if err != nil {
		return signature, fmt.Errorf("could not castOrUnmarshal data %v %v", methodResponse.Data, err.Error())
	}
	return data, nil
}
func (p2p *P2PMethodsImpl) SendP2PMessage(id peer.ID, p protocol.ID, msg P2PMessage) error {
	eventBus := p2p.eventBus
	methodResponse := ServiceMethod(eventBus, p2p.owner, "p2p", "send_p2p_message", id, p, msg)
	if methodResponse.Error != nil {
		return methodResponse.Error
	}
	return nil
}
func (p2p *P2PMethodsImpl) ConnectToP2PNode(nodeP2PConnection string, nodePeerID peer.ID) error {
	eventBus := p2p.eventBus
	methodResponse := ServiceMethod(eventBus, p2p.owner, "p2p", "connect_to_p2p_node", nodeP2PConnection, nodePeerID)
	if methodResponse.Error != nil {
		return methodResponse.Error
	}
	return nil
}
func (p2p *P2PMethodsImpl) GetHostAddress() (hostAddress string) {
	eventBus := p2p.eventBus
	err := retry.Do(func() error {
		methodResponse := ServiceMethod(eventBus, p2p.owner, "p2p", "get_host_address")
		if methodResponse.Error != nil {
			return methodResponse.Error
		}
		var data string
		err := castOrUnmarshal(methodResponse.Data, &data)
		if err != nil {
			return err
		}
		hostAddress = data
		return nil
	})
	if err != nil {
		logging.WithError(err).Fatal("could not get host address")
	}
	return
}

type KeygennofsmMethods interface {
	SetOwner(owner string)
	GetOwner() (owner string)

	ReceiveMessage(keygenMessage keygennofsm.KeygenMessage) error
	ReceiveBFTMessage(keygenMessage keygennofsm.KeygenMessage) error
}

type KeygennofsmMethodsImpl struct {
	owner    string
	eventBus eventbus.Bus
}

func (k *KeygennofsmMethodsImpl) GetOwner() (owner string) {
	return k.owner
}
func (k *KeygennofsmMethodsImpl) SetOwner(owner string) {
	k.owner = owner
}
func (k *KeygennofsmMethodsImpl) ReceiveMessage(keygenMessage keygennofsm.KeygenMessage) error {
	methodResponse := ServiceMethod(k.eventBus, k.owner, "keygennofsm", "receive_message", keygenMessage)
	if methodResponse.Error != nil {
		return methodResponse.Error
	}
	return nil
}
func (k *KeygennofsmMethodsImpl) ReceiveBFTMessage(keygenMessage keygennofsm.KeygenMessage) error {
	methodResponse := ServiceMethod(k.eventBus, k.owner, "keygennofsm", "receive_BFT_message", keygenMessage)
	if methodResponse.Error != nil {
		return methodResponse.Error
	}
	return nil
}

type PSSMethods interface {
	SetOwner(owner string)
	GetOwner() (owner string)

	PSSInstanceExists(protocolPrefix PSSProtocolPrefix) (exists bool)
	GetPSSProtocolPrefix(oldEpoch int, newEpoch int) (pssProtocolPrefix PSSProtocolPrefix)
	ReceiveBFTMessage(protocolPrefix PSSProtocolPrefix, pssMessage pss.PSSMessage) error
	GetNewNodesN(protocolPrefix PSSProtocolPrefix) (newN int)
	GetNewNodesK(protocolPrefix PSSProtocolPrefix) (newK int)
	GetNewNodesT(protocolPrefix PSSProtocolPrefix) (newT int)
	GetOldNodesN(protocolPrefix PSSProtocolPrefix) (oldN int)
	GetOldNodesK(protocolPrefix PSSProtocolPrefix) (oldK int)
	GetOldNodesT(protocolPrefix PSSProtocolPrefix) (oldT int)
	NewPSSNode(pssStartData PSSStartData, isDealer bool, isPlayer bool) error
	SendPSSMessageToNode(protocolPrefix PSSProtocolPrefix, pssNodeDetails pss.NodeDetails, pssMessage pss.PSSMessage) error
}
type PSSMethodsImpl struct {
	owner    string
	eventBus eventbus.Bus
}

func (pss *PSSMethodsImpl) GetOwner() (owner string) {
	return pss.owner
}
func (pss *PSSMethodsImpl) SetOwner(owner string) {
	pss.owner = owner
}
func (pss *PSSMethodsImpl) PSSInstanceExists(protocolPrefix PSSProtocolPrefix) (exists bool) {
	err := retry.Do(func() error {
		methodResponse := ServiceMethod(pss.eventBus, pss.owner, "pss", "PSS_instance_exists", protocolPrefix)
		if methodResponse.Error != nil {
			return methodResponse.Error
		}
		var data bool
		err := castOrUnmarshal(methodResponse.Data, &data)
		if err != nil {
			return err
		}
		exists = data
		return nil
	})
	if err != nil {
		logging.WithError(err).Fatal("could not check if pss instance exists")
	}
	return
}
func (pss *PSSMethodsImpl) GetPSSProtocolPrefix(oldEpoch int, newEpoch int) (pssProtocolPrefix PSSProtocolPrefix) {
	methodResponse := ServiceMethod(pss.eventBus, pss.owner, "pss", "get_PSS_protocol_prefix", oldEpoch, newEpoch)
	if methodResponse.Error != nil {
		logging.WithError(methodResponse.Error).Fatal("could not get pss protocol prefix")
	}
	var data PSSProtocolPrefix
	err := castOrUnmarshal(methodResponse.Data, &data)
	if err != nil {
		logging.WithError(err).Error("could not castOrUnmarshal")
	}
	return data
}
func (pss *PSSMethodsImpl) ReceiveBFTMessage(protocolPrefix PSSProtocolPrefix, pssMessage pss.PSSMessage) error {
	methodResponse := ServiceMethod(pss.eventBus, pss.owner, "pss", "receive_BFT_message", protocolPrefix, pssMessage)
	if methodResponse.Error != nil {
		return methodResponse.Error
	}
	return nil
}
func (pss *PSSMethodsImpl) GetEndIndex() (endIndex int) {
	methodResponse := ServiceMethod(pss.eventBus, pss.owner, "pss", "get_end_index")
	if methodResponse.Error != nil {
		logging.WithError(methodResponse.Error).Fatal("could not get end index")
	}
	var data int
	err := castOrUnmarshal(methodResponse.Data, &data)
	if err != nil {
		logging.WithError(err).Error("could not castOrUnmarshal")
	}
	return data
}
func (pss *PSSMethodsImpl) GetNewNodesN(protocolPrefix PSSProtocolPrefix) (newN int) {
	err := retry.Do(func() error {
		methodResponse := ServiceMethod(pss.eventBus, pss.owner, "pss", "get_new_nodes_n", protocolPrefix)
		if methodResponse.Error != nil {
			return methodResponse.Error
		}
		var data int
		err := castOrUnmarshal(methodResponse.Data, &data)
		if err != nil {
			return err
		}
		newN = data
		return nil
	})
	if err != nil {
		logging.WithError(err).Fatal("could not get new N")
	}
	return
}
func (pss *PSSMethodsImpl) GetNewNodesK(protocolPrefix PSSProtocolPrefix) (newK int) {
	err := retry.Do(func() error {
		methodResponse := ServiceMethod(pss.eventBus, pss.owner, "pss", "get_new_nodes_k", protocolPrefix)
		if methodResponse.Error != nil {
			return methodResponse.Error
		}
		var data int
		err := castOrUnmarshal(methodResponse.Data, &data)
		if err != nil {
			return err
		}
		newK = data
		return nil
	})
	if err != nil {
		logging.WithError(err).Fatal("could not get new K")
	}
	return
}
func (pss *PSSMethodsImpl) GetNewNodesT(protocolPrefix PSSProtocolPrefix) (newT int) {
	err := retry.Do(func() error {
		methodResponse := ServiceMethod(pss.eventBus, pss.owner, "pss", "get_new_nodes_t", protocolPrefix)
		if methodResponse.Error != nil {
			return methodResponse.Error
		}
		var data int
		err := castOrUnmarshal(methodResponse.Data, &data)
		if err != nil {
			return err
		}
		newT = data
		return nil
	})
	if err != nil {
		logging.WithError(err).Fatal("could not get new T")
	}
	return
}
func (pss *PSSMethodsImpl) GetOldNodesN(protocolPrefix PSSProtocolPrefix) (oldN int) {
	err := retry.Do(func() error {
		methodResponse := ServiceMethod(pss.eventBus, pss.owner, "pss", "get_old_nodes_n", protocolPrefix)
		if methodResponse.Error != nil {
			return methodResponse.Error
		}
		var data int
		err := castOrUnmarshal(methodResponse.Data, &data)
		if err != nil {
			return err
		}
		oldN = data
		return nil
	})
	if err != nil {
		logging.WithError(err).Fatal("could not get old N")
	}
	return
}
func (pss *PSSMethodsImpl) GetOldNodesK(protocolPrefix PSSProtocolPrefix) (oldK int) {
	err := retry.Do(func() error {
		methodResponse := ServiceMethod(pss.eventBus, pss.owner, "pss", "get_old_nodes_k", protocolPrefix)
		if methodResponse.Error != nil {
			return methodResponse.Error
		}
		var data int
		err := castOrUnmarshal(methodResponse.Data, &data)
		if err != nil {
			return err
		}
		oldK = data
		return nil
	})
	if err != nil {
		logging.WithError(err).Fatal("could not get old K")
	}
	return
}
func (pss *PSSMethodsImpl) GetOldNodesT(protocolPrefix PSSProtocolPrefix) (oldT int) {
	err := retry.Do(func() error {
		methodResponse := ServiceMethod(pss.eventBus, pss.owner, "pss", "get_old_nodes_t", protocolPrefix)
		if methodResponse.Error != nil {
			return methodResponse.Error
		}
		var data int
		err := castOrUnmarshal(methodResponse.Data, &data)
		if err != nil {
			return nil
		}
		oldT = data
		return nil
	})
	if err != nil {
		logging.WithError(err).Fatal("could not get old T")
	}
	return
}
func (pss *PSSMethodsImpl) NewPSSNode(pssStartData PSSStartData, isDealer bool, isPlayer bool) error {
	methodResponse := ServiceMethod(pss.eventBus, pss.owner, "pss", "new_PSS_node", pssStartData, isDealer, isPlayer)
	if methodResponse.Error != nil {
		return methodResponse.Error
	}
	return nil
}
func (pss *PSSMethodsImpl) SendPSSMessageToNode(protocolPrefix PSSProtocolPrefix, pssNodeDetails pss.NodeDetails, pssMessage pss.PSSMessage) error {
	methodResponse := ServiceMethod(pss.eventBus, pss.owner, "pss", "send_PSS_message_to_node", protocolPrefix, pssNodeDetails, pssMessage)
	if methodResponse.Error != nil {
		return methodResponse.Error
	}
	return nil
}

type MappingMethods interface {
	NewMappingNode(mappingStartData MappingStartData, isOldNode bool, isNewNode bool) error
	ReceiveBFTMessage(mappingID mapping.MappingID, mappingMessage mapping.MappingMessage) error
	GetMappingID(oldEpoch int, newEpoch int) mapping.MappingID
	MappingInstanceExists(mappingID mapping.MappingID) (exists bool)
	GetFreezeState(mappingID mapping.MappingID) (freezeState int, lastUnassignedIndex uint)
	SetFreezeState(mappingID mapping.MappingID, freezeState int, lastUnassignedIndex uint)
	ProposeFreeze(mappingID mapping.MappingID) error
	MappingSummaryFrozen(mappingID mapping.MappingID, mappingSummaryMessage mapping.MappingSummaryMessage) error
	GetMappingProtocolPrefix(oldEpoch int, newEpoch int) MappingProtocolPrefix
}

type MappingMethodsImpl struct {
	owner    string
	eventBus eventbus.Bus
}

func (m *MappingMethodsImpl) GetOwner() (owner string) {
	return m.owner
}
func (m *MappingMethodsImpl) SetOwner(owner string) {
	m.owner = owner
}
func (m *MappingMethodsImpl) NewMappingNode(mappingStartData MappingStartData, isOldNode bool, isNewNode bool) error {
	methodResponse := ServiceMethod(m.eventBus, m.owner, "mapping", "new_mapping_node", mappingStartData, isOldNode, isNewNode)
	if methodResponse.Error != nil {
		return methodResponse.Error
	}
	return nil
}
func (m *MappingMethodsImpl) ReceiveBFTMessage(mappingID mapping.MappingID, mappingMessage mapping.MappingMessage) error {
	methodResponse := ServiceMethod(m.eventBus, m.owner, "mapping", "receive_BFT_message", mappingID, mappingMessage)
	if methodResponse.Error != nil {
		return methodResponse.Error
	}
	return nil
}

func (m *MappingMethodsImpl) GetMappingID(oldEpoch int, newEpoch int) mapping.MappingID {
	var mappingID mapping.MappingID
	err := retry.Do(func() error {
		methodResponse := ServiceMethod(m.eventBus, m.owner, "mapping", "get_mapping_ID", oldEpoch, newEpoch)
		if methodResponse.Error != nil {
			return methodResponse.Error
		}
		var data mapping.MappingID
		err := castOrUnmarshal(methodResponse.Data, &data)
		if err != nil {
			return err
		}
		mappingID = data
		return nil
	})
	if err != nil {
		logging.Fatal("could not get mappingID")
	}
	return mappingID
}
func (m *MappingMethodsImpl) MappingInstanceExists(mappingID mapping.MappingID) (exists bool) {
	err := retry.Do(func() error {
		methodResponse := ServiceMethod(m.eventBus, m.owner, "mapping", "mapping_instance_exists", mappingID)
		if methodResponse.Error != nil {
			return methodResponse.Error
		}
		var data bool
		err := castOrUnmarshal(methodResponse.Data, &data)
		if err != nil {
			return err
		}
		exists = data
		return nil
	})
	if err != nil {
		logging.WithError(err).Fatal("could not check if pss instance exists")
	}
	return
}
func (m *MappingMethodsImpl) GetFreezeState(mappingID mapping.MappingID) (freezeState int, lastUnassignedIndex uint) {
	methodResponse := ServiceMethod(m.eventBus, m.owner, "mapping", "get_freeze_state", mappingID)
	if methodResponse.Error != nil {
		logging.WithError(methodResponse.Error).Error("could not get freeze state")
		return
	}
	var data FreezeStateData
	err := castOrUnmarshal(methodResponse.Data, &data)
	if err != nil {
		logging.Error("could not case return data to struct type for get freeze state")
		return
	}
	freezeState = data.FreezeState
	lastUnassignedIndex = data.LastUnassignedIndex
	return
}
func (m *MappingMethodsImpl) SetFreezeState(mappingID mapping.MappingID, freezeState int, lastUnassignedIndex uint) {
	methodResponse := ServiceMethod(m.eventBus, m.owner, "mapping", "set_freeze_state", mappingID, freezeState, lastUnassignedIndex)
	if methodResponse.Error != nil {
		logging.WithError(methodResponse.Error).Error("could not set freeze state")
	}
}
func (m *MappingMethodsImpl) ProposeFreeze(mappingID mapping.MappingID) error {
	methodResponse := ServiceMethod(m.eventBus, m.owner, "mapping", "propose_freeze", mappingID)
	return methodResponse.Error
}

func (m *MappingMethodsImpl) MappingSummaryFrozen(mappingID mapping.MappingID, mappingSummaryMessage mapping.MappingSummaryMessage) error {
	methodResponse := ServiceMethod(m.eventBus, m.owner, "mapping", "mapping_summary_frozen", mappingID, mappingSummaryMessage)
	return methodResponse.Error
}
func (m *MappingMethodsImpl) GetMappingProtocolPrefix(oldEpoch int, newEpoch int) (mappingProtocolPrefix MappingProtocolPrefix) {
	err := retry.Do(func() error {
		methodResponse := ServiceMethod(m.eventBus, m.owner, "mapping", "get_mapping_protocol_prefix", oldEpoch, newEpoch)
		if methodResponse.Error != nil {
			return methodResponse.Error
		}
		var data MappingProtocolPrefix
		err := castOrUnmarshal(methodResponse.Data, &data)
		if err != nil {
			return err
		}
		mappingProtocolPrefix = data
		return nil
	})
	if err != nil {
		logging.WithError(err).Fatal("could not check if pss instance exists")
	}
	return
}

type DatabaseMethods interface {
	SetOwner(owner string)
	GetOwner() (owner string)

	StoreKeygenCommitmentMatrix(keyIndex big.Int, c [][]common.Point) error
	StorePSSCommitmentMatrix(keyIndex big.Int, c [][]common.Point) error
	StoreCompletedKeygenShare(keyIndex big.Int, si big.Int, siprime big.Int) error
	StoreCompletedPSSShare(keyIndex big.Int, si big.Int, siprime big.Int) error
	StorePublicKeyToIndex(publicKey common.Point, keyIndex big.Int) error
	RetrieveCommitmentMatrix(keyIndex big.Int) (c [][]common.Point, err error)
	RetrievePublicKeyToIndex(publicKey common.Point) (keyIndex big.Int, err error)
	RetrieveIndexToPublicKey(keyIndex big.Int) (publicKey common.Point, err error)
	IndexToPublicKeyExists(keyIndex big.Int) bool
	RetrieveCompletedShare(keyIndex big.Int) (Si big.Int, Siprime big.Int, err error)
	GetShareCount() (count int)
	GetKeygenStarted(keygenID string) (started bool)
	SetKeygenStarted(keygenID string, started bool) error
	StoreConnectionDetails(nodeAddress ethCommon.Address, connectionDetails ConnectionDetails) error
	RetrieveConnectionDetails(nodeAddress ethCommon.Address) (connectionDetails ConnectionDetails, err error)
	StoreNodePubKey(nodeAddress ethCommon.Address, pubKey common.Point) error
	RetrieveNodePubKey(nodeAddress ethCommon.Address) (pubKey common.Point, err error)
}
type DatabaseMethodsImpl struct {
	owner    string
	eventBus eventbus.Bus
}

func (m *DatabaseMethodsImpl) GetOwner() (owner string) {
	return m.owner
}
func (m *DatabaseMethodsImpl) SetOwner(owner string) {
	m.owner = owner
}

func (db *DatabaseMethodsImpl) StoreNodePubKey(nodeAddress ethCommon.Address, pubKey common.Point) error {
	methodResponse := ServiceMethod(db.eventBus, db.owner, "database", "store_node_pub_key", nodeAddress, pubKey)
	if methodResponse.Error != nil {
		return methodResponse.Error
	}
	return nil
}
func (db *DatabaseMethodsImpl) RetrieveNodePubKey(nodeAddress ethCommon.Address) (pubKey common.Point, err error) {
	methodResponse := ServiceMethod(db.eventBus, db.owner, "database", "retrieve_node_pub_key", nodeAddress)
	if methodResponse.Error != nil {
		return pubKey, nil
	}
	var data common.Point
	err = castOrUnmarshal(methodResponse.Data, &data)
	if err != nil {
		return pubKey, err
	}
	return data, nil
}
func (db *DatabaseMethodsImpl) StoreConnectionDetails(nodeAddress ethCommon.Address, connectionDetails ConnectionDetails) error {
	methodResponse := ServiceMethod(db.eventBus, db.owner, "database", "store_connection_details", nodeAddress, connectionDetails)
	if methodResponse.Error != nil {
		return methodResponse.Error
	}
	return nil
}
func (db *DatabaseMethodsImpl) RetrieveConnectionDetails(nodeAddress ethCommon.Address) (connectionDetails ConnectionDetails, err error) {
	methodResponse := ServiceMethod(db.eventBus, db.owner, "database", "retrieve_connection_details", nodeAddress)
	if methodResponse.Error != nil {
		return connectionDetails, methodResponse.Error
	}
	var data ConnectionDetails
	err = castOrUnmarshal(methodResponse.Data, &data)
	if err != nil {
		return connectionDetails, err
	}
	return data, nil
}
func (db *DatabaseMethodsImpl) StoreKeygenCommitmentMatrix(keyIndex big.Int, c [][]common.Point) error {
	methodResponse := ServiceMethod(db.eventBus, db.owner, "database", "store_keygen_commitment_matrix", keyIndex, c)
	if methodResponse.Error != nil {
		return methodResponse.Error
	}
	return nil
}
func (db *DatabaseMethodsImpl) StorePSSCommitmentMatrix(keyIndex big.Int, c [][]common.Point) error {
	methodResponse := ServiceMethod(db.eventBus, db.owner, "database", "store_PSS_commitment_matrix", keyIndex, c)
	if methodResponse.Error != nil {
		return methodResponse.Error
	}
	return nil
}
func (db *DatabaseMethodsImpl) StoreCompletedKeygenShare(keyIndex big.Int, si big.Int, siprime big.Int) error {
	methodResponse := ServiceMethod(db.eventBus, db.owner, "database", "store_completed_keygen_share", keyIndex, si, siprime)
	if methodResponse.Error != nil {
		return methodResponse.Error
	}
	return nil
}
func (db *DatabaseMethodsImpl) StoreCompletedPSSShare(keyIndex big.Int, si big.Int, siprime big.Int) error {
	methodResponse := ServiceMethod(db.eventBus, db.owner, "database", "store_completed_PSS_share", keyIndex, si, siprime)
	if methodResponse.Error != nil {
		return methodResponse.Error
	}
	return nil
}
func (db *DatabaseMethodsImpl) StorePublicKeyToIndex(publicKey common.Point, keyIndex big.Int) error {
	methodResponse := ServiceMethod(db.eventBus, db.owner, "database", "store_public_key_to_index", publicKey, keyIndex)
	if methodResponse.Error != nil {
		return methodResponse.Error
	}
	return nil
}
func (db *DatabaseMethodsImpl) RetrieveCommitmentMatrix(keyIndex big.Int) (c [][]common.Point, err error) {
	methodResponse := ServiceMethod(db.eventBus, db.owner, "database", "retrieve_commitment_matrix", keyIndex)
	if methodResponse.Error != nil {
		return c, methodResponse.Error
	}
	var data [][]common.Point
	err = castOrUnmarshal(methodResponse.Data, &data)
	if err != nil {
		return c, err
	}
	return data, nil
}
func (db *DatabaseMethodsImpl) RetrievePublicKeyToIndex(publicKey common.Point) (keyIndex big.Int, err error) {
	methodResponse := ServiceMethod(db.eventBus, db.owner, "database", "retrieve_public_key_to_index", publicKey)
	if methodResponse.Error != nil {
		return keyIndex, methodResponse.Error
	}
	var data big.Int
	err = castOrUnmarshal(methodResponse.Data, &data)
	if err != nil {
		return keyIndex, err
	}
	return data, nil
}
func (db *DatabaseMethodsImpl) RetrieveIndexToPublicKey(keyIndex big.Int) (publicKey common.Point, err error) {
	methodResponse := ServiceMethod(db.eventBus, db.owner, "database", "retrieve_index_to_public_key", keyIndex)
	if methodResponse.Error != nil {
		return publicKey, methodResponse.Error
	}
	var data common.Point
	err = castOrUnmarshal(methodResponse.Data, &data)
	if err != nil {
		return publicKey, err
	}
	return data, nil
}

func (db *DatabaseMethodsImpl) IndexToPublicKeyExists(keyIndex big.Int) (exists bool) {
	err := retry.Do(func() error {
		methodResponse := ServiceMethod(db.eventBus, db.owner, "database", "index_to_public_key_exists", keyIndex)
		if methodResponse.Error != nil {
			return methodResponse.Error
		}
		var data bool
		err := castOrUnmarshal(methodResponse.Data, &data)
		if err != nil {
			return err
		}
		exists = data
		return nil
	})
	if err != nil {
		logging.WithError(err).Fatal("could not get index_to_public_key_exists")
	}
	return
}

func (db *DatabaseMethodsImpl) RetrieveCompletedShare(keyIndex big.Int) (Si big.Int, Siprime big.Int, err error) {
	methodResponse := ServiceMethod(db.eventBus, db.owner, "database", "retrieve_completed_share", keyIndex)
	if methodResponse.Error != nil {
		err = methodResponse.Error
		return
	}
	var data struct {
		Si      big.Int
		Siprime big.Int
	}
	err = castOrUnmarshal(methodResponse.Data, &data)
	if err != nil {
		return Si, Siprime, err
	}
	Si = data.Si
	Siprime = data.Siprime
	return
}
func (db *DatabaseMethodsImpl) GetShareCount() (count int) {
	err := retry.Do(func() error {
		methodResponse := ServiceMethod(db.eventBus, db.owner, "database", "get_share_count")
		if methodResponse.Error != nil {
			return methodResponse.Error
		}
		var data int
		err := castOrUnmarshal(methodResponse.Data, &data)
		if err != nil {
			return err
		}
		count = data
		return nil
	})
	if err != nil {
		logging.WithError(err).Fatal("could not get share count")
	}
	return
}

func (db *DatabaseMethodsImpl) GetKeygenStarted(keygenID string) (started bool) {
	err := retry.Do(func() error {
		methodResponse := ServiceMethod(db.eventBus, db.owner, "database", "get_keygen_started", keygenID)
		if methodResponse.Error != nil {
			return methodResponse.Error
		}
		var data bool
		err := castOrUnmarshal(methodResponse.Data, &data)
		if err != nil {
			return err
		}
		started = data
		return nil
	})
	if err != nil {
		logging.WithError(err).Fatal("could not get keygen started")
	}
	return
}
func (db *DatabaseMethodsImpl) SetKeygenStarted(keygenID string, started bool) error {
	methodResponse := ServiceMethod(db.eventBus, db.owner, "database", "set_keygen_started", keygenID, started)
	if methodResponse.Error != nil {
		return methodResponse.Error
	}
	return nil
}

type VerifierMethods interface {
	SetOwner(owner string)
	GetOwner() (owner string)

	Verify(rawMessage *bijson.RawMessage) (valid bool, verifierID string, err error)
	CleanToken(verifierIdentifier string, idtoken string) (cleanedToken string, err error)
	ListVerifiers() (verifiers []string)
}
type VerifierMethodsImpl struct {
	owner    string
	eventBus eventbus.Bus
}

func (m *VerifierMethodsImpl) GetOwner() (owner string) {
	return m.owner
}
func (m *VerifierMethodsImpl) SetOwner(owner string) {
	m.owner = owner
}

// we pass in pointers here because it has a custom marshaller
func (v *VerifierMethodsImpl) Verify(rawMessage *bijson.RawMessage) (valid bool, verifierID string, err error) {
	methodResponse := ServiceMethod(v.eventBus, v.owner, "verifier", "verify", rawMessage)
	if methodResponse.Error != nil {
		err = methodResponse.Error
		return
	}
	var data struct {
		Valid      bool
		VerifierID string
	}
	err = castOrUnmarshal(methodResponse.Data, &data)
	if err != nil {
		return valid, verifierID, err
	}
	valid = data.Valid
	verifierID = data.VerifierID
	return
}
func (v *VerifierMethodsImpl) CleanToken(verifierIdentifier string, idtoken string) (cleanedToken string, err error) {
	methodResponse := ServiceMethod(v.eventBus, v.owner, "verifier", "clean_token", verifierIdentifier, idtoken)
	if methodResponse.Error != nil {
		err = methodResponse.Error
		return
	}
	var data string
	err = castOrUnmarshal(methodResponse.Data, &data)
	if err != nil {
		return cleanedToken, err
	}
	cleanedToken = data
	return
}
func (v *VerifierMethodsImpl) ListVerifiers() (verifiers []string) {
	err := retry.Do(func() error {
		methodResponse := ServiceMethod(v.eventBus, v.owner, "verifier", "list_verifiers")
		if methodResponse.Error != nil {
			return methodResponse.Error
		}
		var data []string
		err := castOrUnmarshal(methodResponse.Data, &data)
		if err != nil {
			return err
		}
		verifiers = data
		return nil
	})
	if err != nil {
		logging.WithError(err).Fatal("could not list verifiers")
	}
	return
}

type CacheMethods interface {
	SetOwner(owner string)
	GetOwner() (owner string)

	TokenCommitExists(verifier string, tokenCommitment string) (exists bool)
	GetTokenCommitKey(verifier string, tokenCommitment string) (pubKey common.Point)
	RecordTokenCommit(verifier string, tokenCommitment string, pubKey common.Point)
	SignerSigExists(signature string) (exists bool)
	RecordSignerSig(signature string)
}
type CacheMethodsImpl struct {
	owner    string
	eventBus eventbus.Bus
}

func (m *CacheMethodsImpl) GetOwner() (owner string) {
	return m.owner
}
func (m *CacheMethodsImpl) SetOwner(owner string) {
	m.owner = owner
}
func (cache *CacheMethodsImpl) TokenCommitExists(verifier string, tokenCommitment string) (exists bool) {
	err := retry.Do(func() error {
		methodResponse := ServiceMethod(cache.eventBus, cache.owner, "cache", "token_commit_exists", verifier, tokenCommitment)
		if methodResponse.Error != nil {
			return methodResponse.Error
		}
		var data bool
		err := castOrUnmarshal(methodResponse.Data, &data)
		if err != nil {
			return err
		}
		exists = data
		return nil
	})
	if err != nil {
		logging.WithError(err).Fatal("could not check if token commit exists")
	}
	return
}

func (cache *CacheMethodsImpl) GetTokenCommitKey(verifier string, tokenCommitment string) (pubKey common.Point) {
	err := retry.Do(func() error {
		methodResponse := ServiceMethod(cache.eventBus, cache.owner, "cache", "get_token_commit_key", verifier, tokenCommitment)
		if methodResponse.Error != nil {
			return methodResponse.Error
		}
		var data common.Point
		err := castOrUnmarshal(methodResponse.Data, &data)
		if err != nil {
			return err
		}
		pubKey = data
		return nil
	})
	if err != nil {
		logging.WithError(err).Fatal("could not check if token commit exists")
	}
	return
}

func (cache *CacheMethodsImpl) RecordTokenCommit(verifier string, tokenCommitment string, pubKey common.Point) {
	methodResponse := ServiceMethod(cache.eventBus, cache.owner, "cache", "record_token_commit", verifier, tokenCommitment, pubKey)
	if methodResponse.Error != nil {
		logging.WithError(methodResponse.Error).Error("could not record token commit")
	}
}

func (cache *CacheMethodsImpl) SignerSigExists(signature string) (exists bool) {
	err := retry.Do(func() error {
		methodResponse := ServiceMethod(cache.eventBus, cache.owner, "cache", "signer_sig_exists", signature)
		if methodResponse.Error != nil {
			return methodResponse.Error
		}
		var data bool
		err := castOrUnmarshal(methodResponse.Data, &data)
		if err != nil {
			return err
		}
		exists = data
		return nil
	})
	if err != nil {
		logging.WithError(err).Fatal("could not check if signer signature exists")
	}
	return
}

func (cache *CacheMethodsImpl) RecordSignerSig(signature string) {
	methodResponse := ServiceMethod(cache.eventBus, cache.owner, "cache", "record_signer_sig", signature)
	if methodResponse.Error != nil {
		logging.WithError(methodResponse.Error).Error("could not record signer signature")
	}
}
