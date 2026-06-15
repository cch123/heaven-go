// Package pajamaparty ports Pajama Party (pajamaParty).
//
// Unity logic reference:
// Assets/Scripts/Games/PajamaParty/PajamaParty.cs
// Assets/Scripts/Games/PajamaParty/CtrPillowPlayer.cs
// Assets/Scripts/Games/PajamaParty/CtrPillowMonkey.cs
//
// Runtime notes:
//   - Mako, the bed, and the background use the extracted scene Animator clips.
//   - Monkeys are runtime Instantiate( MonkeyPrefab ) clones, so they use
//     kart.Template instances instead of one shared scene subtree.
//   - Jumping and pillow throws are not animation curves in Unity; their local
//     y/scale/rotation are written by script each frame, mirrored here in Update.
package pajamaparty

import (
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"hsdemo/engine"
	"hsdemo/kart"
	"hsdemo/riq"
)

const (
	actionAlt = 1

	makoAnim   = "Mako_Root/Mako"
	makoSprite = "Mako_Root/Mako"
	makoShadow = "Mako_Root/Shadow"
	makoPillow = "Mako_Root/Pillow_Root/Mako_Pillow"
	pillowRoot = "Mako_Root/Pillow_Root"
	bgRoot     = "Bg"
	bedRoot    = "Bed"

	highSuffix = "_H"
)

type Module struct {
	ctx  *engine.Ctx
	proj kart.Aff

	monkeyT *kart.Template
	monkeys [5][5]*monkey
	player  player

	highState  bool
	expectHigh bool
	bgState    bool

	highCameraHeight float64
	cameraHighStart  float64
	castleStart      float64

	monkeyNormal [4]float64
	monkeyHigh   [4]float64

	autoBop       bool
	lastBeatPulse float64
	lastCatchSnd  int
}

type player struct {
	jumpStart, jumpLength, jumpHeight float64
	hasJumped, jumpNG                 bool

	canJump, canCharge, charging bool

	throwStart, throwLength, throwHeight float64
	throwType, hasThrown, throwNG        bool

	canSleep, longSleep, sleptThisFrame bool
	startedSleeping                     bool
}

type monkey struct {
	inst        *kart.Instance
	row, col    int
	x, y, z     float64
	scale       float64
	order       int
	shouldBop   bool
	shouldntBop bool

	jumpStart float64
	jumpAlt   int
	hasJumped bool

	throwStart float64
	hasThrown  bool
}

func New() engine.Module { return &Module{} }

func (m *Module) ID() string { return "pajamaParty" }

func (m *Module) Load(ctx *engine.Ctx) error {
	m.ctx = ctx
	if err := ctx.LoadAssets("pajamaParty"); err != nil {
		return err
	}
	m.proj = kart.Translate(engine.ScreenW/2, engine.ScreenH/2).Mul(kart.Scale(54, -54))
	m.monkeyT = kart.NewTemplate(ctx.Assets, ctx.Role("MonkeyPrefab"))
	m.cameraHighStart = math.Inf(1)
	m.castleStart = math.Inf(1)
	m.lastBeatPulse = math.Inf(-1)
	m.player = player{
		jumpStart: math.Inf(-1), throwStart: math.Inf(-1),
		canJump: true, canCharge: true, throwType: true,
	}
	game := ctx.Assets.Extra.Components["game"]
	m.highCameraHeight = num(game.Nums, "HighCameraHeight", 11)
	m.monkeyNormal = rgba(game.Nums, "monkeyNrmColour", [4]float64{0.9137255, 0.91764706, 0.07450981, 1})
	m.monkeyHigh = rgba(game.Nums, "monkeyHighColour", [4]float64{0.6666667, 0.9843137, 1, 1})
	m.spawnMonkeys()
	m.applyMonkeyCostumes()
	ctx.Scene.SetActive(makoPillow, false)
	ctx.Scene.PlayState(makoAnim, "NoPose", 0, 1)
	ctx.Scene.PlayState(bedRoot, "NoPose", 0, 1)
	ctx.Scene.PlayState(bgRoot, "NoPose", 0, 1)
	return nil
}

