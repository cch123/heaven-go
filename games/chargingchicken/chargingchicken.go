// Package chargingchicken ports Charging Chicken's public action surface and
// main charge/blastoff loop.
package chargingchicken

import (
	"image/color"
	"math"
	"sort"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"hsdemo/engine"
	"hsdemo/games/internal/particlefx"
	"hsdemo/kart"
	"hsdemo/riq"
)

type chargeEvt struct {
	beat, length      float64
	drum              int
	cowbell, bubble   bool
	endText, textLen  int
	success, fail     string
	destination       int
	customDestination string
	helmet            bool
}

type journeyEvt struct {
	beat, length float64
}

type colorEvt struct {
	beat, length float64
	a0, a1       [4]float64
	b0, b1       [4]float64
}

type Module struct {
	ctx  *engine.Ctx
	proj kart.Aff

	chicken     string
	water       string
	parallax    string
	unparallax  string
	birdsAnim   string
	stars       string
	clouds      string
	planets     string
	doodles     string
	birds       string
	yardsText   string
	endingText  string
	bubbleText  string
	countBubble string
	helmet      string
	fallHelmet  string
	bgHigh      string
	bgLow       string
	gradient    string
	headlight   string

	chickenMat string
	carMat     string
	cloudMat   string
	waterMat   string

	chickenT     *kart.Template
	islandT      *kart.Template
	current      *island
	next         *island
	chickenGhost *kart.Instance

	charger       string
	fakeChicken   string
	platform      string
	bigLandmass   string
	smallLandmass string
	stoneSplash   string
	chickenSplash string
	collapseOK    string
	collapseNG    string
	grassL        string
	grassR        string

	particles *particlefx.Runtime
	effects   []particlefx.Effect

	charges   []chargeEvt
	journeys  []journeyEvt
	bgCols    []colorEvt
	carCols   []colorEvt
	cloudCols []colorEvt
	lightCols []colorEvt

	nextInputReady  float64
	inputting       bool
	canBlastOff     bool
	playerSucceeded bool
	successKillBeat float64
	drumSwitch      float64
	drumVolume      float64
	drumFadeStart   float64
	drumFadeLength  float64
	drumFadeIn      bool
	drumLoud        bool
	drumReset       bool
	drumTempVolume  float64

	yardsTemplate    string
	yardsEditable    bool
	yardsLength      float64
	bubbleEnd        float64
	bubbleScale      float64
	bubbleA, bubbleB float64
	bubbleGrow       bool

	journeyStartBeat float64
	journeyLength    float64
	lastT            float64
	hasLastT         bool
	parallaxX        float64
}

func New() engine.Module {
	return &Module{
		successKillBeat: math.MaxFloat64,
		drumVolume:      1,
		drumTempVolume:  1,
		drumFadeIn:      true,
		drumReset:       true,
		drumSwitch:      -math.MaxFloat64,
		bubbleScale:     1.038702,
		yardsTemplate:   "<color=#FFFF00>%</color> yards to the goal.",
	}
}

func (m *Module) ID() string { return "chargingChicken" }

