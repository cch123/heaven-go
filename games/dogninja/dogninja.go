// Package dogninja ports Dog Ninja's object throws, slice/barely branches,
// falling halves, bop/prepare flow, sign fly-in, and cue sounds.
package dogninja

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
	dirLeft = iota
	dirRight
	dirBoth
	dirAlternate
)

const (
	typeRandom = iota
	typeApple
	typeBroccoli
	typeCarrot
	typeCucumber
	typePepper
	typePotato
	typeBone
	typePan
	typeTire
	typeAirBatter
	typeKarateka
	typeIaiGiriGaiden
	typeThumpFarm
	typeBattingShow
	typeMeatGrinder
	typeIdol
	typeTacoBell
)

var bgColor = color.NRGBA{R: 0x8a, G: 0xc7, B: 0xff, A: 0xff}

type bopEvt struct {
	beat, length float64
	auto, bop    bool
}

type throwEvt struct {
	beat              float64
	direction         int
	diffObjs          bool
	shouldPrepare     bool
	muteThrow         bool
	typ, typeL, typeR int
	uid               int
}

type cutEvt struct {
	beat, length float64
	sound        bool
	text         string
}

type thrownObject struct {
	ev        throwEvt
	eventIdx  int
	fromLeft  bool
	direction int
	typ       int
	shouldSfx bool

	active     bool
	dead       bool
	barely     bool
	barelyBeat float64
	objPos     [3]float64
	rot        float64
}

type halfObject struct {
	startBeat float64
	startTime float64
	objPos    [3]float64
	fromLeft  bool
	leftHalf  bool
	sprite    string
	scale     [2]float64
	order     int
	rotSpeed  float64
	bpmMod    float64
	dead      bool
}

type Module struct {
	ctx  *engine.Ctx
	proj kart.Aff

	bops     []bopEvt
	throws   []throwEvt
	cuts     []cutEvt
	prepares []float64
	heres    []float64

	dogPath  string
	birdPath string
	textPath string

	objectScale [2]float64
	objectOrder int
	leftScale   [2]float64
	leftOrder   int
	rightScale  [2]float64
	rightOrder  int

	fullSprites  []string
	leftSprites  []string
	rightSprites []string
	curves       map[string]kmdata.Curve

	lastSideL    bool
	autoBop      bool
	queuePrepare bool
	preparing    bool
	lastPulse    float64

	objects []*thrownObject
	halves  []*halfObject
	spawned map[int]bool
}

func New() engine.Module {
	return &Module{autoBop: true, lastPulse: math.Inf(-1), spawned: map[int]bool{}}
}

func (m *Module) ID() string { return "dogNinja" }

func (m *Module) Load(ctx *engine.Ctx) error {
	m.ctx = ctx
	if err := ctx.LoadAssets("dogNinja"); err != nil {
		return err
	}
	if err := ctx.Assets.ApplyTexts(); err != nil {
		return err
	}
	m.proj = kart.Translate(engine.ScreenW/2, engine.ScreenH/2).Mul(kart.Scale(54, -54))
	m.dogPath = roleOr(ctx, "DogAnim", "Dog")
	m.birdPath = roleOr(ctx, "BirdAnim", "BirdFull")
	m.textPath = roleOr(ctx, "CutEverythingText", "BirdFull/CutEverythingSign/textstuffs")
	m.curves = ctx.Assets.Extra.Curves

	m.objectScale, m.objectOrder = nodeWorldScaleOrder(ctx, roleOr(ctx, "ObjectBase", "Holders/ObjectHolder/ThrownObject"))
	m.leftScale, m.leftOrder = nodeWorldScaleOrder(ctx, "Holders/HalvesHolder/HalvesLeftBase")
	m.rightScale, m.rightOrder = nodeWorldScaleOrder(ctx, "Holders/HalvesHolder/HalvesRightBase")

	if game := ctx.Assets.Extra.Components["game"]; game.SpriteArrays != nil {
		m.fullSprites = appendObjectTypePlaceholder(game.RefArrays["ObjectTypes"], game.SpriteArrays["ObjectTypes"])
	}
	throw := ctx.Assets.Extra.Components["throwObject"]
	m.leftSprites = append(m.leftSprites, throw.SpriteArrays["objectLeftHalves"]...)
	m.rightSprites = append(m.rightSprites, throw.SpriteArrays["objectRightHalves"]...)

	ctx.Scene.PlayDefaultState(m.dogPath, 0, ctx.SecPerBeat(0))
	ctx.Scene.PlayDefaultState(m.birdPath, 0, ctx.SecPerBeat(0))
	return nil
}

