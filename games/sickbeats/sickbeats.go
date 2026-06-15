// Package sickbeats ports Sick Beats' four-way virus movement, fork/key input
// feedback, organism damage chain, doctor movement, recolors, and background
// color events from Assets/Scripts/Games/SickBeats.
package sickbeats

import (
	"fmt"
	"image/color"
	"math"
	"math/rand"
	"sort"
	"strconv"

	"github.com/hajimehoshi/ebiten/v2"

	"hsdemo/engine"
	"hsdemo/kart"
	"hsdemo/riq"
)

const (
	dirRight = iota
	dirUp
	dirLeft
	dirDown
)

const (
	actionLeft  = 1
	actionRight = 2
	actionDown  = 3
	actionUp    = 4
)

const refillBeat = 1.5

var (
	defaultBG      = [4]float64{0, 149.0 / 255.0, 0, 1}
	defaultPipe    = [4]float64{43.0 / 255.0, 1, 0, 1}
	defaultOutline = [4]float64{33.0 / 255.0, 1, 0, 1}
	defaultCorner  = [4]float64{52.0 / 255.0, 1, 0, 1}

	defaultVirusColors = [4][4]float64{
		{0, 1, 1, 1},
		{1, 0.25, 0.75, 1},
		{0, 0, 0, 1},
		{1, 1, 1, 1},
	}

	dashPatterns = [][]float64{
		{2},
		{1.75, 2},
		{1.75, 1.875, 2},
	}
)

type bopEvt struct {
	beat, length float64
	bop, auto    bool
}

type bgEvt struct {
	beat, length       float64
	bg0, bg1           [4]float64
	pipe0, pipe1       [4]float64
	outline0, outline1 [4]float64
	corner0, corner1   [4]float64
	ease               int
}

type colorEvt struct {
	beat   float64
	colors [4][4]float64
}

type doctorMoveEvt struct {
	beat, length              float64
	doMove, doRotate, doScale bool
	startMove, endMove        [2]float64
	startRot, endRot          float64
	startScale, endScale      [2]float64
	sticky                    bool
	ease                      int
}

type colorEase struct {
	beat, length float64
	start, end   [4]float64
	ease         int
}

type virus struct {
	mod       *Module
	inst      *kart.Instance
	startBeat float64
	position  int
	life      int
	isJust    bool
	dead      bool
}

type Module struct {
	ctx  *engine.Ctx
	proj kart.Aff

	virusT  *kart.Template
	viruses []*virus

	bops      []bopEvt
	bgEvts    []bgEvt
	colorEvts []colorEvt
	moves     []doctorMoveEvt

	bgColor      colorEase
	pipeColor    colorEase
	outlineColor colorEase
	cornerColor  colorEase
	virusColors  [4][4]float64

	isForkPop [4]bool
	isMiss    [4]bool
	isPrepare [4]bool
	orgAlive  bool

	gameEndBeat  float64
	docShockBeat float64
	lastPulse    int

	doctorBasePos   [2]float64
	doctorBaseScale [2]float64
}

func New() engine.Module {
	m := &Module{
		gameEndBeat:  math.Inf(1),
		docShockBeat: math.Inf(-1),
		lastPulse:    math.MinInt,
		bgColor:      colorEase{start: defaultBG, end: defaultBG},
		pipeColor:    colorEase{start: defaultPipe, end: defaultPipe},
		outlineColor: colorEase{start: defaultOutline, end: defaultOutline},
		cornerColor:  colorEase{start: defaultCorner, end: defaultCorner},
		virusColors:  defaultVirusColors,
	}
	for i := range m.isForkPop {
		m.isForkPop[i] = true
	}
	m.orgAlive = true
	return m
}

func (m *Module) ID() string { return "sickBeats" }

