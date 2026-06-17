// Package airboarder ports Airboarder's event timing, input windows, sounds,
// and script-driven colors. The current renderer combines a temporary 2D layer
// with the extracted sky MeshRenderer while the engine grows full imported-FBX
// material-texture and camera support for the original 3D scene; see README.
package airboarder

import (
	"math"
	"sort"

	"github.com/hajimehoshi/ebiten/v2"

	"hsdemo/engine"
	"hsdemo/kart"
	"hsdemo/riq"
)

const (
	gameID = "airboarder"

	poseIdle   = "hover"
	poseBop    = "bop"
	poseDuck   = "duck"
	poseCharge = "charge"
	poseHold   = "hold"
	poseJump   = "jump"
	poseLetsGo = "letsgo"
	poseHit1   = "hit1"
	poseHit2   = "hit2"

	obstacleDuck = iota
	obstacleCrouch
	obstacleJump
)

var (
	defaultBG     = [4]float64{0.9921569, 0.7686275, 0.9921569, 1}
	defaultFloor  = [4]float64{1, 1, 1, 1}
	defaultStripe = [4]float64{0.8274511, 0.1254902, 0.8078432, 1}
	defaultCloud  = [4]float64{1, 1, 1, 1}
)

type boarder struct {
	x, y      float64
	pose      string
	poseBeat  float64
	cantBop   bool
	cantUntil float64
}

type bopEvt struct {
	beat, length float64
	toggle       bool
	auto         bool
}

type colorEvt struct {
	beat, length float64
	a0, a1       [4]float64
	b0, b1       [4]float64
	ease         int
}

type cameraEvt struct {
	beat, length float64
	rotY, rotX   float64
	zoom         float64
	x, y         float64
	ease         int
	additive     bool
}

type obstacle struct {
	kind       int
	appearBeat float64
	targetBeat float64
	broken     bool
	shake      bool
	effectBeat float64
}

type Module struct {
	ctx  *engine.Ctx
	proj kart.Aff

	cpu1   boarder
	cpu2   boarder
	player boarder

	bops       []bopEvt
	bgEvents   []colorEvt
	floorEvts  []colorEvt
	cameraEvts []cameraEvt
	obstacles  []*obstacle

	wantsCrouch bool
	lastPulse   int
	switchBeat  float64

	floorLoopDelta float64
}

func New() engine.Module { return &Module{} }

func (m *Module) ID() string { return gameID }

func (m *Module) Load(ctx *engine.Ctx) error {
	m.ctx = ctx
	if err := ctx.LoadAssets(gameID); err != nil {
		return err
	}
	m.proj = kart.Translate(engine.ScreenW/2, engine.ScreenH/2).Mul(kart.Scale(54, -54))
	m.floorLoopDelta = floorMoveDelta(ctx.Assets.Anims["Animations/floor/move"])
	m.resetBoarders(0)

	// Only the single-geometry sky MeshRenderer is drawn today; playing the
	// controller roots still keeps the remaining extracted 3D data exercised
	// until multi-geometry FBX and skinned rendering land.
	for _, role := range []string{"CPU1", "CPU2", "Player", "Dog", "Tail", "Floor", "archBasic", "wallBasic"} {
		if p := ctx.Role(role); p != "" {
			ctx.Scene.PlayDefaultState(p, 0, ctx.SecPerBeat(0))
		}
	}
	return nil
}

func (m *Module) OnEvent(e *riq.Entity) {
	switch e.Datamodel {
	case "airboarder/bop":
		ev := bopEvt{
			beat: e.Beat, length: e.Length,
			toggle: boolDefault(e, "toggle", true),
			auto:   boolDefault(e, "auto", false),
		}
		m.bops = append(m.bops, ev)
		if ev.toggle {
			for i := 0; i < int(ev.length); i++ {
				b := ev.beat + float64(i)
				m.ctx.At(b, func() { m.bop(b) })
			}
		}
	case "airboarder/duck":
		m.prepareJump(e.Beat, boolDefault(e, "ready", true))
		m.requestObstacle(e.Beat-25, e.Beat, obstacleDuck)
	case "airboarder/crouch":
		m.prepareJump(e.Beat, boolDefault(e, "ready", true))
		m.requestObstacle(e.Beat-25, e.Beat, obstacleCrouch)
	case "airboarder/jump":
		m.prepareJump(e.Beat, boolDefault(e, "ready", false))
		m.requestObstacle(e.Beat-25, e.Beat, obstacleJump)
	case "airboarder/forceCharge":
		m.ctx.At(e.Beat, func() { m.forceCharge(e.Beat) })
	case "airboarder/letsGo":
		m.yeahLetsGo(e.Beat, boolDefault(e, "sound", true))
	case "airboarder/fade background":
		m.bgEvents = append(m.bgEvents, colorEvt{
			beat: e.Beat, length: e.Length, ease: int(e.Float("ease", 0)),
			a0: colorParam(e, "colorStart", defaultBG), a1: colorParam(e, "colorEnd", defaultBG),
			b0: colorParam(e, "cloudStart", defaultCloud), b1: colorParam(e, "cloudEnd", defaultCloud),
		})
	case "airboarder/fade floor":
		m.floorEvts = append(m.floorEvts, colorEvt{
			beat: e.Beat, length: e.Length, ease: int(e.Float("ease", 0)),
			a0: colorParam(e, "colorStart", defaultFloor), a1: colorParam(e, "colorEnd", defaultFloor),
			b0: colorParam(e, "stripeStart", defaultStripe), b1: colorParam(e, "stripeEnd", defaultStripe),
		})
	case "airboarder/camera":
		m.cameraEvts = append(m.cameraEvts, cameraEvt{
			beat: e.Beat, length: e.Length,
			rotY: e.Float("valA", 0), rotX: e.Float("RotateX", 0),
			zoom: e.Float("valB", 1), x: e.Float("cameraX", 0), y: e.Float("cameraY", 0),
			ease: int(e.Float("type", 0)), additive: boolDefault(e, "additive", true),
		})
	}
}

