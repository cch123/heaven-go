package animalacrobat

import (
	"math"
	"testing"

	"hsdemo/kart"
	"hsdemo/kmdata"
)

func loadAnimalAssets(t *testing.T) *kart.Assets {
	t.Helper()
	as, err := kart.Load("../../assets/animalAcrobat", 44100)
	if err != nil {
		t.Skipf("assets not extracted: %v", err)
	}
	return as
}

func TestAnimalAcrobatBindingsAndTemplates(t *testing.T) {
	as := loadAnimalAssets(t)
	for _, role := range []string{
		"_elephant", "_giraffe", "_monkeysLong", "_monkeysShort", "_gorilla",
		"_scroll", "_playerMonkey", "_spotlightMain", "_partyPoppers",
	} {
		if as.Roles[role] == "" {
			t.Fatalf("missing role %s", role)
		}
	}
	for _, root := range []string{"Elephant", "Giraffe", "WhiteMonkeys", "WhiteMonkey", "Gorilla"} {
		if kart.NewTemplate(as, root) == nil {
			t.Fatalf("missing template %s", root)
		}
	}
}

func TestAnimalAcrobatControllersAndSounds(t *testing.T) {
	as := loadAnimalAssets(t)
	wantStates := map[string][]string{
		"Elephant":          {"ElephantIdle", "ElephantEar"},
		"GiraffeRoot":       {"GiraffeIdle", "GiraffeEar"},
		"Gorilla":           {"GorillaIdle", "GorillaMiss"},
		"WhiteMonkeysPivot": {"WhiteMonkeysIdle", "WhiteMonkeysSwing"},
		"FireHoop":          {"FireIdle", "FireClose"},
		"ConfettiPop":       {"PopIntro"},
		"PlayerMonkey":      {"PlayerIdle", "PlayerBop", "PlayerJump", "PlayerAir", "PlayerHang", "PlayerHanging", "PlayerLand"},
	}
	for ctrl, states := range wantStates {
		c, ok := as.Controllers[ctrl]
		if !ok {
			t.Fatalf("missing controller %s", ctrl)
		}
		for _, st := range states {
			cs, ok := c.States[st]
			if !ok {
				t.Errorf("controller %s missing state %s", ctrl, st)
				continue
			}
			if cs.Clip != "" && as.Anims[cs.Clip] == nil {
				t.Errorf("controller %s state %s references missing clip %s", ctrl, st, cs.Clip)
			}
		}
	}
	for _, snd := range []string{
		"start", "eek", "catch", "giraffeCatch", "release", "giraffeJump",
		"giraffeDrumroll", "giraffeCymbal", "turn", "land", "miss", "cracker",
		"applause", "common_nearMiss",
	} {
		if len(as.Sounds[snd]) == 0 {
			t.Fatalf("missing sound %s", snd)
		}
	}
}

func TestAnimalAcrobatObstacleComponents(t *testing.T) {
	as := loadAnimalAssets(t)
	comps := as.Extra.Components
	for _, root := range []string{"Elephant", "Giraffe", "WhiteMonkeys", "WhiteMonkey", "Gorilla"} {
		ob := obstacleComponent(comps, root)
		if ob.Path == "" {
			t.Fatalf("missing obstacle component for %s", root)
		}
		for _, key := range []string{"_rotateRoot", "_gripPoint", "_endPoint"} {
			if ob.Refs[key] == "" {
				t.Errorf("%s missing ref %s", root, key)
			}
		}
		if ob.Nums["_fullRotRange"] == 0 {
			t.Errorf("%s missing full rotation range", root)
		}
		if root != "Gorilla" {
			in := inputComponent(comps, root, root == "Giraffe")
			if in.Path == "" {
				t.Fatalf("missing input component for %s", root)
			}
			for _, key := range []string{"_monkey", "_holdParticle", "_sweatParticle", "_gripShadow", "_endShadow"} {
				if in.Refs[key] == "" {
					t.Errorf("%s missing input ref %s", root, key)
				}
			}
			spec := (&Module{}).makeInputSpec(root, in)
			if spec.holdParticleRel == "" || spec.sweatParticleRel == "" {
				t.Errorf("%s particle refs were not converted to template-relative paths", root)
			}
			if spec.gripShadowRel == "" || spec.endShadowRel == "" {
				t.Errorf("%s shadow refs were not converted to template-relative paths", root)
			}
		}
	}
}

