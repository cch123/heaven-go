package nightwalkagb

import (
	"math"
	"math/rand"
	"sort"

	"github.com/hajimehoshi/ebiten/v2"

	"hsdemo/engine"
	"hsdemo/kart"
	"hsdemo/kmdata"
	"hsdemo/riq"
)

type Module struct {
	ctx  *engine.Ctx
	proj kart.Aff

	playerRoot  string
	textPath    string
	textbox     string
	textboxBody string

	bgMat            string
	starMat          string
	platformMat      string
	platformLightMat string
	fishMat          string

	platformT *kart.Template
	starT     *kart.Template
	cfg       platformCfg
	starCfg   starCfg
	player    playYan
	stars     starField
	platforms []*platformInst

	countIns   []countInEvt
	rawHeights []rawHeightEvt
	heights    []heightEvt
	types      []typeEvt
	fishBeats  []float64
	rolls      []rollEvt
	ends       []endEvt
	noJumps    []noJumpEvt
	textboxes  []textboxEvt
	bgs        []bgEvt
	colors     []colorEvt

	countInBeat    float64
	countInLength  float64
	endBeat        float64
	requiredJumps  int
	requiredJumpsP int
	evolveAmount   int
	hitJumps       int

	raiseBeat     float64
	lastHeight    float64
	heightToRaise float64
	holderY       float64
	lastUnits     int
	currentUnits  int
	stopStars     bool
	stopped       bool

	bg        bgEvt
	lastPulse int
	rng       *rand.Rand
}

func New() engine.Module {
	return &Module{
		countInBeat: math.Inf(-1), countInLength: 8, endBeat: math.Inf(1),
		evolveAmount: 1, raiseBeat: math.Inf(-1), lastPulse: -1,
		bg:  bgEvt{startTop: black, endTop: black, startBottom: black, endBottom: black},
		rng: rand.New(rand.NewSource(0x1919a6b)),
	}
}

func (m *Module) ID() string { return gameID }

func (m *Module) Load(ctx *engine.Ctx) error {
	m.ctx = ctx
	if err := ctx.LoadAssets(gameID); err != nil {
		return err
	}
	if err := ctx.Assets.ApplyTexts(); err != nil {
		return err
	}
	m.proj = kart.Translate(engine.ScreenW/2, engine.ScreenH/2).Mul(kart.Scale(54, -54))

	comps := ctx.Assets.Extra.Components
	game := comps["game"]
	m.playerRoot = roleOr(ctx, "playYan", game.Refs["playYan"])
	m.textPath = roleOr(ctx, "Text", game.Refs["Text"])
	m.textbox = roleOr(ctx, "TextboxGO", game.Refs["TextboxGO"])
	m.textboxBody = roleOr(ctx, "TextboxSprite", game.Refs["TextboxSprite"])
	m.bgMat = game.Refs["BGMat"]
	m.starMat = game.Refs["StarMat"]
	m.platformMat = game.Refs["PlatformMat"]
	m.platformLightMat = game.Refs["PlatLightMat"]
	m.fishMat = game.Refs["FishMat"]

	m.cfg = readPlatformCfg(comps["platformHandler"], comps["platform"])
	m.starCfg = readStarCfg(comps["starHandler"])
	m.platformT = kart.NewTemplate(ctx.Assets, m.cfg.root)
	m.starT = kart.NewTemplate(ctx.Assets, m.starCfg.root)
	m.player = newPlayYan(m, game)
	m.stars = newStarField(m)

	ctx.Scene.SetActive(m.cfg.root, false)
	ctx.Scene.SetActive(m.starCfg.root, false)
	ctx.Scene.SetActive(m.textbox, false)
	m.applyColors(defaultColorEvt(0))
	m.applyBG(m.bg)
	return nil
}

