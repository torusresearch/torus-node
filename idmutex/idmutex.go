package idmutex

import (
	"bytes"
	"fmt"
	"runtime"
	"strconv"
	"sync"
)

type goroutineID uint64

type Mutex struct {
	sync.Mutex
	accessor goroutineID
}

func (m *Mutex) Lock() {
	gID := getGoroutineID()
	if m.accessor == gID {
		panic("goroutineID " + fmt.Sprint(gID) + " already tried to lock this mutex. Deadlock.")
	}
	m.Mutex.Lock()
	m.accessor = gID
}

func (m *Mutex) Unlock() {
	m.accessor = 0
	m.Mutex.Unlock()
}

// if you use this for anything other than debugging you will go straight to hell
func getGoroutineID() goroutineID {
	b := make([]byte, 64)
	b = b[:runtime.Stack(b, false)]
	b = bytes.TrimPrefix(b, []byte("goroutine "))
	b = b[:bytes.IndexByte(b, ' ')]
	n, _ := strconv.ParseUint(string(b), 10, 64)
	return goroutineID(n)
}
