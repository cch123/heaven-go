// Package taptrial 是 Tap Trial（tapTrial）的玩法模块，
// 逻辑对应 Assets/Scripts/Games/TapTrial/TapTrial.cs + TapTrialPlayer.cs：
//
//	tap：        B 备拍（ook），B+1 判定（tapMonkey 1.4 倍音调 0.5 音量）
//	double tap： B/B+0.5 备拍（ookook×2），B+1、B+1.5 两连判定
//	triple tap： B/B+0.5 摆姿（ooki1/2），B+2/2.5/3 三连判定（左右交替）
//	jump tap (prep)：备跳；B 起跳（EaseOutQuad 上 0.5 拍 / EaseInQuad 下
//	    0.5 拍，玩家 4 猴 3 单位），B+1 空中判定；final/alt 收尾姿势
//	scroll event：背景 UV 加速滚动 + 白闪（maxScrollSpeed 0.25、加速度
//	    0.01、flash 上限 0.8）；失误/near 时 ResetScroll
//	giraffe events：长颈鹿 Enter/Exit（DoNormalizedAnimation+ease）/Blink
//	bop / background color / char color：bop 区间、五组色 ColorEase、
//	    girl/monkey 调色板（_ColorAlpha/Bravo/Delta）
//
// 猴拍手/玩家拍手粒子为等价手写实现（小星星迸散）。
package taptrial

import (
	"math"
	"math/rand"

	"github.com/hajimehoshi/ebiten/v2"

	"hsdemo/engine"
	"hsdemo/kart"
	"hsdemo/riq"
)

func hexc(r, g, b byte) [4]float64 {
	return [4]float64{float64(r) / 255, float64(g) / 255, float64(b) / 255, 1}
}

var (
	defA      = hexc(181, 255, 206)
	defB      = hexc(132, 231, 165)
	defC      = hexc(148, 255, 181)
	defStageA = hexc(165, 231, 255)
	defStageB = hexc(74, 140, 189)
	charDef   = [6][4]float64{
		hexc(0xFF, 0xFF, 0xFF), hexc(0xE7, 0x00, 0x10), hexc(0xF7, 0xB5, 0xB5),
		hexc(0xEF, 0x6B, 0x21), hexc(0xFF, 0xFF, 0xFF), hexc(0xF7, 0xB5, 0xB5),
	}
)

const (
	jumpHeight       = 4.0
	monkeyJumpHeight = 3.0
	maxFlashOpacity  = 0.8
	maxScrollSpeed   = 0.25
	accelSpeed       = 0.01
)

type bgEvt struct {
	beat, length float64
	from, to     [5][4]float64
	ease         int
}

type star struct {
	x, y, vx, vy, born float64
}

type Module struct {
	ctx  *engine.Ctx
	proj kart.Aff

	bops []struct {
		beat float64
		auto bool
	}
	cues []struct{ beat, end float64 } // tap 系事件区间（canBop 恢复判断）

	bg        bgEvt
	bgEvts    []bgEvt
	canBop    bool
	lastPulse float64
	jumpBeat  float64

	scrolling, flashing bool
	scrollMult          float64
	scrollSpeed         float64
	scrollY             float64
	lastT               float64
	hasLastT            bool

	giraffeAnim int // 0 Enter 1 Exit 2 Blink
	giraffeBeat float64
	giraffeLen  float64
	giraffeEase int
	tripleTaps  int
	stars       []star
}

func New() engine.Module {
	m := &Module{canBop: true, jumpBeat: math.Inf(-1), giraffeBeat: -1}
	m.bg = bgEvt{from: [5][4]float64{defA, defB, defC, defStageA, defStageB},
		to: [5][4]float64{defA, defB, defC, defStageA, defStageB}, ease: 1}
	return m
}

func (m *Module) ID() string { return "tapTrial" }

func (m *Module) Load(ctx *engine.Ctx) error {
	m.ctx = ctx
	if err := ctx.LoadAssets("tapTrial"); err != nil {
		return err
	}
	m.proj = kart.Translate(engine.ScreenW/2, engine.ScreenH/2).Mul(kart.Scale(54, -54))
	m.recolorChars(charDef)
	return nil
}

