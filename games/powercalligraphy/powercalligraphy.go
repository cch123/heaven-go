// Package powercalligraphy ports Power Calligraphy's paper flow, brush
// feedback, chounin animations, and background color events.
//
// Unity logic reference:
// Assets/Scripts/Games/PowerCalligraphy/PowerCalligraphy.cs
// Assets/Scripts/Games/PowerCalligraphy/Writing.cs
// Assets/Scripts/Games/PowerCalligraphy/Fude.cs
package powercalligraphy

import (
	"fmt"
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
	charRe = iota
	charComma
	charChikara
	charOnore
	charSun
	charKokoro
	charFace
	charFaceKR
	charNone
)

const (
	actionBasic = 0
	actionFlick = 1
)

const (
	chouninDance = iota
	chouninBow
	chouninIdle
)

type bopEvt struct {
	beat, length float64
	bop, auto    bool
}

type paperEvt struct {
	beat float64
	typ  int
}

type colorEvt struct {
	beat, length float64
	from, to     [4]float64
	ease         int
}

type chouninKid struct {
	path string
	x, y float64
}

type Module struct {
	ctx  *engine.Ctx
	proj kart.Aff

	shiftRoot  string
	paperRoot  string
	endPaper   string
	bgPlane    string
	fudePos    string
	fudeAnim   string
	shiftAnim  string
	playerFude string

	paperDefs [charNone]paperDef
	templates [charNone]*kart.Template
	papers    []*paperInst
	nowPaper  *paperInst

	shiftWorld    kart.Aff
	haveShift     bool
	scrollSpeed   [2]float64
	chouninSpeed  float64
	chouninType   int
	chouninMoving bool
	chounin       [2][]chouninKid

	bops          []bopEvt
	paperEvents   []paperEvt
	colorEvents   []colorEvt
	endBeat       float64
	gameStartBeat float64
	isPrepare     bool
	lastBeat      float64
	haveLastBeat  bool

	fude fudeState
}

func New() engine.Module { return &Module{chouninType: -1, endBeat: 64} }

func (m *Module) ID() string { return "powerCalligraphy" }

func (m *Module) Load(ctx *engine.Ctx) error {
	m.ctx = ctx
	if err := ctx.LoadAssets("powerCalligraphy"); err != nil {
		return err
	}
	m.proj = kart.Translate(engine.ScreenW/2, engine.ScreenH/2).Mul(kart.Scale(54, -54))

	m.shiftRoot = ctx.Role("shiftHolder")
	m.paperRoot = ctx.Role("paperHolder")
	m.endPaper = ctx.Role("endPaper")
	m.bgPlane = ctx.Role("BGPlane")
	m.fudePos = ctx.Role("fudePosAnim")
	m.fudeAnim = ctx.Role("fudeAnim")
	m.shiftAnim = ctx.Role("shiftAnim")
	m.playerFude = ctx.Role("playerFude")
	game := ctx.Assets.Extra.Components["game"]
	m.scrollSpeed = [2]float64{numDefault(game.Nums, "scrollSpeed.x", 6), numDefault(game.Nums, "scrollSpeed.y", -10)}
	m.chouninSpeed = numDefault(game.Nums, "chouninSpeed", 0.6)
	if err := m.loadPapers(ctx.Assets); err != nil {
		return err
	}
	m.loadChounin(ctx.Assets)
	m.fude = newFudeState(ctx.Assets.Extra.Components["fude"])
	m.playFude("fude-none", 0, ctx.SecPerBeat(0))
	m.ctx.Scene.PlayState(m.fudePos, "fudePos-none", 0, ctx.SecPerBeat(0))
	m.ctx.Scene.PlayDefaultState(m.endPaper, 0, ctx.SecPerBeat(0))
	m.applyBG([4]float64{1, 1, 1, 1})
	return nil
}

func (m *Module) loadPapers(as *kart.Assets) error {
	roots := as.Extra.RefArrays["basePapers"]
	if len(roots) < charNone {
		return fmt.Errorf("basePapers extracted %d entries, want %d", len(roots), charNone)
	}
	for typ := 0; typ < charNone; typ++ {
		root := roots[typ]
		t := kart.NewTemplate(as, root)
		if t == nil {
			return fmt.Errorf("paper template %q missing", root)
		}
		m.templates[typ] = t
		def, ok := loadPaperDef(as, root, typ)
		if !ok {
			return fmt.Errorf("AnimPattern for %q missing", root)
		}
		m.paperDefs[typ] = def
	}
	return nil
}

