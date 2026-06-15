package powercalligraphy

import (
	"strings"
	"testing"

	"hsdemo/engine"
	"hsdemo/kart"
	"hsdemo/kmdata"
)

func loadAuditAssets(t *testing.T) *kart.Assets {
	t.Helper()
	as, err := kart.Load("../../assets/powerCalligraphy", engine.SampleRate)
	if err != nil {
		t.Fatalf("assets not extracted: %v", err)
	}
	return as
}

func TestPowerCalligraphyRolesTemplatesPatternsAndSounds(t *testing.T) {
	as := loadAuditAssets(t)
	for field, want := range map[string]string{
		"shiftHolder": "shift",
		"paperHolder": "PaperHolder",
		"endPaper":    "PaperHolder/paper",
		"BGPlane":     "BGPlane",
		"fudePosAnim": "shift/fude",
		"fudeAnim":    "shift/fude/sprite",
		"shiftAnim":   "shift",
		"playerFude":  "shift/fude/sprite",
	} {
		if got := as.Roles[field]; got != want {
			t.Fatalf("role %s = %q, want %q", field, got, want)
		}
	}
	roots := as.Extra.RefArrays["basePapers"]
	if len(roots) != charNone {
		t.Fatalf("basePapers = %d, want %d", len(roots), charNone)
	}
	wantLens := map[string]int{
		"paper_re": 5, "paper_comma": 6, "paper_chikara": 8, "paper_onore": 7,
		"paper_sun": 7, "paper_kokoro": 9, "paper_face": 21, "paper_face_kr": 21,
	}
	for typ, root := range roots {
		if tmpl := kart.NewTemplate(as, root); tmpl == nil {
			t.Fatalf("template %s not extractable", root)
		}
		def, ok := loadPaperDef(as, root, typ)
		if !ok {
			t.Fatalf("missing AnimPattern for %s", root)
		}
		if got := len(def.pattern); got != wantLens[root] {
			t.Fatalf("AnimPattern %s = %d items, want %d", root, got, wantLens[root])
		}
		if root == "paper_face" && def.nextBeat != 11 {
			t.Fatalf("paper_face nextBeat = %g, want 11", def.nextBeat)
		}
	}
	for _, snd := range []string{
		"6", "7", "8", "9", "brush1", "brush2", "brush3", "brushTap",
		"comma1", "comma2", "comma3", "reShout",
		"releaseA1", "releaseA2", "releaseB1", "releaseB2",
	} {
		if len(as.Sounds[snd]) == 0 {
			t.Fatalf("missing sound %s", snd)
		}
	}
}

func TestPowerCalligraphyFudeComponent(t *testing.T) {
	as := loadAuditAssets(t)
	fude := as.Extra.Components["fude"]
	for field, want := range map[string]string{
		"handRenderer":  "shift/fude/sprite/arm/hand",
		"thumbRenderer": "shift/fude/sprite/arm/thumb",
		"stickRenderer": "shift/fude/sprite/blush/stick",
		"tipRenderer":   "shift/fude/sprite/blush/tip",
		"ballRenderer":  "shift/fude/sprite/blush/ball",
	} {
		if got := fude.Refs[field]; got != want {
			t.Fatalf("fude ref %s = %q, want %q", field, got, want)
		}
	}
	if fude.Nums["REDRATE_1"] != 0.1 || fude.Nums["REDRATE_2"] != 0.25 {
		t.Fatalf("red thresholds = %v", fude.Nums)
	}
	sprites := map[string]bool{}
	for _, s := range fude.SpriteArrays["sprites"] {
		sprites[s] = true
	}
	for _, s := range []string{"hand_2", "thumb_2", "fude_stick_6_2", "fude_tip_12_2", "fude_ball_2"} {
		if !sprites[s] {
			t.Fatalf("fude sprite list missing %s", s)
		}
	}
}

func TestPowerCalligraphyControllersAndAnimationPaths(t *testing.T) {
	as := loadAuditAssets(t)
	for ctrlName, states := range map[string][]string{
		"fude":         {"fude-none", "fude-prepare", "fude-halt", "fude-sweep", "fude-sweep-end", "fude-tap"},
		"fudePos-re":   {"0", "1", "2", "3just", "3fast", "3late"},
		"paper-re":     {"1", "2", "3just", "3fast", "3late"},
		"shift-re":     {"1", "2", "3just"},
		"paper-face":   {"21just", "21fast", "21late"},
		"fudePos-face": {"21just", "21fast", "21late"},
		"chounin0":     {"dance0", "dance1", "bow0", "bow1", "fall0", "fall1"},
		"chounin1":     {"dance0", "dance1", "bow0", "bow1", "fall0", "fall1"},
		"paper":        {"paper-end"},
		"fudePos":      {"fudePos-end", "fudePos-none"},
	} {
		ctrl, ok := as.Controllers[ctrlName]
		if !ok {
			t.Fatalf("missing controller %s", ctrlName)
		}
		for _, st := range states {
			if _, ok := ctrl.States[st]; !ok {
				t.Fatalf("controller %s missing state %s", ctrlName, st)
			}
		}
	}
	checkControllerClips(t, as)
}

func checkControllerClips(t *testing.T, as *kart.Assets) {
	t.Helper()
	nodes := map[string]bool{}
	for _, n := range as.Rig.Nodes {
		nodes[n.Path] = true
	}
	covered := map[string]bool{}
	for ctrlName, ctrl := range as.Controllers {
		root := rootForController(as, ctrlName)
		if root == "" {
			continue
		}
		if !nodes[root] {
			t.Fatalf("controller %s root %q missing", ctrlName, root)
		}
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
		if !covered[clip] {
			t.Fatalf("clip %s is not driven by any controller state", clip)
		}
	}
}

func rootForController(as *kart.Assets, ctrlName string) string {
	switch {
	case strings.HasPrefix(ctrlName, "paper-"):
		return "paper_" + strings.TrimPrefix(ctrlName, "paper-")
	case strings.HasPrefix(ctrlName, "shift-"), ctrlName == "shift":
		return "shift"
	case strings.HasPrefix(ctrlName, "fudePos"):
		return "shift/fude"
	case ctrlName == "fude":
		return "shift/fude/sprite"
	case ctrlName == "paper":
		return "PaperHolder/paper"
	case ctrlName == "chounin0" || ctrlName == "chounin1":
		for path, c := range as.Animators {
			if c == ctrlName {
				return path
			}
		}
	}
	return ""
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

func checkSupportedAttrs(t *testing.T, clip string, anim *kmdata.Anim) {
	t.Helper()
	for path, attrs := range anim.Floats {
		for attr := range attrs {
			switch attr {
			case "m_Color.r", "m_Color.g", "m_Color.b", "m_Color.a",
				"material._Color.r", "material._Color.g", "material._Color.b", "material._Color.a",
				"material._AddColor.r", "material._AddColor.g", "material._AddColor.b", "material._AddColor.a",
				"m_Size.x", "m_Size.y", "m_FlipX", "m_FlipY",
				"m_SortingOrder", "m_IsActive", "m_Enabled":
			default:
				t.Fatalf("clip %s path %s unsupported float attr %s", clip, path, attr)
			}
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
