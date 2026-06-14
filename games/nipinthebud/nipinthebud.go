// Package nipinthebud ports Nip In the Bud's Leilani bop/prepare flow,
// mosquito and mayfly Bezier paths, Bubble warning, background color fade,
// and catch/barely/miss/whiff reactions.
package nipinthebud

import (
	"math"
	"sort"

	"github.com/hajimehoshi/ebiten/v2"

	"hsdemo/engine"
	"hsdemo/kart"
	"hsdemo/kmdata"
	"hsdemo/riq"
)

const (
	bugMosquito = iota
	bugMayfly
)

const (
	bugStarting = iota
	bugApproaching
	bugFleeing
	bugExiting
)

const faceLayerKey = "nipInTheBud:face"

var defaultBG = [4]float64{0.5215686274509804, 0.796078431372549, 1, 1}

type bopEvt struct {
	beat, length float64
	bop, auto    bool
}

type bgEvt struct {
	beat, length float64
	from, to     [4]float64
	ease         int
}

type bugCue struct {
	beat     float64
	kind     int
	reaction bool
}

type bug struct {
	cue                               bugCue
	inst                              *kart.Instance
	state                             int
	startBeat, approachBeat, exitBeat float64
	fleeBeat                          float64
	dead                              bool
	mirrored                          bool
	z                                 float64
}

type Module struct {
	ctx  *engine.Ctx
	proj kart.Aff

	leilani string
	bubble  string
	bg      string

	mosquitoT *kart.Template
	mayflyT   *kart.Template
	curves    map[string]kmdata.Curve

	bops []bopEvt
	bgs  []bgEvt
	cues []bugCue
	bugs []*bug

	bopExpression string
	queuePrepare  bool
	preparing     bool
	queueBopReset bool
}

func New() engine.Module { return &Module{bopExpression: "Neutral"} }

func (m *Module) ID() string { return "nipInTheBud" }

func (m *Module) Load(ctx *engine.Ctx) error {
	m.ctx = ctx
	if err := ctx.LoadAssets("nipInTheBud"); err != nil {
		return err
	}
	m.proj = kart.Translate(engine.ScreenW/2, engine.ScreenH/2).Mul(kart.Scale(54, -54))
	m.leilani = roleOr(ctx, "Leilani", "Scene/Leilani")
	m.bubble = roleOr(ctx, "Bubble", "bubble")
	m.bg = roleOr(ctx, "bg", "bg stuff/sky")
	m.curves = ctx.Assets.Extra.Curves
	m.mosquitoT = kart.NewTemplate(ctx.Assets, roleOr(ctx, "Mosquito", "Scene/mosquito"))
	m.mayflyT = kart.NewTemplate(ctx.Assets, roleOr(ctx, "Mayfly", "Scene/mayfly"))
	m.resetScene(0)
	return nil
}

func roleOr(ctx *engine.Ctx, key, fallback string) string {
	if p := ctx.Role(key); p != "" {
		return p
	}
	return fallback
}

func (m *Module) OnEvent(e *riq.Entity) {
	switch e.Datamodel {
	case "nipInTheBud/bop":
		m.bops = append(m.bops, bopEvt{
			beat: e.Beat, length: e.Length,
			bop:  boolDefault(e, "toggle", true),
			auto: boolParam(e, "auto"),
		})
	case "nipInTheBud/prepare":
		b := e.Beat
		m.ctx.At(b, func() { m.doPrepare(b) })
	case "nipInTheBud/spawnMosquito":
		m.cues = append(m.cues, bugCue{beat: e.Beat, kind: bugMosquito, reaction: boolParam(e, "reaction")})
	case "nipInTheBud/spawnMayfly":
		m.cues = append(m.cues, bugCue{beat: e.Beat, kind: bugMayfly, reaction: boolDefault(e, "reaction", true)})
	case "nipInTheBud/fade background":
		m.bgs = append(m.bgs, bgEvt{
			beat: e.Beat, length: e.Length,
			from: colorParam(e, "colorStart", defaultBG),
			to:   colorParam(e, "colorEnd", defaultBG),
			ease: int(e.Float("ease", 0)),
		})
	}
}

