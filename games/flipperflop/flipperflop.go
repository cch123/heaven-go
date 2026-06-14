// Package flipperflop ports Flipper-Flop's flip/roll inputs, Captain Tuck
// voice choreography, flipper state machine, roll lane movement, and snow puffs.
package flipperflop

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
	"hsdemo/riq"
)

const (
	whoFlippers = iota
	whoCaptain
	whoBoth
	whoNone
)

const (
	captainNormal = iota
	captainRoll
	captainMiss
	captainSuccess
)

const (
	appreciationNone = iota
	appreciationGood
	appreciationGoodJob
	appreciationNice
	appreciationWellDone
	appreciationYes
	appreciationRandom
)

const rollAction = 1

type bopEvt struct {
	beat, length float64
	who, auto    int
}

type flipEvt struct {
	beat, length     float64
	roll             bool
	uh, appreciation int
	thatsIt          bool
	heart            bool
	barber           bool
}

type attentionEvt struct {
	beat   float64
	mute   bool
	remix5 bool
}

type rollVoiceEvt struct {
	beat   float64
	amount int
	now    bool
}

type walkEvt struct {
	beat, length float64
	ease         int
}

type flipper struct {
	root        string
	face        string
	leftImpact  string
	rightImpact string
	player      bool
	left        bool
	up          bool
	canBlink    bool
}

type snowDrop struct {
	x, y     float64
	vx, vy   float64
	life     float64
	size     float64
	alpha    float64
	spawnKey float64
}

type snowPuff struct {
	beat  float64
	path  string
	drops []snowDrop
}

type Module struct {
	ctx  *engine.Ctx
	proj kart.Aff

	captainPath  string
	captainFace  string
	flippersRoot string

	flippers []*flipper
	player   *flipper

	bops       []bopEvt
	flips      []flipEvt
	triple     []float64
	attentions []attentionEvt
	rollVoices []rollVoiceEvt
	walks      []walkEvt
	toggles    []float64

	goBopFlip bool
	goBopTuck bool
	lastPulse int

	missed       bool
	readyRoll    bool
	captainBop   int
	captainShown bool

	flippersBaseX float64
	flippersBaseY float64
	rollDistance  float64

	queuedMoves []float64
	moveLeftFor map[float64]bool
	isMoving    bool
	moveLeft    bool
	lastX       float64
	currentX    float64
	lastCamX    float64
	currentCamX float64
	flippersX   float64
	cameraX     float64

	walking       bool
	walkStartBeat float64
	walkLength    float64
	walkEase      int

	snow []snowPuff
}

func New() engine.Module { return &Module{lastPulse: -1, captainShown: true} }

func (m *Module) ID() string { return "flipperFlop" }

func (m *Module) Load(ctx *engine.Ctx) error {
	m.ctx = ctx
	if err := ctx.LoadAssets("flipperFlop"); err != nil {
		return err
	}
	m.proj = kart.Translate(engine.ScreenW/2, engine.ScreenH/2).Mul(kart.Scale(54, -54))
	m.captainPath = "CaptainTuck"
	m.captainFace = "CaptainTuck/HeadHolder/Head"
	m.flippersRoot = "Flippers"
	m.flippersBaseX = -0.3
	m.flippersBaseY = -0.5254178
	m.rollDistance = 1
	m.lastX, m.currentX, m.flippersX = m.flippersBaseX, m.flippersBaseX, m.flippersBaseX
	m.moveLeftFor = map[float64]bool{}
	m.flippers = []*flipper{
		{
			root: "Flippers/FlipperHolder/Flipper", face: "Flippers/FlipperHolder/Flipper/FaceHolder/Face",
			leftImpact: "Flippers/FlipperHolder/Flipper/LeftImpact", rightImpact: "Flippers/FlipperHolder/Flipper/RightImpact",
			canBlink: true,
		},
		{
			root: "Flippers/FlipperHolder (1)/Flipper (1)", face: "Flippers/FlipperHolder (1)/Flipper (1)/FaceHolder/Face",
			leftImpact: "Flippers/FlipperHolder (1)/LeftImpact", rightImpact: "Flippers/FlipperHolder (1)/RightImpact",
			canBlink: true,
		},
		{
			root: "Flippers/FlipperHolder (2)/Flipper (2)", face: "Flippers/FlipperHolder (2)/Flipper (2)/FaceHolder/Face",
			leftImpact: "Flippers/FlipperHolder (2)/LeftImpact", rightImpact: "Flippers/FlipperHolder (2)/RightImpact",
			canBlink: true,
		},
	}
	m.player = &flipper{
		root: "Flippers/FlipperHolderPlayer/FlipperPlayer", face: "Flippers/FlipperHolderPlayer/FlipperPlayer/FaceHolder/Face",
		leftImpact: "Flippers/FlipperHolderPlayer/FlipperPlayer/LeftImpact", rightImpact: "Flippers/FlipperHolderPlayer/FlipperPlayer/RightImpact",
		player: true, canBlink: true,
	}
	m.initScene(0)
	return nil
}

