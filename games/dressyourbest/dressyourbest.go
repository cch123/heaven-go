// Package dressyourbest ports Dress Your Best's listen/repeat interval,
// layered character faces, sewing-machine hit feedback, cameo walk, mapped
// light colors, and legacy background color event.
package dressyourbest

import (
	"math"
	"sort"

	"github.com/hajimehoshi/ebiten/v2"

	"hsdemo/engine"
	"hsdemo/kart"
	"hsdemo/riq"
)

const (
	charGirl = iota
	charMonkey
	charBoth
)

const (
	faceIdle = iota
	faceLooking
	faceHappy
	faceSad
)

const (
	lightIdle = iota
	lightRepeating
	lightCorrect
	lightIncorrect
)

const animScale = 0.5

var defaultBG = [4]float64{0.84, 0.58, 0.87, 1}

type bopEvt struct {
	beat, length float64
	characters   int
	auto, bop    bool
}

type intervalEvt struct {
	beat, length        float64
	autoPass, autoReact bool
}

type callEvt struct {
	beat float64
	sfx  int
}

type passEvt struct {
	beat      float64
	autoReact bool
}

type reactEvt struct {
	beat, length float64
}

type emotionEvt struct {
	beat      float64
	character int
	face      int
}

type cameoEvt struct {
	beat, length float64
	anim, ease   int
}

type bgEvt struct {
	beat, length float64
	start, end   [4]float64
	ease         int
}

type lightPair struct {
	inside, outside [4]float64
}

type Module struct {
	ctx  *engine.Ctx
	proj kart.Aff

	girl, monkey     string
	sewing, reaction string
	cameo            string
	newBG, oldBG     string
	lightPath        string
	lightMat         string
	lightStates      []lightPair

	bops      []bopEvt
	intervals []intervalEvt
	calls     []callEvt
	passes    []passEvt
	reacts    []reactEvt
	emotions  []emotionEvt
	cameos    []cameoEvt
	bgs       []bgEvt

	girlBop, monkeyBop bool
	girlFace           int
	monkeyFace         int
	startIntervalEnd   float64
	hitCount           int
	hasMissed          bool
	lastPulse          int

	activeCameo cameoEvt
	hasCameo    bool
}

func New() engine.Module {
	return &Module{
		girlBop: true, monkeyBop: true,
		lastPulse: math.MinInt,
	}
}

func (m *Module) ID() string { return "dressYourBest" }

