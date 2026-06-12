// Package sneakyspirits 是 Sneaky Spirits（sneakySpirits）的玩法模块，
// 逻辑对应 Assets/Scripts/Games/SneakySpirits/SneakySpirits.cs：
//
//	spawnGhost(L)：幽灵沿墙 7 格冒头（B+L*i，高度/音量 = volumeN%），
//	    B+L*3 上弦（BowDraw），B+L*7 判定（flick 通道=主键）：
//	    命中 → 随机死法（鼻/嘴/身/颊）+ 开门 1 拍；slowDown 变体附加
//	    0.25 倍速慢动作（本谱面未用，遇到时打日志）；
//	    near → 弓空放 + GhostBarely；miss → 幽灵逃走 + 嘲笑
//	fakeGhost / movebow / forceReload：假幽灵、弓进出场、强制上弦
//
// 雨景 ParticleSystem 为等价手写实现（垂直雨丝 + 底部溅花）。
package sneakyspirits

import (
	"log"
	"math"
	"math/rand"

	"github.com/hajimehoshi/ebiten/v2"

	"hsdemo/engine"
	"hsdemo/kart"
	"hsdemo/riq"
)

// MovingGhost.prefab：子节点 Sprite（scale 2.3）播放 Move/MoveDown，
// 切片 Ghost_move。
const ghostScale = 2.3

type movingGhost struct {
	pos       int     // 0..6
	spawnBeat float64 // 出现拍
	length    float64
	yOff      float64 // -(1-vol%)*2.5
}

type deathGhost struct {
	anim      string
	startBeat float64
}

type missGhost struct {
	beat   float64
	barely bool // GhostBarely（near）：2 拍；GhostMiss+laugh：2.5 拍
}

type bowMove struct {
	beat, length float64
	enter        bool
	ease         int
}

type Module struct {
	ctx  *engine.Ctx
	proj kart.Aff

	ghosts  []movingGhost
	death   *deathGhost
	misses  []missGhost
	arrowT  float64 // ArrowMiss 显示起拍（-1 = 无）
	bowMove *bowMove
	loaded  bool
	slowT   float64 // 慢动作结束拍（slowDown；本谱面未用）

	rain []raindrop
}

type raindrop struct {
	x, y, speed float64
}

func New() engine.Module { return &Module{arrowT: -10, slowT: -10} }

func (m *Module) ID() string { return "sneakySpirits" }

func (m *Module) Load(ctx *engine.Ctx) error {
	m.ctx = ctx
	if err := ctx.LoadAssets("sneakySpirits"); err != nil {
		return err
	}
	m.proj = kart.Translate(engine.ScreenW/2, engine.ScreenH/2).Mul(kart.Scale(54, -54))
	// 雨滴（手写粒子，覆盖屏宽，随机相位）
	for i := 0; i < 70; i++ {
		m.rain = append(m.rain, raindrop{
			x: rand.Float64()*20 - 10, y: rand.Float64() * 12,
			speed: 14 + rand.Float64()*4,
		})
	}
	return nil
}

func (m *Module) positions() []string { return m.ctx.Assets.Extra.RefArrays["ghostPositions"] }

// ---------- 事件 ----------

