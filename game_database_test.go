package main

import (
	"strings"
	"testing"
)

func TestFindLocalGameEntryAliases(t *testing.T) {
	cases := []string{"Tanks Blitz", "World of Tanks Blitz", "танки блиц", "Мир Танков", "STALCRAFT", "Genshin Impact"}
	for _, q := range cases {
		e := findLocalGameEntry(q)
		if e == nil {
			t.Fatalf("expected local DB hit for %q", q)
		}
		if e.Cover == "" {
			t.Fatalf("expected cover for %q", q)
		}
		if e.MinVRAM <= 0 {
			t.Fatalf("expected min_vram for %q", q)
		}
	}
}

func TestResolveGameMetaLocal(t *testing.T) {
	a := &App{}
	g := a.ResolveGameMeta("Tanks Blitz")
	if g.ID == "" || g.Name == "" {
		t.Fatalf("ResolveGameMeta local failed: %+v", g)
	}
}

func TestFolderAliasStrictName(t *testing.T) {
	title, ok := canonicalFolderTitle("cities2")
	if !ok || title != "Cities: Skylines II" {
		t.Fatalf("cities2 alias: %q ok=%v", title, ok)
	}
	title, ok = canonicalFolderTitle("oblivion_remaster")
	if !ok || !strings.Contains(title, "Oblivion Remastered") {
		t.Fatalf("oblivion alias: %q", title)
	}
	if namesMatchStrict("cities2", "Super Jigsaw Puzzle: Cities 2") {
		t.Fatal("substring must not match a puzzle title")
	}
	if namesMatchStrict("oblivion remaster", "Turok 3") {
		t.Fatal("unrelated title matched")
	}
	if !namesMatchStrict("Cities: Skylines II", "cities skylines ii") {
		t.Fatal("normalized equality failed")
	}
	repaired := repairHiddenGamePref(HiddenGamePref{ID: "cities2", Name: "Super Jigsaw Puzzle", PosterURL: "http://wrong"})
	if repaired.Name != "Cities: Skylines II" || repaired.PosterURL != "" {
		t.Fatalf("repair: %+v", repaired)
	}
	if findLocalGameEntry("cities2") != nil && !namesMatchStrict("cities2", findLocalGameEntry("cities2").Title) && !namesMatchStrict("cities2", findLocalGameEntry("cities2").ID) {
		t.Fatal("local DB substring hit for cities2")
	}
}
