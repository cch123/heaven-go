// Package djschool ports DJ School's hold/scratch call-and-response,
// DJ Yellow expressions, turntable animation, and voice/SFX timing from
// Assets/Scripts/Games/DJSchool.
package djschool

import (
	"image/color"
	"math"
	"sort"

	"github.com/hajimehoshi/ebiten/v2"

	"hsdemo/engine"
	"hsdemo/kart"
	"hsdemo/riq"
)

const gameID = "djSchool"

const (
	voiceStandard = iota
	voiceCool
	voiceHyped
)

const (
	lineCheckItOut = iota
	lineLetsGo
	lineOhYeah
	lineOhYeahAlt
	lineYay
)

const (
	headNeutralLeft = iota
	headNeutralRight
	headCrossEyed
	headHappy
	headFocused
	headUpFirst
	headUpSecond
)

const (
	djSchoolMainHighpassHz = 10
	djSchoolMainLowpassHz  = 22000
	djSchoolMainGainDB     = 0.01

	djSchoolHoldHighpassHz = 2909
	djSchoolHoldLowpassHz  = 2064
	djSchoolHoldGainDB     = 10
)

type bopEvt struct {
	beat, length float64
	bop, auto    bool
}

type fxEvt struct {
	beat float64
	on   bool
}

type flashEvt struct {
	path string
	beat float64
}

type Module struct {
	ctx  *engine.Ctx
	proj kart.Aff

	student, yellow, yellowHead string
	turntable, flash, flashInv  string
	headSprites                 []string

	bops []bopEvt
	fx   []fxEvt

	studentHolding bool
	studentMissed  bool
	studentSwiping bool
	shouldHold     bool
	yellowHolding  bool
	andStop        bool
	soundFX        bool

	headExpr      int
	headReversed  bool
	yellowBopLeft bool
	smileBeat     float64
	lastPulse     int
	canBooBeat    float64
	flashes       []flashEvt
}

func New() engine.Module {
	return &Module{lastPulse: -1, smileBeat: math.Inf(-1), canBooBeat: math.Inf(-1)}
}

func (m *Module) ID() string { return gameID }

func (m *Module) Load(ctx *engine.Ctx) error {
	m.ctx = ctx
	if err := ctx.LoadAssets(gameID); err != nil {
		return err
	}
	m.proj = kart.Translate(engine.ScreenW/2, engine.ScreenH/2).Mul(kart.Scale(54, -54))

	comp := ctx.Assets.Extra.Components
	game := comp["game"]
	student := comp["student"]
	yellow := comp["djYellow"]
	m.student = roleOr(ctx, "student", game.Refs["student"], "Student")
	m.yellow = roleOr(ctx, "djYellow", game.Refs["djYellow"], "DJ Yellow")
	m.yellowHead = firstNonEmpty(yellow.Refs["djYellowHeadSprite"], "DJ Yellow/Head")
	m.headSprites = append(m.headSprites, yellow.SpriteArrays["djYellowHeadSprites"]...)
	m.turntable = firstNonEmpty(student.Refs["TurnTable"], "TurnTable_Player")
	m.flash = firstNonEmpty(student.Refs["flashFX"], "flash")
	m.flashInv = firstNonEmpty(student.Refs["flashFXInverse"], "flashInverse")

	m.resetScene(0)
	return nil
}

