package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/shirou/gopsutil/v3/process"
	"golang.org/x/sys/windows"
)

const (
	ecoMeasureWait     = 5 * time.Second
	ecoCPUEffectMinPct = 1.5 // below this → "applied but soft / no noticeable effect"
	ecoRAMEffectMinMB  = 8
)

var (
	procGetProcessInformation = modKernel32.NewProc("GetProcessInformation")
	procGetPriorityClass      = modKernel32.NewProc("GetPriorityClass")
)

// EcoFail describes one process that could not be changed.
type EcoFail struct {
	Name   string `json:"name"`
	PID    int32  `json:"pid"`
	Reason string `json:"reason"`
}

// EcoApplyResult is the Stage-2 before/after feedback payload.
type EcoApplyResult struct {
	OK            bool      `json:"ok"`
	AppliedCount  int       `json:"appliedCount"`
	Failed        []EcoFail `json:"failed"`
	Confirmed     bool      `json:"confirmed"` // EcoQoS and/or IDLE priority verified
	MessageRu     string    `json:"messageRu"`
	MessageEn     string    `json:"messageEn"`
	CPUBefore     float64   `json:"cpuBefore"`
	CPUAfter      float64   `json:"cpuAfter"`
	RAMBeforeMB   float64   `json:"ramBeforeMB"`
	RAMAfterMB    float64   `json:"ramAfterMB"`
	EffectNoted   bool      `json:"effectNoted"`
	CanRevert     bool      `json:"canRevert"`
	SessionID     string    `json:"sessionId"`
	SoftEffect    bool      `json:"softEffect"`
}

type ecoPIDState struct {
	PID          int32
	Name         string
	Priority     uint32
	EcoWasOn     bool
	CPUBefore    float64
	RAMBeforeMB  float64
}

type ecoSession struct {
	ID        string
	At        time.Time
	PIDs      []ecoPIDState
	CPUBefore float64
	RAMBefore float64
	CPUAfter  float64
	RAMAfter  float64
	Reverted  bool
}

type ecoHistoryEntry struct {
	AppName     string    `json:"appName"`
	At          time.Time `json:"at"`
	RAMBeforeMB float64   `json:"ramBeforeMB"`
	RAMAfterMB  float64   `json:"ramAfterMB"`
	CPUBefore   float64   `json:"cpuBefore"`
	CPUAfter    float64   `json:"cpuAfter"`
	Result      string    `json:"result"`
	SessionID   string    `json:"sessionId"`
	CanRevert   bool      `json:"canRevert"`
}

var (
	ecoSessionMu sync.Mutex
	ecoSessions  = map[string]*ecoSession{}
	ecoHistory   []ecoHistoryEntry
)

// ApplyEcoWithFeedback enables Eco Mode, verifies OS state, measures before/after (≈5s).
func (a *App) ApplyEcoWithFeedback(name string) EcoApplyResult {
	return a.applyEcoMeasured(name, 0, true)
}

// ApplyEcoPIDWithFeedback enables Eco Mode for one PID with the same measurement path.
func (a *App) ApplyEcoPIDWithFeedback(pid int) EcoApplyResult {
	return a.applyEcoMeasured("", int32(pid), true)
}

// RevertEcoSession restores priorities/EcoQoS for a previous ApplyEcoWithFeedback session.
func (a *App) RevertEcoSession(sessionID string) EcoApplyResult {
	ecoSessionMu.Lock()
	sess := ecoSessions[sessionID]
	ecoSessionMu.Unlock()
	if sess == nil || sess.Reverted {
		// Fall back to reverting whatever is still tracked.
		return a.RevertAllEco()
	}
	pids := make([]int32, 0, len(sess.PIDs))
	for _, p := range sess.PIDs {
		pids = append(pids, p.PID)
	}
	res := a.revertPIDs(pids, true)
	res.SessionID = sessionID
	return res
}