func (m *Module) OnEvent(e *riq.Entity) {
	b := e.Beat
	switch actionName(e) {
	case "countIn8":
		m.countIns = append(m.countIns, countInEvt{beat: b, length: e.Length, mute: boolParam(e, "mute"), kind: 8})
	case "countIn4":
		m.countIns = append(m.countIns, countInEvt{beat: b, length: e.Length, mute: boolParam(e, "mute"), kind: 4})
	case "height":
		m.rawHeights = append(m.rawHeights, rawHeightEvt{
			beat: b, value: intDefault(e, "value", 1),
			rmin: intDefault(e, "rmin", 0), rmax: intDefault(e, "rmax", 0),
		})
	case "type":
		ev := typeEvt{beat: b, platformType: intDefault(e, "type", platformLollipop), fillType: intDefault(e, "fill", fillNone)}
		m.types = append(m.types, ev)
		if ev.platformType == platformUmbrella {
			m.scheduleFillSound(ev.beat, ev.fillType)
		}
	case "fish":
		m.fishBeats = append(m.fishBeats, b)
		if boolParam(e, "cue") {
			m.playFishCue(b)
		}
	case "roll":
		m.rolls = append(m.rolls, rollEvt{beat: b, length: e.Length, mute: boolParam(e, "mute")})
	case "end":
		m.ends = append(m.ends, endEvt{beat: b, minAmount: intDefault(e, "minAmount", 10), minAmountP: intDefault(e, "minAmountP", 0)})
	case "noJump":
		m.noJumps = append(m.noJumps, noJumpEvt{beat: b, length: e.Length})
	case "walkingCountIn":
		m.walkingCountIn(b, e.Length)
	case "display textbox":
		m.textboxes = append(m.textboxes, textboxEvt{
			beat: b, length: e.Length,
			text: stringDefault(e, "text", "Let's try to reach the stars before the music ends!"),
			x:    floatDefault(e, "x", 0), y: floatDefault(e, "y", 2),
			width: floatDefault(e, "valA", 3), height: floatDefault(e, "valB", 1),
		})
	case "evolveAmount":
		amount := intDefault(e, "am", 1)
		m.ctx.At(b, func() { m.evolveAmount = amount })
	case "forceEvolve":
		amount, repeat := intDefault(e, "am", 1), intDefault(e, "repeat", 1)
		for i := 0; i < repeat; i++ {
			beat := b + e.Length*float64(i)
			m.ctx.At(beat, func() { m.stars.evolve(amount) })
		}
	case "changeBgColor":
		ev := bgEvt{
			beat: b, length: e.Length,
			startTop: colorParam(e, "startTop", black), endTop: colorParam(e, "endTop", black),
			startBottom: colorParam(e, "startBtm", black), endBottom: colorParam(e, "endBtm", black),
			ease: intDefault(e, "ease", 0),
		}
		m.bgs = append(m.bgs, ev)
		m.ctx.At(b, func() { m.bg = ev })
	case "setColors":
		ev := colorEvt{
			beat:          b,
			platform:      colorParam(e, "platform", defaultPlatform),
			platformBeam:  colorParam(e, "platformBeam", defaultPlatformBeam),
			platformLight: colorParam(e, "platformLight", defaultPlatformLight),
			fish:          colorParam(e, "fish", defaultFish),
			fishShock:     colorParam(e, "fishShock", defaultFishShock),
			fishShade:     colorParam(e, "fishShockShade", defaultFishShade),
			star:          colorParam(e, "star", defaultStar),
			starFace:      colorParam(e, "starFace", defaultStarFace),
		}
		m.colors = append(m.colors, ev)
		m.ctx.At(b, func() { m.applyColors(ev) })
	}
}

func (m *Module) Ready() {
	m.sortEvents()
	m.selectCountInAndEnd(0)
	m.markValidRolls()
	m.buildHeights()
	m.scheduleCountIns()
	m.scheduleRollCues()
	m.scheduleTextboxes()
	m.spawnPlatforms(0)
}

func (m *Module) OnSwitch(beat float64) {
	m.selectCountInAndEnd(beat)
	m.persistState(beat)
	m.resetRuntime(beat)
}

func (m *Module) Whiff(beat float64) {
	if !m.player.expectingPrimary(beat) {
		m.player.whiff(beat)
	}
}

func (m *Module) WhiffAction(beat float64, action int) {
	if action == actionAlt {
		return
	}
	m.Whiff(beat)
}

func (m *Module) Update(_, beat float64) {
	m.updatePulse(beat)
	m.updateHeight(beat)
	m.player.update(beat)
	m.stars.update(beat)
	for _, p := range m.platforms {
		p.update(beat)
	}
}

func (m *Module) Draw(screen *ebiten.Image, _, beat float64) {
	m.applyBGAt(beat)
	screen.Fill(rgba(m.bgColorAt(beat, false)))
	m.ctx.SampleScene(beat)
	for _, p := range m.platforms {
		p.queue(m.ctx.Scene, beat)
	}
	m.stars.queue(m.ctx.Scene, beat)
	m.ctx.Scene.Draw(screen, m.proj)
}

