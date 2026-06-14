// Package octopusmachine ports Octopus Machine's repeat interval queue,
// Octo-Pop animator states, per-octopus material recolors, bubbles, text, and
// press/release timing from Assets/Scripts/Games/OctopusMachine.
package octopusmachine

import (
	"bytes"
	"image/color"
	"math"
	"math/rand"
	"sort"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"golang.org/x/image/font/gofont/goregular"

	"hsdemo/engine"
	"hsdemo/kart"
	"hsdemo/kmdata"
	"hsdemo/riq"
)

const (
	bopBop = iota
	bopHappy
	bopAngry
	bopOops

	actionSqueeze = "Squeeze"
	actionRelease = "Release"
	actionPop     = "Pop"
)

var (
	defaultBG       = [4]float64{1, 0.87, 0.24, 1}
	defaultOcto     = [4]float64{0.97, 0.235, 0.54, 1}
	defaultSqueezed = [4]float64{1, 0, 0, 1}
	octoFill        = [4]float64{0.9764706, 0.73333335, 0.101960786, 1}
	octoOutline     = [4]float64{0.84313726, 0.3254902, 0, 0.9843137}
)

type intervalEvt struct {
	beat, length float64
}

type callEvt struct {
	beat, length float64
	action       string
	prepare      bool
	prepBeats    float64
}

type bopEvt struct {
	beat               float64
	which              int
	singleBop, keepBop bool
}

type colorEvt struct {
	beat, length float64
	bg0, bg1     [4]float64
	octo         [3][4]float64
	squeezed     [3][4]float64
	ease         int
}

type textEvt struct {
	beat          float64
	text, youText string
}

type modEvt struct {
	beat   float64
	x, y   [3]float64
	active [3]bool
}

type bubbleEvt struct {
	beat            float64
	instant, active bool
	strength, speed float64
}

type colorEase struct {
	beat, length float64
	from, to     [4]float64
	ease         int
}

type octopus struct {
	path   string
	sr     []string
	srAll  []string
	player bool
	num    int
	active bool

	cantBop         bool
	isSqueezed      bool
	isPreparing     bool
	queuePrepare    float64
	lastSqueezeBeat float64
}

type bubbleParticle struct {
	x, y  float64
	r     float64
	born  float64
	life  float64
	speed float64
	front bool
}

type Module struct {
	ctx  *engine.Ctx
	proj kart.Aff
	rng  *rand.Rand

	bgPath   string
	textPath string
	octos    []*octopus

	intervals []intervalEvt
	calls     []callEvt
	bops      []bopEvt
	colors    []colorEvt
	texts     []textEvt
	mods      []modEvt
	bubbles   []bubbleEvt

	queuedSqueezes []float64
	queuedReleases []float64
	queuedPops     []float64

	bgEase       colorEase
	octoColors   [3][4]float64
	squeezeColor [3][4]float64

	queuePrepare    float64
	intervalStarted bool
	beatInterval    float64
	hasMissed       bool
	bopStatus       int
	bopIterate      int
	autoAction      bool
	keepBop         bool
	lastPulse       int

	youText  string
	mainText string
	youFace  *text.GoTextFace

	bubbleActive      bool
	bubbleStrength    float64
	bubbleSpeed       float64
	bubbleCarry       float64
	bubbleLastT       float64
	bubbleHasLastTime bool
	particles         []bubbleParticle
}

func New() engine.Module {
	return &Module{
		rng:          rand.New(rand.NewSource(0x0c70)),
		queuePrepare: math.Inf(1),
		bgEase:       colorEase{from: defaultBG, to: defaultBG},
	}
}

func (m *Module) ID() string { return "octopusMachine" }

func (m *Module) Load(ctx *engine.Ctx) error {
	m.ctx = ctx
	if err := ctx.LoadAssets("octopusMachine"); err != nil {
		return err
	}
	if err := ctx.Assets.ApplyTexts(); err != nil {
		return err
	}
	m.proj = kart.Translate(engine.ScreenW/2, engine.ScreenH/2).Mul(kart.Scale(54, -54))
	m.bgPath = roleOr(ctx, "bg", "Background")
	m.textPath = roleOr(ctx, "Text", "Text")
	m.loadOctopodes()
	src, err := text.NewGoTextFaceSource(bytes.NewReader(goregular.TTF))
	if err == nil {
		m.youFace = &text.GoTextFace{Source: src, Size: 22}
	}
	m.reset(0)
	return nil
}

