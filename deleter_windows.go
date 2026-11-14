//go:build windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	foDelete          = 0x0003
	fofSilent         = 0x0004
	fofNoConfirmation = 0x0010
	fofAllowUndo      = 0x0040 // FOF_ALLOWUNDO: Preserves undo info (sends to Recycle Bin)
	fofNoErrorUI      = 0x0400
	fofNoConfirmMkDir = 0x0200
)

type shFileOpStructW struct {
	Hwnd                 uintptr
	WFunc                uint32
	PFrom                *uint16
	PTo                  *uint16
	FFlags               uint16
	FAnyOperationsAborted int32
	HNameMappings        uintptr
	LpszProgressTitle    *uint16
}

var (
	modShell32Delete     = windows.NewLazySystemDLL("shell32.dll")
	procSHFileOperationW = modShell32Delete.NewProc("SHFileOperationW")
)

func clearReadOnlyAttr(path string) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return
	}
	_ = windows.SetFileAttributes(p, windows.FILE_ATTRIBUTE_NORMAL)
}

func stripTreeAttributes(dirPath string) {
	_ = filepath.WalkDir(dirPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d != nil && d.Type()&os.ModeSymlink != 0 {
			return nil
		}
		clearReadOnlyAttr(path)
		_ = os.Chmod(path, 0o777)
		return nil
	})
}

func shFileDelete(dirPath string) error {
	// Double-null-terminated path list required by SHFileOperationW.
	u, err := syscall.UTF16FromString(dirPath)
	if err != nil {
		return err
	}
	u = append(u, 0) // extra NUL
	fromPtr := &u[0]

	op := shFileOpStructW{
		WFunc:  foDelete,
		PFrom:  fromPtr,
		FFlags: fofSilent | fofNoConfirmation | fofNoErrorUI | fofNoConfirmMkDir,
	}
	ret, _, callErr := procSHFileOperationW.Call(uintptr(unsafe.Pointer(&op)))
	if ret != 0 {
		if callErr != nil && callErr != windows.ERROR_SUCCESS {
			return fmt.Errorf("SHFileOperationW=%d: %v", ret, callErr)
		}
		return fmt.Errorf("SHFileOperationW failed code=%d", ret)
	}
	if op.FAnyOperationsAborted != 0 {
		return fmt.Errorf("SHFileOperationW aborted")
	}
	return nil
}

func powershellForceRemove(dirPath string) error {
	esc := strings.ReplaceAll(dirPath, "'", "''")
	cmd := RunHiddenCommand(
		"powershell.exe",
		"-NoProfile",
		"-ExecutionPolicy", "Bypass",
		"-Command",
		fmt.Sprintf("Remove-Item -LiteralPath '%s' -Recurse -Force -ErrorAction SilentlyContinue", esc),
	)
	return cmd.Run()
}

// FastDeleteDirectory removes directory trees rapidly by stripping readonly flags
// and calling native os.RemoveAll (10-20x faster than SHFileOperationW).
// If os.RemoveAll encounters permission or file-locking barriers, it checks the
// Restart Manager (rstrtmgr.dll) or falls back to PowerShell / SHFileOperationW.
func FastDeleteDirectory(path string) error {
	path = filepath.Clean(strings.TrimSpace(path))
	if path == "" || path == "." || path == string(os.PathSeparator) {
		return fmt.Errorf("refusing to delete empty/root path")
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil
	}
	if !isDeletableInstallPath(path) && !isSafeAppLeftoverPath(path) {
		return fmt.Errorf("refusing unsafe path: %s", path)
	}

	// 1) Fast attribute strip (WalkDir, skipping symlinks)
	stripTreeAttributes(path)

	// 2) Native Go os.RemoveAll (10-20x faster than shell operations)
	err := os.RemoveAll(path)
	if err == nil {
		if _, err2 := os.Stat(path); os.IsNotExist(err2) {
			return nil
		}
	}

	// 3) Fallback: if os.RemoveAll encountered error or Access Denied,
	// check Restart Manager for locking processes and retry
	if procs, findErr := FindLockingProcesses(path); findErr == nil && len(procs) > 0 {
		_, _, _ = TerminateLockingProcesses(procs)
	}
	stripTreeAttributes(path)
	if err := os.RemoveAll(path); err == nil {
		if _, err2 := os.Stat(path); os.IsNotExist(err2) {
			return nil
		}
	}

	// 4) Fallback to PowerShell -Recurse -Force
	_ = powershellForceRemove(path)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil
	}

	// 5) Fallback to shell delete (SHFileOperationW)
	if err := shFileDelete(path); err == nil {
		if _, err2 := os.Stat(path); os.IsNotExist(err2) {
			return nil
		}
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil
	}
	return fmt.Errorf("folder still present after fast delete: %s", path)
}

