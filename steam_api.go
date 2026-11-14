package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// GameRequirementsData holds precise parsed system requirements from Steam Store API or calibration.
type GameRequirementsData struct {
	AppID           int     `json:"app_id"`
	Name            string  `json:"name"`
	Year            int     `json:"year"`
	MinVRAMGB       float64 `json:"min_vram_gb"`
	RecVRAMGB       float64 `json:"rec_vram_gb"`
	StorageNeedGB   float64 `json:"storage_need_gb"`
	MinStorageLabel string  `json:"min_storage_label"`
	MinVRAMLabel    string  `json:"min_vram_label"`
	IsIntegrated    bool    `json:"is_integrated"`
	Source          string  `json:"source"` // "steam_store", "cache", "calibrated"
}

var (
	gameReqsCacheMu   sync.RWMutex
	gameReqsCache     = make(map[string]GameRequirementsData)
	gameReqsCacheOnce sync.Once
)

func gameReqsCacheFilePath() string {
	local := os.Getenv("LOCALAPPDATA")
	if local == "" {
		local = os.TempDir()
	}
	dir := filepath.Join(local, "PulseCore", "cache")
	_ = os.MkdirAll(dir, 0o755)
	return filepath.Join(dir, "game_reqs.json")
}

func initGameReqsCache() {
	gameReqsCacheOnce.Do(func() {
		p := gameReqsCacheFilePath()
		data, err := os.ReadFile(p)
		if err != nil {
			return
		}
		var stored map[string]GameRequirementsData
		if err := json.Unmarshal(data, &stored); err == nil && stored != nil {
			gameReqsCache = stored
		}
	})
}

func saveGameReqsCache() {
	gameReqsCacheMu.RLock()
	data, err := json.MarshalIndent(gameReqsCache, "", "  ")
	gameReqsCacheMu.RUnlock()
	if err != nil {
		return
	}
	p := gameReqsCacheFilePath()
	_ = os.WriteFile(p, data, 0o644)
}

// GetCachedOrFetchGameReqs returns precise requirements using Steam Store API or calibration.
func GetCachedOrFetchGameReqs(appID int, gameTitle string, yearHint int, categoryHint string) GameRequirementsData {
	initGameReqsCache()

	key := ""
	if appID > 0 {
		key = fmt.Sprintf("app_%d", appID)
	} else if gameTitle != "" {
		key = strings.ToLower(normalizeGameKey(gameTitle))
	}
	if key == "" {
		return calibrateRequirementsByYear(yearHint, categoryHint, gameTitle)
	}

	gameReqsCacheMu.RLock()
	cached, ok := gameReqsCache[key]
	gameReqsCacheMu.RUnlock()
	if ok && (cached.MinVRAMGB > 0 || cached.StorageNeedGB > 0) {
		return cached
	}

	// Fetch from Steam API if appID available
	if appID > 0 {
		data, err := fetchSteamAppDetailsPrecise(appID)
		if err == nil && (data.MinVRAMGB > 0 || data.StorageNeedGB > 0) {
			data.Source = "steam_store"
			gameReqsCacheMu.Lock()
			gameReqsCache[key] = data
			if data.Name != "" {
				nameKey := strings.ToLower(normalizeGameKey(data.Name))
				gameReqsCache[nameKey] = data
			}
			gameReqsCacheMu.Unlock()
			go saveGameReqsCache()
			return data
		}
	}

	// Calibration fallback
	calibrated := calibrateRequirementsByYear(yearHint, categoryHint, gameTitle)
	if appID > 0 {
		calibrated.AppID = appID
	}
	if gameTitle != "" && calibrated.Name == "" {
		calibrated.Name = gameTitle
	}

	gameReqsCacheMu.Lock()
	gameReqsCache[key] = calibrated
	gameReqsCacheMu.Unlock()
	go saveGameReqsCache()

	return calibrated
}

