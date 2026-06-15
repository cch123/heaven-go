// Package freezeframe ports Freeze Frame's car photo cues, T.J. Snapper
// overlay, photo result screen, intro sign/lights, crowd, walkers, and
// camera-man transform events.
package freezeframe

import (
	"math"
	"sort"

	"github.com/hajimehoshi/ebiten/v2"

	"hsdemo/engine"
	"hsdemo/kart"
	"hsdemo/riq"
)

type Module struct {
	ctx  *engine.Ctx
	proj kart.Aff

	cameraMan, overlay, crosshair string
	shutter, dimRect              string
	results, introSign            string
	crowd                         string
	crowdFarLeft, crowdLeft       string
	crowdRight, crowdFarRight     string
	billboards                    string
	farSpawn, nearSpawn           string
	walkerSpawn                   string

	photoRoots []string
	farT       *kart.Template
	nearT      *kart.Template
	walkerT    *kart.Template

	cameraBasePos   [2]float64
	cameraBaseScale [2]float64
	farSpawnPos     [2]float64
	nearSpawnPos    [2]float64
	walkerSpawnPos  [2]float64

	bops        []bopEvt
	cues        []carCueEvt
	shows       []showPhotosEvt
	walkers     []walkerEvt
	crowds      []crowdEvt
	introSigns  []introSignEvt
	introLights []introLightsEvt
	overlays    []overlayEvt
	moves       []cameraMoveEvt

	spawns []carSpawnEvt

	showOverlay, showCameraMan bool
	followCamera               bool
	showCrowd, showBillboard   bool
	autoBop, autoBlink         bool
	crosshairOn                bool
	showingPhotos              bool
	lastPulse                  int

	activeSign introSignEvt
	signMoving bool
	moveRun    moveRuntime
	rotRun     rotateRuntime
	scaleRun   scaleRuntime

	photos       []photoArgs
	activeCars   []*carInstance
	activeWalker []*walkerInstance
}

func New() engine.Module {
	return &Module{
		showOverlay: true, showCameraMan: true, followCamera: true,
		autoBop: true, autoBlink: true, crosshairOn: true, lastPulse: -1,
		cameraBaseScale: [2]float64{1, 1},
	}
}

func (m *Module) ID() string { return gameID }

func (m *Module) Load(ctx *engine.Ctx) error {
	m.ctx = ctx
	if err := ctx.LoadAssets(gameID); err != nil {
		return err
	}
	m.proj = kart.Translate(engine.ScreenW/2, engine.ScreenH/2).Mul(kart.Scale(54, -54))

	game := ctx.Assets.Extra.Components["game"]
	m.cameraMan = roleOr(ctx, "CameraMan", game.Refs["CameraMan"])
	m.overlay = roleOr(ctx, "Overlay", game.Refs["Overlay"])
	m.crosshair = roleOr(ctx, "Crosshair", game.Refs["Crosshair"])
	m.shutter = roleOr(ctx, "Shutter", game.Refs["Shutter"])
	m.dimRect = roleOr(ctx, "DimRect", game.Refs["DimRect"])
	m.results = roleOr(ctx, "Results", game.Refs["Results"])
	m.introSign = roleOr(ctx, "IntroSign", game.Refs["IntroSign"])
	m.crowd = roleOr(ctx, "Crowd", game.Refs["Crowd"])
	m.crowdFarLeft = roleOr(ctx, "CrowdFarLeft", game.Refs["CrowdFarLeft"])
	m.crowdLeft = roleOr(ctx, "CrowdLeft", game.Refs["CrowdLeft"])
	m.crowdRight = roleOr(ctx, "CrowdRight", game.Refs["CrowdRight"])
	m.crowdFarRight = roleOr(ctx, "CrowdFarRight", game.Refs["CrowdFarRight"])
	m.billboards = roleOr(ctx, "Billboards", game.Refs["Billboards"])
	m.farSpawn = roleOr(ctx, "FarCarSpawn", game.Refs["FarCarSpawn"])
	m.nearSpawn = roleOr(ctx, "NearCarSpawn", game.Refs["NearCarSpawn"])
	m.walkerSpawn = roleOr(ctx, "WalkerSpawn", game.Refs["WalkerSpawn"])

	m.photoRoots = append([]string(nil), ctx.Assets.Extra.RefArrays["Photographs"]...)
	if len(m.photoRoots) == 0 {
		m.photoRoots = append([]string(nil), game.RefArrays["Photographs"]...)
	}
	if len(m.photoRoots) == 0 {
		for i := 1; i <= 6; i++ {
			m.photoRoots = append(m.photoRoots, "HUD/Canvas/Photos/Photograph"+itoa(i))
		}
	}
	m.farT = kart.NewTemplate(ctx.Assets, game.Refs["FarCarPrefab"])
	m.nearT = kart.NewTemplate(ctx.Assets, game.Refs["NearCarPrefab"])
	m.walkerT = kart.NewTemplate(ctx.Assets, game.Refs["WalkerPrefab"])

	if p, ok := nodePos(ctx.Assets, m.cameraMan); ok {
		m.cameraBasePos = p
	}
	if s, ok := nodeScale(ctx.Assets, m.cameraMan); ok {
		m.cameraBaseScale = s
	}
	if p, ok := nodePos(ctx.Assets, m.farSpawn); ok {
		m.farSpawnPos = p
	}
	if p, ok := nodePos(ctx.Assets, m.nearSpawn); ok {
		m.nearSpawnPos = p
	}
	if p, ok := nodePos(ctx.Assets, m.walkerSpawn); ok {
		m.walkerSpawnPos = p
	}

	m.initScene(0)
	return nil
}

