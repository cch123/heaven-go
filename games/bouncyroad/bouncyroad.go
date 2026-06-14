// Package bouncyroad ports Bouncy Road's ball chain, podium animation,
// gradient background, custom note pitches, and two-button input flow.
package bouncyroad

import (
	"image/color"
	"math"
	"sort"
	"strconv"

	"github.com/hajimehoshi/ebiten/v2"

	"hsdemo/engine"
	"hsdemo/kart"
	"hsdemo/kmdata"
	"hsdemo/riq"
)

const (
	rightAction = 3 // Heaven Studio IA_Alt / A-button pole.
	leftAction  = 1 // Heaven Studio D-pad pole.
	fallY       = -10.0
)

var (
	defaultTop = [4]float64{0.003921569, 0.59607846, 0.99607843, 1}
	defaultBot = [4]float64{0, 0, 0, 1}

	// Local Bezier handles serialized on BouncyRoad/curveBounce keypoints.
	// BouncyRoad.cs clones this template, rotates each point around Unity's Y
	// axis to face the next pole, scales local X by pole distance, then scales
	// local Y by the event length in GetHeightCurve.
	baseP0LH = [3]float64{0.25, -4.2, 0}
	baseP0RH = [3]float64{-0.25, 4.2, 0}
	baseP1LH = [3]float64{0.25, 4.2, 0}
	baseP1RH = [3]float64{0, 0, 0}
)

type ballEvt struct {
	beat, length              float64
	goal, useCustom, separate bool
	color                     [4]float64
	bounceNote, rightNote     int
	leftNote, goalNote        int
	separateBounceNotes       []int
}

type bgEvt struct {
	beat, length float64
	top0, top1   [4]float64
	bot0, bot1   [4]float64
	ease         int
}

type ball struct {
	module     *Module
	startBeat  float64
	lengthBeat float64
	goal       bool
	useCustom  bool
	color      [4]float64

	bounceNote int
	bounce     []int
	rightNote  int
	leftNote   int
	goalNote   int

	curves       []kmdata.Curve
	current      *kmdata.Curve
	override     kmdata.Curve
	currentBeat  float64
	curveDur     float64
	active, dead bool
	isMiss       bool
}

type Module struct {
	ctx  *engine.Ctx
	proj kart.Aff

	ballEvents []ballEvt
	bgEvents   []bgEvt
	balls      []*ball
	things     []string

	curveCache   map[float64][]kmdata.Curve
	bounceBeats  []float64
	lastGoalBeat float64

	ballSprite string
	ballScale  [2]float64
	ballOrder  int

	bgImg       *ebiten.Image
	bgPix       []byte
	bgTop, bgBt [4]float64
}

func New() engine.Module { return &Module{curveCache: map[float64][]kmdata.Curve{}} }

func (m *Module) ID() string { return "bouncyRoad" }

func (m *Module) Load(ctx *engine.Ctx) error {
	m.ctx = ctx
	if err := ctx.LoadAssets("bouncyRoad"); err != nil {
		return err
	}
	m.proj = kart.Translate(engine.ScreenW/2, engine.ScreenH/2).Mul(kart.Scale(42, -42))
	m.ctx.Scene.SetActive(m.ctx.Role("BGGradient"), false)
	m.ctx.Scene.SetActive(m.ctx.Role("BGHigh"), false)
	m.ctx.Scene.SetActive(m.ctx.Role("BGLow"), false)

	if idx, ok := ctx.Assets.NodeIndex(ctx.Role("baseBall")); ok {
		n := ctx.Assets.Rig.Nodes[idx]
		m.ballSprite = n.Sprite
		m.ballScale = n.Scale
		m.ballOrder = n.Order
	}
	if m.ballSprite == "" {
		m.ballSprite = "Ball"
		m.ballScale = [2]float64{0.75, 0.75}
		m.ballOrder = 1
	}

	if idx, ok := ctx.Assets.NodeIndex(ctx.Role("ThingsTrans")); ok {
		for i, n := range ctx.Assets.Rig.Nodes {
			if n.Parent == idx {
				m.things = append(m.things, ctx.Assets.Rig.Nodes[i].Path)
			}
		}
	}
	return nil
}

