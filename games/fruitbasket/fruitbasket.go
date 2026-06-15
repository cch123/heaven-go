// Package fruitbasket ports Fruit Basket's rolling fruit cues, Courtney
// expressions, daydream bubbles, hoop reactions, and recolor timeline.
package fruitbasket

import (
	"image/color"
	"math"
	"sort"

	"github.com/hajimehoshi/ebiten/v2"

	"hsdemo/engine"
	"hsdemo/kart"
	"hsdemo/kmdata"
	"hsdemo/riq"
)

const (
	sideLeft = iota
	sideRight
	sideBoth
)

const (
	fruitApple = iota
	fruitLemon
	fruitMelon
)

const (
	exprNone = iota
	exprHappy
	exprCry
)

const (
	daydreamRandom = iota
	daydreamBurger
	daydreamBurgerAndDrink
	daydreamGirl
	daydreamTicTacToe
	daydreamSpider
	daydreamSkateboard
	daydreamDog
	daydreamBasketballPlayer
	daydreamPanda
	daydreamPizza
)

var defaultCourtneyColor = [4]float64{13.0 / 255, 144.0 / 255, 227.0 / 255, 1}

type bopEvt struct {
	beat, length float64
	bop, auto    bool
}

type fruitEvt struct {
	beat, length       float64
	side, fruit        int
	successExpression  int
	missExpression     int
	expressionDuration float64
	bothSidesRole      int
}

type daydreamEvt struct {
	beat, length float64
	typ, expr    int
}

type expressionEvt struct {
	beat, length float64
	expr         int
}

type colorEvt struct {
	beat, length float64
	start, end   [4]float64
	ease         int
}

type Module struct {
	ctx  *engine.Ctx
	proj kart.Aff

	gameComp kmdata.Component
	paths    map[string]curvePath

	courtney string
	cats     string
	hoopL    string
	hoopR    string
	body     string
	extended string
	hole     string
	pipeL    string
	pipeR    string

	fruitTemplates [3]*kart.Template
	bubbleTemplate *kart.Template

	bops        []bopEvt
	fruitEvents []fruitEvt
	daydreams   []daydreamEvt
	expressions []expressionEvt
	colors      []colorEvt

	fruits    []*activeFruit
	bubbles   []*daydreamBubble
	particles []scoreParticle

	goBop       bool
	expressing  bool
	lastPulse   int
	lastDunkHit float64

	basketBits map[int]int
}

func New() engine.Module {
	return &Module{
		goBop:       true,
		lastPulse:   -1 << 30,
		lastDunkHit: math.Inf(-1),
		basketBits:  map[int]int{},
	}
}

func (m *Module) ID() string { return "fruitBasket" }

func (m *Module) Load(ctx *engine.Ctx) error {
	m.ctx = ctx
	if err := ctx.LoadAssets("fruitBasket"); err != nil {
		return err
	}
	m.proj = kart.Translate(engine.ScreenW/2, engine.ScreenH/2).Mul(kart.Scale(54, -54))
	m.gameComp = ctx.Assets.Extra.Components["game"]
	m.paths = readCurvePaths(ctx, m.gameComp.Lists["fruitPaths"])

	m.courtney = refOr(ctx, m.gameComp, "courtneyAnimator", "Courtney")
	m.cats = refOr(ctx, m.gameComp, "catsAnimator", "BG/Cats")
	m.hoopL = refOr(ctx, m.gameComp, "hoopL", "HoopL")
	m.hoopR = refOr(ctx, m.gameComp, "hoopR", "HoopR")
	m.body = refOr(ctx, m.gameComp, "courtneySprite", "Courtney/Body")
	m.extended = refOr(ctx, m.gameComp, "courtneyExtendedSprite", "Courtney/Extended")
	m.hole = refOr(ctx, m.gameComp, "courtneyHoleSprite", "Courtney/Hole")
	melonComp := ctx.Assets.Extra.Components["melon"]
	m.pipeL = refOr(ctx, melonComp, "leftPipeAnim", "PipeL")
	m.pipeR = refOr(ctx, melonComp, "rightPipeAnim", "PipeR")

	m.fruitTemplates[fruitApple] = kart.NewTemplate(ctx.Assets, refOr(ctx, m.gameComp, "applePrefab", "Apple"))
	m.fruitTemplates[fruitLemon] = kart.NewTemplate(ctx.Assets, refOr(ctx, m.gameComp, "lemonPrefab", "Lemon"))
	m.fruitTemplates[fruitMelon] = kart.NewTemplate(ctx.Assets, refOr(ctx, m.gameComp, "melonPrefab", "Melon"))
	m.bubbleTemplate = kart.NewTemplate(ctx.Assets, refOr(ctx, m.gameComp, "thoughtBubblePrefab", "ThoughtBubble"))

	for _, path := range []string{"Apple", "Lemon", "Melon", "ThoughtBubble"} {
		ctx.Scene.SetActive(path, false)
	}
	return nil
}

