//go:build windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	foDelete          = 0x0003
	fofSilent         = 0x0004
	fofNoConfirmation = 0x0010
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
	_ = filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
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

// ForceDeleteAll clears attributes, then deletes via SHFileOperationW (Shift+Delete),
// with PowerShell and os.RemoveAll as fallbacks — needed for 100GB+ game trees.
func ForceDeleteAll(path string) error {
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

	stripTreeAttributes(path)

	// 1) Shell delete (handles long trees / readonly better than RemoveAll).
	if err := shFileDelete(path); err == nil {
		if _, err2 := os.Stat(path); os.IsNotExist(err2) {
			return nil
		}
	}

	// 2) PowerShell -Recurse -Force
	_ = powershellForceRemove(path)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil
	}

	// 3) Last resort: Go RemoveAll after another attribute pass.
	stripTreeAttributes(path)
	_ = os.RemoveAll(path)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil
	}
	return fmt.Errorf("folder still present after force delete: %s", path)
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
