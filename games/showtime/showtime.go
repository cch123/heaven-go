// Package showtime ports Showtime's penguin cue timing, monkey button swing,
// launcher ball arcs, and mapped-material background color changes.
package showtime

import (
	"image/color"
	"math"
	"sort"

	"github.com/hajimehoshi/ebiten/v2"

	"hsdemo/engine"
	"hsdemo/kart"
	"hsdemo/kmdata"
	"hsdemo/riq"
)

const (
	penguinGray = iota
	penguinWhite
	penguinBig
)

const (
	easeLinear   = 0
	easeInQuad   = 2
	easeInCubic  = 5
	easeOutCubic = 6
)

var (
	defaultA = [4]float64{0.2666, 1, 1, 1}
	defaultB = [4]float64{0, 0.8588235294117647, 0.8941176470588236, 1}
	defaultC = [4]float64{0, 0.6745098039215687, 0.666, 1}
)

type bopEvt struct {
	beat, length float64
	bop, auto    bool
}

type penguinEvt struct {
	beat float64
	typ  int
}

type bgEvt struct {
	beat, length   float64
	a0, a1, b0, b1 [4]float64
	c0, c1         [4]float64
	ease           int
}

type motion struct {
	curve      string
	start, dur float64
	ease       int
	staticX    float64
	staticY    float64
	hasStatic  bool
}

type activePenguin struct {
	inst         *kart.Instance
	typ          int
	animRel      string
	clipPrefix   string
	startBeat    float64
	checkBeat    float64
	slideBeat    float64
	successCheck bool
	dead         bool
	motion       motion
}

type activeBall struct {
	inst   *kart.Instance
	motion motion
	dead   bool
}

type Module struct {
	ctx  *engine.Ctx
	proj kart.Aff

	monkey, button, launcher string
	blockOne, blockTwo       string

	penguinT [3]*kart.Template
	ballT    *kart.Template
	curves   map[string]kmdata.Curve

	bops     []bopEvt
	penguins []penguinEvt
	bgs      []bgEvt

	activePenguins []*activePenguin
	balls          []*activeBall

	lastPulse      float64
	lastBopBeat    float64
	monkeyBusyBeat float64
}

func New() engine.Module {
	return &Module{
		lastPulse:   math.Inf(-1),
		lastBopBeat: math.Inf(-1),
	}
}

func (m *Module) ID() string { return "showtime" }

func (m *Module) Load(ctx *engine.Ctx) error {
	m.ctx = ctx
	if err := ctx.LoadAssets("showtime"); err != nil {
		return err
	}
	m.proj = kart.Translate(engine.ScreenW/2, engine.ScreenH/2).Mul(kart.Scale(54, -54))
	m.monkey = roleOr(ctx, "MonkeyAnim", "Gameplay/Player Stuff/Monkey(new)")
	m.button = roleOr(ctx, "ButtonAnim", "Gameplay/Player Stuff/Mechanism/ButtonBase")
	m.launcher = roleOr(ctx, "LauncherAnim", "Gameplay/Player Stuff/Mechanism/LauncherNew")
	m.blockOne = roleOr(ctx, "blockOneAnim", "Gameplay/Penguin Side/Platforms (penguin side)/GameObject")
	m.blockTwo = roleOr(ctx, "blockTwoAnim", "Gameplay/Penguin Side/Platforms (penguin side)/GameObject (1)")
	m.curves = ctx.Assets.Extra.Curves
	m.penguinT[penguinGray] = kart.NewTemplate(ctx.Assets, "penguinGray")
	m.penguinT[penguinWhite] = kart.NewTemplate(ctx.Assets, "penguinWhite")
	m.penguinT[penguinBig] = kart.NewTemplate(ctx.Assets, "penguinBig")
	m.ballT = kart.NewTemplate(ctx.Assets, "showtimeBall")
	m.applyBackground(-math.MaxFloat64)
	return nil
}

