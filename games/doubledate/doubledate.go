// Package doubledate ports Double Date's ball trajectories, boy/girl/weasel
// reactions, background time switches, and cue sounds from
// Assets/Scripts/Games/DoubleDate.
package doubledate

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

const (
	ballSoccer = iota
	ballBasket
	ballFootball
)

const (
	dayTimeDay = iota
	dayTimeSunset
)

const ballOffsetY = 0.5

type bopEvt struct {
	beat, length float64
	bop, auto    bool
}

type ballEvt struct {
	beat float64
	kind int
	jump bool
	uid  int
}

type boolEvt struct {
	beat float64
	on   bool
}

type timeEvt struct {
	beat float64
	time int
}

type pathPoint struct {
	tag         string
	pos         [3]float64
	height      float64
	duration    float64
	useLastReal bool
	values      map[string]float64
}

type ballPath struct {
	name   string
	points []pathPoint
}

type activeBall struct {
	uid           int
	kind          int
	jump          bool
	inst          *kart.Instance
	shadow        *kart.Instance
	path          *ballPath
	pathName      string
	pathStartBeat float64
	lastRealPos   [3]float64
	lastBeat      float64
	rot           float64
	order         int
	scaleMul      float64
	shadowOn      bool
	deathBeat     float64
}

type leafParticle struct {
	bornT, life      float64
	x, y, vx, vy     float64
	rot, rotV, scale float64
	alpha, gravity   float64
}

type renderColor struct {
	path string
	base [4]float64
}

type Module struct {
	ctx  *engine.Ctx
	proj kart.Aff
	rng  *rand.Rand

	comp kmdata.Component

	boy, girl, weasels, tree       string
	clouds, girlObj                string
	girlWeaselObj, girlShockObj    string
	bg, bush, bgGradient, bgSquare string

	soccerT, basketT, footballT, shadowT *kart.Template
	shadowRootScale                      [2]float64
	paths                                map[string]*ballPath

	animSpeed                        float64
	cloudSpeed, cloudDistance        float64
	floorHeight                      float64
	shadowDepthMin, shadowDepthMax   float64
	skyColor, noonColor, squareColor [4]float64
	bgIntro, bgLong                  string
	sceneColors                      []renderColor
	materialTint, fillColor          [4]float64

	bops   []bopEvt
	balls  []ballEvt
	blush  []float64
	girls  []boolEvt
	stares []boolEvt
	times  []timeEvt
	bgs    []boolEvt

	activeBalls []*activeBall
	leaves      []leafParticle

	girlsPresent bool
	staring      bool
	bgActive     bool
	dayTime      int

	canBop          bool
	weaselsCanBop   bool
	weaselsNotHit   bool
	lastGirlGacha   float64
	lastWeaselGacha float64
	lastHitWeasel   float64
	lastBopBeat     float64
}

func New() engine.Module {
	return &Module{
		rng:             rand.New(rand.NewSource(0xd0d1e)),
		paths:           map[string]*ballPath{},
		animSpeed:       1,
		girlsPresent:    true,
		bgActive:        true,
		dayTime:         dayTimeSunset,
		canBop:          true,
		weaselsCanBop:   true,
		weaselsNotHit:   true,
		lastGirlGacha:   math.Inf(-1),
		lastWeaselGacha: math.Inf(-1),
		lastHitWeasel:   math.Inf(-1),
		lastBopBeat:     math.Inf(-1),
	}
}

func (m *Module) ID() string { return "doubleDate" }