func (m *Module) OnEvent(e *riq.Entity) {
	switch e.Datamodel {
	case "bouncyRoad/ball":
		if e.Length == 0 {
			return
		}
		useCustom := boolParam(e, "useCustomNotes")
		separate := useCustom && boolParam(e, "separateBounceNotes")
		var notes []int
		if separate {
			notes = make([]int, 12)
			notes[0] = noteParam(e, "bounceNote", 0)
			for i := 1; i < 12; i++ {
				notes[i] = noteParam(e, "bounceNote"+strconv.Itoa(i+1), 0)
			}
		}
		m.ballEvents = append(m.ballEvents, ballEvt{
			beat: e.Beat, length: e.Length, goal: boolParamDefault(e, "goal", true),
			useCustom: useCustom, separate: separate, color: colorParam(e, "color", [4]float64{1, 1, 1, 1}),
			bounceNote: noteParam(e, "bounceNote", 0), rightNote: noteParam(e, "rightNote", 0),
			leftNote: noteParam(e, "leftNote", 0), goalNote: noteParam(e, "goalNote", 0),
			separateBounceNotes: notes,
		})
	case "bouncyRoad/background appearance":
		m.bgEvents = append(m.bgEvents, bgEvt{
			beat: e.Beat, length: e.Length, ease: int(e.Float("ease", 1)),
			top0: colorParam(e, "colorBG1Start", defaultTop),
			top1: colorParam(e, "colorBG1End", defaultTop),
			bot0: colorParam(e, "colorBG2Start", defaultBot),
			bot1: colorParam(e, "colorBG2End", defaultBot),
		})
	}
}

func (m *Module) Ready() {
	sort.SliceStable(m.ballEvents, func(i, j int) bool {
		return m.ballEvents[i].beat-m.ballEvents[i].length < m.ballEvents[j].beat-m.ballEvents[j].length
	})
	sort.SliceStable(m.bgEvents, func(i, j int) bool { return m.bgEvents[i].beat < m.bgEvents[j].beat })
	for _, ev := range m.ballEvents {
		m.scheduleBall(ev)
	}
}

func (m *Module) OnSwitch(beat float64) {}

func (m *Module) Whiff(beat float64) { m.WhiffAction(beat, 0) }

func (m *Module) WhiffAction(beat float64, action int) {
	switch action {
	case rightAction:
		m.playPodium(12, beat)
		m.ctx.SoundVol("rightBlank", 0.5)
	case leftAction:
		m.playPodium(13, beat)
		m.ctx.SoundVol("leftBlank", 0.5)
	}
}

func (m *Module) Update(t, beat float64) {
	alive := m.balls[:0]
	for _, b := range m.balls {
		if !b.dead || beat < b.startBeat+b.lengthBeat*16 {
			alive = append(alive, b)
		}
	}
	m.balls = alive
}

func (m *Module) Draw(screen *ebiten.Image, t, beat float64) {
	top, bot := m.bgAt(beat)
	m.drawGradient(screen, top, bot)

	sc := m.ctx.Scene
	m.ctx.SampleScene(beat)
	for _, b := range m.balls {
		if !b.active || b.dead || b.current == nil {
			continue
		}
		p := b.posAt(beat)
		w := kart.Translate(p[0], p[1]).Mul(kart.Scale(m.ballScale[0], m.ballScale[1]))
		sc.Queue(kart.ExtraSprite{
			Sprite: m.ballSprite, World: w, Z: p[2], Order: m.ballOrder, Tint: b.color,
		})
	}
	sc.Draw(screen, m.proj)
}

func (m *Module) scheduleBall(ev ballEvt) {
	b := &ball{
		module: m, startBeat: ev.beat, lengthBeat: ev.length, goal: ev.goal,
		useCustom: ev.useCustom, color: ev.color, bounceNote: ev.bounceNote,
		bounce: ev.separateBounceNotes, rightNote: ev.rightNote,
		leftNote: ev.leftNote, goalNote: ev.goalNote,
		curves: m.heightCurves(ev.length),
	}
	m.balls = append(m.balls, b)
	m.playBounceSound(b)

	m.ctx.At(b.startBeat-b.lengthBeat, func() { b.setCurve(0, b.startBeat-b.lengthBeat, b.lengthBeat) })
	for i := 0; i < 14; i++ {
		i := i
		beat := b.startBeat + float64(i)*b.lengthBeat
		m.ctx.At(beat, func() {
			if b.isMiss {
				return
			}
			if i < 12 {
				m.playPodium(i, beat)
			}
			b.setCurve(1+i, beat, b.lengthBeat)
		})
	}
	m.ctx.ScheduleInputAction(b.startBeat+11*b.lengthBeat, rightAction,
		func(state float64, _ engine.Judgment) { b.rightSuccess() },
		func() { b.rightMiss() })
}

