// Package manzai ports Rhythm Heaven Fever's Manzai runtime.
package manzai

import (
	"fmt"
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"

	"hsdemo/engine"
	"hsdemo/kart"
	"hsdemo/riq"
)

const (
	actionBasic = 0
	actionAlt   = 3 // HS InputAction_Alt maps to pad South; engine channel L/Down/X.
)

const (
	whoKasuke = iota
	whoKosuke
	whoBoth
)

const (
	sideInside = iota
	sideOutside
)

const (
	crowdDefault = iota
	crowdPractice
	crowdSilent
)

const (
	crowdIdle = iota
	crowdBop
	crowdCheer
	crowdUproar
	crowdAngry
	crowdJump
)

const (
	defaultPun = 4
)

type bopEvt struct {
	beat, length float64
	who          int
	bop, auto    bool
}

type punEvt struct {
	beat, length   float64
	pun            int
	pitched, boing bool
	crowd          int
}

type featherParticle struct {
	born       float64
	x, y       float64
	vx, vy     float64
	size, spin float64
	life       float64
	sprite     string
	tint       [4]float64
}

type punDef struct {
	name           string
	short          bool
	boingSyllables int
}

var punDefs = map[int]punDef{
	0:  {name: "AichiniAichinna"},
	1:  {name: "AmmeteAmena"},
	4:  {name: "FutongaFuttonda"},
	13: {name: "MikangaMikannai"},
	15: {name: "OkanewaOkkane"},
	20: {name: "RakudawaRakugana"},
	24: {name: "SarugaSaru", short: true, boingSyllables: 3},
	34: {name: "Muted"},
}

var randomPunValues = []int{0, 1, 4, 13, 15, 20, 24}

type Module struct {
	ctx  *engine.Ctx
	proj kart.Aff

	stage, birds, raven, vulture string
	haiL, haiR, donaiyanen       string
	pivotL, pivotR, pivotD       string
	crowd                        string

	bops []bopEvt
	puns []punEvt

	ravenCanBop, vultureCanBop         bool
	ravenCanBopTemp, vultureCanBopTemp bool
	canDodge                           bool

	isMoving         bool
	movingStartBeat  float64
	movingLength     float64
	moveAnim         string
	moveEase         int
	lastPulse        float64
	lastWhiffBeat    float64
	randomBubbleBoth float64

	isPreparingForBoing bool
	missedWrongButton   bool
	boingHasCrowdSounds bool

	hitHaiL, hitHaiR, hitDonaiyanen bool

	crowdCanCheerSound    bool
	crowdCanCheerAnim     bool
	crowdIsCheering       bool
	crowdLastMissAnimBeat float64
	startJumpBeat         float64
	jumpLength            float64
	jumpHeight            float64

	altTargets []float64

	featherBase [2]float64
	particles   []featherParticle
}

func New() engine.Module {
	return &Module{
		ravenCanBop:           true,
		vultureCanBop:         true,
		ravenCanBopTemp:       true,
		vultureCanBopTemp:     true,
		canDodge:              true,
		boingHasCrowdSounds:   true,
		crowdCanCheerSound:    true,
		crowdCanCheerAnim:     true,
		lastPulse:             math.Inf(-1),
		lastWhiffBeat:         math.Inf(-1),
		crowdLastMissAnimBeat: math.Inf(-1),
		startJumpBeat:         math.Inf(-1),
		jumpHeight:            1,
	}
}

func (m *Module) ID() string { return "manzai" }

func (m *Module) Load(ctx *engine.Ctx) error {
	m.ctx = ctx
	if err := ctx.LoadAssets("manzai"); err != nil {
		return err
	}
	m.proj = kart.Translate(engine.ScreenW/2, engine.ScreenH/2).Mul(kart.Scale(54, -54))

	m.stage = "BG"
	m.birds = "Birds"
	m.raven = "Birds/RavenHolder/Raven"
	m.vulture = "Birds/VultureHolder/Vulture"
	m.haiL = "Birds/RavenHolder/Bubbles/PivotL/SpeechL"
	m.haiR = "Birds/RavenHolder/Bubbles/PivotR/SpeechR"
	m.donaiyanen = "Birds/RavenHolder/Bubbles/PivotD/SpeechD"
	m.pivotL = "Birds/RavenHolder/Bubbles/PivotL"
	m.pivotR = "Birds/RavenHolder/Bubbles/PivotR"
	m.pivotD = "Birds/RavenHolder/Bubbles/PivotD"
	m.crowd = "Crowd"
	m.featherBase = nodeWorldPos(ctx.Assets, "Birds/VultureHolder/Vulture/Feathers")
	return nil
}

