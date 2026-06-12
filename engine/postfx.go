// postfx.go：ppe/*（Post Processing Effects）的移植。
//
// HS 用 Unity PostProcessing v2（PPv2）+ X-PostProcessing 实现屏幕后处理，
// 事件（VFXManager.cs）按拍序折叠：prog>=0 即应用（钳 [0,1]），后续事件覆盖、
// 终值持久——与 vfx/move camera 相同的语义。
//
// 这里把游戏画面渲染到离屏帧，再经 Kage shader 链复刻 PPv2 公式：
//
//	PixelizeQuad（BeforeStack，UV 网格吸附）
//	→ Lens Distortion（UV 重映射）→ Chromatic Aberration（3 段光谱采样）
//	→ Bloom 叠加（阈值软膝 + 高斯模糊，简化为 1/4 分辨率两轮）
//	→ Vignette（classic 模式）→ Grain（hash 噪声近似胶片颗粒）
//	→ Color Grading LDR（LMS 白平衡/滤色/色相/饱和/亮度/LogC 对比度）
//
// 已知简化（详见 README）：bloom 用固定两轮 1/4 分辨率模糊近似 PPv2 的
// mip 金字塔；grain 用 hash 噪声近似烘焙噪声纹理；anamorphicRatio 未实现
//（全部关卡取 0）；technicolor 未实现（全部关卡未启用）。
package engine

import (
	"image/color"
	"log"
	"math"
	"sort"

	"github.com/hajimehoshi/ebiten/v2"

	"hsdemo/riq"
)

type fxEvt struct {
	beat, length float64
	data         map[string]any
}

// postFX 收集 ppe 事件并执行后处理链。
type postFX struct {
	evts map[string][]fxEvt // kind（vignette/cabb/...）→ 按拍排序

	uber      *ebiten.Shader
	preShader *ebiten.Shader // bloom 阈值预滤
	blur      *ebiten.Shader // 可分离高斯（方向作 uniform）

	frame     *ebiten.Image // 全分辨率游戏画面
	bloomFull *ebiten.Image // 全分辨率 bloom 结果（关闭时为黑）
	q1, q2    *ebiten.Image // 1/4 分辨率工作缓冲
}

func (fx *postFX) add(e *riq.Entity) {
	kind := e.Datamodel[len("ppe/"):]
	switch kind {
	case "vignette", "cabb", "bloom", "lensD", "grain", "colorGrading", "pixelQuad":
		if fx.evts == nil {
			fx.evts = map[string][]fxEvt{}
		}
		fx.evts[kind] = append(fx.evts[kind], fxEvt{e.Beat, e.Length, e.Data})
	default:
		log.Printf("engine: ppe/%s 未实现，跳过（出现时需补 postfx.go）", kind)
	}
}

func (fx *postFX) reset() { fx.evts = nil }

func (fx *postFX) active() bool { return len(fx.evts) > 0 }

func (fx *postFX) sortAll() {
	for _, list := range fx.evts {
		sort.Slice(list, func(i, j int) bool { return list[i].beat < list[j].beat })
	}
}

// evalNum 按 VFXManager 语义折叠一种效果的某个 start/end 参数对。
func evalNum(list []fxEvt, beat float64, key string, def float64) float64 {
	v := def
	for _, e := range list {
		if beat < e.beat {
			break
		}
		prog := 1.0
		if e.length > 0 {
			prog = clamp01((beat - e.beat) / e.length)
		}
		ease := int(num(e.data, "ease", 0))
		v = Ease(ease, num(e.data, key+"Start", def), num(e.data, key+"End", def), prog)
	}
	return v
}

// evalColor 折叠颜色参数对（分量缓动，VfxColorEase 同语义）。
func evalColor(list []fxEvt, beat float64, key string, def [4]float64) [4]float64 {
	v := def
	for _, e := range list {
		if beat < e.beat {
			break
		}
		prog := 1.0
		if e.length > 0 {
			prog = clamp01((beat - e.beat) / e.length)
		}
		ease := int(num(e.data, "ease", 0))
		c0 := colorOf(e.data, key+"Start", def)
		c1 := colorOf(e.data, key+"End", def)
		for i := 0; i < 4; i++ {
			v[i] = Ease(ease, c0[i], c1[i], prog)
		}
	}
	return v
}