func (b *ball) rightSuccess() {
	now := b.module.ctx.Beat()
	b.module.ctx.SoundPitch("ballRight", 1, b.pitch(b.rightNote))
	b.module.playPodium(12, now)
	b.module.ctx.ScheduleInputAction(b.startBeat+12*b.lengthBeat, leftAction,
		func(state float64, _ engine.Judgment) { b.leftSuccess() },
		func() { b.leftMiss() })
}

func (b *ball) rightMiss() {
	now := b.module.ctx.Beat()
	b.module.ctx.SoundPitch("ballBounce", 1, b.pitch(b.bounceNote))
	b.missWithCurve(len(b.curves)-2, now)
}

func (b *ball) leftSuccess() {
	now := b.module.ctx.Beat()
	b.module.ctx.SoundPitch("ballLeft", 1, b.pitch(b.leftNote))
	b.module.playPodium(13, now)

	goalBeat := b.startBeat + 14*b.lengthBeat
	if math.Abs(b.module.lastGoalBeat-goalBeat) <= 0.001 {
		b.goal = false
	} else {
		b.module.lastGoalBeat = goalBeat
	}
	if b.goal {
		b.module.ctx.SoundAtPitchPan(goalBeat, "goal", 1, b.pitch(b.goalNote), 0)
	}
	b.module.ctx.At(goalBeat, func() {
		b.module.playPodium(14, goalBeat)
		b.setCurve(15, goalBeat, b.lengthBeat)
	})
	b.module.ctx.At(b.startBeat+15*b.lengthBeat, func() { b.dead = true })
}

func (b *ball) leftMiss() {
	now := b.module.ctx.Beat()
	b.module.ctx.SoundPitch("ballBounce", 1, b.pitch(b.bounceNote))
	b.missWithCurve(len(b.curves)-1, now)
}

func (b *ball) missWithCurve(idx int, beat float64) {
	pos := b.posAt(beat)
	b.override = b.curves[idx]
	movePoint(&b.override.Points[0], pos)
	b.current = &b.override
	b.currentBeat = beat
	b.curveDur = b.lengthBeat / 2
	b.isMiss = true
	b.active = true
	b.module.ctx.At(beat+b.lengthBeat/2, func() { b.dead = true })
}

func (b *ball) setCurve(idx int, beat, dur float64) {
	if idx < 0 || idx >= len(b.curves) || b.dead {
		return
	}
	b.current = &b.curves[idx]
	b.currentBeat = beat
	b.curveDur = dur
	b.active = true
}

func (b *ball) posAt(beat float64) [3]float64 {
	if b.current == nil {
		return [3]float64{}
	}
	dur := b.curveDur
	if dur <= 0 {
		dur = b.lengthBeat
	}
	return kart.EvalBezier(*b.current, clamp01((beat-b.currentBeat)/dur))
}

func (b *ball) pitch(note int) float64 {
	if !b.useCustom {
		return 1
	}
	return math.Exp2(float64(note) / 12)
}

func (m *Module) playBounceSound(b *ball) {
	volume := 0.5
	pan := -0.5
	for i := 0; i < 12; i++ {
		if i >= 4 {
			volume += 0.0625
		}
		if i >= 9 {
			pan -= 0.16666
		} else {
			pan += 0.11111
		}
		bounceBeat := b.startBeat + float64(i)*b.lengthBeat
		soundBeat := bounceBeat
		usedVolume := volume
		if containsBeat(m.bounceBeats, bounceBeat) {
			usedVolume = volume * 0.5
			soundBeat += 0.025
			if containsBeat(m.bounceBeats, soundBeat) {
				soundBeat -= 0.05
			}
		}
		pitch := b.pitch(b.bounceNote)
		if len(b.bounce) == 12 {
			pitch = b.pitch(b.bounce[i])
		}
		m.ctx.SoundAtPitchPan(soundBeat, "ballBounce", usedVolume, pitch, pan)
		m.bounceBeats = append(m.bounceBeats, bounceBeat)
	}
}

func (m *Module) playPodium(i int, beat float64) {
	if i < 0 || i >= len(m.things) {
		return
	}
	m.ctx.Scene.PlayState(m.things[i], "podium", beat, m.ctx.SecPerBeat(beat))
}

