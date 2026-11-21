package main

import (
	"strings"
	"testing"
)

func TestParseHotkeyChord_Default(t *testing.T) {
	p := ParseHotkeyChord(defaultMiniHUDHotkey)
	if !p.Valid {
		t.Fatalf("default invalid: %s", p.Reason)
	}
	if p.Chord != "Ctrl+Shift+M" {
		t.Fatalf("chord=%s", p.Chord)
	}
	if p.Mods&(modControl|modShift) != (modControl|modShift) {
		t.Fatalf("mods=%#x", p.Mods)
	}
	if p.VK != 'M' {
		t.Fatalf("vk=%#x", p.VK)
	}
}

func TestParseHotkeyChord_Variants(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"ctrl+shift+m", "Ctrl+Shift+M"},
		{"Ctrl + Alt + F8", "Ctrl+Alt+F8"},
		{"SHIFT+CTRL+1", "Ctrl+Shift+1"},
	}
	for _, c := range cases {
		p := ParseHotkeyChord(c.in)
		if !p.Valid {
			t.Fatalf("%q invalid: %s", c.in, p.Reason)
		}
		if p.Chord != c.want {
			t.Fatalf("%q => %s want %s", c.in, p.Chord, c.want)
		}
	}
}

func TestParseHotkeyChord_Invalid(t *testing.T) {
	for _, in := range []string{"", "M", "Ctrl+", "Ctrl+Shift+M+N", "Ctrl+Foo"} {
		p := ParseHotkeyChord(in)
		if p.Valid {
			t.Fatalf("%q should be invalid", in)
		}
		if p.Reason == "" {
			t.Fatalf("%q missing reason", in)
		}
	}
}

func TestParseHotkeyChord_NoLongDashes(t *testing.T) {
	p := ParseHotkeyChord("Ctrl+Shift+M")
	if strings.ContainsAny(p.Chord, "\u2013\u2014") {
		t.Fatal("chord has long dash")
	}
}

// Hardware/OS-only (manual): live RegisterHotKey while a game holds exclusive fullscreen focus.
