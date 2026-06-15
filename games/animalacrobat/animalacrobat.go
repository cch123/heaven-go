package animalacrobat

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

func (m *Module) ID() string { return "animalAcrobat" }

func (m *Module) Load(ctx *engine.Ctx) error {
	m.ctx = ctx
	if err := ctx.LoadAssets(m.ID()); err != nil {
		return err
	}
	m.proj = kart.Translate(engine.ScreenW/2, engine.ScreenH/2).Mul(kart.Scale(54, -54))
	m.player = ctx.Role("_playerMonkey")
	m.spotlight = ctx.Role("_spotlightMain")
	m.partyPoppers = ctx.Role("_partyPoppers")

	comps := ctx.Assets.Extra.Components
	if c, ok := comps["game"]; ok {
		m.gameNums = c.Nums
	}
	if c, ok := comps["player"]; ok {
		m.playerNums = c.Nums
	}
	if c, ok := comps["bgTileManager0"]; ok {
		m.bgTileA = ref(c.Refs, "_bgTileFirst")
		m.bgTileB = ref(c.Refs, "_bgTileSecond")
	}

	for _, bind := range []struct {
		kind animalKind
		role string
	}{
		{kindElephant, "_elephant"},
		{kindGiraffe, "_giraffe"},
		{kindMonkeysLong, "_monkeysLong"},
		{kindMonkeyShort, "_monkeysShort"},
		{kindGorilla, "_gorilla"},
	} {
		root := ctx.Role(bind.role)
		t := kart.NewTemplate(ctx.Assets, root)
		if t == nil {
			continue
		}
		m.templates[bind.kind] = t
		ctx.Scene.SetActive(root, false)
		m.specs[bind.kind] = m.makeObstacleSpec(root, obstacleComponent(comps, root))
		if bind.kind != kindGorilla {
			m.inputs[bind.kind] = m.makeInputSpec(root, inputComponent(comps, root, bind.kind == kindGiraffe))
		}
	}

	ctx.Scene.SetActive(m.partyPoppers, false)
	ctx.Scene.SetActive(m.spotlight, true)
	ctx.Scene.PlayDefaultState(m.player, 0, ctx.SecPerBeat(0))
	return nil
}

func (m *Module) makeObstacleSpec(root string, c kmdata.Component) obstacleSpec {
	s := obstacleSpec{
		holdLength:       num(c.Nums, "_holdLength", 1),
		holdPadding:      num(c.Nums, "_holdPadding", 0),
		holdPaddingStart: num(c.Nums, "_holdPaddingStart", 0),
		fullRotRange:     num(c.Nums, "_fullRotRange", 160),
		ease:             int(num(c.Nums, "_ease", 0)),
		rotateRoot:       ref(c.Refs, "_rotateRoot"),
		gripPoint:        ref(c.Refs, "_gripPoint"),
		endPoint:         ref(c.Refs, "_endPoint"),
	}
	s.rotRel = relPath(root, s.rotateRoot)
	return s
}

func (m *Module) makeInputSpec(root string, c kmdata.Component) inputSpec {
	in := inputSpec{
		holdLength: num(c.Nums, "_holdLength", 1),
		monkey:     ref(c.Refs, "_monkey"),
	}
	in.monkeyRel = relPath(root, in.monkey)
	return in
}

func (m *Module) OnEvent(e *riq.Entity) {
	switch e.Datamodel {
	case "animalAcrobat/bop":
		m.bops = append(m.bops, bopCall{beat: e.Beat, length: e.Length})
	case "animalAcrobat/elephant":
		m.rawAnimals = append(m.rawAnimals, animalCall{beat: e.Beat, length: 4, kind: kindElephant})
	case "animalAcrobat/giraffe":
		m.rawAnimals = append(m.rawAnimals, animalCall{beat: e.Beat, length: 8, kind: kindGiraffe})
	case "animalAcrobat/monkeys":
		m.rawAnimals = append(m.rawAnimals, animalCall{beat: e.Beat, length: 5, kind: kindMonkeysLong})
		m.rawAnimals = append(m.rawAnimals, animalCall{beat: e.Beat + 5, length: 3, kind: kindMonkeyShort})
	case "animalAcrobat/monkeyLong":
		m.rawAnimals = append(m.rawAnimals, animalCall{beat: e.Beat, length: 5, kind: kindMonkeysLong})
	case "animalAcrobat/monkeyShort":
		m.rawAnimals = append(m.rawAnimals, animalCall{beat: e.Beat, length: 3, kind: kindMonkeyShort})
	case "animalAcrobat/toggleSpotlight":
		active := boolParam(e, "active", true)
		m.ctx.At(e.Beat, func() { m.ctx.Scene.SetActive(m.spotlight, active) })
	case "animalAcrobat/confetti":
		beat := e.Beat
		m.ctx.At(beat, func() { m.popConfetti(beat) })
	case "animalAcrobat/bgcolor":
		m.bgEvents = append(m.bgEvents, bgEvent{
			beat:   e.Beat,
			length: e.Length,
			fromA:  colorParam(e, "colorAStart", defaultBGAlpha),
			toA:    colorParam(e, "colorA", defaultBGAlpha),
			fromB:  colorParam(e, "colorBStart", defaultBGBravo),
			toB:    colorParam(e, "colorB", defaultBGBravo),
			ease:   int(e.Float("ease", 0)),
		})
	}
}

