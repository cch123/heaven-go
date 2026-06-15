package rapmen

import (
	"math"

	"hsdemo/engine"
	"hsdemo/kart"
)

func (m *Module) scheduleRap(ev rapEvt) {
	if !ev.mute {
		m.scheduleRapSounds(ev)
	}
	switch ev.cue {
	case "desuka":
		m.rapDesuka(ev)
	case "kamone":
		m.rapKamone(ev)
	case "saiko":
		m.rapSaiko(ev)
	}
}

func (m *Module) scheduleRapSounds(ev rapEvt) {
	for _, s := range rapSounds(ev) {
		s := s
		m.ctx.SoundAtPitchPanOff(ev.beat+s.beat, soundName(s.clip), s.vol, 1, s.pan, s.offset)
	}
}

func rapSounds(ev rapEvt) []soundCue {
	voice := ev.voice
	if ev.gender == 1 {
		voice = ev.womanVoice
	}
	switch ev.cue {
	case "desuka":
		return desukaSounds(ev.gender == 1, voice)
	case "kamone":
		return kamoneSounds(ev.gender == 1, voice)
	case "saiko":
		return saikoSounds(ev.gender == 1, voice)
	default:
		return nil
	}
}

func (m *Module) rapDesuka(ev rapEvt) {
	text := m.rapText(ev)
	short := (ev.gender == 0 && ev.voice == 4) || (ev.gender == 1 && ev.womanVoice == 3)
	start := ev.beat
	target := ev.beat + 3
	if short {
		start = ev.beat - 0.25
		target = ev.beat + 2
	}
	m.ctx.At(start, func() {
		m.setText(text, 1)
		m.redCanBop = false
		anim := "yo"
		if short {
			anim = "kamone"
		}
		m.rapperAnim("Red", anim, start)
	})
	if !short {
		for _, off := range []float64{1, 2} {
			b := ev.beat + off
			m.ctx.At(b, func() { m.rapperAnim("Red", "yo", b) })
		}
		m.ctx.At(ev.beat+2.5, func() { m.rapperAnim("Player", "prepare", ev.beat+2.5) })
	} else {
		m.ctx.At(ev.beat+1.5, func() { m.rapperAnim("Player", "prepare", ev.beat+1.5) })
	}
	m.scheduleInput(target, false)
}

func (m *Module) rapKamone(ev rapEvt) {
	text := m.rapText(ev)
	m.ctx.At(ev.beat, func() {
		m.setText(text, 2)
		m.redCanBop = false
		m.rapperAnim("Red", "kamone", ev.beat)
	})
	m.ctx.At(ev.beat+1.25, func() { m.rapperAnim("Red", "yo", ev.beat+1.25) })
	m.ctx.At(ev.beat+2, func() {
		m.rapperAnim("Player", "prepare", ev.beat+2)
		m.clearText()
	})
	m.ctx.At(ev.beat+3, func() { m.rapperAnim("Player", "prepare", ev.beat+3) })
	m.scheduleInput(ev.beat+2.5, false)
	m.scheduleInput(ev.beat+3.5, false)
}

func (m *Module) rapSaiko(ev rapEvt) {
	text := m.rapText(ev)
	rapAnim := "saiko"
	if ev.voice >= 5 {
		rapAnim = "yo"
	}
	m.ctx.At(ev.beat, func() {
		m.setText(text, 3)
		m.redCanBop = false
		m.rapperAnim("Red", rapAnim, ev.beat)
	})
	m.ctx.At(ev.beat+1, func() { m.rapperAnim("Red", rapAnim, ev.beat+1) })
	m.ctx.At(ev.beat+1.5, func() { m.rapperAnim("Player", "prepare", ev.beat+1.5) })
	m.scheduleInput(ev.beat+2, false)
	m.scheduleInput(ev.beat+2.5, true)
}

func (m *Module) scheduleInput(beat float64, alt bool) {
	m.ctx.ScheduleInput(beat, func(state float64, _ engine.Judgment) {
		m.rapJust(beat, state, alt)
	}, func() { m.rapMiss(beat) })
}

func (m *Module) rapJust(beat, state float64, alt bool) {
	if state >= 1 || state <= -1 {
		m.ctx.SoundPitchPan("drum", m.drumVolume, m.drumPitch, 0.25)
	} else {
		m.ctx.SoundPitchPan("cymbal", m.cymbalVolume, m.cymbalPitch, 0.25)
		m.ctx.SoundPitchPan("drum", math.Max(0, m.drumVolume-0.3), m.drumPitch, 0.25)
		m.flashParticles(beat)
	}
	if m.yellowWoman {
		if alt {
			m.ctx.SoundPitchPan("rapWomen/uhnnnW", 1, 1, 0.25)
		} else {
			m.ctx.SoundPitchPan("rapWomen/uhnW", 1, 1, 0.25)
		}
	} else if alt {
		m.ctx.SoundPitchPan("uhnnn", 1, 1, 0.25)
	} else {
		m.ctx.SoundPitchPan("uhn", 1, 1, 0.25)
	}
	m.rapperAnim("Red", "just", beat)
	m.rapperAnim("Player", "just", beat)
	m.redCanBop = true
	m.clearText()
	m.showUhn(beat)
}

