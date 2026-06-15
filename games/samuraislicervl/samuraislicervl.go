// Package samuraislicervl ports Rhythm Heaven Fever's Samurai Slice logic:
// demon summons, horde combo timing, smog/foreground toggles, lightning, and
// the Samurai animator state flow from Assets/Scripts/Games/SamuraiSliceRvl.
package samuraislicervl

import (
	"image/color"
	"math"
	"math/rand"
	"sort"

	"github.com/hajimehoshi/ebiten/v2"

	"hsdemo/engine"
	"hsdemo/kart"
	"hsdemo/kmdata"
	"hsdemo/riq"
)

const (
	demonSmall = iota
	demonMedium
	demonBig
	demonHuge
)

const (
	actionSlice = 0
	actionCombo = 3 // HS InputAction_Alt maps to pad South; engine channel 3.
)

type bopEvt struct {
	beat, length float64
	bop, auto    bool
}

type boolEvt struct {
	beat float64
	on   bool
}

type smogEvt struct {
	beat, length float64
	show         bool
	ease         int
}

type Module struct {
	ctx  *engine.Ctx
	proj kart.Aff
	rng  *rand.Rand

	samurai string
	fg      string

	demonT        *kart.Template
	hordeT        *kart.Template
	lightningT    *kart.Template
	flashT        *kart.Template
	smogParticleT *kart.Template
	slicedT       [4]*kart.Template
	hordeSlicedT  *kart.Template

	demonComp kmdata.Component
	gameComp  kmdata.Component
	curves    map[string]kmdata.Curve

	slicedCfg map[string]slicedConfig
	hordeCfg  hordeConfig
	smog      smogState

	hordeSpawnPositions [][2]float64

	bops     []bopEvt
	thunders []boolEvt
	fgs      []boolEvt
	smogs    []smogEvt

	demons  []*demon
	slices  []*slicedDemon
	hordes  []*hordeSequence
	flashes []*flashEffect

	autoBop            bool
	isReady            bool
	isHolding          bool
	isDemonSuccess     bool
	thunderEffect      bool
	nextPrepareBeat    int
	lastSuccessfulBeat float64
	lastDemonBeat      float64
	lastPulse          int
	stopRolling        func()
}

func New() engine.Module {
	return &Module{
		rng:                rand.New(rand.NewSource(0x515a1)),
		autoBop:            true,
		nextPrepareBeat:    1,
		lastSuccessfulBeat: math.Inf(-1),
		lastDemonBeat:      math.Inf(-1),
		lastPulse:          -1 << 30,
	}
}

func (m *Module) ID() string { return "samuraiSliceRvl" }

func (m *Module) Load(ctx *engine.Ctx) error {
	m.ctx = ctx
	if err := ctx.LoadAssets("samuraiSliceRvl"); err != nil {
		return err
	}
	m.proj = kart.Translate(engine.ScreenW/2, engine.ScreenH/2).Mul(kart.Scale(54, -54))
	m.gameComp = ctx.Assets.Extra.Components["game"]
	m.demonComp = ctx.Assets.Extra.Components["demon"]
	m.curves = ctx.Assets.Extra.Curves

	m.samurai = refOr(ctx, m.gameComp, "SamuraiAnim", "SteveHolder")
	m.fg = refOr(ctx, m.gameComp, "fgHolder", "FGHolder")

	m.demonT = kart.NewTemplate(ctx.Assets, refOr(ctx, m.gameComp, "demonholder", "DemonHolder"))
	m.hordeT = kart.NewTemplate(ctx.Assets, refOr(ctx, m.gameComp, "hordeDemonPrefab", "HordePrefab"))
	m.lightningT = kart.NewTemplate(ctx.Assets, refOr(ctx, m.gameComp, "flashholder", "LightningHolder"))
	m.flashT = kart.NewTemplate(ctx.Assets, "Flash")
	m.smogParticleT = kart.NewTemplate(ctx.Assets, "SmogParticlePrefab")
	m.hordeSlicedT = kart.NewTemplate(ctx.Assets, refOr(ctx, m.gameComp, "hordeSlicedPrefab", "SlicedHorde"))
	for i, root := range m.gameComp.RefArrays["slicedDemonPrefabs"] {
		if i < len(m.slicedT) {
			m.slicedT[i] = kart.NewTemplate(ctx.Assets, root)
		}
	}
	m.hordeSpawnPositions = vec2List(m.gameComp.Lists["hordeSpawnPositions"])
	m.slicedCfg = m.loadSlicedConfigs()
	m.hordeCfg = m.loadHordeConfig()
	m.smog = newSmogState(ctx, m.rng)

	// Authored prefab roots are inactive templates in Unity. Keep them out of
	// the scene and render runtime instances through kart.Template.Queue.
	for _, p := range []string{
		"DemonHolder", "DemonSliced", "SmallDemonSlicedPrefab", "MediumDemonSliced",
		"BigDemonSliced", "HugeDemonSliced", "Horde", "HordePrefab",
		"SlicedHorde", "LightningHolder", "Flash", "Smog", "SmogParticlePrefab",
	} {
		ctx.Scene.SetActive(p, false)
	}
	ctx.Scene.PlayDefaultState(m.samurai, 0, ctx.SecPerBeat(0))
	return nil
}

