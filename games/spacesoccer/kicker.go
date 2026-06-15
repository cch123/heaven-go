package spacesoccer

import (
	"math"

	"hsdemo/engine"
	"hsdemo/kart"
)

type kicker struct {
	game *Module
	inst *kart.Instance

	player        bool
	index         int
	kickTimes     int
	kickLeft      bool
	kickLeftWhiff bool
	canKick       bool
	canHighKick   bool
	canToe        bool
	stopBall      bool
	ball          *ball

	enterAnim  string
	enterBeat  float64
	enterLen   float64
	enterEase  int
	groupOrder int
	z          float64
	base       [2]float64
	floatW     float64
	floatH     float64
	floatPhase float64
}

func newKicker(m *Module, idx int, player bool) *kicker {
	k := &kicker{
		game: m, inst: m.kickerT.NewInstance(),
		index: idx, player: player, canKick: true,
		enterAnim: "Present", enterLen: 1,
		floatW: 0.7, floatH: 0.35,
	}
	k.inst.PlayDefaultState(bodyRel, 0, m.ctx.SecPerBeat(0))
	k.inst.PlayDefaultState(holderRel, 0, m.ctx.SecPerBeat(0))
	k.inst.PlayDefaultState(flamesRel, 0, m.ctx.SecPerBeat(0))
	k.applyPalette(m.kickerPalette, m.platformPalette, m.firePalette)
	return k
}

func (k *kicker) setEnterAnim(beat, length float64, anim string, ease int) {
	k.enterBeat = beat
	k.enterLen = length
	k.enterAnim = anim
	k.enterEase = ease
	k.applyEnterAnim(beat)
}

func (k *kicker) applyEnterAnim(beat float64) {
	if k.enterAnim == "" {
		return
	}
	u := 1.0
	if k.enterLen > 0 {
		u = (beat - k.enterBeat) / k.enterLen
	}
	u = math.Max(0, math.Min(1, u))
	norm := engine.Ease(k.enterEase, 0, 1, u)
	k.inst.PlayFrozen(holderRel, k.enterAnim, norm)
}

func (k *kicker) update(beat float64) {
	k.kickLeft = k.kickTimes%2 != 0
	k.applyEnterAnim(beat)
	if !k.player || k.stopBall {
		return
	}
	if k.game.ctx.ReleasedNow() {
		k.handleUnscoredRelease(beat)
	}
}

func (k *kicker) handleUnscoredRelease(beat float64) {
	if k.ball == nil || !(k.game.ctx.ExpectingReleaseNow() || k.ball.canKick) {
		if k.canToe {
			k.toe(false, false)
		} else {
			k.missAnim(beat)
		}
	}
	if k.ball == nil {
		return
	}
	if k.ball.waitKickRelease {
		k.ball.waitKickRelease = false
		return
	}
	if k.ball.canKick && !k.game.ctx.ExpectingReleaseNow() {
		k.ball.canKick = false
		k.kick(false, false, false)
	}
}

func (k *kicker) dispenseBall(beat float64) {
	if k.player {
		target := beat + k.ball.animLength(ballDispensing)
		k.game.ctx.ScheduleInput(target, func(state float64, j engine.Judgment) {
			k.kickJust(target, state)
		}, func() {
			k.miss(target)
		})
		return
	}

	beatToKick := beat + k.ball.animLength(ballDispensing)
	if beatToKick < k.game.nowBeat {
		beatToKick = k.ball.nextAnimBeat
	}
	if k.ball.state == ballHighKicked {
		k.game.at(beatToKick-0.5, func() { k.kick(true, true, false) })
		k.game.at(beatToKick, func() { k.toe(true, false) })
		k.game.at(beatToKick+k.ball.animLength(ballToe), func() {
			k.kickCheck(true, false, beatToKick+k.ball.animLength(ballToe), false)
		})
		return
	}
	k.game.at(beatToKick, func() {
		k.kickCheck(true, false, beatToKick, false)
	})
}

