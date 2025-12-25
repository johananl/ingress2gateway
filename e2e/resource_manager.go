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
	rm.mu.Lock()
	defer rm.mu.Unlock()

	state, exists := rm.resources[key]
	if !exists {
		state = &resourceState{
			ready: make(chan struct{}),
			count: 0,
		}
		rm.resources[key] = state

		// Run installation asynchronously.
		go func() {
			defer close(state.ready)
			state.cleanup = install()
		}()
	}
	state.count++

	var once sync.Once // Protect against multiple cleanups by same caller
	var done <-chan struct{}

	return Resource{
		Cleanup: func() <-chan struct{} {
			once.Do(func() {
				done = rm.release(key)
			})
			return done
		},
		Wait: func() {
			<-state.ready
		},
	}
}

// Decrements the reference count for a resource and triggers cleanup when the count
// reaches zero. It returns a channel that is closed when cleanup completes.
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
			delete(rm.resources, key)
			rm.mu.Unlock()
			// Wait for installation to complete before cleanup, then run cleanup outside the lock.
			// TODO: Potential race. Another caller could try to acquire here.
			<-state.ready
			if state.cleanup != nil {
				state.cleanup()
			}
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
	// Wait blocks until the resource is installed and ready for use.
	Wait func()
}

// InstallFunc is a synchronous install function which returns a synchronous cleanup function.
type InstallFunc func() CleanupFunc

// CleanupFunc is a function which contains logic for cleaning up a resource.
type CleanupFunc func()

// Tracks a shared resource's state.
type resourceState struct {
	cleanup CleanupFunc
	ready   chan struct{} // Closed when installation completes
	count   int           // Reference count
}