func (m *Module) loadOctopodes() {
	paths := m.ctx.Assets.Extra.RefArrays["octopodes"]
	byPath := map[string]kmdata.Component{}
	for _, c := range m.ctx.Assets.Extra.Components {
		if strings.HasPrefix(c.Path, "Octopodes/Octopus") {
			byPath[c.Path] = c
		}
	}
	for i, p := range paths {
		c := byPath[p]
		o := &octopus{
			path: p, active: true, queuePrepare: math.Inf(1),
			sr:     append([]string(nil), c.RefArrays["sr"]...),
			srAll:  append([]string(nil), c.RefArrays["srAll"]...),
			player: c.Nums["player"] > 0.5,
			num:    i,
		}
		if n, ok := c.Nums["octoNum"]; ok {
			o.num = int(n)
		}
		m.octos = append(m.octos, o)
	}
}

func roleOr(ctx *engine.Ctx, key, fallback string) string {
	if p := ctx.Role(key); p != "" {
		return p
	}
	return fallback
}

func (m *Module) OnEvent(e *riq.Entity) {
	switch e.Datamodel {
	case "octopusMachine/bop":
		ev := bopEvt{
			beat: e.Beat, which: intParam(e, "whichBop", bopBop),
			singleBop: boolDefault(e, "singleBop", true),
			keepBop:   boolParam(e, "keepBop"),
		}
		m.bops = append(m.bops, ev)
		m.ctx.At(ev.beat, func() {
			if ev.keepBop {
				m.bopStatus = ev.which
			}
			m.keepBop = ev.keepBop
			if ev.singleBop {
				for _, o := range m.octos {
					m.playBop(o, ev.which, ev.beat)
				}
			}
		})
	case "octopusMachine/startInterval":
		ev := intervalEvt{beat: e.Beat, length: e.Length}
		m.intervals = append(m.intervals, ev)
		m.ctx.At(ev.beat, func() { m.startInterval(ev.beat, ev.length) })
	case "octopusMachine/squeeze", "octopusMachine/release", "octopusMachine/pop":
		ev := callEvt{beat: e.Beat, length: e.Length, action: datamodelAction(e.Datamodel)}
		ev.prepare = boolDefault(e, "shouldPrep", true)
		ev.prepBeats = e.Float("prepBeats", 1)
		m.calls = append(m.calls, ev)
		if ev.action == actionSqueeze && ev.prepare {
			m.ctx.At(ev.beat-ev.prepBeats, func() { m.queuePrepareAt(ev.beat - ev.prepBeats) })
		}
		m.ctx.At(ev.beat, func() {
			if !m.intervalStarted {
				m.startInterval(ev.beat, ev.length)
			}
		})
	case "octopusMachine/automaticActions":
		b := e.Beat
		force := boolDefault(e, "forceBop", true)
		autoBop := boolDefault(e, "autoBop", true)
		autoText := boolDefault(e, "autoText", true)
		hitText := e.Str("hitText", "Good!")
		missText := e.Str("missText", "Wrong! Try again!")
		m.ctx.At(b, func() { m.autoActions(force, autoBop, autoText, hitText, missText, b) })
	case "octopusMachine/forceSqueeze":
		b := e.Beat
		m.ctx.At(b, func() {
			for _, o := range m.octos {
				m.forceSqueeze(o, b)
			}
		})
	case "octopusMachine/prepare":
		b := e.Beat
		m.ctx.At(b, func() { m.queuePrepareAt(b) })
	case "octopusMachine/bubbles":
		ev := bubbleEvt{
			beat: e.Beat, instant: boolDefault(e, "isInstant", true),
			active:   intParam(e, "setActive", 0) == 0,
			strength: e.Float("particleStrength", 3),
			speed:    e.Float("particleSpeed", 5),
		}
		m.bubbles = append(m.bubbles, ev)
		m.ctx.At(ev.beat, func() { m.setBubbles(ev.instant, ev.active, ev.strength, ev.speed) })
	case "octopusMachine/changeText":
		ev := textEvt{beat: e.Beat, text: e.Str("text", "Do what the others do."), youText: e.Str("youText", "You")}
		m.texts = append(m.texts, ev)
		m.ctx.At(ev.beat, func() { m.changeText(ev.text, ev.youText) })
	case "octopusMachine/changeColor":
		ev := makeColorEvt(e)
		m.colors = append(m.colors, ev)
		m.ctx.At(ev.beat, func() { m.applyColorEvent(ev) })
	case "octopusMachine/octopusModifiers":
		ev := modEvt{beat: e.Beat}
		ev.x = [3]float64{e.Float("oct1x", -4.64), e.Float("oct2x", -0.637), e.Float("oct3x", 3.363)}
		ev.y = [3]float64{e.Float("oct1y", 2.5), e.Float("oct2y", 0), e.Float("oct3y", -2.5)}
		ev.active = [3]bool{boolDefault(e, "oct1", true), boolDefault(e, "oct2", true), boolDefault(e, "oct3", true)}
		m.mods = append(m.mods, ev)
		m.ctx.At(ev.beat, func() { m.applyModifiers(ev) })
	}
}