func (m *Module) Load(ctx *engine.Ctx) error {
	m.ctx = ctx
	if err := ctx.LoadAssets("chargingChicken"); err != nil {
		return err
	}
	m.proj = kart.Translate(engine.ScreenW/2, engine.ScreenH/2).Mul(kart.Scale(54, -54))
	as := ctx.Assets
	m.chicken = roleOr(as, "ChickenAnim", "Car")
	m.water = roleOr(as, "WaterAnim", "Sea")
	m.parallax = roleOr(as, "ParallaxFade", "Parallax")
	m.unparallax = roleOr(as, "UnParallaxFade", "UnParallax")
	m.birdsAnim = roleOr(as, "BirdsAnim", "Parallax/Birds")
	m.stars = roleOr(as, "Stars", "Parallax/Stars")
	m.clouds = roleOr(as, "Clouds", "Parallax/CloudySky")
	m.planets = roleOr(as, "Planets", "Parallax/Planets")
	m.doodles = roleOr(as, "Doodles", "Parallax/Doodles")
	m.birds = roleOr(as, "Birds", "Parallax/Birds")
	m.yardsText = roleOr(as, "yardsText", "Yards Text")
	m.endingText = roleOr(as, "endingText", "ArrivalText")
	m.bubbleText = roleOr(as, "bubbleText", "BubblePivot/CountBubble/Count Text")
	m.countBubble = roleOr(as, "countBubble", "BubblePivot")
	m.helmet = roleOr(as, "Helmet", "Car/Helmet")
	m.fallHelmet = roleOr(as, "FallingHelmet", "Car/FallingCar/CarWindow (1)/Helmet")
	m.bgHigh = roleOr(as, "bgHigh", "BG/Color1")
	m.bgLow = roleOr(as, "bgLow", "BG/Color2")
	m.gradient = roleOr(as, "gradient", "BG/Gradient")
	m.headlight = roleOr(as, "headlightColor", "Car/CarBody/HeadLight")
	game := as.Extra.Components["game"]
	m.chickenMat = game.Refs["chickenColors"]
	m.carMat = game.Refs["chickenColorsCar"]
	m.cloudMat = game.Refs["chickenColorsCloud"]
	m.waterMat = game.Refs["chickenColorsWater"]

	islandComp := as.Extra.Components["island"]
	m.charger = islandComp.Refs["ChargerAnim"]
	m.fakeChicken = islandComp.Refs["FakeChickenAnim"]
	m.platform = islandComp.Refs["PlatformAnim"]
	m.bigLandmass = islandComp.Refs["BigLandmass"]
	m.smallLandmass = islandComp.Refs["SmallLandmass"]
	m.stoneSplash = islandComp.Refs["StoneSplashEffect"]
	m.chickenSplash = islandComp.Refs["ChickenSplashEffect"]
	m.collapseOK = islandComp.Refs["IslandCollapse"]
	m.collapseNG = islandComp.Refs["IslandCollapseNg"]
	m.grassL = islandComp.Refs["GrassL"]
	m.grassR = islandComp.Refs["GrassR"]
	m.particles = particlefx.New(as, m.proj, 1.2)

	m.islandT = kart.NewTemplate(as, "Island")
	m.current = newIsland(m, 0)
	m.next = newIsland(m, platformDistance*1.5)
	ctx.Scene.SetActive("Island", false)
	ctx.Scene.SetActive(m.countBubble, false)
	ctx.Scene.SetActive(m.helmet, false)
	ctx.Scene.SetActive(m.fallHelmet, false)
	ctx.Scene.PlayDefaultState(m.chicken, 0, ctx.SecPerBeat(0))
	ctx.Scene.PlayState(m.water, "Scroll", 0, 0.2)
	ctx.Scene.PlayDefaultState(m.parallax, 0, ctx.SecPerBeat(0))
	ctx.Scene.PlayDefaultState(m.unparallax, 0, ctx.SecPerBeat(0))
	ctx.Scene.PlayDefaultState(m.birdsAnim, 0, ctx.SecPerBeat(0))
	_ = ctx.Assets.SetText(m.yardsText, "")
	_ = ctx.Assets.SetText(m.endingText, "")
	_ = ctx.Assets.SetText(m.bubbleText, "")
	return nil
}

