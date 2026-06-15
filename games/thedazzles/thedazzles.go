// Package thedazzles ports The Dazzles' hold/pose call-and-response,
// per-box palette changes, girl expression state, and pose/star particles from
// Assets/Scripts/Games/TheDazzles.
package thedazzles

import (
	"image/color"
	"math"
	"math/rand"
	"sort"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"hsdemo/engine"
	"hsdemo/kart"
	"hsdemo/riq"
)

const gameID = "theDazzles"

const (
	emotionNeutral = iota
	emotionHappy
	emotionAngry
	emotionOuch
)

const (
	countDS = iota
	countMegamix
	countRandom
)

var (
	defaultExterior = rgba(156, 254, 246)
	defaultInterior = rgba(66, 255, 239)
	defaultWall     = rgba(0, 222, 197)
	defaultRoof     = rgba(0, 189, 172)
)

type bopEvt struct {
	beat, length float64
	goBop, auto  bool
}

type crouchEvt struct {
	beat, length float64
	countIn      int
}

type poseEvt struct {
	beat, length                 float64
	upLeft, upMiddle, upRight    float64
	downLeft, downMiddle, player float64
	stars, cheer                 bool
}

type colorEvt struct {
	beat, length float64
	ext0, ext1   [4]float64
	int0, int1   [4]float64
	wall0, wall1 [4]float64
	roof0, roof1 [4]float64
	ease         int
}

type girlState struct {
	root, holder, hold, blackFlash, head string
	headPrefix                           string
	canBop, holding, preparing, ouched   bool
	emotion                              int
}

type burst struct {
	beat  float64
	stars bool
	seed  int
}

type Module struct {
	ctx  *engine.Ctx
	proj kart.Aff
	rng  *rand.Rand

	player girlState
	npc    []girlState

	poseRoot, starsRoot string
	interiorMat         string
	outerBoxes          []string

	bops     []bopEvt
	crouches []crouchEvt
	poses    []poseEvt
	colors   []colorEvt

	crouchEndBeat float64
	doingPoses    bool
	shouldHold    bool
	lastPulse     int
	bursts        []burst
}

func New() engine.Module {
	return &Module{
		rng:           rand.New(rand.NewSource(1)),
		crouchEndBeat: math.Inf(-1),
		lastPulse:     -1 << 30,
	}
}

func (m *Module) ID() string { return gameID }

func (m *Module) Load(ctx *engine.Ctx) error {
	m.ctx = ctx
	if err := ctx.LoadAssets(gameID); err != nil {
		return err
	}
	m.proj = kart.Translate(engine.ScreenW/2, engine.ScreenH/2).Mul(kart.Scale(54, -54))

	comp := ctx.Assets.Extra.Components["game"]
	npcRoots := comp.RefArrays["npcGirls"]
	if len(npcRoots) == 0 {
		npcRoots = []string{
			"NpcHolder/Girl",
			"NpcHolder (1)/Girl",
			"NpcHolder (2)/Girl",
			"NpcHolder (3)/Girl",
			"NpcHolder (4)/Girl",
		}
	}
	for _, p := range npcRoots {
		m.npc = append(m.npc, newGirl(p))
	}
	m.player = newGirl(roleOr(ctx, "player", comp.Refs["player"], "PlayerHolder/Girl"))
	m.poseRoot = roleOr(ctx, "poseEffect", comp.Refs["poseEffect"], "PoseParticle")
	m.starsRoot = roleOr(ctx, "starsEffect", comp.Refs["starsEffect"], "NightWalkStars")
	m.interiorMat = comp.Refs["interiorMat"]
	if m.interiorMat == "" {
		m.interiorMat = "boxinterior"
	}
	for _, n := range ctx.Assets.Rig.Nodes {
		if strings.HasPrefix(n.Path, "Dividers/Box") && !strings.HasSuffix(n.Path, "/Box") {
			m.outerBoxes = append(m.outerBoxes, n.Path)
		}
	}
	m.applyBoxColors(defaultExterior, defaultInterior, defaultWall, defaultRoof)
	m.resetGirlVisuals(0)
	return nil
}

