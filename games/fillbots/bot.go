package fillbots

import (
	"math"

	"github.com/hajimehoshi/ebiten/v2"

	"hsdemo/engine"
	"hsdemo/kart"
)

type bot struct {
	mod  *Module
	inst *kart.Instance
	spec botSpec

	size       botSize
	startBeat  float64
	holdLength float64
	end        endAnim
	altOK      bool

	state botState

	startX, startY        float64
	lerpDistance          float64
	lerpIdleDistance      float64
	conveyerStartBeat     float64
	conveyerLength        float64
	conveyerRestartLength float64

	stack     bool
	stackBeat float64
	stackLen  float64

	legsFallen bool
	bodyFallen bool
	headFallen bool

	fuel, lampOff, lampOn [4]float64
	fill                  float64
	exploded              bool
	releaseResolved       bool
	beepNext              float64
	stopWater             func()

	dead   bool
	deadAt float64
}

func newBot(m *Module, ev spawnEvt, tmpl *kart.Template, spec botSpec) *bot {
	in := tmpl.NewInstance()
	b := &bot{
		mod: m, inst: in, spec: spec,
		size: ev.size, startBeat: ev.beat, holdLength: ev.holdLength, end: ev.end, altOK: ev.altOK,
		startX: in.Offset[0], startY: in.Offset[1],
		conveyerStartBeat: ev.beat + 3, conveyerLength: 1, conveyerRestartLength: 0.5,
		fuel: ev.fuel, lampOff: ev.lampOff, lampOn: ev.lampOn, beepNext: math.Inf(1),
	}
	b.lerpDistance = -b.startX
	b.lerpIdleDistance = b.lerpDistance
	b.initInstance(ev.beat)
	b.schedule(ev.beat)
	return b
}

func (b *bot) initInstance(beat float64) {
	in := b.inst
	in.PlayDefaultState("FullBody", beat, secondsScale(b.mod.ctx, beat))
	in.PlayDefaultState("FullBody/Fill", beat, secondsScale(b.mod.ctx, beat))
	in.PlayDefaultState("Legs", beat, secondsScale(b.mod.ctx, beat))
	in.PlayDefaultState("Body", beat, secondsScale(b.mod.ctx, beat))
	in.PlayDefaultState("Head", beat, secondsScale(b.mod.ctx, beat))
	in.SetActive("FullBody", false)
	in.SetActive("Legs", true)
	in.SetActive("Body", true)
	in.SetActive("Head", true)
	in.SetActive("FullBody/Fire", false)
	b.setLimbStartPositions()
	b.applyPalette()
}

func (b *bot) schedule(beat float64) {
	m := b.mod
	m.ctx.At(beat, func() {
		b.legsFallen = true
		b.inst.SetPos("Legs", b.spec.legsBase[0], b.spec.legsBase[1])
		b.inst.PlayState("Legs", "Impact", beat, secondsScale(m.ctx, beat))
	})
	m.ctx.At(beat+1, func() {
		b.bodyFallen = true
		b.inst.SetPos("Body", b.spec.bodyBase[0], b.spec.bodyBase[1])
		b.inst.PlayState("Body", "Impact", beat+1, secondsScale(m.ctx, beat+1))
	})
	m.ctx.At(beat+2, func() {
		b.headFallen = true
		b.inst.SetPos("Head", b.spec.headBase[0], b.spec.headBase[1])
		b.inst.PlayState("Head", "Impact", beat+2, secondsScale(m.ctx, beat+2))
	})
	m.ctx.At(beat+3, func() {
		b.inst.SetActive("FullBody", true)
		b.inst.SetActive("Legs", false)
		b.inst.SetActive("Body", false)
		b.inst.SetActive("Head", false)
	})
	for i := 0; i <= 2; i++ {
		m.ctx.SoundAt(beat+float64(i), botPrefix(b.size)+"Fall", 1)
	}
	m.ctx.ScheduleInputCond(beat+4, func() bool { return b.state == stateIdle && !b.dead },
		func(state float64, _ engine.Judgment) { b.justHold(state, m.ctx.Beat()) },
		func() {},
	)
}

