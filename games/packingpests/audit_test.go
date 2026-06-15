package packingpests

import (
	"path/filepath"
	"testing"

	"hsdemo/engine"
	"hsdemo/kart"
	"hsdemo/kmdata"
)

func loadAuditAssets(t *testing.T) *kart.Assets {
	t.Helper()
	as, err := kart.Load(filepath.Join("..", "..", "assets", "packingPests"), engine.SampleRate)
	if err != nil {
		t.Fatal(err)
	}
	return as
}

func TestBindingsTemplatesPathsAndSounds(t *testing.T) {
	as := loadAuditAssets(t)
	for _, role := range []string{
		"Candy", "Spider", "boxfront",
		"handAnim", "lowerHandAnim", "upperHandAnim", "signAnim",
		"spiderCrawlAnim", "spiderAnim", "curtainAnim",
		"HandAnimPlayer", "HandAnim1", "HandAnim2", "HandAnim3", "HandAnim4",
		"HandAnim5", "HandAnim6", "HandAnim7", "HandAnim8",
	} {
		if as.Roles[role] == "" {
			t.Fatalf("missing role %s", role)
		}
	}
	if kart.NewTemplate(as, as.Roles["Candy"]) == nil {
		t.Fatalf("Candy template %q not extractable", as.Roles["Candy"])
	}
	if kart.NewTemplate(as, as.Roles["Spider"]) == nil {
		t.Fatalf("Spider template %q not extractable", as.Roles["Spider"])
	}

	comp := as.Extra.Components["game"]
	paths := map[string]bool{}
	for _, n := range as.Rig.Nodes {
		paths[n.Path] = true
	}
	wantPaths := map[string]bool{
		"candyThrow": true, "spiderThrow": true, "candyCatch": true, "None": true,
		"candyBarely": true, "candyWrong": true, "spiderBarely": true, "spiderWrong": true,
	}
	for _, item := range comp.Lists["objectPaths"] {
		name := item.Strs["name"]
		if !wantPaths[name] {
			t.Fatalf("unexpected objectPath %q", name)
		}
		delete(wantPaths, name)
		pts := item.Items["positions"]
		if len(pts) != 2 {
			t.Fatalf("objectPath %s positions = %d, want 2", name, len(pts))
		}
		for _, pt := range pts {
			if target := pt.Refs["target"]; !paths[target] {
				t.Fatalf("objectPath %s target %q missing from scene", name, target)
			}
		}
	}
	for missing := range wantPaths {
		t.Fatalf("missing objectPath %s", missing)
	}

	for _, snd := range []string{
		"SE_SHIWAKE_EN_BALL_CATCH_A", "SE_SHIWAKE_EN_BALL_CATCH_B", "SE_SHIWAKE_EN_BALL_CATCH_C",
		"SE_SHIWAKE_EN_BALL_OUT", "SE_SHIWAKE_EN_INSECT_ATTACK_A", "SE_SHIWAKE_EN_INSECT_ATTACK_B",
		"SE_SHIWAKE_EN_INSECT_OUT", "SE_SHIWAKE_EN_MISS_BALL_ATTACK_A", "SE_SHIWAKE_EN_MISS_BALL_ATTACK_B",
		"SE_SHIWAKE_EN_MISS_BALL_THROUGH_A", "SE_SHIWAKE_EN_MISS_BALL_THROUGH_B",
		"SE_SHIWAKE_EN_MISS_INSECT_CATCH_A", "SE_SHIWAKE_EN_MISS_INSECT_CATCH_B",
		"SE_SHIWAKE_EN_MISS_INSECT_THROUGH_A", "SE_SHIWAKE_EN_MISS_INSECT_THROUGH_B",
		"SE_SHIWAKE_EN_OSII", "SE_SHIWAKE_EN_SWING", "SE_SHIWAKE_EN_SWING_CATCH_A",
		"SE_SHIWAKE_EN_SWING_CATCH_B", "SE_SHIWAKE_EN_VOICE_READY1", "SE_SHIWAKE_EN_VOICE_READY2",
		"SE_SHIWAKE_EN_VOICE_READY3", "SE_SHIWAKE_EN_VOICE_REST_A", "SE_SHIWAKE_EN_VOICE_REST_B",
	} {
		if as.Sounds[snd] == nil {
			t.Fatalf("missing sound %s", snd)
		}
	}
}

