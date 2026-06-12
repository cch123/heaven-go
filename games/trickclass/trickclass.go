// Package trickclass 是 Trick on the Class（trickClass）的玩法模块，
// 逻辑对应 Assets/Scripts/Games/TrickClass/{TrickClass,MobTrickObj}.cs：
//
//	toss/plane B-1 警示气泡 → B 投掷（音效序列 + 投掷动画 + 生成对象）
//	            对象沿 Bezier 曲线飞行 flyBeats 拍，判定在 B + dodgeBeats
//	blast      B 蓄力（girlCharge 序列 + Charge 动画），判定 B+2
//	判定成功躲闪、NG 被击中、miss 穿过；躲闪有冷却（playerCanDodge）
package trickclass

import (
	"image/color"
	"math"
	"math/rand"
	"sort"

	"github.com/hajimehoshi/ebiten/v2"

	"hsdemo/engine"
	"hsdemo/kart"
	"hsdemo/kmdata"
	"hsdemo/riq"
)

var bgColor = color.RGBA{0xec, 0xed, 0xe4, 255}

// TrickObjType（与 C# 同值）
const (
	typBall = iota
	typChair
	typShock
	typPhone
	typPlane
)

type bopEvt struct {
	beat, length float64
	manual, auto bool
}

// tmplNode 是对象模板子树里一个可绘制节点（相对模板根的变换）。
type tmplNode struct {
	rel    kart.Aff
	sprite string
	order  int
	flipX  bool
}

type tossObj struct {
	startBeat  float64
	flyBeats   float64
	dodgeBeats float64
	curve      kmdata.Curve
	nodes      []tmplNode
	typ        int
	flyType    int
	missed     bool
	rot        float64
	lastPos    [3]float64 // 上一帧世界位置（飞机朝向用帧差，原版同式）
	hasLast    bool

	falling bool // phone 被躲开后自由落体
	fallPos [3]float64
	fallV   float64
	gravity float64
	lastT   float64
}

type Module struct {
	ctx  *engine.Ctx
	proj kart.Aff

	objs    []*tossObj
	bops    []bopEvt
	endBeat float64

	canDodge      float64 // playerCanDodge（拍）
	playerBopGate float64 // playerBopStart
	girlBopGate   float64 // girlBopStart
	girlBopOff    bool    // blast 期间禁用
}

func New() engine.Module {
	return &Module{canDodge: math.Inf(-1), playerBopGate: math.Inf(-1), girlBopGate: math.Inf(-1)}
}

func (m *Module) ID() string { return "trickClass" }

func (m *Module) Load(ctx *engine.Ctx) error {
	m.ctx = ctx
	if err := ctx.LoadAssets("trickClass"); err != nil {
		return err
	}
	m.proj = kart.Translate(engine.ScreenW/2, engine.ScreenH/2).Mul(kart.Scale(54, -54))
	return nil
}

func (m *Module) OnSwitch(beat float64) {
	// 各 Animator 的 controller 默认状态（girl/Player 的 NoPose 把角色摆进
	// 坐姿——prefab 原始摆位是悬浮在课桌上方的）
	m.ctx.Play(m.ctx.Role("warnAnim"), "WarnBubble/NoPose", beat, 0.5)
	m.ctx.Play(m.ctx.Role("girlAnim"), "Girl/NoPose", beat, 0.5)
	m.ctx.Play(m.ctx.Role("playerAnim"), "Player/NoPose", beat, 0.5)
}

// ---------- 事件 ----------

func (m *Module) OnEvent(e *riq.Entity) {
	b := e.Beat
	if end := b + e.Length; end > m.endBeat {
		m.endBeat = end
	}
	switch e.Datamodel {
	case "trickClass/toss":
		m.scheduleToss(b, int(e.Float("obj", 0)), boolParam(e, "nx"))
	case "trickClass/chair":
		m.scheduleToss(b, typChair, false)
	case "trickClass/shock":
		m.scheduleToss(b, typShock, false)
	case "trickClass/phone":
		m.scheduleToss(b, typPhone, boolParam(e, "nx"))
	case "trickClass/plane":
		m.scheduleToss(b, typPlane, false)
	case "trickClass/blast":
		m.scheduleBlast(b)
	case "trickClass/bop":
		m.bops = append(m.bops, bopEvt{b, e.Length, boolParam(e, "bop"), boolParam(e, "autoBop")})
	}
}

var throwSeq = [...]string{"ballThrow", "chairThrow", "shockThrow", "phoneThrow", "planeThrow"}

