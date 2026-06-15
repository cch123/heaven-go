// Package workingdough ports Working Dough (workingDough).
//
// Unity logic reference:
// Assets/Scripts/Games/WorkingDough/WorkingDough.cs
// Assets/Scripts/Games/WorkingDough/NPCDoughBall.cs
// Assets/Scripts/Games/WorkingDough/PlayerEnterDoughBall.cs
// Assets/Scripts/Games/WorkingDough/BGBall.cs
package workingdough

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

	paths map[string]bouncePath

	smallBallT, bigBallT        *kart.Template
	playerSmallT, playerBigT    *kart.Template
	bgSmallT, bgBigT            *kart.Template
	balls                       []*doughBall
	breakBursts                 []breakBurst
	ballEvents                  []ballEvent
	intervals                   []intervalEvent
	passTurns                   []float64
	bgColors                    []colorEvent
	flash                       colorEvent
	arrowLeftNPC, arrowRightNPC string
	arrowLeftPlayer             string
	arrowRightPlayer            string
	whiteArrow, redArrow        string
	npcImpact, playerImpact     string
	missImpact                  string
	spaceshipLights, shipObject string
	bgObjects                   []string
	npcOpen, playerOpen         bool
	bigMode, bigModePlayer      bool
	bgDisabled, shipOnly        bool
	spaceshipRisen              bool
	gandwHasEntered             bool
}

type ballEvent struct {
	beat     float64
	big      bool
	hasGandw bool
	flash    [4]float64
}

type intervalEvent struct {
	beat, length float64
	auto         bool
}

type colorEvent struct {
	beat, length float64
	from, to     [4]float64
	ease         int
	active       bool
}

func New() engine.Module { return &Module{gandwHasEntered: true, flash: colorEvent{to: transparent}} }

func (m *Module) ID() string { return "workingDough" }

func (m *Module) Load(ctx *engine.Ctx) error {
	m.ctx = ctx
	if err := ctx.LoadAssets("workingDough"); err != nil {
		return err
	}
	m.proj = kart.Translate(engine.ScreenW/2, engine.ScreenH/2).Mul(kart.Scale(54, -54))
	m.paths = loadBouncePaths(ctx.Assets)
	m.smallBallT = kart.NewTemplate(ctx.Assets, role(ctx, "smallBallNPC"))
	m.bigBallT = kart.NewTemplate(ctx.Assets, role(ctx, "bigBallNPC"))
	m.playerSmallT = kart.NewTemplate(ctx.Assets, role(ctx, "playerEnterSmallBall"))
	m.playerBigT = kart.NewTemplate(ctx.Assets, role(ctx, "playerEnterBigBall"))
	m.bgSmallT = kart.NewTemplate(ctx.Assets, role(ctx, "smallBGBall"))
	m.bgBigT = kart.NewTemplate(ctx.Assets, role(ctx, "bigBGBall"))

	game := ctx.Assets.Extra.Components["game"]
	m.whiteArrow = game.Sprites["whiteArrowSprite"]
	m.redArrow = game.Sprites["redArrowSprite"]
	m.arrowLeftNPC = role(ctx, "arrowSRLeftNPC")
	m.arrowRightNPC = role(ctx, "arrowSRRightNPC")
	m.arrowLeftPlayer = role(ctx, "arrowSRLeftPlayer")
	m.arrowRightPlayer = role(ctx, "arrowSRRightPlayer")
	m.npcImpact = role(ctx, "npcImpact")
	m.playerImpact = role(ctx, "playerImpact")
	m.missImpact = role(ctx, "missImpact")
	m.spaceshipLights = role(ctx, "spaceshipLights")
	m.shipObject = role(ctx, "shipObject")
	m.bgObjects = append([]string{}, game.RefArrays["bgObjects"]...)

	m.playDefaultScene(0)
	ctx.Scene.SetActive(m.npcImpact, false)
	ctx.Scene.SetActive(m.playerImpact, false)
	ctx.Scene.SetActive(m.missImpact, false)
	ctx.Scene.SetActive(m.spaceshipLights, false)
	m.setArrow(m.arrowLeftNPC, false)
	m.setArrow(m.arrowRightNPC, false)
	m.setArrow(m.arrowLeftPlayer, false)
	m.setArrow(m.arrowRightPlayer, false)
	return nil
}