// evalFlag 取"当前生效事件"的布尔参数（最后一个 beat<=now 的事件）。
func evalFlag(list []fxEvt, beat float64, key string, def bool) bool {
	v := def
	for _, e := range list {
		if beat < e.beat {
			break
		}
		v = flag(e.data, key, def)
	}
	return v
}

func num(m map[string]any, key string, def float64) float64 {
	if v, ok := m[key].(float64); ok {
		return v
	}
	return def
}

func flag(m map[string]any, key string, def bool) bool {
	if v, ok := m[key].(bool); ok {
		return v
	}
	return def
}

func colorOf(m map[string]any, key string, def [4]float64) [4]float64 {
	cm, ok := m[key].(map[string]any)
	if !ok {
		return def
	}
	get := func(k string, d float64) float64 {
		if f, ok := cm[k].(float64); ok {
			return f
		}
		return d
	}
	return [4]float64{get("r", def[0]), get("g", def[1]), get("b", def[2]), get("a", def[3])}
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// ---------- 每帧参数求值 ----------

type fxParams struct {
	// vignette
	vigOn                       bool
	vigInt, vigSmooth, vigRound float64
	vigRounded                  float64
	vigColor                    [4]float64
	vigX, vigY                  float64
	// cabb
	caAmt float64
	// lensD（预计算 PPv2 入参）
	lensTheta, lensSigma, lensIntensity, lensIX, lensIY float64
	// grain
	grainInt, grainSize, grainColored float64
	// colorGrading
	gradeOn                  bool
	balR, balG, balB         float64
	filter                   [4]float64
	hue, sat, bright, contra float64
	// pixelQuad
	pixSize, pixRatio, pixSX, pixSY float64
	// bloom
	bloomOn                         bool
	bloomInt, bloomThr, bloomKnee   float64
	bloomTint                       [4]float64
}

func (fx *postFX) eval(beat float64) fxParams {
	var p fxParams
	if l := fx.evts["vignette"]; len(l) > 0 {
		inten := evalNum(l, beat, "inten", 0)
		p.vigOn = inten != 0 && evalFlag(l, beat, "enable", true)
		if p.vigOn {
			p.vigInt = inten
			p.vigSmooth = evalNum(l, beat, "smooth", 0.2)
			p.vigRound = evalNum(l, beat, "round", 1)
			if evalFlag(l, beat, "rounded", false) {
				p.vigRounded = 1
			}
			p.vigColor = evalColor(l, beat, "color", [4]float64{0, 0, 0, 1})
			p.vigX = evalNum(l, beat, "xLoc", 0.5)
			p.vigY = evalNum(l, beat, "yLoc", 0.5)
		}
	}
	if l := fx.evts["cabb"]; len(l) > 0 {
		inten := evalNum(l, beat, "inten", 0)
		if inten != 0 && evalFlag(l, beat, "enable", true) {
			p.caAmt = inten * 0.05 // PPv2: _ChromaticAberration_Amount = intensity * 0.05
		}
	}
	if l := fx.evts["lensD"]; len(l) > 0 {
		inten := evalNum(l, beat, "inten", 0)
		if inten != 0 && evalFlag(l, beat, "enable", true) {
			// PPv2 LensDistortion 入参换算
			amount := 1.6 * math.Max(math.Abs(inten), 1)
			theta := math.Min(160, amount) * math.Pi / 180
			sigma := 2 * math.Tan(theta*0.5)
			p.lensTheta, p.lensSigma, p.lensIntensity = theta, sigma, inten
			p.lensIX = math.Max(evalNum(l, beat, "x", 1), 1e-4)
			p.lensIY = math.Max(evalNum(l, beat, "y", 1), 1e-4)
		}
	}
	if l := fx.evts["grain"]; len(l) > 0 {
		inten := evalNum(l, beat, "inten", 0)
		if inten != 0 && evalFlag(l, beat, "enable", true) {
			p.grainInt = inten
			p.grainSize = evalNum(l, beat, "size", 1)
			if evalFlag(l, beat, "colored", true) {
				p.grainColored = 1
			}
		}
	}
	if l := fx.evts["colorGrading"]; len(l) > 0 && evalFlag(l, beat, "enable", true) {
		p.gradeOn = true
		temp := evalNum(l, beat, "temp", 0)
		tint := evalNum(l, beat, "tint", 0)
		p.balR, p.balG, p.balB = whiteBalance(temp, tint)
		p.filter = evalColor(l, beat, "color", [4]float64{1, 1, 1, 1})
		p.hue = evalNum(l, beat, "hueShift", 0) / 360
		p.sat = evalNum(l, beat, "sat", 0)/100 + 1
		p.bright = evalNum(l, beat, "bright", 0)/100 + 1
		p.contra = evalNum(l, beat, "con", 0)/100 + 1
	}
	if l := fx.evts["pixelQuad"]; len(l) > 0 {
		sz := evalNum(l, beat, "pixelSize", 0)
		if sz != 0 && evalFlag(l, beat, "enable", true) {
			p.pixSize = (1.01 - sz) * 200 // X-PostProcessing: size = (1.01-pixelSize)*200
			p.pixRatio = evalNum(l, beat, "ratio", 1)
			p.pixSX = evalNum(l, beat, "xScale", 0.5625)
			p.pixSY = evalNum(l, beat, "yScale", 1)
		}
	}
	if l := fx.evts["bloom"]; len(l) > 0 {
		inten := evalNum(l, beat, "inten", 0)
		if inten != 0 && evalFlag(l, beat, "enable", true) {
			p.bloomOn = true
			p.bloomInt = math.Exp2(inten/10) - 1 // PPv2 intensity 响应曲线
			p.bloomThr = evalNum(l, beat, "threshold", 1)
			p.bloomKnee = evalNum(l, beat, "softKnee", 0.5)
			p.bloomTint = evalColor(l, beat, "color", [4]float64{1, 1, 1, 1})
		}
	}
	return p
}

// whiteBalance 计算 PPv2 的 LMS 白平衡系数（temperature/tint ∈ [-100,100]）。
func whiteBalance(temp, tint float64) (float64, float64, float64) {
	t1, t2 := temp/60, tint/60 // PPv2: range scaled /60
	x := 0.31271 - t1*b2f(t1 < 0, 0.1, 0.05)
	y := standardIlluminantY(x) + t2*0.05
	// CIExyToLMS
	Y := 1.0
	X := Y * x / y
	Z := Y * (1 - x - y) / y
	L := 0.7328*X + 0.4296*Y - 0.1624*Z
	M := -0.7036*X + 1.6975*Y + 0.0061*Z
	S := 0.0030*X + 0.0136*Y + 0.9834*Z
	// D65 白点的 LMS
	const w1L, w1M, w1S = 0.949237, 1.03542, 1.08728
	return w1L / L, w1M / M, w1S / S
}

func standardIlluminantY(x float64) float64 {
	return 2.87*x - 3*x*x - 0.27509507
}

func b2f(c bool, t, f float64) float64 {
	if c {
		return t
	}
	return f
}

// ---------- 渲染 ----------

func (fx *postFX) ensure() error {
	if fx.uber != nil {
		return nil
	}
	var err error
	if fx.uber, err = ebiten.NewShader([]byte(uberKage)); err != nil {
		return err
	}
	if fx.preShader, err = ebiten.NewShader([]byte(bloomPreKage)); err != nil {
		return err
	}
	if fx.blur, err = ebiten.NewShader([]byte(blurKage)); err != nil {
		return err
	}
	fx.frame = ebiten.NewImage(ScreenW, ScreenH)
	fx.bloomFull = ebiten.NewImage(ScreenW, ScreenH)
	fx.q1 = ebiten.NewImage(ScreenW/4, ScreenH/4)
	fx.q2 = ebiten.NewImage(ScreenW/4, ScreenH/4)
	return nil
}

// Target 返回游戏画面的渲染目标（需先 ensure）。
func (fx *postFX) Target() *ebiten.Image {
	fx.frame.Clear()
	return fx.frame
}

// Apply 把离屏帧经后处理链画到 dst。
func (fx *postFX) Apply(dst *ebiten.Image, beat, t float64) {
	p := fx.eval(beat)

	// bloom 前置链：阈值预滤 → 两轮可分离高斯（1/4 分辨率）→ 升采样
	fx.bloomFull.Fill(color.Black)
	if p.bloomOn {
		knee := p.bloomThr*p.bloomKnee + 1e-5
		fx.q1.Clear()
		op := &ebiten.DrawRectShaderOptions{}
		op.GeoM.Scale(0.25, 0.25)
		op.Images[0] = fx.frame
		op.Uniforms = map[string]any{
			"Threshold": float32(p.bloomThr),
			"Curve": []float32{float32(p.bloomThr - knee), float32(knee * 2),
				float32(0.25 / knee)},
		}
		fx.q1.DrawRectShader(ScreenW, ScreenH, fx.preShader, op)
		for i := 0; i < 2; i++ {
			fx.q2.Clear()
			bo := &ebiten.DrawRectShaderOptions{}
			bo.Images[0] = fx.q1
			bo.Uniforms = map[string]any{"Dir": []float32{1, 0}}
			fx.q2.DrawRectShader(ScreenW/4, ScreenH/4, fx.blur, bo)
			fx.q1.Clear()
			bo = &ebiten.DrawRectShaderOptions{}
			bo.Images[0] = fx.q2
			bo.Uniforms = map[string]any{"Dir": []float32{0, 1}}
			fx.q1.DrawRectShader(ScreenW/4, ScreenH/4, fx.blur, bo)
		}
		uo := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
		uo.GeoM.Scale(4, 4)
		fx.bloomFull.DrawImage(fx.q1, uo)
	}

	op := &ebiten.DrawRectShaderOptions{}
	op.Images[0] = fx.frame
	op.Images[1] = fx.bloomFull
	op.Uniforms = map[string]any{
		"Pixel":   []float32{float32(p.pixSize), float32(p.pixRatio), float32(p.pixSX), float32(p.pixSY)},
		"Lens":    []float32{float32(p.lensTheta), float32(p.lensSigma), float32(p.lensIntensity), float32(p.caAmt)},
		"LensXY":  []float32{float32(p.lensIX), float32(p.lensIY)},
		"Vig":     []float32{float32(p.vigInt * 3), float32(p.vigSmooth * 5), float32((1-p.vigRound)*6 + p.vigRound), float32(p.vigRounded)},
		"VigCol":  []float32{float32(p.vigColor[0]), float32(p.vigColor[1]), float32(p.vigColor[2])},
		"VigCtr":  []float32{float32(p.vigX), float32(p.vigY)},
		"VigOn":   b32(p.vigOn),
		"GradeOn": b32(p.gradeOn),
		"Balance": []float32{float32(p.balR), float32(p.balG), float32(p.balB)},
		"Filter":  []float32{float32(p.filter[0]), float32(p.filter[1]), float32(p.filter[2])},
		"HSB":     []float32{float32(p.hue), float32(p.sat), float32(p.bright), float32(p.contra)},
		"Grain":   []float32{float32(p.grainInt), float32(p.grainSize), float32(p.grainColored), float32(math.Mod(t, 64))},
		"BloomIT": []float32{float32(p.bloomInt * p.bloomTint[0]), float32(p.bloomInt * p.bloomTint[1]), float32(p.bloomInt * p.bloomTint[2])},
	}
	dst.DrawRectShader(ScreenW, ScreenH, fx.uber, op)
}

func b32(b bool) float32 {
	if b {
		return 1
	}
	return 0
}

// ---------- Kage shaders ----------

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
