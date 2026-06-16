// Package fillbots ports the Fillbots event surface, conveyor timing, hold /
// release input loop, extracted controller animations, and original sounds.
package fillbots

import (
	"math"
	"sort"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"hsdemo/engine"
	"hsdemo/kart"
	"hsdemo/riq"
)

type Module struct {
	ctx  *engine.Ctx
	proj kart.Aff

	filler       string
	conveyerBelt string
	bgPlane      string
	blackoutPath string

	gears          []string
	meters         []string
	metersFuel     []string
	fillerRenderer []string
	otherRenderer  []string

	templates map[botSize]*kart.Template
	specs     map[botSize]botSpec

	bops      []bopEvt
	bgEvents  []bgEvt
	objEvents []objectEvt
	pending   []spawnEvt
	bots      []*bot

	fillerPosition           botSize
	fillerHolding            bool
	conveyerStartBeat        float64
	conveyerNormalizedOffset float64
	toggleGlobal             int
	lastPulse                int
	blackout                 bool
}

func New() engine.Module {
	return &Module{
		templates:         map[botSize]*kart.Template{},
		specs:             map[botSize]botSpec{},
		fillerPosition:    sizeMedium,
		conveyerStartBeat: -1,
		lastPulse:         -1,
	}
}

func (m *Module) ID() string { return gameID }

func (m *Module) Load(ctx *engine.Ctx) error {
	m.ctx = ctx
	if err := ctx.LoadAssets(gameID); err != nil {
		return err
	}
	m.proj = kart.Translate(engine.ScreenW/2, engine.ScreenH/2).Mul(kart.Scale(54, -54))

	as := ctx.Assets
	game := as.Extra.Components["game"]
	m.filler = game.Refs["filler"]
	m.conveyerBelt = game.Refs["conveyerBelt"]
	m.bgPlane = game.Refs["BGPlane"]
	m.blackoutPath = game.Refs["blackout"]
	m.gears = append([]string(nil), game.RefArrays["gears"]...)
	m.meters = append([]string(nil), game.RefArrays["meters"]...)
	m.metersFuel = append([]string(nil), game.RefArrays["metersFuel"]...)
	m.fillerRenderer = append([]string(nil), game.RefArrays["fillerRenderer"]...)
	m.otherRenderer = append([]string(nil), game.RefArrays["otherRenderer"]...)

	for _, size := range []botSize{sizeSmall, sizeMedium, sizeLarge} {
		root := botRoot(size)
		m.templates[size] = kart.NewTemplate(as, root)
		m.specs[size] = m.loadBotSpec(size, root)
		ctx.Scene.SetActive(root, false)
	}
	ctx.Scene.SetActive(m.blackoutPath, false)
	ctx.Scene.PlayDefaultState(m.filler, 0, ctx.SecPerBeat(0))
	ctx.Scene.PlayDefaultState(m.conveyerBelt, 0, ctx.SecPerBeat(0))
	for _, meter := range m.meters {
		ctx.Scene.PlayDefaultState(meter, 0, ctx.SecPerBeat(0))
	}
	return nil
}

func (m *Module) OnEvent(e *riq.Entity) {
	switch {
	case hasPrefixDatamodel(e, "bop"):
		ev := bopEvt{
			beat: e.Beat, length: e.Length,
			toggle: boolDefault(e, "toggle", true),
			auto:   boolParam(e, "auto"),
		}
		m.bops = append(m.bops, ev)
		if ev.toggle {
			for i := 0; i < int(ev.length); i++ {
				beat := ev.beat + float64(i)
				m.ctx.At(beat, func() { m.bop(beat) })
			}
		}
	case hasPrefixDatamodel(e, "small"):
		m.queueSpawn(e, sizeSmall, 1)
	case hasPrefixDatamodel(e, "medium"):
		m.queueSpawn(e, sizeMedium, 3)
	case hasPrefixDatamodel(e, "large"):
		m.queueSpawn(e, sizeLarge, 7)
	case hasPrefixDatamodel(e, "custom"):
		m.queueSpawn(e, botSize(intDefault(e, "size", int(sizeMedium))), math.Max(0, e.Length-5))
	case hasPrefixDatamodel(e, "blackout"):
		beat := e.Beat
		m.ctx.At(beat, func() { m.blackout = !m.blackout })
	case hasPrefixDatamodel(e, "background appearance"):
		m.bgEvents = append(m.bgEvents, bgEventFrom(e))
	case hasPrefixDatamodel(e, "object appearance"):
		m.objEvents = append(m.objEvents, objectEvt{
			beat: e.Beat,
			fuel: colorParam(e, "colorFuel", defaultFuel), lampOff: colorParam(e, "colorLampOff", defaultLampOff), lampOn: colorParam(e, "colorLampOn", defaultLampOn),
			impact: colorParam(e, "colorImpact", defaultImpact), filler: colorParam(e, "colorFiller", defaultRenderer), conveyer: colorParam(e, "colorConveyer", defaultRenderer),
		})
	}
}