func (m *Module) loadChounin(as *kart.Assets) {
	roots := as.Extra.RefArrays["Chounin"]
	for side := 0; side < len(roots) && side < 2; side++ {
		rootIdx := -1
		for i, n := range as.Rig.Nodes {
			if n.Path == roots[side] {
				rootIdx = i
				break
			}
		}
		if rootIdx < 0 {
			continue
		}
		for _, n := range as.Rig.Nodes {
			if n.Parent == rootIdx {
				m.chounin[side] = append(m.chounin[side], chouninKid{path: n.Path, x: n.Pos[0], y: n.Pos[1]})
			}
		}
	}
}

func (m *Module) OnEvent(e *riq.Entity) {
	b := e.Beat
	if end := b + math.Max(e.Length, 0); end > m.endBeat {
		m.endBeat = end
	}
	switch e.Datamodel {
	case "powerCalligraphy/bop":
		ev := bopEvt{beat: b, length: e.Length, bop: boolParamDefault(e, "bop", true), auto: boolParam(e, "bopAuto")}
		m.bops = append(m.bops, ev)
		if ev.bop {
			for i := 0.0; i < ev.length; i++ {
				bb := ev.beat + i
				m.ctx.At(bb, func() { m.bop(bb) })
			}
		}
	case "powerCalligraphy/re":
		m.addPaperEvent(b, charRe)
	case "powerCalligraphy/comma":
		m.addPaperEvent(b, charComma)
	case "powerCalligraphy/chikara":
		m.addPaperEvent(b, charChikara)
	case "powerCalligraphy/onore":
		m.addPaperEvent(b, charOnore)
	case "powerCalligraphy/sun":
		m.addPaperEvent(b, charSun)
	case "powerCalligraphy/kokoro":
		m.addPaperEvent(b, charKokoro)
	case "powerCalligraphy/face":
		typ := charFace
		if boolParam(e, "korean") {
			typ = charFaceKR
		}
		m.addPaperEvent(b, typ)
	case "powerCalligraphy/changeScrollSpeed":
		x, y := e.Float("x", 0), e.Float("y", 0)
		m.ctx.At(b, func() { m.changeScrollSpeed(x, y) })
	case "powerCalligraphy/chounin events":
		typ, pos := int(e.Float("type", 0)), e.Float("pos", 0)
		m.ctx.At(b, func() { m.playChouninAnimation(typ, pos) })
	case "powerCalligraphy/fade background":
		m.colorEvents = append(m.colorEvents, colorEvt{
			beat: b, length: e.Length,
			from: colorParam(e, "colorStart", [4]float64{1, 1, 1, 1}),
			to:   colorParam(e, "colorEnd", [4]float64{1, 1, 1, 1}),
			ease: int(e.Float("ease", 0)),
		})
	case "powerCalligraphy/end":
		m.ctx.At(b, func() { m.theEnd(b) })
	}
}

func (m *Module) addPaperEvent(beat float64, typ int) {
	m.paperEvents = append(m.paperEvents, paperEvt{beat: beat, typ: typ})
	m.ctx.At(beat, func() { m.write(beat, typ) })
}

func (m *Module) Ready() {
	sort.SliceStable(m.bops, func(i, j int) bool { return m.bops[i].beat < m.bops[j].beat })
	sort.SliceStable(m.paperEvents, func(i, j int) bool { return m.paperEvents[i].beat < m.paperEvents[j].beat })
	sort.SliceStable(m.colorEvents, func(i, j int) bool { return m.colorEvents[i].beat < m.colorEvents[j].beat })
	for b := 0.0; b <= m.endBeat+4; b++ {
		bb := b
		m.ctx.At(bb, func() {
			if m.inBopRegion(bb) {
				m.bop(bb)
			}
		})
	}
}

func (m *Module) OnSwitch(beat float64) {
	m.gameStartBeat = beat
	m.nextPrepare(beat)
	m.haveLastBeat = false
}

func (m *Module) Whiff(beat float64) { m.WhiffAction(beat, actionBasic) }

func (m *Module) WhiffAction(_ float64, action int) {
	if m.nowPaper == nil || !m.nowPaper.onGoing {
		return
	}
	switch {
	case action == actionBasic && m.nowPaper.stroke == strokeTome:
		m.nowPaper.processInput("fast")
		m.chouninMiss()
		m.ctx.ScoreMiss()
	case action == actionFlick && m.nowPaper.stroke != strokeTome:
		m.nowPaper.processInput("fast")
		m.chouninMiss()
		m.ctx.ScoreMiss()
	}
}

func (m *Module) Update(t, beat float64) {
	m.applyColors(beat)
	m.updateChouninMotion(beat)
	m.updatePapers(beat)
	m.updateFude(beat)
	m.ctx.SampleScene(beat)
	m.shiftWorld, m.haveShift = m.ctx.Scene.NodeWorld(m.shiftRoot)
	_ = t
}

func (m *Module) Draw(screen *ebiten.Image, _ float64, beat float64) {
	for _, p := range m.papers {
		p.queue(m.ctx.Scene, beat, m.activePaperWorld())
	}
	m.ctx.Scene.Draw(screen, m.proj)
}

