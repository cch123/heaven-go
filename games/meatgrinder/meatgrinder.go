// Package meatgrinder 是 Meat Grinder（meatGrinder）的玩法模块，
// 逻辑逐行对应 Assets/Scripts/Games/MeatGrinder/{MeatGrinder,Meat}.cs：
//
//	MeatToss     B 抛肉（toss 音效 + 生成 dark/bacon 肉块），判定 B+1
//	*Interval    B-1 startSignal + BossSignal；区间内每个 MeatCall：signal + BossCall
//	             auto pass：区间结束后按相对拍回声生成 light 肉块（判定差一个区间长）
//	passTurn     手动过回合（只认 StartInterval 事件，C# 同语义）
//	命中：肉块换 Hit 剪辑滑进绞肉机 + Tack 锤击 + 肉末粒子；barely（NG 命中）= tink
//	miss：Tack 满脸肉（tackMeated 状态机循环）+ Boss 不悦；whiff：空锤
//	cartGuy      底部推车人按缓动横移（仅移动期间可见，每拍 bop）
//	gears        背景齿轮转速时间轴（Big 反向，速度缓动）
//	changeText   绞肉机铭牌 TMP 文本
package meatgrinder

import (
	"image/color"
	"math"
	"math/rand"
	"sort"

	"github.com/hajimehoshi/ebiten/v2"

	"hsdemo/engine"
	"hsdemo/kart"
	"hsdemo/riq"
)

// 主题色（Minigame 注册色 "501d18"）
var bgColor = color.RGBA{0x50, 0x1d, 0x18, 255}

// 肉块类型（Meat.MeatType 同值）
const (
	typDark = iota
	typLight
	typBacon
)

var typName = [...]string{"DarkMeat", "LightMeat", "BaconBall"}

// meat 是一个飞行中/被击中的肉块实例（Instantiate(MeatBase) 等价物）。
type meat struct {
	startBeat float64 // 判定在 startBeat+1（ScheduleInput(startBeat, 1)）
	typ       int
	hit       bool
	hitBeat   float64
	dead      bool

	rot     float64
	lastX   float64
	lastY   float64
	hasLast bool
}

type bopEvt struct {
	beat, length float64
	bop, auto    bool
}

type cartEvt struct {
	beat, length float64
	phone        bool
	dir          string // "Right"/"Left"
	ease         int
}

type gearEvt struct {
	beat, length float64
	ease         int
	speed        float64
}

type intervalEvt struct {
	beat, length float64
	auto         bool
}

type callEvt struct {
	beat                           float64
	tackReact, bossReact           int
	tackReactBeats, bossReactBeats float64
}

// 肉末粒子（MeatSplash ParticleSystem 的手写等价物，prefab 序列化参数：
// burst 5 颗、初速 5、重力系数 4、生存 0.45s、尺寸 0.5、球形发射半径 1、
// 贴图表 MeatGrinder_0..8 按肉类型取 1/3 区间、sortingOrder 1000；
// simulationSpeed 在每次命中时设为 0.5/secPerBeat）
type particle struct {
	born       float64 // 出生时的模拟时钟（sim 秒）
	px, py, pz float64
	vx, vy, vz float64
	sprite     string
}

const (
	splashLife  = 0.45
	splashSpeed = 5.0
	splashGrav  = 4 * 9.81
	splashSize  = 0.5
	splashCount = 5
)

type Module struct {
	ctx  *engine.Ctx
	proj kart.Aff

	meats     []*meat
	bops      []bopEvt
	carts     []cartEvt
	gearEvts  []gearEvt
	intervals []intervalEvt
	calls     []callEvt
	passes    []float64 // 手动 passTurn 拍
	endBeat   float64

	bossAnnoyed bool

	// 齿轮：transform.Rotate 的积分（度），Big 取负（C# gear.name=="Big" ? -1 : 1）
	gearAngle float64
	gearIdx   []int
	gearSign  []float64

	// 粒子模拟时钟
	simT     float64
	simSpeed float64
	parts    []particle

	lastT    float64
	hasLastT bool

	// Meat.cs 序列化参数
	flyH, flyHAlt float64
	meatScale     [2]float64
	meatSprites   []string // meats[]（飞行外观：DarkMeat_Smear 等）
	startPath     string
	startAltPath  string
	hitPath       string
	missPath      string
}

