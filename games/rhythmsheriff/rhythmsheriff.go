// Package rhythmsheriff ports Rhythm Sheriff's target timing, dog sheriff
// animation state, mapped-material palette, and tumbleweed particle controls.
package rhythmsheriff

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
	targetCat = iota
	targetRat
)

const (
	bopNormal = 1
	bopReady  = 2
	bopReturn = 3
)

type bopEvt struct {
	beat, length float64
	bop, auto    bool
}

type targetEvt struct {
	beat        float64
	typ         int
	autoHolster bool
	holsterBeat float64
}

type particleEvt struct {
	beat                         float64
	instant                      bool
	intensity, back, front, vary float64
	col                          [4]float64
}

type stormEvt struct {
	beat, length float64
	col          [4]float64
}

type targetInst struct {
	inst          *kart.Instance
	beat          float64
	typ           int
	side          string
	hit           bool
	hitBeat       float64
	canWhiff      bool
	whiffBeat     float64
	deadAfterBeat float64
}

type tumbleCfg struct {
	on             bool
	rate           float64
	size, sizeVary float64
	col            [4]float64
}

type tumble struct {
	x, y  float64
	vx    float64
	size  float64
	rot   float64
	layer int
	col   [4]float64
}

type Module struct {
	ctx  *engine.Ctx
	proj kart.Aff

	dog, targetRoot string
	targetT         *kart.Template

	bops      []bopEvt
	targets   []targetEvt
	particles []particleEvt
	storms    []stormEvt
	endBeat   float64

	bopType   int
	lastPulse float64
	rng       *rand.Rand

	pitches map[string]float64

	liveTargets []*targetInst

	backCfg, frontCfg tumbleCfg
	stormOn           bool
	stormCol          [4]float64
	tumbles           []tumble
	lastT             float64
	hasLastT          bool
	spawnAcc          [3]float64
}

func New() engine.Module {
	return &Module{
		bopType:   bopNormal,
		lastPulse: math.Inf(-1),
		rng:       rand.New(rand.NewSource(20191101)),
	}
}

func (m *Module) ID() string { return "rhythmSheriff" }

func (m *Module) Load(ctx *engine.Ctx) error {
	m.ctx = ctx
	if err := ctx.LoadAssets("rhythmSheriff"); err != nil {
		return err
	}
	m.proj = kart.Translate(engine.ScreenW/2, engine.ScreenH/2).Mul(kart.Scale(54, -54))
	m.dog = roleOr(ctx, "dogSheriff", "DogSheriff")
	m.targetRoot = roleOr(ctx, "targetObj", "TargetHolder/Target")
	m.targetT = kart.NewTemplate(ctx.Assets, m.targetRoot)
	ctx.Scene.SetActive(m.targetRoot, false)
	m.readPitches()
	m.setPalettes()
	return nil
}

func (m *Module) readPitches() {
	nums := m.ctx.Assets.Extra.Components["game"].Nums
	m.pitches = map[string]float64{
		"ratPitch":      numDefault(nums, "ratPitch", 2.15),
		"ratLowerPitch": numDefault(nums, "ratLowerPitch", 0.5),
		"ratFinalPitch": numDefault(nums, "ratFinalPitch", 1.85),
		"catPitch":      numDefault(nums, "catPitch", 1.05),
		"catLowerPitch": numDefault(nums, "catLowerPitch", 0.3),
		"catFinalPitch": numDefault(nums, "catFinalPitch", 0.95),
	}
}

func (m *Module) setPalettes() {
	for name, pal := range map[string]kart.Palette{
		"Fur":     pal(rgb(1, 1, 1), rgb(0.86666673, 0.7725491, 0.41960788), rgb(0.86666673, 0.7725491, 0.41960788)),
		"Clothes": pal(rgb(0.40000004, 0.40000004, 0.40000004), rgb(0.39607847, 0.20000002, 0), rgb(0.39607847, 0.20000002, 0)),
		"Bandana": pal(rgb(1, 1, 1), rgb(0.60784316, 0, 0.003921569), rgb(0.5803922, 0.003921569, 0.011764707)),
		"Gun":     pal(rgb(0.39607844, 0.2, 0), rgb(0.4, 0.4, 0.4), rgb(0.4, 0.4, 0.4)),
		"Sky":     pal(rgb(0.73333335, 0.70980394, 0.5803922), rgb(1, 1, 1), rgb(0.48235297, 0.62352943, 0.63529414)),
		"Ground":  pal(rgb(0.6901961, 0.50980395, 0.3529412), rgb(1, 1, 1), rgb(0.6901961, 0.50980395, 0.3529412)),
		"Rocks":   pal(rgb(0.25882354, 0.29411766, 0.32156864), rgb(0.854902, 0.41176474, 0.4039216), rgb(0.30588236, 0.3803922, 0.5019608)),
		"Bush":    pal(rgb(1, 1, 1), rgb(0.69803923, 0.63529414, 0.5058824), rgb(0.2784314, 0.3019608, 0.33333334)),
		"Tear":    pal(rgb(1, 1, 1), rgb(0, 0.60784316, 0.58475345), rgb(0, 0.60784316, 0.58431375)),
	} {
		m.ctx.Scene.SetPaletteFor(name, pal)
	}
}

