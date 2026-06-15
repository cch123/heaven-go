// Package catchoftheday ports Catch of the Day's three fish cues, lake
// scene switching, Ann movement overrides, background layout/color handling,
// school-fish distraction, manta motion, and hand-written bubble particles.
package catchoftheday

import (
	"image/color"
	"math"
	"math/rand"
	"sort"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"hsdemo/engine"
	"hsdemo/kart"
	"hsdemo/kmdata"
	"hsdemo/riq"
)

const (
	fishQuick = iota + 1
	fishPause
	fishThree
)

const (
	layoutRandom = -1
	layoutA      = 0
	layoutB      = 1
	layoutC      = 2

	maxLakes       = 50
	schoolFishMax  = 250
	schoolFishSpin = 45 * math.Pi / 180
)

var white = [4]float64{1, 1, 1, 1}

type fishEvent struct {
	id     int
	kind   int
	beat   float64
	length float64

	layout         int
	useCustomColor bool
	topColor       [4]float64
	bottomColor    [4]float64
	sceneDelay     float64
	fgManta        bool
	bgManta        bool
	schoolFish     bool
	fishDensity    float64
	countIn        bool
	fakeOut        bool
	crossfade      bool
}

type colorEvent struct {
	beat     float64
	override bool
	top      [3][4]float64
	bottom   [3][4]float64
}

type moveEvent struct {
	beat, length float64
	doMove       bool
	startMove    [2]float64
	endMove      [2]float64
	doRotate     bool
	startRot     float64
	endRot       float64
	doScale      bool
	startScale   [2]float64
	endScale     [2]float64
	ease         int
	sticky       bool
}

type schoolFish struct {
	inst *kart.Instance
	y    float64
	rot  float64
}

type bubble struct {
	x, y  float64
	delay float64
	life  float64
	size  float64
}

type lake struct {
	ev       *fishEvent
	inst     *kart.Instance
	layout   int
	sort     int
	dead     bool
	fishOut  bool
	disposed bool

	rendererOffset [2]float64
	crossfadeStart float64
	school         []schoolFish
	bubbles        []bubble
}

type Module struct {
	ctx  *engine.Ctx
	proj kart.Aff

	lakeT       *kart.Template
	schoolFishT *kart.Template

	anglerRoot string
	anglerAnim string

	defaultTop    [3][4]float64
	defaultBottom [3][4]float64
	topColors     [3][4]float64
	bottomColors  [3][4]float64

	fishes []fishEvent
	colors []colorEvent
	moves  []moveEvent
	lakes  map[int]*lake

	lastLayout int
}

func New() engine.Module { return &Module{lastLayout: layoutRandom} }

func (m *Module) ID() string { return "catchOfTheDay" }

func (m *Module) Load(ctx *engine.Ctx) error {
	m.ctx = ctx
	if err := ctx.LoadAssets("catchOfTheDay"); err != nil {
		return err
	}
	m.proj = kart.Translate(engine.ScreenW/2, engine.ScreenH/2).Mul(kart.Scale(54, -54))
	game := ctx.Assets.Extra.Components["game"]
	m.anglerRoot = refOr(ctx, game, "AnglerTransform", "StickyCanvas/Angler")
	m.anglerAnim = refOr(ctx, game, "Angler", "StickyCanvas/Angler/Character")
	m.defaultTop = colorArray(game.Lists["_TopColors"], defaultTopColors())
	m.defaultBottom = colorArray(game.Lists["_BottomColors"], defaultBottomColors())
	m.topColors, m.bottomColors = m.defaultTop, m.defaultBottom
	m.lakeT = kart.NewTemplate(ctx.Assets, refOr(ctx, game, "LakeScenePrefab", "LakeScene"))
	m.schoolFishT = kart.NewTemplate(ctx.Assets, "SchoolFish")
	m.lakes = map[int]*lake{}

	// The main prefab contains a dummy lake for authoring. Unity renders only
	// instantiated LakeScene prefabs during play, so the authored dummy and
	// hidden template roots are disabled in the scene instance and drawn via
	// kart.Template instances instead.
	ctx.Scene.SetActive("Main/LakeScene", false)
	ctx.Scene.SetActive("LakeScene", false)
	ctx.Scene.SetActive("SchoolFish", false)
	ctx.Scene.SetActive("Bubble", false)
	ctx.Scene.PlayDefaultState(m.anglerAnim, 0, ctx.SecPerBeat(0))
	m.applyAngler(0)
	return nil
}

