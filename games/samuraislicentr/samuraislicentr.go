// Package samuraislicentr ports Samurai Slice (Nintendo DS)'s launcher,
// thrown-object state machine, moonrise, faster warning, and ambient particle
// controls from Assets/Scripts/Games/SamuraiSliceNtr.
package samuraislicentr

import (
	"image/color"
	"math"
	"math/rand"
	"sort"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"hsdemo/engine"
	"hsdemo/kart"
	"hsdemo/kmdata"
	"hsdemo/riq"
)

const (
	objMelon = iota
	objFish
	objDemon
	objMelon2B2T
)

const (
	actionSlice = 0
	actionStep  = 3 // HS InputAction_Alt/South; engine channel L/Down/X.
)

const (
	particleNone = iota
	particleCherry
	particleLeaf
	particleLeafBroken
	particleSnow
)

var (
	defaultDayColor   = [4]float64{0xc0 / 255.0, 0xc0 / 255.0, 0xc0 / 255.0, 1}
	defaultNightColor = [4]float64{0x00 / 255.0, 0x2b / 255.0, 0x8c / 255.0, 1}
)

type bopEvt struct {
	beat, length float64
	bop, auto    bool
}

type moonEvt struct {
	beat, length float64
	enter        bool
	ease         int
	text         string
	changeSky    bool
	color        [4]float64
}

type fasterEvt struct {
	beat, length float64
	typ          int
	darken       bool
}

type moonMove struct {
	moonEvt
	active bool
	from   [4]float64
	to     [4]float64
}

type ambientParticle struct {
	born, life float64
	x, y       float64
	vx, vy     float64
	size       float64
	col        [4]float64
}

type Module struct {
	ctx  *engine.Ctx
	proj kart.Aff
	rng  *rand.Rand

	samurai  string
	launcher string
	child    string
	object   string
	bg       string
	faster   string
	overlay  string
	moon     string
	moonTxt  string

	gameComp   kmdata.Component
	objectComp kmdata.Component
	childComp  kmdata.Component
	curves     map[string]kmdata.Curve
	objT       *kart.Template
	childT     *kart.Template

	bops    []bopEvt
	moons   []moonEvt
	fasters []fasterEvt
	objects []*samObject
	kids    []*carryChild

	stepping    bool
	autoBop     bool
	lastPulse   int
	moonState   moonMove
	bgColor     [4]float64
	effectType  int
	effectRate  float64
	effectWind  float64
	effectAccum float64
	parts       []ambientParticle
	lastT       float64
	hasLastT    bool
}

func New() engine.Module {
	return &Module{
		rng:       rand.New(rand.NewSource(0x5a71ce)),
		autoBop:   true,
		lastPulse: -1 << 30,
		bgColor:   defaultDayColor,
	}
}

func (m *Module) ID() string { return "samuraiSliceNtr" }

