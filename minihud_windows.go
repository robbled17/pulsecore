//go:build windows

package main

import (
	"math"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	modUser32MH             = windows.NewLazySystemDLL("user32.dll")
	modGdi32MH              = windows.NewLazySystemDLL("gdi32.dll")
	procUpdateLayeredWindow = modUser32MH.NewProc("UpdateLayeredWindow")
	procGetDCMH             = modUser32MH.NewProc("GetDC")
	procReleaseDCMH         = modUser32MH.NewProc("ReleaseDC")
	procGetWindowRectMH     = modUser32MH.NewProc("GetWindowRect")
	procGetWindowLongPtrMH  = modUser32MH.NewProc("GetWindowLongPtrW")
	procSetWindowLongPtrMH  = modUser32MH.NewProc("SetWindowLongPtrW")
	procSetWindowPosMH      = modUser32MH.NewProc("SetWindowPos")
	procCreateDIBSection    = modGdi32MH.NewProc("CreateDIBSection")
	procCreateCompatibleDC  = modGdi32MH.NewProc("CreateCompatibleDC")
	procDeleteDC            = modGdi32MH.NewProc("DeleteDC")
	procSelectObjectMH      = modGdi32MH.NewProc("SelectObject")
	procDeleteObjectMH      = modGdi32MH.NewProc("DeleteObject")
)

type pointMH struct {
	X int32
	Y int32
}

type sizeMH struct {
	Cx int32
	Cy int32
}

type rectMH struct {
	Left   int32
	Top    int32
	Right  int32
	Bottom int32
}

type dibBitmapInfoHeader struct {
	BiSize          uint32
	BiWidth         int32
	BiHeight        int32
	BiPlanes        uint16
	BiBitCount      uint16
	BiCompression   uint32
	BiSizeImage     uint32
	BiXPelsPerMeter int32
	BiYPelsPerMeter int32
	BiClrUsed       uint32
	BiClrImportant  uint32
}

type dibBitmapInfo struct {
	BmiHeader dibBitmapInfoHeader
	BmiColors [1]uint32
}

type blendFunc struct {
	BlendOp             byte
	BlendFlags          byte
	SourceConstantAlpha byte
	AlphaFormat         byte
}

const (
	wsExLayeredOS = 0x00080000
	gwlExStyleOS  = ^uintptr(19) // -20
	biRGBOS       = 0
	dibRGBColors  = 0
	acSrcOverOS   = 0x00
	acSrcAlphaOS  = 0x01
	ulwAlphaOS    = 0x00000002
)

// clearLayeredWindow removes the WS_EX_LAYERED extended style when returning to standard window mode.
func clearLayeredWindow(hwnd windows.HWND) {
	if hwnd == 0 {
		return
	}
	exStyle, _, _ := procGetWindowLongPtrMH.Call(uintptr(hwnd), gwlExStyleOS)
	if (exStyle & wsExLayeredOS) != 0 {
		_, _, _ = procSetWindowLongPtrMH.Call(uintptr(hwnd), gwlExStyleOS, exStyle&^wsExLayeredOS)
		_, _, _ = procSetWindowPosMH.Call(uintptr(hwnd), 0, 0, 0, 0, 0, 0x0027 /* SWP_NOSIZE | SWP_NOMOVE | SWP_NOZORDER | SWP_FRAMECHANGED */)
	}
}

// updateLayeredCapsule draws a clean 32-bit ARGB capsule for Mini-HUD without rectangular bleed or underlay shadow.
func updateLayeredCapsule(hwnd windows.HWND, width, height int, isWin11 bool) bool {
	return updateLayeredCapsuleInternal(hwnd, width, height, isWin11, false)
}

