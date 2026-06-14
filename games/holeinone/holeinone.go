// Package holeinone ports Hole in One's monkey/mandrill pitches, golfer
// swing timing, cup-in zooms, whale cue, and scripted ball Bezier motion.
package holeinone

import (
	"fmt"
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

const (
	ballMonkey = iota
	ballMandrill
)

type bopEvt struct {
	beat, length float64
	bop, auto    bool
	mandrillType int
}

type monkeyEvt struct {
	beat float64
}

type mandrillEvt struct {
	beat float64
}

type whaleEvt struct {
	beat, length float64
	ease         int
	appear       bool
}

type ballCfg struct {
	shadowStartY, shadowEndY float64
	floorY, shadowMinSize    float64
}

type activeBall struct {
	inst        *kart.Instance
	kind        int
	currentBeat float64
	curve       int
	cfg         ballCfg
	hit, dead   bool
}

type grassParticle struct {
	inst  *kart.Instance
	start float64
	life  float64
	z     float64
}

type Module struct {
	ctx  *engine.Ctx
	proj kart.Aff

	monkey, monkeyHead string
	mandrill, golfer   string
	hole, ballEffect   string
	baseBall, grassFX  string
	grassArea          string

	ballT, grassT *kart.Template
	curves        map[string]kmdata.Curve
	ballCfg       ballCfg

	bops      []bopEvt
	monkeys   []monkeyEvt
	mandrills []mandrillEvt
	whales    []whaleEvt

	balls []*activeBall
	grass []*grassParticle

	currentMandrillBopType int
	lastMonkeyThrowBeat    float64
	lastPulse              float64
	lastBopBeat            float64
	canBop                 bool
	isWhale                bool
	rng                    *rand.Rand
}

func New() engine.Module {
	return &Module{
		lastPulse:           math.Inf(-1),
		lastBopBeat:         math.Inf(-1),
		lastMonkeyThrowBeat: math.Inf(-1),
		canBop:              true,
		rng:                 rand.New(rand.NewSource(0x1eaf)),
	}
}

func (m *Module) ID() string { return "holeInOne" }

func (m *Module) Load(ctx *engine.Ctx) error {
	m.ctx = ctx
	if err := ctx.LoadAssets("holeInOne"); err != nil {
		return err
	}
	m.proj = kart.Translate(engine.ScreenW/2, engine.ScreenH/2).Mul(kart.Scale(54, -54))

	m.monkey = roleOr(ctx, "MonkeyAnim", "Monkey")
	m.monkeyHead = roleOr(ctx, "MonkeyHeadAnim", "Monkey/MonkeyHead")
	m.mandrill = roleOr(ctx, "MandrillAnim", "Mandrill")
	m.golfer = roleOr(ctx, "GolferAnim", "Golfer")
	m.hole = roleOr(ctx, "Hole", "GolfHole")
	m.ballEffect = roleOr(ctx, "BallEffectAnim", "BallEffect")
	m.baseBall = roleOr(ctx, "baseBall", "Golfball")
	m.grassFX = roleOr(ctx, "grassEffectPrefab", "GrassEffect")
	m.grassArea = roleOr(ctx, "grassArea", "GrassEffects/GrassArea")

	m.ballT = kart.NewTemplate(ctx.Assets, m.baseBall)
	m.grassT = kart.NewTemplate(ctx.Assets, m.grassFX)
	m.curves = ctx.Assets.Extra.Curves
	m.ballCfg = readBallCfg(ctx.Assets.Extra.Components["ball"])

	ctx.Scene.SetActive(m.baseBall, false)
	ctx.Scene.SetActive(m.grassFX, false)
	ctx.Scene.SetActive(m.hole, false)
	return nil
}

func (m *Module) OnEvent(e *riq.Entity) {
	b := e.Beat
	switch e.Datamodel {
	case "holeInOne/bop":
		m.bops = append(m.bops, bopEvt{
			beat: b, length: e.Length,
			bop:          boolParamDefault(e, "bop", true),
			auto:         boolParam(e, "autoBop"),
			mandrillType: int(e.Float("mandrillBopType", 0)),
		})
	case "holeInOne/monkey":
		m.monkeys = append(m.monkeys, monkeyEvt{beat: b})
	case "holeInOne/mandrill":
		m.mandrills = append(m.mandrills, mandrillEvt{beat: b})
	case "holeInOne/whale":
		m.whales = append(m.whales, whaleEvt{
			beat: b, length: e.Length,
			ease:   int(e.Float("ease", 0)),
			appear: boolParamDefault(e, "appear", true),
		})
	}
}

func (m *Module) Ready() {
	sort.Slice(m.bops, func(i, j int) bool { return m.bops[i].beat < m.bops[j].beat })
	sort.Slice(m.monkeys, func(i, j int) bool { return m.monkeys[i].beat < m.monkeys[j].beat })
	sort.Slice(m.mandrills, func(i, j int) bool { return m.mandrills[i].beat < m.mandrills[j].beat })
	sort.Slice(m.whales, func(i, j int) bool { return m.whales[i].beat < m.whales[j].beat })

	for _, ev := range m.bops {
		ev := ev
		if !ev.bop {
			continue
		}
		for b := ev.beat; b < ev.beat+ev.length-1e-6; b++ {
			bb := b
			m.ctx.At(bb, func() { m.bop(bb) })
		}
	}
	for _, ev := range m.monkeys {
		m.doMonkey(ev.beat)
	}
	for _, ev := range m.mandrills {
		m.doMandrill(ev.beat)
	}
	for _, ev := range m.whales {
		ev := ev
		m.ctx.At(ev.beat, func() { m.startWhale(ev) })
	}
}

func (m *Module) OnSwitch(beat float64) {
	sec := m.ctx.SecPerBeat(beat)
	for _, p := range []string{
		"Background/Birds", "Background/Birds (1)", "Background/Clouds/MovingClouds",
		"Background/IslandArea", "Seagulls", m.monkey, m.monkeyHead, m.mandrill,
		m.golfer, m.hole, m.ballEffect,
	} {
		m.ctx.Scene.PlayDefaultState(p, beat, sec)
	}
	m.lastPulse = math.Floor(beat)
	m.lastBopBeat = math.Inf(-1)
	m.canBop = true
	m.isWhale = m.whaleVisibleAt(beat)
}

func (m *Module) Whiff(beat float64) {
	if m.playingAny(m.golfer, beat, "GolferWhiff", "GolferJust", "GolferThroughMandrill") {
		return
	}
	m.ctx.Sound("SE_GOLF_SWING")
	m.ctx.Scene.PlayState(m.golfer, "GolferWhiff", beat, m.ctx.SecPerBeat(beat))
}

func (m *Module) Update(_, beat float64) {
	if p := math.Floor(beat); p > m.lastPulse {
		m.lastPulse = p
		if m.autoBopAt(p) {
			m.bop(p)
		}
	}
	m.updateGrass(beat)
}

func (m *Module) Draw(screen *ebiten.Image, _, beat float64) {
	screen.Fill(color.RGBA{0x71, 0xc8, 0xf0, 0xff})
	m.ctx.SampleScene(beat)
	for _, b := range m.balls {
		if b.dead {
			continue
		}
		b.update(beat, m.curves)
		b.inst.Queue(m.ctx.Scene, beat, kart.Identity(), 0)
	}
	for _, g := range m.grass {
		g.inst.Queue(m.ctx.Scene, beat, kart.Identity(), g.z)
	}
	m.ctx.Scene.Draw(screen, m.proj)
}

func (m *Module) bop(beat float64) {
	if math.Abs(beat-m.lastBopBeat) < 1e-6 {
		return
	}
	m.lastBopBeat = beat
	m.currentMandrillBopType = m.latestMandrillBopType(beat)
	sec := m.ctx.SecPerBeat(beat)
	if !m.playingAny(m.monkey, beat, "MonkeyPrepare", "MonkeyThrow", "MonkeySpin") {
		m.ctx.Scene.PlayState(m.monkey, "MonkeyBop", beat, sec)
	}
	if !m.playingAny(m.mandrill, beat, "MandrillReady1", "MandrillReady2", "MandrillReady3", "MandrillPitch") {
		m.ctx.Scene.PlayState(m.mandrill, mandrillBopState(m.currentMandrillBopType), beat, sec)
	}
	if m.canBop && !m.playingAny(m.golfer, beat, "GolferJust", "GolferWhiff", "GolferPrepare", "GolferThrough", "GolferMiss", "GolferThroughMandrill") && !m.ctx.PressingNow() {
		m.ctx.Scene.PlayState(m.golfer, "GolferBop", beat, sec)
	}
}

func (m *Module) doMonkey(beat float64) {
	m.ctx.SoundAt(beat, "SE_GOLF_MONKEY_READY", 1)
	m.ctx.SoundAt(beat+1, "SE_GOLF_MONKEY", 1)
	m.ctx.SoundAt(beat+1, "SE_GOLF_MONKEY_THROW", 1)
	m.ctx.SoundAtOff(beat+2, "SE_GOLF_AUTO_SWING", 1, 0.083)

	skipPrepare := math.Abs(beat-m.lastMonkeyThrowBeat) < 1e-6
	m.lastMonkeyThrowBeat = beat + 1
	m.ctx.At(beat, func() {
		if !skipPrepare {
			m.ctx.Scene.PlayState(m.monkey, "MonkeyPrepare", beat, m.ctx.SecPerBeat(beat))
		}
	})
	m.ctx.At(beat+1, func() {
		m.ctx.Scene.PlayState(m.monkey, "MonkeyThrow", beat+1, m.ctx.SecPerBeat(beat+1))
		m.autoPrepare(beat + 1)
		m.spawnBall(beat, ballMonkey)
	})
	target := beat + 2
	m.ctx.ScheduleInputAction(target, 0,
		func(state float64, _ engine.Judgment) { m.monkeySuccess(target, state) },
		func() { m.monkeyMiss(target) },
	)
}

func (m *Module) doMandrill(beat float64) {
	m.ctx.SoundAt(beat, "SE_GOLF_GORILLA4", 1)
	m.ctx.SoundAt(beat+1, "SE_GOLF_GORILLA1", 1)
	m.ctx.SoundAt(beat+2, "SE_GOLF_GORILLA2", 1)
	m.ctx.SoundAt(beat+3, "SE_GOLF_GORILLA3", 1)
	m.ctx.SoundAt(beat+3, "SE_GOLF_GORILLA5", 1)

	m.ctx.At(beat, func() { m.ctx.Scene.PlayState(m.mandrill, "MandrillReady1", beat, m.ctx.SecPerBeat(beat)) })
	m.ctx.At(beat+1, func() {
		m.ctx.Scene.PlayState(m.mandrill, "MandrillReady2", beat+1, m.ctx.SecPerBeat(beat+1))
	})
	m.ctx.At(beat+2, func() {
		m.ctx.Scene.PlayState(m.mandrill, "MandrillReady3", beat+2, m.ctx.SecPerBeat(beat+2))
		m.autoPrepare(beat + 2)
		m.spawnBall(beat, ballMandrill)
	})
	m.ctx.At(beat+3, func() {
		m.ctx.Scene.PlayState(m.mandrill, "MandrillPitch", beat+3, m.ctx.SecPerBeat(beat+3))
		m.ctx.Scene.PlayState(m.monkey, "MonkeySpin", beat+3, m.ctx.SecPerBeat(beat+3))
		m.spawnGrassEffects(beat + 3)
	})
	target := beat + 3
	m.ctx.ScheduleInputAction(target, 0,
		func(state float64, _ engine.Judgment) { m.mandrillSuccess(target, state) },
		func() { m.mandrillMiss(target) },
	)
}

func (m *Module) autoPrepare(beat float64) {
	m.ctx.Scene.PlayState(m.golfer, "GolferPrepare", beat, m.ctx.SecPerBeat(beat))
	m.ctx.At(beat+0.7, func() {
		if !m.playingAny(m.golfer, beat+0.7, "GolferJust") {
			m.ctx.Scene.PlayState(m.golfer, "GolferThrough", beat+0.7, m.ctx.SecPerBeat(beat+0.7))
		}
	})
}

func (m *Module) spawnBall(startBeat float64, kind int) {
	if m.ballT == nil {
		return
	}
	b := &activeBall{
		inst:  m.ballT.NewInstance(),
		kind:  kind,
		curve: 0,
		cfg:   m.ballCfg,
	}
	if kind == ballMonkey {
		b.currentBeat = startBeat + 1
		b.setSmallVisible(true)
		b.setBigVisible(false)
	} else {
		b.currentBeat = startBeat + 2
		b.setSmallVisible(false)
		b.setBigVisible(false)
	}
	m.balls = append(m.balls, b)
}

func (m *Module) monkeySuccess(beat, state float64) {
	b := m.latestLiveBall(ballMonkey)
	if b == nil || b.hit || b.dead {
		return
	}
	b.hit = true
	if math.Abs(state) >= 0.8 {
		m.ctx.Sound("SE_GOLF_MISS_SHOT_LEFT")
		m.ctx.SoundAt(beat+1, "SE_GOLF_WATER_IN_LEFT", 1)
		m.ctx.Scene.PlayState(m.monkeyHead, "MonkeyMissHead", beat, m.ctx.SecPerBeat(beat))
		m.ctx.Scene.PlayState(m.golfer, "GolferMiss", beat, m.ctx.SecPerBeat(beat))
		b.dead = true
		return
	}
	success := 1 + m.rng.Intn(3)
	b.curve = 1
	b.currentBeat = beat
	m.ctx.Sound("SE_GOLF_SHOT")
	if m.isWhale {
		m.ctx.SoundAt(beat+2, "SE_GOLF_WHALE", 1)
	} else {
		switch success {
		case 1:
			m.ctx.SoundAt(beat+2, "SE_GOLF_CUP_IN", 1)
		case 2:
			m.ctx.SoundAt(beat+2, "SE_GOLF_CUP_IN_ROLL", 1)
		default:
			m.ctx.SoundAt(beat+2, "backspin1", 1)
			m.ctx.SoundAt(beat+3, "backspin2", 1)
		}
	}
	m.ctx.Scene.PlayState(m.monkeyHead, "MonkeyJustHead", beat, m.ctx.SecPerBeat(beat))
	m.ctx.Scene.PlayState(m.golfer, "GolferJust", beat, m.ctx.SecPerBeat(beat))
	m.ctx.At(beat+1.5, func() { m.ctx.Scene.SetActive(m.hole, true) })
	m.ctx.At(beat+2, func() {
		m.ctx.Scene.PlayState(m.hole, fmt.Sprintf("ZoomSmall%d", success), beat+2, m.ctx.SecPerBeat(beat+2))
		b.dead = true
	})
}

func (m *Module) monkeyMiss(beat float64) {
	b := m.latestLiveBall(ballMonkey)
	m.ctx.Sound(fmt.Sprintf("SE_GOLF_MISS_THROUGH%d", 1+m.rng.Intn(3)))
	m.ctx.Scene.PlayState(m.monkeyHead, "MonkeySadHead", beat, m.ctx.SecPerBeat(beat))
	m.canBop = false
	m.ctx.At(beat+2, func() {
		m.canBop = true
		if b != nil {
			b.dead = true
		}
	})
}

func (m *Module) mandrillSuccess(beat, state float64) {
	b := m.latestLiveBall(ballMandrill)
	if b == nil || b.hit || b.dead {
		return
	}
	b.hit = true
	b.setBigVisible(true)
	if math.Abs(state) >= 0.8 {
		m.ctx.Sound("SE_GOLF_MISS_SHOT_LEFT")
		m.ctx.SoundAt(beat+1, "SE_GOLF_WATER_IN_LEFT", 1)
		m.ctx.Scene.PlayState(m.ballEffect, "BallEffectJust", beat, m.ctx.SecPerBeat(beat))
		m.ctx.Scene.PlayState(m.golfer, "GolferJust", beat, m.ctx.SecPerBeat(beat))
		b.dead = true
		return
	}
	b.curve = 1
	b.currentBeat = beat
	m.ctx.Sound("SE_GOLF_SHOT_GORILLA_BIG_BALL")
	if m.isWhale {
		m.ctx.SoundAt(beat+2, "SE_GOLF_WHALE", 1)
	} else {
		m.ctx.SoundAt(beat+2, "SE_GOLF_CUP_IN_GORILLA", 1)
	}
	m.ctx.Scene.PlayState(m.ballEffect, "BallEffectJust", beat, m.ctx.SecPerBeat(beat))
	m.ctx.Scene.PlayState(m.golfer, "GolferJust", beat, m.ctx.SecPerBeat(beat))
	m.ctx.At(beat+1.5, func() { m.ctx.Scene.SetActive(m.hole, true) })
	m.ctx.At(beat+2, func() {
		m.ctx.Scene.PlayState(m.hole, "ZoomBig", beat+2, m.ctx.SecPerBeat(beat+2))
		b.dead = true
	})
}

func (m *Module) mandrillMiss(beat float64) {
	b := m.latestLiveBall(ballMandrill)
	if b != nil {
		b.setSmallVisible(true)
		b.setBigVisible(false)
	}
	m.ctx.Sound("SE_GOLF_MISS_BALL_ATTACK_BIG_GORI")
	m.ctx.Sound(fmt.Sprintf("SE_GOLF_MISS_BALL_ATTACK_BIG_VOICE%d", 1+m.rng.Intn(5)))
	m.ctx.Scene.PlayState(m.monkey, "MonkeySpin", beat, m.ctx.SecPerBeat(beat))
	m.ctx.Scene.PlayState(m.ballEffect, "BallEffectThrough", beat, m.ctx.SecPerBeat(beat))
	m.ctx.Scene.PlayState(m.golfer, "GolferThroughMandrill", beat, m.ctx.SecPerBeat(beat))
	m.canBop = false
	m.ctx.At(beat+2, func() {
		m.canBop = true
		if b != nil {
			b.dead = true
		}
	})
}

func (m *Module) startWhale(ev whaleEvt) {
	// Heaven Studio keeps the whale's transform motion disabled in C#; the
	// runtime-visible behavior is the sign cue plus toggling cup-in SFX.
	m.ctx.Sound("sign")
	m.isWhale = ev.appear
}

func (m *Module) spawnGrassEffects(beat float64) {
	if m.grassT == nil {
		return
	}
	// Original Coroutine spawns a dense left-side burst immediately and keeps
	// adding clumps for half a beat; positions are randomized inside grassArea.
	for i := 0; i < 9; i++ {
		m.addGrass(beat, true)
	}
	for i := 0; i < 26; i++ {
		t := beat + 0.5*float64(i)/26
		tt := t
		m.ctx.At(tt, func() { m.addGrass(tt, false) })
	}
}

func (m *Module) addGrass(beat float64, leftThird bool) {
	in := m.grassT.NewInstance()
	state := fmt.Sprintf("GrassEffect%d", 1+m.rng.Intn(3))
	speed := 0.6 + m.rng.Float64()*1.4
	in.PlayState("", state, beat, m.ctx.SecPerBeat(beat)*speed)
	x0, x1, y0, y1 := m.grassBounds()
	if leftThird {
		x1 = x0 + (x1-x0)/3
	}
	x := x0 + m.rng.Float64()*(x1-x0)
	y := y0 + m.rng.Float64()*(y1-y0)
	in.Offset = [2]float64{x, y}
	m.grass = append(m.grass, &grassParticle{
		inst:  in,
		start: beat,
		life:  secondsToBeats(m.ctx, beat, 4),
		z:     -0.01 + m.rng.Float64()*0.02,
	})
}

func (m *Module) updateGrass(beat float64) {
	if len(m.grass) == 0 {
		return
	}
	out := m.grass[:0]
	for _, g := range m.grass {
		if beat-g.start <= g.life {
			out = append(out, g)
		}
	}
	m.grass = out
}

func (m *Module) grassBounds() (x0, x1, y0, y1 float64) {
	for _, n := range m.ctx.Assets.Rig.Nodes {
		if n.Path == m.grassArea {
			w, h := n.Size[0], n.Size[1]
			if w <= 0 {
				w = 8
			}
			if h <= 0 {
				h = 2
			}
			return n.Pos[0] - w/2, n.Pos[0] + w/2, n.Pos[1] - h/2, n.Pos[1] + h/2
		}
	}
	return -5.5, 4.5, -3.2, -1.2
}

func (m *Module) latestLiveBall(kind int) *activeBall {
	for i := len(m.balls) - 1; i >= 0; i-- {
		if m.balls[i].kind == kind && !m.balls[i].dead {
			return m.balls[i]
		}
	}
	return nil
}

func (m *Module) playingAny(path string, beat float64, names ...string) bool {
	st, playing := m.ctx.Scene.StateInfo(path, beat)
	if !playing {
		return false
	}
	for _, n := range names {
		if st == n {
			return true
		}
	}
	return false
}

func (m *Module) autoBopAt(beat float64) bool {
	if len(m.bops) == 0 {
		return true
	}
	for _, ev := range m.bops {
		if beat >= ev.beat && beat < ev.beat+ev.length-1e-6 {
			return ev.auto
		}
	}
	return false
}

func (m *Module) latestMandrillBopType(beat float64) int {
	out := m.currentMandrillBopType
	for _, ev := range m.bops {
		if ev.beat > beat+1e-6 {
			break
		}
		out = ev.mandrillType
	}
	return out
}

func (m *Module) whaleVisibleAt(beat float64) bool {
	visible := false
	for _, ev := range m.whales {
		if ev.beat > beat+1e-6 {
			break
		}
		if beat < ev.beat+ev.length || ev.length <= 0 {
			visible = ev.appear
		}
	}
	return visible
}

func (b *activeBall) update(beat float64, curves map[string]kmdata.Curve) {
	if b.dead {
		return
	}
	key := fmt.Sprintf("ball.curve%d", b.curve)
	c, ok := curves[key]
	if !ok {
		return
	}
	if b.curve == 0 {
		u := clamp01((beat - b.currentBeat) / 3)
		if u > 0.55 {
			u = (u-0.55)/4 + 0.55
		}
		pos := kart.EvalBezier(c, u)
		b.inst.Offset = [2]float64{pos[0], pos[1]}
		shadowY := lerp(b.cfg.shadowStartY, b.cfg.shadowEndY, u)
		b.inst.SetPos("Shadow", 0, shadowY-pos[1])
		b.inst.SetScale("Shadow", 1, 1)
		return
	}
	u := clamp01((beat - b.currentBeat) / 2)
	pos := kart.EvalBezier(c, u)
	b.inst.Offset = [2]float64{pos[0], pos[1]}
	end := kart.EvalBezier(c, 1)
	shadowY := lerp(b.cfg.floorY, end[1], u)
	scale := lerp(1, b.cfg.shadowMinSize, u)
	b.inst.SetPos("Shadow", 0, shadowY-pos[1])
	b.inst.SetScale("Shadow", scale, scale)
	b.inst.SetScale("Shadow/BShadow", scale, scale)
	if u > 0.23 {
		b.inst.SetOrder("Shadow", -101)
		b.inst.SetOrder("Shadow/BShadow", -101)
		b.inst.SetOrder("", -99)
		b.inst.SetOrder("Bigball", -99)
	}
}

func (b *activeBall) setSmallVisible(on bool) {
	alpha := 0.0
	if on {
		alpha = 1
	}
	b.inst.SetColor("", [4]float64{1, 1, 1, alpha})
	b.inst.SetColor("Shadow", [4]float64{1, 1, 1, alpha})
}

func (b *activeBall) setBigVisible(on bool) {
	alpha := 0.0
	if on {
		alpha = 1
	}
	b.inst.SetColor("Bigball", [4]float64{1, 1, 1, alpha})
	b.inst.SetActive("Shadow/BShadow", on)
}

func readBallCfg(comp kmdata.Component) ballCfg {
	return ballCfg{
		shadowStartY:  numDefault(comp.Nums, "shadowStartY", -3),
		shadowEndY:    numDefault(comp.Nums, "shadowEndY", -4.5),
		floorY:        numDefault(comp.Nums, "floorY", -3.5),
		shadowMinSize: numDefault(comp.Nums, "shadowMinSize", 0.4),
	}
}

func mandrillBopState(t int) string {
	switch t {
	case 1:
		return "MandrillBop2"
	case 2:
		return "MandrillBop3"
	default:
		return "MandrillBop"
	}
}

func roleOr(ctx *engine.Ctx, key, fallback string) string {
	if v := ctx.Role(key); v != "" {
		return v
	}
	return fallback
}

func boolParam(e *riq.Entity, key string) bool { return boolParamDefault(e, key, false) }

func boolParamDefault(e *riq.Entity, key string, def bool) bool {
	if _, ok := e.Data[key]; !ok {
		return def
	}
	return e.Float(key, 0) != 0
}

func numDefault(m map[string]float64, key string, def float64) float64 {
	if m == nil {
		return def
	}
	if v, ok := m[key]; ok {
		return v
	}
	return def
}

func secondsToBeats(ctx *engine.Ctx, beat, sec float64) float64 {
	spb := ctx.SecPerBeat(beat)
	if spb <= 0 {
		return 0
	}
	return sec / spb
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

func lerp(a, b, t float64) float64 { return a + (b-a)*t }
