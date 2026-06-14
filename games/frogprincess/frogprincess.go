// Package frogprincess ports Frog Princess' two-step hold/jump cue,
// lotus/leaves scrolling, miss/barely branches, splash particle, and BG color.
package frogprincess

import (
	"image/color"
	"math"
	"math/rand"
	"sort"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"hsdemo/engine"
	"hsdemo/kart"
	"hsdemo/riq"
)

var defaultBG = [4]float64{0.482, 0.74, 0.87, 1}

type jumpEvt struct {
	beat float64
}

type bgEvt struct {
	beat, length float64
	c0, c1       [4]float64
	ease         int
}

type moveCo struct {
	path        string
	start, dur  float64
	from, to, y float64
	active      bool
}

type splash struct {
	beat      float64
	particles []splashParticle
}

type splashParticle struct {
	ang, dist, radius float64
}

type Module struct {
	ctx  *engine.Ctx
	proj kart.Aff

	jumps []jumpEvt
	bgs   []bgEvt

	frogPath     string
	princessPath string
	leavesPath   string
	lotusesPath  string
	bgPath       string
	splashPath   string

	lotusHoldPath string
	lotusJumpPath string

	moveDistance float64
	moveTime     float64
	lotusX       float64
	leavesX      float64
	lotusY       float64
	leavesY      float64
	moves        []moveCo
	splashes     []splash

	isPrepare bool
	isHold    bool
	isGone    bool
	isJust    bool
}

func New() engine.Module {
	return &Module{moveDistance: 7.7, moveTime: 0.3}
}

func (m *Module) ID() string { return "frogPrincess" }

func (m *Module) Load(ctx *engine.Ctx) error {
	m.ctx = ctx
	if err := ctx.LoadAssets("frogPrincess"); err != nil {
		return err
	}
	m.proj = kart.Translate(engine.ScreenW/2, engine.ScreenH/2).Mul(kart.Scale(54, -54))
	m.frogPath = roleOr(ctx, "frogAnim", "frog")
	m.princessPath = roleOr(ctx, "princessAnim", "princess")
	m.leavesPath = roleOr(ctx, "Leaves", "Leaves")
	m.lotusesPath = roleOr(ctx, "Lotuses", "Lotuses")
	m.bgPath = roleOr(ctx, "BGPlane", "BG")
	m.splashPath = roleOr(ctx, "splashEffect", "SplashEffect")
	m.lotusHoldPath = "Lotuses/lotus (2)"
	m.lotusJumpPath = "Lotuses/lotus (3)"

	if game := ctx.Assets.Extra.Components["game"]; game.Nums != nil {
		if v, ok := game.Nums["moveDistance"]; ok {
			m.moveDistance = v
		}
		if v, ok := game.Nums["moveTime"]; ok {
			m.moveTime = v
		}
	}
	m.lotusY = nodeY(ctx, m.lotusesPath)
	m.leavesY = nodeY(ctx, m.leavesPath)
	ctx.Scene.SetActive(m.splashPath, false)
	ctx.Scene.PlayDefaultState(m.frogPath, 0, ctx.SecPerBeat(0))
	ctx.Scene.PlayDefaultState(m.princessPath, 0, ctx.SecPerBeat(0))
	for _, p := range []string{"Lotuses/lotus (1)", "Lotuses/lotus (2)", "Lotuses/lotus (3)", "Lotuses/lotus (4)"} {
		ctx.Scene.PlayDefaultState(p, 0, ctx.SecPerBeat(0))
	}
	return nil
}

func roleOr(ctx *engine.Ctx, key, fallback string) string {
	if p := ctx.Role(key); p != "" {
		return p
	}
	return fallback
}

func nodeY(ctx *engine.Ctx, path string) float64 {
	for _, n := range ctx.Assets.Rig.Nodes {
		if n.Path == path {
			return n.Pos[1]
		}
	}
	return 0
}