// updateLayeredCapsuleInternal draws a clean 32-bit ARGB capsule with a completely cleared (memset 0) buffer.
func updateLayeredCapsuleInternal(hwnd windows.HWND, width, height int, isWin11 bool, isFPSOverlay bool) bool {
	if hwnd == 0 || width <= 0 || height <= 0 {
		return false
	}

	// Ensure WS_EX_LAYERED is active
	exStyle, _, _ := procGetWindowLongPtrMH.Call(uintptr(hwnd), gwlExStyleOS)
	if (exStyle & wsExLayeredOS) == 0 {
		_, _, _ = procSetWindowLongPtrMH.Call(uintptr(hwnd), gwlExStyleOS, exStyle|wsExLayeredOS)
	}

	hdcScreen, _, _ := procGetDCMH.Call(0)
	if hdcScreen == 0 {
		return false
	}
	defer procReleaseDCMH.Call(0, hdcScreen)

	memDC, _, _ := procCreateCompatibleDC.Call(hdcScreen)
	if memDC == 0 {
		return false
	}
	defer procDeleteDC.Call(memDC)

	var bmi dibBitmapInfo
	bmi.BmiHeader.BiSize = uint32(unsafe.Sizeof(bmi.BmiHeader))
	bmi.BmiHeader.BiWidth = int32(width)
	bmi.BmiHeader.BiHeight = -int32(height) // top-down
	bmi.BmiHeader.BiPlanes = 1
	bmi.BmiHeader.BiBitCount = 32
	bmi.BmiHeader.BiCompression = biRGBOS

	var pBits unsafe.Pointer
	hBitmap, _, _ := procCreateDIBSection.Call(
		memDC,
		uintptr(unsafe.Pointer(&bmi)),
		dibRGBColors,
		uintptr(unsafe.Pointer(&pBits)),
		0,
		0,
	)
	if hBitmap == 0 || pBits == nil {
		return false
	}
	defer procDeleteObjectMH.Call(hBitmap)

	oldBmp, _, _ := procSelectObjectMH.Call(memDC, hBitmap)
	defer procSelectObjectMH.Call(memDC, oldBmp)

	// Step 1: Force entire pixel buffer to 0 (clean transparent ARGB) - eliminates all rectangular bleed and shadows
	totalPixels := width * height
	pixelSlice := unsafe.Slice((*uint32)(pBits), totalPixels)
	for i := range pixelSlice {
		pixelSlice[i] = 0
	}

	// Step 2: Draw the capsule strictly within 0..width and 0..height
	radius := float64(height) / 2.0
	leftCenter := radius
	rightCenter := float64(width) - radius
	if rightCenter < leftCenter {
		rightCenter = leftCenter
	}

	// Capsule colors (premultiplied ARGB):
	// Solid opaque #0f172a (alpha=255) ensures lower windows NEVER shine/bleed through!
	bgA := uint32(255)
	bgR := uint32(15)
	bgG := uint32(23)
	bgB := uint32(42)

	var bdrA, bdrR, bdrG, bdrB uint32
	if isFPSOverlay {
		// F12 overlay cyan border: #38bdf8 (R=56, G=189, B=248)
		bdrA = 255
		bdrR = 56
		bdrG = 189
		bdrB = 248
	} else if isWin11 {
		// Mini-HUD Win11: clean white border
		bdrA = 255
		bdrR = 64
		bdrG = 75
		bdrB = 92
	} else {
		// Mini-HUD Win10: border #334155 (R=51, G=65, B=85)
		bdrA = 255
		bdrR = 51
		bdrG = 65
		bdrB = 85
	}

	bgPixel := (bgA << 24) | (bgR << 16) | (bgG << 8) | bgB
	bdrPixel := (bdrA << 24) | (bdrR << 16) | (bdrG << 8) | bdrB

	for y := 0; y < height; y++ {
		fy := float64(y) + 0.5
		dy := fy - radius

		for x := 0; x < width; x++ {
			fx := float64(x) + 0.5
			var dist float64

			if fx < leftCenter {
				dx := fx - leftCenter
				dist = math.Sqrt(dx*dx + dy*dy)
			} else if fx > rightCenter {
				dx := fx - rightCenter
				dist = math.Sqrt(dx*dx + dy*dy)
			} else {
				dist = math.Abs(dy)
			}

			if dist <= radius-1.0 {
				pixelSlice[y*width+x] = bgPixel
			} else if dist <= radius {
				pixelSlice[y*width+x] = bdrPixel
			}
		}
	}

	ptDst := pointMH{X: 0, Y: 0}
	var rect rectMH
	procGetWindowRectMH.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&rect)))
	ptDst.X = rect.Left
	ptDst.Y = rect.Top

	szDst := sizeMH{Cx: int32(width), Cy: int32(height)}
	ptSrc := pointMH{X: 0, Y: 0}

	blend := blendFunc{
		BlendOp:             acSrcOverOS,
		BlendFlags:          0,
		SourceConstantAlpha: 255,
		AlphaFormat:         acSrcAlphaOS,
	}

	r, _, _ := procUpdateLayeredWindow.Call(
		uintptr(hwnd),
		hdcScreen,
		uintptr(unsafe.Pointer(&ptDst)),
		uintptr(unsafe.Pointer(&szDst)),
		memDC,
		uintptr(unsafe.Pointer(&ptSrc)),
		0,
		uintptr(unsafe.Pointer(&blend)),
		ulwAlphaOS,
	)

	return r != 0
}
