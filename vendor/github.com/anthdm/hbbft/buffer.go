package hbbft

import (
	"math/rand"
	"sync"
	"time"
)

// buffer holds an uncommited pool of arbitrary data. In blockchain parlance
// this would be the unbound transaction list.
type buffer struct {
	lock sync.RWMutex
	data []Transaction
}

// newBuffer return a new initialized buffer with the max data cap set to 1024.
func newBuffer() *buffer {
	return &buffer{
		data: make([]Transaction, 0, 1024),
	}
}

// push pushes v to the buffer.
func (b *buffer) push(tx Transaction) {
	b.lock.Lock()
	defer b.lock.Unlock()
	b.data = append(b.data, tx)
}

// @optimize: This can be much more efficient.
// delete removes the given slice of Transactions from the buffer.
func (b *buffer) delete(txx []Transaction) {
	b.lock.Lock()
	defer b.lock.Unlock()
	temp := make(map[string]Transaction)
	for i := 0; i < len(b.data); i++ {
		temp[string(b.data[i].Hash())] = b.data[i]
	}
	for i := 0; i < len(txx); i++ {
		delete(temp, string(txx[i].Hash()))
	}
	data := make([]Transaction, len(temp))
	i := 0
	for _, tx := range temp {
		data[i] = tx
		i++
	}
	b.data = data
}

// len return the current length of the buffer.
func (b *buffer) len() int {
	b.lock.RLock()
	defer b.lock.RUnlock()
	return len(b.data)
}

// sample will return (n) elements from the buffer with (m) as its maximum
// upperbound.
// [ a ]
// [ b ]
// [ c ] m
// [ d ]
// In the above example there can be only sampled between a, b and c.
// If the length of the buffer is smaller then 1 no transactions are being
// returned. If the length of the buffer is smaller then (m) the total length of
// the buffer will be used as upperbound.
func (b *buffer) sample(n, m int) []Transaction {
	txx := []Transaction{}
	for i := 0; i < n; i++ {
		if b.len() <= 1 {
			break
		}
		if b.len() < m {
			m = b.len() - 1
		}
		index := rand.Intn(m)
		txx = append(txx, b.data[index])
	}
	return txx
}

func sample(txx []Transaction, n int) []Transaction {
	s := []Transaction{}
	for i := 0; i < n; i++ {
		s = append(s, txx[rand.Intn(len(txx))])
	}
	return s
}

func init() { rand.Seed(time.Now().UnixNano()) }
