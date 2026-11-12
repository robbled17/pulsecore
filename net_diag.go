package main

import (
	"net"
	"strings"
	"time"
)

var publicDNSTargets = []string{
	"1.1.1.1:53",
	"8.8.8.8:53",
	"77.88.8.8:53",
}

// DiagnoseLagOrigin probes the real default gateway and public DNS via TCP dials
// (locale-independent; no ping.exe parsing).
func (a *App) DiagnoseLagOrigin() LagDiagnosis {
	diag := LagDiagnosis{
		OK:         true,
		PublicHost: "1.1.1.1",
	}

	gw := resolveDefaultGatewayIPv4()
	diag.GatewayHost = gw
	if gw != "" {
		diag.GatewayMs = probeGatewayLatencyMs(gw)
	} else {
		diag.GatewayMs = -1
	}

	publicHost, publicMs := probeInternetLatencyMs()
	if publicHost != "" {
		diag.PublicHost = publicHost
	}
	diag.PublicMs = publicMs

	internetOK := publicMs >= 0
	gatewayOK := diag.GatewayMs >= 0

	if !internetOK {
		diag.OK = false
		diag.Level = "offline"
		diag.VerdictKey = "netVerdictOffline"
		diag.Verdict = diag.VerdictKey
		diag.PublicBadge = "netBadgeFail"
		diag.PublicBadgeOK = false
		diag.LocalJitterMs = -1
		diag.JitterBadge = "netBadgeUnavailable"
		diag.JitterBadgeOK = false
		if gatewayOK {
			diag.GatewayBadgeOK = diag.GatewayMs <= 15
			if diag.GatewayBadgeOK {
				diag.GatewayBadge = "netBadgeIdeal"
			} else if diag.GatewayMs <= 50 {
				diag.GatewayBadge = "netBadgeWeak"
			} else {
				diag.GatewayBadge = "netBadgeWeak"
			}
		} else {
			diag.GatewayBadge = "netBadgeFail"
			diag.GatewayBadgeOK = false
		}
		return diag
	}

	// Internet reachable - measure local scheduler jitter for PC-side verdicts.
	diag.LocalJitterMs = measureLocalJitterMs()

	diag.GatewayBadgeOK = gatewayOK && diag.GatewayMs <= 15
	if !gatewayOK {
		diag.GatewayBadge = "netBadgeFail"
		diag.GatewayBadgeOK = false
	} else if diag.GatewayMs <= 5 {
		diag.GatewayBadge = "netBadgeIdeal"
		diag.GatewayBadgeOK = true
	} else if diag.GatewayMs <= 15 {
		diag.GatewayBadge = "netBadgeIdeal"
		diag.GatewayBadgeOK = true
	} else if diag.GatewayMs <= 50 {
		diag.GatewayBadge = "netBadgeWeak"
		diag.GatewayBadgeOK = false
	} else {
		diag.GatewayBadge = "netBadgeWeak"
		diag.GatewayBadgeOK = false
	}

	diag.PublicBadgeOK = publicMs >= 0 && publicMs <= 40
	if publicMs <= 40 {
		diag.PublicBadge = "netBadgeFast"
		diag.PublicBadgeOK = true
	} else if publicMs <= 100 {
		diag.PublicBadge = "netBadgeSlow"
		diag.PublicBadgeOK = false
	} else {
		diag.PublicBadge = "netBadgeSlow"
		diag.PublicBadgeOK = false
	}

	diag.JitterBadgeOK = diag.LocalJitterMs >= 0 && diag.LocalJitterMs <= 20
	if diag.JitterBadgeOK {
		diag.JitterBadge = "netBadgeStable"
	} else {
		diag.JitterBadge = "netBadgeUnstable"
	}

	switch {
	case gatewayOK && diag.GatewayMs > 50:
		diag.Level = "wifi"
		diag.VerdictKey = "netVerdictWifi"
	case gatewayOK && diag.GatewayMs < 8 && publicMs > 120:
		diag.Level = "provider"
		diag.VerdictKey = "netVerdictISP"
	case !gatewayOK && internetOK:
		// Gateway probe failed (closed ports) but internet works - still OK overall.
		diag.Level = "ok"
		diag.VerdictKey = "netVerdictOK"
	case diag.LocalJitterMs > 150 && publicMs < 80:
		diag.Level = "pc"
		diag.VerdictKey = "netVerdictPC"
	default:
		diag.Level = "ok"
		diag.VerdictKey = "netVerdictOK"
	}
	diag.Verdict = diag.VerdictKey
	return diag
}