func (m *Module) OnEvent(e *riq.Entity) {
	switch e.Datamodel {
	case gameID + "/bop":
		m.bops = append(m.bops, bopEvt{
			beat: e.Beat, length: e.Length,
			goBop: boolDefault(e, "toggle2", true),
			auto:  boolParam(e, "toggle"),
		})
	case gameID + "/crouch", gameID + "/crouchStretch":
		m.crouches = append(m.crouches, crouchEvt{
			beat: e.Beat, length: e.Length,
			countIn: int(e.Float("countIn", countDS)),
		})
	case gameID + "/poseThree":
		m.poses = append(m.poses, poseEvt{beat: e.Beat, length: e.Length,
			upLeft: 0, upMiddle: 1, upRight: 2, downLeft: 0, downMiddle: 1, player: 2,
			stars: boolParam(e, "toggle"), cheer: boolDefault(e, "toggle2", true)})
	case gameID + "/poseTwo":
		m.poses = append(m.poses, poseEvt{beat: e.Beat, length: e.Length,
			upLeft: 0, upMiddle: 0, upRight: 0, downLeft: 2, downMiddle: 2, player: 2,
			stars: boolDefault(e, "toggle", true), cheer: boolDefault(e, "toggle2", true)})
	case gameID + "/poseSixDiagonal":
		m.poses = append(m.poses, poseEvt{beat: e.Beat, length: e.Length,
			upLeft: 0, upMiddle: 2.75, upRight: 1.5, downLeft: 2, downMiddle: 0.75, player: 3.5,
			stars: boolParam(e, "toggle"), cheer: boolDefault(e, "toggle2", true)})
	case gameID + "/poseSixColumns":
		m.poses = append(m.poses, poseEvt{beat: e.Beat, length: e.Length,
			upLeft: 0, upMiddle: 0.5, upRight: 1, downLeft: 2, downMiddle: 2.5, player: 3,
			stars: boolParam(e, "toggle"), cheer: boolDefault(e, "toggle2", true)})
	case gameID + "/poseSix":
		m.poses = append(m.poses, poseEvt{beat: e.Beat, length: e.Length,
			upLeft: 0, upMiddle: 0.5, upRight: 1, downLeft: 1.5, downMiddle: 2, player: 2.5,
			stars: boolDefault(e, "toggle", true), cheer: boolDefault(e, "toggle2", true)})
	case gameID + "/customPose":
		m.poses = append(m.poses, poseEvt{
			beat: e.Beat, length: e.Length,
			upLeft: e.Float("upLeft", 0), upMiddle: e.Float("upMiddle", 1), upRight: e.Float("upRight", 2),
			downLeft: e.Float("downLeft", 0), downMiddle: e.Float("downMiddle", 1), player: e.Float("player", 2),
			stars: boolParam(e, "toggle"), cheer: boolDefault(e, "toggle2", true),
		})
	case gameID + "/forceHold":
		m.ctx.At(e.Beat, func() { m.forceHold() })
	case gameID + "/boxColor":
		def := colorEvt{beat: e.Beat, length: e.Length,
			ext0: defaultExterior, ext1: defaultExterior,
			int0: defaultInterior, int1: defaultInterior,
			wall0: defaultWall, wall1: defaultWall,
			roof0: defaultRoof, roof1: defaultRoof}
		m.colors = append(m.colors, colorEvt{
			beat: e.Beat, length: e.Length,
			ext0: colorParam(e, "extStart", def.ext0), ext1: colorParam(e, "extEnd", def.ext1),
			int0: colorParam(e, "intStart", def.int0), int1: colorParam(e, "intEnd", def.int1),
			wall0: colorParam(e, "wallStart", def.wall0), wall1: colorParam(e, "wallEnd", def.wall1),
			roof0: colorParam(e, "roofStart", def.roof0), roof1: colorParam(e, "roofEnd", def.roof1),
			ease: int(e.Float("ease", 0)),
		})
	}
}

