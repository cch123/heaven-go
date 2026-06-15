package supersamuraislice

import (
	"math"

	"hsdemo/engine"
	"hsdemo/kart"
)

const (
	demonRightUp = iota + 1
	demonLeftUp
	demonRightDown
	demonLeftDown
	demonLeft
	demonRight
)

const (
	demonLive = iota
	demonDead
)

type smallPlan struct {
	beat, boomAt, flipBack float64
	count                  int
	enableBoom             bool
	slashIdx               int
}

type bigPlan struct {
	beat, boomDelay float64
	invert          bool
	enableBoom      bool
}

type activeDemon struct {
	inst      *kart.Instance
	path      curvePath
	startBeat float64
	deathBeat float64
	state     int
	mirror    bool
	big       bool
}

func (m *Module) spawnSmall(p smallPlan) {
	if m.smallT == nil {
		return
	}
	d := &activeDemon{
		inst:      m.smallT.NewInstance(),
		startBeat: p.beat,
		deathBeat: p.boomAt + 16,
		path:      m.paths[smallPathName(p.count)],
		mirror:    p.count == demonLeftUp || p.count == demonLeftDown,
	}
	d.inst.PlayDefaultState("", p.beat, m.ctx.SecPerBeat(p.beat))
	if d.mirror {
		d.inst.Scale = [2]float64{-1, 1}
	}
	m.demons = append(m.demons, d)
	m.playSmallAppear(p.beat, p.count)
	m.ctx.ScheduleInputAny(p.beat+2,
		func(state float64, _ engine.Judgment) { m.hitSmall(d, p, state) },
		func() { m.missSmall(d) },
	)
	if p.count == demonLeftUp || p.count == demonLeftDown {
		m.flipSamurai(p.beat+1, true)
		m.flipSamurai(p.beat+2+p.flipBack, false)
	}
}

func (m *Module) playSmallAppear(beat float64, count int) {
	switch count {
	case demonRightDown, demonLeftDown:
		m.ctx.SoundAt(beat, "SE_IAI_NEW_ENEMY_SMALL", 1)
		if m.waterActive && !m.eagleActive {
			m.ctx.SoundAtPitchPan(beat+1, "SE_IAI_NEW_ENEMY_SMALL_WATER", 1, 1+m.rng.Float64()*0.2, 0)
		}
	default:
		m.ctx.SoundAt(beat, "SE_IAI_NEW_ENEMY_SMALL", 1)
	}
}

func (m *Module) hitSmall(d *activeDemon, p smallPlan, state float64) {
	if d.state == demonDead {
		return
	}
	if p.slashIdx > 4 {
		p.slashIdx = 4
	}
	m.playSamurai(smallSlashState(p.slashIdx), m.ctx.Beat())
	explodeType := smallExplodeType(p.slashIdx)
	d.state = demonDead
	if p.enableBoom {
		m.ctx.SoundAt(p.boomAt, "SE_IAI_NEW_ENEMY_DIE_SMALL", 1)
	}
	if math.Abs(state) >= 1 {
		d.inst.PlayState("", "Break_Miss", m.ctx.Beat(), 0.5)
		m.ctx.Sound("SE_IAI_NEW_OSII")
	} else {
		m.playSmallBreak(d, p.slashIdx)
	}
	m.ctx.At(p.boomAt, func() {
		d.inst.PlayState("", "disappear", p.boomAt, 0.5)
		m.effects = append(m.effects, effect{beat: p.boomAt, typ: explodeType, pos: d.position(m.ctx.Beat())})
	})
}

func (m *Module) playSmallBreak(d *activeDemon, idx int) {
	switch idx {
	case 0:
		m.ctx.Sound("SE_IAI_NEW_HIT1")
		d.inst.PlayState("", "Break_H", m.ctx.Beat(), 0.5)
	case 1:
		m.ctx.Sound("SE_IAI_NEW_HIT1")
		d.inst.PlayState("", "Break_V", m.ctx.Beat(), 0.5)
	case 2:
		m.ctx.Sound("SE_IAI_NEW_HIT2")
		d.inst.PlayState("", "Break_V", m.ctx.Beat(), 0.5)
	case 3:
		m.ctx.Sound("SE_IAI_NEW_HIT_KICK")
		d.inst.PlayState("", "Break_K", m.ctx.Beat(), 0.5)
	default:
		m.ctx.Sound("SE_IAI_NEW_KIAI_BARRIER_BASE")
		m.ctx.SoundPitchPan("SE_IAI_NEW_KIAI_BARRIER_SHOCK", 1, 1, m.rng.Float64()*1.7)
		d.inst.PlayState("", "Break_P", m.ctx.Beat(), 0.5)
		m.effects = append(m.effects, effect{beat: m.ctx.Beat(), typ: effectLightning, pos: d.position(m.ctx.Beat())})
	}
}

func (m *Module) missSmall(d *activeDemon) {
	if d.state == demonDead {
		return
	}
	d.state = demonDead
	d.deathBeat = m.ctx.Beat() + 1
	m.ctx.Sound("SE_IAI_NEW_YARARE1")
	m.playSamurai("Slash_Through_R", m.ctx.Beat())
}