func (m *Module) OnEvent(e *riq.Entity) {
	switch e.Datamodel {
	case "catchOfTheDay/fish1":
		m.addFish(e, fishQuick)
	case "catchOfTheDay/fish2":
		m.addFish(e, fishPause)
	case "catchOfTheDay/fish3":
		m.addFish(e, fishThree)
	case "catchOfTheDay/color":
		ev := colorEvent{beat: e.Beat, override: boolParamDefault(e, "override", true)}
		ev.top[0] = colorParam(e, "topColorA", m.defaultTop[0])
		ev.bottom[0] = colorParam(e, "bottomColorA", m.defaultBottom[0])
		ev.top[1] = colorParam(e, "topColorB", m.defaultTop[1])
		ev.bottom[1] = colorParam(e, "bottomColorB", m.defaultBottom[1])
		ev.top[2] = colorParam(e, "topColorC", m.defaultTop[2])
		ev.bottom[2] = colorParam(e, "bottomColorC", m.defaultBottom[2])
		m.colors = append(m.colors, ev)
		m.ctx.At(e.Beat, func() { m.applyColorOverride(ev) })
	case "catchOfTheDay/moveAngler":
		mv := moveEvent{
			beat: e.Beat, length: e.Length,
			doMove:     boolParam(e, "doMove"),
			startMove:  [2]float64{e.Float("startMoveX", 0), e.Float("startMoveY", 0)},
			endMove:    [2]float64{e.Float("endMoveX", 0), e.Float("endMoveY", 0)},
			doRotate:   boolParam(e, "doRotate"),
			startRot:   e.Float("startRotDegrees", 0),
			endRot:     e.Float("endRotDegrees", 0),
			doScale:    boolParam(e, "doScale"),
			startScale: [2]float64{e.Float("startScaleX", 1), e.Float("startScaleY", 1)},
			endScale:   [2]float64{e.Float("endScaleX", 1), e.Float("endScaleY", 1)},
			ease:       intParam(e, "ease", 0),
			sticky:     boolParam(e, "sticky"),
		}
		m.moves = append(m.moves, mv)
	}
}

func (m *Module) Ready() {
	sort.Slice(m.fishes, func(i, j int) bool { return m.fishes[i].beat < m.fishes[j].beat })
	sort.Slice(m.colors, func(i, j int) bool { return m.colors[i].beat < m.colors[j].beat })
	sort.Slice(m.moves, func(i, j int) bool { return m.moves[i].beat < m.moves[j].beat })
}

func (m *Module) OnSwitch(beat float64) {
	m.lakes = map[int]*lake{}
	m.lastLayout = layoutRandom
	m.topColors, m.bottomColors = m.defaultTop, m.defaultBottom
	for _, c := range m.colors {
		if c.beat > beat {
			break
		}
		m.applyColorOverride(c)
	}
	m.ctx.Scene.PlayDefaultState(m.anglerAnim, beat, m.ctx.SecPerBeat(beat))
	m.applyAngler(beat)
	for i := range m.fishes {
		if m.fishActiveAt(&m.fishes[i], beat) {
			m.newLake(&m.fishes[i], beat)
		}
	}
	if len(m.lakes) == 0 {
		m.spawnNextFish(beat)
	}
}

func (m *Module) Whiff(beat float64) {
	for _, l := range m.liveLakes() {
		if !l.fishOut && beat < targetBeat(l.ev)+engine.WinNG*4 {
			m.through(l, beat)
			return
		}
	}
}

func (m *Module) Update(_ float64, beat float64) {
	m.applyAngler(beat)
	for _, l := range m.lakes {
		if l.dead {
			continue
		}
		m.updateLake(l, beat)
		if l.crossfadeStart > 0 && beat >= l.crossfadeStart+1 {
			l.dead = true
		}
	}
}

func (m *Module) Draw(screen *ebiten.Image, _ float64, beat float64) {
	screen.Fill(color.NRGBA{0xb5, 0xde, 0xde, 0xff})
	sc := m.ctx.Scene
	m.ctx.SampleScene(beat)

	for _, l := range m.liveLakes() {
		m.updateLake(l, beat)
		alpha := 1.0
		scale := 1.0
		if l.crossfadeStart > 0 {
			u := clamp01(beat - l.crossfadeStart)
			alpha = 1 - u
			scale = 1 + u*0.875
		}
		l.inst.Scale = [2]float64{scale, scale}
		m.tintLake(l, alpha)
		l.inst.Queue(sc, beat, kart.Identity(), float64(l.sort))
		m.queueSchoolFish(sc, l, beat, alpha)
	}
	sc.Draw(screen, m.proj)
	m.drawBubbles(screen, beat)
}

