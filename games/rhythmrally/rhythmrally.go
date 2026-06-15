// Package rhythmrally ports Rhythm Rally's cue timing, rally speeds, ball
// Bezier travel, core sound timing, background toggle, and player input flow.
//
// Heaven Studio renders this game with imported 3D models. The current Go
// renderer only consumes the extracted 2D scene and sprite metadata, so this
// module uses the official curves and sounds with a lightweight 2D table and
// paddler representation until the model renderer is ported.
package rhythmrally

import (
	"image/color"
	"math"
	"sort"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"hsdemo/engine"
	"hsdemo/kart"
	"hsdemo/kmdata"
	"hsdemo/riq"
)

const (
	speedSlow = iota
	speedNormal
	speedFast
	speedSuperFast
)

const (
	tableHitTime = 0.58
	ballAction   = 0
)

var (
	rallyBG      = color.RGBA{R: 0xf4, G: 0x7d, B: 0xc0, A: 0xff}
	voidBG       = color.RGBA{R: 0xf7, G: 0xf7, B: 0xf7, A: 0xff}
	tableColor   = color.RGBA{R: 0xc7, G: 0x13, B: 0x24, A: 0xff}
	tableLine    = color.RGBA{R: 0xff, G: 0xf0, B: 0xf0, A: 0xff}
	paddlerColor = color.RGBA{R: 0xc9, G: 0x00, B: 0x13, A: 0xff}
	playerColor  = color.RGBA{R: 0x21, G: 0x40, B: 0xc9, A: 0xff}
	ballColor    = color.RGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
	shadowColor  = color.RGBA{R: 0x20, G: 0x00, B: 0x20, A: 0x55}
)

type serveEvt struct {
	beat  float64
	speed int
}

type tossEvt struct {
	beat, length float64
	first        bool
}

type bgEvt struct {
	beat, length float64
	void         bool
	start, end   [4]float64
	ease         int
}

type camEvt struct {
	beat, length float64
	rot, zoom    float64
	additive     bool
	ease         int
}

type ballState struct {
	started, served, missed bool
	tossing                 bool
	ballActive, inPose      bool
	serveBeat, targetBeat   float64
	tossBeat, tossLength    float64
	missBeat, missSide      float64
	speed                   int
}

type hitFx struct {
	beat float64
	z    float64
}

type Module struct {
	ctx  *engine.Ctx
	proj kart.Aff

	curves map[string]kmdata.Curve

	serves []serveEvt
	tosses []tossEvt
	bgs    []bgEvt
	cams   []camEvt

	ball ballState
	fxs  []hitFx

	bgColor [4]float64
	bgVoid  bool

	playerState   string
	opponentState string
}

func New() engine.Module { return &Module{} }

func (m *Module) ID() string { return "rhythmRally" }

func (m *Module) Load(ctx *engine.Ctx) error {
	m.ctx = ctx
	if err := ctx.LoadAssets("rhythmRally"); err != nil {
		return err
	}
	m.proj = kart.Translate(engine.ScreenW/2, engine.ScreenH/2+60).Mul(kart.Scale(130, -130))
	m.curves = ctx.Assets.Extra.Curves
	m.ball.speed = speedNormal
	m.ball.missSide = 1
	m.ball.ballActive = false
	m.bgColor = [4]float64{1, 1, 1, 1}
	m.playerState, m.opponentState = "Idle", "Idle"
	return nil
}

