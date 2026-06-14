package gleeclub

import (
	"math"
	"strings"
	"testing"

	"hsdemo/kart"
)

func loadAssets(t *testing.T) *kart.Assets {
	t.Helper()
	as, err := kart.Load("../../assets/gleeClub", 44100)
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
	for _, role := range []string{"heartAnim", "condAnim", "leftChorusKid", "middleChorusKid", "playerChorusKid"} {
		if p := as.Roles[role]; p == "" || !nodeSet[p] {
			t.Errorf("role %s = %q 未解析", role, p)
		}
	}
	game := as.Extra.Components["game"]
	if game.Refs["kidMaterial"] != "ChorusKidRGB" || game.Refs["bgMaterial"] != "BackgroundRGB" {
		t.Fatalf("materials = kid %q bg %q", game.Refs["kidMaterial"], game.Refs["bgMaterial"])
	}
	wantKids := map[string]string{"kid0": "ChorusKid", "kid1": "ChorusKid (1)", "kid2": "Player"}
	for key, path := range wantKids {
		kid := as.Extra.Components[key]
		if kid.Path != path || kid.Refs["anim"] != path || kid.Refs["sr"] != path+"/Sprite" {
			t.Errorf("%s component = %#v", key, kid)
		}
	}
	for _, snd := range []string{
		"BatonDown", "BatonUp", "LoudWailLoop", "LoudWailStart", "StopWail", "WailLoop",
		"togetherEN-01", "togetherEN-02", "togetherEN-03", "togetherEN-04",
	} {
		if _, ok := as.Sounds[snd]; !ok {
			t.Errorf("缺音效 %s", snd)
		}
	}
}

func TestControllersResolve(t *testing.T) {
	as := loadAssets(t)
	for _, ctrl := range []string{"ChorusKid", "Conductor", "Heart"} {
		if _, ok := as.Controllers[ctrl]; !ok {
			t.Fatalf("缺 controller %s", ctrl)
		}
	}
	for name, ctrl := range as.Controllers {
		for st, s := range ctrl.States {
			if s.Clip == "" {
				t.Errorf("controller %s 状态 %s 无剪辑", name, st)
				continue
			}
			if _, ok := as.Anims[s.Clip]; !ok {
				t.Errorf("controller %s 状态 %s 剪辑 %q 缺失", name, st, s.Clip)
			}
		}
	}
	nodeSet := map[string]bool{}
	for _, n := range as.Rig.Nodes {
		nodeSet[n.Path] = true
	}
	for path := range as.Animators {
		if !nodeSet[path] {
			t.Errorf("animator 绑定路径 %q 不在场景树", path)
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
			t.Errorf("剪辑 %q 无 controller 状态驱动", name)
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
					t.Errorf("剪辑 %s 曲线 path %q（根 %q）在场景树中落空", st.Clip, p, animPath)
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
					t.Errorf("剪辑 %s path %q 属性 %q 不支持", name, path, attr)
				}
			}
		}
	}
}

func TestSpriteSwapsResolve(t *testing.T) {
	as := loadAssets(t)
	for name, a := range as.Anims {
		for path, keys := range a.Sprites {
			for _, k := range keys {
				if k.Name == "" {
					continue
				}
				if _, ok := as.Sheet.Sprites[k.Name]; !ok {
					t.Errorf("剪辑 %s path %q 切片 %q 不在图集", name, path, k.Name)
				}
			}
		}
	}
}

func TestRuntimeSemantics(t *testing.T) {
	if math.Abs(pitchFromSemitones(12)-2) > 1e-9 {
		t.Fatalf("12 semitones pitch = %.6f, want 2", pitchFromSemitones(12))
	}
	if mouthBoth != 0 || mouthOnlyOpen != 1 || mouthOnlyClose != 2 {
		t.Fatalf("MouthOpenClose enum mapping changed")
	}
	ev := colorEase{beat: 4, length: 4, from: defaultFloor, to: defaultWall}
	got := ev.at(6)
	if got[0] <= defaultWall[0] || got[0] >= defaultFloor[0] {
		t.Fatalf("color ease midpoint = %#v", got)
	}
}