func (m *Module) Load(ctx *engine.Ctx) error {
	m.ctx = ctx
	if err := ctx.LoadAssets("dressYourBest"); err != nil {
		return err
	}
	m.proj = kart.Translate(engine.ScreenW/2, engine.ScreenH/2).Mul(kart.Scale(54, -54))

	m.girl = roleOr(ctx, "girlAnim", "Girl")
	m.monkey = roleOr(ctx, "monkeyAnim", "Monkey")
	m.sewing = roleOr(ctx, "sewingAnim", "SewingMachine")
	m.reaction = roleOr(ctx, "reactionAnim", "Reaction")
	m.cameo = roleOr(ctx, "cameoAnim", "Background/Cameo")
	m.newBG = roleOr(ctx, "newBG", "Background")
	m.oldBG = roleOr(ctx, "bgSpriteRenderer", "Old BG Placeholder")
	m.lightPath = roleOr(ctx, "lightRenderer", "SewingMachine/Light")

	game := ctx.Assets.Extra.Components["game"]
	m.lightMat = game.Refs["lightMaterialTemplate"]
	if m.lightMat == "" {
		m.lightMat = "LightMat"
	}
	for _, item := range game.Lists["lightStates"] {
		m.lightStates = append(m.lightStates, lightPair{
			inside:  colorFromNums(item.Nums, "inside"),
			outside: colorFromNums(item.Nums, "outside"),
		})
	}
	m.ctx.Scene.SetActive(m.cameo, false)
	m.ctx.Scene.SetActive(m.reaction, false)
	m.setLight(lightIdle)
	m.applyBackground(0)
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
	case "dressYourBest/bop":
		ev := bopEvt{
			beat: e.Beat, length: e.Length,
			characters: int(e.Float("characters", charBoth)),
			auto:       boolParamDefault(e, "auto", true),
			bop:        boolParamDefault(e, "bop", true),
		}
		m.bops = append(m.bops, ev)
		m.ctx.At(ev.beat, func() { m.toggleBopping(ev.characters, ev.auto) })
		if ev.bop {
			for i := 0.0; i < ev.length; i++ {
				b := ev.beat + i
				m.ctx.At(b, func() { m.doBop(ev.characters, b) })
			}
		}
	case "dressYourBest/start interval":
		m.intervals = append(m.intervals, intervalEvt{
			beat: e.Beat, length: e.Length,
			autoPass:  boolParamDefault(e, "autoPass", true),
			autoReact: boolParamDefault(e, "autoReact", true),
		})
	case "dressYourBest/monkey call":
		m.calls = append(m.calls, callEvt{beat: e.Beat, sfx: int(e.Float("callSfx", 0))})
	case "dressYourBest/pass turn":
		ev := passEvt{beat: e.Beat, autoReact: boolParamDefault(e, "auto", true)}
		m.passes = append(m.passes, ev)
		m.ctx.SoundAt(ev.beat, "pass_turn", 1)
	case "dressYourBest/interval react":
		m.reacts = append(m.reacts, reactEvt{beat: e.Beat, length: e.Length})
	case "dressYourBest/change emotion":
		m.emotions = append(m.emotions, emotionEvt{
			beat: e.Beat, character: int(e.Float("character", charGirl)), face: int(e.Float("face", faceIdle)),
		})
	case "dressYourBest/cameo":
		m.cameos = append(m.cameos, cameoEvt{
			beat: e.Beat, length: e.Length,
			anim: int(e.Float("animation", 0)) + 1, ease: int(e.Float("ease", 0)),
		})
	case "dressYourBest/background appearance":
		m.bgs = append(m.bgs, bgEvt{
			beat: e.Beat, length: e.Length, ease: int(e.Float("ease", 0)),
			start: colorParam(e, "start", defaultBG),
			end:   colorParam(e, "end", defaultBG),
		})
	}
}

func (m *Module) Ready() {
	sort.Slice(m.bops, func(i, j int) bool { return m.bops[i].beat < m.bops[j].beat })
	sort.Slice(m.intervals, func(i, j int) bool { return m.intervals[i].beat < m.intervals[j].beat })
	sort.Slice(m.calls, func(i, j int) bool { return m.calls[i].beat < m.calls[j].beat })
	sort.Slice(m.bgs, func(i, j int) bool { return m.bgs[i].beat < m.bgs[j].beat })
	for _, ev := range m.intervals {
		ev := ev
		m.queueStartInterval(ev, math.Inf(-1))
	}
	for _, ev := range m.passes {
		ev := ev
		m.ctx.At(ev.beat, func() {
			if iv, ok := m.lastIntervalBefore(ev.beat); ok {
				m.passTurn(ev.beat, ev.autoReact, iv)
			}
		})
	}
	for _, ev := range m.reacts {
		ev := ev
		m.ctx.At(ev.beat, func() { m.intervalReact(ev.beat, ev.length) })
	}
	for _, ev := range m.emotions {
		ev := ev
		m.ctx.At(ev.beat, func() { m.changeEmotion(ev.character, ev.face, ev.beat) })
	}
	for _, ev := range m.cameos {
		ev := ev
		m.ctx.At(ev.beat, func() {
			m.activeCameo, m.hasCameo = ev, true
			m.ctx.Scene.SetActive(m.cameo, true)
		})
	}
}

