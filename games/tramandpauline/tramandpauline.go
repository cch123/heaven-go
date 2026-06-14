// Package tramandpauline ports Tram & Pauline's dual-trampoline jump,
// transformation, curtain, audience reaction, and smoke effects.
package tramandpauline

import (
	"image/color"
	"math"
	"math/rand"
	"sort"

	"github.com/hajimehoshi/ebiten/v2"

	"hsdemo/engine"
	"hsdemo/kart"
	"hsdemo/kmdata"
	"hsdemo/riq"
)

const (
	whoPauline = 0
	whoTram    = 1
	whoBoth    = 2

	leftAction  = 1 // HS InputAction_Left: pad directions / left touch / west baton.
	rightAction = 2 // HS InputAction_Right: east pad / right touch / east baton.
)

var bgColor = color.NRGBA{R: 0xa5, G: 0xaa, B: 0xe9, A: 0xff}

type prepareEvt struct {
	beat float64
	who  int
}

type jumpEvt struct {
	beat  float64
	who   int
	react bool
}

type shapeEvt struct {
	beat          float64
	tram, pauline bool
}

type curtainEvt struct {
	beat, length float64
	ease         int
	rise         bool
}

type particleBurst struct {
	startBeat float64
	origin    string
	seed      int
}

type animalKid struct {
	mod *Module

	label          string
	rootBodyPath   string
	bodyPath       string
	trampolinePath string
	particlePath   string
	smokePath      string

	jumpHeight     float64
	jumpHeightIdle float64
	prepareHeight  float64

	jumpBeat    float64
	prepareBeat float64
	preparing   bool
	isFox       bool
	isBarely    bool
	bodyState   string
}

type Module struct {
	ctx  *engine.Ctx
	proj kart.Aff

	prepares []prepareEvt
	jumps    []jumpEvt
	shapes   []shapeEvt
	curtains []curtainEvt

	tram    *animalKid
	pauline *animalKid

	curtainPath string
	audience    string

	curtainBeat   float64
	curtainLength float64
	curtainEase   int
	goingUp       bool

	bursts []particleBurst
}

func New() engine.Module { return &Module{curtainBeat: -1, curtainLength: 1, goingUp: true} }

func (m *Module) ID() string { return "tramAndPauline" }

func (m *Module) Load(ctx *engine.Ctx) error {
	m.ctx = ctx
	if err := ctx.LoadAssets("tramAndPauline"); err != nil {
		return err
	}
	m.proj = kart.Translate(engine.ScreenW/2, engine.ScreenH/2).Mul(kart.Scale(54, -54))
	m.curtainPath = roleOr(ctx, "curtainAnim", "BgObjects/Curtain")
	m.audience = roleOr(ctx, "audienceAnim", "BgObjects/audience")
	m.pauline = m.newKid("Pauline", "kid0", "PaulineObjects/PaulineRoot", "PaulineObjects/PaulineRoot/Pauline", "PaulineObjects/TrampolineRight")
	m.tram = m.newKid("Tram", "kid1", "TramObjects/TramRoot", "TramObjects/TramRoot/Tram", "TramObjects/TrampolineLeft")

	m.ctx.Scene.PlayDefaultState(m.curtainPath, 0, ctx.SecPerBeat(0))
	m.ctx.Scene.PlayDefaultState(m.audience, 0, ctx.SecPerBeat(0))
	m.pauline.setTransform(true)
	m.tram.setTransform(true)
	m.pauline.playBody("FoxIdle", 0, ctx.SecPerBeat(0))
	m.tram.playBody("FoxIdle", 0, ctx.SecPerBeat(0))
	return nil
}

