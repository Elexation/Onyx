// Command fixturegen emits test fixture bytes to stdout for the Onyx thumbnail
// test suite. It encodes images using the same stdlib + x/image libraries the
// server uses to decode them, so fixture generation and production decoding
// share the same implementation surface.
//
// Usage:
//
//	fixturegen -kind=jpg -w=1920 -h=1080                  plain JPEG
//	fixturegen -kind=jpg -w=640  -h=480  -orientation=6   JPEG with EXIF orientation
//	fixturegen -kind=png -w=1 -h=1                         1x1 PNG
//	fixturegen -kind=gif-animated -w=8 -h=8                2-frame animated GIF
//	fixturegen -kind=bmp -w=16 -h=16
//	fixturegen -kind=tiff -w=16 -h=16
//	fixturegen -kind=jpg -w=8 -h=8 -corrupt=20             first 20 bytes of a JPEG
//	fixturegen -kind=empty                                 zero bytes
package main

import (
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"os"

	"golang.org/x/image/bmp"
	"golang.org/x/image/tiff"
)

func main() {
	kind := flag.String("kind", "", "jpg|png|gif|gif-animated|bmp|tiff|empty")
	w := flag.Int("w", 16, "width in pixels")
	h := flag.Int("h", 16, "height in pixels")
	orientation := flag.Int("orientation", 0, "EXIF Orientation tag (1-8) for jpg; 0 = no EXIF segment")
	corrupt := flag.Int("corrupt", -1, "truncate output to first N bytes (-1 = no truncation)")
	decodeCenter := flag.String("decode-center", "", "path to image file; decode and print center pixel as 'R G B' to stdout")
	flag.Parse()

	if *decodeCenter != "" {
		decodeCenterPixel(*decodeCenter)
		return
	}

	if *kind == "empty" {
		truncateAndEmit(nil, *corrupt)
		return
	}

	if *w < 1 || *h < 1 {
		die("width and height must be >= 1")
	}

	var out bytes.Buffer
	switch *kind {
	case "jpg":
		img := solidRGBA(*w, *h, color.RGBA{R: 220, G: 40, B: 40, A: 255})
		if err := jpeg.Encode(&out, img, &jpeg.Options{Quality: 90}); err != nil {
			die("jpeg encode: %v", err)
		}
		if *orientation >= 1 && *orientation <= 8 {
			out = *spliceEXIFOrientation(&out, uint16(*orientation))
		}
	case "png":
		img := solidRGBA(*w, *h, color.RGBA{R: 40, G: 180, B: 40, A: 255})
		if err := png.Encode(&out, img); err != nil {
			die("png encode: %v", err)
		}
	case "gif":
		img := solidPaletted(*w, *h)
		if err := gif.Encode(&out, img, nil); err != nil {
			die("gif encode: %v", err)
		}
	case "gif-animated":
		frame1 := solidPaletted(*w, *h)
		frame2 := solidPaletted(*w, *h)
		// Make the second frame visually distinct so a naive "take last frame"
		// decode would be detectable. The thumbnail service decodes via
		// image.Decode which returns the FIRST frame — the test asserts on
		// first-frame content.
		for y := 0; y < *h; y++ {
			for x := 0; x < *w; x++ {
				frame2.SetColorIndex(x, y, 1)
			}
		}
		g := &gif.GIF{
			Image:     []*image.Paletted{frame1, frame2},
			Delay:     []int{10, 10},
			LoopCount: 0,
		}
		if err := gif.EncodeAll(&out, g); err != nil {
			die("gif-animated encode: %v", err)
		}
	case "bmp":
		img := solidRGBA(*w, *h, color.RGBA{R: 40, G: 40, B: 220, A: 255})
		if err := bmp.Encode(&out, img); err != nil {
			die("bmp encode: %v", err)
		}
	case "tiff":
		img := solidRGBA(*w, *h, color.RGBA{R: 220, G: 220, B: 40, A: 255})
		if err := tiff.Encode(&out, img, nil); err != nil {
			die("tiff encode: %v", err)
		}
	case "":
		die("-kind is required")
	default:
		die("unknown kind %q", *kind)
	}

	truncateAndEmit(out.Bytes(), *corrupt)
}