func (m *Module) OnEvent(e *riq.Entity) {
	b := e.Beat
	if end := b + e.Length; end > m.endBeat {
		m.endBeat = end
	}
	switch e.Datamodel {
	case "rhythmSheriff/bop":
		m.bops = append(m.bops, bopEvt{beat: b, length: e.Length, bop: boolParamDefault(e, "toggle2", true), auto: boolParam(e, "toggle")})
	case "rhythmSheriff/slowtarget":
		m.targets = append(m.targets, targetEvt{beat: b, typ: targetCat, autoHolster: boolParamDefault(e, "auto", true), holsterBeat: e.Float("holbeat", 3)})
	case "rhythmSheriff/fasttarget":
		m.targets = append(m.targets, targetEvt{beat: b, typ: targetRat, autoHolster: boolParamDefault(e, "auto", true), holsterBeat: e.Float("holbeat", 3)})
	case "rhythmSheriff/particle effects":
		m.particles = append(m.particles, particleEvt{
			beat: b, instant: boolParam(e, "instant"),
			intensity: e.Float("intensity", 0.1),
			back:      e.Float("sizeBack", 1.2),
			front:     e.Float("sizeFront", 1.6),
			vary:      e.Float("sizeVar", 0.1),
			col:       colorParam(e, "color", [4]float64{0.87, 0.71, 0.35, 1}),
		})
	case "rhythmSheriff/tumbleweedstorm":
		m.storms = append(m.storms, stormEvt{beat: b, length: e.Length, col: colorParam(e, "color", [4]float64{0.87, 0.71, 0.35, 1})})
	}
}

func (m *Module) Ready() {
	sort.Slice(m.bops, func(i, j int) bool { return m.bops[i].beat < m.bops[j].beat })
	sort.Slice(m.targets, func(i, j int) bool { return m.targets[i].beat < m.targets[j].beat })
	for _, ev := range m.bops {
		if !ev.bop {
			continue
		}
		for b := ev.beat; b < ev.beat+ev.length-1e-6; b++ {
			bb := b
			m.ctx.At(bb, func() { m.dogBop(bb, false) })
		}
	}
	for _, ev := range m.targets {
		ev := ev
		m.scheduleTarget(ev)
	}
	for _, ev := range m.particles {
		ev := ev
		m.ctx.At(ev.beat, func() {
			m.backCfg = tumbleCfg{on: ev.intensity > 0, rate: ev.intensity, size: ev.back, sizeVary: ev.vary, col: ev.col}
			m.frontCfg = tumbleCfg{on: ev.intensity > 0, rate: ev.intensity, size: ev.front, sizeVary: ev.vary, col: ev.col}
			if ev.instant {
				m.seedTumbles(0, 8)
				m.seedTumbles(1, 8)
			}
		})
	}
	for _, ev := range m.storms {
		ev := ev
		m.ctx.At(ev.beat, func() {
			m.stormOn = true
			m.stormCol = ev.col
			m.seedTumbles(2, 18)
		})
		m.ctx.At(ev.beat+ev.length, func() { m.stormOn = false })
	}
}

func (m *Module) OnSwitch(beat float64) {
	m.ctx.Scene.PlayDefaultState(m.dog, beat, m.ctx.SecPerBeat(beat))
	m.lastPulse = math.Floor(beat)
	m.bopType = bopNormal
	m.hasLastT = false
}

