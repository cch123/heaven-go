// Package seesaw 是 See-Saw（seeSaw）的玩法模块，
// 逻辑对应 Assets/Scripts/Games/SeeSaw/{SeeSaw,SeeSawGuy}.cs：
//
//	四种跳跃事件 <saw 飞行长度><see 飞行长度>：longLong(4) longShort(3)
//	shortLong(3) shortShort(2)。事件拍 see 落板把 saw 弹起，输入在
//	B+2（long）/B+1（short）接 saw 落板，把 see 弹起，链式衔接。
//	high 跳：弹起更高（height 参数插值 12..28），落板 Lightning + 爆闪
//	+ 轨道珠粒子 + 伪相机跟随（gameTrans 下移）。
//	changeBgColor 渐变背景双色；recolor 调色板映射换色（贴图 RGB 掩码）。
//
// bop/choke 事件未被任何官方关卡使用（全部 Pack-In 谱面核对过），
// 未实现——出现时打日志跳过。
package seesaw

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

// 跳跃状态（SeeSawGuy.JumpState 同值语义）
const (
	stNone = iota
	stStartJump
	stStartJumpIn
	stOutOut
	stInIn
	stInOut
	stOutIn
	stEndJumpOut
	stEndJumpIn
	stHighOutOut
	stHighOutIn
	stHighInOut
	stHighInIn
)

// 落地类型（LandType）
const (
	landBig = iota
	landMiss
	landBarely
	landNormal
)

// jumpPath 是序列化 SuperCurveObject.Path 的运行时形态。
type jumpPath struct {
	from, to string // 目标节点 path（空 = pos(0,0)）
	dur      float64
	height   float64
}

type jumpEvt struct {
	beat, length float64
	model        string // longLong / longShort / shortLong / shortShort
	high         bool
	height       float64
	camMove      bool
}

type bgEvt struct {
	beat, length           float64
	from1, to1, from2, to2 [4]float64
	ease                   int
}

type recolorEvt struct {
	beat          float64
	fill, outline [4]float64
}

type orb struct {
	born   float64
	px, py float64
	vx, vy float64
	g      float64
	life   float64
	size   float64
	sprite string
}

// guy 是 SeeSawGuy 的运行时状态。
type guy struct {
	inst       *kart.Instance
	see        bool
	state      int
	lastState  int
	path       jumpPath
	startBeat  float64
	heightLast float64
	midAirDone bool
	canBop     bool
	rot        float64
	animRel    string // 实例内 Animator 相对 path（"See"/"Saw"）
	holderPath string
}

type Module struct {
	ctx  *engine.Ctx
	proj kart.Aff

	jumps    []jumpEvt // 过滤后（GrabJumpEvents 语义）
	bgEvts   []bgEvt
	recolors []recolorEvt

	see, saw *guy
	paths    map[string]jumpPath

	jumpIdx    int
	canPrepare bool
	camMove    bool
	camHeight  float64 // 高跳相机 path 的运行时高度
	camDur     float64
	camOn      bool
	camBeat    float64

	orbs []orb

	defaultFill, defaultOutline [4]float64
}

func New() engine.Module {
	return &Module{canPrepare: true, camMove: true}
}

func (m *Module) ID() string { return "seeSaw" }

// 主题色（Minigame 注册色 #FF00E4 顶 / #FFB4F7 底）
var bgColor = color.RGBA{0xff, 0xb4, 0xf7, 255}

