package froghop

import "hsdemo/riq"

func (m *Module) bop(blue, orange, green bool, beat float64) {
	if blue {
		m.singer.bop(beat)
	}
	if orange {
		m.leader.bop(beat)
	}
	if green {
		for _, f := range m.back {
			f.bop(beat)
		}
	}
}

func (m *Module) talk(fs []*frog, state string, end float64) {
	for _, f := range fs {
		f.talk(m.ctx.Beat(), state, end)
	}
}

func (m *Module) npcHop(fs []*frog, beat float64, long bool) {
	for _, f := range fs {
		if f != m.player {
			f.hop(beat, 0, long)
		}
	}
}

func (m *Module) npcCharge(fs []*frog, beat float64) {
	for _, f := range fs {
		if f != m.player {
			f.charge(beat, 0)
		}
	}
}

func (m *Module) npcSpin(fs []*frog, beat float64, hs bool) {
	if hs {
		m.leader.spin(beat, false)
		m.singer.spin(beat, true)
		return
	}
	for _, f := range fs {
		if f != m.player {
			f.spin(beat, false)
		}
	}
}

func (m *Module) playerHopNormal(beat, state float64) {
	if barely(state) {
		m.ctx.SoundVol("miss2", 1.5)
		m.lightMiss(false, true)
	} else {
		m.ctx.Sound("SE_NTR_FROG_EN_P_BEAT")
	}
	m.playerHop(beat, false)
}

func (m *Module) playerHopYa(beat, state float64) {
	p := pitchAt(m.ctx, beat, m.pitches)
	m.ctx.SoundPitch("SE_NTR_FROG_EN_P_HA", 1, p)
	if barely(state) {
		m.ctx.SoundVol("miss2", 1.5)
		m.lightMiss(false, true)
	} else {
		m.ctx.Sound("SE_NTR_FROG_EN_POP_DEFAULT")
	}
	m.playerHop(beat, false)
}

func (m *Module) playerHopHoo(beat, state float64) {
	p := pitchAt(m.ctx, beat, m.pitches)
	m.ctx.SoundPitch("SE_NTR_FROG_EN_P_HAAI", 1, p)
	if barely(state) {
		m.ctx.SoundVol("miss2", 1.5)
		m.lightMiss(false, true)
	} else {
		m.ctx.Sound("SE_NTR_FROG_EN_POP_HAAI")
	}
	m.playerHop(beat, true)
}

func (m *Module) playerHopYeah(beat, state float64, accent bool) {
	p := pitchAt(m.ctx, beat, m.pitches)
	m.ctx.SoundPitch("SE_NTR_FROG_EN_P_HAI", 1, p)
	if barely(state) {
		m.ctx.SoundVol("miss2", 1.5)
		m.lightMiss(false, true)
	} else {
		m.ctx.Sound("SE_NTR_FROG_EN_POP_DEFAULT")
	}
	m.playerHop(beat, accent)
}

func (m *Module) playerCharge(beat, state float64) {
	p := pitchAt(m.ctx, beat, m.pitches)
	m.ctx.SoundPitch("SE_NTR_FROG_EN_P_KURU_1", 1, p)
	m.ctx.SoundAtPitchPan(beat+0.5, "SE_NTR_FROG_EN_P_KURU_2", 1, p, 0)
	if barely(state) {
		m.ctx.SoundVol("miss2", 1.5)
		m.lightMiss(false, true)
	}
	m.globalSide *= -1
	m.player.charge(beat, m.globalSide)
}

func (m *Module) playerSpin(beat, state float64) {
	p := pitchAt(m.ctx, beat, m.pitches)
	m.ctx.SoundPitch("SE_NTR_FROG_EN_P_LIN", 1, p)
	if barely(state) {
		m.ctx.SoundVol("miss2", 1.5)
		m.lightMiss(false, false)
	}
	m.player.spin(beat, false)
}

func (m *Module) playerHop(beat float64, long bool) {
	m.globalSide *= -1
	m.player.hop(beat, m.globalSide, long)
}

func (m *Module) playerMiss(beat float64, flip bool) {
	if flip {
		m.globalSide *= -1
	}
	m.lightMiss(false, false)
	if !flip || m.globalSide > 0 {
		m.player.bump(beat)
	}
}

func (m *Module) playerMissNoFlip(beat float64) {
	m.lightMiss(false, false)
	m.player.bump(beat)
}

func (m *Module) lightMiss(whiff bool, sweat bool) {
	if whiff {
		m.ctx.ScoreMiss()
	}
	if sweat {
		m.player.sweat(m.ctx.Beat())
		return
	}
	for _, f := range m.other {
		f.glare(m.ctx.Beat())
	}
}

func (m *Module) setSpotlights(front, back, dark bool) {
	for _, f := range m.front {
		f.darken(front || !dark)
	}
	m.ctx.Scene.SetColorOver(m.mikeL, colorIf(front || !dark, white, dim))
	m.ctx.Scene.SetColorOver(m.mikeR, colorIf(front || !dark, white, dim))
	m.ctx.Scene.SetActive(m.darkness, dark)
	m.ctx.Scene.SetActive(m.spotFront, front)
	m.ctx.Scene.SetActive(m.spotBack, back)
}

