// Package packingpests ports Packing Pests' candy/spider throws, wrong-action
// windows, worker hand slides, curtain slides, sign calls, and catch/miss
// feedback from Assets/Scripts/Games/PackingPests.
package packingpests

import (
	"fmt"
	"image/color"
	"math"
	"sort"

	"github.com/hajimehoshi/ebiten/v2"

	"hsdemo/engine"
	"hsdemo/kart"
	"hsdemo/kmdata"
	"hsdemo/riq"
)

const (
	animScale = 0.5

	actionBasic = 0
	actionAlt   = 3

	objNone = iota
	objCandyThrow
	objSpiderThrow
	objCandyCatch
	objCandyBarely
	objSpiderBarely
	objCandyWrong
	objSpiderWrong
)

type throwPath struct {
	from, to string
	dur      float64
	height   float64
}

type moveEvt struct {
	beat, length     float64
	player, all, alt bool
	anim             string
	ease             int
}

type curtainEvt struct {
	beat, length float64
	anim         string
	ease         int
}

type Module struct {
	ctx  *engine.Ctx
	proj kart.Aff

	candyT, spiderT *kart.Template
	paths           map[string]throwPath
	objects         []*throwObject

	candyRoot, spiderRoot string
	boxfront              string
	hand, lower, upper    string
	sign                  string
	spiderCrawl           string
	spiderAnim            string
	curtain               string
	workerPlayer          string
	workersAlt            []string

	moving, movingAlt bool
	sliding           bool
	moveStart         float64
	moveLength        float64
	moveEase          int
	moveAnim          string
	curtainAnim       string

	workerEvents  []moveEvt
	curtainEvents []curtainEvt
}

func New() engine.Module { return &Module{} }

func (m *Module) ID() string { return "packingPests" }

func (m *Module) Load(ctx *engine.Ctx) error {
	m.ctx = ctx
	if err := ctx.LoadAssets("packingPests"); err != nil {
		return err
	}
	m.proj = kart.Translate(engine.ScreenW/2, engine.ScreenH/2).Mul(kart.Scale(54, -54))

	m.candyRoot = ctx.Role("Candy")
	m.spiderRoot = ctx.Role("Spider")
	m.boxfront = ctx.Role("boxfront")
	m.hand = ctx.Role("handAnim")
	m.lower = ctx.Role("lowerHandAnim")
	m.upper = ctx.Role("upperHandAnim")
	m.sign = ctx.Role("signAnim")
	m.spiderCrawl = ctx.Role("spiderCrawlAnim")
	m.spiderAnim = ctx.Role("spiderAnim")
	m.curtain = ctx.Role("curtainAnim")
	m.workerPlayer = ctx.Role("HandAnimPlayer")
	for i := 1; i <= 8; i++ {
		m.workersAlt = append(m.workersAlt, ctx.Role(fmt.Sprintf("HandAnim%d", i)))
	}

	m.candyT = kart.NewTemplate(ctx.Assets, m.candyRoot)
	m.spiderT = kart.NewTemplate(ctx.Assets, m.spiderRoot)
	if m.candyT == nil || m.spiderT == nil {
		return fmt.Errorf("packingPests templates missing: candy=%q spider=%q", m.candyRoot, m.spiderRoot)
	}
	m.paths = loadObjectPaths(ctx.Assets.Extra.Components["game"])
	if len(m.paths) == 0 {
		return fmt.Errorf("packingPests objectPaths missing")
	}

	// Candy/Spider are serialized scene objects used as Instantiate templates.
	// Keeping them visible would duplicate every thrown object at the origin.
	ctx.Scene.SetActive(m.candyRoot, false)
	ctx.Scene.SetActive(m.spiderRoot, false)
	m.playIdle(0)
	return nil
}

func loadObjectPaths(comp kmdata.Component) map[string]throwPath {
	out := map[string]throwPath{}
	for _, item := range comp.Lists["objectPaths"] {
		pts := item.Items["positions"]
		if len(pts) < 2 {
			continue
		}
		name := item.Strs["name"]
		out[name] = throwPath{
			from:   pts[0].Refs["target"],
			to:     pts[1].Refs["target"],
			dur:    pts[0].Nums["duration"],
			height: pts[0].Nums["height"],
		}
	}
	return out
}

