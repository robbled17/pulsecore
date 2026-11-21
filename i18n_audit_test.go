package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func loadI18nFile(t *testing.T) string {
	t.Helper()
	candidates := []string{
		filepath.Join("frontend", "src", "i18n.js"),
		filepath.Join("..", "frontend", "src", "i18n.js"),
	}
	for _, c := range candidates {
		b, err := os.ReadFile(c)
		if err == nil {
			return string(b)
		}
	}
	t.Fatal("frontend/src/i18n.js not found")
	return ""
}

func TestI18nSevenLangsComplete(t *testing.T) {
	text := loadI18nFile(t)
	langs := []string{"ru", "en", "kk", "tt", "de", "ja", "zh"}
	keys := map[string]map[string]struct{}{}
	for _, lang := range langs {
		re := regexp.MustCompile(`(?m)^\s{2}` + lang + `:\s*\{`)
		loc := re.FindStringIndex(text)
		if loc == nil {
			t.Fatalf("lang %s missing", lang)
		}
		start := loc[1]
		depth := 1
		i := start
		for i < len(text) && depth > 0 {
			switch text[i] {
			case '{':
				depth++
			case '}':
				depth--
			}
			i++
		}
		block := text[start : i-1]
		set := map[string]struct{}{}
		prop := regexp.MustCompile(`(?m)(?:^|[,\{])\s*([A-Za-z_][A-Za-z0-9_]*)\s*:`)
		for _, m := range prop.FindAllStringSubmatch(block, -1) {
			set[m[1]] = struct{}{}
		}
		keys[lang] = set
	}
	ru := keys["ru"]
	if len(ru) < 200 {
		t.Fatalf("ru key count too low: %d", len(ru))
	}
	for _, lang := range langs {
		var missing []string
		for k := range ru {
			if _, ok := keys[lang][k]; !ok {
				missing = append(missing, k)
			}
		}
		if len(missing) > 0 {
			t.Fatalf("%s missing %d keys vs ru, e.g. %v", lang, len(missing), missing[:min(8, len(missing))])
		}
	}
}

func TestI18nNoEmEnDashes(t *testing.T) {
	text := loadI18nFile(t)
	if strings.Contains(text, "\u2014") || strings.Contains(text, "\u2013") {
		t.Fatal("i18n.js must not contain em/en dashes (U+2014 / U+2013)")
	}
	// Also scan frontend user-visible JS/HTML lightly
	roots := []string{filepath.Join("frontend", "src"), filepath.Join("frontend", "index.html")}
	for _, root := range roots {
		_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil || info == nil || info.IsDir() {
				return nil
			}
			if !strings.HasSuffix(path, ".js") && !strings.HasSuffix(path, ".html") && !strings.HasSuffix(path, ".css") {
				return nil
			}
			if strings.Contains(path, "node_modules") || strings.Contains(path, "dist") {
				return nil
			}
			b, err := os.ReadFile(path)
			if err != nil {
				return nil
			}
			s := string(b)
			if strings.Contains(s, "\u2014") || strings.Contains(s, "\u2013") {
				t.Errorf("em/en dash in %s", path)
			}
			return nil
		})
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
