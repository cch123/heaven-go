// Package spaceball ports Spaceball's pitched balls, batter swing, alien
// umpire, costume palettes, camera zoom, and background color controls.
package spaceball

import (
	"image/color"
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
	ballBaseball = iota
	ballOnigiri
	ballAlien
	ballTacobell
	ballApple
	ballStar
)

const (
	costumeStandard = iota
	costumeBunny
	costumeSphereHead
	costumeStandardCyan
	costumeOrangeAlien
	costumeSquareHead
	costumeCustom
)

var (
	defaultBG   = [4]float64{0, 0, 0.4509804, 1}
	defaultRoom = [4]float64{0, 0.8588235, 0.07058824, 1}
	white       = [4]float64{1, 1, 1, 1}
	stageFill   = color.NRGBA{R: 0x00, G: 0x00, B: 0x73, A: 0xff}
)

type bgEvt struct {
	beat, length       float64
	bgStart, bgEnd     [4]float64
	roomStart, roomEnd [4]float64
	ease               int
}

type camEvt struct {
	beat, length float64
	dist         float64
	ease         int
}

type bgEase struct {
	beat, length float64
	from, to     [4]float64
	ease         int
}

type ball struct {
	start, hitBeat float64
	kind           int
	high, quick    bool
	offbeat        bool
	isTacobell     bool
	hit, near      bool
	dead           bool
	hitPos         [3]float64
	hitRot         float64
	endX           float64
	startRot       float64
	sprite         string
	scale          float64
}

type Module struct {
	ctx  *engine.Ctx
	proj kart.Aff

	ballRoot string
	disp     string
	dust     string
	alien    string
	bg       string
	square   string
	room     string
	hole     string
	shadow   string
	shadow2  string
	player   string
	playerSR string
	hat      string
	bat      string

	ballSprites []string
	curves      map[string]kmdata.Curve
	balls       []*ball
	bgEvents    []bgEvt
	camEvents   []camEvt
	bgEase      bgEase
	roomEase    bgEase

	costumeMats []string
	costume     int
	mask        int
	batColors   [][4]float64
	batColor    [4]float64
	customPal   kart.Palette

	alienShowBeat float64
	alienShowing  bool
	alienHiding   bool
}

func New() engine.Module { return &Module{} }

func (m *Module) ID() string { return "spaceball" }

func (m *Module) Load(ctx *engine.Ctx) error {
	m.ctx = ctx
	if err := ctx.LoadAssets("spaceball"); err != nil {
		return err
	}
	m.proj = kart.Translate(engine.ScreenW/2, engine.ScreenH/2).Mul(kart.Scale(54, -54))
	game := ctx.Assets.Extra.Components["game"]
	player := ctx.Assets.Extra.Components["player"]
	m.ballRoot = roleOr(ctx, "Ball", game.Refs["Ball"], "Balls/Ball")
	m.disp = roleOr(ctx, "Dispenser", game.Refs["Dispenser"], "Dispenser")
	m.dust = roleOr(ctx, "Dust", game.Refs["Dust"], "Dust")
	m.alien = roleOr(ctx, "alien", game.Refs["alien"], "Alien")
	m.bg = roleOr(ctx, "bg", game.Refs["bg"], "SpaceBG")
	m.square = roleOr(ctx, "square", game.Refs["square"], "RoomBG/Square")
	m.room = roleOr(ctx, "room", game.Refs["room"], "RoomBG")
	m.hole = roleOr(ctx, "hole", game.Refs["hole"], "Hole")
	m.shadow = roleOr(ctx, "shadow", game.Refs["shadow"], "Player/playershadow")
	m.shadow2 = roleOr(ctx, "shadow2", game.Refs["shadow2"], "Alien/alien_shadow")
	m.player = player.Path
	if m.player == "" {
		m.player = "Player"
	}
	m.playerSR = player.Refs["PlayerSprite"]
	m.hat = player.Refs["Hat"]
	m.bat = player.Refs["Bat"]
	m.ballSprites = append([]string(nil), game.SpriteArrays["BallSprites"]...)
	m.ballSprites = normalizeBallSprites(m.ballSprites)
	m.curves = ctx.Assets.Extra.Curves
	m.costumeMats = append([]string(nil), game.RefArrays["CostumeColors"]...)
	m.batColors = readBatColors(player.Lists["BatColors"])
	m.bgEase = bgEase{from: defaultBG, to: defaultBG}
	m.roomEase = bgEase{from: defaultRoom, to: defaultRoom}
	m.customPal = kart.Palette{Alpha: [4]float64{0, 0, 0, 1}, Fill: defaultRoom, Outline: white}
	m.costume = costumeStandard
	m.mask = costumeStandard
	if len(m.batColors) > 0 {
		m.batColor = m.batColors[0]
	} else {
		m.batColor = [4]float64{0.80784315, 0.7411765, 0, 1}
	}
	m.applyCostumePalette()
	m.resetScene(0)
	return nil
}

