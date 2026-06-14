// Package launchparty ports Launch Party's launch-pad movement, pitched rocket
// cues, count-number display, miss/barely feedback, and launch effects.
package launchparty

import (
	"math"
	"math/rand"
	"sort"

	"github.com/hajimehoshi/ebiten/v2"

	"hsdemo/engine"
	"hsdemo/kart"
	"hsdemo/kmdata"
	"hsdemo/riq"
)

const (
	padMovePath   = "LaunchPadMove"
	padRotatePath = "LaunchPadMove/LaunchPad"
	padSpritePath = "LaunchPadMove/LaunchPad/Sprite"
	rocketScale   = 0.05
)

type rocketType int

const (
	rocketFamily rocketType = iota
	rocketCracker
	rocketBell
	rocketBowling
)

type rocketCue struct {
	beat, offset float64
	typ          rocketType
	notes        []int
}

type posMove struct {
	beat, length float64
	x, y         float64
	ease         int
}

type rotMove struct {
	beat, length float64
	rot          float64
	ease         int
}

type rocketInst struct {
	cue        rocketCue
	state      string
	stateBeat  float64
	stateScale float64

	numberActive bool
	numberState  string
	numberBeat   float64
	numberScale  float64

	noInput bool
	barely  bool
	deadAt  float64
}

type Module struct {
	ctx  *engine.Ctx
	proj kart.Aff

	cues     []rocketCue
	rockets  []*rocketInst
	posMoves []posMove
	rotMoves []rotMove

	padIdx int
}

func New() engine.Module { return &Module{padIdx: -1} }

func (m *Module) ID() string { return "launchParty" }

func (m *Module) Load(ctx *engine.Ctx) error {
	m.ctx = ctx
	if err := ctx.LoadAssets("launchParty"); err != nil {
		return err
	}
	m.proj = kart.Translate(engine.ScreenW/2, engine.ScreenH/2).Mul(kart.Scale(54, -54))
	if idx, ok := ctx.Scene.Index(padRotatePath); ok {
		m.padIdx = idx
	}
	m.playDefaults(0)
	return nil
}

func (m *Module) OnEvent(e *riq.Entity) {
	switch e.Datamodel {
	case "launchParty/rocket":
		m.cues = append(m.cues, rocketCue{beat: e.Beat, offset: e.Float("offset", -1), typ: rocketFamily, notes: notes(e, []int{2, 4, 5, 7})})
	case "launchParty/partyCracker":
		m.cues = append(m.cues, rocketCue{beat: e.Beat, offset: e.Float("offset", -1), typ: rocketCracker, notes: notes(e, []int{4, 5, 7, 9, 11, 12})})
	case "launchParty/bell":
		m.cues = append(m.cues, rocketCue{beat: e.Beat, offset: e.Float("offset", -1), typ: rocketBell, notes: notes(e, []int{0, 2, 4, 5, 7, 9, 11, 12, 0})})
	case "launchParty/bowlingPin":
		m.cues = append(m.cues, rocketCue{beat: e.Beat, offset: e.Float("offset", -1), typ: rocketBowling, notes: notes(e, []int{5, -1, 0, -1, 0, -1, 0, -1, 0, -1, 0, -1, 0, 7, 7})})
	case "launchParty/posMove":
		m.posMoves = append(m.posMoves, posMove{
			beat: e.Beat, length: e.Length,
			x: e.Float("xPos", 0), y: e.Float("yPos", 0),
			ease: int(e.Float("ease", 0)),
		})
	case "launchParty/rotMove":
		m.rotMoves = append(m.rotMoves, rotMove{beat: e.Beat, length: e.Length, rot: e.Float("rot", 0), ease: int(e.Float("ease", 0))})
	case "launchParty/toggleStars", "launchParty/scrollSpeed":
		// These loader actions are hidden in Heaven Studio and their functions are
		// commented out in LaunchParty.cs, so the original runtime performs no work.
	}
}

