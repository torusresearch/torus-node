package dkgnode

import (
	"fmt"
	"sync"

	"github.com/anthdm/hbbft"
)

// NewTransport implements a local Transport. This is used to test hbbft
// without going over the network.
type NewTransport struct {
	lock          sync.RWMutex
	peers         map[uint64]*NewTransport
	nodeReference NodeReference
	consumeCh     chan hbbft.RPC
	addr          uint64
}

// NewNewTransport returns a new NewTransport.
func NewNewTransport(addr uint64, nodeReference NodeReference) *NewTransport {
	return &NewTransport{
		peers:         make(map[uint64]*NewTransport),
		nodeReference: nodeReference,
		consumeCh:     make(chan hbbft.RPC, 1024), // nodes * nodes should be fine here,
		addr:          addr,
	}
}

// Transport is an interface that allows the abstraction of network transports.

// Consume returns a channel used for consuming and responding to RPC
// requests.
// Consume implements the Transport interface.
func (t *NewTransport) Consume() <-chan hbbft.RPC {
	return t.consumeCh
}

// SendProofMessages will equally spread the given messages under the
// participating nodes.
// SendProofMessages implements the Transport interface.
func (t *NewTransport) SendProofMessages(id uint64, msgs []interface{}) error {
	i := 0
	for addr := range t.peers {
		if err := t.makeRPC(id, addr, msgs[i]); err != nil {
			return err
		}
		i++
	}
	return nil
}

// Broadcast multicasts the given messages to all connected nodes.
// Broadcast implements the Transport interface.
func (t *NewTransport) Broadcast(id uint64, msg interface{}) error {
	for addr := range t.peers {
		if err := t.makeRPC(id, addr, msg); err != nil {
			return err
		}
	}
	return nil
}

// SendMessage implements the transport interface.
func (t *NewTransport) SendMessage(from, to uint64, msg interface{}) error {
	return t.makeRPC(from, to, msg)
}

// Connect is used to connect this tranport to another transport.
// Connect implements the Transport interface.
func (t *NewTransport) Connect(addr uint64, tr hbbft.Transport) {
	trans := tr.(*NewTransport)
	t.lock.Lock()
	defer t.lock.Unlock()
	t.peers[addr] = trans
}

// Addr returns the address of the transport. We address transport by the
// id of the node.
// Addr implements the Transport interface.
func (t *NewTransport) Addr() uint64 {
	return t.addr
}

func (t *NewTransport) makeRPC(id, addr uint64, msg interface{}) error {
	t.lock.RLock()
	peer, ok := t.peers[addr]
	t.lock.RUnlock()

	if !ok {
		return fmt.Errorf("failed to connect with %d", addr)
	}
	peer.nodeReference.JSONClient.Call(
		"hbbft",
		&hbbft.RPC{
			NodeID:  id,
			Payload: msg,
		})

	//server.go responds to this request

	// peer.consumeCh <- hbbft.RPC{
	// 	NodeID:  id,
	// 	Payload: msg,
	// }
	return nil
}
