// Package airrally ports the Air Rally runtime: rally flow, count-ins,
// distance/enter camera depth, day-night tinting, weather/tree emitters,
// bird/rainbow prefabs, and island motion.
package airrally

import (
	"math"
	"sort"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"hsdemo/engine"
	"hsdemo/kart"
	"hsdemo/kmdata"
	"hsdemo/riq"
)

const (
	distClose = iota
	distFar
	distFarther
	distFarthest
)

var distZ = []float64{3.55, 35.16, 105.16, 255.16}
var countInOffsets = []float64{0.142, 0.140, 0.150, 0.160}
var nyaOffsets = []float64{-0.01, -0.01, 0.003, -0.01}
var whooshRallyOffsets = []float64{0, 0.210, 0.210, 0.170}
var whooshBaBumOffsets = []float64{0, 0.380, 0.380, 0.380}
var baBumOffsets = [4][4]float64{
	{0.009, 0.017, 0.014, 0.010},
	{0.003, 0.020, 0.004, 0.010},
	{0.008, 0.080, 0.075, 0.028},
	{0.012, 0.040, 0.026, 0.040},
}
var baBumFarAltOffsets = []float64{0.001, 0.012, 0.012, 0.012}

const (
	wayPointHomeZ      = 3.16
	wayPointBeatLength = 1.0
	wayPointEnter      = -10.0

	// AirRally prefab: IslandsManager.loopMult=0.35, speedMult=0.0125.
	// IslandsManager.Start divides speedMult by loopMult before RvlIsland.Update.
	islandLoopMult  = 0.35
	islandSpeedMult = 0.0125 / islandLoopMult
)

const (
	dayModeDay = iota
	dayModeTwilight
	dayModeNight
)

var (
	noonColor       = [4]float64{1, 0.83795786, 0.6556604, 1}
	nightColor      = [4]float64{0.29803923, 0.47410837, 0.6039216, 1}
	noonColorCloud  = [4]float64{1, 0.93626714, 0.8632076, 1}
	nightColorCloud = [4]float64{0.5137255, 0.8208757, 1, 1}
)

type distanceEvt struct {
	beat, length float64
	typ, ease    int
}

type enterEvt struct {
	beat, length float64
	ease         int
}

type speedEvt struct {
	beat, length float64
	from, to     float64
	ease         int
}

type dayEvt struct {
	beat, length float64
	start, end   int
	ease         int
}

type cloudEvt struct {
	beat, length    float64
	main, side, top int
	speed, endSpeed float64
	ease            int
}

type snowEvt struct {
	beat, length    float64
	cps             int
	speed, endSpeed float64
	ease            int
}

type treeEvt struct {
	beat, length    float64
	enable          bool
	main, side      int
	speed, endSpeed float64
	ease            int
}

type birdEvt struct {
	beat           float64
	typ            int
	xSpeed, zSpeed float64
	startZ         float64
	invert         bool
}

type baBumEvt struct {
	beat       float64
	count, alt bool
}

type shuttleFlight struct {
	active          bool
	startBeat       float64
	flyBeats        float64
	returning       bool
	long, tossed    bool
	dist            int
	missThroughBeat float64
}

type rainbowEvt struct {
	beat, speed, start float64
}

type interval struct {
	beat, length float64
}

type rallyIsland struct {
	path        string
	spritePaths []string
	startZ      float64
	norm        float64
	offset      float64
	fadeLeft    float64
}

type Module struct {
	ctx  *engine.Ctx
	proj kart.Aff

	distances       []distanceEvt
	enters          []enterEvt
	dayEvents       []dayEvt
	cloudEvents     []cloudEvt
	snowEvents      []snowEvt
	treeEvents      []treeEvt
	birdEvents      []birdEvt
	islandSpeeds    []speedEvt
	rallies         map[float64]bool
	baBums          map[float64]baBumEvt
	catches         map[float64]bool
	silences        []interval
	rainbows        []rainbowEvt
	islands         []rallyIsland
	islandEndZ      float64
	islandLastT     float64
	islandHasT      bool
	weather         airWeather
	weatherLastT    float64
	weatherHasT     bool
	bgTintPaths     []string
	cloudTintPaths  []string
	objectTintPaths []string
	lightTintPaths  []string
	activeEnter     enterEvt
	hasActiveEnter  bool

	shuttle shuttleFlight

	scheduledRally map[float64]bool
	scheduledBaBum map[float64]bool
}

func New() engine.Module {
	return &Module{
		rallies:        map[float64]bool{},
		baBums:         map[float64]baBumEvt{},
		catches:        map[float64]bool{},
		scheduledRally: map[float64]bool{},
		scheduledBaBum: map[float64]bool{},
	}
}

func (m *Module) ID() string { return "airRally" }

func (m *Module) Load(ctx *engine.Ctx) error {
	m.ctx = ctx
	if err := ctx.LoadAssets("airRally"); err != nil {
		return err
	}
	m.proj = kart.Translate(engine.ScreenW/2, engine.ScreenH/2).Mul(kart.Scale(54, -54))
	m.ctx.Scene.PlayState(m.ctx.Role("Baxter"), "Idle", 0, 1)
	m.ctx.Scene.PlayState(m.ctx.Role("Forthington"), "Idle", 0, 1)
	m.ctx.Scene.SetActive(m.ctx.Role("Shuttlecock"), false)
	m.initIslands()
	m.initTintPaths()
	m.initWeather()
	return nil
}

