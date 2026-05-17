package sromap

import (
	"encoding/binary"
	"fmt"
	"image"
	"os"
)

const (
	ddjSignature  = "JMXVDDJ"
	ddjHeaderSize = 20
	ddsHeaderSize = 128
)

type DDJImage struct {
	Width  int
	Height int
	RGBA   *image.RGBA
	Format string
}

func LoadDDJ(path string) (*DDJImage, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return DecodeDDJ(data)
}

func DecodeDDJ(data []byte) (*DDJImage, error) {
	offset := 0
	switch {
	case len(data) >= 4 && string(data[:4]) == "DDS ":
	case len(data) >= ddjHeaderSize && string(data[:7]) == ddjSignature:
		offset = ddjHeaderSize
	default:
		return nil, fmt.Errorf("not a DDJ/DDS file")
	}
	if len(data) < offset+ddsHeaderSize {
		return nil, fmt.Errorf("ddj: file truncated before DDS header")
	}
	if string(data[offset:offset+4]) != "DDS " {
		return nil, fmt.Errorf("ddj: bad DDS magic")
	}
	header := data[offset+4 : offset+ddsHeaderSize]
	height := int(binary.LittleEndian.Uint32(header[8:12]))
	width := int(binary.LittleEndian.Uint32(header[12:16]))
	fourCC := string(header[80:84])

	if width <= 0 || height <= 0 || width > 8192 || height > 8192 {
		return nil, fmt.Errorf("ddj: invalid dimensions %dx%d", width, height)
	}

	pixelData := data[offset+ddsHeaderSize:]
	rgba := image.NewRGBA(image.Rect(0, 0, width, height))

	switch fourCC {
	case "DXT1":
		if err := decodeDXT1(pixelData, width, height, rgba.Pix); err != nil {
			return nil, err
		}
	case "DXT3":
		if err := decodeDXT3(pixelData, width, height, rgba.Pix); err != nil {
			return nil, err
		}
	case "DXT5":
		if err := decodeDXT5(pixelData, width, height, rgba.Pix); err != nil {
			return nil, err
		}
	default:
		flags := binary.LittleEndian.Uint32(header[76:80])
		if flags&0x40 != 0 && fourCC[0] == 0 {
			if err := decodeUncompressed(pixelData, width, height, header, rgba.Pix); err != nil {
				return nil, err
			}
		} else {
			return nil, fmt.Errorf("ddj: unsupported format %q", fourCC)
		}
	}
	return &DDJImage{Width: width, Height: height, RGBA: rgba, Format: fourCC}, nil
}

func decodeDXT1(src []byte, width, height int, dst []byte) error {
	if err := checkBlocks(src, width, height, 8); err != nil {
		return err
	}
	blocks := (width + 3) / 4
	rows := (height + 3) / 4
	for by := 0; by < rows; by++ {
		for bx := 0; bx < blocks; bx++ {
			off := (by*blocks + bx) * 8
			decodeBC1Block(src[off:off+8], bx*4, by*4, width, height, dst, false)
		}
	}
	return nil
}

func decodeDXT3(src []byte, width, height int, dst []byte) error {
	if err := checkBlocks(src, width, height, 16); err != nil {
		return err
	}
	blocks := (width + 3) / 4
	rows := (height + 3) / 4
	for by := 0; by < rows; by++ {
		for bx := 0; bx < blocks; bx++ {
			off := (by*blocks + bx) * 16
			alphas := src[off : off+8]
			decodeBC1Block(src[off+8:off+16], bx*4, by*4, width, height, dst, true)
			for py := 0; py < 4; py++ {
				row := uint16(alphas[py*2]) | uint16(alphas[py*2+1])<<8
				for px := 0; px < 4; px++ {
					x := bx*4 + px
					y := by*4 + py
					if x >= width || y >= height {
						continue
					}
					a4 := byte((row >> uint(px*4)) & 0xF)
					dst[(y*width+x)*4+3] = (a4 << 4) | a4
				}
			}
		}
	}
	return nil
}

func decodeDXT5(src []byte, width, height int, dst []byte) error {
	if err := checkBlocks(src, width, height, 16); err != nil {
		return err
	}
	blocks := (width + 3) / 4
	rows := (height + 3) / 4
	for by := 0; by < rows; by++ {
		for bx := 0; bx < blocks; bx++ {
			off := (by*blocks + bx) * 16
			a0 := src[off]
			a1 := src[off+1]
			var alpha [8]uint16
			alpha[0] = uint16(a0)
			alpha[1] = uint16(a1)
			if a0 > a1 {
				for i := 1; i < 7; i++ {
					alpha[i+1] = (uint16(7-i)*uint16(a0) + uint16(i)*uint16(a1)) / 7
				}
			} else {
				for i := 1; i < 5; i++ {
					alpha[i+1] = (uint16(5-i)*uint16(a0) + uint16(i)*uint16(a1)) / 5
				}
				alpha[6] = 0
				alpha[7] = 255
			}
			var bits uint64
			for i := 0; i < 6; i++ {
				bits |= uint64(src[off+2+i]) << uint(i*8)
			}
			decodeBC1Block(src[off+8:off+16], bx*4, by*4, width, height, dst, true)
			for py := 0; py < 4; py++ {
				for px := 0; px < 4; px++ {
					x := bx*4 + px
					y := by*4 + py
					if x >= width || y >= height {
						continue
					}
					idx := (bits >> uint((py*4+px)*3)) & 0x7
					dst[(y*width+x)*4+3] = byte(alpha[idx])
				}
			}
		}
	}
	return nil
}

