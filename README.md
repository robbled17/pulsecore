# PulseCore v2.0

Lightweight Windows hardware intelligence desk app built with **Go + Wails v2 + Vanilla JS**.

## Features

1. **Lag Spotter** — micro-freeze detection with culprit process + core ID  
2. **Ghost Hunter** — background vampire index with soft freeze (`NtSuspendProcess`)  
3. **Smart Turbo & Thermal Control** — Silent / Balanced / Full Throttle via `powercfg`  
4. **VRAM & 3D Engine Inspector** — dedicated/shared VRAM without heavy SDKs  
5. **Floating Mini-HUD** — always-on-top 240×55 overlay  
6. **Emergency Hardware Alarm** — thermal toast + `MessageBeep`

## Build

```bash
wails dev
wails build
```

Binary output: `build/bin/pulsecore.exe`

## Requirements

- Windows 10/11 with WebView2
- Go 1.22+
- Node.js 18+
- Wails CLI v2
