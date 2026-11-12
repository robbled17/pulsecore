package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type dismissedProblemsStore struct {
	IDs map[string]int64 `json:"ids"` // problemID -> unix dismiss time
}

type dismissedProblemsEngine struct {
	mu     sync.Mutex
	store  dismissedProblemsStore
	loaded bool
}

func dismissedProblemsPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "PulseCore", "dismissed_problems.json"), nil
}

func (a *App) ensureDismissedProblems() *dismissedProblemsEngine {
	if a.dismissedProblems == nil {
		a.dismissedProblems = &dismissedProblemsEngine{
			store: dismissedProblemsStore{IDs: map[string]int64{}},
		}
	}
	return a.dismissedProblems
}

func (a *App) loadDismissedProblems() {
	e := a.ensureDismissedProblems()
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.loaded {
		return
	}
	e.loaded = true
	path, err := dismissedProblemsPath()
	if err != nil {
		return
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		e.store.IDs = map[string]int64{}
		return
	}
	var file dismissedProblemsStore
	if json.Unmarshal(raw, &file) != nil || file.IDs == nil {
		e.store.IDs = map[string]int64{}
		return
	}
	e.store = file
}

func (a *App) saveDismissedProblems() {
	e := a.ensureDismissedProblems()
	e.mu.Lock()
	defer e.mu.Unlock()
	path, err := dismissedProblemsPath()
	if err != nil {
		return
	}
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	raw, err := json.MarshalIndent(e.store, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(path, raw, 0o644)
}

// DismissHomeProblem hides a home warning until conditions clear (or user clears).
func (a *App) DismissHomeProblem(problemID string) bool {
	problemID = strings.TrimSpace(problemID)
	if problemID == "" {
		return false
	}
	a.loadDismissedProblems()
	e := a.ensureDismissedProblems()
	e.mu.Lock()
	if e.store.IDs == nil {
		e.store.IDs = map[string]int64{}
	}
	e.store.IDs[problemID] = time.Now().Unix()
	e.mu.Unlock()
	a.saveDismissedProblems()
	return true
}

func (a *App) isProblemDismissed(problemID string) bool {
	a.loadDismissedProblems()
	e := a.ensureDismissedProblems()
	e.mu.Lock()
	defer e.mu.Unlock()
	_, ok := e.store.IDs[problemID]
	return ok
}

func (a *App) clearDismissedProblem(problemID string) {
	a.loadDismissedProblems()
	e := a.ensureDismissedProblems()
	e.mu.Lock()
	delete(e.store.IDs, problemID)
	e.mu.Unlock()
	a.saveDismissedProblems()
}

// filterDismissedProblems drops dismissed items; auto-clears disk_full when below 80%.
func (a *App) filterDismissedProblems(probs []HomeProblem, sysUsed float64, sysOK bool) []HomeProblem {
	if sysOK && sysUsed < 80 {
		a.clearDismissedProblem("disk_full")
	}
	out := make([]HomeProblem, 0, len(probs))
	for _, p := range probs {
		if a.isProblemDismissed(p.ID) {
			continue
		}
		out = append(out, p)
	}
	return out
}
