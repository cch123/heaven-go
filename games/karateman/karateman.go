// Package karateman ports the Pack-In Karate Man cue path onto engine.App.
//
// The module uses the legacy single-rig Karate Man extraction: Joe is rendered
// through kart.RigInst while tossed objects are driven by the original
// KarateManPot.ProgressToFlyPosition stage parameters.
package karateman

import (
	"fmt"
	"image/color"
	"math"
	"sort"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"hsdemo/engine"
	"hsdemo/games/internal/particlefx"
	"hsdemo/kart"
	"hsdemo/kmdata"
	"hsdemo/riq"
)

const (
	joeScreenX = 310.0
	groundY    = 470.0
	rigFitH    = 330.0

	karateJoeFaceLayer = "joe/head-face"

	inSpinDeg = 125.0
	outSpin   = -9.0
	camD      = 10.0

	pitchMod = 125.0
)

var (
	defaultJoeBodyColor      = [4]float64{1, 1, 1, 1}
	defaultJoeHighlightColor = [4]float64{0.81, 0.81, 0.81, 1}
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

const (
	potKindNormal = iota
	potKindCombo1
	potKindCombo2
	potKindCombo3
	potKindCombo4
	potKindCombo5
	potKindComboEnd
	potKindKickBarrel
	potKindKickPayload
)

const (
	comboModeDisabled = iota
	comboModeNormal
	comboModeJump
)

const (
	karateHitParticleBarrel = iota
	karateHitParticleOther
	karateHitParticleBomb
	karateHitParticlePot
	karateHitParticleRock
	karateHitParticleLight
	karateHitParticleBombSmall
	karateHitParticleKick
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
	kind      int
	tint      [4]float64
	kickBall  bool
	kickBeat  float64
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

type delayedParticleSpec struct {
	delay  float64
	root   int
	curve  int
	u      float64
	flight bool
	sound  string
	volume float64
}

type Module struct {
	ctx      *engine.Ctx
	joe      *kart.RigInst
	word     *kart.RigInst
	proj     kart.Aff
	wordProj kart.Aff
	unit     float64

	pots []*pot

	bops             []bopMarker
	bgs              []bgEvt
	particles        *particlefx.Runtime
	particleRoots    map[int]string
	hitParticleRoots map[int]string
	hitEffects       []particlefx.Effect

	particleType      int
	particleWind      float64
	particleIntensity float64
	particleStartT    float64
	particlePrewarm   bool

	comboMode      int
	noriMode       int
	noriMax        float64
	nori           float64
	gameplayFxType int

	words []wordEvt
	marks []hitMark

	itemTint [4]float64
	starTint [4]float64

	prepareUntil   float64
	lastBeat       int
	lastPunchT     float64
	comboSeq       int
	specialCamBeat float64
	specialCamEnd  float64
	specialCamRet  float64

	seriousActive bool
	seriousFlash  bool
	seriousRed    [4]float64
	seriousWhite  [4]float64
	seriousBlack  [4]float64
	seriousHit    float64
}

func New() engine.Module {
	return &Module{
		lastBeat:       -1,
		prepareUntil:   math.Inf(-1),
		comboMode:      comboModeNormal,
		specialCamBeat: math.Inf(1),
		specialCamEnd:  math.Inf(-1),
		seriousRed:     [4]float64{1, 0, 0, 1},
		seriousWhite:   [4]float64{1, 1, 1, 1},
		seriousBlack:   [4]float64{0, 0, 0, 1},
		seriousHit:     math.Inf(-1),
	}
}

func (m *Module) ID() string { return "karateman" }

func (m *Module) Load(ctx *engine.Ctx) error {
	m.ctx = ctx
	if err := ctx.LoadAssets("karateman"); err != nil {
		return err
	}
	m.joe = kart.NewRig(ctx.Assets)
	wordAssets := *ctx.Assets
	wordAssets.Rig = karateWordRig()
	m.word = kart.NewRig(&wordAssets)
	m.applyJoeAppearance(defaultJoeBodyColor, defaultJoeHighlightColor, false)
	_, minY, _, maxY := m.joe.BBox()
	m.unit = rigFitH / (maxY - minY)
	m.proj = kart.Translate(joeScreenX, groundY+m.unit*ctx.Assets.Stage.FloorY).Mul(kart.Scale(m.unit, -m.unit))
	// The Word prefab is a sibling of Joe in Unity's scene root. RigInst clears
	// root translation by design, so the root x offset lives in the projection
	// while the prefab's 0.8 scale remains on the synthetic root node.
	m.wordProj = kart.Translate(engine.ScreenW/2-0.5*54, engine.ScreenH/2).Mul(kart.Scale(54, -54))
	m.itemTint = [4]float64{1, 1, 1, 1}
	m.starTint = [4]float64{1, 1, 1, 1}
	m.particles = particlefx.NewIncludingInactive(ctx.Assets, m.proj, 6)
	m.particleRoots = map[int]string{
		1: karateParticleRoot(ctx.Assets, "Snow"),
		2: karateParticleRoot(ctx.Assets, "Fire"),
		3: karateParticleRoot(ctx.Assets, "Rain"),
	}
	m.hitParticleRoots = map[int]string{
		karateHitParticleBarrel:    karateParticleRoot(ctx.Assets, "krt_barrel00"),
		karateHitParticleOther:     karateParticleRoot(ctx.Assets, "krt_other00"),
		karateHitParticleBomb:      karateParticleRoot(ctx.Assets, "krt_bomb00"),
		karateHitParticlePot:       karateParticleRoot(ctx.Assets, "krt_pot00"),
		karateHitParticleRock:      karateParticleRoot(ctx.Assets, "krt_rock00"),
		karateHitParticleLight:     karateParticleRoot(ctx.Assets, "krt_light00"),
		karateHitParticleBombSmall: karateParticleRoot(ctx.Assets, "krt_bomb01"),
		karateHitParticleKick:      karateParticleRoot(ctx.Assets, "krt_kick00"),
	}
	return nil
}

func (m *Module) applyJoeAppearance(body, highlight [4]float64, wig bool) {
	pal := kart.Palette{
		Alpha:   body,
		Fill:    [4]float64{1, 0, 0, 1},
		Outline: highlight,
	}
	// These paths are the SpriteRenderers using karateman_cellshader.mat in
	// the Unity prefab. Shadow sprites deliberately stay outside this palette.
	for _, path := range []string{"Head", "Body", "LeftArm", "RightArm", "LeftLeg", "RightLeg"} {
		m.joe.SetPalette(path, pal)
	}
	m.joe.SetActive("Head/Wig", wig)
}

func (m *Module) OnEvent(e *riq.Entity) {
	b := e.Beat
	switch e.Datamodel {
	case "karateman/hit":
		m.scheduleHit(e)
	case "karateman/bulb":
		m.scheduleBulb(e)
	case "karateman/combo":
		m.scheduleCombo(e)
	case "karateman/kick":
		m.scheduleKick(e)
	case "karateman/warnings":
		m.scheduleWord(e)
	case "karateman/hitX":
		// Legacy hidden warning action. Unity's converter offsets type 0..6 by
		// one and maps type>=7 to the "hit one" warning; missing type is ignored.
		if kind, ok := legacyHitXWarningKind(e); ok {
			m.scheduleWord(&riq.Entity{Beat: e.Beat, Length: e.Length, Data: map[string]any{
				"whichWarning": float64(kind),
			}})
		}
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
	case "karateman/set background effects":
		m.addLegacyBackground(e)
	case "karateman/set object colors":
		m.ctx.At(b, func() {
			m.applyJoeAppearance(
				colorParam(e, "colorA", defaultJoeBodyColor),
				colorParam(e, "colorB", defaultJoeHighlightColor),
				boolParam(e, "wig"),
			)
			m.itemTint = colorParam(e, "colorC", [4]float64{1, 1, 1, 1})
			if int(e.Float("star", 0)) == 0 {
				m.starTint = m.itemTint
			} else {
				m.starTint = colorParam(e, "colorD", [4]float64{1, 1, 1, 1})
			}
		})
	case "karateman/particle effects":
		typ := int(e.Float("type", 0))
		wind := e.Float("valA", 1)
		intensity := e.Float("valB", 1)
		instant := boolParam(e, "instant")
		m.ctx.At(b, func() {
			m.particleType = typ
			m.particleWind = wind
			m.particleIntensity = intensity
			m.particleStartT = m.ctx.Time()
			m.particlePrewarm = instant
		})
	case "karateman/force facial expression":
		face := int(e.Float("type", 0))
		m.ctx.At(b, func() {
			m.playJoeFace(face, b)
		})
	case "karateman/special camera":
		length := e.Length
		if length <= 0 {
			length = 8
		}
		ret := math.Min(2, length*0.5)
		end := math.MaxFloat64
		if boolParamDefault(e, "toggle", true) {
			end = b + length - 0.001
		}
		m.ctx.At(b, func() {
			m.specialCamBeat = b
			m.specialCamEnd = end
			m.specialCamRet = ret
		})
	case "karateman/set gameplay modifiers":
		mode := int(e.Float("type", 0))
		fxType := int(e.Float("fxType", 0))
		combo := int(e.Float("combo", comboModeNormal))
		tMax := e.Float("TengokuMax", 5)
		mMax := e.Float("MaxMania", 10)
		m.ctx.At(b, func() {
			m.noriMode = mode
			m.comboMode = combo
			m.noriMax = 0
			switch mode {
			case 1:
				m.noriMax = math.Max(1, tMax)
			case 2, 3:
				m.noriMax = math.Max(1, mMax)
			}
			if m.noriMax > 0 && m.nori <= 0 {
				m.nori = m.noriMax * 0.5
			}
			m.gameplayFxType = fxType
		})
	case "karateman/toggleseriousBG":
		active := boolParamDefault(e, "boolserious", true)
		red := colorParam(e, "redColor", [4]float64{1, 0, 0, 1})
		white := colorParam(e, "whiteColor", [4]float64{1, 1, 1, 1})
		black := colorParam(e, "blackColor", [4]float64{0, 0, 0, 1})
		flashing := boolParamDefault(e, "flashing", true)
		m.ctx.At(b, func() {
			m.seriousActive = active
			m.seriousRed, m.seriousWhite, m.seriousBlack = red, white, black
			m.seriousFlash = flashing
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
	m.joe.ClearLayer(karateJoeFaceLayer)
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
	if len(m.hitEffects) > 0 {
		alive := m.hitEffects[:0]
		for _, fx := range m.hitEffects {
			if t-fx.StartT < fx.Life {
				alive = append(alive, fx)
			}
		}
		m.hitEffects = alive
	}
}

func (m *Module) Draw(screen *ebiten.Image, t, beat float64) {
	bg, shadow, texture, fx, texType, fxType := m.appearanceAt(beat)
	if fxType == 0 {
		fxType = m.gameplayFxType
	}
	screen.Fill(rgba(bg))
	m.drawTexture(screen, texture, texType, beat)
	m.drawFX(screen, fx, fxType, beat)
	vector.DrawFilledRect(screen, 0, float32(groundY), engine.ScreenW, engine.ScreenH-float32(groundY), shadeRGBA(bg, 0.78), false)
	m.drawParticles(screen, t)
	m.drawSeriousBG(screen, beat)

	proj := m.cameraProj(beat)
	m.joe.Sample(t)
	m.joe.Draw(screen, proj)
	m.drawPots(screen, t, beat, shadow, proj)
	m.drawHitParticles(screen, t, proj)
	m.drawHitMarks(screen, beat)
	m.drawWords(screen, t, beat)
	m.drawNori(screen)
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
		kind:      potKindNormal,
		tint:      [4]float64{1, 1, 1, 1},
	}
	m.schedulePot(p, throwSound(b), !boolParam(e, "mute"), 0)
}

func (m *Module) scheduleBulb(e *riq.Entity) {
	b := e.Beat
	bulbType := int(e.Float("type", 0))
	if bulbType == 0 {
		bulbType = m.nextBulbType(b)
	}
	tint := bulbTint(bulbType, colorParam(e, "colorA", [4]float64{1, 1, 1, 1}))
	throw := bulbThrowSound(b, bulbType, int(e.Float("sfx", 0)), stringParam(e, "throwSfx", "lightbulbOut"))
	hit := bulbHitSound(bulbType, int(e.Float("sfx", 0)), stringParam(e, "hitSfx", "lightbulbHit"))
	p := &pot{
		throwBeat: b,
		hitBeat:   b + 1,
		rot0:      deterministicRot(b, len(m.pots)),
		typ:       hitLightbulb,
		sprite:    "karateman_bulb",
		hitSound:  hit,
		kind:      potKindNormal,
		tint:      tint,
	}
	m.schedulePot(p, throw, !boolParam(e, "mute"), 0)
}

func (m *Module) scheduleCombo(e *riq.Entity) {
	b := e.Beat
	m.ctx.SoundAt(b, "barrelOutCombos", 1)
	if !boolParam(e, "disableVoice") {
		m.scheduleComboVoice(b, boolParam(e, "pitchVoice"), e.Float("forcePitch", 1), boolParamDefault(e, "cutOut", true))
	}
	for i, off := range []float64{0, 0.25, 0.5, 0.75, 1, 1.5} {
		kind := potKindCombo1 + i
		sprite := "karateman_pot"
		if kind == potKindComboEnd {
			sprite = "karateman_barrel"
		}
		throwBeat := b + off
		p := &pot{
			throwBeat: throwBeat,
			hitBeat:   throwBeat + 1,
			rot0:      deterministicRot(throwBeat, len(m.pots)+i),
			typ:       hitPot,
			sprite:    sprite,
			hitSound:  comboHitSound(kind),
			heavy:     kind == potKindComboEnd,
			kind:      kind,
			tint:      [4]float64{1, 1, 1, 1},
		}
		m.schedulePot(p, "", false, comboAction(kind))
	}
}

func (m *Module) scheduleKick(e *riq.Entity) {
	b := e.Beat
	offset := e.Float("KickOffset", 0)
	m.ctx.SoundAt(b, "barrelOutKicks", 1)
	if !boolParam(e, "disableVoice") {
		m.scheduleKickVoice(b, offset, boolParam(e, "pitchVoice"), e.Float("forcePitch", 1), boolParamDefault(e, "cutOut", true))
	}
	p := &pot{
		throwBeat: b,
		hitBeat:   b + 1,
		rot0:      deterministicRot(b, len(m.pots)),
		typ:       hitPot,
		sprite:    "karateman_barrel",
		hitSound:  "barrelBreak",
		heavy:     true,
		kind:      potKindKickBarrel,
		tint:      [4]float64{1, 1, 1, 1},
		kickBall:  boolParam(e, "toggle"),
		kickBeat:  b + 1.75 + offset,
	}
	m.schedulePot(p, "", false, 0)
}

func (m *Module) schedulePot(p *pot, throw string, playThrow bool, action int) {
	active := m.ctx.GameAt(p.throwBeat) == m.ID() || m.ctx.GameAt(p.hitBeat) == m.ID()
	if active {
		m.pots = append(m.pots, p)
	}
	if playThrow && throw != "" {
		m.ctx.SoundAt(p.throwBeat, throw, 1)
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
	if action > 0 {
		// The legacy input layer has only coarse action channels, so the combo
		// windows stay score-equivalent while still accepting the alternate key
		// used by Heaven Studio's AltDown/AltUp combo path.
		alt := m.ctx.ScheduleInputActionCond(
			p.hitBeat,
			action,
			func() bool { return p.result == potFlying && m.ctx.GameAt(p.hitBeat) == m.ID() },
			func(state float64, j engine.Judgment) { m.hitPot(p, state, j) },
			func() {},
		)
		alt.NoAutoplay = true
	}
	m.ctx.At(p.throwBeat+2, func() {
		if p.result == potMiss {
			m.ctx.Sound("karate_through")
			m.noriThrough()
		}
	})
	m.scheduleDelayedParticle(p, p.throwBeat, potMiss)
}

func (m *Module) scheduleComboVoice(beat float64, bpmPitch bool, forcePitch float64, _ bool) {
	for _, s := range []struct {
		off  float64
		name string
	}{
		{1, "en/punchy1"},
		{1.25, "en/punchy2"},
		{1.5, "en/punchy3"},
		{1.75, "en/punchy4"},
		{2, "en/ko"},
		{2.5, "en/pow"},
	} {
		b := beat + s.off
		pitch := forcePitch
		if bpmPitch {
			pitch = m.ctx.BPMAt(b) / pitchMod
		}
		m.ctx.SoundAtPitchOff(b, s.name, 1, pitch, 0)
	}
}

func (m *Module) scheduleKickVoice(beat, offset float64, bpmPitch bool, forcePitch float64, _ bool) {
	for _, s := range []struct {
		off  float64
		name string
	}{
		{1, "en/punchKick1"},
		{1.5, "en/punchKick2"},
		{1.75 + offset, "en/punchKick3"},
		{2.5 + offset, "en/punchKick4"},
	} {
		b := beat + s.off
		pitch := forcePitch
		if bpmPitch {
			pitch = m.ctx.BPMAt(b) / pitchMod
		}
		m.ctx.SoundAtPitchOff(b, s.name, 1, pitch, 0)
	}
}

func (m *Module) nextBulbType(beat float64) int {
	next := math.Inf(1)
	typ := 1
	for _, p := range m.pots {
		if p.throwBeat < beat || p.throwBeat >= next {
			continue
		}
		switch p.kind {
		case potKindCombo1, potKindCombo2, potKindCombo3, potKindCombo4, potKindCombo5, potKindComboEnd:
			next, typ = p.throwBeat, 2
		case potKindKickBarrel, potKindKickPayload:
			next, typ = p.throwBeat, 3
		}
	}
	return typ
}

func bulbTint(typ int, custom [4]float64) [4]float64 {
	switch typ {
	case 2:
		return [4]float64{0.23137255, 1, 1, 1}
	case 3:
		return [4]float64{1, 1, 0, 1}
	case 4:
		return custom
	default:
		return [4]float64{1, 1, 1, 1}
	}
}

func bulbThrowSound(beat float64, typ, sfx int, custom string) string {
	base := "Lightbulb"
	if sfx == 3 {
		return custom
	}
	if sfx == 2 || (sfx == 0 && typ == 3) {
		base = "LightbulbNtr"
	}
	if math.Abs(math.Mod(beat, 1)-0.5) < 0.0001 {
		return "offbeat" + base + "Out"
	}
	return strings.ToLower(base[:1]) + base[1:] + "Out"
}

func bulbHitSound(typ, sfx int, custom string) string {
	if sfx == 3 {
		return custom
	}
	if sfx == 2 || (sfx == 0 && typ == 3) {
		return "lightbulbNtrHit"
	}
	return "lightbulbHit"
}

func comboHitSound(kind int) string {
	switch kind {
	case potKindCombo3:
		return "comboHit2"
	case potKindCombo4, potKindCombo5:
		return "comboHit3"
	case potKindComboEnd:
		return "comboHit4"
	default:
		return "comboHit1"
	}
}

func comboAction(kind int) int {
	switch kind {
	case potKindCombo1, potKindComboEnd:
		return 1
	default:
		return 0
	}
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

func (m *Module) addLegacyBackground(e *riq.Entity) {
	b := e.Beat
	preset := int(e.Float("type", 0))
	bgStart := m.backgroundColorAt(b)
	bgEnd := bgPreset(preset)
	if preset == 6 {
		bgEnd = colorParam(e, "colorA", bgStart)
	}
	shadowStart, shadowEnd := tintColor(bgStart), tintColor(bgEnd)
	if int(e.Float("type2", 0)) == 1 {
		shadowEnd = colorParam(e, "colorB", shadowEnd)
	}
	textureStart, textureEnd := tintColor(bgStart), tintColor(bgEnd)
	if int(e.Float("type5", 0)) == 1 {
		textureStart = colorParam(e, "colorC", textureStart)
		textureEnd = colorParam(e, "colorD", textureEnd)
	}
	m.bgs = append(m.bgs, bgEvt{
		beat: b, length: e.Length, bg0: bgStart, bg1: bgEnd,
		shadow0: shadowStart, shadow1: shadowEnd,
		texture0: textureStart, texture1: textureEnd,
		fx0:  colorParam(e, "colorC", [4]float64{0.7647, 0.196, 0, 0.2}),
		fx1:  colorParam(e, "colorD", [4]float64{0.7647, 0.196, 0, 0.2}),
		ease: 0, textureType: int(e.Float("type4", 0)), fxType: int(e.Float("type3", 0)),
	})
}

func (m *Module) hitPot(p *pot, state float64, j engine.Judgment) {
	beat := m.ctx.Beat()
	if j == engine.JudgeNG || math.Abs(state) >= 1 {
		p.result = potNG
		p.judgeT = m.ctx.Time()
		m.playPotAction(p, beat, false)
		m.ctx.PlayCommon("miss")
		m.scheduleDelayedParticle(p, beat, potNG)
		m.noriNG()
		return
	}
	p.result = potHit
	p.judgeT = m.ctx.Time()
	m.playPotAction(p, beat, true)
	m.ctx.Sound(p.hitSound)
	m.marks = append(m.marks, hitMark{beat: beat, x: m.ctx.Assets.Stage.HitPos[0], y: m.ctx.Assets.Stage.HitPos[1], color: m.starTint})
	m.spawnHitParticles(p, beat)
	m.scheduleDelayedParticle(p, beat, potHit)
	m.seriousHit = beat
	m.noriHit()
	if p.kind == potKindKickBarrel {
		m.spawnKickPayload(p)
	}
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

func (m *Module) playPotAction(p *pot, beat float64, hit bool) {
	if p.kind == potKindKickPayload {
		if hit {
			m.lastPunchT = m.ctx.Time()
			m.playJoe("ManKick", beat)
			return
		}
		m.playJoe("LowKickMiss", beat)
		return
	}
	switch p.kind {
	case potKindCombo1:
		m.comboSeq = 1
		m.playJoe("Jab", beat)
	case potKindCombo2:
		m.comboSeq = 2
		m.playJoe("Straight", beat)
	case potKindCombo3:
		m.comboSeq = 3
		m.playJoe("LowJab", beat)
	case potKindCombo4:
		m.comboSeq = 4
		if hit {
			m.playJoe("LowKick", beat)
		} else {
			m.playJoe("LowKickMiss", beat)
			m.ctx.Sound("comboMiss")
		}
	case potKindCombo5:
		m.comboSeq = 5
		m.playJoe("BackHand", beat)
	case potKindComboEnd:
		m.comboSeq = 0
		if m.comboMode == comboModeJump {
			m.playJoe("UpperCutJump", beat)
		} else {
			m.playJoe("UpperCut", beat)
		}
	case potKindKickBarrel:
		m.playPunch(true, beat)
	case potKindNormal:
		m.playPunch(p.heavy, beat)
	default:
		m.playPunch(p.heavy, beat)
	}
}

func (m *Module) spawnKickPayload(src *pot) {
	sprite := "karateman_bomb"
	sound := "bombKick"
	typ := hitBomb
	if src.kickBall {
		sprite = "karateman_ball"
		typ = hitBall
	}
	throwBeat := src.throwBeat + 1
	if src.kickBeat > throwBeat {
		throwBeat = src.kickBeat - 0.75
	}
	p := &pot{
		throwBeat: throwBeat,
		hitBeat:   src.kickBeat,
		rot0:      deterministicRot(src.kickBeat, len(m.pots)),
		typ:       typ,
		sprite:    sprite,
		hitSound:  sound,
		heavy:     true,
		kind:      potKindKickPayload,
		tint:      [4]float64{1, 1, 1, 1},
	}
	m.schedulePot(p, "", false, 0)
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

func (m *Module) playJoeFace(face int, beat float64) {
	clip := fmt.Sprintf("karateman/Head/Face%02d", face)
	m.joe.PlayLayer(karateJoeFaceLayer, "Head", clip, m.ctx.BeatToTime(beat))
}

func (m *Module) cameraProj(beat float64) kart.Aff {
	u := 0.0
	if beat >= m.specialCamBeat && beat <= m.specialCamEnd {
		u = 1
		switch {
		case m.specialCamRet > 0 && beat <= m.specialCamBeat+m.specialCamRet:
			u = engine.Ease(6, 0, 1, clamp01((beat-m.specialCamBeat)/m.specialCamRet))
		case m.specialCamRet > 0 && beat >= m.specialCamEnd-m.specialCamRet:
			u = engine.Ease(3, 1, 0, clamp01((beat-(m.specialCamEnd-m.specialCamRet))/m.specialCamRet))
		}
	}
	// Prefab camera points are near=(0,-0.8,-8.25), far=(0,-0.6,-10).
	// In the 2D projection this is equivalent to a slight zoom out and upward
	// move, while preserving the legacy rig's extracted world coordinates.
	scale := 1 - 0.175*u
	y := -18 * u
	return kart.Translate(joeScreenX, groundY+y+m.unit*m.ctx.Assets.Stage.FloorY).
		Mul(kart.Scale(m.unit*scale, -m.unit*scale))
}

func (m *Module) drawSeriousBG(screen *ebiten.Image, beat float64) {
	if !m.seriousActive {
		return
	}
	topH := float32(engine.ScreenH) * 0.48
	bottomY := topH
	white := rgba(m.seriousWhite)
	black := rgba(m.seriousBlack)
	red := rgba(m.seriousRed)
	if m.seriousFlash && beat-m.seriousHit >= 0 && beat-m.seriousHit < 0.25 {
		white, black = black, white
	}
	vector.DrawFilledRect(screen, 0, 0, engine.ScreenW, topH, white, false)
	vector.DrawFilledRect(screen, 0, bottomY, engine.ScreenW, engine.ScreenH-bottomY, black, false)
	for i := -1; i < 8; i++ {
		x := float32(i*170) + float32(math.Mod(beat*36, 170))
		vector.StrokeLine(screen, x, 20, x+90, topH-10, 18, red, false)
		vector.StrokeLine(screen, x+70, bottomY+18, x-30, engine.ScreenH-20, 18, red, false)
	}
}

func (m *Module) drawNori(screen *ebiten.Image) {
	if m.noriMode == 0 || m.noriMax <= 0 {
		return
	}
	const w, h = float32(210), float32(14)
	x := float32(engine.ScreenW) - w - 28
	y := float32(28)
	if m.noriMode == 3 {
		x = float32(engine.ScreenW/2) - w/2
		y = float32(engine.ScreenH) - 34
	}
	vector.DrawFilledRect(screen, x-2, y-2, w+4, h+4, color.RGBA{0, 0, 0, 170}, false)
	vector.DrawFilledRect(screen, x, y, w, h, color.RGBA{45, 25, 20, 220}, false)
	fill := w * float32(clamp01(m.nori/m.noriMax))
	col := color.RGBA{255, 218, 57, 240}
	if m.noriMode == 2 || m.noriMode == 3 {
		col = color.RGBA{255, 80, 80, 240}
	}
	vector.DrawFilledRect(screen, x, y, fill, h, col, false)
}

func (m *Module) noriHit() {
	if m.noriMax <= 0 {
		return
	}
	m.nori = math.Min(m.noriMax, m.nori+1)
	m.ctx.Sound("nori_just")
}

func (m *Module) noriNG() {
	if m.noriMax <= 0 {
		return
	}
	m.nori = math.Max(0, m.nori-1.5)
	m.ctx.Sound("nori_ng")
}

func (m *Module) noriThrough() {
	if m.noriMax <= 0 {
		return
	}
	m.nori = math.Max(0, m.nori-1)
	m.ctx.Sound("nori_through")
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

func (m *Module) drawPots(screen *ebiten.Image, t, beat float64, shadow [4]float64, proj kart.Aff) {
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
			m.drawPotShadow(screen, drawX, s, shadow, alpha, proj)
			world := kart.Translate(drawX, drawY).Mul(kart.Rotate(rot)).Mul(kart.Scale(s, s))
			tint := scaleAlpha(mulColor(m.itemTint, p.tint), alpha)
			m.ctx.Assets.DrawSpriteTint(screen, p.sprite, world, proj, false, tint)
		case potHit:
			dt := t - p.judgeT
			if dt > 1.0 {
				continue
			}
			x := st.HitPos[0] - 14*dt
			y := st.HitPos[1] + 10*dt - 14*dt*dt
			world := kart.Translate(x, y).Mul(kart.Rotate(p.rot0 + outSpin*dt))
			m.ctx.Assets.DrawSpriteTint(screen, p.sprite, world, proj, false, mulColor(m.itemTint, p.tint))
		}
	}
}

func (m *Module) drawPotShadow(screen *ebiten.Image, x, s float64, shadow [4]float64, alpha float64, proj kart.Aff) {
	st := &m.ctx.Assets.Stage
	y := st.HitPos[1] + (st.FloorY+0.05-st.HitPos[1])*s
	world := kart.Translate(x, y).Mul(kart.Scale(s, s))
	m.ctx.Assets.DrawSpriteTint(screen, "karateman_object_shadow", world, proj, false, scaleAlpha(shadow, alpha*0.6))
}

func (m *Module) drawHitMarks(screen *ebiten.Image, beat float64) {
	for _, h := range m.marks {
		u := clamp01((beat - h.beat) / 0.35)
		s := 1 + 1.5*u
		world := kart.Translate(h.x+0.3, h.y).Mul(kart.Scale(s, s))
		m.ctx.Assets.DrawSpriteTint(screen, "karateman_hiteffect_0", world, m.proj, false, scaleAlpha(h.color, 1-u))
	}
}

func (m *Module) drawHitParticles(screen *ebiten.Image, t float64, proj kart.Aff) {
	if m.particles == nil {
		return
	}
	m.particles.Proj = proj
	for _, fx := range m.hitEffects {
		m.particles.Draw(screen, fx, t)
	}
}

func (m *Module) drawWords(screen *ebiten.Image, t, beat float64) {
	if m.word == nil {
		return
	}
	var active *wordEvt
	for _, w := range m.words {
		if beat < w.beat || beat > w.clear {
			continue
		}
		if active == nil || w.beat >= active.beat {
			wc := w
			active = &wc
		}
	}
	if active == nil {
		return
	}
	m.word.Play(wordClip(active.kind), m.ctx.BeatToTime(active.beat))
	m.word.Sample(t)
	m.word.Draw(screen, m.wordProj)
}

func (m *Module) drawTexture(screen *ebiten.Image, tint [4]float64, typ int, beat float64) {
	sprite := karateBGTextureSprite(typ)
	if sprite == "" {
		return
	}
	m.drawScreenSprite(screen, sprite, tint, karateBGTextureCovers(typ))
}

func (m *Module) drawFX(screen *ebiten.Image, tint [4]float64, typ int, beat float64) {
	sprite := karateBGFXSprite(m.ctx.Assets.Anims, typ, beat)
	if sprite == "" {
		return
	}
	m.drawScreenSprite(screen, sprite, tint, true)
}

func karateBGTextureSprite(typ int) string {
	switch typ {
	case 1, 4:
		return "bg_gradient"
	case 2:
		return "radial_gradient"
	case 3:
		return "karate_bg_bloody"
	default:
		return ""
	}
}

func karateBGTextureCovers(typ int) bool {
	// Gradient/SeniorGradient are SpriteRenderer sliced backgrounds; the other
	// texture sprites keep their source aspect and cover the camera view.
	return typ == 2 || typ == 3
}

func karateBGFXClip(typ int) string {
	switch typ {
	case 1:
		return "bg/Sunburst"
	case 2, 3:
		// Old Remix Editor builds serialized the rings effect as 3; keep it
		// resolving to the same official clip so imported custom levels behave.
		return "bg/Rings"
	default:
		return ""
	}
}

func karateBGFXSprite(anims map[string]*kmdata.Anim, typ int, beat float64) string {
	clip := anims[karateBGFXClip(typ)]
	if clip == nil || clip.Duration <= 0 {
		return ""
	}
	// KarateMan.cs drives DoNormalizedAnimation with (songPos * 0.5) % 1,
	// so the two-frame background effect advances on beat position, not wall time.
	at := math.Mod(math.Max(beat, 0)*0.5, 1) * clip.Duration
	pose := kart.SampleClipNode(clip, "", at)
	if !pose.HasSprite {
		return ""
	}
	return pose.Sprite
}

func (m *Module) drawScreenSprite(screen *ebiten.Image, sprite string, tint [4]float64, cover bool) {
	img := m.ctx.Assets.Sub(sprite)
	if img == nil {
		return
	}
	b := img.Bounds()
	w, h := float64(b.Dx()), float64(b.Dy())
	if w <= 0 || h <= 0 {
		return
	}
	sx := float64(engine.ScreenW) / w
	sy := float64(engine.ScreenH) / h
	if cover {
		s := math.Max(sx, sy)
		sx, sy = s, s
	}
	op := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
	op.GeoM.Translate(-w/2, -h/2)
	op.GeoM.Scale(sx, sy)
	op.GeoM.Translate(float64(engine.ScreenW)/2, float64(engine.ScreenH)/2)
	op.ColorScale.Scale(float32(tint[0]), float32(tint[1]), float32(tint[2]), float32(tint[3]))
	screen.DrawImage(img, op)
}

func (m *Module) drawParticles(screen *ebiten.Image, t float64) {
	if m.particleType == 0 || m.particleIntensity <= 0 || m.particles == nil {
		return
	}
	m.particles.Proj = m.proj
	root := m.particleRoots[m.particleType]
	rate := m.particleIntensity * 6
	if m.particleType == 3 {
		rate = m.particleIntensity * 32
	}
	stream, ok := m.particles.NewStream(root, root, [2]float64{}, m.particleStartT, rate)
	if !ok {
		return
	}
	stream.Wind[0] = m.particleWind
	stream.Prewarm = m.particlePrewarm
	m.particles.DrawStream(screen, stream, t)
}

func (m *Module) spawnHitParticles(p *pot, beat float64) {
	switch p.kind {
	case potKindComboEnd:
		pos := m.stageHitPos(5)
		m.addHitParticle(karateHitParticleBarrel, pos, beat, 0, [4]float64{}, false)
		m.addHitParticle(karateHitParticleOther, pos, beat, deterministicRot(beat, len(m.hitEffects)+1), [4]float64{}, false)
	case potKindKickBarrel:
		pos := m.stageHitPos(1)
		m.addHitParticle(karateHitParticleBarrel, pos, beat, -5*math.Pi/180, [4]float64{}, false)
		m.addHitParticle(karateHitParticleOther, pos, beat, deterministicRot(beat, len(m.hitEffects)+1), [4]float64{}, false)
	case potKindKickPayload:
		pos := m.stageCurvePoint(6, 0.5)
		root := karateHitParticleBomb
		if p.typ == hitBall {
			root = karateHitParticleOther
		}
		m.addHitParticle(root, pos, beat, 0, [4]float64{}, false)
	default:
		root, pos, rot, tint, hasTint := m.hitParticleForPot(p, beat)
		m.addHitParticle(root, pos, beat, rot, tint, hasTint)
	}
}

func (m *Module) hitParticleForPot(p *pot, beat float64) (root int, pos [2]float64, rot float64, tint [4]float64, hasTint bool) {
	pos = m.stageHitPos(1)
	switch p.kind {
	case potKindCombo3:
		pos = m.stageHitPos(2)
	case potKindCombo4:
		pos = m.stageHitPos(3)
	case potKindCombo5:
		pos = m.stageHitPos(4)
	}
	switch p.typ {
	case hitLightbulb:
		return karateHitParticleLight, pos, deterministicRot(beat, len(m.hitEffects)), p.tint, p.tint[3] > 0
	case hitRock:
		return karateHitParticleRock, pos, 0, [4]float64{}, false
	case hitBomb:
		return karateHitParticleBomb, pos, deterministicRot(beat, len(m.hitEffects)), [4]float64{}, false
	case hitBall, hitCooking, hitAlien, hitTaco:
		return karateHitParticleOther, pos, deterministicRot(beat, len(m.hitEffects)), [4]float64{}, false
	default:
		switch p.kind {
		case potKindCombo1, potKindCombo2, potKindCombo3, potKindCombo4, potKindCombo5:
			return karateHitParticleOther, pos, deterministicRot(beat, len(m.hitEffects)), [4]float64{}, false
		default:
			return karateHitParticlePot, pos, 0, m.starTint, true
		}
	}
}

func (m *Module) addHitParticle(rootIdx int, pos [2]float64, beat, rot float64, tint [4]float64, hasTint bool) {
	if m.particles == nil {
		return
	}
	root := m.hitParticleRoots[rootIdx]
	if root == "" {
		return
	}
	startT := m.ctx.Time()
	fx, ok := m.particles.NewEffect(root, root, pos, beat, startT)
	if !ok {
		return
	}
	fx.Rot = rot
	fx.Tint = tint
	fx.HasTint = hasTint
	m.hitEffects = append(m.hitEffects, fx)
}

func (m *Module) scheduleDelayedParticle(p *pot, startBeat float64, result potResult) {
	spec, ok := karateDelayedParticle(p, result)
	if !ok {
		return
	}
	eventBeat := startBeat + spec.delay
	m.ctx.At(eventBeat, func() {
		if p.result != result {
			return
		}
		pos := m.delayedParticlePos(p, spec, eventBeat)
		m.addHitParticle(spec.root, pos, eventBeat, 0, [4]float64{}, false)
		if spec.sound != "" {
			vol := spec.volume
			if vol <= 0 {
				vol = 1
			}
			m.ctx.SoundVol(spec.sound, vol)
		}
	})
}

func (m *Module) delayedParticlePos(p *pot, spec delayedParticleSpec, beat float64) [2]float64 {
	if spec.flight {
		x, y, _, _ := karatePotFlight(m.ctx.Assets.Stage, p.throwBeat, p.rot0, beat)
		return [2]float64{x, y}
	}
	return m.stageCurvePoint(spec.curve, spec.u)
}

func karateParticleRoot(as *kart.Assets, name string) string {
	suffix := "/" + name
	for _, ps := range as.Particles.Systems {
		if ps.Path == name || strings.HasSuffix(ps.Path, suffix) {
			return ps.Path
		}
	}
	return ""
}

func (m *Module) stageHitPos(idx int) [2]float64 {
	st := &m.ctx.Assets.Stage
	if idx >= 0 && idx < len(st.HitPositions) {
		return st.HitPositions[idx]
	}
	return st.HitPos
}

func (m *Module) stageCurvePoint(idx int, u float64) [2]float64 {
	st := &m.ctx.Assets.Stage
	if idx >= 0 && idx < len(st.ItemCurves) && len(st.ItemCurves[idx].Points) > 0 {
		p := kart.EvalBezier(st.ItemCurves[idx], clamp01(u))
		return [2]float64{p[0], p[1]}
	}
	return m.stageHitPos(1)
}

func karateDelayedParticle(p *pot, result potResult) (delayedParticleSpec, bool) {
	if p.typ != hitBomb {
		return delayedParticleSpec{}, false
	}
	switch result {
	case potHit:
		if p.kind == potKindKickPayload {
			// KickJustOrNg success switches KickBomb to ItemCurves[7] and lets it
			// travel for three beats before spawning HitParticles[6].
			return delayedParticleSpec{delay: 3, root: karateHitParticleBombSmall, curve: 7, u: 1}, true
		}
		// ItemHitEffect(Bomb) switches to ItemCurves[1]; Update later spawns
		// HitParticles[7] and plays the low-volume bombBreak tail.
		return delayedParticleSpec{delay: 1, root: karateHitParticleKick, curve: 1, u: 1, sound: "bombBreak", volume: 0.25}, true
	case potNG:
		if p.kind == potKindKickPayload {
			// Bad kick uses ItemCurves[8] and explodes as soon as that one-beat
			// curve reaches the end.
			return delayedParticleSpec{delay: 1, root: karateHitParticleKick, curve: 8, u: 1}, true
		}
		return delayedParticleSpec{delay: 1, root: karateHitParticleKick, curve: 6, u: 1}, true
	case potMiss:
		if p.kind == potKindKickPayload {
			return delayedParticleSpec{delay: 1.5, root: karateHitParticleKick, curve: 6, u: 1}, true
		}
		return delayedParticleSpec{delay: 2, root: karateHitParticleKick, flight: true}, true
	default:
		return delayedParticleSpec{}, false
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

func wordClip(kind int) string {
	idx := kind
	if kind >= 3 {
		idx = kind - 1
	}
	return fmt.Sprintf("word/Word%02d", clampInt(idx, 0, 6))
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

func legacyHitXWarningKind(e *riq.Entity) (int, bool) {
	if e.Data == nil {
		return 0, false
	}
	if _, ok := e.Data["type"]; !ok {
		return 0, false
	}
	typ := int(e.Float("type", 0))
	if typ < 7 {
		return typ + 1, true
	}
	return 0, true
}

func karateWordRig() kmdata.Rig {
	return kmdata.Rig{Nodes: []kmdata.Node{
		{Name: "Word", Path: "", Parent: -1, Scale: [2]float64{0.8, 0.8}},
		{Name: "Main", Path: "Main", Parent: 0, Pos: [2]float64{-4.4, 1}, Scale: [2]float64{4.5, 4.5}},
		{Name: "Sub", Path: "Sub", Parent: 0, Pos: [2]float64{-1.403, 1}, Scale: [2]float64{4.5, 4.5}},
		{Name: "Exclaim", Path: "Exclaim", Parent: 0, Pos: [2]float64{-1.08, 1}, Scale: [2]float64{4.5, 4.5}},
	}}
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

func mulColor(a, b [4]float64) [4]float64 {
	return [4]float64{a[0] * b[0], a[1] * b[1], a[2] * b[2], a[3] * b[3]}
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

func stringParam(e *riq.Entity, key, def string) string {
	if v, ok := e.Data[key].(string); ok && v != "" {
		return v
	}
	return def
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
