package fruitbasket

import "hsdemo/kart"

type daydreamBubble struct {
	inst        *kart.Instance
	beat        float64
	fadeOutBeat float64
	dead        bool
	fading      bool
}

func (m *Module) spawnDaydream(ev daydreamEvt) {
	if m.bubbleTemplate == nil {
		return
	}
	dream := daydreamName(ev.typ, ev.beat)
	b := &daydreamBubble{
		inst:        m.bubbleTemplate.NewInstance(),
		beat:        ev.beat,
		fadeOutBeat: ev.beat + ev.length - 1,
	}
	if b.fadeOutBeat < ev.beat {
		b.fadeOutBeat = ev.beat
	}
	b.inst.SetActive("", true)
	b.inst.PlayState("", "thoughtBubble", ev.beat, m.ctx.SecPerBeat(ev.beat))
	b.inst.PlayState("", "thoughtBubbleFadeIn", ev.beat, m.ctx.SecPerBeat(ev.beat))
	b.inst.PlayState("BubbleL/Thought", dream, ev.beat, m.ctx.SecPerBeat(ev.beat))
	m.bubbles = append(m.bubbles, b)

	expr := "Daydream" + daydreamExpressionName(ev.expr)
	m.ctx.Scene.PlayState(m.courtney, expr+"Start", ev.beat, 0.5)
	m.ctx.At(ev.beat+ev.length, func() {
		m.ctx.Scene.PlayState(m.courtney, expr+"Stop", ev.beat+ev.length, 0.5)
	})
}

func daydreamName(typ int, beat float64) string {
	if typ == daydreamRandom {
		typ = 1 + int(eventRand(beat, 17)*float64(daydreamPizza))
	}
	switch typ {
	case daydreamBurgerAndDrink:
		return "BurgerAndDrink"
	case daydreamGirl:
		return "Girl"
	case daydreamTicTacToe:
		return "TicTacToe"
	case daydreamSpider:
		return "Spider"
	case daydreamSkateboard:
		return "Skateboard"
	case daydreamDog:
		return "Dog"
	case daydreamPanda:
		return "Panda"
	default:
		// The HS enum also contains BasketballPlayer and Pizza, but this prefab
		// ships no matching Thought controller state. Unity falls back to the
		// existing bubble content; Burger is the controller's authored default.
		return "burger"
	}
}

func (b *daydreamBubble) update(m *Module, beat float64) {
	if b.dead || b.inst == nil {
		return
	}
	if !b.fading && beat >= b.fadeOutBeat {
		b.fading = true
		b.inst.PlayState("", "thoughtBubbleFadeOut", b.fadeOutBeat, m.ctx.SecPerBeat(b.fadeOutBeat))
	}
	if beat > b.fadeOutBeat+1 {
		b.dead = true
	}
}

func (b *daydreamBubble) queue(sc *kart.SceneInst, beat float64) {
	if b.dead || b.inst == nil {
		return
	}
	b.inst.Queue(sc, beat, kart.Identity(), 0)
}
