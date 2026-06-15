package bossanova

const (
	maleBounce1 = iota
	maleBounce3
	maleBounce4
	maleBounce6
	maleSpin
	femaleBounce2
	femaleBounce4
	femaleSpin
)

func (s *shape) voiceNormal() {
	alt := s.voiceVariant == 1
	switch s.voiceline {
	case maleBounce1:
		if alt {
			s.atOrNow(s.startBeat+1, "Bossa/SE_BOSSA_EN_4", 1)
			s.atOrNow(s.startBeat+2, "Bossa/SE_BOSSA_EN_5", 1)
		} else {
			s.atOrNow(s.startBeat+1, "Bossa/SE_BOSSA_EN_8", 0.89)
			s.atOrNow(s.startBeat+2, "Bossa/SE_BOSSA_EN_9", 1)
		}
	case maleBounce3:
		if alt {
			s.mod.ctx.Sound("Bossa/SE_BOSSA_EN_6")
		} else {
			s.mod.ctx.Sound("Bossa/SE_BOSSA_EN_10")
		}
	case maleBounce4:
		if alt {
			s.atOrNow(s.startBeat+1, "Bossa/SE_BOSSA_EN_7", 1)
		} else {
			s.atOrNow(s.startBeat+1, "Bossa/SE_BOSSA_EN_11", 0.89)
		}
		s.atOrNow(s.startBeat+2, "Bossa/SE_BOSSA_EN_5", 1)
	case maleBounce6:
		s.mod.ctx.Sound("Bossa/SE_BOSSA_EN_6")
	case maleSpin:
		s.mod.ctx.SoundVol("Bossa/SE_BOSSA_EN_12", 0.75)
	case femaleBounce2:
		if alt {
			if s.spin {
				s.atOrNow(s.startBeat+1, "Nova/SE_BOSSA_EN_30", 0.62)
			} else {
				s.atOrNow(s.startBeat+1, "Nova/SE_BOSSA_EN_31", 0.55)
			}
		} else if s.spin {
			s.atOrNow(s.startBeat+1, "Nova/SE_BOSSA_EN_28", 0.89)
		} else {
			s.atOrNow(s.startBeat+1, "Nova/SE_BOSSA_EN_29", 0.73)
		}
		s.atOrNow(s.startBeat+2, "Nova/SE_BOSSA_EN_37", 0.55)
	case femaleBounce4:
		if alt {
			s.mod.ctx.SoundVol("Nova/SE_BOSSA_EN_34", 0.89)
		} else {
			s.mod.ctx.SoundVol("Nova/SE_BOSSA_EN_34", 0.73)
		}
	case femaleSpin:
		s.mod.ctx.SoundVol("Nova/SE_BOSSA_EN_38", 0.62)
	}
}

func (s *shape) voicePlayful() {
	switch s.voiceVariant {
	case 0:
		s.voicePlayful0()
	case 1:
		s.voicePlayful1()
	case 2:
		s.voicePlayful2()
	}
}

func (s *shape) voicePlayful0() {
	switch s.voiceline {
	case maleBounce1:
		s.atOrNow(s.startBeat+1, "Bossa/SE_BOSSA_EN_13", 1)
		s.atOrNow(s.startBeat+2, "Bossa/SE_BOSSA_EN_14", 1)
	case maleBounce3:
		s.mod.ctx.Sound("Bossa/SE_BOSSA_EN_15")
	case maleBounce4:
		s.atOrNow(s.startBeat+1, "Bossa/SE_BOSSA_EN_16", 0.62)
		s.atOrNow(s.startBeat+2, "Bossa/SE_BOSSA_EN_22", 1)
	case maleBounce6:
		s.mod.ctx.Sound("Bossa/SE_BOSSA_EN_15")
	case maleSpin:
		s.playPlayfulSpin()
	case femaleBounce2:
		s.atOrNow(s.startBeat+1, "Nova/SE_BOSSA_EN_39", 0.75)
		s.atOrNow(s.startBeat+2, "Nova/SE_BOSSA_EN_37", 0.47)
	case femaleBounce4:
		s.mod.ctx.SoundVol("Nova/SE_BOSSA_EN_40", 0.75)
	case femaleSpin:
		s.mod.ctx.SoundVol("Nova/SE_BOSSA_EN_45", 0.67)
	}
}