func (m *Module) OnEvent(e *riq.Entity) {
	b := e.Beat
	switch e.Datamodel {
	case "airRally/rally":
		m.rallies[b] = true
	case "airRally/ba bum bum bum":
		m.baBums[b] = baBumEvt{beat: b, count: boolParam(e, "toggle"), alt: boolParam(e, "toggle2")}
	case "airRally/catch":
		m.catches[b] = true
	case "airRally/silence":
		m.silences = append(m.silences, interval{b, e.Length})
	case "airRally/set distance":
		m.distances = append(m.distances, distanceEvt{beat: b, length: e.Length, typ: int(e.Float("type", 0)), ease: int(e.Float("ease", 0))})
	case "airRally/enter":
		ev := enterEvt{beat: b, length: e.Length, ease: int(e.Float("ease", 0))}
		m.enters = append(m.enters, ev)
		m.ctx.At(b, func() {
			m.activeEnter, m.hasActiveEnter = ev, true
			m.ctx.Sound("planesSpeedUp")
		})
	case "airRally/islandSpeed":
		m.islandSpeeds = append(m.islandSpeeds, speedEvt{
			beat: b, length: e.Length,
			from: e.Float("speed", 1), to: e.Float("endSpeed", 1),
			ease: int(e.Float("ease", 0)),
		})
	case "airRally/day":
		m.dayEvents = append(m.dayEvents, dayEvt{
			beat: b, length: e.Length,
			start: int(e.Float("start", 0)), end: int(e.Float("end", 0)),
			ease: int(e.Float("ease", 0)),
		})
	case "airRally/cloud":
		m.cloudEvents = append(m.cloudEvents, cloudEvt{
			beat: b, length: e.Length,
			main: int(e.Float("main", 30)), side: int(e.Float("side", 10)), top: int(e.Float("top", 0)),
			speed: e.Float("speed", 1), endSpeed: e.Float("endSpeed", 1),
			ease: int(e.Float("ease", 0)),
		})
	case "airRally/snowflake":
		m.snowEvents = append(m.snowEvents, snowEvt{
			beat: b, length: e.Length,
			cps:   int(e.Float("cps", 0)),
			speed: e.Float("speed", 1), endSpeed: e.Float("endSpeed", 1),
			ease: int(e.Float("ease", 0)),
		})
	case "airRally/tree":
		m.treeEvents = append(m.treeEvents, treeEvt{
			beat: b, length: e.Length,
			enable: boolParam(e, "enable"),
			main:   int(e.Float("main", 0)), side: int(e.Float("side", 0)),
			speed: e.Float("speed", 1), endSpeed: e.Float("endSpeed", 1),
			ease: int(e.Float("ease", 0)),
		})
	case "airRally/spawnBird":
		m.birdEvents = append(m.birdEvents, birdEvt{
			beat:   b,
			typ:    int(e.Float("type", 0)),
			xSpeed: e.Float("xSpeed", 1), zSpeed: e.Float("zSpeed", 1),
			startZ: e.Float("startZ", 200), invert: boolParam(e, "invert"),
		})
	case "airRally/forward":
		reset := boolParam(e, "reset")
		m.ctx.At(b, func() {
			state := "Forward"
			if reset {
				state = "Idle"
			}
			m.ctx.Scene.PlayFrozen(m.ctx.Role("Baxter"), state, 0)
			m.ctx.Scene.PlayFrozen(m.ctx.Role("Forthington"), state, 0)
		})
	case "airRally/4beat":
		m.scheduleCount4(b, e.Length)
	case "airRally/8beat":
		m.scheduleCount8(b, e.Length)
	case "airRally/forthington voice lines":
		m.scheduleVoice(b, int(e.Float("type", 0)))
	case "airRally/rainbow":
		m.rainbows = append(m.rainbows, rainbowEvt{beat: b, speed: e.Float("speed", 1), start: e.Float("start", 100)})
	}
}

func (m *Module) Ready() {
	sort.Slice(m.distances, func(i, j int) bool { return m.distances[i].beat < m.distances[j].beat })
	sort.Slice(m.enters, func(i, j int) bool { return m.enters[i].beat < m.enters[j].beat })
	sort.Slice(m.dayEvents, func(i, j int) bool { return m.dayEvents[i].beat < m.dayEvents[j].beat })
	sort.Slice(m.cloudEvents, func(i, j int) bool { return m.cloudEvents[i].beat < m.cloudEvents[j].beat })
	sort.Slice(m.snowEvents, func(i, j int) bool { return m.snowEvents[i].beat < m.snowEvents[j].beat })
	sort.Slice(m.treeEvents, func(i, j int) bool { return m.treeEvents[i].beat < m.treeEvents[j].beat })
	sort.Slice(m.birdEvents, func(i, j int) bool { return m.birdEvents[i].beat < m.birdEvents[j].beat })
	sort.Slice(m.islandSpeeds, func(i, j int) bool { return m.islandSpeeds[i].beat < m.islandSpeeds[j].beat })
	sort.Slice(m.silences, func(i, j int) bool { return m.silences[i].beat < m.silences[j].beat })
	for b := range m.rallies {
		m.scheduleRally(b)
	}
	for b, ev := range m.baBums {
		m.scheduleBaBum(b, ev.count, ev.alt)
	}
}

func (m *Module) OnSwitch(beat float64) {
	if beat == 0 {
		m.resetIslandMotion()
	}
	m.resetWeather(beat)
	m.persistEnter(beat)
	m.ctx.Scene.PlayState(m.ctx.Role("Baxter"), "Idle", beat, m.ctx.SecPerBeat(beat))
	m.ctx.Scene.PlayState(m.ctx.Role("Forthington"), "Idle", beat, m.ctx.SecPerBeat(beat))
}

func (m *Module) Whiff(beat float64) {
	m.ctx.Scene.PlayState(m.ctx.Role("Baxter"), "Hit", beat, 0.5)
	m.ctx.Sound("swing")
}

func (m *Module) Update(t, beat float64) {
	m.updateWeather(t, beat)
	if !m.treeStateAt(beat).enable {
		m.updateIslands(t, beat)
	}
}

func (m *Module) Draw(screen *ebiten.Image, t, beat float64) {
	screen.Fill(colorRGBA{181, 255, 255, 255})
	sc := m.ctx.Scene
	bgTint, cloudTint, objectTint, lightAlpha := m.dayColorsAt(beat)
	sc.SetZOver(m.ctx.Role("Baxter"), m.baxterZAt(beat))
	sc.SetZOver(m.ctx.Role("Forthington"), m.forthZAt(beat))
	m.applyStaticColorOverrides(sc, bgTint, cloudTint, objectTint, lightAlpha)
	treeState := m.treeStateAt(beat)
	sc.SetActive("Trees", treeState.enable)
	sc.SetActive("IslandManager", !treeState.enable)
	if !treeState.enable {
		m.applyIslandOverrides(sc, objectTint, lightAlpha)
	}
	m.ctx.SampleScene(beat)
	m.queueWeather(sc, beat, cloudTint, objectTint)
	m.queueBirds(sc, t, beat, objectTint)
	m.queueRainbow(sc, t, beat)
	m.queueShuttle(sc, beat)
	sc.Draw(screen, m.proj)
}

func (m *Module) scheduleRally(beat float64) {
	if m.scheduledRally[beat] || beat >= m.ctx.NextSwitchBeat(beat) {
		return
	}
	m.scheduledRally[beat] = true
	dist := m.distanceAt(beat)
	distName := m.distanceNameAt(beat, false)
	if dist != distClose {
		m.ctx.SoundAtOff(beat+1, "whooshForth_"+distName, 1, whooshRallyOffsets[dist])
	}
	if _, isBaBum := m.baBums[beat]; !m.silentAt(beat) && !isBaBum {
		m.ctx.SoundAtOff(beat, "en/nya_"+distName, 1, nyaOffsets[dist])
	}
	m.ctx.At(beat-0.5, func() {
		// ServeObject only creates the pre-serve toss when no shuttle is already
		// in flight. During recursive rallies the previous return is still active
		// until this serve starts, so resetting here makes the shuttle snap away.
		if !m.shuttle.active {
			m.startShuttle(beat-0.5, 0.5, false, false, true)
			m.ctx.Scene.PlayState(m.ctx.Role("Forthington"), "Ready", beat-0.5, 0.5)
		}
	})
	m.ctx.At(beat, func() {
		m.startShuttle(beat, 1, false, false, false)
		m.ctx.Scene.PlayState(m.ctx.Role("Forthington"), "Hit", beat, 0.5)
		m.ctx.Scene.PlayState(m.ctx.Role("Baxter"), m.readyState(beat), beat, 0.5)
		m.ctx.Sound("hitForth_" + distName)
	})
	if _, isBaBum := m.baBums[beat]; !isBaBum {
		m.ctx.At(beat+1, func() {
			m.ctx.Scene.PlayState(m.ctx.Role("Forthington"), "Ready", beat+1, 0.5)
		})
	}
	m.ctx.ScheduleInputAction(beat+1, 0, func(state float64, j engine.Judgment) {
		m.baxterHit(beat+1, false)
	}, func() {
		m.missShuttle(beat+1, false)
	})

	if ev, ok := m.baBums[beat]; ok {
		m.scheduleBaBum(beat, ev.count, ev.alt)
		return
	}
	if !m.catches[beat+2] {
		m.scheduleRally(beat + 2)
	}
}

