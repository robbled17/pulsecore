package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func pulseBackupsDir() string {
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		if cfg, err := os.UserConfigDir(); err == nil {
			base = cfg
		} else {
			base = os.TempDir()
		}
	}
	dir := filepath.Join(base, "PulseCore", "backups")
	_ = os.MkdirAll(dir, 0o755)
	return dir
}

// backupRegistryChange writes a .reg-style snapshot before any registry mutation.
// Returns the backup file path (best-effort; empty on failure).
func backupRegistryChange(displayPath, content string) (string, error) {
	if IsForbiddenRegistryPath(displayPath) {
		return "", fmt.Errorf("forbidden registry path: %s", displayPath)
	}
	dir := pulseBackupsDir()
	stamp := time.Now().Format("20060102-150405")
	safe := sanitizeBackupName(displayPath)
	name := fmt.Sprintf("reg-%s-%s.reg", stamp, safe)
	path := filepath.Join(dir, name)

	body := content
	if strings.TrimSpace(body) == "" {
		body = fmt.Sprintf("Windows Registry Editor Version 5.00\r\n\r\n; PulseCore backup before change\r\n; Path: %s\r\n; Deleted/modified at %s\r\n",
			displayPath, time.Now().Format(time.RFC3339))
	}
	if !strings.HasPrefix(body, "Windows Registry Editor") {
		body = "Windows Registry Editor Version 5.00\r\n\r\n" + body
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		return "", err
	}
	return path, nil
}

func sanitizeBackupName(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '\\', r == '/', r == ' ', r == '.', r == '-':
			b.WriteByte('-')
		}
		if b.Len() > 80 {
			break
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "key"
	}
	return out
}

func registryBackupSnippet(path, valueName, valueData string) string {
	var b strings.Builder
	b.WriteString("Windows Registry Editor Version 5.00\r\n\r\n")
	b.WriteString(fmt.Sprintf("; PulseCore auto-backup\r\n"))
	key := path
	if strings.HasPrefix(strings.ToLower(key), "hkcu\\") {
		key = "HKEY_CURRENT_USER\\" + key[5:]
	} else if strings.HasPrefix(strings.ToLower(key), "hklm\\") {
		key = "HKEY_LOCAL_MACHINE\\" + key[5:]
	}
	b.WriteString(fmt.Sprintf("[%s]\r\n", key))
	if valueName != "" {
		esc := strings.ReplaceAll(valueData, `\`, `\\`)
		esc = strings.ReplaceAll(esc, `"`, `\"`)
		b.WriteString(fmt.Sprintf("\"%s\"=\"%s\"\r\n", valueName, esc))
	}
	return b.String()
}
