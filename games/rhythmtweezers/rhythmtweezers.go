// Package rhythmtweezers ports Rhythm Tweezers' call/response intervals,
// vegetable swaps, no-peeking sign, short hairs, and curly-hair hold/release
// behaviour from Heaven Studio's RhythmTweezers C# implementation.
package rhythmtweezers

import (
	"fmt"
	"image/color"
	"math"
	"math/rand"
	"sort"

	"github.com/hajimehoshi/ebiten/v2"

	"hsdemo/engine"
	"hsdemo/kart"
	"hsdemo/riq"
)

const (
	vegOnion = iota
	vegPotato
	vegBeet
	vegPumpkin
	vegRadish
	vegWatermelon
	vegLightbulb
	vegCount
)

const (
	noPeekFull = iota
	noPeekHalfRight
	noPeekHalfLeft
)

const (
	smileDefault = iota
	smileTengoku
	smileAlt
)

const (
	vegDupeOffset = 16.7
	hairArcDeg    = 116
	hairArcStart  = -58
)

var (
	defaultBgColor      = [4]float64{0.631, 0.31, 0.631, 1}
	defaultVeggieColors = [vegCount][4]float64{
		{0.7843137254901961, 0.588235294117, 0, 1},
		{1, 0.8627450980392157, 0, 1},
		{1, 0.5411765, 0.9607843, 1},
		{1, 0.454902, 0.2392157, 1},
		{1, 0.9215686, 0.7215686, 1},
		{0.1294118, 1, 0, 1},
		{0.6179246, 0.7467621, 1, 0.5},
	}
)

type intervalEvt struct {
	idx            int
	beat, length   float64
	autoPassTurn   bool
	autoSwapVeggie bool
	vegType        int
	uniqueColor    bool
	color          [4]float64
	legacyNoSwap   bool
}

type hairEvt struct {
	idx       int
	beat      float64
	long      bool
	interval  int
	relative  float64
	judgeBeat float64
}

type passEvt struct {
	beat float64
}

type vegEvt struct {
	beat        float64
	vegType     int
	uniqueColor bool
	color       [4]float64
	instant     bool
}

type bgEvt struct {
	beat, length float64
	from, to     [4]float64
	ease         int
}

type veggieColorEvt struct {
	beat   float64
	colors [vegCount][4]float64
}

type smileEvt struct {
	beat float64
	typ  int
}

type noPeekEvt struct {
	beat, length float64
	typ          int
}

type vegTransition struct {
	beat, length float64
	toType       int
	toColor      [4]float64
}

type hairInst struct {
	event hairEvt
	inst  *kart.Instance
	rot   float64

	hit     bool
	missed  bool
	pulling bool
	done    bool

	pullBeat float64
	pullEnd  float64
	stopLoop func()
}

type passState struct {
	beat, endBeat float64
	inst          *kart.Instance
	hairs         []*hairInst
	hairsLeft     int
	eyeSize       int
	holding       bool
	heldLong      bool
	dead          bool
}

type droppedHair struct {
	born      float64
	x, y      float64
	vx, vy    float64
	rot, spin float64
	sprite    string
}

type noPeekState struct {
	event noPeekEvt
	inst  *kart.Instance
}

type Module struct {
	ctx  *engine.Ctx
	proj kart.Aff

	hairT    *kart.Template
	longT    *kart.Template
	tweezerT *kart.Template
	noPeekT  *kart.Template

	intervals   []intervalEvt
	hairEvents  []hairEvt
	passEvents  []passEvt
	vegEvents   []vegEvt
	bgEvents    []bgEvt
	colorEvents []veggieColorEvt
	smileEvents []smileEvt
	peekEvents  []noPeekEvt

	startedIntervals map[int]bool
	startedPasses    map[float64]bool
	spawnedByEvent   map[int]*hairInst

	hairs       []*hairInst
	passes      []*passState
	activePass  *passState
	dropped     []droppedHair
	noPeekSigns []*noPeekState

	veggieColors [vegCount][4]float64
	currentType  int
	currentColor [4]float64
	firstVeggie  bool
	transition   *vegTransition
}

func New() engine.Module { return &Module{} }

func (m *Module) ID() string { return "rhythmTweezers" }