func (m *Module) addFish(e *riq.Entity, kind int) {
	ev := fishEvent{
		id: kind*100000 + len(m.fishes), kind: kind,
		beat: e.Beat, length: defaultFishLength(kind),
		layout:      intParam(e, "layout", layoutRandom),
		sceneDelay:  e.Float("sceneDelay", 2),
		fgManta:     boolParam(e, "fgManta"),
		bgManta:     boolParam(e, "bgManta"),
		schoolFish:  boolParam(e, "schoolFish"),
		fishDensity: e.Float("fishDensity", 1),
		countIn:     boolParam(e, "countIn"),
		fakeOut:     boolParam(e, "fakeOut"),
		crossfade:   boolParamDefault(e, "crossfade", true),
	}
	if e.Length > 0 {
		ev.length = e.Length
	}
	ev.useCustomColor = boolParam(e, "useCustomColor")
	ev.topColor = colorParam(e, "colorTop", m.defaultTop[0])
	ev.bottomColor = colorParam(e, "colorBottom", m.defaultBottom[0])
	m.fishes = append(m.fishes, ev)
	m.scheduleFish(&m.fishes[len(m.fishes)-1])
}

func (m *Module) scheduleFish(ev *fishEvent) {
	m.scheduleCueSounds(ev)
	m.ctx.At(ev.beat, func() {
		if m.ctx.GameAt(ev.beat) == m.ID() {
			m.newLake(ev, ev.beat)
		}
	})
	for _, p := range pickSchedule(ev) {
		p := p
		m.ctx.At(p.beat, func() {
			if l := m.lakes[ev.id]; l != nil {
				m.pick(l, p.beat, p.down)
			}
		})
	}
	bite := targetBeat(ev) - 0.1
	m.ctx.At(bite, func() {
		if l := m.lakes[ev.id]; l != nil && !l.fishOut {
			l.inst.PlayState("Renderer/FishAnimator", fishState(ev.kind, "Bite", false), bite, 0.5)
		}
	})
	dispose := targetBeat(ev) + ev.sceneDelay
	m.ctx.At(dispose, func() {
		if l := m.lakes[ev.id]; l != nil {
			m.disposeLake(l, dispose)
		}
	})
	m.ctx.ScheduleInputCond(targetBeat(ev),
		func() bool { return m.ctx.GameAt(targetBeat(ev)) == m.ID() },
		func(state float64, _ engine.Judgment) {
			if l := m.lakes[ev.id]; l != nil {
				if state > -1 && state < 1 {
					m.just(l, targetBeat(ev))
				} else {
					m.nearMiss(l, targetBeat(ev))
				}
			}
		},
		func() {
			if l := m.lakes[ev.id]; l != nil {
				m.out(l, targetBeat(ev))
			}
		})
}

func (m *Module) scheduleCueSounds(ev *fishEvent) {
	switch ev.kind {
	case fishQuick:
		m.ctx.SoundAt(ev.beat, "quick1", 1)
		m.ctx.SoundAt(ev.beat+1, "quick2", 1)
	case fishPause:
		m.ctx.SoundAt(ev.beat, "pausegill1", 1)
		m.ctx.SoundAt(ev.beat+0.5, "pausegill2", 1)
		m.ctx.SoundAt(ev.beat+1, "pausegill3", 1)
		if ev.countIn {
			m.ctx.SoundAt(ev.beat+2, "common_count-ins_and", 1)
			m.ctx.SoundAt(ev.beat+3, goCountSound(ev), 1)
		}
	case fishThree:
		m.ctx.SoundAt(ev.beat, "threefish1", 1)
		m.ctx.SoundAt(ev.beat+0.25, "threefish2", 1)
		m.ctx.SoundAt(ev.beat+0.5, "threefish3", 1)
		m.ctx.SoundAt(ev.beat+1, "threefish4", 1)
		if ev.countIn {
			m.ctx.SoundAt(ev.beat+2, "common_count-ins_one1", 1)
			m.ctx.SoundAt(ev.beat+3, "common_count-ins_two1", 1)
			m.ctx.SoundAt(ev.beat+4, "common_count-ins_three1", 1)
			m.ctx.SoundAt(ev.beat+4.5, goCountSound(ev), 1)
		}
	}
}

