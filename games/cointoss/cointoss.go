// Package cointoss ports Coin Toss's single-coin toss/catch flow, hand swap,
// image overlay, cowbell cue variant, and background color easing.
package cointoss

import (
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"

	"hsdemo/engine"
	"hsdemo/kart"
	"hsdemo/riq"
)

var (
	defaultBG = [4]float64{0.97, 0.97, 0.26, 1}
	defaultFG = [4]float64{1.0, 1.0, 0.51, 1}
)

type bgEvt struct {
	beat, length float64
	bg0, bg1     [4]float64
	fg0, fg1     [4]float64
	ease         int
}

type imageEvt struct {
	beat, length             float64
	instantShow, instantHide bool
}

type activeCoin struct {
	beat     float64
	judge    float64
	audience bool
}

type Module struct {
	ctx  *engine.Ctx
	proj kart.Aff

	bg     bgEvt
	bgEvts []bgEvt
	images []imageEvt

	coin      *activeCoin
	coinOnMan bool
}

func New() engine.Module {
	return &Module{bg: bgEvt{bg0: defaultBG, bg1: defaultBG, fg0: defaultFG, fg1: defaultFG}}
}

func (m *Module) ID() string { return "coinToss" }

func (m *Module) Load(ctx *engine.Ctx) error {
	m.ctx = ctx
	if err := ctx.LoadAssets("coinToss"); err != nil {
		return err
	}
	m.proj = kart.Translate(engine.ScreenW/2, engine.ScreenH/2).Mul(kart.Scale(36, -36))
	white := ebiten.NewImage(2, 2)
	white.Fill(color.White)
	ctx.Assets.RegisterSprite("__coinTossWhite", white, 1, 0.5, 0.5)
	return nil
}

func (m *Module) OnEvent(e *riq.Entity) {
	b := e.Beat
	switch e.Datamodel {
	case "coinToss/toss":
		typ := int(e.Float("type", 0))
		audience := boolParam(e, "toggle")
		m.ctx.At(b, func() { m.tossCoin(b, typ, audience) })
	case "coinToss/set hand":
		hand := int(e.Float("hand", 0))
		m.ctx.At(b, func() { m.setHand(hand, b) })
	case "coinToss/show image":
		ev := imageEvt{
			beat: b, length: e.Length,
			instantShow: boolDefault(e, "instantShow", true),
			instantHide: boolParam(e, "instantHide"),
		}
		m.images = append(m.images, ev)
		m.ctx.At(b, func() {
			state := "ImageFadeIn"
			if ev.instantShow {
				state = "ImageShow"
			}
			m.ctx.Scene.PlayState(m.ctx.Role("imageAnim"), state, b, 0.5)
		})
		m.ctx.At(b+e.Length, func() {
			state := "ImageFadeOut"
			if ev.instantHide {
				state = "Idle"
			}
			m.ctx.Scene.PlayState(m.ctx.Role("imageAnim"), state, b+e.Length, 0.5)
		})
	case "coinToss/fade background color":
		m.backgroundColor(bgEvt{
			beat: b, length: e.Length,
			bg0:  colorParam(e, "colorStart", defaultBG),
			bg1:  colorParam(e, "colorEnd", defaultBG),
			fg0:  colorParam(e, "colorStartF", defaultFG),
			fg1:  colorParam(e, "colorEndF", defaultFG),
			ease: int(e.Float("ease", 1)),
		})
	case "coinToss/set background color":
		ca := colorParam(e, "colorA", defaultBG)
		cb := colorParam(e, "colorB", defaultFG)
		m.backgroundColor(bgEvt{beat: b, length: e.Length, bg0: ca, bg1: ca, fg0: cb, fg1: cb, ease: 0})
	}
}

func (m *Module) Ready() {}

func (m *Module) OnSwitch(beat float64) {
	m.bg = bgEvt{bg0: defaultBG, bg1: defaultBG, fg0: defaultFG, fg1: defaultFG}
	for _, ev := range m.bgEvts {
		if ev.beat > beat {
			break
		}
		m.bg = ev
	}
}

func (m *Module) Whiff(beat float64) {
	// CatchEmpty is the way-off/early hand animation while the original
	// PlayerActionEvent remains alive for the real catch window.
	if m.coin != nil {
		m.ctx.Scene.PlayState(m.ctx.Role("handAnimator"), "Catch_empty", beat, 0.5)
	}
}

func (m *Module) Update(t, beat float64) {}

func (m *Module) Draw(screen *ebiten.Image, t, beat float64) {
	bg, fg := m.colorsAt(beat)
	screen.Fill(toRGBA(bg))
	sc := m.ctx.Scene
	sc.SetColorOver(m.ctx.Role("fg"), fg)
	sc.SetColorOver(m.ctx.Role("bg"), bg)
	sc.SetColorOver(m.ctx.Role("imageBG"), bg)
	m.ctx.SampleScene(beat)
	if a := m.imageAlphaAt(beat); a > 0 {
		c := bg
		c[3] *= a
		if world, ok := sc.NodeWorld(m.ctx.Role("imageBG")); ok {
			sc.Queue(kart.ExtraSprite{
				Sprite: "__coinTossWhite", World: world,
				Order: 8, Tint: c,
			})
		}
	}
	sc.Draw(screen, m.proj)
}

