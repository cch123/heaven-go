package tossboys

func (m *Module) setReceiver(who int) {
	m.currentKid = who
}

func (m *Module) currentReceiver() *kidState { return m.receiver(m.currentKid) }

func (m *Module) receiver(who int) *kidState {
	if k, ok := m.kids[who]; ok {
		return k
	}
	return nil
}

func (m *Module) specialPath() string {
	switch m.currentKid {
	case kidAo:
		return roleOr(m.ctx, "specialAo", "SpecialOverlay/Aokun")
	case kidKii:
		return roleOr(m.ctx, "specialKii", "SpecialOverlay/Kiiyan")
	default:
		return roleOr(m.ctx, "specialAka", "SpecialOverlay/Akachan")
	}
}

func (m *Module) doSpecial(beat float64) {
	m.unspecial()
	m.specialKid = m.currentKid
	if k := m.currentReceiver(); k != nil {
		k.crouchPrepare(beat)
	}
	path := m.specialPath()
	m.ctx.Scene.SetActive(path, true)
	m.ctx.Scene.PlayState(path, "FadeIn", beat, 0.5)
	switch m.currentKid {
	case kidAka:
		m.ctx.SoundAt(beat, "redSpecial1", 1)
		m.ctx.SoundAt(beat+0.25, "redSpecial2", 1)
		m.ctx.SoundAtOff(beat+0.25, "redSpecialCharge", 1, 0.085)
	case kidAo:
		m.ctx.SoundAt(beat, "blueSpecial1", 1)
		m.ctx.SoundAt(beat+0.25, "blueSpecial2", 1)
	case kidKii:
		m.ctx.SoundAt(beat, "yellowSpecial", 1)
	}
}

func (m *Module) unspecial() {
	for _, path := range []string{
		roleOr(m.ctx, "specialAka", "SpecialOverlay/Akachan"),
		roleOr(m.ctx, "specialAo", "SpecialOverlay/Aokun"),
		roleOr(m.ctx, "specialKii", "SpecialOverlay/Kiiyan"),
	} {
		m.ctx.Scene.SetActive(path, false)
	}
	if k := m.receiver(m.specialKid); k != nil {
		k.crouch = false
	}
	m.specialKid = kidNone
}
