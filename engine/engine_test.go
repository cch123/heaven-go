package engine_test

import (
	"os"
	"testing"

	"hsdemo/engine"
	"hsdemo/games/somen"
	"hsdemo/games/trickclass"
)

const (
	packInDir   = "/Users/xargin/Downloads/Heaven Studio.app/Contents/Resources/Data/StreamingAssets/Library Pack-In/Heaven Studio Pack-In Levels"
	packInLevel = packInDir + "/Rhythm Somen.riq"
	trickLevel  = packInDir + "/Trick on the Class.riq"
)

// TestLoadSomenLevel 无窗口验证：riq 加载 → 模块实例化 → 事件分发 → 输入/动作调度。
func TestLoadSomenLevel(t *testing.T) {
	if _, err := os.Stat(packInLevel); err != nil {
		t.Skipf("pack-in level not present: %v", err)
	}
	engine.Register("rhythmSomen", somen.New)

	app, err := engine.New("../assets", packInLevel)
	if err != nil {
		t.Fatalf("engine.New: %v", err)
	}
	stats := app.LoadStats()

	// 谱面构成：far 12 + close 31 + both 11×2 = 65 个判定输入
	if stats.Inputs != 65 {
		t.Errorf("inputs = %d, want 65", stats.Inputs)
	}
	if stats.Actions == 0 {
		t.Error("no scheduled actions")
	}
	if stats.EndBeat != 215 {
		t.Errorf("endBeat = %v, want 215", stats.EndBeat)
	}
	if len(stats.Unported) != 0 {
		t.Errorf("unexpected unported games: %v", stats.Unported)
	}
	if stats.StarBeat != 175 {
		t.Errorf("starBeat = %v, want 175", stats.StarBeat)
	}
}

// TestLoadTrickLevel 验证 trickClass 关卡加载：toss 78 + plane 19 + blast 4 = 101 输入。
func TestLoadTrickLevel(t *testing.T) {
	if _, err := os.Stat(trickLevel); err != nil {
		t.Skipf("pack-in level not present: %v", err)
	}
	engine.Register("trickClass", trickclass.New)

	app, err := engine.New("../assets", trickLevel)
	if err != nil {
		t.Fatalf("engine.New: %v", err)
	}
	stats := app.LoadStats()
	if stats.Inputs != 101 {
		t.Errorf("inputs = %d, want 101", stats.Inputs)
	}
	if len(stats.Unported) != 0 {
		t.Errorf("unexpected unported games: %v", stats.Unported)
	}
}
