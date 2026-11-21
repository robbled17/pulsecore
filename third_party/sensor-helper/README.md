# PulseCore sensor helper

Sidecar binary `pulsecore-sensor-helper.exe` is embedded into `pulsecore.exe` and
extracted on launch to `%LOCALAPPDATA%\PulseCore\bin\` when missing or when
`VERSION` changes.

## Build (Windows CI / local cross-compile)

```bash
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" \
  -o third_party/sensor-helper/pulsecore-sensor-helper.exe ./cmd/sensor-helper
```

The helper reads ACPI thermal zones and (when present) `nvidia-smi` for GPU
temperature / utilization. SMART stays in the main app via Storage cmdlets and
is not polled every metrics tick.
