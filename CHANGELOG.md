# Changelog

## 1.0.0

### Added
- Public showcase storefront README with module screenshots and demo GIFs under `assets/showcase/`
- Landing: dark `theme-color` / `color-scheme` to stop iOS overscroll white flash
- Landing: continuous rAF scroll for Dynamics CPU/GPU/RAM chart (sparkline parity)

### Changed
- Official public product version set to **1.0.0** across Go, PE VersionInfo, Wails, frontend, and docs
- Download CTA points at `releases/latest/download/pulsecore.exe`
- Author attribution cleaned to Robert Sharafutdinov (`robbled17`)

### Fixed (silent audit)
- Production builds ignore `setActiveTab('dev')` and clear any Dev Mode active state at boot
- Release workflow sets `overwrite_files` / `make_latest` so a force-moved `v1.0.0` tag replaces assets cleanly


## 1.6.5

### Added
- Windows manifest `requireAdministrator` embedded into `pulsecore.exe`
- Startup self-elevation via `ShellExecuteW` `runas` when the process is not elevated
- Release asset `SHA256SUMS.txt` next to `pulsecore.exe`
- Optional Authenticode `signtool` step when repo secret `SIGN_CERT_PFX` is set

### Changed
- PE VersionInfo: ProductName PulseCore, FileDescription PulseCore System Monitor & Utility, CompanyName robbled17, version 1.6.5
- Release builds explicitly avoid UPX and other packers


## 1.6.4

### Added
- Update checker: GitHub Releases latest with About/Settings check and launch banner when newer
- Version and updates block at the bottom of Settings

### Fixed
- Horizontal overflow on Overview, Lags, Hardware, Upgrade, Uninstall, GamePulse, and Settings at common laptop widths


## 1.6.3

### Fixed
- Windows build: remove duplicate `isProcessElevated` so `go test` / `go vet` compile again


## 1.6.2

### Added
- Embedded sensor helper extracted to `%LOCALAPPDATA%\PulseCore\bin` when missing or version-changed (single-file `pulsecore.exe`)
- Unavailable sensor tip with restart-as-administrator action (7 langs)
- Hardware/docs cores labeled 1..N (never Core 0)

### Fixed
- Poll path caches helper/nvidia-smi so heavy WMI/PowerShell is not run every tick
- Large cards/lists no longer use heavy `backdrop-filter` blur on weak WebView2 laptops
- Charts throttled to ~24 FPS and fully paused when tray-minimized or inactive
- Landing `#features` dense even grid (3/2/1) with equal row heights and no span holes
- Dev Mode hidden in production `wails build` (kept for `wails dev` / Vite DEV only)

### Needs hardware
- Live admin elevation for ACPI/SMART on real Windows notebooks
- Dual-GPU laptop temp preference (discrete vs iGPU) on Optimus/AMD switchable graphics
- Tray minimize chart pause on a real laptop session


## 1.6.1

### Fixed
- Self-diagnostics modal: no duplicate Working line under badges; facts use translated labels; MB shown as GB where useful; check time as local datetime

## 1.6.0

### Added
- About: Check sensors self-diagnostics modal (CPU/GPU/RAM/drives+SMART/registry+cleaner/run mode) with copy report
- Single instance: Windows named mutex `Global\PulseCoreSingleInstance` restores existing window on second launch
- Settings: global Mini-HUD hotkey (default Ctrl+Shift+M), persisted, fail-soft registration
- Frontend smoke test (Playwright + mock harness) wired into PR Windows CI

### Fixed
- Diagnostics never invent sensor numbers; unavailable / fallback / admin statuses only
- Non-Windows / fixture path returns graceful unavailable statuses without panic

### Needs hardware
- Live dual-launch while tray-minimized or Mini-HUD floating
- Live Mini-HUD hotkey inside exclusive fullscreen games

## 1.5.1

### Fixed
- Lags: core-peak capsule padding so timestamp / process / PID stays inside the card
- Lags: Network and Sleep blockers as a symmetric 2-column grid with sleep `i` tooltip (7 langs)
- GamePulse: full catalog in a scrollable responsive grid; instant RU+EN search; Steam 2:3 poster cover
- Settings: unified font family/weight/size; Reset profile moved to power-profile header row
- Landing Dynamics: live CPU/GPU/RAM legend with percentages and 100%/50%/0% scale grid
- Landing Upgrade pitfalls: inherit font and card-style markers (no tiny yellow type)
- Landing Uninstall demo: styled capsule buttons for Kill & remove / Replay demo

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
