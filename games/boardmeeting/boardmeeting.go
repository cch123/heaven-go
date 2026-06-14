// Package boardmeeting ports Board Meeting's dynamic executive row, chair
// spin/stop cues, assistant count-in, bop regions, and looped chair audio.
package boardmeeting

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
	defaultExecutiveCount = 4
	assistantLocalOffset  = 5.359
)

var bgColor = color.NRGBA{R: 0xd3, G: 0x79, B: 0x12, A: 0xff}

type bopEvt struct {
	beat, length float64
	bop, auto    bool
}

type spinEquiEvt struct {
	beat, length float64
}

type spinRangeEvt struct {
	beat       float64
	start, end int
}

type stopEvt struct {
	beat, length float64
	assistant    bool
}

type countEvt struct {
	beat  float64
	count int
}

type executive struct {
	inst *kart.Instance
	mod  *Module

	player     bool
	canBop     bool
	spinning   bool
	preparing  bool
	smileCount int
	stopRoll   func()
	spinStart  float64
	loopStart  float64
	spinTS     float64
}

type Module struct {
	ctx  *engine.Ctx
	proj kart.Aff

	bops   []bopEvt
	equis  []spinEquiEvt
	rangs  []spinRangeEvt
	stops  []stopEvt
	preps  []float64
	counts []countEvt

	assistantPath string
	farLeftPath   string
	farRightPath  string
	execRoot      string

	farLeftX, farRightX float64
	execCount           int
	execT               *kart.Template
	execs               []*executive
	firstSpinner        *executive

	assistantCanBop bool
	execsCanBop     bool
	missCounter     int
	stopChair       func()

	spinDuration float64
	shakeStart   float64
	shakeEnd     float64
	endBeat      float64
}

func New() engine.Module {
	return &Module{
		execCount:       defaultExecutiveCount,
		assistantCanBop: true,
		execsCanBop:     true,
		spinDuration:    0.21666667,
	}
}

func (m *Module) ID() string { return "boardMeeting" }

func (m *Module) Load(ctx *engine.Ctx) error {
	m.ctx = ctx
	if err := ctx.LoadAssets("boardMeeting"); err != nil {
		return err
	}
	m.proj = kart.Translate(engine.ScreenW/2, engine.ScreenH/2).Mul(kart.Scale(54, -54))
	m.assistantPath = roleOr(ctx, "assistantAnim", "bm_assistant")
	m.farLeftPath = roleOr(ctx, "farLeft", "bm_farLeftPos")
	m.farRightPath = roleOr(ctx, "farRight", "bm_farRightPos")
	m.execRoot = "bm_executive"
	m.farLeftX = nodeX(ctx, m.farLeftPath)
	m.farRightX = nodeX(ctx, m.farRightPath)
	if anim, ok := ctx.Assets.Anims["Executive/Spin"]; ok && anim.Duration > 0 {
		m.spinDuration = anim.Duration
	}
	m.execT = kart.NewTemplate(ctx.Assets, m.execRoot)
	ctx.Scene.SetActive(m.execRoot, false)
	ctx.Scene.PlayDefaultState(m.assistantPath, 0, ctx.SecPerBeat(0))
	m.setExecutiveCount(defaultExecutiveCount, 0)
	return nil
}

func roleOr(ctx *engine.Ctx, key, fallback string) string {
	if p := ctx.Role(key); p != "" {
		return p
	}
	return fallback
}

func nodeX(ctx *engine.Ctx, path string) float64 {
	for _, n := range ctx.Assets.Rig.Nodes {
		if n.Path == path {
			return n.Pos[0]
		}
	}
	return 0
}