func (m *Module) backgroundColor(ev bgEvt) {
	m.bgEvts = append(m.bgEvts, ev)
	m.ctx.At(ev.beat, func() { m.bg = ev })
}

func (m *Module) tossCoin(beat float64, typ int, audience bool) {
	if m.coin != nil {
		return
	}
	m.ctx.Sound("throw")
	if m.coinOnMan {
		m.ctx.Scene.PlayState(m.ctx.Role("manHand"), "Toss", beat, 0.5)
		m.ctx.Scene.PlayState(m.ctx.Role("handHolder"), "Enter", beat, 0.5)
		m.ctx.Scene.PlayState(m.ctx.Role("handAnimator"), "Idle_open", beat, 0.5)
		m.ctx.At(beat+1, func() {
			m.ctx.Scene.PlayState(m.ctx.Role("manHolder"), "ManExit", beat+1, 0.5)
		})
	} else {
		m.ctx.Scene.PlayState(m.ctx.Role("handAnimator"), "Throw", beat, 0.5)
	}
	m.coinOnMan = false
	m.coin = &activeCoin{beat: beat, judge: beat + 6, audience: audience}
	if typ == 1 {
		m.ctx.Sound("cowbell1")
		for i := 1; i <= 6; i++ {
			name := "cowbell2"
			if i%2 == 0 {
				name = "cowbell1"
			}
			m.ctx.SoundAtOff(beat+float64(i), name, 1, 0.01)
		}
	}
	m.ctx.ScheduleInputAction(beat+6, 0,
		func(state float64, _ engine.Judgment) { m.catchSuccess(beat) },
		func() { m.catchMiss(beat) })
}

func (m *Module) catchSuccess(beat float64) {
	c := m.coin
	m.ctx.Sound("catch")
	if c != nil && c.audience {
		m.ctx.Sound("common_applause")
	}
	m.ctx.Scene.PlayState(m.ctx.Role("handAnimator"), "Catch_success", m.ctx.Beat(), 0.5)
	m.coin = nil
}

func (m *Module) catchMiss(beat float64) {
	c := m.coin
	m.ctx.Sound("miss")
	if c != nil && c.audience {
		// Heaven Studio calls SoundByte.PlayOneShot("audience/disappointed");
		// the current resource pack stores that common reaction as audienceSad.
		m.ctx.Sound("common_audienceSad")
	}
	m.ctx.Scene.PlayState(m.ctx.Role("handAnimator"), "Pickup", m.ctx.Beat(), 0.5)
	m.coin = nil
}

func (m *Module) setHand(hand int, beat float64) {
	if m.coin != nil {
		return
	}
	m.coinOnMan = hand == 1
	if m.coinOnMan {
		m.ctx.Scene.PlayState(m.ctx.Role("manHolder"), "ManShow", beat, 0.5)
		m.ctx.Scene.PlayState(m.ctx.Role("handHolder"), "Offscreen", beat, 0.5)
		return
	}
	m.ctx.Scene.PlayState(m.ctx.Role("manHolder"), "Idle", beat, 0.5)
	m.ctx.Scene.PlayState(m.ctx.Role("handHolder"), "Idle", beat, 0.5)
}

func (m *Module) colorsAt(beat float64) ([4]float64, [4]float64) {
	ev := m.bg
	norm := 1.0
	if ev.length > 0 {
		norm = clamp01((beat - ev.beat) / ev.length)
	}
	var bg, fg [4]float64
	for i := 0; i < 4; i++ {
		bg[i] = engine.Ease(ev.ease, ev.bg0[i], ev.bg1[i], norm)
		fg[i] = engine.Ease(ev.ease, ev.fg0[i], ev.fg1[i], norm)
	}
	return bg, fg
}

func (m *Module) imageAlphaAt(beat float64) float64 {
	alpha := 0.0
	for _, ev := range m.images {
		switch {
		case beat < ev.beat:
			continue
		case beat < ev.beat+ev.length:
			if ev.instantShow {
				alpha = 1
			} else {
				alpha = clamp01((beat - ev.beat) * 0.5)
			}
		case !ev.instantHide && beat < ev.beat+ev.length+2:
			alpha = 1 - clamp01((beat-ev.beat-ev.length)*0.5)
		case ev.instantHide:
			alpha = 0
		}
	}
	return alpha
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
	m, ok := v.(map[string]any)
	if !ok {
		return def
	}
	return [4]float64{
		num(m["r"], def[0]), num(m["g"], def[1]), num(m["b"], def[2]), num(m["a"], def[3]),
	}
}

func num(v any, def float64) float64 {
	if f, ok := v.(float64); ok {
		return f
	}
	return def
}

func clamp01(v float64) float64 {
	return math.Max(0, math.Min(1, v))
}

func toRGBA(c [4]float64) color.NRGBA {
	return color.NRGBA{
		R: uint8(clamp01(c[0]) * 255),
		G: uint8(clamp01(c[1]) * 255),
		B: uint8(clamp01(c[2]) * 255),
		A: uint8(clamp01(c[3]) * 255),
	}
}
