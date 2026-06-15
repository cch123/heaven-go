package fillbots

import (
	"image/color"
	"math"
	"strings"

	"hsdemo/engine"
	"hsdemo/kart"
	"hsdemo/riq"
)

const gameID = "fillbots"

const (
	sizeSmall botSize = iota
	sizeMedium
	sizeLarge
)

const (
	endBoth endAnim = iota
	endAce
	endJust
)

const (
	stateIdle botState = iota
	stateHolding
	stateAce
	stateJust
	stateNG
	stateDance
)

var (
	defaultBG       = [4]float64{1, 1, 1, 1}
	defaultMeter    = [4]float64{1, 0.88, 0.88, 1}
	defaultFuel     = [4]float64{1, 0.385, 0.385, 1}
	defaultLampOff  = [4]float64{0.635, 0.635, 0.185, 1}
	defaultLampOn   = [4]float64{1, 1, 0.42, 1}
	defaultImpact   = [4]float64{1, 0.59, 0.01, 1}
	defaultRenderer = [4]float64{1, 1, 1, 1}
	transparent     = [4]float64{}
)

type botSize int
type endAnim int
type botState int

type bopEvt struct {
	beat, length float64
	toggle       bool
	auto         bool
}

type bgEvt struct {
	beat, length float64
	ease         int
	bg0, bg1     [4]float64
	m0, m1       [6][4]float64
}

type objectEvt struct {
	beat                     float64
	fuel, lampOff, lampOn    [4]float64
	impact, filler, conveyer [4]float64
}

type spawnEvt struct {
	beat, holdLength       float64
	size                   botSize
	fuel, lampOff, lampOn  [4]float64
	end                    endAnim
	altOK, stop, customCol bool
}

type botSpec struct {
	root              string
	holdLength        float64
	limbFallHeight    float64
	flyDistance       float64
	stackDistanceRate float64
	legsBase          [2]float64
	bodyBase          [2]float64
	headBase          [2]float64
	rootScale         [2]float64
	fillPosY          float64
	fillScaleY        float64
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

func colorParam(e *riq.Entity, key string, def [4]float64) [4]float64 {
	v, ok := e.Data[key]
	if !ok {
		return def
	}
	switch c := v.(type) {
	case []any:
		if len(c) >= 4 {
			return [4]float64{num(c[0], def[0]), num(c[1], def[1]), num(c[2], def[2]), num(c[3], def[3])}
		}
	case []float64:
		if len(c) >= 4 {
			return [4]float64{c[0], c[1], c[2], c[3]}
		}
	case map[string]any:
		return [4]float64{num(c["r"], def[0]), num(c["g"], def[1]), num(c["b"], def[2]), num(c["a"], def[3])}
	}
	return def
}

func num(v any, def float64) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	default:
		return def
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

func lerp(a, b, u float64) float64 { return a + (b-a)*u }

func colorAtEase(a, b [4]float64, ease int, u float64) [4]float64 {
	return [4]float64{
		engine.Ease(ease, a[0], b[0], u),
		engine.Ease(ease, a[1], b[1], u),
		engine.Ease(ease, a[2], b[2], u),
		engine.Ease(ease, a[3], b[3], u),
	}
}

func rgba(c [4]float64) color.NRGBA {
	return color.NRGBA{
		R: uint8(clamp01(c[0])*255 + 0.5),
		G: uint8(clamp01(c[1])*255 + 0.5),
		B: uint8(clamp01(c[2])*255 + 0.5),
		A: uint8(clamp01(c[3])*255 + 0.5),
	}
}

func botSuffix(size botSize) string {
	switch size {
	case sizeSmall:
		return "Small"
	case sizeLarge:
		return "Large"
	default:
		return "Medium"
	}
}

func botPrefix(size botSize) string {
	switch size {
	case sizeSmall:
		return "small"
	case sizeLarge:
		return "big"
	default:
		return "medium"
	}
}

func botRoot(size botSize) string { return "Bot" + botSuffix(size) }

func botFillClip(size botSize) string { return "Animations/" + botSuffix(size) + "/Fill" }

func botPalette(fuel, lamp [4]float64) kart.Palette {
	return kart.Palette{
		Alpha:   lamp,
		Fill:    fuel,
		Outline: [4]float64{1, 1, 1, 1},
	}
}

func impactPalette(impact [4]float64) kart.Palette {
	return kart.Palette{
		Alpha:   impact,
		Fill:    [4]float64{1, 1, 1, 1},
		Outline: [4]float64{1, 1, 1, 1},
	}
}

func nodePos(as *kart.Assets, path string) [2]float64 {
	for i := range as.Rig.Nodes {
		if as.Rig.Nodes[i].Path == path {
			return as.Rig.Nodes[i].Pos
		}
	}
	return [2]float64{}
}

func nodeScale(as *kart.Assets, path string) [2]float64 {
	for i := range as.Rig.Nodes {
		if as.Rig.Nodes[i].Path == path {
			return as.Rig.Nodes[i].Scale
		}
	}
	return [2]float64{1, 1}
}

func animLastY(as *kart.Assets, clip, group string, scale bool) float64 {
	anim := as.Anims[clip]
	if anim == nil {
		return 0
	}
	if scale {
		if c, ok := anim.Scale[group]; ok && len(c.Y) > 0 {
			return c.Y[len(c.Y)-1].V
		}
		return 0
	}
	if c, ok := anim.Pos[group]; ok && len(c.Y) > 0 {
		return c.Y[len(c.Y)-1].V
	}
	return 0
}

func secondsScale(ctx *engine.Ctx, beat float64) float64 {
	if ctx == nil {
		return 0.5
	}
	return ctx.SecPerBeat(math.Max(beat, 0))
}

func normalized(start, length, beat float64) float64 {
	if length <= 0 {
		if beat >= start {
			return 1
		}
		return 0
	}
	return (beat - start) / length
}

func hasPrefixDatamodel(e *riq.Entity, action string) bool {
	return strings.HasPrefix(e.Datamodel, gameID+"/"+action)
}
