package catchytune

import (
	"testing"

	"hsdemo/kart"
)

func TestFruitScheduleMatchesUnityPreDrop(t *testing.T) {
	tests := []struct {
		name       string
		pineapple  bool
		wantStart  float64
		wantSpawn  float64
		wantJudge  float64
		wantVisDur float64
		wantBarely float64
		wantMiss   float64
	}{
		{"orange", false, 9, 9, 13, 6, 1, 1.5},
		{"pineapple", true, 8, 9, 16, 12, 2, 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fruitSchedule(10, tt.pineapple)
			if got.startBeat != tt.wantStart || got.spawnBeat != tt.wantSpawn || got.judgeBeat != tt.wantJudge {
				t.Fatalf("timing = start %.1f spawn %.1f judge %.1f, want %.1f %.1f %.1f",
					got.startBeat, got.spawnBeat, got.judgeBeat, tt.wantStart, tt.wantSpawn, tt.wantJudge)
			}
			if got.visibleDur != tt.wantVisDur || got.barelyDur != tt.wantBarely || got.missDelay != tt.wantMiss {
				t.Fatalf("durations = visible %.1f barely %.1f miss %.1f, want %.1f %.1f %.1f",
					got.visibleDur, got.barelyDur, got.missDelay, tt.wantVisDur, tt.wantBarely, tt.wantMiss)
			}
		})
	}
}

func TestCatchyTuneExtractedAssets(t *testing.T) {
	as, err := kart.Load("../../assets/catchyTune", 44100)
	if err != nil {
		t.Skipf("assets not extracted: %v", err)
	}
	for _, role := range []string{
		"plalinAnim", "alalinAnim", "orangeBase", "pineappleBase",
		"fruitHolder", "heartMessage", "bg2",
	} {
		if as.Roles[role] == "" {
			t.Errorf("role %s 未解析", role)
		}
	}
	for _, clip := range []string{
		"Characters/bop", "Characters/catch orange", "Characters/catch pineapple",
		"Characters/idle", "Characters/miss", "Characters/miss pineapple",
		"Characters/smile", "Characters/stopsmile", "Characters/whiff",
		"Fruit/orange bounce", "Fruit/pineapple bounce", "Fruit/fruit barely",
	} {
		if _, ok := as.Anims[clip]; !ok {
			t.Errorf("缺动画剪辑 %s", clip)
		}
	}
	for _, snd := range []string{
		"leftOrange", "rightOrange", "leftPineapple", "rightPineapple",
		"leftOrangeCatch", "rightOrangeCatch", "leftPineappleCatch", "rightPineappleCatch",
		"fruitThrough", "whiff", "barely left", "barely right",
	} {
		if _, ok := as.Sounds[snd]; !ok {
			t.Errorf("缺音效 %s", snd)
		}
	}
	if _, ok := as.Controllers["Alalin and plalin"]; !ok {
		t.Error("缺角色 AnimatorController")
	}
	if _, ok := as.Controllers["orange"]; !ok {
		t.Error("缺 orange AnimatorController")
	}
	if kart.NewTemplate(as, as.Roles["orangeBase"]) == nil {
		t.Error("orangeBase 模板未解析")
	}
	pineappleT := kart.NewTemplate(as, as.Roles["pineappleBase"])
	if pineappleT == nil {
		t.Fatal("pineappleBase 模板未解析")
	}
	// Pineapple 的 controller 在 Unity 中复用游戏目录外资源，提取器只保留
	// Fruit/pineapple bounce 剪辑；实例 raw-clip 采样必须能覆盖这个缺口。
	inst := pineappleT.NewInstance()
	inst.PlayNormalized("", "Fruit/pineapple bounce", 0.5)
	inst.Queue(kart.NewScene(as), 0, kart.Identity(), 0)
}
