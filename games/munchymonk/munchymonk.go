// Package munchymonk 是 Munchy Monk（munchyMonk）的玩法模块，
// 逻辑对应 Assets/Scripts/Games/MunchyMonk/{MunchyMonk,Dumpling}.cs：
//
//	One：B 给 1 个饺子（判定 B+1）；TwoTwo：B-0.5/B 给 2 个（B+1、B+1.5）；
//	Three：B/B+1.3/B+2.3 给 3 个（B+1、B+2、B+3）。命中吞下（Eat ts 0.4 +
//	smear + gulp，计数长胡子），barely 砸头，miss 滚落，提前打 HitHead+Miss。
//	胡子等级（inputsTilGrow/Modifiers）、Stare/Blush、MonkMove 归一化移动、
//	ScrollBackground 滚动坡道（17 个 ScrollObject）、CloudMonkey 横穿。
package munchymonk

import (
	"image/color"
	"log"
	"math"
	"sort"

	"github.com/hajimehoshi/ebiten/v2"

	"hsdemo/engine"
	"hsdemo/kart"
	"hsdemo/riq"
)

// 主题色 "b9fffc"
var bgColor = color.RGBA{0xb9, 0xff, 0xfc, 255}

type dumpling struct {
	inst     *kart.Instance
	color    [4]float64
	spawn    float64
	dead     bool
	fallable bool // canDestroy（动画毕销毁）
	deadAt   float64
}

type bopEvt struct {
	beat, length float64
	bop, auto    bool
}

type scrollObj struct {
	path           string
	xs, ys         float64
	negX, posX     float64
	auto           bool
	offX           float64
}

type Module struct {
	ctx  *engine.Ctx
	proj kart.Aff

	dumpT     *kart.Template
	dumplings []*dumpling // 存活栈（输入取栈顶）
	bops      []bopEvt
	endBeat   float64

	needBlush bool
	isStaring bool
	noBlush   bool

	gulps        int
	growLevel    int
	inputsTil    int
	disableBaby  bool

	// MonkMove / ScrollBG
	moveBeat, moveLen float64
	moveAnim          string
	moveEase          int
	moving            bool
	scrollBeat        float64
	scrollLen         float64
	scrollFrom, scrollTo float64
	scrollEase        int
	scrollRamp        bool
	scrollCur         float64

	scrolls []scrollObj

	// CloudMonkey
	monkeyOn   bool
	monkeySpd  float64
	monkeyX    float64
	lastT      float64
	hasLastT   bool
}

func New() engine.Module { return &Module{inputsTil: 10} }

func (m *Module) ID() string { return "munchyMonk" }

func (m *Module) Load(ctx *engine.Ctx) error {
	m.ctx = ctx
	if err := ctx.LoadAssets("munchyMonk"); err != nil {
		return err
	}
	m.proj = kart.Translate(engine.ScreenW/2, engine.ScreenH/2).Mul(kart.Scale(54, -54))
	m.dumpT = kart.NewTemplate(ctx.Assets, ctx.Role("DumplingObj"))
	for name, c := range ctx.Assets.Extra.Components {
		if len(name) > 6 && name[:6] == "scroll" {
			m.scrolls = append(m.scrolls, scrollObj{
				path: c.Path,
				xs:   c.Nums["XSpeed"], ys: c.Nums["YSpeed"],
				negX: c.Nums["NegativeBounds.x"], posX: c.Nums["PositiveBounds.x"],
				auto: c.Nums["AutoScroll"] != 0,
			})
		}
	}
	sort.Slice(m.scrolls, func(i, j int) bool { return m.scrolls[i].path < m.scrolls[j].path })
	return nil
}

// ---------- 事件 ----------

