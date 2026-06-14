// Package rhythmfighter ports Rhythm Fighter's call-and-response intervals,
// two-action dodge inputs, note display, fighter walk/fall positioning, and
// ready/fight cue timing.
package rhythmfighter

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
	whoBoth = iota
	whoOpponent
	whoPlayer
	whoNone
)

const (
	displayNormal = iota
	displayQuick
)

const (
	actionPunch = 0
	actionKick  = 3 // HS InputAction_Alt maps to pad South; engine action 3 is L/Down/X.
)

type bopEvt struct {
	beat, length         float64
	whoBops, whoBopsAuto int
}

type callEvt struct {
	beat float64
	kick bool
}

type startIntervalEvt struct {
	beat, length  float64
	autoPass      bool
	display       bool
	displayType   int
	ignoreDisplay bool
}

type passEvt struct {
	beat        float64
	display     bool
	displayType int
}

type displayEvt struct {
	beat, length float64
	displayType  int
	delay        float64
}

type noteInst struct {
	inst        *kart.Instance
	endBeat     float64
	toggleBeat  float64
	visibleInit bool
	x, y        float64
}

type displayWindow struct {
	start, end  float64
	x, y        float64
	baseState   string
	lengthState string
	call        bool
}

type Module struct {
	ctx  *engine.Ctx
	proj kart.Aff

	fighterR, fighterL string
	holderR, holderL   string
	displayHolder      string
	musicNote          string
	lightsL, lightsR   string
	fightText, spot    string

	noteT *kart.Template

	bops     []bopEvt
	calls    []callEvt
	starts   []startIntervalEvt
	passes   []passEvt
	displays []displayEvt
	falls    []float64

	notes          []*noteInst
	displayWindows []displayWindow

	fighterDistance bool
	canDodge        bool
	lBop, rBop      bool
	canBop          bool
	lastPulse       float64
	lastBopBeat     map[string]float64
}

func New() engine.Module {
	return &Module{
		fighterDistance: true,
		lBop:            true,
		rBop:            true,
		canBop:          true,
		lastPulse:       math.Inf(-1),
		lastBopBeat:     map[string]float64{},
	}
}

func (m *Module) ID() string { return "rhythmFighter" }

func (m *Module) Load(ctx *engine.Ctx) error {
	m.ctx = ctx
	if err := ctx.LoadAssets("rhythmFighter"); err != nil {
		return err
	}
	m.proj = kart.Translate(engine.ScreenW/2, engine.ScreenH/2).Mul(kart.Scale(54, -54))
	m.fighterR = roleOr(ctx, "fighterR", "holder r/fighter")
	m.fighterL = roleOr(ctx, "fighterL", "holder l/fighter")
	m.holderR = roleOr(ctx, "holderR", "holder r")
	m.holderL = roleOr(ctx, "holderL", "holder l")
	m.displayHolder = roleOr(ctx, "displayHolder", "note display")
	m.musicNote = roleOr(ctx, "musicNote", "Note")
	m.lightsL = roleOr(ctx, "lightsL", "lights/spotlight l")
	m.lightsR = roleOr(ctx, "lightsR", "lights/spotlight r")
	m.fightText = roleOr(ctx, "fightText", "Fight")
	m.spot = roleOr(ctx, "spotLight", "spot")
	m.noteT = kart.NewTemplate(ctx.Assets, m.musicNote)
	ctx.Scene.SetActive(m.displayHolder, false)
	ctx.Scene.SetActive(m.musicNote, false)
	return nil
}

