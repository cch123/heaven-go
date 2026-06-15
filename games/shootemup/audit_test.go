package shootemup

import (
	"strings"
	"testing"

	"hsdemo/engine"
	"hsdemo/kart"
	"hsdemo/kmdata"
)

func loadAuditAssets(t *testing.T) *kart.Assets {
	t.Helper()
	as, err := kart.Load("../../assets/shootEmUp", engine.SampleRate)
	if err != nil {
		t.Fatalf("assets not extracted: %v", err)
	}
	return as
}

func TestShootEmUpControllersTemplatesAndSounds(t *testing.T) {
	as := loadAuditAssets(t)
	for path, ctrl := range map[string]string{
		"IntroGate":               "IntroGate",
		"Monitor":                 "MonitorHolder",
		"Monitor/monitor/Captain": "Captain",
		"ship":                    "ship",
		"laser":                   "laser",
		"DamageEffect":            "DamageEffect",
		"prefabs/enemy":           "enemy",
		"prefabs/trajectory":      "trajectory",
		"prefabs/origin":          "origin",
		"prefabs/impact":          "impact",
		"prefabs/missimpact":      "missimpact",
	} {
		if got := as.Animators[path]; got != ctrl {
			t.Fatalf("animator %s = %q, want %q", path, got, ctrl)
		}
	}
	for _, root := range []string{"prefabs/enemy", "prefabs/trajectory", "prefabs/origin", "prefabs/impact", "prefabs/missimpact"} {
		if kart.NewTemplate(as, root) == nil {
			t.Fatalf("template %s not extractable", root)
		}
	}
	for _, snd := range []string{"15", "16", "commEnd", "commStart", "gate1", "gate2", "gate3", "shoot", "spawn"} {
		if len(as.Sounds[snd]) == 0 {
			t.Fatalf("missing sound %s", snd)
		}
	}
	for _, sprite := range []string{"shoot_ring", "shoot_sparkle", "shoot_piece1", "smoke1"} {
		if _, ok := as.Sheet.Sprites[sprite]; !ok {
			t.Fatalf("missing particle sprite %s", sprite)
		}
	}
}

func TestShootEmUpControllerStatesAndClipPaths(t *testing.T) {
	as := loadAuditAssets(t)
	want := map[string][]string{
		"IntroGate":     {"gateHidden", "gateShow", "gateOpen1", "gateOpen2", "gateOpen3"},
		"MonitorHolder": {"monitorHidden", "monitorIn", "monitorOut", "monitorIdle"},
		"Captain":       {"capHidden", "capShow", "capHide", "capIdle", "capTalk", "capBop"},
		"ship":          {"shipIdle", "shipShoot", "shipDamage"},
		"laser":         {"idle", "laser"},
		"DamageEffect":  {"Idle", "damage"},
		"enemy":         {"Basic", "Practice", "Endless", "Arrange", "Remix9", "Lockstep", "enemySpawn", "enemyAttack", "enemyMissLeft", "enemyMissRight"},
		"trajectory":    {"trajectory", "trajectory_damage"},
		"origin":        {"origin"},
		"impact":        {"impact"},
		"missimpact":    {"missimpact"},
	}
	for ctrl, states := range want {
		c, ok := as.Controllers[ctrl]
		if !ok {
			t.Fatalf("missing controller %s", ctrl)
		}
		for _, st := range states {
			if _, ok := c.States[st]; !ok {
				t.Fatalf("controller %s missing state %s", ctrl, st)
			}
		}
	}

	nodes := map[string]bool{}
	for _, n := range as.Rig.Nodes {
		nodes[n.Path] = true
	}
	covered := map[string]bool{}
	for root, ctrlName := range as.Animators {
		ctrl := as.Controllers[ctrlName]
		for stName, st := range ctrl.States {
			if st.Clip == "" {
				continue
			}
			covered[st.Clip] = true
			anim := as.Anims[st.Clip]
			if anim == nil {
				t.Fatalf("controller %s state %s missing clip %s", ctrlName, stName, st.Clip)
			}
			checkAnimPaths(t, root, st.Clip, anim, nodes)
			checkSupportedAttrs(t, st.Clip, anim)
		}
	}
	for clip := range as.Anims {
		if strings.Contains(clip, "/") && !covered[clip] {
			t.Fatalf("clip %s is not driven by any controller state", clip)
		}
	}
}

func TestShootEmUpPlacementPatternsAndEnemyPalette(t *testing.T) {
	if len(placementPatterns) != 3 {
		t.Fatalf("patterns = %d, want 3", len(placementPatterns))
	}
	for pi, pattern := range placementPatterns {
		if len(pattern) != 13 {
			t.Fatalf("pattern %d rows = %d, want 13", pi, len(pattern))
		}
		for count, row := range pattern {
			if len(row) != count+1 {
				t.Fatalf("pattern %d row %d positions = %d", pi, count+1, len(row))
			}
		}
	}
	if p := placementFor(1, 5, 2); p != (vec2{0, 1}) {
		t.Fatalf("pattern B count 5 index 2 = %#v, want center", p)
	}
	if enemySprite(enemyArrange) != "shoot_enemy_8" || enemySprite(enemyLockstep) != "shoot_enemy_1" {
		t.Fatalf("enemy type sprite mapping drifted")
	}
}

func checkAnimPaths(t *testing.T, root, clip string, anim *kmdata.Anim, nodes map[string]bool) {
	t.Helper()
	for p := range animPathSet(anim) {
		if root == "prefabs/trajectory/sprite" && p == "sprite" {
			// The prefab includes an unused child Animator on sprite with the
			// same controller as the trajectory root. Unity cannot resolve this
			// self-recursive binding either, and C# only drives the root Animator.
			continue
		}
		full := root
		if p != "" {
			full += "/" + p
		}
		if !nodes[full] {
			t.Fatalf("clip %s path %q under %q resolves to missing node %q", clip, p, root, full)
		}
	}
}

func animPathSet(anim *kmdata.Anim) map[string]bool {
	out := map[string]bool{}
	for p := range anim.Pos {
		out[p] = true
	}
	for p := range anim.Euler {
		out[p] = true
	}
	for p := range anim.Scale {
		out[p] = true
	}
	for p := range anim.Sprites {
		out[p] = true
	}
	for p := range anim.Floats {
		out[p] = true
	}
	return out
}

func checkSupportedAttrs(t *testing.T, clip string, anim *kmdata.Anim) {
	t.Helper()
	for _, attrs := range anim.Floats {
		for attr := range attrs {
			if !supportedFloatAttr(attr) {
				t.Fatalf("clip %s uses unsupported attr %s", clip, attr)
			}
		}
	}
}

func supportedFloatAttr(attr string) bool {
	switch attr {
	case "m_FlipX", "m_FlipY", "m_SortingOrder", "m_IsActive", "m_Enabled", "m_Size.x", "m_Size.y":
		return true
	}
	return strings.HasPrefix(attr, "m_Color.") ||
		strings.HasPrefix(attr, "m_fontColor.") ||
		strings.HasPrefix(attr, "material._AddColor.")
}
