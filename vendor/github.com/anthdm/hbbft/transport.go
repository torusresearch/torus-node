package hbbft

// RPC holds the payload send between participants in the consensus.
type RPC struct {
	// NodeID is the unique identifier of the sending node.
	// TODO: consider renaming this to SenderID.
	NodeID uint64
	// Payload beeing send.
	Payload interface{}
}

// Transport is an interface that allows the abstraction of network transports.
type Transport interface {
	// Consume returns a channel used for consuming and responding to RPC
	// requests.
	Consume() <-chan RPC

	// SendProofMessages will equally spread the given messages under the
	// participating nodes.
	SendProofMessages(from uint64, msgs []interface{}) error

	// Broadcast multicasts the given messages to all connected nodes.
	Broadcast(from uint64, msg interface{}) error

	SendMessage(from, to uint64, msg interface{}) error

	// Connect is used to connect this tranport to another transport.
	Connect(uint64, Transport)

	// Addr returns the address of the transport. We address transport by the
	// id of the node.
	Addr() uint64
}
