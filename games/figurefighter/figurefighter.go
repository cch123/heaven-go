// Package figurefighter ports Figure Fighter's cue timing, crowd engagement,
// black bars, bag break, whiff, and shoulder-fart animation rules from
// Assets/Scripts/Games/FigureFighter/FigureFighter.cs.
package figurefighter

import (
	"fmt"
	"image/color"
	"math"
	"math/rand"
	"sort"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"hsdemo/engine"
	"hsdemo/kart"
	"hsdemo/riq"
)

const (
	crowdNeutral = iota
	crowdHalfRiled
	crowdFullyRiled
)

var stageColor = color.RGBA{0x5d, 0xe1, 0xfb, 0xff}

type bopEvt struct {
	beat, length            float64
	dollManual, dollPulse   bool
	crowdManual, crowdPulse bool
}

type engagementEvt struct {
	beat       float64
	engagement int
	chant      bool
}

type barsEvt struct {
	beat    float64
	enabled bool
	sticky  bool
}

type chainParticle struct {
	x, y   float64
	vx, vy float64
	size   float64
	line   float64
	angle  float64
}

type chainBurst struct {
	beat      float64
	role      string
	particles []chainParticle
}

type Module struct {
	ctx  *engine.Ctx
	proj kart.Aff

	doll       string
	crowd      string
	bag        string
	bagCopy    string
	button     string
	lights     string
	top        string
	bars       string
	fart       string
	bagObject  string
	chain1     string
	chain2     string
	stickyRoot string

	bops        []bopEvt
	engagements []engagementEvt
	barsEvents  []barsEvt

	canBop          bool
	isPreparing     bool
	dollBop         bool
	crowdBop        bool
	toggleBars      bool
	stickyBars      bool
	crowdChant      bool
	crowdEngagement int
	strongPunch     bool
	lastPulse       int

	rng    *rand.Rand
	bursts []chainBurst
}

func New() engine.Module {
	return &Module{
		canBop: true, dollBop: true, crowdBop: true, toggleBars: true,
		crowdChant: true, lastPulse: -1 << 30,
		rng: rand.New(rand.NewSource(0x4649474854)),
	}
}

func (m *Module) ID() string { return "figureFighter" }

func (m *Module) Load(ctx *engine.Ctx) error {
	m.ctx = ctx
	if err := ctx.LoadAssets("figureFighter"); err != nil {
		return err
	}
	m.proj = kart.Translate(engine.ScreenW/2, engine.ScreenH/2).Mul(kart.Scale(54, -54))
	m.doll = roleOr(ctx, "dollAnim", "Doll")
	m.crowd = roleOr(ctx, "crowdAnim", "Background/Crowds")
	m.bag = roleOr(ctx, "bagAnim", "Bag")
	m.bagCopy = roleOr(ctx, "bagCopyAnim", "BagCopy")
	m.button = roleOr(ctx, "buttonAnim", "Button")
	m.lights = roleOr(ctx, "lightsAnim", "Background/Lights")
	m.top = roleOr(ctx, "topAnim", "TopsSpots")
	m.bars = roleOr(ctx, "barsAnim", "StickyBars/Bars")
	m.fart = roleOr(ctx, "fartAnim", "Fart")
	m.bagObject = roleOr(ctx, "bagObject", m.bag)
	m.chain1 = roleOr(ctx, "chainParticles1", "ChainParticle/Chain")
	m.chain2 = roleOr(ctx, "chainParticles2", "ChainParticle/Chain (1)")
	m.stickyRoot = componentRef(ctx, "StickyLayer", "StickyBars")
	m.hideBars()
	return nil
}

func roleOr(ctx *engine.Ctx, key, fallback string) string {
	if p := ctx.Role(key); p != "" {
		return p
	}
	return fallback
}

func componentRef(ctx *engine.Ctx, key, fallback string) string {
	if c, ok := ctx.Assets.Extra.Components["game"]; ok {
		if p := c.Refs[key]; p != "" {
			return p
		}
	}
	return fallback
}

