// Package marchingorders 是 Marching Orders（marchingOrders）的玩法模块，
// 逻辑对应 Assets/Scripts/Games/MarchingOrders/MarchingOrders.cs：
//
//	attention：B-1 起 zentai 号令（音效序列），Sarge B+0.25 说话
//	march：B 起 susume 号令；B+1 全员原地踏步（MarchL）；此后每拍
//	       其余 cadet 交替踏步 + 玩家输入（主键），直到 halt
//	faceTurn：左/右转头（方向键输入，B+3 判定；fast 变体 B+2）
//	halt：tomare 号令，B+1 立定（替代键输入）
//	background：三组映射材质调色板（tiles/pipes/conveyor）+ 墙/地底色
//	go：传送带滚动（官方关卡未使用，按 ScrollObject 语义实现）
package marchingorders

import (
	"math"
	"sort"

	"github.com/hajimehoshi/ebiten/v2"

	"hsdemo/engine"
	"hsdemo/kart"
	"hsdemo/riq"
)

// 主题色 "ffb108"
var bgFill = [4]float64{1, 0.69, 0.03, 1}

type marchEvt struct {
	beat       float64
	noVoice    bool
	march      bool
}

type bopEvt struct {
	beat, length    float64
	bop, auto, clap bool
}

type bgEvt struct {
	beat   float64
	preset int
	colors [9][4]float64 // fill,t1,t2,t3,p1,p2,p3,c1,c2
}

type Module struct {
	ctx  *engine.Ctx
	proj kart.Aff

	marches []marchEvt
	halts   []float64
	bops    []bopEvt
	bgs     []bgEvt

	goBop      bool
	clap       bool
	otherCount int
	playerStep int
	lastMiss   float64

	conveyorSpeed float64 // go 事件（未用时 0）
	conveyorOn    bool
	conveyorX     float64
	lastT         float64
	hasLastT      bool
	endBeat       float64
}

func New() engine.Module { return &Module{lastMiss: -10} }

func (m *Module) ID() string { return "marchingOrders" }

func (m *Module) Load(ctx *engine.Ctx) error {
	m.ctx = ctx
	if err := ctx.LoadAssets("marchingOrders"); err != nil {
		return err
	}
	m.proj = kart.Translate(engine.ScreenW/2, engine.ScreenH/2).Mul(kart.Scale(54, -54))
	return nil
}

// 全部 cadet（其余 3 个）与玩家。
func (m *Module) others() []string {
	return m.ctx.Assets.Extra.RefArrays["Cadets"]
}

func (m *Module) othersHeads() []string {
	return m.ctx.Assets.Extra.RefArrays["CadetHeads"]
}

func (m *Module) player() string     { return m.ctx.Role("CadetPlayer") }
func (m *Module) playerHead() string { return m.ctx.Role("CadetHeadPlayer") }

// ---------- 事件 ----------

