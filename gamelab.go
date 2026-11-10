package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

//go:embed embedded_games.json
var embeddedGamesJSON []byte

// GameInfo describes a title in the FPS Predictor catalog.
type GameInfo struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	Genre      string  `json:"genre"`
	Weight     float64 `json:"weight"`
	SteamAppID int     `json:"steamAppId"`
	PosterURL  string  `json:"posterUrl"`
	HeaderURL  string  `json:"headerUrl,omitempty"`
	Category   string  `json:"category"`
	Year       int     `json:"year,omitempty"`
	Badge      string  `json:"badge,omitempty"`
	Engine     string  `json:"engine,omitempty"`
	Synthetic  bool    `json:"synthetic,omitempty"`
	FromSteam  bool    `json:"fromSteam,omitempty"`
}

// FPSPrediction is estimated average FPS across quality presets.
type FPSPrediction struct {
	GameID       string   `json:"gameId"`
	GameName     string   `json:"gameName"`
	PosterURL    string   `json:"posterUrl"`
	HeaderURL    string   `json:"headerUrl"`
	Genre        string   `json:"genre"`
	Badge        string   `json:"badge"`
	Engine       string   `json:"engine"`
	Year         int      `json:"year"`
	CPUScore     float64  `json:"cpuScore"`
	GPUScore     float64  `json:"gpuScore"`
	DualChannel  bool     `json:"dualChannel"`
	IsIntegrated bool     `json:"isIntegrated"`
	VRAMGB       float64  `json:"vramGB"`
	RAMGB        float64  `json:"ramGB"`
	CPUCores     int      `json:"cpuCores"`
	LowAvg       int      `json:"lowAvg"`
	MedAvg       int      `json:"medAvg"`
	HighAvg      int      `json:"highAvg"`
	UltraAvg     int      `json:"ultraAvg"`
	LowRange     string   `json:"lowRange"`
	MedRange     string   `json:"medRange"`
	HighRange    string   `json:"highRange"`
	UltraRange   string   `json:"ultraRange"`
	Recommend    string   `json:"recommend"`
	Note         string   `json:"note"`
	Warnings     []string `json:"warnings"`
	Category     string   `json:"category"`
	Verdict      string   `json:"verdict"`
	VerdictLevel string   `json:"verdictLevel"`
}

// SystemPowerProfile is the live hardware scorecard used by GamePulse.
type SystemPowerProfile struct {
	GPUName        string  `json:"gpuName"`
	IsIntegrated   bool    `json:"isIntegrated"`
	VRAMGB         float64 `json:"vramGB"`
	GPUPowerScore  float64 `json:"gpuPowerScore"`
	CPUName        string  `json:"cpuName"`
	CPUCores       int     `json:"cpuCores"`
	CPUThreads     int     `json:"cpuThreads"`
	CPUFreqMHz     float64 `json:"cpuFreqMHz"`
	CPUPowerScore  float64 `json:"cpuPowerScore"`
	RAMGB          float64 `json:"ramGB"`
	IsDualChannel  bool    `json:"isDualChannel"`
}

type qualityPreset string

const (
	presetLow   qualityPreset = "low"
	presetMed   qualityPreset = "med"
	presetHigh  qualityPreset = "high"
	presetUltra qualityPreset = "ultra"
)

// gameReq defines mathematical requirements for one quality preset at 1080p.
type gameReq struct {
	BaseFPS     float64
	ReqGPUScore float64
	ReqCPUScore float64
	ReqVRAMGB   float64
}

type gameReqSet struct {
	Low, Med, High, Ultra gameReq
	CapFPS                float64 // 0 = no cap; e.g. 60 for lightweight titles
	Heavy                 bool
}

var gameCatalog []GameInfo
var steamCache = map[string]GameInfo{}

