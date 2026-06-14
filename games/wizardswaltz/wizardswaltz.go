// Package wizardswaltz ports Wizard's Waltz's interval/plant/pass-turn flow.
package wizardswaltz

import (
	"image/color"
	"math"
	"sort"

	"github.com/hajimehoshi/ebiten/v2"

	"hsdemo/engine"
	"hsdemo/kart"
	"hsdemo/riq"
)

var bgColor = color.NRGBA{R: 0xff, G: 0xef, B: 0x9c, A: 0xff}

type interval struct {
	beat, length float64
	auto         bool
}

type plantEvt struct {
	beat float64
}

type passEvt struct {
	beat float64
}

type plant struct {
	inst       *kart.Instance
	createBeat float64
	interval   float64
	z          float64
	animSeq    int
	dead       bool
}

type Module struct {
	ctx  *engine.Ctx
	proj kart.Aff

	intervals []interval
	plantsRaw []plantEvt
	passes    []passEvt

	plantT *kart.Template
	plants []*plant

	wizardPath string
	shadowPath string
	girlPath   string
	holderPath string
	flowerPath []string

	beatInterval     float64
	wizardBeatOffset float64
	xRange           float64
	zRange           float64
	yRange           float64
	plantYOffset     float64

	flowerCount int
	magicSeq    int
}

func New() engine.Module {
	return &Module{
		beatInterval: 6,
		xRange:       6, zRange: 3.5, yRange: 0.5, plantYOffset: -2,
	}
}

func (m *Module) ID() string { return "wizardsWaltz" }

func (m *Module) Load(ctx *engine.Ctx) error {
	m.ctx = ctx
	if err := ctx.LoadAssets("wizardsWaltz"); err != nil {
		return err
	}
	m.proj = kart.Translate(engine.ScreenW/2, engine.ScreenH/2).Mul(kart.Scale(54, -54))

	m.wizardPath = roleOr(ctx, "wizard", "Wizard")
	m.girlPath = roleOr(ctx, "girl", "Girl")
	m.holderPath = roleOr(ctx, "plantHolder", "Plants")
	if comp := ctx.Assets.Extra.Components; comp != nil {
		if g := comp["game"]; g.Nums != nil {
			m.xRange = numDefault(g.Nums, "xRange", m.xRange)
			m.zRange = numDefault(g.Nums, "zRange", m.zRange)
			m.yRange = numDefault(g.Nums, "yRange", m.yRange)
			m.plantYOffset = numDefault(g.Nums, "plantYOffset", m.plantYOffset)
		}
		if w := comp["wizard"]; w.Refs != nil {
			m.shadowPath = w.Refs["shadow"]
		}
		if g := comp["girl"]; len(g.RefArrays["flowers"]) > 0 {
			m.flowerPath = append(m.flowerPath, g.RefArrays["flowers"]...)
		}
	}
	if m.shadowPath == "" {
		m.shadowPath = "WizardShadow"
	}
	if len(m.flowerPath) == 0 {
		for i := 1; i <= 6; i++ {
			m.flowerPath = append(m.flowerPath, "Girl/Flowers/Flower"+string(rune('0'+i)))
		}
	}

	m.plantT = kart.NewTemplate(ctx.Assets, roleOr(ctx, "plantBase", "Prefabs/Plant"))
	ctx.Scene.SetActive("Prefabs", false)
	m.applyFlowers()
	ctx.Scene.PlayState(m.wizardPath, "Idle", 0, ctx.SecPerBeat(0))
	ctx.Scene.PlayState(m.girlPath, "Idle", 0, ctx.SecPerBeat(0))
	return nil
}

func roleOr(ctx *engine.Ctx, key, fallback string) string {
	if p := ctx.Role(key); p != "" {
		return p
	}
	return fallback
}

func numDefault(nums map[string]float64, key string, def float64) float64 {
	if v, ok := nums[key]; ok {
		return v
	}
	return def
}

func (m *Module) OnEvent(e *riq.Entity) {
	switch e.Datamodel {
	case "wizardsWaltz/start interval":
		m.intervals = append(m.intervals, interval{beat: e.Beat, length: e.Length, auto: boolDefault(e, "auto", true)})
	case "wizardsWaltz/plant":
		m.plantsRaw = append(m.plantsRaw, plantEvt{beat: e.Beat})
	case "wizardsWaltz/passTurn":
		m.passes = append(m.passes, passEvt{beat: e.Beat})
	}
}

