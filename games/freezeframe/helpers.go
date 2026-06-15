package freezeframe

import (
	"image/color"
	"math"
	"strconv"
	"strings"

	"hsdemo/engine"
	"hsdemo/kart"
	"hsdemo/riq"
)

const gameID = "freezeFrame"

type carType int

const (
	carSlow carType = iota
	carFast
)

type photoType int

const (
	photoRandom photoType = iota
	photoDefault
	photoNinja
	photoGhost
	photoRats
	photoPeace
	photoGirlfriendRight
	photoGirlfriendLeft
	photoDude1Right
	photoDude1Left
	photoDude2Right
	photoDude2Left
)

type personType int

const (
	personDude1 personType = iota
	personDude2
	personGirlfriend
)

type personDirection int

const (
	directionRandom personDirection = iota
	directionRight
	directionLeft
)

type gradeType int

const (
	gradeSymbols gradeType = iota
	gradeThumbs
	gradeNone
)

type bopEvt struct {
	beat, length     float64
	bop, autoBop     bool
	blink, autoBlink bool
}

type carCueEvt struct {
	beat     float64
	kind     carType
	photo    photoType
	mute     bool
	clear    bool
	autoShow bool
	grade    gradeType
	audience bool
}

type showPhotosEvt struct {
	beat, length float64
	grade        gradeType
	audience     bool
	clearCache   bool
}

type walkerEvt struct {
	beat, length float64
	kind         personType
	dir          personDirection
	layer        int
}

type crowdEvt struct {
	beat            float64
	show, custom    bool
	farLeft, left   int
	right, farRight int
	billboard       bool
}

type introSignEvt struct {
	beat, length float64
	enter        bool
	ease         int
}

type introLightsEvt struct {
	beat, length float64
	on           bool
}

type overlayEvt struct {
	beat                float64
	showOverlay, showTJ bool
	followCamera        bool
}

type cameraMoveEvt struct {
	beat, length float64
	move         bool
	startX       float64
	startY       float64
	endX         float64
	endY         float64
	rotate       bool
	startRot     float64
	endRot       float64
	scale        bool
	startSX      float64
	startSY      float64
	endSX        float64
	endSY        float64
	ease         int
}

type photoArgs struct {
	car   carType
	typ   photoType
	state int // -2 = empty/miss, -1 = early, 0 = perfect, 1 = late
	clear bool
}

type moveRuntime struct {
	active    bool
	beat, len float64
	x0, y0    float64
	x1, y1    float64
	ease      int
}

type rotateRuntime struct {
	active     bool
	beat, len  float64
	start, end float64
	ease       int
}

type scaleRuntime struct {
	active    bool
	beat, len float64
	x0, y0    float64
	x1, y1    float64
	ease      int
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

func roleOr(ctx *engine.Ctx, key, fallback string) string {
	if p := ctx.Role(key); p != "" {
		return p
	}
	return fallback
}

func nodePos(as *kart.Assets, path string) ([2]float64, bool) {
	for _, n := range as.Rig.Nodes {
		if n.Path == path {
			return n.Pos, true
		}
	}
	return [2]float64{}, false
}

func nodeScale(as *kart.Assets, path string) ([2]float64, bool) {
	for _, n := range as.Rig.Nodes {
		if n.Path == path {
			return n.Scale, true
		}
	}
	return [2]float64{}, false
}

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
	u := (beat - start) / length
	if u < 0 {
		return 0
	}
	if u > 1 {
		return 1
	}
	return u
}

func frozen01(v float64) float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func itoa(v int) string { return strconv.Itoa(v) }
