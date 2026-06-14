// Package chameleon ports Chameleon's fly approach/catch flow, eye tracking,
// background color events, crown toggle, looped fly hum, and count-in cues.
package chameleon

import (
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
	flyClose = iota
	flyFar

	closeAction = 0
	farAction   = 3
)

var (
	defaultTop = [4]float64{0.204, 0.385, 0.064, 1}
	defaultBot = [4]float64{0.070, 0.133, 0.020, 1}
)

type flyEvt struct {
	beat, length float64
	typ          int
	countIn      bool
}

type bgEvt struct {
	beat, length float64
	top0, top1   [4]float64
	bot0, bot1   [4]float64
	ease         int
}

type modEvt struct {
	beat  float64
	crown bool
}

type fly struct {
	inst *kart.Instance
	rng  *rand.Rand

	startBeat, lengthBeat float64
	currentBeat           float64
	typ                   int

	pos, moveCur, moveNext, moveEnd [2]float64
	randomAngle                     float64
	moveFast                        bool
	animOn                          bool
	fall                            bool
	dead                            bool
	stopLoop                        func()
}

type Module struct {
	ctx  *engine.Ctx
	proj kart.Aff

	fliesEv []flyEvt
	bgs     []bgEvt
	mods    []modEvt

	flyT         *kart.Template
	baseFlyPath  string
	chameleon    string
	eyePath      string
	crownPath    string
	gradientPath string
	bgHighPath   string
	bgLowPath    string

	flies      []*fly
	currentFly *fly

	eyeIdx     int
	eyeBaseRad float64
	eyeAngle   float64

	bgImg     *ebiten.Image
	bgPix     []byte
	bgTopLast [4]float64
	bgBotLast [4]float64
}

func New() engine.Module { return &Module{eyeAngle: 15, bgTopLast: [4]float64{-1, -1, -1, -1}} }

func (m *Module) ID() string { return "chameleon" }

func (m *Module) Load(ctx *engine.Ctx) error {
	m.ctx = ctx
	if err := ctx.LoadAssets("chameleon"); err != nil {
		return err
	}
	m.proj = kart.Translate(engine.ScreenW/2, engine.ScreenH/2).Mul(kart.Scale(54, -54))
	m.baseFlyPath = roleOr(ctx, "baseFly", "Fly")
	m.chameleon = roleOr(ctx, "chameleonAnim", "Chameleon")
	m.eyePath = roleOr(ctx, "chameleonEye", "Chameleon/head/eye")
	m.crownPath = roleOr(ctx, "Crown", "Chameleon/head/crown")
	m.gradientPath = roleOr(ctx, "gradient", "BGContainer/Back/Gradient")
	m.bgHighPath = roleOr(ctx, "bgHigh", "BGContainer/Back/BGHigh")
	m.bgLowPath = roleOr(ctx, "bgLow", "BGContainer/Back/BGLow")

	m.flyT = kart.NewTemplate(ctx.Assets, m.baseFlyPath)
	ctx.Scene.SetActive(m.baseFlyPath, false)
	ctx.Scene.PlayDefaultState(m.chameleon, 0, ctx.SecPerBeat(0))
	if idx, ok := ctx.Scene.Index(m.eyePath); ok {
		m.eyeIdx = idx
		m.eyeBaseRad = ctx.Assets.Rig.Nodes[idx].RotZ
	}
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
	case "chameleon/far":
		length := e.Length - 4
		if length < 0 {
			length = 4
		}
		m.fliesEv = append(m.fliesEv, flyEvt{beat: e.Beat, length: length, typ: flyFar, countIn: boolParam(e, "countIn")})
	case "chameleon/close":
		length := e.Length - 4
		if length < 0 {
			length = 4
		}
		m.fliesEv = append(m.fliesEv, flyEvt{beat: e.Beat, length: length, typ: flyClose, countIn: boolParam(e, "countIn")})
	case "chameleon/background appearance":
		m.bgs = append(m.bgs, bgEvt{
			beat: e.Beat, length: e.Length, ease: int(e.Float("ease", 1)),
			top0: colorParam(e, "colorBG1Start", defaultTop),
			top1: colorParam(e, "colorBG1End", defaultTop),
			bot0: colorParam(e, "colorBG2Start", defaultBot),
			bot1: colorParam(e, "colorBG2End", defaultBot),
		})
	case "chameleon/modifiers":
		m.mods = append(m.mods, modEvt{beat: e.Beat, crown: boolDefault(e, "enableCrown", true)})
	}
}

