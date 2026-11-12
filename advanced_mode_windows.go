//go:build windows

package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/robbled17/zeropulse/loc"
	"golang.org/x/sys/windows"
)

const (
	pawnioSetupURL    = "https://github.com/namazso/PawnIO.Setup/releases/download/2.2.0/PawnIO_setup.exe"
	pawnioSetupSHA256 = "1F519A22E47187F70A1379A48CA604981C4FCF694F4E65B734AAA74A9FBA3032"
	helperAppVersion  = "1.0.0"
)

var (
	advMu             sync.Mutex
	advStateCache     AdvancedModeState
	advStateAt        time.Time
	advStarting       bool
	advStartingSince  time.Time
	advStartingWindow = 15 * time.Second
)

func programDataPulseCore() string {
	base := os.Getenv("PROGRAMDATA")
	if base == "" {
		base = `C:\ProgramData`
	}
	return filepath.Join(base, helperStateDirName)
}

func localAppDataPulseCore() string {
	base, err := os.UserCacheDir()
	if err != nil || base == "" {
		base = filepath.Join(os.Getenv("LOCALAPPDATA"), "PulseCore")
		return base
	}
	// UserCacheDir on Windows is LocalAppData
	return filepath.Join(base, "PulseCore")
}

func uiLogDir() string {
	return filepath.Join(localAppDataPulseCore(), "logs")
}

func helperStatePath() string {
	return filepath.Join(programDataPulseCore(), "helper-state.json")
}

func helperReadyPath() string {
	return filepath.Join(programDataPulseCore(), "helper.ready")
}

func secureHelperDir() string {
	pf := os.Getenv("ProgramFiles")
	if pf == "" {
		pf = `C:\Program Files`
	}
	return filepath.Join(pf, "PulseCore", "helper")
}

func secureHelperExe() string {
	return filepath.Join(secureHelperDir(), "PulseCoreHelper.exe")
}

func loadHelperState() (helperStateFile, error) {
	raw, err := os.ReadFile(helperStatePath())
	if err != nil {
		return helperStateFile{}, err
	}
	hadBOM := bytes.HasPrefix(raw, []byte{0xEF, 0xBB, 0xBF})
	// .NET Encoding.UTF8 may write BOM; strip it so json.Unmarshal succeeds.
	b := bytes.TrimPrefix(raw, []byte{0xEF, 0xBB, 0xBF})
	var st helperStateFile
	if err := json.Unmarshal(b, &st); err != nil {
		return helperStateFile{}, fmt.Errorf("helper-state.json unmarshal: %w", err)
	}
	if hadBOM {
		// Best-effort rewrite without BOM so other tools / older builds also work.
		_ = saveHelperState(st)
	}
	return st, nil
}

func saveHelperState(st helperStateFile) error {
	dir := programDataPulseCore()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(helperStatePath(), b, 0644)
}

func advancedModeEnabled() bool {
	st, err := loadHelperState()
	return err == nil && st.InstalledByPulseCore && st.Enabled
}

func appendUILog(line string) {
	dir := uiLogDir()
	_ = os.MkdirAll(dir, 0755)
	f := filepath.Join(dir, "ui.log")
	_ = appendFileLine(f, time.Now().Format("2006-01-02 15:04:05.000")+" "+line)
}

func appendFileLine(path, line string) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(line + "\n")
	return err
}

func pawnIOPresent() bool {
	if _, err := os.Stat(`C:\Program Files\PawnIO\PawnIOLib.dll`); err == nil {
		return true
	}
	out, _ := runHiddenCombinedOutput("sc", "query", "PawnIO")
	s := string(out)
	return strings.Contains(s, "RUNNING") || strings.Contains(s, "STOPPED")
}

func pawnIORunning() bool {
	out, _ := runHiddenCombinedOutput("sc", "query", "PawnIO")
	return strings.Contains(string(out), "RUNNING")
}

