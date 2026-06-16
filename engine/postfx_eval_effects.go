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
	if evalFlag(list, beat, "technicolor", false) {
		p.techInt = evalNum(list, beat, "tech", 0)
		p.techExposure = 8 - evalNum(list, beat, "exposure", 1)
		p.techBalance = [3]float64{
			1 - evalNum(list, beat, "red", 0.5),
			1 - evalNum(list, beat, "green", 0.5),
			1 - evalNum(list, beat, "blue", 0.5),
		}
	}
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
	p.bloomAna = math.Max(-1, math.Min(1, evalNum(list, beat, "ana", 0)))
	p.bloomTint = evalColor(list, beat, "color", [4]float64{1, 1, 1, 1})
}

func evalRetroTVParams(p *fxParams, list []fxEvt, beat float64) {
	if len(list) == 0 {
		return
	}
	inten := evalNum(list, beat, "inten", 0)
	enabled := evalFlag(list, beat, "enable", true)
	if inten != 0 && enabled {
		p.retroDistort = inten
		p.retroRGB = evalNum(list, beat, "rgb", 1)
		p.retroBottom = evalNum(list, beat, "bottom", 0.02)
		p.retroNoise = evalNum(list, beat, "noise", 0.3)
	}
	if !enabled || !evalFlag(list, beat, "HSonVHS", false) {
		return
	}
	p.vhsBleed = evalNum(list, beat, "bleedInt", 0.5)
	p.vhsIterations = int(math.Max(2, math.Min(8, math.Round(evalPlainNum(list, beat, "bleedIteration", 2)))))
	p.vhsGrain = evalNum(list, beat, "vhsGrain", 0.1)
	p.vhsGrainScale = evalNum(list, beat, "grainScale", 0.1)
	p.vhsStripeDensity = evalNum(list, beat, "stripeDen", 0.1)
	p.vhsStripeOpacity = evalNum(list, beat, "stripeOpac", 1)
	p.vhsEdgeIntensity = evalNum(list, beat, "edgeInt", 0.5)
	p.vhsEdgeDistance = evalNum(list, beat, "edgeDist", 0.002)
	p.vhsOn = p.vhsBleed > 0 || p.vhsEdgeIntensity > 0 ||
		(p.vhsStripeDensity > 0 && p.vhsStripeOpacity > 0) || p.vhsGrain > 0
}

func evalScanJitterParams(p *fxParams, list []fxEvt, beat float64) {
	if len(list) == 0 {
		return
	}
	inten := evalNum(list, beat, "inten", 0)
	if inten != 0 && evalFlag(list, beat, "enable", true) {
		p.scanJitter = inten
	}
}

func evalScreenJumpParams(p *fxParams, list []fxEvt, beat float64) {
	if len(list) == 0 {
		return
	}
	inten := evalNum(list, beat, "inten", 0)
	if inten != 0 && evalFlag(list, beat, "enable", true) {
		p.screenJump = inten
	}
}

func evalGaussianBlurParams(p *fxParams, list []fxEvt, beat float64) {
	if len(list) == 0 {
		return
	}
	inten := evalNum(list, beat, "inten", 0)
	if inten != 0 && evalFlag(list, beat, "enable", true) {
		p.gaussBlur = inten
	}
}

func evalGrainyBlurParams(p *fxParams, list []fxEvt, beat float64) {
	if len(list) == 0 {
		return
	}
	inten := evalNum(list, beat, "inten", 0)
	if inten != 0 && evalFlag(list, beat, "enable", true) {
		p.grainBlur = inten
	}
}

func evalDirectionalBlurParams(p *fxParams, list []fxEvt, beat float64) {
	if len(list) == 0 {
		return
	}
	inten := evalNum(list, beat, "inten", 0)
	if inten == 0 || !evalFlag(list, beat, "enable", true) {
		return
	}
	p.dirBlur = inten
	p.dirAngle = evalNum(list, beat, "angle", 0) * math.Pi / 180
}

func evalAnalogNoiseParams(p *fxParams, list []fxEvt, beat float64) {
	if len(list) == 0 {
		return
	}
	speed := evalNum(list, beat, "inten", 0)
	if speed == 0 || !evalFlag(list, beat, "enable", true) {
		return
	}
	p.analogSpeed = speed
	p.analogFade = evalNum(list, beat, "fading", 0)
	p.analogThresh = evalNum(list, beat, "threshold", 0)
}