func (m *Module) OnEvent(e *riq.Entity) {
	if end := e.Beat + e.Length; end > m.endBeat {
		m.endBeat = end
	}
	switch e.Datamodel {
	case "boardMeeting/bop":
		m.bops = append(m.bops, bopEvt{
			beat: e.Beat, length: e.Length,
			bop:  boolDefault(e, "bop", true),
			auto: boolParam(e, "auto"),
		})
	case "boardMeeting/prepare":
		m.preps = append(m.preps, e.Beat)
	case "boardMeeting/spinEqui":
		m.equis = append(m.equis, spinEquiEvt{beat: e.Beat, length: e.Length})
	case "boardMeeting/spin":
		m.rangs = append(m.rangs, spinRangeEvt{
			beat:  e.Beat,
			start: int(e.Float("start", 1)),
			end:   int(e.Float("end", 4)),
		})
	case "boardMeeting/stop":
		m.stops = append(m.stops, stopEvt{beat: e.Beat, length: e.Length})
	case "boardMeeting/assStop":
		m.stops = append(m.stops, stopEvt{beat: e.Beat, length: 2, assistant: true})
	case "boardMeeting/changeCount":
		m.counts = append(m.counts, countEvt{beat: e.Beat, count: int(e.Float("amount", defaultExecutiveCount))})
	}
}

func (m *Module) Ready() {
	sort.SliceStable(m.bops, func(i, j int) bool { return m.bops[i].beat < m.bops[j].beat })
	sort.SliceStable(m.equis, func(i, j int) bool { return m.equis[i].beat < m.equis[j].beat })
	sort.SliceStable(m.rangs, func(i, j int) bool { return m.rangs[i].beat < m.rangs[j].beat })
	sort.SliceStable(m.stops, func(i, j int) bool { return m.stops[i].beat < m.stops[j].beat })
	sort.Float64s(m.preps)
	sort.SliceStable(m.counts, func(i, j int) bool { return m.counts[i].beat < m.counts[j].beat })

	for _, ev := range m.counts {
		ev := ev
		m.ctx.At(ev.beat, func() { m.setExecutiveCount(ev.count, ev.beat) })
	}
	for _, b := range m.preps {
		beat := b
		m.ctx.At(beat, func() { m.prepare(beat) })
	}
	for _, ev := range m.equis {
		ev := ev
		m.scheduleSpinEqui(ev)
	}
	for _, ev := range m.rangs {
		ev := ev
		m.ctx.At(ev.beat, func() { m.spinRange(ev.beat, ev.start, ev.end) })
	}
	for _, ev := range m.stops {
		ev := ev
		if ev.assistant {
			m.scheduleAssistantStop(ev.beat)
		} else {
			m.scheduleStop(ev)
		}
	}
	m.scheduleBops()
}

func (m *Module) OnSwitch(beat float64) {
	m.ctx.Scene.PlayDefaultState(m.assistantPath, beat, m.ctx.SecPerBeat(beat))
	m.setExecutiveCount(m.countAt(beat), beat)
}

func (m *Module) Whiff(beat float64) {
	p := m.player()
	if p == nil || !p.spinning {
		return
	}
	p.stop(false, beat)
	m.ctx.Sound("miss")
	m.ctx.PlayCommon("miss")
	m.ctx.ScoreMiss()
}

func (m *Module) Update(_, _ float64) {}

func (m *Module) Draw(screen *ebiten.Image, _ float64, beat float64) {
	screen.Fill(bgColor)
	m.ctx.SampleScene(beat)
	proj := m.proj
	if beat < m.shakeEnd {
		u := clamp01((beat - m.shakeStart) / math.Max(0.001, m.shakeEnd-m.shakeStart))
		shake := math.Sin(u*math.Pi*18) * (1 - u) * 0.5 * 54
		proj = kart.Translate(engine.ScreenW/2+shake, engine.ScreenH/2).Mul(kart.Scale(54, -54))
	}
	for _, ex := range m.execs {
		if ex.inst != nil {
			ex.inst.Queue(m.ctx.Scene, beat, kart.Identity(), 0)
		}
	}
	m.ctx.Scene.Draw(screen, proj)
}

func (m *Module) scheduleBops() {
	scheduled := map[float64]bool{}
	add := func(beat float64) {
		if scheduled[beat] {
			return
		}
		scheduled[beat] = true
		m.ctx.At(beat, func() { m.singleBop() })
	}
	for i, ev := range m.bops {
		if ev.bop {
			for k := 0; k < int(ev.length); k++ {
				add(ev.beat + float64(k))
			}
		}
		if !ev.auto {
			continue
		}
		end := math.Max(ev.beat+ev.length, m.endBeat+4)
		if sw := m.ctx.NextSwitchBeat(ev.beat); !math.IsInf(sw, 1) && sw < end {
			end = sw
		}
		for j := i + 1; j < len(m.bops); j++ {
			if m.bops[j].beat > ev.beat && m.bops[j].beat < end {
				end = m.bops[j].beat
				break
			}
		}
		for b := ev.beat; b < end-1e-6; b++ {
			add(b)
		}
	}
}

