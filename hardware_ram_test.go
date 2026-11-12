package main

import (
	"strings"
	"testing"
)

func TestFormatRAMModuleModelCommon(t *testing.T) {
	got := formatRAMModuleModel("Kingston", "KF432C16BBK2/16", 2, 2*8*1024*1024*1024)
	if !strings.Contains(got, "2") || !strings.Contains(got, "8") {
		t.Fatalf("got %q", got)
	}
	if strings.Contains(got, "2353") {
		t.Fatalf("absurd size: %q", got)
	}
}

func TestFormatRAMModuleModelRejectsAbsurd(t *testing.T) {
	got := formatRAMModuleModel("X", "PART", 2, 2*2353*1024*1024*1024)
	if strings.Contains(got, "2353") {
		t.Fatalf("should omit absurd size: %q", got)
	}
}
