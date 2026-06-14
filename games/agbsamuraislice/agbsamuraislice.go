// Package agbsamuraislice ports Samurai Slice (Iai Giri)'s demon approach
// curves, samurai/fire/fog animator states, slice state machine, and fog
// MultiSound timing from Assets/Scripts/Games/SamuraiSliceAgb.
package agbsamuraislice

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
	yokaiSmall = iota
	yokaiRibbon
	yokaiMedium
	yokaiFlying
	yokaiBig
	yokaiMaya
)

const floorHeight = 1.9

type bopEvt struct {
	beat, length float64
	auto, toggle bool
}

type demonEvt struct {
	beat     float64
	typ      int
	slowDown bool
	uid      int
}

type fogEvt struct {
	beat, length float64
	opacity      float64
	long, mute   bool
}

type enterEvt struct {
	beat, length float64
}

type stateEvt struct {
	beat  float64
	state int
}

type activeYokai struct {
	ev          demonEvt
	inst        *kart.Instance
	shadow      *kart.Instance
	big         bool
	maya        bool
	dead        bool
	missed      bool
	shadowOffX  float64
	curvePrefix string
}

type burstParticle struct {
	born, life float64
	pos, vel   [2]float64
	size       float64
	col        [4]float64
}

type Module struct {
	ctx  *engine.Ctx
	proj kart.Aff
	rng  *rand.Rand

	samurai   string
	fireAnim  string
	fireRoot  string
	fogAnim   string
	yokaiRoot string
	mayaRoot  string

	yokaiT     *kart.Template
	mayaT      *kart.Template
	shadowT    *kart.Template
	bigShadowT *kart.Template

	gameComp  kmdata.Component
	yokaiComp kmdata.Component
	mayaComp  kmdata.Component
	curves    map[string]kmdata.Curve
	fogParts  []string

	bops   []bopEvt
	demons []demonEvt
	fogs   []fogEvt
	enters []enterEvt
	states []stateEvt

	active []*activeYokai
	parts  []burstParticle

	autoBop   bool
	lastPulse int

	samuraiState int
	fireState    int
	sliceState   int

	stepStart float64
	stepLen   float64
	startPos  float64
	stepDist  float64

	fireDisappearing bool
	fireLeaveBeat    float64
	fireNormalY      float64

	fogBeat      float64
	fogLen       float64
	fogOpacity   float64
	fogFadeStart float64
	fogAlpha     float64
	fogActive    bool
}

func New() engine.Module {
	return &Module{
		rng:       rand.New(rand.NewSource(0x1a17)),
		autoBop:   true,
		lastPulse: -1 << 30,
		stepStart: math.Inf(1),
		fogBeat:   math.Inf(1),
		stepLen:   16,
	}
}

func (m *Module) ID() string { return "agbSamuraiSlice" }

func (m *Module) Load(ctx *engine.Ctx) error {
	m.ctx = ctx
	if err := ctx.LoadAssets("agbSamuraiSlice"); err != nil {
		return err
	}
	m.proj = kart.Translate(engine.ScreenW/2, engine.ScreenH/2).Mul(kart.Scale(54, -54))
	m.gameComp = ctx.Assets.Extra.Components["game"]
	m.yokaiComp = ctx.Assets.Extra.Components["yokai"]
	m.mayaComp = ctx.Assets.Extra.Components["maya"]
	m.curves = ctx.Assets.Extra.Curves

	m.samurai = refOr(ctx, m.gameComp, "samuraiAnim", "samurai")
	m.fireAnim = refOr(ctx, m.gameComp, "fireAnim", "fireparent/fire1")
	m.fireRoot = refOr(ctx, m.gameComp, "fireParent", "fireparent")
	m.fogAnim = refOr(ctx, m.gameComp, "fogAnim", "narumayo canon")
	m.yokaiRoot = refOr(ctx, m.gameComp, "yokaiEntity", "yokai1")
	m.mayaRoot = refOr(ctx, m.gameComp, "mayaFeyFromAceAttorney", "AA1_Maya_0")
	m.fogParts = append([]string(nil), m.gameComp.RefArrays["fogSprite"]...)
	if len(m.fogParts) == 0 {
		m.fogParts = []string{"narumayo canon/fog1", "narumayo canon/fog2"}
	}

	m.startPos = numOr(m.gameComp, "startPosition", -10.5)
	m.stepDist = numOr(m.gameComp, "stepDistance", 5)
	m.fireNormalY = numOr(m.gameComp, "fireNormalY", 0)

	m.yokaiT = kart.NewTemplate(ctx.Assets, m.yokaiRoot)
	m.mayaT = kart.NewTemplate(ctx.Assets, m.mayaRoot)
	m.shadowT = kart.NewTemplate(ctx.Assets, refOr(ctx, m.yokaiComp, "shadow", "shadow"))
	m.bigShadowT = kart.NewTemplate(ctx.Assets, refOr(ctx, m.yokaiComp, "bigShadowObject", "bigshadow"))
	m.resetScene(0)
	return nil
}