func (m *Module) Load(ctx *engine.Ctx) error {
	m.ctx = ctx
	if err := ctx.LoadAssets("doubleDate"); err != nil {
		return err
	}
	m.proj = kart.Translate(engine.ScreenW/2, engine.ScreenH/2).Mul(kart.Scale(54, -54))
	m.comp = ctx.Assets.Extra.Components["game"]

	m.boy = refOr(ctx, m.comp, "boyAnim", "Boy")
	m.girl = refOr(ctx, m.comp, "girlAnim", "Girl")
	m.weasels = refOr(ctx, m.comp, "weasels", "Weasels")
	m.tree = refOr(ctx, m.comp, "treeAnim", "Tree")
	m.clouds = refOr(ctx, m.comp, "clouds", "Background/CloudGroup")
	m.girlObj = refOr(ctx, m.comp, "girlObj", "Girl")
	m.girlWeaselObj = refOr(ctx, m.comp, "girlWeaselObj", "Weasels/WeaselGirl")
	m.girlShockObj = refOr(ctx, m.comp, "girlWeaselShockObj", "Weasels/Shock2")
	m.bg = refOr(ctx, m.comp, "bgGO", "Background")
	m.bush = refOr(ctx, m.comp, "bushGO", "Weasels/WeaselBush")
	m.bgGradient = refOr(ctx, m.comp, "bgGradient", "Background/GradientBackground")
	m.bgSquare = refOr(ctx, m.comp, "bgSquare", "Background/GradientBackground/Square")

	m.animSpeed = numOr(m.comp, "_animSpeed", 1)
	m.cloudSpeed = numOr(m.comp, "cloudSpeed", 0.06)
	m.cloudDistance = numOr(m.comp, "cloudDistance", 20)
	m.floorHeight = numOr(m.comp, "floorHeight", -2.75)
	m.shadowDepthMin = numOr(m.comp, "shadowDepthScaleMin", 5.25)
	m.shadowDepthMax = numOr(m.comp, "shadowDepthScaleMax", 0.25)
	m.skyColor = colorField(m.comp, "_skyColor", [4]float64{0.64705884, 1, 1, 1})
	m.noonColor = colorField(m.comp, "noonColor", [4]float64{0.9529412, 0.87058824, 0.7764706, 1})
	m.bgIntro = spriteOr(m.comp, "bgIntro", "GradientIntro")
	m.bgLong = spriteOr(m.comp, "bgLong", "GradientBackground")

	m.soccerT = kart.NewTemplate(ctx.Assets, refOr(ctx, m.comp, "soccer", "SoccerBall"))
	m.basketT = kart.NewTemplate(ctx.Assets, refOr(ctx, m.comp, "basket", "BasketBall"))
	m.footballT = kart.NewTemplate(ctx.Assets, refOr(ctx, m.comp, "football", "Football"))
	m.shadowT = kart.NewTemplate(ctx.Assets, refOr(ctx, m.comp, "dropShadow", "DropShadow"))
	m.shadowRootScale = nodeScale(ctx.Assets, refOr(ctx, m.comp, "dropShadow", "DropShadow"))
	m.squareColor = nodeColor(ctx.Assets, m.bgSquare, [4]float64{0.93725497, 0.52156866, 0.2901961, 1})
	m.sceneColors = spriteNodeColors(ctx.Assets)
	m.paths = parseBallPaths(m.comp.Lists["ballBouncePaths"])

	ctx.Scene.SetActive(refOr(ctx, m.comp, "soccer", "SoccerBall"), false)
	ctx.Scene.SetActive(refOr(ctx, m.comp, "basket", "BasketBall"), false)
	ctx.Scene.SetActive(refOr(ctx, m.comp, "football", "Football"), false)
	ctx.Scene.SetActive(refOr(ctx, m.comp, "dropShadow", "DropShadow"), false)
	ctx.Scene.SetActive(refOr(ctx, m.comp, "leaves", "Leaves"), false)
	m.resetScene(0)
	return nil
}

func (m *Module) OnEvent(e *riq.Entity) {
	switch e.Datamodel {
	case "doubleDate/bop":
		m.bops = append(m.bops, bopEvt{
			beat: e.Beat, length: e.Length,
			bop:  boolDefault(e, "bop", true),
			auto: boolParam(e, "autoBop"),
		})
	case "doubleDate/soccer":
		m.balls = append(m.balls, ballEvt{beat: e.Beat, kind: ballSoccer, jump: boolParam(e, "b"), uid: len(m.balls)})
	case "doubleDate/basket":
		m.balls = append(m.balls, ballEvt{beat: e.Beat, kind: ballBasket, jump: boolParam(e, "b"), uid: len(m.balls)})
	case "doubleDate/football":
		m.balls = append(m.balls, ballEvt{beat: e.Beat, kind: ballFootball, jump: boolDefault(e, "b", true), uid: len(m.balls)})
	case "doubleDate/blush":
		m.blush = append(m.blush, e.Beat)
	case "doubleDate/toggleGirls":
		m.girls = append(m.girls, boolEvt{beat: e.Beat, on: boolParam(e, "b")})
	case "doubleDate/stare":
		m.stares = append(m.stares, boolEvt{beat: e.Beat, on: boolDefault(e, "b", true)})
	case "doubleDate/time":
		m.times = append(m.times, timeEvt{beat: e.Beat, time: int(e.Float("d", dayTimeSunset))})
	case "doubleDate/toggleBG":
		m.bgs = append(m.bgs, boolEvt{beat: e.Beat, on: boolDefault(e, "b", true)})
	}
}