// fetchSteamAppDetailsPrecise queries Steam Store API and parses pc_requirements.
func fetchSteamAppDetailsPrecise(appID int) (GameRequirementsData, error) {
	out := GameRequirementsData{AppID: appID}
	client := &http.Client{Timeout: 4 * time.Second}
	u := fmt.Sprintf("https://store.steampowered.com/api/appdetails?appids=%d&l=russian", appID)
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return out, err
	}
	req.Header.Set("User-Agent", "PulseCore/1.1")
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return out, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return out, err
	}

	var root map[string]struct {
		Success bool            `json:"success"`
		Data    json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &root); err != nil {
		return out, err
	}

	entry, ok := root[strconv.Itoa(appID)]
	if !ok || !entry.Success || len(entry.Data) == 0 {
		return out, fmt.Errorf("app %d details not available", appID)
	}

	var appData struct {
		Name            string `json:"name"`
		ReleaseDate     struct {
			Date string `json:"date"`
		} `json:"release_date"`
		PCRequirements json.RawMessage `json:"pc_requirements"`
	}
	_ = json.Unmarshal(entry.Data, &appData)
	out.Name = appData.Name

	// Year
	if m := regexp.MustCompile(`(?:19|20)\d{2}`).FindString(appData.ReleaseDate.Date); m != "" {
		out.Year, _ = strconv.Atoi(m)
	}

	// Parse pc_requirements block
	var minText, recText string
	if len(appData.PCRequirements) > 0 {
		var reqObj struct {
			Minimum     string `json:"minimum"`
			Recommended string `json:"recommended"`
		}
		if err := json.Unmarshal(appData.PCRequirements, &reqObj); err == nil {
			minText = reqObj.Minimum
			recText = reqObj.Recommended
		}
	}

	minVRAM, minVRAMLbl, minInt := parseVRAMFromHTML(minText, out.Year)
	recVRAM, _, _ := parseVRAMFromHTML(recText, out.Year)
	storageGB, storageLbl := parseStorageFromHTML(minText)
	if storageGB <= 0 {
		storageGB, storageLbl = parseStorageFromHTML(recText)
	}

	out.MinVRAMGB = minVRAM
	out.RecVRAMGB = recVRAM
	out.StorageNeedGB = storageGB
	out.MinVRAMLabel = minVRAMLbl
	out.MinStorageLabel = storageLbl
	out.IsIntegrated = minInt

	// If missing, calibrate based on year
	if out.MinVRAMGB <= 0 || out.StorageNeedGB <= 0 {
		fallback := calibrateRequirementsByYear(out.Year, "", out.Name)
		if out.MinVRAMGB <= 0 {
			out.MinVRAMGB = fallback.MinVRAMGB
			out.MinVRAMLabel = fallback.MinVRAMLabel
		}
		if out.RecVRAMGB <= 0 {
			out.RecVRAMGB = fallback.RecVRAMGB
		}
		if out.StorageNeedGB <= 0 {
			out.StorageNeedGB = fallback.StorageNeedGB
			out.MinStorageLabel = fallback.MinStorageLabel
		}
	}

	return out, nil
}

