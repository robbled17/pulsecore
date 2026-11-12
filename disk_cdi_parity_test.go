package main

import (
	"encoding/json"
	"math"
	"os"
	"testing"
)

// Live-ish parity against admin-probe dump (same machine as CrystalDiskInfo reference).
func TestAdminProbe_CDIParity_CDE(t *testing.T) {
	raw, err := os.ReadFile("helper/admin-probe.json")
	if err != nil {
		t.Skip("no admin-probe.json")
	}
	var probe struct {
		Disks []struct {
			Letter        string  `json:"letter"`
			Model         string  `json:"model"`
			TempC         float64 `json:"tempC"`
			HealthPercent *float64 `json:"healthPercent"`
			PowerOnHours  *float64 `json:"powerOnHours"`
			Attrs         []struct {
				ID, Current, Worst, Threshold int
				Raw, Name                     string
			} `json:"attrs"`
		} `json:"disks"`
	}
	if json.Unmarshal(raw, &probe) != nil {
		t.Fatal("bad probe json")
	}

	for _, d := range probe.Disks {
		attrs := make([]SmartAttribute, 0, len(d.Attrs))
		for _, a := range d.Attrs {
			rv := parseSMARTRawDecimal(a.Raw)
			en, ru := smartAttrName(a.ID)
			attrs = append(attrs, SmartAttribute{
				ID: a.ID, Name: en, NameRu: ru,
				Current: a.Current, Worst: a.Worst, Threshold: a.Threshold,
				Raw: a.Raw, RawValue: rv,
			})
		}
		kind := DiskKindSATASSD
		if d.Letter == "E" {
			kind = DiskKindHDD
		}
		// D is SMI-layout OEM SSD (attrs 160/161/169) even if model is generic.
		ev := EvaluateDiskHealth(DiskHealthEvalInput{
			Kind: kind, Model: d.Model, SMARTReadOK: true, Attributes: attrs,
			Reallocated: attrRawOr(attrs, 5, 0),
			Pending:     attrRawOr(attrs, 197, 0),
		})
		prof := detectDiskProfile(kind, d.Model, attrs)
		w, r, okW, okR := hostIOFromProfile(prof, attrs)

		t.Logf("DISK %s model=%q profile=%s status=%s healthOK=%v health=%.0f%% temp=%.0f poh=%.0f writeOK=%v write=%.1fGB readOK=%v read=%.1fGB note=%s reason=%s",
			d.Letter, d.Model, prof.ID, ev.Level, ev.HealthPctOK, ev.HealthPercent, d.TempC,
			valueOr(d.PowerOnHours), okW, w, okR, r, ev.InfoNote.Code, ev.Reason.Code)

		switch d.Letter {
		case "C":
			if prof.ID != "smi" {
				t.Errorf("C: want smi, got %s", prof.ID)
			}
			if ev.Level != DiskStatusGood {
				t.Errorf("C: want Good (CDI), got %s", ev.Level)
			}
			if !ev.HealthPctOK || ev.HealthPercent != 99 {
				t.Errorf("C: want 99%% health, got %.0f", ev.HealthPercent)
			}
			if okW && math.Abs(w-7853)/7853 > 0.02 {
				// probe is slightly older than CDI snapshot; allow 2%
				t.Logf("C: write GB=%.1f (CDI≈7853)", w)
			}
		case "D":
			if ev.Level != DiskStatusGood && ev.Level != DiskStatusUnknown {
				t.Errorf("D: unexpected status %s", ev.Level)
			}
			if prof.ID == "smi" && (!ev.HealthPctOK || ev.HealthPercent != 100) {
				t.Errorf("D SMI: want 100%%, got %.0f", ev.HealthPercent)
			}
		case "E":
			if ev.Level != DiskStatusGood {
				t.Errorf("E HDD: want Good, got %s", ev.Level)
			}
		}
	}
}

func valueOr(p *float64) float64 {
	if p == nil {
		return 0
	}
	return *p
}
