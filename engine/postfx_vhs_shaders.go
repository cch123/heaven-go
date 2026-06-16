package engine

const vhsNoiseGenKage = `//kage:unit pixels
package main

var DstSize vec2
var HorizontalNoisePos float
var HorizontalNoisePower float
var SpeckScaleOffset vec4

func sample0Repeat(uv vec2) vec3 {
	o := imageSrc0Origin()
	s := imageSrc0Size()
	return imageSrc0At(o + fract(fract(uv)+vec2(1.0))*s).rgb
}

func sample1Repeat(uv vec2) vec3 {
	o := imageSrc1Origin()
	s := imageSrc1Size()
	return imageSrc1At(o + fract(fract(uv)+vec2(1.0))*s).rgb
}

func Fragment(dst vec4, src vec2, color vec4) vec4 {
	uv := src / DstSize
	horizontal := sample0Repeat(vec2(HorizontalNoisePos, uv.y)).r
	speck := sample1Repeat((uv-SpeckScaleOffset.zw)*SpeckScaleOffset.xy).r
	threshold := pow(clamp((1.0-horizontal)*(1.0-horizontal), 0.0, 1.0), HorizontalNoisePower)
	if speck > threshold {
		return vec4(1.0)
	}
	return vec4(0.0, 0.0, 0.0, 1.0)
}
`

const vhsSmearKage = `//kage:unit pixels
package main

var DstSize vec2
var Smear vec2 // (offsetScale, attenuation)

func sample0(uv vec2) float {
	o := imageSrc0Origin()
	s := imageSrc0Size()
	return imageSrc0At(o + clamp(uv, vec2(0.0), vec2(1.0))*s).r
}

func Fragment(dst vec4, src vec2, color vec4) vec4 {
	uv := src / DstSize
	texelX := 1.0 / imageSrc0Size().x
	c := sample0(uv)
	for i := 1; i <= 4; i++ {
		c += sample0(uv-vec2(texelX*Smear.x*float(i), 0.0)) * exp(-Smear.y*float(i))
	}
	return vec4(vec3(c), 1.0)
}
`

const vhsDownsampleKage = `//kage:unit pixels
package main

var SrcSize vec2
var NoiseOpacity float

func sample0(uv vec2) vec3 {
	o := imageSrc0Origin()
	s := imageSrc0Size()
	return imageSrc0At(o + clamp(uv, vec2(0.0), vec2(1.0))*s).rgb
}

func sample1(uv vec2) float {
	o := imageSrc1Origin()
	s := imageSrc1Size()
	return imageSrc1At(o + clamp(uv, vec2(0.0), vec2(1.0))*s).r
}

func Fragment(dst vec4, src vec2, color vec4) vec4 {
	uv := src / SrcSize
	offset := 1.0 / imageSrc0Size()
	offset.x *= 2.0
	c := sample0(uv+vec2(offset.x, offset.y)) +
		sample0(uv+vec2(-offset.x, offset.y)) +
		sample0(uv+vec2(offset.x, -offset.y)) +
		sample0(uv+vec2(-offset.x, -offset.y))
	c *= 0.25
	c += vec3(sample1(uv) * NoiseOpacity)
	return vec4(c, 1.0)
}
`

const vhsUpsampleKage = `//kage:unit pixels
package main

var DstSize vec2
var Blend float

func sample0(uv vec2) vec3 {
	o := imageSrc0Origin()
	s := imageSrc0Size()
	return imageSrc0At(o + clamp(uv, vec2(0.0), vec2(1.0))*s).rgb
}

func sample1(uv vec2) vec3 {
	o := imageSrc1Origin()
	s := imageSrc1Size()
	return imageSrc1At(o + clamp(uv, vec2(0.0), vec2(1.0))*s).rgb
}

func Fragment(dst vec4, src vec2, color vec4) vec4 {
	uv := src / DstSize
	return vec4(mix(sample0(uv), sample1(uv), Blend), 1.0)
}
`

