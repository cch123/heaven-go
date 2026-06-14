// Package fireworks ports Fireworks' rocket, sparkler, bomb, count-in,
// background toggle, hit flash, and explosion effects.
package fireworks

import (
	"image/color"
	"math"
	"math/rand"
	"sort"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"hsdemo/engine"
	"hsdemo/kart"
	"hsdemo/kmdata"
	"hsdemo/riq"
)

const (
	spawnLeft = iota
	spawnRight
	spawnMiddle
)

const (
	explosionBig = iota
	explosionDonut
	explosionSwirl
	explosionSmile
	explosionMixed
)

const (
	countOne = iota
	countTwo
	countThree
	countHey
)

type rocketCue struct {
	id        int
	beat      float64
	sparkler  bool
	spawn     int
	practice  bool
	explosion int
	applause  bool
	offset    float64
}

type bombCue struct {
	id       int
	beat     float64
	practice bool
	applause bool
}

type bgEvt struct {
	beat, length float64
	stars, faces bool
	top0, top1   [4]float64
	bot0, bot1   [4]float64
	city0, city1 [4]float64
	ease         int
}

type countEvt struct {
	beat float64
	kind int
}

type rocketState struct {
	cue      rocketCue
	hitBeat  float64
	exploded bool
	barely   bool
	success  bool
	burst    burst
}

type bombState struct {
	cue      bombCue
	start    float64
	hitBeat  float64
	exploded bool
	success  bool
	burst    burst
}

type burst struct {
	beat      float64
	x, y      float64
	particles []particle
}

type particle struct {
	x, y       float64
	vx, vy     float64
	life       float64
	scale      float64
	spriteA    string
	spriteB    string
	tint       [4]float64
	startDelay float64
}

type flashFade struct {
	beat, length float64
}

type Module struct {
	ctx  *engine.Ctx
	proj kart.Aff

	rockets []rocketCue
	bombs   []bombCue
	bgs     []bgEvt
	counts  []countEvt

	activeRockets map[int]*rocketState
	activeBombs   map[int]*bombState
	flash         flashFade
}

func New() engine.Module { return &Module{} }

func (m *Module) ID() string { return "fireworks" }

func (m *Module) Load(ctx *engine.Ctx) error {
	m.ctx = ctx
	if err := ctx.LoadAssets("fireworks"); err != nil {
		return err
	}
	m.proj = kart.Translate(engine.ScreenW/2, engine.ScreenH/2).Mul(kart.Scale(54, -54))
	m.activeRockets = map[int]*rocketState{}
	m.activeBombs = map[int]*bombState{}
	m.applyBG(defaultBG(0), 0)
	return nil
}

func (m *Module) OnEvent(e *riq.Entity) {
	switch e.Datamodel {
	case "fireworks/firework":
		m.addRocket(e, false)
	case "fireworks/sparkler":
		m.addRocket(e, true)
	case "fireworks/bomb":
		m.bombs = append(m.bombs, bombCue{
			id: len(m.bombs), beat: e.Beat,
			practice: boolParam(e, "toggle"), applause: boolParam(e, "applause"),
		})
	case "fireworks/countIn":
		m.counts = append(m.counts, countEvt{beat: e.Beat, kind: int(e.Float("count", countOne))})
	case "fireworks/changeBG":
		m.bgs = append(m.bgs, bgEvt{
			beat: e.Beat, length: e.Length,
			stars: boolDefault(e, "stars", true), faces: boolParam(e, "faces"),
			top0:  colorParam(e, "startTop", [4]float64{0, 8.0 / 255, 32.0 / 255, 1}),
			top1:  colorParam(e, "endTop", [4]float64{0, 8.0 / 255, 32.0 / 255, 1}),
			bot0:  colorParam(e, "startBot", [4]float64{0, 51.0 / 255, 119.0 / 255, 1}),
			bot1:  colorParam(e, "endBot", [4]float64{0, 51.0 / 255, 119.0 / 255, 1}),
			city0: colorParam(e, "startCity", [4]float64{0, 8.0 / 255, 32.0 / 255, 1}),
			city1: colorParam(e, "endCity", [4]float64{0, 8.0 / 255, 32.0 / 255, 1}),
			ease:  int(e.Float("ease", 0)),
		})
	case "fireworks/altBG":
		on := boolDefault(e, "toggle", true)
		m.bgs = append(m.bgs, bgEvt{
			beat: e.Beat, length: e.Length,
			stars: !on, faces: on,
			top0:  [4]float64{0, 8.0 / 255, 32.0 / 255, 1},
			top1:  [4]float64{0, 8.0 / 255, 32.0 / 255, 1},
			bot0:  [4]float64{0, 51.0 / 255, 119.0 / 255, 1},
			bot1:  [4]float64{0, 51.0 / 255, 119.0 / 255, 1},
			city0: [4]float64{0, 8.0 / 255, 32.0 / 255, 1},
			city1: [4]float64{0, 8.0 / 255, 32.0 / 255, 1},
		})
	}
}

