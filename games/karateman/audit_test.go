package karateman

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"hsdemo/games/internal/particlefx"
	"hsdemo/kart"
	"hsdemo/kmdata"
	"hsdemo/riq"
)

type karateManAssets struct {
	Sheet  kmdata.Sheet
	Rig    kmdata.Rig
	Stage  kmdata.Stage
	Anims  map[string]*kmdata.Anim
	Parts  kmdata.ParticleData
	Sounds map[string]bool
}

func loadKarateManAssets(t *testing.T) *karateManAssets {
	t.Helper()
	root := filepath.Join("..", "..", "assets", "karateman")
	as := &karateManAssets{
		Anims:  map[string]*kmdata.Anim{},
		Sounds: map[string]bool{},
	}
	readKarateManJSON(t, filepath.Join(root, "sprites.json"), &as.Sheet)
	readKarateManJSON(t, filepath.Join(root, "rig.json"), &as.Rig)
	readKarateManJSON(t, filepath.Join(root, "stage.json"), &as.Stage)
	readKarateManJSON(t, filepath.Join(root, "anims.json"), &as.Anims)
	readKarateManJSON(t, filepath.Join(root, "particles.json"), &as.Parts)

	soundRoot := filepath.Join(root, "sounds")
	if err := filepath.WalkDir(soundRoot, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		ext := strings.ToLower(filepath.Ext(p))
		if ext != ".ogg" && ext != ".wav" {
			return nil
		}
		rel, err := filepath.Rel(soundRoot, p)
		if err != nil {
			return err
		}
		as.Sounds[strings.TrimSuffix(filepath.ToSlash(rel), ext)] = true
		return nil
	}); err != nil {
		t.Skipf("assets not extracted: %v", err)
	}
	return as
}

func readKarateManJSON(t *testing.T, path string, dst any) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("assets not extracted: %v", err)
	}
	if err := json.Unmarshal(raw, dst); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
}

func TestKarateManLegacyRigAndStage(t *testing.T) {
	as := loadKarateManAssets(t)
	if as.Sheet.Atlas != "atlas.png" {
		t.Fatalf("atlas = %q, want atlas.png", as.Sheet.Atlas)
	}
	if got := as.Sheet.Atlases; len(got) != 9 || got[0] != "atlas.png" || got[1] != "atlas1.png" {
		t.Fatalf("atlases = %#v, want two packed atlases plus seven background atlases", got)
	}
	if len(as.Sheet.Sprites) != 101 {
		t.Fatalf("sprites = %d, want 101", len(as.Sheet.Sprites))
	}
	for _, sprite := range []string{
		"karateman_head_0", "karateman_arm_5", "karateman_pot", "karateman_object_shadow",
		"karateman_word_1", "karateman_word_2", "karateman_word_3", "karateman_word_4",
		"karateman_word_exclaim", "karateman_word_mu_en", "karateman_word_combo_en",
		"bg_gradient", "radial_gradient", "karate_bg_bloody",
		"karate_bg_sunburst_1", "karate_bg_sunburst_2",
		"karate_bg_rings_1", "karate_bg_rings_2",
	} {
		if _, ok := as.Sheet.Sprites[sprite]; !ok {
			t.Fatalf("missing sprite %s", sprite)
		}
	}
	for _, path := range []string{"", "Head", "Body", "LeftArm", "RightArm", "LeftLeg", "RightLeg", "ManShadowM"} {
		if !hasKarateManNode(as.Rig, path) {
			t.Fatalf("missing rig node %q", path)
		}
	}
	assertKarateManNear(t, as.Stage.HitPos[0], 2.862)
	assertKarateManNear(t, as.Stage.HitPos[1], 1.07)
	assertKarateManNear(t, as.Stage.FloorY, -2.1)
	assertKarateManNear(t, as.Stage.HitOffset, 0.65)
	assertKarateManNear(t, as.Stage.Slip, 0.13)
}

