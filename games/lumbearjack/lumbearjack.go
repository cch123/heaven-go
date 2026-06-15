// Package lumbearjack ports LumBEARjack's event surface, bear/cat animation
// timing, three object cut chains, snow, and the original sound cues.
package lumbearjack

import (
	"image/color"
	"math"
	"sort"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"hsdemo/engine"
	"hsdemo/kart"
	"hsdemo/riq"
)

type Module struct {
	ctx  *engine.Ctx
	proj kart.Aff

	bear, catRight, catLeft      string
	smallRoot, bigRoot, hugeRoot string
	smallT, bigT, hugeT          *kart.Template
	babyT, bombT                 *kart.Template

	catRightSmall, catRightBig, catRightHuge []string
	catLeftSmall, catLeftBig, catLeftHuge    []string
	bgCats                                   []string

	gameComp     map[string]string
	catMoveSpecs map[string]catMoveSpec
	catMoves     map[string]catMoveRuntime
	bgCatDance   map[string]catDanceRuntime

	bops    []bopEvt
	objects []objectEvt
	cats    []catPresenceEvt
	snows   []snowEvt
	rests   []restEvt

	activeObjects []*cutObject
	babies        []*babyEffect
	bombs         []*bombEffect
	missFx        []*missEffect

	bearNoBop []float64
	catPuts   []objectEvt
	babyIndex int
	rested    bool
	restSound restSoundChoice
	lastPulse int

	snowOn       bool
	snowWind     float64
	snowStrength float64
	snowflakes   []snowflake
}

type catDanceRuntime struct {
	danceBeat, stopBeat float64
}

func New() engine.Module { return &Module{lastPulse: -1} }

func (m *Module) ID() string { return gameID }

func (m *Module) Load(ctx *engine.Ctx) error {
	m.ctx = ctx
	if err := ctx.LoadAssets(gameID); err != nil {
		return err
	}
	m.proj = kart.Translate(engine.ScreenW/2, engine.ScreenH/2).Mul(kart.Scale(54, -54))
	as := ctx.Assets
	game := as.Extra.Components["game"]
	m.gameComp = game.Refs
	m.bear = game.Refs["_bear"]
	m.catRight = game.Refs["_catRight"]
	m.catLeft = game.Refs["_catLeft"]
	m.smallRoot = game.Refs["_smallObjectPrefab"]
	m.bigRoot = game.Refs["_bigObjectPrefab"]
	m.hugeRoot = game.Refs["_hugeObjectPrefab"]
	m.catRightSmall = append([]string(nil), game.RefArrays["_catRightObjectsSmall"]...)
	m.catRightBig = append([]string(nil), game.RefArrays["_catRightObjectsBig"]...)
	m.catRightHuge = append([]string(nil), game.RefArrays["_catRightObjectsHuge"]...)
	m.catLeftSmall = append([]string(nil), game.RefArrays["_catLeftObjectsSmall"]...)
	m.catLeftBig = append([]string(nil), game.RefArrays["_catLeftObjectsBig"]...)
	m.catLeftHuge = append([]string(nil), game.RefArrays["_catLeftObjectsHuge"]...)
	m.bgCats = append([]string(nil), game.RefArrays["_bgCats"]...)

	m.smallT = kart.NewTemplate(as, m.smallRoot)
	m.bigT = kart.NewTemplate(as, m.bigRoot)
	m.hugeT = kart.NewTemplate(as, m.hugeRoot)
	m.babyT = kart.NewTemplate(as, game.Refs["_baby"])
	m.bombT = kart.NewTemplate(as, game.Refs["_bombRef"])
	m.bgCatDance = map[string]catDanceRuntime{}

	for _, root := range []string{m.smallRoot, m.bigRoot, m.hugeRoot, game.Refs["_baby"], game.Refs["_bombRef"], game.Refs["_missObjectRef"]} {
		ctx.Scene.SetActive(root, false)
	}
	for _, p := range []string{
		game.Refs["_smallLogCutParticle"], game.Refs["_canCutParticle"], game.Refs["_batCutParticle"],
		game.Refs["_broomCutParticle"], game.Refs["_barrelCutParticle"], game.Refs["_bookCutParticle"],
		game.Refs["_bigLogHitParticle"], game.Refs["_bigLogCutParticle"], game.Refs["_bigBallCutParticle"],
		game.Refs["_hugeLogHitParticle"], game.Refs["_hugeLogCutParticle"], game.Refs["_freezerChipParticle"],
		game.Refs["_freezerBreakParticle"], game.Refs["_peachHitParticle"], game.Refs["_peachCutParticle"],
	} {
		ctx.Scene.SetActive(p, false)
	}
	ctx.Scene.SetActive(game.Refs["_snowParticle"], false)
	m.disableCatObjects(true)
	m.disableCatObjects(false)
	m.loadCatMoves()
	for i := 0; i < 80; i++ {
		x := -8 + 16*signedSeed(float64(i), 1)
		y := -4 + 10*signedSeed(float64(i), 2)
		m.snowflakes = append(m.snowflakes, snowflake{x: x, y: y, speed: 1.2 + 2*signedSeed(float64(i), 3), drift: -0.5 + signedSeed(float64(i), 4)})
	}
	ctx.Scene.PlayDefaultState(m.bear, 0, sec(ctx, 0))
	for _, c := range append([]string{m.catRight, m.catLeft}, m.bgCatAnimators()...) {
		ctx.Scene.PlayDefaultState(c, 0, sec(ctx, 0))
	}
	return nil
}

