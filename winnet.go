package main

import (
	"net"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

// defaultGatewayIPv4 resolves the IPv4 default gateway via IP Helper
// (GetAdaptersAddresses). No PowerShell / netsh.
func defaultGatewayIPv4() string {
	if gw := gatewayFromAdapters(); gw != "" {
		return gw
	}
	return gatewayFromRoutePrint()
}

func gatewayFromAdapters() string {
	var size uint32 = 16 * 1024
	for attempt := 0; attempt < 4; attempt++ {
		buf := make([]byte, size)
		aa := (*windows.IpAdapterAddresses)(unsafe.Pointer(&buf[0]))
		err := windows.GetAdaptersAddresses(
			windows.AF_INET,
			windows.GAA_FLAG_INCLUDE_PREFIX|windows.GAA_FLAG_INCLUDE_GATEWAYS,
			0,
			aa,
			&size,
		)
		if err == windows.ERROR_BUFFER_OVERFLOW {
			continue
		}
		if err != nil {
			return ""
		}
		for ; aa != nil; aa = aa.Next {
			if aa.OperStatus != windows.IfOperStatusUp {
				continue
			}
			for gw := aa.FirstGatewayAddress; gw != nil; gw = gw.Next {
				ip := gw.Address.IP()
				if ip4 := ip.To4(); ip4 != nil && !ip4.IsUnspecified() && !ip4.IsLoopback() {
					return ip4.String()
				}
			}
		}
		return ""
	}
	return ""
}

// gatewayFromRoutePrint is a no-PowerShell fallback using System32\route.exe.
func gatewayFromRoutePrint() string {
	out, err := runHiddenOutput(system32("route.exe"), "print", "0.0.0.0")
	if err != nil && len(out) == 0 {
		return ""
	}
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 || fields[0] != "0.0.0.0" {
			continue
		}
		gw := fields[2]
		if ip := net.ParseIP(gw); ip != nil && !ip.IsUnspecified() {
			return gw
		}
	}
	return ""
}
