package engine

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
	// X-PostProcessing glitch/blur/edge/color effects.
	scanJitter, screenJump                  float64
	retroDistort, retroRGB, retroBottom     float64
	retroNoise                              float64
	gaussBlur, dirBlur, dirAngle            float64
	edgeOn                                  bool
	edgeWidth, edgeBgFade                   float64
	edgeColor, edgeBgColor                  [4]float64
	neonOn                                  bool
	neonFade, neonWidth, neonBg, neonBright float64
	crFrom, crTo                            [5][4]float64
	crRange, crFuzz                         [5]float64
}
