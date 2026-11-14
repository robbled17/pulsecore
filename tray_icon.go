package main

import (
	"bytes"
	"encoding/binary"
)

// trayIconICO builds a 16×16 tray glyph: dark glass badge + crisp pulse/metric text.
// Returns Windows .ico bytes (one 16×16 32bpp image).
func trayIconICO(value int, r, g, b byte) []byte {
	const size = 16
	rgba := make([]byte, size*size*4)
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			i := (y*size + x) * 4
			// Transparent outer corner pixels for smooth capsule
			isCorner := (x == 0 && y == 0) || (x == size-1 && y == 0) || (x == 0 && y == size-1) || (x == size-1 && y == size-1)
			if isCorner {
				rgba[i+3] = 0
				continue
			}
			isBorder := x == 0 || y == 0 || x == size-1 || y == size-1
			if isBorder {
				// Sleek subtle border #334155
				rgba[i+0] = 51
				rgba[i+1] = 65
				rgba[i+2] = 85
				rgba[i+3] = 230
			} else {
				// Clean dark background #0f172a
				rgba[i+0] = 15
				rgba[i+1] = 23
				rgba[i+2] = 42
				rgba[i+3] = 255
			}
		}
	}
	if value < 0 {
		value = 0
	}
	if value > 99 {
		value = 99
	}

	if value == 0 {
		// Draw sleek ECG pulse wave
		drawPulseGlyph(rgba, size, r, g, b)
	} else {
		// Draw crisp digits with accent color
		drawTrayDigits(rgba, size, value, r, g, b)
		// Small accent status line at bottom
		for x := 4; x <= 11; x++ {
			i := (14*size + x) * 4
			rgba[i+0] = r
			rgba[i+1] = g
			rgba[i+2] = b
			rgba[i+3] = 220
		}
	}

	return encodeICO16(rgba)
}

func drawPulseGlyph(rgba []byte, size int, r, g, b byte) {
	// Baseline at y = 8
	plot := func(x, y int) {
		if x >= 0 && x < size && y >= 0 && y < size {
			i := (y*size + x) * 4
			rgba[i+0] = r
			rgba[i+1] = g
			rgba[i+2] = b
			rgba[i+3] = 255
		}
	}
	for x := 2; x <= 4; x++ {
		plot(x, 8)
	}
	plot(5, 7)
	plot(6, 4)
	plot(7, 3)
	plot(8, 12)
	plot(9, 13)
	plot(10, 6)
	plot(11, 8)
	for x := 12; x <= 13; x++ {
		plot(x, 8)
	}
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
		return v, 56, 189, 248, false // cyan
	case cpuPct <= 85:
		return v, 234, 179, 8, false // yellow
	default:
		return v, 239, 68, 68, false // red
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