func (m *Module) OnEvent(e *riq.Entity) {
	b := e.Beat
	switch e.Datamodel {
	case "figureFighter/bop":
		ev := figureBopEvt(e)
		m.bops = append(m.bops, ev)
		m.ctx.At(b, func() {
			m.dollBop = ev.dollPulse
			m.crowdBop = ev.crowdPulse
		})
		if ev.dollManual || ev.crowdManual {
			for i := 0; float64(i) < ev.length-1e-6; i++ {
				bb := b + float64(i)
				m.ctx.At(bb, func() { m.bop(bb, ev.dollManual, ev.crowdManual) })
			}
		}
	case "figureFighter/crowdEngagement":
		ev := engagementEvt{
			beat:       b,
			engagement: clampInt(intParam(e, "engagement", crowdNeutral), crowdNeutral, crowdFullyRiled),
			chant:      boolDefault(e, "chant", true),
		}
		m.engagements = append(m.engagements, ev)
		m.ctx.At(b, func() { m.setEngagement(ev) })
	case "figureFighter/bars":
		ev := barsEvt{beat: b, enabled: boolDefault(e, "bars", true), sticky: boolParam(e, "sticky")}
		m.barsEvents = append(m.barsEvents, ev)
		m.ctx.At(b, func() {
			m.toggleBars = ev.enabled
			m.stickyBars = ev.sticky
		})
	case "figureFighter/jab":
		and, strong := boolParam(e, "and"), boolParam(e, "strong")
		if and {
			m.ctx.SoundAt(b-0.5, "and", 1)
		}
		m.ctx.At(b, func() { m.jab(b, strong) })
	case "figureFighter/oneTwo":
		and, strong := boolParam(e, "and"), boolParam(e, "strong")
		if and {
			m.ctx.SoundAt(b-0.5, "and", 1)
		}
		m.ctx.At(b, func() { m.oneTwo(b, strong) })
	case "figureFighter/oneTwoFast":
		and, strong := boolParam(e, "and"), boolParam(e, "strong")
		if and {
			m.ctx.SoundAt(b-0.5, "and2", 1)
		}
		m.ctx.At(b, func() { m.oneTwoFast(b, strong) })
	case "figureFighter/goGoGo":
		and, strong := boolParam(e, "and"), boolParam(e, "strong")
		if and {
			m.ctx.SoundAt(b-0.5, "and2", 1)
		}
		m.ctx.At(b, func() { m.goGoGo(b, strong) })
	}
}

func (m *Module) Ready() {
	sort.SliceStable(m.bops, func(i, j int) bool { return m.bops[i].beat < m.bops[j].beat })
	sort.SliceStable(m.engagements, func(i, j int) bool { return m.engagements[i].beat < m.engagements[j].beat })
	sort.SliceStable(m.barsEvents, func(i, j int) bool { return m.barsEvents[i].beat < m.barsEvents[j].beat })
}

func (m *Module) OnSwitch(beat float64) {
	sec := m.ctx.SecPerBeat(beat)
	for path := range m.ctx.Assets.Animators {
		m.ctx.Scene.PlayDefaultState(path, beat, sec)
	}
	m.canBop = true
	m.isPreparing = false
	m.strongPunch = false
	m.restoreEngagement(beat)
	m.restoreBopFlags(beat)
	m.restoreBars(beat)
	m.hideBars()
	m.ctx.Scene.SetActive(m.bagObject, true)
	m.bursts = nil
	m.lastPulse = int(math.Floor(beat))
}

func (m *Module) Whiff(beat float64) {
	m.playState(m.doll, "FigureDeflate", beat)
	m.playState(m.button, "ButtonPress", beat)
	m.ctx.SoundPitch("whiffFart", 1, 0.9+m.rng.Float64()*0.2)
	m.ctx.Sound("whiffPress")
	m.ctx.SoundVol("pump", 0.4)
	m.fartState(-1)
}

func (m *Module) Update(_, beat float64) {
	pulse := int(math.Floor(beat + 1e-6))
	if pulse != m.lastPulse {
		m.lastPulse = pulse
		b := float64(pulse)
		m.applyEngagementPulse(b)
		if m.autoBopAt(b) {
			m.bop(b, m.dollBop, m.crowdBop)
		}
	}
	m.bursts = liveBursts(m.bursts, beat)
}

func (m *Module) Draw(screen *ebiten.Image, _, beat float64) {
	screen.Fill(stageColor)
	m.applyStickyBars(beat)
	m.ctx.SampleScene(beat)
	m.ctx.Scene.Draw(screen, m.proj)
	m.drawChainBursts(screen, beat)
}

func figureBopEvt(e *riq.Entity) bopEvt {
	length := e.Length
	if length <= 0 {
		length = 1
	}
	return bopEvt{
		beat:        e.Beat,
		length:      length,
		dollManual:  boolParam(e, "auto"),
		dollPulse:   boolDefault(e, "bop", true),
		crowdManual: boolDefault(e, "crowd", true),
		crowdPulse:  boolParam(e, "crowdAuto"),
	}
}