func (m *Module) Whiff(beat float64) {
	m.ctx.Sound("miss")
	m.ctx.Sound("whiff")
	m.ctx.Scene.PlayState(m.dog, "ShootNG", beat, 0.5)
	for _, t := range m.liveTargets {
		if !t.canWhiff {
			continue
		}
		t.whiffBeat = beat
	}
}

func (m *Module) Update(t, beat float64) {
	if p := math.Floor(beat); p > m.lastPulse {
		m.lastPulse = p
		if m.autoBopAt(p) {
			m.dogBop(p, true)
		}
	}
	for _, tgt := range m.liveTargets {
		if tgt.deadAfterBeat > 0 && beat >= tgt.deadAfterBeat {
			tgt.canWhiff = false
		}
	}
	m.updateTumbles(t)
}

func (m *Module) Draw(screen *ebiten.Image, _, beat float64) {
	screen.Fill(color.RGBA{0xd9, 0xbd, 0x77, 0xff})
	sc := m.ctx.Scene
	m.ctx.SampleScene(beat)
	for _, tgt := range m.liveTargets {
		if tgt.deadAfterBeat > 0 && beat > tgt.deadAfterBeat+0.5 {
			continue
		}
		m.queueTarget(sc, tgt, beat)
	}
	m.queueTumbles(sc)
	sc.Draw(screen, m.proj)
}

func (m *Module) scheduleTarget(ev targetEvt) {
	m.scheduleTargetSounds(ev)
	m.ctx.At(ev.beat, func() { m.spawnTarget(ev) })
	m.ctx.At(ev.beat+0.25, func() { m.bopType = bopReady })
	m.ctx.At(ev.beat+1, func() {
		if !m.autoBopAt(ev.beat) {
			m.ctx.Scene.PlayState(m.dog, "Ready", ev.beat+1, 0.5)
		}
	})
	length := 2.0
	if ev.typ == targetRat {
		length = 1.5
	}
	m.ctx.ScheduleInput(ev.beat+length,
		func(_ float64, _ engine.Judgment) { m.targetJust(ev.beat) },
		func() { m.targetMiss(ev.beat) })
}

func (m *Module) scheduleTargetSounds(ev targetEvt) {
	p := m.pitches
	if ev.typ == targetCat {
		m.ctx.SoundAtPitchPan(ev.beat, "targetCat", 1, p["catPitch"], 0)
		for i, vol := range []float64{0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9} {
			m.ctx.SoundAtPitchPan(ev.beat+1+0.125*float64(i), "targetCat", vol, p["catPitch"]-p["catLowerPitch"], 0)
		}
		m.ctx.SoundAtPitchPan(ev.beat+2, "targetCat", 1, p["catFinalPitch"], 0)
		return
	}
	m.ctx.SoundAtPitchPan(ev.beat, "targetCat", 1, p["ratPitch"], 0)
	for _, s := range []struct {
		off, vol float64
	}{
		{0.5, 0.9}, {0.625, 0.7}, {0.75, 0.5}, {0.875, 0.3},
		{1.0, 0.9}, {1.125, 0.7}, {1.25, 0.5}, {1.375, 0.3},
	} {
		m.ctx.SoundAtPitchPan(ev.beat+s.off, "targetCat", s.vol, p["ratPitch"]-p["ratLowerPitch"], 0)
	}
	m.ctx.SoundAtPitchPan(ev.beat+1.5, "targetCat", 1, p["ratFinalPitch"], 0)
}

func (m *Module) spawnTarget(ev targetEvt) {
	if m.targetT == nil {
		return
	}
	in := m.targetT.NewInstance()
	side := "Right"
	if m.rng.Intn(2) == 0 {
		side = "Left"
	}
	t := &targetInst{
		inst:          in,
		beat:          ev.beat,
		typ:           ev.typ,
		side:          side,
		canWhiff:      true,
		deadAfterBeat: ev.beat + 3.25,
	}
	m.liveTargets = append(m.liveTargets, t)
}

func (m *Module) targetJust(startBeat float64) {
	m.ctx.Sound("destroyTarget1")
	m.ctx.Sound("destroyTarget3")
	m.ctx.Scene.PlayState(m.dog, "ShootJust", m.ctx.Beat(), 0.5)
	m.bopType = bopReturn
	for _, t := range m.liveTargets {
		if math.Abs(t.beat-startBeat) < 1e-6 {
			t.hit = true
			t.hitBeat = m.ctx.Beat()
			t.canWhiff = false
			return
		}
	}
}

