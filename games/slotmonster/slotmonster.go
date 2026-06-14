// Package slotmonster ports Slot Monster's interval/slot/pass-turn flow,
// button state machine, rolling loop, eye item results, and win particles.
package slotmonster

import (
	"math"
	"math/rand"
	"sort"

	"github.com/hajimehoshi/ebiten/v2"

	"hsdemo/engine"
	"hsdemo/kart"
	"hsdemo/riq"
)

const (
	drumDefault = iota
	drumBass
	drumSnare

	eyeRandom = 0
)

type intervalEvt struct {
	beat, length float64
	auto         bool
	eyeType      int
}

type slotEvt struct {
	beat float64
	drum int
}

type passEvt struct {
	beat, length float64
}

type buttonColorEvt struct {
	beat   float64
	colors [3][4]float64
	flash  [4]float64
}

type gameplayEvt struct {
	beat   float64
	win    bool
	amount float64
	speed  float64
	stars  bool
}

type Module struct {
	ctx  *engine.Ctx
	proj kart.Aff

	sm, coinsRoot string
	eyes          []string
	buttons       [3]slotButton

	intervals []intervalEvt
	slots     []slotEvt
	passes    []passEvt
	colors    []buttonColorEvt
	gameplay  []gameplayEvt

	rng              *rand.Rand
	doWin            bool
	inputsActive     bool
	currentEyeSprite int
	eyeSprites       [3]int
	maxButtons       int
	currentButton    int
	stopRolling      func()

	particleAmount float64
	particleSpeed  float64
	particleStars  bool
	emitters       []winEmitter
	particles      []winParticle
}

func New() engine.Module { return &Module{} }

func (m *Module) ID() string { return "slotMonster" }

func (m *Module) Load(ctx *engine.Ctx) error {
	m.ctx = ctx
	if err := ctx.LoadAssets("slotMonster"); err != nil {
		return err
	}
	m.proj = kart.Translate(engine.ScreenW/2, engine.ScreenH/2).Mul(kart.Scale(54, -54))
	m.sm = roleOr(ctx, "smAnim", "SlotMonster")
	m.coinsRoot = roleOr(ctx, "winParticles", "SlotMonster/Head/Coins")
	m.eyes = append(m.eyes, ctx.Assets.Extra.RefArrays["eyeAnims"]...)
	if len(m.eyes) == 0 {
		m.eyes = []string{"SlotMonster/Head/Eye1/Eye1", "SlotMonster/Head/Eye2/Eye2", "SlotMonster/Head/Eye3/Eye3"}
	}
	for i := range m.buttons {
		m.buttons[i] = m.loadButton(i)
	}
	m.rng = rand.New(rand.NewSource(1))
	m.reset(0)
	return nil
}

func roleOr(ctx *engine.Ctx, key, fallback string) string {
	if p := ctx.Role(key); p != "" {
		return p
	}
	return fallback
}

func (m *Module) OnEvent(e *riq.Entity) {
	switch e.Datamodel {
	case "slotMonster/startInterval":
		m.intervals = append(m.intervals, intervalEvt{
			beat: e.Beat, length: e.Length,
			auto: boolParamDefault(e, "auto", true), eyeType: int(e.Float("eyeType", eyeRandom)),
		})
	case "slotMonster/slot":
		m.slots = append(m.slots, slotEvt{beat: e.Beat, drum: int(e.Float("drum", drumDefault))})
		b := e.Beat
		// Loader inactiveFunction: slot events heard outside this minigame still
		// play the touch sound, but do not create runtime button state.
		m.ctx.At(b, func() {
			if m.ctx.GameAt(b) != m.ID() {
				m.ctx.Sound("start_touch")
			}
		})
	case "slotMonster/passTurn":
		m.passes = append(m.passes, passEvt{beat: e.Beat, length: e.Length})
	case "slotMonster/buttonColor":
		m.colors = append(m.colors, buttonColorEvt{
			beat: e.Beat,
			colors: [3][4]float64{
				colorParam(e, "button1", [4]float64{0.38, 0.98, 0.25, 1}),
				colorParam(e, "button2", [4]float64{0.8, 0.28, 0.95, 1}),
				colorParam(e, "button3", [4]float64{0.87, 0, 0, 1}),
			},
			flash: colorParam(e, "flash", [4]float64{1, 1, 0.68, 1}),
		})
	case "slotMonster/gameplayModifiers":
		m.gameplay = append(m.gameplay, gameplayEvt{
			beat: e.Beat, win: boolParamDefault(e, "lottery", true),
			amount: e.Float("lotteryAmount", 1), speed: e.Float("lotterySpeed", 1),
			stars: boolParamDefault(e, "stars", false),
		})
	}
}

