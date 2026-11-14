//go:build windows

package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// CleanupOldBinary removes any leftover .old binary from previous update.
func (a *App) CleanupOldBinary() {
	exePath, err := os.Executable()
	if err != nil {
		return
	}
	oldPath := exePath + ".old"
	if _, err := os.Stat(oldPath); err == nil {
		_ = os.Remove(oldPath)
	}
}

func fileSHA256(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return fmt.Sprintf("sha256:%x", h.Sum(nil)), nil
}

// CheckForUpdates contacts GitHub API to check for an updated binary in release latest-dev.
func (a *App) CheckForUpdates() (UpdateInfo, error) {
	exePath, err := os.Executable()
	if err != nil {
		return UpdateInfo{Available: false}, nil
	}

	// Dev mode check: if executable is wails dev or temporary, avoid updating
	exeName := strings.ToLower(filepath.Base(exePath))
	if strings.Contains(exeName, "wails") || strings.Contains(exeName, "-dev") {
		return UpdateInfo{Available: false}, nil
	}

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequestWithContext(a.ctx, "GET", "https://api.github.com/repos/robbled17/pulsecore/releases/tags/latest-dev", nil)
	if err != nil {
		return UpdateInfo{Available: false}, err
	}
	req.Header.Set("User-Agent", "PulseCore-Updater")
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := client.Do(req)
	if err != nil {
		return UpdateInfo{Available: false}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return UpdateInfo{Available: false}, fmt.Errorf("github api returned status %d", resp.StatusCode)
	}

	var rel githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return UpdateInfo{Available: false}, err
	}

	var targetAsset *githubAsset
	for i := range rel.Assets {
		if strings.EqualFold(rel.Assets[i].Name, "pulsecore.exe") {
			targetAsset = &rel.Assets[i]
			break
		}
	}

	if targetAsset == nil || targetAsset.BrowserDownloadURL == "" {
		return UpdateInfo{Available: false}, nil
	}

	currHash, hashErr := fileSHA256(exePath)
	isDifferent := false

	if targetAsset.Digest != "" && hashErr == nil {
		isDifferent = !strings.EqualFold(currHash, targetAsset.Digest)
	} else {
		fi, statErr := os.Stat(exePath)
		if statErr == nil {
			if fi.Size() != targetAsset.Size {
				isDifferent = true
			} else if targetAsset.UpdatedAt != "" {
				if t, err := time.Parse(time.RFC3339, targetAsset.UpdatedAt); err == nil {
					if t.After(fi.ModTime().Add(time.Minute)) {
						isDifferent = true
					}
				}
			}
		}
	}

	sizeMB := float64(targetAsset.Size) / (1024 * 1024)
	sizeMB = math.Round(sizeMB*10) / 10

	if !isDifferent {
		return UpdateInfo{Available: false, SizeMB: sizeMB}, nil
	}

	notes := strings.TrimSpace(rel.Body)
	if len(notes) > 300 {
		notes = notes[:297] + "..."
	}

	return UpdateInfo{
		Available:    true,
		DownloadURL:  targetAsset.BrowserDownloadURL,
		SizeMB:       sizeMB,
		PublishedAt:  rel.PublishedAt,
		ReleaseNotes: notes,
		Digest:       targetAsset.Digest,
	}, nil
}

// DownloadAndInstallUpdate downloads the update binary with progress, verifies SHA-256,
// atomically replaces the current binary, spawns the new process, and quits.
func (a *App) DownloadAndInstallUpdate(downloadURL string, expectedDigest string) error {
	if downloadURL == "" {
		return fmt.Errorf("empty download url")
	}

	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("cannot resolve executable path: %w", err)
	}

	newPath := exePath + ".new"
	oldPath := exePath + ".old"

	_ = os.Remove(newPath)

	client := &http.Client{Timeout: 5 * time.Minute}
	req, err := http.NewRequestWithContext(a.ctx, "GET", downloadURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "PulseCore-Updater")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to start download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download server returned %d", resp.StatusCode)
	}

	outFile, err := os.OpenFile(newPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return fmt.Errorf("cannot create temp file: %w", err)
	}

	totalSize := resp.ContentLength
	var downloaded int64
	buf := make([]byte, 64*1024)
	lastEmit := time.Now()

	runtime.EventsEmit(a.ctx, "update:progress", 0)

	for {
		n, rErr := resp.Body.Read(buf)
		if n > 0 {
			if _, wErr := outFile.Write(buf[:n]); wErr != nil {
				_ = outFile.Close()
				_ = os.Remove(newPath)
				return fmt.Errorf("error writing update: %w", wErr)
			}
			downloaded += int64(n)

			if totalSize > 0 && time.Since(lastEmit) >= 150*time.Millisecond {
				pct := int(float64(downloaded) / float64(totalSize) * 100)
				if pct > 99 {
					pct = 99
				}
				runtime.EventsEmit(a.ctx, "update:progress", pct)
				lastEmit = time.Now()
			}
		}

		if rErr != nil {
			if rErr == io.EOF {
				break
			}
			_ = outFile.Close()
			_ = os.Remove(newPath)
			return fmt.Errorf("download stream interrupted: %w", rErr)
		}
	}

	_ = outFile.Close()

	// Verify SHA-256 digest if provided
	if expectedDigest != "" {
		hash, err := fileSHA256(newPath)
		if err != nil {
			_ = os.Remove(newPath)
			return fmt.Errorf("failed to verify hash: %w", err)
		}
		if !strings.EqualFold(hash, expectedDigest) {
			_ = os.Remove(newPath)
			return fmt.Errorf("integrity check failed: expected %s, got %s", expectedDigest, hash)
		}
	}

	runtime.EventsEmit(a.ctx, "update:progress", 100)

	// Atomic hot-swap on Windows:
	// 1. Remove leftover .old if present
	_ = os.Remove(oldPath)

	// 2. Rename running binary to .old
	if err := os.Rename(exePath, oldPath); err != nil {
		_ = os.Remove(newPath)
		return fmt.Errorf("failed to rename current binary: %w", err)
	}

	// 3. Rename .new to exePath
	if err := os.Rename(newPath, exePath); err != nil {
		_ = os.Rename(oldPath, exePath)
		_ = os.Remove(newPath)
		return fmt.Errorf("failed to place new binary: %w", err)
	}

	// 4. Launch new binary as detached process
	cmd := exec.Command(exePath)
	cmd.Dir = filepath.Dir(exePath)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: 0x00000200 /* CREATE_NEW_PROCESS_GROUP */ | 0x00000008 /* DETACHED_PROCESS */,
	}

	if err := cmd.Start(); err != nil {
		_ = os.Remove(exePath)
		_ = os.Rename(oldPath, exePath)
		return fmt.Errorf("failed to launch updated binary: %w", err)
	}

	runtime.EventsEmit(a.ctx, "update:restarting", true)

	go func() {
		time.Sleep(600 * time.Millisecond)
		runtime.Quit(a.ctx)
		os.Exit(0)
	}()

	return nil
}
