package nightwalkagb

import (
	"math"

	"hsdemo/engine"
	"hsdemo/kart"
)

type platformInst struct {
	mod  *Module
	inst *kart.Instance

	startBeat float64
	endBeat   float64
	typ       int

	lastUnits int
	nextUnits int
	addHeight float64

	nextSame   bool
	isFinal    bool
	isFish     bool
	isEnd      bool
	isRoll     bool
	isHidden   bool
	isFalling  bool
	fallRoll   bool
	fallBeat   float64
	stopped    bool
	canKick    bool
	canRelease bool
}

func (m *Module) spawnPlatforms(beat float64) {
	m.platforms = nil
	if m.platformT == nil {
		return
	}
	count := m.cfg.platformCount
	var platformStart float64
	if !math.IsInf(m.countInBeat, -1) {
		platformStart = m.countInBeat + m.countInLength
		if !math.IsInf(m.endBeat, 1) {
			total := int(math.Floor((math.Ceil(m.endBeat) + 2) - platformStart))
			if total > -1 {
				count = total
			} else {
				count = 0
			}
		}
		for i := 0; i < count; i++ {
			start := platformStart + float64(i) - float64(count)*0.5
			hit := platformStart + float64(i)
			m.platforms = append(m.platforms, newPlatform(m, start, hit))
		}
		return
	}
	first := math.Ceil(beat)
	for i := 0; i < count; i++ {
		hit := first + float64(i-count)
		m.platforms = append(m.platforms, newPlatform(m, beat, hit))
	}
}

func newPlatform(m *Module, startBeat, hitBeat float64) *platformInst {
	p := &platformInst{
		mod: m, inst: m.platformT.NewInstance(), startBeat: startBeat, endBeat: hitBeat,
		typ: platformFlower, canKick: true, canRelease: true,
	}
	p.inst.PlayDefaultState("", 0, m.ctx.SecPerBeat(0))
	p.inst.PlayDefaultState(m.cfg.rollPlatform, 0, m.ctx.SecPerBeat(0))
	p.inst.PlayDefaultState(m.cfg.fish, 0, m.ctx.SecPerBeat(0))
	p.inst.PlayDefaultState(m.cfg.fallYan, 0, m.ctx.SecPerBeat(0))
	p.inst.PlayDefaultState(m.cfg.fallYanRoll, 0, m.ctx.SecPerBeat(0))
	p.inst.SetActive(m.cfg.fallYan, false)
	p.inst.SetActive(m.cfg.fallYanRoll, false)

	if hitBeat > m.endBeat+1 {
		p.isHidden = true
		p.hideChildren()
		return p
	}
	if m.rollAt(hitBeat - 1) {
		// The platform immediately after a roll hold is a recycled placeholder
		// in the Unity script; keeping it hidden avoids a duplicate hit window.
		p.isHidden = true
		p.hideChildren()
		return p
	}
	p.isRoll = m.rollAt(hitBeat)
	p.lastUnits = m.heightUnitsAt(hitBeat)
	lookahead := 1.0
	if p.isRoll {
		lookahead = 2
	}
	p.nextUnits = m.heightUnitsAt(hitBeat + lookahead)
	p.addHeight = float64(p.lastUnits) * m.cfg.heightAmount
	p.nextSame = p.lastUnits == p.nextUnits
	p.isFinal = nearBeat(hitBeat, m.endBeat+1)
	p.isFish = m.fishAt(hitBeat)
	p.isEnd = nearBeat(hitBeat, m.endBeat)

	p.configureVisuals()
	if startBeat < hitBeat {
		p.scheduleInput()
	}
	return p
}

func (p *platformInst) hideChildren() {
	for _, rel := range []string{p.mod.cfg.platform, p.mod.cfg.fish, "rollPlatform", p.mod.cfg.fallYan, p.mod.cfg.fallYanRoll} {
		p.inst.SetActive(rel, false)
	}
}

func (p *platformInst) configureVisuals() {
	m := p.mod
	if typ, _, ok := m.typeAt(p.endBeat); ok && !p.isRoll {
		p.typ = typ
	}
	p.inst.SetActive(m.cfg.fish, p.isFish)
	p.inst.SetActive("rollPlatform", p.isRoll)
	p.inst.SetActive(m.cfg.platform, p.nextSame && !p.isFinal && !p.isRoll)
	p.inst.SetActive(m.cfg.rollLong, p.isRoll && p.nextSame && !p.isFinal && !p.isEnd)
	p.inst.SetActive(m.cfg.rollLong2, p.isRoll && p.nextSame && !p.isFinal && !p.isEnd)
	if p.isEnd {
		if p.isRoll {
			p.playRollState("EndIdle", 0, 0.5)
		} else {
			p.inst.PlayState("", "EndIdle", 0, 0.5)
		}
	}
}

