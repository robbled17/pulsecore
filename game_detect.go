package main

import (
	"path/filepath"
	"strings"
)

func (a *App) isGameExcluded(base string) bool {
	if base == "" {
		return true
	}
	st := a.ensureStats()
	st.mu.Lock()
	defer st.mu.Unlock()
	for _, ex := range st.store.GameExclusions {
		if strings.ToLower(filepath.Base(ex)) == base {
			return true
		}
	}
	return false
}
