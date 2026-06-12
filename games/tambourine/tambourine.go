// Package tambourine 是 Tambourine（tambourine）的玩法模块，
// 逻辑对应 Assets/Scripts/Games/Tambourine/Tambourine.cs：
//
//	beat intervals(L, auto)：区间起点清状态（汗/蛙/哭脸），区间内的
//	    shake/hit 由猴子示范（MonkeyShake/MonkeySmack + 随机猴声）；
//	    auto=true 时区间结束自动 pass turn（length 1）
//	pass turn：tp 随机音 + MonkeyPassTurn + 笑脸 0.3 拍；玩家在
//	    beat+length+相对偏移 复述每个输入（hit=南键通道，shake=主键），
//	    判定时刻猴子 bop
//	success：本轮无失误 → 花粒子 + sweep + note1-4 + 笑脸 1 拍
//	bop / fade background：双角色 bop 区间 / 背景色 ColorEase
//
// 失误链：near/miss/空按 → 汗 + 蛙登场 +（区间外）哭脸；区间起点复位。
// 花粒子为等价手写实现。
package tambourine

import (
	"math"
	"math/rand"
	"sort"

	"github.com/hajimehoshi/ebiten/v2"

	"hsdemo/engine"
	"hsdemo/kart"
	"hsdemo/riq"
)

var defaultBG = [4]float64{0.22, 0.55, 0.82, 1}

type colorEase struct {
	beat, length float64
	c0, c1       [4]float64
	ease         int
}

type interval struct {
	beat, length float64
	auto         bool
}

type inputEvt struct {
	beat float64
	hit  bool
}

type flower struct {
	x, y, vx, vy, born float64
}

type Module struct {
	ctx  *engine.Ctx
	proj kart.Aff

	intervals []interval
	inputs    []inputEvt
	passes    []struct{ beat, length float64 }
	bg        colorEase
	bgEvts    []colorEase

	monkeyBop bool
	handsBop  bool
	missed    bool // hasMissedThisInterval
	frog      bool
	lastPulse float64
	lastT     float64
	flowers   []flower
}

func New() engine.Module {
	return &Module{bg: colorEase{c0: defaultBG, c1: defaultBG}}
}

func (m *Module) ID() string { return "tambourine" }

func (m *Module) Load(ctx *engine.Ctx) error {
	m.ctx = ctx
	if err := ctx.LoadAssets("tambourine"); err != nil {
		return err
	}
	m.proj = kart.Translate(engine.ScreenW/2, engine.ScreenH/2).Mul(kart.Scale(54, -54))
	ctx.Scene.SetActive(ctx.Role("happyFace"), false)
	ctx.Scene.SetActive(ctx.Role("sadFace"), false)
	return nil
}

func rnd15() string { return []string{"1", "2", "3", "4", "5"}[rand.Intn(5)] }

// ---------- 事件 ----------

func (m *Module) OnEvent(e *riq.Entity) {
	b := e.Beat
	ctx := m.ctx
	switch e.Datamodel {
	case "tambourine/bop":
		who := int(e.Float("whoBops", 2))
		auto := int(e.Float("whoBopsAuto", 3))
		ctx.At(b, func() {
			m.monkeyBop = auto == 0 || auto == 2
			m.handsBop = auto == 1 || auto == 2
		})
		for i := 0.0; i < e.Length; i++ {
			bb := b + i
			ctx.At(bb, func() { m.bopOnce(who, bb) })
		}
	case "tambourine/beat intervals":
		m.intervals = append(m.intervals, interval{b, e.Length, boolParam(e, "auto")})
	case "tambourine/shake":
		m.inputs = append(m.inputs, inputEvt{b, false})
	case "tambourine/hit":
		m.inputs = append(m.inputs, inputEvt{b, true})
	case "tambourine/pass turn":
		m.passes = append(m.passes, struct{ beat, length float64 }{b, e.Length})
	case "tambourine/success":
		ctx.At(b, func() { m.successFace(b) })
	case "tambourine/fade background":
		ce := colorEase{
			beat: b, length: e.Length,
			c0: colorParam(e, "colorStart", [4]float64{1, 1, 1, 1}),
			c1: colorParam(e, "colorEnd", defaultBG),
			ease: int(e.Float("ease", 0)),
		}
		m.bgEvts = append(m.bgEvts, ce)
		ctx.At(b, func() { m.bg = ce })
	}
}

