package sickbeats

import (
	"strings"
	"testing"

	"hsdemo/engine"
	"hsdemo/kart"
	"hsdemo/kmdata"
)

func loadAuditAssets(t *testing.T) *kart.Assets {
	t.Helper()
	as, err := kart.Load("../../assets/sickBeats", engine.SampleRate)
	if err != nil {
		t.Fatalf("assets not extracted: %v", err)
	}
	return as
}

func TestSickBeatsTemplatesControllersAndSounds(t *testing.T) {
	as := loadAuditAssets(t)
	if tmpl := kart.NewTemplate(as, "Prefabs/virus"); tmpl == nil {
		t.Fatalf("virus prefab template is not extractable")
	}
	for _, path := range []string{doctorAnim, radioAnim, orgAnim, keyAnim, forkRight, forkUp, forkLeft, forkDown} {
		if _, ok := as.Animators[path]; !ok {
			t.Fatalf("missing animator binding for %s", path)
		}
	}
	for ctrl, states := range map[string][]string{
		"doctor":   {"idle", "bop", "shock0", "shock1", "Vsign"},
		"radio":    {"idle", "bop"},
		"key":      {"idle", "keep", "push", "up"},
		"organism": {"idleAdd", "appear", "bop", "damage", "vanish"},
		"fork":     {"idle", "out", "repop", "resist0", "resist1", "resist2", "resist3", "stab0", "stab1", "stab2", "stab3", "stabFast0", "stabLate3"},
		"virus":    {"idle", "summon", "appear", "appear0", "appear1", "appear2", "appear3", "dash", "dash0", "dash1", "dash2", "dash3", "resist", "resist0", "resist1", "resist2", "resist3", "stab", "stabFast", "stabLate", "enter", "hide", "laugh"},
	} {
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
	for _, snd := range []string{"appear0", "appear1", "dash", "fork0", "fork1", "fork2", "hit", "bad", "resist", "virusIn", "miss", "fadeout"} {
		if len(as.Sounds[snd]) == 0 {
			t.Fatalf("missing sound %s", snd)
		}
	}
}

func TestSickBeatsAnimationPathsResolve(t *testing.T) {
	as := loadAuditAssets(t)
	checkAllAnimatorPaths(t, as)
}

func checkAllAnimatorPaths(t *testing.T, as *kart.Assets) {
	t.Helper()
	nodes := map[string]bool{}
	for _, n := range as.Rig.Nodes {
		nodes[n.Path] = true
	}
	for root, ctrlName := range as.Animators {
		ctrl := as.Controllers[ctrlName]
		for state, st := range ctrl.States {
			if st.Clip == "" {
				continue
			}
			anim, ok := as.Anims[st.Clip]
			if !ok {
				t.Fatalf("%s.%s clip %s missing", ctrlName, state, st.Clip)
			}
			checkAnimPaths(t, nodes, root, st.Clip, anim)
		}
	}
}

func checkAnimPaths(t *testing.T, nodes map[string]bool, root, clip string, anim *kmdata.Anim) {
	t.Helper()
	check := func(path string) {
		full := root
		if path != "" {
			if full == "" {
				full = path
			} else {
				full += "/" + path
			}
		}
		if !nodes[full] {
			t.Fatalf("clip %s path %q resolves to missing node %q", clip, path, full)
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

func TestSickBeatsRuntimeMappings(t *testing.T) {
	if actionForDir(dirRight) != actionRight || actionForDir(dirUp) != actionUp || actionForDir(dirLeft) != actionLeft || actionForDir(dirDown) != actionDown {
		t.Fatalf("direction action mapping changed")
	}
	if p := virusPalette(defaultVirusColors, 0); p.Fill != defaultVirusColors[1] || p.Outline != defaultVirusColors[0] {
		t.Fatalf("life 0 palette mismatch: %#v", p)
	}
	if got := len(dashPatterns[dirDown-1]); got != dirDown {
		t.Fatalf("down dash pattern = %d steps, want %d", got, dirDown)
	}
}

func TestSickBeatsAllExtractedClipsAreCovered(t *testing.T) {
	as := loadAuditAssets(t)
	covered := map[string]bool{}
	for _, ctrl := range as.Controllers {
		for _, st := range ctrl.States {
			if st.Clip != "" {
				covered[st.Clip] = true
				covered[strings.TrimPrefix(st.Clip, "Animations/")] = true
			}
		}
	}
	for clip := range as.Anims {
		if strings.Contains(clip, "/") && !covered[clip] {
			t.Fatalf("extracted clip %s is not referenced by any controller state", clip)
		}
	}
}
