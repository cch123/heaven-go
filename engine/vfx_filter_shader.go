package engine

const lutKage = `//kage:unit pixels
package main

var Blend float

func Fragment(dst vec4, src vec2, color vec4) vec4 {
	srcColor := imageSrc0At(src)
	c := srcColor.rgb
	// 32³ LUT 条带（1024×32）：x = b 切片*32 + r*31。
	// Unity Texture2D 的 v=0 在图像底部；Ebitengine 的 y=0 在图像顶部，
	// 因此 LUT 的绿色轴要反向，否则 default_lut 都会把黑色采成绿色。
	// DrawRectShader 在 pixel unit 下会把同尺寸 source 映射到各自的本地
	// 像素坐标；LUT 是完整的 1024×32 source，所以这里直接用条带坐标。
	// 不能加画面 source 的 origin，否则在 Ebitengine atlas 里会整屏采偏。
	b := clamp(c.b, 0.0, 1.0) * 31.0
	bLo := floor(b)
	fr := b - bLo
	y := (1.0 - clamp(c.g, 0.0, 1.0))*31.0 + 0.5
	lutLo := vec2(bLo*32.0+clamp(c.r, 0.0, 1.0)*31.0+0.5, y)
	lutHi := vec2(min(bLo+1.0, 31.0)*32.0+clamp(c.r, 0.0, 1.0)*31.0+0.5, y)
	lo := imageSrc1At(lutLo).rgb
	hi := imageSrc1At(lutHi).rgb
	graded := mix(lo, hi, fr)
	defaultLo := imageSrc2At(lutLo).rgb
	defaultHi := imageSrc2At(lutHi).rgb
	defaultColor := mix(defaultLo, defaultHi, fr)
	return vec4(mix(defaultColor, graded, Blend), srcColor.a)
}
`