func (m *Module) OnEvent(e *riq.Entity) {
	switch e.Datamodel {
	case "chargingChicken/input":
		m.charges = append(m.charges, chargeEvt{
			beat: e.Beat, length: e.Length, drum: intDefault(e, "drumbeat", 1),
			cowbell: boolDefault(e, "cowbell", true), bubble: boolParam(e, "bubble"),
			endText: intDefault(e, "endText", 0), textLen: intDefault(e, "textLength", 4),
			success: strDefault(e, "success", "Well Done!"), fail: strDefault(e, "fail", "Too bad..."),
			destination: intDefault(e, "destination", 1), customDestination: strDefault(e, "customDestination", "You arrived in The Backrooms!"),
			helmet: boolParam(e, "spaceHelmet"),
		})
	case "chargingChicken/journeyLength":
		m.journeys = append(m.journeys, journeyEvt{beat: e.Beat, length: e.Length})
	case "chargingChicken/bubbleShrink":
		beat, length := e.Beat, e.Length
		grow, instant := boolParam(e, "grow"), boolParam(e, "instant")
		m.ctx.At(beat, func() { m.bubbleShrink(beat, length, grow, instant) })
	case "chargingChicken/textEdit":
		beat := e.Beat
		txt := strDefault(e, "text", "# yards to the goal.")
		col := colorParam(e, "color", hex(0xff, 0xff, 0))
		m.ctx.At(beat, func() { m.textEdit(txt, col) })
	case "chargingChicken/musicFade":
		beat, length := e.Beat, e.Length
		fadeIn, instant, drums, reset := boolParam(e, "fadeIn"), boolParam(e, "instant"), boolDefault(e, "drums", true), boolDefault(e, "reset", true)
		m.ctx.At(beat, func() { m.musicFade(beat, length, fadeIn, instant, drums, reset) })
	case "chargingChicken/changeBgColor":
		m.bgCols = append(m.bgCols, colorEvt{beat: e.Beat, length: e.Length,
			a0: colorParam(e, "colorFrom", defaultBGTop), a1: colorParam(e, "colorTo", defaultBGTop),
			b0: colorParam(e, "colorFrom2", defaultBGBottom), b1: colorParam(e, "colorTo2", defaultBGBottom)})
	case "chargingChicken/changeCarColor":
		m.carCols = append(m.carCols, colorEvt{beat: e.Beat, length: e.Length,
			a0: colorParam(e, "colorFrom", defaultCar), a1: colorParam(e, "colorTo", defaultCar),
			b0: colorParam(e, "colorFrom2", defaultCarCharge), b1: colorParam(e, "colorTo2", defaultCarCharge)})
	case "chargingChicken/changeCloudColor":
		m.cloudCols = append(m.cloudCols, colorEvt{beat: e.Beat, length: e.Length,
			a0: colorParam(e, "colorFrom", defaultCloud), a1: colorParam(e, "colorTo", defaultCloud),
			b0: colorParam(e, "colorFrom2", defaultCloud2), b1: colorParam(e, "colorTo2", defaultCloud2)})
	case "chargingChicken/changeFgLight":
		m.lightCols = append(m.lightCols, colorEvt{beat: e.Beat, length: e.Length,
			a0: [4]float64{e.Float("lightFrom", 1), e.Float("lightFrom", 1), e.Float("lightFrom", 1), 1},
			a1: [4]float64{e.Float("lightTo", 1), e.Float("lightTo", 1), e.Float("lightTo", 1), 1},
			b0: [4]float64{1, 1, 1, e.Float("headLightFrom", 0)}, b1: [4]float64{1, 1, 1, e.Float("headLightTo", 0)}})
	case "chargingChicken/parallaxObjects":
		ev := e
		m.ctx.At(e.Beat, func() { m.parallaxObjects(ev) })
	case "chargingChicken/unParallaxObjects":
		ev := e
		m.ctx.At(e.Beat, func() { m.unParallaxObjects(ev) })
	case "chargingChicken/parallaxProgress":
		ev := e
		m.ctx.At(e.Beat, func() { m.parallaxProgress(ev) })
	case "chargingChicken/lookhaha":
		beat, length := e.Beat, e.Length
		m.ctx.At(beat, func() { m.look(beat, length) })
	case "chargingChicken/explodehaha":
		m.ctx.At(e.Beat, func() { m.explode(m.ctx.Beat(), 3) })
	}
}

func (m *Module) Ready() {
	sort.Slice(m.charges, func(i, j int) bool { return m.charges[i].beat < m.charges[j].beat })
	sort.Slice(m.journeys, func(i, j int) bool { return m.journeys[i].beat < m.journeys[j].beat })
	for _, ev := range m.charges {
		ev := ev
		if ev.cowbell {
			for i := 4; i >= 1; i-- {
				m.ctx.SoundAt(ev.beat-float64(i), "cowbell", 1)
			}
		}
		m.chargeUp(ev, 4)
	}
}

func (m *Module) OnSwitch(beat float64) {
	m.drumSwitch = beat
	m.successKillBeat = math.MaxFloat64
	if m.current == nil {
		m.current = newIsland(m, 0)
	}
	if m.next == nil {
		m.next = newIsland(m, platformDistance*1.5)
	}
	m.current.setDefaults(beat)
	m.next.setDefaults(beat)
	m.ctx.Scene.PlayDefaultState(m.chicken, beat, m.ctx.SecPerBeat(beat))
	m.ctx.Scene.SetActive(m.countBubble, false)
	_ = m.ctx.Assets.SetText(m.yardsText, "")
	_ = m.ctx.Assets.SetText(m.endingText, "")
}

