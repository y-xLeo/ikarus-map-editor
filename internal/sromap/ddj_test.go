package sromap

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// build a minimal DXT1 DDJ file: 4x4 image, both endpoints opaque
func makeMinimalDDJ(t *testing.T, fourCC string, blockBytes []byte) []byte {
	t.Helper()
	var b bytes.Buffer

	// 20-byte JMXVDDJ header
	b.WriteString("JMXVDDJ 1000")
	for i := 0; i < 20-12; i++ {
		b.WriteByte(0)
	}
	// DDS magic
	b.WriteString("DDS ")
	// 124-byte DDS header
	header := make([]byte, 124)
	binary.LittleEndian.PutUint32(header[0:4], 124)
	binary.LittleEndian.PutUint32(header[4:8], 0x1007)
	binary.LittleEndian.PutUint32(header[8:12], 4)  // height
	binary.LittleEndian.PutUint32(header[12:16], 4) // width
	binary.LittleEndian.PutUint32(header[72:76], 32)
	binary.LittleEndian.PutUint32(header[76:80], 4)
	copy(header[80:84], fourCC)
	b.Write(header)
	b.Write(blockBytes)
	return b.Bytes()
}

func TestDecodeDDJ_DXT1Solid(t *testing.T) {
	// color0 = pure red (0xF800), color1 = pure red, indices = 0
	block := make([]byte, 8)
	binary.LittleEndian.PutUint16(block[0:2], 0xF800)
	binary.LittleEndian.PutUint16(block[2:4], 0xF800)
	// indices all 0 means pick color0
	data := makeMinimalDDJ(t, "DXT1", block)

	img, err := DecodeDDJ(data)
	if err != nil {
		t.Fatalf("DecodeDDJ: %v", err)
	}
	if img.Width != 4 || img.Height != 4 {
		t.Fatalf("dimensions wrong: %dx%d", img.Width, img.Height)
	}
	// Top-left pixel should be red (~248, 0, 0, 255)
	r := img.RGBA.Pix[0]
	g := img.RGBA.Pix[1]
	bl := img.RGBA.Pix[2]
	a := img.RGBA.Pix[3]
	if r < 240 || g != 0 || bl != 0 || a != 255 {
		t.Errorf("expected red pixel, got rgba=(%d,%d,%d,%d)", r, g, bl, a)
	}
}

func TestDecodeDDJ_BadSignature(t *testing.T) {
	if _, err := DecodeDDJ([]byte("nope")); err == nil {
		t.Fatal("expected error for non-DDJ input")
	}
}