func datamodelAction(dm string) string {
	switch {
	case strings.HasSuffix(dm, "/release"):
		return actionRelease
	case strings.HasSuffix(dm, "/pop"):
		return actionPop
	default:
		return actionSqueeze
	}
}

func makeColorEvt(e *riq.Entity) colorEvt {
	ev := colorEvt{
		beat: e.Beat, length: e.Length, ease: intParam(e, "ease", 0),
		bg0: colorParam(e, "color1", defaultBG),
		bg1: colorParam(e, "color2", defaultBG),
	}
	individual := boolParam(e, "individual")
	ev.octo[0] = colorParam(e, "octoColor", defaultOcto)
	ev.squeezed[0] = colorParam(e, "squeezedColor", defaultSqueezed)
	if individual {
		ev.octo[1] = colorParam(e, "octoColor2", defaultOcto)
		ev.octo[2] = colorParam(e, "octoColor3", defaultOcto)
		ev.squeezed[1] = colorParam(e, "squeezedColor2", defaultSqueezed)
		ev.squeezed[2] = colorParam(e, "squeezedColor3", defaultSqueezed)
	} else {
		ev.octo[1], ev.octo[2] = ev.octo[0], ev.octo[0]
		ev.squeezed[1], ev.squeezed[2] = ev.squeezed[0], ev.squeezed[0]
	}
	return ev
}

func (m *Module) Ready() {
	sort.Slice(m.intervals, func(i, j int) bool { return m.intervals[i].beat < m.intervals[j].beat })
	sort.Slice(m.calls, func(i, j int) bool { return m.calls[i].beat < m.calls[j].beat })
	sort.Slice(m.bops, func(i, j int) bool { return m.bops[i].beat < m.bops[j].beat })
	sort.Slice(m.colors, func(i, j int) bool { return m.colors[i].beat < m.colors[j].beat })
	sort.Slice(m.texts, func(i, j int) bool { return m.texts[i].beat < m.texts[j].beat })
	sort.Slice(m.mods, func(i, j int) bool { return m.mods[i].beat < m.mods[j].beat })
}

func (m *Module) OnSwitch(beat float64) {
	m.reset(beat)
	for _, ev := range m.bops {
		if ev.beat > beat {
			break
		}
		m.keepBop = ev.keepBop
		if ev.keepBop {
			m.bopStatus = ev.which
		}
	}
	for _, ev := range m.colors {
		if ev.beat > beat {
			break
		}
		m.applyColorEvent(ev)
	}
	for _, ev := range m.texts {
		if ev.beat > beat {
			break
		}
		m.changeText(ev.text, ev.youText)
	}
	for _, ev := range m.mods {
		if ev.beat > beat {
			break
		}
		m.applyModifiers(ev)
	}
}