func (m *Module) Ready() {
	sort.SliceStable(m.bops, func(i, j int) bool { return m.bops[i].beat < m.bops[j].beat })
	sort.SliceStable(m.balls, func(i, j int) bool { return m.balls[i].beat < m.balls[j].beat })
	sort.Float64s(m.blush)
	sort.SliceStable(m.girls, func(i, j int) bool { return m.girls[i].beat < m.girls[j].beat })
	sort.SliceStable(m.stares, func(i, j int) bool { return m.stares[i].beat < m.stares[j].beat })
	sort.SliceStable(m.times, func(i, j int) bool { return m.times[i].beat < m.times[j].beat })
	sort.SliceStable(m.bgs, func(i, j int) bool { return m.bgs[i].beat < m.bgs[j].beat })

	for _, b := range m.bopBeats() {
		bb := b
		m.ctx.At(bb, func() { m.singleBop(bb) })
	}
	for _, ev := range m.balls {
		ev := ev
		m.scheduleBall(ev)
	}
	for _, b := range m.blush {
		bb := b
		m.ctx.At(bb, func() { m.girlBlush(bb) })
	}
	for _, ev := range m.girls {
		ev := ev
		m.ctx.At(ev.beat, func() { m.toggleGirls(ev.on) })
	}
	for _, ev := range m.stares {
		ev := ev
		m.ctx.At(ev.beat, func() { m.toggleStare(ev.on) })
	}
	for _, ev := range m.times {
		ev := ev
		m.ctx.At(ev.beat, func() { m.setTime(ev.time) })
	}
	for _, ev := range m.bgs {
		ev := ev
		m.ctx.At(ev.beat, func() { m.toggleBackground(ev.on) })
	}
}

func (m *Module) OnSwitch(beat float64) {
	if beat <= 0 {
		m.activeBalls = nil
		m.leaves = nil
	}
	m.resetScene(beat)
	for _, ev := range m.girls {
		if ev.beat >= beat {
			break
		}
		m.toggleGirls(ev.on)
	}
	for _, ev := range m.stares {
		if ev.beat >= beat {
			break
		}
		m.toggleStare(ev.on)
	}
	for _, ev := range m.times {
		if ev.beat >= beat {
			break
		}
		m.setTime(ev.time)
	}
	for _, ev := range m.bgs {
		if ev.beat >= beat {
			break
		}
		m.toggleBackground(ev.on)
	}
}

func (m *Module) Whiff(beat float64) {
	m.ctx.Sound("kick_whiff")
	m.kick(beat, true, true, false, false)
}

func (m *Module) Update(_, beat float64) {
	if m.ctx.PressedNow() {
		m.playBoyState("Ready", beat)
		m.setCanBop(false)
	}
	if m.ctx.ReleasedNow() && !m.ctx.ExpectingPressNow() {
		m.playBoyState("UnReady", beat)
		m.scheduleCanBop(true, beat, 0.13333334, m.animSpeed)
	}
	m.updateClouds()
	m.updateBalls(beat)
}

func (m *Module) Draw(screen *ebiten.Image, _ float64, beat float64) {
	screen.Fill(toNRGBA(m.fillColor))
	m.applySceneTint()
	m.updateClouds()
	m.ctx.SampleScene(beat)
	for _, b := range m.activeBalls {
		if b.deadAt(beat) {
			continue
		}
		b.queue(m, beat)
	}
	m.queueLeaves(beat)
	m.ctx.Scene.Draw(screen, m.proj)
}

func (m *Module) resetScene(beat float64) {
	m.girlsPresent = true
	m.staring = false
	m.bgActive = true
	m.dayTime = dayTimeSunset
	m.canBop = true
	m.weaselsCanBop = true
	m.weaselsNotHit = true
	m.lastGirlGacha = math.Inf(-1)
	m.lastWeaselGacha = math.Inf(-1)
	m.lastHitWeasel = math.Inf(-1)
	m.lastBopBeat = math.Inf(-1)
	sec := m.ctx.SecPerBeat(beat)
	m.ctx.Scene.PlayDefaultState(m.boy, beat, sec)
	m.ctx.Scene.PlayDefaultState(m.girl, beat, sec)
	m.ctx.Scene.PlayDefaultState(m.weasels, beat, sec)
	m.ctx.Scene.PlayDefaultState(m.tree, beat, sec)
	m.ctx.Scene.SetBool(m.boy, "Stare", false)
	m.toggleGirls(true)
	m.toggleBackground(true)
	m.setTime(dayTimeSunset)
}

func (m *Module) bopBeats() []float64 {
	seen := map[float64]bool{}
	var out []float64
	add := func(b float64) {
		if seen[b] {
			return
		}
		seen[b] = true
		out = append(out, b)
	}
	fallbackEnd := 0.0
	for _, ev := range m.balls {
		fallbackEnd = math.Max(fallbackEnd, ev.beat+16)
	}
	for _, ev := range m.bops {
		fallbackEnd = math.Max(fallbackEnd, ev.beat+ev.length+16)
	}
	for i, ev := range m.bops {
		if ev.bop {
			for k := 0.0; k < ev.length-1e-6; k++ {
				add(ev.beat + k)
			}
		}
		if ev.auto {
			until := m.ctx.NextSwitchBeat(ev.beat)
			if math.IsInf(until, 1) {
				until = fallbackEnd
			}
			if i+1 < len(m.bops) {
				until = m.bops[i+1].beat
			}
			for b := math.Ceil(ev.beat); b < until-1e-6; b++ {
				add(b)
			}
		}
	}
	sort.Float64s(out)
	return out
}