func (m *Module) Load(ctx *engine.Ctx) error {
	m.ctx = ctx
	if err := ctx.LoadAssets("rhythmTweezers"); err != nil {
		return err
	}
	m.proj = kart.Translate(engine.ScreenW/2, engine.ScreenH/2).Mul(kart.Scale(54, -54))
	m.hairT = kart.NewTemplate(ctx.Assets, "VegetableHolder/Vegetable/HairPrefabs/HairHolder")
	m.longT = kart.NewTemplate(ctx.Assets, "VegetableHolder/Vegetable/HairPrefabs/LongHairHolder")
	m.tweezerT = kart.NewTemplate(ctx.Assets, "TweezerHolder")
	m.noPeekT = kart.NewTemplate(ctx.Assets, "noPeek_2")

	m.veggieColors = defaultVeggieColors
	m.currentType = vegOnion
	m.currentColor = m.veggieColors[vegOnion]
	m.firstVeggie = true
	m.startedIntervals = map[int]bool{}
	m.startedPasses = map[float64]bool{}
	m.spawnedByEvent = map[int]*hairInst{}

	ctx.Scene.SetActive("VegetableHolder/Vegetable/HairPrefabs", false)
	ctx.Scene.SetActive("VegetableHolder/VegetableDuplicate", false)
	ctx.Scene.SetActive("VegetableHolder/VegetableDuplicate/LightBulb", false)
	ctx.Scene.SetActive("TweezerHolder", false)
	ctx.Scene.SetActive("noPeek_2", false)
	m.applyVegetable(vegOnion, m.currentColor)
	return nil
}

func (m *Module) OnEvent(e *riq.Entity) {
	switch e.Datamodel {
	case "rhythmTweezers/simple interval", "rhythmTweezers/start interval":
		length := e.Length
		if length <= 0 {
			length = 4
		}
		legacyNoSwap := false
		if _, ok := e.Data["autoSwap"]; !ok {
			// Heaven Studio upgraded v0 interval entities by adding autoSwap=false.
			legacyNoSwap = true
		}
		m.intervals = append(m.intervals, intervalEvt{
			idx: len(m.intervals), beat: e.Beat, length: length,
			autoPassTurn:   boolDefault(e, "auto", true),
			autoSwapVeggie: boolDefault(e, "autoSwap", true),
			vegType:        clampVeg(int(e.Float("type", 0))),
			uniqueColor:    boolParam(e, "uniqueColor"),
			color:          colorParam(e, "colorA", defaultVeggieColors[clampVeg(int(e.Float("type", 0)))]),
			legacyNoSwap:   legacyNoSwap,
		})
	case "rhythmTweezers/short hair", "rhythmTweezers/long hair":
		m.hairEvents = append(m.hairEvents, hairEvt{
			idx: len(m.hairEvents), beat: e.Beat, long: e.Datamodel == "rhythmTweezers/long hair",
		})
	case "rhythmTweezers/passTurn":
		m.passEvents = append(m.passEvents, passEvt{beat: e.Beat})
	case "rhythmTweezers/next vegetable":
		typ := clampVeg(int(e.Float("type", 0)))
		m.vegEvents = append(m.vegEvents, vegEvt{
			beat: e.Beat, vegType: typ, uniqueColor: boolParam(e, "uniqueColor"),
			color: colorParam(e, "colorA", defaultVeggieColors[typ]), instant: boolParam(e, "instant"),
		})
	case "rhythmTweezers/change vegetable":
		typ := clampVeg(int(e.Float("type", 0)))
		m.vegEvents = append(m.vegEvents, vegEvt{
			beat: e.Beat, vegType: typ, uniqueColor: boolParam(e, "uniqueColor"),
			color: colorParam(e, "colorA", defaultVeggieColors[typ]), instant: true,
		})
	case "rhythmTweezers/noPeek":
		length := e.Length
		if length <= 0 {
			length = 4
		}
		m.peekEvents = append(m.peekEvents, noPeekEvt{
			beat: e.Beat, length: length, typ: int(e.Float("type", noPeekFull)),
		})
	case "rhythmTweezers/fade background color":
		m.bgEvents = append(m.bgEvents, bgEvt{
			beat: e.Beat, length: e.Length,
			from: colorParam(e, "colorA", [4]float64{1, 1, 1, 1}),
			to:   colorParam(e, "colorB", defaultBgColor),
			ease: int(e.Float("ease", 0)),
		})
	case "rhythmTweezers/set veggie color":
		ev := veggieColorEvt{beat: e.Beat, colors: defaultVeggieColors}
		for i, key := range []string{"colorA", "colorB", "colorC", "colorD", "colorE", "colorF", "colorG"} {
			ev.colors[i] = colorParam(e, key, defaultVeggieColors[i])
		}
		m.colorEvents = append(m.colorEvents, ev)
	case "rhythmTweezers/altSmile":
		m.smileEvents = append(m.smileEvents, smileEvt{beat: e.Beat, typ: int(e.Float("type", smileDefault))})
	}
}

