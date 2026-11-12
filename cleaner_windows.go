//go:build windows

package main

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	sherbNoConfirmation = 0x00000001
	sherbNoProgressUI   = 0x00000002
	sherbNoSound        = 0x00000004
)

var (
	modShell32            = windows.NewLazySystemDLL("shell32.dll")
	procSHEmptyRecycleBinW = modShell32.NewProc("SHEmptyRecycleBinW")
)

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
