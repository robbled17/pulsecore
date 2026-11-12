package main

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

//go:embed all:helper/dist
var embeddedHelperFS embed.FS

const embeddedHelperMarker = "1.0.1-embed-pawnio-install"

func localHelperBinDir() (string, error) {
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		var err error
		base, err = os.UserCacheDir()
		if err != nil {
			return "", err
		}
	}
	dir := filepath.Join(base, "PulseCore", "bin")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

func helperEmbedMarkerPath(dir string) string {
	return filepath.Join(dir, ".pulsecore-helper-version")
}

// ensureEmbeddedHelper extracts the bundled helper into %LOCALAPPDATA%\PulseCore\bin
// when missing or outdated. Returns the directory containing PulseCoreHelper.exe.
func ensureEmbeddedHelper() (string, error) {
	dir, err := localHelperBinDir()
	if err != nil {
		return "", err
	}
	exe := filepath.Join(dir, "PulseCoreHelper.exe")
	marker := helperEmbedMarkerPath(dir)
	needExtract := true
	if _, err := os.Stat(exe); err == nil {
		if raw, err := os.ReadFile(marker); err == nil && strings.TrimSpace(string(raw)) == embeddedHelperMarker {
			needExtract = false
		}
	}
	if !needExtract {
		return dir, nil
	}
	if err := extractEmbeddedHelper(dir); err != nil {
		return "", err
	}
	_ = os.WriteFile(marker, []byte(embeddedHelperMarker), 0o644)
	if _, err := os.Stat(exe); err != nil {
		return "", fmt.Errorf("embedded helper extract failed: %w", err)
	}
	return dir, nil
}

func extractEmbeddedHelper(dstDir string) error {
	root := "helper/dist"
	return fs.WalkDir(embeddedHelperFS, root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		// Skip debug symbols from the portable payload.
		if strings.HasSuffix(strings.ToLower(d.Name()), ".pdb") {
			return nil
		}
		out := filepath.Join(dstDir, rel)
		if d.IsDir() {
			return os.MkdirAll(out, 0o755)
		}
		data, err := embeddedHelperFS.ReadFile(path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
			return err
		}
		tmp := out + ".tmp"
		if err := os.WriteFile(tmp, data, 0o755); err != nil {
			return err
		}
		_ = os.Remove(out)
		return os.Rename(tmp, out)
	})
}
