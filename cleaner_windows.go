//go:build windows

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	sherbNoConfirmation   = 0x00000001
	sherbNoProgressUI     = 0x00000002
	sherbNoSound          = 0x00000004
	seeMaskNoCloseProcess = 0x00000040
)

var (
	modShell32             = windows.NewLazySystemDLL("shell32.dll")
	procSHEmptyRecycleBinW = modShell32.NewProc("SHEmptyRecycleBinW")
	procSHQueryRecycleBinW = modShell32.NewProc("SHQueryRecycleBinW")
	procShellExecuteExW    = modShell32.NewProc("ShellExecuteExW")
)

type shellExecuteInfoW struct {
	cbSize       uint32
	fMask        uint32
	hwnd         windows.Handle
	lpVerb       *uint16
	lpFile       *uint16
	lpParameters *uint16
	lpDirectory  *uint16
	nShow        int32
	hInstApp     windows.Handle
	lpIDList     uintptr
	lpClass      *uint16
	hkeyClass    windows.Handle
	dwHotKey     uint32
	hIcon        windows.Handle
	hProcess     windows.Handle
}

// emptyRecycleBinNative calls SHEmptyRecycleBinW for all drives (hwnd=0, pszRootPath=nil).
func emptyRecycleBinNative() error {
	r, _, err := procSHEmptyRecycleBinW.Call(
		0,
		0, // nil = all bins
		uintptr(sherbNoConfirmation|sherbNoProgressUI|sherbNoSound),
	)
	if r != 0 {
		// S_OK == 0; non-zero HRESULT
		if err != windows.ERROR_SUCCESS && err != nil {
			return err
		}
		return windows.Errno(r)
	}
	_ = unsafe.Sizeof(r)
	return nil
}

// queryRecycleBinBytes uses SHQueryRecycleBinW (all drives). -1 means the call failed.
func queryRecycleBinBytes() int64 {
	// SHQUERYRBINFO: DWORD cbSize, padding, int64 i64Size, int64 i64NumItems.
	var info struct {
		cbSize uint32
		_      uint32
		size   int64
		items  int64
	}
	info.cbSize = uint32(unsafe.Sizeof(info))
	r, _, _ := procSHQueryRecycleBinW.Call(0, uintptr(unsafe.Pointer(&info)))
	if r != 0 {
		return -1
	}
	if info.size < 0 {
		return 0
	}
	return info.size
}

// runElevatedPowerShell runs PowerShell code with UTF-8 encoding and elevation when needed.
func runElevatedPowerShell(script string) (string, error) {
	// Always enforce UTF-8 output encoding to avoid CP866 mojibake
	prefixedScript := "[Console]::OutputEncoding = [System.Text.Encoding]::UTF8; $OutputEncoding = [System.Text.Encoding]::UTF8;\r\n" + script

	if IsProcessElevated() {
		out, err := runHiddenCombinedOutput(
			"powershell.exe",
			"-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass",
			"-WindowStyle", "Hidden", "-Command", prefixedScript,
		)
		return string(out), err
	}

	// Non-elevated: launch elevated PowerShell via ShellExecuteExW with verb 'runas'
	tmpScript := filepath.Join(os.TempDir(), fmt.Sprintf("pc_elev_%d.ps1", time.Now().UnixNano()))
	tmpOut := filepath.Join(os.TempDir(), fmt.Sprintf("pc_elev_%d.out", time.Now().UnixNano()))
	defer os.Remove(tmpScript)
	defer os.Remove(tmpOut)

	wrapper := fmt.Sprintf(`[Console]::OutputEncoding = [System.Text.Encoding]::UTF8; $OutputEncoding = [System.Text.Encoding]::UTF8;
try {
  & {
%s
  } *>&1 | Out-File -FilePath %s -Encoding utf8
} catch {
  $_ | Out-File -FilePath %s -Encoding utf8
  exit 1
}`, script, quotePS(tmpOut), quotePS(tmpOut))

	if err := os.WriteFile(tmpScript, []byte(wrapper), 0600); err != nil {
		return "", err
	}

	verbPtr, err := windows.UTF16PtrFromString("runas")
	if err != nil {
		return "", err
	}
	filePtr, err := windows.UTF16PtrFromString("powershell.exe")
	if err != nil {
		return "", err
	}
	params := fmt.Sprintf("-NoProfile -ExecutionPolicy Bypass -WindowStyle Hidden -File %s", quotePS(tmpScript))
	paramsPtr, err := windows.UTF16PtrFromString(params)
	if err != nil {
		return "", err
	}

	var sei shellExecuteInfoW
	sei.cbSize = uint32(unsafe.Sizeof(sei))
	sei.fMask = seeMaskNoCloseProcess
	sei.lpVerb = verbPtr
	sei.lpFile = filePtr
	sei.lpParameters = paramsPtr
	sei.nShow = windows.SW_HIDE

	r1, _, execErr := procShellExecuteExW.Call(uintptr(unsafe.Pointer(&sei)))
	if r1 == 0 {
		return "", execErr
	}
	if sei.hProcess != 0 {
		defer windows.CloseHandle(sei.hProcess)
		_, _ = windows.WaitForSingleObject(sei.hProcess, 60000)
	}

	b, _ := os.ReadFile(tmpOut)
	return string(b), nil
}

func quotePS(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}