func (m *Module) recolorChars(c [6][4]float64) {
	sc := m.ctx.Scene
	sc.SetPaletteFor("tapgirl", kart.Palette{Alpha: c[1], Fill: c[0], Outline: c[2]})
	sc.SetPaletteFor("tapmonkeys", kart.Palette{Alpha: c[3], Fill: c[4], Outline: c[5]})
}

func (m *Module) monkeys() []string { return []string{m.ctx.Role("monkeyL"), m.ctx.Role("monkeyR")} }

func (m *Module) playMonkeys(state string, beat float64) {
	for _, p := range m.monkeys() {
		m.ctx.Scene.PlayState(p, state, beat, 0.5)
	}
}

func (m *Module) playPlayer(state string, beat float64) {
	m.ctx.Scene.PlayState(m.ctx.Role("player"), state, beat, 0.5)
}

// ---------- 事件 ----------

func (m *Module) OnEvent(e *riq.Entity) {
	b := e.Beat
	ctx := m.ctx
	switch e.Datamodel {
	case "tapTrial/bop":
		auto := boolParam(e, "toggle2")
		m.bops = append(m.bops, struct {
			beat float64
			auto bool
		}{b, auto})
		if boolParam(e, "toggle") {
			for i := 0.0; i < e.Length; i++ {
				bb := b + i
				ctx.At(bb, func() { m.singleBop(bb) })
			}
		}
	case "tapTrial/tap":
		m.cue(b, b+2)
		ctx.SoundAt(b, "ook", 1)
		ctx.At(b, func() {
			m.playMonkeys("TapPrepare", b)
			m.playPlayer("TapPrepare", b)
		})
		m.tapMonkeySound(b + 1)
		ctx.At(b+1, func() { m.playMonkeys("Tap", b+1); m.monkeyStars() })
		m.restoreBop(b + 1.5)
		ctx.ScheduleInput(b+1, func(st float64, _ engine.Judgment) { m.tapHit(st, "Tap") }, m.missCue)
	case "tapTrial/double tap":
		m.cue(b, b+2)
		ctx.SoundAt(b, "ookook", 1)
		ctx.SoundAt(b+0.5, "ookook", 1)
		ctx.At(b, func() {
			m.playMonkeys("DoubleTapPrepare", b)
			m.playPlayer("DoubleTapPrepare", b)
		})
		ctx.At(b+0.5, func() { m.playMonkeys("DoubleTapPrepare_2", b+0.5) })
		m.tapMonkeySound(b + 1)
		m.tapMonkeySound(b + 1.5)
		ctx.At(b+1, func() { m.playMonkeys("DoubleTap", b+1); m.monkeyStars() })
		ctx.At(b+1.5, func() { m.playMonkeys("DoubleTap", b+1.5); m.monkeyStars() })
		m.restoreBop(b + 1.5)
		for _, t := range []float64{1, 1.5} {
			ctx.ScheduleInput(b+t, func(st float64, _ engine.Judgment) { m.tapHit(st, "DoubleTap") }, m.missCue)
		}
	case "tapTrial/triple tap":
		m.cue(b, b+4)
		ctx.SoundAt(b, "ooki1", 1)
		ctx.SoundAt(b+0.5, "ooki2", 1)
		ctx.At(b, func() {
			m.tripleTaps = 0
			m.playMonkeys("PostPrepare_1", b)
			m.playPlayer("PosePrepare_1", b)
		})
		ctx.At(b+0.5, func() {
			m.playMonkeys("PostPrepare_2", b+0.5)
			m.playPlayer("PosePrepare_2", b+0.5)
		})
		for i, t := range []float64{2, 2.5, 3} {
			anims := []string{"PostTap", "PostTap_2", "PostTap"}[i]
			m.tapMonkeySound(b + t)
			bb := b + t
			ctx.At(bb, func() { m.playMonkeys(anims, bb); m.monkeyStars() })
			ctx.ScheduleInput(bb, func(st float64, _ engine.Judgment) { m.tripleHit(st) }, m.missCue)
		}
	case "tapTrial/jump tap prep":
		ctx.At(b, func() {
			m.canBop = false
			m.playMonkeys("JumpPrepare", b)
			m.playPlayer("JumpPrepare", b)
		})
	case "tapTrial/jump tap":
		final := boolParam(e, "final")
		alt := boolParam(e, "altfinalpose")
		m.cue(b, b+2)
		snd := "jumptap1"
		if final {
			snd = "jumptap2"
		}
		ctx.SoundAt(b, snd, 1)
		m.tapMonkeySound(b + 1)
		ctx.At(b, func() {
			m.jumpBeat = b
			pAnim, mAnim := "JumpTap", "JumpTap"
			if final {
				pAnim, mAnim = "FinalJump", "Jump"
			}
			m.playPlayer(pAnim, b)
			m.playMonkeys(mAnim, b)
		})
		ctx.At(b+1, func() {
			mAnim := "Jumpactualtap"
			if final {
				mAnim = "FinalJumpTap"
				if alt {
					mAnim = "FinalJumpTapAlt"
				}
			}
			m.playMonkeys(mAnim, b+1)
			m.monkeyStars()
			m.monkeyStars()
		})
		if final {
			m.restoreBopFinal(b + 1.5)
		} else {
			m.restoreBopNever(b + 1.5)
		}
		ctx.ScheduleInput(b+1,
			func(st float64, _ engine.Judgment) { m.jumpHit(st, final, alt) },
			func() { m.jumpMiss(final) })
	case "tapTrial/scroll event":
		on, fl := boolParam(e, "toggle"), boolParam(e, "flash")
		mult := e.Float("m", 1)
		ctx.At(b, func() {
			m.scrolling, m.flashing, m.scrollMult = on, fl, mult
			m.resetScroll()
		})
	case "tapTrial/background color":
		be := bgEvt{beat: b, length: e.Length, ease: int(e.Float("ease", 0))}
		keysF := []string{"colorFrom", "colorFrom2", "colorFrom3", "colorFromStage", "colorFromStage2"}
		keysT := []string{"colorTo", "colorTo2", "colorTo3", "colorToStage", "colorToStage2"}
		defs := [5][4]float64{defA, defB, defC, defStageA, defStageB}
		for i := 0; i < 5; i++ {
			be.from[i] = colorParam(e, keysF[i], defs[i])
			be.to[i] = colorParam(e, keysT[i], defs[i])
		}
		m.bgEvts = append(m.bgEvts, be)
		ctx.At(b, func() { m.bg = be })
	case "tapTrial/char color":
		var c [6][4]float64
		for i := 0; i < 6; i++ {
			c[i] = colorParam(e, []string{"color1", "color2", "color3", "color4", "color5", "color6"}[i], charDef[i])
		}
		ctx.At(b, func() { m.recolorChars(c) })
	case "tapTrial/giraffe events":
		typ := int(e.Float("toggle", 0))
		ease := int(e.Float("instant", 0))
		length := e.Length
		ctx.At(b, func() {
			m.giraffeAnim, m.giraffeBeat, m.giraffeLen, m.giraffeEase = typ, b, length, ease
			if typ == 2 {
				m.ctx.Scene.PlayState(m.ctx.Role("giraffe"), "Blink", b, 0.5)
			}
		})
	}
}