func (m *Module) OnEvent(e *riq.Entity) {
	switch e.Datamodel {
	case gameID + "/bop":
		m.bops = append(m.bops, bopEvt{
			beat: e.Beat, length: e.Length,
			bop: boolDefault(e, "toggle2", true), auto: boolParam(e, "toggle"),
		})
	case gameID + "/and stop ooh":
		m.scheduleAndStop(e.Beat, boolDefault(e, "toggle", true), true)
	case gameID + "/break c'mon ooh":
		m.scheduleBreakCmon(e.Beat, int(e.Float("type", voiceStandard)), boolDefault(e, "toggle", true), true)
	case gameID + "/scratch-o hey":
		m.scheduleScratcho(e.Beat, int(e.Float("type", voiceStandard)), boolParam(e, "toggle"), boolDefault(e, "toggle2", true))
	case gameID + "/dj voice lines":
		m.scheduleVoiceLine(e.Beat, int(e.Float("type", lineCheckItOut)))
	case gameID + "/sound FX":
		m.fx = append(m.fx, fxEvt{beat: e.Beat, on: boolDefault(e, "toggle", true)})
		m.ctx.At(e.Beat, func() { m.soundFX = boolDefault(e, "toggle", true) })
	case gameID + "/forceHold":
		m.ctx.At(e.Beat, func() { m.forceHold(e.Beat) })
	}
}

func (m *Module) Ready() {
	sort.SliceStable(m.bops, func(i, j int) bool { return m.bops[i].beat < m.bops[j].beat })
	sort.SliceStable(m.fx, func(i, j int) bool { return m.fx[i].beat < m.fx[j].beat })
	for _, ev := range m.bops {
		ev := ev
		if !ev.bop {
			continue
		}
		for b := ev.beat; b < ev.beat+ev.length-1e-6; b++ {
			bb := b
			m.ctx.At(bb, func() { m.bopAll(bb) })
		}
	}
}

func (m *Module) OnSwitch(beat float64) {
	m.resetScene(beat)
	m.lastPulse = int(math.Floor(beat))
	m.persistFX(beat)
}

func (m *Module) Whiff(beat float64) {
	if m.studentHolding {
		return
	}
	m.onMissHoldForPlayerInput(beat)
	m.studentHolding = true
	m.ctx.ScoreMiss()
}

func (m *Module) Update(_, beat float64) {
	if m.ctx.ReleasedNow() && !m.ctx.ExpectingReleaseNow() && m.studentHolding {
		m.unHold(beat)
		m.shouldHold = false
		m.ctx.ScoreMiss()
	}
	if !m.ctx.App.Autoplay && m.shouldHold && !m.ctx.PressingNow() && !m.ctx.ExpectingReleaseNow() {
		m.unHold(beat)
		m.shouldHold = false
		m.ctx.ScoreMiss()
	}

	pulse := int(math.Floor(beat + 1e-6))
	if pulse > m.lastPulse {
		for b := m.lastPulse + 1; b <= pulse; b++ {
			if m.autoBopAt(float64(b)) {
				m.bopAll(float64(b))
			}
		}
		m.lastPulse = pulse
	}
}

func (m *Module) Draw(screen *ebiten.Image, _, beat float64) {
	screen.Fill(color.RGBA{0x3f, 0xd0, 0xff, 0xff})
	m.ctx.SampleScene(beat)
	m.updateFlashFX(beat)
	m.ctx.Scene.Draw(screen, m.proj)
}

func (m *Module) scheduleAndStop(beat float64, ooh, doSound bool) {
	active := true
	if doSound {
		m.ctx.At(beat, func() {
			if m.yellowHolding {
				active = false
				return
			}
			m.ctx.Sound("andStop1")
		})
		m.guardedSoundAtOff(beat+0.5, "andStop2", 1, 0.120, func() bool { return active })
		if ooh {
			m.guardedSoundAt(beat+1.5, "oohAlt", 1, func() bool { return active })
		}
	}
	m.ctx.At(beat+0.5, func() {
		if !active {
			return
		}
		m.playYellowCue("BreakCmon", beat+0.5)
		m.setYellowSpeakingHead(beat + 0.5)
	})
	m.ctx.At(beat+1.5, func() {
		if !active {
			return
		}
		m.ctx.Scene.PlayState(m.yellow, "Hold", beat+1.5, 1)
		m.yellowHolding = true
		m.reverseHead(false)
	})
	m.ctx.At(beat, func() {
		if active {
			m.andStop = true
		}
	})
	m.scheduleHoldInputCond(beat+1.5, func() bool { return active })
}

