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
var Grain vec4   // (intensity, size, colored, time)
var BloomIT vec3 // intensity * tint

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
		c.r = sampleUV(distortUV(uv)).r
		c.g = sampleUV(distortUV(uv + delta)).g
		c.b = sampleUV(distortUV(uv + delta*2.0)).b
	} else {
		c = sampleUV(duv)
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

	return vec4(clamp(c, vec3(0), vec3(1)), 1)
}
`