func (m *Module) OnSwitch(beat float64) {
	m.persistBop(beat)
	m.applyBackground(beat)
	m.setLight(lightIdle)
	m.lastPulse = int(math.Floor(beat)) - 1
	for _, ev := range m.intervals {
		if beat >= ev.beat && beat <= ev.beat+ev.length {
			m.startIntervalEnd = m.intervalEnd(ev)
			m.changeEmotion(charGirl, faceLooking, beat)
			break
		}
	}
}

func (m *Module) Whiff(beat float64) { m.WhiffAction(beat, 0) }

func (m *Module) WhiffAction(beat float64, _ int) {
	m.changeEmotion(charGirl, faceSad, beat)
	m.playSewing("Miss", beat)
	m.ctx.Sound("whiff_hit")
	if beat >= m.startIntervalEnd {
		m.hasMissed = true
	}
	// The engine already records an unexpected press as a whiff.
}

func (m *Module) Update(t, beat float64) {
	m.applyBackground(beat)
	m.updateCameo(beat)
	m.pulseBeats(beat)
}

func (m *Module) Draw(screen *ebiten.Image, t, beat float64) {
	m.ctx.SampleScene(beat)
	m.ctx.Scene.Draw(screen, m.proj)
}

func (m *Module) queueStartInterval(ev intervalEvt, startBeat float64) {
	needed := m.callsIn(ev)
	if len(needed) == 0 || startBeat >= ev.beat+ev.length {
		return
	}
	m.ctx.At(ev.beat, func() {
		m.startIntervalEnd = m.intervalEnd(ev)
		m.changeEmotion(charGirl, faceLooking, ev.beat)
	})
	for _, call := range needed {
		call := call
		if call.beat < startBeat {
			continue
		}
		m.ctx.SoundAt(call.beat, call.sound(), 1)
		m.ctx.At(call.beat, func() {
			m.ctx.Scene.PlayLayer(m.monkey+"/body", m.monkey, "Monkey/MonkeyCall", call.beat, animScale)
			// C# sets monkeyFaceCurrent to Idle before playing CallFace so the
			// later reaction reset cannot mistake this transient face for Happy/Sad.
			m.monkeyFace = faceIdle
			m.ctx.Scene.PlayLayer(m.monkey+"/face", m.monkey, "Faces/MonkeyCallFace", call.beat, animScale)
		})
	}
	if ev.autoPass {
		passBeat := ev.beat + ev.length
		m.ctx.SoundAt(passBeat, "pass_turn", 1)
		m.ctx.At(passBeat, func() { m.passTurn(passBeat, ev.autoReact, ev) })
	}
}

func (m *Module) passTurn(beat float64, autoReact bool, ev intervalEvt) {
	needed := m.callsIn(ev)
	if len(needed) == 0 {
		return
	}
	m.changeEmotion(charGirl, faceIdle, beat)
	m.setLight(lightRepeating)
	m.hitCount = 0
	for _, call := range needed {
		call := call
		target := beat + (call.beat - ev.beat) + 1
		m.ctx.ScheduleInputAny(target, func(state float64, _ engine.Judgment) {
			m.onHit(state, m.ctx.Beat())
		}, m.onMiss)
	}
	if autoReact {
		reactBeat := (beat * 2) - ev.beat + 1
		m.ctx.At(reactBeat, func() { m.intervalReact(reactBeat, 1) })
	}
}

func (m *Module) callsIn(ev intervalEvt) []callEvt {
	var out []callEvt
	for _, call := range m.calls {
		if call.beat >= ev.beat && call.beat <= ev.beat+ev.length {
			out = append(out, call)
		}
	}
	return out
}

func (m *Module) intervalEnd(ev intervalEvt) float64 {
	end := ev.beat + ev.length
	for _, call := range m.callsIn(ev) {
		if math.Abs(call.beat-end) < 1e-9 {
			return end + 1
		}
	}
	return end
}