func TestKarateManAllAnimationGroupsExtracted(t *testing.T) {
	as := loadKarateManAssets(t)
	if len(as.Anims) != 109 {
		t.Fatalf("animation json keys = %d, want 109", len(as.Anims))
	}
	for _, key := range []string{
		"bg/BarelyFace", "bg/FaceIdle", "bg/HitFace", "bg/NoPose", "bg/Rings", "bg/Serious", "bg/SeriousHit", "bg/Sunburst",
		"item/HitMark", "item/Item00", "item/Item01", "item/Item02", "item/Item03", "item/Item04", "item/Item05", "item/Item06", "item/Item07", "item/Item08", "item/Item09", "item/Item99",
		"karateman/BackHand", "karateman/Beat", "karateman/Head/Face00", "karateman/Head/Face08", "karateman/Jab", "karateman/JabNoNuri", "karateman/LowJab", "karateman/LowKick", "karateman/LowKickMiss", "karateman/ManCharge", "karateman/ManChargeOut", "karateman/ManKick", "karateman/ManReturn", "karateman/NoPose", "karateman/Prepare", "karateman/Straight", "karateman/ToReady", "karateman/UpperCut", "karateman/UpperCutJump",
		"overlay/NoriFull", "overlay/NoriNone",
		"word/NoPose", "word/Word00", "word/Word01", "word/Word02", "word/Word03", "word/Word04", "word/Word05", "word/Word06",
	} {
		if as.Anims[key] == nil {
			t.Fatalf("missing namespaced animation %s", key)
		}
	}
	for _, legacyKey := range []string{"Beat", "Jab", "Straight", "Prepare"} {
		if as.Anims[legacyKey] == nil {
			t.Fatalf("missing legacy bare animation key %s", legacyKey)
		}
	}
	if as.Anims["NoPose"] != nil {
		t.Fatal("duplicate NoPose clips must stay namespaced instead of sharing a bare key")
	}
}

func TestKarateManWeatherParticleSystemsExtracted(t *testing.T) {
	as := loadKarateManAssets(t)
	if len(as.Parts.Systems) != 26 {
		t.Fatalf("particle systems = %d, want 26", len(as.Parts.Systems))
	}
	for _, tc := range []struct {
		path   string
		active bool
		max    int
	}{
		{path: "karateman/Effect/Snow", active: true, max: 1280},
		{path: "karateman/Effect/Snow/SnowFront", active: true, max: 1000},
		{path: "karateman/Effect/Fire", active: false, max: 2048},
		{path: "karateman/Effect/Fire/FireFront", active: true, max: 1000},
		{path: "karateman/Effect/Rain", active: false, max: 1280},
		{path: "karateman/Effect/Rain/RainSplash", active: true, max: 1000},
	} {
		ps := mustFindKarateParticle(t, as.Parts, tc.path)
		if ps.Active != tc.active {
			t.Fatalf("%s active = %v, want %v", tc.path, ps.Active, tc.active)
		}
		if !ps.Enabled || !ps.Emission.Enabled || !ps.Renderer.Enabled {
			t.Fatalf("%s should keep enabled emission and renderer flags: %#v", tc.path, ps)
		}
		if ps.MaxParticles != tc.max {
			t.Fatalf("%s maxParticles = %d, want %d", tc.path, ps.MaxParticles, tc.max)
		}
		if ps.StartLifetime.Scalar <= 0 && len(ps.StartLifetime.Max) == 0 {
			t.Fatalf("%s missing startLifetime curve", tc.path)
		}
		if ps.StartSize.Scalar <= 0 && len(ps.StartSize.Max) == 0 {
			t.Fatalf("%s missing startSize curve", tc.path)
		}
	}
}

