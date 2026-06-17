package engine

const lutKage = `//kage:unit pixels
package main

var Blend float

func Fragment(dst vec4, src vec2, color vec4) vec4 {
	c := imageSrc0At(src).rgb
	// 32³ LUT 条带（1024×32）：x = b 切片*32 + r*31。
	// Unity Texture2D 的 v=0 在图像底部；Ebitengine 的 y=0 在图像顶部，
	// 因此 LUT 的绿色轴要反向，否则 default_lut 都会把黑色采成绿色。
	// LUT 是第二张源图；Ebitengine 可能把不同源图放在不同 atlas 区域，
	// 因此 lookup 坐标必须使用 imageSrc1Origin()，不能沿用画面源的 origin。
	o := imageSrc1Origin()
	b := clamp(c.b, 0.0, 1.0) * 31.0
	bLo := floor(b)
	fr := b - bLo
	y := (1.0 - clamp(c.g, 0.0, 1.0))*31.0 + 0.5
	lo := imageSrc1At(o + vec2(bLo*32.0+clamp(c.r, 0.0, 1.0)*31.0+0.5, y)).rgb
	hi := imageSrc1At(o + vec2(min(bLo+1.0, 31.0)*32.0+clamp(c.r, 0.0, 1.0)*31.0+0.5, y)).rgb
	graded := mix(lo, hi, fr)
	return vec4(mix(c, graded, Blend), 1)
}
`