// ForceDeleteAll delegates to FastDeleteDirectory for high-speed tree deletion.
func ForceDeleteAll(path string) error {
	return FastDeleteDirectory(path)
}

// isDeletableInstallPath allows known game/library roots beyond AppData leftovers.
func isDeletableInstallPath(p string) bool {
	p = strings.ToLower(filepath.Clean(p))
	if p == "" || len(p) < 4 {
		return false
	}
	blockedExact := []string{
		`c:\`, `c:\windows`, `c:\program files`, `c:\program files (x86)`,
		`c:\users`, `c:\programdata`,
	}
	for _, b := range blockedExact {
		if p == b {
			return false
		}
	}
	blockedContains := []string{
		`\windows\`, `\system32\`, `\syswow64\`, `\winsxs\`,
		`\microsoft\windows`,
	}
	for _, b := range blockedContains {
		if strings.Contains(p, b) {
			return false
		}
	}
	if isSafeAppLeftoverPath(p) {
		return true
	}
	markers := []string{
		`\steamapps\common\`,
		`\steamapps\sourcemods\`,
		`\epic games\`,
		`\rockstar games\`,
		`\ubisoft\`,
		`\ubisoft game launcher\`,
		`\gog galaxy\games\`,
		`\xboxgames\`,
		`\ea games\`,
		`\origin games\`,
		`\battle.net\`,
		`\riot games\`,
	}
	for _, m := range markers {
		if strings.Contains(p, m) {
			return true
		}
	}
	// Depth >= 2 under a drive letter (e.g. D:\Games\RDR2).
	vol := filepath.VolumeName(p)
	rest := strings.TrimPrefix(p, strings.ToLower(vol))
	rest = strings.TrimPrefix(rest, `\`)
	if vol != "" && strings.Count(rest, `\`) >= 1 && len(rest) >= 5 {
		return true
	}
	return false
}

// openPathInExplorer opens a folder, or selects a file in Explorer.
func openPathInExplorer(folderPath string) error {
	folderPath = filepath.Clean(strings.TrimSpace(folderPath))
	if folderPath == "" {
		return fmt.Errorf("empty path")
	}
	info, err := os.Stat(folderPath)
	if err != nil {
		// Still try to open parent if possible.
		parent := filepath.Dir(folderPath)
		if _, err2 := os.Stat(parent); err2 == nil {
			return exec.Command("explorer.exe", parent).Start()
		}
		return err
	}
	if info.IsDir() {
		return exec.Command("explorer.exe", folderPath).Start()
	}
	return exec.Command("explorer.exe", "/select,", folderPath).Start()
}

// recycleFileOrDir sends folder to Windows Recycle Bin with undo capability.
func recycleFileOrDir(dirPath string) error {
	dirPath = filepath.Clean(strings.TrimSpace(dirPath))
	if dirPath == "" || dirPath == "." || dirPath == string(os.PathSeparator) {
		return fmt.Errorf("refusing to recycle empty/root path")
	}
	stripTreeAttributes(dirPath)

	u, err := syscall.UTF16FromString(dirPath)
	if err != nil {
		return err
	}
	u = append(u, 0) // double null terminator
	fromPtr := &u[0]

	op := shFileOpStructW{
		WFunc:  foDelete,
		PFrom:  fromPtr,
		FFlags: fofAllowUndo | fofSilent | fofNoConfirmation | fofNoErrorUI,
	}
	ret, _, callErr := procSHFileOperationW.Call(uintptr(unsafe.Pointer(&op)))
	if ret != 0 {
		if callErr != nil && callErr != windows.ERROR_SUCCESS {
			return fmt.Errorf("SHFileOperationW recycle code=%d: %v", ret, callErr)
		}
		return fmt.Errorf("SHFileOperationW recycle failed code=%d", ret)
	}
	if op.FAnyOperationsAborted != 0 {
		return fmt.Errorf("recycle operation aborted")
	}
	return nil
}

// restoreFromRecycleBin searches Windows Recycle Bin and restores item by invoking "undelete".
func restoreFromRecycleBin(name string, originalPath string) error {
	escName := strings.ReplaceAll(name, "'", "''")
	script := fmt.Sprintf(`$sh = New-Object -ComObject Shell.Application; $bin = $sh.Namespace(10); foreach ($item in $bin.Items()) { if ($item.Name -eq '%s') { $item.InvokeVerb('undelete'); break } }`, escName)
	cmd := RunHiddenCommand(
		"powershell.exe",
		"-NoProfile",
		"-ExecutionPolicy", "Bypass",
		"-Command",
		script,
	)
	_ = cmd.Run()
	time.Sleep(350 * time.Millisecond)
	if _, err := os.Stat(originalPath); err == nil {
		return nil
	}
	return fmt.Errorf("could not restore '%s' from recycle bin", name)
}
