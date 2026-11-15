//go:build windows

package main

import (
	"os"
	"testing"
)

func TestIsExcludedConflictProcess(t *testing.T) {
	myPid := uint32(os.Getpid())

	// Self and WebView exclusions
	if !isExcludedConflictProcess("pulsecore.exe", 0) {
		t.Errorf("expected pulsecore.exe to be excluded")
	}
	if !isExcludedConflictProcess("pulsecore", 0) {
		t.Errorf("expected pulsecore to be excluded")
	}
	if !isExcludedConflictProcess("anything.exe", myPid) {
		t.Errorf("expected current process PID (%d) to be excluded", myPid)
	}
	if !isExcludedConflictProcess("msedgewebview2.exe", 1234) {
		t.Errorf("expected msedgewebview2.exe to be excluded")
	}
	if !isExcludedConflictProcess("pulsecorehelper.exe", 1234) {
		t.Errorf("expected pulsecorehelper.exe to be excluded")
	}

	// Critical Windows system processes
	systemProcs := []string{
		"explorer.exe", "dwm.exe", "csrss.exe", "svchost.exe",
		"searchhost.exe", "searchindexer.exe", "ctfmon.exe",
		"startmenuexperiencehost.exe", "shellexperiencehost.exe",
		"system", "winlogon.exe",
	}
	for _, proc := range systemProcs {
		if !isExcludedConflictProcess(proc, 9999) {
			t.Errorf("expected system process %s to be excluded", proc)
		}
	}

	// Normal user processes must NOT be excluded
	allowedProcs := []string{
		"Antigravity.exe", "chrome.exe", "discord.exe", "notepad.exe", "game.exe",
	}
	for _, proc := range allowedProcs {
		if isExcludedConflictProcess(proc, 8888) {
			t.Errorf("expected user process %s to NOT be excluded", proc)
		}
	}
}

func TestMatchProcessWithFolder(t *testing.T) {
	// 1. Process name matches folder name tokens (Antigravity Tools <-> Antigravity.exe)
	if !matchProcessWithFolder("Antigravity.exe", "", "", "", nil, "Antigravity Tools", "") {
		t.Errorf("expected Antigravity.exe to match folder 'Antigravity Tools'")
	}

	// 2. Protected/excluded process must never match
	if matchProcessWithFolder("pulsecore.exe", "", "", "", nil, "Antigravity Tools", "") {
		t.Errorf("pulsecore.exe should never match")
	}
	if matchProcessWithFolder("explorer.exe", "", "", "", nil, "Antigravity Tools", "") {
		t.Errorf("explorer.exe should never match")
	}

	// 3. ExePath inside folderPath
	if !matchProcessWithFolder("worker.exe", `C:\Apps\MyProgram\bin\worker.exe`, "", "", nil, "MyProgram", `C:\Apps\MyProgram`) {
		t.Errorf("expected worker.exe inside folderPath to match")
	}

	// 4. CWD inside folderPath
	if !matchProcessWithFolder("node.exe", `C:\Node\node.exe`, `C:\Projects\Site\server`, "", nil, "Site", `C:\Projects\Site`) {
		t.Errorf("expected node.exe with CWD inside folderPath to match")
	}

	// 5. Window title match
	if !matchProcessWithFolder("javaw.exe", "", "", "", []string{"Minecraft 1.20 - In-Game"}, "Minecraft", "") {
		t.Errorf("expected window title match for Minecraft")
	}

	// 6. Completely unrelated process
	if matchProcessWithFolder("notepad.exe", `C:\Windows\notepad.exe`, `C:\Windows`, "", []string{"Untitled - Notepad"}, "Antigravity Tools", `C:\Projects\Antigravity Tools`) {
		t.Errorf("notepad.exe should not match Antigravity Tools")
	}
}

func TestFormatConflictOverlayLabels(t *testing.T) {
	// Single process
	procsSingle := []LockProcessInfo{
		{PID: 100, ProcessName: "Antigravity.exe"},
	}
	title, sub, btn := formatConflictOverlayLabels(procsSingle, "Antigravity Tools")
	if title != "🔒 Держит: Antigravity.exe" {
		t.Errorf("unexpected title: %s", title)
	}
	if sub != "Объект: Antigravity Tools" {
		t.Errorf("unexpected sub: %s", sub)
	}
	if btn != "Закрыть и удалить" {
		t.Errorf("unexpected btn: %s", btn)
	}

	// Multiple processes
	procsMulti := []LockProcessInfo{
		{PID: 100, ProcessName: "Antigravity.exe"},
		{PID: 101, ProcessName: "worker.exe"},
	}
	titleM, subM, _ := formatConflictOverlayLabels(procsMulti, "Antigravity Tools")
	if titleM != "🔒 Держит: Antigravity.exe и ещё 1" {
		t.Errorf("unexpected multi title: %s", titleM)
	}
	if subM != "Объект: Antigravity Tools" {
		t.Errorf("unexpected multi sub: %s", subM)
	}

	// Zero processes (unresolved locker)
	titleZ, subZ, _ := formatConflictOverlayLabels(nil, "Antigravity Tools")
	if titleZ != "🔒 Заблокировано: Antigravity Tools" {
		t.Errorf("unexpected zero title: %s", titleZ)
	}
	if subZ != "Папка или файл заняты процессом" {
		t.Errorf("unexpected zero sub: %s", subZ)
	}

	// Filter out pulsecore.exe if accidentally passed
	procsWithPulsecore := []LockProcessInfo{
		{PID: 100, ProcessName: "pulsecore.exe"},
		{PID: 101, ProcessName: "Antigravity.exe"},
	}
	titleF, subF, _ := formatConflictOverlayLabels(procsWithPulsecore, "Antigravity Tools")
	if titleF != "🔒 Держит: Antigravity.exe" {
		t.Errorf("expected pulsecore.exe to be filtered out, got: %s", titleF)
	}
	if subF != "Объект: Antigravity Tools" {
		t.Errorf("unexpected filtered sub: %s", subF)
	}
}
