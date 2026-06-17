// Package builttoscaleds ports Built to Scale (DS/NTR) timing onto engine.App.
//
// The extracted DS bundle is mesh-only: it has Animator clips, sounds, and
// MeshRenderer bindings but no SpriteRenderer atlas. The static world now uses
// the extracted MeshRenderer scene, while dynamic block objects still use the
// existing 2D timing layer until the mesh prefab instancing path is complete.
package builttoscaleds

import (
	"image/color"
	"math"
	"sort"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"hsdemo/engine"
	"hsdemo/kart"
	"hsdemo/riq"
)

const (
	blockFramesPerSecond = 24.0
	blockHitFrame        = 39.0
	blockTotalFrames     = 80.0
	spawnFrameOffset     = -3.0
	pianoFadeSec         = 0.1

	actionFlick = 0
)

var (
	defaultObjectColor  = [4]float64{1, 1, 1, 1}
	defaultShooterColor = [4]float64{1, 1, 1, 1}
	defaultEnvColor     = [4]float64{0, 1, 0, 1}
)

type blockEvt struct {
	beat, length float64
	silent       bool
	staccato     bool
	notes        [6]int
}

type colorEvt struct {
	beat                 float64
	object, shooter, env [4]float64
}

type lightEvt struct {
	beat, length float64
	auto         bool
	light        bool
}

type cameraEvt struct {
	beat, length float64
	rot, zoom    float64
	ease         int
	additive     bool
	startRot     float64
	startZoom    float64
	endRot       float64
	endZoom      float64
}

type blockState int

const (
	blockMoving blockState = iota
	blockHit
	blockNear
	blockMiss
	blockSunk
)

type block struct {
	evt                             blockEvt
	createBeat, windupBeat, hitBeat float64
	sinkBeat                        float64
	state                           blockState
	stateBeat                       float64
	inst                            *kart.Instance
}

type fxKind int

const (
	fxHit fxKind = iota
	fxNear
	fxRod
)

type effect struct {
	beat, endBeat float64
	kind          fxKind
	x, y          float64
	inst          *kart.Instance
}

type Module struct {
	ctx  *engine.Ctx
	proj kart.Aff

	events []blockEvt
	colors []colorEvt
	lights []lightEvt
	cams   []cameraEvt

	blocks  []*block
	effects []effect

	blocksHolder, partsHolder            string
	movingBlocksT, flyingRodT, hitPartsT *kart.Template
	missPartsT                           *kart.Template

	objectColor  [4]float64
	shooterColor [4]float64
	envColor     [4]float64

	shooterState string
	shooterBeat  float64
	lastShotOut  bool
}

func New() engine.Module {
	return &Module{
		objectColor: defaultObjectColor, shooterColor: defaultShooterColor,
		envColor: defaultEnvColor, shooterState: "Idle",
	}
}

func (m *Module) ID() string { return "builtToScaleDS" }

func (m *Module) Load(ctx *engine.Ctx) error {
	m.ctx = ctx
	if err := ctx.LoadAssets("builtToScaleDS"); err != nil {
		return err
	}
	m.proj = kart.Translate(engine.ScreenW/2, engine.ScreenH/2).Mul(kart.Scale(54, -54))
	m.blocksHolder = ctx.Role("blocksHolder")
	m.partsHolder = ctx.Role("partsHolder")
	m.movingBlocksT = kart.NewTemplate(ctx.Assets, ctx.Role("movingBlocksBase"))
	m.flyingRodT = kart.NewTemplate(ctx.Assets, ctx.Role("flyingRodBase"))
	m.hitPartsT = kart.NewTemplate(ctx.Assets, ctx.Role("hitPartsBase"))
	m.missPartsT = kart.NewTemplate(ctx.Assets, ctx.Role("missPartsBase"))
	m.playSceneDefaults(0)
	m.hideDynamicMeshPrefabs()
	return nil
}

