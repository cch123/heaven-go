// Package spacedance 是 Space Dance（spaceDance）的玩法模块，
// 逻辑对应 Assets/Scripts/Games/SpaceDance/SpaceDance.cs：
//
//	turn right：B 起手（TurnRightStart），B+1 判定（→ 键，通道 2）
//	sit down：  B 起手（SitDownStart），B+1 判定（↓ 键，通道 3）
//	punch：     B/B+0.5/B+1 蓄拳交替，B+1.5 判定（主键）
//	bop：       区间语义（auto 参数划定自动 bop 区），dancers/gramps 分控
//	changeBG：  背景色 ColorEase；charColor：舞者/老爹三色调色板
//	scroll：    三层视差星空 UV 滚动（Canvas RawImage 9×9 平铺等价）
//	shootingStar / grampsAnims：流星与老爹说话/吸气循环
//
// 注：官方谱面遗留的 spaceDance/bopToggle 在现行 HS 无处理器（加载即忽略），
// 此处同样忽略。
package spacedance

import (
	"math"

	"github.com/hajimehoshi/ebiten/v2"

	"hsdemo/engine"
	"hsdemo/kart"
	"hsdemo/riq"
)

var defaultBG = [4]float64{0, 0.161, 0.839, 1}

// 调色板默认（SpaceDance.cs 静态色）。
var (
	dancerDef = kart.Palette{
		Alpha:   hex(0x6B, 0xF7, 0xFF),
		Fill:    hex(0xFF, 0xFF, 0xFF),
		Outline: hex(0xFD, 0xE6, 0x0D),
	}
	grampsDef = kart.Palette{
		Alpha:   hex(0xFF, 0xFF, 0x4A),
		Fill:    hex(0xFF, 0xFF, 0xFF),
		Outline: hex(0xFF, 0xFF, 0xFF),
	}
)

func hex(r, g, b byte) [4]float64 {
	return [4]float64{float64(r) / 255, float64(g) / 255, float64(b) / 255, 1}
}

// 星空层（BG/Canvas/Stars0..2：RawImage 2040×1496@100ppu，UV 9×9 平铺）。
const starTileW, starTileH = 20.4, 14.96

var starLayers = []struct {
	sprite string
	z      float64
	order  int
}{
	{"BGStarsUS2", 6, -4}, // Stars2（最远）
	{"BGStarsUS1", 3, -3}, // Stars1
	{"BGStarsUS2", 0, -2}, // Stars0
}

type colorEase struct {
	beat, length float64
	c0, c1       [4]float64
	ease         int
}

type bopEvt struct {
	beat, length             float64
	bop, auto, gramps, gAuto bool
}

type starEvt struct {
	beat, length float64
	ease         int
}

type Module struct {
	ctx  *engine.Ctx
	proj kart.Aff

	bg     colorEase
	bgEvts []colorEase
	bops   []bopEvt

	canBop      bool
	grampsBop   bool // grampsCanBop
	grampsAuto  bool // spaceGrampsShouldBop
	lastPulse   float64
	grampsLoop  int // 0=无 1=talk 2=sniff
	star        starEvt
	hasStar     bool

	xMult, yMult   float64
	normX, normY   float64
	lastT          float64
	hasLastT       bool
}

func New() engine.Module {
	return &Module{canBop: true, grampsBop: true, bg: colorEase{c0: defaultBG, c1: defaultBG}}
}

func (m *Module) ID() string { return "spaceDance" }

func (m *Module) Load(ctx *engine.Ctx) error {
	m.ctx = ctx
	if err := ctx.LoadAssets("spaceDance"); err != nil {
		return err
	}
	m.proj = kart.Translate(engine.ScreenW/2, engine.ScreenH/2).Mul(kart.Scale(54, -54))
	ctx.Scene.SetPaletteFor("DancerMat", dancerDef)
	ctx.Scene.SetPaletteFor("GrampsMat", grampsDef)
	return nil
}

func (m *Module) playerD() string { return m.ctx.Role("DancerP") }
func (m *Module) gramps() string  { return m.ctx.Role("Gramps") }