func (m *Module) playDefaultScene(beat float64) {
	sec := m.ctx.SecPerBeat(math.Max(beat, 0))
	m.ctx.Scene.PlayState(role(m.ctx, "conveyerAnimator"), "ConveyerBelt", beat, sec)
	m.ctx.Scene.PlayState(role(m.ctx, "doughDudesHolderAnim"), "OnGround", beat, sec)
	for _, p := range []string{
		role(m.ctx, "ballTransporterLeftNPC"), role(m.ctx, "ballTransporterRightNPC"),
		role(m.ctx, "ballTransporterLeftPlayer"), role(m.ctx, "ballTransporterRightPlayer"),
		role(m.ctx, "NPCBallTransporters"), role(m.ctx, "PlayerBallTransporters"),
		role(m.ctx, "gandwAnim"), role(m.ctx, "spaceshipAnimator"),
	} {
		m.ctx.Scene.PlayDefaultState(p, beat, sec)
	}
}

func (m *Module) OnEvent(e *riq.Entity) {
	switch e.Datamodel {
	case "workingDough/small ball":
		m.ballEvents = append(m.ballEvents, ballEvent{beat: e.Beat, flash: white})
	case "workingDough/big ball":
		m.ballEvents = append(m.ballEvents, ballEvent{
			beat: e.Beat, big: true, hasGandw: boolParam(e, "hasGandw"),
			flash: colorParam(e, "flashColor", white),
		})
	case "workingDough/beat intervals":
		m.intervals = append(m.intervals, intervalEvent{
			beat: e.Beat, length: e.Length, auto: boolParamDefault(e, "auto", true),
		})
	case "workingDough/passTurn":
		m.passTurns = append(m.passTurns, e.Beat)
	case "workingDough/rise spaceship":
		m.riseShip(e.Beat, e.Length)
	case "workingDough/launch spaceship":
		m.launchShip(e.Beat, e.Length)
	case "workingDough/lift dough dudes":
		m.elevate(e.Beat, e.Length, boolParam(e, "toggle"))
	case "workingDough/instant lift":
		b := e.Beat
		up := boolParamDefault(e, "toggle", true)
		m.ctx.At(b, func() { m.instantElevation(b, up) })
	case "workingDough/mr game and watch enter or exit":
		m.gandwEnterOrExit(e.Beat, e.Length, boolParam(e, "toggle"))
	case "workingDough/instant game and watch":
		b := e.Beat
		exit := boolParam(e, "toggle")
		m.ctx.At(b, func() { m.instantGANDW(b, exit) })
	case "workingDough/disableBG":
		b := e.Beat
		ship := boolParam(e, "ship")
		m.ctx.At(b, func() { m.disableBG(ship) })
	case "workingDough/bgcolor":
		m.bgColors = append(m.bgColors, colorEvent{
			beat: e.Beat, length: e.Length,
			from: colorParam(e, "colorStart", black),
			to:   colorParam(e, "colorEnd", black),
			ease: int(e.Float("ease", 0)), active: true,
		})
	}
}

func (m *Module) Ready() {
	sort.SliceStable(m.ballEvents, func(i, j int) bool { return m.ballEvents[i].beat < m.ballEvents[j].beat })
	sort.SliceStable(m.intervals, func(i, j int) bool { return m.intervals[i].beat < m.intervals[j].beat })
	sort.Float64s(m.passTurns)
	sort.SliceStable(m.bgColors, func(i, j int) bool { return m.bgColors[i].beat < m.bgColors[j].beat })
	for _, iv := range m.intervals {
		m.setIntervalStart(iv.beat, m.gameSwitchBeatFor(iv), iv.length, iv.auto)
	}
	for _, beat := range m.passTurns {
		if iv, ok := m.lastIntervalBefore(beat); ok {
			m.passTurn(beat, iv.length, iv.beat)
		}
	}
}