func (m *Module) Ready() {
	sort.SliceStable(m.intervals, func(i, j int) bool { return m.intervals[i].beat < m.intervals[j].beat })
	sort.SliceStable(m.hairEvents, func(i, j int) bool { return m.hairEvents[i].beat < m.hairEvents[j].beat })
	sort.SliceStable(m.passEvents, func(i, j int) bool { return m.passEvents[i].beat < m.passEvents[j].beat })
	sort.SliceStable(m.vegEvents, func(i, j int) bool { return m.vegEvents[i].beat < m.vegEvents[j].beat })
	sort.SliceStable(m.bgEvents, func(i, j int) bool { return m.bgEvents[i].beat < m.bgEvents[j].beat })
	sort.SliceStable(m.colorEvents, func(i, j int) bool { return m.colorEvents[i].beat < m.colorEvents[j].beat })
	sort.SliceStable(m.smileEvents, func(i, j int) bool { return m.smileEvents[i].beat < m.smileEvents[j].beat })
	sort.SliceStable(m.peekEvents, func(i, j int) bool { return m.peekEvents[i].beat < m.peekEvents[j].beat })

	for i := range m.colorEvents {
		ev := m.colorEvents[i]
		m.ctx.At(ev.beat, func() {
			m.veggieColors = ev.colors
			if m.transition == nil {
				m.currentColor = m.veggieColor(m.currentType, false, m.currentColor)
				m.applyVegetable(m.currentType, m.currentColor)
			}
		})
	}
	for i := range m.smileEvents {
		ev := m.smileEvents[i]
		m.ctx.At(ev.beat, func() { m.setSmile(ev.typ) })
	}
	for i := range m.vegEvents {
		ev := m.vegEvents[i]
		m.ctx.At(ev.beat, func() {
			if ev.instant {
				m.changeVegetableImmediate(ev.vegType, ev.uniqueColor, ev.color)
			} else {
				m.nextVegetable(ev.beat, ev.vegType, ev.uniqueColor, ev.color)
			}
		})
	}
	for i := range m.peekEvents {
		ev := m.peekEvents[i]
		m.ctx.At(ev.beat-1, func() {
			if m.ctx.GameAt(ev.beat) == m.ID() {
				m.spawnNoPeek(ev)
			}
		})
	}
	for i := range m.intervals {
		ev := m.intervals[i]
		m.ctx.At(ev.beat, func() {
			if m.ctx.GameAt(ev.beat) == m.ID() {
				m.startInterval(ev, ev.beat)
			}
		})
	}
	for _, ev := range m.passEvents {
		ev := ev
		m.ctx.At(ev.beat, func() {
			if m.ctx.GameAt(ev.beat) == m.ID() {
				m.passTurnStandalone(ev.beat)
			}
		})
	}
}

func (m *Module) OnSwitch(beat float64) {
	m.restorePersistentState(beat)
	m.ctx.Scene.PlayDefaultState("VegetableHolder/Vegetable", beat, m.ctx.SecPerBeat(beat))
	for _, ev := range m.intervals {
		if ev.beat <= beat && beat < ev.beat+ev.length+0.25 {
			m.startInterval(ev, beat)
		}
	}
	for _, ev := range m.peekEvents {
		if beat >= ev.beat-1 && beat <= ev.beat+ev.length+1 {
			m.spawnNoPeek(ev)
		}
	}
}

func (m *Module) Whiff(beat float64) {
	if m.activePass == nil || m.activePass.inst == nil {
		return
	}
	m.dropHeldHair(m.activePass, beat)
	m.activePass.inst.PlayState("", "Tweezers_Pluck", beat, m.ctx.SecPerBeat(beat))
	m.ctx.Sound(fmt.Sprintf("click%d", 1+rand.Intn(6)))
}