func (m *Module) spawnMonkeys() {
	if m.monkeyT == nil {
		return
	}
	root := sceneNodePos(m.ctx.Assets, m.ctx.Role("SpawnRoot"))
	const radius = 2.75
	scale := 1.0
	order := 10
	spawnX, spawnY, spawnZ := root[0]-radius*3, root[1], 0.0
	for y := 0; y < 5; y++ {
		for x := 0; x < 5; x++ {
			spawnX += radius * scale
			if y == 0 && x == 2 {
				continue
			}
			inst := m.monkeyT.NewInstance()
			inst.Scale = [2]float64{scale, scale}
			inst.Offset = [2]float64{spawnX, spawnY}
			inst.SetActive("Pillow", false)
			inst.PlayState("Monkey", "NoPose", 0, 1)
			inst.SetGroupOrder(order)
			m.monkeys[x][y] = &monkey{
				inst: inst, row: y, col: x, x: spawnX, y: spawnY, z: spawnZ,
				scale: scale, order: order, jumpStart: math.Inf(-1), throwStart: math.Inf(-1),
			}
		}
		scale -= 0.1
		spawnX = root[0] - radius*3*scale
		spawnY = root[1] + radius/3.75*float64(y+1)
		spawnZ = radius / 5 * float64(y+1)
		order--
	}
}

func (m *Module) OnEvent(e *riq.Entity) {
	b := e.Beat
	switch e.Datamodel {
	case "pajamaParty/bop":
		m.scheduleBop(b, e.Length, boolParam(e, "bop"), boolParam(e, "autoBop"))
	case "pajamaParty/jump (side to middle)":
		m.doThreeJump(b)
	case "pajamaParty/jump (back to front)":
		m.doFiveJump(b)
	case "pajamaParty/slumber":
		m.doSleepSequence(b, boolParam(e, "toggle"), int(e.Float("type", 0)))
	case "pajamaParty/throw":
		m.doThrowSequence(b, boolParam(e, "high"))
	case "pajamaParty/open background":
		m.openBackground(b, e.Length, boolParam(e, "instant"))
	case "pajamaParty/dream boats":
		m.ctx.At(b, func() { m.ctx.Scene.PlayStateLayer("boats", bgRoot, "BoatsAppear", b, 1) })
	case "pajamaParty/high mode":
		toggle := boolParam(e, "toggle")
		m.ctx.At(b, func() { m.forceToggleHigh(toggle, b) })
	case "pajamaParty/instant slumber":
		action := int(e.Float("type", 0))
		m.ctx.At(b+e.Length-1, func() { m.doInstantSleep(b+e.Length-1, action) })
	}
}

func (m *Module) Ready() {}

func (m *Module) OnSwitch(beat float64) {
	m.lastBeatPulse = math.Floor(beat) - 1
}

func (m *Module) Whiff(beat float64) { m.WhiffAction(beat, 0) }

func (m *Module) WhiffAction(beat float64, action int) {
	switch action {
	case 0:
		if m.player.canSleep {
			m.sleepThrough()
			return
		}
		if m.player.canJump && !m.player.sleptThisFrame {
			m.ctx.PlayCommon("miss")
			m.playerJump(beat, true, false)
			m.ctx.ScoreMiss()
		}
	case actionAlt:
		if m.player.canCharge && !m.player.charging {
			m.startCharge()
		}
	}
}

func (m *Module) Update(t, beat float64) {
	m.updateAutoBop(beat)
	m.updateCamera(beat)
	m.updatePlayer(beat)
	m.updateMonkeys(beat)
	if m.player.charging && altReleasedNow() {
		m.ctx.PlayCommon("miss")
		m.endCharge(beat, false, false)
		m.ctx.ScoreMiss()
	}
}

func (m *Module) Draw(screen *ebiten.Image, _ float64, beat float64) {
	for _, mk := range m.allMonkeys() {
		mk.inst.Queue(m.ctx.Scene, beat, kart.Identity(), mk.z)
	}
	m.ctx.Scene.Draw(screen, m.proj)
}

