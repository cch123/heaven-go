// Package claptrap ports Clap Trap's delayed hand strike cue, doll reactions,
// spotlight/shadow state, runtime recolors, and whiff cooldown.
package claptrap

import (
	"image/color"
	"math"
	"math/rand"
	"sort"

	"github.com/hajimehoshi/ebiten/v2"

	"hsdemo/engine"
	"hsdemo/kart"
	"hsdemo/riq"
)

const (
	clapTypeHand = iota
	clapTypePaw
	clapTypeGreenOnion
)

var (
	defaultBG        = [4]float64{1, 239.0 / 255.0, 17.0 / 255.0, 1}
	defaultLeft      = [4]float64{16.0 / 255.0, 181.0 / 255.0, 231.0 / 255.0, 1}
	defaultRight     = [4]float64{236.0 / 255.0, 116.0 / 255.0, 15.0 / 255.0, 1}
	defaultSpotlight = [4]float64{1, 1, 1, 1}
	blackBG          = [4]float64{0, 0, 0, 1}
)

type clapEvt struct {
	beat, length float64
	typ          int
	spotlight    bool
}

type dollEvt struct {
	beat    float64
	animate int
}

type forceEvt struct {
	beat  float64
	force bool
}

type bgEvt struct {
	beat, length float64
	c0, c1       [4]float64
	ease         int
}

type handEvt struct {
	beat             float64
	left, right      [4]float64
	spotTop, spotBot [4]float64
	spotGlow         [4]float64
}

type swordInst struct {
	inst       *kart.Instance
	expireBeat float64
}

type Module struct {
	ctx  *engine.Ctx
	proj kart.Aff

	claps []clapEvt
	dolls []dollEvt
	force []forceEvt
	bgs   []bgEvt
	hands []handEvt

	bgPath        string
	stageLeft     string
	stageRight    string
	stageLeftRim  string
	stageRightRim string
	spotlightPath string
	dollPath      string
	headPath      string
	armsPath      string
	bodyPath      string
	effectPath    string
	swordObjPath  string
	spotMat       string
	shadowPaths   []string

	swordT *kart.Template
	swords []*swordInst

	currentSpotlightClaps int
	forceSpotlight        bool
	spotlightActive       bool
	bodyLit               bool
	canClapUntil          float64
}

func New() engine.Module { return &Module{spotMat: "SpotlightMaterial"} }

func (m *Module) ID() string { return "clapTrap" }

func (m *Module) Load(ctx *engine.Ctx) error {
	m.ctx = ctx
	if err := ctx.LoadAssets("clapTrap"); err != nil {
		return err
	}
	m.proj = kart.Translate(engine.ScreenW/2, engine.ScreenH/2).Mul(kart.Scale(54, -54))
	m.bgPath = roleOr(ctx, "bg", "background")
	m.stageLeft = roleOr(ctx, "stageLeft", "clapTrapDoll/arms/armLeft/glove")
	m.stageRight = roleOr(ctx, "stageRight", "clapTrapDoll/arms/armRight/glove")
	m.stageLeftRim = roleOr(ctx, "stageLeftRim", "clapTrapDoll/arms/armLeft/glove/gloveRim")
	m.stageRightRim = roleOr(ctx, "stageRightRim", "clapTrapDoll/arms/armRight/glove/gloveRim")
	m.spotlightPath = roleOr(ctx, "spotlight", "spotlight")
	m.dollPath = roleOr(ctx, "doll", "clapTrapDoll")
	m.headPath = roleOr(ctx, "dollHead", "clapTrapDoll/dollHead")
	m.armsPath = roleOr(ctx, "dollArms", "clapTrapDoll/arms")
	m.bodyPath = roleOr(ctx, "dollBody", "clapTrapDoll/body")
	m.effectPath = roleOr(ctx, "clapEffect", "clapTrapDoll/clapEffect")
	m.swordObjPath = roleOr(ctx, "swordObj", "sword/sword")
	m.shadowPaths = []string{
		roleOr(ctx, "shadowHead", "clapTrapDoll/dollHead/head/shadow"),
		roleOr(ctx, "shadowLeftArm", "clapTrapDoll/arms/armLeft/shadow"),
		roleOr(ctx, "shadowLeftGlove", "clapTrapDoll/arms/armLeft/glove/shadow"),
		roleOr(ctx, "shadowLeftGloveRim", "clapTrapDoll/arms/armLeft/glove/gloveRim/shadow"),
		roleOr(ctx, "shadowRightArm", "clapTrapDoll/arms/armRight/shadow"),
		roleOr(ctx, "shadowRightGlove", "clapTrapDoll/arms/armRight/glove/shadow"),
		roleOr(ctx, "shadowRightGloveRim", "clapTrapDoll/arms/armRight/glove/gloveRim/shadow"),
	}
	if game := ctx.Assets.Extra.Components["game"]; game.Refs != nil {
		if mat := game.Refs["spotlightMaterial"]; mat != "" {
			m.spotMat = mat
		}
	}
	m.swordT = kart.NewTemplate(ctx.Assets, m.swordObjPath)
	ctx.Scene.SetActive(m.swordObjPath, false)
	m.initAnimators(0)
	m.applyHandColors(defaultLeft, defaultRight, defaultSpotlight, defaultBG, defaultSpotlight)
	m.setSpotlight(false, 0)
	return nil
}