func (m *Module) initScene(beat float64) {
	sec := m.ctx.SecPerBeat(math.Max(beat, 0))
	m.ctx.Scene.PlayDefaultState(m.captainPath, beat, sec)
	m.ctx.Scene.PlayDefaultState(m.captainFace, beat, sec)
	for _, f := range append(m.flippers[:], m.player) {
		m.ctx.Scene.PlayDefaultState(f.root, beat, sec)
		m.ctx.Scene.PlayDefaultState(f.face, beat, sec)
		m.ctx.Scene.SetActive(f.leftImpact, false)
		m.ctx.Scene.SetActive(f.rightImpact, false)
		f.left, f.up, f.canBlink = false, false, true
	}
	m.ctx.Scene.SetPosOver(m.flippersRoot, m.flippersBaseX, m.flippersBaseY)
	m.ctx.Scene.SetActive(m.captainPath, m.captainShown)
	m.captainBop = captainNormal
}

func (m *Module) OnEvent(e *riq.Entity) {
	switch e.Datamodel {
	case "flipperFlop/bop":
		m.bops = append(m.bops, bopEvt{
			beat: e.Beat, length: e.Length,
			who:  int(e.Float("whoBops", whoBoth)),
			auto: int(e.Float("whoBopsAuto", whoNone)),
		})
	case "flipperFlop/attentionCompany":
		m.attentions = append(m.attentions, attentionEvt{beat: e.Beat, mute: boolParam(e, "toggle")})
	case "flipperFlop/attentionCompanyAlt":
		m.attentions = append(m.attentions, attentionEvt{beat: e.Beat, mute: boolParam(e, "toggle"), remix5: true})
	case "flipperFlop/flipping":
		length := e.Length
		if length <= 0 {
			length = 4
		}
		m.flips = append(m.flips, flipEvt{beat: e.Beat, length: length})
	case "flipperFlop/tripleFlip":
		m.triple = append(m.triple, e.Beat)
	case "flipperFlop/flipperRollVoiceLine":
		m.rollVoices = append(m.rollVoices, rollVoiceEvt{
			beat: e.Beat, amount: clampInt(int(e.Float("amount", 1)), 1, 10), now: boolParam(e, "toggle"),
		})
	case "flipperFlop/flipperRolling":
		length := e.Length
		if length <= 0 {
			length = 4
		}
		m.flips = append(m.flips, flipEvt{
			beat: e.Beat, length: length, roll: true,
			uh: int(e.Float("uh", 0)), thatsIt: boolParam(e, "thatsIt"),
			appreciation: int(e.Float("appreciation", appreciationNone)),
			heart:        boolParam(e, "heart"),
			barber:       boolParam(e, "barber"),
		})
	case "flipperFlop/walk":
		length := e.Length
		if length <= 0 {
			length = 4
		}
		m.walks = append(m.walks, walkEvt{beat: e.Beat, length: length, ease: int(e.Float("ease", 0))})
	case "flipperFlop/toggleCaptain":
		m.toggles = append(m.toggles, e.Beat)
	}
}

