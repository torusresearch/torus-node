package hbbft

import (
	"fmt"
	"sync"
)

// LocalTransport implements a local Transport. This is used to test hbbft
// without going over the network.
type LocalTransport struct {
	lock      sync.RWMutex
	peers     map[uint64]*LocalTransport
	consumeCh chan RPC
	addr      uint64
}

// NewLocalTransport returns a new LocalTransport.
func NewLocalTransport(addr uint64) *LocalTransport {
	return &LocalTransport{
		peers:     make(map[uint64]*LocalTransport),
		consumeCh: make(chan RPC, 1024), // nodes * nodes should be fine here,
		addr:      addr,
	}
}

// Consume implements the Transport interface.
func (t *LocalTransport) Consume() <-chan RPC {
	return t.consumeCh
}

// SendProofMessages implements the Transport interface.
func (t *LocalTransport) SendProofMessages(id uint64, msgs []interface{}) error {
	i := 0
	for addr := range t.peers {
		if err := t.makeRPC(id, addr, msgs[i]); err != nil {
			return err
		}
		i++
	}
	return nil
}

// Broadcast implements the Transport interface.
func (t *LocalTransport) Broadcast(id uint64, msg interface{}) error {
	for addr := range t.peers {
		if err := t.makeRPC(id, addr, msg); err != nil {
			return err
		}
	}
	return nil
}

// SendMessage implements the transport interface.
func (t *LocalTransport) SendMessage(from, to uint64, msg interface{}) error {
	return t.makeRPC(from, to, msg)
}

// Connect implements the Transport interface.
func (t *LocalTransport) Connect(addr uint64, tr Transport) {
	trans := tr.(*LocalTransport)
	t.lock.Lock()
	defer t.lock.Unlock()
	t.peers[addr] = trans
}

// Addr implements the Transport interface.
func (t *LocalTransport) Addr() uint64 {
	return t.addr
}

func (t *LocalTransport) makeRPC(id, addr uint64, msg interface{}) error {
	t.lock.RLock()
	peer, ok := t.peers[addr]
	t.lock.RUnlock()

	if !ok {
		return fmt.Errorf("failed to connect with %d", addr)
	}
	peer.consumeCh <- RPC{
		NodeID:  id,
		Payload: msg,
	}
	return nil
}
