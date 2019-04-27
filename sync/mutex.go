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
	Accessors map[goroutineID]bool
}

func (m *Mutex) Lock() {
	currGoroutineID := getGoroutineID()
	if m.Accessors[currGoroutineID] {
		panic("goroutineID " + fmt.Sprint(currGoroutineID) + " already tried to lock this mutex. Deadlock.")
	}
	m.Mutex.Lock()
}

func (m *Mutex) Unlock() {
	currGoroutineID := getGoroutineID()
	m.Mutex.Unlock()
	delete(m.Accessors, currGoroutineID)
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