func (m *Module) OnEvent(e *riq.Entity) {
	switch e.Datamodel {
	case "builtToScaleDS/spawn blocks":
		m.addBlockEvent(e)
	case "builtToScaleDS/play piano":
		length := e.Length
		if length <= 0 {
			length = 0.5
		}
		m.schedulePiano(e.Beat, length, int(e.Float("type", 0)))
	case "builtToScaleDS/color":
		ev := colorEvt{
			beat:    e.Beat,
			object:  colorParam(e, "object", defaultObjectColor),
			shooter: colorParam(e, "shooter", defaultShooterColor),
			env:     colorParam(e, "bg", defaultEnvColor),
		}
		m.colors = append(m.colors, ev)
		m.ctx.At(e.Beat, func() {
			m.objectColor, m.shooterColor, m.envColor = ev.object, ev.shooter, ev.env
		})
	case "builtToScaleDS/lights":
		ev := lightEvt{beat: e.Beat, length: e.Length, auto: boolParamDefault(e, "auto", true), light: boolParam(e, "light")}
		m.lights = append(m.lights, ev)
	case "builtToScaleDS/camera":
		m.cams = append(m.cams, cameraEvt{
			beat: e.Beat, length: e.Length, rot: e.Float("valA", 0), zoom: e.Float("valB", 1),
			ease: int(e.Float("type", 0)), additive: boolParamDefault(e, "additive", true),
		})
	}
}

func (m *Module) Ready() {
	sort.Slice(m.events, func(i, j int) bool { return m.events[i].beat < m.events[j].beat })
	sort.Slice(m.colors, func(i, j int) bool { return m.colors[i].beat < m.colors[j].beat })
	sort.Slice(m.lights, func(i, j int) bool { return m.lights[i].beat < m.lights[j].beat })
	sort.Slice(m.cams, func(i, j int) bool { return m.cams[i].beat < m.cams[j].beat })

	rot, zoom := 0.0, 1.0
	for i := range m.cams {
		ev := &m.cams[i]
		ev.startRot, ev.startZoom = rot, zoom
		if ev.additive {
			ev.endRot = rot + ev.rot
		} else {
			ev.endRot = ev.rot
		}
		ev.endZoom = ev.zoom
		rot, zoom = ev.endRot, ev.endZoom
	}
}

func (m *Module) OnSwitch(beat float64) {
	m.blocks = nil
	m.effects = nil
	m.setShooterState("Idle", beat)
	m.lastShotOut = false
	m.objectColor, m.shooterColor, m.envColor = defaultObjectColor, defaultShooterColor, defaultEnvColor
	for _, ev := range m.colors {
		if ev.beat <= beat {
			m.objectColor, m.shooterColor, m.envColor = ev.object, ev.shooter, ev.env
		}
	}
	for _, ev := range m.events {
		create := spawnBeat(ev)
		if beat >= create && beat < hitBeat(ev)+2*ev.length {
			m.spawnBlock(ev, create)
		}
	}
	m.playSceneDefaults(beat)
}

func (m *Module) Whiff(beat float64) {
	m.shoot(beat)
	m.ctx.Sound("Boing")
	m.spawnPieceEffect(beat, fxRod, m.flyingRodT, "Fly", "FlyingRod", shooterX(), laneY())
	m.lastShotOut = true
}

func (m *Module) Update(_, beat float64) {
	if m.shooterState == "Shoot" && beat-m.shooterBeat > 0.45 {
		m.setShooterState("Idle", beat)
		m.lastShotOut = false
	}
	if m.shooterState == "Windup" {
		hasWindup := false
		for _, b := range m.blocks {
			if b.state == blockMoving && beat >= b.windupBeat && beat < b.hitBeat {
				hasWindup = true
				break
			}
		}
		if !hasWindup {
			m.setShooterState("Idle", beat)
		}
	}
	for _, b := range m.blocks {
		if b.state == blockMoving && beat >= b.windupBeat && beat < b.hitBeat {
			m.setShooterState("Windup", beat)
		}
		if b.state == blockMiss && beat >= b.sinkBeat {
			b.state = blockSunk
			b.stateBeat = beat
		}
	}
	if len(m.effects) > 0 {
		alive := m.effects[:0]
		for _, fx := range m.effects {
			endBeat := fx.endBeat
			if endBeat <= 0 {
				endBeat = fx.beat + 2
			}
			if beat < endBeat {
				alive = append(alive, fx)
			}
		}
		m.effects = alive
	}
}