func (m *Module) Update(_, beat float64) {
	for _, h := range m.hairs {
		if !h.pulling || h.done {
			continue
		}
		u := clamp01((beat - h.pullBeat) / 0.5)
		h.inst.PlayNormalized("", "Hairs/LoopPull", u)
		if m.activePass != nil && m.activePass.inst != nil {
			m.activePass.inst.PlayNormalized("", "Tweezers/Tweezers_LongPluck", u)
		}
		if m.ctx.ReleasedNow() && !m.ctx.ExpectingReleaseNow() && u < 1 {
			m.endLongEarly(h, beat)
			continue
		}
		if u >= 1 {
			m.ctx.AutoHitRelease(h.pullEnd)
		}
	}
	m.pruneDead(beat)
}

func (m *Module) Draw(screen *ebiten.Image, t, beat float64) {
	screen.Fill(toNRGBA(m.bgAt(beat)))
	m.updateVegetableTransition(beat)

	sc := m.ctx.Scene
	m.ctx.SampleScene(beat)
	hairWorld, _ := sc.NodeWorld("VegetableHolder/Vegetable/Hairs")
	for _, h := range m.hairs {
		if h.done && beat > h.judgeBeat()+3 {
			continue
		}
		h.inst.Rot = h.rot
		h.inst.Queue(sc, beat, hairWorld, 0)
	}
	for _, p := range m.passes {
		if p.inst == nil || p.dead {
			continue
		}
		m.updateTweezer(p, beat)
		p.inst.Queue(sc, beat, kart.Identity(), 0)
	}
	for _, n := range m.noPeekSigns {
		m.updateNoPeek(n, beat)
		n.inst.Queue(sc, beat, kart.Identity(), 0)
	}
	for _, d := range m.dropped {
		age := math.Max(0, beat-d.born)
		x := d.x + d.vx*age
		y := d.y + d.vy*age - 0.45*age*age
		w := kart.Translate(x, y).Mul(kart.Rotate(d.rot + d.spin*age)).Mul(kart.Scale(0.8, 0.8))
		sc.Queue(kart.ExtraSprite{Sprite: d.sprite, World: w, Order: 2, Tint: [4]float64{0, 0, 0, 1}})
	}
	sc.Draw(screen, m.proj)
	_ = t
}

func (m *Module) startInterval(ev intervalEvt, gameSwitchBeat float64) {
	if m.startedIntervals[ev.idx] {
		return
	}
	m.startedIntervals[ev.idx] = true
	if ev.legacyNoSwap {
		ev.autoSwapVeggie = false
	}
	if m.firstVeggie {
		m.firstVeggie = false
		if ev.autoSwapVeggie {
			m.changeVegetableImmediate(ev.vegType, ev.uniqueColor, ev.color)
		}
	} else if ev.autoSwapVeggie {
		swapBeat := ev.beat - 0.5
		m.ctx.At(swapBeat, func() { m.nextVegetable(swapBeat, ev.vegType, ev.uniqueColor, ev.color) })
	}

	hairs := m.hairsInInterval(ev)
	for i := range hairs {
		h := hairs[i]
		h.interval = ev.idx
		h.relative = h.beat - ev.beat
		h.judgeBeat = ev.beat + ev.length + h.relative
		rot := hairRotation(h.relative, ev.length)
		inactive := h.beat < gameSwitchBeat
		m.spawnHair(h, rot, inactive)
	}
	if ev.autoPassTurn {
		m.schedulePassTurn(ev.beat+ev.length, ev.length, hairs)
	}
}

func (m *Module) hairsInInterval(ev intervalEvt) []hairEvt {
	var out []hairEvt
	for _, h := range m.hairEvents {
		if h.beat >= ev.beat && h.beat < ev.beat+ev.length {
			out = append(out, h)
		}
	}
	return out
}

func (m *Module) spawnHair(ev hairEvt, rot float64, inactive bool) *hairInst {
	if h := m.spawnedByEvent[ev.idx]; h != nil {
		return h
	}
	if !inactive {
		m.stopTransitionIfActive(ev.beat)
	}
	t := m.hairT
	state := "SmallAppear"
	if ev.long {
		t = m.longT
		state = "LongAppear"
	}
	if t == nil {
		return nil
	}
	in := t.NewInstance()
	h := &hairInst{event: ev, inst: in, rot: rot}
	if inactive {
		in.PlayFrozen("", state, 1)
	} else {
		in.PlayState("", state, ev.beat, m.ctx.SecPerBeat(ev.beat))
		if ev.long {
			m.ctx.SoundAt(ev.beat, "longAppear", 1)
		} else {
			m.ctx.SoundAt(ev.beat, "shortAppear", 1)
		}
	}
	m.spawnedByEvent[ev.idx] = h
	m.hairs = append(m.hairs, h)
	return h
}

