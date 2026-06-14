// Package ringside ports Ringside's interview cues, wrestler/reporter
// reactions, camera flashes, pose zoom, and newspaper gag.
package ringside

import (
	"fmt"
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
	flickAction = 1 // HS InputAction_FlickPress; mapped to F/arrow side input.
	poseAction  = 3 // HS InputAction_Alt/South; mapped to L/Down/X.
)

const (
	questionRandom = 4
	poseRandom     = 3
)

var bgDefault = [4]float64{0x5a / 255.0, 0x5a / 255.0, 0x5a / 255.0, 1}

type bopEvt struct {
	beat, length     float64
	manual, autoMode bool
}

type questionEvt struct {
	beat, length float64
	alt          bool
	variant      int
	flash        bool
}

type bigGuyEvt struct {
	beat    float64
	variant int
	flash   bool
}

type poseEvt struct {
	beat           float64
	and            bool
	variant        int
	keepZoomedOut  bool
	newspaperBeats float64
	ease           int
}

type sweatEvt struct {
	beat float64
	on   bool
}

type fadeEvt struct {
	beat, length float64
	from, to     [4]float64
}

type poseCam struct {
	beat          float64
	keepZoomedOut bool
	ease          int
}

type paperState struct {
	active bool
	end    float64
}

type particle struct {
	born      float64
	life      float64
	x, y      float64
	vx, vy    float64
	rot, spin float64
	scale     float64
	sprite    string
	tint      [4]float64
}

type Module struct {
	ctx        *engine.Ctx
	proj       kart.Aff
	paperScene *kart.SceneInst

	bops      []bopEvt
	questions []questionEvt
	bigGuys   []bigGuyEvt
	poses     []poseEvt
	sweats    []sweatEvt

	lastBeat      int
	canBop        bool
	camFlash      bool
	shouldNoInput bool
	missedBigGuy  bool
	reporterHeart bool
	hitPose       bool
	currentPose   int

	flash fadeEvt
	bg    fadeEvt
	bgNow [4]float64

	poseCams     []poseCam
	keepZoomOut  bool
	activePaper  paperState
	kidsLaughEnd func()

	flashParts []particle
	sweatParts []particle
	sweatOn    bool
}

func New() engine.Module { return &Module{canBop: true, bgNow: bgDefault} }

func (m *Module) ID() string { return "ringside" }

func (m *Module) Load(ctx *engine.Ctx) error {
	m.ctx = ctx
	if err := ctx.LoadAssets("ringside"); err != nil {
		return err
	}
	m.proj = kart.Translate(engine.ScreenW/2, engine.ScreenH/2).Mul(kart.Scale(35, -35))
	m.paperScene = kart.NewScene(ctx.Assets)
	m.initScene(ctx.Scene)
	m.initPaperScene()
	return nil
}

func (m *Module) initScene(sc *kart.SceneInst) {
	sc.SetActive("Flash", false)
	sc.SetActive("PoseFlash", false)
	sc.SetActive("Newspaper", false)
	m.playDefaultStates(sc, 0)
}

func (m *Module) initPaperScene() {
	for _, p := range []string{"BG2", "Reporter", "Wrestler", "Audience", "Flash", "PoseFlash"} {
		m.paperScene.SetActive(p, false)
	}
	m.paperScene.SetActive("Newspaper", false)
}

func (m *Module) playDefaultStates(sc *kart.SceneInst, beat float64) {
	sec := m.ctx.SecPerBeat(beat)
	sc.PlayDefaultState("Wrestler", beat, sec)
	sc.PlayDefaultState("Reporter", beat, sec)
	sc.PlayDefaultState("Reporter/Upper/HeadAnim", beat, sec)
	sc.PlayDefaultState("Audience", beat, sec)
}