func (m *Module) Ready() {
	sort.SliceStable(m.cues, func(i, j int) bool { return m.cues[i].beat < m.cues[j].beat })
	sort.SliceStable(m.posMoves, func(i, j int) bool { return m.posMoves[i].beat < m.posMoves[j].beat })
	sort.SliceStable(m.rotMoves, func(i, j int) bool { return m.rotMoves[i].beat < m.rotMoves[j].beat })
	for _, cue := range m.cues {
		m.scheduleRocket(cue)
	}
}

func (m *Module) OnSwitch(beat float64) {
	m.playDefaults(beat)
	m.ctx.Scene.SetPosOver(padMovePath, 0, -2.4)
	if m.padIdx >= 0 {
		m.ctx.Scene.SetSpinIdx(m.padIdx, 0)
	}
	m.rockets = liveRockets(m.rockets, beat)
}

func (m *Module) playDefaults(beat float64) {
	sec := m.ctx.SecPerBeat(beat)
	for _, p := range []string{"Background/Canvas/Earth/Glare", padRotatePath, padSpritePath} {
		m.ctx.Scene.PlayDefaultState(p, beat, sec)
	}
}

func (m *Module) scheduleRocket(c rocketCue) {
	r := &rocketInst{
		cue: c, state: "RocketHidden", stateBeat: c.beat, stateScale: m.ctx.SecPerBeat(c.beat),
		noInput: true,
	}
	m.ctx.At(c.beat, func() { m.rockets = append(m.rockets, r) })
	m.ctx.At(c.beat+c.offset, func() {
		r.state, r.stateBeat, r.stateScale = "RocketRise", c.beat+c.offset, 0.5
		r.noInput = false
	})

	switch c.typ {
	case rocketFamily:
		m.scheduleFamily(r)
	case rocketCracker:
		m.scheduleCracker(r)
	case rocketBell:
		m.scheduleBell(r)
	case rocketBowling:
		m.scheduleBowling(r)
	}
}

func (m *Module) scheduleFamily(r *rocketInst) {
	b := r.cue.beat
	m.soundPitchAt(b, "rocket_prepare", 0)
	m.soundPitchAt(b, "rocket_note", r.cue.notes[0])
	m.soundPitchAt(b+1, "rocket_note", r.cue.notes[1])
	m.soundPitchAt(b+2, "rocket_note", r.cue.notes[2])
	m.numberAt(r, b, "CountThree")
	m.numberAt(r, b+1, "CountTwo")
	m.numberAt(r, b+2, "CountOne")
	m.scheduleInput(r, b+3)
}

func (m *Module) scheduleCracker(r *rocketInst) {
	b := r.cue.beat
	m.soundPitchAt(b, "rocket_prepare", 0)
	for i, off := range []float64{0, 0.66, 1, 1.33, 1.66} {
		m.soundPitchAt(b+off, "popper_note", r.cue.notes[i])
	}
	for _, ev := range []struct {
		off   float64
		state string
	}{{0, "CountFive"}, {0.66, "CountFour"}, {1, "CountThree"}, {1.33, "CountTwo"}, {1.66, "CountOne"}} {
		m.numberAt(r, b+ev.off, ev.state)
	}
	m.scheduleInput(r, b+2)
}

func (m *Module) scheduleBell(r *rocketInst) {
	b := r.cue.beat
	m.soundPitchAt(b, "rocket_prepare", 0)
	m.soundPitchAt(b, "bell_note", r.cue.notes[0])
	for i, off := range []float64{1, 1.16, 1.33, 1.5, 1.66, 1.83} {
		m.soundPitchAt(b+off, "bell_short", r.cue.notes[i+1])
	}
	for _, ev := range []struct {
		off   float64
		state string
	}{{0, "CountSeven"}, {1, "CountSix"}, {1.16, "CountFive"}, {1.33, "CountFour"}, {1.5, "CountThree"}, {1.66, "CountTwo"}, {1.83, "CountOne"}} {
		m.numberAt(r, b+ev.off, ev.state)
	}
	m.scheduleInput(r, b+2)
}

