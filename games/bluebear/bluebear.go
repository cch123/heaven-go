// Package bluebear 是 Blue Bear（blueBear）的玩法模块，
// 逻辑对应 Assets/Scripts/Games/BlueBear/{BlueBear,Treat}.cs：
//
//	donut（右键，飞 2 拍，自转 -280°/拍）/ cake（左键，飞 3 拍，+280°/拍）
//	沿 _treatCurves 抛物线飞向嘴；命中咬下（Cry/Long 变体）+ 屑粒子 +
//	计数阈值脸颊碎屑；barely 弹飞；漏拍飞过。嘴部开合走 HeadAndBody
//	状态机的 ShouldOpenMouth bool（treat 在空中或 open 事件时张嘴）。
//	stretchEmotion 归一化情绪动画（OpenEyes/Sad/Smile 链）；story 回忆画。
package bluebear

import (
	"math"
	"sort"

	"github.com/hajimehoshi/ebiten/v2"

	"hsdemo/engine"
	"hsdemo/kart"
	"hsdemo/riq"
)

// 主题色 "b4e6f6"
var bgColor = [4]float64{0xb4 / 255.0, 0xe6 / 255.0, 0xf6 / 255.0, 1}

const rotSpeed = 280.0 // 度/拍

type treat struct {
	inst      *kart.Instance
	isCake    bool
	startBeat float64 // 路径起拍（barely 时重设）
	judgeBeat float64 // 原判定拍（销毁计时基准）
	flyBeats  float64
	hold      bool
	shouldOpn bool
	dead      bool
	// 路径（barely 时重指）
	p0, p1 [2]float64
	dur    float64
	height float64
}

type emotionEvt struct {
	beat, length float64
	typ          int
	instant      bool
}

type storyEvt struct {
	beat, length float64
	story        int
	enter        bool
}

type crumbEvt struct {
	beat        float64
	right, left int
	reset       bool
}

type particle struct {
	born   float64
	px, py float64
	vx, vy float64
	size   float64
	col    [4]float64
}

type Module struct {
	ctx  *engine.Ctx
	proj kart.Aff

	treats   []*treat
	donutT   *kart.Template
	cakeT    *kart.Template
	emotions []emotionEvt // stretch（非 instant）
	instants []emotionEvt
	stories  []storyEvt
	crumbs   []crumbEvt

	// 情绪驱动状态（UpdateEmotions）
	emoIdx         int
	emoCancelBeat  float64
	emoFirstFrame  bool
	crying         bool
	wantMouthOpen  bool
	openCount      int
	storyIdx       int
	squashing      bool
	eaten          int
	rightThreshold int
	leftThreshold  int

	donutPath, cakePath struct {
		p0, p1 [2]float64
		dur    float64
		height float64
	}
	donutGrad, cakeGrad [][4]float64

	parts []particle
}

func New() engine.Module {
	return &Module{emoCancelBeat: -1, emoFirstFrame: true, rightThreshold: 15, leftThreshold: 30}
}

func (m *Module) ID() string { return "blueBear" }

func (m *Module) Load(ctx *engine.Ctx) error {
	m.ctx = ctx
	if err := ctx.LoadAssets("blueBear"); err != nil {
		return err
	}
	m.proj = kart.Translate(engine.ScreenW/2, engine.ScreenH/2).Mul(kart.Scale(54, -54))

	game := ctx.Assets.Extra.Components["game"]
	for _, p := range game.Lists["_treatCurves"] {
		pos := p.Items["positions"]
		if len(pos) < 2 {
			continue
		}
		dst := &m.donutPath
		if p.Strs["name"] == "Cake" {
			dst = &m.cakePath
		}
		dst.p0 = [2]float64{pos[0].Nums["pos.x"], pos[0].Nums["pos.y"]}
		dst.p1 = [2]float64{pos[1].Nums["pos.x"], pos[1].Nums["pos.y"]}
		dst.dur = pos[0].Nums["duration"]
		dst.height = pos[0].Nums["height"]
	}
	for _, k := range game.Lists["donutGradient"] {
		m.donutGrad = append(m.donutGrad, [4]float64{k.Nums["r"], k.Nums["g"], k.Nums["b"], 1})
	}
	for _, k := range game.Lists["cakeGradient"] {
		m.cakeGrad = append(m.cakeGrad, [4]float64{k.Nums["r"], k.Nums["g"], k.Nums["b"], 1})
	}
	m.donutT = kart.NewTemplate(ctx.Assets, ctx.Role("donutBase"))
	m.cakeT = kart.NewTemplate(ctx.Assets, ctx.Role("cakeBase"))
	// 屑粒子方块（粒子渲染器默认材质 = 纯色方块）
	white := ebiten.NewImage(8, 8)
	white.Fill(colorRGBA{255, 255, 255, 255})
	ctx.Assets.RegisterSprite("__crumb", white, 100, 0.5, 0.5)
	return nil
}