func (m *Module) addRocket(e *riq.Entity, sparkler bool) {
	m.rockets = append(m.rockets, rocketCue{
		id: len(m.rockets), beat: e.Beat, sparkler: sparkler,
		spawn: int(e.Float("whereToSpawn", spawnMiddle)), practice: boolParam(e, "toggle"),
		explosion: int(e.Float("explosionType", explosionMixed)),
		applause:  boolParam(e, "applause"),
		offset:    e.Float("offSet", 0),
	})
}

func (m *Module) Ready() {
	sort.SliceStable(m.rockets, func(i, j int) bool { return m.rockets[i].beat < m.rockets[j].beat })
	sort.SliceStable(m.bombs, func(i, j int) bool { return m.bombs[i].beat < m.bombs[j].beat })
	sort.SliceStable(m.bgs, func(i, j int) bool { return m.bgs[i].beat < m.bgs[j].beat })
	sort.SliceStable(m.counts, func(i, j int) bool { return m.counts[i].beat < m.counts[j].beat })

	for _, cue := range m.rockets {
		cue := cue
		if cue.sparkler {
			m.ctx.SoundAtOff(cue.beat, "nuei", 1, 0.223)
		} else {
			m.ctx.SoundAt(cue.beat, "rocket_2", 1)
		}
		m.schedulePractice(cue)
		m.ctx.At(cue.beat, func() {
			if m.ctx.GameAt(cue.beat) == m.ID() {
				m.spawnRocket(cue)
			}
		})
		hitBeat := cue.beat + rocketTimer(cue.sparkler)
		m.ctx.ScheduleInputCond(hitBeat,
			func() bool { return m.ctx.GameAt(hitBeat) == m.ID() },
			func(state float64, _ engine.Judgment) { m.hitRocket(cue, hitBeat, state) },
			func() {},
		)
	}
	for _, cue := range m.bombs {
		cue := cue
		m.ctx.SoundAt(cue.beat, "tamaya_4", 1)
		if cue.practice {
			m.ctx.SoundAt(cue.beat+2, "practiceHai", 1)
		}
		m.ctx.At(cue.beat+1, func() {
			if m.ctx.GameAt(cue.beat+1) == m.ID() {
				m.spawnBomb(cue)
			}
		})
		hitBeat := cue.beat + 2
		m.ctx.ScheduleInputCond(hitBeat,
			func() bool { return m.ctx.GameAt(hitBeat) == m.ID() },
			func(state float64, _ engine.Judgment) { m.hitBomb(cue, hitBeat, state) },
			func() {},
		)
	}
	for _, ev := range m.counts {
		ev := ev
		m.ctx.SoundAt(ev.beat, countSound(ev.kind), 1)
	}
}

func (m *Module) OnSwitch(beat float64) {
	m.activeRockets = map[int]*rocketState{}
	m.activeBombs = map[int]*bombState{}
	m.flash = flashFade{}
	m.applyBG(m.bgAt(beat), beat)
	for _, cue := range m.rockets {
		if cue.beat <= beat && beat < cue.beat+rocketLifetime(cue.sparkler) {
			m.spawnRocket(cue)
		}
	}
	for _, cue := range m.bombs {
		if cue.beat+1 <= beat && beat < cue.beat+4 {
			m.spawnBomb(cue)
		}
	}
}

func (m *Module) Whiff(float64) {
	m.ctx.Sound("miss")
}

func (m *Module) Update(_ float64, beat float64) {
	m.applyBG(m.bgAt(beat), beat)
	m.cleanup(beat)
}

func (m *Module) Draw(screen *ebiten.Image, _ float64, beat float64) {
	screen.Fill(color.NRGBA{0, 8, 32, 255})
	m.ctx.SampleScene(beat)
	m.ctx.Scene.Draw(screen, m.proj)
	m.drawRockets(screen, beat)
	m.drawBombs(screen, beat)
	m.drawFlash(screen, beat)
}