func (m *Module) OnEvent(e *riq.Entity) {
	switch e.Datamodel {
	case "rhythmFighter/bop":
		m.bops = append(m.bops, bopEvt{
			beat: e.Beat, length: e.Length,
			whoBops:     intParam(e, "whoBops", whoBoth),
			whoBopsAuto: intParam(e, "whoBopsAuto", whoNone),
		})
	case "rhythmFighter/punch":
		m.calls = append(m.calls, callEvt{beat: e.Beat})
	case "rhythmFighter/kick":
		m.calls = append(m.calls, callEvt{beat: e.Beat, kick: true})
	case "rhythmFighter/start interval":
		m.starts = append(m.starts, startIntervalEvt{
			beat: e.Beat, length: e.Length,
			autoPass:      boolParamDefault(e, "autoPass", true),
			display:       boolParamDefault(e, "display", true),
			displayType:   intParam(e, "type", displayNormal),
			ignoreDisplay: boolParam(e, "ignoreDisplay"),
		})
	case "rhythmFighter/pass turn":
		m.passes = append(m.passes, passEvt{
			beat: e.Beat, display: boolParamDefault(e, "display", true),
			displayType: intParam(e, "type", displayNormal),
		})
	case "rhythmFighter/display":
		m.displays = append(m.displays, displayEvt{
			beat: e.Beat, length: e.Length,
			displayType: intParam(e, "type", displayNormal),
			delay:       e.Float("delay", 0),
		})
	case "rhythmFighter/fall":
		m.falls = append(m.falls, e.Beat)
	}
}

func (m *Module) Ready() {
	sort.Slice(m.bops, func(i, j int) bool { return m.bops[i].beat < m.bops[j].beat })
	sort.Slice(m.calls, func(i, j int) bool { return m.calls[i].beat < m.calls[j].beat })
	sort.Slice(m.starts, func(i, j int) bool { return m.starts[i].beat < m.starts[j].beat })
	sort.Slice(m.passes, func(i, j int) bool { return m.passes[i].beat < m.passes[j].beat })
	sort.Slice(m.displays, func(i, j int) bool { return m.displays[i].beat < m.displays[j].beat })
	sort.Float64s(m.falls)

	for _, ev := range m.bops {
		ev := ev
		m.ctx.At(ev.beat, func() {
			m.lBop = ev.whoBopsAuto == whoBoth || ev.whoBopsAuto == whoOpponent
			m.rBop = ev.whoBopsAuto == whoBoth || ev.whoBopsAuto == whoPlayer
		})
		for b := ev.beat; b < ev.beat+ev.length-1e-6; b++ {
			bb := b
			m.ctx.At(bb, func() {
				if ev.whoBops == whoBoth || ev.whoBops == whoOpponent {
					m.doBop(bb, true)
				}
				if ev.whoBops == whoBoth || ev.whoBops == whoPlayer {
					m.doBop(bb, false)
				}
			})
		}
	}
	for _, ev := range m.starts {
		ev := ev
		m.queueStartInterval(ev.beat-1, ev.length, ev.autoPass, ev.display, ev.displayType, ev.ignoreDisplay, math.Inf(-1))
	}
	for _, ev := range m.passes {
		ev := ev
		m.ctx.At(ev.beat, func() {
			m.ctx.Sound("ready")
			m.ctx.SoundAt(ev.beat+2, "fight", 1)
			m.passTurn(ev.beat, ev.display, ev.displayType, false, math.NaN(), math.NaN(), false, nil, false)
		})
	}
	for _, ev := range m.displays {
		ev := ev
		m.display(ev.beat-1, ev.length, ev.displayType, ev.delay, false, false)
	}
	for _, beat := range m.falls {
		b := beat
		m.ctx.At(b, func() { m.fightersFall(b) })
	}
}

func (m *Module) OnSwitch(beat float64) {
	sec := m.ctx.SecPerBeat(beat)
	for _, p := range []string{m.fighterR, m.fighterL, m.holderR, m.holderL, m.displayHolder, m.lightsL, m.lightsR, m.fightText, m.spot} {
		m.ctx.Scene.PlayDefaultState(p, beat, sec)
	}
	m.ctx.Scene.SetActive(m.displayHolder, false)
	m.ctx.Scene.SetActive(m.musicNote, false)
	m.lastPulse = math.Floor(beat)
	m.lastBopBeat = map[string]float64{}
	m.fighterDistance = !m.inCloseRangeAt(beat)
	m.canDodge = m.canDodgeAt(beat)
	m.canBop = !m.inCallIntervalAt(beat)
	m.restoreDisplayAt(beat)
}

