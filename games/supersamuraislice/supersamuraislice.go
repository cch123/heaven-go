// Package supersamuraislice ports Super Samurai Slice's cue flow.
package supersamuraislice

import (
	"image/color"
	"math"
	"math/rand"
	"sort"

	"github.com/hajimehoshi/ebiten/v2"

	"hsdemo/engine"
	"hsdemo/kart"
	"hsdemo/riq"
)

const (
	actionSlice = 0
	actionAlt   = 3
)

const (
	platformSkateboard = iota
	platformEagle
)

const (
	effectExplode1 = iota
	effectExplode2
	effectExplode3
	effectLightning
	effectWaterL
	effectWaterR
)

type bopEvt struct {
	beat, length float64
	bop, auto    bool
}

type scrollEvt struct {
	beat, speed, offset float64
	platform            int
	instant, off, mute  bool
}

type smallEvt struct {
	beat, boomDelay, flipBack float64
	auto, autoBoom            bool
	count                     int
	enableBoom                bool
}

type effect struct {
	beat, startT float64
	typ          int
	pos          [2]float64
}

type Module struct {
	ctx  *engine.Ctx
	proj kart.Aff
	rng  *rand.Rand

	samurai       string
	samuraiHolder string
	samuraiShadow string
	bgAnim        string
	bg            string
	water         string
	waterGO       string
	fg            string
	fog           string
	fogGO         string
	cloud         string
	flash         string
	lightning     string
	waterL        string
	waterR        string
	skateRoot     string
	skate         string
	eagleRoot     string
	eagle         string

	smallT  *kart.Template
	mediumT *kart.Template
	paths   map[string]curvePath

	bops       []bopEvt
	scrolls    []scrollEvt
	smallRaw   []smallEvt
	smallPlans []smallPlan
	bigs       []bigPlan

	demons  []*activeDemon
	effects []effect

	autoBop          bool
	lastPulse        int
	waterActive      bool
	fogActive        bool
	isOn             bool
	skateboardActive bool
	eagleActive      bool
	largeDemonActive bool
	direction        int

	scrollSpeed float64
	scrollX     float64
	lastT       float64
	hasLastT    bool

	basePos map[string][2]float64
}

func New() engine.Module {
	return &Module{
		rng:         rand.New(rand.NewSource(0x53534c)),
		autoBop:     true,
		waterActive: true,
		lastPulse:   -1 << 30,
		basePos:     map[string][2]float64{},
	}
}

func (m *Module) ID() string { return "superSamuraiSlice" }

func (m *Module) Load(ctx *engine.Ctx) error {
	m.ctx = ctx
	if err := ctx.LoadAssets("superSamuraiSlice"); err != nil {
		return err
	}
	m.proj = kart.Translate(engine.ScreenW/2, engine.ScreenH/2).Mul(kart.Scale(54, -54))
	as := ctx.Assets
	m.samurai = roleOr(as, "SamuraiAnim", "SamuraiHolder/GameObject/Samurai")
	m.samuraiHolder = roleOr(as, "Samurai1Anim", "SamuraiHolder")
	m.samuraiShadow = roleOr(as, "SamuraiShadow", "SamuraiHolder/GameObject/Samurai/SamuraiHolder/Shadow")
	m.bgAnim = roleOr(as, "BGAnim", "BackgroundHolder/Holder")
	m.bg = roleOr(as, "BG", "BackgroundHolder/Holder/Main")
	m.water = roleOr(as, "Water", "BackgroundHolder/Holder/WaterHolder")
	m.waterGO = roleOr(as, "waterGO", "BackgroundHolder/Holder/WaterHolder")
	m.fg = roleOr(as, "FG", "BackgroundHolder/Holder/ForegroundHolder")
	m.fog = roleOr(as, "Fog", "BackgroundHolder/Holder/Fog")
	m.fogGO = roleOr(as, "fogGO", "BackgroundHolder/Holder/Fog")
	m.cloud = roleOr(as, "Cloud", "BackgroundHolder/Holder/CloudHolder")
	m.flash = roleOr(as, "flash", "Flash")
	m.lightning = roleOr(as, "lightning", "SamuraiHolder/GameObject/lightning/Main")
	m.waterL = roleOr(as, "waterL", "WaterParticle/WaterL")
	m.waterR = roleOr(as, "waterR", "WaterParticle/WaterR")
	m.skateRoot = roleOr(as, "Skateboard1Anim", "BackgroundHolder/Holder/Skateboard")
	m.skate = roleOr(as, "SkateboardAnim", "BackgroundHolder/Holder/Skateboard/Skateboard")
	m.eagleRoot = roleOr(as, "Eagle1Anim", "PlatformHolder/Eagle")
	m.eagle = roleOr(as, "EagleAnim", "PlatformHolder/Eagle/Eagle")
	m.smallT = kart.NewTemplate(as, roleOr(as, "SmallDemon", "demon"))
	m.mediumT = kart.NewTemplate(as, roleOr(as, "MediumDemon", "mediumDemon"))
	m.paths = loadCurvePaths(as)
	for _, p := range []string{m.bg, m.water, m.fg, m.fog, m.cloud} {
		m.basePos[p] = nodePos(as, p)
	}
	for _, p := range []string{"demon", "mediumDemon", "WaterParticle", m.skate, m.eagle} {
		ctx.Scene.SetActive(p, false)
	}
	m.resetScene(0)
	return nil
}

