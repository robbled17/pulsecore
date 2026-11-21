# Changelog

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
