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
}
