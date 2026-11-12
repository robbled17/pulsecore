package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// EcoLimitedGroup is one EXE family currently limited by PulseCore.
type EcoLimitedGroup struct {
	Name         string  `json:"name"`
	FriendlyName string  `json:"friendlyName"`
	Count        int     `json:"count"`
	PIDs         []int32 `json:"pids"`
	AppliedAt    string  `json:"appliedAt"`
	SessionID    string  `json:"sessionId"`
}

type persistedEcoPID struct {
	PID      int32  `json:"pid"`
	Name     string `json:"name"`
	Priority uint32 `json:"priority"`
}

type persistedEcoSession struct {
	ID   string            `json:"id"`
	At   time.Time         `json:"at"`
	PIDs []persistedEcoPID `json:"pids"`
}

type persistedEcoFile struct {
	Sessions []persistedEcoSession `json:"sessions"`
}

func ecoStatePath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "PulseCore", "eco_active.json"), nil
}

func (a *App) persistEcoState() {
	ecoSessionMu.Lock()
	defer ecoSessionMu.Unlock()
	path, err := ecoStatePath()
	if err != nil {
		return
	}
	file := persistedEcoFile{Sessions: make([]persistedEcoSession, 0)}
	for id, sess := range ecoSessions {
		if sess == nil || sess.Reverted {
			continue
		}
		alive := make([]persistedEcoPID, 0, len(sess.PIDs))
		for _, p := range sess.PIDs {
			if processStillAlive(p.PID) {
				alive = append(alive, persistedEcoPID{PID: p.PID, Name: p.Name, Priority: p.Priority})
			}
		}
		if len(alive) == 0 {
			continue
		}
		file.Sessions = append(file.Sessions, persistedEcoSession{ID: id, At: sess.At, PIDs: alive})
	}
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	raw, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return
	}
	_ = os.Rename(tmp, path)
}

func (a *App) loadEcoState() {
	path, err := ecoStatePath()
	if err != nil {
		return
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var file persistedEcoFile
	if json.Unmarshal(raw, &file) != nil {
		return
	}
	ecoSessionMu.Lock()
	defer ecoSessionMu.Unlock()
	if ecoSessions == nil {
		ecoSessions = map[string]*ecoSession{}
	}
	a.mu.Lock()
	if a.ecoPIDs == nil {
		a.ecoPIDs = map[int32]bool{}
	}
	for _, s := range file.Sessions {
		pids := make([]ecoPIDState, 0, len(s.PIDs))
		for _, p := range s.PIDs {
			if !processStillAlive(p.PID) {
				continue
			}
			pids = append(pids, ecoPIDState{PID: p.PID, Name: p.Name, Priority: p.Priority})
			a.ecoPIDs[p.PID] = true
		}
		if len(pids) == 0 {
			continue
		}
		ecoSessions[s.ID] = &ecoSession{ID: s.ID, At: s.At, PIDs: pids}
	}
	a.mu.Unlock()
}

// GetEcoLimitedGroups returns active Eco groups for the Lag tab panel.
func (a *App) GetEcoLimitedGroups() []EcoLimitedGroup {
	a.pruneDeadEcoPIDs()
	ecoSessionMu.Lock()
	defer ecoSessionMu.Unlock()

	type agg struct {
		name   string
		pids   map[int32]struct{}
		at     time.Time
		sessID string
	}
	by := map[string]*agg{}
	for id, sess := range ecoSessions {
		if sess == nil || sess.Reverted {
			continue
		}
		for _, p := range sess.PIDs {
			if !processStillAlive(p.PID) {
				continue
			}
			key := strings.ToLower(filepath.Base(p.Name))
			if key == "" {
				continue
			}
			g := by[key]
			if g == nil {
				g = &agg{name: p.Name, pids: map[int32]struct{}{}, at: sess.At, sessID: id}
				by[key] = g
			}
			g.pids[p.PID] = struct{}{}
			if sess.At.Before(g.at) {
				g.at = sess.At
				g.sessID = id
			}
			if g.name == "" {
				g.name = p.Name
			}
		}
	}
	out := make([]EcoLimitedGroup, 0, len(by))
	for _, g := range by {
		pids := make([]int32, 0, len(g.pids))
		for pid := range g.pids {
			pids = append(pids, pid)
		}
		sort.Slice(pids, func(i, j int) bool { return pids[i] < pids[j] })
		out = append(out, EcoLimitedGroup{
			Name:         g.name,
			FriendlyName: friendlyProcessName(g.name),
			Count:        len(pids),
			PIDs:         pids,
			AppliedAt:    g.at.Format("15:04:05"),
			SessionID:    g.sessID,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i].FriendlyName) < strings.ToLower(out[j].FriendlyName)
	})
	return out
}

