// Package nailcarpenter ports Nail Carpenter's pattern-driven nail board,
// shoji slide, weak/strong hammer inputs, sweet punish windows, and cue sounds.
package nailcarpenter

import (
	"math"
	"sort"

	"github.com/hajimehoshi/ebiten/v2"

	"hsdemo/engine"
	"hsdemo/kart"
	"hsdemo/riq"
)

const (
	patternSeekTime = 8.0

	objectNail = iota
	objectLongNail
	objectSweet
	objectForceCherry
	objectForcePudding
	objectForceCherryPudding
	objectForceShortCake
	objectForceLayerCake
	objectNone
	objectLongCharge
)

const (
	patternPudding = iota
	patternCherry
	patternCake
	patternCakeLong
	patternPuddingOld
	patternCherryOld
	patternCakeOld
	patternCakeLongOld
	patternNone
)

const (
	sweetPudding = iota
	sweetCherryPudding
	sweetShortCake
	sweetCherry
	sweetLayerCake
	sweetNone = -1
)

const (
	actionRegular = 0 // HS Pad East / IA_PadBasicPress.
	actionStrong  = 3 // HS Pad South / IA_PadAltPress.
)

const (
	scrollMetresPerBeat   = -2.25
	legacyScrollMult      = 2.0
	boardWidth            = 19.2
	boardY                = -8.8
	shojiFullOpenX        = 17.8
	carpenterLayerKeyFace = "nailCarpenter:face"
)

type patternItem struct {
	beat float64
	typ  int
}

type patternEvt struct {
	beat, length float64
	typ          int
}

type legacyEvt struct {
	beat float64
	set  bool
}

type slideEvt struct {
	beat, length float64
	ratio        float64
	ease         int
}

type hammerWindow struct {
	beat   float64
	action int
}

type movingObject struct {
	targetBeat float64
	inst       *kart.Instance
	dead       bool
}

type nailObj struct {
	movingObject
}

type sweetObj struct {
	movingObject
	typ    int
	broken bool
}

type Module struct {
	ctx  *engine.Ctx
	proj kart.Aff

	nailT, longNailT, sweetT *kart.Template
	patterns                 []patternEvt
	legacy                   []legacyEvt
	slides                   []slideEvt
	hammers                  []hammerWindow

	nails  []*nailObj
	sweets []*sweetObj

	lastPulse   int
	scrollSpeed float64
}

func New() engine.Module {
	return &Module{lastPulse: math.MinInt, scrollSpeed: scrollMetresPerBeat}
}

func (m *Module) ID() string { return "nailCarpenter" }

func (m *Module) Load(ctx *engine.Ctx) error {
	m.ctx = ctx
	if err := ctx.LoadAssets("nailCarpenter"); err != nil {
		return err
	}
	m.proj = kart.Translate(engine.ScreenW/2, engine.ScreenH/2).Mul(kart.Scale(24, -24))
	m.nailT = kart.NewTemplate(ctx.Assets, "ScrollingItems/Prefabs/Nail")
	m.longNailT = kart.NewTemplate(ctx.Assets, "ScrollingItems/Prefabs/LongNail")
	m.sweetT = kart.NewTemplate(ctx.Assets, "ScrollingItems/Prefabs/Sweet")
	m.resetScene(0)
	return nil
}