func (m *Module) Load(ctx *engine.Ctx) error {
	m.ctx = ctx
	if err := ctx.LoadAssets("seeSaw"); err != nil {
		return err
	}
	m.proj = kart.Translate(engine.ScreenW/2, engine.ScreenH/2).Mul(kart.Scale(54, -54))

	// jumpPaths（双点：from→to + duration/height）
	m.paths = map[string]jumpPath{}
	for _, p := range ctx.Assets.Extra.Components["game"].Lists["jumpPaths"] {
		name := p.Strs["name"]
		pos := p.Items["positions"]
		if len(pos) < 2 {
			continue
		}
		m.paths[name] = jumpPath{
			from:   pos[0].Refs["target"],
			to:     pos[1].Refs["target"],
			dur:    pos[0].Nums["duration"],
			height: pos[0].Nums["height"],
		}
	}

	mkGuy := func(holder, animRel string, see bool) *guy {
		t := kart.NewTemplate(ctx.Assets, holder)
		g := &guy{inst: t.NewInstance(), see: see, canBop: true, animRel: animRel, holderPath: holder}
		return g
	}
	m.see = mkGuy(ctx.Role("see"), "See", true)
	m.saw = mkGuy(ctx.Role("saw"), "Saw", false)

	// 调色板默认（FillColor 白 / OutlineColor #0A103C）
	m.defaultFill = [4]float64{1, 1, 1, 1}
	m.defaultOutline = [4]float64{0.03921569, 0.0627451, 0.2352941, 1}
	return nil
}

// ---------- 事件 ----------

func (m *Module) OnEvent(e *riq.Entity) {
	b := e.Beat
	switch e.Datamodel {
	case "seeSaw/longLong", "seeSaw/longShort", "seeSaw/shortLong", "seeSaw/shortShort":
		m.jumps = append(m.jumps, jumpEvt{
			beat: b, length: e.Length, model: e.Datamodel[len("seeSaw/"):],
			high: boolParam(e, "high"), height: e.Float("height", 0),
			camMove: boolParam(e, "camMove"),
		})
	case "seeSaw/changeBgColor":
		m.bgEvts = append(m.bgEvts, bgEvt{
			beat: b, length: e.Length,
			from1: colorParam(e, "colorFrom"), to1: colorParam(e, "colorTo"),
			from2: colorParam(e, "colorFrom2"), to2: colorParam(e, "colorTo2"),
			ease: int(e.Float("ease", 0)),
		})
	case "seeSaw/recolor":
		m.recolors = append(m.recolors, recolorEvt{
			beat: b, fill: colorParam(e, "fill"), outline: colorParam(e, "outline"),
		})
	case "seeSaw/bop", "seeSaw/choke":
		log.Printf("seeSaw: 事件 %s 未实现（官方关卡未使用）", e.Datamodel)
	}
}

// Ready：过滤重叠跳跃事件（GrabJumpEvents）并静态展开时间轴。
func (m *Module) Ready() {
	sort.Slice(m.jumps, func(i, j int) bool { return m.jumps[i].beat < m.jumps[j].beat })
	sort.Slice(m.bgEvts, func(i, j int) bool { return m.bgEvts[i].beat < m.bgEvts[j].beat })
	sort.Slice(m.recolors, func(i, j int) bool { return m.recolors[i].beat < m.recolors[j].beat })
	if len(m.jumps) > 1 {
		kept := m.jumps[:1]
		good := m.jumps[0].beat + m.jumps[0].length
		for _, j := range m.jumps[1:] {
			if j.beat < good {
				continue // 与前一事件重叠：丢弃
			}
			good = j.beat + j.length
			kept = append(kept, j)
		}
		m.jumps = kept
	}

	for i := range m.jumps {
		m.scheduleJump(i)
	}
}

// sawDur/seeDur：事件名两段的飞行拍数。
func sawDur(model string) float64 {
	if model[0] == 'l' {
		return 2
	}
	return 1
}

// nextChained 报告事件 i+1 是否与 i 无缝衔接。
func (m *Module) nextChained(i int) bool {
	return i+1 < len(m.jumps) && m.jumps[i+1].beat == m.jumps[i].beat+m.jumps[i].length
}

// startLandIn 对应 DetermineStartLandInOrOut（基于事件 i）。
func (m *Module) startLandIn(i int) bool {
	e := m.jumps[i]
	in := e.model == "shortLong" || e.model == "shortShort"
	if m.nextChained(i) {
		if e.model == "shortLong" {
			in = false
		} else if e.model == "longShort" {
			in = true
		}
	}
	return in
}