func (m *Module) Ready() {
	sort.SliceStable(m.bops, func(i, j int) bool { return m.bops[i].beat < m.bops[j].beat })
	sort.SliceStable(m.bgEvents, func(i, j int) bool { return m.bgEvents[i].beat < m.bgEvents[j].beat })
	sort.SliceStable(m.objEvents, func(i, j int) bool { return m.objEvents[i].beat < m.objEvents[j].beat })
}

func (m *Module) OnSwitch(beat float64) {
	m.lastPulse = int(math.Floor(beat)) - 1
	m.spawnPending(beat)
	m.applyObjectColors(beat)
	m.ctx.Scene.PlayDefaultState(m.filler, beat, m.ctx.SecPerBeat(math.Max(beat, 0)))
	m.ctx.Scene.PlayDefaultState(m.conveyerBelt, beat, m.ctx.SecPerBeat(math.Max(beat, 0)))
	for _, meter := range m.meters {
		m.ctx.Scene.PlayDefaultState(meter, beat, m.ctx.SecPerBeat(math.Max(beat, 0)))
	}
}

func (m *Module) Whiff(beat float64) {
	m.playFiller("Hold"+botSuffix(m.fillerPosition), beat, 0.5)
	m.ctx.Sound("armExtension")
}

func (m *Module) Update(_ float64, beat float64) {
	m.spawnPending(beat)
	if m.ctx.ReleasedNow() && !m.ctx.ExpectingReleaseNow() {
		if b := m.firstHoldingBot(); b != nil {
			b.handleReleaseWhiff(beat)
		}
		m.playFiller("ReleaseWhiff"+botSuffix(m.fillerPosition), beat, 0.5)
		m.ctx.Sound("armRetractionWhiff")
		if m.fillerHolding {
			m.ctx.Sound("armRetractionPop")
		}
	}
	for pulse := m.lastPulse + 1; pulse <= int(math.Floor(beat)); pulse++ {
		if pulse >= 0 && m.autoBopAt(float64(pulse)) {
			m.bop(float64(pulse))
		}
		m.lastPulse = pulse
	}
	for _, b := range m.bots {
		b.update(beat)
	}
	m.compactBots(beat)
	m.updateConveyerBelt(beat)
}

func (m *Module) Draw(screen *ebiten.Image, _ float64, beat float64) {
	bg, meterColors := m.bgAt(beat)
	m.applyObjectColors(beat)
	m.applyBackgroundColors(bg)
	m.ctx.SampleScene(beat)
	m.drawFlatSceneSurfaces(screen, bg, meterColors)
	for _, b := range m.bots {
		b.queue(beat)
	}
	m.ctx.Scene.Draw(screen, m.proj)
}

func (m *Module) queueSpawn(e *riq.Entity, size botSize, holdLength float64) {
	ev := spawnEvt{
		beat: e.Beat, holdLength: holdLength, size: size,
		fuel: colorParam(e, "colorFuel", defaultFuel), lampOff: colorParam(e, "colorLampOff", defaultLampOff), lampOn: colorParam(e, "colorLampOn", defaultLampOn),
		end: endAnim(intDefault(e, "type", int(endBoth))), altOK: boolParam(e, "alt"), stop: boolParam(e, "stop"), customCol: boolParam(e, "color"),
	}
	m.ctx.At(ev.beat, func() {
		if m.ctx.GameAt(ev.beat) != gameID {
			m.pending = append(m.pending, ev)
			return
		}
		m.spawnFillbot(ev)
	})
	if boolParam(e, "practice") {
		m.fillErUp(e.Beat + 3)
	}
}

func (m *Module) spawnPending(beat float64) {
	if len(m.pending) == 0 || m.ctx.GameAt(beat) != gameID {
		return
	}
	for _, ev := range m.pending {
		m.spawnFillbot(ev)
	}
	m.pending = m.pending[:0]
}