func (m *Module) OnEvent(e *riq.Entity) {
	switch e.Datamodel {
	case "nailCarpenter/puddingNailNew":
		m.patterns = append(m.patterns, patternEvt{beat: e.Beat, length: e.Length, typ: patternPudding})
	case "nailCarpenter/cherryNailNew":
		m.patterns = append(m.patterns, patternEvt{beat: e.Beat, length: e.Length, typ: patternCherry})
	case "nailCarpenter/cakeNailNew":
		m.patterns = append(m.patterns, patternEvt{beat: e.Beat, length: e.Length, typ: patternCake})
	case "nailCarpenter/cakeLongNailNew":
		m.patterns = append(m.patterns, patternEvt{beat: e.Beat, length: e.Length, typ: patternCakeLong})
	case "nailCarpenter/puddingNail":
		m.patterns = append(m.patterns, patternEvt{beat: e.Beat, length: e.Length, typ: patternPuddingOld})
	case "nailCarpenter/cherryNail":
		m.patterns = append(m.patterns, patternEvt{beat: e.Beat, length: e.Length, typ: patternCherryOld})
	case "nailCarpenter/cakeNail":
		m.patterns = append(m.patterns, patternEvt{beat: e.Beat, length: e.Length, typ: patternCakeOld})
	case "nailCarpenter/cakeLongNail":
		m.patterns = append(m.patterns, patternEvt{beat: e.Beat, length: e.Length, typ: patternCakeLongOld})
	case "nailCarpenter/legacySpeed":
		m.legacy = append(m.legacy, legacyEvt{beat: e.Beat, set: boolDefault(e, "set", true)})
	case "nailCarpenter/slideFusuma":
		ev := slideEvt{
			beat: e.Beat, length: e.Length,
			ratio: e.Float("fillRatio", 0.3),
			ease:  int(e.Float("ease", 0)),
		}
		m.slides = append(m.slides, ev)
		if !boolParam(e, "mute") {
			m.ctx.SoundAt(ev.beat, "open", 1)
		}
	}
}

func (m *Module) Ready() {
	sort.SliceStable(m.legacy, func(i, j int) bool { return m.legacy[i].beat < m.legacy[j].beat })
	sort.SliceStable(m.slides, func(i, j int) bool { return m.slides[i].beat < m.slides[j].beat })
	sort.SliceStable(m.patterns, func(i, j int) bool { return m.patterns[i].beat < m.patterns[j].beat })

	for _, ev := range m.legacy {
		ev := ev
		m.ctx.At(ev.beat, func() { m.scrollSpeed = scrollSpeedForLegacy(ev.set) })
	}

	lastPattern := patternPudding
	for _, ev := range m.patterns {
		if ev.length == 0 {
			continue
		}
		typ := m.patternTypeAt(ev)
		for i := 0; i < int(math.Ceil(ev.length/patternSeekTime)); i++ {
			segBeat := ev.beat + patternSeekTime*float64(i)
			segLen := math.Min(ev.length-patternSeekTime*float64(i), patternSeekTime)
			lastPattern = m.spawnPattern(segBeat, segLen, typ, lastPattern)
		}
	}
}

func (m *Module) OnSwitch(beat float64) {
	m.resetScene(beat)
	m.scrollSpeed = m.scrollSpeedAt(beat)
	m.lastPulse = int(math.Floor(beat))
}

func (m *Module) Whiff(beat float64) { m.WhiffAction(beat, actionRegular) }

func (m *Module) WhiffAction(beat float64, action int) {
	switch action {
	case actionRegular, actionStrong:
		m.ctx.ScoreMiss()
		m.ctx.PlayCommon("miss")
		m.ctx.Scene.PlayState("Carpenter", "carpenterHit", beat, 0.25)
	}
}

func (m *Module) Update(_ float64, beat float64) {
	m.updatePulse(beat)
	m.ctx.Scene.SetPosOver("Board", math.Mod(beat*m.scrollSpeed, boardWidth), boardY)
	m.ctx.Scene.SetPosOver("FusumaContainer", m.shojiXAt(beat), 0)

	m.nails = liveNails(m.nails, beat)
	m.sweets = liveSweets(m.sweets, beat)
}

func (m *Module) Draw(screen *ebiten.Image, _ float64, beat float64) {
	sc := m.ctx.Scene
	m.ctx.SampleScene(beat)
	holder, _ := sc.NodeWorld("ScrollingItems/NailHolder")
	for _, n := range m.nails {
		if n.dead || !objectVisible(n.targetBeat, beat) {
			continue
		}
		n.inst.Queue(sc, beat, objectWorld(holder, n.targetBeat, beat, m.scrollSpeed), 0)
	}
	for _, s := range m.sweets {
		if s.dead || !objectVisible(s.targetBeat, beat) {
			continue
		}
		s.inst.Queue(sc, beat, objectWorld(holder, s.targetBeat, beat, m.scrollSpeed), 0)
	}
	sc.Draw(screen, m.proj)
}

