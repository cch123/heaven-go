// Package bossanova ports Bossa Nova's partner throw pattern, side switching,
// Bezier-driven ball/cube travel, voice variants, background modes, and hit
// burst particles from Assets/Scripts/Games/BossaNova.
package bossanova

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
	animScale = 0.5

	bgOnePlayer = iota
	bgTwoPlayer
)

var defaultTwoPlayerBG = [4]float64{0.4509804, 0.4509804, 0.4509804, 1}

type patternEvt struct {
	beat, length float64
}

type spinEvt struct {
	beat float64
}

type forceEvt struct {
	beat float64
	side int
}

type bgEvt struct {
	beat, length float64
	typ          int
	c0, c1       [4]float64
	ease         int
}

type plannedThrow struct {
	start     float64
	voice     int
	cube      bool
	spinVoice bool
}

type ringBurst struct {
	beat float64
	t    float64
	left bool
}

type Module struct {
	ctx  *engine.Ctx
	proj kart.Aff

	bossaAnim, novaAnim      string
	cloudAnim, positionAnim  string
	bgOne, bgTwo, bgTwoSR    string
	ringL, ringR             string
	ballSpec, cubeSpec       shapeSpec
	patterns                 []patternEvt
	spins                    []spinEvt
	forces                   []forceEvt
	bgs                      []bgEvt
	shapes                   []*shape
	rings                    []ringBurst
	restoredPatternInSwitch  map[float64]bool
	bossaR, bossaWhiffR      bool
	alternateSpinVoice       bool
	angerLevel               int
	voiceVariant             int
	emotion                  int
	playfulSpinRandomization int
	bg                       bgEvt
}

func New() engine.Module {
	return &Module{
		emotion:                 1,
		bg:                      bgEvt{typ: bgOnePlayer, c0: defaultTwoPlayerBG, c1: defaultTwoPlayerBG},
		restoredPatternInSwitch: map[float64]bool{},
	}
}

func (m *Module) ID() string { return "bossaNova" }

func (m *Module) Load(ctx *engine.Ctx) error {
	m.ctx = ctx
	if err := ctx.LoadAssets("bossaNova"); err != nil {
		return err
	}
	// The prefab's authored background spans 32x18 Unity units; scale 30 maps it
	// exactly to the 960x540 play surface without cropping the one-player vista.
	m.proj = kart.Translate(engine.ScreenW/2, engine.ScreenH/2).Mul(kart.Scale(30, -30))

	game := ctx.Assets.Extra.Components["game"].Refs
	m.bossaAnim = refOr(ctx, game, "bossaAnim", "CharacterPositions/Bossa")
	m.novaAnim = refOr(ctx, game, "novaAnim", "CharacterPositions/Nova")
	m.cloudAnim = refOr(ctx, game, "cloudAnim", "Cloud")
	m.positionAnim = refOr(ctx, game, "positionAnim", "CharacterPositions")
	m.bgOne = refOr(ctx, game, "bgOne", "BG/1P BG")
	m.bgTwo = refOr(ctx, game, "bgTwo", "BG/2P BG")
	m.bgTwoSR = refOr(ctx, game, "bgTwoSR", "BG/2P BG/Square")
	m.ringL = refOr(ctx, game, "ringL", "RingL")
	m.ringR = refOr(ctx, game, "ringR", "RingR")

	m.ballSpec = loadShapeSpec(ctx, "ballShape", false)
	m.cubeSpec = loadShapeSpec(ctx, "cubeShape", true)
	for _, p := range []string{m.ballSpec.path, m.cubeSpec.path, m.ballSpec.shadowPath} {
		ctx.Scene.SetActive(p, false)
	}
	m.setBG(m.bg)
	m.playDefaultAnimators(0)
	return nil
}

func refOr(ctx *engine.Ctx, refs map[string]string, key, fallback string) string {
	if refs != nil {
		if p := refs[key]; p != "" {
			return p
		}
	}
	if p := ctx.Role(key); p != "" {
		return p
	}
	return fallback
}

