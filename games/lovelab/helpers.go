package lovelab

import (
	"math"
	"sort"

	"hsdemo/engine"
	"hsdemo/kart"
	"hsdemo/kmdata"
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

func boolDefault(e *riq.Entity, key string, def bool) bool {
	if _, ok := e.Data[key]; !ok {
		return def
	}
	return e.Float(key, 0) != 0
}

func intParam(e *riq.Entity, key string, def int) int {
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
	case bool:
		if n {
			return 1
		}
		return 0
	}
	return def
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

func templateLast(as *kart.Assets, path string) *kart.Template {
	for i := len(as.Rig.Nodes) - 1; i >= 0; i-- {
		if as.Rig.Nodes[i].Path == path {
			return kart.NewTemplateIdx(as, i)
		}
	}
	return nil
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

func (m *Module) activeAt(beat float64) bool {
	g := m.ctx.GameAt(beat)
	return g == "" || g == gameID
}

func (m *Module) canHitNow() bool { return m.activeAt(m.ctx.Beat()) }

func (m *Module) currentBeat() float64 {
	if m.ctx == nil {
		return 0
	}
	return m.ctx.Beat()
}

func (m *Module) shakesBetween(start, end float64) []shakeEvt {
	out := make([]shakeEvt, 0)
	for _, s := range m.shakes {
		if s.beat >= start-1e-6 && s.beat < end-1e-6 {
			out = append(out, s)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].beat < out[j].beat })
	return out
}

func (m *Module) inBopRegion(beat float64) bool {
	for _, ev := range m.bops {
		if ev.auto && beat >= ev.beat-1e-6 && beat < ev.beat+ev.length-1e-6 {
			return true
		}
	}
	return false
}

func (m *Module) nodeWorld(path string) (float64, float64, bool) {
	idx := -1
	for i := range m.ctx.Assets.Rig.Nodes {
		if m.ctx.Assets.Rig.Nodes[i].Path == path {
			idx = i
			break
		}
	}
	if idx < 0 {
		return 0, 0, false
	}
	var chain []int
	for idx >= 0 {
		chain = append(chain, idx)
		idx = m.ctx.Assets.Rig.Nodes[idx].Parent
	}
	world := kart.Identity()
	for i := len(chain) - 1; i >= 0; i-- {
		n := m.ctx.Assets.Rig.Nodes[chain[i]]
		world = world.Mul(kart.TRS(n.Pos[0], n.Pos[1], n.RotZ, n.Scale[0], n.Scale[1]))
	}
	x, y := world.Apply(0, 0)
	return x, y, true
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

func easeOutBack(t float64) float64 {
	const c1 = 1.70158
	const c3 = c1 + 1
	x := t - 1
	return 1 + c3*x*x*x + c1*x*x
}

func easeInQuad(t float64) float64 { return t * t }

func flaskPalette(c [4]float64) kart.Palette {
	p := kart.DefaultPalette()
	p.Alpha = c
	return p
}

func characterPalette(alpha, bravo, delta [4]float64) kart.Palette {
	p := kart.DefaultPalette()
	p.Alpha = alpha
	p.Fill = bravo
	p.Outline = delta
	return p
}

func compRef(c kmdata.Component, key, fallback string) string {
	if c.Refs != nil {
		if v := c.Refs[key]; v != "" {
			return v
		}
	}
	return fallback
}
