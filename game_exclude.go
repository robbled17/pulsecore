package main

import (
	_ "embed"
	"encoding/json"
	"regexp"
	"strings"
	"sync"
)

//go:embed game_exclude_rules.json
var gameExcludeRulesJSON []byte

type gameExcludeRules struct {
	ExactNames          []string `json:"exactNames"`
	NamePatterns        []string `json:"namePatterns"`
	EpicNamespaces      []string `json:"epicNamespaces"`
	EpicAppNamePatterns []string `json:"epicAppNamePatterns"`
	PathContains        []string `json:"pathContains"`
}

type compiledExcludeRules struct {
	exact       map[string]struct{}
	nameRes     []*regexp.Regexp
	epicNS      map[string]struct{}
	epicAppRes  []*regexp.Regexp
	pathNeedles []string
}

var (
	excludeRulesOnce sync.Once
	excludeRules     compiledExcludeRules
)

func loadGameExcludeRules() compiledExcludeRules {
	excludeRulesOnce.Do(func() {
		var raw gameExcludeRules
		if err := json.Unmarshal(gameExcludeRulesJSON, &raw); err != nil {
			return
		}
		exact := make(map[string]struct{}, len(raw.ExactNames))
		for _, n := range raw.ExactNames {
			k := strings.ToLower(strings.TrimSpace(n))
			if k != "" {
				exact[k] = struct{}{}
			}
		}
		nameRes := make([]*regexp.Regexp, 0, len(raw.NamePatterns))
		for _, p := range raw.NamePatterns {
			re, err := regexp.Compile(p)
			if err == nil {
				nameRes = append(nameRes, re)
			}
		}
		epicNS := make(map[string]struct{}, len(raw.EpicNamespaces))
		for _, n := range raw.EpicNamespaces {
			k := strings.ToLower(strings.TrimSpace(n))
			if k != "" {
				epicNS[k] = struct{}{}
			}
		}
		epicAppRes := make([]*regexp.Regexp, 0, len(raw.EpicAppNamePatterns))
		for _, p := range raw.EpicAppNamePatterns {
			re, err := regexp.Compile(p)
			if err == nil {
				epicAppRes = append(epicAppRes, re)
			}
		}
		paths := make([]string, 0, len(raw.PathContains))
		for _, p := range raw.PathContains {
			p = strings.TrimSpace(p)
			if p != "" {
				paths = append(paths, strings.ToLower(strings.ReplaceAll(p, `/`, `\`)))
			}
		}
		excludeRules = compiledExcludeRules{
			exact:       exact,
			nameRes:     nameRes,
			epicNS:      epicNS,
			epicAppRes:  epicAppRes,
			pathNeedles: paths,
		}
	})
	return excludeRules
}

// nonGameExcludeInput carries fields used by rule-based launcher/engine filtering.
type nonGameExcludeInput struct {
	Name             string
	ID               string
	InstallPath      string
	EpicNamespace    string
	EpicAppName      string
	EpicIsApp        *bool // nil = unknown; false = tooling / non-application
	EpicCategoryHint string
}

// isExcludedNonGame returns true for launchers, engines, and redistributables.
// Matching is rule-based (exact names, anchored patterns, Epic namespace/type, paths) —
// not bare substring checks that would drop real titles like "SteamWorld Dig 2".
func isExcludedNonGame(in nonGameExcludeInput) bool {
	r := loadGameExcludeRules()
	name := strings.TrimSpace(in.Name)
	lowName := strings.ToLower(name)

	if _, ok := r.exact[lowName]; ok {
		return true
	}
	for _, re := range r.nameRes {
		if re.MatchString(name) {
			return true
		}
	}

	ns := strings.ToLower(strings.TrimSpace(in.EpicNamespace))
	if ns != "" {
		if _, ok := r.epicNS[ns]; ok {
			return true
		}
	}
	app := strings.TrimSpace(in.EpicAppName)
	if app == "" {
		// Fall back to id tail after "epic:".
		id := strings.ToLower(strings.TrimSpace(in.ID))
		if i := strings.LastIndex(id, ":"); i >= 0 {
			app = id[i+1:]
		}
	}
	for _, re := range r.epicAppRes {
		if re.MatchString(app) {
			return true
		}
	}
	if in.EpicIsApp != nil && !*in.EpicIsApp {
		return true
	}
	cat := strings.ToLower(strings.TrimSpace(in.EpicCategoryHint))
	if cat != "" && (cat == "engine" || cat == "editor" || cat == "tool" || cat == "launcher" ||
		strings.Contains(cat, "unreal") || strings.Contains(cat, "redistributable")) {
		return true
	}

	path := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(in.InstallPath), `/`, `\`))
	if path != "" {
		for _, needle := range r.pathNeedles {
			if strings.Contains(path, needle) {
				return true
			}
		}
	}
	return false
}

func isExcludedGameInfo(g GameInfo) bool {
	return isExcludedNonGame(nonGameExcludeInput{
		Name:        g.Name,
		ID:          g.ID,
		InstallPath: g.InstallPath,
	})
}

// GetUserHiddenGames returns titles the user hid from GamePulse.
// Includes poster/year so the modal can reuse the same cover path as the catalog.
func (a *App) GetUserHiddenGames() []HiddenGamePref {
	a.mu.RLock()
	defer a.mu.RUnlock()
	out := make([]HiddenGamePref, 0, len(a.prefs.HiddenGames))
	for _, h := range a.prefs.HiddenGames {
		id := strings.TrimSpace(h.ID)
		if id == "" {
			continue
		}
		item := repairHiddenGamePref(HiddenGamePref{
			ID:        id,
			Name:      strings.TrimSpace(h.Name),
			PosterURL: strings.TrimSpace(h.PosterURL),
			Year:      h.Year,
		})
		if item.PosterURL == "" {
			resolved := resolveGame(item.ID)
			if resolved.PosterURL != "" {
				item.PosterURL = resolved.PosterURL
			} else if resolved.HeaderURL != "" {
				item.PosterURL = resolved.HeaderURL
			} else if item.Name != "" {
				resolvedByName := resolveGame(item.Name)
				if resolvedByName.PosterURL != "" {
					item.PosterURL = resolvedByName.PosterURL
				} else if resolvedByName.HeaderURL != "" {
					item.PosterURL = resolvedByName.HeaderURL
				}
			}
			if item.Year <= 0 && resolved.Year > 0 {
				item.Year = resolved.Year
			}
			if item.Name == "" || looksLikeSlug(item.Name, item.ID) {
				if resolved.Name != "" {
					item.Name = resolved.Name
				}
			}
		}
		out = append(out, item)
	}
	return out
}

// HideUserGame marks a catalog/installed title as hidden by the user.
func (a *App) HideUserGame(id, name string) bool {
	return a.HideUserGameMeta(id, name, "", 0)
}

// HideUserGameMeta stores cover/year so the hidden-games modal can render cards.
func (a *App) HideUserGameMeta(id, name, posterURL string, year int) bool {
	id = strings.TrimSpace(id)
	if id == "" {
		return false
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	low := strings.ToLower(id)
	for i, h := range a.prefs.HiddenGames {
		if strings.EqualFold(h.ID, id) || strings.ToLower(h.ID) == low {
			if posterURL != "" {
				a.prefs.HiddenGames[i].PosterURL = posterURL
			}
			if year > 0 {
				a.prefs.HiddenGames[i].Year = year
			}
			if name != "" {
				a.prefs.HiddenGames[i].Name = strings.TrimSpace(name)
			}
			_ = a.savePrefs(a.prefs)
			return true
		}
	}
	a.prefs.HiddenGames = append(a.prefs.HiddenGames, HiddenGamePref{
		ID: id, Name: strings.TrimSpace(name), PosterURL: strings.TrimSpace(posterURL), Year: year,
	})
	_ = a.savePrefs(a.prefs)
	return true
}

// RestoreHiddenGame removes a title from the user-hidden list.
func (a *App) RestoreHiddenGame(id string) bool {
	id = strings.TrimSpace(id)
	if id == "" {
		return false
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	next := a.prefs.HiddenGames[:0]
	found := false
	for _, h := range a.prefs.HiddenGames {
		if strings.EqualFold(h.ID, id) {
			found = true
			continue
		}
		next = append(next, h)
	}
	if !found {
		return false
	}
	a.prefs.HiddenGames = append([]HiddenGamePref(nil), next...)
	_ = a.savePrefs(a.prefs)
	return true
}

func looksLikeSlug(name, id string) bool {
	n := strings.TrimSpace(name)
	if n == "" || strings.EqualFold(n, id) {
		return true
	}
	return !strings.Contains(n, " ") && (strings.Contains(n, "_") || strings.Contains(n, "-") || strings.ToLower(n) == strings.ToLower(id))
}

// repairHiddenGamePref replaces a fuzzy false title (cities2 → a puzzle, oblivion_remaster → Turok)
// with the folder alias. A mismatched poster is dropped so GetUserHiddenGames can load the real cover.
func repairHiddenGamePref(h HiddenGamePref) HiddenGamePref {
	title, ok := canonicalFolderTitle(h.ID)
	if !ok {
		title, ok = canonicalFolderTitle(h.Name)
	}
	if !ok {
		return h
	}
	if namesMatchStrict(h.Name, title) {
		return h
	}
	h.Name = title
	h.PosterURL = ""
	return h
}

func (a *App) isUserHiddenGameID(id string) bool {
	id = strings.TrimSpace(id)
	if id == "" || a == nil {
		return false
	}
	a.mu.RLock()
	defer a.mu.RUnlock()
	for _, h := range a.prefs.HiddenGames {
		if strings.EqualFold(h.ID, id) {
			return true
		}
	}
	return false
}
