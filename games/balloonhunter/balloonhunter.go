// Package balloonhunter ports Balloon Hunter's bird calls, layered hunter/bird
// faces, three balloon timings, five-balloon rock throw, background animals,
// rock barely curve, and balloon pop particle burst.
package balloonhunter

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
	animScale = 0.5

	animalRabbit = 0
	animalBoar   = 1
	animalMoose  = 2

	dirLeft  = 0
	dirRight = 1
)

type balloonKind int

const (
	balloonSlow balloonKind = iota
	balloonFast
	balloonFive
)

type bopEvt struct {
	beat, length float64
	auto, bop    bool
	emote        bool
}

type bgAnimalEvt struct {
	beat, length float64
	typ, dir     int
	startY, endY float64
}

type Module struct {
	ctx  *engine.Ctx
	proj kart.Aff

	hunter, bird string
	rock         string
	rockSmear    string
	cloudHolder  string

	slowPath, fastPath, fivePath string
	bgAnimalPath                 string

	slowT, fastT, fiveT *kart.Template
	bgAnimalT           *kart.Template

	hunterBop, birdBop       bool
	preparing, tossPreparing bool
	bopExpression            string
	lastPulse                int

	rockActive bool
	rockStart  float64
	rockNormal [2]float64
	rockCurve  kmdata.Curve

	bops       []bopEvt
	bgAnimals  []bgAnimalEvt
	balloons   []*balloon
	liveAnimal []*bgAnimal
	particles  []popParticle

	rng *rand.Rand
}

func New() engine.Module {
	return &Module{
		hunterBop:     true,
		birdBop:       true,
		bopExpression: "Neutral",
		lastPulse:     math.MinInt,
		rng:           rand.New(rand.NewSource(1)),
	}
}

func (m *Module) ID() string { return "balloonHunter" }

func (m *Module) Load(ctx *engine.Ctx) error {
	m.ctx = ctx
	if err := ctx.LoadAssets("balloonHunter"); err != nil {
		return err
	}
	m.proj = kart.Translate(engine.ScreenW/2, engine.ScreenH/2).Mul(kart.Scale(54, -54))

	m.hunter = roleOr(ctx, "hunterAnim", "Hunter")
	m.bird = roleOr(ctx, "birdAnim", "Bird")
	m.rock = roleOr(ctx, "rock", "Rock")
	m.rockSmear = roleOr(ctx, "rockSmear", "RockSmear")
	m.cloudHolder = "BG/CloudHolder"
	m.slowPath = roleOr(ctx, "slowBalloon", "BalloonSlow")
	m.fastPath = roleOr(ctx, "fastBalloon", "BalloonFast")
	m.fivePath = roleOr(ctx, "balloonFive", "BalloonFive")
	m.bgAnimalPath = roleOr(ctx, "bgAnimal", "BG/AnimalsBG")

	m.slowT = kart.NewTemplate(ctx.Assets, m.slowPath)
	m.fastT = kart.NewTemplate(ctx.Assets, m.fastPath)
	m.fiveT = kart.NewTemplate(ctx.Assets, m.fivePath)
	m.bgAnimalT = kart.NewTemplate(ctx.Assets, m.bgAnimalPath)

	if c, ok := ctx.Assets.Extra.Curves["rockMissCurve"]; ok {
		m.rockCurve = c
	} else {
		m.rockCurve = ctx.Assets.Extra.Curves["game.rockMissCurve"]
	}
	m.rockNormal = nodePos(ctx.Assets, m.rock)

	for _, p := range []string{m.slowPath, m.fastPath, m.fivePath, m.bgAnimalPath, m.rock} {
		ctx.Scene.SetActive(p, false)
	}
	m.initAnimators(0)
	return nil
}

func roleOr(ctx *engine.Ctx, key, fallback string) string {
	if p := ctx.Role(key); p != "" {
		return p
	}
	return fallback
}

func nodePos(as *kart.Assets, path string) [2]float64 {
	for _, n := range as.Rig.Nodes {
		if n.Path == path {
			return n.Pos
		}
	}
	return [2]float64{}
}

func (m *Module) initAnimators(beat float64) {
	sec := m.ctx.SecPerBeat(beat)
	m.ctx.Scene.PlayDefaultState(m.hunter, beat, sec)
	m.ctx.Scene.PlayDefaultState(m.bird, beat, sec)
	m.ctx.Scene.PlayDefaultState(m.cloudHolder, beat, sec)
	m.ctx.Scene.PlayDefaultState(m.rockSmear, beat, sec)
	m.playHunterFace("Neutral", beat)
	m.playBirdFace("Neutral", beat)
}