func (m *Module) Ready() {
	sort.SliceStable(m.bops, func(i, j int) bool { return m.bops[i].beat < m.bops[j].beat })
	sort.SliceStable(m.flips, func(i, j int) bool { return m.flips[i].beat < m.flips[j].beat })
	sort.Float64s(m.triple)
	sort.SliceStable(m.attentions, func(i, j int) bool { return m.attentions[i].beat < m.attentions[j].beat })
	sort.SliceStable(m.rollVoices, func(i, j int) bool { return m.rollVoices[i].beat < m.rollVoices[j].beat })
	sort.SliceStable(m.walks, func(i, j int) bool { return m.walks[i].beat < m.walks[j].beat })
	sort.Float64s(m.toggles)

	for _, ev := range m.bops {
		ev := ev
		m.ctx.At(ev.beat, func() {
			m.goBopFlip = ev.auto == whoFlippers || ev.auto == whoBoth
			m.goBopTuck = ev.auto == whoCaptain || ev.auto == whoBoth
		})
		for i := 0.0; i < ev.length; i++ {
			b := ev.beat + i
			m.ctx.At(b, func() { m.singleBop(ev.who, b) })
		}
	}
	for _, ev := range m.attentions {
		ev := ev
		m.scheduleAttention(ev)
	}
	for _, ev := range m.rollVoices {
		ev := ev
		m.scheduleRollVoice(ev)
	}
	for _, ev := range m.flips {
		ev := ev
		m.scheduleFlips(ev)
	}
	for _, b := range m.triple {
		b := b
		m.scheduleTriple(b)
	}
	for _, ev := range m.walks {
		ev := ev
		m.ctx.At(ev.beat, func() {
			m.captainShown = true
			m.ctx.Scene.SetActive(m.captainPath, true)
			m.walking, m.walkStartBeat, m.walkLength, m.walkEase = true, ev.beat, ev.length, ev.ease
		})
	}
	for _, b := range m.toggles {
		b := b
		m.ctx.At(b, func() {
			m.captainShown = !m.captainShown
			m.ctx.Scene.SetActive(m.captainPath, m.captainShown)
		})
	}
	sort.Float64s(m.queuedMoves)
}

func (m *Module) OnSwitch(beat float64) {
	m.initScene(beat)
	m.lastPulse = int(math.Floor(beat))
}

func (m *Module) Whiff(beat float64) { m.WhiffAction(beat, 0) }

func (m *Module) WhiffAction(beat float64, action int) {
	if action == 0 || action == rollAction {
		m.player.flip(m, beat, false, false, false, true)
	}
}

func (m *Module) Update(_ float64, beat float64) {
	m.updateBeatPulse(beat)
	m.updateWalking(beat)
	m.updateMovement(beat)
	m.keepSnow(beat)
}

func (m *Module) Draw(screen *ebiten.Image, _ float64, beat float64) {
	screen.Fill(color.NRGBA{0xc5, 0xf1, 0xff, 0xff})
	m.ctx.Scene.SetPosOver(m.flippersRoot, m.flippersX, m.flippersBaseY)
	cam := m.ctx.CameraAt(beat)
	m.ctx.Scene.SetCamera(cam[0]+m.cameraX, cam[1], cam[2])
	m.ctx.Scene.Sample(beat)
	m.ctx.Scene.Draw(screen, m.proj)
	m.drawSnow(screen, beat)
}

func (m *Module) updateBeatPulse(beat float64) {
	p := int(math.Floor(beat + 1e-6))
	if p <= m.lastPulse {
		return
	}
	for b := m.lastPulse + 1; b <= p; b++ {
		if b >= 0 {
			if m.goBopFlip {
				m.singleBop(whoFlippers, float64(b))
			}
			if m.goBopTuck {
				m.singleBop(whoCaptain, float64(b))
			}
		}
	}
	m.lastPulse = p
}

func (m *Module) updateWalking(beat float64) {
	if !m.walking || m.walkLength <= 0 {
		return
	}
	u := clamp01((beat - m.walkStartBeat) / m.walkLength)
	m.ctx.Scene.PlayFrozen(m.captainPath, "CaptainTuckWalkIntro", engine.Ease(m.walkEase, 0, 1, u))
	if u >= 1 {
		m.walking = false
	}
}