func (m *Module) Ready() {}

func (m *Module) cue(b, end float64) {
	m.cues = append(m.cues, struct{ beat, end float64 }{b, end})
	m.ctx.At(b, func() { m.canBop = false })
}

// restoreBop：B+1.5 若 [B+1,B+2) 无后续 cue 则恢复 bop。
func (m *Module) restoreBop(at float64) {
	m.ctx.At(at, func() {
		if !m.cueAt(at-0.5, at+0.5) {
			m.canBop = true
		}
	})
}

func (m *Module) restoreBopFinal(at float64) { m.restoreBop(at) }

func (m *Module) restoreBopNever(at float64) {
	m.ctx.At(at, func() {
		if !m.cueAt(at-0.5, at+0.5) {
			m.canBop = false // 非 final 跳后保持
		}
	})
}

func (m *Module) cueAt(from, to float64) bool {
	for _, c := range m.cues {
		if c.beat >= from && c.beat < to {
			return true
		}
	}
	return false
}

// tapMonkey 音：1.4 倍音调、0.5 音量。
func (m *Module) tapMonkeySound(beat float64) {
	m.ctx.At(beat, func() { m.ctx.SoundPitch("tapMonkey", 0.5, 1.4) })
}

// ---------- 判定 ----------

func (m *Module) tapHit(state float64, anim string) {
	if state < 1 && state > -1 {
		m.ctx.Sound("tap")
		m.playerStars()
	} else {
		m.ctx.PlayCommon("nearMiss")
		m.resetScroll()
	}
	m.playPlayer(anim, m.ctx.Beat())
}