func (m *Module) OnEvent(e *riq.Entity) {
	b := e.Beat
	switch e.Datamodel {
	case "rhythmRally/bop":
		bop, auto := boolParamDefault(e, "bop", true), boolParam(e, "bopAuto")
		if bop || auto {
			for i := 0; i < int(math.Ceil(e.Length)); i++ {
				beat := b + float64(i)
				m.ctx.At(beat, func() { m.bop(beat) })
			}
		}
	case "rhythmRally/whistle":
		m.ctx.SoundAt(b, "Whistle", 1)
	case "rhythmRally/toss ball":
		ev := tossEvt{beat: b, length: e.Length, first: true}
		m.tosses = append(m.tosses, ev)
		m.ctx.At(b, func() { m.toss(ev, 6) })
	case "rhythmRally/rally":
		m.scheduleRally(b, e.Length, speedNormal)
	case "rhythmRally/slow rally":
		m.scheduleRally(b, e.Length, speedSlow)
	case "rhythmRally/fast rally":
		m.prepareFastRally(b, speedFast, boolParam(e, "muteAudio"))
	case "rhythmRally/superfast rally":
		m.prepareFastRally(b, speedSuperFast, boolParam(e, "muteAudio"))
	case "rhythmRally/tonktinktonk":
		m.tonkTinkTonk(b, e.Length)
	case "rhythmRally/superfast stretchable":
		m.superFastStretchable(b, e.Length)
	case "rhythmRally/pose":
		remove := boolParam(e, "remove")
		m.ctx.At(b, func() { m.pose(remove) })
	case "rhythmRally/camera":
		m.cams = append(m.cams, camEvt{
			beat: b, length: e.Length, rot: e.Float("valA", 0), zoom: e.Float("valB", 1),
			ease: int(e.Float("type", 0)), additive: boolParamDefault(e, "additive", true),
		})
	case "rhythmRally/bg":
		ev := bgEvt{
			beat: b, length: e.Length, void: boolParamDefault(e, "void", true),
			start: colorParam(e, "start", [4]float64{1, 1, 1, 1}),
			end:   colorParam(e, "end", [4]float64{1, 1, 1, 1}),
			ease:  int(e.Float("ease", 0)),
		}
		m.bgs = append(m.bgs, ev)
		m.ctx.At(b, func() { m.applyBG(ev) })
	}
}

func (m *Module) Ready() {
	sort.Slice(m.bgs, func(i, j int) bool { return m.bgs[i].beat < m.bgs[j].beat })
	sort.Slice(m.cams, func(i, j int) bool { return m.cams[i].beat < m.cams[j].beat })
}

func (m *Module) OnSwitch(beat float64) {
	m.playerState, m.opponentState = "Idle", "Idle"
	m.bgColor = [4]float64{1, 1, 1, 1}
	for _, ev := range m.bgs {
		if ev.beat <= beat {
			m.applyBG(ev)
		}
	}
}

func (m *Module) Whiff(beat float64) {
	m.playerState = "Swing"
}

func (m *Module) Update(_, beat float64) {
	if len(m.fxs) > 0 {
		alive := m.fxs[:0]
		for _, fx := range m.fxs {
			if beat-fx.beat < 1.25 {
				alive = append(alive, fx)
			}
		}
		m.fxs = alive
	}
}

func (m *Module) Draw(screen *ebiten.Image, _, beat float64) {
	screen.Fill(m.backgroundAt(beat))
	drawTable(screen)
	m.drawPaddler(screen, false, beat)
	m.drawPaddler(screen, true, beat)
	if m.ball.ballActive {
		p := m.ballPosition(beat)
		m.drawBall(screen, p)
	}
	for _, fx := range m.fxs {
		m.drawBounceFX(screen, fx, beat)
	}
}

func (m *Module) scheduleServe(ev serveEvt) {
	m.serves = append(m.serves, ev)
	m.ctx.At(ev.beat, func() { m.serve(ev.beat, ev.speed) })
}

func (m *Module) scheduleRally(beat, length float64, speed int) {
	interval := rallyInterval(speed)
	if length <= 0 {
		length = interval
	}
	for off := 0.0; off < length; off += interval {
		m.scheduleServe(serveEvt{beat: beat + off, speed: speed})
	}
}

