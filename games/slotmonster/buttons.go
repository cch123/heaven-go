package slotmonster

import (
	"math"

	"hsdemo/kart"
)

const slotFlashFrames = 4

type slotButton struct {
	root       string
	srs        []string
	color      [4]float64
	flashColor [4]float64
	pressed    bool
	missed     bool
	flashing   bool
	flashT     float64
}

func (m *Module) loadButton(i int) slotButton {
	comp := m.ctx.Assets.Extra.Components["button"+intString(i)]
	root := comp.Path
	if root == "" && i < len(m.ctx.Assets.Extra.RefArrays["buttons"]) {
		root = m.ctx.Assets.Extra.RefArrays["buttons"][i]
	}
	srs := append([]string(nil), comp.RefArrays["srs"]...)
	if len(srs) == 0 && root != "" {
		srs = []string{root + "/ButtonBottom", root + "/Button"}
	}
	color := [4]float64{1, 1, 1, 1}
	if len(srs) > 0 {
		color = nodeColor(m.ctx.Assets, srs[0], color)
	}
	return slotButton{
		root: root, srs: srs, color: color,
		flashColor: [4]float64{1, 1, 0.68, 1}, pressed: true,
	}
}

func (b *slotButton) reset() {
	b.pressed = true
	b.missed = false
	b.flashing = false
}

func (b *slotButton) ready(sc *kart.SceneInst, beat float64) {
	if b.root != "" {
		sc.PlayState(b.root, "PopUp", beat, 0.5)
	}
	b.pressed = false
	b.flashing = false
	b.missed = false
}

func (b *slotButton) press(sc *kart.SceneInst, beat float64, isMiss bool) {
	if b.root != "" {
		sc.PlayState(b.root, "Press", beat, 0.5)
	}
	b.pressed = true
	b.flashing = false
	b.missed = isMiss
}

func (b *slotButton) tryFlash(sc *kart.SceneInst, beat, flashT float64) {
	if b.pressed {
		return
	}
	if b.root != "" {
		sc.PlayState(b.root, "Flash", beat, 0.5)
	}
	b.flashing = true
	b.flashT = flashT
}

func (b *slotButton) applyColor(sc *kart.SceneInst, t float64) {
	c := b.color
	switch {
	case b.pressed:
		c = lerpColor(b.color, [4]float64{0, 0, 0, 1}, 0.5)
	case b.flashing:
		// SlotButton.AnimateColor is driven by Flash.anim events with frames 0..4.
		// Replaying the same four 60 Hz frames keeps the event-only color fade
		// without adding generic AnimationEvent support to kart.
		frame := math.Floor((t - b.flashT) * 60)
		if frame >= slotFlashFrames {
			b.flashing = false
			c = b.color
		} else {
			u := frame / slotFlashFrames
			if u < 0 {
				u = 0
			}
			c = lerpColor(b.flashColor, b.color, u)
		}
	}
	for _, sr := range b.srs {
		sc.SetColorOver(sr, c)
	}
}

func nodeColor(as *kart.Assets, path string, def [4]float64) [4]float64 {
	for _, n := range as.Rig.Nodes {
		if n.Path == path && n.Color != [4]float64{} {
			return n.Color
		}
	}
	return def
}

func lerpColor(a, b [4]float64, t float64) [4]float64 {
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	return [4]float64{
		a[0] + (b[0]-a[0])*t,
		a[1] + (b[1]-a[1])*t,
		a[2] + (b[2]-a[2])*t,
		a[3] + (b[3]-a[3])*t,
	}
}