func (m *Module) OnEvent(e *riq.Entity) {
	switch e.Datamodel {
	case "balloonHunter/bop":
		ev := bopEvt{
			beat: e.Beat, length: e.Length,
			auto:  boolDefault(e, "auto", true),
			bop:   boolDefault(e, "toggle", true),
			emote: boolDefault(e, "emote", true),
		}
		if ev.length <= 0 {
			ev.length = 1
		}
		m.bops = append(m.bops, ev)
		m.ctx.At(ev.beat, func() {
			m.hunterBop, m.birdBop = ev.auto, ev.auto
			m.preparing, m.tossPreparing = false, false
			if m.bopExpression != "Neutral" {
				m.resetBopExpression(ev.beat)
			}
		})
		if ev.bop {
			for i := 0; i < int(ev.length); i++ {
				b := ev.beat + float64(i)
				m.ctx.At(b, func() { m.bopNow(b, ev.emote) })
			}
		}
	case "balloonHunter/prepare":
		b := e.Beat
		m.ctx.At(b, func() { m.prepare(b) })
	case "balloonHunter/balloonSlow":
		b := e.Beat
		m.soundAtLead(b, "tweetN_base", m.soundLength("tweetN_base")-0.05837)
		m.ctx.At(b, func() { m.queueBalloonSlow(b) })
	case "balloonHunter/balloonFast":
		b := e.Beat
		m.ctx.SoundAt(b, "tweetN_fast1", 1)
		m.ctx.SoundAt(b+0.75, "tweetN_fast2", 1)
		m.ctx.At(b, func() { m.sendBalloonFast(b) })
	case "balloonHunter/balloonBoth":
		b := e.Beat
		m.ctx.SoundAt(b, "tweetN_both", 1)
		m.ctx.SoundAt(b+0.75, "tweetN_both", 1)
		m.ctx.At(b, func() { m.sendBalloonBoth(b) })
	case "balloonHunter/balloonFive":
		b := e.Beat
		moose := boolParam(e, "moose")
		m.ctx.SoundAt(b, "tweetN_slow", 1)
		m.ctx.SoundAt(b+1, "tweetN_slow", 1)
		m.soundAtLead(b+4, "charge", m.soundLength("charge"))
		m.ctx.At(b, func() { m.sendBalloonFive(b, moose) })
	case "balloonHunter/bgAnimal":
		ev := bgAnimalEvt{
			beat: e.Beat, length: e.Length,
			typ:    int(e.Float("type", animalRabbit)),
			dir:    int(e.Float("direction", dirLeft)),
			startY: e.Float("startY", 0),
			endY:   e.Float("endY", 0),
		}
		if ev.length <= 0 {
			ev.length = 16
		}
		m.bgAnimals = append(m.bgAnimals, ev)
		m.ctx.At(ev.beat, func() { m.spawnBgAnimal(ev) })
	}
}

func (m *Module) Ready() {
	sort.SliceStable(m.bops, func(i, j int) bool { return m.bops[i].beat < m.bops[j].beat })
	sort.SliceStable(m.bgAnimals, func(i, j int) bool { return m.bgAnimals[i].beat < m.bgAnimals[j].beat })
}

func (m *Module) OnSwitch(beat float64) {
	m.initAnimators(beat)
	m.lastPulse = int(math.Floor(beat))
}

func (m *Module) Whiff(beat float64) {
	if m.tossPreparing {
		m.toss(beat, true)
		return
	}
	m.playHunterBody("Miss", beat)
	m.playHunterFace("Blow", beat)
	m.ctx.Sound("blow")
	m.preparing = false
}

func (m *Module) Update(t, beat float64) {
	m.updateBeatPulse(beat)
	m.updateRock(beat)
	m.invalidateEarlyFivePresses()
	m.pruneBalloons(beat)
	m.pruneAnimals(beat)
	m.pruneParticles(t)
}

func (m *Module) Draw(screen *ebiten.Image, t, beat float64) {
	screen.Fill(color.NRGBA{0xd5, 0xef, 0xff, 0xff})
	sc := m.ctx.Scene
	m.ctx.SampleScene(beat)
	for _, a := range m.liveAnimal {
		a.queue(sc, beat)
	}
	for _, b := range m.balloons {
		b.queue(sc, beat)
	}
	m.queueParticles(sc, t)
	sc.Draw(screen, m.proj)
}