func (m *Module) OnEvent(e *riq.Entity) {
	switch e.Datamodel {
	case "samuraiSliceRvl/bop":
		ev := bopEvt{
			beat: e.Beat, length: e.Length,
			bop:  boolDefault(e, "bop", true),
			auto: boolParam(e, "auto"),
		}
		m.bops = append(m.bops, ev)
		m.ctx.At(e.Beat, func() { m.autoBop = ev.auto })
		if ev.bop {
			for i := 0; float64(i) < ev.length-1e-6; i++ {
				b := ev.beat + float64(i)
				m.ctx.At(b, func() { m.playSamurai("SamuraiBop", b, 1) })
			}
		}
	case "samuraiSliceRvl/demon":
		typ := intDefault(e, "type", demonSmall)
		prepareBeat := intDefault(e, "prepareBeat", 1)
		m.ctx.At(e.Beat, func() { m.spawnDemon(e.Beat, typ, prepareBeat) })
	case "samuraiSliceRvl/combodemon":
		m.ctx.At(e.Beat, func() { m.comboDemon(e.Beat) })
	case "samuraiSliceRvl/thunder":
		ev := boolEvt{beat: e.Beat, on: boolDefault(e, "thunder", true)}
		m.thunders = append(m.thunders, ev)
		m.ctx.At(e.Beat, func() { m.thunderEffect = ev.on })
	case "samuraiSliceRvl/smog control":
		ev := smogEvt{
			beat: e.Beat, length: e.Length,
			show: boolDefault(e, "type", true),
			ease: intDefault(e, "ease", 2),
		}
		m.smogs = append(m.smogs, ev)
		m.ctx.At(e.Beat, func() { m.startSmog(ev) })
	case "samuraiSliceRvl/toggleFG":
		ev := boolEvt{beat: e.Beat, on: boolParam(e, "show")}
		m.fgs = append(m.fgs, ev)
		m.ctx.At(e.Beat, func() { m.ctx.Scene.SetActive(m.fg, ev.on) })
	}
}

func (m *Module) Ready() {
	sort.SliceStable(m.bops, func(i, j int) bool { return m.bops[i].beat < m.bops[j].beat })
	sort.SliceStable(m.thunders, func(i, j int) bool { return m.thunders[i].beat < m.thunders[j].beat })
	sort.SliceStable(m.fgs, func(i, j int) bool { return m.fgs[i].beat < m.fgs[j].beat })
	sort.SliceStable(m.smogs, func(i, j int) bool { return m.smogs[i].beat < m.smogs[j].beat })
}