func (m *Module) OnEvent(e *riq.Entity) {
	switch e.Datamodel {
	case "manzai/bop":
		m.bops = append(m.bops, bopEvt{
			beat:   e.Beat,
			length: e.Length,
			who:    int(e.Float("who", whoBoth)),
			bop:    boolParamDefault(e, "bop", true),
			auto:   boolParamDefault(e, "auto", false),
		})
	case "manzai/pun", "manzai/boing":
		length := e.Length
		if length <= 0 {
			length = 4
		}
		pun := int(e.Float("pun", defaultPun))
		if boolParamDefault(e, "random", true) {
			idx := int(math.Floor(eventRand(e.Beat, 104) * float64(len(randomPunValues))))
			if idx >= len(randomPunValues) {
				idx = len(randomPunValues) - 1
			}
			pun = randomPunValues[idx]
		}
		ev := punEvt{
			beat:    e.Beat,
			length:  length,
			pun:     pun,
			pitched: boolParamDefault(e, "pitch", true),
			boing:   e.Datamodel == "manzai/boing",
			crowd:   int(e.Float("crowd", crowdDefault)),
		}
		m.puns = append(m.puns, ev)
		m.schedulePun(ev)
	case "manzai/slide":
		beat, length := e.Beat, e.Length
		side := int(e.Float("goToSide", sideOutside))
		ease := int(e.Float("ease", 3))
		anim := boolParamDefault(e, "animation", true)
		m.ctx.At(beat, func() { m.birdsSlide(beat, length, side, ease, anim) })
	case "manzai/lights":
		on := boolParamDefault(e, "lightsEnabled", false)
		m.ctx.At(e.Beat, func() { m.toggleLights(on, e.Beat) })
	case "manzai/crowd":
		beat, length := e.Beat, e.Length
		anim := int(e.Float("animation", crowdBop))
		loop := int(e.Float("loop", 4))
		m.scheduleCrowdAnimation(beat, length, anim, loop)
	}
}

func (m *Module) Ready() {
	for _, ev := range m.bops {
		ev := ev
		if ev.bop {
			for i := 0; float64(i) < ev.length; i++ {
				b := ev.beat + float64(i)
				if ev.who == whoKasuke || ev.who == whoBoth {
					m.ctx.At(b, func() { m.bopRaven(b) })
				}
				if ev.who == whoKosuke || ev.who == whoBoth {
					m.ctx.At(b, func() { m.bopVulture(b) })
				}
			}
		}
	}
}

func (m *Module) OnSwitch(beat float64) {
	if !m.hasActivePun(beat) {
		for _, path := range []string{m.stage, m.birds, m.raven, m.vulture, m.haiL, m.haiR, m.donaiyanen, m.crowd} {
			m.ctx.Scene.PlayDefaultState(path, beat, m.ctx.SecPerBeat(beat))
		}
	}
	m.ravenCanBop, m.vultureCanBop = true, true
	m.ravenCanBopTemp, m.vultureCanBopTemp = true, true
	m.canDodge = true
	m.lastPulse = math.Floor(beat)
}

func (m *Module) Whiff(beat float64) { m.WhiffAction(beat, actionBasic) }

