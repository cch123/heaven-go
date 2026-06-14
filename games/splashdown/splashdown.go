// Package splashdown ports Splashdown's Synchrette line cues, jump/dive
// choreography, splash prefabs, hand-written water particles, and result sounds.
package splashdown

import (
	"image/color"
	"math"
	"math/rand"
	"sort"
	"strconv"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"hsdemo/engine"
	"hsdemo/kart"
	"hsdemo/kmdata"
	"hsdemo/riq"
)

const (
	stateFloat = iota
	stateDive
	stateJump
	stateRaise
	stateStand
	stateJumpIntoWater
)

type cueKind int

const (
	cueDive cueKind = iota
	cueAppear
	cueJump
	cueTogether
	cueTogetherR9
	cueIntro
	cueForceDive
	cueAmount
)

type cue struct {
	kind         cueKind
	beat, length float64
	appearType   int
	amount       int
	dolphin      bool
	alleyoop     bool
}

type synchrette struct {
	inst       *kart.Instance
	rootX      float64
	rootY      float64
	state      int
	startBeat  float64
	missedJump bool
}

type splashInst struct {
	inst      *kart.Instance
	beat      float64
	kind      string
	rootX     float64
	particles []drop
}

type drop struct {
	x, y   float64
	vx, vy float64
	life   float64
	size   float64
}

type Module struct {
	ctx  *engine.Ctx
	proj kart.Aff

	holderPath string
	crowdPath  string
	waterPath  string

	synchT  *kart.Template
	splashT *kart.Template

	synchRel string
	animRel  string
	throwRel string

	jumpHeight float64
	jumpStart  float64
	distance   float64

	cues []cue

	synchrettes []*synchrette
	player      *synchrette
	splashes    []*splashInst

	introBeat      float64
	introLength    float64
	gameSwitchBeat float64
}

func New() engine.Module { return &Module{introBeat: -1, gameSwitchBeat: -1} }

func (m *Module) ID() string { return "splashdown" }

func (m *Module) Load(ctx *engine.Ctx) error {
	m.ctx = ctx
	if err := ctx.LoadAssets("splashdown"); err != nil {
		return err
	}
	m.proj = kart.Translate(engine.ScreenW/2, engine.ScreenH/2).Mul(kart.Scale(54, -54))
	game := ctx.Assets.Extra.Components["game"]
	syn := ctx.Assets.Extra.Components["synchrette"]
	m.holderPath = refOr(ctx, game, "synchretteHolder", "SynchrettesHolder")
	m.crowdPath = refOr(ctx, game, "crowdAnim", "Background/Crowd")
	m.waterPath = "Background/Water"
	m.synchT = kart.NewTemplate(ctx.Assets, refOr(ctx, game, "synchrettePrefab", ctx.Role("synchrettePrefab")))
	m.splashT = kart.NewTemplate(ctx.Assets, refOr(ctx, syn, "splashPrefab", "Splashes"))
	m.synchRel = relPath(syn.Refs["synchretteTransform"], "SynchretteHolder")
	m.animRel = relPath(syn.Refs["anim"], "SynchretteHolder")
	m.throwRel = relPath(syn.Refs["throwAnim"], "SynchretteHolder")
	if m.synchRel == "" {
		m.synchRel = "PosHolder"
	}
	if m.animRel == "" {
		m.animRel = "PosHolder/Synchrette"
	}
	if m.throwRel == "" {
		m.throwRel = "ThrowSynchrette"
	}
	m.jumpHeight = numOr(syn, "jumpHeight", 5)
	m.jumpStart = numOr(syn, "jumpStart", -3)
	m.distance = numOr(game, "synchretteDistance", 3.75)
	m.resetScene(0)
	return nil
}

func refOr(ctx *engine.Ctx, c kmdata.Component, field, fallback string) string {
	if c.Refs != nil && c.Refs[field] != "" {
		return c.Refs[field]
	}
	if p := ctx.Role(field); p != "" {
		return p
	}
	return fallback
}

func numOr(c kmdata.Component, field string, fallback float64) float64 {
	if c.Nums != nil {
		if v, ok := c.Nums[field]; ok {
			return v
		}
	}
	return fallback
}