// Per-game requirement tables — BaseFPS is expected FPS when GPUPowerScore == ReqGPUScore.
var gameRequirements = map[string]gameReqSet{
	"cyberpunk": {
		Low:   gameReq{55, 55, 38, 3},
		Med:   gameReq{55, 70, 48, 6},
		High:  gameReq{60, 68, 58, 8},
		Ultra: gameReq{58, 78, 70, 10},
		Heavy: true,
	},
	"rdr2": {
		Low:   gameReq{50, 48, 36, 3},
		Med:   gameReq{50, 62, 46, 5},
		High:  gameReq{55, 72, 55, 8},
		Ultra: gameReq{52, 85, 65, 10},
		Heavy: true,
	},
	"gtav": {
		Low:   gameReq{60, 28, 30, 1},
		Med:   gameReq{58, 38, 36, 2},
		High:  gameReq{55, 48, 42, 3},
		Ultra: gameReq{50, 58, 48, 4},
	},
	"cs2": {
		Low:   gameReq{90, 30, 40, 1},
		Med:   gameReq{80, 40, 48, 2},
		High:  gameReq{70, 52, 55, 3},
		Ultra: gameReq{60, 62, 60, 4},
	},
	"dota2": {
		Low:   gameReq{100, 22, 32, 1},
		Med:   gameReq{90, 32, 40, 1.5},
		High:  gameReq{75, 42, 48, 2},
		Ultra: gameReq{65, 52, 55, 3},
	},
	"tanksblitz": {
		Low:   gameReq{60, 18, 25, 0.5},
		Med:   gameReq{60, 22, 28, 0.5},
		High:  gameReq{60, 26, 32, 1},
		Ultra: gameReq{60, 30, 35, 1},
		CapFPS: 60,
	},
	"fortnite": {
		Low:   gameReq{70, 32, 36, 2}, // Performance mode baseline
		Med:   gameReq{55, 48, 45, 4},
		High:  gameReq{50, 60, 52, 6},
		Ultra: gameReq{45, 72, 58, 8},
	},
	"valorant": {
		Low:   gameReq{100, 24, 35, 1},
		Med:   gameReq{90, 34, 42, 1.5},
		High:  gameReq{80, 44, 50, 2},
		Ultra: gameReq{70, 54, 55, 3},
	},
	"minecraft": {
		Low:   gameReq{90, 18, 28, 0.5},
		Med:   gameReq{80, 28, 35, 1},
		High:  gameReq{70, 38, 42, 2},
		Ultra: gameReq{55, 50, 50, 3},
	},
	"genshin": {
		Low:   gameReq{50, 40, 35, 2},
		Med:   gameReq{48, 52, 42, 3},
		High:  gameReq{45, 62, 50, 4},
		Ultra: gameReq{40, 72, 58, 6},
		Heavy: true,
	},
	"roblox": {
		Low:   gameReq{80, 16, 24, 0.5},
		Med:   gameReq{70, 24, 30, 1},
		High:  gameReq{60, 32, 36, 1.5},
		Ultra: gameReq{50, 42, 42, 2},
	},
	"lol": {
		Low:   gameReq{100, 20, 30, 1},
		Med:   gameReq{90, 30, 38, 1},
		High:  gameReq{80, 40, 45, 2},
		Ultra: gameReq{70, 50, 52, 2},
	},
	"wot": {
		Low:   gameReq{60, 32, 34, 1.5},
		Med:   gameReq{55, 42, 42, 2},
		High:  gameReq{50, 52, 48, 3},
		Ultra: gameReq{45, 62, 55, 4},
	},
	"rust": {
		Low:   gameReq{50, 45, 42, 3},
		Med:   gameReq{48, 58, 50, 5},
		High:  gameReq{45, 70, 58, 6},
		Ultra: gameReq{40, 82, 65, 8},
		Heavy: true,
	},
	"forza5": {
		Low:   gameReq{50, 42, 38, 3},
		Med:   gameReq{48, 55, 46, 5},
		High:  gameReq{50, 68, 55, 6},
		Ultra: gameReq{48, 80, 62, 8},
		Heavy: true,
	},
	"godot": {
		Low:   gameReq{120, 12, 20, 0.5},
		Med:   gameReq{100, 18, 25, 0.5},
		High:  gameReq{90, 24, 30, 1},
		Ultra: gameReq{80, 32, 35, 1},
	},
}

func init() {
	_ = json.Unmarshal(embeddedGamesJSON, &gameCatalog)
	for i := range gameCatalog {
		enrichGamePosters(&gameCatalog[i])
		if strings.TrimSpace(gameCatalog[i].Badge) == "" {
			gameCatalog[i].Badge = badgeForCategory(gameCatalog[i].Category, gameCatalog[i].Year)
		}
	}
	if len(gameCatalog) == 0 {
		gameCatalog = []GameInfo{{ID: "cs2", Name: "Counter-Strike 2", Genre: "FPS", Weight: 0.72, SteamAppID: 730, Category: "shooter", PosterURL: steamLibraryPoster(730)}}
	}
}

func enrichGamePosters(g *GameInfo) {
	if g.SteamAppID <= 0 {
		return
	}
	if strings.TrimSpace(g.PosterURL) == "" {
		g.PosterURL = steamLibraryPoster(g.SteamAppID)
	}
	if strings.TrimSpace(g.HeaderURL) == "" {
		g.HeaderURL = steamHeaderPoster(g.SteamAppID)
	}
}

func steamLibraryPoster(appID int) string {
	if appID <= 0 {
		return ""
	}
	return fmt.Sprintf("https://shared.cloudflare.steamstatic.com/store_item_assets/steam/apps/%d/library_600x900.jpg", appID)
}

func steamHeaderPoster(appID int) string {
	if appID <= 0 {
		return ""
	}
	return fmt.Sprintf("https://shared.cloudflare.steamstatic.com/store_item_assets/steam/apps/%d/header.jpg", appID)
}

func steamPoster(appID int) string { return steamLibraryPoster(appID) }

func badgeForCategory(cat string, year int) string {
	switch strings.ToLower(cat) {
	case "classic":
		if year > 0 && year < 2005 {
			return "2D Classic / Retro"
		}
		return "Classic / Low-spec"
	case "indie":
		return "Indie"
	case "shooter", "moba":
		return "eSports"
	case "aaa":
		if year >= 2020 {
			return "AAA Next-Gen"
		}
		return "AAA"
	case "sports", "sim":
		return "Sim"
	default:
		return "Indie"
	}
}

// ListGames returns the GamePulse catalog.
func (a *App) ListGames() []GameInfo {
	out := make([]GameInfo, len(gameCatalog))
	copy(out, gameCatalog)
	return out
}

// GameMeta is an alias used by the online Steam search API surface.
type GameMeta = GameInfo