func (m *Module) playDefaultAnimators(beat float64) {
	sec := m.ctx.SecPerBeat(beat)
	for _, root := range []string{m.positionAnim, m.bossaAnim, m.novaAnim, m.cloudAnim} {
		m.ctx.Scene.PlayDefaultState(root, beat, sec)
	}
}

func (m *Module) OnEvent(e *riq.Entity) {
	switch e.Datamodel {
	case "bossaNova/pattern":
		length := e.Length
		if length <= 0 {
			length = 4
		}
		m.patterns = append(m.patterns, patternEvt{beat: e.Beat, length: length})
	case "bossaNova/throwBall":
		start, voice := e.Beat-1, int(e.Float("voice", maleBounce1))
		m.ctx.At(start, func() { m.throwBall(start, voice, false) })
	case "bossaNova/throwCube":
		start, voice := e.Beat-1, int(e.Float("voice", femaleBounce4))
		m.ctx.At(start, func() { m.throwCube(start, voice, false) })
	case "bossaNova/spin":
		ev := spinEvt{beat: e.Beat}
		m.spins = append(m.spins, ev)
		m.ctx.At(ev.beat, func() { m.spin(ev.beat) })
	case "bossaNova/forcePosition":
		ev := forceEvt{beat: e.Beat, side: int(e.Float("side", 0))}
		m.forces = append(m.forces, ev)
		m.ctx.At(ev.beat, func() { m.forcePosition(ev.beat, ev.side) })
	case "bossaNova/emotion":
		beat, voice := e.Beat, int(e.Float("voice", 1))
		m.ctx.At(beat, func() { m.setVoice(beat, voice) })
	case "bossaNova/setBG":
		ev := bgEvt{
			beat: e.Beat, length: e.Length, typ: int(e.Float("type", bgOnePlayer)),
			c0:   colorParam(e, "colorStart", defaultTwoPlayerBG),
			c1:   colorParam(e, "colorEnd", defaultTwoPlayerBG),
			ease: int(e.Float("ease", 0)),
		}
		m.bgs = append(m.bgs, ev)
		m.ctx.At(ev.beat, func() { m.setBG(ev) })
	}
}

func (m *Module) Ready() {
	sort.Slice(m.patterns, func(i, j int) bool { return m.patterns[i].beat < m.patterns[j].beat })
	sort.Slice(m.spins, func(i, j int) bool { return m.spins[i].beat < m.spins[j].beat })
	sort.Slice(m.forces, func(i, j int) bool { return m.forces[i].beat < m.forces[j].beat })
	sort.Slice(m.bgs, func(i, j int) bool { return m.bgs[i].beat < m.bgs[j].beat })
	for _, ev := range m.patterns {
		ev := ev
		// HS runs Pattern as a preFunction two beats early, so voiceVariant and
		// side forcing are resolved before the first throw at beat-1.
		m.ctx.At(ev.beat-2, func() { m.pattern(ev.beat) })
	}
}

func (m *Module) OnSwitch(beat float64) {
	m.playDefaultAnimators(beat)
	m.bossaR = m.sideAt(beat)
	m.bossaWhiffR = m.whiffSideAt(beat)
	m.setBG(m.bgAt(beat))
	m.restoreActivePatternShapes(beat)
}

func (m *Module) Whiff(beat float64) {
	m.ctx.Scene.PlayState(m.bossaAnim, "Whiff", beat, animScale)
	if m.bossaWhiffR {
		m.ctx.Sound("SE_BOSSA_EN_SWING_BALL")
	} else {
		m.ctx.Sound("SE_BOSSA_EN_SWING_NUT")
	}
}

func (m *Module) Update(_, beat float64) {
	live := m.shapes[:0]
	for _, s := range m.shapes {
		if !s.dead {
			live = append(live, s)
		}
	}
	m.shapes = live

	now := m.ctx.Time()
	rings := m.rings[:0]
	for _, r := range m.rings {
		if now-r.t < 1.0 {
			rings = append(rings, r)
		}
	}
	m.rings = rings
}