func (m *Module) OnSwitch(beat float64) {
	m.balls = liveBalls(m.balls, beat)
}

func (m *Module) Whiff(beat float64) { m.WhiffAction(beat, 0) }

func (m *Module) WhiffAction(beat float64, action int) {
	switch action {
	case 0:
		m.playPlayerJump(beat, false)
	case actionBig:
		m.playPlayerJump(beat, true)
	}
}

func (m *Module) Update(_, beat float64) {
	m.applyColors(beat)
	m.ctx.SampleScene(beat)
	m.updateBalls(beat)
	m.updateBreakBursts(beat)
}

func (m *Module) Draw(screen *ebiten.Image, _ float64, beat float64) {
	for _, b := range m.balls {
		b.inst.Queue(m.ctx.Scene, beat, kart.Identity(), 0)
	}
	for _, p := range m.breakBursts {
		p.queue(m.ctx.Scene, beat)
	}
	m.ctx.Scene.Draw(screen, m.proj)
}

func (m *Module) setIntervalStart(beat, gameSwitchBeat, length float64, autoPassTurn bool) {
	relevant := m.ballsBetween(beat, beat+length)
	hasBig := false
	for _, ball := range relevant {
		if ball.beat >= gameSwitchBeat {
			m.spawnNPCBall(ball.beat-1, ball.big, ball.hasGandw)
			m.onSpawnBall(ball.beat, ball.big)
		}
		hasBig = hasBig || ball.big
	}
	if autoPassTurn {
		m.passTurn(beat+length, length, beat)
	}
	m.ctx.At(beat-1, func() {
		m.bigMode = hasBig
		if m.bigMode {
			m.playNormal(role(m.ctx, "NPCBallTransporters"), "NPCGoBigMode", beat-1)
		}
		if !m.npcOpen {
			m.openNPCTransporters(beat - 1)
			if m.gandwHasEntered && !m.bgDisabled {
				m.playNormal(role(m.ctx, "gandwAnim"), "GANDWLeverUp", beat-1)
			}
		}
	})
}

func (m *Module) passTurn(beat, length, startBeat float64) {
	m.ctx.At(beat-1, func() {
		m.openPlayerTransporters(beat - 1)
		relevant := m.ballsBetween(startBeat, startBeat+length)
		hasBig := false
		for _, ball := range relevant {
			rel := ball.beat - startBeat
			m.spawnPlayerBall(beat+rel-1, ball.big, ball.flash, ball.hasGandw)
			hasBig = hasBig || ball.big
		}
		m.bigModePlayer = hasBig
		if m.bigModePlayer {
			m.playNormal(role(m.ctx, "PlayerBallTransporters"), "PlayerGoBigMode", beat-1)
		}
	})
	m.ctx.At(beat, func() {
		if m.gandwHasEntered && !m.bgDisabled {
			m.playNormal(role(m.ctx, "gandwAnim"), "MrGameAndWatchLeverDown", beat)
		}
	})
	m.ctx.At(beat+1, func() {
		if iv, ok := m.lastIntervalBefore(beat + 1); !ok || beat+1 > iv.beat+iv.length {
			m.closeNPCTransporters(beat + 1)
		}
		if m.bigMode {
			m.playNormal(role(m.ctx, "NPCBallTransporters"), "NPCExitBigMode", beat+1)
			m.bigMode = false
		}
	})
	m.ctx.At(beat+length+1, func() {
		m.closePlayerTransporters(beat + length + 1)
		if m.bigModePlayer {
			m.playNormal(role(m.ctx, "PlayerBallTransporters"), "PlayerExitBigMode", beat+length+1)
			m.bigModePlayer = false
		}
	})
}