func (m *Module) Whiff(beat float64) { m.WhiffAction(beat, actionPunch) }

func (m *Module) WhiffAction(beat float64, action int) {
	if !m.canDodge {
		return
	}
	if action == actionPunch || action == actionKick {
		m.ctx.Scene.PlayState(m.fighterR, "Whiff", beat, 0.5)
	}
}

func (m *Module) Update(_, beat float64) {
	if p := math.Floor(beat); p > m.lastPulse {
		m.lastPulse = p
		if m.lBop {
			m.doBop(p, true)
		}
		if m.rBop {
			m.doBop(p, false)
		}
	}
}

func (m *Module) Draw(screen *ebiten.Image, _, beat float64) {
	screen.Fill(color.RGBA{255, 139, 216, 255})
	m.ctx.SampleScene(beat)
	for _, note := range m.notes {
		if note.inst == nil || beat >= note.endBeat {
			continue
		}
		visible := note.visibleInit
		if beat >= note.toggleBeat {
			visible = !note.visibleInit
		}
		if !visible {
			continue
		}
		note.inst.Queue(m.ctx.Scene, beat, kart.Translate(note.x, note.y), 0)
	}
	m.ctx.Scene.Draw(screen, m.proj)
}

func (m *Module) queueStartInterval(beat, length float64, autoPass, noteDisplay bool, typ int, ignoreDisplay bool, startBeat float64) {
	needed := m.neededCalls(beat+1, length)
	if len(needed) == 0 || startBeat >= beat+1+length {
		return
	}
	if noteDisplay {
		m.display(beat, length, typ, 0, true, true)
	}
	m.ctx.At(beat, func() { m.canBop = false })
	for _, call := range needed {
		call := call
		if call.beat < startBeat || !displayStepMatches(call.beat, beat, typ, ignoreDisplay) {
			continue
		}
		name := "punch"
		state := "Punch"
		if call.kick {
			name = "kick"
			state = "Kick"
		}
		m.ctx.SoundAt(call.beat, name, 1)
		m.ctx.At(call.beat, func() { m.ctx.Scene.PlayState(m.fighterL, state, call.beat, 0.5) })
	}
	m.ctx.At(beat+length, func() { m.canBop = true })
	if autoPass {
		passBeat := beat + autoPassOffset(length)
		m.ctx.At(passBeat, func() {
			m.passTurn(passBeat, noteDisplay, typ, false, beat+1, length, ignoreDisplay, needed, true)
		})
	}
}