func relPath(path, root string) string {
	if path == root {
		return ""
	}
	prefix := root + "/"
	if len(path) > len(prefix) && path[:len(prefix)] == prefix {
		return path[len(prefix):]
	}
	return path
}

func (m *Module) OnEvent(e *riq.Entity) {
	c := cue{beat: e.Beat, length: e.Length}
	switch e.Datamodel {
	case "splashdown/dive":
		c.kind = cueDive
	case "splashdown/appear":
		c.kind = cueAppear
		c.appearType = intParam(e, "type", 1)
	case "splashdown/jump":
		c.kind = cueJump
		c.dolphin = boolParamDefault(e, "dolphin", true)
	case "splashdown/together":
		c.kind = cueTogether
		c.alleyoop = boolParam(e, "al")
	case "splashdown/togetherR9":
		c.kind = cueTogetherR9
		c.alleyoop = boolParam(e, "al")
	case "splashdown/intro":
		c.kind = cueIntro
	case "splashdown/forceDive":
		c.kind = cueForceDive
	case "splashdown/amount":
		c.kind = cueAmount
		c.amount = intParam(e, "amount", 3)
	default:
		return
	}
	m.cues = append(m.cues, c)
}

func (m *Module) Ready() {
	sort.SliceStable(m.cues, func(i, j int) bool { return m.cues[i].beat < m.cues[j].beat })
	amount := 3
	for _, c := range m.cues {
		c := c
		if c.length <= 0 {
			c.length = defaultLength(c.kind)
		}
		switch c.kind {
		case cueAmount:
			amount = clampInt(c.amount, 3, 5)
			m.ctx.At(c.beat, func() { m.spawnSynchrettes(c.amount, c.beat) })
		case cueIntro:
			m.scheduleIntro(c, amount)
		case cueForceDive:
			m.ctx.At(c.beat, func() { m.forceDive(c.beat) })
		case cueDive:
			if !m.cueDuringIntro(c.beat) {
				m.scheduleDive(c, amount-1)
			}
		case cueAppear:
			if !m.cueDuringIntro(c.beat) {
				m.scheduleAppear(c, amount-1)
			}
		case cueJump:
			if !m.cueDuringIntro(c.beat) {
				m.scheduleJump(c, amount-1)
			}
		case cueTogether:
			if !m.cueDuringIntro(c.beat) {
				m.scheduleTogether(c, amount-1, 2, false)
			}
		case cueTogetherR9:
			if !m.cueDuringIntro(c.beat) {
				m.scheduleTogether(c, amount-1, 1, true)
			}
		}
	}
}

func defaultLength(k cueKind) float64 {
	switch k {
	case cueJump:
		return 2
	case cueTogether:
		return 4
	case cueTogetherR9:
		return 3
	case cueIntro:
		return 8
	default:
		return 0.5
	}
}

func (m *Module) OnSwitch(beat float64) {
	m.gameSwitchBeat = beat
	sec := m.ctx.SecPerBeat(beat)
	m.ctx.Scene.PlayDefaultState(m.waterPath, beat, sec)
	m.ctx.Scene.PlayDefaultState(m.crowdPath, beat, sec)
}

func (m *Module) Whiff(beat float64) {
	if m.isIntroing(beat) || m.player == nil {
		return
	}
	m.ctx.PlayCommon("miss")
	m.ctx.Sound("downPlayer")
	m.player.goDown(m, beat, true)
	m.ctx.ScoreMiss()
}

func (m *Module) Update(_ float64, beat float64) {
	if m.ctx.ReleasedNow() && !m.ctx.ExpectingReleaseNow() && !m.isIntroing(beat) && m.player != nil {
		m.ctx.PlayCommon("miss")
		m.player.appear(m, beat, true, 1)
		m.ctx.Sound("upPlayer")
		m.ctx.ScoreMiss()
	}
	keep := m.splashes[:0]
	for _, sp := range m.splashes {
		if beat < sp.beat+1.2 {
			keep = append(keep, sp)
		}
	}
	m.splashes = keep
}

