// Package nightwalkagb ports Night Walk GBA's platform stream, Play-Yan jump
// paths, star field evolution, count-ins, roll cues, fish shocks, and palette
// events from Heaven Studio's AgbNightWalk scripts.
package nightwalkagb

import (
	"image/color"
	"math"
	"strings"

	"hsdemo/kart"
	"hsdemo/kmdata"
	"hsdemo/riq"
)

const (
	gameID    = "nightWalkAgb"
	actionAlt = 3 // HS InputAction_AltDown uses the South/down-style action.

	platformFlower   = 1
	platformLollipop = 2
	platformUmbrella = 3

	fillNone = iota
	fillPattern1
	fillPattern2
	fillPattern3
)

var (
	defaultPlatform      = rgba01(0.5019608, 0.9176471, 0.8862745, 1)
	defaultPlatformBeam  = rgba01(0.06666667, 0.1607843, 0.509804, 1)
	defaultPlatformLight = rgba01(0, 0, 0.8, 1)
	defaultFish          = rgba01(0.4666667, 0.8901961, 1, 1)
	defaultFishShock     = rgba01(0.9960784, 0.99607841, 0.09411765, 1)
	defaultFishShade     = rgba01(0.9529412, 0.772549, 0.02745098, 1)
	defaultStar          = rgba01(0.01176471, 0.08627451, 0.9529412, 1)
	defaultStarFace      = rgba01(0.5176471, 0.8431373, 0.9607843, 1)
	black                = rgba01(0, 0, 0, 1)
)

// hitJumpsPersist mirrors the static AgbNightWalk.hitJumpsPersist counter.
// Heaven Studio resets it only when playback stops, not between switchGame
// entries, so keeping package state preserves multi-segment end requirements.
var hitJumpsPersist int

type countInEvt struct {
	beat, length float64
	mute         bool
	kind         int
}

type rawHeightEvt struct {
	beat       float64
	value      int
	rmin, rmax int
}

type heightEvt struct {
	beat  float64
	value int
}

type typeEvt struct {
	beat         float64
	platformType int
	fillType     int
}

type rollEvt struct {
	beat, length float64
	mute         bool
	valid        bool
}

type endEvt struct {
	beat       float64
	minAmount  int
	minAmountP int
}

type noJumpEvt struct {
	beat, length float64
}

type textboxEvt struct {
	beat, length        float64
	text                string
	x, y, width, height float64
}

type bgEvt struct {
	beat, length float64
	startTop     [4]float64
	endTop       [4]float64
	startBottom  [4]float64
	endBottom    [4]float64
	ease         int
}

type colorEvt struct {
	beat          float64
	platform      [4]float64
	platformBeam  [4]float64
	platformLight [4]float64
	fish          [4]float64
	fishShock     [4]float64
	fishShade     [4]float64
	star          [4]float64
	starFace      [4]float64
}

type platformCfg struct {
	defaultYPos      float64
	heightAmount     float64
	platformDistance float64
	playerXPos       float64
	starLength       float64
	starHeight       float64
	platformCount    int
	root             string
	platform         string
	fallYan          string
	fallYanRoll      string
	fish             string
	rollPlatform     string
	rollLong         string
	rollLong2        string
}

type starCfg struct {
	root           string
	boundaryX      float64
	boundaryY      float64
	starCount      int
	blinkFrequency float64
	blinkAmount    int
}

type jumpPath struct {
	name     string
	duration float64
	height   float64
	start    [2]float64
	end      [2]float64
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

func rgba01(r, g, b, a float64) [4]float64 { return [4]float64{r, g, b, a} }

func rgba(c [4]float64) color.NRGBA {
	return color.NRGBA{
		R: byte(clamp01(c[0]) * 255),
		G: byte(clamp01(c[1]) * 255),
		B: byte(clamp01(c[2]) * 255),
		A: byte(clamp01(c[3]) * 255),
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

func norm(beat, start, length float64) float64 {
	if length <= 0 {
		return 1
	}
	return clamp01((beat - start) / length)
}

func nearBeat(a, b float64) bool { return math.Abs(a-b) < 1e-6 }

func relPath(root, path string) string {
	path = strings.TrimPrefix(path, root)
	return strings.TrimPrefix(path, "/")
}

func ref(m map[string]string, key string) string {
	if m == nil {
		return ""
	}
	return m[key]
}

func num(m map[string]float64, key string, def float64) float64 {
	if m == nil {
		return def
	}
	if v, ok := m[key]; ok {
		return v
	}
	return def
}

func nonzeroInt(v, def int) int {
	if v == 0 {
		return def
	}
	return v
}

func highJumpSound(n int) string { return "common_games_nightWalkRvl_highJump" + string(rune('0'+n)) }

func platformState(kind int, barely bool) string {
	switch kind {
	case platformLollipop:
		if barely {
			return "LollipopBarely"
		}
		return "Lollipop"
	case platformUmbrella:
		if barely {
			return "UmbrellaBarely"
		}
		return "Umbrella"
	default:
		if barely {
			return "FlowerBarely"
		}
		return "Flower"
	}
}

func readPath(item kmdata.ComponentItem) jumpPath {
	p := jumpPath{name: item.Strs["name"]}
	if arr := item.Items["positions"]; len(arr) >= 2 {
		a, b := arr[0], arr[1]
		p.duration = a.Nums["duration"]
		p.height = a.Nums["height"]
		p.start = [2]float64{a.Nums["pos.x"], a.Nums["pos.y"]}
		p.end = [2]float64{b.Nums["pos.x"], b.Nums["pos.y"]}
	}
	return p
}

func samplePath(p jumpPath, beat, startBeat float64) (float64, float64) {
	if p.duration <= 0 {
		return p.end[0], p.end[1]
	}
	u := (beat - startBeat) / p.duration
	x := p.start[0] + (p.end[0]-p.start[0])*u
	y := p.start[1] + (p.end[1]-p.start[1])*u
	yMul := u*2 - 1
	y += (-(yMul * yMul) + 1) * p.height
	return x, y
}

func palette(alpha, bravo, delta [4]float64) kart.Palette {
	return kart.Palette{Alpha: alpha, Fill: bravo, Outline: delta}
}