func (m *Module) singleBop() {
	if m.assistantCanBop {
		if m.missCounter > 0 {
			m.playAssistant("MissBop", m.ctx.Beat())
		} else {
			m.playAssistant("Bop", m.ctx.Beat())
		}
	}
	if m.missCounter > 0 {
		m.missCounter--
	}
	if !m.execsCanBop {
		return
	}
	for _, ex := range m.execs {
		ex.bop(m.ctx.Beat())
	}
}

func (m *Module) prepare(beat float64) {
	m.ctx.Sound("prepare")
	for _, ex := range m.execs {
		ex.prepare(beat)
	}
}

func (m *Module) scheduleSpinEqui(ev spinEquiEvt) {
	m.ctx.At(ev.beat, func() {
		m.startChairLoop()
		if len(m.execs) > 0 {
			m.firstSpinner = m.execs[0]
		}
		for i := 0; i < m.execCount; i++ {
			i := i
			b := ev.beat + ev.length*float64(i)
			m.ctx.At(b, func() {
				if i >= len(m.execs) {
					return
				}
				m.execs[i].spin(m.spinSoundName(i), i == 0, b)
			})
		}
	})
}

func (m *Module) spinRange(beat float64, start, end int) {
	if start > m.execCount || end > m.execCount || start < 1 || end < start {
		return
	}
	forceStart := false
	if m.stopChair != nil {
		m.stopChair()
	}
	if m.stopChair == nil {
		m.startChairLoop()
		m.firstSpinner = m.execs[start-1]
		forceStart = true
	}
	for i := start - 1; i < end && i < len(m.execs); i++ {
		m.execs[i].spin(m.spinSoundName(i), forceStart, beat)
	}
}

func (m *Module) scheduleStop(ev stopEvt) {
	m.ctx.At(ev.beat, func() {
		m.execsCanBop = false
		count := m.execCount
		for i := 0; i < count; i++ {
			if i >= len(m.execs) || m.execs[i].player {
				break
			}
			i := i
			b := ev.beat + ev.length*float64(i)
			m.ctx.SoundAt(b, m.stopSoundName(i), 1)
			m.ctx.At(b, func() { m.execs[i].stop(true, b) })
		}
		m.ctx.At(ev.beat+ev.length*float64(count)+0.5, func() { m.execsCanBop = true })
		target := ev.beat + ev.length*float64(count-1)
		m.ctx.ScheduleInputCond(target, m.playerSpinning,
			func(state float64, _ engine.Judgment) { m.justStop(target, state) },
			func() { m.missStop(target) })
	})
}

func (m *Module) scheduleAssistantStop(beat float64) {
	m.ctx.At(beat, func() {
		m.assistantCanBop = false
		m.playAssistant("One", beat)
	})
	two := "two"
	if math.Abs(beat-math.Round(beat)) > 1e-6 {
		two = "twoUra"
	}
	m.ctx.SoundAt(beat, "one", 1)
	m.ctx.SoundAt(beat+0.5, two, 1)
	m.ctx.SoundAt(beat+1, "three", 1)
	m.ctx.SoundAt(beat+2, "stopAll", 1)
	m.ctx.At(beat+1, func() { m.playAssistant("Three", beat+1) })
	m.ctx.At(beat+2, func() {
		for _, ex := range m.execs {
			if !ex.player {
				ex.stop(true, beat+2)
			}
		}
		if !m.playerSpinning() {
			m.stopChairLoop()
		}
	})
	m.ctx.At(beat+2.5, func() { m.assistantCanBop = true })
	m.ctx.ScheduleInputCond(beat+2, m.playerSpinning,
		func(state float64, _ engine.Judgment) { m.justAssistant(beat+2, state) },
		func() { m.missAssistant(beat + 2) })
}

func (m *Module) justStop(beat, state float64) {
	p := m.player()
	if p == nil || !p.spinning {
		return
	}
	m.stopChairLoop()
	if state >= 1 || state <= -1 {
		m.ctx.Sound("missThrough")
		m.ctx.PlayCommon("miss")
		p.stop(false, beat)
		return
	}
	m.ctx.Sound("stopPlayer")
	p.stop(true, beat)
	m.ctx.At(beat+1, func() { m.smileAll(beat + 1) })
}

