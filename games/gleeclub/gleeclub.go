// Package gleeclub ports Glee Club's call-and-response singing, conductor
// baton cues, Together Now yell, presence toggles, palette controls, and
// pitch-shifted looped wails.
package gleeclub

import (
	"image/color"
	"math"
	"sort"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"hsdemo/engine"
	"hsdemo/kart"
	"hsdemo/riq"
)

const (
	kidLeft = iota
	kidMiddle
	kidPlayer
)

const (
	mouthBoth = iota
	mouthOnlyOpen
	mouthOnlyClose
)

var (
	defaultWhite = hex(0xff, 0xff, 0xff)
	defaultBlack = hex(0x00, 0x00, 0x00)
	defaultHeart = hex(0xf6, 0x93, 0xfe)
	defaultFloor = hex(0xff, 0xff, 0xff)
	defaultWall  = hex(0xce, 0xcf, 0xce)
	stageFill    = color.NRGBA{R: 0xce, G: 0xcf, B: 0xce, A: 0xff}
)

type queuedSinging struct {
	startBeat                      float64
	length                         float64
	semiTones, semiTonesPlayer     int
	closeMouth                     int
	repeating                      bool
	semiTonesLeft2, semiTonesLeft3 int
	semiTonesMiddle2               int
}

type singInterval struct {
	start, length float64
	queue         []queuedSinging
	passed        bool
}

type charColorEvt struct {
	beat                 float64
	main, outline, heart [4]float64
}

type bgColorEvt struct {
	beat, length         float64
	floorStart, floorEnd [4]float64
	wallStart, wallEnd   [4]float64
	ease                 int
}

type colorEase struct {
	beat, length float64
	from, to     [4]float64
	ease         int
}

type chorusKid struct {
	mod        *Module
	path       string
	spritePath string
	player     bool

	currentPitch          float64
	gameSwitchFadeOutTime float64
	singing               bool
	disappeared           bool
	shouldMegaClose       bool
	stopLoop              func()
}

type Module struct {
	ctx  *engine.Ctx
	proj kart.Aff

	kids [3]*chorusKid

	heartPath string
	condPath  string
	kidMat    string
	bgMat     string

	currentInterval *singInterval
	intervals       []*singInterval

	missed bool

	charEvents  []charColorEvt
	bgEvents    []bgColorEvt
	charMain    [4]float64
	charOutline [4]float64
	charHeart   [4]float64
	floorEase   colorEase
	wallEase    colorEase
}

func New() engine.Module { return &Module{} }

func (m *Module) ID() string { return "gleeClub" }

func (m *Module) Load(ctx *engine.Ctx) error {
	m.ctx = ctx
	if err := ctx.LoadAssets("gleeClub"); err != nil {
		return err
	}
	m.proj = kart.Translate(engine.ScreenW/2, engine.ScreenH/2).Mul(kart.Scale(54, -54))

	game := ctx.Assets.Extra.Components["game"]
	m.heartPath = roleOr(ctx, "heartAnim", game.Refs["heartAnim"], "Heart")
	m.condPath = roleOr(ctx, "condAnim", game.Refs["condAnim"], "Conductor")
	m.kidMat = game.Refs["kidMaterial"]
	m.bgMat = game.Refs["bgMaterial"]
	if m.kidMat == "" {
		m.kidMat = "ChorusKidRGB"
	}
	if m.bgMat == "" {
		m.bgMat = "BackgroundRGB"
	}

	m.kids[kidLeft] = m.loadKid(kidLeft, "leftChorusKid", "ChorusKid", false)
	m.kids[kidMiddle] = m.loadKid(kidMiddle, "middleChorusKid", "ChorusKid (1)", false)
	m.kids[kidPlayer] = m.loadKid(kidPlayer, "playerChorusKid", "Player", true)
	m.charMain, m.charOutline, m.charHeart = defaultWhite, defaultBlack, defaultHeart
	m.floorEase = colorEase{from: defaultFloor, to: defaultFloor}
	m.wallEase = colorEase{from: defaultWall, to: defaultWall}
	m.applyCharPalette()
	m.applyBackgroundPalette(0)
	m.resetScene(0)
	return nil
}

