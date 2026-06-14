// Package tunnel ports Tunnel's cowbell input loop, scrolling background,
// moving tunnel wall, music ducking, and hand Bezier animation.
package tunnel

import (
	"image/color"
	"math"
	"sort"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"hsdemo/engine"
	"hsdemo/kart"
	"hsdemo/kmdata"
	"hsdemo/riq"
)

const (
	postTunnelScreenTime = 0.25
	defaultChunksPerSec  = 4.0
	defaultChunkSize     = 13.7
)

var bgLoops = []struct {
	near string
	far  string
}{
	{near: "Beach", far: "BeachFar"},
	{near: "Desert"},
	{near: "Field", far: "FieldFar"},
	{near: "City", far: "CityFar"},
	{near: "Night", far: "NightFar"},
	{near: "Moai"},
	{near: "CropStomp", far: "CropStompFar"},
	{near: "Quiz", far: "QuizFar"},
}

type cowbellEvt struct {
	beat, length float64
	started      bool
}

type tunnelEvt struct {
	beat, length float64
	volume       float64
	fadeDuration float64
}

type bgEvt struct {
	beat float64
	typ  int
}

type Module struct {
	ctx  *engine.Ctx
	proj kart.Aff

	cowbells []cowbellEvt
	tunnels  []tunnelEvt
	bgs      []bgEvt

	bgPaths []string

	wallPath         string
	wallRendererPath string
	frontHandPath    string
	cowbellPath      string
	driverPath       string
	driverArmsPath   string
	handParentPath   string
	handParentWorld  [2]float64
	handCurve        kmdata.Curve

	chunksPerSec float64
	chunkSize    float64
	tunnelTint   [4]float64
	tunnelScreen [4]float64

	handStart       float64
	canMiss         bool
	tunnelActive    bool
	inTunnel        bool
	tunnelStartTime float64
	tunnelEndTime   float64
	fadeDuration    float64

	stopRight  func()
	stopMiddle func()
	stopLeft   func()
}

func New() engine.Module {
	return &Module{
		chunksPerSec: defaultChunksPerSec,
		chunkSize:    defaultChunkSize,
		tunnelTint:   [4]float64{1, 0.89411765, 0.7921569, 1},
		tunnelScreen: [4]float64{0.09803922, 0.09803922, 0.09803922, 1},
		handStart:    -1,
	}
}

func (m *Module) ID() string { return "tunnel" }

func (m *Module) Load(ctx *engine.Ctx) error {
	m.ctx = ctx
	if err := ctx.LoadAssets("tunnel"); err != nil {
		return err
	}
	m.proj = kart.Translate(engine.ScreenW/2, engine.ScreenH/2).Mul(kart.Scale(54, -54))

	m.wallPath = roleOr(ctx, "tunnelWall", "Tunnel")
	m.wallRendererPath = roleOr(ctx, "tunnelWallRenderer", m.wallPath)
	m.frontHandPath = roleOr(ctx, "frontHand", "Player/Hand_Front")
	m.cowbellPath = roleOr(ctx, "cowbellAnimator", "Player/Cowbell")
	m.driverPath = roleOr(ctx, "driverAnimator", "Driver")
	m.driverArmsPath = "Driver/Arms"
	m.handParentPath = parentPath(m.frontHandPath)
	m.handParentWorld = nodeWorldPos(ctx, m.handParentPath)
	if c, ok := ctx.Assets.Extra.Curves["handCurve"]; ok {
		m.handCurve = c
	}
	if comp := ctx.Assets.Extra.Components["game"]; comp.Nums != nil {
		m.chunksPerSec = numDefault(comp.Nums, "tunnelChunksPerSec", m.chunksPerSec)
		m.chunkSize = numDefault(comp.Nums, "tunnelWallChunkSize", m.chunkSize)
		m.tunnelTint = colorFromNums(comp.Nums, "tunnelTint", m.tunnelTint)
		m.tunnelScreen = colorFromNums(comp.Nums, "tunnelScreen", m.tunnelScreen)
	}
	if refs := ctx.Assets.Extra.RefArrays["bg"]; len(refs) > 0 {
		m.bgPaths = append(m.bgPaths, refs...)
	} else if refs := ctx.Assets.Extra.Components["game"].RefArrays["bg"]; len(refs) > 0 {
		m.bgPaths = append(m.bgPaths, refs...)
	}

	m.ctx.Scene.SetActive(m.wallPath, false)
	m.ctx.Scene.SetPosOver(m.wallPath, m.chunkSize, 0)
	m.ctx.Scene.SetSizeOver(m.wallRendererPath, m.chunkSize, m.chunkSize)
	m.ctx.Scene.PlayDefaultState(m.cowbellPath, 0, ctx.SecPerBeat(0))
	m.playDriver("Idle", 0)
	m.ctx.Scene.PlayDefaultState(m.driverArmsPath, 0, ctx.SecPerBeat(0))
	m.setBg(0, 0)
	return nil
}

