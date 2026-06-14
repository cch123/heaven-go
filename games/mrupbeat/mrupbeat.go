// Package mrupbeat ports Mr. Upbeat's offbeat stepping, metronome,
// antenna blips, count-in sounds, ding stop cue, palette controls, and
// animation-event input lockout.
package mrupbeat

import (
	"image/color"
	"math"
	"sort"

	"github.com/hajimehoshi/ebiten/v2"

	"hsdemo/engine"
	"hsdemo/kart"
	"hsdemo/riq"
)

var (
	bgDefault     = [4]float64{224.0 / 255, 224.0 / 255, 224.0 / 255, 1}
	white         = [4]float64{1, 1, 1, 1}
	blipDefault   = [4]float64{0, 1, 0, 1}
	shadowDefault = [4]float64{0.12156863, 0, 0, 0.18039216}
	stageFill     = color.NRGBA{R: 0xe0, G: 0xe0, B: 0xe0, A: 0xff}
)

const (
	stepStateSpeed = 0.25
	stepTimeScale  = 0.5
	// Step.anim fires ToggleStepping(1) at clip time 0.033333s. The scene
	// player advances clip seconds as beatDelta*stateSpeed*timeScale, so this
	// is when whiff stepping becomes legal again.
	stepUnlockBeatDelta = 0.033333335 / (stepStateSpeed * stepTimeScale)
)

type prepareEvt struct {
	beat, length          float64
	startBlip, startStep  float64
	forceOnbeat, countIn  bool
	activeAtPrepare       bool
	visualBlipStart       float64
	inactiveBlipUntilBeat float64
}

type dingEvt struct {
	beat                  float64
	applause, stopBlip    bool
	playDing, activeState bool
}

type bgEvt struct {
	beat, length float64
	start, end   [4]float64
	ease         int
}

type upbeatColorEvt struct {
	beat         float64
	blip, shadow [4]float64
	setShadow    bool
}

type blipEvt struct {
	beat                          float64
	letter                        string
	shouldGrow, reset, shouldBlip bool
	length                        int
}

type stepChain struct {
	start, stop float64
}

type Module struct {
	ctx  *engine.Ctx
	proj kart.Aff

	prepares []prepareEvt
	dings    []dingEvt
	bgs      []bgEvt
	colours  []upbeatColorEvt
	blips    []blipEvt
	forces   []stepChain

	metronomePath string
	manPath       string
	bgPath        string
	blipPath      string
	antennaPath   string
	textPath      string
	blipMat       string
	shadows       []string
	shadowSRs     []string

	shadowDefaults map[string][4]float64
	chains         []stepChain

	stepIterate     int
	facingLeft      bool
	canStep         bool
	canStepFromAnim bool
	shouldGrow      bool
	shouldBlip      bool
	blipString      string
	blipSize        int
	blipLength      int
}

func New() engine.Module { return &Module{} }

func (m *Module) ID() string { return "mrUpbeat" }

