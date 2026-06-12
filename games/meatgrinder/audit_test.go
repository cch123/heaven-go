package meatgrinder

// 交付前强制审计（heaven-go 移植规范）：
// 剪辑驱动完整性、曲线 path 解析、float 属性支持面、sprite 换帧解析、
// 角色/引用绑定。资产未提取时跳过（先 go run ./cmd/extract -game meatGrinder）。

import (
	"strings"
	"testing"

	"hsdemo/kart"
)

func loadAssets(t *testing.T) *kart.Assets {
	t.Helper()
	as, err := kart.Load("../../assets/meatGrinder", 44100)
	if err != nil {
		t.Skipf("assets not extracted: %v", err)
	}
	return as
}

// 不被任何代码路径驱动的剪辑（逐一核对过原因）：
//   - GearSpin*：Gear.controller 的状态从未被触发——C# 在 Update 里直接
//     transform.Rotate 旋转齿轮（本移植同式），controller 默认态 Nothing
//   - Meat/MeatThrown：MeatAnim.controller 无对应状态、C# 无引用（legacy）
//   - Meat/MeatIdle：默认态 speed=0 的空剪辑（肉块外观由 sr.sprite 静态赋值）
//   - Tack/TackHitBarelyIdle：空剪辑（0 曲线）；controller 里的 TackBarelyIdle
//     状态引用的是一个已删除的 guid，且无转换可达（prefab 死数据）
//   - Boss/BossIdleMiss：无转换可达、C# 不按名播放（prefab 死数据）
//   - Cart Guy/CartguyIdle：CartGuy.controller 默认态 Idle 无 motion，
//     此空剪辑（0 曲线）不挂在任何状态上
var undrivenClips = map[string]string{
	"GearSpinOnceRight":      "gear controller 状态未被触发（脚本直接旋转）",
	"GearSpinOnceLeft":       "同上",
	"GearSpinRight":          "同上",
	"GearSpinLeft":           "同上",
	"Meat/MeatThrown":        "controller 无状态、C# 无引用（legacy）",
	"Meat/MeatIdle":          "默认态 speed=0 空剪辑",
	"Tack/TackHitBarelyIdle": "空剪辑；对应状态引用已删除 guid 且不可达",
	"Boss/BossIdleMiss":      "不可达状态（prefab 死数据）",
	"Cart Guy/CartguyIdle":   "默认态无 motion 的空剪辑",
}

// moduleDrivenClips：不经 controller、由模块代码显式驱动的剪辑。
var moduleDrivenClips = map[string]bool{
	"Meat/DarkMeatHit":          true, // SampleClipNode（多实例）
	"Meat/LightMeatHit":         true,
	"Meat/BaconBallHit":         true,
	"Cart Guy/CartguyMoveRight": true, // PlayNormalized
	"Cart Guy/CartguyMoveLeft":  true,
}

// TestAllClipsAccounted：每个 .anim 要么被 controller 状态引用，
// 要么被模块显式驱动，要么在"未驱动白名单"里给出原因。
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
			// 裸名 key 是命名空间 key 的别名（仅 basename 全局唯一时导出）
			continue
		}
		if !ctrlClips[name] && !moduleDrivenClips[name] {
			if _, ok := undrivenClips[name]; !ok {
				t.Errorf("剪辑 %q 无驱动路径且不在白名单", name)
			}
		}
	}
	// 白名单不能漂移：列出的剪辑必须真实存在（裸名条目在 anims 里也有 ns 形式）
	for name := range undrivenClips {
		if _, ok := as.Anims[name]; !ok {
			t.Errorf("白名单剪辑 %q 不存在（白名单过期）", name)
		}
	}
}

// TestClipPathResolution：controller 引用的每个剪辑 × 每条曲线 path，
// 从 Animator 根拼出全路径对场景树验证，零落空。
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
	check := func(clipName, root string) {
		a := as.Anims[clipName]
		if a == nil {
			t.Errorf("剪辑 %q 缺失", clipName)
			return
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
			if !nodeSet[resolve(root, p)] {
				t.Errorf("剪辑 %s 曲线 path %q（根 %q）在场景树中落空", clipName, p, root)
			}
		}
	}
	for animPath, ctrlName := range as.Animators {
		c := as.Controllers[ctrlName]
		for stName, st := range c.States {
			if st.Clip == "" {
				continue
			}
			_ = stName
			check(st.Clip, animPath)
		}
	}
	// 模块显式驱动的肉块剪辑：曲线只允许在根（path ""），模块按根采样
	for clip := range moduleDrivenClips {
		if !strings.HasPrefix(clip, "Meat/") {
			continue
		}
		a := as.Anims[clip]
		if a == nil {
			t.Errorf("剪辑 %q 缺失", clip)
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
			if p != "" {
				t.Errorf("肉块剪辑 %s 含非根曲线 path %q（模块按根采样会丢失）", clip, p)
			}
		}
	}
}

// TestFloatAttrsSupported：float 曲线属性必须全部在运行时支持集合内。
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
					t.Errorf("剪辑 %s path %q 的属性 %q 运行时不支持", name, path, attr)
				}
			}
		}
	}
}

// TestSpriteSwapsResolve：所有 sprite 换帧关键帧必须解析到图集切片
//（空名 = 该帧隐藏，是合法值）。
func TestSpriteSwapsResolve(t *testing.T) {
	as := loadAssets(t)
	for name, a := range as.Anims {
		for path, keys := range a.Sprites {
			for _, k := range keys {
				if k.Name == "" {
					continue
				}
				if _, ok := as.Sheet.Sprites[k.Name]; !ok {
					t.Errorf("剪辑 %s path %q 引用切片 %q 不在图集", name, path, k.Name)
				}
			}
		}
	}
}

// TestRolesAndRefs：脚本绑定与 Meat 模板引用全部解析到场景节点。
func TestRolesAndRefs(t *testing.T) {
	as := loadAssets(t)
	nodeSet := map[string]bool{}
	for _, n := range as.Rig.Nodes {
		nodeSet[n.Path] = true
	}
	for _, f := range []string{"GrinderText", "MeatBase", "MeatSplash", "BossAnim", "TackAnim", "CartGuyParentAnim", "CartGuyAnim"} {
		p, ok := as.Roles[f]
		if !ok || !nodeSet[p] {
			t.Errorf("role %s -> %q 未解析", f, p)
		}
	}
	tmpl := as.Roles["MeatBase"]
	for _, f := range []string{"startPosition", "startPositionAlt", "hitPosition", "missPosition"} {
		p := as.Extra.ObjRefs[tmpl][f]
		if !nodeSet[p] {
			t.Errorf("Meat.%s -> %q 未解析", f, p)
		}
	}
	if len(as.Extra.ObjSprites[tmpl]["meats"]) != 3 {
		t.Errorf("meats 切片数 = %d, want 3", len(as.Extra.ObjSprites[tmpl]["meats"]))
	}
	for _, s := range as.Extra.ObjSprites[tmpl]["meats"] {
		if _, ok := as.Sheet.Sprites[s]; !ok {
			t.Errorf("meats 切片 %q 不在图集", s)
		}
	}
	if got := len(as.Extra.RefArrayIdx["Gears"]); got != 5 {
		t.Errorf("Gears 数 = %d, want 5", got)
	}
	// 粒子贴图表（手写粒子用 MeatGrinder_0..8）
	for i := 0; i < 9; i++ {
		name := "MeatGrinder_" + string(rune('0'+i))
		if _, ok := as.Sheet.Sprites[name]; !ok {
			t.Errorf("粒子切片 %s 不在图集", name)
		}
	}
}