func (m *Module) Ready() {
	sort.Slice(m.intervals, func(i, j int) bool { return m.intervals[i].beat < m.intervals[j].beat })
	sort.Slice(m.slots, func(i, j int) bool { return m.slots[i].beat < m.slots[j].beat })
	sort.Slice(m.passes, func(i, j int) bool { return m.passes[i].beat < m.passes[j].beat })
	sort.Slice(m.colors, func(i, j int) bool { return m.colors[i].beat < m.colors[j].beat })
	sort.Slice(m.gameplay, func(i, j int) bool { return m.gameplay[i].beat < m.gameplay[j].beat })

	for _, ev := range m.colors {
		ev := ev
		m.ctx.At(ev.beat, func() { m.applyButtonColor(ev) })
	}
	for _, ev := range m.gameplay {
		ev := ev
		m.ctx.At(ev.beat, func() { m.applyGameplay(ev) })
	}
	for _, ev := range m.intervals {
		ev := ev
		m.ctx.At(ev.beat, func() { m.startInterval(ev, 0) })
	}
	for _, ev := range m.passes {
		ev := ev
		m.ctx.At(ev.beat, func() { m.passTurn(ev.beat, ev.length, -1, nil) })
	}
}

func (m *Module) OnSwitch(beat float64) {
	m.reset(beat)
	for _, ev := range m.colors {
		if ev.beat > beat {
			break
		}
		m.applyButtonColor(ev)
	}
	for _, ev := range m.gameplay {
		if ev.beat > beat {
			break
		}
		m.applyGameplay(ev)
	}
	for _, ev := range m.intervals {
		if ev.beat >= beat {
			break
		}
		if ev.beat+ev.length > beat {
			m.startInterval(ev, beat)
		}
	}
}

func (m *Module) Whiff(beat float64) {
	if !m.inputsActive || m.currentButton < 0 || m.currentButton >= m.maxButtons {
		return
	}
	// SlotButton.Update only treats free presses outside any pending input
	// window as button misses. During a scheduled window the input system owns
	// the miss/barely resolution.
	if m.ctx.ExpectingPressNow() {
		return
	}
	if !m.buttons[m.currentButton].pressed {
		m.hitButton(false, 0, beat)
		m.ctx.ScoreMiss()
	}
}

func (m *Module) Update(t, beat float64) {
	m.currentButton = m.firstUnpressedButton()
	m.updateParticles(t)
}

func (m *Module) Draw(screen *ebiten.Image, t, beat float64) {
	for i := range m.buttons {
		m.buttons[i].applyColor(m.ctx.Scene, t)
	}
	m.ctx.SampleScene(beat)
	m.queueParticles(t)
	m.ctx.Scene.Draw(screen, m.proj)
}

func (m *Module) reset(beat float64) {
	if m.stopRolling != nil {
		m.stopRolling()
		m.stopRolling = nil
	}
	m.doWin = true
	m.inputsActive = false
	m.currentEyeSprite = 1
	m.eyeSprites = [3]int{}
	m.maxButtons = 0
	m.currentButton = -1
	m.particleAmount, m.particleSpeed, m.particleStars = 1, 1, false
	m.emitters = nil
	m.particles = nil
	m.ctx.Scene.PlayDefaultState(m.sm, beat, m.ctx.SecPerBeat(beat))
	for _, eye := range m.eyes {
		m.ctx.Scene.PlayFrozen(eye, "Idle", 1)
	}
	for i := range m.buttons {
		m.buttons[i].reset()
		m.ctx.Scene.PlayDefaultState(m.buttons[i].root, beat, m.ctx.SecPerBeat(beat))
	}
}