func (m *Module) scheduleToss(b float64, typ int, variant bool) {
	ctx := m.ctx
	ex := &ctx.Assets.Extra

	tmplPath := ex.RefArrays["objPrefab"][typ]
	if variant && typ < len(ex.RefArrays["objPrefabVariant"]) && ex.RefArrays["objPrefabVariant"][typ] != "" {
		tmplPath = ex.RefArrays["objPrefabVariant"][typ]
	}
	nums := ex.ObjNums[tmplPath]
	strs := ex.ObjStrs[tmplPath]
	flyBeats, dodgeBeats := nums["flyBeats"], nums["dodgeBeats"]
	curve := m.tossCurve(typ)

	ctx.PlaySeq(throwSeq[typ], b)

	warn := ex.Strings["objWarnAnim"][typ]
	if variant && typ < len(ex.Strings["objWarnAnimVariant"]) {
		warn = ex.Strings["objWarnAnimVariant"][typ]
	}
	warnPath, girl := ctx.Role("warnAnim"), ctx.Role("girlAnim")
	throwAnim := ex.Strings["objThrowAnim"][typ]

	ctx.At(b-1, func() { ctx.Play(warnPath, warn, b-1, ctx.SecPerBeat(b)) })

	var obj *tossObj
	ctx.At(b, func() {
		ctx.Play(warnPath, "WarnBubble/NoPose", b, ctx.SecPerBeat(b))
		ctx.Play(girl, "Girl/"+throwAnim, b, 1)
		m.girlBopGate = b + 0.75
		obj = &tossObj{
			startBeat: b, flyBeats: flyBeats, dodgeBeats: dodgeBeats, curve: curve,
			nodes: m.buildTmpl(tmplPath), typ: typ,
			flyType: int(nums["flyType"]), gravity: nums["gravity"],
			rot: rand.Float64() * 2 * math.Pi,
		}
		m.objs = append(m.objs, obj)
	})

	ctx.ScheduleInput(b+dodgeBeats, func(state float64, _ engine.Judgment) {
		if obj == nil {
			return
		}
		beat := ctx.Beat()
		if state <= -1 || state >= 1 { // NG：被砸中
			m.playerDodgeNg(typ == typShock)
			if typ != typShock {
				ctx.SoundAt(obj.startBeat+obj.flyBeats, strs["missSound"], 0.4)
			}
			ctx.SoundVol(strs["missSound"], 0.6)
			ctx.Sound("common_miss") // SoundByte.PlayOneShot("miss")
			m.objMiss(obj)
			return
		}
		// just：躲开
		phone := typ == typPhone
		m.playerDodge(beat, phone, phone)
		ctx.SoundPitch(strs["justSound"], 0.8, 0.85+rand.Float64()*0.3)
		ctx.SoundAt(obj.startBeat+obj.flyBeats, strs["missSound"], 0.4)
		if phone {
			obj.falling = true
			obj.fallPos = kart.EvalBezier(obj.curve, 0.5)
			obj.lastT = ctx.Time()
			obj.startBeat = beat
			obj.flyBeats = 1
		}
	}, func() {
		if obj == nil {
			return
		}
		ctx.SoundVol(strs["missSound"], 1)
		m.objMiss(obj)
		m.playerThrough(typ == typShock)
	})
}

func (m *Module) tossCurve(typ int) kmdata.Curve {
	ex := &m.ctx.Assets.Extra
	switch typ {
	case typPlane:
		return ex.Curves["planeTossCurve"]
	case typShock:
		return ex.Curves["shockTossCurve"]
	default:
		return ex.Curves["ballTossCurve"]
	}
}

// objMiss 对应 MobTrickObj.DoObjMiss：切换到 miss 曲线 / 消失。
func (m *Module) objMiss(o *tossObj) {
	ex := &m.ctx.Assets.Extra
	o.missed = true
	switch o.typ {
	case typPlane:
		// 原版 DoObjMiss：startBeat += dodgeBeats（以判定拍为新起点）
		o.startBeat += o.dodgeBeats
		o.curve = ex.Curves["planeMissCurve"]
		o.flyBeats = 4
		// 朝向重置为 miss 曲线起始方向（GetPoint(0)→GetPoint(1e-6)）
		p0 := kart.EvalBezier(o.curve, 0)
		p1 := kart.EvalBezier(o.curve, 1e-6)
		o.rot = math.Atan2(p1[1]-p0[1], p1[0]-p0[0])
		o.lastPos, o.hasLast = p1, true
	case typShock:
		o.flyBeats = 0 // 立即销毁
	default:
		o.startBeat += o.dodgeBeats
		o.curve = ex.Curves["ballMissCurve"]
		o.flyBeats = 1.25
	}
}

