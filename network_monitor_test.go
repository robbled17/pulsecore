package main

import (
	"testing"
	"time"
)

func TestGetNetworkMonitorSystemCounters(t *testing.T) {
	a := &App{}
	s1 := a.GetNetworkMonitor()
	if !s1.OK {
		t.Skip("network counters unavailable in this environment")
	}
	time.Sleep(200 * time.Millisecond)
	s2 := a.GetNetworkMonitor()
	if !s2.OK {
		t.Fatal("second sample should stay OK")
	}
	if s2.PerProcessNoteRu == "" || s2.PerProcessNoteEn == "" {
		t.Fatal("honesty note required")
	}
	// Session totals should be non-negative
	if s2.SessionSendMB < 0 || s2.SessionRecvMB < 0 {
		t.Fatal("negative session totals")
	}
}

func TestSampleProcessConnectionsShape(t *testing.T) {
	list := sampleProcessConnections(5)
	for _, e := range list {
		if e.PID <= 0 {
			t.Fatal("bad pid")
		}
		if e.RatesKnown {
			t.Fatal("must not claim per-process rates without a driver")
		}
		if e.ConnCount <= 0 {
			t.Fatal("conn count")
		}
	}
}