func (p *platformInst) scheduleInput() {
	m := p.mod
	if p.isRoll {
		in := m.ctx.ScheduleInputAction(p.endBeat, actionAlt, func(state float64, _ engine.Judgment) {
			p.justRollHold(state)
		}, p.rollMissHold)
		if m.noJumpAt(p.endBeat) || p.isFish {
			in.NoAutoplay = true
		}
		m.ctx.At(p.endBeat, func() {
			if p.stopped {
				return
			}
			m.ctx.Sound("boxKick")
			if p.canKick {
				p.inst.PlayState("", "Kick", p.endBeat, 0.5)
			}
		})
		if p.nextSame && !p.isEnd {
			m.ctx.At(p.endBeat+0.5, func() {
				if p.stopped {
					return
				}
				m.ctx.Sound("boxKick")
				if p.canRelease {
					p.playRollState("Kick", p.endBeat+0.5, 0.5)
				}
			})
		}
		return
	}
	in := m.ctx.ScheduleInput(p.endBeat, func(state float64, _ engine.Judgment) {
		if p.isEnd {
			p.justEnd(state)
		} else {
			p.just(state)
		}
	}, p.miss)
	if m.noJumpAt(p.endBeat) || p.isFish {
		in.NoAutoplay = true
	}
	if p.nextSame && !p.isEnd {
		m.ctx.At(p.endBeat, func() {
			if p.stopped {
				return
			}
			m.ctx.Sound("boxKick")
			if p.canKick {
				p.inst.PlayState("", "Kick", p.endBeat, 0.5)
			}
		})
	}
}

func (p *platformInst) update(beat float64) {
	if p.isHidden {
		return
	}
	if !p.stopped {
		u := (beat - p.startBeat) / math.Max(p.endBeat-p.startBeat, 1e-6)
		x0 := p.mod.cfg.playerXPos + (p.endBeat-p.startBeat)*p.mod.cfg.platformDistance
		x := x0 + (p.mod.cfg.playerXPos-x0)*u
		p.inst.Offset = [2]float64{x, p.mod.cfg.defaultYPos + p.addHeight}
	}
	if p.isFalling {
		u := norm(beat, p.fallBeat, 2)
		y := engine.Ease(2, 0, -12, u)
		if p.fallRoll {
			p.inst.SetPos(p.mod.cfg.fallYanRoll, 0, y)
		} else {
			p.inst.SetPos(p.mod.cfg.fallYan, 0, y)
		}
	}
	if !p.isEnd || p.stopped {
		return
	}
	if p.mod.hitJumps >= p.mod.requiredJumps && hitJumpsPersist >= p.mod.requiredJumpsP {
		if p.isRoll {
			if p.inst.CurrentState(p.mod.cfg.rollPlatform) != "EndGlow" {
				p.playRollState("EndGlow", beat, 0.5)
			}
		} else if p.inst.CurrentState("") != "EndGlow" {
			p.inst.PlayState("", "EndGlow", beat, 0.5)
		}
	}
}

func (p *platformInst) queue(sc *kart.SceneInst, beat float64) {
	if p.isHidden {
		return
	}
	p.inst.Queue(sc, beat, kart.Translate(0, -p.mod.holderY), 0)
}

func (p *platformInst) just(state float64) {
	m := p.mod
	beat := m.ctx.Beat()
	p.canKick = false
	m.raiseHeight(beat, p.lastUnits, p.nextUnits)
	m.player.jump(beat, p.isFinal)
	if p.isFish {
		m.ctx.At(beat+0.5, func() {
			m.player.shock(false)
			p.inst.PlayState(m.cfg.fish, "Shock", beat+0.5, 0.5)
			m.stopAll()
			m.destroyPlatforms(p.endBeat+2, p.endBeat-2, p.endBeat+6)
		})
		m.ctx.At(p.endBeat+4, func() {
			m.player.fall(p.endBeat + 4)
			p.inst.PlayState(m.cfg.fish, "FishIdle", p.endBeat+4, 0.5)
		})
	}
	barely := math.Abs(state) >= 1
	if barely {
		m.ctx.Sound("ng")
		p.inst.PlayState("", platformState(p.typ, true), beat, 0.5)
		return
	}
	if _, fill, ok := m.typeAt(p.endBeat + 1); ok && fill != fillNone {
		m.ctx.Sound("fillStart")
	} else {
		m.ctx.Sound("jump" + string(rune('0'+p.typ)))
	}
	p.inst.PlayState("", platformState(p.typ, false), beat, 0.5)
	m.stars.evolve(m.evolveAmount)
	m.hitJumps++
	hitJumpsPersist++
}

func (p *platformInst) justEnd(state float64) {
	m := p.mod
	beat := m.ctx.Beat()
	if m.hitJumps >= m.requiredJumps && hitJumpsPersist >= m.requiredJumpsP {
		p.inst.PlayState("", "EndPop", beat, 0.5)
		m.stopAll()
		m.destroyPlatforms(beat+2, p.endBeat-2, p.endBeat+1)
		m.player.floatUp(beat)
		m.stars.devolve()
		return
	}
	m.raiseHeight(beat, p.lastUnits, p.nextUnits)
	m.player.jump(beat, false)
}

