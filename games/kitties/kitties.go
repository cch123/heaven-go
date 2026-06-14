// Package kitties 是 Kitties!（kitties）的玩法模块，
// 逻辑对应 Assets/Scripts/Games/Kitties/Kitties.cs + CtrTeppanPlayer.cs：
//
//	clap：三只猫依次登场（B/B+0.75/B+1.5，PopIn/MicePopIn/FacePopIn），
//	      非玩家猫 B+2.5/B+3 拍手（Clap1/Clap2 等），玩家两拍输入；
//	      生成方位 type 0 直列 / 1 斜下 / 2 斜上 / 3 大脸（带 z 透视与旋转），
//	      inverse 镜像；keep=false 时 B+3.5 收场
//	roll：滚动序列（roll1-4 + spin1-10），B+2 按住（南键通道）、
//	      B+2.75 松开；中途空按/漏放 → RollFail
//	fish：鱼竿（官方关未用，按 C# 时序实现）
//	bgcolor：背景色 ColorEase
//
// 已知简化：spinnya 循环音的随机变调（±5%）未实现（循环重采样不支持）。
package kitties

import (
	"math"
	"math/rand"

	"github.com/hajimehoshi/ebiten/v2"

	"hsdemo/engine"
	"hsdemo/kart"
	"hsdemo/riq"
)

// spawnPos：Spawn() 的硬编码世界坐标表 [type][inverse][cat] = (x,y,z)。
var spawnPos = [3][2][3][3]float64{
	{ // type 0 Straight
		{{-5.11, -1.25, 0}, {0.32, -1.25, 0}, {5.75, -1.25, 0}},
		{{5.75, -1.25, 0}, {0.32, -1.25, 0}, {-5.11, -1.5, 0}},
	},
	{ // type 1 DownDiagonal
		{{-6.61, 1.75, 6}, {0.32, -0.25, 2}, {4.25, -1.75, -2}},
		{{6.61, 1.75, 6}, {0.32, -0.25, 2}, {-4.25, -1.75, -2}},
	},
	{ // type 2 UpDiagonal
		{{4.25, -1.75, -2}, {0.32, -0.25, 2}, {-6.61, 1.75, 6}},
		{{-4.25, -1.75, -2}, {0.32, -0.25, 2}, {6.61, 1.75, 6}},
	},
}

// type 3 CloseUp：固定坐标与 z 旋转。
var facePos = [3][3]float64{{-8.21, 3.7, 0}, {7.51, 4.2, 0}, {0.32, -4.25, 0}}
var faceRot = [3]float64{-135, 135, 0}

type colorEase struct {
	beat, length float64
	c0, c1       [4]float64
	ease         int
}

type clapEvt struct {
	beat                float64
	typ                 int
	mice, inverse, keep bool
}

type Module struct {
	ctx  *engine.Ctx
	proj kart.Aff

	bg        colorEase
	bgEvts    []colorEase
	spawnType int
	hasSpun   bool
	stopNya   func()
	endBeat   float64
}

func New() engine.Module { return &Module{bg: colorEase{c0: white, c1: white}} }

var white = [4]float64{1, 1, 1, 1}

func (m *Module) ID() string { return "kitties" }

func (m *Module) Load(ctx *engine.Ctx) error {
	m.ctx = ctx
	if err := ctx.LoadAssets("kitties"); err != nil {
		return err
	}
	m.proj = kart.Translate(engine.ScreenW/2, engine.ScreenH/2).Mul(kart.Scale(54, -54))
	m.stopNya = func() {}
	return nil
}

func (m *Module) cats() []string   { return m.ctx.Assets.Extra.RefArrays["Cats"] }
func (m *Module) catIdx(n int) int { return m.ctx.Assets.Extra.RefArrayIdx["Cats"][n] }
func (m *Module) player() string   { return m.ctx.Role("player") }

// ---------- 事件 ----------