func TestControllersStatesAndAnimationPaths(t *testing.T) {
	as := loadAuditAssets(t)
	for ctrlName, states := range map[string][]string{
		"Curtains":      {"Enter", "Exit", "idle"},
		"WorkerHands":   {"Enter", "Exit", "idle"},
		"Worker":        {"Beat", "Through", "idle"},
		"Upper":         {"CatchBug", "CatchCandy", "CatchJust02", "CatchMiss", "CatchOut", "Damage", "HitJust", "HitOut", "Through", "idle"},
		"Lower":         {"CatchBug", "CatchJust00", "CatchJust01", "CatchMiss", "CatchOut", "Damage", "Through", "idle"},
		"MessageHolder": {"Message00", "Message01", "idle"},
		"Spider":        {"Enter", "idle"},
		"SpiderSwat":    {"Hit", "idle"},
	} {
		ctrl, ok := as.Controllers[ctrlName]
		if !ok {
			t.Fatalf("missing controller %s", ctrlName)
		}
		for _, st := range states {
			state, ok := ctrl.States[st]
			if !ok {
				t.Fatalf("controller %s missing state %s", ctrlName, st)
			}
			if state.Clip == "" || as.Anims[state.Clip] == nil {
				t.Fatalf("controller %s state %s clip %q missing", ctrlName, st, state.Clip)
			}
		}
	}

	nodes := map[string]bool{}
	for _, n := range as.Rig.Nodes {
		nodes[n.Path] = true
	}
	supported := map[string]bool{
		"m_IsActive": true, "m_Enabled": true, "m_SortingOrder": true,
		"m_FlipX": true, "m_FlipY": true, "m_Size.x": true, "m_Size.y": true,
		"m_Color.r": true, "m_Color.g": true, "m_Color.b": true, "m_Color.a": true,
		"m_fontColor.r": true, "m_fontColor.g": true, "m_fontColor.b": true, "m_fontColor.a": true,
	}
	for root, ctrlName := range as.Animators {
		ctrl := as.Controllers[ctrlName]
		if ctrl.States == nil {
			t.Fatalf("animator %s references missing controller %s", root, ctrlName)
		}
		for stName, st := range ctrl.States {
			if st.Clip == "" {
				continue
			}
			anim := as.Anims[st.Clip]
			if anim == nil {
				t.Fatalf("controller %s state %s references missing clip %s", ctrlName, stName, st.Clip)
			}
			checkAnimPaths(t, root, st.Clip, anim, nodes, supported)
		}
	}
}

func checkAnimPaths(t *testing.T, root, clip string, anim *kmdata.Anim, nodes map[string]bool, supported map[string]bool) {
	t.Helper()
	checkPath := func(path string) {
		full := root
		if path != "" {
			full += "/" + path
		}
		if !nodes[full] {
			t.Fatalf("clip %s path %q under root %q missing (%q)", clip, path, root, full)
		}
	}
	for p := range anim.Pos {
		checkPath(p)
	}
	for p := range anim.Euler {
		checkPath(p)
	}
	for p := range anim.Scale {
		checkPath(p)
	}
	for p := range anim.Sprites {
		checkPath(p)
	}
	for p, attrs := range anim.Floats {
		checkPath(p)
		for attr := range attrs {
			if !supported[attr] {
				t.Fatalf("clip %s uses unsupported attr %s", clip, attr)
			}
		}
	}
}
