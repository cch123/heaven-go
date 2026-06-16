package monkeywatch

import (
	"testing"

	"hsdemo/kart"
)

func loadAssets(t *testing.T) *kart.Assets {
	t.Helper()
	as, err := kart.Load("../../assets/monkeyWatch", 44100)
	if err != nil {
		t.Skipf("assets not extracted: %v", err)
	}
	return as
}

func TestMonkeyWatchExtractedBindings(t *testing.T) {
	as := loadAssets(t)
	for _, role := range []string{"cameraAnchor", "cameraTransform", "cameraMoveable", "monkeyClockArrow", "monkeyHandler", "backgroundHandler", "balloonHandler", "middleMonkey"} {
		if as.Roles[role] == "" {
			t.Fatalf("missing role %s", role)
		}
	}
	for _, root := range []string{"YellowMonkey", "PinkMonkey"} {
		if kart.NewTemplate(as, root) == nil {
			t.Fatalf("missing template %s", root)
		}
	}
	if len(as.Extra.Components["background"].RefArrays["srsIn"]) == 0 || len(as.Extra.Components["background"].RefArrays["srsOut"]) == 0 {
		t.Fatalf("background fade sprite arrays were not extracted")
	}
}

func TestMonkeyWatchControllersAndSounds(t *testing.T) {
	as := loadAssets(t)
	for ctrl, states := range map[string][]string{
		"YellowMonkey":      {"Appear", "Prepare1", "Prepare8", "Clap1", "Clap8", "Just", "Barely", "Miss", "PinkAppear", "PinkPrepare1", "PinkClap1", "PinkJust", "PinkBarely", "PinkMiss"},
		"PlayerMonkey":      {"PlayerIdle", "PlayerClap", "PlayerClapBig", "PlayerClapBarely"},
		"MonkeyClickerAnim": {"Click"},
		"WatchHoleAnim":     {"HoleOpen", "HoleClose"},
		"MiddleMonkey":      {"MiddleMonkeyBop", "MiddleMonkeyMiss"},
		"HotAirBalloon":     {"HotAirBalloon"},
	} {
		c, ok := as.Controllers[ctrl]
		if !ok {
			t.Fatalf("missing controller %s", ctrl)
		}
		for _, st := range states {
			if _, ok := c.States[st]; !ok {
				t.Fatalf("controller %s missing state %s", ctrl, st)
			}
		}
	}
	for _, sound := range []string{"clapOffbeat", "clapOnbeat1", "clapOnbeat5", "voiceUki1", "voiceKi1", "common_miss", "common_nearMiss"} {
		if len(as.Sounds[sound]) == 0 {
			t.Fatalf("missing sound %s", sound)
		}
	}
}

func TestMonkeyWatchAutoSecondState(t *testing.T) {
	m := &Module{
		claps: []clapEvt{
			{beat: 0, min: 5, auto: false},
			{beat: 16, auto: true},
		},
		pinks: []pinkEvt{{beat: 4, length: 3}},
	}
	m.buildMonkeyTimeline()
	m.claps[0].min = m.startSecondForClap(0)
	if got := m.startSecondForClap(1); got != 14 {
		t.Fatalf("auto second = %d, want 14", got)
	}
}

func TestMonkeyWatchUsesCSharpRoundToEven(t *testing.T) {
	if round(4.5) != 4 || round(5.5) != 6 {
		t.Fatalf("round half ties must match C# Math.Round: 4.5=%d 5.5=%d", round(4.5), round(5.5))
	}
}