func (m *Module) OnEvent(e *riq.Entity) {
	b := e.Beat
	ctx := m.ctx
	switch e.Datamodel {
	case "kitties/clap":
		ce := clapEvt{
			beat: b, typ: int(e.Float("type", 0)),
			mice: boolParam(e, "toggle"), inverse: boolParam(e, "toggle1"),
			keep: boolParam(e, "toggle2"),
		}
		m.scheduleClap(ce)
	case "kitties/roll":
		m.scheduleRoll(b, boolParam(e, "toggle"))
	case "kitties/fish":
		m.scheduleFish(b)
	case "kitties/instantSpawn":
		typ := int(e.Float("type", 0))
		mice, inv := boolParam(e, "toggle"), boolParam(e, "toggle1")
		ctx.At(b, func() {
			for c := 0; c < 3; c++ {
				m.spawn(typ, c, mice, inv, true, true)
			}
		})
	case "kitties/bgcolor":
		ce := colorEase{
			beat: b, length: e.Length,
			c0: colorParam(e, "colorStart", white), c1: colorParam(e, "colorEnd", white),
			ease: int(e.Float("ease", 0)),
		}
		m.bgEvts = append(m.bgEvts, ce)
		ctx.At(b, func() { m.bg = ce })
	}
}

func (m *Module) Ready() {}

// ---------- clap ----------

func (m *Module) scheduleClap(ce clapEvt) {
	ctx := m.ctx
	b := ce.beat
	// nya 三声（每声 2 个随机变体）+ 其余猫的拍手声
	ctx.SoundAt(b, nyaVar(1), 1)
	ctx.SoundAt(b+0.75, nyaVar(2), 1)
	ctx.SoundAt(b+1.5, nyaVar(3), 1)
	ctx.SoundAt(b+2.5, "clapMiss1", 1)
	ctx.SoundAt(b+3, "clapMiss2", 1)

	clap1, clap2 := "Clap1", "Clap2"
	if ce.typ == 3 {
		clap1, clap2 = "FaceClap", "FaceClap"
	} else if ce.mice {
		clap1, clap2 = "MiceClap1", "MiceClap2"
	}
	for c := 0; c < 2; c++ {
		c := c
		spawnBeat := b + []float64{0, 0.75}[c]
		ctx.At(spawnBeat, func() { m.spawn(ce.typ, c, ce.mice, ce.inverse, c == 0, false) })
		ctx.At(b+2.5, func() { m.playCat(c, clap1) })
		ctx.At(b+3, func() { m.playCat(c, clap2) })
	}
	ctx.At(b+1.5, func() {
		m.spawn(ce.typ, 2, ce.mice, ce.inverse, false, false)
		m.spawnType = ce.typ
	})

	ctx.ScheduleInput(b+2.5, func(st float64, _ engine.Judgment) { m.clapHit(st, 1) }, func() {})
	ctx.ScheduleInput(b+3, func(st float64, _ engine.Judgment) { m.clapHit(st, 2) }, func() {})

	if !ce.keep {
		ctx.At(b+3.5, func() { m.removeCats() })
	}
}

func nyaVar(n int) string {
	return []string{"nya1_", "nya2_", "nya3_"}[n-1] + []string{"1", "2"}[rand.Intn(2)]
}

func (m *Module) clapHit(state float64, which int) {
	ctx := m.ctx
	if m.spawnType != 3 {
		if state >= 1 || state <= -1 {
			ctx.Sound("tink")
			m.playCat(2, "ClapMiss")
			return
		}
		ctx.Sound("clapPlayer" + []string{"1", "2"}[which-1])
		m.playCat(2, "Clap"+[]string{"1", "2"}[which-1])
		return
	}
	// 大脸：近失 tink+FaceClapFail 后仍接 FaceClap（C# 无 else，保持原样）
	if state >= 1 || state <= -1 {
		ctx.Sound("tink")
		m.playCat(2, "FaceClapFail")
	}
	ctx.Sound("clapPlayer" + []string{"1", "2"}[which-1])
	m.playCat(2, "FaceClap")
}

