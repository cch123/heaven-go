package thedazzles

import (
	"path/filepath"
	"strings"
	"testing"

	"hsdemo/engine"
	"hsdemo/kart"
	"hsdemo/kmdata"
	"hsdemo/riq"
)

func loadAuditAssets(t *testing.T) *kart.Assets {
	t.Helper()
	as, err := kart.Load(filepath.Join("..", "..", "assets", gameID), engine.SampleRate)
	if err != nil {
		t.Fatal(err)
	}
	return as
}

func TestBindingsSoundsAndEffectSprites(t *testing.T) {
	as := loadAuditAssets(t)
	nodes := nodeSet(as)
	comp := as.Extra.Components["game"]
	if as.Roles["player"] != "PlayerHolder/Girl" || comp.Refs["player"] != "PlayerHolder/Girl" {
		t.Fatalf("player binding roles=%q refs=%q", as.Roles["player"], comp.Refs["player"])
	}
	if as.Roles["poseEffect"] != "PoseParticle" || as.Roles["starsEffect"] != "NightWalkStars" {
		t.Fatalf("effect roles = %v", as.Roles)
	}
	wantNPC := []string{"NpcHolder/Girl", "NpcHolder (1)/Girl", "NpcHolder (2)/Girl", "NpcHolder (3)/Girl", "NpcHolder (4)/Girl"}
	if got := comp.RefArrays["npcGirls"]; len(got) != len(wantNPC) {
		t.Fatalf("npcGirls len = %d, want %d", len(got), len(wantNPC))
	} else {
		for i, want := range wantNPC {
			if got[i] != want || !nodes[want] {
				t.Fatalf("npcGirls[%d] = %q, want existing %q", i, got[i], want)
			}
		}
	}
	if comp.Refs["interiorMat"] != "boxinterior" {
		t.Fatalf("interiorMat = %q", comp.Refs["interiorMat"])
	}
	for _, p := range append(wantNPC, "PlayerHolder/Girl") {
		for _, suffix := range []string{"", "/HoldEffect", "/BlackFlash", "/Head"} {
			if !nodes[p+suffix] {
				t.Fatalf("girl path %q missing", p+suffix)
			}
		}
	}
	for _, name := range []string{
		"applause", "crouch", "hold1", "hold2", "hold3", "holdDS2", "holdDS3",
		"miss", "pose", "posePlayer", "stars1", "stars2", "stars3", "stars4", "stars5",
	} {
		if len(as.Sounds[name]) == 0 {
			t.Fatalf("missing sound %s", name)
		}
	}
	for i := 0; i <= 9; i++ {
		if _, ok := as.Sheet.Sprites["dazzleseffects_"+smallInt(i)]; !ok {
			t.Fatalf("missing effect sprite dazzleseffects_%d", i)
		}
	}
}

func TestControllersClipsAndAnimationPaths(t *testing.T) {
	as := loadAuditAssets(t)
	for ctrlName, states := range map[string][]string{
		"DazzlesGirl": {"Idle", "Prepare", "Hold", "Shake", "Pose", "MissPose", "MissEndPose", "EndPose", "EndPrepare", "StopHold", "Ouch", "IdleBop", "HappyBop", "OuchBop"},
		"GirlHolder":  {"Lit", "Dark", "PoseFlash", "MissFlash"},
		"HoldEffect":  {"HoldNothing", "HoldBox", "ReleaseBox"},
	} {
		ctrl, ok := as.Controllers[ctrlName]
		if !ok {
			t.Fatalf("missing controller %s", ctrlName)
		}
		for _, stName := range states {
			st, ok := ctrl.States[stName]
			if !ok {
				t.Fatalf("controller %s missing state %s", ctrlName, stName)
			}
			if st.Clip == "" || as.Anims[st.Clip] == nil {
				t.Fatalf("controller %s state %s clip %q missing", ctrlName, stName, st.Clip)
			}
		}
	}
	nodes := nodeSet(as)
	for root, ctrlName := range as.Animators {
		if !nodes[root] {
			t.Fatalf("animator path %q missing", root)
		}
		for stName, st := range as.Controllers[ctrlName].States {
			if st.Clip == "" {
				t.Fatalf("controller %s state %s has no clip", ctrlName, stName)
			}
			checkAnimPaths(t, root, st.Clip, as.Anims[st.Clip], nodes)
		}
	}
}

func TestAllUnityClipsAccountedAndSpriteSwapsResolve(t *testing.T) {
	as := loadAuditAssets(t)
	ctrlClips := map[string]bool{}
	for _, ctrl := range as.Controllers {
		for _, st := range ctrl.States {
			ctrlClips[st.Clip] = true
		}
	}
	for name := range as.Anims {
		if strings.HasPrefix(name, "Animations/") && !ctrlClips[name] {
			t.Errorf("Unity clip %q has no controller state", name)
		}
	}
	for name, a := range as.Anims {
		for path, keys := range a.Sprites {
			for _, k := range keys {
				if k.Name == "" {
					continue
				}
				if _, ok := as.Sheet.Sprites[k.Name]; !ok {
					t.Errorf("clip %s path %q sprite %q missing from atlas", name, path, k.Name)
				}
			}
		}
	}
}

func TestRuntimeHelpersMatchScriptSemantics(t *testing.T) {
	ev := &riq.Entity{Data: map[string]any{"toggle2": true}}
	if !boolDefault(ev, "toggle2", false) || !boolDefault(ev, "missing", true) || boolDefault(ev, "missing", false) {
		t.Fatal("bool defaults no longer match loader semantics")
	}
	got := uniquePoseSounds([]float64{0, 1, 2, 0, 1}, 2)
	if len(got) != 2 || got[0] != 0 || got[1] != 1 {
		t.Fatalf("uniquePoseSounds = %v", got)
	}
	if headPrefix("NpcHolder (4)/Girl") != "LeftUp" || headPrefix("PlayerHolder/Girl") != "Player" {
		t.Fatal("head prefix mapping no longer matches serialized npc order")
	}
}

func nodeSet(as *kart.Assets) map[string]bool {
	nodes := map[string]bool{}
	for _, n := range as.Rig.Nodes {
		nodes[n.Path] = true
	}
	return nodes
}

func checkAnimPaths(t *testing.T, root, clip string, a *kmdata.Anim, nodes map[string]bool) {
	t.Helper()
	check := func(rel string) {
		full := root
		if rel != "" {
			if full == "" {
				full = rel
			} else {
				full += "/" + rel
			}
		}
		if !nodes[full] {
			t.Fatalf("clip %s targets missing path %q (root %q rel %q)", clip, full, root, rel)
		}
	}
	for rel := range a.Pos {
		check(rel)
	}
	for rel := range a.Scale {
		check(rel)
	}
	for rel := range a.Euler {
		check(rel)
	}
	for rel := range a.Sprites {
		check(rel)
	}
	for rel := range a.Floats {
		check(rel)
	}
}