func (m *Module) Ready() {
	sort.Slice(m.bops, func(i, j int) bool { return m.bops[i].beat < m.bops[j].beat })
	sort.Slice(m.crouches, func(i, j int) bool { return m.crouches[i].beat < m.crouches[j].beat })
	sort.Slice(m.poses, func(i, j int) bool { return m.poses[i].beat < m.poses[j].beat })
	sort.Slice(m.colors, func(i, j int) bool { return m.colors[i].beat < m.colors[j].beat })

	for _, ev := range m.bops {
		ev := ev
		if ev.goBop {
			for b := ev.beat; b < ev.beat+ev.length-1e-6; b++ {
				bb := b
				m.ctx.At(bb, func() { m.bopAll(bb) })
			}
		}
	}
	for _, ev := range m.crouches {
		ev := ev
		m.scheduleCrouchSounds(ev)
		m.crouchStretchable(ev)
	}
	for _, ev := range m.poses {
		ev := ev
		m.schedulePose(ev)
	}
}

func (m *Module) OnSwitch(beat float64) {
	m.resetGirlVisuals(beat)
	m.lastPulse = int(math.Floor(beat))
	m.applyPersistentColor(beat)
}

func (m *Module) Whiff(beat float64) {
	m.prepareGirl(&m.player, beat, false)
	m.ctx.Sound("miss")
	for i := range m.npc {
		if m.npc[i].emotion != emotionOuch {
			m.npc[i].emotion = emotionAngry
		}
	}
}

func (m *Module) Update(_, beat float64) {
	if m.ctx.ReleasedNow() && !m.ctx.ExpectingReleaseNow() {
		if m.doingPoses {
			m.poseGirl(&m.player, beat, false, true)
			m.ctx.Sound("miss")
			for i := range m.npc {
				m.ouchGirl(&m.npc[i], beat)
			}
			m.shouldHold = false
		} else if m.shouldHold {
			m.unprepareGirl(&m.player, beat)
			m.shouldHold = false
		}
	}
	pulse := int(math.Floor(beat + 1e-6))
	if pulse > m.lastPulse {
		for b := m.lastPulse + 1; b <= pulse; b++ {
			if m.autoBopAt(float64(b)) {
				m.bopAll(float64(b))
			}
		}
		m.lastPulse = pulse
	}
}

func (m *Module) Draw(screen *ebiten.Image, _, beat float64) {
	screen.Fill(color.RGBA{0, 0, 0, 0xff})
	m.applyBoxEase(beat)
	m.ctx.SampleScene(beat)
	m.queueBursts(beat)
	m.ctx.Scene.Draw(screen, m.proj)
}

func (m *Module) crouchStretchable(ev crouchEvt) {
	actual := ev.length / 3
	m.ctx.At(ev.beat, func() { m.crouchEndBeat = ev.beat + ev.length })
	m.ctx.ScheduleInput(ev.beat+2*actual, func(state float64, _ engine.Judgment) {
		m.player.canBop = false
		m.prepareGirl(&m.player, ev.beat+2*actual, true)
	}, nil)
	m.ctx.At(ev.beat, func() {
		m.npc[1].canBop = false
		m.npc[4].canBop = false
		m.prepareGirl(&m.npc[1], ev.beat, true)
		m.prepareGirl(&m.npc[4], ev.beat, true)
	})
	m.ctx.At(ev.beat+actual, func() {
		m.npc[0].canBop = false
		m.npc[3].canBop = false
		m.prepareGirl(&m.npc[0], ev.beat+actual, true)
		m.prepareGirl(&m.npc[3], ev.beat+actual, true)
	})
	m.ctx.At(ev.beat+2*actual, func() {
		m.npc[2].canBop = false
		m.prepareGirl(&m.npc[2], ev.beat+2*actual, true)
	})
}