func roleOr(ctx *engine.Ctx, key, fallback string) string {
	if p := ctx.Role(key); p != "" {
		return p
	}
	return fallback
}

func (m *Module) initAnimators(beat float64) {
	ts := m.ctx.SecPerBeat(beat)
	for _, p := range []string{m.dollPath, m.headPath, m.armsPath, m.bodyPath, m.effectPath} {
		m.ctx.Scene.PlayDefaultState(p, beat, ts)
	}
	m.bodyLit = false
}

func (m *Module) OnEvent(e *riq.Entity) {
	switch e.Datamodel {
	case "clapTrap/clap":
		length := e.Length
		if length <= 0 {
			length = 1
		}
		m.claps = append(m.claps, clapEvt{
			beat: e.Beat, length: length,
			typ:       int(e.Float("sword", clapTypeHand)),
			spotlight: boolDefault(e, "spotlight", true),
		})
	case "clapTrap/doll animations":
		m.dolls = append(m.dolls, dollEvt{beat: e.Beat, animate: int(e.Float("animate", 1))})
	case "clapTrap/spotlight":
		m.force = append(m.force, forceEvt{beat: e.Beat, force: boolDefault(e, "force", true)})
	case "clapTrap/background color":
		m.bgs = append(m.bgs, bgEvt{
			beat: e.Beat, length: e.Length, ease: int(e.Float("ease", 1)),
			c0: colorParam(e, "bgColor", defaultBG),
			c1: colorParam(e, "bgColorEnd", defaultBG),
		})
	case "clapTrap/hand color":
		m.hands = append(m.hands, handEvt{
			beat:     e.Beat,
			left:     colorParam(e, "left", defaultLeft),
			right:    colorParam(e, "right", defaultRight),
			spotTop:  colorParam(e, "spotlightTop", defaultSpotlight),
			spotBot:  colorParam(e, "spotlightBottom", defaultBG),
			spotGlow: colorParam(e, "spotlightGlow", defaultSpotlight),
		})
	}
}

func (m *Module) Ready() {
	sort.SliceStable(m.claps, func(i, j int) bool { return m.claps[i].beat < m.claps[j].beat })
	sort.SliceStable(m.dolls, func(i, j int) bool { return m.dolls[i].beat < m.dolls[j].beat })
	sort.SliceStable(m.force, func(i, j int) bool { return m.force[i].beat < m.force[j].beat })
	sort.SliceStable(m.bgs, func(i, j int) bool { return m.bgs[i].beat < m.bgs[j].beat })
	sort.SliceStable(m.hands, func(i, j int) bool { return m.hands[i].beat < m.hands[j].beat })

	for _, ev := range m.claps {
		ev := ev
		m.scheduleClap(ev)
	}
	for _, ev := range m.dolls {
		ev := ev
		m.ctx.At(ev.beat, func() { m.dollAnimation(ev.beat, ev.animate) })
	}
	for _, ev := range m.force {
		ev := ev
		m.ctx.At(ev.beat, func() {
			m.forceSpotlight = ev.force
			if ev.force {
				m.setSpotlight(true, ev.beat)
			}
		})
	}
}

func (m *Module) OnSwitch(beat float64) {
	m.initAnimators(beat)
	m.applyPersistent(beat)
	m.setSpotlight(m.spotlightActive, beat)
}