func (m *Module) loadKid(idx int, role, fallback string, player bool) *chorusKid {
	key := "kid" + string(rune('0'+idx))
	comp := m.ctx.Assets.Extra.Components[key]
	path := roleOr(m.ctx, role, comp.Path, fallback)
	sprite := comp.Refs["sr"]
	if sprite == "" {
		sprite = path + "/Sprite"
	}
	return &chorusKid{
		mod: m, path: path, spritePath: sprite, player: player,
		currentPitch: 1, stopLoop: func() {},
	}
}

func roleOr(ctx *engine.Ctx, role, comp, fallback string) string {
	if p := ctx.Role(role); p != "" {
		return p
	}
	if comp != "" {
		return comp
	}
	return fallback
}

func (m *Module) OnEvent(e *riq.Entity) {
	b := e.Beat
	switch e.Datamodel {
	case "gleeClub/intervalStart":
		m.currentInterval = m.newInterval(b, lengthOr(e.Length, 1))
	case "gleeClub/sing":
		m.scheduleSing(e)
	case "gleeClub/passTurn":
		m.scheduleCurrentPassTurn(b, e.Length)
	case "gleeClub/baton":
		m.schedulePreBaton(b)
	case "gleeClub/togetherNow":
		m.scheduleTogetherNow(b, intParam(e, "semiTones", -1), intParam(e, "semiTones1", 4),
			intParam(e, "semiTonesPlayer", 10), e.Float("pitch", 1))
	case "gleeClub/colorChar":
		ev := charColorEvt{
			beat:    b,
			main:    colorParam(e, "colorR", defaultWhite),
			outline: colorParam(e, "colorB", defaultBlack),
			heart:   colorParam(e, "colorG", defaultHeart),
		}
		m.charEvents = append(m.charEvents, ev)
		m.ctx.At(b, func() { m.setCharColors(ev.main, ev.outline, ev.heart) })
	case "gleeClub/colorBG":
		ev := bgColorEvt{
			beat: b, length: e.Length,
			floorStart: colorParam(e, "startR", defaultFloor),
			floorEnd:   colorParam(e, "endR", defaultFloor),
			wallStart:  colorParam(e, "startG", defaultWall),
			wallEnd:    colorParam(e, "endG", defaultWall),
			ease:       int(e.Float("ease", 0)),
		}
		m.bgEvents = append(m.bgEvents, ev)
		m.ctx.At(b, func() { m.setBackgroundEase(ev) })
	case "gleeClub/forceSing":
		left, mid, player := intParam(e, "semiTones", 0), intParam(e, "semiTones1", 0), intParam(e, "semiTonesPlayer", 0)
		m.ctx.At(b, func() { m.forceSing(b, left, mid, player) })
	case "gleeClub/presence":
		left, mid, player := !boolParam(e, "left"), !boolParam(e, "middle"), !boolParam(e, "player")
		m.ctx.At(b, func() { m.toggleKidsPresence(left, mid, player) })
	case "gleeClub/fadeOutTime":
		f0, f1, fp := e.Float("fade", 0), e.Float("fade1", 0), e.Float("fadeP", 0)
		m.ctx.At(b, func() {
			m.kids[kidLeft].gameSwitchFadeOutTime = f0
			m.kids[kidMiddle].gameSwitchFadeOutTime = f1
			m.kids[kidPlayer].gameSwitchFadeOutTime = fp
		})
	}
}

func (m *Module) Ready() {
	sort.Slice(m.charEvents, func(i, j int) bool { return m.charEvents[i].beat < m.charEvents[j].beat })
	sort.Slice(m.bgEvents, func(i, j int) bool { return m.bgEvents[i].beat < m.bgEvents[j].beat })
	for _, iv := range m.intervals {
		if !iv.passed {
			m.schedulePassTurn(iv.start+iv.length, 0, iv)
			iv.passed = true
		}
	}
}

func (m *Module) OnSwitch(beat float64) {
	m.persistColor(beat)
	m.resetScene(beat)
	if !m.ctx.PressingNow() && !m.ctx.App.Autoplay && !m.kids[kidPlayer].disappeared {
		m.kids[kidPlayer].startSinging(false, beat)
		m.kids[kidLeft].missPose(beat)
		m.kids[kidMiddle].missPose(beat)
	}
}

