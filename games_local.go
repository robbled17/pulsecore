package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/sys/windows/registry"
)

func normalizeGameKey(name string) string {
	n := strings.ToLower(strings.TrimSpace(name))
	n = strings.ReplaceAll(n, "™", "")
	n = strings.ReplaceAll(n, "®", "")
	re := regexp.MustCompile(`[^a-z0-9а-яё]+`)
	return re.ReplaceAllString(n, "")
}

// folderTitleAliases maps install-folder / slug ids to the real store title.
// Keys are normalizeGameKey output (letters and digits only).
var folderTitleAliases = map[string]string{
	"cities2":                          "Cities: Skylines II",
	"citiesskylines2":                  "Cities: Skylines II",
	"citiesskylinesii":                 "Cities: Skylines II",
	"oblivionremaster":                 "The Elder Scrolls IV: Oblivion Remastered",
	"oblivionremastered":               "The Elder Scrolls IV: Oblivion Remastered",
	"elderscrollsivoblivionremastered": "The Elder Scrolls IV: Oblivion Remastered",
	"skyrimspecialedition":             "The Elder Scrolls V: Skyrim Special Edition",
	"skyrimse":                         "The Elder Scrolls V: Skyrim Special Edition",
	"fallout4":                         "Fallout 4",
	"witcher3":                         "The Witcher 3: Wild Hunt",
	"thewitcher3":                      "The Witcher 3: Wild Hunt",
	"baldursgate3":                     "Baldur's Gate 3",
	"bg3":                              "Baldur's Gate 3",
	"cyberpunk2077":                    "Cyberpunk 2077",
	"rdr2":                             "Red Dead Redemption 2",
	"reddeadredemption2":               "Red Dead Redemption 2",
	"buckshotroulette":                 "Buckshot Roulette",
}

// canonicalFolderTitle resolves a folder slug before any fuzzy catalog search.
func canonicalFolderTitle(query string) (string, bool) {
	key := normalizeGameKey(query)
	key = strings.TrimPrefix(key, "steam")
	if key == "" {
		return "", false
	}
	if title, ok := folderTitleAliases[key]; ok && strings.TrimSpace(title) != "" {
		return title, true
	}
	return "", false
}

// namesMatchStrict is equality after case and punctuation folding.
// A query must not match a longer DLC or puzzle title that merely contains the words.
func namesMatchStrict(a, b string) bool {
	if strings.TrimSpace(a) == "" || strings.TrimSpace(b) == "" {
		return false
	}
	if normGameKey(a) == normGameKey(b) {
		return true
	}
	return normalizeGameKey(a) != "" && normalizeGameKey(a) == normalizeGameKey(b)
}

// normalizeGameInfo fills safe defaults so missing covers/fields never panic callers.
func normalizeGameInfo(g GameInfo) GameInfo {
	g.ID = strings.TrimSpace(g.ID)
	g.Name = strings.TrimSpace(g.Name)
	g.Genre = strings.TrimSpace(g.Genre)
	g.Category = strings.TrimSpace(g.Category)
	g.Badge = strings.TrimSpace(g.Badge)
	g.Engine = strings.TrimSpace(g.Engine)
	g.PosterURL = strings.TrimSpace(g.PosterURL)
	g.HeaderURL = strings.TrimSpace(g.HeaderURL)
	g.InstallPath = strings.TrimSpace(g.InstallPath)
	if g.ID == "" && g.Name != "" {
		g.ID = "name:" + normalizeGameKey(g.Name)
	}
	if g.Weight < 0 {
		g.Weight = 0
	}
	if g.Year < 0 {
		g.Year = 0
	}
	if g.SteamAppID < 0 {
		g.SteamAppID = 0
	}
	return g
}

// keepInstalledGame applies non-game exclusion rules; false means drop the entry.
func keepInstalledGame(g GameInfo, epic nonGameExcludeInput) bool {
	in := epic
	if in.Name == "" {
		in.Name = g.Name
	}
	if in.ID == "" {
		in.ID = g.ID
	}
	if in.InstallPath == "" {
		in.InstallPath = g.InstallPath
	}
	if isExcludedNonGame(in) || isExcludedGameInfo(g) {
		return false
	}
	return true
}

