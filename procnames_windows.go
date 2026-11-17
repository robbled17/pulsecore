//go:build windows

package main

import (
	"path/filepath"
	"strings"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	modVersionDll               = windows.NewLazySystemDLL("version.dll")
	procGetFileVersionInfoSizeW = modVersionDll.NewProc("GetFileVersionInfoSizeW")
	procGetFileVersionInfoW     = modVersionDll.NewProc("GetFileVersionInfoW")
	procVerQueryValueW          = modVersionDll.NewProc("VerQueryValueW")

	versionInfoCache   = make(map[string]string)
	versionInfoCacheMu sync.RWMutex
)

// getProcessVersionInfo retrieves FileDescription and CompanyName using Win32 VersionInfo API.
func getProcessVersionInfo(exePath string) string {
	cleanPath := filepath.Clean(strings.TrimSpace(exePath))
	if cleanPath == "" {
		return ""
	}

	versionInfoCacheMu.RLock()
	if cached, ok := versionInfoCache[cleanPath]; ok {
		versionInfoCacheMu.RUnlock()
		return cached
	}
	versionInfoCacheMu.RUnlock()

	desc := extractVersionInfoWin32(cleanPath)

	versionInfoCacheMu.Lock()
	if len(versionInfoCache) > 1000 {
		versionInfoCache = make(map[string]string)
	}
	versionInfoCache[cleanPath] = desc
	versionInfoCacheMu.Unlock()

	return desc
}

func extractVersionInfoWin32(exePath string) string {
	pathPtr, err := windows.UTF16PtrFromString(exePath)
	if err != nil {
		return ""
	}

	var dummyHandle uint32
	size, _, _ := procGetFileVersionInfoSizeW.Call(
		uintptr(unsafe.Pointer(pathPtr)),
		uintptr(unsafe.Pointer(&dummyHandle)),
	)
	if size == 0 {
		return ""
	}

	data := make([]byte, size)
	ret, _, _ := procGetFileVersionInfoW.Call(
		uintptr(unsafe.Pointer(pathPtr)),
		0,
		size,
		uintptr(unsafe.Pointer(&data[0])),
	)
	if ret == 0 {
		return ""
	}

	// Try common language sub-blocks for FileDescription
	subBlocks := []string{
		`\StringFileInfo\040904b0\FileDescription`,
		`\StringFileInfo\041904b0\FileDescription`,
		`\StringFileInfo\000004b0\FileDescription`,
		`\StringFileInfo\040904e4\FileDescription`,
		`\StringFileInfo\041904e4\FileDescription`,
	}

	fileDesc := ""
	for _, sb := range subBlocks {
		val := queryVerValue(data, sb)
		if val != "" {
			fileDesc = val
			break
		}
	}

	// Also check CompanyName if available
	companyName := ""
	for _, sb := range []string{
		`\StringFileInfo\040904b0\CompanyName`,
		`\StringFileInfo\041904b0\CompanyName`,
	} {
		val := queryVerValue(data, sb)
		if val != "" {
			companyName = val
			break
		}
	}

	if fileDesc != "" && companyName != "" && !strings.Contains(strings.ToLower(fileDesc), strings.ToLower(companyName)) {
		return fileDesc + " (" + companyName + ")"
	}
	return fileDesc
}

func queryVerValue(data []byte, subBlock string) string {
	if len(data) == 0 {
		return ""
	}
	sbPtr, err := windows.UTF16PtrFromString(subBlock)
	if err != nil {
		return ""
	}

	var bufPtr uintptr
	var bufLen uint32
	ret, _, _ := procVerQueryValueW.Call(
		uintptr(unsafe.Pointer(&data[0])),
		uintptr(unsafe.Pointer(sbPtr)),
		uintptr(unsafe.Pointer(&bufPtr)),
		uintptr(unsafe.Pointer(&bufLen)),
	)
	if ret != 0 && bufPtr != 0 && bufLen > 0 {
		// bufPtr points to a null-terminated UTF-16 string
		chars := (*[2048]uint16)(unsafe.Pointer(bufPtr))
		length := int(bufLen)
		if length > 2048 {
			length = 2048
		}
		// Strip trailing null if included in bufLen
		for length > 0 && chars[length-1] == 0 {
			length--
		}
		if length > 0 {
			return strings.TrimSpace(windows.UTF16ToString(chars[:length]))
		}
	}
	return ""
}
