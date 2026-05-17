package sromap

import (
	"encoding/binary"
	"fmt"
	"math"
)

type BinReader struct {
	Data []byte
	Pos  int
}

func NewBinReader(data []byte) *BinReader {
	return &BinReader{Data: data}
}

func (r *BinReader) Remaining() int { return len(r.Data) - r.Pos }

func (r *BinReader) Skip(n int) error {
	if n < 0 || r.Pos+n > len(r.Data) {
		return fmt.Errorf("bin: cannot skip %d at pos %d (len %d)", n, r.Pos, len(r.Data))
	}
	r.Pos += n
	return nil
}

func (r *BinReader) Seek(offset int) error {
	if offset < 0 || offset > len(r.Data) {
		return fmt.Errorf("bin: cannot seek to %d (len %d)", offset, len(r.Data))
	}
	r.Pos = offset
	return nil
}

func (r *BinReader) U8() (byte, error) {
	if r.Pos+1 > len(r.Data) {
		return 0, fmt.Errorf("bin: EOF at %d (u8)", r.Pos)
	}
	v := r.Data[r.Pos]
	r.Pos++
	return v, nil
}

func (r *BinReader) U16() (uint16, error) {
	if r.Pos+2 > len(r.Data) {
		return 0, fmt.Errorf("bin: EOF at %d (u16)", r.Pos)
	}
	v := binary.LittleEndian.Uint16(r.Data[r.Pos : r.Pos+2])
	r.Pos += 2
	return v, nil
}

func (r *BinReader) U32() (uint32, error) {
	if r.Pos+4 > len(r.Data) {
		return 0, fmt.Errorf("bin: EOF at %d (u32)", r.Pos)
	}
	v := binary.LittleEndian.Uint32(r.Data[r.Pos : r.Pos+4])
	r.Pos += 4
	return v, nil
}

func (r *BinReader) F32() (float32, error) {
	v, err := r.U32()
	if err != nil {
		return 0, err
	}
	return math.Float32frombits(v), nil
}

func (r *BinReader) ASCII(n int) (string, error) {
	if n < 0 || r.Pos+n > len(r.Data) {
		return "", fmt.Errorf("bin: ASCII length %d at pos %d overflows", n, r.Pos)
	}
	s := string(r.Data[r.Pos : r.Pos+n])
	r.Pos += n
	return s, nil
}

func (r *BinReader) LenString() (string, error) {
	length, err := r.U32()
	if err != nil {
		return "", err
	}
	if length == 0 {
		return "", nil
	}
	if length > 1<<14 {
		return "", fmt.Errorf("bin: implausible string length %d at pos %d", length, r.Pos)
	}
	return r.ASCII(int(length))
}
