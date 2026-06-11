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
