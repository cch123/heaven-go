package kart

import (
	"testing"

	"hsdemo/kmdata"
)

func TestSetRenderOverDoesNotDeactivateChildren(t *testing.T) {
	as := &Assets{
		Rig: kmdata.Rig{Nodes: []kmdata.Node{
			{Name: "Head", Path: "Head", Parent: -1, Scale: [2]float64{1, 1}, Sprite: "head"},
			{Name: "FacePoser", Path: "Head/FacePoser", Parent: 0, Scale: [2]float64{1, 1}, Sprite: "face"},
		}},
		Sheet: kmdata.Sheet{PPU: 100, Sprites: map[string]kmdata.SpriteInfo{
			"head": {W: 1, H: 1, PPU: 100},
			"face": {W: 1, H: 1, PPU: 100},
		}},
		Anims:       map[string]*kmdata.Anim{},
		Controllers: map[string]kmdata.Controller{},
		Animators:   kmdata.Animators{},
	}
	scene := NewScene(as)

	// SpriteRenderer.enabled only hides the renderer on that GameObject. It must
	// not behave like GameObject.SetActive(false), which would also hide the
	// FacePoser child and recreate Fan Club's missing-head regression.
	scene.SetRenderOver("Head", false)
	scene.SetActive("Head/FacePoser", true)
	scene.Sample(0)

	if scene.state[0].renderOn {
		t.Fatal("parent renderer stayed enabled")
	}
	if !scene.actives[0] || !scene.actives[1] || !scene.state[1].renderOn {
		t.Fatalf("child renderer should remain visible: parentActive=%v childActive=%v childRender=%v", scene.actives[0], scene.actives[1], scene.state[1].renderOn)
	}
}

func TestTemplateGroupOrderSurvivesAnimatedSortingOrder(t *testing.T) {
	as := &Assets{
		Rig: kmdata.Rig{Nodes: []kmdata.Node{
			{Name: "Root", Path: "Root", Parent: -1, Scale: [2]float64{1, 1}},
			{Name: "Child", Path: "Root/Child", Parent: 0, Scale: [2]float64{1, 1}, Sprite: "child", Order: 1},
		}},
		Sheet: kmdata.Sheet{PPU: 100, Sprites: map[string]kmdata.SpriteInfo{
			"child": {W: 1, H: 1, PPU: 100},
		}},
		Anims: map[string]*kmdata.Anim{
			"Sort": {
				Duration: 1,
				Floats: map[string]map[string][]kmdata.Key{
					"Child": {"m_SortingOrder": {{T: 0, V: 3}}},
				},
			},
		},
		Controllers: map[string]kmdata.Controller{},
		Animators:   kmdata.Animators{},
	}
	scene := NewScene(as)
	inst := NewTemplate(as, "Root").NewInstance()
	inst.SetGroupOrder(2)
	inst.Play("", "Sort", 0, 1)
	inst.Queue(scene, 0, Identity(), 0)

	if len(scene.queued) != 1 {
		t.Fatalf("queued sprites = %d, want 1", len(scene.queued))
	}
	if got, want := scene.queued[0].Order, 203; got != want {
		t.Fatalf("queued order = %d, want %d", got, want)
	}
}