func (m *Module) serve(beat float64, speed int) {
	target, bounceOffset := targetAndBounce(speed)
	m.ball = ballState{
		started: true, served: true, ballActive: true, serveBeat: beat,
		targetBeat: target, speed: speed, missSide: 1,
	}
	m.opponentState = "Swing"
	m.ctx.SoundAt(beat, "Serve", 1)
	m.ctx.SoundAt(beat+bounceOffset, "ServeBounce", 1)
	m.ctx.At(beat+bounceOffset, func() { m.bounceFX(beat + bounceOffset) })
	m.ctx.ScheduleInputAction(beat+target, ballAction, func(state float64, _ engine.Judgment) {
		if state >= 1 || state <= -1 {
			m.nearMiss(state)
			return
		}
		m.ace()
	}, func() { m.missBall() })
}

func (m *Module) ace() {
	m.ball.served = false
	m.ball.missed = false
	m.ball.ballActive = true
	m.playerState = "Swing"
	bounce := m.returnBounceBeat()
	m.ctx.Sound("Return")
	m.ctx.SoundAt(bounce, "ReturnBounce", 1)
	m.ctx.At(bounce, func() { m.bounceFX(bounce) })
}

func (m *Module) nearMiss(state float64) {
	m.playerState = "Swing"
	m.ball.served = false
	m.ball.missed = true
	m.ball.tossing = false
	m.ball.missBeat = m.ctx.Beat()
	m.ball.missSide = -state
	m.ball.ballActive = true
	m.ctx.PlayCommon("miss")
	m.missBall()
}

func (m *Module) missBall() {
	m.ball.served = false
	m.ball.missed = true
	whistle := m.returnBounceBeat()
	m.ctx.SoundAt(whistle, "Whistle", 1)
}

func (m *Module) returnBounceBeat() float64 {
	target := m.ball.serveBeat + m.ball.targetBeat + 1
	switch m.ball.speed {
	case speedSlow:
		target = m.ball.serveBeat + m.ball.targetBeat + 2
	case speedSuperFast:
		target = m.ball.serveBeat + m.ball.targetBeat + 0.5
	}
	return target
}

func (m *Module) toss(ev tossEvt, height float64) {
	if ev.length <= 0 {
		ev.length = 1
	}
	if ev.first {
		height *= ev.length / 2
	}
	m.ball.tossing = true
	m.ball.started = false
	m.ball.ballActive = true
	m.ball.tossBeat = ev.beat
	m.ball.tossLength = ev.length
	if ev.first {
		m.opponentState = "Ready1"
	}
}

func (m *Module) pose(remove bool) {
	m.playerState, m.opponentState = "Pose", "Pose"
	m.ball.inPose = true
	m.ball.ballActive = !remove
}

func (m *Module) bop(beat float64) {
	if m.ball.inPose {
		return
	}
	m.playerState, m.opponentState = "Beat", "Beat"
}

func (m *Module) prepareFastRally(beat float64, speed int, mute bool) {
	switch speed {
	case speedFast:
		m.scheduleServe(serveEvt{beat: beat + 2, speed: speedFast})
		if !mute {
			m.tonkTinkTonk(beat, 1.5)
		}
	case speedSuperFast:
		m.superFastStretchable(beat+4, 8)
		if !mute {
			m.tonkTinkTonk(beat, 4)
		}
	}
}

func (m *Module) superFastStretchable(beat, length float64) {
	for i := 0.0; i < length; i += 2 {
		m.scheduleServe(serveEvt{beat: beat + i, speed: speedSuperFast})
	}
}

func (m *Module) tonkTinkTonk(beat, length float64) {
	tink := false
	for off := 0.0; off < length; off += 0.5 {
		name := "Tonk"
		if tink {
			name = "Tink"
		}
		tink = !tink
		m.ctx.SoundAt(beat+off, name, 1)
	}
}

func (m *Module) bounceFX(beat float64) {
	z := m.ballPosition(beat)[2]
	m.fxs = append(m.fxs, hitFx{beat: beat, z: z})
}

func (m *Module) applyBG(ev bgEvt) {
	m.bgVoid = ev.void
	m.bgColor = ev.end
}

