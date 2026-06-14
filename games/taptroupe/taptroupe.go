// Package taptroupe ports Tap Troupe's stepping/tapping call-and-response,
// corner reactions, OK/party-popper endings, spotlights, and zoom-out camera.
package taptroupe

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

const (
	tapBam = iota
	tapTap
	tapBamReady
	tapBamTapReady
	tapLast
)

const (
	okayA = iota
	okayB
	okayC
	okayRandom
)

const (
	okAnimNormal = iota
	okAnimPopper
	okAnimSign
	okAnimRandom
)

type bopEvt struct {
	beat, length float64
	bop, auto    bool
}

type stepEvt struct {
	beat, length float64
	startTap     bool
}

type tapEvt struct {
	beat, length    float64
	okay            bool
	okayType        int
	animType        int
	popperBeats     float64
	randomVoiceLine bool
	noReady         bool
}

type spotEvt struct {
	beat                                    float64
	on, player, midLeft, midRight, leftMost bool
}

type zoomEvt struct {
	beat, length float64
	ease         int
}

type tapper struct {
	root           string
	mirror         bool
	dontSwitchNext bool
}

type corner struct {
	root, body, expr, popper string
}

type confetti struct {
	x, y   float64
	vx, vy float64
	life   float64
	col    color.NRGBA
}

type popperBurst struct {
	beat  float64
	path  string
	drops []confetti
}

type Module struct {
	ctx  *engine.Ctx
	proj kart.Aff

	playerTapper *tapper
	npcTappers   []*tapper
	playerCorner *corner
	npcCorners   []*corner

	bops      []bopEvt
	steps     []stepEvt
	taps      []tapEvt
	spots     []spotEvt
	zooms     []zoomEvt
	missFaces []struct {
		beat float64
		on   bool
	}

	goBop      bool
	lastPulse  int
	prepareTap bool
	tapping    bool
	stepping   bool
	missedTaps bool
	canSpit    bool

	currentTapAnim  int
	shouldSwitch    bool
	useTutorialFace bool
	stepSound       int

	zoomActive bool
	zoomStart  float64
	zoomLength float64
	zoomEase   int
	zoomY      float64
	zoomZ      float64

	poppers []popperBurst
}

func New() engine.Module { return &Module{lastPulse: -1, canSpit: true, stepSound: 1} }

func (m *Module) ID() string { return "tapTroupe" }

func (m *Module) Load(ctx *engine.Ctx) error {
	m.ctx = ctx
	if err := ctx.LoadAssets("tapTroupe"); err != nil {
		return err
	}
	m.proj = kart.Translate(engine.ScreenW/2, engine.ScreenH/2).Mul(kart.Scale(54, -54))
	m.playerTapper = &tapper{root: "Legs/TapperPlayer"}
	m.npcTappers = []*tapper{
		{root: "Legs/Tapper"},
		{root: "Legs/Tapper (1)"},
		{root: "Legs/Tapper (2)"},
	}
	m.playerCorner = newCorner("UnderForegroundElementsHolder/CornerTappers/CornerTapperPlayer")
	m.npcCorners = []*corner{
		newCorner("UnderForegroundElementsHolder/CornerTappers/CornerTapper"),
		newCorner("UnderForegroundElementsHolder/CornerTappers/CornerTapper (1)"),
		newCorner("UnderForegroundElementsHolder/CornerTappers/CornerTapper (2)"),
	}
	m.initScene(0)
	return nil
}

func newCorner(root string) *corner {
	body := root + "/Parts/Body"
	return &corner{
		root: root, body: body, expr: body + "/HeadHolder/Head",
		popper: body + "/PartyPopper",
	}
}

