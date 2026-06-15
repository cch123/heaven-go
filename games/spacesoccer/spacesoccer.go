// Package spacesoccer ports Space Soccer's keep-up chain, high-kick/toe
// release cue, NPC kicker formation moves, recolors, scrolling dot background,
// and script-driven SuperCurveObject ball paths from Heaven Studio.
package spacesoccer

import (
	"math"
	"sort"

	"github.com/hajimehoshi/ebiten/v2"

	"hsdemo/engine"
	"hsdemo/kart"
	"hsdemo/riq"
)

type dispenseEvt struct {
	beat, length float64
	playSound    bool
	down         bool
	auto         bool
	interval     int
}

type highKickEvt struct {
	beat, length float64
}

type npcEvt struct {
	beat, length float64
	choice       int
	ease         int
	amount       int
	x, y, z      float64
	preset       int
}

type easeEvt struct {
	beat, length float64
	ease         int
	x, y, z      float64
}

type playerMoveEvt struct {
	beat, length float64
	ease         int
	preset       int
	sound        int
	x, y, z      float64
}

type bgEvt struct {
	beat, length       float64
	ease               int
	start, end         [4]float64
	startDots, endDots [4]float64
}

type kickColorEvt struct {
	beat                float64
	outfit, boots, skin [4]float64
}

type platColorEvt struct {
	beat                               float64
	top, side, outline, flame, midFire [4]float64
}

type scrollEvt struct {
	beat float64
	x, y float64
}

type stopEvt struct {
	beat float64
	stop bool
}

type Module struct {
	ctx  *engine.Ctx
	proj kart.Aff

	kickerT *kart.Template
	ballT   *kart.Template
	paths   map[string]ballPath

	kickers []*kicker
	balls   []*ball

	dispenses   []dispenseEvt
	highKicks   []highKickEvt
	npcEvents   []npcEvt
	easeEvents  []easeEvt
	playerMoves []playerMoveEvt
	bgEvents    []bgEvt
	kickColors  []kickColorEvt
	platColors  []platColorEvt
	scrolls     []scrollEvt
	stops       []stopEvt

	kickerPalette   kart.Palette
	platformPalette kart.Palette
	firePalette     kart.Palette

	nowBeat           float64
	lastBeat          float64
	lastTime          float64
	haveTime          bool
	scrollX, scrollY  float64
	xBaseSpeed        float64
	yBaseSpeed        float64
	xScrollMultiplier float64
	yScrollMultiplier float64
	currentStop       bool
	ballDispensed     bool
	lastDispensedBeat float64
	hitBeats          []float64
	floatTime         float64
}

func New() engine.Module {
	return &Module{
		paths:             map[string]ballPath{},
		kickerPalette:     palette(kickLavender, white, kickPurple),
		platformPalette:   palette(platOutline, platTop, platSide),
		firePalette:       palette(kickLavender, fireYellow, white),
		xBaseSpeed:        1,
		yBaseSpeed:        1,
		xScrollMultiplier: 0.1,
		yScrollMultiplier: 0.3,
		lastBeat:          math.Inf(-1),
	}
}

func (m *Module) ID() string { return "spaceSoccer" }

func (m *Module) Load(ctx *engine.Ctx) error {
	m.ctx = ctx
	if err := ctx.LoadAssets("spaceSoccer"); err != nil {
		return err
	}
	m.proj = kart.Translate(engine.ScreenW/2, engine.ScreenH/2).Mul(kart.Scale(54, -54))

	game := ctx.Assets.Extra.Components["game"]
	if v := game.Nums["xBaseSpeed"]; v != 0 {
		m.xBaseSpeed = v
	}
	if v := game.Nums["yBaseSpeed"]; v != 0 {
		m.yBaseSpeed = v
	}
	m.paths = loadBallPaths(game.Lists["ballPaths"])
	m.kickerT = kart.NewTemplate(ctx.Assets, roleOr(ctx, "kickerPrefab", "KickerPrefab"))
	m.ballT = kart.NewTemplate(ctx.Assets, roleOr(ctx, "ballRef", "BallHolder"))

	for _, path := range []string{"SpaceKickerHolder", "KickerPrefab", "BallHolder", "Canvas"} {
		ctx.Scene.SetActive(path, false)
	}
	m.ensureKickers(1)
	return nil
}