func (m *Module) targetMiss(startBeat float64) {
	m.ctx.SoundAt(startBeat+3, "sink", 1)
	for _, t := range m.liveTargets {
		if math.Abs(t.beat-startBeat) < 1e-6 {
			t.canWhiff = false
			break
		}
	}
	m.ctx.At(startBeat+2.25, func() { m.bopType = bopNormal })
}

func (m *Module) dogBop(beat float64, fromAuto bool) {
	if fromAuto {
		if st, playing := m.ctx.Scene.StateInfo(m.dog, beat); playing && (st == "ShootNG" || st == "ShootJust" || st == "Ready") {
			return
		}
	}
	switch m.bopType {
	case bopReady:
		m.ctx.Scene.PlayState(m.dog, "Ready", beat, 0.5)
	case bopReturn:
		m.ctx.Scene.PlayState(m.dog, "Return", beat, 0.5)
		m.bopType = bopNormal
		m.ctx.Sound("holster")
	default:
		m.ctx.Scene.PlayState(m.dog, "Bop", beat, 0.5)
	}
}

func (m *Module) queueTarget(sc *kart.SceneInst, tgt *targetInst, beat float64) {
	main := "Target/CatTarget"
	typ := "Target/Cat"
	side := "Target/CatTarget" + tgt.side
	if tgt.typ == targetRat {
		main, typ, side = "Target/RatTarget", "Target/Rat", "Target/RatTarget"+tgt.side
	}
	mainAnim := m.ctx.Assets.Anims[main]
	sideAnim := m.ctx.Assets.Anims[side]
	typeAnim := m.ctx.Assets.Anims[typ]
	at := math.Max(0, (beat-tgt.beat)*0.5)
	board := kart.SampleClipNode(mainAnim, "board", at)
	sidePose := kart.SampleClipNode(sideAnim, "board", at)
	boardX, boardY := 0.0, -4.85
	if board.HasPos[0] || sidePose.HasPos[0] || board.HasPos[1] || sidePose.HasPos[1] {
		if board.HasPos[0] {
			boardX = board.Pos[0]
		}
		if board.HasPos[1] {
			boardY = board.Pos[1]
		}
		if sidePose.HasPos[0] {
			boardX += sidePose.Pos[0]
		}
		if sidePose.HasPos[1] {
			boardY += sidePose.Pos[1]
		}
		tgt.inst.SetPos("board", boardX, boardY)
	}
	boardSX, boardSY := 0.33, 0.33
	if board.HasScale[0] || board.HasScale[1] {
		if board.HasScale[0] {
			boardSX = board.Scale[0]
		}
		if board.HasScale[1] {
			boardSY = board.Scale[1]
		}
		tgt.inst.SetScale("board", boardSX, boardSY)
	}
	rot := 0.0
	if board.HasRot {
		rot = board.RotDeg * math.Pi / 180
	}
	if tgt.whiffBeat > 0 {
		if pose := kart.SampleClipNode(m.ctx.Assets.Anims["Target/WhiffSpin"], "board", (beat-tgt.whiffBeat)*0.5); pose.HasRot {
			rot = pose.RotDeg * math.Pi / 180
		}
	}
	tgt.inst.SetRot("board", rot)
	m.applyTargetSprites(tgt, typeAnim, at)
	for _, path := range []string{"board/back", "board/front"} {
		if v, ok := kart.SampleClipFloat(mainAnim, path, "m_Enabled", at); ok {
			tgt.inst.SetActive(path, v > 0.5)
		}
	}
	tgt.inst.Queue(sc, beat, kart.Identity(), 0)
	if tgt.hit && beat-tgt.hitBeat < 0.6 {
		world := kart.Translate(tgt.inst.Offset[0], tgt.inst.Offset[1]).
			Mul(kart.TRS(boardX, boardY, rot, boardSX, boardSY)).
			Mul(kart.TRS(0, 0, 0, 0.9, 0.9))
		sc.Queue(kart.ExtraSprite{Sprite: "hole", World: world, Order: 40, Tint: [4]float64{1, 1, 1, 1}})
	}
}

func (m *Module) applyTargetSprites(tgt *targetInst, anim *kmdata.Anim, at float64) {
	for _, path := range []string{"board/back", "board/front"} {
		pose := kart.SampleClipNode(anim, path, at)
		if pose.HasSprite {
			tgt.inst.SetSprite(path, pose.Sprite)
		}
	}
}

