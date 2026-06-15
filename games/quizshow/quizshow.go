// Package quizshow ports Quiz Show's call-and-response counter game from
// Assets/Scripts/Games/QuizShow/QuizShow.cs.
package quizshow

import (
	"image/color"
	"math"
	"math/rand"
	"sort"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"hsdemo/engine"
	"hsdemo/kart"
	"hsdemo/riq"
)

const (
	clockBoth = iota
	clockStart
	clockEnd
	clockNeither
)

const (
	buttonRandom = iota
	buttonDpadOnly
	buttonAOnly
	buttonAlternatingDpad
	buttonAlternatingA
)

const (
	explodeContestant = iota
	explodeHost
	explodeSign
)

var stageColor = color.RGBA{0xc9, 0x6e, 0xfa, 0xff}

type pressEvt struct {
	beat float64
	dpad bool
}

type randomEvt struct {
	beat, length float64
	min, max     int
	which        int
	consecutive  bool
}

type intervalEvt struct {
	idx          int
	beat, length float64
	auto         bool
	timeUpSound  bool
	consecutive  bool
	visualClock  bool
	audioClock   int
}

type passEvt struct {
	idx          int
	beat, length float64
	timeUpSound  bool
	consecutive  bool
	visualClock  bool
	audioClock   int
}

type reactionEvt struct {
	beat             float64
	audience, jingle bool
	revealInstant    bool
}

type revealEvt struct {
	beat, length float64
}

type stageEvt struct {
	beat  float64
	stage int
}

type countModEvt struct {
	beat  float64
	reset bool
}

type forceEvt struct {
	beat   float64
	target int
}

type timerState struct {
	active bool
	start  float64
	length float64
}

type playerWindow struct {
	start float64
	end   float64
}

type spark struct {
	angle float64
	speed float64
	life  float64
	size  float32
}

type burst struct {
	beat   float64
	origin [2]float64
	sparks []spark
}

type Module struct {
	ctx  *engine.Ctx
	proj kart.Aff

	contesteeLeftArm  string
	contesteeRightArm string
	contesteeHead     string
	hostLeftArm       string
	hostRightArm      string
	hostHead          string
	sign              string
	timerNeedle       string
	timerRoot         string
	blackOut          string
	firstDigit        string
	secondDigit       string
	hostFirstDigit    string
	hostSecondDigit   string
	contCounter       string
	hostCounter       string
	contExplosion     string
	hostExplosion     string
	signExplosion     string

	contestantDigits []string
	hostDigits       []string
	explodedCounter  string

	presses   []pressEvt
	randoms   []randomEvt
	intervals []intervalEvt
	passes    []passEvt
	reveals   []revealEvt
	reactions []reactionEvt
	stages    []stageEvt
	countMods []countModEvt
	forces    []forceEvt

	startedIntervals map[int]bool
	startedPasses    map[int]bool

	shouldResetCount  bool
	doingConsecutive  bool
	shouldPrepareArms bool
	contExploded      bool
	hostExploded      bool
	signExploded      bool
	pressCount        int
	countToMatch      int
	currentStage      int
	usedRhythm        bool
	inputScores       []float64
	playerWindows     []playerWindow
	timer             timerState
	rng               *rand.Rand
	bursts            []burst
}

func New() engine.Module {
	return &Module{
		shouldPrepareArms: true,
		usedRhythm:        true,
		rng:               rand.New(rand.NewSource(0x5155495a)),
		startedIntervals:  map[int]bool{},
		startedPasses:     map[int]bool{},
	}
}

func (m *Module) ID() string { return "quizShow" }