func (m *Module) OnEvent(e *riq.Entity) {
	b := e.Beat
	if end := b + e.Length; end > m.endBeat {
		m.endBeat = end
	}
	ctx := m.ctx
	switch e.Datamodel {
	case "marchingOrders/bop":
		bp := bopEvt{b, e.Length, boolParam(e, "bop"), boolParam(e, "autoBop"), boolParam(e, "clap")}
		m.bops = append(m.bops, bp)
		ctx.At(b, func() { m.goBop, m.clap = bp.auto, bp.clap })
		if bp.bop {
			for i := 0.0; i < bp.length; i++ {
				bb := b + i
				ctx.At(bb, func() { m.allBop(bb) })
			}
		}
	case "marchingOrders/attention":
		ctx.PlaySeq("zentai", b-1) // AttentionSound(beat)：序列内部偏移 0.75/1/1.5
		ctx.At(b+0.25, func() { m.sargeTalk(b + 0.25) })
	case "marchingOrders/march":
		me := marchEvt{b, boolParam(e, "disableVoice"), boolParam(e, "shouldMarch")}
		m.marches = append(m.marches, me)
		if !me.noVoice {
			ctx.PlaySeq("susume", b)
			ctx.At(b, func() { m.sargeTalk(b) })
		}
		// PreMarch：B+1 全员 MarchL
		ctx.At(b+1, func() {
			for _, c := range m.others() {
				ctx.Scene.PlayState(c, "MarchL", b+1, 0.5)
			}
			ctx.Scene.PlayState(m.player(), "MarchL", b+1, 0.5)
		})
	case "marchingOrders/faceTurn", "marchingOrders/faceTurnFast":
		fast := e.Datamodel == "marchingOrders/faceTurnFast"
		right := int(e.Float("direction", 0)) == 0
		point := boolParam(e, "point")
		m.scheduleFaceTurn(b, right, fast, point)
	case "marchingOrders/halt":
		m.halts = append(m.halts, b)
		mute := boolParam(e, "mute")
		if !mute {
			ctx.PlaySeq("tomare", b)
			ctx.At(b, func() { m.sargeTalk(b) })
		}
		ctx.ScheduleInputAction(b+1, 3, func(state float64, _ engine.Judgment) {
			if state <= -1 || state >= 1 {
				ctx.PlayCommon("nearMiss")
			} else {
				ctx.SoundVol("stepPlayer", 0.75)
			}
			ctx.Scene.PlayState(m.player(), "Halt", ctx.Beat(), 0.5)
		}, func() { m.miss(false) })
		ctx.At(b+1, func() {
			for _, c := range m.others() {
				ctx.Scene.PlayState(c, "Halt", b+1, 0.5)
			}
		})
	case "marchingOrders/forceMarching":
		for i := 0.0; i < e.Length; i++ {
			bb := b + i
			ctx.ScheduleInput(bb, m.marchHit, func() { m.miss(false) })
			ctx.At(bb, func() { m.otherStep(bb) })
		}
	case "marchingOrders/background":
		bg := bgEvt{beat: b, preset: int(e.Float("preset", 0))}
		for i, k := range []string{"colorFill", "colorTiles1", "colorTiles2", "colorTiles3",
			"colorPipes1", "colorPipes2", "colorPipes3", "colorConveyor1", "colorConveyor2"} {
			bg.colors[i] = colorParam(e, k)
		}
		m.bgs = append(m.bgs, bg)
		ctx.At(b, func() { m.applyBG(bg) })
	case "marchingOrders/go":
		length := e.Length
		start := boolParam(e, "start")
		dir := int(e.Float("direction", 0))
		ctx.At(b, func() {
			speed := 20.0
			if dir != 0 {
				speed = -20
			}
			m.conveyorSpeed = speed / length * (60 / m.ctx.SecPerBeat(b) / 100)
			m.conveyorOn = start
		})
	}
}

// Ready：行进链静态展开（march → 每拍输入/踏步，直到 halt）+ 自动 bop。
func (m *Module) Ready() {
	sort.Slice(m.halts, func(i, j int) bool { return m.halts[i] < m.halts[j] })
	for b := 0.0; b <= m.endBeat; b++ {
		bb := b
		m.ctx.At(bb, func() {
			if m.goBop {
				m.allBop(bb)
			}
		})
	}
	for _, me := range m.marches {
		if !me.march {
			continue
		}
		// 下一个 halt（无则到谱面尾不会发生——官方谱面都有 halt）
		end := math.Inf(1)
		for _, h := range m.halts {
			if h > me.beat {
				end = h
				break
			}
		}
		if math.IsInf(end, 1) {
			continue
		}
		for mm := me.beat + 1; mm <= end-1; mm++ {
			judge := mm + 1
			m.ctx.ScheduleInput(judge, m.marchHit, func() { m.miss(false) })
			m.ctx.At(judge, func() { m.otherStep(judge) })
		}
	}
}

func (m *Module) marchHit(state float64, _ engine.Judgment) {
	ctx := m.ctx
	if state <= -1 || state >= 1 {
		ctx.PlayCommon("nearMiss")
	} else {
		ctx.SoundVol("stepPlayer", 0.75)
	}
	m.playerStep++
	anim := "MarchL"
	if m.playerStep%2 != 0 {
		anim = "MarchR"
	}
	ctx.Scene.PlayState(m.player(), anim, ctx.Beat(), 0.5)
}

func (m *Module) otherStep(beat float64) {
	m.otherCount++
	anim := "MarchL"
	if m.otherCount%2 != 0 {
		anim = "MarchR"
	}
	for _, c := range m.others() {
		m.ctx.Scene.PlayState(c, anim, beat, 0.5)
	}
	m.ctx.Sound("stepOther")
}