func (m *Module) Ready() {
	sort.SliceStable(m.bops, func(i, j int) bool { return m.bops[i].beat < m.bops[j].beat })
	sort.SliceStable(m.bgEvents, func(i, j int) bool { return m.bgEvents[i].beat < m.bgEvents[j].beat })
	sort.SliceStable(m.floorEvts, func(i, j int) bool { return m.floorEvts[i].beat < m.floorEvts[j].beat })
	sort.SliceStable(m.cameraEvts, func(i, j int) bool { return m.cameraEvts[i].beat < m.cameraEvts[j].beat })
	sort.SliceStable(m.obstacles, func(i, j int) bool { return m.obstacles[i].appearBeat < m.obstacles[j].appearBeat })
}

func (m *Module) OnSwitch(beat float64) {
	m.switchBeat = beat
	m.lastPulse = int(math.Floor(beat)) - 1
	m.wantsCrouch = false
	m.resetBoarders(beat)
	m.persistSceneColors(beat)
	for _, role := range []string{"CPU1", "CPU2", "Player", "Dog", "Tail", "Floor"} {
		if p := m.ctx.Role(role); p != "" {
			m.ctx.Scene.PlayDefaultState(p, beat, m.ctx.SecPerBeat(beat))
		}
	}
}

func (m *Module) Whiff(beat float64) {
	if m.wantsCrouch {
		m.playBoarder(&m.player, poseCharge, beat, 1.5)
		m.player.cantBop = true
		return
	}
	m.playBoarder(&m.player, poseDuck, beat, 1.5)
	m.ctx.Sound("crouch")
	m.ctx.Sound("crouchvox")
	m.player.cantBop = true
	m.player.cantUntil = beat + 1.5
}

func (m *Module) Update(_ float64, beat float64) {
	m.clearCantFlags(beat)
	if m.ctx.ReleasedNow() && !m.ctx.ExpectingReleaseNow() && m.wantsCrouch {
		m.playBoarder(&m.player, poseHold, beat, 0.5)
		m.player.cantBop = false
	}
	for pulse := m.lastPulse + 1; pulse <= int(math.Floor(beat)); pulse++ {
		if pulse >= 0 && m.autoBopAt(float64(pulse)) {
			m.bop(float64(pulse))
		}
		m.lastPulse = pulse
	}
	m.sampleSceneAnimations(beat)
}

func (m *Module) Draw(screen *ebiten.Image, _ float64, beat float64) {
	_, cloud := m.bgAt(beat)
	floor, stripe := m.floorAt(beat)
	m.drawTemporaryFallbackBG(screen, beat)
	m.ctx.SampleSceneZ(beat, m.cameraZoomAdd(beat))
	m.ctx.Scene.Draw(screen, m.proj)
	m.drawTemporaryForeground(screen, beat, rgba(cloud), rgba(floor), rgba(stripe))
}

func (m *Module) resetBoarders(beat float64) {
	m.cpu1 = boarder{x: -4.6, y: -0.6, pose: poseIdle, poseBeat: beat}
	m.cpu2 = boarder{x: -0.25, y: -0.6, pose: poseIdle, poseBeat: beat}
	m.player = boarder{x: 4.2, y: -0.6, pose: poseIdle, poseBeat: beat}
}

func (m *Module) clearCantFlags(beat float64) {
	for _, b := range []*boarder{&m.cpu1, &m.cpu2, &m.player} {
		if b.cantUntil > 0 && beat >= b.cantUntil {
			b.cantBop = false
			b.cantUntil = 0
		}
		if poseExpired(b.pose, beat-b.poseBeat) {
			b.pose = poseIdle
		}
	}
}

func poseExpired(pose string, age float64) bool {
	switch pose {
	case poseCharge, poseHold:
		return false
	case poseBop:
		return age > 0.5
	default:
		return age > 1.5
	}
}

func (m *Module) sampleSceneAnimations(beat float64) {
	u := beat / 5
	if p := m.ctx.Role("Floor"); p != "" {
		m.ctx.Scene.PlayFrozen(p, "moving", u-math.Floor(u))
	}
	if p := m.ctx.Role("Dog"); p != "" {
		m.ctx.Scene.PlayNormalized(p, "Animations/dog/run", math.Mod(u*7.5, 1))
		m.ctx.Scene.PlayLayerNormalized(p+":wag", p, "Animations/dog/wag", math.Mod(u*2.5, 1))
	}
}

func (m *Module) persistSceneColors(_ float64) {
	// The authoritative color state is sampled in drawTemporary2D. This hook
	// mirrors Airboarder.PersistColor's switch-time responsibility and leaves a
	// single place to wire MeshRenderer material colors once supported.
}
