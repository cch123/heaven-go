// Package totemclimb 是 Totem Climb（totemClimb）的玩法模块，
// 逻辑对应 Assets/Scripts/Games/TotemClimb/ 全部脚本：
//
//	start B-1 起跳；每拍跳上一根图腾（lerp+抛物线，提前 justEarlyTime 落地）
//	triple 段：青蛙三段跳（0.5 拍×2 + 1 拍），落脚板块逐块塌落
//	high 段：B 抓住龙（按住），B+2 甩出（release）2 拍超级跳；
//	         滚动在抓住期间冻结、飞行期间 2 倍追回（ScrollUpdate）
//	stop：anim=true 时落到末柱（Land + OpenWings + 羽毛粒子）
//	bird / bop / 背景滚动对 / 柱子围栏网格 / 地面平铺 照搬各 Manager 脚本
//
// 双层滚动：Game.local = -2s、Game/Scrollable.local = +s（合成 -s），
// 背景与鸟群是兄弟节点不受滚动影响。场景树的 Game 子树整体隐藏，
// 舞台内容全部经 kart.Template 实例 + Queue 注入统一排序。
package totemclimb

import (
	"image/color"
	"math"
	"sort"

	"github.com/hajimehoshi/ebiten/v2"

	"hsdemo/engine"
	"hsdemo/kart"
	"hsdemo/riq"
)

// 主题色（Minigame 注册色 "94543f"）
var bgColor = color.RGBA{0x94, 0x54, 0x3f, 255}

type tripleEvt struct{ beat, length float64 }
type highEvt struct{ beat float64 }
type stopEvt struct {
	beat float64
	anim bool
}
type bopEvt struct{ beat, length float64 }
type birdEvt struct {
	beat, speed float64
	penguin     bool
	amount      int
}

// jumpSeg 是跳者当前运动段。
type jumpSeg struct {
	kind       int // 0=跳跃（lerp+抛物线） 1=抓龙保持
	beat       float64
	duration   float64 // 拍
	height     float64
	from, to   func() [2]float64 // 世界锚点（随滚动实时求值）
	miss       bool
	nearMiss   bool
	high       bool
	playedFall bool
	landed     bool
}

type bird struct {
	inst  *kart.Instance
	speed float64
}

type frogInst struct {
	beat  float64
	inst  *kart.Instance
	wings bool
}

type dragonInst struct {
	beat float64
	inst *kart.Instance
}

// particle 是手写粒子（prefab 序列化参数见 emit* 注释）。
type particle struct {
	born     float64
	px, py   float64
	vx, vy   float64
	sprite   string
	size     float64
	gravity  float64
	life     float64
	shrink   bool    // SizeModule 1→0
	holdFrac float64 // 衰减前保持原尺寸的寿命占比
	order    int
}

type Module struct {
	ctx  *engine.Ctx
	proj kart.Aff

	// 事件
	hasStart              bool
	startBeat             float64
	endBeat               float64
	pillarEnd             float64
	useEndTotem           bool
	hideFence, hideGround bool
	cueIn                 bool
	allStops              []stopEvt
	allAboves             []float64
	triplesRaw            []tripleEvt
	highsRaw              []highEvt
	triples               []tripleEvt
	highs                 []highEvt
	bops                  []bopEvt
	birdsEvt              []birdEvt

	// 序列化参数
	scrollX, scrollY      float64
	xDist, yDist          float64
	jumpH, jumpHT, jumpHH float64

	// 模板与实例
	totemT, frogT, dragonT, endT, birdT, jumperT *kart.Template
	fakeT                                        []*kart.Template
	totems                                       map[int]*kart.Instance
	frogs                                        []*frogInst
	dragons                                      []*dragonInst
	endInst                                      *kart.Instance
	jumper                                       *kart.Instance
	birds                                        []*bird

	seg       jumpSeg
	jumperOn  bool
	holding   bool
	holdBeat  float64
	canUnHold bool

	parts     []particle
	partSimT  float64
	partSpd   float64
	trailOn   bool
	trailMiss bool
	trailAcc  float64

	lastT    float64
	hasLastT bool
}

func New() engine.Module {
	return &Module{
		startBeat: math.Inf(1), endBeat: math.Inf(1), pillarEnd: math.Inf(1),
		totems: map[int]*kart.Instance{}, partSpd: 1, jumperOn: true,
	}
}

func (m *Module) ID() string { return "totemClimb" }

func (m *Module) Load(ctx *engine.Ctx) error {
	m.ctx = ctx
	if err := ctx.LoadAssets("totemClimb"); err != nil {
		return err
	}
	m.proj = kart.Translate(engine.ScreenW/2, engine.ScreenH/2).Mul(kart.Scale(54, -54))

	comp := ctx.Assets.Extra.Components
	m.scrollX = comp["game"].Nums["_scrollSpeedX"]
	m.scrollY = comp["game"].Nums["_scrollSpeedY"]
	m.xDist = comp["totemManager"].Nums["_xDistance"]
	m.yDist = comp["totemManager"].Nums["_yDistance"]
	m.jumpH = comp["jumper"].Nums["_jumpHeight"]
	m.jumpHT = comp["jumper"].Nums["_jumpHeightTriple"]
	m.jumpHH = comp["jumper"].Nums["_jumpHighHeight"]

	as := ctx.Assets
	m.totemT = kart.NewTemplate(as, comp["totemManager"].Refs["_totemTransform"])
	m.frogT = kart.NewTemplate(as, comp["totemManager"].Refs["_frogTransform"])
	m.dragonT = kart.NewTemplate(as, comp["totemManager"].Refs["_dragon"])
	m.endT = kart.NewTemplate(as, comp["totemManager"].Refs["_endTotemTransform"])
	m.birdT = kart.NewTemplate(as, comp["birdManager"].Refs["_birdRef"])
	m.jumperT = kart.NewTemplate(as, ctx.Role("_jumper"))
	for _, p := range []string{"Game/Scrollable/FakeTotems/FakeTotem", "Game/Scrollable/FakeTotems/FakeTotem (1)"} {
		if t := kart.NewTemplate(as, p); t != nil {
			m.fakeT = append(m.fakeT, t)
		}
	}
	m.jumper = m.jumperT.NewInstance()
	return nil
}

