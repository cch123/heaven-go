// Package lovelizards ports Love Lizards' layered lizard animations, dynamic
// normal/d-pad input switching, reaction sounds, and background color timeline.
package lovelizards

import (
	"math"
	"math/rand"
	"sort"

	"github.com/hajimehoshi/ebiten/v2"

	"hsdemo/engine"
	"hsdemo/kart"
	"hsdemo/riq"
)

const (
	whoBoth = iota
	whoFemale
	whoMale
	whoNone

	actionNormal = 0
	actionPad    = 1
)

var (
	defaultBack   = hex(155, 91, 0)
	defaultMid    = hex(202, 138, 28)
	defaultFront  = hex(254, 198, 56)
	defaultShadow = hex(171, 100, 4)
)

type bgEvent struct {
	beat, length     float64
	back0, back1     [4]float64
	mid0, mid1       [4]float64
	front0, front1   [4]float64
	shadow0, shadow1 [4]float64
	ease             int
}

type colorEase struct {
	beat, length float64
	from, to     [4]float64
	ease         int
}

type femaleToggleEvent struct {
	beat float64
	on   bool
}

type danceEvent struct {
	beat, length float64
	triplet      bool
	react        bool
}

type Module struct {
	ctx  *engine.Ctx
	proj kart.Aff

	male, female, guide    string
	bgBack, bgMid, bgFront string
	shadowPaths            []string

	backEase, midEase, frontEase colorEase
	shadowEase                   colorEase
	bgEvents                     []bgEvent
	toggles                      []femaleToggleEvent
	dances                       []danceEvent

	rng          *rand.Rand
	dPad         bool
	hasMissed    bool
	maleBop      bool
	femaleBop    bool
	bopL         bool
	femaleOn     bool
	canBop       bool
	doingCue     bool
	guideActive  bool
	cueBeat      float64
	lastPulse    int
	lastBopPulse int
	activeInputs []*engine.Input
}

func New() engine.Module { return &Module{} }

func (m *Module) ID() string { return "loveLizards" }

func (m *Module) Load(ctx *engine.Ctx) error {
	m.ctx = ctx
	if err := ctx.LoadAssets("loveLizards"); err != nil {
		return err
	}
	m.proj = kart.Translate(engine.ScreenW/2, engine.ScreenH/2).Mul(kart.Scale(54, -54))
	m.male = roleOr(ctx, "MaleLizard", "MaleLizard")
	m.female = roleOr(ctx, "FemaleLizard", "FemaleLizard")
	m.guide = roleOr(ctx, "Guide", "GuideParent")
	m.bgBack = roleOr(ctx, "background1", "Back")
	m.bgMid = roleOr(ctx, "background2", "Mid")
	m.bgFront = roleOr(ctx, "background3", "Front")
	for _, n := range ctx.Assets.Rig.Nodes {
		if n.Mat == "Shadow" {
			m.shadowPaths = append(m.shadowPaths, n.Path)
		}
	}
	m.rng = rand.New(rand.NewSource(1))
	m.reset(0)
	return nil
}

func roleOr(ctx *engine.Ctx, key, fallback string) string {
	if p := ctx.Role(key); p != "" {
		return p
	}
	return fallback
}

func (m *Module) OnEvent(e *riq.Entity) {
	switch e.Datamodel {
	case "loveLizards/bop":
		m.scheduleBop(e)
	case "loveLizards/dance":
		m.scheduleDance(e)
	case "loveLizards/React":
		b := e.Beat
		m.ctx.At(b, func() { m.maleReact(b) })
	case "loveLizards/toggle female":
		on := boolParamDefault(e, "toggle", true)
		m.toggles = append(m.toggles, femaleToggleEvent{beat: e.Beat, on: on})
		m.ctx.At(e.Beat, func() { m.toggleFemale(on) })
	case "loveLizards/changeBG":
		m.bgEvents = append(m.bgEvents, bgEvent{
			beat: e.Beat, length: e.Length,
			back0: colorParam(e, "start", defaultBack), back1: colorParam(e, "end", defaultBack),
			mid0: colorParam(e, "start2", defaultMid), mid1: colorParam(e, "end2", defaultMid),
			front0: colorParam(e, "start3", defaultFront), front1: colorParam(e, "end3", defaultFront),
			shadow0: colorParam(e, "shadowStart", defaultShadow), shadow1: colorParam(e, "shadowEnd", defaultShadow),
			ease: intParam(e, "ease", 0),
		})
	}
}

func (m *Module) Ready() {
	sort.Slice(m.bgEvents, func(i, j int) bool { return m.bgEvents[i].beat < m.bgEvents[j].beat })
	sort.Slice(m.toggles, func(i, j int) bool { return m.toggles[i].beat < m.toggles[j].beat })
	sort.Slice(m.dances, func(i, j int) bool { return m.dances[i].beat < m.dances[j].beat })
}

