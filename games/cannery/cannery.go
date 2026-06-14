// Package cannery ports The Cannery's conveyor scroll, alarm bop/color,
// blackout toggle, and two-layer can hit animations.
package cannery

import (
	"math"
	"math/rand"
	"sort"

	"github.com/hajimehoshi/ebiten/v2"

	"hsdemo/engine"
	"hsdemo/kart"
	"hsdemo/riq"
)

var (
	alarmColor   = [4]float64{0.8627, 0.3725, 0.0313, 1}
	alarmFill    = [4]float64{0.49803922, 0.49803922, 0.5058824, 1}
	alarmOutline = [4]float64{0.24705882, 0.24313726, 0.24705882, 1}
)

type bopEvt struct {
	beat, length float64
	auto, bop    bool
}

type speedEvt struct {
	beat, length float64
	from, to     float64
	ease         int
}

type colorEvt struct {
	beat, length float64
	from, to     [4]float64
	ease         int
}

type canEvt struct {
	beat float64
}

type canInst struct {
	start float64
	inst  *kart.Instance
	flip  bool
}

type Module struct {
	ctx  *engine.Ctx
	proj kart.Aff

	canRoot, blackout, conveyor, alarm, ding, canner string
	bgAnims                                          []string
	canT                                             *kart.Template

	bops      []bopEvt
	cans      []canEvt
	speedEvts []speedEvt
	colorEvts []colorEvt
	blackouts []float64

	liveCans []*canInst

	alarmBop   bool
	blackoutOn bool
	bgTime     float64
	speed      speedEvt
	color      colorEvt
	lastT      float64
	hasLastT   bool
	lastBeat   int
	rng        *rand.Rand
}

func New() engine.Module { return &Module{} }

func (m *Module) ID() string { return "cannery" }

func (m *Module) Load(ctx *engine.Ctx) error {
	m.ctx = ctx
	if err := ctx.LoadAssets("cannery"); err != nil {
		return err
	}
	m.proj = kart.Translate(engine.ScreenW/2, engine.ScreenH/2).Mul(kart.Scale(54, -54))
	m.canRoot = roleOr(ctx, "can", "CanParent")
	m.blackout = roleOr(ctx, "blackout", "Blackout")
	m.conveyor = roleOr(ctx, "conveyorBeltAnim", "ConveyorBelt")
	m.alarm = roleOr(ctx, "alarmAnim", "Alarm")
	m.ding = roleOr(ctx, "dingAnim", "AlarmFlash")
	m.canner = roleOr(ctx, "cannerAnim", "CannerParent/Canner")
	m.bgAnims = ctx.Assets.Extra.RefArrays["bgAnims"]
	m.canT = kart.NewTemplate(ctx.Assets, m.canRoot)
	m.rng = rand.New(rand.NewSource(1))
	m.reset(0)
	return nil
}

func roleOr(ctx *engine.Ctx, key, fallback string) string {
	if p := ctx.Role(key); p != "" {
		return p
	}
	return fallback
}

func (m *Module) OnEvent(e *riq.Entity) {
	switch e.Datamodel {
	case "cannery/bop":
		ev := bopEvt{
			beat: e.Beat, length: e.Length,
			auto: boolParam(e, "auto"),
			bop:  boolDefault(e, "toggle", true),
		}
		m.bops = append(m.bops, ev)
		m.ctx.At(ev.beat, func() { m.alarmBop = ev.auto })
		if ev.bop {
			for i := 0; float64(i) < ev.length; i++ {
				b := ev.beat + float64(i)
				m.ctx.At(b, func() { m.playAlarmBop(b) })
			}
		}
	case "cannery/can":
		ev := canEvt{beat: e.Beat}
		m.cans = append(m.cans, ev)
		m.ctx.SoundAt(ev.beat, "ding", 1)
		m.ctx.At(ev.beat, func() {
			if m.ctx.GameAt(ev.beat) == m.ID() {
				m.sendCan(ev.beat)
			}
		})
	case "cannery/blackout":
		b := e.Beat
		m.blackouts = append(m.blackouts, b)
		m.ctx.At(b, func() { m.setBlackout(!m.blackoutOn) })
	case "cannery/backgroundModifiers":
		m.speedEvts = append(m.speedEvts, speedEvt{
			beat: e.Beat, length: e.Length,
			from: e.Float("startSpeed", 10),
			to:   e.Float("endSpeed", 10),
			ease: intParam(e, "ease", 0),
		})
	case "cannery/alarmColor":
		m.colorEvts = append(m.colorEvts, colorEvt{
			beat: e.Beat, length: e.Length,
			from: colorParam(e, "startColor", alarmColor),
			to:   colorParam(e, "endColor", alarmColor),
			ease: intParam(e, "ease", 0),
		})
	}
}