func (m *Module) Draw(screen *ebiten.Image, t, beat float64) {
	screen.Fill(color.NRGBA{0xff, 0xb9, 0xf2, 0xff})
	m.applyBGColor(beat)
	m.ctx.SampleScene(beat)
	for _, s := range m.shapes {
		s.queue(m.ctx.Scene, t, beat)
	}
	m.queueRings(m.ctx.Scene, t)
	m.ctx.Scene.Draw(screen, m.proj)
}

func (m *Module) pattern(beat float64) {
	spin := m.hasSpinIn(beat, beat+4)
	if ev, ok := m.lastForceIn(beat-2, beat); ok {
		m.forcePosition(ev.beat, ev.side)
	}
	if m.emotion == 2 {
		m.voiceVariant = (m.voiceVariant + 1) % 3
	} else {
		m.voiceVariant = (m.voiceVariant + 1) % 2
	}
	for _, th := range m.patternThrows(beat, spin) {
		th := th
		m.ctx.At(th.start, func() {
			if th.cube {
				m.throwCube(th.start, th.voice, th.spinVoice)
			} else {
				m.throwBall(th.start, th.voice, th.spinVoice)
				if th.voice == maleBounce1 && !m.bossaR {
					m.ctx.SoundAt(beat, "Nova/SE_BOSSA_EN_36", 0.55)
				}
			}
		})
	}
	m.ctx.At(beat-0.25, func() {
		if m.angerLevel > 4 {
			m.angerLevel = 4
		}
	})
}

func (m *Module) patternThrows(beat float64, spin bool) []plannedThrow {
	out := []plannedThrow{
		{start: beat - 1, voice: maleBounce1},
		{start: beat, voice: femaleBounce2, cube: true},
		{start: beat + 0.5, voice: maleBounce3},
		{start: beat + 1, voice: maleBounce4, spinVoice: spin},
		{start: beat + 1.5, voice: femaleBounce4, cube: true, spinVoice: spin},
	}
	if spin {
		out[3].voice = maleSpin
		out[4].voice = femaleSpin
	} else {
		out = append(out, plannedThrow{start: beat + 2.5, voice: maleBounce6})
	}
	return out
}

func (m *Module) throwBall(beat float64, voiceline int, spin bool) {
	m.spawnShape(&m.ballSpec, beat, voiceline, spin, m.bossaR, true, m.ctx.Beat())
}

func (m *Module) throwCube(beat float64, voiceline int, spin bool) {
	m.spawnShape(&m.cubeSpec, beat, voiceline, spin, !m.bossaR, true, m.ctx.Beat())
}

func (m *Module) spawnShape(spec *shapeSpec, beat float64, voiceline int, spin bool, forBossa bool, schedule bool, nowBeat float64) {
	if spec.template == nil {
		return
	}
	s := newShape(m, spec, beat, voiceline, m.voiceVariant, spin, forBossa)
	if nowBeat >= beat+1 {
		s.isEntering = false
		s.isHit = true
	}
	if schedule {
		s.schedule()
	}
	m.shapes = append(m.shapes, s)
}

func (m *Module) spin(beat float64) {
	m.bossaR = !m.bossaR
	for _, snd := range []struct {
		off  float64
		name string
	}{
		{0, "SE_BOSSA_EN_CHANGE_PUSH"},
		{1, "SE_BOSSA_EN_CHANGE_ROLL"},
		{1, "SE_BOSSA_EN_CHANGE_ROLL_1"},
		{1.25, "SE_BOSSA_EN_CHANGE_ROLL_2"},
		{1.5, "SE_BOSSA_EN_CHANGE_ROLL_3"},
		{1.75, "SE_BOSSA_EN_CHANGE_ROLL_4"},
	} {
		snd := snd
		m.ctx.SoundAt(beat+snd.off, snd.name, 1)
	}
	m.ctx.Scene.PlayState(m.cloudAnim, "Sink", beat, animScale)
	if m.bossaR {
		m.ctx.Scene.PlayState(m.positionAnim, "Sink Left", beat, animScale)
	} else {
		m.ctx.Scene.PlayState(m.positionAnim, "Sink Right", beat, animScale)
	}
	m.spinVoice(beat + 1)
	m.ctx.At(beat+1, func() {
		m.bossaWhiffR = m.bossaR
		if m.bossaR {
			m.ctx.Scene.PlayState(m.bossaAnim, "Spin Right", beat+1, animScale)
			m.ctx.Scene.PlayState(m.novaAnim, "Spin Left", beat+1, animScale)
			m.ctx.Scene.PlayState(m.positionAnim, "Spin Right", beat+1, animScale)
		} else {
			m.ctx.Scene.PlayState(m.bossaAnim, "Spin Left", beat+1, animScale)
			m.ctx.Scene.PlayState(m.novaAnim, "Spin Right", beat+1, animScale)
			m.ctx.Scene.PlayState(m.positionAnim, "Spin Left", beat+1, animScale)
		}
		m.ctx.Scene.PlayState(m.cloudAnim, "Spin", beat+1, animScale)
	})
}