func (m *Module) Whiff(beat float64) {
	if !m.inputting {
		m.current.inst.PlayState(relIsland(m.charger), "Bounce", beat, 0.5)
		m.ctx.Sound("somen_catch")
		m.ctx.Sound("somen_catch_old")
	}
}

func (m *Module) Update(t, beat float64) {
	if m.hasLastT {
		dt := t - m.lastT
		if m.current != nil && m.current.moving {
			m.parallaxX -= dt * 2.2
		}
	}
	m.lastT, m.hasLastT = t, true
	if m.current != nil {
		m.current.update(beat)
	}
	if m.next != nil {
		m.next.update(beat)
	}
	m.effects = liveEffects(m.effects, t)
	m.applyBubble(beat)
	m.applyColors(beat)
	m.applyParallax()
}

func (m *Module) Draw(screen *ebiten.Image, t, beat float64) {
	screen.Fill(color.RGBA{R: 0x6e, G: 0xd6, B: 0xff, A: 0xff})
	m.ctx.SampleScene(beat)
	if m.current != nil {
		m.current.queue(m.ctx.Scene, beat)
	}
	if m.next != nil {
		m.next.queue(m.ctx.Scene, beat)
	}
	m.ctx.Scene.Draw(screen, m.proj)
	for _, fx := range m.effects {
		m.particles.Draw(screen, fx, t)
	}
}

func (m *Module) chargeUp(ev chargeEvt, lateness float64) {
	length := math.Ceil(ev.length)
	if length < 4 {
		length = 4
	}
	if ev.beat < m.nextInputReady {
		return
	}
	m.nextInputReady = ev.beat + length*2
	journeyBeat := ev.beat + length
	journeyLength := length - 1
	if j, ok := m.customJourney(ev.beat + length); ok {
		journeyLength = j
	}
	m.yardsLength = length
	m.successKillBeat = ev.beat - 1
	m.schedulePrep(ev.beat, lateness)
	m.ctx.ScheduleInputAction(ev.beat-1, actionPress, func(state float64, _ engine.Judgment) {
		m.startCharging(ev.drum, state)
	}, func() { m.startMiss() })
	m.ctx.ScheduleInputReleaseCond(ev.beat+length, func() bool { return m.inputting }, func(state float64, _ engine.Judgment) {
		m.blastOff(ev, state, false)
		if math.Abs(state) >= 1 {
			m.ctx.Sound("miss")
		}
	}, func() { m.endMiss() })
	m.ctx.At(ev.beat-2, func() {
		m.setYardsText()
		if m.next != nil {
			m.next.spawnStones(journeyBeat, journeyLength, lateness < 2)
		}
	})
	m.ctx.At(ev.beat-1, func() {
		m.ctx.Scene.PlayState(m.chicken, "Prepare", ev.beat-1, 0.5)
		m.bubbleEnd = ev.beat + length
		m.ctx.Sound("SE_CHIKEN_BLOCK_SET")
		m.ctx.Scene.SetActive(m.helmet, ev.helmet)
		m.ctx.Scene.SetActive(m.fallHelmet, ev.helmet)
		m.spawnJourney(journeyBeat, journeyLength)
	})
	m.ctx.At(ev.beat, func() {
		m.ctx.Scene.SetActive(m.countBubble, ev.bubble)
		m.successKillBeat = math.MaxFloat64
	})
	m.ctx.At(ev.beat+1, func() {
		m.canBlastOff = true
		if !m.inputting {
			m.collapseUnderPlayer()
		}
	})
	m.ctx.At(ev.beat+length+1, func() { m.explode(m.ctx.Beat(), length) })
	for i := 1; float64(i) < length+1; i++ {
		b := ev.beat + float64(i)
		m.ctx.At(b, func() {
			if m.inputting {
				m.current.charge(b)
			}
		})
	}
	m.scheduleDrums(ev.beat, length+1, ev.drum)
	m.ctx.At(journeyBeat+journeyLength, func() {
		m.setEndText(ev)
	})
}