func (m *Module) OnEvent(e *riq.Entity) {
	switch e.Datamodel {
	case "spaceSoccer/ball dispense":
		iv := int(e.Float("interval", 2))
		if iv <= 0 {
			iv = 2
		}
		m.dispenses = append(m.dispenses, dispenseEvt{
			beat: e.Beat, length: e.Length,
			playSound: !boolParam(e, "toggle"),
			down:      boolParam(e, "down"),
			auto:      boolParamDefault(e, "auto", true),
			interval:  iv,
		})
	case "spaceSoccer/high kick-toe!":
		m.highKicks = append(m.highKicks, highKickEvt{beat: e.Beat, length: e.Length})
	case "spaceSoccer/npc kickers enter or exit":
		m.npcEvents = append(m.npcEvents, npcEvt{
			beat: e.Beat, length: e.Length,
			choice: int(e.Float("choice", animEnter)), ease: int(e.Float("ease", 0)),
			amount: int(e.Float("amount", 5)),
			x:      e.Float("x", 2), y: e.Float("y", -0.5), z: e.Float("z", 1.25),
			preset: int(e.Float("preset", presetFive)),
		})
	case "spaceSoccer/easePos":
		m.easeEvents = append(m.easeEvents, easeEvt{
			beat: e.Beat, length: e.Length, ease: int(e.Float("ease", 0)),
			x: e.Float("x", 2), y: e.Float("y", -0.5), z: e.Float("z", 1.25),
		})
	case "spaceSoccer/pMove":
		m.playerMoves = append(m.playerMoves, playerMoveEvt{
			beat: e.Beat, length: e.Length, ease: int(e.Float("ease", 0)),
			preset: int(e.Float("preset", playerPresetLaunchStart)),
			sound:  int(e.Float("sound", launchSoundNone)),
			x:      e.Float("x", 0), y: e.Float("y", 0), z: e.Float("z", 0),
		})
	case "spaceSoccer/changeBG":
		m.bgEvents = append(m.bgEvents, bgEvt{
			beat: e.Beat, length: e.Length, ease: int(e.Float("ease", 0)),
			start:     colorParam(e, "start", defaultBG),
			end:       colorParam(e, "end", defaultBG),
			startDots: colorParam(e, "startDots", defaultDots),
			endDots:   colorParam(e, "endDots", defaultDots),
		})
	case "spaceSoccer/changeKick":
		m.kickColors = append(m.kickColors, kickColorEvt{
			beat: e.Beat, outfit: colorParam(e, "outfit", kickLavender),
			boots: colorParam(e, "boots", kickPurple), skin: colorParam(e, "skin", white),
		})
	case "spaceSoccer/changePlat":
		m.platColors = append(m.platColors, platColorEvt{
			beat: e.Beat,
			top:  colorParam(e, "top", platTop), side: colorParam(e, "side", platSide),
			outline: colorParam(e, "outline", platOutline),
			flame:   colorParam(e, "flame", kickLavender), midFire: colorParam(e, "mid", fireYellow),
		})
	case "spaceSoccer/scroll":
		m.scrolls = append(m.scrolls, scrollEvt{beat: e.Beat, x: e.Float("x", 0.1), y: e.Float("y", 0.3)})
	case "spaceSoccer/stopBall":
		m.stops = append(m.stops, stopEvt{beat: e.Beat, stop: boolParamDefault(e, "toggle", true)})
	case "spaceSoccer/npc kickers instant enter or exit":
		choice := animEnter
		if boolParam(e, "toggle") {
			choice = animExit
		}
		m.npcEvents = append(m.npcEvents, npcEvt{
			beat: e.Beat, length: e.Length, choice: choice, ease: 1,
			amount: 5, x: 1.75, y: 0.25, z: 0.75, preset: presetCustom,
		})
	}
}