func (p *platformInst) miss() {
	m := p.mod
	if p.nextSame {
		m.player.walk()
		m.ctx.SoundAt(p.endBeat+0.5, "open"+string(rune('0'+p.typ)), 1)
		m.ctx.At(p.endBeat+0.5, func() { p.inst.PlayState("", "Note", p.endBeat+0.5, 0.5) })
		return
	}
	m.stopAll()
	m.destroyPlatforms(p.endBeat+2, p.endBeat-2, p.endBeat+6)
	m.ctx.Sound("wot")
	m.player.hide()
	p.inst.SetActive(m.cfg.fallYan, true)
	p.inst.PlayState(m.cfg.fallYan, "FallSmear", m.ctx.Beat(), 0.5)
}

func (p *platformInst) justRollHold(state float64) {
	m := p.mod
	beat := m.ctx.Beat()
	p.canKick = false
	m.ctx.ScheduleInputActionRelease(p.endBeat+0.5, actionAlt, func(state float64, _ engine.Judgment) {
		p.justRollRelease(state)
	}, p.rollMissRelease)
	if math.Abs(state) >= 1 {
		p.inst.PlayState("", "FlowerBarely", beat, 0.5)
		return
	}
	m.player.roll(beat)
	m.ctx.Sound(highJumpSound(5))
	p.inst.PlayState("", "Flower", beat, 0.5)
}

func (p *platformInst) justRollRelease(state float64) {
	m := p.mod
	beat := m.ctx.Beat()
	p.canRelease = false
	if p.isEnd && m.hitJumps >= m.requiredJumps && hitJumpsPersist >= m.requiredJumpsP {
		p.playRollState("EndPop", beat, 0.5)
		m.stopAll()
		m.destroyPlatforms(beat+2, p.endBeat-2, p.endBeat+1)
		m.player.floatUp(beat)
		m.stars.devolve()
	} else {
		m.raiseHeight(beat, p.lastUnits, p.nextUnits)
		m.player.highJump(beat, p.isFinal, math.Abs(state) >= 1)
	}
	if p.isFish {
		m.player.shock(true)
		p.inst.PlayState(m.cfg.fish, "Shock", beat, 0.5)
		m.stopAll()
		m.destroyPlatforms(beat+2, p.endBeat-2, p.endBeat+6)
		m.ctx.At(beat+4, func() {
			m.player.fall(beat + 4)
			p.inst.PlayState(m.cfg.fish, "FishIdle", beat+4, 0.5)
		})
	}
	if math.Abs(state) >= 1 {
		m.ctx.Sound("ng")
		if !p.isEnd {
			p.playRollState("UmbrellaBarely", beat, 0.5)
		}
		return
	}
	m.ctx.Sound(highJumpSound(7))
	if !p.isEnd {
		p.playRollState("Umbrella", beat, 0.5)
	}
	m.stars.evolve(m.evolveAmount * 2)
	m.hitJumps += 2
	hitJumpsPersist += 2
}

func (p *platformInst) rollMissHold() {
	m := p.mod
	m.ctx.ScheduleInputActionRelease(p.endBeat+0.5, actionAlt, func(state float64, _ engine.Judgment) {
		p.justRollRelease(state)
	}, p.rollMissRelease).NoScore = true
	m.player.walk()
	m.ctx.SoundAt(p.endBeat+0.5, "open"+string(rune('0'+p.typ)), 1)
	m.ctx.At(p.endBeat+0.5, func() { p.inst.PlayState("", "Note", p.endBeat+0.5, 0.5) })
}

func (p *platformInst) rollMissRelease() {
	m := p.mod
	if p.nextSame && !p.isEnd {
		m.player.walk()
		m.ctx.SoundAt(p.endBeat+0.5, "open"+string(rune('0'+p.typ)), 1)
		m.ctx.At(p.endBeat+0.5, func() { p.playRollState("Note", p.endBeat+0.5, 0.5) })
		return
	}
	m.stopAll()
	m.destroyPlatforms(p.endBeat+1.5, p.endBeat-2, p.endBeat+6)
	m.ctx.Sound("wot")
	m.player.hide()
	p.inst.SetActive(m.cfg.fallYanRoll, true)
	p.inst.PlayState(m.cfg.fallYanRoll, "FallSmear", m.ctx.Beat(), 0.5)
}

func (p *platformInst) disappear(beat float64) {
	p.inst.PlayState("", "Destroy", beat, 0.5)
	p.playRollState("Destroy", beat, 0.5)
	p.mod.ctx.Sound("disappear")
	if p.inst.CurrentState(p.mod.cfg.fallYan) == "FallSmear" || p.inst.CurrentState(p.mod.cfg.fallYanRoll) == "FallSmear" {
		p.mod.ctx.Sound("fall")
		p.isFalling = true
		p.fallBeat = beat
	}
}

func (p *platformInst) playRollState(state string, beat, timeScale float64) {
	// Unity reuses JumpPlatform.controller on rollPlatform/RodHolder while some
	// clips still bind sibling paths under rollPlatform/*. Drive both roots so
	// the roll rod and long panels are sampled from the same state.
	p.inst.PlayState(p.mod.cfg.rollPlatform, state, beat, timeScale)
	p.inst.PlayStateLayer("roll-root:"+state, "", state, beat, timeScale)
}