func (m *Module) scheduleBall(ev ballEvt) {
	switch ev.kind {
	case ballSoccer:
		m.ctx.SoundAt(ev.beat, "soccerBounce", 1)
	case ballBasket:
		m.ctx.SoundAt(ev.beat, "basketballBounce", 1)
		m.ctx.SoundAt(ev.beat+0.75, "basketballBounce", 1)
	case ballFootball:
		m.ctx.SoundAt(ev.beat, "footballBounce", 1)
		m.ctx.SoundAt(ev.beat+0.75, "footballBounce", 1)
	}
	m.ctx.At(ev.beat, func() { m.spawnBall(ev) })
	target := ev.beat + 1
	if ev.kind == ballFootball {
		target = ev.beat + 1.5
	}
	m.ctx.ScheduleInput(target,
		func(state float64, _ engine.Judgment) { m.ballHit(ev.uid, state) },
		func() { m.ballMiss(ev.uid) },
	)
}

func (m *Module) spawnBall(ev ballEvt) {
	var tmpl *kart.Template
	var pathName string
	switch ev.kind {
	case ballSoccer:
		tmpl, pathName = m.soccerT, "SoccerIn"
	case ballBasket:
		tmpl, pathName = m.basketT, "BasketBallIn"
	case ballFootball:
		tmpl, pathName = m.footballT, "FootBallInNoHit"
	}
	if tmpl == nil || m.paths[pathName] == nil {
		return
	}
	in := tmpl.NewInstance()
	in.SetColor("", m.materialTint)
	b := &activeBall{
		uid: ev.uid, kind: ev.kind, jump: ev.jump, inst: in,
		path: m.paths[pathName], pathName: pathName, pathStartBeat: ev.beat - 1,
		lastBeat: math.Inf(-1), order: 1, scaleMul: 1, shadowOn: true, deathBeat: math.Inf(1),
	}
	b.updatePose(m, ev.beat-1)
	if m.shadowT != nil {
		b.shadow = m.shadowT.NewInstance()
		b.shadow.SetColor("Shadow", m.materialTint)
	}
	m.activeBalls = append(m.activeBalls, b)
}

func (m *Module) ballHit(uid int, state float64) {
	b := m.findBall(uid)
	if b == nil || b.deadAt(m.ctx.Beat()) {
		return
	}
	now := m.ctx.Beat()
	b.updateLastReal(m, now)
	b.pathStartBeat = now
	b.deathBeat = now + 3
	if state >= 1 || state <= -1 {
		suffix := "Late"
		if state < 0 {
			suffix = "Early"
		}
		switch b.kind {
		case ballSoccer:
			b.setPath(m, "SoccerNg"+suffix)
		case ballBasket:
			b.setPath(m, "BasketBallNg"+suffix)
		case ballFootball:
			b.setPath(m, "FootBallNg"+suffix)
			b.deathBeat = now + 4
		}
		m.ctx.PlayCommon("miss")
		m.kick(now, false, false, true, false)
		b.order = 8
		b.inst.SetOrder("", 8)
		return
	}
	switch b.kind {
	case ballSoccer:
		b.setPath(m, "SoccerJust")
		m.kick(now, true, true, true, b.jump)
		m.ctx.Sound("kick")
	case ballBasket:
		b.setPath(m, "BasketBallJust")
		m.kick(now, true, true, true, b.jump)
		m.ctx.Sound("kick")
	case ballFootball:
		b.setPath(m, "FootBallJust")
		m.kick(now, true, false, true, b.jump)
		m.ctx.Sound("footballKick")
		b.deathBeat = now + 12
		m.ctx.At(now+1, func() {
			if b.deadAt(m.ctx.Beat()) {
				return
			}
			b.shadowOn = false
			b.order = -5
			b.scaleMul *= 0.25
			b.inst.SetOrder("", -5)
			b.updateLastReal(m, now+1)
			b.setPath(m, "FootBallFall")
			b.pathStartBeat = now + 1
		})
	}
}