func (m *Module) spinVoice(beat float64) {
	if m.bossaR {
		m.alternateSpinVoice = !m.alternateSpinVoice
	}
	switch m.emotion {
	case 1:
		if m.bossaR {
			if m.alternateSpinVoice {
				m.ctx.SoundAt(beat, "Bossa/SE_BOSSA_EN_60", 0.62)
			} else {
				m.ctx.SoundAt(beat, "Bossa/SE_BOSSA_EN_61", 0.62)
			}
		} else if m.alternateSpinVoice {
			m.ctx.SoundAt(beat, "Nova/SE_BOSSA_EN_64", 0.22)
		} else {
			m.ctx.SoundAt(beat, "Nova/SE_BOSSA_EN_65", 0.22)
		}
	case 2:
		if m.bossaR {
			if m.alternateSpinVoice {
				m.ctx.SoundAt(beat, "Bossa/SE_BOSSA_EN_62", 0.62)
			} else {
				m.ctx.SoundAt(beat, "Bossa/SE_BOSSA_EN_63", 0.62)
			}
		} else if m.alternateSpinVoice {
			m.ctx.SoundAt(beat, "Nova/SE_BOSSA_EN_66", 0.22)
		} else {
			m.ctx.SoundAt(beat, "Nova/SE_BOSSA_EN_67", 0.22)
		}
	}
}

func (m *Module) forcePosition(beat float64, side int) {
	m.bossaR = side == 1
	if m.bossaR {
		m.ctx.Scene.PlayState(m.positionAnim, "IdleR", beat, animScale)
	} else {
		m.ctx.Scene.PlayState(m.positionAnim, "IdleL", beat, animScale)
	}
}

func (m *Module) setVoice(_ float64, voice int) {
	if m.emotion != 0 {
		m.emotion = voice
	}
	if m.emotion != 1 && m.voiceVariant == 2 {
		m.voiceVariant = 0
	}
}

func (m *Module) setBG(ev bgEvt) {
	m.bg = ev
	if ev.typ == bgOnePlayer {
		m.ctx.Scene.SetActive(m.bgOne, true)
		m.ctx.Scene.SetActive(m.bgTwo, false)
		return
	}
	m.ctx.Scene.SetActive(m.bgOne, false)
	m.ctx.Scene.SetActive(m.bgTwo, true)
}

func (m *Module) applyBGColor(beat float64) {
	if m.bg.typ != bgTwoPlayer {
		return
	}
	m.ctx.Scene.SetColorOver(m.bgTwoSR, m.bg.colorAt(beat))
}

func (e bgEvt) colorAt(beat float64) [4]float64 {
	if e.length <= 0 {
		return e.c1
	}
	u := (beat - e.beat) / e.length
	return [4]float64{
		engine.Ease(e.ease, e.c0[0], e.c1[0], u),
		engine.Ease(e.ease, e.c0[1], e.c1[1], u),
		engine.Ease(e.ease, e.c0[2], e.c1[2], u),
		engine.Ease(e.ease, e.c0[3], e.c1[3], u),
	}
}

