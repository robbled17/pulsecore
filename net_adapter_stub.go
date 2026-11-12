//go:build !windows

package main

func probeNetworkAdapterInfo() adapterInfo {
	return adapterInfo{}
}
