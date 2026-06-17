package wariodemambo

import (
	"math"

	"hsdemo/engine"
	"hsdemo/kart"
	"hsdemo/riq"
)

func eventName(e *riq.Entity) string {
	for i := len(e.Datamodel) - 1; i >= 0; i-- {
		if e.Datamodel[i] == '/' {
			return e.Datamodel[i+1:]
		}
	}
	return e.Datamodel
}

func roleOr(ctx *engine.Ctx, key, fallback string) string {
	if p := ctx.Role(key); p != "" {
		return p
	}
	return fallback
}

func boolParam(e *riq.Entity, key string) bool { return boolDefault(e, key, false) }

func boolDefault(e *riq.Entity, key string, def bool) bool {
	if _, ok := e.Data[key]; !ok {
		return def
	}
	return e.Float(key, 0) != 0
}

func intParam(e *riq.Entity, key string, def int) int {
	return int(e.Float(key, float64(def)))
}

func numField(nums map[string]float64, key string, def float64) float64 {
	if v, ok := nums[key]; ok {
		return v
	}
	return def
}

func colorField(nums map[string]float64, prefix string, def [4]float64) [4]float64 {
	return [4]float64{
		numField(nums, prefix+".r", def[0]),
		numField(nums, prefix+".g", def[1]),
		numField(nums, prefix+".b", def[2]),
		numField(nums, prefix+".a", def[3]),
	}
}

func (m *Module) activeAt(beat float64) bool {
	g := m.ctx.GameAt(beat)
	return g == "" || g == gameID
}

func (m *Module) play(path, state string, beat float64) {
	if path == "" || state == "" {
		return
	}
	m.ctx.Scene.PlayState(path, state, beat, 0.5)
}

func (m *Module) playDefault(path string, beat float64) {
	if path == "" {
		return
	}
	m.ctx.Scene.PlayDefaultState(path, beat, m.ctx.SecPerBeat(math.Max(beat, 0)))
}

func (m *Module) point(path string) [2]float64 {
	if m == nil || m.ctx == nil || m.ctx.Assets == nil {
		return [2]float64{}
	}
	byPath := map[string]int{}
	for i, n := range m.ctx.Assets.Rig.Nodes {
		byPath[n.Path] = i
	}
	i, ok := byPath[path]
	if !ok {
		return [2]float64{}
	}
	stack := make([]int, 0, 8)
	for i >= 0 {
		stack = append(stack, i)
		i = m.ctx.Assets.Rig.Nodes[i].Parent
	}
	w := kart.Identity()
	for j := len(stack) - 1; j >= 0; j-- {
		n := m.ctx.Assets.Rig.Nodes[stack[j]]
		w = w.Mul(kart.TRS(n.Pos[0], n.Pos[1], n.RotZ, n.Scale[0], n.Scale[1]))
	}
	return [2]float64{w.Tx, w.Ty}
}

func (m *Module) mamboMainMaterialExcludes() []string {
	return []string{
		m.warioFace,
		m.warioBody + "/Squiggly",
		// Unity exposes mainMat as a serialized material instance, while the
		// extractor collapses several SpriteRenderer material references to the
		// same asset name. Only Wario's separate face instance and the squiggly
		// overlay are outside this scripted mainMat write; dancer heads stay on
		// mainMat in the prefab and must receive the same MamboDoodle pass.
	}
}

func (m *Module) currentBeat() float64 {
	if m.ctx == nil {
		return 0
	}
	return m.ctx.Beat()
}

func (m *Module) appendExpectedInput(beat float64) {
	m.expectedInputs = append(m.expectedInputs, beat)
}

func (m *Module) expectingAnyWarioInputNow() bool {
	now := m.ctx.Time()
	for _, b := range m.expectedInputs {
		t := m.ctx.BeatToTime(b)
		if math.Abs(now-t) <= engine.WinNG {
			return true
		}
	}
	return false
}

func (m *Module) canHitNow() bool {
	return m.activeAt(m.ctx.Beat())
}

func (m *Module) inBopRegion(beat float64) bool {
	for _, ev := range m.bops {
		if ev.auto && beat >= ev.beat-1e-6 && beat < ev.beat+ev.length-1e-6 {
			return true
		}
	}
	return false
}

func (m *Module) intervalIsGoingOn(beat float64) bool {
	for _, iv := range m.intervals {
		if beat >= iv.beat && beat < iv.beat+iv.length {
			return true
		}
	}
	return false
}

func (m *Module) lastIntervalBefore(beat float64) (intervalEvt, bool) {
	for i := len(m.intervals) - 1; i >= 0; i-- {
		iv := m.intervals[i]
		if iv.beat <= beat {
			return iv, true
		}
	}
	return intervalEvt{}, false
}

func (m *Module) inputsBetween(start, end float64) []inputEvt {
	out := make([]inputEvt, 0)
	for _, in := range m.inputs {
		if in.beat >= start-1e-6 && in.beat < end-1e-6 {
			out = append(out, in)
		}
	}
	return out
}

func (m *Module) crHasEventAt(beat float64) bool {
	for _, ev := range m.crEvents {
		if math.Abs(ev.beat-beat) < 1e-6 {
			return true
		}
	}
	return false
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

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
