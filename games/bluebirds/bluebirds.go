// Package bluebirds ports Blue Birds' peck/stretch call-response game,
// captain/bird movement controls, story cards, gradient background, pitched
// voice cues, and the small AnimationEvents script attached to each bird.
package bluebirds

import (
	"math"
	"sort"

	"github.com/hajimehoshi/ebiten/v2"

	"hsdemo/engine"
	"hsdemo/kart"
	"hsdemo/riq"
)

const animScale = 0.5

var (
	defaultTop    = [4]float64{246.0 / 255, 1, 230.0 / 255, 1}
	defaultBottom = [4]float64{73.0 / 255, 205.0 / 255, 156.0 / 255, 1}
	white         = [4]float64{1, 1, 1, 1}
)

type bopEvt struct {
	beat, length float64
	bop, auto    bool
}

type moveEvt struct {
	beat, length   float64
	startX, startY float64
	endX, endY     float64
	ease           int
}

type gradEvt struct {
	beat, length float64
	top0, top1   [4]float64
	bot0, bot1   [4]float64
	ease         int
}

type pitchEvt struct {
	beat    float64
	enabled bool
}

type Module struct {
	ctx  *engine.Ctx
	proj kart.Aff

	captain, captainHolder string
	bird1, bird2, bird3    string
	effect1, effect2       string
	effect3, memory        string
	memorySprite, finText  string
	captainRoot, birdRoot  string
	captainBase, birdBase  [2]float64
	gradientMat            string
	memoryImages           []string

	bops       []bopEvt
	captMoves  []moveEvt
	birdMoves  []moveEvt
	gradients  []gradEvt
	pitches    []pitchEvt
	animSerial map[string]int

	canBop, canBopOthers bool
	canStretch           bool
	isSuccess            bool
	isStackedInputs      bool
	canPitch             bool
	lastPulse            int
}

func New() engine.Module {
	return &Module{
		canBop: true, canBopOthers: true,
		animSerial: map[string]int{},
		lastPulse:  math.MinInt,
	}
}

func (m *Module) ID() string { return "blueBirds" }

