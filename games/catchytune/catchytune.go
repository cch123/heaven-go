// Package catchytune ports Catchy Tune's two-player fruit catching flow:
// Bop regions, pre-drop cue sounds, left/right fruit judgments, barely/miss
// fall-through animation, catch smile timing, and whiff feedback.
package catchytune

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
	sideLeft  = 0
	sideRight = 1
	sideBoth  = 2

	whoBoth   = 0
	whoAlalin = 1 // right-side character
	whoPlalin = 2 // left-side character
	whoNone   = 3

	leftAction  = 1 // D-pad side in HS; mapped to F/Left/Up in the engine.
	rightAction = 0 // A-button side in HS; mapped to Space/J/mouse primary.
)

var bgColor = color.NRGBA{R: 0xf2, G: 0xf2, B: 0xf2, A: 0xff}

type bopEvt struct {
	beat, length float64
	who, auto    int
}

type fruitTimings struct {
	startBeat  float64
	spawnBeat  float64
	judgeBeat  float64
	visibleDur float64
	barelyDur  float64
	missDelay  float64
	beatLength float64
}

type fruit struct {
	timing fruitTimings

	sideRight bool
	pineapple bool
	smile     bool
	endSmile  float64

	inst        *kart.Instance
	alive       bool
	dead        bool
	barelyStart float64
	destroyBeat float64
}

type Module struct {
	ctx  *engine.Ctx
	proj kart.Aff

	orangeT, pineappleT *kart.Template

	bops   []bopEvt
	fruits []*fruit

	autoWho   int
	lastPulse float64

	stopCatchLeft, stopCatchRight float64
	startSmile, stopSmile         float64
}

func New() engine.Module { return &Module{autoWho: whoNone, lastPulse: math.Inf(-1)} }

func (m *Module) ID() string { return "catchyTune" }

func (m *Module) Load(ctx *engine.Ctx) error {
	m.ctx = ctx
	if err := ctx.LoadAssets("catchyTune"); err != nil {
		return err
	}
	m.proj = kart.Translate(engine.ScreenW/2, engine.ScreenH/2).Mul(kart.Scale(36, -36))
	m.orangeT = kart.NewTemplate(ctx.Assets, ctx.Role("orangeBase"))
	m.pineappleT = kart.NewTemplate(ctx.Assets, ctx.Role("pineappleBase"))
	ctx.Scene.SetActive(ctx.Role("heartMessage"), false)
	m.setNormalFace()
	return nil
}

func (m *Module) OnEvent(e *riq.Entity) {
	b := e.Beat
	switch e.Datamodel {
	case "catchyTune/bop":
		who := int(e.Float("bop", whoBoth))
		auto := int(e.Float("bopAuto", whoNone))
		m.bops = append(m.bops, bopEvt{beat: b, length: e.Length, who: who, auto: auto})
		m.ctx.At(b, func() { m.autoWho = auto })
		for i := 0; i < int(e.Length); i++ {
			bb := b + float64(i)
			m.ctx.At(bb, func() { m.bopOnce(who, bb) })
		}
	case "catchyTune/orange":
		m.preDropFruit(b, int(e.Float("side", sideLeft)), boolParam(e, "smile"), false, e.Float("endSmile", 2))
	case "catchyTune/pineapple":
		m.preDropFruit(b, int(e.Float("side", sideLeft)), boolParam(e, "smile"), true, e.Float("endSmile", 2))
	}
}

func (m *Module) Ready() {
	sort.Slice(m.bops, func(i, j int) bool { return m.bops[i].beat < m.bops[j].beat })
	sort.Slice(m.fruits, func(i, j int) bool { return m.fruits[i].timing.startBeat < m.fruits[j].timing.startBeat })
}

func (m *Module) OnSwitch(beat float64) {
	m.autoWho = whoNone
	for _, b := range m.bops {
		if b.beat > beat {
			break
		}
		m.autoWho = b.auto
	}
	m.lastPulse = math.Floor(beat)
	m.setNormalFace()
}

func (m *Module) Whiff(beat float64) { m.WhiffAction(beat, rightAction) }

func (m *Module) WhiffAction(beat float64, action int) {
	switch action {
	case leftAction, 3:
		m.catchWhiff(false, beat)
	case rightAction, 2:
		m.catchWhiff(true, beat)
	}
}