func (m *Module) Whiff(beat float64) {
	if p := m.player(); p != nil && p.active {
		m.octoAction(p, actionSqueeze, beat)
		m.ctx.PlayCommon("nearMiss")
		m.hasMissed = true
	}
}

func (m *Module) Update(t, beat float64) {
	if p := m.player(); p != nil && p.active && m.ctx.ReleasedNow() && !m.ctx.ExpectingReleaseNow() {
		m.octoAction(p, actionRelease, beat)
		m.ctx.PlayCommon("nearMiss")
		m.hasMissed = true
	}
	if m.queuePrepare <= beat {
		for _, o := range m.octos {
			o.queuePrepare = m.queuePrepare
		}
		if txt := m.currentText(); txt == "Good!" || txt == "Wrong! Try Again!" || txt == "Wrong! Try again!" {
			m.changeText("", m.youText)
		}
		m.queuePrepare = math.Inf(1)
	}
	for _, o := range m.octos {
		m.updatePrepare(o, beat)
	}
	m.beatPulse(beat)
	m.updateBubbles(t)
}

func (m *Module) Draw(screen *ebiten.Image, t float64, beat float64) {
	bg := m.bgEase.at(beat)
	screen.Fill(toRGBA(bg))
	m.ctx.Scene.SetColorOver(m.bgPath, bg)
	m.drawBubbles(screen, t, false)
	m.ctx.SampleScene(beat)
	m.ctx.Scene.Draw(screen, m.proj)
	m.drawBubbles(screen, t, true)
	m.drawYou(screen)
}

func (m *Module) reset(beat float64) {
	m.queuePrepare = math.Inf(1)
	m.intervalStarted = false
	m.beatInterval = 1
	m.hasMissed = false
	m.bopStatus = bopBop
	m.bopIterate = 0
	m.autoAction = false
	m.keepBop = false
	m.lastPulse = int(math.Floor(beat)) - 1
	m.bgEase = colorEase{from: defaultBG, to: defaultBG}
	m.octoColors = [3][4]float64{defaultOcto, defaultOcto, defaultOcto}
	m.squeezeColor = [3][4]float64{defaultSqueezed, defaultSqueezed, defaultSqueezed}
	m.queuedSqueezes = nil
	m.queuedReleases = nil
	m.queuedPops = nil
	m.particles = nil
	m.bubbleActive = false
	m.bubbleHasLastTime = false
	_ = m.ctx.Assets.SetText(m.textPath, "")
	m.mainText = ""
	m.youText = "You"
	m.ctx.Scene.SetPaletteFor("octopus", octoPalette(defaultOcto))
	for i, o := range m.octos {
		o.active = true
		o.cantBop = false
		o.isPreparing = false
		o.isSqueezed = false
		o.queuePrepare = math.Inf(1)
		if i < 3 {
			o.num = i
		}
		m.ctx.Scene.SetActive(o.path, true)
		m.ctx.Scene.PlayDefaultState(o.path, beat, m.ctx.SecPerBeat(beat))
		m.applyOctoColor(o, false)
	}
}

func (m *Module) startInterval(beat, length float64) {
	if m.intervalStarted {
		return
	}
	m.intervalStarted = true
	m.beatInterval = length
	calls := m.relevantCalls(beat, beat+length)
	for i, ev := range calls {
		if ev.action != actionSqueeze {
			if i != 0 && calls[i-1].action != actionSqueeze {
				continue
			}
		}
		if len(m.octos) > 0 {
			m.octoAction(m.octos[0], ev.action, ev.beat)
		}
		off := ev.beat - beat
		switch ev.action {
		case actionRelease:
			m.queuedReleases = append(m.queuedReleases, off)
		case actionPop:
			m.queuedPops = append(m.queuedPops, off)
		default:
			m.queuedSqueezes = append(m.queuedSqueezes, off)
		}
	}
	m.ctx.At(beat+length, func() { m.passTurn(beat + length) })
}