func (m *Module) Ready() {
	sort.SliceStable(m.fliesEv, func(i, j int) bool { return m.fliesEv[i].beat < m.fliesEv[j].beat })
	sort.SliceStable(m.bgs, func(i, j int) bool { return m.bgs[i].beat < m.bgs[j].beat })
	sort.SliceStable(m.mods, func(i, j int) bool { return m.mods[i].beat < m.mods[j].beat })
	for _, ev := range m.mods {
		ev := ev
		m.ctx.At(ev.beat, func() { m.ctx.Scene.SetActive(m.crownPath, ev.crown) })
	}
	for _, ev := range m.fliesEv {
		ev := ev
		m.scheduleFly(ev)
	}
}

func (m *Module) OnSwitch(beat float64) {
	crown := false
	for _, ev := range m.mods {
		if ev.beat > beat {
			break
		}
		crown = ev.crown
	}
	m.ctx.Scene.SetActive(m.crownPath, crown)
	m.ctx.Scene.PlayDefaultState(m.chameleon, beat, m.ctx.SecPerBeat(beat))
}

func (m *Module) Whiff(beat float64) { m.WhiffAction(beat, closeAction) }

func (m *Module) WhiffAction(beat float64, action int) {
	switch action {
	case farAction:
		m.ctx.Scene.PlayState(m.chameleon, "tongueFar", beat, 0.5)
		m.ctx.Sound("blankFar")
	default:
		m.ctx.Scene.PlayState(m.chameleon, "tongueClose", beat, 0.5)
		m.ctx.Sound("blankClose")
	}
}

func (m *Module) Update(_ float64, beat float64) {
	for _, f := range m.flies {
		if !f.dead {
			f.update(beat)
		}
	}
	dst := m.flies[:0]
	for _, f := range m.flies {
		if !f.dead {
			dst = append(dst, f)
		}
	}
	m.flies = dst
	m.updateEye(beat)
}

func (m *Module) Draw(screen *ebiten.Image, _ float64, beat float64) {
	top, bot := m.bgAt(beat)
	m.drawGradient(screen, top, bot)
	sc := m.ctx.Scene
	sc.SetColorOver(m.gradientPath, top)
	sc.SetColorOver(m.bgHighPath, top)
	sc.SetColorOver(m.bgLowPath, bot)
	m.ctx.SampleScene(beat)
	for _, f := range m.flies {
		if !f.dead && f.inst != nil {
			f.inst.Queue(sc, beat, kart.Identity(), 0)
		}
	}
	sc.Draw(screen, m.proj)
}

func (m *Module) scheduleFly(ev flyEvt) {
	if ev.countIn {
		m.ctx.At(ev.beat+ev.length, func() { m.ctx.PlayCommon("three2") })
		m.ctx.At(ev.beat+ev.length+1, func() { m.ctx.PlayCommon("two2") })
		m.ctx.At(ev.beat+ev.length+2, func() { m.ctx.PlayCommon("one2") })
	}
	prefix := flyPrefix(ev.typ)
	m.ctx.SoundAt(ev.beat+ev.length, "fly"+prefix+"1", 1)
	m.ctx.SoundAt(ev.beat+ev.length+1, "fly"+prefix+"2", 1)
	m.ctx.SoundAt(ev.beat+ev.length+2, "fly"+prefix+"3", 1)
	m.ctx.At(ev.beat, func() {
		f := m.newFly(ev.beat, ev.length, ev.typ)
		if f == nil {
			return
		}
		m.flies = append(m.flies, f)
		m.currentFly = f
	})
	target := ev.beat + ev.length + 3
	action := closeAction
	if ev.typ == flyFar {
		action = farAction
	}
	m.ctx.ScheduleInputAction(target, action,
		func(state float64, _ engine.Judgment) {
			if m.currentFly != nil && !m.currentFly.dead && m.currentFly.startBeat == ev.beat && m.currentFly.typ == ev.typ {
				m.catchFly(m.currentFly, state, target)
			}
		},
		func() {
			if m.currentFly != nil && !m.currentFly.dead && m.currentFly.startBeat == ev.beat && m.currentFly.typ == ev.typ {
				m.missFly(m.currentFly, target)
			}
		})
}

