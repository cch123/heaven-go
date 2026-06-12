// Package lockstep 是 Lockstep（lockstep）的玩法模块，
// 逻辑对应 Assets/Scripts/Games/Lockstep/Lockstep.cs：
//
//	onbeatSwitch/offbeatSwitch：切到整拍/反拍行进。音效为 preFunction
//	    （nha×4 / hai×3+hahai；hai 尾音被下一个 offbeatSwitch 截断、
//	    ho×4 渐弱被下一个 onbeatSwitch 截断）；背景双色闪烁序列；
//	    行进递归链 MarchRecursive 从 onbeat+2 / offbeat+3.5 启动，
//	    switch 表（事件 beat+length-1）命中时链整体移位半拍
//	stepping/marching：行进链直接启动 / 固定步数强制行进（官方关未用）
//	bop：玩家+人群 Bop（toggle2 = 每拍自动）
//	set colours：背景双色 + stepper 三色调色板（_ColorAlpha/Bravo/Delta）
//	zoom：相机 z 档位（Regular..ExtremelyFar）
//	hai/ho：单发音效
//
// 人群渲染：原版用 3 台正交相机把 stepper 棋盘格渲到 RenderTexture，
// 经 NearTop/NearBottom（1:1）与 DV 四块巨型 quad（128×128 平铺）铺满
// 世界——等价于"同尺度无限棋盘格"（x 周期 2.55、行距 3.48、隔行错位
// 1.275，相位过 (0.015, 0.24)）。这里按主 stepper（OverPlayer）的当前
// 切片直接平铺可见区域，玩家（stepswitcherP，scale 1.1）盖在其格点上。
package lockstep

import (
	"math"
	"sort"

	"github.com/hajimehoshi/ebiten/v2"

	"hsdemo/engine"
	"hsdemo/kart"
	"hsdemo/riq"
)

// 默认配色（Lockstep.cs 的静态属性）。
var (
	defBGOn  = hex(0xf0, 0x33, 0x8d)
	defBGOff = hex(0xBC, 0x31, 0x8B)
	defOut   = hex(0x9A, 0x27, 0x60)
	defDark  = hex(0x73, 0x73, 0x73)
	defLight = hex(0xFF, 0xFF, 0xFF)
)

func hex(r, g, b byte) [4]float64 {
	return [4]float64{float64(r) / 255, float64(g) / 255, float64(b) / 255, 1}
}

// 人群棋盘格参数（lockstep.prefab：stepswitchers 网格 + 相机/quad 几何）。
const (
	gridDX    = 2.55  // 同行 stepper 间距
	gridDY    = 3.48  // 行距
	gridXBase = 0.015 // 玩家行（j 偶）相位
	gridXOdd  = 1.29  // 错位行（j 奇）相位
	gridYBase = 0.24  // 玩家行世界 y
	stepScale = 1.1
)

type switchEvt struct {
	beat, length float64
	offbeat      bool
	visual       bool
	sound        bool
	hai          int  // onbeat：尾随 hai 数
	ho           bool // offbeat：渐弱 ho
}

type stepEvt struct {
	beat, length float64
	sound        bool
	amount       int
	visual       bool
	force        bool
}

type colourEvt struct {
	beat                 float64
	on, off              [4]float64
	out, dark, light     [4]float64
}

type Module struct {
	ctx  *engine.Ctx
	proj kart.Aff

	switchEvts []switchEvt
	switchSet  map[float64]bool
	colours    []colourEvt
	steppings  []stepEvt
	stops      []float64
	baches     [][2]float64

	onCol, offCol [4]float64
	offActive     bool
	goBop         bool
	altStep       bool
	missStage     int // 0=NotMissed 1=MissedOff 2=MissedOn
	zoomZ         float64
	lastPulse     float64
}

func New() engine.Module { return &Module{} }

func (m *Module) ID() string { return "lockstep" }

