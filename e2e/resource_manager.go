package e2e

import (
	"sync"
)

type ResourceManager struct {
	mu        sync.Mutex
	resources map[string]*resourceState
}

type resourceState struct {
	cleanup func()
	count   int
}

var globalResourceManager = &ResourceManager{
	resources: make(map[string]*resourceState),
}

// Acquire acquires a resource. If it's not initialized, installFunc is called.
// installFunc should return a cleanup function.
// Returns a release function that should be called when the resource is no longer needed.
func (rm *ResourceManager) Acquire(key string, installFunc func() func()) func() {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	state, ok := rm.resources[key]
	if !ok {
		cleanup := installFunc()
		state = &resourceState{
			cleanup: cleanup,
			count:   0,
		}
		rm.resources[key] = state
	}
	state.count++

	return func() {
		rm.Release(key)
	}
}

func (rm *ResourceManager) Release(key string) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	state, ok := rm.resources[key]
	if !ok {
		// Should not happen if used correctly.
		return
	}

	state.count--
	if state.count <= 0 {
		if state.cleanup != nil {
			state.cleanup()
		}
		delete(rm.resources, key)
	}
}