func (m *Module) schedulePractice(cue rocketCue) {
	if !cue.practice {
		return
	}
	if cue.sparkler {
		m.ctx.SoundAt(cue.beat+1, "practiceHai", 1)
		return
	}
	m.ctx.SoundAt(cue.beat, "practice1", 1)
	m.ctx.SoundAt(cue.beat+1, "practice2", 1)
	m.ctx.SoundAt(cue.beat+2, "practice3", 1)
	m.ctx.SoundAt(cue.beat+3, "practiceHai", 1)
}

func (m *Module) spawnRocket(cue rocketCue) {
	m.activeRockets[cue.id] = &rocketState{cue: cue, hitBeat: cue.beat + rocketTimer(cue.sparkler)}
}

func (m *Module) spawnBomb(cue bombCue) {
	m.activeBombs[cue.id] = &bombState{cue: cue, start: cue.beat + 1, hitBeat: cue.beat + 2}
}

func (m *Module) hitRocket(cue rocketCue, hitBeat, state float64) {
	r := m.activeRockets[cue.id]
	if r == nil {
		r = &rocketState{cue: cue, hitBeat: hitBeat}
		m.activeRockets[cue.id] = r
	}
	x, y := rocketPos(cue, hitBeat)
	r.exploded = true
	r.barely = math.Abs(state) >= 1
	r.success = !r.barely
	if r.barely {
		m.ctx.Sound("miss")
		r.burst = barelyBurst(hitBeat, x, y)
		return
	}
	m.ctx.Sound("explode_5")
	r.burst = newBurst(hitBeat, x, y, cue.explosion)
	if cue.applause {
		m.ctx.SoundAt(hitBeat+1, "common_applause", 1)
	}
}

func (m *Module) hitBomb(cue bombCue, hitBeat, state float64) {
	b := m.activeBombs[cue.id]
	if b == nil {
		b = &bombState{cue: cue, start: cue.beat + 1, hitBeat: hitBeat}
		m.activeBombs[cue.id] = b
	}
	x, y := bombPos(cue.beat+1, hitBeat)
	b.exploded = true
	b.success = math.Abs(state) < 1
	if !b.success {
		m.ctx.Sound("miss")
		b.burst = barelyBurst(hitBeat, x, y)
		return
	}
	m.ctx.Sound("taikoExplode")
	m.flash = flashFade{beat: hitBeat, length: 0.5}
	b.burst = newBurst(hitBeat, x, y, explosionBig)
	if cue.applause {
		m.ctx.SoundAt(hitBeat+1, "common_applause", 1)
	}
}

func (m *Module) cleanup(beat float64) {
	for id, r := range m.activeRockets {
		if beat > r.cue.beat+rocketLifetime(r.cue.sparkler) || (r.exploded && beat > r.hitBeat+2.4) {
			delete(m.activeRockets, id)
		}
	}
	for id, b := range m.activeBombs {
		if beat > b.cue.beat+5 || (b.exploded && beat > b.hitBeat+2.4) {
			delete(m.activeBombs, id)
		}
	}
}

func (m *Module) drawRockets(screen *ebiten.Image, beat float64) {
	for _, r := range m.activeRockets {
		if beat < r.cue.beat {
			continue
		}
		if !r.exploded {
			x, y := rocketPos(r.cue, beat)
			sprite := rocketSprite(m.ctx.Assets.Anims, r.cue.sparkler, beat-r.cue.beat)
			m.ctx.Assets.DrawSpriteOpts(screen, sprite, kart.Translate(x, y), m.proj, kart.SpriteOpts{})
			continue
		}
		m.drawBurst(screen, r.burst, beat)
	}
}

func (m *Module) drawBombs(screen *ebiten.Image, beat float64) {
	for _, b := range m.activeBombs {
		if beat < b.start {
			continue
		}
		if !b.exploded {
			x, y := bombPos(b.start, beat)
			m.ctx.Assets.DrawSpriteOpts(screen, "bomb", kart.Translate(x, y), m.proj, kart.SpriteOpts{})
			continue
		}
		if beat < b.hitBeat+0.45 {
			sprite := explosionSprite(m.ctx.Assets.Anims, beat-b.hitBeat)
			m.ctx.Assets.DrawSpriteOpts(screen, sprite, kart.Translate(b.burst.x, b.burst.y), m.proj, kart.SpriteOpts{})
		}
		m.drawBurst(screen, b.burst, beat)
	}
}