func otherAppsUsePawnIO() bool {
	markers := []string{
		`C:\Program Files\OpenRGB\PawnIOLib.dll`,
		`C:\Program Files\LibreHardwareMonitor\LibreHardwareMonitor.exe`,
		`C:\Program Files\FanControl\FanControl.exe`,
	}
	for _, p := range markers {
		if _, err := os.Stat(p); err == nil {
			return true
		}
	}
	for _, name := range []string{"OpenRGB", "FanControl", "LibreHardwareMonitor"} {
		out, _ := runHiddenCombinedOutput("tasklist", "/FI", "IMAGENAME eq "+name+".exe")
		if strings.Contains(strings.ToLower(string(out)), strings.ToLower(name+".exe")) {
			return true
		}
	}
	return false
}

func currentUserSID() (string, error) {
	token, err := windows.OpenCurrentProcessToken()
	if err != nil {
		return "", err
	}
	defer token.Close()
	user, err := token.GetTokenUser()
	if err != nil {
		return "", err
	}
	return user.User.Sid.String(), nil
}

func helperProcessRunning() bool {
	return helperProcessPID() > 0
}

func helperProcessPID() int {
	out, err := runHiddenCombinedOutput("tasklist", "/FI", "IMAGENAME eq PulseCoreHelper.exe", "/FO", "CSV", "/NH")
	if err != nil {
		return 0
	}
	s := strings.TrimSpace(string(out))
	if s == "" || !strings.Contains(strings.ToLower(s), "pulsecorehelper.exe") {
		return 0
	}
	// "PulseCoreHelper.exe","12345","Console","1","12,345 K"
	parts := strings.Split(s, ",")
	if len(parts) < 2 {
		return 0
	}
	pidStr := strings.Trim(parts[1], `"`)
	var pid int
	fmt.Sscanf(pidStr, "%d", &pid)
	return pid
}

func taskExists() bool {
	err := runHiddenCommand("schtasks", "/Query", "/TN", helperTaskName)
	return err == nil
}

func taskIsRunning() bool {
	out, err := runHiddenCombinedOutput("schtasks", "/Query", "/TN", helperTaskName, "/FO", "LIST")
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(out), "\n") {
		low := strings.ToLower(strings.TrimSpace(line))
		if strings.HasPrefix(low, "status:") {
			return strings.Contains(low, "running")
		}
	}
	return false
}

func markAdvStarting() {
	advMu.Lock()
	advStarting = true
	advStartingSince = time.Now()
	advStateAt = time.Time{}
	advMu.Unlock()
}

func clearAdvStarting() {
	advMu.Lock()
	advStarting = false
	advStartingSince = time.Time{}
	advStateAt = time.Time{}
	advMu.Unlock()
}

func invalidateSensorCaches(a *App) {
	invalidateHelperClientCache()
	if a == nil {
		return
	}
	a.mu.Lock()
	a.lastTemp = time.Time{}
	a.cachedTemp = 0
	a.tempUnavailable = true
	a.cachedTempSource = ""
	a.cachedTempLabelRu = ""
	a.cachedTempLabelEn = ""
	a.diskSMARTCache = map[string]DiskHealthReport{}
	a.diskSMARTCacheAt = map[string]time.Time{}
	a.mu.Unlock()
}

// GetAdvancedModeState returns UI status for the advanced mode toggle.
func (a *App) GetAdvancedModeState() AdvancedModeState {
	advMu.Lock()
	if time.Since(advStateAt) < 800*time.Millisecond && advStateCache.UpdatedAt != "" {
		out := advStateCache
		advMu.Unlock()
		return out
	}
	advMu.Unlock()

	st := buildAdvancedModeState()
	advMu.Lock()
	advStateCache = st
	advStateAt = time.Now()
	advMu.Unlock()
	return st
}

