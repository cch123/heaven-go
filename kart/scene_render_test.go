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
	q := scene.queued[0]
	if !q.HasGroup || q.GroupOrder != 2 {
		t.Fatalf("queued group metadata = %#v, want group order 2", q)
	}
	if got, want := q.Order, 3; got != want {
		t.Fatalf("queued local order = %d, want %d", got, want)
	}
}

func TestControllerClipFallsBackFromImportedEmptyShell(t *testing.T) {
	as := &Assets{
		Rig: kmdata.Rig{Nodes: []kmdata.Node{
			{Name: "Root", Path: "Root", Parent: -1, Scale: [2]float64{1, 1}},
			{Name: "Head", Path: "Root/Head", Parent: 0, Scale: [2]float64{1, 1}, Sprite: "head_idle"},
		}},
		Sheet: kmdata.Sheet{PPU: 100, Sprites: map[string]kmdata.SpriteInfo{
			"head_idle": {W: 1, H: 1, PPU: 100},
			"head_anim": {W: 1, H: 1, PPU: 100},
		}},
		Anims: map[string]*kmdata.Anim{
			"Animations/Jump": {Duration: 1},
			"Jump": {
				Duration: 1,
				Pos: map[string]kmdata.XYCurve{
					"Head": {X: []kmdata.Key{{T: 0, V: 2}}},
				},
				Sprites: map[string][]kmdata.SwapKey{
					"Head": {{T: 0, Name: "head_anim"}},
				},
			},
		},
		Controllers: map[string]kmdata.Controller{
			"Ctrl": {Default: "Jump", States: map[string]kmdata.CtrlState{
				"Jump": {Clip: "Animations/Jump", Speed: 1},
			}},
		},
		Animators: kmdata.Animators{"Root": "Ctrl"},
	}

	scene := NewScene(as)
	scene.PlayState("Root", "Jump", 0, 1)
	scene.Sample(0)
	if got, want := scene.state[1].sprite, "head_anim"; got != want {
		t.Fatalf("scene resolved sprite = %q, want %q", got, want)
	}
	if got, want := scene.state[1].pos[0], 2.0; math.Abs(got-want) > 1e-9 {
		t.Fatalf("scene resolved x = %v, want %v", got, want)
	}

	scene = NewScene(as)
	inst := NewTemplate(as, "Root").NewInstance()
	inst.PlayState("", "Jump", 0, 1)
	inst.Queue(scene, 0, Identity(), 0)
	if len(scene.queued) != 1 {
		t.Fatalf("queued sprites = %d, want 1", len(scene.queued))
	}
	if got, want := scene.queued[0].Sprite, "head_anim"; got != want {
		t.Fatalf("template resolved sprite = %q, want %q", got, want)
	}
	if got, want := scene.queued[0].World.Tx, 2.0; math.Abs(got-want) > 1e-9 {
		t.Fatalf("template resolved x = %v, want %v", got, want)
	}
}

func TestSceneCameraFOVChangesPerspectiveScale(t *testing.T) {
	as := &Assets{
		Rig:         kmdata.Rig{Nodes: []kmdata.Node{{Name: "Root", Path: "Root", Parent: -1, Scale: [2]float64{1, 1}}}},
		Anims:       map[string]*kmdata.Anim{},
		Controllers: map[string]kmdata.Controller{},
		Animators:   kmdata.Animators{},
	}
	scene := NewScene(as)
	scene.SetCamera(0, 0, -CamDist)
	view, ok := scene.camView(0)
	if !ok {
		t.Fatal("default camera should see z=0")
	}
	if math.Abs(view.A-1) > 1e-9 || math.Abs(view.D-1) > 1e-9 {
		t.Fatalf("default camera scale = (%v,%v), want 1", view.A, view.D)
	}

	scene.SetCameraFOV(90)
	view, ok = scene.camView(0)
	if !ok {
		t.Fatal("90-degree camera should see z=0")
	}
	if got, want := view.A, 0.5; math.Abs(got-want) > 1e-9 {
		t.Fatalf("90-degree FOV scale = %v, want %v", got, want)
	}
	if got, want := CameraFocalDistance(90), 5.0; math.Abs(got-want) > 1e-9 {
		t.Fatalf("90-degree focal distance = %v, want %v", got, want)
	}

	scene.SetCameraFOV(0)
	view, _ = scene.camView(0)
	if math.Abs(view.A-1) > 1e-9 {
		t.Fatalf("reset FOV scale = %v, want default 1", view.A)
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
