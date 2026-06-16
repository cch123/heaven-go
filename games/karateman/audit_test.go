package karateman

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"hsdemo/kmdata"
	"hsdemo/riq"
)

type karateManAssets struct {
	Sheet  kmdata.Sheet
	Rig    kmdata.Rig
	Stage  kmdata.Stage
	Anims  map[string]*kmdata.Anim
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
	if got, want := as.Sheet.Atlases, []string{"atlas.png", "atlas1.png"}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("atlases = %#v, want %#v", got, want)
	}
	if len(as.Sheet.Sprites) != 94 {
		t.Fatalf("sprites = %d, want 94", len(as.Sheet.Sprites))
	}
	for _, sprite := range []string{
		"karateman_head_0", "karateman_arm_5", "karateman_pot", "karateman_object_shadow",
		"karateman_word_1", "karateman_word_2", "karateman_word_3", "karateman_word_4",
		"karateman_word_exclaim", "karateman_word_mu_en", "karateman_word_combo_en",
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

func hasKarateManNode(r kmdata.Rig, path string) bool {
	for _, n := range r.Nodes {
		if n.Path == path {
			return true
		}
	}
	return false
}

func assertKarateManNear(t *testing.T, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 0.0001 {
		t.Fatalf("got %v, want %v", got, want)
	}
}