func TestKarateManWeatherParticleRuntimeUsesInactiveRoots(t *testing.T) {
	as := loadKarateManAssets(t)
	kartAssets := &kart.Assets{Particles: as.Parts}
	active := particlefx.Roots(kartAssets)
	withInactive := particlefx.RootsIncludingInactive(kartAssets)

	for _, root := range []string{
		"karateman/Effect/Snow",
		"karateman/Effect/Fire",
		"karateman/Effect/Rain",
	} {
		if len(withInactive[root]) != 2 {
			t.Fatalf("%s including inactive has %d systems, want root + child", root, len(withInactive[root]))
		}
	}
	// Fire and Rain root emitters are inactive in the prefab because
	// KarateMan.SetParticleEffect activates the selected Effect before Play().
	// The runtime must include those inactive roots or the main weather layer is lost.
	if len(active["karateman/Effect/Fire"]) >= len(withInactive["karateman/Effect/Fire"]) {
		t.Fatalf("active-only Fire roots unexpectedly include inactive root emitter")
	}
	if got := karateParticleRoot(kartAssets, "Rain"); got != "karateman/Effect/Rain" {
		t.Fatalf("karateParticleRoot(Rain) = %q, want karateman/Effect/Rain", got)
	}
}

func TestKarateManFaceClipsAreHeadScoped(t *testing.T) {
	as := loadKarateManAssets(t)
	for face := 0; face <= 8; face++ {
		key := fmt.Sprintf("karateman/Head/Face%02d", face)
		clip := as.Anims[key]
		if clip == nil {
			t.Fatalf("missing %s", key)
		}
		for path := range clip.Pos {
			assertKarateManHeadPath(t, key, path)
		}
		for path := range clip.Euler {
			assertKarateManHeadPath(t, key, path)
		}
		for path := range clip.Scale {
			assertKarateManHeadPath(t, key, path)
		}
		for path := range clip.Sprites {
			assertKarateManHeadPath(t, key, path)
		}
		for path := range clip.Floats {
			assertKarateManHeadPath(t, key, path)
		}
	}
}

func TestKarateManWordClipsResolveSprites(t *testing.T) {
	as := loadKarateManAssets(t)
	for _, key := range []string{
		"word/Word00", "word/Word01", "word/Word02", "word/Word03",
		"word/Word04", "word/Word05", "word/Word06",
	} {
		clip := as.Anims[key]
		if clip == nil {
			t.Fatalf("missing word clip %s", key)
		}
		for path, keys := range clip.Sprites {
			for _, k := range keys {
				if k.Name == "" {
					continue
				}
				if _, ok := as.Sheet.Sprites[k.Name]; !ok {
					t.Fatalf("%s path %s references missing sprite %s", key, path, k.Name)
				}
			}
		}
	}
}

func TestKarateManWordRuntimeMapping(t *testing.T) {
	cases := map[int]string{
		0: "word/Word00",
		1: "word/Word01",
		2: "word/Word02",
		3: "word/Word02",
		4: "word/Word03",
		5: "word/Word04",
		6: "word/Word05",
		7: "word/Word06",
	}
	for kind, want := range cases {
		if got := wordClip(kind); got != want {
			t.Fatalf("wordClip(%d) = %s, want %s", kind, got, want)
		}
	}

	rig := karateWordRig()
	for _, path := range []string{"", "Main", "Sub", "Exclaim"} {
		if !hasKarateManNode(rig, path) {
			t.Fatalf("word rig missing node %q", path)
		}
	}
	if got, ok := legacyHitXWarningKind(&riq.Entity{Data: map[string]any{"type": 2.0}}); !ok || got != 3 {
		t.Fatalf("legacy hitX type 2 = %d,%v; want 3,true", got, ok)
	}
	if got, ok := legacyHitXWarningKind(&riq.Entity{Data: map[string]any{"type": 7.0}}); !ok || got != 0 {
		t.Fatalf("legacy hitX type 7 = %d,%v; want 0,true", got, ok)
	}
	if _, ok := legacyHitXWarningKind(&riq.Entity{Data: map[string]any{}}); ok {
		t.Fatal("legacy hitX without type should be ignored")
	}
}

