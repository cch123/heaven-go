package engine

import "math"

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
	bloomOn                       bool
	bloomInt, bloomThr, bloomKnee float64
	bloomTint                     [4]float64
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
