package hbbft

import (
	"bytes"
	"encoding/gob"
	"math"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

// HBMessage is the top level message. It holds the epoch where the message was
// created and the actual payload.
type HBMessage struct {
	Epoch   uint64
	Payload interface{}
}

// Config holds the configuration of the top level HoneyBadger protocol as for
// its sub-protocols.
type Config struct {
	// Number of participating nodes.
	N int
	// Number of faulty nodes.
	F int
	// Unique identifier of the node.
	ID uint64
	// Identifiers of the participating nodes.
	Nodes []uint64
	// Maximum number of transactions that will be comitted in one epoch.
	BatchSize int
}

// HoneyBadger represents the top-level protocol of the hbbft consensus.
type HoneyBadger struct {
	// Config holds the configuration of the engine. This may not change
	// after engine initialization.
	Config
	// The instances of the "Common Subset Algorithm for some epoch.
	acsInstances map[uint64]*ACS
	// The unbound buffer of transactions.
	txBuffer *buffer
	// current epoch.
	epoch uint64

	lock sync.RWMutex
	// Transactions that are commited in the corresponding epochs.
	outputs map[uint64][]Transaction
	// Que of messages that need to be broadcast after processing a message.
	messageQue *messageQue
	// Counter that counts the number of messages sent in one epoch.
	msgCount int
}

// NewHoneyBadger returns a new HoneyBadger instance.
func NewHoneyBadger(cfg Config) *HoneyBadger {
	return &HoneyBadger{
		Config:       cfg,
		acsInstances: make(map[uint64]*ACS),
		txBuffer:     newBuffer(),
		outputs:      make(map[uint64][]Transaction),
		messageQue:   newMessageQue(),
	}
}

// Messages returns all the internal messages from the message que. Note that
// the que will be empty after invoking this method.
func (hb *HoneyBadger) Messages() []MessageTuple {
	return hb.messageQue.messages()
}

// AddTransaction adds the given transaction to the internal buffer.
func (hb *HoneyBadger) AddTransaction(tx Transaction) {
	hb.txBuffer.push(tx)
}

// HandleMessage will process the given ACSMessage for the given epoch.
func (hb *HoneyBadger) HandleMessage(sid, epoch uint64, msg *ACSMessage) error {
	hb.msgCount++
	acs, ok := hb.acsInstances[epoch]
	if !ok {
		// Ignore this message, it comes from an older epoch.
		if epoch < hb.epoch {
			log.Warnf("ignoring old epoch")
			return nil
		}
		acs = NewACS(hb.Config)
		hb.acsInstances[epoch] = acs
	}
	if err := acs.HandleMessage(sid, msg); err != nil {
		return err
	}
	hb.addMessages(acs.messageQue.messages())
	if hb.epoch == epoch {
		return hb.maybeProcessOutput()
	}
	hb.removeOldEpochs(epoch)
	return nil
}

// Start attempt to start the consensus engine.
// TODO(@anthdm): Reconsider API change.
func (hb *HoneyBadger) Start() error {
	return hb.propose()
}

// LenMempool returns the number of transactions in the buffer.
func (hb *HoneyBadger) LenMempool() int {
	return hb.txBuffer.len()
}

// Outputs returns the commited transactions per epoch.
func (hb *HoneyBadger) Outputs() map[uint64][]Transaction {
	hb.lock.RLock()
	out := hb.outputs
	hb.lock.RUnlock()

	hb.lock.Lock()
	defer hb.lock.Unlock()
	hb.outputs = make(map[uint64][]Transaction)
	return out
}

// propose will propose a new batch for the current epoch.
func (hb *HoneyBadger) propose() error {
	if hb.txBuffer.len() == 0 {
		time.Sleep(2 * time.Second)
		return hb.propose()
	}
	batchSize := hb.BatchSize
	// If no batch size is configured, choose somewhat of an ideal batch size
	// that will scale with the number of nodes added to the network as
	// decribed in the paper.
	if batchSize == 0 {
		// TODO: clean this up, factor out into own function? Make batch size
		// configurable.
		scalar := 20
		batchSize = (len(hb.Nodes) * 2) * scalar
	}
	batchSize = int(math.Min(float64(batchSize), float64(hb.txBuffer.len())))
	n := int(math.Max(float64(1), float64(batchSize/len(hb.Nodes))))
	batch := sample(hb.txBuffer.data[:batchSize], n)

	buf := new(bytes.Buffer)
	if err := gob.NewEncoder(buf).Encode(batch); err != nil {
		return err
	}
	acs := hb.getOrNewACSInstance(hb.epoch)
	if err := acs.InputValue(buf.Bytes()); err != nil {
		return err
	}
	hb.addMessages(acs.messageQue.messages())
	return nil
}

func (hb *HoneyBadger) maybeProcessOutput() error {
	start := time.Now()
	acs, ok := hb.acsInstances[hb.epoch]
	if !ok {
		return nil
	}
	outputs := acs.Output()
	if outputs == nil || len(outputs) == 0 {
		return nil
	}

	txMap := make(map[string]Transaction)
	for _, output := range outputs {
		var txx []Transaction
		if err := gob.NewDecoder(bytes.NewReader(output)).Decode(&txx); err != nil {
			return err
		}
		// Dedup the transactions, buffers could occasionally pick the same
		// transaction.
		for _, tx := range txx {
			txMap[string(tx.Hash())] = tx
		}
	}
	txBatch := make([]Transaction, len(txMap))
	i := 0
	for _, tx := range txMap {
		txBatch[i] = tx
		i++
	}
	// Delete the transactions from the buffer.
	hb.txBuffer.delete(txBatch)
	// Add the transaction to the commit log.
	hb.outputs[hb.epoch] = txBatch
	hb.epoch++

	if hb.epoch%100 == 0 {
		log.Debugf("node (%d) commited (%d) transactions in epoch (%d) took %v",
			hb.ID, len(txBatch), hb.epoch, time.Since(start))
	}
	hb.msgCount = 0

	return hb.propose()
}

func (hb *HoneyBadger) getOrNewACSInstance(epoch uint64) *ACS {
	if acs, ok := hb.acsInstances[epoch]; ok {
		return acs
	}
	acs := NewACS(hb.Config)
	hb.acsInstances[epoch] = acs
	return acs
}

// removeOldEpochs removes the ACS instances that have already been terminated.
func (hb *HoneyBadger) removeOldEpochs(epoch uint64) {
	for i := epoch; i < hb.epoch-1; i++ {
		delete(hb.acsInstances, i)
	}
}

func (hb *HoneyBadger) addMessages(msgs []MessageTuple) {
	for _, msg := range msgs {
		hb.messageQue.addMessage(HBMessage{hb.epoch, msg.Payload}, msg.To)
	}
}
