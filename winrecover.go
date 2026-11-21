package main

import (
	"sync"
	"time"
)

const defaultSensorTimeout = 4 * time.Second

// withRecoverTimeout runs fn in a child goroutine with panic recovery.
// If fn does not finish within timeout, the caller returns without waiting
// (the child may still finish later; used only for best-effort WMI/WinAPI probes).
// Never use for destructive operations.
func withRecoverTimeout(timeout time.Duration, fn func()) (finished bool) {
	if timeout <= 0 {
		timeout = defaultSensorTimeout
	}
	done := make(chan struct{})
	go func() {
		defer func() {
			_ = recover()
			close(done)
		}()
		fn()
	}()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-done:
		return true
	case <-timer.C:
		return false
	}
}

// withRecover runs fn and swallows panics. Prefer withRecoverTimeout for WMI.
func withRecover(fn func()) {
	defer func() { _ = recover() }()
	fn()
}

// sensorGate serializes OLE/WMI probes so overlapping UI polls do not stack.
var sensorGate sync.Mutex

func withSensorGate(timeout time.Duration, fn func()) bool {
	locked := make(chan struct{})
	go func() {
		sensorGate.Lock()
		close(locked)
	}()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-locked:
		defer sensorGate.Unlock()
		return withRecoverTimeout(timeout, fn)
	case <-timer.C:
		return false
	}
}
