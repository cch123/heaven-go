package engine

import (
	"image/color"
	"math"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"hsdemo/riq"
)

// TestKageCompile 验证三个后处理 shader 能通过 Kage 编译。
func TestKageCompile(t *testing.T) {
	for name, src := range map[string]string{
		"uber": uberKage, "bloomPre": bloomPreKage, "blur": blurKage,
		"vhsNoise": vhsNoiseGenKage, "vhsSmear": vhsSmearKage,
		"vhsDown": vhsDownsampleKage, "vhsUp": vhsUpsampleKage,
		"vhsComposite": vhsCompositeKage, "vhsGrain": vhsGrainKage,
	} {
		if _, err := ebiten.NewShader([]byte(src)); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestPostFXApplyPathRuns(t *testing.T) {
	var fx postFX
	if err := fx.ensure(); err != nil {
		t.Fatal(err)
	}
	fx.Target().Fill(colorRGBA(0x20, 0x40, 0x80, 0xff))
	dst := ebiten.NewImage(ScreenW, ScreenH)
	fx.Apply(dst, "../assets", 0, 1)
}

// TestWhiteBalanceNeutral 中性参数应得到 ≈1 的系数。
func TestWhiteBalanceNeutral(t *testing.T) {
	r, g, b := whiteBalance(0, 0)
	for _, v := range []float64{r, g, b} {
		if v < 0.95 || v > 1.05 {
			t.Errorf("中性白平衡系数偏离 1: %v %v %v", r, g, b)
		}
	}
}

func TestPostFXAcceptsXPostProcessingEventsSeenInCustomLevels(t *testing.T) {
	var fx postFX
	for _, dm := range []string{
		"ppe/colorReplace",
		"ppe/scanJitter",
		"ppe/screenJump",
		"ppe/retroTv",
		"ppe/edgeDetect",
		"ppe/sobelNeon",
		"ppe/gaussBlur",
		"ppe/grainBlur",
		"ppe/dirBlur",
		"ppe/analogNoise",
		"ppe/liquidScreen",
		"ppe/aurora",
	} {
		fx.add(&riq.Entity{Datamodel: dm, Data: map[string]any{"enable": true}})
	}
	for _, kind := range []string{
		"colorReplace", "scanJitter", "screenJump", "retroTv",
		"edgeDetect", "sobelNeon", "gaussBlur", "grainBlur", "dirBlur",
		"analogNoise", "liquidScreen", "aurora",
	} {
		if len(fx.evts[kind]) != 1 {
			t.Fatalf("%s event not registered: %#v", kind, fx.evts)
		}
	}
}

func TestEdgeDetectUsesUnityColorPairFields(t *testing.T) {
	list := []fxEvt{{
		beat:   0,
		length: 2,
		data: map[string]any{
			"enable":     true,
			"intenStart": 1.0,
			"intenEnd":   1.0,
			"color":      map[string]any{"r": 0.0, "g": 0.0, "b": 0.0, "a": 1.0},
			"color2":     map[string]any{"r": 1.0, "g": 0.5, "b": 0.0, "a": 1.0},
			"bgStart":    0.0,
			"bgEnd":      1.0,
			"bgColor":    map[string]any{"r": 1.0, "g": 1.0, "b": 1.0, "a": 1.0},
			"bgColor2":   map[string]any{"r": 0.0, "g": 0.0, "b": 1.0, "a": 1.0},
			"ease":       0.0,
		},
	}}
	var p fxParams
	evalEdgeDetectParams(&p, list, 1)
	if !p.edgeOn {
		t.Fatal("edgeDetect should be enabled")
	}
	if math.Abs(p.edgeColor[0]-0.5) > 1e-9 || math.Abs(p.edgeColor[1]-0.25) > 1e-9 {
		t.Fatalf("edge color did not ease color/color2: %#v", p.edgeColor)
	}
	if math.Abs(p.edgeBgColor[0]-0.5) > 1e-9 || math.Abs(p.edgeBgColor[2]-1.0) > 1e-9 {
		t.Fatalf("edge background color did not ease bgColor/bgColor2: %#v", p.edgeBgColor)
	}
}

func TestColorReplaceDisableHidesAllSlots(t *testing.T) {
	list := []fxEvt{
		{
			beat: 0,
			data: map[string]any{
				"enable":        true,
				"colorSlot":     1.0,
				"originalColor": map[string]any{"r": 1.0, "g": 0.0, "b": 0.0, "a": 1.0},
				"newColor":      map[string]any{"r": 0.0, "g": 1.0, "b": 0.0, "a": 1.0},
				"range":         0.2,
				"fuzziness":     0.5,
			},
		},
		{
			beat: 1,
			data: map[string]any{
				"enable":    false,
				"colorSlot": 2.0,
			},
		},
	}
	var p fxParams
	evalColorReplaceParams(&p, list, 2)
	if p.crRange != ([5]float64{}) || p.crFuzz != ([5]float64{}) || p.crFrom != ([5][4]float64{}) || p.crTo != ([5][4]float64{}) {
		t.Fatalf("disabled colorReplace should not output any active slot: %#v", p)
	}
}

func TestColorGradingTechnicolorParams(t *testing.T) {
	list := []fxEvt{{
		beat:   0,
		length: 2,
		data: map[string]any{
			"enable":        true,
			"technicolor":   true,
			"techStart":     0.2,
			"techEnd":       0.8,
			"exposureStart": 2.0,
			"exposureEnd":   4.0,
			"redStart":      0.2,
			"redEnd":        0.6,
			"greenStart":    0.4,
			"greenEnd":      0.8,
			"blueStart":     0.1,
			"blueEnd":       0.3,
			"ease":          0.0,
		},
	}}
	var p fxParams
	evalColorGradingParams(&p, list, 1)
	if !p.gradeOn {
		t.Fatal("colorGrading should be enabled")
	}
	if math.Abs(p.techInt-0.5) > 1e-9 {
		t.Fatalf("technicolor intensity did not ease: %v", p.techInt)
	}
	if math.Abs(p.techExposure-5.0) > 1e-9 {
		t.Fatalf("technicolor exposure should use XPost 8-exposure transform: %v", p.techExposure)
	}
	wantBal := [3]float64{0.6, 0.4, 0.8}
	for i := range wantBal {
		if math.Abs(p.techBalance[i]-wantBal[i]) > 1e-9 {
			t.Fatalf("technicolor balance[%d] = %v, want %v", i, p.techBalance[i], wantBal[i])
		}
	}
}

func TestBloomAnamorphicRatioParams(t *testing.T) {
	list := []fxEvt{{
		beat:   0,
		length: 2,
		data: map[string]any{
			"enable":     true,
			"intenStart": 1.0,
			"intenEnd":   1.0,
			"anaStart":   -1.0,
			"anaEnd":     1.0,
			"ease":       0.0,
		},
	}}
	var p fxParams
	evalBloomParams(&p, list, 1.5)
	if !p.bloomOn {
		t.Fatal("bloom should be enabled")
	}
	if math.Abs(p.bloomAna-0.5) > 1e-9 {
		t.Fatalf("anamorphic ratio did not ease: %v", p.bloomAna)
	}
	x, y := bloomAnamorphicBlurScale(p.bloomAna)
	if math.Abs(x-1.5) > 1e-9 || math.Abs(y-1.0) > 1e-9 {
		t.Fatalf("positive anamorphic ratio should bias horizontal blur: %v %v", x, y)
	}
	x, y = bloomAnamorphicBlurScale(-0.75)
	if math.Abs(x-1.0) > 1e-9 || math.Abs(y-1.75) > 1e-9 {
		t.Fatalf("negative anamorphic ratio should bias vertical blur: %v %v", x, y)
	}
}

func TestRetroTVVHSParams(t *testing.T) {
	list := []fxEvt{{
		beat:   0,
		length: 2,
		data: map[string]any{
			"enable":          true,
			"HSonVHS":         true,
			"bleedIntStart":   0.2,
			"bleedIntEnd":     0.6,
			"bleedIteration":  5.0,
			"vhsGrainStart":   0.1,
			"vhsGrainEnd":     0.3,
			"grainScaleStart": 0.4,
			"grainScaleEnd":   0.8,
			"stripeDenStart":  0.2,
			"stripeDenEnd":    0.4,
			"stripeOpacStart": 0.6,
			"stripeOpacEnd":   1.0,
			"edgeIntStart":    0.5,
			"edgeIntEnd":      1.5,
			"edgeDistStart":   0.001,
			"edgeDistEnd":     0.003,
			"ease":            0.0,
		},
	}}
	var p fxParams
	evalRetroTVParams(&p, list, 1)
	if !p.vhsOn {
		t.Fatal("HSonVHS should enable VHS even when CRT distortion intensity is zero")
	}
	if p.vhsIterations != 5 {
		t.Fatalf("VHS iterations = %d, want 5", p.vhsIterations)
	}
	checks := map[string][2]float64{
		"bleed":         {p.vhsBleed, 0.4},
		"grain":         {p.vhsGrain, 0.2},
		"grainScale":    {p.vhsGrainScale, 0.6},
		"stripeDensity": {p.vhsStripeDensity, 0.3},
		"stripeOpacity": {p.vhsStripeOpacity, 0.8},
		"edgeIntensity": {p.vhsEdgeIntensity, 1.0},
		"edgeDistance":  {p.vhsEdgeDistance, 0.002},
	}
	for name, vals := range checks {
		if math.Abs(vals[0]-vals[1]) > 1e-9 {
			t.Fatalf("%s = %v, want %v", name, vals[0], vals[1])
		}
	}
}

func TestVHSApplyPathRuns(t *testing.T) {
	src := ebiten.NewImage(ScreenW, ScreenH)
	src.Fill(colorRGBA(0x40, 0x80, 0xc0, 0xff))
	dst := ebiten.NewImage(ScreenW, ScreenH)
	var v vhsPostFX
	ok := v.apply(dst, src, "../assets", fxParams{
		vhsOn:            true,
		vhsBleed:         0.4,
		vhsIterations:    2,
		vhsGrain:         0.2,
		vhsGrainScale:    0.6,
		vhsStripeDensity: 0.3,
		vhsStripeOpacity: 0.8,
		vhsEdgeIntensity: 1.0,
		vhsEdgeDistance:  0.002,
	}, 1)
	if !ok {
		t.Fatal("VHS apply path should initialize and draw")
	}
}

func colorRGBA(r, g, b, a uint8) color.RGBA {
	return color.RGBA{R: r, G: g, B: b, A: a}
}

func TestAfterStackXPostProcessingParams(t *testing.T) {
	var p fxParams
	evalGrainyBlurParams(&p, []fxEvt{{
		beat:   0,
		length: 2,
		data: map[string]any{
			"enable":     true,
			"intenStart": 2.0,
			"intenEnd":   6.0,
			"ease":       0.0,
		},
	}}, 1)
	if math.Abs(p.grainBlur-4.0) > 1e-9 {
		t.Fatalf("grainBlur radius did not ease: %v", p.grainBlur)
	}

	p = fxParams{}
	evalAnalogNoiseParams(&p, []fxEvt{{
		beat:   0,
		length: 2,
		data: map[string]any{
			"enable":         true,
			"intenStart":     0.2,
			"intenEnd":       0.6,
			"fadingStart":    0.1,
			"fadingEnd":      0.5,
			"thresholdStart": 0.3,
			"thresholdEnd":   0.9,
			"ease":           0.0,
		},
	}}, 1)
	if math.Abs(p.analogSpeed-0.4) > 1e-9 || math.Abs(p.analogFade-0.3) > 1e-9 || math.Abs(p.analogThresh-0.6) > 1e-9 {
		t.Fatalf("analogNoise params did not ease: %#v", p)
	}

	p = fxParams{}
	evalLiquidScreenParams(&p, []fxEvt{{
		beat: 0,
		data: map[string]any{
			"enable":   true,
			"intenEnd": 3.0,
			"horizEnd": 4.0,
			"vertEnd":  5.0,
		},
	}}, 3)
	if p.liquidSpeed != 3 || p.liquidHoriz != 4 || p.liquidVert != 5 {
		t.Fatalf("liquidScreen should use end fields directly: %#v", p)
	}

	p = fxParams{}
	evalAuroraParams(&p, []fxEvt{{
		beat:   0,
		length: 2,
		data: map[string]any{
			"enable":      true,
			"intenStart":  0.2,
			"intenEnd":    0.6,
			"sizeStart":   0.1,
			"sizeEnd":     0.5,
			"smoothStart": 0.2,
			"smoothEnd":   0.8,
			"solidStart":  0.1,
			"solidEnd":    0.3,
			"redStart":    1.0,
			"redEnd":      0.6,
			"greenStart":  0.4,
			"greenEnd":    0.8,
			"blueStart":   0.2,
			"blueEnd":     0.4,
			"speed":       1.5,
			"ease":        0.0,
		},
	}}, 1)
	if !p.auroraOn || math.Abs(p.auroraFade-0.4) > 1e-9 || math.Abs(p.auroraArea-0.3) > 1e-9 {
		t.Fatalf("aurora primary params did not ease: %#v", p)
	}
	if math.Abs(p.auroraSmooth-p.auroraArea) > 1e-9 {
		t.Fatalf("aurora smoothness should mirror Unity's newSize assignment: %#v", p)
	}
	if math.Abs(p.auroraChange-2.0) > 1e-9 || math.Abs(p.auroraSpeed-1.5) > 1e-9 {
		t.Fatalf("aurora scalar params wrong: %#v", p)
	}
	wantColor := [3]float64{0.8, 0.6, 0.3}
	for i := range wantColor {
		if math.Abs(p.auroraColor[i]-wantColor[i]) > 1e-9 {
			t.Fatalf("aurora color[%d] = %v, want %v", i, p.auroraColor[i], wantColor[i])
		}
	}
}
