// Package froghop ports Frog Hop's frog controller states, count-ins,
// Ya-hoo/Yeah yeah yeah/Spin response cues, spotlights, whiffs, and core
// custom appearance events.
package froghop

import (
	"math"
	"sort"

	"github.com/hajimehoshi/ebiten/v2"

	"hsdemo/engine"
	"hsdemo/kart"
	"hsdemo/riq"
)

type Module struct {
	ctx  *engine.Ctx
	proj kart.Aff

	player *frog
	leader *frog
	singer *frog
	other  []*frog
	front  []*frog
	back   []*frog
	all    []*frog

	darkness, spotFront, spotBack string
	spotFrontColor, spotBackColor string
	mikeL, mikeR                  string
	stage, stageTop               string
	bgHigh, bgLow, gradient       string

	bops       []bopEvt
	counts     []countEvt
	countForce []countForceEvt
	hops       []hopEvt
	cues       []cueEvt
	thanks     []thankEvt
	mouths     []mouthEvt
	spots      []spotlightEvt
	bgs        []bgEvt
	stages     []stageEvt
	frogColors []frogColorEvt
	forces     []forceEvt
	pitches    []pitchEvt
	disables   []disableEvt

	globalSide int
	lastPulse  int
}

func New() engine.Module { return &Module{globalSide: -1, lastPulse: -1} }

func (m *Module) ID() string { return gameID }

func (m *Module) Load(ctx *engine.Ctx) error {
	m.ctx = ctx
	if err := ctx.LoadAssets(gameID); err != nil {
		return err
	}
	m.proj = kart.Translate(engine.ScreenW/2, engine.ScreenH/2).Mul(kart.Scale(54, -54))
	m.darkness = roleOr(ctx, "Darkness", "Darkness")
	m.spotFront = roleOr(ctx, "SpotlightFront", "SpotlightsFront")
	m.spotBack = roleOr(ctx, "SpotlightBack", "SpotlightsBack")
	m.spotFrontColor = roleOr(ctx, "SpotlightFrontColor", "SpotlightsFront/SpotlightsFrontColor")
	m.spotBackColor = roleOr(ctx, "SpotlightBackColor", "SpotlightsBack/SpotlightsBackColor")
	m.mikeL = roleOr(ctx, "Mike", "Mike")
	m.mikeR = roleOr(ctx, "Mike2", "Mike2")
	m.stage = roleOr(ctx, "Stage", "Stage")
	m.stageTop = roleOr(ctx, "StageTop", "Stage/StageTop")
	m.bgHigh = roleOr(ctx, "bgHigh", "BG/Color1")
	m.bgLow = roleOr(ctx, "bgLow", "BG/Color2")
	m.gradient = roleOr(ctx, "gradient", "BG/Gradient")

	m.player = newFrog(ctx, roleOr(ctx, "PlayerFrog", "frogPlayer"), frogBackup)
	m.leader = newFrog(ctx, roleOr(ctx, "LeaderFrog", "frogLeader"), frogLeader)
	m.singer = newFrog(ctx, roleOr(ctx, "SingerFrog", "frogSinger"), frogSinger)
	for _, p := range ctx.Assets.Extra.RefArrays["OtherFrogs"] {
		m.other = append(m.other, newFrog(ctx, p, frogBackup))
	}
	if len(m.other) == 0 {
		for _, p := range []string{"frog1", "frog2", "frog3"} {
			m.other = append(m.other, newFrog(ctx, p, frogBackup))
		}
	}
	m.front = []*frog{m.leader, m.singer}
	m.back = append([]*frog{m.player}, m.other...)
	m.all = append(append([]*frog{m.player}, m.other...), m.leader, m.singer)
	m.initScene(0)
	return nil
}

