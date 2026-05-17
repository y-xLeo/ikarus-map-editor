package sromap

import (
	"bufio"
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"strings"
)

// OBJVertex is one unique position+UV+normal combo emitted into our vertex
// stream. The editor's render pipeline only uses XYZ+UV but downstream BMS
// export needs unit normals, so we keep them on the struct and let the
// renderer ignore the trailing fields.
type OBJVertex struct {
	X, Y, Z    float32
	U, V       float32
	NX, NY, NZ float32 // unit normal; (0,0,0) if the OBJ had no `vn` lines
}

// OBJMesh is the fully-resolved indexed mesh from an OBJ file.
type OBJMesh struct {
	Vertices []OBJVertex
	Indices  []uint32
	BBoxMin  [3]float32
	BBoxMax  [3]float32
}

// LoadOBJ parses an OBJ file. Faces with >3 vertices are fan-triangulated.
// Vertex/texcoord/normal indices may be 1-based positive or negative (OBJ
// supports both). Missing /vt or /vn slots are tolerated; if there are no
// texcoords at all, UV is (0,0) for every vertex.
func LoadOBJ(path string) (*OBJMesh, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return ParseOBJ(f)
}

func ParseOBJ(r io.Reader) (*OBJMesh, error) {
	var (
		positions [][3]float32
		uvs       [][2]float32
		normals   [][3]float32
	)
	type combo struct {
		v, t, n int32 // 1-based; 0 means "missing"
	}
	uniqueIdx := make(map[combo]uint32, 16384)
	out := &OBJMesh{}
	bbMin := [3]float32{float32(math.Inf(1)), float32(math.Inf(1)), float32(math.Inf(1))}
	bbMax := [3]float32{float32(math.Inf(-1)), float32(math.Inf(-1)), float32(math.Inf(-1))}

	resolve := func(s string, max int) (int32, error) {
		if s == "" {
			return 0, nil
		}
		n, err := strconv.Atoi(s)
		if err != nil {
			return 0, err
		}
		if n == 0 {
			return 0, nil
		}
		if n < 0 {
			n = max + 1 + n // OBJ relative-from-end index
		}
		if n < 1 || n > max {
			return 0, fmt.Errorf("face index %d out of range (1..%d)", n, max)
		}
		return int32(n), nil
	}

	emit := func(c combo) (uint32, error) {
		if idx, ok := uniqueIdx[c]; ok {
			return idx, nil
		}
		if int(c.v) < 1 || int(c.v) > len(positions) {
			return 0, fmt.Errorf("face references unknown position %d", c.v)
		}
		p := positions[c.v-1]
		var uv [2]float32
		if c.t > 0 && int(c.t) <= len(uvs) {
			uv = uvs[c.t-1]
		}
		var n [3]float32
		if c.n > 0 && int(c.n) <= len(normals) {
			n = normals[c.n-1]
		}
		v := OBJVertex{X: p[0], Y: p[1], Z: p[2], U: uv[0], V: uv[1], NX: n[0], NY: n[1], NZ: n[2]}
		idx := uint32(len(out.Vertices))
		out.Vertices = append(out.Vertices, v)
		uniqueIdx[c] = idx
		return idx, nil
	}

	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 1<<20), 1<<22)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		raw := scanner.Text()
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		switch fields[0] {
		case "v":
			if len(fields) < 4 {
				return nil, fmt.Errorf("line %d: malformed v", lineNo)
			}
			x, _ := strconv.ParseFloat(fields[1], 32)
			y, _ := strconv.ParseFloat(fields[2], 32)
			z, _ := strconv.ParseFloat(fields[3], 32)
			pos := [3]float32{float32(x), float32(y), float32(z)}
			positions = append(positions, pos)
			for i := 0; i < 3; i++ {
				if pos[i] < bbMin[i] {
					bbMin[i] = pos[i]
				}
				if pos[i] > bbMax[i] {
					bbMax[i] = pos[i]
				}
			}
		case "vt":
			if len(fields) < 3 {
				return nil, fmt.Errorf("line %d: malformed vt", lineNo)
			}
			u, _ := strconv.ParseFloat(fields[1], 32)
			v, _ := strconv.ParseFloat(fields[2], 32)
			// OBJ V is bottom-up, GPU UVs are typically top-down; flip here so
			// PNG textures display upright.
			uvs = append(uvs, [2]float32{float32(u), 1 - float32(v)})
		case "vn":
			if len(fields) < 4 {
				return nil, fmt.Errorf("line %d: malformed vn", lineNo)
			}
			x, _ := strconv.ParseFloat(fields[1], 32)
			y, _ := strconv.ParseFloat(fields[2], 32)
			z, _ := strconv.ParseFloat(fields[3], 32)
			normals = append(normals, [3]float32{float32(x), float32(y), float32(z)})
		case "f":
			face := fields[1:]
			if len(face) < 3 {
				return nil, fmt.Errorf("line %d: face has %d vertices", lineNo, len(face))
			}
			combos := make([]combo, 0, len(face))
			for _, tok := range face {
				parts := strings.Split(tok, "/")
				var c combo
				v, err := resolve(parts[0], len(positions))
				if err != nil {
					return nil, fmt.Errorf("line %d: %w", lineNo, err)
				}
				c.v = v
				if len(parts) > 1 {
					t, err := resolve(parts[1], len(uvs))
					if err != nil {
						return nil, fmt.Errorf("line %d: %w", lineNo, err)
					}
					c.t = t
				}
				if len(parts) > 2 {
					n, err := resolve(parts[2], len(normals))
					if err != nil {
						return nil, fmt.Errorf("line %d: %w", lineNo, err)
					}
					c.n = n
				}
				combos = append(combos, c)
			}
			// Fan-triangulate convex polygons; OBJ doesn't guarantee convex
			// but most exporters (including Blender) emit convex faces.
			for i := 1; i < len(combos)-1; i++ {
				a, err := emit(combos[0])
				if err != nil {
					return nil, fmt.Errorf("line %d: %w", lineNo, err)
				}
				b, err := emit(combos[i])
				if err != nil {
					return nil, fmt.Errorf("line %d: %w", lineNo, err)
				}
				cI, err := emit(combos[i+1])
				if err != nil {
					return nil, fmt.Errorf("line %d: %w", lineNo, err)
				}
				out.Indices = append(out.Indices, a, b, cI)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(out.Vertices) == 0 {
		return nil, fmt.Errorf("OBJ has no usable geometry")
	}
	out.BBoxMin = bbMin
	out.BBoxMax = bbMax
	return out, nil
}