// scheduleJump 静态展开事件 i 的时间轴动作与判定。
func (m *Module) scheduleJump(i int) {
	ctx := m.ctx
	e := m.jumps[i]
	b := e.beat
	sd := sawDur(e.model)
	long := sd == 2 // saw 飞行段是否为 long（决定 Just/Miss 变体与音效）

	// 准备跳（canPrepare：上一段链结束后，see 从地面起跳）
	inJump := m.startLandIn(i)
	prepBeat := b - 1
	if !inJump {
		prepBeat = b - 2
	}
	ctx.At(prepBeat, func() {
		if !m.canPrepare {
			return
		}
		m.canPrepare = false
		ctx.SoundAt(prepBeat, "prepareHigh", 1)
		st := stStartJump
		if inJump {
			st = stStartJumpIn
		}
		m.setGuyState(m.see, st, prepBeat, false, 0)
		m.see.canBop = false
	})

	// 事件拍：see 落板、saw 弹起
	ctx.At(b, func() {
		m.jumpIdx = i
		m.camMove = e.camMove
		m.saw.canBop = false
		m.plankMirror(true)
		m.plankAnim("Good", b)
		if e.high {
			m.landGuy(m.see, landBig, long)
			ctx.Sound("otherHighJump")
		} else {
			m.landGuy(m.see, landNormal, false)
			if long {
				ctx.Sound("otherLongJump")
			} else {
				ctx.Sound("otherShortJump")
			}
		}
		if long {
			ctx.Sound("otherVoiceLong1")
			ctx.SoundAt(b+1, "otherVoiceLong2", 1)
			if e.high {
				ctx.SoundAt(b+1, "midAirShine", 1)
			}
		} else {
			ctx.Sound("otherVoiceShort1")
			ctx.SoundAt(b+0.5, "otherVoiceShort2", 1)
			if e.high {
				ctx.SoundAt(b+0.5, "midAirShine", 1)
			}
		}
		// saw 起跳（DetermineSawJump：下一事件决定 In/Out 落点）
		m.determineSawJump(i, b, e.high, e.height)

		// 链断开时的 see 终落
		if !m.nextChained(i) {
			m.saw.canBop = true
			endLen := m.endLen(e.model)
			ctx.SoundAt(b+endLen, "otherLand", 1)
			ctx.At(b+endLen-0.25, func() { m.see.canBop = true })
			ctx.At(b+endLen, func() {
				m.landGuy(m.see, landNormal, m.seeEndsOut())
				m.canPrepare = true
			})
		}
	})

	// 判定输入（saw 落板）
	ctx.ScheduleInput(b+sd, func(state float64, _ engine.Judgment) {
		ib := b + sd
		m.plankMirror(false)
		if state <= -1 || state >= 1 { // barely（NG 命中）
			m.plankAnim("Bad", ib)
			ctx.Sound("ow")
			m.landGuy(m.saw, landBarely, long)
			m.determineSeeJump(i, ib, true, e.high, e.height)
			return
		}
		m.determineSeeJump(i, ib, false, e.high, e.height)
		if e.high {
			m.plankAnim("Lightning", ib)
			if long {
				ctx.Sound("explosionBlack")
			} else {
				ctx.Sound("explosionWhite")
			}
			m.landGuy(m.saw, landBig, long)
		} else {
			m.plankAnim("Good", ib)
			m.landGuy(m.saw, landNormal, false)
		}
		// 下一事件 high 时玩家弹起音效换高跳版
		if m.nextChained(i) && m.jumps[i+1].high {
			ctx.Sound("playerHighJump")
		} else if long {
			ctx.Sound("playerLongJump")
		} else {
			ctx.Sound("playerShortJump")
		}
		if long {
			ctx.Sound("playerVoiceLong1")
			ctx.Sound("just")
			ctx.SoundAtOff(ib+1, "playerVoiceLong2", 1, 0.0104166666)
		} else {
			ctx.Sound("playerVoiceShort1")
			ctx.Sound("just")
			ctx.SoundAt(ib+0.5, "playerVoiceShort2", 1)
		}
	}, func() {
		ib := b + sd
		m.plankMirror(false)
		m.plankAnim("Bad", ib)
		ctx.Sound("miss")
		m.landGuy(m.saw, landMiss, long)
		m.determineSeeJump(i, ib, true, e.high, e.height)
	})
}

