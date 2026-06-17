// Package fanclub ports Fan Club's call/response runtime.
package fanclub

import (
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"

	"hsdemo/engine"
	"hsdemo/kart"
	"hsdemo/riq"
)

const (
	idolBopBoth = iota
	idolBopIdol
	idolBopSpectators
	idolBopNone
)

const (
	idolAnimBop = iota
	idolAnimPeaceVocal
	idolAnimPeace
	idolAnimClap
	idolAnimCall
	idolAnimResponse
	idolAnimJump
	idolAnimBigCall
	idolAnimSquat
	idolAnimWink
	idolAnimDab
)

const (
	responseThrough = iota
	responseJump
	responseThroughFast
	responseJumpFast
)

const (
	stageReset = iota
	stageFlash
	stageSpot
)

const (
	perfNormal = iota
	perfArrange
)

const (
	fanCount = 12
	radius   = 1.5
)

type interval struct {
	beat, length float64
}

func (i interval) contains(beat float64) bool {
	return beat >= i.beat && beat < i.beat+i.length
}

type bopEvt struct {
	beat, length float64
	target       int
	auto         int
}

type Module struct {
	ctx  *engine.Ctx
	proj kart.Aff

	stage, arisa, arisaRoot, arisaShadow string
	blueRoot, blue, blueShadow           string
	orangeRoot, orange, orangeShadow     string
	fanTemplate                          *kart.Template

	fans    []*fan
	blueD   dancer
	orangeD dancer
	effects fanClubEffects

	bops        []bopEvt
	endBeat     float64
	autoBop     int
	performance int

	noBop, noResponse, noCall, noSpecBop interval
	responseToggle                       bool
	idolJumpStart                        float64
	lastPulse                            float64
	noJudgement                          bool
	noJudgementInput                     bool
}

func New() engine.Module {
	return &Module{
		autoBop:       idolBopNone,
		performance:   perfNormal,
		idolJumpStart: math.Inf(-1),
		lastPulse:     math.Inf(-1),
	}
}

func (m *Module) ID() string { return "fanClub" }

func (m *Module) Load(ctx *engine.Ctx) error {
	m.ctx = ctx
	if err := ctx.LoadAssets("fanClub"); err != nil {
		return err
	}
	m.proj = kart.Translate(engine.ScreenW/2, engine.ScreenH/2).Mul(kart.Scale(54, -54))
	m.stage = roleOr(ctx, "StageAnimator", "Background")
	m.arisa = roleOr(ctx, "Arisa", "Idol_rootMotion/Idol")
	m.arisaRoot = roleOr(ctx, "ArisaRootMotion", "Idol_rootMotion")
	m.arisaShadow = roleOr(ctx, "ArisaShadow", "idol_Shadow")
	m.blue = roleOr(ctx, "Blue", "dancerR_rootMotion/Blue")
	m.orange = roleOr(ctx, "Orange", "dancerL_rootMotion/Orange")
	m.blueRoot, m.blueShadow = "dancerR_rootMotion", "dancerR_Shadow"
	m.orangeRoot, m.orangeShadow = "dancerL_rootMotion", "dancerL_Shadow"
	m.fanTemplate = kart.NewTemplate(ctx.Assets, roleOr(ctx, "spectator", "Fan"))
	ctx.Scene.SetActive(roleOr(ctx, "spectator", "Fan"), false)
	m.blueD = newDancer(ctx, m.blue, m.blueRoot, m.blueShadow)
	m.orangeD = newDancer(ctx, m.orange, m.orangeRoot, m.orangeShadow)
	m.spawnFans()
	m.setPerformance(perfNormal, 0)
	m.toSpot(true)
	return nil
}