func (m *Module) newLake(ev *fishEvent, now float64) *lake {
	if existing := m.lakes[ev.id]; existing != nil {
		return existing
	}
	if len(m.lakes) >= maxLakes || m.lakeT == nil {
		return nil
	}
	sortIdx := m.fishIndex(ev)
	l := &lake{
		ev: ev, inst: m.lakeT.NewInstance(), sort: -sortIdx,
		rendererOffset: randOffset(ev),
	}
	l.layout = m.resolveLayout(ev)
	m.lastLayout = l.layout
	m.setupLake(l, now)
	m.lakes[ev.id] = l
	return l
}

func (m *Module) setupLake(l *lake, now float64) {
	ev := l.ev
	sec := m.ctx.SecPerBeat(ev.beat)
	l.inst.SetPos("Renderer", l.rendererOffset[0], l.rendererOffset[1])
	l.inst.SetGroupOrder(l.sort)
	l.inst.PlayDefaultState("Renderer/BigManta", ev.beat, sec)
	l.inst.PlayDefaultState("Renderer/SmallManta", ev.beat, sec)
	for i := 1; i <= 8; i++ {
		l.inst.PlayDefaultState("Renderer/Background/Fish/BGFish"+itoa(i), ev.beat, sec)
	}
	if ev.kind == fishThree && ev.fakeOut {
		l.inst.PlayState("Renderer/FishAnimator", "Fish1_Wait", ev.beat, 0.5)
		l.inst.PlayState("Renderer/FishAnimator", "Fish3_WaitB", ev.beat-4, 0.5)
	} else {
		l.inst.PlayState("Renderer/FishAnimator", fishState(ev.kind, "Wait", false), ev.beat, 0.5)
	}
	l.inst.PlayState("Renderer/Background", layoutState(l.layout), ev.beat, 0.5)
	top, bottom := m.colorsFor(ev, l.layout)
	m.applyLakeColors(l, top, bottom)
	l.inst.SetActive("Renderer/BigManta", ev.fgManta)
	l.inst.SetActive("Renderer/SmallManta", ev.bgManta)
	l.inst.SetActive("Renderer/FishSchool", ev.schoolFish && ev.fishDensity > 0)
	if ev.schoolFish && ev.fishDensity > 0 {
		l.school = m.makeSchoolFish(ev)
	}
	l.bubbles = m.makeBubbles(ev)
	m.syncLakeState(l, now)
}

func (m *Module) syncLakeState(l *lake, beat float64) {
	ev := l.ev
	if beat >= targetBeat(ev)-0.1 {
		l.inst.PlayState("Renderer/FishAnimator", fishState(ev.kind, "Bite", false), targetBeat(ev)-0.1, 0.5)
		return
	}
	lastPick := math.Inf(-1)
	down := false
	for _, p := range pickSchedule(ev) {
		if p.beat <= beat && p.beat >= lastPick {
			lastPick, down = p.beat, p.down
		}
	}
	if !math.IsInf(lastPick, -1) {
		l.inst.PlayState("Renderer/FishAnimator", fishState(ev.kind, "Pick", down), lastPick, 0.5)
		m.ctx.Scene.PlayState(m.anglerAnim, "Pick", lastPick, 0.5)
	}
}

func (m *Module) updateLake(l *lake, beat float64) {
	ev := l.ev
	if ev.fgManta {
		l.inst.SetPos("Renderer/BigManta", ev.beat-beat+4.5, 0)
	}
	if ev.bgManta {
		l.inst.SetPos("Renderer/SmallManta", 1.25+(beat-ev.beat)*0.13, 0)
	}
	if ev.schoolFish {
		l.inst.SetRot("Renderer/FishSchool", beat*schoolFishSpin)
	}
}

func (m *Module) disposeLake(l *lake, beat float64) {
	if l.disposed {
		return
	}
	l.disposed = true
	delete(m.lakes, l.ev.id)
	if len(m.lakes) == 0 {
		m.spawnNextFish(beat)
	}
	if l.ev.crossfade {
		l.crossfadeStart = beat
		m.lakes[l.ev.id] = l
	} else {
		l.dead = true
	}
}

func (m *Module) spawnNextFish(beat float64) bool {
	nextSwitch := m.ctx.NextSwitchBeat(beat)
	for i := range m.fishes {
		f := &m.fishes[i]
		if f.beat < beat || f.beat >= nextSwitch {
			continue
		}
		for _, c := range m.colors {
			if c.beat >= beat && c.beat <= f.beat {
				m.applyColorOverride(c)
			}
		}
		m.newLake(f, beat)
		return true
	}
	return false
}