func (m *Module) scheduleBlast(b float64) {
	ctx := m.ctx
	girl, player := ctx.Role("girlAnim"), ctx.Role("playerAnim")

	ctx.PlaySeq("girlCharge", b)
	ctx.At(b, func() {
		m.girlBopOff = true
		ctx.Play(girl, "Girl/Charge0", b, 1)
	})
	ctx.At(b+0.75, func() { ctx.Play(girl, "Girl/Charge1", b+0.75, 1) })
	ctx.At(b+1.5, func() { ctx.Play(girl, "Girl/Charge1", b+1.5, 1) })
	ctx.At(b+4, func() { m.girlBopOff = false })

	ctx.ScheduleInput(b+2, func(state float64, _ engine.Judgment) {
		beat := ctx.Beat()
		if state <= -1 || state >= 1 {
			ctx.Sound("shock_impact")
			ctx.Play(girl, "Girl/BlastNg", beat, 0.5)
			m.playerDodgeNg(true)
			return
		}
		ctx.Play(girl, "Girl/BlastDodged", beat, 0.5)
		if m.canDodge > beat {
			return
		}
		ctx.Sound("blast_dodge")
		ctx.Play(player, "Player/DodgeBlast0", beat, 1)
		m.playerBopGate = beat + 1.25
		m.canDodge = beat + 1
		ctx.SoundAt(b+3, "blast_dodge_return", 1)
		ctx.At(b+3, func() { ctx.Play(player, "Player/DodgeBlast1", b+3, 1) })
	}, func() {
		beat := ctx.Beat()
		ctx.Play(girl, "Girl/BlastNg", beat, 0.5)
		ctx.Sound("blast_miss")
		ctx.Play(player, "Player/ThroughBlast", beat, 1)
		m.playerBopGate = beat + 1.5
		m.canDodge = beat + 0.5
	})
}

// ---------- 角色反应（对应 PlayerDodge / PlayerDodgeNg / PlayerThrough） ----------

func (m *Module) playerDodge(beat float64, slow, alt bool) {
	if m.canDodge > beat {
		return
	}
	ctx := m.ctx
	ctx.Sound("player_dodge")
	anim, ts := "Player/Dodge", 1.0
	if alt {
		anim = "Player/DodgeAlt"
	}
	if slow {
		ts = 0.6
	}
	ctx.Play(ctx.Role("playerAnim"), anim, beat, ts)
	m.playerBopGate = beat + 0.75
}

func (m *Module) playerDodgeNg(shock bool) {
	beat := m.ctx.Beat()
	anim := "Player/DodgeNg"
	if shock {
		anim = "Player/DodgeNgShock"
	}
	m.ctx.Play(m.ctx.Role("playerAnim"), anim, beat, 1)
	m.playerBopGate = beat + 0.75
	m.canDodge = beat + 0.15
}

func (m *Module) playerThrough(shock bool) {
	beat := m.ctx.Beat()
	anim := "Player/Through"
	if shock {
		anim = "Player/ThroughShock"
	}
	m.ctx.Play(m.ctx.Role("playerAnim"), anim, beat, 1)
	m.playerBopGate = beat + 0.75
	m.canDodge = beat + 0.15
}

// Whiff：非预期按键 → 慢速躲闪 + 0.6 拍冷却（原版 Update FlickPress 分支）。
func (m *Module) Whiff(beat float64) {
	if m.canDodge > beat {
		return
	}
	m.playerDodge(beat, true, false)
	m.canDodge = beat + 0.6
}

// ---------- bop ----------

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
	player, girl := m.ctx.Role("playerAnim"), m.ctx.Role("girlAnim")
	for b := range bopBeats {
		b := b
		m.ctx.At(b, func() {
			if b > m.playerBopGate {
				m.ctx.Play(player, "Player/Bop", b, 1)
			}
			if b > m.girlBopGate && !m.girlBopOff {
				m.ctx.Play(girl, "Girl/Bop", b, 1)
			}
		})
	}
}

// ---------- 对象模板绘制 ----------