func (m *Module) loadCatMoves() {
	m.catMoveSpecs = map[string]catMoveSpec{}
	m.catMoves = map[string]catMoveRuntime{}
	for _, c := range m.ctx.Assets.Extra.Components {
		if c.Refs["_otherPoint"] == "" {
			continue
		}
		this, ok := nodePos(m.ctx.Assets, c.Path)
		if !ok {
			continue
		}
		other := this
		if c.Nums["_usePoint"] != 0 {
			if p, ok := nodePos(m.ctx.Assets, c.Refs["_otherPoint"]); ok {
				other = p
			}
		} else {
			other[0] += c.Nums["_slideOffset"] / 2
		}
		spec := catMoveSpec{path: c.Path, this: this, other: other}
		m.catMoveSpecs[c.Path] = spec
		startIn := c.Nums["_startAtOther"] == 0
		m.catMoves[c.Path] = catMoveRuntime{spec: spec, inToScene: startIn}
		if !startIn {
			m.ctx.Scene.SetPosOver(c.Path, other[0], other[1])
		}
	}
}

func (m *Module) bgCatAnimators() []string {
	out := make([]string, 0, len(m.bgCats))
	for _, p := range m.bgCats {
		out = append(out, p+"/CatHolder")
	}
	return out
}

func (m *Module) OnEvent(e *riq.Entity) {
	switch actionName(e) {
	case "bop":
		m.bops = append(m.bops, bopEvt{beat: e.Beat, length: e.Length, bop: whoBops(intDefault(e, "bop", int(whoBoth))), auto: whoBops(intDefault(e, "auto", int(whoNone)))})
	case "small", "smallS":
		ev := objectEvt{beat: e.Beat, length: e.Length, kind: objSmall, small: smallType(intDefault(e, "type", 0)), huh: huhChoice(intDefault(e, "huh", 0)), cat: catPutChoice(intDefault(e, "cat", 0)), bomb: boolDefault(e, "bomb", true), sound: boolDefault(e, "sound", true)}
		m.objects = append(m.objects, ev)
	case "big", "bigS":
		ev := objectEvt{beat: e.Beat, length: e.Length, kind: objBig, big: bigType(intDefault(e, "type", 0)), cat: catPutChoice(intDefault(e, "cat", 0)), sound: boolDefault(e, "sound", true)}
		m.objects = append(m.objects, ev)
	case "huge", "hugeS":
		ev := objectEvt{beat: e.Beat, length: e.Length, kind: objHuge, huge: hugeType(intDefault(e, "type", 0)), cat: catPutChoice(intDefault(e, "cat", 0)), zoom: boolDefault(e, "zoom", true), baby: boolDefault(e, "baby", true), pBaby: boolDefault(e, "pBaby", true), sound: boolDefault(e, "sound", true)}
		m.objects = append(m.objects, ev)
	case "cats":
		m.cats = append(m.cats, catPresenceEvt{beat: e.Beat, length: e.Length, main: mainCatChoice(intDefault(e, "main", 0)), bg: intDefault(e, "bg", 0), instant: boolParam(e, "instant"), dance: boolDefault(e, "dance", true)})
	case "sigh":
		m.rests = append(m.rests, restEvt{beat: e.Beat, instant: boolParam(e, "instant"), sound: restSoundChoice(intDefault(e, "sound", 0))})
	case "snow":
		m.snows = append(m.snows, snowEvt{beat: e.Beat, length: e.Length, on: boolDefault(e, "on", true), instant: boolParam(e, "instant"), wind: floatDefault(e, "wS", 1), particles: floatDefault(e, "pS", 30)})
	}
}