func (m *Module) scheduleBop(beat, length float64, doesBop, auto bool) {
	m.ctx.At(beat, func() {
		m.autoBop = auto
		for _, mk := range m.allMonkeys() {
			mk.shouldBop = auto
		}
	})
	if !doesBop {
		return
	}
	for i := 0; i < int(length); i++ {
		bb := beat + float64(i)
		m.ctx.At(bb, func() { m.bopSingle(bb) })
	}
}

func (m *Module) updateAutoBop(beat float64) {
	cur := math.Floor(beat)
	for m.lastBeatPulse < cur {
		m.lastBeatPulse++
		if m.autoBop {
			m.bopSingle(m.lastBeatPulse)
		}
	}
}

func (m *Module) bopSingle(beat float64) {
	suf := m.suffix()
	if !m.player.hasThrown && !m.player.startedSleeping && m.player.canCharge {
		m.ctx.Scene.PlayState(makoAnim, "MakoBeat"+suf, beat, 0.5)
	}
	for _, mk := range m.allMonkeys() {
		if mk.shouldBop && !mk.hasThrown && !mk.shouldntBop {
			mk.inst.PlayState("Monkey", "MonkeyBeat"+suf, beat, 0.5)
		}
	}
}

func (m *Module) doThreeJump(beat float64) {
	m.scheduleJumpInput(beat)
	m.ctx.SoundAt(beat, "three1", 1)
	m.ctx.SoundAt(beat+1, "three2", 1)
	m.ctx.SoundAt(beat+2, "three3", 1)
	m.ctx.At(beat, func() {
		m.jumpCol(0, beat, 1)
		m.jumpCol(4, beat, 1)
	})
	m.ctx.At(beat+1, func() {
		m.jumpCol(1, beat+1, 3)
		m.jumpCol(3, beat+1, 3)
	})
	m.ctx.At(beat+2, func() { m.jumpCol(2, beat+2, 1) })
}

func (m *Module) doFiveJump(beat float64) {
	m.scheduleJumpInput(beat)
	for i, s := range []string{"five1", "five2", "five3", "five4", "five5"} {
		m.ctx.SoundAt(beat+0.5*float64(i), s, 1)
	}
	m.ctx.At(beat, func() { m.jumpRow(4, beat, 1) })
	m.ctx.At(beat+0.5, func() { m.jumpRow(3, beat+0.5, 2) })
	m.ctx.At(beat+1, func() { m.jumpRow(2, beat+1, 1) })
	m.ctx.At(beat+1.5, func() { m.jumpRow(1, beat+1.5, 2) })
	m.ctx.At(beat+2, func() { m.jumpRow(0, beat+2, 1) })
}

func (m *Module) scheduleJumpInput(beat float64) {
	m.ctx.ScheduleInputCond(beat+2, func() bool { return m.player.canJump },
		func(state float64, _ engine.Judgment) {
			if state <= -1 || state >= 1 {
				m.ctx.PlayCommon("miss")
				m.playerJump(m.ctx.Beat(), false, true)
			} else {
				m.ctx.Sound("jumpJust")
				m.playerJump(m.ctx.Beat(), false, false)
			}
		}, func() {})
}

func (m *Module) playerJump(beat float64, pressout, ng bool) {
	m.player.startedSleeping = false
	m.player.jumpStart = beat
	m.player.jumpLength = 1
	m.player.jumpHeight = 4
	if ng || pressout {
		m.player.jumpLength = 0.5
		m.player.jumpHeight = 2
	}
	m.player.jumpNG = ng
	m.player.hasJumped = true
	m.player.canCharge = false
	m.player.canJump = false
	m.ctx.Scene.PlayState(makoAnim, "MakoJump"+m.suffix(), beat, animScale(m.ctx, "Anime/Mako/MakoJump"+m.suffix(), m.player.jumpLength))
}

func (m *Module) jumpRow(row int, beat float64, alt int) {
	for x := 0; x < 5; x++ {
		if mk := m.monkeys[x][row]; mk != nil {
			m.jumpMonkey(mk, beat, alt)
		}
	}
}

func (m *Module) jumpCol(col int, beat float64, alt int) {
	for y := 0; y < 5; y++ {
		if mk := m.monkeys[col][y]; mk != nil {
			m.jumpMonkey(mk, beat, alt)
		}
	}
}

