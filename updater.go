package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	appVersion            = "1.0.0"
	githubLatestAPI       = "https://api.github.com/repos/robbled17/pulsecore/releases/latest"
	githubExeDownloadURL  = "https://github.com/robbled17/pulsecore/releases/latest/download/pulsecore.exe"
	updateCheckTimeout    = 4 * time.Second
	updateStatusUpToDate  = "up_to_date"
	updateStatusAvailable = "update_available"
	updateStatusOffline   = "offline"
)

// UpdateCheckResult is returned by CheckForUpdates.
type UpdateCheckResult struct {
	Status         string `json:"status"`
	CurrentVersion string `json:"currentVersion"`
	LatestVersion  string `json:"latestVersion"`
	Notes          string `json:"notes"`
	DownloadURL    string `json:"downloadURL"`
}

type githubReleaseLatest struct {
	TagName string `json:"tag_name"`
	Body    string `json:"body"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

// AppVersion returns the embedded product version (no leading v).
func AppVersion() string {
	return appVersion
}

// CheckForUpdates queries GitHub Releases latest with a short timeout.
// Offline or API failures return status "offline" without panicking.
func (a *App) CheckForUpdates() UpdateCheckResult {
	return checkForUpdates(context.Background(), appVersion, githubLatestAPI, updateCheckTimeout)
}

func checkForUpdates(ctx context.Context, current string, apiURL string, timeout time.Duration) UpdateCheckResult {
	cur := normalizeVersion(current)
	out := UpdateCheckResult{
		Status:         updateStatusOffline,
		CurrentVersion: cur,
		LatestVersion:  "",
		Notes:          "",
		DownloadURL:    githubExeDownloadURL,
	}

	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, apiURL, nil)
	if err != nil {
		return out
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "PulseCore/"+cur)

	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return out
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return out
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return out
	}

	var rel githubReleaseLatest
	if err := json.Unmarshal(body, &rel); err != nil {
		return out
	}

	latest := normalizeVersion(rel.TagName)
	if latest == "" {
		return out
	}

	out.LatestVersion = latest
	out.Notes = shortReleaseNotes(rel.Body)
	out.DownloadURL = pickExeDownloadURL(rel)
	cmp := compareSemver(cur, latest)
	if cmp < 0 {
		out.Status = updateStatusAvailable
	} else {
		out.Status = updateStatusUpToDate
	}
	return out
}

func pickExeDownloadURL(rel githubReleaseLatest) string {
	for _, a := range rel.Assets {
		name := strings.ToLower(strings.TrimSpace(a.Name))
		if name == "pulsecore.exe" && strings.TrimSpace(a.BrowserDownloadURL) != "" {
			return a.BrowserDownloadURL
		}
	}
	for _, a := range rel.Assets {
		name := strings.ToLower(strings.TrimSpace(a.Name))
		if strings.HasSuffix(name, ".exe") && strings.Contains(name, "pulsecore") && strings.TrimSpace(a.BrowserDownloadURL) != "" {
			return a.BrowserDownloadURL
		}
	}
	return githubExeDownloadURL
}

func shortReleaseNotes(body string) string {
	body = strings.ReplaceAll(body, "\r\n", "\n")
	lines := strings.Split(body, "\n")
	var picked []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "#") {
			continue
		}
		picked = append(picked, line)
		if len(picked) >= 3 {
			break
		}
	}
	notes := strings.Join(picked, " ")
	if len(notes) > 220 {
		notes = notes[:217] + "..."
	}
	return notes
}

func normalizeVersion(v string) string {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "v")
	v = strings.TrimPrefix(v, "V")
	return v
}

// compareSemver returns -1 if a < b, 0 if equal, 1 if a > b.
// Non-numeric / empty segments are treated as 0.
func compareSemver(a, b string) int {
	a = normalizeVersion(a)
	b = normalizeVersion(b)
	as := strings.Split(a, ".")
	bs := strings.Split(b, ".")
	n := len(as)
	if len(bs) > n {
		n = len(bs)
	}
	for i := 0; i < n; i++ {
		var av, bv int
		if i < len(as) {
			av = parseSemverPart(as[i])
		}
		if i < len(bs) {
			bv = parseSemverPart(bs[i])
		}
		if av < bv {
			return -1
		}
		if av > bv {
			return 1
		}
	}
	return 0
}

func parseSemverPart(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	// Strip pre-release / build metadata for numeric compare (1.0.0-rc1 -> 1.0.0)
	for i, r := range s {
		if r < '0' || r > '9' {
			s = s[:i]
			break
		}
	}
	if s == "" {
		return 0
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return n
}

// FormatAppVersionLabel returns a display tag like "v1.0.0".
func FormatAppVersionLabel() string {
	return fmt.Sprintf("v%s", appVersion)
}
