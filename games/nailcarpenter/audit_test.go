package nailcarpenter

import (
	"math"
	"path"
	"testing"

	"hsdemo/engine"
	"hsdemo/kart"
)

func loadAuditAssets(t *testing.T) *kart.Assets {
	t.Helper()
	as, err := kart.Load("../../assets/nailCarpenter", engine.SampleRate)
	if err != nil {
		t.Skipf("assets not extracted: %v", err)
	}
	return as
}

func TestExtractedAssets(t *testing.T) {
	as := loadAuditAssets(t)
	for _, node := range []string{
		"Carpenter", "Carpenter/ExclamRed", "Carpenter/ExclamBlue",
		"Board", "ScrollingItems/Prefabs/Nail", "ScrollingItems/Prefabs/LongNail",
		"ScrollingItems/Prefabs/Sweet", "ScrollingItems/NailHolder",
		"BGContainer/BG", "FusumaContainer/Fusuma",
	} {
		if _, ok := as.NodeIndex(node); !ok {
			t.Fatalf("missing scene node %s", node)
		}
	}
	for _, snd := range []string{
		"HammerStrong", "HammerWeak", "alarm", "one", "open", "signal1", "signal2", "three",
	} {
		if as.Sounds[snd] == nil {
			t.Fatalf("missing sound %s", snd)
		}
	}
	for _, root := range []string{
		"ScrollingItems/Prefabs/Nail", "ScrollingItems/Prefabs/LongNail", "ScrollingItems/Prefabs/Sweet",
	} {
		if kart.NewTemplate(as, root) == nil {
			t.Fatalf("template %s not created", root)
		}
	}
}

func TestAnimationClipsControllersAndPaths(t *testing.T) {
	as := loadAuditAssets(t)
	for ctrl, states := range map[string][]string{
		"Carpenter": {"carpenterArmUp", "carpenterHit", "carpenterHitLong", "carpenterIdle", "eyeBlink", "eyeBlinkFast", "eyeOpen", "eyeSmile"},
		"Exclam":    {"exclamAppear", "exclamNothing"},
		"Nail":      {"nailBendLeft", "nailBendRight", "nailHammered", "nailIdle", "nailMiss", "nailStrongHammered"},
		"LongNail":  {"longNailBendLeft", "longNailBendRight", "longNailHammered", "longNailIdle", "longNailMiss", "longNailWeakHammered"},
		"Sweet": {
			"cherryBreak", "cherryIdle", "cherryPuddingBeat", "cherryPuddingBreak", "cherryPuddingIdle",
			"layerCakeBeat", "layerCakeBreak", "layerCakeIdle", "puddingBeat", "puddingBreak", "puddingIdle",
			"shortCakeBeat", "shortCakeBreak", "shortCakeIdle",
		},
	} {
		c := as.Controllers[ctrl]
		if c.States == nil {
			t.Fatalf("missing controller %s", ctrl)
		}
		for _, st := range states {
			cs, ok := c.States[st]
			if !ok {
				t.Fatalf("controller %s missing state %s", ctrl, st)
			}
			if cs.Clip != "" && as.Anims[cs.Clip] == nil {
				t.Fatalf("controller %s state %s references missing clip %s", ctrl, st, cs.Clip)
			}
		}
	}
	wantAnimators := map[string]string{
		"Carpenter":                                    "Carpenter",
		"Carpenter/ExclamRed":                          "Exclam",
		"Carpenter/ExclamBlue":                         "Exclam",
		"ScrollingItems/Prefabs/Nail/Pivot/Sprite":     "Nail",
		"ScrollingItems/Prefabs/LongNail/Pivot/Sprite": "LongNail",
		"ScrollingItems/Prefabs/Sweet/Pivot/Sprite":    "Sweet",
	}
	for root, ctrl := range wantAnimators {
		if got := as.Animators[root]; got != ctrl {
			t.Fatalf("animator %s = %q, want %q", root, got, ctrl)
		}
	}
	for clip, root := range map[string]string{
		"Animations/carpenterArmUp": "Carpenter", "Animations/carpenterHit": "Carpenter",
		"Animations/carpenterHitLong": "Carpenter", "Animations/carpenterIdle": "Carpenter",
		"Animations/eyeBlink": "Carpenter", "Animations/eyeBlinkFast": "Carpenter",
		"Animations/eyeOpen": "Carpenter", "Animations/eyeSmile": "Carpenter",
		"Animations/exclamAppear": "Carpenter/ExclamRed", "Animations/exclamNothing": "Carpenter/ExclamRed",
		"Animations/nailBendLeft":         "ScrollingItems/Prefabs/Nail/Pivot/Sprite",
		"Animations/nailBendRight":        "ScrollingItems/Prefabs/Nail/Pivot/Sprite",
		"Animations/nailHammered":         "ScrollingItems/Prefabs/Nail/Pivot/Sprite",
		"Animations/nailIdle":             "ScrollingItems/Prefabs/Nail/Pivot/Sprite",
		"Animations/nailMiss":             "ScrollingItems/Prefabs/Nail/Pivot/Sprite",
		"Animations/nailStrongHammered":   "ScrollingItems/Prefabs/Nail/Pivot/Sprite",
		"Animations/longNailBendLeft":     "ScrollingItems/Prefabs/LongNail/Pivot/Sprite",
		"Animations/longNailBendRight":    "ScrollingItems/Prefabs/LongNail/Pivot/Sprite",
		"Animations/longNailHammered":     "ScrollingItems/Prefabs/LongNail/Pivot/Sprite",
		"Animations/longNailIdle":         "ScrollingItems/Prefabs/LongNail/Pivot/Sprite",
		"Animations/longNailMiss":         "ScrollingItems/Prefabs/LongNail/Pivot/Sprite",
		"Animations/longNailWeakHammered": "ScrollingItems/Prefabs/LongNail/Pivot/Sprite",
		"Sweet/cherryBreak":               "ScrollingItems/Prefabs/Sweet/Pivot/Sprite",
		"Sweet/cherryIdle":                "ScrollingItems/Prefabs/Sweet/Pivot/Sprite",
		"Sweet/cherryPuddingBeat":         "ScrollingItems/Prefabs/Sweet/Pivot/Sprite",
		"Sweet/cherryPuddingBreak":        "ScrollingItems/Prefabs/Sweet/Pivot/Sprite",
		"Sweet/cherryPuddingIdle":         "ScrollingItems/Prefabs/Sweet/Pivot/Sprite",
		"Sweet/layerCakeBeat":             "ScrollingItems/Prefabs/Sweet/Pivot/Sprite",
		"Sweet/layerCakeBreak":            "ScrollingItems/Prefabs/Sweet/Pivot/Sprite",
		"Sweet/layerCakeIdle":             "ScrollingItems/Prefabs/Sweet/Pivot/Sprite",
		"Sweet/puddingBeat":               "ScrollingItems/Prefabs/Sweet/Pivot/Sprite",
		"Sweet/puddingBreak":              "ScrollingItems/Prefabs/Sweet/Pivot/Sprite",
		"Sweet/puddingIdle":               "ScrollingItems/Prefabs/Sweet/Pivot/Sprite",
		"Sweet/shortCakeBeat":             "ScrollingItems/Prefabs/Sweet/Pivot/Sprite",
		"Sweet/shortCakeBreak":            "ScrollingItems/Prefabs/Sweet/Pivot/Sprite",
		"Sweet/shortCakeIdle":             "ScrollingItems/Prefabs/Sweet/Pivot/Sprite",
	} {
		assertClipPaths(t, as, clip, root)
	}
}