func (m *Module) updateMovement(beat float64) {
	for len(m.queuedMoves) > 0 && beat >= m.queuedMoves[0] {
		if !m.isMoving {
			start := m.queuedMoves[0]
			m.moveLeft = m.moveLeftFor[start]
			delta := m.rollDistance
			if m.moveLeft {
				delta = -delta
			}
			m.currentX = m.flippersX + delta
			m.currentCamX = m.cameraX + delta
			m.isMoving = true
			if m.moveLeft {
				m.spawnSnow("Flippers/SnowRight", start)
			} else {
				m.spawnSnow("Flippers/SnowLeft", start)
			}
		}
		start := m.queuedMoves[0]
		uMove := (beat - start) / 0.5
		uCam := (beat - start) / 1
		if uMove <= 1 {
			m.flippersX = engine.Ease(3, m.lastX, m.currentX, uMove) // EaseOutQuad
		} else {
			m.flippersX = m.currentX
		}
		if uCam <= 1 {
			m.cameraX = engine.Ease(4, m.lastCamX, m.currentCamX, uCam) // EaseInOutQuad
			return
		}
		m.cameraX = m.currentCamX
		m.queuedMoves = m.queuedMoves[1:]
		m.isMoving = false
		m.lastX, m.lastCamX = m.currentX, m.currentCamX
	}
}

func (m *Module) singleBop(who int, beat float64) {
	switch who {
	case whoFlippers:
		m.bopFlippers(beat)
	case whoCaptain:
		m.captainTuckBop(beat)
	case whoBoth:
		m.bopFlippers(beat)
		m.captainTuckBop(beat)
	}
}

func (m *Module) bopFlippers(beat float64) {
	for _, f := range m.flippers {
		f.bop(m, beat)
	}
	m.player.bop(m, beat)
}

func (m *Module) captainTuckBop(beat float64) {
	switch m.captainBop {
	case captainRoll:
		m.ctx.Scene.PlayState(m.captainPath, "CaptainRoll", beat, 0.5)
	case captainMiss:
		m.ctx.Scene.PlayState(m.captainPath, "CaptainTuckMissBop", beat, 0.5)
	case captainSuccess:
		m.ctx.Scene.PlayState(m.captainPath, "CaptainSucessBop", beat, 0.5)
	default:
		m.ctx.Scene.PlayState(m.captainPath, "CaptainBop", beat, 0.5)
	}
}

func (m *Module) scheduleAttention(ev attentionEvt) {
	if ev.mute {
		m.ctx.SoundAt(ev.beat+attentionEndOffset(ev.remix5), "attention/attention7", 1)
	} else {
		m.ctx.SoundAt(ev.beat-0.25, "attention/attention1", 1)
		m.ctx.SoundAtOff(ev.beat, "attention/attention2", 1, 0.025)
		m.ctx.SoundAtOff(ev.beat+0.5, "attention/attention3", 1, 0.055)
		m.ctx.SoundAtOff(ev.beat+2, "attention/attention4", 1, 0.06)
		m.ctx.SoundAt(ev.beat+2.25, "attention/attention5", 1)
		m.ctx.SoundAt(ev.beat+2.5, "attention/attention6", 1)
		m.ctx.SoundAtOff(ev.beat+attentionEndOffset(ev.remix5), "attention/attention7", 1, 0.025)
	}
	for _, off := range []float64{-0.25, 0, 0.5, 2, 2.25, 2.5} {
		if ev.mute && off < 2 {
			continue
		}
		b := ev.beat + off
		m.ctx.At(b, func() { m.ctx.Scene.PlayState(m.captainFace, "CaptainTuckSpeakExpression", b, 0.5) })
	}
	prepare := ev.beat + 3
	if ev.remix5 {
		prepare = ev.beat + 2
	}
	m.ctx.At(prepare, func() {
		for _, f := range append(m.flippers[:], m.player) {
			f.prepare(m, prepare)
		}
	})
}

func attentionEndOffset(remix5 bool) float64 {
	if remix5 {
		return 2
	}
	return 3
}