func (m *Module) Update(t, beat float64) {
	if m.stopCatchLeft > 0 && m.stopCatchLeft <= beat {
		m.ctx.Scene.PlayState(m.ctx.Role("plalinAnim"), "idle", beat, 0.5)
		m.stopCatchLeft = 0
	}
	if m.stopCatchRight > 0 && m.stopCatchRight <= beat {
		m.ctx.Scene.PlayState(m.ctx.Role("alalinAnim"), "idle", beat, 0.5)
		m.stopCatchRight = 0
	}
	if m.startSmile > 0 && m.startSmile <= beat {
		m.setSmileFace(true)
		m.ctx.Scene.SetActive(m.ctx.Role("heartMessage"), true)
		m.startSmile = 0
	}
	if m.stopSmile > 0 && m.stopSmile <= beat {
		m.setNormalFace()
		m.ctx.Scene.SetActive(m.ctx.Role("heartMessage"), false)
		m.stopSmile = 0
	}

	if p := math.Floor(beat); p > m.lastPulse {
		m.lastPulse = p
		m.bopAuto(p)
	}

	for _, f := range m.fruits {
		if f.dead {
			continue
		}
		if f.destroyBeat > 0 && beat >= f.destroyBeat {
			f.dead = true
		}
	}
}

func (m *Module) Draw(screen *ebiten.Image, t, beat float64) {
	screen.Fill(bgColor)
	sc := m.ctx.Scene
	m.ctx.SampleScene(beat)

	holder, _ := sc.NodeWorld(m.ctx.Role("fruitHolder"))
	for _, f := range m.fruits {
		if f.dead || beat < f.timing.spawnBeat {
			continue
		}
		m.ensureFruitInst(f)
		if f.inst == nil {
			continue
		}
		f.sampleFruitAnim(beat)
		world := holder
		if f.sideRight {
			world = holder.Mul(kart.Scale(-1, 1))
		}
		f.inst.Queue(sc, beat, world, 0)
	}

	sc.Draw(screen, m.proj)
}

func (m *Module) preDropFruit(eventBeat float64, side int, smile, pineapple bool, endSmile float64) {
	tm := fruitSchedule(eventBeat, pineapple)
	if side == sideLeft || side == sideBoth {
		m.scheduleFruit(&fruit{timing: tm, pineapple: pineapple, smile: smile, endSmile: endSmile})
		m.scheduleCueSounds(tm.startBeat, false, pineapple)
	}
	if side == sideRight || side == sideBoth {
		m.scheduleFruit(&fruit{timing: tm, sideRight: true, pineapple: pineapple, smile: smile, endSmile: endSmile})
		m.scheduleCueSounds(tm.startBeat, true, pineapple)
	}
}

func (m *Module) scheduleFruit(f *fruit) {
	m.fruits = append(m.fruits, f)
	m.ctx.At(f.timing.spawnBeat, func() { m.spawnFruit(f) })
	action := leftAction
	if f.sideRight {
		action = rightAction
	}
	m.ctx.ScheduleInputAction(f.timing.judgeBeat, action,
		func(state float64, _ engine.Judgment) { m.catchFruit(f, state) },
		func() { m.missFruit(f) })
}

func (m *Module) spawnFruit(f *fruit) {
	if f.dead || f.alive {
		return
	}
	m.ensureFruitInst(f)
	f.alive = true
}

func (m *Module) ensureFruitInst(f *fruit) {
	if f.inst != nil {
		return
	}
	if f.pineapple {
		if m.pineappleT != nil {
			f.inst = m.pineappleT.NewInstance()
		}
	} else if m.orangeT != nil {
		f.inst = m.orangeT.NewInstance()
	}
}

func (f *fruit) sampleFruitAnim(beat float64) {
	clip := "Fruit/orange bounce"
	if f.pineapple {
		clip = "Fruit/pineapple bounce"
	}
	if f.barelyStart > 0 {
		norm := clamp01((beat - f.barelyStart) / f.timing.barelyDur)
		f.inst.PlayNormalized("", "Fruit/fruit barely", norm)
		return
	}
	norm := clamp01((beat - f.timing.startBeat) / f.timing.visibleDur)
	f.inst.PlayNormalized("", clip, norm)
}

func (m *Module) scheduleCueSounds(startBeat float64, sideRight, pineapple bool) {
	base := fruitSound(sideRight, pineapple)
	if pineapple {
		m.ctx.SoundAt(startBeat+2, base, 1)
		m.ctx.SoundAt(startBeat+4, base, 1)
		m.ctx.SoundAt(startBeat+6, base, 1)
		return
	}
	m.ctx.SoundAt(startBeat+1, base, 1)
	m.ctx.SoundAt(startBeat+2, base, 1)
	m.ctx.SoundAt(startBeat+3, base, 1)
}

func (m *Module) catchFruit(f *fruit, state float64) {
	if f.dead {
		return
	}
	now := m.ctx.Beat()
	if state <= -1 || state >= 1 {
		f.barelyStart = now
		f.destroyBeat = now + f.timing.barelyDur
		m.catchBarely(f.sideRight, now)
		return
	}
	m.ctx.Sound(fruitSound(f.sideRight, f.pineapple) + "Catch")
	m.catchSuccess(f, now)
	f.dead = true
}

func (m *Module) missFruit(f *fruit) {
	if f.dead {
		return
	}
	now := m.ctx.Beat()
	m.catchMiss(f.sideRight, f.pineapple, now)
	f.destroyBeat = f.timing.judgeBeat + f.timing.missDelay
}

