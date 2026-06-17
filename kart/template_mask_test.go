package kart

import (
	"math"
	"testing"

	"hsdemo/kmdata"
)

func TestTemplateQueuePreservesTransparentSpriteMasks(t *testing.T) {
	as := &Assets{
		Rig: kmdata.Rig{Nodes: []kmdata.Node{
			{Name: "Bot", Path: "Bot", Parent: -1, Scale: [2]float64{1, 1}},
			{Name: "Mask", Path: "Bot/Mask", Parent: 0, Scale: [2]float64{1, 1}, Sprite: "mask", Color: [4]float64{1, 1, 1, 0}, Mask: true},
			{Name: "Fill", Path: "Bot/Fill", Parent: 0, Scale: [2]float64{1, 1}, Sprite: "fill", MaskIn: 1},
		}},
	}
	tmpl := NewTemplate(as, "Bot")
	if tmpl == nil {
		t.Fatal("missing template")
	}
	scene := NewScene(as)
	tmpl.NewInstance().Queue(scene, 0, Identity(), 0)

	var sawMask, sawMaskIn bool
	for _, q := range scene.queued {
		if q.Sprite == "mask" {
			sawMask = true
			if !q.Mask {
				t.Fatalf("mask queued without Mask flag: %#v", q)
			}
			if q.Tint[3] != 0 {
				t.Fatalf("transparent SpriteMask tint alpha = %v, want 0", q.Tint[3])
			}
		}
		if q.Sprite == "fill" {
			sawMaskIn = true
			if q.MaskIn != 1 {
				t.Fatalf("masked fill queued with MaskIn = %d, want 1", q.MaskIn)
			}
		}
	}
	if !sawMask || !sawMaskIn {
		t.Fatalf("queued mask=%v maskIn=%v, want both true; queue=%#v", sawMask, sawMaskIn, scene.queued)
	}
}

func TestTemplateNodeWorldHonorsRuntimeOverrides(t *testing.T) {
	as := &Assets{
		Rig: kmdata.Rig{Nodes: []kmdata.Node{
			{Name: "Root", Path: "Root", Parent: -1, Scale: [2]float64{1, 1}},
			{Name: "Pivot", Path: "Root/Pivot", Parent: 0, Pos: [2]float64{1, 0}, Scale: [2]float64{1, 1}},
			{Name: "Anchor", Path: "Root/Pivot/Anchor", Parent: 1, Pos: [2]float64{0, 2}, Scale: [2]float64{1, 1}},
		}},
	}
	in := NewTemplate(as, "Root").NewInstance()
	in.Offset = [2]float64{2, 3}
	in.SetRot("", math.Pi/2)
	in.SetScale("", 2, 2)
	in.SetPos("Pivot", 0.5, -0.5)
	in.SetRot("Pivot", -math.Pi/2)

	got, ok := in.NodeWorld("Pivot/Anchor", Identity())
	if !ok {
		t.Fatal("missing anchor")
	}
	want := TRS(2, 3, math.Pi/2, 2, 2).
		Mul(TRS(0.5, -0.5, -math.Pi/2, 1, 1)).
		Mul(TRS(0, 2, 0, 1, 1))
	if got != want {
		t.Fatalf("NodeWorld = %#v, want %#v", got, want)
	}
}