// Ready：区间静态展开（猴子示范 + auto pass turn + 玩家复述）。
func (m *Module) Ready() {
	ctx := m.ctx
	sort.Slice(m.inputs, func(i, j int) bool { return m.inputs[i].beat < m.inputs[j].beat })
	for _, iv := range m.intervals {
		iv := iv
		ctx.At(iv.beat, func() {
			m.missed = false
			m.desummonFrog()
			ctx.Scene.SetActive(ctx.Role("sadFace"), false)
		})
		// 猴子示范
		for _, in := range m.inputsIn(iv.beat, iv.beat+iv.length) {
			in := in
			name := "monkey/shake/ms"
			anim := "MonkeyShake"
			if in.hit {
				name, anim = "monkey/hit/mh", "MonkeySmack"
			}
			ctx.SoundAt(in.beat, name+rnd15(), 1)
			ctx.At(in.beat, func() {
				ctx.Scene.PlayState(ctx.Role("monkeyAnimator"), anim, in.beat, 0.5)
			})
		}
		if iv.auto {
			m.passTurn(iv.beat+iv.length, iv, 1)
		}
	}
	// 手动 pass turn
	for _, p := range m.passes {
		var last *interval
		for i := range m.intervals {
			if m.intervals[i].beat <= p.beat {
				last = &m.intervals[i]
			}
		}
		if last != nil {
			m.passTurn(p.beat, *last, p.length)
		}
	}
}

func (m *Module) inputsIn(from, to float64) []inputEvt {
	var out []inputEvt
	for _, in := range m.inputs {
		if in.beat >= from && in.beat < to {
			out = append(out, in)
		}
	}
	return out
}

func (m *Module) passTurn(beat float64, iv interval, length float64) {
	ctx := m.ctx
	ctx.SoundAt(beat, "monkey/turnPass/tp"+rnd15(), 1)
	ctx.At(beat, func() {
		ctx.Scene.PlayState(ctx.Role("monkeyAnimator"), "MonkeyPassTurn", beat, 0.5)
		ctx.Scene.SetActive(ctx.Role("happyFace"), true)
	})
	ctx.At(beat+0.3, func() { ctx.Scene.SetActive(ctx.Role("happyFace"), false) })
	for _, in := range m.inputsIn(iv.beat, iv.beat+iv.length) {
		in := in
		rel := in.beat - iv.beat
		judge := beat + length + rel
		action := 0
		if in.hit {
			action = 1 // 南键通道
		}
		ctx.ScheduleInputAction(judge, action,
			func(st float64, _ engine.Judgment) { m.just(st, in.hit) },
			func() { m.missJudged() })
		ctx.At(judge, func() { m.bopOnce(0, judge) }) // 判定拍猴子 bop
	}
}

// ---------- 判定 ----------

func (m *Module) just(state float64, hit bool) {
	ctx := m.ctx
	anim, snd := "Shake", "player/shake/ps"
	if hit {
		anim, snd = "Smack", "player/hit/ph"
	}
	if state >= 1 || state <= -1 {
		ctx.Scene.PlayState(ctx.Role("handsAnimator"), anim, ctx.Beat(), 0.5)
		ctx.Sound(snd + rnd15())
		ctx.Sound("miss")
		ctx.Scene.PlayState(ctx.Role("sweatAnimator"), "Sweating", ctx.Beat(), 0.5)
		m.flagMiss()
		return
	}
	ctx.Scene.SetActive(ctx.Role("sadFace"), false)
	ctx.Scene.PlayState(ctx.Role("handsAnimator"), anim, ctx.Beat(), 0.5)
	ctx.Sound(snd + rnd15())
}