func (m *Module) sortEvents() {
	sort.Slice(m.countIns, func(i, j int) bool { return m.countIns[i].beat < m.countIns[j].beat })
	sort.Slice(m.rawHeights, func(i, j int) bool { return m.rawHeights[i].beat < m.rawHeights[j].beat })
	sort.Slice(m.types, func(i, j int) bool { return m.types[i].beat < m.types[j].beat })
	sort.Slice(m.fishBeats, func(i, j int) bool { return m.fishBeats[i] < m.fishBeats[j] })
	sort.Slice(m.rolls, func(i, j int) bool { return m.rolls[i].beat < m.rolls[j].beat })
	sort.Slice(m.ends, func(i, j int) bool { return m.ends[i].beat < m.ends[j].beat })
	sort.Slice(m.noJumps, func(i, j int) bool { return m.noJumps[i].beat < m.noJumps[j].beat })
	sort.Slice(m.textboxes, func(i, j int) bool { return m.textboxes[i].beat < m.textboxes[j].beat })
	sort.Slice(m.bgs, func(i, j int) bool { return m.bgs[i].beat < m.bgs[j].beat })
	sort.Slice(m.colors, func(i, j int) bool { return m.colors[i].beat < m.colors[j].beat })
}

func (m *Module) selectCountInAndEnd(beat float64) {
	m.countInBeat, m.countInLength = math.Inf(-1), 8
	for _, ev := range m.countIns {
		if ev.beat <= beat || len(m.countIns) == 1 {
			m.countInBeat, m.countInLength = ev.beat, ev.length
		}
	}
	if len(m.countIns) > 0 && math.IsInf(m.countInBeat, -1) {
		last := m.countIns[len(m.countIns)-1]
		m.countInBeat, m.countInLength = last.beat, last.length
	}
	m.endBeat, m.requiredJumps, m.requiredJumpsP = math.Inf(1), 0, 0
	for _, ev := range m.ends {
		if ev.beat >= beat {
			m.endBeat, m.requiredJumps, m.requiredJumpsP = ev.beat, ev.minAmount, ev.minAmountP
		}
	}
	if math.IsInf(m.endBeat, 1) && len(m.ends) > 0 {
		last := m.ends[len(m.ends)-1]
		m.endBeat, m.requiredJumps, m.requiredJumpsP = last.beat, last.minAmount, last.minAmountP
	}
}

func (m *Module) markValidRolls() {
	for i := range m.rolls {
		ev := &m.rolls[i]
		ev.valid = true
		for j := range m.rolls {
			if i == j {
				continue
			}
			other := m.rolls[j]
			if ev.beat >= other.beat && ev.beat < other.beat+other.length {
				ev.valid = false
				break
			}
		}
		if !ev.valid {
			continue
		}
		countBeat, countLen := m.countInBeat, m.countInLength
		if math.IsInf(countBeat, -1) {
			countBeat, countLen = 0, 0
		}
		if ev.beat < countBeat+countLen || math.Abs(math.Mod(ev.beat-countBeat, 1)) > 1e-6 {
			ev.valid = false
		}
	}
}

func (m *Module) buildHeights() {
	m.heights = nil
	for _, ev := range m.rawHeights {
		if m.heightSuppressedByRoll(ev.beat) {
			continue
		}
		add := 0
		if ev.rmax >= ev.rmin {
			add = ev.rmin + m.rng.Intn(ev.rmax-ev.rmin+1)
		}
		m.heights = append(m.heights, heightEvt{beat: ev.beat, value: ev.value + add})
	}
}

func (m *Module) heightSuppressedByRoll(beat float64) bool {
	for _, ev := range m.rolls {
		if ev.valid && beat >= ev.beat+1 && beat < ev.beat+2 {
			return true
		}
	}
	return false
}

func (m *Module) scheduleCountIns() {
	for i, ev := range m.countIns {
		if ev.mute || (len(m.countIns) > 1 && i != 0) {
			continue
		}
		if ev.kind == 8 {
			for _, off := range []float64{0, 2, 4, 5, 6, 7} {
				m.ctx.SoundAt(ev.beat+off, "common_count-ins_cowbell", 1)
			}
		} else {
			for _, off := range []float64{0, 1, 2, 3} {
				m.ctx.SoundAt(ev.beat+off, "common_count-ins_cowbell", 1)
			}
		}
	}
}

