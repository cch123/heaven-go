package powercalligraphy

import "hsdemo/kmdata"

const (
	fudeModeIdle = iota
	fudeModeTap
	fudeModeHalt
	fudeModeSweep
	fudeModeSweepEnd
	fudeModePrepare
)

type fudeState struct {
	hand, thumb string
	stick, tip  string
	ball        string
	red1, red2  float64
	redRate     float64
	mode        int
	startBeat   float64
	timeScale   float64
	lastStick   int
	lastTip     int
}

func newFudeState(comp kmdata.Component) fudeState {
	return fudeState{
		hand:      comp.Refs["handRenderer"],
		thumb:     comp.Refs["thumbRenderer"],
		stick:     comp.Refs["stickRenderer"],
		tip:       comp.Refs["tipRenderer"],
		ball:      comp.Refs["ballRenderer"],
		red1:      numDefault(comp.Nums, "REDRATE_1", 0.1),
		red2:      numDefault(comp.Nums, "REDRATE_2", 0.25),
		mode:      fudeModeIdle,
		timeScale: 1,
	}
}

func (m *Module) playFude(state string, beat, timeScale float64) {
	m.ctx.Scene.PlayState(m.fudeAnim, state, beat, timeScale)
	m.fude.startBeat = beat
	if timeScale > 0 {
		m.fude.timeScale = timeScale
	}
	switch state {
	case "fude-none":
		m.fude.mode = fudeModeIdle
	case "fude-tap":
		m.fude.mode = fudeModeTap
	case "fude-halt":
		m.fude.mode = fudeModeHalt
	case "fude-sweep":
		m.fude.mode = fudeModeSweep
	case "fude-sweep-end":
		m.fude.mode = fudeModeSweepEnd
	case "fude-prepare":
		// fude-prepare has no AnimationEvents in Unity. Keep the last brush
		// sprites while the transform clip moves the brush into place.
		m.fude.mode = fudeModePrepare
	}
}

func (m *Module) updateFude(beat float64) {
	stick, tip, ok := m.fude.frame(beat)
	if ok {
		m.fude.lastStick, m.fude.lastTip = stick, tip
	} else {
		stick, tip = m.fude.lastStick, m.fude.lastTip
	}
	red := m.fude.red()
	m.ctx.Scene.SetSpriteOver(m.fude.hand, "hand_"+itoa(red))
	m.ctx.Scene.SetSpriteOver(m.fude.thumb, "thumb_"+itoa(red))
	m.ctx.Scene.SetSpriteOver(m.fude.stick, "fude_stick_"+itoa(stick)+"_"+itoa(red))
	m.ctx.Scene.SetSpriteOver(m.fude.tip, "fude_tip_"+itoa(tip)+"_"+itoa(red))
	m.ctx.Scene.SetSpriteOver(m.fude.ball, "fude_ball_"+itoa(red))
}

func (f *fudeState) red() int {
	switch {
	case f.redRate >= f.red2:
		return 2
	case f.redRate >= f.red1:
		return 1
	default:
		return 0
	}
}

func (f *fudeState) frame(beat float64) (stick, tip int, ok bool) {
	elapsed := (beat - f.startBeat) * f.timeScale
	if elapsed < 0 {
		elapsed = 0
	}
	switch f.mode {
	case fudeModeIdle:
		return 0, 0, true
	case fudeModeTap:
		return 0, 12, true
	case fudeModeHalt:
		return haltFrame(elapsed), haltFrame(elapsed) + 7, true
	case fudeModeSweep:
		frame := sweepFrame(elapsed)
		return sweepStickTip(frame)
	case fudeModeSweepEnd:
		frame := sweepEndFrame(elapsed)
		return sweepStickTip(frame)
	case fudeModePrepare:
		return 0, 0, false
	default:
		return 0, 0, true
	}
}

func haltFrame(elapsed float64) int {
	switch {
	case elapsed < 1.0/60:
		return 0
	case elapsed < 2.0/60:
		return 2
	case elapsed < 3.0/60:
		return 3
	default:
		return 4
	}
}

func sweepFrame(elapsed float64) int {
	switch {
	case elapsed < 1.0/60:
		return 0
	case elapsed < 2.0/60:
		return 2
	case elapsed < 3.0/60:
		return 3
	case elapsed < 4.0/60:
		return 4
	case elapsed < 1.0/15:
		return 5
	default:
		loopT := (elapsed - 1.0/15) * 0.4
		idx := int(loopT/(1.0/60)) % 3
		if idx == 1 {
			return 7
		}
		return 6
	}
}

func sweepEndFrame(elapsed float64) int {
	switch {
	case elapsed < 1.0/60:
		return 5
	case elapsed < 2.0/60:
		return 4
	case elapsed < 3.0/60:
		return 3
	case elapsed < 4.0/60:
		return 2
	default:
		return 0
	}
}

func sweepStickTip(frame int) (stick, tip int, ok bool) {
	if frame <= 5 {
		return 0, frame + 1, true
	}
	return 2, frame%2 + 5, true
}

func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	var buf [16]byte
	i := len(buf)
	n := v
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