func refOr(ctx *engine.Ctx, c kmdata.Component, field, fallback string) string {
	if c.Refs != nil && c.Refs[field] != "" {
		return c.Refs[field]
	}
	if p := ctx.Role(field); p != "" {
		return p
	}
	return fallback
}

func numOr(c kmdata.Component, key string, fallback float64) float64 {
	if c.Nums != nil {
		if v, ok := c.Nums[key]; ok {
			return v
		}
	}
	return fallback
}

func (m *Module) OnEvent(e *riq.Entity) {
	b := e.Beat
	switch e.Datamodel {
	case "agbSamuraiSlice/bop":
		ev := bopEvt{beat: b, length: e.Length, auto: boolDefault(e, "auto", true), toggle: boolDefault(e, "toggle", true)}
		m.bops = append(m.bops, ev)
		m.ctx.At(b, func() {
			m.autoBop = ev.auto
			if ev.toggle {
				for i := 0.0; i < ev.length-1e-6; i++ {
					bb := ev.beat + i
					m.ctx.At(bb, func() { m.bop(bb) })
				}
			}
		})
	case "agbSamuraiSlice/demon":
		ev := demonEvt{beat: b, typ: intParam(e, "type", yokaiSmall), slowDown: boolParam(e, "slowDown"), uid: len(m.demons)}
		m.demons = append(m.demons, ev)
		m.ctx.SoundAt(b+5, "yokaijump", 1)
		m.ctx.ScheduleInput(b+6, func(state float64, j engine.Judgment) { m.hitDemon(ev, state, j) }, func() { m.missDemon(ev) })
		m.ctx.At(b, func() { m.spawnYokai(ev) })
	case "agbSamuraiSlice/fog":
		ev := fogEvt{
			beat: b, length: e.Length, opacity: e.Float("opacity", 100),
			long: boolParam(e, "long"), mute: boolDefault(e, "mute", true),
		}
		m.fogs = append(m.fogs, ev)
		m.scheduleFogSounds(ev)
		m.ctx.At(b, func() { m.setFog(ev) })
	case "agbSamuraiSlice/rest":
		m.ctx.At(b, func() { m.rest(b) })
	case "agbSamuraiSlice/enter":
		ev := enterEvt{beat: b, length: e.Length}
		m.enters = append(m.enters, ev)
		m.ctx.At(b, func() { m.enter(ev) })
	case "agbSamuraiSlice/set state":
		ev := stateEvt{beat: b, state: intParam(e, "state", 0)}
		m.states = append(m.states, ev)
		m.ctx.At(b, func() { m.setState(ev.state, b) })
	}
}

func (m *Module) Ready() {
	sort.Slice(m.bops, func(i, j int) bool { return m.bops[i].beat < m.bops[j].beat })
	sort.Slice(m.demons, func(i, j int) bool { return m.demons[i].beat < m.demons[j].beat })
	sort.Slice(m.fogs, func(i, j int) bool { return m.fogs[i].beat < m.fogs[j].beat })
	sort.Slice(m.enters, func(i, j int) bool { return m.enters[i].beat < m.enters[j].beat })
	sort.Slice(m.states, func(i, j int) bool { return m.states[i].beat < m.states[j].beat })
}