// ---------- 事件 ----------

func (m *Module) OnEvent(e *riq.Entity) {
	b := e.Beat
	ctx := m.ctx
	switch e.Datamodel {
	case "blueBear/donut", "blueBear/cake":
		isCake := e.Datamodel == "blueBear/cake"
		long := boolParam(e, "long")
		open := true
		if _, has := e.Data["open"]; has {
			open = boolParam(e, "open")
		}
		// TreatSound（preFunction：事件拍出声）
		ctx.At(b, func() {
			if isCake {
				ctx.Sound("cake")
			} else {
				ctx.Sound("donut")
			}
			m.spawnTreat(b, isCake, long, open)
		})
		m.scheduleTreatInput(b, isCake, long)
	case "blueBear/stretchEmotion":
		typ := int(e.Float("type", -1))
		instant := boolParam(e, "instant") || typ == -1 || typ == 3 || typ == 4 || typ == 5
		ev := emotionEvt{b, e.Length, typ, instant}
		if instant {
			m.instants = append(m.instants, ev)
			ctx.At(b, func() { m.setEmotion(b, typ) })
		} else {
			m.emotions = append(m.emotions, ev)
		}
	case "blueBear/setEmotion": // legacy 数据模型（旧谱面）：即时设置
		typ := int(e.Float("type", -1))
		m.instants = append(m.instants, emotionEvt{b, e.Length, typ, true})
		ctx.At(b, func() { m.setEmotion(b, typ) })
	case "blueBear/wind":
		ctx.At(b, func() { ctx.Scene.PlayState("Wind", "Wind", b, 0.5) })
	case "blueBear/sigh":
		ctx.At(b, func() { m.bearState("Sigh", b) })
	case "blueBear/open":
		ctx.At(b, func() { m.wantMouthOpen = true })
	case "blueBear/story":
		m.stories = append(m.stories, storyEvt{
			b, e.Length, int(e.Float("story", 0)), boolParam(e, "enter"),
		})
	case "blueBear/crumb":
		m.crumbs = append(m.crumbs, crumbEvt{
			b, int(e.Float("right", 15)), int(e.Float("left", 30)), boolParam(e, "reset"),
		})
		ctx.At(b, func() {
			c := m.crumbs[0]
			for _, cc := range m.crumbs {
				if cc.beat <= b {
					c = cc
				}
			}
			m.rightThreshold, m.leftThreshold = c.right, c.left
			if c.reset {
				m.eaten = 0
			}
			m.updateCrumbs()
		})
	}
}

func (m *Module) Ready() {
	sort.Slice(m.emotions, func(i, j int) bool { return m.emotions[i].beat < m.emotions[j].beat })
	sort.Slice(m.stories, func(i, j int) bool { return m.stories[i].beat < m.stories[j].beat })
}

// ---------- treat ----------

func (m *Module) spawnTreat(b float64, isCake, long, open bool) {
	t := &treat{
		isCake: isCake, startBeat: b, judgeBeat: b, hold: long, shouldOpn: open,
	}
	if isCake {
		t.inst = m.cakeT.NewInstance()
		t.flyBeats = 3
		t.p0, t.p1, t.dur, t.height = m.cakePath.p0, m.cakePath.p1, m.cakePath.dur, m.cakePath.height
	} else {
		t.inst = m.donutT.NewInstance()
		t.flyBeats = 2
		t.p0, t.p1, t.dur, t.height = m.donutPath.p0, m.donutPath.p1, m.donutPath.dur, m.donutPath.height
	}
	if open {
		m.openCount++
	}
	m.treats = append(m.treats, t)
	m.squashBag(isCake)
}