func New() engine.Module { return &Module{simSpeed: 1} }

func (m *Module) ID() string { return "meatGrinder" }

func (m *Module) Load(ctx *engine.Ctx) error {
	m.ctx = ctx
	if err := ctx.LoadAssets("meatGrinder"); err != nil {
		return err
	}
	if err := ctx.Assets.ApplyTexts(); err != nil { // GRINDER 铭牌
		return err
	}
	m.proj = kart.Translate(engine.ScreenW/2, engine.ScreenH/2).Mul(kart.Scale(54, -54))

	ex := &ctx.Assets.Extra
	tmpl := ctx.Role("MeatBase") // "MeatHolder/Meat"
	m.flyH = ex.ObjNums[tmpl]["meatFlyHeight"]
	m.flyHAlt = ex.ObjNums[tmpl]["meatFlyHeightAlt"]
	m.meatSprites = ex.ObjSprites[tmpl]["meats"]
	refs := ex.ObjRefs[tmpl]
	m.startPath, m.startAltPath = refs["startPosition"], refs["startPositionAlt"]
	m.hitPath, m.missPath = refs["hitPosition"], refs["missPosition"]
	if i, ok := ctx.Assets.NodeIndex(tmpl); ok {
		m.meatScale = ctx.Assets.Rig.Nodes[i].Scale
	}
	for _, gi := range ex.RefArrayIdx["Gears"] {
		m.gearIdx = append(m.gearIdx, gi)
		sign := 1.0
		if ctx.Assets.Rig.Nodes[gi].Name == "Big" {
			sign = -1
		}
		m.gearSign = append(m.gearSign, sign)
	}
	return nil
}

func (m *Module) OnSwitch(beat float64) {
	// Animator 激活即按真实秒速播放 controller 默认状态（Idle 循环）
	sec := m.ctx.SecPerBeat(beat)
	sc := m.ctx.Scene
	sc.PlayDefaultState(m.ctx.Role("BossAnim"), beat, sec)
	sc.PlayDefaultState(m.ctx.Role("TackAnim"), beat, sec)
	sc.PlayDefaultState(m.ctx.Role("CartGuyAnim"), beat, sec)
	sc.PlayDefaultState(m.ctx.Role("CartGuyParentAnim"), beat, sec)
}

// ---------- 事件收集 ----------

func (m *Module) OnEvent(e *riq.Entity) {
	b := e.Beat
	if end := b + e.Length; end > m.endBeat {
		m.endBeat = end
	}
	switch e.Datamodel {
	case "meatGrinder/bop":
		m.bops = append(m.bops, bopEvt{b, e.Length, boolParam(e, "bop"), boolParam(e, "bossBop")})
	case "meatGrinder/MeatToss":
		typ := typDark
		if boolParam(e, "bacon") {
			typ = typBacon
		}
		m.ctx.At(b, func() { m.ctx.Sound("toss") })
		m.spawnMeat(b, typ, reactionsOf(e))
	case "meatGrinder/SimpleInterval", "meatGrinder/StartInterval":
		m.intervals = append(m.intervals, intervalEvt{b, e.Length, boolParam(e, "auto")})
	case "meatGrinder/MeatCall":
		c := callEvt{beat: b}
		c.tackReact, c.tackReactBeats, c.bossReact, c.bossReactBeats = reactionsOf(e).unpack()
		m.calls = append(m.calls, c)
	case "meatGrinder/passTurn":
		m.passes = append(m.passes, b)
	case "meatGrinder/expressions":
		tack, boss := int(e.Float("tackExpression", 1)), int(e.Float("bossExpression", 0))
		m.ctx.At(b, func() { m.doExpressions(b, tack, boss) })
	case "meatGrinder/cartGuy":
		dir := "Right"
		if int(e.Float("direction", 0)) == 1 {
			dir = "Left"
		}
		ce := cartEvt{b, e.Length, boolParam(e, "spider"), dir, int(e.Float("ease", 0))}
		m.carts = append(m.carts, ce)
		if ce.phone { // C#: CartGuyAnim.Play("Phone")（真实秒速循环）
			m.ctx.At(b, func() {
				m.ctx.Scene.PlayState(m.ctx.Role("CartGuyAnim"), "Phone", b, m.ctx.SecPerBeat(b))
			})
		}
	case "meatGrinder/gears":
		m.gearEvts = append(m.gearEvts, gearEvt{b, e.Length, int(e.Float("ease", 0)), e.Float("speed", 1)})
	case "meatGrinder/changeText":
		txt := e.Str("text", "GRINDER")
		m.ctx.At(b, func() {
			if err := m.ctx.Assets.SetText(m.ctx.Role("GrinderText"), txt); err != nil {
				// 文本节点缺失属资产问题，加载期已验证；运行期失败仅跳过
				_ = err
			}
		})
	}
}