func (m *Module) rapMiss(beat float64) {
	m.clearText()
	m.ctx.SoundPitchPan("miss", 1, 1, 0.25)
	m.rapperAnim("Player", "miss", beat)
	m.redCanBop = true
}

func (m *Module) redBanter(ev banterEvt) {
	if ev.voice == 2 {
		if ev.gender == 1 {
			m.ctx.SoundPitchPan("rapWomen/yoW", 1, 1, -0.25)
		} else {
			m.ctx.SoundPitchPan("yo", 1, 1, -0.25)
		}
		if ev.playAnim {
			m.rapperAnim("Red", "yo", ev.beat)
		}
		return
	}
	if ev.gender == 1 {
		m.ctx.SoundPitchPanOff("rapWomen/yeahW", 1, 1, -0.25, m.tailOffset("rapWomen/yeahW", 0.391))
	} else {
		m.ctx.SoundPitchPanOff("yeah", 1, 1, -0.25, m.tailOffset("yeah", 0.391))
	}
	if ev.playAnim {
		m.rapperAnim("Red", "kamone", ev.beat)
	}
}

func (m *Module) tailOffset(name string, keepTailSec float64) float64 {
	pcm := m.ctx.Assets.Sounds[name]
	dur := float64(len(pcm)/4) / engine.SampleRate
	if off := dur - keepTailSec; off > 0 {
		return off
	}
	return 0
}

func (m *Module) setText(s string, idx int) {
	_ = m.ctx.Assets.SetText(m.text, cleanText(s))
	m.ctx.Scene.SetColorOver(m.text, textColor(idx))
}

func (m *Module) clearText() {
	_ = m.ctx.Assets.SetText(m.text, "")
}

func textColor(idx int) [4]float64 {
	switch idx {
	case 1:
		return [4]float64{0.56, 0.88, 1, 1}
	case 2:
		return [4]float64{1, 0.55, 0.9, 1}
	case 3:
		return [4]float64{1, 0.93, 0.25, 1}
	default:
		return [4]float64{1, 1, 1, 1}
	}
}

func (m *Module) applyToggle(red, yellow int) {
	m.redWoman = red == 1
	m.yellowWoman = yellow == 1
	m.ctx.Scene.SetActive(m.red, !m.redWoman)
	m.ctx.Scene.SetActive(m.cherry, m.redWoman)
	m.ctx.Scene.SetActive(m.yellow, !m.yellowWoman)
	m.ctx.Scene.SetActive(m.blue, m.yellowWoman)
}

func (m *Module) rapperAnim(who, anim string, beat float64) {
	path := m.red
	if who == "Red" && m.redWoman {
		path = m.cherry
	} else if who == "Player" {
		path = m.yellow
		if m.yellowWoman {
			path = m.blue
		}
	}
	m.ctx.Scene.PlayState(path, anim, beat, 0.5)
}

func (m *Module) lateBeatPulse(beat float64) {
	if m.redBop && m.redCanBop {
		m.rapperAnim("Red", "bop", beat)
	}
	if m.yellowBop && m.yellowCanBop {
		m.rapperAnim("Player", "bop", beat)
	}
}

func (m *Module) flashParticles(beat float64) {
	for _, p := range m.particles {
		m.ctx.Scene.SetActive(p, true)
		p := p
		m.ctx.At(beat+0.35, func() { m.ctx.Scene.SetActive(p, false) })
	}
}

func (m *Module) showUhn(beat float64) {
	m.ctx.Scene.SetActive(m.uhnParticle, true)
	m.ctx.At(beat+0.35, func() { m.ctx.Scene.SetActive(m.uhnParticle, false) })
}

func (m *Module) hideExpiredParticles(_ float64) {}

func (m *Module) applyBackgroundEvent(ev bgEvt) {
	m.bg = ev
	if ev.typ == 2 {
		m.ctx.Scene.PlayState(m.background, "backgroundWomen", ev.beat, 0.5)
	} else {
		m.ctx.Scene.PlayState(m.background, "backgroundMen", ev.beat, 0.5)
	}
}

func (m *Module) applyBackground(beat float64) {
	u := norm(beat, m.bg.beat, m.bg.length)
	a := colorAt(m.bg.ease, m.bg.a0, m.bg.a1, u)
	b := colorAt(m.bg.ease, m.bg.b0, m.bg.b1, u)
	c := colorAt(m.bg.ease, m.bg.c0, m.bg.c1, u)
	d := colorAt(m.bg.ease, m.bg.d0, m.bg.d1, u)
	m.ctx.Scene.SetPaletteFor(m.bgMat, kart.Palette{Alpha: b, Fill: c, Outline: a})
	m.ctx.Scene.SetPaletteFor(m.speakerMat, kart.Palette{Alpha: b, Fill: d, Outline: d})
}