func (m *Module) Ready() {
	sort.Slice(m.intervals, func(i, j int) bool { return m.intervals[i].beat < m.intervals[j].beat })
	sort.Slice(m.plantsRaw, func(i, j int) bool { return m.plantsRaw[i].beat < m.plantsRaw[j].beat })
	for _, iv := range m.intervals {
		iv := iv
		m.ctx.At(iv.beat, func() {
			m.wizardBeatOffset = iv.beat
			m.beatInterval = iv.length
		})
		for _, pe := range m.plantsIn(iv.beat, iv.beat+iv.length) {
			pe := pe
			m.ctx.SoundAt(pe.beat, "plant", 1)
			m.ctx.At(pe.beat, func() { m.spawnPlant(pe.beat, iv, false) })
		}
		if iv.auto {
			m.passTurn(iv.beat+iv.length, iv)
		}
	}
	for _, p := range m.passes {
		if iv, ok := m.intervalForPass(p.beat); ok {
			m.passTurn(p.beat, iv)
		}
	}
}

func (m *Module) plantsIn(from, to float64) []plantEvt {
	var out []plantEvt
	for _, pe := range m.plantsRaw {
		if pe.beat >= from && pe.beat < to {
			out = append(out, pe)
		}
	}
	return out
}

func (m *Module) intervalForPass(beat float64) (interval, bool) {
	var found interval
	ok := false
	for _, iv := range m.intervals {
		if iv.beat <= beat {
			found, ok = iv, true
		}
	}
	return found, ok
}

func (m *Module) OnSwitch(beat float64) {
	m.setWizardOffset(beat)
	m.applyFlowers()
}

func (m *Module) setWizardOffset(beat float64) {
	if len(m.intervals) == 0 {
		return
	}
	for _, iv := range m.intervals {
		if beat >= iv.beat && beat < iv.beat+iv.length {
			m.wizardBeatOffset = iv.beat
			m.beatInterval = iv.length
			return
		}
	}
	for _, iv := range m.intervals {
		if iv.beat >= beat {
			m.wizardBeatOffset = iv.beat
			m.beatInterval = iv.length
			return
		}
	}
	last := m.intervals[len(m.intervals)-1]
	m.wizardBeatOffset = last.beat
	m.beatInterval = last.length
}

func (m *Module) Whiff(beat float64) {
	m.pressMagic(beat)
}

func (m *Module) Update(t, beat float64) {
	m.updateWizard(beat)
	dst := m.plants[:0]
	for _, p := range m.plants {
		if !p.dead && beat <= p.createBeat+p.interval*1.5 {
			dst = append(dst, p)
		}
	}
	m.plants = dst
}

func (m *Module) Draw(screen *ebiten.Image, t, beat float64) {
	screen.Fill(bgColor)
	sc := m.ctx.Scene
	m.ctx.SampleScene(beat)
	base := kart.Identity()
	if w, ok := sc.NodeWorld(m.holderPath); ok {
		base = w
	}
	for _, p := range m.plants {
		p.inst.Queue(sc, beat, base, p.z)
	}
	sc.Draw(screen, m.proj)
}

func (m *Module) passTurn(beat float64, iv interval) {
	ctx := m.ctx
	ctx.At(beat-0.25, func() {
		m.beatInterval = iv.length
		m.wizardBeatOffset = beat
		for _, p := range m.plants {
			m.placePlant(p, beat)
		}
	})
	for _, pe := range m.plantsIn(iv.beat, iv.beat+iv.length) {
		pe := pe
		judge := beat + (pe.beat - iv.beat)
		ctx.ScheduleInputAction(judge, 0,
			func(state float64, _ engine.Judgment) { m.hitPlant(pe.beat, state) },
			func() {})
	}
}

func (m *Module) spawnPlant(createBeat float64, iv interval, inactive bool) *plant {
	if p := m.findPlant(createBeat); p != nil {
		return p
	}
	if m.plantT == nil {
		return nil
	}
	p := &plant{inst: m.plantT.NewInstance(), createBeat: createBeat, interval: iv.length}
	m.placePlant(p, iv.beat)
	if inactive {
		p.inst.PlayState("", "IdlePlant", createBeat, m.ctx.SecPerBeat(createBeat))
	} else {
		p.playState(m, "Appear", createBeat)
		p.scheduleState(m, "IdlePlant", createBeat)
	}
	m.plants = append(m.plants, p)
	return p
}

func (m *Module) findPlant(createBeat float64) *plant {
	for _, p := range m.plants {
		if math.Abs(p.createBeat-createBeat) < 1e-6 {
			return p
		}
	}
	return nil
}

