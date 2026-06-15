package sumobrothers

import (
	"math"
	"sort"

	"github.com/hajimehoshi/ebiten/v2"

	"hsdemo/engine"
	"hsdemo/kart"
	"hsdemo/riq"
)

func (m *Module) Load(ctx *engine.Ctx) error {
	m.ctx = ctx
	if err := ctx.LoadAssets(gameID); err != nil {
		return err
	}
	m.proj = kart.Translate(engine.ScreenW/2, engine.ScreenH/2).Mul(kart.Scale(54, -54))

	game := ctx.Assets.Extra.Components["game"]
	m.inu = roleOr(ctx, "inuSensei", refOr(game.Refs, "inuSensei", "inuSensei"))
	m.pBody = roleOr(ctx, "sumoBrotherP", refOr(game.Refs, "sumoBrotherP", "sumoBrotherP"))
	m.gBody = roleOr(ctx, "sumoBrotherG", refOr(game.Refs, "sumoBrotherG", "sumoBrotherG"))
	m.pHead = roleOr(ctx, "sumoBrotherPHead", refOr(game.Refs, "sumoBrotherPHead", "sumoBrotherP/head/headdy"))
	m.gHead = roleOr(ctx, "sumoBrotherGHead", refOr(game.Refs, "sumoBrotherGHead", "sumoBrotherG/head/headdy"))
	m.impact = roleOr(ctx, "impact", refOr(game.Refs, "impact", "misc/impact"))
	m.glasses = roleOr(ctx, "glasses", refOr(game.Refs, "glasses", "misc/glasses"))
	m.bgMove = roleOr(ctx, "bgMove", refOr(game.Refs, "bgMove", "backgroundChanges/bgMove"))
	m.bgStatic = roleOr(ctx, "bgStatic", refOr(game.Refs, "bgStatic", "backgroundChanges/bgStatic"))
	m.bgTopPath = roleOr(ctx, "bgTop", refOr(game.Refs, "bgTop", "background/backgroundExtend"))
	m.bgBottomPath = roleOr(ctx, "bgBtm", refOr(game.Refs, "bgBtm", "background/backgroundExtend2"))
	m.bgMat = refOr(game.Refs, "backgroundMaterial", "BGColor")
	m.mawashiMat = refOr(game.Refs, "mawashiMaterial", "Mawashis")
	if v := game.Nums["stompShakeSpeed"]; v > 0 {
		m.shakeSpeed = v
	}

	m.resetRuntime(0)
	return nil
}

func (m *Module) OnEvent(e *riq.Entity) {
	b := e.Beat
	switch eventName(e) {
	case "bop":
		ev := *e
		m.ctx.At(b, func() {
			if m.activeAt(b) {
				m.bop(b, ev.Length, boolDefault(&ev, "bopInu", true), boolDefault(&ev, "bopSumo", true),
					boolParam(&ev, "bopInuAuto"), boolParam(&ev, "bopSumoAuto"))
			}
		})
	case "crouch":
		ev := *e
		m.ctx.At(b, func() {
			if m.activeAt(b) {
				m.crouch(b, ev.Length, boolDefault(&ev, "inuT", true), boolDefault(&ev, "sumoT", true))
			}
		})
	case "stompSignal":
		ev := signalEvt{
			beat: b, length: e.Length, mute: boolParam(e, "mute"),
			look: boolDefault(e, "look", true), direction: intParam(e, "direction", stompAutomatic),
		}
		m.signals = append(m.signals, ev)
		if m.activeAt(b) {
			m.ctx.At(b, func() { m.stompSignal(ev.beat, ev.mute, !ev.mute, ev.look, ev.direction) })
		} else if !ev.mute {
			m.stompSignalSound(b)
		}
	case "slapSignal":
		ev := signalEvt{beat: b, length: e.Length, mute: boolParam(e, "mute"), slap: true}
		m.signals = append(m.signals, ev)
		if m.activeAt(b) {
			m.ctx.At(b, func() { m.slapSignal(ev.beat, ev.mute, !ev.mute) })
		} else if !ev.mute {
			m.slapSignalSound(b)
		}
	case "endPose":
		ev := *e
		m.ctx.At(b, func() {
			if m.activeAt(b) {
				m.endPose(b, boolDefault(&ev, "random", true), intParam(&ev, "type", poseSquat),
					intParam(&ev, "bg", bgGreatWave), boolDefault(&ev, "confetti", true),
					boolDefault(&ev, "alternate", true), boolDefault(&ev, "throw", true))
			}
		})
	case "background color":
		ev := bgColorEvt{
			beat: b, length: e.Length,
			top0: colorParam(e, "colorFrom", defaultBgTop),
			top1: colorParam(e, "colorTo", defaultBgTop),
			bot0: colorParam(e, "colorFrom2", defaultBgBottom),
			bot1: colorParam(e, "colorTo2", defaultBgBottom),
			ease: intParam(e, "ease", 0),
		}
		m.bgColors = append(m.bgColors, ev)
		m.ctx.At(b, func() { m.setBackgroundColor(ev) })
	case "mawashi color":
		ev := mawashiEvt{
			beat:  b,
			left:  colorParam(e, "colorLeft", defaultMawashiL),
			right: colorParam(e, "colorRight", defaultMawashiR),
		}
		m.mawashis = append(m.mawashis, ev)
		m.ctx.At(b, func() { m.setMawashiColor(ev.left, ev.right) })
	case "look":
		length := e.Length
		m.ctx.At(b, func() {
			if m.activeAt(b) {
				m.lookAtCamera(b, length)
			}
		})
	case "forceinput":
		ev := *e
		m.ctx.At(b-1, func() {
			if m.activeAt(b) {
				m.forceInputs(b, ev.Length, intParam(&ev, "type", forceSlap),
					intParam(&ev, "direction", stompAutomatic), boolParam(&ev, "center"),
					boolParam(&ev, "switch"), boolDefault(&ev, "prepare", true))
			}
		})
	}
}

