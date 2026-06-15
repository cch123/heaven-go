// Package drummerduel ports Drummer Duel's call-and-response drum intervals,
// referee/cheerleader cues, camera handoff, anger face palettes, and chant
// sounds.
package drummerduel

import (
	"image/color"
	"math"
	"sort"

	"github.com/hajimehoshi/ebiten/v2"

	"hsdemo/engine"
	"hsdemo/kart"
	"hsdemo/riq"
)

const (
	hitAuto = iota
	hitDon
	hitDo
	hitKo
)

const (
	camCenter = iota
	camLeft
	camRight
)

const autoCamLength = 0.5

type bopEvt struct {
	beat, length           float64
	cheer, drummer         bool
	cheerAuto, drummerAuto bool
}

type intervalEvt struct {
	beat, length  float64
	auto, camMove bool
	successVoice  bool
	pattern       int
}

type hitEvt struct {
	beat     float64
	hitSound int
}

type passEvt struct {
	beat, length float64
}

type camEvt struct {
	beat, length float64
	pos, ease    int
}

type angerEvt struct {
	beat  float64
	angry bool
}

type chantEvt struct {
	beat float64
	typ  int
}

type npcEvt struct {
	beat                 float64
	cheer, referee, plat bool
}

type endPoseEvt struct {
	beat float64
}

type Module struct {
	ctx  *engine.Ctx
	proj kart.Aff

	referee, taikoL, taikoR    string
	drummerL, drummerR         string
	cheerObj, refObj, platform string
	cheerLeft, cheerRight      []string

	bops      []bopEvt
	intervals []intervalEvt
	hits      []hitEvt
	passes    []passEvt
	cameras   []camEvt
	angers    []angerEvt
	chants    []chantEvt
	npcs      []npcEvt
	ends      []endPoseEvt

	endBeat float64

	goBopCheer, goBopDrummer bool
	allowBopCheer            bool
	allDon, isRight          bool
	isAngry, isWhiffing      bool
	hasMissed, isDrumming    bool
	lastPulse                int

	cameraLeft, cameraCenter, cameraRight float64
	cameraX                               float64
	cameraFrom, cameraTo                  float64
	cameraStart, cameraLen                float64
	cameraEase                            int
	cameraMoving                          bool
	cameraLoc                             int
}

func New() engine.Module {
	return &Module{
		goBopCheer:    true,
		goBopDrummer:  true,
		allowBopCheer: true,
		allDon:        true,
		isRight:       true,
		lastPulse:     math.MinInt,
	}
}

func (m *Module) ID() string { return "drummerDuel" }

func (m *Module) Load(ctx *engine.Ctx) error {
	m.ctx = ctx
	if err := ctx.LoadAssets("drummerDuel"); err != nil {
		return err
	}
	m.proj = kart.Translate(engine.ScreenW/2, engine.ScreenH/2).Mul(kart.Scale(54, -54))
	m.referee = roleOr(ctx, "referee", "Referee")
	m.taikoL = roleOr(ctx, "taikoLeft", "Taikos/TaikoL")
	m.taikoR = roleOr(ctx, "taikoRight", "Taikos/TaikoR")
	m.drummerL = roleOr(ctx, "drummerLeft", "Drummers/DrummerL")
	m.drummerR = roleOr(ctx, "drummerRight", "Drummers/DrummerR")
	m.cheerObj = roleOr(ctx, "cheerLeadersObj", "Cheerleaders")
	m.refObj = roleOr(ctx, "refereeObj", "Referee")
	m.platform = roleOr(ctx, "refereePlatformObj", "Platforms/Center")
	m.cheerLeft = append([]string(nil), ctx.Assets.Extra.RefArrays["cheerLeadersLeft"]...)
	m.cheerRight = append([]string(nil), ctx.Assets.Extra.RefArrays["cheerLeadersRight"]...)
	if len(m.cheerLeft) == 0 {
		m.cheerLeft = []string{"Cheerleaders/Left/CheerleaderL", "Cheerleaders/Left/CheerleaderM", "Cheerleaders/Left/CheerleaderR"}
	}
	if len(m.cheerRight) == 0 {
		m.cheerRight = []string{"Cheerleaders/Right/CheerleaderL", "Cheerleaders/Right/CheerleaderM", "Cheerleaders/Right/CheerleaderR"}
	}
	game := ctx.Assets.Extra.Components["game"]
	m.cameraLeft = game.Nums["cameraLeft"]
	m.cameraCenter = game.Nums["cameraCenter"]
	m.cameraRight = game.Nums["cameraRight"]
	m.applyDefaultPalettes()
	m.applyDrummerHeadColors()
	m.resetCamera(camCenter, m.cameraCenter)
	m.initScene(0)
	return nil
}