type reactions struct {
	tack, boss           int
	tackBeats, bossBeats float64
}

func (r reactions) unpack() (int, float64, int, float64) {
	return r.tack, r.tackBeats, r.boss, r.bossBeats
}

func reactionsOf(e *riq.Entity) reactions {
	return reactions{
		tack: int(e.Float("tackReaction", 0)), tackBeats: e.Float("tackReactionBeats", 1),
		boss: int(e.Float("bossReaction", 0)), bossBeats: e.Float("bossReactionBeats", 0),
	}
}

// ---------- 调度（Ready：全量事件就绪后） ----------

func (m *Module) Ready() {
	sort.Slice(m.calls, func(i, j int) bool { return m.calls[i].beat < m.calls[j].beat })
	sort.Slice(m.gearEvts, func(i, j int) bool { return m.gearEvts[i].beat < m.gearEvts[j].beat })
	sort.Slice(m.carts, func(i, j int) bool { return m.carts[i].beat < m.carts[j].beat })
	sort.Slice(m.bops, func(i, j int) bool { return m.bops[i].beat < m.bops[j].beat })

	boss := m.ctx.Role("BossAnim")

	// interval：B-1 startSignal + BossSignal；区间内 call：signal + BossCall；auto pass
	for _, iv := range m.intervals {
		iv := iv
		m.ctx.SoundAt(iv.beat-1, "startSignal", 1)
		m.ctx.At(iv.beat-1, func() {
			// DoScaledAnimationFromBeatAsync("BossSignal", 0.5, beat-1)
			m.ctx.Scene.PlayState(boss, "BossSignal", iv.beat-1, 0.5)
		})
		calls := m.callsIn(iv.beat, iv.beat+iv.length)
		for _, c := range calls {
			c := c
			m.ctx.SoundAt(c.beat, "signal", 1)
			m.ctx.At(c.beat, func() {
				m.ctx.Scene.PlayState(boss, "BossCall", c.beat, 0.5)
			})
		}
		if iv.auto {
			m.passTurn(iv.beat+iv.length, iv)
		}
	}
	// 手动 passTurn：只认 StartInterval（C# GetAllInGameManagerList(["StartInterval"])；
	// 本移植 SimpleInterval/StartInterval 已合并存储，语义一致：取最近的区间）
	for _, pb := range m.passes {
		var last *intervalEvt
		for i := range m.intervals {
			if m.intervals[i].beat <= pb {
				last = &m.intervals[i]
			}
		}
		if last != nil {
			m.passTurn(pb, *last)
		}
	}

	// 每拍脉冲（OnLateBeatPulse 等价物）：boss 自动 bop（区间内）+ 推车人 bop
	for b := 0.0; b <= m.endBeat; b++ {
		b := b
		m.ctx.At(b, func() {
			if m.inBopRegion(b) {
				m.bossBop(b, true)
			}
			m.cartBop(b)
		})
	}
	// 手动 bop（Bop()：排除 BossCall/BossSignal，不排除 BossScared）
	for _, bp := range m.bops {
		if !bp.bop {
			continue
		}
		for i := 0.0; i < bp.length; i++ {
			bb := bp.beat + i
			m.ctx.At(bb, func() { m.bossBop(bb, false) })
		}
	}
}