func (m *Module) OnEvent(e *riq.Entity) {
	switch actionName(e) {
	case "bop":
		m.bops = append(m.bops, bopEvt{beat: e.Beat, length: e.Length, blue: boolDefault(e, "blue", true), orange: boolDefault(e, "orange", true), green: boolDefault(e, "greens", true)})
	case "count":
		m.counts = append(m.counts, countEvt{beat: e.Beat, start: boolDefault(e, "start", true), leader: boolDefault(e, "leader", true), backup: boolParam(e, "backup")})
	case "countforce":
		m.countForce = append(m.countForce, countForceEvt{beat: e.Beat, number: intDefault(e, "syllable", 0), leader: boolDefault(e, "leader", true), backup: boolParam(e, "backup")})
	case "hop":
		m.hops = append(m.hops, hopEvt{beat: e.Beat})
	case "stop":
		m.hops = append(m.hops, hopEvt{beat: e.Beat, stop: true})
	case "twoshake":
		m.cues = append(m.cues, cueEvt{beat: e.Beat, kind: "two", spotlights: boolDefault(e, "spotlights", true), jazz: boolParam(e, "jazz"), enabled: boolDefault(e, "enabled", true)})
	case "threeshake":
		m.cues = append(m.cues, cueEvt{beat: e.Beat, kind: "three", spotlights: boolDefault(e, "spotlights", true), jazz: boolParam(e, "jazz"), enabled: boolDefault(e, "enabled", true)})
	case "spin":
		m.cues = append(m.cues, cueEvt{beat: e.Beat, kind: "spin", spotlights: boolDefault(e, "spotlights", true), jazz: boolParam(e, "jazz"), enabled: boolDefault(e, "enabled", true), hs: boolParam(e, "hs")})
	case "thankyou":
		m.thanks = append(m.thanks, thankEvt{beat: e.Beat, pitched: boolParam(e, "pitched"), manual: boolParam(e, "override"), pitch: floatDefault(e, "overPitch", 1)})
	case "mouthwide", "mouthnarrow":
		state := "Wide"
		if actionName(e) == "mouthnarrow" {
			state = "Narrow"
		}
		m.mouths = append(m.mouths, mouthEvt{beat: e.Beat, length: e.Length, state: state, blue: boolDefault(e, "blue", true), orange: boolParam(e, "orange"), green: boolParam(e, "greens")})
	case "mouthspecial":
		m.mouths = append(m.mouths, mouthEvt{beat: e.Beat, length: e.Length, state: "Special", wink: true, blue: boolDefault(e, "blue", true), orange: boolParam(e, "orange"), green: boolParam(e, "greens")})
	case "spotlights":
		m.spots = append(m.spots, spotlightEvt{beat: e.Beat, front: boolDefault(e, "front", true), back: boolParam(e, "back"), dark: boolDefault(e, "dark", true)})
	case "changeBgColor":
		m.bgs = append(m.bgs, bgEvt{
			beat: e.Beat, length: e.Length, ease: intDefault(e, "ease", 1),
			fromTop: colorParam(e, "colorFrom", defaultBGTop), toTop: colorParam(e, "colorTo", defaultBGTop),
			fromBottom: colorParam(e, "colorFrom2", defaultBGBottom), toBottom: colorParam(e, "colorTo2", defaultBGBottom),
		})
	case "colorStage":
		m.stages = append(m.stages, stageEvt{
			beat:  e.Beat,
			top:   colorParam(e, "color1", [4]float64{1, 1, 1, 1}),
			rim:   colorParam(e, "color2", [4]float64{0xc0 / 255.0, 0xf3 / 255.0, 0x6d / 255.0, 1}),
			trim:  colorParam(e, "color3", [4]float64{0xd5 / 255.0, 0xf6 / 255.0, 0x5a / 255.0, 1}),
			base:  colorParam(e, "color4", [4]float64{0x94 / 255.0, 0xc5 / 255.0, 0x39 / 255.0, 1}),
			mikeL: boolDefault(e, "mikeL", true), mikeR: boolParam(e, "mikeR"),
			front: colorParam(e, "color5", white), back: colorParam(e, "color6", white),
		})
	case "colorSingerFrog":
		m.frogColors = append(m.frogColors, m.frogColorEvent(e, "singer"))
	case "colorLeaderFrog":
		m.frogColors = append(m.frogColors, m.frogColorEvent(e, "leader"))
	case "colorBackupFrog":
		m.frogColors = append(m.frogColors, m.frogColorEvent(e, "backup"))
	case "disableBlue":
		m.disables = append(m.disables, disableEvt{beat: e.Beat, disable: boolDefault(e, "disable", true)})
	case "force":
		m.forces = append(m.forces, forceEvt{beat: e.Beat, length: e.Length, front: boolDefault(e, "front", true), back: boolDefault(e, "back", true)})
	case "pitching":
		m.pitches = append(m.pitches, pitchEvt{beat: e.Beat, enabled: boolParam(e, "pitched"), manual: boolParam(e, "override"), pitch: floatDefault(e, "overPitch", 1)})
	}
}