func roleOr(ctx *engine.Ctx, role, comp, fallback string) string {
	if p := ctx.Role(role); p != "" {
		return p
	}
	if comp != "" {
		return comp
	}
	return fallback
}

func (m *Module) OnEvent(e *riq.Entity) {
	b := e.Beat
	switch e.Datamodel {
	case "spaceball/shoot":
		m.scheduleShoot(b, false, false, false, intParam(e, "type", ballBaseball))
	case "spaceball/shootFast":
		m.scheduleShoot(b, false, true, false, intParam(e, "type", ballBaseball))
	case "spaceball/shootHigh":
		m.scheduleShoot(b, true, false, false, intParam(e, "type", ballBaseball))
	case "spaceball/shootOff":
		m.scheduleShoot(b, false, false, true, intParam(e, "type", ballBaseball))
	case "spaceball/prepare dispenser":
		m.ctx.At(b, func() { m.prepareDispenser(b) })
	case "spaceball/costume":
		typ, mask := intParam(e, "type", costumeStandard), intParam(e, "Mask", 0)
		skin := colorParam(e, "SkinColor", defaultRoom)
		clothes := colorParam(e, "ClothesColor", white)
		clothes2 := colorParam(e, "ClothesColor2", [4]float64{0, 0, 0, 1})
		bat := colorParam(e, "BatColor", [4]float64{0.80784315, 0.7411765, 0, 1})
		m.ctx.At(b, func() { m.setCostume(typ, mask, skin, clothes, clothes2, bat) })
	case "spaceball/alien":
		hide := boolParam(e, "hide")
		m.ctx.At(b, func() { m.showAlien(b, hide) })
	case "spaceball/camera":
		dist := -e.Float("valA", 10)
		if dist > 0 {
			dist = 0
		}
		m.camEvents = append(m.camEvents, camEvt{beat: b, length: e.Length, dist: dist, ease: int(e.Float("ease", 0))})
	case "spaceball/fade background":
		ev := bgEvt{
			beat: b, length: e.Length,
			bgStart:   colorParam(e, "colorStart", defaultBG),
			bgEnd:     colorParam(e, "colorEnd", defaultBG),
			roomStart: colorParam(e, "startRoomColor", defaultRoom),
			roomEnd:   colorParam(e, "endRoomColor", defaultRoom),
			ease:      int(e.Float("ease", 0)),
		}
		m.bgEvents = append(m.bgEvents, ev)
		m.ctx.At(b, func() { m.setBackground(ev) })
	}
}

func (m *Module) Ready() {
	sort.Slice(m.bgEvents, func(i, j int) bool { return m.bgEvents[i].beat < m.bgEvents[j].beat })
	sort.Slice(m.camEvents, func(i, j int) bool { return m.camEvents[i].beat < m.camEvents[j].beat })
}

func (m *Module) OnSwitch(beat float64) {
	m.balls = nil
	m.persistColor(beat)
	m.resetScene(beat)
}

func (m *Module) Whiff(beat float64) {
	m.ctx.Sound("swing")
	m.swing(beat)
}

