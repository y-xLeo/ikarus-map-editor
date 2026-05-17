package sromap

import "math"

// RegionTransform describes an axis-aligned region-content remapping used by
// the paste-with-rotation feature. Rotation is applied first (0/90/180/270
// degrees clockwise in the top-down view), then flipX, then flipZ.
//
// Yaw convention here: the client matrix uses local +Z as the object's
// "facing" direction, with yaw measured clockwise from +Z in the top-down
// view. So a world CW rotation increases yaw by the same angle.
type RegionTransform struct {
	Rotation int  // 0, 90, 180, 270 (degrees CW)
	FlipX    bool // mirror across the vertical axis (swap +X / -X)
	FlipZ    bool // mirror across the horizontal axis (swap +Z / -Z)
}

// IsIdentity returns true when applying this transform is a no-op.
func (t RegionTransform) IsIdentity() bool {
	return t.Rotation == 0 && !t.FlipX && !t.FlipZ
}

// TransformGrid returns the (nx, nz) where the value at input (gx, gz) should
// end up, given a grid sized [gridSize × gridSize] (so the largest valid
// coordinate is gridSize-1).
func (t RegionTransform) TransformGrid(gx, gz, gridSize int) (int, int) {
	last := gridSize - 1
	var nx, nz int
	switch t.Rotation {
	case 90:
		nx, nz = gz, last-gx
	case 180:
		nx, nz = last-gx, last-gz
	case 270:
		nx, nz = last-gz, gx
	default:
		nx, nz = gx, gz
	}
	if t.FlipX {
		nx = last - nx
	}
	if t.FlipZ {
		nz = last - nz
	}
	return nx, nz
}

// TransformPos rotates/flips a continuous local-region position (units in
// [0, regionSize]) around the region's center.
func (t RegionTransform) TransformPos(x, z, regionSize float32) (float32, float32) {
	half := regionSize / 2
	rx := x - half
	rz := z - half
	var nrx, nrz float32
	switch t.Rotation {
	case 90:
		nrx, nrz = rz, -rx
	case 180:
		nrx, nrz = -rx, -rz
	case 270:
		nrx, nrz = -rz, rx
	default:
		nrx, nrz = rx, rz
	}
	if t.FlipX {
		nrx = -nrx
	}
	if t.FlipZ {
		nrz = -nrz
	}
	return nrx + half, nrz + half
}

// TransformBounds rotates/flips an axis-aligned bbox in local-region coords.
// All four corners are mapped through TransformPos and a new normalised bbox
// is returned.
func (t RegionTransform) TransformBounds(minX, minZ, maxX, maxZ, regionSize float32) (float32, float32, float32, float32) {
	xs := [4]float32{}
	zs := [4]float32{}
	xs[0], zs[0] = t.TransformPos(minX, minZ, regionSize)
	xs[1], zs[1] = t.TransformPos(maxX, minZ, regionSize)
	xs[2], zs[2] = t.TransformPos(minX, maxZ, regionSize)
	xs[3], zs[3] = t.TransformPos(maxX, maxZ, regionSize)
	nMinX, nMaxX := xs[0], xs[0]
	nMinZ, nMaxZ := zs[0], zs[0]
	for i := 1; i < 4; i++ {
		if xs[i] < nMinX {
			nMinX = xs[i]
		}
		if xs[i] > nMaxX {
			nMaxX = xs[i]
		}
		if zs[i] < nMinZ {
			nMinZ = zs[i]
		}
		if zs[i] > nMaxZ {
			nMaxZ = zs[i]
		}
	}
	return nMinX, nMinZ, nMaxX, nMaxZ
}

// TransformYaw maps a yaw angle (radians, clockwise from +Z when viewed
// top-down) through the transform.
func (t RegionTransform) TransformYaw(yaw float32) float32 {
	y := float64(yaw)
	switch t.Rotation {
	case 90:
		y += math.Pi / 2
	case 180:
		y += math.Pi
	case 270:
		y -= math.Pi / 2
	}
	if t.FlipX {
		y = -y
	}
	if t.FlipZ {
		y = math.Pi - y
	}
	// Wrap to (-π, π] to keep numbers tidy.
	y = math.Mod(y+math.Pi, 2*math.Pi)
	if y <= 0 {
		y += 2 * math.Pi
	}
	y -= math.Pi
	return float32(y)
}

// PermuteFloat32Grid returns a new slice with `in` transformed under t,
// using the supplied side length (e.g. MeshGridSize for heights). Panics
// if len(in) != size*size.
func (t RegionTransform) PermuteFloat32Grid(in []float32, size int) []float32 {
	if len(in) != size*size {
		panic("PermuteFloat32Grid: size mismatch")
	}
	if t.IsIdentity() {
		out := make([]float32, len(in))
		copy(out, in)
		return out
	}
	out := make([]float32, size*size)
	for gz := 0; gz < size; gz++ {
		for gx := 0; gx < size; gx++ {
			nx, nz := t.TransformGrid(gx, gz, size)
			out[nz*size+nx] = in[gz*size+gx]
		}
	}
	return out
}

// PermuteUint16Grid mirrors PermuteFloat32Grid for uint16 grids (tile IDs).
func (t RegionTransform) PermuteUint16Grid(in []uint16, size int) []uint16 {
	if len(in) != size*size {
		panic("PermuteUint16Grid: size mismatch")
	}
	if t.IsIdentity() {
		out := make([]uint16, len(in))
		copy(out, in)
		return out
	}
	out := make([]uint16, size*size)
	for gz := 0; gz < size; gz++ {
		for gx := 0; gx < size; gx++ {
			nx, nz := t.TransformGrid(gx, gz, size)
			out[nz*size+nx] = in[gz*size+gx]
		}
	}
	return out
}

// PermuteByteGrid mirrors PermuteFloat32Grid for byte grids (NVM plane type).
func (t RegionTransform) PermuteByteGrid(in []byte, size int) []byte {
	if len(in) != size*size {
		panic("PermuteByteGrid: size mismatch")
	}
	if t.IsIdentity() {
		out := make([]byte, len(in))
		copy(out, in)
		return out
	}
	out := make([]byte, size*size)
	for gz := 0; gz < size; gz++ {
		for gx := 0; gx < size; gx++ {
			nx, nz := t.TransformGrid(gx, gz, size)
			out[nz*size+nx] = in[gz*size+gx]
		}
	}
	return out
}
