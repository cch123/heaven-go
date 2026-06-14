// Package loverap ports Love Rap's call-and-response cue timing, rapper/MC
// animator layers, speech bubbles, voice MultiSound offsets, and background
// color/wetness events from Assets/Scripts/Games/LoveRap/LoveRap.cs.
package loverap

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
	cueIntoYou = iota + 1
	cueCrazy
	cueFoSho
	cueAllAboutYou
)

const (
	inputIntoYou = iota + 1
	inputCrazy
	inputCrazyOffbeat
	inputFoSho
	inputFoShoOffbeat
	inputAllAboutYou
)

var (
	black = [4]float64{0, 0, 0, 1}
	wet0  = [4]float64{1, 1, 1, 0}
)

type bgEvt struct {
	beat, length float64
	c0, c1       [4]float64
	w0, w1       [4]float64
	ease         int
}

type bopEvt struct {
	beat, length float64
	bop, auto    bool
}

type cueEvt struct {
	beat, length float64
	kind, input  int
	heart        bool
	text         string
}

type soundEvt struct {
	at     float64
	name   string
	offset float64
}

type Module struct {
	ctx  *engine.Ctx
	proj kart.Aff

	bops []bopEvt
	bgs  []bgEvt
	cues []cueEvt
	bg   bgEvt

	inputType int
	heart     bool
	lastAnim  string
	lastBeat  float64
	lastPulse int
}

func New() engine.Module {
	return &Module{
		bg:        bgEvt{c0: black, c1: black, w0: wet0, w1: wet0},
		lastBeat:  math.Inf(-1),
		lastPulse: -1 << 30,
	}
}

func (m *Module) ID() string { return "loveRap" }

func (m *Module) Load(ctx *engine.Ctx) error {
	m.ctx = ctx
	if err := ctx.LoadAssets("loveRap"); err != nil {
		return err
	}
	if err := ctx.Assets.ApplyTexts(); err != nil {
		return err
	}
	m.proj = kart.Translate(engine.ScreenW/2, engine.ScreenH/2).Mul(kart.Scale(54, -54))
	return nil
}

func (m *Module) OnEvent(e *riq.Entity) {
	b := e.Beat
	switch e.Datamodel {
	case "loveRap/bop":
		ev := bopEvt{beat: b, length: e.Length, bop: boolParam(e, "bop"), auto: boolParam(e, "auto")}
		m.bops = append(m.bops, ev)
		if ev.bop {
			for i := 0.0; i < ev.length-1e-6; i++ {
				bb := b + i
				m.ctx.At(bb, func() { m.bop(bb) })
			}
		}
	case "loveRap/intoYou":
		m.scheduleCue(cueEvt{
			beat: b, length: cueLength(e, 2), kind: cueIntoYou, input: inputIntoYou,
			heart: boolParam(e, "heart"), text: intoYouText(boolParam(e, "heart")),
		})
	case "loveRap/crazyIntoYou":
		input := inputCrazy
		if offbeat(b) {
			input = inputCrazyOffbeat
		}
		m.scheduleCue(cueEvt{beat: b, length: cueLength(e, 3), kind: cueCrazy, input: input, text: "Crazy into you!"})
	case "loveRap/foSho":
		input := inputFoSho
		if offbeat(b) {
			input = inputFoShoOffbeat
		}
		m.scheduleCue(cueEvt{beat: b, length: cueLength(e, 1.5), kind: cueFoSho, input: input, text: "Fo' sho'!"})
	case "loveRap/allAboutYou":
		m.scheduleCue(cueEvt{beat: b, length: cueLength(e, 2.5), kind: cueAllAboutYou, input: inputAllAboutYou, text: "All about you!"})
	case "loveRap/setBG":
		ev := bgEvt{
			beat: b, length: e.Length,
			c0: colorParam(e, "colorStart", black), c1: colorParam(e, "colorEnd", black),
			w0:   [4]float64{1, 1, 1, e.Float("wetnessStart", 0)},
			w1:   [4]float64{1, 1, 1, e.Float("wetnessEnd", 0)},
			ease: int(e.Float("ease", 0)),
		}
		m.bgs = append(m.bgs, ev)
		m.ctx.At(b, func() { m.bg = ev })
	}
}