// buildTmpl 收集模板子树的可绘制节点（相对模板根的变换，根自身的局部变换归一）。
func (m *Module) buildTmpl(rootPath string) []tmplNode {
	rig := &m.ctx.Assets.Rig
	rootIdx := -1
	for i := range rig.Nodes {
		if rig.Nodes[i].Path == rootPath {
			rootIdx = i
			break
		}
	}
	if rootIdx < 0 {
		return nil
	}
	// Instantiate(obj, objHolder) 保留模板根 localScale 并继承祖先链的线性变换；
	// 世界平移由曲线点直接给出，故 rel[root] 只含累计的缩放/旋转
	sx, sy, rot := 1.0, 1.0, 0.0
	for i := rootIdx; i >= 0; i = rig.Nodes[i].Parent {
		n := &rig.Nodes[i]
		sx *= n.Scale[0]
		sy *= n.Scale[1]
		rot += n.RotZ
	}
	rel := map[int]kart.Aff{rootIdx: kart.TRS(0, 0, rot, sx, sy)}
	var out []tmplNode
	add := func(i int, a kart.Aff) {
		n := &rig.Nodes[i]
		if n.Sprite != "" && !n.Hidden {
			out = append(out, tmplNode{rel: a, sprite: n.Sprite, order: n.Order, flipX: n.FlipX})
		}
	}
	add(rootIdx, rel[rootIdx])
	for i := rootIdx + 1; i < len(rig.Nodes); i++ {
		n := &rig.Nodes[i]
		pa, ok := rel[n.Parent]
		if !ok {
			continue // 不在子树内
		}
		a := pa.Mul(kart.TRS(n.Pos[0], n.Pos[1], n.RotZ, n.Scale[0], n.Scale[1]))
		rel[i] = a
		add(i, a)
	}
	sort.SliceStable(out, func(a, b int) bool { return out[a].order < out[b].order })
	return out
}

// ---------- 帧更新 / 绘制 ----------

func (m *Module) Update(t, beat float64) {
	alive := m.objs[:0]
	for _, o := range m.objs {
		if o.falling {
			dt := t - o.lastT
			o.lastT = t
			o.fallV += o.gravity * dt
			o.fallPos[1] -= o.fallV * dt
		}
		flyPos := (beat - o.startBeat) / o.flyBeats
		if o.flyBeats <= 0 || (beat-o.startBeat) > o.flyBeats+1 {
			continue // 销毁
		}
		_ = flyPos
		alive = append(alive, o)
	}
	m.objs = alive
}

func (m *Module) Draw(screen *ebiten.Image, t, beat float64) {
	screen.Fill(bgColor)
	m.ctx.Scene.Sample(beat)
	m.ctx.Scene.Draw(screen, m.proj)

	for _, o := range m.objs {
		var pos [3]float64
		switch {
		case o.falling:
			pos = o.fallPos
		default:
			flyPos := (beat - o.startBeat) / o.flyBeats
			if flyPos > 1 {
				flyPos = 1
			}
			if !o.missed {
				flyPos *= 0.95
			}
			pos = kart.EvalBezier(o.curve, flyPos)
			// 旋转：原版 MobTrickObj.Update 用世界系"上一帧位置差"求朝向
			//（transform.eulerAngles 是世界角，透视不改变绕 z 的精灵旋转）
			switch o.flyType {
			case 1: // 朝向运动方向（纸飞机）
				if o.hasLast {
					dx, dy := pos[0]-o.lastPos[0], pos[1]-o.lastPos[1]
					if dx*dx+dy*dy > 1e-12 {
						o.rot = math.Atan2(dy, dx)
					}
				}
			case 2: // 不旋转（闪电）
				o.rot = 0
			default: // 自转 360°/s
				o.rot += (1.0 / float64(ebiten.TPS())) * 2 * math.Pi
			}
			o.lastPos, o.hasLast = pos, true
		}
		// 透视：对象沿曲线从背景（女孩 z=16）飞向前景（男孩 z=0），近大远小
		ps := persp(pos[2])
		if ps <= 0 {
			continue
		}
		world := kart.Translate(pos[0]*ps, pos[1]*ps).
			Mul(kart.Rotate(o.rot)).
			Mul(kart.Scale(ps, ps))
		for _, n := range o.nodes {
			m.ctx.Assets.DrawSprite(screen, n.sprite, world.Mul(n.rel), m.proj, n.flipX, 1)
		}
	}
}

// persp 返回深度 z 处的透视缩放（GameCamera：s = D/(D+z)）。
func persp(z float64) float64 { return kart.CamDist / (kart.CamDist + z) }

func boolParam(e *riq.Entity, key string) bool {
	if v, ok := e.Data[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return false
}