func roleOr(ctx *engine.Ctx, key, fallback string) string {
	if p := ctx.Role(key); p != "" {
		return p
	}
	return fallback
}

func nodeWorldScaleOrder(ctx *engine.Ctx, path string) ([2]float64, int) {
	idx, ok := ctx.Assets.NodeIndex(path)
	if !ok {
		return [2]float64{1, 1}, 0
	}
	chain := []int{}
	for i := idx; i >= 0; i = ctx.Assets.Rig.Nodes[i].Parent {
		chain = append(chain, i)
	}
	w := kart.Identity()
	for i := len(chain) - 1; i >= 0; i-- {
		n := ctx.Assets.Rig.Nodes[chain[i]]
		w = w.Mul(kart.TRS(n.Pos[0], n.Pos[1], n.RotZ, n.Scale[0], n.Scale[1]))
	}
	n := ctx.Assets.Rig.Nodes[idx]
	return [2]float64{math.Hypot(w.A, w.B), math.Hypot(w.C, w.D)}, n.Order
}

func appendObjectTypePlaceholder(refs, sprites []string) []string {
	out := append([]string{}, sprites...)
	if len(refs) > 0 && refs[0] == "" {
		out = append([]string{""}, out...)
	}
	return out
}

func (m *Module) OnEvent(e *riq.Entity) {
	switch e.Datamodel {
	case "dogNinja/Bop":
		m.bops = append(m.bops, bopEvt{
			beat: e.Beat, length: e.Length, auto: boolParam(e, "auto"), bop: boolDefault(e, "toggle", true),
		})
	case "dogNinja/Prepare":
		m.prepares = append(m.prepares, e.Beat)
	case "dogNinja/ThrowObject":
		uid := len(m.throws)
		m.throws = append(m.throws, throwEvt{
			beat: e.Beat, direction: int(e.Float("direction", dirAlternate)),
			diffObjs: boolParam(e, "diffObjs"), shouldPrepare: boolDefault(e, "shouldPrepare", true),
			muteThrow: boolParam(e, "muteThrow"), typ: int(e.Float("type", typeRandom)),
			typeL: int(e.Float("typeL", typeRandom)), typeR: int(e.Float("typeR", typeRandom)), uid: uid,
		})
	case "dogNinja/CutEverything":
		m.cuts = append(m.cuts, cutEvt{
			beat: e.Beat, length: e.Length, sound: boolDefault(e, "toggle", true), text: stringParam(e, "text", "Cut everything!"),
		})
	case "dogNinja/HereWeGo":
		m.heres = append(m.heres, e.Beat)
	}
}

func (m *Module) Ready() {
	sort.SliceStable(m.bops, func(i, j int) bool { return m.bops[i].beat < m.bops[j].beat })
	sort.SliceStable(m.throws, func(i, j int) bool { return m.throws[i].beat < m.throws[j].beat })
	sort.SliceStable(m.cuts, func(i, j int) bool { return m.cuts[i].beat < m.cuts[j].beat })
	sort.Float64s(m.prepares)
	sort.Float64s(m.heres)

	for _, ev := range m.bops {
		ev := ev
		m.ctx.At(ev.beat, func() { m.autoBop = ev.auto })
		if ev.bop {
			for i := 0; i < int(ev.length); i++ {
				b := ev.beat + float64(i)
				m.ctx.At(b, func() { m.playDog("Bop", b, 0.5) })
			}
		}
	}
	for _, b := range m.prepares {
		beat := b
		m.ctx.At(beat, func() { m.doPrepare(beat) })
	}
	for i, ev := range m.throws {
		i, ev := i, ev
		m.scheduleThrow(i, ev)
	}
	for _, ev := range m.cuts {
		ev := ev
		m.ctx.At(ev.beat, func() { m.cutEverything(ev) })
		m.ctx.At(ev.beat+ev.length, func() {
			m.ctx.Scene.PlayState(m.birdPath, "FlyOut", ev.beat+ev.length, m.ctx.SecPerBeat(ev.beat+ev.length))
		})
	}
	for _, b := range m.heres {
		beat := b
		m.ctx.SoundAt(beat, "here", 1)
		m.ctx.SoundAt(beat+0.5, "we", 1)
		m.ctx.SoundAt(beat+1, "go", 1)
	}
}

