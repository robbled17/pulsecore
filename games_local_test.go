package main

import "testing"

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