func mergeGameCatalog(base []GameInfo, extras ...[]GameInfo) []GameInfo {
	out := make([]GameInfo, 0, len(base)+32)
	index := map[string]int{}
	add := func(g GameInfo) {
		g = normalizeGameInfo(g)
		if g.Name == "" {
			return
		}
		if !keepInstalledGame(g, nonGameExcludeInput{}) {
			return
		}
		key := normalizeGameKey(g.Name)
		if key == "" {
			return
		}
		if i, ok := index[key]; ok {
			cur := out[i]
			if g.SteamAppID > 0 && cur.SteamAppID <= 0 {
				cur.SteamAppID = g.SteamAppID
			}
			if cur.PosterURL == "" {
				cur.PosterURL = g.PosterURL
			}
			if cur.HeaderURL == "" {
				cur.HeaderURL = g.HeaderURL
			}
			if g.FromSteam && !strings.Contains(cur.Badge, "Steam") {
				if cur.Badge == "" {
					cur.Badge = "Steam"
				} else if !strings.Contains(cur.Badge, "Epic") {
					cur.Badge = "Steam"
				} else {
					cur.Badge = "Steam / Epic"
				}
			}
			if strings.Contains(g.Badge, "Epic") {
				if strings.Contains(cur.Badge, "Steam") {
					cur.Badge = "Steam / Epic"
				} else if cur.Badge == "" {
					cur.Badge = "Epic"
				}
			}
			if g.Weight > cur.Weight {
				cur.Weight = g.Weight
			}
			out[i] = cur
			return
		}
		index[key] = len(out)
		out = append(out, g)
	}
	for _, g := range base {
		add(g)
	}
	for _, list := range extras {
		for _, g := range list {
			add(g)
		}
	}
	return out
}

func steamInstallPath() string {
	k, err := registry.OpenKey(registry.CURRENT_USER, `Software\Valve\Steam`, registry.QUERY_VALUE)
	if err != nil {
		k, err = registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\WOW6432Node\Valve\Steam`, registry.QUERY_VALUE)
		if err != nil {
			return ""
		}
	}
	defer k.Close()
	path, _, err := k.GetStringValue("SteamPath")
	if err != nil || path == "" {
		path, _, _ = k.GetStringValue("InstallPath")
	}
	return strings.ReplaceAll(path, `/`, `\`)
}

func parseLibraryFolders(vdf string) []string {
	out := make([]string, 0, 4)
	// "path"		"D:\\SteamLibrary"
	re := regexp.MustCompile(`(?i)"path"\s+"([^"]+)"`)
	for _, m := range re.FindAllStringSubmatch(vdf, -1) {
		p := strings.ReplaceAll(m[1], `\\`, `\`)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func parseAppManifest(path string) (appID int, name, installDir string, sizeBytes int64) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return 0, "", "", 0
	}
	text := string(raw)
	if m := regexp.MustCompile(`(?i)"appid"\s+"(\d+)"`).FindStringSubmatch(text); len(m) == 2 {
		appID, _ = strconv.Atoi(m[1])
	}
	if m := regexp.MustCompile(`(?i)"name"\s+"([^"]+)"`).FindStringSubmatch(text); len(m) == 2 {
		name = m[1]
	}
	if m := regexp.MustCompile(`(?i)"installdir"\s+"([^"]+)"`).FindStringSubmatch(text); len(m) == 2 {
		installDir = m[1]
	}
	if m := regexp.MustCompile(`(?i)"SizeOnDisk"\s+"(\d+)"`).FindStringSubmatch(text); len(m) == 2 {
		sizeBytes, _ = strconv.ParseInt(m[1], 10, 64)
	}
	return appID, name, installDir, sizeBytes
}