func (m *Module) scheduleBaBum(beat float64, count, alt bool) {
	if m.scheduledBaBum[beat] || beat >= m.ctx.NextSwitchBeat(beat) {
		return
	}
	m.scheduledBaBum[beat] = true
	for i, bb := range []float64{beat - 0.5, beat, beat + 1, beat + 2} {
		name := "en/baBumBumBum_" + m.baBumDistanceName(bb, alt) + string(rune('1'+i))
		m.ctx.SoundAtOff(bb, name, 1, m.baBumOffset(bb, i, alt))
	}
	dist2 := m.distanceAt(beat + 2)
	distName2 := m.distanceNameAt(beat+2, false)
	if dist2 != distClose {
		m.ctx.SoundAtOff(beat+4, "whooshForth_"+distName2+"2", 1, whooshBaBumOffsets[dist2])
		m.ctx.SoundAt(beat+2, "hitForth_"+distName2+"2", 1)
	} else {
		m.ctx.SoundAt(beat+2, "hitForth_Close", 1)
	}
	if count && !m.catches[beat+6] {
		for _, c := range []struct {
			off float64
			num int
		}{{3, 2}, {4, 3}, {5, 4}} {
			bb := beat + c.off
			name := "en/countIn" + string(rune('0'+c.num)) + m.distanceNameAt(bb, false)
			m.ctx.SoundAtOff(bb, name, 1, countInOffsets[c.num-1])
		}
	}
	m.ctx.At(beat+1, func() {
		m.ctx.Scene.PlayState(m.ctx.Role("Forthington"), "Ready", beat+1, 0.5)
	})
	m.ctx.At(beat+1.5, func() {
		if !m.shuttle.active {
			m.startShuttle(beat+1.5, 0.5, false, false, true)
		}
	})
	m.ctx.At(beat+2, func() {
		m.startShuttle(beat+2, 2, false, true, false)
		m.ctx.Scene.PlayState(m.ctx.Role("Forthington"), "Hit", beat+2, 0.5)
		m.ctx.Scene.PlayState(m.ctx.Role("Baxter"), m.readyState(beat+2), beat+2, 0.5)
	})
	for _, bb := range []float64{beat + 3, beat + 3.5} {
		bb := bb
		m.ctx.At(bb, func() {
			m.ctx.Scene.PlayState(m.ctx.Role("Forthington"), "TalkShort", bb, 0.5)
		})
	}
	m.ctx.At(beat+4, func() {
		m.ctx.Scene.PlayState(m.ctx.Role("Forthington"), "Ready", beat+4, 0.5)
	})
	m.ctx.ScheduleInputAction(beat+4, 0, func(state float64, j engine.Judgment) {
		m.baxterHit(beat+4, true)
	}, func() {
		m.missShuttle(beat+4, true)
	})
	if m.catches[beat+6] {
		m.ctx.At(beat+6, func() { m.catchBirdie(beat + 6) })
		return
	}
	if ev, ok := m.baBums[beat+4]; ok {
		m.scheduleBaBum(beat+4, ev.count, ev.alt)
		return
	}
	m.scheduleRally(beat + 6)
}

func (m *Module) scheduleCount4(beat, length float64) {
	unit := length / 4
	for i := 0; i < 4; i++ {
		bb := beat + float64(i)*unit
		name := "en/countIn" + string(rune('1'+i)) + m.distanceNameAt(bb, false)
		m.ctx.SoundAtOff(bb, name, 1, countInOffsets[i])
		m.ctx.At(bb, func() {
			m.ctx.Scene.PlayState(m.ctx.Role("Forthington"), "TalkShort", bb, 0.5)
		})
	}
}

func (m *Module) scheduleCount8(beat, length float64) {
	unit := length / 8
	for _, step := range []struct {
		idx int
		num int
	}{
		{0, 1},
		{2, 2},
		{4, 1},
		{5, 2},
		{6, 3},
		{7, 4},
	} {
		bb := beat + float64(step.idx)*unit
		name := "en/countIn" + string(rune('0'+step.num)) + m.distanceNameAt(bb, false)
		m.ctx.SoundAtOff(bb, name, 1, countInOffsets[step.num-1])
		m.ctx.At(bb, func() {
			m.ctx.Scene.PlayState(m.ctx.Role("Forthington"), "TalkShort", bb, 0.5)
		})
	}
}

func (m *Module) scheduleVoice(beat float64, typ int) {
	if typ < 0 || typ > 3 {
		return
	}
	name := "en/countIn" + string(rune('1'+typ)) + m.distanceNameAt(beat, false)
	m.ctx.SoundAtOff(beat, name, 1, countInOffsets[typ])
	m.ctx.At(beat, func() {
		m.ctx.Scene.PlayState(m.ctx.Role("Forthington"), "TalkShort", beat, 0.5)
	})
}

func (m *Module) baxterHit(beat float64, long bool) {
	m.ctx.Scene.PlayState(m.ctx.Role("Baxter"), "Hit", beat, 0.5)
	m.ctx.Sound("hitBaxter_" + m.distanceNameAt(beat, false))
	fly := 1.0
	if long {
		fly = 2
	}
	m.startShuttle(beat, fly, true, long, false)
	if m.catches[beat+fly] {
		bb := beat + fly
		m.ctx.At(bb, func() { m.catchBirdie(bb) })
	}
}

func (m *Module) missShuttle(beat float64, long bool) {
	m.ctx.Scene.PlayState(m.ctx.Role("Baxter"), "Hit", beat, 0.5)
	m.ctx.PlayCommon("miss")
	m.shuttle.active = false
}

func (m *Module) catchBirdie(beat float64) {
	m.ctx.Scene.PlayState(m.ctx.Role("Forthington"), "Catch", beat, 0.5)
	m.ctx.Sound("birdieCatch")
	m.shuttle.active = false
}

func (m *Module) startShuttle(beat, flyBeats float64, returning, long, tossed bool) {
	m.shuttle = shuttleFlight{
		active: true, startBeat: beat, flyBeats: flyBeats,
		returning: returning, long: long, tossed: tossed, dist: m.distanceAt(beat),
	}
}

