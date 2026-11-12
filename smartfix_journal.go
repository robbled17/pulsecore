package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const smartFixJournalMax = 50

// SmartFixJournalStep is one applied change with enough Prev to revert.
type SmartFixJournalStep struct {
	StepID     string `json:"stepId"`
	ScenarioID string `json:"scenarioId"`
	ActionID   string `json:"actionId"`
	ActionArg  string `json:"actionArg"`
	Reversible bool   `json:"reversible"`
	Kind       string `json:"kind"` // cleanup|startup|eco|power|settings|game_mode|ram
	PrevJSON   string `json:"prevJson"`
	AfterJSON  string `json:"afterJson,omitempty"`
	OK         bool   `json:"ok"`
	Detail     string `json:"detail,omitempty"`
}

// SmartFixJournalSession groups steps from one ApplySmartFix call.
type SmartFixJournalSession struct {
	ID           string                `json:"id"`
	At           string                `json:"at"`
	ScoreBefore  int                   `json:"scoreBefore"`
	ScoreAfter   int                   `json:"scoreAfter"`
	FreedBytes   int64                 `json:"freedBytes"`
	FreedRAMMB   float64               `json:"freedRAMMB"`
	Steps        []SmartFixJournalStep `json:"steps"`
	Reverted     bool                  `json:"reverted"`
	RevertedAt   string                `json:"revertedAt,omitempty"`
}

type smartFixJournalStore struct {
	Sessions []SmartFixJournalSession `json:"sessions"`
}

type smartFixJournalEngine struct {
	mu     sync.Mutex
	store  smartFixJournalStore
	loaded bool
	dirty  bool
}

func smartFixJournalPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "PulseCore", "smartfix_journal.json"), nil
}

func (a *App) ensureSmartFixJournal() *smartFixJournalEngine {
	if a.smartFixJournal == nil {
		a.smartFixJournal = &smartFixJournalEngine{store: smartFixJournalStore{}}
	}
	return a.smartFixJournal
}

func (a *App) loadSmartFixJournal() {
	j := a.ensureSmartFixJournal()
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.loaded {
		return
	}
	j.loaded = true
	path, err := smartFixJournalPath()
	if err != nil {
		return
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var file smartFixJournalStore
	if json.Unmarshal(raw, &file) != nil {
		return
	}
	j.store = file
	j.pruneLocked()
}

func (j *smartFixJournalEngine) pruneLocked() {
	if len(j.store.Sessions) <= smartFixJournalMax {
		return
	}
	j.store.Sessions = j.store.Sessions[len(j.store.Sessions)-smartFixJournalMax:]
}

func (a *App) flushSmartFixJournal() {
	j := a.ensureSmartFixJournal()
	j.mu.Lock()
	defer j.mu.Unlock()
	if !j.dirty {
		return
	}
	path, err := smartFixJournalPath()
	if err != nil {
		return
	}
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	j.pruneLocked()
	raw, err := json.MarshalIndent(j.store, "", "  ")
	if err != nil {
		return
	}
	if os.WriteFile(path, raw, 0o644) == nil {
		j.dirty = false
	}
}

func (a *App) appendSmartFixSession(sess SmartFixJournalSession) {
	a.loadSmartFixJournal()
	j := a.ensureSmartFixJournal()
	j.mu.Lock()
	defer j.mu.Unlock()
	if sess.At == "" {
		sess.At = time.Now().Format(time.RFC3339)
	}
	j.store.Sessions = append(j.store.Sessions, sess)
	j.pruneLocked()
	j.dirty = true
}

func (a *App) getSmartFixSession(id string) (SmartFixJournalSession, bool) {
	a.loadSmartFixJournal()
	j := a.ensureSmartFixJournal()
	j.mu.Lock()
	defer j.mu.Unlock()
	for i := range j.store.Sessions {
		if j.store.Sessions[i].ID == id {
			return j.store.Sessions[i], true
		}
	}
	return SmartFixJournalSession{}, false
}

func (a *App) markSmartFixSessionReverted(id string) {
	a.loadSmartFixJournal()
	j := a.ensureSmartFixJournal()
	j.mu.Lock()
	defer j.mu.Unlock()
	for i := range j.store.Sessions {
		if j.store.Sessions[i].ID == id {
			j.store.Sessions[i].Reverted = true
			j.store.Sessions[i].RevertedAt = time.Now().Format(time.RFC3339)
			j.dirty = true
			return
		}
	}
}

// GetLastRevertibleSmartFix returns the newest non-reverted session that has reversible steps.
func (a *App) GetLastRevertibleSmartFix() SmartFixJournalSession {
	a.loadSmartFixJournal()
	j := a.ensureSmartFixJournal()
	j.mu.Lock()
	defer j.mu.Unlock()
	for i := len(j.store.Sessions) - 1; i >= 0; i-- {
		s := j.store.Sessions[i]
		if s.Reverted {
			continue
		}
		for _, st := range s.Steps {
			if st.Reversible && st.OK {
				return s
			}
		}
	}
	return SmartFixJournalSession{}
}

func newSmartFixSessionID() string {
	return time.Now().Format("20060102-150405") + "-" + shortRand(4)
}

func shortRand(n int) string {
	const alphabet = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	now := uint64(time.Now().UnixNano())
	mod := uint64(len(alphabet))
	for i := 0; i < n; i++ {
		now = now*1103515245 + 12345
		b[i] = alphabet[now%mod]
	}
	return string(b)
}
