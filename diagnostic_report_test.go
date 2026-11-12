package main

import (
	"strings"
	"testing"
)

func TestPreviewDiagnosticReport(t *testing.T) {
	a := &App{}
	p := a.PreviewDiagnosticReport()
	if len(p.Includes) == 0 || len(p.Excludes) == 0 {
		t.Fatal("preview must list includes/excludes")
	}
	if len(p.Formats) != 2 {
		t.Fatal("txt+json")
	}
}

func TestExportDiagnosticReportTXTJSON(t *testing.T) {
	a := &App{}
	a.metrics.CPUName = "TestCPU"
	a.metrics.RAMTotalMB = 16384
	a.cpuName = "TestCPU"
	txt := a.ExportDiagnosticReport("txt")
	if !txt.OK || !strings.Contains(txt.Content, "Диагностический отчёт PulseCore") {
		t.Fatalf("txt: %+v", txt)
	}
	js := a.ExportDiagnosticReport("json")
	if !js.OK || !strings.Contains(js.Content, `"pulseCoreVersion"`) {
		t.Fatalf("json: %+v", js)
	}
	bad := a.ExportDiagnosticReport("xml")
	if bad.OK {
		t.Fatal("xml must fail")
	}
}

func TestFilepathBaseSafe(t *testing.T) {
	if filepathBaseSafe(`C:\Users\Bob\App\chrome.exe`) != "chrome.exe" {
		t.Fatal("base")
	}
}
