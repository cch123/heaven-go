// Package karateman ports the Pack-In Karate Man cue path onto engine.App.
//
// The module uses the legacy single-rig Karate Man extraction: Joe is rendered
// through kart.RigInst while tossed objects are driven by the original
// KarateManPot.ProgressToFlyPosition stage parameters.
package karateman

import (
	"bytes"
	"fmt"
	"image/color"
	"math"
	"sort"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"golang.org/x/image/font/gofont/goregular"

	"hsdemo/engine"
	"hsdemo/kart"
	"hsdemo/kmdata"
	"hsdemo/riq"
)

const (
	joeScreenX = 310.0
	groundY    = 470.0
	rigFitH    = 330.0

	inSpinDeg = 125.0
	outSpin   = -9.0
	camD      = 10.0

	pitchMod = 125.0
)

const (
	hitPot = iota
	hitLightbulb
	hitRock
	hitBall
	hitCooking = 6
	hitAlien   = 7
	hitBomb    = 8
	hitTaco    = 999
)

type potResult int

const (
	potFlying potResult = iota
	potHit
	potNG
	potMiss
)

type pot struct {
	throwBeat float64
	hitBeat   float64
	rot0      float64
	typ       int
	sprite    string
	hitSound  string
	heavy     bool
	result    potResult
	judgeT    float64
}

type bopMarker struct {
	beat float64
	auto bool
}

type bgEvt struct {
	beat, length       float64
	bg0, bg1           [4]float64
	shadow0, shadow1   [4]float64
	texture0, texture1 [4]float64
	fx0, fx1           [4]float64
	ease               int
	textureType        int
	fxType             int
}

type wordEvt struct {
	beat, clear float64
	kind        int
}

type hitMark struct {
	beat  float64
	x, y  float64
	color [4]float64
}

type Module struct {
	ctx  *engine.Ctx
	joe  *kart.RigInst
	proj kart.Aff
	unit float64

	pots []*pot

	bops []bopMarker
	bgs  []bgEvt
	// Pack-In uses one fire particle setup in The Miner Grind. These are a
	// hand-rendered stand-in for the three Unity ParticleSystems until the
	// generic ParticleSystem exporter covers the legacy Karate Man prefab.
	particleType      int
	particleWind      float64
	particleIntensity float64

	words []wordEvt
	marks []hitMark

	itemTint [4]float64
	starTint [4]float64

	prepareUntil float64
	lastBeat     int
	lastPunchT   float64

	faceBig *text.GoTextFace
}

func New() engine.Module { return &Module{lastBeat: -1, prepareUntil: math.Inf(-1)} }

func (m *Module) ID() string { return "karateman" }

func (m *Module) Load(ctx *engine.Ctx) error {
	m.ctx = ctx
	if err := ctx.LoadAssets("karateman"); err != nil {
		return err
	}
	m.joe = kart.NewRig(ctx.Assets)
	_, minY, _, maxY := m.joe.BBox()
	m.unit = rigFitH / (maxY - minY)
	m.proj = kart.Translate(joeScreenX, groundY+m.unit*ctx.Assets.Stage.FloorY).Mul(kart.Scale(m.unit, -m.unit))
	m.itemTint = [4]float64{1, 1, 1, 1}
	m.starTint = [4]float64{1, 1, 1, 1}

	src, err := text.NewGoTextFaceSource(bytes.NewReader(goregular.TTF))
	if err != nil {
		return err
	}
	m.faceBig = &text.GoTextFace{Source: src, Size: 46}
	return nil
}