func roleOr(ctx *engine.Ctx, key, fallback string) string {
	if p := ctx.Role(key); p != "" {
		return p
	}
	return fallback
}

func parentPath(path string) string {
	if i := strings.LastIndexByte(path, '/'); i >= 0 {
		return path[:i]
	}
	return ""
}

func nodeWorldPos(ctx *engine.Ctx, path string) [2]float64 {
	idx, ok := ctx.Assets.NodeIndex(path)
	if !ok {
		return [2]float64{}
	}
	chain := []int{}
	for i := idx; i >= 0; i = ctx.Assets.Rig.Nodes[i].Parent {
		chain = append(chain, i)
	}
	w := kart.Identity()
	for i := len(chain) - 1; i >= 0; i-- {
		n := ctx.Assets.Rig.Nodes[chain[i]]
		w = w.Mul(kart.TRS(n.Pos[0], n.Pos[1], n.RotZ, n.Scale[0], n.Scale[1]))
	}
	return [2]float64{w.Tx, w.Ty}
}

func numDefault(nums map[string]float64, key string, def float64) float64 {
	if v, ok := nums[key]; ok {
		return v
	}
	return def
}

func colorFromNums(nums map[string]float64, prefix string, def [4]float64) [4]float64 {
	return [4]float64{
		numDefault(nums, prefix+".r", def[0]),
		numDefault(nums, prefix+".g", def[1]),
		numDefault(nums, prefix+".b", def[2]),
		numDefault(nums, prefix+".a", def[3]),
	}
}

func (m *Module) OnEvent(e *riq.Entity) {
	switch e.Datamodel {
	case "tunnel/cowbell":
		m.cowbells = append(m.cowbells, cowbellEvt{beat: e.Beat, length: e.Length})
	case "tunnel/tunnel":
		m.tunnels = append(m.tunnels, tunnelEvt{
			beat: e.Beat, length: e.Length,
			volume:       e.Float("volume", 10) / 100,
			fadeDuration: e.Float("duration", 2),
		})
	case "tunnel/countin":
		for i := 0; i < int(e.Length); i++ {
			name := "en/one"
			if i%2 == 1 {
				name = "en/two"
			}
			m.ctx.SoundAt(e.Beat+float64(i), name, 1)
		}
	case "tunnel/bg":
		m.bgs = append(m.bgs, bgEvt{beat: e.Beat, typ: int(e.Float("type", 0))})
	}
}

func (m *Module) Ready() {
	sort.SliceStable(m.cowbells, func(i, j int) bool { return m.cowbells[i].beat < m.cowbells[j].beat })
	sort.SliceStable(m.tunnels, func(i, j int) bool { return m.tunnels[i].beat < m.tunnels[j].beat })
	sort.SliceStable(m.bgs, func(i, j int) bool { return m.bgs[i].beat < m.bgs[j].beat })

	for i := range m.cowbells {
		i := i
		ev := m.cowbells[i]
		if m.ctx.GameAt(ev.beat) == m.ID() {
			m.startCowbellEvent(i, ev.beat)
		}
	}
	for _, ev := range m.tunnels {
		ev := ev
		m.ctx.At(ev.beat, func() { m.startTunnel(ev) })
	}
	for _, ev := range m.bgs {
		ev := ev
		m.ctx.At(ev.beat, func() { m.setBg(ev.typ, ev.beat) })
	}
}

