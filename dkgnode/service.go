package dkgnode

import (
	"sync/atomic"

	logging "github.com/sirupsen/logrus"
)

// ServiceCore is a service that is implemented by a user.
type ServiceCore interface {
	Name() string
	OnStart() error
	OnStop() error
	Call(method string, args ...interface{}) (result interface{}, err error)
	SetBaseService(*BaseService)
}

// Service represents a way to start, stop and get the status of some service. BaseService is the
// default implementation and should be used by most users.
// SetServiceCore allows a user to set a ServiceCore. Users should implement their logic using
// ServiceCore.
type Service interface {
	Start() error
	Stop() error
	IsRunning() bool
	SetServiceCore(ServiceCore)
	String() string
}

/*
Services can be started and then stopped.

Users provide an implementation of ServiceCore and BaseService guarantees that OnStart and OnStop
are called at most once.
Starting an already started service will panic.
Stopping an already stopped (or non-started) service will panic.

Usage:

	// Implement ServiceCore through OnStart() and OnStop().
	type FooServiceCore struct {
		// private fields
	}

	func (fs *FooServiceCore) OnStart() error {
		// initialize private fields
		// start subroutines, etc.
	}

	func (fs *FooServiceCore) OnStop() error {
		// close/destroy private fields
		// stop subroutines, etc.
	}

	fs := NewBaseService("MyAwesomeService", &FooServiceCore{})
	fs.Start() // this calls OnStart()
	fs.Stop() // this calls OnStop()
*/

// BaseService provides the guarantees that a ServiceCore can only be started and stopped once.
type BaseService struct {
	name    string
	start   uint32 // atomic
	started uint32 // atomic
	stopped uint32 // atomic
	quit    chan struct{}

	// The "subclass" of BaseService
	impl ServiceCore
}

// NewBaseService returns a base service that wraps an implementation of ServiceCore and handles
// starting and stopping.
func NewBaseService(impl ServiceCore) *BaseService {
	bs := &BaseService{
		name: impl.Name(),
		quit: make(chan struct{}),
		impl: impl,
	}
	bs.impl.SetBaseService(bs)
	return bs
}

func (bs *BaseService) Name() string {
	return bs.impl.Name()
}

// Query implements Service
func (bs *BaseService) Call(method string, args ...interface{}) (interface{}, error) {
	return bs.impl.Call(method, args...)
}

// Start implements Service
func (bs *BaseService) Start() (bool, error) {
	if atomic.CompareAndSwapUint32(&bs.start, 0, 1) {
		if atomic.LoadUint32(&bs.stopped) == 1 {
			logging.WithField("bsname", bs.name).Info("not starting baseservice -- already stopped")
			return false, nil
		} else {
			logging.WithField("bsname", bs.name).Info("starting service")
		}
		err := bs.impl.OnStart()
		if err != nil {
			// revert flag
			atomic.StoreUint32(&bs.start, 0)
			return false, err
		}
		atomic.StoreUint32(&bs.started, 1)
		return true, err
	} else {
		logging.WithField("bsname", bs.name).Debug("not starting baseservice -- already stopped")
		return false, nil
	}
}

// Stop implements Service
func (bs *BaseService) Stop() bool {
	if atomic.CompareAndSwapUint32(&bs.stopped, 0, 1) {
		logging.WithField("bsname", bs.name).Info("stopping service")
		err := bs.impl.OnStop()
		if err != nil {
			logging.WithField("bsname", bs.impl.Name()).WithError(err).Error("could not stop baseservice")
		}
		close(bs.quit)
		return true
	} else {
		logging.WithField("bsname", bs.name).Debug("stopping baseservice (ignoring: already stopped)")
		return false
	}
}

// IsRunning implements Service
func (bs *BaseService) IsRunning() bool {
	return atomic.LoadUint32(&bs.started) == 1 && atomic.LoadUint32(&bs.stopped) == 0
}

// SetServiceCore implements SetServiceCore
func (bs *BaseService) SetServiceCore(service ServiceCore) {
	bs.impl = service
}

// String implements Service
func (bs *BaseService) String() string {
	return bs.name
}

func (bs *BaseService) Wait() {
	<-bs.quit
}
