package mrupbeat

import (
	"strings"
	"testing"

	"hsdemo/kart"
)

func loadAssets(t *testing.T) *kart.Assets {
	t.Helper()
	as, err := kart.Load("../../assets/mrUpbeat", 44100)
	if err != nil {
		t.Skipf("assets not extracted: %v", err)
	}
	if err := as.ApplyTexts(); err != nil {
		t.Fatalf("ApplyTexts: %v", err)
	}
	return as
}

func TestPrepareStartsMatchUnityUpdater(t *testing.T) {
	blip, step := prepareStarts(336, 8, false)
	if blip != 336.5 || step != 344 {
		t.Fatalf("legacy offbeat prepare = blip %.1f step %.1f, want 336.5 344", blip, step)
	}
	blip, step = prepareStarts(820, 4, true)
	if blip != 820 || step != 823.5 {
		t.Fatalf("force-onbeat prepare = blip %.1f step %.1f, want 820 823.5", blip, step)
	}
	if got := alignBlipStart(344, 336.5); got != 344.5 {
		t.Fatalf("aligned inactive blip = %.1f, want 344.5", got)
	}
}

func TestCountNamesMatchUnityEnum(t *testing.T) {
	for i, want := range []string{"1", "2", "3", "4", "a"} {
		if got := countName(i); got != want {
			t.Fatalf("countName(%d) = %q, want %q", i, got, want)
		}
	}
}

func TestExtractedAssets(t *testing.T) {
	as := loadAssets(t)
	nodeSet := map[string]bool{}
	for _, n := range as.Rig.Nodes {
		nodeSet[n.Path] = true
	}
	for _, role := range []string{"metronomeAnim", "man", "bg"} {
		if p := as.Roles[role]; p == "" || !nodeSet[p] {
			t.Errorf("role %s = %q 未解析", role, p)
		}
	}
	game := as.Extra.Components["game"]
	man := as.Extra.Components["man"]
	if game.Refs["blipMaterial"] != "blip" {
		t.Fatalf("blip material = %q, want blip", game.Refs["blipMaterial"])
	}
	for _, ref := range []string{"anim", "blipAnim", "antennaLight", "blipText"} {
		if p := man.Refs[ref]; p == "" || !nodeSet[p] {
			t.Errorf("man ref %s = %q 未解析", ref, p)
		}
	}
	if len(man.RefArrays["shadows"]) != 2 {
		t.Fatalf("shadows = %#v, want 2 entries", man.RefArrays["shadows"])
	}
	if len(as.Extra.RefArrays["shadowSr"]) != 3 {
		t.Fatalf("shadowSr = %#v, want 3 renderers", as.Extra.RefArrays["shadowSr"])
	}
	if len(as.Texts) != 1 || as.Texts[0].Path != "Letter" {
		t.Fatalf("texts = %#v, want Letter", as.Texts)
	}
	for _, snd := range []string{
		"1", "2", "3", "4", "a", "blip", "ding", "metronomeLeft", "metronomeRight", "step",
		"common_applause", "common_miss", "common_nearMiss",
	} {
		if _, ok := as.Sounds[snd]; !ok {
			t.Errorf("缺音效 %s", snd)
		}
	}
}

func TestControllersResolve(t *testing.T) {
	as := loadAssets(t)
	for _, ctrl := range []string{"BlipAnimator", "Metronome", "MrUpbeatAnimator"} {
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