func (m *Module) OnEvent(e *riq.Entity) {
	b := e.Beat
	ctx := m.ctx
	switch e.Datamodel {
	case "sneakySpirits/spawnGhost", "sneakySpirits/simpleSpawnGhost":
		length := e.Length
		if e.Datamodel == "sneakySpirits/simpleSpawnGhost" {
			length = 1
		}
		if length == 0 {
			length = 1
		}
		slow := boolParam(e, "slowDown")
		if slow {
			log.Printf("sneakySpirits: slowDown 慢动作变体未实现（0.25 倍速音频；官方非 PRACTICE 关未用）")
		}
		vols := make([]float64, 7)
		for i := 0; i < 7; i++ {
			vols[i] = e.Float([]string{"volume1", "volume2", "volume3", "volume4", "volume5", "volume6", "volume7"}[i], 100) / 100
		}
		// PreSpawnGhost：7 声 moving（音量 = volumeN%，偏移 0.019）
		for i := 0; i < 7; i++ {
			ctx.SoundAtOff(b+length*float64(i), "moving", vols[i], 0.019)
		}
		// 幽灵冒头
		for i := 0; i < 7; i++ {
			g := movingGhost{pos: i, spawnBeat: b + length*float64(i), length: length,
				yOff: -(1 - vols[i]) * 2.5}
			m.ghosts = append(m.ghosts, g)
		}
		// 上弦（非触屏：B+L*3）
		ctx.At(b+length*3, func() { m.forceReload() })
		// 判定（flick=主键按下）
		ctx.ScheduleInput(b+length*7,
			func(st float64, _ engine.Judgment) { m.just(st, b+length*7) },
			func() { m.miss(b + length*7) })
	case "sneakySpirits/fakeGhost":
		pos := int(e.Float("position", 1)) - 1
		vol := e.Float("volume", 100) / 100
		length := e.Length
		if length == 0 {
			length = 1
		}
		ctx.SoundAtOff(b, "moving", vol, 0.019)
		m.ghosts = append(m.ghosts, movingGhost{pos: pos, spawnBeat: b, length: length,
			yOff: -(1 - vol) * 2.5})
	case "sneakySpirits/movebow":
		bm := bowMove{b, e.Length, boolParam(e, "exit"), int(e.Float("ease", 0))}
		ctx.At(b, func() { m.bowMove = &bm })
	case "sneakySpirits/forceReload":
		ctx.At(b, func() { m.forceReload() })
	}
}

func (m *Module) Ready() {}

func (m *Module) forceReload() {
	if m.loaded {
		return
	}
	m.ctx.Scene.PlayState(m.ctx.Role("bowAnim"), "BowDraw", m.ctx.Beat(), 0.25)
	m.loaded = true
}

// ---------- 判定 ----------

func (m *Module) just(state float64, beat float64) {
	if !m.loaded {
		return
	}
	ctx := m.ctx
	if state >= 1 || state <= -1 {
		ctx.Sound("ghostScared")
		m.whiffArrow(beat)
		m.misses = append(m.misses, missGhost{beat: beat, barely: true})
		return
	}
	// Success：随机死法
	anims := []string{"GhostDieNose", "GhostDieMouth", "GhostDieBody", "GhostDieCheek"}
	m.death = &deathGhost{anim: anims[rand.Intn(4)], startBeat: beat}
	m.loaded = false
	ctx.Sound("hit")
	ctx.Scene.PlayState(ctx.Role("bowAnim"), "BowRecoil", beat, 0.25)
	ctx.Scene.PlayState(ctx.Role("doorAnim"), "DoorOpen", beat, 0.5)
	ctx.At(beat+1, func() {
		ctx.Scene.PlayState(ctx.Role("doorAnim"), "DoorClose", ctx.Beat(), 0.5)
	})
}

func (m *Module) miss(beat float64) {
	ctx := m.ctx
	ctx.Sound("ghostEscape")
	m.misses = append(m.misses, missGhost{beat: beat, barely: false})
	ctx.At(beat+1, func() {
		if ctx.GameAt(ctx.Beat()) == "sneakySpirits" {
			ctx.Sound("laugh")
		}
	})
}

// whiffArrow：空放（箭飞出 + 弓后坐，3 拍后消失）。
func (m *Module) whiffArrow(beat float64) {
	ctx := m.ctx
	m.arrowT = beat
	ctx.Scene.PlayState(ctx.Role("arrowMissPrefab"), "ArrowRecoil", beat, 0.5)
	ctx.Scene.PlayState(ctx.Role("bowAnim"), "BowRecoil", beat, 0.25)
	m.loaded = false
	ctx.SoundPitch("arrowMiss", 1, 2)
}

// Whiff：flick 空按（有箭时）。
func (m *Module) Whiff(beat float64) {
	if !m.loaded {
		return
	}
	m.whiffArrow(beat)
}

func (m *Module) OnSwitch(beat float64) {
	m.loaded = false
	// InitGhosts：跨段进行中的 spawnGhost 已由 OnEvent 静态展开覆盖
	sc := m.ctx.Scene
	sc.SetActive(m.ctx.Role("deathGhostPrefab"), false)
	sc.SetActive(m.ctx.Role("ghostMissPrefab"), false)
	sc.SetActive(m.ctx.Role("arrowMissPrefab"), false)
}

