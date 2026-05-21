package service

import (
	"encoding/binary"
	"image"
	"io"
	"path"
	"strings"
)

// isJPEG reports whether the path has a .jpg or .jpeg extension.
func isJPEG(relPath string) bool {
	ext := strings.ToLower(path.Ext(relPath))
	return ext == ".jpg" || ext == ".jpeg"
}

// readJPEGOrientation returns the EXIF Orientation value (1-8) from a JPEG
// stream, or 1 on any parse error or missing tag. The reader is left at an
// arbitrary position — callers must seek back to 0 before decoding.
func readJPEGOrientation(r io.Reader) int {
	var soi [2]byte
	if _, err := io.ReadFull(r, soi[:]); err != nil || soi[0] != 0xFF || soi[1] != 0xD8 {
		return 1
	}
	for {
		var marker [2]byte
		if _, err := io.ReadFull(r, marker[:]); err != nil {
			return 1
		}
		if marker[0] != 0xFF {
			return 1
		}
		// EOI or SOS: no more metadata segments
		if marker[1] == 0xD9 || marker[1] == 0xDA {
			return 1
		}
		var lenBuf [2]byte
		if _, err := io.ReadFull(r, lenBuf[:]); err != nil {
			return 1
		}
		segLen := int(binary.BigEndian.Uint16(lenBuf[:])) - 2
		if segLen < 0 {
			return 1
		}
		if marker[1] != 0xE1 { // not APP1
			if _, err := io.CopyN(io.Discard, r, int64(segLen)); err != nil {
				return 1
			}
			continue
		}
		if segLen < 6 {
			return 1
		}
		data := make([]byte, segLen)
		if _, err := io.ReadFull(r, data); err != nil {
			return 1
		}
		if string(data[:6]) != "Exif\x00\x00" {
			continue
		}
		return parseTIFFOrientation(data[6:])
	}
}

// parseTIFFOrientation reads the Orientation tag (0x0112) from a TIFF header
// beginning with the byte-order marker. Returns 1 if the tag is missing or
// the header is malformed.
func parseTIFFOrientation(tiff []byte) int {
	if len(tiff) < 8 {
		return 1
	}
	var bo binary.ByteOrder
	switch string(tiff[:2]) {
	case "II":
		bo = binary.LittleEndian
	case "MM":
		bo = binary.BigEndian
	default:
		return 1
	}
	if bo.Uint16(tiff[2:4]) != 0x002A {
		return 1
	}
	ifdOff := int(bo.Uint32(tiff[4:8]))
	if ifdOff < 0 || ifdOff+2 > len(tiff) {
		return 1
	}
	count := int(bo.Uint16(tiff[ifdOff : ifdOff+2]))
	base := ifdOff + 2
	for i := 0; i < count; i++ {
		off := base + i*12
		if off+12 > len(tiff) {
			return 1
		}
		if bo.Uint16(tiff[off:off+2]) != 0x0112 {
			continue
		}
		if bo.Uint16(tiff[off+2:off+4]) != 3 { // SHORT
			return 1
		}
		v := int(bo.Uint16(tiff[off+8 : off+10]))
		if v < 1 || v > 8 {
			return 1
		}
		return v
	}
	return 1
}

// applyOrientation returns a new image with the given EXIF orientation
// applied. Orientation 1 returns src unchanged. Called AFTER resize so the
// per-pixel loops operate on the small thumbnail, not the full-resolution
// source.
func applyOrientation(src image.Image, orientation int) image.Image {
	if orientation == 1 {
		return src
	}
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	// Orientations 5-8 swap width and height.
	var dst *image.RGBA
	switch orientation {
	case 2, 3, 4:
		dst = image.NewRGBA(image.Rect(0, 0, w, h))
	case 5, 6, 7, 8:
		dst = image.NewRGBA(image.Rect(0, 0, h, w))
	default:
		return src
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := src.At(b.Min.X+x, b.Min.Y+y)
			switch orientation {
			case 2:
				dst.Set(w-1-x, y, c)
			case 3:
				dst.Set(w-1-x, h-1-y, c)
			case 4:
				dst.Set(x, h-1-y, c)
			case 5:
				dst.Set(y, x, c)
			case 6:
				dst.Set(h-1-y, x, c)
			case 7:
				dst.Set(h-1-y, w-1-x, c)
			case 8:
				dst.Set(y, w-1-x, c)
			}
		}
	}
	return dst
}