func localSteamPoster(steamRoot string, appID int) string {
	if steamRoot == "" || appID <= 0 {
		return ""
	}
	p := filepath.Join(steamRoot, "appcache", "librarycache", fmt.Sprintf("%d_library_600x900.jpg", appID))
	if _, err := os.Stat(p); err == nil {
		return "file:///" + strings.ReplaceAll(p, `\`, `/`)
	}
	return steamLibraryPoster(appID)
}

func collectInstalledSteamGames() []GameInfo {
	root := steamInstallPath()
	if root == "" {
		return nil
	}
	folders := []string{root}
	vdfPath := filepath.Join(root, "steamapps", "libraryfolders.vdf")
	if raw, err := os.ReadFile(vdfPath); err == nil {
		folders = append(folders, parseLibraryFolders(string(raw))...)
	}
	seen := map[int]bool{}
	out := make([]GameInfo, 0, 32)
	for _, folder := range folders {
		apps := filepath.Join(folder, "steamapps")
		matches, _ := filepath.Glob(filepath.Join(apps, "appmanifest_*.acf"))
		for _, mf := range matches {
			appID, name, manifestDir, sizeBytes := parseAppManifest(mf)
			if appID <= 0 || name == "" || seen[appID] {
				continue
			}
			installDir := filepath.Join(apps, "common")
			if manifestDir != "" {
				installDir = filepath.Join(apps, "common", manifestDir)
			}
			var sizeGB float64
			if sizeBytes > 0 {
				sizeGB = float64(sizeBytes) / (1024 * 1024 * 1024)
			}
			g := normalizeGameInfo(GameInfo{
				ID:            fmt.Sprintf("steam:%d", appID),
				Name:          name,
				SteamAppID:    appID,
				InstallPath:   installDir,
				InstallSizeGB: sizeGB,
				FromSteam:     true,
			})
			if !keepInstalledGame(g, nonGameExcludeInput{Name: name, ID: g.ID, InstallPath: installDir}) {
				continue
			}
			seen[appID] = true
			cat, year, badge, genre, weight := classifyTitle(name, 0)
			g.Genre = genre
			g.Weight = weight + 0.05
			g.PosterURL = localSteamPoster(root, appID)
			g.HeaderURL = steamHeaderPoster(appID)
			g.Category = cat
			g.Year = year
			g.Badge = finalizeGameBadge(g, requirementsFor(g))
			if g.Badge == "" {
				g.Badge = badge
			}
			out = append(out, normalizeGameInfo(g))
		}
	}
	return out
}

type epicManifest struct {
	DisplayName          string   `json:"DisplayName"`
	InstallLocation      string   `json:"InstallLocation"`
	LaunchExecutable     string   `json:"LaunchExecutable"`
	AppName              string   `json:"AppName"`
	CatalogNamespace     string   `json:"CatalogNamespace"`
	CatalogItemId        string   `json:"CatalogItemId"`
	AppCategories        []string `json:"AppCategories"`
	Category             string   `json:"Category"`
	bIsIncomplete        bool     `json:"bIsIncomplete"`
	bIsApplication       *bool    `json:"bIsApplication"`
	bIsIncompleteInstall bool     `json:"bIsIncompleteInstall"`
}

func collectInstalledEpicGames() []GameInfo {
	dir := `C:\ProgramData\Epic\EpicGamesLauncher\Data\Manifests`
	matches, err := filepath.Glob(filepath.Join(dir, "*.item"))
	if err != nil {
		return nil
	}
	out := make([]GameInfo, 0, 16)
	seen := map[string]bool{}
	for _, path := range matches {
		raw, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var m epicManifest
		if json.Unmarshal(raw, &m) != nil {
			continue
		}
		name := strings.TrimSpace(m.DisplayName)
		if name == "" || m.bIsIncomplete || m.bIsIncompleteInstall {
			continue
		}
		// Tooling / non-application Epic entries often omit LaunchExecutable.
		if m.LaunchExecutable == "" && (m.bIsApplication == nil || !*m.bIsApplication) {
			continue
		}
		appName := strings.TrimSpace(m.AppName)
		catHint := strings.TrimSpace(m.Category)
		if catHint == "" && len(m.AppCategories) > 0 {
			catHint = strings.Join(m.AppCategories, " ")
		}
		key := normalizeGameKey(name)
		if key == "" || seen[key] {
			continue
		}
		id := "epic:" + key
		if appName != "" {
			id = "epic:" + strings.ToLower(appName)
		}
		g := normalizeGameInfo(GameInfo{
			ID:          id,
			Name:        name,
			InstallPath: strings.TrimSpace(m.InstallLocation),
			Badge:       "Epic",
		})
		excl := nonGameExcludeInput{
			Name:             name,
			ID:               id,
			InstallPath:      g.InstallPath,
			EpicNamespace:    m.CatalogNamespace,
			EpicAppName:      appName,
			EpicIsApp:        m.bIsApplication,
			EpicCategoryHint: catHint,
		}
		if !keepInstalledGame(g, excl) {
			continue
		}
		seen[key] = true
		cat, year, _, genre, weight := classifyTitle(name, 0)
		g.Genre = genre
		g.Weight = weight + 0.03
		g.Category = cat
		g.Year = year
		out = append(out, normalizeGameInfo(g))
	}
	return out
}

func installedGameLibraries() []GameInfo {
	return ScanInstalledGames()
}

// ScanInstalledGames discovers Steam + Epic installs and merges duplicates.
func ScanInstalledGames() []GameInfo {
	return mergeGameCatalog(nil, collectInstalledSteamGames(), collectInstalledEpicGames())
}

// ScanInstalledGames is exposed to the frontend for manual refresh.
func (a *App) ScanInstalledGames() []GameInfo {
	return ScanInstalledGames()
}