func (m *Module) passTurnStandalone(beat float64) {
	ev := m.activeIntervalBefore(beat)
	if ev == nil {
		return
	}
	hairs := m.hairsInInterval(*ev)
	for i := range hairs {
		hairs[i].interval = ev.idx
		hairs[i].relative = hairs[i].beat - ev.beat
		hairs[i].judgeBeat = beat + hairs[i].relative
		if m.spawnedByEvent[hairs[i].idx] == nil {
			m.spawnHair(hairs[i], hairRotation(hairs[i].relative, ev.length), false)
		}
	}
	m.schedulePassTurn(beat, ev.length, hairs)
}

func (m *Module) activeIntervalBefore(beat float64) *intervalEvt {
	for i := len(m.intervals) - 1; i >= 0; i-- {
		ev := &m.intervals[i]
		if ev.beat <= beat && beat <= ev.beat+ev.length+4 {
			return ev
		}
	}
	return nil
}

func (m *Module) schedulePassTurn(beat, length float64, hairs []hairEvt) {
	if m.startedPasses[beat] {
		return
	}
	m.startedPasses[beat] = true
	state := &passState{beat: beat, endBeat: beat + length, hairsLeft: len(hairs)}
	if m.tweezerT != nil {
		state.inst = m.tweezerT.NewInstance()
		state.inst.PlayDefaultState("", beat, m.ctx.SecPerBeat(beat))
	}
	m.passes = append(m.passes, state)
	m.ctx.At(beat-0.25, func() {
		m.activePass = state
		state.hairsLeft = len(hairs)
		for i := range hairs {
			ev := hairs[i]
			h := m.spawnedByEvent[ev.idx]
			if h == nil {
				h = m.spawnHair(ev, hairRotation(ev.relative, length), false)
			}
			if h == nil {
				continue
			}
			state.hairs = append(state.hairs, h)
			if ev.long {
				m.scheduleLongHair(h, state)
			} else {
				m.scheduleShortHair(h, state)
			}
		}
	})
}

func (m *Module) scheduleShortHair(h *hairInst, pass *passState) {
	target := pass.beat + h.event.relative
	m.ctx.ScheduleInputCond(target,
		func() bool { return !h.done && m.ctx.GameAt(target) == m.ID() },
		func(state float64, _ engine.Judgment) {
			if state >= 1 || state <= -1 {
				m.nearShortHair(h, pass, target)
				return
			}
			m.aceShortHair(h, pass, target)
		},
		func() {},
	)
}

func (m *Module) scheduleLongHair(h *hairInst, pass *passState) {
	target := pass.beat + h.event.relative
	h.pullBeat = target
	h.pullEnd = target + 0.5
	m.ctx.ScheduleInputCond(target,
		func() bool { return !h.done && m.ctx.GameAt(target) == m.ID() },
		func(state float64, _ engine.Judgment) {
			if state >= 1 || state <= -1 {
				h.done = true
				return
			}
			h.pulling = true
			h.stopLoop = m.ctx.SoundLoop(fmt.Sprintf("longPull%d", 1+rand.Intn(4)))
			m.ctx.ScheduleInputReleaseCond(h.pullEnd,
				func() bool { return h.pulling && !h.done },
				func(state float64, _ engine.Judgment) {
					if state <= -1 {
						m.endLongEarly(h, m.ctx.Beat())
						return
					}
					m.aceLongHair(h, pass, h.pullEnd)
				},
				func() {},
			)
		},
		func() {},
	)
}

func (m *Module) aceShortHair(h *hairInst, pass *passState, beat float64) {
	if h.done {
		return
	}
	m.dropHeldHair(pass, beat)
	h.inst.SetActive("Hair", false)
	h.inst.SetActive("Stubble", true)
	h.inst.SetActive("Missed", false)
	h.done, h.hit = true, true
	pass.hairsLeft--
	pass.eyeSize = minInt(pass.eyeSize+1, 10)
	m.ctx.Sound(fmt.Sprintf("shortPluck%d", 1+rand.Intn(20)))
	m.playHop(pass)
	m.playTweezer(pass, "Tweezers_Pluck_Success", beat)
	pass.holding = true
	pass.heldLong = false
}