func (m *Module) scheduleBreakCmon(beat float64, typ int, ooh, doSound bool) {
	active := true
	if doSound {
		sounds := breakSounds(typ)
		m.ctx.At(beat, func() {
			if m.yellowHolding {
				active = false
				return
			}
			m.ctx.Sound(sounds[0])
		})
		m.guardedSoundAtOff(beat+1, sounds[1], 1, 0.030, func() bool { return active })
		if ooh {
			m.guardedSoundAt(beat+2, sounds[2], 1, func() bool { return active })
		}
	}
	for _, off := range []float64{0, 1} {
		off := off
		m.ctx.At(beat+off, func() {
			if !active {
				return
			}
			m.playYellowCue("BreakCmon", beat+off)
			m.setYellowSpeakingHead(beat + off)
		})
	}
	m.ctx.At(beat+2, func() {
		if !active {
			return
		}
		m.ctx.Scene.PlayState(m.yellow, "Hold", beat+2, 0.5)
		m.yellowHolding = true
		m.reverseHead(false)
	})
	m.ctx.At(beat, func() {
		if active {
			m.andStop = true
		}
	})
	m.scheduleHoldInputCond(beat+2, func() bool { return active })
}

func (m *Module) scheduleScratcho(beat float64, typ int, remix4, cheer bool) {
	sounds := scratchSounds(typ)
	timing := 2.0
	beatOffset := 2.0
	beatOffset2 := 2.05
	if remix4 {
		timing = 1.5
		beatOffset = 1.5
		beatOffset2 = 1.55
	}
	m.ctx.SoundAt(beat, sounds[0], 1)
	m.ctx.SoundAt(beat+0.25, sounds[1], 1)
	m.ctx.SoundAt(beat+0.5, sounds[2], 1)
	m.ctx.SoundAtOff(beat+1, sounds[3], 1, 0.050)
	m.ctx.SoundAtOff(beat+beatOffset, sounds[4], 1, 0.070)

	m.ctx.At(beat, func() { m.playYellowCue("Scratcho", beat) })
	m.ctx.At(beat+0.5, func() { m.playYellowCue("Scratcho2", beat+0.5) })
	m.ctx.At(beat+1, func() { m.playYellowCue("Scratcho", beat+1) })
	m.ctx.At(beat+beatOffset2, func() {
		m.playYellowCue("Hey", beat+beatOffset2)
		m.yellowHolding = false
	})

	target := beat + timing
	m.ctx.ScheduleInputRelease(target, func(state float64, _ engine.Judgment) {
		m.onHitSwipe(target, state, cheer)
	}, func() {
		m.onMissSwipe(target)
	})
	m.andStop = false
}

func (m *Module) scheduleHoldInputCond(target float64, canHit func() bool) {
	m.ctx.ScheduleInputCond(target, canHit, func(_ float64, _ engine.Judgment) {
		m.onHitHold(target)
	}, func() {
		m.onMissHold(target)
	})
}

func (m *Module) scheduleVoiceLine(beat float64, typ int) {
	switch typ {
	case lineLetsGo:
		m.ctx.SoundAt(beat, "letsGo1", 1)
		m.ctx.SoundAt(beat+0.5, "letsGo2", 1)
	case lineOhYeah:
		m.ctx.SoundAt(beat, "ohYeah1", 1)
		m.ctx.SoundAt(beat+0.5, "ohYeah2", 1)
	case lineOhYeahAlt:
		m.ctx.SoundAt(beat, "ohYeahAlt1", 1)
		m.ctx.SoundAt(beat+0.5, "ohYeahAlt2", 1)
		m.ctx.SoundAt(beat+1, "ohYeahAlt3", 1)
	case lineYay:
		m.ctx.SoundAt(beat, "yay", 1)
	default:
		m.ctx.SoundAt(beat, "checkItOut1", 1)
		m.ctx.SoundAt(beat+0.25, "checkItOut2", 1)
		m.ctx.SoundAt(beat+0.5, "checkItOut3", 1)
	}
}