func (m *Module) OnEvent(e *riq.Entity) {
	b := e.Beat
	if end := b + e.Length; end > m.endBeat {
		m.endBeat = end
	}
	switch e.Datamodel {
	case "fanClub/bop":
		m.bops = append(m.bops, bopEvt{
			beat: b, length: e.Length,
			target: int(e.Float("type", idolBopBoth)),
			auto:   int(e.Float("type2", idolBopNone)),
		})
	case "fanClub/yeah, yeah, yeah":
		noArisa, noCrowd := boolParam(e, "toggle"), boolParam(e, "toggle2")
		m.scheduleHai(b, noArisa, noCrowd)
	case "fanClub/I suppose":
		noArisa, noCrowd := boolParam(e, "toggle"), boolParam(e, "toggle2")
		rt, alt := int(e.Float("type", responseThrough)), boolParam(e, "alt")
		m.scheduleKamone(b, noArisa, noCrowd, rt, alt)
	case "fanClub/double clap":
		m.scheduleBigReady(b, boolParam(e, "toggle"))
	case "fanClub/play idol animation":
		typ, who := int(e.Float("type", idolAnimBop)), int(e.Float("who", 0))
		m.ctx.At(b, func() { m.playIdolAnimation(b, e.Length, typ, who) })
	case "fanClub/play stage animation":
		typ := int(e.Float("type", stageFlash))
		m.ctx.At(b, func() { m.playStage(typ, b) })
	case "fanClub/friend walk":
		exit, instant := boolParam(e, "exit"), boolParam(e, "instant")
		m.ctx.At(b, func() { m.dancerTravel(b, e.Length, exit, instant) })
	case "fanClub/set performance type":
		typ := int(e.Float("type", perfNormal))
		m.ctx.At(b, func() { m.setPerformance(typ, b) })
	case "fanClub/finish":
		m.ctx.At(b, func() { m.finalCheer(b) })
	case "fanClub/arisa faceposer":
		m.scheduleFaceposer(e)
	}
}

func (m *Module) Ready() {
	for _, ev := range m.bops {
		ev := ev
		m.ctx.At(ev.beat, func() { m.autoBop = ev.auto })
		if ev.target != idolBopNone {
			for b := ev.beat; b < ev.beat+ev.length-1e-6; b++ {
				bb := b
				m.ctx.At(bb, func() { m.bopSingle(ev.target, bb) })
			}
		}
		if ev.auto != idolBopNone {
			end := m.autoBopEnd(ev.beat)
			for b := ev.beat; b < end-1e-6; b++ {
				bb := b
				m.ctx.At(bb, func() { m.bopSingle(m.autoBop, bb) })
			}
		}
	}
}

func (m *Module) OnSwitch(beat float64) {
	sec := m.ctx.SecPerBeat(beat)
	m.ctx.Scene.PlayDefaultState(m.stage, beat, sec)
	m.ctx.Scene.PlayState(m.arisa, "NoPose"+m.performanceSuffix(), beat, sec)
	m.blueD.applyActive(m.ctx.Scene)
	m.orangeD.applyActive(m.ctx.Scene)
	for _, f := range m.fans {
		f.play(m, "NoPose", beat)
	}
	m.toSpot(true)
	m.noJudgement = false
	m.noJudgementInput = false
	m.lastPulse = math.Floor(beat)
}

func (m *Module) Whiff(beat float64) {
	if m.noJudgement {
		if p := m.player(); p != nil {
			p.clapStart(m, beat, true, false, 0)
		}
		return
	}
	if m.player() != nil {
		m.player().clapStart(m, beat, false, false, 0)
	}
}

func (m *Module) Update(_, beat float64) {
	if p := math.Floor(beat); p > m.lastPulse {
		m.lastPulse = p
		m.bopSingle(m.autoBop, p)
	}
	m.updateJump(beat)
	m.blueD.update(m, beat)
	m.orangeD.update(m, beat)
	for _, f := range m.fans {
		f.update(m, beat)
	}
}

func (m *Module) Draw(screen *ebiten.Image, _, beat float64) {
	screen.Fill(color.Black)
	m.ctx.SampleScene(beat)
	for _, f := range m.fans {
		f.queue(m.ctx.Scene, beat)
	}
	m.effects.queue(m, beat)
	m.ctx.Scene.Draw(screen, m.proj)
}

func (m *Module) player() *fan {
	if len(m.fans) > 3 {
		return m.fans[3]
	}
	return nil
}

func (m *Module) performanceSuffix() string {
	if m.performance == perfArrange {
		return "Arrange"
	}
	return ""
}

func (m *Module) setPerformance(typ int, beat float64) {
	m.performance = typ
	m.ctx.Scene.PlayState(m.arisa, "NoPose"+m.performanceSuffix(), beat, m.ctx.SecPerBeat(beat))
}

func (m *Module) disableSpecBop(beat, length float64) {
	if m.noSpecBop.contains(m.ctx.Beat()) {
		newLen := (beat - m.noSpecBop.beat) + length
		if newLen > m.noSpecBop.length {
			m.noSpecBop.length = newLen
		}
		return
	}
	m.noSpecBop = interval{beat: beat, length: length}
}

func (m *Module) autoBopEnd(start float64) float64 {
	end := math.Max(m.endBeat+4, start+4)
	if sw := m.ctx.NextSwitchBeat(start); !math.IsInf(sw, 1) && sw > start {
		end = sw
	}
	for _, ev := range m.bops {
		if ev.beat > start && ev.beat < end {
			return ev.beat
		}
	}
	return end
}