func (m *Module) OnEvent(e *riq.Entity) {
	b := e.Beat
	switch e.Datamodel {
	case "karateman/hit":
		m.scheduleHit(e)
	case "karateman/warnings":
		m.scheduleWord(e)
	case "karateman/bop":
		toggle := boolParamDefault(e, "toggle2", true)
		auto := boolParam(e, "toggle")
		m.bops = append(m.bops, bopMarker{beat: b, auto: auto})
		if toggle {
			for i := 0; i < int(math.Ceil(e.Length)); i++ {
				beat := b + float64(i)
				m.ctx.At(beat, func() { m.bopAt(beat) })
			}
		}
	case "karateman/prepare":
		length := e.Length
		if length <= 0 {
			length = 1
		}
		m.ctx.At(b, func() {
			m.prepareUntil = b + length
			m.playJoe("Prepare", b)
		})
		m.ctx.At(b+length, func() {
			if m.ctx.GameAt(b+length) == m.ID() && m.ctx.Beat() >= m.prepareUntil {
				m.bopAt(b + length)
			}
		})
	case "karateman/background appearance":
		m.addBackground(e)
	case "karateman/set object colors":
		m.ctx.At(b, func() {
			m.itemTint = colorParam(e, "colorC", [4]float64{1, 1, 1, 1})
			m.starTint = colorParam(e, "colorD", [4]float64{1, 1, 1, 1})
		})
	case "karateman/particle effects":
		typ := int(e.Float("type", 0))
		wind := e.Float("valA", 1)
		intensity := e.Float("valB", 1)
		m.ctx.At(b, func() {
			m.particleType = typ
			m.particleWind = wind
			m.particleIntensity = intensity
		})
	case "karateman/force facial expression":
		face := int(e.Float("type", 0))
		m.ctx.At(b, func() {
			// The extracted legacy rig is single-layer. Playing the face clip is
			// faithful for forced-face moments but cannot be blended over a body
			// action until RigInst supports additive/layered clips.
			m.playJoe(fmt.Sprintf("Face%02d", face), b)
		})
	}
}

func (m *Module) Ready() {
	sort.Slice(m.pots, func(i, j int) bool { return m.pots[i].hitBeat < m.pots[j].hitBeat })
	sort.Slice(m.bops, func(i, j int) bool { return m.bops[i].beat < m.bops[j].beat })
	sort.Slice(m.bgs, func(i, j int) bool { return m.bgs[i].beat < m.bgs[j].beat })
}

func (m *Module) OnSwitch(beat float64) {
	m.lastBeat = int(math.Floor(beat)) - 1
	m.prepareUntil = math.Inf(-1)
	m.playJoe("Beat", beat)
}

func (m *Module) Whiff(beat float64) {
	m.playPunch(false, beat)
	m.ctx.Sound("swingNoHit")
}

func (m *Module) Update(t, beat float64) {
	if b := int(math.Floor(beat)); b != m.lastBeat && b >= 0 {
		m.lastBeat = b
		if m.autoBopAt(float64(b)) && t-m.lastPunchT > m.jabDur() && beat >= m.prepareUntil {
			m.bopAt(float64(b))
		}
	}
	if len(m.marks) > 0 {
		alive := m.marks[:0]
		for _, h := range m.marks {
			if beat-h.beat < 0.35 {
				alive = append(alive, h)
			}
		}
		m.marks = alive
	}
}

func (m *Module) Draw(screen *ebiten.Image, t, beat float64) {
	bg, shadow, texture, fx, texType, fxType := m.appearanceAt(beat)
	screen.Fill(rgba(bg))
	m.drawTexture(screen, texture, texType, beat)
	vector.DrawFilledRect(screen, 0, float32(groundY), engine.ScreenW, engine.ScreenH-float32(groundY), shadeRGBA(bg, 0.78), false)
	m.drawParticles(screen, beat)

	m.joe.Sample(t)
	m.joe.Draw(screen, m.proj)
	m.drawPots(screen, t, beat, shadow)
	m.drawHitMarks(screen, beat)
	m.drawWords(screen, beat)
	m.drawFX(screen, fx, fxType, beat)
}

func (m *Module) scheduleHit(e *riq.Entity) {
	b := e.Beat
	typ := int(e.Float("type", 0))
	p := &pot{
		throwBeat: b,
		hitBeat:   b + 1,
		rot0:      deterministicRot(b, len(m.pots)),
		typ:       typ,
		sprite:    hitTypeSprite(typ),
		hitSound:  hitTypeSound(typ),
		heavy:     hitTypeHeavy(typ),
	}
	active := m.ctx.GameAt(b) == m.ID() || m.ctx.GameAt(b+1) == m.ID()
	if active {
		m.pots = append(m.pots, p)
	}
	if !boolParam(e, "mute") {
		m.ctx.SoundAt(b, throwSound(b), 1)
	}
	if !active {
		return
	}

	m.ctx.ScheduleInputCond(
		p.hitBeat,
		func() bool { return p.result == potFlying && m.ctx.GameAt(p.hitBeat) == m.ID() },
		func(state float64, j engine.Judgment) { m.hitPot(p, state, j) },
		func() { m.missPot(p) },
	)
	m.ctx.At(b+2, func() {
		if p.result == potMiss {
			m.ctx.Sound("karate_through")
		}
	})
}

