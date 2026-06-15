// Package monkeywatch ports Monkey Watch's recursive clap spawning, pink
// monkey intervals, watch hand timing, zoom/background fades, balloon motion,
// and original cue sounds.
package monkeywatch

import (
	"math"
	"sort"
	"time"

	"github.com/hajimehoshi/ebiten/v2"

	"hsdemo/engine"
	"hsdemo/kart"
	"hsdemo/riq"
)

type Module struct {
	ctx  *engine.Ctx
	proj kart.Aff

	cameraMoveable, watchHand, clicker string
	playerMonkey, middleMonkey         string
	monkeyHandler                      string
	anchorHour, anchorMinute           string
	hotAirBalloon, balloonAnchor       string
	balloonTarget                      string
	hotAirShadow                       string
	yellowClap, pinkClap               string

	zoomInSprites, zoomOutSprites []string
	balloonSprites                []string
	holePaths                     []string
	holePos                       [][2]float64

	yellowT, pinkT *kart.Template

	claps    []clapEvt
	pinks    []pinkEvt
	custom   []customPinkEvt
	appear   []appearEvt
	zooms    []zoomEvt
	balloons []balloonEvt

	monkeyEvents []monkeyTimelineEvt
	monkeys      []*watchMonkey
	watchHoleIdx int
	maxMonkeys   int
	sequenceEnds []float64

	// Unity advances the delayed camera rotation when monkeys enter the watch,
	// but advances the clicker hand only after the player's judged clap/miss.
	// Keeping the two angles separate avoids double-counting successful claps.
	cameraWantAngle  float64
	cameraAngleDelay float64
	handAngle        float64
	delayRate        float64
	cameraMoving     bool
	lastPulse        int

	zoomStart   float64
	zoomIn      bool
	fullZoom    float64
	zoomInLen   float64
	zoomOutLen  float64
	zoomInEase  int
	zoomOutEase int

	bgFadeBeat, bgFadeLength float64
	bgFadeOut                bool
	bgRealTime               bool
	bgHour, bgMinute         int
}

func New() engine.Module {
	return &Module{delayRate: 0.5, zoomStart: -2, zoomIn: true, maxMonkeys: 30, lastPulse: -1}
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
	clock := as.Extra.Components["clockArrow"]
	bg := as.Extra.Components["background"]
	balloon := as.Extra.Components["balloon"]
	handler := as.Extra.Components["monkeyHandler"]

	m.cameraMoveable = game.Refs["cameraMoveable"]
	m.watchHand = game.Refs["monkeyClockArrow"]
	m.middleMonkey = game.Refs["middleMonkey"]
	m.monkeyHandler = game.Refs["monkeyHandler"]
	m.clicker = clock.Refs["anchorRotateTransform"]
	m.playerMonkey = clock.Refs["playerMonkeyAnim"]
	m.yellowClap = clock.Refs["yellowClap"]
	m.pinkClap = clock.Refs["pinkClap"]
	m.anchorHour = bg.Refs["anchorHour"]
	m.anchorMinute = bg.Refs["anchorMinute"]
	m.zoomInSprites = append([]string(nil), bg.RefArrays["srsIn"]...)
	m.zoomOutSprites = append([]string(nil), bg.RefArrays["srsOut"]...)
	m.hotAirBalloon = balloon.Refs["balloonTrans"]
	m.balloonAnchor = balloon.Refs["anchor"]
	m.balloonTarget = balloon.Refs["target"]
	m.hotAirShadow = balloon.Refs["shadow"]
	m.balloonSprites = append([]string(nil), balloon.RefArrays["srs"]...)
	if handler.Nums["maxMonkeys"] > 0 {
		m.maxMonkeys = int(handler.Nums["maxMonkeys"])
	}
	m.fullZoom = game.Nums["fullZoomOut"]
	m.zoomInLen = game.Nums["zoomInBeatLength"]
	m.zoomOutLen = game.Nums["zoomOutBeatLength"]
	m.zoomInEase = int(game.Nums["zoomInEase"])
	m.zoomOutEase = int(game.Nums["zoomOutEase"])
	if m.zoomInLen <= 0 {
		m.zoomInLen = 2
	}
	if m.zoomOutLen <= 0 {
		m.zoomOutLen = 2
	}
	m.yellowT = kart.NewTemplate(as, "YellowMonkey")
	m.pinkT = kart.NewTemplate(as, "PinkMonkey")
	ctx.Scene.SetActive("YellowMonkey", false)
	ctx.Scene.SetActive("PinkMonkey", false)
	for i := 1; i <= 60; i++ {
		path := m.monkeyHandler + "/WatchHole" + itoa2(i)
		m.holePaths = append(m.holePaths, path)
		if p, ok := nodePos(as, path); ok {
			m.holePos = append(m.holePos, p)
		} else {
			m.holePos = append(m.holePos, [2]float64{})
		}
	}
	ctx.Scene.PlayDefaultState(m.watchHand, 0, ctx.SecPerBeat(0))
	ctx.Scene.PlayDefaultState(m.playerMonkey, 0, ctx.SecPerBeat(0))
	ctx.Scene.PlayDefaultState(m.middleMonkey, 0, ctx.SecPerBeat(0))
	ctx.Scene.PlayDefaultState(m.hotAirBalloon, 0, ctx.SecPerBeat(0))
	return nil
}

