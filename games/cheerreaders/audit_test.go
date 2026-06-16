package cheerreaders

// 交付前审计（Code Remix 四件套共用模式）：controller 状态→剪辑、
// animator 绑定路径、roles/refArrays、海报切片命名空间、书窗遮罩。

import (
	"math"
	"testing"

	"hsdemo/kart"
)

func loadAssets(t *testing.T) *kart.Assets {
	t.Helper()
	as, err := kart.Load("../../assets/cheerReaders", 44100)
	if err != nil {
		t.Skipf("assets not extracted: %v", err)
	}
	return as
}

// TestControllersResolve：每个 controller 状态的剪辑都存在；
// animators.json 绑定路径都在场景树。
func TestControllersResolve(t *testing.T) {
	as := loadAssets(t)
	for name, ctrl := range as.Controllers {
		if name == "Facial Features" {
			continue // 空 controller（prefab 死数据：无状态）
		}
		for st, s := range ctrl.States {
			if name == "BaseAnim" && st == "OpenIdle" {
				// prefab 死数据：motion guid 50e61f71… 的 .anim 已从上游仓库
				// 删除，且无任何转换进入该状态（OpenBook 保持末帧）
				continue
			}
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

// TestRowsAndMasks：12 个 NPC（4+4+3）+ 玩家 + 对应书窗遮罩。
func TestRowsAndMasks(t *testing.T) {
	as := loadAssets(t)
	want := map[string]int{
		"firstRow": 4, "secondRow": 4, "thirdRow": 3,
		"topMasks": 4, "middleMasks": 4, "bottomMasks": 3,
	}
	nodeSet := map[string]bool{}
	maskCnt := 0
	for _, n := range as.Rig.Nodes {
		nodeSet[n.Path] = true
		if n.Mask {
			maskCnt++
		}
	}
	for key, n := range want {
		arr := as.Extra.RefArrays[key]
		if len(arr) != n {
			t.Errorf("%s = %d, want %d", key, len(arr), n)
		}
		for _, p := range arr {
			if !nodeSet[p] {
				t.Errorf("%s 引用 %q 不在场景树", key, p)
			}
		}
	}
	// 24 个 SpriteMask（12 书 × 2 半页）+ playerBook 下的 2 个
	if maskCnt < 24 {
		t.Errorf("SpriteMask 节点 = %d, want >= 24", maskCnt)
	}
	for _, role := range []string{
		"playerMask", "missPoster", "topPoster", "middlePoster", "bottomPoster", "player",
		"CheerCaption0", "CheerCaption1", "CheerUnderlay0", "CheerUnderlay1", "StickyCaptions",
	} {
		if p := as.Roles[role]; p == "" || !nodeSet[p] {
			t.Errorf("role %s = %q 未解析", role, p)
		}
	}
}

// TestPlayableGirlsUsePepSquadPrefab：根部 GirlHolder 是不参与脚本播放的
// authoring 模板；真正会 PlayState 的 NPC/player 都是 pepSquad 实例，必须带有
// BaseAnim 翻书剪辑引用的 Body (1) 与 OpenBook 页面。
func TestPlayableGirlsUsePepSquadPrefab(t *testing.T) {
	as := loadAssets(t)
	nodeSet := map[string]bool{}
	for _, n := range as.Rig.Nodes {
		nodeSet[n.Path] = true
	}
	for _, missing := range []string{"GirlHolder/Body (1)", "GirlHolder/OpenBook1", "GirlHolder/OpenBook2"} {
		if nodeSet[missing] {
			t.Fatalf("authoring template unexpectedly gained runtime-only node %q", missing)
		}
	}
	for _, key := range []string{"firstRow", "secondRow", "thirdRow"} {
		for _, p := range as.Extra.RefArrays[key] {
			assertPepSquadBookNodes(t, nodeSet, p)
		}
	}
	assertPepSquadBookNodes(t, nodeSet, as.Roles["player"])
}

func assertPepSquadBookNodes(t *testing.T, nodeSet map[string]bool, root string) {
	t.Helper()
	for _, sub := range []string{"/Body (1)", "/OpenBook1", "/OpenBook2"} {
		if !nodeSet[root+sub] {
			t.Fatalf("%s missing required BaseAnim node %s", root, sub)
		}
	}
}

// TestCaptionTextNodes：toggleCaption 使用原 prefab 的 TMP 文本/underlay 节点；
// 这些节点必须能被运行时 SetText 动态改字，不能退回手写 HUD。
func TestCaptionTextNodes(t *testing.T) {
	as := loadAssets(t)
	if len(as.Texts) != 4 {
		t.Fatalf("texts = %d, want 4 caption/underlay nodes", len(as.Texts))
	}
	texts := map[string]bool{}
	for _, tn := range as.Texts {
		texts[tn.Path] = true
		if tn.HAlign != 2 {
			t.Errorf("%s hAlign = %d, want center", tn.Path, tn.HAlign)
		}
	}
	for _, font := range []string{"Seurat B.otf", "SeuratBHolelessM.ttf"} {
		if _, ok := as.Fonts[font]; !ok {
			t.Errorf("font %q not extracted", font)
		}
	}
	for _, role := range []string{"CheerCaption0", "CheerCaption1", "CheerUnderlay0", "CheerUnderlay1"} {
		p := as.Roles[role]
		if !texts[p] {
			t.Errorf("%s role path %q not in texts.json", role, p)
		}
	}
	if err := as.ApplyTexts(); err != nil {
		t.Fatalf("ApplyTexts: %v", err)
	}
	for _, role := range []string{"CheerCaption0", "CheerUnderlay0"} {
		if err := as.SetText(as.Roles[role], "One! Two! Three!"); err != nil {
			t.Fatalf("SetText(%s): %v", role, err)
		}
	}
}

// TestPosterSprites：14 套海报的 4 个切片全部可解析（含命名空间回退）。
func TestPosterSprites(t *testing.T) {
	as := loadAssets(t)
	for _, file := range posterFiles {
		for _, part := range []string{"TopPart", "MiddlePart", "BottomPart", "Miss"} {
			if _, ok := as.Sheet.Sprites[file+"/"+part]; ok {
				continue
			}
			if _, ok := as.Sheet.Sprites[part]; ok {
				continue // 首扫描文件未命名空间化
			}
			t.Errorf("海报 %s/%s 切片缺失", file, part)
		}
	}
}

// TestGirlFaces：每个 NPC 的 faceSprites 子节点与 Blush 对存在。
func TestGirlFaces(t *testing.T) {
	as := loadAssets(t)
	nodeSet := map[string]bool{}
	for _, n := range as.Rig.Nodes {
		nodeSet[n.Path] = true
	}
	for _, key := range []string{"firstRow", "secondRow", "thirdRow"} {
		for _, p := range as.Extra.RefArrays[key] {
			for _, sub := range []string{"/Head/faceSprites", "/Head/faceSprites/Blush", "/Head/faceSprites/Blush (1)"} {
				if !nodeSet[p+sub] {
					t.Errorf("%s%s 不在场景树", p, sub)
				}
			}
		}
	}
}

// TestVoiceSounds：三套语音（Solo/Girls + All/yay）按相对路径 key 加载。
func TestVoiceSounds(t *testing.T) {
	as := loadAssets(t)
	for _, k := range []string{
		"Solo/123/oneTwoThreeS1", "Girls/123/onegirls",
		"Solo/UpToYou/itsUpToYouS5", "Girls/UpToYou/yougirls",
		"Solo/LetsGoRead/bunchaBooksS9", "Girls/LetsGoRead/bunchaBooksgirls9",
		"Solo/RRSBBB/rahRahSisBoomBaBoomS6", "Girls/RRSBBB/rahRahSisBoomBaBoomgirls6",
		"Solo/OKItsOn/okItsOnS5", "Girls/OKItsOn/okItsOngirls5",
		"All/yay", "Solo/yayS", "Girls/yayGirls",
		"bookHorizontal", "bookVertical", "bookDiagonal", "bookBoom",
		"bookSpin", "bookSpinLoop", "bookOpen", "bookPlayer",
		"whistle1", "whistle2", "letsGoRead", "doingoing",
	} {
		if _, ok := as.Sounds[k]; !ok {
			t.Errorf("音效 %q 缺失", k)
		}
	}
}

// TestYayParticleSystems：Yay() 直接 Play 的 WhiteParticle/BlackParticle
// 必须保留为可定位的原 prefab 粒子节点，并使用各自 UVModule 里的纸花切片。
func TestYayParticleSystems(t *testing.T) {
	as := loadAssets(t)
	nodeSet := map[string]bool{}
	for _, n := range as.Rig.Nodes {
		nodeSet[n.Path] = true
	}
	for role, wantSprite := range map[string]string{
		"whiteYayParticle": cheerConfettiWhiteSprite,
		"blackYayParticle": cheerConfettiBlackSprite,
	} {
		if p := as.Roles[role]; p == "" || !nodeSet[p] {
			t.Fatalf("role %s = %q 未解析到场景节点", role, p)
		}
		sp, ok := as.Sheet.Sprites[wantSprite]
		if !ok {
			t.Fatalf("%s sprite %q 缺失", role, wantSprite)
		}
		if sp.PPU != 100 || sp.PivotX != 0.5 || sp.PivotY != 0.5 {
			t.Fatalf("%s sprite metadata = %#v, want ppu=100 pivot=(0.5,0.5)", wantSprite, sp)
		}
	}
	if got, want := as.Sheet.Sprites[cheerConfettiWhiteSprite].W, 276; got != want {
		t.Fatalf("%s width = %d, want %d", cheerConfettiWhiteSprite, got, want)
	}
	if got, want := as.Sheet.Sprites[cheerConfettiBlackSprite].W, 275; got != want {
		t.Fatalf("%s width = %d, want %d", cheerConfettiBlackSprite, got, want)
	}
}

func TestYayParticlePrefabConstants(t *testing.T) {
	if cheerConfettiParticleCount != 25 {
		t.Fatalf("particle count = %d, want 25 from rate 50 * length 0.5", cheerConfettiParticleCount)
	}
	checkNear(t, "lifetime", cheerConfettiLifetimeSec, 0.32)
	checkNear(t, "shape width", cheerConfettiShapeWidth, 13)
	checkNear(t, "shape height", cheerConfettiShapeHeight, 7)
	checkNear(t, "start rotation", cheerConfettiStartRotMax, 0.87266463)
	checkNear(t, "rotation over lifetime", cheerConfettiSpin, 13.08997)
	if cheerConfettiOrder != 99 {
		t.Fatalf("sorting order = %d, want 99", cheerConfettiOrder)
	}
}

func TestYayParticleCurves(t *testing.T) {
	checkNear(t, "alpha at birth", cheerConfettiAlpha(0), 0)
	checkNear(t, "alpha peak", cheerConfettiAlpha(cheerConfettiAlphaPeak), 1)
	checkNear(t, "alpha death", cheerConfettiAlpha(1), 0)
	checkNear(t, "size key0", cheerConfettiSize(0), cheerConfettiSizeKey0V)
	checkNear(t, "size key1", cheerConfettiSize(cheerConfettiSizeKey1T), cheerConfettiSizeKey1V)
	if cheerConfettiSize(0.5) < 0.9 {
		t.Fatalf("Hermite size curve midlife = %v, expected Unity tangent overshoot", cheerConfettiSize(0.5))
	}
}

func checkNear(t *testing.T, name string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-6 {
		t.Fatalf("%s = %v, want %v", name, got, want)
	}
}