func (m *Module) scheduleWord(e *riq.Entity) {
	b := e.Beat
	kind := int(e.Float("whichWarning", e.Float("type", 2)))
	clear := wordClearBeat(b, e.Length, kind, boolParam(e, "customLength"))
	m.words = append(m.words, wordEvt{beat: b, clear: clear, kind: kind})
	if boolParam(e, "mute") || kind > 4 {
		return
	}
	hitVoice, numberVoice := wordVoice(kind)
	pitch := e.Float("forcePitch", 1)
	if boolParam(e, "pitchVoice") {
		pitch = m.ctx.BPMAt(b+0.5) / pitchMod
	}
	m.ctx.SoundAtPitchOff(b+0.5, hitVoice, 1, pitch, 0.042)
	m.ctx.SoundAtPitchOff(b+1, numberVoice, 1, pitch, 0)
}

func (m *Module) addBackground(e *riq.Entity) {
	b := e.Beat
	preset := int(e.Float("presetBg", 0))
	bgStart := m.backgroundColorAt(b)
	bgEnd := bgPreset(preset)
	if preset == 6 {
		bgStart = colorParam(e, "startColor", bgStart)
		bgEnd = colorParam(e, "endColor", bgStart)
	}
	shadowStart, shadowEnd := tintColor(bgStart), tintColor(bgEnd)
	if int(e.Float("shadowType", 0)) == 1 {
		shadowStart = colorParam(e, "shadowStart", shadowStart)
		shadowEnd = colorParam(e, "shadowEnd", shadowEnd)
	}
	textureStart, textureEnd := tintColor(bgStart), tintColor(bgEnd)
	if !boolParamDefault(e, "autoColor", true) {
		textureStart = colorParam(e, "startTexture", textureStart)
		textureEnd = colorParam(e, "endTexture", textureEnd)
	}
	ev := bgEvt{
		beat: b, length: e.Length, bg0: bgStart, bg1: bgEnd,
		shadow0: shadowStart, shadow1: shadowEnd,
		texture0: textureStart, texture1: textureEnd,
		fx0:  colorParam(e, "fxStart", [4]float64{0.7647, 0.196, 0, 0.2}),
		fx1:  colorParam(e, "fxEnd", [4]float64{0.7647, 0.196, 0, 0.2}),
		ease: int(e.Float("ease", 0)), textureType: int(e.Float("textureType", 0)), fxType: int(e.Float("fxType", 0)),
	}
	m.bgs = append(m.bgs, ev)
}

func (m *Module) hitPot(p *pot, state float64, j engine.Judgment) {
	beat := m.ctx.Beat()
	if j == engine.JudgeNG || math.Abs(state) >= 1 {
		p.result = potNG
		p.judgeT = m.ctx.Time()
		m.playPunch(p.heavy, beat)
		m.ctx.PlayCommon("miss")
		return
	}
	p.result = potHit
	p.judgeT = m.ctx.Time()
	m.playPunch(p.heavy, beat)
	m.ctx.Sound(p.hitSound)
	m.marks = append(m.marks, hitMark{beat: beat, x: m.ctx.Assets.Stage.HitPos[0], y: m.ctx.Assets.Stage.HitPos[1], color: m.starTint})
}

func (m *Module) missPot(p *pot) {
	p.result = potMiss
	p.judgeT = m.ctx.Time()
}

func (m *Module) playPunch(heavy bool, beat float64) {
	m.lastPunchT = m.ctx.Time()
	if heavy {
		m.playJoe("Straight", beat)
		return
	}
	m.playJoe("Jab", beat)
}

func (m *Module) bopAt(beat float64) {
	if m.ctx.GameAt(beat) != m.ID() || beat < m.prepareUntil {
		return
	}
	m.playJoe("Beat", beat)
}

func (m *Module) playJoe(clip string, beat float64) {
	m.joe.Play(clip, m.ctx.BeatToTime(beat))
}

func (m *Module) jabDur() float64 {
	if a, ok := m.ctx.Assets.Anims["Jab"]; ok {
		return a.Duration
	}
	return 0.28
}

func (m *Module) autoBopAt(beat float64) bool {
	auto := false
	for _, ev := range m.bops {
		if ev.beat > beat {
			break
		}
		auto = ev.auto
	}
	return auto
}