func (m *Module) Load(ctx *engine.Ctx) error {
	m.ctx = ctx
	if err := ctx.LoadAssets("blueBirds"); err != nil {
		return err
	}
	if err := ctx.Assets.ApplyTexts(); err != nil {
		return err
	}
	m.proj = kart.Translate(engine.ScreenW/2, engine.ScreenH/2).Mul(kart.Scale(54, -54))

	m.captain = roleOr(ctx, "captainAnim", "CaptainTransform/Captain/BirdHolder/Bird")
	m.captainHolder = roleOr(ctx, "captainHolderAnim", "CaptainTransform/Captain/BirdHolder")
	m.bird1 = roleOr(ctx, "bird1Anim", "Birds/CPUBirdLeft/BlueBird")
	m.bird2 = roleOr(ctx, "bird2Anim", "Birds/CPUBirdMiddle/BlueBird")
	m.bird3 = roleOr(ctx, "bird3Anim", "Birds/PlayerBird/BlueBird")
	m.effect1 = roleOr(ctx, "effect1Anim", "Birds/CPUBirdLeft/BlueBird/Effect")
	m.effect2 = roleOr(ctx, "effect2Anim", "Birds/CPUBirdMiddle/BlueBird/Effect")
	m.effect3 = roleOr(ctx, "effect3Anim", "Birds/PlayerBird/BlueBird/Effect")
	m.memory = roleOr(ctx, "memoryAnim", "Story/image")
	m.memorySprite = roleOr(ctx, "memorySprite", m.memory)
	m.finText = roleOr(ctx, "finText", "text")
	m.captainRoot = roleOr(ctx, "CaptainTransform", "CaptainTransform")
	m.birdRoot = roleOr(ctx, "BirdTransform", "Birds")
	if idx, ok := ctx.Assets.NodeIndex(m.captainRoot); ok {
		m.captainBase = ctx.Assets.Rig.Nodes[idx].Pos
	}
	if idx, ok := ctx.Assets.NodeIndex(m.birdRoot); ok {
		m.birdBase = ctx.Assets.Rig.Nodes[idx].Pos
	}

	game := ctx.Assets.Extra.Components["game"]
	m.gradientMat = game.Refs["gradientMat"]
	m.memoryImages = append(m.memoryImages, game.SpriteArrays["memoryImage"]...)
	if m.gradientMat == "" {
		m.gradientMat = "gradient"
	}

	ctx.Scene.SetActive(m.finText, false)
	m.applyGradient(0)
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
	case "blueBirds/bop":
		ev := bopEvt{beat: e.Beat, length: e.Length, bop: boolParamDefault(e, "bop", true), auto: boolParam(e, "auto")}
		m.bops = append(m.bops, ev)
		if ev.bop {
			for i := 0.0; i < ev.length; i++ {
				b := ev.beat + i
				m.ctx.At(b, func() { m.playAllBop(b) })
			}
		}
	case "blueBirds/peck":
		b := e.Beat
		m.ctx.At(b-0.25, func() { m.noBopping() })
		m.ctx.At(b, func() { m.peckYourBeak(b) })
	case "blueBirds/stretch":
		b := e.Beat
		m.ctx.At(b-0.25, func() { m.noBopping() })
		m.ctx.At(b, func() { m.stretchOutYourNeck(b) })
	case "blueBirds/showMemory":
		b, length := e.Beat, e.Length
		memory := int(e.Float("memory", 0))
		m.ctx.At(b, func() { m.showMemory(memory, b, length) })
	case "blueBirds/showText":
		show := boolParamDefault(e, "showtext", true)
		m.ctx.At(e.Beat, func() { m.ctx.Scene.SetActive(m.finText, show) })
	case "blueBirds/hideCaptain":
		b := e.Beat
		state := int(e.Float("state", 0))
		m.ctx.At(b, func() {
			if state == 1 {
				m.playCaptainHolder("MoveIn", b)
			} else {
				m.playCaptainHolder("MoveOut", b)
			}
		})
	case "blueBirds/moveCaptain":
		if boolParam(e, "doMove") {
			m.captMoves = append(m.captMoves, readMove(e))
		}
	case "blueBirds/moveBirds":
		if boolParam(e, "doMove") {
			m.birdMoves = append(m.birdMoves, readMove(e))
		}
	case "blueBirds/gradient":
		m.gradients = append(m.gradients, gradEvt{
			beat: e.Beat, length: e.Length, ease: int(e.Float("ease", 0)),
			top0: colorParam(e, "startTop", defaultTop),
			top1: colorParam(e, "endTop", defaultTop),
			bot0: colorParam(e, "startBottom", defaultBottom),
			bot1: colorParam(e, "endBottom", defaultBottom),
		})
	case "blueBirds/pitching":
		m.pitches = append(m.pitches, pitchEvt{beat: e.Beat, enabled: boolParam(e, "enabled")})
	}
}

func readMove(e *riq.Entity) moveEvt {
	return moveEvt{
		beat: e.Beat, length: e.Length,
		startX: e.Float("startMoveX", 0), startY: e.Float("startMoveY", 0),
		endX: e.Float("endMoveX", 0), endY: e.Float("endMoveY", 0),
		ease: int(e.Float("ease", 0)),
	}
}

func (m *Module) Ready() {
	sort.Slice(m.bops, func(i, j int) bool { return m.bops[i].beat < m.bops[j].beat })
	sort.Slice(m.captMoves, func(i, j int) bool { return m.captMoves[i].beat < m.captMoves[j].beat })
	sort.Slice(m.birdMoves, func(i, j int) bool { return m.birdMoves[i].beat < m.birdMoves[j].beat })
	sort.Slice(m.gradients, func(i, j int) bool { return m.gradients[i].beat < m.gradients[j].beat })
	sort.Slice(m.pitches, func(i, j int) bool { return m.pitches[i].beat < m.pitches[j].beat })
}