func (m *Module) Load(ctx *engine.Ctx) error {
	m.ctx = ctx
	if err := ctx.LoadAssets("lockstep"); err != nil {
		return err
	}
	m.proj = kart.Translate(engine.ScreenW/2, engine.ScreenH/2).Mul(kart.Scale(54, -54))
	// 人群子树不直接绘制（原版主相机只看 RT quad），由 Draw 平铺
	ctx.Scene.SetActive("stepswitchers", false)
	m.onCol, m.offCol = defBGOn, defBGOff
	m.applyStepperPalette(defOut, defDark, defLight)
	return nil
}

func (m *Module) player() string { return m.ctx.Role("stepswitcherPlayer") }
func (m *Module) master() string { return m.ctx.Role("masterStepperAnim") }

func (m *Module) applyStepperPalette(out, dark, light [4]float64) {
	p := kart.Palette{Alpha: out, Fill: dark, Outline: light}
	m.ctx.Scene.SetPaletteFor("StepperMaterial", p)
	m.ctx.Scene.SetPaletteFor("PlayerMaterial", p)
}

// ---------- 事件 ----------

func (m *Module) OnEvent(e *riq.Entity) {
	b := e.Beat
	ctx := m.ctx
	switch e.Datamodel {
	case "lockstep/bop":
		auto := boolParam(e, "toggle2")
		ctx.At(b, func() { m.goBop = auto })
		if boolParam(e, "toggle") {
			for i := 0.0; i < e.Length; i++ {
				bb := b + i
				ctx.At(bb, func() { m.stepperAnim("Bop", true, bb) })
			}
		}
	case "lockstep/onbeatSwitch":
		m.switchEvts = append(m.switchEvts, switchEvt{
			beat: b, length: e.Length, offbeat: false,
			visual: boolParam(e, "visual"), sound: boolParam(e, "sound"),
			hai: int(e.Float("hai", 0)),
		})
	case "lockstep/offbeatSwitch":
		m.switchEvts = append(m.switchEvts, switchEvt{
			beat: b, length: e.Length, offbeat: true,
			visual: boolParam(e, "visual"), sound: boolParam(e, "sound"),
			ho: boolParam(e, "ho"),
		})
	case "lockstep/stepping":
		m.steppings = append(m.steppings, stepEvt{
			beat: b, sound: boolParam(e, "sound"),
			amount: int(e.Float("amount", 1)), visual: boolParam(e, "visual"),
		})
	case "lockstep/marching":
		m.steppings = append(m.steppings, stepEvt{
			beat: b, length: e.Length, sound: boolParam(e, "sound"),
			amount: int(e.Float("amount", 1)), visual: boolParam(e, "visual"),
			force: true,
		})
	case "lockstep/stopStepping":
		m.stops = append(m.stops, b-0.5) // preFunctionLength 0.5
	case "lockstep/hai":
		ctx.SoundAtOff(b, "hai", 1, 0.018)
	case "lockstep/ho":
		ctx.SoundAtOff(b, "ho", 1, 0.015)
	case "lockstep/set colours":
		ce := colourEvt{
			beat: b,
			on:   colorParam(e, "colorA", defBGOn), off: colorParam(e, "colorB", defBGOff),
			out: colorParam(e, "objColA", defOut), dark: colorParam(e, "objColB", defDark),
			light: colorParam(e, "objColC", defLight),
		}
		m.colours = append(m.colours, ce)
		ctx.At(b, func() { m.applyColours(ce) })
	case "lockstep/zoom":
		z := [5]float64{0, -4.5, -11, -26, -63}[int(e.Float("zoom", 0))%5]
		ctx.At(b, func() { m.zoomZ = z })
	case "lockstep/bach":
		m.baches = append(m.baches, [2]float64{b, e.Length})
	}
}

func (m *Module) applyColours(ce colourEvt) {
	m.onCol, m.offCol = ce.on, ce.off
	m.applyStepperPalette(ce.out, ce.dark, ce.light)
}