func (m *Module) passTurn(beat float64, noteDisplay bool, typ int, inactive bool, startIntervalBeat, startIntervalLength float64, ignoreDisplay bool, needed []callEvt, fromAutoPass bool) {
	if math.IsNaN(startIntervalBeat) || math.IsNaN(startIntervalLength) {
		ev, ok := m.lastStartBefore(beat)
		if !ok {
			return
		}
		startIntervalBeat = ev.beat
		startIntervalLength = ev.length
		ignoreDisplay = ev.ignoreDisplay
	}
	if needed == nil {
		needed = m.neededCalls(startIntervalBeat, startIntervalLength)
	}
	if len(needed) == 0 {
		return
	}
	if fromAutoPass {
		m.ctx.SoundAt(beat, "ready", 1)
		m.ctx.SoundAt(beat+2, "fight", 1)
	}
	if m.fighterDistance {
		if inactive {
			m.fighterDistance = false
			m.ctx.Scene.PlayState(m.holderL, "Advanced", beat, 0.5)
			m.ctx.Scene.PlayState(m.holderR, "Advanced", beat, 0.5)
		} else {
			m.ctx.At(beat, func() {
				m.ctx.Scene.PlayState(m.holderL, "Advance", beat, 0.5)
				m.ctx.Scene.PlayState(m.holderR, "Advance", beat, 0.5)
				m.ctx.Scene.PlayState(m.fighterL, "Walk", beat, 0.5)
				m.ctx.Scene.PlayState(m.fighterR, "Walk", beat, 0.5)
				m.fighterDistance = false
			})
			for i := 1; i <= 2; i++ {
				b := beat + float64(i)
				m.ctx.At(b, func() {
					m.ctx.Scene.PlayState(m.fighterL, "Walk", b, 0.5)
					m.ctx.Scene.PlayState(m.fighterR, "Walk", b, 0.5)
				})
			}
		}
	}
	m.ctx.At(beat+2, func() { m.ctx.Scene.PlayState(m.fightText, "Show", beat+2, 0.5) })
	if noteDisplay {
		m.display(beat+3, startIntervalLength, typ, 0, false, true)
	}
	m.ctx.At(beat+4, func() { m.canDodge = true })
	for _, call := range needed {
		call := call
		if !displayStepMatches(call.beat, beat, typ, ignoreDisplay) {
			continue
		}
		relative := call.beat - startIntervalBeat
		actBeat := beat + 4 + relative
		state := "Punch"
		action := actionPunch
		onHit := m.punchHit
		onMiss := m.punchMiss
		if call.kick {
			state = "Kick"
			action = actionKick
			onHit = m.kickHit
			onMiss = m.kickMiss
		}
		m.ctx.At(actBeat, func() { m.ctx.Scene.PlayState(m.fighterL, state, actBeat, 0.5) })
		m.ctx.ScheduleInputAction(actBeat, action, onHit, onMiss)
	}
	m.ctx.At(beat+4+startIntervalLength, func() { m.canDodge = false })
}

func (m *Module) fightersFall(beat float64) {
	if m.fighterDistance {
		return
	}
	m.ctx.Scene.PlayState(m.holderL, "Fall", beat, 0.5)
	m.ctx.Scene.PlayState(m.holderR, "Fall", beat, 0.5)
	m.fighterDistance = true
	for i := 0; i <= 2; i++ {
		b := beat + float64(i)
		m.ctx.At(b, func() {
			m.ctx.Scene.PlayState(m.fighterL, "Retreat", b, 0.5)
			m.ctx.Scene.PlayState(m.fighterR, "Retreat", b, 0.5)
		})
	}
}

func (m *Module) doBop(beat float64, opponent bool) {
	if !m.canBop || !m.fighterDistance {
		return
	}
	path := m.fighterR
	key := "R"
	if opponent {
		path = m.fighterL
		key = "L"
	}
	if math.Abs(beat-m.lastBopBeat[key]) < 1e-6 {
		return
	}
	m.lastBopBeat[key] = beat
	m.ctx.Scene.PlayState(path, "Bop", beat, 0.5)
}

func (m *Module) punchHit(state float64, _ engine.Judgment) {
	beat := m.ctx.App.BeatNow()
	m.ctx.Scene.PlayState(m.fighterR, "Duck", beat, 0.5)
	m.ctx.Sound("punch_dodge")
}

func (m *Module) punchMiss() {
	beat := m.ctx.App.BeatNow()
	m.ctx.Scene.PlayState(m.fighterR, "Punched", beat, 0.5)
	m.ctx.Sound("punch_hit")
	m.ctx.Sound("punch")
}

func (m *Module) kickHit(state float64, _ engine.Judgment) {
	beat := m.ctx.App.BeatNow()
	m.ctx.Scene.PlayState(m.fighterR, "Jump", beat, 0.5)
	m.ctx.Sound("kick_dodge")
}

