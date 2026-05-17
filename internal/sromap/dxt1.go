package sromap

import (
	"encoding/binary"
	"fmt"
)

// EncodeDXT1 compresses an RGBA8 image into DXT1 (BC1). Width and height must
// both be multiples of 4. Output is width*height/2 bytes (8 bytes per 4x4
// block). Alpha is ignored.
func EncodeDXT1(pix []byte, width, height int) ([]byte, error) {
	if width <= 0 || height <= 0 {
		return nil, fmt.Errorf("dxt1: invalid size %dx%d", width, height)
	}
	if width%4 != 0 || height%4 != 0 {
		return nil, fmt.Errorf("dxt1: width/height must be multiples of 4")
	}
	if len(pix) < width*height*4 {
		return nil, fmt.Errorf("dxt1: pixel buffer too small")
	}
	blocksW := width / 4
	blocksH := height / 4
	out := make([]byte, blocksW*blocksH*8)
	for by := 0; by < blocksH; by++ {
		for bx := 0; bx < blocksW; bx++ {
			encodeDXT1Block(pix, bx*4, by*4, width, out[(by*blocksW+bx)*8:])
		}
	}
	return out, nil
}

func encodeDXT1Block(pix []byte, x0, y0, w int, dst []byte) {
	var blk [16][3]byte
	for py := 0; py < 4; py++ {
		row := (y0 + py) * w * 4
		for px := 0; px < 4; px++ {
			off := row + (x0+px)*4
			blk[py*4+px] = [3]byte{pix[off], pix[off+1], pix[off+2]}
		}
	}

	// Pick endpoints as per-channel min/max. Coarse but sufficient for our
	// almost-monochrome shadow maps.
	minC := blk[0]
	maxC := blk[0]
	for i := 1; i < 16; i++ {
		for k := 0; k < 3; k++ {
			if blk[i][k] < minC[k] {
				minC[k] = blk[i][k]
			}
			if blk[i][k] > maxC[k] {
				maxC[k] = blk[i][k]
			}
		}
	}
	c0 := rgbTo565(maxC[0], maxC[1], maxC[2])
	c1 := rgbTo565(minC[0], minC[1], minC[2])
	// Force c0 > c1 so the decoder uses the 4-colour palette (no transparency).
	if c0 < c1 {
		c0, c1 = c1, c0
	}

	var pal [4][3]int
	r, g, b := unpack565(c0)
	pal[0] = [3]int{int(r), int(g), int(b)}
	r, g, b = unpack565(c1)
	pal[1] = [3]int{int(r), int(g), int(b)}
	if c0 != c1 {
		for k := 0; k < 3; k++ {
			pal[2][k] = (2*pal[0][k] + pal[1][k]) / 3
			pal[3][k] = (pal[0][k] + 2*pal[1][k]) / 3
		}
	} else {
		pal[2] = pal[0]
		pal[3] = pal[0]
	}

	var indices uint32
	for i := 0; i < 16; i++ {
		best := 0
		bestD := int(1 << 30)
		for j := 0; j < 4; j++ {
			dr := int(blk[i][0]) - pal[j][0]
			dg := int(blk[i][1]) - pal[j][1]
			db := int(blk[i][2]) - pal[j][2]
			d := dr*dr + dg*dg + db*db
			if d < bestD {
				bestD = d
				best = j
			}
		}
		indices |= uint32(best) << uint(i*2)
	}

	dst[0] = byte(c0)
	dst[1] = byte(c0 >> 8)
	dst[2] = byte(c1)
	dst[3] = byte(c1 >> 8)
	binary.LittleEndian.PutUint32(dst[4:8], indices)
}

func rgbTo565(r, g, b byte) uint16 {
	return uint16(r>>3)<<11 | uint16(g>>2)<<5 | uint16(b>>3)
}

// unpack565 returns 8-bit RGB approximations of an R5G6B5 colour.
func unpack565(v uint16) (byte, byte, byte) {
	r5 := byte((v >> 11) & 0x1F)
	g6 := byte((v >> 5) & 0x3F)
	b5 := byte(v & 0x1F)
	return (r5 << 3) | (r5 >> 2),
		(g6 << 2) | (g6 >> 4),
		(b5 << 3) | (b5 >> 2)
}
