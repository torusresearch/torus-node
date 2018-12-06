package hbbft

/*
DEPRECATED
*/

import (
	"encoding/binary"
	"encoding/gob"
	"fmt"
	"log"
	"math/rand"
	"strconv"
	"sync"
	"time"

	"github.com/torusresearch/torus/dkgnode"
	"github.com/anthdm/hbbft"
	jsonrpcclient "github.com/ybbus/jsonrpc"
)

const (
	lenNodes  = 11
	batchSize = 500
	numCores  = 4
)

var wg = &sync.WaitGroup{}

type message struct {
	from    uint64
	payload hbbft.MessageTuple
}

var (
	txDelay  = (0 * time.Millisecond) / numCores
	messages = make(chan message, 1024*1024)
	relayCh  = make(chan *Transaction, 1024)
)

func RunHbbft() {
	var (
		nodes = makeNetwork(lenNodes)
	)
	//node set up phase
	for _, node := range nodes {
		go node.run()
		go func(node *Server) {
			if err := node.hb.Start(); err != nil {
				log.Fatal(err)
			}
			//send out messages for initialization
			for _, msg := range node.hb.Messages() {
				nodes[node.id].transport.SendMessage(node.id, msg.To, msg.Payload)
			}
		}(node)

		//run listener loop for each server
		go func(node *Server) {
			for {
				select {
				case msg := <-node.rpcCh:
					switch t := msg.Payload.(type) {
					case hbbft.HBMessage:
						if err := node.hb.HandleMessage(msg.NodeID, t.Epoch, t.Payload.(*hbbft.ACSMessage)); err != nil {
							log.Fatal(err)
						}
					}
					// TODO: Seperate these two to run on their own triggers?
					for _, msg := range node.hb.Messages() {
						nodes[node.id].transport.SendMessage(node.id, msg.To, msg.Payload)
					}
				}
			}
		}(node)
	}

	// handle user sending transactions to all nodes.
	for {
		select {
		case tx := <-relayCh:
			for _, node := range nodes {
				node.addTransactions(tx)
			}
		}
	}
}

// Server represents the local node.
type Server struct {
	id          uint64
	hb          *hbbft.HoneyBadger
	transport   hbbft.Transport
	rpcCh       <-chan hbbft.RPC
	lock        sync.RWMutex
	mempool     map[string]*Transaction
	totalCommit int
	start       time.Time
}

func newServer(id uint64, tr hbbft.Transport, nodes []uint64) *Server {
	hb := hbbft.NewHoneyBadger(hbbft.Config{
		N:         len(nodes),
		ID:        id,
		Nodes:     nodes,
		BatchSize: batchSize,
	})
	return &Server{
		id:        id,
		transport: tr,
		hb:        hb,
		rpcCh:     tr.Consume(),
		mempool:   make(map[string]*Transaction),
		start:     time.Now(),
	}
}

// Simulate the delay of verifying a transaction.
func (s *Server) verifyTransaction(tx *Transaction) bool {
	time.Sleep(txDelay)
	return true
}

func (s *Server) addTransactions(txx ...*Transaction) {
	for _, tx := range txx {
		if s.verifyTransaction(tx) {
			s.lock.Lock()
			s.mempool[string(tx.Hash())] = tx
			s.lock.Unlock()

			// Add this transaction to the hbbft buffer.
			s.hb.AddTransaction(tx)
			// relay the transaction to all other nodes in the network.
			// go func() {
			// if err := s.transport.Broadcast(s.hb.ID, tx); err != nil {
			// 	fmt.Println("ERROR BBROADCASTING")
			// }

			// for i := 0; i < len(s.hb.Nodes); i++ {
			// 	if uint64(i) != s.hb.ID {
			// 		relayCh <- tx

			// 	}
			// }
			// }()
		}
	}
}

// Loop that is creating bunch of random transactions.
func (s *Server) txLoop() {
	timer := time.NewTicker(1 * time.Second)
	for {
		<-timer.C
		s.addTransactions(makeTransactions(1000)...)
	}
}

func (s *Server) commitLoop() {
	timer := time.NewTicker(time.Second * 2)
	n := 0
	for {
		select {
		case <-timer.C:
			out := s.hb.Outputs()
			for _, txx := range out {
				for _, tx := range txx {
					hash := tx.Hash()
					s.lock.Lock()
					if _, ok := s.mempool[string(hash)]; !ok {
						// Transaction is not in our mempool which implies we
						// need to do verification.
						s.verifyTransaction(tx.(*Transaction))
					}
					n++
					delete(s.mempool, string(hash))
					s.lock.Unlock()
				}
			}
			s.totalCommit += n
			delta := time.Since(s.start)
			if s.id == 1 {
				fmt.Println("")
				fmt.Println("===============================================")
				fmt.Printf("SERVER (%d)\n", s.id)
				fmt.Printf("commited %d transactions over %v\n", s.totalCommit, delta)
				fmt.Printf("throughput %d TX/s\n", s.totalCommit/int(delta.Seconds()))
				fmt.Println("===============================================")
				fmt.Println("")
			}
			n = 0
		}
	}
}

func (s *Server) run() {
	go s.txLoop()
	go s.commitLoop()
}

func makeNetwork(n int) []*Server {
	transports := make([]hbbft.Transport, n)
	nodes := make([]*Server, n)
	startingPort := 8001
	for i := 0; i < n; i++ {
		//edits ports here
		rpcClient := jsonrpcclient.NewClient("http://localhost:" + strconv.FormatInt(int64(startingPort+i), 10) + "/jrpc")
		tmpTransport := NewNewTransport(uint64(i), &dkgnode.NodeReference{JSONClient: rpcClient})
		transports[i] = tmpTransport
		nodes[i] = newServer(uint64(i), transports[i], makeids(n))
		go setUpHbbftServer(strconv.FormatInt(int64(startingPort+i), 10), nodes[i], nodes, tmpTransport)
	}
	connectTransports(transports)
	return nodes
}

func connectTransports(tt []hbbft.Transport) {
	for i := 0; i < len(tt); i++ {
		for ii := 0; ii < len(tt); ii++ {
			if ii == i {
				continue
			}
			tt[i].Connect(tt[ii].Addr(), tt[ii])
		}
	}
}

func makeids(n int) []uint64 {
	ids := make([]uint64, n)
	for i := 0; i < n; i++ {
		ids[i] = uint64(i)
	}
	return ids
}

// Transaction represents a transacion -\_(^_^)_/-.
type Transaction struct {
	Nonce uint64
}

func newTransaction() *Transaction {
	return &Transaction{rand.Uint64()}
}

// Hash implements the hbbft.Transaction interface.
func (t *Transaction) Hash() []byte {
	buf := make([]byte, 8)
	binary.LittleEndian.PutUint64(buf, t.Nonce)
	return buf
}

func makeTransactions(n int) []*Transaction {
	txx := make([]*Transaction, n)
	for i := 0; i < n; i++ {
		txx[i] = newTransaction()
	}
	return txx
}

func init() {
	// logrus.SetLevel(logrus.DebugLevel)
	rand.Seed(time.Now().UnixNano())
	gob.Register(&Transaction{})
}