func (m *Module) spawnFillbot(ev spawnEvt) {
	t := m.templates[ev.size]
	if t == nil {
		return
	}
	if ev.holdLength <= 0 {
		ev.holdLength = m.specs[ev.size].holdLength
	}
	if !ev.customCol {
		obj := m.objectAt(ev.beat)
		ev.fuel, ev.lampOff, ev.lampOn = obj.fuel, obj.lampOff, obj.lampOn
	}
	b := newBot(m, ev, t, m.specs[ev.size])
	m.bots = append(m.bots, b)

	falling := []*bot{}
	for _, old := range m.bots {
		if old != b && old.startBeat < ev.beat && old.startBeat+3 >= ev.beat {
			falling = append(falling, old)
		}
	}
	if len(falling) > 0 {
		beat := ev.beat
		m.ctx.At(beat-0.25, func() {
			for _, old := range falling {
				old.stackToLeft(beat, 0.25)
			}
			if m.conveyerStartBeat == -2 {
				m.conveyerStartBeat = beat - 0.25
			}
		})
		m.ctx.At(beat, func() {
			m.renewConveyerNormalizedOffset(beat)
			m.conveyerStartBeat = -2
		})
	} else {
		beat := ev.beat
		m.ctx.At(beat-0.5, func() {
			m.renewConveyerNormalizedOffset(beat - 0.5)
			m.conveyerStartBeat = -2
		})
	}

	remaining := []*bot{}
	for _, old := range m.bots {
		if old != b && old.conveyerRestartLength < 0 {
			remaining = append(remaining, old)
		}
	}
	if ev.stop {
		b.conveyerRestartLength = -1
	}
	beat := ev.beat
	m.ctx.At(beat+3, func() {
		if !m.ctx.PressingNow() && !m.fillerHolding {
			m.playFiller("FillerPrepare", beat+3, 0.5)
		}
		m.conveyerStartBeat = beat + 3
		m.fillerPosition = ev.size
		for _, old := range remaining {
			if old.state == stateIdle {
				old.conveyerRestartLength = 0.5
			}
			if old.conveyerStartBeat == -2 {
				old.conveyerStartBeat = beat + 3
			}
		}
	})
}

func (m *Module) fillErUp(beat float64) {
	m.ctx.SoundAt(beat-0.5, "fillErUp1", 1)
	m.ctx.SoundAt(beat-0.25, "fillErUp2", 1)
	m.ctx.SoundAt(beat, "fillErUp3", 1)
}

func (m *Module) bop(beat float64) {
	toggle := m.toggleGlobal
	for _, meter := range m.meters {
		state := "Up"
		if toggle&1 == 1 {
			state = "Down"
		}
		m.ctx.Scene.PlayState(meter, state, beat, 0.5)
		toggle ^= 1
	}
	m.toggleGlobal ^= 1
	for _, b := range m.bots {
		if b.state == stateDance {
			b.successDance(beat)
		}
	}
}

func (m *Module) autoBopAt(beat float64) bool {
	active := false
	for _, ev := range m.bops {
		if ev.beat > beat {
			break
		}
		active = ev.auto
	}
	return active
}

func (m *Module) updateConveyerBelt(beat float64) {
	n := normalized(m.conveyerStartBeat, 1, beat)
	v := m.conveyerNormalizedOffset
	if m.conveyerStartBeat >= 0 && n >= 0 {
		v += n
	}
	playTime := math.Mod(v, 1) / 4
	if playTime < 0 {
		playTime += 0.25
	}
	m.ctx.Scene.PlayNormalized(m.conveyerBelt, "Animations/ConveyerMove", playTime)
	for _, gear := range m.gears {
		m.ctx.Scene.SetSpinOver(gear, lerp(0, -math.Pi/2, v))
	}
}

func (m *Module) renewConveyerNormalizedOffset(beat float64) {
	if m.conveyerStartBeat == -1 || m.conveyerStartBeat == -2 {
		return
	}
	n := normalized(m.conveyerStartBeat, 1, beat)
	if n >= 0 {
		m.conveyerNormalizedOffset = math.Mod(m.conveyerNormalizedOffset+n, 4)
	}
}

func (m *Module) playFiller(state string, beat, timeScale float64) {
	m.ctx.Scene.PlayState(m.filler, state, beat, timeScale)
}

func (m *Module) firstHoldingBot() *bot {
	for _, b := range m.bots {
		if b.state == stateHolding {
			return b
		}
	}
	return nil
}

func (m *Module) compactBots(beat float64) {
	dst := m.bots[:0]
	for _, b := range m.bots {
		if !b.dead || beat < b.deadAt {
			dst = append(dst, b)
		}
	}
	m.bots = dst
}

func (m *Module) applyObjectColors(beat float64) {
	obj := m.objectAt(beat)
	m.ctx.Scene.SetPaletteFor("impact_mat", impactPalette(obj.impact))
	m.ctx.Scene.SetColorOver(m.conveyerBelt, obj.conveyer)
	for _, p := range m.fillerRenderer {
		m.ctx.Scene.SetColorOver(p, obj.filler)
	}
}

func (m *Module) objectAt(beat float64) objectEvt {
	cur := objectEvt{
		fuel: defaultFuel, lampOff: defaultLampOff, lampOn: defaultLampOn,
		impact: defaultImpact, filler: defaultRenderer, conveyer: defaultRenderer,
	}
	for _, ev := range m.objEvents {
		if ev.beat > beat {
			break
		}
		cur = ev
	}
	return cur
}