func (m *Module) OnSwitch(beat float64) {
	m.canBop, m.canBopOthers = true, true
	m.applyPitch(beat)
	m.applyGradient(beat)
	m.applyMovement(beat)
	m.lastPulse = int(math.Floor(beat)) - 1
	for _, p := range []string{m.bird1, m.bird2, m.bird3, m.effect1, m.effect2, m.effect3, m.captain, m.captainHolder, m.memory} {
		m.animSerial[p]++
		m.ctx.Scene.PlayDefaultState(p, beat, m.ctx.SecPerBeat(beat))
	}
}

func (m *Module) Whiff(beat float64) {
	m.playBird(m.bird3, "Pick", beat)
	m.playEffect(m.effect3, "Miss", beat)
	m.ctx.Sound("miss")
	m.playCaptain("Angry", beat)
	// The engine has already counted this unexpected press as a whiff.
	// Calling ScoreMiss here would double-count one physical input.
}

func (m *Module) Update(t, beat float64) {
	m.applyPitch(beat)
	m.applyMovement(beat)
	m.applyGradient(beat)
	m.handleUnexpectedRelease(beat)
	m.pulseBeats(beat)
}

func (m *Module) Draw(screen *ebiten.Image, t, beat float64) {
	m.applyGradient(beat)
	m.ctx.SampleScene(beat)
	m.ctx.Scene.Draw(screen, m.proj)
}

func (m *Module) peckYourBeak(beat float64) {
	pitch := m.voicePitch(beat)
	m.handleStackedEvents(beat, 1)

	m.ctx.SoundAtPitchPan(beat, "peck", 1, pitch, 0)
	m.ctx.SoundAtPitchPan(beat+0.5, "ur", 1, pitch, 0)
	m.ctx.SoundAtPitchPan(beat+1, "beak", 1, pitch, 0)
	for _, off := range []float64{2, 2.5, 3} {
		m.ctx.SoundAt(beat+off, "peck1", 1)
	}

	for _, off := range []float64{0, 0.5, 1} {
		b := beat + off
		m.ctx.At(b, func() { m.playCaptain("Talk", b) })
	}
	for _, off := range []float64{2, 2.5, 3} {
		b := beat + off
		m.ctx.At(b, func() {
			m.playCpuEffectAttack(b)
			m.playBird(m.bird1, "PickSuccessAlt", b)
			m.playBird(m.bird2, "PickSuccessAlt", b)
		})
	}
	m.ctx.At(beat+1.75, func() { m.isStackedInputs = true })
	m.ctx.At(beat+2.90, func() {
		m.isStackedInputs = false
		m.canBop, m.canBopOthers = true, true
	})

	for _, off := range []float64{2, 2.5, 3} {
		hitBeat := beat + off
		m.ctx.ScheduleInput(hitBeat, func(state float64, _ engine.Judgment) { m.justPeck(state, m.ctx.Beat()) }, m.miss)
	}
}