func (m *Module) Whiff(beat float64) {
	if m.kids[kidPlayer].disappeared {
		return
	}
	m.kids[kidPlayer].stopSinging(false, true, beat)
	m.kids[kidLeft].missPose(beat)
	m.kids[kidMiddle].missPose(beat)
	m.ctx.ScoreMiss()
}

func (m *Module) Update(_, beat float64) {
	if !m.kids[kidPlayer].disappeared && m.ctx.ReleasedNow() && !m.ctx.ExpectingReleaseNow() {
		m.kids[kidPlayer].startSinging(false, beat)
		m.kids[kidLeft].missPose(beat)
		m.kids[kidMiddle].missPose(beat)
		m.ctx.ScoreMiss()
	}
}

func (m *Module) Draw(screen *ebiten.Image, _, beat float64) {
	screen.Fill(stageFill)
	m.applyBackgroundPalette(beat)
	m.ctx.SampleScene(beat)
	m.ctx.Scene.Draw(screen, m.proj)
}

func (m *Module) resetScene(beat float64) {
	sec := m.ctx.SecPerBeat(beat)
	m.ctx.Scene.PlayDefaultState(m.heartPath, beat, sec)
	m.ctx.Scene.PlayDefaultState(m.condPath, beat, sec)
	for _, k := range m.kids {
		k.killLoop()
		k.singing = false
		k.shouldMegaClose = false
		m.ctx.Scene.SetBool(k.path, "Mega", false)
		m.ctx.Scene.SetColorOver(k.spritePath, [4]float64{1, 1, 1, 1})
		m.ctx.Scene.PlayDefaultState(k.path, beat, sec)
	}
}

func (m *Module) newInterval(beat, length float64) *singInterval {
	iv := &singInterval{start: beat, length: length}
	m.intervals = append(m.intervals, iv)
	return iv
}

func (m *Module) intervalForSing(beat, length float64) *singInterval {
	if m.currentInterval == nil || beat > m.currentInterval.start+m.currentInterval.length {
		m.currentInterval = m.newInterval(beat, length)
	}
	return m.currentInterval
}

func (m *Module) scheduleSing(e *riq.Entity) {
	b, length := e.Beat, lengthOr(e.Length, 1)
	iv := m.intervalForSing(b, length)
	closeMouth := intParam(e, "close", mouthBoth)
	leftPitch := pitchFromSemitones(intParam(e, "semiTones", 4))
	m.ctx.At(b, func() {
		if closeMouth != mouthOnlyClose {
			m.kids[kidLeft].currentPitch = leftPitch
			m.kids[kidLeft].startSinging(false, b)
		}
	})
	m.ctx.At(b+length, func() {
		if closeMouth != mouthOnlyOpen {
			m.kids[kidLeft].stopSinging(false, true, b+length)
		}
	})
	// The original game queues the left kid's phrase and, at PassTurn, replays
	// it for the middle kid and player. Keeping this as an interval queue avoids
	// future events overwriting runtime state before their pass turn arrives.
	iv.queue = append(iv.queue, queuedSinging{
		startBeat: b - iv.start, length: length,
		semiTones:        intParam(e, "semiTones1", 4),
		semiTonesPlayer:  intParam(e, "semiTonesPlayer", 4),
		closeMouth:       closeMouth,
		repeating:        boolParam(e, "repeat"),
		semiTonesLeft2:   intParam(e, "semiTonesLeft2", 0),
		semiTonesLeft3:   intParam(e, "semiTonesLeft3", 0),
		semiTonesMiddle2: intParam(e, "semiTonesMiddle2", 0),
	})
}

func (m *Module) scheduleCurrentPassTurn(beat, length float64) {
	if m.currentInterval == nil {
		return
	}
	m.schedulePassTurn(beat, length, m.currentInterval)
	m.currentInterval.passed = true
	m.currentInterval = nil
}