// SearchGames filters the local embedded catalog only (fast).
func (a *App) SearchGames(query string) []GameInfo {
	raw := strings.TrimSpace(query)
	q := strings.ToLower(raw)
	if q == "" {
		return a.ListGames()
	}
	aliases := expandSearchAliases(q)
	out := make([]GameInfo, 0)
	for _, g := range gameCatalog {
		name := strings.ToLower(g.Name)
		id := strings.ToLower(g.ID)
		genre := strings.ToLower(g.Genre)
		hit := strings.Contains(name, q) || strings.Contains(id, q) || strings.Contains(genre, q)
		if !hit {
			for _, al := range aliases {
				if strings.Contains(name, al) || strings.Contains(id, al) {
					hit = true
					break
				}
			}
		}
		if hit {
			out = append(out, g)
		}
	}
	return out
}

// SearchGamesOnline queries the full Steam store catalog.
func (a *App) SearchGamesOnline(query string) []GameMeta {
	raw := strings.TrimSpace(query)
	if raw == "" {
		return nil
	}
	aliases := expandSearchAliases(strings.ToLower(raw))
	hits := searchSteamStore(raw)
	if len(hits) == 0 && len(aliases) > 0 {
		hits = searchSteamStore(aliases[0])
	}
	if len(hits) == 0 {
		// Legacy community endpoint as fallback.
		hits = searchSteamApps(raw)
	}
	for _, g := range hits {
		steamCache[strings.ToLower(g.ID)] = g
		if g.SteamAppID > 0 {
			steamCache[fmt.Sprintf("steam:%d", g.SteamAppID)] = g
		}
	}
	return hits
}

func expandSearchAliases(q string) []string {
	table := map[string][]string{
		"как достать соседа": {"neighbours from hell", "neighbors from hell"},
		"соседей":            {"neighbours from hell"},
		"герои 3":            {"heroes of might and magic iii", "homm3"},
		"герои iii":          {"heroes of might and magic iii"},
		"герои 5":            {"heroes of might and magic v"},
		"ведьмак 3":          {"witcher 3"},
		"ведьмак":            {"witcher"},
		"кдло":               {"counter-strike", "cs2"},
		"кс":                 {"counter-strike"},
		"кс го":              {"counter-strike"},
		"дота":               {"dota 2"},
		"танки":              {"world of tanks"},
		"танки блиц":         {"world of tanks blitz"},
		"майкрафт":           {"minecraft"},
		"гта":                {"grand theft auto"},
		"гта 5":              {"gta v", "grand theft auto v"},
		"гта сан":            {"san andreas"},
		"халф лайф":          {"half-life"},
		"халф-лайф":          {"half-life"},
		"флиаут":             {"flatout"},
		"флэтаут":            {"flatout"},
		"нфс":                {"need for speed"},
		"nfs underground":    {"need for speed underground"},
	}
	if v, ok := table[q]; ok {
		return v
	}
	for k, v := range table {
		if strings.Contains(q, k) {
			return v
		}
	}
	return nil
}

type steamSearchHit struct {
	AppID string `json:"appid"`
	Name  string `json:"name"`
	Logo  string `json:"logo"`
	Icon  string `json:"icon"`
}

type steamStoreSearchResp struct {
	Items []struct {
		ID   int    `json:"id"`
		Type string `json:"type"`
		Name string `json:"name"`
		Tiny string `json:"tiny_image"`
	} `json:"items"`
	Total int `json:"total"`
}

func searchSteamStore(query string) []GameInfo {
	q := strings.TrimSpace(query)
	if q == "" {
		return nil
	}
	client := &http.Client{Timeout: 5 * time.Second}
	endpoint := fmt.Sprintf(
		"https://store.steampowered.com/api/storesearch/?term=%s&l=russian&cc=RU",
		url.QueryEscape(q),
	)
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("User-Agent", "PulseCore/1.0")
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil || len(body) == 0 {
		return nil
	}
	var parsed steamStoreSearchResp
	if err := json.Unmarshal(body, &parsed); err != nil || len(parsed.Items) == 0 {
		return nil
	}
	out := make([]GameInfo, 0, minInt(12, len(parsed.Items)))
	for i, item := range parsed.Items {
		if i >= 12 {
			break
		}
		if item.ID <= 0 || strings.TrimSpace(item.Name) == "" {
			continue
		}
		// Prefer apps over bundles/DLC noise when type is present.
		if item.Type != "" && !strings.EqualFold(item.Type, "app") && !strings.EqualFold(item.Type, "game") {
			continue
		}
		g := gameFromSteamHit(item.Name, item.ID, false)
		out = append(out, g)
	}
	return out
}

func searchSteamApps(query string) []GameInfo {
	q := strings.TrimSpace(query)
	if q == "" {
		return nil
	}
	client := &http.Client{Timeout: 4 * time.Second}
	endpoint := "https://steamcommunity.com/actions/SearchApps/" + url.PathEscape(q)
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("User-Agent", "PulseCore/1.0")
	resp, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil || len(body) == 0 {
		return nil
	}
	var hits []steamSearchHit
	if err := json.Unmarshal(body, &hits); err != nil || len(hits) == 0 {
		return nil
	}
	out := make([]GameInfo, 0, minInt(8, len(hits)))
	for i, h := range hits {
		if i >= 8 {
			break
		}
		appID, _ := strconv.Atoi(strings.TrimSpace(h.AppID))
		if appID <= 0 || strings.TrimSpace(h.Name) == "" {
			continue
		}
		g := gameFromSteamHit(h.Name, appID, false)
		out = append(out, g)
	}
	return out
}