func (m *Module) Load(ctx *engine.Ctx) error {
	m.ctx = ctx
	if err := ctx.LoadAssets("sickBeats"); err != nil {
		return err
	}
	m.proj = kart.Translate(engine.ScreenW/2, engine.ScreenH/2).Mul(kart.Scale(54, -54))
	m.virusT = kart.NewTemplate(ctx.Assets, "Prefabs/virus")
	if m.virusT == nil {
		return fmt.Errorf("sickBeats: missing virus prefab template")
	}
	if idx, ok := ctx.Assets.NodeIndex(doctorRoot); ok {
		n := ctx.Assets.Rig.Nodes[idx]
		m.doctorBasePos = n.Pos
		m.doctorBaseScale = n.Scale
	} else {
		return fmt.Errorf("sickBeats: missing doctor transform %q", doctorRoot)
	}
	m.initScene(0)
	return nil
}

func (m *Module) initScene(beat float64) {
	sec := m.ctx.SecPerBeat(math.Max(beat, 0))
	sc := m.ctx.Scene
	for _, root := range []string{doctorAnim, radioAnim, orgAnim, keyAnim, forkRight, forkUp, forkLeft, forkDown} {
		sc.PlayDefaultState(root, beat, sec)
	}
	sc.SetActive("Prefabs/virus", false)
	for i := range m.isForkPop {
		m.isForkPop[i] = true
		m.isMiss[i], m.isPrepare[i] = false, false
	}
	m.orgAlive = true
}

func (m *Module) OnSwitch(beat float64) {
	m.gameEndBeat = m.ctx.NextSwitchBeat(beat)
	m.lastPulse = int(math.Floor(beat))
	m.persistVirusColor(beat)
	m.persistBGColor(beat)
	m.initScene(beat)
}

func (m *Module) OnEvent(e *riq.Entity) {
	b := e.Beat
	switch e.Datamodel {
	case "sickBeats/bop":
		ev := bopEvt{
			beat: b, length: e.Length,
			bop:  boolParamDefault(e, "toggle2", true),
			auto: boolParam(e, "toggle"),
		}
		m.bops = append(m.bops, ev)
		if ev.bop {
			for i := 0; float64(i) < ev.length; i++ {
				bb := b + float64(i)
				m.ctx.At(bb, func() { m.bopOnce(bb) })
			}
		}
	case "sickBeats/virus":
		dir, typ := int(e.Float("direction", dirRight)), int(e.Float("type", 0))
		m.ctx.At(b, func() { m.moveVirus(b, clampDir(dir), clampLife(typ)) })
	case "sickBeats/appear":
		dir, typ := int(e.Float("direction", dirRight)), int(e.Float("type", 0))
		m.ctx.At(b, func() {
			v := m.spawnVirus(b, clampDir(dir), clampLife(typ))
			v.appear()
		})
	case "sickBeats/dash":
		dir, typ := int(e.Float("direction", dirUp)), int(e.Float("type", 0))
		beats := []float64{e.Float("param1", 0), e.Float("param2", 0.125), e.Float("param3", 0.25)}
		m.ctx.At(b, func() { m.virusDashManual(b, clampDirMinOne(dir), clampLife(typ), beats) })
	case "sickBeats/summon":
		typ := int(e.Float("type", 0))
		m.ctx.At(b, func() { m.virusSummonManual(b, clampLife(typ)) })
	case "sickBeats/BGApp":
		ev := bgEvt{
			beat: b, length: e.Length,
			bg0: colorParam(e, "start", defaultBG), bg1: colorParam(e, "end", defaultBG),
			pipe0: colorParam(e, "startPipe", defaultPipe), pipe1: colorParam(e, "endPipe", defaultPipe),
			outline0: colorParam(e, "startOutline", defaultOutline), outline1: colorParam(e, "endOutline", defaultOutline),
			corner0: colorParam(e, "startCorner", defaultCorner), corner1: colorParam(e, "endCorner", defaultCorner),
			ease: int(e.Float("ease", 0)),
		}
		m.bgEvts = append(m.bgEvts, ev)
		m.ctx.At(b, func() { m.applyBGEvent(ev) })
	case "sickBeats/virusColor":
		ev := colorEvt{beat: b, colors: [4][4]float64{
			colorParam(e, "colorVirus1", defaultVirusColors[0]),
			colorParam(e, "colorVirus2", defaultVirusColors[1]),
			colorParam(e, "colorVirus3", defaultVirusColors[2]),
			colorParam(e, "colorVirus4", defaultVirusColors[3]),
		}}
		m.colorEvts = append(m.colorEvts, ev)
		m.ctx.At(b, func() { m.updateVirusColors(ev.colors) })
	case "sickBeats/moveDoctor":
		m.moves = append(m.moves, doctorMoveEvt{
			beat: b, length: e.Length,
			doMove: boolParam(e, "doMove"), doRotate: boolParam(e, "doRotate"), doScale: boolParam(e, "doScale"),
			startMove: [2]float64{e.Float("startMoveX", 0), e.Float("startMoveY", 0)},
			endMove:   [2]float64{e.Float("endMoveX", 0), e.Float("endMoveY", 0)},
			startRot:  e.Float("startRotDegrees", 0), endRot: e.Float("endRotDegrees", 0),
			startScale: [2]float64{e.Float("startScaleX", 1), e.Float("startScaleY", 1)},
			endScale:   [2]float64{e.Float("endScaleX", 1), e.Float("endScaleY", 1)},
			sticky:     boolParam(e, "sticky"),
			ease:       int(e.Float("ease", 0)),
		})
	}
}