func (m *Module) forthZAt(beat float64) float64 {
	if z, ok := m.enterZAt(beat); ok {
		return z
	}
	cur := wayPointHomeZ
	for _, e := range m.distances {
		if beat < e.beat {
			break
		}
		target := distZ[clampDist(e.typ)]
		if beat < e.beat+wayPointBeatLength {
			u := (beat - e.beat) / wayPointBeatLength
			return engine.Ease(e.ease, cur, target, u)
		}
		cur = target
	}
	return cur
}

func (m *Module) baxterZAt(beat float64) float64 {
	if z, ok := m.enterZAt(beat); ok {
		return z
	}
	return wayPointHomeZ
}

func (m *Module) enterZAt(beat float64) (float64, bool) {
	ev := m.activeEnter
	if !m.hasActiveEnter {
		if m.ctx != nil && m.ctx.App != nil {
			return 0, false
		}
		var ok bool
		ev, ok = m.persistedEnterAt(beat)
		if !ok {
			return 0, false
		}
	}
	if beat < ev.beat {
		return wayPointEnter, true
	}
	if ev.length <= 0 {
		return wayPointHomeZ, true
	}
	if beat <= ev.beat+ev.length {
		u := (beat - ev.beat) / ev.length
		return engine.Ease(ev.ease, wayPointEnter, wayPointHomeZ, u), true
	}
	return 0, false
}

func (m *Module) persistEnter(beat float64) {
	if ev, ok := m.persistedEnterAt(beat); ok {
		m.activeEnter, m.hasActiveEnter = ev, true
		return
	}
	m.hasActiveEnter = false
}

func (m *Module) persistedEnterAt(beat float64) (enterEvt, bool) {
	nextSwitch := math.Inf(1)
	if m.ctx != nil && m.ctx.App != nil {
		nextSwitch = m.ctx.NextSwitchBeat(beat)
	}
	for _, e := range m.enters {
		if e.beat >= beat && e.beat < nextSwitch {
			return e, true
		}
	}
	var active enterEvt
	ok := false
	for _, e := range m.enters {
		if e.beat >= beat {
			break
		}
		if beat < e.beat+e.length {
			active, ok = e, true
		}
	}
	return active, ok
}

func (m *Module) distanceAt(beat float64) int {
	d := distClose
	for _, e := range m.distances {
		if e.beat > beat {
			break
		}
		d = clampDist(e.typ)
	}
	return d
}

func clampDist(d int) int {
	if d < distClose {
		return distClose
	}
	if d > distFarthest {
		return distFarthest
	}
	return d
}

func (m *Module) distanceNameAt(beat float64, farFarther bool) string {
	switch m.distanceAt(beat) {
	case distFar:
		return "Far"
	case distFarther:
		if farFarther {
			return "Far"
		}
		return "Farther"
	case distFarthest:
		return "Farthest"
	default:
		return "Close"
	}
}

func (m *Module) readyState(beat float64) string {
	return m.distanceNameAt(beat, true) + "Ready"
}

func (m *Module) baBumDistanceName(beat float64, alt bool) string {
	name := m.distanceNameAt(beat, false)
	if name == "Far" && alt {
		return "FarAlt"
	}
	return name
}

func (m *Module) baBumOffset(beat float64, idx int, alt bool) float64 {
	d := m.distanceAt(beat)
	if d == distFar && alt {
		return baBumFarAltOffsets[idx]
	}
	return baBumOffsets[d][idx]
}

func (m *Module) silentAt(beat float64) bool {
	for _, s := range m.silences {
		if beat >= s.beat && beat < s.beat+s.length {
			return true
		}
	}
	return false
}

func (m *Module) initIslands() {
	roots := []string{"IslandManager/Island", "IslandManager/island1", "IslandManager/Island2", "IslandManager/island3"}
	nodes := m.ctx.Assets.Rig.Nodes
	byPath := map[string]int{}
	for i, n := range nodes {
		byPath[n.Path] = i
	}
	var minZ, maxZ float64
	found := false
	for _, p := range roots {
		i, ok := byPath[p]
		if !ok {
			continue
		}
		z := nodes[i].PosZ
		if !found || z < minZ {
			minZ = z
		}
		if !found || z > maxZ {
			maxZ = z
		}
		found = true
	}
	if !found || maxZ == minZ {
		return
	}
	m.islandEndZ = -(maxZ - minZ) * islandLoopMult
	m.islands = m.islands[:0]
	for _, p := range roots {
		i, ok := byPath[p]
		if !ok {
			continue
		}
		z := nodes[i].PosZ
		it := rallyIsland{
			path:   p,
			startZ: z,
			offset: (1 - (z-minZ)/(maxZ-minZ)) / islandLoopMult,
		}
		prefix := p + "/"
		for _, n := range nodes {
			if n.Sprite == "" {
				continue
			}
			if n.Path == p || len(n.Path) > len(prefix) && n.Path[:len(prefix)] == prefix {
				it.spritePaths = append(it.spritePaths, n.Path)
			}
		}
		m.islands = append(m.islands, it)
	}
	m.resetIslandMotion()
}

func (m *Module) resetIslandMotion() {
	for i := range m.islands {
		m.islands[i].norm = 0
		m.islands[i].fadeLeft = 0
	}
	m.islandHasT = false
}

func (m *Module) updateIslands(t, beat float64) {
	if len(m.islands) == 0 {
		return
	}
	if !m.islandHasT || t < m.islandLastT {
		m.islandLastT = t
		m.islandHasT = true
		return
	}
	dt := t - m.islandLastT
	m.islandLastT = t
	if dt <= 0 {
		return
	}
	speed := islandSpeedMult * m.islandSpeedAt(beat)
	for i := range m.islands {
		it := &m.islands[i]
		it.norm += speed * dt
		wrapped := false
		if m.islandZ(*it) < m.islandEndZ {
			// RvlIsland resets to -normalizedOffset so the island rejoins the
			// far end of the staggered queue instead of snapping to its own startZ.
			it.norm = -it.offset
			it.fadeLeft = 0.4
			wrapped = true
		}
		if it.fadeLeft > 0 && !wrapped {
			it.fadeLeft = math.Max(0, it.fadeLeft-dt)
		}
	}
}

func (m *Module) islandZ(it rallyIsland) float64 {
	return it.startZ + m.islandEndZ*it.norm
}

func (m *Module) applyIslandOverrides(sc *kart.SceneInst, objectTint [4]float64, lightAlpha float64) {
	for _, it := range m.islands {
		sc.SetZOver(it.path, m.islandZ(it))
		alpha := 1.0
		if it.fadeLeft > 0 {
			alpha = 1 - it.fadeLeft/0.4
		}
		for _, p := range it.spritePaths {
			if strings.HasSuffix(p, "island2lights") {
				sc.SetColorOver(p, [4]float64{1, 1, 1, lightAlpha * alpha})
				continue
			}
			c := objectTint
			c[3] *= alpha
			sc.SetColorOver(p, c)
		}
	}
}

