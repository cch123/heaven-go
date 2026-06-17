package airboarder

import (
	"image/color"
	"math"

	"hsdemo/engine"
	"hsdemo/kmdata"
	"hsdemo/riq"
)

func boolDefault(e *riq.Entity, key string, def bool) bool {
	if _, ok := e.Data[key]; !ok {
		return def
	}
	return e.Float(key, 0) != 0
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

func (m *Module) autoBopAt(beat float64) bool {
	on := false
	for _, ev := range m.bops {
		if ev.beat > beat {
			break
		}
		on = ev.auto
	}
	return on
}

func (m *Module) bgAt(beat float64) (sky, cloud [4]float64) {
	return m.colorPairAt(m.bgEvents, beat, defaultBG, defaultCloud)
}

func (m *Module) floorAt(beat float64) (floor, stripe [4]float64) {
	return m.colorPairAt(m.floorEvts, beat, defaultFloor, defaultStripe)
}

func (m *Module) colorPairAt(events []colorEvt, beat float64, da, db [4]float64) ([4]float64, [4]float64) {
	a, b := da, db
	for _, ev := range events {
		if ev.beat > beat {
			break
		}
		u := 1.0
		if ev.length > 0 && beat < ev.beat+ev.length {
			u = (beat - ev.beat) / ev.length
		}
		a = easeColor(ev.ease, ev.a0, ev.a1, u)
		b = easeColor(ev.ease, ev.b0, ev.b1, u)
	}
	return a, b
}

func easeColor(kind int, a, b [4]float64, u float64) [4]float64 {
	return [4]float64{
		engine.Ease(kind, a[0], b[0], u),
		engine.Ease(kind, a[1], b[1], u),
		engine.Ease(kind, a[2], b[2], u),
		engine.Ease(kind, a[3], b[3], u),
	}
}

func (m *Module) cameraStateAt(beat float64) cameraState {
	const defaultPivot = 0
	var (
		seen               bool
		activeBeat         float64
		activeLength       float64
		activeEase         int
		lastRotY, nextRotY float64
		lastRotX, nextRotX float64
		lastZoom, nextZoom = 1.0, 1.0
		lastX, nextX       float64
		lastY, nextY       float64
		activePivot        = defaultPivot
	)
	for _, ev := range m.cameraEvts {
		if ev.beat > beat {
			break
		}
		seen = true
		activeBeat, activeLength, activeEase = ev.beat, ev.length, ev.ease
		lastRotY = math.Mod(nextRotY, 360)
		lastRotX, nextRotX = nextRotX, ev.rotX
		lastZoom, nextZoom = nextZoom, ev.zoom
		lastX, nextX = nextX, ev.x
		lastY, nextY = nextY, ev.y
		if ev.additive {
			nextRotY = lastRotY + ev.rotY
		} else {
			nextRotY = ev.rotY
		}
		activePivot = ev.pivot
	}
	if !seen {
		return cameraState{zoom: 1, pivot: defaultPivot}
	}
	u := 1.0
	if activeLength > 0 && beat < activeBeat+activeLength {
		u = (beat - activeBeat) / activeLength
	}
	return cameraState{
		rotY:  engine.Ease(activeEase, lastRotY, nextRotY, u),
		rotX:  engine.Ease(activeEase, lastRotX, nextRotX, u),
		zoom:  engine.Ease(activeEase, lastZoom, nextZoom, u),
		x:     engine.Ease(activeEase, lastX, nextX, u),
		y:     engine.Ease(activeEase, lastY, nextY, u),
		pivot: activePivot,
	}
}

func cameraZoomAdd(zoom float64) float64 {
	if zoom <= 0 {
		return 0
	}
	// The original camera scales the CameraPivot. Until the 3D camera path is
	// represented directly, convert zoom into a conservative local Z offset.
	return (1/zoom - 1) * 5
}

func floorMoveDelta(anim *kmdata.Anim) float64 {
	if anim == nil {
		return 0
	}
	keys := anim.Pos[""].X
	if len(keys) < 2 {
		return 0
	}
	return keys[len(keys)-1].V - keys[0].V
}

func (m *Module) floorStripeOffset(beat, spacing float64) float64 {
	if spacing <= 0 {
		return 0
	}
	if m.floorLoopDelta == 0 {
		return math.Mod(beat*62, spacing)
	}
	// Airboarder.Update drives Floor.Play("moving", 0,
	// GetPositionFromBeat(startBeat, 5f)) every frame. The visible fallback
	// floor must use that same 5-beat cycle instead of an arbitrary beat speed,
	// otherwise it visibly drifts against the extracted floor controller.
	phase := math.Mod(beat/5, 1)
	if phase < 0 {
		phase += 1
	}
	return math.Mod(phase*m.floorLoopDelta*54, spacing)
}

func rgba(c [4]float64) color.RGBA {
	clamp := func(v float64) uint8 {
		if v <= 0 {
			return 0
		}
		if v >= 1 {
			return 255
		}
		return uint8(math.Round(v * 255))
	}
	return color.RGBA{R: clamp(c[0]), G: clamp(c[1]), B: clamp(c[2]), A: clamp(c[3])}
}

func lerp(a, b, u float64) float64 { return a + (b-a)*u }

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
