// Package drummingpractice ports Drumming Practice's bop, drum-hit,
// NPC drummer move, Mii face, and background/streak behavior.
package drummingpractice

import (
	"image/color"
	"math"
	"math/rand"
	"sort"

	"github.com/hajimehoshi/ebiten/v2"

	"hsdemo/engine"
	"hsdemo/kart"
	"hsdemo/riq"
)

var (
	defaultTop    = [4]float64{43.0 / 255.0, 207.0 / 255.0, 51.0 / 255.0, 1}
	defaultBottom = [4]float64{1, 1, 1, 1}
	defaultStreak = [4]float64{1, 247.0 / 255.0, 0, 1}
	miiSprites    = []string{
		"mii_guestA", "mii_guestB", "mii_guestC", "mii_guestD",
		"mii_guestE", "mii_guestF", "mii_matt", "mii_tsunku",
		"mii_marshal", "mii_senior", "mii_tibby", "mii_error",
	}
)

type bopEvt struct {
	beat, length float64
	bop, auto    bool
}

type drumEvt struct {
	beat     float64
	applause bool
}

type miiEvt struct {
	beat                float64
	player, left, right int
	allToPlayer         bool
}

type moveEvt struct {
	beat, length float64
	exit         bool
	ease         int
}

type bgEvt struct {
	beat, length float64
	top0, top1   [4]float64
	bot0, bot1   [4]float64
	streak       [4]float64
	ease         int
}

type drummer struct {
	path, face string
	player     bool
	mii        int
	count      int
	canBopBeat float64
	hitUntil   float64
}

type Module struct {
	ctx  *engine.Ctx
	proj kart.Aff

	player *drummer
	left   *drummer
	right  *drummer
	all    []*drummer

	npcPath      string
	gradientPath string
	streakPaths  []string

	bops  []bopEvt
	drums []drumEvt
	miis  []miiEvt
	moves []moveEvt
	bgs   []bgEvt

	count       int
	endBeat     float64
	hitLockBeat float64

	streakAtT float64
	npcState  string
}

func New() engine.Module {
	return &Module{
		hitLockBeat: 1,
		streakAtT:   math.Inf(-1),
	}
}

func (m *Module) ID() string { return "drummingPractice" }

func (m *Module) Load(ctx *engine.Ctx) error {
	m.ctx = ctx
	if err := ctx.LoadAssets("drummingPractice"); err != nil {
		return err
	}
	m.proj = kart.Translate(engine.ScreenW/2, engine.ScreenH/2).Mul(kart.Scale(54, -54))
	m.npcPath = roleOr(ctx, "NPCDrummers", "NPCDrummers")
	m.gradientPath = roleOr(ctx, "backgroundGradient", "Background/Gradient")
	m.streakPaths = ctx.Assets.Extra.Components["game"].RefArrays["streaks"]
	if len(m.streakPaths) == 0 {
		for i := 0; i < 14; i++ {
			name := "Streak"
			if i > 0 {
				name = name + " (" + itoa(i) + ")"
			}
			m.streakPaths = append(m.streakPaths, "Streaks/"+name)
		}
	}
	m.loadDrummers()
	if d := m.longestHitDuration(); d > 0 {
		m.hitLockBeat = d / 0.6
	}
	m.setMiis(-1, -1, -1, false)
	m.ctx.Scene.PlayDefaultState(m.npcPath, 0, m.ctx.SecPerBeat(0))
	for _, d := range m.all {
		m.ctx.Scene.PlayDefaultState(d.path, 0, m.ctx.SecPerBeat(0))
	}
	return nil
}

func (m *Module) loadDrummers() {
	m.player = &drummer{path: roleOr(m.ctx, "player", "PlayerDrummer"), face: "PlayerDrummer/Body/Head", player: true, canBopBeat: -2}
	m.left = &drummer{path: roleOr(m.ctx, "leftDrummer", "NPCDrummers/LeftDrummerHolder/LeftDrummer"), face: "NPCDrummers/LeftDrummerHolder/LeftDrummer/Body/Head", canBopBeat: -2}
	m.right = &drummer{path: roleOr(m.ctx, "rightDrummer", "NPCDrummers/RightDrummerHolder/RightDrummer"), face: "NPCDrummers/RightDrummerHolder/RightDrummer/Body/Head", canBopBeat: -2}
	m.all = []*drummer{m.player, m.left, m.right}
	for _, c := range m.ctx.Assets.Extra.Components {
		var d *drummer
		switch c.Path {
		case m.player.path:
			d = m.player
		case m.left.path:
			d = m.left
		case m.right.path:
			d = m.right
		}
		if d == nil {
			continue
		}
		if p := c.Refs["face"]; p != "" {
			d.face = p
		}
	}
}

