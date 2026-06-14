// Package basketballgirls ports Basketball Girls' ball catch/dunk routine,
// bops, local camera zoom, and background color easing.
package basketballgirls

import (
	"fmt"
	"image/color"
	"math"
	"math/rand"
	"sort"

	"github.com/hajimehoshi/ebiten/v2"

	"hsdemo/engine"
	"hsdemo/kart"
	"hsdemo/riq"
)

var defaultBG = [4]float64{1, 0.937, 0.224, 1}

type bopEvt struct {
	beat, length float64
	toggle, auto bool
}

type ballEvt struct {
	beat float64
}

type zoomEvt struct {
	beat, length float64
	ease         int
	in           bool
}

type bgEvt struct {
	beat, length float64
	c0, c1       [4]float64
	ease         int
}

type noBopInterval struct {
	start, end float64
}

func (iv noBopInterval) contains(beat float64) bool {
	return beat >= iv.start && beat < iv.end
}

type ball struct {
	inst *kart.Instance
	beat float64
	dead bool
}

type Module struct {
	ctx  *engine.Ctx
	proj kart.Aff

	bops  []bopEvt
	balls []ballEvt
	zooms []zoomEvt
	bgs   []bgEvt

	activeBalls []*ball
	ballT       *kart.Template

	girlLeft  string
	girlRight string
	goal      string
	bgPlane   string

	leftNoBop  []noBopInterval
	rightNoBop []noBopInterval

	camFar  [3]float64
	camNear [3]float64
}

func New() engine.Module {
	return &Module{
		camFar: [3]float64{0, 0, -10}, camNear: [3]float64{0, 3.16, -2.5},
	}
}

func (m *Module) ID() string { return "basketballGirls" }

func (m *Module) Load(ctx *engine.Ctx) error {
	m.ctx = ctx
	if err := ctx.LoadAssets("basketballGirls"); err != nil {
		return err
	}
	m.proj = kart.Translate(engine.ScreenW/2, engine.ScreenH/2).Mul(kart.Scale(54, -54))
	m.girlLeft = roleOr(ctx, "girlLeftAnim", "GirlLeft")
	m.girlRight = roleOr(ctx, "girlRightAnim", "GirlRight")
	m.goal = roleOr(ctx, "goalAnim", "goal")
	m.bgPlane = roleOr(ctx, "BGPlane", "BG")
	m.ballT = kart.NewTemplate(ctx.Assets, roleOr(ctx, "baseBall", "ball"))
	m.loadCameraRefs()
	ctx.Scene.SetActive(roleOr(ctx, "baseBall", "ball"), false)
	ctx.Scene.PlayState(m.girlLeft, "idle", 0, ctx.SecPerBeat(0))
	ctx.Scene.PlayState(m.girlRight, "idle", 0, ctx.SecPerBeat(0))
	ctx.Scene.PlayState(m.goal, "idle", 0, ctx.SecPerBeat(0))
	return nil
}

func roleOr(ctx *engine.Ctx, key, fallback string) string {
	if p := ctx.Role(key); p != "" {
		return p
	}
	return fallback
}

func (m *Module) loadCameraRefs() {
	refs := m.ctx.Assets.Extra.RefArrays["CameraPosition"]
	if len(refs) >= 2 {
		if p, ok := m.nodePos3(refs[0]); ok {
			m.camFar = p
		}
		if p, ok := m.nodePos3(refs[1]); ok {
			m.camNear = p
		}
	}
}

func (m *Module) nodePos3(path string) ([3]float64, bool) {
	for _, n := range m.ctx.Assets.Rig.Nodes {
		if n.Path == path {
			return [3]float64{n.Pos[0], n.Pos[1], n.PosZ}, true
		}
	}
	return [3]float64{}, false
}

func (m *Module) OnEvent(e *riq.Entity) {
	switch e.Datamodel {
	case "basketballGirls/bop":
		m.bops = append(m.bops, bopEvt{
			beat: e.Beat, length: e.Length,
			toggle: boolDefault(e, "toggle", true),
			auto:   boolParam(e, "auto"),
		})
	case "basketballGirls/ball":
		m.balls = append(m.balls, ballEvt{beat: e.Beat})
	case "basketballGirls/zoom":
		m.zooms = append(m.zooms, zoomEvt{
			beat: e.Beat, length: e.Length,
			ease: int(e.Float("ease", 0)),
			in:   boolDefault(e, "toggle", true),
		})
	case "basketballGirls/background appearance":
		m.bgs = append(m.bgs, bgEvt{
			beat: e.Beat, length: e.Length,
			c0:   colorParam(e, "colorBGStart", defaultBG),
			c1:   colorParam(e, "colorBGEnd", defaultBG),
			ease: int(e.Float("ease", 0)),
		})
	}
}