// parseStorageFromHTML extracts exact storage size from requirements HTML.
func parseStorageFromHTML(html string) (float64, string) {
	if html == "" {
		return 0, ""
	}
	text := stripHTMLTags(html)

	// Patterns:
	// "Место на диске: 50 MB"
	// "Storage: 50 MB available space"
	// "40 MB available space"
	// "150 GB available space"
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)(?:storage|место на диске|дисковое пространство|hard drive space|hard drive|disk space)[:\s]*<strong>?[:\s]*(\d+(?:[.,]\d+)?)\s*(mb|gb|мб|гб)`),
		regexp.MustCompile(`(?i)(\d+(?:[.,]\d+)?)\s*(mb|gb|мб|гб)\s*(?:available|свободного|free|space)`),
		regexp.MustCompile(`(?i)(?:storage|место на диске|диск)[:\s]*(\d+(?:[.,]\d+)?)\s*(mb|gb|мб|гб)`),
	}

	for _, re := range patterns {
		if m := re.FindStringSubmatch(text); len(m) >= 3 {
			valStr := strings.ReplaceAll(m[1], ",", ".")
			val, err := strconv.ParseFloat(valStr, 64)
			if err != nil || val <= 0 {
				continue
			}
			unit := strings.ToLower(m[2])
			if unit == "mb" || unit == "мб" {
				gb := val / 1024.0
				return gb, fmt.Sprintf("%.0f МБ", val)
			}
			return val, fmt.Sprintf("%.1f ГБ", val)
		}
	}
	return 0, ""
}

// parseVRAMFromHTML extracts video memory requirement from requirements HTML.
func parseVRAMFromHTML(html string, yearHint int) (vramGB float64, label string, isIntegrated bool) {
	if html == "" {
		return 0, "", false
	}
	text := stripHTMLTags(html)
	lower := strings.ToLower(text)

	// Look for MB/GB next to graphics / vram / video
	vramPatterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)(?:видеокарта|графический процессор|graphics|video card|video|vram|видеопамять)[:\s]*[^\n\r<]{0,60}?(\d+(?:[.,]\d+)?)\s*(mb|gb|мб|гб)`),
		regexp.MustCompile(`(?i)(\d+(?:[.,]\d+)?)\s*(mb|gb|мб|гб).{0,30}(?:vram|video ram|видеопамя|видеокар)`),
	}

	for _, re := range vramPatterns {
		if m := re.FindStringSubmatch(text); len(m) >= 3 {
			valStr := strings.ReplaceAll(m[1], ",", ".")
			val, err := strconv.ParseFloat(valStr, 64)
			if err != nil || val <= 0 {
				continue
			}
			unit := strings.ToLower(m[2])
			if unit == "mb" || unit == "мб" {
				gb := val / 1024.0
				return gb, fmt.Sprintf("%.0f МБ", val), false
			}
			return val, fmt.Sprintf("%.1f ГБ", val), false
		}
	}

	// If no explicit VRAM is stated, check for legacy APIs or integrated hints for pre-2010 games
	if strings.Contains(lower, "directx 9") || strings.Contains(lower, "directx 8") ||
		strings.Contains(lower, "directx 7") || strings.Contains(lower, "opengl") ||
		strings.Contains(lower, "3dfx") || strings.Contains(lower, "svga") || strings.Contains(lower, "vga") ||
		(yearHint > 0 && yearHint < 2010) {
		return 0.128, "128 МБ", true
	}

	if strings.Contains(lower, "integrated") || strings.Contains(lower, "встроенн") {
		return 0.128, "Встроенная", true
	}

	return 0, "", false
}

func stripHTMLTags(s string) string {
	re := regexp.MustCompile(`<[^>]*>`)
	clean := re.ReplaceAllString(s, " ")
	clean = strings.ReplaceAll(clean, "&nbsp;", " ")
	clean = strings.ReplaceAll(clean, "&quot;", "\"")
	clean = strings.ReplaceAll(clean, "&amp;", "&")
	return clean
}