func (m *Module) Whiff(beat float64) {
	if beat < m.canClapUntil {
		return
	}
	m.ctx.Sound("clap")
	m.play(m.armsPath, "ArmsWhiff", beat)
	m.play(m.effectPath, "ClapEffect", beat)
	m.canClapUntil = beat + 0.4
}

func (m *Module) Update(_ float64, beat float64) {
	if m.spotlightActive && m.currentSpotlightClaps == 0 && !m.forceSpotlight {
		m.setSpotlight(false, beat)
	}
	dst := m.swords[:0]
	for _, sw := range m.swords {
		if beat < sw.expireBeat {
			dst = append(dst, sw)
		}
	}
	m.swords = dst
}

func (m *Module) Draw(screen *ebiten.Image, _ float64, beat float64) {
	bg := m.bgAt(beat)
	if m.spotlightActive {
		bg = blackBG
	}
	screen.Fill(rgba(bg))
	m.ctx.Scene.SetColorOver(m.bgPath, bg)
	h := m.handAt(beat)
	m.applyHandColors(h.left, h.right, h.spotTop, h.spotBot, h.spotGlow)
	m.ctx.SampleScene(beat)
	for _, sw := range m.swords {
		if sw.inst != nil && beat < sw.expireBeat {
			sw.inst.Queue(m.ctx.Scene, beat, kart.Identity(), 0)
		}
	}
	m.ctx.Scene.Draw(screen, m.proj)
}

func (m *Module) scheduleClap(ev clapEvt) {
	m.ctx.SoundAt(ev.beat, "donk", 1)
	m.ctx.SoundAt(ev.beat+ev.length, "donk", 1)
	m.ctx.SoundAt(ev.beat+ev.length*2, "donk", 1)
	m.ctx.SoundAt(ev.beat+ev.length*3.5, "whiff", 1)
	m.ctx.At(ev.beat, func() {
		if ev.spotlight {
			m.currentSpotlightClaps++
			m.setSpotlight(true, ev.beat)
		}
	})
	target := clapTarget(ev.beat, ev.length)
	m.ctx.ScheduleInput(target,
		func(state float64, _ engine.Judgment) { m.hitClap(ev, target, state) },
		func() { m.missClap(ev, target) })
}

func clapTarget(beat, length float64) float64 { return beat + length*4 }

func (m *Module) hitClap(ev clapEvt, beat, state float64) {
	if state >= 1 || state <= -1 {
		m.ctx.Sound(randomChoice("barely", 2, beat, 11))
		m.play(m.headPath, "HeadBarely", beat)
	} else {
		m.ctx.Sound(randomChoice("goodClap", 4, beat, 23))
		m.play(m.headPath, "HeadHit", beat)
		if math.Abs(state) <= 0.2 {
			m.ctx.Sound("clapAce")
		} else {
			m.ctx.Sound("clapGood")
		}
	}
	m.play(m.armsPath, "ArmsHit", beat)
	m.play(m.dollPath, "DollHit", beat)
	m.play(m.effectPath, "ClapEffect", beat)
	m.showSword(ev.typ, true, beat)
	m.finishSpotlightClap(ev.spotlight)
}

func (m *Module) missClap(ev clapEvt, beat float64) {
	m.ctx.Sound("miss")
	m.play(m.headPath, "HeadMiss", beat)
	m.play(m.armsPath, "ArmsMiss", beat)
	m.play(m.dollPath, "DollMiss", beat)
	m.showSword(ev.typ, false, beat)
	m.finishSpotlightClap(ev.spotlight)
}

func (m *Module) finishSpotlightClap(active bool) {
	if !active {
		return
	}
	if m.currentSpotlightClaps > 0 {
		m.currentSpotlightClaps--
	}
}

func (m *Module) showSword(typ int, hit bool, beat float64) {
	if m.swordT == nil {
		return
	}
	inst := m.swordT.NewInstance()
	suffix := "Miss"
	if hit {
		suffix = "Hit"
	}
	inst.PlayState("", "sword"+clapTypeName(typ)+suffix, beat, 0.5)
	m.swords = append(m.swords, &swordInst{inst: inst, expireBeat: beat + 1.4})
}

func clapTypeName(typ int) string {
	switch typ {
	case clapTypePaw:
		return "Paw"
	case clapTypeGreenOnion:
		return "GreenOnion"
	default:
		return "Hand"
	}
}