func (m *Module) stretchOutYourNeck(beat float64) {
	pitch := m.voicePitch(beat)
	m.handleStackedEvents(beat, 2)

	m.ctx.SoundAtPitchPan(beat, "stretch", 1, pitch, 0)
	m.ctx.SoundAtPitchPan(beat+0.75, "out", 1, pitch, 0)
	m.ctx.SoundAtPitchPan(beat+1.5, "your", 1, pitch, 0)
	m.ctx.SoundAtPitchPan(beat+2, "neck", 1, pitch, 0)
	m.ctx.SoundAt(beat+3, "peck1", 1)
	m.ctx.SoundAt(beat+4, "flap", 1)

	for _, off := range []float64{0, 0.75, 1.5, 2} {
		b := beat + off
		m.ctx.At(b, func() { m.playCaptain("Talk", b) })
	}
	m.ctx.At(beat+3, func() {
		b := beat + 3
		m.playCpuEffectAttack(b)
		m.playBird(m.bird1, "TameSuccess", b)
		m.playBird(m.bird2, "TameSuccess", b)
	})
	m.ctx.At(beat+4, func() {
		b := beat + 4
		m.playBird(m.bird1, "Shout", b)
		m.playBird(m.bird2, "Shout", b)
	})
	m.ctx.At(beat+2.75, func() { m.isStackedInputs = true })
	m.ctx.At(beat+4.25, func() {
		m.isStackedInputs = false
		m.canBop, m.canBopOthers = true, true
	})

	m.ctx.ScheduleInput(beat+3, func(state float64, _ engine.Judgment) { m.justHold(state, m.ctx.Beat()) }, m.miss)
	m.ctx.ScheduleInputRelease(beat+4, func(state float64, _ engine.Judgment) { m.justShout(state, m.ctx.Beat()) }, m.miss)
}

func (m *Module) voicePitch(beat float64) float64 {
	if !m.canPitch {
		return 1
	}
	// Heaven Studio multiplies by Conductor.TimelinePitch. The Go conductor
	// currently has no chart-wide pitch warp, so BPM/130 is the complete term.
	return m.ctx.BPMAt(beat) / 130
}

func (m *Module) justPeck(state, beat float64) {
	if math.Abs(state) >= 1 {
		m.playBird(m.bird3, "Pick", beat)
		m.ctx.Sound("miss")
		m.playCaptain("Angry", beat)
		m.playEffect(m.effect3, "Miss", beat)
		m.isSuccess = false
		return
	}
	if m.ctx.App.Autoplay {
		m.playBird(m.bird3, "PickSuccessAlt", beat)
	} else {
		m.playBird(m.bird3, "PickSuccess", beat)
	}
	m.ctx.Sound("tap")
	m.playEffect(m.effect3, "Attack", beat)
	m.isSuccess = true
}

func (m *Module) miss() {
	m.playCaptain("Angry", m.ctx.Beat())
	m.isSuccess = false
}

func (m *Module) justHold(state, beat float64) {
	if math.Abs(state) >= 1 {
		m.playCaptain("Angry", beat)
		m.playBird(m.bird3, "Tame", beat)
		m.playEffect(m.effect3, "Miss", beat)
		m.ctx.Sound("miss")
		m.isSuccess = false
		return
	}
	m.playBird(m.bird3, "TameSuccess", beat)
	m.playEffect(m.effect3, "Attack", beat)
	m.ctx.Sound("hold")
	m.isSuccess = true
}

func (m *Module) justShout(state, beat float64) {
	m.isSuccess = false
	m.canStretch = false
	if math.Abs(state) >= 1 {
		m.playCaptain("Angry", beat)
		m.playBird(m.bird3, "Miss", beat)
		m.ctx.Sound("miss")
		return
	}
	m.playCaptain("Smile", beat)
	m.playBird(m.bird3, "Shout", beat)
	m.ctx.Sound("release")
}

func (m *Module) handleStackedEvents(beat float64, inputType int) {
	if m.isStackedInputs {
		return
	}
	switch inputType {
	case 1:
		m.ctx.At(beat, func() {
			m.playBird(m.bird1, "PrePick01", beat)
			m.playBird(m.bird2, "PrePick01", beat)
			m.playBird(m.bird3, "PrePick01", beat)
		})
		m.ctx.At(beat+1, func() {
			b := beat + 1
			m.playBird(m.bird1, "PrePick02", b)
			m.playBird(m.bird2, "PrePick02", b)
			m.playBird(m.bird3, "PrePick02", b)
		})
	case 2:
		m.ctx.At(beat, func() { m.playBird(m.bird1, "PreShout01", beat) })
		m.ctx.At(beat+0.75, func() { m.playBird(m.bird2, "PreShout01", beat+0.75) })
		m.ctx.At(beat+1.5, func() { m.playBird(m.bird3, "PreShout01", beat+1.5) })
		m.ctx.At(beat+2, func() {
			b := beat + 2
			m.playBird(m.bird1, "PreShout02", b)
			m.playBird(m.bird2, "PreShout02", b)
			m.playBird(m.bird3, "PreShout02", b)
		})
	}
}