func (m *Module) callsIn(from, to float64) []callEvt {
	var out []callEvt
	for _, c := range m.calls {
		if c.beat >= from && c.beat < to {
			out = append(out, c)
		}
	}
	return out
}

// passTurn 对应 C# PassTurn：每个 call 在 passBeat+rel-1 生成 light 肉块（判定 +1）。
func (m *Module) passTurn(passBeat float64, iv intervalEvt) {
	for _, c := range m.callsIn(iv.beat, iv.beat+iv.length) {
		rel := c.beat - iv.beat
		m.spawnMeat(passBeat+rel-1, typLight, reactions{
			tack: c.tackReact, tackBeats: c.tackReactBeats,
			boss: c.bossReact, bossBeats: c.bossReactBeats,
		})
	}
}

// inBopRegion 对应 SetupBopRegion("meatGrinder","bop","bossBop")：
// bop 事件的 bossBop 参数划定自动 bop 区间（首个事件前默认关）。
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

// bossBop：auto=OnLateBeatPulse（排除 BossCall/BossSignal/BossScared），
// manual=Bop()（只排除前两者）；bossAnnoyed 时播 BossMiss。
func (m *Module) bossBop(beat float64, auto bool) {
	boss := m.ctx.Role("BossAnim")
	st, playing := m.ctx.Scene.StateInfo(boss, beat)
	if playing && (st == "BossCall" || st == "BossSignal" || (auto && st == "BossScared")) {
		return
	}
	anim := "Bop"
	if m.bossAnnoyed {
		anim = "BossMiss"
	}
	m.ctx.Scene.PlayState(boss, anim, beat, 0.5)
}

func (m *Module) cartBop(beat float64) {
	active, evt, _ := m.cartAt(beat)
	if !active {
		return
	}
	anim := "Bop"
	if evt.phone {
		anim = "PhoneBop"
	}
	m.ctx.Scene.PlayState(m.ctx.Role("CartGuyAnim"), anim, beat, 0.5)
}

func (m *Module) doExpressions(beat float64, tack, boss int) {
	tackNames := [...]string{"", "TackContent", "TackSmug", "TackWonder"}
	bossNames := [...]string{"", "BossEyebrow", "BossScared"}
	if tack > 0 && tack < len(tackNames) {
		m.ctx.Scene.PlayState(m.ctx.Role("TackAnim"), tackNames[tack], beat, 0.5)
	}
	if boss > 0 && boss < len(bossNames) {
		m.ctx.Scene.PlayState(m.ctx.Role("BossAnim"), bossNames[boss], beat, 0.5)
	}
}

// ---------- 肉块 ----------

func (m *Module) spawnMeat(spawnBeat float64, typ int, r reactions) {
	ctx := m.ctx
	// C# inactiveFunction 语义：游戏未激活时 MeatToss 只放音效，不生成肉块/判定
	//（QueueMeatToss 在原版即为空操作）
	if ctx.GameAt(spawnBeat+1) != "meatGrinder" {
		return
	}
	var mt *meat
	ctx.At(spawnBeat, func() {
		mt = &meat{startBeat: spawnBeat, typ: typ}
		m.meats = append(m.meats, mt)
	})
	tack, boss := ctx.Role("TackAnim"), ctx.Role("BossAnim")
	ctx.ScheduleInput(spawnBeat+1, func(state float64, j engine.Judgment) {
		if mt == nil || mt.dead {
			return
		}
		beat := ctx.Beat()
		isBarely := j == engine.JudgeNG // state >= 1f or <= -1f
		mt.hit, mt.hitBeat = true, beat

		ctx.Scene.SetBool(tack, "tackMeated", false)
		m.bossAnnoyed = isBarely
		if isBarely {
			ctx.Sound("tink")
			ctx.Scene.PlayState(tack, "TackHitBarely", beat, 0.5)
		} else {
			ctx.Sound("meathit") // C# "meatHit"，资产文件名小写
			ctx.Scene.PlayState(tack, "TackHitSuccess", beat, 0.5)
		}
		m.emitSplash(typ, beat)
		if r.tack > 0 {
			ctx.At(spawnBeat+r.tackBeats+1, func() { m.doExpressions(ctx.Beat(), r.tack, 0) })
		}
		if r.boss > 0 {
			ctx.At(spawnBeat+r.bossBeats+1, func() { m.doExpressions(ctx.Beat(), 0, r.boss) })
		}
	}, func() {
		if mt == nil || mt.dead {
			return
		}
		beat := ctx.Beat()
		m.bossAnnoyed = true
		ctx.Sound("miss")
		// TackMissDarkMeat/.. 剪辑结束后由状态机按 tackMeated 进入满脸肉循环
		ctx.Scene.PlayState(tack, "TackMiss"+typName[mt.typ], beat, 0.5)
		ctx.Scene.SetBool(tack, "tackMeated", true)
		ctx.Scene.PlayState(boss, "BossMiss", beat, 0.5)
		mt.dead = true // Destroy(gameObject)
	})
}

