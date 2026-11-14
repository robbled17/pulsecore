package main

import (
	"crypto/sha1"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

//go:embed embedded_game_db.json
var embeddedGameDBJSON []byte

// GameDBEntry is an offline CIS / launcher-heavy title with VRAM/RAM and cover.
type GameDBEntry struct {
	ID       string   `json:"id"`
	Title    string   `json:"title"`
	Aliases  []string `json:"aliases"`
	MinVRAM  float64  `json:"min_vram"`
	RecVRAM  float64  `json:"rec_vram"`
	MinRAM   float64  `json:"min_ram"`
	RecRAM   float64  `json:"rec_ram"`
	Year     int      `json:"year"`
	Category string   `json:"category"`
	Cover    string   `json:"cover"`
}

var (
	gameDBOnce sync.Once
	gameDB     []GameDBEntry
)

func loadGameDB() []GameDBEntry {
	gameDBOnce.Do(func() {
		_ = json.Unmarshal(embeddedGameDBJSON, &gameDB)
		if gameDB == nil {
			gameDB = []GameDBEntry{}
		}
	})
	return gameDB
}

func normGameKey(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, "ё", "е")
	repl := strings.NewReplacer("-", " ", "_", " ", ":", " ", "'", "", "\"", "")
	s = repl.Replace(s)
	return strings.Join(strings.Fields(s), " ")
}

func findLocalGameEntry(query string) *GameDBEntry {
	q := normGameKey(query)
	if q == "" {
		return nil
	}
	db := loadGameDB()
	for i := range db {
		e := &db[i]
		cands := make([]string, 0, 2+len(e.Aliases))
		cands = append(cands, e.Title, e.ID)
		cands = append(cands, e.Aliases...)
		for _, c := range cands {
			if namesMatchStrict(q, c) {
				return e
			}
		}
	}
	return nil
}

func findLocalGameEntryForInfo(game GameInfo) *GameDBEntry {
	if e := findLocalGameEntry(game.ID); e != nil {
		return e
	}
	if e := findLocalGameEntry(game.Name); e != nil {
		return e
	}
	return nil
}

func gameInfoFromDBEntry(e *GameDBEntry) GameInfo {
	if e == nil {
		return GameInfo{}
	}
	cover := strings.TrimSpace(e.Cover)
	if cover != "" {
		cover = EnsureCoverCached(e.Title, cover)
	}
	cat := e.Category
	if cat == "" {
		cat = "indie"
	}
	return GameInfo{
		ID:        e.ID,
		Name:      e.Title,
		Genre:     cat,
		Weight:    0.6,
		PosterURL: cover,
		HeaderURL: cover,
		Category:  cat,
		Year:      e.Year,
		Badge:     badgeForCategory(cat, e.Year),
		Engine:    "LocalDB",
	}
}

// ResolveGameMeta cascades: local JSON → Steam Store → RAWG, then caches the cover.
func (a *App) ResolveGameMeta(query string) GameInfo {
	q := strings.TrimSpace(query)
	if q == "" {
		return GameInfo{}
	}
	// Folder/id aliases win before any catalog or store search.
	if title, ok := canonicalFolderTitle(q); ok {
		q = title
	}
	if e := findLocalGameEntry(q); e != nil {
		return gameInfoFromDBEntry(e)
	}
	hits := searchSteamStore(q)
	if g, ok := firstStrictGameHit(q, hits); ok {
		return cacheResolvedCover(g)
	}
	hits = searchSteamApps(q)
	if g, ok := firstStrictGameHit(q, hits); ok {
		return cacheResolvedCover(g)
	}
	hits = searchRAWGGames(q)
	if g, ok := firstStrictGameHit(q, hits); ok {
		return cacheResolvedCover(g)
	}
	return GameInfo{}
}

type cheapSharkGameHit struct {
	GameID   string `json:"gameID"`
	SteamApp string `json:"steamAppID"`
	External string `json:"external"`
	Thumb    string `json:"thumb"`
}

func searchCheapSharkCover(title string) string {
	q := strings.TrimSpace(title)
	if q == "" {
		return ""
	}
	client := &http.Client{Timeout: 5 * time.Second}
	urlStr := fmt.Sprintf("https://www.cheapshark.com/api/1.0/games?title=%s&limit=5", url.QueryEscape(q))
	req, err := http.NewRequest(http.MethodGet, urlStr, nil)
	if err != nil {
		return ""
	}
	req.Header.Set("User-Agent", "PulseCore/1.0")
	resp, err := client.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		if resp != nil {
			_ = resp.Body.Close()
		}
		return ""
	}
	defer resp.Body.Close()
	var hits []cheapSharkGameHit
	if err := json.NewDecoder(resp.Body).Decode(&hits); err != nil || len(hits) == 0 {
		return ""
	}
	for _, h := range hits {
		if h.Thumb != "" {
			return h.Thumb
		}
	}
	return ""
}

