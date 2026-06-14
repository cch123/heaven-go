// Package rhythmtestgba ports Rhythm Test GBA's beeping screen, countdown, and
// keep-the-beat button test from Heaven Studio.
package rhythmtestgba

import (
	"image/color"
	"math"
	"sort"

	"github.com/hajimehoshi/ebiten/v2"

	"hsdemo/engine"
	"hsdemo/kart"
	"hsdemo/riq"
)

const (
	pulseNote = iota
	pulseText
)

type interval struct {
	start, end float64
}

func (in interval) contains(v float64) bool { return v >= in.start && v < in.end }

type countinEvt struct {
	beat, length float64
	toggle, auto bool
	image        int
	textFlash    bool
	text         string
	hasSound     bool
}

type stopEvt struct {
	beat, length float64
	mute, finish bool
	text         string
}

type countdownEvt struct {
	beat, length float64
	count        int
}

type hideEvt struct {
	beat float64
	show bool
}

type Module struct {
	ctx  *engine.Ctx
	proj kart.Aff

	note, text      string
	button          string
	flash           string
	numberBG        string
	number          string
	textAnimator    string
	lastPulse       float64
	endBeat         float64
	canBeep         bool
	keepPressing    bool
	disableCount    bool
	beepHasSound    bool
	screenFXType    int
	beatTextFlash   bool
	beepType        int
	countins        []countinEvt
	stops           []stopEvt
	countdowns      []countdownEvt
	hides           []hideEvt
	startKeepBeats  []float64
	noBopIntervals  []interval
	noBeepIntervals []interval
}

func New() engine.Module { return &Module{lastPulse: math.Inf(-1)} }

func (m *Module) ID() string { return "rhythmTestGBA" }

func (m *Module) Load(ctx *engine.Ctx) error {
	m.ctx = ctx
	if err := ctx.LoadAssets("rhythmTestGBA"); err != nil {
		return err
	}
	if err := ctx.Assets.ApplyTexts(); err != nil {
		return err
	}
	m.proj = kart.Translate(engine.ScreenW/2, engine.ScreenH/2).Mul(kart.Scale(54, -54))

	m.note = roleOr(ctx, "noteFlash", "Note")
	m.text = roleOr(ctx, "screenText", "Text")
	m.button = roleOr(ctx, "buttonAnimator", "Button")
	m.flash = roleOr(ctx, "flashAnimator", "Note")
	m.numberBG = roleOr(ctx, "numberBGAnimator", "Countdown/BG")
	m.number = roleOr(ctx, "numberAnimator", "Countdown/Number")
	m.textAnimator = roleOr(ctx, "textAnimator", "Text")
	_ = ctx.Assets.SetText(m.text, "")
	ctx.Scene.SetActive(m.note, false)
	return nil
}

func (m *Module) OnEvent(e *riq.Entity) {
	b := e.Beat
	if end := b + e.Length; end > m.endBeat {
		m.endBeat = end
	}
	switch e.Datamodel {
	case "rhythmTestGBA/countin":
		ev := countinEvt{
			beat: b, length: e.Length,
			toggle:    boolParamDefault(e, "toggle", true),
			auto:      boolParam(e, "auto"),
			image:     int(e.Float("image", 0)),
			textFlash: boolParamDefault(e, "textFlash", true),
			text:      e.Str("textDisplay", "Get ready..."),
			hasSound:  boolParamDefault(e, "hasSound", true),
		}
		m.countins = append(m.countins, ev)
		m.noBopIntervals = append(m.noBopIntervals, interval{b, b + 1})
		m.noBeepIntervals = append(m.noBeepIntervals, interval{b, b + e.Length})
	case "rhythmTestGBA/button":
		m.startKeepBeats = append(m.startKeepBeats, b)
	case "rhythmTestGBA/stopktb":
		ev := stopEvt{
			beat: b, length: e.Length,
			mute:   boolParam(e, "mutecue"),
			finish: boolParamDefault(e, "finishText", true),
			text:   e.Str("textDisplay", "Test complete!"),
		}
		m.stops = append(m.stops, ev)
		m.noBeepIntervals = append(m.noBeepIntervals, interval{b, b + e.Length})
	case "rhythmTestGBA/countdown":
		count := int(e.Float("val1", 3))
		if count < 1 {
			count = 1
		} else if count > 9 {
			count = 9
		}
		m.countdowns = append(m.countdowns, countdownEvt{beat: b, length: e.Length, count: count})
	case "rhythmTestGBA/hidecount":
		m.hides = append(m.hides, hideEvt{beat: b, show: boolParamDefault(e, "togglecount", true)})
	}
}