func (m *Module) spawnNPCBall(beat float64, big, hasGandw bool) {
	m.ctx.At(beat, func() {
		if b := newNPCBall(m, beat, big, hasGandw); b != nil {
			m.balls = append(m.balls, b)
		}
	})
	m.ctx.At(beat, func() { m.setArrow(m.arrowLeftNPC, true) })
	m.ctx.At(beat+0.1, func() { m.setArrow(m.arrowLeftNPC, false) })
	m.ctx.At(beat+1, func() {
		m.playNPCJump(beat+1, big)
		m.ctx.Scene.SetActive(m.npcImpact, true)
	})
	m.ctx.At(beat+1.1, func() { m.ctx.Scene.SetActive(m.npcImpact, false) })
	m.ctx.At(beat+1.9, func() { m.setArrow(m.arrowRightNPC, true) })
	m.ctx.At(beat+2, func() { m.setArrow(m.arrowRightNPC, false) })
}

func (m *Module) onSpawnBall(beat float64, big bool) {
	if big {
		m.ctx.SoundAt(beat, "hitBigOther", 1)
		m.ctx.SoundAt(beat, "bigOther", 1)
	} else {
		m.ctx.SoundAt(beat, "hitSmallOther", 1)
		m.ctx.SoundAt(beat, "smallOther", 1)
	}
}

func (m *Module) spawnPlayerBall(beat float64, big bool, flash [4]float64, hasGandw bool) {
	m.ctx.At(beat, func() {
		if b := newPlayerBall(m, beat, big, flash, hasGandw); b != nil {
			m.balls = append(m.balls, b)
		}
	})
	m.ctx.At(beat, func() { m.setArrow(m.arrowLeftPlayer, true) })
	m.ctx.At(beat+0.1, func() { m.setArrow(m.arrowLeftPlayer, false) })
}

func (m *Module) spawnBGBall(beat float64, big, hasGandw bool) {
	if b := newBGBall(m, beat, big, hasGandw); b != nil {
		m.balls = append(m.balls, b)
	}
	m.ctx.At(beat+9, func() {
		if !m.spaceshipRisen && !m.bgDisabled {
			m.playNormal(role(m.ctx, "spaceshipAnimator"), "AbsorbBall", beat+9)
		}
	})
}

func (m *Module) playPlayerHit(beat float64, big bool, flash [4]float64) {
	if big {
		m.ctx.Sound("bigPlayer")
		m.ctx.Sound("hitBigPlayer")
		m.animatePlayerJump(beat, true)
		m.flash = colorEvent{beat: beat, length: 0.5, from: flash, to: [4]float64{flash[0], flash[1], flash[2], 0}, active: true}
		return
	}
	m.ctx.Sound("smallPlayer")
	m.ctx.Sound("hitSmallPlayer")
	m.animatePlayerJump(beat, false)
}

func (m *Module) playPlayerJump(beat float64, big bool) {
	state, snd := "SmallDoughJump", "smallPlayer"
	if big {
		state, snd = "BigDoughJump", "bigPlayer"
	}
	m.ctx.Sound(snd)
	m.playScaled(role(m.ctx, "doughDudesPlayer"), state, beat, 0.5)
}

func (m *Module) animatePlayerJump(beat float64, big bool) {
	state := "SmallDoughJump"
	if big {
		state = "BigDoughJump"
	}
	m.playScaled(role(m.ctx, "doughDudesPlayer"), state, beat, 0.5)
}

func (m *Module) playNPCJump(beat float64, big bool) {
	state := "SmallDoughJump"
	if big {
		state = "BigDoughJump"
	}
	m.playScaled(role(m.ctx, "doughDudesNPC"), state, beat, 0.5)
}

func (m *Module) hitImpact(beat float64) {
	m.ctx.Scene.SetActive(m.playerImpact, true)
	m.ctx.At(beat+0.1, func() { m.ctx.Scene.SetActive(m.playerImpact, false) })
}

func (m *Module) spawnBreakBurst(beat float64, origin [2]float64) {
	m.breakBursts = append(m.breakBursts, newBreakBurst(beat, origin))
}