func (m *Module) OnEvent(e *riq.Entity) {
	switch e.Datamodel {
	case "frogPrincess/jump":
		m.jumps = append(m.jumps, jumpEvt{beat: e.Beat})
	case "frogPrincess/background appearance":
		m.bgs = append(m.bgs, bgEvt{
			beat: e.Beat, length: e.Length, ease: int(e.Float("ease", 1)),
			c0: colorParam(e, "colorBGStart", defaultBG),
			c1: colorParam(e, "colorBGEnd", defaultBG),
		})
	}
}

func (m *Module) Ready() {
	sort.SliceStable(m.jumps, func(i, j int) bool { return m.jumps[i].beat < m.jumps[j].beat })
	sort.SliceStable(m.bgs, func(i, j int) bool { return m.bgs[i].beat < m.bgs[j].beat })
	for _, ev := range m.jumps {
		ev := ev
		m.ctx.At(ev.beat, func() { m.startJump(ev.beat) })
	}
}

func (m *Module) OnSwitch(beat float64) {
	m.ctx.Scene.PlayDefaultState(m.frogPath, beat, m.ctx.SecPerBeat(beat))
	m.ctx.Scene.PlayDefaultState(m.princessPath, beat, m.ctx.SecPerBeat(beat))
}

func (m *Module) Whiff(beat float64) {
	m.ctx.ScoreMiss()
	m.holdFastAnim(beat)
}

func (m *Module) Update(_ float64, beat float64) {
	if m.ctx.ReleasedNow() && !m.ctx.ExpectingReleaseNow() {
		m.ctx.ScoreMiss()
		m.jumpFastAnim(beat)
	}
	for i := range m.moves {
		mv := &m.moves[i]
		if !mv.active {
			continue
		}
		u := 1.0
		if mv.dur > 0 && beat < mv.start+mv.dur {
			u = clamp01((beat - mv.start) / mv.dur)
			u = u * u * (3 - 2*u) // Mathf.SmoothStep
		} else {
			mv.active = false
		}
		x := mv.from + (mv.to-mv.from)*u
		m.ctx.Scene.SetPosOver(mv.path, x, mv.y)
		if mv.path == m.lotusesPath {
			m.lotusX = x
		} else if mv.path == m.leavesPath {
			m.leavesX = x
		}
	}
	dst := m.splashes[:0]
	for _, sp := range m.splashes {
		if beat < sp.beat+0.5 {
			dst = append(dst, sp)
		}
	}
	m.splashes = dst
}

func (m *Module) Draw(screen *ebiten.Image, _ float64, beat float64) {
	bg := m.bgAt(beat)
	screen.Fill(rgba(bg))
	m.ctx.Scene.SetColorOver(m.bgPath, bg)
	m.ctx.SampleScene(beat)
	m.ctx.Scene.Draw(screen, m.proj)
	m.drawSplashes(screen, beat)
}

func (m *Module) startJump(beat float64) {
	if m.isGone {
		return
	}
	m.isPrepare = true
	m.ctx.At(beat, func() { m.readyAnim(beat) })
	m.ctx.At(beat+1, func() { m.readyAnim(beat + 1) })
	holdBeat := beat + 2
	m.ctx.ScheduleInput(holdBeat,
		func(state float64, _ engine.Judgment) { m.justHold(holdBeat, state) },
		func() { m.holdMissAnim(holdBeat) })
}

func (m *Module) justHold(target float64, state float64) {
	m.isJust = false
	jumpBeat := target + 1
	m.ctx.ScheduleInputRelease(jumpBeat,
		func(state float64, _ engine.Judgment) { m.justJump(jumpBeat, state) },
		func() { m.jumpMissAnim(jumpBeat) })
	if state >= 1 || state <= -1 {
		m.holdBarelyAnim(target)
		return
	}
	m.holdAnim(target)
}

func (m *Module) justJump(target float64, state float64) {
	m.isJust = true
	if state >= 1 || state <= -1 {
		m.jumpBarelyAnim(target)
		return
	}
	m.jumpAnim(target)
}