func (m *Module) Ready() {
	sort.Slice(m.countins, func(i, j int) bool { return m.countins[i].beat < m.countins[j].beat })
	sort.Slice(m.stops, func(i, j int) bool { return m.stops[i].beat < m.stops[j].beat })
	sort.Slice(m.countdowns, func(i, j int) bool { return m.countdowns[i].beat < m.countdowns[j].beat })
	sort.Slice(m.hides, func(i, j int) bool { return m.hides[i].beat < m.hides[j].beat })
	sort.Float64s(m.startKeepBeats)

	for _, ev := range m.countins {
		ev := ev
		m.ctx.At(ev.beat, func() { m.ktbPrep(ev) })
		if ev.toggle {
			for b := ev.beat; b < ev.beat+ev.length-1e-6; b++ {
				bb := b
				m.ctx.At(bb, func() { m.playFlashFX(bb) })
			}
		}
	}
	for _, ev := range m.hides {
		ev := ev
		m.ctx.At(ev.beat, func() { m.disableCount = !ev.show })
	}
	for _, b := range m.startKeepBeats {
		m.scheduleKeepBeat(b)
	}
	for _, ev := range m.stops {
		ev := ev
		m.ctx.At(ev.beat, func() { m.stopKeepbeat(ev) })
	}
	for _, ev := range m.countdowns {
		ev := ev
		m.ctx.At(ev.beat, func() { m.preCountdown(ev) })
	}
}

func (m *Module) OnSwitch(beat float64) {
	sec := m.ctx.SecPerBeat(beat)
	for _, p := range []string{m.button, m.flash, m.numberBG, m.number, m.textAnimator} {
		m.ctx.Scene.PlayDefaultState(p, beat, sec)
	}
	m.lastPulse = math.Floor(beat)
}

func (m *Module) Whiff(beat float64) { m.pressButton(beat) }

func (m *Module) Update(_, beat float64) {
	if p := math.Floor(beat); p > m.lastPulse {
		m.lastPulse = p
		if m.canBeep && m.autoBeepAt(p) {
			m.playFlashFX(p)
		}
	}
}

func (m *Module) Draw(screen *ebiten.Image, _, beat float64) {
	screen.Fill(color.RGBA{0x2d, 0xd8, 0x16, 0xff})
	m.ctx.SampleScene(beat)
	m.ctx.Scene.Draw(screen, m.proj)
}

func (m *Module) ktbPrep(ev countinEvt) {
	m.canBeep = true
	if ev.image == pulseText {
		_ = m.ctx.Assets.SetText(m.text, ev.text)
	}
	m.beepType = 0
	m.screenFXType = ev.image
	m.beepHasSound = ev.hasSound
	m.beatTextFlash = ev.textFlash
	if m.beepHasSound {
		m.playBeep()
	}
	if ev.toggle {
		m.playFakeFlashFX(ev.image, ev.textFlash, ev.beat)
	}
}

func (m *Module) playFlashFX(beat float64) {
	m.ctx.Scene.PlayState(m.number, "Idle", beat, m.ctx.SecPerBeat(beat))
	m.ctx.Scene.PlayState(m.numberBG, "Idle", beat, m.ctx.SecPerBeat(beat))
	if !m.inAny(m.noBopIntervals, beat) && m.beepHasSound && !m.inAny(m.noBeepIntervals, beat) {
		m.playBeep()
	}
	m.playScreenFX(m.screenFXType, m.beatTextFlash, beat)
	m.ctx.At(beat+0.9, func() { m.ctx.Scene.SetActive(m.note, false) })
}

func (m *Module) playFakeFlashFX(kind int, textFlash bool, beat float64) {
	m.ctx.Scene.PlayState(m.number, "Idle", beat, m.ctx.SecPerBeat(beat))
	m.ctx.Scene.PlayState(m.numberBG, "Idle", beat, m.ctx.SecPerBeat(beat))
	m.playScreenFX(kind, textFlash, beat)
	m.ctx.At(beat+0.9, func() { m.ctx.Scene.SetActive(m.note, false) })
}

func (m *Module) playScreenFX(kind int, textFlash bool, beat float64) {
	switch {
	case kind == pulseNote:
		m.ctx.Scene.SetActive(m.note, true)
		m.ctx.Scene.PlayState(m.flash, "KTBPulse", beat, 0.5)
		_ = m.ctx.Assets.SetText(m.text, "")
	case textFlash:
		m.ctx.Scene.SetActive(m.note, false)
		m.ctx.Scene.PlayState(m.textAnimator, "TextFlash", beat, 0.5)
	default:
		m.ctx.Scene.SetActive(m.note, false)
		m.ctx.Scene.PlayState(m.textAnimator, "TextIdle", beat, 0.5)
	}
}