func (m *Module) Ready() {
	sort.SliceStable(m.bops, func(i, j int) bool { return m.bops[i].beat < m.bops[j].beat })
	sort.SliceStable(m.bgs, func(i, j int) bool { return m.bgs[i].beat < m.bgs[j].beat })
	sort.SliceStable(m.cues, func(i, j int) bool { return m.cues[i].beat < m.cues[j].beat })

	bopBeats := map[float64]bool{}
	for i, ev := range m.bops {
		if ev.bop {
			for k := 0; float64(k) < ev.length; k++ {
				bopBeats[ev.beat+float64(k)] = true
			}
		}
		if ev.auto {
			end := ev.beat + ev.length
			for j := i + 1; j < len(m.bops); j++ {
				if m.bops[j].beat > ev.beat && m.bops[j].beat < end {
					end = m.bops[j].beat
					break
				}
			}
			for b := math.Ceil(ev.beat); b < end; b++ {
				bopBeats[b] = true
			}
		}
	}
	for b := range bopBeats {
		b := b
		m.ctx.At(b, func() { m.bop(b) })
	}

	for _, cue := range m.cues {
		cue := cue
		switch cue.kind {
		case bugMosquito:
			m.scheduleMosquito(cue)
		case bugMayfly:
			m.scheduleMayfly(cue)
		}
	}
}

func (m *Module) OnSwitch(beat float64) {
	m.resetScene(beat)
	m.bugs = liveBugs(m.bugs, beat)
}

func (m *Module) Whiff(beat float64) {
	m.playBody("SnapWhiff", beat, 0.5)
	m.ctx.Sound("whiff")
}

func (m *Module) Update(_ float64, beat float64) {
	if m.queuePrepare && !m.preparing && m.readyToPrepare(beat) {
		m.doPrepare(beat)
		m.queuePrepare = false
	}
	for _, b := range m.bugs {
		b.update(m, beat)
	}
	m.bugs = liveBugs(m.bugs, beat)
}

func (m *Module) Draw(screen *ebiten.Image, _ float64, beat float64) {
	m.ctx.Scene.SetColorOver(m.bg, m.bgAt(beat))
	m.ctx.SampleScene(beat)
	for _, b := range m.bugs {
		if !b.dead && b.inst != nil {
			b.inst.Queue(m.ctx.Scene, beat, kart.Identity(), b.z)
		}
	}
	m.ctx.Scene.Draw(screen, m.proj)
}

func (m *Module) resetScene(beat float64) {
	m.bopExpression = "Neutral"
	m.queuePrepare = false
	m.preparing = false
	m.queueBopReset = false
	m.ctx.Scene.SetActive(roleOr(m.ctx, "Mosquito", "Scene/mosquito"), false)
	m.ctx.Scene.SetActive(roleOr(m.ctx, "Mayfly", "Scene/mayfly"), false)
	sec := m.ctx.SecPerBeat(beat)
	m.ctx.Scene.PlayDefaultState(m.leilani, beat, sec)
	m.ctx.Scene.PlayDefaultState(m.bubble, beat, sec)
	m.ctx.Scene.SetColorOver(m.bg, m.bgAt(beat))
}

func (m *Module) scheduleMosquito(cue bugCue) {
	m.ctx.SoundAt(cue.beat, "mosquito1", 1)
	m.ctx.SoundAt(cue.beat+1, "mosquito2", 1)
	m.ctx.At(cue.beat, func() {
		if m.mosquitoT == nil {
			return
		}
		inst := m.mosquitoT.NewInstance()
		inst.PlayDefaultState("", cue.beat, m.ctx.SecPerBeat(cue.beat))
		b := &bug{cue: cue, inst: inst, state: bugStarting, startBeat: cue.beat, approachBeat: cue.beat + 1}
		b.update(m, cue.beat)
		m.bugs = append(m.bugs, b)
	})
	m.ctx.At(cue.beat+1, func() { m.queuePrepare = true })
	target := cue.beat + 2
	m.ctx.ScheduleInput(target,
		func(state float64, _ engine.Judgment) {
			if b := m.findBug(cue); b != nil {
				m.hitBug(b, state)
			}
		},
		func() {
			if b := m.findBug(cue); b != nil {
				m.missBug(b)
			}
		})
}

