package lovelab

import (
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"

	"hsdemo/engine"
	"hsdemo/kart"
)

func (m *Module) Load(ctx *engine.Ctx) error {
	m.ctx = ctx
	if err := ctx.LoadAssets(gameID); err != nil {
		return err
	}
	m.proj = kart.Translate(engine.ScreenW/2, engine.ScreenH/2).Mul(kart.Scale(54, -54))
	game := ctx.Assets.Extra.Components["game"]

	m.labGuy = roleOr(ctx, "labGuy", compRef(game, "labGuy", "Guy"))
	m.labGuyHead = roleOr(ctx, "labGuyHead", compRef(game, "labGuyHead", "Guy/Head/HeadHolder"))
	m.labGuyArm = roleOr(ctx, "labGuyArm", compRef(game, "labGuyArm", "Guy/ArmHolder/Arm"))
	m.labGirl = roleOr(ctx, "labGirl", compRef(game, "labGirl", "Girl"))
	m.labGirlHead = roleOr(ctx, "labGirlHead", compRef(game, "labGirlHead", "Girl/Head/HeadHolder"))
	m.labGirlArm = roleOr(ctx, "labGirlArm", compRef(game, "labGirlArm", "Girl/ArmHolder/Arm"))
	m.labAssistant = roleOr(ctx, "labAssistant", compRef(game, "labAssistant", "Assistant"))
	m.labAssistantHead = roleOr(ctx, "labAssistantHead", compRef(game, "labAssistantHead", "Assistant/Head/HeadHolder"))
	m.labAssistantArm = roleOr(ctx, "labAssistantArm", compRef(game, "labAssistantArm", "Assistant/ArmHolder/Arm"))
	m.flaskSprite = roleOr(ctx, "flaskSpriteRend", compRef(game, "flaskSpriteRend", "Guy/ArmHolder/Arm/Flask"))
	m.girlFlaskSprite = roleOr(ctx, "girlFlaskSpriteRend", compRef(game, "girlFlaskSpriteRend", "Girl/ArmHolder/Arm/Flask"))
	m.weirdFlaskSprite = roleOr(ctx, "weirdFlaskSpriteRend", compRef(game, "weirdFlaskSpriteRend", "Assistant/ArmHolder/Arm/Flask"))
	m.heartBox = roleOr(ctx, "heartBox", compRef(game, "heartBox", "HeartBox"))
	m.boxPerson = roleOr(ctx, "boxPerson", compRef(game, "boxPerson", "SunsetBg/BoxPerson"))
	m.boxPersonDay = roleOr(ctx, "boxPersonDay", compRef(game, "boxPersonDay", "DayBg/BoxPerson"))
	m.spotlight = roleOr(ctx, "spotlightShader", compRef(game, "spotlightShader", "Shaders"))
	m.spotConeRoot = roleOr(ctx, "spotlightShaderCone", compRef(game, "spotlightShaderCone", "Shaders (spot)"))
	m.spotCone = roleOr(ctx, "spotlightCone", compRef(game, "spotlightCone", "Shaders (spot)/spotlight"))
	m.clouds = roleOr(ctx, "clouds", compRef(game, "clouds", "SunsetBg/CloudHolder"))
	m.sunsetBG = roleOr(ctx, "sunsetBG", compRef(game, "sunsetBG", "SunsetBg"))
	m.dayBG = roleOr(ctx, "dayBG", compRef(game, "dayBG", "DayBg"))
	m.girlHeader = roleOr(ctx, "girlHeaderShader", compRef(game, "girlHeaderShader", "Girl/Head/HeadHolder/Shading"))
	m.boyHeader = roleOr(ctx, "boyHeaderShader", compRef(game, "boyHeaderShader", "Guy/Head/HeadHolder/Shading"))
	m.weirdHeader = roleOr(ctx, "weirdHeaderShader", compRef(game, "weirdHeaderShader", "Assistant/Head/HeadHolder/Shading"))
	m.endPoint = roleOr(ctx, "endPoint", compRef(game, "endPoint", "HeartBox/EndPoint"))

	m.boyLiquid = colorField(game.Nums, "boyLiquidColor", m.boyLiquid)
	m.girlLiquid = colorField(game.Nums, "girlLiquidColor", m.girlLiquid)
	m.weirdLiquid = colorField(game.Nums, "weirdLiquidColor", m.weirdLiquid)
	m.cloudSpeed = numField(game.Nums, "cloudSpeed", m.cloudSpeed)
	m.cloudDistance = numField(game.Nums, "cloudDistance", m.cloudDistance)
	m.flaskArcsBoy = append([]string(nil), ctx.Assets.Extra.Strings["flaskArcToBoy"]...)
	m.flaskArcsGirl = append([]string(nil), ctx.Assets.Extra.Strings["flaskArcToGirl"]...)
	m.paths = loadFlaskPaths(game.Lists["flaskBouncePath"])

	m.flaskTemplate = templateLast(ctx.Assets, "Flask")
	m.girlFlaskTemplate = templateLast(ctx.Assets, "GirlFlask")
	m.guyHeartTemplate = templateLast(ctx.Assets, "Hearts")
	m.girlHeartTemplate = templateLast(ctx.Assets, "GirlHearts")
	m.completeHeartTemplate = templateLast(ctx.Assets, "CompleteHearts")

	m.initializeScene(0)
	return nil
}