func (s *shape) voicePlayful1() {
	switch s.voiceline {
	case maleBounce1:
		s.atOrNow(s.startBeat+1, "Bossa/SE_BOSSA_EN_17", 1)
		s.atOrNow(s.startBeat+2, "Bossa/SE_BOSSA_EN_18", 1)
	case maleBounce3:
		s.mod.ctx.Sound("Bossa/SE_BOSSA_EN_19")
	case maleBounce4:
		s.atOrNow(s.startBeat+1, "Bossa/SE_BOSSA_EN_20", 1)
		s.atOrNow(s.startBeat+2, "Bossa/SE_BOSSA_EN_14", 1)
	case maleBounce6:
		s.mod.ctx.Sound("Bossa/SE_BOSSA_EN_15")
	case maleSpin:
		s.playPlayfulSpin()
	case femaleBounce2:
		s.atOrNow(s.startBeat+1, "Nova/SE_BOSSA_EN_41", 0.75)
		s.atOrNow(s.startBeat+2, "Nova/SE_BOSSA_EN_37", 0.47)
	case femaleBounce4:
		s.mod.ctx.SoundVol("Nova/SE_BOSSA_EN_42", 0.56)
	case femaleSpin:
		s.mod.ctx.SoundVol("Nova/SE_BOSSA_EN_45", 0.67)
	}
}

func (s *shape) voicePlayful2() {
	switch s.voiceline {
	case maleBounce1:
		s.atOrNow(s.startBeat+1, "Bossa/SE_BOSSA_EN_21", 1)
		s.atOrNow(s.startBeat+2, "Bossa/SE_BOSSA_EN_22", 1)
	case maleBounce3:
		s.mod.ctx.Sound("Bossa/SE_BOSSA_EN_23")
	case maleBounce4:
		s.atOrNow(s.startBeat+1, "Bossa/SE_BOSSA_EN_24", 1)
		s.atOrNow(s.startBeat+2, "Bossa/SE_BOSSA_EN_14", 1)
	case maleBounce6:
		s.mod.ctx.SoundVol("Bossa/SE_BOSSA_EN_15", 0.75)
	case maleSpin:
		s.playPlayfulSpin()
	case femaleBounce2:
		s.atOrNow(s.startBeat+1, "Nova/SE_BOSSA_EN_43", 0.75)
		s.atOrNow(s.startBeat+2, "Nova/SE_BOSSA_EN_37", 0.47)
	case femaleBounce4:
		s.mod.ctx.SoundVol("Nova/SE_BOSSA_EN_44", 0.75)
	case femaleSpin:
		s.mod.ctx.SoundVol("Nova/SE_BOSSA_EN_45", 0.67)
	}
}

func (s *shape) playPlayfulSpin() {
	// UnityEngine.Random.Range(1, 2) with int bounds is upper-exclusive, so this
	// intentionally advances by exactly one each time instead of using Go rand.
	s.mod.playfulSpinRandomization = (s.mod.playfulSpinRandomization + 1) % 3
	switch s.mod.playfulSpinRandomization {
	case 0:
		s.mod.ctx.Sound("Bossa/SE_BOSSA_EN_25")
	case 1:
		s.mod.ctx.Sound("Bossa/SE_BOSSA_EN_26")
	case 2:
		s.mod.ctx.Sound("Bossa/SE_BOSSA_EN_27")
	}
}

func (s *shape) voiceAngry() {
	alt := s.voiceVariant == 1
	switch s.voiceline {
	case maleBounce1:
		s.atOrNow(s.startBeat+1, "Bossa/SE_BOSSA_EN_47", 0.75)
		if alt && s.spin {
			s.atOrNow(s.startBeat+2, "Bossa/SE_BOSSA_EN_5", 0.75)
		} else {
			s.atOrNow(s.startBeat+2, "Bossa/SE_BOSSA_EN_9", 0.75)
		}
	case maleBounce3:
		if alt && s.spin {
			s.mod.ctx.SoundVol("Bossa/SE_BOSSA_EN_6", 0.75)
		} else {
			s.mod.ctx.SoundVol("Bossa/SE_BOSSA_EN_10", 0.75)
		}
	case maleBounce4:
		s.atOrNow(s.startBeat+1, "Bossa/SE_BOSSA_EN_48", 0.75)
		s.atOrNow(s.startBeat+2, "Bossa/SE_BOSSA_EN_5", 0.75)
	case maleBounce6:
		s.mod.ctx.SoundVol("Bossa/SE_BOSSA_EN_6", 0.75)
	case maleSpin:
		if alt {
			s.mod.ctx.SoundVol("Bossa/SE_BOSSA_EN_49", 0.75)
		} else {
			s.mod.ctx.SoundVol("Bossa/SE_BOSSA_EN_50", 0.75)
		}
	case femaleBounce2:
		s.atOrNow(s.startBeat+1, "Nova/SE_BOSSA_EN_51", 1)
		s.atOrNow(s.startBeat+2, "Nova/SE_BOSSA_EN_37", 0.62)
	case femaleBounce4:
		if alt {
			s.mod.ctx.SoundVol("Nova/SE_BOSSA_EN_52", 0.75)
		} else {
			s.mod.ctx.SoundVol("Nova/SE_BOSSA_EN_53", 0.89)
		}
	case femaleSpin:
		if alt {
			s.mod.ctx.Sound("Nova/SE_BOSSA_EN_54")
		} else {
			s.mod.ctx.Sound("Nova/SE_BOSSA_EN_55")
		}
	}
}