func (m *Module) readyAnim(beat float64) {
	if m.isGone {
		return
	}
	m.ctx.Sound("ready")
	if st, _ := m.ctx.Scene.StateInfo(m.frogPath, beat); st != "jump" {
		m.play(m.frogPath, "ready", beat)
		m.play(m.princessPath, "ready", beat)
		m.play(m.princessPath, "wary", beat)
	}
}

func (m *Module) holdAnim(beat float64) {
	m.isHold = true
	m.updatePos()
	m.play(m.lotusHoldPath, "hold", beat)
	m.play(m.frogPath, "hold", beat)
	m.ctx.Sound("lean")
	m.play(m.princessPath, "hold", beat)
}

func (m *Module) holdBarelyAnim(beat float64) {
	m.isHold = true
	m.updatePos()
	m.play(m.lotusHoldPath, "hold", beat)
	m.play(m.frogPath, "hold", beat)
	m.ctx.Sound("lean")
	m.ctx.Sound("7")
	m.play(m.princessPath, "holdBarely", beat)
	m.play(m.princessPath, "surpriseHoldBarely", beat)
}

func (m *Module) holdMissAnim(beat float64) {
	if !m.isPrepare {
		return
	}
	m.isGone = true
	m.updatePos()
	m.ctx.Sound("A")
	m.play(m.lotusHoldPath, "fall", beat)
	m.play(m.frogPath, "fall", beat)
	m.play(m.princessPath, "fallBackward", beat)
	m.appear(beat+0.5, false)
}

func (m *Module) holdFastAnim(beat float64) {
	if m.isHold || m.isGone {
		return
	}
	m.isGone = true
	m.isPrepare = false
	m.updatePos()
	m.play(m.lotusHoldPath, "hold", beat)
	m.play(m.frogPath, "hold", beat)
	m.ctx.Sound("lean")
	m.ctx.Sound("A")
	m.play(m.princessPath, "fallForward", beat)
	m.play(m.princessPath, "surpriseFall", beat)
	m.ctx.At(beat+0.75, func() {
		m.play(m.lotusHoldPath, "release", beat+0.75)
		m.play(m.frogPath, "release", beat+0.75)
	})
	m.appear(beat, false)
}

func (m *Module) jumpAnim(beat float64) {
	m.isHold = false
	m.updatePos()
	m.play(m.lotusHoldPath, "release", beat)
	m.moveScrollers(beat)
	m.ctx.Sound("jump")
	m.play(m.lotusJumpPath, "jump", beat)
	m.play(m.frogPath, "jump", beat)
	m.play(m.princessPath, "jump", beat)
	m.play(m.princessPath, "happy", beat)
}

func (m *Module) jumpBarelyAnim(beat float64) {
	m.isHold = false
	m.updatePos()
	m.play(m.lotusHoldPath, "release", beat)
	m.moveScrollers(beat)
	m.ctx.Sound("jump")
	m.ctx.SoundAt(beat+0.5, "7", 1)
	m.play(m.lotusJumpPath, "jumpBarely", beat)
	m.play(m.frogPath, "jumpBarely", beat)
	m.play(m.princessPath, "jumpBarely", beat)
	m.play(m.princessPath, "surpriseJumpBarely", beat)
}

func (m *Module) jumpMissAnim(beat float64) {
	if !m.isHold || m.isGone {
		return
	}
	m.isHold = false
	m.isGone = true
	m.updatePos()
	m.ctx.Sound("A")
	m.play(m.lotusHoldPath, "fall", beat)
	m.play(m.frogPath, "fall", beat)
	m.play(m.princessPath, "fallForward", beat)
	m.appear(beat, false)
}

func (m *Module) jumpFastAnim(beat float64) {
	if !m.isHold || m.isGone {
		return
	}
	m.isHold = false
	m.isGone = true
	m.updatePos()
	m.ctx.Sound("jump")
	m.play(m.lotusHoldPath, "release", beat)
	m.moveScrollers(beat)
	m.play(m.frogPath, "jumpFast", beat)
	m.play(m.princessPath, "jumpFast", beat)
	m.ctx.At(beat+0.5, func() {
		m.ctx.Sound("A")
		m.splashes = append(m.splashes, newSplash(beat+0.5, int64(beat*1000)+17))
	})
	m.appear(beat, true)
}

