package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/robbled17/zeropulse/loc"
)

// DiskStatusLevel matches CrystalDiskInfo-style traffic lights.
const (
	DiskStatusGood     = "good"
	DiskStatusCaution  = "caution"
	DiskStatusBad      = "bad"
	DiskStatusUnknown  = "unknown"
)

// DiskKind classifies media for health % rules.
const (
	DiskKindNVMe    = "nvme"
	DiskKindSATASSD = "sata_ssd"
	DiskKindHDD     = "hdd"
	DiskKindUnknown = "unknown"
)

// DiskHealthEval is the pure result of SMART-based status evaluation.
// Reason/HealthSource are codes+params for frontend i18n; Ru/En kept for logs/tests.
type DiskHealthEval struct {
	Level            string // good|caution|bad|unknown
	StatusCode       string // diskStateGood|… for frontend
	StatusRu         string
	StatusEn         string
	Reason           loc.Msg
	ReasonRu         string
	ReasonEn         string
	// InfoNote is a calm All-specs note (no pill / home card), e.g. stable remapped blocks.
	InfoNote         loc.Msg
	TriggerAttrID    int
	TriggerAttrHex   string
	HealthPercent    float64
	HealthPctOK      bool // false for HDD / unknown wear
	HealthSource     string
	HealthSourceMsg  loc.Msg
	HealthSourceRu   string
	HealthSourceEn   string
	ProfileID        string
	LogLine          string
}

func setDiskLevel(out *DiskHealthEval, level string) {
	out.Level = level
	switch level {
	case DiskStatusGood:
		out.StatusCode = "diskStateGood"
		out.StatusRu, out.StatusEn = "Хорошо", "Good"
	case DiskStatusCaution:
		out.StatusCode = "diskStateWarn"
		out.StatusRu, out.StatusEn = "Внимание", "Caution"
	case DiskStatusBad:
		out.StatusCode = "diskStateBad"
		out.StatusRu, out.StatusEn = "Плохо", "Bad"
	default:
		out.StatusCode = "diskStateUnknown"
		out.StatusRu, out.StatusEn = "Неизвестно", "Unknown"
	}
}

func setDiskReason(out *DiskHealthEval, code string, params map[string]interface{}, ru, en string) {
	out.Reason = loc.M(code, params)
	out.ReasonRu, out.ReasonEn = ru, en
}

func setDiskHealthSource(out *DiskHealthEval, code string, params map[string]interface{}, ru, en string) {
	out.HealthSourceMsg = loc.M(code, params)
	out.HealthSourceRu, out.HealthSourceEn = ru, en
}

// DiskHealthEvalInput is everything needed to evaluate without IO.
type DiskHealthEvalInput struct {
	Kind              string // nvme|sata_ssd|hdd|unknown
	Model             string
	Attributes        []SmartAttribute
	NVMeCriticalWarn  byte
	NVMeSpare         int
	NVMeSpareThresh   int
	NVMePercentUsed   int
	NVMePercentUsedOK bool
	NVMeMediaErrors   uint64
	Reallocated       int64
	Pending           int64
	Uncorrectable     int64
	SMARTReadOK       bool // false → Unknown
	// Prior* from local disk-watch history (optional trend).
	PriorPendingOK      bool
	PriorPending        int64
	PriorReallocatedOK  bool
	PriorReallocated    int64
}

// ATA attributes CrystalDiskInfo treats as critical (normalized ≤ threshold → Bad).
var ataCriticalAttrIDs = map[int]bool{
	5: true, 10: true, 196: true, 197: true, 198: true, 201: true, 220: true,
}

// Sector counter attrs: raw > 0 → Caution (if not already Bad).
var ataSectorCautionIDs = map[int]bool{
	5: true, 196: true, 197: true, 198: true,
}

// Wear attribute candidates (normalized remaining life 1-100). Expandable by vendor.
// Value meaning: higher = more life left (CDI / common SSD firmware).
type wearAttrSpec struct {
	ID       int
	NameHint string
	// Invert: if true, normalized value is "used %" and life = 100-value
	Invert bool
}

var defaultWearAttrs = []wearAttrSpec{
	{ID: 202, NameHint: "Percent Lifetime Remain"},
	{ID: 231, NameHint: "SSD Life Left"},
	{ID: 233, NameHint: "Media Wearout"},
	{ID: 177, NameHint: "Wear Leveling"},
	{ID: 173, NameHint: "Wear Leveling Count"},
	{ID: 232, NameHint: "Available Reserved Space"},
}

