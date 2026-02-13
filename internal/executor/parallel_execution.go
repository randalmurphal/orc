package executor

import (
	"maps"
	"sync"
)

// safeVars provides thread-safe access to a map[string]string.
// It is used by parallel execution tests to validate concurrent map safety.
type safeVars struct {
	mu   sync.RWMutex
	vars map[string]string
}

func newSafeVars() *safeVars {
	return &safeVars{
		vars: make(map[string]string),
	}
}

func (sv *safeVars) Set(key, value string) {
	sv.mu.Lock()
	defer sv.mu.Unlock()
	sv.vars[key] = value
}

func (sv *safeVars) Get(key string) string {
	sv.mu.RLock()
	defer sv.mu.RUnlock()
	return sv.vars[key]
}

func (sv *safeVars) Clone() map[string]string {
	sv.mu.RLock()
	defer sv.mu.RUnlock()
	result := make(map[string]string, len(sv.vars))
	maps.Copy(result, sv.vars)
	return result
}