func (m *Module) backgroundColorAt(beat float64) [4]float64 {
	bg, _, _, _, _, _ := m.appearanceAt(beat)
	return bg
}

func (m *Module) appearanceAt(beat float64) (bg, shadow, texture, fx [4]float64, textureType, fxType int) {
	bg = bgPreset(0)
	shadow = tintColor(bg)
	texture = tintColor(bg)
	fx = [4]float64{0.7647, 0.196, 0, 0.2}
	for _, ev := range m.bgs {
		if beat < ev.beat {
			break
		}
		u := 1.0
		if ev.length > 0 && beat < ev.beat+ev.length {
			u = (beat - ev.beat) / ev.length
		}
		bg = easeColor(ev.ease, ev.bg0, ev.bg1, u)
		shadow = easeColor(ev.ease, ev.shadow0, ev.shadow1, u)
		texture = easeColor(ev.ease, ev.texture0, ev.texture1, u)
		fx = easeColor(ev.ease, ev.fx0, ev.fx1, u)
		textureType, fxType = ev.textureType, ev.fxType
	}
	return
}

func (m *Module) drawPots(screen *ebiten.Image, t, beat float64, shadow [4]float64) {
	st := &m.ctx.Assets.Stage
	for _, p := range m.pots {
		if beat < p.throwBeat {
			continue
		}
		switch p.result {
		case potFlying, potMiss, potNG:
			alpha := 1.0
			if p.result != potFlying {
				alpha = 1 - (t-p.judgeT)/0.7
				if alpha <= 0 {
					continue
				}
			}
			x, y, z, rot := karatePotFlight(*st, p.throwBeat, p.rot0, beat)
			s := camD / (camD + z)
			if s <= 0 || s > 12 {
				continue
			}
			drawX := st.HitPos[0] + (x-st.HitPos[0])*s
			drawY := st.HitPos[1] + (y-st.HitPos[1])*s
			m.drawPotShadow(screen, drawX, s, shadow, alpha)
			world := kart.Translate(drawX, drawY).Mul(kart.Rotate(rot)).Mul(kart.Scale(s, s))
			tint := scaleAlpha(m.itemTint, alpha)
			m.ctx.Assets.DrawSpriteTint(screen, p.sprite, world, m.proj, false, tint)
		case potHit:
			dt := t - p.judgeT
			if dt > 1.0 {
				continue
			}
			x := st.HitPos[0] - 14*dt
			y := st.HitPos[1] + 10*dt - 14*dt*dt
			world := kart.Translate(x, y).Mul(kart.Rotate(p.rot0 + outSpin*dt))
			m.ctx.Assets.DrawSpriteTint(screen, p.sprite, world, m.proj, false, m.itemTint)
		}
	}
}

func (m *Module) drawPotShadow(screen *ebiten.Image, x, s float64, shadow [4]float64, alpha float64) {
	st := &m.ctx.Assets.Stage
	y := st.HitPos[1] + (st.FloorY+0.05-st.HitPos[1])*s
	world := kart.Translate(x, y).Mul(kart.Scale(s, s))
	m.ctx.Assets.DrawSpriteTint(screen, "karateman_object_shadow", world, m.proj, false, scaleAlpha(shadow, alpha*0.6))
}

func (m *Module) drawHitMarks(screen *ebiten.Image, beat float64) {
	for _, h := range m.marks {
		u := clamp01((beat - h.beat) / 0.35)
		s := 1 + 1.5*u
		world := kart.Translate(h.x+0.3, h.y).Mul(kart.Scale(s, s))
		m.ctx.Assets.DrawSpriteTint(screen, "karateman_hiteffect_0", world, m.proj, false, scaleAlpha(h.color, 1-u))
	}
}

func (m *Module) drawWords(screen *ebiten.Image, beat float64) {
	for _, w := range m.words {
		if beat < w.beat || beat > w.clear {
			continue
		}
		u := clamp01((beat - w.beat) / 0.25)
		y := engine.ScreenH*0.23 - (1-u)*18
		m.drawText(screen, wordLabel(w.kind), engine.ScreenW/2, y, color.RGBA{255, 255, 255, 235}, true)
	}
}