func (m *Module) OnSwitch(beat float64) {
	m.resetScene(beat)
	for _, ev := range m.states {
		if ev.beat > beat {
			break
		}
		m.setState(ev.state, ev.beat)
	}
	for _, ev := range m.enters {
		if ev.beat <= beat && beat < ev.beat+ev.length {
			m.stepStart, m.stepLen = ev.beat, ev.length
		}
		if ev.beat > beat {
			m.ctx.Scene.SetPosOver(m.samurai, m.startPos, 0)
			break
		}
	}
	for _, ev := range m.fogs {
		if ev.beat <= beat && beat < ev.beat+ev.length {
			m.setFog(ev)
			m.updateFog(beat)
		}
	}
	for _, ev := range m.demons {
		if ev.beat <= beat && beat <= ev.beat+5 {
			m.spawnYokai(ev)
		}
	}
	m.lastPulse = int(math.Floor(beat))
}

func (m *Module) Whiff(beat float64) {
	if m.ctx.Scene.Current(m.samurai) == "Animations/Miss" {
		return
	}
	m.slice(beat)
	m.ctx.Sound("swing")
}

func (m *Module) Update(t, beat float64) {
	pulse := int(math.Floor(beat + 1e-6))
	if pulse != m.lastPulse {
		m.lastPulse = pulse
		if m.autoBop && (m.ctx.Scene.Current(m.samurai) == "" || m.ctx.Scene.Current(m.samurai) == "Animations/Idle") {
			m.bop(float64(pulse))
		}
	}
	m.updateEnter(beat)
	m.updateFire(beat)
	m.updateFog(beat)
	m.updateParticles(t)
}

func (m *Module) Draw(screen *ebiten.Image, _ float64, beat float64) {
	screen.Fill(color.RGBA{182, 181, 182, 255})
	m.ctx.SampleScene(beat)
	for _, y := range m.active {
		m.queueYokai(y, beat)
	}
	m.ctx.Scene.Draw(screen, m.proj)
	m.drawParticles(screen, m.ctx.Time())
}

func (m *Module) resetScene(beat float64) {
	sec := m.ctx.SecPerBeat(beat)
	for p := range m.ctx.Assets.Animators {
		m.ctx.Scene.PlayDefaultState(p, beat, sec)
	}
	for _, p := range []string{
		m.yokaiRoot, m.mayaRoot, "shadow", "bigshadow", "Refs", "heydontlookatthis",
		"yokaihalve", "mayahalve", "yokaiflyinghalve", "yokaihalve2", "yokaihalveflying2", "yokaihalve3",
		"yokaibarelyparticle", "mayabarelyparticle", "yokaibarelyparticle (1)",
		"yokai2barelyparticle", "yokai2barelyparticle (1)", "yokai3barelyparticle",
	} {
		m.ctx.Scene.SetActive(p, false)
	}
	m.ctx.Scene.SetPosOver(m.samurai, 0, 0)
	m.ctx.Scene.SetPosOver(m.fireRoot, 0, m.fireNormalY)
	m.ctx.Scene.SetActive(m.fireAnim, false)
	for _, p := range m.fogParts {
		m.ctx.Scene.SetColorOver(p, [4]float64{1, 1, 1, 0})
	}
	m.active = nil
	m.parts = nil
	m.autoBop = true
	m.samuraiState, m.fireState, m.sliceState = 0, 0, 0
	m.stepStart, m.stepLen = math.Inf(1), 16
	m.fireDisappearing = false
	m.fogBeat, m.fogLen, m.fogOpacity, m.fogFadeStart, m.fogAlpha = math.Inf(1), 0, 0, 0, 0
	m.fogActive = false
}

func (m *Module) bop(beat float64) {
	m.playSamuraiState([]string{"Bop", "Bop1", "Bop2"}[clampi(m.samuraiState, 0, 2)], beat)
}

func (m *Module) slice(beat float64) {
	m.playSamuraiState([]string{"Slice", "Slice1", "Slice2"}[clampi(m.samuraiState, 0, 2)], beat)
}

func (m *Module) playSamuraiState(state string, beat float64) {
	m.ctx.Scene.PlayState(m.samurai, state, beat, 0.5)
}