func (m *Module) nearShortHair(h *hairInst, pass *passState, beat float64) {
	if h.done {
		return
	}
	m.dropHeldHair(pass, beat)
	h.inst.SetActive("Hair", false)
	h.inst.SetActive("Missed", true)
	h.done, h.missed = true, true
	m.ctx.Sound("barely")
	m.ctx.Scene.PlayState("VegetableHolder/Vegetable", "Blink", beat, m.ctx.SecPerBeat(beat))
	m.playTweezer(pass, "Tweezers_Pluck_Fail", beat)
	pass.holding = true
	pass.heldLong = false
}

func (m *Module) aceLongHair(h *hairInst, pass *passState, beat float64) {
	if h.done {
		return
	}
	m.dropHeldHair(pass, beat)
	if h.stopLoop != nil {
		h.stopLoop()
	}
	h.inst.SetActive("LongHairHolder/Hair", false)
	h.inst.SetActive("LongHairHolder/Loop", false)
	h.inst.SetActive("Stubble", true)
	h.pulling = false
	h.done, h.hit = true, true
	pass.hairsLeft--
	pass.eyeSize = minInt(pass.eyeSize+1, 10)
	m.ctx.Sound("longPullEnd")
	m.playHop(pass)
	m.playTweezer(pass, "Tweezers_Pluck_Success", beat)
	pass.holding = true
	pass.heldLong = true
}

func (m *Module) endLongEarly(h *hairInst, beat float64) {
	if h.done {
		return
	}
	if h.stopLoop != nil {
		h.stopLoop()
	}
	h.pulling = false
	h.done = true
	if h.inst != nil {
		u := clamp01((beat - h.pullBeat) / 0.5)
		h.inst.PlayNormalized("", "Hairs/LoopPull", u)
	}
	if m.activePass != nil {
		m.playTweezer(m.activePass, "Tweezers_Idle", beat)
	}
	m.ctx.ScoreMiss()
}

func (m *Module) playHop(pass *passState) {
	state := "HopFinal"
	if pass.hairsLeft > 0 {
		state = fmt.Sprintf("Hop%d", pass.eyeSize)
	}
	if m.currentType == vegLightbulb {
		if pass.hairsLeft <= 0 {
			state = "HopFinalLightBulb"
		} else {
			state = "Hop0"
		}
	}
	m.ctx.Scene.PlayState("VegetableHolder/Vegetable", state, m.ctx.Beat(), m.ctx.SecPerBeat(m.ctx.Beat()))
}

func (m *Module) playTweezer(pass *passState, state string, beat float64) {
	if pass != nil && pass.inst != nil {
		pass.inst.PlayState("", state, beat, m.ctx.SecPerBeat(beat))
	}
}

func (m *Module) dropHeldHair(pass *passState, beat float64) {
	if pass == nil || !pass.holding {
		return
	}
	rot := tweezerRotation(pass, beat)
	x, y := tweezerPos(beat)
	m.dropped = append(m.dropped, droppedHair{
		born: beat, x: x, y: y, vx: rand.Float64()*0.2 - 0.1, vy: 0.18 + rand.Float64()*0.12,
		rot: rot, spin: (rand.Float64()*240 - 120) * math.Pi / 180, sprite: "hair_4",
	})
	pass.holding = false
}

func (m *Module) nextVegetable(beat float64, typ int, unique bool, c [4]float64) {
	m.ctx.SoundAt(beat, "register", 1)
	m.transition = &vegTransition{
		beat: beat, length: 0.45, toType: clampVeg(typ),
		toColor: m.veggieColor(clampVeg(typ), unique, c),
	}
	m.ctx.Scene.SetActive("VegetableHolder/VegetableDuplicate", true)
	m.ctx.Scene.SetSpriteOver("VegetableHolder/VegetableDuplicate", veggieSprite(typ))
	m.ctx.Scene.SetColorOver("VegetableHolder/VegetableDuplicate", m.transition.toColor)
	m.ctx.Scene.SetActive("VegetableHolder/VegetableDuplicate/LightBulb", typ == vegLightbulb)
}

func (m *Module) changeVegetableImmediate(typ int, unique bool, c [4]float64) {
	m.transition = nil
	m.ctx.Scene.SetPosOver("VegetableHolder", 0, -0.2)
	m.ctx.Scene.SetActive("VegetableHolder/VegetableDuplicate", false)
	m.currentType = clampVeg(typ)
	m.currentColor = m.veggieColor(m.currentType, unique, c)
	m.applyVegetable(m.currentType, m.currentColor)
	m.resetVegetable(m.ctx.Beat())
}

