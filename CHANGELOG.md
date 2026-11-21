# Changelog

## 1.5.0

### Fixed
- Windows CI: `go test -v ./...` runs on `windows-latest` before building `pulsecore.exe` (release + PR workflows)
- Hardware tab: SSD / disk S.M.A.R.T. accordion with per-drive expand (wear %, temp, TBW, power-on, status)
- Sensors: Mini-HUD and Hardware temp chips use translated unavailable status when ACPI is missing
- Desktop battery wear field shows N/A instead of a Russian ATX caption in non-RU UIs
- App icon: tighter transparent crop so tray/taskbar icon has no empty white-looking margins
- Cleaner: forbid LOCALAPPDATA\Packages (and WindowsApps / Start Menu package trees) even when LOCALAPPDATA is redirected
- SMART: never invent 50%/70%/100% wear when the reliability counter is missing
- Dev Mode badge stays session-only (not restored from localStorage)
- Landing Upgrade demo recommendations follow language (RU/EN; other langs use EN)
- i18n: translate insightAct / fpsWarn / mhShowTemps leftovers for kk/tt/de/ja/zh

### Needs hardware
- Live SMBIOS chassis + batteryPresent on real Windows notebooks
- Live RAM slot WMI vs Hardware/Upgrade free-slot sync on soldered SO-DIMM mixes
- Live S.M.A.R.T. PowerShell probes on NVMe / SATA / USB bridges
- Eco Mode soft-skip against live Defender processes (guards unit-tested)

### Unverified
- Junction / reparse-point deletes on NTFS (unit-tested with EvalSymlinks on Unix temp dirs)
- End-to-end TerminateProcess against live third-party apps (guards unit-tested; no live kills in CI)

## 1.4.1

### Fixed
- Upgrade Advisor: hard laptop split - never recommend desktop CPU/GPU; RAM only when free slots > 0; battery / mobile CPU-GPU heuristics when SMBIOS chassis is wrong
- RAM free-slot count shared between Hardware and Upgrade via `normalizeRAMSlots`
- Lags / Eco Mode: filter Windows Defender and critical OS (MsMpEng, NisSrv, SecurityHealthService, SearchIndexer, System, svchost, …); never suggest Eco on them
- Quick Clean UI restored at top of Uninstall tab with dry-run size preview before delete (whitelist unchanged)
- Battery wear field shows percent only (`8%`), not duplicated `Износ 8%`
- GitHub Release attaches bare `pulsecore.exe` alongside the zip for direct download

## 1.4.0

### Fixed
- Disk cleaner: whitelist-only deletes, skip open/recent files, symlink/junction escape guards, path normalize (`\\?\`, UNC, env, `..`) before every delete, dry-run preview API
- Uninstaller / Eco: all TerminateProcess paths fail-closed via `IsProtectedSystemTarget`; never kill PulseCore/parents/WebView; InstallLocation depth ≥ 2; trace cleanup timeouts/depth/cancel; no system registry keys in diffs
- Registry: `.reg` backup required or cancel; SYSTEM / Windows / Defender / Explorer (+ WOW6432Node) protected
- WMI / WinAPI sensors wrapped with `recover` + timeouts; `nvidia-smi` resolved to absolute paths only
- Sensors: no fake full-battery / zero-temp inventing; translated unavailable statuses; mock fixtures for laptop dual GPU, SMT / no-SMT CPUs, multi-disk SMART
- UI: Esc closes modals; Mini-HUD unchanged; Dev badge stays hidden in production until About 7-click; brand icon border cleanup
- i18n: full key parity across ru/en/kk/tt/de/ja/zh; no em/en dashes in user-visible locale strings; dash autotest

### Fixed-needs-hardware
- Live ACPI temp / SMART / dual-GPU Optimus switching / DPI + dual-monitor layout on real Windows machines
- PDH counters (not used; GPU fallback is process-heuristic)

### Unverified
- Real junction reparse-point deletes on NTFS (unit-tested with EvalSymlinks on Unix temp dirs)
- End-to-end TerminateProcess against live third-party apps (guards unit-tested; no live kills in CI)

## 1.3.2

- Landing uninstaller animation and smooth charts (prior release)

## 1.3.1

- Safe cleaner OS guardrails hotfix