func (m *Module) schedulePassTurn(beat, length float64, iv *singInterval) {
	if iv == nil || len(iv.queue) == 0 {
		return
	}
	queue := append([]queuedSinging(nil), iv.queue...)
	beatInterval := iv.length
	m.ctx.At(beat, func() {
		m.missed = false
		if !m.kids[kidPlayer].disappeared {
			m.showHeart(beat + length + beatInterval*2 + 1)
		}
		for _, sing := range queue {
			m.scheduleQueuedSinging(beat, length, beatInterval, sing)
		}
	})
}

func (m *Module) scheduleQueuedSinging(passBeat, passLength, beatInterval float64, sing queuedSinging) {
	inputBeat := passBeat + passLength + sing.startBeat + beatInterval
	if !m.kids[kidPlayer].disappeared {
		m.scheduleSingInput(inputBeat, sing.length, sing.closeMouth, pitchFromSemitones(sing.semiTonesPlayer))
	}
	start := passBeat + passLength + sing.startBeat
	stop := start + sing.length
	midPitch := pitchFromSemitones(sing.semiTones)
	leftPitch2 := pitchFromSemitones(sing.semiTonesLeft2)
	leftPitch3 := pitchFromSemitones(sing.semiTonesLeft3)
	midPitch2 := pitchFromSemitones(sing.semiTonesMiddle2)
	m.ctx.At(start, func() {
		if sing.closeMouth != mouthOnlyClose {
			m.kids[kidMiddle].currentPitch = midPitch
			m.kids[kidMiddle].startSinging(false, start)
			if sing.repeating {
				m.kids[kidLeft].currentPitch = leftPitch2
				m.kids[kidLeft].startSinging(false, start)
			}
		}
	})
	m.ctx.At(stop, func() {
		if sing.closeMouth != mouthOnlyOpen {
			m.kids[kidMiddle].stopSinging(false, true, stop)
			if sing.repeating {
				m.kids[kidLeft].stopSinging(false, true, stop)
			}
		}
	})
	repeatStart := start + beatInterval
	repeatStop := stop + beatInterval
	m.ctx.At(repeatStart, func() {
		if sing.closeMouth != mouthOnlyClose && sing.repeating {
			m.kids[kidMiddle].currentPitch = midPitch2
			m.kids[kidLeft].currentPitch = leftPitch3
			m.kids[kidMiddle].startSinging(false, repeatStart)
			m.kids[kidLeft].startSinging(false, repeatStart)
		}
	})
	m.ctx.At(repeatStop, func() {
		if sing.closeMouth != mouthOnlyOpen && sing.repeating {
			m.kids[kidMiddle].stopSinging(false, true, repeatStop)
			m.kids[kidLeft].stopSinging(false, true, repeatStop)
		}
	})
}

func (m *Module) scheduleSingInput(beat, length float64, closeMouth int, pitch float64) {
	shouldClose := closeMouth != mouthOnlyOpen
	shouldOpen := closeMouth != mouthOnlyClose
	if shouldOpen {
		m.ctx.ScheduleInputRelease(beat, func(float64, engine.Judgment) {
			if !m.kids[kidPlayer].singing {
				m.kids[kidPlayer].currentPitch = pitch
				m.kids[kidPlayer].startSinging(false, m.ctx.Beat())
			}
		}, func() { m.missed = true })
	}
	if shouldClose {
		m.ctx.ScheduleInput(beat+length, func(float64, engine.Judgment) {
			m.kids[kidPlayer].stopSinging(false, true, m.ctx.Beat())
		}, func() { m.missed = true })
	}
}

func (m *Module) schedulePreBaton(beat float64) {
	m.ctx.SoundAt(beat, "BatonUp", 1)
	m.ctx.SoundAt(beat+1, "BatonDown", 1)
	if active := m.firstActiveBeatAtOrAfter(beat); !math.IsNaN(active) {
		// If the preFunction ran while Glee Club was inactive, Unity queues the
		// Baton call until the object is active. Sounds still play at authored time.
		visualBeat := beat
		if active > beat {
			visualBeat = active
		}
		m.scheduleBaton(visualBeat)
	}
}

