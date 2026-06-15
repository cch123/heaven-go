package nightwalkagb

func (m *Module) playFishCue(beat float64) {
	for _, off := range []float64{-2, -1.5, -1} {
		m.ctx.SoundAt(beat+off, "common_count-ins_cowbell", 1)
	}
}

func (m *Module) walkingCountIn(beat, length float64) {
	for i := 0; i < int(length); i++ {
		b := beat + float64(i)
		m.ctx.SoundAt(b, "boxKick", 1)
		m.ctx.SoundAt(b+0.5, "open1", 1)
	}
}

func (m *Module) scheduleFillSound(beat float64, fill int) {
	third := 1.0 / 3.0
	add := func(off float64, name string) {
		m.ctx.At(beat+off, func() {
			if !m.stopped {
				m.ctx.Sound(name)
			}
		})
	}
	switch fill {
	case fillPattern1:
		add(-third*2, "fill1A")
		add(-0.5, "fill1B")
		add(-third, "fill1C")
		add(-third*0.5, "fill1D")
	case fillPattern2:
		add(-third*2, "fill2A")
		add(-0.5, "fill2B")
		add(-third*0.5, "fill2C")
	case fillPattern3:
		add(-third*2, "fill3A")
		add(-0.5, "fill3B")
	}
}