func (m *Module) relevantCalls(start, end float64) []callEvt {
	out := []callEvt{}
	for _, c := range m.calls {
		if c.beat >= start && c.beat < end {
			out = append(out, c)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].beat < out[j].beat })
	return out
}

func (m *Module) passTurn(beat float64) {
	m.intervalStarted = false
	if len(m.octos) < 3 {
		return
	}
	for _, off := range m.queuedSqueezes {
		target := beat + off
		m.octoAction(m.octos[1], actionSqueeze, target)
		m.ctx.ScheduleInput(beat+m.beatInterval+off,
			func(state float64, _ engine.Judgment) { m.inputHit(m.octos[2], actionSqueeze, state) },
			func() { m.inputMiss() })
	}
	for _, off := range m.queuedReleases {
		target := beat + off
		m.octoAction(m.octos[1], actionRelease, target)
		m.ctx.ScheduleInputRelease(beat+m.beatInterval+off,
			func(state float64, _ engine.Judgment) { m.inputHit(m.octos[2], actionRelease, state) },
			func() { m.inputMiss() })
	}
	for _, off := range m.queuedPops {
		target := beat + off
		m.octoAction(m.octos[1], actionPop, target)
		m.ctx.ScheduleInputRelease(beat+m.beatInterval+off,
			func(state float64, _ engine.Judgment) { m.inputHit(m.octos[2], actionPop, state) },
			func() { m.inputMiss() })
	}
	m.queuedSqueezes = nil
	m.queuedReleases = nil
	m.queuedPops = nil
}

func (m *Module) inputHit(o *octopus, action string, state float64) {
	m.octoAction(o, action, m.ctx.App.BeatNow())
	if nearMiss(state) {
		m.ctx.PlayCommon("nearMiss")
	}
}

func (m *Module) inputMiss() { m.hasMissed = true }

func (m *Module) player() *octopus {
	for _, o := range m.octos {
		if o.player {
			return o
		}
	}
	if len(m.octos) >= 3 {
		return m.octos[2]
	}
	return nil
}

func (m *Module) queuePrepareAt(beat float64) { m.queuePrepare = beat }

func (m *Module) updatePrepare(o *octopus, beat float64) {
	if o.queuePrepare > beat {
		return
	}
	state, playing := m.ctx.Scene.StateInfo(o.path, beat)
	if !(o.isPreparing || o.isSqueezed || (playing && (state == actionRelease || state == actionPop))) {
		m.ctx.Scene.PlayState(o.path, "Prepare", o.queuePrepare, 0.5)
		o.isPreparing = true
		m.applyOctoColor(o, false)
	}
	o.queuePrepare = math.Inf(1)
}

func (m *Module) beatPulse(beat float64) {
	pulse := int(math.Floor(beat))
	if pulse <= m.lastPulse {
		return
	}
	for b := m.lastPulse + 1; b <= pulse; b++ {
		if m.bopIterate >= 3 {
			m.bopStatus = bopBop
			m.bopIterate = 0
			m.autoAction = false
		}
		if m.autoAction {
			m.bopIterate++
		}
		for _, o := range m.octos {
			o.cantBop = !m.keepBop
			m.requestBop(o, float64(b))
		}
	}
	m.lastPulse = pulse
}

func (m *Module) requestBop(o *octopus, beat float64) {
	state, playing := m.ctx.Scene.StateInfo(o.path, beat)
	if playing && (state == "Bop" || state == "Happy" || state == "Angry" || state == "Oops" || state == actionRelease || state == actionPop) {
		return
	}
	if !o.isPreparing && !o.isSqueezed && !o.cantBop {
		m.playBop(o, m.bopStatus, beat)
	}
}

func (m *Module) playBop(o *octopus, which int, beat float64) {
	if which == bopAngry && o.player {
		which = bopOops
	}
	state := "Bop"
	switch which {
	case bopHappy:
		state = "Happy"
	case bopAngry:
		state = "Angry"
	case bopOops:
		state = "Oops"
	}
	m.ctx.Scene.PlayState(o.path, state, beat, 0.5)
	o.isPreparing = false
	o.isSqueezed = false
	m.applyOctoColor(o, false)
}

