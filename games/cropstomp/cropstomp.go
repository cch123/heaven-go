// Package cropstomp ports Crop Stomp's marching farmer, veggie/mole two-step
// inputs, scrolling background, collection bag, and hit particle burst.
package cropstomp

import (
	"image/color"
	"math"
	"math/rand"
	"sort"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"hsdemo/engine"
	"hsdemo/kart"
	"hsdemo/kmdata"
	"hsdemo/riq"
)

const (
	stepDistance      = 2.115
	grassScrollPeriod = 8.595 // (1182 - 25 - 11) / 600 * 4.5, from grass sprite border/PPU.
	dotsScrollPeriod  = 19.2
	pickedRotSpeed    = 1080 * math.Pi / 180
)

var (
	defaultBG    = [4]float64{192.0 / 255, 240.0 / 255, 184.0 / 255, 1}
	defaultDots  = [4]float64{248.0 / 255, 248.0 / 255, 248.0 / 255, 1}
	defaultGrass = [4]float64{120.0 / 255, 248.0 / 255, 40.0 / 255, 1}
	white        = [4]float64{1, 1, 1, 1}
)

type vegEvent struct {
	beat, length float64
}

type moleEvent struct {
	beat float64
	mute bool
}

type endEvent struct {
	beat float64
	mute bool
}

type collectEvent struct {
	beat             float64
	threshold, limit int
	force            bool
	forceAmount      int
}

type bgEvent struct {
	beat, length         float64
	bgStart, bgEnd       [4]float64
	dotStart, dotEnd     [4]float64
	grassStart, grassEnd [4]float64
	ease                 int
}

type colorEase struct {
	beat, length float64
	from, to     [4]float64
	ease         int
}

type veggie struct {
	inst       *kart.Instance
	target     float64
	rootX      float64
	isMole     bool
	sprite     string
	veggieType int

	state        int // 0=waiting, 1=stomped/flying, 2=picked, -1=missed/boinked
	stompedBeat  float64
	pickedBeat   float64
	pickTarget   float64
	pickTime     float64
	deadAt       float64
	boinked      bool
	moleLaughing bool
}

type particle struct {
	x, y   float64
	vx, vy float64
	born   float64
	life   float64
	size   float64
}

type Module struct {
	ctx  *engine.Ctx
	proj kart.Aff

	scrolling    string
	farmer       string
	legs         string
	body         string
	grass        string
	dots         string
	bg           string
	veggieHolder string
	startPlant   string
	plantLeft    string
	plantRight   string
	plantLast    string

	veggieT *kart.Template
	moleT   *kart.Template

	curves        map[string]kmdata.Curve
	veggieSprites []string

	vegEvents     []vegEvent
	moleEvents    []moleEvent
	endEvents     []endEvent
	collectEvents []collectEvent
	bgEvents      []bgEvent

	veggies []*veggie
	parts   []particle

	marching   bool
	marchStart float64
	marchEnd   float64
	willNotHum bool
	farmerX    float64
	stepCount  int
	isStepping bool
	shakeBeat  float64
	shakeUntil float64

	bgEase    colorEase
	dotEase   colorEase
	grassEase colorEase

	collectedPlants int
	plantThreshold  int
	plantLimit      int
	lastVeggieType  int
}

func New() engine.Module { return &Module{} }

func (m *Module) ID() string { return "cropStomp" }

