package main

import (
	"net"
	"time"

	psnet "github.com/shirou/gopsutil/v3/net"
)

// NetworkMonitorSnapshot is a lightweight local network view (no PowerShell hot loop).
type NetworkMonitorSnapshot struct {
	OK            bool    `json:"ok"`
	BytesSent     uint64  `json:"bytesSent"`
	BytesRecv     uint64  `json:"bytesRecv"`
	SendMBps      float64 `json:"sendMBps"`
	RecvMBps      float64 `json:"recvMBps"`
	InterfacesUp  int     `json:"interfacesUp"`
	PrimaryIface  string  `json:"primaryIface"`
	GatewayProbeMs float64 `json:"gatewayProbeMs"`
	GatewayOK     bool    `json:"gatewayOK"`
	NoteKey       string  `json:"noteKey"`
	Level         string  `json:"level"` // ok | warn | bad
}

func (a *App) sampleNetCounters() (sent, recv uint64, iface string, up int) {
	stats, err := psnet.IOCounters(true)
	if err != nil {
		return 0, 0, "", 0
	}
	best := ""
	var bestTotal uint64
	for _, s := range stats {
		if s.Name == "lo" || s.Name == "Loopback" {
			continue
		}
		up++
		total := s.BytesSent + s.BytesRecv
		sent += s.BytesSent
		recv += s.BytesRecv
		if total >= bestTotal {
			bestTotal = total
			best = s.Name
		}
	}
	return sent, recv, best, up
}

func probeGatewayRTT() (float64, bool) {
	// Prefer default gateway discovery via UDP dial trick (no admin).
	conn, err := net.DialTimeout("udp", "8.8.8.8:80", 800*time.Millisecond)
	if err != nil {
		return 0, false
	}
	local := conn.LocalAddr().String()
	_ = conn.Close()
	host, _, _ := net.SplitHostPort(local)
	if host == "" {
		host = "127.0.0.1"
	}
	// Soft TCP connect to common gateway .1 of /24 if possible.
	ip := net.ParseIP(host)
	gw := ""
	if ip != nil && ip.To4() != nil {
		v4 := ip.To4()
		gw = net.IPv4(v4[0], v4[1], v4[2], 1).String()
	}
	if gw == "" {
		return 0, false
	}
	start := time.Now()
	c, err := net.DialTimeout("tcp", gw+":80", 900*time.Millisecond)
	ms := float64(time.Since(start)) / float64(time.Millisecond)
	if err != nil {
		// Timeout still gives an RTT-ish upper bound; mark not OK.
		return ms, false
	}
	_ = c.Close()
	return ms, true
}

// GetNetworkMonitor returns counters + soft gateway probe (on-demand, not in metrics tick).
func (a *App) GetNetworkMonitor() NetworkMonitorSnapshot {
	sent, recv, iface, up := a.sampleNetCounters()
	now := time.Now()
	snap := NetworkMonitorSnapshot{
		OK: true, BytesSent: sent, BytesRecv: recv,
		PrimaryIface: iface, InterfacesUp: up, NoteKey: "netMonOK", Level: "ok",
	}
	a.mu.Lock()
	if !a.netPrevAt.IsZero() {
		sec := now.Sub(a.netPrevAt).Seconds()
		if sec > 0 {
			if sent >= a.netPrevSent {
				snap.SendMBps = float64(sent-a.netPrevSent) / (1024 * 1024) / sec
			}
			if recv >= a.netPrevRecv {
				snap.RecvMBps = float64(recv-a.netPrevRecv) / (1024 * 1024) / sec
			}
		}
	}
	a.netPrevSent = sent
	a.netPrevRecv = recv
	a.netPrevAt = now
	a.mu.Unlock()

	ms, ok := probeGatewayRTT()
	snap.GatewayProbeMs = ms
	snap.GatewayOK = ok
	if !ok {
		snap.Level = "warn"
		snap.NoteKey = "netMonGatewayWarn"
	}
	if up == 0 {
		snap.Level = "bad"
		snap.NoteKey = "netMonDown"
		snap.OK = false
	}
	return snap
}