func (m *Module) OnEvent(e *riq.Entity) {
	b := e.Beat
	switch e.Datamodel {
	case "packingPests/pattern":
		m.pattern(b)
	case "packingPests/pattern2":
		m.pattern2(b, boolParam(e, "pitch", false))
	case "packingPests/patternArrange1":
		m.throwCandy(b)
		m.throwSpider(b + 1.5)
	case "packingPests/patternArrange2":
		if boolParam(e, "warning", true) {
			m.ding(b-0.5, true)
		}
		m.throwCandy(b + 0.5)
		m.throwSpider(b + 2)
	case "packingPests/candy":
		m.throwCandy(b)
	case "packingPests/spider":
		m.throwSpider(b)
	case "packingPests/ding":
		m.ding(b, boolParam(e, "hmm", true))
	case "packingPests/worker":
		ev := moveEvt{
			beat: b, length: lengthDefault(e, 4),
			player: boolParam(e, "player", false),
			all:    boolParam(e, "all", true),
			alt:    boolParam(e, "allAlt", false),
			anim:   sideAnim(intParam(e, "inout", 0)),
			ease:   intParam(e, "ease", 0),
		}
		m.workerEvents = append(m.workerEvents, ev)
		m.ctx.At(b, func() { m.applyWorkerMove(ev) })
	case "packingPests/curtainNew":
		ev := curtainEvt{
			beat: b, length: lengthDefault(e, 4),
			anim: sideAnim(intParam(e, "inout", 0)),
			ease: intParam(e, "ease", 0),
		}
		m.curtainEvents = append(m.curtainEvents, ev)
		m.ctx.At(b, func() { m.applyCurtainMove(ev) })
	case "packingPests/curtains":
		// SlideCurtain is empty in the Unity source; keep the event as a no-op.
	}
}

func (m *Module) Ready() {
	sort.SliceStable(m.workerEvents, func(i, j int) bool { return m.workerEvents[i].beat < m.workerEvents[j].beat })
	sort.SliceStable(m.curtainEvents, func(i, j int) bool { return m.curtainEvents[i].beat < m.curtainEvents[j].beat })
}

func (m *Module) OnSwitch(beat float64) {
	m.playIdle(beat)
	m.catchUpMoveState(beat)
}

func (m *Module) playIdle(beat float64) {
	sec := m.ctx.SecPerBeat(beat)
	for _, p := range []string{m.curtain, m.workerPlayer} {
		m.ctx.Scene.PlayDefaultState(p, beat, sec)
	}
	for _, p := range m.workersAlt {
		m.ctx.Scene.PlayDefaultState(p, beat, sec)
	}
	for _, p := range []string{m.hand, m.lower, m.upper, m.sign, m.spiderCrawl, m.spiderAnim} {
		m.ctx.Scene.PlayDefaultState(p, beat, sec)
	}
}

func (m *Module) catchUpMoveState(beat float64) {
	for _, ev := range m.workerEvents {
		if ev.beat > beat {
			break
		}
		m.applyWorkerMove(ev)
	}
	for _, ev := range m.curtainEvents {
		if ev.beat > beat {
			break
		}
		m.applyCurtainMove(ev)
		if ev.length > 0 && beat >= ev.beat+ev.length {
			m.sliding = false
		}
	}
}

func (m *Module) Whiff(beat float64) { m.WhiffAction(beat, actionBasic) }

func (m *Module) WhiffAction(beat float64, action int) {
	switch action {
	case actionAlt:
		m.ctx.Sound("SE_SHIWAKE_EN_SWING_CATCH_A")
		m.ctx.Sound("SE_SHIWAKE_EN_SWING_CATCH_B")
		m.playArm(m.upper, "CatchOut", beat)
		m.playArm(m.lower, "CatchOut", beat)
	default:
		m.ctx.Sound("SE_SHIWAKE_EN_SWING")
		m.playArm(m.upper, "HitOut", beat)
	}
}

func (m *Module) Update(_ float64, beat float64) {
	if m.moving {
		m.playWorkerFrozen(m.workerPlayer, beat)
	}
	if m.movingAlt {
		for _, p := range m.workersAlt {
			m.playWorkerFrozen(p, beat)
		}
	}
	if m.sliding {
		u := normalized(beat, m.moveStart, m.moveLength)
		v := engine.Ease(m.moveEase, 0, 1, u)
		m.ctx.Scene.PlayFrozen(m.curtain, m.curtainAnim, v)
		if u >= 1 {
			m.sliding = false
		}
	}

	alive := m.objects[:0]
	for _, o := range m.objects {
		if !o.expired(beat) {
			alive = append(alive, o)
		}
	}
	m.objects = alive
}