func (m *Module) OnSwitch(beat float64) {
	m.reset(beat)
	for _, ev := range m.toggles {
		if ev.beat > beat {
			break
		}
		m.toggleFemale(ev.on)
	}
	m.persistBackground(beat)
	m.resumeDance(beat)
}

func (m *Module) Whiff(beat float64) { m.WhiffAction(beat, actionNormal) }

func (m *Module) WhiffAction(beat float64, action int) {
	if !m.femaleOn {
		return
	}
	switch {
	case action == actionNormal && !m.dPad:
		m.femaleSlide(beat, false)
		m.setInputs(m.activeInputs, true)
		m.hasMissed = true
	case action == actionPad && m.dPad:
		m.femaleSlide(beat, true)
		m.setInputs(m.activeInputs, false)
		m.hasMissed = true
	}
}

func (m *Module) Update(_ float64, beat float64) {
	if m.cueBeat > math.Inf(-1) {
		u := (beat - m.cueBeat) / 3
		m.doingCue = u >= 0 && u <= 1
		if u > 1 {
			m.canBop = true
		}
	}
	pulse := int(math.Floor(beat))
	if pulse > m.lastPulse {
		for b := m.lastPulse + 1; b <= pulse; b++ {
			if b != m.lastBopPulse {
				m.autoBop(float64(b))
			}
			m.bopL = !m.bopL
		}
		m.lastPulse = pulse
	}
}

func (m *Module) Draw(screen *ebiten.Image, _ float64, beat float64) {
	m.applyColors(beat)
	m.ctx.SampleScene(beat)
	m.ctx.Scene.Draw(screen, m.proj)
}

func (m *Module) reset(beat float64) {
	m.dPad = false
	m.hasMissed = false
	m.maleBop = true
	m.femaleBop = true
	m.bopL = false
	m.femaleOn = true
	m.canBop = true
	m.doingCue = false
	m.guideActive = true
	m.cueBeat = math.Inf(-1)
	m.lastPulse = int(math.Floor(beat)) - 1
	m.lastBopPulse = m.lastPulse
	m.activeInputs = nil
	m.backEase = colorEase{from: defaultBack, to: defaultBack}
	m.midEase = colorEase{from: defaultMid, to: defaultMid}
	m.frontEase = colorEase{from: defaultFront, to: defaultFront}
	m.shadowEase = colorEase{from: defaultShadow, to: defaultShadow}
	m.clearLayeredPlayers(beat)
	m.toggleFemale(true)
}

func (m *Module) clearLayeredPlayers(beat float64) {
	for _, layer := range []string{
		"male:body", "male:react",
		"female:body", "female:slide", "female:hold", "female:mouth",
	} {
		root := m.female
		if layer[0] == 'm' {
			root = m.male
		}
		m.ctx.Scene.PlayStateLayer(layer, root, "Idle", beat, m.ctx.SecPerBeat(beat))
	}
	m.ctx.Scene.PlayStateLayer("guide:body", m.guide, "GuideDown", beat, m.ctx.SecPerBeat(beat))
}

func (m *Module) scheduleBop(e *riq.Entity) {
	beat, length := e.Beat, e.Length
	who := intParam(e, "toggle", whoBoth)
	auto := intParam(e, "auto", whoNone)
	m.ctx.At(beat, func() {
		m.bopL = false
		m.setAutoBop(auto)
	})
	for i := 0; float64(i) < length; i++ {
		b := beat + float64(i)
		m.ctx.At(b, func() {
			m.bopOnce(who, b)
			m.bopL = !m.bopL
			m.lastBopPulse = int(math.Floor(b))
		})
	}
}

func (m *Module) setAutoBop(who int) {
	switch who {
	case whoBoth:
		m.maleBop, m.femaleBop = true, true
	case whoFemale:
		m.maleBop, m.femaleBop = false, true
	case whoMale:
		m.maleBop, m.femaleBop = true, false
	default:
		m.maleBop, m.femaleBop = false, false
	}
}

func (m *Module) autoBop(beat float64) {
	if !m.canBop {
		return
	}
	switch {
	case m.maleBop && m.femaleBop:
		m.bopOnce(whoBoth, beat)
	case m.maleBop:
		m.bopOnce(whoMale, beat)
	case m.femaleBop:
		m.bopOnce(whoFemale, beat)
	}
}

