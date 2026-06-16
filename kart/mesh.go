package kart

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"

	"hsdemo/kmdata"
)

// Unity built-in mesh IDs are serialized as guid "0" plus a stable fileID.
// BuiltToScaleDS currently uses Plane for the scrolling floor grid and Cube/
// Capsule-like thin dividers. Imported FBX geometry still needs real vertex
// extraction; keeping that unsupported here makes missing 3D data explicit.
func builtinMeshFootprint(ref kmdata.AssetRef) (w, h float64, ok bool) {
	if ref.GUID != "" && ref.GUID != "0" {
		return 0, 0, false
	}
	switch ref.FileID {
	case 10209: // Plane: Unity's built-in plane is 10x10 units.
		return 10, 10, true
	case 10206, 10202: // Cube/Capsule stand-ins used by thin divider meshes.
		return 1, 1, true
	default:
		return 0, 0, false
	}
}

func (s *SceneInst) meshTint(node int, b *kmdata.MeshBinding) [4]float64 {
	tint := [4]float64{1, 1, 1, 1}
	if len(b.Materials) > 0 {
		if mat, ok := s.as.Meshes.Materials[b.Materials[0].GUID]; ok {
			if c, ok := mat.Colors["_Color"]; ok {
				tint = c
			}
		}
	}
	st := &s.state[node]
	if st.hasMatColor {
		tint = st.matColor
	}
	tint[3] *= st.matAlpha * st.matOpacity
	return tint
}

func (s *SceneInst) meshDrawable(bindingIdx int) (nodeIdx int, tint [4]float64, ok bool) {
	b := &s.as.Meshes.Bindings[bindingIdx]
	i, ok := s.byPath[b.Path]
	if !ok || b.Renderer != "MeshRenderer" || !b.Enabled || !s.actives[i] || !s.state[i].renderOn {
		return 0, [4]float64{}, false
	}
	if _, _, ok := builtinMeshFootprint(b.Mesh); !ok {
		return 0, [4]float64{}, false
	}
	tint = s.meshTint(i, b)
	if tint[3] <= 0 {
		return 0, [4]float64{}, false
	}
	return i, tint, true
}

func (s *SceneInst) drawMeshBinding(dst *ebiten.Image, bindingIdx, nodeIdx int, world, proj Aff) {
	b := &s.as.Meshes.Bindings[bindingIdx]
	w, h, ok := builtinMeshFootprint(b.Mesh)
	if !ok {
		return
	}
	drawSolidQuad(dst, proj.Mul(world), w, h, s.meshTint(nodeIdx, b))
}

func drawSolidQuad(dst *ebiten.Image, m Aff, w, h float64, tint [4]float64) {
	if tint[3] <= 0 || w <= 0 || h <= 0 {
		return
	}
	x0, y0 := -w/2, -h/2
	x1, y1 := w/2, h/2
	points := [4][2]float64{{x0, y0}, {x1, y0}, {x1, y1}, {x0, y1}}
	var vs [4]ebiten.Vertex
	for i, p := range points {
		x, y := m.Apply(p[0], p[1])
		vs[i] = ebiten.Vertex{
			DstX:   float32(x),
			DstY:   float32(y),
			SrcX:   1,
			SrcY:   1,
			ColorR: float32(tint[0]),
			ColorG: float32(tint[1]),
			ColorB: float32(tint[2]),
			ColorA: float32(tint[3]),
		}
	}
	dst.DrawTriangles(vs[:], []uint16{0, 1, 2, 0, 2, 3}, meshWhitePixel(), &ebiten.DrawTrianglesOptions{AntiAlias: true})
}

var meshWhite *ebiten.Image

func meshWhitePixel() *ebiten.Image {
	if meshWhite == nil {
		meshWhite = ebiten.NewImage(3, 3)
		meshWhite.Fill(color.White)
	}
	return meshWhite
}