func evalLiquidScreenParams(p *fxParams, list []fxEvt, beat float64) {
	for _, e := range list {
		if beat < e.beat {
			break
		}
		speed := num(e.data, "intenEnd", 0)
		if speed == 0 || !flag(e.data, "enable", true) {
			p.liquidSpeed, p.liquidHoriz, p.liquidVert = 0, 0, 0
			continue
		}
		p.liquidSpeed = speed
		p.liquidHoriz = num(e.data, "horizEnd", 1)
		p.liquidVert = num(e.data, "vertEnd", 1)
	}
}

func evalEdgeDetectParams(p *fxParams, list []fxEvt, beat float64) {
	if len(list) == 0 {
		return
	}
	inten := evalNum(list, beat, "inten", 0.3)
	if inten == 0 || !evalFlag(list, beat, "enable", true) {
		return
	}
	p.edgeOn = true
	p.edgeWidth = inten
	p.edgeColor = evalColorPair(list, beat, "color", "color2", [4]float64{0, 0, 0, 1})
	p.edgeBgFade = evalNum(list, beat, "bg", 1)
	p.edgeBgColor = evalColorPair(list, beat, "bgColor", "bgColor2", [4]float64{1, 1, 1, 1})
}

func evalSobelNeonParams(p *fxParams, list []fxEvt, beat float64) {
	if len(list) == 0 || !evalFlag(list, beat, "enable", true) {
		return
	}
	p.neonFade = evalNum(list, beat, "inten", 0)
	p.neonWidth = evalNum(list, beat, "edgeWidth", 0)
	p.neonBg = evalNum(list, beat, "bgFade", 1)
	p.neonBright = evalNum(list, beat, "brightness", 1)
	p.neonOn = p.neonFade != 0 || p.neonWidth != 0 || p.neonBg != 1 || p.neonBright != 1
}

func evalAuroraParams(p *fxParams, list []fxEvt, beat float64) {
	if len(list) == 0 {
		return
	}
	inten := evalNum(list, beat, "inten", 0)
	if inten == 0 || !evalFlag(list, beat, "enable", true) {
		return
	}
	p.auroraOn = true
	p.auroraFade = inten
	p.auroraArea = evalNum(list, beat, "size", 0)
	// VFXManager.cs computes smoothness but writes vignetteSmothness from newSize.
	// Keep that typo-compatible behavior so editor-authored events match Unity.
	p.auroraSmooth = p.auroraArea
	p.auroraChange = evalNum(list, beat, "solid", 0.1) * 10
	p.auroraSpeed = evalPlainNum(list, beat, "speed", 1)
	p.auroraColor = [3]float64{
		evalNum(list, beat, "red", 1),
		evalNum(list, beat, "green", 1),
		evalNum(list, beat, "blue", 1),
	}
}

func evalColorReplaceParams(p *fxParams, list []fxEvt, beat float64) {
	enabled := true
	for _, e := range list {
		if beat < e.beat {
			break
		}
		enabled = flag(e.data, "enable", true)
		slot := int(num(e.data, "colorSlot", 1)) - 1
		if slot < 0 || slot >= len(p.crRange) {
			continue
		}
		if flag(e.data, "clear", false) {
			p.crFrom[slot], p.crTo[slot] = [4]float64{}, [4]float64{}
			p.crRange[slot], p.crFuzz[slot] = 0, 0
			continue
		}
		if !enabled {
			continue
		}
		prog := 1.0
		if e.length > 0 {
			prog = clamp01((beat - e.beat) / e.length)
		}
		ease := int(num(e.data, "ease", 0))
		from0 := colorOf(e.data, "originalColor", [4]float64{0, 0, 0, 1})
		from1 := colorOf(e.data, "originalColor2", [4]float64{0, 0, 0, 1})
		to0 := colorOf(e.data, "newColor", [4]float64{1, 1, 1, 1})
		to1 := colorOf(e.data, "newColor2", [4]float64{1, 1, 1, 1})
		for i := 0; i < 4; i++ {
			p.crFrom[slot][i] = Ease(ease, from0[i], from1[i], prog)
			p.crTo[slot][i] = Ease(ease, to0[i], to1[i], prog)
		}
		p.crRange[slot] = Ease(ease, num(e.data, "range", 0.2), num(e.data, "range2", 0.2), prog)
		p.crFuzz[slot] = Ease(ease, num(e.data, "fuzziness", 0.5), num(e.data, "fuzziness2", 0.5), prog)
	}
	if !enabled {
		p.crFrom, p.crTo = [5][4]float64{}, [5][4]float64{}
		p.crRange, p.crFuzz = [5]float64{}, [5]float64{}
	}
}