func (m *Module) just(l *lake, beat float64) {
	if l.fishOut {
		return
	}
	l.fishOut = true
	l.inst.PlayState("Renderer/FishAnimator", fishState(l.ev.kind, "Just", false), beat, 0.5)
	m.ctx.Scene.PlayState(m.anglerAnim, "Just", beat, 0.5)
	switch l.ev.kind {
	case fishQuick:
		m.ctx.Sound("quick3")
	case fishPause:
		m.ctx.Sound("pausegill4")
	case fishThree:
		m.ctx.Sound("threefish5")
	}
}

func (m *Module) nearMiss(l *lake, beat float64) {
	if l.fishOut {
		return
	}
	l.fishOut = true
	l.inst.PlayState("Renderer/FishAnimator", fishState(l.ev.kind, "Miss", false), beat, 0.5)
	m.ctx.Scene.PlayState(m.anglerAnim, "Miss", beat, 0.5)
	m.ctx.Sound("nearMiss")
}

func (m *Module) through(l *lake, beat float64) {
	if l.fishOut {
		return
	}
	l.inst.PlayState("Renderer/FishAnimator", fishState(l.ev.kind, "Through", false), beat, 0.5)
	m.ctx.Scene.PlayState(m.anglerAnim, "Through", beat, 0.5)
	switch l.ev.kind {
	case fishQuick:
		m.ctx.Sound("quick_laugh")
	case fishPause:
		m.ctx.Sound("pausegill_laugh")
	case fishThree:
		m.ctx.Sound("threefish_laugh")
	}
}

func (m *Module) out(l *lake, beat float64) {
	if l.fishOut {
		return
	}
	l.fishOut = true
	l.inst.PlayState("Renderer/FishAnimator", fishState(l.ev.kind, "Out", false), beat, 0.5)
	m.ctx.Scene.PlayState(m.anglerAnim, "Through", beat, 0.5)
	m.maybeFleeBGFish(l, beat)
	m.ctx.Sound("common_miss")
}

func (m *Module) pick(l *lake, beat float64, down bool) {
	if l.fishOut {
		return
	}
	l.inst.PlayState("Renderer/FishAnimator", fishState(l.ev.kind, "Pick", down), beat, 0.5)
	m.ctx.Scene.PlayState(m.anglerAnim, "Pick", beat, 0.5)
}

func (m *Module) applyColorOverride(ev colorEvent) {
	if ev.override {
		m.topColors, m.bottomColors = ev.top, ev.bottom
	} else {
		m.topColors, m.bottomColors = m.defaultTop, m.defaultBottom
	}
}

func (m *Module) applyAngler(beat float64) {
	basePos, baseScale := m.nodeBase(m.anglerRoot)
	posOff := [2]float64{}
	scaleMul := [2]float64{1, 1}
	rot := 0.0
	sticky := false
	for _, mv := range m.moves {
		if mv.beat > beat {
			break
		}
		u := 1.0
		if mv.length > 0 && beat < mv.beat+mv.length {
			u = clamp01((beat - mv.beat) / mv.length)
		}
		e := engine.Ease(mv.ease, 0, 1, u)
		if mv.doMove {
			posOff[0] = lerp(mv.startMove[0], mv.endMove[0], e)
			posOff[1] = lerp(mv.startMove[1], mv.endMove[1], e)
		}
		if mv.doRotate {
			rot = lerp(mv.startRot, mv.endRot, e) * math.Pi / 180
		}
		if mv.doScale {
			scaleMul[0] = lerp(mv.startScale[0], mv.endScale[0], e)
			scaleMul[1] = lerp(mv.startScale[1], mv.endScale[1], e)
		}
		sticky = mv.sticky
	}
	if sticky {
		cam := m.ctx.CameraAt(beat)
		posOff[0] += cam[0]
		posOff[1] += cam[1]
	}
	m.ctx.Scene.SetPosOver(m.anglerRoot, basePos[0]+posOff[0], basePos[1]+posOff[1])
	m.ctx.Scene.SetSpinOver(m.anglerRoot, rot)
	m.ctx.Scene.SetScaleOver(m.anglerRoot, baseScale[0]*scaleMul[0], baseScale[1]*scaleMul[1])
}

func (m *Module) nodeBase(path string) ([2]float64, [2]float64) {
	if idx, ok := m.ctx.Assets.NodeIndex(path); ok {
		n := m.ctx.Assets.Rig.Nodes[idx]
		return n.Pos, n.Scale
	}
	return [2]float64{}, [2]float64{1, 1}
}