func (m *Module) Load(ctx *engine.Ctx) error {
	m.ctx = ctx
	if err := ctx.LoadAssets("quizShow"); err != nil {
		return err
	}
	m.proj = kart.Translate(engine.ScreenW/2, engine.ScreenH/2).Mul(kart.Scale(54, -54))

	m.contesteeLeftArm = roleOr(ctx, "contesteeLeftArmAnim", "Contestee/LeftArm")
	m.contesteeRightArm = roleOr(ctx, "contesteeRightArmAnim", "Contestee/RightArm")
	m.contesteeHead = roleOr(ctx, "contesteeHead", "Contestee/Head")
	m.hostLeftArm = roleOr(ctx, "hostLeftArmAnim", "QuizHost/LeftArm")
	m.hostRightArm = roleOr(ctx, "hostRightArmAnim", "QuizHost/RightArm")
	m.hostHead = roleOr(ctx, "hostHead", "QuizHost/Head")
	m.sign = roleOr(ctx, "signAnim", "SignHolder")
	m.timerNeedle = roleOr(ctx, "timerTransform", "Timer/Stopwatch/Needle")
	m.timerRoot = roleOr(ctx, "stopWatchRef", "Timer")
	m.blackOut = roleOr(ctx, "blackOut", "Blackout")
	m.firstDigit = roleOr(ctx, "firstDigitSr", "Contestee/Counter/FirstDigit")
	m.secondDigit = roleOr(ctx, "secondDigitSr", "Contestee/Counter/SecondDigit")
	m.hostFirstDigit = roleOr(ctx, "hostFirstDigitSr", "QuizHost/Counter/FirstDigit")
	m.hostSecondDigit = roleOr(ctx, "hostSecondDigitSr", "QuizHost/Counter/SecondDigit")
	m.contCounter = roleOr(ctx, "contCounter", "Contestee/Counter/Sprite")
	m.hostCounter = roleOr(ctx, "hostCounter", "QuizHost/Counter/Sprite")
	m.contExplosion = roleOr(ctx, "contExplosion", "Contestee/Counter/Explosion")
	m.hostExplosion = roleOr(ctx, "hostExplosion", "QuizHost/Counter/Explosion")
	m.signExplosion = roleOr(ctx, "signExplosion", "SignHolder/Explosion")

	if comp, ok := ctx.Assets.Extra.Components["game"]; ok {
		m.contestantDigits = append(m.contestantDigits, comp.SpriteArrays["contestantNumberSprites"]...)
		m.hostDigits = append(m.hostDigits, comp.SpriteArrays["hostNumberSprites"]...)
		m.explodedCounter = comp.Sprites["explodedCounter"]
	}
	if len(m.contestantDigits) == 0 {
		m.contestantDigits = []string{"ZeroCon", "OneCon", "TwoCon", "ThreeCon", "FourCon", "FiveCon", "SixCon", "SevenCon", "EightCon", "NineCon"}
	}
	if len(m.hostDigits) == 0 {
		m.hostDigits = []string{"ZeroHost", "OneHost", "TwoHost", "ThreeHost", "FourHost", "FiveHost", "SixHost", "SevenHost", "EightHost", "NineHost", "QuestionHost"}
	}
	if m.explodedCounter == "" {
		m.explodedCounter = "CounterGrey"
	}
	m.resetVisuals(0)
	return nil
}

func roleOr(ctx *engine.Ctx, key, fallback string) string {
	if p := ctx.Role(key); p != "" {
		return p
	}
	return fallback
}