// Whiff 对应 Update 的非预期按键分支。
func (m *Module) Whiff(beat float64) {
	tack := m.ctx.Role("TackAnim")
	m.ctx.Scene.PlayState(tack, "TackEmptyHit", beat, 0.5)
	m.ctx.Scene.SetBool(tack, "tackMeated", false)
	m.ctx.Sound("whiff")
	m.bossAnnoyed = false
}

// ---------- 粒子 ----------

var splashRange = [3][2]float64{{0, 0.333}, {0.334, 0.666}, {0.667, 1}}

func (m *Module) emitSplash(typ int, beat float64) {
	// main.simulationSpeed = 0.5 / secPerBeat（影响整个系统，含已存活粒子）
	m.simSpeed = 0.5 / m.ctx.SecPerBeat(beat)
	world, ok := m.ctx.Scene.NodeWorld("MeatSplash")
	if !ok {
		return
	}
	lo, hi := splashRange[typ][0], splashRange[typ][1]
	for i := 0; i < splashCount; i++ {
		// 球形发射（radius 1、radiusThickness 1 = 全体积）：
		// 方向 = 自球心的径向，速度 5
		dx, dy, dz := randUnit3()
		r := math.Cbrt(rand.Float64())
		frame := lo + rand.Float64()*(hi-lo)
		fi := int(frame * 9)
		if fi > 8 {
			fi = 8
		}
		m.parts = append(m.parts, particle{
			born: m.simT,
			px:   world.Tx + dx*r, py: world.Ty + dy*r, pz: dz * r,
			vx: dx * splashSpeed, vy: dy * splashSpeed, vz: dz * splashSpeed,
			sprite: "MeatGrinder_" + string(rune('0'+fi)),
		})
	}
}

func randUnit3() (float64, float64, float64) {
	for {
		x, y, z := rand.Float64()*2-1, rand.Float64()*2-1, rand.Float64()*2-1
		if d := x*x + y*y + z*z; d > 1e-6 && d <= 1 {
			n := math.Sqrt(d)
			return x / n, y / n, z / n
		}
	}
}

// ---------- 缓动（Util.EasingFunction，level 用到 Linear=0 / Instant=1） ----------

func ease(kind int, start, end, v float64) float64 {
	if v < 0 {
		v = 0
	}
	if v > 1 {
		v = 1
	}
	switch kind {
	case 1: // Instant：恒为终值
		return end
	case 2: // EaseInQuad
		return start + (end-start)*v*v
	case 3: // EaseOutQuad
		return start + (end-start)*(1-(1-v)*(1-v))
	case 4: // EaseInOutQuad
		if v < 0.5 {
			return start + (end-start)*2*v*v
		}
		return start + (end-start)*(1-2*(1-v)*(1-v))
	default: // Linear（未实现的罕见缓动按线性处理并不静默：见 README 已知简化）
		return start + (end-start)*v
	}
}

// gearSpeedAt 重建 ChangeGears 速度时间轴（初始 newGearSpeed=1；
// 每个事件从上一事件目标值缓动到自身 speed）。
func (m *Module) gearSpeedAt(beat float64) float64 {
	prev, cur := 1.0, 1.0
	for _, e := range m.gearEvts {
		if beat < e.beat {
			break
		}
		norm := 1.0
		if e.length > 0 {
			norm = (beat - e.beat) / e.length
		}
		cur = ease(e.ease, prev, e.speed, norm)
		prev = e.speed
	}
	return cur
}