func (m *Module) jumpMonkey(mk *monkey, beat float64, alt int) {
	mk.jumpStart = beat
	mk.jumpAlt = alt
	mk.hasJumped = true
	mk.shouldntBop = true
	state := "MonkeyJump"
	if alt > 1 {
		state = "MonkeyJump0" + itoa(alt)
	}
	mk.inst.PlayState("Monkey", state+m.suffix(), beat, animScale(m.ctx, "Anime/Monkey/"+state+m.suffix(), 1))
}

func (m *Module) doThrowSequence(beat float64, high bool) {
	m.scheduleThrowInput(beat)
	m.playThrowSequenceSound(beat)
	if high {
		m.cameraHighStart = beat + 3
		m.expectHigh = true
	}
	m.ctx.At(beat+2, func() { m.monkeyCharge(beat + 2) })
	m.ctx.At(beat+3, func() { m.monkeyThrow(beat+3, high) })
}

func (m *Module) playThrowSequenceSound(beat float64) {
	for _, s := range []struct {
		off  float64
		name string
	}{
		{0, "throw1"}, {0.5, "throw2"}, {1, "throw3"},
		{1.5, "throw4a"}, {2, "charge"},
	} {
		m.ctx.SoundAt(beat+s.off, s.name, 1)
	}
}

func (m *Module) scheduleThrowInput(beat float64) {
	m.ctx.ScheduleInputAction(beat+2, actionAlt,
		func(state float64, _ engine.Judgment) {
			m.startCharge()
			m.player.throwNG = state <= -1 || state >= 1
			m.ctx.Sound("throw4")
		}, func() {})
	m.ctx.ScheduleInputActionReleaseCond(beat+3, actionAlt, func() bool { return m.player.charging },
		func(state float64, _ engine.Judgment) {
			if state <= -1 || state >= 1 {
				m.ctx.PlayCommon("miss")
				m.player.throwNG = true
			} else {
				m.ctx.Sound("throw5")
			}
			m.endCharge(m.ctx.Beat(), true, m.player.throwNG)
		}, func() {})
}

func (m *Module) startCharge() {
	m.player.startedSleeping = false
	m.player.canJump = false
	m.player.charging = true
	m.ctx.Scene.PlayState(makoAnim, "MakoReady"+m.suffix(), m.ctx.Beat(), 1)
}

func (m *Module) endCharge(beat float64, hit, ng bool) {
	m.projectileThrow(beat, !hit, ng)
	m.player.charging = false
	m.player.canCharge = false
	if m.expectHigh {
		m.ctx.At(beat+0.5, func() { m.toggleHighState(hit && !ng, beat+0.5, false) })
		m.ctx.At(beat+2, func() {
			if hit && !ng {
				m.ctx.Scene.PlayState(makoAnim, "MakoThrow"+m.suffix(), beat+2, 1)
				m.applyMonkeyCostumes()
			}
		})
	}
	if hit {
		m.ctx.Scene.PlayState(makoAnim, "MakoThrow"+m.suffix(), beat, 1)
	} else {
		m.ctx.Scene.PlayState(makoAnim, "MakoThrowOut"+m.suffix(), beat, 0.5)
		m.ctx.At(beat+0.5, func() {
			m.ctx.Scene.PlayState(makoAnim, "MakoPickUp"+m.suffix(), beat+0.5, 1)
			m.playCatch()
			m.ctx.Scene.SetActive(makoPillow, false)
			m.player.canCharge = true
			m.player.canJump = true
		})
	}
}

func (m *Module) projectileThrow(beat float64, drop, ng bool) {
	m.player.throwNG = ng
	m.player.throwStart = beat
	m.player.hasThrown = true
	m.player.throwType = !drop
	m.ctx.Scene.SetActive(makoPillow, true)
	if drop {
		m.player.throwLength = 0.5
		m.player.throwHeight = 0
		m.ctx.Scene.PlayState(makoPillow, "ThrowOut", beat, 1)
	} else {
		m.player.throwLength = 4
		m.player.throwHeight = 14
		if ng {
			m.player.throwLength = 1
			m.player.throwHeight = 1.5
		}
	}
}