func (m *Module) newKid(label, componentKey, rootFallback, bodyFallback, trampolineFallback string) *animalKid {
	kid := &animalKid{
		mod:            m,
		label:          label,
		rootBodyPath:   rootFallback,
		bodyPath:       bodyFallback,
		trampolinePath: trampolineFallback,
		jumpHeight:     3,
		jumpHeightIdle: 0.8,
		prepareHeight:  0.5,
		jumpBeat:       math.Inf(-1),
		prepareBeat:    math.Inf(-1),
		isFox:          true,
	}
	if comp, ok := m.ctx.Assets.Extra.Components[componentKey]; ok {
		kid.rootBodyPath = refDefault(comp, "rootBody", kid.rootBodyPath)
		kid.bodyPath = refDefault(comp, "bodyAnim", kid.bodyPath)
		kid.trampolinePath = refDefault(comp, "trampolineAnim", kid.trampolinePath)
		kid.particlePath = refDefault(comp, "transformParticle", "")
		kid.smokePath = refDefault(comp, "smokeParticle", "")
		kid.jumpHeight = numDefault(comp.Nums, "jumpHeight", kid.jumpHeight)
		kid.jumpHeightIdle = numDefault(comp.Nums, "jumpHeightIdle", kid.jumpHeightIdle)
		kid.prepareHeight = numDefault(comp.Nums, "prepareHeight", kid.prepareHeight)
	}
	m.ctx.Scene.PlayDefaultState(kid.bodyPath, 0, m.ctx.SecPerBeat(0))
	m.ctx.Scene.PlayDefaultState(kid.trampolinePath, 0, m.ctx.SecPerBeat(0))
	return kid
}

func roleOr(ctx *engine.Ctx, key, fallback string) string {
	if p := ctx.Role(key); p != "" {
		return p
	}
	return fallback
}

func refDefault(c kmdata.Component, key, def string) string {
	if v := c.Refs[key]; v != "" {
		return v
	}
	return def
}

func numDefault(nums map[string]float64, key string, def float64) float64 {
	if v, ok := nums[key]; ok {
		return v
	}
	return def
}

func (m *Module) OnEvent(e *riq.Entity) {
	switch e.Datamodel {
	case "tramAndPauline/prepare":
		m.prepares = append(m.prepares, prepareEvt{beat: e.Beat, who: int(e.Float("who", whoPauline))})
	case "tramAndPauline/tram":
		m.jumps = append(m.jumps, jumpEvt{beat: e.Beat, who: whoTram, react: boolParam(e, "toggle")})
	case "tramAndPauline/pauline":
		m.jumps = append(m.jumps, jumpEvt{beat: e.Beat, who: whoPauline, react: boolParam(e, "toggle")})
	case "tramAndPauline/shape":
		m.shapes = append(m.shapes, shapeEvt{
			beat: e.Beat, tram: boolDefault(e, "tram", true), pauline: boolDefault(e, "pauline", true),
		})
	case "tramAndPauline/curtains":
		m.curtains = append(m.curtains, curtainEvt{
			beat: e.Beat, length: e.Length, ease: int(e.Float("ease", 0)), rise: boolParam(e, "toggle"),
		})
	}
}

func (m *Module) Ready() {
	sort.SliceStable(m.prepares, func(i, j int) bool { return m.prepares[i].beat < m.prepares[j].beat })
	sort.SliceStable(m.jumps, func(i, j int) bool { return m.jumps[i].beat < m.jumps[j].beat })
	sort.SliceStable(m.shapes, func(i, j int) bool { return m.shapes[i].beat < m.shapes[j].beat })
	sort.SliceStable(m.curtains, func(i, j int) bool { return m.curtains[i].beat < m.curtains[j].beat })

	for _, ev := range m.shapes {
		ev := ev
		m.ctx.At(ev.beat, func() { m.setTransformation(ev.tram, ev.pauline) })
	}
	for _, ev := range m.curtains {
		ev := ev
		m.ctx.At(ev.beat, func() { m.setCurtain(ev) })
	}
	for _, ev := range m.prepares {
		ev := ev
		m.ctx.At(ev.beat, func() { m.prepare(ev.beat, ev.who, false) })
	}
	for _, ev := range m.jumps {
		ev := ev
		m.scheduleJump(ev)
	}
}