func (m *Module) scheduleTreatInput(b float64, isCake, long bool) {
	ctx := m.ctx
	flyBeats := 2.0
	action := 0 // donut：右键（主键）
	if isCake {
		flyBeats = 3
		action = 1 // cake：左键（副键）
	}
	ctx.ScheduleInputAction(b+flyBeats, action, func(state float64, _ engine.Judgment) {
		t := m.findTreat(b, isCake)
		if t == nil {
			return
		}
		beat := ctx.Beat()
		if state >= 1 || state <= -1 { // barely：咬空弹飞
			ctx.PlayCommon("miss")
			m.bearFrozenBite(isCake)
			cur := m.treatPos(t, beat)
			t.p0 = cur
			t.height = 4
			t.dur = 1
			dx := 1.5
			if isCake {
				dx = -1.5
			}
			t.p1 = [2]float64{cur[0] + dx, cur[1] - 6}
			t.startBeat = beat
			return
		}
		// 命中：咬下
		if isCake {
			ctx.Sound("chompCake")
		} else {
			ctx.Sound("chompDonut")
		}
		m.bite(beat, isCake, t.hold)
		m.eaten++
		m.updateCrumbs()
		m.spawnCrumbParticles(t, beat)
		m.killTreat(t)
	}, func() {
		// 漏拍：treat 飞过（Out 为空），flyPos>2 时销毁
	})
}

func (m *Module) findTreat(b float64, isCake bool) *treat {
	for _, t := range m.treats {
		if !t.dead && t.judgeBeat == b && t.isCake == isCake {
			return t
		}
	}
	return nil
}

func (m *Module) killTreat(t *treat) {
	t.dead = true
	if t.shouldOpn {
		m.openCount--
	}
}

func (m *Module) treatPos(t *treat, beat float64) [2]float64 {
	tt := 0.0
	if t.dur > 0 {
		tt = (beat - t.startBeat) / t.dur
	}
	if tt < 0 {
		tt = 0
	}
	x := t.p0[0] + (t.p1[0]-t.p0[0])*tt
	y := t.p0[1] + (t.p1[1]-t.p0[1])*tt
	yMul := tt*2 - 1
	y += (1 - yMul*yMul) * t.height
	return [2]float64{x, y}
}

// ---------- 咬合 / 情绪 ----------

func (m *Module) bearState(state string, beat float64) {
	m.ctx.Scene.PlayState("Bear/HeadAndBody", state, beat, 0.5)
}

func (m *Module) bite(beat float64, left, long bool) {
	m.emoCancelBeat = beat
	m.wantMouthOpen = false
	name := ""
	if long {
		name = "Long"
	}
	if m.crying {
		name += "Cry"
	}
	name += "Bite"
	if left {
		name += "L"
	} else {
		name += "R"
	}
	m.bearState(name, beat)
}

// bearFrozenBite：barely 的咬空（DoScaledAnimationAsync(name, 0) = 冻结首帧）。
func (m *Module) bearFrozenBite(left bool) {
	name := "BiteR"
	if left {
		name = "BiteL"
	}
	m.ctx.Scene.PlayFrozen("Bear/HeadAndBody", name, 0)
}

func (m *Module) setEmotion(beat float64, typ int) {
	m.emoCancelBeat = beat
	m.wantMouthOpen = false
	m.crying = false
	switch typ {
	case -1:
		m.bearState("Idle", beat)
	case 3:
		m.bearState("EyesClosed", beat)
	case 5:
		m.bearState("CryIdle", beat)
		m.crying = true
	case 4:
		m.bearState("SmileIdle", beat)
	}
}

func (m *Module) updateCrumbs() {
	sc := m.ctx.Scene
	sc.SetActive(m.ctx.Role("leftCrumb"), m.eaten >= m.leftThreshold)
	sc.SetActive(m.ctx.Role("rightCrumb"), m.eaten >= m.rightThreshold)
}