func (m *Module) updateVegetableTransition(beat float64) {
	tr := m.transition
	if tr == nil {
		m.ctx.Scene.SetPosOver("VegetableHolder", 0, -0.2)
		return
	}
	u := (beat - tr.beat) / tr.length
	if u >= 1 {
		m.stopTransitionIfActive(beat)
		return
	}
	if u < 0 {
		u = 0
	}
	x := engine.Ease(16, 0, -vegDupeOffset, u)
	m.ctx.Scene.SetPosOver("VegetableHolder", x, -0.2)
}

func (m *Module) stopTransitionIfActive(beat float64) {
	tr := m.transition
	if tr == nil {
		return
	}
	m.currentType = tr.toType
	m.currentColor = tr.toColor
	m.transition = nil
	m.ctx.Scene.SetPosOver("VegetableHolder", 0, -0.2)
	m.ctx.Scene.SetActive("VegetableHolder/VegetableDuplicate", false)
	m.applyVegetable(m.currentType, m.currentColor)
	m.resetVegetable(beat)
	for _, p := range m.passes {
		if tweezerRotation(p, beat) > 0 {
			p.dead = true
		}
	}
}

func (m *Module) resetVegetable(beat float64) {
	if m.activePass != nil {
		m.dropHeldHair(m.activePass, beat)
	}
	m.hairs = nil
	m.spawnedByEvent = map[int]*hairInst{}
	m.dropped = nil
	m.ctx.Scene.PlayState("VegetableHolder/Vegetable", "Idle", beat, m.ctx.SecPerBeat(beat))
}

func (m *Module) applyVegetable(typ int, c [4]float64) {
	typ = clampVeg(typ)
	m.ctx.Scene.SetSpriteOver("VegetableHolder/Vegetable", veggieSprite(typ))
	m.ctx.Scene.SetSpriteOver("VegetableHolder/VegetableDuplicate", veggieSprite(typ))
	m.ctx.Scene.SetColorOver("VegetableHolder/Vegetable", c)
	m.ctx.Scene.SetColorOver("VegetableHolder/VegetableDuplicate", c)
	m.ctx.Scene.SetActive("VegetableHolder/Vegetable/LightBulb", typ == vegLightbulb)
	m.ctx.Scene.SetActive("VegetableHolder/Vegetable/LightHolder", typ == vegLightbulb)
	m.ctx.Scene.SetActive("VegetableHolder/VegetableDuplicate/LightBulb", false)
}

func (m *Module) veggieColor(typ int, unique bool, c [4]float64) [4]float64 {
	if unique {
		if c[3] == 0 {
			c[3] = 1
		}
		return c
	}
	return m.veggieColors[clampVeg(typ)]
}

func (m *Module) setSmile(typ int) {
	m.ctx.Scene.SetBool("VegetableHolder/Vegetable", "UseAltSmile", typ == smileAlt)
	m.ctx.Scene.SetBool("VegetableHolder/Vegetable", "UseOpenEyeSmile", typ == smileTengoku)
}

func (m *Module) spawnNoPeek(ev noPeekEvt) {
	for _, n := range m.noPeekSigns {
		if math.Abs(n.event.beat-ev.beat) < 1e-6 {
			return
		}
	}
	if m.noPeekT == nil {
		return
	}
	in := m.noPeekT.NewInstance()
	in.SetSprite("", noPeekSprite(ev.typ))
	m.noPeekSigns = append(m.noPeekSigns, &noPeekState{event: ev, inst: in})
}

func (m *Module) updateNoPeek(n *noPeekState, beat float64) {
	ev := n.event
	switch {
	case beat < ev.beat:
		n.inst.PlayNormalized("", "Animations/NoPeekRise", clamp01(beat-(ev.beat-1)))
	case beat < ev.beat+ev.length:
		n.inst.PlayNormalized("", "Animations/NoPeekRise", 1)
	default:
		n.inst.PlayNormalized("", "Animations/NoPeekLower", clamp01(beat-(ev.beat+ev.length)))
	}
	n.inst.SetSprite("", noPeekSprite(ev.typ))
}

func (m *Module) updateTweezer(p *passState, beat float64) {
	if p.inst == nil {
		return
	}
	p.inst.Offset = [2]float64{m.vegetableHolderX(beat), 3.68}
	p.inst.Rot = tweezerRotation(p, beat)
}

