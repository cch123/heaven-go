package engine

const lutKage = `//kage:unit pixels
package main

var Blend float

func sampleTargetLUT(o vec2, b float, r0 float, r1 float, g0 float, g1 float, rf float, gf float) vec3 {
	c00 := imageSrc1At(o + vec2(b*32.0+r0+0.5, g0+0.5)).rgb
	c10 := imageSrc1At(o + vec2(b*32.0+r1+0.5, g0+0.5)).rgb
	c01 := imageSrc1At(o + vec2(b*32.0+r0+0.5, g1+0.5)).rgb
	c11 := imageSrc1At(o + vec2(b*32.0+r1+0.5, g1+0.5)).rgb
	return mix(mix(c00, c10, rf), mix(c01, c11, rf), gf)
}

func sampleDefaultLUT(o vec2, b float, r0 float, r1 float, g0 float, g1 float, rf float, gf float) vec3 {
	c00 := imageSrc2At(o + vec2(b*32.0+r0+0.5, g0+0.5)).rgb
	c10 := imageSrc2At(o + vec2(b*32.0+r1+0.5, g0+0.5)).rgb
	c01 := imageSrc2At(o + vec2(b*32.0+r0+0.5, g1+0.5)).rgb
	c11 := imageSrc2At(o + vec2(b*32.0+r1+0.5, g1+0.5)).rgb
	return mix(mix(c00, c10, rf), mix(c01, c11, rf), gf)
}

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
	r := clamp(c.r, 0.0, 1.0) * 31.0
	g := (1.0 - clamp(c.g, 0.0, 1.0)) * 31.0
	b := clamp(c.b, 0.0, 1.0) * 31.0
	r0 := floor(r)
	r1 := min(r0+1.0, 31.0)
	rf := r - r0
	g0 := floor(g)
	g1 := min(g0+1.0, 31.0)
	gf := g - g0
	bLo := floor(b)
	bHi := min(bLo+1.0, 31.0)
	bf := b - bLo
	lo := sampleTargetLUT(o, bLo, r0, r1, g0, g1, rf, gf)
	hi := sampleTargetLUT(o, bHi, r0, r1, g0, g1, rf, gf)
	graded := mix(lo, hi, bf)
	defaultLo := sampleDefaultLUT(o, bLo, r0, r1, g0, g1, rf, gf)
	defaultHi := sampleDefaultLUT(o, bHi, r0, r1, g0, g1, rf, gf)
	defaultColor := mix(defaultLo, defaultHi, bf)
	return vec4(mix(defaultColor, graded, Blend), srcColor.a)
}
`
