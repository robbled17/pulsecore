package main

import (
	"errors"
	"strings"
	"testing"
)

func TestHumanEcoErr(t *testing.T) {
	if got := humanEcoErr(errors.New("access denied")); got != "нет доступа" {
		t.Fatalf("got %q", got)
	}
	if got := humanEcoErr(errors.New("protected process")); got != "защищённый процесс" {
		t.Fatalf("got %q", got)
	}
}

func TestFormatEcoFails(t *testing.T) {
	msg := formatEcoFailsRu([]EcoFail{{Name: "chrome.exe", PID: 1, Reason: "нет доступа"}})
	if !strings.Contains(msg, "chrome.exe") || !strings.Contains(msg, "нет доступа") {
		t.Fatalf("msg=%q", msg)
	}
}