func (m *Module) tripleHit(state float64) {
	left := m.tripleTaps%2 == 0
	m.tripleTaps++
	if state < 1 && state > -1 {
		m.ctx.Sound("tap")
		m.playerStars()
	} else {
		m.ctx.PlayCommon("nearMiss")
		m.resetScroll()
	}
	anim := "PoseTap_R"
	if left {
		anim = "PoseTap_L"
	}
	m.playPlayer(anim, m.ctx.Beat())
}

func (m *Module) jumpHit(state float64, final, alt bool) {
	ace := state < 1 && state > -1
	if ace {
		m.ctx.Sound("tap")
		m.playerStars()
		m.playerStars()
	} else {
		m.ctx.PlayCommon("nearMiss")
		m.resetScroll()
	}
	anim := "JumpTap_Success"
	if final {
		anim = "FinalJump_Tap"
		if alt {
			anim = "FinalJump_TapAlt"
		}
	}
	m.playPlayer(anim, m.ctx.Beat())
}

func (m *Module) jumpMiss(final bool) {
	anim, snd := "JumpTap_Miss", "jumpmiss"
	if final {
		anim, snd = "FinalJump_Miss", "lastjumpmiss"
	}
	m.playPlayer(anim, m.ctx.Beat())
	m.ctx.Sound(snd)
	m.giraffeMiss()
	m.resetScroll()
}

func (m *Module) missCue() {
	m.giraffeMiss()
	m.resetScroll()
}

func (m *Module) giraffeMiss() {
	if m.giraffeAnim != 1 { // Exit 中不打断
		m.ctx.Scene.PlayState(m.ctx.Role("giraffe"), "Miss", m.ctx.Beat(), 0.5)
	}
}

// Whiff：state 机简化——非跳跃期空按按普通拍处理（ScoreMiss 由引擎计 whiff）。
func (m *Module) Whiff(beat float64) {
	if beat-m.jumpBeat >= 0 && beat-m.jumpBeat <= 1 {
		return // Jumping 态不响应
	}
	m.ctx.ScoreMiss()
	m.resetScroll()
	m.playPlayer("Tap", beat)
}

func (m *Module) singleBop(beat float64) {
	if !m.canBop {
		return
	}
	m.playMonkeys("Bop", beat)
	m.playPlayer("Bop", beat)
}

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

func (m *Module) resetScroll() {
	m.scrollSpeed, m.scrollY = 0, 0
}

// ---------- 粒子 ----------

func (m *Module) monkeyStars() { m.burst(-3.4, 0.6); m.burst(3.4, 0.6) }
func (m *Module) playerStars() { m.burst(-0.8, 0.8); m.burst(0.8, 0.8) }

func (m *Module) burst(x, y float64) {
	for i := 0; i < 6; i++ {
		ang := rand.Float64() * 2 * math.Pi
		sp := 2 + rand.Float64()*2.5
		m.stars = append(m.stars, star{x: x, y: y, vx: math.Cos(ang) * sp,
			vy: math.Sin(ang)*sp + 1.5, born: m.lastT})
	}
}

// ---------- 引擎接口 ----------

func (m *Module) OnSwitch(beat float64) {
	m.recolorChars(charDef)
	m.bg = bgEvt{from: [5][4]float64{defA, defB, defC, defStageA, defStageB},
		to: [5][4]float64{defA, defB, defC, defStageA, defStageB}, ease: 1}
	for _, be := range m.bgEvts {
		if be.beat < beat {
			m.bg = be
		}
	}
	m.lastPulse = math.Floor(beat)
	m.canBop = true
}