func (m *Module) scheduleBaton(beat float64) {
	m.ctx.At(beat, func() {
		m.missed = false
		if !m.kids[kidPlayer].disappeared {
			m.ctx.ScheduleInput(beat+1, func(float64, engine.Judgment) {
				m.kids[kidPlayer].stopSinging(false, true, m.ctx.Beat())
				m.showHeart(beat + 2)
			}, func() { m.missed = true })
		}
		m.ctx.Scene.PlayState(m.condPath, "ConductorBatonUp", beat, 0.5)
	})
	m.ctx.At(beat+1, func() {
		m.ctx.Scene.PlayState(m.condPath, "ConductorBatonDown", beat+1, 0.5)
		m.kids[kidLeft].stopSinging(false, true, beat+1)
		m.kids[kidMiddle].stopSinging(false, true, beat+1)
	})
	m.ctx.At(beat+2, func() {
		m.ctx.Scene.PlayState(m.condPath, "ConductorIdle", beat+2, m.ctx.SecPerBeat(beat+2))
	})
}

func (m *Module) scheduleTogetherNow(beat float64, semiLeft, semiMiddle, semiPlayer int, conductorPitch float64) {
	for i, name := range []string{"togetherEN-01", "togetherEN-02", "togetherEN-03"} {
		m.ctx.SoundAtPitchPan(beat+0.5+0.5*float64(i), name, 1, conductorPitch, 0)
	}
	m.ctx.SoundAtPitchOff(beat+2, "togetherEN-04", 1, conductorPitch, 0.02)
	playerPitch := pitchFromSemitones(semiPlayer)
	m.ctx.At(beat, func() {
		if !m.kids[kidPlayer].disappeared {
			m.ctx.ScheduleInputRelease(beat+2.5, func(float64, engine.Judgment) {
				m.kids[kidPlayer].currentPitch = playerPitch
				m.kids[kidPlayer].startYell(m.ctx.Beat())
			}, func() {})
			m.ctx.ScheduleInput(beat+3.5, func(float64, engine.Judgment) {
				m.kids[kidPlayer].stopSinging(true, true, m.ctx.Beat())
			}, func() { m.missed = true })
		}
	})
	leftPitch, middlePitch := pitchFromSemitones(semiLeft), pitchFromSemitones(semiMiddle)
	m.ctx.At(beat+1.5, func() {
		m.kids[kidLeft].startCrouch(beat + 1.5)
		m.kids[kidMiddle].startCrouch(beat + 1.5)
		m.kids[kidPlayer].startCrouch(beat + 1.5)
	})
	m.ctx.At(beat+2.5, func() {
		m.kids[kidLeft].currentPitch = leftPitch
		m.kids[kidMiddle].currentPitch = middlePitch
		m.kids[kidLeft].startYell(beat + 2.5)
		m.kids[kidMiddle].startYell(beat + 2.5)
	})
	m.ctx.At(beat+3.5, func() {
		m.kids[kidLeft].stopSinging(true, true, beat+3.5)
		m.kids[kidMiddle].stopSinging(true, true, beat+3.5)
	})
	m.ctx.At(beat+6, func() {
		if !m.kids[kidPlayer].disappeared {
			m.showHeart(beat + 6)
		}
	})
}

func (m *Module) forceSing(beat float64, left, middle, player int) {
	m.kids[kidLeft].currentPitch = pitchFromSemitones(left)
	m.kids[kidMiddle].currentPitch = pitchFromSemitones(middle)
	m.kids[kidPlayer].currentPitch = pitchFromSemitones(player)
	m.kids[kidLeft].startSinging(true, beat)
	m.kids[kidMiddle].startSinging(true, beat)
	if !m.ctx.PressingNow() || m.ctx.App.Autoplay {
		m.kids[kidPlayer].startSinging(true, beat)
	} else {
		m.missed = true
	}
}

func (m *Module) toggleKidsPresence(left, middle, player bool) {
	m.kids[kidLeft].togglePresence(left)
	m.kids[kidMiddle].togglePresence(middle)
	m.kids[kidPlayer].togglePresence(player)
}