func (a *App) applyEcoMeasured(name string, singlePID int32, enable bool) EcoApplyResult {
	if !enable {
		return EcoApplyResult{OK: false, MessageRu: "Ожидается включение Eco Mode", MessageEn: "Eco Mode enable expected"}
	}

	// Defender special path - soft success, no OpenProcess.
	if name != "" && isDefenderTarget(name) {
		_ = throttleDefenderScanCPU(15)
		return EcoApplyResult{
			OK:           true,
			AppliedCount: 1,
			Confirmed:    true,
			SoftEffect:   true,
			EffectNoted:  false,
			MessageRu:    "Ограничение сканирования Защитника запрошено (до 15% CPU). Заметный эффект зависит от активного скана.",
			MessageEn:    "Defender scan budget capped (15% CPU). Noticeable effect depends on an active scan.",
		}
	}

	pids := resolveEcoPIDs(name, singlePID)
	if len(pids) == 0 {
		return EcoApplyResult{
			OK:        false,
			MessageRu: "Процесс не найден или уже завершён",
			MessageEn: "Process not found or already exited",
		}
	}

	beforeStates := make([]ecoPIDState, 0, len(pids))
	var cpuBefore, ramBefore float64
	for _, pid := range pids {
		st := samplePIDEco(pid)
		beforeStates = append(beforeStates, st)
		cpuBefore += st.CPUBefore
		ramBefore += st.RAMBeforeMB
	}

	applied := make([]ecoPIDState, 0, len(pids))
	failed := make([]EcoFail, 0)
	confirmed := 0

	for _, st := range beforeStates {
		n := st.Name
		if n == "" {
			n = name
		}
		if isEcoProtected(n) || isPPLProtected(n) {
			failed = append(failed, EcoFail{Name: n, PID: st.PID, Reason: "защищённый системный процесс"})
			continue
		}
		if err := a.ToggleEcoMode(int(st.PID), true); err != nil {
			failed = append(failed, EcoFail{Name: n, PID: st.PID, Reason: humanEcoErr(err)})
			continue
		}
		on, prio, _ := readProcessEcoState(uint32(st.PID))
		ok := on || prio == idlePriorityClass
		if ok {
			confirmed++
		}
		st.Name = n
		applied = append(applied, st)
	}

	if len(applied) == 0 {
		res := EcoApplyResult{OK: false, Failed: failed, AppliedCount: 0}
		res.MessageRu = "Не удалось применить Eco Mode"
		res.MessageEn = "Failed to apply Eco Mode"
		if len(failed) > 0 {
			res.MessageRu += ": " + formatEcoFailsRu(failed)
			res.MessageEn += ": " + formatEcoFailsEn(failed)
		}
		return res
	}

	// Measure after settling.
	time.Sleep(ecoMeasureWait)

	var cpuAfter, ramAfter float64
	stillAlive := 0
	for _, st := range applied {
		if !processStillAlive(st.PID) {
			continue
		}
		stillAlive++
		cur := samplePIDEco(st.PID)
		cpuAfter += cur.CPUBefore
		ramAfter += cur.RAMBeforeMB
	}

	cpuDelta := cpuBefore - cpuAfter
	ramDelta := ramBefore - ramAfter
	effect := (cpuDelta >= ecoCPUEffectMinPct) || (ramDelta >= ecoRAMEffectMinMB)
	soft := !effect

	sessionID := fmt.Sprintf("eco-%d", time.Now().UnixNano())
	sess := &ecoSession{
		ID:        sessionID,
		At:        time.Now(),
		PIDs:      applied,
		CPUBefore: cpuBefore,
		RAMBefore: ramBefore,
		CPUAfter:  cpuAfter,
		RAMAfter:  ramAfter,
	}
	ecoSessionMu.Lock()
	ecoSessions[sessionID] = sess
	ecoHistory = append([]ecoHistoryEntry{{
		AppName:     displayEcoName(name, applied),
		At:          sess.At,
		RAMBeforeMB: ramBefore,
		RAMAfterMB:  ramAfter,
		CPUBefore:   cpuBefore,
		CPUAfter:    cpuAfter,
		Result:      "applied",
		SessionID:   sessionID,
		CanRevert:   stillAlive > 0,
	}}, ecoHistory...)
	if len(ecoHistory) > 40 {
		ecoHistory = ecoHistory[:40]
	}
	ecoSessionMu.Unlock()
	a.persistEcoState()

	ramFreed := ramBefore - ramAfter
	if ramFreed < 0 {
		ramFreed = 0
	}
	a.recordStatsEco(ramFreed)

	res := EcoApplyResult{
		OK:           true,
		AppliedCount: len(applied),
		Failed:       failed,
		Confirmed:    confirmed > 0,
		CPUBefore:    cpuBefore,
		CPUAfter:     cpuAfter,
		RAMBeforeMB:  ramBefore,
		RAMAfterMB:   ramAfter,
		EffectNoted:  effect,
		SoftEffect:   soft,
		CanRevert:    stillAlive > 0,
		SessionID:    sessionID,
	}

	res.MessageRu = fmt.Sprintf("Режим эффективности применён к %d процессам", len(applied))
	res.MessageEn = fmt.Sprintf("Efficiency mode applied to %d process(es)", len(applied))
	if confirmed == 0 {
		res.MessageRu += " (подтвердить EcoQoS/приоритет не удалось)"
		res.MessageEn += " (could not verify EcoQoS/priority)"
	}
	if soft {
		res.MessageRu += ". Применено, но заметного эффекта нет (на этом процессоре режим эффективности действует мягко)"
		res.MessageEn += ". Applied, but no noticeable effect (efficiency mode is soft on this CPU)"
	} else {
		if ramDelta >= ecoRAMEffectMinMB {
			res.MessageRu += fmt.Sprintf(". Измерено: RAM %+.0f МБ, CPU %+.1f%%", -ramDelta, -cpuDelta)
			res.MessageEn += fmt.Sprintf(". Measured: RAM %+.0f MB, CPU %+.1f%%", -ramDelta, -cpuDelta)
		} else {
			res.MessageRu += fmt.Sprintf(". Измерено: нагрузка CPU %+.1f%%", -cpuDelta)
			res.MessageEn += fmt.Sprintf(". Measured: CPU load %+.1f%%", -cpuDelta)
		}
	}
	if len(failed) > 0 {
		res.MessageRu += ". " + formatEcoFailsRu(failed)
		res.MessageEn += ". " + formatEcoFailsEn(failed)
	}
	return res
}