// ---------- roll ----------

func (m *Module) scheduleRoll(b float64, keep bool) {
	ctx := m.ctx
	// canClap 门：上一个 clap/instantSpawn 留场才有效（C# 运行时 bool，
	// 此处依赖 clap keep=true 的谱面约定——官方关均满足；若未留场，
	// 输入照常生成（autoplay 语义不变），仅视觉无猫）。
	for i := 0; i < 4; i++ {
		ctx.SoundAt(b+0.5*float64(i), []string{"roll1", "roll2", "roll3", "roll4"}[i], 1)
	}
	for i := 0; i < 10; i++ {
		ctx.SoundAt(b+2+0.075*float64(i), []string{
			"spin1", "spin2", "spin3", "spin4", "spin5",
			"spin6", "spin7", "spin8", "spin9", "spin10"}[i], 1)
	}
	for c := 0; c < 3; c++ {
		c := c
		for i := 0; i < 4; i++ {
			bb := b + 0.5*float64(i)
			ctx.At(bb, func() { m.playCat(c, "RollStart") })
		}
		if c < 2 {
			ctx.At(b+2, func() { m.playCat(c, "Rolling") })
			ctx.At(b+2.75, func() { m.playCat(c, "RollEnd") })
		}
	}
	// 玩家：B+2 按住（南键通道=动作 1），B+2.75 松开
	ctx.ScheduleInputAction(b+2, 1,
		func(st float64, _ engine.Judgment) { m.spinStart(b) },
		func() { m.spinMissOne() })
	if !keep {
		ctx.At(b+3.5, func() { m.removeCats() })
	}
}

func (m *Module) spinStart(b float64) {
	ctx := m.ctx
	m.hasSpun = true
	target := b + 2
	for i := 0; i < 5; i++ {
		ctx.SoundAt(target+0.15*float64(i), []string{
			"spinplayer1", "spinplayer2", "spinplayer3", "spinplayer4", "spinplayer5"}[i], 1)
	}
	m.stopNya()
	m.stopNya = ctx.SoundLoopVol("spinnya", 0.85)
	m.playCat(2, "Rolling")
	// ScheduleRollFinish（hasSpun 时才挂松开判定）
	ctx.ScheduleInputRelease(target+0.75,
		func(st float64, _ engine.Judgment) { m.spinFinish() },
		func() { m.spinMissTwo() })
}

func (m *Module) spinFinish() {
	m.stopNya()
	m.stopNya = func() {}
	m.ctx.Sound("roll6")
	m.playCat(2, "RollEnd")
	m.hasSpun = false
}

func (m *Module) spinMissOne() {
	m.hasSpun = false
	m.ctx.SoundAt(m.ctx.Beat()+0.75, "roll6", 0.1)
}

func (m *Module) spinMissTwo() {
	if m.hasSpun {
		m.rollFail()
	}
	m.ctx.SoundVol("roll6", 0.3)
}

func (m *Module) rollFail() {
	m.stopNya()
	m.stopNya = func() {}
	m.ctx.PlayCommon("miss")
	m.playCat(2, "RollFail")
	m.hasSpun = false
}

// ---------- fish ----------

