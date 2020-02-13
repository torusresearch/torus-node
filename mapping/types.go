package mapping

import (
	"errors"
	"math/big"
	"strconv"
	"strings"

	cmap "github.com/orcaman/concurrent-map"
	logging "github.com/sirupsen/logrus"
	"github.com/torusresearch/bijson"
	"github.com/torusresearch/torus-common/common"
	pcmn "github.com/torusresearch/torus-node/common"
	"github.com/torusresearch/torus-node/idmutex"
	"github.com/torusresearch/torus-node/version"
)

type MappingNode struct {
	idmutex.Mutex
	NodeDetails    NodeDetails
	OldNodes       NodeNetwork
	NewNodes       NodeNetwork
	NodeIndex      int
	Transport      MappingTransport
	DataSource     MappingDataSource
	MappingSummary *MappingSummary
	MappingKeys    *MappingKeys
	IsOldNode      bool
	IsNewNode      bool
}
type MappingSummary struct {
	SendCount           *NodeDetailsToIntCMap
	MappingSummaryStore *TransferSummaryIDToTransferSummaryCMap
	MappingSummaryCount *TransferSummaryToIntCMap
}
type TransferSummaryID string
type TransferSummary struct {
	LastUnassignedIndex uint `json:"last_unassigned_index"`
}

func (t *TransferSummary) ID() TransferSummaryID {
	return TransferSummaryID(strconv.Itoa(int(t.LastUnassignedIndex)))
}

type MappingKeys struct {
	SendCount *MappingKeyIDToNodeDetailsIDToIntCMap
}
type MappingKeyID string
type MappingKey struct {
	Index     big.Int             `json:"index"`
	PublicKey common.Point        `json:"publicKey"`
	Threshold int                 `json:"threshold"`
	Verifiers map[string][]string `json:"verifiers"` // Verifier => VerifierID
}

func (m *MappingKey) ID() MappingKeyID {
	byt, err := bijson.Marshal(m)
	if err != nil {
		logging.WithField("mappingKey", m).Error("Could not marshal mappingKey")
		return MappingKeyID("")
	}
	return MappingKeyID(string(byt))
}

type MappingTransport interface {
	Init()
	GetType() string
	SetMappingNode(*MappingNode) error
	Sign([]byte) ([]byte, error)
	Send(NodeDetails, MappingMessage) error
	Receive(NodeDetails, MappingMessage) error
	SendBroadcast(MappingMessage) error
	ReceiveBroadcast(MappingMessage) error
	Output(interface{})
}

type MappingDataSource interface {
	Init()
	GetType() string
	SetMappingNode(*MappingNode) error
	RetrieveKeyMapping(index big.Int) (MappingKey, error)
}

func CreateMappingMessage(r MappingMessageRaw) MappingMessage {
	return MappingMessage{
		Version:   mappingMessageVersion(version.NodeVersion),
		MappingID: r.MappingID,
		Method:    r.Method,
		Data:      r.Data,
	}
}

type MappingMessageRaw struct {
	MappingID MappingID
	Method    string
	Data      []byte
}

type mappingMessageVersion string

// MappingMessage should not be used directly, use the CreateMappingMessage constructor
type MappingMessage struct {
	Version   mappingMessageVersion `json:"version,omitempty"`
	MappingID MappingID             `json:"mappingID"`
	Method    string                `json:"method"`
	Data      []byte                `json:"data"`
}

// MappingID is the identifying string for MappingMessage
type MappingID string

const NullMappingID = MappingID("")

type MappingIDDetails struct {
	OldEpoch int
	NewEpoch int
}

func (mappingIDDetails *MappingIDDetails) ToMappingID() MappingID {
	return MappingID(strings.Join([]string{strconv.Itoa(mappingIDDetails.OldEpoch), strconv.Itoa(mappingIDDetails.NewEpoch)}, pcmn.Delimiter1))
}

func (mappingIDDetails *MappingIDDetails) FromMappingID(mappingID MappingID) error {
	s := string(mappingID)
	substrings := strings.Split(s, pcmn.Delimiter1)

	if len(substrings) != 2 {
		return errors.New("Error parsing PSSIDDetails, too few fields")
	}
	oldEpoch, err := strconv.Atoi(substrings[0])
	if err != nil {
		return err
	}
	newEpoch, err := strconv.Atoi(substrings[1])
	if err != nil {
		return err
	}
	mappingIDDetails.OldEpoch = oldEpoch
	mappingIDDetails.NewEpoch = newEpoch
	return nil
}