func (m *Module) playerThrough(beat float64) {
	m.ctx.Scene.PlayState(makoAnim, "MakoThrough"+m.suffix(), beat, 0.5)
	m.player.charging = false
	m.player.canCharge = false
	m.player.canJump = false
	m.ctx.At(beat+0.5, func() {
		m.player.canCharge = true
		m.player.canJump = true
	})
}

func (m *Module) monkeyCharge(beat float64) {
	for _, mk := range m.allMonkeys() {
		mk.shouldntBop = true
		mk.inst.PlayState("Monkey", "MonkeyReady"+m.suffix(), beat, 1)
	}
}

func (m *Module) monkeyThrow(beat float64, highCheck bool) {
	for _, mk := range m.allMonkeys() {
		mk.inst.PlayState("Monkey", "MonkeyThrow"+m.suffix(), beat, 1)
		mk.throwStart = beat
		mk.hasThrown = true
		mk.inst.SetActive("Pillow", true)
		if highCheck {
			mk := mk
			m.ctx.At(beat+2, func() { mk.inst.PlayState("Monkey", "MonkeyThrow"+m.suffix(), beat+2, 1) })
		}
	}
}

func (m *Module) doSleepSequence(beat float64, alt bool, action int) {
	m.startSleepSequence(beat, alt, action)
	m.monkeySleep(beat, action)
	for _, s := range []struct {
		off  float64
		name string
	}{
		{0, "siesta1"}, {0.5, "siesta2"}, {1, "siesta3"}, {2.5, "siesta3"}, {4, "siesta3"},
	} {
		m.ctx.SoundAt(beat+s.off, s.name, 1)
	}
}

func (m *Module) startSleepSequence(beat float64, alt bool, action int) {
	m.ctx.ScheduleInputCond(beat+4, func() bool { return m.player.canSleep },
		func(state float64, _ engine.Judgment) {
			m.player.sleptThisFrame = true
			m.player.canSleep = false
			if state <= -1 || state >= 1 {
				m.ctx.Scene.PlayState(makoAnim, "MakoSleepNg"+m.suffix(), m.ctx.Beat(), 1)
				return
			}
			m.ctx.Sound("siesta4")
			m.ctx.Scene.PlayState(makoAnim, "MakoSleepJust"+m.suffix(), m.ctx.Beat(), 1)
			if !m.player.longSleep {
				m.ctx.At(beat+7, func() {
					m.ctx.Scene.PlayState(makoAnim, "MakoAwake"+m.suffix(), beat+7, 0.5)
					m.ctx.Sound("siestaDone")
				})
			}
			m.player.longSleep = false
		},
		func() { m.sleepOut() })

	m.player.charging = false
	m.player.canCharge = false
	m.player.canJump = false
	m.player.startedSleeping = true
	m.player.jumpStart = math.Inf(-1)
	m.ctx.Scene.SetPosOver(makoSprite, 0, 0)
	m.ctx.Scene.SetScaleOver(makoShadow, 1.65, 1)
	m.ctx.Scene.SetActive(makoPillow, false)
	m.player.hasThrown = false
	if action == 1 {
		m.player.longSleep = true
	}
	ready := "MakoReadySleep"
	if alt {
		ready = "MakoReadySleep01"
	}
	m.ctx.At(beat, func() { m.ctx.Scene.PlayState(makoAnim, "MakoSleep00"+m.suffix(), beat, 1) })
	m.ctx.At(beat+0.5, func() { m.ctx.Scene.PlayState(makoAnim, "MakoSleep01"+m.suffix(), beat+0.5, 1) })
	m.ctx.At(beat+1, func() {
		m.player.canSleep = true
		m.player.canJump = false
	})
	m.ctx.At(beat+3, func() {
		if m.player.canSleep {
			m.ctx.Scene.PlayState(makoAnim, ready+m.suffix(), beat+3, 1)
		}
	})
	restore := beat + 8
	if m.player.longSleep {
		restore = beat + 4
	}
	m.ctx.At(restore, func() {
		m.player.canCharge = true
		m.player.canJump = true
		m.player.startedSleeping = false
	})
}