func (m *Module) heightCurves(length float64) []kmdata.Curve {
	if c, ok := m.curveCache[length]; ok {
		return c
	}
	n := len(m.things)
	if n < 2 {
		return nil
	}
	posCurve := m.ctx.Assets.Extra.Curves["PosCurve"]
	curves := make([]kmdata.Curve, 0, n+3)
	curves = append(curves, m.generateCurve(evalBezierLoose(posCurve, -1/float64(n-1)), evalBezierLoose(posCurve, 0), length))
	for i := 0; i < n-1; i++ {
		curves = append(curves, m.generateCurve(m.nodePos(m.things[i]), m.nodePos(m.things[i+1]), length))
	}
	curves = append(curves, m.generateCurve(evalBezierLoose(posCurve, 1), evalBezierLoose(posCurve, float64(n)/float64(n-1)), length))
	curves = append(curves, missCurve(curves[13]))
	curves = append(curves, missCurve(curves[14]))
	m.curveCache[length] = curves
	return curves
}

func (m *Module) generateCurve(p0, p1 [3]float64, length float64) kmdata.Curve {
	dist := distance3(p0, p1)
	angle := math.Atan2(p0[2]-p1[2], p0[0]-p1[0])
	rotY := -angle
	return kmdata.Curve{
		Sampling: m.ctx.Assets.Extra.Curves["baseBounceCurve"].Sampling,
		Points: []kmdata.CurvePoint{
			{P: p0, LH: transformHandle(p0, baseP0LH, rotY, dist, length), RH: transformHandle(p0, baseP0RH, rotY, dist, length)},
			{P: p1, LH: transformHandle(p1, baseP1LH, rotY, dist, length), RH: transformHandle(p1, baseP1RH, rotY, dist, length)},
		},
	}
}

func (m *Module) nodePos(path string) [3]float64 {
	if idx, ok := m.ctx.Assets.NodeIndex(path); ok {
		n := m.ctx.Assets.Rig.Nodes[idx]
		return [3]float64{n.Pos[0], n.Pos[1], n.PosZ}
	}
	return [3]float64{}
}

func (m *Module) bgAt(beat float64) ([4]float64, [4]float64) {
	top, bot := defaultTop, defaultBot
	for _, ev := range m.bgEvents {
		if beat < ev.beat {
			break
		}
		if ev.length > 0 && beat < ev.beat+ev.length {
			u := clamp01((beat - ev.beat) / ev.length)
			return easeColor(ev.ease, ev.top0, ev.top1, u), easeColor(ev.ease, ev.bot0, ev.bot1, u)
		}
		top, bot = ev.top1, ev.bot1
	}
	return top, bot
}

func missCurve(c kmdata.Curve) kmdata.Curve {
	out := c
	out.Points = append([]kmdata.CurvePoint(nil), c.Points...)
	movePoint(&out.Points[1], [3]float64{out.Points[1].P[0], out.Points[1].P[1] + fallY, out.Points[1].P[2]})
	return out
}

func movePoint(p *kmdata.CurvePoint, to [3]float64) {
	d := [3]float64{to[0] - p.P[0], to[1] - p.P[1], to[2] - p.P[2]}
	for i := 0; i < 3; i++ {
		p.P[i] += d[i]
		p.LH[i] += d[i]
		p.RH[i] += d[i]
	}
}

func transformHandle(origin, local [3]float64, rotY, dist, heightScale float64) [3]float64 {
	x, y, z := local[0]*dist, local[1]*heightScale, local[2]
	sin, cos := math.Sin(rotY), math.Cos(rotY)
	return [3]float64{
		origin[0] + cos*x + sin*z,
		origin[1] + y,
		origin[2] - sin*x + cos*z,
	}
}

func evalBezierLoose(c kmdata.Curve, t float64) [3]float64 {
	pts := c.Points
	n := len(pts)
	switch n {
	case 0:
		return [3]float64{}
	case 1:
		return pts[0].P
	}
	sampling := c.Sampling
	if sampling <= 0 {
		sampling = 25
	}
	sub := sampling/(n-1) + 1
	segs := make([]float64, n-1)
	total := 0.0
	for i := range segs {
		segs[i] = segLen(pts[i], pts[i+1], sub)
		total += segs[i]
	}
	if total <= 0 {
		return pts[n-1].P
	}
	totalPercent := 0.0
	seg := -1
	subPercent := 0.0
	for i := 0; i < n-1; i++ {
		subPercent = segs[i] / total
		if subPercent+totalPercent > t {
			seg = i
			break
		}
		totalPercent += subPercent
	}
	if seg < 0 {
		seg = n - 2
		subPercent = segs[seg] / total
		totalPercent -= subPercent
	}
	return cubic((t-totalPercent)/subPercent, pts[seg].P, pts[seg].RH, pts[seg+1].LH, pts[seg+1].P)
}