func buildAdvancedModeState() AdvancedModeState {
	file, loadErr := loadHelperState()
	enabled := loadErr == nil && file.Enabled && file.InstalledByPulseCore
	present := pawnIOPresent()
	procRunning := helperProcessRunning()
	pipeOK := pipeExists()
	taskOK := taskExists()
	taskRun := taskIsRunning()
	pid := helperProcessPID()

	helperOK := false
	var drv HelperDriver
	var getResp HelperResponse
	accessDenied := false
	lastErr := lastHelperError()
	if enabled {
		if resp, err := helperGet(); err == nil {
			helperOK = true
			drv = resp.Driver
			getResp = resp
		} else {
			errStr := err.Error()
			lastErr = errStr
			low := strings.ToLower(errStr)
			if strings.Contains(low, "access") || strings.Contains(low, "denied") || strings.Contains(errStr, "5") {
				accessDenied = true
			}
			if helperPing() {
				helperOK = true
			}
		}
	}

	advMu.Lock()
	starting := advStarting && time.Since(advStartingSince) < advStartingWindow
	if advStarting && !starting {
		advStarting = false
	}
	if helperOK {
		advStarting = false
		starting = false
	}
	advMu.Unlock()

	out := AdvancedModeState{
		Enabled:         enabled,
		HelperRunning:   helperOK,
		PawnIOPresent:   present,
		PawnIOOwnedByUs: file.PawnIOInstalledByUs,
		UpdatedAt:       time.Now().Format(time.RFC3339),
		ExplainRu:       "Устанавливается помощник в Program Files (задача планировщика с правами администратора) и при необходимости официальный драйвер PawnIO. Права нужны один раз. Отключить - этим же переключателем.",
		ExplainEn:       "Installs the helper under Program Files (scheduled task with admin rights) and, if needed, official PawnIO. One admin consent. Turn off with the same switch.",
		LastError:       lastErr,
		TaskRunning:     taskRun,
		ProcessRunning:  procRunning,
		ProcessPID:      pid,
		PipePresent:     pipeOK,
		ProtocolVersion: file.ProtocolVersion,
		HelperVersion:   file.HelperVersion,
	}
	if out.ProtocolVersion == 0 {
		out.ProtocolVersion = helperProtocolVersion
	}
	if helperOK {
		if t := lastHelperOKTime(); !t.IsZero() {
			out.LastHelperReplyAt = t.Format(time.RFC3339)
		}
		if getResp.V > 0 {
			out.ProtocolVersion = getResp.V
		}
	}

	switch {
	case !enabled:
		out.Phase = "off"
		out.Status = "off"
		out.StatusRu, out.StatusEn = "выключен", "off"
	case starting && !helperOK:
		out.Phase = "starting"
		out.Status = "starting"
		out.StatusRu, out.StatusEn = "включается", "starting"
		out.ActionRu = "Запускаю помощник…"
		out.ActionEn = "Starting helper…"
	case helperOK && drv.Blocked:
		out.Phase = "error"
		out.Status = "driver_blocked"
		out.DriverBlocked = true
		out.DriverBlockedReason = drv.BlockedReason
		out.ErrorCode = "driver_blocked"
		out.StatusRu, out.StatusEn = "ошибка", "error"
		out.ActionRu = "Защитник / Memory Integrity заблокировал драйвер: " + drv.BlockedReason
		out.ActionEn = "Defender / Memory Integrity blocked the driver: " + drv.BlockedReason
	case helperOK:
		out.Phase = "ok"
		out.Status = "ok"
		out.StatusRu, out.StatusEn = "работает", "running"
	case accessDenied:
		out.Phase = "error"
		out.Status = "pipe_access_denied"
		out.ErrorCode = "pipe_access_denied"
		out.StatusRu, out.StatusEn = "ошибка", "error"
		out.ActionRu = "Нет доступа к каналу. Нажмите «Перезапустить помощник»."
		out.ActionEn = "No access to the helper channel. Click “Restart helper”."
	case !taskOK:
		out.Phase = "error"
		out.Status = "task_missing"
		out.ErrorCode = "task_missing"
		out.StatusRu, out.StatusEn = "ошибка", "error"
		out.ActionRu = "Задача не создана. Выключите и снова включите расширенный режим (UAC)."
		out.ActionEn = "Scheduled task missing. Turn advanced mode off and on again (UAC)."
	case !procRunning:
		out.Phase = "error"
		out.Status = "task_not_running"
		out.ErrorCode = "task_not_running"
		out.StatusRu, out.StatusEn = "ошибка", "error"
		out.ActionRu = "Задача не запущена. Нажмите «Перезапустить помощник»."
		out.ActionEn = "Helper task is not running. Click “Restart helper”."
	case !pipeOK:
		out.Phase = "error"
		out.Status = "pipe_missing"
		out.ErrorCode = "pipe_missing"
		out.StatusRu, out.StatusEn = "ошибка", "error"
		out.ActionRu = "Нет канала помощника. Возможно, Защитник заблокировал помощник - перезапустите или проверьте исключения."
		out.ActionEn = "Helper channel missing. Defender may have blocked the helper - restart or check exclusions."
	default:
		out.Phase = "error"
		out.Status = "helper_down"
		out.ErrorCode = "helper_down"
		out.StatusRu, out.StatusEn = "ошибка", "error"
		out.ActionRu = "Канал не отвечает. Перезапустите помощник или скопируйте диагностику."
		out.ActionEn = "Channel not responding. Restart the helper or copy diagnostics."
	}
	return out
}