func (m *Module) dancers() []string {
	return []string{m.ctx.Role("DancerP"), m.ctx.Role("Dancer1"), m.ctx.Role("Dancer2"), m.ctx.Role("Dancer3")}
}

func (m *Module) others() []string {
	return []string{m.ctx.Role("Dancer1"), m.ctx.Role("Dancer2"), m.ctx.Role("Dancer3")}
}

// ---------- 事件 ----------

func (m *Module) OnEvent(e *riq.Entity) {
	b := e.Beat
	ctx := m.ctx
	switch e.Datamodel {
	case "spaceDance/bop":
		be := bopEvt{b, e.Length, boolParam(e, "bop"), boolParam(e, "auto"),
			boolParam(e, "gramps"), boolParam(e, "grampsAuto")}
		m.bops = append(m.bops, be)
		ctx.At(b, func() { m.grampsAuto = be.gAuto })
		if be.bop || be.gramps {
			for i := 0.0; i < be.length; i++ {
				bb := b + i
				ctx.At(bb, func() {
					if be.bop {
						m.bopDancers(bb)
					}
					if be.gramps {
						m.bopGramps(bb)
					}
				})
			}
		}
	case "spaceDance/turn right":
		m.turnRightSfx(b, int(e.Float("whoSpeaks", 0)))
		m.doTurnRight(b, boolParam(e, "gramps"))
	case "spaceDance/sit down":
		m.sitDownSfx(b, int(e.Float("whoSpeaks", 0)))
		m.doSitDown(b, boolParam(e, "gramps"))
	case "spaceDance/punch":
		m.punchSfx(b, int(e.Float("whoSpeaks", 0)))
		m.doPunch(b, boolParam(e, "gramps"))
	case "spaceDance/changeBG":
		ce := colorEase{
			beat: b, length: e.Length,
			c0: colorParam(e, "start", defaultBG), c1: colorParam(e, "end", defaultBG),
			ease: int(e.Float("ease", 0)),
		}
		m.bgEvts = append(m.bgEvts, ce)
		ctx.At(b, func() { m.bg = ce })
	case "spaceDance/charColor":
		dp := kart.Palette{Alpha: colorParam(e, "dMain", dancerDef.Alpha),
			Fill: colorParam(e, "dFace", dancerDef.Fill), Outline: colorParam(e, "dHand", dancerDef.Outline)}
		gp := kart.Palette{Alpha: colorParam(e, "gMain", grampsDef.Alpha),
			Fill: colorParam(e, "gFace", grampsDef.Fill), Outline: colorParam(e, "gMouth", grampsDef.Outline)}
		ctx.At(b, func() {
			ctx.Scene.SetPaletteFor("DancerMat", dp)
			ctx.Scene.SetPaletteFor("GrampsMat", gp)
		})
	case "spaceDance/scroll":
		x, y := e.Float("x", 0), e.Float("y", 0)
		ctx.At(b, func() { m.xMult, m.yMult = x, y })
	case "spaceDance/shootingStar":
		se := starEvt{b, e.Length, int(e.Float("ease", 0))}
		ctx.At(b, func() { m.star, m.hasStar = se, true })
	case "spaceDance/grampsAnims":
		typ, loop := int(e.Float("type", 1)), boolParam(e, "toggle")
		ctx.At(b, func() { m.grampsAnims(b, typ, loop) })
	case "spaceDance/bopToggle":
		// 旧版事件：现行 HS 无处理器（加载即忽略），保持一致
	}
}

func (m *Module) Ready() {}

// ---------- bop ----------

// autoBopAt：SetupBopRegion("spaceDance","bop","auto") 的区间语义。
func (m *Module) autoBopAt(beat float64) bool {
	on := false
	for _, be := range m.bops {
		if be.beat > beat {
			break
		}
		on = be.auto
	}
	return on
}

func (m *Module) bopDancers(beat float64) {
	if !m.canBop {
		return
	}
	for _, d := range m.dancers() {
		m.ctx.Scene.PlayState(d, "Bop", beat, 0.5)
	}
}

func (m *Module) bopGramps(beat float64) {
	if !m.grampsBop {
		return
	}
	m.ctx.Scene.PlayState(m.gramps(), "GrampsBop", beat, 0.5)
}

