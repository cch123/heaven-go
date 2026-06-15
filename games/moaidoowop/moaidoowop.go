// Package moaidoowop ports Moai Doo-Wop's call-and-response vocals,
// day/night background, moai position swaps, palette/accessory changes, and
// miss splash effects from Assets/Scripts/Games/MoaiDooWop/MoaiDooWop.cs.
package moaidoowop

import (
	"image/color"
	"math"
	"sort"

	"github.com/hajimehoshi/ebiten/v2"

	"hsdemo/engine"
	"hsdemo/kart"
	"hsdemo/riq"
)

const gameID = "moaiDooWop"

var (
	stageColor          = color.RGBA{0x01, 0x98, 0x93, 0xff}
	defaultHead         = [4]float64{0.345098, 0.3137255, 0.1254902, 1}
	defaultGlassesFrame = [4]float64{0.9725, 0, 0, 1}
	defaultLens         = [4]float64{0.5, 0.313725, 0, 1}
)

type bopEvt struct {
	beat, length float64
	auto, bop    bool
}

type cueEvt struct {
	beat, length float64
	shout        bool
}

type intervalEvt struct {
	idx                int
	beat, length       float64
	autoPass, autoMoai bool
	autoBeat           float64
}

type passEvt struct {
	beat float64
}

type timeEvt struct {
	beat, length float64
	tod, ease    int
}

type moveEvt struct {
	beat              float64
	cpuPos, playerPos int
	instant           bool
}

type visualState struct {
	mHead, mFrame, mLens [4]float64
	mGlasses             bool
	mHat                 int
	fHead, fFrame, fLens [4]float64
	fGlasses             bool
	fHat                 int
}

type colorEvt struct {
	beat float64
	v    visualState
}

type bgState struct {
	active       bool
	beat, length float64
	clip         string
	ease         int
	finalNight   bool
}

type missBurst struct {
	beat   float64
	index  int
	anchor string
}

type Module struct {
	ctx  *engine.Ctx
	proj kart.Aff

	cpuMoai, playerMoai string
	cpuMove, playerMove string
	bg                  string

	maleMat, femaleMat string
	glassesM, glassesF string
	mRibbon, fRibbon   string
	mFlower, fFlower   string

	birds   []string
	bgBirds []string
	poops   []string

	bops      []bopEvt
	cues      []cueEvt
	intervals []intervalEvt
	passes    []passEvt
	times     []timeEvt
	moves     []moveEvt
	colors    []colorEvt

	cpuUp, playerUp bool
	lastPulse       int
	bgEase          bgState
	missCount       int
	misses          []missBurst

	playerLoopStop func()
}

func New() engine.Module {
	return &Module{
		cpuUp: true, playerUp: true, lastPulse: math.MinInt,
	}
}

func (m *Module) ID() string { return gameID }

func (m *Module) Load(ctx *engine.Ctx) error {
	m.ctx = ctx
	if err := ctx.LoadAssets(gameID); err != nil {
		return err
	}
	m.proj = kart.Translate(engine.ScreenW/2, engine.ScreenH/2).Mul(kart.Scale(54, -54))

	comp := ctx.Assets.Extra.Components["game"]
	roleOr := func(role, fallback string) string {
		if p := ctx.Role(role); p != "" {
			return p
		}
		if p := comp.Refs[role]; p != "" {
			return p
		}
		return fallback
	}

	m.cpuMoai = roleOr("cpuMoaiAnim", "Objects/Moais/Moaio/Moai")
	m.playerMoai = roleOr("playerMoaiAnim", "Objects/Moais/Moaiko/Moaio")
	m.cpuMove = roleOr("cpuMoaiMoveAnim", "Objects/Moais/Moaio")
	m.playerMove = roleOr("playerMoaiMoveAnim", "Objects/Moais/Moaiko")
	m.bg = roleOr("bgAnim", "BG/Background")
	m.glassesM = roleOr("GlassesM", "Objects/Moais/Moaio/Moai/moai_Head/Moai_Glasses")
	m.glassesF = roleOr("GlassesF", "Objects/Moais/Moaiko/Moaio/moai_Head/Moai_Glasses")
	// The serialized MRibbon reference points at a prefab alias that does not
	// survive flattening, but the instantiated renderer is present at this path.
	m.mRibbon = roleOr("MRibbon", "Objects/Moais/Moaio/Moai/moai_Head/moai_Ribbon")
	m.fRibbon = roleOr("FRibbon", "Objects/Moais/Moaiko/Moaio/moai_Head/moai_Ribbon")
	m.mFlower = roleOr("MFlower", "Objects/Moais/Moaio/Moai/moai_Head/moai_Flower")
	m.fFlower = roleOr("FFlower", "Objects/Moais/Moaiko/Moaio/moai_Head/moai_Flower")
	m.maleMat = comp.Refs["MoaiColorM"]
	m.femaleMat = comp.Refs["MoaiColorF"]
	if m.maleMat == "" {
		m.maleMat = "MoaiMaleMaterial"
	}
	if m.femaleMat == "" {
		m.femaleMat = "MoaiFemaleMaterial"
	}
	m.birds = append([]string(nil), comp.RefArrays["birdAnims"]...)
	m.bgBirds = append([]string(nil), comp.RefArrays["bgBirdAnims"]...)
	// The poopAnims list cannot be resolved by fileID because the clip targets
	// a nested PoopSplash prefab. FallEffect anchors are present under the same
	// bird objects, and Draw queues the extracted PoopSplash/Remain/Drip sprites
	// at those anchors using the PoopFall curves.
	m.poops = []string{
		"Objects/Moais/Moaio/Moai/moai_Head/BirdMoai/FallEffect",
		"Objects/FGBirds/Bird1/FallEffect",
		"Objects/FGBirds/Bird2/FallEffect",
	}
	m.applyVisual(defaultVisual())
	return nil
}