func (k *kicker) kick(hit, highKick, barely bool) {
	if k.stopBall {
		return
	}
	if k.player {
		pitch := 1.0
		if barely {
			pitch = 0.95
		}
		k.game.ctx.SoundPitch("kick", 1, pitch)
	}

	switch {
	case hit && highKick:
		if k.kickLeft {
			k.play("HighKickLeft_0", k.game.nowBeat)
		} else {
			k.play("HighKickRight_0", k.game.nowBeat)
		}
	case hit && barely:
		if k.kickLeft {
			k.play("BarelyLeft", k.game.nowBeat)
		} else {
			k.play("BarelyRight", k.game.nowBeat)
		}
	case hit:
		if k.kickLeft {
			k.play("KickLeft", k.game.nowBeat)
		} else {
			k.play("KickRight", k.game.nowBeat)
		}
	default:
		if highKick {
			if k.kickLeftWhiff {
				k.play("HighKickLeft_0", k.game.nowBeat)
			} else {
				k.play("HighKickRight_0", k.game.nowBeat)
			}
		} else if k.kickLeftWhiff {
			k.play("KickLeft", k.game.nowBeat)
		} else {
			k.play("KickRight", k.game.nowBeat)
		}
		k.kickLeftWhiff = !k.kickLeftWhiff
	}

	if k.ball == nil {
		return
	}
	if !highKick {
		k.kickTimes++
		if hit {
			k.ball.kick(k.player)
		}
	}
}

func (k *kicker) highKick(hit bool) {
	if k.stopBall {
		return
	}
	k.kickTimes++
	if hit {
		if k.kickLeft {
			k.play("HighKickLeft_0", k.game.nowBeat)
		} else {
			k.play("HighKickRight_0", k.game.nowBeat)
		}
	} else {
		if k.kickLeftWhiff {
			k.play("HighKickLeft_0", k.game.nowBeat)
		} else {
			k.play("HighKickRight_0", k.game.nowBeat)
		}
		k.kickLeftWhiff = !k.kickLeftWhiff
	}
	if k.player {
		k.game.ctx.Sound("highkicktoe1")
	}
	if hit && k.ball != nil {
		k.ball.highKick()
		if k.player {
			k.game.ctx.Sound("highkicktoe1_hit")
		}
	}
}

func (k *kicker) toe(hit, flick bool) {
	if k.stopBall {
		return
	}
	if k.kickLeft {
		k.play("ToeLeft", k.game.nowBeat)
	} else {
		k.play("ToeRight", k.game.nowBeat)
	}
	if k.player {
		k.game.ctx.Sound("highkicktoe3")
		if hit && k.ball != nil {
			k.game.ctx.Sound("highkicktoe3_hit")
		}
	}
	if hit && k.ball != nil {
		k.ball.toe()
	}
	if !flick {
		k.kickTimes++
	}
}

func (k *kicker) kickCheck(hit, overrideState bool, beat float64, barely bool) {
	if k.stopBall {
		return
	}
	k.canKick = true
	k.canHighKick = false
	for _, hk := range k.game.highKicks {
		if hk.beat-0.5 <= k.game.nowBeat && hk.beat+1 > k.game.nowBeat {
			k.canHighKick = true
			k.canKick = false
			if k.ball != nil {
				k.ball.highKickSwing = 0.5
			}
			break
		}
	}
	if k.canHighKick {
		k.highKick(hit)
		if !k.player && k.ball != nil {
			next := beat + k.ball.animLength(ballKicked)
			toeBeat := beat + k.ball.animLength(ballToe)
			k.game.at(next, func() { k.kick(true, true, false) })
			k.game.at(toeBeat, func() { k.toe(true, false) })
			k.game.at(toeBeat+1.5, func() { k.kickCheck(true, false, toeBeat+1.5, false) })
		}
		return
	}
	if k.canKick || overrideState {
		k.kick(hit, false, barely)
		if !k.player && k.ball != nil {
			next := beat + k.ball.animLength(ballKicked)
			k.game.at(next, func() { k.kickCheck(true, false, next, false) })
		}
	}
}

