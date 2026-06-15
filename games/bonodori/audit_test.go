package bonodori

import (
	"path/filepath"
	"strings"
	"testing"

	"hsdemo/engine"
	"hsdemo/kart"
	"hsdemo/kmdata"
)

func loadAuditAssets(t *testing.T) *kart.Assets {
	t.Helper()
	as, err := kart.Load(filepath.Join("..", "..", "assets", "bonOdori"), engine.SampleRate)
	if err != nil {
		t.Fatal(err)
	}
	if err := as.ApplyTexts(); err != nil {
		t.Fatalf("ApplyTexts: %v", err)
	}
	return as
}

func TestBindingsTextAndSounds(t *testing.T) {
	as := loadAuditAssets(t)
	wantRoles := map[string]string{
		"darkPlane": "Square",
		"Judge":     "Judge",
		"JudgeFace": "Judge/Head/Face",
	}
	for role, want := range wantRoles {
		if got := as.Roles[role]; got != want {
			t.Fatalf("role %s = %q, want %q", role, got, want)
		}
	}
	if got := as.Extra.RefArrays["Donpans"]; strings.Join(got, ",") != "Player,CPU1,CPU2,CPU3" {
		t.Fatalf("Donpans = %#v", got)
	}
	for _, key := range []string{"Texts", "TextsBlue"} {
		if got := as.Extra.RefArrays[key]; len(got) != 5 {
			t.Fatalf("%s refs = %d, want 5", key, len(got))
		}
	}
	if len(as.Texts) != 10 {
		t.Fatalf("texts = %d, want 10", len(as.Texts))
	}
	for _, tn := range as.Texts {
		if !strings.HasPrefix(tn.Path, "Line/Line") {
			t.Fatalf("unexpected text path %q", tn.Path)
		}
		if tn.HAlign != 1 || tn.VAlign != 512 {
			t.Fatalf("text %s alignment = (%d,%d), want left/middle", tn.Path, tn.HAlign, tn.VAlign)
		}
	}
	for _, snd := range []string{
		"pan1", "pan2", "pan3", "pa1", "pa_n1", "pa_n2",
		"don1", "don2", "don3", "don4", "do1", "do2", "do_n1", "do_n2",
		"clap", "clap2", "common_nearMiss",
	} {
		if len(as.Sounds[snd]) == 0 {
			t.Fatalf("missing sound %s", snd)
		}
	}
}

func TestControllersClipsAndAnimationPaths(t *testing.T) {
	as := loadAuditAssets(t)
	wantStates := map[string][]string{
		"Player":     {"Bop", "Bow", "ClapFront", "ClapSide", "NeutralBopped", "NeutralClapFront"},
		"CPU":        {"Bop", "BopHappy", "Bow", "ClapFront", "ClapSide", "NeutralBopped", "NeutralClapFront"},
		"Judge":      {"Bop", "Idle"},
		"JudgeFace":  {"Happy", "Neutral", "Sad"},
		"CPU1Face":   {"Annoyed", "CPU1Neut"},
		"CPU2Face":   {"Annoyed", "CPU2Neut"},
		"CPU3Face":   {"Annoyed", "CPU3Neut"},
		"PlayerFace": {"PlayerNeut"},
		"DarkPlane":  {"Appear", "GoAway", "StayOff", "StayOn"},
	}
	for ctrlName, states := range wantStates {
		ctrl, ok := as.Controllers[ctrlName]
		if !ok {
			t.Fatalf("missing controller %s", ctrlName)
		}
		for _, state := range states {
			st, ok := ctrl.States[state]
			if !ok {
				t.Fatalf("controller %s missing state %s", ctrlName, state)
			}
			if st.Clip != "" && as.Anims[st.Clip] == nil {
				t.Fatalf("controller %s state %s clip %q missing", ctrlName, state, st.Clip)
			}
		}
	}
	for _, clip := range []string{
		"Players/Bop", "Players/BopHappy", "Players/Bow", "Players/ClapFront", "Players/ClapSide",
		"Players/Idle", "Players/NeutralBopped", "Players/NeutralClapSide", "Players/NeutralUnbopped",
		"Judge/Bop", "Judge/Idle",
		"Face/Happy", "Face/Neutral", "Face/Sad",
		"Expressions/Annoyed", "Expressions/CPU1Neut", "Expressions/CPU2Neut", "Expressions/CPU3Neut", "Expressions/PlayerNeut",
		"DarkPlane/Appear", "DarkPlane/GoAway", "DarkPlane/StayOff", "DarkPlane/StayOn",
	} {
		if as.Anims[clip] == nil {
			t.Fatalf("missing clip %s", clip)
		}
	}
	nodes := map[string]bool{}
	for _, n := range as.Rig.Nodes {
		nodes[n.Path] = true
	}
	for root, ctrlName := range as.Animators {
		ctrl := as.Controllers[ctrlName]
		for stateName, st := range ctrl.States {
			if st.Clip == "" {
				continue
			}
			checkAnimPaths(t, root, ctrlName, stateName, st.Clip, as.Anims[st.Clip], nodes)
		}
	}
}