func (m *Module) scheduleRollVoice(ev rollVoiceEvt) {
	if ev.now {
		m.ctx.SoundAtOff(ev.beat, "count/flipperRollCountNow", 1, 0.037)
	} else {
		offset := map[int]float64{1: 0.003, 2: 0.02, 3: 0.02, 4: 0.035, 5: 0.05, 6: 0.06, 7: 0.03, 8: 0.008, 9: 0.02, 10: 0.01}[ev.amount]
		m.ctx.SoundAtOff(ev.beat, "count/flipperRollCount"+strconv.Itoa(ev.amount), 1, offset)
		if ev.amount == 7 {
			m.ctx.SoundAtOff(ev.beat+0.25, "count/flipperRollCount7B", 1, 0.05)
		}
	}
	m.ctx.SoundAtOff(ev.beat+0.5, "count/flipperRollCountA", 1, 0.05)
	m.ctx.SoundAtOff(ev.beat+0.75, "count/flipperRollCountB", 1, 0.015)
	last := "count/flipperRollCountS"
	if ev.now || ev.amount == 1 {
		last = "count/flipperRollCountC"
	}
	m.ctx.SoundAtOff(ev.beat+1, last, 1, 0.015)
	for _, off := range []float64{0, 0.5, 0.75, 1} {
		b := ev.beat + off
		m.ctx.At(b, func() { m.ctx.Scene.PlayState(m.captainFace, "CaptainTuckSpeakExpression", b, 0.5) })
	}
}

func (m *Module) scheduleFlips(ev flipEvt) {
	flopCount := 1
	recounts := 0
	for i := 0; i < int(ev.length); i++ {
		idx := i
		target := ev.beat + float64(i)
		if ev.roll {
			m.ctx.ScheduleInputAction(target, rollAction,
				func(state float64, _ engine.Judgment) { m.justRoll(target, state) },
				func() { m.missRoll(target) })
			m.queuedMoves = append(m.queuedMoves, target)
			m.ctx.At(target-1.5, func() { m.readyRoll = true })
			m.ctx.At(target-0.5, func() {
				m.readyRoll = true
				m.moveLeftFor[target] = m.flippers[0].left
			})
			m.ctx.At(target, func() {
				for _, f := range m.flippers {
					f.flip(m, target, true, true, false, false)
				}
			})
			m.scheduleRollCount(ev, target, idx, flopCount, recounts)
			if ev.appreciation != appreciationNone && ev.uh == 0 && idx+1 == int(ev.length) {
				m.scheduleAppreciation(target+1, ev.appreciation, ev.heart)
			}
			if ev.appreciation == appreciationNone && ev.uh == 0 && idx+1 == int(ev.length) {
				resetBeat := ev.beat + ev.length - 0.1
				m.ctx.At(resetBeat, func() { m.resetCaptainMood() })
			}
			if idx+1 < int(ev.length) {
				flopCount++
			}
			if flopCount > 4 {
				flopCount = 1
				recounts++
			}
		} else {
			m.ctx.ScheduleInput(target,
				func(state float64, _ engine.Judgment) { m.justFlip(target, state) },
				func() { m.player.flip(m, target, false, false, false, false) })
			m.ctx.At(target-1, func() { m.readyRoll = false })
			m.ctx.At(target, func() {
				for _, f := range m.flippers {
					f.flip(m, target, false, true, false, false)
				}
				m.captainTuckBop(target)
			})
		}
	}
	if ev.roll {
		off := ev.beat + ev.length - 1
		m.ctx.At(off+0.25, func() { m.readyRoll = false })
		m.scheduleRollTail(ev, flopCount)
	}
}