func (m *Module) WhiffAction(beat float64, action int) {
	switch action {
	case actionBasic:
		if m.altExpectedNow() {
			if m.boingHasCrowdSounds {
				m.ctx.Sound("missWrongButton")
			} else {
				m.ctx.Sound("missClick")
			}
			m.kasukeHaiAnim(beat)
			m.missedWrongButton = true
			m.crowdAngryNow(beat)
			return
		}
		m.kasukeHaiAnim(beat)
		m.lastWhiffBeat = beat
		m.ravenCanBopTemp = false
		m.crowdAngryNow(beat)
	case actionAlt:
		m.ctx.Sound("miss2")
		m.play(m.raven, "Spin", beat)
		m.lastWhiffBeat = beat
		m.ravenCanBopTemp = false
		if m.canDodge {
			m.play(m.vulture, "Dodge", beat)
			m.vultureCanBopTemp = false
		}
		m.crowdAngryNow(beat)
	}
}

func (m *Module) Update(t, beat float64) {
	if p := math.Floor(beat); p > m.lastPulse {
		m.lastPulse = p
		m.autoBop(p)
	}
	if m.isMoving {
		u := clamp01((beat - m.movingStartBeat) / math.Max(m.movingLength, 1e-6))
		m.ctx.Scene.PlayFrozen(m.birds, m.moveAnim, engine.Ease(m.moveEase, 0, 1, u))
		if u >= 1 {
			m.isMoving = false
		}
	}
	if beat > m.lastWhiffBeat+1 {
		m.ravenCanBopTemp = true
		m.vultureCanBopTemp = true
	}
	if !m.canDodge {
		m.vultureCanBopTemp = true
	}
	m.updateCrowdJump(beat)
	m.updateParticles(t)
}

func (m *Module) Draw(screen *ebiten.Image, _, beat float64) {
	screen.Fill(color.RGBA{0x72, 0x00, 0x3d, 0xff})
	m.ctx.SampleScene(beat)
	m.queueParticles()
	m.ctx.Scene.Draw(screen, m.proj)
}

func (m *Module) schedulePun(ev punEvt) {
	m.schedulePunSounds(ev)
	m.ctx.At(ev.beat, func() { m.doPun(ev, true) })
}

func (m *Module) doPun(ev punEvt, registerTargets bool) {
	m.crowdCanCheerSound = true
	m.boingHasCrowdSounds = true
	if ev.boing {
		m.doPunBoing(ev, registerTargets)
	} else {
		m.doPunHai(ev, registerTargets)
	}
	m.ctx.At(ev.beat+4, func() { m.crowdAnimReset(ev.beat + 4) })
}

func (m *Module) doPunHai(ev punEvt, registerTargets bool) {
	m.vultureCanBop = false
	m.canDodge = false
	def := punDefs[ev.pun]
	firstLeft := eventRand(ev.beat, 1) < 0.5
	if registerTargets {
		m.scheduleHaiInput(ev, ev.beat+2.5, firstLeft, ev.pitched)
		m.scheduleHaiInput(ev, ev.beat+3.0, !firstLeft, ev.pitched)
	}
	if ev.crowd == crowdPractice {
		pitch, offset := m.pitched(ev.beat, ev.pitched, 0.03)
		m.ctx.SoundAtPitchOff(ev.beat+2.5, "crowdHai1", 0.7, pitch, offset)
		m.ctx.SoundAtPitchOff(ev.beat+3.0, "crowdHai2", 0.7, pitch, offset)
	}

	m.vultureTalks(ev.beat, def.short)
	m.ctx.At(ev.beat+2.25, func() {
		m.ravenCanBop = false
		m.canDodge = true
		m.randomBubbleBoth = eventRand(ev.beat, 2)*0.2 - 0.1
		m.crowdCanCheerSound = ev.crowd == crowdDefault
	})
	m.ctx.At(ev.beat+3.25, func() {
		m.vultureCanBop = true
		m.ravenCanBop = true
		m.audienceRespond(ev)
	})
}