// endLen：链断开时 see 终落的拍数（C# 各事件分支）。
func (m *Module) endLen(model string) float64 {
	switch model {
	case "longLong":
		return 4
	case "longShort":
		if m.seeEndsOut() {
			return 4
		}
		return 3
	case "shortLong":
		if m.seeEndsOut() {
			return 3
		}
		return 2
	default: // shortShort
		return 2
	}
}

// seeEndsOut 对应 see.ShouldEndJumpOut（按 see 的 lastState）。
func (m *Module) seeEndsOut() bool {
	switch m.see.lastState {
	case stInOut, stOutOut, stStartJump, stHighOutOut, stHighInOut:
		return true
	}
	return false
}

// determineSawJump 对应 DetermineSawJump（事件 i 触发，currentJumpIndex 已 +1）。
func (m *Module) determineSawJump(i int, beat float64, high bool, height float64) {
	cur := m.jumps[i]
	fromOut := cur.model == "longShort" || cur.model == "longLong"
	toOut := true
	if i+1 < len(m.jumps) {
		next := m.jumps[i+1].model
		toOut = next == "longShort" || next == "longLong"
	} else if !fromOut {
		toOut = false
	}
	var st int
	switch {
	case fromOut && toOut:
		st = pick(high, stHighOutOut, stOutOut)
	case fromOut && !toOut:
		st = pick(high, stHighOutIn, stOutIn)
	case !fromOut && toOut:
		st = pick(high, stHighInOut, stInOut)
	default:
		st = pick(high, stHighInIn, stInIn)
	}
	m.setGuyState(m.saw, st, beat, false, height)
}

// determineSeeJump 对应 DetermineSeeJump（输入回调里触发，prev = 事件 i）。
func (m *Module) determineSeeJump(i int, beat float64, miss, highArg bool, height float64) {
	prev := m.jumps[i]
	prevLong := prev.model == "longLong" || prev.model == "shortLong" // see 飞行段 long → Out 系
	if m.nextChained(i) {
		next := m.jumps[i+1]
		shouldHigh := next.high || highArg
		nextOut := next.model == "longLong" || next.model == "shortLong"
		var st int
		if prevLong {
			if nextOut {
				st = pick(shouldHigh, stHighOutOut, stOutOut)
			} else {
				st = pick(shouldHigh, stHighOutIn, stOutIn)
			}
		} else {
			// C#：短链分支只看 highArg（next.high 不参与）
			if nextOut {
				st = pick(highArg, stHighInOut, stInOut)
			} else {
				st = pick(highArg, stHighInIn, stInIn)
			}
		}
		m.setGuyState(m.see, st, beat, miss, height)
		return
	}
	if m.seeEndsOutCur() {
		m.setGuyState(m.see, stEndJumpOut, beat, miss, 0)
	} else {
		m.setGuyState(m.see, stEndJumpIn, beat, miss, 0)
	}
}

// seeEndsOutCur：EndJump 方向按 see 的当前状态（SetState 内 lastState=current 前调用）。
func (m *Module) seeEndsOutCur() bool {
	switch m.see.state {
	case stInOut, stOutOut, stStartJump, stHighOutOut, stHighInOut:
		return true
	}
	return false
}

func pick(c bool, t, f int) int {
	if c {
		return t
	}
	return f
}

// ---------- guy 状态机（SeeSawGuy.SetState / Land / Update） ----------