func displayEcoName(name string, applied []ecoPIDState) string {
	if strings.TrimSpace(name) != "" {
		return filepath.Base(name)
	}
	if len(applied) > 0 && applied[0].Name != "" {
		return filepath.Base(applied[0].Name)
	}
	return "process"
}

func resolveEcoPIDs(name string, singlePID int32) []int32 {
	if singlePID > 0 {
		if processStillAlive(singlePID) {
			return []int32{singlePID}
		}
		return nil
	}
	candidates := expandEcoCandidates(name)
	filtered := make([]string, 0, len(candidates))
	for _, c := range candidates {
		if isPPLProtected(c) || isEcoProtected(c) {
			continue
		}
		filtered = append(filtered, c)
	}
	if len(filtered) == 0 {
		return nil
	}
	raw := listPIDsByExeNames(filtered)
	out := make([]int32, 0, len(raw))
	for _, p := range raw {
		out = append(out, int32(p))
	}
	if len(out) == 0 {
		procs, err := process.Processes()
		if err != nil {
			return nil
		}
		for _, p := range procs {
			n, err := p.Name()
			if err != nil {
				continue
			}
			ln := strings.ToLower(n)
			for _, c := range filtered {
				base := strings.ToLower(filepath.Base(c))
				if ln == base || strings.TrimSuffix(ln, ".exe") == strings.TrimSuffix(base, ".exe") {
					out = append(out, p.Pid)
					break
				}
			}
		}
	}
	return out
}

func samplePIDEco(pid int32) ecoPIDState {
	st := ecoPIDState{PID: pid}
	on, prio, _ := readProcessEcoState(uint32(pid))
	st.EcoWasOn = on
	st.Priority = prio
	p, err := process.NewProcess(pid)
	if err != nil {
		return st
	}
	if n, err := p.Name(); err == nil {
		st.Name = n
	}
	if raw, err := p.CPUPercent(); err == nil {
		st.CPUBefore = normalizeProcessCPU(raw)
	}
	if mi, err := p.MemoryInfo(); err == nil && mi != nil {
		st.RAMBeforeMB = float64(mi.RSS) / (1024 * 1024)
	}
	return st
}