func (m *Module) OnEvent(e *riq.Entity) {
	switch e.Datamodel {
	case "fruitBasket/bop":
		m.bops = append(m.bops, bopEvt{
			beat: e.Beat, length: e.Length,
			bop:  boolDefault(e, "bop", true),
			auto: boolParam(e, "bopAuto"),
		})
	case "fruitBasket/apple":
		m.addFruitEvent(e, fruitApple, 3)
	case "fruitBasket/lemon":
		m.addFruitEvent(e, fruitLemon, 5)
	case "fruitBasket/melon":
		m.addFruitEvent(e, fruitMelon, 6)
	case "fruitBasket/daydream":
		m.daydreams = append(m.daydreams, daydreamEvt{
			beat: e.Beat, length: e.Length,
			typ:  int(e.Float("thought", daydreamRandom)),
			expr: int(e.Float("daydreamExpression", 0)),
		})
	case "fruitBasket/expression":
		m.expressions = append(m.expressions, expressionEvt{
			beat: e.Beat, length: e.Length,
			expr: int(e.Float("expression", 0)),
		})
	case "fruitBasket/color":
		m.colors = append(m.colors, colorEvt{
			beat: e.Beat, length: e.Length,
			start: colorParam(e, "colorStart", defaultCourtneyColor),
			end:   colorParam(e, "colorEnd", defaultCourtneyColor),
			ease:  int(e.Float("ease", 1)),
		})
	}
}

func (m *Module) addFruitEvent(e *riq.Entity, fruit int, defaultLength float64) {
	length := e.Length
	if length <= 0 {
		length = defaultLength
	}
	side := int(e.Float("side", sideLeft))
	ev := fruitEvt{
		beat: e.Beat, length: length,
		side: side, fruit: fruit,
		successExpression:  int(e.Float("successExpression", exprNone)),
		missExpression:     int(e.Float("missExpression", exprNone)),
		expressionDuration: e.Float("expressionDuration", 0),
	}
	if side == sideBoth {
		left, right := ev, ev
		left.side, left.bothSidesRole = sideLeft, 1
		right.side, right.bothSidesRole = sideRight, 2
		m.fruitEvents = append(m.fruitEvents, left, right)
		return
	}
	m.fruitEvents = append(m.fruitEvents, ev)
}

func (m *Module) Ready() {
	sort.Slice(m.bops, func(i, j int) bool { return m.bops[i].beat < m.bops[j].beat })
	sort.Slice(m.fruitEvents, func(i, j int) bool { return m.fruitEvents[i].beat < m.fruitEvents[j].beat })
	sort.Slice(m.daydreams, func(i, j int) bool { return m.daydreams[i].beat < m.daydreams[j].beat })
	sort.Slice(m.expressions, func(i, j int) bool { return m.expressions[i].beat < m.expressions[j].beat })
	sort.Slice(m.colors, func(i, j int) bool { return m.colors[i].beat < m.colors[j].beat })

	for _, b := range m.bops {
		b := b
		m.ctx.At(b.beat, func() { m.goBop = b.auto })
		if b.bop {
			for i := 0; i < int(math.Ceil(b.length)); i++ {
				beat := b.beat + float64(i)
				if beat < b.beat+b.length {
					beat := beat
					m.ctx.At(beat, func() { m.bop(beat) })
				}
			}
		}
	}
	for _, ev := range m.fruitEvents {
		ev := ev
		m.ctx.At(ev.beat, func() { m.spawnFruit(ev) })
	}
	for _, ev := range m.daydreams {
		ev := ev
		m.ctx.At(ev.beat, func() { m.spawnDaydream(ev) })
	}
	for _, ev := range m.expressions {
		ev := ev
		m.ctx.At(ev.beat, func() { m.courtneyExpression(expressionNameAll(ev.expr), ev.beat+ev.length) })
	}
}

func (m *Module) OnSwitch(beat float64) {
	m.goBop = true
	for _, b := range m.bops {
		if b.beat > beat {
			break
		}
		m.goBop = b.auto
	}
	m.lastPulse = int(math.Floor(beat)) - 1
	m.lastDunkHit = math.Inf(-1)
	m.fruits = liveFruits(m.fruits, beat)
	m.bubbles = liveBubbles(m.bubbles, beat)
	m.ctx.Scene.PlayDefaultState(m.courtney, beat, m.ctx.SecPerBeat(beat))
	m.ctx.Scene.PlayDefaultState(m.cats, beat, m.ctx.SecPerBeat(beat))
	m.ctx.Scene.PlayDefaultState(m.hoopL, beat, m.ctx.SecPerBeat(beat))
	m.ctx.Scene.PlayDefaultState(m.hoopR, beat, m.ctx.SecPerBeat(beat))
	m.ctx.Scene.PlayDefaultState(m.pipeL, beat, m.ctx.SecPerBeat(beat))
	m.ctx.Scene.PlayDefaultState(m.pipeR, beat, m.ctx.SecPerBeat(beat))
	m.applyCourtneyColor(beat)
}