func (m *Module) ballMiss(uid int) {
	b := m.findBall(uid)
	if b == nil || b.deadAt(m.ctx.Beat()) {
		return
	}
	switch b.kind {
	case ballSoccer:
		m.ctx.Sound("weasel_hide")
		m.missKick(b.pathStartBeat+2.25, false)
		b.deathBeat = m.ctx.Beat() + 4
	case ballBasket:
		m.ctx.Sound("weasel_hide")
		m.missKick(b.pathStartBeat+2.25, false)
	case ballFootball:
		now := m.ctx.Beat()
		if now > m.lastHitWeasel+2.25 {
			b.setPath(m, "FootBallIn")
			b.order = 8
			b.inst.SetOrder("", 8)
			impact := pointTimeByTag(b.path, "impact")
			if impact > 0 {
				impactBeat := b.pathStartBeat + impact
				m.ctx.SoundAt(impactBeat, "weasel_hit", 1)
				m.ctx.SoundAt(impactBeat, "weasel_scream", 1)
				m.missKick(impactBeat, true)
			}
		}
		b.deathBeat = now + 5
	}
}

func (m *Module) findBall(uid int) *activeBall {
	for _, b := range m.activeBalls {
		if b.uid == uid {
			return b
		}
	}
	return nil
}

func (m *Module) updateBalls(beat float64) {
	out := m.activeBalls[:0]
	for _, b := range m.activeBalls {
		if b.deadAt(beat) || beat-b.pathStartBeat > 30 {
			continue
		}
		b.updatePose(m, beat)
		out = append(out, b)
	}
	m.activeBalls = out
}

func (b *activeBall) deadAt(beat float64) bool { return beat >= b.deathBeat }

func (b *activeBall) setPath(m *Module, name string) {
	if p := m.paths[name]; p != nil {
		b.path = p
		b.pathName = name
	}
}

func (b *activeBall) updateLastReal(m *Module, beat float64) {
	pos, _, _ := samplePath(b.path, beat, b.pathStartBeat, b.lastRealPos)
	b.lastRealPos = pos
}

func (b *activeBall) updatePose(m *Module, beat float64) {
	if b.path == nil {
		return
	}
	pos, height, value := samplePath(b.path, math.Max(beat, b.pathStartBeat), b.pathStartBeat, b.lastRealPos)
	pos[1] += ballOffsetY
	if math.IsInf(b.lastBeat, -1) {
		b.lastBeat = beat
	} else if beat >= b.lastBeat {
		b.rot -= value("rot") * (beat - b.lastBeat) * math.Pi / 180
		b.lastBeat = beat
	}
	b.inst.Offset = [2]float64{pos[0], pos[1]}
	b.inst.Rot = b.rot
	b.inst.Scale = [2]float64{b.scaleMul, b.scaleMul}
	b.inst.SetColor("", m.materialTint)
	b.inst.SetOrder("", b.order)
	if b.shadow != nil {
		shadowY := math.Min(pos[1]-height, m.floorHeight)
		b.shadow.Offset = [2]float64{pos[0], shadowY}
		s := clamp01((pos[1] - m.shadowDepthMin) / (m.shadowDepthMax - m.shadowDepthMin))
		if m.shadowRootScale[0] != 0 {
			b.shadow.Scale[0] = s / m.shadowRootScale[0]
		}
		if m.shadowRootScale[1] != 0 {
			b.shadow.Scale[1] = s / m.shadowRootScale[1]
		}
		b.shadow.SetColor("Shadow", m.materialTint)
	}
}

func (b *activeBall) queue(m *Module, beat float64) {
	b.updatePose(m, beat)
	if b.shadow != nil && b.shadowOn {
		b.shadow.Queue(m.ctx.Scene, beat, kart.Identity(), 0)
	}
	b.inst.Queue(m.ctx.Scene, beat, kart.Identity(), 0)
}

func samplePath(path *ballPath, currentBeat, startBeat float64, lastReal [3]float64) ([3]float64, float64, func(string) float64) {
	if path == nil || len(path.points) < 2 {
		return lastReal, 0, func(string) float64 { return 0 }
	}
	rel := currentBeat - startBeat
	current := path.points[0]
	next := path.points[1]
	posTime := 0.0
	for i := 0; i < len(path.points)-1; i++ {
		current = path.points[i]
		next = path.points[i+1]
		if rel >= posTime && rel < posTime+current.duration {
			break
		}
		if i+1 < len(path.points)-1 {
			posTime += current.duration
			continue
		}
		break
	}
	t := 1.0
	if current.duration != 0 {
		t = (rel - posTime) / current.duration
	}
	start := current.pos
	if current.useLastReal {
		start = lastReal
	}
	end := next.pos
	pos := [3]float64{
		lerp(start[0], end[0], t),
		lerp(start[1], end[1], t),
		lerp(start[2], end[2], t),
	}
	yMul := t*2 - 1
	height := (-(yMul * yMul) + 1) * current.height
	pos[1] += height
	return pos, height, func(key string) float64 { return current.values[key] }
}