func (m *Module) Load(ctx *engine.Ctx) error {
	m.ctx = ctx
	if err := ctx.LoadAssets("samuraiSliceNtr"); err != nil {
		return err
	}
	if err := ctx.Assets.ApplyTexts(); err != nil {
		return err
	}
	m.proj = kart.Translate(engine.ScreenW/2, engine.ScreenH/2).Mul(kart.Scale(54, -54))
	m.gameComp = ctx.Assets.Extra.Components["game"]
	m.objectComp = ctx.Assets.Extra.Components["object"]
	m.childComp = ctx.Assets.Extra.Components["child"]
	m.curves = ctx.Assets.Extra.Curves

	m.samurai = roleOr(ctx, "player", "Samurai")
	m.launcher = roleOr(ctx, "launcher", "Launcher")
	m.child = roleOr(ctx, "childParent", "Child")
	m.object = roleOr(ctx, "objectPrefab", "ObjectRoot")
	m.bg = roleOr(ctx, "background", "BG/background")
	m.faster = roleOr(ctx, "fasterWarning", "Faster")
	m.overlay = roleOr(ctx, "darknessOverlay", "Faster/overlay")
	m.moon = roleOr(ctx, "theMoon", "BG/Moon")
	m.moonTxt = roleOr(ctx, "moonText", "BG/Moon/moon/text")
	m.objT = kart.NewTemplate(ctx.Assets, m.object)
	m.childT = kart.NewTemplate(ctx.Assets, m.child)
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
	b := e.Beat
	switch e.Datamodel {
	case "samuraiSliceNtr/bop":
		m.bops = append(m.bops, bopEvt{
			beat: b, length: e.Length,
			bop:  boolDefault(e, "bop", true),
			auto: boolParam(e, "bopAuto"),
		})
	case "samuraiSliceNtr/melon":
		typ := objMelon
		if boolParam(e, "2b2t") {
			typ = objMelon2B2T
		}
		m.ctx.At(b, func() { m.spawnObject(b, typ, intParam(e, "valA", 1)) })
	case "samuraiSliceNtr/fish":
		m.ctx.At(b, func() { m.spawnObject(b, objFish, intParam(e, "valA", 1)) })
	case "samuraiSliceNtr/demon":
		m.ctx.At(b, func() { m.spawnObject(b, objDemon, intParam(e, "valA", 1)) })
	case "samuraiSliceNtr/spawn object":
		typ := intParam(e, "type", objMelon)
		m.ctx.At(b, func() { m.spawnObject(b, typ, intParam(e, "valA", 1)) })
	case "samuraiSliceNtr/faster":
		ev := fasterEvt{beat: b, length: e.Length, typ: intParam(e, "type", 0), darken: boolDefault(e, "overlay", true)}
		m.fasters = append(m.fasters, ev)
		m.ctx.At(b, func() { m.startFaster(ev) })
		m.ctx.At(b+ev.length, func() { m.endFaster(ev) })
	case "samuraiSliceNtr/moon":
		ev := moonEvt{
			beat: b, length: e.Length, enter: boolDefault(e, "exit", true),
			ease: intParam(e, "ease", 0), text: e.Str("text", ""),
			changeSky: boolDefault(e, "changecolor", true),
			color:     colorParam(e, "color", defaultNightColor),
		}
		m.moons = append(m.moons, ev)
		m.ctx.At(b, func() { m.startMoon(ev, b) })
	case "samuraiSliceNtr/particle effects":
		typ := intParam(e, "type", particleNone)
		instant := boolParam(e, "instant")
		wind := e.Float("valA", 0)
		rate := e.Float("valB", 1)
		m.ctx.At(b, func() { m.setParticleEffect(typ, instant, wind, rate, b) })
	}
}

func (m *Module) Ready() {
	sort.SliceStable(m.bops, func(i, j int) bool { return m.bops[i].beat < m.bops[j].beat })
	sort.SliceStable(m.moons, func(i, j int) bool { return m.moons[i].beat < m.moons[j].beat })

	bopBeats := map[float64]bool{}
	for i, ev := range m.bops {
		if ev.bop {
			for k := 0; float64(k) < ev.length-1e-6; k++ {
				bopBeats[ev.beat+float64(k)] = true
			}
		}
		if ev.auto {
			end := ev.beat + ev.length
			for j := i + 1; j < len(m.bops); j++ {
				if m.bops[j].beat > ev.beat {
					end = m.bops[j].beat
					break
				}
			}
			for bb := math.Ceil(ev.beat); bb < end; bb++ {
				bopBeats[bb] = true
			}
		}
	}
	for bb := range bopBeats {
		beat := bb
		m.ctx.At(beat, func() { m.bop(beat) })
	}
}

func (m *Module) OnSwitch(beat float64) {
	m.resetScene(beat)
	m.objects = liveObjects(m.objects, beat)
	m.kids = liveKids(m.kids, beat)
	m.applyMoonAt(beat)
}

func (m *Module) Whiff(beat float64) { m.WhiffAction(beat, actionSlice) }

func (m *Module) WhiffAction(beat float64, action int) {
	switch action {
	case actionStep:
		m.doStep(beat)
	case actionSlice:
		m.doSlice(beat)
	}
}

func (m *Module) Update(t, beat float64) {
	dt := 0.0
	if m.hasLastT {
		dt = math.Max(0, math.Min(0.1, t-m.lastT))
	}
	m.lastT, m.hasLastT = t, true

	if m.autoBop {
		pulse := int(math.Floor(beat + 1e-6))
		if pulse != m.lastPulse && math.Abs(beat-float64(pulse)) < 0.08 {
			m.lastPulse = pulse
			m.bop(float64(pulse))
		}
	}
	m.updateMoon(beat)
	for _, o := range m.objects {
		o.update(m, beat, dt)
	}
	m.objects = liveObjects(m.objects, beat)
	m.kids = liveKids(m.kids, beat)
	m.updateParticles(beat, dt)
}