func (m *Module) OnSwitch(beat float64) {
	m.playDriver("Idle", beat)
	m.ctx.Scene.PlayDefaultState(m.driverArmsPath, beat, m.ctx.SecPerBeat(beat))
	m.setBg(m.bgAt(beat), beat)
	for i := range m.cowbells {
		if !m.cowbells[i].started && m.cowbells[i].beat <= beat {
			m.startCowbellEvent(i, beat)
		}
	}
}

func (m *Module) Whiff(beat float64) {
	m.hitCowbell(beat)
	if m.canMiss {
		m.playDriver("Angry1", beat)
		m.ctx.ScoreMiss()
	}
}

func (m *Module) Update(t, beat float64) {
	if len(m.handCurve.Points) > 0 {
		p := math.Min(math.Max(beat-m.handStart, 0), 1)
		pt := kart.EvalBezier(m.handCurve, easeOutQuad(p))
		m.ctx.Scene.SetPosOver(m.frontHandPath, pt[0]-m.handParentWorld[0], pt[1]-m.handParentWorld[1])
	}
	if m.tunnelActive {
		x := m.chunkSize - m.chunksPerSec*m.chunkSize*(t-m.tunnelStartTime)
		m.ctx.Scene.SetPosOver(m.wallPath, x, 0)
	}
	if m.inTunnel && t >= m.tunnelEndTime+postTunnelScreenTime {
		restoreBeat := m.ctx.TimeToBeat(m.tunnelEndTime + postTunnelScreenTime)
		m.ctx.FadeMusicVolume(restoreBeat, m.fadeDuration, 1)
		m.ctx.Scene.SetMaterialOver(m.wallRendererPath, [4]float64{1, 1, 1, 1}, [4]float64{})
		m.inTunnel = false
	}
}

func (m *Module) Draw(screen *ebiten.Image, t, beat float64) {
	screen.Fill(color.NRGBA{R: 0x9b, G: 0xd5, B: 0xff, A: 0xff})
	m.ctx.SampleScene(beat)
	m.ctx.Scene.Draw(screen, m.proj)
}

func (m *Module) startCowbellEvent(idx int, fromBeat float64) {
	ev := m.cowbells[idx]
	m.cowbells[idx].started = true
	m.ctx.At(fromBeat, func() { m.canMiss = true })

	end := m.nextCowbellBeat(ev.beat)
	if sw := m.ctx.NextSwitchBeat(fromBeat); sw < end {
		end = sw
	}
	startK := math.Ceil(fromBeat - ev.beat)
	if startK < 0 {
		startK = 0
	}
	for target := ev.beat + startK; target < end; target++ {
		target := target
		m.ctx.ScheduleInput(target,
			func(state float64, j engine.Judgment) {
				now := m.ctx.Beat()
				m.hitCowbell(now)
				if math.Abs(state) >= 1 {
					m.playDriver("Disturbed", now)
				} else {
					m.playDriver("Idle", now)
				}
			},
			func() { m.playDriver("Angry1", m.ctx.Beat()) },
		)
	}
}

func (m *Module) nextCowbellBeat(beat float64) float64 {
	next := math.Inf(1)
	for _, ev := range m.cowbells {
		if ev.beat > beat && ev.beat < next {
			next = ev.beat
		}
	}
	return next
}

func (m *Module) hitCowbell(beat float64) {
	m.ctx.Sound("common_count-ins_cowbell")
	m.handStart = beat
	m.ctx.Scene.PlayState(m.cowbellPath, "Shake", beat, m.ctx.SecPerBeat(beat))
}

func (m *Module) playDriver(state string, beat float64) {
	m.ctx.Scene.PlayState(m.driverPath, state, beat, m.ctx.SecPerBeat(beat))
}