func (m *Module) spawnYokai(ev demonEvt) {
	for _, y := range m.active {
		if y.ev.uid == ev.uid && !y.dead {
			return
		}
	}
	typ := ev.typ
	maya := typ == yokaiMaya
	if maya {
		typ = yokaiSmall
	}
	tmpl := m.yokaiT
	comp := m.yokaiComp
	prefix := "yokai"
	if maya {
		tmpl, comp, prefix = m.mayaT, m.mayaComp, "maya"
	}
	if tmpl == nil {
		return
	}
	inst := tmpl.NewInstance()
	inst.PlayDefaultState("", ev.beat, m.ctx.SecPerBeat(ev.beat))
	m.playYokaiHop(inst, typ, ev.beat)
	shadowT := m.shadowT
	big := typ == yokaiBig
	if big {
		shadowT = m.bigShadowT
	}
	var shadow *kart.Instance
	if shadowT != nil {
		shadow = shadowT.NewInstance()
	}
	m.active = append(m.active, &activeYokai{
		ev: ev, inst: inst, shadow: shadow, big: big, maya: maya,
		shadowOffX: numOr(comp, "shadowOffsetX", 0), curvePrefix: prefix,
	})
}

func (m *Module) playYokaiHop(inst *kart.Instance, typ int, beat float64) {
	state := "Jump"
	switch typ {
	case yokaiRibbon:
		state = "Flying"
	case yokaiMedium:
		state = "Jump1"
	case yokaiFlying:
		state = "Flying1"
	case yokaiBig:
		state = "YokaiWalk"
	}
	for i := 0; i < 6; i++ {
		bb := beat + float64(i)
		m.ctx.At(bb, func() { inst.PlayState("", state, bb, 0.5) })
	}
}

func (m *Module) hitDemon(ev demonEvt, state float64, j engine.Judgment) {
	_ = j
	beat := m.ctx.Beat()
	m.slice(beat)
	y := m.findYokai(ev.uid)
	if y != nil {
		y.dead = true
	}
	barely := math.Abs(state) >= 1
	if barely {
		m.ctx.Scene.SetActive(m.fireAnim, false)
		m.fireState, m.samuraiState, m.sliceState = 0, 0, 0
		m.ctx.Sound("barely")
		m.emitBurst(ev, true, beat)
	} else {
		if m.sliceState < 5 {
			m.sliceState++
		}
		m.ctx.Sound(m.sliceSound(ev.slowDown))
		m.emitBurst(ev, false, beat)
		if m.samuraiState < 2 {
			m.samuraiState++
		}
		if m.fireState < 8 {
			m.fireState++
		}
		if m.fireState > 2 {
			m.ctx.Scene.SetActive(m.fireAnim, true)
			m.ctx.Scene.PlayState(m.fireAnim, "Fire"+itoa(m.fireState-2), beat, 0.5)
		}
		if ev.slowDown {
			// HS also pitch-shifts the whole minigame for one beat. The current
			// engine has no chart-wide resampler yet, so this preserves the
			// visible Flash animation and slow-version slice SFX while README
			// tracks the remaining audio-engine gap explicitly.
			m.ctx.Scene.PlayState("flash", "Flash", beat, 0.5)
		}
	}
	m.slice(beat)
	m.fogSlice(barely, beat)
}

func (m *Module) missDemon(ev demonEvt) {
	beat := m.ctx.Beat()
	y := m.findYokai(ev.uid)
	if y != nil {
		y.missed = true
	}
	m.ctx.Sound("miss")
	m.ctx.Scene.PlayState(m.samurai, "Miss", beat, 0.5)
	m.samuraiState, m.fireState, m.sliceState = 0, 0, 0
	m.ctx.Scene.SetActive(m.fireAnim, false)
	m.fogSlice(true, beat)
}

func (m *Module) findYokai(uid int) *activeYokai {
	for _, y := range m.active {
		if y.ev.uid == uid {
			return y
		}
	}
	return nil
}

func (m *Module) sliceSound(slow bool) string {
	switch {
	case m.sliceState >= 5 && slow:
		return "slicehighslow"
	case m.sliceState >= 5:
		return "slicehigh"
	case m.sliceState >= 3 && slow:
		return "slicemediumslow"
	case m.sliceState >= 3:
		return "slicemedium"
	case slow:
		return "slicelowslow"
	default:
		return "slicelow"
	}
}

func (m *Module) setState(state int, beat float64) {
	m.samuraiState = clampi(state, 0, 2)
	m.fireState = state
	if m.fireState > 2 {
		m.ctx.Scene.SetActive(m.fireAnim, true)
		m.ctx.Scene.PlayState(m.fireAnim, "Fire"+itoa(m.fireState-2), beat, 0.5)
	}
	m.playSamuraiState([]string{"Idle", "Idle1", "Idle2"}[m.samuraiState], beat)
	m.sliceState = clampi(state, 0, 5)
}