func (m *Module) startInterval(iv intervalEvt, gameSwitchBeat float64) {
	if m.stopRolling != nil {
		m.stopRolling()
		m.stopRolling = nil
	}
	slotActions := m.slotsIn(iv)
	if len(slotActions) == 0 {
		return
	}
	m.ctx.Sound("start_touch")
	m.ctx.Scene.PlayState(m.sm, "Prepare", iv.beat, 0.5)
	for _, eye := range m.eyes {
		m.ctx.Scene.PlayFrozen(eye, "Idle", 1)
	}

	m.maxButtons = min(len(slotActions), len(m.buttons))
	for i := 0; i < m.maxButtons; i++ {
		m.buttons[i].ready(m.ctx.Scene, iv.beat)
		if slotActions[i].beat < gameSwitchBeat {
			continue
		}
		slot := slotActions[i]
		idx := i
		m.ctx.SoundAt(slot.beat, "common_"+drumSFX(slot), 1)
		m.ctx.At(slot.beat, func() { m.buttons[idx].tryFlash(m.ctx.Scene, slot.beat, m.ctx.BeatToTime(slot.beat)) })
	}
	m.currentButton = m.firstUnpressedButton()
	if iv.auto {
		m.ctx.At(iv.beat+iv.length, func() {
			if iv.eyeType == eyeRandom {
				m.currentEyeSprite = m.rng.Intn(9) + 1
			} else {
				m.currentEyeSprite = iv.eyeType - 1
			}
			m.passTurn(iv.beat+iv.length, 1, iv.beat, slotActions)
		})
	}
}

func (m *Module) passTurn(beat, length, startBeat float64, slotActions []slotEvt) {
	if len(slotActions) == 0 {
		iv, ok := m.previousInterval(beat)
		if !ok {
			return
		}
		if startBeat < 0 {
			startBeat = iv.beat
		}
		slotActions = m.slotsIn(iv)
	}
	if len(slotActions) == 0 {
		return
	}
	if startBeat < 0 {
		startBeat = slotActions[0].beat
	}

	m.ctx.Scene.PlayState(m.sm, "Release", beat, 0.5)
	for _, eye := range m.eyes {
		m.ctx.Scene.PlayState(eye, "Spin", beat, 0.5)
	}
	m.ctx.Sound("start_rolling")
	if m.stopRolling != nil {
		m.stopRolling()
	}
	m.stopRolling = m.ctx.SoundLoop("rolling")
	m.ctx.At(beat, func() { m.inputsActive = true })
	m.currentButton = m.firstUnpressedButton()

	limit := min(len(slotActions), len(m.buttons))
	m.maxButtons = limit
	m.currentButton = m.firstUnpressedButton()
	for i := 0; i < limit; i++ {
		idx := i
		slotBeat := slotActions[i].beat
		target := beat + length + slotBeat - startBeat
		m.ctx.At(target, func() { m.buttons[idx].tryFlash(m.ctx.Scene, target, m.ctx.BeatToTime(target)) })
		var onMiss func()
		if i == len(slotActions)-1 {
			onMiss = func() { m.buttonEndMiss(m.ctx.Beat()) }
		}
		m.ctx.ScheduleInputCond(target,
			func() bool { return m.currentButton == idx && !m.buttons[idx].pressed },
			func(state float64, _ engine.Judgment) { m.buttonHit(state, m.ctx.Beat()) },
			onMiss)
	}
}

func (m *Module) buttonHit(state, beat float64) {
	timing := 0
	switch {
	case state >= 1:
		timing = -1
	case state <= -1:
		timing = 1
	}
	m.hitButton(true, timing, beat)
	if timing != 0 {
		m.ctx.PlayCommon("nearMiss")
	}
}

func (m *Module) hitButton(isHit bool, timing int, beat float64) {
	m.currentButton = m.firstUnpressedButton()
	if m.currentButton < 0 || m.currentButton >= m.maxButtons {
		return
	}
	isLast := m.currentButton == m.maxButtons-1
	m.buttons[m.currentButton].press(m.ctx.Scene, beat, !isHit || timing != 0)

	end := m.currentButton + 1
	if isLast {
		end = len(m.eyes)
	}
	for i := m.currentButton; i < end && i < len(m.eyes); i++ {
		state, _ := m.ctx.Scene.StateInfo(m.eyes[i], beat)
		if state != "Spin" {
			continue
		}
		anim := m.eyeResultAnim(i, isHit, timing)
		m.ctx.Scene.PlayState(m.eyes[i], anim, beat, m.ctx.SecPerBeat(beat))
	}

	isMiss := m.anyButtonMissed()
	sfx := "stop_"
	if isLast && isHit && !isMiss {
		sfx += "hit"
	} else {
		sfx += intString(m.currentButton + 1)
	}
	m.ctx.Sound(sfx)
	if !isLast {
		return
	}
	if m.stopRolling != nil {
		m.stopRolling()
		m.stopRolling = nil
	}
	m.inputsActive = false
	if isHit && !isMiss {
		m.ctx.Scene.PlayState(m.sm, "Win", beat, 0.5)
		if m.doWin {
			m.ctx.Sound("win")
			m.spawnWinEmitter(beat)
		}
		return
	}
	m.ctx.Scene.PlayState(m.sm, "Lose", beat, 0.5)
}