func (m *Module) longestHitDuration() float64 {
	maxDur := 0.0
	for _, name := range []string{"Animations/DrummerHitLeft", "Animations/DrummerHitRight", "DrummerHitLeft", "DrummerHitRight"} {
		if a := m.ctx.Assets.Anims[name]; a != nil && a.Duration > maxDur {
			maxDur = a.Duration
		}
	}
	return maxDur
}

func roleOr(ctx *engine.Ctx, key, fallback string) string {
	if p := ctx.Role(key); p != "" {
		return p
	}
	return fallback
}

func (m *Module) OnEvent(e *riq.Entity) {
	b := e.Beat
	if end := b + e.Length; end > m.endBeat {
		m.endBeat = end
	}
	switch e.Datamodel {
	case "drummingPractice/bop":
		m.bops = append(m.bops, bopEvt{
			beat: b, length: e.Length,
			bop:  boolDefault(e, "bop", true),
			auto: boolParam(e, "autoBop"),
		})
	case "drummingPractice/drum":
		m.drums = append(m.drums, drumEvt{beat: b, applause: boolDefault(e, "toggle", true)})
	case "drummingPractice/set mii":
		m.miis = append(m.miis, miiEvt{
			beat: b, player: int(e.Float("type", -1)), left: int(e.Float("type2", -1)), right: int(e.Float("type3", -1)),
			allToPlayer: boolParam(e, "toggle"),
		})
	case "drummingPractice/move npc drummers":
		m.moves = append(m.moves, moveEvt{
			beat: b, length: e.Length,
			exit: boolParam(e, "exit"),
			ease: int(e.Float("ease", 0)),
		})
	case "drummingPractice/set background color":
		m.bgs = append(m.bgs, bgEvt{
			beat: b, length: e.Length,
			top0:   colorParam(e, "colorAStart", defaultTop),
			top1:   colorParam(e, "colorA", defaultTop),
			bot0:   colorParam(e, "colorBStart", defaultBottom),
			bot1:   colorParam(e, "colorB", defaultBottom),
			streak: colorParam(e, "colorC", defaultStreak),
			ease:   int(e.Float("ease", 0)),
		})
	}
}

func (m *Module) Ready() {
	sort.Slice(m.bops, func(i, j int) bool { return m.bops[i].beat < m.bops[j].beat })
	sort.Slice(m.drums, func(i, j int) bool { return m.drums[i].beat < m.drums[j].beat })
	sort.Slice(m.miis, func(i, j int) bool { return m.miis[i].beat < m.miis[j].beat })
	sort.Slice(m.moves, func(i, j int) bool { return m.moves[i].beat < m.moves[j].beat })
	sort.Slice(m.bgs, func(i, j int) bool { return m.bgs[i].beat < m.bgs[j].beat })
	for _, ev := range m.miis {
		ev := ev
		m.ctx.At(ev.beat, func() { m.setMiis(ev.player, ev.left, ev.right, ev.allToPlayer) })
	}
	for _, ev := range m.drums {
		ev := ev
		m.ctx.At(ev.beat, func() { m.prepare(ev.beat, ev.applause) })
		m.ctx.At(ev.beat+1, func() {
			m.ctx.Sound("drum")
			m.hit(m.left, true, false, false, ev.beat+1)
			m.hit(m.right, true, false, false, ev.beat+1)
		})
		m.ctx.ScheduleInput(ev.beat+1,
			func(state float64, _ engine.Judgment) { m.just(ev.beat, state, ev.applause) },
			func() { m.miss(ev.beat) })
	}
	m.scheduleBops()
}

func (m *Module) scheduleBops() {
	for i, ev := range m.bops {
		if !ev.bop {
			continue
		}
		end := ev.beat + ev.length
		if ev.auto {
			end = m.autoBopEnd(i)
		}
		for b := ev.beat; b < end-1e-6; b++ {
			bb := b
			m.ctx.At(bb, func() { m.bop(bb) })
		}
	}
}

func (m *Module) autoBopEnd(i int) float64 {
	ev := m.bops[i]
	end := math.Max(ev.beat+ev.length, m.endBeat+4)
	if sw := m.ctx.NextSwitchBeat(ev.beat); !math.IsInf(sw, 1) {
		end = sw
	}
	for j := i + 1; j < len(m.bops); j++ {
		if m.bops[j].beat > ev.beat && m.bops[j].beat < end {
			end = m.bops[j].beat
			break
		}
	}
	return end
}