func (m *Module) Update(_, beat float64) {
	for i := range m.balls {
		if !m.balls[i].dead && beat > m.balls[i].start+12 {
			m.balls[i].dead = true
		}
	}
	m.balls = liveBalls(m.balls)
}

func (m *Module) Draw(screen *ebiten.Image, _, beat float64) {
	screen.Fill(stageFill)
	m.applyBackground(beat)
	m.updateAlien(beat)
	addZ := m.cameraAddZ(beat)
	m.ctx.SampleSceneZ(beat, addZ)
	for _, b := range m.balls {
		if sp, world, z, ok := m.ballDrawState(b, beat); ok {
			m.ctx.Scene.Queue(kart.ExtraSprite{
				Sprite: sp, World: world, Z: z,
				Layer: 0, Order: 1000,
				Tint: white,
			})
		}
	}
	m.ctx.Scene.Draw(screen, m.proj)
}

func (m *Module) resetScene(beat float64) {
	sec := m.ctx.SecPerBeat(beat)
	m.ctx.Scene.SetActive(m.ballRoot, false)
	m.ctx.Scene.SetActive(m.dust, false)
	m.ctx.Scene.PlayDefaultState(m.player, beat, sec)
	m.ctx.Scene.PlayDefaultState(m.disp, beat, sec)
	m.ctx.Scene.PlayDefaultState(m.alien, beat, sec)
	m.setHatFrame(0)
	m.ctx.Scene.SetColorOver(m.bat, m.batColor)
}

func (m *Module) scheduleShoot(beat float64, high, quick, offbeat bool, typ int) {
	m.ctx.At(beat-1, func() { m.prepareDispenser(beat - 1) })
	m.ctx.At(beat, func() {
		if high {
			m.ctx.Sound("longShoot")
		} else if offbeat {
			m.ctx.SoundPitch("longShoot", 1, 0.5)
		}
		// Unity's Shoot method intentionally falls through here: high/offbeat
		// balls play longShoot first, then the normal shoot cue unless quick is set.
		if quick {
			m.ctx.SoundPitch("longShoot", 1, 1.5)
		} else {
			m.ctx.Sound("shoot")
		}
		m.prepareBall(beat, high, quick, offbeat, typ)
		m.ctx.Scene.PlayState(m.disp, "DispenserShoot", beat, m.ctx.SecPerBeat(beat))
	})
}

func (m *Module) prepareBall(beat float64, high, quick, offbeat bool, typ int) {
	b := &ball{
		start: beat, high: high, quick: quick, offbeat: offbeat, kind: typ,
		sprite: ballSprite(m.ballSprites, typ), scale: ballScale(typ),
		isTacobell: typ == ballTacobell,
		startRot:   float64(rand.New(rand.NewSource(int64(beat*1000) + int64(typ)*97)).Intn(360)),
	}
	m.balls = append(m.balls, b)
	target := beat + ballBeatLength(high, quick, offbeat)
	m.ctx.ScheduleInput(target, func(state float64, _ engine.Judgment) {
		if state >= 1 || state <= -1 {
			m.nearMiss(b, m.ctx.Beat())
			return
		}
		m.hitBall(b, m.ctx.Beat())
	}, func() { m.missBall(b, target) })
}

func (m *Module) prepareDispenser(beat float64) {
	m.ctx.Scene.PlayState(m.disp, "DispenserPrepare", beat, m.ctx.SecPerBeat(beat))
}

func (m *Module) hitBall(b *ball, beat float64) {
	if b.dead || b.hit || b.near {
		return
	}
	b.hit = true
	b.hitBeat = beat
	pos, rot := m.ballPose(b, beat)
	b.hitPos, b.hitRot = pos, rot
	b.endX = 4 + rand.New(rand.NewSource(int64(b.start*2048))).Float64()*12
	if b.isTacobell {
		m.ctx.Sound("tacobell")
	}
	m.ctx.Sound("hit")
	if m.ctx.App.Autoplay {
		m.ctx.Sound("swing")
	}
	m.swing(beat)
}

