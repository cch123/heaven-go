package spaceball

import (
	"strings"
	"testing"

	"hsdemo/kart"
)

func loadAssets(t *testing.T) *kart.Assets {
	t.Helper()
	as, err := kart.Load("../../assets/spaceball", 44100)
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
	for _, role := range []string{"bg", "square", "room", "hole", "shadow", "shadow2", "Ball", "BallsHolder", "Dispenser", "Dust", "alien"} {
		if p := as.Roles[role]; p == "" || !nodeSet[p] {
			t.Errorf("role %s = %q 未解析", role, p)
		}
	}
	game := as.Extra.Components["game"]
	if len(normalizeBallSprites(game.SpriteArrays["BallSprites"])) != ballStar+1 {
		t.Fatalf("BallSprites 未补齐到官方 6 种")
	}
	for _, comp := range []string{"ball", "player"} {
		if as.Extra.Components[comp].Path == "" {
			t.Fatalf("缺 component %s", comp)
		}
	}
	for _, key := range []string{"ball.pitchLowCurve", "ball.pitchHighCurve", "ball.pitchQuickCurve", "ball.pitchOffbeatCurve"} {
		if c := as.Extra.Curves[key]; len(c.Points) != 2 {
			t.Errorf("曲线 %s 控制点数 = %d, want 2", key, len(c.Points))
		}
	}
	for _, snd := range []string{"shoot", "longShoot", "hit", "fall", "swing", "tacobell", "common_miss"} {
		if _, ok := as.Sounds[snd]; !ok {
			t.Errorf("缺音效 %s", snd)
		}
	}
}

func TestControllersResolve(t *testing.T) {
	as := loadAssets(t)
	for _, ctrl := range []string{"Alien", "Dispenser", "Dust", "Player"} {
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

func TestRuntimeMappings(t *testing.T) {
	sprites := normalizeBallSprites([]string{"", "", "", "", ""})
	for typ, want := range map[int]string{
		ballBaseball: "spaceball_ball",
		ballOnigiri:  "spaceball_riceball",
		ballAlien:    "spaceball_umpire_3",
		ballTacobell: "tacobell",
		ballApple:    "apple",
		ballStar:     "star",
	} {
		if sprites[typ] != want {
			t.Fatalf("ball sprite %d = %q, want %q", typ, sprites[typ], want)
		}
	}
	if ballBeatLength(false, false, false) != 1 || ballBeatLength(false, true, false) != 0.5 ||
		ballBeatLength(false, false, true) != 1.5 || ballBeatLength(true, false, false) != 2 {
		t.Fatalf("Spaceball 判定拍长映射改变")
	}
	if curveKey(true, false, false) != "ball.pitchHighCurve" ||
		curveKey(false, true, false) != "ball.pitchQuickCurve" ||
		curveKey(false, false, true) != "ball.pitchOffbeatCurve" ||
		curveKey(false, false, false) != "ball.pitchLowCurve" {
		t.Fatalf("Spaceball 曲线映射改变")
	}
	if _, offs := hatFrames(2); len(offs) != 5 || offs[0] != [2]float64{0.18, -1.34} {
		t.Fatalf("兔帽帧偏移未按 prefab 保留")
	}
	if p := costumePalette(costumeStandard); p.Fill != [4]float64{0.38823533, 0.9058824, 0, 1} || p.Outline != white {
		t.Fatalf("默认服装 palette = %#v", p)
	}
}
