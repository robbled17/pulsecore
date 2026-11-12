//go:build !windows

package main

import "time"

func icmpPingIPv4(host string, timeout time.Duration) float64 {
	_ = host
	_ = timeout
	return -1
}