func (m *Module) Draw(screen *ebiten.Image, _, beat float64) {
	rot, zoom := m.cameraAt(beat)
	env := m.envAt(beat)
	screen.Fill(toRGBA(env))
	m.drawOfficialMeshScene(screen, env, rot, zoom, beat)
	m.drawConveyor(screen, env, zoom, beat)
	m.drawElevator(screen, zoom)
	for _, b := range m.blocks {
		m.drawBlock(screen, b, beat, zoom)
	}
	m.drawShooter(screen, zoom)
	m.drawEffects(screen, beat, zoom)
	m.drawLights(screen, env, beat)
}

func (m *Module) addBlockEvent(e *riq.Entity) {
	length := e.Length
	if length <= 0 {
		length = 1
	}
	ev := blockEvt{
		beat: e.Beat, length: length, silent: boolParam(e, "silent"), staccato: boolParam(e, "staccato"),
		notes: [6]int{
			int(e.Float("note1", 0)), int(e.Float("note2", 2)), int(e.Float("note3", 4)),
			int(e.Float("note4", 5)), int(e.Float("note5", 7)), int(e.Float("note6", 12)),
		},
	}
	m.events = append(m.events, ev)
	create := spawnBeat(ev)
	m.ctx.At(create, func() {
		if m.ctx.GameAt(ev.beat) == m.ID() || m.ctx.GameAt(hitBeat(ev)) == m.ID() {
			m.spawnBlock(ev, create)
		}
	})
	if !ev.silent {
		noteLen, endLen := noteLengths(ev.length, ev.staccato)
		for i := 0; i < 4; i++ {
			m.schedulePiano(ev.beat+ev.length*float64(i), noteLen, ev.notes[i])
		}
		m.schedulePiano(ev.beat+ev.length*4, endLen, ev.notes[4])
	}
	m.ctx.ScheduleInputCond(
		hitBeat(ev),
		func() bool { return m.findBlock(ev) != nil && m.ctx.GameAt(hitBeat(ev)) == m.ID() },
		func(state float64, j engine.Judgment) { m.hitBlock(ev, state, j) },
		func() { m.missBlock(ev) },
	)
}

func (m *Module) spawnBlock(ev blockEvt, create float64) {
	if m.findBlock(ev) != nil {
		return
	}
	b := &block{
		evt: ev, createBeat: create, windupBeat: windupBeat(ev),
		hitBeat: hitBeat(ev), sinkBeat: hitBeat(ev) + 2*ev.length,
	}
	if m.movingBlocksT != nil {
		b.inst = m.movingBlocksT.NewInstance()
	}
	m.blocks = append(m.blocks, b)
	m.ctx.Scene.PlayState(m.ctx.Role("elevatorAnim"), "MakeRod", create, 1)
	m.ctx.At(b.sinkBeat, func() {
		if b.state == blockMiss {
			m.ctx.Sound("Sink")
		}
	})
}

func (m *Module) hitBlock(ev blockEvt, state float64, j engine.Judgment) {
	b := m.findBlock(ev)
	if b == nil {
		return
	}
	beat := m.ctx.Beat()
	if j == engine.JudgeNG || math.Abs(state) >= 1 {
		b.state = blockNear
		b.stateBeat = beat
		m.shoot(beat)
		m.ctx.Sound("Crumble")
		m.spawnPieceEffect(beat, fxNear, m.missPartsT, "PartsMiss", "MissParts", shooterX()-30, laneY())
		return
	}
	b.state = blockHit
	b.stateBeat = beat
	m.shoot(beat)
	m.ctx.Sound("Hit")
	if !b.evt.silent {
		noteLen := 0.75
		if b.evt.staccato {
			noteLen = 0.5
		}
		m.playPianoNow(noteLen, b.evt.notes[5])
	}
	m.spawnPieceEffect(beat, fxHit, m.hitPartsT, "PartsHit", "HitParts", shooterX()-28, laneY())
}