func (m *Module) Draw(screen *ebiten.Image, _ float64, beat float64) {
	screen.Fill(color.NRGBA{0x32, 0x7b, 0xf5, 0xff})
	m.ctx.SampleScene(beat)
	holder, _ := m.ctx.Scene.NodeWorld(m.holderPath)
	for _, s := range m.allSynchrettes() {
		s.update(m, beat)
		s.inst.Queue(m.ctx.Scene, beat, holder, 0)
	}
	for _, sp := range m.splashes {
		base := holder.Mul(kart.Translate(sp.rootX, 0))
		sp.inst.Queue(m.ctx.Scene, beat, base, 0)
	}
	m.ctx.Scene.Draw(screen, m.proj)
	m.drawDrops(screen, beat, holder)
}

func (m *Module) resetScene(beat float64) {
	sec := m.ctx.SecPerBeat(beat)
	m.ctx.Scene.PlayDefaultState(m.waterPath, beat, sec)
	m.ctx.Scene.PlayDefaultState(m.crowdPath, beat, sec)
	m.introBeat = -1
	m.introLength = 0
	m.spawnSynchrettes(3, -1)
}

func (m *Module) spawnSynchrettes(amount int, beat float64) {
	if m.synchT == nil {
		return
	}
	amount = clampInt(amount, 3, 5)
	shouldDive := false
	if beat >= 0 {
		shouldDive = m.lastInputCueBefore(beat) == cueDive
	}
	startX := -(float64(amount/2) * m.distance)
	if amount%2 == 0 {
		startX += m.distance / 2
	}
	m.synchrettes = nil
	m.player = nil
	for i := 0; i < amount; i++ {
		inst := m.synchT.NewInstance()
		rootX := startX + m.distance*float64(i)
		inst.Offset = [2]float64{rootX, -3.68}
		inst.PlayDefaultState(m.animRel, beat, m.ctx.SecPerBeat(math.Max(beat, 0)))
		inst.SetActive(m.throwRel, false)
		s := &synchrette{inst: inst, rootX: rootX, rootY: -3.68, state: stateFloat}
		if shouldDive {
			s.goDown(m, beat, false)
		}
		if i < amount-1 {
			m.synchrettes = append(m.synchrettes, s)
		} else {
			m.player = s
		}
	}
}

func (m *Module) lastInputCueBefore(beat float64) cueKind {
	last := cueKind(-1)
	for _, c := range m.cues {
		if c.beat >= beat || c.beat < m.gameSwitchBeat {
			continue
		}
		switch c.kind {
		case cueDive, cueAppear, cueJump, cueTogether, cueTogetherR9:
			last = c.kind
		}
	}
	return last
}

func (m *Module) scheduleDive(c cue, count int) {
	for i := 0; i < count; i++ {
		idx := i
		b := c.beat + float64(i)*c.length
		m.ctx.At(b, func() {
			if idx < len(m.synchrettes) {
				m.synchrettes[idx].goDown(m, b, true)
			}
		})
		m.ctx.SoundAt(b, "whistle", 1)
		m.ctx.SoundAt(b, "downOthers", 1)
	}
	target := c.beat + float64(count)*c.length
	m.ctx.SoundAt(target, "whistle", 1)
	m.ctx.ScheduleInput(target, func(state float64, _ engine.Judgment) {
		if m.player == nil {
			return
		}
		m.ctx.Sound("downPlayer")
		m.player.goDown(m, m.ctx.Beat(), true)
		if math.Abs(state) >= 1 {
			m.ctx.PlayCommon("miss")
		}
	}, func() {})
}

func (m *Module) scheduleAppear(c cue, count int) {
	typ := clampInt(c.appearType, 1, 3)
	for i := 0; i < count; i++ {
		idx := i
		b := c.beat + float64(i)*c.length
		m.ctx.At(b, func() {
			if idx < len(m.synchrettes) {
				m.synchrettes[idx].appear(m, b, false, typ)
			}
		})
		m.ctx.SoundAt(b, "whistle", 1)
		m.ctx.SoundAt(b, "upOthers", 1)
	}
	target := c.beat + float64(count)*c.length
	m.ctx.SoundAt(target, "whistle", 1)
	m.ctx.ScheduleInputRelease(target, func(state float64, _ engine.Judgment) {
		if m.player == nil {
			return
		}
		m.ctx.Sound("upPlayer")
		if math.Abs(state) >= 1 {
			m.ctx.PlayCommon("miss")
			m.player.appear(m, m.ctx.Beat(), true, typ)
			return
		}
		m.player.appear(m, m.ctx.Beat(), false, typ)
	}, func() {})
}

