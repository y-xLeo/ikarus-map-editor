package sromap

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
)

// EncodeDDJ wraps the DXT1-compressed bytes of an RGBA image into a
// JMXVDDJ container that LoadDDJ accepts. Alpha is dropped (DXT1 is 4-color
// no-alpha mode); Silkroad uses DXT3/DXT5 for transparent surfaces.
//
// Layout:
//
//	[20 bytes] "JMXVDDJ 1000" + 8 bytes padding
//	[4 bytes]  "DDS "
//	[124 bytes] DDS header (DXT1 fourCC)
//	[blocks]   8 bytes per 4x4 pixel block
//
// Width and height must be ≥ 4 and multiples of 4; callers should pre-resize.
func EncodeDDJ(img *image.RGBA) ([]byte, error) {
	if img == nil {
		return nil, fmt.Errorf("ddj encode: nil image")
	}
	w := img.Rect.Dx()
	h := img.Rect.Dy()
	if w < 4 || h < 4 {
		return nil, fmt.Errorf("ddj encode: image too small (%dx%d, min 4x4)", w, h)
	}
	if w%4 != 0 || h%4 != 0 {
		return nil, fmt.Errorf("ddj encode: dimensions must be multiples of 4, got %dx%d", w, h)
	}

	blockBytes, err := EncodeDXT1(img.Pix, w, h)
	if err != nil {
		return nil, fmt.Errorf("ddj encode: %w", err)
	}

	var buf bytes.Buffer
	buf.Grow(20 + 128 + len(blockBytes))

	// JMXVDDJ wrapper. Bytes 12-15 are the size of everything from byte 12
	// onwards (file_size - 12). Bytes 16-19 are a "3" version marker — both
	// observed in every real DDJ. Without these the engine fails to load
	// the texture and renders the model as untextured grey.
	buf.WriteString("JMXVDDJ 1000")
	sizeAt := buf.Len()
	buf.Write(make([]byte, 4)) // patched at end with file_size - 12
	var version [4]byte
	binary.LittleEndian.PutUint32(version[:], 3)
	buf.Write(version[:])

	buf.WriteString("DDS ")
	header := make([]byte, 124)
	binary.LittleEndian.PutUint32(header[0:4], 124) // dwSize
	// dwFlags: caps + h + w + pixelformat + mipmapcount (0x20000). Real
	// DDJs ship with 10 mip levels; we declare 1 so the engine doesn't
	// look for missing extra data.
	binary.LittleEndian.PutUint32(header[4:8], 0x00021007)
	binary.LittleEndian.PutUint32(header[8:12], uint32(h))
	binary.LittleEndian.PutUint32(header[12:16], uint32(w))
	// dwPitchOrLinearSize: real DDJs put the horizontal block count here
	// (w/4). For DXT1 / DDPF_FOURCC the field's exact value isn't strictly
	// defined by the spec, but matching real-file values is safer.
	binary.LittleEndian.PutUint32(header[16:20], uint32(w/4))
	// dwMipMapCount = 1 (just the base level we wrote).
	binary.LittleEndian.PutUint32(header[24:28], 1)
	binary.LittleEndian.PutUint32(header[72:76], 32) // pixel format dwSize
	binary.LittleEndian.PutUint32(header[76:80], 4)  // DDPF_FOURCC
	copy(header[80:84], "DXT1")
	// dwCaps1: DDSCAPS_COMPLEX (0x8) | DDSCAPS_TEXTURE (0x1000) | DDSCAPS_MIPMAP (0x400000).
	binary.LittleEndian.PutUint32(header[104:108], 0x00401008)
	buf.Write(header)

	buf.Write(blockBytes)

	data := buf.Bytes()
	binary.LittleEndian.PutUint32(data[sizeAt:sizeAt+4], uint32(len(data)-12))
	return data, nil
}

// EncodeDDJFromImage is a convenience wrapper that converts an arbitrary
// image.Image to RGBA, optionally downscales it (longest axis = maxDim),
// and snaps both dims down to multiples of 4 before encoding.
func EncodeDDJFromImage(src image.Image, maxDim int) ([]byte, error) {
	rgba, err := imageToRGBAResized(src, maxDim)
	if err != nil {
		return nil, err
	}
	return EncodeDDJ(rgba)
}

func imageToRGBAResized(src image.Image, maxDim int) (*image.RGBA, error) {
	b := src.Bounds()
	sw, sh := b.Dx(), b.Dy()
	w, h := sw, sh
	if maxDim > 0 && (w > maxDim || h > maxDim) {
		if w >= h {
			h = h * maxDim / w
			w = maxDim
		} else {
			w = w * maxDim / h
			h = maxDim
		}
	}
	w = (w / 4) * 4
	h = (h / 4) * 4
	if w < 4 || h < 4 {
		return nil, fmt.Errorf("source image too small after downscale: %dx%d", w, h)
	}
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		sy := y * sh / h
		for x := 0; x < w; x++ {
			sx := x * sw / w
			r, g, bb, a := src.At(b.Min.X+sx, b.Min.Y+sy).RGBA()
			off := (y*w + x) * 4
			dst.Pix[off] = byte(r >> 8)
			dst.Pix[off+1] = byte(g >> 8)
			dst.Pix[off+2] = byte(bb >> 8)
			dst.Pix[off+3] = byte(a >> 8)
		}
	}
	return dst, nil
}
