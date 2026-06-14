// Package gardendance ports Garden Dance's flower body/face animation layers,
// repeated dance loop, pose/triplet cues, Mr. Bird, and sun timeline controls.
package gardendance

import (
	"image/color"
	"math"
	"math/rand"
	"sort"

	"github.com/hajimehoshi/ebiten/v2"

	"hsdemo/engine"
	"hsdemo/kart"
	"hsdemo/riq"
)

const (
	whoFlowers = iota
	whoSun
	whoBoth
	whoNone
)

const (
	sunEnter = iota
	sunExit
)

type bopEvt struct {
	beat, length float64
	auto, bop    int
}

type danceEvt struct {
	beat float64
}

type poseEvt struct {
	beat    float64
	triplet bool
}

type forceEvt struct {
	beat, length float64
}

type birdEvt struct {
	beat, length float64
	side         int
}

type sunEvt struct {
	beat, length float64
	anim         int
	instant      bool
}

type flowerState struct {
	path       string
	danceRight bool
	canBlink   bool
}

type Module struct {
	ctx  *engine.Ctx
	proj kart.Aff

	playerPath string
	sunPath    string
	birdPath   string
	npcPaths   []string
	flowers    map[string]*flowerState

	bops   []bopEvt
	dances []danceEvt
	poses  []poseEvt
	forces []forceEvt
	birds  []birdEvt
	suns   []sunEvt

	flowerBop  bool
	sunBop     bool
	sunActive  bool
	birdActive bool

	sunBeat, sunLength   float64
	sunClip              string
	birdBeat, birdLength float64
	birdClip             string

	npcAnger          int
	keepDancing       bool
	startNoDance      float64
	startRegularDance float64
	endDance          float64
	lastPulse         float64
	rng               *rand.Rand
	endBeat           float64
}

func New() engine.Module {
	return &Module{
		flowers:           map[string]*flowerState{},
		flowerBop:         true,
		sunBop:            true,
		sunActive:         true,
		startNoDance:      math.Inf(-1),
		startRegularDance: math.Inf(-1),
		endDance:          math.Inf(-1),
		lastPulse:         math.Inf(-1),
		rng:               rand.New(rand.NewSource(20250603)),
	}
}

func (m *Module) ID() string { return "gardenDance" }

func (m *Module) Load(ctx *engine.Ctx) error {
	m.ctx = ctx
	if err := ctx.LoadAssets("gardenDance"); err != nil {
		return err
	}
	m.proj = kart.Translate(engine.ScreenW/2, engine.ScreenH/2).Mul(kart.Scale(54, -54))
	m.playerPath = roleOr(ctx, "flowerPlayer", "PurpleFlower")
	m.sunPath = roleOr(ctx, "sunAnim", "Sun")
	m.birdPath = roleOr(ctx, "birdAnim", "Mr. Bird")
	m.npcPaths = append(m.npcPaths, ctx.Assets.Extra.RefArrays["flowers"]...)
	m.readFlowerComponents()
	return nil
}

func (m *Module) readFlowerComponents() {
	for _, comp := range m.ctx.Assets.Extra.Components {
		if comp.Path == "" {
			continue
		}
		m.flowers[comp.Path] = &flowerState{
			path:       comp.Path,
			danceRight: comp.Nums["danceRight"] != 0,
			canBlink:   true,
		}
	}
	for _, p := range append([]string{m.playerPath}, m.npcPaths...) {
		if _, ok := m.flowers[p]; !ok {
			m.flowers[p] = &flowerState{path: p, canBlink: true}
		}
	}
}

func (m *Module) OnEvent(e *riq.Entity) {
	if end := e.Beat + e.Length; end > m.endBeat {
		m.endBeat = end
	}
	switch e.Datamodel {
	case "gardenDance/bop":
		m.bops = append(m.bops, bopEvt{
			beat: e.Beat, length: e.Length,
			auto: intParam(e, "auto", whoNone),
			bop:  intParam(e, "toggle", whoBoth),
		})
	case "gardenDance/dance":
		m.dances = append(m.dances, danceEvt{beat: e.Beat - 1})
	case "gardenDance/stop":
		b := e.Beat
		m.ctx.At(b, func() {
			m.keepDancing = false
			m.endDance = b
		})
	case "gardenDance/pose":
		m.poses = append(m.poses, poseEvt{beat: e.Beat})
	case "gardenDance/triplet":
		m.poses = append(m.poses, poseEvt{beat: e.Beat, triplet: true})
	case "gardenDance/force":
		m.forces = append(m.forces, forceEvt{beat: e.Beat, length: e.Length})
	case "gardenDance/bird":
		m.birds = append(m.birds, birdEvt{beat: e.Beat, length: e.Length, side: intParam(e, "side", 0)})
	case "gardenDance/sun":
		m.suns = append(m.suns, sunEvt{
			beat: e.Beat, length: e.Length,
			anim:    intParam(e, "whichAnim", sunEnter),
			instant: boolParam(e, "instant"),
		})
	}
}

