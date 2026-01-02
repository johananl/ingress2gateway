package e2e

import (
	"sync"
)

var globalResourceManager = &ResourceManager{
	resources: make(map[string]*resourceState),
}

// ResourceManager manages shared resources used by tests. It allows safe reuse of resources which
// have expensive setup and/or teardown by multiple concurrent tests.
type ResourceManager struct {
	// The methods on this type are designed to return immediately: Any long-running operation
	// should run asynchronously. The mutex is used only for thread-safe access to the internal
	// state and is NOT designed to remain locked while a long-running resource operation is
	// executing.
	mu        sync.Mutex
	resources map[string]*resourceState
}

// Acquire returns a shared resource identified by key.
//
// If the resource does not exist, install is called asynchronously to create it. The returned
// Resource allows callers to wait for installation to complete and to trigger cleanup. Subsequent calls with the same key return
// immediately without calling install again.
//
// Each caller MUST call the Cleanup() method on the returned Resource to ensure resource release
// takes place.
func (rm *ResourceManager) Acquire(key string, install InstallFunc) Resource {
	for {
		rm.mu.Lock()
		state, exists := rm.resources[key]
		if exists && state.cleaningUp != nil {
			// Resource is being cleaned up - wait and retry.
			cleaningUp := state.cleaningUp
			rm.mu.Unlock()
			<-cleaningUp
			continue
		}

		if !exists {
			state = &resourceState{
				ready: make(chan struct{}),
				count: 0,
			}
			rm.resources[key] = state

			// Run installation asynchronously.
			go func() {
				defer close(state.ready)
				cleanup, err := install()
				if err != nil {
					state.err = err
					return
				}
				state.cleanup = cleanup
			}()
		}
		state.count++
		rm.mu.Unlock()

		var once sync.Once // Protect against multiple cleanups by same caller
		var done <-chan struct{}

		return Resource{
			Cleanup: func() <-chan struct{} {
				once.Do(func() {
					done = rm.release(key)
				})
				return done
			},
			Wait: func() error {
				<-state.ready
				return state.err
			},
		}
	}
}

// Decrements the reference count for a resource and triggers cleanup when the count reaches zero.
// Returns a channel that is closed when cleanup completes.
func (rm *ResourceManager) release(key string) <-chan struct{} {
	done := make(chan struct{})

	go func() {
		defer close(done)

		rm.mu.Lock()
		state, ok := rm.resources[key]
		if !ok {
			rm.mu.Unlock()
			return
		}

		state.count--
		if state.count <= 0 {
			// Mark the resource as cleaning up before releasing the lock. This prevents new
			// Acquire calls from using a resource that is being cleaned up.
			state.cleaningUp = make(chan struct{})
			rm.mu.Unlock()

			// Wait for installation to complete before running cleanup.
			<-state.ready
			if state.cleanup != nil {
				state.cleanup()
			}

			// Remove the resource from the map and signal cleanup is done.
			rm.mu.Lock()
			delete(rm.resources, key)
			close(state.cleaningUp)
			rm.mu.Unlock()
		} else {
			rm.mu.Unlock()
		}
	}()

	return done
}

// Resource represents a resource managed by the ResourceManager.
type Resource struct {
	// Cleanup releases the resource's underlying resources.
	Cleanup func() <-chan struct{}
	// Wait blocks until the resource is installed and ready for use. If there was an error during
	// the installation, the error is returned.
	Wait func() error
}

// InstallFunc is a synchronous install function which returns a synchronous cleanup function or an
// installation error.
type InstallFunc func() (CleanupFunc, error)

// CleanupFunc is a function which contains logic for cleaning up a resource.
type CleanupFunc func()

// Tracks a shared resource's state.
type resourceState struct {
	cleanup    CleanupFunc
	ready      chan struct{} // Closed when installation completes
	cleaningUp chan struct{} // Closed when cleanup completes
	err        error         // An installation error
	count      int           // Reference count
}
