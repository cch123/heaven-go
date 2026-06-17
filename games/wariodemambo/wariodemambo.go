package wariodemambo

import (
	"image/color"
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
	if err := ctx.Assets.ApplyTexts(); err != nil {
		return err
	}
	m.proj = kart.Translate(engine.ScreenW/2, engine.ScreenH/2).Mul(kart.Scale(54, -54))

	game := ctx.Assets.Extra.Components["game"]
	m.commandText = roleOr(ctx, "commandText", game.Refs["commandText"])
	m.endPose = roleOr(ctx, "endPose", game.Refs["endPose"])
	m.spotL = roleOr(ctx, "spotlightLTrans", game.Refs["spotlightLTrans"])
	m.spotR = roleOr(ctx, "spotlightRTrans", game.Refs["spotlightRTrans"])
	m.spotLTarget = roleOr(ctx, "DancerLSpotPos", game.Refs["DancerLSpotPos"])
	m.spotRTarget = roleOr(ctx, "DancerRSpotPos", game.Refs["DancerRSpotPos"])
	m.spotWario = roleOr(ctx, "WarioSpotPos", game.Refs["WarioSpotPos"])
	m.textAnim = roleOr(ctx, "textAnimator", game.Refs["textAnimator"])
	m.dancerL = roleOr(ctx, "dancerLeftAnim", game.Refs["dancerLeftAnim"])
	m.dancerLArm = roleOr(ctx, "dancerLeftArmAnim", game.Refs["dancerLeftArmAnim"])
	m.dancerLHead = roleOr(ctx, "dancerLeftHeadAnim", game.Refs["dancerLeftHeadAnim"])
	m.dancerLJump = roleOr(ctx, "dancerLeftJumpAnim", game.Refs["dancerLeftJumpAnim"])
	m.dancerR = roleOr(ctx, "dancerRightAnim", game.Refs["dancerRightAnim"])
	m.dancerRArm = roleOr(ctx, "dancerRightArmAnim", game.Refs["dancerRightArmAnim"])
	m.dancerRHead = roleOr(ctx, "dancerRightHeadAnim", game.Refs["dancerRightHeadAnim"])
	m.dancerRJump = roleOr(ctx, "dancerRightJumpAnim", game.Refs["dancerRightJumpAnim"])
	m.warioBody = roleOr(ctx, "warioBodyAnim", game.Refs["warioBodyAnim"])
	m.warioArm = roleOr(ctx, "warioArmAnim", game.Refs["warioArmAnim"])
	m.warioFace = roleOr(ctx, "warioFaceAnim", game.Refs["warioFaceAnim"])
	m.warioJump = roleOr(ctx, "warioJumpAnim", game.Refs["warioJumpAnim"])
	m.topLight = roleOr(ctx, "topLightAnim", game.Refs["topLightAnim"])
	m.leftLight = roleOr(ctx, "leftLightAnim", game.Refs["leftLightAnim"])
	m.rightLight = roleOr(ctx, "rightLightAnim", game.Refs["rightLightAnim"])
	m.mainMat = game.Refs["mainMat"]
	m.lightMat = game.Refs["lightMat"]
	m.floorLightMat = game.Refs["floorLightMat"]
	m.blueAdd = colorField(game.Nums, "blueAddColor", m.blueAdd)
	m.redAdd = colorField(game.Nums, "redAddColor", m.redAdd)

	m.onPlay(0)
	return nil
}

func (m *Module) OnEvent(e *riq.Entity) {
	b := e.Beat
	switch eventName(e) {
	case "bop":
		m.bops = append(m.bops, bopEvt{
			beat: b, length: e.Length,
			auto:    boolDefault(e, "auto", true),
			mambo:   boolDefault(e, "mambo", true),
			dancers: boolDefault(e, "dancers", true),
			lights:  boolDefault(e, "lights", true),
		})
	case "beat intervals":
		m.intervals = append(m.intervals, intervalEvt{
			idx: len(m.intervals), beat: b, length: e.Length,
			autoPass:   boolDefault(e, "auto", true),
			memorize:   boolDefault(e, "memorize", true),
			numbers:    boolDefault(e, "numbers", true),
			text:       boolDefault(e, "text", true),
			left:       boolParam(e, "left"),
			resetColor: boolDefault(e, "resetColor", true),
		})
	case "turn":
		m.inputs = append(m.inputs, inputEvt{beat: b})
	case "jump":
		m.inputs = append(m.inputs, inputEvt{beat: b, jump: true})
	case "pass turn":
		m.passes = append(m.passes, passEvt{
			beat: b, length: e.Length,
			numbers: boolDefault(e, "numbers", true),
			text:    boolDefault(e, "text", true),
		})
	case "showText":
		m.texts = append(m.texts, textEvt{beat: b, length: e.Length, text: e.Str("text", "Turn it up!")})
	case "reaction":
		m.reactions = append(m.reactions, reactionEvt{beat: b, length: e.Length, resetColor: boolDefault(e, "resetColor", true)})
	case "lightsStage":
		m.lights = append(m.lights, lightEvt{beat: b, stage: intParam(e, "stage", lightsStage1)})
	case "introVoice":
		m.ctx.SoundAt(b+0.5, "ladiesandgentlemen", 1)
		m.ctx.SoundAt(b+8.5, "wariodemambo", 1)
	case "dance":
		m.dances = append(m.dances, danceEvt{beat: b, length: e.Length, typ: intParam(e, "dance", danceStationary)})
	case "changeColors":
		m.colors = append(m.colors, colorEvt{
			beat: b,
			red:  boolDefault(e, "red", true),
			dim:  boolDefault(e, "dim", true),
		})
	case "defaultText":
		// Hidden WIP action in Heaven Studio with no function delegate. Keep it
		// explicitly inert so official action coverage catches future changes.
	}
}