func TestAnimalAcrobatActionParticleSystems(t *testing.T) {
	as := loadAnimalAssets(t)
	for _, sprite := range []string{acrobatStarSprite, acrobatRingSprite, acrobatSweatSprite} {
		if _, ok := as.Sheet.Sprites[sprite]; !ok {
			t.Fatalf("missing ParticleSystem UV sprite %s", sprite)
		}
	}

	m := &Module{}
	m.spawnStarBurst(12, 3, 1, 2, acrobatParticleHold, 0x1234)
	if len(m.particles) != acrobatSparkleBurstCount+acrobatRingBurstCount {
		t.Fatalf("hold particles = %d, want %d", len(m.particles), acrobatSparkleBurstCount+acrobatRingBurstCount)
	}
	var stars, rings int
	for _, p := range m.particles {
		switch p.sprite {
		case acrobatStarSprite:
			stars++
			if p.order != acrobatSparkleOrder || p.sizeProfile != sizeProfileSparkle {
				t.Fatalf("hold sparkle particle did not mirror Sparkle renderer/profile: %#v", p)
			}
		case acrobatRingSprite:
			rings++
			if p.order != acrobatRingOrder || p.sizeProfile != sizeProfileRing {
				t.Fatalf("ring particle did not mirror Ring renderer/profile: %#v", p)
			}
		default:
			t.Fatalf("unexpected hold particle sprite %s", p.sprite)
		}
	}
	if stars != acrobatSparkleBurstCount || rings != acrobatRingBurstCount {
		t.Fatalf("hold particle split stars=%d rings=%d", stars, rings)
	}

	m.particles = nil
	m.spawnSweatBurst(12, 3, 1, 2)
	if len(m.particles) != acrobatSweatBurstCount*2 {
		t.Fatalf("sweat particles = %d, want %d", len(m.particles), acrobatSweatBurstCount*2)
	}
	var delayed int
	for _, p := range m.particles {
		if p.sprite != acrobatSweatSprite || p.order != acrobatSweatOrder || p.sizeProfile != sizeProfileSweat {
			t.Fatalf("unexpected sweat particle: %#v", p)
		}
		if math.Abs(p.born-(3+acrobatSweatStartDelayBS)) < 1e-9 {
			delayed++
		}
	}
	if delayed != acrobatSweatBurstCount {
		t.Fatalf("delayed sweat particles = %d, want %d", delayed, acrobatSweatBurstCount)
	}
}