func (m *Module) resetScene(beat float64) {
	sec := m.ctx.SecPerBeat(math.Max(beat, 0))
	for _, p := range []string{m.samurai, m.samuraiHolder, m.skateRoot, m.skate, m.eagleRoot, m.eagle, m.bgAnim, m.cloud, m.flash} {
		m.ctx.Scene.PlayDefaultState(p, beat, sec)
	}
	m.ctx.Scene.SetActive(m.samuraiShadow, true)
	m.ctx.Scene.SetActive(m.skate, m.skateboardActive && !m.eagleActive)
	m.ctx.Scene.SetActive(m.eagle, m.eagleActive)
	m.ctx.Scene.SetActive(m.waterGO, m.waterActive)
	m.ctx.Scene.SetActive(m.fogGO, m.fogActive)
	m.ctx.Scene.PlayState(m.flash, "idle", beat, 0.5)
}

func (m *Module) OnEvent(e *riq.Entity) {
	switch e.Datamodel {
	case "superSamuraiSlice/bop":
		ev := bopEvt{beat: e.Beat, length: e.Length, bop: boolDefault(e, "bop", true), auto: boolParam(e, "auto")}
		m.bops = append(m.bops, ev)
	case "superSamuraiSlice/smallDemon":
		m.smallRaw = append(m.smallRaw, smallEvt{
			beat: e.Beat, auto: boolDefault(e, "auto", true), count: intDefault(e, "count", 1),
			autoBoom: boolDefault(e, "autoBoom", true), boomDelay: e.Float("boomBeat", 1),
			flipBack: e.Float("flipback", 1), enableBoom: boolDefault(e, "enableBoom", true),
		})
	case "superSamuraiSlice/bigDemon":
		m.bigs = append(m.bigs, bigPlan{
			beat: e.Beat, invert: boolParam(e, "invert"),
			boomDelay: e.Float("boomBeat", 1), enableBoom: boolDefault(e, "enableBoom", true),
		})
	case "superSamuraiSlice/scroll":
		ev := scrollEvt{
			beat: e.Beat, speed: e.Float("speed", 1), platform: intDefault(e, "platform", platformSkateboard),
			instant: boolParam(e, "instant"), off: boolParam(e, "off"), offset: e.Float("offset", 1), mute: boolParam(e, "mute"),
		}
		m.scrolls = append(m.scrolls, ev)
		m.ctx.At(e.Beat, func() { m.scroll(ev) })
	case "superSamuraiSlice/fogToggle":
		fog, water := boolParam(e, "fog"), boolDefault(e, "water", true)
		m.ctx.At(e.Beat, func() {
			m.fogActive, m.waterActive = fog, water
			m.ctx.Scene.SetActive(m.fogGO, fog)
			m.ctx.Scene.SetActive(m.waterGO, water)
		})
	}
}

func (m *Module) Ready() {
	sort.Slice(m.bops, func(i, j int) bool { return m.bops[i].beat < m.bops[j].beat })
	sort.Slice(m.smallRaw, func(i, j int) bool { return m.smallRaw[i].beat < m.smallRaw[j].beat })
	sort.Slice(m.bigs, func(i, j int) bool { return m.bigs[i].beat < m.bigs[j].beat })
	m.planSmallDemons()
	for _, ev := range m.bops {
		ev := ev
		m.ctx.At(ev.beat, func() { m.autoBop = ev.auto })
		if ev.bop {
			for i := 0; float64(i) < ev.length-1e-6; i++ {
				b := ev.beat + float64(i)
				m.ctx.At(b, func() { m.playSamurai("Beat", b) })
			}
		}
	}
	for _, p := range m.smallPlans {
		p := p
		m.ctx.At(p.beat, func() { m.spawnSmall(p) })
	}
	for _, p := range m.bigs {
		p := p
		m.ctx.At(p.beat, func() { m.spawnBig(p) })
	}
}

func (m *Module) OnSwitch(beat float64) {
	m.resetScene(beat)
	m.lastPulse = int(math.Floor(beat)) - 1
	m.demons = liveDemons(m.demons, beat)
	m.effects = liveEffects(m.effects, m.ctx.Time())
}

