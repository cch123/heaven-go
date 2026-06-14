// Package mannequinfactory ports Mannequin Factory's two-step head timing,
// sign text, background color, and per-head template animations.
package mannequinfactory

import (
	"math/rand"
	"sort"
	"strconv"

	"github.com/hajimehoshi/ebiten/v2"

	"hsdemo/engine"
	"hsdemo/kart"
	"hsdemo/kmdata"
	"hsdemo/riq"
)

const (
	cueAligned = iota
	cueMisaligned

	actionStamp = 0
	actionSlap  = 1
)

var defaultBG = [4]float64{0.97, 0.94, 0.51, 1}

type bgEvt struct {
	beat, length float64
	from, to     [4]float64
	ease         int
}

type colorEase struct {
	beat, length float64
	from, to     [4]float64
	ease         int
}

type headEvt struct {
	beat float64
	cue  int
}

type headInst struct {
	start    float64
	turn     int
	inst     *kart.Instance
	deadBeat float64
}

type Module struct {
	ctx  *engine.Ctx
	proj kart.Aff

	hand, stamp, bg, sign, headRoot string
	headT                           *kart.Template
	headComp                        kmdata.Component
	headSprites                     []string
	eyeSprites                      []string

	heads    []*headInst
	events   []headEvt
	bgEvents []bgEvt
	bgEase   colorEase
	rng      *rand.Rand
}

func New() engine.Module { return &Module{} }

func (m *Module) ID() string { return "mannequinFactory" }

func (m *Module) Load(ctx *engine.Ctx) error {
	m.ctx = ctx
	if err := ctx.LoadAssets("mannequinFactory"); err != nil {
		return err
	}
	if err := ctx.Assets.ApplyTexts(); err != nil {
		return err
	}
	m.proj = kart.Translate(engine.ScreenW/2, engine.ScreenH/2).Mul(kart.Scale(54, -54))
	m.hand = roleOr(ctx, "HandAnim", "HandParent")
	m.stamp = roleOr(ctx, "StampAnim", "EyeStamper")
	m.bg = roleOr(ctx, "bg", "Background/BGColor")
	m.sign = roleOr(ctx, "SignText", "Background/Sign/SignText")
	m.headRoot = roleOr(ctx, "MannequinHeadObject", "MannequinHeadHolder/MannequinHead")
	m.headT = kart.NewTemplate(ctx.Assets, m.headRoot)
	m.headComp = ctx.Assets.Extra.Components["head"]
	m.headSprites = m.headComp.SpriteArrays["heads"]
	m.eyeSprites = m.headComp.SpriteArrays["eyes"]
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
	case "mannequinFactory/headOut", "mannequinFactory/misalignedHeadOut", "mannequinFactory/randomHeadOut":
		cue := cueAligned
		switch e.Datamodel {
		case "mannequinFactory/misalignedHeadOut":
			cue = cueMisaligned
		case "mannequinFactory/randomHeadOut":
			cue = m.rng.Intn(2)
		}
		m.events = append(m.events, headEvt{beat: e.Beat, cue: cue})
		m.scheduleHeadOutSFX(e.Beat, cue, 0)
		b := e.Beat
		m.ctx.At(b, func() { m.spawnHead(b, cue) })
	case "mannequinFactory/changeText":
		b, txt := e.Beat, e.Str("text", "Training in progress!")
		m.ctx.At(b, func() { _ = m.ctx.Assets.SetText(m.sign, txt) })
	case "mannequinFactory/bgColor":
		m.bgEvents = append(m.bgEvents, bgEvt{
			beat: e.Beat, length: e.Length,
			from: colorParam(e, "colorStart", defaultBG),
			to:   colorParam(e, "colorEnd", defaultBG),
			ease: intParam(e, "ease", 0),
		})
	}
}

func (m *Module) Ready() {
	sort.Slice(m.events, func(i, j int) bool { return m.events[i].beat < m.events[j].beat })
	sort.Slice(m.bgEvents, func(i, j int) bool { return m.bgEvents[i].beat < m.bgEvents[j].beat })
}

func (m *Module) OnSwitch(beat float64) {
	m.reset(beat)
	for _, ev := range m.events {
		if ev.beat >= beat {
			break
		}
		if ev.beat+2.75 > beat {
			m.spawnHead(ev.beat, ev.cue)
		}
	}
	m.persistBG(beat)
}

func (m *Module) Whiff(beat float64) { m.WhiffAction(beat, actionStamp) }

func (m *Module) WhiffAction(beat float64, action int) {
	switch action {
	case actionSlap:
		if !m.animPlaying(m.hand, beat, "SlapEmpty", "SlapJust") {
			m.ctx.Scene.PlayState(m.hand, "SlapEmpty", beat, 0.3)
		}
	case actionStamp:
		if !m.animPlaying(m.stamp, beat, "StampEmpty", "StampJust") {
			m.ctx.Scene.PlayState(m.stamp, "StampEmpty", beat, 0.3)
		}
	}
}