func (m *Module) justAssistant(beat, state float64) {
	p := m.player()
	if p == nil || !p.spinning {
		return
	}
	m.stopChairLoop()
	if state >= 1 || state <= -1 {
		m.ctx.Sound("missThrough")
		m.ctx.PlayCommon("miss")
		p.stop(false, beat)
		return
	}
	p.stop(true, beat)
	m.playAssistant("Stop", beat)
	m.ctx.Sound("stopAllPlayer")
	m.shakeStart, m.shakeEnd = beat, beat+0.5
	m.ctx.At(beat+1, func() { m.smileAll(beat + 1) })
}

func (m *Module) missStop(beat float64) {
	p := m.player()
	if p == nil || !p.spinning {
		return
	}
	p.stop(false, beat)
	m.ctx.Sound("missThrough")
	m.ctx.PlayCommon("miss")
	m.stopChairLoop()
}

func (m *Module) missAssistant(beat float64) {
	m.missStop(beat)
	m.playAssistant("MissIdle", beat)
	m.missCounter = 2
}

func (m *Module) setExecutiveCount(count int, beat float64) {
	if count < 3 {
		count = 3
	}
	if count > 5 {
		count = 5
	}
	for _, ex := range m.execs {
		ex.killLoops()
	}
	m.execCount = count
	m.execs = m.execs[:0]
	if m.execT == nil {
		return
	}
	between := math.Abs(m.farLeftX-m.farRightX) / 3
	start := -(between * float64(count) / 2)
	m.ctx.Scene.SetPosOver(m.assistantPath, start+assistantLocalOffset, 0)
	for i := 0; i < count; i++ {
		in := m.execT.NewInstance()
		in.Offset = [2]float64{start + between*float64(i+1), 0}
		in.PlayDefaultState("", beat, m.ctx.SecPerBeat(beat))
		applyExecutiveOrder(in, i)
		ex := &executive{inst: in, mod: m, canBop: true, player: i == count-1}
		m.execs = append(m.execs, ex)
	}
	m.firstSpinner = nil
	m.stopChairLoop()
}

func applyExecutiveOrder(in *kart.Instance, i int) {
	off := i * 10
	for rel, base := range map[string]int{
		"bm_shadow":             -3,
		"bm_chair/bm_chairlegs": -2,
		"bm_chair/bm_seat":      -1,
		"bm_exbody":             0,
		"bm_arm":                0,
		"bm_impact":             0,
		"bm_star":               0,
		"bm_star (1)":           0,
		"bm_star (2)":           0,
		"bm_star (3)":           0,
		"bm_star (4)":           0,
		"bm_star (5)":           0,
		"bm_star (6)":           0,
		"bm_star (7)":           0,
		"bm_exhead":             1,
	} {
		in.SetOrder(rel, base+off)
	}
}

func (m *Module) countAt(beat float64) int {
	count := defaultExecutiveCount
	for _, ev := range m.counts {
		if ev.beat > beat {
			break
		}
		count = ev.count
	}
	return count
}

func (m *Module) spinSoundName(index int) string {
	ex := m.execCount
	if ex < 4 {
		ex = 4
	}
	switch index {
	case ex - 3:
		return "B"
	case ex - 2:
		return "C"
	case ex - 1:
		return "Player"
	default:
		return "A"
	}
}

func (m *Module) stopSoundName(index int) string {
	ex := m.execCount
	if ex < 4 {
		ex = 4
	}
	switch index {
	case ex - 3:
		return "stopB"
	case ex - 2:
		return "stopC"
	default:
		return "stopA"
	}
}

func (m *Module) player() *executive {
	if len(m.execs) == 0 {
		return nil
	}
	return m.execs[len(m.execs)-1]
}

func (m *Module) playerSpinning() bool {
	p := m.player()
	return p != nil && p.spinning
}

func (m *Module) smileAll(beat float64) {
	for _, ex := range m.execs {
		ex.smile(beat)
	}
}

func (m *Module) playAssistant(state string, beat float64) {
	m.ctx.Scene.PlayState(m.assistantPath, state, beat, 0.5)
}