func (m *Module) updateBeatPulse(beat float64) {
	p := int(math.Floor(beat + 1e-6))
	if p <= m.lastPulse {
		return
	}
	for b := m.lastPulse + 1; b <= p; b++ {
		if b >= 0 {
			m.autoPulse(float64(b))
		}
	}
	m.lastPulse = p
}

func (m *Module) autoPulse(beat float64) {
	if m.hunterBop && m.hunterIdle(beat) && !m.preparing {
		m.playHunterBody("Bop", beat)
		m.playHunterFace(m.bopExpression, beat)
		if m.bopExpression != "Neutral" {
			m.resetBopExpression(beat)
		}
	}
	if m.birdBop && m.birdIdle(beat) {
		m.playBirdBody("Bop", beat)
		m.playBirdFace(m.bopExpression, beat)
	}
	for _, a := range m.liveAnimal {
		a.bop(beat)
	}
}

func (m *Module) bopNow(beat float64, emote bool) {
	face := "Neutral"
	if emote {
		face = m.bopExpression
	}
	if m.hunterIdle(beat) && !m.preparing {
		m.playHunterBody("Bop", beat)
		m.playHunterFace(face, beat)
	}
	if m.birdIdle(beat) {
		m.playBirdBody("Bop", beat)
		m.playBirdFace(face, beat)
	}
	if emote && m.bopExpression != "Neutral" {
		m.resetBopExpression(beat)
	}
}

func (m *Module) resetBopExpression(beat float64) {
	m.ctx.At(beat+1, func() { m.bopExpression = "Neutral" })
}

func (m *Module) hunterIdle(beat float64) bool {
	st, playing := m.ctx.Scene.StateInfo(m.hunter, beat)
	return st == "" || st == "Idle" || !playing
}

func (m *Module) birdIdle(beat float64) bool {
	st, playing := m.ctx.Scene.StateInfo(m.bird, beat)
	return st == "" || st == "Idle" || st == "Neutral" || !playing
}

func (m *Module) playHunterBody(state string, beat float64) {
	m.ctx.Scene.PlayState(m.hunter, state, beat, animScale)
}

func (m *Module) playHunterFace(state string, beat float64) {
	m.ctx.Scene.PlayStateLayer(m.hunter+":face", m.hunter, state, beat, animScale)
}

func (m *Module) playBirdBody(state string, beat float64) {
	m.ctx.Scene.PlayState(m.bird, state, beat, animScale)
}

func (m *Module) playBirdFace(state string, beat float64) {
	m.ctx.Scene.PlayStateLayer(m.bird+":face", m.bird, state, beat, animScale)
}

func (m *Module) prepare(beat float64) {
	if m.preparing {
		return
	}
	m.playHunterBody("Prepare", beat)
	m.playHunterFace("Hold", beat)
	m.preparing = true
}

func (m *Module) tossPrepare(beat float64) {
	m.playHunterBody("HunterTossPrep", beat)
	m.playHunterFace("Prep Toss", beat)
	m.preparing = true
	m.tossPreparing = true
}

func (m *Module) toss(beat float64, miss bool) {
	m.playHunterBody("HunterToss", beat)
	if miss {
		m.playHunterFace("Shock", beat)
		m.bopExpression = "Shock"
	} else {
		m.playHunterFace("Determined", beat)
		m.bopExpression = "Determined"
		m.ctx.Scene.PlayState(m.rockSmear, "RockSmear", beat, animScale)
	}
	m.preparing = false
	m.tossPreparing = false
}

func (m *Module) birdCall(beat float64) {
	m.playBirdBody("Bop", beat)
	m.playBirdFace("Call", beat)
}

func (m *Module) birdFlap(beat float64) {
	m.playBirdBody("Flap", beat)
	m.playBirdFace("CallBig", beat)
}

func (m *Module) birdScared(beat float64) {
	m.playBirdBody("Scared", beat)
	m.playBirdFace("CallScared", beat)
}

func (m *Module) birdCover(beat float64) {
	m.playBirdBody("Cover", beat)
}

func (m *Module) queueBalloonSlow(beat float64) {
	m.sendBalloonSlow(beat)
	m.ctx.At(beat, func() { m.birdCall(beat) })
	m.ctx.At(beat+1, func() { m.prepare(beat + 1) })
}

