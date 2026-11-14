package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExcludeNonGamesRules(t *testing.T) {
	exclude := []string{
		"Unreal Engine 5.4",
		"Epic Games Launcher",
		"Godot Engine",
		"Unity Hub",
		"Steam",
	}
	for _, name := range exclude {
		if !isExcludedNonGame(nonGameExcludeInput{Name: name}) {
			t.Fatalf("expected exclude: %q", name)
		}
	}
	keep := []string{
		"SteamWorld Dig 2",
		"Unity of Command",
		"Waiting for Godot",
	}
	for _, name := range keep {
		if isExcludedNonGame(nonGameExcludeInput{Name: name}) {
			t.Fatalf("expected keep: %q", name)
		}
	}
}

func TestExcludeEpicUnrealNamespace(t *testing.T) {
	if !isExcludedNonGame(nonGameExcludeInput{
		Name:          "Unreal Engine 5.4",
		ID:            "epic:ue_5.4",
		EpicNamespace: "ue",
		EpicAppName:   "ue_5.4",
	}) {
		t.Fatal("Epic Unreal namespace must be excluded")
	}
}

func TestNormalizeGameInfoMissingFields(t *testing.T) {
	g := normalizeGameInfo(GameInfo{Name: "Coverless Title"})
	if g.Name != "Coverless Title" {
		t.Fatalf("name: %q", g.Name)
	}
	if g.ID == "" {
		t.Fatal("id must be synthesized when missing")
	}
	if g.PosterURL != "" || g.HeaderURL != "" {
		t.Fatal("missing covers must stay empty strings, not panic")
	}
	// merge must accept coverless games that are real titles
	out := mergeGameCatalog(nil, []GameInfo{{Name: "Coverless Title"}})
	if len(out) != 1 || out[0].PosterURL != "" {
		t.Fatalf("unexpected merge: %+v", out)
	}
}

func TestMergeDropsEnginesKeepsGames(t *testing.T) {
	out := mergeGameCatalog(nil, []GameInfo{
		{ID: "epic:ue_5.4", Name: "Unreal Engine 5.4"},
		{ID: "x:egl", Name: "Epic Games Launcher"},
		{ID: "x:godot", Name: "Godot Engine"},
		{ID: "x:uhub", Name: "Unity Hub"},
		{ID: "steam:0", Name: "Steam"},
		{ID: "steam:1", Name: "SteamWorld Dig 2"},
		{ID: "x:uoc", Name: "Unity of Command"},
		{ID: "x:wfg", Name: "Waiting for Godot"},
	})
	names := map[string]bool{}
	for _, g := range out {
		names[g.Name] = true
	}
	for _, bad := range []string{"Unreal Engine 5.4", "Epic Games Launcher", "Godot Engine", "Unity Hub", "Steam"} {
		if names[bad] {
			t.Fatalf("should be excluded: %s", bad)
		}
	}
	for _, good := range []string{"SteamWorld Dig 2", "Unity of Command", "Waiting for Godot"} {
		if !names[good] {
			t.Fatalf("should remain: %s", good)
		}
	}
}

func TestBuckshotIsNotAAATemplate(t *testing.T) {
	g := synthesizeGame("Buckshot Roulette")
	if strings.EqualFold(g.Category, "aaa") || strings.Contains(strings.ToLower(g.Badge), "aaa") {
		t.Fatalf("small title classified as AAA: %+v", g)
	}
	req := requirementsFor(g)
	if req.High.ReqVRAMGB > 2.01 {
		t.Fatalf("default VRAM too high: %.2f", req.High.ReqVRAMGB)
	}
	if req.ReqRAMGB > 4.01 {
		t.Fatalf("default RAM too high: %.2f", req.ReqRAMGB)
	}
	size, ok := storageNeedFor(g)
	if !ok || size > 8 || size <= 0 {
		t.Fatalf("indie size=%v ok=%v", size, ok)
	}
	if strings.Contains(strings.ToLower(finalizeGameBadge(g, req)), "aaa") {
		t.Fatal("AAA badge without high requirements")
	}
}

func TestInstalledSizeBeatsDefault(t *testing.T) {
	g := GameInfo{Name: "Buckshot Roulette", Category: "indie", InstallSizeGB: 1.2}
	size, ok := storageNeedFor(g)
	if !ok || size < 1.1 || size > 1.3 {
		t.Fatalf("folder size ignored: %v", size)
	}
}

func TestAAABadgeRequiresRealWeight(t *testing.T) {
	light := GameInfo{Name: "Tiny Indie", Category: "aaa", Badge: "AAA", Year: 2024}
	req := requirementsFor(light)
	if strings.Contains(strings.ToLower(finalizeGameBadge(light, req)), "aaa") && req.High.ReqVRAMGB < 6 {
		size, _ := storageNeedFor(light)
		if size <= 35 {
			t.Fatalf("AAA badge on light title size=%.1f vram=%.1f", size, req.High.ReqVRAMGB)
		}
	}
	heavy := GameInfo{ID: "cyberpunk", Name: "Cyberpunk 2077", Category: "aaa", Year: 2020}
	hreq := requirementsFor(heavy)
	badge := finalizeGameBadge(heavy, hreq)
	if !strings.Contains(strings.ToLower(badge), "aaa") {
		t.Fatalf("known heavy title lost AAA: %s vram=%.1f", badge, hreq.High.ReqVRAMGB)
	}
}

func TestParseAppManifestSize(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "appmanifest_1.acf")
	body := `"AppState"
{
	"appid"		"123"
	"name"		"Buckshot Roulette"
	"installdir"		"Buckshot Roulette"
	"SizeOnDisk"		"2147483648"
}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	id, name, install, size := parseAppManifest(path)
	if id != 123 || name != "Buckshot Roulette" || install != "Buckshot Roulette" || size != 2147483648 {
		t.Fatalf("manifest parse: %d %q %q %d", id, name, install, size)
	}
}