func (m *Module) OnEvent(e *riq.Entity) {
	switch e.Datamodel {
	case gameID + "/bop":
		m.bops = append(m.bops, bopEvt{
			beat: e.Beat, length: e.Length,
			auto: boolParam(e, "auto"), bop: boolDefault(e, "bop", true),
		})
	case gameID + "/start interval":
		m.intervals = append(m.intervals, intervalEvt{
			idx: len(m.intervals), beat: e.Beat, length: e.Length,
			autoPass: boolDefault(e, "auto", true),
			autoMoai: boolDefault(e, "autoMoai", true),
			autoBeat: e.Float("autoBeat", -1.5),
		})
	case gameID + "/pass turn":
		m.passes = append(m.passes, passEvt{beat: e.Beat})
	case gameID + "/dooWop":
		m.cues = append(m.cues, cueEvt{beat: e.Beat, length: e.Length})
	case gameID + "/shout":
		m.cues = append(m.cues, cueEvt{beat: e.Beat, length: e.Length, shout: true})
	case gameID + "/setTime":
		m.times = append(m.times, timeEvt{
			beat: e.Beat, length: e.Length,
			tod: int(e.Float("time", 1)), ease: int(e.Float("ease", 0)),
		})
	case gameID + "/moveMoai":
		m.moves = append(m.moves, moveEvt{
			beat:   e.Beat,
			cpuPos: int(e.Float("cpuPos", 0)), playerPos: int(e.Float("playerPos", 0)),
			instant: boolParam(e, "instant"),
		})
	case gameID + "/moaiColor":
		m.colors = append(m.colors, colorEvt{beat: e.Beat, v: visualFromEvent(e)})
	}
}

func (m *Module) Ready() {
	sort.Slice(m.bops, func(i, j int) bool { return m.bops[i].beat < m.bops[j].beat })
	sort.Slice(m.cues, func(i, j int) bool { return m.cues[i].beat < m.cues[j].beat })
	sort.Slice(m.intervals, func(i, j int) bool { return m.intervals[i].beat < m.intervals[j].beat })
	sort.Slice(m.passes, func(i, j int) bool { return m.passes[i].beat < m.passes[j].beat })
	sort.Slice(m.times, func(i, j int) bool { return m.times[i].beat < m.times[j].beat })
	sort.Slice(m.moves, func(i, j int) bool { return m.moves[i].beat < m.moves[j].beat })
	sort.Slice(m.colors, func(i, j int) bool { return m.colors[i].beat < m.colors[j].beat })

	for _, ev := range m.bops {
		ev := ev
		if ev.bop {
			for i := 0.0; i < ev.length; i++ {
				b := ev.beat + i
				m.ctx.At(b, func() { m.bopBirds(b) })
			}
		}
	}
	for _, ev := range m.intervals {
		m.scheduleInterval(ev)
	}
	for _, ev := range m.passes {
		if last, ok := m.lastIntervalBefore(ev.beat); ok {
			m.schedulePass(ev.beat, last.beat, last.length, last.autoMoai, last.autoBeat)
		}
	}
	for _, ev := range m.times {
		ev := ev
		m.ctx.At(ev.beat, func() { m.startTimeEase(ev) })
	}
	for _, ev := range m.moves {
		ev := ev
		m.ctx.At(ev.beat, func() { m.moveManual(ev.beat, ev.cpuPos, ev.playerPos, ev.instant) })
	}
	for _, ev := range m.colors {
		ev := ev
		m.ctx.At(ev.beat, func() { m.applyVisual(ev.v) })
	}
}

