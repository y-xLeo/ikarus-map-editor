package editor

import "sromapedit/internal/sromap"

type nvmSeamSide struct {
	side byte
	dx   int
	dy   int
}

var nvmSeamSides = []nvmSeamSide{
	{side: 'W', dx: -1, dy: 0},
	{side: 'E', dx: 1, dy: 0},
	{side: 'S', dx: 0, dy: -1},
	{side: 'N', dx: 0, dy: 1},
}

// syncReciprocalGlobalEdges makes neighbor NVM boundary records match a
// locally rebuilt region. Rebuilding one region can split its side of a
// cross-region edge; the neighbor keeps the old coarse edge unless we rewrite
// its reciprocal GlobalEdges too. Cells and internal topology in the neighbor
// are left untouched.
func (s *Server) syncReciprocalGlobalEdges(rx, ry int, local *sromap.NVM) (int, error) {
	localID := uint16(ry)<<8 | uint16(rx)
	updated := 0
	for _, side := range nvmSeamSides {
		nx, ny := rx+side.dx, ry+side.dy
		if nx < 0 || nx > 255 || ny < 0 || ny > 127 {
			continue
		}
		neighborID := uint16(ny)<<8 | uint16(nx)
		desired := reciprocalEdgesForNeighbor(local, side.side, localID, neighborID)
		if len(desired) == 0 {
			continue
		}
		for _, path := range sromap.ExistingNVMPaths(s.Root, nx, ny) {
			neighbor, err := sromap.LoadNVM(path)
			if err != nil {
				return updated, err
			}
			if !replaceReciprocalEdges(neighbor, oppositeSide(side.side), neighborID, localID, desired) {
				continue
			}
			if err := backupOnce(path); err != nil {
				return updated, err
			}
			if err := neighbor.Save(path); err != nil {
				return updated, err
			}
			_ = s.mirrorToExport(path)
			updated++
		}
	}
	return updated, nil
}

func reciprocalEdgesForNeighbor(local *sromap.NVM, side byte, localID, neighborID uint16) []sromap.NVMGlobalEdge {
	out := make([]sromap.NVMGlobalEdge, 0)
	for _, e := range local.GlobalEdges {
		if !globalEdgeOnLocalSide(e, side, localID, neighborID) {
			continue
		}
		localSlot := localGlobalEdgeSlot(e, localID)
		if localSlot < 0 {
			continue
		}
		localCell := e.Cell0
		neighborCell := e.Cell1
		if localSlot == 1 {
			localCell = e.Cell1
			neighborCell = e.Cell0
		}
		r := e
		r.Region0 = int16(neighborID)
		r.Region1 = int16(localID)
		r.Cell0 = neighborCell
		r.Cell1 = localCell
		r.Dir0 = dirForSide(oppositeSide(side))
		r.Dir1 = dirForSide(side)
		switch side {
		case 'W':
			r.MinX, r.MaxX = float32(sromap.RegionSize), float32(sromap.RegionSize)
		case 'E':
			r.MinX, r.MaxX = 0, 0
		case 'S':
			r.MinZ, r.MaxZ = float32(sromap.RegionSize), float32(sromap.RegionSize)
		case 'N':
			r.MinZ, r.MaxZ = 0, 0
		}
		out = append(out, r)
	}
	return out
}

func replaceReciprocalEdges(nvm *sromap.NVM, side byte, neighborID, localID uint16, desired []sromap.NVMGlobalEdge) bool {
	first := -1
	kept := make([]sromap.NVMGlobalEdge, 0, len(nvm.GlobalEdges))
	current := make([]sromap.NVMGlobalEdge, 0, len(desired))
	for _, e := range nvm.GlobalEdges {
		if globalEdgeOnLocalSide(e, side, neighborID, localID) {
			if first < 0 {
				first = len(kept)
			}
			current = append(current, e)
			continue
		}
		kept = append(kept, e)
	}
	if globalEdgeSlicesEqual(current, desired) {
		return false
	}
	if first < 0 {
		first = len(kept)
	}
	next := make([]sromap.NVMGlobalEdge, 0, len(kept)+len(desired))
	next = append(next, kept[:first]...)
	next = append(next, desired...)
	next = append(next, kept[first:]...)
	nvm.GlobalEdges = next
	return true
}

func globalEdgeOnLocalSide(e sromap.NVMGlobalEdge, side byte, localID, neighborID uint16) bool {
	if !globalEdgeOnBoundarySide(e, side) {
		return false
	}
	hasLocal := (e.Region0 >= 0 && uint16(e.Region0) == localID) || (e.Region1 >= 0 && uint16(e.Region1) == localID)
	hasNeighbor := (e.Region0 >= 0 && uint16(e.Region0) == neighborID) || (e.Region1 >= 0 && uint16(e.Region1) == neighborID)
	return hasLocal && hasNeighbor
}

func globalEdgeOnBoundarySide(e sromap.NVMGlobalEdge, side byte) bool {
	const extent = float32(sromap.RegionSize)
	switch side {
	case 'N':
		return e.MinZ == extent && e.MaxZ == extent
	case 'S':
		return e.MinZ == 0 && e.MaxZ == 0
	case 'E':
		return e.MinX == extent && e.MaxX == extent
	case 'W':
		return e.MinX == 0 && e.MaxX == 0
	default:
		return false
	}
}

func dirForSide(side byte) uint8 {
	switch side {
	case 'N':
		return sromap.NVMDirNorth
	case 'E':
		return sromap.NVMDirEast
	case 'S':
		return sromap.NVMDirSouth
	case 'W':
		return sromap.NVMDirWest
	default:
		return 0xff
	}
}

func globalEdgeSlicesEqual(a, b []sromap.NVMGlobalEdge) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !globalEdgesEqual(a[i], b[i]) {
			return false
		}
	}
	return true
}

func globalEdgesEqual(a, b sromap.NVMGlobalEdge) bool {
	return a.MinX == b.MinX &&
		a.MinZ == b.MinZ &&
		a.MaxX == b.MaxX &&
		a.MaxZ == b.MaxZ &&
		a.Flag == b.Flag &&
		a.Dir0 == b.Dir0 &&
		a.Dir1 == b.Dir1 &&
		a.Cell0 == b.Cell0 &&
		a.Cell1 == b.Cell1 &&
		a.Region0 == b.Region0 &&
		a.Region1 == b.Region1
}
