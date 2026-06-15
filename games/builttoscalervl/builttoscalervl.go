// Package builttoscalervl ports Built to Scale (Wii): rod spawn/bounce/shoot
// planning, player block state, square assembly cues, presence slides, and all
// prefab Animator states extracted from Heaven Studio.
package builttoscalervl

import (
	"image/color"
	"math"
	"sort"
	"strconv"

	"github.com/hajimehoshi/ebiten/v2"

	"hsdemo/engine"
	"hsdemo/kart"
	"hsdemo/kmdata"
	"hsdemo/riq"
)

var bgColor = color.NRGBA{R: 0x1a, G: 0xd2, B: 0x1a, A: 0xff}

type spawnEvent struct {
	beat, length        float64
	currentPos, nextPos int
	id                  int
}

type shootEvent struct {
	beat float64
	id   int
	mute bool
}

type bounceEvent struct {
	beat float64
	id   int
	pos  int
}

type outEvent struct {
	beat float64
	id   int
}

type Module struct {
	ctx  *engine.Ctx
	proj kart.Aff

	rodT, leftSquareT, rightSquareT, assembledT *kart.Template
	widgetPath                                  string
	blocks                                      [4]*blockState

	curves map[string]kmdata.Curve

	missAngle, fallingAngle                     float64
	leftSquareAnim, rightSquareAnim             string
	leftSquareCorrection, rightSquareCorrection [2]float64

	spawns   []spawnEvent
	shoots   []shootEvent
	bounces  []bounceEvent
	outs     []outEvent
	presence []presenceEvent

	gameStartBeat float64
	gameEndBeat   float64
	widgets       []scheduledWidget
	widgetIndex   int

	rods      []*rod
	squares   []*square
	assembled []*assembled
	nowBeat   float64
}

func New() engine.Module {
	return &Module{gameEndBeat: math.Inf(1)}
}

func (m *Module) ID() string { return "builtToScaleRvl" }

func (m *Module) Load(ctx *engine.Ctx) error {
	m.ctx = ctx
	if err := ctx.LoadAssets("builtToScaleRvl"); err != nil {
		return err
	}
	m.proj = kart.Translate(engine.ScreenW/2, engine.ScreenH/2).Mul(kart.Scale(54, -54))
	m.curves = ctx.Assets.Extra.Curves
	m.widgetPath = ctx.Role("widgetHolder")

	m.rodT = kart.NewTemplate(ctx.Assets, ctx.Role("baseRod"))
	m.leftSquareT = kart.NewTemplate(ctx.Assets, ctx.Role("baseLeftSquare"))
	m.rightSquareT = kart.NewTemplate(ctx.Assets, ctx.Role("baseRightSquare"))
	m.assembledT = kart.NewTemplate(ctx.Assets, ctx.Role("baseAssembled"))

	m.loadBlocks()
	m.loadRodParams()
	m.loadSquareParams()
	for i := 0; i < 4; i++ {
		if b := m.blocks[i]; b != nil {
			ctx.Scene.PlayDefaultState(b.path, 0, ctx.SecPerBeat(0))
		}
	}
	return nil
}

func (m *Module) loadBlocks() {
	for _, c := range m.ctx.Assets.Extra.Components {
		if c.Path == "" || c.Nums == nil {
			continue
		}
		pos, ok := c.Nums["position"]
		if !ok {
			continue
		}
		idx := int(pos)
		if !inRange(idx) {
			continue
		}
		nodeIdx, ok := m.ctx.Assets.NodeIndex(c.Path)
		if !ok {
			continue
		}
		node := m.ctx.Assets.Rig.Nodes[nodeIdx]
		m.blocks[idx] = &blockState{
			path: c.Path, base: node.Pos,
			slideOffset: c.Nums["_slideOffset"],
			closeBeat:   math.Inf(-1),
			shootBeat:   math.Inf(-1),
		}
	}
}

func (m *Module) loadRodParams() {
	c := componentByPath(m.ctx.Assets.Extra.Components, m.ctx.Role("baseRod"))
	m.missAngle = c.Nums["missAngle"]
	m.fallingAngle = c.Nums["fallingAngle"]
	if m.fallingAngle == 0 {
		m.fallingAngle = 45
	}
}

func (m *Module) loadSquareParams() {
	left := componentByPath(m.ctx.Assets.Extra.Components, m.ctx.Role("baseLeftSquare"))
	right := componentByPath(m.ctx.Assets.Extra.Components, m.ctx.Role("baseRightSquare"))
	m.leftSquareAnim = left.Strs["anim"]
	m.rightSquareAnim = right.Strs["anim"]
	if m.leftSquareAnim == "" {
		m.leftSquareAnim = "left"
	}
	if m.rightSquareAnim == "" {
		m.rightSquareAnim = "right"
	}
	m.leftSquareCorrection = [2]float64{left.Nums["CorrectionPos.x"], left.Nums["CorrectionPos.y"]}
	m.rightSquareCorrection = [2]float64{right.Nums["CorrectionPos.x"], right.Nums["CorrectionPos.y"]}
}

