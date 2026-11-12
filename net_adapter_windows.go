//go:build windows

package main

import (
	"net"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

var virtualAdapterNeedles = []string{
	"tun", "tap", "wg", "wireguard", "radmin", "virtual", "bluetooth",
	"vethernet", "hyper-v", "vmware", "virtualbox", "loopback", "wintun",
	"nordlynx", "openvpn", "hamachi", "zerotier", "tailscale",
}

func isVirtualOrVPNAdapter(name string) bool {
	low := strings.ToLower(strings.TrimSpace(name))
	if low == "" {
		return true
	}
	for _, n := range virtualAdapterNeedles {
		if strings.Contains(low, n) {
			return true
		}
	}
	return false
}

func probeNetworkAdapterInfo() adapterInfo {
	out := adapterInfo{}
	gw := resolveDefaultGatewayIPv4()
	out.Gateway = gw

	type cand struct {
		info     adapterInfo
		hasGW    bool
		hasMAC   bool
		priority int
	}
	var best *cand

	ifaces, err := net.Interfaces()
	if err == nil {
		for _, iface := range ifaces {
			if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
				continue
			}
			if isVirtualOrVPNAdapter(iface.Name) {
				continue
			}
			if len(iface.HardwareAddr) == 0 {
				continue
			}
			addrs, err := iface.Addrs()
			if err != nil {
				continue
			}
			var localIP string
			for _, a := range addrs {
				ipNet, ok := a.(*net.IPNet)
				if !ok || ipNet.IP == nil || ipNet.IP.To4() == nil || ipNet.IP.IsLoopback() {
					continue
				}
				localIP = ipNet.IP.String()
				break
			}
			if localIP == "" {
				continue
			}
			info := adapterInfo{
				LocalIP:       localIP,
				AdapterName:   iface.Name,
				AdapterStatus: "Up",
				ConnType:      "Ethernet",
				Gateway:       gw,
			}
			c := cand{info: info, hasMAC: true, hasGW: gw != ""}
			if gw != "" {
				c.priority = 2
			} else {
				c.priority = 1
			}
			if best == nil || c.priority > best.priority {
				cp := c
				best = &cp
			}
		}
	}

	if best != nil {
		out = best.info
		out.DNS = dnsServersFromPowerShell()
	} else {
		out.DNS = dnsServersFromPowerShell()
	}

	wifi := wifiInfoFromNetsh()
	if (wifi.SSID != "" || wifi.Signal > 0) && !isVirtualOrVPNAdapter(wifi.Adapter) {
		out.ConnType = "Wi-Fi"
		out.WifiSSID = wifi.SSID
		out.WifiSignal = wifi.Signal
		out.WifiBand = wifi.Band
		if wifi.Adapter != "" {
			out.AdapterName = wifi.Adapter
		}
		out.AdapterStatus = "Up"
		if out.Gateway == "" {
			out.Gateway = gw
		}
	}
	if out.ConnType == "" && out.LocalIP != "" {
		out.ConnType = "Ethernet"
	}
	if mbps := linkSpeedMbps(out.AdapterName); mbps > 0 && mbps < 200000 {
		// Ignore absurd virtual speeds (e.g. 100000 Mbps).
		out.LinkMbps = mbps
		out.LinkSpeedLabel = formatLinkSpeed(mbps)
	}
	return out
}

func formatLinkSpeed(mbps float64) string {
	if mbps <= 0 {
		return ""
	}
	if mbps >= 1000 {
		gb := mbps / 1000
		if gb == float64(int(gb)) {
			return strconv.Itoa(int(gb)) + " Gbps"
		}
		return strconv.FormatFloat(gb, 'f', 1, 64) + " Gbps"
	}
	if mbps == float64(int(mbps)) {
		return strconv.Itoa(int(mbps)) + " Mbps"
	}
	return strconv.FormatFloat(mbps, 'f', 0, 64) + " Mbps"
}

func dnsServersFromPowerShell() []string {
	cmd := `(Get-DnsClientServerAddress -AddressFamily IPv4 | Where-Object { $_.ServerAddresses } | Select-Object -ExpandProperty ServerAddresses -Unique | Select-Object -First 4) -join ','`
	raw, err := runHiddenOutput("powershell", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", cmd)
	if err != nil {
		return nil
	}
	parts := strings.Split(strings.TrimSpace(string(raw)), ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if net.ParseIP(p) != nil {
			out = append(out, p)
		}
	}
	return out
}

type wifiSnap struct {
	SSID    string
	Signal  int
	Band    string
	Adapter string
}

func wifiInfoFromNetsh() wifiSnap {
	cmd := exec.Command("netsh", "wlan", "show", "interfaces")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	raw, err := cmd.Output()
	if err != nil {
		return wifiSnap{}
	}
	text := string(raw)
	var snap wifiSnap
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		low := strings.ToLower(line)
		if strings.HasPrefix(low, "ssid") && !strings.Contains(low, "bssid") {
			if i := strings.Index(line, ":"); i >= 0 {
				snap.SSID = strings.TrimSpace(line[i+1:])
			}
		}
		if strings.Contains(low, "signal") {
			if i := strings.Index(line, ":"); i >= 0 {
				num := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(line[i+1:]), "%"))
				if v, err := strconv.Atoi(num); err == nil {
					snap.Signal = v
				}
			}
		}
		if strings.Contains(low, "band") || strings.Contains(low, "radio type") {
			if i := strings.Index(line, ":"); i >= 0 {
				snap.Band = strings.TrimSpace(line[i+1:])
			}
		}
		if strings.HasPrefix(low, "name") {
			if i := strings.Index(line, ":"); i >= 0 {
				snap.Adapter = strings.TrimSpace(line[i+1:])
			}
		}
	}
	return snap
}

func linkSpeedMbps(adapterName string) float64 {
	if adapterName == "" || isVirtualOrVPNAdapter(adapterName) {
		return 0
	}
	safe := strings.ReplaceAll(adapterName, "'", "''")
	cmd := `(Get-NetAdapter -Name '` + safe + `' -ErrorAction SilentlyContinue | Select-Object -First 1 -ExpandProperty LinkSpeed)`
	raw, err := runHiddenOutput("powershell", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", cmd)
	if err != nil {
		return 0
	}
	s := strings.ToLower(strings.TrimSpace(string(raw)))
	s = strings.ReplaceAll(s, ",", ".")
	mult := 1.0
	switch {
	case strings.Contains(s, "gbps"):
		mult = 1000
		s = strings.ReplaceAll(s, "gbps", "")
	case strings.Contains(s, "mbps"):
		s = strings.ReplaceAll(s, "mbps", "")
	}
	s = strings.TrimSpace(s)
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return v * mult
}