func (m *Module) initScene(beat float64) {
	sec := m.ctx.SecPerBeat(math.Max(beat, 0))
	m.ctx.Scene.PlayDefaultState("", beat, sec)
	for _, t := range m.allTappers() {
		m.ctx.Scene.PlayDefaultState(t.root, beat, sec)
		t.mirror, t.dontSwitchNext = false, false
		m.ctx.Scene.SetMirrorX(t.root, false)
	}
	for _, c := range m.allCorners() {
		m.ctx.Scene.PlayDefaultState(c.root, beat, sec)
		m.ctx.Scene.PlayDefaultState(c.body, beat, sec)
		m.ctx.Scene.PlayDefaultState(c.expr, beat, sec)
	}
	m.setSpotlights(false, false, false, false, false)
	m.prepareTap, m.tapping, m.stepping, m.missedTaps = false, false, false, false
	m.canSpit, m.stepSound, m.goBop = true, 1, false
	m.zoomActive, m.zoomY, m.zoomZ = false, 0, 0
}

func (m *Module) OnEvent(e *riq.Entity) {
	switch e.Datamodel {
	case "tapTroupe/bop":
		m.bops = append(m.bops, bopEvt{
			beat: e.Beat, length: e.Length,
			bop: boolDefault(e, "bop", true), auto: boolParam(e, "bopAuto"),
		})
	case "tapTroupe/stepping":
		length := e.Length
		if length <= 0 {
			length = 4
		}
		m.steps = append(m.steps, stepEvt{beat: e.Beat, length: length, startTap: boolParam(e, "startTap")})
	case "tapTroupe/tapping":
		length := e.Length
		if length <= 0 {
			length = 3
		}
		m.taps = append(m.taps, tapEvt{
			beat: e.Beat, length: length,
			okay: boolDefault(e, "okay", true), okayType: int(e.Float("okayType", okayA)),
			animType: int(e.Float("animType", okAnimNormal)), popperBeats: e.Float("popperBeats", 2),
			randomVoiceLine: boolDefault(e, "randomVoiceLine", true), noReady: boolParam(e, "noReady"),
		})
	case "tapTroupe/spotlights":
		m.spots = append(m.spots, spotEvt{
			beat: e.Beat, on: boolDefault(e, "toggle", true),
			player: boolDefault(e, "player", true), midLeft: boolParam(e, "middleLeft"),
			midRight: boolParam(e, "middleRight"), leftMost: boolParam(e, "leftMost"),
		})
	case "tapTroupe/zoomOut":
		length := e.Length
		if length <= 0 {
			length = 4
		}
		m.zooms = append(m.zooms, zoomEvt{beat: e.Beat, length: length, ease: int(e.Float("ease", 3))})
	case "tapTroupe/tutorialMissFace":
		m.missFaces = append(m.missFaces, struct {
			beat float64
			on   bool
		}{e.Beat, boolDefault(e, "toggle", true)})
	}
}

func (m *Module) Ready() {
	sort.SliceStable(m.bops, func(i, j int) bool { return m.bops[i].beat < m.bops[j].beat })
	sort.SliceStable(m.steps, func(i, j int) bool { return m.steps[i].beat < m.steps[j].beat })
	sort.SliceStable(m.taps, func(i, j int) bool { return m.taps[i].beat < m.taps[j].beat })
	sort.SliceStable(m.spots, func(i, j int) bool { return m.spots[i].beat < m.spots[j].beat })
	sort.SliceStable(m.zooms, func(i, j int) bool { return m.zooms[i].beat < m.zooms[j].beat })

	for _, ev := range m.bops {
		ev := ev
		m.ctx.At(ev.beat, func() { m.goBop = ev.auto })
		if ev.bop {
			for i := 0.0; i < ev.length; i++ {
				b := ev.beat + i
				m.ctx.At(b, func() { m.bopSingle(b) })
			}
		}
	}
	for _, ev := range m.steps {
		ev := ev
		m.scheduleStepping(ev)
	}
	for _, ev := range m.taps {
		ev := ev
		m.scheduleTapping(ev)
	}
	for _, ev := range m.spots {
		ev := ev
		m.ctx.At(ev.beat, func() { m.setSpotlights(ev.on, ev.player, ev.midLeft, ev.midRight, ev.leftMost) })
	}
	for _, ev := range m.zooms {
		ev := ev
		m.ctx.At(ev.beat, func() {
			m.zoomActive, m.zoomStart, m.zoomLength, m.zoomEase = true, ev.beat, ev.length, ev.ease
		})
	}
	for _, ev := range m.missFaces {
		ev := ev
		m.ctx.At(ev.beat, func() { m.useTutorialFace = ev.on })
	}
}