// vendorWearAttrs extends wear detection for known model prefixes (case-insensitive).
var vendorWearAttrs = map[string][]wearAttrSpec{
	"samsung": {{ID: 177}, {ID: 173}, {ID: 241, Invert: true}}, // 241 sometimes endurance on Samsung - prefer 177
	"kingston": {{ID: 231}, {ID: 233}, {ID: 202}},
	"crucial":  {{ID: 202}, {ID: 231}},
	"wd ":      {{ID: 241, Invert: true}, {ID: 231}, {ID: 232}},
	"western":  {{ID: 231}, {ID: 232}},
	"compit":   {{ID: 231}, {ID: 233}, {ID: 177}, {ID: 169}, {ID: 245, Invert: true}},
	"intel":    {{ID: 233}, {ID: 232}},
}

// EvaluateDiskHealth implements CrystalDiskInfo-like status rules (pure).
func EvaluateDiskHealth(in DiskHealthEvalInput) DiskHealthEval {
	out := DiskHealthEval{Level: DiskStatusUnknown}
	if !in.SMARTReadOK {
		setDiskLevel(&out, DiskStatusUnknown)
		setDiskReason(&out, "diskReasonSmartUnread", nil, "SMART не прочитан", "SMART was not read")
		setDiskHealthSource(&out, "diskHealthSrcSmartUnread", nil, "не определяется - SMART не прочитан", "unavailable - SMART not read")
		out.LogLine = "disk_status=unknown reason=smart_not_read"
		return out
	}

	switch in.Kind {
	case DiskKindNVMe:
		evalNVMeHealth(in, &out)
	case DiskKindSATASSD:
		evalSATASSDHealth(in, &out)
	case DiskKindHDD:
		evalHDDHealth(in, &out)
	default:
		// Try ATA attrs if present, else unknown
		if len(in.Attributes) > 0 {
			evalATAGeneric(in, &out)
		} else {
			setDiskLevel(&out, DiskStatusUnknown)
			setDiskReason(&out, "diskReasonKindUnknown", nil, "тип диска и SMART-атрибуты не определены", "disk kind and SMART attributes unknown")
			setDiskHealthSource(&out, "diskHealthSrcUnavailable", nil, "не определяется", "unavailable")
			out.LogLine = "disk_status=unknown reason=kind_unknown"
		}
	}
	return out
}