func (m *Module) bopOnce(who int, beat float64) {
	if !m.canBop {
		return
	}
	maleState := "MaleLizardBopL"
	femaleState := "FemaleLizardBopR"
	if m.bopL {
		maleState = "MaleLizardBopR"
		femaleState = "FemaleLizardBopL"
	}
	switch who {
	case whoBoth:
		m.play(m.male, "male:body", maleState, beat, 0.5)
		if m.femaleOn {
			m.play(m.female, "female:body", femaleState, beat, 0.5)
			m.play(m.female, "female:hold", "Idle", beat, 0.5)
		}
	case whoFemale:
		if m.femaleOn {
			m.play(m.female, "female:body", femaleState, beat, 0.5)
			m.play(m.female, "female:hold", "Idle", beat, 0.5)
		}
	case whoMale:
		m.play(m.male, "male:body", maleState, beat, 0.5)
	}
}

func (m *Module) scheduleDance(e *riq.Entity) {
	beat := e.Beat
	triplet := boolParam(e, "toggle")
	react := boolParamDefault(e, "react", true)
	m.dances = append(m.dances, danceEvent{beat: beat, length: e.Length, triplet: triplet, react: react})
	m.dance(beat, triplet, react)
}

func (m *Module) dance(beat float64, triplet, react bool) {
	first := 0.25
	second := 0.5
	if triplet {
		first = 0.33
		second = 0.67
	}
	cueFemale := false
	m.ctx.At(beat-1, func() {
		cueFemale = m.femaleOn
		m.play(m.male, "male:body", "MaleLizardStepR", beat-1, 0.5)
		if m.femaleOn {
			m.play(m.female, "female:body", "FemaleLizardStepR", beat-1, 0.5)
		}
		m.cueBeat = beat - 1
		m.canBop = false
	})
	m.ctx.At(beat-first, func() {
		m.play(m.male, "male:body", "MaleLizardStepL", beat-first, 0.5)
		if m.femaleOn {
			m.play(m.female, "female:body", "FemaleLizardStepL", beat-first, 0.5)
		}
	})
	m.ctx.At(beat, func() {
		m.play(m.male, "male:body", "MaleLizardStepR", beat, 0.5)
		if m.femaleOn {
			m.play(m.female, "female:body", "FemaleLizardStepR", beat, 0.5)
		}
	})
	m.ctx.At(beat+second, func() {
		m.play(m.male, "male:body", "MaleLizardStepL", beat+second, 0.5)
		if m.femaleOn {
			m.play(m.female, "female:body", "FemaleLizardStepL", beat+second, 0.5)
			m.play(m.female, "female:hold", "FemaleLizardHold", beat+second, 0.5)
		}
	})
	for _, b := range []float64{beat - first, beat, beat + second} {
		m.ctx.SoundAt(b, "cowbell", 1)
	}

	m.ctx.At(beat+1, func() {
		m.play(m.male, "male:body", "MaleLizardShakeL", beat+1, 0.5)
		m.play(m.female, "female:body", "Idle", beat+1, 0.5)
	})
	shakeBeats := []float64{beat + 1, beat + 1.25, beat + 1.5, beat + 1.75}
	if triplet {
		shakeBeats = []float64{beat + 1, beat + 1.33, beat + 1.67, beat + 2}
	}
	shakeStates := []string{"MaleLizardShakeL", "MaleLizardShakeR", "MaleLizardShakeL", "MaleLizardShakeR"}
	for i, b := range shakeBeats {
		state := shakeStates[i]
		if i > 0 {
			m.ctx.At(b, func() { m.play(m.male, "male:body", state, b, 0.5) })
		}
		m.ctx.SoundAt(b, "maleShake", 1)
	}
	m.ctx.At(beat+2, func() {
		if !m.doingCue {
			m.canBop = true
		}
	})

	inputs := make([]*engine.Input, 0, 4)
	targets := shakeBeats
	for _, target := range targets {
		in := m.ctx.ScheduleInputActionCond(target, actionNormal, func() bool { return cueFemale },
			func(float64, engine.Judgment) { m.onHit(inputs) },
			func() { m.hasMissed = true })
		inputs = append(inputs, in)
	}
	m.ctx.At(beat-1, func() {
		if cueFemale {
			m.activeInputs = inputs
			m.setInputs(inputs, m.dPad)
		}
	})
	end := beat + 2.5
	if triplet {
		end = beat + 3
	}
	m.ctx.At(end, func() {
		if cueFemale && react {
			m.maleReact(end)
		}
		if cueFemale && m.femaleOn {
			m.play(m.female, "female:hold", "FemaleLizardRelease", end, 0.5)
		}
		if sameInputSlice(m.activeInputs, inputs) {
			m.activeInputs = nil
		}
	})
}

func (m *Module) onHit(inputs []*engine.Input) {
	m.femaleSlide(m.ctx.Beat(), m.dPad)
	m.setInputs(inputs, !m.dPad)
}