func (m *Module) islandSpeedAt(beat float64) float64 {
	speed := 1.0
	for _, e := range m.islandSpeeds {
		if beat < e.beat {
			break
		}
		if e.length > 0 && beat < e.beat+e.length {
			u := (beat - e.beat) / e.length
			return engine.Ease(e.ease, e.from, e.to, u)
		}
		speed = e.to
	}
	return speed
}

func (m *Module) queueShuttle(sc *kart.SceneInst, beat float64) {
	if !m.shuttle.active || m.shuttle.flyBeats <= 0 {
		return
	}
	u := (beat - m.shuttle.startBeat) / m.shuttle.flyBeats
	if u < 0 || u > 1.25 {
		return
	}
	if u > 1 {
		u = 1
	}
	forthPath, baxterPath := m.ctx.Role("Forthington")+"/Root_Forthington/Root_Body/Birdie", m.ctx.Role("Baxter")+"/Shuttle_Root"
	fromPath, toPath := forthPath, baxterPath
	forthZ, baxterZ := m.forthZAt(beat), m.baxterZAt(beat)
	if m.shuttle.tossed {
		fromPath, toPath = forthPath, forthPath
		baxterZ = forthZ
	} else if m.shuttle.returning {
		fromPath, toPath = toPath, fromPath
		forthZ, baxterZ = baxterZ, forthZ
	}
	from, ok1 := sc.NodeWorld(fromPath)
	to, ok2 := sc.NodeWorld(toPath)
	if !ok1 || !ok2 {
		return
	}
	x0, y0 := from.Apply(0, 0)
	x1, y1 := to.Apply(0, 0)
	x := x0 + (x1-x0)*u
	y := y0 + (y1-y0)*u
	height := 2.5
	if m.shuttle.long {
		height = 5.2
	}
	if m.shuttle.tossed {
		height = 2.5
	}
	arc := 1 - math.Pow(u*2-1, 2)
	y += arc * height
	z := forthZ + (baxterZ-forthZ)*u
	rot := -math.Pi / 2
	if !m.shuttle.tossed {
		rot = math.Atan2(y1-y0, x1-x0) - math.Pi/2
	}
	sc.Queue(kart.ExtraSprite{
		Sprite: "Shuttlecock",
		World:  kart.Translate(x, y).Mul(kart.Rotate(rot)).Mul(kart.Scale(0.7, 0.7)),
		Z:      z,
		Order:  20,
	})
}

func (m *Module) queueRainbow(sc *kart.SceneInst, t, beat float64) {
	for _, r := range m.rainbows {
		born := r.beat
		if m.ctx != nil && m.ctx.App != nil {
			born = m.ctx.BeatToTime(r.beat)
		}
		ageSec := t - born
		if ageSec < 0 || ageSec > 60 {
			continue
		}
		alpha := clamp01(beat-r.beat) * 0.07058824
		z := r.start - 2*r.speed*ageSec
		if z < -20 {
			continue
		}
		root := kart.Translate(-5.2, 5.7).Mul(kart.Scale(3, 3))
		for _, part := range []struct {
			x, y, rot, sx float64
		}{
			{2.37, -1.55, math.Pi / 2, 1},
			{15.8, -1.55, 3 * math.Pi / 2, -1},
		} {
			sc.Queue(kart.ExtraSprite{
				Sprite: "rainbow",
				World:  root.Mul(kart.Translate(part.x, part.y)).Mul(kart.Rotate(part.rot)).Mul(kart.Scale(part.sx, 1)),
				Z:      z,
				Order:  -99,
				Tint:   [4]float64{1, 1, 1, alpha},
			})
		}
	}
}

func (m *Module) initTintPaths() {
	if m.ctx == nil || m.ctx.Assets == nil {
		return
	}
	for _, n := range m.ctx.Assets.Rig.Nodes {
		if n.Sprite == "" {
			continue
		}
		switch {
		case n.Path == "BG/Color":
			m.bgTintPaths = append(m.bgTintPaths, n.Path)
		case strings.HasPrefix(n.Path, "BG/bgclouds"), strings.HasPrefix(n.Path, "Clouds_"), strings.HasPrefix(n.Path, "Snowflakes"):
			m.cloudTintPaths = append(m.cloudTintPaths, n.Path)
		case strings.HasSuffix(n.Path, "island2lights"):
			m.lightTintPaths = append(m.lightTintPaths, n.Path)
		default:
			m.objectTintPaths = append(m.objectTintPaths, n.Path)
		}
	}
}

func (m *Module) applyStaticColorOverrides(sc *kart.SceneInst, bgTint, cloudTint, objectTint [4]float64, lightAlpha float64) {
	for _, p := range m.bgTintPaths {
		sc.SetColorOver(p, bgTint)
	}
	for _, p := range m.cloudTintPaths {
		sc.SetColorOver(p, cloudTint)
	}
	for _, p := range m.objectTintPaths {
		sc.SetColorOver(p, objectTint)
	}
	for _, p := range m.lightTintPaths {
		sc.SetColorOver(p, [4]float64{1, 1, 1, lightAlpha})
	}
}

func (m *Module) dayColorsAt(beat float64) (bgTint, cloudTint, objectTint [4]float64, lightAlpha float64) {
	bgTint, cloudTint, objectTint = white(), white(), white()
	for _, e := range m.dayEvents {
		if beat < e.beat {
			break
		}
		fromObj, fromBg, fromCloud, fromLight := dayPalette(e.start)
		toObj, toBg, toCloud, toLight := dayPalette(e.end)
		if e.length > 0 && beat < e.beat+e.length {
			u := (beat - e.beat) / e.length
			return easeColor(e.ease, fromBg, toBg, u), easeColor(e.ease, fromCloud, toCloud, u), easeColor(e.ease, fromObj, toObj, u), engine.Ease(e.ease, fromLight, toLight, u)
		}
		bgTint, cloudTint, objectTint, lightAlpha = toBg, toCloud, toObj, toLight
	}
	return bgTint, cloudTint, objectTint, lightAlpha
}

func dayPalette(mode int) (objectTint, bgTint, cloudTint [4]float64, lightAlpha float64) {
	switch mode {
	case dayModeTwilight:
		return [4]float64{0, 0, 0, 1}, noonColor, noonColorCloud, 0
	case dayModeNight:
		return white(), nightColor, nightColorCloud, 1
	default:
		return white(), white(), white(), 0
	}
}

func easeColor(ease int, from, to [4]float64, u float64) [4]float64 {
	return [4]float64{
		engine.Ease(ease, from[0], to[0], u),
		engine.Ease(ease, from[1], to[1], u),
		engine.Ease(ease, from[2], to[2], u),
		engine.Ease(ease, from[3], to[3], u),
	}
}

const (
	weatherCloud = iota
	weatherSnow
	weatherTree
)

type airWeather struct {
	rng uint64

	cloudMain  *weatherManager
	cloudLeft  *weatherManager
	cloudRight *weatherManager
	cloudTop   *weatherManager
	snow       *weatherManager
	treeMain   *weatherManager
	treeLeft   *weatherManager
	treeRight  *weatherManager
	treeLI     *weatherManager
	treeRI     *weatherManager
	all        []*weatherManager
	trees      []*weatherManager
}