func (m *Module) showHeart(beat float64) {
	m.ctx.At(beat, func() {
		if m.missed {
			m.kids[kidLeft].missPose(beat)
			m.kids[kidMiddle].missPose(beat)
			return
		}
		m.ctx.Scene.PlayState(m.heartPath, "HeartIdle", beat, m.ctx.SecPerBeat(beat))
	})
	m.ctx.At(beat+2, func() {
		m.ctx.Scene.PlayState(m.heartPath, "HeartNothing", beat+2, m.ctx.SecPerBeat(beat+2))
	})
}

func (m *Module) firstActiveBeatAtOrAfter(beat float64) float64 {
	if m.ctx.GameAt(beat) == m.ID() {
		return beat
	}
	for _, e := range m.ctx.Entities() {
		if e.Beat < beat || !strings.HasPrefix(e.Datamodel, "gameManager/switchGame/") {
			continue
		}
		if strings.TrimPrefix(e.Datamodel, "gameManager/switchGame/") == m.ID() {
			return e.Beat
		}
	}
	return math.NaN()
}

func (k *chorusKid) togglePresence(disappear bool) {
	if disappear {
		k.mod.ctx.Scene.SetColorOver(k.spritePath, [4]float64{1, 1, 1, 0})
		k.stopSinging(false, false, k.mod.ctx.Beat())
		k.mod.ctx.Scene.PlayState(k.path, "Idle", k.mod.ctx.Beat(), k.mod.ctx.SecPerBeat(k.mod.ctx.Beat()))
		k.disappeared = true
		return
	}
	k.disappeared = false
	k.mod.ctx.Scene.SetColorOver(k.spritePath, [4]float64{1, 1, 1, 1})
	if k.player && !k.mod.ctx.PressingNow() && !k.mod.ctx.App.Autoplay {
		k.startSinging(false, k.mod.ctx.Beat())
		k.mod.kids[kidLeft].missPose(k.mod.ctx.Beat())
		k.mod.kids[kidMiddle].missPose(k.mod.ctx.Beat())
	}
}

func (k *chorusKid) missPose(beat float64) {
	state, _ := k.mod.ctx.Scene.StateInfo(k.path, beat)
	if !k.singing && state == "Idle" {
		k.mod.ctx.Scene.PlayState(k.path, "MissIdle", beat, k.mod.ctx.SecPerBeat(beat))
	}
}

func (k *chorusKid) startCrouch(beat float64) {
	if k.singing || k.disappeared {
		return
	}
	k.mod.ctx.Scene.PlayState(k.path, "CrouchStart", beat, k.mod.ctx.SecPerBeat(beat))
}

func (k *chorusKid) startYell(beat float64) {
	if k.singing || k.disappeared {
		return
	}
	k.singing = true
	k.mod.ctx.Scene.SetBool(k.path, "Mega", true)
	k.mod.ctx.Scene.PlayState(k.path, "OpenMouth", beat, k.mod.ctx.SecPerBeat(beat))
	k.shouldMegaClose = true
	k.killLoop()
	k.mod.ctx.Sound("LoudWailStart")
	k.stopLoop = k.mod.ctx.SoundLoopPitchVol("LoudWailLoop", k.currentPitch, 1)
	k.stopLoopAtNextSwitch(beat)
	k.mod.ctx.At(beat+1, func() { k.unYell(beat + 1) })
}

func (k *chorusKid) unYell(beat float64) {
	state, _ := k.mod.ctx.Scene.StateInfo(k.path, beat)
	if k.singing && state != "YellIdle" {
		k.mod.ctx.Scene.PlayState(k.path, "YellIdle", beat, k.mod.ctx.SecPerBeat(beat))
	}
}

func (k *chorusKid) startSinging(forced bool, beat float64) {
	if (k.singing && !forced) || k.disappeared {
		return
	}
	k.singing = true
	k.mod.ctx.Scene.SetBool(k.path, "Mega", false)
	k.shouldMegaClose = false
	k.mod.ctx.Scene.PlayState(k.path, "OpenMouth", beat, k.mod.ctx.SecPerBeat(beat))
	k.killLoop()
	k.stopLoop = k.mod.ctx.SoundLoopPitchVol("WailLoop", k.currentPitch, 1)
	k.stopLoopAtNextSwitch(beat)
}