func (m *Module) OnEvent(e *riq.Entity) {
	switch actionName(e) {
	case "appear":
		m.appear = append(m.appear, appearEvt{beat: e.Beat, length: e.Length, count: intDefault(e, "value", 4)})
	case "clap":
		m.claps = append(m.claps, clapEvt{beat: e.Beat, length: e.Length, auto: boolDefault(e, "auto", true), min: intDefault(e, "min", 0)})
	case "off", "offStretch":
		m.pinks = append(m.pinks, pinkEvt{beat: e.Beat, length: e.Length, muteOoki: boolParam(e, "muteC"), muteEek: boolParam(e, "muteE")})
	case "offInterval":
		m.pinks = append(m.pinks, pinkEvt{beat: e.Beat, length: e.Length, interval: true, muteOoki: boolParam(e, "muteC"), muteEek: boolParam(e, "muteE")})
	case "offCustom":
		m.custom = append(m.custom, customPinkEvt{beat: e.Beat})
	case "zoomOut":
		m.zooms = append(m.zooms, zoomEvt{beat: e.Beat, out: true, instant: boolParam(e, "instant"), timeMode: intDefault(e, "timeMode", 0), hour: intDefault(e, "hour", 3), minute: intDefault(e, "minute", 0)})
	case "zoomIn":
		m.zooms = append(m.zooms, zoomEvt{beat: e.Beat, out: false, instant: boolParam(e, "instant"), timeMode: intDefault(e, "timeMode", 0), hour: intDefault(e, "hour", 3), minute: intDefault(e, "minute", 0)})
	case "balloon":
		m.balloons = append(m.balloons, balloonEvt{
			beat: e.Beat, length: e.Length,
			x0: floatDefault(e, "xStart", 0), x1: floatDefault(e, "xEnd", 0),
			y0: floatDefault(e, "yStart", 0), y1: floatDefault(e, "yEnd", 0),
			a0: floatDefault(e, "angleStart", 0), a1: floatDefault(e, "angleEnd", 0),
			ease: intDefault(e, "ease", 0),
		})
	}
}

func (m *Module) Ready() {
	sort.SliceStable(m.claps, func(i, j int) bool { return m.claps[i].beat < m.claps[j].beat })
	sort.SliceStable(m.pinks, func(i, j int) bool { return m.pinks[i].beat < m.pinks[j].beat })
	sort.SliceStable(m.custom, func(i, j int) bool { return m.custom[i].beat < m.custom[j].beat })
	sort.SliceStable(m.appear, func(i, j int) bool { return m.appear[i].beat < m.appear[j].beat })
	sort.SliceStable(m.zooms, func(i, j int) bool { return m.zooms[i].beat < m.zooms[j].beat })
	sort.SliceStable(m.balloons, func(i, j int) bool { return m.balloons[i].beat < m.balloons[j].beat })
	m.buildMonkeyTimeline()
	for i := range m.claps {
		m.claps[i].min = m.startSecondForClap(i)
	}
	m.buildMonkeyTimeline()
	m.schedulePinkSounds()
	m.scheduleClapSequences()
	m.scheduleAppear()
	for _, z := range m.zooms {
		z := z
		m.ctx.At(z.beat, func() { m.applyZoom(z) })
	}
}

