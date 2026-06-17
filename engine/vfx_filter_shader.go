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
	// imageSrcNAt 的入参使用 source0 的像素坐标系。LUT 条带坐标是 source-local
	// 偏移，必须加 source0 origin；不能加 imageSrc1/2Origin，否则会把 atlas
	// origin 应用两次。
	o := imageSrc0Origin()
	b := clamp(c.b, 0.0, 1.0) * 31.0
	bLo := floor(b)
	fr := b - bLo
	y := (1.0 - clamp(c.g, 0.0, 1.0))*31.0 + 0.5
	lutLo := vec2(bLo*32.0+clamp(c.r, 0.0, 1.0)*31.0+0.5, y)
	lutHi := vec2(min(bLo+1.0, 31.0)*32.0+clamp(c.r, 0.0, 1.0)*31.0+0.5, y)
	lo := imageSrc1At(o + lutLo).rgb
	hi := imageSrc1At(o + lutHi).rgb
	graded := mix(lo, hi, fr)
	defaultLo := imageSrc2At(o + lutLo).rgb
	defaultHi := imageSrc2At(o + lutHi).rgb
	defaultColor := mix(defaultLo, defaultHi, fr)
	return vec4(mix(defaultColor, graded, Blend), srcColor.a)
}
`