func (m *Module) OnSwitch(beat float64) {
	if m.playerLoopStop != nil {
		m.playerLoopStop()
		m.playerLoopStop = nil
	}
	sec := m.ctx.SecPerBeat(beat)
	for _, p := range []string{m.cpuMoai, m.playerMoai, m.cpuMove, m.playerMove, m.bg} {
		m.ctx.Scene.PlayDefaultState(p, beat, sec)
	}
	for i, p := range m.bgBirds {
		m.ctx.Scene.PlayState(p, "BirdBG", beat-float64(i)*0.17, 0.5)
	}
	for _, p := range m.birds {
		m.ctx.Scene.PlayDefaultState(p, beat, sec)
	}
	m.cpuUp, m.playerUp = true, true
	m.lastPulse = int(math.Floor(beat))
	m.misses = nil
	m.applyPersistentTime(beat)
	m.applyVisual(m.persistedVisual())
}

func (m *Module) Whiff(beat float64) {
	if m.ctx.ExpectingPressNow() {
		return
	}
	m.openPlayer(beat, 1)
	m.ctx.ScheduleInputRelease(beat, func(float64, engine.Judgment) {
		m.finishWhiffRelease(beat)
	}, nil)
	m.ctx.ScoreMiss()
	m.callMiss(beat)
}

func (m *Module) Update(_, beat float64) {
	if m.ctx.ReleasedNow() && !m.ctx.ExpectingReleaseNow() && m.playerLoopStop != nil {
		m.closePlayer(beat)
		m.ctx.Sound("rightWop")
	}
	pulse := int(math.Floor(beat + 1e-6))
	if pulse > m.lastPulse {
		for b := m.lastPulse + 1; b <= pulse; b++ {
			if m.autoBopAt(float64(b)) {
				m.bopBirds(float64(b))
			}
		}
		m.lastPulse = pulse
	}
}

func (m *Module) Draw(screen *ebiten.Image, _, beat float64) {
	screen.Fill(stageColor)
	m.applyBackground(beat)
	m.ctx.SampleScene(beat)
	m.queueMissBursts(beat)
	m.ctx.Scene.Draw(screen, m.proj)
}

func (m *Module) scheduleInterval(ev intervalEvt) {
	m.ctx.At(ev.beat+ev.autoBeat, func() {
		if ev.autoMoai {
			m.moveAuto(ev.beat+ev.autoBeat, true, false)
		}
	})
	for _, cue := range m.cuesBetween(ev.beat, ev.beat+ev.length) {
		cue := cue
		if cue.shout {
			m.ctx.SoundAt(cue.beat, "leftPah", 1)
			m.ctx.At(cue.beat, func() {
				m.ctx.Scene.PlayState(m.cpuMoai, "Moai_Shout", cue.beat, 0.5)
			})
			continue
		}
		m.scheduleLoop(cue.beat, cue.beat+cue.length, "leftDoo")
		m.ctx.SoundAt(cue.beat+cue.length, "leftWop", 1)
		m.ctx.At(cue.beat, func() {
			m.ctx.Scene.PlayState(m.cpuMoai, "Moai_Shout_Open", cue.beat, 0.5)
		})
		m.ctx.At(cue.beat+cue.length, func() {
			m.ctx.Scene.PlayState(m.cpuMoai, "moai_Shout_Close", cue.beat+cue.length, 0.5)
		})
	}
	if ev.autoPass {
		m.schedulePass(ev.beat+ev.length, ev.beat, ev.length, ev.autoMoai, ev.autoBeat)
	}
}