func (a *App) pruneDeadEcoPIDs() {
	changed := false
	ecoSessionMu.Lock()
	for _, sess := range ecoSessions {
		if sess == nil || sess.Reverted {
			continue
		}
		alive := make([]ecoPIDState, 0, len(sess.PIDs))
		for _, p := range sess.PIDs {
			if processStillAlive(p.PID) {
				alive = append(alive, p)
			} else {
				changed = true
			}
		}
		if len(alive) == 0 {
			sess.Reverted = true
			sess.PIDs = nil
			changed = true
		} else if len(alive) != len(sess.PIDs) {
			sess.PIDs = alive
			changed = true
		}
	}
	ecoSessionMu.Unlock()

	a.mu.Lock()
	live := map[int32]bool{}
	ecoSessionMu.Lock()
	for _, sess := range ecoSessions {
		if sess == nil || sess.Reverted {
			continue
		}
		for _, p := range sess.PIDs {
			live[p.PID] = true
		}
	}
	ecoSessionMu.Unlock()
	for pid := range a.ecoPIDs {
		if !live[pid] {
			delete(a.ecoPIDs, pid)
			changed = true
		}
	}
	a.mu.Unlock()

	if changed {
		a.persistEcoState()
	}
}

// RevertEcoGroup restores all limited PIDs for one EXE name.
func (a *App) RevertEcoGroup(name string) EcoApplyResult {
	key := strings.ToLower(filepath.Base(strings.TrimSpace(name)))
	if key == "" {
		return EcoApplyResult{OK: false, MessageRu: "Имя процесса пустое", MessageEn: "Empty process name"}
	}
	groups := a.GetEcoLimitedGroups()
	var target *EcoLimitedGroup
	for i := range groups {
		if strings.ToLower(filepath.Base(groups[i].Name)) == key {
			target = &groups[i]
			break
		}
	}
	if target == nil {
		return EcoApplyResult{OK: false, MessageRu: "Ограничений для этой программы нет", MessageEn: "No limits for this app"}
	}
	return a.revertPIDs(target.PIDs, true)
}

// RevertAllEco restores every PulseCore Eco limitation.
func (a *App) RevertAllEco() EcoApplyResult {
	groups := a.GetEcoLimitedGroups()
	pids := make([]int32, 0)
	for _, g := range groups {
		pids = append(pids, g.PIDs...)
	}
	if len(pids) == 0 {
		return EcoApplyResult{
			OK:        true,
			MessageRu: "Активных ограничений нет",
			MessageEn: "No active limits",
		}
	}
	return a.revertPIDs(pids, true)
}