func (m *Module) setEngagement(ev engagementEvt) {
	m.crowdEngagement = ev.engagement
	m.crowdChant = ev.chant
}

func (m *Module) applyEngagementPulse(beat float64) {
	for _, ev := range m.engagements {
		if math.Abs(ev.beat-beat) < 1e-6 {
			m.setEngagement(ev)
		}
	}
}

func (m *Module) restoreEngagement(beat float64) {
	m.crowdEngagement = crowdNeutral
	m.crowdChant = true
	for _, ev := range m.engagements {
		if ev.beat >= beat {
			break
		}
		m.setEngagement(ev)
	}
}

func (m *Module) restoreBopFlags(beat float64) {
	m.dollBop = true
	m.crowdBop = true
	for _, ev := range m.bops {
		if ev.beat > beat {
			break
		}
		m.dollBop = ev.dollPulse
		m.crowdBop = ev.crowdPulse
	}
}

func (m *Module) restoreBars(beat float64) {
	m.toggleBars = true
	m.stickyBars = false
	for _, ev := range m.barsEvents {
		if ev.beat > beat {
			break
		}
		m.toggleBars = ev.enabled
		m.stickyBars = ev.sticky
	}
}

func (m *Module) autoBopAt(beat float64) bool {
	on := false
	for _, ev := range m.bops {
		if ev.beat > beat {
			break
		}
		on = ev.dollManual
	}
	return on
}

func (m *Module) bop(beat float64, doll, crowd bool) {
	if doll && m.canBop && !m.playingPriority(beat) {
		m.playState(m.doll, "FigureBop", beat)
	}
	if crowd {
		m.playState(m.crowd, crowdBopState(m.crowdEngagement), beat)
	}
}

func crowdBopState(engagement int) string {
	return fmt.Sprintf("CrowdBop%d", clampInt(engagement, crowdNeutral, crowdFullyRiled))
}

func (m *Module) jab(beat float64, strong bool) {
	m.canBop = false
	m.isPreparing = true
	m.strongPunch = strong
	m.ctx.SoundAt(beat, "jab", 1)
	if m.crowdEngagement == crowdFullyRiled && m.crowdChant {
		m.ctx.SoundAt(beat+1, "crowdJab", 1)
	}
	m.ctx.ScheduleInput(beat+1, func(state float64, _ engine.Judgment) {
		m.justJab(beat+1, state)
	}, func() {
		m.missJab(beat + 1)
	})
	m.ctx.At(beat+1, func() { m.isPreparing = false })
	m.ctx.At(beat+1.75, func() {
		if !m.isPreparing {
			m.canBop = true
		}
	})
}

func (m *Module) oneTwo(beat float64, strong bool) {
	m.strongPunch = strong
	m.ctx.SoundAt(beat, "oneTwo1", 1)
	m.ctx.SoundAt(beat+1, "oneTwo2", 1)
	if m.crowdEngagement == crowdFullyRiled && m.crowdChant {
		m.ctx.SoundAt(beat+2, "crowdOne", 1)
		m.ctx.SoundAt(beat+3, "crowdTwo", 1)
	}
	m.ctx.ScheduleInput(beat+2, func(state float64, _ engine.Judgment) {
		m.justOneTwoFirst(beat+2, state)
	}, func() {
		m.missOneTwoFirst(beat + 2)
	})
	m.ctx.ScheduleInput(beat+3, func(state float64, _ engine.Judgment) {
		m.justOneTwoSecond(beat+3, state)
	}, func() {
		m.missOneTwoSecond(beat + 3)
	})
	m.canBop = false
	m.isPreparing = true
	m.playPrepWhenFree(beat)
	if m.toggleBars {
		m.showBars()
		m.playState(m.bars, "CloseIn1", beat)
	}
	m.ctx.At(beat+1, func() {
		if m.toggleBars {
			m.playState(m.bars, "CloseIn2", beat+1)
		}
	})
	m.ctx.At(beat+2, func() { m.isPreparing = false })
	m.ctx.At(beat+3.5, func() {
		if !m.isPreparing {
			m.canBop = true
		}
	})
}