func (m *Module) Ready() {
	sort.Slice(m.bops, func(i, j int) bool { return m.bops[i].beat < m.bops[j].beat })
	sort.Slice(m.cans, func(i, j int) bool { return m.cans[i].beat < m.cans[j].beat })
	sort.Slice(m.speedEvts, func(i, j int) bool { return m.speedEvts[i].beat < m.speedEvts[j].beat })
	sort.Slice(m.colorEvts, func(i, j int) bool { return m.colorEvts[i].beat < m.colorEvts[j].beat })
	sort.Float64s(m.blackouts)
}

func (m *Module) OnSwitch(beat float64) {
	m.reset(beat)
	for _, ev := range m.bops {
		if ev.beat >= beat {
			break
		}
		m.alarmBop = ev.auto
	}
	toggles := 0
	for _, b := range m.blackouts {
		if b >= beat {
			break
		}
		toggles++
	}
	m.setBlackout(toggles%2 == 1)
	m.persistSpeed(beat)
	m.persistColor(beat)
	for _, ev := range m.cans {
		if beat <= ev.beat-2 {
			continue
		}
		if beat >= ev.beat+1 {
			break
		}
		m.sendCan(ev.beat)
	}
}

func (m *Module) Whiff(beat float64) {
	m.ctx.Scene.PlayState(m.canner, "CanEmpty", beat, 0.5)
	m.ctx.PlayCommon("nearMiss")
}

func (m *Module) Update(t, beat float64) {
	if m.hasLastT && t > m.lastT {
		m.persistSpeed(beat)
		m.bgTime = math.Mod(m.bgTime+(t-m.lastT)*(m.speedAt(beat)/10), 1)
	}
	m.lastT, m.hasLastT = t, true
	whole := int(math.Floor(beat))
	if whole > m.lastBeat {
		for b := m.lastBeat + 1; b <= whole; b++ {
			if m.alarmBop {
				m.playAlarmBop(float64(b))
			}
		}
		m.lastBeat = whole
	}
}

func (m *Module) Draw(screen *ebiten.Image, _ float64, beat float64) {
	m.applyAlarmColor(beat)
	m.ctx.Scene.PlayNormalized(m.conveyor, "ConveyorBelt/ConveyorBeltMove", math.Mod(beat/2, 1))
	for _, bg := range m.bgAnims {
		m.ctx.Scene.PlayNormalized(bg, "BG/ClawConveyorScroll", m.bgTime)
	}
	m.ctx.SampleScene(beat)
	alive := m.liveCans[:0]
	for _, c := range m.liveCans {
		if beat <= c.start+2 {
			u := clamp01((beat - c.start) / 2)
			c.inst.PlayNormalized("", "Can/CanMove", u)
			if c.flip {
				c.inst.SetScale("", -1, 1)
			}
			c.inst.Queue(m.ctx.Scene, beat, kart.Identity(), 0)
			alive = append(alive, c)
		}
	}
	m.liveCans = alive
	m.ctx.Scene.Draw(screen, m.proj)
}

func (m *Module) reset(beat float64) {
	m.liveCans = nil
	m.alarmBop = true
	m.blackoutOn = false
	m.bgTime = 0
	m.speed = speedEvt{from: 10, to: 10}
	m.color = colorEvt{from: alarmColor, to: alarmColor}
	m.hasLastT = false
	m.lastBeat = int(math.Floor(beat)) - 1
	sec := m.ctx.SecPerBeat(beat)
	for _, root := range []string{m.alarm, m.ding, m.canner, m.conveyor} {
		m.ctx.Scene.PlayDefaultState(root, beat, sec)
	}
	for _, bg := range m.bgAnims {
		m.ctx.Scene.PlayDefaultState(bg, beat, sec)
	}
	m.ctx.Scene.SetActive(m.canRoot, false)
	m.ctx.Scene.SetActive(m.blackout, false)
	m.applyAlarmColor(beat)
}