// updateEmotions 对应 UpdateEmotions（归一化情绪动画 + 链推进）。
func (m *Module) updateEmotions(beat float64) {
	for {
		if len(m.emotions) == 0 || m.emoIdx >= len(m.emotions) {
			return
		}
		e := m.emotions[m.emoIdx]
		if beat > e.beat+e.length {
			m.emoIdx++
			m.crying = e.typ == 2
			m.emoFirstFrame = true
			if e.typ == 1 { // Smile 结束 → StopSmile
				m.bearState("StopSmile", beat)
			}
			continue
		}
		if beat >= e.beat && beat < e.beat+e.length && m.emoCancelBeat < e.beat {
			m.crying = e.typ == 2
			if m.emoFirstFrame && e.typ == 1 {
				m.bearState("Smile", beat)
				m.emoFirstFrame = false
			}
			norm := clamp01((beat - e.beat) / e.length)
			anim := ""
			switch e.typ {
			case 0:
				anim = "OpenEyes"
			case 2:
				anim = "Sad"
			}
			if anim != "" {
				m.ctx.Scene.PlayNormalized("Bear/HeadAndBody", anim, norm)
			}
		}
		return
	}
}

// updateStory 对应 UpdateStory（回忆画归一化动画）。
func (m *Module) updateStory(beat float64) {
	for {
		if m.storyIdx >= len(m.stories) {
			return
		}
		s := m.stories[m.storyIdx]
		if beat >= s.beat+s.length && m.storyIdx+1 != len(m.stories) {
			m.storyIdx++
			continue
		}
		norm := clamp01((beat - s.beat) / s.length)
		names := []string{"Flashback0", "Flashback1", "Flashback2", "Flashback3", "Breakup"}
		name := names[4]
		if s.story >= 0 && s.story < 4 {
			name = names[s.story]
		}
		if !s.enter {
			name += "Exit"
		}
		if beat >= s.beat {
			m.ctx.Scene.PlayNormalized("Story", name, norm)
		}
		return
	}
}

func (m *Module) squashBag(isCake bool) {
	sc := m.ctx.Scene
	beat := m.ctx.Beat()
	m.squashing = true
	sc.PlayState("Bear/HeadAndBody/BagHolder2/BagHolder", "Squashing", beat, 0.5)
	sc.SetActive(m.ctx.Role("individualBagHolder"), true)
	if isCake {
		sc.PlayState("Bear/HeadAndBody/BagHolder2/BagHolder/Bags_Individual/CakeBag", "CakeSquash", beat, 0.5)
	} else {
		sc.PlayState("Bear/HeadAndBody/BagHolder2/BagHolder/Bags_Individual/DonutBag", "DonutSquash", beat, 0.5)
	}
}

// spawnCrumbParticles：Prefabs/Crumbs PS（burst 8、初速 4、重力 1.5g、
// 尺寸 0.25..0.5、颜色取 treat 渐变随机键）。
func (m *Module) spawnCrumbParticles(t *treat, beat float64) {
	grad := m.donutGrad
	if t.isCake {
		grad = m.cakeGrad
	}
	if len(grad) == 0 {
		grad = [][4]float64{{1, 1, 1, 1}}
	}
	holder, _ := m.ctx.Scene.NodeWorld(m.ctx.Role("foodHolder"))
	pos := m.treatPos(t, beat)
	wx, wy := holder.Tx+pos[0], holder.Ty+pos[1]
	now := m.ctx.Time()
	for i := 0; i < 8; i++ {
		ang := randF() * 2 * math.Pi
		spd := 4 * (0.5 + randF()*0.5)
		m.parts = append(m.parts, particle{
			born: now, px: wx, py: wy,
			vx: math.Cos(ang) * spd, vy: math.Sin(ang) * spd,
			size: 0.25 + randF()*0.25,
			col:  grad[int(randF()*float64(len(grad)))%len(grad)],
		})
	}
}

var rngState uint64 = 0x9e3779b97f4a7c15

func randF() float64 {
	rngState ^= rngState << 13
	rngState ^= rngState >> 7
	rngState ^= rngState << 17
	return float64(rngState%1e9) / 1e9
}

// ---------- 生命周期 ----------

