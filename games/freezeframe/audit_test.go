package freezeframe

import (
	"testing"

	"hsdemo/kart"
)

func loadAssets(t *testing.T) *kart.Assets {
	t.Helper()
	as, err := kart.Load("../../assets/freezeFrame", 44100)
	if err != nil {
		t.Skipf("assets not extracted: %v", err)
	}
	return as
}

func TestFreezeFrameExtractedBindings(t *testing.T) {
	as := loadAssets(t)
	for _, role := range []string{
		"CameraMan", "Photograph1", "Photograph2", "Photograph3", "Results",
		"IntroSign", "Overlay", "Crosshair", "Shutter", "DimRect",
		"FarCarSpawn", "NearCarSpawn", "WalkerSpawn", "Crowd",
		"CrowdFarLeft", "CrowdLeft", "CrowdRight", "CrowdFarRight", "Billboards",
	} {
		if as.Roles[role] == "" {
			t.Fatalf("missing role %s", role)
		}
	}
	game := as.Extra.Components["game"]
	if got := len(game.RefArrays["Photographs"]); got != 6 {
		t.Fatalf("Photographs count = %d, want 6", got)
	}
	for _, ref := range []string{"FarCarPrefab", "NearCarPrefab", "WalkerPrefab"} {
		if game.Refs[ref] == "" {
			t.Fatalf("missing game ref %s", ref)
		}
		if kart.NewTemplate(as, game.Refs[ref]) == nil {
			t.Fatalf("template %s (%s) not instantiable", ref, game.Refs[ref])
		}
	}
}

func TestFreezeFrameControllersAndSounds(t *testing.T) {
	as := loadAssets(t)
	for ctrl, states := range map[string][]string{
		"CameraMan": {"Idle", "Bop", "Flash", "Happy", "Oops", "Cry"},
		"Crowd":     {"Show", "Hide"},
		"FarCar":    {"Idle", "SlowCarGo", "FastCarGo"},
		"NearCar":   {"Idle", "SlowCarGo", "FastCarGo"},
		"Intro":     {"Enter", "Exit", "Light01", "Light02", "Light03", "LightsOff"},
		"Photograph": {
			"NoCar", "Show", "Hide",
			"SlowCar_Early", "SlowCar_Perfect", "SlowCar_Late",
			"FastCar_Early", "FastCar_Perfect", "FastCar_Late",
			"Cameo_Ninja", "Cameo_Ghost", "Cameo_Rats", "Cameo_PeaceSlow", "Cameo_PeaceFast",
			"Cameo_Girlfriend_Right_Perfect", "Cameo_Dude1_Left_Early", "Cameo_Dude2_Right_Late",
		},
		"Result":  {"None", "Maru", "Sankaku", "Batsu", "ThumbsUp", "ThumbsSide", "ThumbsDown"},
		"Shutter": {"Hold", "Shut"},
		"Walker":  {"Stay", "Dude1", "Dude2", "Girlfriend", "EnterLeft", "EnterRight", "Bop"},
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
	for _, sound := range []string{
		"slowCarFar", "fastCarFar", "fastCarNear", "shutter",
		"pictureShow", "result_Hi", "result_Ok", "result_Ng",
		"beginningSignal1", "beginningSignal2", "common_miss", "common_applause",
	} {
		if len(as.Sounds[sound]) == 0 {
			t.Fatalf("missing sound %s", sound)
		}
	}
}