func (m *Module) OnEvent(e *riq.Entity) {
	switch e.Datamodel {
	case "ringside/toggleBop":
		m.bops = append(m.bops, bopEvt{
			beat: e.Beat, length: e.Length,
			manual: boolDefault(e, "bop2", true), autoMode: boolParam(e, "bop"),
		})
	case "ringside/question", "ringside/questionScaled":
		length := e.Length
		if length <= 0 {
			length = 4
		}
		m.questions = append(m.questions, questionEvt{
			beat: e.Beat, length: length, alt: boolParam(e, "alt"),
			variant: int(e.Float("variant", questionRandom)), flash: boolDefault(e, "flash", true),
		})
	case "ringside/woahYouGoBigGuy":
		m.bigGuys = append(m.bigGuys, bigGuyEvt{
			beat: e.Beat, variant: int(e.Float("variant", questionRandom)), flash: boolDefault(e, "flash", true),
		})
	case "ringside/poseForTheFans":
		m.poses = append(m.poses, poseEvt{
			beat: e.Beat, and: boolParam(e, "and"), variant: int(e.Float("variant", poseRandom)),
			keepZoomedOut: boolParam(e, "keepZoomedOut"), newspaperBeats: e.Float("newspaperBeats", 0),
			ease: int(e.Float("ease", 3)),
		})
	case "ringside/toggleSweat":
		m.sweats = append(m.sweats, sweatEvt{beat: e.Beat, on: boolParam(e, "sweat")})
	}
}

func (m *Module) Ready() {
	sort.SliceStable(m.bops, func(i, j int) bool { return m.bops[i].beat < m.bops[j].beat })
	sort.SliceStable(m.questions, func(i, j int) bool { return m.questions[i].beat < m.questions[j].beat })
	sort.SliceStable(m.bigGuys, func(i, j int) bool { return m.bigGuys[i].beat < m.bigGuys[j].beat })
	sort.SliceStable(m.poses, func(i, j int) bool { return m.poses[i].beat < m.poses[j].beat })
	sort.SliceStable(m.sweats, func(i, j int) bool { return m.sweats[i].beat < m.sweats[j].beat })

	for _, ev := range m.bops {
		ev := ev
		if ev.manual {
			for b := ev.beat; b < ev.beat+ev.length; b++ {
				bb := b
				m.ctx.At(bb, func() { m.bop(bb) })
			}
		}
	}
	for _, ev := range m.questions {
		ev := ev
		m.scheduleQuestion(ev)
	}
	for _, ev := range m.bigGuys {
		ev := ev
		m.scheduleBigGuy(ev)
	}
	for _, ev := range m.poses {
		ev := ev
		m.schedulePose(ev)
	}
	for _, ev := range m.sweats {
		ev := ev
		m.ctx.At(ev.beat, func() { m.sweatOn = ev.on })
	}
}

func (m *Module) OnSwitch(beat float64) {
	m.canBop = true
	m.camFlash = true
	m.shouldNoInput = false
	m.hitPose = false
	m.playDefaultStates(m.ctx.Scene, beat)
	m.ctx.Scene.SetActive("PoseFlash", false)
	m.ctx.Scene.SetActive("Flash", false)
	m.ctx.Scene.SetScaleOver("Wrestler", 1, 1)
	m.restorePaperState(beat)
	m.lastBeat = int(math.Floor(beat)) - 1
}

func (m *Module) restorePaperState(beat float64) {
	m.activePaper = paperState{}
	m.keepZoomOut = false
	for _, ev := range m.poses {
		end := ev.beat + 3 + ev.newspaperBeats
		if ev.newspaperBeats > 0 && beat >= ev.beat+3 && beat < end {
			m.activePaper = paperState{active: true, end: end}
			m.keepZoomOut = true
			m.paperScene.SetActive("Newspaper", true)
			m.paperScene.PlayState("Newspaper", "NewspaperIdle", beat, m.ctx.SecPerBeat(beat))
		}
	}
	if !m.activePaper.active {
		m.paperScene.SetActive("Newspaper", false)
	}
}

func (m *Module) Whiff(beat float64) { m.WhiffAction(beat, 0) }