func (m *Module) OnEvent(e *riq.Entity) {
	switch e.Datamodel {
	case "quizShow/dPad":
		m.presses = append(m.presses, pressEvt{beat: e.Beat, dpad: true})
	case "quizShow/aButton":
		m.presses = append(m.presses, pressEvt{beat: e.Beat})
	case "quizShow/randomPresses":
		m.randoms = append(m.randoms, randomEvt{
			beat: e.Beat, length: durationDefault(e.Length, 0.5),
			min: intParam(e, "min", 0), max: intParam(e, "max", 1),
			which: intParam(e, "random", buttonRandom), consecutive: boolDefault(e, "con", true),
		})
	case "quizShow/intervalStart":
		length := e.Length
		if length <= 0 {
			length = 7
		}
		m.intervals = append(m.intervals, intervalEvt{
			idx: len(m.intervals), beat: e.Beat, length: length,
			auto: boolDefault(e, "auto", true), timeUpSound: boolDefault(e, "sound", true),
			consecutive: boolParam(e, "con"), visualClock: boolDefault(e, "visual", true),
			audioClock: intParam(e, "audio", clockBoth),
		})
	case "quizShow/passTurn":
		m.passes = append(m.passes, passEvt{
			idx: len(m.passes), beat: e.Beat, length: maxFloat(e.Length, 0),
			timeUpSound: boolDefault(e, "sound", true), consecutive: boolParam(e, "con"),
			visualClock: boolDefault(e, "visual", true), audioClock: intParam(e, "audio", clockBoth),
		})
	case "quizShow/revealAnswer":
		m.reveals = append(m.reveals, revealEvt{beat: e.Beat, length: durationDefault(e.Length, 4)})
	case "quizShow/answerReaction":
		m.reactions = append(m.reactions, reactionEvt{
			beat: e.Beat, audience: boolDefault(e, "audience", true),
			jingle: boolParam(e, "jingle"), revealInstant: boolParam(e, "reveal"),
		})
	case "quizShow/skillStar":
		b := e.Beat
		m.ctx.At(b, func() {
			if m.usedRhythm {
				m.ctx.AwardSkillStar()
			}
		})
	case "quizShow/changeStage":
		m.stages = append(m.stages, stageEvt{beat: e.Beat, stage: intParam(e, "value", 1)})
	case "quizShow/countMod":
		m.countMods = append(m.countMods, countModEvt{beat: e.Beat, reset: boolDefault(e, "value", true)})
	case "quizShow/forceExplode":
		m.forces = append(m.forces, forceEvt{beat: e.Beat, target: intParam(e, "value", explodeContestant)})
	}
}

func (m *Module) Ready() {
	m.expandRandomPresses()
	sort.SliceStable(m.presses, func(i, j int) bool { return m.presses[i].beat < m.presses[j].beat })
	sort.SliceStable(m.intervals, func(i, j int) bool { return m.intervals[i].beat < m.intervals[j].beat })
	sort.SliceStable(m.passes, func(i, j int) bool { return m.passes[i].beat < m.passes[j].beat })
	sort.SliceStable(m.reveals, func(i, j int) bool { return m.reveals[i].beat < m.reveals[j].beat })
	sort.SliceStable(m.reactions, func(i, j int) bool { return m.reactions[i].beat < m.reactions[j].beat })
	sort.SliceStable(m.stages, func(i, j int) bool { return m.stages[i].beat < m.stages[j].beat })
	sort.SliceStable(m.countMods, func(i, j int) bool { return m.countMods[i].beat < m.countMods[j].beat })
	sort.SliceStable(m.forces, func(i, j int) bool { return m.forces[i].beat < m.forces[j].beat })

	for _, ev := range m.intervals {
		ev := ev
		m.ctx.At(ev.beat, func() {
			if m.ctx.GameAt(ev.beat) == m.ID() {
				m.startInterval(ev, ev.beat)
			}
		})
	}
	for _, ev := range m.passes {
		ev := ev
		m.ctx.At(ev.beat, func() {
			if m.ctx.GameAt(ev.beat) == m.ID() {
				m.passTurnStandalone(ev, ev.beat)
			}
		})
	}
	for _, ev := range m.reveals {
		ev := ev
		m.ctx.At(ev.beat, func() { m.revealAnswer(ev) })
	}
	for _, ev := range m.reactions {
		ev := ev
		m.ctx.At(ev.beat, func() { m.answerReaction(ev) })
	}
	for _, ev := range m.stages {
		ev := ev
		m.ctx.At(ev.beat, func() { m.currentStage = clampInt(ev.stage, 0, 4) })
	}
	for _, ev := range m.countMods {
		ev := ev
		m.ctx.At(ev.beat, func() { m.shouldResetCount = ev.reset })
	}
	for _, ev := range m.forces {
		ev := ev
		m.ctx.At(ev.beat, func() { m.forceExplode(ev.target, ev.beat, true) })
	}
}

