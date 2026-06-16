package kart

import (
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
