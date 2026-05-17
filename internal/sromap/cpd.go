package sromap

import (
	"fmt"
	"os"
	"strings"
)

type CPD struct {
	Path          string
	Name          string
	CollisionPath string
	Resources     []string
}

func LoadCPD(path string) (*CPD, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	c, err := DecodeCPD(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	c.Path = path
	return c, nil
}

func DecodeCPD(data []byte) (*CPD, error) {
	if len(data) < 12 || !strings.HasPrefix(string(data[:12]), "JMXVCPD") {
		return nil, fmt.Errorf("cpd: bad signature")
	}
	r := NewBinReader(data)
	if err := r.Skip(12); err != nil {
		return nil, err
	}
	collisionOffset, err := r.U32()
	if err != nil {
		return nil, err
	}
	resourceOffset, err := r.U32()
	if err != nil {
		return nil, err
	}

	cpd := &CPD{}
	if collisionOffset > 0 && int(collisionOffset) < len(data) {
		if err := r.Seek(int(collisionOffset)); err == nil {
			if s, err := r.LenString(); err == nil {
				cpd.CollisionPath = s
			}
		}
	}
	if resourceOffset > 0 && int(resourceOffset) < len(data) {
		if err := r.Seek(int(resourceOffset)); err != nil {
			return cpd, nil
		}
		count, err := r.U32()
		if err != nil {
			return cpd, nil
		}
		if count > 1024 {
			return cpd, nil
		}
		for i := uint32(0); i < count; i++ {
			p, err := r.LenString()
			if err != nil {
				break
			}
			cpd.Resources = append(cpd.Resources, p)
		}
	}
	return cpd, nil
}