func (m *Module) OnSwitch(beat float64) {
	for _, ev := range m.miis {
		if ev.beat > beat {
			break
		}
		m.setMiis(ev.player, ev.left, ev.right, ev.allToPlayer)
	}
}

func (m *Module) Whiff(beat float64) {
	m.hit(m.player, false, false, false, beat)
}

func (m *Module) Update(t, beat float64) {
	for _, d := range m.all {
		if d.hitUntil > 0 && beat >= d.hitUntil {
			d.hitUntil = 0
		}
	}
	m.updateNPCMove(beat)
}

func (m *Module) Draw(screen *ebiten.Image, t, beat float64) {
	top, bottom, streak := m.colorsAt(beat)
	screen.Fill(toRGBA(bottom))
	m.ctx.Scene.SetColorOver(m.gradientPath, top)
	alpha := m.streakAlpha(t)
	for _, p := range m.streakPaths {
		c := streak
		c[3] = alpha
		m.ctx.Scene.SetColorOver(p, c)
	}
	m.ctx.SampleScene(beat)
	m.ctx.Scene.Draw(screen, m.proj)
}

func (m *Module) prepare(beat float64, _ bool) {
	typ := m.count % 2
	for _, d := range m.all {
		d.prepare(m.ctx, beat, typ)
	}
	m.count++
	m.setFaces(0)
	m.ctx.Sound("prepare")
}

func (d *drummer) prepare(ctx *engine.Ctx, beat float64, typ int) {
	d.canBopBeat = beat
	d.count = typ
	timeScale := ctx.SecPerBeat(beat)
	if typ%2 == 0 {
		ctx.Scene.PlayState(d.path, "PrepareLeft", beat, timeScale)
	} else {
		ctx.Scene.PlayState(d.path, "PrepareRight", beat, timeScale)
	}
}

func (m *Module) just(startBeat, state float64, applause bool) {
	if state >= 1 || state <= -1 {
		// DrummerHit.Just intentionally falls through after Hit(false);
		// barely inputs therefore play the miss feedback and then the success
		// feedback, matching the shipped C# rather than "fixing" the branch.
		m.playerHitResult(false, applause, startBeat)
	}
	m.playerHitResult(true, applause, startBeat)
}

func (m *Module) miss(startBeat float64) {
	m.setFaces(2)
	m.ctx.At(startBeat+2, func() { m.setFaces(0) })
}

func (m *Module) playerHitResult(ok bool, applause bool, startBeat float64) {
	m.hit(m.player, ok, applause, true, m.ctx.Beat())
	if ok {
		m.setFaces(1)
		return
	}
	m.setFaces(2)
	m.ctx.At(startBeat+2, func() { m.setFaces(0) })
}

func (m *Module) hit(d *drummer, ok, applause, force bool, beat float64) {
	if d == nil {
		return
	}
	if d.player && force {
		if ok {
			m.hitSound(applause)
			m.streak()
		} else {
			m.missSound()
		}
	}
	if d.hitUntil > 0 && beat < d.hitUntil {
		return
	}
	state := "HitLeft"
	if d.count%2 != 0 {
		state = "HitRight"
	}
	m.ctx.Scene.PlayState(d.path, state, beat, 0.6)
	d.count++
	d.hitUntil = beat + m.hitLockBeat
	if d.player && !force {
		if ok {
			m.hitSound(applause)
		} else {
			m.missSound()
		}
	}
}

func (m *Module) hitSound(applause bool) {
	m.ctx.Sound("hit")
	if applause {
		m.ctx.Sound("common_applause")
	}
}

func (m *Module) missSound() {
	m.ctx.Sound("miss")
}

func (m *Module) streak() {
	m.streakAtT = m.ctx.Time()
}

func (m *Module) bop(beat float64) {
	for _, d := range m.all {
		if beat > d.canBopBeat+2 {
			m.ctx.Scene.PlayState(d.path, "Bop", beat, m.ctx.SecPerBeat(beat))
		}
	}
}

func (m *Module) setFaces(face int) {
	for _, d := range m.all {
		m.ctx.Scene.SetSpriteOver(d.face, miiFaceSprite(d.mii, face))
	}
}

func (m *Module) setMiis(playerFace, leftFace, rightFace int, all bool) {
	playerFace = normalizeMii(playerFace)
	leftFace = normalizeMii(leftFace)
	rightFace = normalizeMii(rightFace)
	if playerFace < 0 {
		m.player.mii = randomMii(leftFace, rightFace)
	} else {
		m.player.mii = playerFace
	}
	if all && playerFace >= 0 {
		m.left.mii = playerFace
		m.right.mii = playerFace
	} else {
		if leftFace < 0 {
			m.left.mii = randomMii(m.player.mii)
		} else {
			m.left.mii = leftFace
		}
		if rightFace < 0 {
			m.right.mii = randomMii(m.player.mii, m.left.mii)
		} else {
			m.right.mii = rightFace
		}
	}
	m.setFaces(0)
}