func (m *Module) backgroundAt(beat float64) color.RGBA {
	base := rallyBG
	if m.bgVoid {
		base = voidBG
	}
	col := m.bgColorAt(beat)
	return color.RGBA{
		R: uint8(clamp255(float64(base.R) * col[0])),
		G: uint8(clamp255(float64(base.G) * col[1])),
		B: uint8(clamp255(float64(base.B) * col[2])),
		A: 0xff,
	}
}

func (m *Module) bgColorAt(beat float64) [4]float64 {
	out := m.bgColor
	for _, ev := range m.bgs {
		if beat < ev.beat {
			continue
		}
		if ev.length <= 0 || beat >= ev.beat+ev.length {
			out = ev.end
			continue
		}
		u := clamp01((beat - ev.beat) / ev.length)
		out = lerpColor(ev.start, ev.end, ease01(u, ev.ease))
	}
	return out
}

func (m *Module) ballPosition(beat float64) [3]float64 {
	b := m.ball
	switch {
	case !b.started && b.tossing:
		return curvePoint(m.curves["tossCurve"], clamp01((beat-b.tossBeat)/max(0.0001, b.tossLength)), 1)
	case b.missed && b.tossing:
		return curvePoint(m.curves["tossCurve"], clamp01((beat-b.tossBeat)/max(0.0001, b.tossLength)), 1)
	case b.missed:
		p := curvePoint(m.curves["missCurve"], clamp01(beat-b.missBeat), 1)
		p[0] *= b.missSide
		return p
	case b.started:
		hitBeat, d1, d2 := m.flightTiming()
		u1 := (beat - hitBeat) / d1
		var pos float64
		if u1 >= 1 {
			u2 := (beat - hitBeat - d1) / d2
			pos = tableHitTime + u2*(1-tableHitTime)
		} else {
			pos = u1 * tableHitTime
		}
		height := m.flightHeight(b.speed, b.served, u1 >= 1)
		curve := m.curves["returnCurve"]
		if b.served {
			curve = m.curves["serveCurve"]
		}
		p := curvePoint(curve, math.Max(0, pos), height)
		if pos > 1.05 {
			m.ball.ballActive = false
		}
		return p
	default:
		return [3]float64{0.12, 0.74, 2.08}
	}
}

func (m *Module) flightTiming() (hitBeat, d1, d2 float64) {
	hitBeat, d1, d2 = m.ball.serveBeat, 1, 1
	switch m.ball.speed {
	case speedNormal:
		if !m.ball.served {
			hitBeat = m.ball.serveBeat + 2
		}
	case speedFast:
		if !m.ball.served {
			hitBeat, d1, d2 = m.ball.serveBeat+1, 1, 2
		} else {
			d1, d2 = 0.5, 0.5
		}
	case speedSuperFast:
		if !m.ball.served {
			hitBeat = m.ball.serveBeat + 1
		}
		d1, d2 = 0.5, 0.5
	case speedSlow:
		if !m.ball.served {
			hitBeat = m.ball.serveBeat + 4
		}
		d1, d2 = 2, 2
	}
	return hitBeat, d1, d2
}

func (m *Module) flightHeight(speed int, served, secondLeg bool) float64 {
	switch {
	case (speed == speedFast && served) || speed == speedSuperFast:
		return 0.75
	case speed == speedFast && !served && secondLeg:
		return 2
	case speed == speedSlow:
		return 3
	default:
		return 1.25
	}
}

func targetAndBounce(speed int) (target, bounce float64) {
	switch speed {
	case speedSlow:
		return 4, 2
	case speedFast, speedSuperFast:
		return 1, 0.5
	default:
		return 2, 1
	}
}

func rallyInterval(speed int) float64 {
	switch speed {
	case speedSlow:
		return 8
	case speedSuperFast:
		return 2
	default:
		return 4
	}
}

func curvePoint(c kmdata.Curve, u, height float64) [3]float64 {
	p := kart.EvalBezier(c, clamp01(u))
	p[1] = (p[1]-0.712)*height + 0.712
	return p
}