func (m *Module) oneTwoFast(beat float64, strong bool) {
	m.strongPunch = strong
	m.ctx.SoundAt(beat, "fastOneTwo1", 1)
	m.ctx.SoundAt(beat+0.5, "fastOneTwo2", 1)
	if m.crowdEngagement == crowdFullyRiled && m.crowdChant {
		m.ctx.SoundAt(beat+1, "crowdOneFast", 1)
		m.ctx.SoundAt(beat+1.5, "crowdTwoFast", 1)
	}
	m.ctx.ScheduleInput(beat+1, func(state float64, _ engine.Judgment) {
		m.justOneTwoFirst(beat+1, state)
	}, func() {
		m.missOneTwoFirst(beat + 1)
	})
	m.ctx.ScheduleInput(beat+1.5, func(state float64, _ engine.Judgment) {
		m.justOneTwoSecond(beat+1.5, state)
	}, func() {
		m.missOneTwoSecond(beat + 1.5)
	})
	m.canBop = false
	m.ctx.At(beat+2, func() {
		if !m.isPreparing {
			m.canBop = true
		}
	})
}

func (m *Module) goGoGo(beat float64, strong bool) {
	m.strongPunch = strong
	m.ctx.SoundAt(beat, "go1", 1)
	m.ctx.SoundAt(beat+0.5, "go2", 1)
	m.ctx.SoundAt(beat+1, "go3", 1)
	if m.crowdEngagement == crowdFullyRiled && m.crowdChant {
		m.ctx.SoundAt(beat+2, "crowdGo1", 1)
		m.ctx.SoundAt(beat+2.5, "crowdGo2", 1)
		m.ctx.SoundAt(beat+3, "crowdGo3", 1)
	}
	m.ctx.ScheduleInput(beat+2, func(state float64, _ engine.Judgment) {
		m.justOneTwoFirst(beat+2, state)
	}, func() {
		m.missOneTwoFirst(beat + 2)
	})
	m.ctx.ScheduleInput(beat+2.5, func(state float64, _ engine.Judgment) {
		m.justOneTwoFirst(beat+2.5, state)
	}, func() {
		m.missOneTwoFirst(beat + 2.5)
	})
	m.ctx.ScheduleInput(beat+3, func(state float64, _ engine.Judgment) {
		m.justGo(beat+3, state)
	}, func() {
		m.missGo(beat + 3)
	})
	m.canBop = false
	m.isPreparing = true
	m.playPrepWhenFree(beat)
	if m.toggleBars {
		m.showBars()
		m.playState(m.bars, "CloseIn1", beat)
	}
	m.ctx.At(beat+0.5, func() {
		if m.toggleBars {
			m.playState(m.bars, "CloseIn2", beat+0.5)
		}
	})
	m.ctx.At(beat+1, func() {
		if m.toggleBars {
			m.playState(m.bars, "CloseIn3", beat+1)
		}
	})
	m.ctx.At(beat+2, func() { m.isPreparing = false })
	m.ctx.At(beat+3.5, func() {
		if !m.isPreparing {
			m.canBop = true
		}
	})
}

func (m *Module) justJab(beat, state float64) {
	m.hideBars()
	if isNG(state) {
		m.ctx.Sound("barely")
		m.playState(m.doll, "FigureBarely1", beat)
		m.playState(m.button, "ButtonPress", beat)
		m.playState(m.bag, "BagBarely", beat)
		m.fartState(state)
		return
	}
	if m.strongPunch {
		m.strongHit(beat, "jabHit", m.chain1)
	} else {
		m.ctx.Sound("jabHit")
		m.ctx.SoundVol("pump", 0.4)
		m.playState(m.doll, "FigureJab", beat)
		m.playState(m.button, "ButtonPress", beat)
		m.playState(m.bag, "BagHit1", beat)
	}
	m.fartState(state)
}

func (m *Module) missJab(beat float64) {
	m.hideBars()
	m.playState(m.doll, "FigureWhiff1", beat)
	m.playState(m.bag, "BagThrough", beat)
	m.ctx.Sound("failHit")
}

func (m *Module) justOneTwoFirst(beat, state float64) {
	m.hideBars()
	if isNG(state) {
		m.playState(m.button, "ButtonPress", beat)
		m.playState(m.doll, "FigureBarely2", beat)
		m.ctx.Sound("barely")
		m.playState(m.bag, "BagBarely", beat)
		m.fartState(state)
		return
	}
	m.ctx.Sound("oneTwoHit1")
	m.ctx.SoundVol("pump", 0.4)
	m.playState(m.doll, "FigureJab", beat)
	m.playState(m.button, "ButtonPress", beat)
	m.playState(m.bag, "BagHit1", beat)
	m.fartState(state)
}