// calibrateRequirementsByYear produces accurate realistic minimums by release era and genre.
func calibrateRequirementsByYear(year int, category string, name string) GameRequirementsData {
	lowerName := strings.ToLower(name)

	// Specific classic overrides
	if strings.Contains(lowerName, "doom") && (strings.Contains(lowerName, "1993") || strings.Contains(lowerName, "classic") || year == 1993) {
		return GameRequirementsData{
			Name: "DOOM (1993)", Year: 1993,
			MinVRAMGB: 0.032, RecVRAMGB: 0.064,
			StorageNeedGB: 0.05, MinStorageLabel: "50 МБ", MinVRAMLabel: "32 МБ",
			Source: "calibrated",
		}
	}
	if strings.Contains(lowerName, "quake") && !strings.Contains(lowerName, "champions") && (year <= 1999 || strings.Contains(lowerName, "1996")) {
		return GameRequirementsData{
			Name: "Quake", Year: 1996,
			MinVRAMGB: 0.032, RecVRAMGB: 0.064,
			StorageNeedGB: 0.1, MinStorageLabel: "100 МБ", MinVRAMLabel: "32 МБ",
			Source: "calibrated",
		}
	}
	if strings.Contains(lowerName, "starcraft") && !strings.Contains(lowerName, "ii") && (year <= 2000) {
		return GameRequirementsData{
			Name: "StarCraft", Year: 1998,
			MinVRAMGB: 0.032, RecVRAMGB: 0.064,
			StorageNeedGB: 0.3, MinStorageLabel: "300 МБ", MinVRAMLabel: "32 МБ",
			Source: "calibrated",
		}
	}
	if strings.Contains(lowerName, "counter-strike") && (strings.Contains(lowerName, "1.6") || year <= 2003 || lowerName == "cs16") {
		return GameRequirementsData{
			Name: "Counter-Strike 1.6", Year: 2003,
			MinVRAMGB: 0.128, RecVRAMGB: 0.256,
			StorageNeedGB: 0.8, MinStorageLabel: "800 МБ", MinVRAMLabel: "128 МБ",
			Source: "calibrated",
		}
	}
	if strings.Contains(lowerName, "buckshot") {
		return GameRequirementsData{
			Name: "Buckshot Roulette", Year: 2024,
			MinVRAMGB: 0.512, RecVRAMGB: 1.0,
			StorageNeedGB: 0.4, MinStorageLabel: "400 МБ", MinVRAMLabel: "512 МБ",
			Source: "calibrated",
		}
	}

	// 1. Pre-2000: VRAM 32-64 МБ, disk ~50-500 МБ
	if year > 0 && year <= 2000 {
		return GameRequirementsData{
			Year: year, MinVRAMGB: 0.032, RecVRAMGB: 0.064,
			StorageNeedGB: 0.15, MinStorageLabel: "150 МБ", MinVRAMLabel: "32 МБ",
			Source: "calibrated",
		}
	}

	// 2. 2001-2008: VRAM 128-256 МБ, disk ~1-4 ГБ
	if year > 2000 && year <= 2008 {
		return GameRequirementsData{
			Year: year, MinVRAMGB: 0.128, RecVRAMGB: 0.256,
			StorageNeedGB: 2.0, MinStorageLabel: "2 ГБ", MinVRAMLabel: "128 МБ",
			Source: "calibrated",
		}
	}

	// 3. 2009-2014: VRAM 512 МБ - 1 ГБ, disk ~5-15 ГБ
	if year > 2008 && year <= 2014 {
		return GameRequirementsData{
			Year: year, MinVRAMGB: 0.512, RecVRAMGB: 1.0,
			StorageNeedGB: 8.0, MinStorageLabel: "8 ГБ", MinVRAMLabel: "512 МБ",
			Source: "calibrated",
		}
	}

	// 4. Indie / 2D games
	catLower := strings.ToLower(category)
	if catLower == "indie" || catLower == "casual" || strings.Contains(lowerName, "indie") || strings.Contains(lowerName, "2d") {
		return GameRequirementsData{
			Year: year, MinVRAMGB: 0.512, RecVRAMGB: 1.0,
			StorageNeedGB: 1.5, MinStorageLabel: "1.5 ГБ", MinVRAMLabel: "512 МБ",
			Source: "calibrated",
		}
	}

	// Modern general defaults
	if year >= 2021 {
		return GameRequirementsData{
			Year: year, MinVRAMGB: 4.0, RecVRAMGB: 8.0,
			StorageNeedGB: 50.0, MinStorageLabel: "50 ГБ", MinVRAMLabel: "4 ГБ",
			Source: "calibrated",
		}
	}

	return GameRequirementsData{
		Year: year, MinVRAMGB: 2.0, RecVRAMGB: 4.0,
		StorageNeedGB: 25.0, MinStorageLabel: "25 ГБ", MinVRAMLabel: "2 ГБ",
		Source: "calibrated",
	}
}