func (m *Module) OnSwitch(beat float64) {
	m.initScene(beat)
	m.lastPulse = int(math.Floor(beat))
}

func (m *Module) Whiff(beat float64) {
	m.ctx.SoundVol("miss", 1)
	if m.canSpit && !m.useTutorialFace {
		m.ctx.SoundVol("spit", 0.5)
	}
	m.ctx.ScoreMiss()
	m.setMissFaces(missSpit)
	if m.useTutorialFace {
		m.setMissFaces(missLOL)
	}
	if m.tapping {
		m.missedTaps = true
		m.playerTapper.tap(m, beat, m.currentTapAnim, false, m.shouldSwitch)
		m.playerCorner.bop(m, beat)
	} else {
		m.playerTapper.step(m, beat, false, true)
		m.playerCorner.bop(m, beat)
	}
	m.canSpit = false
}

func (m *Module) Update(_ float64, beat float64) {
	m.updateBeatPulse(beat)
	m.updateZoom(beat)
	m.keepPoppers(beat)
}

func (m *Module) Draw(screen *ebiten.Image, _ float64, beat float64) {
	screen.Fill(color.NRGBA{0xf5, 0xf5, 0xf5, 0xff})
	cam := m.ctx.CameraAt(beat)
	m.ctx.Scene.SetCamera(cam[0], cam[1]+m.zoomY, cam[2]+m.zoomZ)
	m.ctx.Scene.Sample(beat)
	m.ctx.Scene.Draw(screen, m.proj)
	m.drawPoppers(screen, beat)
}

func (m *Module) updateBeatPulse(beat float64) {
	p := int(math.Floor(beat + 1e-6))
	if p <= m.lastPulse {
		return
	}
	for b := m.lastPulse + 1; b <= p; b++ {
		if b >= 0 && m.goBop {
			m.bopSingle(float64(b))
		}
	}
	m.lastPulse = p
}

func (m *Module) updateZoom(beat float64) {
	if !m.zoomActive || m.zoomLength <= 0 {
		m.zoomY, m.zoomZ = 0, 0
		m.ctx.Scene.PlayFrozen("", "NoZoomOut", 0)
		return
	}
	u := clamp01((beat - m.zoomStart) / m.zoomLength)
	m.zoomY = engine.Ease(m.zoomEase, 0, 30, u)
	m.zoomZ = engine.Ease(m.zoomEase, 0, -100, u)
	animU := clamp01(u * 4)
	m.ctx.Scene.PlayNormalized("", "Animations/ZoomOut", animU)
	for _, t := range m.allTappers() {
		m.ctx.Scene.PlayNormalized(t.root, "Animations/FeetFadeOut", clamp01(u*32))
	}
}

func (m *Module) scheduleStepping(ev stepEvt) {
	for i := 0; i < int(ev.length); i++ {
		target := ev.beat + float64(i)
		m.ctx.ScheduleInput(target, func(state float64, _ engine.Judgment) {
			m.justStep(target, state)
		}, func() { m.missStep() })
		m.ctx.At(target, func() {
			m.npcStep(target, true, true)
			m.ctx.SoundVol("other1", 0.75)
		})
	}
	m.ctx.At(ev.beat-1, func() {
		if m.tapping {
			return
		}
		m.npcStep(ev.beat-1, false, false)
		m.playerTapper.step(m, ev.beat-1, false, false)
		m.playerCorner.bop(m, ev.beat-1)
	})
	m.ctx.At(ev.beat, func() {
		if ev.startTap {
			m.ctx.Sound("startTap")
		}
		m.stepping = true
	})
	m.ctx.At(ev.beat+ev.length+1, func() { m.stepping = false })
}