func (m *Module) OnEvent(e *riq.Entity) {
	b := e.Beat
	if end := b + e.Length; end > m.endBeat {
		m.endBeat = end
	}
	ctx := m.ctx
	switch e.Datamodel {
	case "munchyMonk/Bop":
		bp := bopEvt{b, e.Length, boolParam(e, "bop"), boolParam(e, "autoBop")}
		m.bops = append(m.bops, bp)
		if bp.bop {
			for i := 0.0; i < bp.length; i++ {
				bb := b + i
				ctx.At(bb, func() {
					m.needBlush = false
					m.monkBop(bb)
				})
			}
		}
	case "munchyMonk/One":
		colorOne := [4]float64{1, 1, 1, 1}
		if boolParam(e, "uniqueColor") {
			colorOne = colorParam(e, "oneColor")
		}
		ctx.PlaySeq("one_go", b)
		ctx.At(b, func() {
			m.giver(1, "GiveIn", b)
			m.spawnDumpling(b, 0, colorOne)
		})
		ctx.At(b+0.5, func() { m.giver(1, "GiveOut", b+0.5) })
		m.scheduleDumplingInput(b + 1)
	case "munchyMonk/TwoTwo":
		c1 := [4]float64{1, 0.51, 0.45, 1}
		c2 := c1
		if boolParam(e, "uniqueColor") {
			c1, c2 = colorParam(e, "twoColor1"), colorParam(e, "twoColor2")
		}
		ctx.PlaySeq("two_go", b)
		ctx.At(b-0.5, func() {
			m.giver(2, "GiveIn", b-0.5)
			m.spawnDumpling(b-0.5, 1, c1)
		})
		ctx.At(b, func() {
			m.giver(2, "GiveOut", b)
			m.spawnDumpling(b-0.5, 2, c2)
		})
		m.scheduleDumplingInput(b + 1)
		m.scheduleDumplingInput(b + 1.5)
	case "munchyMonk/Three":
		c1 := [4]float64{0.34, 0.77, 0.36, 1}
		c2, c3 := c1, c1
		if boolParam(e, "uniqueColor") {
			c1 = colorParam(e, "threeColor1")
			c2 = colorParam(e, "threeColor2")
			c3 = colorParam(e, "threeColor3")
		}
		ctx.PlaySeq("three_go", b)
		ctx.At(b, func() {
			m.giver(3, "GiveIn", b)
			m.spawnDumpling(b, 3, c1)
		})
		ctx.At(b+0.5, func() { m.giver(3, "GiveOut", b+0.5) })
		ctx.At(b+1.3, func() {
			m.giver(3, "GiveIn", b+1.3)
			m.spawnDumpling(b+1.3, 4, c2)
		})
		ctx.At(b+1.75, func() { m.giver(3, "GiveOut", b+1.75) })
		ctx.At(b+2.3, func() {
			m.giver(3, "GiveIn", b+2.3)
			m.spawnDumpling(b+2.3, 5, c3)
		})
		ctx.At(b+2.75, func() { m.giver(3, "GiveOut", b+2.75) })
		m.scheduleDumplingInput(b + 1)
		m.scheduleDumplingInput(b + 2)
		m.scheduleDumplingInput(b + 3)
	case "munchyMonk/defaultColors":
		// 默认色已在事件分支取色处覆盖；官方关卡未用 unique 默认变更
	case "munchyMonk/Modifiers":
		inputsTil := int(e.Float("inputsTil", 10))
		reset := boolParam(e, "resetLevel")
		setLevel := int(e.Float("setLevel", 0))
		noBaby := boolParam(e, "disableBaby")
		blush := true
		if _, has := e.Data["shouldBlush"]; has {
			blush = boolParam(e, "shouldBlush")
		}
		ctx.At(b, func() {
			if m.inputsTil != inputsTil && m.inputsTil > 0 {
				m.gulps = inputsTil*m.growLevel + inputsTil*(m.gulps%m.inputsTil)/m.inputsTil
				m.inputsTil = inputsTil
			}
			if setLevel != 0 {
				m.growLevel = setLevel
				m.gulps = setLevel * inputsTil
				m.applyStache(true)
			}
			if reset {
				m.growLevel, m.gulps = 0, 0
				ctx.Scene.SetActive(ctx.Role("StacheHolder"), false)
				ctx.Scene.SetActive(ctx.Role("BrowHolder"), false)
			}
			m.noBlush = !blush
			m.disableBaby = noBaby
			ctx.Scene.SetActive(ctx.Role("Baby"), !noBaby)
		})
	case "munchyMonk/MonkAnimation":
		which := int(e.Float("whichAnim", 0))
		if boolParam(e, "vineBoom") {
			log.Printf("munchyMonk: vineBoom 音效（fanClub/arisa_dab）未提取——官方关卡未使用")
		}
		ctx.At(b, func() {
			if which == 1 {
				m.monkState("Blush", b)
				m.needBlush = false
			} else {
				m.monkState("Stare", b)
				m.isStaring = true
			}
		})
	case "munchyMonk/MonkMove":
		side := int(e.Float("goToSide", 0))
		ease := int(e.Float("ease", 0))
		length := e.Length
		ctx.At(b, func() {
			m.moveBeat, m.moveLen, m.moveEase, m.moving = b, length, ease, true
			m.moveAnim = "GoRight"
			if side != 0 {
				m.moveAnim = "GoLeft"
			}
		})
	case "munchyMonk/ScrollBackground":
		speed := e.Float("scrollSpeed", 5)
		ease := int(e.Float("ease", 0))
		length := e.Length
		ctx.At(b, func() {
			m.scrollBeat, m.scrollLen = b, length
			m.scrollFrom, m.scrollTo = m.scrollCur, speed
			m.scrollEase, m.scrollRamp = ease, true
		})
	case "munchyMonk/CloudMonkey":
		start := boolParam(e, "start")
		dir := int(e.Float("direction", 0))
		length := e.Length
		ctx.At(b, func() {
			wasOn := m.monkeyOn
			m.monkeyOn = start
			spd := 34.0
			if dir != 0 {
				spd = -34
			}
			m.monkeySpd = spd / length * (60 / m.ctx.SecPerBeat(b) / 100)
			if !wasOn {
				m.monkeyX = -5
				if dir != 0 {
					m.monkeyX = 15.5
				}
			}
			m.ctx.Scene.SetActive(m.ctx.Role("CloudMonkey"), true)
		})
	}
}