func (m *Module) Ready() {
	sort.SliceStable(m.dispenses, func(i, j int) bool { return m.dispenses[i].beat < m.dispenses[j].beat })
	sort.SliceStable(m.highKicks, func(i, j int) bool { return m.highKicks[i].beat < m.highKicks[j].beat })
	sort.SliceStable(m.npcEvents, func(i, j int) bool { return m.npcEvents[i].beat < m.npcEvents[j].beat })
	sort.SliceStable(m.easeEvents, func(i, j int) bool { return m.easeEvents[i].beat < m.easeEvents[j].beat })
	sort.SliceStable(m.playerMoves, func(i, j int) bool { return m.playerMoves[i].beat < m.playerMoves[j].beat })
	sort.SliceStable(m.bgEvents, func(i, j int) bool { return m.bgEvents[i].beat < m.bgEvents[j].beat })
	sort.SliceStable(m.kickColors, func(i, j int) bool { return m.kickColors[i].beat < m.kickColors[j].beat })
	sort.SliceStable(m.platColors, func(i, j int) bool { return m.platColors[i].beat < m.platColors[j].beat })
	sort.SliceStable(m.scrolls, func(i, j int) bool { return m.scrolls[i].beat < m.scrolls[j].beat })
	sort.SliceStable(m.stops, func(i, j int) bool { return m.stops[i].beat < m.stops[j].beat })

	for _, ev := range m.dispenses {
		ev := ev
		m.ctx.At(ev.beat, func() {
			if m.ctx.GameAt(ev.beat) == m.ID() {
				m.dispense(ev.beat, ev.playSound, false, ev.down, ev.auto, ev.interval)
			} else if ev.playSound {
				m.dispenseSound(ev.beat, ev.down)
			}
		})
	}
	for _, ev := range m.playerMoves {
		ev := ev
		m.ctx.At(ev.beat, func() { m.playLaunchSound(ev) })
	}
}

func (m *Module) OnSwitch(beat float64) {
	m.nowBeat = beat
	m.lastBeat = beat
	m.haveTime = false
	m.balls = liveBalls(m.balls)
	m.applyStaticState(beat)
	for _, ev := range m.dispenses {
		if ev.beat > beat {
			break
		}
		if ev.beat+ev.length <= beat {
			continue
		}
		isOnSwitch := beatEq(ev.beat, beat)
		m.dispense(ev.beat, isOnSwitch && ev.playSound, false, isOnSwitch && ev.down, ev.auto, ev.interval)
		break
	}
}

func (m *Module) Whiff(beat float64) {
	if len(m.kickers) == 0 {
		return
	}
	p := m.kickers[0]
	if p.ball == nil {
		p.kickCheck(false, true, 0, false)
		return
	}
	p.kick(false, p.ball.canKick, false)
}

func (m *Module) Update(t, beat float64) {
	m.nowBeat = beat
	dt := 0.0
	if m.haveTime {
		dt = math.Max(0, t-m.lastTime)
	}
	m.haveTime = true
	m.lastTime = t
	m.floatTime += dt

	sx, sy := m.scrollSpeedAt(beat)
	m.xScrollMultiplier, m.yScrollMultiplier = sx, sy
	m.scrollX -= m.xBaseSpeed * m.xScrollMultiplier * dt
	m.scrollY += m.yBaseSpeed * m.yScrollMultiplier * dt

	m.applyStaticState(beat)
	m.applyPulseMisses(beat)
	for _, k := range m.kickers {
		k.update(beat)
	}
	for _, b := range m.balls {
		b.update(beat)
	}
	m.balls = liveBalls(m.balls)
	m.ctx.SampleScene(beat)
}

func (m *Module) Draw(screen *ebiten.Image, _ float64, beat float64) {
	bg, dots := m.bgColorsAt(beat)
	screen.Fill(rgba(bg))
	m.queueBackground(dots)
	for _, k := range m.kickers {
		base := k.base
		if k.floatW != 0 || k.floatH != 0 {
			phase := m.floatTime + k.floatPhase
			base[0] += math.Cos(phase) * k.floatW
			base[1] += math.Sin(phase) * k.floatH
		}
		k.inst.Offset = base
		k.inst.SetGroupOrder(k.groupOrder)
		k.inst.Queue(m.ctx.Scene, beat, kart.Identity(), k.z)
	}
	for _, b := range m.balls {
		if b.dead || b.kicker == nil {
			continue
		}
		holder, ok := b.kicker.inst.NodeWorld(holderRel, kart.Identity())
		if !ok {
			holder = kart.Translate(b.kicker.inst.Offset[0], b.kicker.inst.Offset[1])
		}
		b.inst.Queue(m.ctx.Scene, beat, holder, b.kicker.z)
	}
	m.ctx.Scene.Draw(screen, m.proj)
}

func (m *Module) applyStaticState(beat float64) {
	m.applyPalettes(beat)
	m.applyFormation(beat)
	m.applyPlayerPosition(beat)
	stop := m.stopAt(beat)
	if stop != m.currentStop {
		m.currentStop = stop
		for _, k := range m.kickers {
			k.stop(stop)
		}
	}
}