func (m *Module) doPunBoing(ev punEvt, registerTargets bool) {
	m.vultureCanBop = false
	m.canDodge = false
	def := punDefs[ev.pun]
	syllables := boingSyllables(def)
	if registerTargets {
		target := ev.beat + 2.5
		m.altTargets = append(m.altTargets, target)
		m.ctx.ScheduleInputAction(target, actionAlt, func(state float64, j engine.Judgment) {
			m.boingJust(target, state, j, ev.pitched)
		}, func() {
			if ev.crowd == crowdDefault {
				m.boingMiss(ev.beat + 2.5)
			}
		})
	}
	if ev.crowd == crowdPractice {
		pitch, offset := m.pitched(ev.beat, ev.pitched, 0.03)
		for _, snd := range []struct {
			name string
			off  float64
		}{
			{"crowdDon1", 0}, {"crowdDon2", 0.25}, {"crowdDon3", 0.75}, {"crowdDon4", 1.0},
		} {
			off := 0.0
			if snd.off == 0 {
				off = offset
			}
			m.ctx.SoundAtPitchOff(ev.beat+2.5+snd.off, snd.name, 0.7, pitch, off)
		}
	}

	m.vultureTalks(ev.beat, def.short)
	m.ctx.At(ev.beat+0.5, func() { m.isPreparingForBoing = true })
	boingBeat := ev.beat + 1.25
	dodgeBeat := ev.beat + 1.5
	if syllables == 6 {
		boingBeat = ev.beat + 1.5
		dodgeBeat = ev.beat + 1.75
	}
	m.ctx.At(boingBeat, func() { m.play(m.vulture, "Boing", boingBeat) })
	m.ctx.At(dodgeBeat, func() { m.canDodge = true })
	m.ctx.At(ev.beat+2, func() {
		m.ravenCanBop = false
		m.slapReady(ev.beat + 2)
	})
	m.ctx.At(ev.beat+2.25, func() {
		m.boingHasCrowdSounds = ev.crowd == crowdDefault
		m.crowdCanCheerSound = ev.crowd == crowdDefault
	})
	m.ctx.At(ev.beat+3.0, func() { m.audienceRespond(ev) })
	m.ctx.At(ev.beat+3.2, func() { m.isPreparingForBoing = false })
	m.ctx.At(ev.beat+3.25, func() {
		m.vultureCanBop = true
		m.ravenCanBop = true
	})
}

func (m *Module) schedulePunSounds(ev punEvt) {
	def := punDefs[ev.pun]
	pitch, offset := m.pitched(ev.beat, ev.pitched, 0.05)
	syllables := 9
	if ev.boing {
		syllables = boingSyllables(def)
	}
	for i := 0; i < syllables; i++ {
		off := 0.0
		if i == 0 {
			off = offset
		}
		m.ctx.SoundAtPitchOff(ev.beat+float64(i)*0.25, fmt.Sprintf("%s%d", def.name, i+1), 1, pitch, off)
	}
	if ev.boing {
		b := ev.beat + 1.25
		if syllables == 6 {
			b = ev.beat + 1.5
		}
		m.ctx.SoundAtPitchOff(b, "boing", 0.8, pitch, 0)
		m.ctx.SoundAtPitchOff(b, "comedy", 0.8, pitch, 0)
	}
}

func (m *Module) scheduleHaiInput(ev punEvt, target float64, left bool, pitched bool) {
	m.ctx.ScheduleInput(target, func(state float64, j engine.Judgment) {
		side := 1
		if left {
			side = 0
		}
		m.haiJust(target, state, j, side, pitched)
	}, func() {
		if ev.crowd == crowdDefault {
			m.haiMiss(target)
		}
	})
}

func (m *Module) haiJust(beat, state float64, j engine.Judgment, side int, pitched bool) {
	pitch, _ := m.pitched(beat, pitched, 0)
	m.ctx.SoundPitch("hai", 1, pitch)
	if j == engine.JudgeNG || math.Abs(state) >= 1 {
		m.ctx.Sound("miss1")
		m.ctx.Sound("missClick")
		m.crowdAngryNow(beat)
	} else {
		m.ctx.Sound("haiAccent")
		if m.crowdCanCheerSound && m.crowdCanCheerAnim && !m.crowdIsCheering {
			m.play(m.crowd, "Cheer", beat)
			m.crowdIsCheering = true
		}
		if side == 0 {
			m.ctx.Scene.SetSpinOver(m.pivotL, m.randomBubbleBoth+eventRand(beat, 3)*0.08-0.04)
			m.play(m.haiL, "HaiL", beat)
			m.hitHaiL = true
		} else {
			m.ctx.Scene.SetSpinOver(m.pivotR, m.randomBubbleBoth+eventRand(beat, 4)*0.08-0.04)
			m.play(m.haiR, "HaiR", beat)
			m.hitHaiR = true
		}
	}
	m.play(m.raven, "Talk", beat)
	m.play(m.vulture, "Bop", beat)
}

