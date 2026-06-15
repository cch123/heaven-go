package tossboys

import (
	"sort"

	"github.com/hajimehoshi/ebiten/v2"

	"hsdemo/engine"
	"hsdemo/kart"
	"hsdemo/riq"
)

type passEvent struct {
	beat, length float64
	datamodel    string
	who          int
}

type dispenseEvent struct {
	beat, length           float64
	who, interval          int
	auto, ignore, callAuto bool
	call                   bool
}

type bopEvent struct {
	beat, length float64
	bop          bool
}

type bgEvent struct {
	beat, length float64
	ease         int
	start, end   [4]float64
}

type guitaristEvent struct {
	beat   float64
	toggle bool
	colors [6][4]float64
}

type guitaristAnimEvent struct {
	beat float64
	typ  int
}

type Module struct {
	ctx  *engine.Ctx
	proj kart.Aff

	kids        map[int]*kidState
	currentKid  int
	lastKid     int
	specialKid  int
	currentType string
	currentLen  float64
	ball        *tossBall

	ballTemplate *kart.Template
	paths        map[string]ballPath
	targets      map[string][3]float64
	passEvents   []passEvent
	passByBeat   map[int64]passEvent
	dispenses    []dispenseEvent
	bops         []bopEvent
	bgEvents     []bgEvent
	guitarists   []guitaristEvent
	guitarAnims  []guitaristAnimEvent

	bgColor      [4]float64
	guitarActive bool
}

func New() engine.Module {
	return &Module{
		kids:       map[int]*kidState{},
		currentKid: kidNone,
		lastKid:    kidNone,
		specialKid: kidNone,
		bgColor:    defaultBG,
	}
}

func (m *Module) ID() string { return "tossBoys" }

func (m *Module) Load(ctx *engine.Ctx) error {
	m.ctx = ctx
	if err := ctx.LoadAssets("tossBoys"); err != nil {
		return err
	}
	m.proj = kart.Translate(engine.ScreenW/2, engine.ScreenH/2).Mul(kart.Scale(64, -64))
	m.paths = loadBallPaths(ctx.Assets.Extra.Components["game"].Lists["ballPaths"])
	m.targets = sceneTargets(ctx)
	m.passByBeat = map[int64]passEvent{}
	m.ballTemplate = kart.NewTemplate(ctx.Assets, roleOr(ctx, "ballPrefab", "Ball"))
	m.loadKids()
	for _, path := range []string{
		roleOr(ctx, "ballPrefab", "Ball"),
		roleOr(ctx, "specialAka", "SpecialOverlay/Akachan"),
		roleOr(ctx, "specialAo", "SpecialOverlay/Aokun"),
		roleOr(ctx, "specialKii", "SpecialOverlay/Kiiyan"),
		roleOr(ctx, "soshi", "Soshi"),
	} {
		ctx.Scene.SetActive(path, false)
	}
	for _, path := range []string{"Akachan", "Aokun", "Kiiyan", "HatchHolder", "Soshi"} {
		ctx.Scene.PlayDefaultState(path, 0, ctx.SecPerBeat(0))
	}
	return nil
}

func (m *Module) loadKids() {
	for _, key := range []struct {
		kid      int
		role     string
		fallback string
		action   int
	}{
		{kidAka, "akachan", "Akachan", actionAka},
		{kidAo, "aokun", "Aokun", actionAo},
		{kidKii, "kiiyan", "Kiiyan", actionKii},
	} {
		path := roleOr(m.ctx, key.role, key.fallback)
		prefix := ""
		if c, ok := componentByPath(m.ctx.Assets.Extra.Components, path); ok {
			prefix = c.Strs["prefix"]
		}
		if prefix == "" {
			prefix = map[int]string{kidAka: "Aka", kidAo: "Ao", kidKii: "Kii"}[key.kid]
		}
		m.kids[key.kid] = newKid(m.ctx, path, prefix, key.action)
	}
}

func (m *Module) OnEvent(e *riq.Entity) {
	switch e.Datamodel {
	case "tossBoys/bop":
		m.bops = append(m.bops, bopEvent{beat: e.Beat, length: e.Length, bop: boolParamDefault(e, "bop", true)})
	case "tossBoys/dispense":
		iv := int(e.Float("interval", 2))
		if iv <= 0 {
			iv = 2
		}
		m.dispenses = append(m.dispenses, dispenseEvent{
			beat: e.Beat, length: e.Length, who: int(e.Float("who", kidAka)),
			auto: boolParamDefault(e, "auto", true), interval: iv,
			ignore: boolParamDefault(e, "ignore", true), callAuto: boolParamDefault(e, "callAuto", false),
			call: boolParamDefault(e, "call", false),
		})
	case "tossBoys/pass", "tossBoys/dual", "tossBoys/high", "tossBoys/lightning", "tossBoys/blur", "tossBoys/pop":
		m.passEvents = append(m.passEvents, passEvent{
			beat: e.Beat, length: e.Length, datamodel: e.Datamodel,
			who: int(e.Float("who", kidAo)),
		})
	case "tossBoys/changeBG":
		m.bgEvents = append(m.bgEvents, bgEvent{
			beat: e.Beat, length: e.Length, ease: int(e.Float("ease", 0)),
			start: colorParam(e, "start", defaultBG), end: colorParam(e, "end", defaultBG),
		})
	case "tossBoys/guitarist":
		g := guitaristEvent{beat: e.Beat, toggle: boolParamDefault(e, "toggle", true)}
		defaults := [6][4]float64{
			{0.980, 0.808, 0.678, 1}, {1, 1, 1, 1}, {0.4, 0.78, 1, 1},
			{0.2, 0.35, 0.835, 1}, {0, 0, 0, 1}, {1, 1, 1, 1},
		}
		for i := range g.colors {
			g.colors[i] = colorParam(e, "colors"+string(rune('0'+i)), defaults[i])
		}
		m.guitarists = append(m.guitarists, g)
	case "tossBoys/guitarist anim":
		m.guitarAnims = append(m.guitarAnims, guitaristAnimEvent{beat: e.Beat, typ: int(e.Float("type", 0))})
	}
}