func (m *Module) startChairLoop() {
	if m.stopChair != nil {
		return
	}
	m.stopChair = m.ctx.SoundLoop("chairLoop")
}

func (m *Module) stopChairLoop() {
	if m.stopChair != nil {
		m.stopChair()
		m.stopChair = nil
	}
}

func (m *Module) stopChairLoopIfLastToStop() {
	spinning := 0
	for _, ex := range m.execs {
		if ex.spinning {
			spinning++
		}
	}
	if spinning > 1 {
		return
	}
	m.stopChairLoop()
}

func (ex *executive) prepare(beat float64) {
	if ex.spinning {
		return
	}
	ex.preparing = true
	ex.canBop = false
	ex.inst.PlayState("", "Prepare", beat, 0.5)
}

func (ex *executive) spin(soundToPlay string, forceStart bool, beat float64) {
	if ex.spinning {
		return
	}
	m := ex.mod
	ex.spinning = true
	ex.preparing = false
	ex.canBop = false
	ts := m.ctx.SecPerBeat(beat)
	if m.firstSpinner == nil || ex == m.firstSpinner || forceStart {
		ex.spinStart = beat
		ex.spinTS = ts
		ex.loopStart = beat + m.spinDuration/ts
		ex.inst.PlayState("", "Spin", beat, ts)
	} else {
		state, start, firstTS := m.firstSpinner.spinStateAt(beat)
		ex.spinStart = m.firstSpinner.spinStart
		ex.loopStart = m.firstSpinner.loopStart
		ex.spinTS = firstTS
		ex.inst.PlayState("", state, start, firstTS)
	}
	m.ctx.Sound("rollPrepare" + soundToPlay)
	ex.startRollLoop(soundToPlay, beat)
}

func (ex *executive) spinStateAt(beat float64) (state string, startBeat, timeScale float64) {
	if ex.spinTS <= 0 {
		return "Spin", beat, ex.mod.ctx.SecPerBeat(beat)
	}
	if beat < ex.loopStart {
		return "Spin", ex.spinStart, ex.spinTS
	}
	return "LoopSpin", ex.loopStart, ex.spinTS
}

func (ex *executive) startRollLoop(suffix string, beat float64) {
	offsetSec := 0.0
	switch suffix {
	case "A", "B":
		offsetSec = 0.01041666666
	case "C", "Player":
		offsetSec = 0.02083333333
	}
	start := beat + 0.5 - offsetSec/ex.mod.ctx.SecPerBeat(beat)
	ex.mod.ctx.At(start, func() {
		if !ex.spinning || ex.stopRoll != nil {
			return
		}
		ex.stopRoll = ex.mod.ctx.SoundLoop("roll" + suffix)
	})
}

func (ex *executive) stop(hit bool, beat float64) {
	if !ex.spinning {
		return
	}
	ex.spinning = false
	state, ts := "Miss", 0.25
	if hit {
		state, ts = "Stop", 0.5
	}
	ex.inst.PlayState("", state, beat, ts)
	ex.stopRollLoop()
	ex.mod.stopChairLoopIfLastToStop()
	ex.mod.ctx.At(beat+1.5, func() { ex.canBop = true })
}

func (ex *executive) bop(beat float64) {
	if !ex.canBop || ex.spinning || ex.preparing {
		return
	}
	if ex.smileCount > 0 {
		ex.inst.PlayState("", "SmileBop", beat, 0.5)
		ex.smileCount--
		return
	}
	ex.inst.PlayState("", "Bop", beat, 0.5)
}

func (ex *executive) smile(beat float64) {
	if ex.spinning {
		return
	}
	if !ex.preparing {
		ex.inst.PlayState("", "SmileIdle", beat, 0.5)
	}
	ex.smileCount = 2
}

func (ex *executive) killLoops() {
	ex.stopRollLoop()
}

func (ex *executive) stopRollLoop() {
	if ex.stopRoll != nil {
		ex.stopRoll()
		ex.stopRoll = nil
	}
}

func boolParam(e *riq.Entity, key string) bool { return e.Float(key, 0) != 0 }

func boolDefault(e *riq.Entity, key string, def bool) bool {
	if _, ok := e.Data[key]; !ok {
		return def
	}
	return boolParam(e, key)
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