func (m *Module) scheduleTapping(ev tapEvt) {
	if !ev.noReady {
		m.ctx.SoundAt(ev.beat-2, "tapReady1", 1)
		m.ctx.SoundAt(ev.beat-1, "tapReady2", 1)
	}
	m.ctx.At(ev.beat-1.1, func() { m.prepareTap = true })
	m.ctx.At(ev.beat, func() { m.prepareTap = false })

	actual := actualTapLength(ev.length)
	m.ctx.SoundAt(ev.beat, "tapAnd", 1)

	secondBam := false
	finalBeat := ev.beat + actual
	for i := 0.0; i < actual; i += 0.75 {
		sound, other := "bamvoice1", "other3"
		spawn := ev.beat + i + 0.5
		anim, switchFeet, npcHit := tapBam, true, true
		clearTapping := false

		switch {
		case i+0.75 >= actual:
			sound, other = "startTap", "other2"
			spawn = math.Ceil(ev.beat + i)
			finalBeat = spawn
			anim, switchFeet, clearTapping = tapLast, false, true
		case i+1.5 >= actual:
			sound = "tapvoice2"
			anim, switchFeet = tapTap, false
		case i+2.25 >= actual:
			sound = "tapvoice1"
			anim = tapTap
			switchFeet = actual == 2.25
		default:
			if secondBam {
				sound = "bamvoice2"
			}
			switch {
			case i+3 >= actual && actual == 3:
				anim, switchFeet = tapTap, true
			case i+3 >= actual:
				anim, switchFeet = tapBamTapReady, true
			case i == 0:
				anim, switchFeet = tapBamReady, false
			default:
				anim, switchFeet = tapBam, true
			}
		}

		prepareBeat := spawn - 0.3
		m.ctx.At(prepareBeat, func() {
			m.currentTapAnim = anim
			m.shouldSwitch = switchFeet
		})
		m.ctx.At(spawn, func() {
			m.npcTap(spawn, anim, npcHit, switchFeet)
			if clearTapping {
				m.ctx.At(spawn+0.1, func() { m.tapping = false })
			}
		})
		m.ctx.SoundAt(spawn, sound, 1)
		m.ctx.SoundAt(spawn, other, 1)
		m.ctx.ScheduleInput(spawn, func(state float64, _ engine.Judgment) {
			m.justTap(spawn, state)
		}, func() { m.missTap() })
		secondBam = !secondBam
	}

	okType := ev.okayType
	if okType == okayRandom {
		okType = deterministicChoice(ev.beat, 3)
	}
	ok1 := string([]byte{'A' + byte(deterministicChoice(ev.beat+0.37, 3))})
	ok2 := []string{"A", "B", "C"}[clampInt(okType, 0, 2)]
	animType := ev.animType
	if animType == okAnimRandom {
		animType = deterministicChoice(ev.beat+0.73, 3)
	}

	m.ctx.At(ev.beat-1, func() {
		if !m.stepping {
			m.npcStep(ev.beat-1, false, true)
			m.playerTapper.step(m, ev.beat-1, false, true)
			m.playerCorner.bop(m, ev.beat-1)
		}
	})
	m.ctx.At(ev.beat, func() {
		m.tapping = true
		m.missedTaps = false
	})
	m.ctx.At(finalBeat, func() {
		if !m.missedTaps && animType == okAnimPopper {
			m.playerCorner.partyPopper(m, finalBeat+ev.popperBeats)
			for _, c := range m.npcCorners {
				c.partyPopper(m, finalBeat+ev.popperBeats)
			}
		}
	})
	m.ctx.At(finalBeat+0.5, func() {
		if m.missedTaps || !ev.okay {
			return
		}
		m.playerCorner.okay(m, finalBeat+0.5)
		for _, c := range m.npcCorners {
			c.okay(m, finalBeat+0.5)
		}
		m.ctx.Sound("okay" + ok1 + "1")
		m.ctx.SoundVol("okay"+ok2+"2", 0.75)
	})
	m.ctx.At(finalBeat+1, func() {
		if !m.missedTaps && ev.okay && ev.randomVoiceLine {
			r := deterministicChoice(ev.beat+1.9, 100)
			if r == 0 {
				m.ctx.Sound("woo")
			} else if r == 1 {
				m.ctx.SoundVol("laughter", 0.4)
			}
		}
		if m.missedTaps || animType != okAnimSign {
			return
		}
		m.playerCorner.okaySign(m, finalBeat+1)
		for _, c := range m.npcCorners {
			c.okaySign(m, finalBeat+1)
		}
	})
}