func (m *Module) drawBall(screen *ebiten.Image, p [3]float64) {
	x, y, scale := projectRhythmPoint(p)
	vector.DrawFilledCircle(screen, float32(x), float32(projectRhythmY(-0.399, p[2])), float32(12*scale), shadowColor, true)
	vector.DrawFilledCircle(screen, float32(x), float32(y), float32(8*scale), ballColor, true)
	vector.StrokeCircle(screen, float32(x), float32(y), float32(8*scale), 1.5, color.RGBA{R: 0xdd, G: 0xdd, B: 0xee, A: 0xff}, true)
}

func (m *Module) drawBounceFX(screen *ebiten.Image, fx hitFx, beat float64) {
	u := clamp01((beat - fx.beat) / 0.65)
	x, y, scale := projectRhythmPoint([3]float64{0, 0.2, fx.z})
	alpha := uint8((1 - u) * 180)
	vector.StrokeCircle(screen, float32(x), float32(y), float32((24+40*u)*scale), 3, color.RGBA{R: 0xff, G: 0xff, B: 0xff, A: alpha}, true)
}

func (m *Module) drawPaddler(screen *ebiten.Image, player bool, beat float64) {
	x, y, sc := projectRhythmPoint([3]float64{-0.18, 0.2, -2.35})
	col := playerColor
	state := m.playerState
	if !player {
		x, y, sc = projectRhythmPoint([3]float64{0.18, 0.62, 2.35})
		col = paddlerColor
		state = m.opponentState
	}
	r := float32(30 * sc)
	if state == "Swing" {
		r *= 1.12
	}
	vector.DrawFilledCircle(screen, float32(x), float32(y), r, col, true)
	vector.DrawFilledRect(screen, float32(x)-r*0.2, float32(y)-r*0.2, r*1.4, r*0.28, col, true)
}

func drawTable(screen *ebiten.Image) {
	cx, cy := float32(engine.ScreenW/2), float32(engine.ScreenH/2+76)
	w, h := float32(440), float32(126)
	vector.DrawFilledRect(screen, cx-w/2, cy-h/2, w, h, tableColor, true)
	vector.StrokeRect(screen, cx-w/2, cy-h/2, w, h, 4, tableLine, true)
	vector.StrokeLine(screen, cx, cy-h/2, cx, cy+h/2, 3, tableLine, true)
}

func projectRhythmPoint(p [3]float64) (float64, float64, float64) {
	zScale := 1 - p[2]*0.08
	if zScale < 0.62 {
		zScale = 0.62
	}
	if zScale > 1.55 {
		zScale = 1.55
	}
	return engine.ScreenW/2 + p[0]*360*zScale, projectRhythmY(p[1], p[2]), zScale
}

func projectRhythmY(y, z float64) float64 {
	zScale := 1 - z*0.08
	if zScale < 0.62 {
		zScale = 0.62
	}
	if zScale > 1.55 {
		zScale = 1.55
	}
	return engine.ScreenH/2 + 112 - y*155*zScale + z*8
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
	switch c := v.(type) {
	case []any:
		if len(c) >= 4 {
			return [4]float64{num(c[0], def[0]), num(c[1], def[1]), num(c[2], def[2]), num(c[3], def[3])}
		}
	case []float64:
		if len(c) >= 4 {
			return [4]float64{c[0], c[1], c[2], c[3]}
		}
	case map[string]any:
		return [4]float64{num(c["r"], def[0]), num(c["g"], def[1]), num(c["b"], def[2]), num(c["a"], def[3])}
	}
	return def
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

func lerpColor(a, b [4]float64, t float64) [4]float64 {
	return [4]float64{
		a[0] + (b[0]-a[0])*t,
		a[1] + (b[1]-a[1])*t,
		a[2] + (b[2]-a[2])*t,
		a[3] + (b[3]-a[3])*t,
	}
}

func ease01(u float64, ease int) float64 {
	u = clamp01(u)
	switch ease {
	case 1:
		return u
	default:
		return u
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

func clamp255(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return v
}