func (m *Module) drawBurst(screen *ebiten.Image, b burst, beat float64) {
	for _, p := range b.particles {
		t := beat - b.beat - p.startDelay
		if t < 0 || t > p.life {
			continue
		}
		u := t / p.life
		x := b.x + p.x + p.vx*t
		y := b.y + p.y + p.vy*t - 0.45*t*t
		sprite := p.spriteA
		if p.spriteB != "" && int(math.Floor(t*12))%2 == 1 {
			sprite = p.spriteB
		}
		tint := p.tint
		tint[3] *= 1 - u
		world := kart.Translate(x, y).Mul(kart.Scale(p.scale, p.scale))
		m.ctx.Assets.DrawSpriteOpts(screen, sprite, world, m.proj, kart.SpriteOpts{Tint: tint})
	}
}

func (m *Module) drawFlash(screen *ebiten.Image, beat float64) {
	if m.flash.length <= 0 {
		return
	}
	u := (beat - m.flash.beat) / m.flash.length
	if u < 0 || u > 1 {
		return
	}
	alpha := float32(1 - u)
	vector.DrawFilledRect(screen, 0, 0, engine.ScreenW, engine.ScreenH, color.NRGBA{255, 255, 255, uint8(alpha * 210)}, true)
}

func rocketTimer(sparkler bool) float64 {
	if sparkler {
		return 1
	}
	return 3
}

func rocketLifetime(sparkler bool) float64 {
	if sparkler {
		return 3.2
	}
	return 9.2
}

func rocketPos(cue rocketCue, beat float64) (float64, float64) {
	x, spawnY := spawnPoint(cue.spawn)
	startY := spawnY - cue.offset
	timer := rocketTimer(cue.sparkler)
	norm := (beat - cue.beat) / timer
	factor := 0.4
	if cue.sparkler {
		factor = 0.5
	}
	y := startY + (7-cue.offset-startY)*norm*factor
	return x, y
}

func spawnPoint(spawn int) (float64, float64) {
	switch spawn {
	case spawnLeft:
		return -5.04, -6.48
	case spawnRight:
		return 5.04, -6.48
	default:
		return 0, -6.48
	}
}

func bombPos(startBeat, beat float64) (float64, float64) {
	start := -2.75424106
	startY := -5.1646384
	end := -0.01966106
	endY := 0.601241
	u := clamp01(beat - startBeat)
	return start + (end-start)*u, startY + (endY-startY)*u
}

func rocketSprite(anims map[string]*kmdata.Anim, sparkler bool, elapsedBeat float64) string {
	name := "Animations/Rocket"
	if sparkler {
		name = "Animations/Sparkler"
	}
	return animSprite(anims[name], elapsedBeat*0.18, "rocket_0")
}

func explosionSprite(anims map[string]*kmdata.Anim, elapsedBeat float64) string {
	return animSprite(anims["Animations/ExplodeBomb"], elapsedBeat*0.12, "explosion_0")
}

func animSprite(anim *kmdata.Anim, t float64, fallback string) string {
	if anim == nil {
		return fallback
	}
	if anim.Loop && anim.Duration > 0 {
		t = math.Mod(t, anim.Duration)
	}
	keys := anim.Sprites[""]
	sprite := fallback
	for _, k := range keys {
		if k.T <= t {
			sprite = k.Name
		}
	}
	return sprite
}