// cartAt 返回当前生效的 cartGuy 事件（C#：事件 fire 时设 cartEase，
// normalized>=1 后 length 置 0 → 推车隐藏）。
func (m *Module) cartAt(beat float64) (bool, *cartEvt, float64) {
	var evt *cartEvt
	for i := range m.carts {
		if m.carts[i].beat <= beat {
			evt = &m.carts[i]
		}
	}
	if evt == nil || evt.length <= 0 {
		return false, nil, 0
	}
	norm := (beat - evt.beat) / evt.length
	if norm >= 1 {
		return false, nil, 0
	}
	if norm < 0 {
		norm = 0
	}
	return true, evt, norm
}

// ---------- 帧更新 ----------

func (m *Module) Update(t, beat float64) {
	dt := 0.0
	if m.hasLastT && t > m.lastT {
		dt = t - m.lastT
	}
	m.lastT, m.hasLastT = t, true

	// 齿轮：deltaTime * speed * 50 / secPerBeat（度），Big 反向
	m.gearAngle += dt * m.gearSpeedAt(beat) * 50 / m.ctx.SecPerBeat(beat)
	for i, gi := range m.gearIdx {
		m.ctx.Scene.SetSpinIdx(gi, m.gearSign[i]*m.gearAngle*math.Pi/180)
	}

	// 推车人：仅移动期间可见，按缓动采样 Move 剪辑
	active, evt, norm := m.cartAt(beat)
	m.ctx.Scene.SetActive(m.ctx.Role("CartGuyParentAnim"), active)
	if active {
		v := ease(evt.ease, 0, 1, norm)
		m.ctx.Scene.PlayNormalized(m.ctx.Role("CartGuyParentAnim"), "Cart Guy/CartguyMove"+evt.dir, v)
	}

	// 粒子模拟时钟
	m.simT += dt * m.simSpeed
	alive := m.parts[:0]
	for _, p := range m.parts {
		if m.simT-p.born <= splashLife {
			alive = append(alive, p)
		}
	}
	m.parts = alive

	// 肉块清理：DestroySelf 动画事件（DarkMeat/LightMeat Hit 剪辑 0.3333s 处 =
	// 剪辑末尾；Bacon 无事件保持末帧，对齐 Unity）；miss 已即时 dead
	keep := m.meats[:0]
	for _, mt := range m.meats {
		if mt.dead {
			continue
		}
		if mt.hit && mt.typ != typBacon {
			clip := m.ctx.Assets.Anims["Meat/"+typName[mt.typ]+"Hit"]
			if clip != nil && (beat-mt.hitBeat)*0.5 >= clip.Duration {
				continue
			}
		}
		keep = append(keep, mt)
	}
	m.meats = keep
}

// ---------- 绘制 ----------

func (m *Module) Draw(screen *ebiten.Image, t, beat float64) {
	screen.Fill(bgColor)
	sc := m.ctx.Scene
	m.ctx.SampleScene(beat)

	m.queueMeats(t, beat)
	m.queueSplash()

	sc.Draw(screen, m.proj)
}