// ---------- 事件 ----------

func (m *Module) OnEvent(e *riq.Entity) {
	b := e.Beat
	switch e.Datamodel {
	case "totemClimb/start":
		if !m.hasStart { // C#：取第一个 start
			m.hasStart = true
			m.startBeat = b
			m.hideFence = boolParam(e, "hide")
			m.hideGround = boolParam(e, "ground")
			m.cueIn = boolParam(e, "cue")
		}
	case "totemClimb/stop":
		m.allStops = append(m.allStops, stopEvt{b, boolParam(e, "anim")})
	case "totemClimb/above":
		m.allAboves = append(m.allAboves, b)
	case "totemClimb/triple":
		m.triplesRaw = append(m.triplesRaw, tripleEvt{b, e.Length})
	case "totemClimb/high":
		m.highsRaw = append(m.highsRaw, highEvt{b})
		// preFunction：ready1/ready2（对所有 high 实体，无过滤）
		m.ctx.At(b-2, func() { m.ctx.PlayCommon("ready1") })
		m.ctx.At(b-1, func() { m.ctx.PlayCommon("ready2") })
	case "totemClimb/startCue": // StartCueIn(beat+2)：beatchange_low ×2
		m.ctx.SoundAt(b, "beatchange_low", 1)
		m.ctx.SoundAt(b+1, "beatchange_low", 1)
	case "totemClimb/tripleCue": // TripleCueIn(beat+2)：beatchange ×3
		m.ctx.SoundAt(b, "beatchange", 1)
		m.ctx.SoundAt(b+0.5, "beatchange", 1)
		m.ctx.SoundAt(b+1, "beatchange", 1)
	case "totemClimb/bird":
		be := birdEvt{
			beat: b, speed: e.Float("speed", 3),
			penguin: int(e.Float("type", 0)) == 1,
			amount:  clampI(int(e.Float("amount", 1)), 1, 3),
		}
		m.birdsEvt = append(m.birdsEvt, be)
		m.ctx.At(b, func() { m.spawnBird(be) })
	case "totemClimb/bop":
		m.bops = append(m.bops, bopEvt{b, e.Length})
	}
}