type weatherManager struct {
	kind int
	root [2]float64
	z    float64

	max     int
	prebake float64
	rate    int
	speed   float64

	spawnX, spawnY float64
	baseSpeed      float64
	fadeDist       float64
	lifeTime       float64
	fadeInTime     float64
	scale          float64
	sprites        []string

	lastSpawn float64
	objs      []weatherObj
}

type weatherObj struct {
	x, y, z float64
	age     float64
	rot     float64
	sprite  string
	tree    int
}

type cloudState struct {
	main, side, top int
	speed           float64
}

type snowState struct {
	cps   int
	speed float64
}

type treeState struct {
	enable     bool
	main, side int
	speed      float64
}

func (m *Module) initWeather() {
	m.weather.rng = 0x4a697252616c6c79
	// Values are serialized on the Air Rally prefab's CloudsManager/Cloud
	// components. Keeping them here avoids a silent downgrade when the extracted
	// scene only contains the inactive pool templates.
	m.weather.cloudMain = &weatherManager{kind: weatherCloud, root: [2]float64{0, -7}, z: 128, max: 300, prebake: 1, rate: 30, speed: 1, spawnX: 65, spawnY: 2, baseSpeed: 60, fadeDist: 24, lifeTime: 3, fadeInTime: 0.25, scale: 1.75, sprites: []string{"Cloud_0", "Cloud_1", "Cloud_2"}}
	m.weather.cloudLeft = &weatherManager{kind: weatherCloud, root: [2]float64{-80, 0}, z: 128, max: 100, prebake: 1, rate: 10, speed: 1, spawnX: 10, spawnY: 5, baseSpeed: 52, fadeDist: 24, lifeTime: 3, fadeInTime: 0.25, scale: 2.5, sprites: []string{"Cloud_0", "Cloud_1", "Cloud_2"}}
	m.weather.cloudRight = &weatherManager{kind: weatherCloud, root: [2]float64{80, 0}, z: 128, max: 100, prebake: 1, rate: 10, speed: 1, spawnX: 10, spawnY: 5, baseSpeed: 52, fadeDist: 24, lifeTime: 3, fadeInTime: 0.25, scale: 2.5, sprites: []string{"Cloud_0", "Cloud_1", "Cloud_2"}}
	m.weather.cloudTop = &weatherManager{kind: weatherCloud, root: [2]float64{0, -3}, z: 128, max: 100, prebake: 1, rate: 0, speed: 1, spawnX: 76, spawnY: 2, baseSpeed: 60, fadeDist: 24, lifeTime: 3, fadeInTime: 0.25, scale: 1.75, sprites: []string{"Cloud_0", "Cloud_1", "Cloud_2"}}
	m.weather.snow = &weatherManager{kind: weatherSnow, root: [2]float64{0, 0}, z: 128, max: 200, prebake: 1, rate: 0, speed: 1, spawnX: 100, spawnY: 50, baseSpeed: 60, fadeDist: 24, lifeTime: 3, fadeInTime: 0.25, scale: 0.7, sprites: []string{"snowflake"}}
	m.weather.treeMain = &weatherManager{kind: weatherTree, root: [2]float64{0, -8}, z: 128, max: 100, prebake: 1, rate: 0, speed: 1, spawnX: 30, spawnY: 1, baseSpeed: 60, fadeDist: 24, lifeTime: 2, fadeInTime: 0.25, scale: 1}
	m.weather.treeLeft = &weatherManager{kind: weatherTree, root: [2]float64{-80, -7}, z: 128, max: 80, prebake: 1, rate: 0, speed: 1, spawnX: 30, spawnY: 1, baseSpeed: 60, fadeDist: 24, lifeTime: 2, fadeInTime: 0.25, scale: 1}
	m.weather.treeRight = &weatherManager{kind: weatherTree, root: [2]float64{80, -7}, z: 128, max: 80, prebake: 1, rate: 0, speed: 1, spawnX: 30, spawnY: 1, baseSpeed: 60, fadeDist: 24, lifeTime: 2, fadeInTime: 0.25, scale: 1}
	m.weather.treeLI = &weatherManager{kind: weatherTree, root: [2]float64{-40, -8}, z: 128, max: 100, prebake: 1, rate: 0, speed: 1, spawnX: 30, spawnY: 1, baseSpeed: 60, fadeDist: 24, lifeTime: 2, fadeInTime: 0.25, scale: 1}
	m.weather.treeRI = &weatherManager{kind: weatherTree, root: [2]float64{40, -8}, z: 128, max: 100, prebake: 1, rate: 0, speed: 1, spawnX: 30, spawnY: 1, baseSpeed: 60, fadeDist: 24, lifeTime: 2, fadeInTime: 0.25, scale: 1}
	m.weather.all = []*weatherManager{
		m.weather.cloudMain, m.weather.cloudLeft, m.weather.cloudRight, m.weather.cloudTop,
		m.weather.snow, m.weather.treeMain, m.weather.treeLeft, m.weather.treeRight, m.weather.treeLI, m.weather.treeRI,
	}
	m.weather.trees = []*weatherManager{m.weather.treeMain, m.weather.treeLeft, m.weather.treeRight, m.weather.treeLI, m.weather.treeRI}
	m.resetWeather(0)
}

func (m *Module) resetWeather(beat float64) {
	if len(m.weather.all) == 0 {
		return
	}
	now := 0.0
	if m.ctx != nil && m.ctx.App != nil {
		now = m.ctx.BeatToTime(beat)
	}
	m.applyWeatherState(beat)
	for _, mgr := range m.weather.all {
		mgr.reset(now, &m.weather.rng)
	}
	m.weatherHasT = false
}

func (m *Module) updateWeather(t, beat float64) {
	if len(m.weather.all) == 0 {
		return
	}
	m.applyWeatherState(beat)
	if !m.weatherHasT || t < m.weatherLastT {
		m.weatherLastT = t
		m.weatherHasT = true
		return
	}
	dt := t - m.weatherLastT
	m.weatherLastT = t
	if dt <= 0 {
		return
	}
	tree := m.treeStateAt(beat)
	for _, mgr := range m.weather.all {
		if mgr.kind == weatherTree && !tree.enable {
			continue
		}
		mgr.update(dt, t, &m.weather.rng)
	}
}

func (m *Module) applyWeatherState(beat float64) {
	cloud := m.cloudStateAt(beat)
	m.weather.cloudMain.rate, m.weather.cloudMain.speed = cloud.main, cloud.speed
	m.weather.cloudLeft.rate, m.weather.cloudLeft.speed = cloud.side, cloud.speed
	m.weather.cloudRight.rate, m.weather.cloudRight.speed = cloud.side, cloud.speed
	m.weather.cloudTop.rate, m.weather.cloudTop.speed = cloud.top, cloud.speed

	snow := m.snowStateAt(beat)
	m.weather.snow.rate, m.weather.snow.speed = snow.cps, snow.speed

	tree := m.treeStateAt(beat)
	rates := []int{tree.main, tree.side, tree.side, tree.main, tree.main}
	for i, mgr := range m.weather.trees {
		if tree.enable {
			mgr.rate = rates[i]
		} else {
			mgr.rate = 0
		}
		mgr.speed = tree.speed
	}
}