func TestAnimalAcrobatConfettiParticleSystem(t *testing.T) {
	as := loadAnimalAssets(t)
	for _, path := range confettiStreamPaths {
		if _, ok := as.NodeIndex(path); !ok {
			t.Fatalf("missing PartyPoppers stream node %s", path)
		}
	}
	anim := as.Anims["Animations/PopIntro"]
	if anim == nil {
		t.Fatal("missing PopIntro animation")
	}
	if math.Abs(anim.Duration-acrobatConfettiPopIntroSec) > 1e-8 {
		t.Fatalf("PopIntro duration = %.8f, want %.8f", anim.Duration, acrobatConfettiPopIntroSec)
	}

	registerConfettiSprite(as)
	sp, ok := as.Sheet.Sprites[acrobatConfettiSprite]
	if !ok {
		t.Fatal("confetti runtime sprite was not registered")
	}
	if sp.PPU != 1 || math.Abs(sp.PivotY-acrobatConfettiMeshPivotY) > 1e-12 {
		t.Fatalf("confetti sprite metadata = %#v", sp)
	}

	base := kart.Translate(2, 3)
	m := &Module{}
	m.spawnConfettiBurst(24, 10, base, 0x1234)
	if len(m.particles) != acrobatConfettiBurstCount {
		t.Fatalf("confetti particles = %d, want %d", len(m.particles), acrobatConfettiBurstCount)
	}
	wantX := acrobatConfettiMeshWidth * acrobatConfettiStartSizeX
	wantY := acrobatConfettiMeshHeight * acrobatConfettiStartSizeY
	for _, p := range m.particles {
		if p.sprite != acrobatConfettiSprite || p.order != acrobatConfettiOrder {
			t.Fatalf("unexpected confetti renderer data: %#v", p)
		}
		if !p.local || p.base != base {
			t.Fatalf("confetti should be simulated in stream-local space: %#v", p)
		}
		if p.sizeProfile != sizeProfileConfetti || p.alpha != alphaProfileConfetti {
			t.Fatalf("confetti profiles not attached: %#v", p)
		}
		if p.life < acrobatConfettiLifeMinSec/acrobatConfettiSimSpeed ||
			p.life > acrobatConfettiLifeMaxSec/acrobatConfettiSimSpeed {
			t.Fatalf("confetti life = %v outside prefab range", p.life)
		}
		if math.Abs(p.startSize-wantX) > 1e-12 || math.Abs(p.startSizeY-wantY) > 1e-12 {
			t.Fatalf("confetti mesh size = %.12f %.12f, want %.12f %.12f", p.startSize, p.startSizeY, wantX, wantY)
		}
		if p.ay != acrobatConfettiGravity {
			t.Fatalf("confetti gravity = %v, want %v", p.ay, acrobatConfettiGravity)
		}
	}

	x0, y0 := confettiSizeFactors(0)
	xMid, yMid := confettiSizeFactors(0.5)
	xEnd, yEnd := confettiSizeFactors(1)
	if x0 != 1 || y0 != 0 || xMid != 1 || yMid != 1 || xEnd != 0 || yEnd != 0 {
		t.Fatalf("confetti size curve mismatch: start=(%v,%v) mid=(%v,%v) end=(%v,%v)", x0, y0, xMid, yMid, xEnd, yEnd)
	}
	if confettiAlpha(0.7) != 1 || confettiAlpha(1) != 0 {
		t.Fatalf("confetti alpha curve mismatch")
	}
}

func TestAnimalAcrobatTrailLifecycle(t *testing.T) {
	m := &Module{}
	m.startTrail(5, 1, 2)
	if !m.trail.active {
		t.Fatal("trail should be active after SpawnTrail")
	}
	if len(m.particles) == 0 {
		t.Fatal("prewarmed trail should seed particles")
	}
	m.stopTrail(5.1)
	if m.trail.active {
		t.Fatal("trail should stop after KillTrail")
	}
	for _, p := range m.particles {
		if p.alpha != alphaProfileTrail {
			continue
		}
		if remaining := p.born + p.life - 5.1; remaining > 0.2000001 {
			t.Fatalf("trail particle remaining life = %v, want <= 0.2", remaining)
		}
	}
}

func TestAnimalAcrobatObstacleComponentDoesNotMatchInputFamily(t *testing.T) {
	comps := map[string]kmdata.Component{
		"obstacleInput0": {Path: "Elephant", Refs: map[string]string{"_monkey": "wrong"}},
		"obstacle0":      {Path: "Elephant", Refs: map[string]string{"_rotateRoot": "right"}},
	}
	if got := obstacleComponent(comps, "Elephant").Refs["_rotateRoot"]; got != "right" {
		t.Fatalf("obstacleComponent returned %q, want obstacle family component", got)
	}
}