func (m *Module) sleepThrough() {
	if !m.player.canSleep {
		return
	}
	m.ctx.Scene.PlayState(makoAnim, "MakoSleepThrough"+m.suffix(), m.ctx.Beat(), 1)
	m.player.canSleep = false
}

func (m *Module) sleepOut() {
	if !m.player.canSleep {
		return
	}
	m.ctx.Scene.PlayState(makoAnim, "MakoSleepOut"+m.suffix(), m.ctx.Beat(), 0.5)
	m.ctx.Sound("siestaBad")
	m.player.canSleep = false
}

func (m *Module) monkeySleep(beat float64, action int) {
	for _, mk := range m.allMonkeys() {
		mk.shouldntBop = true
		mk.throwStart = math.Inf(-1)
		mk.hasThrown = false
		mk.inst.SetActive("Pillow", false)
		mk.jumpStart = math.Inf(-1)
		mk.inst.SetPos("Monkey", 0, 0)
		mk.inst.SetScale("Shadow", 1.2, 0.8)
		mk := mk
		m.ctx.At(beat, func() { mk.inst.PlayState("Monkey", "MonkeySleep00"+m.suffix(), beat, 1) })
		m.ctx.At(beat+0.5, func() { mk.inst.PlayState("Monkey", "MonkeySleep01"+m.suffix(), beat+0.5, 1) })
		switch {
		case mk.col == 0 || mk.col == 4:
			m.ctx.At(beat+1, func() { mk.inst.PlayState("Monkey", "MonkeySleep02"+m.suffix(), beat+1, 1) })
		case mk.col == 1 || mk.col == 3:
			m.ctx.At(beat+1.5, func() { mk.inst.PlayState("Monkey", "MonkeyReadySleep"+m.suffix(), beat+1.5, 1) })
			m.ctx.At(beat+2.5, func() { mk.inst.PlayState("Monkey", "MonkeySleep02"+m.suffix(), beat+2.5, 1) })
		default:
			m.ctx.At(beat+3, func() { mk.inst.PlayState("Monkey", "MonkeyReadySleep"+m.suffix(), beat+3, 1) })
			m.ctx.At(beat+4, func() { mk.inst.PlayState("Monkey", "MonkeySleep02"+m.suffix(), beat+4, 1) })
		}
		if action != 1 {
			m.ctx.At(beat+7, func() { mk.inst.PlayState("Monkey", "MonkeyAwake"+m.suffix(), beat+7, 0.5) })
		}
	}
}

func (m *Module) doInstantSleep(deslumber float64, action int) {
	m.ctx.Scene.PlayState(makoAnim, "MakoSleepJust"+m.suffix(), 0, 1)
	for _, mk := range m.allMonkeys() {
		mk.inst.PlayState("Monkey", "MonkeySleep02"+m.suffix(), 0, 1)
	}
	if action == 1 {
		return
	}
	m.ctx.At(deslumber, func() {
		m.ctx.Scene.PlayState(makoAnim, "MakoAwake"+m.suffix(), deslumber, 0.5)
		m.ctx.Sound("siestaDone")
		for _, mk := range m.allMonkeys() {
			mk.inst.PlayState("Monkey", "MonkeyAwake"+m.suffix(), deslumber, 0.5)
		}
	})
}

func (m *Module) openBackground(beat, length float64, instant bool) {
	m.ctx.At(beat, func() {
		m.bgState = !m.bgState
		state := "SlideClose"
		if m.bgState {
			state = "SlideOpen"
		}
		if instant {
			m.ctx.Scene.PlayNormalized(bgRoot, "Anime/BG/"+state, 1)
			return
		}
		ts := 1.0
		if length > 0 {
			ts = 1 / length
		}
		m.ctx.Scene.PlayStateLayer("slide", bgRoot, state, beat, ts)
	})
}

func (m *Module) forceToggleHigh(toggle bool, beat float64) {
	m.expectHigh = false
	m.highState = toggle
	m.castleStart = beat - 4
	m.applyMonkeyCostumes()
	m.ctx.Scene.PlayState(makoAnim, "NoPose"+m.suffix(), beat, 1)
	for _, mk := range m.allMonkeys() {
		mk.inst.PlayState("Monkey", "NoPose"+m.suffix(), beat, 1)
	}
	state := "CastleHide"
	if m.highState {
		state = "CastleAppear"
	}
	m.ctx.Scene.PlayStateLayer("castle", bgRoot, state, m.castleStart, 1)
}