func (m *Module) Ready() {
	sort.Slice(m.bops, func(i, j int) bool { return m.bops[i].beat < m.bops[j].beat })
	sort.Slice(m.dances, func(i, j int) bool { return m.dances[i].beat < m.dances[j].beat })
	sort.Slice(m.poses, func(i, j int) bool { return m.poses[i].beat < m.poses[j].beat })
	sort.Slice(m.forces, func(i, j int) bool { return m.forces[i].beat < m.forces[j].beat })
	sort.Slice(m.birds, func(i, j int) bool { return m.birds[i].beat < m.birds[j].beat })
	sort.Slice(m.suns, func(i, j int) bool { return m.suns[i].beat < m.suns[j].beat })

	for _, ev := range m.bops {
		ev := ev
		m.ctx.At(ev.beat, func() {
			m.flowerBop = ev.auto == whoFlowers || ev.auto == whoBoth
			m.sunBop = ev.auto == whoSun || ev.auto == whoBoth
		})
		for b := ev.beat; b < ev.beat+ev.length-1e-6; b++ {
			bb := b
			m.ctx.At(bb, func() { m.doBop(bb, ev.bop) })
		}
	}
	for _, ev := range m.dances {
		b := ev.beat
		m.ctx.At(b, func() {
			m.keepDancing = true
			m.scheduleDanceStep(b)
		})
	}
	for _, ev := range m.poses {
		ev := ev
		if ev.triplet {
			m.scheduleTriplet(ev.beat)
		} else {
			m.schedulePose(ev.beat)
		}
	}
	for _, ev := range m.forces {
		ev := ev
		for i := 0; i < int(ev.length); i++ {
			b := ev.beat + float64(i)
			m.ctx.At(b, func() { m.npcDance(false) })
			m.ctx.ScheduleInputAction(b, 0, m.danceHit, m.danceMiss)
		}
	}
	for _, ev := range m.birds {
		ev := ev
		m.ctx.At(ev.beat, func() { m.startBird(ev.beat, ev.length, ev.side) })
	}
	for _, ev := range m.suns {
		ev := ev
		m.ctx.At(ev.beat, func() { m.startSun(ev.beat, ev.length, ev.anim, ev.instant) })
	}
}

func (m *Module) OnSwitch(beat float64) {
	sec := m.ctx.SecPerBeat(beat)
	for _, p := range append([]string{m.playerPath, m.sunPath, m.birdPath}, m.npcPaths...) {
		m.ctx.Scene.PlayDefaultState(p, beat, sec)
	}
	m.lastPulse = math.Floor(beat)
	m.restoreStateAt(beat)
}

func (m *Module) restoreStateAt(beat float64) {
	m.keepDancing = false
	m.flowerBop, m.sunBop = true, true
	m.sunActive, m.birdActive = true, false
	m.sunClip, m.birdClip = "", ""
	for _, ev := range m.bops {
		if ev.beat <= beat {
			m.flowerBop = ev.auto == whoFlowers || ev.auto == whoBoth
			m.sunBop = ev.auto == whoSun || ev.auto == whoBoth
		}
	}
	for _, ev := range m.dances {
		if ev.beat <= beat {
			m.keepDancing = true
		}
	}
	for _, ev := range m.poses {
		if ev.beat <= beat && beat < ev.beat+4 {
			m.startNoDance = ev.beat + 2
			m.startRegularDance = ev.beat + 4
		}
	}
	for _, ev := range m.suns {
		if ev.beat > beat {
			break
		}
		if ev.instant {
			if ev.anim == sunEnter {
				m.sunActive = true
				m.ctx.Scene.PlayState(m.sunPath, "Idle", ev.beat, 0.5)
			} else {
				m.sunActive = false
				m.ctx.Scene.PlayState(m.sunPath, "Hide", ev.beat, 0.5)
			}
			continue
		}
		if beat < ev.beat+ev.length {
			m.startSun(ev.beat, ev.length, ev.anim, false)
		} else {
			m.sunActive = ev.anim == sunEnter
			if m.sunActive {
				m.ctx.Scene.PlayState(m.sunPath, "Idle", beat, 0.5)
			} else {
				m.ctx.Scene.PlayState(m.sunPath, "Hide", beat, 0.5)
			}
		}
	}
	for _, ev := range m.birds {
		if beat >= ev.beat && beat < ev.beat+ev.length {
			m.startBird(ev.beat, ev.length, ev.side)
		}
	}
}

