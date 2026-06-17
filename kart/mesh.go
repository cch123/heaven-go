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
	case 10207: // Sphere: draw as a projected disc for 2D Ebitengine output.
		return 1, 1, true
	case 10209: // Plane: Unity's built-in plane is 10x10 units.
		return 10, 10, true
	case 10206, 10202: // Cube/Capsule stand-ins used by thin divider meshes.
		return 1, 1, true
	default:
		return 0, 0, false
	}
}

func (s *SceneInst) meshMaterialTint(b *kmdata.MeshBinding) [4]float64 {
	tint := [4]float64{1, 1, 1, 1}
	if len(b.Materials) > 0 {
		if mat, ok := s.as.Meshes.Materials[b.Materials[0].GUID]; ok {
			if c, ok := mat.Colors["_Color"]; ok {
				tint = c
			}
			if st, ok := s.matFor[mat.Name]; ok && st.hasColor {
				tint = st.color
			}
		}
	}
	return tint
}

func meshStateTint(base [4]float64, st *sceneNodeState) [4]float64 {
	tint := base
	if st.hasMatColor {
		tint = st.matColor
	}
	tint[3] *= st.matAlpha * st.matOpacity
	return tint
}

func (s *SceneInst) meshTint(node int, b *kmdata.MeshBinding) [4]float64 {
	return meshStateTint(s.meshMaterialTint(b), &s.state[node])
}

func (s *SceneInst) meshRenderable(bindingIdx int) bool {
	if bindingIdx < 0 || bindingIdx >= len(s.as.Meshes.Bindings) {
		return false
	}
	b := &s.as.Meshes.Bindings[bindingIdx]
	if b.Renderer != "MeshRenderer" || !b.Enabled {
		return false
	}
	if _, _, ok := builtinMeshFootprint(b.Mesh); ok {
		return true
	}
	return s.meshGeometry(b) != nil
}

func (s *SceneInst) meshDrawable(bindingIdx int) (nodeIdx int, tint [4]float64, ok bool) {
	b := &s.as.Meshes.Bindings[bindingIdx]
	i, ok := s.byPath[b.Path]
	if !ok || !s.meshRenderable(bindingIdx) || !s.actives[i] || !s.state[i].renderOn {
		return 0, [4]float64{}, false
	}
	tint = s.meshTint(i, b)
	if tint[3] <= 0 {
		return 0, [4]float64{}, false
	}
	return i, tint, true
}

func (s *SceneInst) drawMeshBinding(dst *ebiten.Image, bindingIdx, nodeIdx int, world, proj Aff) {
	s.drawMeshBindingTinted(dst, bindingIdx, world, proj, s.meshTint(nodeIdx, &s.as.Meshes.Bindings[bindingIdx]))
}

