package main

import "testing"

func TestSteamAppIDFromCoverURL(t *testing.T) {
	cases := []struct {
		url string
		id  int
	}{
		{"https://shared.cloudflare.steamstatic.com/store_item_assets/steam/apps/730/library_600x900.jpg", 730},
		{"https://cdn.akamai.steamstatic.com/steam/apps/570/header.jpg", 570},
		{"https://example.com/cover.jpg", 0},
		{"", 0},
	}
	for _, c := range cases {
		got := steamAppIDFromCoverURL(c.url)
		if got != c.id {
			t.Fatalf("url %q: got %d want %d", c.url, got, c.id)
		}
	}
	key := coverCacheKeyFor("cs2", cases[0].url)
	if key != "steam_730" {
		t.Fatalf("cache key %q", key)
	}
}
