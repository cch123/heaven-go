package magicgirl

import (
	"strings"
	"testing"

	"hsdemo/engine"
	"hsdemo/kart"
	"hsdemo/kmdata"
)

func loadAuditAssets(t *testing.T) *kart.Assets {
	t.Helper()
	as, err := kart.Load("../../assets/magicGirl", engine.SampleRate)
	if err != nil {
		t.Fatalf("assets not extracted: %v", err)
	}
	return as
}

func TestMagicGirlRolesCurvesAndSounds(t *testing.T) {
	as := loadAuditAssets(t)
	for field, want := range map[string]string{
		"MakoObject":      "MagicGirl",
		"Mako":            "MagicGirl/Mako",
		"MakoFace":        "MagicGirl/Mako/HeadandHair/Head",
		"MonsterHands":    "MonsterHand",
		"TransfComponent": "TransformationComponent",
		"jumpEffect":      "JumpParticle",
	} {
		if got := as.Roles[field]; got != want {
			t.Fatalf("role %s = %q, want %q", field, got, want)
		}
	}
	for i := 0; i < 4; i++ {
		key := "monster" + itoa(i) + ".fleeCurve"
		if got := len(as.Extra.Curves[key].Points); got != 2 {
			t.Fatalf("%s has %d points, want 2", key, got)
		}
		comp := as.Extra.Components["monster"+itoa(i)]
		if comp.Path != "Monster"+itoa(i) {
			t.Fatalf("monster%d component path = %q", i, comp.Path)
		}
	}
	for _, snd := range []string{
		"doingoing", "enemy", "hand1", "hand2", "hit", "hold",
		"pass_turn", "sparkle", "spawn_monster", "common_miss", "common_nearMiss",
	} {
		if len(as.Sounds[snd]) == 0 {
			t.Fatalf("missing sound %s", snd)
		}
	}
}

func TestMagicGirlControllersAndAnimationPaths(t *testing.T) {
	as := loadAuditAssets(t)
	wantStates := map[string][]string{
		"MGMako": {
			"Bop", "Prepare", "Hold", "Jump", "U_Left_Move", "U_Right_Move",
			"D_Left_Move", "D_Right_Move", "Hurt_L", "Hurt_R",
			"Uniform", "PhaseA", "PhaseB", "PhaseC", "PhaseD", "Final",
			"FullFlash", "FlashQuick", "IdleFlash",
		},
		"MGHead":         {"FaceIdle", "FaceBarely", "FaceMiss", "FaceSmile", "FaceWink"},
		"Monster":        {"MonsterIdle", "MonsterAppear", "MonsterScared", "MonsterAttackU", "MonsterAttackD"},
		"MonsterHand":    {"Idle", "Appear", "Attack", "Hide"},
		"TransfCompAnim": {"Idle", "Appear", "DressA", "DressB", "Hand", "Legs", "Hide"},
	}
	for ctrlName, states := range wantStates {
		ctrl, ok := as.Controllers[ctrlName]
		if !ok {
			t.Fatalf("missing controller %s", ctrlName)
		}
		for _, state := range states {
			if _, ok := ctrl.States[state]; !ok {
				t.Fatalf("controller %s missing state %s", ctrlName, state)
			}
		}
	}
	checkAllAnimatorPaths(t, as)
}

func checkAllAnimatorPaths(t *testing.T, as *kart.Assets) {
	t.Helper()
	nodes := map[string]bool{}
	for _, n := range as.Rig.Nodes {
		nodes[n.Path] = true
	}
	covered := map[string]bool{}
	for root, ctrlName := range as.Animators {
		if !nodes[root] {
			t.Fatalf("animator root %s missing from scene", root)
		}
		ctrl := as.Controllers[ctrlName]
		for stName, st := range ctrl.States {
			if st.Clip == "" {
				continue
			}
			anim := as.Anims[st.Clip]
			if anim == nil {
				t.Fatalf("controller %s state %s missing clip %s", ctrlName, stName, st.Clip)
			}
			covered[st.Clip] = true
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

func checkAnimPaths(t *testing.T, root, clip string, anim *kmdata.Anim, nodes map[string]bool) {
	t.Helper()
	for p := range animPathSet(anim) {
		full := root
		if p != "" {
			full += "/" + p
		}
		if !nodes[full] {
			t.Fatalf("clip %s path %q resolves to missing node %q", clip, p, full)
		}
	}
}

func animPathSet(anim *kmdata.Anim) map[string]bool {
	paths := map[string]bool{}
	for p := range anim.Pos {
		paths[p] = true
	}
	for p := range anim.Scale {
		paths[p] = true
	}
	for p := range anim.Euler {
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

func checkSupportedAttrs(t *testing.T, clip string, anim *kmdata.Anim) {
	t.Helper()
	for path, attrs := range anim.Floats {
		for attr := range attrs {
			switch attr {
			case "m_Color.r", "m_Color.g", "m_Color.b", "m_Color.a",
				"material._Color.r", "material._Color.g", "material._Color.b", "material._Color.a",
				"material._AddColor.r", "material._AddColor.g", "material._AddColor.b", "material._AddColor.a",
				"material._BlendColor.r", "material._BlendColor.g", "material._BlendColor.b", "material._BlendColor.a",
				"m_Size.x", "m_Size.y", "m_FlipX", "m_FlipY",
				"m_SortingOrder", "m_IsActive", "m_Enabled":
			default:
				t.Fatalf("clip %s path %s unsupported float attr %s", clip, path, attr)
			}
		}
	}
}