func (m *Module) Draw(screen *ebiten.Image, _ float64, beat float64) {
	m.ctx.Scene.SetColorOver(m.bg, m.bgColor)
	m.ctx.SampleScene(beat)
	for _, k := range m.kids {
		k.queue(m, beat)
	}
	for _, o := range m.objects {
		o.queue(m, beat)
	}
	m.ctx.Scene.Draw(screen, m.proj)
	m.drawParticles(screen, beat)
}

func (m *Module) resetScene(beat float64) {
	m.stepping = false
	m.autoBop = true
	m.bgColor = defaultDayColor
	m.moonState = moonMove{}
	m.ctx.Scene.SetActive(m.object, false)
	m.ctx.Scene.SetActive(m.overlay, false)
	sec := m.ctx.SecPerBeat(beat)
	for _, p := range []string{m.samurai, m.launcher, m.child, m.faster, m.moon} {
		m.ctx.Scene.PlayDefaultState(p, beat, sec)
	}
	_ = m.ctx.Assets.SetText(m.moonTxt, "555\nSAMU\nRAI")
	m.ctx.Scene.SetColorOver(m.bg, m.bgColor)
}

func (m *Module) bop(beat float64) {
	m.ctx.Scene.PlayState(m.child, "ChildBeat", beat, 0.5)
}

func (m *Module) doStep(beat float64) {
	m.ctx.Sound("ntrSamurai_launchThrough")
	m.stepping = true
	m.ctx.Scene.PlayState(m.samurai, "Step", beat, 0.5)
	m.ctx.Scene.PlayState(m.launcher, "Launch", beat, 0.5)
}

func (m *Module) doUnStep(beat float64) {
	if !m.stepping {
		return
	}
	m.stepping = false
	m.ctx.Scene.PlayState(m.samurai, "Unstep", beat, 0.5)
	m.ctx.Scene.PlayState(m.launcher, "UnStep", beat, 0.5)
}

func (m *Module) doSlice(beat float64) {
	if m.stepping {
		m.ctx.Scene.PlayState(m.launcher, "UnStep", beat, 0.5)
	}
	m.stepping = false
	m.ctx.Sound("ntrSamurai_through")
	m.ctx.Scene.PlayState(m.samurai, "Slash", beat, 0.5)
}

func (m *Module) startFaster(ev fasterEvt) {
	switch ev.typ {
	case 1:
		m.ctx.Sound("faster_question")
	case 2:
		m.ctx.Sound("faster_weird")
	default:
		m.ctx.Sound("faster_normal")
	}
	m.ctx.Scene.PlayState(m.faster, "Enter", ev.beat, 0.5)
	m.ctx.Scene.SetActive(m.overlay, ev.darken)
	if ev.darken {
		m.bgColor = [4]float64{0, 0, 0, 1}
	}
}

func (m *Module) endFaster(ev fasterEvt) {
	m.ctx.Scene.PlayState(m.faster, "Exit", ev.beat+ev.length, 0.5)
	m.ctx.Scene.SetActive(m.overlay, false)
	if ev.darken {
		m.bgColor = m.moonColorAt(ev.beat + ev.length)
	}
}

func (m *Module) startMoon(ev moonEvt, beat float64) {
	_ = m.ctx.Assets.SetText(m.moonTxt, ev.text)
	from, to := defaultDayColor, defaultDayColor
	if ev.changeSky {
		if ev.enter {
			to = ev.color
		} else {
			from = ev.color
		}
	}
	m.moonState = moonMove{moonEvt: ev, active: true, from: from, to: to}
	m.updateMoon(beat)
}

func (m *Module) updateMoon(beat float64) {
	if !m.moonState.active {
		return
	}
	ev := m.moonState.moonEvt
	u := clamp01((beat - ev.beat) / math.Max(ev.length, 1e-9))
	norm := engine.Ease(ev.ease, 0, 1, u)
	if !ev.enter {
		m.ctx.Scene.PlayFrozen(m.moon, "Exit", norm)
	} else {
		m.ctx.Scene.PlayFrozen(m.moon, "Enter", norm)
	}
	m.bgColor = lerpColor(m.moonState.from, m.moonState.to, ev.ease, u)
	if u >= 1 {
		m.moonState.active = false
	}
}