type wikipediaSummaryResp struct {
	Thumbnail struct {
		Source string `json:"source"`
	} `json:"thumbnail"`
}

func searchWikipediaCover(title string) string {
	q := strings.TrimSpace(title)
	if q == "" {
		return ""
	}
	client := &http.Client{Timeout: 5 * time.Second}
	urlStr := fmt.Sprintf("https://en.wikipedia.org/api/rest_v1/page/summary/%s", url.PathEscape(q))
	req, err := http.NewRequest(http.MethodGet, urlStr, nil)
	if err != nil {
		return ""
	}
	req.Header.Set("User-Agent", "PulseCore/1.0")
	resp, err := client.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		if resp != nil {
			_ = resp.Body.Close()
		}
		return ""
	}
	defer resp.Body.Close()
	var sum wikipediaSummaryResp
	if err := json.NewDecoder(resp.Body).Decode(&sum); err == nil && sum.Thumbnail.Source != "" {
		return sum.Thumbnail.Source
	}
	return ""
}

var (
	coverLinksLock sync.RWMutex
	coverLinksMap  map[string]string
)

func coverLinksFilePath() string {
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		base, _ = os.UserCacheDir()
	}
	dir := filepath.Join(base, "PulseCore")
	_ = os.MkdirAll(dir, 0o755)
	return filepath.Join(dir, "cover_links.json")
}

func loadCoverLinks() map[string]string {
	coverLinksLock.Lock()
	defer coverLinksLock.Unlock()
	if coverLinksMap != nil {
		return coverLinksMap
	}
	coverLinksMap = make(map[string]string)
	p := coverLinksFilePath()
	data, err := os.ReadFile(p)
	if err == nil {
		_ = json.Unmarshal(data, &coverLinksMap)
	}
	return coverLinksMap
}

func saveCoverLink(key, urlStr string) {
	if key == "" || urlStr == "" {
		return
	}
	coverLinksLock.Lock()
	defer coverLinksLock.Unlock()
	if coverLinksMap == nil {
		coverLinksMap = make(map[string]string)
	}
	coverLinksMap[key] = urlStr
	p := coverLinksFilePath()
	if data, err := json.MarshalIndent(coverLinksMap, "", "  "); err == nil {
		_ = os.WriteFile(p, data, 0o644)
	}
}

// ResolveGameCover performs a multi-stage lookup for a game poster:
// 1. Check local links cache (%LOCALAPPDATA%\PulseCore\cover_links.json)
// 2. Local database
// 3. Steam Store Search API
// 4. CheapShark API (covers Tarkov, EA, Epic, Ubisoft, etc.)
// 5. Wikipedia API
// Returns direct HTTPS URL with zero disk footprint.
func (a *App) ResolveGameCover(query string) string {
	q := strings.TrimSpace(query)
	if q == "" {
		return ""
	}
	if title, ok := canonicalFolderTitle(q); ok {
		q = title
	}
	key := coverCacheKey(q)

	// 1. Check cover_links.json
	links := loadCoverLinks()
	coverLinksLock.RLock()
	cachedURL, found := links[key]
	coverLinksLock.RUnlock()
	if found && cachedURL != "" {
		return cachedURL
	}

	// 2. Local database
	if e := findLocalGameEntry(q); e != nil && e.Cover != "" {
		saveCoverLink(key, e.Cover)
		return e.Cover
	}

	// 3. Steam Store Search
	hits := searchSteamStore(q)
	for _, h := range hits {
		if namesMatchStrict(q, h.Name) || len(hits) == 1 {
			targetURL := h.PosterURL
			if targetURL == "" {
				targetURL = h.HeaderURL
			}
			if targetURL != "" {
				saveCoverLink(key, targetURL)
				return targetURL
			}
		}
	}

	// 4. CheapShark API
	if thumb := searchCheapSharkCover(q); thumb != "" {
		saveCoverLink(key, thumb)
		return thumb
	}

	// 5. Wikipedia API
	if thumb := searchWikipediaCover(q); thumb != "" {
		saveCoverLink(key, thumb)
		return thumb
	}

	return ""
}

func firstStrictGameHit(query string, hits []GameInfo) (GameInfo, bool) {
	for _, h := range hits {
		if namesMatchStrict(query, h.Name) {
			return h, true
		}
	}
	return GameInfo{}, false
}

func cacheResolvedCover(g GameInfo) GameInfo {
	if g.PosterURL != "" {
		g.PosterURL = EnsureCoverCached(g.Name, g.PosterURL)
	}
	if g.HeaderURL != "" {
		g.HeaderURL = EnsureCoverCached(g.Name+"-hdr", g.HeaderURL)
	}
	return g
}

func coversCacheDir() (string, error) {
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		var err error
		base, err = os.UserCacheDir()
		if err != nil {
			return "", err
		}
	}
	dir := filepath.Join(base, "PulseCore", "cache", "covers")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