func (m *Module) customJourney(beat float64) (float64, bool) {
	for _, j := range m.journeys {
		if math.Abs(j.beat-beat) < 1e-6 {
			return j.length, true
		}
	}
	return 0, false
}

func (m *Module) schedulePrep(beat, lateness float64) {
	for step := 4; step >= 1; step-- {
		step := step
		at := beat - float64(step)
		m.ctx.At(at, func() {
			if m.next != nil && lateness > float64(step-1) {
				m.next.inst.PlayState(relIsland(m.charger), "Prep"+itoa(5-step), at, 0.5)
			}
		})
	}
}

func (m *Module) startCharging(drum int, state float64) {
	m.inputting = true
	m.pumpSound(state)
	m.startDrumSound(drum)
	m.ctx.Scene.PlayState(m.chicken, "Charge", m.ctx.Beat(), 0.5)
	m.current.charge(m.ctx.Beat())
	m.canBlastOff = false
}

func (m *Module) startMiss() {
	m.inputting = false
	m.yardsEditable = false
	_ = m.ctx.Assets.SetText(m.yardsText, "")
	m.ctx.Scene.SetActive(m.countBubble, false)
}

func (m *Module) startDrumSound(drum int) {
	switch drum {
	case 0:
	case 5:
		m.ctx.Sound("feverkick")
	case 6:
		m.ctx.Sound("dskick")
	case 7:
		m.ctx.Sound("gbakick")
	case 8:
		m.ctx.Sound("MISC1")
	case 9:
		m.ctx.Sound("MISC21")
	case 10:
		m.ctx.Sound("practicekick")
	default:
		m.ctx.Sound("kick")
		m.ctx.Sound("hihat")
	}
}

func (m *Module) pumpSound(state float64) {
	if math.Abs(state) >= 1 {
		m.ctx.Sound("miss")
	} else {
		m.ctx.Sound("PumpStart")
	}
}

func (m *Module) endMiss() {
	if m.inputting {
		m.ctx.Scene.PlayState(m.chicken, "Bomb", m.ctx.Beat(), 0.5)
	}
}

func (m *Module) blastOff(ev chargeEvt, state float64, missed bool) {
	m.inputting = false
	m.canBlastOff = false
	m.playerSucceeded = !missed
	m.ctx.Sound("SE_CHIKEN_CAR_START")
	m.ctx.Scene.PlayState(m.chicken, "Ride", m.ctx.Beat(), 0.5)
	m.yardsEditable = false
	_ = m.ctx.Assets.SetText(m.yardsText, "")
	m.ctx.Scene.SetActive(m.countBubble, false)
	m.current.idle(m.ctx.Beat())
	dur := math.Max(m.journeyLength, 0.5)
	offset := state * 1.03 * 1.3
	m.current.beginMove(m.ctx.Beat(), m.ctx.Beat()+dur, -m.journeyLength*platformDistance*platformsPerBeat-platformDistance*1.5-offset)
	m.next.beginMove(m.ctx.Beat(), m.ctx.Beat()+dur, -offset)
	m.ctx.At(m.journeyStartBeat+m.journeyLength, func() {
		m.current.moving = false
		m.next.moving = false
		m.look(m.ctx.Beat(), 2)
	})
}

func (m *Module) spawnJourney(beat, length float64) {
	m.current = m.next
	distance := length*platformDistance*platformsPerBeat + platformDistance*1.5
	m.next = newIsland(m, distance)
	m.next.inst.SetActive(relIsland(m.bigLandmass), true)
	m.next.inst.SetActive(relIsland(m.smallLandmass), false)
	m.next.collapse(beat+length, true)
	m.journeyStartBeat = beat
	m.journeyLength = length
}

func (m *Module) collapseUnderPlayer() {
	m.inputting = false
	m.current.collapse(m.ctx.Beat(), false)
	m.chickenFall(false)
}

func (m *Module) explode(beat, length float64) {
	if !m.inputting && length != 3 {
		return
	}
	m.inputting = false
	m.ctx.SoundPitchPan("SE_NTR_ROBOT_EN_BAKUHATU_PITCH100", 1, 1, 0)
	m.yardsEditable = false
	_ = m.ctx.Assets.SetText(m.yardsText, "")
	m.ctx.Scene.SetActive(m.countBubble, false)
	m.ctx.Scene.PlayState(m.chicken, "Gone", beat, 0.5)
	m.current.inst.PlayState(relIsland(m.fakeChicken), "Burn", beat, m.ctx.SecPerBeat(beat))
}