func (m *Module) scheduleJump(c cue, count int) {
	for i := 0; i < count; i++ {
		idx := i
		b := c.beat + float64(i)*c.length
		m.ctx.At(b, func() {
			if idx < len(m.synchrettes) {
				m.synchrettes[idx].jump(m, b, false, !c.dolphin)
			}
		})
		m.ctx.SoundAt(b, "yeah", 1)
		m.ctx.SoundAt(b, "jumpOthers", 1)
		if c.dolphin {
			m.ctx.SoundAt(b+1, "rollOthers", 1)
		}
		m.ctx.SoundAt(b+1.75, "splashOthers", 1)
	}
	target := c.beat + float64(count)*c.length
	m.ctx.SoundAt(target, "yeah", 1)
	m.schedulePlayerJump(target, !c.dolphin)
}

func (m *Module) scheduleTogether(c cue, count int, lead float64, remix9 bool) {
	if remix9 {
		m.ctx.SoundAt(c.beat, "togetherRemix9", 1)
	} else {
		m.ctx.SoundAt(c.beat, "together", 1)
	}
	jumpBeat := c.beat + lead
	m.ctx.At(jumpBeat, func() {
		for i := 0; i < count && i < len(m.synchrettes); i++ {
			m.synchrettes[i].jump(m, jumpBeat, false, c.alleyoop)
		}
	})
	if c.alleyoop {
		m.ctx.SoundAt(jumpBeat, "jumpOthers", 1)
		m.ctx.SoundAtOff(jumpBeat+0.5, "alleyOop1", 1, 0.014)
		m.ctx.SoundAt(jumpBeat+0.75, "alleyOop2", 1)
		m.ctx.SoundAtOff(jumpBeat+1, "alleyOop3", 1, 0.014)
		m.ctx.SoundAt(jumpBeat+1.75, "splashOthers", 1)
	} else {
		m.ctx.SoundAt(jumpBeat, "jumpOthers", 1)
		m.ctx.SoundAt(jumpBeat+1, "rollOthers", 1)
		m.ctx.SoundAt(jumpBeat+1.75, "splashOthers", 1)
	}
	m.schedulePlayerJump(jumpBeat, c.alleyoop)
}

func (m *Module) schedulePlayerJump(target float64, noDolphin bool) {
	m.ctx.ScheduleInputRelease(target, func(state float64, _ engine.Judgment) {
		if m.player == nil {
			return
		}
		diveBeat := target
		m.ctx.Sound("jumpPlayer")
		m.ctx.SoundAt(diveBeat+1.75, "splashPlayer", 1)
		if math.Abs(state) >= 1 {
			m.player.jump(m, diveBeat, true, noDolphin)
			return
		}
		if !noDolphin {
			m.ctx.SoundAt(diveBeat+1, "rollPlayer", 1)
		}
		m.player.jump(m, diveBeat, false, noDolphin)
		m.ctx.At(diveBeat+1.75, func() { m.ctx.Scene.PlayState(m.crowdPath, "CrowdCheer", diveBeat+1.75, 0.5) })
		m.ctx.At(diveBeat+4, func() { m.ctx.Scene.PlayState(m.crowdPath, "CrowdIdle", diveBeat+4, 0.5) })
	}, func() {})
}

func (m *Module) scheduleIntro(c cue, count int) {
	m.ctx.At(c.beat, func() {
		m.introBeat = c.beat
		m.introLength = c.length
	})
	for i := 0; i < int(c.length-1); i++ {
		b := c.beat + float64(i)
		m.ctx.At(b, func() {
			for _, s := range m.allSynchrettes() {
				s.bop(m, b)
			}
		})
	}
	jumpBeat := c.beat + c.length - 1
	m.ctx.At(jumpBeat, func() {
		for _, s := range m.allSynchrettes() {
			s.jumpIntoWater(m, jumpBeat)
		}
	})
	m.ctx.SoundAt(c.beat+c.length-0.25, "start", 1)
}

func (m *Module) forceDive(beat float64) {
	if m.isIntroing(beat) {
		return
	}
	for _, s := range m.allSynchrettes() {
		s.goDown(m, beat, false)
	}
}