func (m *Module) Ready() {
	sort.SliceStable(m.passEvents, func(i, j int) bool { return m.passEvents[i].beat < m.passEvents[j].beat })
	for _, ev := range m.passEvents {
		if _, exists := m.passByBeat[beatKey(ev.beat)]; !exists {
			m.passByBeat[beatKey(ev.beat)] = ev
		}
	}
	sort.SliceStable(m.dispenses, func(i, j int) bool { return m.dispenses[i].beat < m.dispenses[j].beat })
	sort.SliceStable(m.bops, func(i, j int) bool { return m.bops[i].beat < m.bops[j].beat })
	sort.SliceStable(m.bgEvents, func(i, j int) bool { return m.bgEvents[i].beat < m.bgEvents[j].beat })
	sort.SliceStable(m.guitarists, func(i, j int) bool { return m.guitarists[i].beat < m.guitarists[j].beat })
	sort.SliceStable(m.guitarAnims, func(i, j int) bool { return m.guitarAnims[i].beat < m.guitarAnims[j].beat })

	for _, ev := range m.bops {
		if !ev.bop {
			continue
		}
		for i := 0; i < int(ev.length); i++ {
			b := ev.beat + float64(i)
			m.ctx.At(b, func() { m.singleBop(b) })
		}
	}
	for _, ev := range m.dispenses {
		ev := ev
		m.ctx.At(ev.beat, func() {
			if m.ctx.GameAt(ev.beat) == m.ID() {
				m.dispense(ev, true)
			} else {
				m.dispenseSound(ev.beat, ev.who, ev.call)
			}
		})
		if ev.auto {
			m.scheduleAutoDispense(ev)
		}
	}
	for _, ev := range m.guitarAnims {
		ev := ev
		m.ctx.At(ev.beat, func() { m.guitarAnim(ev.beat, ev.typ) })
	}
}

func (m *Module) OnSwitch(beat float64) {
	m.applyPersistentState(beat)
	for _, ev := range m.dispenses {
		if ev.beat > beat {
			break
		}
		if ev.beat+ev.length >= beat {
			m.dispense(ev, false)
			break
		}
	}
}

func (m *Module) Whiff(beat float64) { m.WhiffAction(beat, actionAka) }

func (m *Module) WhiffAction(beat float64, action int) {
	switch action {
	case actionAo:
		m.kids[kidAo].hitBall(beat, false)
	case 1, 2, actionKii:
		m.kids[kidKii].hitBall(beat, false)
	default:
		m.kids[kidAka].hitBall(beat, false)
	}
}

func (m *Module) Update(t, beat float64) {
	m.applyPersistentState(beat)
	m.ctx.Scene.SetColorOver(roleOr(m.ctx, "bg", "Background"), m.bgColor)
	if m.ball != nil {
		m.ball.update(m, beat)
	}
	m.ctx.SampleScene(beat)
}

func (m *Module) Draw(screen *ebiten.Image, _ float64, beat float64) {
	if m.ball != nil && !m.ball.dead {
		m.ball.inst.Queue(m.ctx.Scene, beat, kart.Identity(), m.ball.pos[2])
	}
	m.ctx.Scene.Draw(screen, m.proj)
}

func (m *Module) applyPersistentState(beat float64) {
	m.bgColor = defaultBG
	for _, ev := range m.bgEvents {
		if ev.beat > beat {
			break
		}
		u := 1.0
		if ev.length > 0 {
			u = clamp01((beat - ev.beat) / ev.length)
		}
		for i := 0; i < 4; i++ {
			m.bgColor[i] = engine.Ease(ev.ease, ev.start[i], ev.end[i], u)
		}
	}
	showGuitar := false
	for _, ev := range m.guitarists {
		if ev.beat > beat {
			break
		}
		showGuitar = ev.toggle
	}
	m.guitarActive = showGuitar
	m.ctx.Scene.SetActive(roleOr(m.ctx, "soshi", "Soshi"), showGuitar)
	if showGuitar {
		m.ctx.Scene.SetColorOver(roleOr(m.ctx, "soshiPants", "Soshi/Pants"), [4]float64{0.4, 0.78, 1, 1})
	}
}

func (m *Module) singleBop(beat float64) {
	for _, k := range m.kids {
		k.bop(beat)
	}
	if m.guitarActive {
		m.ctx.Scene.PlayState(roleOr(m.ctx, "soshiAnim", "Soshi"), "Bop", beat, 0.5)
	}
}

func (m *Module) guitarAnim(beat float64, typ int) {
	if !m.guitarActive {
		return
	}
	state := "Strum"
	if typ == 1 {
		state = "Blink"
	}
	m.ctx.Scene.PlayStateLayer("soshi-"+state, roleOr(m.ctx, "soshiAnim", "Soshi"), state, beat, 0.5)
}