func (m *Module) OnSwitch(beat float64) {
	if beat <= 0 {
		m.objects = nil
		m.halves = nil
		m.spawned = map[int]bool{}
		m.lastSideL = false
		m.autoBop = true
		m.queuePrepare = false
		m.preparing = false
		m.lastPulse = math.Inf(-1)
	}
	m.ctx.Scene.PlayDefaultState(m.dogPath, beat, m.ctx.SecPerBeat(beat))
	m.ctx.Scene.PlayDefaultState(m.birdPath, beat, m.ctx.SecPerBeat(beat))
	for i, ev := range m.throws {
		if beat >= ev.beat && beat < ev.beat+2.45 {
			m.spawnThrow(i, ev)
		}
		if beat > ev.beat && beat < ev.beat+1 && ev.shouldPrepare {
			m.playDog("Prepare", beat, 0.5)
			m.preparing = true
		}
	}
}

func (m *Module) Whiff(beat float64) {
	slice := "WhiffRight"
	if rand.Intn(2) == 0 {
		slice = "WhiffLeft"
	}
	m.playDog(slice, beat, 0.5)
	m.ctx.Sound("whiff")
	m.stopPrepare()
}

func (m *Module) Update(t, beat float64) {
	if m.queuePrepare && !m.preparing && m.dogCanPrepare(beat) {
		m.playDog("Prepare", beat, 0.5)
		m.preparing = true
		m.queuePrepare = false
	}
	if p := math.Floor(beat); p > m.lastPulse {
		m.lastPulse = p
		if m.autoBop && !m.preparing && !m.queuePrepare && m.dogCanBop(p) {
			m.playDog("Bop", p, 0.5)
		}
	}
	for _, o := range m.objects {
		o.update(m, beat)
	}
	for _, h := range m.halves {
		h.update(m, t, beat)
	}
	m.objects = filterObjects(m.objects)
	m.halves = filterHalves(m.halves)
}

func (m *Module) Draw(screen *ebiten.Image, t float64, beat float64) {
	screen.Fill(bgColor)
	sc := m.ctx.Scene
	m.ctx.SampleScene(beat)
	for _, o := range m.objects {
		if o.dead {
			continue
		}
		p := o.pos(m, beat)
		if p == nil {
			continue
		}
		sc.Queue(kart.ExtraSprite{
			Sprite: m.fullSprite(o.typ),
			World:  kart.Translate((*p)[0], (*p)[1]).Mul(kart.Rotate(o.rot)).Mul(kart.Scale(m.objectScale[0], m.objectScale[1])),
			Order:  m.objectOrder,
		})
	}
	for _, h := range m.halves {
		if h.dead {
			continue
		}
		p := h.pos(m, beat)
		if p == nil {
			continue
		}
		sc.Queue(kart.ExtraSprite{
			Sprite: h.sprite,
			World:  kart.Translate((*p)[0], (*p)[1]).Mul(kart.Rotate(h.rot(t))).Mul(kart.Scale(h.scale[0], h.scale[1])),
			Order:  h.order,
		})
	}
	sc.Draw(screen, m.proj)
}

func (m *Module) scheduleThrow(idx int, ev throwEvt) {
	if !ev.muteThrow {
		types := ev.types()
		count := 1
		if ev.direction == dirBoth && ev.diffObjs {
			count = 2
		}
		for i := 0; i < count; i++ {
			m.ctx.SoundAt(ev.beat, sfxFromType(types[i])+"1", 1)
		}
	}
	m.ctx.At(ev.beat, func() {
		if ev.shouldPrepare {
			m.queuePrepare = true
		}
		m.spawnThrow(idx, ev)
	})
	m.ctx.ScheduleInput(ev.beat+1, func(state float64, _ engine.Judgment) {
		hitAny := false
		for _, o := range m.objects {
			if o.eventIdx == idx && !o.dead && o.active {
				m.hitObject(o, state)
				hitAny = true
			}
		}
		if !hitAny {
			m.missObject()
		}
	}, func() { m.missObject() })
}

func (m *Module) spawnThrow(idx int, ev throwEvt) {
	if m.spawned[idx] {
		return
	}
	m.spawned[idx] = true
	types := ev.types()
	count := 1
	if ev.direction == dirBoth {
		count = 2
	}
	for i := 0; i < count; i++ {
		altern := ev.direction == dirLeft
		if ev.direction == dirAlternate {
			altern = !m.lastSideL
		}
		fromLeft := altern
		if ev.direction == dirBoth {
			fromLeft = i == 0
		}
		cutdir := dirBoth
		if ev.direction != dirBoth {
			if altern {
				cutdir = dirLeft
			} else {
				cutdir = dirRight
			}
		}
		typ := types[1]
		if fromLeft {
			typ = types[0]
		}
		if typ == typeRandom {
			typ = randomObjectType(ev.beat, ev.uid, !fromLeft && ev.diffObjs)
		}
		shouldSfx := true
		if ev.direction == dirBoth {
			shouldSfx = ev.diffObjs || fromLeft == (types[0] == types[1])
		}
		m.objects = append(m.objects, &thrownObject{
			ev: ev, eventIdx: idx, fromLeft: fromLeft, direction: cutdir, typ: typ, shouldSfx: shouldSfx, active: true,
		})
	}
	if ev.direction == dirAlternate {
		m.lastSideL = !m.lastSideL
	} else {
		m.lastSideL = ev.direction != dirRight
	}
}

