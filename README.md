# PulseCore

Local Windows performance diagnosis and optimization assistant (Go + Wails v2 + Vanilla JS).

Modernized on branch `pulseee/roadmap` (Stages 1–12): Lag Black Box, Eco before/after, weekly local stats, tray live tooltip, Game Mode sessions, Process Radar / Leak Watch, System Health, Startup Analyzer + autostart, Network Monitor, Diagnostic Report, Intelligence profiles, and polish (config persistence, emit gating when hidden).

## Features (high level)

- Live CPU / GPU / RAM / disk / battery metrics
- «Почему лагнуло?» Black Box with honest UNKNOWN verdicts + timeline
- Eco Mode (EcoQoS) with before/after deltas
- GamePulse FPS predictor **and** separate Game Mode session detector
- Tray: close-to-tray, live CPU/RAM tooltip, Game Mode toggle
- Local-only stats/config under OS app-data (`PulseCore/`)

## Build

```bash
cd frontend && npm install && npm run build && cd ..
wails build
```

Binary output: `build/bin/pulsecore.exe`

Cross-compile check (no run):

```bash
GOOS=windows GOARCH=amd64 go build -o pulsecore.exe .
```

## Requirements

- Windows 10/11 with WebView2
- Go 1.22+ (repo uses 1.26)
- Node.js 18+
- Wails CLI v2

## Privacy

No telemetry. Diagnostic reports and `stats.json` stay on disk.