func (m *Module) forceHold(beat float64) {
	m.studentHolding = true
	m.studentMissed = false
	m.shouldHold = true
	m.playStudent("Hold", beat, 1)
	m.ctx.Scene.PlayState(m.turntable, "Student_Turntable_Hold", beat, 0.5)
	m.startRadioFX(beat)
	m.ctx.Scene.PlayState(m.yellow, "Hold", beat, 1)
	m.changeHead(headFocused)
	m.yellowHolding = true
}

func (m *Module) onHitHold(beat float64) {
	m.studentHolding = true
	m.studentMissed = false
	m.shouldHold = true
	m.ctx.Sound("recordStop")
	m.playStudent("Hold", beat, 0.5)
	m.ctx.Scene.PlayState(m.turntable, "Student_Turntable_StartHold", beat, 0.5)
	m.startRadioFX(beat)
	m.flashFX(beat, true)
}

func (m *Module) onMissHold(beat float64) {
	m.boo(beat)
	m.crossEyesForMiss()
	m.studentMissed = true
}

func (m *Module) onMissHoldForPlayerInput(beat float64) {
	m.studentHolding = true
	m.studentMissed = true
	m.ctx.Sound("recordStop")
	m.playStudent("Hold", beat, 0.5)
	m.ctx.Scene.PlayState(m.turntable, "Student_Turntable_StartHold", beat, 0.5)
	m.startRadioFX(beat)
	m.crossEyesForMiss()
}

func (m *Module) onHitSwipe(target, state float64, cheer bool) {
	m.shouldHold = false
	m.studentHolding = false
	m.studentSwiping = true
	m.ctx.Sound("recordSwipe")
	m.playStudent("Swipe", target, 1)
	ng := state >= 1 || state <= -1
	if !ng && !m.studentMissed {
		m.studentMissed = false
		m.shouldHold = false
		m.flashFX(target, false)
		m.ctx.At(target+4, func() { m.studentSwiping = false })
		m.changeHead(headUpSecond)
		m.reverseHead(false)
		m.smileBeat = target + 1
		if cheer {
			m.ctx.SoundAt(target+1, "cheer", 0.8)
		}
	} else {
		m.studentMissed = true
		m.onMissSwipeForPlayerInput(target + 1)
		m.ctx.At(target+4, func() { m.studentSwiping = false })
	}
	m.ctx.Scene.PlayState(m.turntable, "Student_Turntable_Swipe", target, 0.5)
	m.stopRadioFX(target)
}

func (m *Module) onMissSwipe(target float64) {
	m.studentHolding = false
	m.studentMissed = true
	m.booAt(target+1, 0.8)
	m.ctx.At(target+1, func() {
		if m.autoBopAt(target + 1) {
			m.changeHead(headCrossEyed)
			m.reverseHead(!m.yellowHolding)
		}
	})
}

func (m *Module) onMissSwipeForPlayerInput(beat float64) {
	m.studentHolding = false
	m.studentMissed = true
	m.stopRadioFX(beat)
	m.ctx.At(beat, func() {
		if m.autoBopAt(beat) {
			m.changeHead(headCrossEyed)
			m.reverseHead(!m.yellowHolding)
		}
	})
}

func (m *Module) unHold(beat float64) {
	m.studentHolding = false
	m.playStudent("Unhold", beat, 0.5)
	m.boo(beat)
	m.studentMissed = true
	m.stopRadioFX(beat)
	m.ctx.Scene.PlayState(m.turntable, "Student_Turntable_Idle", beat, 0.5)
	m.crossEyesForMiss()
}