func (m *Module) Whiff(beat float64) {
	m.flowerDance(m.playerPath, beat, false, false, false)
	for _, p := range m.npcPaths {
		m.flowerGlare(p, beat)
	}
}

func (m *Module) Update(_, beat float64) {
	m.updateSun(beat)
	m.updateBird(beat)
	if p := math.Floor(beat); p > m.lastPulse {
		m.lastPulse = p
		if m.flowerBop {
			m.doBop(p, whoFlowers)
		}
		if m.sunBop {
			m.doBop(p, whoSun)
		}
		if m.birdActive {
			m.ctx.Scene.PlayStateLayer("gardenDance/bird/flap", m.birdPath, "Flap", p, 0.5)
		}
	}
	for _, f := range m.flowers {
		if f.canBlink && m.rng.Intn(600) == 0 {
			m.ctx.Scene.PlayStateLayer(f.path+"/face", f.path, "Blink", beat, 0.5)
		}
	}
}

func (m *Module) Draw(screen *ebiten.Image, _, beat float64) {
	screen.Fill(color.RGBA{0x92, 0xcb, 0xe8, 0xff})
	m.ctx.SampleScene(beat)
	m.ctx.Scene.Draw(screen, m.proj)
}

func (m *Module) scheduleDanceStep(beat float64) {
	betweenDanceValues := beat+1 < m.startRegularDance && beat+1 >= m.startNoDance
	if !betweenDanceValues {
		m.ctx.ScheduleInputAction(beat+1, 0, m.danceHit, m.danceMiss)
		m.ctx.At(beat+1, func() { m.npcDance(false) })
	}
	m.ctx.At(beat+1, func() {
		if m.keepDancing && beat+1 <= m.endBeat+8 {
			m.scheduleDanceStep(beat + 1)
		}
	})
}

func (m *Module) schedulePose(beat float64) {
	m.ctx.At(beat, func() {
		m.startNoDance = beat + 2
		m.startRegularDance = beat + 4
	})
	m.ctx.SoundAt(beat, "whistle1", 1)
	m.ctx.SoundAt(beat+0.5, "whistle2", 1)
	m.ctx.SoundAt(beat+1, "whistle3", 1)
	m.ctx.ScheduleInputAction(beat+2, 0, m.pDanceHit, m.danceMiss)
	m.ctx.ScheduleInputAction(beat+2.5, 0, m.poseHit, m.danceMiss)
	for _, b := range []float64{beat, beat + 0.5, beat + 1} {
		bb := b
		m.ctx.At(bb, func() { m.birdWhistle(bb) })
	}
	m.ctx.At(beat+2, func() { m.npcDance(true) })
	m.ctx.At(beat+2.5, func() { m.npcPose() })
}

func (m *Module) scheduleTriplet(beat float64) {
	m.ctx.At(beat, func() {
		m.startNoDance = beat + 2
		m.startRegularDance = beat + 4
	})
	m.ctx.SoundAt(beat, "tripletWhistle", 1)
	m.ctx.SoundAt(beat+1, "tripletWhistle", 1)
	m.ctx.ScheduleInputAction(beat+2, 0, m.tripletHit, m.danceMiss)
	m.ctx.ScheduleInputAction(beat+2.66667, 0, m.pDanceHit, m.danceMiss)
	m.ctx.ScheduleInputAction(beat+3.3333302, 0, m.poseHit, m.danceMiss)
	m.ctx.At(beat, func() { m.birdWhistle(beat) })
	m.ctx.At(beat+1, func() { m.birdWhistle(beat + 1) })
	m.ctx.At(beat+2, func() { m.npcTriplet() })
	m.ctx.At(beat+2.66667, func() { m.npcDance(true) })
	m.ctx.At(beat+3.3333302, func() { m.npcPose() })
}