func (m *Module) resolveLayout(ev *fishEvent) int {
	layout := ev.layout
	if layout == layoutRandom {
		if m.lastLayout == layoutRandom {
			return layoutA
		}
		options := []int{layoutA, layoutB, layoutC}
		var filtered []int
		for _, v := range options {
			if v != m.lastLayout {
				filtered = append(filtered, v)
			}
		}
		rng := rand.New(rand.NewSource(int64(ev.id)*97 + int64(ev.beat*1000)))
		return filtered[rng.Intn(len(filtered))]
	}
	if layout < layoutA || layout > layoutC {
		return layoutA
	}
	return layout
}

func (m *Module) colorsFor(ev *fishEvent, layout int) ([4]float64, [4]float64) {
	if ev.useCustomColor {
		return ev.topColor, ev.bottomColor
	}
	return m.topColors[layout], m.bottomColors[layout]
}

func (m *Module) applyLakeColors(l *lake, top, bottom [4]float64) {
	for _, rel := range []string{
		"Renderer/Background/Backdrop/GradientBG",
		"Renderer/Background/Backdrop/ColorBG_Top",
	} {
		l.inst.SetColor(rel, top)
	}
	l.inst.SetColor("Renderer/Background/Backdrop/ColorBG_Bottom", bottom)
	for i := 1; i <= 8; i++ {
		l.inst.SetColor("Renderer/Background/Fish/BGFish"+itoa(i)+"/Sprite", bottom)
	}
}

func (m *Module) makeSchoolFish(ev *fishEvent) []schoolFish {
	if m.schoolFishT == nil || ev.fishDensity <= 0 {
		return nil
	}
	count := int(math.Ceil(clamp01(ev.fishDensity) * schoolFishMax))
	rng := rand.New(rand.NewSource(int64(ev.id)*131 + 7))
	out := make([]schoolFish, 0, count)
	for i := 0; i < count; i++ {
		rot := (rng.Float64()*360 - 180) * math.Pi / 180
		sf := schoolFish{inst: m.schoolFishT.NewInstance(), y: 3 + rng.Float64()*8, rot: rot}
		sf.inst.SetOrder("Body", (i+1)*2)
		sf.inst.SetOrder("Fin", (i+1)*2+1)
		sf.inst.SetOrder("Eye", (i+1)*2+1)
		out = append(out, sf)
	}
	return out
}

func (m *Module) makeBubbles(ev *fishEvent) []bubble {
	anchors := m.bubbleAnchors()
	if len(anchors) == 0 {
		return nil
	}
	rng := rand.New(rand.NewSource(int64(ev.id)*173 + 13))
	rng.Shuffle(len(anchors), func(i, j int) { anchors[i], anchors[j] = anchors[j], anchors[i] })
	count := rng.Intn(4)
	out := make([]bubble, 0, count)
	for i := 0; i < count; i++ {
		out = append(out, bubble{
			x: anchors[i][0], y: anchors[i][1],
			delay: rng.Float64() * 0.35,
			life:  1.1 + rng.Float64()*0.45,
			size:  0.05 + rng.Float64()*0.04,
		})
	}
	return out
}

func (m *Module) bubbleAnchors() [][2]float64 {
	var out [][2]float64
	prefix := "LakeScene/Renderer/Background/Particles/Bubble"
	for i := 1; i <= 8; i++ {
		if idx, ok := m.ctx.Assets.NodeIndex(prefix + itoa(i)); ok {
			out = append(out, m.ctx.Assets.Rig.Nodes[idx].Pos)
		}
	}
	return out
}

func (m *Module) queueSchoolFish(sc *kart.SceneInst, l *lake, beat, alpha float64) {
	if len(l.school) == 0 || alpha <= 0 {
		return
	}
	base := kart.Translate(l.rendererOffset[0], l.rendererOffset[1]).Mul(kart.Rotate(beat * schoolFishSpin))
	for _, sf := range l.school {
		if alpha < 0.999 {
			sf.inst.SetColor("Body", [4]float64{1, 1, 1, alpha})
			sf.inst.SetColor("Fin", [4]float64{1, 1, 1, alpha})
			sf.inst.SetColor("Eye", [4]float64{1, 1, 1, alpha})
		}
		sf.inst.Offset = [2]float64{0, sf.y}
		sf.inst.Rot = -sf.rot - math.Pi
		sf.inst.Queue(sc, beat, base.Mul(kart.Rotate(sf.rot)), float64(l.sort))
	}
}