func (m *Module) scheduleBowling(r *rocketInst) {
	b := r.cue.beat
	m.soundPitchAt(b, "rocket_pin_prepare", 0)
	m.soundPitchAt(b, "pin", r.cue.notes[0])
	for i, ev := range []struct {
		off, skip float64
	}{{0, 0.02}, {0.16, 0.02}, {0.33, 0.06}, {0.5, 0.1}, {0.66, 0.16}, {0.83, 0.22}, {1, 0.3}, {1.16, 0.4}, {1.33, 0.6}, {1.5, 0.75}, {1.66, 0.89}, {1.83, 0}} {
		m.ctx.SoundAtPitchOff(b+ev.off, "flute", 1, pitch(r.cue.notes[i+1]), ev.skip)
	}
	m.numberAt(r, b, "CountOne")
	m.scheduleInput(r, b+2)
}

func (m *Module) scheduleInput(r *rocketInst, target float64) {
	m.ctx.ScheduleInputCond(target,
		func() bool { return m.ctx.GameAt(target) == m.ID() },
		func(state float64, _ engine.Judgment) { m.hitRocket(r, target, state) },
		func() { m.missRocket(r, target) },
	)
}

func (m *Module) hitRocket(r *rocketInst, beat, state float64) {
	r.noInput = true
	if r.barely {
		r.numberActive = false
		r.deadAt = beat + 1
		return
	}
	if state >= 1 || state <= -1 {
		m.badRocket(r, beat)
		return
	}
	m.successRocket(r, beat)
}

func (m *Module) successRocket(r *rocketInst, beat float64) {
	r.state, r.stateBeat, r.stateScale = "RocketLaunch", beat, m.ctx.SecPerBeat(beat)
	r.numberActive, r.numberState, r.numberBeat, r.numberScale = true, "CountImpact", beat, 0.5
	r.deadAt = beat + 1
	m.ctx.Scene.PlayState(padSpritePath, "SizeUp", beat, 1)
	switch r.cue.typ {
	case rocketFamily:
		m.soundPitchNow("rocket_note", r.cue.notes[3])
		m.ctx.Sound("rocket_family")
	case rocketCracker:
		m.soundPitchNow("popper_note", r.cue.notes[5])
		m.ctx.Sound("rocket_crackerblast")
	case rocketBell:
		m.soundPitchNow("bell_note", r.cue.notes[7])
		m.soundPitchNow("bell_blast", r.cue.notes[8])
	case rocketBowling:
		m.ctx.SoundPitchOff("flute", 1, pitch(r.cue.notes[13]), 0.89)
		m.soundPitchNow("pin", r.cue.notes[14])
		m.ctx.Sound("rocket_bowling")
	}
}

func (m *Module) missRocket(r *rocketInst, beat float64) {
	r.noInput = true
	r.numberActive = false
	r.state, r.stateBeat, r.stateScale = "RocketMiss", beat, m.ctx.SecPerBeat(beat)
	r.deadAt = beat + 1
	m.ctx.Sound("miss")
}

func (m *Module) badRocket(r *rocketInst, beat float64) {
	r.noInput = true
	r.numberActive = false
	r.barely = true
	r.state = "RocketBarelyLeft"
	if rand.Intn(2) == 0 {
		r.state = "RocketBarelyRight"
	}
	r.stateBeat, r.stateScale = beat, m.ctx.SecPerBeat(beat)
	r.deadAt = beat + 1
	m.ctx.Sound("miss")
	m.ctx.Sound("rocket_endBad")
}

func (m *Module) Whiff(beat float64) {
	for _, r := range m.rockets {
		if r.noInput || r.deadAt > 0 || r.barely {
			continue
		}
		m.badRocket(r, beat)
		m.ctx.ScoreMiss()
		return
	}
}

func (m *Module) Update(_, beat float64) {
	m.rockets = liveRockets(m.rockets, beat)
}

