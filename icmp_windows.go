//go:build windows

package main

import (
	"encoding/binary"
	"net"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	modIphlpapi      = windows.NewLazySystemDLL("iphlpapi.dll")
	procIcmpCreate   = modIphlpapi.NewProc("IcmpCreateFile")
	procIcmpClose    = modIphlpapi.NewProc("IcmpCloseHandle")
	procIcmpSendEcho = modIphlpapi.NewProc("IcmpSendEcho")
)

type icmpEchoReply struct {
	Address  uint32
	Status   uint32
	RTT      uint32
	DataSize uint16
	Reserved uint16
	Data     uintptr
	Options  [8]byte // ICMP_OPTIONS padded
}

var (
	icmpMu     sync.Mutex
	icmpHandle windows.Handle
)

func icmpEnsureHandle() (windows.Handle, error) {
	icmpMu.Lock()
	defer icmpMu.Unlock()
	if icmpHandle != 0 {
		return icmpHandle, nil
	}
	r, _, err := procIcmpCreate.Call()
	if r == 0 || r == uintptr(syscall.InvalidHandle) {
		if err != nil {
			return 0, err
		}
		return 0, windows.ERROR_INVALID_HANDLE
	}
	icmpHandle = windows.Handle(r)
	return icmpHandle, nil
}

// icmpPingIPv4 sends one ICMP echo via IcmpSendEcho (no admin required).
// Returns RTT in milliseconds, or -1 on failure.
func icmpPingIPv4(host string, timeout time.Duration) float64 {
	ip := net.ParseIP(host)
	if ip == nil {
		ips, err := net.LookupIP(host)
		if err != nil || len(ips) == 0 {
			return -1
		}
		for _, cand := range ips {
			if v4 := cand.To4(); v4 != nil {
				ip = v4
				break
			}
		}
	}
	v4 := ip.To4()
	if v4 == nil {
		return -1
	}
	addr := binary.LittleEndian.Uint32(v4)

	h, err := icmpEnsureHandle()
	if err != nil || h == 0 {
		return -1
	}
	if timeout <= 0 {
		timeout = 1200 * time.Millisecond
	}
	ms := uint32(timeout / time.Millisecond)
	if ms < 200 {
		ms = 200
	}
	req := []byte("PulseCore")
	replyBuf := make([]byte, int(unsafe.Sizeof(icmpEchoReply{}))+8+len(req)+16)
	r1, _, _ := procIcmpSendEcho.Call(
		uintptr(h),
		uintptr(addr),
		uintptr(unsafe.Pointer(&req[0])),
		uintptr(len(req)),
		0,
		uintptr(unsafe.Pointer(&replyBuf[0])),
		uintptr(len(replyBuf)),
		uintptr(ms),
	)
	if r1 == 0 {
		return -1
	}
	rep := (*icmpEchoReply)(unsafe.Pointer(&replyBuf[0]))
	if rep.Status != 0 {
		return -1
	}
	return float64(rep.RTT)
}
