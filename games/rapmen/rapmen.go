// Package rapmen ports RAPMEN's rapper animation states, call/response input
// timing, subtitles, gender toggles, background palette events, and cue sounds.
package rapmen

import (
	"math"
	"sort"

	"github.com/hajimehoshi/ebiten/v2"

	"hsdemo/engine"
	"hsdemo/kart"
	"hsdemo/riq"
)

type Module struct {
	ctx  *engine.Ctx
	proj kart.Aff

	red, yellow  string
	cherry, blue string
	text         string
	background   string
	bgMat        string
	speakerMat   string
	particles    []string
	uhnParticle  string

	bops    []bopEvt
	raps    []rapEvt
	banters []banterEvt
	texts   []textEvt
	bgs     []bgEvt
	toggles []toggleEvt

	redBop, yellowBop       bool
	redCanBop, yellowCanBop bool
	redWoman, yellowWoman   bool
	subtitleLanguage        int
	lastPulse               int

	cymbalPitch, cymbalVolume float64
	drumPitch, drumVolume     float64

	bg bgEvt
}

func New() engine.Module {
	return &Module{
		redBop: true, yellowBop: true, redCanBop: true, yellowCanBop: true,
		subtitleLanguage: 1, lastPulse: -1,
		cymbalPitch: 0.98, cymbalVolume: 0.35, drumPitch: 1.05, drumVolume: 0.85,
		bg: defaultBG(),
	}
}

func (m *Module) ID() string { return gameID }

func (m *Module) Load(ctx *engine.Ctx) error {
	m.ctx = ctx
	if err := ctx.LoadAssets(gameID); err != nil {
		return err
	}
	if err := ctx.Assets.ApplyTexts(); err != nil {
		return err
	}
	m.proj = kart.Translate(engine.ScreenW/2, engine.ScreenH/2).Mul(kart.Scale(54, -54))
	game := ctx.Assets.Extra.Components["game"]
	m.red = roleOr(ctx, "rapperRed", game.Refs["rapperRed"])
	m.yellow = roleOr(ctx, "rapperYellow", game.Refs["rapperYellow"])
	m.cherry = roleOr(ctx, "rapperCherry", game.Refs["rapperCherry"])
	m.blue = roleOr(ctx, "rapperBlue", game.Refs["rapperBlue"])
	m.text = roleOr(ctx, "rapText", game.Refs["rapText"])
	m.background = roleOr(ctx, "background", game.Refs["background"])
	m.bgMat = game.Refs["backgroundMaterial"]
	m.speakerMat = game.Refs["speakerMaterial"]
	m.uhnParticle = roleOr(ctx, "uhnParticle", game.Refs["uhnParticle"])
	m.particles = append([]string(nil), game.RefArrays["justParticles"]...)
	if len(m.particles) == 0 {
		m.particles = append([]string(nil), ctx.Assets.Extra.RefArrays["justParticles"]...)
	}
	m.cymbalPitch = nonzero(game.Nums["cymbalPitch"], m.cymbalPitch)
	m.cymbalVolume = nonzero(game.Nums["cymbalVolume"], m.cymbalVolume)
	m.drumPitch = nonzero(game.Nums["drumPitch"], m.drumPitch)
	m.drumVolume = nonzero(game.Nums["drumVolume"], m.drumVolume)
	m.initScene(0)
	return nil
}

func (m *Module) OnEvent(e *riq.Entity) {
	switch actionName(e) {
	case "bop":
		m.bops = append(m.bops, bopEvt{
			beat: e.Beat, length: e.Length,
			red: boolDefault(e, "r", true), yellow: boolDefault(e, "y", true),
			redAuto: boolParam(e, "ra"), yellowAuto: boolParam(e, "ya"),
		})
	case "desuka", "kamone", "saiko":
		m.raps = append(m.raps, rapEvt{
			beat: e.Beat, cue: actionName(e), gender: intDefault(e, "gender", 0),
			voice: intDefault(e, "voice", 1), womanVoice: intDefault(e, "voice2", 1),
			caption: intDefault(e, "caption", 0), text: stringDefault(e, "text", ""),
			mute: boolParam(e, "mute"),
		})
	case "banter":
		m.banters = append(m.banters, banterEvt{beat: e.Beat, gender: intDefault(e, "gender", 0), voice: intDefault(e, "voice", 1), playAnim: boolDefault(e, "anim", true)})
	case "setText":
		m.texts = append(m.texts, textEvt{beat: e.Beat, length: e.Length, text: stringDefault(e, "text", ""), color: intDefault(e, "color", 0)})
	case "background color":
		m.bgs = append(m.bgs, bgEvt{
			beat: e.Beat, length: e.Length, typ: intDefault(e, "bg", 1), ease: intDefault(e, "ease", 0),
			a0: colorParam(e, "colorFrom", defaultA), a1: colorParam(e, "colorTo", defaultA),
			b0: colorParam(e, "colorFrom2", defaultB), b1: colorParam(e, "colorTo2", defaultB),
			c0: colorParam(e, "colorFrom3", defaultC), c1: colorParam(e, "colorTo3", defaultC),
			d0: colorParam(e, "colorFrom4", defaultD), d1: colorParam(e, "colorTo4", defaultD),
		})
	case "language":
		beat := e.Beat
		lang := intDefault(e, "voice", 1)
		m.ctx.At(beat, func() { m.subtitleLanguage = lang })
	case "womenToggle":
		m.toggles = append(m.toggles, toggleEvt{beat: e.Beat, red: intDefault(e, "redsGender", 0), yellow: intDefault(e, "yellowsGender", 0)})
	}
}