func (m *Module) OnEvent(e *riq.Entity) {
	b := e.Beat
	if end := b + e.Length; end > m.endBeat {
		m.endBeat = end
	}
	switch e.Datamodel {
	case "drummerDuel/bop":
		m.bops = append(m.bops, bopEvt{
			beat: b, length: e.Length,
			cheer:       boolParamDefault(e, "bopCheer", true),
			drummer:     boolParamDefault(e, "bopDrummer", true),
			cheerAuto:   boolParam(e, "bopCheerAuto"),
			drummerAuto: boolParam(e, "bopDrummerAuto"),
		})
	case "drummerDuel/beat intervals":
		m.intervals = append(m.intervals, intervalEvt{
			beat: b, length: e.Length,
			auto:         boolParamDefault(e, "auto", true),
			camMove:      boolParamDefault(e, "camMove", true),
			successVoice: boolParamDefault(e, "successvoice", true),
			pattern:      intParam(e, "pattern", 0),
		})
	case "drummerDuel/hitdrum":
		m.hits = append(m.hits, hitEvt{beat: b, hitSound: intParam(e, "hitsound", hitAuto)})
	case "drummerDuel/drummer turnover":
		m.passes = append(m.passes, passEvt{beat: b, length: e.Length})
	case "drummerDuel/move camera":
		m.cameras = append(m.cameras, camEvt{
			beat: b, length: e.Length,
			pos:  intParam(e, "camPos", camCenter),
			ease: intParam(e, "ease", 0),
		})
	case "drummerDuel/angry":
		m.angers = append(m.angers, angerEvt{beat: b, angry: boolParamDefault(e, "anger", true)})
	case "drummerDuel/chant":
		m.chants = append(m.chants, chantEvt{beat: b, typ: intParam(e, "chantType", 0)})
	case "drummerDuel/endPose":
		m.ends = append(m.ends, endPoseEvt{beat: b})
	case "drummerDuel/npctoggle":
		m.npcs = append(m.npcs, npcEvt{
			beat:    b,
			cheer:   boolParamDefault(e, "cheer", true),
			referee: boolParamDefault(e, "ref", true),
			plat:    boolParamDefault(e, "plat", true),
		})
	}
}

func (m *Module) Ready() {
	sort.Slice(m.bops, func(i, j int) bool { return m.bops[i].beat < m.bops[j].beat })
	sort.Slice(m.intervals, func(i, j int) bool { return m.intervals[i].beat < m.intervals[j].beat })
	sort.Slice(m.hits, func(i, j int) bool { return m.hits[i].beat < m.hits[j].beat })
	sort.Slice(m.passes, func(i, j int) bool { return m.passes[i].beat < m.passes[j].beat })
	sort.Slice(m.cameras, func(i, j int) bool { return m.cameras[i].beat < m.cameras[j].beat })
	sort.Slice(m.angers, func(i, j int) bool { return m.angers[i].beat < m.angers[j].beat })
	sort.Slice(m.chants, func(i, j int) bool { return m.chants[i].beat < m.chants[j].beat })
	sort.Slice(m.npcs, func(i, j int) bool { return m.npcs[i].beat < m.npcs[j].beat })
	sort.Slice(m.ends, func(i, j int) bool { return m.ends[i].beat < m.ends[j].beat })

	m.scheduleBops()
	for _, ev := range m.intervals {
		m.scheduleInterval(ev)
	}
	for _, ev := range m.passes {
		ev := ev
		m.ctx.At(ev.beat, func() { m.passTurnStandalone(ev) })
	}
	for _, ev := range m.cameras {
		ev := ev
		m.ctx.At(ev.beat, func() { m.moveCamera(ev.beat, ev.length, ev.pos, ev.ease) })
	}
	for _, ev := range m.angers {
		ev := ev
		m.ctx.At(ev.beat, func() { m.setAnger(ev.beat, ev.angry) })
	}
	for _, ev := range m.chants {
		m.scheduleChant(ev)
	}
	for _, ev := range m.npcs {
		ev := ev
		m.ctx.At(ev.beat, func() { m.setNPCs(ev.cheer, ev.referee, ev.plat) })
	}
	for _, ev := range m.ends {
		ev := ev
		m.scheduleEndPose(ev.beat)
	}
}

