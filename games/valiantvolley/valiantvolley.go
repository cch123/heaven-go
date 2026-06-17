// Package valiantvolley ports ValiANT Volley's WIP ant volley runtime:
// interval scheduling, dirt/fruit input channels, ant reactions, and
// Bezier-authored object travel from Assets/Scripts/Games/ValiantVolley.
package valiantvolley

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
	objDirt = iota
	objFruit
)

const (
	bopNormal = iota
	bopHappy
	bopAngry
	bopOops
)

const actionFruit = 3

type bopEvt struct {
	beat, length float64
	which        int
	single, keep bool
}

type hitEvt struct {
	beat, length float64
	typ          int
	shouldPrep   bool
	prepBeats    float64
}

type intervalEvt struct {
	beat, length float64
}

type Module struct {
	ctx  *engine.Ctx
	proj kart.Aff

	ants       [3]*ant
	objectT    *kart.Template
	objectComp kmdata.Component
	curves     map[string]kmdata.Curve

	bops      []bopEvt
	hits      []hitEvt
	intervals []intervalEvt
	objects   []*volleyObject

	bopStatus           int
	multiInputInterval  bool
	multiIntervalBeat   float64
	multiIntervalLength float64
	lastPulse           int
}

func New() engine.Module {
	return &Module{
		bopStatus:           bopNormal,
		multiIntervalBeat:   math.Inf(1),
		multiIntervalLength: 0,
		lastPulse:           -1 << 30,
	}
}

func (m *Module) ID() string { return "valiantVolley" }

func (m *Module) Load(ctx *engine.Ctx) error {
	m.ctx = ctx
	if err := ctx.LoadAssets("valiantVolley"); err != nil {
		return err
	}
	m.proj = kart.Translate(engine.ScreenW/2, engine.ScreenH/2).Mul(kart.Scale(54, -54))
	m.curves = ctx.Assets.Extra.Curves
	m.objectComp = ctx.Assets.Extra.Components["object"]
	m.objectT = kart.NewTemplate(ctx.Assets, refOr(ctx, m.objectComp, "anim", "ObjectHolder"))
	ctx.Scene.SetActive(refOr(ctx, m.objectComp, "anim", "ObjectHolder"), false)

	antPaths := ctx.Assets.Extra.Components["game"].RefArrays["ants"]
	if len(antPaths) == 0 {
		antPaths = []string{"Ants/AntLeft", "Ants/AntMiddle", "Ants/AntPlayer"}
	}
	for i := 0; i < len(m.ants) && i < len(antPaths); i++ {
		m.ants[i] = &ant{path: antPaths[i], num: i, player: i == 2, queuePrepare: math.Inf(1), lastVolleyBeat: math.Inf(-1)}
	}
	return nil
}

func (m *Module) OnEvent(e *riq.Entity) {
	switch e.Datamodel {
	case "valiantVolley/bop":
		m.bops = append(m.bops, bopEvt{
			beat: e.Beat, length: e.Length,
			which:  int(e.Float("whichBop", bopNormal)),
			single: boolDefault(e, "singleBop", true),
			keep:   boolParam(e, "keepBop"),
		})
	case "valiantVolley/startInterval":
		m.intervals = append(m.intervals, intervalEvt{beat: e.Beat, length: e.Length})
	case "valiantVolley/dirtHit":
		m.hits = append(m.hits, hitEvt{
			beat: e.Beat, length: e.Length, typ: objDirt,
			shouldPrep: boolDefault(e, "shouldPrep", true),
			prepBeats:  e.Float("prepBeats", 1),
		})
	case "valiantVolley/fruitHit":
		m.hits = append(m.hits, hitEvt{
			beat: e.Beat, length: e.Length, typ: objFruit,
			shouldPrep: boolDefault(e, "shouldPrep", true),
			prepBeats:  e.Float("prepBeats", 1),
		})
	}
}

func (m *Module) Ready() {
	sort.Slice(m.bops, func(i, j int) bool { return m.bops[i].beat < m.bops[j].beat })
	sort.Slice(m.hits, func(i, j int) bool { return m.hits[i].beat < m.hits[j].beat })
	sort.Slice(m.intervals, func(i, j int) bool { return m.intervals[i].beat < m.intervals[j].beat })
	m.scheduleBops()
	m.scheduleIntervalsAndObjects()
}

func (m *Module) OnSwitch(beat float64) {
	m.objects = liveObjects(m.objects, beat)
	m.bopStatus = bopNormal
	m.multiInputInterval = false
	m.multiIntervalBeat = math.Inf(1)
	m.multiIntervalLength = 0
	for _, b := range m.bops {
		if b.beat > beat {
			break
		}
		if b.keep {
			m.bopStatus = b.which
		}
	}
	for _, a := range m.ants {
		if a == nil {
			continue
		}
		a.reset()
		m.ctx.Scene.PlayDefaultState(a.path, beat, m.ctx.SecPerBeat(beat))
	}
	m.lastPulse = int(math.Floor(beat)) - 1
}

func (m *Module) Whiff(beat float64) { m.WhiffAction(beat, 0) }

func (m *Module) WhiffAction(beat float64, _ int) {
	if a := m.ants[2]; a != nil {
		a.action(m, beat, "dirtHit", 1)
		m.ctx.PlayCommon("nearMiss")
	}
}

func (m *Module) Update(t, beat float64) {
	whole := int(math.Floor(beat))
	for b := m.lastPulse + 1; b <= whole; b++ {
		m.pulse(float64(b))
	}
	m.lastPulse = whole
	for _, a := range m.ants {
		if a != nil {
			a.update(m, beat)
		}
	}
	for _, o := range m.objects {
		o.update(m, t, beat)
	}
	m.objects = liveObjects(m.objects, beat)
}

func (m *Module) Draw(screen *ebiten.Image, t, beat float64) {
	screen.Fill(color.RGBA{80, 8, 8, 255})
	m.ctx.SampleScene(beat)
	for _, o := range m.objects {
		o.queue(m.ctx.Scene, t, beat)
	}
	m.ctx.Scene.Draw(screen, m.proj)
}

func (m *Module) pulse(beat float64) {
	m.syncBopGate(beat)
	for _, a := range m.ants {
		if a != nil {
			a.requestBop(m, beat)
		}
	}
}

func (m *Module) syncBopGate(beat float64) bool {
	keepBop := m.bopActive(beat)
	for _, a := range m.ants {
		if a != nil {
			a.cantBop = !keepBop
		}
	}
	return keepBop
}

func (m *Module) bopActive(beat float64) bool {
	if len(m.bops) == 0 {
		return true
	}
	active := false
	lastBeat := math.Inf(-1)
	for _, b := range m.bops {
		if b.beat > beat {
			break
		}
		// SetupBopRegion keeps the first bop event on a beat and stores only the
		// keepBop toggle. singleBop is handled by Bop itself, not by the auto-bop
		// region, so interval gates must ignore the event length.
		if nearly(b.beat, lastBeat) {
			continue
		}
		active = b.keep
		lastBeat = b.beat
	}
	return active
}

func liveObjects(in []*volleyObject, beat float64) []*volleyObject {
	out := in[:0]
	for _, o := range in {
		if o != nil && !o.dead && beat <= o.dieBeat {
			out = append(out, o)
		}
	}
	return out
}