func (m *Module) OnSwitch(beat float64) {
	m.persistCurtain(beat)
	m.persistTransformation(beat)
	m.persistPrepare(beat)
	m.ctx.Scene.PlayDefaultState(m.audience, beat, m.ctx.SecPerBeat(beat))
}

func (m *Module) Whiff(beat float64) { m.WhiffAction(beat, rightAction) }

func (m *Module) WhiffAction(_ float64, _ int) {}

func (m *Module) Update(_ float64, beat float64) {
	m.updateCurtain(beat)
	m.pauline.update(beat)
	m.tram.update(beat)
	alive := m.bursts[:0]
	for _, b := range m.bursts {
		if beat-b.startBeat <= 0.6 {
			alive = append(alive, b)
		}
	}
	m.bursts = alive
}

func (m *Module) Draw(screen *ebiten.Image, _ float64, beat float64) {
	screen.Fill(bgColor)
	sc := m.ctx.Scene
	m.ctx.SampleScene(beat)
	m.queueBursts(sc, beat)
	sc.Draw(screen, m.proj)
}

func (m *Module) scheduleJump(ev jumpEvt) {
	m.ctx.At(ev.beat, func() {
		switch ev.who {
		case whoTram:
			m.ctx.Sound(randomJumpSound("jumpL"))
			m.tram.jump(ev.beat)
		case whoPauline:
			m.ctx.Sound(randomJumpSound("jumpR"))
			m.pauline.jump(ev.beat)
		case whoBoth:
			m.ctx.Sound(randomJumpSound("jumpL"))
			m.ctx.Sound(randomJumpSound("jumpR"))
			m.tram.jump(ev.beat)
			m.pauline.jump(ev.beat)
		}
	})

	target := jumpTarget(ev.beat)
	switch ev.who {
	case whoTram:
		m.scheduleTransformInput(target, leftAction, m.tram, "transformTram", ev.react)
	case whoPauline:
		m.scheduleTransformInput(target, rightAction, m.pauline, "transformPauline", ev.react)
	case whoBoth:
		m.scheduleTransformInput(target, leftAction, m.tram, "transformTram", ev.react)
		m.scheduleTransformInput(target, rightAction, m.pauline, "transformPauline", ev.react)
	}
}

func (m *Module) scheduleTransformInput(target float64, action int, kid *animalKid, sound string, react bool) {
	m.ctx.ScheduleInputAction(target, action, func(state float64, _ engine.Judgment) {
		now := m.ctx.Beat()
		barely := state >= 1 || state <= -1
		kid.transform(barely, now)
		if barely {
			m.ctx.Sound("common_miss")
			return
		}
		m.ctx.Sound(sound)
		if react {
			m.ctx.At(target+1, func() { m.ctx.Scene.PlayState(m.audience, "Happy", target+1, 0.5) })
		}
	}, func() {})
}

func (m *Module) prepare(beat float64, who int, inactive bool) {
	if who == whoPauline || who == whoBoth {
		m.pauline.prepare(beat, inactive)
	}
	if who == whoTram || who == whoBoth {
		m.tram.prepare(beat, inactive)
	}
}

func (m *Module) setTransformation(tram, pauline bool) {
	m.tram.setTransform(tram)
	m.pauline.setTransform(pauline)
}

func (m *Module) setCurtain(ev curtainEvt) {
	if ev.length <= 0 {
		ev.length = 1
	}
	m.goingUp = ev.rise
	m.curtainBeat = ev.beat
	m.curtainLength = ev.length
	m.curtainEase = ev.ease
}

func (m *Module) updateCurtain(beat float64) {
	if m.curtainBeat < 0 {
		return
	}
	p := clamp01((beat - m.curtainBeat) / math.Max(0.001, m.curtainLength))
	from, to := 0.0, 1.0
	if m.goingUp {
		from, to = 1, 0
	}
	m.ctx.Scene.PlayFrozen(m.curtainPath, "Curtain", engine.Ease(m.curtainEase, from, to, p))
}

func (m *Module) persistCurtain(beat float64) {
	for _, ev := range m.curtains {
		if ev.beat >= beat {
			break
		}
		m.setCurtain(ev)
	}
}