func (m *Module) resetScene(beat float64) {
	sc := m.ctx.Scene
	sc.SetActive("ScrollingItems/Prefabs", false)
	sc.PlayDefaultState("Carpenter", beat, m.ctx.SecPerBeat(beat))
	sc.PlayDefaultState("Carpenter/ExclamRed", beat, m.ctx.SecPerBeat(beat))
	sc.PlayDefaultState("Carpenter/ExclamBlue", beat, m.ctx.SecPerBeat(beat))
	sc.SetPosOver("Board", math.Mod(beat*m.scrollSpeed, boardWidth), boardY)
	sc.SetPosOver("FusumaContainer", m.shojiXAt(beat), 0)
}

func (m *Module) updatePulse(beat float64) {
	pulse := int(math.Floor(beat))
	if pulse <= m.lastPulse {
		return
	}
	for b := m.lastPulse + 1; b <= pulse; b++ {
		// Unity blinks randomly on OnBeatPulse when no strong-hammer window is
		// active. A deterministic modulo keeps replay/autoplay verification stable.
		if b%10 == 0 && !m.hammerExpectedNow(actionStrong) {
			bb := float64(b)
			m.ctx.Scene.PlayStateLayer(carpenterLayerKeyFace, "Carpenter", "eyeBlinkFast", bb, 0.25)
		}
	}
	m.lastPulse = pulse
}

func (m *Module) spawnPattern(beat, length float64, typ, lastPattern int) int {
	items := patternItems(typ)
	patLen := patternLength(typ)
	if len(items) == 0 || patLen <= 0 {
		return lastPattern
	}
	for i := 0; i < int(math.Ceil(length/patLen)); i++ {
		itemBeat := beat + patLen*float64(i)
		m.spawnPatternSegment(itemBeat, typ, lastPattern, items)
		lastPattern = typ
	}
	return lastPattern
}

func (m *Module) spawnPatternSegment(beat float64, typ, lastPattern int, items []patternItem) {
	for _, item := range items {
		itemBeat := beat + item.beat
		switch item.typ {
		case objectLongCharge:
			m.ctx.SoundAt(itemBeat, "signal2", 1)
			m.ctx.At(itemBeat, func() {
				m.ctx.Scene.PlayState("Carpenter", "carpenterArmUp", itemBeat, 0.25)
			})
		case objectNail:
			m.spawnNail(itemBeat, false)
		case objectLongNail:
			m.spawnNail(itemBeat, true)
		case objectSweet:
			m.spawnSweetByPattern(itemBeat, typ, lastPattern)
		case objectForceCherry:
			m.spawnSweet(itemBeat, sweetCherry)
		case objectForcePudding:
			m.ctx.SoundAt(itemBeat, "one", 1)
			m.spawnSweet(itemBeat, sweetPudding)
		case objectForceCherryPudding:
			m.ctx.SoundAt(itemBeat, "three", 1)
			m.spawnSweet(itemBeat, sweetCherryPudding)
		case objectForceShortCake:
			m.ctx.SoundAt(itemBeat, "alarm", 1)
			m.flashExclam(itemBeat, "Carpenter/ExclamRed")
			m.spawnSweet(itemBeat, sweetShortCake)
		case objectForceLayerCake:
			m.ctx.SoundAt(itemBeat, "signal1", 1)
			m.flashExclam(itemBeat, "Carpenter/ExclamBlue")
			m.spawnSweet(itemBeat, sweetLayerCake)
		}
	}
}