func (m *Module) Ready() {
	m.sortEvents()
	for _, ev := range m.bops {
		ev := ev
		for i := 0; i < int(ev.length); i++ {
			b := ev.beat + float64(i)
			m.ctx.At(b, func() { m.bop(ev.blue, ev.orange, ev.green, b) })
		}
	}
	for _, ev := range m.counts {
		ev := ev
		m.scheduleCount(ev)
	}
	for _, ev := range m.countForce {
		ev := ev
		m.scheduleCountForce(ev)
	}
	m.scheduleRegularHops()
	for _, ev := range m.cues {
		ev := ev
		m.scheduleCue(ev)
	}
	for _, ev := range m.thanks {
		ev := ev
		m.scheduleThank(ev)
	}
	for _, ev := range m.mouths {
		ev := ev
		m.ctx.At(ev.beat, func() { m.scheduleMouth(ev) })
	}
	for _, ev := range m.spots {
		ev := ev
		m.ctx.At(ev.beat, func() { m.setSpotlights(ev.front, ev.back, ev.dark) })
	}
	for _, ev := range m.stages {
		ev := ev
		m.ctx.At(ev.beat, func() { m.applyStage(ev) })
	}
	for _, ev := range m.frogColors {
		ev := ev
		m.ctx.At(ev.beat, func() { m.applyFrogColor(ev) })
	}
	for _, ev := range m.disables {
		ev := ev
		m.ctx.At(ev.beat, func() { m.ctx.Scene.SetActive(m.singer.path, !ev.disable) })
	}
	for _, ev := range m.forces {
		ev := ev
		m.scheduleForce(ev)
	}
}

func (m *Module) OnSwitch(beat float64) {
	m.lastPulse = int(math.Floor(beat)) - 1
	m.initScene(beat)
	m.applyPersistent(beat)
}

func (m *Module) Whiff(beat float64) { m.WhiffAction(beat, 0) }

func (m *Module) WhiffAction(beat float64, action int) {
	if action == actionAlt {
		m.player.charge(beat, 0)
		m.ctx.PlayCommon("miss")
		m.lightMiss(true, false)
		return
	}
	m.player.hop(beat, 0, false)
	m.ctx.PlayCommon("miss")
	m.lightMiss(true, false)
}

func (m *Module) Update(_ float64, beat float64) {
	for pulse := m.lastPulse + 1; pulse <= int(math.Floor(beat)); pulse++ {
		m.lastPulse = pulse
	}
}

func (m *Module) Draw(screen *ebiten.Image, _ float64, beat float64) {
	top, bottom := m.bgAt(beat)
	screen.Fill(rgba(bottom))
	m.ctx.Scene.SetColorOver(m.bgHigh, top)
	m.ctx.Scene.SetColorOver(m.gradient, top)
	m.ctx.Scene.SetColorOver(m.bgLow, bottom)
	m.ctx.SampleScene(beat)
	m.ctx.Scene.Draw(screen, m.proj)
}

func (m *Module) initScene(beat float64) {
	for _, f := range m.all {
		f.reset(beat)
	}
	m.setSpotlights(false, false, false)
	m.ctx.Scene.SetActive(m.singer.path, true)
	m.applyStage(stageEvt{top: [4]float64{1, 1, 1, 1}, rim: [4]float64{0xc0 / 255.0, 0xf3 / 255.0, 0x6d / 255.0, 1}, trim: [4]float64{0xd5 / 255.0, 0xf6 / 255.0, 0x5a / 255.0, 1}, base: [4]float64{0x94 / 255.0, 0xc5 / 255.0, 0x39 / 255.0, 1}, mikeL: true, front: white, back: white})
}

func (m *Module) sortEvents() {
	sort.SliceStable(m.bops, func(i, j int) bool { return m.bops[i].beat < m.bops[j].beat })
	sort.SliceStable(m.counts, func(i, j int) bool { return m.counts[i].beat < m.counts[j].beat })
	sort.SliceStable(m.countForce, func(i, j int) bool { return m.countForce[i].beat < m.countForce[j].beat })
	sort.SliceStable(m.hops, func(i, j int) bool { return m.hops[i].beat < m.hops[j].beat })
	sort.SliceStable(m.cues, func(i, j int) bool { return m.cues[i].beat < m.cues[j].beat })
	sort.SliceStable(m.thanks, func(i, j int) bool { return m.thanks[i].beat < m.thanks[j].beat })
	sort.SliceStable(m.mouths, func(i, j int) bool { return m.mouths[i].beat < m.mouths[j].beat })
	sort.SliceStable(m.spots, func(i, j int) bool { return m.spots[i].beat < m.spots[j].beat })
	sort.SliceStable(m.bgs, func(i, j int) bool { return m.bgs[i].beat < m.bgs[j].beat })
	sort.SliceStable(m.stages, func(i, j int) bool { return m.stages[i].beat < m.stages[j].beat })
	sort.SliceStable(m.frogColors, func(i, j int) bool { return m.frogColors[i].beat < m.frogColors[j].beat })
	sort.SliceStable(m.forces, func(i, j int) bool { return m.forces[i].beat < m.forces[j].beat })
	sort.SliceStable(m.pitches, func(i, j int) bool { return m.pitches[i].beat < m.pitches[j].beat })
	sort.SliceStable(m.disables, func(i, j int) bool { return m.disables[i].beat < m.disables[j].beat })
}