// pathFor 返回状态对应的 jumpPath（按 see/saw 取前缀），高跳时套用运行时高度。
func (m *Module) pathFor(g *guy, st int, height float64) jumpPath {
	pfx := "Saw"
	if g.see {
		pfx = "See"
	}
	name := ""
	switch st {
	case stStartJump:
		name = "SeeStartJump"
	case stStartJumpIn:
		name = "SeeStartJumpIn"
	case stOutOut:
		name = pfx + "JumpOutOut"
	case stInIn:
		name = pfx + "JumpInIn"
	case stInOut:
		name = pfx + "JumpInOut"
	case stOutIn:
		name = pfx + "JumpOutIn"
	case stEndJumpOut:
		name = "SeeEndJumpOut"
	case stEndJumpIn:
		name = "SeeEndJumpIn"
	case stHighOutOut:
		name = pfx + "HighOutOut"
	case stHighOutIn:
		name = pfx + "HighOutIn"
	case stHighInOut:
		name = pfx + "HighInOut"
	case stHighInIn:
		name = pfx + "HighInIn"
	}
	p := m.paths[name]
	switch st {
	case stHighOutOut, stHighOutIn:
		p.height = lerp(12, 28, height)
		m.camHeight, m.camDur = lerp(10, 26, height), 2
	case stHighInOut, stHighInIn:
		p.height = lerp(9, 20, height)
		m.camHeight, m.camDur = lerp(7, 18, height), 1
	}
	return p
}

func lerp(a, b, t float64) float64 { return a + (b-a)*t }

func (m *Module) setGuyState(g *guy, st int, beat float64, miss bool, height float64) {
	g.lastState = g.state
	g.state = st
	g.startBeat = beat
	g.heightLast = 0
	g.midAirDone = false
	g.rot = 0
	g.path = m.pathFor(g, st, height)

	// 起跳动画（SetState 分支）
	outStart := "Jump_OutOut_Start"
	inStart := "Jump_InIn_Start"
	outInStart := "Jump_OutIn_Start"
	badOut, badIn := "BadOut_SeeReact", "BadIn_SeeReact"
	play := func(name string) { g.inst.PlayState(g.animRel, name, beat, 0.5) }
	switch st {
	case stOutOut, stStartJump, stHighOutOut:
		play(pickS(miss, badOut, outStart))
	case stInIn, stInOut, stStartJumpIn, stHighInOut, stHighInIn:
		play(pickS(miss, badIn, inStart))
	case stOutIn, stEndJumpOut, stEndJumpIn, stHighOutIn:
		play(pickS(miss, badOut, outInStart))
	}
	// 高跳的变身动画（仅 saw）
	if !g.see {
		switch st {
		case stHighOutOut:
			m.ctx.At(beat+1, func() { g.inst.PlayState(g.animRel, "Jump_OutOut_Transform", beat+1, 0.5) })
		case stHighOutIn:
			m.ctx.At(beat+1, func() { g.inst.PlayState(g.animRel, "Jump_OutIn_Transform", beat+1, 0.5) })
		case stHighInOut:
			m.ctx.At(beat+0.5, func() { g.inst.PlayState(g.animRel, "Jump_OutOut_Transform", beat+0.5, 0.5) })
		case stHighInIn:
			m.ctx.At(beat+0.5, func() { g.inst.PlayState(g.animRel, "Jump_OutIn_Transform", beat+0.5, 0.5) })
		}
	}
	// 相机跟随（saw 高跳）
	if !g.see {
		switch st {
		case stHighOutOut, stHighOutIn, stHighInOut, stHighInIn:
			m.camOn, m.camBeat = true, beat
		default:
			m.camOn = false
		}
	}
}

func pickS(c bool, t, f string) string {
	if c {
		return t
	}
	return f
}