func gameFromSteamHit(name string, appID int, enrich bool) GameInfo {
	cat, year, badge, genre, weight := classifyTitle(name, 0)
	if enrich {
		details := fetchSteamAppDetails(appID)
		if details.Year > 0 {
			year = details.Year
		}
		if details.MinVRAMGB > 0 && details.MinVRAMGB <= 1.0 {
			cat, badge, weight = "classic", "Classic / Low-spec", 0.30
		}
		if details.Name != "" {
			name = details.Name
		}
	}
	if year > 0 && year <= 2012 {
		cat, badge, weight = "classic", "Classic / Low-spec", 0.28
		if year < 2005 {
			badge = "2D Classic / Retro"
			genre = "2D Classic / Retro"
		}
	}
	g := GameInfo{
		ID:         fmt.Sprintf("steam:%d", appID),
		Name:       name,
		Genre:      genre,
		Weight:     weight,
		SteamAppID: appID,
		Category:   cat,
		Year:       year,
		Badge:      badge,
		Engine:     "Steam",
		PosterURL:  steamLibraryPoster(appID),
		HeaderURL:  steamHeaderPoster(appID),
		FromSteam:  true,
	}
	return g
}

type steamAppDetails struct {
	Name      string
	Year      int
	MinVRAMGB float64
}

func fetchSteamAppDetails(appID int) steamAppDetails {
	out := steamAppDetails{}
	client := &http.Client{Timeout: 4 * time.Second}
	u := fmt.Sprintf("https://store.steampowered.com/api/appdetails?appids=%d&l=russian", appID)
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return out
	}
	req.Header.Set("User-Agent", "PulseCore/1.0")
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return out
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 3<<20))
	if err != nil {
		return out
	}
	text := string(body)
	lower := strings.ToLower(text)

	// Name
	if m := regexp.MustCompile(`"name"\s*:\s*"([^"\\]*(?:\\.[^"\\]*)*)"`).FindStringSubmatch(text); len(m) == 2 {
		out.Name = strings.ReplaceAll(m[1], `\"`, `"`)
	}
	// Release year: "date":"16 Nov, 2009" / "16 ноября 2009"
	if m := regexp.MustCompile(`"date"\s*:\s*"[^"]*?((?:19|20)\d{2})`).FindStringSubmatch(text); len(m) == 2 {
		out.Year, _ = strconv.Atoi(m[1])
	}
	// Min VRAM from pc_requirements HTML
	re := regexp.MustCompile(`(?i)(\d+)\s*(mb|gb).{0,24}(video|vram|graphics|видео|видеопамя)`)
	if m := re.FindStringSubmatch(lower); len(m) >= 3 {
		n, _ := strconv.Atoi(m[1])
		if strings.EqualFold(m[2], "mb") {
			out.MinVRAMGB = float64(n) / 1024.0
		} else {
			out.MinVRAMGB = float64(n)
		}
	} else {
		re2 := regexp.MustCompile(`(?i)(video|graphics|vram|видео|видеопамя).{0,32}(\d+)\s*(mb|gb)`)
		if m := re2.FindStringSubmatch(lower); len(m) >= 4 {
			n, _ := strconv.Atoi(m[2])
			if strings.EqualFold(m[3], "mb") {
				out.MinVRAMGB = float64(n) / 1024.0
			} else {
				out.MinVRAMGB = float64(n)
			}
		}
	}
	return out
}

func fetchSteamMinVRAMGB(appID int) float64 {
	return fetchSteamAppDetails(appID).MinVRAMGB
}

func classifyTitle(name string, yearHint int) (cat string, year int, badge, genre string, weight float64) {
	q := strings.ToLower(name)
	year = yearHint

	classicHints := []string{
		"half-life", "half life", "neighbours from hell", "neighbors from hell",
		"heroes of might", "flatout", "underground", "san andreas", "vice city",
		"diablo", "starcraft", "warcraft", "age of empires", "stronghold",
		"gothic", "thief", "system shock", "deus ex", "quake", "doom 3",
		"counter-strike 1", "1.6", "postal 2", "max payne", "mafia:", "bully",
		"prototype", "oblivion", "morrowind", "fallout 3", "new vegas",
		"left 4 dead", "team fortress", "portal", "terraria", "gmod",
		"как достать соседа", "герои 3", "герои iii",
	}
	indieHints := []string{"indie", "pixel", "2d", "roguelike", "metroidvania", "hollow", "celeste", "hades", "balatro"}
	shooterHints := []string{"counter-strike", "valorant", "call of duty", "battlefield", "cs2", "fps"}
	mobaHints := []string{"dota", "league of legends", "moba"}

	isClassic := year > 0 && year <= 2012
	for _, h := range classicHints {
		if strings.Contains(q, h) {
			isClassic = true
			break
		}
	}
	if strings.Contains(q, "2d") || strings.Contains(q, "pixel") || strings.Contains(q, "retro") {
		isClassic = true
	}

	switch {
	case isClassic:
		cat, weight, genre, badge = "classic", 0.28, "Classic / Low-spec", "Classic / Low-spec"
		if year == 0 {
			year = 2005
		}
		if year < 2005 {
			badge = "2D Classic / Retro"
			genre = "2D Classic / Retro"
		}
	case containsAny(q, indieHints):
		cat, weight, genre, badge = "indie", 0.42, "Indie", "Indie"
	case containsAny(q, shooterHints):
		cat, weight, genre, badge = "shooter", 0.78, "Shooter", "eSports"
	case containsAny(q, mobaHints):
		cat, weight, genre, badge = "moba", 0.58, "MOBA", "eSports"
	default:
		// Unknown modern titles — mid AAA, not automatic "heavy unplayable".
		cat, weight, genre, badge = "aaa", 0.95, "Action", "AAA"
		if year >= 2020 {
			badge = "AAA Next-Gen"
		}
	}
	return
}

