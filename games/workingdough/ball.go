package workingdough

import (
	"math"

	"hsdemo/engine"
	"hsdemo/kart"
)

type ballKind int

const (
	ballNPC ballKind = iota
	ballPlayer
	ballBG
)

type playerBallState int

const (
	playerEntering playerBallState = iota
	playerHit
	playerBarely
	playerMiss
	playerWeak
)

type doughBall struct {
	mod      *Module
	inst     *kart.Instance
	kind     ballKind
	big      bool
	hasGandw bool

	startBeat float64
	hitBeat   float64
	stateBeat float64
	state     playerBallState
	dead      bool

	canJust  bool
	canWrong bool
}

func newNPCBall(m *Module, beat float64, big, hasGandw bool) *doughBall {
	t := m.smallBallT
	if big {
		t = m.bigBallT
	}
	if t == nil {
		return nil
	}
	b := &doughBall{
		mod: m, inst: t.NewInstance(), kind: ballNPC,
		big: big, hasGandw: hasGandw, startBeat: beat,
	}
	b.setGandw(hasGandw)
	return b
}

func newPlayerBall(m *Module, beat float64, big bool, flash [4]float64, hasGandw bool) *doughBall {
	t := m.playerSmallT
	if big {
		t = m.playerBigT
	}
	if t == nil {
		return nil
	}
	b := &doughBall{
		mod: m, inst: t.NewInstance(), kind: ballPlayer,
		big: big, hasGandw: hasGandw, startBeat: beat, hitBeat: beat + 1,
		stateBeat: beat, state: playerEntering, canJust: true, canWrong: true,
	}
	b.setGandw(hasGandw)
	b.scheduleInputs(flash)
	return b
}

func newBGBall(m *Module, beat float64, big, hasGandw bool) *doughBall {
	t := m.bgSmallT
	if big {
		t = m.bgBigT
	}
	if t == nil {
		return nil
	}
	b := &doughBall{
		mod: m, inst: t.NewInstance(), kind: ballBG,
		big: big, hasGandw: hasGandw, startBeat: beat,
	}
	b.setGandw(hasGandw)
	return b
}

func (b *doughBall) setGandw(active bool) {
	if b.inst != nil {
		b.inst.SetActive("GANDWPanic", active)
	}
}

func (b *doughBall) scheduleInputs(flash [4]float64) {
	action := 0
	wrongAction := actionBig
	if b.big {
		action, wrongAction = actionBig, 0
	}
	right := b.mod.ctx.ScheduleInputActionCond(b.hitBeat, action,
		func() bool { return b.canJust && !b.dead && b.state == playerEntering },
		func(state float64, _ engine.Judgment) { b.just(state, flash) },
		func() { b.miss() },
	)
	if right != nil {
		right.CanHit = func() bool { return b.canJust && !b.dead && b.state == playerEntering }
	}
	wrong := b.mod.ctx.ScheduleInputActionNoScore(b.hitBeat, wrongAction,
		func(state float64, _ engine.Judgment) { b.wrong(state) },
		nil,
	)
	if wrong != nil {
		wrong.NoAutoplay = true
		wrong.CanHit = func() bool { return b.canWrong && !b.dead && b.state == playerEntering }
	}
}

func (b *doughBall) update(beat float64) bool {
	if b == nil || b.dead || b.inst == nil {
		return false
	}
	switch b.kind {
	case ballNPC:
		if beat >= b.startBeat+2 {
			return false
		}
		b.inst.Offset = b.mod.pathPos("NPCBall", beat-b.startBeat)
	case ballBG:
		if beat >= b.startBeat+9 {
			return false
		}
		b.inst.Offset = b.mod.pathPos("BGBall", beat-b.startBeat)
		b.inst.Rot = (beat - b.startBeat) * math.Pi / 2
	case ballPlayer:
		if !b.updatePlayer(beat) {
			return false
		}
	}
	return true
}

func (b *doughBall) updatePlayer(beat float64) bool {
	switch b.state {
	case playerEntering:
		if beat < b.startBeat {
			b.inst.Offset = [2]float64{-80, -80}
		} else {
			b.inst.Offset = b.mod.pathPos("PlayerEnter", beat-b.startBeat)
		}
	case playerHit:
		if beat >= b.stateBeat+1 {
			return false
		}
		b.inst.Offset = b.mod.pathPos("PlayerHit", beat-b.stateBeat)
	case playerBarely:
		if beat >= b.stateBeat+2 {
			return false
		}
		b.inst.Offset = b.mod.pathPos("PlayerBarely", beat-b.stateBeat)
	case playerMiss:
		if beat >= b.stateBeat+1 {
			return false
		}
		b.inst.Offset = b.mod.pathPos("PlayerMiss", beat-b.stateBeat)
	case playerWeak:
		if beat >= b.stateBeat+1 {
			return false
		}
		b.inst.Offset = b.mod.pathPos("PlayerWeak", beat-b.stateBeat)
	}
	return true
}

func (b *doughBall) just(state float64, flash [4]float64) {
	if !b.canJust || b.dead {
		return
	}
	b.canWrong = false
	beat := b.mod.ctx.Beat()
	b.stateBeat = beat
	b.mod.hitImpact(beat)
	if state >= 1 || state <= -1 {
		b.state = playerBarely
		b.mod.ctx.Sound("common_miss")
		b.mod.playPlayerJump(beat, b.big)
		return
	}
	b.state = playerHit
	b.mod.playPlayerHit(beat, b.big, flash)
	hasGandw := b.hasGandw
	b.mod.ctx.At(beat+0.9, func() { b.mod.setArrow(b.mod.arrowRightPlayer, true) })
	b.mod.ctx.At(beat+1, func() { b.mod.setArrow(b.mod.arrowRightPlayer, false) })
	b.mod.ctx.At(beat+2, func() { b.mod.spawnBGBall(beat+2, b.big, hasGandw) })
}

func (b *doughBall) wrong(_ float64) {
	if !b.canWrong || b.dead {
		return
	}
	b.canJust = false
	beat := b.mod.ctx.Beat()
	b.mod.hitImpact(beat)
	if b.big {
		b.state = playerWeak
		b.stateBeat = beat
		b.mod.playPlayerJump(beat, false)
		b.mod.ctx.Sound("tooBig")
	} else {
		b.mod.spawnBreakBurst(beat, b.inst.Offset)
		b.mod.playPlayerJump(beat, true)
		b.mod.ctx.Sound("tooSmall")
		b.dead = true
	}
	b.mod.ctx.ScoreMiss()
}

func (b *doughBall) miss() {
	if !b.canJust || b.dead {
		return
	}
	b.canWrong = false
	beat := b.hitBeat
	now := b.mod.ctx.Beat()
	if beat < now-0.05 {
		beat = now
	}
	b.state = playerMiss
	b.stateBeat = beat
	b.mod.ctx.At(beat+0.25, func() {
		b.mod.ctx.Scene.SetActive(b.mod.missImpact, true)
		b.mod.ctx.Sound("BallMiss")
	})
	b.mod.ctx.At(beat+0.35, func() { b.mod.ctx.Scene.SetActive(b.mod.missImpact, false) })
}
