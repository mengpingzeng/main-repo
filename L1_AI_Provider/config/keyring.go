package config

import (
	"encoding/json"
	"os"
	"sync"
)

type KeyRing struct {
	keys      []string
	idx       int
	mu        sync.Mutex
	statePath string
	label     string
}

func NewKeyRing(label string, keys []string, statePath string) *KeyRing {
	kr := &KeyRing{
		keys:      keys,
		statePath: statePath,
		label:     label,
	}
	kr.load()
	return kr
}

func (kr *KeyRing) load() {
	data, err := os.ReadFile(kr.statePath)
	if err != nil {
		return
	}
	var state map[string]int
	if json.Unmarshal(data, &state) != nil {
		return
	}
	if idx, ok := state[kr.label]; ok && idx >= 0 && idx < len(kr.keys) {
		kr.idx = idx
	}
}

func (kr *KeyRing) Count() int {
	return len(kr.keys)
}

func (kr *KeyRing) Next() string {
	kr.mu.Lock()
	defer kr.mu.Unlock()

	if len(kr.keys) == 0 {
		return ""
	}

	key := kr.keys[kr.idx]
	kr.idx = (kr.idx + 1) % len(kr.keys)
	kr.persist()
	return key
}

func (kr *KeyRing) persist() {
	existing := make(map[string]int)
	if data, err := os.ReadFile(kr.statePath); err == nil {
		json.Unmarshal(data, &existing)
	}
	existing[kr.label] = kr.idx

	dir := dirOf(kr.statePath)
	if dir != "" {
		os.MkdirAll(dir, 0755)
	}
	data, _ := json.MarshalIndent(existing, "", "  ")
	os.WriteFile(kr.statePath, data, 0600)
}

func dirOf(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' || path[i] == '\\' {
			return path[:i]
		}
	}
	return ""
}