func (m *Module) kickMiss() {
	beat := m.ctx.App.BeatNow()
	m.ctx.Scene.PlayState(m.fighterR, "Kicked", beat, 0.5)
	m.ctx.Sound("kick_hit")
	m.ctx.Sound("kick")
}

func (m *Module) display(beat, length float64, typ int, delay float64, call, disappear bool) {
	holderX, holderY := 0.0, 2.781
	if call {
		holderX, holderY = -3.2, 1.93
	}
	count := int(math.Ceil(length))
	if typ == displayQuick {
		count *= 2
	}
	if count < 1 {
		count = 1
	}
	noteX := 1.17
	if length > 4 {
		noteX = 2.91
	}
	noteDistance := 0.0
	if count > 1 {
		noteDistance = noteX * 2 / float64(count-1)
	}
	endBeat := beat + length + delay
	if typ == displayQuick {
		endBeat += 0.5
	}
	baseState := "Normal"
	if call {
		baseState = "Call"
	}
	lengthState := "Four"
	if length > 4 {
		lengthState = "Eight"
	}
	m.displayWindows = append(m.displayWindows, displayWindow{
		start: beat, end: endBeat, x: holderX, y: holderY,
		baseState: baseState, lengthState: lengthState, call: call,
	})

	m.ctx.At(beat, func() {
		m.ctx.Scene.SetActive(m.displayHolder, true)
		m.ctx.Scene.SetPosOver(m.displayHolder, holderX, holderY)
		if call {
			m.ctx.Scene.PlayState(m.spot, "Show", beat, 0.5)
		}
		m.ctx.Scene.PlayStateLayer("rhythmFighter/display/base", m.displayHolder, baseState, beat, 0.5)
		m.ctx.Scene.PlayStateLayer("rhythmFighter/display/length", m.displayHolder, lengthState, beat, 0.5)
	})
	m.ctx.At(endBeat, func() { m.ctx.Scene.SetActive(m.displayHolder, false) })
	if call {
		m.ctx.At(endBeat+1, func() { m.ctx.Scene.PlayState(m.spot, "Hide", endBeat+1, 0.5) })
	}

	for i := 0; i < count; i++ {
		idx := i
		spawnBeat := beat + 1 + float64(idx)/noteStepDiv(typ)
		clip := noteClip(idx, typ)
		m.ctx.SoundAt(spawnBeat, noteSound(idx, typ), 1)
		m.ctx.At(spawnBeat, func() {
			m.ctx.Scene.PlayState(m.lightsL, "Flash", spawnBeat, 0.5)
			m.ctx.Scene.PlayState(m.lightsR, "Flash", spawnBeat, 0.5)
		})
		if m.noteT == nil {
			continue
		}
		clipBeat := spawnBeat
		if disappear {
			clipBeat = beat
		}
		inst := m.noteT.NewInstance()
		// The serialized Note template has an editor position, but RhythmFighter
		// overwrites transform.position immediately after Instantiate.
		inst.Offset = [2]float64{0, 0}
		inst.PlayState("", clip, clipBeat, 1)
		m.notes = append(m.notes, &noteInst{
			inst:        inst,
			endBeat:     endBeat,
			toggleBeat:  spawnBeat,
			visibleInit: disappear,
			x:           -noteX + noteDistance*float64(idx) + holderX,
			y:           holderY,
		})
	}
}

func (m *Module) neededCalls(beat, length float64) []callEvt {
	out := make([]callEvt, 0)
	for _, call := range m.calls {
		if call.beat >= beat-1e-6 && call.beat <= beat+length+1e-6 {
			out = append(out, call)
		}
	}
	return out
}

func (m *Module) lastStartBefore(beat float64) (startIntervalEvt, bool) {
	for i := len(m.starts) - 1; i >= 0; i-- {
		ev := m.starts[i]
		if ev.beat+ev.length <= beat+1e-6 {
			return ev, true
		}
	}
	return startIntervalEvt{}, false
}