func (m *Module) scheduleMayfly(cue bugCue) {
	spawnBeat := cue.beat + 2
	m.ctx.SoundAt(cue.beat, "blink1", 1)
	m.ctx.SoundAt(cue.beat+1, "blink2", 1)
	m.ctx.At(cue.beat, func() { m.ctx.Scene.PlayState(m.bubble, "alert1", cue.beat, 0.5) })
	m.ctx.At(cue.beat+1, func() { m.ctx.Scene.PlayState(m.bubble, "alert2", cue.beat+1, 0.5) })
	m.ctx.At(cue.beat+2, func() { m.ctx.Scene.PlayState(m.bubble, "disable", cue.beat+2, 0.5) })
	m.ctx.SoundAt(spawnBeat, "mayfly1", 1)
	m.ctx.SoundAt(spawnBeat+1, "mayfly2", 1)
	m.ctx.At(spawnBeat, func() {
		if m.mayflyT == nil {
			return
		}
		inst := m.mayflyT.NewInstance()
		inst.PlayDefaultState("", spawnBeat, m.ctx.SecPerBeat(spawnBeat))
		b := &bug{
			cue: cue, inst: inst, state: bugStarting,
			startBeat: spawnBeat, approachBeat: spawnBeat + 1, exitBeat: spawnBeat + 2,
			mirrored: true,
		}
		b.update(m, spawnBeat)
		m.bugs = append(m.bugs, b)
	})
	m.ctx.At(cue.beat+3, func() { m.queuePrepare = true })
	target := cue.beat + 4
	m.ctx.ScheduleInput(target,
		func(state float64, _ engine.Judgment) {
			if b := m.findBug(cue); b != nil {
				m.hitBug(b, state)
			}
		},
		func() {
			if b := m.findBug(cue); b != nil {
				m.missBug(b)
			}
		})
}

func (m *Module) findBug(cue bugCue) *bug {
	for _, b := range m.bugs {
		if !b.dead && b.cue == cue {
			return b
		}
	}
	return nil
}

func (m *Module) hitBug(b *bug, state float64) {
	m.stopPrepare()
	if state >= 1 || state <= -1 {
		m.playBody("SnapMiss", m.ctx.Beat(), 0.5)
		if b.cue.reaction {
			m.bopExpression = "Sad"
		}
		m.ctx.Sound("barely")
		b.state = bugFleeing
		b.fleeBeat = m.ctx.Beat()
		b.mirrored = !b.mirrored
		return
	}
	m.playBody("Snap", m.ctx.Beat(), 0.5)
	if b.cue.reaction {
		m.bopExpression = "Happy"
	}
	m.ctx.Sound("catch")
	b.dead = true
}

func (m *Module) missBug(b *bug) {
	if !m.preparing {
		return
	}
	m.playBody("Unprepare", m.ctx.Beat(), 0.5)
	if b.cue.reaction {
		m.bopExpression = "Sad"
	}
	m.stopPrepare()
}

func (m *Module) doPrepare(beat float64) {
	m.playBody("Prepare", beat, 0.5)
	m.playFace("PrepFace", beat, 0.5)
	m.preparing = true
}

func (m *Module) stopPrepare() {
	m.preparing = false
	m.queuePrepare = false
}

func (m *Module) bop(beat float64) {
	if m.preparing || m.queuePrepare || !m.readyToBop(beat) {
		return
	}
	m.playBody("Bop", beat, 0.5)
	m.playFace(m.bopExpression, beat, 0.5)
	if (m.bopExpression == "Happy" || m.bopExpression == "Sad") && !m.queueBopReset {
		m.queueBopReset = true
		m.ctx.At(beat+1, func() {
			m.bopExpression = "Neutral"
			m.queueBopReset = false
		})
	}
}

func (m *Module) readyToPrepare(beat float64) bool {
	st, playing := m.ctx.Scene.StateInfo(m.leilani, beat)
	return !playing || st == "Idle" || st == "Bop"
}

func (m *Module) readyToBop(beat float64) bool {
	st, playing := m.ctx.Scene.StateInfo(m.leilani, beat)
	return !playing || st == "Idle"
}

