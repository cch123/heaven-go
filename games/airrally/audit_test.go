package airrally

import (
	"math"
	"testing"

	"hsdemo/engine"
	"hsdemo/kart"
	"hsdemo/riq"
)

func loadAirRallyAssets(t *testing.T) *kart.Assets {
	t.Helper()
	as, err := kart.Load("../../assets/airRally", 44100)
	if err != nil {
		t.Skipf("assets not extracted: %v", err)
	}
	return as
}

func TestIslandMotionUsesPrefabLoopParameters(t *testing.T) {
	as := loadAirRallyAssets(t)
	m := &Module{ctx: &engine.Ctx{Assets: as, Scene: kart.NewScene(as)}}
	m.initIslands()

	if got := len(m.islands); got != 4 {
		t.Fatalf("islands = %d, want 4", got)
	}
	if math.Abs(m.islandEndZ-(-13.475)) > 1e-9 {
		t.Fatalf("islandEndZ = %.6f, want -13.475", m.islandEndZ)
	}
	if math.Abs(m.islands[0].offset-(1/islandLoopMult)) > 1e-9 {
		t.Fatalf("first island offset = %.6f, want %.6f", m.islands[0].offset, 1/islandLoopMult)
	}
	for _, it := range m.islands {
		if len(it.spritePaths) == 0 {
			t.Fatalf("%s has no sprite paths for fade reset", it.path)
		}
	}
}

func TestIslandMotionAdvancesAndWraps(t *testing.T) {
	as := loadAirRallyAssets(t)
	m := &Module{ctx: &engine.Ctx{Assets: as, Scene: kart.NewScene(as)}}
	m.initIslands()

	start := m.islandZ(m.islands[0])
	m.updateIslands(0, 0)
	m.updateIslands(2, 0)
	moved := m.islandZ(m.islands[0])
	want := start + m.islandEndZ*islandSpeedMult*2
	if math.Abs(moved-want) > 1e-9 {
		t.Fatalf("island z after 2s = %.6f, want %.6f", moved, want)
	}

	m.islands[0].norm = (m.islands[0].startZ - m.islandEndZ) / -m.islandEndZ
	m.updateIslands(2.1, 0)
	wrapped := m.islandZ(m.islands[0])
	if math.Abs(wrapped-48) > 1e-9 {
		t.Fatalf("wrapped island z = %.6f, want 48", wrapped)
	}
	if m.islands[0].fadeLeft <= 0 {
		t.Fatal("wrapped island did not start fade-in")
	}
}

func TestIslandSpeedEventInterpolates(t *testing.T) {
	m := &Module{islandSpeeds: []speedEvt{{beat: 4, length: 4, from: 1, to: 3, ease: 0}}}
	if got := m.islandSpeedAt(2); got != 1 {
		t.Fatalf("speed before event = %.3f, want 1", got)
	}
	if got := m.islandSpeedAt(6); got != 2 {
		t.Fatalf("speed mid event = %.3f, want 2", got)
	}
	if got := m.islandSpeedAt(9); got != 3 {
		t.Fatalf("speed after event = %.3f, want 3", got)
	}
}

func TestDistanceUsesPrefabWaypointBeatLength(t *testing.T) {
	m := &Module{distances: []distanceEvt{{beat: 4, typ: distFar, ease: 0}}}

	if got := m.forthZAt(3.5); got != wayPointHomeZ {
		t.Fatalf("z before distance = %.3f, want %.3f", got, wayPointHomeZ)
	}
	if got := m.forthZAt(4.5); math.Abs(got-19.16) > 1e-9 {
		t.Fatalf("z mid distance = %.3f, want 19.160", got)
	}
	if got := m.forthZAt(5.25); got != distZ[distFar] {
		t.Fatalf("z after distance = %.3f, want %.3f", got, distZ[distFar])
	}
}

func TestEnterUsesPrefabDepthAndMovesBothPlanes(t *testing.T) {
	m := &Module{enters: []enterEvt{{beat: 4, length: 2, ease: 0}}}

	if got := m.forthZAt(3); got != wayPointEnter {
		t.Fatalf("forth z before enter = %.3f, want %.3f", got, wayPointEnter)
	}
	if got := m.baxterZAt(3); got != wayPointEnter {
		t.Fatalf("baxter z before enter = %.3f, want %.3f", got, wayPointEnter)
	}
	wantMid := (wayPointEnter + wayPointHomeZ) / 2
	if got := m.forthZAt(5); math.Abs(got-wantMid) > 1e-9 {
		t.Fatalf("forth z mid enter = %.3f, want %.3f", got, wantMid)
	}
	if got := m.baxterZAt(6.5); got != wayPointHomeZ {
		t.Fatalf("baxter z after enter = %.3f, want %.3f", got, wayPointHomeZ)
	}
}

