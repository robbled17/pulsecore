package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"time"
)

func logCrash(component string, r interface{}) {
	defer func() { _ = recover() }()
	local := os.Getenv("LOCALAPPDATA")
	if local == "" {
		local = os.TempDir()
	}
	dir := filepath.Join(local, "PulseCore")
	_ = os.MkdirAll(dir, 0o755)
	logFile := filepath.Join(dir, "crash.log")

	stack := string(debug.Stack())
	msg := fmt.Sprintf("=== CRASH [%s] in %s ===\nPanic: %v\nStack:\n%s\n============================\n\n",
		time.Now().Format("2006-01-02 15:04:05"), component, r, stack)

	f, err := os.OpenFile(logFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err == nil {
		_, _ = f.WriteString(msg)
		_ = f.Close()
	}
	fmt.Fprint(os.Stderr, msg)
}