func evalNVMeHealth(in DiskHealthEvalInput, out *DiskHealthEval) {
	if in.NVMePercentUsedOK {
		out.HealthPercent = float64(100 - in.NVMePercentUsed)
		if out.HealthPercent < 0 {
			out.HealthPercent = 0
		}
		if out.HealthPercent > 100 {
			out.HealthPercent = 100
		}
		out.HealthPctOK = true
		out.HealthSource = "nvme_percentage_used"
		setDiskHealthSource(out, "diskHealthSrcNVMe", nil, "NVMe: 100 − Percentage Used", "NVMe: 100 − Percentage Used")
	} else {
		setDiskHealthSource(out, "diskHealthSrcNoPctUsed", nil, "не определяется - Percentage Used недоступен", "unavailable - Percentage Used missing")
	}

	if in.NVMeCriticalWarn != 0 {
		hex := fmt.Sprintf("%02X", in.NVMeCriticalWarn)
		setDiskLevel(out, DiskStatusBad)
		setDiskReason(out, "diskReasonNVMeCritWarn", loc.P("hex", hex),
			fmt.Sprintf("NVMe Critical Warning = 0x%s", hex),
			fmt.Sprintf("NVMe Critical Warning = 0x%s", hex))
		out.LogLine = fmt.Sprintf("disk_status=bad trigger=nvme_critical_warning=0x%s", hex)
		log.Printf("[disk-health] %s model=%q", out.LogLine, in.Model)
		return
	}
	if in.NVMeSpareThresh > 0 && in.NVMeSpare > 0 && in.NVMeSpare <= in.NVMeSpareThresh {
		setDiskLevel(out, DiskStatusBad)
		setDiskReason(out, "diskReasonNVMeSpare", loc.P("spare", in.NVMeSpare, "thresh", in.NVMeSpareThresh),
			fmt.Sprintf("Available Spare (%d) ≤ порога (%d)", in.NVMeSpare, in.NVMeSpareThresh),
			fmt.Sprintf("Available Spare (%d) ≤ threshold (%d)", in.NVMeSpare, in.NVMeSpareThresh))
		out.LogLine = fmt.Sprintf("disk_status=bad trigger=nvme_spare=%d thresh=%d", in.NVMeSpare, in.NVMeSpareThresh)
		log.Printf("[disk-health] %s model=%q", out.LogLine, in.Model)
		return
	}
	if out.HealthPctOK && out.HealthPercent <= 5 {
		pct := int(out.HealthPercent)
		setDiskLevel(out, DiskStatusBad)
		setDiskReason(out, "diskReasonSSDLifeCrit", loc.P("pct", pct),
			fmt.Sprintf("Остаток ресурса SSD ≤ 5%% (%.0f%%)", out.HealthPercent),
			fmt.Sprintf("SSD life remaining ≤ 5%% (%.0f%%)", out.HealthPercent))
		out.LogLine = fmt.Sprintf("disk_status=bad trigger=nvme_life=%.0f", out.HealthPercent)
		log.Printf("[disk-health] %s model=%q", out.LogLine, in.Model)
		return
	}
	if out.HealthPctOK && out.HealthPercent < 10 {
		pct := int(out.HealthPercent)
		setDiskLevel(out, DiskStatusCaution)
		setDiskReason(out, "diskReasonSSDLifeLow", loc.P("pct", pct),
			fmt.Sprintf("Низкий остаток ресурса SSD (%.0f%%)", out.HealthPercent),
			fmt.Sprintf("Low SSD life remaining (%.0f%%)", out.HealthPercent))
		out.LogLine = fmt.Sprintf("disk_status=caution trigger=nvme_life=%.0f", out.HealthPercent)
		log.Printf("[disk-health] %s model=%q", out.LogLine, in.Model)
		return
	}
	if in.NVMeMediaErrors > 0 {
		setDiskLevel(out, DiskStatusCaution)
		setDiskReason(out, "diskReasonNVMeMedia", loc.P("n", in.NVMeMediaErrors),
			fmt.Sprintf("Media Errors = %d", in.NVMeMediaErrors),
			fmt.Sprintf("Media Errors = %d", in.NVMeMediaErrors))
		out.LogLine = fmt.Sprintf("disk_status=caution trigger=nvme_media_errors=%d", in.NVMeMediaErrors)
		log.Printf("[disk-health] %s model=%q", out.LogLine, in.Model)
		return
	}
	setGood(out)
	out.LogLine = "disk_status=good kind=nvme"
}