func (s *SceneInst) drawMeshBindingTinted(dst *ebiten.Image, bindingIdx int, world, proj Aff, tint [4]float64) {
	b := &s.as.Meshes.Bindings[bindingIdx]
	w, h, ok := builtinMeshFootprint(b.Mesh)
	if ok {
		if b.Mesh.FileID == 10207 {
			drawSolidEllipse(dst, proj.Mul(world), w/2, h/2, tint)
			return
		}
		tex, env := s.meshTexture(b)
		if tex != nil {
			drawTexturedBuiltinQuad(dst, proj.Mul(world), w, h, tint, tex, env)
			return
		}
		drawSolidQuad(dst, proj.Mul(world), w, h, tint)
		return
	}
	if g := s.meshGeometry(b); g != nil {
		tex, env := s.meshTexture(b)
		drawMeshGeometry(dst, proj.Mul(world), g, tint, tex, env)
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
	if off, ok := s.texFor[mat.Name]; ok {
		env.Offset = off
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

func drawSolidEllipse(dst *ebiten.Image, m Aff, rx, ry float64, tint [4]float64) {
	if tint[3] <= 0 || rx <= 0 || ry <= 0 {
		return
	}
	const segments = 32
	vs := make([]ebiten.Vertex, segments+1)
	cx, cy := m.Apply(0, 0)
	vs[0] = ebiten.Vertex{
		DstX:   float32(cx),
		DstY:   float32(cy),
		SrcX:   1,
		SrcY:   1,
		ColorR: float32(tint[0]),
		ColorG: float32(tint[1]),
		ColorB: float32(tint[2]),
		ColorA: float32(tint[3]),
	}
	for i := 0; i < segments; i++ {
		a := 2 * math.Pi * float64(i) / segments
		x, y := m.Apply(math.Cos(a)*rx, math.Sin(a)*ry)
		vs[i+1] = ebiten.Vertex{
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
	idx := make([]uint16, 0, segments*3)
	for i := 0; i < segments; i++ {
		next := i + 2
		if next > segments {
			next = 1
		}
		idx = append(idx, 0, uint16(i+1), uint16(next))
	}
	dst.DrawTriangles(vs, idx, meshWhitePixel(), &ebiten.DrawTrianglesOptions{AntiAlias: true})
}

func drawTexturedBuiltinQuad(dst *ebiten.Image, m Aff, w, h float64, tint [4]float64, tex *ebiten.Image, env kmdata.TextureEnv) {
	if tint[3] <= 0 || w <= 0 || h <= 0 || tex == nil {
		return
	}
	scale := env.Scale
	if scale == [2]float64{} {
		scale = [2]float64{1, 1}
	}
	// Unity repeats built-in mesh UVs when material tiling is above 1. Split the
	// quad on tile boundaries so Ebitengine never interpolates across the wrap
	// discontinuity from u/v=1 back to 0.
	ux := repeatBreaks(env.Offset[0], env.Offset[0]+scale[0])
	vy := repeatBreaks(env.Offset[1], env.Offset[1]+scale[1])
	if len(ux) < 2 || len(vy) < 2 {
		return
	}
	b := tex.Bounds()
	tw, th := float64(b.Dx()), float64(b.Dy())
	vs := make([]ebiten.Vertex, 0, (len(ux)-1)*(len(vy)-1)*4)
	is := make([]uint16, 0, (len(ux)-1)*(len(vy)-1)*6)
	for yi := 0; yi+1 < len(vy); yi++ {
		for xi := 0; xi+1 < len(ux); xi++ {
			if len(vs)+4 > math.MaxUint16 {
				return
			}
			u0, u1 := ux[xi], ux[xi+1]
			v0, v1 := vy[yi], vy[yi+1]
			x0, y0 := builtinQuadLocal(w, h, u0, v0, env.Offset, scale)
			x1, y1 := builtinQuadLocal(w, h, u1, v1, env.Offset, scale)
			p := [4][2]float64{{x0, y0}, {x1, y0}, {x1, y1}, {x0, y1}}
			uv := [4][2]float64{
				{repeatCoord(u0, u0), repeatCoord(v0, v0)},
				{repeatCoord(u1, u0), repeatCoord(v0, v0)},
				{repeatCoord(u1, u0), repeatCoord(v1, v0)},
				{repeatCoord(u0, u0), repeatCoord(v1, v0)},
			}
			base := uint16(len(vs))
			for i := range p {
				x, y := m.Apply(p[i][0], p[i][1])
				vs = append(vs, ebiten.Vertex{
					DstX:   float32(x),
					DstY:   float32(y),
					SrcX:   float32(uv[i][0]*tw) + float32(b.Min.X),
					SrcY:   float32((1-uv[i][1])*th) + float32(b.Min.Y),
					ColorR: float32(tint[0]),
					ColorG: float32(tint[1]),
					ColorB: float32(tint[2]),
					ColorA: float32(tint[3]),
				})
			}
			is = append(is, base, base+1, base+2, base, base+2, base+3)
		}
	}
	dst.DrawTriangles(vs, is, tex, &ebiten.DrawTrianglesOptions{AntiAlias: true})
}

func repeatBreaks(a, b float64) []float64 {
	if a == b || math.IsNaN(a) || math.IsNaN(b) || math.IsInf(a, 0) || math.IsInf(b, 0) {
		return nil
	}
	if b < a {
		a, b = b, a
	}
	out := []float64{a}
	for x := math.Floor(a) + 1; x < b; x++ {
		out = append(out, x)
	}
	out = append(out, b)
	return out
}

func builtinQuadLocal(w, h, u, v float64, offset, scale [2]float64) (float64, float64) {
	xp := 0.0
	if scale[0] != 0 {
		xp = (u - offset[0]) / scale[0]
	}
	yp := 0.0
	if scale[1] != 0 {
		yp = (v - offset[1]) / scale[1]
	}
	return -w/2 + xp*w, -h/2 + yp*h
}

func repeatCoord(v, segmentStart float64) float64 {
	f := v - math.Floor(v)
	if math.Abs(f) < 1e-9 && v > segmentStart {
		return 1
	}
	return f
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