func (m *Module) missJudged() {
	m.summonFrog()
	m.ctx.Scene.PlayState(m.ctx.Role("sweatAnimator"), "Sweating", m.ctx.Beat(), 0.5)
	m.flagMiss()
}

// flagMiss：missed 置位 +（区间外）哭脸。
func (m *Module) flagMiss() {
	m.missed = true
	if !m.intervalGoing() {
		m.ctx.Scene.SetActive(m.ctx.Role("sadFace"), true)
	}
}

func (m *Module) intervalGoing() bool {
	b := m.ctx.Beat()
	for _, iv := range m.intervals {
		if b >= iv.beat && b < iv.beat+iv.length {
			return true
		}
	}
	return false
}

// WhiffAction：空按（Nrm=shake / Alt=hit）。
func (m *Module) WhiffAction(beat float64, action int) {
	ctx := m.ctx
	anim, snd := "Shake", "player/shake/ps"
	if action == 1 {
		anim, snd = "Smack", "player/hit/ph"
	}
	ctx.Scene.PlayState(ctx.Role("handsAnimator"), anim, beat, ctx.SecPerBeat(beat))
	ctx.Sound(snd + rnd15())
	ctx.Scene.PlayState(ctx.Role("sweatAnimator"), "Sweating", beat, ctx.SecPerBeat(beat))
	m.summonFrog()
	ctx.ScoreMiss()
	m.flagMiss()
}

func (m *Module) Whiff(beat float64) { m.WhiffAction(beat, 0) }

// ---------- 笑脸/蛙/花 ----------

func (m *Module) successFace(beat float64) {
	ctx := m.ctx
	m.desummonFrog()
	if m.missed {
		return
	}
	m.spawnFlowers()
	ctx.Sound("player/turnPass/sweep")
	for i, n := range []string{"note1", "note2", "note3", "note4"} {
		ctx.SoundAt(beat+0.1*float64(i), "player/turnPass/"+n, 1)
	}
	ctx.Scene.SetActive(ctx.Role("happyFace"), true)
	ctx.At(beat+1, func() { ctx.Scene.SetActive(ctx.Role("happyFace"), false) })
}

func (m *Module) summonFrog() {
	if m.frog {
		return
	}
	m.ctx.Sound("frog")
	m.ctx.Scene.PlayState(m.ctx.Role("frogAnimator"), "FrogEnter", m.ctx.Beat(), m.ctx.SecPerBeat(m.ctx.Beat()))
	m.frog = true
}

func (m *Module) desummonFrog() {
	if !m.frog {
		return
	}
	m.ctx.Scene.PlayState(m.ctx.Role("frogAnimator"), "FrogExit", m.ctx.Beat(), m.ctx.SecPerBeat(m.ctx.Beat()))
	m.frog = false
}

func (m *Module) spawnFlowers() {
	for i := 0; i < 18; i++ {
		ang := rand.Float64()*math.Pi - math.Pi/2
		sp := 3 + rand.Float64()*4
		m.flowers = append(m.flowers, flower{
			x: 0, y: -1, vx: math.Sin(ang) * sp, vy: math.Cos(ang)*sp + 2, born: m.lastT,
		})
	}
}

func (m *Module) bopOnce(who int, beat float64) {
	ctx := m.ctx
	if who == 0 || who == 2 {
		ctx.Scene.PlayState(ctx.Role("monkeyAnimator"), "MonkeyBop", beat, 0.5)
	}
	if who == 1 || who == 2 {
		ctx.Scene.PlayState(ctx.Role("handsAnimator"), "Bop", beat, 0.5)
	}
}

// ---------- 引擎接口 ----------