func (m *Module) handleUnexpectedRelease(beat float64) {
	if !m.ctx.ReleasedNow() || m.ctx.ExpectingReleaseNow() {
		return
	}
	if m.canStretch {
		m.playBird(m.bird3, "Shout", beat)
		m.ctx.Sound("release")
		m.playCaptain("Angry", beat)
		m.canStretch = false
		m.ctx.ScoreMiss()
		return
	}
	if m.isSuccess {
		m.playBird(m.bird3, "PickSuccessRelease", beat)
	} else {
		m.playBird(m.bird3, "PickRelease", beat)
	}
	m.isSuccess = false
}

func (m *Module) noBopping() {
	m.canBop = false
	m.canBopOthers = false
}

func (m *Module) playAllBop(beat float64) {
	m.playBird(m.bird1, "Beat", beat)
	m.playBird(m.bird2, "Beat", beat)
	m.playBird(m.bird3, "Beat", beat)
	m.playCaptain("Beat", beat)
}

func (m *Module) pulseBeats(beat float64) {
	pulse := int(math.Floor(beat + 1e-9))
	if m.lastPulse == math.MinInt {
		m.lastPulse = pulse - 1
	}
	if pulse <= m.lastPulse {
		return
	}
	for p := m.lastPulse + 1; p <= pulse; p++ {
		if p >= 0 {
			m.beatPulse(float64(p))
		}
	}
	m.lastPulse = pulse
}