func (m *Module) spawnSweetByPattern(beat float64, typ, lastPattern int) {
	sweetType := sweetForPattern(typ)
	switch typ {
	case patternPudding, patternPuddingOld:
		m.ctx.SoundAt(beat, "one", 1)
	case patternCherry, patternCherryOld:
		m.ctx.SoundAt(beat, "three", 1)
	case patternCake:
		m.ctx.SoundAt(beat, "alarm", 0.978)
		m.ctx.SoundAt(beat+1.5, "one", 1)
		m.flashExclam(beat, "Carpenter/ExclamRed")
	case patternCakeOld:
		m.ctx.SoundAt(beat, "alarm", 0.978)
		m.ctx.SoundAt(beat+0.75, "one", 1)
		m.flashExclam(beat, "Carpenter/ExclamRed")
	case patternCakeLong:
		m.ctx.SoundAt(beat, "signal1", 1)
		m.ctx.SoundAt(beat+2, "one", 1)
		m.flashExclam(beat, "Carpenter/ExclamBlue")
	case patternCakeLongOld:
		m.ctx.SoundAt(beat, "signal1", 1)
		m.ctx.SoundAt(beat+1, "one", 1)
		m.flashExclam(beat, "Carpenter/ExclamBlue")
	}
	if lastPattern == patternCake || lastPattern == patternCakeOld {
		sweetType = sweetCherry
	}
	if sweetType != sweetNone {
		m.spawnSweet(beat, sweetType)
	}
}

func (m *Module) spawnNail(target float64, long bool) {
	tpl := m.nailT
	if long {
		tpl = m.longNailT
	}
	if tpl == nil {
		return
	}
	obj := &nailObj{
		movingObject: movingObject{targetBeat: target, inst: tpl.NewInstance()},
	}
	obj.inst.PlayDefaultState("", target, m.ctx.SecPerBeat(target))
	m.nails = append(m.nails, obj)
	if long {
		m.scheduleLongNailInput(obj)
	} else {
		m.scheduleNailInput(obj)
	}
}

func (m *Module) scheduleNailInput(n *nailObj) {
	m.registerHammer(n.targetBeat, actionRegular)
	m.registerHammer(n.targetBeat, actionStrong)
	m.ctx.ScheduleInputAction(n.targetBeat, actionRegular,
		func(state float64, _ engine.Judgment) { m.nailRegularJust(n, state) },
		func() { m.nailMiss(n) })
	wrong := m.ctx.ScheduleInputAction(n.targetBeat, actionStrong,
		func(state float64, _ engine.Judgment) { m.nailStrongWrong(n, state) },
		func() {})
	wrong.NoScore = true
	wrong.NoAutoplay = true
}

func (m *Module) scheduleLongNailInput(n *nailObj) {
	m.registerHammer(n.targetBeat, actionRegular)
	m.registerHammer(n.targetBeat, actionStrong)
	m.ctx.ScheduleInputAction(n.targetBeat, actionStrong,
		func(state float64, _ engine.Judgment) { m.longNailStrongJust(n, state) },
		func() { m.longNailMiss(n) })
	wrong := m.ctx.ScheduleInputAction(n.targetBeat, actionRegular,
		func(state float64, _ engine.Judgment) { m.longNailWeakWrong(n, state) },
		func() {})
	wrong.NoScore = true
	wrong.NoAutoplay = true
}

func (m *Module) nailRegularJust(n *nailObj, state float64) {
	beat := m.ctx.Beat()
	m.ctx.Scene.PlayState("Carpenter", "carpenterHit", beat, 0.25)
	if offWindow(state) {
		n.inst.PlayState("Pivot/Sprite", pickState(state, "nailBendLeft", "nailBendRight"), beat, 0.25)
		m.ctx.PlayCommon("miss")
		return
	}
	m.ctx.Sound("HammerWeak")
	n.inst.PlayState("Pivot/Sprite", "nailHammered", beat, 0.25)
}