func (m *Module) WhiffAction(beat float64, action int) {
	if m.shouldNoInput {
		return
	}
	m.ctx.ScoreMiss()
	switch action {
	case flickAction:
		m.ctx.Sound("muscles2")
		m.playWrestler("BigGuyTwo", beat, 0.5)
		m.playReporter("FlinchReporter", beat, 0.5)
		m.playHead("Flinch", beat, 0.5)
		m.ctx.Sound("barely")
	case poseAction:
		pose := 1 + rand.Intn(6)
		m.playWrestler(fmt.Sprintf("Pose%d", pose), beat, 0.5)
		m.playReporter("FlinchReporter", beat, 0.5)
		m.playHead("Flinch", beat, 0.5)
		m.ctx.Sound(fmt.Sprintf("badpose_%d", 1+rand.Intn(6)))
		m.ctx.Scene.SetScaleOver("Wrestler", 1.1, 1.1)
		m.ctx.At(beat+0.1, func() { m.ctx.Scene.SetScaleOver("Wrestler", 1, 1) })
	default:
		m.playWrestler("YeMiss", beat, 0.25)
		m.ctx.Sound("confusedanswer")
		m.playHead("Miss", beat, 0.5)
	}
}

func (m *Module) Update(_, beat float64) {
	whole := int(math.Floor(beat))
	if whole != m.lastBeat {
		m.lastBeat = whole
		if m.canBop && m.autoBopAt(float64(whole)) {
			m.bop(float64(whole))
		}
	}
	if m.sweatOn && rand.Intn(8) == 0 {
		m.sweatParts = append(m.sweatParts, particle{
			born: beat, life: 1.1, x: 2.7 + rand.Float64()*1.0, y: -2.2 + rand.Float64()*1.1,
			vx: rand.Float64()*0.12 - 0.06, vy: -0.45 - rand.Float64()*0.3,
			scale: 0.55 + rand.Float64()*0.2, sprite: "Sweat", tint: [4]float64{1, 1, 1, 0.9},
		})
	}
	m.pruneParticles(beat)
}

func (m *Module) Draw(screen *ebiten.Image, _ float64, beat float64) {
	bg := m.bgAt(beat)
	screen.Fill(toRGBA(bg))
	cam := m.cameraAt(beat)
	appCam := m.ctx.CameraAt(beat)

	sc := m.ctx.Scene
	sc.SetCamera(appCam[0]+cam[0], appCam[1]+cam[1], appCam[2]+cam[2])
	sc.Sample(beat)
	sc.Draw(screen, m.proj)
	m.drawSweat(screen, beat)

	if m.activePaper.active {
		vector.DrawFilledRect(screen, 0, 0, engine.ScreenW, engine.ScreenH, color.RGBA{0, 0, 0, 255}, false)
		m.paperScene.SetCamera(appCam[0]+cam[0], appCam[1]+cam[1], appCam[2]+cam[2])
		m.paperScene.Sample(beat)
		m.paperScene.Draw(screen, m.proj)
	}

	m.drawFlashParticles(screen, beat)
	if c := m.flashAt(beat); c[3] > 0 {
		vector.DrawFilledRect(screen, 0, 0, engine.ScreenW, engine.ScreenH, toRGBA(c), false)
	}
}

func (m *Module) scheduleQuestion(ev questionEvt) {
	beat := ev.beat
	variant := chooseQuestionVariant(ev.variant)
	if ev.length <= 2 {
		m.ctx.At(beat-0.5, func() {
			m.playReporter("WubbaLubbaDubbaThatTrue", beat-0.5, 0.4)
			m.playHead("Wubba", beat-0.5, 0.4)
		})
		m.thatTrue(beat-1, variant, ev.flash)
		return
	}
	m.ctx.At(beat, func() {
		m.playReporter("WubbaLubbaDubbaThatTrue", beat, 0.4)
		m.playHead("Wubba", beat, 0.4)
	})
	m.ctx.SoundAtOff(beat, fmt.Sprintf("en/wubdub_var%d_1", variant), 1, 0.015)
	if !ev.alt {
		m.ctx.SoundAtOff(beat+0.25, fmt.Sprintf("en/wubdub_var%d_2", variant), 1, 0.002)
	}
	extend := ev.length - 3
	totalExtend := 0
	if extend > 0 {
		for i := 0; i < int(extend); i++ {
			b := beat + float64(i)
			m.ctx.SoundAtOff(b+0.5, fmt.Sprintf("en/wubdub_var%d_3", variant), 1, 0.003)
			m.ctx.SoundAt(b+0.75, fmt.Sprintf("en/wubdub_var%d_4", variant), 1)
			m.ctx.SoundAt(b+1, fmt.Sprintf("en/wubdub_var%d_5", variant), 1)
			m.ctx.SoundAt(b+1.25, fmt.Sprintf("en/wubdub_var%d_6", variant), 1)
			totalExtend++
		}
	}
	m.thatTrue(beat+float64(totalExtend), variant, ev.flash)
}