func (m *Module) placePlant(p *plant, offsetBeat float64) {
	x, y, z := m.flowerPos(p.createBeat, offsetBeat)
	p.inst.Offset = [2]float64{x, y}
	p.z = z
	p.inst.SetOrder("SpriteHolder", int(math.Round(-z)))
}

func (m *Module) flowerPos(beat, offsetBeat float64) (float64, float64, float64) {
	am := m.beatInterval / 2
	if am == 0 {
		am = 3
	}
	songPos := beat - offsetBeat
	c := math.Cos(math.Pi * songPos / am)
	x := math.Sin(math.Pi*songPos/am) * m.xRange
	y := m.plantYOffset + c*(m.yRange*1.5)
	z := c * m.zRange
	return x, y, z
}

func (m *Module) updateWizard(beat float64) {
	am := m.beatInterval / 2
	if am == 0 {
		am = 3
	}
	songPos := beat - m.wizardBeatOffset
	c := math.Cos(math.Pi * songPos / am)
	x := math.Sin(math.Pi*songPos/am) * m.xRange
	y := c * m.yRange
	z := c * m.zRange
	m.ctx.Scene.SetPosOver(m.wizardPath, x, 3-y*0.5)
	m.ctx.Scene.SetMirrorX(m.wizardPath, y > 0)
	m.ctx.Scene.SetZOver(m.wizardPath, z)
	m.ctx.Scene.SetPosOver(m.shadowPath, x, m.plantYOffset+y*1.5)
	m.ctx.Scene.SetZOver(m.shadowPath, z)
}

func (m *Module) hitPlant(createBeat, state float64) {
	beat := m.ctx.Beat()
	m.pressMagic(beat)
	p := m.findPlant(createBeat)
	if p == nil {
		return
	}
	if state >= 1 || state <= -1 {
		m.ctx.Sound("common_miss")
		p.playState(m, "Eat", beat)
		p.scheduleState(m, "EatLoop", beat)
		m.girlMood("Sad", -1, beat)
		return
	}
	m.ctx.Sound("grow")
	p.playState(m, "Hit", beat)
	p.scheduleState(m, "IdleFlower", beat)
	m.girlMood("Happy", 1, beat)
}

func (m *Module) pressMagic(beat float64) {
	m.magicSeq++
	seq := m.magicSeq
	m.ctx.Sound("wand")
	m.ctx.Scene.PlayState(m.wizardPath, "Magic", beat, m.ctx.SecPerBeat(beat))
	if d := m.stateBeats("WizardAnimator", "Magic", beat); d > 0 {
		m.ctx.At(beat+d, func() {
			if m.magicSeq == seq {
				m.ctx.Scene.PlayState(m.wizardPath, "Idle", beat+d, m.ctx.SecPerBeat(beat+d))
			}
		})
	}
}

func (p *plant) playState(m *Module, state string, beat float64) {
	p.animSeq++
	p.inst.PlayState("", state, beat, m.ctx.SecPerBeat(beat))
}

func (p *plant) scheduleState(m *Module, state string, beat float64) {
	seq := p.animSeq
	if d := m.stateBeats("PlantAnimator", p.inst.CurrentState(""), beat); d > 0 {
		m.ctx.At(beat+d, func() {
			if p.animSeq == seq {
				p.playState(m, state, beat+d)
			}
		})
	}
}

func (m *Module) stateBeats(ctrlName, stateName string, beat float64) float64 {
	ctrl, ok := m.ctx.Assets.Controllers[ctrlName]
	if !ok {
		return 0
	}
	st, ok := ctrl.States[stateName]
	if !ok || st.Clip == "" || st.Speed <= 0 {
		return 0
	}
	anim, ok := m.ctx.Assets.Anims[st.Clip]
	if !ok || anim.Duration <= 0 {
		return 0
	}
	return anim.Duration / (m.ctx.SecPerBeat(beat) * st.Speed)
}

func (m *Module) girlMood(state string, add int, beat float64) {
	m.ctx.Scene.PlayState(m.girlPath, state, beat, m.ctx.SecPerBeat(beat))
	m.setFlowers(add)
}

func (m *Module) setFlowers(add int) {
	m.flowerCount += add
	if m.flowerCount < 0 {
		m.flowerCount = 0
	}
	if m.flowerCount > len(m.flowerPath) {
		m.flowerCount = len(m.flowerPath)
	}
	m.applyFlowers()
}

func (m *Module) applyFlowers() {
	for i, p := range m.flowerPath {
		m.ctx.Scene.SetActive(p, i < m.flowerCount)
	}
}

func boolDefault(e *riq.Entity, key string, def bool) bool {
	if _, ok := e.Data[key]; !ok {
		return def
	}
	return e.Float(key, 0) != 0
}