func (m *Module) haiMiss(beat float64) {
	m.ctx.Sound("disappointed")
	m.crowdAngryNow(beat)
}

func (m *Module) boingJust(beat, state float64, j engine.Judgment, pitched bool) {
	pitch, _ := m.pitched(beat, pitched, 0)
	for _, snd := range []struct {
		name string
		off  float64
	}{{"donaiyanen1", 0}, {"donaiyanen2", 0.25}, {"donaiyanen3", 0.75}, {"donaiyanen4", 1.0}} {
		m.ctx.SoundAtPitchOff(beat+snd.off, snd.name, 1, pitch, 0)
	}
	if j == engine.JudgeNG || math.Abs(state) >= 1 {
		m.ctx.Sound("miss1")
		m.ctx.Sound("missClick")
		m.crowdAngryNow(beat)
	} else {
		m.ctx.Sound("donaiyanenAccent")
		if m.crowdCanCheerSound && m.crowdCanCheerAnim {
			m.play(m.crowd, "Uproar", beat)
			m.crowdIsCheering = true
		}
		m.spawnFeathers(beat)
		m.hitDonaiyanen = true
	}
	m.play(m.raven, "Attack", beat)
	m.play(m.vulture, "Damage", beat)
	m.ctx.Scene.SetSpinOver(m.pivotD, eventRand(beat, 5)*0.4-0.2)
	m.ctx.Scene.SetPosOver(m.pivotD, eventRand(beat, 6)*3.4-1.5, eventRand(beat, 7)+1)
	m.play(m.donaiyanen, "Donaiyanen", beat)
}

func (m *Module) boingMiss(beat float64) {
	if !m.missedWrongButton {
		m.ctx.Sound("disappointed")
	} else {
		m.missedWrongButton = false
	}
	m.crowdAngryNow(beat)
}

func (m *Module) audienceRespond(ev punEvt) {
	if ev.crowd == crowdDefault {
		if m.hitHaiL && m.hitHaiR {
			m.ctx.SoundAt(ev.beat+3.5, "haiClap", 1)
		}
		if m.hitDonaiyanen {
			m.ctx.SoundAt(ev.beat+3.0, "donaiyanenLaugh", 1)
		}
		m.crowdCanCheerSound = false
	}
	m.hitHaiL, m.hitHaiR, m.hitDonaiyanen = false, false, false
}

func (m *Module) birdsSlide(beat, length float64, side, ease int, animation bool) {
	m.vultureCanBop = false
	m.ravenCanBop = false
	if animation {
		m.play(m.raven, "Move", beat)
		m.play(m.vulture, "Move", beat)
	}
	m.movingStartBeat = beat
	m.movingLength = length
	if side == sideInside {
		m.moveAnim = "SlideIn"
		m.canDodge = true
	} else {
		m.moveAnim = "SlideOut"
		m.canDodge = false
	}
	m.moveEase = ease
	m.isMoving = true
	m.ctx.At(beat+0.75, func() {
		m.vultureCanBop = true
		m.ravenCanBop = true
	})
}

func (m *Module) toggleLights(on bool, beat float64) {
	if on {
		m.play(m.stage, "LightsOff", beat)
		return
	}
	m.play(m.stage, "LightsOn", beat)
}

