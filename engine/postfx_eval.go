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
	return p
}
