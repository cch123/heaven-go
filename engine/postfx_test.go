package engine

import (
	"math"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"hsdemo/riq"
)

// TestKageCompile 验证三个后处理 shader 能通过 Kage 编译。
func TestKageCompile(t *testing.T) {
	for name, src := range map[string]string{
		"uber": uberKage, "bloomPre": bloomPreKage, "blur": blurKage,
	} {
		if _, err := ebiten.NewShader([]byte(src)); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
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
		"ppe/dirBlur",
	} {
		fx.add(&riq.Entity{Datamodel: dm, Data: map[string]any{"enable": true}})
	}
	for _, kind := range []string{
		"colorReplace", "scanJitter", "screenJump", "retroTv",
		"edgeDetect", "sobelNeon", "gaussBlur", "dirBlur",
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