func (m *Module) toggleHighState(hit bool, beat float64, instant bool) {
	m.expectHigh = false
	m.highState = hit && !m.highState
	if !hit {
		m.highState = false
	}
	state := "FloatsFar"
	if m.highState {
		state = "FloatsNear"
	}
	m.ctx.Scene.PlayStateLayer("floats", bgRoot, state, beat, 1)
	m.castleStart = beat
	if instant {
		m.castleStart = beat - 4
	}
	castle := "CastleHide"
	if m.highState {
		castle = "CastleAppear"
	}
	m.ctx.Scene.PlayStateLayer("castle", bgRoot, castle, m.castleStart, 1)
}

func (m *Module) applyMonkeyCostumes() {
	c := m.monkeyNormal
	if m.highState {
		c = m.monkeyHigh
	}
	for _, mk := range m.allMonkeys() {
		for _, rel := range []string{
			"Monkey/ArmL", "Monkey/ArmL/ArmAttatch", "Monkey/ArmR",
			"Monkey/Body", "Monkey/BodyOther", "Monkey/Leg",
		} {
			mk.inst.SetColor(rel, c)
		}
	}
}

func (m *Module) updateCamera(beat float64) {
	if beat >= m.cameraHighStart && beat < m.cameraHighStart+4 {
		prog := (beat - m.cameraHighStart) / 4
		yMul := prog*2 - 1
		yWeight := -(yMul * yMul) + 1
		m.ctx.Scene.SetCamera(0, yWeight*m.highCameraHeight, -10)
		return
	}
	m.ctx.Scene.SetCamera(0, 0, -10)
}

func (m *Module) updatePlayer(beat float64) {
	p := &m.player
	if p.hasJumped && beat >= p.jumpStart && beat <= p.jumpStart+p.jumpLength {
		pos := (beat - p.jumpStart) / p.jumpLength
		y := parabola(pos)
		m.ctx.Scene.SetPosOver(makoSprite, 0, p.jumpHeight*y)
		m.ctx.Scene.SetScaleOver(makoShadow, (1-y*0.2)*1.65, 1-y*0.2)
	} else {
		if p.hasJumped {
			p.canJump, p.canCharge, p.hasJumped = true, true, false
			m.bedImpact(beat)
			if p.jumpNG {
				m.ctx.Scene.PlayState(makoAnim, "MakoCatchNg"+m.suffix(), beat, 1)
			} else if p.jumpHeight != 4 {
				m.ctx.Scene.PlayState(makoAnim, "MakoCatch"+m.suffix(), beat, 1)
			} else {
				m.ctx.Scene.PlayState(makoAnim, "MakoLand"+m.suffix(), beat, 1)
			}
			p.jumpNG = false
		}
		m.ctx.Scene.SetPosOver(makoSprite, 0, 0)
		m.ctx.Scene.SetScaleOver(makoShadow, 1.65, 1)
	}
	if p.hasThrown && beat >= p.throwStart && beat <= p.throwStart+p.throwLength {
		pos := (beat - p.throwStart) / p.throwLength
		if p.throwType {
			m.ctx.Scene.SetPosOver(pillowRoot, 0, p.throwHeight*parabola(pos)+0.5)
		}
		m.ctx.Scene.SetSpinOver(makoPillow, -2*math.Pi*(beat-p.throwStart))
	} else if p.hasThrown {
		m.ctx.Scene.SetPosOver(pillowRoot, 0, 0)
		m.ctx.Scene.SetSpinOver(makoPillow, 0)
		m.ctx.Scene.SetActive(makoPillow, false)
		p.hasThrown = false
		if p.throwNG {
			m.ctx.Scene.PlayState(makoAnim, "MakoCatchNg"+m.suffix(), beat, 1)
		} else {
			m.ctx.Scene.PlayState(makoAnim, "MakoCatch"+m.suffix(), beat, 1)
		}
		m.playCatch()
		p.throwNG = false
		p.canCharge, p.canJump = true, true
	}
	p.sleptThisFrame = false
}