func (m *Module) thatTrue(beat float64, variant int, doFlash bool) {
	m.ctx.SoundAtOff(beat+0.5, fmt.Sprintf("en/wubdub_var%d_7", variant), 1, 0.025)
	m.ctx.SoundAt(beat+0.5, "wubdub_konk_1", 1)
	m.ctx.SoundAt(beat+0.75, "wubdub_konk_2", 1)
	m.ctx.SoundAtOff(beat+1, fmt.Sprintf("en/wubdub_var%d_8", variant), 1, 0.018)
	m.ctx.SoundAt(beat+1, "wubdub_konk_3", 1)
	m.ctx.SoundAt(beat+1, "mic_swoosh", 1)
	target := beat + 2
	m.ctx.ScheduleInputCond(target,
		func() bool { return m.ctx.GameAt(target) == m.ID() },
		func(state float64, _ engine.Judgment) { m.hitQuestion(target, state) },
		func() { m.missQuestion(target) },
	)
	m.ctx.At(beat+0.5, func() {
		m.playReporter("ThatTrue", beat+0.5, 0.5)
		m.playHead("IsThat", beat+0.5, 0.5)
	})
	m.ctx.At(beat+1.5, func() {
		m.canBop = false
		m.camFlash = doFlash
	})
	m.ctx.At(beat+2.5, func() { m.canBop = true })
}

func (m *Module) hitQuestion(target, state float64) {
	if state >= 1 || state <= -1 {
		m.playWrestler("Cough", target, 0.5)
		m.ctx.Sound("cough")
		m.playHead("Late", target, 0.5)
		m.ctx.Sound(fmt.Sprintf("huhaudience%d", rand.Intn(2)))
		m.ctx.At(target+0.9, func() { m.reporterIdle(target + 0.9) })
		return
	}
	m.playWrestler("Ye", target, 0.5)
	m.playHead("ExtendSmile", target, 0.5)
	m.ctx.Sound(fmt.Sprintf("ye%d", 1+rand.Intn(3)))
	m.ctx.At(target+0.5, func() {
		m.ctx.Sound("yeCamera")
		if m.camFlash {
			m.startFlash(target+0.5, 0.5)
			m.ctx.Scene.SetActive("Flash", true)
		}
		m.playReporter("IdleReporter", target+0.5, 0.5)
		m.playHead("Smile", target+0.5, 0.5)
	})
	m.ctx.At(target+0.6, func() { m.ctx.Scene.SetActive("Flash", false) })
	m.ctx.At(target+0.9, func() { m.playHead("Idle", target+0.9, 0.5) })
}

func (m *Module) missQuestion(target float64) {
	m.playHead("Late", target, 0.5)
	m.ctx.Sound(fmt.Sprintf("huhaudience%d", rand.Intn(2)))
	m.ctx.At(target+0.5, func() { m.playReporter("IdleReporter", target+0.5, 0.5) })
	m.ctx.At(target+0.9, func() { m.playHead("Idle", target+0.9, 0.5) })
}

