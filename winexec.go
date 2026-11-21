package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

const (
	hiddenCmdTimeout = 5 * time.Second
	createNoWindow   = 0x08000000 // CREATE_NO_WINDOW
)

func windowsRoot() string {
	root := os.Getenv("SystemRoot")
	if root == "" {
		root = `C:\Windows`
	}
	return root
}

// system32 returns an absolute path under %SystemRoot%\System32 to avoid PATH hijack.
func system32(name string) string {
	base := filepath.Base(strings.TrimSpace(name))
	if base == "" || base == "." || base == ".." {
		return name
	}
	return filepath.Join(windowsRoot(), "System32", base)
}

// systemPowerShell is the boxed Windows PowerShell host (no PATH lookup).
func systemPowerShell() string {
	return filepath.Join(windowsRoot(), "System32", "WindowsPowerShell", "v1.0", "powershell.exe")
}

// nvidiaSMIPath returns a known absolute nvidia-smi location or empty if missing.
// Never relies on PATH to avoid hijack.
func nvidiaSMIPath() string {
	pf := os.Getenv("ProgramFiles")
	candidates := []string{
		filepath.Join(windowsRoot(), "System32", "nvidia-smi.exe"),
	}
	if pf != "" {
		candidates = append([]string{
			filepath.Join(pf, "NVIDIA Corporation", "NVSMI", "nvidia-smi.exe"),
		}, candidates...)
	}
	for _, c := range candidates {
		if strings.TrimSpace(c) == "" {
			continue
		}
		if st, err := os.Stat(c); err == nil && !st.IsDir() {
			return c
		}
	}
	return ""
}

func hiddenSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: createNoWindow,
	}
}

func runHiddenCommand(name string, args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), hiddenCmdTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.SysProcAttr = hiddenSysProcAttr()
	return cmd.Run()
}

func runHiddenOutput(name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), hiddenCmdTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.SysProcAttr = hiddenSysProcAttr()
	return cmd.Output()
}
