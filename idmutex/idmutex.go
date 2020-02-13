package idmutex

import (
	deadlock "sync"
	// "github.com/sasha-s/go-deadlock"
)

// github.com/sasha-s/go-deadlock

// func init() {
// 	deadlock.Opts.DeadlockTimeout = 120 * time.Second
// }

// type goroutineID uint64

type Mutex struct {
	deadlock.Mutex
	// accessor goroutineID
}

// Uncomment the code below for debugging deadlocks

func (m *Mutex) Lock() {
	// gID := getGoroutineID()
	// if m.accessor == gID {
	// 	panic("goroutineID " + fmt.Sprint(gID) + " already tried to lock this mutex. Deadlock.")
	// }
	m.Mutex.Lock()
	// m.accessor = gID
}

func (m *Mutex) Unlock() {
	// m.accessor = 0
	m.Mutex.Unlock()
}

// if you use this for anything other than debugging you will go straight to hell
// func getGoroutineID() goroutineID {
// 	b := make([]byte, 64)
// 	b = b[:runtime.Stack(b, false)]
// 	b = bytes.TrimPrefix(b, []byte("goroutine "))
// 	b = b[:bytes.IndexByte(b, ' ')]
// 	n, _ := strconv.ParseUint(string(b), 10, 64)
// 	return goroutineID(n)
// }