func (m *Module) doBop(beat float64, bop int) {
	canFlowerBop := !m.keepDancing && (beat < m.startNoDance || beat > m.startRegularDance) && beat > m.endDance
	switch bop {
	case whoFlowers:
		if canFlowerBop {
			m.bopFlowers(beat)
		}
	case whoSun:
		if m.sunActive {
			m.ctx.Scene.PlayState(m.sunPath, "Bop", beat, 0.5)
		}
	case whoBoth:
		if canFlowerBop {
			m.bopFlowers(beat)
		}
		if m.sunActive {
			m.ctx.Scene.PlayState(m.sunPath, "Bop", beat, 0.5)
		}
	}
}

func (m *Module) bopFlowers(beat float64) {
	for _, p := range append([]string{m.playerPath}, m.npcPaths...) {
		m.flowerBopAt(p, beat)
	}
	m.npcAnger = 0
}

func (m *Module) flowerBopAt(path string, beat float64) {
	m.ctx.Scene.PlayStateLayer(path+"/body", path, "Bop", beat, 0.5)
	m.ctx.Scene.PlayStateLayer(path+"/face", path, "IdleFace", beat, 0.5)
	if f := m.flowers[path]; f != nil {
		f.canBlink = true
	}
}

func (m *Module) flowerGlare(path string, beat float64) {
	m.ctx.Scene.PlayStateLayer(path+"/face", path, "Glare", beat, 0.5)
}

func (m *Module) flowerDance(path string, beat float64, hit, barely, pose bool) {
	f := m.flower(path)
	state := "DanceL"
	if pose {
		state = "PDanceL"
	}
	if f.danceRight {
		if pose {
			state = "PDanceR"
		} else {
			state = "DanceR"
		}
	}
	m.ctx.Scene.PlayStateLayer(path+"/body", path, state, beat, 0.5)
	if hit {
		face := "IdleFace"
		if barely {
			face = "Barely"
		}
		m.ctx.Scene.PlayStateLayer(path+"/face", path, face, beat, 0.5)
	} else {
		m.ctx.Scene.PlayStateLayer(path+"/face", path, "MissFace", beat, 0.5)
	}
	if barely {
		m.ctx.Sound("nearMiss")
	} else {
		m.ctx.Sound("dance")
	}
	f.danceRight = !f.danceRight
}

func (m *Module) flowerPose(path string, beat float64, player, barely, glare bool) {
	f := m.flower(path)
	state := "PoseL"
	if f.danceRight {
		state = "PoseR"
	}
	m.ctx.Scene.PlayStateLayer(path+"/body", path, state, beat, 0.5)
	if !glare {
		face := "PoseFace"
		if barely {
			face = "Barely"
		}
		m.ctx.Scene.PlayStateLayer(path+"/face", path, face, beat, 0.5)
	}
	if barely {
		m.ctx.Sound("nearMiss")
	} else {
		m.ctx.Sound("dance")
	}
	if player && !barely {
		m.ctx.Sound("sparkle")
	}
	f.danceRight = !f.danceRight
}

func (m *Module) flowerTriplet(path string, beat float64, player, barely, glare bool) {
	f := m.flower(path)
	state := "TripletL"
	if f.danceRight {
		state = "TripletR"
	}
	m.ctx.Scene.PlayStateLayer(path+"/body", path, state, beat, 0.5)
	if !glare {
		face := "TripletFace"
		if barely {
			face = "Barely"
		}
		m.ctx.Scene.PlayStateLayer(path+"/face", path, face, beat, 0.5)
	}
	if barely {
		m.ctx.Sound("nearMiss")
	} else {
		m.ctx.Sound("sway")
	}
	f.danceRight = !f.danceRight
}

func (m *Module) flowerMiss(path string, beat float64) {
	m.ctx.Scene.PlayStateLayer(path+"/face", path, "MissFace", beat, 0.5)
}

func (m *Module) flower(path string) *flowerState {
	if f := m.flowers[path]; f != nil {
		return f
	}
	f := &flowerState{path: path, canBlink: true}
	m.flowers[path] = f
	return f
}