func (m *Module) scheduleRollCount(ev flipEvt, target float64, idx, flopCount, recounts int) {
	sound := "count/flopCount" + strconv.Itoa(flopCount)
	switch {
	case recounts == 1:
		sound += "B"
	case recounts > 1 && flopCount < 3:
		sound += "C"
	case recounts > 1:
		sound += "B"
	}
	if ev.thatsIt && idx+1 == int(ev.length) {
		noise := flopCount
		if noise == 4 {
			noise = 2
		}
		noiseSound := "count/flopNoise" + strconv.Itoa(noise)
		if ev.barber {
			m.ctx.At(target, func() {
				m.ctx.Sound("appreciation/thatsit1")
				m.ctx.Sound(noiseSound)
				m.ctx.Scene.PlayState(m.captainPath, "CaptainBop", target, 0.5)
				m.ctx.Scene.PlayState(m.captainFace, "CaptainTuckSpeakExpression", target, 0.5)
			})
			m.ctx.At(target+0.5, func() {
				m.ctx.Sound("appreciation/thatsit2")
				m.ctx.Scene.PlayState(m.captainFace, "CaptainTuckSpeakExpression", target+0.5, 0.5)
			})
		} else {
			m.ctx.At(target-0.5, func() {
				m.ctx.Sound("appreciation/thatsit1")
				m.ctx.Scene.PlayState(m.captainFace, "CaptainTuckSpeakExpression", target-0.5, 0.5)
			})
			m.ctx.At(target, func() {
				m.ctx.Sound("appreciation/thatsit2")
				m.ctx.Scene.PlayState(m.captainFace, "CaptainTuckSpeakExpression", target, 0.5)
				m.ctx.Sound(noiseSound)
				m.ctx.Scene.PlayState(m.captainPath, "CaptainBop", target, 0.5)
			})
		}
		return
	}
	fail := "count/flopCountFail" + strconv.Itoa(flopCount)
	m.ctx.At(target, func() {
		voice := sound
		if m.missed {
			voice = fail
			m.captainBop = captainMiss
			m.ctx.Scene.PlayState(m.captainFace, "CaptainTuckMissExpression", target, 0.5)
		} else {
			m.captainBop = captainRoll
			m.ctx.Scene.PlayState(m.captainFace, "CaptainTuckRollExpression", target, 0.5)
		}
		m.captainTuckBop(target)
		m.ctx.Sound(voice)
	})
}

func (m *Module) scheduleRollTail(ev flipEvt, flopCount int) {
	length := ev.length
	if ev.uh > 0 && flopCount != 4 {
		for i := 0; i < ev.uh; i++ {
			idx := i
			voiceIdx := 3 - ev.uh + i + 1
			voice := "uh" + strconv.Itoa(voiceIdx)
			fail := "uhfail" + strconv.Itoa(voiceIdx)
			b := ev.beat + length + float64(i)
			m.ctx.At(b, func() {
				play := voice
				if m.missed {
					play = fail
					m.captainBop = captainMiss
					m.ctx.Scene.PlayState(m.captainFace, "CaptainTuckMissSpeakExpression", b, 0.5)
				} else {
					m.captainBop = captainRoll
					m.ctx.Scene.PlayState(m.captainFace, "CaptainTuckRollExpression", b, 0.5)
				}
				m.captainTuckBop(b)
				m.ctx.Sound(play)
				_ = idx
			})
		}
		m.scheduleAppreciation(ev.beat+length+float64(ev.uh), ev.appreciation, ev.heart)
	} else if ev.uh > 0 && flopCount == 4 {
		m.scheduleAppreciation(ev.beat+length, ev.appreciation, ev.heart)
	}
}

func (m *Module) scheduleAppreciation(beat float64, appreciation int, heart bool) {
	m.ctx.At(beat, func() {
		m.appreciationVoice(beat, appreciation, heart)
		if !m.missed && appreciation != appreciationNone {
			m.captainBop = captainSuccess
		} else {
			m.captainBop = captainNormal
			m.ctx.Scene.PlayState(m.captainFace, "CaptainTuckNeutralExpression", beat, 0.5)
		}
		m.captainTuckBop(beat)
		m.missed = false
	})
	m.ctx.At(beat+1, func() { m.missed = false })
	m.ctx.At(beat+2.1, func() { m.resetCaptainMood() })
}

func (m *Module) appreciationVoice(beat float64, appreciation int, heart bool) {
	if m.missed {
		return
	}
	if appreciation == appreciationRandom {
		appreciation = 1 + rand.Intn(5)
	}
	name := map[int]string{
		appreciationGood: "good", appreciationGoodJob: "goodjob", appreciationNice: "nice",
		appreciationWellDone: "welldone", appreciationYes: "yes",
	}[appreciation]
	if name == "" {
		return
	}
	m.ctx.Sound("appreciation/" + name)
	face := "CaptainTuckSpeakExpression"
	if heart {
		face = "CaptainTuckBlushExpression"
	}
	m.ctx.Scene.PlayState(m.captainFace, face, beat, 0.5)
	if !heart && (appreciation == appreciationGoodJob || appreciation == appreciationWellDone) {
		m.ctx.At(beat+0.5, func() { m.ctx.Scene.PlayState(m.captainFace, "CaptainTuckSpeakExpression", beat+0.5, 0.5) })
	}
}