func (m *Module) playWorkerFrozen(path string, beat float64) {
	v := engine.Ease(m.moveEase, 0, 1, normalized(beat, m.moveStart, m.moveLength))
	m.ctx.Scene.PlayFrozen(path, m.moveAnim, v)
}

func (m *Module) Draw(screen *ebiten.Image, _ float64, beat float64) {
	screen.Fill(color.RGBA{0xba, 0xb8, 0x9d, 255})
	sc := m.ctx.Scene
	m.ctx.SampleScene(beat)
	for _, o := range m.objects {
		o.queue(beat)
	}
	sc.Draw(screen, m.proj)
}

func (m *Module) pattern(beat float64) {
	m.throwCandy(beat)
	m.throwSpider(beat + 2)
}

func (m *Module) pattern2(beat float64, pitched bool) {
	m.ctx.At(beat, func() { m.ctx.Scene.PlayState(m.sign, "Message00", beat, animScale) })
	pitch := 1.0
	offset := 0.013
	if pitched {
		ratio := m.ctx.BPMAt(beat) / 136
		pitch = ratio
		if ratio > 0 {
			offset = 0.013 / ratio
		}
	}
	m.ctx.SoundAtPitchOff(beat, "SE_SHIWAKE_EN_VOICE_READY1", 1, pitch, 0)
	m.ctx.SoundAtPitchOff(beat+0.25, "SE_SHIWAKE_EN_VOICE_READY2", 1, pitch, offset)
	m.ctx.SoundAtPitchOff(beat+0.5, "SE_SHIWAKE_EN_VOICE_READY3", 1, pitch, 0)
	m.throwCandy(beat)
	m.throwSpider(beat + 1.5)
	m.throwSpider(beat + 2.5)
}

func (m *Module) throwCandy(beat float64) {
	m.ctx.SoundAt(beat, "SE_SHIWAKE_EN_BALL_OUT", 1)
	m.ctx.At(beat, func() {
		m.ctx.Scene.PlayState(m.hand, "Beat", beat, animScale)
		o := newThrowObject(m, m.candyT, false, beat)
		m.objects = append(m.objects, o)
		o.schedule()
	})
}

func (m *Module) throwSpider(beat float64) {
	m.ctx.SoundAt(beat, "SE_SHIWAKE_EN_INSECT_OUT", 1)
	m.ctx.At(beat, func() {
		o := newThrowObject(m, m.spiderT, true, beat)
		m.objects = append(m.objects, o)
		o.schedule()
	})
}

func (m *Module) ding(beat float64, hmm bool) {
	m.ctx.SoundAt(beat, "SE_SHIWAKE_EN_VOICE_REST_A", 1)
	if hmm {
		m.ctx.SoundAt(beat+0.5, "SE_SHIWAKE_EN_VOICE_REST_B", 1)
	}
	m.ctx.At(beat, func() { m.ctx.Scene.PlayState(m.sign, "Message01", beat, animScale) })
}

func (m *Module) applyWorkerMove(ev moveEvt) {
	// Unity applies these bools in three independent if-blocks. allAlt wins if
	// combined with all/player, so keep the same ordering instead of collapsing
	// the parameters into a single enum.
	if ev.player {
		m.moving = true
		m.movingAlt = false
	}
	if ev.all {
		m.moving = true
		m.movingAlt = true
	}
	if ev.alt {
		m.moving = false
		m.movingAlt = true
	}
	m.moveStart = ev.beat
	m.moveLength = ev.length
	m.moveAnim = ev.anim
	m.moveEase = ev.ease
}

func (m *Module) applyCurtainMove(ev curtainEvt) {
	m.sliding = true
	m.moveStart = ev.beat
	m.moveLength = ev.length
	m.curtainAnim = ev.anim
	m.moveEase = ev.ease
}

func (m *Module) setBoxOrder(order int) {
	m.ctx.Scene.SetOrderOver(m.boxfront, order)
}

func (m *Module) playArm(path, state string, beat float64) {
	m.ctx.Scene.PlayState(path, state, beat, animScale)
}

func normalized(beat, start, length float64) float64 {
	if length <= 0 {
		return 1
	}
	return (beat - start) / length
}

func sideAnim(side int) string {
	if side == 0 {
		return "Enter"
	}
	return "Exit"
}

func boolParam(e *riq.Entity, key string, def bool) bool {
	if _, ok := e.Data[key]; !ok {
		return def
	}
	return e.Float(key, 0) != 0
}

func intParam(e *riq.Entity, key string, def int) int {
	return int(e.Float(key, float64(def)))
}