func (m *Module) spawnBig(p bigPlan) {
	if m.mediumT == nil || m.largeDemonActive {
		return
	}
	d := &activeDemon{
		inst:      m.mediumT.NewInstance(),
		startBeat: p.beat,
		deathBeat: p.beat + 20,
		big:       true,
		mirror:    p.invert,
	}
	if p.invert {
		d.path = m.paths["CurveL"]
		d.inst.Scale = [2]float64{-1, 1}
		m.direction = 1
		m.flipSamurai(p.beat+1, true)
		m.flipSamurai(p.beat+6, false)
	} else {
		d.path = m.paths["CurveR"]
		m.direction = 0
	}
	d.inst.PlayDefaultState("", p.beat, m.ctx.SecPerBeat(p.beat))
	m.demons = append(m.demons, d)
	m.largeDemonActive = true
	m.ctx.SoundAt(p.beat, "SE_IAI_NEW_ENEMY_MID_1", 1)
	m.ctx.SoundAt(p.beat+0.5, "SE_IAI_NEW_ENEMY_MID_2", 1)
	m.ctx.SoundAt(p.beat+1, "SE_IAI_NEW_ENEMY_MID_3", 1)
	m.ctx.ScheduleInputActionCond(p.beat+3, actionAlt,
		func() bool { return d.state != demonDead },
		func(_ float64, _ engine.Judgment) { m.holdBig(d, p, m.ctx.Beat()) },
		func() { m.missHoldBig(d, p.beat+3) },
	)
}

func (m *Module) holdBig(d *activeDemon, p bigPlan, beat float64) {
	m.playPlatformGuard(beat)
	m.ctx.SoundAt(beat, "SE_IAI_NEW_GUARD", 1)
	m.ctx.SoundAt(beat+0.5, "SE_IAI_NEW_GUARD", 1)
	m.ctx.SoundAt(beat+0.75, "SE_IAI_NEW_GUARD", 1)
	m.ctx.SoundAt(beat+1, "SE_IAI_NEW_GUARD", 1)
	m.ctx.SoundAt(beat+1.5, "SE_IAI_NEW_GUARD", 1)
	m.playSamurai("Guard", beat)
	d.inst.PlayState("", "Attack", beat, 0.5)
	m.ctx.ScheduleInputActionReleaseCond(beat+2, actionAlt,
		func() bool { return d.state != demonDead },
		func(_ float64, _ engine.Judgment) { m.releaseBig(d, p, m.ctx.Beat()) },
		func() { m.missReleaseBig(d, beat+2) },
	)
}

func (m *Module) missHoldBig(d *activeDemon, beat float64) {
	m.playPlatformGuard(beat)
	for _, off := range []float64{0, 0.5, 0.75, 1, 1.5} {
		m.ctx.SoundAt(beat+off, "SE_IAI_NEW_YARARE1", 1)
	}
	m.playSamurai("Guard_Through", beat)
	d.inst.PlayState("", "Attack", beat, 0.5)
	m.ctx.At(beat+2, func() {
		m.largeDemonActive = false
		m.platformIdle(beat + 2)
		m.playSamurai("Counter_Through", beat+2)
		d.inst.PlayState("", "GoAway", beat+2, 0.5)
		m.ctx.Sound("SE_IAI_NEW_YARARE1")
		d.state, d.deathBeat = demonDead, beat+3
	})
}

func (m *Module) releaseBig(d *activeDemon, p bigPlan, beat float64) {
	m.largeDemonActive = false
	m.platformIdle(beat)
	m.playSamurai("Counter", beat)
	m.ctx.Scene.PlayState(m.flash, "flash", beat, 0.5)
	d.inst.PlayState("", "Break", beat, 0.5)
	m.ctx.Sound("SE_IAI_NEW_HIT_MID")
	boom := beat + p.boomDelay
	if p.enableBoom {
		m.ctx.SoundAt(boom, "SE_IAI_NEW_ENEMY_DIE_MID", 1)
	}
	m.ctx.At(beat+1, func() { m.ctx.Scene.PlayState(m.flash, "idle", beat+1, 0.5) })
	m.ctx.At(boom, func() {
		d.inst.PlayState("", "disappear", boom, 0.5)
		d.state = demonDead
		d.deathBeat = boom + 16
		m.effects = append(m.effects, effect{beat: boom, typ: effectBoom, pos: d.position(m.ctx.Beat())})
	})
}

func (m *Module) missReleaseBig(d *activeDemon, beat float64) {
	m.largeDemonActive = false
	m.platformIdle(beat)
	m.playSamurai("Counter_Through", beat)
	d.inst.PlayState("", "GoAway", beat, 0.5)
	m.ctx.Sound("SE_IAI_NEW_YARARE1")
	d.state = demonDead
	d.deathBeat = beat + 1
}

func (d *activeDemon) update(beat float64) {
	d.inst.Offset = d.position(beat)
}

func (d *activeDemon) position(beat float64) [2]float64 {
	return d.path.eval(beat - d.startBeat)
}

func smallPathName(count int) string {
	switch count {
	case demonLeftUp:
		return "leftUp"
	case demonRightDown:
		return "rightDown"
	case demonLeftDown:
		return "leftDown"
	default:
		return "rightUp"
	}
}

func smallSlashState(idx int) string {
	switch idx {
	case 1:
		return "Slash01"
	case 2:
		return "Slash02"
	case 3:
		return "Kick"
	case 4:
		return "Punch"
	default:
		return "Slash00"
	}
}

func smallExplodeType(idx int) int {
	switch idx {
	case 0:
		return 2
	case 3:
		return 3
	default:
		return 1
	}
}