func evalSATASSDHealth(in DiskHealthEvalInput, out *DiskHealthEval) {
	prof := detectDiskProfile(DiskKindSATASSD, in.Model, in.Attributes)
	out.ProfileID = prof.ID

	// Unknown SSD profile: do not guess scary sector meanings.
	if prof.ID == "unknown_ssd" {
		if life, id, ok := wearLifeFromProfile(prof, in.Attributes); ok {
			out.HealthPercent = float64(life)
			out.HealthPctOK = true
			out.HealthSource = "sata_wear"
			setDiskHealthSource(out, "diskHealthSrcSATAWear", loc.P("id", id, "hex", fmt.Sprintf("%02X", id)),
				fmt.Sprintf("SATA SSD: атрибут износа ID %d", id),
				fmt.Sprintf("SATA SSD: wear attribute ID %d", id))
		}
		setDiskLevel(out, DiskStatusUnknown)
		setDiskReason(out, "diskReasonSSDProfileUnknown", nil,
			"Профиль контроллера SSD неизвестен — статус не оцениваю",
			"SSD controller profile unknown — not scoring status")
		out.LogLine = "disk_status=unknown reason=ssd_profile_unknown"
		return
	}

	// Bad: normalized current ≤ threshold on critical attrs (CDI-style).
	if bad, id, msg, reasonRu, reasonEn := ataCriticalFail(in.Attributes); bad {
		setDiskLevel(out, DiskStatusBad)
		out.Reason = msg
		out.ReasonRu, out.ReasonEn = reasonRu, reasonEn
		out.TriggerAttrID = id
		out.TriggerAttrHex = fmt.Sprintf("%02X", id)
		out.LogLine = fmt.Sprintf("disk_status=bad trigger=attr_0x%02X current<=threshold", id)
		log.Printf("[disk-health] %s model=%q reason=%s", out.LogLine, in.Model, reasonRu)
		applyWearPercentProfile(in, prof, out)
		return
	}

	wearOK := applyWearPercentProfile(in, prof, out)
	if wearOK {
		wearLife := int(out.HealthPercent)
		wearID := 0
		if id, _, ok := wearLifeFromProfile(prof, in.Attributes); ok {
			wearID = id
		} else if id, _, ok, _ := findWearLife(in.Model, in.Attributes); ok {
			wearID = id
		}
		if wearLife <= 1 {
			setDiskLevel(out, DiskStatusBad)
			setDiskReason(out, "diskReasonSATAWearCrit", loc.P("attrId", wearID, "value", wearLife),
				fmt.Sprintf("Остаток ресурса SSD ≤ порога (атрибут %d = %d)", wearID, wearLife),
				fmt.Sprintf("SSD life at/below threshold (attribute %d = %d)", wearID, wearLife))
			out.TriggerAttrID = wearID
			out.TriggerAttrHex = fmt.Sprintf("%02X", wearID)
			out.LogLine = fmt.Sprintf("disk_status=bad trigger=wear_attr_0x%02X=%d", wearID, wearLife)
			return
		}
		if wearLife < 10 {
			setDiskLevel(out, DiskStatusCaution)
			setDiskReason(out, "diskReasonSATAWearLow", loc.P("attrId", wearID, "pct", wearLife),
				fmt.Sprintf("Низкий остаток ресурса (атрибут %d = %d%%)", wearID, wearLife),
				fmt.Sprintf("Low life remaining (attribute %d = %d%%)", wearID, wearLife))
			out.TriggerAttrID = wearID
			out.LogLine = fmt.Sprintf("disk_status=caution trigger=wear_attr_0x%02X=%d", wearID, wearLife)
			return
		}
	}

	pending := in.Pending
	realloc := in.Reallocated
	if pending == 0 {
		if v, ok := attrRawByID(in.Attributes, 197); ok {
			pending = int64(v & 0xFFFF)
		}
	}
	if realloc == 0 {
		if v, ok := attrRawByID(in.Attributes, 5); ok {
			realloc = int64(v & 0xFFFF)
		}
	}

	// Trend / large absolute — only then Caution for SSD sector counters.
	if grew, from, to, which := ssdSectorGrowth(prof, pending, realloc, in); grew {
		setDiskLevel(out, DiskStatusCaution)
		if which == "pending" {
			setDiskReason(out, "diskReasonPendingGrew", loc.P("from", from, "to", to),
				fmt.Sprintf("%d секторов ожидают переназначения (было %d)", to, from),
				fmt.Sprintf("%d sectors pending reallocation (was %d)", to, from))
			out.TriggerAttrID, out.TriggerAttrHex = 197, "C5"
		} else {
			setDiskReason(out, "diskReasonReallocatedGrew", loc.P("from", from, "to", to),
				fmt.Sprintf("Переназначенных блоков: выросло с %d до %d", from, to),
				fmt.Sprintf("Remapped blocks grew from %d to %d", from, to))
			out.TriggerAttrID, out.TriggerAttrHex = 5, "05"
		}
		out.LogLine = fmt.Sprintf("disk_status=caution trigger=%s_growth %d→%d", which, from, to)
		return
	}
	if prof.SectorLargeThreshold > 0 && (pending >= prof.SectorLargeThreshold || realloc >= prof.SectorLargeThreshold) {
		n := pending
		hex, id := "C5", 197
		code := "diskReasonPendingHuman"
		ru, en := fmt.Sprintf("%d секторов ожидают переназначения", n), fmt.Sprintf("%d sectors pending reallocation", n)
		if realloc >= pending {
			n, hex, id = realloc, "05", 5
			code = "diskReasonReallocatedHuman"
			ru, en = fmt.Sprintf("%d переназначенных блоков", n), fmt.Sprintf("%d remapped blocks", n)
		}
		setDiskLevel(out, DiskStatusCaution)
		setDiskReason(out, code, loc.P("n", n), ru, en)
		out.TriggerAttrID, out.TriggerAttrHex = id, hex
		out.LogLine = fmt.Sprintf("disk_status=caution trigger=sector_large n=%d", n)
		return
	}

	// Stable low remapped/pending: Good + calm All-specs note (no pill).
	if pending > 0 || realloc > 0 {
		n := pending
		kind := "pending"
		if realloc >= pending {
			n, kind = realloc, "reallocated"
		}
		if kind == "pending" {
			out.InfoNote = loc.M("diskNoteStablePending", loc.P("n", n))
		} else {
			out.InfoNote = loc.M("diskNoteStableRemap", loc.P("n", n))
		}
	}

	setGood(out)
	out.LogLine = fmt.Sprintf("disk_status=good kind=sata_ssd profile=%s", prof.ID)
}

