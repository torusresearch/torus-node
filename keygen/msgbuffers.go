package keygen

import (
	"github.com/torusresearch/torus-public/sync"
	"math/big"

	"github.com/torusresearch/torus-public/logging"
)

// Here we store by Key index to allow for faster fetching (less iteration) when accessing the buffer
type KEYGENBuffer struct {
	sync.Mutex
	Buffer               map[string](map[string]*KEYGENMsgLog)   // keyIndex => nodeIndex => buffer
	ReceivedDKGCompletes map[int](map[string]*KEYGENDKGComplete) // From int (array ) to M big.Int (in hex) to DKGComplete
}

type KEYGENMsgLog struct {
	ReceivedSend   *KEYGENSend             // Polynomials for respective commitment matrix.
	ReceivedEchoes map[string]*KEYGENEcho  // From(M) big.Int (in hex) to Echo
	ReceivedReadys map[string]*KEYGENReady // From(M) big.Int (in hex) to Ready

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
		buf.ReceivedDKGCompletes = make(map[int](map[string]*KEYGENDKGComplete)) // From int (array ) to M big.Int (in hex) to DKGComplete

		for _, v := range nodeList {
			buf.Buffer[strKeyIndex][v.Text(16)] = &KEYGENMsgLog{
				ReceivedEchoes: make(map[string]*KEYGENEcho),  // From(M) big.Int (in hex) to Echo
				ReceivedReadys: make(map[string]*KEYGENReady), // From(M) big.Int (in hex) to Ready

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

func (buf *KEYGENBuffer) StoreKEYGENDKGComplete(msg KEYGENDKGComplete, from big.Int) error {
	buf.Lock()
	defer buf.Unlock()
	_, ok := buf.ReceivedDKGCompletes[msg.Nonce]
	if !ok {
		buf.ReceivedDKGCompletes[msg.Nonce] = make(map[string]*KEYGENDKGComplete)
	}
	buf.ReceivedDKGCompletes[msg.Nonce][from.Text(16)] = &msg
	return nil
}

//TODO: Handle failed message
// Retrieve from the message buffer and iterate over messages
func (buf *KEYGENBuffer) RetrieveKEYGENSends(keyIndex big.Int, dealer big.Int) *KEYGENSend {
	buf.Lock()
	defer buf.Unlock()
	logging.Debugf("RetrieveKEYGENSends called where %v", buf.Buffer[keyIndex.Text(16)][dealer.Text(16)].ReceivedSend == nil)
	return buf.Buffer[keyIndex.Text(16)][dealer.Text(16)].ReceivedSend
}

func (buf *KEYGENBuffer) RetrieveKEYGENEchoes(keyIndex big.Int, dealer big.Int) map[string]*KEYGENEcho {
	buf.Lock()
	defer buf.Unlock()
	logging.Debugf("RetrieveKEYGENEchos called with %v msgs", len(buf.Buffer[keyIndex.Text(16)][dealer.Text(16)].ReceivedEchoes))
	return buf.Buffer[keyIndex.Text(16)][dealer.Text(16)].ReceivedEchoes
}

func (buf *KEYGENBuffer) RetrieveKEYGENReadys(keyIndex big.Int, dealer big.Int) map[string]*KEYGENReady {
	buf.Lock()
	defer buf.Unlock()
	logging.Debugf("RetrieveKEYGENReadys called with %v msgs", len(buf.Buffer[keyIndex.Text(16)][dealer.Text(16)].ReceivedReadys))
	return buf.Buffer[keyIndex.Text(16)][dealer.Text(16)].ReceivedReadys
}

func (buf *KEYGENBuffer) RetrieveKEYGENDKGComplete(nonce int, dealer big.Int) map[string]*KEYGENDKGComplete {
	buf.Lock()
	defer buf.Unlock()
	logging.Debugf("RetrieveKEYGENDKGCompletes called with %v msgs", len(buf.ReceivedDKGCompletes[nonce]))
	return buf.ReceivedDKGCompletes[nonce]
}

func (buf *KEYGENBuffer) CheckLengthOfEcho(keyIndex big.Int, dealer big.Int) int {
	buf.Lock()
	defer buf.Unlock()
	return len(buf.Buffer[keyIndex.Text(16)][dealer.Text(16)].ReceivedEchoes)
}

func (buf *KEYGENBuffer) CheckLengthOfReady(keyIndex big.Int, dealer big.Int) int {
	buf.Lock()
	defer buf.Unlock()
	return len(buf.Buffer[keyIndex.Text(16)][dealer.Text(16)].ReceivedReadys)
}

// func (buf *KEYGENMsgLog) RetrieveKEYGENShareCompletes(keyIndex big.Int, ki *AVSSKeygen) {
// 	buf.Lock()
// 	defer buf.Unlock()
// 	for _, wrappedSend := range buf.ReceivedShareCompletes[keyIndex.Text(16)] {
// 		go (*ki).OnKEYGENDKGComplete(wrappedSend.Msg.(KEYGENShareComplete), wrappedSend.From)
// 	}
// }