func resolveDefaultGatewayIPv4() string {
	if gw := gatewayFromPowerShell(); gw != "" {
		return gw
	}
	if gw := gatewayFromRoutePrint(); gw != "" {
		return gw
	}
	return ""
}

func gatewayFromPowerShell() string {
	cmds := []string{
		`(Get-NetRoute -DestinationPrefix '0.0.0.0/0' | Where-Object { $_.NextHop -ne '0.0.0.0' } | Sort-Object RouteMetric,InterfaceMetric | Select-Object -First 1 -ExpandProperty NextHop)`,
		`(Get-NetIPConfiguration | Where-Object { $_.IPv4DefaultGateway -ne $null -and $_.NetAdapter.Status -eq 'Up' } | Select-Object -First 1).IPv4DefaultGateway.NextHop`,
	}
	for _, cmd := range cmds {
		out, err := runHiddenOutput("powershell", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", cmd)
		if err != nil {
			continue
		}
		gw := strings.TrimSpace(string(out))
		gw = strings.Trim(gw, "\"'` \r\n\t")
		if ip := net.ParseIP(gw); ip != nil && ip.To4() != nil && !ip.IsUnspecified() {
			return ip.String()
		}
	}
	return ""
}

func gatewayFromRoutePrint() string {
	out, err := runHiddenOutput("route", "print", "0.0.0.0")
	if err != nil && len(out) == 0 {
		return ""
	}
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		if fields[0] != "0.0.0.0" {
			continue
		}
		// route print: Network Destination | Netmask | Gateway | Interface | Metric
		gw := fields[2]
		if ip := net.ParseIP(gw); ip != nil && ip.To4() != nil && !ip.IsUnspecified() {
			return ip.String()
		}
	}
	return ""
}

func probeGatewayLatencyMs(gatewayIP string) float64 {
	if net.ParseIP(gatewayIP) == nil {
		return -1
	}
	ports := []string{"80", "443", "53", "8080"}
	best := float64(-1)
	for _, port := range ports {
		ms := tcpDialLatencyMs(net.JoinHostPort(gatewayIP, port), 800*time.Millisecond)
		if ms < 0 {
			continue
		}
		if best < 0 || ms < best {
			best = ms
		}
		// First success is enough for LAN - keep the best of a couple ports.
		if best <= 5 {
			return best
		}
	}
	return best
}

func probeInternetLatencyMs() (host string, ms float64) {
	best := float64(-1)
	bestHost := ""
	for _, target := range publicDNSTargets {
		lat := tcpDialLatencyMs(target, 1200*time.Millisecond)
		if lat < 0 {
			continue
		}
		h, _, _ := net.SplitHostPort(target)
		if best < 0 || lat < best {
			best = lat
			bestHost = h
		}
		// First successful DNS dial is enough for a reliable public RTT.
		break
	}
	if best < 0 {
		return "", -1
	}
	return bestHost, best
}

func tcpDialLatencyMs(address string, timeout time.Duration) float64 {
	start := time.Now()
	conn, err := net.DialTimeout("tcp", address, timeout)
	if err != nil {
		return -1
	}
	_ = conn.Close()
	ms := float64(time.Since(start).Milliseconds())
	if ms < 1 {
		ms = 1
	}
	return ms
}

func measureLocalJitterMs() float64 {
	const quantum = 8 * time.Millisecond
	var maxOver time.Duration
	samples := make([]time.Duration, 0, 12)
	for i := 0; i < 12; i++ {
		start := time.Now()
		time.Sleep(quantum)
		over := time.Since(start) - quantum
		if over < 0 {
			over = 0
		}
		samples = append(samples, over)
		if over > maxOver {
			maxOver = over
		}
	}
	return float64(maxOver) / float64(time.Millisecond)
}