func (m *Module) femaleSlide(beat float64, down bool) {
	if !m.femaleOn {
		return
	}
	slide, mouth, guide := "FemaleLizardSlideUp", "FemaleLizardMouthUp", "GuideUp"
	if down {
		slide, mouth, guide = "FemaleLizardSlideDown", "FemaleLizardMouthDown", "GuideDown"
	}
	m.play(m.female, "female:slide", slide, beat, 0.5)
	m.play(m.female, "female:mouth", mouth, beat, 0.5)
	if m.guideActive {
		m.play(m.guide, "guide:body", guide, beat, 0.5)
	}
	m.ctx.Sound(m.femaleSound())
}

func (m *Module) femaleSound() string {
	// UnityEngine.Random.Range(1, 6) with int arguments is max-exclusive.
	return "female" + string(rune('1'+m.rng.Intn(5)))
}

func (m *Module) setInputs(inputs []*engine.Input, setDPad bool) {
	m.dPad = setDPad
	action := actionNormal
	if setDPad {
		action = actionPad
	}
	for _, in := range inputs {
		if in != nil {
			in.Action = action
		}
	}
}

func (m *Module) maleReact(beat float64) {
	if m.hasMissed {
		m.play(m.male, "male:react", "MaleLizardYawn", beat, 0.5)
		m.ctx.Sound("maleYawn")
	} else {
		m.play(m.male, "male:react", "MaleLizardSmile", beat, 0.5)
		m.ctx.Sound("maleHeart")
	}
	m.hasMissed = false
}

func (m *Module) toggleFemale(on bool) {
	m.femaleOn = on
	m.guideActive = on
	m.ctx.Scene.SetActive(m.female, on)
	m.ctx.Scene.SetActive(m.guide, on)
}

func (m *Module) play(root, layer, state string, beat, scale float64) {
	m.ctx.Scene.PlayStateLayer(layer, root, state, beat, scale)
}

func (m *Module) persistBackground(beat float64) {
	m.backEase = colorEase{from: defaultBack, to: defaultBack}
	m.midEase = colorEase{from: defaultMid, to: defaultMid}
	m.frontEase = colorEase{from: defaultFront, to: defaultFront}
	m.shadowEase = colorEase{from: defaultShadow, to: defaultShadow}
	for _, ev := range m.bgEvents {
		if ev.beat >= beat {
			break
		}
		m.setBackground(ev)
	}
}

func (m *Module) setBackground(ev bgEvent) {
	m.backEase = colorEase{beat: ev.beat, length: ev.length, from: ev.back0, to: ev.back1, ease: ev.ease}
	m.midEase = colorEase{beat: ev.beat, length: ev.length, from: ev.mid0, to: ev.mid1, ease: ev.ease}
	m.frontEase = colorEase{beat: ev.beat, length: ev.length, from: ev.front0, to: ev.front1, ease: ev.ease}
	m.shadowEase = colorEase{beat: ev.beat, length: ev.length, from: ev.shadow0, to: ev.shadow1, ease: ev.ease}
}

func (m *Module) applyColors(beat float64) {
	m.persistBackground(beat + 1e-9)
	m.ctx.Scene.SetColorOver(m.bgBack, m.backEase.at(beat))
	m.ctx.Scene.SetColorOver(m.bgMid, m.midEase.at(beat))
	m.ctx.Scene.SetColorOver(m.bgFront, m.frontEase.at(beat))
	shadow := m.shadowEase.at(beat)
	for _, p := range m.shadowPaths {
		m.ctx.Scene.SetMaterialOver(p, [4]float64{1, 1, 1, 1}, shadow)
	}
}

func (m *Module) resumeDance(beat float64) {
	for i := len(m.dances) - 1; i >= 0; i-- {
		ev := m.dances[i]
		if ev.beat-1 < beat && ev.beat+ev.length >= beat {
			m.dance(ev.beat, ev.triplet, ev.react)
			return
		}
	}
}

func (e colorEase) at(beat float64) [4]float64 {
	if e.length <= 0 {
		return e.to
	}
	u := clamp01((beat - e.beat) / e.length)
	return [4]float64{
		engine.Ease(e.ease, e.from[0], e.to[0], u),
		engine.Ease(e.ease, e.from[1], e.to[1], u),
		engine.Ease(e.ease, e.from[2], e.to[2], u),
		engine.Ease(e.ease, e.from[3], e.to[3], u),
	}
}

func sameInputSlice(a, b []*engine.Input) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func boolParam(e *riq.Entity, key string) bool { return e.Float(key, 0) != 0 }

func boolParamDefault(e *riq.Entity, key string, def bool) bool {
	if _, ok := e.Data[key]; !ok {
		return def
	}
	return boolParam(e, key)
}

func intParam(e *riq.Entity, key string, def int) int { return int(e.Float(key, float64(def))) }

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

func hex(r, g, b uint8) [4]float64 {
	return [4]float64{float64(r) / 255, float64(g) / 255, float64(b) / 255, 1}
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
