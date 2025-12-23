package e2e

import (
	"sync"
)

var globalResourceManager = &ResourceManager{
	resources: make(map[string]*resourceState),
}

type resourceState struct {
	cleanup func() <-chan struct{}
	wait    WaitFunc
	count   int
}

// CleanupFunc is a function which contains logic for cleaning up a resource.
type CleanupFunc func()

// WaitFunc is a function which contains logic for waiting for a resource while its being created.
type WaitFunc func()

// Resource represents a resource managed by the ResourceManager.
type Resource struct {
	Cleanup func() <-chan struct{}
	Wait    WaitFunc
}

// CreateFunc is a function which contains logic for creating a resource.
type CreateFunc func() Resource

// SyncInstallFunc is a synchronous install function that returns a synchronous cleanup function.
type SyncInstallFunc func() CleanupFunc

// ResourceManager manages shared resources used by tests. It allows safe reuse of resource with an
// expensive setup and/or teardown by multiple, concurrent tests.
type ResourceManager struct {
	mu        sync.Mutex
	resources map[string]*resourceState
}

// Acquire returns a shared resource identified by key. If the resource does not exist, createFunc
// is called to create it. The returned Resource is a handle for the underlying resource which
// allows callers to wait for the resource as well as clean it up. Subsequent calls with the same
// key return the Resource immediately without re-creating the resource.
func (rm *ResourceManager) Acquire(key string, install SyncInstallFunc) Resource {
	return rm.acquire(key, func() Resource {
		return asyncInstall(install)()
	})
}

func (rm *ResourceManager) acquire(key string, createFunc CreateFunc) Resource {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	state, ok := rm.resources[key]
	if !ok {
		resource := createFunc()
		state = &resourceState{
			cleanup: resource.Cleanup,
			wait:    resource.Wait,
			count:   0,
		}
		rm.resources[key] = state
	}
	state.count++

	return Resource{
		Cleanup: func() <-chan struct{} {
			return rm.releaseAsync(key)
		},
		Wait: state.wait,
	}
}

func (rm *ResourceManager) releaseAsync(key string) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		rm.release(key)
	}()
	return done
}

func (rm *ResourceManager) release(key string) {
	rm.mu.Lock()

	state, ok := rm.resources[key]
	if !ok {
		rm.mu.Unlock()
		// Should not happen if used correctly.
		return
	}

	state.count--
	if state.count <= 0 {
		delete(rm.resources, key)
		rm.mu.Unlock()
		// Run cleanup outside the lock to allow parallel cleanups.
		if state.cleanup != nil {
			state.cleanup()
		}
	} else {
		rm.mu.Unlock()
	}
}

// Wraps a synchronous install function to make it async.
func asyncInstall(install SyncInstallFunc) CreateFunc {
	return func() Resource {
		done := make(chan struct{})
		var cleanup CleanupFunc
		go func() {
			defer close(done)
			cleanup = install()
		}()

		return Resource{
			Cleanup: func() <-chan struct{} {
				cleanupDone := make(chan struct{})
				go func() {
					defer close(cleanupDone)
					<-done
					if cleanup != nil {
						cleanup()
					}
				}()
				return cleanupDone
			},
			Wait: func() {
				<-done
			},
		}
	}
}
