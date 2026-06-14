package holeinone

import (
	"strings"
	"testing"

	"hsdemo/kart"
	"hsdemo/kmdata"
	"hsdemo/synth"
)

func loadAssets(t *testing.T) *kart.Assets {
	t.Helper()
	as, err := kart.Load("../../assets/holeInOne", synth.SampleRate)
	if err != nil {
		t.Skipf("assets not extracted: %v", err)
	}
	return as
}

func TestHoleInOneBindingsComponentsAndCurves(t *testing.T) {
	as := loadAssets(t)
	for _, role := range []string{
		"baseBall", "MonkeyAnim", "MonkeyHeadAnim", "MandrillAnim", "GolferAnim",
		"Hole", "HoleAnim", "GrassEffectAnim", "BallEffectAnim", "grassEffectPrefab", "grassArea",
	} {
		if as.Roles[role] == "" {
			t.Fatalf("missing role %s", role)
		}
	}
	if kart.NewTemplate(as, as.Roles["baseBall"]) == nil {
		t.Fatalf("missing ball template")
	}
	if kart.NewTemplate(as, as.Roles["grassEffectPrefab"]) == nil {
		t.Fatalf("missing grass effect template")
	}
	ball, ok := as.Extra.Components["ball"]
	if !ok {
		t.Fatalf("missing ball component")
	}
	for _, ref := range []string{"ballSR", "shadowSR", "bigBallSR", "bigShadowSR"} {
		if ball.Refs[ref] == "" {
			t.Fatalf("missing ball ref %s", ref)
		}
	}
	for _, n := range []string{"shadowStartY", "shadowEndY", "floorY", "shadowMinSize"} {
		if _, ok := ball.Nums[n]; !ok {
			t.Fatalf("missing ball number %s", n)
		}
	}
	for _, key := range []string{"ball.curve0", "ball.curve1"} {
		c := as.Extra.Curves[key]
		if len(c.Points) < 2 {
			t.Fatalf("curve %s has %d points", key, len(c.Points))
		}
	}
}

func TestHoleInOneControllersAndSounds(t *testing.T) {
	as := loadAssets(t)
	want := map[string][]string{
		"Monkey":      {"MonkeyIdle", "MonkeyBop", "MonkeyPrepare", "MonkeyThrow", "MonkeySpin"},
		"MonkeyHead":  {"MonkeyHead", "MonkeyJustHead", "MonkeyMissHead", "MonkeySadHead"},
		"Mandrill":    {"MandrillIdle", "MandrillBop", "MandrillBop2", "MandrillBop3", "MandrillReady1", "MandrillReady2", "MandrillReady3", "MandrillPitch"},
		"Golfer":      {"GolferIdle", "GolferBop", "GolferPrepare", "GolferThrough", "GolferJust", "GolferWhiff", "GolferMiss", "GolferThroughMandrill"},
		"GolfHole":    {"ZoomIdle", "ZoomSmall1", "ZoomSmall2", "ZoomSmall3", "ZoomBig"},
		"BallEffect":  {"BallEffect1", "BallEffectJust", "BallEffectThrough"},
		"GrassEffect": {"GrassEffect0", "GrassEffect1", "GrassEffect2", "GrassEffect3"},
	}
	for ctrl, states := range want {
		c, ok := as.Controllers[ctrl]
		if !ok {
			t.Fatalf("missing controller %s", ctrl)
		}
		for _, st := range states {
			cs, ok := c.States[st]
			if !ok {
				t.Errorf("controller %s missing state %s", ctrl, st)
				continue
			}
			if cs.Clip != "" && as.Anims[cs.Clip] == nil {
				t.Errorf("controller %s state %s references missing clip %s", ctrl, st, cs.Clip)
			}
		}
	}
	for _, snd := range []string{
		"SE_GOLF_MONKEY_READY", "SE_GOLF_MONKEY", "SE_GOLF_MONKEY_THROW", "SE_GOLF_AUTO_SWING",
		"SE_GOLF_GORILLA1", "SE_GOLF_GORILLA2", "SE_GOLF_GORILLA3", "SE_GOLF_GORILLA4", "SE_GOLF_GORILLA5",
		"SE_GOLF_SHOT", "SE_GOLF_SHOT_GORILLA_BIG_BALL", "SE_GOLF_CUP_IN", "SE_GOLF_CUP_IN_GORILLA",
		"SE_GOLF_MISS_THROUGH1", "SE_GOLF_MISS_BALL_ATTACK_BIG_GORI", "SE_GOLF_SWING", "sign",
	} {
		if len(as.Sounds[snd]) == 0 {
			t.Fatalf("missing sound %s", snd)
		}
	}
}

func TestHoleInOneClipPathCoverageAndProperties(t *testing.T) {
	as := loadAssets(t)
	scenePaths := map[string]bool{}
	for _, n := range as.Rig.Nodes {
		scenePaths[n.Path] = true
	}
	for root, ctrlName := range as.Animators {
		ctrl := as.Controllers[ctrlName]
		for state, st := range ctrl.States {
			if st.Clip == "" || st.Clip == "None" {
				continue
			}
			checkAnimPaths(t, as.Anims[st.Clip], st.Clip, func(path string) bool {
				if path == "" {
					return scenePaths[root]
				}
				return scenePaths[root+"/"+path]
			})
			if as.Anims[st.Clip] == nil {
				t.Errorf("controller %s state %s missing clip %s", ctrlName, state, st.Clip)
			}
		}
	}
	for clip, anim := range as.Anims {
		for _, attrs := range anim.Floats {
			for attr := range attrs {
				if !supportedFloatAttr(attr) {
					t.Errorf("clip %s uses unsupported float attr %s", clip, attr)
				}
			}
		}
	}
}

func checkAnimPaths(t *testing.T, anim *kmdata.Anim, clip string, okPath func(string) bool) {
	t.Helper()
	if anim == nil {
		t.Errorf("clip %s missing", clip)
		return
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
	for p := range anim.Floats {
		paths[p] = true
	}
	for p := range paths {
		if !okPath(p) {
			t.Errorf("clip %s path %q not found under runtime root", clip, p)
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