func (m *Module) autoBopAt(beat float64) bool {
	if len(m.bops) == 0 {
		return true
	}
	on := false
	for _, ev := range m.bops {
		if ev.beat > beat {
			break
		}
		on = ev.auto
	}
	return on
}

func (m *Module) updateTumbles(t float64) {
	if !m.hasLastT {
		m.lastT, m.hasLastT = t, true
		return
	}
	dt := t - m.lastT
	m.lastT = t
	if dt <= 0 || dt > 0.25 {
		return
	}
	m.emitTumbles(dt)
	dst := m.tumbles[:0]
	for _, p := range m.tumbles {
		p.x += p.vx * dt
		p.rot += p.vx * dt * 1.8
		if p.x > -13 && p.x < 13 {
			dst = append(dst, p)
		}
	}
	m.tumbles = dst
}

func (m *Module) emitTumbles(dt float64) {
	configs := []tumbleCfg{m.backCfg, m.frontCfg, {on: m.stormOn, rate: 25, size: 2.2, sizeVary: 0.6, col: m.stormCol}}
	for layer, cfg := range configs {
		if !cfg.on || cfg.rate <= 0 {
			continue
		}
		m.spawnAcc[layer] += cfg.rate * dt
		for m.spawnAcc[layer] >= 1 {
			m.spawnAcc[layer]--
			m.spawnTumble(layer, cfg)
		}
	}
}

func (m *Module) seedTumbles(layer, n int) {
	cfg := m.backCfg
	if layer == 1 {
		cfg = m.frontCfg
	} else if layer == 2 {
		cfg = tumbleCfg{on: true, rate: 25, size: 2.2, sizeVary: 0.6, col: m.stormCol}
	}
	for i := 0; i < n; i++ {
		m.spawnTumble(layer, cfg)
		m.tumbles[len(m.tumbles)-1].x = -11 + m.rng.Float64()*22
	}
}

func (m *Module) spawnTumble(layer int, cfg tumbleCfg) {
	size := cfg.size + (m.rng.Float64()*2-1)*cfg.sizeVary
	if size < 0.1 {
		size = 0.1
	}
	y := -3.25
	order := -35
	if layer == 1 {
		y, order = -3.85, 35
	} else if layer == 2 {
		y, order = -2.4+m.rng.Float64()*2.2, 120
	}
	m.tumbles = append(m.tumbles, tumble{
		x: -12.5, y: y, vx: 2.8 + m.rng.Float64()*1.8,
		size: size * 0.25, rot: m.rng.Float64() * math.Pi * 2,
		layer: order, col: cfg.col,
	})
}

func (m *Module) queueTumbles(sc *kart.SceneInst) {
	for _, p := range m.tumbles {
		world := kart.TRS(p.x, p.y, p.rot, p.size, p.size)
		sc.Queue(kart.ExtraSprite{Sprite: "tumbleweed", World: world, Order: p.layer, Tint: p.col})
	}
}

func roleOr(ctx *engine.Ctx, key, fallback string) string {
	if p := ctx.Role(key); p != "" {
		return p
	}
	return fallback
}

func boolParam(e *riq.Entity, key string) bool { return e.Float(key, 0) != 0 }

func boolParamDefault(e *riq.Entity, key string, def bool) bool {
	if _, ok := e.Data[key]; !ok {
		return def
	}
	return boolParam(e, key)
}

func colorParam(e *riq.Entity, key string, def [4]float64) [4]float64 {
	v, ok := e.Data[key]
	if !ok {
		return def
	}
	m, ok := v.(map[string]any)
	if !ok {
		return def
	}
	return [4]float64{
		numAny(m["r"], def[0]),
		numAny(m["g"], def[1]),
		numAny(m["b"], def[2]),
		numAny(m["a"], def[3]),
	}
}

func numDefault(m map[string]float64, key string, def float64) float64 {
	if v, ok := m[key]; ok {
		return v
	}
	return def
}

func numAny(v any, def float64) float64 {
	if f, ok := v.(float64); ok {
		return f
	}
	return def
}

func rgb(r, g, b float64) [4]float64 { return [4]float64{r, g, b, 1} }

func pal(a, b, d [4]float64) kart.Palette {
	return kart.Palette{Alpha: a, Fill: b, Outline: d}
}