func (m *Module) sendCan(beat float64) {
	if m.canT == nil || m.hasCan(beat) {
		return
	}
	m.atOrNow(beat, func() { m.ctx.Scene.PlayState(m.ding, "Ding", beat, 0.5) })
	c := &canInst{start: beat, inst: m.canT.NewInstance(), flip: m.rng.Float64() >= 0.5}
	c.inst.PlayDefaultState("", beat, m.ctx.SecPerBeat(beat))
	if c.flip {
		c.inst.SetScale("", -1, 1)
	}
	m.liveCans = append(m.liveCans, c)
	target := beat + 1
	m.ctx.ScheduleInput(target, func(state float64, _ engine.Judgment) {
		m.hitCan(c, state, target)
	}, nil)
}

func (m *Module) atOrNow(beat float64, fn func()) {
	if m.ctx.Beat() >= beat {
		fn()
		return
	}
	m.ctx.At(beat, fn)
}

func (m *Module) hasCan(beat float64) bool {
	for _, c := range m.liveCans {
		if math.Abs(c.start-beat) < 1e-6 {
			return true
		}
	}
	return false
}

func (m *Module) hitCan(c *canInst, state, target float64) {
	hitBeat := m.ctx.Beat()
	m.ctx.Sound("can")
	c.inst.PlayStateLayer("can:hit", "", "Can", hitBeat, 0.5)
	if state >= 1 || state <= -1 {
		m.ctx.PlayCommon("miss")
		m.ctx.Scene.PlayState(m.canner, "CanBarely", hitBeat, 0.5)
		m.ctx.At(target+0.35, func() {
			c.inst.PlayStateLayer("can:hit", "", "Reopen", target+0.35, 0.5)
		})
		return
	}
	m.ctx.Scene.PlayState(m.canner, "Can", hitBeat, 0.5)
}

func (m *Module) setBlackout(on bool) {
	m.blackoutOn = on
	m.ctx.Scene.SetActive(m.blackout, on)
}

func (m *Module) playAlarmBop(beat float64) {
	m.ctx.Scene.PlayState(m.alarm, "Bop", beat, 0.5)
}

func (m *Module) persistSpeed(beat float64) {
	m.speed = speedEvt{from: 10, to: 10}
	for _, ev := range m.speedEvts {
		if ev.beat >= beat {
			break
		}
		m.speed = ev
	}
}

func (m *Module) persistColor(beat float64) {
	m.color = colorEvt{from: alarmColor, to: alarmColor}
	for _, ev := range m.colorEvts {
		if ev.beat >= beat {
			break
		}
		m.color = ev
	}
}

func (m *Module) speedAt(beat float64) float64 {
	if m.speed.length <= 0 {
		return m.speed.to
	}
	u := clamp01((beat - m.speed.beat) / m.speed.length)
	return engine.Ease(m.speed.ease, m.speed.from, m.speed.to, u)
}

func (m *Module) applyAlarmColor(beat float64) {
	m.persistColor(beat + 1e-9)
	c := m.colorAt(beat)
	m.ctx.Scene.SetPaletteFor("AlarmMat", kart.Palette{
		Alpha: c, Fill: alarmFill, Outline: alarmOutline,
	})
}

func (m *Module) colorAt(beat float64) [4]float64 {
	if m.color.length <= 0 {
		c := m.color.to
		c[3] = 1
		return c
	}
	u := clamp01((beat - m.color.beat) / m.color.length)
	return [4]float64{
		engine.Ease(m.color.ease, m.color.from[0], m.color.to[0], u),
		engine.Ease(m.color.ease, m.color.from[1], m.color.to[1], u),
		engine.Ease(m.color.ease, m.color.from[2], m.color.to[2], u),
		1,
	}
}

func boolParam(e *riq.Entity, key string) bool { return e.Float(key, 0) != 0 }

func boolDefault(e *riq.Entity, key string, def bool) bool {
	if _, ok := e.Data[key]; !ok {
		return def
	}
	return boolParam(e, key)
}

func intParam(e *riq.Entity, key string, def int) int { return int(e.Float(key, float64(def))) }

func colorParam(e *riq.Entity, key string, def [4]float64) [4]float64 {
	v, ok := e.Data[key]
	if !ok {
		return def
	}
	m, ok := v.(map[string]any)
	if !ok {
		return def
	}
	return [4]float64{num(m["r"], def[0]), num(m["g"], def[1]), num(m["b"], def[2]), num(m["a"], def[3])}
}

func num(v any, def float64) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	default:
		return def
	}
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