func (m *Module) Ready() {
	sort.Slice(m.balls, func(i, j int) bool { return m.balls[i].beat < m.balls[j].beat })
	sort.Slice(m.zooms, func(i, j int) bool { return m.zooms[i].beat < m.zooms[j].beat })
	sort.Slice(m.bgs, func(i, j int) bool { return m.bgs[i].beat < m.bgs[j].beat })
	for _, be := range m.balls {
		m.leftNoBop = append(m.leftNoBop, noBopInterval{be.beat, be.beat + 2})
		be := be
		m.ctx.At(be.beat, func() { m.spawnBall(be.beat) })
		m.ctx.ScheduleInputAction(be.beat+1, 0,
			func(state float64, _ engine.Judgment) { m.catchBall(be.beat, state) },
			func() { m.missBall(be.beat) })
	}
	for _, bp := range m.bops {
		if !bp.toggle && !bp.auto {
			continue
		}
		for i := 0.0; i < bp.length; i++ {
			beat := bp.beat + i
			m.ctx.At(beat, func() { m.bop(beat) })
		}
	}
}

func (m *Module) OnSwitch(beat float64) {}

func (m *Module) Whiff(beat float64) {
	if m.inNoBop(m.rightNoBop, beat) {
		return
	}
	if st, playing := m.ctx.Scene.StateInfo(m.girlRight, beat); st == "blank" && playing {
		return
	}
	m.ctx.Sound("A")
	m.ctx.Scene.PlayState(m.girlRight, "blank", beat, m.ctx.SecPerBeat(beat))
}

func (m *Module) Update(t, beat float64) {
	dst := m.activeBalls[:0]
	for _, b := range m.activeBalls {
		if !b.dead && beat <= b.beat+3 {
			dst = append(dst, b)
		}
	}
	m.activeBalls = dst
}

func (m *Module) Draw(screen *ebiten.Image, t, beat float64) {
	bg := m.bgAt(beat)
	screen.Fill(toRGBA(bg))
	sc := m.ctx.Scene
	sc.SetColorOver(m.bgPlane, bg)
	base := m.ctx.CameraAt(beat)
	local := m.localCameraAt(beat)
	sc.SetCamera(base[0]+local[0], base[1]+local[1], base[2]+(local[2]+10))
	sc.Sample(beat)
	for _, b := range m.activeBalls {
		b.inst.Queue(sc, beat, kart.Identity(), 0)
	}
	sc.Draw(screen, m.proj)
}

func (m *Module) spawnBall(beat float64) {
	if m.ballT == nil {
		return
	}
	m.ctx.Sound("voice")
	m.ctx.Scene.PlayState(m.girlLeft, "prepare", beat, m.ctx.SecPerBeat(beat))
	b := &ball{inst: m.ballT.NewInstance(), beat: beat}
	b.inst.PlayState("", "prepare", beat, m.ctx.SecPerBeat(beat))
	m.activeBalls = append(m.activeBalls, b)
}

func (m *Module) catchBall(startBeat, state float64) {
	b := m.findBall(startBeat)
	m.rightNoBop = append(m.rightNoBop, noBopInterval{startBeat + 1, startBeat + 2})
	m.ctx.Sound("catch")
	m.ctx.Scene.PlayState(m.girlRight, "catch", m.ctx.Beat(), m.ctx.SecPerBeat(m.ctx.Beat()))
	if b != nil {
		b.inst.PlayState("", "catch", m.ctx.Beat(), m.ctx.SecPerBeat(m.ctx.Beat()))
	}
	if state <= -1 || state >= 1 {
		m.ctx.SoundAt(startBeat+1.5, "throw", 1)
		m.ctx.SoundAt(startBeat+2, "6", 1)
		m.ctx.Scene.PlayState(m.girlLeft, "pass", m.ctx.Beat(), m.ctx.SecPerBeat(m.ctx.Beat()))
		m.ctx.At(startBeat+1.5, func() {
			if b := m.findBall(startBeat); b != nil {
				b.inst.PlayState("", "shootBarely", startBeat+1.5, m.ctx.SecPerBeat(startBeat+1.5))
			}
			m.ctx.Scene.PlayState(m.girlRight, "shoot", startBeat+1.5, m.ctx.SecPerBeat(startBeat+1.5))
		})
		m.ctx.At(startBeat+2, func() {
			if b := m.findBall(startBeat); b != nil {
				b.inst.PlayState("", "barely", startBeat+2, m.ctx.SecPerBeat(startBeat+2))
			}
			m.ctx.Scene.PlayState(m.goal, "barely", startBeat+2, m.ctx.SecPerBeat(startBeat+2))
		})
		m.ctx.At(startBeat+3, func() { m.destroyBall(startBeat) })
		return
	}
	m.ctx.SoundAt(startBeat+1.5, "throw", 1)
	m.ctx.SoundAt(startBeat+2, "dunk", 1)
	m.ctx.SoundAt(startBeat+2.5, "ok1", 1)
	m.ctx.SoundAt(startBeat+3, "ok2", 1)
	if !m.hasLeftNoBopStart(startBeat + 1) {
		m.ctx.Scene.PlayState(m.girlLeft, "pass", m.ctx.Beat(), m.ctx.SecPerBeat(m.ctx.Beat()))
	}
	m.ctx.At(startBeat+1.5, func() {
		if b := m.findBall(startBeat); b != nil {
			b.inst.PlayState("", "shootJust", startBeat+1.5, m.ctx.SecPerBeat(startBeat+1.5))
		}
		m.ctx.Scene.PlayState(m.girlRight, "shoot", startBeat+1.5, m.ctx.SecPerBeat(startBeat+1.5))
	})
	m.ctx.At(startBeat+2, func() {
		if b := m.findBall(startBeat); b != nil {
			b.inst.PlayState("", "just", startBeat+2, m.ctx.SecPerBeat(startBeat+2))
		}
		m.ctx.Scene.PlayState(m.goal, "just", startBeat+2, m.ctx.SecPerBeat(startBeat+2))
	})
	m.ctx.At(startBeat+3, func() { m.destroyBall(startBeat) })
}