func (m *Module) Ready() {
	sort.Slice(m.rawAnimals, func(i, j int) bool { return m.rawAnimals[i].beat < m.rawAnimals[j].beat })
	sort.Slice(m.bops, func(i, j int) bool { return m.bops[i].beat < m.bops[j].beat })
	sort.Slice(m.bgEvents, func(i, j int) bool { return m.bgEvents[i].beat < m.bgEvents[j].beat })
	m.buildAnimalQueue()
	m.scheduleBops()
	for _, ev := range m.bgEvents {
		ev := ev
		m.ctx.At(ev.beat, func() {
			m.bg = bgEase{beat: ev.beat, length: ev.length, fromA: ev.fromA, toA: ev.toA, fromB: ev.fromB, toB: ev.toB, ease: ev.ease}
		})
	}
}

func (m *Module) buildAnimalQueue() {
	m.animals = nil
	if len(m.rawAnimals) == 0 {
		return
	}
	nextBeat := m.rawAnimals[0].beat
	summated := num(m.gameNums, "_jumpStartDistance", defaultJumpStartDistance)
	for _, call := range m.rawAnimals {
		if math.Abs(call.beat-nextBeat) > 0.01 {
			continue
		}
		ob := m.newObstacle(call, summated)
		if ob == nil {
			nextBeat += call.length
			continue
		}
		m.animals = append(m.animals, ob)
		dist := rotationDistance(m.ctx.Assets, ob.spec)
		summated += dist
		if call.kind == kindGiraffe {
			summated += num(m.playerNums, "_jumpDistanceGiraffe", defaultJumpDistanceGiraffe)
		} else {
			summated += num(m.playerNums, "_jumpDistance", defaultJumpDistance)
		}
		nextBeat += call.length
	}
	if len(m.animals) == 0 {
		return
	}
	last := m.animals[len(m.animals)-1]
	gorilla := m.newObstacle(animalCall{beat: last.beat + last.length, length: 4, kind: kindGorilla}, summated+num(m.gameNums, "_jumpStartCameraDistance", defaultJumpStartCameraDelta))
	if gorilla != nil {
		gorilla.end = true
		m.animals = append(m.animals, gorilla)
	}
	first := m.animals[0]
	startBeat := first.beat - 1
	m.ctx.SoundAt(startBeat, "start", 1)
	m.ctx.At(startBeat, func() { m.startInitialJump(startBeat, first) })
	for i, ob := range m.animals {
		if ob.kind == kindGorilla {
			continue
		}
		ob.end = i+1 >= len(m.animals)-1
		m.scheduleObstacle(ob)
	}
}

func (m *Module) newObstacle(call animalCall, x float64) *acrobatObstacle {
	t := m.templates[call.kind]
	if t == nil {
		return nil
	}
	spec := m.specs[call.kind]
	in := m.inputs[call.kind]
	inst := t.NewInstance()
	inst.Offset = [2]float64{x, 0}
	inst.PlayDefaultState("", 0, m.ctx.SecPerBeat(0))
	for _, root := range []string{"GiraffeRoot", "FireHoop", "WhiteMonkeysPivot", "TrunkRoot/PlayerMonkey", "GiraffeRoot/PlayerMonkey", "MonkeyPivot/PlayerMonkey", "WhiteMonkeysPivot/WhiteMonkey (3)/PlayerMonkey"} {
		inst.PlayDefaultState(root, 0, m.ctx.SecPerBeat(0))
	}
	gx, gy := nodePos(m.ctx.Assets, spec.gripPoint)
	ex, ey := nodePos(m.ctx.Assets, spec.endPoint)
	return &acrobatObstacle{
		kind: call.kind, beat: call.beat, length: call.length, inst: inst, spec: spec, input: in,
		x: x, gripX: gx, gripY: gy, endX: ex, endY: ey, canHit: true,
	}
}

