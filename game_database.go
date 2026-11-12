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

func scoreGameNameMatch(q, cand string) int {
	if q == "" || cand == "" {
		return 0
	}
	if cand == q {
		return 100
	}
	// Whole-word / token match only — avoids "eft" inside "theft" (GTA V).
	padded := " " + q + " "
	token := " " + cand + " "
	if strings.Contains(padded, token) {
		return 90
	}
	if len(cand) >= 6 && (strings.HasPrefix(q, cand+" ") || strings.HasSuffix(q, " "+cand)) {
		return 85
	}
	if len(cand) >= 8 && len(q) >= 8 {
		if strings.Contains(q, cand) {
			return 75
		}
		if strings.Contains(cand, q) {
			return 72
		}
	}
	return 0
}

func findLocalGameEntry(query string) *GameDBEntry {
	q := normGameKey(query)
	if q == "" {
		return nil
	}
	db := loadGameDB()
	bestIdx := -1
	bestScore := 0
	for i := range db {
		e := &db[i]
		cands := make([]string, 0, 2+len(e.Aliases))
		cands = append(cands, e.Title, e.ID)
		cands = append(cands, e.Aliases...)
		for _, c := range cands {
			score := scoreGameNameMatch(q, normGameKey(c))
			if score > bestScore {
				bestScore = score
				bestIdx = i
			}
		}
	}
	if bestScore < 72 || bestIdx < 0 {
		return nil
	}
	return &db[bestIdx]
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
	if e := findLocalGameEntry(q); e != nil {
		return gameInfoFromDBEntry(e)
	}
	hits := searchSteamStore(q)
	if len(hits) == 0 {
		hits = searchSteamApps(q)
	}
	if len(hits) == 0 {
		hits = searchRAWGGames(q)
	}
	if len(hits) == 0 {
		return GameInfo{}
	}
	g := hits[0]
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

// EnsureCoverCached downloads a poster once into %LOCALAPPDATA%\PulseCore\cache\covers\.
// Returns a file:// URL on success, or the original remote URL on failure.
func EnsureCoverCached(title, remoteURL string) string {
	remoteURL = strings.TrimSpace(remoteURL)
	if remoteURL == "" {
		return ""
	}
	if strings.HasPrefix(strings.ToLower(remoteURL), "file:") {
		return remoteURL
	}
	dir, err := coversCacheDir()
	if err != nil {
		return remoteURL
	}
	ext := ".jpg"
	lower := strings.ToLower(remoteURL)
	switch {
	case strings.Contains(lower, ".png"):
		ext = ".png"
	case strings.Contains(lower, ".webp"):
		ext = ".webp"
	}
	name := coverCacheKeyFor(title, remoteURL) + ext
	dest := filepath.Join(dir, name)
	if st, err := os.Stat(dest); err == nil && st.Size() > 512 {
		return coverFileURL(dest)
	}
	client := &http.Client{Timeout: 8 * time.Second}
	req, err := http.NewRequest(http.MethodGet, remoteURL, nil)
	if err != nil {
		return remoteURL
	}
	req.Header.Set("User-Agent", "PulseCore/1.0")
	resp, err := client.Do(req)
	if err != nil {
		return remoteURL
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return remoteURL
	}
	tmp := dest + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return remoteURL
	}
	_, copyErr := io.Copy(f, io.LimitReader(resp.Body, 8<<20))
	_ = f.Close()
	if copyErr != nil {
		_ = os.Remove(tmp)
		return remoteURL
	}
	if err := os.Rename(tmp, dest); err != nil {
		_ = os.Remove(tmp)
		return remoteURL
	}
	return coverFileURL(dest)
}

type rawgSearchResp struct {
	Results []struct {
		ID        int     `json:"id"`
		Name      string  `json:"name"`
		Released  string  `json:"released"`
		BGImage   string  `json:"background_image"`
		Rating    float64 `json:"rating"`
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
	return req
}