func (m *Module) scheduleCrouchSounds(ev crouchEvt) {
	actual := ev.length / 3
	typ := ev.countIn
	if typ == countRandom {
		typ = m.rng.Intn(2)
	}
	switch typ {
	case countDS:
		m.ctx.SoundAtOff(ev.beat, "holdDS3", 0.75, 0.212)
		m.ctx.SoundAtOff(ev.beat+actual, "holdDS2", 0.75, 0.242)
		m.ctx.SoundAtOff(ev.beat+2*actual, "hold1", 1, 0.019)
	case countMegamix:
		m.ctx.SoundAtOff(ev.beat, "hold3", 1, 0.267)
		m.ctx.SoundAtOff(ev.beat+actual, "hold2", 1, 0.266)
		m.ctx.SoundAtOff(ev.beat+2*actual, "hold1", 1, 0.019)
	}
}

func (m *Module) schedulePose(ev poseEvt) {
	target := ev.beat + ev.player
	m.ctx.ScheduleInputRelease(target, func(state float64, j engine.Judgment) {
		m.shouldHold = false
		m.ctx.Sound("pose")
		m.ctx.Sound("posePlayer")
		if j == engine.JudgeNG {
			m.poseGirl(&m.player, target, true, true)
			return
		}
		m.successPose(target, ev.stars, ev.cheer)
	}, func() { m.missPose() })

	crouchBeat := ev.beat - 1
	if crouchBeat < m.crouchEndBeat {
		crouchBeat = m.crouchEndBeat - 1
	}
	m.ctx.SoundAt(crouchBeat, "crouch", 1)
	for _, s := range uniquePoseSounds([]float64{ev.upLeft, ev.upMiddle, ev.upRight, ev.downLeft, ev.downMiddle}, ev.player) {
		m.ctx.SoundAt(ev.beat+s, "pose", 1)
	}
	poses := []struct {
		off float64
		idx int
	}{
		{ev.upLeft, 4},
		{ev.upMiddle, 3},
		{ev.upRight, 2},
		{ev.downLeft, 1},
		{ev.downMiddle, 0},
	}
	sort.SliceStable(poses, func(i, j int) bool { return poses[i].off < poses[j].off })
	for _, p := range poses {
		p := p
		m.startReleaseBox(&m.npc[p.idx], ev.beat+p.off)
		m.ctx.At(ev.beat+p.off, func() { m.poseGirl(&m.npc[p.idx], ev.beat+p.off, true, true) })
	}
	m.startReleaseBox(&m.player, target)
	m.ctx.At(ev.beat-1, func() {
		for i := range m.npc {
			m.npc[i].canBop = false
			m.holdGirl(&m.npc[i], ev.beat-1)
		}
		m.player.canBop = false
		m.holdGirl(&m.player, ev.beat-1)
	})
	m.ctx.At(ev.beat, func() { m.doingPoses = true })
	m.ctx.At(target, func() { m.doingPoses = false })
	m.ctx.At(ev.beat+ev.length, func() {
		for i := range m.npc {
			m.endPoseGirl(&m.npc[i], ev.beat+ev.length)
		}
		m.endPoseGirl(&m.player, ev.beat+ev.length)
	})
	m.ctx.At(ev.beat+ev.length+0.1, func() {
		for i := range m.npc {
			m.npc[i].canBop = true
		}
		m.player.canBop = true
	})
}

func (m *Module) successPose(beat float64, stars, cheer bool) {
	m.poseGirl(&m.player, beat, true, true)
	if cheer {
		m.ctx.Sound("applause")
	}
	for i := range m.npc {
		m.npc[i].emotion = emotionHappy
	}
	m.player.emotion = emotionHappy
	if stars {
		n := m.rng.Intn(5) + 1
		m.ctx.Sound("stars" + smallInt(n))
		m.bursts = append(m.bursts, burst{beat: beat, stars: true, seed: n})
	} else {
		m.bursts = append(m.bursts, burst{beat: beat, seed: m.rng.Int()})
	}
}

func (m *Module) missPose() {
	for i := range m.npc {
		if m.npc[i].emotion != emotionOuch {
			m.npc[i].emotion = emotionAngry
		}
	}
	m.player.ouched = true
}

func (m *Module) forceHold() {
	m.shouldHold = true
	for i := range m.npc {
		m.prepareGirl(&m.npc[i], m.ctx.Beat(), true)
	}
	m.prepareGirl(&m.player, m.ctx.Beat(), true)
}