func (m *Module) missBlock(ev blockEvt) {
	b := m.findBlock(ev)
	if b == nil {
		return
	}
	b.state = blockMiss
	b.stateBeat = m.ctx.Beat()
}

func (m *Module) shoot(beat float64) {
	m.setShooterState("Shoot", beat)
}

func (m *Module) setShooterState(state string, beat float64) {
	if m.shooterState == state && state != "Shoot" {
		return
	}
	m.shooterState = state
	m.shooterBeat = beat
	if root := m.ctx.Role("shooterAnim"); root != "" {
		m.ctx.Scene.PlayState(root, state, beat, 1)
	}
}

func (m *Module) findBlock(ev blockEvt) *block {
	for _, b := range m.blocks {
		if b.evt.beat == ev.beat && b.evt.length == ev.length {
			return b
		}
	}
	return nil
}

func (m *Module) schedulePiano(beat, length float64, semitone int) {
	stopBeat := pianoEndBeat(beat, length)
	m.ctx.At(beat, func() {
		if m.ctx.GameAt(beat) == m.ID() {
			m.ctx.SoundLoopPitchVolUntil("Piano", semitonePitch(semitone), 0.8, stopBeat, pianoFadeSec)
		}
	})
}

func (m *Module) playPianoNow(length float64, semitone int) {
	m.ctx.SoundLoopPitchVolUntil("Piano", semitonePitch(semitone), 0.8, pianoEndBeat(m.ctx.Beat(), length), pianoFadeSec)
}

func (m *Module) spawnPieceEffect(beat float64, kind fxKind, tmpl *kart.Template, state, ctrl string, x, y float64) {
	fx := effect{beat: beat, kind: kind, x: x, y: y, endBeat: beat + 2}
	if tmpl != nil {
		fx.inst = tmpl.NewInstance()
		fx.inst.PlayState("", state, beat, m.ctx.SecPerBeat(beat))
		if dur := m.stateDurationBeats(ctrl, state, beat); dur > 0 {
			fx.endBeat = beat + dur
		}
	}
	m.effects = append(m.effects, fx)
}

func (m *Module) stateDurationBeats(ctrlName, stateName string, beat float64) float64 {
	ctrl, ok := m.ctx.Assets.Controllers[ctrlName]
	if !ok {
		return 0
	}
	st, ok := ctrl.States[stateName]
	if !ok || st.Clip == "" {
		return 0
	}
	anim := m.ctx.Assets.Anims[st.Clip]
	if anim == nil {
		return 0
	}
	speed := st.Speed
	if speed == 0 {
		return 0
	}
	secPerBeat := m.ctx.SecPerBeat(beat)
	if secPerBeat <= 0 {
		return 0
	}
	return anim.Duration / (secPerBeat * speed)
}

func pianoEndBeat(beat, length float64) float64 {
	return beat + math.Max(0, length)
}

func (m *Module) cameraAt(beat float64) (rot, zoom float64) {
	rot, zoom = 0, 1
	for _, ev := range m.cams {
		if beat < ev.beat {
			break
		}
		u := 1.0
		if ev.length > 0 && beat < ev.beat+ev.length {
			u = (beat - ev.beat) / ev.length
		}
		rot = engine.Ease(ev.ease, ev.startRot, ev.endRot, u)
		zoom = engine.Ease(ev.ease, ev.startZoom, ev.endZoom, u)
	}
	if zoom <= 0 {
		zoom = 1
	}
	return rot, zoom
}

func (m *Module) envAt(beat float64) [4]float64 {
	env := m.envColor
	for _, ev := range m.colors {
		if ev.beat <= beat {
			env = ev.env
		}
	}
	return env
}