func (m *Module) lastIntervalBefore(beat float64) (intervalEvt, bool) {
	var out intervalEvt
	ok := false
	for _, ev := range m.intervals {
		if ev.beat+ev.length >= beat {
			break
		}
		out, ok = ev, true
	}
	return out, ok
}

func (m *Module) onHit(state, beat float64) {
	m.hitCount++
	m.ctx.Sound("hit_1")
	m.ctx.SoundPitch("hit_2", 1, math.Exp2(float64(m.hitCount)/12))
	if math.Abs(state) >= 1 {
		m.ctx.SoundVol("common_nearMiss", 2)
		m.playSewing("Miss", beat)
		m.hasMissed = true
		return
	}
	m.playSewing("Hit", beat)
}

func (m *Module) onMiss() {
	m.hitCount = 0
	m.hasMissed = true
}

func (m *Module) intervalReact(beat, length float64) {
	reaction := faceHappy
	light := lightCorrect
	sound := "correct"
	anim := "Correct"
	if m.hasMissed || m.hitCount <= 0 {
		reaction, light, sound, anim = faceSad, lightIncorrect, "incorrect", "Incorrect"
	}
	m.changeEmotion(charBoth, reaction, beat)
	m.setLight(light)
	m.ctx.Scene.SetActive(m.reaction, true)
	m.ctx.Scene.PlayLayer(m.reaction+"/react", m.reaction, "Reaction/Reaction"+anim, beat, animScale)
	m.ctx.Sound(sound)
	m.ctx.At(beat+length, func() {
		m.ctx.Scene.SetActive(m.reaction, false)
		if m.girlFace == reaction {
			m.changeEmotion(charGirl, faceIdle, beat+length)
		}
		if m.monkeyFace == reaction {
			m.changeEmotion(charMonkey, faceIdle, beat+length)
		}
		m.setLight(lightIdle)
	})
	m.hasMissed = false
}

func (m *Module) persistBop(beat float64) {
	m.girlBop, m.monkeyBop = true, true
	for _, ev := range m.bops {
		if ev.beat > beat {
			break
		}
		m.toggleBopping(ev.characters, ev.auto)
	}
}

func (m *Module) toggleBopping(characters int, on bool) {
	if characters == charGirl || characters == charBoth {
		m.girlBop = on
	}
	if characters == charMonkey || characters == charBoth {
		m.monkeyBop = on
	}
}

func (m *Module) doBop(characters int, beat float64) {
	if characters == charGirl || characters == charBoth {
		m.ctx.Scene.PlayLayer(m.girl+"/body", m.girl, "Girl/GirlBop", beat, animScale)
	}
	if characters == charMonkey || characters == charBoth {
		m.ctx.Scene.PlayLayer(m.monkey+"/body", m.monkey, "Monkey/MonkeyBop", beat, animScale)
	}
}

func (m *Module) pulseBeats(beat float64) {
	pulse := int(math.Floor(beat + 1e-9))
	if m.lastPulse == math.MinInt {
		m.lastPulse = pulse - 1
	}
	if pulse <= m.lastPulse {
		return
	}
	for p := m.lastPulse + 1; p <= pulse; p++ {
		if p < 0 {
			continue
		}
		b := float64(p)
		if m.girlBop {
			m.ctx.Scene.PlayLayer(m.girl+"/body", m.girl, "Girl/GirlBop", b, animScale)
		}
		if m.monkeyBop && b >= m.startIntervalEnd {
			m.ctx.Scene.PlayLayer(m.monkey+"/body", m.monkey, "Monkey/MonkeyBop", b, animScale)
		}
	}
	m.lastPulse = pulse
}