func (m *Module) scheduleFaceTurn(b float64, right, fast, point bool) {
	ctx := m.ctx
	turnLength := 1.0
	if fast {
		turnLength = 0
	}
	dirName := "left"
	action := 1
	suffix := "L"
	if right {
		dirName, action, suffix = "right", 2, "R"
	}
	fastSfx := ""
	if fast {
		fastSfx = "fast"
	}
	ctx.SoundAt(b, dirName+"FaceTurn1"+fastSfx, 1)
	ctx.SoundAt(b+0.5, dirName+"FaceTurn2"+fastSfx, 1)
	ctx.SoundAt(b+turnLength+1, dirName+"FaceTurn3", 1)
	ctx.SoundAt(b+turnLength+2, "turnAction", 1)
	ctx.At(b, func() { m.sargeTalk(b) })
	ctx.At(b+turnLength+1, func() { m.sargeTalk(b + turnLength + 1) })
	ctx.At(b+turnLength+2, func() {
		beat := b + turnLength + 2
		for _, h := range m.othersHeads() {
			ctx.Scene.PlayState(h, "Face"+suffix, beat, 0.5)
		}
		if point {
			for _, c := range m.others() {
				ctx.Scene.PlayState(c, "Point"+suffix, beat, 0.5)
			}
		}
	})
	ctx.ScheduleInputAction(b+turnLength+2, action, func(state float64, _ engine.Judgment) {
		beat := ctx.Beat()
		if state <= -1 || state >= 1 {
			ctx.PlayCommon("nearMiss")
		} else {
			ctx.Sound("turnActionPlayer")
		}
		ctx.Scene.PlayState(m.playerHead(), "Face"+suffix, beat, 0.5)
		if point {
			ctx.Scene.PlayState(m.player(), "Point"+suffix, beat, 0.5)
		}
	}, func() { m.miss(false) })
}

func (m *Module) sargeTalk(beat float64) {
	m.ctx.Scene.PlayState(m.ctx.Role("Sarge"), "Talk", beat, 0.5)
}

func (m *Module) allBop(beat float64) {
	anim := "Bop"
	if m.clap {
		anim = "Clap"
	}
	for _, c := range m.others() {
		m.ctx.Scene.PlayState(c, anim, beat, 0.5)
	}
	m.ctx.Scene.PlayState(m.player(), anim, beat, 0.5)
}

// miss：怒气蒸汽（1.1 拍限频）。whiff=true 时来自空击（ScoreMiss 由 engine 统计）。
func (m *Module) miss(whiff bool) {
	beat := m.ctx.Beat()
	if beat-m.lastMiss <= 1.1 {
		return
	}
	m.lastMiss = beat
	m.ctx.PlayCommon("miss")
	m.ctx.SoundVol("steam", 0.75)
	m.ctx.Scene.PlayState(m.ctx.Role("Sarge"), "Anger", beat, 0.5)
	m.ctx.Scene.PlayState(m.ctx.Role("Steam"), "Steam", beat, 0.5)
}

// ---------- 背景调色 ----------

var yellowPreset = [9][4]float64{
	{0.26, 0.36, 0.39, 1}, {1, 0.76, 0.52, 1}, {1, 0.6, 0.2, 1}, {1, 0.68, 0, 1},
	{0.41, 0.54, 0.34, 1}, {0.43, 0.8, 0.45, 1}, {0.48, 0.89, 0.54, 1},
	{0.16, 0.25, 0.3, 1}, {0.55, 0.57, 0.04, 1},
}
var bluePreset = [9][4]float64{
	{0.25, 0.45, 0.52, 1}, {0.45, 0.71, 0.81, 1}, {0.65, 0.87, 0.94, 1}, {0.65, 0.87, 0.94, 1},
	{0.36, 0.58, 0.64, 1}, {0.48, 0.65, 0.71, 1}, {0.48, 0.65, 0.71, 1},
	{0.32, 0.55, 0.62, 1}, {0.17, 0.31, 0.35, 1},
}

func (m *Module) applyBG(bg bgEvt) {
	c := bg.colors
	switch bg.preset {
	case 0:
		c = yellowPreset
	case 1:
		c = bluePreset
	}
	m.setMaterials(c)
}

