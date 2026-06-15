package chargingchicken

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"hsdemo/kmdata"
)

type chargingChickenAssets struct {
	Rig         kmdata.Rig
	Roles       kmdata.Roles
	Extra       kmdata.Extra
	Anims       map[string]*kmdata.Anim
	Controllers map[string]kmdata.Controller
	Animators   kmdata.Animators
	Sounds      map[string]bool
}

func loadChargingChickenAssets(t *testing.T) *chargingChickenAssets {
	t.Helper()
	root := filepath.Join("..", "..", "assets", "chargingChicken")
	as := &chargingChickenAssets{
		Anims:  map[string]*kmdata.Anim{},
		Sounds: map[string]bool{},
	}
	readAssetJSON(t, filepath.Join(root, "scene.json"), &as.Rig)
	readAssetJSON(t, filepath.Join(root, "roles.json"), &as.Roles)
	readAssetJSON(t, filepath.Join(root, "extra.json"), &as.Extra)
	readAssetJSON(t, filepath.Join(root, "anims.json"), &as.Anims)
	readAssetJSON(t, filepath.Join(root, "controllers.json"), &as.Controllers)
	readAssetJSON(t, filepath.Join(root, "animators.json"), &as.Animators)
	soundRoot := filepath.Join(root, "sounds")
	if err := filepath.WalkDir(soundRoot, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		ext := strings.ToLower(filepath.Ext(p))
		if ext != ".wav" && ext != ".ogg" {
			return nil
		}
		rel, err := filepath.Rel(soundRoot, p)
		if err != nil {
			return err
		}
		as.Sounds[strings.TrimSuffix(filepath.ToSlash(rel), ext)] = true
		return nil
	}); err != nil {
		t.Skipf("assets not extracted: %v", err)
	}
	return as
}

func readAssetJSON(t *testing.T, path string, dst any) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("assets not extracted: %v", err)
	}
	if err := json.Unmarshal(raw, dst); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
}

func TestChargingChickenBindings(t *testing.T) {
	as := loadChargingChickenAssets(t)
	wantRoles := map[string]string{
		"gradient":       "BG/Gradient",
		"bgLow":          "BG/Color2",
		"bgHigh":         "BG/Color1",
		"ChickenAnim":    "Car",
		"WaterAnim":      "Sea",
		"ParallaxFade":   "Parallax",
		"UnParallaxFade": "UnParallax",
		"BirdsAnim":      "Parallax/Birds",
		"Stars":          "Parallax/Stars",
		"Clouds":         "Parallax/CloudySky",
		"Planets":        "Parallax/Planets",
		"Doodles":        "Parallax/Doodles",
		"Birds":          "Parallax/Birds",
		"yardsText":      "Yards Text",
		"endingText":     "ArrivalText",
		"bubbleText":     "BubblePivot/CountBubble/Count Text",
		"countBubble":    "BubblePivot",
		"Helmet":         "Car/Helmet",
		"FallingHelmet":  "Car/FallingCar/CarWindow (1)/Helmet",
		"headlightColor": "Car/CarBody/HeadLight",
	}
	for role, want := range wantRoles {
		if got := as.Roles[role]; got != want {
			t.Fatalf("role %s = %q, want %q", role, got, want)
		}
	}
}

func TestChargingChickenComponents(t *testing.T) {
	as := loadChargingChickenAssets(t)
	game := as.Extra.Components["game"]
	for field, want := range map[string]string{
		"chickenColors":        "chicken_cellshader",
		"chickenColorsCar":     "chicken_carshader",
		"chickenColorsCloud":   "chicken_cloudshader",
		"chickenColorsDoodles": "chicken_mirage",
		"chickenColorsWater":   "chicken_invert",
	} {
		if got := game.Refs[field]; got != want {
			t.Fatalf("game ref %s = %q, want %q", field, got, want)
		}
	}

	island := as.Extra.Components["island"]
	if island.Path != "Island" {
		t.Fatalf("island component path = %q, want Island", island.Path)
	}
	// These Island.cs refs drive runtime platform generation, collapse effects,
	// respawn motion, and helmet toggles; missing any of them would force a
	// runtime port to guess Unity prefab structure.
	for field, want := range map[string]string{
		"ChargerAnim":         "Island/offset/GameObject/Landmass",
		"FakeChickenAnim":     "Island/offset/GameObject/Landmass/Car",
		"PlatformAnim":        "Island/offset/StonePlatform",
		"IslandPos":           "Island",
		"CollapsedLandmass":   "Island/offset/GameObject/Landmass",
		"BigLandmass":         "Island/offset/GameObject/BigLandmass",
		"SmallLandmass":       "Island/offset/GameObject/Landmass/SmallLandmass",
		"FullLandmass":        "Island/offset/GameObject/Landmass",
		"Helmet":              "Island/offset/GameObject/Landmass/Car/Helmet",
		"StoneSplashEffect":   "Island/offset/Splash",
		"IslandCollapse":      "Island/offset/GameObject/Landmass/CollapseParticles",
		"IslandCollapseNg":    "Island/offset/GameObject/Landmass/CollapseParticlesNg",
		"ChickenSplashEffect": "Island/offset/ChickenSplash",
		"GrassL":              "Island/offset/GameObject/BigLandmass/LeftEdge/GrassFallL",
		"GrassR":              "Island/offset/GameObject/BigLandmass/RightEdge/GrassFallR",
		"PlatformBase":        "Island/offset/StonePlatform",
	} {
		if got := island.Refs[field]; got != want {
			t.Fatalf("island ref %s = %q, want %q", field, got, want)
		}
	}
}

