// Package airrally ports the Air Rally subset used by Character Select.
//
// Covered events: rally, ba bum bum bum, catch, set distance, forward,
// 4beat, forthington voice lines, and rainbow. The full environmental systems
// (cloud/tree/snow density, bird prefabs, day-night material swaps) are still
// listed as known simplifications in README.
package airrally

import (
	"math"
	"sort"

	"github.com/hajimehoshi/ebiten/v2"

	"hsdemo/engine"
	"hsdemo/kart"
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

type distanceEvt struct {
	beat, length float64
	typ, ease    int
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

type Module struct {
	ctx  *engine.Ctx
	proj kart.Aff

	distances []distanceEvt
	rallies   map[float64]bool
	baBums    map[float64]baBumEvt
	catches   map[float64]bool
	silences  []interval
	rainbows  []rainbowEvt

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
	case "airRally/forthington voice lines":
		m.scheduleVoice(b, int(e.Float("type", 0)))
	case "airRally/rainbow":
		m.rainbows = append(m.rainbows, rainbowEvt{beat: b, speed: e.Float("speed", 1), start: e.Float("start", 100)})
	}
}

func (m *Module) Ready() {
	sort.Slice(m.distances, func(i, j int) bool { return m.distances[i].beat < m.distances[j].beat })
	sort.Slice(m.silences, func(i, j int) bool { return m.silences[i].beat < m.silences[j].beat })
	for b := range m.rallies {
		m.scheduleRally(b)
	}
	for b, ev := range m.baBums {
		m.scheduleBaBum(b, ev.count, ev.alt)
	}
}

func (m *Module) OnSwitch(beat float64) {
	m.ctx.Scene.PlayState(m.ctx.Role("Baxter"), "Idle", beat, m.ctx.SecPerBeat(beat))
	m.ctx.Scene.PlayState(m.ctx.Role("Forthington"), "Idle", beat, m.ctx.SecPerBeat(beat))
}

func (m *Module) Whiff(beat float64) {
	m.ctx.Scene.PlayState(m.ctx.Role("Baxter"), "Hit", beat, 0.5)
	m.ctx.Sound("swing")
}

func (m *Module) Update(t, beat float64) {}

func (m *Module) Draw(screen *ebiten.Image, t, beat float64) {
	screen.Fill(colorRGBA{181, 255, 255, 255})
	sc := m.ctx.Scene
	sc.SetZOver(m.ctx.Role("Forthington"), m.forthZAt(beat))
	m.ctx.SampleScene(beat)
	m.queueRainbow(sc, beat)
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
	cur := distZ[distClose]
	for _, e := range m.distances {
		if beat < e.beat {
			break
		}
		target := distZ[clampDist(e.typ)]
		if e.length > 0 && beat < e.beat+e.length {
			u := (beat - e.beat) / e.length
			return engine.Ease(e.ease, cur, target, u)
		}
		cur = target
	}
	return cur
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
	forthZ, baxterZ := distZ[clampDist(m.shuttle.dist)], distZ[distClose]
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

func (m *Module) queueRainbow(sc *kart.SceneInst, beat float64) {
	for _, r := range m.rainbows {
		if beat < r.beat || beat > r.beat+12 {
			continue
		}
		age := beat - r.beat
		alpha := math.Min(age, 1)
		z := r.start - age*r.speed*20
		sc.Queue(kart.ExtraSprite{
			Sprite: "rainbow",
			World:  kart.Translate(0, 2).Mul(kart.Scale(2.4, 2.4)),
			Z:      z,
			Order:  -80,
			Tint:   [4]float64{1, 1, 1, alpha},
		})
	}
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