func (m *Module) OnSwitch(beat float64) {
	m.lastPulse = int(math.Floor(beat)) - 1
	m.cameraMoving = false
	m.monkeys = nil
	m.applyCameraSegment(beat)
	m.applyZoomAt(beat)
}

func (m *Module) Whiff(beat float64) {
	m.ctx.PlayCommon("miss")
	m.ctx.Scene.PlayState(m.middleMonkey, "MiddleMonkeyMiss", beat, 0.4)
	m.ctx.Scene.PlayState(m.playerMonkey, "PlayerClapBarely", beat, 0.4)
}

func (m *Module) Update(_ float64, beat float64) {
	for pulse := m.lastPulse + 1; pulse <= int(math.Floor(beat)); pulse++ {
		if pulse >= 0 {
			m.ctx.Scene.PlayState(m.middleMonkey, "MiddleMonkeyBop", float64(pulse), 0.4)
		}
		m.lastPulse = pulse
	}
	m.updateCamera(beat)
	m.compactMonkeys(beat)
}

func (m *Module) Draw(screen *ebiten.Image, _ float64, beat float64) {
	m.updateCamera(beat)
	m.updateBackground(beat)
	m.updateBalloon(beat)
	m.ctx.SampleScene(beat)
	base := m.cameraBase()
	for _, mk := range m.monkeys {
		mk.queue(m.ctx.Scene, beat, base, m.cameraZ(beat))
	}
	m.ctx.Scene.Draw(screen, m.proj)
}

func (m *Module) compactMonkeys(beat float64) {
	dst := m.monkeys[:0]
	for _, mk := range m.monkeys {
		if !mk.dead || beat <= mk.deadBeat+0.75 {
			dst = append(dst, mk)
		}
	}
	m.monkeys = dst
}

func (m *Module) applyCameraSegment(beat float64) {
	next := m.ctx.NextSwitchBeat(beat)
	first := -1
	for i, c := range m.claps {
		if c.beat >= beat && c.beat < next {
			first = i
			break
		}
	}
	startAngle := 0.0
	if first >= 0 {
		startAngle = float64(m.claps[first].min) * degreePerMonkey
		m.watchHoleIdx = m.claps[first].min % 60
	}
	m.cameraWantAngle = startAngle
	m.cameraAngleDelay = startAngle
	m.handAngle = startAngle
	m.moveArrowTo(m.handAngle)
	for _, ap := range m.appear {
		if ap.beat < beat && ap.beat+float64(ap.count)*ap.length > beat {
			m.scheduleAppearEvent(ap, beat)
		}
	}
}

func (m *Module) updateCamera(beat float64) {
	if m.cameraMoving && m.cameraAngleDelay < m.cameraWantAngle {
		m.cameraAngleDelay += degreePerMonkey * 0.08
		if m.cameraAngleDelay > m.cameraWantAngle {
			m.cameraAngleDelay = m.cameraWantAngle
		}
	}
	if m.cameraAngleDelay >= m.cameraWantAngle {
		m.cameraMoving = false
	}
	m.ctx.Scene.SetSpinOver(m.cameraMoveable, radians(m.cameraAngleDelay))
	m.moveArrowTo(m.handAngle)
	t := clamp01((beat - m.zoomStart) / m.zoomLen())
	var x, y, z float64
	if m.zoomIn {
		x = ease(0, 0, t, m.zoomInEase)
		y = ease(0, 0, t, m.zoomInEase)
		z = ease(m.fullZoom, 0, t, m.zoomInEase)
	} else {
		x = ease(0, 0, t, m.zoomOutEase)
		y = ease(0, 0, t, m.zoomOutEase)
		z = ease(0, m.fullZoom, t, m.zoomOutEase)
	}
	m.ctx.Scene.SetPosOver(m.cameraMoveable, x, -y)
	m.ctx.Scene.SetZOver(m.cameraMoveable, z)
}

func (m *Module) zoomLen() float64 {
	if m.zoomIn {
		return m.zoomInLen
	}
	return m.zoomOutLen
}

func (m *Module) cameraZ(beat float64) float64 {
	t := clamp01((beat - m.zoomStart) / m.zoomLen())
	if m.zoomIn {
		return ease(m.fullZoom, 0, t, m.zoomInEase)
	}
	return ease(0, m.fullZoom, t, m.zoomOutEase)
}