func (m *Module) scheduleTriple(beat float64) {
	m.ctx.SoundAt(beat, "ding", 1)
	m.ctx.SoundAt(beat+0.5, "ding", 1)
	m.ctx.SoundAt(beat+1, "ding", 1)
	for _, off := range []float64{2, 2.5, 3} {
		target := beat + off
		m.ctx.ScheduleInput(target,
			func(state float64, _ engine.Judgment) { m.justFlip(target, state) },
			func() { m.player.flip(m, target, false, false, false, false) })
		m.ctx.At(target, func() {
			for _, f := range m.flippers {
				f.flip(m, target, false, true, false, false)
			}
		})
	}
	m.ctx.At(beat+2, func() { m.ctx.Scene.PlayState(m.captainPath, "CaptainBop", beat+2, 0.5) })
	m.ctx.At(beat+3, func() { m.ctx.Scene.PlayState(m.captainPath, "CaptainBop", beat+3, 0.5) })
	m.ctx.At(beat+2.75, func() { m.readyRoll = false })
}

func (m *Module) justFlip(beat, state float64) {
	m.player.flip(m, beat, false, true, math.Abs(state) >= 1, false)
}

func (m *Module) justRoll(beat, state float64) {
	m.player.flip(m, beat, true, true, math.Abs(state) >= 1, false)
}

func (m *Module) missRoll(beat float64) {
	m.player.flip(m, beat, true, false, false, false)
	m.missed = true
}

func (m *Module) resetCaptainMood() {
	m.missed = false
	m.captainBop = captainNormal
	m.ctx.Scene.PlayState(m.captainFace, "CaptainTuckNeutralExpression", m.ctx.Beat(), 0.5)
}

func (f *flipper) prepare(m *Module, beat float64) {
	m.ctx.Scene.PlayState(f.root, "PrepareFlop", beat, 0.5)
}

func (f *flipper) bop(m *Module, beat float64) {
	m.ctx.Scene.PlayState(f.root, "FlipperBop", beat, 0.5)
	m.ctx.Scene.PlayState(f.face, "FaceNormal", beat, 0.5)
	f.canBlink = true
}

func (f *flipper) impact(m *Module, beat float64, enableRight bool) {
	if enableRight {
		m.ctx.Scene.SetActive(f.rightImpact, true)
	} else {
		m.ctx.Scene.SetActive(f.leftImpact, true)
	}
	m.ctx.Scene.PlayState(f.face, "FaceAngry", beat, 0.5)
	m.ctx.At(beat+0.1, func() {
		m.ctx.Scene.SetActive(f.leftImpact, false)
		m.ctx.Scene.SetActive(f.rightImpact, false)
	})
	m.ctx.At(beat+0.3, func() { m.ctx.Scene.PlayState(f.face, "FaceAnnoyed", beat+0.3, 0.5) })
}