func (m *Module) changeEmotion(character, face int, beat float64) {
	if character == charGirl || character == charBoth {
		m.girlFace = face
		m.ctx.Scene.PlayLayer(m.girl+"/face", m.girl, girlFaceClip(face), beat, animScale)
	}
	if character == charMonkey || character == charBoth {
		m.monkeyFace = face
		m.ctx.Scene.PlayLayer(m.monkey+"/face", m.monkey, monkeyFaceClip(face), beat, animScale)
	}
}

func girlFaceClip(face int) string {
	switch face {
	case faceLooking:
		return "Faces/GirlLooking"
	case faceHappy:
		return "Faces/GirlCorrect"
	case faceSad:
		return "Faces/GirlIncorrect"
	default:
		return "Faces/GirlDefault"
	}
}

func monkeyFaceClip(face int) string {
	switch face {
	case faceHappy:
		return "Faces/MonkeyCorrect"
	case faceSad:
		return "Faces/MonkeyIncorrect"
	default:
		return "Faces/MonkeyDefault"
	}
}

func (m *Module) playSewing(state string, beat float64) {
	clip := "SewingMachine/Sew"
	if state == "Miss" {
		clip = "SewingMachine/SewNot"
	}
	m.ctx.Scene.PlayLayer(m.sewing+"/hit", m.sewing, clip, beat, animScale)
}

func (m *Module) setLight(state int) {
	if state < 0 || state >= len(m.lightStates) {
		return
	}
	pair := m.lightStates[state]
	pal := kart.DefaultPalette()
	pal.Alpha = pair.inside
	pal.Fill = pair.outside
	// Unity instantiates lightMaterialTemplate onto lightRenderer in Awake.
	// Keep the same per-renderer boundary here so the mapped light colors never
	// leak to other sprites that happen to share extraction-time material data.
	m.ctx.Scene.SetPaletteOver(m.lightPath, pal)
}

func (m *Module) updateCameo(beat float64) {
	if !m.hasCameo || m.activeCameo.length <= 0 {
		m.ctx.Scene.SetActive(m.cameo, false)
		return
	}
	ev := m.activeCameo
	u := (beat - ev.beat) / ev.length
	if u >= 1 {
		m.hasCameo = false
		m.ctx.Scene.SetActive(m.cameo, false)
		return
	}
	v := engine.Ease(ev.ease, 0, 1, u)
	m.ctx.Scene.SetActive(m.cameo, true)
	m.ctx.Scene.PlayFrozen(m.cameo, cameoState(ev.anim), v)
}

func cameoState(n int) string {
	if n < 1 {
		n = 1
	}
	if n > 5 {
		n = 5
	}
	return "CameoWalk" + string(rune('0'+n))
}

func (m *Module) applyBackground(beat float64) {
	c := defaultBG
	useLegacy := false
	for _, ev := range m.bgs {
		if ev.beat > beat {
			break
		}
		useLegacy = true
		u := 1.0
		if ev.length > 0 && beat < ev.beat+ev.length {
			u = (beat - ev.beat) / ev.length
		}
		c = easeColor(ev.ease, ev.start, ev.end, u)
	}
	if useLegacy {
		m.ctx.Scene.SetActive(m.newBG, false)
		m.ctx.Scene.SetColorOver(m.oldBG, c)
	}
}

func easeColor(ease int, a, b [4]float64, u float64) [4]float64 {
	return [4]float64{
		engine.Ease(ease, a[0], b[0], u),
		engine.Ease(ease, a[1], b[1], u),
		engine.Ease(ease, a[2], b[2], u),
		engine.Ease(ease, a[3], b[3], u),
	}
}

func (call callEvt) sound() string {
	if call.sfx == 1 {
		return "monkey_call_2"
	}
	return "monkey_call_1"
}

func colorFromNums(nums map[string]float64, prefix string) [4]float64 {
	return [4]float64{nums[prefix+".r"], nums[prefix+".g"], nums[prefix+".b"], nums[prefix+".a"]}
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

func num(v any, def float64) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case int:
		return float64(x)
	}
	return def
}
