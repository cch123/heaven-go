package workingdough

import (
	"strings"
	"testing"

	"hsdemo/engine"
	"hsdemo/kart"
	"hsdemo/kmdata"
)

func loadAuditAssets(t *testing.T) *kart.Assets {
	t.Helper()
	as, err := kart.Load("../../assets/workingDough", engine.SampleRate)
	if err != nil {
		t.Fatalf("assets not extracted: %v", err)
	}
	return as
}

func TestWorkingDoughRolesTemplatesAndSounds(t *testing.T) {
	as := loadAuditAssets(t)
	for field, want := range map[string]string{
		"doughDudesNPC":        "DoughDudesHolder/DoughDudesNPC",
		"doughDudesPlayer":     "DoughDudesHolder/DoughDudesPlayer",
		"smallBallNPC":         "Small_Ball",
		"bigBallNPC":           "Big_Ball",
		"playerEnterSmallBall": "PlayerEnterSmallBall",
		"playerEnterBigBall":   "PlayerEnterBigBall",
		"smallBGBall":          "BGSmallBall",
		"bigBGBall":            "BGBigBall",
		"breakParticleEffect":  "BreakingParticle",
		"arrowSRLeftNPC":       "NPCBallTransporters/BallTransporterLeftNPC/Arrow/Arrow_Fill",
		"arrowSRRightPlayer":   "PlayerBallTransporters/BallTransporterRightPlayer/Arrow/Arrow_Fill",
		"backgroundSR":         "Background",
		"flashSR":              "Flash",
		"spaceshipAnimator":    "SpaceshipParts",
		"doughDudesHolderAnim": "DoughDudesHolder",
		"gandwAnim":            "MrGameAndWatch",
	} {
		if got := as.Roles[field]; got != want {
			t.Fatalf("role %s = %q, want %q", field, got, want)
		}
	}
	for _, field := range []string{
		"smallBallNPC", "bigBallNPC", "playerEnterSmallBall", "playerEnterBigBall",
		"smallBGBall", "bigBGBall", "breakParticleEffect",
	} {
		if tmpl := kart.NewTemplate(as, as.Roles[field]); tmpl == nil {
			t.Fatalf("template %s at %q not extractable", field, as.Roles[field])
		}
	}
	for _, snd := range []string{
		"bigPlayer", "hitBigPlayer", "hitSmallOther", "tooBig", "BallMiss",
		"tooSmallAr", "hitSmallPlayer", "smallOther", "tooSmall",
		"smallPlayer", "bigOther", "hitBigOther", "LaunchRobot", "common_miss",
	} {
		if len(as.Sounds[snd]) == 0 {
			t.Fatalf("missing sound %s", snd)
		}
	}
}

func TestWorkingDoughCurvesAndScriptRefs(t *testing.T) {
	as := loadAuditAssets(t)
	game := as.Extra.Components["game"]
	if got := game.Sprites["whiteArrowSprite"]; got != "WhiteArrow" {
		t.Fatalf("white arrow sprite = %q", got)
	}
	if got := game.Sprites["redArrowSprite"]; got != "RedArrow" {
		t.Fatalf("red arrow sprite = %q", got)
	}
	if got := game.RefArrays["bgObjects"]; len(got) != 2 || got[0] != "Background/Disable" || got[1] != "MrGameAndWatch" {
		t.Fatalf("bgObjects = %#v", got)
	}
	paths := loadBouncePaths(as)
	for _, name := range []string{"NPCBall", "BGBall", "PlayerEnter", "PlayerHit", "PlayerBarely", "PlayerWeak", "PlayerMiss"} {
		if len(paths[name].points) == 0 {
			t.Fatalf("missing ballBouncePath %s", name)
		}
	}
	if got := paths["NPCBall"].eval(1); got != nodePos(as, "BallHolder/CurvePoints/NPCBallHit") {
		t.Fatalf("NPCBall at 1 = %#v, want hit point", got)
	}
	mid := paths["PlayerEnter"].eval(0.5)
	if mid[1] <= nodePos(as, "BallHolder/CurvePoints/PlayerBallHit")[1] {
		t.Fatalf("PlayerEnter midpoint y = %g, expected arced path above hit point", mid[1])
	}
	if got := paths["BGBall"].points[0].duration; got != 8 {
		t.Fatalf("BGBall first segment duration = %g, want 8", got)
	}
}

func TestWorkingDoughControllersAndAnimationPaths(t *testing.T) {
	as := loadAuditAssets(t)
	wantStates := map[string][]string{
		"ConveyerBelt":           {"ConveyerBelt"},
		"DoughDudesHolder":       {"OnGround", "InAir", "LiftUp", "LiftDown"},
		"DoughDudes":             {"SmallDoughJump", "BigDoughJump"},
		"NPCBallTransporters":    {"NPCGoBigMode", "NPCExitBigMode"},
		"PlayerBallTransporters": {"PlayerGoBigMode", "PlayerExitBigMode"},
		"BallTransporterLeft":    {"BallTransporterLeftOpen", "BallTransporterLeftClose", "BallTransporterLeftOpened", "BallTransporterLeftClosed"},
		"BallTransporterRight":   {"BallTransporterRightOpen", "BallTransporterRightClose", "BallTransporterRightOpened", "BallTransporterRightClosed"},
		"SpaceshipParts":         {"RiseSpaceship", "Risen", "AbsorbBall", "SpaceshipShake", "SpaceshipLaunch", "SpaceshipLaunched"},
		"Lights":                 {"SpaceshipLights"},
		"MrGameAndWatch":         {"GANDWEnter", "GANDWLeave", "GANDWLeft", "GANDWLeverUp", "MrGameAndWatchLeverDown"},
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
