package engine

func (fx *postFX) eval(beat float64) fxParams {
	var p fxParams
	evalVignetteParams(&p, fx.evts["vignette"], beat)
	evalChromaticAberrationParams(&p, fx.evts["cabb"], beat)
	evalLensDistortionParams(&p, fx.evts["lensD"], beat)
	evalGrainParams(&p, fx.evts["grain"], beat)
	evalColorGradingParams(&p, fx.evts["colorGrading"], beat)
	evalPixelQuadParams(&p, fx.evts["pixelQuad"], beat)
	evalBloomParams(&p, fx.evts["bloom"], beat)
	evalRetroTVParams(&p, fx.evts["retroTv"], beat)
	evalScanJitterParams(&p, fx.evts["scanJitter"], beat)
	evalScreenJumpParams(&p, fx.evts["screenJump"], beat)
	evalGaussianBlurParams(&p, fx.evts["gaussBlur"], beat)
	evalGrainyBlurParams(&p, fx.evts["grainBlur"], beat)
	evalDirectionalBlurParams(&p, fx.evts["dirBlur"], beat)
	evalAnalogNoiseParams(&p, fx.evts["analogNoise"], beat)
	evalLiquidScreenParams(&p, fx.evts["liquidScreen"], beat)
	evalEdgeDetectParams(&p, fx.evts["edgeDetect"], beat)
	evalSobelNeonParams(&p, fx.evts["sobelNeon"], beat)
	evalColorReplaceParams(&p, fx.evts["colorReplace"], beat)
	evalAuroraParams(&p, fx.evts["aurora"], beat)
	return p
}