// Ready：switch 表 + preFunction 音效（forcePlay，与游戏激活无关）。
func (m *Module) Ready() {
	sort.Slice(m.switchEvts, func(i, j int) bool { return m.switchEvts[i].beat < m.switchEvts[j].beat })
	sort.Slice(m.colours, func(i, j int) bool { return m.colours[i].beat < m.colours[j].beat })
	m.switchSet = map[float64]bool{}
	for _, sw := range m.switchEvts {
		m.switchSet[sw.beat+sw.length-1] = true
	}
	ctx := m.ctx
	for _, sw := range m.switchEvts {
		b := sw.beat
		if !sw.offbeat {
			// OnbeatSwitchSound：nha 节奏 + 尾随 hai（被下一 offbeatSwitch 截断）
			if sw.sound {
				ctx.SoundAt(b, "nha1", 1)
				ctx.SoundAtOff(b+0.5, "nha2", 1, 0.01)
				ctx.SoundAt(b+1, "nha1", 1)
				ctx.SoundAtOff(b+1.5, "nha2", 1, 0.01)
			}
			cut := m.nextSwitch(b, true)
			for i := 0; i < sw.hai; i++ {
				if t := b + 2 + float64(i); t < cut {
					ctx.SoundAtOff(t, "hai", 1, 0.018)
				}
			}
		} else {
			// OffbeatSwitchSound：hai×3 + hahai + 渐弱 ho×4（被下一 onbeatSwitch 截断）
			if sw.sound {
				ctx.SoundAtOff(b, "hai", 1, 0.018)
				ctx.SoundAtOff(b+1, "hai", 1, 0.018)
				ctx.SoundAtOff(b+2, "hai", 1, 0.018)
				ctx.SoundAt(b+3, "hahai1", 1)
				ctx.SoundAtOff(b+3.5, "hahai2", 1, 0.014)
			}
			if sw.ho {
				cut := m.nextSwitch(b, false)
				vols := []float64{1, 0.6835514, 0.3395127, 0.1200322}
				for i, v := range vols {
					if t := b + 4.5 + float64(i); t < cut {
						ctx.SoundAtOff(t, "ho", v, 0.015)
					}
				}
			}
		}
	}
}

// nextSwitch 返回 b 之后第一个 offbeat（wantOff）/onbeat switch 事件的拍。
func (m *Module) nextSwitch(b float64, wantOff bool) float64 {
	for _, sw := range m.switchEvts {
		if sw.beat > b && sw.offbeat == wantOff {
			return sw.beat
		}
	}
	return math.Inf(1)
}

// ---------- 段落调度（OnGameSwitch → QueueSwitches） ----------

func (m *Module) OnSwitch(beat float64) {
	ctx := m.ctx
	s, e := beat, ctx.NextSwitchBeat(beat)
	// 新实例状态（原版每段重建 Lockstep 实例）
	m.altStep, m.missStage, m.offActive, m.goBop = false, 0, false, false
	m.zoomZ = 0
	// PersistColors：段首应用最近一次 set colours
	m.onCol, m.offCol = defBGOn, defBGOff
	m.applyStepperPalette(defOut, defDark, defLight)
	for _, ce := range m.colours {
		if ce.beat < s {
			m.applyColours(ce)
		}
	}
	ctx.Scene.PlayDefaultState(m.player(), beat, 0.5)
	ctx.Scene.PlayDefaultState(m.master(), beat, 0.5)

	// 链启动候选：switch 事件（onbeat+1.5 → 链 +2；offbeat+3 → 链 +3.5）
	// 与 stepping 事件；最早者启动（marchRecursing 抑制其余）。
	type cand struct{ action, start float64 }
	var cands []cand
	for _, sw := range m.switchEvts {
		if sw.beat >= e {
			continue
		}
		b := sw.beat
		var acts [][2]float64 // (拍, 1=off 色)
		var action, start float64
		if !sw.offbeat {
			acts = [][2]float64{{b, 0}, {b + 0.5, 1}, {b + 1, 0}, {b + 1.5, 1}, {b + 2, 0}}
			action, start = b+1.5, b+2
		} else {
			acts = [][2]float64{{b, 1}, {b + 1, 0}, {b + 2, 1}, {b + 3, 0}, {b + 3.5, 1}}
			action, start = b+3, b+3.5
		}
		if sw.visual {
			for _, a := range acts {
				if a[0]+0.5 >= s && a[0] < e {
					off := a[1] != 0
					ctx.At(a[0], func() { m.setBGOff(off) })
				}
			}
		}
		if action+0.5 >= s {
			cands = append(cands, cand{action, start})
		}
	}
	for _, st := range m.steppings {
		if st.force || st.beat < s || st.beat >= e {
			continue
		}
		st := st
		cands = append(cands, cand{st.beat, st.beat})
		ctx.At(st.beat, func() {
			if st.visual {
				m.setBGOff(math.Mod(st.beat, 1) != 0)
			}
		})
		if st.sound {
			m.voiceRow(st.beat, st.amount)
		}
	}
	sort.Slice(cands, func(i, j int) bool { return cands[i].action < cands[j].action })
	if len(cands) > 0 {
		m.walkChain(cands[0].start, e)
	}
	// 强制行进（marching）：固定步数、固定奇偶
	for _, st := range m.steppings {
		if !st.force || st.beat >= e || st.beat+st.length <= s {
			continue
		}
		st := st
		off := math.Mod(st.beat, 1) != 0
		if st.sound {
			m.voiceRow(st.beat, st.amount)
		}
		if st.visual {
			ctx.At(st.beat, func() { m.setBGOff(off) })
		}
		for i := 0.0; i < st.length; i++ {
			if b := st.beat + i; b < e {
				m.scheduleStep(b, off)
			}
		}
	}
}