// landGuy 对应 SeeSawGuy.Land。
func (m *Module) landGuy(g *guy, landType int, getUpOut bool) {
	beat := m.ctx.Beat()
	g.rot = 0
	landedOut := false
	switch g.state {
	case stInOut, stOutOut, stStartJump, stHighOutOut, stHighInOut:
		landedOut = true
	case stEndJumpOut, stEndJumpIn:
		// 终落：回地面，转 Neutral
		g.state, g.lastState = stNone, g.state
		g.inst.Offset = m.nodeWorld("Game/Curves/See/SeeStartJump/Point0")
		neutral := "NeutralSaw"
		if g.see {
			neutral = "NeutralSee"
		}
		g.inst.PlayState(g.animRel, neutral, beat, 0.5)
		return
	}
	if landType == landBig && !g.see {
		m.spawnOrbs(!landedOut)
	}
	landOut := "In"
	if landedOut {
		landOut = "Out"
	}
	suffix := ""
	switch landType {
	case landBig:
		suffix = "_Big"
	case landMiss:
		suffix = "_Miss"
	case landBarely:
		suffix = "_Barely"
	}
	g.inst.PlayState(g.animRel, "Land_"+landOut+suffix, beat, 0.5)
	if landType != landBarely {
		getUp := "GetUp_" + landOut + suffix
		delay := 0.5
		if getUpOut {
			delay = 1
		}
		m.ctx.At(beat+delay, func() { g.inst.PlayState(g.animRel, getUp, beat+delay, 0.5) })
	}
	if landedOut {
		g.inst.Offset = m.nodeWorld(pickS(g.see, "Game/Curves/See/OutSee", "Game/Curves/Saw/OutSaw"))
	} else {
		g.inst.Offset = m.nodeWorld(pickS(g.see, "Game/Curves/See/InSee", "Game/Curves/Saw/InSaw"))
	}
	g.lastState = g.state
	g.state = stNone
}

// nodeWorld 取场景节点世界坐标（gameTrans 伪相机位移已含在内）。
func (m *Module) nodeWorld(path string) [2]float64 {
	a, ok := m.ctx.Scene.NodeWorld(path)
	if !ok {
		return [2]float64{}
	}
	return [2]float64{a.Tx, a.Ty}
}

// evalPath：SuperCurveObject.GetPathPositionFromBeat（双点 lerp + 抛物线；
// lerp 不钳上界——超时继续外推，由 Land 截断）。
func (m *Module) evalPath(p jumpPath, beat, startBeat float64) ([2]float64, float64) {
	t := 0.0
	if p.dur > 0 {
		t = (beat - startBeat) / p.dur
	}
	if t < 0 {
		t = 0
	}
	from := m.nodeWorld(p.from)
	to := m.nodeWorld(p.to)
	x := from[0] + (to[0]-from[0])*t
	y := from[1] + (to[1]-from[1])*t
	yMul := t*2 - 1
	h := (1 - yMul*yMul) * p.height
	return [2]float64{x, y + h}, h
}

// ---------- plank / 调色 / 粒子 ----------

func (m *Module) plankAnim(name string, beat float64) {
	m.ctx.Scene.PlayState(m.ctx.Role("seeSawAnim"), name, beat, 0.5)
}

func (m *Module) plankMirror(mirror bool) {
	m.ctx.Scene.SetMirrorX(m.ctx.Role("seeSawAnim"), mirror)
}

// spawnOrbs：高跳大落地的轨道珠（prefab：burst 8、初速 45、重力 6/7g、
// 生存 2/5s、尺寸 0.8/1.3；白珠 Seesaw1_33 / 黑珠 Seesaw1_34）。
func (m *Module) spawnOrbs(white bool) {
	src := "Game/Guys/LeftOrbs"
	sprite := "Seesaw1_33"
	life, size, grav := 5.0, 1.3, 7*9.81
	if !white {
		src = "Game/Guys/RightOrbs"
		sprite = "Seesaw1_34"
		life, size, grav = 2.0, 0.8, 6*9.81
	}
	// PlayScaledAsync(0.65)：粒子时钟 0.65/secPerBeat
	sim := 0.65 / m.ctx.SecPerBeat(m.ctx.Beat())
	pos := m.nodeWorld(src)
	t := m.ctx.Time()
	for i := 0; i < 8; i++ {
		ang := randF() * 2 * math.Pi
		spd := 45 * (0.5 + randF()*0.5)
		m.orbs = append(m.orbs, orb{
			born: t, px: pos[0], py: pos[1],
			vx: math.Cos(ang) * spd * sim, vy: math.Abs(math.Sin(ang)) * spd * sim,
			g: grav * sim * sim, life: life / sim, size: size, sprite: sprite,
		})
	}
}

var rngState uint64 = 0x2545f4914f6cdd1d

func randF() float64 {
	rngState ^= rngState << 13
	rngState ^= rngState >> 7
	rngState ^= rngState << 17
	return float64(rngState%1e9) / 1e9
}

