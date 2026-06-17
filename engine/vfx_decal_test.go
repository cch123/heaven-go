package engine

import (
	"math"
	"testing"

	"hsdemo/kart"
	"hsdemo/riq"
)

func TestDecalEventParsesUnityFields(t *testing.T) {
	var fx decalFX
	fx.add(&riq.Entity{
		Datamodel: "vfx/display decal",
		Beat:      1,
		Length:    8,
		Data: map[string]any{
			"sprite":       "baguette",
			"ease":         0.0,
			"layer":        3.0,
			"displayLayer": 0.0,
			"sticky":       true,
			"filter":       0.0,
			"sX":           1.0,
			"sY":           2.0,
			"sZ":           3.0,
			"sWidth":       0.5,
			"sHeight":      0.75,
			"sRot":         10.0,
			"sColor":       map[string]any{"r": 1.0, "g": 0.5, "b": 0.25, "a": 0.75},
			"eX":           4.0,
			"eY":           5.0,
			"eZ":           6.0,
			"eWidth":       1.5,
			"eHeight":      1.75,
			"eRot":         90.0,
			"eColor":       map[string]any{"r": 0.0, "g": 0.25, "b": 0.5, "a": 0.0},
		},
	})

	if len(fx.evts) != 1 {
		t.Fatalf("events = %d, want 1", len(fx.evts))
	}
	e := fx.evts[0]
	if e.sprite != "baguette" || e.layer != 3 || e.displayLayer != decalLayerGame || !e.sticky || e.filter != 0 {
		t.Fatalf("event fields not parsed: %#v", e)
	}
	got, col := e.sample(5)
	if math.Abs(got.x-2.5) > 1e-9 || math.Abs(got.y-3.5) > 1e-9 || math.Abs(got.rot-50) > 1e-9 {
		t.Fatalf("sample transform = %#v", got)
	}
	if math.Abs(col[0]-0.5) > 1e-9 || math.Abs(col[3]-0.375) > 1e-9 {
		t.Fatalf("sample color = %#v", col)
	}
}

func TestGameLayerDecalProjectionUsesUnityPPUAndCamera(t *testing.T) {
	e := decalEvt{
		beat: 1, length: 8, ease: 0, displayLayer: decalLayerGame,
		start: decalTransform{x: 1.832, y: 1.2214, z: 0, w: 0.5, h: 0.5},
		end:   decalTransform{x: 0, y: 2.4428, z: 0, w: 0.5, h: 0.5},
		c0:    [4]float64{1, 1, 1, 1},
		c1:    [4]float64{1, 1, 1, 1},
	}
	cam := [3]float64{0, 2.0786, -2.887}
	x, y, scale, ok := e.project(1, cam)
	if !ok {
		t.Fatal("projection unexpectedly outside camera")
	}
	p := kart.CamDist / (0 - cam[2])
	wantX := float64(ScreenW)/2 + 1.832*p*54
	wantY := float64(ScreenH)/2 - (1.2214-cam[1])*p*54
	wantScale := p * 54 / customSpritePPU
	if math.Abs(x-wantX) > 1e-9 || math.Abs(y-wantY) > 1e-9 || math.Abs(scale-wantScale) > 1e-9 {
		t.Fatalf("projection = (%v,%v,%v), want (%v,%v,%v)", x, y, scale, wantX, wantY, wantScale)
	}
	if width := 1920 * 0.5 * scale; width < 1700 || width > 1900 {
		t.Fatalf("Fan Club Dance baguette width = %v px, expected camera-filling decal", width)
	}
}