func (m *Module) Draw(screen *ebiten.Image, _, beat float64) {
	screen.Fill(colorA59FAB())
	sc := m.ctx.Scene
	x, y := m.padPosAt(beat)
	sc.SetPosOver(padMovePath, x, y)
	if m.padIdx >= 0 {
		sc.SetSpinIdx(m.padIdx, m.padRotAt(beat)*math.Pi/180)
	}
	m.ctx.SampleScene(beat)
	if root, ok := sc.NodeWorld(padRotatePath); ok {
		for _, r := range m.rockets {
			m.drawRocket(sc, root.Mul(kart.Scale(rocketScale, rocketScale)), r, beat)
		}
	}
	sc.Draw(screen, m.proj)
}

func (m *Module) drawRocket(sc *kart.SceneInst, root kart.Aff, r *rocketInst, beat float64) {
	anim := m.ctx.Assets.Anims["Animations/"+r.state]
	if anim == nil {
		return
	}
	at := clipAt(anim, beat, r.stateBeat, r.stateScale)
	rocketWorld := root.Mul(nodeTRS(anim, "Rocket", at))
	if active(anim, "Rocket/RocketSprite", at, true) {
		sc.Queue(kart.ExtraSprite{Sprite: rocketSprite(r.cue.typ), World: rocketWorld, Order: order(anim, "Rocket/RocketSprite", at, 8), Tint: [4]float64{1, 1, 1, 1}})
	}
	for _, part := range []struct {
		path, sprite string
		order        int
	}{{"Smoke0", "Base Colors_2", 3}, {"Smoke1", "Base Colors_2", 3}, {"Smear", "Smear", 9}, {"Boom", "Boom", 11}} {
		if !active(anim, part.path, at, false) {
			continue
		}
		sprite := part.sprite
		if p := kart.SampleClipNode(anim, part.path, at); p.HasSprite {
			sprite = p.Sprite
		}
		if sprite == "" {
			continue
		}
		flipX := false
		if v, ok := kart.SampleClipFloat(anim, part.path, "m_FlipX", at); ok {
			flipX = v > 0.5
		}
		sc.Queue(kart.ExtraSprite{Sprite: sprite, World: root.Mul(nodeTRS(anim, part.path, at)), Order: order(anim, part.path, at, part.order), FlipX: flipX, Tint: [4]float64{1, 1, 1, 1}})
	}
	if r.numberActive {
		m.drawNumber(sc, root, r, beat)
	}
}

func (m *Module) drawNumber(sc *kart.SceneInst, root kart.Aff, r *rocketInst, beat float64) {
	anim := m.ctx.Assets.Anims["Animations/"+r.numberState]
	if anim == nil {
		return
	}
	at := clipAt(anim, beat, r.numberBeat, r.numberScale)
	pose := kart.SampleClipNode(anim, "", at)
	if !pose.HasSprite || pose.Sprite == "" {
		return
	}
	alpha := 1.0
	if v, ok := kart.SampleClipFloat(anim, "", "m_Color.a", at); ok {
		alpha = v
	}
	sc.Queue(kart.ExtraSprite{
		Sprite: pose.Sprite,
		World:  root.Mul(kart.Translate(-0.0035, 0.016)),
		Order:  12,
		Tint:   [4]float64{1, 1, 1, alpha},
	})
}

func (m *Module) numberAt(r *rocketInst, beat float64, state string) {
	m.ctx.At(beat, func() {
		r.numberActive = true
		r.numberState, r.numberBeat, r.numberScale = state, beat, m.ctx.SecPerBeat(beat)
	})
}

func (m *Module) soundPitchAt(beat float64, name string, semitone int) {
	m.ctx.SoundAtPitchOff(beat, name, 1, pitch(semitone), 0)
}

func (m *Module) soundPitchNow(name string, semitone int) {
	m.ctx.SoundPitch(name, 1, pitch(semitone))
}