func (m *Module) OnEvent(e *riq.Entity) {
	switch e.Datamodel {
	case "showtime/bop":
		m.bops = append(m.bops, bopEvt{
			beat: e.Beat, length: e.Length,
			bop:  boolParamDefault(e, "toggle", true),
			auto: boolParam(e, "autobop"),
		})
	case "showtime/penguinGray":
		m.penguins = append(m.penguins, penguinEvt{beat: e.Beat, typ: penguinGray})
	case "showtime/penguinWhite":
		m.penguins = append(m.penguins, penguinEvt{beat: e.Beat, typ: penguinWhite})
	case "showtime/penguinBig":
		m.penguins = append(m.penguins, penguinEvt{beat: e.Beat, typ: penguinBig})
	case "showtime/background":
		m.bgs = append(m.bgs, bgEvt{
			beat: e.Beat, length: e.Length,
			a0: colorParam(e, "startA", defaultA), a1: colorParam(e, "endA", defaultA),
			b0: colorParam(e, "startB", defaultB), b1: colorParam(e, "endB", defaultB),
			c0: colorParam(e, "startC", defaultC), c1: colorParam(e, "endC", defaultC),
			ease: int(e.Float("ease", 0)),
		})
	}
}

func (m *Module) Ready() {
	sort.Slice(m.bops, func(i, j int) bool { return m.bops[i].beat < m.bops[j].beat })
	sort.Slice(m.penguins, func(i, j int) bool { return m.penguins[i].beat < m.penguins[j].beat })
	sort.Slice(m.bgs, func(i, j int) bool { return m.bgs[i].beat < m.bgs[j].beat })
	for _, ev := range m.bops {
		if !ev.bop {
			continue
		}
		for b := ev.beat; b < ev.beat+ev.length-1e-6; b++ {
			bb := b
			m.ctx.At(bb, func() { m.bop(bb) })
		}
	}
	for _, ev := range m.penguins {
		ev := ev
		m.schedulePenguin(ev.beat, ev.typ)
	}
}

func (m *Module) OnSwitch(beat float64) {
	sec := m.ctx.SecPerBeat(beat)
	for _, p := range []string{
		"Background/BG Stripes", "WaterHolder", "WaterHolder/Water/showtime_27",
		m.button, m.launcher, m.blockOne, m.blockTwo,
	} {
		m.ctx.Scene.PlayDefaultState(p, beat, sec)
	}
	m.ctx.Scene.Play(m.monkey, "MonkeyNew/Idle", beat, sec)
	m.lastPulse = math.Floor(beat)
	m.lastBopBeat = math.Inf(-1)
	m.applyBackground(beat)
}

func (m *Module) Whiff(beat float64) {
	if beat < m.monkeyBusyBeat {
		return
	}
	m.hitMonkey(beat, false)
}

func (m *Module) Update(_, beat float64) {
	if p := math.Floor(beat); p > m.lastPulse {
		m.lastPulse = p
		if m.autoBopAt(p) {
			m.bop(p)
		}
	}
	m.applyBackground(beat)
	m.updateObjects(beat)
}

func (m *Module) Draw(screen *ebiten.Image, _, beat float64) {
	screen.Fill(color.RGBA{0x00, 0xdc, 0xe4, 0xff})
	m.ctx.SampleScene(beat)
	for _, p := range m.activePenguins {
		if !p.dead {
			p.inst.Queue(m.ctx.Scene, beat, kart.Identity(), 0)
		}
	}
	for _, b := range m.balls {
		if !b.dead {
			b.inst.Queue(m.ctx.Scene, beat, kart.Identity(), 0)
		}
	}
	m.ctx.Scene.Draw(screen, m.proj)
}

func (m *Module) bop(beat float64) {
	if math.Abs(beat-m.lastBopBeat) < 1e-6 || beat < m.monkeyBusyBeat {
		return
	}
	m.lastBopBeat = beat
	m.ctx.Scene.Play(m.monkey, "MonkeyNew/Bop", beat, m.ctx.SecPerBeat(beat)*0.5)
	m.monkeyBusyBeat = beat + 0.5
}