func (m *Module) bopAll(beat float64) {
	for i := range m.npc {
		m.bopGirl(&m.npc[i], beat)
	}
	m.bopGirl(&m.player, beat)
}

func (m *Module) autoBopAt(beat float64) bool {
	on := false
	for _, ev := range m.bops {
		if ev.beat > beat {
			break
		}
		on = ev.auto
	}
	return on
}

func (m *Module) prepareGirl(g *girlState, beat float64, hit bool) {
	if hit {
		m.ctx.Scene.PlayState(g.hold, "HoldBox", beat, 0.25)
		m.ctx.Scene.SetActive(g.blackFlash, true)
		m.ctx.Scene.PlayState(g.holder, "Dark", beat, 1)
	}
	g.holding = true
	g.ouched = false
	if g.preparing {
		m.holdGirl(g, beat)
		return
	}
	m.ctx.Scene.PlayState(g.root, "Prepare", beat, 1)
}

func (m *Module) startReleaseBox(g *girlState, beat float64) {
	m.ctx.At(beat-1, func() {
		if g.holding {
			m.ctx.Scene.PlayState(g.hold, "ReleaseBox", beat-1, 0.25)
		}
	})
}

func (m *Module) poseGirl(g *girlState, beat float64, hit bool, flash bool) {
	if hit {
		m.ctx.Scene.PlayState(g.root, "Pose", beat, 0.5)
		m.ctx.Scene.PlayState(g.holder, "PoseFlash", beat, 0.5)
		g.ouched = false
	} else {
		m.ctx.Scene.PlayState(g.root, "MissPose", beat, 0.5)
		if flash {
			m.ctx.Scene.PlayState(g.holder, "MissFlash", beat, 0.5)
		}
		g.emotion = emotionOuch
		g.ouched = true
	}
	m.ctx.Scene.PlayState(g.hold, "HoldNothing", beat, 1)
	g.holding = false
	g.preparing = false
	m.ctx.Scene.SetActive(g.blackFlash, false)
}

func (m *Module) endPoseGirl(g *girlState, beat float64) {
	if g.holding || g.ouched {
		return
	}
	m.ctx.Scene.PlayState(g.root, "EndPose", beat, 0.5)
}

func (m *Module) holdGirl(g *girlState, beat float64) {
	g.preparing = true
	if !g.holding {
		return
	}
	m.ctx.Scene.PlayState(g.root, "Hold", beat, 0.5)
}

func (m *Module) ouchGirl(g *girlState, beat float64) {
	m.ctx.Scene.PlayState(g.root, "Ouch", beat, 0.5)
	g.emotion = emotionOuch
	g.ouched = true
}

func (m *Module) unprepareGirl(g *girlState, beat float64) {
	m.ctx.ScoreMiss()
	m.ctx.Scene.PlayState(g.hold, "HoldNothing", beat, 1)
	g.canBop = true
	if g.preparing {
		m.ctx.Scene.PlayState(g.root, "StopHold", beat, 0.5)
	} else {
		m.ctx.Scene.PlayState(g.root, "EndPrepare", beat, 0.5)
	}
	g.holding = false
	g.preparing = false
	g.ouched = true
	m.ctx.Scene.SetActive(g.blackFlash, false)
	m.ctx.Scene.PlayState(g.holder, "Lit", beat, 1)
}

func (m *Module) bopGirl(g *girlState, beat float64) {
	if !g.canBop || g.holding {
		return
	}
	switch g.emotion {
	case emotionHappy:
		m.ctx.Scene.PlayState(g.root, "HappyBop", beat, 0.4)
	case emotionAngry:
		m.ctx.Scene.SetSpriteOver(g.head, g.headPrefix+"Angry")
		m.ctx.Scene.PlayState(g.root, "IdleBop", beat, 0.4)
	case emotionOuch:
		m.ctx.Scene.PlayState(g.root, "OuchBop", beat, 0.4)
	default:
		m.ctx.Scene.PlayState(g.root, "IdleBop", beat, 0.4)
	}
}