func (m *Module) OnSwitch(beat float64) {
	sc := m.ctx.Scene
	sec := m.ctx.SecPerBeat(beat)
	for _, p := range []string{
		"Bear/HeadAndBody", "Bear/HeadAndBody/BagHolder2/BagHolder", "BackgroundScene",
		"Bear/HeadAndBody/Head/FaceCrumbs/Left", "Bear/HeadAndBody/Head/FaceCrumbs/Right",
	} {
		sc.PlayDefaultState(p, beat, sec)
	}
	m.updateCrumbs()
}

func (m *Module) Whiff(beat float64) { m.WhiffAction(beat, 0) }

// WhiffAction：空击（左/右口）——whiff 音随机变调 ±1 半音 + 咬空。
func (m *Module) WhiffAction(beat float64, action int) {
	st := float64(int(randF()*3) - 1) // -1..1 半音
	m.ctx.SoundPitch("whiff", 1, math.Exp2(st/12))
	m.bite(beat, action == 1, false)
}

func (m *Module) Update(t, beat float64) {
	// 嘴部开合 bool（Update 每帧）
	m.ctx.Scene.SetBool("Bear/HeadAndBody", "ShouldOpenMouth", m.openCount != 0 || m.wantMouthOpen)
	if m.openCount != 0 || m.wantMouthOpen {
		m.emoCancelBeat = beat
	}

	m.updateEmotions(beat)
	m.updateStory(beat)

	// 包袋复位（LateUpdate）
	if m.squashing {
		d, _ := m.ctx.Scene.StateInfo("Bear/HeadAndBody/BagHolder2/BagHolder/Bags_Individual/DonutBag", beat)
		c, _ := m.ctx.Scene.StateInfo("Bear/HeadAndBody/BagHolder2/BagHolder/Bags_Individual/CakeBag", beat)
		if (d == "DonutIdle" || d == "") && (c == "CakeIdle" || c == "") {
			m.squashing = false
			m.ctx.Scene.PlayState("Bear/HeadAndBody/BagHolder2/BagHolder", "Idle", beat, 0.5)
		}
	}

	// treat 飞行与销毁（flyPos > 2）
	alive := m.treats[:0]
	for _, tr := range m.treats {
		if tr.dead {
			continue
		}
		if (beat-tr.judgeBeat)/tr.flyBeats > 2 {
			m.killTreat(tr)
			continue
		}
		alive = append(alive, tr)
	}
	m.treats = alive

	// 粒子清理
	keep := m.parts[:0]
	for _, p := range m.parts {
		if t-p.born <= 5 {
			keep = append(keep, p)
		}
	}
	m.parts = keep
}

func (m *Module) Draw(screen *ebiten.Image, t, beat float64) {
	screen.Fill(toRGBA(bgColor))
	sc := m.ctx.Scene
	m.ctx.SampleScene(beat)

	// treats（foodHolder 本地空间 + 自转）
	holder, _ := sc.NodeWorld(m.ctx.Role("foodHolder"))
	for _, tr := range m.treats {
		pos := m.treatPos(tr, beat)
		rot := -rotSpeed
		if tr.isCake {
			rot = rotSpeed
		}
		tr.inst.Offset = pos
		tr.inst.Rot = rot * (beat - tr.judgeBeat) * math.Pi / 180
		tr.inst.Queue(sc, beat, holder, 0)
	}

	// 屑粒子
	for _, p := range m.parts {
		age := t - p.born
		x := p.px + p.vx*age
		y := p.py + p.vy*age - 0.5*1.5*9.81*age*age
		w := kart.Translate(x, y).Mul(kart.Scale(p.size, p.size))
		sc.Queue(kart.ExtraSprite{Sprite: "__crumb", World: w, Order: 60, Tint: p.col})
	}

	sc.Draw(screen, m.proj)
}

func toRGBA(c [4]float64) (out colorRGBA) {
	return colorRGBA{uint8(c[0] * 255), uint8(c[1] * 255), uint8(c[2] * 255), uint8(c[3] * 255)}
}

type colorRGBA struct{ R, G, B, A uint8 }

func (c colorRGBA) RGBA() (r, g, b, a uint32) {
	return uint32(c.R) * 0x101, uint32(c.G) * 0x101, uint32(c.B) * 0x101, uint32(c.A) * 0x101
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
