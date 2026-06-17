// vfx_decal.go: vfx/display decal.
//
// Heaven Studio imports RIQ Resources/Sprites as Unity Sprites with a centered
// pivot and the default 100 pixels-per-unit. Decal events then animate the
// SpriteRenderer transform directly; keeping that PPU is what lets full-screen
// custom images land at the same size as the Unity chart.
package engine

import (
	"math"
	"sort"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"hsdemo/kart"
	"hsdemo/riq"
)

const customSpritePPU = 100.0

const (
	decalLayerBackground = -1
	decalLayerGame       = 0
	decalLayerDecal      = 1
	decalLayerOverlay    = 2
)

type decalEvt struct {
	beat, length float64
	sprite       string
	ease         int
	layer        int
	displayLayer int
	sticky       bool
	filter       int
	start, end   decalTransform
	c0, c1       [4]float64
}

type decalTransform struct {
	x, y, z float64
	w, h    float64
	rot     float64
}

type decalFX struct {
	evts []decalEvt
}

func (d *decalFX) add(e *riq.Entity) {
	d.evts = append(d.evts, decalEvt{
		beat: e.Beat, length: e.Length,
		sprite:       e.Str("sprite", ""),
		ease:         int(e.Float("ease", 1)),
		layer:        int(e.Float("layer", 0)),
		displayLayer: int(e.Float("displayLayer", decalLayerDecal)),
		sticky:       boolParam(e, "sticky"),
		filter:       int(e.Float("filter", 1)),
		start: decalTransform{
			x: e.Float("sX", 0), y: e.Float("sY", 0), z: e.Float("sZ", 0),
			w: e.Float("sWidth", 1), h: e.Float("sHeight", 1), rot: e.Float("sRot", 0),
		},
		end: decalTransform{
			x: e.Float("eX", 0), y: e.Float("eY", 0), z: e.Float("eZ", 0),
			w: e.Float("eWidth", 1), h: e.Float("eHeight", 1), rot: e.Float("eRot", 0),
		},
		c0: colorParamDefault(e, "sColor", [4]float64{1, 1, 1, 1}),
		c1: colorParamDefault(e, "eColor", [4]float64{1, 1, 1, 1}),
	})
}

func (d *decalFX) reset() { d.evts = nil }

func colorParamDefault(e *riq.Entity, key string, def [4]float64) [4]float64 {
	if _, ok := e.Data[key]; !ok {
		return def
	}
	return colorParam(e, key)
}

func (d *decalFX) DrawGame(dst *ebiten.Image, sprites map[string]*ebiten.Image, beat float64, cam [3]float64) {
	d.drawLayer(dst, sprites, beat, cam, func(e decalEvt) bool {
		return e.displayLayer == decalLayerGame
	})
}

func (d *decalFX) DrawBackground(dst *ebiten.Image, sprites map[string]*ebiten.Image, beat float64, cam [3]float64) {
	d.drawLayer(dst, sprites, beat, cam, func(e decalEvt) bool {
		return e.displayLayer == decalLayerBackground
	})
}

func (d *decalFX) DrawOverlay(dst *ebiten.Image, sprites map[string]*ebiten.Image, beat float64, cam [3]float64) {
	d.drawLayer(dst, sprites, beat, cam, func(e decalEvt) bool {
		return e.displayLayer == decalLayerDecal || e.displayLayer == decalLayerOverlay
	})
}

func (d *decalFX) drawLayer(dst *ebiten.Image, sprites map[string]*ebiten.Image, beat float64, cam [3]float64, keep func(decalEvt) bool) {
	if len(d.evts) == 0 || len(sprites) == 0 {
		return
	}
	active := d.active(beat, keep)
	for _, e := range active {
		img := lookupDecalSprite(sprites, e.sprite)
		if img == nil {
			continue
		}
		drawDecalImage(dst, img, e, beat, cam)
	}
}

func (d *decalFX) active(beat float64, keep func(decalEvt) bool) []decalEvt {
	out := make([]decalEvt, 0)
	for _, e := range d.evts {
		if !keep(e) || e.length <= 0 || beat < e.beat || beat >= e.beat+e.length {
			continue
		}
		out = append(out, e)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].displayLayer != out[j].displayLayer {
			return out[i].displayLayer < out[j].displayLayer
		}
		if out[i].layer != out[j].layer {
			return out[i].layer < out[j].layer
		}
		return out[i].beat < out[j].beat
	})
	return out
}

func lookupDecalSprite(sprites map[string]*ebiten.Image, name string) *ebiten.Image {
	if img, ok := sprites[name]; ok {
		return img
	}
	return sprites[strings.ToLower(name)]
}

func drawDecalImage(dst, img *ebiten.Image, e decalEvt, beat float64, cam [3]float64) {
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	if w == 0 || h == 0 {
		return
	}
	x, y, scale, ok := e.project(beat, cam)
	if !ok {
		return
	}
	t, c := e.sample(beat)

	op := &ebiten.DrawImageOptions{}
	if e.filter != 0 {
		op.Filter = ebiten.FilterLinear
	}
	op.GeoM.Translate(-float64(w)/2, -float64(h)/2)
	op.GeoM.Scale(t.w*scale, t.h*scale)
	if t.rot != 0 {
		op.GeoM.Rotate(t.rot * math.Pi / 180)
	}
	op.GeoM.Translate(x, y)
	op.ColorScale.Scale(float32(c[0]), float32(c[1]), float32(c[2]), float32(c[3]))
	dst.DrawImage(img, op)
}

func (e decalEvt) sample(beat float64) (decalTransform, [4]float64) {
	u := clamp01((beat - e.beat) / e.length)
	t := decalTransform{
		x:   Ease(e.ease, e.start.x, e.end.x, u),
		y:   Ease(e.ease, e.start.y, e.end.y, u),
		z:   Ease(e.ease, e.start.z, e.end.z, u),
		w:   Ease(e.ease, e.start.w, e.end.w, u),
		h:   Ease(e.ease, e.start.h, e.end.h, u),
		rot: Ease(e.ease, e.start.rot, e.end.rot, u),
	}
	c := [4]float64{}
	for i := range c {
		c[i] = Ease(e.ease, e.c0[i], e.c1[i], u)
	}
	return t, c
}

func (e decalEvt) project(beat float64, cam [3]float64) (float64, float64, float64, bool) {
	t, _ := e.sample(beat)
	scale := 54.0 / customSpritePPU
	if e.displayLayer == decalLayerGame && !e.sticky {
		d := t.z - cam[2]
		if d <= 0 {
			return 0, 0, 0, false
		}
		p := kart.CamDist / d
		return float64(ScreenW)/2 + (t.x-cam[0])*p*54,
			float64(ScreenH)/2 - (t.y-cam[1])*p*54,
			p * scale, true
	}
	return float64(ScreenW)/2 + t.x*54,
		float64(ScreenH)/2 - t.y*54,
		scale, true
}