func truncateAndEmit(buf []byte, corrupt int) {
	if corrupt >= 0 && corrupt < len(buf) {
		buf = buf[:corrupt]
	}
	if _, err := os.Stdout.Write(buf); err != nil {
		die("stdout write: %v", err)
	}
}

func solidRGBA(w, h int, c color.RGBA) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, c)
		}
	}
	return img
}

func solidPaletted(w, h int) *image.Paletted {
	palette := color.Palette{
		color.RGBA{R: 10, G: 10, B: 10, A: 255},
		color.RGBA{R: 240, G: 240, B: 240, A: 255},
	}
	img := image.NewPaletted(image.Rect(0, 0, w, h), palette)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetColorIndex(x, y, 0)
		}
	}
	return img
}

// spliceEXIFOrientation inserts an APP1/EXIF segment containing a single
// Orientation tag immediately after the JPEG SOI marker. The result is a
// JPEG that decoders and the onyx EXIF parser both accept.
func spliceEXIFOrientation(in *bytes.Buffer, orientation uint16) *bytes.Buffer {
	src := in.Bytes()
	if len(src) < 2 || src[0] != 0xFF || src[1] != 0xD8 {
		die("spliceEXIF: input is not a JPEG (no SOI)")
	}

	// TIFF block (big-endian: "MM", magic 0x002A, IFD offset 8)
	tiff := new(bytes.Buffer)
	tiff.WriteString("MM")
	binary.Write(tiff, binary.BigEndian, uint16(0x002A))
	binary.Write(tiff, binary.BigEndian, uint32(8)) // IFD starts at offset 8 from TIFF header start
	binary.Write(tiff, binary.BigEndian, uint16(1)) // one IFD entry
	// Entry: tag=0x0112 (Orientation), type=3 (SHORT), count=1, value packed
	binary.Write(tiff, binary.BigEndian, uint16(0x0112))
	binary.Write(tiff, binary.BigEndian, uint16(3))
	binary.Write(tiff, binary.BigEndian, uint32(1))
	binary.Write(tiff, binary.BigEndian, orientation)
	binary.Write(tiff, binary.BigEndian, uint16(0)) // pad to 4-byte value field
	binary.Write(tiff, binary.BigEndian, uint32(0)) // next IFD offset = 0

	// APP1 segment = "Exif\0\0" + TIFF block
	payload := new(bytes.Buffer)
	payload.WriteString("Exif\x00\x00")
	payload.Write(tiff.Bytes())

	// Full APP1 segment with marker and length (length excludes marker, includes itself)
	segLen := payload.Len() + 2
	if segLen > 0xFFFF {
		die("spliceEXIF: APP1 segment too large (%d bytes)", segLen)
	}
	app1 := new(bytes.Buffer)
	app1.WriteByte(0xFF)
	app1.WriteByte(0xE1)
	binary.Write(app1, binary.BigEndian, uint16(segLen))
	app1.Write(payload.Bytes())

	out := new(bytes.Buffer)
	out.Write(src[:2]) // SOI
	out.Write(app1.Bytes())
	out.Write(src[2:])
	return out
}

// decodeCenterPixel reads an image file, decodes it via image.Decode (same
// chain the server uses), samples the center pixel, and prints "R G B" on
// stdout with each channel scaled to 0-255. Used by the phase 19 animated
// GIF test to prove the thumbnail pipeline returned frame 1 content.
func decodeCenterPixel(path string) {
	f, err := os.Open(path)
	if err != nil {
		die("open %s: %v", path, err)
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		die("decode %s: %v", path, err)
	}
	b := img.Bounds()
	cx := b.Min.X + (b.Dx() / 2)
	cy := b.Min.Y + (b.Dy() / 2)
	r, g, bl, _ := img.At(cx, cy).RGBA()
	fmt.Printf("%d %d %d\n", r>>8, g>>8, bl>>8)
}

func die(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "fixturegen: "+format+"\n", args...)
	os.Exit(1)
}