func (m *Module) newFly(start, length float64, typ int) *fly {
	if m.flyT == nil {
		return nil
	}
	f := &fly{
		inst:        m.flyT.NewInstance(),
		rng:         rand.New(rand.NewSource(int64(start*1000) + int64(typ+1)*7919)),
		startBeat:   start,
		lengthBeat:  length,
		currentBeat: start,
		typ:         typ,
	}
	if typ == flyFar {
		f.moveCur = [2]float64{-4.5, 5.4}
		f.moveEnd = [2]float64{5.15, 1.6}
	} else {
		f.moveCur = [2]float64{-6, 5.4}
		f.moveEnd = [2]float64{1.5, -0.25}
	}
	f.randomAngle = f.rng.Float64() * 2 * math.Pi
	f.moveNext = [2]float64{math.Cos(f.randomAngle) + f.moveEnd[0], math.Sin(f.randomAngle) + f.moveEnd[1]}
	f.pos = f.moveCur
	if f.inst != nil {
		f.inst.Offset = f.pos
	}
	f.inst.PlayDefaultState("wing", start, m.ctx.SecPerBeat(start))
	f.stopLoop = m.ctx.SoundLoop("fly" + flyPrefix(typ) + "Loop")
	m.ctx.At(start+length, func() { f.stop() })
	m.ctx.At(start+length+3, func() {
		if !f.dead && !f.animOn {
			f.animOn = true
			f.inst.PlayState("", "moveEnd"+flyPrefix(f.typ), start+length+3, 0.5)
			f.pos = f.moveEnd
		}
	})
	m.ctx.At(start+length+6, func() { f.destroy(m) })
	return f
}

func (f *fly) update(beat float64) {
	if f.animOn {
		return
	}
	startPos := clamp01((beat - f.startBeat) / 2)
	if beat < f.startBeat {
		startPos = 0
	}
	if startPos <= 1 {
		if startPos < 0.5 {
			startPos = 0
		} else {
			startPos = startPos*2 - 1
		}
		f.pos = lerp2(f.moveCur, f.moveNext, startPos)
	} else {
		dur := 0.5
		if f.moveFast {
			dur = 0.17
		}
		curPos := (beat - f.currentBeat) / dur
		if curPos > 1 {
			if (beat-f.startBeat)/2 > 1.5 {
				f.moveFast = true
			}
			f.currentBeat = beat
			f.moveCur = f.moveNext
			f.randomAngle += 0.7 + f.rng.Float64()*(1.3*math.Pi-0.7)
			radius := 1.0
			if f.moveFast {
				radius = 0.5
			}
			f.moveNext = [2]float64{
				f.moveEnd[0] + radius*math.Cos(f.randomAngle),
				f.moveEnd[1] + radius*math.Sin(f.randomAngle),
			}
			f.pos = f.moveCur
		} else {
			u := easeInOutSine(clamp01(curPos))
			f.pos = lerp2(f.moveCur, f.moveNext, u)
		}
	}
	if f.inst != nil {
		f.inst.Offset = f.pos
	}
}

func (m *Module) catchFly(f *fly, state, target float64) {
	prefix := flyPrefix(f.typ)
	f.animOn = true
	m.ctx.Scene.PlayState(m.chameleon, "tongue"+prefix, target, 0.5)
	if state <= -1 || state >= 1 {
		f.fall = true
		f.inst.PlayState("", "fall"+prefix, target, 0.5)
		return
	}
	m.currentFly = nil
	m.ctx.Sound("eatCatch")
	m.ctx.SoundAt(f.startBeat+f.lengthBeat+3.25, "eatGulp", 1)
	f.inst.PlayState("wing", "idle", target, 0.5)
	f.inst.PlayState("", "catch"+prefix, target, 0.5)
	m.ctx.At(f.startBeat+f.lengthBeat+3.25, func() {
		m.ctx.Scene.PlayState(m.chameleon, "gurp", f.startBeat+f.lengthBeat+3.25, 0.5)
	})
}

func (m *Module) missFly(f *fly, target float64) {
	f.animOn = true
	f.inst.PlayState("", "gone"+flyPrefix(f.typ), target, 0.5)
}

func (f *fly) destroy(m *Module) {
	f.dead = true
	f.stop()
	if m.currentFly == f {
		m.currentFly = nil
	}
}

func (f *fly) stop() {
	if f.stopLoop != nil {
		f.stopLoop()
		f.stopLoop = nil
	}
}

func flyPrefix(typ int) string {
	if typ == flyFar {
		return "Far"
	}
	return "Close"
}