func (m *Module) chickenFall(fellTooFar bool) {
	if !fellTooFar {
		m.ctx.Scene.PlayState(m.chicken, "Fall", m.ctx.Beat(), 0.3)
		m.ctx.Sound("SE_CHIKEN_CAR_FALL")
		m.ctx.SoundAt(m.ctx.Beat()+0.6, "SE_CHIKEN_CAR_FALL_WATER", 0.5)
	}
	splashBeat := m.ctx.Beat() + 0.6
	m.ctx.At(splashBeat, func() {
		if m.current != nil {
			m.current.playParticle(splashBeat, m.chickenSplash)
		}
		m.ctx.Scene.PlayState(m.chicken, "Back", m.ctx.Beat(), 0.5)
	})
}

func (m *Module) look(beat, length float64) {
	m.successKillBeat = beat
	m.ctx.Scene.PlayState(m.chicken, "ChickenLookTo", beat, 0.499)
	m.ctx.At(beat+length, func() {
		m.ctx.Scene.PlayState(m.chicken, "ChickenLookFrom", beat+length, 0.5)
	})
}

func (m *Module) scheduleDrums(start, length float64, drum int) {
	if drum == 0 {
		return
	}
	loop := loopLength(drum)
	for b, remain := start, length; remain >= 0; b, remain = b+loop, remain-loop {
		for _, hit := range sortedDrumLoop(drum) {
			hit := hit
			if remain > hit.timing {
				at := b + hit.timing
				m.ctx.At(at, func() {
					if m.inputting && at >= m.drumSwitch {
						m.ctx.SoundVol(drumName(hit.typ), hit.vol*m.drumActualVolume())
					}
				})
			}
		}
	}
}

func (m *Module) drumActualVolume() float64 {
	if m.drumLoud {
		return 1
	}
	if m.drumVolume > m.drumTempVolume {
		return m.drumVolume
	}
	return m.drumTempVolume
}

func (m *Module) setYardsText() {
	txt := strings.ReplaceAll(m.yardsTemplate, "%", itoa(int(m.yardsLength)))
	_ = m.ctx.Assets.SetText(m.yardsText, stripRichText(txt))
	m.yardsEditable = true
}

func (m *Module) textEdit(text string, col [4]float64) {
	_ = col
	m.yardsTemplate = strings.ReplaceAll(text, "#", "%")
	if m.yardsEditable {
		m.setYardsText()
	}
}

func stripRichText(s string) string {
	s = strings.ReplaceAll(s, "<color=#FFFF00>", "")
	s = strings.ReplaceAll(s, "</color>", "")
	return s
}

func (m *Module) setEndText(ev chargeEvt) {
	switch ev.endText {
	case 1:
		if m.playerSucceeded {
			_ = m.ctx.Assets.SetText(m.endingText, ev.success)
		} else {
			_ = m.ctx.Assets.SetText(m.endingText, ev.fail)
		}
	case 2:
		_ = m.ctx.Assets.SetText(m.endingText, destinationText(ev.destination, ev.customDestination))
	default:
		return
	}
	m.ctx.At(m.ctx.Beat()+float64(ev.textLen), func() { _ = m.ctx.Assets.SetText(m.endingText, "") })
}

func (m *Module) bubbleShrink(beat, length float64, grows, instant bool) {
	if instant {
		m.ctx.Scene.SetActive(m.countBubble, grows && m.inputting)
		m.ctx.Scene.SetScaleOver(m.countBubble, m.bubbleScale, m.bubbleScale)
		return
	}
	if grows {
		m.ctx.Scene.SetActive(m.countBubble, m.inputting)
	}
	m.bubbleA, m.bubbleB, m.bubbleGrow = beat, beat+length, grows
	m.ctx.At(beat+length, func() {
		if !grows {
			m.ctx.Scene.SetActive(m.countBubble, false)
			m.ctx.Scene.SetScaleOver(m.countBubble, m.bubbleScale, m.bubbleScale)
		}
	})
}