func (m *Module) OnEvent(e *riq.Entity) {
	switch e.Datamodel {
	case "builtToScaleRvl/spawn rod":
		dir := int(e.Float("direction", dirLeft))
		cur, next := -1, 0
		if dir == dirRight {
			cur, next = 4, 3
		}
		m.spawns = append(m.spawns, spawnEvent{
			beat: e.Beat, length: e.Length, currentPos: cur, nextPos: next,
			id: int(e.Float("id", 1)),
		})
	case "builtToScaleRvl/custom spawn":
		dir := int(e.Float("direction", dirLeft))
		cur := -1
		if dir == dirRight {
			cur = 4
		}
		m.spawns = append(m.spawns, spawnEvent{
			beat: e.Beat, length: e.Length, currentPos: cur,
			nextPos: blockTargetToPos(int(e.Float("target", targetFirst))),
			id:      int(e.Float("id", 1)),
		})
	case "builtToScaleRvl/custom bounce":
		m.bounces = append(m.bounces, bounceEvent{
			beat: e.Beat, id: int(e.Float("id", 1)),
			pos: targetToPos(int(e.Float("target", targetFirst))),
		})
	case "builtToScaleRvl/out sides":
		m.outs = append(m.outs, outEvent{beat: e.Beat, id: int(e.Float("id", 1))})
	case "builtToScaleRvl/shoot rod":
		m.shoots = append(m.shoots, shootEvent{
			beat: e.Beat, id: int(e.Float("id", 1)), mute: boolParam(e, "mute"),
		})
	case "builtToScaleRvl/presence":
		m.presence = append(m.presence, presenceEvent{
			beat: e.Beat, length: e.Length, in: boolParam(e, "in"),
			ease: int(e.Float("ease", 0)),
			blocks: [4]bool{
				boolParam(e, "first"), boolParam(e, "second"),
				boolParam(e, "third"), boolParam(e, "fourth"),
			},
		})
	}
}

func (m *Module) Ready() {
	sort.SliceStable(m.spawns, func(i, j int) bool { return m.spawns[i].beat < m.spawns[j].beat })
	sort.SliceStable(m.shoots, func(i, j int) bool { return m.shoots[i].beat < m.shoots[j].beat })
	sort.SliceStable(m.bounces, func(i, j int) bool { return m.bounces[i].beat < m.bounces[j].beat })
	sort.SliceStable(m.outs, func(i, j int) bool { return m.outs[i].beat < m.outs[j].beat })
	sort.SliceStable(m.presence, func(i, j int) bool { return m.presence[i].beat < m.presence[j].beat })
}

func (m *Module) OnSwitch(beat float64) {
	m.gameStartBeat = beat
	m.gameEndBeat = m.ctx.NextSwitchBeat(beat)
	m.widgets = m.widgets[:0]
	m.widgetIndex = 0
	m.rods = nil
	m.squares = nil
	m.assembled = nil

	for _, ev := range m.spawns {
		if ev.length == 0 || ev.beat+ev.length < m.gameStartBeat || ev.beat >= m.gameEndBeat {
			continue
		}
		bounces := m.calcRodBounce(ev.beat, ev.length, ev.id)
		m.addBounceOutSides(ev.beat, ev.length, ev.currentPos, ev.nextPos, ev.id, &bounces)
		endTime, isShoot, mute := m.calcRodEndTime(ev.beat, ev.length, ev.currentPos, ev.nextPos, ev.id, &bounces)
		m.widgets = append(m.widgets, scheduledWidget{
			beat: ev.beat, length: ev.length, currentPos: ev.currentPos, nextPos: ev.nextPos,
			id: ev.id, bounceItems: bounces, endTime: endTime, isShoot: isShoot, mute: mute,
		})
	}
	sort.SliceStable(m.widgets, func(i, j int) bool { return m.widgets[i].beat < m.widgets[j].beat })
	for i := range m.blocks {
		m.playBlockIdle(i, beat)
	}
}

func (m *Module) Whiff(beat float64) { m.WhiffAction(beat, actionPrimary) }

func (m *Module) WhiffAction(beat float64, action int) {
	switch action {
	case actionAlt:
		m.playBlockShootMiss(2)
	default:
		m.playBlockOpen(2)
	}
}

func (m *Module) Update(t, beat float64) {
	m.nowBeat = beat
	m.updateWidgets(beat)
	m.applyPresence(beat)
	for _, r := range m.rods {
		r.update(beat)
	}
	for _, s := range m.squares {
		s.update(m, beat)
	}
	for _, a := range m.assembled {
		a.update(beat)
	}
	m.pruneDead()
}