func (m *Module) OnSwitch(beat float64) {
	for path := range m.ctx.Assets.Animators {
		m.ctx.Scene.PlayDefaultState(path, beat, m.ctx.SecPerBeat(beat))
	}
	m.resetState(beat)
	m.restoreConfig(beat)
	for _, ev := range m.intervals {
		if ev.beat > beat {
			break
		}
		if ev.beat+ev.length+1+ev.length >= beat {
			m.startInterval(ev, beat)
		}
	}
	for _, ev := range m.passes {
		if ev.beat > beat {
			break
		}
		if last, ok := m.previousInterval(ev.beat); ok && ev.beat+ev.length+last.length >= beat {
			m.passTurnStandalone(ev, beat)
		}
	}
}

func (m *Module) Whiff(beat float64) {
	m.WhiffAction(beat, 0)
}

func (m *Module) WhiffAction(beat float64, action int) {
	if !m.playerActiveAt(beat) {
		return
	}
	m.usedRhythm = false
	m.contesteePressButton(action != 2, beat)
}

func (m *Module) Update(_, beat float64) {
	m.bursts = liveBursts(m.bursts, beat)
}

func (m *Module) Draw(screen *ebiten.Image, _, beat float64) {
	screen.Fill(stageColor)
	m.applyTimer(beat)
	m.ctx.SampleScene(beat)
	m.ctx.Scene.Draw(screen, m.proj)
	m.drawBursts(screen, beat)
}

func (m *Module) resetState(beat float64) {
	m.shouldResetCount = false
	m.doingConsecutive = false
	m.shouldPrepareArms = true
	m.contExploded = false
	m.hostExploded = false
	m.signExploded = false
	m.pressCount = 0
	m.countToMatch = 0
	m.currentStage = 0
	m.usedRhythm = true
	m.inputScores = nil
	m.playerWindows = nil
	m.timer = timerState{}
	m.bursts = nil
	m.resetVisuals(beat)
}

func (m *Module) resetVisuals(beat float64) {
	if m.ctx == nil || m.ctx.Scene == nil {
		return
	}
	m.ctx.Scene.SetActive(m.blackOut, false)
	m.ctx.Scene.SetActive(m.timerRoot, false)
	m.ctx.Scene.SetActive(m.contExplosion, false)
	m.ctx.Scene.SetActive(m.hostExplosion, false)
	m.ctx.Scene.SetActive(m.signExplosion, false)
	m.ctx.Scene.SetSpinOver(m.timerNeedle, 0)
	m.ctx.Scene.SetColorOver(m.firstDigit, [4]float64{1, 1, 1, 1})
	m.ctx.Scene.SetColorOver(m.secondDigit, [4]float64{1, 1, 1, 1})
	m.ctx.Scene.SetColorOver(m.hostFirstDigit, [4]float64{1, 1, 1, 1})
	m.ctx.Scene.SetColorOver(m.hostSecondDigit, [4]float64{1, 1, 1, 1})
	m.ctx.Scene.SetSpriteOver(m.contCounter, "")
	m.ctx.Scene.SetSpriteOver(m.hostCounter, "")
	m.setContestantCount(0)
	m.setHostQuestion()
	m.playState(m.sign, "SignIdle", beat)
}

func (m *Module) restoreConfig(beat float64) {
	for _, ev := range m.stages {
		if ev.beat > beat {
			break
		}
		m.currentStage = clampInt(ev.stage, 0, 4)
	}
	for _, ev := range m.countMods {
		if ev.beat > beat {
			break
		}
		m.shouldResetCount = ev.reset
	}
}

func (m *Module) expandRandomPresses() {
	for _, ev := range m.randoms {
		if ev.min > ev.max {
			continue
		}
		amount := ev.min
		if ev.max > ev.min {
			amount += m.rng.Intn(ev.max - ev.min + 1)
		}
		if amount < 1 {
			continue
		}
		if ev.consecutive {
			for i := 0; i < amount; i++ {
				m.presses = append(m.presses, pressEvt{beat: ev.beat + float64(i)*ev.length, dpad: m.randomDpad(ev.which, i)})
			}
			continue
		}
		remaining := amount
		for i := 0; i < ev.max && remaining > 0; i++ {
			slotsLeft := ev.max - i
			if m.rng.Intn(2) == 1 && slotsLeft != remaining {
				continue
			}
			m.presses = append(m.presses, pressEvt{beat: ev.beat + float64(i)*ev.length, dpad: m.randomDpad(ev.which, i)})
			remaining--
		}
	}
}

