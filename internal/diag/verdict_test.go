package diag

import (
	"testing"
	"time"
)

func TestVerdictRAMPressure(t *testing.T) {
	hit := Snapshot{At: time.Now(), RAMFreeMB: 120, CPUPercent: 40}
	v := Evaluate(hit, nil, 400)
	if v.Cause != CauseRAMPressure || v.Level != LevelCrit {
		t.Fatalf("got %+v", v)
	}
}

func TestVerdictDiskBusy(t *testing.T) {
	hit := Snapshot{
		At:            time.Now(),
		RAMFreeMB:     2000,
		DiskReadMBps:  30,
		DiskWriteMBps: 20,
		TopProcesses:  []ProcessSample{{Name: "SearchIndexer.exe", DiskMBps: 25}},
	}
	v := Evaluate(hit, nil, 500)
	if v.Cause != CauseDiskBusy {
		t.Fatalf("cause=%s", v.Cause)
	}
	if v.Culprit != "SearchIndexer.exe" {
		t.Fatalf("culprit=%s", v.Culprit)
	}
}

func TestVerdictUnknownWhenWeak(t *testing.T) {
	hit := Snapshot{At: time.Now(), RAMFreeMB: 4000, CPUPercent: 12, GPUPercent: 5}
	v := Evaluate(hit, nil, 320)
	if v.Level != LevelUnknown || v.Cause != CauseUnknown {
		t.Fatalf("expected UNKNOWN, got %+v", v)
	}
}

func TestVerdictProcessHot(t *testing.T) {
	hit := Snapshot{
		At:         time.Now(),
		RAMFreeMB:  3000,
		CPUPercent: 92,
		TopProcesses: []ProcessSample{
			{Name: "chrome.exe", CPUPercent: 55},
		},
	}
	v := Evaluate(hit, nil, 350)
	if v.Cause != CauseProcess || v.Culprit != "chrome.exe" {
		t.Fatalf("got %+v", v)
	}
}

func TestSmoothnessWarmupAndWatching(t *testing.T) {
	s := SmoothnessFromIncidents(nil, 2)
	if s["messageKey"] != "verdictWarmup" {
		t.Fatalf("warmup: %+v", s)
	}
	s = SmoothnessFromIncidents(nil, 10)
	if s["messageKey"] != "verdictWatching" {
		t.Fatalf("watching: %+v", s)
	}
}

func TestBlackBoxRecordHitch(t *testing.T) {
	bb := NewBlackBox()
	bb.PushSnapshot(Snapshot{
		At:         time.Now(),
		RAMFreeMB:  100,
		CPUPercent: 50,
	})
	inc := bb.RecordHitch(450, "flight")
	if inc.Verdict.Cause != CauseRAMPressure {
		t.Fatalf("verdict=%+v", inc.Verdict)
	}
	if len(bb.Incidents()) != 1 {
		t.Fatalf("incidents=%d", len(bb.Incidents()))
	}
	sm := bb.SmoothnessStatus()
	if sm["level"] != string(LevelCrit) {
		t.Fatalf("smoothness=%+v", sm)
	}
}