func (m *Module) eyeResultAnim(i int, isHit bool, timing int) string {
	eyeSprite := m.currentEyeSprite
	if !isHit {
		for tries := 0; tries < 32; tries++ {
			candidate := m.rng.Intn(9) + 1
			if candidate == m.currentEyeSprite || m.eyeSpriteUsed(candidate) {
				continue
			}
			eyeSprite = candidate
			break
		}
		m.eyeSprites[i] = eyeSprite
		return "EyeItem" + intString(eyeSprite)
	}
	if timing == -1 {
		eyeSprite = (eyeSprite + 1) % 9
	}
	anim := "EyeItem" + intString(eyeSprite+1)
	if timing != 0 {
		anim += "Barely"
	}
	return anim
}

func (m *Module) buttonEndMiss(beat float64) {
	if m.stopRolling != nil {
		m.stopRolling()
		m.stopRolling = nil
	}
	m.inputsActive = false
	m.ctx.Scene.PlayState(m.sm, "Lose", beat, 0.5)
	for _, eye := range m.eyes {
		state, _ := m.ctx.Scene.StateInfo(eye, beat)
		if state == "Spin" {
			m.ctx.Scene.PlayFrozen(eye, "Idle", 1)
		}
	}
}

func (m *Module) applyButtonColor(ev buttonColorEvt) {
	for i := range m.buttons {
		m.buttons[i].color = ev.colors[i]
	}
	for i := range m.buttons {
		m.buttons[i].flashColor = ev.flash
	}
}

func (m *Module) applyGameplay(ev gameplayEvt) {
	m.doWin = ev.win
	m.particleAmount = ev.amount
	m.particleSpeed = ev.speed
	if m.particleSpeed <= 0 {
		m.particleSpeed = 1
	}
	m.particleStars = ev.stars
}

func (m *Module) slotsIn(iv intervalEvt) []slotEvt {
	var out []slotEvt
	for _, s := range m.slots {
		if s.beat >= iv.beat && s.beat < iv.beat+iv.length {
			out = append(out, s)
		}
	}
	return out
}

func (m *Module) previousInterval(beat float64) (intervalEvt, bool) {
	for i := len(m.intervals) - 1; i >= 0; i-- {
		if m.intervals[i].beat+m.intervals[i].length < beat {
			return m.intervals[i], true
		}
	}
	return intervalEvt{}, false
}

func (m *Module) firstUnpressedButton() int {
	for i := 0; i < m.maxButtons && i < len(m.buttons); i++ {
		if !m.buttons[i].pressed {
			return i
		}
	}
	return -1
}

func (m *Module) anyButtonMissed() bool {
	for i := 0; i < m.maxButtons && i < len(m.buttons); i++ {
		if m.buttons[i].missed {
			return true
		}
	}
	return false
}

func (m *Module) eyeSpriteUsed(sprite int) bool {
	for _, s := range m.eyeSprites {
		if s == sprite {
			return true
		}
	}
	return false
}

func drumSFX(s slotEvt) string {
	switch s.drum {
	case drumBass:
		return "bassDrumNTR"
	case drumSnare:
		return "snareDrumNTR"
	default:
		if math.Abs(s.beat-math.Round(s.beat)) < 1e-6 {
			return "bassDrumNTR"
		}
		return "snareDrumNTR"
	}
}

func intString(v int) string {
	if v == 10 {
		return "10"
	}
	return string(rune('0' + v))
}

func boolParamDefault(e *riq.Entity, key string, def bool) bool {
	if v, ok := e.Data[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
		return e.Float(key, 0) != 0
	}
	return def
}

func colorParam(e *riq.Entity, key string, def [4]float64) [4]float64 {
	raw, ok := e.Data[key].(map[string]any)
	if !ok {
		return def
	}
	out := def
	for i, k := range []string{"r", "g", "b", "a"} {
		if v, ok := raw[k].(float64); ok {
			out[i] = v
		}
	}
	return out
}