func (m *Module) schedulePass(passBeat, intervalBeat, intervalLength float64, autoMoai bool, autoBeat float64) {
	m.ctx.At(passBeat+autoBeat, func() {
		if autoMoai {
			m.moveAuto(passBeat+autoBeat, false, false)
		}
	})
	for _, cue := range m.cuesBetween(intervalBeat, intervalBeat+intervalLength) {
		cue := cue
		target := passBeat + (cue.beat - intervalBeat)
		if cue.shout {
			m.ctx.ScheduleInput(target, func(state float64, j engine.Judgment) {
				m.justUserShout(target, state, j)
			}, func() { m.callMiss(target) })
			continue
		}
		m.ctx.ScheduleInput(target, func(state float64, j engine.Judgment) {
			m.justPress(target, j)
		}, func() { m.callMiss(target) })
		m.ctx.ScheduleInputRelease(target+cue.length, func(state float64, j engine.Judgment) {
			m.justRelease(target+cue.length, j)
		}, func() { m.callMiss(target + cue.length) })
	}
}

func (m *Module) justPress(beat float64, j engine.Judgment) {
	if j == engine.JudgeNG {
		m.callMiss(beat)
	}
	m.openPlayer(beat, 1)
}

func (m *Module) justRelease(beat float64, j engine.Judgment) {
	if j == engine.JudgeNG {
		m.callMiss(beat)
	}
	m.closePlayer(beat)
	m.ctx.Sound("rightWop")
}

func (m *Module) justUserShout(target, _ float64, _ engine.Judgment) {
	m.openPlayer(target, 0.25)
	// HS schedules the shout release at the current press time, not at a fixed
	// cue-length offset. Registering it dynamically preserves tap-release timing.
	releaseBeat := m.ctx.Beat()
	m.ctx.ScheduleInputRelease(releaseBeat, func(float64, engine.Judgment) {
		if m.playerLoopStop != nil {
			m.playerLoopStop()
			m.playerLoopStop = nil
		}
		m.ctx.Scene.PlayState(m.playerMoai, "Moai_Shout", releaseBeat, 0.5)
		m.ctx.Sound("rightPah")
	}, func() { m.callMiss(releaseBeat) })
}

func (m *Module) finishWhiffRelease(beat float64) {
	if m.playerLoopStop != nil {
		m.playerLoopStop()
		m.playerLoopStop = nil
	}
	m.ctx.Scene.PlayState(m.playerMoai, "Moai_Shout", beat, 0.5)
	m.ctx.Sound("rightPah")
}

func (m *Module) openPlayer(beat, vol float64) {
	m.ctx.Scene.PlayState(m.playerMoai, "Moai_Shout_Open", beat, 0.5)
	if m.playerLoopStop == nil {
		m.playerLoopStop = m.ctx.SoundLoopVol("rightDoo", vol)
	}
}

func (m *Module) closePlayer(beat float64) {
	m.ctx.Scene.PlayState(m.playerMoai, "moai_Shout_Close", beat, 0.5)
	if m.playerLoopStop != nil {
		m.playerLoopStop()
		m.playerLoopStop = nil
	}
}

func (m *Module) callMiss(beat float64) {
	m.ctx.PlayCommon("miss")
	m.ctx.Scene.PlayState(m.cpuMoai, "Moai_Miss", beat, 0.5)
	idx := m.missCount + 1
	anchor := m.poops[m.missCount%len(m.poops)]
	m.misses = append(m.misses, missBurst{beat: beat, index: idx, anchor: anchor})
	m.missCount = (m.missCount + 1) % 5
}

func (m *Module) moveAuto(beat float64, cpu bool, instant bool) {
	if !instant {
		m.ctx.Sound("switch")
	}
	speed := 0.5
	if instant {
		speed = 500
	}
	if cpu {
		if !m.cpuUp {
			m.ctx.Scene.PlayState(m.cpuMove, "MoaioUp", beat, speed)
		}
		m.ctx.Scene.PlayState(m.playerMove, "MoaikoDown", beat, speed)
		m.cpuUp, m.playerUp = true, false
		return
	}
	m.ctx.Scene.PlayState(m.cpuMove, "MoaioDown", beat, speed)
	m.ctx.Scene.PlayState(m.playerMove, "MoaikoUp", beat, speed)
	m.cpuUp, m.playerUp = false, true
}

