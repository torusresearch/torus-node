package hbbft

import (
	"fmt"
	"sync"

	log "github.com/sirupsen/logrus"
)

// AgreementMessage holds the epoch and the message sent in the BBA protocol.
type AgreementMessage struct {
	// Epoch when this message was sent.
	Epoch int
	// The actual contents of the message.
	Message interface{}
}

// NewAgreementMessage constructs a new AgreementMessage.
func NewAgreementMessage(e int, msg interface{}) *AgreementMessage {
	return &AgreementMessage{
		Epoch:   e,
		Message: msg,
	}
}

// BvalRequest holds the input value of the binary input.
type BvalRequest struct {
	Value bool
}

// AuxRequest holds the output value.
type AuxRequest struct {
	Value bool
}

// BBA is the Binary Byzantine Agreement build from a common coin protocol.
type BBA struct {
	// Config holds the BBA configuration.
	Config
	// Current epoch.
	epoch uint32
	//  Bval requests we accepted this epoch.
	binValues []bool
	// sentBvals are the binary values this instance sent.
	sentBvals []bool
	// recvBval is a mapping of the sender and the receveived binary value.
	recvBval map[uint64]bool
	// recvAux is a mapping of the sender and the receveived Aux value.
	recvAux map[uint64]bool
	// Whether this bba is terminated or not.
	done bool
	// output and estimated of the bba protocol. This can be either nil or a
	// boolean.
	output, estimated, decision interface{}
	//delayedMessages are messages that are received by a node that is already
	// in a later epoch. These messages will be qued an handled the next epoch.
	delayedMessages []delayedMessage

	lock sync.RWMutex
	// Que of AgreementMessages that need to be broadcasted after each received
	// message.
	messages []*AgreementMessage

	// control flow tuples for internal channel communication.
	inputCh   chan bbaInputTuple
	messageCh chan bbaMessageTuple
	msgCount  int
}

// NewBBA returns a new instance of the Binary Byzantine Agreement.
func NewBBA(cfg Config) *BBA {
	if cfg.F == 0 {
		cfg.F = (cfg.N - 1) / 3
	}
	bba := &BBA{
		Config:          cfg,
		recvBval:        make(map[uint64]bool),
		recvAux:         make(map[uint64]bool),
		sentBvals:       []bool{},
		binValues:       []bool{},
		inputCh:         make(chan bbaInputTuple),
		messageCh:       make(chan bbaMessageTuple),
		messages:        []*AgreementMessage{},
		delayedMessages: []delayedMessage{},
	}
	go bba.run()
	return bba
}

// Control flow structure for internal channel communication. Allowing us to
// avoid the use of mutexes and eliminates race conditions.
type (
	bbaMessageTuple struct {
		senderID uint64
		msg      *AgreementMessage
		err      chan error
	}

	bbaInputTuple struct {
		value bool
		err   chan error
	}

	delayedMessage struct {
		sid uint64
		msg *AgreementMessage
	}
)

// InputValue will set the given val as the initial value to be proposed in the
// Agreement and returns an initial AgreementMessage or an error.
func (b *BBA) InputValue(val bool) error {
	t := bbaInputTuple{
		value: val,
		err:   make(chan error),
	}
	b.inputCh <- t
	return <-t.err
}

// HandleMessage will process the given rpc message. The caller is resposible to
// make sure only RPC messages are passed that are elligible for the BBA protocol.
func (b *BBA) HandleMessage(senderID uint64, msg *AgreementMessage) error {
	b.msgCount++
	t := bbaMessageTuple{
		senderID: senderID,
		msg:      msg,
		err:      make(chan error),
	}
	b.messageCh <- t
	return <-t.err
}

// AcceptInput returns true whether this bba instance is elligable for accepting
// a new input value.
func (b *BBA) AcceptInput() bool {
	return b.epoch == 0 && b.estimated == nil
}

// Output will return the output of the bba instance. If the output was not nil
// then it will return the output else nil. Note that after consuming the output
// its will be set to nil forever.
func (b *BBA) Output() interface{} {
	if b.output != nil {
		out := b.output
		b.output = nil
		return out
	}
	return nil
}

// Messages returns the que of messages. The message que get's filled after
// processing a protocol message. After calling this method the que will
// be empty. Hence calling Messages can only occur once in a single roundtrip.
func (b *BBA) Messages() []*AgreementMessage {
	b.lock.RLock()
	msgs := b.messages
	b.lock.RUnlock()

	b.lock.Lock()
	defer b.lock.Unlock()
	b.messages = []*AgreementMessage{}
	return msgs
}

func (b *BBA) addMessage(msg *AgreementMessage) {
	b.lock.Lock()
	defer b.lock.Unlock()
	b.messages = append(b.messages, msg)
}

// run makes sure we only process 1 message at the same time, avoiding mutexes
// and race conditions.
func (b *BBA) run() {
	for {
		select {
		case t := <-b.inputCh:
			t.err <- b.inputValue(t.value)
		case t := <-b.messageCh:
			t.err <- b.handleMessage(t.senderID, t.msg)
		}
	}
}

// inputValue will set the given val as the initial value to be proposed in the
// Agreement.
func (b *BBA) inputValue(val bool) error {
	// Make sure we are in the first epoch round.
	if b.epoch != 0 || b.estimated != nil {
		return nil
	}
	b.estimated = val
	b.sentBvals = append(b.sentBvals, val)
	b.addMessage(NewAgreementMessage(int(b.epoch), &BvalRequest{val}))
	return b.handleBvalRequest(b.ID, val)
}