func (m *Module) nearMiss(b *ball, beat float64) {
	if b.dead || b.hit || b.near {
		return
	}
	b.near = true
	b.hitBeat = beat
	b.hitPos, b.hitRot = m.ballPose(b, beat)
	m.ctx.PlayCommon("miss")
	m.ctx.ScoreMiss()
}

func (m *Module) missBall(b *ball, beat float64) {
	if b.dead || b.hit || b.near {
		return
	}
	b.dead = true
	m.ctx.Sound("fall")
	m.ctx.ScoreMiss()
	m.ctx.Scene.SetActive(m.dust, true)
	m.ctx.Scene.PlayState(m.dust, "Dust", beat, m.ctx.SecPerBeat(beat))
	m.ctx.At(beat+0.5, func() { m.ctx.Scene.SetActive(m.dust, false) })
}

func (m *Module) swing(beat float64) {
	m.ctx.Scene.PlayState(m.player, "Swing", beat, m.ctx.SecPerBeat(beat))
	sec := m.ctx.SecPerBeat(beat)
	for frame, clipSec := range []float64{0, 0.05, 0.10, 0.15} {
		frame, bb := frame, beat+clipSec/sec
		m.ctx.At(bb, func() { m.setHatFrame(frame) })
	}
}

func (m *Module) setCostume(typ, mask int, skin, clothes, clothes2, bat [4]float64) {
	m.costume = typ
	m.mask = typ
	if typ == costumeCustom {
		m.mask = mask
		m.customPal = kart.Palette{Alpha: clothes2, Fill: skin, Outline: clothes}
		m.batColor = bat
	} else if typ >= 0 && typ < len(m.batColors) {
		m.batColor = m.batColors[typ]
	}
	m.applyCostumePalette()
	m.ctx.Scene.SetColorOver(m.bat, m.batColor)
	m.ctx.Scene.PlayState(m.player, "Idle", m.ctx.Beat(), m.ctx.SecPerBeat(m.ctx.Beat()))
	m.setHatFrame(0)
}

func (m *Module) applyCostumePalette() {
	p := costumePalette(m.costume)
	if m.costume == costumeCustom {
		p = m.customPal
	}
	// The SpriteRenderer's material path is fixed to player_1 in the prefab.
	// Updating that palette reproduces SetCostume without mutating scene nodes.
	m.ctx.Scene.SetPaletteFor("player_1", p)
}

func (m *Module) setHatFrame(frame int) {
	sprites, offsets := hatFrames(m.mask)
	if len(sprites) == 0 {
		m.ctx.Scene.SetSpriteOver(m.hat, "")
		return
	}
	if frame >= len(sprites) {
		frame = 0
	}
	m.ctx.Scene.SetSpriteOver(m.hat, sprites[frame])
	if frame < len(offsets) {
		m.ctx.Scene.SetPosOver(m.hat, offsets[frame][0], offsets[frame][1])
	}
}

func (m *Module) showAlien(beat float64, hide bool) {
	m.alienShowing = true
	m.alienHiding = hide
	m.alienShowBeat = beat
	if hide {
		m.ctx.Scene.PlayState(m.alien, "AlienHide", beat, m.ctx.SecPerBeat(beat))
	}
}

func (m *Module) updateAlien(beat float64) {
	if m.alienShowing {
		u := (beat - m.alienShowBeat) / 1
		if !m.alienHiding {
			m.ctx.Scene.PlayNormalized(m.alien, "Animations/AlienShow", u)
		}
		if u >= 2 {
			m.alienShowing = false
		}
		return
	}
	if !m.alienHiding {
		m.ctx.Scene.PlayNormalized(m.alien, "Animations/AlienSwing", beat-math.Floor(beat))
	}
}

func (m *Module) ballDrawState(b *ball, beat float64) (string, kart.Aff, float64, bool) {
	if b.dead {
		return "", kart.Identity(), 0, false
	}
	pos, rot := m.ballPose(b, beat)
	scale := 0.25 * b.scale
	world := kart.Translate(pos[0], pos[1]).Mul(kart.Rotate(rot * math.Pi / 180)).Mul(kart.Scale(scale, scale))
	return b.sprite, world, pos[2], true
}