// voiceRow：stepping 起步语音（整拍 hai / 反拍 ho × amount）。
func (m *Module) voiceRow(b float64, amount int) {
	off := math.Mod(b, 1) != 0
	name, offset := "hai", 0.018
	if off {
		name, offset = "ho", 0.015
	}
	for i := 0; i < amount; i++ {
		m.ctx.SoundAtOff(b+float64(i), name, 1, offset)
	}
}

// walkChain：MarchRecursive 的静态展开（switch 表命中时移位半拍，
// 链步到段尾/stopStepping 为止）。
func (m *Module) walkChain(start, e float64) {
	b := start
	for {
		if m.switchSet[b-0.5] {
			b -= 0.5
		}
		if b >= e || m.stopped(b-1) {
			return
		}
		m.scheduleStep(b, math.Mod(b, 1) != 0)
		b++
	}
}

func (m *Module) stopped(atBeat float64) bool {
	for _, sb := range m.stops {
		if atBeat >= sb {
			return true
		}
	}
	return false
}

// scheduleStep：一步行进 = 脚步声（foot1/2 交替）+ 人群踏步动画 + 玩家判定。
func (m *Module) scheduleStep(b float64, off bool) {
	ctx := m.ctx
	foot := "foot1"
	if m.altStep {
		foot = "foot2"
	}
	m.altStep = !m.altStep
	ctx.SoundAt(b, foot, 1)
	state := "OnbeatMarch"
	if off {
		state = "OffbeatMarch"
	}
	ctx.At(b, func() {
		ctx.Scene.PlayState(m.master(), state, b, 0.5)
		if m.bachAt(b) {
			bach := "BachOn"
			if off {
				bach = "BachOff"
			}
			ctx.Scene.PlayState(m.ctx.Role("bach"), bach, b, 0.5)
		}
	})
	ctx.ScheduleInput(b, func(st float64, _ engine.Judgment) { m.hit(st, off) },
		func() { m.judgedMiss(off) })
}

func (m *Module) bachAt(b float64) bool {
	for _, ev := range m.baches {
		if b >= ev[0] && b < ev[0]+ev[1] {
			return true
		}
	}
	return false
}

func (m *Module) hit(state float64, off bool) {
	ctx := m.ctx
	m.missStage = 0
	anim := "OnbeatMarch"
	if off {
		anim = "OffbeatMarch"
	}
	ctx.Scene.PlayState(m.player(), anim, ctx.Beat(), 0.5)
	if state >= 1 || state <= -1 {
		ctx.PlayCommon("nearMiss")
		return
	}
	if off {
		ctx.Sound("drumOff")
	} else {
		ctx.Sound("drumOn")
	}
}

