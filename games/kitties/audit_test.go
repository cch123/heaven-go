package kitties

// 交付前审计：controller/绑定/roles/音效/生成坐标表。

import (
	"testing"

	"hsdemo/kart"
)

func loadAssets(t *testing.T) *kart.Assets {
	t.Helper()
	as, err := kart.Load("../../assets/kitties", 44100)
	if err != nil {
		t.Skipf("assets not extracted: %v", err)
	}
	return as
}

func TestControllersResolve(t *testing.T) {
	as := loadAssets(t)
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
	k := as.Controllers["Kitty"]
	for _, st := range []string{"PopIn", "MicePopIn", "FacePopIn", "Clap1", "Clap2",
		"MiceClap1", "MiceClap2", "FaceClap", "RollStart", "Rolling", "RollEnd",
		"RollFail", "ClapMiss", "FishNotice", "FishNotice2", "FishNotice3"} {
		if _, ok := k.States[st]; !ok {
			t.Errorf("Kitty 缺状态 %s", st)
		}
	}
}

func TestCatsAndSounds(t *testing.T) {
	as := loadAssets(t)
	nodeSet := map[string]bool{}
	for _, n := range as.Rig.Nodes {
		nodeSet[n.Path] = true
	}
	cats := as.Extra.RefArrays["Cats"]
	if len(cats) != 3 {
		t.Fatalf("Cats = %d, want 3", len(cats))
	}
	for _, p := range cats {
		if !nodeSet[p] || !nodeSet[p+"/Kitty"] {
			t.Errorf("猫 %q 或其 Kitty 子体不在场景树", p)
		}
	}
	if len(as.Extra.RefArrayIdx["Cats"]) != 3 {
		t.Errorf("Cats 下标缺失（SetSpinIdx 需要）")
	}
	for _, k := range []string{"nya1_1", "nya1_2", "nya2_1", "nya3_2", "clapMiss1",
		"clapMiss2", "clapPlayer1", "clapPlayer2", "tink", "roll1", "roll4", "roll6",
		"spin1", "spin10", "spinnya", "spinplayer1", "spinplayer5", "fish1", "fish4"} {
		if _, ok := as.Sounds[k]; !ok {
			t.Errorf("音效 %q 缺失", k)
		}
	}
}