func (k *chorusKid) stopSinging(mega bool, playSound bool, beat float64) {
	if !k.singing || k.disappeared {
		return
	}
	k.singing = false
	if mega {
		k.mod.ctx.Scene.SetBool(k.path, "Mega", true)
	}
	state := "CloseMouth"
	if mega {
		state = "MegaCloseMouth"
	}
	k.mod.ctx.Scene.PlayState(k.path, state, beat, k.mod.ctx.SecPerBeat(beat))
	k.killLoop()
	if playSound {
		k.mod.ctx.Sound("StopWail")
	}
}

func (k *chorusKid) killLoop() {
	if k.stopLoop != nil {
		k.stopLoop()
	}
	k.stopLoop = func() {}
}

func (k *chorusKid) stopLoopAtNextSwitch(beat float64) {
	next := k.mod.ctx.NextSwitchBeat(beat)
	if math.IsInf(next, 1) {
		return
	}
	k.mod.ctx.At(next, func() {
		if k.mod.ctx.GameAt(next+1e-6) != k.mod.ID() {
			k.killLoop()
		}
	})
}

func (m *Module) setCharColors(main, outline, heart [4]float64) {
	m.charMain, m.charOutline, m.charHeart = main, outline, heart
	m.applyCharPalette()
}

func (m *Module) applyCharPalette() {
	m.ctx.Scene.SetPaletteFor(m.kidMat, kart.Palette{
		Alpha: m.charMain, Fill: m.charHeart, Outline: m.charOutline,
	})
}

func (m *Module) setBackgroundEase(ev bgColorEvt) {
	m.floorEase = colorEase{beat: ev.beat, length: ev.length, from: ev.floorStart, to: ev.floorEnd, ease: ev.ease}
	m.wallEase = colorEase{beat: ev.beat, length: ev.length, from: ev.wallStart, to: ev.wallEnd, ease: ev.ease}
}

func (m *Module) applyBackgroundPalette(beat float64) {
	floor := m.floorEase.at(beat)
	wall := m.wallEase.at(beat)
	m.ctx.Scene.SetPaletteFor(m.bgMat, kart.Palette{Alpha: floor, Fill: wall, Outline: defaultWhite})
}

func (m *Module) persistColor(beat float64) {
	m.setCharColors(defaultWhite, defaultBlack, defaultHeart)
	for _, ev := range m.charEvents {
		if ev.beat >= beat {
			break
		}
		m.setCharColors(ev.main, ev.outline, ev.heart)
	}
	m.floorEase = colorEase{from: defaultFloor, to: defaultFloor}
	m.wallEase = colorEase{from: defaultWall, to: defaultWall}
	for _, ev := range m.bgEvents {
		if ev.beat >= beat {
			break
		}
		m.setBackgroundEase(ev)
	}
}

func (e colorEase) at(beat float64) [4]float64 {
	if e.length <= 0 {
		return e.to
	}
	u := clamp01((beat - e.beat) / e.length)
	return [4]float64{
		engine.Ease(e.ease, e.from[0], e.to[0], u),
		engine.Ease(e.ease, e.from[1], e.to[1], u),
		engine.Ease(e.ease, e.from[2], e.to[2], u),
		engine.Ease(e.ease, e.from[3], e.to[3], u),
	}
}

func pitchFromSemitones(semi int) float64 {
	return math.Exp2(float64(semi) / 12)
}

func lengthOr(v, def float64) float64 {
	if v <= 0 {
		return def
	}
	return v
}

func intParam(e *riq.Entity, key string, def int) int {
	return int(e.Float(key, float64(def)))
}

func boolParam(e *riq.Entity, key string) bool { return e.Float(key, 0) != 0 }

func colorParam(e *riq.Entity, key string, def [4]float64) [4]float64 {
	v, ok := e.Data[key]
	if !ok {
		return def
	}
	m, ok := v.(map[string]any)
	if !ok {
		return def
	}
	return [4]float64{
		num(m["r"], def[0]),
		num(m["g"], def[1]),
		num(m["b"], def[2]),
		num(m["a"], def[3]),
	}
}

func num(v any, def float64) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	default:
		return def
	}
}

func hex(r, g, b uint8) [4]float64 {
	return [4]float64{float64(r) / 255, float64(g) / 255, float64(b) / 255, 1}
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