func (m *Module) cameraBase() kart.Aff {
	return kart.Rotate(radians(m.cameraAngleDelay))
}

func (m *Module) moveArrowTo(angle float64) {
	m.ctx.Scene.SetSpinOver(m.clicker, radians(-angle))
}

func (m *Module) applyZoom(z zoomEvt) {
	length := m.zoomOutLen
	if !z.out {
		length = m.zoomInLen
	}
	if z.instant {
		m.zoomStart = z.beat - length
	} else {
		m.zoomStart = z.beat
	}
	m.zoomIn = !z.out
	m.bgFadeBeat = z.beat
	m.bgFadeLength = 0.25
	if z.instant {
		m.bgFadeLength = 0
	}
	m.bgFadeOut = z.out
	m.bgRealTime = z.timeMode == 0
	m.bgHour, m.bgMinute = z.hour, z.minute
}

func (m *Module) applyZoomAt(beat float64) {
	for _, z := range m.zooms {
		if z.beat < beat && z.beat+2 > beat {
			m.applyZoom(z)
		}
	}
}

func (m *Module) updateBackground(beat float64) {
	t := 1.0
	if m.bgFadeLength > 0 {
		t = clamp01((beat - m.bgFadeBeat) / m.bgFadeLength)
	}
	inA, outA := t, 1-t
	if m.bgFadeOut {
		inA, outA = 1-t, t
	}
	for _, p := range m.zoomInSprites {
		m.ctx.Scene.SetColorOver(p, [4]float64{1, 1, 1, inA})
	}
	for _, p := range m.zoomOutSprites {
		m.ctx.Scene.SetColorOver(p, [4]float64{1, 1, 1, outA})
	}
	hour, min, sec := m.bgHour, m.bgMinute, 0
	if m.bgRealTime {
		now := time.Now()
		hour, min, sec = now.Hour(), now.Minute(), now.Second()
	}
	h := (float64(hour%12) + float64(min)/60 + float64(sec)/3600) / 12 * 360
	mi := (float64(min) + float64(sec)/60) / 60 * 360
	m.ctx.Scene.SetSpinOver(m.anchorHour, radians(-h))
	m.ctx.Scene.SetSpinOver(m.anchorMinute, radians(-mi))
}

func (m *Module) updateBalloon(beat float64) {
	if len(m.balloons) == 0 {
		for _, p := range m.balloonSprites {
			m.ctx.Scene.SetColorOver(p, [4]float64{1, 1, 1, 0})
		}
		m.ctx.Scene.SetColorOver(m.hotAirShadow, [4]float64{1, 1, 1, 0})
		return
	}
	first := m.balloons[0].beat
	last := m.balloons[len(m.balloons)-1].beat + m.balloons[len(m.balloons)-1].length - 0.25
	alpha := clamp01((beat - first) / 0.25)
	if beat >= first {
		alpha = 1 - clamp01((beat-last)/0.25)
	}
	for _, p := range m.balloonSprites {
		m.ctx.Scene.SetColorOver(p, [4]float64{1, 1, 1, alpha})
	}
	m.ctx.Scene.SetColorOver(m.hotAirShadow, [4]float64{1, 1, 1, 0.35 * alpha})
	var cur *balloonEvt
	for i := range m.balloons {
		if beat >= m.balloons[i].beat && beat <= m.balloons[i].beat+m.balloons[i].length {
			cur = &m.balloons[i]
			break
		}
	}
	if cur == nil {
		return
	}
	t := clamp01((beat - cur.beat) / cur.length)
	x := ease(cur.x0, cur.x1, t, cur.ease)
	y := ease(cur.y0, cur.y1, t, cur.ease)
	a := ease(cur.a0, cur.a1, t, cur.ease)
	target, _ := nodePos(m.ctx.Assets, m.balloonTarget)
	balloon := m.ctx.Assets.Extra.Components["balloon"]
	m.ctx.Scene.SetPosOver(m.hotAirBalloon, target[0]+balloon.Nums["xOffset"]+x, target[1]+balloon.Nums["yOffset"]+y)
	m.ctx.Scene.SetSpinOver(m.balloonAnchor, radians(a))
}

func itoa2(v int) string {
	if v < 10 {
		return itoa(v)
	}
	return string(rune('0'+v/10)) + string(rune('0'+v%10))
}