func (m *Module) OnEvent(e *riq.Entity) {
	switch actionName(e) {
	case "bop":
		m.bops = append(m.bops, bopEvt{
			beat: e.Beat, length: e.Length,
			bop: boolParam(e, "bop"), autoBop: boolDefault(e, "autoBop", true),
			blink: boolParam(e, "blink"), autoBlink: boolDefault(e, "autoBlink", true),
		})
	case "slowCar", "fastCar":
		kind := carSlow
		if actionName(e) == "fastCar" {
			kind = carFast
		}
		m.cues = append(m.cues, carCueEvt{
			beat: e.Beat, kind: kind, photo: photoType(intDefault(e, "variant", int(photoRandom))),
			mute: boolParam(e, "mute"), clear: boolParam(e, "clear"),
			autoShow: boolDefault(e, "autoShowPhotos", true),
			grade:    gradeType(intDefault(e, "gradeType", int(gradeThumbs))),
			audience: boolDefault(e, "audience", true),
		})
	case "showPhotos":
		m.shows = append(m.shows, showPhotosEvt{
			beat: e.Beat, length: e.Length,
			grade:    gradeType(intDefault(e, "gradeType", int(gradeThumbs))),
			audience: boolDefault(e, "audience", true), clearCache: boolDefault(e, "clearCache", true),
		})
	case "clearPhotos":
		m.ctx.At(e.Beat, func() { m.clearPhotos() })
	case "spawnPerson":
		m.walkers = append(m.walkers, walkerEvt{
			beat: e.Beat, length: e.Length,
			kind:  personType(intDefault(e, "personType", int(personDude1))),
			dir:   personDirection(intDefault(e, "direction", int(directionRandom))),
			layer: intDefault(e, "layer", 0),
		})
	case "spawnCrowd":
		m.crowds = append(m.crowds, crowdEvt{
			beat: e.Beat, show: boolDefault(e, "crowd", true), custom: boolParam(e, "customCrowd"),
			farLeft: intDefault(e, "crowdFarLeft", 2), left: intDefault(e, "crowdLeft", 1),
			right: intDefault(e, "crowdRight", 0), farRight: intDefault(e, "crowdFarRight", 2),
			billboard: boolParam(e, "billboard"),
		})
	case "introSign":
		m.introSigns = append(m.introSigns, introSignEvt{beat: e.Beat, length: e.Length, enter: boolDefault(e, "enter", true), ease: intDefault(e, "ease", 0)})
	case "introLights":
		m.introLights = append(m.introLights, introLightsEvt{beat: e.Beat, length: e.Length, on: boolDefault(e, "lightsOn", true)})
	case "toggleOverlay":
		m.overlays = append(m.overlays, overlayEvt{
			beat: e.Beat, showOverlay: boolDefault(e, "showOverlay", true),
			showTJ: boolDefault(e, "showCameraMan", true), followCamera: boolDefault(e, "followCamera", true),
		})
	case "neoMoveCameraMan":
		m.moves = append(m.moves, cameraMoveEvt{
			beat: e.Beat, length: e.Length, move: boolParam(e, "doMove"),
			startX: floatDefault(e, "startMoveX", 0), startY: floatDefault(e, "startMoveY", 0),
			endX: floatDefault(e, "endMoveX", 0), endY: floatDefault(e, "endMoveY", 0),
			rotate: boolParam(e, "doRotate"), startRot: floatDefault(e, "startRotDegrees", 0), endRot: floatDefault(e, "endRotDegrees", 0),
			scale: boolParam(e, "doScale"), startSX: floatDefault(e, "startScaleX", 1), startSY: floatDefault(e, "startScaleY", 1),
			endSX: floatDefault(e, "endScaleX", 1), endSY: floatDefault(e, "endScaleY", 1),
			ease: intDefault(e, "ease", 0),
		})
	case "moveCameraMan":
		m.moves = append(m.moves, cameraMoveEvt{
			beat: e.Beat, length: e.Length, move: true,
			startX: floatDefault(e, "startPosX", 0), startY: floatDefault(e, "startPosY", 0),
			endX: floatDefault(e, "endPosX", 0), endY: floatDefault(e, "endPosY", 0),
			ease: intDefault(e, "ease", 0),
		})
	case "rotateCameraMan":
		m.moves = append(m.moves, cameraMoveEvt{
			beat: e.Beat, length: e.Length, rotate: true,
			startRot: floatDefault(e, "startRot", 0), endRot: floatDefault(e, "endRot", 0),
			ease: intDefault(e, "ease", 0),
		})
	case "scaleCameraMan":
		m.moves = append(m.moves, cameraMoveEvt{
			beat: e.Beat, length: e.Length, scale: true,
			startSX: floatDefault(e, "startSizeX", 1), startSY: floatDefault(e, "startSizeY", 1),
			endSX: floatDefault(e, "endSizeX", 1), endSY: floatDefault(e, "endSizeY", 1),
			ease: intDefault(e, "ease", 0),
		})
	}
}