func (m *Module) randomDpad(which, i int) bool {
	dpad := m.rng.Intn(2) == 1
	switch which {
	case buttonDpadOnly:
		return true
	case buttonAOnly:
		return false
	case buttonAlternatingDpad:
		return i%2 == 0
	case buttonAlternatingA:
		return i%2 != 0
	default:
		return dpad
	}
}

func (m *Module) startInterval(ev intervalEvt, nowBeat float64) {
	if m.startedIntervals[ev.idx] {
		return
	}
	m.startedIntervals[ev.idx] = true
	m.atOrNow(ev.beat, nowBeat, func() {
		m.beginInterval(ev, nowBeat)
	})
	for _, input := range m.inputsBetween(ev.beat, ev.beat+ev.length) {
		input := input
		if input.beat < nowBeat {
			continue
		}
		m.atOrNow(input.beat, nowBeat, func() { m.hostPressButton(input.beat, input.dpad) })
	}
}

func (m *Module) beginInterval(ev intervalEvt, nowBeat float64) {
	if m.shouldPrepareArms {
		m.ctx.Scene.PlayFrozen(m.hostLeftArm, "HostLeftPrepare", 1)
		m.ctx.Scene.PlayFrozen(m.hostRightArm, "HostPrepare", 1)
		m.playState(m.contesteeHead, "ContesteeHeadIdle", ev.beat)
	}
	if !m.doingConsecutive {
		m.pressCount = 0
	}
	m.setContestantCount(0)
	m.setHostQuestion()
	if ev.auto {
		m.passTurn(ev.beat+ev.length, ev.beat, ev.length, ev.timeUpSound, ev.consecutive, ev.visualClock, ev.audioClock, 1, nowBeat)
	}
}

func (m *Module) passTurnStandalone(ev passEvt, nowBeat float64) {
	if m.startedPasses[ev.idx] {
		return
	}
	m.startedPasses[ev.idx] = true
	iv, ok := m.previousInterval(ev.beat)
	if !ok {
		return
	}
	m.passTurn(ev.beat, iv.beat, iv.length, ev.timeUpSound, ev.consecutive, ev.visualClock, ev.audioClock, ev.length, nowBeat)
}

func (m *Module) previousInterval(beat float64) (intervalEvt, bool) {
	for i := len(m.intervals) - 1; i >= 0; i-- {
		if m.intervals[i].beat <= beat {
			return m.intervals[i], true
		}
	}
	return intervalEvt{}, false
}