func (m *Module) Ready() {
	sort.SliceStable(m.objects, func(i, j int) bool { return m.objects[i].beat < m.objects[j].beat })
	sort.SliceStable(m.bops, func(i, j int) bool { return m.bops[i].beat < m.bops[j].beat })
	sort.SliceStable(m.cats, func(i, j int) bool { return m.cats[i].beat < m.cats[j].beat })
	sort.SliceStable(m.snows, func(i, j int) bool { return m.snows[i].beat < m.snows[j].beat })
	m.catPuts = append([]objectEvt(nil), m.objects...)
	for _, ev := range m.objects {
		m.addBearNoBops(ev)
		if ev.sound {
			m.scheduleObjectSounds(ev, -1)
		}
		ev := ev
		m.ctx.At(ev.beat+m.unit(ev), func() {
			if m.ctx.GameAt(ev.beat) == gameID {
				m.spawnObject(ev, -1)
			}
		})
		m.scheduleCatPut(ev)
	}
	for _, bp := range m.bops {
		m.scheduleBop(bp.beat, bp.length, bp.bop, -1)
	}
	for _, cp := range m.cats {
		cp := cp
		m.ctx.At(cp.beat, func() { m.applyCatPresence(cp, false) })
	}
	for _, sn := range m.snows {
		sn := sn
		m.ctx.At(sn.beat, func() { m.setSnow(sn.on, sn.wind, sn.particles) })
	}
	for _, r := range m.rests {
		r := r
		m.ctx.At(r.beat, func() { m.restBear(r.beat, r.instant, r.sound) })
	}
}

func (m *Module) OnSwitch(beat float64) {
	m.lastPulse = int(math.Floor(beat)) - 1
	m.applyPresenceAt(beat)
	m.applySnowAt(beat)
	m.applyRestAt(beat)
	m.spawnPersistedObjects(beat)
	m.persistBabies(beat)
	if !m.rested {
		m.ctx.Scene.PlayDefaultState(m.bear, beat, sec(m.ctx, math.Max(beat, 0)))
	}
}

func (m *Module) Whiff(beat float64) {
	m.swingWhiff(beat, true)
}

func (m *Module) Update(_ float64, beat float64) {
	for pulse := m.lastPulse + 1; pulse <= int(math.Floor(beat)); pulse++ {
		if pulse >= 0 {
			m.autoBop(float64(pulse))
		}
		m.lastPulse = pulse
	}
	for _, o := range m.activeObjects {
		o.update(beat)
	}
	m.compactObjects(beat)
	m.compactEffects(beat)
}

func (m *Module) Draw(screen *ebiten.Image, _ float64, beat float64) {
	screen.Fill(color.RGBA{0xf4, 0x93, 0xa7, 0xff})
	m.updateCatMoves(beat)
	m.updateBgCatDance(beat)
	m.ctx.SampleScene(beat)
	for _, o := range m.activeObjects {
		o.queue(m.ctx.Scene, beat)
	}
	for _, b := range m.babies {
		b.queue(m.ctx.Scene, beat)
	}
	for _, b := range m.bombs {
		b.queue(m.ctx.Scene, beat)
	}
	m.ctx.Scene.Draw(screen, m.proj)
	for _, fx := range m.missFx {
		fx.draw(screen, m.proj, beat)
	}
	m.drawSnow(screen, beat)
}

func (m *Module) compactObjects(beat float64) {
	dst := m.activeObjects[:0]
	for _, o := range m.activeObjects {
		if !o.dead && beat <= o.beat+o.length+2 {
			dst = append(dst, o)
		}
	}
	m.activeObjects = dst
}