func (m *Module) Update(t, beat float64) {
	if p := math.Floor(beat); p > m.lastPulse {
		m.lastPulse = p
		if m.autoBopAt(p) {
			m.singleBop(p)
		}
	}
	// 跳跃（玩家/猴 root 抛物线）
	sc := m.ctx.Scene
	norm := beat - m.jumpBeat
	if norm >= 0 && norm <= 1 {
		var py, my float64
		if norm < 0.5 {
			py = engine.Ease(3, 0, jumpHeight, norm/0.5) // EaseOutQuad 上
			my = engine.Ease(3, 0, monkeyJumpHeight, norm/0.5)
		} else {
			py = engine.Ease(2, jumpHeight, 0, (norm-0.5)/0.5) // EaseInQuad 下
			my = engine.Ease(2, monkeyJumpHeight, 0, (norm-0.5)/0.5)
		}
		sc.SetPosOver(m.ctx.Role("rootPlayer"), 0, py)
		sc.SetPosOver(m.ctx.Role("rootMonkeyL"), 0, my)
		sc.SetPosOver(m.ctx.Role("rootMonkeyR"), 0, my)
	} else {
		sc.ClearPosOver(m.ctx.Role("rootPlayer"))
		sc.ClearPosOver(m.ctx.Role("rootMonkeyL"))
		sc.ClearPosOver(m.ctx.Role("rootMonkeyR"))
	}
	// 滚动加速
	if m.hasLastT {
		dt := t - m.lastT
		if m.scrolling {
			m.scrollY += m.scrollSpeed * dt
			m.scrollSpeed = math.Min(maxScrollSpeed, m.scrollSpeed+accelSpeed*dt)
		}
	}
	m.lastT, m.hasLastT = t, true
	// 长颈鹿 Enter/Exit
	if m.giraffeBeat >= 0 && m.giraffeAnim != 2 {
		gn := 0.0
		if m.giraffeLen > 0 {
			gn = (beat - m.giraffeBeat) / m.giraffeLen
		}
		if gn >= 0 && gn <= 1 {
			anim := "Enter"
			if m.giraffeAnim == 1 {
				anim = "Exit"
			}
			sc.PlayNormalized(m.ctx.Role("giraffe"), "Animations/"+anim,
				engine.Ease(m.giraffeEase, 0, 1, gn))
		}
	}
}

func (m *Module) Draw(screen *ebiten.Image, t, beat float64) {
	// 五组色 ColorEase → 背景/舞台调色板
	norm := 1.0
	if m.bg.length > 0 {
		norm = clamp01((beat - m.bg.beat) / m.bg.length)
	}
	var cur [5][4]float64
	for i := 0; i < 5; i++ {
		for j := 0; j < 4; j++ {
			cur[i][j] = engine.Ease(m.bg.ease, m.bg.from[i][j], m.bg.to[i][j], norm)
		}
	}
	sc := m.ctx.Scene
	sc.SetPaletteFor("tapplatform", kart.Palette{Alpha: [4]float64{1, 1, 1, 1}, Fill: cur[3], Outline: cur[4]})

	screen.Fill(toRGBA(cur[0]))
	// 背景方格（backgroundMaterial 三色棋盘 + UV 滚动的等价手绘）
	tile := 1.6 * 54.0
	offY := math.Mod(m.scrollY*m.scrollMult*tile*4, tile*2)
	for row := -1; row < 8; row++ {
		for col := 0; col < 12; col++ {
			cx := float64(col) * tile
			cy := float64(row)*tile + offY
			var cc [4]float64
			switch (row + col) % 2 {
			case 0:
				cc = cur[1]
			default:
				cc = cur[2]
			}
			ebitenFillRect(screen, cx, cy, tile-2, tile-2, toRGBA(cc))
		}
	}
	m.ctx.SampleScene(beat)
	sc.Draw(screen, m.proj)

	// 白闪（scroll event flash）
	if m.flashing && m.scrolling {
		alpha := clamp01(m.scrollY) * maxFlashOpacity
		if alpha > 0 {
			ebitenFillRect(screen, 0, 0, engine.ScreenW, engine.ScreenH,
				colorRGBA{255, 255, 255, uint8(alpha * 255)})
		}
	}
	// 星星粒子
	alive := m.stars[:0]
	for _, s := range m.stars {
		age := t - s.born
		if age > 0.8 {
			continue
		}
		alive = append(alive, s)
		x := s.x + s.vx*age
		y := s.y + s.vy*age - 5*age*age
		px := float64(engine.ScreenW)/2 + x*54
		py := float64(engine.ScreenH)/2 - y*54
		ebitenFillRect(screen, px-2.5, py-2.5, 5, 5, colorRGBA{255, 245, 160, 235})
	}
	m.stars = alive
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