func TestKarateManBackgroundClipsResolveOfficialSprites(t *testing.T) {
	as := loadKarateManAssets(t)
	for _, tc := range []struct {
		typ     int
		clip    string
		sprites []string
	}{
		{typ: 1, clip: "bg/Sunburst", sprites: []string{"karate_bg_sunburst_1", "karate_bg_sunburst_2"}},
		{typ: 2, clip: "bg/Rings", sprites: []string{"karate_bg_rings_1", "karate_bg_rings_2"}},
		{typ: 3, clip: "bg/Rings", sprites: []string{"karate_bg_rings_1", "karate_bg_rings_2"}},
	} {
		if got := karateBGFXClip(tc.typ); got != tc.clip {
			t.Fatalf("karateBGFXClip(%d) = %s, want %s", tc.typ, got, tc.clip)
		}
		clip := as.Anims[tc.clip]
		if clip == nil {
			t.Fatalf("missing %s", tc.clip)
		}
		for _, sprite := range tc.sprites {
			if _, ok := as.Sheet.Sprites[sprite]; !ok {
				t.Fatalf("%s references missing sprite %s", tc.clip, sprite)
			}
		}
	}
	for typ, want := range map[int]string{
		1: "bg_gradient",
		2: "radial_gradient",
		3: "karate_bg_bloody",
		4: "bg_gradient",
	} {
		if got := karateBGTextureSprite(typ); got != want {
			t.Fatalf("karateBGTextureSprite(%d) = %s, want %s", typ, got, want)
		}
		if _, ok := as.Sheet.Sprites[want]; !ok {
			t.Fatalf("texture type %d uses missing sprite %s", typ, want)
		}
	}
	if karateBGTextureCovers(1) || !karateBGTextureCovers(2) || !karateBGTextureCovers(3) || karateBGTextureCovers(4) {
		t.Fatal("background texture aspect rules drifted from Unity sliced vs regular sprites")
	}
}

func TestKarateManAllSoundsExtracted(t *testing.T) {
	as := loadKarateManAssets(t)
	if len(as.Sounds) != 57 {
		t.Fatalf("sounds = %d, want 57", len(as.Sounds))
	}
	for _, key := range []string{
		"objectOut", "potHit", "punchKickHit1", "swingNoHit", "karate_through",
		"alienHit", "barrelBreak", "barrelOutCombos", "barrelOutKicks",
		"bombBreak", "bombHit", "bombKick", "comboHit1", "comboHit2", "comboHit3", "comboHit4", "comboMiss",
		"lightbulbOut", "lightbulbHit", "lightbulbNtrOut", "offbeatLightbulbOut", "offbeatObjectOut",
		"nori_just", "nori_ng", "nori_through", "rockHit", "rockHit_fullNori", "soccerHit", "swingKick", "swingNoHit_alt",
		"en/one", "en/two", "en/three", "en/threeAlt", "en/four", "en/hit", "en/hitAlt", "en/ko", "en/pow", "en/punchy4",
	} {
		if !as.Sounds[key] {
			t.Fatalf("missing sound %s", key)
		}
	}
}

func mustFindKarateParticle(t *testing.T, data kmdata.ParticleData, path string) kmdata.ParticleSystem {
	t.Helper()
	for _, ps := range data.Systems {
		if ps.Path == path {
			return ps
		}
	}
	t.Fatalf("missing particle system %s", path)
	return kmdata.ParticleSystem{}
}

func hasKarateManNode(r kmdata.Rig, path string) bool {
	for _, n := range r.Nodes {
		if n.Path == path {
			return true
		}
	}
	return false
}

func assertKarateManHeadPath(t *testing.T, clip, path string) {
	t.Helper()
	if path != "Head" && !strings.HasPrefix(path, "Head/") {
		t.Fatalf("%s drives non-Head path %q", clip, path)
	}
}

func assertKarateManNear(t *testing.T, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 0.0001 {
		t.Fatalf("got %v, want %v", got, want)
	}
}