func (m *Module) applyStage(ev stageEvt) {
	m.ctx.Scene.SetColorOver(m.stageTop, ev.top)
	m.ctx.Scene.SetColorOver(m.stage, mix3(ev.rim, ev.trim, ev.base))
	m.ctx.Scene.SetActive(m.mikeL, ev.mikeL)
	m.ctx.Scene.SetActive(m.mikeR, ev.mikeR)
	m.ctx.Scene.SetColorOver(m.spotFrontColor, withAlpha(ev.front, 0.5))
	m.ctx.Scene.SetColorOver(m.spotBackColor, withAlpha(ev.back, 0.5))
}

func (m *Module) applyFrogColor(ev frogColorEvt) {
	for _, f := range m.frogsForGroup(ev.group) {
		f.recolor(ev.skin, ev.tummy, ev.pants, ev.belt, ev.sclera, ev.lip, ev.lipstick, ev.hasBelt)
	}
}

func (m *Module) applyPersistent(beat float64) {
	for _, ev := range m.stages {
		if ev.beat < beat {
			m.applyStage(ev)
		}
	}
	for _, ev := range m.frogColors {
		if ev.beat < beat {
			m.applyFrogColor(ev)
		}
	}
	for _, ev := range m.spots {
		if ev.beat < beat {
			m.setSpotlights(ev.front, ev.back, ev.dark)
		}
	}
	for _, ev := range m.disables {
		if ev.beat < beat {
			m.ctx.Scene.SetActive(m.singer.path, !ev.disable)
		}
	}
}

func (m *Module) bgAt(beat float64) ([4]float64, [4]float64) {
	top, bottom := defaultBGTop, defaultBGBottom
	for _, ev := range m.bgs {
		if ev.beat > beat {
			break
		}
		u := 1.0
		if ev.length > 0 && beat < ev.beat+ev.length {
			u = clamp01((beat - ev.beat) / ev.length)
		}
		top = easeColor(ev.ease, ev.fromTop, ev.toTop, u)
		bottom = easeColor(ev.ease, ev.fromBottom, ev.toBottom, u)
	}
	return top, bottom
}

func (m *Module) frogColorEvent(e *riq.Entity, group string) frogColorEvt {
	def := frogDefaults(group)
	return frogColorEvt{
		beat: e.Beat, group: group,
		skin: colorParam(e, "color1", def[0]), tummy: colorParam(e, "color2", def[1]),
		pants: colorParam(e, "color3", def[2]), belt: colorParam(e, "color4", def[3]),
		sclera: colorParam(e, "color5", def[4]), lip: colorParam(e, "color6", def[5]),
		lipstick: boolDefault(e, "lipstick", group == "leader"), hasBelt: boolDefault(e, "belt", group != "backup"),
	}
}

func (m *Module) frogsFor(blue, orange, green bool) []*frog {
	var fs []*frog
	if blue {
		fs = append(fs, m.singer)
	}
	if orange {
		fs = append(fs, m.leader)
	}
	if green {
		fs = append(fs, m.back...)
	}
	return fs
}

func (m *Module) frogsForGroup(group string) []*frog {
	switch group {
	case "singer":
		return []*frog{m.singer}
	case "leader":
		return []*frog{m.leader}
	default:
		return m.back
	}
}

func barely(state float64) bool { return state >= 1 || state <= -1 }

func withAlpha(c [4]float64, a float64) [4]float64 { c[3] *= a; return c }

func colorIf(ok bool, yes, no [4]float64) [4]float64 {
	if ok {
		return yes
	}
	return no
}

func frogDefaults(group string) [][4]float64 {
	switch group {
	case "singer":
		return [][4]float64{
			{0x69 / 255.0, 0xa6 / 255.0, 1, 1},
			{0xac / 255.0, 0xee / 255.0, 0xe5 / 255.0, 1},
			{0x0c / 255.0, 0x59 / 255.0, 1, 1},
			{0xf9 / 255.0, 0x2d / 255.0, 0x5f / 255.0, 1},
			white,
			{0x8b / 255.0, 0x42 / 255.0, 0xc0 / 255.0, 1},
		}
	case "leader":
		return [][4]float64{
			{1, 0x95 / 255.0, 0x4e / 255.0, 1},
			{0xf9 / 255.0, 0xd7 / 255.0, 0xc4 / 255.0, 1},
			{0xf9 / 255.0, 0x2d / 255.0, 0x5f / 255.0, 1},
			{0x0c / 255.0, 0x59 / 255.0, 1, 1},
			white,
			{0xeb / 255.0, 0x36 / 255.0, 0, 1},
		}
	default:
		return [][4]float64{
			{0x3d / 255.0, 0xdf / 255.0, 0x30 / 255.0, 1},
			{1, 0xf7 / 255.0, 0x69 / 255.0, 1},
			{0x16 / 255.0, 0x54 / 255.0, 0x23 / 255.0, 1},
			{0x1e / 255.0, 0x6f / 255.0, 0x18 / 255.0, 1},
			white,
			{0xeb / 255.0, 0x36 / 255.0, 0, 1},
		}
	}
}