func (m *Module) scheduleCrowdAnimation(beat, length float64, anim, loop int) {
	loopBeat := float64(loop) * 0.25
	if loopBeat <= 0 {
		loopBeat = 1
	}
	if anim != crowdBop {
		m.ctx.At(beat-0.25, func() { m.crowdCanCheerAnim = false })
	}
	if anim != crowdBop && anim != crowdJump {
		m.ctx.At(beat, func() { m.doCrowdAnimation(anim, loopBeat, beat) })
		m.ctx.At(beat+length, func() { m.play(m.crowd, "Idle", beat+length) })
	} else {
		for i := 0; float64(i)*loopBeat < length; i++ {
			b := beat + float64(i)*loopBeat
			// Source passes the original beat into doCrowdAnimation for Jump;
			// preserving that quirk keeps jump arcs aligned with the C# port.
			m.ctx.At(b, func() { m.doCrowdAnimation(anim, loopBeat, beat) })
		}
	}
	m.ctx.At(beat+length, func() { m.crowdCanCheerAnim = true })
}

func (m *Module) doCrowdAnimation(anim int, loop, beat float64) {
	switch anim {
	case crowdBop:
		if !m.crowdIsCheering && m.crowdLastMissAnimBeat+2 < m.ctx.Beat() {
			m.play(m.crowd, "Bop", m.ctx.Beat())
		}
	case crowdJump:
		m.jumpHeight = math.Min(loop, 2)
		m.jumpLength = loop
		m.startJumpBeat = beat
	default:
		m.play(m.crowd, crowdState(anim), m.ctx.Beat())
	}
}

func (m *Module) crowdAnimReset(beat float64) {
	if m.crowdIsCheering {
		m.play(m.crowd, "Idle", beat)
		m.crowdIsCheering = false
	}
}

func (m *Module) crowdAngryNow(beat float64) {
	if m.crowdCanCheerAnim {
		m.play(m.crowd, "Angry", beat)
		m.crowdIsCheering = false
		m.crowdLastMissAnimBeat = beat
	}
}

func (m *Module) updateCrowdJump(beat float64) {
	if math.IsInf(m.startJumpBeat, -1) || m.jumpLength <= 0 {
		return
	}
	u := (beat - m.startJumpBeat) / m.jumpLength
	if u < 0 || u > 1 {
		m.ctx.Scene.ClearPosOver(m.crowd)
		return
	}
	yMul := u*2 - 1
	y := m.jumpHeight * (-(yMul * yMul) + 1)
	m.ctx.Scene.SetPosOver(m.crowd, 0, y)
}

func (m *Module) autoBop(beat float64) {
	for _, ev := range m.bops {
		if !ev.auto || beat < ev.beat || beat >= ev.beat+ev.length {
			continue
		}
		if ev.who == whoKasuke || ev.who == whoBoth {
			m.bopRaven(beat)
		}
		if ev.who == whoKosuke || ev.who == whoBoth {
			m.bopVulture(beat)
		}
	}
}

func (m *Module) bopRaven(beat float64) {
	if m.ravenCanBop && m.ravenCanBopTemp {
		m.play(m.raven, "Bop", beat)
	}
}

func (m *Module) bopVulture(beat float64) {
	if m.vultureCanBop && m.vultureCanBopTemp {
		m.play(m.vulture, "Bop", beat)
	}
}

func (m *Module) vultureTalks(beat float64, short bool) {
	for _, off := range []float64{0, 0.5} {
		o := off
		m.ctx.At(beat+o, func() { m.play(m.vulture, "Talk", beat+o) })
	}
	if short {
		return
	}
	m.ctx.At(beat+1.0, func() { m.play(m.vulture, "Talk", beat+1.0) })
	m.ctx.At(beat+1.5, func() { m.play(m.vulture, "Talk", beat+1.5) })
}

func (m *Module) slapReady(beat float64) { m.play(m.raven, "Ready", beat) }

func (m *Module) kasukeHaiAnim(beat float64) {
	m.ctx.Sound("hai")
	m.play(m.raven, "Talk", beat)
	m.play(m.vulture, "Bop", beat)
}

func (m *Module) play(path, state string, beat float64) {
	m.ctx.Scene.PlayState(path, state, beat, 0.5)
}