func (m *Module) cueDuringIntro(beat float64) bool {
	for _, c := range m.cues {
		if c.kind != cueIntro || c.length <= 0 {
			continue
		}
		if beat >= c.beat && beat <= c.beat+c.length {
			return true
		}
	}
	return false
}

func (m *Module) isIntroing(beat float64) bool {
	return m.introLength > 0 && beat >= m.introBeat && beat <= m.introBeat+m.introLength
}

func (m *Module) allSynchrettes() []*synchrette {
	out := make([]*synchrette, 0, len(m.synchrettes)+1)
	out = append(out, m.synchrettes...)
	if m.player != nil {
		out = append(out, m.player)
	}
	return out
}

func (s *synchrette) update(m *Module, beat float64) {
	y, rot := 0.0, 0.0
	switch s.state {
	case stateFloat:
		up := math.Mod(beat-1, 2)
		if up < 0 {
			up += 2
		}
		down := math.Mod(beat, 2)
		if down < 0 {
			down += 2
		}
		if up <= 1 {
			y = easeInOutQuad(0, -0.5, up)
		} else {
			y = easeInOutQuad(-0.5, 0, down)
		}
	case stateDive:
		y = -6
	case stateJump:
		up := (beat - s.startBeat)
		down := (beat - (s.startBeat + 1))
		if up <= 1 {
			y = easeOutCubic(m.jumpStart, m.jumpHeight, clamp01(up))
		} else {
			y = easeInCubic(m.jumpHeight, m.jumpStart, clamp01(down))
		}
		if !s.missedJump {
			rotU := clamp01((beat - (s.startBeat + 1)) / 0.25)
			rot = 2 * math.Pi * rotU
		}
	case stateRaise:
		u := clamp01(beat - s.startBeat)
		y = lerp(-6, 0, u)
		if u >= 1 {
			s.state = stateFloat
		}
	case stateStand:
		y = 2.73
	case stateJumpIntoWater:
		up := (beat - s.startBeat) / 0.5
		if up <= 1 {
			y = easeOutQuad(2.73, 4, clamp01(up))
		} else {
			y = easeInQuad(4, -3, clamp01((beat-(s.startBeat+0.5))/0.5))
		}
	}
	s.inst.Offset = [2]float64{s.rootX, s.rootY}
	s.inst.SetPos(m.synchRel, 0, y)
	s.inst.SetRot(m.synchRel, rot)
}

func (s *synchrette) setState(m *Module, state int, beat float64) {
	s.state = state
	s.startBeat = beat
	s.inst.SetRot(m.synchRel, 0)
}

func (s *synchrette) appear(m *Module, beat float64, miss bool, typ int) {
	s.setState(m, stateFloat, beat)
	if miss {
		s.inst.PlayState(m.animRel, "MissAppear", beat, 0.4)
	} else {
		s.inst.PlayState(m.animRel, "Appear"+strconv.Itoa(clampInt(typ, 1, 3)), beat, 0.4)
	}
	m.spawnSplash(s.rootX, beat, "Appearsplash")
}

func (s *synchrette) goDown(m *Module, beat float64, splash bool) {
	s.setState(m, stateDive, beat)
	if splash {
		m.spawnSplash(s.rootX, beat, "GodownSplash")
	}
}

func (s *synchrette) bop(m *Module, beat float64) {
	s.inst.PlayState(m.animRel, "Bop", beat, 0.5)
	if s.state != stateStand {
		s.setState(m, stateStand, beat)
	}
}

func (s *synchrette) jumpIntoWater(m *Module, beat float64) {
	s.inst.PlayState(m.animRel, "Idle", beat, 0.5)
	s.setState(m, stateJumpIntoWater, beat)
	m.ctx.At(beat+0.75, func() { m.spawnSplash(s.rootX, beat+0.75, "GodownSplash") })
	m.ctx.At(beat+1, func() { s.setState(m, stateRaise, beat+1) })
}