func (m *Module) scheduleBops() {
	for _, b := range m.bops {
		end := b.beat + b.length
		for beat := math.Ceil(b.beat); beat < end; beat++ {
			if m.firstJumpBeat() > 0 && beat >= m.firstJumpBeat() {
				continue
			}
			beat := beat
			m.ctx.At(beat, func() {
				if m.holding == nil {
					m.playPlayer("PlayerBop", beat)
				}
			})
		}
	}
}

func (m *Module) firstJumpBeat() float64 {
	if len(m.animals) == 0 {
		return math.Inf(1)
	}
	return m.animals[0].beat - 1
}

func (m *Module) OnSwitch(beat float64) {
	m.bg = bgEase{fromA: defaultBGAlpha, toA: defaultBGAlpha, fromB: defaultBGBravo, toB: defaultBGBravo}
	for _, ev := range m.bgEvents {
		if ev.beat <= beat {
			m.bg = bgEase{beat: ev.beat, length: ev.length, fromA: ev.fromA, toA: ev.toA, fromB: ev.fromB, toB: ev.toB, ease: ev.ease}
		}
	}
	m.lastBop = int(math.Floor(beat)) - 1
	m.holding = nil
	m.drumrollStop = nil
	m.ctx.Scene.SetActive(m.partyPoppers, false)
	m.ctx.Scene.SetActive(m.player, true)
	m.ctx.Scene.PlayDefaultState(m.player, beat, m.ctx.SecPerBeat(beat))
}

func (m *Module) Whiff(beat float64) {
	if m.holding == nil && !m.jumpActive {
		m.playPlayer("PlayerBop", beat)
	}
}

func (m *Module) Update(t, beat float64) {
	m.updateObstacleRotation(beat)
	m.updateEarlyRelease(beat)
	m.updatePlayer(beat)
	m.updateAutoBop(beat)
	m.updateCamera(beat)
	cam := m.ctx.SampleScene(beat)
	m.cameraWX, m.cameraWY = cam[0]+m.cameraX, cam[1]
	m.ctx.Scene.SetCamera(m.cameraWX, m.cameraWY, cam[2])
}

func (m *Module) Draw(screen *ebiten.Image, t, beat float64) {
	a, b := m.bg.colorsAt(beat)
	screen.Fill(toRGBA(a))
	if m.bgTileA != "" {
		m.ctx.Scene.SetMaterialOver(m.bgTileA, b, [4]float64{})
	}
	if m.bgTileB != "" {
		m.ctx.Scene.SetMaterialOver(m.bgTileB, b, [4]float64{})
	}
	for _, ob := range m.animals {
		if beat < ob.beat-6 || beat > ob.beat+ob.length+8 {
			continue
		}
		ob.inst.Queue(m.ctx.Scene, beat, kart.Identity(), 0)
	}
	m.ctx.Scene.Draw(screen, m.proj)
	m.drawSparkles(screen, beat)
}

func (m *Module) popConfetti(beat float64) {
	m.ctx.Scene.SetActive(m.partyPoppers, true)
	m.ctx.Scene.PlayState("PartyPoppers/ConfettiL", "PopIntro", beat, m.ctx.SecPerBeat(beat))
	m.ctx.Scene.PlayState("PartyPoppers/ConfettiR", "PopIntro", beat, m.ctx.SecPerBeat(beat))
	delay := 0.23333333 / m.ctx.SecPerBeat(beat)
	m.ctx.SoundAt(beat+delay, "cracker", 1)
	popperOffBeat := beat + delay + 1.2
	m.emitSparkle(beat+delay, -7.3, -2.9, color.NRGBA{255, 255, 255, 230})
	m.emitSparkle(beat+delay, 7.3, -2.9, color.NRGBA{255, 255, 120, 230})
	m.ctx.At(popperOffBeat, func() { m.ctx.Scene.SetActive(m.partyPoppers, false) })
}