func (m *Module) passTurn(beat, intervalBeat, intervalLength float64, timeUpSound, consecutive, visualClock bool, audioClock int, length, nowBeat float64) {
	m.inputScores = nil
	ngEarlyBeats := engine.WinNG / m.ctx.SecPerBeat(beat+length)
	playerStart := beat + length - ngEarlyBeats
	playerEnd := beat + length + intervalLength
	m.playerWindows = append(m.playerWindows, playerWindow{start: playerStart, end: playerEnd})

	relevant := m.inputsBetween(intervalBeat, intervalBeat+intervalLength)
	sort.SliceStable(relevant, func(i, j int) bool { return relevant[i].beat < relevant[j].beat })
	for _, input := range relevant {
		input := input
		target := beat + length + input.beat - intervalBeat
		if target < nowBeat-engine.WinNG/m.ctx.SecPerBeat(target) {
			continue
		}
		m.ctx.ScheduleInputAny(target, func(_ float64, j engine.Judgment) {
			m.contesteePressButton(input.dpad, target)
			if j == engine.JudgeNG {
				m.usedRhythm = false
				return
			}
			m.inputScores = append(m.inputScores, judgmentAccuracy(j))
		}, func() {
			m.usedRhythm = false
		})
	}

	for m.countToMatch >= 100 {
		m.countToMatch -= 100
	}
	m.doingConsecutive = consecutive
	timeUpBeat := 0.0
	if audioClock == clockBoth || audioClock == clockStart {
		m.atOrNow(beat, nowBeat, func() { m.ctx.Sound("timerStart") })
		timeUpBeat = 0.5
	}
	if audioClock == clockEnd {
		timeUpBeat = 0.5
	}

	m.atOrNow(beat, nowBeat, func() {
		if consecutive {
			m.countToMatch += len(relevant)
		} else {
			m.countToMatch = len(relevant)
		}
		if m.shouldPrepareArms {
			m.playState(m.contesteeLeftArm, "LeftPrepare", beat)
			m.playState(m.contesteeRightArm, "RIghtPrepare", beat)
		}
		if !consecutive {
			m.playState(m.hostLeftArm, "HostLeftRest", beat)
			m.playState(m.hostRightArm, "HostRightRest", beat)
		}
		m.shouldPrepareArms = false
		if visualClock {
			m.ctx.Scene.SetActive(m.timerRoot, true)
			m.timer = timerState{active: true, start: beat + length, length: intervalLength}
		}
	})

	endBeat := beat + length + intervalLength
	m.atOrNow(endBeat, nowBeat, func() {
		if !consecutive {
			if audioClock == clockBoth || audioClock == clockEnd {
				m.ctx.Sound("timerStop")
			}
			m.playState(m.contesteeLeftArm, "LeftRest", endBeat)
			m.playState(m.contesteeRightArm, "RightRest", endBeat)
			m.shouldPrepareArms = true
		}
		if visualClock {
			m.timer = timerState{}
			m.ctx.Scene.SetActive(m.timerRoot, false)
			m.ctx.Scene.SetSpinOver(m.timerNeedle, 0)
		}
	})

	if timeUpSound && !consecutive {
		m.atOrNow(endBeat+timeUpBeat, nowBeat, func() { m.ctx.Sound("timeUp") })
	}
}

func (m *Module) inputsBetween(beat, endBeat float64) []pressEvt {
	var out []pressEvt
	for _, p := range m.presses {
		if p.beat >= beat && p.beat < endBeat {
			out = append(out, p)
		}
	}
	return out
}

func (m *Module) hostPressButton(beat float64, dpad bool) {
	if m.currentStage == 0 {
		m.playState(m.contesteeHead, "ContesteeHeadIdle", beat)
		m.playState(m.hostHead, "HostIdleHead", beat)
	} else {
		m.playState(m.hostHead, "HostStage"+digitString(clampInt(m.currentStage, 1, 4)), beat)
	}
	if dpad {
		m.ctx.Sound("hostDPad")
		m.playState(m.hostRightArm, "HostRightHit", beat)
		return
	}
	m.ctx.Sound("hostA")
	m.playState(m.hostLeftArm, "HostLeftHit", beat)
}

func (m *Module) contesteePressButton(dpad bool, beat float64) {
	if m.currentStage == 0 {
		m.playState(m.contesteeHead, "ContesteeHeadIdle", beat)
	} else {
		stage := m.currentStage
		if stage == 4 {
			stage = 3
		}
		m.playState(m.contesteeHead, "ContesteeHeadStage"+digitString(clampInt(stage, 1, 3)), beat)
	}
	if dpad {
		m.ctx.Sound("contestantDPad")
		m.playState(m.contesteeLeftArm, "LeftArmPress", beat)
	} else {
		m.ctx.Sound("contestantA")
		m.playState(m.contesteeRightArm, "RightArmHit", beat)
	}
	m.pressCount++
	if m.shouldResetCount && m.pressCount > 99 {
		m.pressCount = 0
	}
	switch {
	case m.pressCount < 100:
		m.setContestantCount(m.pressCount)
	case m.pressCount == 100:
		m.forceExplode(explodeContestant, beat, true)
	case m.pressCount == 120:
		m.forceExplode(explodeHost, beat, true)
	case m.pressCount == 150:
		m.forceExplode(explodeSign, beat, true)
	}
}