func (m *Module) Ready() {
	m.sortEvents()
	m.buildCarSpawns()
	for _, ev := range m.bops {
		ev := ev
		m.ctx.At(ev.beat, func() {
			m.autoBop, m.autoBlink = ev.autoBop, ev.autoBlink
		})
		for i := 0; i < int(ev.length); i++ {
			b := ev.beat + float64(i)
			if ev.bop {
				m.ctx.At(b, func() { m.bop(b) })
			}
			if ev.blink {
				m.ctx.At(b, func() { m.crosshairBlink() })
			}
		}
	}
	m.scheduleCarCues()
	m.scheduleAutoShows()
	for _, ev := range m.shows {
		ev := ev
		m.ctx.At(ev.beat, func() { m.showPhotos(ev.beat, ev.length, ev.grade, ev.audience, ev.clearCache) })
	}
	for _, ev := range m.walkers {
		ev := ev
		m.ctx.At(ev.beat, func() { m.spawnWalker(ev, m.ctx.Beat()) })
	}
	for _, ev := range m.crowds {
		ev := ev
		m.ctx.At(ev.beat, func() { m.applyCrowd(ev) })
	}
	for _, ev := range m.introSigns {
		ev := ev
		m.ctx.At(ev.beat, func() { m.startIntroSign(ev) })
	}
	for _, ev := range m.introLights {
		ev := ev
		m.scheduleIntroLights(ev)
	}
	for _, ev := range m.overlays {
		ev := ev
		m.ctx.At(ev.beat, func() { m.applyOverlay(ev) })
	}
	for _, ev := range m.moves {
		ev := ev
		m.ctx.At(ev.beat, func() { m.applyCameraMove(ev) })
	}
	for _, ev := range m.spawns {
		ev := ev
		m.ctx.At(ev.beat, func() { m.spawnCar(ev) })
	}
}

func (m *Module) OnSwitch(beat float64) {
	m.lastPulse = int(math.Floor(beat)) - 1
	m.activeCars = nil
	m.activeWalker = nil
	m.initScene(beat)
	m.applyPersistent(beat)
	m.spawnPersistedCars(beat)
	m.spawnPersistedWalkers(beat)
}

func (m *Module) Whiff(beat float64) {
	if m.showingPhotos {
		return
	}
	m.cameraFlash(beat)
}

func (m *Module) Update(_, beat float64) {
	for pulse := m.lastPulse + 1; pulse <= int(math.Floor(beat)); pulse++ {
		if pulse >= 0 {
			if m.autoBop && !m.showingPhotos {
				m.bop(float64(pulse))
			}
			if m.autoBlink {
				m.crosshairBlink()
			}
		}
		m.lastPulse = pulse
	}
	m.updateIntroSign(beat)
	m.updateCameraMan(beat)
	m.updateVisibility()
	m.compactRuntime(beat)
}

func (m *Module) Draw(screen *ebiten.Image, _, beat float64) {
	screen.Fill(rgba([4]float64{0x8b / 255.0, 0x93 / 255.0, 0xb4 / 255.0, 1}))
	m.ctx.SampleScene(beat)
	for _, car := range m.activeCars {
		car.queue(m, beat)
	}
	for _, w := range m.activeWalker {
		w.queue(m, beat)
	}
	m.ctx.Scene.Draw(screen, m.proj)
}

func (m *Module) sortEvents() {
	sort.SliceStable(m.bops, func(i, j int) bool { return m.bops[i].beat < m.bops[j].beat })
	sort.SliceStable(m.cues, func(i, j int) bool { return m.cues[i].beat < m.cues[j].beat })
	sort.SliceStable(m.shows, func(i, j int) bool { return m.shows[i].beat < m.shows[j].beat })
	sort.SliceStable(m.walkers, func(i, j int) bool { return m.walkers[i].beat < m.walkers[j].beat })
	sort.SliceStable(m.crowds, func(i, j int) bool { return m.crowds[i].beat < m.crowds[j].beat })
	sort.SliceStable(m.introSigns, func(i, j int) bool { return m.introSigns[i].beat < m.introSigns[j].beat })
	sort.SliceStable(m.introLights, func(i, j int) bool { return m.introLights[i].beat < m.introLights[j].beat })
	sort.SliceStable(m.overlays, func(i, j int) bool { return m.overlays[i].beat < m.overlays[j].beat })
	sort.SliceStable(m.moves, func(i, j int) bool { return m.moves[i].beat < m.moves[j].beat })
}