func (m *Module) inCallIntervalAt(beat float64) bool {
	for _, ev := range m.starts {
		if beat >= ev.beat-1 && beat < ev.beat-1+ev.length {
			return true
		}
	}
	return false
}

func (m *Module) inCloseRangeAt(beat float64) bool {
	close := false
	for _, ev := range m.passes {
		if beat >= ev.beat && beat < nextFallAfter(m.falls, ev.beat) {
			close = true
		}
	}
	for _, ev := range m.starts {
		passBeat := ev.beat - 1 + autoPassOffset(ev.length)
		if ev.autoPass && beat >= passBeat && beat < nextFallAfter(m.falls, passBeat) {
			close = true
		}
	}
	return close
}

func (m *Module) canDodgeAt(beat float64) bool {
	for _, ev := range m.passes {
		start, end := ev.beat+4, ev.beat+4+m.lastStartLengthBefore(ev.beat)
		if beat >= start && beat < end {
			return true
		}
	}
	for _, ev := range m.starts {
		if !ev.autoPass {
			continue
		}
		passBeat := ev.beat - 1 + autoPassOffset(ev.length)
		if beat >= passBeat+4 && beat < passBeat+4+ev.length {
			return true
		}
	}
	return false
}

func (m *Module) lastStartLengthBefore(beat float64) float64 {
	if ev, ok := m.lastStartBefore(beat); ok {
		return ev.length
	}
	return 0
}

func (m *Module) restoreDisplayAt(beat float64) {
	for _, w := range m.displayWindows {
		if beat >= w.start && beat < w.end {
			m.ctx.Scene.SetActive(m.displayHolder, true)
			m.ctx.Scene.SetPosOver(m.displayHolder, w.x, w.y)
			m.ctx.Scene.PlayStateLayer("rhythmFighter/display/base", m.displayHolder, w.baseState, w.start, 0.5)
			m.ctx.Scene.PlayStateLayer("rhythmFighter/display/length", m.displayHolder, w.lengthState, w.start, 0.5)
			if w.call {
				m.ctx.Scene.PlayState(m.spot, "Show", w.start, 0.5)
			}
			return
		}
	}
}

func autoPassOffset(length float64) float64 {
	v := 0.0
	for i := 1.0; i < length+4; i *= 2 {
		v = i * 2
	}
	return v - 3
}

func displayStepMatches(callBeat, baseBeat float64, typ int, ignore bool) bool {
	if ignore {
		return true
	}
	step := 1.0
	if typ == displayQuick {
		step = 0.5
	}
	diff := callBeat - baseBeat
	return math.Abs(diff/step-math.Round(diff/step)) < 1e-6
}

func noteStepDiv(typ int) float64 {
	if typ == displayQuick {
		return 2
	}
	return 1
}

func noteClip(idx, typ int) string {
	period := 4
	if typ == displayQuick {
		period = 8
	}
	if idx%period == 0 {
		if typ == displayQuick {
			return "EightO"
		}
		return "QuarterO"
	}
	if typ == displayQuick {
		return "EightW"
	}
	return "QuarterW"
}

func noteSound(idx, typ int) string {
	period := 4
	if typ == displayQuick {
		period = 8
	}
	if idx%period == 0 {
		return "ding_first"
	}
	return "ding"
}

func nextFallAfter(falls []float64, beat float64) float64 {
	for _, f := range falls {
		if f >= beat {
			return f
		}
	}
	return math.Inf(1)
}

func roleOr(ctx *engine.Ctx, key, fallback string) string {
	if v := ctx.Role(key); v != "" {
		return v
	}
	return fallback
}

func intParam(e *riq.Entity, key string, def int) int { return int(e.Float(key, float64(def))) }

func boolParam(e *riq.Entity, key string) bool { return boolParamDefault(e, key, false) }

func boolParamDefault(e *riq.Entity, key string, def bool) bool {
	if _, ok := e.Data[key]; !ok {
		return def
	}
	return e.Float(key, 0) != 0
}