func (m *Module) Whiff(beat float64) { m.WhiffAction(beat, actionSlice) }

func (m *Module) WhiffAction(beat float64, action int) {
	if action != actionSlice || m.largeDemonActive {
		return
	}
	m.playSamurai("Slash00", beat)
	m.ctx.SoundPitchPan("SE_IAI_NEW_SWING1", 1, 1, randomPan(m.rng))
}

func (m *Module) Update(t, beat float64) {
	m.pulseBop(beat)
	if m.hasLastT {
		m.scrollX += -m.scrollSpeed * (t - m.lastT)
	}
	m.lastT, m.hasLastT = t, true
	for _, d := range m.demons {
		d.update(beat)
	}
	m.demons = liveDemons(m.demons, beat)
	m.effects = liveEffects(m.effects, t)
}

func (m *Module) Draw(screen *ebiten.Image, t, beat float64) {
	screen.Fill(color.RGBA{R: 0x43, G: 0x5e, B: 0x90, A: 0xff})
	m.applyScrollOffsets()
	m.ctx.SampleScene(beat)
	for _, d := range m.demons {
		d.inst.Queue(m.ctx.Scene, beat, kart.Identity(), 0)
	}
	m.ctx.Scene.Draw(screen, m.proj)
	for _, fx := range m.effects {
		m.drawEffect(screen, fx, t)
	}
}

func (m *Module) pulseBop(beat float64) {
	pulse := int(math.Floor(beat + 1e-6))
	if pulse == m.lastPulse || beat+1e-6 < float64(pulse) {
		return
	}
	m.lastPulse = pulse
	if !m.autoBop {
		return
	}
	state, playing := m.ctx.Scene.StateInfo(m.samurai, beat)
	if playing && state != "" && state != "idle" {
		return
	}
	m.playSamurai("Beat", float64(pulse))
}

func (m *Module) playSamurai(state string, beat float64) {
	m.ctx.Scene.PlayState(m.samurai, state, beat, 0.5)
}

func (m *Module) flipSamurai(beat float64, left bool) {
	m.ctx.At(beat, func() { m.ctx.Scene.SetMirrorX(m.samurai, left) })
}

func (m *Module) playPlatformGuard(beat float64) {
	if m.skateboardActive {
		if m.direction == 1 {
			m.ctx.Scene.PlayState(m.skate, "guard_FromL", beat, 0.5)
		} else {
			m.ctx.Scene.PlayState(m.skate, "guard_FromR", beat, 0.5)
		}
	}
	if m.eagleActive {
		if m.direction == 1 {
			m.ctx.Scene.PlayState(m.eagle, "fromL", beat, 0.5)
		} else {
			m.ctx.Scene.PlayState(m.eagle, "fromR", beat, 0.5)
		}
	}
}

func (m *Module) platformIdle(beat float64) {
	if m.skateboardActive {
		m.ctx.Scene.PlayState(m.skate, "idle", beat, 0.5)
	}
	if m.eagleActive {
		m.ctx.Scene.PlayState(m.eagle, "idle", beat, 0.5)
	}
}

func (m *Module) scroll(ev scrollEvt) {
	if ev.off && m.isOn {
		m.ctx.Scene.PlayState(m.cloud, "fade", ev.beat, 0.5)
		m.scrollSpeed = 0
		if m.eagleActive {
			m.ctx.Scene.PlayState(m.bgAnim, "down", ev.beat, 0.5)
		}
		m.ctx.At(ev.beat+ev.offset, func() {
			if m.skateboardActive && !m.eagleActive {
				m.ctx.Scene.PlayState(m.samuraiHolder, "exit", ev.beat+ev.offset, 0.5)
				m.ctx.Scene.PlayState(m.skateRoot, "exit", ev.beat+ev.offset, 0.5)
			}
		})
		m.ctx.At(ev.beat+ev.offset+3, func() {
			if m.eagleActive {
				m.ctx.Scene.PlayState(m.samuraiHolder, "idle", ev.beat+ev.offset+3, 0.5)
				m.ctx.Scene.PlayState(m.eagleRoot, "exit", ev.beat+ev.offset+3, 0.5)
			}
		})
		m.ctx.At(ev.beat+ev.offset+4, func() {
			m.skateboardActive, m.eagleActive, m.isOn = false, false, false
			m.ctx.Scene.SetActive(m.skate, false)
			m.ctx.Scene.SetActive(m.eagle, false)
			m.ctx.Scene.SetActive(m.samuraiShadow, true)
			m.ctx.Scene.PlayState(m.cloud, "idle", ev.beat+ev.offset+4, 0.5)
		})
		return
	}
	if ev.off {
		return
	}
	m.scrollSpeed = ev.speed
	m.ctx.Scene.SetActive(m.samuraiShadow, false)
	m.isOn = true
	if ev.instant {
		m.ctx.Scene.PlayState(m.samuraiHolder, "instant", ev.beat, 0.5)
	} else {
		m.ctx.Scene.PlayState(m.samuraiHolder, "enter", ev.beat, 0.5)
	}
	switch ev.platform {
	case platformEagle:
		m.enterEagle(ev)
	default:
		m.enterSkateboard(ev)
	}
}

