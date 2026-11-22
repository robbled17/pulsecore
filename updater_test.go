package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCompareSemver(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"1.0.0", "1.0.1", -1},
		{"v1.0.0", "v1.0.1", -1},
		{"1.0.0", "1.0.0", 0},
		{"v1.0.0", "1.0.0", 0},
		{"1.0.1", "1.0.0", 1},
		{"1.1.0", "1.0.9", 1},
		{"2.0.0", "1.9.9", 1},
		{"1.0", "1.0.0", 0},
		{"1.0.0", "1.0", 0},
		{"1.0.0-rc1", "1.0.0", 0},
	}
	for _, tc := range cases {
		got := compareSemver(tc.a, tc.b)
		if got != tc.want {
			t.Fatalf("compareSemver(%q, %q) = %d, want %d", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestNormalizeVersion(t *testing.T) {
	if got := normalizeVersion(" v1.0.0 "); got != "1.0.0" {
		t.Fatalf("normalizeVersion got %q", got)
	}
}

func TestCheckForUpdatesOnline(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"tag_name": "v1.0.1",
			"body":     "## PulseCore v1.0.1\n\n- Demo notes line\n- Second line",
			"assets": []map[string]string{
				{
					"name":                 "pulsecore.exe",
					"browser_download_url": "https://example.com/pulsecore.exe",
				},
			},
		})
	}))
	defer srv.Close()

	res := checkForUpdates(context.Background(), "1.0.0", srv.URL, 2*time.Second)
	if res.Status != updateStatusAvailable {
		t.Fatalf("status=%q want %q notes=%q", res.Status, updateStatusAvailable, res.Notes)
	}
	if res.LatestVersion != "1.0.1" {
		t.Fatalf("latest=%q", res.LatestVersion)
	}
	if res.DownloadURL != "https://example.com/pulsecore.exe" {
		t.Fatalf("download=%q", res.DownloadURL)
	}
	if res.CurrentVersion != "1.0.0" {
		t.Fatalf("current=%q", res.CurrentVersion)
	}
}

func TestCheckForUpdatesUpToDate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"tag_name": "v1.0.0",
			"body":     "same",
			"assets":   []any{},
		})
	}))
	defer srv.Close()

	res := checkForUpdates(context.Background(), "v1.0.0", srv.URL, 2*time.Second)
	if res.Status != updateStatusUpToDate {
		t.Fatalf("status=%q", res.Status)
	}
}

func TestCheckForUpdatesOffline(t *testing.T) {
	res := checkForUpdates(context.Background(), "1.0.0", "http://127.0.0.1:1/nope", 200*time.Millisecond)
	if res.Status != updateStatusOffline {
		t.Fatalf("status=%q want offline", res.Status)
	}
	if res.CurrentVersion != "1.0.0" {
		t.Fatalf("current=%q", res.CurrentVersion)
	}
}
