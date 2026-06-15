package engine

const lutKage = `//kage:unit pixels
package main

var Blend float

func Fragment(dst vec4, src vec2, color vec4) vec4 {
	c := imageSrc0At(src).rgb
	// 32³ LUT 条带（1024×32）：x = b 切片*32 + r*31，y = g*31。
	// 像素模式下各 imageSrcNAt 共用 src0 的坐标系（同尺寸源按 src0
	// origin 对齐），LUT 局部坐标须加 imageSrc0Origin()。
	o := imageSrc0Origin()
	b := clamp(c.b, 0.0, 1.0) * 31.0
	bLo := floor(b)
	fr := b - bLo
	lo := imageSrc1At(o + vec2(bLo*32.0+clamp(c.r, 0.0, 1.0)*31.0+0.5, clamp(c.g, 0.0, 1.0)*31.0+0.5)).rgb
	hi := imageSrc1At(o + vec2(min(bLo+1.0, 31.0)*32.0+clamp(c.r, 0.0, 1.0)*31.0+0.5, clamp(c.g, 0.0, 1.0)*31.0+0.5)).rgb
	graded := mix(lo, hi, fr)
	return vec4(mix(c, graded, Blend), 1)
}
`