func (m *Module) padPosAt(beat float64) (float64, float64) {
	x, y := 0.0, -2.4
	for _, ev := range m.posMoves {
		if beat < ev.beat {
			break
		}
		fromX, fromY := x, y
		toX, toY := ev.x, ev.y
		if ev.length <= 0 || beat >= ev.beat+ev.length {
			x, y = toX, toY
			continue
		}
		u := (beat - ev.beat) / ev.length
		x = engine.Ease(ev.ease, fromX, toX, u)
		y = engine.Ease(ev.ease, fromY, toY, u)
	}
	return x, y
}

func (m *Module) padRotAt(beat float64) float64 {
	rot := 0.0
	for _, ev := range m.rotMoves {
		if beat < ev.beat {
			break
		}
		from, to := rot, ev.rot
		if ev.length <= 0 || beat >= ev.beat+ev.length {
			rot = to
			continue
		}
		rot = engine.Ease(ev.ease, from, to, (beat-ev.beat)/ev.length)
	}
	return rot
}

func nodeTRS(anim *kmdata.Anim, path string, at float64) kart.Aff {
	p := kart.SampleClipNode(anim, path, at)
	x, y := 0.0, 0.0
	if p.HasPos[0] {
		x = p.Pos[0]
	}
	if p.HasPos[1] {
		y = p.Pos[1]
	}
	sx, sy := 1.0, 1.0
	if p.HasScale[0] {
		sx = p.Scale[0]
	}
	if p.HasScale[1] {
		sy = p.Scale[1]
	}
	rot := 0.0
	if p.HasRot {
		rot = p.RotDeg * math.Pi / 180
	}
	return kart.TRS(x, y, rot, sx, sy)
}

func clipAt(anim *kmdata.Anim, beat, start, scale float64) float64 {
	if anim == nil || scale <= 0 {
		return 0
	}
	at := (beat - start) * scale
	if at < 0 {
		return 0
	}
	if anim.Loop && anim.Duration > 0 {
		return math.Mod(at, anim.Duration)
	}
	if at > anim.Duration {
		return anim.Duration
	}
	return at
}

func active(anim *kmdata.Anim, path string, at float64, def bool) bool {
	if v, ok := kart.SampleClipFloat(anim, path, "m_IsActive", at); ok {
		return v > 0.5
	}
	if v, ok := kart.SampleClipFloat(anim, path, "m_Enabled", at); ok {
		return v > 0.5
	}
	return def
}

func order(anim *kmdata.Anim, path string, at float64, def int) int {
	if v, ok := kart.SampleClipFloat(anim, path, "m_SortingOrder", at); ok {
		return int(v)
	}
	return def
}

func liveRockets(in []*rocketInst, beat float64) []*rocketInst {
	out := in[:0]
	for _, r := range in {
		if r.deadAt > 0 && beat >= r.deadAt {
			continue
		}
		out = append(out, r)
	}
	return out
}

func rocketSprite(t rocketType) string {
	switch t {
	case rocketCracker:
		return "PartyCracker"
	case rocketBell:
		return "Bell"
	case rocketBowling:
		return "BowlingPin"
	default:
		return "FamilyRocket"
	}
}

func pitch(semitone int) float64 { return math.Pow(2, float64(semitone)/12) }

func notes(e *riq.Entity, defaults []int) []int {
	out := append([]int(nil), defaults...)
	for i := range out {
		key := "note" + itoa(i+1)
		if v, ok := e.Data[key].(float64); ok {
			out[i] = int(v)
		}
	}
	return out
}

func itoa(v int) string {
	if v < 10 {
		return string(byte('0' + v))
	}
	return "1" + string(byte('0'+v-10))
}

func colorA59FAB() colorRGBA { return colorRGBA{0xA5, 0x9F, 0xAB, 0xFF} }

type colorRGBA struct{ R, G, B, A uint8 }

func (c colorRGBA) RGBA() (r, g, b, a uint32) {
	return uint32(c.R) * 0x101, uint32(c.G) * 0x101, uint32(c.B) * 0x101, uint32(c.A) * 0x101
}
