// Package builttoscaleds ports Built to Scale (DS/NTR) timing onto engine.App.
//
// The extracted DS bundle is mesh-only: it has Animator clips, sounds, and
// MeshRenderer bindings but no SpriteRenderer atlas. Static world geometry,
// dynamic block/parts prefabs, script-driven material colors, light patterns,
// and belt texture scrolling all run through the extracted MeshRenderer data.
package builttoscaleds

import (
	"image/color"
	"math"
	"sort"

	"github.com/hajimehoshi/ebiten/v2"

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
	lightTweenSec        = 0.2

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
	objectMat    string
	shooterMat   string
	gridMat      string
	elevatorMat  string
	beltMat      string
	beltSpeed    float64
	cameraFOV    float64
	cameraPivot  string
	cameraPos    string
	switchBeat   float64
	firstLights  []string
	secondLights []string

	shooterState string
	shooterBeat  float64
	lastShotOut  bool
}

func New() engine.Module {
	return &Module{
		objectColor: defaultObjectColor, shooterColor: defaultShooterColor,
		envColor: defaultEnvColor, beltSpeed: 1, shooterState: "Idle",
	}
}

func (m *Module) ID() string { return "builtToScaleDS" }

func (m *Module) Load(ctx *engine.Ctx) error {
	m.ctx = ctx
	if err := ctx.LoadAssets("builtToScaleDS"); err != nil {
		return err
	}
	m.loadMaterialRefs()
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

func (m *Module) loadMaterialRefs() {
	game := m.ctx.Assets.Extra.Components["game"]
	m.objectMat = strOr(game.Refs["objectMaterial"], "Object")
	m.shooterMat = strOr(game.Refs["shooterMaterial"], "Shooter")
	m.gridMat = strOr(game.Refs["gridPlaneMaterial"], "GridPlane")
	m.elevatorMat = strOr(game.Refs["elevatorMaterial"], "Grid")
	m.beltMat = strOr(game.Refs["beltMaterial"], "Belt")
	if v := game.Nums["beltSpeed"]; v != 0 {
		m.beltSpeed = v
	}
	m.cameraFOV = game.Nums["cameraFoV"]
	m.cameraPivot = m.ctx.Role("cameraPivot")
	m.cameraPos = m.ctx.Role("camPos")
	m.firstLights = append([]string(nil), game.RefArrays["firstPatternLights"]...)
	m.secondLights = append([]string(nil), game.RefArrays["secondPatternLights"]...)
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
}

func (m *Module) OnSwitch(beat float64) {
	m.switchBeat = beat
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

func (m *Module) Draw(screen *ebiten.Image, t, beat float64) {
	rot, zoom := m.cameraAt(beat)
	env := m.envAt(beat)
	screen.Fill(toRGBA(env))
	m.drawOfficialMeshScene(screen, env, rot, zoom, beat, t)
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
	return m.cameraAtFrom(beat, m.switchBeat)
}

func (m *Module) cameraAtFrom(beat, switchBeat float64) (rot, zoom float64) {
	rot, zoom = 0, 1
	for _, ev := range m.cams {
		if ev.beat < switchBeat {
			continue
		}
		if beat < ev.beat {
			break
		}
		startRot, startZoom := rot, zoom
		endRot := ev.rot
		if ev.additive {
			endRot = startRot + ev.rot
		}
		endZoom := ev.zoom
		u := 1.0
		if ev.length > 0 && beat < ev.beat+ev.length {
			u = (beat - ev.beat) / ev.length
		}
		rot = engine.Ease(ev.ease, startRot, endRot, u)
		zoom = engine.Ease(ev.ease, startZoom, endZoom, u)
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

func (m *Module) drawOfficialMeshScene(screen *ebiten.Image, env [4]float64, rot, zoom, beat, songTime float64) {
	m.applyMeshMaterials(env, beat, songTime)
	m.ctx.Scene.SetCameraFOV(m.cameraFOV)
	if pos, q, ok := m.cameraPose(rot, zoom); ok {
		m.ctx.Scene.SetCameraQuat(pos[0], pos[1], pos[2], q)
	} else {
		m.ctx.Scene.SetCameraYaw(rot)
	}
	m.ctx.SampleScene(beat)
	m.queueOfficialDynamicMeshes(beat)
	m.ctx.Scene.Draw(screen, m.proj)
}

func (m *Module) cameraPose(rotDeg, zoom float64) ([3]float64, [4]float64, bool) {
	if m.ctx == nil || m.ctx.Assets == nil || m.cameraPivot == "" || m.cameraPos == "" {
		return [3]float64{}, [4]float64{}, false
	}
	pi, ok := m.ctx.Assets.NodeIndex(m.cameraPivot)
	if !ok {
		return [3]float64{}, [4]float64{}, false
	}
	ci, ok := m.ctx.Assets.NodeIndex(m.cameraPos)
	if !ok {
		return [3]float64{}, [4]float64{}, false
	}
	pivot := m.ctx.Assets.Rig.Nodes[pi]
	cam := m.ctx.Assets.Rig.Nodes[ci]
	if zoom <= 0 || math.IsNaN(zoom) || math.IsInf(zoom, 0) {
		zoom = 1
	}
	yaw := quatFromYaw(rotDeg * math.Pi / 180)
	local := [3]float64{cam.Pos[0] * zoom, cam.Pos[1] * zoom, cam.PosZ * zoom}
	offset := quatRotate(yaw, local)
	worldPos := [3]float64{
		pivot.Pos[0] + offset[0],
		pivot.Pos[1] + offset[1],
		pivot.PosZ + offset[2],
	}
	localQ := nodeQuatOrZ(cam.Quat, cam.RotZ)
	worldQ := quatNormalize(quatMul(yaw, localQ))
	forward := quatRotate(worldQ, [3]float64{0, 0, 1})
	// BuiltToScaleDS.cs sets GameCamera.AdditionalPosition to
	// camPos.position + camPos.forward*10 every frame. Applying the same offset
	// keeps CameraPos pitch/yaw authoritative instead of treating rot as a 2D
	// screen-space spin.
	worldPos[0] += forward[0] * kart.CamDist
	worldPos[1] += forward[1] * kart.CamDist
	worldPos[2] += forward[2] * kart.CamDist
	return worldPos, worldQ, true
}

func (m *Module) applyMeshMaterials(env [4]float64, beat, songTime float64) {
	sc := m.ctx.Scene
	sc.SetMaterialColorFor(m.objectMat, m.objectColor)
	sc.SetMaterialColorFor(m.shooterMat, m.shooterColor)
	sc.SetMaterialColorFor(m.beltMat, env)
	sc.SetMaterialColorFor(m.gridMat, env)
	sc.SetMaterialColorFor(m.elevatorMat, env)

	first, second := m.lightColorsAt(beat, env)
	for _, mat := range m.firstLights {
		sc.SetMaterialColorFor(mat, first)
	}
	for _, mat := range m.secondLights {
		sc.SetMaterialColorFor(mat, second)
	}
	sc.SetMaterialTextureOffsetFor(m.beltMat, beltTextureOffset(m.beltSpeed, songTime))
}

func (m *Module) lightColorsAt(beat float64, env [4]float64) ([4]float64, [4]float64) {
	active, first, transitionBeat := m.lightPatternAt(beat)
	targetA, targetB := lightTargets(active, first, env)
	if m.ctx == nil || transitionBeat < 0 {
		return targetA, targetB
	}
	transitionTime := m.ctx.BeatToTime(transitionBeat)
	elapsed := m.ctx.Time() - transitionTime
	if elapsed < 0 || elapsed >= lightTweenSec {
		return targetA, targetB
	}
	prevBeat := m.ctx.TimeToBeat(transitionTime - 1e-6)
	prevActive, prevFirst, _ := m.lightPatternAt(prevBeat)
	prevA, prevB := lightTargets(prevActive, prevFirst, m.envAt(prevBeat))
	u := clamp01(elapsed / lightTweenSec)
	return lerpColor(prevA, targetA, u), lerpColor(prevB, targetB, u)
}

func (m *Module) lightPatternAt(beat float64) (active, first bool, transitionBeat float64) {
	active, first, transitionBeat = false, true, -1
	for _, ev := range m.lights {
		if beat < ev.beat {
			break
		}
		if ev.auto {
			n := math.Floor(math.Max(0, beat-ev.beat))
			active = true
			first = int(n)%2 == 0
			transitionBeat = ev.beat + n
			continue
		}
		if ev.light && beat < ev.beat+ev.length {
			n := math.Floor(math.Max(0, beat-ev.beat))
			active = true
			first = int(n)%2 == 0
			transitionBeat = ev.beat + n
			continue
		}
		active, first = false, true
		if ev.light && ev.length > 0 && beat >= ev.beat+ev.length {
			transitionBeat = ev.beat + ev.length
		} else {
			transitionBeat = ev.beat
		}
	}
	return active, first, transitionBeat
}

func lightTargets(active, first bool, env [4]float64) ([4]float64, [4]float64) {
	a, b := env, env
	if !active {
		return a, b
	}
	if first {
		a = [4]float64{1, 1, 1, 1}
	} else {
		b = [4]float64{1, 1, 1, 1}
	}
	return a, b
}

func beltTextureOffset(speed, songTime float64) [2]float64 {
	return [2]float64{0, math.Mod(-speed*songTime, 1)}
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

func laneY() float64    { return 348 }
func shooterX() float64 { return 750 }

func semitonePitch(semitone int) float64 { return math.Exp2(float64(semitone) / 12) }

func nodeQuatOrZ(q []float64, rotZ float64) [4]float64 {
	if len(q) >= 4 {
		return quatNormalize([4]float64{q[0], q[1], q[2], q[3]})
	}
	return quatFromZ(rotZ)
}

func quatFromYaw(rad float64) [4]float64 {
	return [4]float64{0, math.Sin(rad / 2), 0, math.Cos(rad / 2)}
}

func quatFromZ(rad float64) [4]float64 {
	return [4]float64{0, 0, math.Sin(rad / 2), math.Cos(rad / 2)}
}

func quatNormalize(q [4]float64) [4]float64 {
	n := math.Sqrt(q[0]*q[0] + q[1]*q[1] + q[2]*q[2] + q[3]*q[3])
	if n <= 0 || math.IsNaN(n) || math.IsInf(n, 0) {
		return [4]float64{0, 0, 0, 1}
	}
	return [4]float64{q[0] / n, q[1] / n, q[2] / n, q[3] / n}
}

func quatMul(a, b [4]float64) [4]float64 {
	return [4]float64{
		a[3]*b[0] + a[0]*b[3] + a[1]*b[2] - a[2]*b[1],
		a[3]*b[1] - a[0]*b[2] + a[1]*b[3] + a[2]*b[0],
		a[3]*b[2] + a[0]*b[1] - a[1]*b[0] + a[2]*b[3],
		a[3]*b[3] - a[0]*b[0] - a[1]*b[1] - a[2]*b[2],
	}
}

func quatRotate(q [4]float64, v [3]float64) [3]float64 {
	q = quatNormalize(q)
	cx := q[1]*v[2] - q[2]*v[1] + q[3]*v[0]
	cy := q[2]*v[0] - q[0]*v[2] + q[3]*v[1]
	cz := q[0]*v[1] - q[1]*v[0] + q[3]*v[2]
	return [3]float64{
		v[0] + 2*(q[1]*cz-q[2]*cy),
		v[1] + 2*(q[2]*cx-q[0]*cz),
		v[2] + 2*(q[0]*cy-q[1]*cx),
	}
}

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

func strOr(v, def string) string {
	if v != "" {
		return v
	}
	return def
}

func toRGBA(c [4]float64) color.RGBA {
	return color.RGBA{uint8(clamp01(c[0]) * 255), uint8(clamp01(c[1]) * 255), uint8(clamp01(c[2]) * 255), uint8(clamp01(c[3]) * 255)}
}

func lerpColor(a, b [4]float64, t float64) [4]float64 {
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