// bgColors 按 changeBgColor 时间轴求当前背景双色。
func (m *Module) bgColors(beat float64) ([4]float64, [4]float64) {
	c1 := [4]float64{1, 0, 0.894, 1}     // #FF00E4
	c2 := [4]float64{1, 0.706, 0.969, 1} // #FFB4F7
	for _, e := range m.bgEvts {
		if beat < e.beat {
			break
		}
		t := 1.0
		if e.length > 0 {
			t = clamp01((beat - e.beat) / e.length)
		}
		for i := 0; i < 3; i++ {
			c1[i] = engine.Ease(e.ease, e.from1[i], e.to1[i], t)
			c2[i] = engine.Ease(e.ease, e.from2[i], e.to2[i], t)
		}
	}
	return c1, c2
}

// palette 按 recolor 时间轴求当前调色板。
func (m *Module) palette(beat float64) ([4]float64, [4]float64) {
	fill, outline := m.defaultFill, m.defaultOutline
	for _, e := range m.recolors {
		if beat < e.beat {
			break
		}
		fill, outline = e.fill, e.outline
	}
	return fill, outline
}

// ---------- 生命周期 ----------

func (m *Module) OnSwitch(beat float64) {
	sc := m.ctx.Scene
	// 场景里的 guy 原件隐藏（实例自绘）
	sc.SetActive(m.ctx.Role("see"), false)
	sc.SetActive(m.ctx.Role("saw"), false)
	sec := m.ctx.SecPerBeat(beat)
	m.see.inst.PlayState("See", "NeutralSee", beat, sec)
	m.saw.inst.PlayState("Saw", "NeutralSaw", beat, sec)
	sc.PlayState(m.ctx.Role("seeSawAnim"), "Neut", beat, sec)
	// 初始位置：see 站台外、saw 站板内/外（首事件为 short* 时 saw 在板内侧已就位）
	m.see.inst.Offset = m.nodeWorld("Game/Curves/See/SeeStartJump/Point0")
	if len(m.jumps) > 0 && (m.jumps[0].model == "shortLong" || m.jumps[0].model == "shortShort") {
		m.saw.inst.Offset = m.nodeWorld("Game/Curves/Saw/InSaw")
		m.saw.inst.PlayFrozen("Saw", "GetUp_In", 1)
	} else {
		m.saw.inst.Offset = m.nodeWorld("Game/Curves/Saw/OutSaw")
	}
}

func (m *Module) Whiff(beat float64) {} // SeeSaw 无空击行为

