package idmutex

import (
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

type MutexedStruct struct {
	Mutex
}

func TestIdMutexDoesNotPanic(t *testing.T) {
	foo := &MutexedStruct{}
	go func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("The code did panic")
			}
		}()
		foo.Lock()
		defer foo.Unlock()
	}()
	time.Sleep(1 * time.Second)
}
func TestIdMutexDoesPanic(t *testing.T) {
	foo := &MutexedStruct{}
	go func() {
		defer func() {
			if r := recover(); r == nil {
				t.Errorf("The code did not panic")
			}
		}()
		foo.Lock()
		defer foo.Unlock()
		foo.Lock()
		defer foo.Unlock()
	}()
	time.Sleep(1 * time.Second)
}

func TestIdMutexUnlockAfterPanic(t *testing.T) {
	go func() {
		time.Sleep(5 * time.Second)
		assert.Fail(t, "Did not complete, potential deadlock")
	}()
	foo := &MutexedStruct{}
	go func() {
		foo.Lock()
		// some code here...
		defer func() {
			recover()
		}()
		defer foo.Unlock()
		foo.Lock()
		defer foo.Unlock()
		panic("Panicking...")
	}()
	time.Sleep(1 * time.Second)
	foo.Lock()
	defer foo.Unlock()

	// reached here without deadlock
	assert.True(t, true)
}