func (m *Module) startTunnel(ev tunnelEvt) {
	if m.ctx.Time() < m.tunnelEndTime+postTunnelScreenTime {
		return
	}
	targetBeat := ev.beat + ev.length
	m.tunnelStartTime = m.ctx.BeatToTime(ev.beat)
	m.tunnelEndTime = m.ctx.BeatToTime(targetBeat)
	m.fadeDuration = ev.fadeDuration

	width := tunnelWallWidth(m.tunnelStartTime, m.tunnelEndTime, m.chunksPerSec, m.chunkSize)
	m.ctx.Scene.SetSizeOver(m.wallRendererPath, width, m.chunkSize)
	m.ctx.Scene.SetPosOver(m.wallPath, m.chunkSize, 0)
	m.ctx.Scene.SetActive(m.wallPath, true)
	m.tunnelActive = true
	m.inTunnel = true
	m.ctx.FadeMusicVolume(ev.beat, ev.fadeDuration, ev.volume)

	m.stopTunnelLoops()
	m.ctx.At(ev.beat, func() { m.stopRight = m.ctx.SoundLoop("tunnelRight") })
	m.ctx.At(ev.beat+6.0/48.0, func() { m.stopMiddle = m.ctx.SoundLoop("tunnelMiddle") })
	m.ctx.At(ev.beat+12.0/48.0, func() { m.stopLeft = m.ctx.SoundLoop("tunnelLeft") })

	tunnelEndBeat := m.ctx.TimeToBeat(m.tunnelEndTime + postTunnelScreenTime)
	m.ctx.At(tunnelEndBeat, func() {
		if m.stopRight != nil {
			m.stopRight()
			m.stopRight = nil
		}
	})
	m.ctx.At(tunnelEndBeat+6.0/48.0, func() {
		if m.stopMiddle != nil {
			m.stopMiddle()
			m.stopMiddle = nil
		}
	})
	m.ctx.At(tunnelEndBeat+12.0/48.0, func() {
		if m.stopLeft != nil {
			m.stopLeft()
			m.stopLeft = nil
		}
	})

	tintBeat := m.ctx.TimeToBeat(m.tunnelStartTime + postTunnelScreenTime)
	m.ctx.At(tintBeat, func() {
		m.ctx.Scene.SetMaterialOver(m.wallRendererPath, m.tunnelTint, m.tunnelScreen)
	})
}

func tunnelWallWidth(startTime, endTime, chunksPerSec, chunkSize float64) float64 {
	durationSec := math.Ceil((endTime-startTime)*4*chunksPerSec) * 0.25 / chunksPerSec
	return durationSec * chunkSize * chunksPerSec
}

func (m *Module) stopTunnelLoops() {
	for _, stop := range []func(){m.stopRight, m.stopMiddle, m.stopLeft} {
		if stop != nil {
			stop()
		}
	}
	m.stopRight, m.stopMiddle, m.stopLeft = nil, nil, nil
}

func (m *Module) setBg(typ int, beat float64) {
	if typ < 0 || typ >= len(m.bgPaths) {
		typ = 0
	}
	for i, p := range m.bgPaths {
		m.ctx.Scene.SetActive(p, i == typ)
	}
	m.playBgLoops(typ, beat)
}

func (m *Module) bgAt(beat float64) int {
	typ := 0
	for _, ev := range m.bgs {
		if ev.beat > beat {
			break
		}
		typ = ev.typ
	}
	if typ < 0 || typ >= len(m.bgPaths) {
		return 0
	}
	return typ
}

func (m *Module) playBgLoops(typ int, beat float64) {
	if typ < 0 || typ >= len(m.bgPaths) || typ >= len(bgLoops) {
		return
	}
	root := m.bgPaths[typ]
	loops := bgLoops[typ]
	if loops.near != "" {
		m.ctx.Scene.PlayLayer(root+":near", root, "Animation/"+loops.near, beat, m.ctx.SecPerBeat(beat))
	}
	if loops.far != "" {
		m.ctx.Scene.PlayLayer(root+":far", root, "Animation/"+loops.far, beat, m.ctx.SecPerBeat(beat))
	}
}

func easeOutQuad(t float64) float64 {
	if t <= 0 {
		return 0
	}
	if t >= 1 {
		return 1
	}
	return 1 - (1-t)*(1-t)
}