func (m *Module) moveManual(beat float64, cpuPos, playerPos int, instant bool) {
	speed := 0.5
	if instant {
		speed = 500
	}
	cpuMoved := false
	playerMoved := false
	if cpuPos == 0 {
		if !m.cpuUp {
			m.ctx.Scene.PlayState(m.cpuMove, "MoaioUp", beat, speed)
			m.cpuUp, cpuMoved = true, true
		}
	} else if m.cpuUp {
		m.ctx.Scene.PlayState(m.cpuMove, "MoaioDown", beat, speed)
		m.cpuUp, cpuMoved = false, true
	}
	if playerPos == 0 {
		if !m.playerUp {
			m.ctx.Scene.PlayState(m.playerMove, "MoaikoUp", beat, speed)
			m.playerUp, playerMoved = true, true
		}
	} else if m.playerUp {
		m.ctx.Scene.PlayState(m.playerMove, "MoaikoDown", beat, speed)
		m.playerUp, playerMoved = false, true
	}
	if !instant && (cpuMoved || playerMoved) {
		m.ctx.Sound("switch")
	}
}

func (m *Module) scheduleLoop(start, end float64, name string) {
	var stop func()
	m.ctx.At(start, func() { stop = m.ctx.SoundLoop(name) })
	m.ctx.At(end, func() {
		if stop != nil {
			stop()
			stop = nil
		}
	})
}

