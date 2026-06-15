package fruitbasket

import (
	"math"
	"strings"
	"testing"

	"hsdemo/kart"
	"hsdemo/kmdata"
)

func loadAssets(t *testing.T) *kart.Assets {
	t.Helper()
	as, err := kart.Load("../../assets/fruitBasket", 44100)
	if err != nil {
		t.Skipf("assets not extracted: %v", err)
	}
	return as
}

func TestFruitBasketBindingsPathsAndSounds(t *testing.T) {
	as := loadAssets(t)
	nodes := nodeSet(as)
	game := as.Extra.Components["game"]
	for field, want := range map[string]string{
		"applePrefab":            "Apple",
		"lemonPrefab":            "Lemon",
		"melonPrefab":            "Melon",
		"thoughtBubblePrefab":    "ThoughtBubble",
		"courtneyAnimator":       "Courtney",
		"courtneySprite":         "Courtney/Body",
		"courtneyExtendedSprite": "Courtney/Extended",
		"courtneyHoleSprite":     "Courtney/Hole",
		"hoopL":                  "HoopL",
		"hoopR":                  "HoopR",
		"catsAnimator":           "BG/Cats",
	} {
		if got := game.Refs[field]; got != want || !nodes[got] {
			t.Fatalf("game ref %s = %q, want existing %q", field, got, want)
		}
	}
	for name, comp := range map[string]kmdata.Component{
		"apple": as.Extra.Components["apple"],
		"lemon": as.Extra.Components["lemon"],
		"melon": as.Extra.Components["melon"],
	} {
		if comp.Path == "" || !nodes[comp.Path] {
			t.Fatalf("%s component path = %q", name, comp.Path)
		}
		if comp.Refs["sprite"] == "" || !nodes[comp.Refs["sprite"]] {
			t.Fatalf("%s sprite ref = %q", name, comp.Refs["sprite"])
		}
	}
	melon := as.Extra.Components["melon"]
	for field, want := range map[string]string{"leftPipeAnim": "PipeL", "rightPipeAnim": "PipeR"} {
		if got := melon.Refs[field]; got != want || !nodes[got] {
			t.Fatalf("melon ref %s = %q, want existing %q", field, got, want)
		}
	}
	for _, snd := range []string{
		"apple", "appleL", "appleR", "lemon", "lemonL", "lemonR",
		"melonL", "melonR", "whistle", "dunk", "common_miss",
		"basket", "basketL", "basketR", "goalHitL", "goalHitR",
		"melonImpactL", "melonImpactR", "melonBasketL", "melonBasketR", "melonBasketCenter",
	} {
		if _, ok := as.Sounds[snd]; !ok {
			t.Errorf("missing sound %s", snd)
		}
	}
}

func TestFruitBasketCurveAndParticleAnchors(t *testing.T) {
	as := loadAssets(t)
	nodes := nodeSet(as)
	game := as.Extra.Components["game"]
	paths := map[string]int{}
	for _, it := range game.Lists["fruitPaths"] {
		name := it.Strs["name"]
		paths[name] = len(it.Items["positions"])
		for i, pos := range it.Items["positions"] {
			if target := pos.Refs["target"]; target != "" && !nodes[target] {
				t.Errorf("path %s point %d target %q missing", name, i, target)
			}
		}
	}
	want := map[string]int{
		"ToRightBasket": 3, "ToLeftBasket": 3,
		"LemonRollRight": 6, "LemonRollLeft": 6,
		"ToLeftBasketMelon": 4, "ToRightBasketMelon": 4,
		"ToLeftBasketMiss": 3, "ToRightBasketMiss": 3,
		"ToLeftMiss": 3, "ToRightMiss": 3,
	}
	for name, count := range want {
		if got := paths[name]; got != count {
			t.Fatalf("path %s has %d points, want %d", name, got, count)
		}
	}
	for _, anchor := range []string{
		"HoopL/AppleScore", "HoopL/LemonScore", "HoopL/MelonScore",
		"HoopR/AppleScore", "HoopR/LemonScore", "HoopR/MelonScore",
	} {
		if !nodes[anchor] {
			t.Errorf("missing score ParticleSystem anchor %s", anchor)
		}
	}
}