func (m *Module) nailStrongWrong(n *nailObj, state float64) {
	beat := m.ctx.Beat()
	m.ctx.ScoreMiss()
	m.ctx.Scene.PlayState("Carpenter", "carpenterHit", beat, 0.25)
	if offWindow(state) {
		n.inst.PlayState("Pivot/Sprite", pickState(state, "nailBendLeft", "nailBendRight"), beat, 0.25)
		m.ctx.PlayCommon("miss")
		return
	}
	m.ctx.Sound("HammerStrong")
	n.inst.PlayState("Pivot/Sprite", "nailStrongHammered", beat, 0.25)
}

func (m *Module) nailMiss(n *nailObj) {
	beat := n.targetBeat
	m.ctx.Scene.PlayStateLayer(carpenterLayerKeyFace, "Carpenter", "eyeBlink", beat, 0.25)
	n.inst.PlayState("Pivot/Sprite", "nailMiss", beat, 0.5)
}

func (m *Module) longNailStrongJust(n *nailObj, state float64) {
	beat := m.ctx.Beat()
	m.ctx.Scene.PlayState("Carpenter", "carpenterHit", beat, 0.25)
	if offWindow(state) {
		n.inst.PlayState("Pivot/Sprite", pickState(state, "longNailBendLeft", "longNailBendRight"), beat, 0.25)
		m.ctx.PlayCommon("miss")
		return
	}
	m.ctx.Sound("HammerStrong")
	n.inst.PlayState("Pivot/Sprite", "longNailHammered", beat, 0.25)
	m.ctx.Scene.PlayStateLayer(carpenterLayerKeyFace, "Carpenter", "eyeSmile", beat, 0.25)
}

func (m *Module) longNailWeakWrong(n *nailObj, state float64) {
	beat := m.ctx.Beat()
	m.ctx.ScoreMiss()
	m.ctx.Scene.PlayState("Carpenter", "carpenterHit", beat, 0.25)
	if offWindow(state) {
		n.inst.PlayState("Pivot/Sprite", pickState(state, "longNailBendLeft", "longNailBendRight"), beat, 0.25)
		m.ctx.PlayCommon("miss")
		return
	}
	m.ctx.Sound("HammerWeak")
	n.inst.PlayState("Pivot/Sprite", "longNailWeakHammered", beat, 0.25)
}

func (m *Module) longNailMiss(n *nailObj) {
	beat := n.targetBeat
	m.ctx.Scene.PlayStateLayer(carpenterLayerKeyFace, "Carpenter", "eyeBlink", beat, 0.25)
	n.inst.PlayState("Pivot/Sprite", "longNailMiss", beat, 0.5)
}

func (m *Module) spawnSweet(target float64, typ int) {
	if m.sweetT == nil || typ == sweetNone {
		return
	}
	obj := &sweetObj{
		movingObject: movingObject{targetBeat: target, inst: m.sweetT.NewInstance()},
		typ:          typ,
	}
	obj.inst.PlayState("Pivot/Sprite", sweetIdleState(typ), target, m.ctx.SecPerBeat(target))
	m.sweets = append(m.sweets, obj)
	m.scheduleSweetInput(obj)
	if st := sweetBeatState(typ); st != "" {
		m.ctx.At(target, func() {
			if !obj.broken {
				obj.inst.PlayState("Pivot/Sprite", st, target, 1)
			}
		})
	}
}

func (m *Module) scheduleSweetInput(s *sweetObj) {
	for _, action := range []int{actionRegular, actionStrong} {
		action := action
		in := m.ctx.ScheduleInputActionCond(s.targetBeat, action,
			func() bool { return !s.dead && !s.broken && !m.hammerExpectedNow(-1) },
			func(float64, engine.Judgment) { m.sweetHit(s) },
			func() {})
		in.NoScore = true
		in.NoAutoplay = true
	}
}

