package ringside

import (
	"strings"
	"testing"

	"hsdemo/kart"
)

func loadAssets(t *testing.T) *kart.Assets {
	t.Helper()
	as, err := kart.Load("../../assets/ringside", 44100)
	if err != nil {
		t.Skipf("assets not extracted: %v", err)
	}
	return as
}

func TestRingsideSceneRoots(t *testing.T) {
	as := loadAssets(t)
	nodeSet := map[string]bool{}
	for _, n := range as.Rig.Nodes {
		nodeSet[n.Path] = true
	}
	for _, p := range []string{
		"BG2", "Reporter", "Reporter/Upper/HeadAnim", "Wrestler",
		"Audience", "Flash", "FlashWhite", "BlackVoid", "PoseFlash",
		"FlashesParticle", "Newspaper", "Newspaper/WrestlerNewspaper",
		"Newspaper/ReporterNewspaper",
	} {
		if !nodeSet[p] {
			t.Errorf("scene path %q missing", p)
		}
	}
}

func TestRingsideControllersResolve(t *testing.T) {
	as := loadAssets(t)
	for _, ctrl := range []string{
		"Audience", "Newspaper", "PoseFlash", "Reporter", "Head 1",
		"Wrestler", "ReporterNewspaper", "WrestlerNewspaper",
	} {
		if _, ok := as.Controllers[ctrl]; !ok {
			t.Fatalf("missing controller %s", ctrl)
		}
	}
	for name, ctrl := range as.Controllers {
		for st, s := range ctrl.States {
			if s.Clip == "" || s.Clip == "None" {
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

func TestRingsideScriptedStatesAndSounds(t *testing.T) {
	as := loadAssets(t)
	for _, want := range []struct {
		ctrl, state string
	}{
		{"Wrestler", "Bop"}, {"Wrestler", "BopPec"}, {"Wrestler", "Ye"},
		{"Wrestler", "YeMiss"}, {"Wrestler", "Cough"}, {"Wrestler", "BigGuyOne"},
		{"Wrestler", "BigGuyTwo"}, {"Wrestler", "PreparePose"}, {"Wrestler", "Pose1"},
		{"Audience", "PoseAudience"}, {"Audience", "PoseAudienceZoomed"},
		{"Reporter", "WubbaLubbaDubbaThatTrue"}, {"Reporter", "ThatTrue"},
		{"Reporter", "Woah"}, {"Reporter", "true"}, {"Reporter", "HeartReporter"},
		{"Reporter", "ExcitedReporter"}, {"Reporter", "FlinchReporter"},
		{"Head 1", "Wubba"}, {"Head 1", "IsThat"}, {"Head 1", "Guy"},
		{"Head 1", "ExtendSmile"}, {"Head 1", "Late"}, {"Head 1", "Heart"},
		{"Head 1", "Excited"}, {"PoseFlash", "PoseFlashing"},
		{"Newspaper", "NewspaperEnter"}, {"Newspaper", "NewspaperEnterRight"},
		{"ReporterNewspaper", "HeartReporterNewspaper"},
		{"ReporterNewspaper", "IdleReporterNewspaper"},
		{"WrestlerNewspaper", "Pose1Newspaper"}, {"WrestlerNewspaper", "Miss1Newspaper"},
	} {
		ctrl, ok := as.Controllers[want.ctrl]
		if !ok {
			t.Fatalf("missing controller %s", want.ctrl)
		}
		st, ok := ctrl.States[want.state]
		if !ok {
			t.Errorf("controller %s missing state %s", want.ctrl, want.state)
			continue
		}
		if st.Clip != "" && st.Clip != "None" && as.Anims[st.Clip] == nil {
			t.Errorf("state %s/%s clip %q missing", want.ctrl, want.state, st.Clip)
		}
	}
	for _, snd := range []string{
		"en/wubdub_var1_1", "en/wubdub_var1_7", "en/wubdub_var3_8",
		"en/bigguy_var1_1", "en/bigguy_var2_5",
		"en/pose_and", "en/pose_var1_1", "en/pose_var2_6",
		"muscles1", "muscles2", "ye1", "yell1", "poseCamera",
		"kidslaugh", "huhaudience0", "huhaudience1", "badpose_1",
		"wubdub_konk_1", "mic_swoosh", "confusedanswer", "barely",
	} {
		if len(as.Sounds[snd]) == 0 {
			t.Errorf("missing sound %q", snd)
		}
	}
}

func TestRingsideAllNamespacedClipsAccounted(t *testing.T) {
	as := loadAssets(t)
	ctrlClips := map[string]bool{}
	for _, c := range as.Controllers {
		for _, st := range c.States {
			if st.Clip != "" && st.Clip != "None" {
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

func TestRingsideClipPathResolution(t *testing.T) {
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
		ctrl := as.Controllers[ctrlName]
		for _, st := range ctrl.States {
			a := as.Anims[st.Clip]
			if a == nil {
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

func TestRingsideFloatAttrsSupported(t *testing.T) {
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

func TestRingsideSpriteSwapsResolve(t *testing.T) {
	as := loadAssets(t)
	for name, a := range as.Anims {
		for path, keys := range a.Sprites {
			for _, k := range keys {
				if k.Name == "" {
					continue
				}
				if _, ok := as.Sheet.Sprites[k.Name]; !ok {
					t.Errorf("clip %s path %q sprite %q missing", name, path, k.Name)
				}
			}
		}
	}
}