// EnableAdvancedMode elevates once: copies helper to Program Files, creates task, runs it.
func (a *App) EnableAdvancedMode(consentPawnIO bool) AdvancedModeActionResult {
	appendUILog("EnableAdvancedMode start consentPawnIO=" + fmt.Sprint(consentPawnIO))
	markAdvStarting()
	invalidateSensorCaches(a)
	presentBefore := pawnIOPresent()
	needPawnIO := !presentBefore
	if needPawnIO && !consentPawnIO {
		clearAdvStarting()
		return AdvancedModeActionResult{
			OK:                false,
			NeedConsentPawnIO: true,
			Message:           loc.M("advNeedPawnIOConsent", nil),
			MessageRu:         "Для температуры CPU нужен драйвер PawnIO (GPL-2.0, namazso). Подтвердите установку официального установщика.",
			MessageEn:         "CPU temperature needs the PawnIO driver (GPL-2.0, namazso). Confirm installing the official setup.",
			State:             a.GetAdvancedModeState(),
			ErrorCode:         "need_pawnio_consent",
			Phase:             "off",
		}
	}

	srcDir, err := findBundledHelper()
	if err != nil {
		appendUILog("bundled helper missing: " + err.Error())
		clearAdvStarting()
		return failAdv("helper_missing",
			"Не найден PulseCoreHelper рядом с приложением. Переустановите PulseCore.",
			"PulseCoreHelper not found next to the app. Reinstall PulseCore.", a)
	}
	bootstrapExe := filepath.Join(srcDir, "PulseCoreHelper.exe")

	sid, _ := currentUserSID()
	st := helperStateFile{
		UserSID:              sid,
		InstalledByPulseCore: true,
		PawnIOWasPresent:     presentBefore,
		PawnIOInstalledByUs:  false,
		TaskName:             helperTaskName,
		HelperPath:           secureHelperExe(),
		Enabled:              true,
		HelperVersion:        helperAppVersion,
		ProtocolVersion:      helperProtocolVersion,
		InstalledAt:          time.Now(),
	}
	_ = saveHelperState(st)

	args := []string{"install", "-src", srcDir}
	if needPawnIO && consentPawnIO {
		if shouldSkipPawnIOInstall() {
			appendUILog("PawnIO already present/running - skip installer")
			st.PawnIOInstalledByUs = false
			_ = saveHelperState(st)
		} else {
			setupPath, err := ensurePawnIOSetup()
			if err != nil {
				clearAdvStarting()
				return failAdv("pawnio_download",
					"Не удалось получить PawnIO: "+err.Error(),
					"Failed to obtain PawnIO: "+err.Error(), a)
			}
			// Pass setup to elevated helper, which must invoke: PawnIO_setup.exe -install
			args = append(args, "-pawnio", setupPath)
			st.PawnIOInstalledByUs = true
			_ = saveHelperState(st)
			_ = os.Setenv("PULSECORE_PAWNIO_OWNED", "1")
		}
	}

	_ = os.Remove(filepath.Join(programDataPulseCore(), "install.done"))
	appendUILog("elevating install src=" + srcDir)
	if err := runElevatedAndWaitDone(bootstrapExe, args...); err != nil {
		st.Enabled = false
		_ = saveHelperState(st)
		appendUILog("elevate failed: " + err.Error())
		clearAdvStarting()
		code := "uac_cancelled"
		low := strings.ToLower(err.Error())
		if !strings.Contains(low, "cancel") && !strings.Contains(err.Error(), "1223") {
			code = "install_failed"
		}
		if strings.Contains(low, "defender") || strings.Contains(low, "blocked") {
			code = "defender_blocked"
		}
		return failAdv(code,
			"UAC отменён или установка не удалась: "+err.Error(),
			"UAC cancelled or install failed: "+err.Error(), a)
	}

	// Ensure task is running even if install's /Run raced.
	_ = runHiddenCommand("schtasks", "/Run", "/TN", helperTaskName)

	waitRes := waitForHelperReady(15 * time.Second)
	invalidateSensorCaches(a)
	advMu.Lock()
	advStateAt = time.Time{}
	advMu.Unlock()
	state := a.GetAdvancedModeState()
	if !waitRes.OK {
		appendUILog("wait failed: " + waitRes.Code + " " + waitRes.Detail)
		// Keep starting window briefly so UI shows "Запускаю…" then error phase.
		res := failAdv(waitRes.Code, waitRes.MessageRu, waitRes.MessageEn, a)
		res.Phase = state.Phase
		res.State = state
		return res
	}

	// Refresh state hash/path from disk (also strips any leftover BOM via save).
	if file, err := loadHelperState(); err == nil {
		file.Enabled = true
		file.InstalledByPulseCore = true
		_ = saveHelperState(file)
	} else {
		_ = saveHelperState(st)
	}
	clearAdvStarting()
	invalidateSensorCaches(a)
	appendUILog("EnableAdvancedMode OK")
	return AdvancedModeActionResult{
		OK:        true,
		Message:   loc.M("advEnableOK", nil),
		MessageRu: "Расширенный режим включён.",
		MessageEn: "Advanced mode enabled.",
		State:     a.GetAdvancedModeState(),
		Phase:     "ok",
	}
}

