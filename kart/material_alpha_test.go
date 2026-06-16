package kart

import (
	"math"
	"testing"

	"hsdemo/kmdata"
)

func TestMaterialAlphaCurvesAffectSceneAndTemplateTint(t *testing.T) {
	as := materialAlphaAssets()

	scene := NewScene(as)
	scene.Play("Root", "Fade", 0, 1)
	scene.Sample(0.5)
	if got, want := scene.state[0].matAlpha, 0.5; math.Abs(got-want) > 1e-9 {
		t.Fatalf("scene material alpha = %v, want %v", got, want)
	}

	tmpl := NewTemplate(as, "Root")
	inst := tmpl.NewInstance()
	inst.Play("", "Fade", 0, 1)
	inst.Queue(scene, 0.5, Identity(), 0)
	if len(scene.queued) != 1 {
		t.Fatalf("queued sprites = %d, want 1", len(scene.queued))
	}
	if got, want := scene.queued[0].Tint[3], 0.5; math.Abs(got-want) > 1e-9 {
		t.Fatalf("template material alpha tint = %v, want %v", got, want)
	}
}

func TestMaterialProgressCurvesAffectSceneAndTemplatePalette(t *testing.T) {
	as := materialAlphaAssets()

	scene := NewScene(as)
	scene.Play("Root", "Charge", 0, 1)
	scene.Sample(0.5)
	if !scene.state[0].hasMatProgress {
		t.Fatal("scene material progress curve was not sampled")
	}
	if got, want := scene.state[0].matProgress, 0.5; math.Abs(got-want) > 1e-9 {
		t.Fatalf("scene material progress = %v, want %v", got, want)
	}

	tmpl := NewTemplate(as, "Root")
	inst := tmpl.NewInstance()
	inst.Play("", "Charge", 0, 1)
	inst.Queue(scene, 0.5, Identity(), 0)
	if len(scene.queued) != 1 {
		t.Fatalf("queued sprites = %d, want 1", len(scene.queued))
	}
	if !scene.queued[0].HasProgress {
		t.Fatal("template material progress curve was not queued")
	}
	if got, want := scene.queued[0].Progress, 0.5; math.Abs(got-want) > 1e-9 {
		t.Fatalf("template material progress = %v, want %v", got, want)
	}
}

func materialAlphaAssets() *Assets {
	keys := []kmdata.Key{
		{T: 0, V: 0.25},
		{T: 1, V: 0.75},
	}
	return &Assets{
		Rig: kmdata.Rig{Nodes: []kmdata.Node{{
			Name:   "Root",
			Path:   "Root",
			Parent: -1,
			Scale:  [2]float64{1, 1},
			Sprite: "dummy",
			Color:  [4]float64{1, 1, 1, 1},
		}}},
		Anims: map[string]*kmdata.Anim{
			"Fade": {
				Duration: 1,
				Floats: map[string]map[string][]kmdata.Key{
					"": {"material._Alpha": keys},
				},
			},
			"Charge": {
				Duration: 1,
				Floats: map[string]map[string][]kmdata.Key{
					"": {"material._Progress": keys},
				},
			},
		},
		Sheet: kmdata.Sheet{PPU: 100, Sprites: map[string]kmdata.SpriteInfo{
			"dummy": {W: 1, H: 1, PPU: 100},
		}},
		Controllers: map[string]kmdata.Controller{},
		Animators:   kmdata.Animators{},
	}
}