func (m *Module) drawBubbles(screen *ebiten.Image, beat float64) {
	for _, l := range m.liveLakes() {
		if len(l.bubbles) == 0 {
			continue
		}
		base := kart.Translate(l.rendererOffset[0], l.rendererOffset[1])
		alphaMul := 1.0
		if l.crossfadeStart > 0 {
			alphaMul = 1 - clamp01(beat-l.crossfadeStart)
		}
		for _, b := range l.bubbles {
			u := (beat - l.ev.beat - b.delay) / b.life
			if u < 0 || u > 1 {
				continue
			}
			x, y := base.Apply(b.x+math.Sin(u*math.Pi*2)*0.04, b.y+u*0.75)
			sx, sy := m.proj.Apply(x, y)
			a := uint8(clamp01((1-u)*alphaMul) * 150)
			vector.DrawFilledCircle(screen, float32(sx), float32(sy), float32(b.size*54*(1+u*0.8)), color.NRGBA{R: 255, G: 255, B: 255, A: a}, true)
		}
	}
}

func (m *Module) tintLake(l *lake, alpha float64) {
	if alpha >= 0.999 {
		return
	}
	for _, n := range l.inst.T.Nodes {
		rn := m.ctx.Assets.Rig.Nodes[n.RigIdx]
		if rn.Sprite != "" {
			l.inst.SetColor(n.RelPath, [4]float64{1, 1, 1, alpha})
		}
	}
	top, bottom := m.colorsFor(l.ev, l.layout)
	top[3] *= alpha
	bottom[3] *= alpha
	m.applyLakeColors(l, top, bottom)
}

func (m *Module) maybeFleeBGFish(l *lake, beat float64) {
	rng := rand.New(rand.NewSource(int64(l.ev.id)*211 + 29))
	if rng.Float64() <= 0.75 {
		return
	}
	for i := 1; i <= 8; i++ {
		state := bgFishFleeState(m.bgFishFleeAnim(i), m.bgFishFlip(i))
		l.inst.PlayState("Renderer/Background/Fish/BGFish"+itoa(i), state, beat, 0.5)
	}
}

func (m *Module) bgFishFleeAnim(idx int) int {
	path := "LakeScene/Renderer/Background/Fish/BGFish" + itoa(idx)
	for _, c := range m.ctx.Assets.Extra.Components {
		if c.Path == path {
			return int(c.Nums["FleeAnim"])
		}
	}
	return 0
}

func (m *Module) bgFishFlip(idx int) bool {
	path := "LakeScene/Renderer/Background/Fish/BGFish" + itoa(idx)
	for _, c := range m.ctx.Assets.Extra.Components {
		if c.Path == path {
			return c.Nums["FlipSprite"] != 0
		}
	}
	return false
}

func (m *Module) fishActiveAt(ev *fishEvent, beat float64) bool {
	return ev.beat <= beat && beat <= ev.beat+ev.length-1+ev.sceneDelay
}

func (m *Module) fishIndex(ev *fishEvent) int {
	for i := range m.fishes {
		if m.fishes[i].id == ev.id {
			return i
		}
	}
	return 0
}

func (m *Module) liveLakes() []*lake {
	out := make([]*lake, 0, len(m.lakes))
	for _, l := range m.lakes {
		if l != nil && !l.dead {
			out = append(out, l)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].sort < out[j].sort })
	return out
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

func colorArray(items []kmdata.ComponentItem, fallback [3][4]float64) [3][4]float64 {
	out := fallback
	for i := 0; i < len(items) && i < 3; i++ {
		out[i] = itemColor(items[i], fallback[i])
	}
	return out
}

func itemColor(it kmdata.ComponentItem, fallback [4]float64) [4]float64 {
	if it.Nums == nil {
		return fallback
	}
	return [4]float64{
		numMap(it.Nums, "r", fallback[0]),
		numMap(it.Nums, "g", fallback[1]),
		numMap(it.Nums, "b", fallback[2]),
		numMap(it.Nums, "a", fallback[3]),
	}
}

func defaultTopColors() [3][4]float64 {
	return [3][4]float64{
		{0.7098039, 0.8705882, 0.8705882, 1},
		{0.7098039, 0.8745099, 0.6784314, 1},
		{0.8705883, 0.8705883, 0.6784314, 1},
	}
}

func defaultBottomColors() [3][4]float64 {
	return [3][4]float64{
		{0.4666667, 0.7372549, 0.8196079, 1},
		{0.3529412, 0.7137255, 0.482353, 1},
		{0.7098039, 0.627451, 0.4196079, 1},
	}
}

