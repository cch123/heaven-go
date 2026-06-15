package rapmen

import (
	"image/color"
	"math"
	"strconv"
	"strings"

	"hsdemo/engine"
	"hsdemo/riq"
)

const gameID = "rapMen"

var (
	defaultA = [4]float64{0x47 / 255.0, 0xa1 / 255.0, 0xdf / 255.0, 1}
	defaultB = [4]float64{0x76 / 255.0, 0xce / 255.0, 0xfd / 255.0, 1}
	defaultC = [4]float64{0xd3 / 255.0, 0xf4 / 255.0, 0xfe / 255.0, 1}
	defaultD = [4]float64{1, 1, 1, 1}
)

type bopEvt struct {
	beat, length        float64
	red, yellow         bool
	redAuto, yellowAuto bool
}

type rapEvt struct {
	beat              float64
	cue               string
	gender            int
	voice, womanVoice int
	caption           int
	text              string
	mute              bool
}

type banterEvt struct {
	beat     float64
	gender   int
	voice    int
	playAnim bool
}

type textEvt struct {
	beat, length float64
	text         string
	color        int
}

type bgEvt struct {
	beat, length float64
	typ, ease    int
	a0, a1       [4]float64
	b0, b1       [4]float64
	c0, c1       [4]float64
	d0, d1       [4]float64
}

type toggleEvt struct {
	beat        float64
	red, yellow int
}

type soundCue struct {
	clip   string
	beat   float64
	vol    float64
	offset float64
	pan    float64
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

func stringDefault(e *riq.Entity, key, def string) string {
	if v, ok := e.Data[key].(string); ok {
		return v
	}
	return def
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

func roleOr(ctx *engine.Ctx, key, fallback string) string {
	if p := ctx.Role(key); p != "" {
		return p
	}
	return fallback
}

func rgba(c [4]float64) color.NRGBA {
	return color.NRGBA{R: byte(clamp01(c[0]) * 255), G: byte(clamp01(c[1]) * 255), B: byte(clamp01(c[2]) * 255), A: byte(clamp01(c[3]) * 255)}
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

func colorAt(ease int, a, b [4]float64, u float64) [4]float64 {
	return [4]float64{
		engine.Ease(ease, a[0], b[0], u),
		engine.Ease(ease, a[1], b[1], u),
		engine.Ease(ease, a[2], b[2], u),
		engine.Ease(ease, a[3], b[3], u),
	}
}

func norm(beat, start, length float64) float64 {
	if length <= 0 {
		return 1
	}
	u := (beat - start) / length
	if u < 0 {
		return 0
	}
	if u > 1 {
		return 1
	}
	return u
}

func cleanText(s string) string {
	s = strings.ReplaceAll(s, "<size=135%>", "\n")
	s = strings.ReplaceAll(s, "|", "\n")
	return s
}

func soundName(name string) string {
	name = strings.TrimPrefix(name, "rapMen/")
	name = strings.TrimPrefix(name, "Sounds/")
	name = strings.TrimSuffix(name, ".wav")
	name = strings.TrimSuffix(name, ".ogg")
	name = strings.ReplaceAll(name, "rapWoman/", "rapWomen/")
	return name
}

func itoa(v int) string { return strconv.Itoa(v) }

func nonzero(v, def float64) float64 {
	if math.Abs(v) < 1e-9 {
		return def
	}
	return v
}

func defaultBG() bgEvt {
	return bgEvt{length: 1, typ: 1, a0: defaultA, a1: defaultA, b0: defaultB, b1: defaultB, c0: defaultC, c1: defaultC, d0: defaultD, d1: defaultD}
}