func (m *Module) sendBalloonSlow(beat float64) {
	m.spawnBalloon(balloonSlow, beat, 3, false, false)
}

func (m *Module) sendBalloonFast(beat float64) {
	m.spawnBalloon(balloonFast, beat, 2.5, false, false)
	m.ctx.At(beat, func() { m.birdFlap(beat) })
	m.ctx.At(beat+0.75, func() {
		m.birdFlap(beat + 0.75)
		m.prepare(beat + 0.75)
	})
}

func (m *Module) sendBalloonBoth(beat float64) {
	m.sendBalloonSlow(beat)
	m.sendBalloonFast(beat)
}

func (m *Module) sendBalloonFive(beat float64, moose bool) {
	m.spawnBalloon(balloonFive, beat+2, 4, true, moose)
	m.ctx.At(beat, func() { m.birdScared(beat) })
	m.ctx.At(beat+1, func() { m.birdScared(beat + 1) })
	m.ctx.At(beat+2, func() {
		m.tossPrepare(beat + 2)
		m.birdCover(beat + 2)
	})
}

func (m *Module) spawnBalloon(kind balloonKind, startBeat, speed float64, isFive, moose bool) {
	b := newBalloon(m, kind, startBeat, speed, isFive, moose)
	if b == nil {
		return
	}
	m.balloons = append(m.balloons, b)
	target := startBeat + speed - 1
	if isFive {
		target = startBeat + speed - 2
	}
	b.input = m.ctx.ScheduleInputCond(target, func() bool { return b.canHit },
		func(state float64, _ engine.Judgment) { b.pop(state) },
		func() { b.miss() })
}

func (m *Module) invalidateEarlyFivePresses() {
	if !m.ctx.PressedNow() || m.ctx.ExpectingPressNow() {
		return
	}
	for _, b := range m.balloons {
		if b.isFive && !b.resolved && !b.dead {
			b.canHit = false
		}
	}
}

func (m *Module) pruneBalloons(beat float64) {
	dst := m.balloons[:0]
	for _, b := range m.balloons {
		if !b.shouldDestroy(beat) {
			dst = append(dst, b)
		}
	}
	m.balloons = dst
}

func (m *Module) doRockBarely(beat float64) {
	m.rockActive = true
	m.rockStart = beat
	m.ctx.Scene.SetActive(m.rock, true)
}

func (m *Module) updateRock(beat float64) {
	if !m.rockActive {
		return
	}
	u := (beat - m.rockStart) / 0.5
	if u > 1 {
		m.rockActive = false
		m.ctx.Scene.SetPosOver(m.rock, m.rockNormal[0], m.rockNormal[1])
		m.ctx.Scene.SetActive(m.rock, false)
		return
	}
	p := kart.EvalBezier(m.rockCurve, clamp01(u))
	m.ctx.Scene.SetActive(m.rock, true)
	m.ctx.Scene.SetPosOver(m.rock, p[0], p[1])
}

func (m *Module) spawnBgAnimal(ev bgAnimalEvt) {
	if m.bgAnimalT == nil {
		return
	}
	a := &bgAnimal{
		mod: m, inst: m.bgAnimalT.NewInstance(),
		startBeat: ev.beat, length: ev.length,
		typ: ev.typ, right: ev.dir == dirRight,
		startY: ev.startY, endY: ev.endY,
	}
	a.start()
	m.liveAnimal = append(m.liveAnimal, a)
}

func (m *Module) pruneAnimals(beat float64) {
	dst := m.liveAnimal[:0]
	for _, a := range m.liveAnimal {
		if a.length > 0 && (beat-a.startBeat)/a.length <= 1 {
			dst = append(dst, a)
		}
	}
	m.liveAnimal = dst
}

func (m *Module) soundLength(name string) float64 {
	pcm := m.ctx.Assets.Sounds[name]
	if len(pcm) == 0 {
		return 0
	}
	return float64(len(pcm)) / float64(engine.SampleRate*4)
}

func (m *Module) soundAtLead(beat float64, name string, leadSec float64) {
	if leadSec <= 0 {
		m.ctx.SoundAt(beat, name, 1)
		return
	}
	m.ctx.SoundAt(beat-leadSec/m.ctx.SecPerBeat(beat), name, 1)
}

