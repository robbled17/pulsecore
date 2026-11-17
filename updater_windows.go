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

func checkAVError(err error) error {
	if err == nil {
		return nil
	}
	errStr := strings.ToLower(err.Error())
	if strings.Contains(errStr, "virus") || strings.Contains(errStr, "potentially unwanted") || strings.Contains(errStr, "0x800700e1") || strings.Contains(errStr, " 225") || strings.Contains(errStr, ": 225") {
		return fmt.Errorf("Защитник Windows или антивирус заблокировал файл обновления (ложное срабатывание). Добавьте папку PulseCore в исключения антивируса")
	}
	return err
}

func fileSHA256(filePath string) (string, error) {
	var f *os.File
	var err error
	// Retry up to 4 times with short sleep in case AV is scanning the newly written file
	for attempt := 0; attempt < 4; attempt++ {
		f, err = os.Open(filePath)
		if err == nil {
			break
		}
		time.Sleep(250 * time.Millisecond)
	}
	if err != nil {
		return "", checkAVError(err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", checkAVError(err)
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
	if err != nil || resp == nil || resp.StatusCode != http.StatusOK {
		if resp != nil && resp.Body != nil {
			_ = resp.Body.Close()
		}
		// If GitHub API rate limits (e.g. HTTP 403 / 429) or fails,
		// seamlessly fallback to direct HEAD check of release asset on CDN
		return a.checkForUpdatesDirectFallback(exePath)
	}
	defer resp.Body.Close()

	var rel githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return a.checkForUpdatesDirectFallback(exePath)
	}

	var targetAsset *githubAsset
	for i := range rel.Assets {
		if strings.EqualFold(rel.Assets[i].Name, "pulsecore.exe") {
			targetAsset = &rel.Assets[i]
			break
		}
	}

	if targetAsset == nil || targetAsset.BrowserDownloadURL == "" {
		return a.checkForUpdatesDirectFallback(exePath)
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

// checkForUpdatesDirectFallback queries the release asset directly on GitHub CDN using HEAD request.
// This avoids GitHub REST API unauthenticated rate limits (HTTP 403 / 429).
func (a *App) checkForUpdatesDirectFallback(exePath string) (UpdateInfo, error) {
	directURL := "https://github.com/robbled17/pulsecore/releases/download/latest-dev/pulsecore.exe"
	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequestWithContext(a.ctx, "HEAD", directURL, nil)
	if err != nil {
		return UpdateInfo{Available: false}, fmt.Errorf("не удалось инициализировать проверку: %w", err)
	}
	req.Header.Set("User-Agent", "PulseCore-Updater")

	resp, err := client.Do(req)
	if err != nil {
		return UpdateInfo{Available: false}, fmt.Errorf("сервер обновлений недоступен: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return UpdateInfo{Available: false}, fmt.Errorf("сервер обновлений вернул статус %d", resp.StatusCode)
	}

	contentLength := resp.ContentLength
	if contentLength <= 0 {
		return UpdateInfo{Available: false}, nil
	}

	sizeMB := float64(contentLength) / (1024 * 1024)
	sizeMB = math.Round(sizeMB*10) / 10

	fi, statErr := os.Stat(exePath)
	isDifferent := false
	if statErr == nil {
		if fi.Size() != contentLength {
			isDifferent = true
		} else if lastModStr := resp.Header.Get("Last-Modified"); lastModStr != "" {
			if t, err := time.Parse(time.RFC1123, lastModStr); err == nil {
				if t.After(fi.ModTime().Add(time.Minute)) {
					isDifferent = true
				}
			}
		}
	}

	if !isDifferent {
		return UpdateInfo{Available: false, SizeMB: sizeMB}, nil
	}

	return UpdateInfo{
		Available:    true,
		DownloadURL:  directURL,
		SizeMB:       sizeMB,
		ReleaseNotes: "Автоматическое обновление PulseCore (latest-dev)",
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

	// Download to a temporary file in OS temp folder to avoid triggering in-place executable dropper heuristics
	tmpPath := filepath.Join(os.TempDir(), fmt.Sprintf("pulsecore-upd-%d.tmp", time.Now().UnixNano()))
	oldPath := exePath + ".old"

	_ = os.Remove(tmpPath)

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

	outFile, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return fmt.Errorf("cannot create temp file: %w", checkAVError(err))
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
				_ = os.Remove(tmpPath)
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
			_ = os.Remove(tmpPath)
			return fmt.Errorf("download stream interrupted: %w", rErr)
		}
	}

	_ = outFile.Close()

	// Verify SHA-256 digest if provided
	if expectedDigest != "" {
		hash, err := fileSHA256(tmpPath)
		if err != nil {
			_ = os.Remove(tmpPath)
			return checkAVError(err)
		}
		if !strings.EqualFold(hash, expectedDigest) {
			_ = os.Remove(tmpPath)
			return fmt.Errorf("integrity check failed: expected %s, got %s", expectedDigest, hash)
		}
	}

	runtime.EventsEmit(a.ctx, "update:progress", 100)

	// Atomic hot-swap on Windows:
	// 1. Remove leftover .old if present
	_ = os.Remove(oldPath)

	// 2. Rename running binary to .old
	if err := os.Rename(exePath, oldPath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("failed to rename current binary: %w", checkAVError(err))
	}

	// 3. Move verified tmpPath to exePath
	if err := os.Rename(tmpPath, exePath); err != nil {
		// Fallback to copy if cross-volume or rename failed
		if cpErr := copyFile(tmpPath, exePath); cpErr != nil {
			_ = os.Rename(oldPath, exePath)
			_ = os.Remove(tmpPath)
			return fmt.Errorf("failed to place new binary: %w", checkAVError(cpErr))
		}
		_ = os.Remove(tmpPath)
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
