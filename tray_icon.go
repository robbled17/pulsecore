package main

import (
	"bytes"
	"encoding/binary"
)

// trayIconRGBA builds a 16×16 tray glyph: solid color + optional 1-2 digit value.
// Returns Windows .ico bytes (one 16×16 32bpp image).
func trayIconICO(value int, r, g, b byte) []byte {
	const size = 16
	rgba := make([]byte, size*size*4)
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			i := (y*size + x) * 4
			// soft rounded square background
			edge := x == 0 || y == 0 || x == size-1 || y == size-1
			rgba[i+0] = r
			rgba[i+1] = g
			rgba[i+2] = b
			if edge {
				rgba[i+0] = byte(int(r) * 7 / 10)
				rgba[i+1] = byte(int(g) * 7 / 10)
				rgba[i+2] = byte(int(b) * 7 / 10)
			}
			rgba[i+3] = 255
		}
	}
	if value < 0 {
		value = 0
	}
	if value > 99 {
		value = 99
	}
	inkR, inkG, inkB := byte(255), byte(255), byte(255)
	if r > 200 && g > 200 && b > 200 {
		inkR, inkG, inkB = 20, 20, 20
	}
	drawTrayDigits(rgba, size, value, inkR, inkG, inkB)
	return encodeICO16(rgba)
}

func trayColorForTemp(tempC float64, cpuPct float64) (value int, r, g, b byte, showTemp bool) {
	if tempC > 0 {
		v := int(tempC + 0.5)
		if v < 0 {
			v = 0
		}
		if v > 99 {
			v = 99
		}
		switch {
		case tempC < 70:
			return v, 34, 197, 94, true // green
		case tempC <= 85:
			return v, 234, 179, 8, true // yellow
		default:
			return v, 239, 68, 68, true // red
		}
	}
	v := int(cpuPct + 0.5)
	if v < 0 {
		v = 0
	}
	if v > 99 {
		v = 99
	}
	switch {
	case cpuPct < 70:
		return v, 56, 189, 248, false
	case cpuPct <= 85:
		return v, 234, 179, 8, false
	default:
		return v, 239, 68, 68, false
	}
}

// Tiny 3×5 digit glyphs packed as 15 bits (row-major).
var trayDigits = [10]uint16{
	0x7B6F, // 0
	0x2492, // 1
	0x73E7, // 2
	0x73CF, // 3
	0x5BC9, // 4
	0x79CF, // 5
	0x79EF, // 6
	0x7249, // 7
	0x7BEF, // 8
	0x7BCF, // 9
}

func drawTrayDigits(rgba []byte, size, value int, r, g, b byte) {
	d1 := value / 10
	d0 := value % 10
	two := value >= 10
	glyphW, glyphH := 3, 5
	gap := 1
	totalW := glyphW
	if two {
		totalW = glyphW*2 + gap
	}
	startX := (size - totalW) / 2
	startY := (size - glyphH) / 2
	if two {
		blitDigit(rgba, size, startX, startY, d1, r, g, b)
		blitDigit(rgba, size, startX+glyphW+gap, startY, d0, r, g, b)
	} else {
		blitDigit(rgba, size, startX, startY, d0, r, g, b)
	}
}

func blitDigit(rgba []byte, size, ox, oy, digit int, r, g, b byte) {
	if digit < 0 || digit > 9 {
		return
	}
	bits := trayDigits[digit]
	for row := 0; row < 5; row++ {
		for col := 0; col < 3; col++ {
			bit := bits & (1 << uint(14-(row*3+col)))
			if bit == 0 {
				continue
			}
			x := ox + col
			y := oy + row
			if x < 0 || y < 0 || x >= size || y >= size {
				continue
			}
			i := (y*size + x) * 4
			rgba[i+0] = r
			rgba[i+1] = g
			rgba[i+2] = b
			rgba[i+3] = 255
		}
	}
}

func encodeICO16(rgba []byte) []byte {
	const (
		size   = 16
		bpp    = 32
		imgOff = 6 + 16 // ICONDIR + one ICONDIRENTRY
	)
	xorSize := size * size * 4
	andRow := ((size + 31) / 32) * 4
	andSize := andRow * size
	dibSize := 40 + xorSize + andSize

	buf := bytes.NewBuffer(make([]byte, 0, imgOff+dibSize))
	// ICONDIR
	_ = binary.Write(buf, binary.LittleEndian, uint16(0))
	_ = binary.Write(buf, binary.LittleEndian, uint16(1))
	_ = binary.Write(buf, binary.LittleEndian, uint16(1))
	// ICONDIRENTRY
	buf.WriteByte(size)
	buf.WriteByte(size)
	buf.WriteByte(0)
	buf.WriteByte(0)
	_ = binary.Write(buf, binary.LittleEndian, uint16(1))
	_ = binary.Write(buf, binary.LittleEndian, uint16(bpp))
	_ = binary.Write(buf, binary.LittleEndian, uint32(dibSize))
	_ = binary.Write(buf, binary.LittleEndian, uint32(imgOff))
	// BITMAPINFOHEADER
	_ = binary.Write(buf, binary.LittleEndian, uint32(40))
	_ = binary.Write(buf, binary.LittleEndian, int32(size))
	_ = binary.Write(buf, binary.LittleEndian, int32(size*2))
	_ = binary.Write(buf, binary.LittleEndian, uint16(1))
	_ = binary.Write(buf, binary.LittleEndian, uint16(bpp))
	_ = binary.Write(buf, binary.LittleEndian, uint32(0))
	_ = binary.Write(buf, binary.LittleEndian, uint32(xorSize+andSize))
	_ = binary.Write(buf, binary.LittleEndian, int32(0))
	_ = binary.Write(buf, binary.LittleEndian, int32(0))
	_ = binary.Write(buf, binary.LittleEndian, uint32(0))
	_ = binary.Write(buf, binary.LittleEndian, uint32(0))
	// XOR bitmap bottom-up BGRA
	for y := size - 1; y >= 0; y-- {
		for x := 0; x < size; x++ {
			i := (y*size + x) * 4
			buf.WriteByte(rgba[i+2]) // B
			buf.WriteByte(rgba[i+1]) // G
			buf.WriteByte(rgba[i+0]) // R
			buf.WriteByte(rgba[i+3]) // A
		}
	}
	// AND mask (opaque → 0)
	and := make([]byte, andSize)
	buf.Write(and)
	return buf.Bytes()
}