func (m *Module) Ready() {
	m.sortEvents()
	for _, ev := range m.bops {
		ev := ev
		m.ctx.At(ev.beat, func() { m.redBop, m.yellowBop = ev.redAuto, ev.yellowAuto })
		for i := 0; i < int(ev.length); i++ {
			b := ev.beat + float64(i)
			m.ctx.At(b, func() {
				if ev.red {
					m.rapperAnim("Red", "bop", b)
				}
				if ev.yellow {
					m.rapperAnim("Player", "bop", b)
				}
			})
		}
	}
	for _, ev := range m.raps {
		ev := ev
		m.scheduleRap(ev)
	}
	for _, ev := range m.banters {
		ev := ev
		m.ctx.At(ev.beat, func() { m.redBanter(ev) })
	}
	for _, ev := range m.texts {
		ev := ev
		m.ctx.At(ev.beat, func() { m.setText(ev.text, ev.color) })
		m.ctx.At(ev.beat+ev.length, func() { m.clearText() })
	}
	for _, ev := range m.bgs {
		ev := ev
		m.ctx.At(ev.beat, func() { m.applyBackgroundEvent(ev) })
	}
	for _, ev := range m.toggles {
		ev := ev
		m.ctx.At(ev.beat, func() { m.applyToggle(ev.red, ev.yellow) })
	}
}

func (m *Module) OnSwitch(beat float64) {
	m.lastPulse = int(math.Floor(beat)) - 1
	m.initScene(beat)
	for _, ev := range m.bgs {
		if ev.beat < beat {
			m.applyBackgroundEvent(ev)
		}
	}
	for _, ev := range m.toggles {
		if ev.beat < beat {
			m.applyToggle(ev.red, ev.yellow)
		}
	}
	m.applyBackground(beat)
}

func (m *Module) Whiff(beat float64) {
	m.ctx.Sound("whiff")
	m.rapperAnim("Player", "bop", beat)
}

func (m *Module) Update(_, beat float64) {
	for pulse := m.lastPulse + 1; pulse <= int(math.Floor(beat)); pulse++ {
		if pulse >= 0 {
			m.lateBeatPulse(float64(pulse))
		}
		m.lastPulse = pulse
	}
	m.applyBackground(beat)
	m.hideExpiredParticles(beat)
}

func (m *Module) Draw(screen *ebiten.Image, _, beat float64) {
	screen.Fill(rgba(defaultB))
	m.ctx.SampleScene(beat)
	m.ctx.Scene.Draw(screen, m.proj)
}

func (m *Module) initScene(beat float64) {
	m.redBop, m.yellowBop = true, true
	m.redCanBop, m.yellowCanBop = true, true
	_ = m.ctx.Assets.SetText(m.text, "")
	for _, p := range m.particles {
		m.ctx.Scene.SetActive(p, false)
	}
	m.ctx.Scene.SetActive(m.uhnParticle, false)
	m.applyToggle(0, 0)
	for _, p := range []string{m.red, m.yellow, m.cherry, m.blue, m.background} {
		m.ctx.Scene.PlayDefaultState(p, beat, m.ctx.SecPerBeat(beat))
	}
	m.bg = defaultBG()
	m.applyBackgroundEvent(m.bg)
}

func (m *Module) sortEvents() {
	sort.SliceStable(m.bops, func(i, j int) bool { return m.bops[i].beat < m.bops[j].beat })
	sort.SliceStable(m.raps, func(i, j int) bool { return m.raps[i].beat < m.raps[j].beat })
	sort.SliceStable(m.banters, func(i, j int) bool { return m.banters[i].beat < m.banters[j].beat })
	sort.SliceStable(m.texts, func(i, j int) bool { return m.texts[i].beat < m.texts[j].beat })
	sort.SliceStable(m.bgs, func(i, j int) bool { return m.bgs[i].beat < m.bgs[j].beat })
	sort.SliceStable(m.toggles, func(i, j int) bool { return m.toggles[i].beat < m.toggles[j].beat })
}
