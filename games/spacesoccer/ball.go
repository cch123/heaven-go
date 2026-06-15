package spacesoccer

import (
	"math"

	"hsdemo/kart"
)

type ballState int

const (
	ballNone ballState = iota
	ballDispensing
	ballKicked
	ballHighKicked
	ballToe
)

type ball struct {
	game   *Module
	kicker *kicker
	inst   *kart.Instance

	state           ballState
	startBeat       float64
	nextAnimBeat    float64
	highKickSwing   float64
	lastSpriteRot   float64
	spriteRot       float64
	canKick         bool
	waitKickRelease bool
	lastKickLeft    bool
	lastReal        [2]float64
	dead            bool
	dispensedAtBeat float64
	initializedMid  bool
}

func newBall(m *Module, k *kicker, beat float64) *ball {
	b := &ball{game: m, kicker: k, inst: m.ballT.NewInstance(), dispensedAtBeat: beat}
	k.ball = b
	b.init(beat)
	return b
}

func (b *ball) init(dispensedBeat float64) {
	now := b.game.nowBeat
	if now == 0 {
		now = dispensedBeat
	}
	if now-dispensedBeat < 2 {
		b.state = ballDispensing
		b.startBeat = dispensedBeat
		b.nextAnimBeat = b.startBeat + b.animLength(ballDispensing)
		b.kicker.kickTimes = 0
		return
	}

	numHighKicks := 0
	for _, hk := range b.game.highKicks {
		switch {
		case hk.beat+hk.length <= now:
			numHighKicks++
			continue
		case hk.beat > now:
			rel := now - dispensedBeat
			b.state = ballKicked
			b.startBeat = dispensedBeat + math.Floor(rel-0.1)
			b.nextAnimBeat = b.startBeat + b.animLength(ballKicked)
			b.kicker.kickTimes = int(math.Floor(rel-0.1)) - numHighKicks - 1
			b.initializedMid = true
			b.update(now)
			return
		case hk.beat+b.animLength(ballHighKicked) > now:
			b.highKickSwing = 0.5
			rel := hk.beat - dispensedBeat
			b.state = ballHighKicked
			b.startBeat = dispensedBeat + math.Ceil(rel)
			b.nextAnimBeat = b.startBeat + b.animLength(ballHighKicked)
			b.kicker.kickTimes = int(math.Ceil(rel)) - numHighKicks - 1
			b.initializedMid = true
			b.update(now)
			return
		default:
			b.highKickSwing = 0.5
			rel := math.Ceil(hk.beat-dispensedBeat) + b.animLength(ballHighKicked)
			b.state = ballToe
			b.startBeat = dispensedBeat + rel
			b.nextAnimBeat = b.startBeat + b.animLength(ballToe)
			b.kicker.kickTimes = int(rel-b.animLength(ballHighKicked)) - numHighKicks
			b.initializedMid = true
			b.update(now)
			return
		}
	}

	rel := now - dispensedBeat
	b.state = ballKicked
	b.startBeat = dispensedBeat + math.Floor(rel-0.1)
	b.nextAnimBeat = b.startBeat + b.animLength(ballKicked)
	b.kicker.kickTimes = int(math.Floor(rel-0.1)) - numHighKicks - 1
	b.initializedMid = true
	b.update(now)
}

func (b *ball) animLength(st ballState) float64 {
	switch st {
	case ballDispensing:
		return 2
	case ballKicked:
		return 1
	case ballHighKicked:
		return 1 + b.highKickSwing
	case ballToe:
		return 1 + b.highKickSwing
	default:
		return 0
	}
}

func (b *ball) kick(player bool) {
	if player {
		b.game.ctx.SoundPitch("ballHit", 1, randomPitchCents(-38, 39))
	}
	b.lastSpriteRot = b.spriteRot
	b.setState(ballKicked)
	b.lastKickLeft = b.kicker.kickLeft
	b.updateLastReal()
}

func (b *ball) highKick() {
	b.lastSpriteRot = b.spriteRot
	b.setState(ballHighKicked)
	b.updateLastReal()
}

func (b *ball) toe() {
	b.lastSpriteRot = b.spriteRot
	b.setState(ballToe)
	b.updateLastReal()
}

func (b *ball) setState(st ballState) {
	b.state = st
	b.startBeat = b.nextAnimBeat
	b.nextAnimBeat += b.animLength(st)
}

func (b *ball) updateLastReal() {
	p := b.posAt(b.game.nowBeat)
	b.lastReal = [2]float64{p[0], p[1]}
}

func (b *ball) posAt(beat float64) [3]float64 {
	switch b.state {
	case ballDispensing:
		return b.game.paths["Dispense"].posAt(math.Max(beat, b.startBeat), b.startBeat, b.lastReal)
	case ballKicked:
		p := b.game.paths["Kick"]
		if b.lastKickLeft {
			p = p.endOverride(-2.5, -6, 0)
		} else {
			p = p.endOverride(0, -6, 0)
		}
		return p.posAt(math.Max(beat, b.startBeat), b.startBeat, b.lastReal)
	case ballHighKicked:
		p := b.game.paths["HighKick"].durationOverride(b.animLength(ballHighKicked) + 0.3)
		return p.posAt(math.Max(beat, b.startBeat), b.startBeat, b.lastReal)
	case ballToe:
		p := b.game.paths["Toe"].durationOverride(b.animLength(ballToe) + 0.35)
		if b.lastKickLeft {
			p = p.endOverride(-0.5, -6, 0)
		} else {
			p = p.endOverride(-1.5, -6, 0)
		}
		return p.posAt(math.Max(beat, b.startBeat), b.startBeat, b.lastReal)
	default:
		return [3]float64{}
	}
}

func (b *ball) update(beat float64) {
	if b.dead {
		return
	}
	pos := b.posAt(beat)
	b.inst.Offset = [2]float64{pos[0], pos[1]}
	switch b.state {
	case ballDispensing:
		u := math.Max(0, math.Min(1, (beat-b.startBeat)/2.35))
		b.spriteRot = -1440 * math.Pi / 180 * u
	case ballKicked:
		u := math.Max(0, math.Min(1, (beat-b.startBeat)/1.5))
		if b.lastKickLeft {
			b.spriteRot = b.lastSpriteRot + 2*math.Pi*u
		} else {
			b.spriteRot = b.lastSpriteRot - 2*math.Pi*u
		}
	case ballHighKicked:
		u := math.Max(0, math.Min(1, (beat-b.startBeat)/(b.animLength(ballHighKicked)+0.3)))
		b.spriteRot = b.lastSpriteRot + 2*math.Pi*u
	}
	b.inst.SetRot("Sprite", b.spriteRot)
}

func randomPitchCents(lo, hi int) float64 {
	if hi <= lo {
		return 1
	}
	cents := lo + int(math.Floor(float64(hi-lo+1)*randFloat()))
	return math.Pow(2, float64(cents)/1200)
}