func (m *Module) OnSwitch(beat float64) {
	m.initScene(beat)
	m.restoreState(beat)
	m.lastPulse = int(math.Floor(beat))
}

func (m *Module) Whiff(beat float64) {
	if !m.isDrumming {
		return
	}
	m.ctx.Sound("drumRightWhiff")
	if !m.hasMissed {
		m.hasMissed = true
		m.ctx.Sound("miss")
	}
	m.playTaiko(m.taikoR, "Whiff", beat)
	m.playDrummer(m.drummerR, "HitArmF", beat)
	m.playDrummer(m.drummerR, "FaceWhiff", beat)
	if m.isAngry {
		m.isWhiffing = true
		m.applyDrummerHeadColors()
	}
	m.cheerAll(m.cheerRight, "Miss", beat)
}

func (m *Module) Update(_ float64, beat float64) {
	m.updateBeatPulse(beat)
	m.updateCamera(beat)
}

func (m *Module) Draw(screen *ebiten.Image, _ float64, beat float64) {
	screen.Fill(color.NRGBA{0xf6, 0xfb, 0xff, 0xff})
	cam := m.ctx.CameraAt(beat)
	// HS writes GameCamera.AdditionalPosition = (-cameraX, 0, 0); Scene.SetCamera
	// takes the actual camera position, so the sign is preserved here.
	m.ctx.Scene.SetCamera(cam[0]-m.cameraX, cam[1], cam[2])
	m.ctx.Scene.Sample(beat)
	m.ctx.Scene.Draw(screen, m.proj)
}

func (m *Module) initScene(beat float64) {
	sec := m.ctx.SecPerBeat(beat)
	for _, path := range []string{m.referee, m.taikoL, m.taikoR, m.drummerL, m.drummerR} {
		m.ctx.Scene.PlayDefaultState(path, beat, sec)
	}
	for _, p := range append(append([]string{}, m.cheerLeft...), m.cheerRight...) {
		m.ctx.Scene.PlayDefaultState(p, beat, sec)
	}
	m.playReferee("HeadNormal", beat)
	m.playReferee("Idle", beat)
	m.cheerAll(m.cheerLeft, "Idle", beat)
	m.cheerAll(m.cheerRight, "Idle", beat)
	m.setNPCs(true, true, true)
}

func (m *Module) restoreState(beat float64) {
	m.goBopCheer, m.goBopDrummer, m.allowBopCheer = true, true, true
	for _, ev := range m.bops {
		if ev.beat >= beat {
			break
		}
		m.goBopCheer = ev.cheerAuto
		m.goBopDrummer = ev.drummerAuto
	}
	m.setAnger(beat, false)
	for _, ev := range m.angers {
		if ev.beat >= beat {
			break
		}
		m.setAnger(ev.beat, ev.angry)
	}
	for _, ev := range m.npcs {
		if ev.beat >= beat {
			break
		}
		m.setNPCs(ev.cheer, ev.referee, ev.plat)
	}
	m.restoreCamera(beat)
}

func roleOr(ctx *engine.Ctx, key, fallback string) string {
	if p := ctx.Role(key); p != "" {
		return p
	}
	return fallback
}

func boolParam(e *riq.Entity, key string) bool { return e.Float(key, 0) != 0 }

func boolParamDefault(e *riq.Entity, key string, def bool) bool {
	v, ok := e.Data[key]
	if !ok {
		return def
	}
	switch x := v.(type) {
	case bool:
		return x
	case float64:
		return x != 0
	default:
		return e.Float(key, 0) != 0
	}
}

func intParam(e *riq.Entity, key string, def int) int {
	return int(e.Float(key, float64(def)))
}
