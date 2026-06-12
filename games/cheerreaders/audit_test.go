package cheerreaders

// 交付前审计（Code Remix 四件套共用模式）：controller 状态→剪辑、
// animator 绑定路径、roles/refArrays、海报切片命名空间、书窗遮罩。

import (
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
	for _, role := range []string{"playerMask", "missPoster", "topPoster", "middlePoster", "bottomPoster", "player"} {
		if p := as.Roles[role]; p == "" || !nodeSet[p] {
			t.Errorf("role %s = %q 未解析", role, p)
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