func (m *Module) scheduleBigGuy(ev bigGuyEvt) {
	beat := ev.beat
	variant := chooseQuestionVariant(ev.variant)
	m.ctx.At(beat, func() {
		m.playReporter("Woah", beat, 0.5)
		m.playHead("Woah", beat, 0.5)
	})
	youBeat := 0.081
	if variant == 3 {
		youBeat = 0.027
	}
	m.ctx.SoundAtOff(beat, fmt.Sprintf("en/bigguy_var%d_1", variant), 1, youBeat)
	m.ctx.SoundAtOff(beat+0.75, fmt.Sprintf("en/bigguy_var%d_2", variant), 1, youBeat)
	m.ctx.SoundAt(beat+1, fmt.Sprintf("en/bigguy_var%d_3", variant), 1)
	m.ctx.SoundAtOff(beat+1.5, fmt.Sprintf("en/bigguy_var%d_4", variant), 1, 0.006)
	m.ctx.SoundAtOff(beat+2, fmt.Sprintf("en/bigguy_var%d_5", variant), 1, 0.009)
	m.ctx.SoundAt(beat+2, "mic_swoosh", 1)

	first := beat + 2.5
	second := beat + 3
	m.ctx.ScheduleInputCond(first,
		func() bool { return m.ctx.GameAt(first) == m.ID() },
		func(state float64, _ engine.Judgment) { m.hitBigGuyFirst(first, state) },
		func() { m.missedBigGuy = true },
	)
	m.ctx.ScheduleInputActionCond(second, flickAction,
		func() bool { return m.ctx.GameAt(second) == m.ID() },
		func(state float64, _ engine.Judgment) { m.hitBigGuySecond(second, state) },
		func() { m.missBigGuySecond(second) },
	)
	m.ctx.At(beat+2, func() {
		m.playReporter("true", beat+2, 0.5)
		m.playHead("Guy", beat+2, 0.5)
	})
	m.ctx.At(beat+2.25, func() {
		m.canBop = false
		m.camFlash = ev.flash
	})
	m.ctx.At(beat+3.5, func() { m.canBop = true })
}

func (m *Module) hitBigGuyFirst(beat, state float64) {
	m.ctx.Sound("muscles1")
	m.playWrestler("BigGuyOne", beat, 0.5)
	m.missedBigGuy = state >= 1 || state <= -1
}

func (m *Module) hitBigGuySecond(beat, state float64) {
	m.ctx.Sound("muscles2")
	m.playWrestler("BigGuyTwo", beat, 0.5)
	if state >= 1 || state <= -1 {
		if m.missedBigGuy {
			m.playHead("Late", beat, 0.5)
			m.ctx.Sound(fmt.Sprintf("huhaudience%d", rand.Intn(2)))
		}
		m.ctx.At(beat+0.5, func() { m.playReporter("IdleReporter", beat+0.5, 0.5) })
		m.ctx.At(beat+0.9, func() { m.playHead("Idle", beat+0.9, 0.5) })
		return
	}
	if !m.missedBigGuy {
		m.playHead("ExtendSmile", beat, 0.5)
		m.ctx.At(beat+0.5, func() {
			m.ctx.Sound("musclesCamera")
			m.playReporter("IdleReporter", beat+0.5, 0.5)
			m.playHead("Smile", beat+0.5, 0.5)
			if m.camFlash {
				m.startFlash(beat+0.5, 0.5)
				m.ctx.Scene.SetActive("Flash", true)
			}
		})
		m.ctx.At(beat+0.6, func() { m.ctx.Scene.SetActive("Flash", false) })
		m.ctx.At(beat+0.9, func() { m.playHead("Idle", beat+0.9, 0.5) })
		return
	}
	m.playHead("Miss", beat, 0.5)
	m.ctx.At(beat+0.5, func() { m.playReporter("IdleReporter", beat+0.5, 0.5) })
	m.ctx.At(beat+0.9, func() { m.playHead("Idle", beat+0.9, 0.5) })
}

func (m *Module) missBigGuySecond(beat float64) {
	m.playHead("Late", beat, 0.5)
	m.ctx.Sound(fmt.Sprintf("huhaudience%d", rand.Intn(2)))
	m.ctx.At(beat+0.5, func() { m.playReporter("IdleReporter", beat+0.5, 0.5) })
	m.ctx.At(beat+0.9, func() {
		m.playHead("Idle", beat+0.9, 0.5)
		m.playWrestler("Idle", beat+0.9, 0.5)
	})
}