func (m *Module) scheduleFish(b float64) {
	ctx := m.ctx
	fish := ctx.Role("Fish")
	ctx.SoundAt(b+2, "fish1", 1)
	ctx.SoundAt(b+2.25, "fish2", 1)
	ctx.SoundAt(b+2.5, "fish3", 1)
	ctx.At(b, func() {
		ctx.Scene.SetActive(fish, true)
		ctx.Scene.PlayState(fish, "FishDangle", b, 0.5)
	})
	ctx.At(b+2, func() { m.playCat(0, "FishNotice") })
	ctx.At(b+2.25, func() { m.playCat(1, "FishNotice2") })
	ctx.At(b+2.5, func() { m.playCat(2, "FishNotice3") })
	ctx.ScheduleInput(b+2.75,
		func(st float64, _ engine.Judgment) {
			m.removeCats()
			ctx.Sound("fish4")
			ctx.Scene.PlayState(fish, "CaughtSuccess", ctx.Beat(), ctx.SecPerBeat(ctx.Beat()))
		},
		func() {
			m.removeCats()
			ctx.PlayCommon("miss")
			ctx.Scene.PlayState(fish, "CaughtFail", ctx.Beat(), ctx.SecPerBeat(ctx.Beat()))
		})
	ctx.At(b+4, func() { ctx.Scene.SetActive(fish, false) })
}

// ---------- 生成/收场 ----------

// spawn 复刻 Kitties.Spawn：定位（含 z 透视/镜像/大脸旋转）+ 子体激活 + 登场动画。
func (m *Module) spawn(typ, cat int, mice, inverse, firstSpawn, instant bool) {
	sc := m.ctx.Scene
	path := m.cats()[cat]
	idx := m.catIdx(cat)
	var p [3]float64
	if typ == 3 {
		p = facePos[cat]
		sc.SetSpinIdx(idx, faceRot[cat]*math.Pi/180)
		sc.SetMirrorX(path, true)
	} else {
		inv := 0
		if inverse {
			inv = 1
		}
		p = spawnPos[typ][inv][cat]
		sc.SetSpinIdx(idx, 0)
		sc.SetMirrorX(path, inverse)
	}
	sc.SetPosOver(path, p[0], p[1])
	sc.SetZOver(path, p[2])
	sc.SetActive(path+"/Kitty", true)

	anim := ""
	switch {
	case typ == 3 && !instant:
		anim = "FacePopIn"
	case typ == 3:
		anim = "FaceIdle"
	case mice && cat < 2 && !instant:
		anim = "MicePopIn"
	case mice && cat < 2:
		anim = "MiceIdle"
	case !instant:
		anim = "PopIn"
	default:
		anim = "Idle"
	}
	m.playCat(cat, anim)
}

func (m *Module) removeCats() {
	for _, p := range m.cats() {
		m.ctx.Scene.SetActive(p+"/Kitty", false)
	}
}

// playCat：Animator.Play(name, 0, 0)（原速：每拍推进 secPerBeat 秒）。
func (m *Module) playCat(cat int, state string) {
	b := m.ctx.Beat()
	m.ctx.Scene.PlayState(m.cats()[cat], state, b, m.ctx.SecPerBeat(b))
}

// ---------- 输入杂项 ----------

func (m *Module) OnSwitch(beat float64) {
	// PersistColor：段首应用最近一次 bgcolor
	m.bg = colorEase{c0: white, c1: white}
	for _, ce := range m.bgEvts {
		if ce.beat < beat {
			m.bg = ce
		}
	}
	m.hasSpun = false
}

// Whiff：主键空按在 Pad 风格下无惩罚（C# 仅 Touch 有反馈）。
func (m *Module) Whiff(beat float64) {}

// WhiffAction：南键（动作 1）空按 → RollFail（CtrTeppanPlayer.Update）。
func (m *Module) WhiffAction(beat float64, action int) {
	if action == 1 {
		m.rollFail()
	}
}

func (m *Module) Update(t, beat float64) {}

func (m *Module) Draw(screen *ebiten.Image, t, beat float64) {
	// bgcolor ColorEase
	norm := 1.0
	if m.bg.length > 0 {
		norm = clamp01((beat - m.bg.beat) / m.bg.length)
	}
	var c [4]float64
	for i := 0; i < 4; i++ {
		c[i] = engine.Ease(m.bg.ease, m.bg.c0[i], m.bg.c1[i], norm)
	}
	screen.Fill(toRGBA(c))
	sc := m.ctx.Scene
	m.ctx.SampleScene(beat)
	sc.Draw(screen, m.proj)
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