func (m *Module) OnSwitch(beat float64) {
	// PersistColor
	m.bg = colorEase{c0: defaultBG, c1: defaultBG}
	for _, ce := range m.bgEvts {
		if ce.beat < beat {
			m.bg = ce
		}
	}
	m.lastPulse = math.Floor(beat)
}

func (m *Module) Update(t, beat float64) {
	m.lastT = t
	if p := math.Floor(beat); p > m.lastPulse {
		m.lastPulse = p
		if m.monkeyBop {
			m.ctx.Scene.PlayState(m.ctx.Role("monkeyAnimator"), "MonkeyBop", p, m.ctx.SecPerBeat(p))
		}
		if m.handsBop {
			m.ctx.Scene.PlayState(m.ctx.Role("handsAnimator"), "Bop", p, m.ctx.SecPerBeat(p))
		}
	}
}

func (m *Module) Draw(screen *ebiten.Image, t, beat float64) {
	// fade background ColorEase（bg 节点为白色底图，直接以底色填充）
	norm := 1.0
	if m.bg.length > 0 {
		norm = clamp01((beat - m.bg.beat) / m.bg.length)
	}
	var c [4]float64
	for i := 0; i < 4; i++ {
		c[i] = engine.Ease(m.bg.ease, m.bg.c0[i], m.bg.c1[i], norm)
	}
	m.ctx.Scene.SetColorOver(m.ctx.Role("bg"), c)

	screen.Fill(toRGBA(c))
	sc := m.ctx.Scene
	sc.Sample(beat)
	sc.Draw(screen, m.proj)

	// 花粒子（flowerParticles 等价手写）
	alive := m.flowers[:0]
	for _, f := range m.flowers {
		age := t - f.born
		if age > 1.4 {
			continue
		}
		alive = append(alive, f)
		x := f.x + f.vx*age
		y := f.y + f.vy*age - 4.5*age*age
		px := float64(engine.ScreenW)/2 + x*54
		py := float64(engine.ScreenH)/2 - y*54
		ebitenFillRect(screen, px-3, py-3, 6, 6, colorRGBA{250, 190, 210, 230})
	}
	m.flowers = alive
}

// ---------- 工具 ----------

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

type colorRGBA struct{ R, G, B, A uint8 }

func toRGBA(c [4]float64) colorRGBA {
	cl := func(v float64) uint8 {
		if v < 0 {
			v = 0
		}
		if v > 1 {
			v = 1
		}
		return uint8(v*255 + 0.5)
	}
	return colorRGBA{cl(c[0] * c[3]), cl(c[1] * c[3]), cl(c[2] * c[3]), cl(c[3])}
}

func (c colorRGBA) RGBA() (r, g, b, a uint32) {
	return uint32(c.R) * 0x101, uint32(c.G) * 0x101, uint32(c.B) * 0x101, uint32(c.A) * 0x101
}

var whitePix *ebiten.Image

func ebitenFillRect(dst *ebiten.Image, x, y, w, h float64, c colorRGBA) {
	if whitePix == nil {
		whitePix = ebiten.NewImage(1, 1)
		whitePix.Fill(colorRGBA{255, 255, 255, 255})
	}
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(w, h)
	op.GeoM.Translate(x, y)
	op.ColorScale.Scale(float32(c.R)/255, float32(c.G)/255, float32(c.B)/255, float32(c.A)/255)
	dst.DrawImage(whitePix, op)
}

func boolParam(e *riq.Entity, key string) bool { return e.Float(key, 0) != 0 }

func colorParam(e *riq.Entity, key string, def [4]float64) [4]float64 {
	if mm, ok := e.Data[key].(map[string]any); ok {
		get := func(k string) float64 {
			if f, ok := mm[k].(float64); ok {
				return f
			}
			return 0
		}
		return [4]float64{get("r"), get("g"), get("b"), get("a")}
	}
	return def
}