// Ready：bop 区间（SetupBopRegion autoBop）逐拍脉冲。
func (m *Module) Ready() {
	sort.Slice(m.bops, func(i, j int) bool { return m.bops[i].beat < m.bops[j].beat })
	for b := 0.0; b <= m.endBeat; b++ {
		bb := b
		m.ctx.At(bb, func() { m.beatPulse(bb) })
	}
}

func (m *Module) inBopRegion(beat float64) bool {
	on := false
	for _, bp := range m.bops {
		if bp.beat > beat {
			break
		}
		on = bp.auto
	}
	return on
}

func (m *Module) beatPulse(beat float64) {
	st, playing := m.ctx.Scene.StateInfo(m.ctx.Role("MonkAnim"), beat)
	if (!playing || st == "Bop" || st == "Idle") && m.inBopRegion(beat) && !m.isStaring {
		m.ctx.Scene.PlayState(m.ctx.Role("MonkAnim"), "Bop", beat, 0.5)
	}
	if !(playing && (st == "Blush" || st == "Stare")) {
		if m.growLevel == 4 {
			m.ctx.Scene.PlayState(m.ctx.Role("BrowAnim"), "Bop", beat, 0.5)
		}
		if m.growLevel > 0 {
			m.ctx.Scene.PlayState(m.ctx.Role("StacheAnim"), stacheAnim("Bop", m.growLevel), beat, 0.5)
		}
	}
	if m.monkeyOn {
		m.ctx.Scene.PlayState(m.ctx.Role("CloudMonkey"), "Bop", beat, 0.5)
	}
}

func stacheAnim(prefix string, level int) string {
	return prefix + string(rune('0'+level))
}

func (m *Module) giver(which int, anim string, beat float64) {
	role := map[int]string{1: "OneGiverAnim", 2: "TwoGiverAnim", 3: "ThreeGiverAnim"}[which]
	m.ctx.Scene.PlayState(m.ctx.Role(role), anim, beat, 0.5)
}

func (m *Module) monkState(name string, beat float64) {
	m.ctx.Scene.PlayState(m.ctx.Role("MonkAnim"), name, beat, 0.5)
}

func (m *Module) monkBop(beat float64) {
	m.monkState("Bop", beat)
	if m.growLevel == 4 {
		m.ctx.Scene.PlayState(m.ctx.Role("BrowAnim"), "Bop", beat, 0.5)
	}
	if m.growLevel > 0 {
		m.ctx.Scene.PlayState(m.ctx.Role("StacheAnim"), stacheAnim("Bop", m.growLevel), beat, 0.5)
	}
}