func TestPatternDataMatchesPrefab(t *testing.T) {
	tests := []struct {
		name string
		typ  int
		want []patternItem
	}{
		{"pudding", patternPudding, []patternItem{{0, objectSweet}, {1, objectNail}, {2, objectNone}}},
		{"cherry", patternCherry, []patternItem{{0, objectSweet}, {1, objectNail}, {2, objectNail}, {3, objectNail}, {4, objectNone}}},
		{"cake", patternCake, []patternItem{{0, objectSweet}, {1, objectNail}, {2, objectForceCherry}, {2.5, objectNail}, {3.5, objectNail}, {4, objectNone}}},
		{"cakeLong", patternCakeLong, []patternItem{{0, objectSweet}, {1, objectNail}, {2, objectLongCharge}, {3, objectLongNail}, {4, objectNone}}},
		{"puddingOld", patternPuddingOld, []patternItem{{0, objectSweet}, {0.5, objectNail}, {1, objectNone}}},
		{"cherryOld", patternCherryOld, []patternItem{{0, objectSweet}, {0.5, objectNail}, {1, objectNail}, {1.5, objectNail}, {2, objectNone}}},
		{"cakeOld", patternCakeOld, []patternItem{{0, objectSweet}, {0.5, objectNail}, {1, objectForceCherry}, {1.25, objectNail}, {1.75, objectNail}, {2, objectNone}}},
		{"cakeLongOld", patternCakeLongOld, []patternItem{{0, objectSweet}, {0.5, objectNail}, {1, objectLongCharge}, {1.5, objectLongNail}, {2, objectNone}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := patternItems(tt.typ)
			if len(got) != len(tt.want) {
				t.Fatalf("len = %d, want %d", len(got), len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("[%d] = %#v, want %#v", i, got[i], tt.want[i])
				}
			}
			if patternLength(tt.typ) != tt.want[len(tt.want)-1].beat {
				t.Fatalf("patternLength mismatch")
			}
		})
	}
}

func TestShojiAndPatternSemantics(t *testing.T) {
	m := &Module{
		slides: []slideEvt{{beat: 4, length: 2, ratio: 0.5, ease: 0}},
		legacy: []legacyEvt{
			{beat: 8, set: true},
			{beat: 16, set: false},
		},
	}
	if got := m.shojiXAt(0); got != shojiFullOpenX {
		t.Fatalf("initial shoji x = %v", got)
	}
	if got := m.shojiXAt(5); math.Abs(got-shojiFullOpenX*0.75) > 1e-9 {
		t.Fatalf("mid shoji x = %v", got)
	}
	if got := m.shojiXAt(7); got != shojiFullOpenX*0.5 {
		t.Fatalf("final shoji x = %v", got)
	}
	if !m.legacyAt(9) || m.legacyAt(17) {
		t.Fatalf("legacyAt changed")
	}
	if sweetForPattern(patternCakeLong) != sweetLayerCake || sweetBeatState(sweetCherry) != "" {
		t.Fatalf("sweet mapping changed")
	}
}

func assertClipPaths(t *testing.T, as *kart.Assets, clip, root string) {
	t.Helper()
	anim := as.Anims[clip]
	if anim == nil {
		t.Fatalf("missing clip %s", clip)
	}
	check := func(curvePath string) {
		full := root
		if curvePath != "" {
			full = path.Join(root, curvePath)
		}
		if _, ok := as.NodeIndex(full); !ok {
			t.Fatalf("%s curve path %q resolved to missing node %q", clip, curvePath, full)
		}
	}
	for p := range anim.Pos {
		check(p)
	}
	for p := range anim.Scale {
		check(p)
	}
	for p := range anim.Euler {
		check(p)
	}
	for p := range anim.Sprites {
		check(p)
	}
	for p := range anim.Floats {
		check(p)
	}
}