func (m *Module) Ready() {
	sort.Slice(m.bops, func(i, j int) bool { return m.bops[i].beat < m.bops[j].beat })
	sort.Slice(m.bgEvts, func(i, j int) bool { return m.bgEvts[i].beat < m.bgEvts[j].beat })
	sort.Slice(m.colorEvts, func(i, j int) bool { return m.colorEvts[i].beat < m.colorEvts[j].beat })
	sort.Slice(m.moves, func(i, j int) bool { return m.moves[i].beat < m.moves[j].beat })
}

func (m *Module) Update(_, beat float64) {
	if pulse := int(math.Floor(beat)); pulse > m.lastPulse {
		for p := m.lastPulse + 1; p <= pulse; p++ {
			if m.autoBopAt(float64(p)) {
				m.bopOnce(float64(p))
			}
		}
		m.lastPulse = pulse
	}
	m.applyBackgroundColors(beat)
	m.applyDoctorTransform(beat)
	m.ctx.SampleScene(beat)
	m.keepLiveViruses(beat)
}

func (m *Module) Draw(screen *ebiten.Image, _, beat float64) {
	screen.Fill(toRGBA(m.bgColor.at(beat)))
	for _, v := range m.viruses {
		if !v.dead {
			v.inst.Queue(m.ctx.Scene, beat, kart.Identity(), 0)
		}
	}
	m.ctx.Scene.Draw(screen, m.proj)
}

func (m *Module) Whiff(beat float64) { m.WhiffAction(beat, -1) }

func (m *Module) WhiffAction(beat float64, action int) {
	dir, ok := dirForAction(action)
	if !ok {
		return
	}
	if m.isForkPop[dir] {
		m.outFork(dir, beat)
	}
}

func (m *Module) moveVirus(beat float64, dir, typ int) {
	v := m.spawnVirus(beat, -1, typ)
	m.ctx.At(beat, func() {
		v.summon(beat)
		v.position++
	})
	switch dir {
	case dirRight:
		v.startBeat = beat + 2
		m.ctx.At(beat+2, func() { v.appear() })
	default:
		pattern := dashPatterns[dir-1]
		for i := 0; i < dir; i++ {
			bb := beat + pattern[i]
			m.ctx.At(bb, func() {
				v.dash(bb)
				v.position++
			})
		}
		v.startBeat = beat + 4
		m.ctx.At(beat+4, func() { v.appear() })
	}
}

func (m *Module) virusDashManual(beat float64, dir, typ int, dashBeats []float64) {
	v := m.spawnVirus(beat, 0, typ)
	for i := 0; i < dir; i++ {
		bb := beat + dashBeats[i]
		m.ctx.At(bb, func() {
			v.dash(bb)
			v.position++
		})
	}
	m.ctx.At(beat+2, func() { v.dead = true })
}

func (m *Module) virusSummonManual(beat float64, typ int) {
	v := m.spawnVirus(beat, -1, typ)
	m.ctx.At(beat, func() {
		v.summon(beat)
		v.position++
	})
	m.ctx.At(beat+2, func() { v.dead = true })
}