func (m *Module) Load(ctx *engine.Ctx) error {
	m.ctx = ctx
	if err := ctx.LoadAssets("mrUpbeat"); err != nil {
		return err
	}
	if err := ctx.Assets.ApplyTexts(); err != nil {
		return err
	}
	m.proj = kart.Translate(engine.ScreenW/2, engine.ScreenH/2).Mul(kart.Scale(54, -54))

	m.metronomePath = roleOr(ctx, "metronomeAnim", "Metronome")
	m.manPath = roleOr(ctx, "man", "MrUpbeat")
	m.bgPath = roleOr(ctx, "bg", "Background")
	game := ctx.Assets.Extra.Components["game"]
	man := ctx.Assets.Extra.Components["man"]
	m.blipMat = game.Refs["blipMaterial"]
	m.blipPath = man.Refs["blipAnim"]
	m.antennaPath = man.Refs["antennaLight"]
	m.textPath = man.Refs["blipText"]
	m.shadows = append(m.shadows, man.RefArrays["shadows"]...)
	m.shadowSRs = append(m.shadowSRs, game.RefArrays["shadowSr"]...)
	if len(m.shadowSRs) == 0 {
		m.shadowSRs = append(m.shadowSRs, ctx.Assets.Extra.RefArrays["shadowSr"]...)
	}
	m.shadowDefaults = map[string][4]float64{}
	for _, p := range m.shadowSRs {
		m.shadowDefaults[p] = nodeColor(ctx, p, shadowDefault)
	}

	m.shouldBlip = true
	m.blipString = man.Strs["blipString"]
	if m.blipString == "" {
		m.blipString = "M"
	}
	m.blipLength = 4
	m.canStepFromAnim = true
	ctx.Scene.PlayDefaultState(m.metronomePath, 0, ctx.SecPerBeat(0))
	ctx.Scene.PlayDefaultState(m.manPath, 0, ctx.SecPerBeat(0))
	ctx.Scene.PlayDefaultState(m.blipPath, 0, ctx.SecPerBeat(0))
	return nil
}

func roleOr(ctx *engine.Ctx, key, fallback string) string {
	if p := ctx.Role(key); p != "" {
		return p
	}
	return fallback
}

func nodeColor(ctx *engine.Ctx, path string, fallback [4]float64) [4]float64 {
	if i, ok := ctx.Assets.NodeIndex(path); ok {
		if c := ctx.Assets.Rig.Nodes[i].Color; c != [4]float64{} {
			return c
		}
	}
	return fallback
}

func (m *Module) OnEvent(e *riq.Entity) {
	b := e.Beat
	switch e.Datamodel {
	case "mrUpbeat/prepare":
		p := prepareEvt{
			beat: b, length: e.Length,
			forceOnbeat:     boolParam(e, "forceOnbeat"),
			countIn:         boolParam(e, "countIn"), // legacy updater defaults missing countIn to false.
			activeAtPrepare: m.ctx.GameAt(b) == m.ID(),
		}
		p.startBlip, p.startStep = prepareStarts(b, e.Length, p.forceOnbeat)
		m.prepares = append(m.prepares, p)
	case "mrUpbeat/ding":
		d := dingEvt{
			beat: b, applause: boolParam(e, "toggle"),
			stopBlip:    boolParamDefault(e, "stopBlipping", true),
			playDing:    boolParamDefault(e, "playDing", true),
			activeState: m.ctx.GameAt(b) == m.ID(),
		}
		m.dings = append(m.dings, d)
	case "mrUpbeat/changeBG":
		ease := int(e.Float("ease", 0))
		if _, ok := e.Data["ease"]; !ok {
			// Heaven Studio's updater converts legacy toggle events: true =
			// Instant, false/missing = Linear.
			if boolParam(e, "toggle") {
				ease = 1
			}
		}
		m.bgs = append(m.bgs, bgEvt{
			beat: b, length: e.Length,
			start: colorParam(e, "start", bgDefault),
			end:   colorParam(e, "end", bgDefault),
			ease:  ease,
		})
	case "mrUpbeat/upbeatColors":
		m.colours = append(m.colours, upbeatColorEvt{
			beat:      b,
			blip:      colorParam(e, "blipColor", blipDefault),
			setShadow: boolParam(e, "setShadow"),
			shadow:    colorParam(e, "shadowColor", shadowDefault),
		})
	case "mrUpbeat/blipEvents":
		m.blips = append(m.blips, blipEvt{
			beat: b, letter: e.Str("letter", ""),
			shouldGrow: boolParamDefault(e, "shouldGrow", true),
			reset:      boolParam(e, "resetBlip"),
			shouldBlip: boolParamDefault(e, "shouldBlip", true),
			length:     int(e.Float("blipLength", 4)),
		})
	case "mrUpbeat/fourBeatCountInOffbeat":
		scheduleCountIn(m.ctx, b, e.Length, boolParamDefault(e, "a", true))
	case "mrUpbeat/countOffbeat":
		m.ctx.SoundAt(b, countName(int(e.Float("number", 0))), 1)
	case "mrUpbeat/forceStepping":
		if m.ctx.GameAt(b) == m.ID() {
			m.forces = append(m.forces, stepChain{start: b, stop: b + e.Length})
		}
	}
}

