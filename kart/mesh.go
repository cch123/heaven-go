package kart

import (
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"

	"hsdemo/kmdata"
)

// Unity built-in mesh IDs are serialized as guid "0" plus a stable fileID.
// BuiltToScaleDS currently uses Plane for the scrolling floor grid and Cube/
// Capsule-like thin dividers. Imported FBX meshes use extracted geometry when
// a guid has exactly one Geometry; multi-geometry submesh matching remains
// explicit work so we do not attach the wrong mesh to a renderer.
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
	if _, _, ok := builtinMeshFootprint(b.Mesh); !ok && s.meshGeometry(b) == nil {
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
	if ok {
		drawSolidQuad(dst, proj.Mul(world), w, h, s.meshTint(nodeIdx, b))
		return
	}
	if g := s.meshGeometry(b); g != nil {
		tex, env := s.meshTexture(b)
		drawMeshGeometry(dst, proj.Mul(world), g, s.meshTint(nodeIdx, b), tex, env)
	}
}

func (s *SceneInst) meshGeometry(b *kmdata.MeshBinding) *kmdata.MeshGeometry {
	if b.Mesh.GUID == "" || b.Mesh.GUID == "0" {
		return nil
	}
	geoms := s.as.Meshes.Geometries[b.Mesh.GUID]
	if len(geoms) == 1 {
		return &geoms[0]
	}
	// New Unity fileID generation does not expose a stable name table in .meta.
	// Until exact submesh matching is implemented, avoid guessing when an FBX
	// contains several Geometry nodes.
	return nil
}

func (s *SceneInst) meshTexture(b *kmdata.MeshBinding) (*ebiten.Image, kmdata.TextureEnv) {
	if len(b.Materials) == 0 {
		return nil, kmdata.TextureEnv{}
	}
	mat, ok := s.as.Meshes.Materials[b.Materials[0].GUID]
	if !ok {
		return nil, kmdata.TextureEnv{}
	}
	env, ok := mat.Textures["_MainTex"]
	if !ok || env.Image == "" || s.as.MeshTex == nil {
		return nil, kmdata.TextureEnv{}
	}
	return s.as.MeshTex[env.Image], env
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

func drawMeshGeometry(dst *ebiten.Image, m Aff, g *kmdata.MeshGeometry, tint [4]float64, tex *ebiten.Image, env kmdata.TextureEnv) {
	if tint[3] <= 0 || len(g.Vertices) == 0 || len(g.Indices) < 3 || len(g.Vertices) > 65535 {
		return
	}
	if tex != nil && len(g.UVs) > 0 && len(g.UVIndices) == len(g.Indices) {
		drawTexturedMeshGeometry(dst, m, g, tint, tex, env)
		return
	}
	vs := make([]ebiten.Vertex, len(g.Vertices))
	for i, p := range g.Vertices {
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
	is := make([]uint16, 0, len(g.Indices))
	for _, idx := range g.Indices {
		if idx < 0 || idx >= len(vs) {
			return
		}
		is = append(is, uint16(idx))
	}
	dst.DrawTriangles(vs, is, meshWhitePixel(), &ebiten.DrawTrianglesOptions{AntiAlias: true})
}

func drawTexturedMeshGeometry(dst *ebiten.Image, m Aff, g *kmdata.MeshGeometry, tint [4]float64, tex *ebiten.Image, env kmdata.TextureEnv) {
	if len(g.Indices) > 65535 {
		return
	}
	b := tex.Bounds()
	w, h := float64(b.Dx()), float64(b.Dy())
	scale := env.Scale
	if scale == [2]float64{} {
		scale = [2]float64{1, 1}
	}
	vs := make([]ebiten.Vertex, len(g.Indices))
	is := make([]uint16, len(g.Indices))
	for i, vi := range g.Indices {
		if vi < 0 || vi >= len(g.Vertices) {
			return
		}
		ui := g.UVIndices[i]
		if ui < 0 || ui >= len(g.UVs) {
			return
		}
		p := g.Vertices[vi]
		uv := g.UVs[ui]
		u := wrap01(uv[0]*scale[0] + env.Offset[0])
		v := wrap01(uv[1]*scale[1] + env.Offset[1])
		x, y := m.Apply(p[0], p[1])
		vs[i] = ebiten.Vertex{
			DstX:   float32(x),
			DstY:   float32(y),
			SrcX:   float32(u*w) + float32(b.Min.X),
			SrcY:   float32((1-v)*h) + float32(b.Min.Y),
			ColorR: float32(tint[0]),
			ColorG: float32(tint[1]),
			ColorB: float32(tint[2]),
			ColorA: float32(tint[3]),
		}
		is[i] = uint16(i)
	}
	dst.DrawTriangles(vs, is, tex, &ebiten.DrawTrianglesOptions{AntiAlias: true})
}

func wrap01(v float64) float64 {
	v = math.Mod(v, 1)
	if v < 0 {
		v += 1
	}
	return v
}

var meshWhite *ebiten.Image

func meshWhitePixel() *ebiten.Image {
	if meshWhite == nil {
		meshWhite = ebiten.NewImage(3, 3)
		meshWhite.Fill(color.White)
	}
	return meshWhite
}