func (m *Module) compactEffects(beat float64) {
	babies := m.babies[:0]
	for _, b := range m.babies {
		if beat <= b.start+8 {
			babies = append(babies, b)
		}
	}
	m.babies = babies
	bombs := m.bombs[:0]
	for _, b := range m.bombs {
		if beat <= b.start+5 {
			bombs = append(bombs, b)
		}
	}
	m.bombs = bombs
	misses := m.missFx[:0]
	for _, fx := range m.missFx {
		if beat <= fx.start+fx.duration {
			misses = append(misses, fx)
		}
	}
	m.missFx = misses
}

func (m *Module) unit(ev objectEvt) float64 {
	switch ev.kind {
	case objSmall:
		return ev.length / 3
	case objBig:
		return ev.length / 4
	default:
		return ev.length / 6
	}
}

func (m *Module) addBearNoBops(ev objectEvt) {
	u := m.unit(ev)
	switch ev.kind {
	case objSmall:
		m.bearNoBop = append(m.bearNoBop, ev.beat+2*u)
		if (ev.small != smallLog || ev.huh == huhOn) && ev.huh != huhOff {
			m.bearNoBop = append(m.bearNoBop, ev.beat+ev.length, ev.beat+ev.length+1)
		}
	case objBig:
		m.bearNoBop = append(m.bearNoBop, ev.beat+2*u, ev.beat+3*u)
	case objHuge:
		m.bearNoBop = append(m.bearNoBop, ev.beat+2*u, ev.beat+3*u, ev.beat+4*u, ev.beat+5*u)
	}
}

func (m *Module) scheduleObjectSounds(ev objectEvt, startUpBeat float64) {
	if ev.beat >= startUpBeat {
		m.ctx.SoundAt(ev.beat, "readyVoice", 1)
	}
	u := m.unit(ev)
	putBeat := ev.beat + u
	if putBeat < startUpBeat {
		return
	}
	switch ev.kind {
	case objSmall:
		switch ev.small {
		case smallCan:
			m.ctx.SoundAt(putBeat, "canPut", 1)
		case smallBroom:
			m.ctx.SoundAt(putBeat, "broomPut", 1)
		case smallBook:
			m.ctx.SoundAt(putBeat, "bookPut", 1)
		default:
			m.ctx.SoundAt(putBeat, "smallLogPut", 1)
		}
	case objBig:
		if ev.big == bigBall {
			m.ctx.SoundAt(putBeat, "bigBallPut", 1)
		} else {
			m.ctx.SoundAt(putBeat, "bigLogPut", 1)
		}
	case objHuge:
		switch ev.huge {
		case hugeFreezer:
			m.ctx.SoundAt(putBeat, "freezerPut", 1)
		case hugePeach:
			m.ctx.SoundAt(putBeat, "peachPut", 1)
		default:
			m.ctx.SoundAt(putBeat, "hugeLogPut", 1)
		}
	}
}

func (m *Module) scheduleBop(beat, length float64, who whoBops, startUpBeat float64) {
	if who == whoNone {
		return
	}
	for i := 0.0; i < length; i++ {
		b := beat + i
		if b < startUpBeat {
			continue
		}
		m.ctx.At(b, func() { m.bop(b, who) })
	}
}

func (m *Module) autoBop(beat float64) {
	who := whoNone
	for _, bp := range m.bops {
		if beat >= bp.beat && beat < bp.beat+bp.length {
			who = bp.auto
		}
	}
	m.bop(beat, who)
}

func (m *Module) bop(beat float64, who whoBops) {
	if who == whoNone {
		return
	}
	noBear := false
	for _, b := range m.bearNoBop {
		if nearBeat(b, beat) {
			noBear = true
			break
		}
	}
	if (who == whoBoth || who == whoBear) && !noBear && !m.rested {
		if st, _ := m.ctx.Scene.StateInfo(m.bear, beat); st != "BeastWhiff" && st != "BeastRest" {
			m.ctx.Scene.PlayState(m.bear, "BeastBop", beat, 0.75)
		}
	}
	if who == whoBoth || who == whoCats {
		if st, _ := m.ctx.Scene.StateInfo(m.catRight, beat); st != "CatGrab" {
			m.ctx.Scene.PlayState(m.catRight, "CatBop", beat, 0.75)
		}
		if st, _ := m.ctx.Scene.StateInfo(m.catLeft, beat); st != "CatGrab" {
			m.ctx.Scene.PlayState(m.catLeft, "CatBop", beat, 0.75)
		}
	}
}