func TestDayNightColorsUsePrefabPalettes(t *testing.T) {
	m := &Module{dayEvents: []dayEvt{{beat: 4, length: 4, start: dayModeTwilight, end: dayModeNight, ease: 0}}}

	bg, cloud, obj, light := m.dayColorsAt(6)
	if math.Abs(obj[0]-0.5) > 1e-9 || math.Abs(obj[3]-1) > 1e-9 {
		t.Fatalf("object tint mid twilight-night = %v, want half-white alpha 1", obj)
	}
	if math.Abs(light-0.5) > 1e-9 {
		t.Fatalf("light alpha mid twilight-night = %.3f, want 0.5", light)
	}
	wantBgR := (noonColor[0] + nightColor[0]) / 2
	if math.Abs(bg[0]-wantBgR) > 1e-9 {
		t.Fatalf("bg red mid twilight-night = %.6f, want %.6f", bg[0], wantBgR)
	}
	wantCloudG := (noonColorCloud[1] + nightColorCloud[1]) / 2
	if math.Abs(cloud[1]-wantCloudG) > 1e-9 {
		t.Fatalf("cloud green mid twilight-night = %.6f, want %.6f", cloud[1], wantCloudG)
	}
}

func TestWeatherDefaultsUsePrefabManagers(t *testing.T) {
	m := &Module{}
	m.initWeather()

	if got := m.weather.cloudMain.rate; got != 30 {
		t.Fatalf("main cloud cps = %d, want 30", got)
	}
	if got := m.weather.cloudMain.max; got != 300 {
		t.Fatalf("main cloud max = %d, want 300", got)
	}
	if got := len(m.weather.cloudMain.objs); got != 30 {
		t.Fatalf("prebaked main clouds = %d, want 30", got)
	}
	if got := m.weather.snow.max; got != 200 {
		t.Fatalf("snow max = %d, want 200", got)
	}
	if got := len(m.weather.snow.objs); got != 0 {
		t.Fatalf("prebaked snowflakes = %d, want 0", got)
	}
}

func TestWeatherEventsDriveRatesAndSpeeds(t *testing.T) {
	m := &Module{
		cloudEvents: []cloudEvt{{beat: 4, length: 4, main: 2, side: 3, top: 4, speed: 1, endSpeed: 3, ease: 0}},
		snowEvents:  []snowEvt{{beat: 4, length: 4, cps: 5, speed: 2, endSpeed: 6, ease: 0}},
		treeEvents:  []treeEvt{{beat: 4, length: 4, enable: true, main: 7, side: 8, speed: 1, endSpeed: 5, ease: 0}},
	}

	cloud := m.cloudStateAt(6)
	if cloud.main != 2 || cloud.side != 3 || cloud.top != 4 || math.Abs(cloud.speed-2) > 1e-9 {
		t.Fatalf("cloud state at beat 6 = %+v, want rates 2/3/4 speed 2", cloud)
	}
	snow := m.snowStateAt(6)
	if snow.cps != 5 || math.Abs(snow.speed-4) > 1e-9 {
		t.Fatalf("snow state at beat 6 = %+v, want cps 5 speed 4", snow)
	}
	tree := m.treeStateAt(6)
	if !tree.enable || tree.main != 7 || tree.side != 8 || math.Abs(tree.speed-3) > 1e-9 {
		t.Fatalf("tree state at beat 6 = %+v, want enabled rates 7/8 speed 3", tree)
	}
}

func TestRallyRecursionUsesFixedGameBoundary(t *testing.T) {
	entities := []riq.Entity{
		{Datamodel: "gameManager/switchGame/airRally", Beat: 100},
		{Datamodel: "airRally/rally", Beat: 104},
		{Datamodel: "gameManager/switchGame/fireworks", Beat: 120},
		{Datamodel: "gameManager/switchGame/spaceSoccer", Beat: 180},
		{Datamodel: "gameManager/end", Beat: 220},
	}
	stop := rallyStopBeatFromEntities(entities, 104)
	if stop != 120 {
		t.Fatalf("rally stop = %.1f, want next switch at 120", stop)
	}
	if !rallyBeatBeforeStop(118, stop) {
		t.Fatalf("rally at 118 should still be inside Air Rally segment")
	}
	if rallyBeatBeforeStop(120, stop) || rallyBeatBeforeStop(122, stop) {
		t.Fatalf("rally recursion should stop at fixed segment boundary")
	}
}

func TestBirdIdleAnimationCurvesAreSampled(t *testing.T) {
	as := loadAirRallyAssets(t)

	ptero := as.Anims["Pterosaur/Idle"]
	if ptero == nil {
		t.Fatal("missing Pterosaur/Idle animation")
	}
	if got := sampleAnimSprite(ptero, "leftWing", 0, ""); got != "ptleftwing0" {
		t.Fatalf("pterosaur first left wing sprite = %q, want ptleftwing0", got)
	}
	if pos := sampleAnimPos(ptero, "leftWing", 0); math.Abs(pos[0]-0.82) > 1e-9 || math.Abs(pos[1]-1.73) > 1e-9 {
		t.Fatalf("pterosaur left wing pos at t=0 = %v, want [0.82 1.73]", pos)
	}

	blue := as.Anims["Bluebird/Idle"]
	if blue == nil {
		t.Fatal("missing Bluebird/Idle animation")
	}
	if got := sampleAnimSprite(blue, "Wing", 0, ""); got != "bbwing0" {
		t.Fatalf("bluebird first wing sprite = %q, want bbwing0", got)
	}
}