func coverCacheKey(title string) string {
	sum := sha1.Sum([]byte(normGameKey(title)))
	return hex.EncodeToString(sum[:])
}

// steamAppIDFromCoverURL extracts /steam/apps/<id>/ from CDN poster URLs.
// Preferring Steam app id avoids title/slug mismatches that map the wrong cover.
func steamAppIDFromCoverURL(remoteURL string) int {
	low := strings.ToLower(remoteURL)
	const marker = "/steam/apps/"
	i := strings.Index(low, marker)
	if i < 0 {
		return 0
	}
	rest := remoteURL[i+len(marker):]
	n := 0
	for _, r := range rest {
		if r < '0' || r > '9' {
			break
		}
		n = n*10 + int(r-'0')
		if n > 1_000_000_000 {
			return 0
		}
	}
	return n
}

func coverCacheKeyFor(title, remoteURL string) string {
	if id := steamAppIDFromCoverURL(remoteURL); id > 0 {
		return fmt.Sprintf("steam_%d", id)
	}
	return coverCacheKey(title)
}

func coverFileURL(absPath string) string {
	absPath = filepath.Clean(absPath)
	p := filepath.ToSlash(absPath)
	if len(p) >= 2 && p[1] == ':' {
		return "file:///" + p
	}
	return "file://" + p
}

// EnsureCoverCached stores direct HTTPS URL in %LOCALAPPDATA%\PulseCore\cover_links.json without saving image files to disk.
func EnsureCoverCached(title, remoteURL string) string {
	remoteURL = strings.TrimSpace(remoteURL)
	if remoteURL == "" {
		return ""
	}
	key := coverCacheKeyFor(title, remoteURL)
	saveCoverLink(key, remoteURL)
	return remoteURL
}

type rawgSearchResp struct {
	Results []struct {
		ID       int     `json:"id"`
		Name     string  `json:"name"`
		Released string  `json:"released"`
		BGImage  string  `json:"background_image"`
		Rating   float64 `json:"rating"`
	} `json:"results"`
}

func rawgAPIKey() string {
	for _, k := range []string{"RAWG_API_KEY", "PULSECORE_RAWG_KEY"} {
		if v := strings.TrimSpace(os.Getenv(k)); v != "" {
			return v
		}
	}
	return ""
}

func searchRAWGGames(query string) []GameInfo {
	q := strings.TrimSpace(query)
	key := rawgAPIKey()
	if q == "" || key == "" {
		return nil
	}
	client := &http.Client{Timeout: 6 * time.Second}
	endpoint := fmt.Sprintf(
		"https://api.rawg.io/api/games?key=%s&search=%s&page_size=8",
		url.QueryEscape(key),
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
	var parsed rawgSearchResp
	if err := json.Unmarshal(body, &parsed); err != nil || len(parsed.Results) == 0 {
		return nil
	}
	out := make([]GameInfo, 0, minInt(8, len(parsed.Results)))
	for i, r := range parsed.Results {
		if i >= 8 || strings.TrimSpace(r.Name) == "" {
			continue
		}
		year := 0
		if len(r.Released) >= 4 {
			fmt.Sscanf(r.Released[:4], "%d", &year)
		}
		cat, y2, badge, genre, weight := classifyTitle(r.Name, year)
		if year == 0 {
			year = y2
		}
		cover := strings.TrimSpace(r.BGImage)
		g := GameInfo{
			ID:        fmt.Sprintf("rawg:%d", r.ID),
			Name:      r.Name,
			Genre:     genre,
			Weight:    weight,
			PosterURL: cover,
			HeaderURL: cover,
			Category:  cat,
			Year:      year,
			Badge:     badge,
			Engine:    "RAWG",
		}
		out = append(out, g)
	}
	return out
}

// applyLocalDBRequirements overlays min/rec VRAM from the offline CIS database.
func applyLocalDBRequirements(game GameInfo, req gameReqSet) gameReqSet {
	e := findLocalGameEntryForInfo(game)
	if e == nil {
		return req
	}
	if e.MinVRAM > 0 {
		req.Low.ReqVRAMGB = e.MinVRAM
		if req.Med.ReqVRAMGB < e.MinVRAM {
			req.Med.ReqVRAMGB = (e.MinVRAM + e.RecVRAM) / 2
			if req.Med.ReqVRAMGB < e.MinVRAM {
				req.Med.ReqVRAMGB = e.MinVRAM
			}
		}
	}
	if e.RecVRAM > 0 {
		req.High.ReqVRAMGB = e.RecVRAM
		req.Ultra.ReqVRAMGB = e.RecVRAM + 1
		if req.Ultra.ReqVRAMGB < req.High.ReqVRAMGB {
			req.Ultra.ReqVRAMGB = req.High.ReqVRAMGB
		}
	}
	if e.RecRAM > 0 {
		req.ReqRAMGB = e.RecRAM
	} else if e.MinRAM > 0 {
		req.ReqRAMGB = e.MinRAM
	}
	return req
}