func defaultFishLength(kind int) float64 {
	switch kind {
	case fishPause:
		return 4
	case fishThree:
		return 5.5
	default:
		return 3
	}
}

func targetBeat(ev *fishEvent) float64 {
	switch ev.kind {
	case fishPause:
		return ev.beat + 3
	case fishThree:
		return ev.beat + 4.5
	default:
		return ev.beat + 2
	}
}

type pickCue struct {
	beat float64
	down bool
}

func pickSchedule(ev *fishEvent) []pickCue {
	switch ev.kind {
	case fishPause:
		return []pickCue{{ev.beat, false}, {ev.beat + 0.5, false}, {ev.beat + 1, false}}
	case fishThree:
		return []pickCue{{ev.beat, false}, {ev.beat + 0.25, true}, {ev.beat + 0.5, false}, {ev.beat + 1, true}}
	default:
		return []pickCue{{ev.beat, false}, {ev.beat + 1, false}}
	}
}

func fishState(kind int, suffix string, down bool) string {
	switch kind {
	case fishPause:
		return "Fish2_" + suffix
	case fishThree:
		if suffix == "Pick" {
			if down {
				return "Fish3_PickDown"
			}
			return "Fish3_PickUp"
		}
		return "Fish3_" + suffix
	default:
		return "Fish1_" + suffix
	}
}

func layoutState(layout int) string {
	switch layout {
	case layoutB:
		return "LayoutB"
	case layoutC:
		return "LayoutC"
	default:
		return "LayoutA"
	}
}

func goCountSound(ev *fishEvent) string {
	rng := rand.New(rand.NewSource(int64(ev.id)*257 + int64(ev.beat*1000)))
	if rng.Float64() > 0.5 {
		return "common_count-ins_go1"
	}
	return "common_count-ins_go2"
}

func randOffset(ev *fishEvent) [2]float64 {
	rng := rand.New(rand.NewSource(int64(ev.id)*307 + 19))
	return [2]float64{rng.Float64() - 0.5, rng.Float64()*0.6 - 0.3}
}

func bgFishFleeState(anim int, flip bool) string {
	switch anim {
	case 1:
		return chooseFlip(flip, "BGFishOut_WSW", "BGFishOut_ESE")
	case 2:
		return chooseFlip(flip, "BGFishOut_SW", "BGFishOut_SE")
	case 3:
		return chooseFlip(flip, "BGFishOut_WNW", "BGFishOut_ENE")
	case 4:
		return chooseFlip(flip, "BGFishOut_NW", "BGFishOut_NE")
	case 8:
		return chooseFlip(flip, "BGFishOut_E", "BGFishOut_W")
	case 9:
		return chooseFlip(flip, "BGFishOut_ESE", "BGFishOut_WSW")
	case 10:
		return chooseFlip(flip, "BGFishOut_SE", "BGFishOut_SW")
	case 11:
		return chooseFlip(flip, "BGFishOut_ENE", "BGFishOut_WNW")
	case 12:
		return chooseFlip(flip, "BGFishOut_NE", "BGFishOut_NW")
	default:
		return chooseFlip(flip, "BGFishOut_W", "BGFishOut_E")
	}
}

func chooseFlip(flip bool, a, b string) string {
	if flip {
		return a
	}
	return b
}

func boolParam(e *riq.Entity, key string) bool { return boolParamDefault(e, key, false) }

func boolParamDefault(e *riq.Entity, key string, fallback bool) bool {
	if v, ok := e.Data[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
		return e.Float(key, 0) != 0
	}
	return fallback
}

func intParam(e *riq.Entity, key string, fallback int) int {
	return int(e.Float(key, float64(fallback)))
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

func numAny(v any, def float64) float64 {
	if f, ok := v.(float64); ok {
		return f
	}
	return def
}

func numMap(m map[string]float64, key string, def float64) float64 {
	if v, ok := m[key]; ok {
		return v
	}
	return def
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

func lerp(a, b, u float64) float64 { return a + (b-a)*u }

func itoa(i int) string {
	if i < 10 {
		return string(rune('0' + i))
	}
	var b [8]byte
	n := len(b)
	for i > 0 {
		n--
		b[n] = byte('0' + i%10)
		i /= 10
	}
	return string(b[n:])
}

func relPath(path, root string) string {
	if path == root {
		return ""
	}
	prefix := root + "/"
	return strings.TrimPrefix(path, prefix)
}
