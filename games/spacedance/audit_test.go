package spacedance

// 交付前审计：controller/绑定/roles/音效/星空层。

import (
	"testing"

	"hsdemo/kart"
)

func loadAssets(t *testing.T) *kart.Assets {
	t.Helper()
	as, err := kart.Load("../../assets/spaceDance", 44100)
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
	d := as.Controllers["dancer"]
	for _, st := range []string{"Bop", "Ouch", "PunchDo", "PunchStartInner", "PunchStartOuter",
		"SitDownDo", "SitDownStart", "TurnRightDo", "TurnRightStart"} {
		if _, ok := d.States[st]; !ok {
			t.Errorf("dancer 缺状态 %s", st)
		}
	}
	g := as.Controllers["gramps"]
	for _, st := range []string{"GrampsBop", "GrampsMiss", "GrampsOhFuck", "GrampsPunchDo",
		"GrampsSitDownDo", "GrampsTurnRightDo", "GrampsTalk", "GrampsSniff", "GrampsStand"} {
		if _, ok := g.States[st]; !ok {
			t.Errorf("gramps 缺状态 %s", st)
		}
	}
}

func TestRolesSoundsStars(t *testing.T) {
	as := loadAssets(t)
	nodeSet := map[string]bool{}
	for _, n := range as.Rig.Nodes {
		nodeSet[n.Path] = true
	}
	for _, r := range []string{"DancerP", "Dancer1", "Dancer2", "Dancer3", "Gramps", "Hit", "bg", "shootingStarAnim"} {
		if p := as.Roles[r]; p == "" || !nodeSet[p] {
			t.Errorf("role %s = %q 未解析", r, p)
		}
	}
	for _, k := range []string{"voicelessTurn", "voicelessSit", "voicelessPunch",
		"dancerTurn", "dancerRight", "dancerLets", "dancerSit", "dancerDown",
		"dancerPa", "dancerPunch", "otherTurn", "otherRight", "otherLets", "otherSit",
		"otherDown", "otherPa", "otherPunch", "inputGood", "inputBad", "inputBad2"} {
		if _, ok := as.Sounds[k]; !ok {
			t.Errorf("音效 %q 缺失", k)
		}
	}
	// 星空层贴图（UV 平铺等价实现的素材）
	for _, l := range starLayers {
		if _, ok := as.Sheet.Sprites[l.sprite]; !ok {
			t.Errorf("星空切片 %q 缺失", l.sprite)
		}
	}
}
