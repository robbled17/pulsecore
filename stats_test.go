package main

import (
	"fmt"
	"testing"
)

func TestTakeLastAndPrevDays(t *testing.T) {
	days := make([]DayStats, 0, 20)
	for i := 1; i <= 14; i++ {
		days = append(days, DayStats{Date: fmt.Sprintf("2026-01-%02d", i), Incidents: i})
	}
	last := takeLastDays(days, 7)
	if len(last) != 7 || last[0].Date != "2026-01-08" || last[6].Date != "2026-01-14" {
		t.Fatalf("last=%v", last)
	}
	prev := takePrevDays(days, 7, 7)
	if len(prev) != 7 || prev[0].Date != "2026-01-01" || prev[6].Date != "2026-01-07" {
		t.Fatalf("prev=%v", prev)
	}
}

func TestTopCulprit(t *testing.T) {
	if topCulprit(map[string]int{"a.exe": 1, "b.exe": 5}) != "b.exe" {
		t.Fatal("expected b.exe")
	}
}
