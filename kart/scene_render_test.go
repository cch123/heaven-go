package kart

import (
	"math"
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

func TestTemplateQueuesMeshRendererBinding(t *testing.T) {
	as := &Assets{
		Rig: kmdata.Rig{Nodes: []kmdata.Node{
			{Name: "Root", Path: "Root", Parent: -1, Pos: [2]float64{2, 3}, Scale: [2]float64{1, 1}},
			{Name: "Plane", Path: "Root/Plane", Parent: 0, Pos: [2]float64{4, 5}, Scale: [2]float64{1, 1}},
		}},
		Sheet: kmdata.Sheet{PPU: 100, Sprites: map[string]kmdata.SpriteInfo{}},
		Meshes: kmdata.MeshData{
			Bindings: []kmdata.MeshBinding{{
				Path:      "Root/Plane",
				Renderer:  "MeshRenderer",
				Mesh:      kmdata.AssetRef{GUID: "0", FileID: 10209},
				Materials: []kmdata.AssetRef{{GUID: "mat"}},
				Enabled:   true,
				Layer:     1,
				Order:     7,
			}},
			Materials: map[string]kmdata.Material{
				"mat": {Name: "PlaneMat", GUID: "mat", Colors: map[string][4]float64{"_Color": {0.25, 0.5, 0.75, 1}}},
			},
		},
		Anims:       map[string]*kmdata.Anim{},
		Controllers: map[string]kmdata.Controller{},
		Animators:   kmdata.Animators{},
	}
	scene := NewScene(as)
	inst := NewTemplate(as, "Root").NewInstance()
	inst.Queue(scene, 0, Identity(), 0)

	if len(scene.queuedMeshes) != 1 {
		t.Fatalf("queued meshes = %d, want 1", len(scene.queuedMeshes))
	}
	q := scene.queuedMeshes[0]
	if q.Binding != 0 || q.Layer != 1 || q.Order != 7 {
		t.Fatalf("queued mesh metadata = %#v", q)
	}
	if got, want := q.World.Tx, 6.0; math.Abs(got-want) > 1e-9 {
		t.Fatalf("queued mesh x = %v, want %v", got, want)
	}
	if got, want := q.World.Ty, 8.0; math.Abs(got-want) > 1e-9 {
		t.Fatalf("queued mesh y = %v, want %v", got, want)
	}
	if q.Tint != [4]float64{0.25, 0.5, 0.75, 1} {
		t.Fatalf("queued mesh tint = %#v", q.Tint)
	}
}

func TestTemplateMeshRendererMaterialAlphaCurvesAffectTint(t *testing.T) {
	as := &Assets{
		Rig: kmdata.Rig{Nodes: []kmdata.Node{
			{Name: "Root", Path: "Root", Parent: -1, Scale: [2]float64{1, 1}},
			{Name: "Plane", Path: "Root/Plane", Parent: 0, Scale: [2]float64{1, 1}},
		}},
		Sheet: kmdata.Sheet{PPU: 100, Sprites: map[string]kmdata.SpriteInfo{}},
		Meshes: kmdata.MeshData{
			Bindings: []kmdata.MeshBinding{{
				Path:      "Root/Plane",
				Renderer:  "MeshRenderer",
				Mesh:      kmdata.AssetRef{GUID: "0", FileID: 10209},
				Materials: []kmdata.AssetRef{{GUID: "mat"}},
				Enabled:   true,
			}},
			Materials: map[string]kmdata.Material{
				"mat": {Name: "PlaneMat", GUID: "mat", Colors: map[string][4]float64{"_Color": {1, 1, 1, 1}}},
			},
		},
		Anims: map[string]*kmdata.Anim{
			"Fade": {
				Duration: 1,
				Floats: map[string]map[string][]kmdata.Key{
					"Plane": {"material._Alpha": {{T: 0, V: 0.5}}},
				},
			},
		},
		Controllers: map[string]kmdata.Controller{},
		Animators:   kmdata.Animators{},
	}
	scene := NewScene(as)
	inst := NewTemplate(as, "Root").NewInstance()
	inst.Play("", "Fade", 0, 1)
	inst.Queue(scene, 0, Identity(), 0)

	if len(scene.queuedMeshes) != 1 {
		t.Fatalf("queued meshes = %d, want 1", len(scene.queuedMeshes))
	}
	if got, want := scene.queuedMeshes[0].Tint[3], 0.5; math.Abs(got-want) > 1e-9 {
		t.Fatalf("queued mesh alpha = %v, want %v", got, want)
	}
}