// RestartHelper ends and re-runs the scheduled helper task (no UAC if task already exists).
func (a *App) RestartHelper() AdvancedModeActionResult {
	appendUILog("RestartHelper")
	if !advancedModeEnabled() {
		return failAdv("off",
			"Расширенный режим выключен.",
			"Advanced mode is off.", a)
	}
	if !taskExists() {
		return failAdv("task_missing",
			"Задача не создана. Включите расширенный режим заново.",
			"Scheduled task missing. Re-enable advanced mode.", a)
	}
	markAdvStarting()
	invalidateSensorCaches(a)
	_ = runHiddenCommand("schtasks", "/End", "/TN", helperTaskName)
	time.Sleep(700 * time.Millisecond)
	if err := runHiddenCommand("schtasks", "/Run", "/TN", helperTaskName); err != nil {
		clearAdvStarting()
		return failAdv("task_not_running",
			"Не удалось запустить задачу помощника: "+err.Error(),
			"Failed to run helper task: "+err.Error(), a)
	}
	waitRes := waitForHelperReady(15 * time.Second)
	invalidateSensorCaches(a)
	if !waitRes.OK {
		return failAdv(waitRes.Code, waitRes.MessageRu, waitRes.MessageEn, a)
	}
	clearAdvStarting()
	return AdvancedModeActionResult{
		OK:      true,
		Message: loc.M("advRestartOK", nil),
		MessageRu: "Помощник перезапущен.",
		MessageEn: "Helper restarted.",
		State: a.GetAdvancedModeState(),
		Phase: "ok",
	}
}

type helperWaitResult struct {
	OK        bool
	Code      string
	Detail    string
	MessageRu string
	MessageEn string
}