// queueMeats 计算肉块位姿并注入场景排序（SpriteRenderer order 25）。
func (m *Module) queueMeats(t, beat float64) {
	sc := m.ctx.Scene
	for _, mt := range m.meats {
		if mt.dead { // miss 回调与下一次 Update 清扫之间不再绘制（Destroy 即时生效）
			continue
		}
		var (
			px, py, rot float64
			sx, sy      = m.meatScale[0], m.meatScale[1]
			sprite      = m.meatSprites[mt.typ]
		)
		if mt.hit {
			// Hit 剪辑（ts=0.5）：曲线在根节点（path ""），局部 = 世界（父为场景根）
			clip := m.ctx.Assets.Anims["Meat/"+typName[mt.typ]+"Hit"]
			if clip == nil {
				continue
			}
			at := (beat - mt.hitBeat) * 0.5
			if at > clip.Duration {
				at = clip.Duration
			}
			pose := kart.SampleClipNode(clip, "", at)
			if pose.HasPos[0] {
				px = pose.Pos[0]
			}
			if pose.HasPos[1] {
				py = pose.Pos[1]
			}
			if pose.HasScale[0] {
				sx = pose.Scale[0]
			}
			if pose.HasScale[1] {
				sy = pose.Scale[1]
			}
			if pose.HasRot {
				rot = pose.RotDeg * math.Pi / 180
			}
			if pose.HasSprite && pose.Sprite != "" {
				sprite = pose.Sprite
			}
		} else {
			px, py = m.meatTrajectory(mt, t)
			// transform.right = 当前位置 - 上一帧位置（飞行朝向）
			if mt.hasLast {
				dx, dy := px-mt.lastX, py-mt.lastY
				if dx*dx+dy*dy > 1e-12 {
					mt.rot = math.Atan2(dy, dx)
				}
			}
			mt.lastX, mt.lastY, mt.hasLast = px, py, true
			rot = mt.rot
		}
		world := kart.Translate(px, py).Mul(kart.Rotate(rot)).Mul(kart.Scale(sx, sy))
		sc.Queue(kart.ExtraSprite{Sprite: sprite, World: world, Order: 25})
	}
}

// meatTrajectory 对应 Meat.Update：起点→（命中点在 miss 线上的投影）→ miss 点
// 的折线插值 + 抛物线高度（light 用 alt 起点与 alt 飞高）。
func (m *Module) meatTrajectory(mt *meat, t float64) (float64, float64) {
	sc := m.ctx.Scene
	startPath := m.startPath
	flyH := m.flyH
	if mt.typ == typLight {
		startPath, flyH = m.startAltPath, m.flyHAlt
	}
	sw, _ := sc.NodeWorld(startPath)
	hw, _ := sc.NodeWorld(m.hitPath)
	mw, _ := sc.NodeWorld(m.missPath)
	sx, sy := sw.Tx, sw.Ty
	hx, hy := hw.Tx, hw.Ty
	mx, my := mw.Tx, mw.Ty

	startT := m.ctx.BeatToTime(mt.startBeat)
	hitT := m.ctx.BeatToTime(mt.startBeat + 1)
	missT := hitT + engine.WinNG // MeatGrinder.ngLateTime

	// 命中点向 start→miss 线投影的参数
	smx, smy := sx-mx, sy-my
	shx, shy := sx-hx, sy-hy
	ratio := (smx*shx + smy*shy) / (smx*smx + smy*smy)
	homx, homy := sx+(mx-sx)*ratio, sy+(my-sy)*ratio

	var px, py, prog float64
	if t >= hitT {
		u := clamp01((t - hitT) / (missT - hitT))
		px, py = homx+(mx-homx)*u, homy+(my-homy)*u
		prog = u*(1-ratio) + ratio
	} else {
		u := clamp01((t - startT) / (hitT - startT))
		px, py = sx+(homx-sx)*u, sy+(homy-sy)*u
		prog = u * ratio
	}
	yMul := prog*2 - 1
	py += flyH * (1 - yMul*yMul)
	return px, py
}

func (m *Module) queueSplash() {
	for _, p := range m.parts {
		age := m.simT - p.born
		x := p.px + p.vx*age
		y := p.py + p.vy*age - 0.5*splashGrav*age*age
		z := p.pz + p.vz*age
		sp, ok := m.ctx.Assets.Sheet.Sprites[p.sprite]
		if !ok {
			continue
		}
		ppu := sp.PPU
		if ppu == 0 {
			ppu = m.ctx.Assets.Sheet.PPU
		}
		// billboard：size 0.5 的方形粒子（贴图拉伸充满，Unity 同语义）
		w := kart.Translate(x, y).Mul(kart.Scale(
			splashSize/(float64(sp.W)/ppu), splashSize/(float64(sp.H)/ppu)))
		m.ctx.Scene.Queue(kart.ExtraSprite{Sprite: p.sprite, World: w, Z: z, Order: 1000})
	}
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