func (m *Module) activePaperWorld() kart.Aff {
	if m.haveShift {
		return m.shiftWorld
	}
	return kart.Identity()
}

func (m *Module) spawnPaper(typ int, beat float64) {
	if typ < 0 || typ >= charNone {
		return
	}
	if m.nowPaper != nil && !m.nowPaper.finished {
		m.nowPaper.finishWorld = m.activePaperWorld()
		m.nowPaper.finished = true
	}
	inst := m.templates[typ].NewInstance()
	inst.SetGroupOrder(1)
	p := &paperInst{
		mod:         m,
		def:         m.paperDefs[typ],
		inst:        inst,
		scrollSpeed: m.scrollSpeed,
		finishWorld: kart.Identity(),
	}
	m.nowPaper = p
	m.papers = append(m.papers, p)
	m.playSceneVariant(m.fudePos, fudePosController(typ), "0", beat, 0.5)
	m.ctx.Scene.PlayState(m.shiftAnim, "shift-none", beat, m.ctx.SecPerBeat(beat))
	m.ctx.Scene.ClearPosOver(m.shiftRoot)
}

func (m *Module) prepare(typ int, beat float64) {
	if m.isPrepare {
		return
	}
	m.isPrepare = true
	m.spawnPaper(typ, beat)
}

func (m *Module) write(beat float64, typ int) {
	m.prepare(typ, beat)
	if m.nowPaper == nil {
		return
	}
	p := m.nowPaper
	p.startBeat = beat
	p.play()
	nextBeat := beat + p.def.nextBeat
	m.ctx.At(beat, func() { m.isPrepare = false })
	m.ctx.At(nextBeat, func() { m.nextPrepare(nextBeat) })
}

func (m *Module) nextPrepare(beat float64) {
	endBeat := m.ctx.NextSwitchBeat(m.gameStartBeat)
	for _, ev := range m.paperEvents {
		if ev.beat >= beat && ev.beat < endBeat {
			m.prepare(ev.typ, beat)
			return
		}
	}
}

func (m *Module) changeScrollSpeed(x, y float64) {
	m.scrollSpeed = [2]float64{x, y}
	if m.nowPaper != nil {
		m.nowPaper.scrollSpeed = m.scrollSpeed
	}
}

func (m *Module) theEnd(beat float64) {
	m.ctx.Scene.SetActive(m.endPaper, true)
	m.ctx.Scene.PlayState(m.fudePos, "fudePos-end", beat, 0.5)
	m.ctx.Scene.PlayState(m.endPaper, "paper-end", beat, 0.5)
}

func (m *Module) playSceneVariant(root, ctrlName, state string, beat, timeScale float64) {
	ctrl, ok := m.ctx.Assets.Controllers[ctrlName]
	if !ok {
		return
	}
	st, ok := ctrl.States[state]
	if !ok || st.Clip == "" || st.Speed*timeScale == 0 {
		return
	}
	m.ctx.Scene.Play(root, st.Clip, beat, timeScale*st.Speed)
}

func (m *Module) updatePapers(beat float64) {
	dBeat := 0.0
	if m.haveLastBeat {
		dBeat = beat - m.lastBeat
		if dBeat < 0 || dBeat > 4 {
			dBeat = 0
		}
	}
	m.lastBeat, m.haveLastBeat = beat, true
	for _, p := range m.papers {
		p.update(beat, dBeat)
	}
	live := m.papers[:0]
	for _, p := range m.papers {
		if !p.dead {
			live = append(live, p)
		}
	}
	m.papers = live
}

func (m *Module) inBopRegion(beat float64) bool {
	on := false
	for _, ev := range m.bops {
		if ev.beat > beat {
			break
		}
		on = ev.auto
	}
	return on
}

func (m *Module) bop(beat float64) {
	if m.chouninType != chouninDance {
		return
	}
	m.chouninMoving = true
	for side := 0; side < 2; side++ {
		for j, kid := range m.chounin[side] {
			state := "dance0"
			if int(math.Mod(beat, 2)) == j%2 {
				state = "dance1"
			}
			m.ctx.Scene.PlayState(kid.path, state, beat, 0.5)
		}
	}
}

func (m *Module) playChouninAnimation(typ int, pos float64) {
	m.chouninMoving = false
	m.chouninType = typ
	switch typ {
	case chouninDance:
		m.chouninMoving = true
		m.bop(m.ctx.Beat())
	case chouninBow:
		m.chouninAnim("bow", m.ctx.Beat())
	default:
		m.chouninAnim("idle", m.ctx.Beat())
	}
	if pos > 0 {
		m.updateChouninPos(pos)
	}
}

