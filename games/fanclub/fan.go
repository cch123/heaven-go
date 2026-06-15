package fanclub

import (
	"math"

	"hsdemo/kart"
)

type fan struct {
	inst          *kart.Instance
	x, y          float64
	player        bool
	jumpStart     float64
	clapStartBeat float64
	stopBeat      bool
	stopCharge    bool
	hasJumped     bool
}

func (m *Module) spawnFans() {
	if m.fanTemplate == nil {
		return
	}
	origin := nodePos(m.ctx, roleOr(m.ctx, "spectatorAnchor", "fan_SpawnAnchor"))
	spawnX, spawnY := origin[0]-radius*2*3, origin[1]
	row := 1
	for i := 0; i < fanCount; i++ {
		in := m.fanTemplate.NewInstance()
		f := &fan{inst: in, x: spawnX, y: spawnY, jumpStart: math.Inf(-1), clapStartBeat: math.Inf(-1)}
		f.player = i == 3
		in.Offset = [2]float64{spawnX, spawnY}
		in.SetGroupOrder(row)
		in.PlayDefaultState("", 0, m.ctx.SecPerBeat(0))
		m.fans = append(m.fans, f)
		spawnX += radius * 2
		if i == 2 {
			spawnX += radius * 2
		}
		if i == 8 {
			spawnX += radius * 4
		}
		if i == 5 {
			spawnX = origin[0] - radius*2*4 + radius
			spawnY = origin[1] - radius
			row++
		}
	}
}

func (f *fan) play(m *Module, state string, beat float64) {
	if f == nil || f.inst == nil {
		return
	}
	f.inst.PlayState("", state, beat, m.ctx.SecPerBeat(beat))
}

func (f *fan) bop(m *Module, beat float64) {
	if f.stopBeat {
		return
	}
	f.play(m, "FanBeat", beat)
}

func (f *fan) clapStart(m *Module, beat float64, hit, charge bool, releaseAfter float64) {
	if !hit {
		m.angerOnMiss()
	}
	if m.noJudgement {
		m.noJudgementInput = true
	}
	f.hasJumped = false
	f.stopBeat = true
	f.jumpStart = math.Inf(-1)
	f.clapStartBeat = beat
	f.play(m, "FanClap", beat)
	m.ctx.SoundVol("play_clap", 1)
	m.ctx.SoundVol("crap_impact", 1)
	if charge {
		m.ctx.At(beat+0.1, func() {
			f.play(m, "FanClapCharge", beat+0.1)
			f.stopCharge = true
		})
	}
	if releaseAfter > 0 && !charge {
		m.ctx.At(beat+releaseAfter, func() { f.free(m, beat+releaseAfter) })
	}
}

func (f *fan) free(m *Module, beat float64) {
	f.play(m, "FanFree", beat)
	f.stopBeat = false
	f.stopCharge = false
	f.clapStartBeat = math.Inf(-1)
}

func (f *fan) jumpStartNow(m *Module, beat float64, hit bool) {
	if !hit {
		m.angerOnMiss()
	}
	f.play(m, "FanJump", beat)
	m.ctx.SoundVol("play_jump", 1)
	f.jumpStart = beat
	f.clapStartBeat = math.Inf(-1)
	f.stopCharge = false
}

func (f *fan) update(m *Module, beat float64) {
	if f == nil || f.inst == nil {
		return
	}
	if beat >= f.jumpStart && beat < f.jumpStart+1 {
		f.hasJumped = true
		u := beat - f.jumpStart
		yw := parabola01(u)
		f.inst.SetPos("root_motion", 0, 3*yw)
		s := (1 - yw*0.8) * 1.4
		f.inst.SetScale("fan_Shadow", s, s)
		f.inst.PlayState("", "FanJump", f.jumpStart, m.ctx.SecPerBeat(f.jumpStart))
		return
	}
	f.inst.SetPos("root_motion", 0, 0)
	f.inst.SetScale("fan_Shadow", 1.4, 1.4)
	if f.hasJumped {
		m.ctx.SoundPitch("landing_impact", 0.25, 0.98)
		if f.player {
			f.play(m, "FanPrepare", beat)
			f.stopBeat = false
		}
	}
	f.hasJumped = false
}

func (f *fan) queue(sc *kart.SceneInst, beat float64) {
	if f != nil && f.inst != nil {
		f.inst.Queue(sc, beat, kart.Identity(), 0)
	}
}