func (m *Module) applyBubble(beat float64) {
	if m.bubbleEnd > 0 {
		left := math.Ceil(m.bubbleEnd - beat - 1)
		if left < 0 {
			left = 0
		}
		_ = m.ctx.Assets.SetText(m.bubbleText, itoa(int(left)))
	}
	if m.bubbleB > m.bubbleA && beat >= m.bubbleA && beat <= m.bubbleB {
		u := clamp01((beat - m.bubbleA) / (m.bubbleB - m.bubbleA))
		scale := lerp(m.bubbleScale, 0, u)
		if m.bubbleGrow {
			scale = m.bubbleScale - scale
		}
		m.ctx.Scene.SetScaleOver(m.countBubble, scale, scale)
	}
}

func (m *Module) musicFade(beat, length float64, fadeIn, instant, drums, reset bool) {
	m.drumFadeStart = beat
	m.drumFadeLength = length
	if instant {
		m.drumFadeLength = 0
	}
	m.drumFadeIn = fadeIn
	m.drumReset = reset
	m.drumLoud = !drums
	m.ctx.FadeMusicVolume(beat, m.drumFadeLength, mapBool(fadeIn, 1, 0))
}

func mapBool(cond bool, a, b float64) float64 {
	if cond {
		return a
	}
	return b
}

func (m *Module) parallaxObjects(e *riq.Entity) {
	length := math.Max(e.Length, 0.001)
	speed := 0.5 / length
	for _, spec := range []struct {
		key, in, out, en, dis string
		layer                 int
	}{
		{"stars", "StarsIn", "StarsOut", "StarsEnable", "StarsDisable", 0},
		{"clouds", "CloudIn", "CloudOut", "CloudEnable", "CloudDisable", 1},
		{"earth", "EarthIn", "EarthOut", "EarthEnable", "EarthDisable", 2},
		{"mars", "MarsIn", "MarsOut", "MarsEnable", "MarsDisable", 3},
		{"doodles", "DoodlesIn", "DoodlesOut", "DoodlesEnable", "DoodlesDisable", 4},
		{"birds", "BirdsIn", "BirdsOut", "BirdsEnable", "BirdsDisable", 5},
	} {
		on := boolParam(e, spec.key)
		state := spec.out
		if on {
			state = spec.in
		}
		if boolParam(e, "instant") {
			state = mapBoolString(on, spec.en, spec.dis)
		}
		m.ctx.Scene.PlayStateLayer("chargingChicken/parallax/"+spec.key, m.parallax, state, e.Beat, speed)
	}
	if boolParam(e, "birds") {
		m.ctx.Scene.PlayState(m.birdsAnim, "BirdsFly", e.Beat, 0.5)
	}
}

func mapBoolString(cond bool, a, b string) string {
	if cond {
		return a
	}
	return b
}

func (m *Module) unParallaxObjects(e *riq.Entity) {
	bg := intDefault(e, "appearance", 0)
	length := math.Max(e.Length, 0.001)
	speed := 0.5 / length
	instant := boolParam(e, "instant")
	switch bg {
	case 1:
		m.ctx.Scene.PlayStateLayer("chargingChicken/unparallax/galaxy", m.unparallax, mapBoolString(instant, "GalaxyEnable", "GalaxyIn"), e.Beat, speed)
		m.ctx.Scene.PlayStateLayer("chargingChicken/unparallax/future", m.unparallax, mapBoolString(instant, "FutureDisable", "FutureOut"), e.Beat, speed)
	case 2:
		m.ctx.Scene.PlayStateLayer("chargingChicken/unparallax/future", m.unparallax, mapBoolString(instant, "FutureEnable", "FutureIn"), e.Beat, speed)
		m.ctx.Scene.PlayStateLayer("chargingChicken/unparallax/galaxy", m.unparallax, mapBoolString(instant, "GalaxyDisable", "GalaxyOut"), e.Beat, speed)
	default:
		m.ctx.Scene.PlayStateLayer("chargingChicken/unparallax/galaxy", m.unparallax, mapBoolString(instant, "GalaxyDisable", "GalaxyOut"), e.Beat, speed)
		m.ctx.Scene.PlayStateLayer("chargingChicken/unparallax/future", m.unparallax, mapBoolString(instant, "FutureDisable", "FutureOut"), e.Beat, speed)
	}
}