func (m *Module) beatPulse(beat float64) {
	if !m.autoBopAt(beat) {
		return
	}
	if m.canBop && !m.birdBlocked(m.bird3, beat) {
		m.playBird(m.bird3, "Beat", beat)
	}
	if !m.canBopOthers {
		return
	}
	if !m.birdBlocked(m.bird1, beat) {
		m.playBird(m.bird1, "Beat", beat)
	}
	if !m.birdBlocked(m.bird2, beat) {
		m.playBird(m.bird2, "Beat", beat)
	}
	if !m.captainBlocked(beat) {
		m.playCaptain("Beat", beat)
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

func (m *Module) birdBlocked(path string, beat float64) bool {
	return playingAny(m.ctx.Scene, path, beat,
		"Pick", "PickRelease", "PickSuccess", "PickSuccessRelease", "PickSuccessAlt",
		"PrePick01", "PrePick02", "Tame", "TameLoop", "TameSuccess",
		"PreShout01", "PreShout02", "Shout", "Miss")
}

func (m *Module) captainBlocked(beat float64) bool {
	return playingAny(m.ctx.Scene, m.captain, beat, "Angry", "Smile", "Talk")
}

func playingAny(sc *kart.SceneInst, path string, beat float64, names ...string) bool {
	st, playing := sc.StateInfo(path, beat)
	if !playing {
		return false
	}
	for _, name := range names {
		if st == name {
			return true
		}
	}
	return false
}

func (m *Module) playBird(path, state string, beat float64) {
	m.playState(path, state, beat)
	seq := m.animSerial[path]
	switch state {
	case "Tame", "TameSuccess":
		m.runIfCurrent(path, seq, func() { m.canBop = false })
		m.scheduleAnimEvent(path, seq, beat+0.5, func() { m.canStretch = true })
	case "TameLoop":
		m.runIfCurrent(path, seq, func() { m.canBop = false })
	case "PrePick02":
		m.runIfCurrent(path, seq, func() { m.canStretch = false })
	case "PickRelease", "PickSuccessRelease":
		m.scheduleAnimEvent(path, seq, beat+0.5, func() { m.canBop = true })
	case "Shout":
		m.scheduleAnimEvent(path, seq, beat+2, func() { m.canBop = true })
	}
}

func (m *Module) playCaptain(state string, beat float64) { m.playState(m.captain, state, beat) }
func (m *Module) playCaptainHolder(state string, beat float64) {
	m.playState(m.captainHolder, state, beat)
}
func (m *Module) playEffect(path, state string, beat float64) { m.playState(path, state, beat) }

func (m *Module) playCpuEffectAttack(beat float64) {
	m.playEffect(m.effect1, "Attack", beat)
	m.playEffect(m.effect2, "Attack", beat)
}

func (m *Module) playState(path, state string, beat float64) {
	m.animSerial[path]++
	m.ctx.Scene.PlayState(path, state, beat, animScale)
}

func (m *Module) scheduleAnimEvent(path string, seq int, beat float64, fn func()) {
	m.ctx.At(beat, func() { m.runIfCurrent(path, seq, fn) })
}

func (m *Module) runIfCurrent(path string, seq int, fn func()) {
	if m.animSerial[path] == seq {
		fn()
	}
}

func (m *Module) showMemory(memory int, beat, length float64) {
	if memory >= 0 && memory < len(m.memoryImages) {
		m.ctx.Scene.SetSpriteOver(m.memorySprite, m.memoryImages[memory])
	}
	m.ctx.Scene.PlayState(m.memory, "fadeIn", beat, animScale)
	actualLength := length - 1
	m.ctx.At(beat+actualLength, func() { m.ctx.Scene.PlayState(m.memory, "fadeOut", beat+actualLength, animScale) })
}

func (m *Module) applyPitch(beat float64) {
	enabled := false
	for _, ev := range m.pitches {
		if ev.beat > beat {
			break
		}
		enabled = ev.enabled
	}
	m.canPitch = enabled
}

func (m *Module) applyMovement(beat float64) {
	applyMove := func(path string, base [2]float64, events []moveEvt) {
		if len(events) == 0 {
			return
		}
		var last *moveEvt
		for i := range events {
			if events[i].beat > beat {
				break
			}
			last = &events[i]
		}
		if last == nil {
			return
		}
		u := 1.0
		if last.length > 0 && beat < last.beat+last.length {
			u = (beat - last.beat) / last.length
		}
		v := engine.Ease(last.ease, 0, 1, u)
		x := base[0] + last.startX + (last.endX-last.startX)*v
		y := base[1] + last.startY + (last.endY-last.startY)*v
		m.ctx.Scene.SetPosOver(path, x, y)
	}
	applyMove(m.captainRoot, m.captainBase, m.captMoves)
	applyMove(m.birdRoot, m.birdBase, m.birdMoves)
}

func (m *Module) applyGradient(beat float64) {
	top, bot := defaultTop, defaultBottom
	for _, ev := range m.gradients {
		if ev.beat > beat {
			break
		}
		u := 1.0
		if ev.length > 0 && beat < ev.beat+ev.length {
			u = (beat - ev.beat) / ev.length
		}
		top = easeColor(ev.ease, ev.top0, ev.top1, u)
		bot = easeColor(ev.ease, ev.bot0, ev.bot1, u)
	}
	pal := kart.DefaultPalette()
	pal.Alpha = bot
	pal.Fill = white
	pal.Outline = top
	m.ctx.Scene.SetPaletteFor(m.gradientMat, pal)
}

func easeColor(ease int, a, b [4]float64, u float64) [4]float64 {
	return [4]float64{
		engine.Ease(ease, a[0], b[0], u),
		engine.Ease(ease, a[1], b[1], u),
		engine.Ease(ease, a[2], b[2], u),
		engine.Ease(ease, a[3], b[3], u),
	}
}

func boolParam(e *riq.Entity, key string) bool { return e.Float(key, 0) != 0 }

func boolParamDefault(e *riq.Entity, key string, def bool) bool {
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