type NodeDetails pcmn.Node

type NodeNetwork struct {
	Nodes   map[NodeDetailsID]NodeDetails
	N       int
	T       int
	K       int
	EpochID int
}

func (nodeNetwork *NodeNetwork) NodeExists(nodeDetails NodeDetails) bool {
	if nodeNetwork.Nodes == nil {
		return false
	}
	_, found := nodeNetwork.Nodes[nodeDetails.ToNodeDetailsID()]
	return found
}

type NodeDetailsID string

func (n *NodeDetails) ToNodeDetailsID() NodeDetailsID {
	return NodeDetailsID(strings.Join([]string{
		strconv.Itoa(n.Index),
		n.PubKey.X.Text(16),
		n.PubKey.Y.Text(16),
	}, pcmn.Delimiter1))
}
func (n *NodeDetails) FromNodeDetailsID(nodeDetailsID NodeDetailsID) {
	s := string(nodeDetailsID)
	substrings := strings.Split(s, pcmn.Delimiter1)

	if len(substrings) != 3 {
		return
	}
	index, err := strconv.Atoi(substrings[0])
	if err != nil {
		return
	}
	n.Index = index
	pubkeyX, ok := new(big.Int).SetString(substrings[1], 16)
	if !ok {
		return
	}
	n.PubKey.X = *pubkeyX
	pubkeyY, ok := new(big.Int).SetString(substrings[2], 16)
	if !ok {
		return
	}
	n.PubKey.Y = *pubkeyY
}

func mapFromNodeList(nodeList []pcmn.Node) (res map[NodeDetailsID]NodeDetails) {
	res = make(map[NodeDetailsID]NodeDetails)
	for _, node := range nodeList {
		nodeDetails := NodeDetails(node)
		res[nodeDetails.ToNodeDetailsID()] = nodeDetails
	}
	return
}

type MappingProposeFreezeMessage struct {
	MappingID MappingID
}

type MappingProposeFreezeBroadcastMessage struct {
	MappingID MappingID
}

type MappingSummaryMessage struct {
	TransferSummary TransferSummary
}
type MappingSummaryBroadcastMessage struct {
	TransferSummary TransferSummary
}
type MappingKeyMessage struct {
	MappingKey MappingKey
}
type MappingKeyBroadcastMessage struct {
	MappingKey MappingKey
}

func stringify(i interface{}) string {
	bytArr, ok := i.([]byte)
	if ok {
		return string(bytArr)
	}
	str, ok := i.(string)
	if ok {
		return str
	}
	byt, err := bijson.Marshal(i)
	if err != nil {
		logging.WithError(err).Error("Could not bijsonmarshal")
	}
	return string(byt)
}

type VerifierIterator struct {
	pcmn.Iterator
}

func (v *VerifierIterator) Value() (val pcmn.VerifierData) {
	val, ok := v.Iterator.Value().(pcmn.VerifierData)
	if !ok {
		val.Err = errors.New("Could not cast to struct")
		logging.WithError(val.Err).Error("verifierIterator value not able to be cast")
	}
	return
}

type NodeDetailsToIntCMap struct {
	cmap.ConcurrentMap
}

func NewNodeDetailsToIntCMap() *NodeDetailsToIntCMap {
	return &NodeDetailsToIntCMap{
		ConcurrentMap: cmap.New(),
	}
}
func (s *NodeDetailsToIntCMap) Get(nodeDetailsID NodeDetailsID) (i int) {
	resInter, ok := s.ConcurrentMap.Get(string(nodeDetailsID))
	if !ok {
		return
	}
	res, ok := resInter.(int)
	if !ok {
		return
	}
	return res
}
func (s *NodeDetailsToIntCMap) Set(nodeDetailsID NodeDetailsID, i int) {
	s.ConcurrentMap.Set(string(nodeDetailsID), i)
}

type TransferSummaryIDToTransferSummaryCMap struct {
	cmap.ConcurrentMap
}