func (m *Module) resetGirlVisuals(beat float64) {
	sec := m.ctx.SecPerBeat(beat)
	for i := range m.npc {
		m.resetGirl(&m.npc[i], beat, sec)
	}
	m.resetGirl(&m.player, beat, sec)
}

func (m *Module) resetGirl(g *girlState, beat, sec float64) {
	g.canBop = true
	g.holding = false
	g.preparing = false
	g.ouched = false
	g.emotion = emotionNeutral
	m.ctx.Scene.SetActive(g.blackFlash, false)
	m.ctx.Scene.PlayDefaultState(g.root, beat, sec)
	m.ctx.Scene.PlayDefaultState(g.holder, beat, sec)
	m.ctx.Scene.PlayDefaultState(g.hold, beat, sec)
	m.ctx.Scene.SetSpriteOver(g.head, g.headPrefix+"Neutral")
}

func (m *Module) applyBoxEase(beat float64) {
	ext, in, wall, roof := defaultExterior, defaultInterior, defaultWall, defaultRoof
	for _, ev := range m.colors {
		if ev.beat > beat {
			break
		}
		u := 1.0
		if ev.length > 0 && beat < ev.beat+ev.length {
			u = clamp01((beat - ev.beat) / ev.length)
		}
		ext = easeColor(ev.ease, ev.ext0, ev.ext1, u)
		in = easeColor(ev.ease, ev.int0, ev.int1, u)
		wall = easeColor(ev.ease, ev.wall0, ev.wall1, u)
		roof = easeColor(ev.ease, ev.roof0, ev.roof1, u)
	}
	m.applyBoxColors(ext, in, wall, roof)
}

func (m *Module) applyPersistentColor(beat float64) {
	ext, in, wall, roof := defaultExterior, defaultInterior, defaultWall, defaultRoof
	for _, ev := range m.colors {
		if ev.beat >= beat {
			break
		}
		u := 1.0
		if ev.length > 0 && beat < ev.beat+ev.length {
			u = clamp01((beat - ev.beat) / ev.length)
		}
		ext = easeColor(ev.ease, ev.ext0, ev.ext1, u)
		in = easeColor(ev.ease, ev.int0, ev.int1, u)
		wall = easeColor(ev.ease, ev.wall0, ev.wall1, u)
		roof = easeColor(ev.ease, ev.roof0, ev.roof1, u)
	}
	m.applyBoxColors(ext, in, wall, roof)
}

func (m *Module) applyBoxColors(ext, in, wall, roof [4]float64) {
	m.ctx.Scene.SetPaletteFor(m.interiorMat, kart.Palette{Alpha: roof, Fill: in, Outline: wall})
	for _, p := range m.outerBoxes {
		// The exterior material reference is a material asset, not a scene node.
		// Override only the outer box renderers so the interior child keeps the
		// three-channel boxinterior palette.
		m.ctx.Scene.SetPaletteOver(p, kart.Palette{Alpha: ext, Fill: ext, Outline: ext})
	}
}

func (m *Module) queueBursts(beat float64) {
	active := m.bursts[:0]
	for _, b := range m.bursts {
		age := beat - b.beat
		if age < 0 {
			active = append(active, b)
			continue
		}
		if age > 1.2 {
			continue
		}
		if b.stars {
			m.queueStarBurst(age)
		} else {
			m.queuePoseBurst(age)
		}
		active = append(active, b)
	}
	m.bursts = active
}

func (m *Module) queuePoseBurst(age float64) {
	root, ok := m.ctx.Scene.NodeWorld(m.poseRoot)
	if !ok {
		return
	}
	alpha := 1 - clamp01(age/0.7)
	for i, part := range []struct {
		sprite string
		x, y   float64
		scale  float64
	}{
		{"dazzleseffects_8", 0, 0, 0.15 + age*0.8},
		{"dazzleseffects_5", -0.8, 0.4, 0.35},
		{"dazzleseffects_5", 0.75, 0.5, 0.3},
		{"dazzleseffects_9", 0.2, -0.5, 0.45},
	} {
		ang := float64(i)*1.7 + age*2
		w := root.Mul(kart.Translate(part.x+math.Cos(ang)*age*1.8, part.y+math.Sin(ang)*age*1.1)).Mul(kart.Scale(part.scale, part.scale))
		m.ctx.Scene.Queue(kart.ExtraSprite{Sprite: part.sprite, World: w, Order: 95 + i, Tint: [4]float64{1, 1, 1, alpha}})
	}
}