func (m *Module) restBear(beat float64, instant bool, sound restSoundChoice) {
	start := 0.0
	if instant {
		start = 1
	}
	m.ctx.Scene.PlayState(m.bear, "BeastRest", beat-start, 0.5)
	m.rested = true
	m.restSound = sound
	if !instant {
		m.ctx.At(beat+1, func() { m.playRestSound() })
	}
}

func (m *Module) playRestSound() {
	switch m.restSound {
	case restA:
		m.ctx.Sound("sighA")
	case restB:
		m.ctx.Sound("sighB")
	case restRandom:
		if signedSeed(m.ctx.Beat(), 12) < 0.5 {
			m.ctx.Sound("sighA")
		} else {
			m.ctx.Sound("sighB")
		}
	}
}

func (m *Module) applyRestAt(beat float64) {
	m.rested = false
	for _, r := range m.rests {
		if r.beat < beat {
			m.rested = true
			m.restSound = r.sound
		}
	}
	if m.rested {
		m.ctx.Scene.PlayState(m.bear, "BeastRest", beat-1, 0.5)
	}
}

func (m *Module) swingWhiff(beat float64, sound bool) {
	if m.rested {
		return
	}
	if sound {
		pitch := math.Pow(2, (-200+400*signedSeed(beat, 7))/1200)
		m.ctx.SoundPitch("swing", 1, pitch)
	}
	m.ctx.Scene.PlayState(m.bear, "BeastWhiff", beat, 0.75)
}

func (m *Module) bearCut(beat float64, huh bool, huhLeft bool, zoom bool) {
	if huh {
		m.ctx.Scene.PlayState(m.bear, "BeastHalfCut", beat, 0.75)
		if huhLeft {
			m.ctx.At(beat+1, func() { m.ctx.Scene.PlayState(m.bear, "BeastHuhL", beat+1, 0.5) })
		} else {
			m.ctx.At(beat+1, func() { m.ctx.Scene.PlayState(m.bear, "BeastHuhR", beat+1, 0.5) })
		}
		m.ctx.At(beat+2, func() { m.ctx.Scene.PlayState(m.bear, "BeastReady", beat+2, 0.75) })
	} else {
		m.ctx.Scene.PlayState(m.bear, "BeastCut", beat, 0.75)
	}
	if zoom {
		m.ctx.Scene.SetPosOver(m.bear, 0, 0)
	}
}

func (m *Module) bearCutMid(beat float64, noImpact bool) {
	if noImpact {
		m.ctx.Scene.PlayState(m.bear, "BeastCutMidNoImpact", beat, 0.75)
		return
	}
	m.ctx.Scene.PlayState(m.bear, "BeastCutMid", beat, 0.75)
}

func (m *Module) setSnow(on bool, wind, strength float64) {
	m.snowOn = on
	m.snowWind = wind
	m.snowStrength = strength
	if m.gameComp["_snowParticle"] != "" {
		m.ctx.Scene.SetActive(m.gameComp["_snowParticle"], on)
	}
}

func (m *Module) applySnowAt(beat float64) {
	var last *snowEvt
	for i := range m.snows {
		if m.snows[i].beat < beat {
			last = &m.snows[i]
		}
	}
	if last != nil {
		m.setSnow(last.on, last.wind, last.particles)
	}
}

func (m *Module) drawSnow(screen *ebiten.Image, beat float64) {
	if !m.snowOn {
		return
	}
	alpha := uint8(math.Min(180, 40+m.snowStrength))
	col := color.RGBA{255, 255, 255, alpha}
	for i, fl := range m.snowflakes {
		x := math.Mod(fl.x+0.04*m.snowWind*beat+float64(i), 18) - 9
		y := math.Mod(fl.y-fl.speed*beat, 11) - 5
		px, py := m.proj.Apply(x+fl.drift*math.Sin(beat+float64(i)), y)
		vector.DrawFilledCircle(screen, float32(px), float32(py), 1.2, col, false)
	}
}