// ---------- turn right / sit down / punch ----------

func (m *Module) turnRightSfx(b float64, who int) {
	ctx := m.ctx
	ctx.SoundAt(b, "voicelessTurn", 1)
	if who == 0 || who == 2 {
		ctx.SoundAt(b, "dancerTurn", 1)
		ctx.SoundAtOff(b+1, "dancerRight", 1, 0.012)
	}
	if who == 1 || who == 2 {
		ctx.SoundAt(b, "otherTurn", 1)
		ctx.SoundAtOff(b+1, "otherRight", 1, 0.005)
	}
}

func (m *Module) doTurnRight(b float64, grampsTurns bool) {
	ctx := m.ctx
	ctx.At(b, func() {
		m.canBop = false
		if grampsTurns {
			m.grampsBop = false
		}
		for _, d := range m.dancers() {
			ctx.Scene.PlayState(d, "TurnRightStart", b, 0.5)
		}
		if grampsTurns {
			ctx.Scene.PlayState(m.gramps(), "GrampsTurnRightStart", b, 0.5)
		}
	})
	ctx.At(b+1, func() {
		for _, d := range m.others() {
			ctx.Scene.PlayState(d, "TurnRightDo", b+1, 0.5)
		}
		if grampsTurns {
			ctx.Scene.PlayState(m.gramps(), "GrampsTurnRightDo", b+1, 0.5)
		}
	})
	ctx.At(b+1.5, func() { m.canBop, m.grampsBop = true, true })
	ctx.ScheduleInputAction(b+1, 2,
		func(st float64, _ engine.Judgment) { m.just(st, "TurnRightDo") },
		func() { m.miss("HitTurn") })
}

func (m *Module) sitDownSfx(b float64, who int) {
	ctx := m.ctx
	ctx.SoundAt(b, "voicelessSit", 1)
	if who == 0 || who == 2 {
		ctx.SoundAtOff(b, "dancerLets", 1, 0.055)
		ctx.SoundAtOff(b+0.5, "dancerSit", 1, 0.05)
		ctx.SoundAtOff(b+1, "dancerDown", 1, 0.004)
	}
	if who == 1 || who == 2 {
		ctx.SoundAtOff(b, "otherLets", 1, 0.02)
		ctx.SoundAtOff(b+0.5, "otherSit", 1, 0.064)
		ctx.SoundAtOff(b+1, "otherDown", 1, 0.01)
	}
}

func (m *Module) doSitDown(b float64, grampsSits bool) {
	ctx := m.ctx
	ctx.At(b, func() {
		m.canBop = false
		if grampsSits {
			m.grampsBop = false
		}
		for _, d := range m.dancers() {
			ctx.Scene.PlayState(d, "SitDownStart", b, 0.5)
		}
		if grampsSits {
			ctx.Scene.PlayState(m.gramps(), "GrampsSitDownStart", b, 0.5)
		}
	})
	ctx.At(b+1, func() {
		for _, d := range m.others() {
			ctx.Scene.PlayState(d, "SitDownDo", b+1, 0.5)
		}
		if grampsSits {
			ctx.Scene.PlayState(m.gramps(), "GrampsSitDownDo", b+1, 0.5)
		}
	})
	ctx.At(b+1.5, func() { m.canBop, m.grampsBop = true, true })
	ctx.ScheduleInputAction(b+1, 3,
		func(st float64, _ engine.Judgment) { m.just(st, "SitDownDo") },
		func() { m.miss("HitSit") })
}

func (m *Module) punchSfx(b float64, who int) {
	ctx := m.ctx
	for i := 0.0; i < 1.5; i += 0.5 {
		ctx.SoundAt(b+i, "voicelessPunch", 1)
	}
	if who == 0 || who == 2 {
		ctx.SoundAt(b, "dancerPa", 1)
		ctx.SoundAt(b+0.5, "dancerPa", 1)
		ctx.SoundAt(b+1, "dancerPa", 1)
		ctx.SoundAt(b+1.5, "dancerPunch", 1)
	}
	if who == 1 || who == 2 {
		ctx.SoundAt(b, "otherPa", 1)
		ctx.SoundAt(b+0.5, "otherPa", 1)
		ctx.SoundAt(b+1, "otherPa", 1)
		ctx.SoundAt(b+1.5, "otherPunch", 1)
	}
}