// setMaterials 对应 UpdateMaterialColor：
// tiles: A=t3 B=t2 D=t1；pipes: A=p2 B=p1 D=p3；conveyor: A=黑 B=c1 D=c2；
// 墙体/地面底色直接着色。
func (m *Module) setMaterials(c [9][4]float64) {
	mats := m.ctx.Assets.Extra.RefArrays["RecolorMats"]
	if len(mats) < 3 {
		return
	}
	sc := m.ctx.Scene
	pal := func(a, b, d [4]float64) kart.Palette {
		return kart.Palette{Alpha: a, Fill: b, Outline: d}
	}
	sc.SetPaletteFor(mats[0], pal(c[3], c[2], c[1]))
	sc.SetPaletteFor(mats[1], pal(c[5], c[4], c[6]))
	sc.SetPaletteFor(mats[2], pal([4]float64{0, 0, 0, 1}, c[7], c[8]))
	bgs := m.ctx.Assets.Extra.RefArrays["BackgroundRecolorable"]
	if len(bgs) >= 2 {
		sc.SetColorOver(bgs[0], c[0])
		sc.SetColorOver(bgs[1], c[7])
	}
}

// ---------- 生命周期 ----------

func (m *Module) OnSwitch(beat float64) {
	sec := m.ctx.SecPerBeat(beat)
	sc := m.ctx.Scene
	for _, p := range append(append([]string{}, m.others()...), m.player(), m.ctx.Role("Sarge"), m.ctx.Role("Steam")) {
		sc.PlayDefaultState(p, beat, sec)
	}
	for _, h := range append(append([]string{}, m.othersHeads()...), m.playerHead()) {
		sc.PlayDefaultState(h, beat, sec)
	}
	m.setMaterials(yellowPreset)
	// PersistColor：开场前最后一个 background 事件
	for _, bg := range m.bgs {
		if bg.beat < beat {
			m.applyBG(bg)
		}
	}
}

func (m *Module) Whiff(beat float64) { m.WhiffAction(beat, 0) }

// WhiffAction：各通道空击的反馈（Update 的 whiff 分支）。
func (m *Module) WhiffAction(beat float64, action int) {
	m.miss(true)
	sc := m.ctx.Scene
	switch action {
	case 0:
		m.playerStep++
		anim := "MarchL"
		if m.playerStep%2 != 0 {
			anim = "MarchR"
		}
		sc.PlayState(m.player(), anim, beat, 0.5)
	case 1:
		sc.PlayState(m.playerHead(), "FaceL", beat, 0.5)
	case 2:
		sc.PlayState(m.playerHead(), "FaceR", beat, 0.5)
	case 3:
		sc.PlayState(m.player(), "Halt", beat, 0.5)
	}
}

func (m *Module) Update(t, beat float64) {
	dt := 0.0
	if m.hasLastT && t > m.lastT {
		dt = t - m.lastT
	}
	m.lastT, m.hasLastT = t, true

	// 传送带（go 事件；官方关卡未使用）
	if m.conveyorOn {
		m.conveyorX += m.conveyorSpeed * dt
		convs := m.ctx.Assets.Extra.RefArrays["ConveyorGo"]
		for _, p := range convs {
			if i, ok := m.ctx.Scene.Index(p); ok {
				n := m.ctx.Assets.Rig.Nodes[i]
				m.ctx.Scene.SetPosOver(p, n.Pos[0]+m.conveyorX, n.Pos[1])
			}
		}
	}
}

func (m *Module) Draw(screen *ebiten.Image, t, beat float64) {
	screen.Fill(toRGBA(bgFill))
	sc := m.ctx.Scene
	sc.Sample(beat)
	sc.Draw(screen, m.proj)
}

func toRGBA(c [4]float64) (out colorRGBA) {
	return colorRGBA{uint8(c[0] * 255), uint8(c[1] * 255), uint8(c[2] * 255), uint8(c[3] * 255)}
}

type colorRGBA struct{ R, G, B, A uint8 }

func (c colorRGBA) RGBA() (r, g, b, a uint32) {
	return uint32(c.R) * 0x101, uint32(c.G) * 0x101, uint32(c.B) * 0x101, uint32(c.A) * 0x101
}

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