func (m *Module) drawText(screen *ebiten.Image, s string, x, y float64, c color.Color, center bool) {
	if m.faceBig == nil {
		return
	}
	if center {
		w, _ := text.Measure(s, m.faceBig, 0)
		x -= w / 2
	}
	op := &text.DrawOptions{}
	op.GeoM.Translate(x, y)
	op.ColorScale.ScaleWithColor(c)
	text.Draw(screen, s, m.faceBig, op)
}

func (m *Module) drawTexture(screen *ebiten.Image, tint [4]float64, typ int, beat float64) {
	if typ == 0 {
		return
	}
	col := rgba(scaleAlpha(tint, 0.22))
	switch typ {
	case 1, 4:
		for y := 0; y < engine.ScreenH; y += 18 {
			alpha := uint8(25 + (y % 36))
			col.A = alpha
			vector.DrawFilledRect(screen, 0, float32(y), engine.ScreenW, 10, col, false)
		}
	case 2:
		cx, cy := float32(engine.ScreenW/2), float32(engine.ScreenH/2)
		for r := float32(60); r < 720; r += 90 {
			col.A = uint8(36)
			vector.StrokeCircle(screen, cx, cy, r+float32(math.Sin(beat)*6), 6, col, false)
		}
	case 3:
		col.A = 34
		for i := 0; i < 12; i++ {
			x := float32(i * 92)
			vector.StrokeLine(screen, x, 0, x-180, engine.ScreenH, 12, col, false)
		}
	}
}

func (m *Module) drawFX(screen *ebiten.Image, tint [4]float64, typ int, beat float64) {
	if typ == 0 {
		return
	}
	col := rgba(tint)
	switch typ {
	case 1:
		cx, cy := float32(joeScreenX+160), float32(groundY-230)
		for i := 0; i < 20; i++ {
			a := float64(i)/20*math.Pi*2 + beat*0.08
			x := cx + float32(math.Cos(a))*620
			y := cy + float32(math.Sin(a))*620
			vector.StrokeLine(screen, cx, cy, x, y, 18, col, false)
		}
	case 2, 3:
		cx, cy := float32(engine.ScreenW/2), float32(engine.ScreenH/2)
		for r := float32(80); r < 680; r += 120 {
			vector.StrokeCircle(screen, cx, cy, r+float32(math.Mod(beat*16, 120)), 10, col, false)
		}
	}
}

func (m *Module) drawParticles(screen *ebiten.Image, beat float64) {
	if m.particleType == 0 || m.particleIntensity <= 0 {
		return
	}
	count := int(28 * math.Max(0.5, m.particleIntensity))
	for i := 0; i < count; i++ {
		seed := float64(i) * 19.371
		x := math.Mod(seed*37+beat*34*m.particleWind, engine.ScreenW+80) - 40
		y := math.Mod(seed*53+beat*80, engine.ScreenH+80) - 40
		switch m.particleType {
		case 1:
			vector.DrawFilledCircle(screen, float32(x), float32(y), 2.2, color.RGBA{245, 250, 255, 170}, false)
		case 2:
			vector.DrawFilledCircle(screen, float32(x), float32(groundY-40-math.Mod(seed*31+beat*70, 280)), 3.5, color.RGBA{255, 115, 36, 120}, false)
		case 3:
			vector.StrokeLine(screen, float32(x), float32(y), float32(x+12*m.particleWind), float32(y+34), 2, color.RGBA{170, 210, 255, 120}, false)
		}
	}
}

func karatePotFlight(st kmdata.Stage, throwBeat, rot0, beat float64) (x, y, z, rot float64) {
	elapsed := beat - throwBeat
	progress := math.Max(elapsed/2, 0)
	rotCap := 2 * (1 - st.Slip)
	if progress > 1-st.Slip {
		progress = 1 - st.Slip
	}

	pHit := progress + (st.HitOffset - 0.5)
	flyH := pHit * (pHit - 1) / (st.HitOffset * (st.HitOffset - 1))
	startX := st.HitPos[0] + st.StartOffset[0]
	endX := st.HitPos[0] - st.StartOffset[0]
	x = startX + (endX-startX)*progress

	rise := math.Min(math.Max(elapsed, 0), 1)
	y = st.FloorY + (st.HitPos[1]-st.FloorY+st.StartOffset[1]*(1-rise))*flyH
	if progress >= 0.5 && y < st.FloorY {
		y = st.FloorY
	}
	z = st.StartOffsetZ * (1 - 2*progress)
	rot = rot0 + inSpinDeg*math.Pi/180*math.Min(math.Max(elapsed, 0), rotCap)
	return
}