func NewTransferSummaryIDToTransferSummaryCMap() *TransferSummaryIDToTransferSummaryCMap {
	return &TransferSummaryIDToTransferSummaryCMap{
		ConcurrentMap: cmap.New(),
	}
}
func (s *TransferSummaryIDToTransferSummaryCMap) Get(transferSummaryID TransferSummaryID) (transferSummary TransferSummary) {
	resInter, ok := s.ConcurrentMap.Get(string(transferSummaryID))
	if !ok {
		return
	}
	res, ok := resInter.(TransferSummary)
	if !ok {
		return
	}
	return res
}
func (s *TransferSummaryIDToTransferSummaryCMap) Set(transferSummaryID TransferSummaryID, transferSummary TransferSummary) {
	s.ConcurrentMap.Set(string(transferSummaryID), transferSummary)
}

type TransferSummaryToIntCMap struct {
	cmap.ConcurrentMap
}

func NewTransferSummaryToIntCMap() *TransferSummaryToIntCMap {
	return &TransferSummaryToIntCMap{
		ConcurrentMap: cmap.New(),
	}
}
func (s *TransferSummaryToIntCMap) Get(transferSummaryID TransferSummaryID) (i int) {
	resInter, ok := s.ConcurrentMap.Get(string(transferSummaryID))
	if !ok {
		return
	}
	res, ok := resInter.(int)
	if !ok {
		return
	}
	return res
}
func (s *TransferSummaryToIntCMap) Set(transferSummaryID TransferSummaryID, i int) {
	s.ConcurrentMap.Set(string(transferSummaryID), i)
}
func (s *TransferSummaryToIntCMap) SyncUpdate(transferSummaryID TransferSummaryID, cb func(valueInMap int) int) (res int) {
	return s.ConcurrentMap.Upsert(string(transferSummaryID), nil, cmap.UpsertCb(func(exist bool, valueInMapInter interface{}, newValueInter interface{}) interface{} {
		var valueInMap int
		valueInMap, _ = valueInMapInter.(int)
		return cb(valueInMap)
	})).(int)
}

type MappingKeyIDToNodeDetailsIDToIntCMap struct {
	cmap.ConcurrentMap
}

func NewMappingKeyIDToNodeDetailsIDToIntCMap() *MappingKeyIDToNodeDetailsIDToIntCMap {
	return &MappingKeyIDToNodeDetailsIDToIntCMap{
		ConcurrentMap: cmap.New(),
	}
}
func (s *MappingKeyIDToNodeDetailsIDToIntCMap) Get(mappingKeyID MappingKeyID) (nodeDetailsIDToIntSyncMap *NodeDetailsIDToIntCMap) {
	resInter, ok := s.ConcurrentMap.Get(string(mappingKeyID))
	if !ok {
		return
	}
	res, ok := resInter.(*NodeDetailsIDToIntCMap)
	if !ok {
		return
	}
	return res
}
func (s *MappingKeyIDToNodeDetailsIDToIntCMap) Set(mappingKeyID MappingKeyID, nodeDetailsIDToIntCMap *NodeDetailsIDToIntCMap) {
	s.ConcurrentMap.Set(string(mappingKeyID), nodeDetailsIDToIntCMap)
}
func (s *MappingKeyIDToNodeDetailsIDToIntCMap) SyncUpdate(mappingKeyID MappingKeyID, cb func(valueInMap *NodeDetailsIDToIntCMap) *NodeDetailsIDToIntCMap) (res *NodeDetailsIDToIntCMap) {
	return s.ConcurrentMap.Upsert(string(mappingKeyID), nil, cmap.UpsertCb(func(exist bool, valueInMapInter interface{}, newValueInter interface{}) interface{} {
		var valueInMap *NodeDetailsIDToIntCMap
		valueInMap, _ = valueInMapInter.(*NodeDetailsIDToIntCMap)
		return cb(valueInMap)
	})).(*NodeDetailsIDToIntCMap)
}

type NodeDetailsIDToIntCMap struct {
	cmap.ConcurrentMap
}

func NewNodeDetailsIDToIntCMap() *NodeDetailsIDToIntCMap {
	return &NodeDetailsIDToIntCMap{
		ConcurrentMap: cmap.New(),
	}
}
func (s *NodeDetailsIDToIntCMap) Get(nodeDetailsID NodeDetailsID) (i int) {
	resInter, ok := s.ConcurrentMap.Get(string(nodeDetailsID))
	if !ok {
		return
	}
	res, ok := resInter.(int)
	if !ok {
		return
	}
	return res
}
func (s *NodeDetailsIDToIntCMap) Set(nodeDetailsID NodeDetailsID, i int) {
	s.ConcurrentMap.Set(string(nodeDetailsID), i)
}