func (m *Module) applyStache(playIdle bool) {
	sc := m.ctx.Scene
	if m.growLevel > 0 {
		sc.SetActive(m.ctx.Role("StacheHolder"), true)
		if playIdle {
			sc.PlayFrozen(m.ctx.Role("StacheAnim"), stacheAnim("Idle", m.growLevel), 0)
		}
	}
	if m.growLevel == 4 {
		sc.SetActive(m.ctx.Role("BrowHolder"), true)
		sc.PlayFrozen(m.ctx.Role("BrowAnim"), "Idle", 0)
	}
}

// ---------- 饺子 ----------

func (m *Module) spawnDumpling(beat float64, spriteIdx int, col [4]float64) {
	d := &dumpling{inst: m.dumpT.NewInstance(), color: col, spawn: beat}
	sprites := m.ctx.Assets.Extra.Components["game"].SpriteArrays["dumplingSprites"]
	if spriteIdx < len(sprites) {
		d.inst.SetSprite("", sprites[spriteIdx])
	}
	d.inst.SetColor("", col)
	if len(m.alive()) >= 1 {
		d.inst.PlayFrozen("", "IdleOnTop", 0)
		if first := m.alive(); len(first) > 0 {
			first[0].inst.PlayState("", "Squish", m.ctx.Beat(), 0.5)
		}
	}
	m.dumplings = append(m.dumplings, d)
}

func (m *Module) alive() []*dumpling {
	var out []*dumpling
	for _, d := range m.dumplings {
		if !d.dead {
			out = append(out, d)
		}
	}
	return out
}

func (m *Module) scheduleDumplingInput(judgeBeat float64) {
	ctx := m.ctx
	ctx.ScheduleInput(judgeBeat, func(state float64, _ engine.Judgment) {
		ds := m.alive()
		if len(ds) == 0 {
			return
		}
		d := ds[len(ds)-1]
		beat := ctx.Beat()
		// HitFunction
		ctx.Scene.SetColorOver("DumplingStuff/DumplingSmear1", d.color)
		ctx.Scene.PlayState(m.ctx.Role("MonkArmsAnim"), "WristSlap", beat, 0.5)
		ctx.Sound("slap")
		m.isStaring = false
		if state >= 1 || state <= -1 {
			m.monkState("Barely", beat)
			d.inst.PlayState("", "HitHead", beat, 0.5)
			ctx.Sound("barely")
			d.fallable = true
			d.deadAt = beat + 2
			m.needBlush = false
			return
		}
		m.ctx.Scene.PlayState(m.ctx.Role("MonkAnim"), "Eat", beat, 0.4)
		ds[0].inst.PlayState("", "FollowHand", beat, 0.5)
		ctx.Scene.PlayFrozen("DumplingStuff/DumplingSmear1", "SmearAppear", 0)
		m.needBlush = true
		ctx.Sound("gulp")
		m.gulps++
		for i := 1; i <= 4; i++ {
			if m.gulps == m.inputsTil*i {
				m.growLevel = i
				m.applyStache(true)
			}
		}
		d.dead = true
	}, func() {
		ds := m.alive()
		if len(ds) == 0 {
			return
		}
		d := ds[len(ds)-1]
		d.inst.PlayState("", "FallOff", m.ctx.Beat(), 0.5)
		d.fallable = true
		d.deadAt = m.ctx.Beat() + 2
	})
}

// ---------- 生命周期 ----------

func (m *Module) OnSwitch(beat float64) {
	sc := m.ctx.Scene
	sec := m.ctx.SecPerBeat(beat)
	for _, role := range []string{"MonkAnim", "MonkArmsAnim", "MonkHolderAnim",
		"OneGiverAnim", "TwoGiverAnim", "ThreeGiverAnim", "CloudMonkey"} {
		sc.PlayDefaultState(m.ctx.Role(role), beat, sec)
	}
	sc.SetActive(m.ctx.Role("CloudMonkey"), false)
	sc.SetActive(m.ctx.Role("DumplingObj"), false) // 模板自绘
	sc.SetActive("DumplingStuff/DumplingSmear1", true)
	sc.SetActive(m.ctx.Role("Baby"), !m.disableBaby)
	m.applyStache(true)
}