func (m *Module) rest(beat float64) {
	if m.samuraiState >= 2 {
		m.ctx.Scene.PlayState(m.samurai, "Idle1", beat, 0.5)
	}
	m.ctx.At(beat+1, func() { m.ctx.Scene.PlayState(m.samurai, "Rest", beat+1, 0.5) })
	m.fireDisappearing = true
	m.fireLeaveBeat = beat
}

func (m *Module) enter(ev enterEvt) {
	m.stepStart, m.stepLen = ev.beat, ev.length
	m.ctx.Scene.SetPosOver(m.samurai, m.startPos, 0)
	for i := 0.0; i < ev.length-1e-6; i++ {
		bb := ev.beat + i
		m.ctx.At(bb, func() { m.bop(bb) })
	}
}

func (m *Module) updateEnter(beat float64) {
	if beat >= m.stepStart+math.Floor(m.stepLen) {
		m.stepStart = math.Inf(1)
		return
	}
	if beat < m.stepStart {
		return
	}
	current := math.Floor(beat - m.stepStart)
	start := m.stepStart + current
	prog := clamp01(((beat - start) / 1.25) * 4)
	move := easeOutQuint(prog)
	step := m.stepDist / math.Floor(m.stepLen)
	m.ctx.Scene.SetPosOver(m.samurai, m.startPos+step*current+step*move, 0)
}

func (m *Module) updateFire(beat float64) {
	if !m.fireDisappearing {
		return
	}
	u := beat - m.fireLeaveBeat
	m.ctx.Scene.SetPosOver(m.fireRoot, 0, m.fireNormalY-1.6*u)
	if u > 1 {
		m.ctx.Scene.SetActive(m.fireAnim, false)
		m.fireDisappearing = false
	}
}

func (m *Module) setFog(ev fogEvt) {
	m.fogBeat, m.fogLen, m.fogOpacity = ev.beat, ev.length, ev.opacity
	m.fogFadeStart = m.fogAlpha
	m.fogActive = ev.opacity > 0
	m.ctx.Scene.PlayState(m.fogAnim, "Idle", ev.beat, 0.5)
}

func (m *Module) updateFog(beat float64) {
	if beat < m.fogBeat || beat > m.fogBeat+m.fogLen {
		return
	}
	fade := 1.0
	if m.fogLen > 0 {
		fade = clamp01((beat - m.fogBeat) / m.fogLen)
	}
	m.fogAlpha = fade*(m.fogOpacity/100)*(1-m.fogFadeStart) + m.fogFadeStart
	for _, p := range m.fogParts {
		m.ctx.Scene.SetColorOver(p, [4]float64{1, 1, 1, m.fogAlpha})
	}
}

func (m *Module) fogSlice(barely bool, beat float64) {
	if !m.fogActive {
		return
	}
	if barely {
		m.fogAlpha = 0
		for _, p := range m.fogParts {
			m.ctx.Scene.SetColorOver(p, [4]float64{1, 1, 1, 0})
		}
	} else {
		m.ctx.Scene.PlayState(m.fogAnim, "SliceFog", beat, 0.5)
	}
	m.fogActive = false
}

func (m *Module) scheduleFogSounds(ev fogEvt) {
	if ev.mute {
		return
	}
	last := 25
	special := "fog25long"
	if ev.long {
		last = 32
		special = "fog25short"
	}
	for i := 1; i <= last; i++ {
		name := "fog" + itoa(i)
		if i == 25 {
			name = special
		}
		m.ctx.SoundAt(ev.beat+0.25*float64(i-1), name, 1)
	}
}

func (m *Module) queueYokai(y *activeYokai, beat float64) {
	if y.dead {
		return
	}
	pos := m.yokaiPos(y, beat)
	if y.missed && beat > y.ev.beat+7 {
		y.dead = true
		return
	}
	shadowY := floorHeight
	if beat < y.ev.beat+5 {
		shadowY = clamp01((beat-y.ev.beat)/5)*-2 + floorHeight
	} else {
		shadowY = -4.43
	}
	if y.shadow != nil {
		y.shadow.Offset = [2]float64{pos[0] + y.shadowOffX, shadowY}
		y.shadow.Queue(m.ctx.Scene, beat, kart.Identity(), 0)
	}
	y.inst.Offset = pos
	y.inst.Queue(m.ctx.Scene, beat, kart.Identity(), 0)
}