func (m *Module) OnSwitch(beat float64) {
	m.initializeScene(beat)
	for _, ev := range m.colors {
		if ev.beat <= beat+1e-6 {
			m.setObjectColors(ev.a, ev.b, ev.c)
		}
	}
	for _, ev := range m.times {
		if ev.beat <= beat+1e-6 {
			m.setTimeOfDay(ev.typ)
		}
	}
	for _, ev := range m.cloudEvts {
		if ev.beat <= beat+1e-6 {
			m.canCloudsMove = ev.on
		}
	}
	for _, ev := range m.spots {
		if ev.beat <= beat+1e-6 {
			m.setSpotlight(ev.active, ev.typ, ev.where)
		}
	}
	for _, ev := range m.bops {
		if ev.beat <= beat+1e-6 {
			m.canBop = ev.auto
		}
	}
	for _, iv := range m.intervals {
		if beat >= iv.beat-1 && beat < iv.beat+iv.length {
			m.preInterval(iv, beat)
			break
		}
	}
}

func (m *Module) initializeScene(beat float64) {
	for _, p := range []string{
		m.labGuy, m.labGuyArm, m.labGirl, m.labGirlArm,
		m.labAssistant, m.labAssistantArm, m.heartBox, m.boxPerson, m.boxPersonDay,
	} {
		m.playDefault(p, beat)
	}
	m.play(m.labGuyHead, "GuyFaceIdle", beat)
	m.play(m.labGirlHead, "GirlIdleFace", beat)
	m.play(m.labAssistantHead, "WeirdFaceIdle", beat)
	m.ctx.Scene.SetActive(m.flaskSprite, false)
	m.ctx.Scene.SetActive(m.girlFlaskSprite, false)
	m.ctx.Scene.SetActive(m.weirdFlaskSprite, false)
	m.flasks = nil
	m.guyHearts = nil
	m.girlHearts = nil
	m.completeHearts = nil
	m.currentHearts = nil
	m.hasMissed = false
	m.hasStartedInterval = false
	m.isHolding = false
	m.isHoldingFlask = false
	m.hasShakenUp = false
	m.releaseValid = true
	m.setObjectColors(m.boyLiquid, m.girlLiquid, m.weirdLiquid)
	m.setTimeOfDay(timeSunset)
	m.setSpotlight(false, spotNormal, spotBoy)
	m.lastPulse = int(math.Floor(beat))
}

func (m *Module) Whiff(beat float64) { m.WhiffAction(beat, 0) }

func (m *Module) WhiffAction(beat float64, action int) {
	if action == 0 {
		if !m.ctx.ExpectingPressNow() && !m.isHolding {
			m.play(m.labGirlArm, "WhiffGrab", beat)
			m.isHolding = true
		}
		return
	}
	if m.ctx.PressingNow() {
		if m.hasShakenUp {
			m.onWhiffDown(beat)
		} else {
			m.onWhiffUp(beat)
		}
	}
}

func (m *Module) Update(_, beat float64) {
	pulse := int(math.Floor(beat + 1e-6))
	if pulse > m.lastPulse {
		m.lastPulse = pulse
		if m.canBop && m.inBopRegion(float64(pulse)) {
			m.bopping(float64(pulse))
		}
	}
	if m.ctx.ReleasedNow() && !m.ctx.ExpectingReleaseNow() && m.isHolding {
		m.play(m.labGirlArm, "ArmIdle", beat)
		if m.isHoldingFlask {
			m.hasShakenUp = false
			m.spawnGirlMissFlask(beat)
			m.releaseValid = false
		}
		m.isHolding = false
		m.isHoldingFlask = false
	}
}

func (m *Module) Draw(screen *ebiten.Image, _, beat float64) {
	screen.Fill(color.RGBA{0xb5, 0xff, 0xff, 0xff})
	m.updateClouds()
	m.ctx.SampleScene(beat)
	m.queueFlasks(beat)
	m.queueHearts(beat)
	m.queueParticles(beat)
	m.ctx.Scene.Draw(screen, m.proj)
}
