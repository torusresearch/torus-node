package keygen

import (
	"math/big"
	"sync"

	"github.com/torusresearch/torus-public/logging"
)

// Here we store by Key index to allow for faster fetching (less iteration) when accessing the buffer
type KEYGENBuffer struct {
	sync.Mutex
	Buffer map[string](map[string]*KEYGENMsgLog) // keyIndex => nodeIndex => buffer
}

type KEYGENMsgLog struct {
	ReceivedSend           *KEYGENSend                     // Polynomials for respective commitment matrix.
	ReceivedEchoes         map[string]*KEYGENEcho          // From(M) big.Int (in hex) to Echo
	ReceivedReadys         map[string]*KEYGENReady         // From(M) big.Int (in hex) to Ready
	ReceivedShareCompletes map[string]*KEYGENShareComplete // From(M) big.Int (in hex) to ShareComplete
}

//Initialize message buffer
func (buf *KEYGENBuffer) InitializeMsgBuffer(startIndex big.Int, numOfKeys int, nodeList []big.Int) {
	buf.Lock()
	defer buf.Unlock()
	buf.Buffer = make(map[string](map[string]*KEYGENMsgLog))

	for i := 0; i < numOfKeys; i++ {
		keyIndex := big.NewInt(int64(i))
		keyIndex.Add(keyIndex, &startIndex)
		strKeyIndex := keyIndex.Text(16)
		buf.Buffer[strKeyIndex] = make(map[string]*KEYGENMsgLog)

		for _, v := range nodeList {
			buf.Buffer[strKeyIndex][v.Text(16)] = &KEYGENMsgLog{
				ReceivedEchoes:         make(map[string]*KEYGENEcho),          // From(M) big.Int (in hex) to Echo
				ReceivedReadys:         make(map[string]*KEYGENReady),         // From(M) big.Int (in hex) to Ready
				ReceivedShareCompletes: make(map[string]*KEYGENShareComplete), // From(M) big.Int (in hex) to ShareComplete
			}
		}
	}
}

// This could be done genericly, but its more efficient this way
func (buf *KEYGENBuffer) StoreKEYGENSend(msg KEYGENSend, from big.Int) error {
	buf.Lock()
	defer buf.Unlock()
	buf.Buffer[msg.KeyIndex.Text(16)][from.Text(16)].ReceivedSend = &msg
	return nil
}

func (buf *KEYGENBuffer) StoreKEYGENEcho(msg KEYGENEcho, from big.Int) error {
	buf.Lock()
	defer buf.Unlock()
	buf.Buffer[msg.KeyIndex.Text(16)][msg.Dealer.Text(16)].ReceivedEchoes[from.Text(16)] = &msg
	return nil
}

func (buf *KEYGENBuffer) StoreKEYGENReady(msg KEYGENReady, from big.Int) error {
	buf.Lock()
	defer buf.Unlock()
	buf.Buffer[msg.KeyIndex.Text(16)][msg.Dealer.Text(16)].ReceivedReadys[from.Text(16)] = &msg
	return nil
}

// func (buf *KEYGENMsgLog) StoreKEYGENShareComplete(msg KEYGENShareComplete, from big.Int) error {
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
func (buf *KEYGENBuffer) RetrieveKEYGENSends(keyIndex big.Int, ki AVSSKeygen) {
	buf.Lock()
	defer buf.Unlock()
	count := 0
	for nodeIndex, buffer := range buf.Buffer[keyIndex.Text(16)] {
		send := buffer.ReceivedSend
		if send != nil {
			intNodeIndex := big.Int{}
			intNodeIndex.SetString(nodeIndex, 16)
			go (ki).OnKEYGENSend(*send, intNodeIndex)
			count++
		}
	}
	logging.Debugf("RetrieveKEYGENSends called with %v msgs", count)
}

func (buf *KEYGENBuffer) RetrieveKEYGENEchoes(keyIndex big.Int, dealer big.Int, ki AVSSKeygen) {
	buf.Lock()
	defer buf.Unlock()

	count := 0
	for from, echo := range buf.Buffer[keyIndex.Text(16)][dealer.Text(16)].ReceivedEchoes {
		intNodeIndex := big.Int{}
		intNodeIndex.SetString(from, 16)
		go (ki).OnKEYGENEcho(*echo, intNodeIndex)
		count++
	}
	logging.Debugf("RetrieveKEYGENEchoes called with %v msgs", count)
}

func (buf *KEYGENBuffer) RetrieveKEYGENReadys(keyIndex big.Int, dealer big.Int, ki AVSSKeygen) {
	buf.Lock()
	defer buf.Unlock()
	count := 0
	for from, ready := range buf.Buffer[keyIndex.Text(16)][dealer.Text(16)].ReceivedReadys {
		intNodeIndex := big.Int{}
		intNodeIndex.SetString(from, 16)
		go (ki).OnKEYGENReady(*ready, intNodeIndex)
		count++
	}
	logging.Debugf("RetrieveKEYGENReadys called with %v msgs", count)
}

// func (buf *KEYGENMsgLog) RetrieveKEYGENShareCompletes(keyIndex big.Int, ki *AVSSKeygen) {
// 	buf.Lock()
// 	defer buf.Unlock()
// 	for _, wrappedSend := range buf.ReceivedShareCompletes[keyIndex.Text(16)] {
// 		go (*ki).OnKEYGENShareComplete(wrappedSend.Msg.(KEYGENShareComplete), wrappedSend.From)
// 	}
// }