func (m *Module) persistTransformation(beat float64) {
	tramFox, paulineFox := true, true
	baseBeat := 0.0
	for _, ev := range m.shapes {
		if ev.beat >= beat {
			break
		}
		baseBeat = ev.beat
		tramFox = ev.tram
		paulineFox = ev.pauline
	}
	tramJumps, paulineJumps := 0, 0
	for _, ev := range m.jumps {
		if ev.beat < baseBeat || ev.beat+1 >= beat {
			continue
		}
		switch ev.who {
		case whoTram:
			tramJumps++
		case whoPauline:
			paulineJumps++
		case whoBoth:
			tramJumps++
			paulineJumps++
		}
	}
	if tramJumps%2 != 0 {
		tramFox = !tramFox
	}
	if paulineJumps%2 != 0 {
		paulineFox = !paulineFox
	}
	m.setTransformation(tramFox, paulineFox)
}

func (m *Module) persistPrepare(beat float64) {
	lastBeat := math.Inf(-1)
	lastWho := -1
	for _, ev := range m.prepares {
		if ev.beat < beat && ev.beat > lastBeat {
			lastBeat, lastWho = ev.beat, ev.who
		}
	}
	for _, ev := range m.jumps {
		if ev.beat < beat && ev.beat > lastBeat {
			lastBeat, lastWho = ev.beat, -1
		}
	}
	if lastWho >= 0 {
		m.prepare(lastBeat, lastWho, true)
	}
}

func (k *animalKid) jump(beat float64) {
	k.jumpBeat = beat
	k.preparing = false
	if k.isBarely {
		k.playBody("JumpBarely", beat, k.mod.ctx.SecPerBeat(beat))
	} else if k.isFox {
		k.playBody("JumpFox", beat, k.mod.ctx.SecPerBeat(beat))
	} else {
		k.playBody("JumpHuman", beat, k.mod.ctx.SecPerBeat(beat))
	}
	k.mod.ctx.Scene.PlayState(k.trampolinePath, "Jump", beat, 0.25)
}

func (k *animalKid) prepare(beat float64, inactive bool) {
	if k.preparing {
		return
	}
	k.prepareBeat = beat
	k.preparing = true
	state := "Prepare"
	if k.isBarely {
		state = "PrepareBarely"
	} else if !k.isFox {
		state = "PrepareHuman"
	}
	if inactive {
		k.mod.ctx.Scene.PlayFrozen(k.bodyPath, state, 1)
		k.bodyState = state
		return
	}
	k.playBody(state, beat, 0.15)
}

func (k *animalKid) transform(barely bool, beat float64) {
	k.isBarely = barely
	state := "TransformFox"
	if barely {
		state = "TransformBarely"
	} else if k.isFox {
		state = "TransformHuman"
	}
	k.playBody(state, beat, 0.15)
	k.mod.spawnBurst(k, beat)
	k.isFox = !k.isFox
}

func (k *animalKid) setTransform(fox bool) {
	k.isFox = fox
	k.isBarely = false
	k.playIdle(k.mod.ctx.Beat())
}

func (k *animalKid) update(beat float64) {
	newY := 0.0
	up := beat - k.jumpBeat
	down := beat - (k.jumpBeat + 1)
	switch {
	case up >= 0 && up <= 1:
		newY = easeOutQuad(0, k.jumpHeight, up)
	case down >= 0 && down <= 1:
		newY = easeInQuad(k.jumpHeight, 0, down)
	case !k.preparing:
		k.playIdle(beat)
		newY = k.bounceUpdate(beat)
	default:
		newY = k.prepareUpdate(beat)
	}
	k.mod.ctx.Scene.SetPosOver(k.rootBodyPath, 0, newY)
}

