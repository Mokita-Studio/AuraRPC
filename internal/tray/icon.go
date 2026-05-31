//go:build windows

package tray

import (
	"bytes"
	"encoding/binary"
	"image/png"

	"aurarpc/internal/tray/icons"
)

// trayIcon returns ICO bytes ready for systray.SetIcon, picking the
// embedded PNG that matches the current Windows taskbar theme.
func trayIcon(dark bool) []byte {
	const size = 16
	var src []byte
	if dark {
		src = icons.ForDark16
	} else {
		src = icons.ForLight16
	}
	ico, err := pngToICO(src, size)
	if err != nil {
		return fallbackIcon()
	}
	return ico
}

// pngToICO wraps PNG bytes in a minimal ICO container. Windows Vista+
// accepts ICO entries that contain a full PNG, so no BMP conversion is needed.
func pngToICO(pngBytes []byte, declaredSize int) ([]byte, error) {
	if _, err := png.Decode(bytes.NewReader(pngBytes)); err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	w := uint8(declaredSize)
	h := uint8(declaredSize)
	if declaredSize >= 256 {
		w, h = 0, 0 // ICO convention: 0 means 256
	}
	// ICONDIR (6 bytes).
	_ = binary.Write(&buf, binary.LittleEndian, uint16(0)) // reserved
	_ = binary.Write(&buf, binary.LittleEndian, uint16(1)) // type=icon
	_ = binary.Write(&buf, binary.LittleEndian, uint16(1)) // count
	// ICONDIRENTRY (16 bytes).
	buf.WriteByte(w)
	buf.WriteByte(h)
	buf.WriteByte(0)                                                   // color count
	buf.WriteByte(0)                                                   // reserved
	_ = binary.Write(&buf, binary.LittleEndian, uint16(1))             // planes
	_ = binary.Write(&buf, binary.LittleEndian, uint16(32))            // bpp
	_ = binary.Write(&buf, binary.LittleEndian, uint32(len(pngBytes))) // size
	_ = binary.Write(&buf, binary.LittleEndian, uint32(6+16))          // offset
	buf.Write(pngBytes)
	return buf.Bytes(), nil
}

// fallbackIcon is a code-drawn 16x16 ICO used only if the embedded PNG
// fails to decode.
func fallbackIcon() []byte {
	const (
		width  = 16
		height = 16
		bpp    = 32
	)
	var buf bytes.Buffer
	pixelData := width * height * 4
	andMask := width * height / 8
	bmpSize := 40 + pixelData + andMask

	buf.Write([]byte{0, 0, 1, 0, 1, 0})
	buf.WriteByte(width)
	buf.WriteByte(height)
	buf.WriteByte(0)
	buf.WriteByte(0)
	_ = binary.Write(&buf, binary.LittleEndian, uint16(1))
	_ = binary.Write(&buf, binary.LittleEndian, uint16(bpp))
	_ = binary.Write(&buf, binary.LittleEndian, uint32(bmpSize))
	_ = binary.Write(&buf, binary.LittleEndian, uint32(22))

	_ = binary.Write(&buf, binary.LittleEndian, uint32(40))
	_ = binary.Write(&buf, binary.LittleEndian, int32(width))
	_ = binary.Write(&buf, binary.LittleEndian, int32(height*2))
	_ = binary.Write(&buf, binary.LittleEndian, uint16(1))
	_ = binary.Write(&buf, binary.LittleEndian, uint16(bpp))
	_ = binary.Write(&buf, binary.LittleEndian, uint32(0))
	_ = binary.Write(&buf, binary.LittleEndian, uint32(pixelData))
	_ = binary.Write(&buf, binary.LittleEndian, int32(0))
	_ = binary.Write(&buf, binary.LittleEndian, int32(0))
	_ = binary.Write(&buf, binary.LittleEndian, uint32(0))
	_ = binary.Write(&buf, binary.LittleEndian, uint32(0))

	for y := height - 1; y >= 0; y-- {
		for x := 0; x < width; x++ {
			dx, dy := x-7, y-7
			d2 := dx*dx + dy*dy
			if d2 <= 36 {
				buf.WriteByte(0x4F)
				buf.WriteByte(0x6B)
				buf.WriteByte(0x7D)
				buf.WriteByte(0xFF)
			} else {
				buf.WriteByte(0)
				buf.WriteByte(0)
				buf.WriteByte(0)
				buf.WriteByte(0)
			}
		}
	}
	for i := 0; i < andMask; i++ {
		buf.WriteByte(0)
	}
	return buf.Bytes()
}
