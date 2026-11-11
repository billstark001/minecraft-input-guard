package win32

import (
	"unsafe"

	"minecraft-input-guard/internal/game"
)

var (
	getCursorInfo = user32.NewProc("GetCursorInfo")
	getClipCursor = user32.NewProc("GetClipCursor")
	getIconInfo   = user32.NewProc("GetIconInfo")
	getDC         = user32.NewProc("GetDC")
	releaseDC     = user32.NewProc("ReleaseDC")
	getObject     = gdi32.NewProc("GetObjectW")
	getDIBits     = gdi32.NewProc("GetDIBits")
	deleteObject  = gdi32.NewProc("DeleteObject")
)

type cursorInfo struct {
	Size, Flags uint32
	Handle      uintptr
	Position    Point
}

// CursorDetector caches the bitmap classification for the current HCURSOR.
// LWJGL 2 may install a transparent cursor without clearing CURSOR_SHOWING.
type CursorDetector struct {
	handle      uintptr
	transparent bool
}

func (d *CursorDetector) Mode() game.MouseMode {
	var cursor cursorInfo
	cursor.Size = uint32(unsafe.Sizeof(cursor))
	ok, _, _ := getCursorInfo.Call(uintptr(unsafe.Pointer(&cursor)))
	if ok == 0 {
		return game.MouseUnknown
	}
	if cursor.Handle != d.handle {
		d.handle = cursor.Handle
		d.transparent = cursor.Handle != 0 && cursor.Flags&1 != 0 && blankCursor(cursor.Handle)
	}
	return game.CursorMode(cursor.Flags, cursor.Handle, d.transparent)
}

func CursorClip() (Rect, bool) {
	var bounds Rect
	ok, _, _ := getClipCursor.Call(uintptr(unsafe.Pointer(&bounds)))
	return bounds, ok != 0
}

type iconInfo struct {
	Icon        int32
	HotX, HotY  uint32
	Mask, Color uintptr
}

type bitmap struct {
	Type, Width, Height, WidthBytes int32
	Planes, BitsPixel               uint16
	Bits                            uintptr
}

type bitmapInfoHeader struct {
	Size                         uint32
	Width, Height                int32
	Planes, BitCount             uint16
	Compression, ImageSize       uint32
	XPelsPerMeter, YPelsPerMeter int32
	ColorsUsed, ColorsImportant  uint32
}

func blankCursor(handle uintptr) bool {
	var info iconInfo
	ok, _, _ := getIconInfo.Call(handle, uintptr(unsafe.Pointer(&info)))
	if ok == 0 {
		return false
	}
	if info.Mask != 0 {
		defer deleteObject.Call(info.Mask)
	}
	if info.Color != 0 {
		defer deleteObject.Call(info.Color)
	}
	dc, _, _ := getDC.Call(0)
	if dc == 0 {
		return false
	}
	defer releaseDC.Call(0, dc)

	mask := bitmapPixels(dc, info.Mask)
	if info.Color != 0 {
		color := bitmapPixels(dc, info.Color)
		return uniform(mask, 0xFFFFFF) && uniform(color, 0)
	}
	// For monochrome cursors, the AND mask is followed by the XOR mask.
	if len(mask)%2 != 0 {
		return false
	}
	half := len(mask) / 2
	return uniform(mask[:half], 0xFFFFFF) && uniform(mask[half:], 0)
}

func bitmapPixels(dc, handle uintptr) []uint32 {
	if handle == 0 {
		return nil
	}
	var desc bitmap
	ok, _, _ := getObject.Call(handle, unsafe.Sizeof(desc), uintptr(unsafe.Pointer(&desc)))
	if ok == 0 || desc.Width < 1 || desc.Width > 512 || desc.Height < 1 || desc.Height > 1024 {
		return nil
	}
	pixels := make([]uint32, int(desc.Width)*int(desc.Height))
	header := bitmapInfoHeader{
		Size: uint32(unsafe.Sizeof(bitmapInfoHeader{})), Width: desc.Width, Height: -desc.Height,
		Planes: 1, BitCount: 32,
	}
	rows, _, _ := getDIBits.Call(dc, handle, 0, uintptr(desc.Height),
		uintptr(unsafe.Pointer(&pixels[0])), uintptr(unsafe.Pointer(&header)), 0)
	if rows != uintptr(desc.Height) {
		return nil
	}
	return pixels
}

func uniform(pixels []uint32, rgb uint32) bool {
	if len(pixels) == 0 {
		return false
	}
	for _, pixel := range pixels {
		if pixel&0xFFFFFF != rgb {
			return false
		}
	}
	return true
}