func (m *Module) schedulePose(ev poseEvt) {
	beat := ev.beat
	if ev.and {
		m.ctx.SoundAt(beat-0.5, "en/pose_and", 1)
	}
	variant := ev.variant
	if variant == poseRandom {
		variant = 1 + rand.Intn(2)
	}
	m.ctx.SoundAtOff(beat, fmt.Sprintf("en/pose_var%d_1", variant), 1, 0.02)
	m.ctx.SoundAt(beat+0.3333, fmt.Sprintf("en/pose_var%d_2", variant), 1)
	m.ctx.SoundAt(beat+0.5, fmt.Sprintf("en/pose_var%d_3", variant), 1)
	m.ctx.SoundAtOff(beat+0.75, fmt.Sprintf("en/pose_var%d_4", variant), 1, 0.022)
	m.ctx.SoundAtOff(beat+1, fmt.Sprintf("en/pose_var%d_5", variant), 1, 0.035)
	m.ctx.SoundAt(beat+1.8, fmt.Sprintf("en/pose_var%d_6", variant), 1)
	m.poseCams = append(m.poseCams, poseCam{beat: beat, keepZoomedOut: ev.keepZoomedOut, ease: ev.ease})

	m.ctx.At(beat, func() {
		if m.keepZoomOut {
			m.ctx.Scene.PlayState("Audience", "PoseAudienceZoomed", beat, 0.25)
		} else {
			m.ctx.Scene.PlayState("Audience", "PoseAudience", beat, 0.25)
		}
	})
	m.ctx.At(beat+1, func() {
		m.reporterHeart = ev.newspaperBeats > 0
		m.playWrestler("PreparePose", beat+1, 0.25)
		m.canBop = false
	})
	target := beat + 3
	m.ctx.ScheduleInputActionCond(target, poseAction,
		func() bool { return m.ctx.GameAt(target) == m.ID() },
		func(state float64, _ engine.Judgment) { m.hitPoseInput(target, state) },
		func() { m.missPose(target) },
	)
	m.ctx.At(beat+4, func() {
		if m.autoBopAt(beat + 4) {
			m.bop(beat + 4)
		} else {
			m.playWrestler("Idle", beat+4, 0.5)
		}
		m.shouldNoInput = false
		m.canBop = true
		m.playReporter("IdleReporter", beat+4, 0.5)
		m.playHead("Idle", beat+4, 0.5)
	})
	if ev.keepZoomedOut {
		m.ctx.At(beat+2.5, func() { m.keepZoomOut = true })
	} else if ev.newspaperBeats <= 0 {
		m.ctx.At(beat+3.99, func() { m.keepZoomOut = false })
	}
	if ev.newspaperBeats > 0 {
		m.ctx.At(beat+3, func() { m.showNewspaper(beat+3, ev.keepZoomedOut, ev.newspaperBeats) })
		m.ctx.At(beat+3+ev.newspaperBeats, func() { m.hideNewspaper() })
	}
}

func (m *Module) hitPoseInput(beat, state float64) {
	m.shouldNoInput = true
	m.ctx.Scene.SetScaleOver("Wrestler", 1.2, 1.2)
	pose := 1 + rand.Intn(6)
	if state >= 1 || state <= -1 {
		m.playWrestler(fmt.Sprintf("Pose%d", pose), beat, 0.5)
		m.ctx.Sound(fmt.Sprintf("badpose_%d", 1+rand.Intn(6)))
		m.playHead("Late", beat, 0.5)
		m.ctx.Sound(fmt.Sprintf("huhaudience%d", rand.Intn(2)))
		m.ctx.At(beat+0.1, func() { m.ctx.Scene.SetScaleOver("Wrestler", 1, 1) })
		return
	}
	m.currentPose = pose
	m.hitPose = true
	m.playWrestler(fmt.Sprintf("Pose%d", pose), beat, 0.5)
	if m.reporterHeart {
		m.playReporter("HeartReporter", beat, 0.5)
		m.playHead("Heart", beat, 0.5)
	} else {
		m.playReporter("ExcitedReporter", beat, 0.5)
		m.playHead("Excited", beat, 0.5)
	}
	m.ctx.Sound(fmt.Sprintf("yell%d", 1+rand.Intn(6)))
	m.startFlash(beat, 1)
	m.bg = fadeEvt{beat: beat, length: 1, from: [4]float64{0, 0, 0, 1}, to: bgDefault}
	m.spawnFlashParticles(beat)
	m.ctx.At(beat+0.1, func() { m.ctx.Scene.SetScaleOver("Wrestler", 1, 1) })
	m.ctx.At(beat+1, func() {
		m.ctx.Sound("poseCamera")
		m.ctx.Scene.SetActive("PoseFlash", true)
		m.ctx.Scene.PlayState("PoseFlash", "PoseFlashing", beat+1, m.ctx.SecPerBeat(beat+1))
	})
	m.ctx.At(beat+1.99, func() { m.ctx.Scene.SetActive("PoseFlash", false) })
}