func applyWearPercentProfile(in DiskHealthEvalInput, prof DiskControllerProfile, out *DiskHealthEval) bool {
	if life, id, ok := wearLifeFromProfile(prof, in.Attributes); ok {
		out.HealthPercent = float64(life)
		out.HealthPctOK = true
		out.HealthSource = "sata_wear"
		setDiskHealthSource(out, "diskHealthSrcSATAWear", loc.P("id", id, "hex", fmt.Sprintf("%02X", id)),
			fmt.Sprintf("SATA SSD: атрибут износа ID %d (0x%02X)", id, id),
			fmt.Sprintf("SATA SSD: wear attribute ID %d (0x%02X)", id, id))
		return true
	}
	wearID, wearLife, wearOK, wearSrc := findWearLife(in.Model, in.Attributes)
	if wearOK {
		out.HealthPercent = float64(wearLife)
		out.HealthPctOK = true
		out.HealthSource = wearSrc
		setDiskHealthSource(out, "diskHealthSrcSATAWear", loc.P("id", wearID, "hex", fmt.Sprintf("%02X", wearID)),
			fmt.Sprintf("SATA SSD: атрибут износа ID %d (0x%02X)", wearID, wearID),
			fmt.Sprintf("SATA SSD: wear attribute ID %d (0x%02X)", wearID, wearID))
		return true
	}
	setDiskHealthSource(out, "diskHealthSrcNoWear", nil,
		"не определяется - атрибут износа не найден для этого SSD",
		"unavailable - wear attribute not found for this SSD")
	return false
}

func ssdSectorGrowth(prof DiskControllerProfile, pending, realloc int64, in DiskHealthEvalInput) (grew bool, from, to int64, which string) {
	th := prof.SectorGrowthThreshold
	if th <= 0 {
		th = 5
	}
	if in.PriorPendingOK && pending > in.PriorPending && (pending-in.PriorPending) >= th {
		return true, in.PriorPending, pending, "pending"
	}
	if in.PriorReallocatedOK && realloc > in.PriorReallocated && (realloc-in.PriorReallocated) >= th {
		return true, in.PriorReallocated, realloc, "reallocated"
	}
	return false, 0, 0, ""
}