func decodeBC1Block(block []byte, x0, y0, width, height int, dst []byte, hasExplicitAlpha bool) {
	c0 := uint16(block[0]) | uint16(block[1])<<8
	c1 := uint16(block[2]) | uint16(block[3])<<8
	var palette [4][4]byte
	palette[0] = rgb565To8(c0)
	palette[1] = rgb565To8(c1)
	palette[0][3] = 255
	palette[1][3] = 255
	if c0 > c1 || hasExplicitAlpha {
		for i := 0; i < 3; i++ {
			palette[2][i] = byte((uint16(palette[0][i])*2 + uint16(palette[1][i])) / 3)
			palette[3][i] = byte((uint16(palette[0][i]) + uint16(palette[1][i])*2) / 3)
		}
		palette[2][3] = 255
		palette[3][3] = 255
	} else {
		for i := 0; i < 3; i++ {
			palette[2][i] = byte((uint16(palette[0][i]) + uint16(palette[1][i])) / 2)
			palette[3][i] = 0
		}
		palette[2][3] = 255
		palette[3][3] = 0
	}
	indices := uint32(block[4]) | uint32(block[5])<<8 | uint32(block[6])<<16 | uint32(block[7])<<24
	for py := 0; py < 4; py++ {
		for px := 0; px < 4; px++ {
			x := x0 + px
			y := y0 + py
			if x >= width || y >= height {
				continue
			}
			idx := (indices >> uint((py*4+px)*2)) & 0x3
			off := (y*width + x) * 4
			dst[off] = palette[idx][0]
			dst[off+1] = palette[idx][1]
			dst[off+2] = palette[idx][2]
			if !hasExplicitAlpha {
				dst[off+3] = palette[idx][3]
			}
		}
	}
}

func rgb565To8(v uint16) [4]byte {
	r := byte(v >> 11 & 0x1F)
	g := byte(v >> 5 & 0x3F)
	b := byte(v & 0x1F)
	return [4]byte{
		(r << 3) | (r >> 2),
		(g << 2) | (g >> 4),
		(b << 3) | (b >> 2),
		0,
	}
}

func decodeUncompressed(src []byte, width, height int, header []byte, dst []byte) error {
	bpp := int(binary.LittleEndian.Uint32(header[84:88]))
	if bpp != 32 && bpp != 24 {
		return fmt.Errorf("ddj: unsupported uncompressed bpp %d", bpp)
	}
	rMask := binary.LittleEndian.Uint32(header[88:92])
	gMask := binary.LittleEndian.Uint32(header[92:96])
	bMask := binary.LittleEndian.Uint32(header[96:100])
	aMask := binary.LittleEndian.Uint32(header[100:104])
	stride := bpp / 8
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			off := (y*width + x) * stride
			if off+stride > len(src) {
				return fmt.Errorf("ddj: truncated uncompressed pixels")
			}
			var pixel uint32
			for i := 0; i < stride; i++ {
				pixel |= uint32(src[off+i]) << uint(i*8)
			}
			r := extractChannel(pixel, rMask)
			g := extractChannel(pixel, gMask)
			b := extractChannel(pixel, bMask)
			a := byte(255)
			if aMask != 0 && bpp == 32 {
				a = extractChannel(pixel, aMask)
			}
			dOff := (y*width + x) * 4
			dst[dOff] = r
			dst[dOff+1] = g
			dst[dOff+2] = b
			dst[dOff+3] = a
		}
	}
	return nil
}

func extractChannel(pixel, mask uint32) byte {
	if mask == 0 {
		return 0
	}
	shift := 0
	for mask&1 == 0 {
		mask >>= 1
		shift++
	}
	maxValue := mask
	value := (pixel >> uint(shift)) & maxValue
	if maxValue == 0 {
		return 0
	}
	return byte((value * 255) / maxValue)
}

func checkBlocks(src []byte, width, height, blockBytes int) error {
	expected := ((width + 3) / 4) * ((height + 3) / 4) * blockBytes
	if len(src) < expected {
		return fmt.Errorf("ddj: pixel data %d bytes, need %d", len(src), expected)
	}
	return nil
}