func (a *App) revertPIDs(pids []int32, markSessions bool) EcoApplyResult {
	restored := 0
	skippedDead := 0
	failed := make([]EcoFail, 0)

	// Build priority lookup from sessions.
	prioByPID := map[int32]uint32{}
	nameByPID := map[int32]string{}
	ecoSessionMu.Lock()
	for _, sess := range ecoSessions {
		if sess == nil {
			continue
		}
		for _, p := range sess.PIDs {
			prioByPID[p.PID] = p.Priority
			if p.Name != "" {
				nameByPID[p.PID] = p.Name
			}
		}
	}
	ecoSessionMu.Unlock()

	seen := map[int32]struct{}{}
	for _, pid := range pids {
		if _, ok := seen[pid]; ok {
			continue
		}
		seen[pid] = struct{}{}
		name := nameByPID[pid]
		if name == "" {
			name = processNameByPID(int(pid))
		}
		if !processStillAlive(pid) {
			skippedDead++
			a.mu.Lock()
			delete(a.ecoPIDs, pid)
			a.mu.Unlock()
			continue
		}
		// Always attempt to clear EcoQoS + restore priority. Do not abort if
		// the process already looks "normal" - still clear our bookkeeping.
		_ = a.ToggleEcoMode(int(pid), false)
		want := prioByPID[pid]
		if want == 0 || want == idlePriorityClass {
			want = normalPriorityClass
		}
		if err := setPriorityClass(uint32(pid), want); err != nil {
			// Non-fatal if EcoQoS already cleared.
			_ = err
		}
		on, prio, _ := readProcessEcoState(uint32(pid))
		if on || prio == idlePriorityClass {
			// Force another clear attempt.
			_ = a.ToggleEcoMode(int(pid), false)
			_ = setPriorityClass(uint32(pid), normalPriorityClass)
			on2, prio2, _ := readProcessEcoState(uint32(pid))
			if on2 || prio2 == idlePriorityClass {
				failed = append(failed, EcoFail{Name: name, PID: pid, Reason: "не удалось снять ограничение"})
				continue
			}
		}
		a.mu.Lock()
		delete(a.ecoPIDs, pid)
		for i := range a.vampires {
			for _, vp := range a.vampires[i].PIDs {
				if vp == pid {
					a.vampires[i].EcoMode = false
					a.vampires[i].Frozen = false
				}
			}
			if a.vampires[i].PID == pid {
				a.vampires[i].EcoMode = false
				a.vampires[i].Frozen = false
			}
		}
		a.mu.Unlock()
		restored++
	}

	if markSessions {
		ecoSessionMu.Lock()
		for _, sess := range ecoSessions {
			if sess == nil || sess.Reverted {
				continue
			}
			left := make([]ecoPIDState, 0, len(sess.PIDs))
			for _, p := range sess.PIDs {
				if _, gone := seen[p.PID]; gone {
					continue
				}
				if processStillAlive(p.PID) {
					left = append(left, p)
				}
			}
			sess.PIDs = left
			if len(left) == 0 {
				sess.Reverted = true
			}
		}
		ecoSessionMu.Unlock()
	}
	a.persistEcoState()

	res := EcoApplyResult{
		OK:           restored > 0 || (restored == 0 && skippedDead > 0 && len(failed) == 0),
		AppliedCount: restored,
		Failed:       failed,
		CanRevert:    false,
		Confirmed:    restored > 0,
	}
	if restored > 0 {
		res.MessageRu = fmt.Sprintf("Возвращено %d процессов", restored)
		res.MessageEn = fmt.Sprintf("Restored %d process(es)", restored)
		if skippedDead > 0 {
			res.MessageRu += fmt.Sprintf(" (%d уже завершились)", skippedDead)
			res.MessageEn += fmt.Sprintf(" (%d already exited)", skippedDead)
		}
	} else if skippedDead > 0 && len(failed) == 0 {
		res.OK = true
		res.MessageRu = "Ограничения сняты: процессы уже завершились"
		res.MessageEn = "Limits cleared: processes already exited"
	} else {
		res.MessageRu = "Не удалось вернуть процессы"
		res.MessageEn = "Could not restore processes"
	}
	if len(failed) > 0 {
		res.MessageRu += ". " + formatEcoFailsRu(failed)
		res.MessageEn += ". " + formatEcoFailsEn(failed)
	}
	return res
}

// HasActiveEcoLimits reports whether PulseCore currently tracks any Eco PIDs.
func (a *App) HasActiveEcoLimits() bool {
	return len(a.GetEcoLimitedGroups()) > 0
}

// handleEcoOnClose asks / auto-clears Eco limits before quitting.
// Returns true if close should be prevented (frontend will show a prompt).
func (a *App) handleEcoOnClose() bool {
	if !a.HasActiveEcoLimits() {
		return false
	}
	a.mu.RLock()
	policy := a.prefs.EcoExitPolicy
	prompted := a.ecoExitConfirmed
	a.mu.RUnlock()
	if prompted {
		return false
	}
	if policy == "" {
		policy = "ask"
	}
	switch policy {
	case "clear":
		_ = a.RevertAllEco()
		return false
	case "keep":
		a.persistEcoState()
		return false
	default: // ask via frontend modal
		if a.ctx != nil {
			runtime.EventsEmit(a.ctx, "eco:exit-prompt", true)
		}
		return true
	}
}

// ConfirmQuitAfterEco finishes the exit prompt: clear=true снимает Eco, затем выходит.
func (a *App) ConfirmQuitAfterEco(clear bool) {
	if clear {
		_ = a.RevertAllEco()
	} else {
		a.persistEcoState()
	}
	a.mu.Lock()
	a.ecoExitConfirmed = true
	a.forceQuit = true
	a.mu.Unlock()
	a.finishQuit()
}