func (m *Module) schedulePenguin(beat float64, typ int) {
	for _, c := range cueSounds(typ) {
		c := c
		m.ctx.SoundAtPitchOff(beat+c.off, "moving", 1, c.pitch, 0)
	}
	start := penguinStartBeat(beat, typ)
	m.ctx.At(start, func() { m.spawnPenguin(start, typ) })
	target := penguinTargetBeat(beat, typ)
	m.ctx.ScheduleInputAction(target, 0,
		func(state float64, _ engine.Judgment) {
			m.penguinHit(target, typ, math.Abs(state) < 1)
		},
		func() { m.penguinMiss(target, typ) },
	)
}

func (m *Module) spawnPenguin(startBeat float64, typ int) {
	t := m.penguinT[typ]
	if t == nil {
		return
	}
	p := &activePenguin{
		inst:       t.NewInstance(),
		typ:        typ,
		animRel:    penguinAnimRel(typ),
		clipPrefix: penguinClipPrefix(typ),
		startBeat:  startBeat,
		checkBeat:  penguinCheckBeat(startBeat, typ),
		slideBeat:  penguinSlideBeat(startBeat, typ),
	}
	p.play(m.ctx, "Idle", startBeat, 0.5)
	p.setMotion("entryCurve", startBeat, 1, easeInQuad)
	m.activePenguins = append(m.activePenguins, p)
	m.schedulePenguinActions(p)
}

func (m *Module) schedulePenguinActions(p *activePenguin) {
	start := p.startBeat
	switch p.typ {
	case penguinWhite:
		m.ctx.At(start+1, func() {
			p.play(m.ctx, "Land", start+1, 0.5)
			p.setMotion("hopCurve", start+1, 0.75, easeLinear)
			m.ctx.Scene.PlayState(m.blockOne, "Squish", start+1, m.ctx.SecPerBeat(start+1)*0.5)
		})
		m.ctx.At(start+2, func() {
			p.clearMotion()
			p.setStatic(m.point("leapStart"))
			p.play(m.ctx, "Prepare", start+2, 1)
		})
	case penguinBig:
		m.ctx.At(start+1, func() {
			p.setMotion("hopCurve", start+1, 2, easeLinear)
			m.ctx.Scene.PlayState(m.blockOne, "Squish", start+1, m.ctx.SecPerBeat(start+1)*0.5)
		})
		m.ctx.At(start+3, func() {
			p.play(m.ctx, "Land", start+3, 0.5)
			p.clearMotion()
			p.setStatic(m.point("leapStart"))
		})
		m.ctx.At(start+4, func() { p.play(m.ctx, "Prepare", start+4, 1) })
	default:
		m.ctx.At(start+1, func() {
			p.setMotion("hopCurve", start+1, 0.75, easeLinear)
			m.ctx.Scene.PlayState(m.blockOne, "Squish", start+1, m.ctx.SecPerBeat(start+1)*0.5)
		})
		m.ctx.At(start+2, func() {
			p.play(m.ctx, "Land", start+2, 0.5)
			p.clearMotion()
			p.setStatic(m.point("leapStart"))
		})
		m.ctx.At(start+2.5, func() { p.play(m.ctx, "Prepare", start+2.5, 1) })
	}
	leap := penguinTargetFromStart(start, p.typ)
	m.ctx.At(leap, func() {
		p.setMotion("leapCurve", leap, 1, easeLinear)
		p.play(m.ctx, "Leap", leap, 1)
	})
	m.ctx.At(p.checkBeat, func() {
		p.setStatic(m.point("fallStart"))
		p.setMotion("fallCurve", p.checkBeat, 0.5, easeLinear)
		if p.successCheck {
			m.ctx.Sound(p.catchSound())
			p.play(m.ctx, "Catch", p.checkBeat, 0.5)
		}
	})
	m.ctx.At(p.slideBeat, func() {
		p.setStatic(m.point("slideStart"))
		p.setMotion("exitCurve", p.slideBeat, 1, easeLinear)
		if p.successCheck {
			p.play(m.ctx, "SlideBall", p.slideBeat, 0.5)
		} else {
			p.play(m.ctx, "Slide", p.slideBeat, 0.5)
		}
	})
	m.ctx.At(p.slideBeat+1.1, func() { p.dead = true })
}

func (m *Module) penguinHit(beat float64, typ int, success bool) {
	p := m.latestPenguin(typ)
	if p != nil {
		p.successCheck = success
	}
	m.hitMonkey(beat, success)
}