func (m *Module) Whiff(beat float64) {
	m.ctx.Scene.PlayState(m.courtney, "whiff", beat, 0.5)
}

func (m *Module) Update(t, beat float64) {
	whole := int(math.Floor(beat))
	for b := m.lastPulse + 1; b <= whole; b++ {
		if m.goBop {
			m.bop(float64(b))
		}
	}
	m.lastPulse = whole
	m.applyCourtneyColor(beat)
	for _, f := range m.fruits {
		f.update(m, t, beat)
	}
	m.fruits = liveFruits(m.fruits, beat)
	for _, b := range m.bubbles {
		b.update(m, beat)
	}
	m.bubbles = liveBubbles(m.bubbles, beat)
	m.updateParticles(t)
}

func (m *Module) Draw(screen *ebiten.Image, t, beat float64) {
	screen.Fill(color.RGBA{255, 255, 255, 255})
	m.ctx.SampleScene(beat)
	for _, f := range m.fruits {
		f.queue(m.ctx.Scene, beat)
	}
	for _, b := range m.bubbles {
		b.queue(m.ctx.Scene, beat)
	}
	m.queueParticles()
	m.ctx.Scene.Draw(screen, m.proj)
}

func (m *Module) bop(beat float64) {
	m.ctx.Scene.PlayState(m.cats, "catsBop", beat, 0.5)
}

func (m *Module) hitFruitAnimation(beat float64) {
	if math.Abs(beat-m.lastDunkHit) < 1e-6 {
		return
	}
	m.lastDunkHit = beat
	m.ctx.Sound("dunk")
	m.ctx.Scene.PlayState(m.courtney, "fruit_hit", beat, 0.5)
}

func (m *Module) missFruitAnimation(beat float64) {
	m.ctx.Sound("common_miss")
	m.ctx.Scene.PlayState(m.courtney, "miss", beat, 0.5)
}

func (m *Module) hoopAnimation(animName string, left bool, fruit int, beat float64) {
	hoop := m.hoopR
	if left {
		hoop = m.hoopL
	}
	m.ctx.Scene.Play(hoop, "Animations/hoop/"+animName, beat, 0.5)
	m.spawnScoreParticles(hoop, fruit, beat)
}

func (m *Module) queueBasketSound(beat float64, sideBit int) {
	key := int(math.Round(beat * 1000))
	if _, exists := m.basketBits[key]; !exists {
		m.ctx.At(beat, func() {
			bits := m.basketBits[key]
			delete(m.basketBits, key)
			switch bits {
			case 3:
				m.ctx.Sound("basket")
			case 2:
				m.ctx.Sound("basketL")
			case 1:
				m.ctx.Sound("basketR")
			}
		})
	}
	m.basketBits[key] |= sideBit
}

func (m *Module) courtneyExpression(name string, stopBeat float64) {
	if m.expressing || name == "None" || name == "" {
		return
	}
	m.expressing = true
	beat := m.ctx.Beat()
	m.ctx.Scene.PlayState(m.courtney, name+"Start", beat, 0.5)
	m.ctx.At(stopBeat, func() {
		m.expressing = false
		m.ctx.Scene.PlayState(m.courtney, name+"Stop", stopBeat, 0.5)
	})
}

func (m *Module) applyCourtneyColor(beat float64) {
	c := defaultCourtneyColor
	for _, ev := range m.colors {
		if ev.beat > beat {
			break
		}
		u := 1.0
		if ev.length > 0 {
			u = clamp01((beat - ev.beat) / ev.length)
		}
		c = easeColor(ev.ease, ev.start, ev.end, u)
	}
	m.ctx.Scene.SetColorOver(m.body, c)
	m.ctx.Scene.SetColorOver(m.extended, c)
	m.ctx.Scene.SetColorOver(m.hole, c)
}

func liveFruits(in []*activeFruit, beat float64) []*activeFruit {
	out := in[:0]
	for _, f := range in {
		if !f.dead && beat <= f.dieBeat {
			out = append(out, f)
		}
	}
	return out
}

func liveBubbles(in []*daydreamBubble, beat float64) []*daydreamBubble {
	out := in[:0]
	for _, b := range in {
		if !b.dead && beat <= b.fadeOutBeat+1 {
			out = append(out, b)
		}
	}
	return out
}
