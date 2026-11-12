//go:build windows

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// runPawnIOSetup launches the official PawnIO_setup.exe with -install or -uninstall.
func runPawnIOSetup(setupPath string, action string) error {
	setupPath = strings.TrimSpace(setupPath)
	action = strings.TrimSpace(strings.ToLower(action))
	if action != "-install" && action != "install" && action != "-uninstall" && action != "uninstall" {
		return fmt.Errorf("pawnio action must be -install or -uninstall")
	}
	if !strings.HasPrefix(action, "-") {
		action = "-" + action
	}
	if setupPath == "" {
		return fmt.Errorf("empty pawnio setup path")
	}
	if _, err := os.Stat(setupPath); err != nil {
		return err
	}
	appendUILog("PawnIO setup " + action + " path=" + setupPath)
	// Elevated UAC path (same as helper install) - required for driver service.
	if err := runElevatedFireAndForget(setupPath, action); err != nil {
		return err
	}
	deadline := time.Now().Add(90 * time.Second)
	for time.Now().Before(deadline) {
		if action == "-install" {
			if pawnIOPresent() || pawnIORunning() {
				return nil
			}
		} else {
			if !pawnIOPresent() && !pawnIORunning() {
				return nil
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	// Setup may have succeeded without our probes catching it yet.
	if action == "-install" && (pawnIOPresent() || pawnIORunning()) {
		return nil
	}
	if action == "-uninstall" && !pawnIOPresent() {
		return nil
	}
	return fmt.Errorf("pawnio %s timed out", action)
}

func shouldSkipPawnIOInstall() bool {
	return pawnIORunning() || pawnIOPresent()
}

func uninstallPawnIOOwned() {
	// Prefer official setup with -uninstall; fall back to Program Files uninstaller.
	setup := filepath.Join(programDataPulseCore(), "pawnio", "PawnIO_setup.exe")
	if _, err := os.Stat(setup); err == nil {
		_ = runPawnIOSetup(setup, "-uninstall")
		return
	}
	uninstaller := `C:\Program Files\PawnIO\uninstall.exe`
	if _, err := os.Stat(uninstaller); err == nil {
		_ = runElevatedFireAndForget(uninstaller, "-uninstall")
		time.Sleep(3 * time.Second)
	}
}