func (m *Module) bgAt(beat float64) bgEvt {
	ev := bgEvt{typ: bgOnePlayer, c0: defaultTwoPlayerBG, c1: defaultTwoPlayerBG}
	for _, bg := range m.bgs {
		if bg.beat >= beat {
			break
		}
		ev = bg
	}
	return ev
}

func (m *Module) restoreActivePatternShapes(beat float64) {
	if len(m.shapes) > 0 {
		return
	}
	for _, p := range m.patterns {
		if m.restoredPatternInSwitch[p.beat] {
			continue
		}
		if !(p.beat-1 <= beat && beat <= p.beat+p.length) {
			continue
		}
		spin := m.hasSpinIn(p.beat, p.beat+4)
		for _, th := range m.patternThrows(p.beat, spin) {
			if th.start > beat || beat > th.start+2.75 {
				continue
			}
			side := m.sideAt(th.start)
			forBossa := side
			spec := &m.ballSpec
			if th.cube {
				spec = &m.cubeSpec
				forBossa = !side
			}
			schedule := th.start+1 > beat
			m.spawnShape(spec, th.start, th.voice, th.spinVoice, forBossa, schedule, beat)
		}
		m.restoredPatternInSwitch[p.beat] = true
	}
}

func (m *Module) hasSpinIn(from, to float64) bool {
	for _, ev := range m.spins {
		if ev.beat > from && ev.beat < to {
			return true
		}
	}
	return false
}

func (m *Module) lastForceIn(from, to float64) (forceEvt, bool) {
	var out forceEvt
	ok := false
	for _, ev := range m.forces {
		if ev.beat <= from {
			continue
		}
		if ev.beat > to {
			break
		}
		out, ok = ev, true
	}
	return out, ok
}

func (m *Module) sideAt(beat float64) bool {
	side := false
	fi, si := 0, 0
	for {
		nextForce := math.Inf(1)
		if fi < len(m.forces) {
			nextForce = m.forces[fi].beat
		}
		nextSpin := math.Inf(1)
		if si < len(m.spins) {
			nextSpin = m.spins[si].beat
		}
		if nextForce > beat && nextSpin > beat {
			break
		}
		if nextForce <= nextSpin {
			side = m.forces[fi].side == 1
			fi++
			continue
		}
		side = !side
		si++
	}
	return side
}

func (m *Module) whiffSideAt(beat float64) bool {
	side := false
	for _, ev := range m.spins {
		if ev.beat+1 > beat {
			break
		}
		side = m.sideAt(ev.beat)
	}
	return side
}

func (m *Module) playRing(left bool, beat float64) {
	m.rings = append(m.rings, ringBurst{beat: beat, t: m.ctx.BeatToTime(beat), left: left})
}

func (m *Module) queueRings(sc *kart.SceneInst, t float64) {
	cols := [][4]float64{
		{1, 0.22, 0.62, 1},
		{0.1, 0.48, 1, 1},
		{0.72, 0.28, 1, 1},
		{0.32, 0.95, 0.28, 1},
	}
	for _, r := range m.rings {
		u := (t - r.t) / 1.0
		if u < 0 || u > 1 {
			continue
		}
		center := [2]float64{2.71, -1.02}
		if r.left {
			center = [2]float64{-2.91, -1.02}
		}
		alpha := 1 - u
		radius := 0.35 + 1.15*u
		for i := 0; i < 8; i++ {
			ang := float64(i)*math.Pi/4 + u*math.Pi
			x := center[0] + math.Cos(ang)*radius
			y := center[1] + math.Sin(ang)*radius*0.65
			scale := 0.28 * (1 - 0.45*u)
			col := cols[i%len(cols)]
			col[3] *= alpha
			sc.Queue(kart.ExtraSprite{
				Sprite: "recieve_main_15",
				World:  kart.TRS(x, y, ang, scale, scale),
				Order:  3010,
				Tint:   col,
			})
		}
	}
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
	return [4]float64{num(m["r"], def[0]), num(m["g"], def[1]), num(m["b"], def[2]), num(m["a"], def[3])}
}

func num(v any, def float64) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	default:
		return def
	}
}
