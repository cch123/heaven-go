package tossboys

func (m *Module) dispenseSound(beat float64, who int, call bool) {
	m.ctx.SoundAt(beat, "ballStart"+kidColor(who, true), 1)
	if !call {
		return
	}
	switch who {
	case kidAka:
		for _, name := range []string{"blueRedHigh", "yellowRedHigh"} {
			m.ctx.SoundAt(beat, name+"1", 1)
			m.ctx.SoundAt(beat+0.25, name+"2", 1)
			m.ctx.SoundAt(beat+0.5, name+"3", 1)
		}
	case kidAo:
		for _, name := range []string{"redBlueHigh", "yellowBlueHigh"} {
			m.ctx.SoundAt(beat, name+"1", 1)
			m.ctx.SoundAt(beat+0.5, name+"2", 1)
		}
	case kidKii:
		for _, name := range []string{"redYellowHigh", "blueYellowHigh"} {
			m.ctx.SoundAt(beat, name+"1", 1)
			m.ctx.SoundAt(beat+0.5, name+"2", 1)
		}
	}
}

func (m *Module) playPassSounds(beat float64, last, current, suffix string, secondBeat, secondOffset, thirdOffset float64) {
	base := last + current + suffix
	m.ctx.SoundAt(beat, base+"1", 1)
	if secondOffset != 0 {
		m.ctx.SoundAtOff(beat+secondBeat, base+"2", 1, secondOffset)
	} else {
		m.ctx.SoundAt(beat+secondBeat, base+"2", 1)
	}
	thirdBeat := beat + 1
	needThird := suffix == "" && secondBeat == 0.5 && (last+current == "blueRed" || last+current == "yellowRed" || last+current == "redYellow")
	if suffix != "" && secondBeat == 0.25 {
		needThird = true
		thirdBeat = beat + 0.5
	}
	if needThird {
		if thirdOffset != 0 {
			m.ctx.SoundAtOff(thirdBeat, base+"3", 1, thirdOffset)
		} else {
			m.ctx.SoundAt(thirdBeat, base+"3", 1)
		}
	}
}