func (m *Module) dollAnimation(beat float64, animate int) {
	switch animate {
	case 0:
		m.play(m.headPath, "HeadIdle", beat)
	case 1:
		m.play(m.headPath, "HeadBreatheIn", beat)
		m.ctx.Sound("deepInhale")
	case 2:
		m.play(m.headPath, "HeadBreatheOut", beat)
		m.ctx.Sound(randomChoice("deepExhale", 2, beat, 37))
	case 3:
		m.play(m.headPath, "HeadTalk", beat)
	}
}

func (m *Module) play(path, state string, beat float64) {
	m.ctx.Scene.PlayState(path, state, beat, 0.5)
}

func (m *Module) setSpotlight(active bool, beat float64) {
	m.spotlightActive = active
	m.ctx.Scene.SetActive(m.spotlightPath, active)
	for _, p := range m.shadowPaths {
		m.ctx.Scene.SetActive(p, active)
	}
	m.setBodyLit(active, beat)
}

func (m *Module) setBodyLit(lit bool, beat float64) {
	if m.bodyLit == lit {
		return
	}
	m.bodyLit = lit
	if lit {
		m.play(m.bodyPath, "BodyIdleLit", beat)
	} else {
		m.play(m.bodyPath, "BodyIdle", beat)
	}
}

func (m *Module) applyPersistent(beat float64) {
	force := false
	for _, ev := range m.force {
		if ev.beat > beat {
			break
		}
		force = ev.force
	}
	m.forceSpotlight = force
	if force {
		m.spotlightActive = true
	} else if m.currentSpotlightClaps == 0 {
		m.spotlightActive = false
	}
	h := m.handAt(beat)
	m.applyHandColors(h.left, h.right, h.spotTop, h.spotBot, h.spotGlow)
}

func (m *Module) applyHandColors(left, right, top, bottom, glow [4]float64) {
	m.ctx.Scene.SetColorOver(m.stageLeft, left)
	m.ctx.Scene.SetColorOver(m.stageLeftRim, left)
	m.ctx.Scene.SetColorOver(m.stageRight, right)
	m.ctx.Scene.SetColorOver(m.stageRightRim, right)
	m.ctx.Scene.SetPaletteFor(m.spotMat, kart.Palette{Alpha: top, Fill: glow, Outline: bottom})
}

func (m *Module) bgAt(beat float64) [4]float64 {
	out := defaultBG
	for _, ev := range m.bgs {
		if ev.beat > beat {
			break
		}
		u := 1.0
		if ev.length > 0 && beat < ev.beat+ev.length {
			u = clamp01((beat - ev.beat) / ev.length)
		}
		out = easeColor(ev.ease, ev.c0, ev.c1, u)
	}
	return out
}

func (m *Module) handAt(beat float64) handEvt {
	out := handEvt{
		left: defaultLeft, right: defaultRight,
		spotTop: defaultSpotlight, spotBot: defaultBG, spotGlow: defaultSpotlight,
	}
	for _, ev := range m.hands {
		if ev.beat > beat {
			break
		}
		out = ev
	}
	return out
}

func easeColor(ease int, a, b [4]float64, u float64) [4]float64 {
	return [4]float64{
		engine.Ease(ease, a[0], b[0], u),
		engine.Ease(ease, a[1], b[1], u),
		engine.Ease(ease, a[2], b[2], u),
		engine.Ease(ease, a[3], b[3], u),
	}
}

func rgba(c [4]float64) color.NRGBA {
	return color.NRGBA{
		R: byte(clamp01(c[0]) * 255),
		G: byte(clamp01(c[1]) * 255),
		B: byte(clamp01(c[2]) * 255),
		A: byte(clamp01(c[3]) * 255),
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

func randomChoice(prefix string, n int, beat float64, salt int64) string {
	if n <= 1 {
		return prefix + "1"
	}
	r := rand.New(rand.NewSource(int64(beat*1000) + salt))
	return prefix + string(rune('1'+r.Intn(n)))
}

func boolParam(e *riq.Entity, key string) bool { return e.Float(key, 0) != 0 }

func boolDefault(e *riq.Entity, key string, def bool) bool {
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
	mm, ok := v.(map[string]any)
	if !ok {
		return def
	}
	get := func(k string, d float64) float64 {
		if n, ok := mm[k].(float64); ok {
			return n
		}
		return d
	}
	return [4]float64{get("r", def[0]), get("g", def[1]), get("b", def[2]), get("a", def[3])}
}