func (m *Module) bopSingle(beat float64) {
	m.playerTapper.bop(m, beat)
	for _, t := range m.npcTappers {
		t.bop(m, beat)
	}
	m.playerCorner.bop(m, beat)
	for _, c := range m.npcCorners {
		c.bop(m, beat)
	}
}

func (m *Module) npcStep(beat float64, hit, switchFeet bool) {
	for _, t := range m.npcTappers {
		t.step(m, beat, hit, switchFeet)
	}
	for _, c := range m.npcCorners {
		c.bop(m, beat)
	}
}

func (m *Module) npcTap(beat float64, anim int, hit, switchFeet bool) {
	for _, t := range m.npcTappers {
		t.tap(m, beat, anim, hit, switchFeet)
	}
	for _, c := range m.npcCorners {
		c.bop(m, beat)
	}
}

func (m *Module) justStep(beat, state float64) {
	m.canSpit = true
	if math.Abs(state) >= 1 {
		m.playerTapper.step(m, beat, false, true)
		m.playerCorner.bop(m, beat)
		m.ctx.Sound("tink")
		m.flipStepSound()
		m.setMissFaces(missSad)
		return
	}
	m.playerTapper.step(m, beat, true, true)
	m.playerCorner.bop(m, beat)
	m.ctx.Sound("step" + string(rune('0'+m.stepSound)))
	m.flipStepSound()
	m.resetFaces()
	m.ctx.At(beat+0.1, func() { m.alignPlayerNextStep() })
}

func (m *Module) justTap(beat, state float64) {
	if math.Abs(state) >= 1 {
		m.missedTaps = true
		m.canSpit = true
		m.playerTapper.tap(m, beat, m.currentTapAnim, false, m.shouldSwitch)
		m.playerCorner.bop(m, beat)
		if m.currentTapAnim == tapLast {
			m.ctx.Sound("tap3")
		} else {
			m.ctx.Sound("tink")
		}
		m.setMissFaces(missSad)
		return
	}
	m.canSpit = true
	m.playerTapper.tap(m, beat, m.currentTapAnim, true, m.shouldSwitch)
	m.playerCorner.bop(m, beat)
	if m.currentTapAnim == tapLast {
		m.ctx.Sound("tap3")
	} else {
		m.ctx.Sound("player3")
	}
	m.resetFaces()
}

func (m *Module) missStep() {
	if m.canSpit && !m.useTutorialFace {
		m.ctx.SoundVol("spit", 0.5)
	}
	m.setMissFaces(missSpit)
	if m.useTutorialFace {
		m.setMissFaces(missLOL)
	}
	m.canSpit = false
}

func (m *Module) missTap() {
	m.missedTaps = true
	m.missStep()
}

func (m *Module) flipStepSound() {
	if m.stepSound == 1 {
		m.stepSound = 2
	} else {
		m.stepSound = 1
	}
}