func (m *Module) Load(ctx *engine.Ctx) error {
	m.ctx = ctx
	if err := ctx.LoadAssets("cropStomp"); err != nil {
		return err
	}
	m.proj = kart.Translate(engine.ScreenW/2, engine.ScreenH/2).Mul(kart.Scale(54, -54))
	game := ctx.Assets.Extra.Components["game"]
	farmer := ctx.Assets.Extra.Components["farmer"]
	veggieComp := ctx.Assets.Extra.Components["veggie"]

	m.scrolling = refOr(ctx, game, "scrollingHolder", "ScrollingItems")
	m.farmer = refOr(ctx, game, "farmerTrans", "ScrollingItems/FarmerHolder")
	m.legs = refOr(ctx, game, "legsAnim", "ScrollingItems/FarmerHolder/Legs")
	m.body = refOr(ctx, game, "bodyAnim", "ScrollingItems/FarmerHolder/Legs/BodyHolder/BodyOffset/Body")
	m.grass = refOr(ctx, game, "grass", "Grass")
	m.dots = refOr(ctx, game, "Dots", "BG/Dots")
	m.bg = refOr(ctx, game, "BG", "BG")
	m.veggieHolder = refOr(ctx, game, "veggieHolder", "ScrollingItems/VeggieHolder")
	m.startPlant = farmer.Refs["startPlant"]
	m.plantLeft = farmer.Refs["plantLeftRef"]
	m.plantRight = farmer.Refs["plantRightRef"]
	m.plantLast = farmer.Refs["plantLastRef"]
	m.veggieSprites = append([]string(nil), veggieComp.SpriteArrays["veggieSprites"]...)
	if len(m.veggieSprites) == 0 {
		m.veggieSprites = []string{"veggie_0", "veggie_1", "veggie_2"}
	}
	m.curves = ctx.Assets.Extra.Curves
	m.veggieT = kart.NewTemplate(ctx.Assets, refOr(ctx, game, "baseVeggie", "ScrollingItems/Prefabs/Veggie"))
	m.moleT = kart.NewTemplate(ctx.Assets, refOr(ctx, game, "baseMole", "ScrollingItems/Prefabs/Mole"))

	m.bgEase = colorEase{from: defaultBG, to: defaultBG}
	m.dotEase = colorEase{from: defaultDots, to: defaultDots}
	m.grassEase = colorEase{from: defaultGrass, to: defaultGrass}
	m.plantThreshold = 8
	m.plantLimit = 80
	m.resetScene(0)
	return nil
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

func (m *Module) OnEvent(e *riq.Entity) {
	b := e.Beat
	switch e.Datamodel {
	case "cropStomp/start marching":
		m.ctx.At(b, func() { m.startMarching(b) })
	case "cropStomp/veggies":
		m.vegEvents = append(m.vegEvents, vegEvent{beat: b, length: e.Length})
	case "cropStomp/mole":
		mute := boolParam(e, "mute")
		m.moleEvents = append(m.moleEvents, moleEvent{beat: b, mute: mute})
		if !mute {
			m.ctx.SoundAtOff(b-2, "moleNyeh", 1, 0.134)
			m.ctx.SoundAtOff(b-1.5, "moleHeh1", 1, 0.05)
			m.ctx.SoundAtOff(b-1, "moleHeh2", 1, 0.061)
		}
	case "cropStomp/end":
		ev := endEvent{beat: b, mute: boolParamDefault(e, "mute", true)}
		m.endEvents = append(m.endEvents, ev)
		m.ctx.At(b, func() {
			m.marchEnd = ev.beat
			m.willNotHum = ev.mute
		})
	case "cropStomp/autoStomp":
		m.ctx.At(b, func() {
			if m.ctx.App.Autoplay {
				m.regularStomp(b, 0)
			}
		})
	case "cropStomp/changeBG":
		ev := bgEvent{
			beat: b, length: e.Length,
			bgStart:    colorParam(e, "start", defaultBG),
			bgEnd:      colorParam(e, "end", defaultBG),
			dotStart:   colorParam(e, "startDots", defaultDots),
			dotEnd:     colorParam(e, "endDots", defaultDots),
			grassStart: colorParam(e, "startGrass", defaultGrass),
			grassEnd:   colorParam(e, "endGrass", defaultGrass),
			ease:       int(e.Float("ease", 0)),
		}
		m.bgEvents = append(m.bgEvents, ev)
		m.ctx.At(b, func() { m.setBackground(ev) })
	case "cropStomp/plantCollect":
		ev := collectEvent{
			beat: b, threshold: intParam(e, "threshold", 8), limit: intParam(e, "limit", 80),
			force: boolParam(e, "force"), forceAmount: intParam(e, "forceAmount", 0),
		}
		m.collectEvents = append(m.collectEvents, ev)
		m.ctx.At(b, func() { m.setCollectThresholds(ev) })
	}
}

func (m *Module) Ready() {
	sort.Slice(m.vegEvents, func(i, j int) bool { return m.vegEvents[i].beat < m.vegEvents[j].beat })
	sort.Slice(m.moleEvents, func(i, j int) bool { return m.moleEvents[i].beat < m.moleEvents[j].beat })
	sort.Slice(m.endEvents, func(i, j int) bool { return m.endEvents[i].beat < m.endEvents[j].beat })
	sort.Slice(m.collectEvents, func(i, j int) bool { return m.collectEvents[i].beat < m.collectEvents[j].beat })
	sort.Slice(m.bgEvents, func(i, j int) bool { return m.bgEvents[i].beat < m.bgEvents[j].beat })
}

func (m *Module) OnSwitch(beat float64) {
	m.resetScene(beat)
	m.persistColor(beat)
	m.applyCollectBefore(beat)
	start := m.switchStartBeat(beat)
	m.startMarching(start)
	m.spawnSegment(start, m.ctx.NextSwitchBeat(start))
}

func (m *Module) Whiff(beat float64) {
	m.ctx.Scene.PlayState(m.body, "Crouch", beat, 0.5)
}

func (m *Module) Update(_, beat float64) {
	if m.ctx.ReleasedNow() && !m.ctx.ExpectingReleaseNow() {
		m.ctx.Scene.PlayState(m.body, "Pick", beat, 0.5)
	}
	for _, v := range m.veggies {
		if v.isMole && v.state == -1 && !v.moleLaughing && beat >= v.target+0.5 {
			v.inst.PlayState("Pivot/Sprite", "Chuckle", beat, 0.5)
			v.moleLaughing = true
		}
	}
	keep := m.veggies[:0]
	for _, v := range m.veggies {
		if v.deadAt > 0 && beat >= v.deadAt {
			continue
		}
		keep = append(keep, v)
	}
	m.veggies = keep

	parts := m.parts[:0]
	for _, p := range m.parts {
		if beat < p.born+p.life {
			parts = append(parts, p)
		}
	}
	m.parts = parts
}

func (m *Module) Draw(screen *ebiten.Image, _, beat float64) {
	bg := m.bgEase.at(beat)
	screen.Fill(toNRGBA(bg))
	m.applySceneColors(beat)
	m.applyScroll(beat)

	sc := m.ctx.Scene
	m.ctx.SampleScene(beat)
	holder, _ := sc.NodeWorld(m.veggieHolder)
	for _, v := range m.veggies {
		m.queueVeggie(v, beat, holder)
	}
	m.queueCollectedPlants(beat)
	sc.Draw(screen, m.proj)
	m.drawParticles(screen, beat)
}

func (m *Module) resetScene(beat float64) {
	sec := m.ctx.SecPerBeat(beat)
	m.ctx.Scene.PlayDefaultState(m.legs, beat, sec)
	m.ctx.Scene.PlayDefaultState(m.body, beat, sec)
	m.ctx.Scene.SetActive("ScrollingItems/Prefabs", false)
	m.ctx.Scene.SetActive(m.startPlant, false)
	m.ctx.Scene.SetActive(m.plantLeft, false)
	m.ctx.Scene.SetActive(m.plantRight, false)
	m.ctx.Scene.SetActive(m.plantLast, false)
	m.ctx.Scene.SetPosOver(m.scrolling, 0, 0)
	m.farmerX = 5
	m.marching = false
	m.marchStart = beat
	m.marchEnd = math.Inf(1)
	m.willNotHum = true
	m.stepCount = 0
	m.isStepping = false
	m.veggies = nil
	m.parts = nil
}

func (m *Module) switchStartBeat(beat float64) float64 {
	start := beat
	for _, e := range m.ctx.Entities() {
		if e.Datamodel == "cropStomp/start marching" && e.Beat >= beat {
			return e.Beat
		}
		if e.Datamodel == "cropStomp/start marching" && e.Beat < beat {
			start = e.Beat + math.Ceil((beat-e.Beat)/2)*2
		}
	}
	return start
}

func (m *Module) startMarching(beat float64) {
	if m.marching && math.Abs(m.marchStart-beat) < 1e-6 {
		return
	}
	m.marching = true
	m.marchStart = beat
	m.marchEnd, m.willNotHum = m.endAfter(beat)
	m.farmerX = 5
	m.stepCount = 0
	m.isStepping = false
	m.scheduleMarch(beat, math.Min(m.marchEnd, m.ctx.NextSwitchBeat(beat)))
}

func (m *Module) endAfter(beat float64) (float64, bool) {
	next := m.ctx.NextSwitchBeat(beat)
	for _, e := range m.endEvents {
		if e.beat >= beat && e.beat < next {
			return e.beat, e.mute
		}
	}
	return next, true
}

func (m *Module) scheduleMarch(start, end float64) {
	occupied := map[int64]bool{}
	for _, ev := range m.vegEvents {
		count := int(math.Ceil(ev.length+1)) / 2
		for i := 0; i < count; i++ {
			b := ev.beat + 2*float64(i)
			if b >= start && b < end {
				occupied[beatKey(b)] = true
			}
		}
	}
	for _, ev := range m.moleEvents {
		if ev.beat >= start && ev.beat < end {
			occupied[beatKey(ev.beat)] = true
		}
	}
	for b, i := start, 0; b < end+0.001; b, i = b+1, i+1 {
		bb, ii := b, i
		m.ctx.At(bb, func() { m.playMarchBeat(bb, ii) })
		if ii%2 == 0 && bb < end && !(m.willNotHum && bb >= m.marchEnd) {
			m.ctx.SoundAt(bb, "hmm", 1)
		}
		if ii%2 == 0 && !occupied[beatKey(bb)] {
			m.ctx.ScheduleInputNoScore(bb, func(state float64, _ engine.Judgment) {
				m.regularStomp(m.ctx.Beat(), state)
			}, func() {})
		}
	}
}

func beatKey(b float64) int64 { return int64(math.Round(b * 1000)) }

func (m *Module) playMarchBeat(beat float64, idx int) {
	if !m.marching || beat < m.marchStart || beat > m.marchEnd+0.001 {
		return
	}
	if idx%2 == 0 {
		if !m.isStepping {
			m.stepCount++
			state := "StepFront"
			if m.stepCount%2 == 0 {
				state = "StepBack"
			}
			m.ctx.Scene.PlayState(m.legs, state, beat, 0.5)
			m.isStepping = true
		}
		return
	}
	state := "LiftFront"
	if m.stepCount%2 != 0 {
		state = "LiftBack"
	}
	m.ctx.Scene.PlayState(m.legs, state, beat, 0.5)
	m.farmerX -= stepDistance
	m.isStepping = false
}

func (m *Module) regularStomp(beat, state float64) {
	if math.Abs(state) >= 1 {
		m.ctx.SoundPitch("stomp", 1, 1.35)
	} else {
		m.ctx.Sound("stomp")
	}
	m.stompMotion(beat)
}

func (m *Module) stompMotion(beat float64) {
	if !m.isStepping {
		m.stepCount++
	}
	state := "StompFront"
	if m.stepCount%2 == 0 {
		state = "StompBack"
	}
	m.ctx.Scene.PlayState(m.legs, state, beat, 0.5)
	m.ctx.Scene.PlayState(m.body, "Stomp", beat, 0.5)
	m.isStepping = true
	m.shakeBeat = beat
	m.shakeUntil = beat + 0.5
}

func (m *Module) spawnSegment(start, end float64) {
	for _, ev := range m.vegEvents {
		if start > ev.beat+ev.length {
			continue
		}
		count := int(math.Ceil(ev.length+1)) / 2
		for i := 0; i < count; i++ {
			target := ev.beat + 2*float64(i)
			if target >= start && target < end {
				m.spawnVeggie(target, start, false)
			}
		}
	}
	for _, ev := range m.moleEvents {
		if ev.beat >= start && ev.beat < end {
			m.spawnVeggie(ev.beat, start, true)
		}
	}
}

func (m *Module) spawnVeggie(target, start float64, mole bool) {
	t := m.veggieT
	if mole {
		t = m.moleT
	}
	if t == nil {
		return
	}
	inst := t.NewInstance()
	if mole {
		inst.PlayDefaultState("Pivot/Sprite", target-2, 0.5)
	}
	typ := int(rand.New(rand.NewSource(int64(target*1000) + 17)).Intn(len(m.veggieSprites)))
	sprite := m.veggieSprites[typ]
	if mole {
		sprite = "mole_1"
	}
	v := &veggie{
		inst: inst, target: target, rootX: (target - start) * -stepDistance / 2,
		isMole: mole, sprite: sprite, veggieType: typ, pickTime: 1,
	}
	if mole {
		v.pickTime = 1.5
	}
	inst.SetSprite("Pivot/Sprite", sprite)
	m.veggies = append(m.veggies, v)
	m.ctx.ScheduleInput(target, func(state float64, _ engine.Judgment) {
		m.stompVeggie(v, m.ctx.Beat(), state)
	}, func() { v.state = -1 })
}

func (m *Module) stompVeggie(v *veggie, beat, state float64) {
	if v.state != 0 {
		return
	}
	m.regularStomp(beat, state)
	m.emitHit(v, beat)
	v.state = 1
	v.stompedBeat = beat
	v.pickTarget = v.target + 1
	if v.isMole {
		v.pickTarget = v.target + 0.5
		v.inst.PlayState("Pivot/Sprite", "Idle", beat, 0.5)
	} else {
		m.ctx.SoundAt(v.pickTarget-0.5, "veggieOh", 1)
	}
	m.ctx.ScheduleInputRelease(v.pickTarget, func(state float64, _ engine.Judgment) {
		m.pickHit(v, m.ctx.Beat(), state)
	}, func() {
		v.state = -1
		if !v.isMole {
			m.ctx.Sound("veggieMiss")
		}
	})
}

func (m *Module) pickHit(v *veggie, beat, state float64) {
	m.ctx.Scene.PlayState(m.body, "Pick", beat, 0.5)
	if v.state != 1 {
		return
	}
	if math.Abs(state) >= 1 {
		v.state = -1
		v.boinked = true
		v.pickedBeat = beat
		m.ctx.PlayCommon("miss")
		return
	}
	v.state = 2
	v.pickedBeat = beat
	if v.isMole {
		m.ctx.Sound("GEUH")
		v.deadAt = beat + v.pickTime
	} else {
		m.ctx.Sound("veggieKay")
		v.deadAt = beat + v.pickTime
		m.ctx.At(v.deadAt, func() { m.collectPlant(v.veggieType) })
	}
}

func (m *Module) queueVeggie(v *veggie, beat float64, holder kart.Aff) {
	pivotY := -0.84
	if v.isMole {
		pivotY = -0.75
	}
	x, y := holder.Tx+v.rootX, holder.Ty
	scaleX, scaleY := 1.0, 1.0
	rot := 0.0
	switch v.state {
	case 1:
		u := clamp01((beat - v.stompedBeat) / maxf(0.001, v.pickTarget-v.stompedBeat+0.1))
		p := kart.EvalBezier(m.curveFor(v, false), u)
		x = holder.Tx + v.rootX + p[0]
		y = p[1] - pivotY
		scaleX = lerp(0.5, 1, clamp01((beat-v.stompedBeat)/0.5))
	case 2:
		u := clamp01((beat - v.pickedBeat) / v.pickTime)
		p := kart.EvalBezier(m.curveFor(v, true), u)
		x, y = p[0], p[1]-pivotY
		secs := m.ctx.BeatToTime(beat) - m.ctx.BeatToTime(v.pickedBeat)
		dir := 1.0
		if v.isMole {
			dir = -1
		}
		rot = dir * pickedRotSpeed * secs
		if !v.isMole {
			s := math.Min(1.5-u, 1)
			if s < 0 {
				s = 0
			}
			scaleX, scaleY = s, s
		}
	case -1:
		if v.boinked {
			u := clamp01((beat - v.pickedBeat) / 1)
			p := kart.EvalBezier(m.curveFor(v, false), u)
			x = holder.Tx + v.rootX + p[0]
			y = p[1] - pivotY
			secs := m.ctx.BeatToTime(beat) - m.ctx.BeatToTime(v.pickedBeat)
			dir := -1.0
			if v.isMole {
				dir = 1
			}
			rot = dir * pickedRotSpeed * secs
		}
	}
	v.inst.Offset = [2]float64{x, y}
	v.inst.Rot = rot
	v.inst.Scale = [2]float64{scaleX, scaleY}
	v.inst.SetSprite("Pivot/Sprite", v.sprite)
	v.inst.Queue(m.ctx.Scene, beat, kart.Identity(), 0)
}

func (m *Module) curveFor(v *veggie, picked bool) kmdata.Curve {
	if picked {
		if v.isMole {
			return m.curves["moleCurve"]
		}
		return m.curves["pickCurve"]
	}
	if v.isMole {
		return m.curves["mole.curve"]
	}
	return m.curves["veggie.curve"]
}

func (m *Module) emitHit(v *veggie, beat float64) {
	r := rand.New(rand.NewSource(int64(v.target*4096) + 23))
	for i := 0; i < 14; i++ {
		ang := -math.Pi/2 + (r.Float64()-0.5)*1.8
		spd := 1.5 + r.Float64()*2.4
		m.parts = append(m.parts, particle{
			x: 2.92 + (r.Float64()-0.5)*0.25, y: -3.79 + (r.Float64()-0.5)*0.12,
			vx: math.Cos(ang) * spd, vy: -math.Sin(ang) * spd,
			born: beat, life: 0.35 + r.Float64()*0.25, size: 0.045 + r.Float64()*0.045,
		})
	}
}

func (m *Module) drawParticles(screen *ebiten.Image, beat float64) {
	c := toNRGBA(m.grassEase.at(beat))
	for _, p := range m.parts {
		u := clamp01((beat - p.born) / p.life)
		t := m.ctx.BeatToTime(beat) - m.ctx.BeatToTime(p.born)
		x := p.x + p.vx*t
		y := p.y + p.vy*t - 6*t*t
		sx, sy := m.proj.Apply(x, y)
		cc := c
		cc.A = uint8(float64(cc.A) * (1 - u))
		vector.DrawFilledCircle(screen, float32(sx), float32(sy), float32(p.size*54*(1-u*0.4)), cc, true)
	}
}

func (m *Module) applyScroll(beat float64) {
	if !m.marching {
		return
	}
	scrollBeat := math.Min(beat, m.marchEnd)
	if scrollBeat < m.marchStart {
		scrollBeat = m.marchStart
	}
	scroll := (scrollBeat - m.marchStart) * stepDistance / 2
	m.ctx.Scene.SetPosOver(m.scrolling, scroll, 0)
	m.ctx.Scene.SetPosOver(m.farmer, m.farmerX, -1.9)
	m.ctx.Scene.SetPosOver(m.grass, math.Mod(scroll, grassScrollPeriod), -5)
	m.ctx.Scene.SetPosOver(m.bg, math.Mod(scroll, dotsScrollPeriod), 1.34)
}

func (m *Module) applySceneColors(beat float64) {
	dots := m.dotEase.at(beat)
	grass := m.grassEase.at(beat)
	m.ctx.Scene.SetColorOver(m.dots, dots)
	m.ctx.Scene.SetColorOver(m.grass, grass)
}

func (m *Module) setBackground(ev bgEvent) {
	m.bgEase = colorEase{beat: ev.beat, length: ev.length, from: ev.bgStart, to: ev.bgEnd, ease: ev.ease}
	m.dotEase = colorEase{beat: ev.beat, length: ev.length, from: ev.dotStart, to: ev.dotEnd, ease: ev.ease}
	m.grassEase = colorEase{beat: ev.beat, length: ev.length, from: ev.grassStart, to: ev.grassEnd, ease: ev.ease}
}

func (m *Module) persistColor(beat float64) {
	m.bgEase = colorEase{from: defaultBG, to: defaultBG}
	m.dotEase = colorEase{from: defaultDots, to: defaultDots}
	m.grassEase = colorEase{from: defaultGrass, to: defaultGrass}
	for _, ev := range m.bgEvents {
		if ev.beat >= beat {
			break
		}
		m.setBackground(ev)
	}
}

func (m *Module) setCollectThresholds(ev collectEvent) {
	m.plantThreshold = ev.threshold
	m.plantLimit = ev.limit
	if m.plantThreshold <= 0 {
		m.plantThreshold = 8
	}
	if m.plantLimit <= 0 {
		m.plantLimit = 80
	}
	if ev.force {
		m.collectedPlants = ev.forceAmount
	}
}

func (m *Module) applyCollectBefore(beat float64) {
	m.plantThreshold, m.plantLimit = 8, 80
	for _, ev := range m.collectEvents {
		if ev.beat >= beat {
			break
		}
		m.setCollectThresholds(ev)
	}
}

func (m *Module) collectPlant(typ int) {
	if m.collectedPlants > m.plantLimit {
		return
	}
	if m.collectedPlants <= m.plantLimit-m.plantThreshold {
		m.lastVeggieType = typ
	}
	m.collectedPlants++
}

func (m *Module) queueCollectedPlants(beat float64) {
	sc := m.ctx.Scene
	sc.SetActive(m.startPlant, m.collectedPlants >= m.plantThreshold)
	holder, ok := sc.NodeWorld("ScrollingItems/FarmerHolder/Legs/BodyHolder/BodyOffset/Body/CollectedVeggieHolder")
	if !ok || m.collectedPlants < m.plantThreshold*2 {
		return
	}
	for i := 0; i <= m.collectedPlants-(m.plantThreshold*2) && i <= m.plantLimit-(m.plantThreshold*2); i += m.plantThreshold {
		real := i / m.plantThreshold
		sprite := "plants2"
		if real%2 != 0 {
			sprite = "plants3"
		}
		if i == m.plantLimit-(m.plantThreshold*2) {
			sprite = m.veggieSprites[m.lastVeggieType%len(m.veggieSprites)]
		}
		world := holder.Mul(kart.Translate(0, float64(real)*0.5+0.1))
		sc.Queue(kart.ExtraSprite{Sprite: sprite, World: world, Layer: 0, Order: -real - 2, Tint: white})
	}
}

func (e colorEase) at(beat float64) [4]float64 {
	if e.length <= 0 {
		return e.to
	}
	u := clamp01((beat - e.beat) / e.length)
	return [4]float64{
		engine.Ease(e.ease, e.from[0], e.to[0], u),
		engine.Ease(e.ease, e.from[1], e.to[1], u),
		engine.Ease(e.ease, e.from[2], e.to[2], u),
		engine.Ease(e.ease, e.from[3], e.to[3], u),
	}
}

func boolParam(e *riq.Entity, key string) bool { return e.Float(key, 0) != 0 }

func boolParamDefault(e *riq.Entity, key string, def bool) bool {
	if _, ok := e.Data[key]; !ok {
		return def
	}
	return boolParam(e, key)
}

func intParam(e *riq.Entity, key string, def int) int { return int(e.Float(key, float64(def))) }

func colorParam(e *riq.Entity, key string, def [4]float64) [4]float64 {
	v, ok := e.Data[key]
	if !ok {
		return def
	}
	m, ok := v.(map[string]any)
	if !ok {
		return def
	}
	return [4]float64{num(m["r"], def[0]), num(m["g"], def[1]), num(m["b"], def[2]), num(m["a"], def[3])}
}

func num(v any, def float64) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	default:
		return def
	}
}

func toNRGBA(c [4]float64) color.NRGBA {
	return color.NRGBA{R: byte255(c[0]), G: byte255(c[1]), B: byte255(c[2]), A: byte255(c[3])}
}

func byte255(v float64) uint8 {
	if v < 0 {
		v = 0
	}
	if v > 1 {
		v = 1
	}
	return uint8(v*255 + 0.5)
}

func lerp(a, b, u float64) float64 { return a + (b-a)*u }

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func maxf(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