func (m *Module) sweetHit(s *sweetObj) {
	if s.broken {
		return
	}
	beat := m.ctx.Beat()
	m.ctx.ScoreMiss()
	s.broken = true
	s.inst.PlayState("Pivot/Sprite", sweetBreakState(s.typ), beat, 0.25)
	m.ctx.Scene.PlayStateLayer(carpenterLayerKeyFace, "Carpenter", "eyeBlink", beat, 0.25)
}

func (m *Module) registerHammer(beat float64, action int) {
	m.hammers = append(m.hammers, hammerWindow{beat: beat, action: action})
}

func (m *Module) hammerExpectedNow(action int) bool {
	now := m.ctx.BeatToTime(m.ctx.Beat())
	for _, h := range m.hammers {
		if action >= 0 && h.action != action {
			continue
		}
		if math.Abs(now-m.ctx.BeatToTime(h.beat)) <= engine.WinNG {
			return true
		}
	}
	return false
}

func (m *Module) flashExclam(beat float64, path string) {
	m.ctx.At(beat, func() { m.ctx.Scene.PlayState(path, "exclamAppear", beat, 0.25) })
}

func (m *Module) patternTypeAt(ev patternEvt) int {
	switch ev.typ {
	case patternPudding, patternCherry, patternCake, patternCakeLong:
		if m.legacyAt(ev.beat) {
			return ev.typ + 4
		}
	}
	return ev.typ
}

func (m *Module) legacyAt(beat float64) bool {
	legacy := false
	for _, ev := range m.legacy {
		if ev.beat > beat {
			break
		}
		legacy = ev.set
	}
	return legacy
}

func (m *Module) scrollSpeedAt(beat float64) float64 {
	for _, ev := range m.patterns {
		if ev.beat <= beat && ev.typ >= patternPuddingOld && ev.typ <= patternCakeLongOld {
			return scrollSpeedForLegacy(true)
		}
	}
	return scrollSpeedForLegacy(m.legacyAt(beat))
}

func (m *Module) shojiXAt(beat float64) float64 {
	lastRatio := 0.0
	for _, ev := range m.slides {
		if beat < ev.beat {
			break
		}
		prevX := shojiFullOpenX * (1 - lastRatio)
		nextX := shojiFullOpenX * (1 - ev.ratio)
		if ev.length <= 0 || beat >= ev.beat+ev.length {
			lastRatio = ev.ratio
			continue
		}
		u := clamp01((beat - ev.beat) / ev.length)
		return engine.Ease(ev.ease, prevX, nextX, u)
	}
	return shojiFullOpenX * (1 - lastRatio)
}

func objectWorld(holder kart.Aff, targetBeat, beat, speed float64) kart.Aff {
	return holder.Mul(kart.Translate((beat-targetBeat)*speed, 0))
}

func objectVisible(targetBeat, beat float64) bool {
	return beat >= targetBeat-patternSeekTime && beat <= targetBeat+9
}

func liveNails(in []*nailObj, beat float64) []*nailObj {
	out := in[:0]
	for _, n := range in {
		n.dead = n.dead || beat >= n.targetBeat+9
		if !n.dead {
			out = append(out, n)
		}
	}
	return out
}

func liveSweets(in []*sweetObj, beat float64) []*sweetObj {
	out := in[:0]
	for _, s := range in {
		s.dead = s.dead || beat >= s.targetBeat+9
		if !s.dead {
			out = append(out, s)
		}
	}
	return out
}

func patternItems(typ int) []patternItem {
	switch typ {
	case patternPudding:
		return puddingPattern
	case patternCherry:
		return cherryPattern
	case patternCake:
		return cakePattern
	case patternCakeLong:
		return cakeLongPattern
	case patternPuddingOld:
		return puddingPatternOld
	case patternCherryOld:
		return cherryPatternOld
	case patternCakeOld:
		return cakePatternOld
	case patternCakeLongOld:
		return cakeLongPatternOld
	}
	return nil
}

func patternLength(typ int) float64 {
	items := patternItems(typ)
	if len(items) == 0 {
		return 0
	}
	return items[len(items)-1].beat
}