func (m *Module) alignPlayerNextStep() {
	if len(m.npcTappers) == 0 {
		return
	}
	if m.playerTapper.mirror != m.npcTappers[0].mirror {
		m.playerTapper.dontSwitchNext = true
	}
}

func (t *tapper) step(m *Module, beat float64, hit, switchFeet bool) {
	if switchFeet && !t.dontSwitchNext {
		t.mirror = !t.mirror
	}
	if t.dontSwitchNext {
		t.dontSwitchNext = false
	}
	m.ctx.Scene.SetMirrorX(t.root, t.mirror)
	state := "StepFeet"
	if hit {
		if m.prepareTap {
			state = "HitStepReadyTap"
		} else {
			state = "HitStepFeet"
		}
	} else if m.prepareTap {
		state = "StepReadyTap"
	}
	m.ctx.Scene.PlayState(t.root, state, beat, 0.5)
}

func (t *tapper) tap(m *Module, beat float64, anim int, hit, switchFeet bool) {
	state := ""
	if hit {
		state = "Hit"
	}
	switch anim {
	case tapBam:
		state += "BamFeet"
		if switchFeet && !t.dontSwitchNext {
			t.mirror = !t.mirror
		}
	case tapTap:
		state += "TapFeet"
		if switchFeet {
			t.mirror = !t.mirror
		}
	case tapBamReady:
		state += "BamReadyFeet"
		if switchFeet {
			t.mirror = !t.mirror
		}
	case tapBamTapReady:
		state += "BamReadyTap"
		if switchFeet {
			t.mirror = !t.mirror
		}
	case tapLast:
		if hit {
			state = "LastTapFeet"
		} else {
			state = "StepFeet"
		}
		if switchFeet {
			t.mirror = !t.mirror
		}
	}
	if t.dontSwitchNext && anim == tapBam {
		t.dontSwitchNext = false
	}
	m.ctx.Scene.SetMirrorX(t.root, t.mirror)
	m.ctx.Scene.PlayState(t.root, state, beat, 0.5)
}

func (t *tapper) bop(m *Module, beat float64) {
	m.ctx.Scene.PlayState(t.root, "BopFeet", beat, 0.5)
}

func (c *corner) bop(m *Module, beat float64) {
	m.ctx.Scene.PlayState(c.root, "Bop", beat, 0.3)
}

func (c *corner) okay(m *Module, beat float64) {
	m.ctx.Scene.PlayState(c.expr, "Okay", beat, 0.25)
}

func (c *corner) okaySign(m *Module, beat float64) {
	m.ctx.Scene.PlayState(c.body, "OkaySign", beat, 0.25)
}

func (c *corner) setFace(m *Module, state string, beat float64) {
	m.ctx.Scene.PlayState(c.expr, state, beat, 0.5)
}

func (c *corner) partyPopper(m *Module, beat float64) {
	m.ctx.Scene.PlayState(c.body, "PartyPopperReady", beat-1, 0.5)
	m.ctx.At(beat, func() { m.ctx.Scene.PlayState(c.body, "PartyPopper", beat, 0.5) })
	m.ctx.At(beat+1, func() {
		m.ctx.Scene.PlayState(c.body, "PartyPopperPop", beat+1, 0.25)
		m.ctx.Sound("popper")
		m.spawnPopper(c.popper, beat+1)
	})
	m.ctx.At(beat+3, func() { m.ctx.Scene.PlayState(c.body, "IdleBody", beat+3, 0.5) })
}

const (
	missSad = iota
	missSpit
	missLOL
)

func (m *Module) setMissFaces(kind int) {
	state := "Sad"
	if kind == missSpit {
		state = "Spit"
	} else if kind == missLOL {
		state = "LOL"
	}
	for _, c := range m.npcCorners {
		c.setFace(m, state, m.ctx.Beat())
	}
}