func (m *Module) lightsActive(beat float64) (bool, bool) {
	active, first := false, true
	for _, ev := range m.lights {
		if beat < ev.beat {
			break
		}
		if ev.auto {
			active = true
			first = int(math.Floor(beat-ev.beat))%2 == 0
			continue
		}
		active = ev.light && beat < ev.beat+ev.length
		first = int(math.Floor(beat-ev.beat))%2 == 0
	}
	return active, first
}

func (m *Module) playSceneDefaults(beat float64) {
	sec := m.ctx.SecPerBeat(beat)
	for _, role := range []string{"shooterAnim", "elevatorAnim"} {
		if root := m.ctx.Role(role); root != "" {
			m.ctx.Scene.PlayDefaultState(root, beat, sec)
		}
	}
}

func (m *Module) hideDynamicMeshPrefabs() {
	for _, role := range []string{"movingBlocksBase", "flyingRodBase", "hitPartsBase", "missPartsBase"} {
		if root := m.ctx.Role(role); root != "" {
			m.ctx.Scene.SetActive(root, false)
		}
	}
}

func (m *Module) drawOfficialMeshScene(screen *ebiten.Image, env [4]float64, rot, zoom, beat float64) {
	for _, b := range m.ctx.Assets.Meshes.Bindings {
		if len(b.Materials) == 0 || b.Materials[0].Name != "GridPlane" {
			continue
		}
		m.ctx.Scene.SetMaterialOver(b.Path, env, [4]float64{})
	}
	m.ctx.SampleScene(beat)
	m.queueOfficialDynamicMeshes(beat)
	proj := m.proj.Mul(kart.Scale(zoom, zoom)).Mul(kart.Rotate(rot * math.Pi / 180))
	m.ctx.Scene.Draw(screen, proj)
}

func (m *Module) queueOfficialDynamicMeshes(beat float64) {
	sc := m.ctx.Scene
	blocksWorld := kart.Identity()
	if m.blocksHolder != "" {
		if w, ok := sc.NodeWorld(m.blocksHolder); ok {
			blocksWorld = w
		}
	}
	for _, b := range m.blocks {
		if b.inst == nil || b.state != blockMoving || beat < b.createBeat || beat >= b.sinkBeat {
			continue
		}
		frame := blockAnimFrame(b.evt, beat, m.ctx.SecPerBeat(beat))
		b.inst.PlayNormalized("", "Models/MovingBlocks/piece_LR", clamp01(frame/blockTotalFrames))
		b.inst.Queue(sc, beat, blocksWorld, 0)
	}

	partsWorld := kart.Identity()
	if m.partsHolder != "" {
		if w, ok := sc.NodeWorld(m.partsHolder); ok {
			partsWorld = w
		}
	}
	for _, fx := range m.effects {
		if fx.inst == nil || (fx.endBeat > 0 && beat >= fx.endBeat) {
			continue
		}
		fx.inst.Queue(sc, beat, partsWorld, 0)
	}
}

func (m *Module) drawConveyor(screen *ebiten.Image, env [4]float64, zoom, beat float64) {
	y := float32(laneY() * zoom)
	h := float32(82 * zoom)
	belt := toRGBA(blend(env, [4]float64{0.05, 0.12, 0.05, 1}, 0.48))
	vector.DrawFilledRect(screen, 0, y-h/2, engine.ScreenW, h, belt, false)
	vector.StrokeLine(screen, 0, y-h/2, engine.ScreenW, y-h/2, 5, color.RGBA{255, 255, 255, 120}, false)
	vector.StrokeLine(screen, 0, y+h/2, engine.ScreenW, y+h/2, 5, color.RGBA{0, 0, 0, 95}, false)
	for x := -80 + math.Mod(beat*96, 80); x < engine.ScreenW+80; x += 80 {
		vector.StrokeLine(screen, float32(x), y-h/2, float32(x+36), y+h/2, 3, color.RGBA{255, 255, 255, 80}, false)
	}
}