func lengthDefault(e *riq.Entity, def float64) float64 {
	if e.Length > 0 {
		return e.Length
	}
	return def
}

type throwObject struct {
	mod       *Module
	inst      *kart.Instance
	spider    bool
	state     int
	startBeat float64
	dead      bool

	canJust  bool
	canWrong bool

	rotation  bool
	rotBase   float64
	rotBeat   float64
	rotSpeed  float64
	killAfter float64
}

func newThrowObject(m *Module, tmpl *kart.Template, spider bool, beat float64) *throwObject {
	state := objCandyThrow
	if spider {
		state = objSpiderThrow
	}
	o := &throwObject{
		mod: m, inst: tmpl.NewInstance(), spider: spider,
		state: state, startBeat: beat, canJust: true, canWrong: true,
		rotBeat: beat, rotSpeed: 180, killAfter: beat + 6,
	}
	return o
}

func (o *throwObject) schedule() {
	target := o.startBeat + 1
	correct := actionAlt
	wrong := actionBasic
	if o.spider {
		correct, wrong = actionBasic, actionAlt
	}
	o.mod.ctx.ScheduleInputActionCond(target, correct,
		func() bool { return o.canJust && !o.dead },
		func(state float64, _ engine.Judgment) { o.just(state) },
		func() { o.miss() })
	wrongIn := o.mod.ctx.ScheduleInputActionCond(target, wrong,
		func() bool { return o.canWrong && !o.dead },
		func(state float64, _ engine.Judgment) { o.wrongInput(state) },
		func() {})
	// ThrowObject uses ScheduleUserInput for the wrong action: the window
	// catches real player mistakes but autoplay must never trigger it, and it
	// reports ScoreMiss manually from WrongInput.
	wrongIn.NoScore = true
	wrongIn.NoAutoplay = true
}

func (o *throwObject) just(state float64) {
	o.canWrong = false
	beat := o.mod.ctx.Beat()
	o.startBeat = beat

	if state >= 1 || state <= -1 {
		o.mod.setBoxOrder(-2)
		if o.spider {
			o.changeState(objSpiderBarely, beat)
			o.mod.ctx.Scene.PlayState(o.mod.spiderAnim, "Enter", beat, animScale)
			o.mod.playArm(o.mod.upper, "HitOut", beat)
		} else {
			o.changeState(objCandyBarely, beat)
			o.mod.playArm(o.mod.upper, "CatchOut", beat)
			o.mod.playArm(o.mod.lower, "CatchOut", beat)
		}
		o.mod.ctx.Sound("SE_SHIWAKE_EN_OSII")
		o.killAfter = beat + 2
		return
	}

	o.mod.setBoxOrder(1)
	if o.spider {
		o.changeState(objNone, beat)
		o.mod.ctx.Scene.PlayState(o.mod.spiderCrawl, "Hit", beat, animScale)
		o.mod.playArm(o.mod.upper, "HitJust", beat)
		o.mod.ctx.Sound("SE_SHIWAKE_EN_INSECT_ATTACK_A")
		o.mod.ctx.Sound("SE_SHIWAKE_EN_INSECT_ATTACK_B")
		o.killAfter = beat + 2
		return
	}

	o.changeState(objNone, beat)
	o.mod.ctx.Sound("SE_SHIWAKE_EN_BALL_CATCH_A")
	o.mod.ctx.Sound("SE_SHIWAKE_EN_BALL_CATCH_B")
	o.mod.ctx.Sound("SE_SHIWAKE_EN_BALL_CATCH_C")
	o.mod.playArm(o.mod.upper, "CatchCandy", beat)
	o.mod.playArm(o.mod.lower, "CatchJust00", beat)
	o.mod.ctx.At(beat+0.5, func() {
		o.mod.playArm(o.mod.upper, "CatchJust02", beat+0.5)
		o.mod.playArm(o.mod.lower, "CatchJust01", beat+0.5)
		o.changeState(objCandyCatch, beat+0.5)
	})
	o.killAfter = beat + 2
}