func (m *Module) resetFaces() {
	for _, c := range m.npcCorners {
		c.setFace(m, "NoExpression", m.ctx.Beat())
	}
}

func (m *Module) setSpotlights(on, player, midLeft, midRight, leftMost bool) {
	m.ctx.Scene.SetActive("ForegroundElements/Darkness", on)
	m.ctx.Scene.SetActive("SpotLights/spotlight", on && player)
	m.ctx.Scene.SetActive("SpotLights/spotlight (2)", on && midLeft)
	m.ctx.Scene.SetActive("SpotLights/spotlight (1)", on && midRight)
	m.ctx.Scene.SetActive("SpotLights/spotlight (3)", on && leftMost)
}

func (m *Module) allTappers() []*tapper {
	out := make([]*tapper, 0, len(m.npcTappers)+1)
	out = append(out, m.npcTappers...)
	out = append(out, m.playerTapper)
	return out
}

func (m *Module) allCorners() []*corner {
	out := make([]*corner, 0, len(m.npcCorners)+1)
	out = append(out, m.npcCorners...)
	out = append(out, m.playerCorner)
	return out
}

func (m *Module) spawnPopper(path string, beat float64) {
	r := rand.New(rand.NewSource(int64(beat*4096) + int64(len(path))*97))
	colors := []color.NRGBA{{255, 105, 180, 230}, {255, 230, 80, 230}, {80, 220, 255, 230}, {130, 255, 120, 230}}
	drops := make([]confetti, 0, 18)
	for i := 0; i < 18; i++ {
		ang := -math.Pi/2 + (r.Float64()-0.5)*math.Pi*1.1
		spd := 1.4 + r.Float64()*2.2
		drops = append(drops, confetti{
			x: (r.Float64() - 0.5) * 0.4, y: 0,
			vx: math.Cos(ang) * spd, vy: -math.Sin(ang) * spd,
			life: 0.8 + r.Float64()*0.5, col: colors[r.Intn(len(colors))],
		})
	}
	m.poppers = append(m.poppers, popperBurst{beat: beat, path: path, drops: drops})
}

func (m *Module) keepPoppers(beat float64) {
	out := m.poppers[:0]
	for _, p := range m.poppers {
		if beat < p.beat+1.8 {
			out = append(out, p)
		}
	}
	m.poppers = out
}

func (m *Module) drawPoppers(screen *ebiten.Image, beat float64) {
	for _, burst := range m.poppers {
		world, ok := m.ctx.Scene.NodeWorld(burst.path)
		if !ok {
			continue
		}
		t := m.ctx.BeatToTime(beat) - m.ctx.BeatToTime(burst.beat)
		for _, d := range burst.drops {
			u := clamp01((beat - burst.beat) / d.life)
			if u >= 1 {
				continue
			}
			x := d.x + d.vx*t
			y := d.y + d.vy*t - 2.6*t*t
			wx, wy := world.Apply(x, y)
			sx, sy := m.proj.Apply(wx, wy)
			c := d.col
			c.A = uint8(float64(c.A) * (1 - u))
			vector.DrawFilledRect(screen, float32(sx-2), float32(sy-2), 4, 4, c, true)
		}
	}
}

func actualTapLength(length float64) float64 {
	actual := length - 0.5
	actual -= math.Mod(actual, 0.75)
	if actual < 2.25 {
		actual = 2.25
	}
	return actual
}

func deterministicChoice(seed float64, n int) int {
	if n <= 0 {
		return 0
	}
	r := rand.New(rand.NewSource(int64(seed * 10000)))
	return r.Intn(n)
}

func boolParam(e *riq.Entity, key string) bool { return e.Float(key, 0) != 0 }

func boolDefault(e *riq.Entity, key string, def bool) bool {
	if _, ok := e.Data[key]; !ok {
		return def
	}
	return e.Float(key, 0) != 0
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

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