func (m *Module) Whiff(beat float64) {
	ctx := m.ctx
	ctx.Scene.PlayState(m.ctx.Role("MonkArmsAnim"), "WristSlap", beat, 0.5)
	ctx.Sound("slap")
	m.isStaring = false
	// 有饺子在场时为"提前打"（EarlyFunction）
	ds := m.alive()
	if len(ds) == 0 {
		return
	}
	d := ds[len(ds)-1]
	m.monkState("Miss", beat)
	ctx.Scene.PlayFrozen("DumplingStuff/DumplingSmear1", "SmearAppear", 0)
	d.inst.PlayState("", "HitHead", beat, 0.5)
	ctx.Sound("miss")
	d.fallable = true
	d.deadAt = beat + 2
	m.needBlush = false
	d.dead = true
}

func (m *Module) Update(t, beat float64) {
	dt := 0.0
	if m.hasLastT && t > m.lastT {
		dt = t - m.lastT
	}
	m.lastT, m.hasLastT = t, true
	sc := m.ctx.Scene

	// LateUpdate：blush
	st, playing := sc.StateInfo(m.ctx.Role("MonkAnim"), beat)
	inEat := playing && (st == "Eat" || st == "Stare" || st == "Barely" || st == "Miss")
	if m.needBlush && !inEat && !m.isStaring && !m.noBlush {
		m.monkState("Blush", beat)
		m.needBlush = false
	}

	// MonkMove
	if m.moving {
		norm := clamp01((beat - m.moveBeat) / m.moveLen)
		sc.PlayNormalized(m.ctx.Role("MonkHolderAnim"), m.moveAnim, engine.Ease(m.moveEase, 0, 1, norm))
		if norm >= 1 {
			m.moving = false
		}
	}

	// ScrollBG 坡道
	if m.scrollRamp {
		norm := clamp01((beat - m.scrollBeat) / m.scrollLen)
		m.scrollCur = engine.Ease(m.scrollEase, m.scrollFrom, m.scrollTo, norm)
		if norm >= 1 {
			m.scrollRamp = false
			m.scrollCur = m.scrollTo
		}
	}
	for i := range m.scrolls {
		s := &m.scrolls[i]
		if !s.auto || s.path == "CloudMonkey" {
			continue
		}
		ni, ok := sc.Index(s.path)
		if !ok {
			continue
		}
		n := m.ctx.Assets.Rig.Nodes[ni]
		s.offX += dt * m.scrollCur * s.xs
		x := n.Pos[0] + s.offX
		span := s.posX - s.negX
		if span > 0 {
			for s.xs > 0 && x >= s.posX {
				s.offX -= span
				x -= span
			}
			for s.xs < 0 && x <= s.negX {
				s.offX += span
				x += span
			}
		}
		sc.SetPosOver(s.path, x, n.Pos[1])
	}

	// CloudMonkey
	if m.monkeyOn {
		m.monkeyX += m.monkeySpd * dt
		sc.SetPosOver(m.ctx.Role("CloudMonkey"), m.monkeyX, 0)
		if m.monkeyX < -5 || m.monkeyX > 15.5 {
			m.monkeyOn = false
			sc.SetActive(m.ctx.Role("CloudMonkey"), false)
		}
	}

	// 饺子清理（barely/fall 动画播毕）
	keep := m.dumplings[:0]
	for _, d := range m.dumplings {
		if d.dead && !d.fallable {
			continue
		}
		if d.fallable && beat >= d.deadAt {
			continue
		}
		keep = append(keep, d)
	}
	m.dumplings = keep
}

func (m *Module) Draw(screen *ebiten.Image, t, beat float64) {
	screen.Fill(bgColor)
	sc := m.ctx.Scene
	sc.Sample(beat)
	holder, _ := sc.NodeWorld("DumplingStuff/DumplingHolder")
	for _, d := range m.dumplings {
		if d.dead && !d.fallable {
			continue
		}
		d.inst.Queue(sc, beat, holder, 0)
	}
	sc.Draw(screen, m.proj)
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

var _ = math.Inf

func boolParam(e *riq.Entity, key string) bool {
	if v, ok := e.Data[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return false
}

func colorParam(e *riq.Entity, key string) [4]float64 {
	out := [4]float64{1, 1, 1, 1}
	if mm, ok := e.Data[key].(map[string]any); ok {
		get := func(k string) float64 {
			if f, ok := mm[k].(float64); ok {
				return f
			}
			return 0
		}
		out = [4]float64{get("r"), get("g"), get("b"), get("a")}
	}
	return out
}