func (m *Module) updateMonkeys(beat float64) {
	for _, mk := range m.allMonkeys() {
		if mk.hasJumped && beat >= mk.jumpStart && beat <= mk.jumpStart+1 {
			pos := beat - mk.jumpStart
			y := parabola(pos)
			mk.inst.SetPos("Monkey", 0, 4*y)
			mk.inst.SetScale("Shadow", (1-y*0.2)*1.2, (1-y*0.2)*0.8)
			if mk.jumpAlt == 2 || mk.jumpAlt == 3 {
				t := pos
				if mk.jumpAlt == 3 {
					t = 1 - pos
				}
				mk.inst.SetRot("Monkey", deg(22.5+(-45*t)))
			}
		} else {
			if mk.hasJumped {
				mk.shouldntBop = false
				mk.hasJumped = false
				m.bedImpact(beat)
				mk.inst.PlayState("Monkey", "MonkeyLand"+m.suffix(), beat, 1)
				mk.inst.SetRot("Monkey", 0)
			}
			mk.inst.SetPos("Monkey", 0, 0)
			mk.inst.SetScale("Shadow", 1.2, 0.8)
		}
		if mk.hasThrown && beat >= mk.throwStart && beat <= mk.throwStart+4 {
			pos := (beat - mk.throwStart) / 4
			mk.inst.SetPos("Pillow", 0, 14*parabola(pos)+1.5)
			mk.inst.SetRot("Pillow", -2*math.Pi*(beat-mk.throwStart))
		} else if mk.hasThrown {
			mk.inst.SetPos("Pillow", 0, 0)
			mk.inst.SetRot("Pillow", 0)
			mk.inst.SetActive("Pillow", false)
			mk.inst.PlayState("Monkey", "MonkeyBeat"+m.suffix(), beat, 1)
			mk.hasThrown = false
			mk.shouldntBop = false
		}
	}
}

func (m *Module) bedImpact(beat float64) {
	m.ctx.Scene.PlayState(bedRoot, "BedImpact", beat, 1)
}

func (m *Module) playCatch() {
	m.ctx.Sound("catch" + itoa(m.lastCatchSnd%2))
	m.lastCatchSnd++
}

func (m *Module) suffix() string {
	if m.highState {
		return highSuffix
	}
	return ""
}

func (m *Module) allMonkeys() []*monkey {
	out := make([]*monkey, 0, 24)
	for y := 0; y < 5; y++ {
		for x := 0; x < 5; x++ {
			if mk := m.monkeys[x][y]; mk != nil {
				out = append(out, mk)
			}
		}
	}
	return out
}

func altReleasedNow() bool {
	return inpututil.IsKeyJustReleased(ebiten.KeyF) ||
		inpututil.IsKeyJustReleased(ebiten.KeyLeft) ||
		inpututil.IsKeyJustReleased(ebiten.KeyUp)
}

func boolParam(e *riq.Entity, key string) bool { return e.Float(key, 0) != 0 }

func num(m map[string]float64, key string, def float64) float64 {
	if v, ok := m[key]; ok {
		return v
	}
	return def
}

func rgba(m map[string]float64, prefix string, def [4]float64) [4]float64 {
	return [4]float64{
		num(m, prefix+".r", def[0]),
		num(m, prefix+".g", def[1]),
		num(m, prefix+".b", def[2]),
		num(m, prefix+".a", def[3]),
	}
}

func sceneNodePos(as *kart.Assets, path string) [2]float64 {
	for _, n := range as.Rig.Nodes {
		if n.Path == path {
			return n.Pos
		}
	}
	return [2]float64{}
}

func parabola(t float64) float64 {
	if t < 0 {
		t = 0
	} else if t > 1 {
		t = 1
	}
	y := t*2 - 1
	return -(y * y) + 1
}

func animScale(ctx *engine.Ctx, clip string, length float64) float64 {
	if a := ctx.Assets.Anims[clip]; a != nil && length > 0 {
		return a.Duration / length
	}
	return 1
}

func deg(v float64) float64 { return v * math.Pi / 180 }

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