func containsAny(q string, hints []string) bool {
	for _, h := range hints {
		if strings.Contains(q, h) {
			return true
		}
	}
	return false
}

func synthesizeGame(query string) GameInfo {
	name := strings.TrimSpace(query)
	if name == "" {
		name = "Custom Title"
	}
	cat, year, badge, genre, weight := classifyTitle(name, 0)
	id := "custom:" + strings.ReplaceAll(strings.ToLower(name), " ", "_")
	return GameInfo{
		ID:        id,
		Name:      name,
		Genre:     genre,
		Weight:    weight,
		Category:  cat,
		Year:      year,
		Badge:     badge,
		Engine:    "Unknown",
		Synthetic: true,
	}
}

func resolveGame(gameID string) GameInfo {
	id := strings.TrimSpace(gameID)
	if cached, ok := steamCache[strings.ToLower(id)]; ok {
		if cached.FromSteam && cached.SteamAppID > 0 && cached.Year == 0 {
			enriched := gameFromSteamHit(cached.Name, cached.SteamAppID, true)
			steamCache[strings.ToLower(enriched.ID)] = enriched
			return enriched
		}
		return cached
	}
	for _, g := range gameCatalog {
		if strings.EqualFold(g.ID, id) || strings.EqualFold(g.Name, id) {
			return g
		}
	}
	if strings.HasPrefix(strings.ToLower(id), "steam:") {
		appStr := strings.TrimPrefix(strings.ToLower(id), "steam:")
		if appID, err := strconv.Atoi(appStr); err == nil && appID > 0 {
			g := gameFromSteamHit("Steam App "+appStr, appID, true)
			steamCache[strings.ToLower(g.ID)] = g
			return g
		}
	}
	if strings.HasPrefix(strings.ToLower(id), "custom:") {
		return synthesizeGame(strings.TrimPrefix(id, "custom:"))
	}
	return synthesizeGame(id)
}