func parseBallPaths(items []kmdata.ComponentItem) map[string]*ballPath {
	out := map[string]*ballPath{}
	for _, item := range items {
		name := item.Strs["name"]
		if name == "" || item.Items == nil {
			continue
		}
		p := &ballPath{name: name}
		for _, pi := range item.Items["positions"] {
			pt := pathPoint{
				tag:         pi.Strs["tag"],
				pos:         [3]float64{pi.Nums["pos.x"], pi.Nums["pos.y"], pi.Nums["pos.z"]},
				height:      pi.Nums["height"],
				duration:    pi.Nums["duration"],
				useLastReal: pi.Nums["useLastRealPos"] != 0,
				values:      map[string]float64{},
			}
			for _, vi := range pi.Items["values"] {
				if key := vi.Strs["key"]; key != "" {
					pt.values[key] = vi.Nums["value"]
				}
			}
			p.points = append(p.points, pt)
		}
		out[name] = p
	}
	return out
}

func pointTimeByTag(path *ballPath, tag string) float64 {
	if path == nil {
		return 0
	}
	t := 0.0
	for _, pt := range path.points {
		if pt.tag == tag {
			return t
		}
		t += pt.duration
	}
	return 0
}

func (m *Module) singleBop(beat float64) {
	if math.Abs(beat-m.lastBopBeat) < 1e-6 {
		return
	}
	m.lastBopBeat = beat
	if m.canBop {
		if m.staring {
			m.playBoyState("IdleBop2", beat)
		} else {
			m.playBoyState("IdleBop", beat)
		}
	}
	if beat > m.lastGirlGacha {
		m.ctx.Scene.PlayState(m.girl, "GirlBop", beat, m.ctx.SecPerBeat(beat)*m.animSpeed)
	}
	m.weaselsBop(beat)
}

func (m *Module) kick(beat float64, hit, forceNoLeaves, weaselsHappy, jump bool) {
	if hit {
		m.playBoyState("Kick", beat)
		m.setCanBop(false)
		m.scheduleCanBop(true, beat, 0.73333335, m.animSpeed)
		if jump {
			m.weaselsJump(beat)
			m.lastGirlGacha = beat + 0.5
			m.ctx.Scene.PlayState(m.girl, "GirlLookUp", beat, m.ctx.SecPerBeat(beat)*m.animSpeed)
		} else if weaselsHappy {
			m.weaselsHappy(beat)
		}
		if !forceNoLeaves {
			m.ctx.At(beat+1, func() {
				m.spawnLeaves(beat + 1)
				m.ctx.Scene.PlayState(m.tree, "TreeRustle", beat+1, m.ctx.SecPerBeat(beat+1)*m.animSpeed)
			})
		}
		return
	}
	m.playBoyState("Barely", beat)
	m.setCanBop(false)
	m.scheduleCanBop(true, beat, 0.8, m.animSpeed)
	m.weaselsSurprise(beat)
}

func (m *Module) missKick(beat float64, hit bool) {
	now := m.ctx.Beat()
	m.lastGirlGacha = now + 1.5
	m.ctx.Scene.PlayState(m.girl, "GirlSad", now, m.ctx.SecPerBeat(now)*m.animSpeed)
	m.lastHitWeasel = now
	if hit {
		m.ctx.At(beat-(0.25/3), func() { m.weaselsHit(beat) })
	} else {
		m.ctx.At(beat+0.25, func() { m.weaselsHide(beat + 0.25) })
	}
}

func (m *Module) girlBlush(beat float64) {
	m.ctx.Scene.PlayState(m.girl, "GirlBlush", beat, m.ctx.SecPerBeat(beat)*m.animSpeed)
}

func (m *Module) toggleGirls(active bool) {
	m.girlsPresent = active
	m.ctx.Scene.SetActive(m.girlObj, active)
	m.ctx.Scene.SetActive(m.girlWeaselObj, active)
	m.ctx.Scene.SetActive(m.girlShockObj, active)
}

func (m *Module) toggleStare(active bool) {
	m.staring = active
	m.ctx.Scene.SetBool(m.boy, "Stare", active)
}

func (m *Module) toggleBackground(active bool) {
	m.bgActive = active
	m.ctx.Scene.SetActive(m.bg, active)
	m.ctx.Scene.SetActive(m.bush, active)
}

func (m *Module) setTime(t int) {
	m.dayTime = t
	if t == dayTimeSunset {
		m.materialTint = m.noonColor
		m.fillColor = mulColor(m.squareColor, m.noonColor)
		m.ctx.Scene.SetSpriteOver(m.bgGradient, m.bgLong)
		return
	}
	m.materialTint = [4]float64{1, 1, 1, 1}
	m.fillColor = m.skyColor
	m.ctx.Scene.SetSpriteOver(m.bgGradient, m.bgIntro)
}