func TestChargingChickenControllersAndSounds(t *testing.T) {
	as := loadChargingChickenAssets(t)
	wantStates := map[string][]string{
		"ChickenAnim": {
			"Nothing", "Idle", "Prepare", "Charge", "Ride", "Success",
			"Fall", "TooFar", "Gone", "Back", "Bomb",
			"ChickenLookTo", "ChickenLooking", "ChickenLookFrom",
		},
		"ChargerAnim": {
			"N/A", "Idle", "Prep1", "Prep2", "Prep3", "Prep4", "Pump", "Bounce",
		},
		"FakeChickenAnim": {"Nothing", "Idle", "Burn", "Respawn"},
		"PlatformAnim":    {"Nothing", "Idle", "Set", "Fall", "Plat1", "Plat2"},
		"WaterAnim":       {"Idle", "Scroll", "AntiScroll"},
		"BirdsAnim":       {"Idle", "BirdsFly"},
		"UnParallaxFade": {
			"Idle", "GalaxyIn", "GalaxyOut", "GalaxyEnable", "GalaxyDisable",
			"FutureIn", "FutureOut", "FutureEnable", "FutureDisable",
		},
		"ParallaxFade": {
			"Idle",
			"StarsIn", "StarsOut", "StarsEnable", "StarsDisable",
			"CloudIn", "CloudOut", "CloudEnable", "CloudDisable",
			"EarthIn", "EarthOut", "EarthEnable", "EarthDisable",
			"MarsIn", "MarsOut", "MarsEnable", "MarsDisable",
			"DoodlesIn", "DoodlesOut", "DoodlesEnable", "DoodlesDisable",
			"BirdsIn", "BirdsOut", "BirdsEnable", "BirdsDisable",
		},
	}
	for ctrl, states := range wantStates {
		c, ok := as.Controllers[ctrl]
		if !ok {
			t.Fatalf("missing controller %s", ctrl)
		}
		for _, state := range states {
			cs, ok := c.States[state]
			if !ok {
				t.Errorf("controller %s missing state %s", ctrl, state)
				continue
			}
			if cs.Clip != "" && as.Anims[cs.Clip] == nil {
				t.Errorf("controller %s state %s references missing clip %s", ctrl, state, cs.Clip)
			}
		}
	}

	for path, ctrl := range map[string]string{
		"Car":                                   "ChickenAnim",
		"Sea":                                   "WaterAnim",
		"Parallax":                              "ParallaxFade",
		"UnParallax":                            "UnParallaxFade",
		"Parallax/Birds":                        "BirdsAnim",
		"Island/offset/GameObject/Landmass":     "ChargerAnim",
		"Island/offset/GameObject/Landmass/Car": "FakeChickenAnim",
		"Island/offset/StonePlatform":           "PlatformAnim",
	} {
		if got := as.Animators[path]; got != ctrl {
			t.Fatalf("animator %s = %q, want %q", path, got, ctrl)
		}
	}

	for _, snd := range []string{
		"cowbell", "PumpStart", "chargeLoop", "chargeRelease",
		"SE_CHIKEN_CAR_START", "SE_CHIKEN_CAR_FALL", "SE_CHIKEN_CAR_FALL_WATER",
		"SE_CHIKEN_LAND_RESET", "SE_CHIKEN_BLOCK_SET",
		"SE_CHIKEN_BLOCK_FALL_PITCH150", "SE_CHIKEN_BLOCK_FALL_WATER_PITCH400",
		"SE_CHIKEN_CHARGE_CANCEL", "SE_CHIKEN_DOSHA",
		"SE_NTR_ROBOT_EN_BAKUHATU_PITCH100",
		"kick", "snare", "hihat", "feverkick", "feversnare", "feverhat",
		"dskick", "dssnare", "dshat", "gbakick", "gbasnare", "gbahat",
		"practicekick", "practicesnare", "practicehat",
	} {
		if !as.Sounds[snd] {
			t.Fatalf("missing sound %s", snd)
		}
	}
}

func TestChargingChickenMappedMaterials(t *testing.T) {
	as := loadChargingChickenAssets(t)
	want := map[string]bool{
		"chicken_carshader":   false,
		"chicken_cellshader":  false,
		"chicken_cloudshader": false,
		"chicken_mirage":      false,
		"chicken_starshader":  false,
	}
	for _, n := range as.Rig.Nodes {
		if _, ok := want[n.Mat]; ok && n.Mapped {
			want[n.Mat] = true
		}
	}
	for mat, seen := range want {
		if !seen {
			t.Fatalf("mapped material %s was not exported on any node", mat)
		}
	}
}
