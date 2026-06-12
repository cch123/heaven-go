// mapped.go：调色板映射精灵的绘制（CellAnime_MappedInvert shader 的移植）。
//
// HS 部分游戏（seeSaw 等）的角色贴图不是直接颜色，而是 RGB 掩码：
//
//	out = _ColorAlpha·r + _ColorBravo·g + _ColorDelta·b
//	out = screen(out, _AddColor)            （_AddColor 默认 0 → 无效果）
//	out *= 顶点色（SpriteRenderer.color）
//	out.rgb = abs(_Threshold - out.rgb)     （_Threshold 1 → 反色，0 → 原样）
//
// 运行时按 recolor 事件换 _ColorBravo（填充）与 _ColorDelta（描边）。
package kart

import (
	"github.com/hajimehoshi/ebiten/v2"
)

// Palette 是映射材质的运行时参数。
type Palette struct {
	Alpha     [4]float64 // _ColorAlpha（默认白，HS 运行时不改）
	Fill      [4]float64 // _ColorBravo
	Outline   [4]float64 // _ColorDelta
	Add       [4]float64 // _AddColor（screen 混合，默认 0）
	Threshold float64    // _Threshold（0 原样，1 反色）
}

// DefaultPalette 返回 shader 默认参数（seeSaw 的初始填充白/描边深蓝在模块里设）。
func DefaultPalette() Palette {
	return Palette{
		Alpha:   [4]float64{1, 1, 1, 1},
		Fill:    [4]float64{1, 1, 1, 1},
		Outline: [4]float64{1, 1, 1, 1},
	}
}

var mappedShader *ebiten.Shader

const mappedKage = `//kage:unit pixels
package main

var Alpha vec4
var Fill vec4
var Outline vec4
var Add vec4
var Threshold float
var Tint vec4

func Fragment(dst vec4, src vec2, color vec4) vec4 {
	c := imageSrc0At(src)
	mapped := Alpha*c.r + Fill*c.g + Outline*c.b
	a := c.a
	scr := 1.0 - (1.0-Add)*(1.0-mapped)
	out := scr * Tint
	out.a = a * Tint.a
	out.r = abs(Threshold - out.r)
	out.g = abs(Threshold - out.g)
	out.b = abs(Threshold - out.b)
	out.r *= out.a
	out.g *= out.a
	out.b *= out.a
	return out
}
`

func ensureMappedShader() *ebiten.Shader {
	if mappedShader == nil {
		s, err := ebiten.NewShader([]byte(mappedKage))
		if err != nil {
			panic("mapped shader: " + err.Error())
		}
		mappedShader = s
	}
	return mappedShader
}

// DrawSpriteMapped 以调色板映射 shader 绘制切片（world/proj 语义同 DrawSpriteOpts）。
func (a *Assets) DrawSpriteMapped(dst *ebiten.Image, name string, world, proj Aff, o SpriteOpts, pal Palette) {
	img := a.Sub(name)
	if img == nil {
		return
	}
	sp := a.Sheet.Sprites[name]
	ppu := a.ppuOf(sp)
	fx, fy := 1.0, 1.0
	if o.FlipX {
		fx = -1
	}
	if o.FlipY {
		fy = -1
	}
	tint := o.Tint
	if tint == [4]float64{} {
		tint = [4]float64{1, 1, 1, 1}
	}
	local := Scale(fx/ppu, -fy/ppu).
		Mul(Translate(-sp.PivotX*float64(sp.W), -(1-sp.PivotY)*float64(sp.H)))
	m := proj.Mul(world).Mul(local)

	sh := ensureMappedShader()
	op := &ebiten.DrawRectShaderOptions{}
	op.GeoM = m.GeoM()
	op.Images[0] = img
	v4 := func(c [4]float64) []float32 {
		return []float32{float32(c[0]), float32(c[1]), float32(c[2]), float32(c[3])}
	}
	op.Uniforms = map[string]any{
		"Alpha": v4(pal.Alpha), "Fill": v4(pal.Fill), "Outline": v4(pal.Outline),
		"Add": v4(pal.Add), "Threshold": float32(pal.Threshold),
		"Tint": v4(tint),
	}
	dst.DrawRectShader(sp.W, sp.H, sh, op)
}