func (m *Module) Ready() {
	sort.SliceStable(m.bops, func(i, j int) bool { return m.bops[i].beat < m.bops[j].beat })
	sort.SliceStable(m.intervals, func(i, j int) bool { return m.intervals[i].beat < m.intervals[j].beat })
	sort.SliceStable(m.inputs, func(i, j int) bool { return m.inputs[i].beat < m.inputs[j].beat })
	sort.SliceStable(m.passes, func(i, j int) bool { return m.passes[i].beat < m.passes[j].beat })
	sort.SliceStable(m.texts, func(i, j int) bool { return m.texts[i].beat < m.texts[j].beat })
	sort.SliceStable(m.reactions, func(i, j int) bool { return m.reactions[i].beat < m.reactions[j].beat })
	sort.SliceStable(m.lights, func(i, j int) bool { return m.lights[i].beat < m.lights[j].beat })
	sort.SliceStable(m.dances, func(i, j int) bool { return m.dances[i].beat < m.dances[j].beat })
	sort.SliceStable(m.colors, func(i, j int) bool { return m.colors[i].beat < m.colors[j].beat })
	m.scheduleEvents()
}

func (m *Module) OnSwitch(beat float64) {
	m.onPlay(beat)
	for _, iv := range m.intervals {
		if iv.beat-4 < beat && iv.beat+iv.length >= beat {
			iv.memorize = false
			m.preInterval(iv, beat)
			break
		}
	}
}

func (m *Module) Whiff(beat float64) { m.WhiffAction(beat, 0) }

func (m *Module) WhiffAction(beat float64, action int) {
	if m.expectingAnyWarioInputNow() {
		return
	}
	switch action {
	case actionJump:
		m.whiffJump(beat)
	case actionRight:
		m.whiffRight(beat)
	case actionLeft, 0:
		m.whiffLeft(beat)
	}
}

func (m *Module) Update(_, beat float64) {
	pulse := int(math.Floor(beat + 1e-6))
	if pulse > m.lastPulse {
		m.lastPulse = pulse
		if m.autoBop && m.inBopRegion(float64(pulse)) {
			m.bop(float64(pulse), true, true, true)
		}
	}
	if len(m.pendingPasses) > 0 && m.activeAt(beat) {
		pending := m.pendingPasses
		m.pendingPasses = nil
		for _, ev := range pending {
			m.passTurnStandalone(ev)
		}
	}
}

func (m *Module) Draw(screen *ebiten.Image, _, beat float64) {
	screen.Fill(color.RGBA{0x00, 0x5b, 0x64, 0xff})
	m.updateDance(beat)
	m.updateSpotlights(beat)
	m.applyColors()
	m.ctx.SampleScene(beat)
	m.ctx.Scene.Draw(screen, m.proj)
}

func (m *Module) onPlay(beat float64) {
	for _, p := range []string{
		m.textAnim, m.dancerL, m.dancerLArm, m.dancerLHead, m.dancerLJump,
		m.dancerR, m.dancerRArm, m.dancerRHead, m.dancerRJump,
		m.warioBody, m.warioArm, m.warioFace, m.warioJump,
		m.topLight, m.leftLight, m.rightLight,
	} {
		m.playDefault(p, beat)
	}
	m.crEvents = nil
	m.pendingPasses = nil
	m.expectedInputs = nil
	m.bopState = bopNormal
	m.dancerBopState = bopNormal
	m.canBop = true
	m.autoBop = true
	m.isDancing = false
	m.armControlsEnabled = true
	m.dancerArmCentered = true
	m.armCentered = true
	m.warioLeft = false
	m.dancerLeft = false
	m.hasFlicked = false
	m.misses = 0
	m.lightsStage = lightsStage1
	m.ctx.Scene.SetActive(m.endPose, false)
	m.currentText = ""
	_ = m.ctx.Assets.SetText(m.commandText, "")
	m.spotLPos = m.point(m.spotWario)
	m.spotRPos = m.point(m.spotWario)
	m.spotLEase = spotEase{}
	m.spotREase = spotEase{}
	m.spotsPos = spotsWario
	m.setColors(false, true)
	m.ctx.Scene.SetPosOver(m.spotL, m.spotLPos[0], m.spotLPos[1])
	m.ctx.Scene.SetPosOver(m.spotR, m.spotRPos[0], m.spotRPos[1])
	m.lastPulse = int(math.Floor(beat))
}
