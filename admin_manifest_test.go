package main

import (
	"os"
	"strings"
	"testing"
)

func TestWindowsManifestRequiresAdministrator(t *testing.T) {
	data, err := os.ReadFile("build/windows/wails.exe.manifest")
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	text := string(data)
	if !strings.Contains(text, `level="requireAdministrator"`) {
		t.Fatal("wails.exe.manifest must request requireAdministrator")
	}
	if strings.Contains(text, `level="asInvoker"`) {
		t.Fatal("wails.exe.manifest must not request asInvoker")
	}
}

func TestPEInfoVersionAndIdentity(t *testing.T) {
	info, err := os.ReadFile("build/windows/info.json")
	if err != nil {
		t.Fatalf("read info.json: %v", err)
	}
	s := string(info)
	if !strings.Contains(s, "PulseCore System Monitor & Utility") {
		t.Fatal("info.json FileDescription missing")
	}
	if !strings.Contains(s, `"ProductName": "{{.Info.ProductName}}"`) {
		t.Fatal("info.json ProductName template missing")
	}
	wails, err := os.ReadFile("wails.json")
	if err != nil {
		t.Fatalf("read wails.json: %v", err)
	}
	w := string(wails)
	if !strings.Contains(w, `"productVersion": "1.0.0"`) {
		t.Fatal("wails.json productVersion must be 1.0.0")
	}
	if !strings.Contains(w, `"companyName": "robbled17"`) {
		t.Fatal("wails.json companyName must be robbled17")
	}
	if appVersion != "1.0.0" {
		t.Fatalf("appVersion=%q want 1.0.0", appVersion)
	}
}