func TestAnimalAcrobatBGTileManagerRefs(t *testing.T) {
	as := loadAnimalAssets(t)
	c := as.Extra.Components["bgTileManager0"]
	if c.Path == "" {
		t.Fatal("missing BGTileManager component")
	}
	first := c.Refs["_bgTileFirst"]
	second := c.Refs["_bgTileSecond"]
	if first == "" || second == "" {
		t.Fatalf("missing BGTileManager tile refs: first=%q second=%q", first, second)
	}
	ax, _ := nodePos(as, first)
	bx, _ := nodePos(as, second)
	if bx <= ax {
		t.Fatalf("bg tile distance must be positive: first=%v second=%v", ax, bx)
	}
}

func TestAnimalAcrobatBGTilePositionsMatchUnityRecycle(t *testing.T) {
	rt := bgTileRuntime{
		firstBase:    [2]float64{-2.3766, 2.57},
		secondBase:   [2]float64{55.77, 2.57},
		tileDistance: 55.77 - (-2.3766),
		ok:           true,
	}
	d := rt.tileDistance
	cases := []struct {
		name    string
		camera  float64
		firstX  float64
		secondX float64
	}{
		{name: "before first threshold", camera: d - 0.01, firstX: rt.firstBase[0], secondX: rt.secondBase[0]},
		{name: "first tile recycled", camera: d, firstX: rt.firstBase[0] + 2*d, secondX: rt.secondBase[0]},
		{name: "second tile recycled", camera: 2 * d, firstX: rt.firstBase[0] + 2*d, secondX: rt.secondBase[0] + 2*d},
		{name: "first tile recycled again", camera: 3 * d, firstX: rt.firstBase[0] + 4*d, secondX: rt.secondBase[0] + 2*d},
	}
	for _, tc := range cases {
		first, second := bgTilePositions(tc.camera, rt)
		if math.Abs(first[0]-tc.firstX) > 1e-6 || math.Abs(second[0]-tc.secondX) > 1e-6 {
			t.Fatalf("%s: got first.x=%v second.x=%v, want %v %v", tc.name, first[0], second[0], tc.firstX, tc.secondX)
		}
		if first[1] != rt.firstBase[1] || second[1] != rt.secondBase[1] {
			t.Fatalf("%s: y changed: first.y=%v second.y=%v", tc.name, first[1], second[1])
		}
	}
}

func TestAnimalAcrobatCameraTargetFollowsUnityAnimalPhases(t *testing.T) {
	elephant := &acrobatObstacle{
		kind: kindElephant,
		beat: 10, length: 4,
		spec:  obstacleSpec{holdPadding: 1, fullRotRange: 160, ease: 0},
		gripY: -2,
	}
	gorilla := &acrobatObstacle{
		kind: kindGorilla,
		beat: 14, length: 4,
		spec: obstacleSpec{},
	}
	m := &Module{
		gameNums: map[string]float64{
			"_jumpStartCameraDistance": 3,
			"_jumpDistance":            6.5,
		},
		animals: []*acrobatObstacle{elephant, gorilla},
	}

	if _, ok := m.cameraTargetAt(8.99); ok {
		t.Fatal("camera should wait until first animal pre-travel window")
	}
	assertCameraTarget(t, m, 9, 0, 0, 0)
	assertCameraTarget(t, m, 9.5, 1.5, 0, 0)

	rotDist := cameraRotationDistance(elephant)
	assertCameraTarget(t, m, 11.5, 3+rotDist*0.5, -2, 0)
	assertCameraTarget(t, m, 14, 3+rotDist+6.5, 0, 0)
}