func (m *Module) justOneTwoSecond(beat, state float64) {
	if isNG(state) {
		m.playState(m.button, "ButtonPress", beat)
		m.playState(m.doll, "FigureBarely3", beat)
		m.ctx.Sound("barely")
		m.playState(m.bag, "BagBarely", beat)
		m.fartState(state)
		return
	}
	if m.strongPunch {
		m.strongHit(beat, "oneTwoHit2", m.chain1)
	} else {
		m.ctx.Sound("oneTwoHit2")
		m.ctx.SoundVol("pump", 0.4)
		m.playState(m.doll, "FigureJab2", beat)
		m.playState(m.button, "ButtonPress", beat)
		m.playState(m.bag, "BagHit2", beat)
	}
	m.fartState(state)
}

func (m *Module) missOneTwoFirst(beat float64) {
	m.hideBars()
	m.playState(m.doll, "FigureWhiff1", beat)
	m.playState(m.bag, "BagThrough", beat)
	m.ctx.Sound("failHit")
}

func (m *Module) missOneTwoSecond(beat float64) {
	m.hideBars()
	m.playState(m.doll, "FigureWhiff2", beat)
	m.playState(m.bag, "BagThrough", beat)
	m.ctx.Sound("failHit")
}

func (m *Module) justGo(beat, state float64) {
	if isNG(state) {
		m.ctx.Sound("barely")
		m.playState(m.doll, "FigureBarely3", beat)
		m.playState(m.button, "ButtonPress", beat)
		m.playState(m.bag, "BagBarely", beat)
		m.fartState(state)
		return
	}
	if m.strongPunch {
		m.strongHit(beat, "oneTwoHit2", m.chain2)
	} else {
		m.ctx.Sound("goLastHit")
		m.ctx.SoundVol("pump", 0.4)
		m.playState(m.doll, "FigureJab2", beat)
		m.playState(m.button, "ButtonPress", beat)
		m.playState(m.bag, "BagHit2", beat)
	}
	m.fartState(state)
}

func (m *Module) missGo(beat float64) {
	m.playState(m.doll, "FigureWhiff2", beat)
	m.playState(m.bag, "BagThrough", beat)
	m.ctx.Sound("failHit")
}

func (m *Module) strongHit(beat float64, hitSound string, chainRole string) {
	m.strongPunch = false
	m.ctx.Sound(hitSound)
	m.ctx.Sound(m.breakSound())
	m.ctx.SoundVol("pump", 0.4)
	m.playState(m.doll, "FigureFinisher", beat)
	m.playState(m.button, "ButtonPress2", beat)
	// chainParticles1/2 are Unity ParticleSystems. The extractor preserves the
	// serialized role anchors, and this burst recreates their short chain-snap
	// flash in game space so the strong-hit effect is not silently dropped.
	m.spawnChainBurst(beat, chainRole)
	m.ctx.Scene.SetActive(m.bagObject, false)
	m.playState(m.bagCopy, "Blow", beat)
	m.ctx.At(beat+1, func() {
		m.ctx.Scene.SetActive(m.bagObject, true)
		m.playState(m.bag, "BagSummon", beat+1)
	})
}

func (m *Module) breakSound() string {
	if m.rng == nil {
		m.rng = rand.New(rand.NewSource(0x4649474854))
	}
	return fmt.Sprintf("break%d", 1+m.rng.Intn(8))
}

func (m *Module) playPrepWhenFree(beat float64) {
	if !m.playingPriority(beat) {
		m.playState(m.doll, "FigurePrep1", beat)
		return
	}
	m.ctx.At(beat+0.5, func() {
		if m.isPreparing {
			m.playState(m.doll, "FigurePrep1", beat+0.5)
		}
	})
}

func (m *Module) playingPriority(beat float64) bool {
	state, playing := m.ctx.Scene.StateInfo(m.doll, beat)
	if !playing {
		return false
	}
	switch state {
	case "FigureJab", "FigureJab1", "FigureJab2", "FigureFinisher",
		"FigureWhiff1", "FigureWhiff2", "FigureDeflate", "FigureDeflateBarely":
		return true
	}
	return false
}

func (m *Module) playState(path, state string, beat float64) {
	if path == "" || state == "" {
		return
	}
	m.ctx.Scene.PlayState(path, state, beat, 0.5)
}