func (k *kicker) kickJust(target, state float64) {
	if k.stopBall {
		return
	}
	if k.ball == nil {
		k.kickCheck(false, true, 0, false)
		k.missBall(target)
		return
	}
	barely := math.Abs(state) >= 1
	k.kickCheck(true, false, 0, barely)
	if k.canHighKick && k.ball != nil {
		releaseTarget := target + k.ball.animLength(ballToe)
		k.game.ctx.ScheduleInputRelease(releaseTarget, func(state float64, j engine.Judgment) {
			k.toeJust(target, releaseTarget, state)
		}, func() {
			k.miss(releaseTarget)
		})
		autoKickBeat := target + k.ball.animLength(ballKicked)
		k.game.at(autoKickBeat, func() { k.kick(true, true, false) })
		k.ball.canKick = true
		k.ball.waitKickRelease = true
		k.game.at(target+0.75, func() {
			k.canToe = !k.game.ctx.PressingNow()
		})
	} else if k.ball != nil {
		next := target + k.ball.animLength(ballKicked)
		k.game.ctx.ScheduleInput(next, func(state float64, j engine.Judgment) {
			k.kickJust(next, state)
		}, func() {
			k.miss(next)
		})
	}
	k.game.hitBeats = append(k.game.hitBeats, target)
}

func (k *kicker) toeJust(highKickTarget, releaseTarget, state float64) {
	if k.stopBall {
		return
	}
	if k.ball == nil || !k.ball.canKick || math.Abs(state) >= 1 {
		k.toe(false, false)
		k.missBall(releaseTarget)
		return
	}
	if !k.canToe {
		k.missAnim(releaseTarget)
		k.missBall(releaseTarget)
		k.game.ctx.ScoreMiss()
		return
	}
	k.toe(true, false)
	next := highKickTarget + 3
	k.game.ctx.ScheduleInput(next, func(state float64, j engine.Judgment) {
		k.kickJust(next, state)
	}, func() {
		k.miss(next)
	})
	k.ball.canKick = false
	k.game.hitBeats = append(k.game.hitBeats, releaseTarget)
}

func (k *kicker) miss(target float64) {
	if k.stopBall {
		return
	}
	if k.ball != nil {
		k.missBall(target)
	}
}

func (k *kicker) missBall(target float64) {
	if k.stopBall {
		return
	}
	if k.ball != nil {
		k.ball.dead = true
	}
	k.ball = nil
	pitch := randomPitchCents(-75, 75)
	if target <= k.game.ctx.Beat()+1e-6 {
		k.game.ctx.SoundPitchPan("missNeutral", 1, pitch, 0)
	} else {
		k.game.ctx.SoundAtPitchPan(target, "missNeutral", 1, pitch, 0)
	}
}

func (k *kicker) missAnim(beat float64) {
	if k.kickLeft {
		k.play("MissLeft", beat)
	} else {
		k.play("MissRight", beat)
	}
}

func (k *kicker) stop(stop bool) {
	k.stopBall = stop
	if stop && k.ball != nil {
		k.ball.dead = true
		k.ball = nil
	}
}

func (k *kicker) play(state string, beat float64) {
	k.inst.PlayState(bodyRel, state, beat, animScale)
}

func (k *kicker) applyPalette(kickerPal, platformPal, firePal kart.Palette) {
	for _, rel := range []string{
		"Space Kicker/Holder/Head",
		"Space Kicker/Holder/Torso",
		"Space Kicker/Holder/LowerTorso",
		"Space Kicker/Holder/LeftArm",
		"Space Kicker/Holder/LeftArm/Hand",
		"Space Kicker/Holder/RightArm",
		"Space Kicker/Holder/RightArm/Hand",
		"Space Kicker/Holder/RightLeg",
		"Space Kicker/Holder/LeftLeg",
		"Space Kicker/Holder/toeFX",
	} {
		k.inst.SetPalette(rel, kickerPal)
	}
	k.inst.SetPalette("Space Kicker/Holder/Head/Mouth", palette(kickerPal.Alpha, kickerPal.Fill, kickerPal.Outline))
	k.inst.SetPalette("Space Kicker/Holder/Platform", platformPal)
	k.inst.SetPalette("Space Kicker/Holder/Platform/Flames", firePal)
}
