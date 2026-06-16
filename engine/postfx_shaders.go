package engine

// bloom 阈值预滤（PPv2 QuadraticThreshold；输入随 GeoM 缩到 1/4 分辨率）。
const bloomPreKage = `//kage:unit pixels
package main

var Threshold float
var Curve vec3

func Fragment(dst vec4, src vec2, color vec4) vec4 {
	c := imageSrc0At(src).rgb
	br := max(c.r, max(c.g, c.b))
	rq := clamp(br-Curve.x, 0, Curve.y)
	rq = Curve.z * rq * rq
	c *= max(rq, br-Threshold) / max(br, 1e-4)
	return vec4(c, 1)
}
`

// 可分离高斯模糊（9 tap）。
const blurKage = `//kage:unit pixels
package main

var Dir vec2

func Fragment(dst vec4, src vec2, color vec4) vec4 {
	w := [5]float{0.227027, 0.1945946, 0.1216216, 0.054054, 0.016216}
	c := imageSrc0At(src).rgb * w[0]
	for i := 1; i < 5; i++ {
		off := Dir * float(i) * 1.5
		c += imageSrc0At(src+off).rgb * w[i]
		c += imageSrc0At(src-off).rgb * w[i]
	}
	return vec4(c, 1)
}
`

// uber：PixelizeQuad → LensDistortion → ChromaticAberration → Bloom →
// Vignette → Grain → ColorGrading（LDR）。坐标用归一化 UV 计算、采样转像素。
const uberKage = `//kage:unit pixels
package main

var Pixel vec4   // (size, ratio, scaleX, scaleY)；size==0 关
var Lens vec4    // (theta, sigma, intensity, caAmount)
var LensXY vec2  // (intensityX, intensityY)
var Vig vec4     // (intensity*3, smoothness*5, roundness', rounded)
var VigCol vec3
var VigCtr vec2
var VigOn float
var GradeOn float
var Balance vec3
var Filter vec3
var HSB vec4     // (hueShift/360, sat, brightness, contrast)
var TechIE vec2  // (technicolor intensity, 8-exposure)
var TechBal vec3 // (1-redBalance, 1-greenBalance, 1-blueBalance)
var Grain vec4   // (intensity, size, colored, time)
var BloomIT vec3 // intensity * tint
var Glitch vec4  // (scanJitter, screenJump, retroDistort, time)
var Retro vec4   // (rgbBlend, bottomCollapse, noiseAmount, unused)
var Blur vec4    // (gaussRadius, dirRadius, dirAngle, unused)
var Edge vec4    // (edgeWidth, backgroundFade, enabled, unused)
var EdgeCol vec3
var EdgeBG vec3
var Neon vec4    // (edgeWidth, edgeFade, brightness, backgroundFade)
var CRFrom [5]vec4
var CRTo [5]vec4
var CRP [5]vec4

func distortUV(uv vec2) vec2 {
	if Lens.z == 0 {
		return uv
	}
	ruv := LensXY * (uv - 0.5)
	ru := length(ruv)
	if Lens.z > 0 {
		wu := ru * Lens.x
		ru2 := tan(wu) / (ru * Lens.y)
		return uv + ruv*(ru2-1)
	}
	ru2 := (1.0 / ru) * (1.0 / Lens.x) * atan(ru*Lens.y)
	return uv + ruv*(ru2-1)
}

func sampleUV(uv vec2) vec3 {
	o := imageSrc0Origin()
	s := imageSrc0Size()
	p := clamp(uv, vec2(0), vec2(1))*s + o
	return imageSrc0At(p).rgb
}

func sampleBloom(uv vec2) vec3 {
	o := imageSrc1Origin()
	s := imageSrc1Size()
	p := clamp(uv, vec2(0), vec2(1))*s + o
	return imageSrc1At(p).rgb
}

func wrap01(uv vec2) vec2 {
	return fract(fract(uv) + vec2(1.0))
}

func hash(p vec2) float {
	h := dot(p, vec2(127.1, 311.7))
	return fract(sin(h) * 43758.5453123)
}

func rgb2hsv(c vec3) vec3 {
	k := vec4(0.0, -1.0/3.0, 2.0/3.0, -1.0)
	p := mix(vec4(c.bg, k.wz), vec4(c.gb, k.xy), step(c.b, c.g))
	q := mix(vec4(p.xyw, c.r), vec4(c.r, p.yzx), step(p.x, c.r))
	d := q.x - min(q.w, q.y)
	e := 1e-10
	return vec3(abs(q.z+(q.w-q.y)/(6.0*d+e)), d/(q.x+e), q.x)
}

func hsv2rgb(c vec3) vec3 {
	k := vec4(1.0, 2.0/3.0, 1.0/3.0, 3.0)
	p := abs(fract(c.xxx+k.xyz)*6.0 - k.www)
	return c.z * mix(k.xxx, clamp(p-k.xxx, 0.0, 1.0), c.y)
}

func colorReplaceOne(c vec3, from vec3, to vec3, prm vec2) vec3 {
	if prm.x == 0 && prm.y == 0 {
		return c
	}
	d := distance(from, c)
	return mix(to, c, clamp((d-prm.x)/max(prm.y, 0.1), 0.0, 1.0))
}

func applyColorReplace(c vec3) vec3 {
	for i := 0; i < 5; i++ {
		c = colorReplaceOne(c, CRFrom[i].rgb, CRTo[i].rgb, CRP[i].xy)
	}
	return c
}

func applyTechnicolor(c vec3) vec3 {
	if TechIE.x <= 0 {
		return c
	}
	cyan := vec3(0.0, 1.30, 1.0)
	magenta := vec3(1.0, 0.0, 1.05)
	yellow := vec3(1.6, 1.6, 0.05)
	exposure := max(TechIE.y, 1e-4)
	balance := 1.0 / max(TechBal*exposure, vec3(1e-4))
	nr := dot(vec2(1.05, 0.620), c.rg*balance.rr)
	ng := dot(vec2(0.30, 1.0), c.rg*balance.gg)
	nb := dot(vec2(1.0, 1.05), c.rb*balance.bb)
	result := (vec3(nr) + cyan) * (vec3(ng) + magenta) * (vec3(nb) + yellow)
	return mix(c, result, clamp(TechIE.x, 0, 1))
}

func scanJitterUV(uv vec2) vec2 {
	if Glitch.x == 0 {
		return uv
	}
	displacement := 0.005 + pow(Glitch.x, 3.0)*0.1
	threshold := clamp(1.0-Glitch.x*1.2, 0.0, 1.0)
	strength := 0.5 + 0.5*cos(Glitch.w*18.0)
	j := hash(vec2(uv.y*2048.0, floor(Glitch.w*60.0)))*2.0 - 1.0
	j *= step(threshold, abs(j)) * displacement * strength
	return wrap01(uv + vec2(j, 0))
}

func screenJumpUV(uv vec2) vec2 {
	if Glitch.y == 0 {
		return uv
	}
	jumpTime := Glitch.w * Glitch.y * 9.8
	y := mix(uv.y, fract(uv.y+jumpTime), Glitch.y)
	return wrap01(vec2(uv.x, y))
}

func retroUV(uv vec2) vec2 {
	if Glitch.z == 0 {
		return uv
	}
	d := uv - 0.5
	r2 := dot(d, d)
	uv += d * r2 * Glitch.z * 0.18
	if Retro.y > 0 {
		collapse := smoothstep(1.0-Retro.y, 1.0, uv.y)
		uv.x = mix(uv.x, 0.5+(uv.x-0.5)*(1.0-collapse*Glitch.z), collapse)
	}
	return uv
}

func sampleGlitched(uv vec2) vec3 {
	uv = retroUV(screenJumpUV(scanJitterUV(uv)))
	c := sampleUV(uv)
	if Blur.x > 0 {
		r := Blur.x * 0.75
		c = c*0.40 +
			(sampleUV(uv+vec2(r, 0)) + sampleUV(uv-vec2(r, 0)))*0.15 +
			(sampleUV(uv+vec2(0, r)) + sampleUV(uv-vec2(0, r)))*0.10 +
			(sampleUV(uv+vec2(r*2, r*2)) + sampleUV(uv-vec2(r*2, r*2)))*0.05
	}
	if Blur.y > 0 {
		dir := vec2(cos(Blur.z), sin(Blur.z)) * Blur.y * 0.004
		sum := vec3(0)
		for k := -6; k <= 6; k++ {
			sum += sampleUV(uv + dir*float(k))
		}
		c = sum / 13.0
	}
	if Glitch.z != 0 && Retro.x > 0 {
		off := Glitch.z * Retro.x * 0.006
		c.r = sampleUV(uv+vec2(off, 0)).r
		c.b = sampleUV(uv-vec2(off, 0)).b
	}
	if Glitch.z != 0 && Retro.z > 0 {
		n := hash(uv*imageSrc0Size()+vec2(floor(Glitch.w*60.0)))
		c += (n-0.5) * Retro.z * Glitch.z * 0.25
	}
	return c
}

func intensity(c vec3) float {
	return sqrt(dot(c, c))
}

func sobelLum(uv vec2, width float) float {
	sz := imageSrc0Size()
	stepUV := vec2(width/sz.x, width/sz.y)
	tl := intensity(sampleGlitched(uv + vec2(-stepUV.x, stepUV.y)))
	ml := intensity(sampleGlitched(uv + vec2(-stepUV.x, 0)))
	bl := intensity(sampleGlitched(uv + vec2(-stepUV.x, -stepUV.y)))
	mt := intensity(sampleGlitched(uv + vec2(0, stepUV.y)))
	mb := intensity(sampleGlitched(uv + vec2(0, -stepUV.y)))
	tr := intensity(sampleGlitched(uv + vec2(stepUV.x, stepUV.y)))
	mr := intensity(sampleGlitched(uv + vec2(stepUV.x, 0)))
	br := intensity(sampleGlitched(uv + vec2(stepUV.x, -stepUV.y)))
	gx := tl + 2.0*ml + bl - tr - 2.0*mr - br
	gy := -tl - 2.0*mt - tr + bl + 2.0*mb + br
	return sqrt(gx*gx + gy*gy)
}

func sobelRGB(uv vec2, width float) vec3 {
	sz := imageSrc0Size()
	stepUV := vec2(width/sz.x, width/sz.y)
	tl := sampleGlitched(uv + vec2(-stepUV.x, stepUV.y))
	ml := sampleGlitched(uv + vec2(-stepUV.x, 0))
	bl := sampleGlitched(uv + vec2(-stepUV.x, -stepUV.y))
	mt := sampleGlitched(uv + vec2(0, stepUV.y))
	mb := sampleGlitched(uv + vec2(0, -stepUV.y))
	tr := sampleGlitched(uv + vec2(stepUV.x, stepUV.y))
	mr := sampleGlitched(uv + vec2(stepUV.x, 0))
	br := sampleGlitched(uv + vec2(stepUV.x, -stepUV.y))
	gx := tl + 2.0*ml + bl - tr - 2.0*mr - br
	gy := -tl - 2.0*mt - tr + bl + 2.0*mb + br
	return sqrt(gx*gx + gy*gy)
}

func linearToLogC(x vec3) vec3 {
	return 0.244161*log(5.555556*x+0.047996)/log(10.0) + 0.386036
}

func logCToLinear(x vec3) vec3 {
	return (pow(vec3(10.0), (x-0.386036)/0.244161) - 0.047996) / 5.555556
}

func Fragment(dst vec4, src vec2, color vec4) vec4 {
	o := imageSrc0Origin()
	s := imageSrc0Size()
	uv := (src - o) / s

	// PixelizeQuad（BeforeStack）
	if Pixel.x > 0 {
		cellX := Pixel.z / Pixel.x
		cellY := Pixel.y * Pixel.w / Pixel.x
		uv = vec2(cellX*floor(uv.x/cellX), cellY*floor(uv.y/cellY))
	}

	duv := distortUV(uv)

	// Chromatic Aberration（fast：3 段光谱 R/G/B）
	var c vec3
	if Lens.w != 0 {
		coords := 2.0*uv - 1.0
		end := uv - coords*dot(coords, coords)*Lens.w
		delta := (end - uv) / 3.0
		c.r = sampleGlitched(distortUV(uv)).r
		c.g = sampleGlitched(distortUV(uv + delta)).g
		c.b = sampleGlitched(distortUV(uv + delta*2.0)).b
	} else {
		c = sampleGlitched(duv)
	}

	// Bloom（已模糊的亮部 × intensity × tint）
	c += sampleBloom(duv) * BloomIT

	// Vignette（PPv2 classic）
	if VigOn > 0.5 {
		d := abs(duv-VigCtr) * Vig.x
		if Vig.w > 0.5 {
			d.x *= s.x / s.y
		}
		d = pow(clamp(d, vec2(0), vec2(1)), vec2(Vig.z))
		vf := pow(clamp(1.0-dot(d, d), 0, 1), Vig.y)
		c *= mix(VigCol, vec3(1.0), vf)
	}

	// Grain（hash 噪声近似 PPv2 胶片颗粒；亮度响应权重）
	if Grain.x > 0 {
		guv := floor(uv * s / max(Grain.y, 0.3))
		tseed := floor(Grain.w * 60.0)
		var n vec3
		if Grain.z > 0.5 {
			n = vec3(hash(guv+vec2(tseed, 0)), hash(guv+vec2(0, tseed)), hash(guv+vec2(tseed, tseed))) - 0.5
		} else {
			n = vec3(hash(guv + vec2(tseed, tseed*2)) - 0.5)
		}
		lum := 1.0 - sqrt(dot(clamp(c, vec3(0), vec3(1)), vec3(0.2126, 0.7152, 0.0722)))
		lum = mix(1.0, lum, 0.8)
		c += c * n * Grain.x * 2.0 * lum
	}

	// Color Grading（LDR，近似 PPv2 Lut2DBaker 顺序）
	if GradeOn > 0.5 {
		c = clamp(c, vec3(0), vec3(1))
		lin := pow(c, vec3(2.2))
		lin *= HSB.z // brightness
		// 白平衡（LMS）
		l := dot(lin, vec3(0.390405, 0.549941, 0.00892632)) * Balance.x
		m := dot(lin, vec3(0.0708416, 0.963172, 0.00135775)) * Balance.y
		sc := dot(lin, vec3(0.0231082, 0.128021, 0.936245)) * Balance.z
		lin.r = dot(vec3(l, m, sc), vec3(2.85847, -1.62879, -0.0248910))
		lin.g = dot(vec3(l, m, sc), vec3(-0.210182, 1.15820, 0.000324281))
		lin.b = dot(vec3(l, m, sc), vec3(-0.0418120, -0.118169, 1.06867))
		lin = max(lin, vec3(0))
		// 滤色
		lin *= Filter
		// 色相
		hsv := rgb2hsv(lin)
		hsv.x = fract(hsv.x + HSB.x)
		lin = hsv2rgb(hsv)
		// 饱和度
		lum2 := dot(lin, vec3(0.2126, 0.7152, 0.0722))
		lin = vec3(lum2) + (lin-vec3(lum2))*HSB.y
		// 对比度（LogC 空间绕 ACEScc 中灰）
		lc := linearToLogC(max(lin, vec3(0)))
		lc = (lc-0.4135884)*HSB.w + 0.4135884
		lin = max(logCToLinear(lc), vec3(0))
		c = pow(clamp(lin, vec3(0), vec3(1)), vec3(1.0/2.2))
	}

	c = applyTechnicolor(c)
	c = applyColorReplace(c)

	if Edge.z > 0.5 {
		g := sobelLum(duv, Edge.x)
		bg := mix(c, EdgeBG, Edge.y)
		c = mix(bg, EdgeCol, clamp(g, 0.0, 1.0))
	}

	if Neon.x != 0 || Neon.y != 0 || Neon.w != 0 {
		g := sobelRGB(duv, Neon.x)
		bg := mix(vec3(0), c, Neon.w)
		c = mix(bg, g, Neon.y) * Neon.z
	}

	return vec4(clamp(c, vec3(0), vec3(1)), 1)
}
`