func (b *bot) setLimbStartPositions() {
	h := b.spec.limbFallHeight
	b.inst.SetPos("Legs", b.spec.legsBase[0], b.spec.legsBase[1]+h)
	b.inst.SetPos("Body", b.spec.bodyBase[0], b.spec.bodyBase[1]+h)
	b.inst.SetPos("Head", b.spec.headBase[0], b.spec.headBase[1]+h)
}

func (b *bot) update(beat float64) {
	if b.dead {
		return
	}
	if !b.legsFallen {
		b.updateLimb("Legs", b.spec.legsBase, b.startBeat, beat)
	}
	if !b.bodyFallen {
		b.updateLimb("Body", b.spec.bodyBase, b.startBeat+1, beat)
	}
	if !b.headFallen {
		b.updateLimb("Head", b.spec.headBase, b.startBeat+2, beat)
	}
	if b.stack {
		b.handleStacking(beat)
	}
	if b.legsFallen && b.bodyFallen && b.headFallen {
		b.handleConveyer(beat)
	}
	if b.state == stateHolding {
		b.handleHolding(beat)
	} else if b.inst.CurrentState("FullBody") != "" {
		b.inst.PlayNormalized("FullBody/Fill", botFillClip(b.size), b.fill)
	}
	b.applyPalette()
}

func (b *bot) updateLimb(rel string, base [2]float64, targetBeat, beat float64) {
	u := clamp01(normalized(targetBeat-0.25, 0.25, beat))
	y := lerp(base[1]+b.spec.limbFallHeight, base[1], u)
	if y < base[1] {
		y = base[1]
	}
	if y > base[1]+b.spec.limbFallHeight {
		y = base[1] + b.spec.limbFallHeight
	}
	b.inst.SetPos(rel, base[0], y)
}

func (b *bot) stackToLeft(beat, length float64) {
	if b.conveyerLength <= b.spec.stackDistanceRate {
		return
	}
	b.stack = true
	b.stackBeat = beat - length
	b.stackLen = length
	b.conveyerStartBeat += b.spec.stackDistanceRate
	b.conveyerLength -= b.spec.stackDistanceRate
}

func (b *bot) handleStacking(beat float64) {
	u := normalized(b.stackBeat, b.stackLen, beat)
	if u >= 0 && u < 1 {
		b.moveConveyer(u, b.lerpDistance*b.spec.stackDistanceRate, 0)
		return
	}
	if u >= 1 {
		b.moveConveyer(1, b.lerpDistance*b.spec.stackDistanceRate, 0)
		b.stopConveyer()
		b.stack = false
	}
}

func (b *bot) handleConveyer(beat float64) {
	if b.conveyerStartBeat < 0 {
		b.stopConveyer()
		return
	}
	u := normalized(b.conveyerStartBeat, b.conveyerLength, beat)
	if u < 0 {
		return
	}
	switch b.state {
	case stateAce:
		b.moveConveyer(u, b.lerpDistance, b.spec.flyDistance)
	case stateIdle:
		b.moveConveyer(u, b.lerpIdleDistance, 0)
	default:
		b.moveConveyer(u, b.lerpDistance, 0)
	}
}

func (b *bot) moveConveyer(u, dx, dy float64) {
	if b.state == stateHolding {
		b.stopConveyer()
		return
	}
	b.inst.Offset[0] = b.startX + dx*u
	b.inst.Offset[1] = b.startY + dy*u
	if u >= 8 {
		b.dead = true
		b.deadAt = b.mod.ctx.Beat()
	}
}

func (b *bot) stopConveyer() {
	b.startX, b.startY = b.inst.Offset[0], b.inst.Offset[1]
	b.lerpIdleDistance = -b.startX
}

