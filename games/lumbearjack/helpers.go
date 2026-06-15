package lumbearjack

import (
	"math"
	"strings"

	"hsdemo/engine"
	"hsdemo/kart"
	"hsdemo/kmdata"
	"hsdemo/riq"
)

const gameID = "lumbearjack"

type whoBops int

const (
	whoBoth whoBops = iota
	whoBear
	whoCats
	whoNone
)

type smallType int

const (
	smallLog smallType = iota
	smallCan
	smallBat
	smallBroom
	smallBarrel
	smallBook
)

type bigType int

const (
	bigLog bigType = iota
	bigBall
)

type hugeType int

const (
	hugeLog hugeType = iota
	hugeFreezer
	hugePeach
)

type huhChoice int

const (
	huhObjectSpecific huhChoice = iota
	huhOff
	huhOn
)

type catPutChoice int

const (
	catAlternate catPutChoice = iota
	catRight
	catLeft
)

type mainCatChoice int

const (
	mainCatRight mainCatChoice = iota
	mainCatLeft
	mainCatBoth
)

type restSoundChoice int

const (
	restRandom restSoundChoice = iota
	restA
	restB
	restNoSound
)

type objectKind int

const (
	objSmall objectKind = iota
	objBig
	objHuge
)

type bopEvt struct {
	beat, length float64
	bop, auto    whoBops
}

type objectEvt struct {
	beat, length     float64
	kind             objectKind
	small            smallType
	big              bigType
	huge             hugeType
	huh              huhChoice
	cat              catPutChoice
	bomb, zoom, baby bool
	sound, pBaby     bool
}

type catPresenceEvt struct {
	beat, length float64
	main         mainCatChoice
	bg           int
	instant      bool
	dance        bool
}

type snowEvt struct {
	beat, length    float64
	on, instant     bool
	wind, particles float64
}

type restEvt struct {
	beat    float64
	instant bool
	sound   restSoundChoice
}

type catMoveSpec struct {
	path        string
	this, other [2]float64
}

type catMoveRuntime struct {
	spec      catMoveSpec
	startBeat float64
	length    float64
	inToScene bool
}

type snowflake struct {
	x, y, speed, drift float64
}

func actionName(e *riq.Entity) string {
	if s, ok := strings.CutPrefix(e.Datamodel, gameID+"/"); ok {
		return s
	}
	return e.Datamodel
}

func boolParam(e *riq.Entity, k string) bool { return e.Float(k, 0) != 0 }

func boolDefault(e *riq.Entity, k string, def bool) bool {
	if _, ok := e.Data[k]; !ok {
		return def
	}
	return e.Float(k, 0) != 0
}

func intDefault(e *riq.Entity, k string, def int) int {
	if _, ok := e.Data[k]; !ok {
		return def
	}
	return int(e.Float(k, float64(def)))
}

func floatDefault(e *riq.Entity, k string, def float64) float64 {
	if _, ok := e.Data[k]; !ok {
		return def
	}
	return e.Float(k, def)
}

func sec(ctx *engine.Ctx, beat float64) float64 {
	if ctx == nil {
		return 0.5
	}
	return ctx.SecPerBeat(beat)
}

func componentByPath(comps map[string]kmdata.Component, path string) (kmdata.Component, bool) {
	for _, c := range comps {
		if c.Path == path {
			return c, true
		}
	}
	return kmdata.Component{}, false
}

func nodePos(as *kart.Assets, path string) ([2]float64, bool) {
	for _, n := range as.Rig.Nodes {
		if n.Path == path {
			return n.Pos, true
		}
	}
	return [2]float64{}, false
}

func rel(root, path string) string {
	if path == root {
		return ""
	}
	return strings.TrimPrefix(path, root+"/")
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

func easeInOutQuad(a, b, t float64) float64 {
	t = clamp01(t)
	if t < 0.5 {
		return a + (b-a)*2*t*t
	}
	u := -1 + (4-2*t)*t
	return a + (b-a)*u
}

func radians(deg float64) float64 { return deg * math.Pi / 180 }

func nearBeat(a, b float64) bool { return math.Abs(a-b) < 1e-4 }

func signedSeed(beat float64, salt float64) float64 {
	v := math.Sin(beat*12.9898+salt*78.233) * 43758.5453
	return v - math.Floor(v)
}