func (ev throwEvt) types() [2]int {
	if ev.diffObjs {
		return [2]int{ev.typeL, ev.typeR}
	}
	return [2]int{ev.typ, ev.typ}
}

func (o *thrownObject) update(m *Module, beat float64) {
	if o.dead {
		return
	}
	if o.active {
		flyPos := (beat - o.ev.beat + 1.1) * 0.31
		if flyPos > 1 {
			o.dead = true
		}
		return
	}
	flyPos := (beat - o.barelyBeat + 1) * 0.3
	if flyPos > 1 {
		o.dead = true
	}
}

func (o *thrownObject) pos(m *Module, beat float64) *[3]float64 {
	if o.active {
		key := "throwObject.RightCurve"
		if o.fromLeft {
			key = "throwObject.LeftCurve"
		}
		p := kart.EvalBezier(m.curves[key], (beat-o.ev.beat+1.1)*0.31)
		o.objPos = p
		return &p
	}
	key := "throwObject.BarelyLeftCurve"
	if o.fromLeft {
		key = "throwObject.BarelyRightCurve"
	}
	p := kart.EvalBezier(m.curves[key], (beat-o.barelyBeat+1)*0.3)
	p[0] += o.objPos[0]
	p[1] += o.objPos[1]
	p[2] += o.objPos[2]
	return &p
}

func (m *Module) hitObject(o *thrownObject, state float64) {
	m.stopPrepare()
	dir := directionName(o.direction)
	now := m.ctx.Beat()
	if state >= 1 || state <= -1 {
		o.active = false
		o.barely = true
		o.barelyBeat = now
		m.playDog("Barely"+dir, now, 0.5)
		if o.shouldSfx {
			m.ctx.Sound("barely")
		}
		return
	}
	m.playDog("Slice"+dir, now, 0.5)
	if o.shouldSfx {
		m.ctx.Sound(sfxFromType(o.typ) + "2")
	}
	m.spawnHalves(o, now)
	o.dead = true
}

func (m *Module) missObject() {
	if !m.preparing {
		return
	}
	m.playDog("Unprepare", m.ctx.Beat(), 0.5)
	m.stopPrepare()
}

func (m *Module) spawnHalves(o *thrownObject, beat float64) {
	leftSprite, rightSprite := m.halfSprites(o.typ)
	bpmMod := 60 / m.ctx.SecPerBeat(beat) / 100
	nowT := m.ctx.Time()
	m.halves = append(m.halves,
		&halfObject{
			startBeat: beat, startTime: nowT, objPos: o.objPos, fromLeft: o.fromLeft, leftHalf: true,
			sprite: leftSprite, scale: m.leftScale, order: m.leftOrder,
			rotSpeed: numDefault(m.ctx.Assets.Extra.Components["halves0"].Nums, "rotSpeed", -140), bpmMod: bpmMod,
		},
		&halfObject{
			startBeat: beat, startTime: nowT, objPos: o.objPos, fromLeft: o.fromLeft, leftHalf: false,
			sprite: rightSprite, scale: m.rightScale, order: m.rightOrder,
			rotSpeed: numDefault(m.ctx.Assets.Extra.Components["halves1"].Nums, "rotSpeed", 140), bpmMod: bpmMod,
		},
	)
}

func (h *halfObject) update(m *Module, _ float64, beat float64) {
	if h.dead {
		return
	}
	if h.flyPos(beat) > 1 {
		h.dead = true
	}
}

func (h *halfObject) pos(m *Module, beat float64) *[3]float64 {
	keyPrefix := "halves0"
	if !h.leftHalf {
		keyPrefix = "halves1"
	}
	field := "fallLeftCurve"
	if h.fromLeft {
		field = "fallRightCurve"
	}
	p := kart.EvalBezier(m.curves[keyPrefix+"."+field], h.flyPos(beat))
	p[0] += h.objPos[0]
	p[1] += h.objPos[1]
	p[2] += h.objPos[2]
	return &p
}