func (m *Module) vegetableHolderX(beat float64) float64 {
	if m.transition == nil {
		return 0
	}
	u := clamp01((beat - m.transition.beat) / m.transition.length)
	return engine.Ease(16, 0, -vegDupeOffset, u)
}

func tweezerRotation(p *passState, beat float64) float64 {
	dur := math.Max(p.endBeat-1-p.beat, 1)
	u := clamp01((beat - p.beat) / dur)
	return (hairArcStart + hairArcDeg*u) * math.Pi / 180
}

func tweezerPos(beat float64) (float64, float64) {
	_ = beat
	return 0, -1.55
}

func (m *Module) restorePersistentState(beat float64) {
	m.veggieColors = defaultVeggieColors
	for _, ev := range m.colorEvents {
		if ev.beat <= beat {
			m.veggieColors = ev.colors
		}
	}
	lastSmile := smileDefault
	for _, ev := range m.smileEvents {
		if ev.beat <= beat {
			lastSmile = ev.typ
		}
	}
	m.setSmile(lastSmile)
}

func (m *Module) bgAt(beat float64) [4]float64 {
	out := defaultBgColor
	for _, ev := range m.bgEvents {
		if beat < ev.beat {
			break
		}
		if ev.length > 0 && beat < ev.beat+ev.length {
			u := clamp01((beat - ev.beat) / ev.length)
			for i := 0; i < 4; i++ {
				out[i] = engine.Ease(ev.ease, ev.from[i], ev.to[i], u)
			}
			return out
		}
		out = ev.to
	}
	return out
}

func (m *Module) pruneDead(beat float64) {
	passes := m.passes[:0]
	for _, p := range m.passes {
		if !p.dead && beat <= p.endBeat+2 {
			passes = append(passes, p)
		}
	}
	m.passes = passes
	if m.activePass != nil && (m.activePass.dead || beat > m.activePass.endBeat+2) {
		m.activePass = nil
	}
	dropped := m.dropped[:0]
	for _, d := range m.dropped {
		if beat < d.born+3 {
			dropped = append(dropped, d)
		}
	}
	m.dropped = dropped
	signs := m.noPeekSigns[:0]
	for _, n := range m.noPeekSigns {
		if beat <= n.event.beat+n.event.length+1 {
			signs = append(signs, n)
		}
	}
	m.noPeekSigns = signs
}

func (h *hairInst) judgeBeat() float64 {
	if h.event.judgeBeat != 0 {
		return h.event.judgeBeat
	}
	return h.event.beat
}

func hairRotation(relative, interval float64) float64 {
	denom := math.Max(interval-1, 1)
	return (hairArcStart + hairArcDeg*clamp01(relative/denom)) * math.Pi / 180
}

func veggieSprite(typ int) string { return fmt.Sprintf("veggies_%d", clampVeg(typ)) }

func noPeekSprite(typ int) string {
	switch typ {
	case noPeekHalfLeft:
		return "noPeek_1"
	case noPeekHalfRight:
		return "noPeek_2"
	default:
		return "noPeek_0"
	}
}

func clampVeg(v int) int {
	if v < 0 || v >= vegCount {
		return vegOnion
	}
	return v
}

func boolParam(e *riq.Entity, key string) bool { return e.Float(key, 0) != 0 }

func boolDefault(e *riq.Entity, key string, def bool) bool {
	if _, ok := e.Data[key]; !ok {
		return def
	}
	return e.Float(key, 0) != 0
}

func colorParam(e *riq.Entity, key string, def [4]float64) [4]float64 {
	v, ok := e.Data[key]
	if !ok {
		return def
	}
	switch c := v.(type) {
	case []any:
		out := def
		for i := 0; i < len(c) && i < 4; i++ {
			if f, ok := c[i].(float64); ok {
				out[i] = f
			}
		}
		return out
	case map[string]any:
		out := def
		for i, k := range []string{"r", "g", "b", "a"} {
			if f, ok := c[k].(float64); ok {
				out[i] = f
			}
		}
		return out
	}
	return def
}

func toNRGBA(c [4]float64) color.NRGBA {
	return color.NRGBA{
		R: uint8(clamp01(c[0]) * 255),
		G: uint8(clamp01(c[1]) * 255),
		B: uint8(clamp01(c[2]) * 255),
		A: uint8(clamp01(c[3]) * 255),
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

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