func evalHDDHealth(in DiskHealthEvalInput, out *DiskHealthEval) {
	// HDD health % from key SMART attrs (not SSD wear-out).
	// Formula (documented for UI tooltip):
	//   start 100;
	//   − min(40, realloc×5)           ID 05 / in.Reallocated
	//   − min(40, pending×10)          ID 197
	//   − min(40, uncorrectable×10)    ID 198
	//   − min(20, realloc_events×5)    ID 196
	//   − 15 if spin-retry raw > 0     ID 10
	//   − 10 if seek-error current < 90 (when attr present) ID 07
	//   clamp 0…100
	out.HealthSource = "hdd_smart_attrs"
	setDiskHealthSource(out, "diskHealthSrcHDD", nil, "по SMART-атрибутам", "from SMART attributes")
	applyHDDHealthPercent(in, out)

	if bad, id, msg, reasonRu, reasonEn := ataCriticalFail(in.Attributes); bad {
		setDiskLevel(out, DiskStatusBad)
		out.Reason = msg
		out.ReasonRu, out.ReasonEn = reasonRu, reasonEn
		out.TriggerAttrID = id
		out.TriggerAttrHex = fmt.Sprintf("%02X", id)
		out.LogLine = fmt.Sprintf("disk_status=bad trigger=attr_0x%02X current<=threshold", id)
		log.Printf("[disk-health] %s model=%q reason=%s", out.LogLine, in.Model, reasonRu)
		return
	}
	if caut, id, raw, _, _, _ := ataSectorCaution(in.Attributes); caut {
		setDiskLevel(out, DiskStatusCaution)
		hex := fmt.Sprintf("%02X", id)
		switch id {
		case 197:
			setDiskReason(out, "diskReasonPendingHuman", loc.P("n", raw),
				fmt.Sprintf("%d секторов ожидают переназначения", raw),
				fmt.Sprintf("%d sectors pending reallocation", raw))
		case 5, 196:
			setDiskReason(out, "diskReasonReallocatedHuman", loc.P("n", raw),
				fmt.Sprintf("%d переназначенных секторов", raw),
				fmt.Sprintf("%d reallocated sectors", raw))
		default:
			setDiskReason(out, "diskReasonUncorrectableHuman", loc.P("n", raw),
				fmt.Sprintf("%d неисправимых секторов", raw),
				fmt.Sprintf("%d uncorrectable sectors", raw))
		}
		out.TriggerAttrID = id
		out.TriggerAttrHex = hex
		out.LogLine = fmt.Sprintf("disk_status=caution trigger=attr_0x%02X raw=%d", id, raw)
		log.Printf("[disk-health] %s model=%q reason=%s", out.LogLine, in.Model, out.ReasonRu)
		return
	}
	// Fallback counters if attrs missing IDs but values passed
	if in.Pending > 0 {
		setDiskLevel(out, DiskStatusCaution)
		setDiskReason(out, "diskReasonPendingHuman", loc.P("n", in.Pending),
			fmt.Sprintf("%d секторов ожидают переназначения", in.Pending),
			fmt.Sprintf("%d sectors pending reallocation", in.Pending))
		out.TriggerAttrID = 197
		out.TriggerAttrHex = "C5"
		out.LogLine = fmt.Sprintf("disk_status=caution trigger=pending=%d", in.Pending)
		log.Printf("[disk-health] %s model=%q", out.LogLine, in.Model)
		return
	}
	if in.Reallocated > 0 {
		setDiskLevel(out, DiskStatusCaution)
		setDiskReason(out, "diskReasonReallocatedHuman", loc.P("n", in.Reallocated),
			fmt.Sprintf("%d переназначенных секторов", in.Reallocated),
			fmt.Sprintf("%d reallocated sectors", in.Reallocated))
		out.TriggerAttrID = 5
		out.TriggerAttrHex = "05"
		out.LogLine = fmt.Sprintf("disk_status=caution trigger=reallocated=%d", in.Reallocated)
		log.Printf("[disk-health] %s model=%q", out.LogLine, in.Model)
		return
	}
	if in.Uncorrectable > 0 {
		setDiskLevel(out, DiskStatusCaution)
		setDiskReason(out, "diskReasonUncorrectableHuman", loc.P("n", in.Uncorrectable),
			fmt.Sprintf("%d неисправимых секторов", in.Uncorrectable),
			fmt.Sprintf("%d uncorrectable sectors", in.Uncorrectable))
		out.TriggerAttrID = 198
		out.LogLine = fmt.Sprintf("disk_status=caution trigger=uncorrectable=%d", in.Uncorrectable)
		log.Printf("[disk-health] %s model=%q", out.LogLine, in.Model)
		return
	}
	if !out.HealthPctOK {
		setDiskLevel(out, DiskStatusUnknown)
		setDiskReason(out, "diskReasonHDDAttrsMissing", nil,
			"Недостаточно SMART-атрибутов для оценки здоровья HDD",
			"Not enough SMART attributes to score HDD health")
		out.LogLine = "disk_status=unknown reason=hdd_attrs_missing"
		return
	}
	setGood(out)
	out.LogLine = "disk_status=good kind=hdd"
}

// applyHDDHealthPercent computes a 0-100 health score for HDD from key SMART counters.
func applyHDDHealthPercent(in DiskHealthEvalInput, out *DiskHealthEval) {
	realloc := attrRawOr(in.Attributes, 5, in.Reallocated)
	pending := attrRawOr(in.Attributes, 197, in.Pending)
	uncorr := attrRawOr(in.Attributes, 198, in.Uncorrectable)
	reallocEvt := attrRawOr(in.Attributes, 196, 0)
	spinRetry := attrRawOr(in.Attributes, 10, 0)

	haveKey := false
	for _, id := range []int{5, 197, 198, 196, 10, 7} {
		if _, ok := attrByID(in.Attributes, id); ok {
			haveKey = true
			break
		}
	}
	if !haveKey && realloc == 0 && pending == 0 && uncorr == 0 {
		out.HealthPctOK = false
		setDiskHealthSource(out, "diskHealthSrcHDDMissing", nil,
			"не оценивается - нет ключевых SMART-атрибутов (05/197/198/…)",
			"unavailable - key SMART attributes missing (05/197/198/…)")
		return
	}

	score := 100.0
	score -= minFloat(40, float64(realloc)*5)
	score -= minFloat(40, float64(pending)*10)
	score -= minFloat(40, float64(uncorr)*10)
	score -= minFloat(20, float64(reallocEvt)*5)
	if spinRetry > 0 {
		score -= 15
	}
	if a, ok := attrByID(in.Attributes, 7); ok && a.Current > 0 && a.Current < 90 {
		score -= 10
	}
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}
	out.HealthPercent = score
	out.HealthPctOK = true
	out.HealthSource = "hdd_smart_attrs"
	setDiskHealthSource(out, "diskHealthSrcHDDAttrs", nil,
		"по SMART-атрибутам (05/197/198/196/10/07)",
		"from SMART attributes (05/197/198/196/10/07)")
}