func (m *Module) ballPose(b *ball, beat float64) ([3]float64, float64) {
	if b.hit {
		u := clamp01((beat - b.hitBeat) / 10)
		pos := [3]float64{
			lerp(b.hitPos[0], b.endX, u),
			lerp(b.hitPos[1], 0, u),
			lerp(b.hitPos[2], -600, u),
		}
		return pos, lerp(b.hitRot, -2260, u)
	}
	if b.near {
		t := math.Max(0, beat-b.hitBeat)
		pos := [3]float64{
			b.hitPos[0] + 4*t,
			b.hitPos[1] + 11*t - 18*t*t,
			b.hitPos[2],
		}
		return pos, b.hitRot - 325*t
	}
	u := (beat - b.start) / (ballBeatLength(b.high, b.quick, b.offbeat) + 0.15)
	curve := m.curves[curveKey(b.high, b.quick, b.offbeat)]
	pos := kart.EvalBezier(curve, u)
	rot := lerp(b.startRot, b.startRot-210, u)
	return pos, rot
}

func (m *Module) setBackground(ev bgEvt) {
	m.bgEase = bgEase{beat: ev.beat, length: ev.length, from: ev.bgStart, to: ev.bgEnd, ease: ev.ease}
	m.roomEase = bgEase{beat: ev.beat, length: ev.length, from: ev.roomStart, to: ev.roomEnd, ease: ev.ease}
}

func (m *Module) applyBackground(beat float64) {
	bg, room := m.bgEase.at(beat), m.roomEase.at(beat)
	for _, p := range []string{m.bg, m.square} {
		m.ctx.Scene.SetColorOver(p, bg)
	}
	for _, p := range []string{m.room, m.hole, m.shadow, m.shadow2} {
		m.ctx.Scene.SetColorOver(p, room)
	}
}

func (m *Module) persistColor(beat float64) {
	m.bgEase = bgEase{from: defaultBG, to: defaultBG}
	m.roomEase = bgEase{from: defaultRoom, to: defaultRoom}
	for _, ev := range m.bgEvents {
		if ev.beat >= beat {
			break
		}
		m.setBackground(ev)
	}
}

func (m *Module) cameraAddZ(beat float64) float64 {
	lastDist, curDist := -10.0, -10.0
	start, length, ease := 0.0, 0.0, 0
	for _, ev := range m.camEvents {
		if ev.beat > beat {
			break
		}
		lastDist, curDist = curDist, ev.dist
		start, length, ease = ev.beat, ev.length, ev.ease
	}
	if length <= 0 || beat >= start+length {
		return curDist + 10
	}
	u := clamp01((beat - start) / length)
	return engine.Ease(ease, lastDist+10, curDist+10, u)
}

func (e bgEase) at(beat float64) [4]float64 {
	if e.length <= 0 {
		return e.to
	}
	u := clamp01((beat - e.beat) / e.length)
	return [4]float64{
		engine.Ease(e.ease, e.from[0], e.to[0], u),
		engine.Ease(e.ease, e.from[1], e.to[1], u),
		engine.Ease(e.ease, e.from[2], e.to[2], u),
		engine.Ease(e.ease, e.from[3], e.to[3], u),
	}
}

func ballBeatLength(high, quick, offbeat bool) float64 {
	switch {
	case high:
		return 2
	case quick:
		return 0.5
	case offbeat:
		return 1.5
	default:
		return 1
	}
}

func curveKey(high, quick, offbeat bool) string {
	switch {
	case high:
		return "ball.pitchHighCurve"
	case quick:
		return "ball.pitchQuickCurve"
	case offbeat:
		return "ball.pitchOffbeatCurve"
	default:
		return "ball.pitchLowCurve"
	}
}

