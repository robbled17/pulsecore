package main

import "testing"

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