func (m *Module) Ready() {
	sort.Slice(m.prepares, func(i, j int) bool { return m.prepares[i].beat < m.prepares[j].beat })
	sort.Slice(m.dings, func(i, j int) bool { return m.dings[i].beat < m.dings[j].beat })
	sort.Slice(m.bgs, func(i, j int) bool { return m.bgs[i].beat < m.bgs[j].beat })
	sort.Slice(m.colours, func(i, j int) bool { return m.colours[i].beat < m.colours[j].beat })

	for _, d := range m.dings {
		if d.playDing {
			m.ctx.SoundAt(d.beat, "ding", 1)
		}
		if d.applause {
			m.ctx.SoundAt(d.beat, "common_applause", 1)
		}
		if d.activeState {
			d := d
			// Ding stops recursive stepping on the next metronome beat. The
			// precomputed step/blip chains are truncated at the same stop beats.
			m.ctx.At(d.beat, func() { m.canStep = false })
		}
	}
	for _, b := range m.blips {
		b := b
		m.ctx.At(b.beat, func() {
			if b.reset {
				m.blipSize = 0
			}
			m.shouldGrow = b.shouldGrow
			m.blipString = b.letter
			m.shouldBlip = b.shouldBlip
			m.blipLength = b.length
		})
	}

	for _, p := range m.prepares {
		if p.countIn {
			scheduleStretchCountIn(m.ctx, p.beat, p.length)
		}
		if p.activeAtPrepare {
			m.scheduleBlips(p.startBlip, m.stopBlipBeatAfter(p.startBlip))
		} else if sw, ok := m.nextSwitchToSelf(p.startBlip); ok && sw < p.startBlip+p.length+1 {
			for t := p.startBlip; t < sw; t++ {
				m.ctx.SoundAt(t, "blip", 1)
			}
			m.scheduleBlips(alignBlipStart(sw, p.startBlip), m.stopBlipBeatAfter(sw))
		}
		stop := m.stopStepBeatAfter(p.startStep)
		m.chains = append(m.chains, stepChain{start: p.startStep, stop: stop})
		m.scheduleSteps(p.startStep, stop)
	}
	for _, f := range m.forces {
		m.chains = append(m.chains, f)
		m.scheduleSteps(f.start, f.stop)
	}
}

func prepareStarts(beat, length float64, forceOnbeat bool) (startBlip, startStep float64) {
	if !forceOnbeat {
		beat = math.Floor(beat) + 0.5
		length = math.Round(length)
	}
	return beat, beat + length - 0.5
}

func alignBlipStart(switchBeat, startBlip float64) float64 {
	frac := math.Mod(switchBeat, 1)
	base := math.Round(switchBeat)
	if math.Abs(frac-0.5) < 1e-6 {
		base = math.Floor(switchBeat)
	}
	return base + math.Mod(startBlip, 1)
}

func (m *Module) nextSwitchToSelf(after float64) (float64, bool) {
	want := "gameManager/switchGame/" + m.ID()
	for _, e := range m.ctx.Entities() {
		if e.Beat > after && e.Datamodel == want {
			return e.Beat, true
		}
	}
	return 0, false
}

func (m *Module) stopBlipBeatAfter(start float64) float64 {
	stop := m.ctx.NextSwitchBeat(start)
	for _, d := range m.dings {
		if d.stopBlip && d.beat-0.5 >= start && d.beat-0.5 < stop {
			stop = d.beat - 0.5
		}
	}
	return stop
}

func (m *Module) stopStepBeatAfter(start float64) float64 {
	stop := m.ctx.NextSwitchBeat(start)
	for _, d := range m.dings {
		if d.beat >= start && d.beat < stop {
			stop = d.beat
		}
	}
	return stop
}