func (h *halfObject) flyPos(beat float64) float64 {
	p1 := beat - h.startBeat
	p2 := p1 / 2
	p3 := p1 / 3
	return (p3*p2+p1)*0.2 + 0.35
}

func (h *halfObject) rot(t float64) float64 {
	sign := -1.0
	if h.fromLeft {
		sign = 1
	}
	return h.rotSpeed * sign * h.bpmMod * math.Pi / 180 * math.Max(0, t-h.startTime)
}

func (m *Module) cutEverything(ev cutEvt) {
	if ev.sound {
		m.ctx.Sound("bird_flap")
	}
	_ = m.ctx.Assets.SetText(m.textPath, ev.text)
	m.ctx.Scene.PlayState(m.birdPath, "FlyIn", ev.beat, 0.5)
}

func (m *Module) doPrepare(beat float64) {
	m.playDog("Prepare", beat, 0.5)
	m.preparing = true
}

func (m *Module) stopPrepare() {
	m.preparing = false
	m.queuePrepare = false
}

func (m *Module) playDog(state string, beat, timeScale float64) {
	m.ctx.Scene.PlayState(m.dogPath, state, beat, timeScale)
}

func (m *Module) dogCanPrepare(beat float64) bool {
	state, playing := m.ctx.Scene.StateInfo(m.dogPath, beat)
	return !playing || state == "Bop" || state == "Idle"
}

func (m *Module) dogCanBop(beat float64) bool {
	state, playing := m.ctx.Scene.StateInfo(m.dogPath, beat)
	return !playing || state == "Idle"
}

func directionName(direction int) string {
	switch direction {
	case dirLeft:
		return "Left"
	case dirRight:
		return "Right"
	default:
		return "Both"
	}
}

func (m *Module) fullSprite(typ int) string {
	if typ >= 0 && typ < len(m.fullSprites) && m.fullSprites[typ] != "" {
		return m.fullSprites[typ]
	}
	return "Apple_Full"
}

func (m *Module) halfSprites(typ int) (string, string) {
	i := typ - 1
	left, right := "Apple_Left", "Apple_Right"
	if i >= 0 && i < len(m.leftSprites) && m.leftSprites[i] != "" {
		left = m.leftSprites[i]
	}
	if i >= 0 && i < len(m.rightSprites) && m.rightSprites[i] != "" {
		right = m.rightSprites[i]
	}
	return left, right
}

func sfxFromType(typ int) string {
	if typ >= typeApple && typ <= typePotato {
		return "fruit"
	}
	switch typ {
	case typeBone:
		return "bone"
	case typePan:
		return "pan"
	case typeTire:
		return "tire"
	case typeAirBatter:
		return "AirBatter"
	case typeKarateka:
		return "Karateka"
	case typeIaiGiriGaiden:
		return "IaiGiriGaiden"
	case typeThumpFarm:
		return "ThumpFarm"
	case typeBattingShow:
		return "BattingShow"
	case typeMeatGrinder:
		return "MeatGrinder"
	case typeIdol:
		return "idol"
	case typeTacoBell:
		return "tacobell"
	default:
		return "fruit"
	}
}

func randomObjectType(beat float64, entityID int, rightAndDiff bool) int {
	add := 0
	if rightAndDiff {
		add = 1
	}
	r := rand.New(rand.NewSource(int64(math.Round(beat)) + int64(entityID) + int64(add)))
	return r.Intn(typePotato-typeApple+1) + typeApple
}

func filterObjects(in []*thrownObject) []*thrownObject {
	out := in[:0]
	for _, o := range in {
		if !o.dead {
			out = append(out, o)
		}
	}
	return out
}

func filterHalves(in []*halfObject) []*halfObject {
	out := in[:0]
	for _, h := range in {
		if !h.dead {
			out = append(out, h)
		}
	}
	return out
}

func numDefault(nums map[string]float64, key string, def float64) float64 {
	if v, ok := nums[key]; ok {
		return v
	}
	return def
}

func boolParam(e *riq.Entity, key string) bool { return boolDefault(e, key, false) }

func boolDefault(e *riq.Entity, key string, def bool) bool {
	v, ok := e.Data[key]
	if !ok {
		return def
	}
	switch x := v.(type) {
	case bool:
		return x
	case float64:
		return x != 0
	case int:
		return x != 0
	default:
		return def
	}
}

func stringParam(e *riq.Entity, key, def string) string {
	if v, ok := e.Data[key].(string); ok {
		return v
	}
	return def
}
