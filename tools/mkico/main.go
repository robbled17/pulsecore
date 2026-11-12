package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
)

func main() {
	root := "."
	if len(os.Args) > 1 {
		root = os.Args[1]
	}
	srcPath := filepath.Join(root, "build", "appicon.png")
	dstPath := filepath.Join(root, "build", "windows", "icon.ico")
	f, err := os.Open(srcPath)
	if err != nil {
		fatal(err)
	}
	defer f.Close()
	src, err := png.Decode(f)
	if err != nil {
		fatal(err)
	}
	sizes := []int{16, 32, 48, 256}
	var entries []icoEntry
	for _, sz := range sizes {
		dst := resizeRGBA(src, sz, sz)
		var buf bytes.Buffer
		if err := png.Encode(&buf, dst); err != nil {
			fatal(err)
		}
		entries = append(entries, icoEntry{size: sz, png: buf.Bytes()})
	}
	if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
		fatal(err)
	}
	out, err := os.Create(dstPath)
	if err != nil {
		fatal(err)
	}
	defer out.Close()
	if err := writeICO(out, entries); err != nil {
		fatal(err)
	}
	fmt.Println("wrote", dstPath)
}

type icoEntry struct {
	size int
	png  []byte
}

func resizeRGBA(src image.Image, w, h int) *image.RGBA {
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	sb := src.Bounds()
	sw, sh := sb.Dx(), sb.Dy()
	for y := 0; y < h; y++ {
		sy := sb.Min.Y + y*sh/h
		for x := 0; x < w; x++ {
			sx := sb.Min.X + x*sw/w
			dst.Set(x, y, src.At(sx, sy))
		}
	}
	return dst
}

func writeICO(w *os.File, entries []icoEntry) error {
	hdr := make([]byte, 6)
	binary.LittleEndian.PutUint16(hdr[0:], 0)
	binary.LittleEndian.PutUint16(hdr[2:], 1)
	binary.LittleEndian.PutUint16(hdr[4:], uint16(len(entries)))
	if _, err := w.Write(hdr); err != nil {
		return err
	}
	offset := 6 + 16*len(entries)
	for _, e := range entries {
		ent := make([]byte, 16)
		if e.size >= 256 {
			ent[0], ent[1] = 0, 0
		} else {
			ent[0], ent[1] = byte(e.size), byte(e.size)
		}
		binary.LittleEndian.PutUint16(ent[4:], 1)
		binary.LittleEndian.PutUint16(ent[6:], 32)
		binary.LittleEndian.PutUint32(ent[8:], uint32(len(e.png)))
		binary.LittleEndian.PutUint32(ent[12:], uint32(offset))
		if _, err := w.Write(ent); err != nil {
			return err
		}
		offset += len(e.png)
	}
	for _, e := range entries {
		if _, err := w.Write(e.png); err != nil {
			return err
		}
	}
	return nil
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