func TestAnimalAcrobatCameraTargetGiraffeZoom(t *testing.T) {
	giraffe := &acrobatObstacle{
		kind: kindGiraffe,
		beat: 10, length: 8,
		spec:  obstacleSpec{holdPadding: 2, fullRotRange: 160, ease: 0},
		gripY: -11.27,
	}
	gorilla := &acrobatObstacle{kind: kindGorilla, beat: 18, length: 4}
	m := &Module{
		gameNums: map[string]float64{
			"_jumpStartCameraDistance": 3,
			"_jumpDistanceGiraffe":     32,
			"_giraffeCameraZoom":       6.6,
		},
		animals: []*acrobatObstacle{giraffe, gorilla},
	}

	_, ok := m.cameraTargetAt(14.4)
	if !ok {
		t.Fatal("expected giraffe release camera target")
	}
	target, _ := m.cameraTargetAt(14.4)
	wantZ := -6.6 * 0.5 * 0.5
	if math.Abs(target.z-wantZ) > 1e-6 {
		t.Fatalf("giraffe zoom z=%v, want %v", target.z, wantZ)
	}

	m.monkeyMissed = true
	target, _ = m.cameraTargetAt(14.4)
	if target.z != 0 {
		t.Fatalf("giraffe miss should suppress zoom, got z=%v", target.z)
	}
}

func assertCameraTarget(t *testing.T, m *Module, beat, wantX, wantY, wantZ float64) {
	t.Helper()
	got, ok := m.cameraTargetAt(beat)
	if !ok {
		t.Fatalf("beat %v: missing camera target", beat)
	}
	if math.Abs(got.x-wantX) > 1e-6 || math.Abs(got.y-wantY) > 1e-6 || math.Abs(got.z-wantZ) > 1e-6 {
		t.Fatalf("beat %v: camera target=(%v,%v,%v), want (%v,%v,%v)", beat, got.x, got.y, got.z, wantX, wantY, wantZ)
	}
}

func TestAnimalAcrobatPlayerRotationMatchesCoroutines(t *testing.T) {
	jump := playerJump{start: 20, dur: 2, rotate: playerRotateJump}
	if got := playerRotationAt(jump, 20, 120); got != 120 {
		t.Fatalf("RotateJump start=%v, want 120", got)
	}
	if got := playerRotationAt(jump, 21, 120); got != 0 {
		t.Fatalf("RotateJump spin phase resets to 0 at beat 21, got %v", got)
	}
	if got := playerRotationAt(jump, 21.25, 120); got != 360 {
		t.Fatalf("RotateJump second spin=%v, want 360", got)
	}
	if got := playerRotationAt(jump, 21.5, 120); got != 0 {
		t.Fatalf("RotateJump end=%v, want 0", got)
	}

	arc := playerJump{start: 20, dur: 4, rotate: playerRotateArc}
	if got := playerRotationAt(arc, 22, 120); got != 240 {
		t.Fatalf("ArcRotate mid=%v, want 240", got)
	}
	if got := playerRotationAt(arc, 24, 120); got != 0 {
		t.Fatalf("ArcRotate end=%v, want 0", got)
	}
}

func TestAnimalAcrobatPlayerShadowMatchesCoroutines(t *testing.T) {
	jump := playerJump{start: 10, dur: 4, shadowMul: 2.2}
	assertShadowScale(t, jump, 10, 3.5, 3.5, 1, 7.7, 7.7)
	assertShadowScale(t, jump, 13.5, 3.5, 3.5, 1, 4.55, 4.55)
	assertShadowScale(t, jump, 14, 3.5, 3.5, 1, 3.5, 3.5)

	land := playerJump{start: 10, dur: 2, shadowMul: 1.4, land: true}
	assertShadowScale(t, land, 12.5, 3.5, 3.5, 1, 32.375, 32.375)
	assertShadowScale(t, land, 13, 3.5, 3.5, 1, 42, 42)
}

func assertShadowScale(t *testing.T, j playerJump, beat, sx, sy, landingBeats, wantX, wantY float64) {
	t.Helper()
	gotX, gotY := playerShadowScaleAt(j, beat, sx, sy, landingBeats)
	if math.Abs(gotX-wantX) > 1e-6 || math.Abs(gotY-wantY) > 1e-6 {
		t.Fatalf("beat %v: shadow scale=(%v,%v), want (%v,%v)", beat, gotX, gotY, wantX, wantY)
	}
}
