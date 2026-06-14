// Package somen 是 Rhythm Sōmen（rhythmSomen）的玩法模块，
// 逻辑逐条对应 Assets/Scripts/Games/RhythmSomen/RhythmSomen.cs。
// 判定与计分由 engine 完成，本模块只负责事件调度与表现反应。
package somen

import (
	"image/color"
	"math"
	"math/rand"
	"sort"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"hsdemo/engine"
	"hsdemo/kart"
	"hsdemo/riq"
)

var bgColor = color.RGBA{0x7a, 0xb9, 0x6e, 255} // Minigame 注册的主题色

type bopEvt struct {
	beat, length float64
	manual, auto bool
}

type Module struct {
	ctx        *engine.Ctx
	proj       kart.Aff
	missed     bool
	hasSlurped bool
	bops       []bopEvt
	endBeat    float64
	splashes   []splash
}

// splash 是接住面条时的水花粒子（对应 prefab 里的 SplashEffect ParticleSystem：
// 2 颗水珠、初速 5、重力系数 2、生存 0.5s、尺寸 0.2）。
type splash struct {
	pos  [2]float64
	vel  [2]float64
	born float64
}

const (
	splashLife    = 0.5
	splashSpeed   = 5.0
	splashGravity = 2 * 9.81
	splashSize    = 0.2
)

// spawnSplash 在 SplashEffect 节点位置喷出一组水珠。
func (m *Module) spawnSplash() {
	world, ok := m.ctx.Scene.NodeWorld("SplashEffect")
	if !ok {
		return
	}
	x, y := world.Tx, world.Ty
	t := m.ctx.Time()
	for i := 0; i < 2; i++ {
		ang := math.Pi/2 + (rand.Float64()-0.5)*math.Pi/3 // 竖直向上 ±30°
		m.splashes = append(m.splashes, splash{
			pos:  [2]float64{x, y},
			vel:  [2]float64{math.Cos(ang) * splashSpeed, math.Sin(ang) * splashSpeed},
			born: t,
		})
	}
}

func (m *Module) drawSplashes(screen *ebiten.Image, t float64) {
	alive := m.splashes[:0]
	for _, s := range m.splashes {
		age := t - s.born
		if age > splashLife {
			continue
		}
		alive = append(alive, s)
		x := s.pos[0] + s.vel[0]*age
		y := s.pos[1] + s.vel[1]*age - 0.5*splashGravity*age*age
		sx, sy := m.proj.Apply(x, y)
		a := 1 - age/splashLife
		r := float32(splashSize / 2 * 54)
		vector.DrawFilledCircle(screen, float32(sx), float32(sy), r,
			color.RGBA{0xcf, 0xe8, 0xff, uint8(220 * a)}, true)
	}
	m.splashes = alive
}

func New() engine.Module { return &Module{} }

func (m *Module) ID() string { return "rhythmSomen" }

func (m *Module) Load(ctx *engine.Ctx) error {
	m.ctx = ctx
	if err := ctx.LoadAssets("rhythmSomen"); err != nil {
		return err
	}
	// 相机：正交半高 5（与 GameCamera 在 z=0 平面的视野等价）→ 54 px/unit
	m.proj = kart.Translate(engine.ScreenW/2, engine.ScreenH/2).Mul(kart.Scale(54, -54))
	return nil
}

// OnSwitch 启动各 Animator 的空闲剪辑（各 controller 的默认状态）。
func (m *Module) OnSwitch(beat float64) {
	m.missed, m.hasSlurped = false, false
	for path, clip := range map[string]string{
		m.ctx.Role("SomenPlayer"):  "NothingMan",
		m.ctx.Role("FrontArm"):     "ArmNothing",
		m.ctx.Role("backArm"):      "BackArmNothing",
		m.ctx.Role("EffectHit"):    "HitNothing",
		m.ctx.Role("EffectSweat"):  "BlobNothing",
		m.ctx.Role("EffectExclam"): "ExclamNothing",
		m.ctx.Role("EffectShock"):  "ShockNothing",
		m.ctx.Role("CloseCrane"):   "Nothing",
		m.ctx.Role("FarCrane"):     "Nothing",
		"Shoot":                    "WaterFlow", // 流水循环（Animator 在 Shoot，曲线 path = Flow）
	} {
		m.ctx.Play(path, clip, beat, 0.5)
	}
}