func sweetForPattern(typ int) int {
	switch typ {
	case patternPudding, patternPuddingOld:
		return sweetPudding
	case patternCherry, patternCherryOld:
		return sweetCherryPudding
	case patternCake, patternCakeOld:
		return sweetShortCake
	case patternCakeLong, patternCakeLongOld:
		return sweetLayerCake
	}
	return sweetNone
}

func sweetIdleState(typ int) string {
	switch typ {
	case sweetPudding:
		return "puddingIdle"
	case sweetCherryPudding:
		return "cherryPuddingIdle"
	case sweetShortCake:
		return "shortCakeIdle"
	case sweetCherry:
		return "cherryIdle"
	case sweetLayerCake:
		return "layerCakeIdle"
	}
	return "puddingIdle"
}

func sweetBeatState(typ int) string {
	switch typ {
	case sweetPudding:
		return "puddingBeat"
	case sweetCherryPudding:
		return "cherryPuddingBeat"
	case sweetShortCake:
		return "shortCakeBeat"
	case sweetLayerCake:
		return "layerCakeBeat"
	}
	return ""
}

func sweetBreakState(typ int) string {
	switch typ {
	case sweetPudding:
		return "puddingBreak"
	case sweetCherryPudding:
		return "cherryPuddingBreak"
	case sweetShortCake:
		return "shortCakeBreak"
	case sweetCherry:
		return "cherryBreak"
	case sweetLayerCake:
		return "layerCakeBreak"
	}
	return "puddingBreak"
}

func scrollSpeedForLegacy(legacy bool) float64 {
	if legacy {
		return scrollMetresPerBeat * legacyScrollMult
	}
	return scrollMetresPerBeat
}

func offWindow(state float64) bool { return state >= 1 || state <= -1 }

func pickState(state float64, early, late string) string {
	if state >= 1 {
		return late
	}
	return early
}

func boolParam(e *riq.Entity, key string) bool { return e.Float(key, 0) != 0 }

func boolDefault(e *riq.Entity, key string, def bool) bool {
	if _, ok := e.Data[key]; !ok {
		return def
	}
	return boolParam(e, key)
}

func clamp01(v float64) float64 {
	switch {
	case v < 0:
		return 0
	case v > 1:
		return 1
	default:
		return v
	}
}

// The eight pattern arrays are copied from nailCarpenter.prefab's serialized
// ObjectPatternItem fields. Keeping them in code makes the rhythm grammar
// auditable even though these arrays are not Unity AnimationClip data.
var (
	puddingPattern = []patternItem{
		{0, objectSweet},
		{1, objectNail},
		{2, objectNone},
	}
	cherryPattern = []patternItem{
		{0, objectSweet},
		{1, objectNail},
		{2, objectNail},
		{3, objectNail},
		{4, objectNone},
	}
	cakePattern = []patternItem{
		{0, objectSweet},
		{1, objectNail},
		{2, objectForceCherry},
		{2.5, objectNail},
		{3.5, objectNail},
		{4, objectNone},
	}
	cakeLongPattern = []patternItem{
		{0, objectSweet},
		{1, objectNail},
		{2, objectLongCharge},
		{3, objectLongNail},
		{4, objectNone},
	}
	puddingPatternOld = []patternItem{
		{0, objectSweet},
		{0.5, objectNail},
		{1, objectNone},
	}
	cherryPatternOld = []patternItem{
		{0, objectSweet},
		{0.5, objectNail},
		{1, objectNail},
		{1.5, objectNail},
		{2, objectNone},
	}
	cakePatternOld = []patternItem{
		{0, objectSweet},
		{0.5, objectNail},
		{1, objectForceCherry},
		{1.25, objectNail},
		{1.75, objectNail},
		{2, objectNone},
	}
	cakeLongPatternOld = []patternItem{
		{0, objectSweet},
		{0.5, objectNail},
		{1, objectLongCharge},
		{1.5, objectLongNail},
		{2, objectNone},
	}
)