func (m *Module) applyMoonAt(beat float64) {
	for _, ev := range m.moons {
		if beat < ev.beat {
			break
		}
		_ = m.ctx.Assets.SetText(m.moonTxt, ev.text)
		from, to := defaultDayColor, defaultDayColor
		if ev.changeSky {
			if ev.enter {
				to = ev.color
			} else {
				from = ev.color
			}
		}
		if beat < ev.beat+ev.length {
			m.moonState = moonMove{moonEvt: ev, active: true, from: from, to: to}
			m.updateMoon(beat)
		} else {
			state, norm := "Enter", 1.0
			if !ev.enter {
				state = "Exit"
			}
			m.ctx.Scene.PlayFrozen(m.moon, state, norm)
			m.bgColor = to
		}
	}
}

func (m *Module) moonColorAt(beat float64) [4]float64 {
	c := defaultDayColor
	for _, ev := range m.moons {
		if beat < ev.beat {
			break
		}
		from, to := defaultDayColor, defaultDayColor
		if ev.changeSky {
			if ev.enter {
				to = ev.color
			} else {
				from = ev.color
			}
		}
		if beat < ev.beat+ev.length {
			u := clamp01((beat - ev.beat) / math.Max(ev.length, 1e-9))
			c = lerpColor(from, to, ev.ease, u)
		} else {
			c = to
		}
	}
	return c
}

func (m *Module) setParticleEffect(typ int, instant bool, wind, rate, beat float64) {
	m.effectType = typ
	m.effectWind = wind
	m.effectRate = rate
	if typ == particleNone {
		m.parts = nil
		return
	}
	if instant {
		for i := 0; i < int(rate); i++ {
			m.spawnAmbientParticle(beat)
		}
	}
}

func (m *Module) updateParticles(beat, dt float64) {
	if dt <= 0 {
		return
	}
	if m.effectType != particleNone && m.effectRate > 0 {
		m.effectAccum += m.effectRate * dt
		for m.effectAccum >= 1 {
			m.spawnAmbientParticle(beat)
			m.effectAccum--
		}
	}
	dst := m.parts[:0]
	for _, p := range m.parts {
		age := beat - p.born
		if age > p.life {
			continue
		}
		p.vx += m.effectWind * 0.02 * dt
		p.x += p.vx * dt
		p.y += p.vy * dt
		p.vy -= 0.7 * dt
		dst = append(dst, p)
	}
	m.parts = dst
}

func (m *Module) spawnAmbientParticle(beat float64) {
	p := ambientParticle{
		born: beat, life: 2.5 + m.rng.Float64()*1.5,
		x: -10 + m.rng.Float64()*20, y: 5 + m.rng.Float64()*4,
		vx: -0.4 + m.rng.Float64()*0.8, vy: -0.4 - m.rng.Float64()*1.2,
		size: 0.05 + m.rng.Float64()*0.08,
		col:  [4]float64{1, 1, 1, 0.8},
	}
	switch m.effectType {
	case particleCherry:
		p.col = [4]float64{1, 0.58, 0.8, 0.9}
	case particleLeaf:
		p.col = [4]float64{0.35, 0.85, 0.25, 0.9}
	case particleLeafBroken:
		p.col = [4]float64{0.62, 0.42, 0.22, 0.9}
	case particleSnow:
		p.x = -15 + m.rng.Float64()*30
		p.y = 7 + m.rng.Float64()*3
		p.vy = -0.25 - m.rng.Float64()*0.45
		p.size = 0.04 + m.rng.Float64()*0.05
	}
	m.parts = append(m.parts, p)
}

func (m *Module) drawParticles(screen *ebiten.Image, beat float64) {
	for _, p := range m.parts {
		age := clamp01((beat - p.born) / p.life)
		x, y := m.proj.Apply(p.x, p.y)
		alpha := uint8(clamp01((1-age)*p.col[3]) * 255)
		c := color.RGBA{uint8(p.col[0] * 255), uint8(p.col[1] * 255), uint8(p.col[2] * 255), alpha}
		r := float32(p.size * 54)
		vector.DrawFilledCircle(screen, float32(x), float32(y), r, c, true)
	}
}

func liveObjects(in []*samObject, beat float64) []*samObject {
	out := in[:0]
	for _, o := range in {
		if o != nil && !o.dead && beat < o.killBeat {
			out = append(out, o)
		}
	}
	return out
}

func liveKids(in []*carryChild, beat float64) []*carryChild {
	out := in[:0]
	for _, k := range in {
		if k != nil && beat < k.killBeat {
			out = append(out, k)
		}
	}
	return out
}