func (s *synchrette) jump(m *Module, beat float64, missed, noDolphin bool) {
	s.missedJump = missed
	s.setState(m, stateJump, beat)
	m.spawnSplash(s.rootX, beat, "Appearsplash")
	s.inst.SetActive(m.throwRel, false)
	if noDolphin {
		state := "JumpOut"
		if missed {
			state = "JumpMiss"
		}
		s.inst.PlayState(m.animRel, state, beat, 0.5)
		s.inst.SetActive(m.throwRel, true)
		s.inst.PlayState(m.throwRel, "Throw", beat, 0.5)
	} else {
		state := "Dolphin"
		if missed {
			state = "DolphinMiss"
		}
		s.inst.PlayState(m.animRel, state, beat, 0.5)
	}
	m.ctx.At(beat+1.75, func() { m.spawnSplash(s.rootX, beat+1.75, "BigSplash") })
	m.ctx.At(beat+2, func() {
		s.inst.PlayState(m.animRel, "Idle", beat+2, 0.5)
		s.inst.SetActive(m.throwRel, false)
		s.setState(m, stateRaise, beat+2)
	})
}

func (m *Module) spawnSplash(rootX, beat float64, kind string) {
	if m.splashT == nil {
		return
	}
	inst := m.splashT.NewInstance()
	inst.PlayState("", kind, beat, 0.5)
	sp := &splashInst{inst: inst, beat: beat, kind: kind, rootX: rootX}
	sp.particles = makeDrops(kind, beat)
	m.splashes = append(m.splashes, sp)
}

func makeDrops(kind string, beat float64) []drop {
	count, life, speed, size := 9, 0.45, 3.1, 0.09
	if kind == "Appearsplash" || kind == "BigSplash" {
		count, life, speed, size = 9, 0.6, 4.4, 0.12
	}
	r := rand.New(rand.NewSource(int64(beat*4096) + int64(len(kind))*97))
	out := make([]drop, 0, count)
	for i := 0; i < count; i++ {
		ang := -math.Pi/2 + (r.Float64()-0.5)*math.Pi*0.9
		spd := speed * (0.75 + r.Float64()*0.45)
		out = append(out, drop{
			x: 0.1 + (r.Float64()-0.5)*0.45, y: -3.45 + (r.Float64()-0.5)*0.18,
			vx: math.Cos(ang) * spd, vy: -math.Sin(ang) * spd,
			life: life * (0.75 + r.Float64()*0.45), size: size * (0.7 + r.Float64()*0.6),
		})
	}
	return out
}

func (m *Module) drawDrops(screen *ebiten.Image, beat float64, holder kart.Aff) {
	for _, sp := range m.splashes {
		for _, p := range sp.particles {
			u := clamp01((beat - sp.beat) / p.life)
			if u >= 1 {
				continue
			}
			t := m.ctx.BeatToTime(beat) - m.ctx.BeatToTime(sp.beat)
			x := p.x + p.vx*t
			y := p.y + p.vy*t - 7*t*t
			wx, wy := holder.Apply(sp.rootX+x, y)
			sx, sy := m.proj.Apply(wx, wy)
			a := uint8(230 * (1 - u))
			vector.DrawFilledCircle(screen, float32(sx), float32(sy), float32(p.size*54*(1-u*0.35)), color.NRGBA{255, 255, 255, a}, true)
		}
	}
}

func intParam(e *riq.Entity, key string, fallback int) int {
	if v, ok := e.Data[key]; ok {
		if f, ok := v.(float64); ok {
			return int(f)
		}
	}
	return fallback
}

func boolParam(e *riq.Entity, key string) bool { return boolParamDefault(e, key, false) }

func boolParamDefault(e *riq.Entity, key string, fallback bool) bool {
	if v, ok := e.Data[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return fallback
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
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

func lerp(a, b, t float64) float64 { return a + (b-a)*t }

func easeInOutQuad(a, b, t float64) float64 {
	if t < 0.5 {
		return a + (b-a)*2*t*t
	}
	return a + (b-a)*(1-math.Pow(-2*t+2, 2)/2)
}

func easeOutCubic(a, b, t float64) float64 { return a + (b-a)*(1-math.Pow(1-t, 3)) }
func easeInCubic(a, b, t float64) float64  { return a + (b-a)*t*t*t }
func easeOutQuad(a, b, t float64) float64  { return a + (b-a)*(1-(1-t)*(1-t)) }
func easeInQuad(a, b, t float64) float64   { return a + (b-a)*t*t }