func (m *Module) enterSkateboard(ev scrollEvt) {
	if m.eagleActive || m.skateboardActive {
		return
	}
	m.ctx.Scene.SetActive(m.skate, true)
	m.skateboardActive = true
	if ev.instant {
		m.ctx.Scene.PlayState(m.skate, "idle", ev.beat, 0.5)
		m.ctx.Scene.PlayState(m.skateRoot, "idle", ev.beat, 0.5)
	} else {
		m.ctx.Scene.PlayState(m.skateRoot, "enter", ev.beat, 0.5)
	}
}

func (m *Module) enterEagle(ev scrollEvt) {
	if m.eagleActive {
		return
	}
	m.ctx.Scene.PlayState(m.bgAnim, pick(ev.instant, "instant", "up"), ev.beat, 0.5)
	m.ctx.Scene.SetActive(m.eagle, true)
	if m.skateboardActive && !ev.instant {
		m.ctx.Scene.PlayState(m.samuraiHolder, "enter2", ev.beat, 0.5)
		m.ctx.Scene.PlayState(m.skateRoot, "exit", ev.beat, 0.5)
		m.ctx.At(ev.beat+1, func() { m.ctx.Scene.SetActive(m.skate, false) })
	} else {
		m.ctx.Scene.SetActive(m.skate, false)
		m.ctx.Scene.PlayState(m.samuraiHolder, pick(ev.instant, "instant", "enter"), ev.beat, 0.5)
	}
	m.ctx.Scene.PlayState(m.eagleRoot, pick(ev.instant, "idle", "enter"), ev.beat, 0.5)
	if !ev.mute {
		m.ctx.Sound("SE_IAI_NEW_BIRD_1")
	}
	// Unity keeps isSkateboardActive true after switching to the eagle; Demon.cs
	// later checks both flags for guard/idles, so preserve the source state model.
	m.skateboardActive, m.eagleActive = true, true
}

func (m *Module) applyScrollOffsets() {
	for _, spec := range []struct {
		path  string
		width float64
	}{
		{m.bg, 12.446}, {m.fg, 6.803}, {m.water, 6.43}, {m.fog, 12.94}, {m.cloud, 6.4},
	} {
		base := m.basePos[spec.path]
		m.ctx.Scene.SetPosOver(spec.path, base[0]+modOffset(m.scrollX, spec.width), base[1])
	}
}

func (m *Module) planSmallDemons() {
	m.smallPlans = nil
	for start := 0; start < len(m.smallRaw); {
		end := start + 1
		for end < len(m.smallRaw) && m.smallRaw[end].beat < m.smallRaw[start].beat+3 {
			end++
		}
		group := m.smallRaw[start:end]
		booms := boomBeatsForGroup(group)
		for i, ev := range group {
			count := ev.count
			if ev.auto {
				count = i + 1
			}
			boomAt := ev.beat + 2 + ev.boomDelay
			if ev.autoBoom && i < len(booms) {
				boomAt = booms[i]
			}
			m.smallPlans = append(m.smallPlans, smallPlan{
				beat: ev.beat, count: count, boomAt: boomAt, flipBack: ev.flipBack,
				enableBoom: ev.enableBoom, slashIdx: i,
			})
		}
		start = end
	}
}

func boomBeatsForGroup(group []smallEvt) []float64 {
	if len(group) == 0 || !group[0].autoBoom {
		return nil
	}
	input := group[0].beat + 2
	switch len(group) {
	case 1:
		return []float64{input + 1}
	case 2:
		if group[1].beat-group[0].beat < 1 {
			return []float64{input + 1, group[1].beat + 3}
		}
		return []float64{input + 2, group[1].beat + 4}
	case 3:
		return []float64{input + 2, input + 2.5, input + 3}
	default:
		return []float64{input + 2, input + 2.25, input + 2.5, input + 3}
	}
}

func liveDemons(in []*activeDemon, beat float64) []*activeDemon {
	out := in[:0]
	for _, d := range in {
		if beat <= d.deathBeat {
			out = append(out, d)
		}
	}
	return out
}

func liveEffects(in []effect, t float64) []effect {
	out := in[:0]
	for _, fx := range in {
		if t-fx.startT < superSliceEffectMaxLife {
			out = append(out, fx)
		}
	}
	return out
}

func pick[T any](cond bool, a, b T) T {
	if cond {
		return a
	}
	return b
}