func (m *Module) Draw(screen *ebiten.Image, t, beat float64) {
	screen.Fill(bgColor)
	sc := m.ctx.Scene
	m.ctx.SampleScene(beat)
	widgetWorld := kart.Identity()
	if w, ok := sc.NodeWorld(m.widgetPath); ok {
		widgetWorld = w
	}
	for _, s := range m.squares {
		if !s.dead && beat >= s.firstBeat {
			s.inst.Queue(sc, beat, widgetWorld, 0)
		}
	}
	for _, r := range m.rods {
		if r.active && !r.dead {
			r.inst.Queue(sc, beat, widgetWorld, 0)
		}
	}
	for _, a := range m.assembled {
		if !a.dead {
			a.inst.Queue(sc, beat, widgetWorld, 0)
		}
	}
	sc.Draw(screen, m.proj)
}

func (m *Module) updateWidgets(beat float64) {
	for m.widgetIndex < len(m.widgets) {
		w := m.widgets[m.widgetIndex]
		if w.beat >= beat+widgetSeekBeats {
			break
		}
		m.spawnRod(w)
		m.widgetIndex++
	}
}

func (m *Module) calcRodBounce(beat, length float64, id int) []customBounceItem {
	var out []customBounceItem
	for _, ev := range m.bounces {
		if ev.beat <= beat || ev.id != id {
			continue
		}
		out = append(out, customBounceItem{
			time: intCeil((ev.beat - beat) / length),
			pos:  ev.pos,
		})
	}
	return out
}

func (m *Module) addBounceOutSides(beat, length float64, currentPos, nextPos, id int, bounces *[]customBounceItem) {
	var first *outEvent
	for i := range m.outs {
		if m.outs[i].beat >= beat+length && m.outs[i].id == id {
			first = &m.outs[i]
			break
		}
	}
	if first == nil {
		return
	}
	earliest := intCeil((first.beat - beat) / length)
	current, next := currentPos, nextPos
	fixed := append([]customBounceItem{}, (*bounces)...)
	for time := 0; time < 1024; time++ {
		if (current == 0 || current == 3) && time >= earliest {
			pos := -1
			if current == 3 {
				pos = 4
			}
			*bounces = append(*bounces, customBounceItem{time: time, pos: pos})
			return
		}
		following := followingPos(current, next, time+1, fixed)
		current, next = next, following
	}
}

func (m *Module) calcRodEndTime(beat, length float64, currentPos, nextPos, id int, bounces *[]customBounceItem) (int, bool, bool) {
	isShoot, mute := false, false
	earliest := int(^uint(0) >> 1)
	for _, ev := range m.shoots {
		if ev.beat >= beat+length && ev.id == id {
			earliest = intCeil((ev.beat - beat) / length)
			isShoot = true
			mute = ev.mute
			break
		}
	}
	filtered := (*bounces)[:0]
	for _, b := range *bounces {
		if b.time < earliest {
			filtered = append(filtered, b)
		}
	}
	*bounces = filtered
	sort.SliceStable(*bounces, func(i, j int) bool { return (*bounces)[i].time < (*bounces)[j].time })
	for _, b := range *bounces {
		if b.pos == -1 || b.pos == 4 {
			return b.time, false, mute
		}
	}
	if !isShoot {
		return earliest, false, mute
	}

	current, next := currentPos, nextPos
	fixed := append([]customBounceItem{}, (*bounces)...)
	for time := 0; time < 1024; time++ {
		if current == 2 && time >= earliest {
			return time, true, mute
		}
		following := followingPos(current, next, time+1, fixed)
		current, next = next, following
	}
	return earliest, true, mute
}

func (m *Module) curve(idx int) kmdata.Curve {
	return m.curves["game.curve"+strconv.Itoa(idx)]
}

func (m *Module) missCurve(idx int) kmdata.Curve {
	return m.curves["game.missCurve"+strconv.Itoa(idx)]
}

func (m *Module) at(beat float64, fn func()) {
	current := m.nowBeat
	if m.ctx != nil && m.ctx.App != nil {
		current = m.ctx.Beat()
	}
	if beat <= current+1e-6 {
		fn()
		return
	}
	m.ctx.At(beat, fn)
}

func (m *Module) soundAt(beat float64, name string, vol float64) {
	current := m.nowBeat
	if m.ctx != nil && m.ctx.App != nil {
		current = m.ctx.Beat()
	}
	if beat <= current+1e-6 {
		m.ctx.SoundVol(name, vol)
		return
	}
	m.ctx.SoundAt(beat, name, vol)
}

func (m *Module) pruneDead() {
	rods := m.rods[:0]
	for _, r := range m.rods {
		if !r.dead {
			rods = append(rods, r)
		}
	}
	m.rods = rods

	squares := m.squares[:0]
	for _, s := range m.squares {
		if !s.dead {
			squares = append(squares, s)
		}
	}
	m.squares = squares

	assembled := m.assembled[:0]
	for _, a := range m.assembled {
		if !a.dead {
			assembled = append(assembled, a)
		}
	}
	m.assembled = assembled
}