func (o *throwObject) wrongInput(float64) {
	o.canJust = false
	if o.state == objCandyCatch || o.state == objCandyBarely {
		return
	}
	beat := o.mod.ctx.Beat()
	target := o.startBeat + 1

	if o.spider {
		o.mod.setBoxOrder(-2)
		o.changeState(objNone, beat)
		o.mod.ctx.Sound("SE_SHIWAKE_EN_MISS_INSECT_CATCH_A")
		o.mod.ctx.Sound("SE_SHIWAKE_EN_MISS_INSECT_CATCH_B")
		o.mod.playArm(o.mod.upper, "CatchBug", beat)
		o.mod.playArm(o.mod.lower, "CatchBug", beat)
		o.mod.ctx.At(target+0.5, func() {
			o.mod.ctx.Scene.PlayState(o.mod.spiderAnim, "Enter", target+0.5, animScale)
			o.mod.playArm(o.mod.upper, "CatchMiss", target+0.5)
			o.mod.playArm(o.mod.lower, "CatchMiss", target+0.5)
			o.changeState(objSpiderWrong, target+0.5)
		})
	} else {
		o.mod.setBoxOrder(-2)
		o.mod.playArm(o.mod.upper, "HitOut", beat)
		o.mod.ctx.Sound("SE_SHIWAKE_EN_MISS_BALL_ATTACK_A")
		o.mod.ctx.Sound("SE_SHIWAKE_EN_MISS_BALL_ATTACK_B")
		o.mod.ctx.PlayCommon("miss")
		o.changeState(objCandyWrong, beat)
	}
	o.mod.ctx.ScoreMiss()
	o.killAfter = o.startBeat + 6
}

func (o *throwObject) miss() {
	beat := o.mod.ctx.Beat()
	if o.spider {
		o.mod.ctx.Sound("SE_SHIWAKE_EN_MISS_INSECT_THROUGH_A")
		o.mod.ctx.Sound("SE_SHIWAKE_EN_MISS_INSECT_THROUGH_B")
		o.mod.playArm(o.mod.upper, "Damage", beat)
		o.mod.playArm(o.mod.lower, "Damage", beat)
	} else {
		o.mod.ctx.Sound("SE_SHIWAKE_EN_MISS_BALL_THROUGH_A")
		o.mod.ctx.Sound("SE_SHIWAKE_EN_MISS_BALL_THROUGH_B")
		o.mod.ctx.Scene.PlayState(o.mod.hand, "Through", beat, animScale)
		o.mod.playArm(o.mod.upper, "Through", beat)
		o.mod.playArm(o.mod.lower, "Through", beat)
	}
	o.killAfter = o.startBeat + 6
}

func (o *throwObject) expired(beat float64) bool {
	return o.dead || beat >= o.killAfter
}

func (o *throwObject) changeState(state int, beat float64) {
	o.rotBase = o.rotationAt(beat)
	o.rotBeat = beat
	o.state = state
	switch state {
	case objCandyCatch:
		o.rotSpeed = -60
	case objSpiderWrong:
		o.rotSpeed = 0
	default:
		o.rotSpeed = 180
	}
}

func (o *throwObject) rotationAt(beat float64) float64 {
	dir := -1.0
	if o.rotation {
		dir = 1
	}
	return o.rotBase + dir*o.rotSpeed*(beat-o.rotBeat)*math.Pi/180
}

func (o *throwObject) queue(beat float64) {
	p := o.mod.evalPath(o.pathName(), math.Max(beat, o.startBeat), o.startBeat)
	o.inst.Offset = p
	o.inst.SetRot("", o.rotationAt(beat))
	o.inst.Queue(o.mod.ctx.Scene, beat, kart.Identity(), 0)
}

func (o *throwObject) pathName() string {
	switch o.state {
	case objCandyThrow:
		return "candyThrow"
	case objSpiderThrow:
		return "spiderThrow"
	case objCandyCatch:
		return "candyCatch"
	case objCandyBarely:
		return "candyBarely"
	case objSpiderBarely:
		return "spiderBarely"
	case objCandyWrong:
		return "candyWrong"
	case objSpiderWrong:
		return "spiderWrong"
	default:
		return "None"
	}
}

func (m *Module) evalPath(name string, beat, startBeat float64) [2]float64 {
	p, ok := m.paths[name]
	if !ok {
		return [2]float64{}
	}
	from := m.nodePos(p.from)
	to := m.nodePos(p.to)
	u := 0.0
	if p.dur > 0 {
		u = (beat - startBeat) / p.dur
	}
	if u < 0 {
		u = 0
	}
	x := from[0] + (to[0]-from[0])*u
	y := from[1] + (to[1]-from[1])*u
	yMul := u*2 - 1
	y += (1 - yMul*yMul) * p.height
	return [2]float64{x, y}
}

func (m *Module) nodePos(path string) [2]float64 {
	if a, ok := m.ctx.Scene.NodeWorld(path); ok {
		return [2]float64{a.Tx, a.Ty}
	}
	return [2]float64{}
}
