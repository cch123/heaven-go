package flipperflop

import (
	"strings"
	"testing"

	"hsdemo/kart"
)

func loadAssets(t *testing.T) *kart.Assets {
	t.Helper()
	as, err := kart.Load("../../assets/flipperFlop", 44100)
	if err != nil {
		t.Skipf("assets not extracted: %v", err)
	}
	return as
}

func TestExtractedAssets(t *testing.T) {
	as := loadAssets(t)
	nodeSet := map[string]bool{}
	for _, n := range as.Rig.Nodes {
		nodeSet[n.Path] = true
	}
	for _, p := range []string{
		"BG", "Flippers", "Flippers/SnowLeft", "Flippers/SnowRight",
		"Flippers/FlipperHolder/Flipper",
		"Flippers/FlipperHolder (1)/Flipper (1)",
		"Flippers/FlipperHolder (2)/Flipper (2)",
		"Flippers/FlipperHolderPlayer/FlipperPlayer",
		"CaptainTuck", "CaptainTuck/HeadHolder/Head",
	} {
		if !nodeSet[p] {
			t.Errorf("scene path %q missing", p)
		}
	}
	for _, snd := range requiredSounds() {
		if _, ok := as.Sounds[snd]; !ok {
			t.Errorf("missing sound %s", snd)
		}
	}
}

func TestControllersResolve(t *testing.T) {
	as := loadAssets(t)
	for _, ctrl := range []string{"CaptainTuck", "CaptainTuckHead", "Flipper", "FaceFlipper"} {
		if _, ok := as.Controllers[ctrl]; !ok {
			t.Fatalf("missing controller %s", ctrl)
		}
	}
	for name, ctrl := range as.Controllers {
		for st, s := range ctrl.States {
			if s.Clip == "" {
				t.Errorf("controller %s state %s has no clip", name, st)
				continue
			}
			if _, ok := as.Anims[s.Clip]; !ok {
				t.Errorf("controller %s state %s clip %q missing", name, st, s.Clip)
			}
		}
	}
	nodeSet := map[string]bool{}
	for _, n := range as.Rig.Nodes {
		nodeSet[n.Path] = true
	}
	for path := range as.Animators {
		if !nodeSet[path] {
			t.Errorf("animator binding path %q missing in scene", path)
		}
	}
}

func TestAllClipsAccounted(t *testing.T) {
	as := loadAssets(t)
	ctrlClips := map[string]bool{}
	for _, c := range as.Controllers {
		for _, st := range c.States {
			if st.Clip != "" {
				ctrlClips[st.Clip] = true
			}
		}
	}
	for name := range as.Anims {
		if !strings.Contains(name, "/") {
			continue
		}
		if !ctrlClips[name] {
			t.Errorf("clip %q has no controller state", name)
		}
	}
}

func TestClipPathResolution(t *testing.T) {
	as := loadAssets(t)
	nodeSet := map[string]bool{}
	for _, n := range as.Rig.Nodes {
		nodeSet[n.Path] = true
	}
	resolve := func(root, curvePath string) string {
		if curvePath == "" {
			return root
		}
		if root == "" {
			return curvePath
		}
		return root + "/" + curvePath
	}
	for animPath, ctrlName := range as.Animators {
		c := as.Controllers[ctrlName]
		for _, st := range c.States {
			a := as.Anims[st.Clip]
			if a == nil {
				continue
			}
			if st.Clip == "Animations/Test" {
				// The shipped Flipper controller contains an unused editor test
				// state whose curve targets "Face" directly; the runtime prefab
				// hierarchy has FaceHolder/Face and the C# gameplay never plays it.
				continue
			}
			paths := map[string]bool{}
			for p := range a.Pos {
				paths[p] = true
			}
			for p := range a.Euler {
				paths[p] = true
			}
			for p := range a.Scale {
				paths[p] = true
			}
			for p := range a.Sprites {
				paths[p] = true
			}
			for p := range a.Floats {
				paths[p] = true
			}
			for p := range paths {
				if !nodeSet[resolve(animPath, p)] {
					t.Errorf("clip %s path %q under root %q misses scene", st.Clip, p, animPath)
				}
			}
		}
	}
}

func TestFloatAttrsSupported(t *testing.T) {
	as := loadAssets(t)
	supported := map[string]bool{
		"m_IsActive": true, "m_Enabled": true, "m_SortingOrder": true,
		"m_FlipX": true, "m_FlipY": true, "m_Size.x": true, "m_Size.y": true,
		"m_Color.r": true, "m_Color.g": true, "m_Color.b": true, "m_Color.a": true,
	}
	for name, a := range as.Anims {
		for path, attrs := range a.Floats {
			for attr := range attrs {
				if !supported[attr] {
					t.Errorf("clip %s path %q attr %q unsupported", name, path, attr)
				}
			}
		}
	}
}

func TestRuntimeRules(t *testing.T) {
	if attentionEndOffset(false) != 3 || attentionEndOffset(true) != 2 {
		t.Fatalf("attention cue offsets changed")
	}
	if flipState(false, false, true) != "FlopLeft" || flipState(false, true, false) != "ReverseFlopRight" {
		t.Fatalf("flop state naming changed")
	}
	if flipState(true, false, true) != "ReverseRollLeft" || flipState(true, true, false) != "RollRight" {
		t.Fatalf("roll state naming changed")
	}
	if missState(true, true) != "ReverseMissFlopLeft" {
		t.Fatalf("miss state naming changed")
	}
}

func requiredSounds() []string {
	out := []string{
		"ding", "failgroan", "flip1", "flip2", "flipB1", "flipB2", "punch",
		"rollL", "rollR", "tink", "uh1", "uh2", "uh3", "uhfail1", "uhfail2", "uhfail3",
	}
	for _, p := range []string{"good", "goodjob", "nice", "thatsit1", "thatsit2", "welldone", "yes"} {
		out = append(out, "appreciation/"+p)
	}
	for i := 1; i <= 7; i++ {
		out = append(out, "attention/attention"+string(rune('0'+i)))
	}
	for i := 1; i <= 10; i++ {
		out = append(out, "count/flipperRollCount"+itoa(i))
	}
	out = append(out,
		"count/flipperRollCount7B", "count/flipperRollCountA", "count/flipperRollCountB",
		"count/flipperRollCountC", "count/flipperRollCountNow", "count/flipperRollCountS",
	)
	for i := 1; i <= 4; i++ {
		out = append(out, "count/flopCount"+itoa(i))
		out = append(out, "count/flopCount"+itoa(i)+"B")
		out = append(out, "count/flopCountFail"+itoa(i))
	}
	out = append(out, "count/flopCount1C", "count/flopCount2C")
	for i := 1; i <= 3; i++ {
		out = append(out, "count/flopNoise"+itoa(i))
	}
	return out
}

func itoa(v int) string {
	if v == 10 {
		return "10"
	}
	return string(rune('0' + v))
}
