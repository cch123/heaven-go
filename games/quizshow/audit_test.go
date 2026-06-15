package quizshow

import (
	"math/rand"
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
	as, err := kart.Load(filepath.Join("..", "..", "assets", "quizShow"), engine.SampleRate)
	if err != nil {
		t.Fatal(err)
	}
	return as
}

func TestBindingsSpritesAndSounds(t *testing.T) {
	as := loadAuditAssets(t)
	wantRoles := map[string]string{
		"contesteeLeftArmAnim":  "Contestee/LeftArm",
		"contesteeRightArmAnim": "Contestee/RightArm",
		"contesteeHead":         "Contestee/Head",
		"hostLeftArmAnim":       "QuizHost/LeftArm",
		"hostRightArmAnim":      "QuizHost/RightArm",
		"hostHead":              "QuizHost/Head",
		"signAnim":              "SignHolder",
		"timerTransform":        "Timer/Stopwatch/Needle",
		"stopWatchRef":          "Timer",
		"blackOut":              "Blackout",
		"firstDigitSr":          "Contestee/Counter/FirstDigit",
		"secondDigitSr":         "Contestee/Counter/SecondDigit",
		"hostFirstDigitSr":      "QuizHost/Counter/FirstDigit",
		"hostSecondDigitSr":     "QuizHost/Counter/SecondDigit",
		"contCounter":           "Contestee/Counter/Sprite",
		"hostCounter":           "QuizHost/Counter/Sprite",
		"contExplosion":         "Contestee/Counter/Explosion",
		"hostExplosion":         "QuizHost/Counter/Explosion",
		"signExplosion":         "SignHolder/Explosion",
	}
	for role, want := range wantRoles {
		if got := as.Roles[role]; got != want {
			t.Fatalf("role %s = %q, want %q", role, got, want)
		}
	}

	comp := as.Extra.Components["game"]
	if got := comp.SpriteArrays["contestantNumberSprites"]; len(got) != 10 {
		t.Fatalf("contestant digits = %v, want 10 sprites", got)
	}
	if got := comp.SpriteArrays["hostNumberSprites"]; len(got) != 11 || got[10] != "QuestionHost" {
		t.Fatalf("host digits = %v, want 0-9 plus QuestionHost", got)
	}
	if comp.Sprites["explodedCounter"] != "CounterGrey" {
		t.Fatalf("explodedCounter = %q, want CounterGrey", comp.Sprites["explodedCounter"])
	}
	for _, sprites := range [][]string{comp.SpriteArrays["contestantNumberSprites"], comp.SpriteArrays["hostNumberSprites"]} {
		for _, sprite := range sprites {
			if _, ok := as.Sheet.Sprites[sprite]; !ok {
				t.Fatalf("digit sprite %q missing from atlas", sprite)
			}
		}
	}
	if _, ok := as.Sheet.Sprites[comp.Sprites["explodedCounter"]]; !ok {
		t.Fatalf("explodedCounter sprite %q missing from atlas", comp.Sprites["explodedCounter"])
	}

	for _, snd := range []string{
		"answerReveal", "audienceCheer", "audienceSad", "contestantA", "contestantDPad",
		"contestantExplode", "correct", "correctJingle", "correctNoApplause", "correctNoMusic",
		"hostA", "hostDPad", "hostExplode", "incorrect", "incorrectJingle", "signExplode",
		"timeUp", "timerStart", "timerStop",
	} {
		if len(as.Sounds[snd]) == 0 {
			t.Fatalf("missing sound %s", snd)
		}
	}
}

func TestControllersClipsAndAnimationPaths(t *testing.T) {
	as := loadAuditAssets(t)
	wantStates := map[string][]string{
		"Head":       {"ContesteeHeadIdle", "ContesteeHeadStage1", "ContesteeHeadStage2", "ContesteeHeadStage3", "ContesteeSad", "ContesteeSmile"},
		"HostHead":   {"HostIdleHead", "HostSad", "HostSmile", "HostStage1", "HostStage2", "HostStage3", "HostStage4"},
		"LeftArm":    {"LeftArmIdle", "LeftArmPress", "LeftPrepare", "LeftPrepareIdle", "LeftRest"},
		"LeftArm 1":  {"HostLeftArmIdle", "HostLeftHit", "HostLeftPrepare", "HostLeftRest"},
		"RightArm":   {"RIghtPrepare", "RightArmHit", "RightArmIdle", "RightPrepareIdle", "RightRest"},
		"RightArm 1": {"HostPrepare", "HostRightArmIdle", "HostRightHit", "HostRightRest"},
		"SignHolder": {"Exploded", "SignIdle"},
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
			if st.Clip == "" || as.Anims[st.Clip] == nil {
				t.Fatalf("controller %s state %s clip %q missing", ctrlName, state, st.Clip)
			}
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

func TestAllUnityClipsAccountedAndSpriteSwapsResolve(t *testing.T) {
	as := loadAuditAssets(t)
	ctrlClips := map[string]bool{}
	for _, c := range as.Controllers {
		for _, st := range c.States {
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

func TestRuntimeHelpersMatchScriptSemantics(t *testing.T) {
	if specificDigit(42, 1) != 2 || specificDigit(42, 2) != 4 || specificDigit(7, 2) != 0 {
		t.Fatalf("specificDigit must match QuizShow.GetSpecificDigit")
	}

	m := &Module{
		rng:     rand.New(rand.NewSource(1)),
		randoms: []randomEvt{{beat: 4, length: 0.5, min: 3, max: 3, which: buttonAlternatingDpad, consecutive: true}},
	}
	m.expandRandomPresses()
	if len(m.presses) != 3 {
		t.Fatalf("expanded presses = %d, want 3", len(m.presses))
	}
	for i, p := range m.presses {
		wantBeat := 4 + float64(i)*0.5
		wantDpad := i%2 == 0
		if p.beat != wantBeat || p.dpad != wantDpad {
			t.Fatalf("press %d = %+v, want beat %.1f dpad %v", i, p, wantBeat, wantDpad)
		}
	}

	ev := &riq.Entity{Data: map[string]any{"auto": true, "audio": float64(clockEnd)}}
	if !boolDefault(ev, "auto", false) || boolDefault(ev, "missing", true) != true || intParam(ev, "audio", clockBoth) != clockEnd {
		t.Fatalf("parameter helpers no longer match RIQ boolean/enum semantics")
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