// handleMessage will process the given rpc message. The caller is resposible to
// make sure only RPC messages are passed that are elligible for the BBA protocol.
func (b *BBA) handleMessage(senderID uint64, msg *AgreementMessage) error {
	if b.done {
		return nil
	}
	// Ignore messages from older epochs.
	if msg.Epoch < int(b.epoch) {
		log.Debugf(
			"id (%d) with epoch (%d) received msg from an older epoch (%d)",
			b.ID, b.epoch, msg.Epoch,
		)
		return nil
	}
	// Messages from later epochs will be qued and processed later.
	if msg.Epoch > int(b.epoch) {
		b.delayedMessages = append(b.delayedMessages, delayedMessage{senderID, msg})
		return nil
	}

	switch t := msg.Message.(type) {
	case *BvalRequest:
		return b.handleBvalRequest(senderID, t.Value)
	case *AuxRequest:
		return b.handleAuxRequest(senderID, t.Value)
	default:
		return fmt.Errorf("unknown BBA message received: %v", t)
	}
}

// handleBvalRequest processes the received binary value and fills up the
// message que if there are any messages that need to be broadcasted.
func (b *BBA) handleBvalRequest(senderID uint64, val bool) error {
	b.lock.Lock()
	b.recvBval[senderID] = val
	b.lock.Unlock()
	lenBval := b.countBvals(val)

	// When receiving n bval(b) messages from 2f+1 nodes: inputs := inputs u {b}
	if lenBval == 2*b.F+1 {
		wasEmptyBinValues := len(b.binValues) == 0
		b.binValues = append(b.binValues, val)
		// If inputs > 0 broadcast output(b) and handle the output ourselfs.
		// Wait until binValues > 0, then broadcast AUX(b). The AUX(b) broadcast
		// may only occure once per epoch.
		if wasEmptyBinValues {
			b.addMessage(NewAgreementMessage(int(b.epoch), &AuxRequest{val}))
			b.handleAuxRequest(b.ID, val)
		}
		return nil
	}
	// When receiving input(b) messages from f + 1 nodes, if inputs(b) is not
	// been sent yet broadcast input(b) and handle the input ourselfs.
	if lenBval == b.F+1 && !b.hasSentBval(val) {
		b.sentBvals = append(b.sentBvals, val)
		b.addMessage(NewAgreementMessage(int(b.epoch), &BvalRequest{val}))
		return b.handleBvalRequest(b.ID, val)
	}
	return nil
}

func (b *BBA) handleAuxRequest(senderID uint64, val bool) error {
	b.lock.Lock()
	b.recvAux[senderID] = val
	b.lock.Unlock()
	b.tryOutputAgreement()
	return nil
}

// tryOutputAgreement waits until at least (N - f) output messages received,
// once the (N - f) messages are received, make a common coin and uses it to
// compute the next decision estimate and output the optional decision value.
func (b *BBA) tryOutputAgreement() {
	if len(b.binValues) == 0 {
		return
	}
	// Wait longer till eventually receive (N - F) aux messages.
	lenOutputs, values := b.countOutputs()
	if lenOutputs < b.N-b.F {
		return
	}

	// TODO: implement a real common coin algorithm.
	coin := b.epoch%2 == 0

	// Continue the BBA until both:
	// - a value b is output in some epoch r
	// - the value (coin r) = b for some round r' > r
	if b.done || b.decision != nil && b.decision.(bool) == coin {
		b.done = true
		return
	}

	log.Debugf(
		"id (%d) is advancing to the next epoch! (%d) received (%d) aux messages",
		b.ID, b.epoch+1, lenOutputs,
	)

	// Start the next epoch.
	b.advanceEpoch()

	if len(values) != 1 {
		b.estimated = coin
	} else {
		b.estimated = values[0]
		// Output may be set only once.
		if b.decision == nil && values[0] == coin {
			b.output = values[0]
			b.decision = values[0]
			log.Debugf("id (%d) outputed a decision (%v) after (%d) msgs", b.ID, values[0], b.msgCount)
			b.msgCount = 0
		}
	}
	estimated := b.estimated.(bool)
	b.sentBvals = append(b.sentBvals, estimated)
	b.addMessage(NewAgreementMessage(int(b.epoch), &BvalRequest{estimated}))

	// handle the delayed messages.
	for _, que := range b.delayedMessages {
		if err := b.handleMessage(que.sid, que.msg); err != nil {
			// TODO: Handle this error properly.
			log.Warn(err)
		}
	}
	b.delayedMessages = []delayedMessage{}
}

// advanceEpoch will reset all the values that are bound to an epoch and increments
// the epoch value by 1.
func (b *BBA) advanceEpoch() {
	b.binValues = []bool{}
	b.sentBvals = []bool{}
	b.recvAux = make(map[uint64]bool)
	b.recvBval = make(map[uint64]bool)
	b.epoch++
}

// countOutputs returns the number of received (aux) messages, the corresponding
// values that where also in our inputs.
func (b *BBA) countOutputs() (int, []bool) {
	m := map[bool]int{}
	for i, val := range b.recvAux {
		m[val] = int(i)
	}
	vals := []bool{}
	for _, val := range b.binValues {
		if _, ok := m[val]; ok {
			vals = append(vals, val)
		}
	}
	return len(b.recvAux), vals
}

// countBvals counts all the received Bval inputs matching b.
func (b *BBA) countBvals(ok bool) int {
	b.lock.RLock()
	defer b.lock.RUnlock()
	n := 0
	for _, val := range b.recvBval {
		if val == ok {
			n++
		}
	}
	return n
}

// hasSentBval return true if we already sent out the given value.
func (b *BBA) hasSentBval(val bool) bool {
	for _, ok := range b.sentBvals {
		if ok == val {
			return true
		}
	}
	return false
}