func (m *Module) playBoyState(state string, beat float64) {
	m.ctx.Scene.PlayState(m.boy, state, beat, m.ctx.SecPerBeat(beat)*m.animSpeed)
}

func (m *Module) setCanBop(on bool) { m.canBop = on }

func (m *Module) scheduleCanBop(on bool, beat, clipSec, speed float64) {
	m.ctx.At(eventBeat(m.ctx, beat, clipSec, speed), func() { m.canBop = on })
}

func (m *Module) weaselsBop(beat float64) {
	if m.weaselsCanBop && m.weaselsNotHit && beat > m.lastWeaselGacha {
		m.ctx.Scene.PlayState(m.weasels, "WeaselsBop", beat, m.ctx.SecPerBeat(beat)*0.5)
	}
}

func (m *Module) weaselsHappy(beat float64) {
	if m.weaselsNotHit && beat > m.lastWeaselGacha {
		m.ctx.Scene.PlayState(m.weasels, "WeaselsHappy", beat, m.ctx.SecPerBeat(beat)*0.5)
		m.weaselsCanBop = false
		m.ctx.At(eventBeat(m.ctx, beat, 0.1, 0.5), func() { m.weaselsCanBop = true })
	}
}

func (m *Module) weaselsJump(beat float64) {
	if m.weaselsNotHit && beat > m.lastWeaselGacha {
		m.lastWeaselGacha = beat + 1
		m.ctx.Scene.PlayState(m.weasels, "WeaselsJump", beat, m.ctx.SecPerBeat(beat)*0.5)
	}
}

func (m *Module) weaselsHide(beat float64) {
	if !m.weaselsNotHit {
		return
	}
	m.weaselsNotHit = false
	m.ctx.Scene.PlayState(m.weasels, "WeaselsHide", beat, m.ctx.SecPerBeat(beat)*0.5)
	m.ctx.At(beat+1.45, func() {
		now := m.ctx.Beat()
		m.lastWeaselGacha = now + 0.5
		m.ctx.Scene.PlayState(m.weasels, "WeaselsAppearUpset", now, m.ctx.SecPerBeat(now)*0.5)
		m.weaselsNotHit = true
	})
}

func (m *Module) weaselsSurprise(beat float64) {
	if m.weaselsNotHit && beat > m.lastWeaselGacha {
		m.lastWeaselGacha = beat + 0.5
		m.ctx.Scene.PlayState(m.weasels, "WeaselsSurprised", beat, m.ctx.SecPerBeat(beat)*0.5)
	}
}

func (m *Module) weaselsHit(beat float64) {
	if !m.weaselsNotHit {
		return
	}
	m.weaselsNotHit = false
	m.ctx.Scene.PlayState(m.weasels, "WeaselsHit", beat, m.ctx.SecPerBeat(beat)*0.5)
	m.ctx.At(beat+2, func() {
		now := m.ctx.Beat()
		m.lastWeaselGacha = now + 0.5
		m.ctx.Scene.PlayState(m.weasels, "WeaselsAppearUpset", now, m.ctx.SecPerBeat(now)*1)
		m.weaselsNotHit = true
	})
}

func eventBeat(ctx *engine.Ctx, beat, clipSec, speed float64) float64 {
	scale := ctx.SecPerBeat(beat) * speed
	if scale <= 0 {
		return beat
	}
	return beat + clipSec/scale
}

func (m *Module) updateClouds() {
	if m.cloudDistance == 0 {
		return
	}
	x := -math.Mod(m.ctx.Time()*m.cloudSpeed, m.cloudDistance)
	m.ctx.Scene.SetPosOver(m.clouds, x, 0)
}

func (m *Module) spawnLeaves(beat float64) {
	origin := m.nodeWorldPos(refOr(m.ctx, m.comp, "leaves", "Leaves"))
	born := m.ctx.BeatToTime(beat)
	for i := 0; i < 34; i++ {
		ang := -0.15 + m.rng.Float64()*1.2
		speed := 1.4 + m.rng.Float64()*4.2
		life := 1.6 + m.rng.Float64()*1.7
		m.leaves = append(m.leaves, leafParticle{
			bornT: born, life: life,
			x:       origin[0] + (m.rng.Float64()-0.5)*0.7,
			y:       origin[1] - 2.6 + (m.rng.Float64()-0.5)*0.5,
			vx:      math.Cos(ang) * speed,
			vy:      math.Sin(ang)*speed + 1.1 + m.rng.Float64()*2.0,
			rot:     m.rng.Float64() * math.Pi * 2,
			rotV:    (m.rng.Float64() - 0.5) * 7,
			scale:   0.18 + m.rng.Float64()*0.24,
			alpha:   0.9,
			gravity: 3.2 + m.rng.Float64()*2.8,
		})
	}
}