func (m *Module) scheduleRollCues() {
	for _, ev := range m.rolls {
		if !ev.valid || ev.mute {
			continue
		}
		for _, cue := range []struct {
			off float64
			n   int
		}{{-1, 1}, {-0.75, 2}, {-0.5, 3}, {-0.25, 4}, {0.25, 6}} {
			m.ctx.SoundAt(ev.beat+cue.off, highJumpSound(cue.n), 1)
		}
	}
}

func (m *Module) scheduleTextboxes() {
	for _, ev := range m.textboxes {
		ev := ev
		m.ctx.At(ev.beat, func() {
			m.ctx.Scene.SetActive(m.textbox, true)
			_ = m.ctx.Assets.SetText(m.textPath, ev.text)
			m.ctx.Scene.SetPosOver(m.textboxBody, ev.x, ev.y)
			m.ctx.Scene.SetSizeOver(m.textboxBody, ev.width*10, ev.height*3.5)
		})
		m.ctx.At(ev.beat+ev.length, func() { m.ctx.Scene.SetActive(m.textbox, false) })
	}
}

func (m *Module) resetRuntime(beat float64) {
	m.stopped, m.stopStars = false, false
	m.hitJumps = 0
	m.evolveAmount = 1
	m.lastPulse = int(math.Floor(beat)) - 1
	m.ctx.Scene.SetActive(m.textbox, false)
	m.ctx.Scene.SetActive(m.playerRoot, true)
	m.ctx.Scene.PlayDefaultState(m.playerRoot, beat, m.ctx.SecPerBeat(beat))
	m.player.reset(beat)
	m.stars.reset(beat)
	m.spawnPlatforms(beat)
	first := math.Ceil(beat)
	lastUnits := m.heightUnitsAt(first - 1)
	currentUnits := m.heightUnitsAt(first)
	m.raiseHeight(first-1, lastUnits, currentUnits)
}

func (m *Module) persistState(beat float64) {
	m.applyColors(defaultColorEvt(0))
	for _, ev := range m.colors {
		if ev.beat < beat {
			m.applyColors(ev)
		}
	}
	m.bg = bgEvt{startTop: black, endTop: black, startBottom: black, endBottom: black}
	for _, ev := range m.bgs {
		if ev.beat < beat {
			m.bg = ev
		}
	}
}

func (m *Module) updatePulse(beat float64) {
	for pulse := m.lastPulse + 1; pulse <= int(math.Floor(beat)); pulse++ {
		if pulse >= 0 && m.player.state == playerWalking {
			m.ctx.Scene.PlayState(m.playerRoot, "Walk", float64(pulse), 0.5)
		}
		m.lastPulse = pulse
	}
}

func (m *Module) updateHeight(beat float64) {
	if math.IsInf(m.raiseBeat, -1) {
		return
	}
	u := norm(beat, m.raiseBeat, 1)
	newHeight := engine.Ease(12, m.lastHeight, m.heightToRaise, u)
	m.holderY = newHeight
	m.stars.normalizedY = -engine.Ease(12, m.cfg.starHeight*float64(m.lastUnits), m.cfg.starHeight*float64(m.currentUnits), u)
}

func (m *Module) raiseHeight(beat float64, lastUnits, currentUnits int) {
	m.raiseBeat = beat
	m.lastUnits, m.currentUnits = lastUnits, currentUnits
	m.lastHeight = float64(lastUnits) * m.cfg.heightAmount
	m.heightToRaise = float64(currentUnits) * m.cfg.heightAmount
	m.holderY = m.lastHeight
}

func (m *Module) heightUnitsAt(beat float64) int {
	total := 0
	for _, ev := range m.heights {
		if ev.beat <= beat {
			total += ev.value
		}
	}
	return total
}

func (m *Module) noJumpAt(beat float64) bool {
	for _, ev := range m.noJumps {
		if beat >= ev.beat && beat < ev.beat+ev.length {
			return true
		}
	}
	return false
}

func (m *Module) fishAt(beat float64) bool {
	for _, b := range m.fishBeats {
		if nearBeat(b, beat) {
			return true
		}
	}
	return false
}

func (m *Module) rollAt(beat float64) bool {
	for _, ev := range m.rolls {
		if ev.valid && nearBeat(ev.beat, beat) {
			return true
		}
	}
	return false
}

func (m *Module) typeAt(beat float64) (int, int, bool) {
	for _, ev := range m.types {
		if nearBeat(ev.beat, beat) {
			return ev.platformType, ev.fillType, true
		}
	}
	return platformFlower, fillNone, false
}

func (m *Module) stopAll() {
	m.stopped = true
	m.stopStars = true
	for _, p := range m.platforms {
		p.stopped = true
	}
}