func processStillAlive(pid int32) bool {
	if pid <= 0 {
		return false
	}
	p, err := process.NewProcess(pid)
	if err != nil {
		return false
	}
	running, err := p.IsRunning()
	return err == nil && running
}

func readProcessEcoState(pid uint32) (ecoOn bool, priority uint32, err error) {
	access := uint32(windows.PROCESS_QUERY_LIMITED_INFORMATION)
	h, err := windows.OpenProcess(access, false, pid)
	if err != nil {
		return false, 0, err
	}
	defer windows.CloseHandle(h)

	rPrio, _, _ := procGetPriorityClass.Call(uintptr(h))
	priority = uint32(rPrio)

	state := processPowerThrottlingState{
		Version:     processPowerThrottlingCurrentVersion,
		ControlMask: processPowerThrottlingExecutionSpeed,
	}
	r1, _, callErr := procGetProcessInformation.Call(
		uintptr(h),
		uintptr(processPowerThrottling),
		uintptr(unsafe.Pointer(&state)),
		uintptr(unsafe.Sizeof(state)),
	)
	if r1 != 0 {
		ecoOn = state.StateMask&processPowerThrottlingExecutionSpeed != 0
	} else if callErr != nil && callErr != windows.ERROR_SUCCESS {
		err = callErr
	}
	return ecoOn, priority, err
}

func setPriorityClass(pid uint32, cls uint32) error {
	access := uint32(windows.PROCESS_SET_INFORMATION | windows.PROCESS_QUERY_LIMITED_INFORMATION)
	h, err := windows.OpenProcess(access, false, pid)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(h)
	r, _, e := procSetPriorityClass.Call(uintptr(h), uintptr(cls))
	if r == 0 {
		return e
	}
	return nil
}

func humanEcoErr(err error) string {
	if err == nil {
		return "неизвестная ошибка"
	}
	msg := err.Error()
	low := strings.ToLower(msg)
	switch {
	case strings.Contains(low, "access"), strings.Contains(low, "отказано"), strings.Contains(msg, "5"):
		return "нет доступа"
	case strings.Contains(low, "защищ"), strings.Contains(low, "protected"), strings.Contains(low, "ppl"):
		return "защищённый процесс"
	case strings.Contains(low, "invalid"):
		return "некорректный PID"
	default:
		if errno, ok := err.(syscall.Errno); ok && (errno == windows.ERROR_ACCESS_DENIED || errno == 5) {
			return "нет доступа"
		}
		return "нет доступа или процесс защищён"
	}
}

func formatEcoFailsRu(fails []EcoFail) string {
	parts := make([]string, 0, len(fails))
	for _, f := range fails {
		label := f.Name
		if label == "" {
			label = fmt.Sprintf("PID %d", f.PID)
		}
		parts = append(parts, fmt.Sprintf("%s (%s)", label, f.Reason))
	}
	return "Не удалось для: " + strings.Join(parts, "; ")
}

func formatEcoFailsEn(fails []EcoFail) string {
	parts := make([]string, 0, len(fails))
	for _, f := range fails {
		label := f.Name
		if label == "" {
			label = fmt.Sprintf("PID %d", f.PID)
		}
		parts = append(parts, fmt.Sprintf("%s (%s)", label, f.Reason))
	}
	return "Failed for: " + strings.Join(parts, "; ")
}

// GetEcoHistory returns recent Eco Mode before/after sessions.
func (a *App) GetEcoHistory() []ecoHistoryEntry {
	ecoSessionMu.Lock()
	defer ecoSessionMu.Unlock()
	out := make([]ecoHistoryEntry, len(ecoHistory))
	copy(out, ecoHistory)
	for i := range out {
		sess := ecoSessions[out[i].SessionID]
		out[i].CanRevert = sess != nil && !sess.Reverted
		if out[i].CanRevert {
			alive := false
			for _, p := range sess.PIDs {
				if processStillAlive(p.PID) {
					alive = true
					break
				}
			}
			out[i].CanRevert = alive
		}
	}
	return out
}