func (m *Module) Update(t, beat float64) {
	// movebow（DoNormalizedAnimation + ease）
	if bm := m.bowMove; bm != nil {
		norm := 0.0
		if bm.length > 0 {
			norm = (beat - bm.beat) / bm.length
		}
		pos := engine.Ease(bm.ease, 0, 1, clamp01(norm))
		anim := "Exit"
		if bm.enter {
			anim = "Enter"
		}
		m.ctx.Scene.PlayNormalized(m.ctx.Role("bowHolderAnim"), "Animations/"+anim, pos)
		if norm >= 1 {
			m.bowMove = nil
		}
	}
}

// ---------- 绘制 ----------

// 主题色 "5a5a9c"
var bgFill = colorRGBA{90, 90, 156, 255}

func (m *Module) Draw(screen *ebiten.Image, t, beat float64) {
	screen.Fill(bgFill)
	ctx := m.ctx
	sc := ctx.Scene

	// 死亡幽灵（DoNormalizedAnimation 1 拍）
	if d := m.death; d != nil {
		norm := beat - d.startBeat
		if norm > 1 {
			sc.SetActive(ctx.Role("deathGhostPrefab"), false)
			m.death = nil
		} else {
			sc.SetActive(ctx.Role("deathGhostPrefab"), true)
			sc.PlayNormalized(ctx.Role("deathGhostPrefab"), "Animations/"+d.anim, math.Max(norm, 0))
		}
	}
	// 逃走/惊吓幽灵
	keep := m.misses[:0]
	for _, ms := range m.misses {
		life := 2.5
		if ms.barely {
			life = 2.0
		}
		if beat-ms.beat > life {
			continue
		}
		keep = append(keep, ms)
		sc.SetActive(ctx.Role("ghostMissPrefab"), true)
		if ms.barely {
			sc.PlayState(ctx.Role("ghostMissPrefab"), "GhostBarely", ms.beat, 0.5)
		} else if beat-ms.beat >= 1 {
			if cur, _ := sc.StateInfo(ctx.Role("ghostMissPrefab"), beat); cur != "GhostLaugh" {
				sc.PlayState(ctx.Role("ghostMissPrefab"), "GhostLaugh", ms.beat+1, 0.25)
			}
		} else {
			if cur, _ := sc.StateInfo(ctx.Role("ghostMissPrefab"), beat); cur != "GhostMiss" {
				sc.PlayState(ctx.Role("ghostMissPrefab"), "GhostMiss", ms.beat, 0.5)
			}
		}
	}
	if len(keep) == 0 && m.misses != nil {
		sc.SetActive(ctx.Role("ghostMissPrefab"), false)
	}
	m.misses = keep
	// 空放箭（3 拍）
	sc.SetActive(ctx.Role("arrowMissPrefab"), beat-m.arrowT <= 3)

	sc.Sample(beat)

	// 沿墙冒头的幽灵（MovingGhost 模板手绘：Move 0..40%，MoveDown 40%..100%）
	moveClip := ctx.Assets.Anims["Animations/Move"]
	downClip := ctx.Assets.Anims["Animations/MoveDown"]
	for _, g := range m.ghosts {
		dt := beat - g.spawnBeat
		if dt < 0 || dt >= g.length {
			continue
		}
		base, ok := sc.NodeWorld(m.positions()[g.pos])
		if !ok {
			continue
		}
		// timeScale = 1/L：clipT(秒) = 拍 × (1/L)
		var sy float64
		if dt < g.length*0.4 {
			sy = kart.SampleClipNode(moveClip, "Sprite", dt/g.length).Pos[1]
		} else {
			sy = kart.SampleClipNode(downClip, "Sprite", (dt-g.length*0.4)/g.length).Pos[1]
		}
		world := base.Mul(kart.Translate(0, g.yOff+sy).Mul(kart.Scale(ghostScale, ghostScale)))
		sc.Queue(kart.ExtraSprite{Sprite: "Ghost_move", World: world, Order: 5})
	}
	sc.Draw(screen, m.proj)

	m.drawRain(screen, t)
}

// drawRain：雨景 ParticleSystem 的等价手写（垂直雨丝）。
func (m *Module) drawRain(screen *ebiten.Image, t float64) {
	for i := range m.rain {
		r := &m.rain[i]
		y := math.Mod(r.y+t*r.speed, 12)
		px := float64(engine.ScreenW)/2 + r.x*54
		py := -54 + (y * 54)
		ebitenFillRect(screen, px, py, 1.5, 18, colorRGBA{200, 205, 235, 110})
	}
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