func (m *Module) openNPCTransporters(beat float64) {
	m.playNormal(role(m.ctx, "ballTransporterLeftNPC"), "BallTransporterLeftOpen", beat)
	m.playNormal(role(m.ctx, "ballTransporterRightNPC"), "BallTransporterRightOpen", beat)
	m.npcOpen = true
}

func (m *Module) closeNPCTransporters(beat float64) {
	m.playNormal(role(m.ctx, "ballTransporterLeftNPC"), "BallTransporterLeftClose", beat)
	m.playNormal(role(m.ctx, "ballTransporterRightNPC"), "BallTransporterRightClose", beat)
	m.npcOpen = false
}

func (m *Module) openPlayerTransporters(beat float64) {
	m.playNormal(role(m.ctx, "ballTransporterLeftPlayer"), "BallTransporterLeftOpen", beat)
	m.playNormal(role(m.ctx, "ballTransporterRightPlayer"), "BallTransporterRightOpen", beat)
	m.playerOpen = true
}

func (m *Module) closePlayerTransporters(beat float64) {
	m.playNormal(role(m.ctx, "ballTransporterLeftPlayer"), "BallTransporterLeftClose", beat)
	m.playNormal(role(m.ctx, "ballTransporterRightPlayer"), "BallTransporterRightClose", beat)
	m.playerOpen = false
}

func (m *Module) setArrow(path string, red bool) {
	sprite := m.whiteArrow
	if red {
		sprite = m.redArrow
	}
	m.ctx.Scene.SetSpriteOver(path, sprite)
}

func (m *Module) pathPos(name string, elapsed float64) [2]float64 {
	if p, ok := m.paths[name]; ok {
		return p.eval(elapsed)
	}
	return [2]float64{}
}

func (m *Module) ballsBetween(start, end float64) []ballEvent {
	var out []ballEvent
	for _, b := range m.ballEvents {
		if b.beat >= start && b.beat < end {
			out = append(out, b)
		}
	}
	return out
}

func (m *Module) lastIntervalBefore(beat float64) (intervalEvent, bool) {
	var last intervalEvent
	ok := false
	for _, iv := range m.intervals {
		if iv.beat > beat {
			break
		}
		last, ok = iv, true
	}
	return last, ok
}

func (m *Module) gameSwitchBeatFor(iv intervalEvent) float64 {
	gameSwitchBeat := iv.beat
	for _, e := range m.ctx.Entities() {
		if !isWorkingDoughSwitch(e) {
			continue
		}
		if e.Beat >= iv.beat && e.Beat < iv.beat+iv.length {
			gameSwitchBeat = e.Beat
			break
		}
	}
	return gameSwitchBeat
}

func (m *Module) playNormal(rootPath, state string, beat float64) {
	m.ctx.Scene.PlayState(rootPath, state, beat, m.ctx.SecPerBeat(math.Max(beat, 0)))
}

func (m *Module) playScaled(rootPath, state string, beat, length float64) {
	m.ctx.Scene.PlayState(rootPath, state, beat, stateScale(m.ctx.Assets, rootPath, state, length))
}

func (m *Module) applyColors(beat float64) {
	m.ctx.Scene.SetColorOver(role(m.ctx, "backgroundSR"), colorAt(m.bgColors, black, beat))
	flash := transparent
	if m.flash.active {
		flash = m.flash.colorAt(beat)
	}
	m.ctx.Scene.SetColorOver(role(m.ctx, "flashSR"), flash)
}

func colorAt(events []colorEvent, def [4]float64, beat float64) [4]float64 {
	cur := def
	for _, ev := range events {
		if ev.beat > beat {
			break
		}
		cur = ev.colorAt(beat)
	}
	return cur
}

func (ev colorEvent) colorAt(beat float64) [4]float64 {
	if !ev.active {
		return ev.to
	}
	if ev.length <= 0 || beat >= ev.beat+ev.length {
		return ev.to
	}
	return lerpColor(ev.from, ev.to, ev.ease, (beat-ev.beat)/ev.length)
}