func normalizeBallSprites(in []string) []string {
	out := make([]string, ballStar+1)
	copy(out, in)
	out[ballBaseball] = fallback(out[ballBaseball], "spaceball_ball")
	out[ballOnigiri] = fallback(out[ballOnigiri], "spaceball_riceball")
	out[ballAlien] = fallback(out[ballAlien], "spaceball_umpire_3")
	out[ballTacobell] = fallback(out[ballTacobell], "tacobell")
	out[ballApple] = fallback(out[ballApple], "apple")
	out[ballStar] = "star"
	return out
}

func ballSprite(s []string, typ int) string {
	if typ >= 0 && typ < len(s) && s[typ] != "" {
		return s[typ]
	}
	return s[ballBaseball]
}

func ballScale(typ int) float64 {
	switch typ {
	case ballOnigiri:
		return 1.2
	case ballTacobell:
		return 2
	case ballApple:
		return 5
	case ballStar:
		return 6
	default:
		return 1
	}
}

func costumePalette(idx int) kart.Palette {
	pals := []kart.Palette{
		{Alpha: [4]float64{0, 0, 0, 1}, Fill: [4]float64{0.38823533, 0.9058824, 0, 1}, Outline: white},
		{Alpha: [4]float64{0.9803922, 0.12941177, 0.6745098, 1}, Fill: [4]float64{0.70980394, 0, 0.83921576, 1}, Outline: [4]float64{1, 0.8078432, 1, 1}},
		{Alpha: [4]float64{0, 0.2901961, 0.93725497, 1}, Fill: [4]float64{0.9058824, 0.41960788, 0, 1}, Outline: [4]float64{1, 1, 0.03137255, 1}},
		{Alpha: [4]float64{0, 0, 0, 1}, Fill: [4]float64{0, 0.90588236, 0.84705883, 1}, Outline: white},
		{Alpha: [4]float64{0.41960785, 0.78039217, 0.9607843, 1}, Fill: [4]float64{0.28235295, 0.4745098, 0.60784316, 1}, Outline: [4]float64{1, 0.6117647, 0.0627451, 1}},
		{Alpha: [4]float64{0.9764706, 0.4627451, 0.09411765, 1}, Fill: [4]float64{0.5176471, 0.26666668, 0.6392157, 1}, Outline: [4]float64{0.7529412, 0.38039216, 0.8862745, 1}},
	}
	if idx >= 0 && idx < len(pals) {
		return pals[idx]
	}
	return pals[0]
}

func hatFrames(mask int) ([]string, [][2]float64) {
	switch mask {
	case 1:
		return []string{"spaceball_hat_0_0"}, [][2]float64{{0, 0}}
	case 2:
		return []string{"spaceball_hat_1_0", "spaceball_hat_1_1", "spaceball_hat_1_2", "spaceball_hat_1_3", "spaceball_hat_1_4"},
			[][2]float64{{0.18, -1.34}, {-0.2, -1.7}, {0.07, -1.67}, {-0.01, -1.62}, {-0.17, -1.62}}
	case 4:
		return []string{"spaceball_hat_2_0"}, [][2]float64{{-0.19, 0.8}}
	case 5:
		return []string{"spaceball_1", "spaceball_2", "spaceball_3", "spaceball_4", "spaceball_0"},
			[][2]float64{{0.18, -1.34}, {-0.2, -1.7}, {0.07, -1.67}, {-0.01, -1.62}, {-0.17, -1.62}}
	default:
		return nil, nil
	}
}

func readBatColors(items []kmdata.ComponentItem) [][4]float64 {
	out := make([][4]float64, 0, len(items))
	for _, it := range items {
		out = append(out, [4]float64{it.Nums["r"], it.Nums["g"], it.Nums["b"], it.Nums["a"]})
	}
	return out
}

func liveBalls(in []*ball) []*ball {
	out := in[:0]
	for _, b := range in {
		if !b.dead {
			out = append(out, b)
		}
	}
	return out
}

func intParam(e *riq.Entity, key string, def int) int { return int(e.Float(key, float64(def))) }

func boolParam(e *riq.Entity, key string) bool { return e.Float(key, 0) != 0 }

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

func fallback(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

func lerp(a, b, u float64) float64 { return a + (b-a)*u }

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