func (m *Module) chouninAnim(prefix string, beat float64) {
	for side := 0; side < 2; side++ {
		state := prefix + "0"
		if side%2 == 1 {
			state = prefix + "1"
		}
		for _, kid := range m.chounin[side] {
			m.ctx.Scene.PlayState(kid.path, state, beat, 0.5)
		}
	}
}

func (m *Module) chouninMiss() {
	m.chouninMoving = false
	beat := m.ctx.Beat()
	current := m.chouninType
	m.ctx.At(beat+1.5, func() {
		if m.chouninType == -1 {
			m.chouninType = current
		}
	})
	m.chouninType = -1
	m.chouninAnim("fall", beat)
}

func (m *Module) updateChouninMotion(beat float64) {
	if !m.chouninMoving || !m.haveLastBeat {
		return
	}
	dBeat := beat - m.lastBeat
	if dBeat <= 0 || dBeat > 4 {
		return
	}
	m.updateChouninPos(m.chouninSpeed * dBeat / 2)
}

func (m *Module) updateChouninPos(pos float64) {
	for i := range m.chounin[0] {
		k := &m.chounin[0][i]
		k.y -= pos
		if k.y < -6 {
			k.y += 12
		}
		m.ctx.Scene.SetPosOver(k.path, k.x, k.y)
	}
	for i := range m.chounin[1] {
		k := &m.chounin[1][i]
		k.y += pos
		if k.y > 6 {
			k.y -= 12
		}
		m.ctx.Scene.SetPosOver(k.path, k.x, k.y)
	}
}

func (m *Module) applyColors(beat float64) {
	c := [4]float64{1, 1, 1, 1}
	for _, ev := range m.colorEvents {
		if ev.beat > beat {
			break
		}
		u := 1.0
		if ev.length > 0 {
			u = (beat - ev.beat) / ev.length
		}
		c = [4]float64{
			engine.Ease(ev.ease, ev.from[0], ev.to[0], u),
			engine.Ease(ev.ease, ev.from[1], ev.to[1], u),
			engine.Ease(ev.ease, ev.from[2], ev.to[2], u),
			engine.Ease(ev.ease, ev.from[3], ev.to[3], u),
		}
	}
	m.applyBG(c)
}

func (m *Module) applyBG(c [4]float64) {
	if m.bgPlane != "" {
		m.ctx.Scene.SetColorOver(m.bgPlane, c)
	}
}

func paperController(typ int) string { return "paper-" + charName(typ) }
func shiftController(typ int) string { return "shift-" + charName(typ) }
func fudePosController(typ int) string {
	if typ == charNone {
		return "fudePos"
	}
	return "fudePos-" + charName(typ)
}

func charName(typ int) string {
	switch typ {
	case charRe:
		return "re"
	case charComma:
		return "comma"
	case charChikara:
		return "chikara"
	case charOnore:
		return "onore"
	case charSun:
		return "sun"
	case charKokoro:
		return "kokoro"
	case charFace:
		return "face"
	case charFaceKR:
		return "face_kr"
	default:
		return "none"
	}
}

func boolParam(e *riq.Entity, key string) bool { return boolParamDefault(e, key, false) }

func boolParamDefault(e *riq.Entity, key string, def bool) bool {
	if _, ok := e.Data[key]; !ok {
		return def
	}
	return e.Float(key, 0) != 0
}

func colorParam(e *riq.Entity, key string, def [4]float64) [4]float64 {
	v, ok := e.Data[key]
	if !ok {
		return def
	}
	switch c := v.(type) {
	case map[string]any:
		return [4]float64{numAny(c["r"], def[0]), numAny(c["g"], def[1]), numAny(c["b"], def[2]), numAny(c["a"], def[3])}
	case []any:
		if len(c) >= 4 {
			return [4]float64{numAny(c[0], def[0]), numAny(c[1], def[1]), numAny(c[2], def[2]), numAny(c[3], def[3])}
		}
	case string:
		if strings.HasPrefix(c, "#") && len(c) == 9 {
			var r, g, b, a uint8
			if _, err := fmt.Sscanf(c, "#%02x%02x%02x%02x", &r, &g, &b, &a); err == nil {
				return [4]float64{float64(r) / 255, float64(g) / 255, float64(b) / 255, float64(a) / 255}
			}
		}
	}
	return def
}

func numAny(v any, def float64) float64 {
	if f, ok := v.(float64); ok {
		return f
	}
	if i, ok := v.(int); ok {
		return float64(i)
	}
	return def
}

func numDefault(nums map[string]float64, key string, def float64) float64 {
	if v, ok := nums[key]; ok {
		return v
	}
	return def
}

func componentByPath(as *kart.Assets, prefix, path string) (kmdata.Component, bool) {
	for name, c := range as.Extra.Components {
		if strings.HasPrefix(name, prefix) && c.Path == path {
			return c, true
		}
	}
	return kmdata.Component{}, false
}