func (m *Module) missPose(beat float64) {
	m.shouldNoInput = true
	m.playHead("Late", beat, 0.5)
	m.ctx.Sound(fmt.Sprintf("huhaudience%d", rand.Intn(2)))
}

func (m *Module) showNewspaper(beat float64, keep bool, duration float64) {
	m.keepZoomOut = true
	m.activePaper = paperState{active: true, end: beat + duration}
	m.paperScene.SetActive("Newspaper", true)
	if rand.Intn(2) == 0 {
		m.paperScene.PlayState("Newspaper", "NewspaperEnter", beat, m.ctx.SecPerBeat(beat))
	} else {
		m.paperScene.PlayState("Newspaper", "NewspaperEnterRight", beat, m.ctx.SecPerBeat(beat))
	}
	if m.hitPose {
		m.paperScene.PlayState("Newspaper/WrestlerNewspaper", fmt.Sprintf("Pose%d", m.currentPose), beat, 0.5)
		m.paperScene.PlayState("Newspaper/ReporterNewspaper", "HeartReporterNewspaper", beat, 0.5)
	} else {
		m.paperScene.PlayState("Newspaper/WrestlerNewspaper", fmt.Sprintf("Miss%dNewspaper", 1+rand.Intn(6)), beat, 0.5)
		m.paperScene.PlayState("Newspaper/ReporterNewspaper", "IdleReporterNewspaper", beat, 0.5)
		m.kidsLaughEnd = m.ctx.SoundLoop("kidslaugh")
	}
	_ = keep
}

func (m *Module) hideNewspaper() {
	m.activePaper = paperState{}
	m.paperScene.SetActive("Newspaper", false)
	if m.kidsLaughEnd != nil {
		m.kidsLaughEnd()
		m.kidsLaughEnd = nil
	}
	m.keepZoomOut = false
}

func (m *Module) bop(beat float64) {
	if !m.canBop || m.ctx.GameAt(beat) != m.ID() {
		return
	}
	if rand.Intn(17) == 0 {
		m.playWrestler("BopPec", beat, 0.5)
	} else {
		m.playWrestler("Bop", beat, 0.5)
	}
}

func (m *Module) autoBopAt(beat float64) bool {
	on := true
	for _, ev := range m.bops {
		if ev.beat > beat {
			break
		}
		on = ev.autoMode
	}
	return on
}

func (m *Module) playWrestler(state string, beat, scale float64) {
	m.ctx.Scene.PlayState("Wrestler", state, beat, scale)
}

func (m *Module) playReporter(state string, beat, scale float64) {
	m.ctx.Scene.PlayState("Reporter", state, beat, scale)
}

func (m *Module) playHead(state string, beat, scale float64) {
	m.ctx.Scene.PlayState("Reporter/Upper/HeadAnim", state, beat, scale)
}

func (m *Module) reporterIdle(beat float64) {
	m.playReporter("IdleReporter", beat, 0.5)
	m.playHead("Idle", beat, 0.5)
}

func (m *Module) startFlash(beat, length float64) {
	m.flash = fadeEvt{beat: beat, length: length, from: [4]float64{1, 1, 1, 1}, to: [4]float64{1, 1, 1, 0}}
}

func (m *Module) flashAt(beat float64) [4]float64 {
	return fadeAt(m.flash, beat, [4]float64{})
}

func (m *Module) bgAt(beat float64) [4]float64 {
	return fadeAt(m.bg, beat, bgDefault)
}

func fadeAt(f fadeEvt, beat float64, fallback [4]float64) [4]float64 {
	if f.length <= 0 || beat < f.beat {
		return fallback
	}
	if beat >= f.beat+f.length {
		return f.to
	}
	u := clamp01((beat - f.beat) / f.length)
	var out [4]float64
	for i := range out {
		out[i] = f.from[i] + (f.to[i]-f.from[i])*u
	}
	return out
}