func (m *Module) scheduleSteps(start, stop float64) {
	if math.IsInf(stop, 1) {
		stop = start + 128
	}
	for beat := start; beat < stop; beat++ {
		b := beat
		m.ctx.At(b, func() { m.metronomeStep(b) })
		m.ctx.ScheduleInput(b+0.5, func(state float64, _ engine.Judgment) {
			m.step(false, m.ctx.Beat())
			if state >= 1 || state <= -1 {
				m.ctx.PlayCommon("nearMiss")
			}
		}, func() {
			m.fall(m.ctx.Beat())
		})
	}
}

func (m *Module) scheduleBlips(start, stop float64) {
	if math.IsInf(stop, 1) {
		stop = start + 128
	}
	for beat := start; beat < stop; beat++ {
		b := beat
		m.ctx.At(b, func() { m.doBlip(b) })
	}
}

func (m *Module) metronomeStep(beat float64) {
	m.canStep = true
	left := m.stepIterate%2 == 0
	dir := "Left"
	if !left {
		dir = "Right"
	}
	m.ctx.Sound("metronome" + dir)
	m.ctx.Scene.PlayState(m.metronomePath, "MetronomeGo"+dir, beat, 1)
	m.stepIterate++
}

func (m *Module) doBlip(beat float64) {
	if !m.shouldBlip {
		return
	}
	m.ctx.Sound("blip")
	real := m.blipLength - 4
	idx := m.blipSize + 1 - real
	if idx < 1 {
		idx = 1
	}
	if idx > 5 {
		idx = 5
	}
	m.ctx.Scene.PlayState(m.blipPath, "Blip"+string(rune('0'+idx)), beat, 0.5)
	textOn := m.blipSize-real >= 4
	m.ctx.Scene.SetActive(m.textPath, textOn)
	if textOn {
		text := m.blipString
		if text == "" {
			text = " "
		}
		_ = m.ctx.Assets.SetText(m.textPath, text)
	}
	if m.shouldGrow && m.blipSize-real < 4 {
		m.blipSize++
	}
}

func (m *Module) OnSwitch(beat float64) {
	m.stepIterate = 0
	m.facingLeft = false
	m.canStep = false
	m.canStepFromAnim = true
	m.shouldGrow = false
	m.shouldBlip = true
	m.blipString = "M"
	m.blipSize = 0
	m.blipLength = 4

	sc := m.ctx.Scene
	sc.PlayDefaultState(m.metronomePath, beat, m.ctx.SecPerBeat(beat))
	sc.PlayDefaultState(m.manPath, beat, m.ctx.SecPerBeat(beat))
	sc.PlayDefaultState(m.blipPath, beat, m.ctx.SecPerBeat(beat))
	sc.SetMirrorX(m.manPath, false)
	sc.SetActive(m.textPath, false)
	for i, p := range m.shadows {
		sc.SetActive(p, i == 0)
	}
	for _, ch := range m.chains {
		if beat >= ch.start-2 && beat < ch.stop {
			m.canStep = true
			break
		}
	}
}

func (m *Module) Whiff(beat float64) {
	if m.canStep && m.canStepFromAnim {
		m.step(true, beat)
	}
}

func (m *Module) Update(t, beat float64) {}

func (m *Module) Draw(screen *ebiten.Image, t, beat float64) {
	screen.Fill(stageFill)
	m.applyVisuals(beat)
	m.ctx.SampleScene(beat)
	if m.textPath != "" {
		if w, ok := m.ctx.Scene.NodeWorld(m.antennaPath); ok {
			m.ctx.Scene.SetPosOver(m.textPath, w.Tx, w.Ty+0.7)
		}
		m.ctx.SampleScene(beat)
	}
	m.ctx.Scene.Draw(screen, m.proj)
}

