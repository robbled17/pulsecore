//go:build windows

package main

import "testing"

func TestIsMainAppWindowTitle(t *testing.T) {
	cases := []struct {
		title    string
		expected bool
	}{
		{"PulseCore", true},
		{"PulseCore - Hardware Intelligence", true},
		{"PulseCore Tray Window", false},
		{"PulseCore Conflict Bar", false},
		{"PulseCore Mini-HUD", false},
		{"PulseCore Overlay", false},
		{"PulseCoreHelper", false},
		{"Chrome - PulseCore Docs", false},
		{"", false},
	}

	for _, c := range cases {
		got := isMainAppWindowTitle(c.title)
		if got != c.expected {
			t.Errorf("isMainAppWindowTitle(%q) = %v, expected %v", c.title, got, c.expected)
		}
	}
}