func (b *bot) justHold(state float64, beat float64) {
	m := b.mod
	m.playFiller("Hold"+botSuffix(m.fillerPosition), beat, 0.5)
	m.ctx.Sound("armExtension")
	if state >= 1 || state <= -1 {
		b.inst.PlayState("FullBody", "HoldBarely", beat, secondsScale(m.ctx, beat))
		return
	}
	m.renewConveyerNormalizedOffset(beat)
	m.conveyerStartBeat = -1
	b.conveyerLength = 1
	b.inst.Offset[0] = 0
	b.state = stateHolding
	m.fillerHolding = true
	b.inst.PlayState("FullBody", "Hold", beat, 1)
	m.ctx.Sound("beep")
	pitch := 3/(b.holdLength+3) + 0.5
	// Unity bends this looping SoundByte upward through the hold. The current
	// audio runtime only supports fixed-pitch loops, so the initial authored
	// pitch is preserved and the missing bend is tracked in README.
	b.stopWater = m.ctx.SoundLoopPitchVol("water", pitch, 1)
	b.beepNext = b.startBeat + 5
	releaseBeat := b.startBeat + 4 + b.holdLength
	m.ctx.ScheduleInputReleaseCond(releaseBeat, func() bool {
		return b.state == stateHolding && !b.releaseResolved && !b.dead
	}, func(state float64, _ engine.Judgment) {
		b.justRelease(state, m.ctx.Beat())
	}, func() {})
	m.ctx.At(releaseBeat+0.25, func() {
		if b.state == stateHolding && !b.releaseResolved && !b.exploded {
			b.handleExplosion(releaseBeat + 0.25)
		}
	})
}

func (b *bot) handleHolding(beat float64) {
	holdStart := b.startBeat + 4
	u := normalized(holdStart, b.holdLength, beat)
	for beat >= b.beepNext && b.beepNext < holdStart+b.holdLength {
		b.mod.ctx.Sound("beep")
		b.inst.PlayState("FullBody", "HoldBeat", b.beepNext, 1)
		b.mod.playFiller("HoldBeat"+botSuffix(b.mod.fillerPosition), b.beepNext, 1)
		b.beepNext++
	}
	b.fill = clamp01(u)
	b.inst.PlayNormalized("FullBody/Fill", botFillClip(b.size), b.fill)
}

func (b *bot) handleExplosion(beat float64) {
	b.exploded = true
	b.releaseResolved = true
	b.inst.PlayState("FullBody", "Beyond", beat, secondsScale(b.mod.ctx, beat))
	stopBeat := b.startBeat + b.holdLength + 5.5
	b.mod.ctx.At(stopBeat, func() {
		b.mod.fillerHolding = false
		b.mod.ctx.Sound("explosion")
		b.stopLoop()
		b.dead = true
		b.deadAt = stopBeat
	})
}

func (b *bot) handleReleaseWhiff(beat float64) {
	if b.state != stateHolding || b.releaseResolved {
		return
	}
	u := normalized(b.startBeat+4, b.holdLength, beat)
	if u < 1 {
		b.inst.PlayState("FullBody", "Dead", beat, secondsScale(b.mod.ctx, beat))
	} else if !b.exploded {
		b.inst.PlayState("FullBody", "ReleaseLate", beat, 0.5)
	}
	b.mod.ctx.Sound("miss")
	b.stopLoop()
	b.releaseResolved = true
	b.state = stateNG
	b.mod.fillerHolding = false
	b.fill = u
	if b.conveyerRestartLength >= 0 {
		b.conveyerStartBeat = beat + b.conveyerRestartLength
		if b.mod.conveyerStartBeat == -1 {
			b.mod.conveyerStartBeat = b.conveyerStartBeat
		}
	} else {
		b.conveyerStartBeat = -2
		b.mod.conveyerStartBeat = -1
	}
	b.mod.ctx.ScoreMiss()
}