func (m *Module) missBall(startBeat float64) {
	m.ctx.Sound("1")
	if b := m.findBall(startBeat); b != nil {
		b.inst.PlayState("", "hit", m.ctx.Beat(), m.ctx.SecPerBeat(m.ctx.Beat()))
	}
	m.ctx.Scene.PlayState(m.girlLeft, "shock", m.ctx.Beat(), m.ctx.SecPerBeat(m.ctx.Beat()))
	m.ctx.Scene.PlayState(m.girlRight, "hit", m.ctx.Beat(), m.ctx.SecPerBeat(m.ctx.Beat()))
	m.rightNoBop = append(m.rightNoBop, noBopInterval{startBeat + 1, startBeat + 2})
	m.ctx.At(startBeat+3, func() { m.destroyBall(startBeat) })
}

func (m *Module) findBall(startBeat float64) *ball {
	for _, b := range m.activeBalls {
		if math.Abs(b.beat-startBeat) < 1e-6 && !b.dead {
			return b
		}
	}
	return nil
}

func (m *Module) destroyBall(startBeat float64) {
	if b := m.findBall(startBeat); b != nil {
		b.dead = true
	}
}

func (m *Module) bop(beat float64) {
	if !m.inNoBop(m.leftNoBop, beat) {
		m.ctx.Sound(fmt.Sprintf("dribble%d", rand.Intn(2)+1))
		m.ctx.SoundAt(beat+0.5, fmt.Sprintf("dribbleEcho%d", rand.Intn(3)+1), 1)
		m.ctx.Scene.PlayState(m.girlLeft, "dribble", beat, m.ctx.SecPerBeat(beat))
	}
	if !m.inNoBop(m.rightNoBop, beat) {
		m.ctx.Scene.PlayState(m.girlRight, "bop", beat, m.ctx.SecPerBeat(beat))
	}
}

func (m *Module) inNoBop(list []noBopInterval, beat float64) bool {
	for _, iv := range list {
		if iv.contains(beat) {
			return true
		}
	}
	return false
}

func (m *Module) hasLeftNoBopStart(beat float64) bool {
	for _, iv := range m.leftNoBop {
		if math.Abs(iv.start-beat) < 1e-6 {
			return true
		}
	}
	return false
}

func (m *Module) localCameraAt(beat float64) [3]float64 {
	cam := m.camFar
	for _, z := range m.zooms {
		if beat < z.beat {
			break
		}
		from, to := m.camFar, m.camNear
		if !z.in {
			from, to = m.camNear, m.camFar
		}
		if z.length > 0 && beat < z.beat+z.length {
			u := (beat - z.beat) / z.length
			for i := 0; i < 3; i++ {
				cam[i] = engine.Ease(z.ease, from[i], to[i], u)
			}
			return cam
		}
		cam = to
	}
	return cam
}

func (m *Module) bgAt(beat float64) [4]float64 {
	ev := bgEvt{c0: defaultBG, c1: defaultBG}
	for _, e := range m.bgs {
		if e.beat > beat {
			break
		}
		ev = e
	}
	u := 1.0
	if ev.length > 0 && beat < ev.beat+ev.length {
		u = (beat - ev.beat) / ev.length
	}
	var c [4]float64
	for i := 0; i < 4; i++ {
		c[i] = engine.Ease(ev.ease, ev.c0[i], ev.c1[i], u)
	}
	return c
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
		num(m["r"], def[0]), num(m["g"], def[1]), num(m["b"], def[2]), num(m["a"], def[3]),
	}
}

func num(v any, def float64) float64 {
	if f, ok := v.(float64); ok {
		return f
	}
	return def
}

func toRGBA(c [4]float64) color.NRGBA {
	return color.NRGBA{
		R: uint8(clamp01(c[0]) * 255),
		G: uint8(clamp01(c[1]) * 255),
		B: uint8(clamp01(c[2]) * 255),
		A: uint8(clamp01(c[3]) * 255),
	}
}

func clamp01(v float64) float64 {
	return math.Max(0, math.Min(1, v))
}