func (m *Module) OnSwitch(beat float64) {
	m.demons = liveDemons(m.demons, beat)
	m.slices = liveSlices(m.slices, beat)
	m.hordes = liveHordes(m.hordes, beat)
	m.flashes = liveFlashes(m.flashes, beat)
	m.isReady = false
	m.isHolding = false
	m.isDemonSuccess = false
	m.lastSuccessfulBeat = math.Inf(-1)
	m.lastDemonBeat = math.Inf(-1)
	m.autoBop = true
	m.thunderEffect = false
	for _, ev := range m.bops {
		if ev.beat > beat {
			break
		}
		m.autoBop = ev.auto
	}
	for _, ev := range m.thunders {
		if ev.beat > beat {
			break
		}
		m.thunderEffect = ev.on
	}
	for _, ev := range m.fgs {
		if ev.beat > beat {
			break
		}
		m.ctx.Scene.SetActive(m.fg, ev.on)
	}
	m.smog.applyAt(m.smogs, beat)
	m.ctx.Scene.PlayDefaultState(m.samurai, beat, m.ctx.SecPerBeat(beat))
}

func (m *Module) Whiff(beat float64) { m.WhiffAction(beat, actionSlice) }

func (m *Module) WhiffAction(beat float64, action int) {
	switch action {
	case actionSlice:
		m.doSlice(beat)
	case actionCombo:
		m.sliceCombo(beat)
	}
}

func (m *Module) Update(t, beat float64) {
	m.pulseBop(beat)
	m.smog.update(t, beat)
	for _, d := range m.demons {
		d.update(m, beat)
	}
	for _, s := range m.slices {
		s.update(beat)
	}
	for _, h := range m.hordes {
		h.update(m, beat)
	}
	for _, f := range m.flashes {
		f.update(beat)
	}
	m.demons = liveDemons(m.demons, beat)
	m.slices = liveSlices(m.slices, beat)
	m.hordes = liveHordes(m.hordes, beat)
	m.flashes = liveFlashes(m.flashes, beat)
}

func (m *Module) Draw(screen *ebiten.Image, _ float64, beat float64) {
	screen.Fill(color.NRGBA{0x59, 0x59, 0x39, 0xff})
	m.ctx.SampleScene(beat)
	m.smog.queue(m.ctx.Scene, beat)
	for _, d := range m.demons {
		d.queue(m.ctx.Scene, beat)
	}
	for _, h := range m.hordes {
		h.queue(m.ctx.Scene, beat)
	}
	for _, s := range m.slices {
		s.queue(m.ctx.Scene, beat)
	}
	for _, f := range m.flashes {
		f.queue(m.ctx.Scene, beat)
	}
	m.ctx.Scene.Draw(screen, m.proj)
}

func (m *Module) pulseBop(beat float64) {
	pulse := int(math.Floor(beat + 1e-6))
	if pulse == m.lastPulse || beat+1e-6 < float64(pulse) {
		return
	}
	m.lastPulse = pulse
	if m.isReady {
		return
	}
	state, playing := m.ctx.Scene.StateInfo(m.samurai, beat)
	if playing && state != "" && state != "SamuraiIdle" {
		return
	}
	if m.autoBop {
		m.playSamurai("SamuraiBop", float64(pulse), 1)
	} else {
		m.playSamurai("SamuraiIdle", float64(pulse), 1)
	}
}

func (m *Module) playSamurai(state string, beat, speed float64) {
	m.ctx.Scene.PlayState(m.samurai, state, beat, m.ctx.SecPerBeat(beat)*speed)
}

func (m *Module) startSmog(ev smogEvt) {
	m.smog.animate(ev.beat, ev.length, ev.show, ev.ease)
	if ev.show && ev.ease != 1 {
		m.ctx.Sound("CLOUD_PRERENDER")
	}
}

func (m *Module) randomThunder() string {
	r := m.rng.Float64()
	switch {
	case r > 0.66:
		return "THUNDER1"
	case r > 0.33:
		return "THUNDER2"
	default:
		return "THUNDER3"
	}
}