const vhsCompositeKage = `//kage:unit pixels
package main

var DstSize vec2
var NoiseOpacity float
var ColorBleedIntensity float
var Edge vec2 // (intensity, distance)

func sample0(uv vec2) vec3 {
	o := imageSrc0Origin()
	s := imageSrc0Size()
	return imageSrc0At(o + clamp(uv, vec2(0.0), vec2(1.0))*s).rgb
}

func sample1(uv vec2) float {
	o := imageSrc1Origin()
	s := imageSrc1Size()
	return imageSrc1At(o + clamp(uv, vec2(0.0), vec2(1.0))*s).r
}

func sample2(uv vec2) vec3 {
	o := imageSrc2Origin()
	s := imageSrc2Size()
	return imageSrc2At(o + clamp(uv, vec2(0.0), vec2(1.0))*s).rgb
}

func sample3(uv vec2) vec3 {
	o := imageSrc3Origin()
	s := imageSrc3Size()
	return imageSrc3At(o + clamp(uv, vec2(0.0), vec2(1.0))*s).rgb
}

func rgbToYCbCr(rgb vec3) vec3 {
	return vec3(
		0.0625 + 0.257*rgb.r + 0.50412*rgb.g + 0.0979*rgb.b,
		0.5 - 0.14822*rgb.r - 0.290*rgb.g + 0.43921*rgb.b,
		0.5 + 0.43921*rgb.r - 0.3678*rgb.g - 0.07142*rgb.b,
	)
}

func yCbCrToRGB(ycbcr vec3) vec3 {
	ycbcr -= vec3(0.0625, 0.5, 0.5)
	return vec3(
		1.164*ycbcr.x + 1.596*ycbcr.z,
		1.164*ycbcr.x - 0.392*ycbcr.y - 0.813*ycbcr.z,
		1.164*ycbcr.x + 2.017*ycbcr.y,
	)
}

func Fragment(dst vec4, src vec2, color vec4) vec4 {
	uv := src / DstSize
	sharp := sample0(uv)
	sharp += vec3(sample1(uv) * NoiseOpacity)
	edges := sharp + vec3(0.5) - sample2(uv-vec2(Edge.y, 0.0))
	sharp += (edges - vec3(0.5)) * Edge.x
	ycc := rgbToYCbCr(sharp)
	blurred := rgbToYCbCr(sample3(uv)).yz
	ycc.yz = mix(ycc.yz, blurred, ColorBleedIntensity)
	return vec4(yCbCrToRGB(ycc), 1.0)
}
`

const vhsGrainKage = `//kage:unit pixels
package main

var DstSize vec2
var Grain vec4 // (intensity, scale, offsetX, offsetY)

func sample0(uv vec2) vec3 {
	o := imageSrc0Origin()
	s := imageSrc0Size()
	return imageSrc0At(o + clamp(uv, vec2(0.0), vec2(1.0))*s).rgb
}

func sampleGrain(uv vec2) vec3 {
	o := imageSrc1Origin()
	s := imageSrc1Size()
	return imageSrc1At(o + fract(fract(uv)+vec2(1.0))*s).rgb
}

func rgbToYCbCr(rgb vec3) vec3 {
	return vec3(
		0.0625 + 0.257*rgb.r + 0.50412*rgb.g + 0.0979*rgb.b,
		0.5 - 0.14822*rgb.r - 0.290*rgb.g + 0.43921*rgb.b,
		0.5 + 0.43921*rgb.r - 0.3678*rgb.g - 0.07142*rgb.b,
	)
}

func yCbCrToRGB(ycbcr vec3) vec3 {
	ycbcr -= vec3(0.0625, 0.5, 0.5)
	return vec3(
		1.164*ycbcr.x + 1.596*ycbcr.z,
		1.164*ycbcr.x - 0.392*ycbcr.y - 0.813*ycbcr.z,
		1.164*ycbcr.x + 2.017*ycbcr.y,
	)
}

func Fragment(dst vec4, src vec2, color vec4) vec4 {
	uv := src / DstSize
	ycc := rgbToYCbCr(sample0(uv))
	gUV := (uv-Grain.zw) * vec2(0.6*Grain.y, Grain.y)
	colorGrain := rgbToYCbCr(sampleGrain(gUV)).yz
	lumGrain := sampleGrain(gUV*4.0 - vec2(0.5)).g
	ycc.yz += (colorGrain - vec2(0.5)) * Grain.x * ycc.x
	ycc.x *= 1.0 + (lumGrain-0.5) * Grain.x * 0.5
	return vec4(yCbCrToRGB(ycc), 1.0)
}
`