func (m *Module) pitched(beat float64, enabled bool, baseOffset float64) (float64, float64) {
	if !enabled {
		return 1, baseOffset
	}
	ratio := m.ctx.BPMAt(beat) / 98
	return ratio, baseOffset / ratio
}

func (m *Module) altExpectedNow() bool {
	now := m.ctx.Time()
	for _, target := range m.altTargets {
		t := m.ctx.BeatToTime(target)
		if now >= t-engine.WinNG && now <= t+engine.WinNG {
			return true
		}
	}
	return false
}

func (m *Module) hasActivePun(beat float64) bool {
	for _, ev := range m.puns {
		if beat >= ev.beat && beat < ev.beat+ev.length {
			return true
		}
	}
	return false
}

func (m *Module) spawnFeathers(beat float64) {
	born := m.ctx.BeatToTime(beat)
	base := m.featherBase
	for i := 0; i < 12; i++ {
		r0 := eventRand(beat, 20+i*4)
		r1 := eventRand(beat, 21+i*4)
		r2 := eventRand(beat, 22+i*4)
		r3 := eventRand(beat, 23+i*4)
		angle := -0.35 + r0*math.Pi*1.15
		speed := 1.0 + r1*1.6
		m.particles = append(m.particles, featherParticle{
			born:   born,
			x:      base[0],
			y:      base[1],
			vx:     math.Cos(angle) * speed,
			vy:     math.Sin(angle)*speed + 1.1,
			size:   0.10 + r2*0.09,
			spin:   (r3*2 - 1) * 8,
			life:   0.34 + r1*0.18,
			sprite: "Comedians_8",
			tint:   [4]float64{1, 1, 1, 1},
		})
	}
}

func (m *Module) updateParticles(t float64) {
	out := m.particles[:0]
	for _, p := range m.particles {
		if t-p.born <= p.life {
			out = append(out, p)
		}
	}
	m.particles = out
}

func (m *Module) queueParticles() {
	t := m.ctx.Time()
	for _, p := range m.particles {
		u := clamp01((t - p.born) / p.life)
		alpha := 1 - u
		x := p.x + p.vx*u
		y := p.y + p.vy*u - 1.8*u*u
		world := kart.Translate(x, y).Mul(kart.Rotate(p.spin * u)).Mul(kart.Scale(p.size*(1+0.5*u), p.size*(1+0.5*u)))
		tint := p.tint
		tint[3] = alpha
		m.ctx.Scene.Queue(kart.ExtraSprite{Sprite: p.sprite, World: world, Layer: 0, Order: 65, Tint: tint})
	}
}

func boingSyllables(def punDef) int {
	if def.boingSyllables != 0 {
		return def.boingSyllables
	}
	return 4
}

func crowdState(anim int) string {
	switch anim {
	case crowdIdle:
		return "Idle"
	case crowdCheer:
		return "Cheer"
	case crowdUproar:
		return "Uproar"
	case crowdAngry:
		return "Angry"
	default:
		return "Bop"
	}
}

func boolParamDefault(e *riq.Entity, key string, def bool) bool {
	fallback := 0.0
	if def {
		fallback = 1
	}
	return e.Float(key, fallback) != 0
}

func eventRand(beat float64, salt int) float64 {
	x := math.Sin(beat*12.9898+float64(salt)*78.233) * 43758.5453
	return x - math.Floor(x)
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

func nodeWorldPos(as *kart.Assets, path string) [2]float64 {
	idx := -1
	for i, n := range as.Rig.Nodes {
		if n.Path == path {
			idx = i
			break
		}
	}
	if idx < 0 {
		return [2]float64{}
	}
	var chain []int
	for i := idx; i >= 0; i = as.Rig.Nodes[i].Parent {
		chain = append(chain, i)
	}
	x, y := 0.0, 0.0
	sx, sy := 1.0, 1.0
	for i := len(chain) - 1; i >= 0; i-- {
		n := as.Rig.Nodes[chain[i]]
		x += n.Pos[0] * sx
		y += n.Pos[1] * sy
		sx *= n.Scale[0]
		sy *= n.Scale[1]
	}
	return [2]float64{x, y}
}
