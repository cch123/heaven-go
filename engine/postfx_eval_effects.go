package engine

import "math"

func evalVignetteParams(p *fxParams, list []fxEvt, beat float64) {
	if len(list) == 0 {
		return
	}
	inten := evalNum(list, beat, "inten", 0)
	p.vigOn = inten != 0 && evalFlag(list, beat, "enable", true)
	if !p.vigOn {
		return
	}
	p.vigInt = inten
	p.vigSmooth = evalNum(list, beat, "smooth", 0.2)
	p.vigRound = evalNum(list, beat, "round", 1)
	if evalFlag(list, beat, "rounded", false) {
		p.vigRounded = 1
	}
	p.vigColor = evalColor(list, beat, "color", [4]float64{0, 0, 0, 1})
	p.vigX = evalNum(list, beat, "xLoc", 0.5)
	p.vigY = evalNum(list, beat, "yLoc", 0.5)
}

func evalChromaticAberrationParams(p *fxParams, list []fxEvt, beat float64) {
	if len(list) == 0 {
		return
	}
	inten := evalNum(list, beat, "inten", 0)
	if inten != 0 && evalFlag(list, beat, "enable", true) {
		p.caAmt = inten * 0.05 // PPv2: _ChromaticAberration_Amount = intensity * 0.05
	}
}

func evalLensDistortionParams(p *fxParams, list []fxEvt, beat float64) {
	if len(list) == 0 {
		return
	}
	inten := evalNum(list, beat, "inten", 0)
	if inten == 0 || !evalFlag(list, beat, "enable", true) {
		return
	}
	// PPv2 LensDistortion 入参换算。
	amount := 1.6 * math.Max(math.Abs(inten), 1)
	theta := math.Min(160, amount) * math.Pi / 180
	sigma := 2 * math.Tan(theta*0.5)
	p.lensTheta, p.lensSigma, p.lensIntensity = theta, sigma, inten
	p.lensIX = math.Max(evalNum(list, beat, "x", 1), 1e-4)
	p.lensIY = math.Max(evalNum(list, beat, "y", 1), 1e-4)
}

func evalGrainParams(p *fxParams, list []fxEvt, beat float64) {
	if len(list) == 0 {
		return
	}
	inten := evalNum(list, beat, "inten", 0)
	if inten == 0 || !evalFlag(list, beat, "enable", true) {
		return
	}
	p.grainInt = inten
	p.grainSize = evalNum(list, beat, "size", 1)
	if evalFlag(list, beat, "colored", true) {
		p.grainColored = 1
	}
}

func evalColorGradingParams(p *fxParams, list []fxEvt, beat float64) {
	if len(list) == 0 || !evalFlag(list, beat, "enable", true) {
		return
	}
	p.gradeOn = true
	temp := evalNum(list, beat, "temp", 0)
	tint := evalNum(list, beat, "tint", 0)
	p.balR, p.balG, p.balB = whiteBalance(temp, tint)
	p.filter = evalColor(list, beat, "color", [4]float64{1, 1, 1, 1})
	p.hue = evalNum(list, beat, "hueShift", 0) / 360
	p.sat = evalNum(list, beat, "sat", 0)/100 + 1
	p.bright = evalNum(list, beat, "bright", 0)/100 + 1
	p.contra = evalNum(list, beat, "con", 0)/100 + 1
}

func evalPixelQuadParams(p *fxParams, list []fxEvt, beat float64) {
	if len(list) == 0 {
		return
	}
	sz := evalNum(list, beat, "pixelSize", 0)
	if sz == 0 || !evalFlag(list, beat, "enable", true) {
		return
	}
	p.pixSize = (1.01 - sz) * 200 // X-PostProcessing: size = (1.01-pixelSize)*200
	p.pixRatio = evalNum(list, beat, "ratio", 1)
	p.pixSX = evalNum(list, beat, "xScale", 0.5625)
	p.pixSY = evalNum(list, beat, "yScale", 1)
}

func evalBloomParams(p *fxParams, list []fxEvt, beat float64) {
	if len(list) == 0 {
		return
	}
	inten := evalNum(list, beat, "inten", 0)
	if inten == 0 || !evalFlag(list, beat, "enable", true) {
		return
	}
	p.bloomOn = true
	p.bloomInt = math.Exp2(inten/10) - 1 // PPv2 intensity 响应曲线。
	p.bloomThr = evalNum(list, beat, "threshold", 1)
	p.bloomKnee = evalNum(list, beat, "softKnee", 0.5)
	p.bloomTint = evalColor(list, beat, "color", [4]float64{1, 1, 1, 1})
}