func (m *Module) pressButton(beat float64) {
	m.ctx.Sound("press")
	m.ctx.Scene.PlayState(m.button, "Press", beat, 0.5)
}

func (m *Module) stopKeepbeat(ev stopEvt) {
	m.keepPressing = false
	m.canBeep = false
	for i := 1; i <= 3; i++ {
		target := ev.beat + float64(i)
		m.ctx.ScheduleInput(target,
			func(_ float64, _ engine.Judgment) { m.pressButton(m.ctx.Beat()) },
			func() {})
	}
	for i := 0; i < 3; i++ {
		b := ev.beat + float64(i)
		m.ctx.At(b, func() { m.playFlashFX(b) })
		if !ev.mute {
			m.ctx.SoundAt(b, "blip2", 1)
		}
	}
	m.ctx.At(ev.beat+3, func() {
		m.ctx.Sound("end_ding")
		if ev.finish {
			_ = m.ctx.Assets.SetText(m.text, ev.text)
			m.ctx.Scene.PlayState(m.textAnimator, "TextIdle", ev.beat+3, 0.5)
		}
	})
}

func (m *Module) preCountdown(ev countdownEvt) {
	if m.keepPressing {
		return
	}
	target := ev.beat + ev.length*float64(ev.count)
	m.ctx.ScheduleInput(target,
		func(_ float64, _ engine.Judgment) { m.pressButton(m.ctx.Beat()) },
		func() {})
	for i := 0; i < ev.count; i++ {
		n := ev.count - i
		b := ev.beat + ev.length*float64(i)
		m.ctx.At(b, func() { m.flashNumber(n, b) })
	}
	m.ctx.At(target, func() { m.flashZero(target) })
}

func (m *Module) flashNumber(n int, beat float64) {
	if m.disableCount {
		m.ctx.Scene.PlayState(m.numberBG, "Idle", beat, m.ctx.SecPerBeat(beat))
		m.ctx.Scene.PlayState(m.number, "Idle", beat, m.ctx.SecPerBeat(beat))
		return
	}
	m.ctx.Scene.PlayState(m.numberBG, "FlashBG", beat, 0.5)
	m.ctx.Scene.PlayState(m.number, numberState(n), beat, 0.5)
	m.ctx.Sound("blip2")
}

func (m *Module) flashZero(beat float64) {
	m.ctx.Scene.PlayState(m.numberBG, "FlashHit", beat, 0.5)
	m.ctx.Scene.PlayState(m.number, "Zero", beat, 0.5)
	m.ctx.Sound("blip3")
}

func (m *Module) scheduleKeepBeat(start float64) {
	m.ctx.At(start, func() { m.keepPressing = true })
	end := m.keepEnd(start)
	for b := start; b <= end+1e-6; b++ {
		target := b
		m.ctx.ScheduleInput(target,
			func(_ float64, _ engine.Judgment) { m.pressButton(m.ctx.Beat()) },
			func() {})
	}
}

func (m *Module) keepEnd(start float64) float64 {
	end := m.ctx.NextSwitchBeat(start)
	if math.IsInf(end, 1) {
		end = m.endBeat + 4
	}
	for _, ev := range m.stops {
		if ev.beat >= start {
			return math.Min(end, ev.beat)
		}
	}
	return end
}

func (m *Module) autoBeepAt(beat float64) bool {
	if len(m.countins) == 0 {
		return true
	}
	on := false
	for _, ev := range m.countins {
		if ev.beat > beat {
			break
		}
		on = ev.auto
	}
	return on
}

func (m *Module) playBeep() {
	switch m.beepType {
	case 1:
		m.ctx.Sound("blip2")
	case 2:
		m.ctx.Sound("end_ding")
	default:
		m.ctx.Sound("blip")
	}
}

func (m *Module) inAny(xs []interval, beat float64) bool {
	for _, x := range xs {
		if x.contains(beat) {
			return true
		}
	}
	return false
}

func numberState(n int) string {
	switch n {
	case 9:
		return "Nine"
	case 8:
		return "Eight"
	case 7:
		return "Seven"
	case 6:
		return "Six"
	case 5:
		return "Five"
	case 4:
		return "Four"
	case 3:
		return "Three"
	case 2:
		return "Two"
	default:
		return "One"
	}
}

func roleOr(ctx *engine.Ctx, key, fallback string) string {
	if p := ctx.Role(key); p != "" {
		return p
	}
	return fallback
}

func boolParam(e *riq.Entity, key string) bool { return e.Float(key, 0) != 0 }

func boolParamDefault(e *riq.Entity, key string, def bool) bool {
	if _, ok := e.Data[key]; !ok {
		return def
	}
	return boolParam(e, key)
}