func (m *Module) drawElevator(screen *ebiten.Image, zoom float64) {
	x, y := float32(154*zoom), float32(laneY()*zoom)
	vector.DrawFilledRect(screen, x-42, y-116, 84, 116, color.RGBA{45, 90, 45, 255}, false)
	vector.StrokeRect(screen, x-42, y-116, 84, 116, 4, color.RGBA{235, 255, 235, 180}, false)
	vector.DrawFilledRect(screen, x-28, y-76, 56, 56, toRGBA(m.objectColor), false)
}

func (m *Module) drawShooter(screen *ebiten.Image, zoom float64) {
	x, y := float32(shooterX()*zoom), float32(laneY()*zoom)
	col := toRGBA(m.shooterColor)
	if m.shooterState == "Windup" {
		x += 20 * float32(math.Sin(m.ctx.Beat()*math.Pi*4))
	}
	if m.shooterState == "Shoot" {
		x -= 34
	}
	vector.DrawFilledRect(screen, x-42, y-50, 74, 100, col, false)
	vector.DrawFilledCircle(screen, x-52, y, 28, col, false)
	vector.StrokeLine(screen, x-88, y, x-24, y, 12, color.RGBA{250, 250, 250, 210}, false)
}

func (m *Module) drawBlock(screen *ebiten.Image, b *block, beat, zoom float64) {
	if beat < b.createBeat || b.state == blockSunk {
		return
	}
	x, y := blockPos(b, beat)
	x *= zoom
	y *= zoom
	switch b.state {
	case blockHit:
		if beat-b.stateBeat > 0.45 {
			return
		}
		x += (beat - b.stateBeat) * 250
	case blockNear:
		if beat-b.stateBeat > 1.2 {
			return
		}
		y += (beat - b.stateBeat) * 170
	case blockMiss:
		if beat >= b.sinkBeat {
			return
		}
		if beat > b.hitBeat {
			y += (beat - b.hitBeat) / (b.sinkBeat - b.hitBeat) * 140
		}
	}
	drawWidget(screen, float32(x), float32(y), float32(zoom), toRGBA(m.objectColor))
}

func (m *Module) drawEffects(screen *ebiten.Image, beat, zoom float64) {
	for _, fx := range m.effects {
		u := beat - fx.beat
		x, y := float32(fx.x*zoom), float32(fx.y*zoom)
		switch fx.kind {
		case fxHit:
			for i := 0; i < 8; i++ {
				a := float64(i) * math.Pi / 4
				r := float32(18 + u*76)
				vector.StrokeLine(screen, x, y, x+float32(math.Cos(a))*r, y+float32(math.Sin(a))*r, 4, color.RGBA{255, 255, 255, uint8(220 * (1 - clamp01(u/0.8)))}, false)
			}
		case fxNear:
			alpha := uint8(210 * (1 - clamp01(u/1.2)))
			vector.StrokeCircle(screen, x, y+float32(u*80), float32(35+u*80), 7, color.RGBA{255, 225, 210, alpha}, false)
		case fxRod:
			alpha := uint8(210 * (1 - clamp01(u/1.0)))
			vector.StrokeLine(screen, x-float32(u*300), y-34, x+80-float32(u*300), y-34, 10, color.RGBA{255, 255, 255, alpha}, false)
		}
	}
}

func (m *Module) drawLights(screen *ebiten.Image, env [4]float64, beat float64) {
	active, first := m.lightsActive(beat)
	if !active {
		return
	}
	c := toRGBA(blend(env, [4]float64{1, 1, 1, 1}, 0.75))
	c.A = 120
	offset := 0
	if !first {
		offset = 1
	}
	for i := 0; i < 5; i++ {
		if i%2 != offset {
			continue
		}
		x := float32(140 + i*170)
		vector.DrawFilledCircle(screen, x, 86, 34, c, false)
	}
}

