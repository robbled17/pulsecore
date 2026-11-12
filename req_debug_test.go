package main

import "testing"

func TestGTAVDoesNotMatchTarkovAlias(t *testing.T) {
	e := findLocalGameEntryForInfo(GameInfo{ID: "gta5", Name: "Grand Theft Auto V", SteamAppID: 271590})
	if e != nil {
		t.Fatalf("GTA V must not resolve to local entry %q", e.ID)
	}
	req := requirementsFor(GameInfo{ID: "gta5", Name: "Grand Theft Auto V", SteamAppID: 271590, Category: "aaa", Year: 2015})
	if req.Ultra.ReqVRAMGB > 5 {
		t.Fatalf("GTA V ultra VRAM inflated: %v", req.Ultra.ReqVRAMGB)
	}
}
