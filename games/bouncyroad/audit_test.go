package bouncyroad

import (
	"os"
	"testing"

	"hsdemo/engine"
	"hsdemo/kart"
	"hsdemo/kmdata"
)

func loadAssetsOrSkip(t *testing.T) *kart.Assets {
	t.Helper()
	if _, err := os.Stat("../../assets/bouncyRoad/scene.json"); err != nil {
		t.Skipf("bouncyRoad assets not extracted: %v", err)
	}
	as, err := kart.Load("../../assets/bouncyRoad", engine.SampleRate)
	if err != nil {
		t.Fatalf("load assets: %v", err)
	}
	return as
}

func TestExtractedBouncyRoadAssetCoverage(t *testing.T) {
	as := loadAssetsOrSkip(t)
	for _, role := range []string{"baseBall", "baseBounceCurve", "CurveHolder", "ThingsTrans", "PosCurve", "BGGradient", "BGHigh", "BGLow"} {
		if as.Roles[role] == "" {
			t.Fatalf("missing role %s", role)
		}
	}
	for _, clip := range []string{"Animations/Thing Podium", "Animations/A Podium", "Animations/D-Pad Podium"} {
		if as.Anims[clip] == nil {
			t.Fatalf("missing animation %s", clip)
		}
	}
	for _, snd := range []string{"ballBounce", "ballLeft", "ballRight", "goal", "leftBlank", "rightBlank"} {
		if len(as.Sounds[snd]) == 0 {
			t.Fatalf("missing sound %s", snd)
		}
	}
	if len(as.Extra.Curves["PosCurve"].Points) != 3 {
		t.Fatalf("PosCurve points = %d, want 3", len(as.Extra.Curves["PosCurve"].Points))
	}
	if len(as.Extra.Curves["baseBounceCurve"].Points) != 2 {
		t.Fatalf("baseBounceCurve points = %d, want 2", len(as.Extra.Curves["baseBounceCurve"].Points))
	}
}

func TestGeneratedCurveSetMatchesUnityAwakeShape(t *testing.T) {
	as := loadAssetsOrSkip(t)
	ctx := &engine.Ctx{Assets: as, Scene: kart.NewScene(as)}
	m := &Module{ctx: ctx, curveCache: map[float64][]kmdata.Curve{}}
	if idx, ok := as.NodeIndex(as.Roles["ThingsTrans"]); ok {
		for _, n := range as.Rig.Nodes {
			if n.Parent == idx {
				m.things = append(m.things, n.Path)
			}
		}
	}
	curves := m.heightCurves(1)
	if len(curves) != 18 {
		t.Fatalf("curve count = %d, want 18", len(curves))
	}
	if curves[16].Points[1].P[1] >= curves[13].Points[1].P[1] {
		t.Fatal("right miss curve did not lower its end point")
	}
	if curves[17].Points[1].P[1] >= curves[14].Points[1].P[1] {
		t.Fatal("left miss curve did not lower its end point")
	}
}