func (m *Module) bopAll(beat float64) {
	if m.studentHolding && !m.studentSwiping {
		m.playStudent("HoldBop", beat, 0.5)
	} else if !m.studentSwiping {
		m.playStudent("IdleBop", beat, 0.5)
	}
	if !m.andStop && !m.yellowHolding {
		if m.smileActive(beat) {
			m.changeHead(headHappy)
		} else if m.headExpr != headCrossEyed {
			m.changeHead(headNeutralLeft)
		}
		m.reverseHead(m.smileActive(beat) || m.headExpr == headCrossEyed)
		if m.yellowBopLeft {
			m.playYellowCue("IdleBop2", beat)
		} else {
			m.playYellowCue("IdleBop", beat)
		}
		m.yellowBopLeft = !m.yellowBopLeft
	} else if m.yellowHolding {
		m.playYellowCue("HoldBop", beat)
	}
}

func (m *Module) autoBopAt(beat float64) bool {
	on := false
	for _, ev := range m.bops {
		if ev.beat > beat {
			break
		}
		on = ev.auto
	}
	return on
}

func (m *Module) setYellowSpeakingHead(beat float64) {
	if m.smileActive(beat) {
		m.changeHead(headHappy)
	} else if m.headExpr != headCrossEyed {
		m.changeHead(headNeutralRight)
	}
	m.reverseHead(false)
}

func (m *Module) crossEyesForMiss() {
	if m.headExpr == headUpFirst || m.headExpr == headUpSecond {
		return
	}
	m.changeHead(headCrossEyed)
	if m.yellowHolding || m.andStop {
		m.reverseHead(false)
	} else {
		m.reverseHead(true)
	}
}

func (m *Module) playStudent(state string, beat, scale float64) {
	m.ctx.Scene.PlayState(m.student, state, beat, scale)
}

func (m *Module) playYellowCue(state string, beat float64) {
	m.ctx.Scene.PlayState(m.yellow, state, beat, 0.5)
}

func (m *Module) changeHead(expr int) {
	if expr == headUpFirst && m.headExpr == headUpSecond {
		return
	}
	if expr >= 0 && expr < len(m.headSprites) {
		m.ctx.Scene.SetSpriteOver(m.yellowHead, m.headSprites[expr])
		m.headExpr = expr
	}
}

func (m *Module) reverseHead(should bool) {
	m.headReversed = should
	m.ctx.Scene.SetMirrorX(m.yellowHead, should)
}

func (m *Module) smileActive(beat float64) bool {
	u := beat - m.smileBeat
	return u >= 0 && u <= 3
}

func (m *Module) flashFX(beat float64, inverse bool) {
	path := m.flash
	if inverse {
		path = m.flashInv
	}
	m.ctx.Scene.SetActive(path, true)
	state := "Flash"
	if inverse {
		state = "FlashInverse"
	}
	m.ctx.Scene.PlayState(path, state, beat, 1)
	m.flashes = append(m.flashes, flashEvt{path: path, beat: beat})
}

func (m *Module) updateFlashFX(beat float64) {
	active := m.flashes[:0]
	for _, f := range m.flashes {
		if beat-f.beat > 0.5 {
			m.ctx.Scene.SetActive(f.path, false)
			continue
		}
		active = append(active, f)
	}
	m.flashes = active
}

func (m *Module) boo(beat float64) {
	if beat < m.canBooBeat {
		return
	}
	m.ctx.SoundVol("boo", 0.8)
	m.canBooBeat = beat + 1
}

func (m *Module) booAt(beat, vol float64) {
	m.ctx.At(beat, func() {
		if beat < m.canBooBeat {
			return
		}
		m.ctx.SoundVol("boo", vol)
		m.canBooBeat = beat + 1
	})
}

func (m *Module) guardedSoundAt(beat float64, name string, vol float64, ok func() bool) {
	m.ctx.At(beat, func() {
		if ok() {
			m.ctx.SoundVol(name, vol)
		}
	})
}