func (m *Module) step(isInput bool, beat float64) {
	if isInput || m.facingCorrectly() {
		if len(m.shadows) >= 2 {
			m.ctx.Scene.SetActive(m.shadows[0], m.facingLeft)
			m.ctx.Scene.SetActive(m.shadows[1], !m.facingLeft)
		}
		m.facingLeft = !m.facingLeft
		m.ctx.Scene.SetMirrorX(m.manPath, m.facingLeft)
	}
	m.canStepFromAnim = false
	m.ctx.At(beat+stepUnlockBeatDelta, func() { m.canStepFromAnim = true })
	m.ctx.Scene.PlayState(m.manPath, "Step", beat, stepTimeScale)
	m.ctx.Sound("step")
}

func (m *Module) fall(beat float64) {
	state := "FallL"
	if m.facingCorrectly() {
		state = "FallR"
	}
	m.ctx.Scene.PlayState(m.manPath, state, beat, 1)
	m.ctx.PlayCommon("miss")
	for _, p := range m.shadows {
		m.ctx.Scene.SetActive(p, false)
	}
	m.facingLeft = !m.facingLeft
	m.ctx.Scene.SetMirrorX(m.manPath, m.facingLeft)
}

func (m *Module) facingCorrectly() bool {
	return (m.stepIterate%2 == 0) == m.facingLeft
}

func (m *Module) applyVisuals(beat float64) {
	bg := m.bgAt(beat)
	m.ctx.Scene.SetColorOver(m.bgPath, bg)
	blip := blipDefault
	shadow, customShadow := shadowDefault, false
	for _, ce := range m.colours {
		if ce.beat > beat {
			break
		}
		blip = ce.blip
		if ce.setShadow {
			shadow = [4]float64{ce.shadow[0], ce.shadow[1], ce.shadow[2], 1}
			customShadow = true
		}
	}
	if m.blipMat != "" {
		m.ctx.Scene.SetPaletteFor(m.blipMat, kart.Palette{Alpha: white, Fill: blip, Outline: white})
	}
	for _, p := range m.shadowSRs {
		c := m.shadowDefaults[p]
		if customShadow {
			c = shadow
		}
		m.ctx.Scene.SetColorOver(p, c)
	}
}

func (m *Module) bgAt(beat float64) [4]float64 {
	cur := bgDefault
	for _, ev := range m.bgs {
		if ev.beat > beat {
			break
		}
		if ev.length <= 0 || beat >= ev.beat+ev.length {
			cur = ev.end
			continue
		}
		v := (beat - ev.beat) / ev.length
		for i := 0; i < 4; i++ {
			cur[i] = engine.Ease(ev.ease, ev.start[i], ev.end[i], v)
		}
	}
	return cur
}

func scheduleCountIn(ctx *engine.Ctx, beat, length float64, a bool) {
	step := length / 4
	if a {
		ctx.SoundAt(beat-0.5*step, "a", 1)
	}
	for i := 0; i < 4; i++ {
		t := beat + float64(i)*step
		if i == 3 {
			ctx.SoundAtOff(t, "4", 1, 0.05)
		} else {
			ctx.SoundAt(t, string(rune('1'+i)), 1)
		}
	}
}

func scheduleStretchCountIn(ctx *engine.Ctx, beat, length float64) {
	start := beat + length - 8
	names := []string{"1", "2", "a", "1", "2", "3", "4"}
	times := []float64{0, 2, 3.5, 4, 5, 6, 7}
	for i, name := range names {
		t := start + times[i]
		if t >= beat {
			ctx.SoundAt(t, name, 1)
		}
	}
}

func countName(n int) string {
	if n < 4 {
		return string(rune('1' + n))
	}
	return "a"
}

func boolParam(e *riq.Entity, key string) bool { return e.Float(key, 0) != 0 }

func boolParamDefault(e *riq.Entity, key string, def bool) bool {
	if _, ok := e.Data[key]; !ok {
		return def
	}
	return boolParam(e, key)
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
