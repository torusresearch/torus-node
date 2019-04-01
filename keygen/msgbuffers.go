package keygen

import (
	"math/big"
	"sync"

	"github.com/torusresearch/torus-public/logging"
)

//TODO: Consider changing to pointers (for better memory usage)
// Here we store by Key index to allow for faster fetching (less iteration) when accessing the buffer
type KEYGENMsgBuffer struct {
	sync.Mutex
	ReceivedSends  map[string][]MsgWrapper // KeyIndex big.Int (in hex) to Send
	ReceivedEchoes map[string][]MsgWrapper // KeyIndex big.Int (in hex) to Echo
	ReceivedReadys map[string][]MsgWrapper // KeyIndex big.Int (in hex) to Ready
	// ReceivedShareCompletes map[string][]MsgWrapper // KeyIndex big.Int (in hex) to ShareComplete
}

// To store From(M) big.Int (in hex)
type MsgWrapper struct {
	From big.Int
	Msg  interface{}
}

//Initialize message buffer
func (buf *KEYGENMsgBuffer) InitializeMsgBuffer(startIndex big.Int, numOfKeys int) {
	buf.Lock()
	defer buf.Unlock()
	buf.ReceivedSends = make(map[string][]MsgWrapper)
	buf.ReceivedEchoes = make(map[string][]MsgWrapper)
	buf.ReceivedReadys = make(map[string][]MsgWrapper)

	for i := 0; i < numOfKeys; i++ {
		keyIndex := big.NewInt(int64(i))
		keyIndex.Add(keyIndex, &startIndex)
		buf.ReceivedSends[keyIndex.Text(16)] = make([]MsgWrapper, 0)
		buf.ReceivedEchoes[keyIndex.Text(16)] = make([]MsgWrapper, 0)
		buf.ReceivedReadys[keyIndex.Text(16)] = make([]MsgWrapper, 0)
	}
}

// This could be done genericly, but its more efficient this way
func (buf *KEYGENMsgBuffer) StoreKEYGENSend(msg KEYGENSend, from big.Int) error {
	buf.Lock()
	defer buf.Unlock()
	wrappedMsg := MsgWrapper{
		From: from,
		Msg:  msg,
	}
	buf.ReceivedSends[msg.KeyIndex.Text(16)] = append(buf.ReceivedSends[msg.KeyIndex.Text(16)], wrappedMsg)
	return nil
}

func (buf *KEYGENMsgBuffer) StoreKEYGENEcho(msg KEYGENEcho, from big.Int) error {
	buf.Lock()
	defer buf.Unlock()
	wrappedMsg := MsgWrapper{
		From: from,
		Msg:  msg,
	}
	buf.ReceivedEchoes[msg.KeyIndex.Text(16)] = append(buf.ReceivedEchoes[msg.KeyIndex.Text(16)], wrappedMsg)
	return nil
}

func (buf *KEYGENMsgBuffer) StoreKEYGENReady(msg KEYGENReady, from big.Int) error {
	buf.Lock()
	defer buf.Unlock()
	wrappedMsg := MsgWrapper{
		From: from,
		Msg:  msg,
	}
	buf.ReceivedReadys[msg.KeyIndex.Text(16)] = append(buf.ReceivedReadys[msg.KeyIndex.Text(16)], wrappedMsg)
	return nil
}

// func (buf *KEYGENMsgBuffer) StoreKEYGENShareComplete(msg KEYGENShareComplete, from big.Int) error {
// 	buf.Lock()
// 	defer buf.Unlock()
// 	wrappedMsg := MsgWrapper{
// 		From: from,
// 		Msg:  msg,
// 	}
// 	buf.ReceivedShareCompletes[msg.KeyIndex.Text(16)] = append(buf.ReceivedShareCompletes[msg.KeyIndex.Text(16)], wrappedMsg)
// 	return nil
// }

//TODO: Handle failed message
// Retrieve from the message buffer and iterate over messages
func (buf *KEYGENMsgBuffer) RetrieveKEYGENSends(keyIndex big.Int, ki AVSSKeygen) {
	buf.Lock()
	defer buf.Unlock()
	logging.Debugf("RetrieveKEYGENSends called with %v msgs", len(buf.ReceivedSends[keyIndex.Text(16)]))
	for _, wrappedSend := range buf.ReceivedSends[keyIndex.Text(16)] {
		go (ki).OnKEYGENSend(wrappedSend.Msg.(KEYGENSend), wrappedSend.From)
	}
}

func (buf *KEYGENMsgBuffer) RetrieveKEYGENEchoes(keyIndex big.Int, ki AVSSKeygen) {
	buf.Lock()
	defer buf.Unlock()
	logging.Debugf("RetrieveKEYGENEchoes called with %v msgs", len(buf.ReceivedSends[keyIndex.Text(16)]))
	for _, wrappedMsg := range buf.ReceivedEchoes[keyIndex.Text(16)] {
		go (ki).OnKEYGENEcho(wrappedMsg.Msg.(KEYGENEcho), wrappedMsg.From)
	}
}

func (buf *KEYGENMsgBuffer) RetrieveKEYGENReadys(keyIndex big.Int, ki AVSSKeygen) {
	buf.Lock()
	defer buf.Unlock()
	logging.Debugf("RetrieveKEYGENReadys called with %v msgs", len(buf.ReceivedSends[keyIndex.Text(16)]))
	for _, wrappedSend := range buf.ReceivedReadys[keyIndex.Text(16)] {
		go (ki).OnKEYGENReady(wrappedSend.Msg.(KEYGENReady), wrappedSend.From)
	}
}

// func (buf *KEYGENMsgBuffer) RetrieveKEYGENShareCompletes(keyIndex big.Int, ki *AVSSKeygen) {
// 	buf.Lock()
// 	defer buf.Unlock()
// 	for _, wrappedSend := range buf.ReceivedShareCompletes[keyIndex.Text(16)] {
// 		go (*ki).OnKEYGENShareComplete(wrappedSend.Msg.(KEYGENShareComplete), wrappedSend.From)
// 	}
// }