func (m *Module) doPunch(b float64, grampsPunches bool) {
	ctx := m.ctx
	inner := func(bb float64) func() {
		return func() {
			for _, d := range m.dancers() {
				ctx.Scene.PlayState(d, "PunchStartInner", bb, 0.5)
			}
			if grampsPunches {
				ctx.Scene.PlayState(m.gramps(), "GrampsPunchStartOdd", bb, 0.5)
			}
		}
	}
	ctx.At(b, func() {
		m.canBop = false
		if grampsPunches {
			m.grampsBop = false
		}
		inner(b)()
	})
	ctx.At(b+0.5, func() {
		for _, d := range m.dancers() {
			ctx.Scene.PlayState(d, "PunchStartOuter", b+0.5, 0.5)
		}
		if grampsPunches {
			ctx.Scene.PlayState(m.gramps(), "GrampsPunchStartEven", b+0.5, 0.5)
		}
	})
	ctx.At(b+1, inner(b+1))
	ctx.At(b+1.5, func() {
		for _, d := range m.others() {
			ctx.Scene.PlayState(d, "PunchDo", b+1.5, 0.5)
		}
		if grampsPunches {
			ctx.Scene.PlayState(m.gramps(), "GrampsPunchDo", b+1.5, 0.5)
		}
	})
	ctx.At(b+2.5, func() { m.canBop, m.grampsBop = true, true })
	ctx.ScheduleInputAction(b+1.5, 0,
		func(st float64, _ engine.Judgment) { m.just(st, "PunchDo") },
		func() { m.miss("HitPunch") })
}

func (m *Module) just(state float64, doAnim string) {
	ctx := m.ctx
	now := ctx.Beat()
	if state >= 1 || state <= -1 {
		ctx.Sound("inputBad")
		ctx.Scene.PlayState(m.playerD(), doAnim, now, 0.5)
		ctx.Scene.PlayState(m.gramps(), "GrampsOhFuck", now, 0.5)
		return
	}
	ctx.Sound("inputGood")
	ctx.Scene.PlayState(m.playerD(), doAnim, now, 0.5)
}

func (m *Module) miss(hitAnim string) {
	ctx := m.ctx
	now := ctx.Beat()
	ctx.Sound("inputBad2")
	ctx.Scene.PlayState(m.playerD(), "Ouch", now, 0.5)
	ctx.Scene.PlayState(ctx.Role("Hit"), hitAnim, now, ctx.SecPerBeat(now))
	ctx.Scene.PlayState(m.gramps(), "GrampsMiss", now, 0.5)
}

// ---------- gramps anims ----------

func (m *Module) grampsAnims(b float64, typ int, loop bool) {
	ctx := m.ctx
	switch typ {
	case 0: // Stand
		ctx.Scene.PlayState(m.gramps(), "GrampsStand", b, ctx.SecPerBeat(b))
		m.grampsLoop = 0
	case 1: // Talk
		if loop {
			m.grampsLoop = 1
			m.grampsTalkLoop(b)
		} else {
			m.grampsLoop = 0
			ctx.Scene.PlayState(m.gramps(), "GrampsTalk", b, 0.5)
		}
	case 2: // Sniff
		if loop {
			m.grampsLoop = 2
			m.grampsSniffLoop(b)
		} else {
			m.grampsLoop = 0
			ctx.Scene.PlayState(m.gramps(), "GrampsSniff", b, 0.5)
		}
	}
}

func (m *Module) grampsTalkLoop(b float64) {
	if m.grampsLoop != 1 {
		return
	}
	m.grampsAuto = false
	ctx := m.ctx
	for _, off := range []float64{0.66666, 1.33333, 2, 3, 3.5} {
		bb := b + off
		ctx.At(bb, func() {
			if m.grampsLoop == 1 {
				ctx.Scene.PlayState(m.gramps(), "GrampsTalk", bb, 0.5)
			}
		})
	}
	ctx.At(b+4, func() { m.grampsTalkLoop(b + 4) })
}