type balloon struct {
	mod  *Module
	inst *kart.Instance
	kind balloonKind

	input *engine.Input

	startBeat float64
	speed     float64
	moveClip  string

	isFive bool
	moose  bool

	canHit       bool
	resolved     bool
	dead         bool
	missed       bool
	mooseFalling bool
	destroyBeat  float64

	popEffect   string
	popParticle string
}

func newBalloon(m *Module, kind balloonKind, startBeat, speed float64, isFive, moose bool) *balloon {
	var tmpl *kart.Template
	compName := ""
	moveClip := ""
	switch kind {
	case balloonSlow:
		tmpl, compName, moveClip = m.slowT, "slowBalloon", "Balloon/SlowBalloonMove"
	case balloonFast:
		tmpl, compName, moveClip = m.fastT, "fastBalloon", "Balloon/FastBalloonMove"
	case balloonFive:
		tmpl, compName, moveClip = m.fiveT, "balloonFive", "Balloon/BalloonFiveMove"
	}
	if tmpl == nil {
		return nil
	}
	comp := m.ctx.Assets.Extra.Components[compName]
	b := &balloon{
		mod: m, inst: tmpl.NewInstance(), kind: kind,
		startBeat: startBeat, speed: speed, moveClip: moveClip,
		isFive: isFive, moose: moose, canHit: true,
		popEffect: comp.Refs["popEffect"], popParticle: comp.Refs["popParticle"],
	}
	if isFive {
		b.inst.SetActive("MooseBody", moose)
	}
	return b
}

func (b *balloon) queue(sc *kart.SceneInst, beat float64) {
	if b.dead {
		return
	}
	if b.mooseFalling {
		b.inst.Queue(sc, beat, kart.Identity(), 0)
		return
	}
	if !b.missed {
		u := (beat - b.startBeat) / b.speed
		b.inst.PlayNormalized("", b.moveClip, clamp01(u))
	}
	b.inst.Queue(sc, beat, kart.Identity(), 0)
}

func (b *balloon) shouldDestroy(beat float64) bool {
	if b.dead {
		return true
	}
	if b.mooseFalling {
		return b.destroyBeat > 0 && beat >= b.destroyBeat
	}
	return b.speed > 0 && (beat-b.startBeat)/b.speed > 1
}

func (b *balloon) pop(state float64) {
	b.resolved = true
	beat := b.mod.ctx.Beat()
	ng := state >= 1 || state <= -1
	if b.isFive {
		b.mod.toss(beat, ng)
	} else {
		b.mod.playHunterBody("Shoot", beat)
		b.mod.playHunterFace("Blow", beat)
		b.mod.ctx.Sound("blow")
	}
	b.mod.preparing = false

	if ng {
		b.mod.ctx.Sound("miss")
		if !b.isFive {
			b.inst.PlayState("", "Miss", beat, animScale)
			b.missed = true
			b.mod.bopExpression = "Sad"
		} else {
			b.mod.doRockBarely(beat)
		}
		if b.moose {
			b.scheduleMooseRaspberry()
		}
		return
	}

	b.mod.ctx.SoundPitch("pop", 1, 0.9+b.mod.rng.Float64()*0.2)
	if b.moose {
		b.mod.ctx.Sound("moose_uhoh")
	}
	if b.popEffect != "" {
		b.mod.ctx.Scene.PlayState(b.popEffect, "Pop", beat, animScale)
	}
	if b.popParticle != "" {
		b.mod.spawnPopParticles(b.popParticle, b.mod.ctx.Time())
	}
	if !b.isFive {
		b.mod.bopExpression = "Happy"
	}
	if !b.moose {
		b.dead = true
		return
	}
	b.inst.PlayStateLayer("moose", "", "MooseFall", beat, animScale)
	b.mooseFalling = true
	b.destroyBeat = b.startBeat + 4
}

func (b *balloon) miss() {
	b.resolved = true
	if b.isFive {
		b.mod.bopExpression = "Shock"
	} else {
		b.mod.bopExpression = "Sad"
	}
	if b.mod.preparing {
		beat := b.mod.ctx.Beat()
		b.mod.playHunterBody("Bop", beat)
		b.mod.playHunterFace("Neutral", beat)
		b.mod.preparing = false
	}
	if b.moose {
		b.scheduleMooseRaspberry()
	}
}

func (b *balloon) scheduleMooseRaspberry() {
	at := b.startBeat + 3
	b.mod.ctx.At(at, func() {
		if b.dead {
			return
		}
		b.inst.PlayStateLayer("moose", "", "MooseRaspberry", at, animScale)
		b.mod.ctx.Sound("moose_raspberry")
	})
}