func (m *Module) catchSuccess(f *fruit, now float64) {
	anim := "catchOrange"
	if f.pineapple {
		anim = "catchPineapple"
	}
	if f.sideRight {
		m.ctx.Scene.PlayState(m.ctx.Role("alalinAnim"), anim, now, 0.5)
		m.stopCatchRight = f.timing.judgeBeat + 0.9
	} else {
		m.ctx.Scene.PlayState(m.ctx.Role("plalinAnim"), anim, now, 0.5)
		m.stopCatchLeft = f.timing.judgeBeat + 0.9
	}
	if f.smile {
		m.startSmile = f.timing.judgeBeat + 1
		m.stopSmile = f.timing.judgeBeat + f.endSmile
	}
}

func (m *Module) catchMiss(sideRight, pineapple bool, now float64) {
	m.ctx.Sound("fruitThrough")
	anim := "missOrange"
	if pineapple {
		anim = "missPineapple"
	}
	if sideRight {
		m.ctx.Scene.PlayState(m.ctx.Role("alalinAnim"), anim, now, 0.5)
		m.stopCatchRight = now + 0.7
	} else {
		m.ctx.Scene.PlayState(m.ctx.Role("plalinAnim"), anim, now, 0.5)
		m.stopCatchLeft = now + 0.7
	}
}

func (m *Module) catchWhiff(sideRight bool, beat float64) {
	m.ctx.Sound("whiff")
	m.whiffAnim(sideRight, beat)
}

func (m *Module) catchBarely(sideRight bool, beat float64) {
	if sideRight {
		m.ctx.Sound("barely right")
	} else {
		m.ctx.Sound("barely left")
	}
	m.whiffAnim(sideRight, beat)
}

func (m *Module) whiffAnim(sideRight bool, beat float64) {
	if sideRight {
		m.ctx.Scene.PlayState(m.ctx.Role("alalinAnim"), "whiff", beat, 0.5)
		m.stopCatchRight = beat + 0.5
	} else {
		m.ctx.Scene.PlayState(m.ctx.Role("plalinAnim"), "whiff", beat, 0.5)
		m.stopCatchLeft = beat + 0.5
	}
}

func (m *Module) bopAuto(beat float64) {
	m.bopOnce(m.autoWho, beat)
}

func (m *Module) bopOnce(who int, beat float64) {
	m.refreshCatchStops(beat)
	if (who == whoPlalin || who == whoBoth) && m.stopCatchLeft == 0 {
		m.ctx.Scene.PlayState(m.ctx.Role("plalinAnim"), "bop", beat, 0.5)
	}
	if (who == whoAlalin || who == whoBoth) && m.stopCatchRight == 0 {
		m.ctx.Scene.PlayState(m.ctx.Role("alalinAnim"), "bop", beat, 0.5)
	}
}

func (m *Module) refreshCatchStops(beat float64) {
	if m.stopCatchLeft > 0 && m.stopCatchLeft <= beat {
		m.stopCatchLeft = 0
	}
	if m.stopCatchRight > 0 && m.stopCatchRight <= beat {
		m.stopCatchRight = 0
	}
}

func (m *Module) setNormalFace() {
	if m.ctx == nil || m.ctx.Scene == nil {
		return
	}
	m.setSmileFace(false)
}

func (m *Module) setSmileFace(smile bool) {
	for _, root := range []string{m.ctx.Role("plalinAnim"), m.ctx.Role("alalinAnim")} {
		if root == "" {
			continue
		}
		// In Unity these zero-length smile/stopsmile clips run on animator
		// layer 1, so they only toggle the face sprites and must not replace
		// the body bop/catch clip on layer 0.
		m.ctx.Scene.SetActive(root+"/head/normal", !smile)
		m.ctx.Scene.SetActive(root+"/head/smile", smile)
	}
}

func fruitSchedule(eventBeat float64, pineapple bool) fruitTimings {
	beatLength := 4.0
	startOffset := 1.0
	visibleExtra := 2.0
	barelyDur := 1.0
	missDelay := 1.5
	if pineapple {
		beatLength = 8
		startOffset = 2
		visibleExtra = 4
		barelyDur = 2
		missDelay = 3
	}
	startBeat := eventBeat - startOffset
	return fruitTimings{
		startBeat:  startBeat,
		spawnBeat:  eventBeat - 1,
		judgeBeat:  startBeat + beatLength,
		visibleDur: beatLength + visibleExtra,
		barelyDur:  barelyDur,
		missDelay:  missDelay,
		beatLength: beatLength,
	}
}

func fruitSound(sideRight, pineapple bool) string {
	s := "left"
	if sideRight {
		s = "right"
	}
	if pineapple {
		return s + "Pineapple"
	}
	return s + "Orange"
}

func boolParam(e *riq.Entity, key string) bool { return e.Float(key, 0) != 0 }

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
