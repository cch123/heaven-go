package totemclimb

// 交付前强制审计（heaven-go 移植规范）：剪辑驱动完整性、曲线 path 解析、
// float 属性支持面、sprite 换帧解析、组件引用绑定。资产未提取时跳过。

import (
	"strings"
	"testing"

	"hsdemo/kart"
)

func loadAssets(t *testing.T) *kart.Assets {
	t.Helper()
	as, err := kart.Load("../../assets/totemClimb", 44100)
	if err != nil {
		t.Skipf("assets not extracted: %v", err)
	}
	return as
}

// 不被 controller 引用且模块不显式驱动的剪辑（核对过原因）。
// 音效盘点备注：Sounds/chargejump.wav 无任何 C# 引用（死资产，不导出使用）。
var undrivenClips = map[string]string{
	"EndTotem/OpenWingsIdle": "OpenWings 的退出转换目标（controller 引用，经状态机到达）",
}

// 模块显式驱动（不经 controller 默认路径）的剪辑：无（totemClimb 全部状态
// 经 PlayState/状态机播放）。

// TestAllClipsAccounted：26 个剪辑要么被 controller 状态引用，要么在白名单。
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
			continue // 裸名别名
		}
		if !ctrlClips[name] {
			if _, ok := undrivenClips[name]; !ok {
				t.Errorf("剪辑 %q 无驱动路径且不在白名单", name)
			}
		}
	}
}

// TestClipPathResolution：controller 引用的剪辑曲线 path 对场景树解析零落空。
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
			if st.Clip == "" {
				continue
			}
			a := as.Anims[st.Clip]
			if a == nil {
				t.Errorf("剪辑 %q 缺失", st.Clip)
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
					t.Errorf("剪辑 %s 曲线 path %q（根 %q）落空", st.Clip, p, animPath)
				}
			}
		}
	}
}

// TestFloatAttrsSupported：float 曲线属性全部在运行时支持集合内。
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

// TestSpriteSwapsResolve：sprite 换帧关键帧全部解析（空名=隐藏，合法）。
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

// TestComponentsAndTemplates：组件 dump 与模板锚点全部就位。
func TestComponentsAndTemplates(t *testing.T) {
	as := loadAssets(t)
	comp := as.Extra.Components
	for _, name := range []string{"game", "jumper", "totemManager", "birdManager",
		"groundManager", "pillarManager", "backgroundManager", "totem", "dragon", "frog"} {
		if _, ok := comp[name]; !ok {
			t.Fatalf("组件 %s 未导出", name)
		}
	}
	if comp["game"].Nums["_scrollSpeedX"] != 3.838 || comp["totemManager"].Nums["_yDistance"] != 1.45 {
		t.Errorf("滚动参数异常: %v", comp["game"].Nums)
	}
	if comp["jumper"].Nums["_jumpHighHeight"] != 4 {
		t.Errorf("_jumpHighHeight = %v, want 4（序列化值，非 C# 字段默认 6）",
			comp["jumper"].Nums["_jumpHighHeight"])
	}
	if comp["birdManager"].Sprites["_penguinSprite"] != "penguin" {
		t.Errorf("penguin 切片未解析: %v", comp["birdManager"].Sprites)
	}
	if got := len(comp["backgroundManager"].Lists["_objects"]); got != 5 {
		t.Errorf("背景滚动对 = %d, want 5", got)
	}
	// 模板与锚点
	for tmplPath, anchors := range map[string][]string{
		comp["totemManager"].Refs["_totemTransform"]:    {"JumperPoint"},
		comp["totemManager"].Refs["_frogTransform"]:     {"JumperPointLeft", "JumperPointMiddle", "JumperPointRight"},
		comp["totemManager"].Refs["_dragon"]:            {"JumperPoint"},
		comp["totemManager"].Refs["_endTotemTransform"]: {"Holder/jumperEnd", "FeatherEffect", "FeatherEffect (1)"},
		comp["birdManager"].Refs["_birdRef"]:            {"BirdRef (1)", "BirdRef (2)"},
	} {
		tm := kart.NewTemplate(as, tmplPath)
		if tm == nil {
			t.Fatalf("模板 %q 不存在", tmplPath)
		}
		in := tm.NewInstance()
		for _, a := range anchors {
			if _, ok := in.NodeWorld(a, kart.Identity()); !ok {
				t.Errorf("模板 %s 锚点 %q 缺失", tmplPath, a)
			}
		}
	}
	// 粒子贴图
	for _, s := range []string{"star", "swirl", "totemclimb_2"} {
		if _, ok := as.Sheet.Sprites[s]; !ok {
			t.Errorf("粒子切片 %s 不在图集", s)
		}
	}
}
