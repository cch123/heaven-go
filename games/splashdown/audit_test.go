package splashdown

import (
	"strings"
	"testing"

	"hsdemo/kart"
)

func loadAssets(t *testing.T) *kart.Assets {
	t.Helper()
	as, err := kart.Load("../../assets/splashdown", 44100)
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
	for _, role := range []string{"synchretteHolder", "synchrettePrefab", "crowdAnim"} {
		if p := as.Roles[role]; p == "" || !nodeSet[p] {
			t.Errorf("role %s = %q 未解析", role, p)
		}
	}
	for _, comp := range []string{"game", "synchrette", "splash"} {
		if as.Extra.Components[comp].Path == "" && comp != "game" {
			t.Fatalf("缺 component %s", comp)
		}
	}
	if kart.NewTemplate(as, as.Roles["synchrettePrefab"]) == nil {
		t.Fatalf("synchrettePrefab 模板不可实例化")
	}
	if kart.NewTemplate(as, as.Extra.Components["synchrette"].Refs["splashPrefab"]) == nil {
		t.Fatalf("splashPrefab 模板不可实例化")
	}
	for _, snd := range []string{
		"whistle", "downOthers", "downPlayer", "upOthers", "upPlayer",
		"yeah", "jumpOthers", "jumpPlayer", "rollOthers", "rollPlayer",
		"splashOthers", "splashPlayer", "together", "togetherRemix9",
		"alleyOop1", "alleyOop2", "alleyOop3", "start", "common_miss",
	} {
		if _, ok := as.Sounds[snd]; !ok {
			t.Errorf("缺音效 %s", snd)
		}
	}
}

func TestControllersResolve(t *testing.T) {
	as := loadAssets(t)
	for _, ctrl := range []string{"Crowd", "Water", "Synchrette ", "Splashes"} {
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

func TestRuntimeRules(t *testing.T) {
	if defaultLength(cueJump) != 2 || defaultLength(cueTogether) != 4 || defaultLength(cueTogetherR9) != 3 || defaultLength(cueIntro) != 8 {
		t.Fatalf("默认事件长度与 Loader 不一致")
	}
	if clampInt(1, 3, 5) != 3 || clampInt(9, 3, 5) != 5 {
		t.Fatalf("Synchrette 数量 clamp 改变")
	}
	m := &Module{cues: []cue{
		{kind: cueDive, beat: 4},
		{kind: cueAppear, beat: 8},
		{kind: cueDive, beat: 12},
	}, gameSwitchBeat: 6}
	if got := m.lastInputCueBefore(11); got != cueAppear {
		t.Fatalf("lastInputCueBefore = %v, want cueAppear", got)
	}
}