func (m *Module) Ready() {
	sort.SliceStable(m.signals, func(i, j int) bool { return m.signals[i].beat < m.signals[j].beat })
	sort.SliceStable(m.bgColors, func(i, j int) bool { return m.bgColors[i].beat < m.bgColors[j].beat })
	sort.SliceStable(m.mawashis, func(i, j int) bool { return m.mawashis[i].beat < m.mawashis[j].beat })
}

func (m *Module) OnSwitch(beat float64) {
	m.nextSwitchBeat = m.ctx.NextSwitchBeat(beat)
	m.persistColors(beat)
	if m.sumoState == stateIdle && m.previousState == stateIdle {
		m.playIdle(beat)
	}
	for _, ev := range m.signals {
		if ev.beat > beat {
			break
		}
		if beat > ev.beat+ev.length {
			continue
		}
		if ev.slap {
			m.slapSignal(ev.beat, true, true)
		} else {
			m.stompSignal(ev.beat, true, true, ev.look, ev.direction)
		}
	}
}

func (m *Module) Whiff(beat float64) {
	if m.previousState == stateSlap || m.previousState == stateIdle {
		m.ctx.Sound("whiff")
		if m.lookingAtCamera {
			m.play(m.pHead, "SumoPSlapLook", beat)
		} else {
			m.play(m.pHead, "SumoPSlap", beat)
		}
		switch m.sumoSlapDir {
		case 2:
			m.play(m.pBody, "SumoSlapToStomp", beat)
		case 1:
			m.play(m.pBody, "SumoSlapFront", beat)
		default:
			m.play(m.pBody, "SumoSlapBack", beat)
		}
	}
	if m.previousState == stateStomp && !m.isPlaying(m.pBody, "SumoStompMiss") {
		m.ctx.Sound("miss")
		m.play(m.inu, "InuFloatMiss", beat)
		m.play(m.pBody, "SumoStompMiss", beat)
		m.play(m.pHead, "SumoPMiss", beat)
	}
}

func (m *Module) Update(_, beat float64) {
	pulse := int(math.Floor(beat))
	if pulse > m.lastPulse {
		m.lastPulse = pulse
		m.onLateBeatPulse(float64(pulse))
	}
	m.updateConfetti(beat)
}

func (m *Module) Draw(screen *ebiten.Image, _, beat float64) {
	top, bottom := m.currentBG(beat)
	screen.Fill(toRGBA(bottom))
	sc := m.ctx.Scene
	sc.SetPaletteFor(m.bgMat, kart.Palette{Alpha: top, Fill: defaultWhite, Outline: bottom})
	sc.SetPaletteFor(m.mawashiMat, m.mawashiPalette())
	sc.SetColorOver(m.bgTopPath, top)
	sc.SetColorOver(m.bgBottomPath, bottom)
	cam := m.ctx.CameraAt(beat)
	sc.SetCamera(cam[0]+m.cameraShakeAt(beat), cam[1], cam[2])
	sc.Sample(beat)
	sc.Draw(screen, m.proj)
	m.drawConfetti(screen, beat)
}

func (m *Module) resetRuntime(beat float64) {
	m.goBopInu, m.goBopSumo = true, true
	m.allowBopInu, m.allowBopSumo = true, true
	m.sumoStompDir = false
	m.sumoSlapDir = 0
	m.sumoPoseType = 0
	m.sumoPoseTypeNext = 0
	m.sumoPoseCurrent = "1"
	m.sumoPoseConfetti = false
	m.bgType, m.bgTypeNext = bgNone, bgNone
	m.sumoState, m.previousState = stateIdle, stateIdle
	m.lookingAtCamera = false
	m.cueActive = false
	m.shakeKeys = nil
	m.confetti = nil
	m.bgRun = colorRun{bgColorEvt: bgColorEvt{
		beat: beat, top0: defaultBgTop, top1: defaultBgTop,
		bot0: defaultBgBottom, bot1: defaultBgBottom,
	}, active: true}
	m.mawashiLeft, m.mawashiRight = defaultMawashiL, defaultMawashiR
	m.playIdle(beat)
	m.play(m.inu, "InuIdle", beat)
	m.play(m.impact, "impactGone", beat)
	m.play(m.glasses, "glassesGone", beat)
	m.play(m.bgMove, "empty", beat)
	m.play(m.bgStatic, "empty", beat)
}

func (m *Module) playIdle(beat float64) {
	m.play(m.pBody, "SumoIdle", beat)
	m.play(m.gBody, "SumoIdle", beat)
	m.play(m.pHead, "SumoPIdle", beat)
	m.play(m.gHead, "SumoGIdle", beat)
}