func drawWidget(screen *ebiten.Image, x, y, scale float32, col color.RGBA) {
	w, h := float32(24)*scale, float32(24)*scale
	for i := 0; i < 6; i++ {
		px := x + float32(i-3)*w*0.86
		py := y
		if i%2 == 1 {
			py -= h * 0.68
		}
		shade := col
		if i%2 == 1 {
			shade.R = uint8(float64(shade.R) * 0.82)
			shade.G = uint8(float64(shade.G) * 0.82)
			shade.B = uint8(float64(shade.B) * 0.82)
		}
		vector.DrawFilledRect(screen, px-w/2, py-h/2, w, h, shade, false)
		vector.StrokeRect(screen, px-w/2, py-h/2, w, h, 2, color.RGBA{0, 0, 0, 120}, false)
	}
}

func spawnBeat(ev blockEvt) float64  { return ev.beat - ev.length }
func windupBeat(ev blockEvt) float64 { return spawnBeat(ev) + ev.length*4 }
func hitBeat(ev blockEvt) float64    { return spawnBeat(ev) + ev.length*5 }

func noteLengths(length float64, staccato bool) (noteLen, endLen float64) {
	noteLen, endLen = length, 0.75
	if staccato && length > 0.5 {
		noteLen, endLen = 0.5, 0.5
	}
	return
}

func blockAnimFrame(ev blockEvt, beat float64, secPerBeat float64) float64 {
	spawnTimeOffset := spawnFrameOffset / blockFramesPerSecond
	secondsPerFrame := 1 / blockFramesPerSecond
	secondsToHitFrame := secondsPerFrame * blockHitFrame
	secondsToHitBeat := secPerBeat*5*ev.length + spawnTimeOffset
	if secondsToHitBeat == 0 {
		return 0
	}
	speedMult := secondsToHitFrame / secondsToHitBeat
	secondsPastSpawn := secPerBeat*(beat-spawnBeat(ev)) + spawnTimeOffset
	frame := blockFramesPerSecond * speedMult * secondsPastSpawn
	// Unity snaps these exact integer boundaries upward to avoid FBX
	// interpolation landing between block poses at the musical hit frames.
	switch int(math.Floor(frame)) {
	case 7, 15, 23, 31, 39, 47:
		return math.Ceil(frame)
	default:
		return frame
	}
}

func blockPos(b *block, beat float64) (x, y float64) {
	u := clamp01((beat - b.createBeat) / (b.hitBeat - b.createBeat))
	x = 154 + (shooterX()-196-154)*u
	y = laneY() - math.Sin(u*math.Pi)*48
	return
}

func laneY() float64    { return 348 }
func shooterX() float64 { return 750 }

func semitonePitch(semitone int) float64 { return math.Exp2(float64(semitone) / 12) }

func colorParam(e *riq.Entity, key string, def [4]float64) [4]float64 {
	v, ok := e.Data[key]
	if !ok {
		return def
	}
	switch c := v.(type) {
	case map[string]any:
		return [4]float64{num(c["r"], def[0]), num(c["g"], def[1]), num(c["b"], def[2]), num(c["a"], def[3])}
	case []any:
		if len(c) >= 4 {
			return [4]float64{num(c[0], def[0]), num(c[1], def[1]), num(c[2], def[2]), num(c[3], def[3])}
		}
	}
	return def
}

func boolParam(e *riq.Entity, key string) bool { return e.Float(key, 0) != 0 }

func boolParamDefault(e *riq.Entity, key string, def bool) bool {
	if _, ok := e.Data[key]; !ok {
		return def
	}
	return boolParam(e, key)
}

func num(v any, def float64) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	case bool:
		if n {
			return 1
		}
		return 0
	}
	return def
}

func toRGBA(c [4]float64) color.RGBA {
	return color.RGBA{uint8(clamp01(c[0]) * 255), uint8(clamp01(c[1]) * 255), uint8(clamp01(c[2]) * 255), uint8(clamp01(c[3]) * 255)}
}

func scaleAlpha(c [4]float64, alpha float64) [4]float64 {
	c[3] *= alpha
	return c
}

func blend(a, b [4]float64, t float64) [4]float64 {
	t = clamp01(t)
	return [4]float64{
		a[0] + (b[0]-a[0])*t,
		a[1] + (b[1]-a[1])*t,
		a[2] + (b[2]-a[2])*t,
		a[3] + (b[3]-a[3])*t,
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