func (m *Module) Update(float64, float64) {}

func (m *Module) Draw(screen *ebiten.Image, _ float64, beat float64) {
	m.applyBG(beat)
	m.ctx.SampleScene(beat)
	alive := m.heads[:0]
	for _, h := range m.heads {
		if beat <= h.deadBeat {
			h.inst.Queue(m.ctx.Scene, beat, kart.Identity(), 0)
			alive = append(alive, h)
		}
	}
	m.heads = alive
	m.ctx.Scene.Draw(screen, m.proj)
}

func (m *Module) reset(beat float64) {
	m.heads = nil
	m.bgEase = colorEase{from: defaultBG, to: defaultBG}
	m.ctx.Scene.PlayDefaultState(m.hand, beat, m.ctx.SecPerBeat(beat))
	m.ctx.Scene.PlayDefaultState(m.stamp, beat, m.ctx.SecPerBeat(beat))
	m.ctx.Scene.SetActive(m.headRoot, false)
}

func (m *Module) spawnHead(beat float64, cue int) {
	if m.headT == nil || len(m.headSprites) < 2 {
		return
	}
	turn := 1
	if cue == cueMisaligned {
		turn = 0
	}
	h := &headInst{
		start: beat, turn: turn, inst: m.headT.NewInstance(), deadBeat: beat + 8,
	}
	h.inst.PlayDefaultState("", beat, m.ctx.SecPerBeat(beat))
	h.inst.SetSprite("HeadParent/Head", m.headSprites[turn])
	h.inst.SetActive("HeadParent/Eyes", false)
	m.heads = append(m.heads, h)

	m.atOrNow(beat+1, func() { h.inst.PlayState("", "Move1", beat+1, 0.3) })
	m.atOrNow(beat+3, func() {
		if m.ctx.App.Autoplay {
			h.inst.PlayState("", "Move2", beat+3, 0.3)
		}
	})
	m.scheduleSlap(h)
	m.atOrNow(beat+4, func() { m.scheduleStamp(h) })
}

func (m *Module) atOrNow(beat float64, fn func()) {
	if m.ctx.Beat() >= beat {
		fn()
		return
	}
	m.ctx.At(beat, fn)
}

func (m *Module) scheduleSlap(h *headInst) {
	target := h.start + 3
	if h.turn == 0 {
		m.ctx.ScheduleInputActionCond(target, actionSlap, func() bool { return !m.animPlaying(m.hand, m.ctx.Beat(), "SlapEmpty", "SlapJust") },
			func(state float64, _ engine.Judgment) { m.slapJust(h, state) },
			func() { h.inst.PlayState("", "Move2", h.start+3, 0.3) })
		return
	}
	in := m.ctx.ScheduleInputActionCond(target, actionSlap, func() bool { return !m.animPlaying(m.hand, m.ctx.Beat(), "SlapEmpty", "SlapJust") },
		func(state float64, _ engine.Judgment) { m.slapUnjust(h, state) },
		func() { h.inst.PlayState("", "Move2", h.start+3, 0.3) })
	// ScheduleUserInput compatibility: wrong-action windows are player-only
	// callbacks, not autoplay hits and not direct accuracy entries.
	in.NoScore = true
	in.NoAutoplay = true
}

func (m *Module) scheduleStamp(h *headInst) {
	target := h.start + 5
	onMiss := func() {
		h.inst.PlayState("", "Move3", target, 0.3)
		m.ctx.At(h.start+6.5, func() {
			m.ctx.Sound("miss")
			h.inst.PlayState("", "Miss", h.start+6.5, 0.3)
			h.deadBeat = h.start + 7.2
		})
	}
	if h.turn == 1 {
		m.ctx.ScheduleInputActionCond(target, actionStamp, func() bool { return !m.animPlaying(m.stamp, m.ctx.Beat(), "StampEmpty", "StampJust") },
			func(state float64, _ engine.Judgment) { m.stampJust(h, state) }, onMiss)
		return
	}
	in := m.ctx.ScheduleInputActionCond(target, actionStamp, func() bool { return !m.animPlaying(m.stamp, m.ctx.Beat(), "StampEmpty", "StampJust") },
		func(state float64, _ engine.Judgment) { m.stampUnjust(h, state) }, onMiss)
	in.NoScore = true
	in.NoAutoplay = true
}

func (m *Module) slapJust(h *headInst, state float64) {
	m.slapHit(h, state)
	if h.turn >= 0 && h.turn < len(m.headSprites) {
		h.inst.SetSprite("HeadParent/Head", m.headSprites[h.turn])
	}
}