func (m *Module) grampsSniffLoop(b float64) {
	if m.grampsLoop != 2 {
		return
	}
	m.grampsAuto = false
	ctx := m.ctx
	for _, off := range []float64{0, 3, 3.5} {
		bb := b + off
		ctx.At(bb, func() {
			if m.grampsLoop == 2 {
				ctx.Scene.PlayState(m.gramps(), "GrampsSniff", bb, 0.5)
			}
		})
	}
	ctx.At(b+5.5, func() { m.grampsSniffLoop(b + 5.5) })
}

// ---------- 引擎接口 ----------

func (m *Module) OnSwitch(beat float64) {
	m.canBop, m.grampsBop = true, true
	m.lastPulse = math.Floor(beat)
	m.hasLastT = false
	// PersistColor
	m.bg = colorEase{c0: defaultBG, c1: defaultBG}
	for _, ce := range m.bgEvts {
		if ce.beat < beat {
			m.bg = ce
		}
	}
}

// WhiffAction：无判定空按（Update 的 GetIsAction 分支，按通道分动画；
// DancerP 正在 Do 动画时不响应）。
func (m *Module) WhiffAction(beat float64, action int) {
	ctx := m.ctx
	if st, ok := ctx.Scene.StateInfo(m.playerD(), beat); ok {
		if st == "PunchDo" || st == "TurnRightDo" || st == "SitDownDo" {
			return
		}
	}
	anim := map[int]string{0: "PunchDo", 2: "TurnRightDo", 3: "SitDownDo"}[action]
	if anim == "" {
		anim = "PunchDo"
	}
	ctx.Sound("inputBad")
	ctx.Scene.PlayState(m.playerD(), anim, beat, 0.5)
	ctx.Scene.PlayState(m.gramps(), "GrampsOhFuck", beat, ctx.SecPerBeat(beat))
}

func (m *Module) Whiff(beat float64) { m.WhiffAction(beat, 0) }

func (m *Module) Update(t, beat float64) {
	// OnBeatPulse：自动 bop 区
	if p := math.Floor(beat); p > m.lastPulse {
		m.lastPulse = p
		if m.autoBopAt(p) {
			m.bopDancers(p)
		}
		if m.grampsAuto {
			m.bopGramps(p)
		}
	}
	// CanvasScroll：UV 滚动累计
	if m.hasLastT {
		dt := t - m.lastT
		m.normX -= m.xMult * dt
		m.normY -= m.yMult * dt
	}
	m.lastT, m.hasLastT = t, true
	// shootingStar
	if m.hasStar {
		norm := 0.0
		if m.star.length > 0 {
			norm = (beat - m.star.beat) / m.star.length
		}
		if norm > 1 {
			m.hasStar = false
		} else if norm >= 0 {
			pos := engine.Ease(m.star.ease, 0, 1, norm)
			m.ctx.Scene.PlayNormalized(m.ctx.Role("shootingStarAnim"), "Animations/ShootingStar", pos)
		}
	}
}

func (m *Module) Draw(screen *ebiten.Image, t, beat float64) {
	// changeBG ColorEase
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
	sc.Sample(beat)

	// 三层视差星空（UV 滚动 → 世界平移，按层平铺铺满可视区）
	offX := math.Mod(-m.normX*starTileW, starTileW)
	offY := math.Mod(-m.normY*starTileH, starTileH)
	for _, l := range starLayers {
		// 透视下可视半宽放大 (z+10)/10 倍，平铺覆盖
		s := (l.z + 10) / 10
		halfW := 8.9*s + starTileW
		halfH := 5.0*s + starTileH
		for x := -halfW; x <= halfW; x += starTileW {
			for y := -halfH; y <= halfH; y += starTileH {
				tx := x + offX
				ty := y + offY
				sc.Queue(kart.ExtraSprite{
					Sprite: l.sprite,
					World:  kart.Translate(tx, ty),
					Z:      l.z,
					Order:  l.order,
				})
			}
		}
	}
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