func (m *Module) yokaiPos(y *activeYokai, beat float64) [2]float64 {
	if y.missed {
		u := clamp01(beat - (y.ev.beat + 6))
		p := kart.EvalBezier(m.curves[y.curvePrefix+".missCurve"], u)
		return [2]float64{p[0], p[1]}
	}
	idx := int(math.Floor(beat - y.ev.beat))
	if idx < 0 {
		idx = 0
	}
	if y.big && idx <= 4 {
		u := clamp01((beat - y.ev.beat) / 5)
		comp := m.yokaiComp
		if y.maya {
			comp = m.mayaComp
		}
		x := numOr(comp, "bigYokaiStartPosition.x", 8.8) + numOr(comp, "bigYokaiXDistance", -10)*u
		yy := numOr(comp, "bigYokaiStartPosition.y", 2.7) + numOr(comp, "bigYokaiYDistance", -1.9)*u
		return [2]float64{x, yy}
	}
	if idx > 6 {
		idx = 6
	}
	key := y.curvePrefix + ".enterCurves" + itoa(idx)
	if y.ev.typ == yokaiRibbon || y.ev.typ == yokaiFlying {
		key = y.curvePrefix + ".flyingCurves" + itoa(idx)
	}
	u := clamp01(beat - (y.ev.beat + float64(idx)))
	p := kart.EvalBezier(m.curves[key], u)
	return [2]float64{p[0], p[1]}
}

func (m *Module) emitBurst(ev demonEvt, barely bool, beat float64) {
	t := m.ctx.Time()
	pos := [2]float64{-3.0069997, 0.29999995}
	if y := m.findYokai(ev.uid); y != nil {
		pos = m.yokaiPos(y, beat)
	}
	count := 18
	base := [4]float64{1, 0.32, 0, 1}
	if barely {
		count = 10
		base = [4]float64{1, 1, 1, 1}
	}
	for i := 0; i < count; i++ {
		ang := -math.Pi*0.15 + (m.rng.Float64()-0.5)*math.Pi*0.9
		if i%2 == 0 {
			ang += math.Pi
		}
		speed := 1.2 + m.rng.Float64()*2.8
		m.parts = append(m.parts, burstParticle{
			born: t, life: 0.42 + m.rng.Float64()*0.22, pos: pos,
			vel:  [2]float64{math.Cos(ang) * speed, math.Sin(ang) * speed},
			size: 0.035 + m.rng.Float64()*0.045, col: base,
		})
	}
}

func (m *Module) updateParticles(t float64) {
	alive := m.parts[:0]
	for _, p := range m.parts {
		if t-p.born <= p.life {
			alive = append(alive, p)
		}
	}
	m.parts = alive
}

func (m *Module) drawParticles(screen *ebiten.Image, t float64) {
	for _, p := range m.parts {
		age := t - p.born
		if age < 0 || age > p.life {
			continue
		}
		u := age / p.life
		x := p.pos[0] + p.vel[0]*age
		y := p.pos[1] + p.vel[1]*age
		sx, sy := m.proj.Apply(x, y)
		a := float32((1 - u) * p.col[3])
		c := color.RGBA{
			R: byte(clamp01(p.col[0]) * 255),
			G: byte(clamp01(p.col[1]) * 255),
			B: byte(clamp01(p.col[2]) * 255),
			A: byte(a * 255),
		}
		r := float32((p.size + p.size*u) * 54)
		vector.DrawFilledCircle(screen, float32(sx), float32(sy), r, c, true)
	}
}

func boolParam(e *riq.Entity, key string) bool { return e.Float(key, 0) != 0 }

func boolDefault(e *riq.Entity, key string, def bool) bool {
	if _, ok := e.Data[key]; !ok {
		return def
	}
	return boolParam(e, key)
}

func intParam(e *riq.Entity, key string, def int) int {
	if _, ok := e.Data[key]; !ok {
		return def
	}
	return int(e.Float(key, float64(def)))
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

func clampi(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func easeOutQuint(u float64) float64 {
	u = 1 - clamp01(u)
	return 1 - u*u*u*u*u
}

func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	buf := [12]byte{}
	i := len(buf)
	n := v
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
