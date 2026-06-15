package sumobrothers

import (
	"image/color"
	"math"
	"strconv"

	"hsdemo/engine"
	"hsdemo/kart"
	"hsdemo/riq"
)

func roleOr(ctx *engine.Ctx, key, fallback string) string {
	if p := ctx.Role(key); p != "" {
		return p
	}
	return fallback
}

func refOr(compRefs map[string]string, key, fallback string) string {
	if p := compRefs[key]; p != "" {
		return p
	}
	return fallback
}

func intParam(e *riq.Entity, key string, def int) int {
	return int(e.Float(key, float64(def)))
}

func boolParam(e *riq.Entity, key string) bool { return boolDefault(e, key, false) }

func boolDefault(e *riq.Entity, key string, def bool) bool {
	if _, ok := e.Data[key]; !ok {
		return def
	}
	return e.Float(key, 0) != 0
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
	case map[string]any:
		return [4]float64{num(c["r"], def[0]), num(c["g"], def[1]), num(c["b"], def[2]), num(c["a"], def[3])}
	}
	return def
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

func colorAtEase(a, b [4]float64, ease int, u float64) [4]float64 {
	return [4]float64{
		engine.Ease(ease, a[0], b[0], u),
		engine.Ease(ease, a[1], b[1], u),
		engine.Ease(ease, a[2], b[2], u),
		engine.Ease(ease, a[3], b[3], u),
	}
}

func toRGBA(c [4]float64) color.NRGBA {
	return color.NRGBA{
		R: byte(clamp01(c[0]) * 255),
		G: byte(clamp01(c[1]) * 255),
		B: byte(clamp01(c[2]) * 255),
		A: byte(clamp01(c[3]) * 255),
	}
}

func poseSuffix(pose int) string {
	if pose <= 0 {
		return "1"
	}
	return strconv.Itoa(pose)
}

func bgStateName(bg int) string {
	switch bg {
	case bgGreatWave:
		return "GreatWave"
	case bgOtaniOniji:
		return "OtaniOniji"
	case bgNerd:
		return "Nerd"
	default:
		return "empty"
	}
}

func bgDarkStateName(bg int) string {
	switch bg {
	case bgGreatWave:
		return "GreatWaveDark"
	case bgOtaniOniji:
		return "OtaniOnijiDark"
	case bgNerd:
		return "NerdDark"
	default:
		return "empty"
	}
}

func (m *Module) play(path, state string, beat float64) {
	if path == "" || state == "" {
		return
	}
	m.ctx.Scene.PlayState(path, state, beat, 0.5)
}

func (m *Module) soundAt(beat float64, name string) {
	m.ctx.SoundAt(beat, name, 1)
}

func (m *Module) activeAt(beat float64) bool {
	g := m.ctx.GameAt(beat)
	return g == "" || g == gameID
}

func (m *Module) isPlaying(path, state string) bool {
	if path == "" || m.ctx == nil || m.ctx.Scene == nil {
		return false
	}
	cur, ok := m.ctx.Scene.StateInfo(path, m.ctx.Beat())
	return ok && cur == state
}

func (m *Module) barely(state float64) bool {
	return math.Abs(state) >= 1
}

func (m *Module) stopPoseLoop() {
	if m.poseLoopStop != nil {
		m.poseLoopStop()
		m.poseLoopStop = nil
	}
}

func (m *Module) currentBG(beat float64) (top, bottom [4]float64) {
	if !m.bgRun.active {
		return defaultBgTop, defaultBgBottom
	}
	u := 1.0
	if m.bgRun.length > 0 {
		u = clamp01((beat - m.bgRun.beat) / m.bgRun.length)
	}
	return colorAtEase(m.bgRun.top0, m.bgRun.top1, m.bgRun.ease, u),
		colorAtEase(m.bgRun.bot0, m.bgRun.bot1, m.bgRun.ease, u)
}

func (m *Module) backgroundPalette(beat float64) kart.Palette {
	top, bottom := m.currentBG(beat)
	return kart.Palette{Alpha: top, Fill: defaultWhite, Outline: bottom}
}

func (m *Module) mawashiPalette() kart.Palette {
	return kart.Palette{Alpha: m.mawashiRight, Fill: defaultWhite, Outline: m.mawashiLeft}
}
