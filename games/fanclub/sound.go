package fanclub

func (m *Module) playSequence(name string, beat float64) {
	for _, c := range fanClubSeq(name) {
		m.ctx.SoundAt(beat+c.beat, c.name, c.vol)
	}
}

type seqClip struct {
	beat float64
	name string
	vol  float64
}

func fanClubSeq(name string) []seqClip {
	const jp = "jp/"
	switch name {
	case "arisa_hai":
		return []seqClip{{0, jp + "arisa_hai_1_jp", 1}, {1, jp + "arisa_hai_2_jp", 1}, {2, jp + "arisa_hai_3_jp", 1}}
	case "crowd_hai":
		return []seqClip{{0, jp + "crowd_hai_jp", 1}}
	case "arisa_kamone":
		return []seqClip{{0, jp + "arisa_ka_jp", 1}, {0.5, jp + "arisa_mo_jp", 1}, {1, jp + "arisa_ne_jp", 1}}
	case "arisa_kamone_fast":
		return []seqClip{{0, jp + "arisa_ka_fast_jp", 1}, {0.4, jp + "arisa_mo_fast_jp", 1}, {0.8, jp + "arisa_ne_fast_jp", 1}}
	case "arisa_iina":
		return []seqClip{{0, jp + "arisa_ii_jp", 1}, {0.5, jp + "arisa_na_jp", 1}}
	case "arisa_iina_fast":
		return []seqClip{{0, jp + "arisa_ii_fast_jp", 1}, {0.5, jp + "arisa_na_fast_jp", 1}}
	case "crowd_kamone":
		return []seqClip{{0, jp + "crowd_ka_jp", 1}, {0.5, jp + "crowd_mo_jp", 1}, {1, jp + "crowd_ne_jp", 1}}
	case "crowd_iina":
		return []seqClip{{0, jp + "crowd_ii_jp", 1}, {0.5, jp + "crowd_na_jp", 1}}
	case "crowd_big_ready":
		return []seqClip{{0, "crowd_big_ready", 1}}
	}
	return nil
}
