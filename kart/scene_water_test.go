package kart

import (
	"path/filepath"
	"testing"
)

// TestWaterFlowAnimates 复现"水流不动"：在 Shoot 上播放 WaterFlow 后，
// Shoot/Flow 的 sprite 应随节拍在 water1/water2 间切换。
func TestWaterFlowAnimates(t *testing.T) {
	as, err := Load(filepath.Join("..", "assets", "rhythmSomen"), 44100)
	if err != nil {
		t.Skipf("assets not extracted: %v", err)
	}
	s := NewScene(as)
	s.Play("Shoot", "WaterFlow", 0, 0.5)

	idx, ok := s.byPath["Shoot/Flow"]
	if !ok {
		t.Fatal("node Shoot/Flow not found")
	}
	seen := map[string]bool{}
	for beat := 0.0; beat < 2.0; beat += 0.05 {
		s.Sample(beat)
		seen[s.state[idx].sprite] = true
	}
	t.Logf("seen sprites: %v", seen)
	if !seen["water1"] || !seen["water2"] {
		t.Errorf("water sprite never cycles: %v", seen)
	}
}