func (m *Module) applyPalettes(beat float64) {
	kp := palette(kickLavender, white, kickPurple)
	for _, ev := range m.kickColors {
		if ev.beat > beat {
			break
		}
		kp = palette(ev.outfit, ev.skin, ev.boots)
	}
	pp := palette(platOutline, platTop, platSide)
	fp := palette(kickLavender, fireYellow, white)
	for _, ev := range m.platColors {
		if ev.beat > beat {
			break
		}
		pp = palette(ev.outline, ev.top, ev.side)
		fp = palette(ev.flame, ev.midFire, white)
	}
	m.kickerPalette, m.platformPalette, m.firePalette = kp, pp, fp
	for _, k := range m.kickers {
		k.applyPalette(kp, pp, fp)
	}
}

func (m *Module) applyFormation(beat float64) {
	amount, x, y, z, active := m.formationAt(beat)
	m.ensureKickers(amount)
	for i, k := range m.kickers {
		if i == 0 {
			continue
		}
		k.base = [2]float64{playerRootX - x*float64(i), -y * float64(i)}
		k.z = z * float64(i)
		k.groupOrder = -i
		if z < 0 {
			k.groupOrder = i
		}
		w := 0.85
		h := 0.5
		if z != 0 {
			w -= math.Pow(math.Abs(z)*10, -1)
			h -= math.Pow(math.Abs(z)*10, -1)
		}
		k.floatW, k.floatH = math.Max(0, w), math.Max(0, h)
		if active != nil {
			k.setEnterAnim(active.beat, active.length, active.anim, active.ease)
		}
	}
}

type activeEnter struct {
	beat, length float64
	anim         string
	ease         int
}

func (m *Module) formationAt(beat float64) (amount int, x, y, z float64, active *activeEnter) {
	amount, x, y, z = 1, 2, -0.5, 1.25
	cur := [3]float64{x, y, z}
	for ni, ei := 0, 0; ; {
		nextNPC := ni < len(m.npcEvents) && m.npcEvents[ni].beat <= beat
		nextEase := ei < len(m.easeEvents) && m.easeEvents[ei].beat <= beat
		if !nextNPC && !nextEase {
			break
		}
		if nextNPC && (!nextEase || m.npcEvents[ni].beat <= m.easeEvents[ei].beat) {
			ev := m.npcEvents[ni]
			ni++
			a, dx, dy, dz := npcPreset(ev)
			amount = a
			cur = [3]float64{dx, dy, dz}
			if beat <= ev.beat+ev.length {
				anim := "Enter"
				if ev.choice == animExit {
					anim = "Exit"
				}
				active = &activeEnter{beat: ev.beat, length: ev.length, anim: anim, ease: ev.ease}
			}
			continue
		}
		ev := m.easeEvents[ei]
		ei++
		start := cur
		end := [3]float64{ev.x, ev.y, ev.z}
		if ev.length > 0 && beat <= ev.beat+ev.length {
			u := (beat - ev.beat) / ev.length
			cur[0] = engine.Ease(ev.ease, start[0], end[0], u)
			cur[1] = engine.Ease(ev.ease, start[1], end[1], u)
			cur[2] = engine.Ease(ev.ease, start[2], end[2], u)
		} else {
			cur = end
		}
	}
	return amount, cur[0], cur[1], cur[2], active
}

func npcPreset(ev npcEvt) (amount int, x, y, z float64) {
	switch ev.preset {
	case presetDuo:
		return 2, 7, -6, 10
	case presetCustom:
		amount = ev.amount
		if amount < 1 {
			amount = 1
		}
		return amount, ev.x, ev.y, ev.z
	default:
		return 5, 2, -0.5, 1.25
	}
}

func (m *Module) applyPlayerPosition(beat float64) {
	if len(m.kickers) == 0 {
		return
	}
	pos := m.playerPosAt(beat)
	m.kickers[0].base = [2]float64{playerRootX - pos[0], pos[1]}
	m.kickers[0].z = pos[2]
	m.kickers[0].groupOrder = 0
}