func (b *bot) justRelease(state float64, beat float64) {
	m := b.mod
	b.stopLoop()
	b.releaseResolved = true
	if b.conveyerRestartLength >= 0 {
		b.conveyerStartBeat = b.startBeat + 4 + b.holdLength + b.conveyerRestartLength
		m.renewConveyerNormalizedOffset(beat)
		if m.conveyerStartBeat != -2 {
			m.conveyerStartBeat = b.conveyerStartBeat
		}
	} else {
		b.conveyerStartBeat = -2
		m.conveyerStartBeat = -1
	}

	suffix := botSuffix(m.fillerPosition)
	if state >= 1 {
		b.state = stateNG
		m.ctx.Sound("miss")
		m.playFiller("ReleaseWhiff"+suffix, beat, 0.5)
		m.ctx.Sound("armRetractionPop")
		b.inst.PlayState("FullBody", "ReleaseLate", beat, 0.5)
		return
	}
	if state <= -1 {
		b.state = stateNG
		m.ctx.Sound("miss")
		m.playFiller("ReleaseWhiff"+suffix, beat, 0.5)
		m.ctx.Sound("armRetractionPop")
		b.inst.PlayState("FullBody", "ReleaseEarly", beat, 0.5)
		return
	}

	if ((b.end == endBoth && math.Abs(state) <= 1) || b.end == endAce) && b.conveyerRestartLength >= 0 {
		b.state = stateAce
		m.ctx.At(beat+0.5, func() { b.inst.PlayState("FullBody", "Fly", beat+0.5, 0.5) })
	} else {
		b.state = stateJust
		if b.size == sizeSmall {
			m.ctx.At(beat+1, func() { b.inst.PlayState("FullBody", "Success", beat+1, 0.5) })
		} else {
			m.ctx.At(beat+0.9, func() { b.state = stateDance })
		}
	}
	m.fillerHolding = false
	m.playFiller("Release"+suffix, beat, 0.5)
	m.ctx.Sound("armRetraction")
	b.inst.PlayState("FullBody", "Release", beat, 1)
	prefix := botPrefix(b.size)
	ok0 := 0.5
	ok1 := 1.0
	if b.altOK {
		ok0, ok1 = 0, 0.5
	}
	m.ctx.SoundAt(beat+ok0, prefix+"Move", 1)
	m.ctx.SoundAt(beat+ok0, prefix+"OK1", 1)
	m.ctx.SoundAt(beat+ok1, prefix+"OK2", 1)
}

func (b *bot) successDance(beat float64) {
	b.inst.PlayState("FullBody", "Success", beat, 0.5)
}

func (b *bot) stopLoop() {
	if b.stopWater != nil {
		b.stopWater()
		b.stopWater = nil
	}
}

func (b *bot) applyPalette() {
	pal := botPalette(b.fuel, b.lampOff)
	if b.state == stateHolding {
		pal.Alpha = b.lampOn
	}
	for _, path := range []string{"FullBody", "Legs", "Body", "Head"} {
		b.inst.SetPalette(path, pal)
	}
}

func (b *bot) queue(beat float64) {
	b.inst.Queue(b.mod.ctx.Scene, beat, kart.Identity(), 0)
}

func (b *bot) drawFuel(screen *ebiten.Image, proj kart.Aff) {
	if b.fill <= 0 || b.state < stateHolding {
		return
	}
	h := b.spec.fillScaleY * clamp01(b.fill)
	if h <= 0 {
		return
	}
	// Template instances currently do not participate in SceneInst's SpriteMask
	// pass, so the Fill SpriteRenderer's mask is approximated with the same
	// animated local position/scale curve and drawn underneath the mapped body.
	fillY := b.spec.fillPosY * clamp01(b.fill)
	world := kart.Translate(b.inst.Offset[0], b.inst.Offset[1]).
		Mul(kart.Scale(b.spec.rootScale[0], b.spec.rootScale[1])).
		Mul(kart.Translate(0, fillY)).
		Mul(kart.Scale(5, h))
	drawWorldRect(screen, proj, world, 1, 1, b.fuel)
}