func (m *Module) queueStarBurst(age float64) {
	root, ok := m.ctx.Scene.NodeWorld(m.starsRoot)
	if !ok {
		return
	}
	alpha := 1 - clamp01(age/1.0)
	for i, part := range []struct {
		sprite string
		x, y   float64
		scale  float64
	}{
		{"dazzleseffects_6", 0, 0, 0.25 + age*0.4},
		{"dazzleseffects_7", -0.5, 0.15, 0.25},
		{"dazzleseffects_8", 0.55, 0.25, 0.2},
		{"dazzleseffects_9", 0, -0.45, 0.4},
	} {
		ang := float64(i)*1.3 + age*3
		w := root.Mul(kart.Translate(part.x+math.Sin(ang)*age, part.y+math.Cos(ang)*age)).Mul(kart.Scale(part.scale, part.scale))
		m.ctx.Scene.Queue(kart.ExtraSprite{Sprite: part.sprite, World: w, Order: 100 + i, Tint: [4]float64{1, 1, 1, alpha}})
	}
}

func newGirl(root string) girlState {
	holder := strings.TrimSuffix(root, "/Girl")
	prefix := headPrefix(root)
	return girlState{
		root: root, holder: holder, hold: root + "/HoldEffect",
		blackFlash: root + "/BlackFlash", head: root + "/Head",
		headPrefix: prefix, canBop: true,
	}
}

func headPrefix(root string) string {
	switch root {
	case "NpcHolder/Girl":
		return "BottomMiddle"
	case "NpcHolder (1)/Girl":
		return "BottomLeft"
	case "NpcHolder (2)/Girl":
		return "RightUp"
	case "NpcHolder (3)/Girl":
		return "MiddleUp"
	case "NpcHolder (4)/Girl":
		return "LeftUp"
	default:
		return "Player"
	}
}

func uniquePoseSounds(v []float64, player float64) []float64 {
	seen := map[float64]bool{}
	var out []float64
	for _, x := range v {
		if x == player || seen[x] {
			continue
		}
		seen[x] = true
		out = append(out, x)
	}
	sort.Float64s(out)
	return out
}

func roleOr(ctx *engine.Ctx, role, ref, fallback string) string {
	if p := ctx.Role(role); p != "" {
		return p
	}
	if ref != "" {
		return ref
	}
	return fallback
}

func rgba(r, g, b byte) [4]float64 {
	return [4]float64{float64(r) / 255, float64(g) / 255, float64(b) / 255, 1}
}

func easeColor(ease int, a, b [4]float64, u float64) [4]float64 {
	return [4]float64{
		engine.Ease(ease, a[0], b[0], u),
		engine.Ease(ease, a[1], b[1], u),
		engine.Ease(ease, a[2], b[2], u),
		engine.Ease(ease, a[3], b[3], u),
	}
}

func boolParam(e *riq.Entity, key string) bool { return e.Float(key, 0) != 0 }

func boolDefault(e *riq.Entity, key string, def bool) bool {
	if _, ok := e.Data[key]; !ok {
		return def
	}
	return boolParam(e, key)
}

func colorParam(e *riq.Entity, key string, def [4]float64) [4]float64 {
	if mm, ok := e.Data[key].(map[string]any); ok {
		return [4]float64{num(mm["r"], def[0]), num(mm["g"], def[1]), num(mm["b"], def[2]), num(mm["a"], def[3])}
	}
	return def
}

func num(v any, def float64) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case int:
		return float64(x)
	}
	return def
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func smallInt(n int) string {
	if n < 0 || n > 9 {
		return ""
	}
	return string(rune('0' + n))
}