type bgAnimal struct {
	mod  *Module
	inst *kart.Instance

	startBeat float64
	length    float64
	typ       int
	right     bool
	startY    float64
	endY      float64

	animalRoot string
}

func (a *bgAnimal) start() {
	a.inst.Scale = [2]float64{0.3, 0.3}
	if !a.right {
		a.inst.Scale[0] = -0.3
	}
	a.inst.SetActive("MooseBody", false)
	a.inst.SetActive("BunnyBody", false)
	a.inst.SetActive("BoarHead", false)
	switch a.typ {
	case animalBoar:
		a.inst.PlayState("", "Boar", a.startBeat, animScale)
		a.inst.SetActive("BoarHead", true)
		a.animalRoot = "BoarHead"
	case animalMoose:
		a.inst.PlayState("", "Moose", a.startBeat, animScale)
		a.inst.SetActive("MooseBody", true)
		a.animalRoot = "MooseBody"
	default:
		a.inst.PlayState("", "Rabbit", a.startBeat, animScale)
		a.inst.SetActive("BunnyBody", true)
		a.animalRoot = "BunnyBody"
	}
}

func (a *bgAnimal) bop(beat float64) {
	if a.animalRoot != "" {
		a.inst.PlayState(a.animalRoot, "Bop", beat, animScale)
	}
}

func (a *bgAnimal) queue(sc *kart.SceneInst, beat float64) {
	if a.length <= 0 {
		return
	}
	u := clamp01((beat - a.startBeat) / a.length)
	startX, endX := 10.45, -10.45
	if a.right {
		startX, endX = -10.45, 10.45
	}
	a.inst.Offset = [2]float64{
		lerp(startX, endX, u),
		lerp(a.startY, a.endY, u),
	}
	a.inst.Queue(sc, beat, kart.Identity(), 0)
}

type popParticle struct {
	birth  float64
	root   string
	sprite string
	x, y   float64
	vx, vy float64
	size   float64
	rot    float64
}

func (m *Module) spawnPopParticles(root string, birth float64) {
	const count = 5
	roots := []string{root}
	// Unity's ParticleSystem.Play() defaults to withChildren=true. The five-shot
	// middle pop has a child ParticleSystem on the opposite side of the moose, so
	// both bursts need to run even though the serialized field points at the parent.
	if root == "PopEffectMiddleL" {
		roots = append(roots, "PopEffectMiddleL/PopEffectMiddleR")
	}
	for _, particleRoot := range roots {
		for i := 0; i < count; i++ {
			dir := m.rng.Float64() * math.Pi * 2
			speed := 8 + m.rng.Float64()
			size := 0.4 + m.rng.Float64()*0.2
			m.particles = append(m.particles, popParticle{
				birth: birth, root: particleRoot, sprite: popSprites[i%len(popSprites)],
				x: 0, y: 0,
				vx: math.Cos(dir) * speed, vy: math.Sin(dir) * speed,
				size: size, rot: dir,
			})
		}
	}
}

var popSprites = []string{
	"balloonHunterCharacters_33",
	"balloonHunterCharacters_34",
	"balloonHunterCharacters_35",
}

func (m *Module) pruneParticles(t float64) {
	dst := m.particles[:0]
	for _, p := range m.particles {
		if t-p.birth <= 2 {
			dst = append(dst, p)
		}
	}
	m.particles = dst
}

func (m *Module) queueParticles(sc *kart.SceneInst, t float64) {
	for _, p := range m.particles {
		age := t - p.birth
		if age < 0 || age > 2 {
			continue
		}
		base, ok := sc.NodeWorld(p.root)
		if !ok {
			continue
		}
		x, y := p.vx*age, p.vy*age
		world := base.Mul(kart.Translate(x, y)).Mul(kart.TRS(0, 0, p.rot+age, p.size, p.size))
		sc.Queue(kart.ExtraSprite{
			Sprite: p.sprite,
			World:  world,
			Order:  0,
			Tint:   [4]float64{1, 1, 1, 1},
		})
	}
}

func boolParam(e *riq.Entity, key string) bool {
	return e.Float(key, 0) != 0
}

func boolDefault(e *riq.Entity, key string, def bool) bool {
	d := 0.0
	if def {
		d = 1
	}
	return e.Float(key, d) != 0
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
