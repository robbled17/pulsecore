//go:build !windows

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func clearReadOnlyAttr(path string) {}

func ForceDeleteAll(path string) error {
	path = filepath.Clean(strings.TrimSpace(path))
	if path == "" {
		return fmt.Errorf("empty path")
	}
	return os.RemoveAll(path)
}

func isDeletableInstallPath(p string) bool { return isSafeAppLeftoverPath(p) }

func openPathInExplorer(folderPath string) error {
	return fmt.Errorf("unsupported")
}