func (m *Module) updatePos() {
	m.lotusX = 0
	m.leavesX = wrapLeavesX(m.leavesX)
	m.ctx.Scene.SetPosOver(m.lotusesPath, m.lotusX, m.lotusY)
	m.ctx.Scene.SetPosOver(m.leavesPath, m.leavesX, m.leavesY)
}

func wrapLeavesX(x float64) float64 { return math.Mod(x-3, 10.7) + 3 }

func (m *Module) moveScrollers(beat float64) {
	m.moves = append(m.moves,
		moveCo{path: m.lotusesPath, start: beat, dur: m.moveTime, from: m.lotusX, to: m.lotusX + m.moveDistance, y: m.lotusY, active: true},
		moveCo{path: m.leavesPath, start: beat, dur: m.moveTime, from: m.leavesX, to: m.leavesX + m.moveDistance, y: m.leavesY, active: true},
	)
}

func (m *Module) appear(beat float64, frog bool) {
	m.ctx.At(beat+0.9, func() {
		m.isGone = false
		m.play(m.princessPath, "idle", beat+0.9)
		m.play(m.princessPath, "appear", beat+0.9)
		if frog {
			m.play(m.frogPath, "idle", beat+0.9)
			m.play(m.frogPath, "appear", beat+0.9)
		}
	})
}

func (m *Module) play(path, state string, beat float64) {
	m.ctx.Scene.PlayState(path, state, beat, 0.5)
}

func (m *Module) bgAt(beat float64) [4]float64 {
	out := defaultBG
	for _, ev := range m.bgs {
		if ev.beat > beat {
			break
		}
		u := 1.0
		if ev.length > 0 && beat < ev.beat+ev.length {
			u = clamp01((beat - ev.beat) / ev.length)
		}
		out = easeColor(ev.ease, ev.c0, ev.c1, u)
	}
	return out
}

func (m *Module) drawSplashes(screen *ebiten.Image, beat float64) {
	for _, sp := range m.splashes {
		u := clamp01((beat - sp.beat) / 0.5)
		if u <= 0 || u >= 1 {
			continue
		}
		c := color.NRGBA{R: 255, G: 255, B: 255, A: byte((1 - u) * 220)}
		for _, p := range sp.particles {
			dist := p.dist * u
			wx := 1.6 + math.Cos(p.ang)*dist
			wy := -4 + math.Sin(p.ang)*dist*0.55 + u*0.25
			sx, sy := m.proj.Apply(wx, wy)
			r := float32(p.radius * 54 * (1 - 0.25*u))
			vector.DrawFilledCircle(screen, float32(sx), float32(sy), r, c, true)
		}
	}
}

func newSplash(beat float64, seed int64) splash {
	r := rand.New(rand.NewSource(seed))
	sp := splash{beat: beat}
	for i := 0; i < 18; i++ {
		ang := -math.Pi*0.1 - float64(i)/17*math.Pi*0.8
		if i%2 == 1 {
			ang += 0.08
		}
		sp.particles = append(sp.particles, splashParticle{
			ang:    ang,
			dist:   0.25 + 0.9*r.Float64(),
			radius: 0.045 + 0.035*r.Float64(),
		})
	}
	return sp
}

func easeColor(ease int, a, b [4]float64, u float64) [4]float64 {
	return [4]float64{
		engine.Ease(ease, a[0], b[0], u),
		engine.Ease(ease, a[1], b[1], u),
		engine.Ease(ease, a[2], b[2], u),
		engine.Ease(ease, a[3], b[3], u),
	}
}

func rgba(c [4]float64) color.NRGBA {
	return color.NRGBA{R: byte(clamp01(c[0]) * 255), G: byte(clamp01(c[1]) * 255), B: byte(clamp01(c[2]) * 255), A: byte(clamp01(c[3]) * 255)}
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