func newBurst(beat, x, y float64, typ int) burst {
	r := rand.New(rand.NewSource(int64(beat*4096) + int64(typ)*131))
	b := burst{beat: beat, x: x, y: y}
	add := func(angle, dist, delay float64, spriteA, spriteB string, tint [4]float64) {
		speed := dist * (0.72 + r.Float64()*0.22)
		b.particles = append(b.particles, particle{
			vx: math.Cos(angle) * speed, vy: math.Sin(angle) * speed,
			life: 1.35 + r.Float64()*0.35, scale: 0.46 + r.Float64()*0.18,
			spriteA: spriteA, spriteB: spriteB, tint: tint, startDelay: delay,
		})
	}
	colors := [][3]string{
		{"sparkGreen_0", "sparkGreen_1", "green"},
		{"sparkRed_0", "sparkRed_1", "red"},
		{"sparkBlue_0", "sparkBlue_1", "blue"},
	}
	tints := map[string][4]float64{
		"green": {0.6, 1, 0.45, 1},
		"red":   {1, 0.45, 0.45, 1},
		"blue":  {0.55, 0.75, 1, 1},
	}
	switch typ {
	case explosionDonut:
		for i := 0; i < 32; i++ {
			c := colors[i%len(colors)]
			add(float64(i)/32*math.Pi*2, 2.8, 0, c[0], c[1], tints[c[2]])
		}
	case explosionSwirl:
		for i := 0; i < 38; i++ {
			c := colors[i%len(colors)]
			ang := float64(i)*0.47 + float64(i)/38*math.Pi*2
			add(ang, 1.1+float64(i)*0.055, float64(i)*0.008, c[0], c[1], tints[c[2]])
		}
	case explosionSmile:
		for i := 0; i < 24; i++ {
			ang := math.Pi*0.18 + float64(i)/23*math.Pi*0.64
			add(ang, 2.2, 0, "sparkRed_0", "sparkRed_1", tints["red"])
		}
		for _, p := range [][2]float64{{-0.8, 0.55}, {0.8, 0.55}} {
			b.particles = append(b.particles, particle{x: p[0], y: p[1], life: 1.2, scale: 0.62, spriteA: "sparkBlue_0", spriteB: "sparkBlue_1", tint: tints["blue"]})
		}
	case explosionMixed:
		for i := 0; i < 36; i++ {
			c := colors[i%len(colors)]
			add(float64(i)/36*math.Pi*2, 1.4+r.Float64()*1.4, r.Float64()*0.05, c[0], c[1], tints[c[2]])
		}
	default:
		for i := 0; i < 42; i++ {
			c := colors[i%len(colors)]
			add(float64(i)/42*math.Pi*2, 2.2+r.Float64()*0.9, 0, c[0], c[1], tints[c[2]])
		}
	}
	return b
}

func barelyBurst(beat, x, y float64) burst {
	b := burst{beat: beat, x: x, y: y}
	for i := 0; i < 10; i++ {
		ang := -math.Pi*0.15 + float64(i)/9*math.Pi*1.3
		b.particles = append(b.particles, particle{
			vx: math.Cos(ang) * 0.9, vy: math.Sin(ang) * 0.9,
			life: 0.7, scale: 0.35, spriteA: "explosion_0",
			tint: [4]float64{0.8, 0.8, 0.8, 0.65},
		})
	}
	return b
}

func (m *Module) bgAt(beat float64) bgEvt {
	cur := defaultBG(0)
	for _, ev := range m.bgs {
		if ev.beat > beat {
			break
		}
		cur = ev
	}
	return cur
}

func defaultBG(beat float64) bgEvt {
	return bgEvt{
		beat: beat, stars: true,
		top0:  [4]float64{0, 8.0 / 255, 32.0 / 255, 1},
		top1:  [4]float64{0, 8.0 / 255, 32.0 / 255, 1},
		bot0:  [4]float64{0, 51.0 / 255, 119.0 / 255, 1},
		bot1:  [4]float64{0, 51.0 / 255, 119.0 / 255, 1},
		city0: [4]float64{0, 8.0 / 255, 32.0 / 255, 1},
		city1: [4]float64{0, 8.0 / 255, 32.0 / 255, 1},
	}
}

func (m *Module) applyBG(ev bgEvt, beat float64) {
	u := 1.0
	if ev.length > 0 {
		u = clamp01((beat - ev.beat) / ev.length)
	}
	top := easeColor(ev.top0, ev.top1, ev.ease, u)
	bot := easeColor(ev.bot0, ev.bot1, ev.ease, u)
	city := easeColor(ev.city0, ev.city1, ev.ease, u)
	m.ctx.Scene.SetColorOver("agasgagag/Gradient", top)
	m.ctx.Scene.SetColorOver("agasgagag/Sky", bot)
	m.ctx.Scene.SetColorOver("agasgagag/City", city)
	m.ctx.Scene.SetColorOver("BG/Cityold", city)
	m.ctx.Scene.SetActive("agasgagag/Stars", ev.stars)
	m.ctx.Scene.SetActive("agasgagag/Faces", ev.faces)
}

func easeColor(a, b [4]float64, ease int, u float64) [4]float64 {
	return [4]float64{
		engine.Ease(ease, a[0], b[0], u),
		engine.Ease(ease, a[1], b[1], u),
		engine.Ease(ease, a[2], b[2], u),
		engine.Ease(ease, a[3], b[3], u),
	}
}

func countSound(kind int) string {
	switch kind {
	case countTwo:
		return "count2"
	case countThree:
		return "count3"
	case countHey:
		return "countHey"
	default:
		return "count1"
	}
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

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