func (m *Module) spawnVirus(beat float64, dir, typ int) *virus {
	v := &virus{
		mod: m, inst: m.virusT.NewInstance(),
		startBeat: beat, position: dir, life: typ,
	}
	v.applyPalette()
	m.viruses = append(m.viruses, v)
	return v
}

func (m *Module) outFork(dir int, beat float64) {
	m.ctx.Scene.PlayState(keyAnim, "push", beat, 0.5)
	m.ctx.Scene.PlayState(forkPath(dir), "out", beat, 0.5)
	m.ctx.Sound("fork" + strconv.Itoa(rand.Intn(3)))
	m.ctx.At(beat+refillBeat, func() { m.repopFork(dir, beat+refillBeat) })
	m.isForkPop[dir] = false
}

func (m *Module) repopFork(dir int, beat float64) {
	m.ctx.Scene.PlayState(forkPath(dir), "repop", beat, 0.5)
	m.isForkPop[dir] = true
}

func (m *Module) bopOnce(beat float64) {
	m.ctx.Scene.PlayState(radioAnim, "bop", beat, 0.5)
	if beat < m.docShockBeat || beat > m.docShockBeat+2 {
		m.ctx.Scene.PlayState(doctorAnim, "bop", beat, 0.5)
	}
	if m.orgAlive {
		m.ctx.Scene.PlayState(orgAnim, "bop", beat, 0.5)
	}
}

func (m *Module) autoBopAt(beat float64) bool {
	on := false
	for _, ev := range m.bops {
		if ev.beat > beat {
			break
		}
		on = ev.auto
	}
	return on
}

func (m *Module) applyBGEvent(ev bgEvt) {
	m.bgColor = colorEase{beat: ev.beat, length: ev.length, start: ev.bg0, end: ev.bg1, ease: ev.ease}
	m.pipeColor = colorEase{beat: ev.beat, length: ev.length, start: ev.pipe0, end: ev.pipe1, ease: ev.ease}
	m.outlineColor = colorEase{beat: ev.beat, length: ev.length, start: ev.outline0, end: ev.outline1, ease: ev.ease}
	m.cornerColor = colorEase{beat: ev.beat, length: ev.length, start: ev.corner0, end: ev.corner1, ease: ev.ease}
}

func (m *Module) persistBGColor(beat float64) {
	m.applyBGEvent(bgEvt{
		beat: beat, bg0: defaultBG, bg1: defaultBG,
		pipe0: defaultPipe, pipe1: defaultPipe,
		outline0: defaultOutline, outline1: defaultOutline,
		corner0: defaultCorner, corner1: defaultCorner,
	})
	for _, ev := range m.bgEvts {
		if ev.beat >= beat {
			break
		}
		m.applyBGEvent(ev)
	}
}

func (m *Module) applyBackgroundColors(beat float64) {
	sc := m.ctx.Scene
	pipe, outline, corner := m.pipeColor.at(beat), m.outlineColor.at(beat), m.cornerColor.at(beat)
	for _, p := range []string{pipeRight, pipeUp, pipeLeft, pipeDown, pipeEnd} {
		sc.SetColorOver(p, pipe)
	}
	for _, p := range []string{outlineRight, outlineUp, outlineLeft, outlineDown, outlineEnd} {
		sc.SetColorOver(p, outline)
	}
	for _, p := range []string{cornerTR, cornerTL, cornerBL, cornerBR} {
		sc.SetColorOver(p, corner)
	}
}

func (m *Module) persistVirusColor(beat float64) {
	m.updateVirusColors(defaultVirusColors)
	for _, ev := range m.colorEvts {
		if ev.beat >= beat {
			break
		}
		m.updateVirusColors(ev.colors)
	}
}

func (m *Module) updateVirusColors(colors [4][4]float64) {
	m.virusColors = colors
	for _, v := range m.viruses {
		v.applyPalette()
	}
}