func (m *Module) playerPosAt(beat float64) [3]float64 {
	cur := [3]float64{0, 0, 0}
	for _, ev := range m.playerMoves {
		if ev.beat > beat {
			break
		}
		_, x, y, z, ease := playerMoveParams(ev)
		target := [3]float64{-x, y, -z}
		start := cur
		if ev.length > 0 && beat <= ev.beat+ev.length {
			u := (beat - ev.beat) / ev.length
			cur[0] = engine.Ease(ease, start[0], target[0], u)
			cur[1] = engine.Ease(ease, start[1], target[1], u)
			cur[2] = engine.Ease(ease, start[2], target[2], u)
		} else {
			cur = target
		}
	}
	return cur
}

func playerMoveParams(ev playerMoveEvt) (sound int, x, y, z float64, ease int) {
	x, y, z, ease, sound = ev.x, ev.y, ev.z, ev.ease, ev.sound
	switch ev.preset {
	case playerPresetLaunchStart:
		x, y, z, sound = -6, 15, 0, launchSoundStart
	case playerPresetLaunchEnd:
		x, y, z, sound = -4, 15, 0, launchSoundEnd
	}
	return sound, x, y, z, ease
}

func (m *Module) playLaunchSound(ev playerMoveEvt) {
	sound, _, _, _, _ := playerMoveParams(ev)
	switch sound {
	case launchSoundStart:
		m.ctx.Sound("jet1")
	case launchSoundEnd:
		m.ctx.Sound("jet2")
	}
}

func (m *Module) ensureKickers(amount int) {
	if amount < 1 {
		amount = 1
	}
	for len(m.kickers) > amount {
		k := m.kickers[len(m.kickers)-1]
		if k.ball != nil {
			k.ball.dead = true
		}
		m.kickers = m.kickers[:len(m.kickers)-1]
	}
	for len(m.kickers) < amount {
		idx := len(m.kickers)
		k := newKicker(m, idx, idx == 0)
		k.floatPhase = 0
		m.kickers = append(m.kickers, k)
	}
	for i, k := range m.kickers {
		k.index = i
		k.player = i == 0
	}
}

func (m *Module) bgColorsAt(beat float64) (bg, dots [4]float64) {
	bg, dots = defaultBG, defaultDots
	for _, ev := range m.bgEvents {
		if ev.beat > beat {
			break
		}
		if ev.length > 0 && beat <= ev.beat+ev.length {
			u := engine.Ease(ev.ease, 0, 1, (beat-ev.beat)/ev.length)
			bg = lerpColor(ev.start, ev.end, u)
			dots = lerpColor(ev.startDots, ev.endDots, u)
		} else {
			bg, dots = ev.end, ev.endDots
		}
	}
	return bg, dots
}

func (m *Module) scrollSpeedAt(beat float64) (x, y float64) {
	x, y = 0.1, 0.3
	for _, ev := range m.scrolls {
		if ev.beat > beat {
			break
		}
		x, y = ev.x, ev.y
	}
	return x, y
}

func (m *Module) stopAt(beat float64) bool {
	stop := false
	for _, ev := range m.stops {
		if ev.beat > beat {
			break
		}
		stop = ev.stop
	}
	return stop
}

func (m *Module) dispense(beat float64, playSound, ignorePlayer, playDown, auto bool, interval int) {
	m.dispenseExec(beat, playSound, ignorePlayer, playDown)
	if !auto {
		return
	}
	m.dispenseRecursion(beat+2, interval, 0)
}

func (m *Module) dispenseRecursion(beat float64, interval int, depth int) {
	if depth > maxAutoDispense {
		return
	}
	dispenseBeat := beat + float64(interval)
	for _, stop := range m.stops {
		if dispenseBeat+2 >= stop.beat {
			return
		}
	}
	m.ctx.At(dispenseBeat, func() {
		if !m.overlapsHighKick(dispenseBeat + 2) {
			m.dispenseExec(dispenseBeat, true, false, false)
		}
		m.dispenseRecursion(dispenseBeat+2, interval, depth+1)
	})
}

func (m *Module) dispenseExec(beat float64, playSound, ignorePlayer, playDown bool) {
	if !m.ballDispensed {
		m.lastDispensedBeat = beat
	}
	m.ballDispensed = true
	for _, k := range m.kickers {
		if k.ball != nil || (ignorePlayer && k.player) {
			continue
		}
		b := newBall(m, k, beat)
		m.balls = append(m.balls, b)
		if k.player && playSound {
			m.dispenseSound(beat, playDown)
		}
		k.dispenseBall(beat)
		k.canKick = true
	}
}

