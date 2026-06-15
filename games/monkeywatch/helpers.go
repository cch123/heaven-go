package monkeywatch

import (
	"math"
	"strconv"
	"strings"

	"hsdemo/kart"
	"hsdemo/riq"
)

const (
	gameID          = "monkeyWatch"
	degreePerMonkey = 6.0
)

type monkeyEventKind int

const (
	eventClap monkeyEventKind = iota
	eventPink
	eventPinkInterval
)

type clapEvt struct {
	beat, length float64
	auto         bool
	min          int
}

type pinkEvt struct {
	beat, length float64
	interval     bool
	muteOoki     bool
	muteEek      bool
}

type customPinkEvt struct {
	beat float64
}

type appearEvt struct {
	beat, length float64
	count        int
}

type zoomEvt struct {
	beat         float64
	out          bool
	instant      bool
	timeMode     int
	hour, minute int
}

type balloonEvt struct {
	beat, length   float64
	x0, x1, y0, y1 float64
	a0, a1         float64
	ease           int
}

type monkeyTimelineEvt struct {
	beat, length float64
	kind         monkeyEventKind
	min          int
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
	return e.Float(key, 0) != 0
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

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func radians(deg float64) float64 { return deg * math.Pi / 180 }

func round(v float64) int { return int(math.Round(v)) }

func ceil(v float64) int { return int(math.Ceil(v)) }

func ease(a, b, t float64, kind int) float64 {
	t = clamp01(t)
	switch kind {
	case 3: // EaseInQuad in HS enum table for this game's zoom-out default.
		t = t * t
	case 9: // EaseOutQuad in HS enum table for this game's zoom-in default.
		t = 1 - (1-t)*(1-t)
	default:
	}
	return a + (b-a)*t
}

func seed01(beat, salt float64) float64 {
	v := math.Sin(beat*12.9898+salt*78.233) * 43758.5453
	return v - math.Floor(v)
}

func soundChoice(base string, beat float64, max int) string {
	n := 1 + int(seed01(beat, float64(max))*float64(max))
	if n < 1 {
		n = 1
	}
	if n > max {
		n = max
	}
	return base + itoa(n)
}

func itoa(v int) string { return strconv.Itoa(v) }

func monkeyDirection(index int) int {
	switch {
	case index >= 4 && index <= 12:
		return 2
	case index >= 12 && index <= 19:
		return 3
	case index >= 19 && index <= 27:
		return 4
	case index >= 27 && index <= 34:
		return 5
	case index >= 34 && index <= 42:
		return 6
	case index >= 42 && index <= 49:
		return 7
	case index >= 49 && index <= 56:
		return 8
	default:
		return 1
	}
}