func (m *Module) Update(t, beat float64) {
	// guy 飞行采样 + 半程动画切换（SeeSawGuy.Update）
	for _, g := range []*guy{m.see, m.saw} {
		if g.state == stNone {
			continue
		}
		pos, h := m.evalPath(g.path, beat, g.startBeat)
		g.inst.Offset = pos
		switch g.state {
		case stStartJump, stStartJumpIn:
			if h < g.heightLast && !g.midAirDone {
				g.midAirDone = true
				fall := "Jump_OutOut_Fall"
				if g.state == stStartJumpIn {
					fall = "Jump_InIn_Fall"
				}
				g.inst.PlayState(g.animRel, fall, beat, 0.5)
			}
			g.heightLast = h
		case stOutOut:
			if beat >= g.startBeat+1 && !g.midAirDone {
				g.midAirDone = true
				g.inst.PlayState(g.animRel, "Jump_OutOut_Fall", beat, 0.5)
			}
		case stInIn:
			if beat >= g.startBeat+0.5 && !g.midAirDone {
				g.midAirDone = true
				g.inst.PlayState(g.animRel, "Jump_InIn_Fall", beat, 0.5)
			}
		case stInOut:
			if beat >= g.startBeat+0.5 {
				if !g.midAirDone {
					g.midAirDone = true
					g.inst.PlayState(g.animRel, "Jump_InOut_Tuck", beat, 0.5)
				}
				sign := -1.0
				if g.see {
					sign = 1
				}
				g.rot = sign * 2 * math.Pi * clamp01((beat-(g.startBeat+0.5))/0.75)
			}
		case stOutIn:
			if beat >= g.startBeat+1 {
				if !g.midAirDone {
					g.midAirDone = true
					g.inst.PlayState(g.animRel, "Jump_OutIn_Tuck", beat, 0.5)
				}
				sign := 1.0
				if g.see {
					sign = -1
				}
				g.rot = sign * 2 * math.Pi * clamp01((beat-(g.startBeat+1))/1)
			}
		case stHighOutOut, stHighOutIn, stHighInOut, stHighInIn:
			if g.see && beat >= g.startBeat+1 && !g.midAirDone {
				g.midAirDone = true
				switch g.state {
				case stHighOutOut:
					g.inst.PlayState(g.animRel, "Jump_OutOut_Fall", beat, 0.5)
				case stHighOutIn:
					g.inst.PlayState(g.animRel, "Jump_OutIn_Tuck", beat, 0.5)
				case stHighInOut:
					g.inst.PlayState(g.animRel, "Jump_InOut_Tuck", beat, 0.5)
				case stHighInIn:
					g.inst.PlayState(g.animRel, "Jump_InIn_Fall", beat, 0.5)
				}
			}
		}
		g.inst.Rot = g.rot
	}

	// 伪相机（saw 高跳：gameTrans.y = -max(cameraPath.y, 0)）
	camY := 0.0
	if m.camOn && m.camMove {
		t01 := 0.0
		if m.camDur > 0 {
			t01 = (beat - m.camBeat) / m.camDur
		}
		if t01 < 0 {
			t01 = 0
		}
		yMul := t01*2 - 1
		camY = math.Max((1-yMul*yMul)*m.camHeight, 0)
	}
	if camY != 0 {
		m.ctx.Scene.SetPosOver("Game", 0, -camY)
	} else {
		m.ctx.Scene.ClearPosOver("Game")
	}

	// 轨道珠清理
	alive := m.orbs[:0]
	for _, o := range m.orbs {
		if t-o.born <= o.life {
			alive = append(alive, o)
		}
	}
	m.orbs = alive
}

func (m *Module) Draw(screen *ebiten.Image, t, beat float64) {
	screen.Fill(bgColor)
	sc := m.ctx.Scene

	// 调色板 + 背景色 + recolors 描边
	fill, outline := m.palette(beat)
	sc.SetPalette(kart.Palette{
		Alpha: [4]float64{1, 1, 1, 1}, Fill: fill, Outline: outline,
	})
	c1, c2 := m.bgColors(beat)
	sc.SetColorOver(m.ctx.Role("bgHigh"), c1)
	sc.SetColorOver(m.ctx.Role("gradient"), c1)
	sc.SetColorOver(m.ctx.Role("bgLow"), c2)
	for _, p := range m.ctx.Assets.Extra.RefArrays["recolors"] {
		sc.SetColorOver(p, outline)
	}

	m.ctx.SampleScene(beat)

	// guy 实例（世界坐标直绘；camY 经场景 Game 覆盖已含在锚点里）
	m.see.inst.Queue(sc, beat, kart.Identity(), 0)
	m.saw.inst.Queue(sc, beat, kart.Identity(), 0)

	// 轨道珠
	for _, o := range m.orbs {
		age := t - o.born
		x := o.px + o.vx*age
		y := o.py + o.vy*age - 0.5*o.g*age*age
		sp, ok := m.ctx.Assets.Sheet.Sprites[o.sprite]
		if !ok {
			continue
		}
		ppu := sp.PPU
		if ppu == 0 {
			ppu = m.ctx.Assets.Sheet.PPU
		}
		w := kart.Translate(x, y).Mul(kart.Scale(
			o.size/(float64(sp.W)/ppu), o.size/(float64(sp.H)/ppu)))
		sc.Queue(kart.ExtraSprite{Sprite: o.sprite, World: w, Order: 30})
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
	if m, ok := e.Data[key].(map[string]any); ok {
		get := func(k string) float64 {
			if f, ok := m[k].(float64); ok {
				return f
			}
			return 0
		}
		out = [4]float64{get("r"), get("g"), get("b"), get("a")}
	}
	return out
}