func (k *animalKid) bounceUpdate(beat float64) float64 {
	start := k.jumpBeat
	if math.IsInf(start, -1) {
		start = 0
	}
	normalized := math.Mod(beat-start, 1)
	if normalized < 0 {
		normalized += 1
	}
	if normalized < 0.5 {
		p := normalized * 2
		k.mod.ctx.Scene.PlayFrozen(k.trampolinePath, "Bounce", easeOutQuad(0, 1, p))
		return easeOutQuad(0, k.jumpHeightIdle, p)
	}
	p := (normalized - 0.5) * 2
	k.mod.ctx.Scene.PlayFrozen(k.trampolinePath, "Bounce", easeInQuad(1, 0, p))
	return easeInQuad(k.jumpHeightIdle, 0, p)
}

func (k *animalKid) prepareUpdate(beat float64) float64 {
	p := (beat - k.prepareBeat) / 0.5
	if p >= 0 && p <= 1 {
		k.mod.ctx.Scene.PlayFrozen(k.trampolinePath, "Prepare", p)
		return easeOutQuad(k.prepareHeight, 0, p)
	}
	if p > 1 {
		k.mod.ctx.Scene.PlayFrozen(k.trampolinePath, "Prepare", 1)
	}
	return 0
}

func (k *animalKid) playIdle(beat float64) {
	state := "FoxIdle"
	if k.isBarely {
		state = "BarelyIdle"
	} else if !k.isFox {
		state = "HumanIdle"
	}
	k.playBody(state, beat, k.mod.ctx.SecPerBeat(beat))
}

func (k *animalKid) playBody(state string, beat, timeScale float64) {
	if k.bodyState == state {
		return
	}
	k.bodyState = state
	k.mod.ctx.Scene.PlayState(k.bodyPath, state, beat, timeScale)
}

func (m *Module) spawnBurst(k *animalKid, beat float64) {
	origin := k.particlePath
	if origin == "" {
		origin = k.bodyPath
	}
	m.bursts = append(m.bursts, particleBurst{
		startBeat: beat,
		origin:    origin,
		seed:      len(m.bursts)*7 + len(k.label),
	})
	if k.smokePath != "" && k.smokePath != origin {
		m.bursts = append(m.bursts, particleBurst{
			startBeat: beat,
			origin:    k.smokePath,
			seed:      len(m.bursts)*7 + len(k.label) + 3,
		})
	}
}

func (m *Module) queueBursts(sc *kart.SceneInst, beat float64) {
	for _, b := range m.bursts {
		p := clamp01((beat - b.startBeat) / 0.5)
		if p >= 1 {
			continue
		}
		base, ok := sc.NodeWorld(b.origin)
		if !ok {
			continue
		}
		for i := 0; i < 8; i++ {
			ang := float64(i)/8*math.Pi*2 + float64(b.seed)*0.37
			dist := (0.18 + 0.13*float64((i+b.seed)%3)) * (0.3 + p)
			rise := 0.35 * p
			scale := 0.35 + 0.45*p + 0.04*float64(i%2)
			alpha := 1 - p
			world := base.Mul(kart.Translate(math.Cos(ang)*dist, math.Sin(ang)*dist+rise)).Mul(kart.Scale(scale, scale))
			sc.Queue(kart.ExtraSprite{
				Sprite: "smoke" + string(rune('1'+i)),
				World:  world,
				Order:  8,
				Tint:   [4]float64{1, 1, 1, alpha},
			})
		}
	}
}

func randomJumpSound(prefix string) string {
	if rand.Intn(2) == 0 {
		return prefix + "1"
	}
	return prefix + "2"
}

func jumpTarget(beat float64) float64 { return beat + 1 }

func boolParam(e *riq.Entity, key string) bool { return boolDefault(e, key, false) }

func boolDefault(e *riq.Entity, key string, def bool) bool {
	v, ok := e.Data[key]
	if !ok {
		return def
	}
	switch x := v.(type) {
	case bool:
		return x
	case float64:
		return x != 0
	case int:
		return x != 0
	default:
		return def
	}
}

func easeOutQuad(start, end, p float64) float64 {
	p = clamp01(p)
	d := end - start
	return start - d*p*(p-2)
}

func easeInQuad(start, end, p float64) float64 {
	p = clamp01(p)
	d := end - start
	return start + d*p*p
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
