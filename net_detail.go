package main

import (
	"context"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// NetworkDetail is the expanded Lags → Network accordion payload (real data only).
type NetworkDetail struct {
	OK              bool     `json:"ok"`
	Checked         bool     `json:"checked"`
	Cached          bool     `json:"cached"`
	Cancelled       bool     `json:"cancelled"`
	VerdictKey      string   `json:"verdictKey"`
	Level           string   `json:"level"`
	SummaryPingMs   float64  `json:"summaryPingMs"`
	ConnType        string   `json:"connType,omitempty"`
	LocalIP         string   `json:"localIP,omitempty"`
	PublicIP        string   `json:"publicIP,omitempty"`
	Gateway         string   `json:"gateway,omitempty"`
	GatewayMs       float64 `json:"gatewayMs"`
	DNSServers      []string `json:"dnsServers,omitempty"`
	DNSMs           float64 `json:"dnsMs"`
	InternetHost    string  `json:"internetHost,omitempty"`
	InternetMs      float64 `json:"internetMs"`
	AdapterName     string  `json:"adapterName,omitempty"`
	AdapterStatus   string  `json:"adapterStatus,omitempty"`
	LinkMbps        float64 `json:"linkMbps,omitempty"`
	LinkSpeedLabel  string  `json:"linkSpeedLabel,omitempty"`
	WifiSignal      int     `json:"wifiSignal,omitempty"`
	WifiBand        string  `json:"wifiBand,omitempty"`
	WifiSSID        string  `json:"wifiSSID,omitempty"`
}

type netCheckCache struct {
	mu     sync.Mutex
	detail NetworkDetail
	at     time.Time
	cancel context.CancelFunc
}

func (a *App) ensureNetCheck() *netCheckCache {
	if a.netCheck == nil {
		a.netCheck = &netCheckCache{}
	}
	return a.netCheck
}

// CancelNetworkCheck aborts an in-flight detailed network probe.
func (a *App) CancelNetworkCheck() {
	c := a.ensureNetCheck()
	c.mu.Lock()
	cancel := c.cancel
	c.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// CheckNetworkDetailed runs on-demand ICMP probes with a short cache (no polling).
func (a *App) CheckNetworkDetailed(force bool) NetworkDetail {
	c := a.ensureNetCheck()
	c.mu.Lock()
	if !force && !c.at.IsZero() && time.Since(c.at) < 45*time.Second {
		out := c.detail
		out.Cached = true
		c.mu.Unlock()
		return out
	}
	if c.cancel != nil {
		c.cancel()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	c.cancel = cancel
	c.mu.Unlock()
	defer cancel()

	out := collectNetworkDetail(ctx)
	c.mu.Lock()
	c.detail = out
	c.at = time.Now()
	c.mu.Unlock()
	return out
}

func collectNetworkDetail(ctx context.Context) NetworkDetail {
	out := NetworkDetail{
		Checked:    true,
		GatewayMs:  -1,
		DNSMs:      -1,
		InternetMs: -1,
	}
	select {
	case <-ctx.Done():
		out.Cancelled = true
		out.VerdictKey = "netVerdictOffline"
		out.Level = "offline"
		return out
	default:
	}

	info := probeNetworkAdapterInfo()
	out.ConnType = info.ConnType
	out.LocalIP = info.LocalIP
	out.PublicIP = resolvePublicIP()
	out.Gateway = info.Gateway
	out.DNSServers = info.DNS
	out.AdapterName = info.AdapterName
	out.AdapterStatus = info.AdapterStatus
	out.LinkMbps = info.LinkMbps
	out.LinkSpeedLabel = info.LinkSpeedLabel
	out.WifiSignal = info.WifiSignal
	out.WifiBand = info.WifiBand
	out.WifiSSID = info.WifiSSID

	gw := out.Gateway
	if gw == "" {
		gw = resolveDefaultGatewayIPv4()
		out.Gateway = gw
	}
	if gw != "" && ctx.Err() == nil {
		out.GatewayMs = icmpPingIPv4(gw, 900*time.Millisecond)
	}

	dnsTarget := ""
	if len(out.DNSServers) > 0 {
		dnsTarget = out.DNSServers[0]
	}
	if dnsTarget == "" {
		dnsTarget = "1.1.1.1"
	}
	if ctx.Err() == nil {
		out.DNSMs = icmpPingIPv4(dnsTarget, 900*time.Millisecond)
	}

	inet := "1.1.1.1"
	out.InternetHost = inet
	if ctx.Err() == nil {
		out.InternetMs = icmpPingIPv4(inet, 1200*time.Millisecond)
	}

	if ctx.Err() != nil {
		out.Cancelled = true
	}

	ping := out.InternetMs
	if ping < 0 {
		ping = out.GatewayMs
	}
	out.SummaryPingMs = ping

	switch {
	case out.InternetMs < 0 && out.GatewayMs < 0:
		out.OK = false
		out.Level = "offline"
		out.VerdictKey = "netVerdictOffline"
	case out.GatewayMs >= 0 && out.GatewayMs > 50:
		out.OK = false
		out.Level = "wifi"
		out.VerdictKey = "netVerdictWifi"
	case out.GatewayMs >= 0 && out.GatewayMs < 8 && out.InternetMs > 120:
		out.OK = false
		out.Level = "provider"
		out.VerdictKey = "netVerdictISP"
	default:
		out.OK = out.InternetMs >= 0 || out.GatewayMs >= 0
		out.Level = "ok"
		out.VerdictKey = "netVerdictOK"
	}
	return out
}

type adapterInfo struct {
	ConnType      string
	LocalIP       string
	Gateway       string
	DNS           []string
	AdapterName   string
	AdapterStatus string
	LinkMbps       float64
	LinkSpeedLabel string
	WifiSignal     int
	WifiBand       string
	WifiSSID       string
}

// maskIPForReport redacts host parts for support exports.
func maskIPForReport(ip string) string {
	ip = strings.TrimSpace(ip)
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return ip
	}
	if v4 := parsed.To4(); v4 != nil {
		return net.IPv4(v4[0], v4[1], 0, 0).String()
	}
	// IPv6: keep first hextet only.
	parts := strings.Split(parsed.String(), ":")
	if len(parts) == 0 {
		return "***"
	}
	return parts[0] + ":***"
}

func maskSSIDForReport(ssid string) string {
	ssid = strings.TrimSpace(ssid)
	if ssid == "" {
		return ""
	}
	if len(ssid) <= 2 {
		return "**"
	}
	return string([]rune(ssid)[:1]) + "***"
}

func resolvePublicIP() string {
	client := &http.Client{Timeout: 2 * time.Second}
	for _, endpoint := range []string{"https://api.ipify.org", "https://icanhazip.com"} {
		resp, err := client.Get(endpoint)
		if err == nil && resp.StatusCode == http.StatusOK {
			b, _ := io.ReadAll(io.LimitReader(resp.Body, 64))
			_ = resp.Body.Close()
			ipStr := strings.TrimSpace(string(b))
			if net.ParseIP(ipStr) != nil {
				return ipStr
			}
		}
		if resp != nil && resp.Body != nil {
			_ = resp.Body.Close()
		}
	}
	return ""
}