func (m *Module) applyDoctorTransform(beat float64) {
	pos := m.doctorBasePos
	scale := m.doctorBaseScale
	rot := 0.0
	sticky := false
	for _, ev := range m.moves {
		if ev.beat > beat {
			break
		}
		u := normBeat(ev.beat, ev.length, beat)
		if ev.doMove {
			pos[0] = m.doctorBasePos[0] + engine.Ease(ev.ease, ev.startMove[0], ev.endMove[0], u)
			pos[1] = m.doctorBasePos[1] + engine.Ease(ev.ease, ev.startMove[1], ev.endMove[1], u)
		}
		if ev.doRotate {
			rot = engine.Ease(ev.ease, ev.startRot, ev.endRot, u) * math.Pi / 180
		}
		if ev.doScale {
			scale[0] = m.doctorBaseScale[0] * engine.Ease(ev.ease, ev.startScale[0], ev.endScale[0], u)
			scale[1] = m.doctorBaseScale[1] * engine.Ease(ev.ease, ev.startScale[1], ev.endScale[1], u)
		}
		sticky = ev.sticky
	}
	if sticky {
		cam := m.ctx.CameraAt(beat)
		pos[0] += cam[0]
		pos[1] += cam[1]
	}
	m.ctx.Scene.SetPosOver(doctorRoot, pos[0], pos[1])
	m.ctx.Scene.SetSpinOver(doctorRoot, rot)
	m.ctx.Scene.SetScaleOver(doctorRoot, scale[0], scale[1])
}

func (m *Module) keepLiveViruses(beat float64) {
	keep := m.viruses[:0]
	for _, v := range m.viruses {
		if v.dead || (v.life < 0 && beat >= v.startBeat+3) {
			continue
		}
		keep = append(keep, v)
	}
	m.viruses = keep
}

func (v *virus) appear() {
	if v.startBeat >= v.mod.gameEndBeat {
		v.dead = true
		return
	}
	pan := 0.0
	switch v.position {
	case dirRight:
		pan = 0.3
	case dirLeft:
		pan = -0.3
	}
	v.mod.ctx.SoundAtPitchPan(v.startBeat, "appear"+strconv.Itoa(rand.Intn(2)), 1, 1, pan)
	v.mod.ctx.At(v.startBeat, func() { v.virusAnim("appear", v.startBeat) })
	v.isJust = false
	targetBeat := v.startBeat + 1
	pos := v.position
	v.mod.ctx.ScheduleInputActionCond(targetBeat, actionForDir(pos),
		func() bool { return v.canJust() },
		func(state float64, _ engine.Judgment) { v.just(state) },
		func() { v.miss() },
	)
	v.mod.ctx.At(v.startBeat, func() { v.mod.isPrepare[pos] = true })
	v.mod.ctx.At(v.startBeat+1.5, func() { v.mod.isPrepare[pos] = false })
}

func (v *virus) dash(beat float64) {
	v.mod.ctx.Sound("dash")
	v.virusAnim("dash", beat)
}

func (v *virus) summon(beat float64) {
	v.virusAnim("summon", beat)
}

func (v *virus) move() {
	v.position++
	if v.position <= dirDown {
		v.startBeat += 2
		v.appear()
		return
	}
	v.kill()
}

func (v *virus) kill() {
	m := v.mod
	m.ctx.ScoreMiss()
	m.ctx.SoundAt(v.startBeat+2, "virusIn", 1)
	m.ctx.SoundAt(v.startBeat+4, "miss", 1)
	m.ctx.SoundAt(v.startBeat+5, "fadeout", 1)
	m.ctx.At(v.startBeat+2, func() {
		v.inst.PlayState("", "laugh", v.startBeat+2, 0.5)
		v.inst.PlayStateLayer("enter", "", "enter", v.startBeat+2, 0.5)
	})
	m.ctx.At(v.startBeat+4, func() {
		v.inst.PlayState("", "hide", v.startBeat+4, 0.5)
		m.ctx.Scene.PlayState(orgAnim, "damage", v.startBeat+4, 0.5)
		m.orgAlive = false
	})
	m.ctx.At(v.startBeat+5, func() { m.ctx.Scene.PlayState(orgAnim, "vanish", v.startBeat+5, 0.5) })
	m.ctx.At(v.startBeat+6, func() { v.inst.PlayState("", "laugh", v.startBeat+6, 0.5) })
	m.ctx.At(v.startBeat+7, func() {
		m.ctx.Scene.PlayState(orgAnim, "idleAdd", v.startBeat+7, 0.5)
		m.ctx.Scene.PlayState(orgAnim, "appear", v.startBeat+7, 0.5)
		m.orgAlive = true
		v.dead = true
	})
	if v.startBeat+6 >= m.docShockBeat+3 {
		m.docShockBeat = v.startBeat + 6
		m.ctx.At(v.startBeat+6, func() { m.ctx.Scene.PlayState(doctorAnim, "shock0", v.startBeat+6, 0.5) })
		m.ctx.At(v.startBeat+7, func() { m.ctx.Scene.PlayState(doctorAnim, "shock1", v.startBeat+7, 0.5) })
		m.ctx.At(v.startBeat+9, func() { m.ctx.Scene.PlayState(doctorAnim, "idle", v.startBeat+9, 0.5) })
	}
}