func (m *Module) revealAnswer(ev revealEvt) {
	m.ctx.Scene.SetActive(m.blackOut, true)
	m.ctx.At(ev.beat+ev.length, func() {
		m.ctx.Sound("answerReveal")
		m.setHostCount(m.countToMatch)
	})
}

func (m *Module) answerReaction(ev reactionEvt) {
	m.ctx.Scene.SetActive(m.blackOut, false)
	if ev.revealInstant {
		m.ctx.Sound("answerReveal")
		m.setHostCount(m.countToMatch)
	}
	if m.pressCount == m.countToMatch {
		m.ctx.Sound("correct")
		m.playState(m.contesteeHead, "ContesteeSmile", ev.beat)
		m.playState(m.hostHead, "HostSmile", ev.beat)
		if ev.audience {
			m.ctx.Sound("audienceCheer")
		}
		if ev.jingle {
			m.ctx.Sound("correctJingle")
		}
		return
	}
	m.ctx.ScoreMiss()
	m.ctx.Sound("incorrect")
	m.playState(m.contesteeHead, "ContesteeSad", ev.beat)
	m.playState(m.hostHead, "HostSad", ev.beat)
	if ev.audience {
		m.ctx.Sound("audienceSad")
	}
	if ev.jingle {
		m.ctx.Sound("incorrectJingle")
	}
}

func (m *Module) forceExplode(target int, beat float64, sound bool) {
	switch target {
	case explodeContestant:
		if m.contExploded {
			return
		}
		if sound {
			m.ctx.Sound("contestantExplode")
		}
		m.ctx.Scene.SetColorOver(m.firstDigit, [4]float64{1, 1, 1, 0})
		m.ctx.Scene.SetColorOver(m.secondDigit, [4]float64{1, 1, 1, 0})
		m.ctx.Scene.SetSpriteOver(m.contCounter, m.explodedCounter)
		m.contExploded = true
		m.spawnBurst(m.contExplosion, beat)
	case explodeHost:
		if m.hostExploded {
			return
		}
		if sound {
			m.ctx.Sound("hostExplode")
		}
		m.ctx.Scene.SetColorOver(m.hostFirstDigit, [4]float64{1, 1, 1, 0})
		m.ctx.Scene.SetColorOver(m.hostSecondDigit, [4]float64{1, 1, 1, 0})
		m.ctx.Scene.SetSpriteOver(m.hostCounter, m.explodedCounter)
		m.hostExploded = true
		m.spawnBurst(m.hostExplosion, beat)
	case explodeSign:
		if m.signExploded {
			return
		}
		if sound {
			m.ctx.Sound("signExplode")
		}
		m.signExploded = true
		m.playState(m.sign, "Exploded", beat)
		m.spawnBurst(m.signExplosion, beat)
	}
}

func (m *Module) setContestantCount(n int) {
	m.setDigitPair(m.firstDigit, m.secondDigit, m.contestantDigits, n)
}

func (m *Module) setHostCount(n int) {
	m.setDigitPair(m.hostFirstDigit, m.hostSecondDigit, m.hostDigits, n)
}

func (m *Module) setHostQuestion() {
	if len(m.hostDigits) <= 10 {
		return
	}
	m.ctx.Scene.SetSpriteOver(m.hostFirstDigit, m.hostDigits[10])
	m.ctx.Scene.SetSpriteOver(m.hostSecondDigit, m.hostDigits[10])
}

func (m *Module) setDigitPair(onesPath, tensPath string, sprites []string, n int) {
	if len(sprites) < 10 {
		return
	}
	m.ctx.Scene.SetSpriteOver(onesPath, sprites[specificDigit(n, 1)])
	m.ctx.Scene.SetSpriteOver(tensPath, sprites[specificDigit(n, 2)])
}

func (m *Module) playState(path, state string, beat float64) {
	m.ctx.Scene.PlayState(path, state, beat, 0.5)
}

func (m *Module) atOrNow(target, now float64, fn func()) {
	if target <= now+1e-6 {
		fn()
		return
	}
	m.ctx.At(target, fn)
}