func (m *Module) dispenseSound(beat float64, playDown bool) {
	if playDown {
		m.soundAt(beat, "down")
	}
	for _, s := range []struct {
		name string
		off  float64
	}{
		{"dispenseNoise", 0}, {"dispenseTumble1", 0},
		{"dispenseTumble2", 0.25}, {"dispenseTumble2B", 0.25},
		{"dispenseTumble3", 0.75}, {"dispenseTumble4", 1},
		{"dispenseTumble5", 1.25}, {"dispenseTumble6", 1.5},
		{"dispenseTumble6B", 1.75},
	} {
		m.soundAt(beat+s.off, s.name)
	}
}

func (m *Module) soundAt(beat float64, name string) {
	if beat <= m.ctx.Beat()+1e-6 {
		m.ctx.Sound(name)
		return
	}
	m.ctx.SoundAt(beat, name, 1)
}

func (m *Module) overlapsHighKick(beat float64) bool {
	for _, hk := range m.highKicks {
		if beat > hk.beat && beat < hk.beat+3 {
			return true
		}
	}
	return false
}

func (m *Module) at(beat float64, fn func()) {
	if beat <= m.nowBeat+1e-6 {
		fn()
		return
	}
	m.ctx.At(beat, fn)
}

func (m *Module) applyPulseMisses(beat float64) {
	if !m.ballDispensed || math.IsInf(m.lastBeat, -1) {
		m.lastBeat = beat
		return
	}
	start := math.Floor(m.lastBeat) + 1
	end := math.Floor(beat)
	for b := start; b <= end; b++ {
		m.onBeatPulse(b)
	}
	m.lastBeat = beat
}

func (m *Module) onBeatPulse(beat float64) {
	if !m.ballDispensed {
		return
	}
	offsetBeat := beat + math.Mod(m.lastDispensedBeat, 1)
	for _, st := range m.stops {
		if offsetBeat >= st.beat {
			return
		}
	}
	if offsetBeat < m.lastDispensedBeat+2 {
		return
	}
	if m.inHighKickRecovery(offsetBeat) {
		if m.highKickAt(offsetBeat-2) && len(m.kickers) > 0 && m.kickers[0].ball == nil {
			if !beatIn(m.hitBeats, offsetBeat-0.5) {
				m.ctx.ScoreMiss()
			}
		}
		return
	}
	if len(m.kickers) > 0 && m.kickers[0].ball == nil && !beatIn(m.hitBeats, offsetBeat) {
		m.ctx.ScoreMiss()
	}
}

func (m *Module) inHighKickRecovery(beat float64) bool {
	for _, hk := range m.highKicks {
		if beat >= hk.beat+1 && beat < hk.beat+3 {
			return true
		}
	}
	return false
}

func (m *Module) highKickAt(beat float64) bool {
	for _, hk := range m.highKicks {
		if beatEq(hk.beat, beat) {
			return true
		}
	}
	return false
}

func (m *Module) queueBackground(tint [4]float64) {
	// Unity scrolls a RawImage UV rect. Ebitengine has no RawImage equivalent in
	// the scene renderer, so the two extracted dot sprites are tiled manually.
	spacing := 3.6
	offX := math.Mod(m.scrollX*spacing, spacing)
	offY := math.Mod(m.scrollY*spacing, spacing)
	for ix := -4; ix <= 4; ix++ {
		for iy := -3; iy <= 3; iy++ {
			x := float64(ix)*spacing + offX
			y := float64(iy)*spacing + offY
			m.ctx.Scene.Queue(kart.ExtraSprite{
				Sprite: "background_0", World: kart.Translate(x, y),
				Order: bgSpriteOrder, Tint: tint,
			})
			m.ctx.Scene.Queue(kart.ExtraSprite{
				Sprite: "background_1", World: kart.Translate(x+spacing*0.5, y+spacing*0.5),
				Order: bgSpriteOrder + 1, Tint: tint,
			})
		}
	}
}

func liveBalls(in []*ball) []*ball {
	out := in[:0]
	for _, b := range in {
		if b != nil && !b.dead {
			out = append(out, b)
		}
	}
	return out
}

func roleOr(ctx *engine.Ctx, key, fallback string) string {
	if p := ctx.Role(key); p != "" {
		return p
	}
	return fallback
}