func (m *Module) judgedMiss(off bool) {
	ctx := m.ctx
	want := 2 // MissedOn
	anim := "OnbeatMiss"
	if off {
		want, anim = 1, "OffbeatMiss"
	}
	if m.missStage == want {
		return
	}
	m.missStage = want
	// Play(anim, 0, 0)：原速播放（每拍推进 secPerBeat 秒）
	ctx.Scene.PlayState(m.player(), anim, ctx.Beat(), ctx.SecPerBeat(ctx.Beat()))
	ctx.Sound("wayOff")
}

func (m *Module) setBGOff(off bool) { m.offActive = off }

// stepperAnim：PlayStepperAnim（玩家 + 人群主 stepper）。
func (m *Module) stepperAnim(state string, player bool, beat float64) {
	if player {
		m.ctx.Scene.PlayState(m.player(), state, beat, 0.5)
	}
	m.ctx.Scene.PlayState(m.master(), state, beat, 0.5)
}

// Whiff：无判定窗口内按键（Update 的 GetIsAction 分支）。
func (m *Module) Whiff(beat float64) {
	ctx := m.ctx
	m.missStage = 0
	anim := "OffbeatMarch"
	if math.Mod(math.Mod(beat-0.25, 1)+1, 1) >= 0.5 {
		anim = "OnbeatMarch"
	}
	ctx.Scene.PlayState(m.player(), anim, beat, 0.5)
	ctx.PlayCommon("miss")
	ctx.ScoreMiss()
}

func (m *Module) Update(t, beat float64) {
	// OnBeatPulse：goBop 时每整拍 Bop
	if m.goBop {
		if p := math.Floor(beat); p > m.lastPulse {
			m.lastPulse = p
			m.stepperAnim("Bop", true, p)
		}
	}
}

// ---------- 绘制 ----------

func (m *Module) Draw(screen *ebiten.Image, t, beat float64) {
	bg := m.onCol
	if m.offActive {
		bg = m.offCol
	}
	screen.Fill(toRGBA(bg))

	ctx := m.ctx
	sc := ctx.Scene
	cam := ctx.CameraAt(beat)
	camZ := cam[2] + m.zoomZ
	sc.SetCamera(cam[0], cam[1], camZ)
	sc.Sample(beat)

	// 人群棋盘格平铺（主 stepper 当前帧；行越靠下绘制越靠前，全部在玩家之下）
	sprite, fx, fy := sc.NodeSprite(m.master())
	if sprite != "" {
		halfH := -camZ/2 + 3.5
		halfW := halfH * float64(engine.ScreenW) / float64(engine.ScreenH)
		jMin := int(math.Floor((cam[1] - halfH - gridYBase) / gridDY))
		jMax := int(math.Ceil((cam[1] + halfH - gridYBase) / gridDY))
		for j := jMin; j <= jMax; j++ {
			y := gridYBase + float64(j)*gridDY
			x0 := gridXBase
			if j%2 != 0 {
				x0 = gridXOdd
			}
			kMin := int(math.Floor((cam[0] - halfW - x0) / gridDX))
			kMax := int(math.Ceil((cam[0] + halfW - x0) / gridDX))
			for k := kMin; k <= kMax; k++ {
				sc.Queue(kart.ExtraSprite{
					Sprite: sprite,
					World:  kart.Translate(x0+float64(k)*gridDX, y).Mul(kart.Scale(stepScale, stepScale)),
					Order:  -100 - j,
					FlipX:  fx, FlipY: fy,
					Mapped: true, Mat: "StepperMaterial",
				})
			}
		}
	}
	sc.Draw(screen, m.proj)
}

// ---------- 工具 ----------

type colorRGBA struct{ R, G, B, A uint8 }

func toRGBA(c [4]float64) (out colorRGBA) {
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

func boolParam(e *riq.Entity, key string) bool {
	return e.Float(key, 0) != 0
}

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