func (m *Module) parallaxProgress(e *riq.Entity) {
	if v := e.Float("starProgress", -1); v >= 0 {
		m.ctx.Scene.SetPosOver(m.stars, (-v-50)*0.32, 0)
	}
	if v := e.Float("cloudProgress", -1); v >= 0 {
		m.ctx.Scene.SetPosOver(m.clouds, -v*0.24, 0)
	}
	if v := e.Float("planetProgress", -1); v >= 0 {
		m.ctx.Scene.SetPosOver(m.planets, -v*0.30, 0)
	}
	if v := e.Float("doodleProgress", -1); v >= 0 {
		m.ctx.Scene.SetPosOver(m.doodles, -v*0.315, 0)
	}
	if v := e.Float("birdProgress", -1); v >= 0 {
		m.ctx.Scene.SetPosOver(m.birds, -v*0.25, 0)
	}
}

func (m *Module) applyParallax() {
	m.ctx.Scene.SetPosOver(m.clouds, mod(m.parallaxX*0.6, 24), 0)
	m.ctx.Scene.SetPosOver(m.stars, mod(m.parallaxX*0.3, 32), 0)
	m.ctx.Scene.SetPosOver(m.planets, mod(m.parallaxX*0.6, 30), 0)
	m.ctx.Scene.SetPosOver(m.doodles, mod(m.parallaxX*0.6, 31.5), 0)
	m.ctx.Scene.SetPosOver(m.birds, mod(m.parallaxX*0.67, 25), 0)
}

func (m *Module) applyColors(beat float64) {
	bg := currentColor(m.bgCols, beat, defaultBGTop, defaultBGBottom)
	m.ctx.Scene.SetColorOver(m.bgHigh, bg[0])
	m.ctx.Scene.SetColorOver(m.gradient, bg[0])
	m.ctx.Scene.SetColorOver(m.bgLow, bg[1])
	car := currentColor(m.carCols, beat, defaultCar, defaultCarCharge)
	cloud := currentColor(m.cloudCols, beat, defaultCloud, defaultCloud2)
	light := currentColor(m.lightCols, beat, defaultLight, [4]float64{1, 1, 1, 0})
	brightness := light[0][0]
	pal := kart.Palette{
		Alpha: [4]float64{brightness, brightness, brightness, 1}, Fill: car[0], Outline: car[1],
		Progress: m.carChargeProgress(beat), UseProgress: true,
	}
	m.ctx.Scene.SetPaletteFor(m.carMat, pal)
	m.ctx.Scene.SetPaletteFor(m.chickenMat, kart.Palette{Alpha: [4]float64{brightness, brightness, brightness, 1}, Fill: defaultCloud, Outline: defaultCloud})
	m.ctx.Scene.SetPaletteFor(m.waterMat, kart.Palette{Alpha: [4]float64{brightness, brightness, brightness, 1}, Fill: defaultCloud, Outline: defaultCloud})
	m.ctx.Scene.SetPaletteFor(m.cloudMat, kart.Palette{Alpha: cloud[0], Fill: cloud[1], Outline: defaultCloud})
	m.ctx.Scene.SetColorOver(m.headlight, light[1])
}

func (m *Module) carChargeProgress(beat float64) float64 {
	if !m.inputting || m.yardsLength <= 0 {
		return 0
	}
	// ChargingChicken.cs writes chickenColorsCar._Progress from
	// GetPositionFromBeat(nextInputReady - yardsTextLength*2, yardsTextLength)
	// while the player is holding the charge.
	return clamp01((beat - (m.nextInputReady - m.yardsLength*2)) / m.yardsLength)
}

func currentColor(list []colorEvt, beat float64, defA, defB [4]float64) [2][4]float64 {
	out := [2][4]float64{defA, defB}
	for _, ev := range list {
		if beat+1e-6 < ev.beat {
			continue
		}
		out[0] = colorEaseAt(ev.beat, ev.length, ev.a0, ev.a1, beat)
		out[1] = colorEaseAt(ev.beat, ev.length, ev.b0, ev.b1, beat)
	}
	return out
}