func (f *flipper) flip(m *Module, beat float64, roll, hit, barely, dontSwitch bool) {
	if roll {
		switch {
		case f.player && hit && !barely:
			side := "R"
			if f.left {
				side = "L"
			}
			m.ctx.Sound("roll" + side)
			m.ctx.Scene.PlayState(f.face, "FaceNormal", beat, 0.5)
			f.canBlink = true
		case f.player && hit && barely:
			m.ctx.Sound("tink")
			m.ctx.Scene.PlayState(f.face, "FaceBarely", beat, 0.5)
			f.canBlink = false
		case f.player && !hit:
			m.ctx.Scene.PlayState(f.face, "FaceOw", beat, 0.5)
			f.canBlink = false
			m.ctx.Sound("failgroan")
			m.bumpIntoOtherSeal(beat, !f.left)
			m.ctx.At(beat+0.3, func() { m.ctx.Scene.PlayState(f.face, "FaceGoofy", beat+0.3, 0.5) })
		}
		f.up = !f.up
	} else {
		switch {
		case f.player && hit && !barely:
			if f.up {
				m.ctx.Sound("flipB" + strconv.Itoa(rand.Intn(2)+1))
			} else {
				m.ctx.Sound("flip" + strconv.Itoa(rand.Intn(2)+1))
			}
			m.ctx.Scene.PlayState(f.face, "FaceNormal", beat, 0.5)
			f.canBlink = true
		case f.player && hit && barely:
			m.ctx.Sound("tink")
			m.ctx.Scene.PlayState(f.face, "FaceBarely", beat, 0.5)
			f.canBlink = false
		case f.player && !hit:
			m.ctx.Scene.PlayState(f.face, "FaceOw", beat, 0.5)
			f.canBlink = false
			m.ctx.SoundVol("failgroan", 0.5)
			m.ctx.SoundVol("punch", 0.5)
			m.ctx.Scene.PlayState(f.root, missState(f.up, f.left), beat, 0.5)
			m.bumpIntoOtherSeal(beat, !f.left)
			m.ctx.At(beat+0.3, func() { m.ctx.Scene.PlayState(f.face, "FaceGoofy", beat+0.3, 0.5) })
		}
	}
	if hit || barely || roll {
		state := flipState(roll, f.up, f.left)
		scale := 0.5
		if roll {
			scale = 0.8
		}
		m.ctx.Scene.PlayState(f.root, state, beat, scale)
	}
	if !dontSwitch {
		f.left = !f.left
	}
}

func (m *Module) bumpIntoOtherSeal(beat float64, toTheLeft bool) {
	if toTheLeft {
		m.flippers[1].impact(m, beat, true)
	} else {
		m.flippers[2].impact(m, beat, false)
	}
}

func flipState(roll bool, up, left bool) string {
	prefix := ""
	if roll {
		if !up {
			prefix = "Reverse"
		}
	} else if up {
		prefix = "Reverse"
	}
	action := "Flop"
	if roll {
		action = "Roll"
	}
	side := "Right"
	if left {
		side = "Left"
	}
	return prefix + action + side
}

func missState(up, left bool) string {
	prefix := ""
	if up {
		prefix = "Reverse"
	}
	side := "Right"
	if left {
		side = "Left"
	}
	return prefix + "MissFlop" + side
}

func (m *Module) spawnSnow(path string, beat float64) {
	r := rand.New(rand.NewSource(int64(beat*8192) + int64(len(path))*131))
	drops := make([]snowDrop, 0, 18)
	dir := 1.0
	if path == "Flippers/SnowRight" {
		dir = -1
	}
	for i := 0; i < 18; i++ {
		drops = append(drops, snowDrop{
			x:     (r.Float64() - 0.5) * 0.55,
			y:     (r.Float64() - 0.5) * 0.25,
			vx:    dir * (0.6 + r.Float64()*1.2),
			vy:    0.8 + r.Float64()*1.4,
			life:  0.45 + r.Float64()*0.35,
			size:  0.035 + r.Float64()*0.055,
			alpha: 0.55 + r.Float64()*0.35,
		})
	}
	m.snow = append(m.snow, snowPuff{beat: beat, path: path, drops: drops})
}

func (m *Module) keepSnow(beat float64) {
	out := m.snow[:0]
	for _, sp := range m.snow {
		if beat < sp.beat+1.2 {
			out = append(out, sp)
		}
	}
	m.snow = out
}

func (m *Module) drawSnow(screen *ebiten.Image, beat float64) {
	for _, sp := range m.snow {
		world, ok := m.ctx.Scene.NodeWorld(sp.path)
		if !ok {
			continue
		}
		t := m.ctx.BeatToTime(beat) - m.ctx.BeatToTime(sp.beat)
		for _, d := range sp.drops {
			u := clamp01((beat - sp.beat) / d.life)
			if u >= 1 {
				continue
			}
			x := d.x + d.vx*t
			y := d.y + d.vy*t - 1.6*t*t
			wx, wy := world.Apply(x, y)
			sx, sy := m.proj.Apply(wx, wy)
			a := uint8(255 * d.alpha * (1 - u))
			vector.DrawFilledCircle(screen, float32(sx), float32(sy), float32(d.size*54*(1-u*0.25)), color.NRGBA{255, 255, 255, a}, true)
		}
	}
}

func boolParam(e *riq.Entity, key string) bool { return e.Float(key, 0) != 0 }

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