func (m *Module) playerActiveAt(beat float64) bool {
	for _, w := range m.playerWindows {
		if beat >= w.start && beat <= w.end {
			return true
		}
	}
	return false
}

func (m *Module) applyTimer(beat float64) {
	if !m.timer.active || m.timer.length <= 0 {
		return
	}
	u := (beat - m.timer.start) / m.timer.length
	if u < 0 || u > 1 {
		return
	}
	m.ctx.Scene.SetSpinOver(m.timerNeedle, -2*math.Pi*u)
}

func (m *Module) spawnBurst(path string, beat float64) {
	m.ctx.SampleScene(beat)
	world, ok := m.ctx.Scene.NodeWorld(path)
	if !ok {
		return
	}
	x, y := world.Apply(0, 0)
	// Unity ParticleSystem data is serialized under the prefab component, but
	// Ebitengine has no particle runtime. This replacement keeps the authored
	// emitter anchors and uses a short radial burst so the explosion events are
	// visible and timed instead of being silently dropped.
	b := burst{beat: beat, origin: [2]float64{x, y}}
	for i := 0; i < 24; i++ {
		b.sparks = append(b.sparks, spark{
			angle: float64(i)*math.Pi*2/24 + m.rng.Float64()*0.22,
			speed: 1.5 + m.rng.Float64()*2.4,
			life:  0.35 + m.rng.Float64()*0.35,
			size:  2 + float32(m.rng.Float64()*3),
		})
	}
	m.bursts = append(m.bursts, b)
}

func liveBursts(in []burst, beat float64) []burst {
	out := in[:0]
	for _, b := range in {
		maxLife := 0.0
		for _, sp := range b.sparks {
			maxLife = math.Max(maxLife, sp.life)
		}
		if beat-b.beat <= maxLife+0.1 {
			out = append(out, b)
		}
	}
	return out
}

func (m *Module) drawBursts(screen *ebiten.Image, beat float64) {
	for _, b := range m.bursts {
		elapsed := m.ctx.BeatToTime(beat) - m.ctx.BeatToTime(b.beat)
		if elapsed < 0 {
			continue
		}
		ox, oy := m.proj.Apply(b.origin[0], b.origin[1])
		for _, sp := range b.sparks {
			u := elapsed / sp.life
			if u < 0 || u > 1 {
				continue
			}
			dist := sp.speed * elapsed * 54
			x := float32(ox + math.Cos(sp.angle)*dist)
			y := float32(oy - math.Sin(sp.angle)*dist)
			alpha := float32(1 - u)
			vector.DrawFilledCircle(screen, x, y, sp.size*(1+float32(u)), color.RGBA{0xff, 0xff, 0xff, uint8(220 * alpha)}, false)
			vector.StrokeLine(screen, float32(ox), float32(oy), x, y, 2*alpha, color.RGBA{0xff, 0x5e, 0x7c, uint8(200 * alpha)}, false)
		}
	}
}

func specificDigit(num, nth int) int {
	if num < 0 {
		num = 0
	}
	pow := 1
	for i := 1; i < nth; i++ {
		pow *= 10
	}
	return (num / pow) % 10
}

func judgmentAccuracy(j engine.Judgment) float64 {
	switch j {
	case engine.JudgeAce:
		return 1
	case engine.JudgeJust:
		return 0.9
	default:
		return 0
	}
}

func digitString(v int) string {
	if v >= 0 && v <= 9 {
		return string(rune('0' + v))
	}
	return "0"
}

func boolParam(e *riq.Entity, key string) bool { return e.Float(key, 0) != 0 }

func boolDefault(e *riq.Entity, key string, def bool) bool {
	if _, ok := e.Data[key]; !ok {
		return def
	}
	return e.Float(key, 0) != 0
}

func intParam(e *riq.Entity, key string, def int) int {
	return int(e.Float(key, float64(def)))
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

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func durationDefault(v, def float64) float64 {
	if v <= 0 {
		return def
	}
	return v
}