func (m *Module) penguinMiss(_ float64, typ int) {
	if p := m.latestPenguin(typ); p != nil {
		p.successCheck = false
	}
}

func (m *Module) hitMonkey(beat float64, success bool) {
	clip := "MonkeyNew/Hit"
	if !success {
		clip = "MonkeyNew/Miss"
	}
	m.ctx.Scene.Play(m.monkey, clip, beat, m.ctx.SecPerBeat(beat)*0.5)
	m.monkeyBusyBeat = beat + 0.5
	m.ctx.Sound("hit")
	m.spawnBall(beat, !success)
}

func (m *Module) spawnBall(beat float64, miss bool) {
	if m.ballT == nil {
		return
	}
	m.ctx.Scene.PlayState(m.button, "Press", beat, m.ctx.SecPerBeat(beat)*0.5)
	m.ctx.Scene.PlayState(m.launcher, "Launch", beat, m.ctx.SecPerBeat(beat)*0.5)
	b := &activeBall{inst: m.ballT.NewInstance()}
	b.setMotion("ballUpCurve", beat, 1, easeOutCubic)
	m.balls = append(m.balls, b)
	m.ctx.At(beat+1, func() {
		if miss {
			b.setMotion("ballDownCurve", beat+1, 0.75, easeInCubic)
			m.ctx.At(beat+2, func() { b.dead = true })
		} else {
			b.dead = true
		}
	})
}

func (m *Module) updateObjects(beat float64) {
	penguins := m.activePenguins[:0]
	for _, p := range m.activePenguins {
		if !p.dead {
			p.update(beat, m.curves)
			penguins = append(penguins, p)
		}
	}
	m.activePenguins = penguins
	balls := m.balls[:0]
	for _, b := range m.balls {
		if !b.dead {
			b.update(beat, m.curves)
			balls = append(balls, b)
		}
	}
	m.balls = balls
}

func (m *Module) latestPenguin(typ int) *activePenguin {
	for i := len(m.activePenguins) - 1; i >= 0; i-- {
		p := m.activePenguins[i]
		if p.typ == typ && !p.dead {
			return p
		}
	}
	return nil
}

func (m *Module) autoBopAt(beat float64) bool {
	if len(m.bops) == 0 {
		return true
	}
	for _, ev := range m.bops {
		if beat >= ev.beat && beat < ev.beat+ev.length-1e-6 {
			return ev.auto
		}
	}
	return false
}

func (m *Module) applyBackground(beat float64) {
	a, b, c := defaultA, defaultB, defaultC
	for _, ev := range m.bgs {
		if beat < ev.beat {
			break
		}
		u := 1.0
		if ev.length > 0 && beat < ev.beat+ev.length {
			u = (beat - ev.beat) / ev.length
		}
		a = easeColor(ev.ease, ev.a0, ev.a1, u)
		b = easeColor(ev.ease, ev.b0, ev.b1, u)
		c = easeColor(ev.ease, ev.c0, ev.c1, u)
	}
	m.ctx.Scene.SetPaletteFor("backgroundRecolorable", kart.Palette{Alpha: b, Fill: a, Outline: c})
}

func (m *Module) point(role string) (float64, float64) {
	path := roleOr(m.ctx, role, "")
	for _, n := range m.ctx.Assets.Rig.Nodes {
		if n.Path == path {
			return n.Pos[0], n.Pos[1]
		}
	}
	return 0, 0
}

func (p *activePenguin) play(ctx *engine.Ctx, state string, beat, speed float64) {
	p.inst.Play(p.animRel, p.clipPrefix+"/"+state, beat, ctx.SecPerBeat(beat)*speed)
}

func (p *activePenguin) setMotion(curve string, start, dur float64, ease int) {
	p.motion = motion{curve: curve, start: start, dur: dur, ease: ease}
}

func (p *activePenguin) setStatic(x, y float64) {
	p.motion.staticX, p.motion.staticY, p.motion.hasStatic = x, y, true
	p.inst.Offset = [2]float64{x, y}
}

func (p *activePenguin) clearMotion() { p.motion = motion{} }