func (v *virus) just(state float64) {
	v.life--
	dir := v.position
	v.mod.ctx.At(v.startBeat+1+refillBeat, func() { v.mod.repopFork(dir, v.startBeat+1+refillBeat) })
	v.mod.isForkPop[dir] = false
	v.isJust = true
	if v.life < 0 {
		switch {
		case state >= 1:
			v.mod.ctx.Sound("bad")
			v.virusAnim("stabLate", v.mod.ctx.Beat())
			v.keyAnim("stabLate", v.mod.ctx.Beat())
		case state <= -1:
			v.mod.ctx.Sound("bad")
			v.virusAnim("stabFast", v.mod.ctx.Beat())
			v.keyAnim("stabFast", v.mod.ctx.Beat())
		default:
			v.mod.ctx.Sound("hit")
			v.virusAnim("stab", v.mod.ctx.Beat())
			v.keyAnim("stab", v.mod.ctx.Beat())
			v.mod.ctx.At(v.startBeat+2, func() {
				v.mod.ctx.Scene.PlayState(doctorAnim, "Vsign", v.startBeat+2, 0.5)
			})
		}
		return
	}
	v.mod.ctx.Sound("resist")
	v.virusAnim("resist", v.mod.ctx.Beat())
	v.keyAnim("resist", v.mod.ctx.Beat())
	v.applyPalette()
	v.move()
}

func (v *virus) miss() {
	dir := v.position
	if dir >= dirRight && dir <= dirDown {
		v.mod.isMiss[dir] = true
		v.mod.ctx.At(v.startBeat+1.5, func() { v.mod.isMiss[dir] = false })
	}
	v.dash(v.mod.ctx.Beat())
	v.move()
}

func (v *virus) canJust() bool {
	if v.position < dirRight || v.position > dirDown {
		return false
	}
	return v.mod.isForkPop[v.position] || v.isJust
}

func (v *virus) virusAnim(name string, beat float64) {
	v.inst.PlayState("", name, beat, 0.5)
	if v.position >= dirRight && v.position <= dirDown {
		v.inst.PlayStateLayer("dir:"+name, "", name+strconv.Itoa(v.position), beat, 0.5)
	}
}

func (v *virus) keyAnim(name string, beat float64) {
	v.mod.ctx.Scene.PlayState(keyAnim, "push", beat, 0.5)
	v.mod.ctx.Scene.PlayState(forkPath(v.position), name+strconv.Itoa(v.position), beat, 0.5)
}

func (v *virus) applyPalette() {
	idx := v.life
	if idx < 0 {
		idx = 0
	}
	if idx > 3 {
		idx = 3
	}
	p := virusPalette(v.mod.virusColors, idx)
	for _, rel := range []string{"sprite", "sprite/dash", "sprite/resist"} {
		v.inst.SetPalette(rel, p)
	}
}

func virusPalette(colors [4][4]float64, idx int) kart.Palette {
	fill := colors[idx]
	outline := colors[idx]
	if idx < 3 {
		fill = colors[idx+1]
	}
	return kart.Palette{
		Alpha:   [4]float64{0.75, 0, 0, 1},
		Fill:    fill,
		Outline: outline,
	}
}

