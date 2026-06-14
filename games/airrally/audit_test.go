package airrally

import (
	"math"
	"testing"

	"hsdemo/engine"
	"hsdemo/kart"
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