func (m *Module) updateBalls(beat float64) {
	dst := m.balls[:0]
	for _, b := range m.balls {
		if b.update(beat) {
			dst = append(dst, b)
		}
	}
	m.balls = dst
}

func liveBalls(in []*doughBall, beat float64) []*doughBall {
	dst := in[:0]
	for _, b := range in {
		if b.update(beat) {
			dst = append(dst, b)
		}
	}
	return dst
}

func (m *Module) updateBreakBursts(beat float64) {
	dst := m.breakBursts[:0]
	for _, b := range m.breakBursts {
		if b.alive(beat) {
			dst = append(dst, b)
		}
	}
	m.breakBursts = dst
}

func (m *Module) riseShip(beat, length float64) {
	m.ctx.At(beat, func() {
		if m.bgDisabled {
			return
		}
		m.spaceshipRisen = true
		m.ensureSpaceshipLights(beat)
		m.playScaled(role(m.ctx, "spaceshipAnimator"), "RiseSpaceship", beat, length)
	})
}

func (m *Module) launchShip(beat, length float64) {
	m.ctx.At(beat, func() {
		if m.bgDisabled {
			return
		}
		m.spaceshipRisen = true
		m.ensureSpaceshipLights(beat)
		m.playNormal(role(m.ctx, "spaceshipAnimator"), "SpaceshipShake", beat)
	})
	m.ctx.At(beat+length, func() {
		if m.bgDisabled {
			return
		}
		m.playNormal(role(m.ctx, "spaceshipAnimator"), "SpaceshipLaunch", beat+length)
		m.ctx.Sound("LaunchRobot")
	})
}

func (m *Module) ensureSpaceshipLights(beat float64) {
	m.ctx.Scene.SetActive(m.spaceshipLights, true)
	m.playNormal(m.spaceshipLights, "SpaceshipLights", beat)
}

func (m *Module) elevate(beat, length float64, up bool) {
	state := "LiftDown"
	if up {
		state = "LiftUp"
	}
	m.ctx.At(beat, func() { m.playScaled(role(m.ctx, "doughDudesHolderAnim"), state, beat, length) })
}

func (m *Module) instantElevation(beat float64, up bool) {
	state := "OnGround"
	if up {
		state = "InAir"
	}
	m.playNormal(role(m.ctx, "doughDudesHolderAnim"), state, beat)
}

func (m *Module) gandwEnterOrExit(beat, length float64, shouldExit bool) {
	m.ctx.At(beat, func() {
		if m.bgDisabled {
			return
		}
		m.gandwHasEntered = false
		state := "GANDWEnter"
		if shouldExit {
			state = "GANDWLeave"
		}
		m.playScaled(role(m.ctx, "gandwAnim"), state, beat, length)
	})
	m.ctx.At(beat+length, func() {
		if !m.bgDisabled {
			m.gandwHasEntered = !shouldExit
		}
	})
}

func (m *Module) instantGANDW(beat float64, shouldExit bool) {
	if m.bgDisabled {
		return
	}
	state := "MrGameAndWatchLeverDown"
	if shouldExit {
		state = "GANDWLeft"
	}
	m.playNormal(role(m.ctx, "gandwAnim"), state, beat)
	m.gandwHasEntered = !shouldExit
}

func (m *Module) disableBG(ship bool) {
	m.shipOnly = ship
	m.bgDisabled = !m.bgDisabled
	for _, p := range m.bgObjects {
		m.ctx.Scene.SetActive(p, !m.bgDisabled || m.shipOnly)
	}
	m.ctx.Scene.SetActive(m.shipObject, !m.bgDisabled && !m.shipOnly)
}

func boolParamDefault(e *riq.Entity, key string, def bool) bool {
	if _, ok := e.Data[key]; !ok {
		return def
	}
	return boolParam(e, key)
}