func requirementsFor(game GameInfo) gameReqSet {
	if req, ok := gameRequirements[strings.ToLower(game.ID)]; ok {
		return req
	}
	// Steam IDs mapped via category / year.
	if game.Year > 0 && game.Year <= 2012 {
		game.Category = "classic"
	}
	switch strings.ToLower(game.Category) {
	case "classic":
		return gameReqSet{
			Low:    gameReq{120, 5, 5, 0.25},
			Med:    gameReq{110, 5, 5, 0.25},
			High:   gameReq{100, 6, 6, 0.5},
			Ultra:  gameReq{90, 8, 8, 0.5},
			CapFPS: 240,
			Heavy:  false,
		}
	case "indie":
		return gameReqSet{
			Low: gameReq{100, 14, 22, 0.5}, Med: gameReq{90, 20, 28, 1},
			High: gameReq{80, 28, 34, 1.5}, Ultra: gameReq{70, 36, 40, 2},
		}
	case "shooter", "moba":
		return gameReqSet{
			Low: gameReq{80, 28, 34, 1}, Med: gameReq{70, 38, 42, 2},
			High: gameReq{60, 50, 50, 3}, Ultra: gameReq{55, 60, 55, 4},
		}
	case "sports", "sim":
		return gameReqSet{
			Low: gameReq{70, 30, 32, 1}, Med: gameReq{65, 42, 40, 2},
			High: gameReq{60, 55, 48, 4}, Ultra: gameReq{55, 68, 55, 6},
		}
	default:
		return gameReqSet{
			Low: gameReq{50, 48, 40, 3}, Med: gameReq{50, 62, 50, 5},
			High: gameReq{55, 74, 58, 8}, Ultra: gameReq{50, 88, 68, 10},
			Heavy: true,
		}
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// buildSystemPowerProfile scores the live machine for GamePulse.
func (a *App) buildSystemPowerProfile() SystemPowerProfile {
	a.mu.RLock()
	hw := a.metrics.Hardware
	cpuName := a.cpuName
	gpuName := a.gpuName
	a.mu.RUnlock()

	if cpuName == "" {
		cpuName = hw.CPU.Name
	}
	if gpuName == "" {
		gpuName = hw.GPU.Name
	}

	cores := hw.CPU.PhysicalCores
	threads := hw.CPU.LogicalCores
	if cores <= 0 {
		cores = threads
	}
	freq := hw.CPU.FrequencyMHz
	ramGB := float64(hw.RAM.TotalMB) / 1024.0
	dual := !hw.RAM.IsSingleChannel && (hw.RAM.SlotsUsed >= 2 || hw.RAM.SlotsUsed == 0)
	if hw.RAM.SlotsUsed == 1 {
		dual = false
	}
	integrated := hw.GPU.IsIntegrated
	if !integrated {
		integrated = detectIntegratedGPU(gpuName, hw.GPU.DedicatedMB)
	}
	vramGB := hw.GPU.VRAMGB
	if integrated {
		vramGB = 0
	} else if vramGB <= 0 && hw.GPU.DedicatedMB > 0 {
		vramGB = hw.GPU.DedicatedMB / 1024.0
	}

	gpuScore := calcGPUPowerScore(gpuName, integrated, vramGB)
	cpuScore := calcCPUPowerScore(cpuName, cores, threads, freq)

	return SystemPowerProfile{
		GPUName:       gpuName,
		IsIntegrated:  integrated,
		VRAMGB:        vramGB,
		GPUPowerScore: gpuScore,
		CPUName:       cpuName,
		CPUCores:      cores,
		CPUThreads:    threads,
		CPUFreqMHz:    freq,
		CPUPowerScore: cpuScore,
		RAMGB:         ramGB,
		IsDualChannel: dual,
	}
}

// GetSystemPowerProfile exposes live GamePulse hardware scoring to the UI.
func (a *App) GetSystemPowerProfile() SystemPowerProfile {
	return a.buildSystemPowerProfile()
}

// PredictGameFPS estimates FPS for any PC using dynamic power scores + requirement math.
func (a *App) PredictGameFPS(gameID string) FPSPrediction {
	game := resolveGame(gameID)
	sys := a.buildSystemPowerProfile()
	reqSet := requirementsFor(game)

	warnings := make([]string, 0, 4)
	note := ""

	lowAvg, lowRange, lowWarn := estimatePreset(sys, reqSet.Low, presetLow, reqSet.CapFPS)
	medAvg, medRange, medWarn := estimatePreset(sys, reqSet.Med, presetMed, reqSet.CapFPS)
	highAvg, highRange, highWarn := estimatePreset(sys, reqSet.High, presetHigh, reqSet.CapFPS)
	ultraAvg, ultraRange, ultraWarn := estimatePreset(sys, reqSet.Ultra, presetUltra, reqSet.CapFPS)

	for _, w := range []string{lowWarn, medWarn, highWarn, ultraWarn} {
		if w == "" {
			continue
		}
		dup := false
		for _, e := range warnings {
			if e == w {
				dup = true
				break
			}
		}
		if !dup {
			warnings = append(warnings, w)
		}
	}

	classic := strings.EqualFold(game.Category, "classic") || (!reqSet.Heavy && reqSet.Low.ReqGPUScore <= 8)
	if !classic {
		if sys.IsIntegrated && !sys.IsDualChannel {
			note = "integrated_single_channel"
			warnings = appendUnique(warnings, "fpsWarnIntegratedSingle")
		} else if !sys.IsDualChannel {
			note = "single_channel"
			warnings = appendUnique(warnings, "fpsWarnSingleChannel")
		}
		if sys.CPUCores > 0 && sys.CPUCores < 4 {
			warnings = appendUnique(warnings, "fpsWarnFewCores")
		}
	} else {
		// Classic / low-spec: drop scary bottleneck warnings.
		filtered := warnings[:0]
		for _, w := range warnings {
			if w == "fpsWarnVRAM" || w == "fpsWarnFewCores" || w == "fpsWarnIntegratedSingle" || w == "fpsWarnSingleChannel" {
				continue
			}
			filtered = append(filtered, w)
		}
		warnings = filtered
	}

	rec := "med"
	switch {
	case ultraAvg >= 55:
		rec = "ultra"
	case highAvg >= 55:
		rec = "high"
	case medAvg >= 55:
		rec = "med"
	default:
		rec = "low"
	}

	verdict, level := dynamicVerdict(sys, reqSet, lowAvg, medAvg, highAvg)
	if classic && lowAvg >= 60 {
		verdict, level = "fpsVerdictSmoothMax", "ok"
	}

	badge := game.Badge
	if badge == "" {
		badge = badgeForCategory(game.Category, game.Year)
	}
	poster := game.PosterURL
	if poster == "" {
		poster = game.HeaderURL
	}

	return FPSPrediction{
		GameID:       game.ID,
		GameName:     game.Name,
		PosterURL:    poster,
		HeaderURL:    game.HeaderURL,
		Genre:        game.Genre,
		Badge:        badge,
		Engine:       game.Engine,
		Year:         game.Year,
		CPUScore:     sys.CPUPowerScore,
		GPUScore:     sys.GPUPowerScore,
		DualChannel:  sys.IsDualChannel,
		IsIntegrated: sys.IsIntegrated,
		VRAMGB:       sys.VRAMGB,
		RAMGB:        sys.RAMGB,
		CPUCores:     sys.CPUCores,
		LowAvg:       lowAvg,
		MedAvg:       medAvg,
		HighAvg:      highAvg,
		UltraAvg:     ultraAvg,
		LowRange:     lowRange,
		MedRange:     medRange,
		HighRange:    highRange,
		UltraRange:   ultraRange,
		Recommend:    rec,
		Note:         note,
		Warnings:     warnings,
		Category:     game.Category,
		Verdict:      verdict,
		VerdictLevel: level,
	}
}

func appendUnique(list []string, item string) []string {
	for _, e := range list {
		if e == item {
			return list
		}
	}
	return append(list, item)
}

func estimatePreset(sys SystemPowerProfile, req gameReq, preset qualityPreset, cap float64) (avg int, rangeStr string, warn string) {
	if req.BaseFPS <= 0 || req.ReqGPUScore <= 0 {
		return 3, "3-5", ""
	}

	gpuRatio := sys.GPUPowerScore / req.ReqGPUScore
	if gpuRatio > 2.5 {
		gpuRatio = 2.5
	}
	if gpuRatio < 0.05 {
		gpuRatio = 0.05
	}

	cpuRatio := 1.0
	if req.ReqCPUScore > 0 {
		cpuRatio = sys.CPUPowerScore / req.ReqCPUScore
		if cpuRatio > 2.0 {
			cpuRatio = 2.0
		}
		if cpuRatio < 0.15 {
			cpuRatio = 0.15
		}
	}
	// Soft CPU blend: GPU dominates, CPU can still bottleneck.
	cpuBlend := 0.70 + 0.30*math.Min(cpuRatio, 1.25)

	ramMul := 1.0
	if sys.IsIntegrated && !sys.IsDualChannel {
		ramMul = 0.68
	} else if !sys.IsDualChannel {
		ramMul = 0.88
	} else if sys.IsIntegrated {
		ramMul = 0.92
	}

	vramMul := 1.0
	warn = ""
	effectiveVRAM := sys.VRAMGB
	if sys.IsIntegrated {
		// iGPU borrows system RAM; only punish heavier presets.
		if preset == presetHigh || preset == presetUltra {
			if sys.RAMGB < req.ReqVRAMGB*2 {
				vramMul = 0.70
				warn = "fpsWarnVRAM"
			}
		}
	} else if req.ReqVRAMGB > 0 && effectiveVRAM+0.05 < req.ReqVRAMGB {
		vramMul = 0.70
		warn = "fpsWarnVRAM"
	}

	fps := req.BaseFPS * math.Min(gpuRatio, 2.5) * cpuBlend * ramMul * vramMul

	// <4 physical cores: High/Ultra CPU wall.
	if sys.CPUCores > 0 && sys.CPUCores < 4 && (preset == presetHigh || preset == presetUltra) {
		fps *= 0.62
		if warn == "" {
			warn = "fpsWarnFewCores"
		}
	}

	// Low system RAM soft penalty for heavy presets.
	if sys.RAMGB > 0 && sys.RAMGB < 8 && (preset == presetHigh || preset == presetUltra) {
		fps *= 0.85
	}

	if cap > 0 && fps > cap {
		fps = cap
	}

	avgF := clampFPSFloat(fps)
	spread := math.Max(2, avgF*0.12)
	minV := clampFPS(int(math.Round(avgF - spread)))
	maxV := clampFPS(int(math.Round(avgF + spread)))
	if maxV < minV {
		minV, maxV = maxV, minV
	}
	avg = clampFPS(int(math.Round(avgF)))
	if maxV <= 5 {
		return avg, "<5", warn
	}
	return avg, fmt.Sprintf("%d-%d", minV, maxV), warn
}

func dynamicVerdict(sys SystemPowerProfile, req gameReqSet, low, med, high int) (string, string) {
	if !req.Heavy && low >= 90 {
		return "fpsVerdictSmoothMax", "ok"
	}
	if req.CapFPS > 0 && low >= 55 && med >= 55 {
		return "fpsVerdictLocked60", "ok"
	}
	if req.Heavy && sys.GPUPowerScore <= 28 && high < 30 {
		return "fpsVerdictWeakHW", "bad"
	}
	if req.Heavy && (med < 20 || (low < 22 && sys.IsIntegrated)) {
		return "fpsVerdictNeedGPU", "bad"
	}
	if high >= 55 {
		return "fpsVerdictPlayable", "ok"
	}
	if med >= 45 {
		return "fpsVerdictPlayable", "ok"
	}
	if low >= 40 {
		return "fpsVerdictOKLow", "warn"
	}
	if low >= 25 {
		return "fpsVerdictBorderline", "warn"
	}
	if sys.IsIntegrated {
		return "fpsVerdictHeavy", "warn"
	}
	return "fpsVerdictBorderline", "warn"
}

func clampFPS(v int) int {
	if v < 3 {
		return 3
	}
	if v > 400 {
		return 400
	}
	return v
}

func clampFPSFloat(v float64) float64 {
	if v < 3 {
		return 3
	}
	if v > 400 {
		return 400
	}
	return v
}

func clampScore(v float64) float64 {
	if v < 5 {
		return 5
	}
	if v > 100 {
		return 100
	}
	return math.Round(v*10) / 10
}

func calcGPUPowerScore(name string, integrated bool, vramGB float64) float64 {
	n := strings.ToLower(name)
	score := 40.0

	switch {
	case strings.Contains(n, "5090"):
		score = 100
	case strings.Contains(n, "5080"):
		score = 96
	case strings.Contains(n, "5070"):
		score = 88
	case strings.Contains(n, "4090"):
		score = 100
	case strings.Contains(n, "4080"):
		score = 94
	case strings.Contains(n, "4070 ti"):
		score = 88
	case strings.Contains(n, "4070"):
		score = 85
	case strings.Contains(n, "4060 ti"):
		score = 72
	case strings.Contains(n, "4060"):
		score = 65
	case strings.Contains(n, "3090"):
		score = 90
	case strings.Contains(n, "3080"):
		score = 84
	case strings.Contains(n, "3070"):
		score = 76
	case strings.Contains(n, "3060 ti"):
		score = 68
	case strings.Contains(n, "3060"):
		score = 60
	case strings.Contains(n, "2060 super"):
		score = 55
	case strings.Contains(n, "2070 super"):
		score = 62
	case strings.Contains(n, "2080"):
		score = 68
	case strings.Contains(n, "2060"):
		score = 50
	case strings.Contains(n, "1660"):
		score = 42
	case strings.Contains(n, "1650"):
		score = 34
	case strings.Contains(n, "1050"):
		score = 26
	case strings.Contains(n, "rx 7900"):
		score = 95
	case strings.Contains(n, "rx 7800"):
		score = 88
	case strings.Contains(n, "rx 7700"):
		score = 80
	case strings.Contains(n, "rx 7600"):
		score = 70
	case strings.Contains(n, "rx 6800"):
		score = 82
	case strings.Contains(n, "rx 6700"):
		score = 72
	case strings.Contains(n, "rx 6600"):
		score = 62
	case strings.Contains(n, "rx 580"):
		score = 38
	case strings.Contains(n, "arc a770"):
		score = 64
	case strings.Contains(n, "arc a750"):
		score = 58
	case strings.Contains(n, "arc a580"):
		score = 48
	case strings.Contains(n, "vega 11") || strings.Contains(n, "vega11"):
		score = 26
	case strings.Contains(n, "vega 8") || strings.Contains(n, "vega8"):
		score = 22
	case strings.Contains(n, "vega 7") || strings.Contains(n, "vega7"):
		score = 20
	case strings.Contains(n, "vega 6") || strings.Contains(n, "vega6"):
		score = 19
	case strings.Contains(n, "vega 5") || strings.Contains(n, "vega5"):
		score = 18
	case strings.Contains(n, "vega 3") || strings.Contains(n, "vega3"):
		score = 12
	case strings.Contains(n, "iris xe"):
		score = 22
	case strings.Contains(n, "iris"):
		score = 18
	case strings.Contains(n, "uhd"):
		score = 12
	case strings.Contains(n, "radeon graphics") || strings.Contains(n, "radeon(tm) graphics"):
		score = 20
	case n == "" || strings.Contains(n, "unknown"):
		score = 18
	default:
		// Heuristic from chip number for unknown discrete cards.
		if m := regexp.MustCompile(`(?:rtx|gtx|rx)\s*([0-9]{3,4})`).FindStringSubmatch(n); len(m) == 2 {
			if num, err := strconv.Atoi(m[1]); err == nil {
				switch {
				case num >= 4000:
					score = 80
				case num >= 3000:
					score = 70
				case num >= 2000:
					score = 55
				case num >= 1600:
					score = 40
				case num >= 1000:
					score = 30
				case num >= 600:
					score = 65
				case num >= 500:
					score = 50
				default:
					score = 35
				}
			}
		}
	}

	if integrated {
		if score > 35 {
			score = 28
		}
		score *= 0.95
	} else {
		// Dedicated VRAM soft boost / penalty.
		switch {
		case vramGB >= 12:
			score *= 1.04
		case vramGB >= 8:
			score *= 1.02
		case vramGB > 0 && vramGB < 4:
			score *= 0.92
		case vramGB > 0 && vramGB < 2:
			score *= 0.85
		}
	}

	if strings.Contains(n, "laptop") || strings.Contains(n, "mobile") || strings.Contains(n, "max-q") || strings.Contains(n, "max-p") {
		score *= 0.90
	}

	return clampScore(score)
}

func calcCPUPowerScore(name string, cores, threads int, freqMHz float64) float64 {
	n := strings.ToLower(name)
	base := 55.0

	switch {
	case strings.Contains(n, "ryzen ai 9") || strings.Contains(n, "ultra 9"):
		base = 92
	case strings.Contains(n, "ultra 7") || strings.Contains(n, "ryzen ai 7"):
		base = 84
	case strings.Contains(n, "ultra 5") || strings.Contains(n, "ryzen ai 5"):
		base = 74
	case strings.Contains(n, "i9-14") || strings.Contains(n, "i9-13") || strings.Contains(n, "ryzen 9 7") || strings.Contains(n, "ryzen 9 9"):
		base = 90
	case strings.Contains(n, "i7-14") || strings.Contains(n, "i7-13") || strings.Contains(n, "ryzen 7 7") || strings.Contains(n, "ryzen 7 9"):
		base = 82
	case strings.Contains(n, "i5-14") || strings.Contains(n, "i5-13") || strings.Contains(n, "ryzen 5 7") || strings.Contains(n, "ryzen 5 9"):
		base = 72
	case strings.Contains(n, "i9") || strings.Contains(n, "ryzen 9"):
		base = 85
	case strings.Contains(n, "i7") || strings.Contains(n, "ryzen 7"):
		base = 75
	case strings.Contains(n, "i5") || strings.Contains(n, "ryzen 5"):
		base = 65
	case strings.Contains(n, "4300u") || strings.Contains(n, "4500u") || strings.Contains(n, "5500u"):
		base = 42
	case strings.Contains(n, "i3") || strings.Contains(n, "ryzen 3"):
		base = 45
	case strings.Contains(n, "celeron") || strings.Contains(n, "pentium") || strings.Contains(n, "athlon"):
		base = 28
	}

	if cores <= 0 {
		cores = 4
	}
	if threads <= 0 {
		threads = cores
	}

	coreFactor := float64(cores) / 6.0
	if coreFactor > 1.35 {
		coreFactor = 1.35
	}
	if coreFactor < 0.45 {
		coreFactor = 0.45
	}

	threadBonus := 1.0 + math.Min(0.12, float64(threads-cores)*0.02)

	freqFactor := 1.0
	if freqMHz > 0 {
		freqFactor = freqMHz / 3200.0
		if freqFactor > 1.25 {
			freqFactor = 1.25
		}
		if freqFactor < 0.7 {
			freqFactor = 0.7
		}
	}

	// Low-power U-series / thin-laptop penalty.
	if regexp.MustCompile(`(?i)(\d{4}u\b|ryzen\s+[3579]\s+\d{4}u)`).MatchString(n) ||
		(strings.Contains(n, "u") && (strings.Contains(n, "ryzen") || strings.Contains(n, "core"))) {
		base *= 0.88
	}

	score := base * (0.55 + 0.45*coreFactor) * threadBonus * freqFactor
	return clampScore(score)
}