func hitTypeSprite(typ int) string {
	switch typ {
	case hitLightbulb:
		return "karateman_bulb"
	case hitRock:
		return "karateman_rock"
	case hitBall:
		return "karateman_ball"
	case hitCooking:
		return "karateman_cookingpot"
	case hitAlien:
		return "karateman_alien"
	case hitBomb:
		return "karateman_bomb"
	case hitTaco:
		return "karateman_tacobell"
	default:
		return "karateman_pot"
	}
}

func hitTypeSound(typ int) string {
	switch typ {
	case hitLightbulb:
		return "lightbulbHit"
	case hitRock:
		return "rockHit"
	case hitBall:
		return "soccerHit"
	case hitCooking:
		return "cookingPot"
	case hitAlien:
		return "alienHit"
	case hitBomb:
		return "bombHit"
	case hitTaco:
		return "rockHit"
	default:
		return "potHit"
	}
}

func hitTypeHeavy(typ int) bool {
	switch typ {
	case hitRock, hitBall, hitCooking, hitAlien, hitBomb, hitTaco:
		return true
	default:
		return false
	}
}

func throwSound(beat float64) string {
	if math.Abs(math.Mod(beat, 1)-0.5) < 0.0001 {
		return "offbeatObjectOut"
	}
	return "objectOut"
}

func wordVoice(kind int) (hit, number string) {
	number = []string{"en/one", "en/two", "en/three", "en/threeAlt", "en/four"}[clampInt(kind, 0, 4)]
	if kind == 3 {
		return "en/hitAlt", number
	}
	return "en/hit", number
}

func wordLabel(kind int) string {
	switch kind {
	case 0:
		return "HIT ONE"
	case 1:
		return "HIT TWO"
	case 2:
		return "HIT THREE"
	case 3:
		return "HIT THREE"
	case 4:
		return "HIT FOUR"
	case 5:
		return "GRR!"
	case 6:
		return "WARNING"
	case 7:
		return "COMBO"
	default:
		return "HIT"
	}
}

func wordClearBeat(beat, length float64, kind int, custom bool) float64 {
	clear := beat + 3
	if kind <= 4 {
		clear = beat + 4
	} else if kind <= 6 {
		clear = beat + 1
	}
	if custom {
		clear = beat + length
	}
	return clear
}

func bgPreset(preset int) [4]float64 {
	switch preset {
	case 1:
		return [4]float64{0.99607843, 0.60784316, 0.98039216, 1}
	case 2:
		return [4]float64{0.42745098, 0.6666667, 0.8784314, 1}
	case 3:
		return [4]float64{1, 0.09411766, 0, 1}
	case 4:
		return [4]float64{1, 0.58431375, 0.3137255, 1}
	case 5:
		return [4]float64{0.9607844, 0.7725491, 0.78823537, 1}
	case 6:
		return [4]float64{0, 0, 0, 0}
	default:
		return [4]float64{1, 0.8156863, 0.21176471, 1}
	}
}

func tintColor(c [4]float64) [4]float64 {
	return [4]float64{c[0] * 0.55, c[1] * 0.55, c[2] * 0.55, c[3]}
}

func easeColor(kind int, a, b [4]float64, u float64) [4]float64 {
	return [4]float64{
		engine.Ease(kind, a[0], b[0], u),
		engine.Ease(kind, a[1], b[1], u),
		engine.Ease(kind, a[2], b[2], u),
		engine.Ease(kind, a[3], b[3], u),
	}
}

func scaleAlpha(c [4]float64, alpha float64) [4]float64 {
	c[3] *= alpha
	return c
}

func rgba(c [4]float64) color.RGBA {
	return color.RGBA{uint8(clamp01(c[0]) * 255), uint8(clamp01(c[1]) * 255), uint8(clamp01(c[2]) * 255), uint8(clamp01(c[3]) * 255)}
}

func shadeRGBA(c [4]float64, shade float64) color.RGBA {
	return rgba([4]float64{c[0] * shade, c[1] * shade, c[2] * shade, c[3]})
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

func deterministicRot(beat float64, idx int) float64 {
	x := math.Sin(beat*12.9898+float64(idx)*78.233) * 43758.5453
	return (x - math.Floor(x)) * math.Pi * 2
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

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
