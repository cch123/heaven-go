package catchoftheday

import (
	"path"
	"testing"

	"hsdemo/engine"
	"hsdemo/kart"
)

func loadAuditAssets(t *testing.T) *kart.Assets {
	t.Helper()
	as, err := kart.Load("../../assets/catchOfTheDay", engine.SampleRate)
	if err != nil {
		t.Skipf("assets not extracted: %v", err)
	}
	return as
}

func TestExtractedSceneRolesTemplatesAndSounds(t *testing.T) {
	as := loadAuditAssets(t)
	for _, node := range []string{
		"LakeScene", "LakeScene/Renderer/FishAnimator", "LakeScene/Renderer/Background",
		"LakeScene/Renderer/BigManta", "LakeScene/Renderer/SmallManta",
		"LakeScene/Renderer/Background/Fish/BGFish1", "SchoolFish", "SchoolFish/Body",
		"StickyCanvas/Angler", "StickyCanvas/Angler/Character",
	} {
		if _, ok := as.NodeIndex(node); !ok {
			t.Fatalf("missing node %s", node)
		}
	}
	for _, role := range []string{"Angler", "LakeScenePrefab", "LakeSceneHolder", "AnglerTransform"} {
		if as.Roles[role] == "" {
			t.Fatalf("missing role %s", role)
		}
	}
	if kart.NewTemplate(as, as.Roles["LakeScenePrefab"]) == nil {
		t.Fatalf("missing LakeScene template %q", as.Roles["LakeScenePrefab"])
	}
	if kart.NewTemplate(as, "SchoolFish") == nil {
		t.Fatal("missing SchoolFish template")
	}
	game := as.Extra.Components["game"]
	for _, ref := range []string{"Angler", "LakeScenePrefab", "LakeSceneHolder", "AnglerTransform", "_StickyCanvas"} {
		if game.Refs[ref] == "" {
			t.Fatalf("game component missing ref %s", ref)
		}
	}
	lake := as.Extra.Components["lake"]
	for _, ref := range []string{
		"FishAnimator", "BGAnimator", "GradientBG", "TopBG", "BottomBG", "Renderer",
		"BigManta", "SmallManta", "FishSchoolHolder", "SchoolFishPrefab",
	} {
		if lake.Refs[ref] == "" {
			t.Fatalf("lake component missing ref %s", ref)
		}
	}
	if len(lake.RefArrays["BGFishes"]) != 8 || len(lake.RefArrays["Bubbles"]) != 8 {
		t.Fatalf("lake arrays not extracted: bg=%d bubbles=%d", len(lake.RefArrays["BGFishes"]), len(lake.RefArrays["Bubbles"]))
	}
	for _, snd := range []string{
		"quick1", "quick2", "quick3", "quick_laugh",
		"pausegill1", "pausegill2", "pausegill3", "pausegill4", "pausegill_laugh",
		"threefish1", "threefish2", "threefish3", "threefish4", "threefish5", "threefish_laugh",
		"nearMiss", "common_miss", "common_count-ins_and", "common_count-ins_go1",
		"common_count-ins_go2", "common_count-ins_one1", "common_count-ins_two1", "common_count-ins_three1",
	} {
		if as.Sounds[snd] == nil {
			t.Fatalf("missing sound %s", snd)
		}
	}
}

func TestControllersAndAnimationPaths(t *testing.T) {
	as := loadAuditAssets(t)
	for root, ctrl := range map[string]string{
		"LakeScene/Renderer/FishAnimator":            "FishAnimator",
		"LakeScene/Renderer/Background":              "Background",
		"LakeScene/Renderer/BigManta":                "BigManta",
		"LakeScene/Renderer/SmallManta":              "SmallManta",
		"LakeScene/Renderer/Background/Fish/BGFish1": "BGFish",
		"SchoolFish":                    "SchoolFish",
		"StickyCanvas/Angler/Character": "Angler",
	} {
		if got := as.Animators[root]; got != ctrl {
			t.Fatalf("animator %s = %q, want %q", root, got, ctrl)
		}
	}
	for ctrl, states := range map[string][]string{
		"Angler":       {"Idle", "Pick", "Just", "Miss", "Through"},
		"Background":   {"LayoutA", "LayoutB", "LayoutC"},
		"BigManta":     {"Idle", "Swim"},
		"SmallManta":   {"Idle"},
		"SchoolFish":   {"Idle"},
		"BGFish":       {"BGFishIdle", "BGFishOut_E", "BGFishOut_W", "BGFishOut_NE", "BGFishOut_SW"},
		"FishAnimator": {"Fish1_Wait", "Fish1_Pick", "Fish1_Bite", "Fish1_Just", "Fish1_Miss", "Fish1_Out", "Fish1_Through", "Fish2_Wait", "Fish2_Pick", "Fish2_Bite", "Fish3_Wait", "Fish3_WaitB", "Fish3_PickUp", "Fish3_PickDown", "Fish3_Bite"},
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
	for ctrl, root := range map[string]string{
		"Angler":       "StickyCanvas/Angler/Character",
		"Background":   "LakeScene/Renderer/Background",
		"BigManta":     "LakeScene/Renderer/BigManta",
		"SmallManta":   "LakeScene/Renderer/SmallManta",
		"SchoolFish":   "SchoolFish",
		"BGFish":       "LakeScene/Renderer/Background/Fish/BGFish1",
		"FishAnimator": "LakeScene/Renderer/FishAnimator",
	} {
		c := as.Controllers[ctrl]
		for state, st := range c.States {
			if st.Clip == "" {
				continue
			}
			assertClipPaths(t, as, st.Clip, root, ctrl+"/"+state)
		}
	}
}

func TestTimingAndStateHelpers(t *testing.T) {
	quick := &fishEvent{kind: fishQuick, beat: 10}
	pause := &fishEvent{kind: fishPause, beat: 10}
	three := &fishEvent{kind: fishThree, beat: 10}
	if targetBeat(quick) != 12 || targetBeat(pause) != 13 || targetBeat(three) != 14.5 {
		t.Fatalf("target beats changed: %v %v %v", targetBeat(quick), targetBeat(pause), targetBeat(three))
	}
	if fishState(fishThree, "Pick", true) != "Fish3_PickDown" || fishState(fishThree, "Pick", false) != "Fish3_PickUp" {
		t.Fatalf("threefish pick states changed")
	}
	if bgFishFleeState(8, false) != "BGFishOut_W" || bgFishFleeState(8, true) != "BGFishOut_E" {
		t.Fatalf("BGFish west flip mapping changed")
	}
}

func assertClipPaths(t *testing.T, as *kart.Assets, clip, root, label string) {
	t.Helper()
	anim := as.Anims[clip]
	if anim == nil {
		t.Fatalf("%s missing clip %s", label, clip)
	}
	check := func(curvePath string) {
		full := root
		if curvePath != "" {
			full = path.Join(root, curvePath)
		}
		if _, ok := as.NodeIndex(full); !ok {
			t.Fatalf("%s clip %s curve path %q resolved to missing node %q", label, clip, curvePath, full)
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