func (m *Module) showBars() {
	if m.bars != "" {
		m.ctx.Scene.SetActive(m.bars, true)
	}
}

func (m *Module) hideBars() {
	if m.bars != "" {
		m.ctx.Scene.SetActive(m.bars, false)
	}
}

func (m *Module) applyStickyBars(beat float64) {
	if m.stickyRoot == "" {
		return
	}
	if !m.stickyBars {
		m.ctx.Scene.SetPosOver(m.stickyRoot, 0, 0)
		return
	}
	cam := m.ctx.CameraAt(beat)
	m.ctx.Scene.SetPosOver(m.stickyRoot, cam[0], cam[1])
}

func (m *Module) fartState(state float64) {
	if !shouldersHeld() {
		return
	}
	if state < 0.2 && state > -0.2 {
		m.playState(m.fart, "gas_L", m.ctx.Beat())
		return
	}
	m.playState(m.fart, "gas_S", m.ctx.Beat())
}

func shouldersHeld() bool {
	left := ebiten.IsKeyPressed(ebiten.KeyF) ||
		ebiten.IsKeyPressed(ebiten.KeyLeft) ||
		ebiten.IsKeyPressed(ebiten.KeyUp)
	right := ebiten.IsKeyPressed(ebiten.KeyK) ||
		ebiten.IsKeyPressed(ebiten.KeyRight) ||
		ebiten.IsKeyPressed(ebiten.KeyDown)
	return left && right
}

func isNG(state float64) bool {
	return state >= 1 || state <= -1
}

func (m *Module) spawnChainBurst(beat float64, role string) {
	const n = 16
	parts := make([]chainParticle, 0, n)
	for i := 0; i < n; i++ {
		a := float64(i)/n*math.Pi*2 + (m.rng.Float64()-0.5)*0.35
		speed := 1.2 + m.rng.Float64()*1.6
		parts = append(parts, chainParticle{
			vx: math.Cos(a) * speed, vy: math.Sin(a) * speed,
			size:  0.045 + m.rng.Float64()*0.055,
			line:  0.18 + m.rng.Float64()*0.24,
			angle: a,
		})
	}
	m.bursts = append(m.bursts, chainBurst{beat: beat, role: role, particles: parts})
}

func (m *Module) drawChainBursts(screen *ebiten.Image, beat float64) {
	for _, b := range m.bursts {
		u := (beat - b.beat) / 0.45
		if u < 0 || u >= 1 {
			continue
		}
		world, ok := m.ctx.Scene.NodeWorld(b.role)
		if !ok {
			continue
		}
		alpha := uint8(255 * (1 - u))
		for _, p := range b.particles {
			x := p.x + p.vx*u
			y := p.y + p.vy*u - 0.25*u*u
			sx, sy := m.proj.Mul(world).Apply(x, y)
			r := float32(p.size * 54 * (1 - 0.25*u))
			vector.DrawFilledCircle(screen, float32(sx), float32(sy), r,
				color.NRGBA{R: 255, G: 238, B: 118, A: alpha}, true)
			ex, ey := m.proj.Mul(world).Apply(x+math.Cos(p.angle)*p.line*(1-u), y+math.Sin(p.angle)*p.line*(1-u))
			vector.StrokeLine(screen, float32(sx), float32(sy), float32(ex), float32(ey), 2,
				color.NRGBA{R: 255, G: 255, B: 255, A: alpha}, true)
		}
	}
}

func liveBursts(in []chainBurst, beat float64) []chainBurst {
	out := in[:0]
	for _, b := range in {
		if beat < b.beat+0.5 {
			out = append(out, b)
		}
	}
	return out
}

func boolParam(e *riq.Entity, key string) bool {
	return e.Float(key, 0) != 0
}

func boolDefault(e *riq.Entity, key string, def bool) bool {
	if e.Data != nil {
		if _, ok := e.Data[key]; ok {
			return boolParam(e, key)
		}
	}
	return def
}

func intParam(e *riq.Entity, key string, def int) int {
	return int(e.Float(key, float64(def)))
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func supportedFloatAttr(attr string) bool {
	switch attr {
	case "m_IsActive", "m_Enabled", "m_FlipX", "m_FlipY", "m_SortingOrder", "m_Size.x", "m_Size.y":
		return true
	}
	return strings.HasPrefix(attr, "m_Color.") || strings.HasPrefix(attr, "m_fontColor.")
}