func (m *Module) OnEvent(e *riq.Entity) {
	ctx := m.ctx
	b := e.Beat
	if end := b + e.Length; end > m.endBeat {
		m.endBeat = end
	}
	switch e.Datamodel {
	case "rhythmSomen/crane (far)":
		m.craneSeq(b, ctx.Role("FarCrane"), "Drop", "Open", "Lift", "somen_lowerfar", false)
		ctx.ScheduleInput(b+3, m.catchHit, m.catchMiss)
	case "rhythmSomen/crane (close)":
		m.craneSeq(b, ctx.Role("CloseCrane"), "DropClose", "OpenClose", "LiftClose", "somen_lowerclose", false)
		ctx.ScheduleInput(b+2, m.catchHit, m.catchMiss)
	case "rhythmSomen/crane (both)":
		m.craneSeq(b, ctx.Role("FarCrane"), "Drop", "Open", "Lift", "somen_lowerfar", true)
		m.craneSeq(b, ctx.Role("CloseCrane"), "DropClose", "OpenClose", "LiftClose", "", false)
		ctx.ScheduleInput(b+2, m.catchHit, m.catchMiss)
		ctx.ScheduleInput(b+3, m.catchHit, m.catchMiss)
	case "rhythmSomen/offbeat bell":
		ctx.At(b, func() {
			ctx.Sound("somen_bell")
			ctx.Play(ctx.Role("EffectExclam"), "ExclamAppear", b, 0.5)
		})
	case "rhythmSomen/slurp":
		ctx.At(b, func() {
			if !m.missed {
				ctx.Play(ctx.Role("backArm"), "BackArmLift", b, 0.5)
				ctx.Play(ctx.Role("FrontArm"), "ArmSlurp", b, 0.5)
				m.hasSlurped = true
			}
		})
		ctx.At(b+1, func() {
			if m.hasSlurped {
				ctx.Play(ctx.Role("backArm"), "BackArmNothing", b+1, 0.5)
				ctx.Play(ctx.Role("FrontArm"), "ArmNothing", b+1, 0.5)
			}
		})
	case "rhythmSomen/bop":
		m.bops = append(m.bops, bopEvt{b, e.Length, boolParam(e, "toggle2"), boolParam(e, "toggle")})
	}
}

// Ready 计算 bop 拍集合（手动逐拍 + 自动区间至下一 bop 事件）。
func (m *Module) Ready() {
	sort.Slice(m.bops, func(i, j int) bool { return m.bops[i].beat < m.bops[j].beat })
	bopBeats := map[float64]bool{}
	for i, bp := range m.bops {
		if bp.manual {
			for k := 0.0; k < bp.length; k++ {
				bopBeats[bp.beat+k] = true
			}
		}
		if bp.auto {
			until := m.endBeat
			if i+1 < len(m.bops) {
				until = m.bops[i+1].beat
			}
			for k := math.Ceil(bp.beat); k < until; k++ {
				bopBeats[k] = true
			}
		}
	}
	man := m.ctx.Role("SomenPlayer")
	for b := range bopBeats {
		b := b
		m.ctx.At(b, func() { m.ctx.Play(man, "HeadBob", b, 0.5) })
	}
}

// craneSeq 调度一组吊臂动作：B 放下，B+1 张爪，B+1.5 收起。
func (m *Module) craneSeq(b float64, crane, drop, open, lift, lowerSfx string, doubleAlarm bool) {
	ctx := m.ctx
	withSound := lowerSfx != "" || doubleAlarm
	ctx.At(b, func() {
		if lowerSfx != "" {
			ctx.Sound(lowerSfx)
		}
		if doubleAlarm {
			ctx.Sound("somen_doublealarm")
		}
		ctx.Play(crane, drop, b, 0.5)
	})
	ctx.At(b+1, func() {
		if withSound {
			ctx.Sound("somen_drop")
		}
		ctx.Play(crane, open, b+1, 0.5)
	})
	ctx.At(b+1.5, func() {
		if withSound {
			ctx.Sound("somen_woosh")
		}
		ctx.Play(crane, lift, b+1.5, 0.5)
	})
}

// catchHit 对应 C# CatchSuccess：|state|>=1（NG 命中）走 NG 分支。
func (m *Module) catchHit(state float64, _ engine.Judgment) {
	ctx := m.ctx
	beat := ctx.Beat()
	ctx.Play(ctx.Role("backArm"), "BackArmNothing", beat, 0) // timeScale 0：定格
	m.hasSlurped = false
	m.spawnSplash()

	if state >= 1 || state <= -1 {
		ctx.Sound("somen_splash")
		ctx.Play(ctx.Role("FrontArm"), "ArmPluckNG", beat, 0.5)
		ctx.Play(ctx.Role("EffectSweat"), "BlobSweating", beat, 0.5)
		m.missed = true
		return
	}
	ctx.Sound("somen_catch")
	ctx.SoundVol("somen_catch_old", 0.25)
	ctx.Play(ctx.Role("FrontArm"), "ArmPluckOK", beat, 0.5)
	ctx.Play(ctx.Role("EffectHit"), "HitAppear", beat, 0.5)
	m.missed = false
}

func (m *Module) catchMiss() {
	m.missed = true
	m.ctx.Play(m.ctx.Role("EffectShock"), "ShockAppear", m.ctx.Beat(), 0.5)
}

// Whiff 对应 C# Update 的非预期按键分支。
func (m *Module) Whiff(beat float64) {
	ctx := m.ctx
	m.hasSlurped = false
	ctx.Sound("somen_mistake")
	ctx.Play(ctx.Role("FrontArm"), "ArmPluck", beat, 0.5)
	ctx.Play(ctx.Role("backArm"), "BackArmNothing", beat, 0.5)
	ctx.Play(ctx.Role("EffectSweat"), "BlobSweating", beat, 0.5)
}

func (m *Module) Update(t, beat float64) {}

func (m *Module) Draw(screen *ebiten.Image, t, beat float64) {
	screen.Fill(bgColor)
	m.ctx.SampleScene(beat)
	m.ctx.Scene.Draw(screen, m.proj)
	m.drawSplashes(screen, t)
}

func boolParam(e *riq.Entity, key string) bool {
	if v, ok := e.Data[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return false
}