func (m *Module) updateEye(beat float64) {
	target := 15.0
	if f := m.currentFly; f != nil && !f.dead && beat >= f.startBeat+1 {
		eye := m.eyeWorld(beat)
		relX, relY := eye[0]-f.pos[0], eye[1]-f.pos[1]
		target = 165 + math.Atan2(relY, relX)*180/math.Pi
		if f.fall {
			if f.typ == flyFar && target < -70 {
				target = -70
			}
			if f.typ == flyClose && target < -100 {
				target = -100
			}
		}
		m.eyeAngle += (target - m.eyeAngle) * 0.05
	} else {
		m.eyeAngle = lerpAngle(m.eyeAngle, target, 0.1)
	}
	if m.eyeIdx != 0 {
		m.ctx.Scene.SetSpinIdx(m.eyeIdx, m.eyeAngle*math.Pi/180-m.eyeBaseRad)
	}
}

func (m *Module) eyeWorld(beat float64) [2]float64 {
	m.ctx.Scene.Sample(beat)
	if aff, ok := m.ctx.Scene.NodeWorld(m.eyePath); ok {
		x, y := aff.Apply(0, 0)
		return [2]float64{x, y}
	}
	return [2]float64{-3.2, 0.8}
}

func (m *Module) bgAt(beat float64) ([4]float64, [4]float64) {
	top, bot := defaultTop, defaultBot
	for _, ev := range m.bgs {
		if ev.beat > beat {
			break
		}
		u := 1.0
		if ev.length > 0 && beat < ev.beat+ev.length {
			u = clamp01((beat - ev.beat) / ev.length)
		}
		top, bot = easeColor(ev.ease, ev.top0, ev.top1, u), easeColor(ev.ease, ev.bot0, ev.bot1, u)
	}
	return top, bot
}

func (m *Module) drawGradient(screen *ebiten.Image, top, bot [4]float64) {
	h := screen.Bounds().Dy()
	if m.bgImg == nil || m.bgImg.Bounds().Dy() != h {
		m.bgImg = ebiten.NewImage(1, h)
		m.bgPix = make([]byte, h*4)
		m.bgTopLast, m.bgBotLast = [4]float64{-1, -1, -1, -1}, [4]float64{-1, -1, -1, -1}
	}
	if m.bgTopLast != top || m.bgBotLast != bot {
		m.bgTopLast, m.bgBotLast = top, bot
		for y := 0; y < h; y++ {
			u := float64(y) / float64(max(1, h-1))
			c := [4]float64{
				top[0]*(1-u) + bot[0]*u,
				top[1]*(1-u) + bot[1]*u,
				top[2]*(1-u) + bot[2]*u,
				top[3]*(1-u) + bot[3]*u,
			}
			r := rgba(c)
			i := y * 4
			m.bgPix[i+0], m.bgPix[i+1], m.bgPix[i+2], m.bgPix[i+3] = r.R, r.G, r.B, r.A
		}
		m.bgImg.WritePixels(m.bgPix)
	}
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(float64(screen.Bounds().Dx()), 1)
	screen.DrawImage(m.bgImg, op)
}

func lerp2(a, b [2]float64, u float64) [2]float64 {
	return [2]float64{a[0] + (b[0]-a[0])*u, a[1] + (b[1]-a[1])*u}
}

func easeInOutSine(u float64) float64 { return -(math.Cos(math.Pi*u) - 1) / 2 }

func lerpAngle(a, b, t float64) float64 {
	d := math.Mod(b-a+180, 360) - 180
	return a + d*t
}

func easeColor(ease int, a, b [4]float64, u float64) [4]float64 {
	return [4]float64{
		engine.Ease(ease, a[0], b[0], u),
		engine.Ease(ease, a[1], b[1], u),
		engine.Ease(ease, a[2], b[2], u),
		engine.Ease(ease, a[3], b[3], u),
	}
}

func rgba(c [4]float64) color.NRGBA {
	return color.NRGBA{
		R: byte(clamp01(c[0]) * 255),
		G: byte(clamp01(c[1]) * 255),
		B: byte(clamp01(c[2]) * 255),
		A: byte(clamp01(c[3]) * 255),
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

func boolParam(e *riq.Entity, key string) bool { return e.Float(key, 0) != 0 }

func boolDefault(e *riq.Entity, key string, def bool) bool {
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
	mm, ok := v.(map[string]any)
	if !ok {
		return def
	}
	get := func(k string, d float64) float64 {
		if n, ok := mm[k].(float64); ok {
			return n
		}
		return d
	}
	return [4]float64{get("r", def[0]), get("g", def[1]), get("b", def[2]), get("a", def[3])}
}
