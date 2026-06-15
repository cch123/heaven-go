package djschool

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

func TestBindingsSoundsAndHeadSprites(t *testing.T) {
	as := loadAuditAssets(t)
	nodes := nodeSet(as)
	comp := as.Extra.Components
	if as.Roles["student"] != "Student" || comp["game"].Refs["student"] != "Student" {
		t.Fatalf("student binding roles=%q refs=%q", as.Roles["student"], comp["game"].Refs["student"])
	}
	if as.Roles["djYellow"] != "DJ Yellow" || comp["game"].Refs["djYellow"] != "DJ Yellow" {
		t.Fatalf("djYellow binding roles=%q refs=%q", as.Roles["djYellow"], comp["game"].Refs["djYellow"])
	}
	for _, p := range []string{
		"Student", "Student/Head", "DJ Yellow", "DJ Yellow/Head",
		"TurnTable_Player", "TurnTable_Yellow", "flash", "flashInverse",
	} {
		if !nodes[p] {
			t.Fatalf("expected node %q", p)
		}
	}
	heads := comp["djYellow"].SpriteArrays["djYellowHeadSprites"]
	if len(heads) != 7 {
		t.Fatalf("head sprite count = %d, want 7", len(heads))
	}
	for _, name := range heads {
		if _, ok := as.Sheet.Sprites[name]; !ok {
			t.Fatalf("head sprite %q missing from atlas", name)
		}
	}
	for _, name := range []string{
		"andStop1", "andStop2", "boo", "breakCmon1", "breakCmon2", "breakCmonAlt1", "breakCmonAlt2",
		"breakCmonLoud1", "breakCmonLoud2", "checkItOut1", "checkItOut2", "checkItOut3", "cheer",
		"hey", "heyAlt", "heyLoud", "letsGo1", "letsGo2", "ohYeah1", "ohYeah2", "ohYeahAlt1",
		"ohYeahAlt2", "ohYeahAlt3", "ooh", "oohAlt", "oohLoud", "recordStop", "recordSwipe",
		"scratchoHey1", "scratchoHey2", "scratchoHey3", "scratchoHey4", "scratchoHeyAlt1",
		"scratchoHeyAlt2", "scratchoHeyAlt3", "scratchoHeyAlt4", "scratchoHeyLoud1",
		"scratchoHeyLoud2", "scratchoHeyLoud3", "scratchoHeyLoud4", "yay",
	} {
		if len(as.Sounds[name]) == 0 {
			t.Fatalf("missing sound %s", name)
		}
	}
}

func TestControllersClipsAndAnimationPaths(t *testing.T) {
	as := loadAuditAssets(t)
	for ctrlName, states := range map[string][]string{
		"Student":          {"Idle", "IdleBop", "Hold", "HoldBop", "Swipe", "Unhold"},
		"DJ Yellow":        {"Idle", "IdleBop", "IdleBop2", "BreakCmon", "Hold", "HoldBop", "Scratcho", "Scratcho2", "Hey"},
		"TurnTable_Player": {"Student_Turntable_Idle", "Student_Turntable_StartHold", "Student_Turntable_Hold", "Student_Turntable_Swipe"},
		"TurnTable":        {"DJYellow_Turntable"},
		"flash":            {"Flash"},
		"flashInverse":     {"FlashInverse"},
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
			if st.Clip != "" && as.Anims[st.Clip] == nil {
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
				continue // Student.Scratcho and TurnTable.Student_Turntable_Swipe are empty compatibility states.
			}
			checkAnimPaths(t, root, stName, st.Clip, as.Anims[st.Clip], nodes)
		}
	}
}

func TestAllUnityClipsAccountedAndSpriteSwapsResolve(t *testing.T) {
	as := loadAuditAssets(t)
	ctrlClips := map[string]bool{}
	for _, ctrl := range as.Controllers {
		for _, st := range ctrl.States {
			if st.Clip != "" {
				ctrlClips[st.Clip] = true
			}
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

func TestRuntimeHelpersMatchLoaderSemantics(t *testing.T) {
	ev := &riq.Entity{Data: map[string]any{"toggle2": true}}
	if !boolDefault(ev, "toggle2", false) || !boolDefault(ev, "missing", true) || boolDefault(ev, "missing", false) {
		t.Fatal("bool defaults no longer match loader semantics")
	}
	if breakSounds(voiceCool)[2] != "oohAlt" || scratchSounds(voiceHyped)[4] != "heyLoud" {
		t.Fatal("voice sound tables no longer match DJSchool.cs")
	}
}

func nodeSet(as *kart.Assets) map[string]bool {
	nodes := map[string]bool{}
	for _, n := range as.Rig.Nodes {
		nodes[n.Path] = true
	}
	return nodes
}

func checkAnimPaths(t *testing.T, root, state, clip string, a *kmdata.Anim, nodes map[string]bool) {
	t.Helper()
	if a == nil {
		t.Fatalf("state %s clip %s missing anim", state, clip)
	}
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
			t.Fatalf("state %s clip %s targets missing path %q", state, clip, full)
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