func (m *Module) destroyPlatforms(startBeat, firstBeat, lastBeat float64) {
	var list []*platformInst
	for _, p := range m.platforms {
		if p.endBeat >= firstBeat && p.endBeat <= lastBeat {
			list = append(list, p)
		}
	}
	sort.Slice(list, func(i, j int) bool { return list[i].endBeat < list[j].endBeat })
	for i, p := range list {
		p := p
		beat := startBeat + float64(i)
		m.ctx.At(beat, func() { p.disappear(beat) })
	}
}

func (m *Module) bgColorAt(beat float64, top bool) [4]float64 {
	ev := m.bg
	u := norm(beat, ev.beat, ev.length)
	a, b := ev.startBottom, ev.endBottom
	if top {
		a, b = ev.startTop, ev.endTop
	}
	return [4]float64{
		engine.Ease(ev.ease, a[0], b[0], u),
		engine.Ease(ev.ease, a[1], b[1], u),
		engine.Ease(ev.ease, a[2], b[2], u),
		1,
	}
}

func (m *Module) applyBG(ev bgEvt) {
	m.bg = ev
	m.applyBGAt(ev.beat + ev.length)
}

func (m *Module) applyBGAt(beat float64) {
	top := m.bgColorAt(beat, true)
	bottom := m.bgColorAt(beat, false)
	m.ctx.Scene.SetPaletteFor(m.bgMat, palette(top, top, bottom))
}

func (m *Module) applyColors(ev colorEvt) {
	m.ctx.Scene.SetPaletteFor(m.platformMat, palette([4]float64{1, 1, 1, 1}, ev.platform, ev.platformBeam))
	m.ctx.Scene.SetPaletteFor(m.platformLightMat, palette([4]float64{1, 1, 1, 1}, ev.platformLight, ev.platformLight))
	m.ctx.Scene.SetPaletteFor(m.fishMat, palette(ev.fish, ev.fishShock, ev.fishShade))
	m.ctx.Scene.SetPaletteFor(m.starMat, palette([4]float64{1, 1, 1, 1}, ev.starFace, ev.star))
}

func defaultColorEvt(beat float64) colorEvt {
	return colorEvt{
		beat: beat, platform: defaultPlatform, platformBeam: defaultPlatformBeam,
		platformLight: defaultPlatformLight, fish: defaultFish, fishShock: defaultFishShock,
		fishShade: defaultFishShade, star: defaultStar, starFace: defaultStarFace,
	}
}

func readPlatformCfg(handler, platform kmdata.Component) platformCfg {
	root := ref(handler.Refs, "platformRef")
	return platformCfg{
		root:             root,
		defaultYPos:      num(handler.Nums, "defaultYPos", -11.76),
		heightAmount:     num(handler.Nums, "heightAmount", 2),
		platformDistance: num(handler.Nums, "platformDistance", 3.8),
		playerXPos:       num(handler.Nums, "playerXPos", -6.78),
		starLength:       num(handler.Nums, "starLength", 18),
		starHeight:       num(handler.Nums, "starHeight", 0.0625),
		platformCount:    nonzeroInt(int(num(handler.Nums, "platformCount", 24)), 24),
		platform:         relPath(root, ref(platform.Refs, "platform")),
		fallYan:          relPath(root, ref(platform.Refs, "fallYan")),
		fallYanRoll:      relPath(root, ref(platform.Refs, "fallYanRoll")),
		fish:             relPath(root, ref(platform.Refs, "fish")),
		rollPlatform:     relPath(root, ref(platform.Refs, "rollPlatform")),
		rollLong:         relPath(root, ref(platform.Refs, "rollPlatformLong")),
		rollLong2:        relPath(root, ref(platform.Refs, "rollPlatformLong2")),
	}
}

func readStarCfg(c kmdata.Component) starCfg {
	return starCfg{
		root:           ref(c.Refs, "starRef"),
		boundaryX:      num(c.Nums, "boundaryX", 10),
		boundaryY:      num(c.Nums, "boundaryY", 6),
		starCount:      nonzeroInt(int(num(c.Nums, "starCount", 32)), 32),
		blinkFrequency: num(c.Nums, "blinkFrequency", 0.125),
		blinkAmount:    nonzeroInt(int(num(c.Nums, "blinkAmount", 3)), 3),
	}
}

func roleOr(ctx *engine.Ctx, key, fallback string) string {
	if p := ctx.Role(key); p != "" {
		return p
	}
	return fallback
}