func (m *Module) guardedSoundAtOff(beat float64, name string, vol, offset float64, ok func() bool) {
	m.ctx.At(beat, func() {
		if ok() {
			m.ctx.SoundPitchOff(name, vol, 1, offset)
		}
	})
}

func (m *Module) startRadioFX(_ float64) {
	if !m.soundFX {
		return
	}
	// Values come from Assets/Resources/MainMixer.mixer, snapshot
	// DJSchool_Hold. Student.cs transitions to it over 0.1 real seconds.
	m.ctx.TransitionMusicFilter(djSchoolHoldHighpassHz, djSchoolHoldLowpassHz, djSchoolHoldGainDB, 0.1)
}

func (m *Module) stopRadioFX(_ float64) {
	if !m.soundFX {
		return
	}
	// Student.cs returns to the Main snapshot over 0.04 real seconds.
	m.ctx.TransitionMusicFilter(djSchoolMainHighpassHz, djSchoolMainLowpassHz, djSchoolMainGainDB, 0.04)
}

func (m *Module) resetScene(beat float64) {
	m.ctx.ResetMusicFilter()
	sec := m.ctx.SecPerBeat(math.Max(beat, 0))
	m.ctx.Scene.PlayDefaultState("", beat, sec)
	m.ctx.Scene.PlayDefaultState(m.student, beat, sec)
	m.ctx.Scene.PlayDefaultState(m.yellow, beat, sec)
	m.ctx.Scene.PlayDefaultState(m.turntable, beat, sec)
	m.ctx.Scene.PlayDefaultState("TurnTable_Yellow", beat, sec)
	m.ctx.Scene.SetActive(m.flash, false)
	m.ctx.Scene.SetActive(m.flashInv, false)
	m.studentHolding, m.studentMissed, m.studentSwiping = false, false, false
	m.shouldHold, m.yellowHolding, m.andStop = false, false, false
	m.yellowBopLeft = false
	m.smileBeat = math.Inf(-1)
	m.canBooBeat = math.Inf(-1)
	m.flashes = nil
	m.headExpr = headNeutralLeft
	m.changeHead(headNeutralLeft)
	m.reverseHead(false)
}

func (m *Module) persistFX(beat float64) {
	m.soundFX = false
	for _, ev := range m.fx {
		if ev.beat >= beat {
			break
		}
		m.soundFX = ev.on
	}
}

func breakSounds(typ int) [3]string {
	switch typ {
	case voiceCool:
		return [3]string{"breakCmonAlt1", "breakCmonAlt2", "oohAlt"}
	case voiceHyped:
		return [3]string{"breakCmonLoud1", "breakCmonLoud2", "oohLoud"}
	default:
		return [3]string{"breakCmon1", "breakCmon2", "ooh"}
	}
}

func scratchSounds(typ int) [5]string {
	switch typ {
	case voiceCool:
		return [5]string{"scratchoHeyAlt1", "scratchoHeyAlt2", "scratchoHeyAlt3", "scratchoHeyAlt4", "heyAlt"}
	case voiceHyped:
		return [5]string{"scratchoHeyLoud1", "scratchoHeyLoud2", "scratchoHeyLoud3", "scratchoHeyLoud4", "heyLoud"}
	default:
		return [5]string{"scratchoHey1", "scratchoHey2", "scratchoHey3", "scratchoHey4", "hey"}
	}
}

func boolParam(e *riq.Entity, key string) bool {
	v, _ := e.Data[key].(bool)
	return v
}

func boolDefault(e *riq.Entity, key string, def bool) bool {
	if v, ok := e.Data[key].(bool); ok {
		return v
	}
	return def
}

func roleOr(ctx *engine.Ctx, role, fromComponent, fallback string) string {
	if p := ctx.Role(role); p != "" {
		return p
	}
	return firstNonEmpty(fromComponent, fallback)
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