func TestFruitBasketControllersClipsAndPaths(t *testing.T) {
	as := loadAssets(t)
	for ctrl, states := range map[string][]string{
		"Cats":          {"catsIdle", "catsBop"},
		"Courtney":      {"idle", "fruit_hit", "miss", "whiff", "HappyStart", "Happy", "HappyStop", "CryStart", "Cry", "CryStop", "DaydreamNoneStart", "DaydreamNone", "DaydreamNoneStop", "DaydreamKissStart", "DaydreamKiss", "DaydreamKissStop", "DaydreamTongueStart", "DaydreamTongue", "DaydreamTongueStop", "DaydreamWorriedStart", "DaydreamWorried", "DaydreamWorriedStop", "test"},
		"Hoop":          {"hoop", "hoopScore", "hoopScoreMelon", "hoopShake"},
		"PipeL":         {"pipeIdle", "pipeFlash"},
		"PipeR":         {"pipeIdle", "pipeFlash"},
		"Thought":       {"burger", "BurgerAndDrink", "Girl", "TicTacToe", "Spider", "Skateboard", "Dog", "Panda"},
		"ThoughtBubble": {"New State", "thoughtBubble", "thoughtBubbleFadeIn", "thoughtBubbleFadeOut"},
	} {
		c, ok := as.Controllers[ctrl]
		if !ok {
			t.Fatalf("missing controller %s", ctrl)
		}
		for _, st := range states {
			if _, ok := c.States[st]; !ok {
				t.Errorf("controller %s missing state %s", ctrl, st)
			}
		}
	}
	for path, ctrl := range map[string]string{
		"BG/Cats":                       "Cats",
		"Courtney":                      "Courtney",
		"HoopL":                         "Hoop",
		"HoopR":                         "Hoop",
		"PipeL":                         "PipeL",
		"PipeR":                         "PipeR",
		"ThoughtBubble":                 "ThoughtBubble",
		"ThoughtBubble/BubbleL/Thought": "Thought",
	} {
		if got := as.Animators[path]; got != ctrl {
			t.Errorf("animator %s = %q, want %q", path, got, ctrl)
		}
	}

	nodes := nodeSet(as)
	covered := map[string]bool{}
	for root, ctrlName := range as.Animators {
		for stName, st := range as.Controllers[ctrlName].States {
			if st.Clip == "" {
				continue
			}
			covered[st.Clip] = true
			a := as.Anims[st.Clip]
			if a == nil {
				t.Errorf("%s/%s missing clip %s", ctrlName, stName, st.Clip)
				continue
			}
			checkAnimPaths(t, a, st.Clip, root, nodes)
			checkSupportedAttrs(t, a, st.Clip)
		}
	}

	// These clips are not reachable from controller motions but are still
	// intentionally accounted for: hoopShake is driven directly from Melon.cs,
	// test has an empty controller state plus a raw clip, and the last two are
	// legacy clips shipped with the prefab but not referenced by FruitBasket.cs.
	rawOrLegacy := map[string]string{
		"Animations/hoop/hoopShake":                  "HoopL",
		"Animations/Courtney/test":                   "Courtney",
		"Animations/Courtney/hitFruit":               "Courtney",
		"Animations/thoughtBubble/thoughtBubbleIdle": "ThoughtBubble",
	}
	for clip, root := range rawOrLegacy {
		a := as.Anims[clip]
		if a == nil {
			t.Errorf("missing raw/legacy clip %s", clip)
			continue
		}
		covered[clip] = true
		checkAnimPaths(t, a, clip, root, nodes)
		checkSupportedAttrs(t, a, clip)
	}
	for name := range as.Anims {
		if strings.Contains(name, "/") && !covered[name] {
			t.Errorf("clip %q is not covered by controller, script, or explicit legacy audit", name)
		}
	}
}

func TestFruitBasketTimingNamesAndCurveMath(t *testing.T) {
	if got := expressionBeatFor(fruitApple, 10); got != 14 {
		t.Fatalf("apple expression beat = %v", got)
	}
	if got := expressionBeatFor(fruitLemon, 10); got != 16 {
		t.Fatalf("lemon expression beat = %v", got)
	}
	if expressionNameAll(4) != "DaydreamTongue" || daydreamExpressionName(3) != "Worried" {
		t.Fatalf("expression enum mapping drifted")
	}
	p := curvePath{points: []pathPoint{
		{pos: [2]float64{0, 0}, dur: 1, height: 4},
		{pos: [2]float64{2, 0}, dur: 1},
		{pos: [2]float64{4, 0}},
	}}
	got, idx := p.at(0.5, 0)
	if idx != 0 || math.Abs(got[0]-1) > 1e-6 || math.Abs(got[1]-4) > 1e-6 {
		t.Fatalf("curve midpoint = %v idx %d, want [1 4] idx 0", got, idx)
	}
}

func nodeSet(as *kart.Assets) map[string]bool {
	out := map[string]bool{}
	for _, n := range as.Rig.Nodes {
		out[n.Path] = true
	}
	return out
}

func checkAnimPaths(t *testing.T, anim *kmdata.Anim, clip, root string, nodes map[string]bool) {
	t.Helper()
	for p := range animPaths(anim) {
		full := root
		if p != "" {
			full = root + "/" + p
		}
		if !nodes[full] {
			t.Errorf("clip %s path %q resolves to missing node %q", clip, p, full)
		}
	}
}

func animPaths(anim *kmdata.Anim) map[string]bool {
	paths := map[string]bool{}
	for p := range anim.Pos {
		paths[p] = true
	}
	for p := range anim.Euler {
		paths[p] = true
	}
	for p := range anim.Scale {
		paths[p] = true
	}
	for p := range anim.Sprites {
		paths[p] = true
	}
	for p := range anim.Floats {
		paths[p] = true
	}
	return paths
}

func checkSupportedAttrs(t *testing.T, anim *kmdata.Anim, clip string) {
	t.Helper()
	for _, attrs := range anim.Floats {
		for attr := range attrs {
			if !supportedFloatAttr(attr) {
				t.Errorf("clip %s uses unsupported attr %s", clip, attr)
			}
		}
	}
}

func supportedFloatAttr(attr string) bool {
	switch attr {
	case "m_FlipX", "m_FlipY", "m_SortingOrder", "m_IsActive", "m_Enabled", "m_Size.x", "m_Size.y":
		return true
	default:
		return strings.HasPrefix(attr, "m_Color.") || strings.HasPrefix(attr, "m_fontColor.")
	}
}