func TestRuntimeHelpersMatchScriptSemantics(t *testing.T) {
	m := &Module{bops: []bopEvt{{beat: 4, auto: true}, {beat: 8, auto: false}}}
	if m.autoBopAt(3.99) || !m.autoBopAt(4) || m.autoBopAt(8) {
		t.Fatalf("autoBopAt did not follow SetupBopRegion semantics")
	}
	if got := cleanLyric("r|PANd|PAN|s"); got != "PANPAN" {
		t.Fatalf("cleanLyric = %q", got)
	}
	line := parseLyric("r|PANd|s|PA|sd|g|N", false)
	if line.plain != "PANPAN" {
		t.Fatalf("parseLyric plain = %q", line.plain)
	}
	if len(line.breaks) != 2 || line.breaks[0] != 3 || line.breaks[1] != 5 {
		t.Fatalf("parseLyric breaks = %#v", line.breaks)
	}
	if len(line.runs) != 3 {
		t.Fatalf("parseLyric runs = %#v", line.runs)
	}
	if line.runs[0].Color != [4]float64{1, 0, 0, 1} || line.runs[1].Scale != 0.9375 || line.runs[2].Color != [4]float64{0, 1, 0, 1} {
		t.Fatalf("parseLyric runs lost color/scale markers: %#v", line.runs)
	}
	blue := parseLyric("r|PAN", true)
	if len(blue.runs) != 1 || blue.runs[0].Color != [4]float64{1, 0, 1, 1} {
		t.Fatalf("parseLyric blue marker = %#v", blue.runs)
	}
	if got := speakClip("pan", panTypePanHold, 1); got != "pa_n2" {
		t.Fatalf("speakClip pan hold = %q", got)
	}
}

func checkAnimPaths(t *testing.T, root, ctrl, state, clip string, anim *kmdata.Anim, nodes map[string]bool) {
	t.Helper()
	if anim == nil {
		t.Fatalf("clip %s missing", clip)
	}
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
	for p, attrs := range anim.Floats {
		paths[p] = true
		for attr := range attrs {
			if !supportedFloatAttr(attr) {
				t.Fatalf("clip %s uses unsupported attr %s", clip, attr)
			}
		}
	}
	for p := range paths {
		full := root
		if p != "" {
			full += "/" + p
		}
		if !nodes[full] {
			t.Fatalf("controller %s state %s clip %s path %q missing under %q", ctrl, state, clip, p, root)
		}
	}
}

func supportedFloatAttr(attr string) bool {
	switch attr {
	case "m_IsActive", "m_Enabled", "m_FlipX", "m_FlipY", "m_SortingOrder", "m_Size.x", "m_Size.y":
		return true
	}
	return strings.HasPrefix(attr, "m_Color.") || strings.HasPrefix(attr, "m_fontColor.")
}
