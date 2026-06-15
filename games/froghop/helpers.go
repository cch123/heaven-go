package froghop

import (
	"image/color"
	"math"
	"strconv"
	"strings"

	"hsdemo/engine"
	"hsdemo/kart"
	"hsdemo/riq"
)

const (
	gameID    = "frogHop"
	actionAlt = 1
)

var (
	defaultBGTop    = [4]float64{0x5a / 255.0, 0x9c / 255.0, 0x28 / 255.0, 1}
	defaultBGBottom = [4]float64{0xd6 / 255.0, 0xee / 255.0, 0xa4 / 255.0, 1}
	white           = [4]float64{1, 1, 1, 1}
	dim             = [4]float64{0.5, 0.5, 0.5, 1}
)

type bopEvt struct {
	beat, length        float64
	blue, orange, green bool
}

type countEvt struct {
	beat                  float64
	start, leader, backup bool
}

type countForceEvt struct {
	beat           float64
	number         int
	leader, backup bool
}

type hopEvt struct {
	beat float64
	stop bool
}

type cueEvt struct {
	beat             float64
	kind             string
	spotlights, jazz bool
	enabled, hs      bool
}

type thankEvt struct {
	beat            float64
	pitched, manual bool
	pitch           float64
}

type mouthEvt struct {
	beat, length        float64
	state               string
	wink                bool
	blue, orange, green bool
}

type spotlightEvt struct {
	beat              float64
	front, back, dark bool
}

type bgEvt struct {
	beat, length float64
	fromTop      [4]float64
	toTop        [4]float64
	fromBottom   [4]float64
	toBottom     [4]float64
	ease         int
}

type stageEvt struct {
	beat                 float64
	top, rim, trim, base [4]float64
	mikeL, mikeR         bool
	front, back          [4]float64
}

type frogColorEvt struct {
	beat        float64
	group       string
	skin, tummy [4]float64
	pants, belt [4]float64
	sclera, lip [4]float64
	lipstick    bool
	hasBelt     bool
}

type forceEvt struct {
	beat, length float64
	front, back  bool
}

type pitchEvt struct {
	beat            float64
	enabled, manual bool
	pitch           float64
}

type disableEvt struct {
	beat    float64
	disable bool
}

func actionName(e *riq.Entity) string {
	if s, ok := strings.CutPrefix(e.Datamodel, gameID+"/"); ok {
		return s
	}
	return e.Datamodel
}

func boolParam(e *riq.Entity, key string) bool { return e.Float(key, 0) != 0 }

func boolDefault(e *riq.Entity, key string, def bool) bool {
	if _, ok := e.Data[key]; !ok {
		return def
	}
	return boolParam(e, key)
}

func intDefault(e *riq.Entity, key string, def int) int {
	if _, ok := e.Data[key]; !ok {
		return def
	}
	return int(e.Float(key, float64(def)))
}

func floatDefault(e *riq.Entity, key string, def float64) float64 {
	if _, ok := e.Data[key]; !ok {
		return def
	}
	return e.Float(key, def)
}

func colorParam(e *riq.Entity, key string, def [4]float64) [4]float64 {
	v, ok := e.Data[key]
	if !ok {
		return def
	}
	mm, ok := v.(map[string]any)
	if !ok {
		return def
	}
	get := func(k string, d float64) float64 {
		if n, ok := mm[k].(float64); ok {
			return n
		}
		return d
	}
	return [4]float64{get("r", def[0]), get("g", def[1]), get("b", def[2]), get("a", def[3])}
}

func rgba(c [4]float64) color.NRGBA {
	return color.NRGBA{
		R: byte(clamp01(c[0]) * 255),
		G: byte(clamp01(c[1]) * 255),
		B: byte(clamp01(c[2]) * 255),
		A: byte(clamp01(c[3]) * 255),
	}
}

func easeColor(ease int, a, b [4]float64, u float64) [4]float64 {
	return [4]float64{
		engine.Ease(ease, a[0], b[0], u),
		engine.Ease(ease, a[1], b[1], u),
		engine.Ease(ease, a[2], b[2], u),
		engine.Ease(ease, a[3], b[3], u),
	}
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func pitchAt(ctx *engine.Ctx, beat float64, pitches []pitchEvt) float64 {
	out := 1.0
	for _, p := range pitches {
		if p.beat > beat {
			break
		}
		if p.manual {
			out = p.pitch
		} else if p.enabled {
			out = ctx.BPMAt(beat) / 156
		} else {
			out = 1
		}
	}
	if out <= 0 || math.IsNaN(out) || math.IsInf(out, 0) {
		return 1
	}
	return out
}

func roleOr(ctx *engine.Ctx, key, fallback string) string {
	if p := ctx.Role(key); p != "" {
		return p
	}
	return fallback
}

func nodeScale(as *kart.Assets, path string) [2]float64 {
	for _, n := range as.Rig.Nodes {
		if n.Path == path {
			return n.Scale
		}
	}
	return [2]float64{1, 1}
}

func itoa(v int) string { return strconv.Itoa(v) }