func (m *Module) npcDance(pose bool) {
	beat := m.ctx.App.BeatNow()
	for _, p := range m.npcPaths {
		m.flowerDance(p, beat, true, false, pose)
		if m.npcAnger > 0 {
			m.flowerGlare(p, beat)
		}
	}
}

func (m *Module) npcPose() {
	beat := m.ctx.App.BeatNow()
	for _, p := range m.npcPaths {
		m.flowerPose(p, beat, false, false, m.npcAnger > 0)
	}
}

func (m *Module) npcTriplet() {
	beat := m.ctx.App.BeatNow()
	for _, p := range m.npcPaths {
		m.flowerTriplet(p, beat, false, false, m.npcAnger > 0)
	}
}

func (m *Module) danceHit(state float64, _ engine.Judgment) {
	beat := m.ctx.App.BeatNow()
	m.flowerDance(m.playerPath, beat, true, math.Abs(state) >= 1, false)
	if m.npcAnger > 0 {
		m.npcAnger--
	}
}

func (m *Module) pDanceHit(state float64, _ engine.Judgment) {
	beat := m.ctx.App.BeatNow()
	m.flowerDance(m.playerPath, beat, true, math.Abs(state) >= 1, true)
	if m.npcAnger > 0 {
		m.npcAnger--
	}
}

func (m *Module) tripletHit(state float64, _ engine.Judgment) {
	beat := m.ctx.App.BeatNow()
	m.flowerTriplet(m.playerPath, beat, true, math.Abs(state) >= 1, false)
	if m.npcAnger > 0 {
		m.npcAnger--
	}
}

func (m *Module) poseHit(state float64, _ engine.Judgment) {
	beat := m.ctx.App.BeatNow()
	m.flowerPose(m.playerPath, beat, true, math.Abs(state) >= 1, false)
	if m.npcAnger > 0 {
		m.npcAnger--
	}
}

func (m *Module) danceMiss() {
	beat := m.ctx.App.BeatNow()
	for _, p := range m.npcPaths {
		m.flowerGlare(p, beat)
	}
	m.flowerMiss(m.playerPath, beat)
	m.npcAnger = 3
}

func (m *Module) startBird(beat, length float64, side int) {
	if length <= 0 {
		length = 1
	}
	m.birdActive = true
	m.birdBeat = beat
	m.birdLength = length
	m.birdClip = "Bird/FlyLeft"
	if side != 0 {
		m.birdClip = "Bird/FlyRight"
	}
}

func (m *Module) updateBird(beat float64) {
	if m.birdClip == "" {
		return
	}
	u := (beat - m.birdBeat) / m.birdLength
	if u > 1 {
		m.birdClip = ""
		m.birdActive = false
		return
	}
	m.ctx.Scene.PlayNormalized(m.birdPath, m.birdClip, u)
}

func (m *Module) birdWhistle(beat float64) {
	if m.birdActive {
		m.ctx.Scene.PlayStateLayer("gardenDance/bird/whistle", m.birdPath, "Whistle", beat, 0.5)
	}
}

func (m *Module) startSun(beat, length float64, anim int, instant bool) {
	if length <= 0 {
		length = 1
	}
	if instant {
		if anim == sunEnter {
			m.sunActive = true
			m.ctx.Scene.PlayState(m.sunPath, "Idle", beat, 0.5)
		} else {
			m.sunActive = false
			m.ctx.Scene.PlayState(m.sunPath, "Hide", beat, 0.5)
		}
		m.sunClip = ""
		return
	}
	m.sunBeat = beat
	m.sunLength = length
	m.sunClip = "Sun/SunEnter"
	m.sunActive = true
	if anim == sunExit {
		m.sunClip = "Sun/SunLeave"
		m.sunActive = false
	}
}

func (m *Module) updateSun(beat float64) {
	if m.sunClip == "" {
		return
	}
	u := (beat - m.sunBeat) / m.sunLength
	if u > 1 {
		m.sunClip = ""
		return
	}
	m.ctx.Scene.PlayNormalized(m.sunPath, m.sunClip, u)
}

func roleOr(ctx *engine.Ctx, key, fallback string) string {
	if v := ctx.Role(key); v != "" {
		return v
	}
	return fallback
}

func intParam(e *riq.Entity, key string, def int) int { return int(e.Float(key, float64(def))) }

func boolParam(e *riq.Entity, key string) bool { return e.Float(key, 0) != 0 }
