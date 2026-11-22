<p align="center">
  <img src="assets/showcase/logo.png" alt="PulseCore" width="96" height="96" />
</p>

<h1 align="center">PulseCore v1.0.0</h1>

<p align="center">
  <strong>See heat. Kill lag. Play smooth.</strong><br/>
  <em>Видишь нагрев. Убираешь лаги. Играешь плавно.</em>
</p>

<p align="center">
  <a href="https://github.com/robbled17/pulsecore/releases/tag/v1.0.0"><img alt="Release" src="https://img.shields.io/badge/release-v1.0.0-0b0f19?style=flat-square" /></a>
  <a href="https://github.com/robbled17/pulsecore/releases"><img alt="Downloads" src="https://img.shields.io/github/downloads/robbled17/pulsecore/total?style=flat-square&label=downloads" /></a>
  <a href="#install"><img alt="Windows" src="https://img.shields.io/badge/Windows-10%20%2F%2011-0078D4?style=flat-square" /></a>
  <a href="#stack"><img alt="Go + Wails" src="https://img.shields.io/badge/Go%20%2B%20Wails-v2-00ADD8?style=flat-square" /></a>
  <a href="LICENSE"><img alt="MIT" src="https://img.shields.io/badge/license-MIT-teal?style=flat-square" /></a>
</p>

<p align="center">
  <img src="assets/showcase/demo-monitoring.gif" alt="PulseCore live monitoring demo" width="1200" />
</p>

<p align="center">
  <a href="https://github.com/robbled17/pulsecore/releases/latest/download/pulsecore.exe"><img alt="Download pulsecore.exe" src="https://img.shields.io/badge/Download%20pulsecore.exe-v1.0.0-6ea8ff?style=for-the-badge&logo=windows&logoColor=white" /></a>
</p>

<p align="center">
  <a href="https://robbled17.github.io/pulsecore/">Product site</a>
  ·
  <a href="https://github.com/robbled17/pulsecore/releases/latest">Releases</a>
  ·
  <a href="https://github.com/robbled17/pulsecore/releases/latest/download/SHA256SUMS.txt">SHA256SUMS.txt</a>
</p>

PulseCore is a lightweight Windows hardware desk app for gamers and power users: live CPU / GPU / RAM / disk metrics, lag spotting, Eco Mode for background vampires, power profiles, S.M.A.R.T., Mini-HUD, Upgrade Advisor, Smart Uninstall, and GamePulse FPS predictions. No heavy SDKs. No cloud account. No telemetry.

## Overview

Live metrics, sparklines, and actionable alerts for thermal, RAM, disk, and lag.

![Overview](assets/showcase/overview.png)

## Lag Spotter + Eco Mode

Per-core spikes, stutter log, network diagnostics, sleep blockers, and soft Eco limits for hungry background processes (safe denylist, undo supported).

![Lags and Eco Mode](assets/showcase/lags-eco.png)

## Hardware

Physical CPU cores labeled 1..N with SMT thread bars, SSD S.M.A.R.T. accordion, discrete GPU preference, RAM channels, and battery wear.

![Hardware](assets/showcase/hardware.png)

## Upgrade Advisor + Smart Uninstaller

Laptop vs desktop hard-split upgrade advice, process precheck, residual scan, install tracing, and Temp/Cache whitelist clean.

![Upgrade Advisor and Uninstaller](assets/showcase/upgrade-uninstaller.png)

<p align="center">
  <img src="assets/showcase/demo-uninstaller.gif" alt="Smart Uninstaller demo" width="1200" />
</p>

## GamePulse

Catalog search plus FPS predictor across Low / Med / High / Ultra with a broad hardware fallback matrix.

![GamePulse](assets/showcase/gamepulse.png)

## Safety and architecture

- Cleaner allowlist only (Temp / Cache style paths); protected OS / security processes stay untouched
- Dangerous uninstall targets are blocked; registry edits keep `.reg` backups before write
- Runs locally on your PC; no telemetry, no account, no cloud sync of personal data
- Windows elevation via embedded `requireAdministrator` when ACPI temps and full SMART need it

## Install

1. Download [`pulsecore.exe`](https://github.com/robbled17/pulsecore/releases/latest/download/pulsecore.exe)
2. Optionally verify SHA256 from [`SHA256SUMS.txt`](https://github.com/robbled17/pulsecore/releases/latest/download/SHA256SUMS.txt)
3. Run `pulsecore.exe` on **Windows 10/11** with **WebView2** (preinstalled on modern Windows)

## Build from source

For developers who want to build locally:

```bash
# Prerequisites: Go 1.22+, Node 18+, Wails CLI v2
go install github.com/wailsapp/wails/v2/cmd/wails@latest

cd frontend && npm install && cd ..
wails build -clean -platform windows/amd64 -ldflags "-s -w"
```

Binary: `build/bin/pulsecore.exe`.

## Stack

- **Go** backend (WMI / PDH / process / power) with graceful degradation
- **Wails v2** desktop shell + WebView2
- **Vanilla JS** frontend (Vite), multi-language UI (RU default)

## Author

Robert Sharafutdinov ([`robbled17`](https://github.com/robbled17))

## License

MIT © Robert Sharafutdinov
