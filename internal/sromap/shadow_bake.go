package sromap

import "math"

type ShadowParams struct {
	SunDir       [3]float32 // direction toward the sun (need not be normalized)
	LitValue     byte       // brightness for lit texels
	ShadowValue  byte       // brightness for shadowed texels
	RayLength    float32    // max ray distance
	OriginOffset float32    // bump along sun dir to avoid self-shadow
	Samples      int        // rays per texel (1 = hard shadows, 8-16 = soft)
	SunRadius    float32    // half-angle radius of the sun disc, in radians; jitter applied within this cone
}

func DefaultShadowParams() ShadowParams {
	return ShadowParams{
		// Azimuth 75°, elevation 40° — calibrated against the original
		// Silkroad bake by trial-and-error in the editor.
		SunDir:       SunFromAngles(75, 40),
		LitValue:     255,
		ShadowValue:  96,
		RayLength:    3500,
		OriginOffset: 0.6,
		Samples:      1,
		SunRadius:    0.04,
	}
}

// SunFromAngles converts azimuth/elevation in degrees into a direction toward
// the sun. Azimuth 0° points along +Z, increases toward +X. Elevation 0° is
// horizontal, 90° is straight up.
func SunFromAngles(azimuthDeg, elevationDeg float32) [3]float32 {
	az := float64(azimuthDeg) * math.Pi / 180
	el := float64(elevationDeg) * math.Pi / 180
	cosEl := math.Cos(el)
	return [3]float32{
		float32(math.Sin(az) * cosEl),
		float32(math.Sin(el)),
		float32(math.Cos(az) * cosEl),
	}
}

// BakeShadows produces an RGBA lightmap of size width*height. Each output texel
// corresponds to a position on the region's terrain (97x97 height grid,
// upsampled bilinearly). N=Samples rays are cast within a cone of half-angle
// SunRadius around SunDir; the fraction of rays that miss everything becomes
// the texel's brightness (lit→shadow lerp).
func BakeShadows(heights [MeshGridSize * MeshGridSize]float32, bvh *BVH, width, height int, p ShadowParams) []byte {
	if p.Samples < 1 {
		p.Samples = 1
	}
	sun := normalize3(p.SunDir)
	tangentU, tangentV := tangentBasis(sun)

	pix := make([]byte, width*height*4)
	invW := 1.0 / float32(width)
	invH := 1.0 / float32(height)
	gridMax := float32(MeshGridSize - 1)

	// Precompute sample directions on the unit cone around +Z, then transform
	// per-pixel into world space. The first sample is the centre ray; the
	// rest are arranged on a Fibonacci spiral inside the sun disc.
	dirs := sampleConeDirs(p.Samples, p.SunRadius, sun, tangentU, tangentV)

	for py := 0; py < height; py++ {
		v := (float32(py) + 0.5) * invH
		gz := v * gridMax
		izi := int(gz)
		if izi >= MeshGridSize-1 {
			izi = MeshGridSize - 2
		}
		fz := gz - float32(izi)
		row := py * width * 4
		for px := 0; px < width; px++ {
			u := (float32(px) + 0.5) * invW
			gx := u * gridMax
			ixi := int(gx)
			if ixi >= MeshGridSize-1 {
				ixi = MeshGridSize - 2
			}
			fx := gx - float32(ixi)

			h00 := heights[izi*MeshGridSize+ixi]
			h10 := heights[izi*MeshGridSize+ixi+1]
			h01 := heights[(izi+1)*MeshGridSize+ixi]
			h11 := heights[(izi+1)*MeshGridSize+ixi+1]
			h0 := h00*(1-fx) + h10*fx
			h1 := h01*(1-fx) + h11*fx
			h := h0*(1-fz) + h1*fz

			origin := [3]float32{
				gx*float32(CellSize) + sun[0]*p.OriginOffset,
				h + sun[1]*p.OriginOffset + 0.5,
				gz*float32(CellSize) + sun[2]*p.OriginOffset,
			}

			litCount := 0
			for _, d := range dirs {
				if !bvh.AnyHit(origin, d, p.RayLength) {
					litCount++
				}
			}
			frac := float32(litCount) / float32(len(dirs))
			val := byte(float32(p.ShadowValue) + frac*(float32(p.LitValue)-float32(p.ShadowValue)))
			off := row + px*4
			pix[off] = val
			pix[off+1] = val
			pix[off+2] = val
			pix[off+3] = 255
		}
	}
	return pix
}

func sampleConeDirs(samples int, radius float32, sun, tangentU, tangentV [3]float32) [][3]float32 {
	if samples <= 1 {
		return [][3]float32{sun}
	}
	dirs := make([][3]float32, 0, samples)
	dirs = append(dirs, sun)
	// Sunflower (Vogel) pattern inside the unit disc.
	const phi = 2.39996322972865332
	for i := 1; i < samples; i++ {
		t := float32(i) / float32(samples-1)
		r := float32(math.Sqrt(float64(t))) * radius
		a := float64(i) * phi
		du := r * float32(math.Cos(a))
		dv := r * float32(math.Sin(a))
		raw := [3]float32{
			sun[0] + tangentU[0]*du + tangentV[0]*dv,
			sun[1] + tangentU[1]*du + tangentV[1]*dv,
			sun[2] + tangentU[2]*du + tangentV[2]*dv,
		}
		dirs = append(dirs, normalize3(raw))
	}
	return dirs
}

// tangentBasis returns two unit vectors perpendicular to n.
func tangentBasis(n [3]float32) ([3]float32, [3]float32) {
	up := [3]float32{0, 1, 0}
	if math.Abs(float64(n[1])) > 0.9 {
		up = [3]float32{1, 0, 0}
	}
	tu := normalize3([3]float32{
		up[1]*n[2] - up[2]*n[1],
		up[2]*n[0] - up[0]*n[2],
		up[0]*n[1] - up[1]*n[0],
	})
	tv := [3]float32{
		n[1]*tu[2] - n[2]*tu[1],
		n[2]*tu[0] - n[0]*tu[2],
		n[0]*tu[1] - n[1]*tu[0],
	}
	return tu, tv
}

func normalize3(v [3]float32) [3]float32 {
	length := float32(math.Sqrt(float64(v[0]*v[0] + v[1]*v[1] + v[2]*v[2])))
	if length == 0 {
		return v
	}
	return [3]float32{v[0] / length, v[1] / length, v[2] / length}
}