func (m *Module) playBody(state string, beat, timeScale float64) {
	m.ctx.Scene.PlayState(m.leilani, state, beat, timeScale)
}

func (m *Module) playFace(state string, beat, timeScale float64) {
	m.ctx.Scene.PlayLayer(faceLayerKey, m.leilani, "Leilani/"+state, beat, timeScale)
}

func (b *bug) update(m *Module, beat float64) {
	var p [3]float64
	switch b.state {
	case bugStarting:
		p = kart.EvalBezier(m.curve(b.cue.kind, "startCurve"), beat-b.startBeat)
		if beat-b.startBeat > 1 {
			if b.cue.kind == bugMayfly {
				b.mirrored = !b.mirrored
				b.inst.SetOrder("body", 50)
				b.inst.SetOrder("wing", 51)
			}
			b.state = bugApproaching
		}
	case bugApproaching:
		u := beat - b.approachBeat
		p = kart.EvalBezier(m.curve(b.cue.kind, "approachCurve"), u)
		if b.cue.kind == bugMosquito {
			if u > 1 {
				b.inst.SetOrder("body", 1000)
				b.inst.SetOrder("wing 1", 1001)
				b.inst.SetOrder("wing 2", 1001)
			}
			if u > 3 {
				b.dead = true
			}
		} else if u > 1 {
			b.inst.SetOrder("body", 1000)
			b.inst.SetOrder("wing", 1001)
			b.state = bugExiting
		}
	case bugFleeing:
		u := beat - b.fleeBeat
		p = kart.EvalBezier(m.curve(b.cue.kind, "fleeCurve"), u)
		if u > 1 {
			b.dead = true
		}
	case bugExiting:
		u := beat - b.exitBeat
		p = kart.EvalBezier(m.curve(b.cue.kind, "exitCurve"), u)
		if u > 1 {
			b.dead = true
		}
	}
	b.inst.Offset = [2]float64{p[0], p[1]}
	b.inst.Scale = [2]float64{1, 1}
	if b.mirrored {
		b.inst.Scale[0] = -1
	}
	b.z = p[2]
}

func (m *Module) curve(kind int, field string) kmdata.Curve {
	prefix := "mosquito"
	if kind == bugMayfly {
		prefix = "mayfly"
	}
	return m.curves[prefix+"."+field]
}

func liveBugs(in []*bug, beat float64) []*bug {
	out := in[:0]
	for _, b := range in {
		if !b.dead && beat-b.startBeat <= 8 {
			out = append(out, b)
		}
	}
	return out
}

func (m *Module) bgAt(beat float64) [4]float64 {
	out := defaultBG
	for _, ev := range m.bgs {
		if beat < ev.beat {
			break
		}
		if ev.length > 0 && beat < ev.beat+ev.length {
			u := (beat - ev.beat) / ev.length
			return lerpColor(ev.from, ev.to, ev.ease, u)
		}
		out = ev.to
	}
	return out
}

func lerpColor(a, b [4]float64, ease int, u float64) [4]float64 {
	return [4]float64{
		engine.Ease(ease, a[0], b[0], u),
		engine.Ease(ease, a[1], b[1], u),
		engine.Ease(ease, a[2], b[2], u),
		engine.Ease(ease, a[3], b[3], u),
	}
}

func boolParam(e *riq.Entity, key string) bool { return e.Float(key, 0) != 0 }

func boolDefault(e *riq.Entity, key string, def bool) bool {
	if _, ok := e.Data[key]; !ok {
		return def
	}
	return boolParam(e, key)
}

func colorParam(e *riq.Entity, key string, def [4]float64) [4]float64 {
	v, ok := e.Data[key]
	if !ok {
		return def
	}
	m, ok := v.(map[string]any)
	if !ok {
		return def
	}
	return [4]float64{
		num(m, "r", def[0]),
		num(m, "g", def[1]),
		num(m, "b", def[2]),
		num(m, "a", def[3]),
	}
}

func num(m map[string]any, k string, def float64) float64 {
	switch v := m[k].(type) {
	case float64:
		return v
	case int:
		return float64(v)
	default:
		return def
	}
}