func TestTemplateResetSubtreeClearsRuntimeOverrides(t *testing.T) {
	as := &Assets{
		Rig: kmdata.Rig{Nodes: []kmdata.Node{
			{Name: "Root", Path: "Root", Parent: -1, Scale: [2]float64{1, 1}},
			{Name: "Actor", Path: "Root/Actor", Parent: 0, Scale: [2]float64{1, 1}},
			{Name: "Head", Path: "Root/Actor/Head", Parent: 1, Scale: [2]float64{1, 1}, Sprite: "head"},
			{Name: "Body", Path: "Root/Actor/Body", Parent: 1, Scale: [2]float64{1, 1}, Sprite: "body"},
			{Name: "Other", Path: "Root/Other", Parent: 0, Scale: [2]float64{1, 1}, Sprite: "other"},
		}},
	}
	inst := NewTemplate(as, "Root").NewInstance()
	inst.SetActive("Actor", true)
	inst.SetSprite("Actor/Head", "missing")
	inst.SetOrder("Actor/Head", 99)
	inst.SetPos("Actor/Body", 2, 3)
	inst.SetActive("Other", false)

	inst.ResetSubtree("Actor")

	if _, ok := inst.actives[1]; ok {
		t.Fatal("Actor active override should be cleared")
	}
	if _, ok := inst.sprites[2]; ok {
		t.Fatal("Actor/Head sprite override should be cleared")
	}
	if _, ok := inst.orders[2]; ok {
		t.Fatal("Actor/Head order override should be cleared")
	}
	if _, ok := inst.pos[3]; ok {
		t.Fatal("Actor/Body position override should be cleared")
	}
	if got, ok := inst.actives[4]; !ok || got {
		t.Fatalf("sibling active override changed: got value=%v ok=%v", got, ok)
	}
}

func TestTemplateResetSubtreeStopsStaleAnimationPlayers(t *testing.T) {
	as := &Assets{
		Rig: kmdata.Rig{Nodes: []kmdata.Node{
			{Name: "Root", Path: "Root", Parent: -1, Scale: [2]float64{1, 1}},
			{Name: "Actor", Path: "Root/Actor", Parent: 0, Scale: [2]float64{1, 1}},
			{Name: "Head", Path: "Root/Actor/Head", Parent: 1, Scale: [2]float64{1, 1}, Sprite: "head"},
			{Name: "Body", Path: "Root/Actor/Body", Parent: 1, Scale: [2]float64{1, 1}, Sprite: "body"},
			{Name: "Other", Path: "Root/Other", Parent: 0, Scale: [2]float64{1, 1}, Sprite: "other"},
		}},
		Sheet: kmdata.Sheet{PPU: 100, Sprites: map[string]kmdata.SpriteInfo{
			"head":  {W: 1, H: 1, PPU: 100},
			"body":  {W: 1, H: 1, PPU: 100},
			"other": {W: 1, H: 1, PPU: 100},
		}},
		Anims: map[string]*kmdata.Anim{
			"HideHead": {
				Duration: 1,
				Floats: map[string]map[string][]kmdata.Key{
					"Head": {"m_IsActive": {{T: 0, V: 0}}},
				},
			},
			"HideOther": {
				Duration: 1,
				Floats: map[string]map[string][]kmdata.Key{
					"Other": {"m_IsActive": {{T: 0, V: 0}}},
				},
			},
		},
	}
	inst := NewTemplate(as, "Root").NewInstance()
	inst.Play("Actor", "HideHead", 0, 1)
	inst.Play("Other", "HideOther", 0, 1)

	scene := NewScene(as)
	inst.Queue(scene, 0, Identity(), 0)
	if sawQueuedSprite(scene, "head") {
		t.Fatal("setup failed: HideHead should hide the actor head")
	}
	if sawQueuedSprite(scene, "other") {
		t.Fatal("setup failed: HideOther should hide the sibling")
	}

	inst.ResetSubtree("Actor")
	scene = NewScene(as)
	inst.Queue(scene, 0, Identity(), 0)
	if !sawQueuedSprite(scene, "head") {
		t.Fatal("actor head should be visible after ResetSubtree stops stale players")
	}
	if sawQueuedSprite(scene, "other") {
		t.Fatal("sibling animation player should remain active outside ResetSubtree")
	}
}

func sawQueuedSprite(scene *SceneInst, sprite string) bool {
	for _, q := range scene.queued {
		if q.Sprite == sprite {
			return true
		}
	}
	return false
}
