package boardmeeting

import (
	"testing"

	"hsdemo/kart"
)

func TestBoardMeetingExtractedAssets(t *testing.T) {
	as, err := kart.Load("../../assets/boardMeeting", 44100)
	if err != nil {
		t.Skipf("assets not extracted: %v", err)
	}
	for _, role := range []string{"farLeft", "farRight", "assistantAnim"} {
		if as.Roles[role] == "" {
			t.Errorf("role %s 未解析", role)
		}
	}
	if as.Roles["farLeft"] != "bm_farLeftPos" || as.Roles["farRight"] != "bm_farRightPos" {
		t.Fatalf("far anchors = %#v", as.Roles)
	}
	for _, ctrl := range []string{"bm_assistant", "bm_executive"} {
		if _, ok := as.Controllers[ctrl]; !ok {
			t.Errorf("缺 controller %s", ctrl)
		}
	}
	for _, clip := range []string{
		"Assistant/Bop", "Assistant/Idle", "Assistant/MissBop", "Assistant/MissIdle",
		"Assistant/One", "Assistant/Stop", "Assistant/Three",
		"Executive/Bop", "Executive/Idle", "Executive/LoopSpin", "Executive/Miss",
		"Executive/Prepare", "Executive/SmileBop", "Executive/SmileIdle",
		"Executive/Spin", "Executive/Stop",
	} {
		if _, ok := as.Anims[clip]; !ok {
			t.Errorf("缺动画剪辑 %s", clip)
		}
	}
	if kart.NewTemplate(as, "bm_executive") == nil {
		t.Fatal("bm_executive template 未解析")
	}
	for _, snd := range []string{
		"chairLoop", "miss", "missThrough", "one", "prepare",
		"rollA", "rollB", "rollC", "rollPlayer",
		"rollPrepareA", "rollPrepareB", "rollPrepareC", "rollPreparePlayer",
		"stopA", "stopAll", "stopAllPlayer", "stopB", "stopC", "stopPlayer",
		"three", "two", "twoUra",
	} {
		if _, ok := as.Sounds[snd]; !ok {
			t.Errorf("缺音效 %s", snd)
		}
	}
}

func TestBoardMeetingSoundNamesMatchUnityIndexRules(t *testing.T) {
	m := &Module{execCount: 5}
	if got := m.spinSoundName(4); got != "Player" {
		t.Fatalf("5th executive spin sound = %q", got)
	}
	if got := m.stopSoundName(2); got != "stopB" {
		t.Fatalf("3rd executive stop sound = %q", got)
	}
	m.execCount = 3
	if got := m.spinSoundName(2); got != "C" {
		t.Fatalf("3-pig player spin sound = %q", got)
	}
	if got := m.stopSoundName(0); got != "stopA" {
		t.Fatalf("3-pig first stop sound = %q", got)
	}
}

func TestApplyExecutiveOrderKeepsLaterExecutivesInFront(t *testing.T) {
	as, err := kart.Load("../../assets/boardMeeting", 44100)
	if err != nil {
		t.Skipf("assets not extracted: %v", err)
	}
	tm := kart.NewTemplate(as, "bm_executive")
	if tm == nil {
		t.Fatal("bm_executive template 未解析")
	}
	in := tm.NewInstance()
	applyExecutiveOrder(in, 4)
	in.SetActive("", true)
	in.PlayDefaultState("", 0, 1)
}