func (m *Module) forceSqueeze(o *octopus, beat float64) {
	m.ctx.Scene.PlayState(o.path, "ForceSqueeze", beat, 0.5)
	o.isSqueezed = true
	o.isPreparing = false
	m.applyOctoColor(o, true)
}

func (m *Module) octoAction(o *octopus, action string, beat float64) {
	m.ctx.At(beat, func() {
		if action != actionRelease || beat-o.lastSqueezeBeat > 0.15 {
			m.ctx.Sound(strings.ToLower(action))
		}
		if action == actionSqueeze {
			o.lastSqueezeBeat = beat
		}
		m.ctx.Scene.PlayState(o.path, action, beat, 0.5)
		o.isSqueezed = action == actionSqueeze
		o.isPreparing = false
		o.queuePrepare = math.Inf(1)
		m.applyAnimationColorEvents(o, action, beat)
	})
}

func (m *Module) applyAnimationColorEvents(o *octopus, action string, beat float64) {
	switch action {
	case actionSqueeze:
		m.applyOctoColor(o, false)
		m.ctx.At(beat+0.1, func() { m.applyOctoColor(o, true) })
	case actionRelease:
		m.ctx.At(beat+0.033333333, func() { m.applyOctoColor(o, false) })
	case "ForceSqueeze":
		m.applyOctoColor(o, true)
	default:
		m.applyOctoColor(o, false)
	}
}

func (m *Module) autoActions(force, autoBop, autoText bool, hitText, missText string, beat float64) {
	m.autoAction = true
	if autoBop {
		if m.hasMissed {
			m.bopStatus = bopAngry
		} else {
			m.bopStatus = bopHappy
		}
	}
	if autoText {
		if m.hasMissed {
			m.changeText(missText, m.youText)
		} else {
			m.changeText(hitText, m.youText)
		}
	}
	for _, o := range m.octos {
		if force {
			m.playBop(o, m.bopStatus, beat)
		}
		o.cantBop = false
	}
	m.hasMissed = false
}

func (m *Module) changeText(s, you string) {
	_ = m.ctx.Assets.SetText(m.textPath, s)
	m.mainText = s
	m.youText = you
}

func (m *Module) currentText() string { return m.mainText }

func (m *Module) applyColorEvent(ev colorEvt) {
	m.bgEase = colorEase{beat: ev.beat, length: ev.length, from: ev.bg0, to: ev.bg1, ease: ev.ease}
	m.octoColors = ev.octo
	m.squeezeColor = ev.squeezed
	for _, o := range m.octos {
		m.applyOctoColor(o, o.isSqueezed)
	}
}

func (m *Module) applyOctoColor(o *octopus, squeezed bool) {
	if o == nil {
		return
	}
	o.isSqueezed = squeezed
	c := defaultOcto
	if o.num >= 0 && o.num < len(m.octoColors) {
		if squeezed {
			c = m.squeezeColor[o.num]
		} else {
			c = m.octoColors[o.num]
		}
	}
	p := octoPalette(c)
	for _, path := range o.sr {
		m.ctx.Scene.SetPaletteOver(path, p)
	}
}

func octoPalette(alpha [4]float64) kart.Palette {
	return kart.Palette{Alpha: alpha, Fill: octoFill, Outline: octoOutline}
}

func (m *Module) applyModifiers(ev modEvt) {
	for i, o := range m.octos {
		if i >= 3 {
			break
		}
		o.active = ev.active[i]
		m.ctx.Scene.SetPosOver(o.path, ev.x[i], ev.y[i])
		// The Unity script fades every renderer but leaves animation state alive.
		// SetActive is visually equivalent here because hidden Octo-Pops also
		// stop accepting player whiffs in Octopus.Update.
		m.ctx.Scene.SetActive(o.path, ev.active[i])
	}
}