func waitForHelperReady(timeout time.Duration) helperWaitResult {
	deadline := time.Now().Add(timeout)
	var lastErr string
	for time.Now().Before(deadline) {
		if helperPing() {
			if _, err := helperGet(); err == nil {
				return helperWaitResult{OK: true}
			}
			// ping ok is enough to consider helper up
			return helperWaitResult{OK: true}
		}
		lastErr = lastHelperError()
		time.Sleep(350 * time.Millisecond)
	}

	if !taskExists() {
		return helperWaitResult{
			Code: "task_missing", Detail: lastErr,
			MessageRu: "Задача планировщика PulseCoreHelper не создана. Включите режим ещё раз и примите UAC.",
			MessageEn: "Scheduled task PulseCoreHelper was not created. Enable again and accept UAC.",
		}
	}
	if !helperProcessRunning() {
		return helperWaitResult{
			Code: "task_not_running", Detail: lastErr,
			MessageRu: "Задача есть, но процесс помощника не запущен. Нажмите «Скопировать диагностику» или переключите режим ещё раз.",
			MessageEn: "Task exists but helper process is not running. Copy diagnostics or toggle the mode again.",
		}
	}
	low := strings.ToLower(lastErr)
	if strings.Contains(low, "access") || strings.Contains(low, "denied") || strings.Contains(lastErr, "5") {
		return helperWaitResult{
			Code: "pipe_access_denied", Detail: lastErr,
			MessageRu: "Нет доступа к каналу помощника (DACL/целостность). Переустановите расширенный режим.",
			MessageEn: "No access to the helper pipe (DACL/integrity). Re-enable advanced mode.",
		}
	}
	if strings.Contains(low, "protocol") || strings.Contains(low, "mismatch") {
		return helperWaitResult{
			Code: "protocol_mismatch", Detail: lastErr,
			MessageRu: "Несовместимая версия протокола помощника. Обновите PulseCore и включите режим снова.",
			MessageEn: "Incompatible helper protocol version. Update PulseCore and re-enable advanced mode.",
		}
	}
	if !pipeExists() {
		return helperWaitResult{
			Code: "pipe_missing", Detail: lastErr,
			MessageRu: "Процесс помощника запущен, но канал не появился. Проверьте журнал в %LOCALAPPDATA%\\PulseCore\\logs.",
			MessageEn: "Helper process is running but the pipe did not appear. Check %LOCALAPPDATA%\\PulseCore\\logs.",
		}
	}
	return helperWaitResult{
		Code: "helper_down", Detail: lastErr,
		MessageRu: "Помощник не отвечает на канале. Проверьте Защитник Windows и логи, затем включите режим снова.",
		MessageEn: "Helper is not answering on the pipe. Check Windows Defender and logs, then re-enable.",
	}
}