func (ce colorEase) at(beat float64) [4]float64 {
	if ce.length <= 0 {
		return ce.end
	}
	u := normBeat(ce.beat, ce.length, beat)
	return [4]float64{
		engine.Ease(ce.ease, ce.start[0], ce.end[0], u),
		engine.Ease(ce.ease, ce.start[1], ce.end[1], u),
		engine.Ease(ce.ease, ce.start[2], ce.end[2], u),
		engine.Ease(ce.ease, ce.start[3], ce.end[3], u),
	}
}

func normBeat(beat, length, now float64) float64 {
	if length <= 0 {
		return 1
	}
	u := (now - beat) / length
	if u < 0 {
		return 0
	}
	if u > 1 {
		return 1
	}
	return u
}

func actionForDir(dir int) int {
	switch dir {
	case dirRight:
		return actionRight
	case dirUp:
		return actionUp
	case dirLeft:
		return actionLeft
	case dirDown:
		return actionDown
	default:
		return -99
	}
}

func dirForAction(action int) (int, bool) {
	switch action {
	case actionRight:
		return dirRight, true
	case actionUp:
		return dirUp, true
	case actionLeft:
		return dirLeft, true
	case actionDown:
		return dirDown, true
	default:
		return 0, false
	}
}

func forkPath(dir int) string {
	switch dir {
	case dirRight:
		return forkRight
	case dirUp:
		return forkUp
	case dirLeft:
		return forkLeft
	case dirDown:
		return forkDown
	default:
		return forkRight
	}
}

func clampDir(v int) int {
	if v < dirRight {
		return dirRight
	}
	if v > dirDown {
		return dirDown
	}
	return v
}

func clampDirMinOne(v int) int {
	if v < 1 {
		return 1
	}
	if v > dirDown {
		return dirDown
	}
	return v
}

func clampLife(v int) int {
	if v < 0 {
		return 0
	}
	if v > 3 {
		return 3
	}
	return v
}

func boolParam(e *riq.Entity, key string) bool { return boolParamDefault(e, key, false) }

func boolParamDefault(e *riq.Entity, key string, def bool) bool {
	if _, ok := e.Data[key]; !ok {
		return def
	}
	return e.Float(key, 0) != 0
}

func colorParam(e *riq.Entity, key string, def [4]float64) [4]float64 {
	v, ok := e.Data[key]
	if !ok {
		return def
	}
	switch c := v.(type) {
	case []any:
		out := def
		for i := 0; i < len(c) && i < 4; i++ {
			if f, ok := c[i].(float64); ok {
				out[i] = f
			}
		}
		return out
	case map[string]any:
		out := def
		for i, k := range []string{"r", "g", "b", "a"} {
			if f, ok := c[k].(float64); ok {
				out[i] = f
			}
		}
		return out
	}
	return def
}

func toRGBA(c [4]float64) color.RGBA {
	return color.RGBA{
		R: uint8(math.Max(0, math.Min(255, c[0]*255))),
		G: uint8(math.Max(0, math.Min(255, c[1]*255))),
		B: uint8(math.Max(0, math.Min(255, c[2]*255))),
		A: 255,
	}
}

const (
	doctorRoot = "StickyBG/Docholder"
	doctorAnim = "StickyBG/Docholder/doctor"
	radioAnim  = "StickyBG/Docholder/radio"
	keyAnim    = "key"
	orgAnim    = "organism"

	forkRight = "fork/fork_right"
	forkUp    = "fork/fork_up"
	forkLeft  = "fork/fork_left"
	forkDown  = "fork/fork_down"

	pipeRight = "pipe/PipeR"
	pipeUp    = "pipe/PipeU"
	pipeLeft  = "pipe/PipeL"
	pipeDown  = "pipe/PipeD"
	pipeEnd   = "pipe/PipeE"

	outlineRight = "pipe/PipeR/PipeOutlineR"
	outlineUp    = "pipe/PipeU/PipeOutlineU"
	outlineLeft  = "pipe/PipeL/PipeOutlineL"
	outlineDown  = "pipe/PipeD/PipeOutlineD"
	outlineEnd   = "pipe/PipeE/PipeOutlineE"

	cornerTR = "pipe/CornerTR"
	cornerTL = "pipe/CornerTL"
	cornerBL = "pipe/CornerBL"
	cornerBR = "pipe/CornerBR"
)