func clampI(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// ---------- 过滤（CalculateStartAndEndBeat / GetHighJumpEvents / GetTripleEvents） ----------

func isOnBeat(start, target float64) bool {
	return math.Mod(target-start, 1) == 0
}

func (m *Module) resolveBeats() {
	if !m.hasStart {
		return
	}
	sort.Slice(m.allStops, func(i, j int) bool { return m.allStops[i].beat < m.allStops[j].beat })
	sort.Float64s(m.allAboves)
	for _, s := range m.allStops {
		if s.beat > m.startBeat {
			m.endBeat, m.useEndTotem = s.beat, s.anim
			break
		}
	}
	for _, a := range m.allAboves {
		if a >= m.startBeat {
			m.pillarEnd = a
			break
		}
	}

	sort.Slice(m.highsRaw, func(i, j int) bool { return m.highsRaw[i].beat < m.highsRaw[j].beat })
	sort.Slice(m.triplesRaw, func(i, j int) bool { return m.triplesRaw[i].beat < m.triplesRaw[j].beat })

	goodAfter := m.startBeat
	for _, h := range m.highsRaw {
		if h.beat < m.startBeat || h.beat >= m.endBeat {
			continue
		}
		if h.beat >= goodAfter && isOnBeat(m.startBeat, h.beat) {
			m.highs = append(m.highs, h)
			goodAfter = h.beat + 4
		}
	}
	lastLen := m.startBeat
	for _, t := range m.triplesRaw {
		if t.beat < m.startBeat || t.beat+t.length > m.endBeat {
			continue
		}
		if t.beat >= lastLen && isOnBeat(m.startBeat, t.beat) {
			conflict := false
			for _, h := range m.highs {
				if h.beat+4 > t.beat && h.beat+4 < t.beat+t.length+4 {
					conflict = true
					break
				}
			}
			if conflict {
				continue
			}
			m.triples = append(m.triples, t)
			lastLen = t.beat + t.length
		}
	}
}

func (m *Module) isTripleBeat(beat float64) bool {
	for _, t := range m.triples {
		if beat >= t.beat && beat < t.beat+t.length {
			return true
		}
	}
	return false
}

func (m *Module) isHighBeat(beat float64) bool {
	_, ok := m.highAt(beat)
	return ok
}

func (m *Module) highAt(beat float64) (highEvt, bool) {
	for _, h := range m.highs {
		if beat >= h.beat && beat < h.beat+4 {
			return h, true
		}
	}
	return highEvt{}, false
}

func (m *Module) isTripleOrHigh(beat float64) bool {
	return m.isHighBeat(beat) || m.isTripleBeat(beat)
}

// ---------- Ready ----------

func (m *Module) Ready() {
	m.resolveBeats()

	// bop（仅 [start, end) 之外生效；start 前的待机律动）
	for _, bp := range m.bops {
		for i := 0.0; i < bp.length; i++ {
			bb := bp.beat + i
			m.ctx.At(bb, func() {
				if bb >= m.startBeat && bb < m.endBeat {
					return
				}
				m.jumper.PlayState("", "Bop", bb, 0.5)
			})
		}
	}

	if !m.hasStart {
		return
	}
	if m.cueIn { // StartCueIn(start)
		m.ctx.SoundAt(m.startBeat-2, "beatchange_low", 1)
		m.ctx.SoundAt(m.startBeat-1, "beatchange_low", 1)
	}

	// 末柱
	if m.useEndTotem && !math.IsInf(m.endBeat, 1) {
		m.endInst = m.endT.NewInstance()
		m.endInst.Offset = [2]float64{
			m.endInst.Offset[0] + m.xDist*(m.endBeat-m.startBeat),
			m.endInst.Offset[1] + m.yDist*(m.endBeat-m.startBeat),
		}
	}
	// 青蛙（每个 triple 每 2 拍一只；围栏顶以上或 hide 时有翅膀）
	pillarEndDist := math.Inf(1)
	if !math.IsInf(m.pillarEnd, 1) {
		pillarEndDist = (m.pillarEnd - m.startBeat) * 1.45
	}
	for _, t := range m.triples {
		for i := 0.0; i < t.length; i += 2 {
			beat := t.beat + i
			in := m.frogT.NewInstance()
			in.Offset = [2]float64{
				in.Offset[0] + m.xDist*(beat-m.startBeat),
				in.Offset[1] + m.yDist*(beat-m.startBeat),
			}
			wings := in.Offset[1] >= pillarEndDist || m.hideFence
			m.frogs = append(m.frogs, &frogInst{beat: beat, inst: in, wings: wings})
		}
	}
	// 龙
	for _, h := range m.highs {
		in := m.dragonT.NewInstance()
		in.Offset = [2]float64{
			in.Offset[0] + m.xDist*(h.beat-m.startBeat),
			in.Offset[1] + m.yDist*(h.beat-m.startBeat),
		}
		m.dragons = append(m.dragons, &dragonInst{beat: h.beat, inst: in})
	}

	// triple 的 enter/exit cue（TripleJumpSound：对全部 triple 实体；
	// 与高跳衔接时跳过对应侧；过滤列表只用于衔接判断）
	for _, t := range m.triplesRaw {
		if len(m.triples) == 0 {
			break
		}
		length := math.Max(t.length, 2)
		doEnter := true
		checkEnter := t.beat - 1
		for m.isHighBeat(checkEnter) {
			checkEnter -= 4
			if m.isTripleBeat(checkEnter) {
				doEnter = false
			}
		}
		if doEnter {
			m.ctx.SoundAt(t.beat-2, "beatchange", 1)
			m.ctx.SoundAt(t.beat-1.5, "beatchange", 1)
			m.ctx.SoundAt(t.beat-1, "beatchange", 1)
		}
		doExit := true
		checkExit := t.beat + length
		for m.isHighBeat(checkExit) {
			checkExit += 4
			if m.isTripleBeat(checkExit) {
				doExit = false
			}
		}
		if doExit {
			m.ctx.SoundAt(checkExit-2, "beatchange_low", 1)
			m.ctx.SoundAt(checkExit-1, "beatchange_low", 1)
		}
	}

	// 起跳 + 输入链
	sb := m.startBeat
	m.ctx.At(sb-1, func() { m.beginJump(sb-1, false, false) })
	m.simInputs(sb - 1)
}

// ---------- 输入链静态展开（StartJumping/TripleJumping/HighJump 的调度部分） ----------

func (m *Module) simInputs(b float64) { // StartJumping(b)
	if b+1 >= m.endBeat {
		return
	}
	switch {
	case m.isHighBeat(b + 1):
		m.scheduleHoldPress(b + 1)
		m.scheduleRelease(b + 3)
		m.simHigh(b + 3)
	case m.isTripleBeat(b + 1):
		m.scheduleTripleEnter(b + 1)
		m.simTriple(b+1, true)
	default:
		m.scheduleJust(b + 1)
		m.simInputs(b + 1)
	}
}

func (m *Module) simTriple(b float64, enter bool) { // TripleJumping(b, enter)
	if b+0.5 >= m.endBeat {
		return
	}
	if enter {
		m.scheduleTripleExit(b + 0.5)
		m.simTriple(b+0.5, false)
	} else {
		m.scheduleJust(b + 0.5)
		m.simInputs(b + 0.5)
	}
}

func (m *Module) simHigh(b float64) { // HighJump(b)
	if b+2 >= m.endBeat {
		return
	}
	switch {
	case m.isHighBeat(b + 2):
		m.scheduleHoldPress(b + 2)
		m.scheduleRelease(b + 4)
		m.simHigh(b + 4)
	case m.isTripleBeat(b + 2):
		m.scheduleTripleEnter(b + 2)
		m.simTriple(b+2, true)
	default:
		m.scheduleJust(b + 2)
		m.simInputs(b + 2)
	}
}

// ---------- 输入回调 ----------

func nearMissOf(state float64) bool { return state >= 1 || state <= -1 }

func (m *Module) scheduleJust(beat float64) {
	ctx := m.ctx
	ctx.ScheduleInput(beat, func(state float64, _ engine.Judgment) {
		isTriple := m.isTripleBeat(beat)
		near := nearMissOf(state)
		m.beginJump(beat, false, near)
		m.bopTotemAt(beat)
		if isTriple {
			m.fallFrogAt(beat, 1)
		}
		if near {
			ctx.PlayCommon("nearMiss")
			return
		}
		m.emitJumpBurst()
		if isTriple {
			ctx.Sound("totemlandb")
		} else {
			ctx.Sound("totemland")
		}
	}, func() {
		m.beginJump(beat, true, false)
		m.bopTotemAt(beat)
		if m.isTripleBeat(beat) {
			m.fallFrogAt(beat, 1)
		}
		ctx.PlayCommon("miss")
	})
}

func (m *Module) scheduleTripleEnter(beat float64) {
	ctx := m.ctx
	ctx.ScheduleInput(beat, func(state float64, _ engine.Judgment) {
		near := nearMissOf(state)
		m.beginTriple(beat, true, false, near)
		m.fallFrogAt(beat, -1)
		if near {
			ctx.PlayCommon("nearMiss")
			return
		}
		m.emitJumpBurst()
		ctx.Sound("totemland")
	}, func() {
		m.beginTriple(beat, true, true, false)
		m.fallFrogAt(beat, -1)
		ctx.PlayCommon("miss")
	})
}

func (m *Module) scheduleTripleExit(beat float64) {
	ctx := m.ctx
	ctx.ScheduleInput(beat, func(state float64, _ engine.Judgment) {
		near := nearMissOf(state)
		m.beginTriple(beat, false, false, near)
		m.fallFrogAt(beat, 0)
		if near {
			ctx.PlayCommon("nearMiss")
			return
		}
		m.emitJumpBurst()
		ctx.Sound("totemland")
	}, func() {
		m.beginTriple(beat, false, true, false)
		m.fallFrogAt(beat, 0)
		ctx.PlayCommon("miss")
	})
}

// scheduleHoldPress：抓龙（JustHold；C# 的 miss 回调为 Empty——
// 漏抓不惩罚，但仍进入保持段以便 release 判定衔接）。
func (m *Module) scheduleHoldPress(beat float64) {
	ctx := m.ctx
	begin := func(playSfx bool) {
		if playSfx {
			stop := ctx.SoundLoop("charge_start")
			ctx.At(beat+1, stop)
			ctx.SoundAt(beat+1, "charge_end", 1)
		}
		m.holdDragonAt(beat)
		m.beginHold(beat)
	}
	ctx.ScheduleInput(beat, func(state float64, _ engine.Judgment) {
		begin(true)
		if nearMissOf(state) {
			ctx.PlayCommon("nearMiss")
		}
	}, func() {
		begin(false)
	})
}

func (m *Module) scheduleRelease(beat float64) {
	ctx := m.ctx
	ctx.ScheduleInputRelease(beat, func(state float64, _ engine.Judgment) {
		// C#: HighJump(beat, state >= 1f && state <= -1f) —— 恒 false，
		// 好坏都按命中姿态飞行（nearMiss 只响音效）
		m.beginHighJump(beat, false)
		m.releaseDragonAt(beat)
		ctx.Sound("superjumpgood")
		if nearMissOf(state) {
			ctx.PlayCommon("nearMiss")
			return
		}
		m.emitJumpBurst()
	}, func() {
		m.beginHighJump(beat, true)
		m.releaseDragonAt(beat)
		ctx.PlayCommon("miss")
	})
}

// ---------- 跳跃段（JumpCo / JumpTripleCo / JumpHighCo / HoldCo） ----------

func (m *Module) justEarlyBeats(beat float64) float64 {
	return engine.WinJust / m.ctx.SecPerBeat(beat)
}

func (m *Module) landTarget(beat float64) func() [2]float64 {
	switch {
	case beat >= m.endBeat && m.useEndTotem:
		return m.endJumperPoint
	case m.isHighBeat(beat):
		return m.dragonPointAt(beat)
	case m.isTripleBeat(beat):
		return m.frogPointAt(beat, -1)
	default:
		return m.totemPointAt(beat)
	}
}

func (m *Module) beginJump(beat float64, miss, near bool) {
	var from func() [2]float64
	switch {
	case beat < m.startBeat:
		from = m.initialPoint
	case m.isTripleBeat(beat):
		from = m.frogPointAt(beat, 1)
	default:
		from = m.totemPointAt(beat)
	}
	dur := 1 - m.justEarlyBeats(beat+1)
	if m.endBeat <= beat+1 {
		dur = 1
	}
	m.seg = jumpSeg{
		beat: beat, duration: dur, height: m.jumpH,
		from: from, to: m.landTarget(beat + 1), miss: miss, nearMiss: near,
	}
	m.holding = false
	m.stopTrail()
	m.playJumpAnim(beat, miss, near, false)
}

func (m *Module) beginTriple(beat float64, enter, miss, near bool) {
	fromPart, toPart := 0, 1
	if enter {
		fromPart, toPart = -1, 0
	}
	m.seg = jumpSeg{
		beat: beat, duration: 0.5 - m.justEarlyBeats(beat+0.5), height: m.jumpHT,
		from: m.frogPointAt(beat, fromPart), to: m.frogPointAt(beat+0.5, toPart),
		miss: miss, nearMiss: near,
	}
	m.stopTrail()
	m.playJumpAnim(beat, miss, near, false)
}

func (m *Module) beginHold(beat float64) {
	m.holding, m.holdBeat, m.canUnHold = true, beat, true
	pt := m.dragonPointAt(beat)
	m.seg = jumpSeg{kind: 1, beat: beat, duration: 2, from: pt, to: pt}
	m.jumper.PlayState("", "Hold", beat, 0.5)
	m.stopTrail()
}

func (m *Module) beginHighJump(beat float64, miss bool) {
	m.holding = false
	dur := 2 - m.justEarlyBeats(beat+2)
	if m.endBeat <= beat+2 {
		dur = 2
	}
	var to func() [2]float64
	switch {
	case m.endBeat <= beat+2 && m.useEndTotem:
		to = m.endJumperPoint
	case m.isHighBeat(beat + 2):
		to = m.dragonPointAt(beat + 2)
	case m.isTripleBeat(beat + 2):
		to = m.frogPointAt(beat+2, -1)
	default:
		to = m.totemPointAt(beat + 2)
	}
	m.seg = jumpSeg{
		beat: beat, duration: dur, height: m.jumpHH,
		from: m.dragonPointAt(beat), to: to, miss: miss, high: true,
	}
	m.playJumpAnim(beat, miss, false, true)
	m.trailOn, m.trailMiss, m.trailAcc = true, miss, 0
	m.partSpd = 0.5 / m.ctx.SecPerBeat(beat)
}

func (m *Module) playJumpAnim(beat float64, miss, near, high bool) {
	switch {
	case high && !miss:
		m.jumper.PlayState("", "HighJump", beat, 0.5)
	case high && miss:
		m.jumper.PlayState("", "HighMiss", beat, 0.5)
	case miss:
		m.jumper.PlayState("", "Miss", beat, 0.5)
	case near:
		m.jumper.PlayState("", "NearMiss", beat, 0.5)
	default:
		m.jumper.PlayState("", "Jump", beat, 0.5)
	}
}

func (m *Module) stopTrail() { m.trailOn = false }

// ---------- 锚点 ----------

// stageAff 返回滚动容器的世界变换（Game -2s + Scrollable +s = -s）。
func (m *Module) stageAff() kart.Aff {
	sx, sy := m.scrollPos(m.ctx.Beat())
	return kart.Translate(-sx, -sy)
}

func (m *Module) initialPoint() [2]float64 {
	i, ok := m.ctx.Assets.NodeIndex("Game/Scrollable/InitialJumperPoint")
	if !ok {
		return [2]float64{}
	}
	n := m.ctx.Assets.Rig.Nodes[i]
	x, y := m.stageAff().Apply(n.Pos[0], n.Pos[1])
	return [2]float64{x, y}
}

func (m *Module) totemPointAt(beat float64) func() [2]float64 {
	return func() [2]float64 {
		in := m.totemInstance(beat)
		if in == nil {
			return m.gridFallback(beat)
		}
		a, _ := in.NodeWorld("JumperPoint", m.stageAff())
		return [2]float64{a.Tx, a.Ty}
	}
}

// gridFallback：图腾不可见时的网格坐标兜底（不应到达；保画面不崩）。
func (m *Module) gridFallback(beat float64) [2]float64 {
	k := beat - m.startBeat
	x, y := m.stageAff().Apply(-1.209-0.036, -2.99+2.869)
	return [2]float64{x + m.xDist*k, y + m.yDist*k}
}

func (m *Module) frogPointAt(beat float64, part int) func() [2]float64 {
	rel := "JumperPointRight"
	switch part {
	case -1:
		rel = "JumperPointLeft"
	case 0:
		rel = "JumperPointMiddle"
	}
	return func() [2]float64 {
		f := m.frogAt(beat)
		if f == nil {
			return m.gridFallback(beat)
		}
		a, _ := f.inst.NodeWorld(rel, m.stageAff())
		return [2]float64{a.Tx, a.Ty}
	}
}

func (m *Module) dragonPointAt(beat float64) func() [2]float64 {
	return func() [2]float64 {
		d := m.dragonAt(beat)
		if d == nil {
			return m.gridFallback(beat)
		}
		a, _ := d.inst.NodeWorld("JumperPoint", m.stageAff())
		return [2]float64{a.Tx, a.Ty}
	}
}

func (m *Module) endJumperPoint() [2]float64 {
	if m.endInst == nil {
		return [2]float64{}
	}
	a, _ := m.endInst.NodeWorld("Holder/jumperEnd", m.stageAff())
	return [2]float64{a.Tx, a.Ty}
}

func (m *Module) frogAt(beat float64) *frogInst {
	for _, f := range m.frogs {
		if beat >= f.beat && beat < f.beat+2 {
			return f
		}
	}
	return nil
}

func (m *Module) dragonAt(beat float64) *dragonInst {
	for _, d := range m.dragons {
		if beat >= d.beat && beat < d.beat+4 {
			return d
		}
	}
	return nil
}

// totemInstance：beat 拍的图腾（可见性规则同 InitBeats/Update：
// 末拍之后无图腾、triple/high 拍无图腾）。
func (m *Module) totemInstance(beat float64) *kart.Instance {
	end := m.endBeat
	if !m.useEndTotem {
		end += 1
	}
	if beat >= end || m.isTripleOrHigh(beat) || beat < m.startBeat {
		return nil
	}
	key := int(math.Round(beat - m.startBeat))
	if in, ok := m.totems[key]; ok {
		return in
	}
	in := m.totemT.NewInstance()
	in.Offset = [2]float64{
		in.Offset[0] + m.xDist*float64(key),
		in.Offset[1] + m.yDist*float64(key),
	}
	m.totems[key] = in
	return in
}

// ---------- 实例动作 ----------

func (m *Module) bopTotemAt(beat float64) {
	if in := m.totemInstance(beat); in != nil {
		in.PlayState("", "Bop", m.ctx.Beat(), 0.5)
	}
}

func (m *Module) fallFrogAt(beat float64, part int) {
	f := m.frogAt(beat)
	if f == nil {
		return
	}
	b := m.ctx.Beat()
	if f.wings { // FallPiece：有翅膀时切到不扇动姿态
		f.inst.PlayFrozen("", "WingsNoFlap", 0)
	}
	switch part {
	case -1:
		f.inst.PlayState("left", "Fall", b, 0.5)
	case 0:
		f.inst.PlayState("middle", "Fall", b, 0.5)
	default:
		f.inst.PlayState("right", "Fall", b, 0.5)
	}
}

func (m *Module) holdDragonAt(beat float64) {
	if d := m.dragonAt(beat); d != nil {
		d.inst.PlayState("", "Hold", m.ctx.Beat(), 0.25)
	}
}

func (m *Module) releaseDragonAt(beat float64) {
	if d := m.dragonAt(beat); d != nil {
		d.inst.PlayState("", "Release", m.ctx.Beat(), 0.5)
	}
}

// ---------- 滚动（ScrollUpdate） ----------

func (m *Module) scrollPos(beat float64) (float64, float64) {
	if !m.hasStart {
		return 0, 0
	}
	beatDist := m.endBeat - m.startBeat
	nb := beat - m.startBeat
	if nb < 0 {
		nb = 0
	}
	if nb > beatDist {
		nb = beatDist
	}
	if h, ok := m.highAt(nb + m.startBeat); ok {
		highBeat := h.beat - m.startBeat
		if nb >= highBeat+2 {
			catch := (beat - (h.beat + 2)) / 2
			catch = math.Max(0, math.Min(1, catch))
			nb = nb - 2 + catch*2
			nb = math.Max(highBeat, math.Min(highBeat+4, nb))
		} else if nb >= highBeat {
			nb = highBeat
		}
	}
	return nb * m.scrollX, nb * m.scrollY
}

// ---------- 鸟 ----------

func (m *Module) spawnBird(be birdEvt) {
	in := m.birdT.NewInstance()
	if be.amount >= 2 {
		in.SetActive("BirdRef (1)", true)
	}
	if be.amount >= 3 {
		in.SetActive("BirdRef (2)", true)
	}
	if be.penguin {
		sp := m.ctx.Assets.Extra.Components["birdManager"].Sprites["_penguinSprite"]
		in.SetSprite("", sp)
		in.SetSprite("BirdRef (1)", sp)
		in.SetSprite("BirdRef (2)", sp)
	}
	m.birds = append(m.birds, &bird{inst: in, speed: be.speed})
}

// ---------- 粒子 ----------

// emitJumpBurst：JumpParticle（star ×4、初速 8、生存 0.2s、尺寸 0.5、
// 尺寸曲线 1-τ²、球形发射半径 1、order 51、节点位于脚下 (0,-1.44)）。
func (m *Module) emitJumpBurst() {
	m.partSpd = 0.5 / m.ctx.SecPerBeat(m.ctx.Beat())
	px, py := m.jumperWorld()
	for i := 0; i < 4; i++ {
		dx, dy := randDir2()
		r := randF()
		m.parts = append(m.parts, particle{
			born: m.partSimT, px: px + dx*r, py: py - 1.44*0.5875 + dy*r,
			vx: dx * 8, vy: dy * 8,
			sprite: "star", size: 0.5, life: 0.2, shrink: true, order: 51,
		})
	}
}

// emitFeathers：FeatherEffect ×2（totemclimb_2 ×5、初速 20、重力 2、
// 生存 2s、尺寸 1、order 100）。
func (m *Module) emitFeathers() {
	m.partSpd = 0.5 / m.ctx.SecPerBeat(m.ctx.Beat())
	if m.endInst == nil {
		return
	}
	for _, rel := range []string{"FeatherEffect", "FeatherEffect (1)"} {
		a, ok := m.endInst.NodeWorld(rel, m.stageAff())
		if !ok {
			continue
		}
		for i := 0; i < 5; i++ {
			dx, dy := randDir2()
			r := randF()
			m.parts = append(m.parts, particle{
				born: m.partSimT, px: a.Tx + dx*r, py: a.Ty + dy*r,
				vx: dx * 20, vy: dy * 20,
				sprite: "totemclimb_2", size: 1, life: 2, gravity: 2 * 9.81, order: 100,
			})
		}
	}
}

var rngState uint64 = 0x9e3779b97f4a7c15

func randF() float64 {
	rngState ^= rngState << 13
	rngState ^= rngState >> 7
	rngState ^= rngState << 17
	return float64(rngState%1e9) / 1e9
}

func randDir2() (float64, float64) {
	ang := randF() * 2 * math.Pi
	return math.Cos(ang), math.Sin(ang)
}

// ---------- 跳者轨迹 ----------

func (m *Module) jumperWorld() (float64, float64) {
	beat := m.ctx.Beat()
	s := &m.seg
	if s.from == nil {
		p := m.initialPoint()
		return p[0], p[1]
	}
	if s.kind == 1 {
		p := s.from()
		return p[0], p[1]
	}
	t := (beat - s.beat) / s.duration
	t = math.Max(0, math.Min(1, t))
	from, to := s.from(), s.to()
	x := from[0] + (to[0]-from[0])*t
	y := from[1] + (to[1]-from[1])*t
	yMul := t*2 - 1
	y += s.height * (1 - yMul*yMul)
	return x, y
}

// ---------- 生命周期 ----------

func (m *Module) OnSwitch(beat float64) {
	sec := m.ctx.SecPerBeat(beat)
	m.jumper.PlayDefaultState("", beat, sec)
	for _, f := range m.frogs {
		if f.wings { // TCFrog.Update：有翅膀即循环扇动
			f.inst.PlayState("", "Wings", beat, sec)
		}
	}
	// 场景树的 Game 子树整体隐藏（舞台全部实例自绘）
	m.ctx.Scene.SetActive("Game", false)
	// 背景滚动对原件隐藏（逐帧手动绘制，含 18 个克隆）
	for _, item := range m.ctx.Assets.Extra.Components["backgroundManager"].Lists["_objects"] {
		m.ctx.Scene.SetActive(item.Refs["first"], false)
		m.ctx.Scene.SetActive(item.Refs["second"], false)
	}
}

func (m *Module) Whiff(beat float64) {} // TCJumper 无空击行为；保持期轮询见 Update

func (m *Module) Update(t, beat float64) {
	dt := 0.0
	if m.hasLastT && t > m.lastT {
		dt = t - m.lastT
	}
	m.lastT, m.hasLastT = t, true

	// HoldCo 轮询：提前松手 → UnHold + ScoreMiss；再按下 → 重新 Hold + nearMiss
	if m.holding {
		if beat >= m.holdBeat+2 {
			m.holding = false
		} else {
			if m.canUnHold && m.ctx.ReleasedNow() && !m.ctx.ExpectingReleaseNow() {
				m.jumper.PlayState("", "UnHold", beat, 0.5)
				m.ctx.ScoreMiss()
				m.canUnHold = false
			}
			if !m.canUnHold && m.ctx.PressedNow() {
				m.jumper.PlayState("", "Hold", beat, 0.5)
				m.ctx.PlayCommon("nearMiss")
				m.ctx.ScoreMiss()
				m.canUnHold = true
			}
		}
	}

	// 跳跃段推进：Fall 切换、落地 Idle、末柱事件
	s := &m.seg
	if s.from != nil && s.kind == 0 {
		norm := (beat - s.beat) / s.duration
		if norm >= 0.5 && !s.playedFall {
			s.playedFall = true
			if !s.miss && !s.nearMiss && !s.high {
				m.jumper.PlayState("", "Fall", beat, 0.5)
			} else if s.high && !s.miss {
				m.jumper.PlayState("", "HighFall", beat, 0.5)
			}
		}
		if norm >= 1 && !s.landed {
			s.landed = true
			m.jumper.PlayState("", "Idle", beat, 0.5)
			m.stopTrail()
			landBeat := s.beat + math.Round(s.duration)
			if landBeat >= m.endBeat && m.hasStart {
				m.ctx.Sound("totemland")
				if m.useEndTotem {
					m.doEndTotem(landBeat)
					m.jumperOn = false
				}
			}
		}
	}

	// 高跳拖尾（HighParticle/Miss：rate 15/s、初速 3、生存 0.5s、
	// 尺寸 0.5、保持到 0.6 寿命后衰减、order 3）
	m.partSimT += dt * m.partSpd
	if m.trailOn {
		m.trailAcc += dt * m.partSpd * 15
		px, py := m.jumperWorld()
		for m.trailAcc >= 1 {
			m.trailAcc--
			dx, dy := randDir2()
			r := randF()
			sp := "star"
			if m.trailMiss {
				sp = "swirl"
			}
			m.parts = append(m.parts, particle{
				born: m.partSimT, px: px + dx*r, py: py + dy*r,
				vx: dx * 3, vy: dy * 3,
				sprite: sp, size: 0.5, life: 0.5, shrink: true, holdFrac: 0.6, order: 3,
			})
		}
	}
	alive := m.parts[:0]
	for _, p := range m.parts {
		if m.partSimT-p.born <= p.life {
			alive = append(alive, p)
		}
	}
	m.parts = alive

	// 鸟移动（speedX/Y × 倍率，向左下，过死线销毁）
	comp := m.ctx.Assets.Extra.Components["birdManager"]
	deathX := -21.37
	if di, ok := m.ctx.Assets.NodeIndex(comp.Refs["_deathThresholdPoint"]); ok {
		deathX = m.ctx.Assets.Rig.Nodes[di].Pos[0]
	}
	keep := m.birds[:0]
	for _, b := range m.birds {
		b.inst.Offset[0] -= comp.Nums["_speedX"] * b.speed * dt
		b.inst.Offset[1] -= comp.Nums["_speedY"] * b.speed * dt
		if b.inst.Offset[0] <= deathX {
			continue
		}
		keep = append(keep, b)
	}
	m.birds = keep

	// 图腾实例清理（滚出左侧）
	nbNow := beat - m.startBeat
	for key := range m.totems {
		if float64(key) < nbNow-6 {
			delete(m.totems, key)
		}
	}
}

func (m *Module) doEndTotem(beat float64) {
	ctx := m.ctx
	ctx.Sound("finallanding")
	m.endInst.PlayState("", "Land", beat, 0.5)
	ctx.SoundAt(beat+1, "openwings", 1)
	ctx.At(beat+1, func() {
		m.endInst.PlayState("", "OpenWings", beat+1, 0.5)
		// OpenWings 的 AnimationEvent ActivateFeatherEffect @ 0.0667s（ts 0.5）
		ctx.At(beat+1+0.06666667/0.5, m.emitFeathers)
	})
}

// ---------- 绘制 ----------

func (m *Module) Draw(screen *ebiten.Image, t, beat float64) {
	screen.Fill(bgColor)
	sc := m.ctx.Scene
	m.ctx.SampleScene(beat)

	stage := m.stageAff()

	m.queueBackground(t)
	if !m.hideFence {
		m.queuePillars(stage)
	}
	if m.hideGround {
		for _, ft := range m.fakeT {
			ft.NewInstance().Queue(sc, beat, stage, 0)
		}
	} else {
		m.queueGround(stage)
	}
	m.queueTotems(beat, stage)
	for _, f := range m.frogs {
		f.inst.Queue(sc, beat, stage, 0)
	}
	for _, d := range m.dragons {
		d.inst.Queue(sc, beat, stage, 0)
	}
	if m.endInst != nil {
		m.endInst.Queue(sc, beat, stage, 0)
	}
	if m.jumperOn {
		px, py := m.jumperWorld()
		m.jumper.Offset = [2]float64{px, py}
		m.jumper.Queue(sc, beat, kart.Identity(), 0)
	}
	for _, b := range m.birds {
		b.inst.Queue(sc, beat, kart.Identity(), 0)
	}
	for _, p := range m.parts {
		m.queueParticle(&p)
	}

	sc.Draw(screen, m.proj)
}

// queueTotems：可见窗口内的图腾（visibility 规则见 totemInstance）。
func (m *Module) queueTotems(beat float64, stage kart.Aff) {
	sx, _ := m.scrollPos(beat)
	// 图腾 k 的屏幕 x ≈ -1.209 + xDist*k - sx；取 [-12, +13] 视窗
	lo := int(math.Floor((sx - 12 + 1.209) / m.xDist))
	hi := int(math.Ceil((sx + 13 + 1.209) / m.xDist))
	if lo < 0 {
		lo = 0
	}
	for k := lo; k <= hi; k++ {
		in := m.totemInstance(m.startBeat + float64(k))
		if in != nil {
			in.Queue(m.ctx.Scene, beat, stage, 0)
		}
	}
}

// queueGround：地面平铺（TCGroundManager：spacing = ground(1).x - ground.x，
// 横向无限回收；起始 j=0）。
func (m *Module) queueGround(stage kart.Aff) {
	as := m.ctx.Assets
	comp := as.Extra.Components["groundManager"]
	gi, ok := as.NodeIndex(comp.Refs["_groundFirst"])
	if !ok {
		return
	}
	g2, _ := as.NodeIndex(comp.Refs["_groundSecond"])
	n := as.Rig.Nodes[gi]
	dx := as.Rig.Nodes[g2].Pos[0] - n.Pos[0]
	sx, _ := m.scrollPos(m.ctx.Beat())
	lo := int(math.Floor((sx - 12 - n.Pos[0]) / dx))
	hi := int(math.Ceil((sx + 13 - n.Pos[0]) / dx))
	if lo < 0 {
		lo = 0
	}
	for j := lo; j <= hi; j++ {
		world := stage.Mul(kart.TRS(n.Pos[0]+dx*float64(j), n.Pos[1], n.RotZ, n.Scale[0], n.Scale[1]))
		m.ctx.Scene.Queue(kart.ExtraSprite{
			Sprite: n.Sprite, World: world, Layer: n.Layer, Order: n.Order,
		})
	}
}

// queuePillars：柱子+围栏网格（TCPillarManager：dx=3.838、dy=10.1，
// 顶行（y+dy ≥ endDistance）激活 pillartop，顶行以上无柱）。
func (m *Module) queuePillars(stage kart.Aff) {
	as := m.ctx.Assets
	comp := as.Extra.Components["pillarManager"]
	p1, ok1 := as.NodeIndex(comp.Refs["_pillarFirst"])
	p2, ok2 := as.NodeIndex(comp.Refs["_pillarSecond"])
	p3, ok3 := as.NodeIndex(comp.Refs["_pillarUp"])
	if !ok1 || !ok2 || !ok3 {
		return
	}
	base := as.Rig.Nodes[p1]
	dx := as.Rig.Nodes[p2].Pos[0] - base.Pos[0]
	dy := as.Rig.Nodes[p3].Pos[1] - base.Pos[1]
	// _endDistance = (above-start)*1.45（C# 中 _pillarStartY 在赋值时仍为 0）
	endDist := math.Inf(1)
	if !math.IsInf(m.pillarEnd, 1) {
		endDist = (m.pillarEnd - m.startBeat) * 1.45
	}
	topRow := math.MaxInt32
	if !math.IsInf(endDist, 1) {
		topRow = int(math.Ceil((endDist - dy - base.Pos[1]) / dy))
		if topRow < 0 {
			topRow = 0
		}
	}
	sx, sy := m.scrollPos(m.ctx.Beat())
	jLo := int(math.Floor((sx - 12 - base.Pos[0]) / dx))
	jHi := int(math.Ceil((sx + 13 - base.Pos[0]) / dx))
	iLo := int(math.Floor((sy - 8 - base.Pos[1] - 11) / dy)) // 柱体含向下延长段，多画一行
	iHi := int(math.Ceil((sy + 8 - base.Pos[1]) / dy))
	if jLo < 0 {
		jLo = 0
	}
	if iLo < 0 {
		iLo = 0
	}
	if iHi > topRow {
		iHi = topRow
	}
	// 子节点（pillartop / pillarextender）相对柱根
	type sub struct {
		dx, dy, sx2, sy2 float64
		sprite           string
		order            int
		top              bool
	}
	var subs []sub
	rootScale := base.Scale
	for i := p1 + 1; i < len(as.Rig.Nodes); i++ {
		n := as.Rig.Nodes[i]
		if n.Parent != p1 {
			break
		}
		subs = append(subs, sub{n.Pos[0], n.Pos[1], n.Scale[0], n.Scale[1], n.Sprite, n.Order, n.Name == "pillartop"})
	}
	for i := iLo; i <= iHi; i++ {
		for j := jLo; j <= jHi; j++ {
			rootAff := stage.Mul(kart.TRS(base.Pos[0]+dx*float64(j), base.Pos[1]+dy*float64(i), 0, rootScale[0], rootScale[1]))
			m.ctx.Scene.Queue(kart.ExtraSprite{Sprite: base.Sprite, World: rootAff, Layer: base.Layer, Order: base.Order})
			for _, s := range subs {
				if s.top && i != topRow {
					continue // pillartop 仅顶行激活
				}
				m.ctx.Scene.Queue(kart.ExtraSprite{
					Sprite: s.sprite,
					World:  rootAff.Mul(kart.TRS(s.dx, s.dy, 0, s.sx2, s.sy2)),
					Layer:  base.Layer, Order: s.order,
				})
			}
		}
	}
}

// queueBackground：背景滚动对（BackgroundScrollPair：first/second + 18 克隆，
// x = offset - songPos×moveSpeed，过左界回卷 fullDistance）。
func (m *Module) queueBackground(t float64) {
	as := m.ctx.Assets
	parentIdx, _ := as.NodeIndex("backgroundbackground/BackgroundObjects")
	parent := as.Rig.Nodes[parentIdx]
	rootIdx, _ := as.NodeIndex("backgroundbackground")
	root := as.Rig.Nodes[rootIdx]
	parentAff := kart.TRS(root.Pos[0], root.Pos[1], root.RotZ, root.Scale[0], root.Scale[1]).
		Mul(kart.TRS(parent.Pos[0], parent.Pos[1], parent.RotZ, parent.Scale[0], parent.Scale[1]))
	baseZ := root.PosZ + parent.PosZ

	for _, item := range as.Extra.Components["backgroundManager"].Lists["_objects"] {
		fi, ok1 := as.NodeIndex(item.Refs["first"])
		si, ok2 := as.NodeIndex(item.Refs["second"])
		if !ok1 || !ok2 {
			continue
		}
		first, second := as.Rig.Nodes[fi], as.Rig.Nodes[si]
		xDist := second.Pos[0] - first.Pos[0]
		fullDist := 20 * xDist // (18+2) × 间距
		speed := item.Nums["moveSpeed"]
		offsets := []float64{first.Pos[0], second.Pos[0]}
		for i := 0; i < 18; i++ {
			offsets = append(offsets, second.Pos[0]+xDist*float64(i+1))
		}
		for _, off := range offsets {
			x := off - t*speed
			if wrap := math.Ceil(math.Max(0, -fullDist/2-x) / fullDist); wrap > 0 {
				x += wrap * fullDist
			}
			world := parentAff.Mul(kart.TRS(x, first.Pos[1], first.RotZ, first.Scale[0], first.Scale[1]))
			m.ctx.Scene.Queue(kart.ExtraSprite{
				Sprite: first.Sprite, World: world, Z: baseZ + first.PosZ,
				Layer: first.Layer, Order: first.Order,
			})
		}
	}
}

func (m *Module) queueParticle(p *particle) {
	age := m.partSimT - p.born
	x := p.px + p.vx*age
	y := p.py + p.vy*age - 0.5*p.gravity*age*age
	size := p.size
	if p.shrink {
		tau := age / p.life
		if tau > p.holdFrac {
			u := (tau - p.holdFrac) / (1 - p.holdFrac)
			size *= 1 - u*u
		}
	}
	if size <= 0 {
		return
	}
	sp, ok := m.ctx.Assets.Sheet.Sprites[p.sprite]
	if !ok {
		return
	}
	ppu := sp.PPU
	if ppu == 0 {
		ppu = m.ctx.Assets.Sheet.PPU
	}
	world := kart.Translate(x, y).Mul(kart.Scale(
		size/(float64(sp.W)/ppu), size/(float64(sp.H)/ppu)))
	m.ctx.Scene.Queue(kart.ExtraSprite{Sprite: p.sprite, World: world, Order: p.order})
}

func boolParam(e *riq.Entity, key string) bool {
	if v, ok := e.Data[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return false
}
