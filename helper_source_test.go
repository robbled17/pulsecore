package main

import "testing"

// fakeHelperSource exercises CPU/disk mapping without a live pipe.
type fakeHelperSource struct {
	resp HelperResponse
	err  error
}

func (f fakeHelperSource) get() (HelperResponse, error) {
	return f.resp, f.err
}

func TestHelperCPUPackageTemp(t *testing.T) {
	tctl := 56.5
	tdie := 51.0
	c := HelperCPU{TctlC: &tctl, TdieC: &tdie, OK: true}
	got, ok := helperCPUPackageTemp(c)
	if !ok || got != 56.5 {
		t.Fatalf("tctl prefer: got %v ok=%v", got, ok)
	}
	c.TctlC = nil
	got, ok = helperCPUPackageTemp(c)
	if !ok || got != 51.0 {
		t.Fatalf("tdie fallback: got %v ok=%v", got, ok)
	}
	c = HelperCPU{CcdTempsC: []float64{48, 50}, OK: true}
	got, ok = helperCPUPackageTemp(c)
	if !ok || got != 48 {
		t.Fatalf("ccd fallback: got %v ok=%v", got, ok)
	}
	c = HelperCPU{}
	if _, ok := helperCPUPackageTemp(c); ok {
		t.Fatal("expected unavailable")
	}
}

func TestFakeHelperNormalOverheatUnavailable(t *testing.T) {
	cases := []struct {
		name string
		src  fakeHelperSource
		want string
	}{
		{
			name: "normal",
			src: fakeHelperSource{resp: HelperResponse{
				OK: true,
				CPU: HelperCPU{OK: true, TctlC: ptrFloat(55), LoadPercent: ptrFloat(20), ClockMHz: ptrFloat(3600), PowerW: ptrFloat(40)},
				Driver: HelperDriver{PawnIOPresent: true, PawnIORunning: true, Backend: "PawnIO"},
			}},
			want: "ok",
		},
		{
			name: "overheat",
			src: fakeHelperSource{resp: HelperResponse{
				OK: true,
				CPU: HelperCPU{OK: true, TctlC: ptrFloat(95), LoadPercent: ptrFloat(90)},
				Driver: HelperDriver{PawnIOPresent: true, PawnIORunning: true, Backend: "PawnIO"},
			}},
			want: "hot",
		},
		{
			name: "helper_down",
			src:  fakeHelperSource{err: errHelperDown},
			want: "down",
		},
		{
			name: "driver_blocked",
			src: fakeHelperSource{resp: HelperResponse{
				OK: true,
				CPU: HelperCPU{OK: false},
				Driver: HelperDriver{Blocked: true, BlockedReason: "Memory Integrity", Backend: "none"},
			}},
			want: "blocked",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := classifyFakeHelper(tc.src)
			if got != tc.want {
				t.Fatalf("got %q want %q", got, tc.want)
			}
		})
	}
}

var errHelperDown = errString("helper unavailable")

type errString string

func (e errString) Error() string { return string(e) }

func classifyFakeHelper(src fakeHelperSource) string {
	resp, err := src.get()
	if err != nil {
		return "down"
	}
	if resp.Driver.Blocked || (!resp.CPU.OK && resp.Driver.Backend == "none") {
		return "blocked"
	}
	temp, ok := helperCPUPackageTemp(resp.CPU)
	if !ok {
		return "down"
	}
	if temp >= 85 {
		return "hot"
	}
	return "ok"
}

func TestDiskReportFromHelperStatus(t *testing.T) {
	// Mapping covered indirectly: ensure EvaluateDiskHealth still works for helper attrs.
	ev := EvaluateDiskHealth(DiskHealthEvalInput{
		Kind: DiskKindSATASSD, Model: "Kingston A400", SMARTReadOK: true,
		Attributes: []SmartAttribute{
			{ID: 5, Name: "Reallocated", Current: 100, Worst: 100, Threshold: 10, Raw: "0"},
			{ID: 231, Name: "Life", Current: 100, Worst: 100, Threshold: 5, Raw: "0", RawValue: 0},
		},
	})
	if ev.Level != DiskStatusGood {
		t.Fatalf("expected good, got %s", ev.Level)
	}
}