func (m *Module) setBubbles(instant, active bool, strength, speed float64) {
	m.bubbleActive = active
	m.bubbleStrength = strength
	m.bubbleSpeed = speed
	if !active && instant {
		m.particles = nil
	}
	if active && instant {
		for i := 0; i < int(strength*5); i++ {
			p := m.spawnBubble(m.bubbleLastT)
			p.born -= m.rng.Float64() * p.life
			m.particles = append(m.particles, p)
		}
	}
}

func (m *Module) updateBubbles(t float64) {
	if !m.bubbleHasLastTime {
		m.bubbleLastT = t
		m.bubbleHasLastTime = true
		return
	}
	dt := t - m.bubbleLastT
	m.bubbleLastT = t
	if dt <= 0 || dt > 1 {
		return
	}
	if m.bubbleActive {
		m.bubbleCarry += dt * m.bubbleStrength * 2
		for m.bubbleCarry >= 1 {
			m.particles = append(m.particles, m.spawnBubble(t))
			m.bubbleCarry--
		}
	}
	alive := m.particles[:0]
	for _, p := range m.particles {
		if t-p.born < p.life {
			alive = append(alive, p)
		}
	}
	m.particles = alive
}

func (m *Module) spawnBubble(t float64) bubbleParticle {
	return bubbleParticle{
		x:     -8 + m.rng.Float64()*16,
		y:     -5.6 + m.rng.Float64()*1.2,
		r:     3 + m.rng.Float64()*9,
		born:  t,
		life:  4 + m.rng.Float64()*4,
		speed: 0.45 + m.rng.Float64()*0.7,
		front: m.rng.Intn(2) == 0,
	}
}

func (m *Module) drawBubbles(dst *ebiten.Image, t float64, front bool) {
	for _, p := range m.particles {
		if p.front != front {
			continue
		}
		u := clamp01((t - p.born) / p.life)
		x := p.x + math.Sin(u*math.Pi*4+p.r)*0.25
		y := p.y + u*p.speed*m.bubbleSpeed
		sx, sy := m.proj.Apply(x, y)
		alpha := uint8((1 - u) * 95)
		if alpha == 0 {
			continue
		}
		vector.StrokeCircle(dst, float32(sx), float32(sy), float32(p.r), 1.4,
			color.RGBA{230, 255, 255, alpha}, true)
	}
}

func (m *Module) drawYou(dst *ebiten.Image) {
	if m.youText == "" {
		return
	}
	m.ctx.Assets.DrawSpriteOpts(dst, "octopus_new_31", kart.Translate(6.06, -3.58), m.proj, kart.SpriteOpts{})
	if m.youFace == nil {
		return
	}
	x, y := m.proj.Apply(7.77, -3.62)
	w, _ := text.Measure(m.youText, m.youFace, 0)
	op := &text.DrawOptions{}
	op.GeoM.Translate(x-w/2, y-14)
	op.ColorScale.ScaleWithColor(color.Black)
	text.Draw(dst, m.youText, m.youFace, op)
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

func boolParam(e *riq.Entity, key string) bool { return e.Float(key, 0) != 0 }

func boolDefault(e *riq.Entity, key string, def bool) bool {
	if _, ok := e.Data[key]; !ok {
		return def
	}
	return boolParam(e, key)
}

func intParam(e *riq.Entity, key string, def int) int {
	if _, ok := e.Data[key]; !ok {
		return def
	}
	return int(e.Float(key, float64(def)))
}

func colorParam(e *riq.Entity, key string, def [4]float64) [4]float64 {
	v, ok := e.Data[key]
	if !ok {
		return def
	}
	m, ok := v.(map[string]any)
	if !ok {
		return def
	}
	c := def
	for i, k := range []string{"r", "g", "b", "a"} {
		if f, ok := m[k].(float64); ok {
			c[i] = f
		}
	}
	return c
}

func toRGBA(c [4]float64) color.RGBA {
	return color.RGBA{uint8(clamp01(c[0]) * 255), uint8(clamp01(c[1]) * 255), uint8(clamp01(c[2]) * 255), uint8(clamp01(c[3]) * 255)}
}

func nearMiss(state float64) bool { return state <= -1 || state >= 1 }

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