func (m *Module) cloudStateAt(beat float64) cloudState {
	st := cloudState{main: 30, side: 10, top: 0, speed: 1}
	for _, e := range m.cloudEvents {
		if beat < e.beat {
			break
		}
		st.main, st.side, st.top = e.main, e.side, e.top
		st.speed = easedEventValue(beat, e.beat, e.length, e.speed, e.endSpeed, e.ease)
	}
	return st
}

func (m *Module) snowStateAt(beat float64) snowState {
	st := snowState{cps: 0, speed: 1}
	for _, e := range m.snowEvents {
		if beat < e.beat {
			break
		}
		st.cps = e.cps
		st.speed = easedEventValue(beat, e.beat, e.length, e.speed, e.endSpeed, e.ease)
	}
	return st
}

func (m *Module) treeStateAt(beat float64) treeState {
	st := treeState{enable: false, speed: 1}
	for _, e := range m.treeEvents {
		if beat < e.beat {
			break
		}
		st.enable, st.main, st.side = e.enable, e.main, e.side
		st.speed = easedEventValue(beat, e.beat, e.length, e.speed, e.endSpeed, e.ease)
	}
	return st
}

func easedEventValue(beat, start, length, from, to float64, ease int) float64 {
	if length > 0 && beat < start+length {
		return engine.Ease(ease, from, to, (beat-start)/length)
	}
	return to
}

func (mgr *weatherManager) reset(now float64, rng *uint64) {
	mgr.lastSpawn = now
	mgr.objs = mgr.objs[:0]
	n := int(math.Round(float64(mgr.rate) * mgr.prebake))
	if n > mgr.max {
		n = mgr.max
	}
	for i := 0; i < n; i++ {
		mgr.spawn(true, rng)
	}
}

func (mgr *weatherManager) update(dt, now float64, rng *uint64) {
	if mgr.rate > 0 && now-mgr.lastSpawn > 1/float64(mgr.rate) {
		mgr.lastSpawn = now
		mgr.spawn(false, rng)
	}
	dst := mgr.objs[:0]
	for _, o := range mgr.objs {
		o.age += dt * mgr.speed
		o.z -= mgr.baseSpeed * mgr.speed * dt
		if o.age <= mgr.lifeTime {
			dst = append(dst, o)
		}
	}
	mgr.objs = dst
}

func (mgr *weatherManager) spawn(prebake bool, rng *uint64) {
	if len(mgr.objs) >= mgr.max {
		return
	}
	o := weatherObj{
		x: mgr.root[0] + randRange(rng, -mgr.spawnX, mgr.spawnX),
		y: mgr.root[1] + randRange(rng, -mgr.spawnY, mgr.spawnY),
		z: 0,
	}
	switch mgr.kind {
	case weatherCloud:
		o.sprite = mgr.sprites[randIndex(rng, len(mgr.sprites))]
	case weatherSnow:
		o.sprite = "snowflake"
		o.rot = randRange(rng, 0, math.Pi*2)
	case weatherTree:
		o.tree = randTreeType(rng)
	}
	if prebake {
		o.age = randRange(rng, 0, mgr.lifeTime)
		o.z -= mgr.baseSpeed * o.age
	}
	mgr.objs = append(mgr.objs, o)
}

func randTreeType(rng *uint64) int {
	r := int(randRange(rng, 0, 200))
	switch {
	case r < 1:
		return 5
	case r < 6:
		return 3
	case r < 11:
		return 2
	case r < 16:
		return 1
	case r < 21:
		return 0
	default:
		return 4
	}
}

func (m *Module) queueWeather(sc *kart.SceneInst, beat float64, cloudTint, objectTint [4]float64) {
	cam := [3]float64{0, 0, -10}
	if m.ctx != nil && m.ctx.App != nil {
		cam = m.ctx.CameraAt(beat)
	}
	for _, mgr := range []*weatherManager{m.weather.cloudMain, m.weather.cloudLeft, m.weather.cloudRight, m.weather.cloudTop, m.weather.snow} {
		mgr.queue(sc, cam, cloudTint)
	}
	if !m.treeStateAt(beat).enable {
		return
	}
	for _, mgr := range m.weather.trees {
		mgr.queue(sc, cam, objectTint)
	}
}

func (mgr *weatherManager) queue(sc *kart.SceneInst, cam [3]float64, tint [4]float64) {
	if mgr == nil {
		return
	}
	for _, o := range mgr.objs {
		z := mgr.z + o.z
		alpha := mgr.alpha(o, z, cam)
		if alpha <= 0 {
			continue
		}
		switch mgr.kind {
		case weatherCloud, weatherSnow:
			c := tint
			c[3] *= alpha
			sc.Queue(kart.ExtraSprite{
				Sprite: o.sprite,
				World:  kart.Translate(o.x, o.y).Mul(kart.Rotate(o.rot)).Mul(kart.Scale(mgr.scale, mgr.scale)),
				Z:      z,
				Order:  0,
				Tint:   c,
			})
		case weatherTree:
			queueTree(sc, o, z, alpha, tint)
		}
	}
}

func (mgr *weatherManager) alpha(o weatherObj, z float64, cam [3]float64) float64 {
	dx, dy, dz := o.x-cam[0], o.y-cam[1], z-cam[2]
	dist := math.Sqrt(dx*dx + dy*dy + dz*dz)
	if dist <= mgr.fadeDist {
		return clamp01(dist / mgr.fadeDist)
	}
	if o.age < mgr.fadeInTime {
		return clamp01(o.age / mgr.fadeInTime)
	}
	return 1
}

func queueTree(sc *kart.SceneInst, o weatherObj, z, alpha float64, tint [4]float64) {
	rootTint := tint
	rootTint[3] *= alpha
	reflTint := tint
	reflTint[3] *= math.Min(alpha, 0.27058825)
	if o.tree == 5 {
		sc.Queue(kart.ExtraSprite{
			Sprite: "threefish",
			World:  kart.Translate(o.x+0.314, o.y-3.784).Mul(kart.Scale(0.5, 0.5)),
			Z:      z,
			Order:  0,
			Tint:   reflTint,
		})
		return
	}
	sprite := "tree" + string(rune('0'+clampInt(o.tree, 0, 4)))
	root := kart.Translate(o.x, o.y).Mul(kart.Scale(0.5, 0.5))
	sc.Queue(kart.ExtraSprite{Sprite: sprite, World: root, Z: z, Order: 0, Tint: rootTint})
	sc.Queue(kart.ExtraSprite{
		Sprite: sprite,
		World:  kart.Translate(o.x+0.314, o.y-3.784).Mul(kart.Scale(0.5, 0.5)),
		Z:      z,
		Order:  0,
		Tint:   reflTint,
	})
	if o.tree == 4 {
		sc.Queue(kart.ExtraSprite{
			Sprite: "tree4crown",
			World:  root.Mul(kart.Translate(-1.02, 4.8)),
			Z:      z,
			Order:  0,
			Tint:   rootTint,
		})
	}
}