func (m *Module) bopBirds(beat float64) {
	for _, p := range m.birds {
		m.ctx.Scene.PlayState(p, "Bird_Bop", beat, 0.5)
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

func (m *Module) cuesBetween(start, end float64) []cueEvt {
	var out []cueEvt
	for _, cue := range m.cues {
		if cue.beat >= start && cue.beat < end {
			out = append(out, cue)
		}
	}
	return out
}

func (m *Module) lastIntervalBefore(beat float64) (intervalEvt, bool) {
	var out intervalEvt
	ok := false
	for _, ev := range m.intervals {
		if ev.beat > beat {
			break
		}
		out, ok = ev, true
	}
	return out, ok
}

func (m *Module) startTimeEase(ev timeEvt) {
	clip := "Background/BGNightToDay"
	if ev.tod == 1 {
		clip = "Background/BGDayToNight"
	}
	m.bgEase = bgState{
		active: true, beat: ev.beat, length: ev.length,
		clip: clip, ease: ev.ease, finalNight: ev.tod == 1,
	}
}

func (m *Module) applyBackground(beat float64) {
	if !m.bgEase.active {
		return
	}
	u := 1.0
	if m.bgEase.length > 0 {
		u = (beat - m.bgEase.beat) / m.bgEase.length
	}
	u = clamp01(u)
	v := engine.Ease(m.bgEase.ease, 0, 1, u)
	m.ctx.Scene.PlayNormalized(m.bg, m.bgEase.clip, v)
	if u >= 1 {
		m.bgEase.active = false
		if m.bgEase.finalNight {
			m.ctx.Scene.PlayState(m.bg, "BGNightHold", beat, m.ctx.SecPerBeat(beat))
		} else {
			m.ctx.Scene.PlayState(m.bg, "BGIdle", beat, m.ctx.SecPerBeat(beat))
		}
	}
}

func (m *Module) applyPersistentTime(beat float64) {
	m.bgEase = bgState{}
	var last *timeEvt
	for i := range m.times {
		if m.times[i].beat <= beat {
			last = &m.times[i]
		}
	}
	if last == nil {
		m.ctx.Scene.PlayState(m.bg, "BGIdle", beat, m.ctx.SecPerBeat(beat))
		return
	}
	if beat < last.beat+last.length {
		m.startTimeEase(*last)
		return
	}
	if last.tod == 1 {
		m.ctx.Scene.PlayState(m.bg, "BGNightHold", beat, m.ctx.SecPerBeat(beat))
	} else {
		m.ctx.Scene.PlayState(m.bg, "BGIdle", beat, m.ctx.SecPerBeat(beat))
	}
}

func (m *Module) applyVisual(v visualState) {
	m.ctx.Scene.SetPaletteFor(m.maleMat, kart.Palette{Alpha: v.mHead, Fill: v.mLens, Outline: v.mFrame})
	m.ctx.Scene.SetPaletteFor(m.femaleMat, kart.Palette{Alpha: v.fHead, Fill: v.fLens, Outline: v.fFrame})
	m.ctx.Scene.SetActive(m.glassesM, v.mGlasses)
	m.ctx.Scene.SetActive(m.glassesF, v.fGlasses)
	m.ctx.Scene.SetActive(m.mRibbon, v.mHat == 1)
	m.ctx.Scene.SetActive(m.fRibbon, v.fHat == 1)
	m.ctx.Scene.SetActive(m.mFlower, v.mHat == 2)
	m.ctx.Scene.SetActive(m.fFlower, v.fHat == 2)
}

func (m *Module) persistedVisual() visualState {
	// EntityPreCheck in the C# script applies the last moaiColor in the chart,
	// not merely the last one before the switch beat. Matching that quirk keeps
	// remixes with global appearance setup stable when entering mid-song.
	if len(m.colors) == 0 {
		return defaultVisual()
	}
	return m.colors[len(m.colors)-1].v
}

func (m *Module) queueMissBursts(beat float64) {
	active := m.misses[:0]
	for _, ms := range m.misses {
		age := (beat - ms.beat) * 0.5
		if age < 0 {
			active = append(active, ms)
			continue
		}
		if age > 1.5 {
			continue
		}
		sprite := poopSprite(ms.index, age)
		if sprite == "" {
			active = append(active, ms)
			continue
		}
		root, ok := m.ctx.Scene.NodeWorld(ms.anchor)
		if !ok {
			active = append(active, ms)
			continue
		}
		x, y := poopOffset(age)
		s := poopScale(age)
		m.ctx.Scene.Queue(kart.ExtraSprite{
			Sprite: sprite,
			World:  root.Mul(kart.Translate(x, y)).Mul(kart.Scale(s, s)),
			Order:  5,
		})
		active = append(active, ms)
	}
	m.misses = active
}

func defaultVisual() visualState {
	return visualState{
		mHead: defaultHead, mFrame: defaultGlassesFrame, mLens: defaultLens, mHat: 0,
		fHead: defaultHead, fFrame: defaultGlassesFrame, fLens: defaultLens, fHat: 1,
	}
}

func visualFromEvent(e *riq.Entity) visualState {
	def := defaultVisual()
	return visualState{
		mHead:    colorParam(e, "Mcolor", def.mHead),
		mFrame:   colorParam(e, "MglassesColor", def.mFrame),
		mLens:    colorParam(e, "Mlenscolor", def.mLens),
		mGlasses: boolParam(e, "MhasGlasses"),
		mHat:     int(e.Float("MoaiMHead", 0)),
		fHead:    colorParam(e, "Fcolor", def.fHead),
		fFrame:   colorParam(e, "FglassesColor", def.fFrame),
		fLens:    colorParam(e, "Flenscolor", def.fLens),
		fGlasses: boolParam(e, "FhasGlasses"),
		fHat:     int(e.Float("MoaiFHead", 1)),
	}
}

func poopSprite(index int, age float64) string {
	switch {
	case age < 1.0/30.0:
		return "PoopSplash" + itoaSmall(index)
	case age < 1.4333333:
		if age < 0.083333336 {
			return "PoopRemain" + itoaSmall(index)
		}
		return "PoopDrip" + itoaSmall(index)
	default:
		return "PoopRemain" + itoaSmall(index)
	}
}

func poopOffset(age float64) (float64, float64) {
	switch {
	case age < 0.033333335:
		return 0, 0
	case age < 0.083333336:
		return 0.05, -0.39
	case age < 1.4333333:
		return 0.07, -0.68
	default:
		return 0, -0.49
	}
}

func poopScale(age float64) float64 {
	switch {
	case age < 1.4333333:
		return 1
	case age < 1.4666667:
		return 0.69064
	default:
		return 0.37320802
	}
}

func itoaSmall(n int) string {
	if n < 0 || n > 9 {
		return ""
	}
	return string(rune('0' + n))
}

func boolParam(e *riq.Entity, key string) bool { return e.Float(key, 0) != 0 }

func boolDefault(e *riq.Entity, key string, def bool) bool {
	if _, ok := e.Data[key]; !ok {
		return def
	}
	return boolParam(e, key)
}

func colorParam(e *riq.Entity, key string, def [4]float64) [4]float64 {
	if mm, ok := e.Data[key].(map[string]any); ok {
		return [4]float64{num(mm["r"], def[0]), num(mm["g"], def[1]), num(mm["b"], def[2]), num(mm["a"], def[3])}
	}
	return def
}

func num(v any, def float64) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case int:
		return float64(x)
	}
	return def
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