func (m *Module) Ready() {
	sort.Slice(m.bops, func(i, j int) bool { return m.bops[i].beat < m.bops[j].beat })
	sort.Slice(m.bgs, func(i, j int) bool { return m.bgs[i].beat < m.bgs[j].beat })
	sort.Slice(m.cues, func(i, j int) bool { return m.cues[i].beat < m.cues[j].beat })
}

func (m *Module) OnSwitch(beat float64) {
	sec := m.ctx.SecPerBeat(beat)
	paths := make([]string, 0, len(m.ctx.Assets.Animators))
	for p := range m.ctx.Assets.Animators {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, p := range paths {
		m.ctx.Scene.PlayDefaultState(p, beat, sec)
	}
	_ = m.ctx.Assets.SetText(m.role("mcText"), "")
	_ = m.ctx.Assets.SetText(m.role("cpuText"), "")
	_ = m.ctx.Assets.SetText(m.role("playerText"), "")
	m.bg = m.bgAt(beat)
	m.restoreCue(beat)
	m.lastPulse = int(math.Floor(beat))
}

func (m *Module) Whiff(beat float64) {
	m.playRole("playerBody", "cough", beat)
	m.playRole("playerMouth", "Cough", beat)
	m.playRole("playerFace", "E", beat)
	// cpuHeadTrans.localScale.x is -1 in the extracted prefab; C# therefore
	// chooses expression G for this layout.
	m.playRole("cpuFace", "G", beat)
	m.playRole("cpuMouth", "Miss", beat)
	m.ctx.Sound("common_miss")
}

func (m *Module) Update(_ float64, beat float64) {
	pulse := int(math.Floor(beat + 1e-6))
	if pulse >= 0 && pulse != m.lastPulse {
		m.lastPulse = pulse
		if m.autoBopAt(float64(pulse)) {
			m.bop(float64(pulse))
		}
	}
}

func (m *Module) Draw(screen *ebiten.Image, _, beat float64) {
	screen.Fill(color.RGBA{0, 0, 0, 255})
	m.ctx.Scene.SetColorOver(m.role("bgSR"), m.colorAt(m.bg, false, beat))
	m.ctx.Scene.SetColorOver(m.role("bgWetSR"), m.colorAt(m.bg, true, beat))
	m.ctx.SampleScene(beat)
	m.ctx.Scene.Draw(screen, m.proj)
}

// ---------- cue scheduling ----------

func (m *Module) scheduleCue(ev cueEvt) {
	m.cues = append(m.cues, ev)
	m.prepareAnim(ev.beat, ev.kind)
	switch ev.kind {
	case cueIntoYou:
		m.intoYou(ev)
	case cueCrazy:
		m.crazyIntoYou(ev)
	case cueFoSho:
		m.foSho(ev)
	case cueAllAboutYou:
		m.allAboutYou(ev)
	}
	m.ctx.At(ev.beat-0.25, func() { m.inputType = ev.input })
	m.ctx.ScheduleInput(ev.beat+hitOffset(ev.kind), m.just, func() {})
}

func (m *Module) intoYou(ev cueEvt) {
	b := ev.beat
	m.playSounds(1, []soundEvt{
		{0, "Woman/SE_RAP_EN_WOMAN_DAISUKI_1", 0},
		{0.5, "Woman/SE_RAP_EN_WOMAN_DAISUKI_2", 0},
		{0.75, "Woman/SE_RAP_EN_WOMAN_DAISUKI_3", 0.08},
	}, b)
	m.cpuSFX(b+1, inputIntoYou)
	m.ctx.At(b, func() {
		m.prepText(b, ev.text)
		m.heart = ev.heart
		m.playRole("mcBubble", "Appear", b)
		m.playRole("mcBody", "D", b)
		m.playRole("mcMouth", "D", b)
		m.playRole("playerFace", "A", b)
		m.playRole("playerMouth", "idle", b)
		m.playRole("cpuMouth", "idle", b)
		m.playRole("cpuFace", "A", b)
	})
	m.ctx.At(b+1, func() {
		m.playRole("mcBubble", "Hide", b+1)
		m.playRole("cpuBubble", "Appear", b+1)
		m.playRole("cpuBody", "D", b+1)
		m.playRole("cpuMouth", "D", b+1)
	})
	m.ctx.At(b+2, func() { m.playRole("cpuBubble", "Hide", b+2) })
}

func (m *Module) crazyIntoYou(ev cueEvt) {
	b := ev.beat
	if ev.input == inputCrazyOffbeat {
		m.playSounds(1, []soundEvt{
			{0, "Woman/SE_RAP_EN_WOMAN_MAJI_OFFBEAT_1", 0},
			{0.25, "Woman/SE_RAP_EN_WOMAN_MAJI_OFFBEAT_2", 0},
			{0.5, "Woman/SE_RAP_EN_WOMAN_MAJI_OFFBEAT_3", 0},
			{0.75, "Woman/SE_RAP_EN_WOMAN_MAJI_OFFBEAT_4", 0.015},
			{1.25, "Woman/SE_RAP_EN_WOMAN_MAJI_OFFBEAT_5", 0.08},
		}, b)
		m.cpuSFX(b+1.5, inputCrazyOffbeat)
	} else {
		m.playSounds(1, []soundEvt{
			{0, "Woman/SE_RAP_EN_WOMAN_MAJI_1", 0},
			{0.25, "Woman/SE_RAP_EN_WOMAN_MAJI_2", 0},
			{0.5, "Woman/SE_RAP_EN_WOMAN_MAJI_3", 0},
			{1, "Woman/SE_RAP_EN_WOMAN_MAJI_4", 0},
			{1.25, "Woman/SE_RAP_EN_WOMAN_MAJI_5", 0.072},
		}, b)
		m.cpuSFX(b+1.5, inputCrazy)
	}
	m.ctx.At(b, func() {
		m.prepText(b, ev.text)
		m.playRole("mcBubble", "Appear", b)
		m.playRole("mcBody", "MD", b)
		m.playRole("mcMouth", crazyMouth(ev.input), b)
		m.playRole("playerFace", "A", b)
		m.playRole("playerMouth", "idle", b)
		m.playRole("cpuMouth", "idle", b)
		m.playRole("cpuFace", "A", b)
	})
	m.ctx.At(b+1.5, func() {
		m.playRole("cpuBubble", "Appear", b+1.5)
		m.playRole("cpuBody", "MD", b+1.5)
		m.playRole("cpuMouth", crazyMouth(ev.input), b+1.5)
	})
	m.ctx.At(b+2, func() { m.playRole("mcBubble", "Hide", b+2) })
	m.ctx.At(b+2.5, func() { m.playRole("cpuBubble", "Hide", b+2.5) })
}

func (m *Module) foSho(ev cueEvt) {
	b := ev.beat
	if ev.input == inputFoShoOffbeat {
		m.playSounds(1, []soundEvt{
			{0, "Woman/SE_RAP_EN_WOMAN_HONTO_OFFBEAT_1", 0.021},
			{0.125, "Woman/SE_RAP_EN_WOMAN_HONTO_OFFBEAT_2", 0},
			{0.5, "Woman/SE_RAP_EN_WOMAN_HONTO_OFFBEAT_3", 0.11},
		}, b)
		m.cpuSFX(b+0.75, inputFoShoOffbeat)
	} else {
		m.playSounds(1, []soundEvt{
			{0, "Woman/SE_RAP_EN_WOMAN_HONTO_1", 0.017},
			{0.5, "Woman/SE_RAP_EN_WOMAN_HONTO_2", 0.117},
		}, b)
		m.cpuSFX(b+0.75, inputFoSho)
	}
	m.ctx.At(b, func() {
		m.prepText(b, ev.text)
		m.playRole("mcBubble", "Appear", b)
		m.playRole("mcBody", "H", b)
		m.playRole("mcMouth", foShoMouth(ev.input), b)
		m.playRole("playerMouth", "HB", b)
		m.playRole("cpuMouth", "HB", b)
		m.playRole("playerFace", "B", b)
		m.playRole("cpuFace", "B", b)
	})
	m.ctx.At(b+0.75, func() {
		m.playRole("cpuBubble", "Appear", b+0.75)
		m.playRole("cpuBody", "H", b+0.75)
		m.playRole("cpuMouth", foShoMouth(ev.input), b+0.75)
	})
	m.ctx.At(b+1, func() { m.playRole("mcBubble", "Hide", b+1) })
	m.ctx.At(b+1.75, func() { m.playRole("cpuBubble", "Hide", b+1.75) })
}

func (m *Module) allAboutYou(ev cueEvt) {
	b := ev.beat
	m.playSounds(1, []soundEvt{
		{0, "Woman/SE_RAP_EN_WOMAN_SUKINANDA_OFFBEAT_1", 0},
		{0.25, "Woman/SE_RAP_EN_WOMAN_SUKINANDA_OFFBEAT_2", 0},
		{0.5, "Woman/SE_RAP_EN_WOMAN_SUKINANDA_OFFBEAT_3", 0},
		{1, "Woman/SE_RAP_EN_WOMAN_SUKINANDA_OFFBEAT_4", 0.058},
	}, b)
	m.cpuSFX(b+1.25, inputAllAboutYou)
	m.ctx.At(b, func() {
		m.prepText(b, ev.text)
		m.playRole("mcBubble", "Appear", b)
		m.playRole("mcBody", "S", b)
		m.playRole("mcMouth", "S", b)
		m.playRole("playerMouth", "SB", b)
		m.playRole("cpuMouth", "SB", b)
		m.playRole("playerFace", "C", b)
		m.playRole("cpuFace", "C", b)
	})
	m.ctx.At(b+1, func() { m.playRole("mcBubble", "Hide", b+1) })
	m.ctx.At(b+1.25, func() {
		m.playRole("cpuBubble", "Appear", b+1.25)
		m.playRole("cpuBody", "S", b+1.25)
		m.playRole("cpuMouth", "S", b+1.25)
	})
	m.ctx.At(b+2.25, func() { m.playRole("cpuBubble", "Hide", b+2.25) })
}

func (m *Module) prepareAnim(beat float64, kind int) {
	anim, length := prepAnim(kind)
	nextPrepare := false
	m.ctx.At(beat-1, func() {
		nextPrepare = anim != m.lastAnim || m.lastBeat+(length+0.5) <= beat
		if nextPrepare {
			m.playRole("mcBody", anim+"B", beat-1)
		}
	})
	m.ctx.At(beat, func() {
		if nextPrepare {
			m.playRole("playerBody", anim+"B", beat)
			m.playRole("cpuBody", anim+"B", beat)
		}
		m.lastAnim = anim
		m.lastBeat = beat
	})
}

func (m *Module) prepText(beat float64, s string) {
	_ = m.ctx.Assets.SetText(m.role("mcText"), s)
	m.ctx.At(beat+0.5, func() {
		_ = m.ctx.Assets.SetText(m.role("cpuText"), s)
		_ = m.ctx.Assets.SetText(m.role("playerText"), s)
	})
}

// ---------- hit response / sounds ----------

func (m *Module) just(state float64, _ engine.Judgment) {
	id := m.inputType
	beat := m.ctx.Beat()
	pitch := 1.0
	if state >= 1 || state <= -1 {
		if state >= 1 {
			pitch = 0.8
		} else {
			pitch = 1.2
		}
		m.playRole("playerBubble", "AppearMiss", beat)
		m.ctx.At(beat+1, func() { m.playRole("playerBubble", "HideMiss", beat+1) })
	} else {
		m.playRole("playerBubble", "Appear", beat)
		m.playRole("playerFlash", "FadeOut", beat)
		m.playRole("cpuFlash", "FadeOut", beat)
		m.playRole("car_lights", "light", beat)
		m.ctx.At(beat+1, func() { m.playRole("playerBubble", "Hide", beat+1) })
	}

	switch id {
	case inputIntoYou:
		m.playSoundsRuntime(pitch, []soundEvt{
			{0, "Right/SE_RAP_EN_MAN_RIGHT_DAISUKI_1", 0},
			{0.5, "Right/SE_RAP_EN_MAN_RIGHT_DAISUKI_2", 0},
			{0.75, "Right/SE_RAP_EN_MAN_RIGHT_DAISUKI_3", 0.034},
		}, beat)
		m.playRole("playerBody", "D", beat)
		m.playRole("playerMouth", "D", beat)
		face := "A"
		if m.heart {
			face = "D"
		}
		m.playRole("playerFace", face, beat)
		m.playRole("cpuFace", face, beat)
	case inputCrazy, inputCrazyOffbeat:
		m.playSoundsRuntime(pitch, crazyRightSounds(id), beat)
		m.playRole("playerBody", "MD", beat)
		m.playRole("playerMouth", crazyMouth(id), beat)
		m.playRole("playerFace", "A", beat)
		m.playRole("cpuFace", "A", beat)
	case inputFoSho, inputFoShoOffbeat:
		m.playSoundsRuntime(pitch, foShoRightSounds(id), beat)
		m.playRole("playerBody", "H", beat)
		m.playRole("playerMouth", foShoMouth(id), beat)
		m.playRole("playerFace", "B", beat)
		m.playRole("cpuFace", "B", beat)
	case inputAllAboutYou:
		m.playSoundsRuntime(pitch, []soundEvt{
			{0, "Right/SE_RAP_EN_MAN_RIGHT_SUKINANDA_1", 0},
			{0.25, "Right/SE_RAP_EN_MAN_RIGHT_SUKINANDA_2", 0},
			{0.5, "Right/SE_RAP_EN_MAN_RIGHT_SUKINANDA_3", 0},
			{1, "Right/SE_RAP_EN_MAN_RIGHT_SUKINANDA_4", 0},
		}, beat)
		m.playRole("playerBody", "S", beat)
		m.playRole("playerMouth", "S", beat)
		m.playRole("playerFace", "C", beat)
		m.playRole("cpuFace", "C", beat)
	}
}

func (m *Module) playSoundsRuntime(pitch float64, sounds []soundEvt, baseBeat float64) {
	now := m.ctx.Beat()
	for _, s := range sounds {
		target := baseBeat + s.at
		if target <= now+1e-6 {
			m.ctx.SoundPitchOff(s.name, 1, pitch, s.offset)
			continue
		}
		m.ctx.SoundAtPitchOff(target, s.name, 1, pitch, s.offset)
	}
}

func (m *Module) cpuSFX(beat float64, id int) {
	switch id {
	case inputIntoYou:
		m.playSounds(1, []soundEvt{
			{0, "Left/SE_RAP_EN_MAN_LEFT_DAISUKI_1", 0},
			{0.5, "Left/SE_RAP_EN_MAN_LEFT_DAISUKI_2", 0},
			{0.75, "Left/SE_RAP_EN_MAN_LEFT_DAISUKI_3", 0.034},
		}, beat)
	case inputCrazy:
		m.playSounds(1, []soundEvt{
			{0, "Left/SE_RAP_EN_MAN_LEFT_MAJI_1", 0},
			{0.25, "Left/SE_RAP_EN_MAN_LEFT_MAJI_2", 0},
			{0.5, "Left/SE_RAP_EN_MAN_LEFT_MAJI_3", 0},
			{1, "Left/SE_RAP_EN_MAN_LEFT_MAJI_4", 0},
			{1.25, "Left/SE_RAP_EN_MAN_LEFT_MAJI_5", 0},
		}, beat)
	case inputCrazyOffbeat:
		m.playSounds(1, []soundEvt{
			{0, "Left/SE_RAP_EN_MAN_LEFT_MAJI_OFFBEAT_1", 0},
			{0.25, "Left/SE_RAP_EN_MAN_LEFT_MAJI_OFFBEAT_2", 0},
			{0.5, "Left/SE_RAP_EN_MAN_LEFT_MAJI_OFFBEAT_3", 0},
			{0.75, "Left/SE_RAP_EN_MAN_LEFT_MAJI_OFFBEAT_4", 0},
			{1.25, "Left/SE_RAP_EN_MAN_LEFT_MAJI_OFFBEAT_5", 0},
		}, beat)
	case inputFoSho:
		// C# case 4 intentionally uses the OFFBEAT left voice, even though the
		// caller names this path the normal Fo' sho' branch.
		m.playSounds(1, []soundEvt{
			{0, "Left/SE_RAP_EN_MAN_LEFT_HONTO_OFFBEAT_1", 0},
			{0.125, "Left/SE_RAP_EN_MAN_LEFT_HONTO_OFFBEAT_2", 0},
			{0.5, "Left/SE_RAP_EN_MAN_LEFT_HONTO_OFFBEAT_3", 0.098},
		}, beat)
	case inputFoShoOffbeat:
		m.playSounds(1, []soundEvt{
			{0, "Left/SE_RAP_EN_MAN_LEFT_HONTO_1", 0.028},
			{0.5, "Left/SE_RAP_EN_MAN_LEFT_HONTO_2", 0.113},
		}, beat)
	case inputAllAboutYou:
		m.playSounds(1, []soundEvt{
			{0, "Left/SE_RAP_EN_MAN_LEFT_SUKINANDA_1", 0},
			{0.25, "Left/SE_RAP_EN_MAN_LEFT_SUKINANDA_2", 0},
			{0.5, "Left/SE_RAP_EN_MAN_LEFT_SUKINANDA_3", 0},
			{1, "Left/SE_RAP_EN_MAN_LEFT_SUKINANDA_4", 0},
		}, beat)
	}
}

func (m *Module) playSounds(pitch float64, sounds []soundEvt, baseBeat float64) {
	for _, s := range sounds {
		s := s
		m.ctx.SoundAtPitchOff(baseBeat+s.at, s.name, 1, pitch, s.offset)
	}
}

// ---------- restore / bop / background ----------

func (m *Module) restoreCue(beat float64) {
	var last *cueEvt
	for i := range m.cues {
		ev := &m.cues[i]
		if ev.beat-1 <= beat && beat <= ev.beat+ev.length {
			last = ev
		}
	}
	if last == nil || beat < last.beat {
		return
	}
	m.inputType = last.input
	m.heart = last.heart
	_ = m.ctx.Assets.SetText(m.role("mcText"), last.text)
	if beat >= last.beat+0.5 {
		_ = m.ctx.Assets.SetText(m.role("cpuText"), last.text)
		_ = m.ctx.Assets.SetText(m.role("playerText"), last.text)
	}
	m.restoreCuePose(*last, beat)
}

func (m *Module) restoreCuePose(ev cueEvt, beat float64) {
	switch ev.kind {
	case cueIntoYou:
		m.playRole("mcBody", "D", beat)
		m.playRole("mcMouth", "D", beat)
		if beat >= ev.beat+1 && beat < ev.beat+2 {
			m.playRole("mcBubble", "Hide", beat)
			m.playRole("cpuBubble", "Appear", beat)
			m.playRole("cpuBody", "D", beat)
			m.playRole("cpuMouth", "D", beat)
		} else if beat < ev.beat+1 {
			m.playRole("mcBubble", "Appear", beat)
		}
	case cueCrazy:
		m.playRole("mcBody", "MD", beat)
		m.playRole("mcMouth", crazyMouth(ev.input), beat)
		if beat >= ev.beat+1.5 && beat < ev.beat+2.5 {
			m.playRole("cpuBubble", "Appear", beat)
			m.playRole("cpuBody", "MD", beat)
			m.playRole("cpuMouth", crazyMouth(ev.input), beat)
		}
		if beat < ev.beat+2 {
			m.playRole("mcBubble", "Appear", beat)
		}
	case cueFoSho:
		m.playRole("mcBody", "H", beat)
		m.playRole("mcMouth", foShoMouth(ev.input), beat)
		if beat >= ev.beat+0.75 && beat < ev.beat+1.75 {
			m.playRole("cpuBubble", "Appear", beat)
			m.playRole("cpuBody", "H", beat)
			m.playRole("cpuMouth", foShoMouth(ev.input), beat)
		}
		if beat < ev.beat+1 {
			m.playRole("mcBubble", "Appear", beat)
		}
	case cueAllAboutYou:
		m.playRole("mcBody", "S", beat)
		m.playRole("mcMouth", "S", beat)
		if beat >= ev.beat+1.25 && beat < ev.beat+2.25 {
			m.playRole("cpuBubble", "Appear", beat)
			m.playRole("cpuBody", "S", beat)
			m.playRole("cpuMouth", "S", beat)
		}
		if beat < ev.beat+1 {
			m.playRole("mcBubble", "Appear", beat)
		}
	}
}

func (m *Module) bop(beat float64) {
	m.playRole("playerRapper", "Beat", beat)
	m.playRole("cpuRapper", "Beat", beat)
	m.playRole("mcLegs", "Beat", beat)
	m.playRole("car", "beat", beat)
}

func (m *Module) autoBopAt(beat float64) bool {
	on := true
	for _, ev := range m.bops {
		if ev.beat > beat {
			break
		}
		on = ev.auto
	}
	return on
}

func (m *Module) bgAt(beat float64) bgEvt {
	bg := bgEvt{c0: black, c1: black, w0: wet0, w1: wet0}
	for _, ev := range m.bgs {
		if ev.beat > beat {
			break
		}
		bg = ev
	}
	return bg
}

func (m *Module) colorAt(ev bgEvt, wet bool, beat float64) [4]float64 {
	norm := 1.0
	if ev.length > 0 {
		norm = clamp01((beat - ev.beat) / ev.length)
	}
	a, b := ev.c0, ev.c1
	if wet {
		a, b = ev.w0, ev.w1
	}
	var out [4]float64
	for i := range out {
		out[i] = engine.Ease(ev.ease, a[i], b[i], norm)
	}
	return out
}

// ---------- small helpers ----------

func (m *Module) role(key string) string { return m.ctx.Role(key) }

func (m *Module) playRole(role, state string, beat float64) {
	if p := m.role(role); p != "" {
		m.ctx.Scene.PlayState(p, state, beat, 0.5)
	}
}

func prepAnim(kind int) (string, float64) {
	switch kind {
	case cueIntoYou:
		return "D", 2
	case cueCrazy:
		return "MD", 3
	case cueFoSho:
		return "H", 1.5
	case cueAllAboutYou:
		return "S", 2.5
	default:
		return "", 0
	}
}

func hitOffset(kind int) float64 {
	switch kind {
	case cueCrazy:
		return 1.5
	case cueFoSho:
		return 0.75
	case cueAllAboutYou:
		return 1.25
	default:
		return 1
	}
}

func cueLength(e *riq.Entity, def float64) float64 {
	if e.Length > 0 {
		return e.Length
	}
	return def
}

func intoYouText(heart bool) string {
	if heart {
		return "Into you ♥"
	}
	return "Into you!"
}

func crazyMouth(input int) string {
	if input == inputCrazyOffbeat {
		return "MD_ura"
	}
	return "MD"
}

func foShoMouth(input int) string {
	if input == inputFoSho {
		return "H_ura"
	}
	return "H"
}

func crazyRightSounds(input int) []soundEvt {
	if input == inputCrazyOffbeat {
		return []soundEvt{
			{0, "Right/SE_RAP_EN_MAN_RIGHT_MAJI_OFFBEAT_1", 0},
			{0.25, "Right/SE_RAP_EN_MAN_RIGHT_MAJI_OFFBEAT_2", 0},
			{0.5, "Right/SE_RAP_EN_MAN_RIGHT_MAJI_OFFBEAT_3", 0},
			{0.75, "Right/SE_RAP_EN_MAN_RIGHT_MAJI_OFFBEAT_4", 0},
			{1.25, "Right/SE_RAP_EN_MAN_RIGHT_MAJI_OFFBEAT_5", 0},
		}
	}
	return []soundEvt{
		{0, "Right/SE_RAP_EN_MAN_RIGHT_MAJI_1", 0},
		{0.25, "Right/SE_RAP_EN_MAN_RIGHT_MAJI_2", 0},
		{0.5, "Right/SE_RAP_EN_MAN_RIGHT_MAJI_3", 0},
		{1, "Right/SE_RAP_EN_MAN_RIGHT_MAJI_4", 0},
		{1.25, "Right/SE_RAP_EN_MAN_RIGHT_MAJI_5", 0},
	}
}

func foShoRightSounds(input int) []soundEvt {
	if input == inputFoSho {
		return []soundEvt{
			{0, "Right/SE_RAP_EN_MAN_RIGHT_HONTO_OFFBEAT_1", 0},
			{0.125, "Right/SE_RAP_EN_MAN_RIGHT_HONTO_OFFBEAT_2", 0},
			{0.5, "Right/SE_RAP_EN_MAN_RIGHT_HONTO_OFFBEAT_3", 0.071},
		}
	}
	return []soundEvt{
		{0, "Right/SE_RAP_EN_MAN_RIGHT_HONTO_1", 0.028},
		{0.5, "Right/SE_RAP_EN_MAN_RIGHT_HONTO_2", 0.069},
	}
}

func offbeat(beat float64) bool { return math.Abs(math.Mod(beat, 0.5)-0.25) < 1e-6 }

func boolParam(e *riq.Entity, key string) bool { return e.Float(key, 0) != 0 }

func colorParam(e *riq.Entity, key string, def [4]float64) [4]float64 {
	v, ok := e.Data[key]
	if !ok {
		return def
	}
	switch c := v.(type) {
	case []any:
		if len(c) >= 4 {
			return [4]float64{num(c[0], def[0]), num(c[1], def[1]), num(c[2], def[2]), num(c[3], def[3])}
		}
	case map[string]any:
		return [4]float64{num(c["r"], def[0]), num(c["g"], def[1]), num(c["b"], def[2]), num(c["a"], def[3])}
	}
	return def
}

func num(v any, def float64) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case int:
		return float64(x)
	default:
		return def
	}
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