func attrByID(attrs []SmartAttribute, id int) (SmartAttribute, bool) {
	for _, a := range attrs {
		if a.ID == id {
			return a, true
		}
	}
	return SmartAttribute{}, false
}

func attrRawOr(attrs []SmartAttribute, id int, fallback int64) int64 {
	if a, ok := attrByID(attrs, id); ok {
		raw := int64(a.RawValue & 0xFFFF)
		if raw == 0 && a.Raw != "" {
			// RawValue may be unset when mapped from helper string - parse leading digits.
			var n int64
			fmt.Sscanf(a.Raw, "%d", &n)
			if n > 0 {
				return n
			}
		}
		return raw
	}
	if fallback > 0 {
		return fallback
	}
	return 0
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func evalATAGeneric(in DiskHealthEvalInput, out *DiskHealthEval) {
	evalHDDHealth(in, out)
	if out.Level != DiskStatusUnknown {
		setDiskHealthSource(out, "diskHealthSrcATAGeneric", nil,
			"SMART-атрибуты (тип носителя уточняется)",
			"SMART attributes (media kind uncertain)")
	}
}

func setGood(out *DiskHealthEval) {
	setDiskLevel(out, DiskStatusGood)
	out.Reason = loc.Msg{}
	out.ReasonRu, out.ReasonEn = "", ""
}

func ataCriticalFail(attrs []SmartAttribute) (bool, int, loc.Msg, string, string) {
	for _, a := range attrs {
		if !ataCriticalAttrIDs[a.ID] {
			continue
		}
		if a.Threshold <= 0 || a.Current <= 0 {
			continue
		}
		if a.Current <= a.Threshold {
			hex := fmt.Sprintf("%02X", a.ID)
			name := a.NameRu
			if name == "" {
				name = a.Name
			}
			enName := a.Name
			if enName == "" {
				enName = name
			}
			ru := fmt.Sprintf("Атрибут %s (%s): текущее %d ≤ порога %d", hex, name, a.Current, a.Threshold)
			en := fmt.Sprintf("Attribute %s (%s): current %d ≤ threshold %d", hex, enName, a.Current, a.Threshold)
			msg := loc.M("diskReasonAttrThreshold", loc.P("hex", hex, "name", enName, "current", a.Current, "thresh", a.Threshold))
			return true, a.ID, msg, ru, en
		}
	}
	return false, 0, loc.Msg{}, "", ""
}

func ataSectorCaution(attrs []SmartAttribute) (bool, int, int64, loc.Msg, string, string) {
	// Prefer pending (197/C5), then reallocated (5), then uncorrectable (198)
	order := []int{197, 5, 198, 196}
	byID := map[int]SmartAttribute{}
	for _, a := range attrs {
		byID[a.ID] = a
	}
	for _, id := range order {
		a, ok := byID[id]
		if !ok || !ataSectorCautionIDs[id] {
			continue
		}
		raw := int64(a.RawValue & 0xFFFF)
		if raw <= 0 {
			continue
		}
		hex := fmt.Sprintf("%02X", id)
		name := a.NameRu
		if name == "" {
			_, name = smartAttrName(id)
		}
		enName := a.Name
		if enName == "" {
			enName = name
		}
		ru := fmt.Sprintf("Атрибут %s (%s) = %d", hex, name, raw)
		en := fmt.Sprintf("Attribute %s (%s) = %d", hex, enName, raw)
		msg := loc.M("diskReasonAttrRaw", loc.P("hex", hex, "name", enName, "n", raw))
		return true, id, raw, msg, ru, en
	}
	return false, 0, 0, loc.Msg{}, "", ""
}

func findWearLife(model string, attrs []SmartAttribute) (id, life int, ok bool, src string) {
	byID := map[int]SmartAttribute{}
	for _, a := range attrs {
		byID[a.ID] = a
	}
	specs := wearSpecsForModel(model)
	for _, sp := range specs {
		a, exists := byID[sp.ID]
		if !exists || a.Current <= 0 || a.Current > 100 {
			continue
		}
		life := a.Current
		if sp.Invert {
			life = 100 - a.Current
			if life < 0 {
				life = 0
			}
		}
		// Skip inverted 241 if it looks like LBAs written (huge raw) - not a wear %
		if sp.ID == 241 && a.RawValue > 100000 {
			continue
		}
		return sp.ID, life, true, "sata_wear"
	}
	return 0, 0, false, ""
}

func wearSpecsForModel(model string) []wearAttrSpec {
	m := strings.ToLower(model)
	out := append([]wearAttrSpec{}, defaultWearAttrs...)
	for prefix, specs := range vendorWearAttrs {
		if strings.Contains(m, prefix) {
			// Vendor specs first
			merged := append([]wearAttrSpec{}, specs...)
			merged = append(merged, out...)
			return merged
		}
	}
	return out
}

func applyWearPercent(in DiskHealthEvalInput, out *DiskHealthEval) {
	id, life, ok, src := findWearLife(in.Model, in.Attributes)
	if !ok {
		return
	}
	out.HealthPercent = float64(life)
	out.HealthPctOK = true
	out.HealthSource = src
	setDiskHealthSource(out, "diskHealthSrcSATAWear", loc.P("id", id, "hex", fmt.Sprintf("%02X", id)),
		fmt.Sprintf("SATA SSD: атрибут износа ID %d (0x%02X)", id, id),
		fmt.Sprintf("SATA SSD: wear attribute ID %d (0x%02X)", id, id))
}

func classifyDiskKind(media, iface, bus string) string {
	u := strings.ToUpper(media + " " + iface + " " + bus)
	if strings.Contains(u, "NVME") {
		return DiskKindNVMe
	}
	if strings.Contains(u, "HDD") || strings.Contains(u, "FIXED HARD") || strings.Contains(u, "ROTATIONAL") {
		return DiskKindHDD
	}
	if strings.Contains(u, "SSD") || strings.Contains(u, "SOLID") {
		return DiskKindSATASSD
	}
	return DiskKindUnknown
}

func classifyBusInterface(bus, iface string) string {
	u := strings.ToUpper(bus + " " + iface)
	switch {
	case strings.Contains(u, "USB"):
		return "USB"
	case strings.Contains(u, "NVME"):
		return "NVMe"
	case strings.Contains(u, "SATA") || strings.Contains(u, "ATA") || strings.Contains(u, "IDE"):
		return "SATA"
	case strings.Contains(u, "SCSI"):
		return "SCSI"
	default:
		if strings.TrimSpace(bus) != "" {
			return bus
		}
		return iface
	}
}

func humanPowerOn(hours float64, ru bool) string {
	if hours <= 0 {
		return ""
	}
	days := hours / 24
	if days < 45 {
		if ru {
			return fmt.Sprintf("≈ %.0f дн", days)
		}
		return fmt.Sprintf("≈ %.0f d", days)
	}
	months := days / 30.44
	if months < 18 {
		if ru {
			return fmt.Sprintf("≈ %.0f мес", months)
		}
		return fmt.Sprintf("≈ %.0f mo", months)
	}
	years := days / 365.25
	if ru {
		return fmt.Sprintf("≈ %.1f года", years)
	}
	return fmt.Sprintf("≈ %.1f y", years)
}

func humanPowerOnMsg(hours float64) loc.Msg {
	if hours <= 0 {
		return loc.Msg{}
	}
	days := hours / 24
	if days < 45 {
		return loc.M("diskPowerOnDays", loc.P("n", int(days+0.5)))
	}
	months := days / 30.44
	if months < 18 {
		return loc.M("diskPowerOnMonths", loc.P("n", int(months+0.5)))
	}
	years := days / 365.25
	return loc.M("diskPowerOnYears", loc.P("n", float64(int(years*10+0.5))/10))
}