func (m *Module) slapUnjust(h *headInst, state float64) {
	h.inst.SetScale("HeadParent/Eyes", -1, 1)
	h.inst.SetScale("HeadParent/Head", -1, 1)
	if len(m.headSprites) > 0 {
		h.inst.SetSprite("HeadParent/Head", m.headSprites[0])
	}
	m.ctx.ScoreMiss()
	m.slapHit(h, state)
}

func (m *Module) slapHit(h *headInst, state float64) {
	if state >= 1 || state <= -1 {
		m.ctx.PlayCommon("nearMiss")
	}
	h.turn++
	m.ctx.Sound("slap")
	m.ctx.Scene.PlayState(m.hand, "SlapJust", m.ctx.Beat(), 0.3)
	h.inst.PlayState("", "Slapped", m.ctx.Beat(), 0.3)
}

func (m *Module) stampHit(h *headInst, state float64) {
	if state >= 1 || state <= -1 {
		m.ctx.PlayCommon("nearMiss")
	}
	h.inst.PlayState("", "Stamp", m.ctx.Beat(), 0.3)
	m.ctx.Scene.PlayState(m.stamp, "StampJust", m.ctx.Beat(), 0.3)
	m.ctx.Sound("eyes")
	h.inst.SetActive("HeadParent/Eyes", true)
}

func (m *Module) stampJust(h *headInst, state float64) {
	m.stampHit(h, state)
	m.ctx.SoundAt(h.start+6, "claw1", 1)
	m.ctx.SoundAt(h.start+6.5, "claw2", 1)
	m.ctx.At(h.start+5.75, func() { h.inst.PlayState("", "Grabbed1", h.start+5.75, 0.3) })
	m.ctx.At(h.start+6, func() { h.inst.PlayState("", "Grabbed2", h.start+6, 0.3) })
	h.deadBeat = h.start + 7.2
}

func (m *Module) stampUnjust(h *headInst, state float64) {
	m.stampHit(h, state)
	if len(m.eyeSprites) > 1 {
		h.inst.SetSprite("HeadParent/Eyes", m.eyeSprites[1])
	}
	m.ctx.At(h.start+6, func() {
		m.ctx.Sound("miss")
		h.inst.PlayState("", "StampMiss", h.start+6, 0.3)
		h.deadBeat = h.start + 7.2
	})
}

func (m *Module) scheduleHeadOutSFX(beat float64, cue int, fromBeat float64) {
	type snd struct {
		beat float64
		name string
	}
	sounds := []snd{
		{beat, "drum"},
		{beat + 0.5, "drum"},
		{beat + 1.5, "drum"},
		{beat + 2, "drum"},
		{beat + 5, "whoosh"},
	}
	if cue == cueAligned {
		for i := 0; i < 7; i++ {
			sounds = append(sounds, snd{beat + 3 + float64(i)*0.1667, "drumroll" + strconv.Itoa(i+1)})
		}
	} else {
		sounds = append(sounds,
			snd{beat + 0.75, "drum"},
			snd{beat + 1, "drum"},
			snd{beat + 3, "whoosh"},
		)
	}
	sort.Slice(sounds, func(i, j int) bool { return sounds[i].beat < sounds[j].beat })
	for _, s := range sounds {
		if s.beat >= fromBeat {
			m.ctx.SoundAt(s.beat, s.name, 1)
		}
	}
}

func (m *Module) animPlaying(root string, beat float64, names ...string) bool {
	state, playing := m.ctx.Scene.StateInfo(root, beat)
	if !playing {
		return false
	}
	for _, n := range names {
		if state == n {
			return true
		}
	}
	return false
}

func (m *Module) persistBG(beat float64) {
	m.bgEase = colorEase{from: defaultBG, to: defaultBG}
	for _, ev := range m.bgEvents {
		if ev.beat >= beat {
			break
		}
		m.bgEase = colorEase{beat: ev.beat, length: ev.length, from: ev.from, to: ev.to, ease: ev.ease}
	}
}

func (m *Module) applyBG(beat float64) {
	m.persistBG(beat + 1e-9)
	m.ctx.Scene.SetColorOver(m.bg, m.bgEase.at(beat))
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

func intParam(e *riq.Entity, key string, def int) int { return int(e.Float(key, float64(def))) }

func colorParam(e *riq.Entity, key string, def [4]float64) [4]float64 {
	v, ok := e.Data[key]
	if !ok {
		return def
	}
	m, ok := v.(map[string]any)
	if !ok {
		return def
	}
	return [4]float64{num(m["r"], def[0]), num(m["g"], def[1]), num(m["b"], def[2]), num(m["a"], def[3])}
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

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