func (p *activePenguin) catchSound() string {
	switch p.typ {
	case penguinWhite:
		return "small4"
	case penguinBig:
		return "large4"
	default:
		return "medium4"
	}
}

func (p *activePenguin) update(beat float64, curves map[string]kmdata.Curve) {
	if p.motion.curve == "" {
		if p.motion.hasStatic {
			p.inst.Offset = [2]float64{p.motion.staticX, p.motion.staticY}
		}
		return
	}
	c, ok := curves[p.motion.curve]
	if !ok {
		return
	}
	u := clamp01((beat - p.motion.start) / p.motion.dur)
	u = engine.Ease(p.motion.ease, 0, 1, u)
	if u < 1 {
		pos := kart.EvalBezier(c, u)
		p.inst.Offset = [2]float64{pos[0], pos[1]}
	}
}

func (b *activeBall) setMotion(curve string, start, dur float64, ease int) {
	b.motion = motion{curve: curve, start: start, dur: dur, ease: ease}
}

func (b *activeBall) update(beat float64, curves map[string]kmdata.Curve) {
	c, ok := curves[b.motion.curve]
	if !ok {
		return
	}
	u := clamp01((beat - b.motion.start) / b.motion.dur)
	u = engine.Ease(b.motion.ease, 0, 1, u)
	if u < 1 {
		pos := kart.EvalBezier(c, u)
		b.inst.Offset = [2]float64{pos[0], pos[1]}
	}
}

type soundCue struct {
	off, pitch float64
}

func cueSounds(typ int) []soundCue {
	switch typ {
	case penguinWhite:
		return []soundCue{{0, 3.1}, {0.5, 2}, {1, 3.1}, {1.5, 2}}
	case penguinBig:
		return []soundCue{{0, 1.5}, {2, 1.2}}
	default:
		return []soundCue{{0, 2.1}, {1, 1.4}}
	}
}

func penguinStartBeat(beat float64, typ int) float64 {
	if typ == penguinWhite {
		return beat - 0.5
	}
	return beat - 1
}

func penguinTargetBeat(beat float64, typ int) float64 {
	if typ == penguinBig {
		return beat + 4
	}
	return beat + 2
}

func penguinTargetFromStart(start float64, typ int) float64 {
	switch typ {
	case penguinWhite:
		return start + 2.5
	case penguinBig:
		return start + 5
	default:
		return start + 3
	}
}

func penguinCheckBeat(start float64, typ int) float64 {
	switch typ {
	case penguinWhite:
		return start + 3.5
	case penguinBig:
		return start + 6
	default:
		return start + 4
	}
}

func penguinSlideBeat(start float64, typ int) float64 {
	switch typ {
	case penguinWhite:
		return start + 4
	case penguinBig:
		return start + 6.5
	default:
		return start + 4.5
	}
}

func penguinAnimRel(typ int) string {
	if typ == penguinBig {
		return "penguinBig"
	}
	return "penguinWhite"
}

func penguinClipPrefix(typ int) string {
	switch typ {
	case penguinGray:
		return "Gray"
	case penguinWhite:
		return "White"
	default:
		return "Big"
	}
}

func roleOr(ctx *engine.Ctx, key, fallback string) string {
	if v := ctx.Role(key); v != "" {
		return v
	}
	return fallback
}

func boolParam(e *riq.Entity, key string) bool { return boolParamDefault(e, key, false) }

func boolParamDefault(e *riq.Entity, key string, def bool) bool {
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
	m, ok := v.(map[string]any)
	if !ok {
		return def
	}
	return [4]float64{
		floatOr(m["r"], def[0]),
		floatOr(m["g"], def[1]),
		floatOr(m["b"], def[2]),
		floatOr(m["a"], def[3]),
	}
}

func easeColor(kind int, a, b [4]float64, u float64) [4]float64 {
	return [4]float64{
		engine.Ease(kind, a[0], b[0], u),
		engine.Ease(kind, a[1], b[1], u),
		engine.Ease(kind, a[2], b[2], u),
		engine.Ease(kind, a[3], b[3], u),
	}
}

func floatOr(v any, def float64) float64 {
	if f, ok := v.(float64); ok {
		return f
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