func normalizeMii(v int) int {
	if v < 0 {
		return -1
	}
	if v >= len(miiSprites) {
		return len(miiSprites) - 1
	}
	return v
}

func randomMii(exclude ...int) int {
	for tries := 0; tries < 64; tries++ {
		v := rand.Intn(len(miiSprites) - 1) // Unity Random.Range(0, Count-1) excludes Error.
		found := false
		for _, ex := range exclude {
			if v == ex {
				found = true
				break
			}
		}
		if !found {
			return v
		}
	}
	return 0
}

func miiFaceSprite(mii, face int) string {
	mii = normalizeMii(mii)
	if mii < 0 {
		mii = 0
	}
	base := miiSprites[mii]
	switch face {
	case 1:
		return base + "_happy"
	case 2:
		return base + "_sad"
	default:
		return base
	}
}

func (m *Module) updateNPCMove(beat float64) {
	clip, norm, moving, final := m.npcAt(beat)
	if moving {
		m.ctx.Scene.PlayNormalized(m.npcPath, clip, norm)
		m.npcState = "moving:" + clip
		return
	}
	if final != "" && final != m.npcState {
		m.ctx.Scene.PlayState(m.npcPath, final, beat, m.ctx.SecPerBeat(beat))
		m.npcState = final
	}
}

func (m *Module) npcAt(beat float64) (clip string, norm float64, moving bool, final string) {
	final = "NPCDrummersEntered"
	for _, ev := range m.moves {
		if beat < ev.beat {
			break
		}
		enterClip, endState := "NPCDrummersEnter", "NPCDrummersEntered"
		if ev.exit {
			enterClip, endState = "NPCDrummersExit", "NPCDrummersExited"
		}
		if ev.length > 0 && beat < ev.beat+ev.length {
			u := (beat - ev.beat) / ev.length
			return enterClip, engine.Ease(ev.ease, 0, 1, u), true, ""
		}
		final = endState
	}
	return "", 0, false, final
}

func (m *Module) colorsAt(beat float64) (top, bottom, streak [4]float64) {
	ev := bgEvt{top0: defaultTop, top1: defaultTop, bot0: defaultBottom, bot1: defaultBottom, streak: defaultStreak}
	for _, bg := range m.bgs {
		if bg.beat > beat {
			break
		}
		ev = bg
	}
	u := 1.0
	if ev.length > 0 && beat < ev.beat+ev.length {
		u = (beat - ev.beat) / ev.length
	}
	for i := 0; i < 4; i++ {
		top[i] = engine.Ease(ev.ease, ev.top0[i], ev.top1[i], u)
		bottom[i] = engine.Ease(ev.ease, ev.bot0[i], ev.bot1[i], u)
	}
	streak = ev.streak
	return top, bottom, streak
}

func (m *Module) streakAlpha(t float64) float64 {
	if math.IsInf(m.streakAtT, -1) {
		return 0
	}
	return 0.7 * math.Exp(-3.5*math.Max(0, t-m.streakAtT))
}

func boolParam(e *riq.Entity, key string) bool { return e.Float(key, 0) != 0 }

func boolDefault(e *riq.Entity, key string, def bool) bool {
	if _, ok := e.Data[key]; !ok {
		return def
	}
	return boolParam(e, key)
}

func colorParam(e *riq.Entity, key string, def [4]float64) [4]float64 {
	v, ok := e.Data[key]
	if !ok {
		return def
	}
	m, ok := v.(map[string]any)
	if !ok {
		return def
	}
	return [4]float64{
		num(m["r"], def[0]), num(m["g"], def[1]), num(m["b"], def[2]), num(m["a"], def[3]),
	}
}

func num(v any, def float64) float64 {
	if f, ok := v.(float64); ok {
		return f
	}
	return def
}

func toRGBA(c [4]float64) color.NRGBA {
	return color.NRGBA{
		R: uint8(clamp01(c[0]) * 255),
		G: uint8(clamp01(c[1]) * 255),
		B: uint8(clamp01(c[2]) * 255),
		A: uint8(clamp01(c[3]) * 255),
	}
}

func clamp01(v float64) float64 {
	return math.Max(0, math.Min(1, v))
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [12]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