func (m *Module) queueLeaves(beat float64) {
	now := m.ctx.Time()
	out := m.leaves[:0]
	for _, p := range m.leaves {
		age := now - p.bornT
		if age < 0 || age > p.life {
			continue
		}
		out = append(out, p)
		x := p.x + p.vx*age
		y := p.y + p.vy*age - 0.5*p.gravity*age*age
		alpha := p.alpha * (1 - age/p.life)
		tint := m.materialTint
		tint[3] *= alpha
		world := kart.Translate(x, y).Mul(kart.Rotate(p.rot + p.rotV*age)).Mul(kart.Scale(p.scale, p.scale))
		m.ctx.Scene.Queue(kart.ExtraSprite{Sprite: "Leaf", World: world, Order: 4, Tint: tint})
	}
	m.leaves = out
}

func (m *Module) applySceneTint() {
	for _, rc := range m.sceneColors {
		m.ctx.Scene.SetColorOver(rc.path, mulColor(rc.base, m.materialTint))
	}
	m.ctx.Scene.SetColorOver(m.bgSquare, m.fillColor)
}

func (m *Module) nodeWorldPos(path string) [2]float64 {
	idx, ok := m.ctx.Assets.NodeIndex(path)
	if !ok {
		return [2]float64{-7.8, 6.84}
	}
	chain := []int{}
	for i := idx; i >= 0; i = m.ctx.Assets.Rig.Nodes[i].Parent {
		chain = append(chain, i)
	}
	w := kart.Identity()
	for i := len(chain) - 1; i >= 0; i-- {
		n := m.ctx.Assets.Rig.Nodes[chain[i]]
		w = w.Mul(kart.TRS(n.Pos[0], n.Pos[1], n.RotZ, n.Scale[0], n.Scale[1]))
	}
	return [2]float64{w.Tx, w.Ty}
}

func refOr(ctx *engine.Ctx, c kmdata.Component, field, fallback string) string {
	if c.Refs != nil && c.Refs[field] != "" {
		return c.Refs[field]
	}
	if p := ctx.Role(field); p != "" {
		return p
	}
	return fallback
}

func spriteOr(c kmdata.Component, field, fallback string) string {
	if c.Sprites != nil && c.Sprites[field] != "" {
		return c.Sprites[field]
	}
	return fallback
}

func numOr(c kmdata.Component, key string, fallback float64) float64 {
	if c.Nums != nil {
		if v, ok := c.Nums[key]; ok {
			return v
		}
	}
	return fallback
}

func colorField(c kmdata.Component, prefix string, fallback [4]float64) [4]float64 {
	if c.Nums == nil {
		return fallback
	}
	out := fallback
	for i, k := range []string{"r", "g", "b", "a"} {
		if v, ok := c.Nums[prefix+"."+k]; ok {
			out[i] = v
		}
	}
	return out
}

func nodeScale(as *kart.Assets, path string) [2]float64 {
	if idx, ok := as.NodeIndex(path); ok {
		return as.Rig.Nodes[idx].Scale
	}
	return [2]float64{1, 1}
}

func nodeColor(as *kart.Assets, path string, fallback [4]float64) [4]float64 {
	if idx, ok := as.NodeIndex(path); ok {
		c := as.Rig.Nodes[idx].Color
		if c != [4]float64{} {
			return c
		}
	}
	return fallback
}

func spriteNodeColors(as *kart.Assets) []renderColor {
	var out []renderColor
	for _, n := range as.Rig.Nodes {
		if n.Sprite == "" {
			continue
		}
		c := n.Color
		if c == [4]float64{} {
			c = [4]float64{1, 1, 1, 1}
		}
		out = append(out, renderColor{path: n.Path, base: c})
	}
	return out
}

func boolParam(e *riq.Entity, key string) bool { return e.Float(key, 0) != 0 }

func boolDefault(e *riq.Entity, key string, def bool) bool {
	if _, ok := e.Data[key]; ok {
		return boolParam(e, key)
	}
	return def
}

func mulColor(a, b [4]float64) [4]float64 {
	return [4]float64{a[0] * b[0], a[1] * b[1], a[2] * b[2], a[3] * b[3]}
}

func toNRGBA(c [4]float64) color.NRGBA {
	return color.NRGBA{
		R: uint8(clamp01(c[0])*255 + 0.5),
		G: uint8(clamp01(c[1])*255 + 0.5),
		B: uint8(clamp01(c[2])*255 + 0.5),
		A: uint8(clamp01(c[3])*255 + 0.5),
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

func lerp(a, b, t float64) float64 { return a + (b-a)*t }