func pipeExists() bool {
	_, err := os.Stat(`\\.\pipe\` + helperPipeName)
	return err == nil
}

func failAdv(code, ru, en string, a *App) AdvancedModeActionResult {
	msgCode := "advErr_" + code
	if code == "" {
		msgCode = "errGeneric"
	}
	return AdvancedModeActionResult{
		OK: false, ErrorCode: code,
		Message:   loc.M(msgCode, loc.P("err", en)),
		MessageRu: ru, MessageEn: en,
		State: a.GetAdvancedModeState(),
	}
}

// DisableAdvancedMode removes the task/helper files; uninstalls PawnIO only if we installed it and unused.
func (a *App) DisableAdvancedMode() AdvancedModeActionResult {
	appendUILog("DisableAdvancedMode")
	clearAdvStarting()
	invalidateSensorCaches(a)
	st, _ := loadHelperState()
	exe := secureHelperExe()
	if _, err := os.Stat(exe); err != nil {
		if bundled, err2 := findBundledHelper(); err2 == nil {
			exe = filepath.Join(bundled, "PulseCoreHelper.exe")
		}
	}
	_ = os.Remove(filepath.Join(programDataPulseCore(), "install.done"))
	_ = runElevatedAndWaitDone(exe, "uninstall")

	removePawnIO := st.PawnIOInstalledByUs && !st.PawnIOWasPresent && !otherAppsUsePawnIO()
	if removePawnIO {
		uninstallPawnIOOwned()
	}

	_ = os.Remove(helperStatePath())
	_ = os.Remove(helperReadyPath())
	// leftover user staging
	_ = os.RemoveAll(filepath.Join(localAppDataPulseCore(), "helper"))
	_ = os.RemoveAll(filepath.Join(os.Getenv("APPDATA"), "PulseCore", "helper"))

	advMu.Lock()
	advStateAt = time.Time{}
	advMu.Unlock()
	return AdvancedModeActionResult{
		OK:      true,
		Message: loc.M("advDisableOK", nil),
		MessageRu: "Расширенный режим выключен.",
		MessageEn: "Advanced mode disabled.",
		State: a.GetAdvancedModeState(),
	}
}

// GetHelperDiagnostics returns a text blob for the "Copy diagnostics" button.
func (a *App) GetHelperDiagnostics() string {
	var b strings.Builder
	b.WriteString("PulseCore advanced-mode diagnostics\n")
	b.WriteString("time=" + time.Now().Format(time.RFC3339) + "\n")
	b.WriteString(fmt.Sprintf("elevated=%v\n", IsProcessElevated()))
	if sid, err := currentUserSID(); err == nil {
		b.WriteString("sid=" + sid + "\n")
	}
	st, err := loadHelperState()
	if err != nil {
		b.WriteString("state=missing\n")
	} else {
		raw, _ := json.MarshalIndent(st, "", "  ")
		b.WriteString("state=\n" + string(raw) + "\n")
	}
	b.WriteString(fmt.Sprintf("taskExists=%v helperProc=%v pipe=%v\n", taskExists(), helperProcessRunning(), pipeExists()))
	b.WriteString("secureExe=" + secureHelperExe() + " exists=" + fmt.Sprint(fileExists(secureHelperExe())) + "\n")
	out, _ := runHiddenCombinedOutput("schtasks", "/Query", "/TN", helperTaskName, "/V", "/FO", "LIST")
	b.WriteString("--- schtasks ---\n")
	b.WriteString(string(out))
	b.WriteString("\n--- last helper error ---\n")
	b.WriteString(lastHelperError() + "\n")
	if ping := helperPing(); ping {
		if resp, err := helperGet(); err == nil {
			b.WriteString(fmt.Sprintf("get ok cpu.ok=%v disks=%d\n", resp.CPU.OK, len(resp.Disks)))
		} else {
			b.WriteString("get err=" + err.Error() + "\n")
		}
	} else {
		b.WriteString("ping=false\n")
	}
	for _, p := range []string{
		filepath.Join(programDataPulseCore(), "helper.log"),
		filepath.Join(uiLogDir(), "helper.log"),
		filepath.Join(uiLogDir(), "ui.log"),
	} {
		b.WriteString("--- " + p + " ---\n")
		if data, err := os.ReadFile(p); err == nil {
			s := string(data)
			if len(s) > 8000 {
				s = s[len(s)-8000:]
			}
			b.WriteString(s)
			if !strings.HasSuffix(s, "\n") {
				b.WriteString("\n")
			}
		} else {
			b.WriteString("(missing)\n")
		}
	}
	return b.String()
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func findBundledHelper() (string, error) {
	// Prefer portable extract from go:embed (single-file distribution).
	if dir, err := ensureEmbeddedHelper(); err == nil {
		exe := filepath.Join(dir, "PulseCoreHelper.exe")
		if _, err := os.Stat(exe); err == nil {
			return dir, nil
		}
	}
	candidates := []string{}
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		candidates = append(candidates,
			filepath.Join(dir, "helper"),
			filepath.Join(dir, "..", "helper", "dist"),
		)
	}
	candidates = append(candidates,
		filepath.Join("helper", "dist"),
		`C:\Projects\pulsecore\helper\dist`,
		`C:\Projects\pulsecore\build\bin\helper`,
		`C:\Projects\pulsecore\helper\PulseCoreHelper\bin\Release\net472\win-x64`,
	)
	for _, c := range candidates {
		exe := filepath.Join(c, "PulseCoreHelper.exe")
		if _, err := os.Stat(exe); err == nil {
			return c, nil
		}
	}
	return "", fmt.Errorf("PulseCoreHelper.exe not found (embed + sidecar)")
}

func ensurePawnIOSetup() (string, error) {
	dir := filepath.Join(programDataPulseCore(), "pawnio")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	dst := filepath.Join(dir, "PawnIO_setup.exe")
	if verifyFileSHA256(dst, pawnioSetupSHA256) {
		return dst, nil
	}
	tmp := dst + ".tmp"
	resp, err := http.Get(pawnioSetupURL)
	if err != nil {
		alt := filepath.Join(os.TempDir(), "PawnIO_setup.exe")
		if verifyFileSHA256(alt, pawnioSetupSHA256) {
			if err := copyFile(alt, dst); err == nil {
				return dst, nil
			}
		}
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("download HTTP %d", resp.StatusCode)
	}
	f, err := os.Create(tmp)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	if _, err := io.Copy(io.MultiWriter(f, h), resp.Body); err != nil {
		f.Close()
		_ = os.Remove(tmp)
		return "", err
	}
	f.Close()
	sum := strings.ToUpper(hex.EncodeToString(h.Sum(nil)))
	if sum != strings.ToUpper(pawnioSetupSHA256) {
		_ = os.Remove(tmp)
		return "", fmt.Errorf("PawnIO SHA256 mismatch: got %s", sum)
	}
	_ = os.Remove(dst)
	if err := os.Rename(tmp, dst); err != nil {
		return "", err
	}
	return dst, nil
}

func verifyFileSHA256(path, want string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return false
	}
	sum := strings.ToUpper(hex.EncodeToString(h.Sum(nil)))
	return sum == strings.ToUpper(want)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

func quoteArgs(args []string) []string {
	out := make([]string, 0, len(args))
	for _, a := range args {
		if strings.ContainsAny(a, " \t\"") {
			out = append(out, `"`+strings.ReplaceAll(a, `"`, `\"`)+`"`)
		} else {
			out = append(out, a)
		}
	}
	return out
}

func runElevatedFireAndForget(exe string, args ...string) error {
	// Already elevated: start helper/install directly - no UAC relaunch loop, no console.
	if IsProcessElevated() {
		return RunHiddenCommand(exe, args...).Start()
	}

	verb, err := windows.UTF16PtrFromString("runas")
	if err != nil {
		return err
	}
	file, err := windows.UTF16PtrFromString(exe)
	if err != nil {
		return err
	}
	param := strings.Join(quoteArgs(args), " ")
	params, err := windows.UTF16PtrFromString(param)
	if err != nil {
		return err
	}
	cwd, err := windows.UTF16PtrFromString(filepath.Dir(exe))
	if err != nil {
		return err
	}
	// Native UAC on the helper only - never cmd.exe / PowerShell Start-Process.
	return windows.ShellExecute(0, verb, file, params, cwd, windows.SW_HIDE)
}

// runElevatedAndWaitDone launches elevated install/uninstall and waits for ProgramData\install.done.
func runElevatedAndWaitDone(exe string, args ...string) error {
	done := filepath.Join(programDataPulseCore(), "install.done")
	_ = os.Remove(done)
	if err := runElevatedFireAndForget(exe, args...); err != nil {
		return err
	}
	deadline := time.Now().Add(120 * time.Second)
	for time.Now().Before(deadline) {
		if b, err := os.ReadFile(done); err == nil {
			line := strings.TrimSpace(strings.SplitN(string(b), "\n", 2)[0])
			if line == "0" {
				return nil
			}
			return fmt.Errorf("elevated install exit %s", line)
		}
		time.Sleep(300 * time.Millisecond)
	}
	return fmt.Errorf("timed out waiting for elevated install")
}