func (m *Module) bgAt(beat float64) ([4]float64, [6][4]float64) {
	bg := defaultBG
	meters := [6][4]float64{}
	for i := range meters {
		meters[i] = defaultMeter
	}
	for _, ev := range m.bgEvents {
		if ev.beat > beat {
			break
		}
		u := clamp01(normalized(ev.beat, ev.length, beat))
		bg = colorAtEase(ev.bg0, ev.bg1, ev.ease, u)
		for i := range meters {
			meters[i] = colorAtEase(ev.m0[i], ev.m1[i], ev.ease, u)
		}
	}
	return bg, meters
}

func (m *Module) applyBackgroundColors(bg [4]float64) {
	for _, p := range m.otherRenderer {
		m.ctx.Scene.SetColorOver(p, bg)
	}
}

func (m *Module) drawFlatSceneSurfaces(screen *ebiten.Image, bg [4]float64, meterColors [6][4]float64) {
	vector.DrawFilledRect(screen, 0, 0, engine.ScreenW, engine.ScreenH, rgba(bg), false)
	for i, path := range m.metersFuel {
		if i >= len(meterColors) {
			break
		}
		if world, ok := m.ctx.Scene.NodeWorld(path); ok {
			drawWorldRect(screen, m.proj, world, 1, 1, meterColors[i])
		}
	}
	if m.blackout {
		vector.DrawFilledRect(screen, 0, 0, engine.ScreenW, engine.ScreenH, rgba([4]float64{0, 0, 0, 1}), false)
	}
}

func drawWorldRect(screen *ebiten.Image, proj kart.Aff, world kart.Aff, w, h float64, c [4]float64) {
	p := proj.Mul(world)
	x0, y0 := p.Apply(-w/2, -h/2)
	x1, y1 := p.Apply(w/2, -h/2)
	x2, y2 := p.Apply(w/2, h/2)
	x3, y3 := p.Apply(-w/2, h/2)
	minX := math.Min(math.Min(x0, x1), math.Min(x2, x3))
	maxX := math.Max(math.Max(x0, x1), math.Max(x2, x3))
	minY := math.Min(math.Min(y0, y1), math.Min(y2, y3))
	maxY := math.Max(math.Max(y0, y1), math.Max(y2, y3))
	vector.DrawFilledRect(screen, float32(minX), float32(minY), float32(maxX-minX), float32(maxY-minY), rgba(c), true)
}

func bgEventFrom(e *riq.Entity) bgEvt {
	ev := bgEvt{
		beat: e.Beat, length: e.Length, ease: intDefault(e, "ease", 1),
		bg0: colorParam(e, "colorBGStart", defaultBG),
		bg1: colorParam(e, "colorBGEnd", defaultBG),
	}
	separate := boolParam(e, "separate")
	common0 := colorParam(e, "colorMetersStart", defaultMeter)
	common1 := colorParam(e, "colorMetersEnd", defaultMeter)
	for i := 0; i < 6; i++ {
		ev.m0[i], ev.m1[i] = common0, common1
		if separate {
			key := string(rune('1' + i))
			ev.m0[i] = colorParam(e, "colorMeter"+key+"Start", defaultMeter)
			ev.m1[i] = colorParam(e, "colorMeter"+key+"End", defaultMeter)
		}
	}
	return ev
}

func (m *Module) loadBotSpec(size botSize, root string) botSpec {
	as := m.ctx.Assets
	spec := botSpec{
		root: root, holdLength: map[botSize]float64{sizeSmall: 1, sizeMedium: 3, sizeLarge: 7}[size],
		limbFallHeight: 15, flyDistance: 2.05, stackDistanceRate: 0.34,
		legsBase: nodePos(as, root+"/Legs"), bodyBase: nodePos(as, root+"/Body"), headBase: nodePos(as, root+"/Head"),
		rootScale:  nodeScale(as, root),
		fillPosY:   animLastY(as, botFillClip(size), "", false),
		fillScaleY: animLastY(as, botFillClip(size), "", true),
	}
	for _, comp := range as.Extra.Components {
		if comp.Path != root {
			continue
		}
		if v, ok := comp.Nums["holdLength"]; ok {
			spec.holdLength = v
		}
		if v, ok := comp.Nums["limbFallHeight"]; ok {
			spec.limbFallHeight = v
		}
		if v, ok := comp.Nums["flyDistance"]; ok {
			spec.flyDistance = v
		}
		if v, ok := comp.Nums["stackDistanceRate"]; ok {
			spec.stackDistanceRate = v
		}
	}
	if spec.rootScale == [2]float64{} {
		spec.rootScale = [2]float64{1, 1}
	}
	return spec
}