type birdOffset struct {
	x, y, z float64
}

type wingSpec struct {
	path     string
	fallback string
}

var (
	pterosaurOffsets = []birdOffset{
		{0, 0, 0}, {8, 0, -8}, {4, 0, -4}, {4, 0, 4}, {2, 0, 2},
		{6, 0, 6}, {2, 0, -2}, {6, 0, -6}, {8, 0, 8},
	}
	gooseOffsets = []birdOffset{
		{0, 0, 0}, {2, 0, -2}, {4, 0, -4}, {6, 0, -6}, {8, 0, -8},
		{2, 0, 2}, {4, 0, 4}, {6, 0, 6}, {8, 0, 8},
	}
	bluebirdOffsets = []birdOffset{{0, 0, 0}, {5, 0, 0}, {11, 0, 0}, {17, 0, 0}}
)

func (m *Module) queueBirds(sc *kart.SceneInst, t, beat float64, objectTint [4]float64) {
	if len(m.birdEvents) == 0 || m.ctx == nil || m.ctx.App == nil || m.ctx.Assets == nil {
		return
	}
	for ei, ev := range m.birdEvents {
		born := m.ctx.BeatToTime(ev.beat)
		ageSec := t - born
		if ageSec < 0 || ageSec > 40 {
			continue
		}
		x0, sign := 190.0, 1.0
		xMult := ev.xSpeed
		if ev.invert {
			x0, sign = -190, -1
			xMult = -ev.xSpeed
		}
		x := x0 - 9*xMult*ageSec
		z := ev.startZ - 7.5*ev.zSpeed*ageSec
		if z < -20 || math.Abs(x) > 260 {
			continue
		}
		root := kart.Translate(x, 14.1).Mul(kart.Scale(sign, 1))
		switch clampInt(ev.typ, 0, 2) {
		case 0:
			m.queueBirdGroup(sc, "Pterosaur/Idle", pterosaurOffsets, root, z, ei, ageSec, objectTint,
				"pterosaurbody", [2]float64{0.59, 1.38},
				[]wingSpec{{"leftWing", "ptleftwing0"}, {"rightWing", "ptrightwing0"}})
		case 1:
			m.queueBirdGroup(sc, "Goose/Idle", gooseOffsets, root, z, ei, ageSec, objectTint,
				"goosebody", [2]float64{0.52, 1.94},
				[]wingSpec{{"leftWing", "gleftwing0"}, {"rightWing", "grightwing0"}})
		case 2:
			m.queueBirdGroup(sc, "Bluebird/Idle", bluebirdOffsets, root, z, ei, ageSec, objectTint,
				"bluebirdbody", [2]float64{0.47, -0.16},
				[]wingSpec{{"Wing", "bbwing0"}})
		}
	}
}

func (m *Module) queueBirdGroup(sc *kart.SceneInst, animName string, offsets []birdOffset, root kart.Aff, z float64, eventIdx int, ageSec float64, tint [4]float64, bodySprite string, bodyPos [2]float64, wings []wingSpec) {
	anim := m.ctx.Assets.Anims[animName]
	if anim == nil {
		return
	}
	for i, off := range offsets {
		localZ := z + off.z
		if localZ < -20 {
			continue
		}
		clipT := ageSec + float64(eventIdx)*0.071 + float64(i)*0.137
		if anim.Duration > 0 {
			clipT = math.Mod(clipT, anim.Duration)
		}
		birdRoot := root.Mul(kart.Translate(off.x, off.y))
		sc.Queue(kart.ExtraSprite{
			Sprite: bodySprite,
			World:  birdRoot.Mul(kart.Translate(bodyPos[0], bodyPos[1])),
			Z:      localZ,
			Order:  0,
			Tint:   tint,
		})
		for _, w := range wings {
			pos := sampleAnimPos(anim, w.path, clipT)
			sprite := sampleAnimSprite(anim, w.path, clipT, w.fallback)
			sc.Queue(kart.ExtraSprite{
				Sprite: sprite,
				World:  birdRoot.Mul(kart.Translate(pos[0], pos[1])),
				Z:      localZ,
				Order:  0,
				Tint:   tint,
			})
		}
	}
}

func sampleAnimPos(anim *kmdata.Anim, path string, t float64) [2]float64 {
	if anim == nil {
		return [2]float64{}
	}
	if c, ok := anim.Pos[path]; ok {
		return [2]float64{sampleKey(c.X, t, 0), sampleKey(c.Y, t, 0)}
	}
	return [2]float64{}
}

func sampleAnimSprite(anim *kmdata.Anim, path string, t float64, fallback string) string {
	if anim == nil {
		return fallback
	}
	keys := anim.Sprites[path]
	if len(keys) == 0 {
		return fallback
	}
	name := keys[0].Name
	for _, k := range keys {
		if k.T > t {
			break
		}
		name = k.Name
	}
	if name == "" {
		return fallback
	}
	return name
}

func sampleKey(keys []kmdata.Key, t, fallback float64) float64 {
	if len(keys) == 0 {
		return fallback
	}
	if t <= keys[0].T {
		return keys[0].V
	}
	for i := 0; i < len(keys)-1; i++ {
		a, b := keys[i], keys[i+1]
		if t > b.T {
			continue
		}
		if math.Abs(a.O) >= kmdata.StepSlope || math.Abs(b.I) >= kmdata.StepSlope || b.T <= a.T {
			return a.V
		}
		u := (t - a.T) / (b.T - a.T)
		return a.V + (b.V-a.V)*u
	}
	return keys[len(keys)-1].V
}

func randRange(rng *uint64, lo, hi float64) float64 {
	return lo + (hi-lo)*rand01(rng)
}

func randIndex(rng *uint64, n int) int {
	if n <= 1 {
		return 0
	}
	i := int(rand01(rng) * float64(n))
	if i >= n {
		return n - 1
	}
	return i
}

func rand01(rng *uint64) float64 {
	x := *rng
	if x == 0 {
		x = 0x4a697252616c6c79
	}
	x ^= x << 13
	x ^= x >> 7
	x ^= x << 17
	*rng = x
	return float64(x&((1<<53)-1)) / float64(1<<53)
}

func white() [4]float64 { return [4]float64{1, 1, 1, 1} }

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func boolParam(e *riq.Entity, key string) bool {
	if v, ok := e.Data[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
		if f, ok := v.(float64); ok {
			return f != 0
		}
	}
	return false
}

type colorRGBA struct{ R, G, B, A uint8 }

func (c colorRGBA) RGBA() (r, g, b, a uint32) {
	r = uint32(c.R) * 0x101
	g = uint32(c.G) * 0x101
	b = uint32(c.B) * 0x101
	a = uint32(c.A) * 0x101
	return
}