func (m *Module) cameraAt(beat float64) [3]float64 {
	var current *poseCam
	for i := range m.poseCams {
		if m.poseCams[i].beat <= beat {
			current = &m.poseCams[i]
		}
	}
	if current == nil {
		return [3]float64{}
	}
	u := (beat - current.beat) / 2.5
	stop := (beat - current.beat) / 3.99
	if stop > 1 && !m.keepZoomOut && !current.keepZoomedOut {
		return [3]float64{}
	}
	// SceneInst receives global camera + this local offset. The local z target
	// is -11.5 so the final absolute camera reaches HS' -21.5 pose zoom.
	target := [3]float64{2.7, -4.61, -11.5}
	if u >= 1 {
		return target
	}
	u = clamp01(u)
	return [3]float64{
		engine.Ease(current.ease, 0, target[0], u),
		engine.Ease(current.ease, 0, target[1], u),
		engine.Ease(current.ease, 0, target[2], u),
	}
}

func (m *Module) spawnFlashParticles(beat float64) {
	for i := 0; i < 18; i++ {
		m.flashParts = append(m.flashParts, particle{
			born: beat, life: 1.0 + rand.Float64()*0.35,
			x: 2.7 + rand.Float64()*6 - 3, y: -4.6 + rand.Float64()*4 - 2,
			vx: rand.Float64()*0.8 - 0.4, vy: rand.Float64()*0.7 - 0.15,
			rot: rand.Float64() * math.Pi, spin: rand.Float64()*4 - 2,
			scale: 0.45 + rand.Float64()*0.55, sprite: "Flash",
			tint: [4]float64{1, 1, 1, 0.9},
		})
	}
}

func (m *Module) drawFlashParticles(screen *ebiten.Image, beat float64) {
	for _, p := range m.flashParts {
		age := beat - p.born
		if age < 0 || age > p.life {
			continue
		}
		u := age / p.life
		w := kart.Translate(p.x+p.vx*age, p.y+p.vy*age).
			Mul(kart.Rotate(p.rot + p.spin*age)).
			Mul(kart.Scale(p.scale*(1+u), p.scale*(1+u)))
		tint := p.tint
		tint[3] *= 1 - u
		m.ctx.Assets.DrawSpriteTint(screen, p.sprite, w, m.proj, false, tint)
	}
}

func (m *Module) drawSweat(screen *ebiten.Image, beat float64) {
	for _, p := range m.sweatParts {
		age := beat - p.born
		if age < 0 || age > p.life {
			continue
		}
		u := age / p.life
		w := kart.Translate(p.x+p.vx*age, p.y+p.vy*age).
			Mul(kart.Rotate(p.rot + p.spin*age)).
			Mul(kart.Scale(p.scale, p.scale))
		tint := p.tint
		tint[3] *= 1 - u
		m.ctx.Assets.DrawSpriteTint(screen, p.sprite, w, m.proj, false, tint)
	}
}

func (m *Module) pruneParticles(beat float64) {
	keepFlash := m.flashParts[:0]
	for _, p := range m.flashParts {
		if beat < p.born+p.life {
			keepFlash = append(keepFlash, p)
		}
	}
	m.flashParts = keepFlash
	keepSweat := m.sweatParts[:0]
	for _, p := range m.sweatParts {
		if beat < p.born+p.life {
			keepSweat = append(keepSweat, p)
		}
	}
	m.sweatParts = keepSweat
}

func chooseQuestionVariant(v int) int {
	if v == questionRandom || v < 1 || v > 3 {
		return 1 + rand.Intn(3)
	}
	return v
}

func boolParam(e *riq.Entity, key string) bool { return e.Float(key, 0) != 0 }

func boolDefault(e *riq.Entity, key string, def bool) bool {
	if _, ok := e.Data[key]; !ok {
		return def
	}
	return e.Float(key, 0) != 0
}

func toRGBA(c [4]float64) color.RGBA {
	return color.RGBA{
		R: uint8(clamp01(c[0]) * 255),
		G: uint8(clamp01(c[1]) * 255),
		B: uint8(clamp01(c[2]) * 255),
		A: uint8(clamp01(c[3]) * 255),
	}
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
