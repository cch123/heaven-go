package riq

import (
	"os"
	"testing"
)

const packInLevel = "/Users/xargin/Downloads/Heaven Studio.app/Contents/Resources/Data/StreamingAssets/Library Pack-In/Heaven Studio Pack-In Levels/Rhythm Somen.riq"

// TestLoadOfficialV1 用官方 Pack-In 关卡验证 v1 布局加载（含 UTF-8 BOM 剥除）。
func TestLoadOfficialV1(t *testing.T) {
	if _, err := os.Stat(packInLevel); err != nil {
		t.Skipf("pack-in level not present: %v", err)
	}
	r, err := Load(packInLevel)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	bm := r.Beatmap
	if bm.Version != 1 {
		t.Errorf("version = %d, want 1", bm.Version)
	}
	if len(bm.Entities) == 0 {
		t.Fatal("no entities")
	}
	if got := bm.Tempos[0].BPM; got != 131 {
		t.Errorf("BPM = %v, want 131", got)
	}
	if r.AudioFormat == AudioUnknown {
		t.Errorf("audio format not detected (file %s)", r.AudioName)
	}
	if bm.Prop("remixtitle") == "" {
		t.Error("remixtitle property missing")
	}

	counts := map[string]int{}
	for i := range bm.Entities {
		counts[bm.Entities[i].Datamodel]++
	}
	// 与谱面已知构成对齐（提前用脚本统计过）
	for dm, want := range map[string]int{
		"rhythmSomen/crane (close)": 31,
		"rhythmSomen/crane (far)":   12,
		"rhythmSomen/crane (both)":  11,
		"rhythmSomen/slurp":         21,
		"gameManager/end":           1,
	} {
		if counts[dm] != want {
			t.Errorf("%s = %d, want %d", dm, counts[dm], want)
		}
	}
}

func TestTrimEntitiesAfterFirstEndDropsEditorLeftovers(t *testing.T) {
	bm := &Beatmap{Entities: []Entity{
		{Datamodel: "gameManager/switchGame/balloonHunter", Beat: 0},
		{Datamodel: "balloonHunter/balloonSlow", Beat: 8, Length: 3},
		{Datamodel: "gameManager/end", Beat: 12, Length: 0.5},
		{Datamodel: "balloonHunter/balloonFast", Beat: 16, Length: 2.5},
		{Datamodel: "gameManager/end", Beat: 20, Length: 0.5},
	}}

	bm.trimEntitiesAfterFirstEnd()

	if got, want := len(bm.Entities), 3; got != want {
		t.Fatalf("entities after trim = %d, want %d", got, want)
	}
	if got := bm.Entities[len(bm.Entities)-1].Datamodel; got != "gameManager/end" {
		t.Fatalf("last kept entity = %q, want first end marker", got)
	}
	for _, e := range bm.Entities {
		if e.Beat > 12 {
			t.Fatalf("kept post-end entity %#v", e)
		}
	}
}

func TestLoadV1PracticeWithEmbeddedCustomSFX(t *testing.T) {
	level := "../levels/Balloon Hunter (PRACTICE).riq"
	if _, err := os.Stat(level); err != nil {
		t.Skipf("local Balloon Hunter practice level not present: %v", err)
	}
	r, err := Load(level)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if r.AudioName != "generated-silence.wav" {
		t.Fatalf("audio name = %q, want generated silence fallback", r.AudioName)
	}
	if r.AudioFormat != AudioWAV {
		t.Fatalf("audio format = %v, want wav", r.AudioFormat)
	}
	sfx, ok := r.CustomSfx["balloon spheres prac"]
	if !ok {
		t.Fatalf("embedded custom sfx missing; keys=%v", keysOf(r.CustomSfx))
	}
	if sfx.Format != AudioOGG {
		t.Fatalf("custom sfx format = %v, want ogg", sfx.Format)
	}
	if got := firstEndOrZero(r.Beatmap.Entities); got != 114 {
		t.Fatalf("first end beat = %v, want 114", got)
	}
	if got := r.Beatmap.BeatToTime(114); got < 52 || got > 53 {
		t.Fatalf("end time = %v sec, want around 52.6", got)
	}
}

func TestLoadV1EmbeddedSprites(t *testing.T) {
	level := "../levels/D. Fan Club Dance.riq"
	if _, err := os.Stat(level); err != nil {
		t.Skipf("local Fan Club Dance level not present: %v", err)
	}
	r, err := Load(level)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	sprite, ok := r.CustomSprites["baguette"]
	if !ok {
		t.Fatalf("embedded sprite missing; keys=%v", keysOfImages(r.CustomSprites))
	}
	if sprite.Name != "Resources/Sprites/baguette.png" {
		t.Fatalf("sprite name = %q, want Resources/Sprites/baguette.png", sprite.Name)
	}
	if len(sprite.Data) < 100_000 {
		t.Fatalf("sprite data unexpectedly small: %d bytes", len(sprite.Data))
	}
	if _, ok := r.CustomSprites["LibraryLevelIcon"]; ok {
		t.Fatal("library icon should not be treated as a decal sprite")
	}
}

func firstEndOrZero(es []Entity) float64 {
	end, _ := firstEndBeat(es)
	return end
}

func keysOf(m map[string]EmbeddedAudio) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func keysOfImages(m map[string]EmbeddedImage) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