func segLen(a, b kmdata.CurvePoint, sampling int) float64 {
	prev := cubic(0, a.P, a.RH, b.LH, b.P)
	total := 0.0
	for i := 0; i < sampling; i++ {
		p := cubic(float64(i+1)/float64(sampling), a.P, a.RH, b.LH, b.P)
		total += distance3(prev, p)
		prev = p
	}
	return total
}

func cubic(t float64, p0, c0, c1, p1 [3]float64) [3]float64 {
	u := 1 - t
	a, b, c, d := u*u*u, 3*u*u*t, 3*u*t*t, t*t*t
	return [3]float64{
		a*p0[0] + b*c0[0] + c*c1[0] + d*p1[0],
		a*p0[1] + b*c0[1] + c*c1[1] + d*p1[1],
		a*p0[2] + b*c0[2] + c*c1[2] + d*p1[2],
	}
}

func distance3(a, b [3]float64) float64 {
	dx, dy, dz := a[0]-b[0], a[1]-b[1], a[2]-b[2]
	return math.Sqrt(dx*dx + dy*dy + dz*dz)
}

func (m *Module) drawGradient(screen *ebiten.Image, top, bot [4]float64) {
	h := screen.Bounds().Dy()
	if m.bgImg == nil || m.bgImg.Bounds().Dy() != h {
		m.bgImg = ebiten.NewImage(1, h)
		m.bgPix = make([]byte, h*4)
		m.bgTop, m.bgBt = [4]float64{-1, -1, -1, -1}, [4]float64{-1, -1, -1, -1}
	}
	if m.bgTop != top || m.bgBt != bot {
		m.bgTop, m.bgBt = top, bot
		for y := 0; y < h; y++ {
			u := float64(y) / float64(max(1, h-1))
			c := [4]float64{
				top[0]*(1-u) + bot[0]*u,
				top[1]*(1-u) + bot[1]*u,
				top[2]*(1-u) + bot[2]*u,
				top[3]*(1-u) + bot[3]*u,
			}
			r := rgba(c)
			i := y * 4
			m.bgPix[i+0] = r.R
			m.bgPix[i+1] = r.G
			m.bgPix[i+2] = r.B
			m.bgPix[i+3] = r.A
		}
		m.bgImg.WritePixels(m.bgPix)
	}
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(float64(screen.Bounds().Dx()), 1)
	screen.DrawImage(m.bgImg, op)
}

func easeColor(ease int, a, b [4]float64, u float64) [4]float64 {
	return [4]float64{
		engine.Ease(ease, a[0], b[0], u),
		engine.Ease(ease, a[1], b[1], u),
		engine.Ease(ease, a[2], b[2], u),
		engine.Ease(ease, a[3], b[3], u),
	}
}

func rgba(c [4]float64) color.RGBA {
	return color.RGBA{R: byte(clamp01(c[0]) * 255), G: byte(clamp01(c[1]) * 255), B: byte(clamp01(c[2]) * 255), A: byte(clamp01(c[3]) * 255)}
}

func boolParam(e *riq.Entity, key string) bool { return e.Float(key, 0) != 0 }

func boolParamDefault(e *riq.Entity, key string, def bool) bool {
	if _, ok := e.Data[key]; !ok {
		return def
	}
	return boolParam(e, key)
}

func colorParam(e *riq.Entity, key string, def [4]float64) [4]float64 {
	if mm, ok := e.Data[key].(map[string]any); ok {
		return [4]float64{num(mm["r"], def[0]), num(mm["g"], def[1]), num(mm["b"], def[2]), num(mm["a"], def[3])}
	}
	return def
}

func noteParam(e *riq.Entity, key string, def int) int {
	return int(math.Round(e.Float(key, float64(def))))
}

func num(v any, def float64) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case int:
		return float64(x)
	}
	return def
}

func containsBeat(xs []float64, beat float64) bool {
	for _, x := range xs {
		if math.Abs(x-beat) < 1e-9 {
			return true
		}
	}
	return false
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
